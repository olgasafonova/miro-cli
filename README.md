# miro-cli

A private working repo for the printing-press-generated Miro CLI and embedded MCP server. Built from the patched `miro-spec-curated.json` to demonstrate what's possible — not yet ready for public release.

## What's in here

Two binaries, both built from one OpenAPI spec:

| Binary | Surface | Purpose |
|---|---|---|
| `miro-developer-platform-pp-cli` | Cobra CLI | Human-facing subcommands across all Miro REST endpoints |
| `miro-developer-platform-pp-mcp` | MCP server (stdio + http transports) | Agent-facing 2-tool dispatcher (`_search` + `_execute`) covering 197 endpoints |

Both share the same generated client code (auth, rate limiting, retry, dry-run), and the MCP server walks the Cobra tree at startup to register tools.

The companion repo `~/Projects/miro-mcp-server` already ships a hand-built MCP server with 91 typed tools including 8 high-leverage composites (`create_sticky_grid`, `generate_diagram`, `get_board_summary`, `bulk_update`, etc.) that have no Miro REST equivalent. Those need to be absorbed into this repo before public reveal — see `composites/README.md` and `HANDOFF.md`.

## Status

**Not ready for public release.** Demo / iteration phase.

What works:
- Read-side and write-side coverage for Miro REST endpoints, validated against the AnalyticsDev Demo board
- Five generator bug fixes upstreamed to `cli-printing-press` (see `docs/BUGFIXES.md`)
- One spec patch applied for `POST /v2/boards/{board_id}/groups` (see `docs/SPEC-PATCHES.md`)
- 16/16 golden tests + zero lint issues in the generator that produced this artifact

What's missing for public reveal:
- Three more spec patches needed for board-group endpoints whose responses still reference the wrong SCIM-shape schema (`get-all`, `get-by-id`, `update`, `get-items-by-id`)
- The 8 composites from `miro-mcp-server` are not yet absorbed
- README + announcement story not yet written for an outside audience
- No CI configured in this repo (the curated artifact's `.golangci.yml` and `.goreleaser.yaml` are present but not wired to GitHub Actions)

See `HANDOFF.md` for the full roadmap.

## Repo layout

```
miro-cli/
├── README.md              # this file
├── HANDOFF.md             # what to work on next, in priority order
├── cmd/                   # generator output: CLI + MCP entry points
├── internal/              # generator output: handlers, client, helpers
├── specs/
│   └── miro-spec-curated.json    # source of truth for the spec (with patches)
├── spec.json              # embedded copy of the spec, synced at generation time
├── composites/
│   └── README.md          # placeholder; the 8 hand-built tools to absorb
├── docs/
│   ├── BUGFIXES.md        # five generator bugs fixed in cli-printing-press
│   ├── SPEC-PATCHES.md    # spec-curation patches; tracks remaining tangents
│   └── GENERATED-README.md # the printing-press-generated README, preserved
├── scripts/
│   ├── regenerate.sh              # re-run the generator against the curated spec
│   └── printing-press-version.txt # generator commit hash this artifact was built from
├── Makefile               # generator output: build/test/lint
├── go.mod / go.sum
├── manifest.json          # generator output: tools manifest
├── AGENTS.md              # generator output: conventions for the printed CLI
├── SKILL.md               # generator output: skill definition
├── .golangci.yml
├── .goreleaser.yaml
└── .printing-press.json   # provenance metadata
```

## Regenerating

The generator that produced this is `~/Projects/cli-printing-press/`. Pinned commit: see `scripts/printing-press-version.txt`.

```bash
./scripts/regenerate.sh
```

The script warns if the local printing-press repo is at a different commit than the pinned version. After regen, the `internal/cli/*.go` files written by hand (composites, once absorbed) are preserved per the generator's `--force` semantics; everything else is overwritten.

## Build

```bash
make build
# binaries land in build/stage/bin/
```

## Run

```bash
export MIRO_ACCESS_TOKEN=...
./build/stage/bin/miro-developer-platform-pp-cli boards items get <board-id>
./build/stage/bin/miro-developer-platform-pp-mcp -transport stdio
```

## Why this exists

Built to demonstrate what a printing-press-generated CLI + MCP can look like for an API the size of Miro's. Two reveal paths preserved:

1. **Merge into `miro-mcp-server`** when ready — adds the printing-press output alongside the existing 91 hand-built tools. Mirrors the `mediawiki-mcp-server` + `wiki` CLI dual-path precedent.
2. **Flip this repo public** as a standalone tool — gives the work its own identity, lets Miro absorb it cleanly if cooperation goes that direction.

Decision deferred until the work in `HANDOFF.md` lands and the conversation with Miro firms up.
