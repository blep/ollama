# Patches Applied to Ollama

This file tracks custom patches applied to our Ollama fork at
`git@github.com:blep/ollama.git`.

- **`blep_stable`** (this branch) — our custom branch with patches.
- **`main`** — pure clone of official `ollama/ollama:main`, no patches.

---

## Patch 1: Forward `num_ctx` in OpenAI-compatible endpoints

**PR:** [#16825](https://github.com/ollama/ollama/pull/16825) — *openai: forward num_ctx to options in OpenAI-compatible endpoints*

**Problem:** The OpenAI-compatible endpoints (`/v1/chat/completions`,
`/v1/completions`) silently ignore the `num_ctx` parameter.  The native
`/api/chat` and `/api/generate` endpoints support it via the `options` map,
but the OpenAI compat layer never forwarded it.

**Fix:** Add `NumCtx *int` to both `ChatCompletionRequest` and
`CompletionRequest` structs, and forward it to the `options` map in
`FromChatRequest()` and `FromCompleteRequest()`.

**Files changed:**
- `openai/openai.go` — +10 lines (struct fields + forward logic)
- `openai/openai_test.go` — +64 lines (tests)

**Commits in `blep_stable`:**
```
0bb18b75 openai: forward num_ctx to options map in FromChatRequest and FromCompleteRequest
fd72ecba openai: add tests for num_ctx forwarding in FromChatRequest and FromCompleteRequest
984a987b Merge branch 'fix-num-ctx'
```

**Status:** PR #16825 is still open upstream as of July 2026.  We cherry-picked
it into `blep_stable`.  Once merged upstream, this patch can be dropped.
