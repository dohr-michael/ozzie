#!/usr/bin/env bash
set -euo pipefail

export OZZIE_PATH=./dev_home

exec go run ./cmd/ozzie "$@"
