#!/usr/bin/env bash
# E2E: Memory — Tests store_memory, query_memories, cross-session recall, and reinforcement
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

section "E2E: Memory (store, query, cross-session recall)"

setup_env
build_ozzie
start_gateway

# ── Test 1: Store a memory explicitly ─────────────────────────────────────

section "Test 1: store_memory"

SID1=$(ozzie_ask_session \
    "Please store a memory with store_memory: title='E2E deploy convention', type='procedure', tags=['e2e','deploy'], content='Always run tests before deploying. Use blue-green deployment for zero downtime.'. Then confirm you stored it." \
    "" 120)

info "Session 1 ID: ${SID1:-<not captured>}"

# Give time for the memory to be written to disk
sleep 5

section "Assertions — store_memory"

soft_assert_memory_exists "deploy"

# Count memories after store
MEMORY_COUNT_AFTER_STORE=$(find "$OZZIE_PATH/memory" -name "*.json" -not -name "index.json" 2>/dev/null | wc -l | tr -d ' ')
info "Memory count after store: $MEMORY_COUNT_AFTER_STORE"

# ── Test 2: Query memories in a new session ───────────────────────────────

section "Test 2: query_memories (cross-session)"

SID2=$(ozzie_ask_session \
    "I need to deploy something. Use query_memories to search for any deployment conventions or procedures we have stored. Tell me what you find." \
    "" 120)

info "Session 2 ID: ${SID2:-<not captured>}"

QUERY_RESPONSE=$(cat "$E2E_RUNDIR/.last_stdout" 2>/dev/null || true)

section "Assertions — query_memories"

# The response should mention the deploy convention we stored
if echo "$QUERY_RESPONSE" | grep -qi "blue-green\|zero downtime\|tests before deploy"; then
    pass "Cross-session query recalled deploy convention"
else
    warn "Query response did not mention deploy convention (model may not have used query_memories)"
fi

soft_assert_log_contains "query_memories"

# ── Test 3: Store a preference and query it back ──────────────────────────

section "Test 3: preference storage and recall"

SID3=$(ozzie_ask_session \
    "Store a memory with store_memory: title='Code style preference', type='preference', tags=['code','style'], content='Use 4 spaces for indentation. Prefer single quotes in JavaScript.'. Confirm it was stored." \
    "" 120)

info "Session 3 ID: ${SID3:-<not captured>}"
sleep 3

# New session: ask about code style
SID4=$(ozzie_ask_session \
    "I'm about to write some JavaScript. Use query_memories to check if we have any code style preferences stored. What indentation should I use?" \
    "" 120)

STYLE_RESPONSE=$(cat "$E2E_RUNDIR/.last_stdout" 2>/dev/null || true)

section "Assertions — preference recall"

if echo "$STYLE_RESPONSE" | grep -qi "4 spaces\|single quotes\|indentation"; then
    pass "Preference recalled in new session"
else
    warn "Style preference not recalled (model may not have used query_memories)"
fi

# ── Test 4: Memory reinforcement (LastUsedAt / Confidence update) ─────────

section "Test 4: reinforcement"

# Find a memory file and record its current state
MEMORY_FILE=$(find "$OZZIE_PATH/memory" -name "mem_*.json" -type f 2>/dev/null | head -1 || true)

if [ -n "$MEMORY_FILE" ]; then
    BEFORE_CONFIDENCE=$(grep -oE '"confidence":\s*[0-9.]+' "$MEMORY_FILE" 2>/dev/null | grep -oE '[0-9.]+' || echo "unknown")
    BEFORE_LAST_USED=$(grep -oE '"last_used_at":\s*"[^"]*"' "$MEMORY_FILE" 2>/dev/null || echo "unknown")
    info "Before reinforcement — confidence: $BEFORE_CONFIDENCE, last_used: $BEFORE_LAST_USED"

    # Query again to trigger reinforcement
    ozzie_ask "Use query_memories to search for 'deploy convention'. Tell me what you find." 120
    sleep 3

    AFTER_CONFIDENCE=$(grep -oE '"confidence":\s*[0-9.]+' "$MEMORY_FILE" 2>/dev/null | grep -oE '[0-9.]+' || echo "unknown")
    AFTER_LAST_USED=$(grep -oE '"last_used_at":\s*"[^"]*"' "$MEMORY_FILE" 2>/dev/null || echo "unknown")
    info "After reinforcement — confidence: $AFTER_CONFIDENCE, last_used: $AFTER_LAST_USED"

    # Check if confidence increased or last_used changed
    if [ "$BEFORE_LAST_USED" != "$AFTER_LAST_USED" ] || [ "$BEFORE_CONFIDENCE" != "$AFTER_CONFIDENCE" ]; then
        pass "Memory reinforced (confidence or last_used_at changed)"
    else
        warn "Memory not reinforced (values unchanged — retrieval may not have matched)"
    fi
else
    warn "No memory files found to test reinforcement"
fi

# ── Test 5: Memory count sanity check ─────────────────────────────────────

section "Test 5: memory store integrity"

FINAL_COUNT=$(find "$OZZIE_PATH/memory" -name "mem_*.json" -type f 2>/dev/null | wc -l | tr -d ' ')
if [ "$FINAL_COUNT" -ge 2 ]; then
    pass "Memory store has $FINAL_COUNT entries (>= 2 expected)"
else
    warn "Memory store has only $FINAL_COUNT entries (expected >= 2)"
fi

# Check memory files are valid JSON
INVALID_JSON=0
for f in "$OZZIE_PATH/memory"/mem_*.json; do
    [ -f "$f" ] || continue
    if ! python3 -c "import json; json.load(open('$f'))" 2>/dev/null; then
        fail "Invalid JSON in memory file: $(basename "$f")"
        INVALID_JSON=$((INVALID_JSON + 1))
    fi
done
if [ "$INVALID_JSON" -eq 0 ] && [ "$FINAL_COUNT" -gt 0 ]; then
    pass "All memory files are valid JSON"
fi

# teardown is called via EXIT trap
