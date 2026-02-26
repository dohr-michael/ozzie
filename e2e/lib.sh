#!/usr/bin/env bash
# E2E test library — shared functions for all E2E scenarios.
set -euo pipefail

# ── Colors ──────────────────────────────────────────────────────────────────

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

section() { echo -e "\n${BLUE}═══ $1 ═══${NC}"; }
pass()    { echo -e "  ${GREEN}✓ $1${NC}"; }
fail()    { echo -e "  ${RED}✗ $1${NC}"; FAILURES=$((FAILURES + 1)); }
warn()    { echo -e "  ${YELLOW}⚠ $1${NC}"; WARNINGS=$((WARNINGS + 1)); }
info()    { echo -e "  ${YELLOW}→ $1${NC}"; }

FAILURES=0
WARNINGS=0
E2E_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$E2E_DIR/.." && pwd)"

# Load e2e/.env if present (E2E_PROVIDER, E2E_LOCAL_*, ANTHROPIC_API_KEY, etc.)
if [ -f "$E2E_DIR/.env" ]; then
    set -a
    # shellcheck source=/dev/null
    source "$E2E_DIR/.env"
    set +a
fi

# ── Environment setup ───────────────────────────────────────────────────────

setup_dirs() {
    local ts
    ts="$(date +%Y%m%d-%H%M%S)"
    export E2E_RUNDIR="$E2E_DIR/target/$ts"
    mkdir -p "$E2E_RUNDIR"
    export OZZIE_PATH="$E2E_RUNDIR/ozzie"
    mkdir -p "$OZZIE_PATH"/{logs,sessions,tasks,memory,skills,plugins,schedules}

    # Random port to avoid collisions
    E2E_PORT=$((20000 + RANDOM % 10000))
    export E2E_PORT

    # .env for Go template resolution (${{ .Env.* }})
    {
        echo "ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY:-}"
        echo "E2E_LOCAL_BASE_URL=${E2E_LOCAL_BASE_URL:-http://localhost:1234/v1}"
        echo "E2E_LOCAL_MODEL=${E2E_LOCAL_MODEL:-local-model}"
        echo "E2E_LOCAL_API_KEY=${E2E_LOCAL_API_KEY:-_}"
    } > "$OZZIE_PATH/.env"

    info "OZZIE_PATH=$OZZIE_PATH"
    info "Port=$E2E_PORT"
}

