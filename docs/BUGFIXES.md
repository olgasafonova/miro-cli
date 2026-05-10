# Generator bugs fixed in `cli-printing-press`

Five bugs surfaced during the Miro curated-run pilot (10-05-2026) and were fixed in the `mvanhorn/cli-printing-press` generator. All fixes are committed there; the current `miro-cli` artifact is built from generator commit `b858a57` (see `scripts/printing-press-version.txt`).

These are upstream generator bugs — they affect every printed CLI, not just Miro. Documenting them here for the Miro conversation: each bug surfaces a class of broken generation that real OpenAPI specs trigger, and the fixes generalize.

## Bug #1 — Positional args index off-by-N

**Where:** `internal/generator/templates/command_endpoint.go.tmpl` (template), `internal/generator/generator.go` (helper)

**Symptom:** When an OpenAPI spec declared filter flags (`limit`, `type`, `role`, etc.) before a positional path param in the params slice, the generator emitted `args[N]` where `N` was the global index in the params slice. The correct subscript is the positional-only index. Live failure: 11 read commands across `boards`/`orgs`/`v2-experimental` — including `boards items get` (args[3]) and an `orgs members enterprise-get-organization` shape (args[6] when six enum-validating flags preceded the path param). Every invocation failed with `"<param> is required"`.

**Fix:** Added `positionalParams` template helper that filters `Endpoint.Params` to positional only; ranged over `positionalParams` instead of `Endpoint.Params` with the global index. Two regression tests cover single-positional-after-flags and multi-positional-with-flag-interleaving cases.

**Commit:** `2884cae` in `cli-printing-press` (combined with bug #5).

## Bug #5 — `--stdin` rejected top-level JSON arrays

**Where:** `internal/generator/templates/command_endpoint.go.tmpl` (POST/PUT/PATCH branches)

**Symptom:** The stdin parser declared `var jsonBody map[string]any`, so endpoints whose `requestBody.schema` was `type: array` (e.g. Miro `POST /v2/boards/{id}/items/bulk`) rejected `[{...}, {...}]` payloads at parse time with `"parsing stdin JSON: cannot unmarshal array into Go value of type map[string]interface{}"`. Compounding factor: `mapRequestBody` in the OpenAPI parser drops array-shaped bodies entirely (it only walks `properties`), so `--stdin` is the ONLY path to send a body for such endpoints — making the parser bug a hard wall.

**Fix:** Changed `var body map[string]any` → `var body any` in POST/PUT/PATCH branches; unmarshal stdin directly into `&body` so any valid JSON value is accepted (object, array, scalar). Routed flag-built bodies through a typed `bodyFields := map[string]any{}` local that's then assigned to `body`, preserving the typed-field path. Parameterized the `bodyMap` helper with a `mapVar` argument.

**Commit:** `2884cae` in `cli-printing-press` (combined with bug #1).

## Bug #3 — `in:query` params dropped on POST/PUT/PATCH

**Where:** `internal/generator/templates/client.go.tmpl` (new client methods), `internal/generator/templates/command_endpoint.go.tmpl` (dispatch logic)

**Symptom:** When an OpenAPI spec declared parameters with `in: query` alongside a request body on a write endpoint, the generator emitted the flag and even validated `required: true`, but the request-build phase dropped the value. Live failure: `boards items attach-tag-to <board> <item> --tag-id <tag>` accepted `--tag-id`, error-checked it as required, then sent `POST /v2/boards/{id}/items/{id}` with no `?tag_id=...` and got HTTP 400 from Miro.

**Fix:** Added six client methods in `client.go.tmpl`: `PostWithParams` / `PutWithParams` / `PatchWithParams` plus their `*AndHeaders` variants. Each delegates to the existing `c.do(METHOD, path, params, body, headers)` — `c.do` always supported query params for every verb; there were just no public entry points. Added `endpointHasQueryOrHeaderParams` template helper to detect when an endpoint needs the `WithParams` dispatch.

**Tangent captured (NOT fixed in same commit):** The MCP-surface templates have the same shape bug — `mcp_tools.go.tmpl:244-249` actively misroutes query-param-shaped agent args INTO `bodyArgs` for POST/PUT/PATCH; `mcp_intents.go.tmpl` and `mcp_code_orch.go.tmpl` drop the query map similarly. The new `*WithParams` client methods are the right primitive to patch all three. Tracked in `HANDOFF.md` Phase 6 follow-ups.

**Commit:** `dc6b5f4` in `cli-printing-press`.

## Bug #4 — GET handlers showed "0 items" for single-key array wrappers

**Where:** `internal/generator/templates/helpers.go.tmpl` (new helper), `internal/generator/templates/command_endpoint.go.tmpl` (apply at top of GET output block)

**Symptom:** Many REST APIs return list resources wrapped in a single-property object — `{tags: [...]}`, `{items: [...]}`, `{results: [...]}` — but the spec parser only special-cases the `{data: [...]}` shape. For every other wrapper key, the generated GET handler unmarshalled the response as `[]json.RawMessage` to count items, the unmarshal silently failed (object → array), and `printAutoTable` + `printProvenance` treated the response as zero items. Live failure: `boards items get-tags-from <board> <item>` returned `{"tags":[{...}]}` from the API; the user saw "0 items" on stderr and an empty table on stdout, even when the item genuinely had tags attached.

**Fix:** Added `unwrapListResponse` runtime helper. Conservative: fires only when the response is a JSON object with exactly one property whose value is a JSON array. Multi-key wrappers (paginated lists with `cursor`/`total` fields) intentionally pass through unchanged so consumers can still read pagination metadata. Applied at the top of the GET output-handling block before count/render.

**Behavior change for downstream consumers:** printed CLIs whose target API returns single-key array wrappers on GET will now expose the inner array as the response body. JSON consumers piping the output need to drop the `.tags`/`.items` jq path. Multi-key wrappers are unchanged.

**Commit:** `b858a57` in `cli-printing-press`.

## Bug #2 — Spec, not generator

**Where:** `~/printing-press/specs/miro-spec-curated.json` (now in this repo at `specs/miro-spec-curated.json`)

**Symptom:** `POST /v2/boards/{board_id}/groups` body schema referenced `#/components/schemas/Group` (SCIM user-group shape) instead of the actual board-item-group shape Miro expects (`{data: {items: [string]}}`). Generated CLI emitted `--id`, `--name`, `--type`, `--description` flags (SCIM fields); Miro's API rejected with HTTP 400 because none of those fields are accepted.

**Diagnosis:** The Miro public OpenAPI spec is incorrect for this endpoint pair. `Group` is a SCIM/user-group schema (id, name, type, description) and the spec wrongly references it from a board-item-group endpoint. There is no parser bug; the parser correctly resolves the only `Group` schema in the spec.

**Fix layer:** SPEC, not generator. See `docs/SPEC-PATCHES.md` for the patch and remaining tangents (other board-group endpoints share the same wrong `$ref`).

**Commit:** none in `cli-printing-press` (correctly nothing to fix there); spec patch lives in this repo at `specs/miro-spec-curated.json`.

## Why these matter for the Miro conversation

Each of these is evidence that:

1. The published Miro OpenAPI spec has accuracy issues that would break any auto-generated client (bug #2 specifically; bug #5's array-body and bug #3's query-param patterns also depend on the spec being correctly written).
2. Common generator-side issues exist in the broader OpenAPI tooling ecosystem (bugs #1, #3, #4, #5 are not Miro-specific — they affect every spec with the same shape patterns).
3. Building a usable CLI from the public Miro spec is non-trivial work that someone has to do; the printing-press output + the curation work in this repo demonstrate one way to do it well.
