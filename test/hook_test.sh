#!/usr/bin/env bash
# Tests for pre-read-intercept.sh hook
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLUGIN_ROOT="$(dirname "$SCRIPT_DIR")"
HOOK="${PLUGIN_ROOT}/hooks/pre-read-intercept.sh"

PASS=0
FAIL=0
incr_pass() { PASS=$((PASS + 1)); }
incr_fail() { FAIL=$((FAIL + 1)); }
SESSION_ID="test-session-$$"

# Use a non-/tmp/ directory so the /tmp/ allowlist doesn't interfere
PROJECT_DIR="/root/projects/Interverse/plugins/clodex/test/.hook-test-$$"

cleanup() {
  rm -rf "$PROJECT_DIR"
  rm -f /tmp/clodex-read-denied-${SESSION_ID}-*
}
trap cleanup EXIT

# Create a project dir with clodex toggle flag
mkdir -p "$PROJECT_DIR/.claude"
echo "2026-02-16T00:00:00-08:00" > "$PROJECT_DIR/.claude/clodex-toggle.flag"

# Create test files
LARGE_GO="$PROJECT_DIR/internal/foo/big.go"
mkdir -p "$(dirname "$LARGE_GO")"
python3 -c "
for i in range(300):
    print(f'// line {i+1}')
" > "$LARGE_GO"

SMALL_GO="$PROJECT_DIR/internal/foo/small.go"
python3 -c "
for i in range(50):
    print(f'// line {i+1}')
" > "$SMALL_GO"

MD_FILE="$PROJECT_DIR/README.md"
python3 -c "
for i in range(500):
    print(f'line {i+1}')
" > "$MD_FILE"

run_hook() {
  local description="$1"
  local env_project_dir="$2"
  local input="$3"
  local expected_pattern="$4"  # "pass" for exit-0-no-output, or regex to match in output

  local output
  output=$(echo "$input" | CLAUDE_PROJECT_DIR="$env_project_dir" bash "$HOOK" 2>/dev/null) || true

  if [[ "$expected_pattern" == "pass" ]]; then
    if [[ -z "$output" ]]; then
      echo "  PASS: $description"
      incr_pass
    else
      echo "  FAIL: $description — expected pass-through (no output), got: $output"
      incr_fail
    fi
  else
    if echo "$output" | grep -qE "$expected_pattern"; then
      echo "  PASS: $description"
      incr_pass
    else
      echo "  FAIL: $description — expected pattern '$expected_pattern', got: $output"
      incr_fail
    fi
  fi
}

echo "=== Hook Tests ==="

# Test 1: No flag file → pass through
echo "--- Test 1: No clodex flag → pass through ---"
NO_FLAG_DIR="$PROJECT_DIR/no-flag-subdir"
mkdir -p "$NO_FLAG_DIR"
INPUT='{"tool_input":{"file_path":"'"$LARGE_GO"'"},"session_id":"'"$SESSION_ID"'"}'
run_hook "no flag file" "$NO_FLAG_DIR" "$INPUT" "pass"

# Test 2: .md file → pass through (even if large)
echo "--- Test 2: .md file → pass through ---"
INPUT='{"tool_input":{"file_path":"'"$MD_FILE"'"},"session_id":"'"$SESSION_ID"'"}'
run_hook "markdown file" "$PROJECT_DIR" "$INPUT" "pass"

# Test 3: Small file (<200 lines) → pass through
echo "--- Test 3: Small file → pass through ---"
INPUT='{"tool_input":{"file_path":"'"$SMALL_GO"'"},"session_id":"'"$SESSION_ID"'"}'
run_hook "small file" "$PROJECT_DIR" "$INPUT" "pass"

# Test 4: Large .go file with flag → deny with codex_query hint
echo "--- Test 4: Large .go file → deny ---"
rm -f /tmp/clodex-read-denied-${SESSION_ID}-*
INPUT='{"tool_input":{"file_path":"'"$LARGE_GO"'"},"session_id":"'"$SESSION_ID"'"}'
run_hook "large go file denied" "$PROJECT_DIR" "$INPUT" '"decision".*"block"'

# Test 5: Second read of same file → pass through (dedup flag)
echo "--- Test 5: Second read → pass through (dedup) ---"
INPUT='{"tool_input":{"file_path":"'"$LARGE_GO"'"},"session_id":"'"$SESSION_ID"'"}'
run_hook "second read passes" "$PROJECT_DIR" "$INPUT" "pass"

# Test 6: Read with offset → pass through
echo "--- Test 6: Read with offset → pass through ---"
rm -f /tmp/clodex-read-denied-${SESSION_ID}-*
INPUT='{"tool_input":{"file_path":"'"$LARGE_GO"'","offset":50},"session_id":"'"$SESSION_ID"'"}'
run_hook "offset read passes" "$PROJECT_DIR" "$INPUT" "pass"

# Test 7: /tmp/ file → pass through
echo "--- Test 7: /tmp/ file → pass through ---"
TMP_GO=$(mktemp /tmp/clodex-test-XXXX.go)
python3 -c "
for i in range(300):
    print(f'// line {i+1}')
" > "$TMP_GO"
INPUT='{"tool_input":{"file_path":"'"$TMP_GO"'"},"session_id":"'"$SESSION_ID"'"}'
run_hook "tmp file passes" "$PROJECT_DIR" "$INPUT" "pass"
rm -f "$TMP_GO"

# Test 8: .json file → pass through
echo "--- Test 8: .json file → pass through ---"
JSON_FILE="$PROJECT_DIR/config.json"
python3 -c "
for i in range(300):
    print(f'  \"key_{i}\": \"value\"')
" > "$JSON_FILE"
INPUT='{"tool_input":{"file_path":"'"$JSON_FILE"'"},"session_id":"'"$SESSION_ID"'"}'
run_hook "json file passes" "$PROJECT_DIR" "$INPUT" "pass"

# Test 9: Deny message includes codex_query hint
echo "--- Test 9: Deny message quality ---"
rm -f /tmp/clodex-read-denied-${SESSION_ID}-*
INPUT='{"tool_input":{"file_path":"'"$LARGE_GO"'"},"session_id":"'"$SESSION_ID"'"}'
run_hook "deny includes codex_query" "$PROJECT_DIR" "$INPUT" "codex_query"

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
