# Spec patches applied to `miro-spec-curated.json`

The Miro public OpenAPI spec ships with several inaccuracies that produce broken generated code. This file tracks the patches applied to the curated copy at `specs/miro-spec-curated.json` and the tangents not yet fixed.

## Applied — bug #2

### Endpoint: `POST /v2/boards/{board_id}/groups`

**Upstream issue:** Both `requestBody.schema.$ref` and `responses.201.content.application/json.schema.$ref` point to the SCIM `Group` schema (a user-group shape with `id`, `name`, `type`, `description`). The actual Miro API expects and returns a board-item-group shape: request `{data: {items: [string]}}`, response `{id, type, data: {items: [string]}, links}`.

**Patch:**
1. Added two new schemas under `components.schemas`:
   - `BoardItemGroupCreateBody` — request body shape
   - `BoardItemGroupResponse` — response 201 shape
2. Re-pointed `requestBody.schema.$ref` from `#/components/schemas/Group` to `#/components/schemas/BoardItemGroupCreateBody`
3. Re-pointed `responses.201.content.application/json.schema.$ref` from `#/components/schemas/GroupResponseShort` to `#/components/schemas/BoardItemGroupResponse`

**Verified live:** `boards groups create uXjVG34x8Cg= --data '{"items":[id1,id2]}'` returns HTTP 201 with the correct response shape. Test group cleaned up afterward.

## Tangents — same class of bug, NOT yet patched

These remain broken in the current spec. Tracked in `HANDOFF.md` Phase 1.

### `GET /v2/boards/{board_id}/groups` (get-all)

`responses.200.content.application/json.schema.$ref` likely points to a paginated wrapper around SCIM `Group`. The actual Miro response is paginated board-item-groups. Need to verify shape with a live call (use the AnalyticsDev Demo board which has at least one item-group from earlier testing) and patch accordingly.

### `GET /v2/boards/{board_id}/groups/{group_id}` (get-by-id)

`responses.200.content.application/json.schema.$ref` → `GroupResponseShort` (SCIM-shaped). Same fix as the create endpoint's response: re-point to `BoardItemGroupResponse`.

### `PATCH /v2/boards/{board_id}/groups/{group_id}` (update)

Both `requestBody` and `responses.200` likely reference the wrong schemas. Same patch shape as create: re-point to `BoardItemGroupCreateBody` (request) and `BoardItemGroupResponse` (response).

### `GET /v2/boards/{board_id}/groups/items` (get-items-by-id)

Path is suspicious — it doesn't include `{group_id}` in the path but the operation suggests it should be group-specific. May be a Miro spec bug, may be a curated-spec typo. Investigate before patching.

### `DELETE /v2/boards/{board_id}/groups/{group_id}?` (line 12232)

Path has a trailing `?` which is invalid OpenAPI syntax. Almost certainly a typo in the upstream Miro spec. Strip the `?`.

## Patch convention

Each patch lives directly in `specs/miro-spec-curated.json` (no separate patch files). The spec is the source of truth; `spec.json` at the repo root is regenerated from it by `scripts/regenerate.sh`.

When adding a new patch:
1. Find the broken `$ref` in `specs/miro-spec-curated.json`
2. Either re-point to an existing correct schema OR add a new schema under `components.schemas` with a `description` that explains why it's needed (referencing this file is helpful)
3. Validate JSON syntax: `python3 -m json.tool specs/miro-spec-curated.json > /dev/null`
4. Regenerate: `./scripts/regenerate.sh`
5. Verify with a live API call against the AnalyticsDev Demo board (`uXjVG34x8Cg=`)
6. Add a section here documenting what changed and why
7. Commit

## Reporting upstream

When the patches stabilize, consider opening issues against Miro's public OpenAPI spec repo (or via their developer-relations contact, especially if cooperation with Miro firms up — see `HANDOFF.md`). Each patch in this file is a candidate upstream-fix proposal.

Doing so would mean future printing-press regenerations don't need the curation step. Until then, the patched spec lives here.
