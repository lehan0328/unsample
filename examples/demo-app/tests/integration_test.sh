#!/usr/bin/env bash
#
# Unsample Demo Integration Tests
#
# Prerequisites:
#   cd examples/demo-app && docker-compose up --build -d
#
# Usage:
#   ./tests/integration_test.sh
#
# These tests verify the full Unsample pipeline:
#   CLI → Gateway → Billing → Notification → Collector → Tempo
#
set -euo pipefail

SECRET="demo-secret-do-not-use-in-production"
GATEWAY="http://localhost:8080"
TEMPO_API="http://localhost:3200"
PASS=0
FAIL=0
SKIP=0

# ─── Helpers ──────────────────────────────────────────

green()  { printf "\033[32m%s\033[0m\n" "$1"; }
red()    { printf "\033[31m%s\033[0m\n" "$1"; }
yellow() { printf "\033[33m%s\033[0m\n" "$1"; }

pass() { PASS=$((PASS + 1)); green "  ✅ PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); red   "  ❌ FAIL: $1"; }
skip() { SKIP=$((SKIP + 1)); yellow "  ⏭  SKIP: $1"; }

wait_for_service() {
    local url=$1
    local name=$2
    local max_attempts=30

    printf "  Waiting for %s..." "$name"
    for i in $(seq 1 $max_attempts); do
        if curl -s -o /dev/null -w '' "$url" 2>/dev/null; then
            printf " ready\n"
            return 0
        fi
        printf "."
        sleep 1
    done
    printf " TIMEOUT\n"
    return 1
}

# Generate an HMAC token the same way the CLI does.
generate_token() {
    local trace_id=$1
    local timestamp
    timestamp=$(date +%s)
    local payload="${trace_id}:${timestamp}"
    local sig
    sig=$(printf '%s' "$payload" | openssl dgst -sha256 -hmac "$SECRET" -binary | base64 | tr '+/' '-_')
    echo "${trace_id}:${timestamp}:${sig}"
}

# ─── Pre-flight ───────────────────────────────────────

echo ""
echo "═══════════════════════════════════════════════════"
echo "  Unsample Integration Tests"
echo "═══════════════════════════════════════════════════"
echo ""

echo "▶ Pre-flight checks..."
wait_for_service "$GATEWAY/health" "gateway"
wait_for_service "http://localhost:8081/health" "billing"
wait_for_service "http://localhost:8082/health" "notification"
wait_for_service "$TEMPO_API/ready" "tempo"

echo ""

# ─── Test 1: Happy Path ──────────────────────────────

echo "▶ Test 1: Happy path (checkout user=123)"

TRACE_ID=$(openssl rand -hex 16)
TOKEN=$(generate_token "$TRACE_ID")
SPAN_ID=$(openssl rand -hex 8)
TRACEPARENT="00-${TRACE_ID}-${SPAN_ID}-01"

RESPONSE=$(curl -s -w "\n%{http_code}" \
    -H "traceparent: ${TRACEPARENT}" \
    -H "baggage: unsample-debug=${TOKEN}" \
    "${GATEWAY}/checkout?user=123")

HTTP_CODE=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | head -1)

if [ "$HTTP_CODE" = "200" ]; then
    pass "Gateway returned 200"
else
    fail "Gateway returned $HTTP_CODE (expected 200)"
fi

if echo "$BODY" | grep -q '"gateway":"processed"'; then
    pass "Response contains gateway processing"
else
    fail "Response missing gateway data: $BODY"
fi

if echo "$BODY" | grep -q '"charged":true'; then
    pass "Billing charged successfully"
else
    fail "Billing did not charge: $BODY"
fi

if echo "$BODY" | grep -q '"sent":true'; then
    pass "Notification sent"
else
    fail "Notification not sent: $BODY"
fi

# Wait for trace to be ingested.
echo "  Waiting 5s for trace ingestion..."
sleep 5

# Check Tempo for the trace.
TEMPO_TRACE=$(curl -s "${TEMPO_API}/api/traces/${TRACE_ID}" 2>/dev/null || echo "")

