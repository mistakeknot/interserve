# interserve

Codex spark classifier and context compression for Claude Code.

## What This Does

interserve provides three MCP tools for token-efficient document handling:

**classify_sections** — takes a markdown document and classifies each section into flux-drive review domains (architecture, safety, correctness, etc.) by dispatching a Codex spark. This lets interflux know which review agents to launch without Claude having to read the entire document first.

**extract_sections** — splits a markdown document by `##` headings while properly handling fenced code blocks. Simple structural extraction, no AI involved.

**codex_query** — delegates file reading to Codex to save Claude's context window. When you need information from a large file but don't want to burn context tokens reading it, codex_query reads it in a separate process and returns a summary.

## Installation

```bash
/plugin install interserve
```

Requires Clavain's `dispatch.sh` for Codex spark dispatch (set `INTERSERVE_DISPATCH_PATH`).

## Architecture

```
cmd/interserve-mcp/    Go MCP server (mark3labs/mcp-go)
bin/launch-mcp.sh      Server launcher
```

Go binary with stdio MCP transport. Starts on-demand, no systemd unit.
