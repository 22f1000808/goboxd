# Security Model

The security model ensures GoBoxd remains resilient against malicious payloads. Seven known security holes are closed by design.

### 1. Path Traversal via Filename
**Closed in**: `internal/validator/filename.go` and `internal/runner/runner.go`
The API validator rejects any filename containing slashes, backslashes, leading dots, or path navigation (`..`). The runner independently verifies via `filepath.Clean` that the resolved absolute path resides strictly within the `jailDir`.

### 2. Shell-style Directory Commands
**Closed in**: Entire codebase
`exec.Command("sh")` and similar shell invocations are strictly banned. Workspace creation and removal exclusively use standard `os` filesystem APIs (`os.MkdirTemp`, `os.RemoveAll`).

### 3. Compiler-Flag Injection
**Closed in**: `internal/validator/flags.go` and `internal/config/flags.go`
The validator matches requested build/run flags against a per-language pattern allow-list defined in `configs/languages.yaml`. Specific risk flags (`-Wl,`, `-fplugin`, `--specs=`, `@`, `=/`) are unconditionally blocked.

### 4. Unbounded Request Size
**Closed in**: `internal/api/server.go` and `internal/validator/validator.go`
Size is capped at four levels:
1. HTTP middleware limits the request body to 1 MiB (`http.MaxBytesReader`).
2. The validator explicitly checks `source` and `stdin` against configured `max_source_bytes` and `max_stdin_bytes`.
3. The validator limits the maximum number of test cases.
4. `nsjail` limits `rlimit_fsize` and `cgroup_mem_max`.

### 5. UID Collisions Under Load
**Closed in**: `internal/jail/template.go` and `internal/jail/execute.go`
We rely on namespace isolation (`clone_newns`, `clone_newpid`, `clone_newipc`) rather than unique UIDs. All payloads run as `UID 65534`, but each jail operates in a separate mount and PID namespace. The workspace is owned by `65534` with `0700` permissions.

### 6. Unbounded Child Output
**Closed in**: `internal/jail/output.go`
Subprocess stdout and stderr streams are piped through a custom `boundedWriter` capping the output at 64 KiB. Exceeding the limit halts accumulation and appends `[...output truncated...]`, preventing OOM crashes.

### 7. Stale Jail Directories
**Closed in**: `internal/runner/run.go` and `internal/runner/orphans.go`
Workspaces are cleaned up via `defer os.RemoveAll` immediately upon job completion or panic. A background sweeper also runs synchronously at boot and every 5 minutes to purge any directories older than 10 minutes.