load_config() {
    local config_template="$E2E_DIR/.config.jsonc"

    if [ -f "$config_template" ]; then
        # Auto-detect: substitute ${E2E_PORT} and copy
        sed "s/\${E2E_PORT}/${E2E_PORT}/g" "$config_template" > "$OZZIE_PATH/config.jsonc"
        info "Config: auto-detected $config_template"
        return
    fi

    # Fallback: interactive wizard
    info "No .config.jsonc found, falling back to wizard"

    local provider="${E2E_PROVIDER:-}"
    if [ -z "$provider" ]; then
        echo ""
        echo -e "${BLUE}Which LLM provider do you want to use?${NC}"
        echo "  1) Anthropic (Claude) — requires ANTHROPIC_API_KEY"
        echo "  2) Local LLM via OpenAI-compatible API (LM Studio, Ollama, etc.)"
        echo ""
        read -rp "Choice [1]: " choice
        case "${choice:-1}" in
            2) provider="local" ;;
            *) provider="anthropic" ;;
        esac
    fi

    local model_config
    if [ "$provider" = "local" ]; then
        local base_url="${E2E_LOCAL_BASE_URL:-http://localhost:1234/v1}"
        local model_name="${E2E_LOCAL_MODEL:-local-model}"
        local api_key="${E2E_LOCAL_API_KEY:-_}"

        if [ -z "${E2E_LOCAL_BASE_URL:-}" ]; then
            read -rp "Base URL [${base_url}]: " input_url
            base_url="${input_url:-$base_url}"
        fi
        if [ -z "${E2E_LOCAL_MODEL:-}" ]; then
            read -rp "Model name [${model_name}]: " input_model
            model_name="${input_model:-$model_name}"
        fi

        model_config=$(cat <<MODELS
        "default": "local",
        "providers": {
            "local": {
                "driver": "openai",
                "model": "${model_name}",
                "base_url": "${base_url}",
                "auth": { "api_key": "${api_key}" },
                "max_tokens": 4096
            }
        }
MODELS
)
        info "Provider: local (${base_url}, model: ${model_name})"
    else
        # Anthropic
        if [ -z "${ANTHROPIC_API_KEY:-}" ]; then
            fail "ANTHROPIC_API_KEY not set"
            exit 1
        fi

        model_config=$(cat <<MODELS
        "default": "claude",
        "providers": {
            "claude": {
                "driver": "anthropic",
                "model": "claude-sonnet-4-6",
                "auth": { "api_key": "\${{ .Env.ANTHROPIC_API_KEY }}" },
                "max_tokens": 4096
            }
        }
MODELS
)
        info "Provider: Anthropic (Claude)"
    fi

    cat > "$OZZIE_PATH/config.jsonc" <<JSONC
{
    "gateway": {
        "host": "127.0.0.1",
        "port": ${E2E_PORT}
    },
    "models": {
${model_config}
    },
    "events": { "buffer_size": 1024 },
    "agent": {
        "system_prompt": "",
        "coordinator": { "default_level": "disabled", "max_validation_rounds": 3 }
    },
    "skills": { "dirs": ["${OZZIE_PATH}/skills"] },
    "tools": { "allowed_dangerous": ["cmd"] }
}
JSONC
}

setup_env() {
    section "Setting up environment"
    setup_dirs
    load_config
}

# ── Build ────────────────────────────────────────────────────────────────────

build_ozzie() {
    section "Building ozzie"
    mkdir -p "$E2E_DIR/.bin"
    (cd "$PROJECT_ROOT" && go build -o "$E2E_DIR/.bin/ozzie" ./cmd/ozzie)
    export PATH="$E2E_DIR/.bin:$PATH"
    pass "Binary built"
}

# ── Gateway lifecycle ────────────────────────────────────────────────────────

start_gateway() {
    section "Starting gateway on port $E2E_PORT"

    ozzie gateway --config "$OZZIE_PATH/config.jsonc" \
        --port "$E2E_PORT" \
        > "$OZZIE_PATH/logs/gateway.log" 2>&1 &
    GATEWAY_PID=$!
    export GATEWAY_PID

    # Wait for gateway to be ready
    local timeout=15
    local waited=0
    while ! curl -sf "http://127.0.0.1:${E2E_PORT}/api/health" > /dev/null 2>&1; do
        sleep 1
        waited=$((waited + 1))
        if [ $waited -ge $timeout ]; then
            fail "Gateway did not start within ${timeout}s"
            echo "--- gateway log ---"
            cat "$OZZIE_PATH/logs/gateway.log" || true
            exit 1
        fi
    done

    pass "Gateway started (PID=$GATEWAY_PID)"
}

stop_gateway() {
    if [ -n "${GATEWAY_PID:-}" ]; then
        kill "$GATEWAY_PID" 2>/dev/null || true
        wait "$GATEWAY_PID" 2>/dev/null || true
        unset GATEWAY_PID
        info "Gateway stopped"
    fi
}

# ── Portable timeout ─────────────────────────────────────────────────────────
# macOS doesn't ship GNU timeout; use gtimeout if available, else fallback.

_run_with_timeout() {
    local secs="$1"; shift
    if command -v gtimeout &>/dev/null; then
        gtimeout "$secs" "$@"
    elif command -v timeout &>/dev/null; then
        timeout "$secs" "$@"
    else
        # Pure-bash fallback: run in background, kill after deadline
        "$@" &
        local pid=$!
        ( sleep "$secs" && kill "$pid" 2>/dev/null ) &
        local watcher=$!
        wait "$pid" 2>/dev/null
        local rc=$?
        kill "$watcher" 2>/dev/null
        wait "$watcher" 2>/dev/null
        return $rc
    fi
}

