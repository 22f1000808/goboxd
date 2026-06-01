# GoBoxd Architecture

GoBoxd is a sandboxed code execution engine built for plug-and-play language support, bounded concurrency, and secure isolation against malicious workloads.

## System Shape
The system consists of one HTTP service, eight Go packages, two YAML configuration files, and a four-layer Dockerfile. Packages follow a strict dependency hierarchy: `types` → `config` → `validator` → `limiter` → `jail` → `runner` → `api`, with `health` and `logging` cutting across.

## Core Design Decisions

1. **Plug-and-play Seam**: The language registry (`configs/languages.yaml`) is the single configuration seam. Adding a language requires no Go code changes. The validator, runner, and health probes parameterize entirely over this registry.

2. **Defensive Boundary (Security)**: The validator enforces safety before any execution. Requests are confirmed safe (filenames clean, sizes bounded, flags allow-listed) before reaching the runner.

3. **Concurrency**: Bounded execution via a semaphore (`internal/limiter`), bounded queue depth, and HTTP service timeouts. It queues requests up to a hard cap, returning `503` when overloaded, effectively mitigating burst loads and Slowloris attacks.

4. **Subprocess Management**: Execution uses `nsjail` via `exec.CommandContext` with explicit process group kills (`Setpgid: true`) on context cancellation. GoBoxd pre-creates the `cgroup-v2` directory to deterministically capture `memory.events`, `pids.events`, and `memory.peak`.

5. **Outcome Classification**: A pure, table-tested `Classify` function maps `nsjail`'s combinations of exit codes, signals, and cgroup events to exact specification statuses (`time_exceeded`, `memory_exceeded`, `runtime_error`, `output_whitespace_mismatch`, etc.).

6. **Graceful Shutdown**: SIGTERM initiates a drain phase. The server rejects new requests with `503`, and waits for in-flight jobs up to a `drain_timeout_s` deadline before `SIGKILL`ing any remaining `nsjail` children.

7. **Workspace Lifecycle**: Temp directories are managed on a tmpfs, reducing I/O overhead. A deferred cleanup and periodic sweeper prevent stale jail accumulation.

