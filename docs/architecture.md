# Architecture

## Request lifecycle

```
Client
  │
  │  POST /run  (JSON, ≤1 MiB body)
  ▼
HTTP layer  (net/http ServeMux, MaxBytesReader)
  │
  ├── requestIDMiddleware  → attach opaque request-id to context
  ├── recoverMiddleware    → catch panics, return 500
  │
  ▼
api.handleRun
  │
  ├── 1. Draining check          → 503 Retry-After if shutting down
  ├── 2. JSON decode              → 400 bad_json on parse failure
  ├── 3. validator.Validate       → 400 + error code on constraint violation
  │       • flag denylist (unconditional, runs first)
  │       • per-language flag allow-list
  │       • source/stdin/expected_stdout size caps
  │       • filename strategy enforcement
  ├── 4. limiter.Acquire         → blocks in queue if pool full
  │                                 503 queue_full if queue also full
  ├── 5. jobs.Track              → register job for graceful-drain wait
  │
  ▼
runner.Run
  │
  ├── newWorkspace(jailRoot)     → os.MkdirTemp with PID prefix
  ├── os.WriteFile(source)
  │
  ├── [if build phase]
  │     jail.Execute(build job)  → nsjail --config <rendered protobuf>
  │     classify build outcome   → BuildResult{status, stdout, stderr, duration_ms}
  │     on build failure: all tests → not_executed, return
  │
  ├── [for each test]
  │     jail.Execute(run job)    → nsjail with stdin piped
  │     jail.Classify(outcome)   → status string
  │     collect TestResult{status, stdout, stderr, duration_ms, memory_peak_kb}
  │
  ├── resolveTopLevel            → first non-accepted test status
  ├── defer cleanup()            → os.RemoveAll(workspace)
  │
  ▼
JSON response (HTTP 200 regardless of code outcome)
```

---

## Registry seam (plug-and-play)

Language configuration lives entirely in `configs/languages.yaml`. The
Go code does not contain language-specific logic.

```
configs/languages.yaml
    │ LoadLanguages() → Registry
    │
    ├── validates each LanguageSpec at parse time
    ├── compiles FlagRule objects from flag_allowlist
    └── sorts IDs for deterministic /info + /readyz output

runner.Run  → registry.Lookup(req.Language)
health.Probe → iterates registry.All() for smoke probes
```

Adding a language requires:
1. One YAML block in `configs/languages.yaml`
2. One install script in `scripts/lang_install/<id>.sh`
3. Zero Go changes (unless custom logic is needed)

Boot validation runs every smoke probe on startup and aborts loudly if
any toolchain is missing.

---

## Jail / cgroup model

Each sandboxed execution writes a nsjail protobuf config to a temp file
and invokes:

```
nsjail --config <tmpdir>/nsjail.cfg --really_quiet
```

The config sets:
- `clone_newnet: true` — network namespace (no outbound access by default)
- `clone_newuser/ns/pid/ipc/uts/cgroup` — full namespace isolation
- `time_limit` — wall-clock deadline enforced by nsjail
- `rlimit_as` — 4× memory_kb (virtual address space)
- `rlimit_fsize` — 16 MiB (file write cap)
- `rlimit_nofile: 64`, `rlimit_nproc: 0`, `rlimit_stack: 0`
- `cgroupv2_mount` — per-job cgroup for memory.peak + OOM detection

Mounts: `/usr`, `/lib`, `/lib64`, `/bin`, `/sbin`, `/etc` read-only;
`/work` as the job workspace (read-only for run phase, read-write for
build phase); `/tmp`, `/dev`, `/proc` as tmpfs/proc.

---

## Concurrency / queue design

```
Limiter (internal/limiter)
  ├── semaphore channel (size = max_concurrent_jobs, default = NumCPU())
  ├── wait queue channel (size = max_queue_depth, default = 100)
  │
  ├── Acquire(): try semaphore → if full, try queue → if full, ErrQueueFull
  └── Release(): return slot to semaphore, unblock waiter if any
```

HTTP calls are synchronous — the client connection holds until the
sandbox completes. `503 + Retry-After` is returned only when the wait
queue is also full.

The bounded pool + FIFO queue provides admission backpressure without the
complexity of priority scheduling. Per-request CPU + wall time are logged
via slog JSON on every response.

---

## Security boundaries

See [docs/security.md](security.md) for the full seven-hole breakdown.

Summary:
1. Filename validation before any filesystem write
2. All fs ops use `os.*` APIs, no shell
3. Flag denylist + per-language allowlist (denylist runs first)
4. Size caps at HTTP layer + validator + rlimit_fsize in sandbox
5. `os.MkdirTemp` with PID prefix — no workspace reuse
6. `capBuffer` 64 KiB cap on stdout + stderr per stream
7. `defer os.RemoveAll` + background sweeper for orphans

---

## Failure modes

| Failure | Response |
|---------|----------|
| nsjail not found at boot | `os.Exit(1)` with clear error log |
| Language toolchain missing at boot | `os.Exit(1)` with language id in error |
| nsjail launch fails (sandbox error) | `internal_error` in build/test result, `HTTP 200` |
| OOM in sandbox | `memory_exceeded` status (cgroup `memory.events:oom_kill`) |
| Wall-clock timeout | `time_exceeded` status (nsjail enforced) |
| Request body > 1 MiB | `HTTP 413` |
| Source > 256 KiB | `HTTP 400` `source_too_large` |
| Queue full (> 100 waiting) | `HTTP 503` `queue_full` + `Retry-After: 1` |
| Panic in handler | `HTTP 500` via recoverMiddleware, error logged |
| SIGTERM received | Draining mode: new requests get `503`, existing drain gracefully |
