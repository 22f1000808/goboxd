// internal/runner/sweeper.go
package runner

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Sweeper removes orphan jail directories. Phase 3 only ships the
// background ticker (every 5 min, dirs older than 10 min). Phase 4
// adds a synchronous boot sweep paired with the security hardening.
type Sweeper struct {
	root   string
	prefix string
	maxAge time.Duration
	tick   time.Duration
	log    *slog.Logger
}

func NewSweeper(jailRoot string, log *slog.Logger) *Sweeper {
	return &Sweeper{
		root:   jailRoot,
		prefix: "job-",
		maxAge: 10 * time.Minute,
		tick:   5 * time.Minute,
		log:    log,
	}
}

// Run blocks until ctx is cancelled, sweeping every s.tick.
func (s *Sweeper) Run(ctx context.Context) {
	t := time.NewTicker(s.tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.sweepOnce()
		}
	}
}

func (s *Sweeper) sweepOnce() {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		if !os.IsNotExist(err) {
			s.log.Warn("sweep: readdir", "err", err)
		}
		return
	}

	cutoff := time.Now().Add(-s.maxAge)
	removed := 0
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), s.prefix) {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		if info.ModTime().After(cutoff) {
			continue
		}

		path := filepath.Join(s.root, e.Name())
		if err := os.RemoveAll(path); err != nil {
			s.log.Warn("sweep: remove", "path", path, "err", err)
			continue
		}

		removed++
	}

	if removed > 0 {
		s.log.Info("sweep complete", "removed", removed)
	}
}
