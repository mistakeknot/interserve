# Clodex

Codex spark classifier and context compression — MCP server exposing `classify_sections`, `extract_sections`, and `codex_query` tools.

## Quick Commands

```bash
cd plugins/clodex && go build -o bin/clodex-mcp ./cmd/clodex-mcp/
cd plugins/clodex && go test ./... -v
```

## Design Decisions (Do Not Re-Ask)

- Go binary (matches interlock-mcp pattern)
- Stdio MCP transport (on-demand, no systemd)
- Delegates tier resolution to dispatch.sh
