#!/usr/bin/env bash
# docs/loadtest/load-test.sh
# ─────────────────────────────────────────────────────────────────────
# Reproducible load test for goboxd Final Stage.
# Drives MemoryHog.java at rising request rates via vegeta, produces
# results.csv + both PNG graphs.
#
# Requirements:
#   - vegeta   (go install github.com/tsenart/vegeta/v12@latest)
#   - jq       (apt install jq)
#   - python3  (with matplotlib: pip3 install matplotlib)
#   - A running goboxd container (see README.md for startup)
#
# Usage:
#   bash docs/loadtest/load-test.sh [BASE_URL]
# ─────────────────────────────────────────────────────────────────────
set -euo pipefail

# Ensure go-installed binaries (vegeta) are on PATH
export PATH="${PATH}:$(go env GOPATH 2>/dev/null)/bin"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BASE_URL="${1:-http://localhost:8080}"
RUN_URL="${BASE_URL}/run"
DURATION="30s"
TIMEOUT="10s"
REPORT_DIR="${SCRIPT_DIR}/reports"

# Rate ladder as specified
RATES=(5 10 25 50 75 100 150 200 300 400)

# ── Preflight checks ────────────────────────────────────────────────
for cmd in vegeta jq python3 curl; do
    if ! command -v "$cmd" &>/dev/null; then
        echo "ERROR: '$cmd' is required but not found." >&2
        exit 1
    fi
done

echo "=== GoBoxd Final Stage Load Test ==="
echo "URL:       ${BASE_URL}"
echo "Duration:  ${DURATION} per step"
echo "Timeout:   ${TIMEOUT}"
echo "Rates:     ${RATES[*]}"
echo "Date:      $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo ""

# ── Wait for server ─────────────────────────────────────────────────
echo -n "Waiting for server health..."
for i in $(seq 1 30); do
    if curl -fsS "${BASE_URL}/healthz" >/dev/null 2>&1; then
        echo " ready."
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo " FAILED: server not reachable at ${BASE_URL}" >&2
        exit 1
    fi
    sleep 2
done

# ── Prepare request payload ─────────────────────────────────────────
PAYLOAD_FILE="${SCRIPT_DIR}/run-request.json"
if [ ! -f "$PAYLOAD_FILE" ]; then
    echo "ERROR: ${PAYLOAD_FILE} not found." >&2
    exit 1
fi

# vegeta target file
TARGET_FILE=$(mktemp)
trap 'rm -f "${TARGET_FILE}"' EXIT

BODY=$(cat "$PAYLOAD_FILE")
cat > "${TARGET_FILE}" <<EOF
POST ${RUN_URL}
Content-Type: application/json
@${PAYLOAD_FILE}
EOF

# ── Create report directory ─────────────────────────────────────────
rm -rf "${REPORT_DIR}"
mkdir -p "${REPORT_DIR}"

# ── Run load steps ──────────────────────────────────────────────────
echo ""
echo "Starting load test ladder..."
echo ""

FAIL_COUNT=0

for rate in "${RATES[@]}"; do
    echo -n "  Rate ${rate} rps ... "
    
    vegeta attack \
        -rate="${rate}/1s" \
        -duration="${DURATION}" \
        -timeout="${TIMEOUT}" \
        -targets="${TARGET_FILE}" \
        2>/dev/null \
    | vegeta report -type=json \
        > "${REPORT_DIR}/report-${rate}.json" 2>/dev/null
    
    # Quick summary
    ERROR_PCT=$(jq -r '((1 - .success) * 100) | . * 100 | round / 100' "${REPORT_DIR}/report-${rate}.json")
    THROUGHPUT=$(jq -r '.throughput | . * 100 | round / 100' "${REPORT_DIR}/report-${rate}.json")
    P99=$(jq -r '(.latencies["99th"] / 1e6) | . * 10 | round / 10' "${REPORT_DIR}/report-${rate}.json")
    
    echo "throughput=${THROUGHPUT} rps, error=${ERROR_PCT}%, p99=${P99}ms"
    
    # Early-stop hint (but continue 2-3 more past first failure)
done

echo ""
echo "All steps complete."
echo ""

# ── Build CSV ────────────────────────────────────────────────────────
CSV_FILE="${SCRIPT_DIR}/results.csv"
echo "target_rps,throughput_rps,duration_s,requests,success,failed,error_pct,p50_ms,p95_ms,p99_ms,max_ms" > "${CSV_FILE}"

for rate in "${RATES[@]}"; do
    REPORT="${REPORT_DIR}/report-${rate}.json"
    if [ ! -f "$REPORT" ]; then continue; fi
    if [ ! -f "$REPORT" ]; then
        continue
    fi
    jq -r --arg r "$rate" '
        [.tests[] | select(.status != "accepted")] as $failed |
        [
            $r,
            (.throughput | . * 100 | round / 100),
            (.duration / 1e9 | . * 100 | round / 100),
            .requests,
            (.requests - ($failed | length)),
            ($failed | length),
            (($failed | length) / .requests * 100 | . * 100 | round / 100),
            (.latencies["50th"] / 1e6 | . * 10 | round / 10),
            (.latencies["95th"] / 1e6 | . * 10 | round / 10),
            (.latencies["99th"] / 1e6 | . * 10 | round / 10),
            (.latencies.max / 1e6 | . * 10 | round / 10)
        ] | @csv' "$REPORT" >> "${CSV_FILE}"
done

echo "CSV written to: ${CSV_FILE}"
echo ""

# ── Plot graphs ──────────────────────────────────────────────────────
echo "Generating graphs..."
python3 "${SCRIPT_DIR}/plot.py" "${CSV_FILE}" "${SCRIPT_DIR}"
echo ""

echo "=== Load test complete ==="
echo "Deliverables:"
echo "  ${CSV_FILE}"
echo "  ${SCRIPT_DIR}/breaking-point.png"
echo "  ${SCRIPT_DIR}/latency.png"
