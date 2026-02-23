#!/usr/bin/env bash
# E2E: Edit File — Tests read_file + edit_file on an existing file
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

section "E2E: Edit File (read + edit existing file)"

setup_env
build_ozzie
start_gateway

TARGET="$E2E_RUNDIR/output"
mkdir -p "$TARGET"

# ── Seed a config file with intentional issues ──────────────────────────────

cat > "$TARGET/config.yaml" << 'YAML'
server:
  host: localhost
  port: 8080
  debug: true
database:
  host: localhost
  port: 5432
  name: myapp_db
  max_connections: 5
YAML

info "Seeded $TARGET/config.yaml with debug: true, max_connections: 5"

# ── Ask the agent to fix the file ───────────────────────────────────────────

ozzie_ask "The file at $TARGET/config.yaml has debug mode enabled in production. Please read the file and change debug from true to false. Also increase max_connections from 5 to 20." 180

wait_tasks_done 180

# ── Assertions ───────────────────────────────────────────────────────────────

section "Assertions — functional"

assert_file_exists "$TARGET/config.yaml"
soft_assert_log_contains "task.created"

section "Assertions — edit results (model-dependent)"

soft_assert_file_contains "$TARGET/config.yaml" "debug: false"
soft_assert_file_contains "$TARGET/config.yaml" "max_connections: 20"
soft_assert_file_not_contains "$TARGET/config.yaml" "debug: true"

# Verify the file still has the untouched fields
soft_assert_file_contains "$TARGET/config.yaml" "host: localhost"
soft_assert_file_contains "$TARGET/config.yaml" "port: 8080"
soft_assert_file_contains "$TARGET/config.yaml" "name: myapp_db"

# teardown is called via EXIT trap
