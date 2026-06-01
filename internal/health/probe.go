// internal/health/probe.go
package health

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"goboxd/internal/config"
)

type ProbeResult struct {
	OK      bool   `json:"ok"`
	Version string `json:"version,omitempty"`
	Error   string `json:"error,omitempty"`
}

type ReadinessReport struct {
	Status    string                 `json:"status"`
	NSJail    ProbeResult            `json:"nsjail"`
	Languages map[string]ProbeResult `json:"languages"`
	CachedAt  time.Time              `json:"cached_at"`
}

func (r ReadinessReport) Ready() bool { return r.Status == StatusReadyOK }

const (
	StatusReadyOK       = "ok"
	StatusReadyDegraded = "degraded"
)

type Probe struct {
	registry  *config.Registry
	nsjailBin string
	ttl       time.Duration

	mu       sync.Mutex
	cached   *ReadinessReport
	inflight chan struct{}
}

func NewProbe(reg *config.Registry, nsjailBin string, ttl time.Duration) *Probe {
	return &Probe{registry: reg, nsjailBin: nsjailBin, ttl: ttl}
}

func (p *Probe) Check(ctx context.Context) ReadinessReport {
	p.mu.Lock()
	if p.cached != nil && time.Since(p.cached.CachedAt) < p.ttl {
		r := snapshotReport(p.cached)
		p.mu.Unlock()
		return r
	}
	if p.inflight != nil {
		wait := p.inflight
		p.mu.Unlock()
		<-wait
		p.mu.Lock()
		var r ReadinessReport
		if p.cached != nil {
			r = snapshotReport(p.cached)
		}
		p.mu.Unlock()
		return r
	}

	done := make(chan struct{})
	p.inflight = done
	p.mu.Unlock()

	report := p.runProbes(ctx)

	p.mu.Lock()
	cp := report
	p.cached = &cp
	p.inflight = nil
	p.mu.Unlock()
	close(done)
	return snapshotReport(&report)
}

func (p *Probe) runProbes(ctx context.Context) ReadinessReport {
	report := ReadinessReport{
		Languages: make(map[string]ProbeResult),
		CachedAt:  time.Now().UTC(),
	}
	// For nsjail, try --version first (newer versions support it), fall back to -h
	report.NSJail = probeNSJail(ctx, p.nsjailBin)

	ids := p.registry.IDs()
	sem := make(chan struct{}, runtime.NumCPU())
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, id := range ids {
		lang, _ := p.registry.Lookup(id)
		bin, args := probeTarget(lang)
		wg.Add(1)
		sem <- struct{}{}
		go func(id, bin string, args []string) {
			defer wg.Done()
			defer func() { <-sem }()
			res := probeOne(ctx, bin, args)
			mu.Lock()
			report.Languages[id] = res
			mu.Unlock()
		}(id, bin, args)
	}
	wg.Wait()

	ready := report.NSJail.OK
	for _, r := range report.Languages {
		if !r.OK {
			ready = false
			break
		}
	}
	if ready {
		report.Status = StatusReadyOK
	} else {
		report.Status = StatusReadyDegraded
	}

	return report
}

func snapshotReport(r *ReadinessReport) ReadinessReport {
	cp := *r
	cp.Languages = make(map[string]ProbeResult, len(r.Languages))
	for k, v := range r.Languages {
		cp.Languages[k] = v
	}
	return cp
}

func (p *Probe) Invalidate() {
	p.mu.Lock()
	p.cached = nil
	p.mu.Unlock()
}

func (p *Probe) Cached() ReadinessReport {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cached == nil {
		return ReadinessReport{}
	}
	return snapshotReport(p.cached)
}

func probeTarget(lang *config.LanguageSpec) (string, []string) {
	args := lang.SmokeProbe
	if len(args) == 0 {
		args = []string{"--version"}
	}
	switch {
	case lang.SmokeProbeCmd != "":
		return lang.SmokeProbeCmd, args
	case lang.Build != nil && lang.Build.Cmd != "":
		return lang.Build.Cmd, args
	default:
		return lang.Run.Cmd, args
	}
}

func probeOne(ctx context.Context, bin string, args []string) ProbeResult {
	if bin == "" {
		return ProbeResult{Error: "no probe binary"}
	}
	pctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(pctx, bin, args...).CombinedOutput()
	if err != nil {
		return ProbeResult{Error: trimErr(err, out)}
	}
	return ProbeResult{OK: true, Version: firstLine(string(out))}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func probeNSJail(ctx context.Context, bin string) ProbeResult {
	if bin == "" {
		return ProbeResult{Error: "no probe binary"}
	}
	// Try --version first (newer nsjail versions support it)
	result := probeOne(ctx, bin, []string{"--version"})
	if result.OK {
		return result
	}
	// Fall back to -h for older nsjail versions (like 3.4)
	result = probeOne(ctx, bin, []string{"-h"})
	if result.OK {
		return ProbeResult{OK: true, Version: "available"}
	}
	// If both failed, return the original error
	return result
}

func trimErr(err error, out []byte) string {
	msg := err.Error()
	if len(out) > 0 {
		msg += ": " + firstLine(string(out))
	}
	if len(msg) > 200 {
		msg = msg[:200]
	}
	return msg
}
