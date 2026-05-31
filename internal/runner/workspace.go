package runner

import (
	"os"
	"path/filepath"

	"goboxd/internal/jail"
)

type workspaceOpts struct {
	JailRoot     string
	CgroupParent string
}

func newWorkspace(opts workspaceOpts) (jail.Workspace, func(), error) {
	if err := os.MkdirAll(opts.JailRoot, 0700); err != nil {
		return jail.Workspace{}, func() {}, err
	}

	dir, err := os.MkdirTemp(opts.JailRoot, "job-*")
	if err != nil {
		return jail.Workspace{}, func() {}, err
	}

	work := filepath.Join(dir, "work")
	if err := os.MkdirAll(work, 0755); err != nil {
		_ = os.RemoveAll(dir)
		return jail.Workspace{}, func() {}, err
	}

	var cgPath string
	if opts.CgroupParent != "" {
		candidate := filepath.Join(opts.CgroupParent, filepath.Base(dir))
		if err := os.Mkdir(candidate, 0o755); err == nil {
			cgPath = candidate
		}
	}

	cleanup := func() {
		if cgPath != "" {
			_ = jail.RemoveCgroup(cgPath)
		}
		_ = os.RemoveAll(dir)
	}
	return jail.Workspace{HostRoot: dir, SandboxPath: work, CgroupPath: cgPath}, cleanup, nil
}
