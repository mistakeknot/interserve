# Interserve — Deprecated

**Status:** Deprecated as of 2026-03-01
**Replacement:** `core/intercore/internal/routing/` (unified routing engine)
**Bead:** iv-it0jq

## What happened

Interserve's three MCP tools (`classify_sections`, `extract_sections`, `codex_query`) have been superseded:

- **classify_sections** — Deterministic keyword matching moved to `intercore/internal/routing`. The LLM classification mode was never used in production.
- **extract_sections** — interflux Method 2 (keyword-based section extraction) provides equivalent functionality without an MCP dependency.
- **codex_query** — Codex dispatch is handled by `ic dispatch spawn` in intercore.

## Migration

- `routing_resolve_model` and `routing_resolve_agents` in `lib-routing.sh` now delegate to `ic route` when available (strangler-fig pattern).
- The interserve PreToolUse:Read hook should be disabled in plugin settings.
- No code changes needed for consumers — the bash API is unchanged.

## Philosophy alignment

> "Plugins are dumb and independent. The platform is smart and aware." — PHILOSOPHY.md

Routing is platform intelligence. It belongs in the kernel (L1), not as a plugin (L3).
