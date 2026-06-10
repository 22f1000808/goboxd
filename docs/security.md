# Security Model

GoBoxd closes seven security holes identified in the spec §06. Each entry
lists the exact location in the source where it is enforced.

## Holes closed

| # | Hole | Closed in | Key line |
|---|------|-----------|----------|
| 1 | Path traversal via filename | `internal/validator/validator.go` | L115–L120 |
| 2 | Shell-style directory commands | `internal/runner/workspace.go` | L24–L35 |
| 3 | Compiler-flag injection | `internal/validator/validator.go` | L28–L36, L136–L149 |
| 4 | Request size limits (dual-layer) | `internal/api/server.go:209`, `internal/validator/validator.go:63–76` | L209, L63 |
| 5 | UID collisions / workspace reuse | `internal/runner/workspace.go` | L23–L24 |
| 6 | Unbounded child output | `internal/jail/execute_linux.go` | L17, L107–L130 |
| 7 | Stale jail directories | `internal/runner/workspace.go:35`, `internal/runner/sweeper.go:74` | L35, L74 |

---

## Hole 1 — Path traversal via filename

**File**: `internal/validator/validator.go:115`

The validator rejects any `source_filename` that contains `/` or `\`,
starts with `.`, or equals `.`/`..`. This prevents directory traversal
before the filename reaches the filesystem.

```go
// validator.go:115
if strings.ContainsAny(name, "/\\") {
    return newErr(CodeInvalidFilename, "filename must not contain path separators")
}
```

---

## Hole 2 — Shell-style directory commands

**File**: `internal/runner/workspace.go:24`

All workspace creation and teardown use Go's `os` package APIs exclusively.
No `exec.Command("sh", "-c", "rm -rf ...")` or equivalent. The entire
codebase contains no shell invocations for filesystem operations.

```go
// workspace.go:24
dir, err := os.MkdirTemp(jailRoot, prefix)
// workspace.go:35
cleanup := func() { _ = os.RemoveAll(dir) }
```

---

## Hole 3 — Compiler-flag injection

**File**: `internal/validator/validator.go:28`

An unconditional denylist blocks the exact tokens named in the spec,
regardless of the per-language allow-list:

```go
// validator.go:28-36
var denylistPrefixes = []string{
    "-fplugin",
    "-x",
    "-B",
    "--specs",
    "-Wl,",
    "@", // response files
}
```

The denylist runs before the allowlist (`validator.go:159`). Any matched
flag returns `HTTP 400` with code `flag_not_allowed`.

---

## Hole 4 — Request size limits (dual-layer)

**Layer 1 — HTTP**: `internal/api/server.go:209`

```go
r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
```

**Layer 2 — Validator**: `internal/validator/validator.go:63–76`

- `source` ≤ `max_source_bytes` (256 KiB default) → `source_too_large`
- Per-test `stdin` ≤ `max_stdin_bytes` (64 KiB) → `stdin_too_large`
- Per-test `expected_stdout` ≤ `max_expected_stdout_bytes` (1 MiB) → `expected_stdout_too_large`
- Test count ≤ `max_tests` (50) → `too_many_tests`

**Layer 3 — Sandbox**: `internal/jail/template.go:37`

```
rlimit_fsize: {{ .FsizeMB }}   # default 16 MiB
```

The two body errors (`MaxBytesReader` 413 vs `source_too_large` 400) are
independently reachable: send a 2 MiB body to hit the HTTP cap; send a
257 KiB body with valid JSON to hit the source cap.

---

## Hole 5 — UID collisions / workspace reuse

**File**: `internal/runner/workspace.go:23`

```go
prefix := fmt.Sprintf("job-%d-", os.Getpid())
dir, err := os.MkdirTemp(jailRoot, prefix)
```

`os.MkdirTemp` appends a cryptographically random suffix. The PID prefix
guarantees inter-process uniqueness. Directories are never reused.

---

## Hole 6 — Unbounded child output

**File**: `internal/jail/execute_linux.go:17`

```go
const outputCap = 64 * 1024  // 64 KiB hard cap per stream
```

Both stdout and stderr are piped through `capBuffer` (L107). On overflow
the buffer stops accepting data and appends `\n[...output truncated...]`.
`Write` always returns `len(p)` to prevent `os/exec`'s `io.Copy` from
returning `ErrShortWrite` (which would cause a spurious 500).

---

## Hole 7 — Stale jail directories

**File**: `internal/runner/workspace.go:35`, `internal/runner/sweeper.go:74`

Every workspace is cleaned up via `defer cleanup()` (`workspace.go:35`)
on all code paths including panics. A background sweeper (`sweeper.go`)
runs at boot and every 5 minutes, removing directories older than 10
minutes that were orphaned by a crash.
