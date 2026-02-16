#!/usr/bin/env bash
# PreToolUse:Read hook — intercept large code file reads when interserve toggle is ON.
# Denies the first read and suggests codex_query. Second read always passes through.
set -euo pipefail

main() {
  local project_dir flag_file payload file_path offset session_id

  project_dir="${CLAUDE_PROJECT_DIR:-.}"
  flag_file="$project_dir/.claude/interserve-toggle.flag"

  # If interserve mode is OFF, pass through
  [[ -f "$flag_file" ]] || exit 0

  command -v jq >/dev/null 2>&1 || exit 0

  payload="$(cat || true)"
  [[ -n "$payload" ]] || exit 0

  file_path="$(jq -r '(.tool_input.file_path // empty)' <<<"$payload" 2>/dev/null || true)"
  [[ -n "$file_path" ]] || exit 0

  # Allow targeted reads (with offset) — Claude knows what it's looking for
  offset="$(jq -r '(.tool_input.offset // empty)' <<<"$payload" 2>/dev/null || true)"
  [[ -z "$offset" ]] || exit 0

  # Allow /tmp/ files
  [[ "$file_path" != /tmp/* ]] || exit 0

  # Allow config/doc extensions (matches interserve-audit.sh allowlist)
  case "$file_path" in
    *.md|*.json|*.yaml|*.yml|*.toml|*.txt|*.csv|*.xml|*.html|*.css|*.svg|*.lock|*.cfg|*.ini|*.conf|*.env|*.pdf|*.png|*.jpg|*.gif|*.ico)
      exit 0
      ;;
  esac

  # Check file exists and count lines
  [[ -f "$file_path" ]] || exit 0
  local line_count
  line_count=$(wc -l < "$file_path" 2>/dev/null) || exit 0
  line_count="${line_count// /}"

  # Allow small files (under 200 lines)
  if [[ "$line_count" -lt 200 ]]; then
    exit 0
  fi

  # Dedup: second read of same file in same session passes through
  session_id="$(jq -r '(.session_id // empty)' <<<"$payload" 2>/dev/null || true)"
  if [[ -n "$session_id" ]]; then
    local file_hash flag
    file_hash=$(echo -n "$file_path" | md5sum | cut -d' ' -f1)
    flag="/tmp/interserve-read-denied-${session_id}-${file_hash}"
    if [[ -f "$flag" ]]; then
      exit 0
    fi
    touch "$flag" 2>/dev/null || true
  fi

  # Deny with codex_query suggestion
  local rel_path="$file_path"
  if [[ "$file_path" == "$project_dir"/* ]]; then
    rel_path="${file_path#"$project_dir"/}"
  fi

  cat <<ENDJSON
{"decision": "block", "reason": "INTERSERVE: ${rel_path} is ${line_count} lines. Use codex_query(question='...', files=['${file_path}']) to save ~${line_count} tokens. Modes: answer (default), summarize, extract."}
ENDJSON
}

main "$@" || true
exit 0
