package runner

import (
	"os"
	"path/filepath"

	"goboxd/internal/jail"
)

func newWorkspace(root string) (jail.Workspace, func(), error) {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return jail.Workspace{}, func() {}, err
	}
	dir, err := os.MkdirTemp(root, "job-")
	if err != nil {
		return jail.Workspace{}, func() {}, err
	}

	cleanup := func() {
		_ = os.RemoveAll(dir)
	}
	work := filepath.Join(dir, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		cleanup()
		return jail.Workspace{}, func() {}, err
	}

	cg := filepath.Join(dir, "cgroup")
	if err := os.MkdirAll(cg, 0o755); err != nil {
		cg = ""
	}

	return jail.Workspace{HostRoot: dir, SandboxPath: work, CgroupPath: cg}, cleanup, nil
}