# ── Commands ─────────────────────────────────────────────────────────────────

ozzie_ask() {
    local msg="$1"
    local secs="${2:-120}"
    info "Sending: ${msg:0:80}..."
    _run_with_timeout "$secs" ozzie ask -y \
        --gateway "ws://127.0.0.1:${E2E_PORT}/api/ws" \
        --timeout "$secs" \
        "$msg" 2>&1 || true
}

# ozzie_ask_session — send a message, capture/reuse session ID for multi-turn.
# Usage:
#   sid=$(ozzie_ask_session "first message")     # new session
#   ozzie_ask_session "follow-up" "$sid"          # same session
ozzie_ask_session() {
    local msg="$1"
    local session="${2:-}"
    local secs="${3:-120}"
    local session_args=()
    if [ -n "$session" ]; then
        session_args=(--session "$session")
    fi
    # info to stderr so it doesn't pollute the captured session ID
    info "Sending (session=${session:-new}): ${msg:0:80}..." >&2
    _run_with_timeout "$secs" ozzie ask -y \
        --gateway "ws://127.0.0.1:${E2E_PORT}/api/ws" \
        --timeout "$secs" \
        ${session_args[@]+"${session_args[@]}"} \
        "$msg" >"$E2E_RUNDIR/.last_stdout" 2>"$E2E_RUNDIR/.last_stderr" || true
    # Print response to terminal for visibility
    cat "$E2E_RUNDIR/.last_stdout" >&2
    # Only the session ID goes to stdout (captured by $(...))
    grep -oE 'sess_[a-zA-Z0-9_-]+' "$E2E_RUNDIR/.last_stderr" 2>/dev/null | head -1 || true
}

ozzie_cmd() {
    ozzie "$@" 2>&1 || true
}

# ── Hard assertions ─────────────────────────────────────────────────────────

assert_file_exists() {
    local file="$1"
    if [ -f "$file" ]; then
        pass "File exists: $(basename "$file")"
    else
        fail "File missing: $file"
    fi
}

assert_dir_exists() {
    local dir="$1"
    if [ -d "$dir" ]; then
        pass "Directory exists: $(basename "$dir")"
    else
        fail "Directory missing: $dir"
    fi
}

assert_log_contains() {
    local pattern="$1"
    if grep -r -q "$pattern" "$OZZIE_PATH/logs/" 2>/dev/null; then
        pass "Log contains: $pattern"
    else
        fail "Log missing: $pattern"
    fi
}

assert_task_count_ge() {
    local expected="$1"
    local count
    count=$(find "$OZZIE_PATH/tasks" -name "meta.json" 2>/dev/null | wc -l | tr -d ' ')
    if [ "$count" -ge "$expected" ]; then
        pass "Task count ($count) >= $expected"
    else
        fail "Task count ($count) < $expected"
    fi
}

assert_task_status() {
    local task_id="$1"
    local expected_status="$2"
    local meta="$OZZIE_PATH/tasks/$task_id/meta.json"
    if [ ! -f "$meta" ]; then
        fail "Task $task_id: meta.json not found"
        return
    fi
    if grep -qE "\"status\":\s*\"$expected_status\"" "$meta" 2>/dev/null; then
        pass "Task $task_id status: $expected_status"
    else
        local actual
        actual=$(grep -oE '"status":\s*"[^"]*"' "$meta" 2>/dev/null | head -1 || echo "unknown")
        fail "Task $task_id expected status '$expected_status', got $actual"
    fi
}

assert_task_has_mailbox() {
    local task_id="$1"
    local mailbox="$OZZIE_PATH/tasks/$task_id/mailbox.jsonl"
    if [ -f "$mailbox" ] && [ -s "$mailbox" ]; then
        pass "Task $task_id: mailbox non-empty"
    else
        fail "Task $task_id: mailbox empty or missing"
    fi
}

