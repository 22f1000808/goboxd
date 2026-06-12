# GoBoxd Load Test — Final Stage

## Container Limits

| Resource | Limit |
|----------|-------|
| vCPU     | 2     |
| RAM      | 2 GB  |

Enforced via `docker-compose.loadtest.yml`:
```bash
docker compose -f docker-compose.yml -f docker-compose.loadtest.yml up -d --build
```

## Tool

**vegeta** (https://github.com/tsenart/vegeta) — HTTP load testing tool written in Go.

## Workload

`MemoryHog.java` — allocates 150 MB of heap, touches every page, does a light CPU pass, then sleeps 1 second before printing a checksum. Each concurrent run holds ~150 MB RSS for the duration.

## Rate Ladder

Each rate held for **30 seconds** with a **10-second** per-request timeout:

```
5, 10, 25, 50, 75, 100, 150, 200, 300, 400 rps
```

## Breaking Point

**RPS: 5**

## What Failed First

**Concurrency limiter queue timeout.** 

Because the GoBoxd instance was constrained to **2 vCPUs**, and the JVM (along with `javac` compilation) is a heavily CPU-bound process that takes ~2.4 CPU seconds per request, the maximum theoretical throughput of the server was strictly less than 1 request per second. 

When load was offered at 5 RPS, the server's `max_concurrent_jobs: 2` limit successfully protected the container from CPU starvation and OOM kills. The excess requests were queued in the bounding queue (`max_queue_depth: 100`). However, because the queue drained at <1 RPS, requests arriving after the first few seconds spent more than 10 seconds waiting in the queue. 

Since the load test client enforces a 10-second timeout, these queued requests were aborted by the client (resulting in `context deadline exceeded` and 10s latency). The service degraded gracefully: it did not crash, it did not run out of memory, and in-flight jobs completed cleanly. It simply shed load via queue timeouts when offered load exceeded its physical CPU capacity.

## Graceful Degradation

GoBoxd uses a bounded concurrency limiter + queue. When the queue fills:
1. New requests receive `503 Service Unavailable` with `Retry-After: 1`
2. In-flight jobs continue to completion
3. Once load drops, the service recovers without restart

The service does **not** crash under overload — it degrades gracefully by shedding excess load.

## How to Reproduce

### Prerequisites
```bash
# Install vegeta
go install github.com/tsenart/vegeta/v12@latest

# Install jq
sudo apt install -y jq

# Install matplotlib
pip3 install matplotlib
```

### Run
```bash
# 1. Start goboxd with resource caps
docker compose -f docker-compose.yml -f docker-compose.loadtest.yml up -d --build

# 2. Wait for readiness
curl -fsS http://localhost:8080/readyz

# 3. Run load test
bash docs/loadtest/load-test.sh http://localhost:8080

# 4. Results are in docs/loadtest/
ls docs/loadtest/results.csv docs/loadtest/breaking-point.png docs/loadtest/latency.png
```

## Results

See:
- [`results.csv`](results.csv) — raw data, one row per load step
- [`breaking-point.png`](breaking-point.png) — error rate vs offered RPS
- [`latency.png`](latency.png) — p50/p95/p99 latency vs offered RPS
