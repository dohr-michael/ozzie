#!/usr/bin/env bash
# E2E: Coordinator Supervised — Tests the full coordinator flow with forced supervised mode
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

section "E2E: Coordinator Supervised"

setup_env
build_ozzie

# Patch config to force supervised mode
info "Patching config: default_level → supervised"
sed -i '' 's/"default_level": *"disabled"/"default_level": "supervised"/' "$OZZIE_PATH/config.jsonc"

start_gateway

TARGET="$E2E_RUNDIR/kvstore"
mkdir -p "$TARGET"

ozzie_ask "Create a standalone Go module at $TARGET that implements a simple key-value store with Get, Set, Delete methods and unit tests. Initialize the module with 'go mod init kvstore' so it has its own go.mod. Make sure it compiles and tests pass." 300

wait_tasks_done 300

# ── Assertions ───────────────────────────────────────────────────────────────

section "Assertions — functional"

assert_log_contains "task.created"
assert_task_count_ge 1

section "Assertions — output (model-dependent)"

soft_assert_file_exists "$TARGET/go.mod"
soft_assert_file_exists "$TARGET/kvstore.go"

section "Assertions — coordinator flow (model-dependent)"

soft_assert_log_contains "task.suspended"
soft_assert_log_contains "task.resumed"
soft_assert_log_contains "task.completed"

# teardown is called via EXIT trap
