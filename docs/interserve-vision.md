# interserve — Vision and Philosophy

**Version:** 0.1.0
**Last updated:** 2026-02-28

## What interserve Is

interserve is an MCP server that keeps Claude's context window free for orchestration by delegating the two most token-expensive operations to Codex spark: reading files and routing document sections. It exposes three tools — `extract_sections`, `classify_sections`, and `codex_query` — and shells out all Codex interaction to `dispatch.sh`, which resolves the execution tier. interserve is the context-preservation layer between Claude and the flux-drive multi-agent review system.

The `pre-read-intercept.sh` hook enforces this without requiring cooperation: when interserve is active, large file reads are intercepted at the PreToolUse level and redirected to `codex_query`. The enforcement is structural, not advisory.

## Why This Exists

Wasted tokens are not a cost concern — they are a quality concern. Context diluted with irrelevant file content increases hallucination risk, crowds out decision-relevant material, and slows feedback loops. interserve exists because routing document sections to the right reviewers and reading large files for specific answers are exactly the tasks that do not need Claude's full reasoning capability. A fast, cheap model can do both. The right architecture routes each task to the cheapest model that clears the bar — and interserve makes that routing automatic.

## Design Principles

1. **Context window is the scarcest resource.** Every tool call that routes work to Codex spark instead of Claude preserves capacity for decisions that require it. interserve never uses Claude for file reads or section classification when Codex can do it.

2. **Enforcement is structural, not advisory.** The hook intercepts large file reads before Claude acts on them. The tool API makes the right path the default path. Correct behavior does not depend on the agent choosing correctly.

3. **Delegate tier resolution, don't own it.** interserve never calls `codex` directly. `dispatch.sh` owns the execution tier — sandbox, model selection, timeout. interserve's job is prompt construction and result parsing, not infrastructure concerns.

4. **Classification produces evidence, not decisions.** `classify_sections` returns a structured routing map with relevance levels and confidence scores. What happens next — which agents run, in what order — is decided upstream. interserve closes the classification loop; it does not own the review loop.

5. **Fail explicitly, never silently.** A classification with no agent receiving >10% priority lines is marked failed, not returned as an empty result. A missing dispatch binary exits the server at startup, not mid-request. Ambiguous states surface immediately.

## Scope

**Does:**
- Split markdown documents into sections by `##` headings (fence-aware, skips frontmatter)
- Classify sections to flux-drive domain agents (fd-safety, fd-correctness, fd-performance, fd-user-product, fd-game-design) and cross-cutting agents (fd-architecture, fd-quality) with relevance and confidence scores
- Delegate file Q&A to Codex spark in three modes: answer, summarize, extract
- Intercept large file reads and suggest `codex_query` via a PreToolUse hook
- Cache query results to avoid redundant Codex calls for the same file+question pair

**Does not:**
- Execute flux-drive review jobs or aggregate results across agents
- Select execution tiers (dispatch.sh owns this)
- Parse non-markdown document formats
- Make routing decisions — it produces classification maps, others act on them

## Direction

- Expand hook coverage to intercept other token-expensive operations (e.g., large Glob results, multi-file Grep outputs) as additional opportunities to redirect work to Codex
- Tighten classification quality feedback: surface confidence distributions and domain mismatch rates as observable metrics, feeding the Demarch measurement loop
- Generalize `extract_sections` to support configurable heading depth and structured non-markdown text, as the flux-drive review system expands beyond markdown documents
