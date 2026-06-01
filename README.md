# goboxd

A Go HTTP service that runs untrusted code inside an nsjail sandbox and
returns per-test results. Hackathon submission for SEEK x Paradox IIT
Madras 2026.

## Framework

`net/http` with `http.ServeMux` (Go 1.22 method-prefixed routes). No
framework dependency, no router middleware tower, fewer moving parts to
audit when the spec's whole point is sandbox isolation and security.

## Status

Working end to end for Python 3 and C++ inside the container. All seven
documented security holes are closed (see `docs/security.md`). `/healthz`,
`/readyz`, and `/info` are live. Concurrency is bounded with a
configurable queue.

## Run it

```
git submodule update --init
make docker-run
```

Then:

```
curl -s localhost:8080/healthz
curl -s localhost:8080/readyz
curl -s -X POST localhost:8080/run \
  -H 'content-type: application/json' \
  --data @testdata/py-hello.json
```

## Layout

- `cmd/goboxd` - binary entry point
- `internal/` - `types`, `config`, `validator`, `limiter`, `jail`,
  `runner`, `health`, `logging`, `api`
- `configs/` - `server.yaml` and `languages.yaml`
- `external/nsjail` - git submodule pinned to upstream tag `3.4`, built
  inside the image
- `docs/` - `api.md`, `languages.md`, `security.md`, `benchmarks.md`,
  `architecture.md`
- `tests/` - black-box HTTP end-to-end tests (build tag `integration`)
- `testdata/` - sample request bodies
- `scripts/load.sh` - vegeta load probe
- `testdata/check.sh` - full pre-submission verification (phases A-H)

## Develop

```
make build test lint
```

`make test` is unit-only and runs on any OS. `make integration` brings
the container up and runs the `tests/` suite against it. `make check`
runs the full pre-submission verification script.
See `docs/architecture.md` for the design walk-through and `docs/benchmarks.md` for concurrency numbers.
