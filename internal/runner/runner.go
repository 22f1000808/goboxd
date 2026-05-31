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

func (r *Runner) Run(ctx context.Context, req *types.RunRequest) (*types.RunResponse, error) {
	lang, _ := r.registry.Lookup(req.Language)

	ws, cleanup, err := newWorkspace(workspaceOpts{
		JailRoot:     r.srv.JailRootDir,
		CgroupParent: r.srv.CgroupParent,
	})
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
	jailArtifact := lang.Artifact

	resp := &types.RunResponse{Tests: make([]types.TestResult, len(req.Tests))}

	if lang.Build != nil {
		br, ok, berr := r.runBuild(ctx, lang, ws, jailSrc, jailArtifact, req.Build)
		if berr != nil {
			return nil, berr
		}
		resp.Build = br
		if !ok {
			for i := range req.Tests {
				resp.Tests[i] = types.TestResult{Status: types.StatusNotExecuted}
			}
			resp.Status = resolveTopLevel(br.Status, resp.Tests)
			return resp, nil
		}
	}

	runLim := mergeLimits(lang.Run.Limits, requestLimits(req.Run))
	runArgs := expandArgs(lang.Run.Args, jailSrc, jailArtifact, flagsFor(req.Run))
	runCmd := expandCmd(lang.Run.Cmd, jailSrc, jailArtifact)

	for i, t := range req.Tests {
		jctx, cancel := context.WithTimeout(ctx, time.Duration(runLim.WallTimeS+2)*time.Second)

		out, runErr := r.execute(jctx, ws, jail.Job{
			Name:         fmt.Sprintf("%s-test-%d", lang.ID, i),
			Workspace:    ws,
			Cmd:          runCmd,
			Args:         runArgs,
			Stdin:        t.Stdin,
			Mounts: []jail.Mount{
				{HostPath: ws.SandboxPath, JailPath: "/work", ReadOnly: true},
				{HostPath: "/usr", JailPath: "/usr", ReadOnly: true},
				{HostPath: "/bin", JailPath: "/bin", ReadOnly: true},
				{HostPath: "/lib", JailPath: "/lib", ReadOnly: true},
				{HostPath: "/lib64", JailPath: "/lib64", ReadOnly: true},
			},
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

	resp.Status = resolveTopLevel(buildStatus(resp.Build), resp.Tests)
	return resp, nil
}

func (r *Runner) runBuild(
	ctx context.Context,
	lang *config.LanguageSpec,
	ws jail.Workspace,
	jailSrc, jailArtifact string,
	override *types.Phase,
) (*types.BuildResult, bool, error) {
	limits := mergeLimits(lang.Build.Limits, requestLimits(override))
	args := expandArgs(lang.Build.Args, jailSrc, jailArtifact, flagsFor(override))
	cmd := expandCmd(lang.Build.Cmd, jailSrc, jailArtifact)

	jctx, cancel := context.WithTimeout(ctx, time.Duration(limits.WallTimeS+2)*time.Second)
	defer cancel()

	out, err := r.execute(jctx, ws, jail.Job{
		Name:         lang.ID + "-build",
		Workspace:    ws,
		Cmd:          cmd,
		Args:         args,
		Mounts: []jail.Mount{
			{HostPath: ws.SandboxPath, JailPath: "/work", ReadOnly: false},
			{HostPath: "/usr", JailPath: "/usr", ReadOnly: true},
			{HostPath: "/bin", JailPath: "/bin", ReadOnly: true},
			{HostPath: "/lib", JailPath: "/lib", ReadOnly: true},
			{HostPath: "/lib64", JailPath: "/lib64", ReadOnly: true},
		},
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

func (r *Runner) execute(ctx context.Context, ws jail.Workspace, job jail.Job) (jail.Outcome, error) {
	if ws.CgroupPath != "" {
		if err := jail.CreateCgroup(ws.CgroupPath, job.MemoryKB, job.MaxProcesses); err != nil {
			return jail.Outcome{}, err
		}
	}
	return jail.Execute(ctx, r.srv.NSJailBinary, job)
}

func requestLimits(phase *types.Phase) *types.Limits {
	if phase == nil {
		return nil
	}
	return phase.Limits
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

func flagsFor(phase *types.Phase) []string {
	if phase == nil {
		return nil
	}
	return phase.Flags
}

func buildStatus(br *types.BuildResult) string {
	if br == nil {
		return ""
	}
	return br.Status
}
