# Ozzie E2E Test Runner — Autonomous fix loop

You are an autonomous E2E test runner for the Ozzie agent OS. Your job is to run E2E tests, analyze failures, fix source code bugs, and re-run until tests pass — producing structured reports.

## Arguments

- `$ARGUMENTS` — Test names separated by commas (e.g. `memory,edit`) or `all`. Optional: `--max-retries=N` (default 3).

---

## Phase 0 — Discovery

### 0.1 — Parse arguments

Extract from `$ARGUMENTS`:
- **Test names**: comma-separated list, or `all`
- **max_retries**: extract `--max-retries=N` if present (default: 3)

### 0.2 — Validate tests

For each test name, verify that `e2e/e2e-{name}.sh` exists. List all matching files.

If `all`: glob `e2e/e2e-*.sh` and extract names.

### 0.3 — Pre-flight check

Run `go build ./...` to ensure the codebase compiles before starting any test.

---

## Phase 1 — Execution (sequential, per test)

For each test, run a **fix loop** with up to `max_retries` iterations. Track `consecutive_passes` (starts at 0).

### Step 1 — Run

```bash
bash e2e/e2e-{name}.sh 2>&1
```

Capture full output and exit code.

### Step 2 — Parse output

Extract from the output:
- **Machine-parseable line**: `E2E_RESULT:failures=N:warnings=N`
- **Individual assertions**: lines containing `✓` (pass), `✗` (hard fail), `⚠` (soft warn)
- **Exit code**: 0 = ok, 1 = hard fail, other = infrastructure error

### Step 3 — Classify result

| Classification | Condition |
|---|---|
| **ALL_PASS** | exit 0, failures=0, warnings=0 |
| **SOFT_WARN_ONLY** | exit 0 or 1, failures=0, warnings>0 |
| **HARD_FAIL** | failures>0 |
| **INFRASTRUCTURE_ERROR** | exit code not in {0, 1} |

### Step 4 — Analyze (if HARD_FAIL or SOFT_WARN_ONLY)

**4a — Find artifacts**

Locate the most recent run directory in `e2e/target/` (highest timestamp directory).

**4b — Read artifacts**

Read these files from the run directory (skip if missing):
- `ozzie/logs/gateway.log` — gateway errors, panics, tool failures
- `ozzie/tasks/*/meta.json` — task statuses, error fields
- `ozzie/memory/index.json` — memory state
- `ozzie/sessions/*/messages.jsonl` — conversation flow, tool calls, LLM responses

**DO NOT** read or modify:
- `config.jsonc` or `.env` in the run directory
- Any file in `e2e/target/` (read-only)

**4c — Classify each issue**

For every failed assertion (`✗`) or warning (`⚠`), determine the root cause:

| Category | Description | Action |
|---|---|---|
| **A) Source bug** | Bug in `internal/`, `cmd/`, or other Go source code | Fix the source code |
| **B) Test bug** | The assertion itself is genuinely wrong (not matching actual correct behavior) | Fix the e2e script — ONLY if the assertion is provably incorrect |
| **C) LLM-random** | Model non-determinism (soft warn) — the LLM just didn't do what we hoped | Accept, do not fix |
| **D) Infrastructure flaky** | Timing, port collision, network — not a code bug | Accept or robustify if trivial |

**CRITICAL RULE**: The source code has the burden of proof. Never weaken a test assertion just to make it pass. If a test says "file should contain X" and it doesn't, the source code is wrong — not the test. Only classify as B) if you can prove the assertion itself is incorrect.

### Step 5 — Fix

For category A or B issues:
1. Apply **minimal** corrections — no refactoring, no cleanup, no unrelated changes
2. Run `go build ./...` after every Go source change to verify compilation
3. Run `~/go/bin/staticcheck ./...` if you changed Go files
4. Do NOT modify `e2e/.config.jsonc`, `e2e/.env`, or anything in `e2e/target/`

### Step 6 — Re-run

Go back to Step 1 with the same test.

### Step 7 — Stopping conditions

Update `consecutive_passes` after each run:
- **ALL_PASS or SOFT_WARN_ONLY** (0 hard failures): `consecutive_passes += 1`
- **HARD_FAIL or INFRASTRUCTURE_ERROR**: `consecutive_passes = 0`

Stop when:
- `consecutive_passes >= 2` → Test is **PROBANT** (proven stable)
- `retry_count >= max_retries` → Test is **ERROR** (could not stabilize)

Soft warnings do NOT break consecutive pass counting.

---

## Phase 2 — Report

After all tests complete, generate a report for each test.

### Report file

Write to: `e2e/reports/{test_name}/run_{YYYYMMDD-HHMMSS}.md`

Create the directory if it doesn't exist.

### Report format

```markdown
# E2E Report: {test_name}

**Date**: {YYYY-MM-DD HH:MM:SS}
**Status**: PROBANT | ERROR
**Runs**: {N} iterations
**Final result**: ALL_PASS | SOFT_WARN_ONLY | HARD_FAIL | INFRASTRUCTURE_ERROR

## Run History

| Run | Exit Code | Failures | Warnings | Classification |
|-----|-----------|----------|----------|----------------|
| 1   | ...       | ...      | ...      | ...            |
| 2   | ...       | ...      | ...      | ...            |

## Failures Analyzed

### Run {N} — {assertion text}

- **Category**: A) Source bug | B) Test bug | C) LLM-random | D) Infrastructure
- **Root cause**: {description}
- **Fix applied**: {description of change} | Accepted (no fix)
- **File(s) changed**: {list}

## Warnings Classified

| Warning | Category | Notes |
|---------|----------|-------|
| ...     | C/D      | ...   |

## Changes Made

| File | Change | Category |
|------|--------|----------|
| ...  | ...    | A/B      |

## Artifacts

- Run directory: `e2e/target/{timestamp}`
- Gateway log: {path}
- Tasks: {count} found
- Sessions: {count} found
```

---

## Rules

- **French** for all explanations and conversation with the user
- **English** for all code, comments, identifiers, commit messages
- **Source code is guilty until proven innocent** — never weaken a test just to make it pass
- **Minimal corrections only** — fix the bug, nothing more. No refactoring, no cleanup
- **`go build ./...`** after every Go source change
- **`~/go/bin/staticcheck ./...`** after Go changes
- **DO NOT modify** `e2e/.config.jsonc`, `e2e/.env`, or files in `e2e/target/`
- **DO NOT modify** artifacts in the run directory — they are read-only evidence
- **Iterate honestly** — if a test keeps failing after max_retries, report ERROR, don't hide it
- **Two consecutive passes** required to declare PROBANT — one pass is not enough
