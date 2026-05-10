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

## Applied — board-item-group ref repointings (Phase 1 follow-up)

Three more endpoints in the same family had the same SCIM-vs-board-group schema confusion as bug #2. All four `GroupResponseShort` references that affected board-item-group endpoints, plus the one stray `Group` reference in the `PUT` body, have been re-pointed at the correct schemas.

### `GET /v2/boards/{board_id}/groups` (get-all)

The 200 response embeds an inline paginated wrapper. Its `data.items.$ref` was `GroupResponseShort`. Re-pointed to `BoardItemGroupResponse`.

### `GET /v2/boards/{board_id}/groups/{group_id}` (get-by-id)

`responses.200.content.application/json.schema.$ref` was `GroupResponseShort`. Re-pointed to `BoardItemGroupResponse`.

### `PUT /v2/boards/{board_id}/groups/{group_id}` (update)

Note: this endpoint is `PUT`, not `PATCH` as HANDOFF originally listed. Two refs fixed:
- `requestBody.content.application/json.schema.$ref`: `Group` → `BoardItemGroupCreateBody`.
- `responses.200.content.application/json.schema.$ref`: `GroupResponseShort` → `BoardItemGroupResponse`.

After these patches, `GroupResponseShort` has zero references in the spec. The schema definition still exists under `components.schemas` and can stay; leaving it keeps the SCIM-shape available if a real consumer surfaces. Safe to delete on a future cleanup pass.

**Verified live (10-05-2026)** against AnalyticsDev Demo board `uXjVG34x8Cg=`:

```bash
# get-all (returns BoardItemGroupResponse[])
miro-developer-platform-pp-cli boards groups get-all uXjVG34x8Cg= --json

# get-by-id (returns BoardItemGroupResponse)
miro-developer-platform-pp-cli boards groups get-by-id uXjVG34x8Cg= <group_id> --json

# create (CLI auto-wraps body in {"data": ...}, so pass the inner shape)
miro-developer-platform-pp-cli boards groups create uXjVG34x8Cg= \
  --data '{"items":["<item_id_1>","<item_id_2>"]}' --json

# update (PUT replaces the group entirely; response includes a NEW id)
miro-developer-platform-pp-cli boards groups update uXjVG34x8Cg= <group_id> \
  --data '{"items":["<item_id_3>","<item_id_4>"]}' --json

# un (HTTP 204; default keeps items on the board)
miro-developer-platform-pp-cli boards groups un uXjVG34x8Cg= <group_id>

# 'delete' alias on the merged un command — routes to the same HTTP DELETE
miro-developer-platform-pp-cli boards groups delete uXjVG34x8Cg= <group_id>
```

Two gotchas surfaced during verification, neither blocking:

- `--quiet` plus `--json` on a successful create silently suppresses the response body (and the new group's `id`). This is a CLI UX issue, not a spec issue. Use `--json` alone if you need to capture the new `id`.
- The board's `get-all` listing showed only 1 group initially despite 2 existing — the second wasn't visible on the first page. Likely a Miro indexing latency, not a spec issue. The `links.self` cursor in the response is the canonical way to paginate; CLI should consume it transparently.

## Applied — trailing `?` typo on `/v2/boards/{board_id}/groups/{group_id}?`

The Miro spec used a trailing `?` on the path key as a workaround for OpenAPI's "one operation per verb per path" rule. Two HTTP-level identical operations were split across two path entries:

- `unGroup` at `/v2/boards/{board_id}/groups/{group_id}` — `delete_items` query param `required=false`
- `deleteGroup` at `/v2/boards/{board_id}/groups/{group_id}?` — same path, `delete_items` `required=true`

**Patch:** removed the `/v2/boards/{board_id}/groups/{group_id}?` path entry entirely. `unGroup` covers both behaviors via its `delete_items` query param. After regen, `boards groups delete` no longer exists as a CLI subcommand — users perform "delete the group and its items" via `boards groups un <board_id> <group_id> --delete-items`.

## Tangents — NOT yet patched

### `GET /v2/boards/{board_id}/groups/items` (get-items-by-id)

The 200 response is an inline shape: `{limit, size, total, data: {id, type, data: [ItemPagedResponse]}}`. The double-`data` nesting is unusual but not obviously wrong; the inner array uses `ItemPagedResponse` which IS the right Miro item shape (this endpoint returns items, not groups, despite the path family). The `group_item_id` query param is required, so the operation is "given a group's item ID, return the items in that group."

Two open questions:
1. Is the double-`data` envelope what Miro really returns, or should the outer object collapse so the array sits at top-level `data`?
2. Why is the path `/v2/boards/{board_id}/groups/items` rather than `/v2/boards/{board_id}/groups/{group_id}/items`? The query-param-as-identifier shape is unusual for a REST list-children endpoint.

Verify both with a live call before patching.

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
