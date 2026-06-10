## HUGO

**Team**: Shivam Mishra

## Framework

Go's `net/http` with `http.ServeMux` (Go 1.22 method-prefixed routes) — no
framework dependencies, fewer moving parts to audit when the whole point is
sandbox isolation.

## How to run locally

```sh
git clone <repo> && cd goboxd
make docker-run          # build image + start container
curl localhost:8080/healthz   # should return {"status":"ok"}
curl localhost:8080/readyz    # per-language status
curl localhost:8080/info      # build info + stats
make test                # unit tests with race detector
make check               # full pre-submission check suite
make load                # load test (needs server running)
```

All make targets work from a fresh clone in under 10 minutes including the
Docker build. No bare `go run` required.

## Security holes closed

7 holes closed per spec §06. All file:line references resolve to real lines.
See [docs/security.md](docs/security.md) for the full breakdown.

| # | Hole | File:line |
|---|------|-----------|
| 1 | Path traversal via filename | [internal/validator/validator.go:115](internal/validator/validator.go#L115) |
| 2 | Shell-style directory commands | [internal/runner/workspace.go:24](internal/runner/workspace.go#L24) |
| 3 | Compiler-flag injection (denylist: `-fplugin`, `-x`, `-B`, `--specs`, `-Wl,`, `@`, `-I/`) | [internal/validator/validator.go:28](internal/validator/validator.go#L28) |
| 4 | Dual-layer size caps (HTTP 1 MiB + validator source/stdin/expected_stdout + rlimit_fsize) | [internal/api/server.go:209](internal/api/server.go#L209), [internal/validator/validator.go:63](internal/validator/validator.go#L63) |
| 5 | Workspace UID uniqueness (PID + random suffix via os.MkdirTemp) | [internal/runner/workspace.go:23](internal/runner/workspace.go#L23) |
| 6 | Unbounded child output (capBuffer 64 KiB, returns len(p) on truncation) | [internal/jail/execute_linux.go:17](internal/jail/execute_linux.go#L17) |
| 7 | Stale jail directories (defer cleanup + background sweeper) | [internal/runner/workspace.go:35](internal/runner/workspace.go#L35), [internal/runner/sweeper.go:74](internal/runner/sweeper.go#L74) |

## Languages supported

**7 in-scope** (spec §02): `bash`, `c`, `cpp`, `java`, `javascript`, `py3`, `verilog`

**4 beyond-seven** (bonus): `go`, `lua`, `ruby`, `rust`

All languages configured in `configs/languages.yaml`. Adding a new language
requires one YAML block + one install script — no Go code changes. Boot
validation aborts with a clear error if any toolchain is missing.

See [docs/languages.md](docs/languages.md) for the timed demo-add drill.

## Benchmarks

See [docs/benchmarks.md](docs/benchmarks.md) for the full results table and
acceptance bars (p99 < 5× p50, 0% 5xx, RSS < 512 MiB).

Load test: `make load` (runs `scripts/load/run.sh` against the live container).
