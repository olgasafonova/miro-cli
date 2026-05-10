# HANDOFF — work to make this demo-ready

Roadmap from current state (initial demo build, 10-05-2026) to "ready to reveal" (either as a public repo flip or as a merge into `miro-mcp-server`). Items are roughly ordered by dependency + value.

## Phase 1 — Finish the spec patches (mostly done)

Bug #2 (the SCIM-vs-board-item-group schema confusion) was patched only for `POST /v2/boards/{board_id}/groups`. Three more endpoints in the same family had the same broken refs.

**Done (10-05-2026 follow-up):** ref repointings applied for the get-all, get-by-id, and update endpoints. Plus the trailing `?` path typo was resolved by dropping the redundant `deleteGroup` operation from the spec entirely (the merged `unGroup` covers both behaviors via `delete_items`). After regen, `boards groups delete` no longer exists; use `boards groups un --delete-items`. Details in `docs/SPEC-PATCHES.md`.

Remaining for Phase 1:

| Item | Status | Notes |
|---|---|---|
| `GET /v2/boards/{board_id}/groups/items` (get-items-by-id) | not patched | Inline shape with double-`data` envelope is suspicious but uses the right item schema (`ItemPagedResponse`). Path is also unusual. Verify with a live call before patching. See `docs/SPEC-PATCHES.md`. |
| Live-call verification of the four patched endpoints | pending | The patches are mechanical, JSON-valid, and the regen output reflects them. Run `--json` calls against AnalyticsDev Demo board (`uXjVG34x8Cg=`) for: `boards groups get-all`, `boards groups get-by-id`, `boards groups update`, `boards groups un --delete-items`. |

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

Also needed for each composite: register as an MCP tool in `internal/mcp/cobratree/` so the agent surface gets them too. The runtime walker mirrors the Cobra tree at server start; adding the Cobra command should make it auto-register, but verify per `composites/README.md` checklist.

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

1. **Merge into `miro-mcp-server`.** Move `cmd/miro-developer-platform-pp-cli/` and `cmd/miro-developer-platform-pp-mcp/` into the existing public repo as new sibling binaries. Existing 91 hand-built tools coexist. One repo, one auth, one release.
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
