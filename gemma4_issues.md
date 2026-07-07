# Gemma 4 Tool Calling in Ollama — Issue Analysis

## Official Gemma 4 Tool Format (Google spec)

Source: https://ai.google.dev/gemma/docs/core/prompt-formatting-gemma4

### Control tokens

| Token Pair | Purpose |
|---|---|
| `<\|tool>` `<tool\|>` | Defines a tool |
| `<\|tool_call>` `<tool_call\|>` | Model's request to use a tool |
| `<\|tool_response>` `<tool_response\|>` | Tool execution result |
| `<\|\"\|>` | String value delimiter (single token) |

### Tool declaration format

```
<|tool><name>{description:<|"|>desc<|"|>,parameters:{properties:{...}}}<tool|>
```

### Tool call output format

```
<|tool_call>call:<name>{key:value,key:value}<tool_call|>
```

Arguments are **NOT JSON** — they use Gemma's native notation:

| JSON Schema type | Gemma 4 format | Example |
|---|---|---|
| string | `key:<\|"\|>value<\|"\|>` | `name:<\|"\|>Paris<\|"\|>` |
| number (integer) | `key:<value>` | `count:42` |
| number (float) | `key:<value>` | `price:3.14` |
| boolean | `key:true` / `key:false` | `active:true` |
| object | `key:{nested:value}` | `address:{city:<\|"\|>Paris<\|"\|>}` |
| array | `[item1,item2]` | `[<\|"\|>a<\|"\|>,<\|"\|>b<\|"\|>]` |

Numbers are rendered raw (no delimiters). Integers and floats use Go's `%d` / `%v` formatting.
Range constraints (`minimum`, `maximum`, `exclusiveMinimum`, `exclusiveMaximum`, `multipleOf`)
are **not supported** by the Gemma 4 template or renderer.

**Type annotation in tool declarations:** The renderer emits `type:<|"|>STRING<|"|>`,
`type:<|"|>NUMBER<|"|>`, `type:<|"|>INTEGER<|"|>`, `type:<|"|>BOOLEAN<|"|>`,
`type:<|"|>OBJECT<|"|>`, `type:<|"|>ARRAY<|"|>` inside property definitions.

### Key constraint (from emansom / community analysis)

Google's official examples show Gemma 4 expecting **flat tool arguments** — no deeply nested structures
(arrays of objects, complex nested dicts). Complex schemas confuse the model, especially smaller variants.

References:
- https://github.com/google-gemma/cookbook/blob/main/docs/capabilities/text/function-calling-gemma4.ipynb
- https://ai.google.dev/gemma/docs/core/prompt-formatting-gemma4

---

## Renderer vs Parser: Two Distinct Code Paths

Ollama handles Gemma 4 tools through two separate components:

### Renderer (`model/renderers/gemma4.go` + Jinja2 template)

Converts a standard JSON tool schema into Gemma 4's native `<|tool>...<tool|>` format.
Runs before the model sees the prompt. Written both as Go code and as a Jinja2 template
(`gemma4_e2b_chat_template.jinja2`).

### Parser (`model/parsers/gemma4.go`)

Parses the model's `<|tool_call>call:...<tool_call|>` output back into structured
`api.ToolCall` objects. Runs after the model generates text.

---

## Renderer Issues (Schema → Gemma format conversion)

### R1: `$defs`/`$ref` not resolved (our bug)

When `pydantic_function_tool` with `strict=True` generates a schema, nested types use
`$defs` + `$ref`:

```json
{
  "$defs": {"Idea": {"properties": {"content": {...}, "variation_of": {...}}}},
  "properties": {"ideas": {"items": {"$ref": "#/$defs/Idea"}}}
}
```

`ToolFunctionParameters.Defs` (line 489 in `api/types.go`) captures `$defs`, but the
Gemma 4 renderer's `renderToolDeclaration()` never reads it. The model receives
`$ref: "#/$defs/Idea"` instead of the actual properties — so it hallucinates parameter
names (e.g. `title`, `logline`, `genre` for a "story ideas" tool).

**No dedicated issue or PR yet.** The Nemotron renderer has explicit `<$defs>` handling
(`model/renderers/nemotron3nano.go:336`), but Gemma 4 does not.

### R2: Reserved JSON Schema keys stripped as parameter names — PR #15703 (OPEN)

Reported in issue #15670. The renderer function `isSchemaStandardKey()` (line 606 of
`gemma4.go`) strips these keys from parameter definitions:
`description`, `type`, `properties`, `required`, `nullable`.

PR #15703 (https://github.com/ollama/ollama/pull/15703) by mverrilli fixes this:
- Renames parameter keys that conflict with JSON Schema reserved words instead of
  stripping them
- `description` is the most commonly affected field name

**Status:** Open. Not merged as of July 2026.

### R3: Flat vs nested tool schema handling

Google's official spec and cookbook show Gemma 4 with flat tool arguments. Complex
nested schemas (arrays of objects, $ref resolution) are documented pain points.
The official Jinja2 template suggests Gemma 4 was designed for simple key-value tool args.

**No dedicated PR.** May require renderer-side flattening or schema simplification for
optimal results.

---

## Parser Issues (Model output → ToolCall conversion)

The parsing pipeline has three layers, applied in order of increasing forgiveness:

1. **Strict parser** (always runs first) — regex-based extraction of `<|"|>` delimiters
   then `json.Unmarshal`
