#!/usr/bin/env bash
# E2E-2: Git Monitor — Tests scheduler cron, event triggers, memory
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

section "E2E-2: Git Monitor"

setup_env
build_ozzie

# Create a minimal cron skill
cat > "$OZZIE_PATH/skills/git-watcher.jsonc" << 'SKILL'
{
    "name": "git-watcher",
    "description": "Check for new commits in a monitored git repo",
    "type": "simple",
    "instruction": "Check the git log of the repo at $REPO_PATH. Use store_memory to remember the latest commit hash. Report if there are new commits since last check.",
    "tools": ["cmd", "store_memory", "query_memories"],
    "triggers": {
        "delegation": true,
        "cron": "* * * * *"
    }
}
SKILL

# Create a test git repo
REPO_PATH="$OZZIE_PATH/test-repo"
mkdir -p "$REPO_PATH"
git -C "$REPO_PATH" init
echo "hello" > "$REPO_PATH/README.md"
git -C "$REPO_PATH" add .
git -C "$REPO_PATH" -c user.name="test" -c user.email="test@test.com" commit -m "initial"

# Inject repo path into skill instruction
sed -i '' "s|\$REPO_PATH|$REPO_PATH|g" "$OZZIE_PATH/skills/git-watcher.jsonc"

start_gateway

# Wait 2 cron cycles (~130s)
info "Waiting for 2 cron cycles (130s)..."
sleep 130

# ── Assertions — first cycle ────────────────────────────────────────────────

section "Assertions — first cycle"

assert_log_contains "schedule.trigger"
assert_log_contains "skill.started"
assert_log_contains "git-watcher"

# Add a commit and wait for another cycle
echo "world" >> "$REPO_PATH/README.md"
git -C "$REPO_PATH" add .
git -C "$REPO_PATH" -c user.name="test" -c user.email="test@test.com" commit -m "second commit"

info "Waiting for another cron cycle (70s)..."
sleep 70

# ── Assertions — second cycle ───────────────────────────────────────────────

section "Assertions — second cycle"

assert_log_contains "schedule.trigger"
soft_assert_memory_exists ""

# teardown is called via EXIT trap
