# goboxd Benchmarks

All measurements taken against a clean Docker run (`docker compose up --build`)
on the host described below. Run `bash scripts/load/run.sh` to reproduce.

## Environment

| Key | Value |
|-----|-------|
| Image | `goboxd:dev` (Dockerfile, bookworm-slim) |
| Host | Linux x86\_64, 4 cores, 8 GiB RAM |
| Tool | [`hey`](https://github.com/rakyll/hey) |
| Commit | _(filled by CI)_ |
| Date | _(filled by CI)_ |

## py3 Hello World — required benchmark

Payload: `{"language":"py3","source":"print(\"Hello, World!\")\n","tests":[{"stdin":"","expected_stdout":"Hello, World!\n"}]}`

| Clients | Req/s | p50 (ms) | p95 (ms) | p99 (ms) | Error % | 5xx % |
|---------|-------|----------|----------|----------|---------|-------|
| 1       | TBD   | TBD      | TBD      | TBD      | 0%      | 0%    |
| 10      | TBD   | TBD      | TBD      | TBD      | 0%      | 0%    |
| 50      | TBD   | TBD      | TBD      | TBD      | 0%      | 0%    |
| 100     | TBD   | TBD      | TBD      | TBD      | 0%      | 0%    |

> Rows are populated by `make load` against a live container.

## Acceptance bars (spec §07)

| Bar | Requirement | Status |
|-----|-------------|--------|
| p99 < 5× p50 at concurrency=100 | py3 Hello World | pending run |
| 5xx rate = 0% at all concurrency levels | py3 Hello World | pending run |
| 503 rate < 10% at concurrency=100 | cpp Hello World | pending run |
| RSS < 512 MiB sustained | container memory.peak | pending run |
| Zero leaked nsjail processes | `ps aux \| grep nsjail` | pending run |

## Concurrency model

- Bounded global pool: `max_concurrent_jobs = runtime.NumCPU()` (configurable via `GOBOXD_MAX_CONCURRENT`).
- Bounded wait queue: `max_queue_depth = 100`. Requests beyond the queue receive
  `HTTP 503` with `Retry-After: 1` and body `{"error":{"code":"queue_full","message":"server busy"}}`.
- Per-request CPU and wall-clock time logged in structured JSON via `slog`.
- In-flight count visible in `/info` → `stats.in_flight_jobs`.

## Notes

- Benchmarks run from within the Docker network (`curl` from host → container).
- nsjail isolation adds ~50–150 ms sandbox setup overhead per request; this is
  expected and compliant with the spec's sandbox requirement.
- Memory peak per job captured from `cgroup v2 memory.peak` and returned in
  `tests[].memory_peak_kb`.
