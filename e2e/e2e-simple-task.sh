#!/usr/bin/env bash
# E2E: Simple Task — Delegate a simple objective to a background task
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

section "E2E: Simple Task"

setup_env
build_ozzie
start_gateway

TARGET="$E2E_RUNDIR/output"
mkdir -p "$TARGET"

ozzie_ask "I need a small text file into $TARGET/haiku.txt containing a haiku about programming.
Delegate this to a background task so I don't have to wait.
Let me know the task ID, then monitor it until it's done." 180

wait_tasks_done 180

# ── Assertions ───────────────────────────────────────────────────────────────

section "Assertions — functional"

assert_log_contains "task.created"
assert_task_count_ge 1

section "Assertions — model-dependent"

soft_assert_log_contains "task.completed"
soft_assert_file_exists "$TARGET/haiku.txt"

# teardown is called via EXIT trap
