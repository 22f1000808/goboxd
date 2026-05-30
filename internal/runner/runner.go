package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"goboxd/internal/config"
	"goboxd/internal/jail"
	"goboxd/internal/types"
)

type Runner struct {
	srv      *config.ServerConfig
	registry *config.Registry
}

func New(srv *config.ServerConfig, reg *config.Registry) *Runner {
	return &Runner{srv: srv, registry: reg}
}

func (r *Runner) Run(ctx context.Context, req *types.RunRequest) (*types.RunResponse, error) {
	lang, _ := r.registry.Lookup(req.Language)
	if lang.Build != nil {
		return nil, fmt.Errorf("build phase not supported in this phase")
	}

	ws, cleanup, err := newWorkspace(r.srv.JailRootDir)
	if err != nil {
		return nil, fmt.Errorf("workspace: %w", err)
	}
	defer cleanup()

	filename := req.SourceFilename
	if filename == "" {
		filename = lang.SourceFilename
	}
	srcPath := filepath.Join(ws.SandboxPath, filename)
	if err := os.WriteFile(srcPath, []byte(req.Source), 0644); err != nil {
		return nil, fmt.Errorf("write source: %w", err)
	}

	resp := &types.RunResponse{Tests: make([]types.TestResult, len(req.Tests))}
	jailSrc := filepath.Join("/work", filename)
	limits := mergeLimits(lang.Run.Limits, runLimits(req))
	args := expandArgs(lang.Run.Args, jailSrc, "", flagsFor(req.Run, lang.Run))

	for i, t := range req.Tests {
		jctx, cancel := context.WithTimeout(ctx, time.Duration(limits.WallTimeS)*time.Second)
		mounts := []jail.Mount{
			{HostPath: ws.SandboxPath, JailPath: "/work", ReadOnly: true},
			{HostPath: "/usr", JailPath: "/usr", ReadOnly: true},
			{HostPath: "/lib", JailPath: "/lib", ReadOnly: true},
			{HostPath: "/lib64", JailPath: "/lib64", ReadOnly: true},
		}
		job := jail.Job{
			Name:         fmt.Sprintf("%s-test-%d", lang.ID, i),
			Workspace:    ws,
			Cmd:          lang.Run.Cmd,
			Args:         args,
			Stdin:        t.Stdin,
			Mounts:       mounts,
			WallTimeS:    limits.WallTimeS,
			MemoryKB:     limits.MemoryKB,
			MaxProcesses: limits.MaxProcesses,
		}

		out, runErr := jail.Execute(jctx, r.srv.NSJailBinary, job)
		cancel()
		if runErr != nil {
			resp.Tests[i] = types.TestResult{Status: types.StatusInternalError}
			continue
		}

		resp.Tests[i] = types.TestResult{
			Status:       jail.Classify(out, t.ExpectedStdout),
			Stdout:       string(out.Stdout),
			Stderr:       string(out.Stderr),
			DurationMS:   out.DurationMS,
			MemoryPeakKB: out.MemoryPeakKB,
		}
	}

	resp.Status = resolveTopLevel("", resp.Tests)
	return resp, nil
}

func runLimits(req *types.RunRequest) *types.Limits {
	if req.Run == nil {
		return nil
	}
	return req.Run.Limits
}

func mergeLimits(def config.Limits, ov *types.Limits) config.Limits {
	out := def
	if ov == nil {
		return out
	}
	if ov.WallTimeS > 0 {
		out.WallTimeS = ov.WallTimeS
	}
	if ov.MemoryKB > 0 {
		out.MemoryKB = ov.MemoryKB
	}
	if ov.MaxProcesses > 0 {
		out.MaxProcesses = ov.MaxProcesses
	}
	return out
}

func flagsFor(phase *types.Phase, _ *config.PhaseSpec) []string {
	if phase == nil {
		return nil
	}
	return phase.Flags
}

func expandArgs(tmpl []string, source, artifact string, flags []string) []string {
	out := make([]string, 0, len(tmpl)+len(flags))
	for _, a := range tmpl {
		switch a {
		case "{{flags}}":
			out = append(out, flags...)
		default:
			a = strings.ReplaceAll(a, "{{source}}", source)
			a = strings.ReplaceAll(a, "{{artifact}}", artifact)
			out = append(out, a)
		}
	}
	return out
}