2. **Repair fallback** (PR #15374) — if strict parser fails, tries heuristic repairs
3. **Warning log** — if both fail, logs a warning and skips the tool call

### Current state in upstream `main`

These PRs are already merged into `main`:
- **#15254** — Fix `<|"|>` → `"` conversion when arg values contain `"` characters
- **#15306** — Rework parser with regex + reference-style JSON conversion (supersedes #15254)
- **#15374** — Add repair fallback pipeline (missing delimiters, single quotes, raw strings)

### Open issues (not fully fixed)

| Issue | Symptom | Root cause | Status |
|---|---|---|---|
| #15315 | Nested `<\|"\|>` delimiters, backticks, single quotes in args | Repair doesn't cover all edge cases | Open |
| #15445 | Unicode/special chars (Chinese, `sed` commands) | Same as #15315 | Open |
| #15539 | `system prompt + think:false + tools` → tool calls leak to `content` | Incidental fix, no regression test | Closed |

### Merged PR details

#### PR #15254 (merged, superseded by #15306)

- **Fix:** Replace naive `strings.ReplaceAll` with a state machine that tracks
  `<|"|>` string boundaries
- **Bug:** `git commit -m "message"` inside a tool argument produced invalid JSON
  because the unescaped `"` broke parsing
- **Scope:** Parser only (no renderer changes)
- **Test coverage:** Added cases for quotes within quoted strings

#### PR #15306 — Rework gemma4 tool call handling (merged)

- **Fix:** Replace the hand-rolled `gemma4ArgsToJSON` state machine with:
  1. Regex `(?s)<\|"\|>(.*?)<\|"\|>` to extract Gemma-quoted strings
  2. Replace them with JSON-style `"..."` strings
  3. `json.Unmarshal` the result
- **Why better:** The reference-style conversion is more robust because it delegates
  structural parsing to the standard JSON parser
- **Scope:** Parser only
- **Lines:** −121 / +20 (simpler than old code)

#### PR #15374 — Add gemma4 tool call repair (merged)

- **Fix:** When `json.Unmarshal` fails, try heuristic repairs:
  1. Missing `<|"|>` delimiters (bare string values)
  2. Single-quoted string values
  3. Dangling Gemma delimiter
  4. Raw terminal string values (when schema says it should be a string)
  5. Missing object close (`}`)
- **Scope:** Parser only (stores tool schemas to use during repair)
- **Lines:** +812 / −9
- **Key insight from emansom (community):** The repair approach is a "hack" —
  the root cause may be that Gemma 4 is not designed for complex nested tool schemas,
  and the proper fix is to simplify schemas before rendering

---

## Test Cases Needed

Based on the official spec and all reported bugs, a comprehensive test suite should cover:

### Renderer tests (schema → Gemma format)

| # | Test case | What it covers |
|---|---|---|
| TR1 | Flat tool: single string arg | `call:get_weather{location:<|\|"|\|>Paris<|\|"|\|>}` |
| TR2 | Quoted strings in arg values | Git commit messages with `"` |
| TR3 | Unicode/special chars in args | Chinese text, `sed` expressions |
| TR4 | `$defs`/`$ref` resolution | Nested type definitions (our bug) |
| TR5 | `description` as parameter name | Regression for PR #15703 |
| TR6 | Nested object args (array of objects) | Schema with `items.properties` |
| TR7 | `type` as parameter name | Regression for PR #15703 |
| TR8 | `nullable`, `required` as parameter names | Regression for PR #15703 |
| TR9 | System prompt + tools + `think:false` | Renderer must not drop tool defs |

### Parser tests (model output → ToolCall)

| # | Test case | What it covers |
|---|---|---|
| TP1 | Well-formed flat call | `call:get_weather{loc:<|\|"|\|>Paris<|\|"|\|>}` |
| TP2 | Quoted strings inside args | `call:exec{cmd:<|\|"|\|>git commit -m "msg"<|\|"|\|>}` |
| TP3 | Missing string delimiters | `call:foo{name:barevalue}` |
| TP4 | Single-quoted values | `call:foo{name:'value'}` |
| TP5 | Missing object close | `call:foo{name:<|\|"|\|>val<|\|"|\|>` (no `}`) |
| TP6 | Dangling delimiter | `call:foo{name:<|\|"|\|>val}` |
| TP7 | Nested nested delimiters in args | `call:exec{cmd:<|\|"|\|>sed 's/foo/bar/g'<|\|"|\|>}` |
| TP8 | Unicode in arg values | `call:write{content:<|\|"|\|>中文<|\|"|\|>}` |
| TP9 | Backticks in arg values | `` call:exec{cmd:<|\|"|\|>`echo hello`<|\|"|\|>} `` |
| TP10 | System prompt + `think:false` + tools | Regression for #15539 |

---

## Priority Patches for `blep_stable`

Ordered by impact:

1. **`$defs`/`$ref` resolution in renderer** (our bug, no PR yet)
   - Resolve `$ref` against `$defs` in `renderToolDeclaration()` before rendering
   - Follow the same pattern as the Nemotron renderer's `<$defs>` block

2. **PR #15703** — Allow reserved JSON Schema keys as parameter names
   - Cherry-pick from mverrilli's branch

3. **PR #15374** — Add repair fallback (already in upstream `main` if we rebase on latest)
