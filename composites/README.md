# composites — placeholder

Eight high-leverage tools from `~/Projects/miro-mcp-server/` that need to be absorbed into this repo before public reveal. They have no Miro REST equivalent so the printing-press generator cannot emit them.

This directory is a placeholder. When absorbed, each composite lands as a hand-authored Cobra subcommand under `internal/cli/` (where the generator's `--force` semantics will preserve it on regen). After all 8 are absorbed, this directory can be deleted or kept as documentation.

## The 8 composites

| Composite | Source path | Notes |
|---|---|---|
| `boards stickies create-grid` | `~/Projects/miro-mcp-server/miro/stickygrid.go:13-80` | 2D grid walk, batch POST per cell, concurrency cap. ~80 LOC. |
| `boards generate-diagram` | `~/Projects/miro-mcp-server/miro/diagrams.go` + `~/Projects/miro-mcp-server/miro/diagrams/` | Mermaid parser to shapes + connectors. ~300 LOC, the largest. |
| `boards summary` | `~/Projects/miro-mcp-server/miro/boards_summary.go:15-54` | List items, group by type, return counts + 5 most recent. ~40 LOC. |
| `boards content` | `~/Projects/miro-mcp-server/miro/boards_summary.go:59-80` | Cursor-walk wrapper, paginated dump for AI. ~25 LOC. |
| `boards search` | `~/Projects/miro-mcp-server/miro/search.go:18-80` | List + client-side text filter. ~65 LOC. |
| `boards items bulk-update` | `~/Projects/miro-mcp-server/miro/bulk.go` (partial) | Concurrency-bounded PATCH fan-out. ~150 LOC. |
| `boards items bulk-delete` | `~/Projects/miro-mcp-server/miro/bulk.go` (partial) | Concurrency-bounded DELETE fan-out. ~150 LOC. |
| `boards desire-paths` | `~/Projects/miro-mcp-server/miro/desirepath/` | Custom analytic. Smallest scope-creep risk; can ship without it for v1. |

## Per-composite checklist when absorbing

For each one:

1. Read the source in `miro-mcp-server/miro/<composite>.go` to understand the logic
2. Create `internal/cli/boards_<resource>_<verb>.go` (or appropriate path) as a hand-authored Cobra subcommand
3. Use the generated client in `internal/client/` (don't import `miro-mcp-server`'s client; use the in-repo one for consistency)
4. Add `cmd.Annotations["mcp:read-only"] = "true"` for read-only composites; omit for mutating ones
5. Add to the parent command's command tree (probably already wired by the runtime walker, but verify)
6. Test live against the AnalyticsDev Demo board
7. Run `./scripts/regenerate.sh` to confirm the file is preserved by the generator's `--force` flag
8. Move on to the next one

## Bug fix during absorption

`boards items frame` 404 → empty list (4-line patch on the generated handler per the original handoff). Trivial. See `internal/cli/boards_items_frame.go` after first regen; the fix is in the response-handling block.

## Why these can't be auto-generated

Each one combines logic that doesn't map cleanly to a single REST endpoint:

- `create-grid` does N parallel POSTs with computed positions
- `generate-diagram` parses Mermaid syntax client-side before issuing POSTs
- `summary` and `content` aggregate / paginate
- `bulk-update` and `bulk-delete` fan out concurrently with rate-limiter cooperation
- `search` does a list + client-side filter (Miro's API has no search endpoint for board items)
- `desire-paths` is a custom analytic over board state

These are the kind of agent-shaped tools that API vendors typically don't ship natively. Keeping them in this repo (rather than upstream in Miro's API) is the right layering.
