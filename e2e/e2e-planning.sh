#!/usr/bin/env bash
# E2E: Planning — Tests natural decomposition of a complex task (Flask → Go migration)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

section "E2E: Planning (Flask → Go migration)"

setup_env
build_ozzie

# Create a mini Flask application
mkdir -p "$OZZIE_PATH/flask-app"
cat > "$OZZIE_PATH/flask-app/app.py" << 'PY'
from flask import Flask, jsonify
app = Flask(__name__)

@app.route("/")
def home():
    return jsonify({"message": "Hello, World!"})

@app.route("/api/health")
def health():
    return jsonify({"status": "ok"})

if __name__ == "__main__":
    app.run(port=5000)
PY

cat > "$OZZIE_PATH/flask-app/requirements.txt" << 'REQ'
flask==3.0.0
REQ

start_gateway

TARGET="$E2E_RUNDIR/go-app"

ozzie_ask "I have a Python Flask app at $OZZIE_PATH/flask-app. I need you to analyze it, then create an equivalent Go application using chi router at $TARGET. The Go app should have the same routes and behavior. Make sure it compiles.
This is a multi-step project — plan your approach carefully." 600

wait_tasks_done 600

# ── Assertions ───────────────────────────────────────────────────────────────

section "Assertions — functional"

assert_file_exists_recursive "$TARGET" "go.mod"
assert_file_exists_recursive "$TARGET" "main.go"

section "Assertions — planning (model-dependent)"

soft_assert_log_contains "task.created"
soft_assert_task_count_ge 2
soft_assert_log_contains "task.completed"

# teardown is called via EXIT trap
