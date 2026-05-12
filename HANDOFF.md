# HANDOFF — work to make this demo-ready

Roadmap from current state (initial demo build, 10-05-2026) to "ready to reveal" (either as a public repo flip or as a merge into `miro-mcp-server`). Items are roughly ordered by dependency + value.

## Phase 6 complete (12-05-2026): printing-press infrastructure deleted

Closed `miro-cli-rpk`. The new hand-authored CLI now stands alone:

- Deleted: `cmd/miro-developer-platform-pp-cli/`, `internal/{cli,store,types,client,config,cache,cliutil}/`, `composites/`, `specs/`, `scripts/regenerate.sh`, `scripts/printing-press-version.txt`, `.printing-press.json`, `docs/SPEC-PATCHES.md`, `docs/BUGFIXES.md`, `docs/GENERATED-README.md`.
- Renamed: `cmd/miro/` → `cmd/miro-cli/` so `go install` produces a `miro-cli` binary by default.
- Updated: `.goreleaser.yaml` (id/main/binary + ldflags), `Makefile` (build target + race-failfast tests), `.gitignore`, `README.md` (install + quick start rewritten — printing-press npx flow gone), `SKILL.md` (prereqs section rewritten + frontmatter renamed `pp-miro-developer-platform` → `miro-cli`).

The new CLI surface (~127 verbs across 19 resource trees) is the entire repo now. Phases 4 (perf), 5 (security + CI), and 3a-remainder (Mermaid diagram port) are the remaining open beads.

## Scope pivot (12-05-2026): MCP wrapper dropped from this repo

The printing-press-generated MCP surface (`cmd/miro-developer-platform-pp-mcp/`, `internal/mcp/`, `manifest.json`) was removed. Rationale:

- The `miro-mcp-server` repo already ships 91 hand-curated tools with workshop/retro semantics; shipping a second Miro MCP from this repo created user confusion ("which do I install?").
- This repo's MCP surface was generator-default, not deliberately designed — the README already de-emphasized it as "advanced".
- The unique MCP value here (local SQL/FTS over synced data via `search`/`sql`/`context` tools) is real but belongs as features inside `miro-mcp-server`, not as a parallel server.

This repo is now CLI + skill only. `scripts/regenerate.sh` strips `cmd/miro-developer-platform-pp-mcp/`, `internal/mcp/`, and `manifest.json` after each generator run, so the MCP wrapper stays gone without any upstream printing-press change. No fork, no PR, no dependency on `cli-printing-press` adding an `Enabled` toggle.

## Scope pivot 2 (12-05-2026): off printing-press, hand-authored CLI

User direction: stop being a printing-press downstream. The generated 262-file CLI surface gets replaced with hand-authored Cobra commands that mirror `miro-mcp-server`'s 91 tools as CLI verbs. The deslop scan made it concrete — the file-health score is gated entirely on generator output (`internal/store/store.go` 3,340 LOC, `internal/types/types.go` 2,633 LOC, etc.), and adding tests where it matters means the code being tested has to survive regen, which generated files don't.

### Target architecture

```
cmd/
  miro/main.go                 # NEW entry point, replaces miro-cli
internal/
  miro/                        # NEW foundation: HTTP client, auth, config, errors, ratelimit, redact, cache
    client.go
    config.go
    auth.go
    errors.go
    ratelimit.go
    redact.go
    cache.go
  tools/                       # NEW per-resource subcommands; one subdir per category
    boards/                    # boards CRUD + special verbs (find, search, share, copy, content, summary, picture, audit, diagram)
    items/                     # generic items + bulk_create/bulk_update/bulk_delete
    stickies/  shapes/  texts/  cards/  connectors/  frames/  images/  docs/  embeds/  app_cards/
    tags/      groups/  mindmap/  tables/  exports/  members/  misc/
```

Old `internal/cli/` (262 generated files), `internal/store/`, `internal/types/`, `internal/client/`, `internal/config/`, `internal/cache/`, `internal/cliutil/`, `cmd/miro-cli/` get deleted as the new code reaches feature parity, not before. While migrating, both binaries coexist; once `cmd/miro/` covers everything the old binary did, the old tree is removed in one commit and `goreleaser`, `Makefile`, `README`, `SKILL.md`, `.printing-press.json`, `scripts/regenerate.sh`, `specs/`, `docs/SPEC-PATCHES.md`, `docs/BUGFIXES.md`, `docs/GENERATED-README.md` all go with it.

