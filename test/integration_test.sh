#!/usr/bin/env bash
# Integration test for clodex MCP server — extract_sections tool only
# (classify_sections requires live Codex CLI, tested separately)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLUGIN_ROOT="$(dirname "$SCRIPT_DIR")"
BINARY="${PLUGIN_ROOT}/bin/clodex-mcp"

echo "=== Building clodex-mcp ==="
cd "$PLUGIN_ROOT"
go build -o "$BINARY" ./cmd/clodex-mcp/

echo "=== Creating test document ==="
TEST_DOC=$(mktemp /tmp/clodex-test-XXXX.md)
cat > "$TEST_DOC" << 'EOF'
---
title: Test Document
---

# Main Title

Introduction.

## Security

Auth flow and credential handling.
Token validation.

## Performance

Query optimization patterns.
Cache invalidation strategy.

## Architecture

Module boundaries and coupling.
Dependency injection.

```python
## Not a section
code_here()
```

## Correctness

Data consistency checks.
Transaction safety.
EOF

echo "=== Testing extract_sections via JSON-RPC ==="
REQUEST='{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"extract_sections","arguments":{"file_path":"'"$TEST_DOC"'"}}}'

# Need to initialize first
INIT='{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.1.0"}}}'
INITIALIZED='{"jsonrpc":"2.0","method":"notifications/initialized"}'

RESPONSE=$(printf '%s\n%s\n%s\n' "$INIT" "$INITIALIZED" "$REQUEST" | "$BINARY" 2>/dev/null | tail -1)

echo "Response: $RESPONSE"

# Check we got sections
SECTION_COUNT=$(echo "$RESPONSE" | python3 -c "
import json,sys
r = json.loads(sys.stdin.read())
sections = json.loads(r['result']['content'][0]['text'])
print(len(sections))
")

echo "Sections found: $SECTION_COUNT"
if [[ "$SECTION_COUNT" -ne 5 ]]; then
    echo "FAIL: expected 5 sections (Preamble, Security, Performance, Architecture, Correctness), got $SECTION_COUNT"
    rm "$TEST_DOC"
    exit 1
fi

echo "=== Verifying code block handling ==="
# The "## Not a section" inside the code block should NOT create a section
HEADINGS=$(echo "$RESPONSE" | python3 -c "
import json,sys
r = json.loads(sys.stdin.read())
sections = json.loads(r['result']['content'][0]['text'])
for s in sections:
    print(s['heading'])
")
if echo "$HEADINGS" | grep -q "Not a section"; then
    echo "FAIL: code block ## was treated as a section heading"
    rm "$TEST_DOC"
    exit 1
fi

echo "=== Verifying YAML frontmatter was skipped ==="
if echo "$HEADINGS" | grep -qi "title\|frontmatter"; then
    echo "FAIL: YAML frontmatter was not skipped"
    rm "$TEST_DOC"
    exit 1
fi

echo "=== All integration tests passed ==="
rm "$TEST_DOC"
