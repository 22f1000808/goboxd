## Bounded Output capture

**Context:** The buggy code can flood stdout/stderr. I cap output at 64KiB without ever buffering more than this in memory even if process writes more than this.

**Pattern:**
The output error or the program delibritly to create overload ouput can load the output capture.

**Where we used it:**
`capBuffer` is used to wraps `bytes.Buffer` with a byte couter and a hard cap.



## Cleanup with workspace isolation

 **Context:** In runner every runner invocation creates a temporary workspace directory that must be removed at the time of success failure or panic and cleanup failure should be logged but must never mask the original execution error. 
 
 **Pattern:**
 Immediately after the `newWorkspace()` succeeds function calls `osRemovAll(ws.HostRoot)`.

 **Where we used it:**
 `internal/runner/runner.go` inside `Runner.Run()`.

 