assert_schedule_count_ge() {
    local expected="$1"
    local count
    count=$(find "$OZZIE_PATH/schedules" -name "meta.json" 2>/dev/null | wc -l | tr -d ' ')
    if [ "$count" -ge "$expected" ]; then
        pass "Schedule count ($count) >= $expected"
    else
        fail "Schedule count ($count) < $expected"
    fi
}

assert_memory_exists() {
    local pattern="${1:-}"
    local count
    count=$(find "$OZZIE_PATH/memory" -name "*.json" -not -name "index.json" 2>/dev/null | wc -l | tr -d ' ')
    if [ "$count" -gt 0 ]; then
        if [ -n "$pattern" ]; then
            if grep -r -q "$pattern" "$OZZIE_PATH/memory/" 2>/dev/null; then
                pass "Memory contains: $pattern"
            else
                fail "Memory missing pattern: $pattern"
            fi
        else
            pass "Memory exists ($count entries)"
        fi
    else
        fail "No memories found"
    fi
}

assert_file_exists_recursive() {
    local dir="$1"
    local name="$2"
    if find "$dir" -name "$name" -type f 2>/dev/null | grep -q .; then
        local found
        found=$(find "$dir" -name "$name" -type f 2>/dev/null | head -1)
        pass "File exists: $name (at ${found#"$dir"/})"
    else
        fail "File missing: $name (not found under $dir)"
    fi
}

assert_file_contains() {
    local file="$1" pattern="$2"
    if [ -f "$file" ] && grep -q "$pattern" "$file"; then
        pass "File $file contains: $pattern"
    else
        fail "File $file missing pattern: $pattern"
    fi
}

assert_file_not_contains() {
    local file="$1" pattern="$2"
    if [ -f "$file" ] && grep -q "$pattern" "$file"; then
        fail "File $file still contains: $pattern"
    else
        pass "File $file does not contain: $pattern"
    fi
}

# ── Soft assertions (warn instead of fail) ──────────────────────────────────
# Use these for capabilities that depend on model intelligence (planning, memory).

soft_assert_file_exists() {
    local file="$1"
    if [ -f "$file" ]; then
        pass "File exists: $(basename "$file")"
    else
        warn "File missing: $file (model may not have created it)"
    fi
}

soft_assert_log_contains() {
    local pattern="$1"
    if grep -r -q "$pattern" "$OZZIE_PATH/logs/" 2>/dev/null; then
        pass "Log contains: $pattern"
    else
        warn "Log missing: $pattern (model did not use this feature)"
    fi
}

soft_assert_task_count_ge() {
    local expected="$1"
    local count
    count=$(find "$OZZIE_PATH/tasks" -name "meta.json" 2>/dev/null | wc -l | tr -d ' ')
    if [ "$count" -ge "$expected" ]; then
        pass "Task count ($count) >= $expected"
    else
        warn "Task count ($count) < $expected (model did not create sub-tasks)"
    fi
}

soft_assert_file_contains() {
    local file="$1" pattern="$2"
    if [ -f "$file" ] && grep -q "$pattern" "$file"; then
        pass "File $file contains: $pattern"
    else
        warn "File $file missing pattern: $pattern (model may not have edited correctly)"
    fi
}

soft_assert_file_not_contains() {
    local file="$1" pattern="$2"
    if [ -f "$file" ] && grep -q "$pattern" "$file"; then
        warn "File $file still contains: $pattern (model may not have removed it)"
    else
        pass "File $file does not contain: $pattern"
    fi
}

soft_assert_file_exists_recursive() {
    local dir="$1"
    local name="$2"
    if find "$dir" -name "$name" -type f 2>/dev/null | grep -q .; then
        local found
        found=$(find "$dir" -name "$name" -type f 2>/dev/null | head -1)
        pass "File exists: $name (at ${found#"$dir"/})"
    else
        warn "File missing: $name (not found under $dir)"
    fi
}

