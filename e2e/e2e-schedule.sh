#!/usr/bin/env bash
# E2E: Dynamic Schedule — Tests schedule creation, triggering, and removal
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

section "E2E: Dynamic Schedule"

setup_env
build_ozzie
start_gateway

TARGET="$E2E_RUNDIR/output"
mkdir -p "$TARGET"

# Prompt 1: create a recurring schedule
ozzie_ask "I want to monitor disk usage on this machine. Set up a recurring check every 20 seconds that runs df -h > $TARGET/disk-usage.txt — limit it to 3 runs maximum." 120

# Wait for 2-3 triggers (~60s)
info "Waiting 60s for schedule triggers..."
sleep 60

# Prompt 2: remove the schedule
ozzie_ask "The disk monitoring is no longer needed. Please remove the recurring schedule." 120

# ── Assertions ───────────────────────────────────────────────────────────────
# NB: All soft — the agent might choose to just run df -h directly instead of scheduling.

section "Assertions — schedule (model-dependent)"

soft_assert_log_contains "schedule.created"
soft_assert_log_contains "schedule.trigger"
soft_assert_task_count_ge 1
soft_assert_log_contains "schedule.removed"
soft_assert_file_exists "$TARGET/disk-usage.txt"

# teardown is called via EXIT trap
