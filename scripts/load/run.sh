#!/bin/bash
# scripts/load/run.sh
# Load test harness for goboxd.
# Usage: bash scripts/load/run.sh [URL]
# Requires: hey (https://github.com/rakyll/hey) or falls back to curl loop.
# Produces output formatted for docs/benchmarks.md.

set -euo pipefail

URL="${GOBOXD_URL:-http://localhost:8080}"
RUN_ENDPOINT="${URL}/run"
HEALTH_ENDPOINT="${URL}/healthz"

# Payload: py3 Hello World - the required spec benchmark case.
PAYLOAD='{"language":"py3","source":"print(\"Hello, World!\")\n","tests":[{"stdin":"","expected_stdout":"Hello, World!\n"}]}'

# Check server is up
if ! curl -fsS "${HEALTH_ENDPOINT}" >/dev/null 2>&1; then
    echo "ERROR: Server not reachable at ${URL}" >&2
    exit 1
fi

echo "=== goboxd load test ==="
echo "URL: ${URL}"
echo "Payload: py3 Hello World"
echo "Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo ""

# Try hey first; fall back to a simple sequential curl loop.
if command -v hey >/dev/null 2>&1; then
    for clients in 1 10 50 100; do
        echo "--- Concurrency: ${clients} ---"
        hey -n $((clients * 10)) -c "${clients}" \
            -m POST \
            -H "Content-Type: application/json" \
            -d "${PAYLOAD}" \
            "${RUN_ENDPOINT}" 2>&1 || true
        echo ""
    done
elif command -v ab >/dev/null 2>&1; then
    TMPFILE=$(mktemp)
    echo -n "${PAYLOAD}" > "${TMPFILE}"
    for clients in 1 10 50 100; do
        echo "--- Concurrency: ${clients} (ab) ---"
        ab -n $((clients * 10)) -c "${clients}" \
            -T "application/json" \
            -p "${TMPFILE}" \
            "${RUN_ENDPOINT}" 2>&1 | grep -E "^(Requests per second|Time per request|Failed)" || true
        echo ""
    done
    rm -f "${TMPFILE}"
else
    echo "NOTE: Install 'hey' (go install github.com/rakyll/hey@latest) for detailed percentile stats."
    echo "Running sequential baseline (1 client, 10 requests):"
    TOTAL=10
    SUCCESS=0
    START=$(date +%s%N)
    for i in $(seq 1 ${TOTAL}); do
        STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
            -X POST -H "Content-Type: application/json" \
            -d "${PAYLOAD}" "${RUN_ENDPOINT}")
        if [ "${STATUS}" = "200" ]; then
            SUCCESS=$((SUCCESS + 1))
        fi
    done
    END=$(date +%s%N)
    ELAPSED_MS=$(( (END - START) / 1000000 ))
    echo "  ${SUCCESS}/${TOTAL} succeeded in ${ELAPSED_MS}ms"
    echo "  Avg: $((ELAPSED_MS / TOTAL))ms/req"
fi

echo ""
echo "=== Memory usage ==="
if docker ps --format "{{.Names}}" 2>/dev/null | grep -q goboxd; then
    docker stats --no-stream --format "table {{.Name}}\t{{.MemUsage}}" 2>/dev/null | grep goboxd || true
fi

echo ""
echo "=== Goroutine leak check ==="
if curl -fsS "${URL}/info" 2>/dev/null | python3 -c "import json,sys; d=json.load(sys.stdin); print('goroutines:', d['stats']['num_goroutine'])" 2>/dev/null; then
    :
fi

echo ""
echo "Load test complete."