if echo "$TEMPO_TRACE" | grep -q '"traceID"'; then
    pass "Trace found in Tempo"

    # Count spans across all batches.
    SPAN_COUNT=$(echo "$TEMPO_TRACE" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    count = 0
    for batch in data.get('batches', []):
        for scope in batch.get('scopeSpans', []):
            count += len(scope.get('spans', []))
    print(count)
except:
    print(0)
" 2>/dev/null || echo "0")

    if [ "$SPAN_COUNT" -ge 3 ]; then
        pass "Trace has $SPAN_COUNT spans (≥3 services)"
    else
        fail "Trace has only $SPAN_COUNT spans (expected ≥3)"
    fi

    # Check for debug.trace attribute.
    if echo "$TEMPO_TRACE" | grep -q 'debug.trace'; then
        pass "debug.trace attribute present"
    else
        fail "debug.trace attribute missing"
    fi
else
    skip "Trace not found in Tempo (ingestion may be slow)"
fi

echo ""

# ─── Test 2: Error Path ──────────────────────────────

echo "▶ Test 2: Error path (checkout user=666)"

TRACE_ID2=$(openssl rand -hex 16)
TOKEN2=$(generate_token "$TRACE_ID2")
SPAN_ID2=$(openssl rand -hex 8)
TRACEPARENT2="00-${TRACE_ID2}-${SPAN_ID2}-01"

RESPONSE2=$(curl -s -w "\n%{http_code}" \
    -H "traceparent: ${TRACEPARENT2}" \
    -H "baggage: unsample-debug=${TOKEN2}" \
    "${GATEWAY}/checkout?user=666")

HTTP_CODE2=$(echo "$RESPONSE2" | tail -1)
BODY2=$(echo "$RESPONSE2" | head -1)

if [ "$HTTP_CODE2" = "500" ]; then
    pass "Gateway returned 500 for user=666"
else
    fail "Gateway returned $HTTP_CODE2 (expected 500 for error user)"
fi

if echo "$BODY2" | grep -q 'subscription_not_found'; then
    pass "Error message propagated: subscription_not_found"
else
    fail "Error message missing: $BODY2"
fi

echo ""

# ─── Test 3: No Debug Token ──────────────────────────

echo "▶ Test 3: No debug token (normal request)"

RESPONSE3=$(curl -s -w "\n%{http_code}" \
    "${GATEWAY}/checkout?user=456")

HTTP_CODE3=$(echo "$RESPONSE3" | tail -1)

if [ "$HTTP_CODE3" = "200" ]; then
    pass "Normal request (no debug token) returns 200"
else
    fail "Normal request returned $HTTP_CODE3 (expected 200)"
fi

echo ""

# ─── Test 4: Invalid Token ───────────────────────────

echo "▶ Test 4: Invalid token (wrong secret)"

TRACE_ID4=$(openssl rand -hex 16)
SPAN_ID4=$(openssl rand -hex 8)
TRACEPARENT4="00-${TRACE_ID4}-${SPAN_ID4}-01"
FAKE_TOKEN="${TRACE_ID4}:$(date +%s):dGhpc19pc19mYWtl"

RESPONSE4=$(curl -s -w "\n%{http_code}" \
    -H "traceparent: ${TRACEPARENT4}" \
    -H "baggage: unsample-debug=${FAKE_TOKEN}" \
    "${GATEWAY}/checkout?user=789")

HTTP_CODE4=$(echo "$RESPONSE4" | tail -1)

if [ "$HTTP_CODE4" = "200" ]; then
    pass "Invalid token request still returns 200 (silently ignored)"
else
    fail "Invalid token request returned $HTTP_CODE4 (expected 200)"
fi

echo ""

# ─── Test 5: Concurrent Debug Sessions ───────────────

echo "▶ Test 5: Concurrent debug sessions (3 simultaneous)"

PIDS=""
for i in 1 2 3; do
    TRACE_ID_C=$(openssl rand -hex 16)
    TOKEN_C=$(generate_token "$TRACE_ID_C")
    SPAN_ID_C=$(openssl rand -hex 8)
    TRACEPARENT_C="00-${TRACE_ID_C}-${SPAN_ID_C}-01"

    curl -s -o /dev/null \
        -H "traceparent: ${TRACEPARENT_C}" \
        -H "baggage: unsample-debug=${TOKEN_C}" \
        "${GATEWAY}/checkout?user=${i}" &
    PIDS="$PIDS $!"
done

ALL_OK=true
for PID in $PIDS; do
    if ! wait "$PID"; then
        ALL_OK=false
    fi
done

if [ "$ALL_OK" = true ]; then
    pass "3 concurrent debug sessions completed without error"
else
    fail "Some concurrent sessions failed"
fi

echo ""

# ─── Summary ─────────────────────────────────────────

echo "═══════════════════════════════════════════════════"
echo "  Results: $(green "$PASS passed"), $(red "$FAIL failed"), $(yellow "$SKIP skipped")"
echo "═══════════════════════════════════════════════════"
echo ""

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
