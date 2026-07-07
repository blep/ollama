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

**Commits:**
```
0bb18b75 openai: forward num_ctx to options map in FromChatRequest and FromCompleteRequest
fd72ecba openai: add tests for num_ctx forwarding in FromChatRequest and FromCompleteRequest
```

**Status:** PR #16825 is still open upstream as of July 2026.  We cherry-picked
it into `blep_stable`.  Once merged upstream, this patch can be dropped.

---

## Patch 2: Allow reserved JSON Schema keys as parameter names (Gemma 4 renderer)

**PR:** [#15703](https://github.com/ollama/ollama/pull/15703) — *model/renderers/gemma4: allow reserved JSON Schema keys as parameter names*

**Problem:** The Gemma 4 tool renderer's `isSchemaStandardKey()` function
silently strips parameter names that conflict with JSON Schema reserved
keywords (`description`, `type`, `properties`, `required`, `nullable`).
A tool with a parameter named `description` would have it completely removed
from the schema sent to the model.

**Fix:** Remove the blanket skip in `writeSchemaProperties()`. Reserved keys
are still filtered in the nested-object fallback path where they appear as
structural schema keys rather than parameter names.

**Files changed:**
- `model/renderers/gemma4.go` — −3 lines (removed `isSchemaStandardKey` skip),
  +6 lines (filtered copy for nested fallback)
- `model/renderers/gemma4_reserved_test.go` — +49 lines (unit test)

**Commit:**
```
568a1ef4 model/renderers/gemma4: allow reserved JSON Schema keys as parameter names
cc8e79eb test: add unit test for reserved JSON Schema keys as parameter names (#15703)
```

**Status:** PR #15703 is open upstream as of July 2026.  Cherry-picked into
`blep_stable`.  Once merged upstream, this patch can be dropped.

---

## Patch 3: Resolve `$defs`/`$ref` in Gemma 4 tool renderer

**PR:** *(our own — no upstream PR yet)*

**Problem:** When `pydantic_function_tool` generates a schema with
`strict=True`, nested types use `$defs` + `$ref` instead of inline
definitions.  The Gemma 4 tool renderer emitted the raw `$ref` literal
(e.g. `items:{$ref:<|"|>#/$defs/Idea<|"|>}`) which the model cannot
resolve — causing it to hallucinate parameter names.

**Fix:** Add a recursive `resolveRefs` method that walks the schema tree
before rendering:
1. Detects `$ref` keys (e.g. `"#/$defs/Idea"`)
2. Looks up the referenced definition in `$defs`
3. Replaces the `$ref` with the inlined schema
4. Recursively resolves any nested refs

**Files changed:**
- `model/renderers/gemma4.go` — +62 lines (resolveRefs, resolveDefs methods
  + wiring into renderToolDeclaration)
- `model/renderers/gemma4_defs_test.go` — +70 lines (unit test)

**Commit:**
```
446e2c1a fix: resolve $defs/$ref in Gemma 4 renderer
```

**Status:** This is our own patch, not derived from an upstream PR.  Should
be proposed upstream once validated.

---

## Test suite

A comprehensive test suite for Gemma 4 tool calling lives at:
`/home/blep/prj/llm_pocs/pocs/gemma4_test_tool_call.py`

Validates: flat schemas, multi-types, nested objects, reserved keyword param
names, system prompt + tools, optional/nullable fields.

All 7 tests pass against `gemma-4-e2b-it:q8_0` with the patches applied.
