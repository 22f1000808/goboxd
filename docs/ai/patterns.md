## Deferred cleanup with error capture

**Context:** You need cleanup (deleting a temp directory) to run on every exit path from a function, including early returns on error.
But you also need to capture the cleanup error without shadowing the original error.

**Pattern:**
Use a named return value and a deferred function that checks whether the original error is nil before overwriting it with the cleanup error.

**Where we used it:**
boxd/runner.go, the runSandbox function. The jail directory cleanup is deferred immediately after creation. If the run succeeds but cleanup fails, the cleanup error is returned. If the run fails, the run error takes precedence.