soft_assert_memory_exists() {
    local pattern="${1:-}"
    local count
    count=$(find "$OZZIE_PATH/memory" -name "*.json" -not -name "index.json" 2>/dev/null | wc -l | tr -d ' ')
    if [ "$count" -gt 0 ]; then
        if [ -n "$pattern" ]; then
            if grep -r -q "$pattern" "$OZZIE_PATH/memory/" 2>/dev/null; then
                pass "Memory contains: $pattern"
            else
                warn "Memory missing pattern: $pattern"
            fi
        else
            pass "Memory exists ($count entries)"
        fi
    else
        warn "No memories found (model did not use store_memory)"
    fi
}

# ── Utilities ───────────────────────────────────────────────────────────────

get_task_ids() {
    find "$OZZIE_PATH/tasks" -mindepth 1 -maxdepth 1 -type d -exec basename {} \; 2>/dev/null | sort
}

get_latest_task_id() {
    local latest=""
    local latest_ts=0
    for dir in "$OZZIE_PATH/tasks"/*/; do
        [ -d "$dir" ] || continue
        local meta="$dir/meta.json"
        [ -f "$meta" ] || continue
        local ts
        ts=$(grep -oE '"created_at":\s*"[^"]*"' "$meta" 2>/dev/null | head -1 | sed 's/.*"created_at":[[:space:]]*"//;s/"//' || echo "")
        if [ -n "$ts" ] && [[ "$ts" > "$latest_ts" ]]; then
            latest_ts="$ts"
            latest="$(basename "$dir")"
        fi
    done
    echo "$latest"
}

wait_task_status() {
    local task_id="$1"
    local expected_status="$2"
    local timeout="${3:-300}"
    local waited=0
    local meta="$OZZIE_PATH/tasks/$task_id/meta.json"

    info "Waiting for task $task_id to reach status '$expected_status' (timeout: ${timeout}s)..."

    while [ $waited -lt "$timeout" ]; do
        if [ -f "$meta" ] && grep -qE "\"status\":\s*\"$expected_status\"" "$meta" 2>/dev/null; then
            pass "Task $task_id reached status: $expected_status"
            return 0
        fi
        sleep 5
        waited=$((waited + 5))
    done

    warn "Task $task_id did not reach status '$expected_status' within ${timeout}s"
    return 1
}

wait_tasks_done() {
    local timeout="${1:-300}"
    local waited=0
    info "Waiting for tasks to complete (timeout: ${timeout}s)..."

    while [ $waited -lt "$timeout" ]; do
        local pending
        pending=$(find "$OZZIE_PATH/tasks" -name "meta.json" -exec grep -lE '"status":\s*"(pending|running|suspended)"' {} \; 2>/dev/null | wc -l | tr -d ' ')
        if [ "$pending" -eq 0 ]; then
            pass "All tasks completed"
            return 0
        fi
        sleep 5
        waited=$((waited + 5))
    done

    fail "Tasks still pending/running after ${timeout}s"
}

# ── Teardown ─────────────────────────────────────────────────────────────────

teardown() {
    section "Teardown"
    stop_gateway
    info "Run artifacts: $E2E_RUNDIR"

    echo "E2E_RESULT:failures=$FAILURES:warnings=$WARNINGS"

    echo ""
    if [ "$WARNINGS" -gt 0 ]; then
        echo -e "${YELLOW}WARNINGS: $WARNINGS soft assertion(s) (model-dependent features)${NC}"
    fi
    if [ "$FAILURES" -gt 0 ]; then
        echo -e "${RED}FAILED: $FAILURES assertion(s) failed${NC}"
        exit 1
    else
        echo -e "${GREEN}ALL PASSED${NC}"
    fi
}

# Ensure cleanup on unexpected exit
trap teardown EXIT
