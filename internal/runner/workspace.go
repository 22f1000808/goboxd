// internal/runner/workspace.go
package runner

import (
	"os"
	"path/filepath"

	"goboxd/internal/jail"
)

func newWorkspace(jailRoot string) (jail.Workspace, func(), error) {
	if err := os.MkdirAll(jailRoot, 0700); err != nil {
		return jail.Workspace{}, func() {}, err
	}

	dir, err := os.MkdirTemp(jailRoot, "job-*")
	if err != nil {
		return jail.Workspace{}, func() {}, err
	}

	work := filepath.Join(dir, "work")
	if err := os.MkdirAll(work, 0755); err != nil {
		_ = os.RemoveAll(dir)
		return jail.Workspace{}, func() {}, err
	}

	cleanup := func() { _ = os.RemoveAll(dir) }
	return jail.Workspace{HostRoot: dir, SandboxPath: work}, cleanup, nil
}
