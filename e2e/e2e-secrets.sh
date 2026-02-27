#!/usr/bin/env bash
# E2E: Secrets — Test age encryption, password prompts, set_secret tool, hot reload.
#
# Flow:
#   1. ozzie wake → generates .age-key
#   2. ozzie gateway starts with encryption enabled
#   3. Ask the agent to collect and store a secret via password prompt
#   4. Verify:
#      - Agent sent a password prompt
#      - Encrypted value (ENC[age:...]) was used (not plaintext)
#      - set_secret was called
#      - Secret is in .env
#      - Config hot-reload works (SIGHUP)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

section "E2E: Secrets (encryption + set_secret + hot reload)"

setup_env
build_ozzie

# ── Step 1: Onboarding — generate age key ────────────────────────────────

section "Onboarding (ozzie wake)"

ozzie wake 2>&1 || true

assert_file_exists "$OZZIE_PATH/.age-key"

# Verify permissions (0600)
PERMS=$(stat -f "%Lp" "$OZZIE_PATH/.age-key" 2>/dev/null || stat -c "%a" "$OZZIE_PATH/.age-key" 2>/dev/null)
if [ "$PERMS" = "600" ]; then
    pass ".age-key has correct permissions (600)"
else
    fail ".age-key permissions: $PERMS (expected 600)"
fi

# ── Step 2: Build secret_flow helper ─────────────────────────────────────

section "Building secret_flow helper"

(cd "$PROJECT_ROOT" && go build -o "$E2E_DIR/.bin/secret_flow" ./e2e/helpers/)
pass "secret_flow built"

# ── Step 3: Start gateway ────────────────────────────────────────────────

start_gateway

# Verify age key is loaded (check gateway log for warning absence)
sleep 1
if grep -q "age key not found" "$OZZIE_PATH/logs/gateway.log" 2>/dev/null; then
    fail "Gateway could not load age key"
else
    pass "Gateway loaded age key (encryption enabled)"
fi

# ── Step 4: Run secret flow via WS ──────────────────────────────────────

section "Secret flow (password prompt → encrypt → set_secret → .env)"

SECRET_VALUE="e2e-discord-token-$(date +%s)"
ENV_NAME="E2E_DISCORD_TOKEN"

secret_flow \
    -gateway "ws://127.0.0.1:${E2E_PORT}/api/ws" \
    -secret "$SECRET_VALUE" \
    -env-name "$ENV_NAME" \
    -timeout 180s \
    > "$E2E_RUNDIR/secret_flow.log" 2>&1 || true

# Display helper output
cat "$E2E_RUNDIR/secret_flow.log"

# ── Step 5: Assertions — functional ─────────────────────────────────────

section "Assertions — functional"

assert_file_exists "$OZZIE_PATH/.age-key"
assert_log_contains "prompt.request"

section "Assertions — model-dependent (agent must use password prompt + set_secret)"

# The agent should have sent a password prompt
soft_assert_log_contains "prompt.response"

# set_secret should have been called
soft_assert_log_contains "set_secret"

# .env should contain the secret (decrypted)
if [ -f "$OZZIE_PATH/.env" ] && grep -q "${ENV_NAME}=" "$OZZIE_PATH/.env" 2>/dev/null; then
    pass ".env contains $ENV_NAME"

    # Verify the value is the actual secret (decrypted, not encrypted)
    if grep -q "${ENV_NAME}=${SECRET_VALUE}" "$OZZIE_PATH/.env" 2>/dev/null; then
        pass ".env value is decrypted plaintext"
    elif grep -q "ENC\[age:" "$OZZIE_PATH/.env" 2>/dev/null; then
        fail ".env contains encrypted blob (should be decrypted)"
    else
        warn ".env value doesn't match expected (model may have altered it)"
    fi
else
    warn ".env missing $ENV_NAME (model may not have completed the flow)"
fi

# ── Step 6: Hot reload (SIGHUP) ─────────────────────────────────────────

section "Hot reload (SIGHUP)"

# Manually write a new secret to .env
echo "SIGHUP_TEST_KEY=sighup_value_42" >> "$OZZIE_PATH/.env"

# Send SIGHUP to gateway
kill -HUP "$GATEWAY_PID" 2>/dev/null || true
sleep 2

# Check gateway log for reload
if grep -q "config reloaded" "$OZZIE_PATH/logs/gateway.log" 2>/dev/null; then
    pass "Config reloaded after SIGHUP"
else
    fail "Config reload not detected in logs after SIGHUP"
fi

# ── Step 7: Security checks ─────────────────────────────────────────────

section "Security checks"

# Verify the secret_flow helper didn't see plaintext in set_secret args
if grep -q "SECURITY:" "$E2E_RUNDIR/secret_flow.log" 2>/dev/null; then
    fail "Plaintext secret leaked to set_secret arguments!"
else
    pass "No plaintext leakage detected"
fi

# Verify the gateway log doesn't contain the raw secret
if grep -q "$SECRET_VALUE" "$OZZIE_PATH/logs/gateway.log" 2>/dev/null; then
    warn "Raw secret found in gateway logs (may be in debug output)"
else
    pass "Raw secret not found in gateway logs"
fi

# teardown is called via EXIT trap
