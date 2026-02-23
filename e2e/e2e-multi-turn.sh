#!/usr/bin/env bash
# E2E: Multi-Turn — Tests context continuity across 2 messages in the same session
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

section "E2E: Multi-Turn (session context continuity)"

setup_env
build_ozzie
start_gateway

TARGET="$E2E_RUNDIR/output"
mkdir -p "$TARGET"

# ── Turn 1: create a file with a secret word ────────────────────────────────

SID=$(ozzie_ask_session \
    "Create a file at $TARGET/secret.txt containing exactly the word BANANA42 on its own line. Nothing else." \
    "" 120)

info "Session ID: ${SID:-<not captured>}"

wait_tasks_done 180

# ── Turn 2: ask what the secret word was (same session) ─────────────────────

if [ -n "$SID" ]; then
    ozzie_ask_session \
        "What was the secret word I asked you to write earlier? Reply with just the word." \
        "$SID" 120 > /dev/null
    TURN2_RESPONSE=$(cat "$E2E_RUNDIR/.last_stdout" 2>/dev/null || true)
    info "Turn 2 response captured"
else
    warn "Could not capture session ID — skipping turn 2"
    TURN2_RESPONSE=""
fi

# ── Assertions ───────────────────────────────────────────────────────────────

section "Assertions — functional"

soft_assert_log_contains "task.created"

section "Assertions — model-dependent"

soft_assert_file_exists "$TARGET/secret.txt"
soft_assert_file_contains "$TARGET/secret.txt" "BANANA42"

# Check session messages count (should have >= 4 lines: 2 user + 2 assistant)
if [ -n "$SID" ]; then
    MESSAGES_FILE="$OZZIE_PATH/sessions/$SID/messages.jsonl"
    if [ -f "$MESSAGES_FILE" ]; then
        LINE_COUNT=$(wc -l < "$MESSAGES_FILE" | tr -d ' ')
        if [ "$LINE_COUNT" -ge 4 ]; then
            pass "Session has $LINE_COUNT messages (>= 4)"
        else
            warn "Session has only $LINE_COUNT messages (expected >= 4)"
        fi
    else
        warn "Messages file not found: $MESSAGES_FILE"
    fi
fi

# Check that the agent recalled the secret word in turn 2
if echo "$TURN2_RESPONSE" | grep -qi "BANANA42"; then
    pass "Turn 2 response contains BANANA42 (context preserved)"
else
    warn "Turn 2 response did not mention BANANA42 (context may not have been loaded)"
fi

# teardown is called via EXIT trap