### Phasing

| Phase | Bead | Scope |
|---|---|---|
| 1 | `miro-cli-fnd` | Foundation packages (`internal/miro/`) — client, config, auth, errors, ratelimit, redact, cache — with tests. Done iteratively; first slice in this session. |
| 2 | `miro-cli-cmd` | New entry point `cmd/miro/main.go` + Cobra root + global flags (--token, --json, --dry-run, --agent, --yes, --idempotent). Reference tool: `miro list-boards`. |
| 3a-3j | `miro-cli-boards`, `miro-cli-items`, `miro-cli-stickies`, `miro-cli-typed-items`, `miro-cli-tags`, `miro-cli-groups`, `miro-cli-mindmap`, `miro-cli-tables`, `miro-cli-exports`, `miro-cli-misc` | One bead per resource category. Each ports the 4-12 tools in that category as hand-authored subcommands with table-driven tests. Spawnable in parallel waves of 3-4 once Phase 2 lands. |
| 4 | `miro-cli-perf` | Performance pass: connection pooling, response caching for read-heavy endpoints, bounded-concurrency bulk-op fan-out, optional local SQLite sync (port miro-cli's old `store.go` if useful). |
| 5 | `miro-cli-sec` | Security pass: token redaction in logs/errors, input validation, share-board allowlist (per `code-review-prompts.md` HG-3), destructive-op confirmation gating, panic recovery, `go mod verify` + `govulncheck` + `gosec` in CI per `rules/mcp-server-patterns.md`. |
| 6 | `miro-cli-clean` | Delete printing-press infrastructure: `cmd/miro-cli/`, `internal/{cli,store,types,client,config,cache,cliutil}/`, `scripts/regenerate.sh`, `scripts/printing-press-version.txt`, `specs/`, `.printing-press.json`, `docs/{SPEC-PATCHES,BUGFIXES,GENERATED-README}.md`, `composites/` (its work is now Phase 3), `manifest.json` references in README. Update `.goreleaser.yaml`, `Makefile`, `README.md`, `SKILL.md`, `HANDOFF.md`. |

### Tool count by category (from `miro-mcp-server/tools/definitions.go`)

| Category | Tools | Examples |
|---|---|---|
| boards | 12 | list/get/create/copy/update/delete/find/search/share/get_content/get_summary/get_picture/get_audit_log/generate_diagram |
| typed items | 40+ | sticky/shape/text/card/app_card/connector/frame/image/doc/embed × {create,get,update,delete} |
| items (generic + bulk) | 9 | list_all/list/get/update/delete + bulk_create/bulk_update/bulk_delete + get_items_by_tag |
| tags | 8 | list/create/get/update/delete + attach/detach + get_item_tags |
| groups | 6 | list/create/get/update/delete/get_group_items |
| board_members | 4 | list/get/update/remove |
| mindmap | 4 | list/create/get/delete |
| exports | 3 | create_export_job/get_export_job_status/get_export_job_results |
| tables | 2 | list/get |
| misc | 3 | get_desire_paths/get_audit_log/(generate_diagram is under boards) |

### Properties the CLI must hold

- **Fast.** HTTP keep-alive, connection pool, response caching with TTL for read-heavy GETs, bounded concurrency for bulk ops (default 8, configurable).
- **Comprehensive.** Coverage parity with `miro-mcp-server` (91 verbs).
- **Secure.** Tokens never on argv, never in logs, never in errors. Redaction in all output paths. `gosec` + `govulncheck` + `go mod verify` in CI. Destructive ops gated by `--yes` or explicit confirmation. Share-board (and any "grant access to third party" op) gated by allowlist. Inputs validated at the boundary. Panic recovery at handler entry.

### Phases 2-6 from the original HANDOFF (pre-pivot) are SUPERSEDED

The original Phases 1-6 below were planned around the generated CLI surface. They are kept for reference but no longer reflect the active plan; the table above is the active roadmap.

Phase 3 (client-pattern backport into `miro-mcp-server`) — see bead `claude-code-config-27e` — is unchanged. Independent of this repo's pivot.

## Phase 1 — Finish the spec patches (mostly done)

Bug #2 (the SCIM-vs-board-item-group schema confusion) was patched only for `POST /v2/boards/{board_id}/groups`. Three more endpoints in the same family had the same broken refs.

**Done (10-05-2026 follow-up):** ref repointings applied for the get-all, get-by-id, and update endpoints. Plus the trailing `?` path typo was resolved by dropping the redundant `deleteGroup` operation from the spec entirely (the merged `unGroup` covers both behaviors via `delete_items`). After regen, `boards groups delete` no longer exists; use `boards groups un --delete-items`. Details in `docs/SPEC-PATCHES.md`.

Remaining for Phase 1:

| Item | Status | Notes |
|---|---|---|
| `GET /v2/boards/{board_id}/groups/items` (get-items-by-id) | verified — no patch needed (10-05-2026) | Live response shape matches the spec's inline definition: `{size, limit, data: {id, data: [Item]}}`. Outer `data` is a single group object; inner `data` is the items array. The unusual path (no `{group_id}`, query param `group_item_id`) is genuinely how the Miro API works. Two minor cosmetic discrepancies noted in `docs/SPEC-PATCHES.md` but neither is blocking. |
| Live-call verification of the four patched endpoints | done (10-05-2026) | All four return shapes that match `BoardItemGroupResponse`. PUT update returns a new `id` per spec. The `delete` alias on the merged `un` command routes through to `DELETE /v2/boards/{board_id}/groups/{group_id}`. Test board (`uXjVG34x8Cg=`) returned to its pre-test 1-group state. See `docs/SPEC-PATCHES.md` for invocation examples. |

## Phase 1 incident (10-05-2026): regenerate.sh `--force` is destructive

`./scripts/regenerate.sh` ran with `--force` and wiped:
- `.git/`
- `composites/`, `docs/`, `scripts/`, `specs/` (the curated spec with my unpushed patches!)
- `HANDOFF.md`
- The 4 hand-authored cohesion-split helpers (`filter_fields.go`, `pagination.go`, `render_csv.go`, `render_table.go`) and the split `helpers.go`

The HANDOFF previously claimed `--force` preserves hand-authored files. That is wrong; the printing-press generator's `--force` resets the output directory before writing. Recovery required re-cloning from `origin/main`, copying back regen output, and re-applying spec patches.

**Mitigations to add before next regen:**

- The script should `git stash --include-untracked` the entire repo before running, so `git stash pop` recovers everything if something goes wrong.
- Or: refuse to run if the working tree contains uncommitted changes.
- Or: refuse to run if `.git/` is inside the output directory and the generator's behavior may include `rm -rf $output`.
- File an upstream issue against `cli-printing-press` for the destructive `--force` semantics; it should at minimum honor `.git/` as never-clobbered.

## Phase 2 — Absorb the 8 composites (1-2 days)

The 8 hand-built tools in `~/Projects/miro-mcp-server/miro/` need to be moved into this repo's `internal/cli/` as hand-authored Cobra subcommands. They have no Miro REST equivalent so the generator can't emit them.

| Composite | Source | Sketch | Effort |
|---|---|---|---|
| `boards stickies create-grid` | `miro/stickygrid.go:13-80` | 2D grid walk, batch POST per cell, concurrency cap | ~80 LOC |
| `boards generate-diagram` | `miro/diagrams.go` + `miro/diagrams/` | Mermaid parser → shapes + connectors | ~300 LOC, largest |
| `boards summary` | `miro/boards_summary.go:15-54` | List items, group by type, return counts + 5 most recent | ~40 LOC |
| `boards content` | `miro/boards_summary.go:59-80` | Cursor-walk wrapper, paginated dump for AI | ~25 LOC |
| `boards search` | `miro/search.go:18-80` | List + client-side text filter | ~65 LOC |
| `boards items bulk-update` | `miro/bulk.go` (partial) | Concurrency-bounded PATCH fan-out | ~150 LOC |
| `boards items bulk-delete` | `miro/bulk.go` (partial) | Concurrency-bounded DELETE fan-out | ~150 LOC |
| `boards desire-paths` | `miro/desirepath/` | Custom analytic — smallest scope-creep risk; can defer for v1 | TBD |

Each lands as a hand-authored file under `internal/cli/`. The generator's `--force` semantics preserve hand-authored `internal/cli/*.go` files on regen, so they survive future regenerations.

The MCP-side registration step that used to live here is gone (see Scope pivot above). When each composite is ready, the equivalent MCP tool needs to land in `miro-mcp-server/tools/definitions.go` and `handlers.go` as a hand-authored entry — track that as a sibling task in `miro-mcp-server`, not here.

**Bug fix during absorption:** `boards items frame` 404 → empty list (4-line patch on the generated handler per the original handoff). Trivial.

## Phase 3 — Backport generated client patterns into `miro-mcp-server` (deferred)

If the eventual reveal path is "merge into miro-mcp-server" (recommended), then the 5 generated client patterns need to land in the existing repo first OR the merge needs to bring the printing-press client in to replace the hand-built one.

The five patterns:
1. OAuth refresh
2. Ceiling-discovery rate limiter
3. APIError truncation
4. sanitizeJSONResponse
5. `--dry-run` token masking

Tracked as bead `bead-27e` in your portfolio (per the original handoff). May not be needed if the merge brings the generated client wholesale.

## Phase 4 — Wire CI (half day)

The curated artifact ships `.golangci.yml` and `.goreleaser.yaml` but no `.github/workflows/`. Before public reveal:

- Add `ci.yml` with `go build`, `go test`, `golangci-lint run`, `govulncheck`
- Add `release.yml` if shipping binaries (use the existing `.goreleaser.yaml`)
- Decide on supply-chain CI per `rules/mcp-server-patterns.md` (`go mod verify`, `go mod tidy` drift check, `gosec`)

## Phase 5 — README + reveal story (half day)

The current `README.md` is internal-facing ("private working repo, demo iteration phase"). For reveal, either:

1. Rewrite as a public-facing README (positioning, install instructions, usage examples, comparison to alternatives)
2. Or write a separate announcement post (LinkedIn, Substack) that explains what it is and why

Don't delete the current README until reveal is confirmed; the internal framing is useful for the iteration phase.

## Phase 6 — Reveal decision

Two paths from `README.md`:

1. **Merge into `miro-mcp-server`.** Move `cmd/miro-cli/` into the existing public repo as a new sibling binary. Existing 91 hand-built tools coexist on the MCP side. One repo, one auth, one release. (The MCP-wrapper half is no longer in this repo — see Scope pivot — so the merge is CLI-only.)
2. **Flip this repo public.** Rename to `miro-cli` or `miro-toolkit` (final name TBD), `gh repo edit --visibility public`. Standalone identity. Easier for Miro to absorb upstream if cooperation goes that direction.

Defer until Phases 1-5 are done and the Miro conversation has firmed up.

## Open questions for the Miro conversation

- What's the status/timeline of Miro's own CLI work?
- Would they want this absorbed upstream, or kept as a community/third-party tool?
- If absorbed: license / attribution / repo transfer logistics
- If third-party: do they want to mark it as "Miro-blessed" or stay arms-length?

## Bead-shaped follow-ups (the MCP-surface tangent from bug #3)

While fixing bug #3 (lift `in:query` params into POST/PUT/PATCH requests), I discovered the same bug in three MCP-surface templates in `cli-printing-press`:

- `mcp_tools.go.tmpl:244-249` actively misroutes query-param-shaped agent args INTO `bodyArgs` for POST/PUT/PATCH (worse than the CLI bug, which silently dropped them)
- `mcp_intents.go.tmpl:174-179` drops the query map for the same verbs
- `mcp_code_orch.go.tmpl:254-259` same pattern

The new `*WithParams` client methods I added in `cli-printing-press` commit `dc6b5f4` are the right primitive to fix all three. Open follow-up commit on the `cli-printing-press` repo. Worth doing before the public reveal of `miro-cli` so the printed MCP surface for Miro is fully correct.
