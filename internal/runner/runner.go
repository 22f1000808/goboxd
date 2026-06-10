// internal/runner/runner.go
// Package runner orchestrates a single /run request: workspace lifecycle,
// optional build phase, per-test execution, and top-level status
// resolution.
package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

// Run executes a validated request. The validator must have run first;
// Run does not re-screen inputs.
func (r *Runner) Run(ctx context.Context, req *types.RunRequest) (*types.RunResponse, error) {
	lang, _ := r.registry.Lookup(req.Language)

	ws, cleanup, err := newWorkspace(r.srv.JailRootDir)
	if err != nil {
		return nil, fmt.Errorf("workspace: %w", err)
	}
	defer cleanup()

	filename := req.SourceFilename
	if filename == "" {
		filename = lang.SourceFilename
	}

	if err := os.WriteFile(filepath.Join(ws.SandboxPath, filename), []byte(req.Source), 0644); err != nil {
		return nil, fmt.Errorf("write source: %w", err)
	}

	jailSrc := filepath.Join("/work", filename)
	// Artifact: client may override for from_request strategy languages (e.g. Java).
	jailArtifact := lang.Artifact
	if req.ArtifactFilename != "" {
		jailArtifact = req.ArtifactFilename
	}

	resp := &types.RunResponse{Tests: make([]types.TestResult, len(req.Tests))}

	if lang.Build != nil {
		br, ok, berr := r.runBuild(ctx, lang, ws, jailSrc, jailArtifact, req.Build)
		if berr != nil {
			return nil, berr
		}
		resp.Build = br
		if !ok {
			// Build failed (or hit an internal error). Per §4.5: mark
			// every test not_executed and short-circuit.
			for i := range req.Tests {
				resp.Tests[i] = types.TestResult{Status: types.StatusNotExecuted}
			}
			resp.Status = resolveTopLevel(br.Status, resp.Tests)
			return resp, nil
		}
	}

	runLim := mergeLimits(lang.Run.Limits, phaseLimits(req.Run))
	runArgs := expandArgs(lang.Run.Args, jailSrc, jailArtifact, flagsFor(req.Run, lang.Run))
	runCmd := expandCmd(lang.Run.Cmd, jailSrc, jailArtifact)

	for i, t := range req.Tests {
		jctx, cancel := context.WithTimeout(ctx, time.Duration(runLim.WallTimeS+2)*time.Second)
		out, runErr := jail.Execute(jctx, r.srv.NSJailBinary, jail.Job{
			Name:         fmt.Sprintf("%s-test-%d", lang.ID, i),
			Workspace:    ws,
			Cmd:          runCmd,
			Args:         runArgs,
			Stdin:        t.Stdin,
			Mounts:       []jail.Mount{{HostPath: ws.SandboxPath, JailPath: "/work", ReadOnly: true}},
			WallTimeS:    runLim.WallTimeS,
			MemoryKB:     runLim.MemoryKB,
			MaxProcesses: runLim.MaxProcesses,
		})
		cancel()
		if runErr != nil {
			resp.Tests[i] = types.TestResult{Status: types.StatusInternalError, Stderr: runErr.Error()}
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

	resp.Status = resolveTopLevel(buildStatusOf(resp.Build), resp.Tests)
	return resp, nil
}

// runBuild renders and executes the build phase. ok=false means the
// caller should propagate not_executed to every test. err != nil is
// reserved for goboxd-side failures; a failing compiler is ok=false
// with StatusBuildFailed (not an error).
func (r *Runner) runBuild(
	ctx context.Context,
	lang *config.LanguageSpec,
	ws jail.Workspace,
	jailSrc, jailArtifact string,
	override *types.Phase,
) (*types.BuildResult, bool, error) {
	limits := mergeLimits(lang.Build.Limits, phaseLimits(override))
	args := expandArgs(lang.Build.Args, jailSrc, jailArtifact, flagsFor(override, lang.Build))
	cmd := expandCmd(lang.Build.Cmd, jailSrc, jailArtifact)

	jctx, cancel := context.WithTimeout(ctx, time.Duration(limits.WallTimeS+2)*time.Second)
	defer cancel()

	out, err := jail.Execute(jctx, r.srv.NSJailBinary, jail.Job{
		Name:      lang.ID + "-build",
		Workspace: ws,
		Cmd:       cmd,
		Args:      args,
		// Build mounts /work RW so the compiler can write the artifact.
		Mounts:       []jail.Mount{{HostPath: ws.SandboxPath, JailPath: "/work", ReadOnly: false}},
		WallTimeS:    limits.WallTimeS,
		MemoryKB:     limits.MemoryKB,
		MaxProcesses: limits.MaxProcesses,
	})
	if err != nil {
		return &types.BuildResult{Status: types.BuildStatusInternalError, Stderr: err.Error()}, false, nil
	}

	br := &types.BuildResult{
		Stdout:     string(out.Stdout),
		Stderr:     string(out.Stderr),
		DurationMS: out.DurationMS,
	}

	if out.TimedOut || out.Signal != 0 || out.ExitCode != 0 {
		br.Status = types.BuildStatusFailed
		return br, false, nil
	}

	br.Status = types.BuildStatusOK
	return br, true, nil
}

// phaseLimits returns the limits override embedded in a request phase,
// or nil if the phase is absent.
func phaseLimits(p *types.Phase) *types.Limits {
	if p == nil {
		return nil
	}
	return p.Limits
}

// mergeLimits overlays the request's limits on top of language defaults.
// Zero values in the override are treated as "not set".
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

// flagsFor returns the request's flags for a phase, or nil if the phase
// is absent. A non-nil phase with empty Flags overrides to "no flags".
// The PhaseSpec arg is reserved for future per-language default flags.
func flagsFor(phase *types.Phase, _ *config.PhaseSpec) []string {
	if phase == nil {
		return nil
	}
	return phase.Flags
}

func buildStatusOf(br *types.BuildResult) string {
	if br == nil {
		return ""
	}
	return br.Status
}
