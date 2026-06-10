// internal/runner/workspace.go
// Creates an isolated workspace directory for a single sandbox execution.
// Uses os.MkdirTemp which generates a cryptographically random suffix,
// combined with the process PID, ensuring uniqueness even across concurrent
// processes (closes §06 hole 5: UID collision).
package runner

import (
	"fmt"
	"os"
	"path/filepath"

	"goboxd/internal/jail"
)

func newWorkspace(jailRoot string) (jail.Workspace, func(), error) {
	if err := os.MkdirAll(jailRoot, 0700); err != nil {
		return jail.Workspace{}, func() {}, err
	}

	// Prefix includes the PID to prevent any cross-process collision; the
	// random suffix from MkdirTemp ensures intra-process uniqueness.
	prefix := fmt.Sprintf("job-%d-", os.Getpid())
	dir, err := os.MkdirTemp(jailRoot, prefix)
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
