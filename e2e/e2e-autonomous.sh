#!/usr/bin/env bash
# E2E: Autonomous Mode — Tests coordinator autonomous execution (no human validation)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

section "E2E: Autonomous Mode (coordinator autonomous)"

setup_env
build_ozzie

# Patch config to force autonomous mode
info "Patching config: default_level → autonomous"
sed -i '' 's/"default_level": *"disabled"/"default_level": "autonomous"/' "$OZZIE_PATH/config.jsonc"

start_gateway

TARGET="$E2E_RUNDIR/hellocli"
mkdir -p "$TARGET"

# ── Ask the agent to create a Go CLI tool ────────────────────────────────────

ozzie_ask "Create a Go CLI tool at $TARGET that takes a name as argument and prints 'Hello, <name>!'. Include a go.mod with 'go mod init hellocli'. Make sure it compiles with 'go build'." 300

wait_tasks_done 300

# ── Assertions ───────────────────────────────────────────────────────────────

section "Assertions — functional"

assert_file_exists_recursive "$TARGET" "go.mod"
assert_file_exists_recursive "$TARGET" "main.go"

section "Assertions — task delegation (model-dependent)"

soft_assert_log_contains "task.created"
soft_assert_task_count_ge 1

section "Assertions — autonomous flow (model-dependent)"

# In autonomous mode, tasks should NOT be suspended
if grep -r -q "task.suspended" "$OZZIE_PATH/logs/" 2>/dev/null; then
    warn "task.suspended found in logs (autonomous mode should not suspend)"
else
    pass "No task.suspended in logs (autonomous flow)"
fi

soft_assert_log_contains "task.completed"

# teardown is called via EXIT trap
