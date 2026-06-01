# GoBoxd API

The REST API manages sandboxed execution and system health.

## POST `/run`
Executes source code in a sandboxed `nsjail` environment.
- Validates the payload against system limits and language flag constraints.
- Acquires a concurrency slot or returns `503 Service Unavailable` if the queue is full.
- Resolves execution status per test and aggregates top-level execution results.

## GET `/healthz`
Fast liveness probe.
- Returns `200 OK` `{"status": "ok"}` while the process is alive, including during graceful shutdown.

## GET `/readyz`
Comprehensive readiness probe.
- Executes smoke tests against `nsjail` and all configured language toolchains.
- Caches results for 30 seconds to prevent self-DoS.
- Returns `200 OK` on success, `503 Service Unavailable` if tests fail or if the server is draining.

## GET `/info`
System reporting endpoint.
- Returns `200 OK` with atomic statistics on server limits, active configurations, toolchain readiness, and live queue telemetry (e.g., jobs in-flight, total, failed).
