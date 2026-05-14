# Miro Developer Platform CLI

<img src="https://content.pstmn.io/47449ea6-0ef7-4af2-bac1-e58a70e61c58/aW1hZ2UucG5n" width="1685" height="593">

`miro-cli` is a single binary that wraps the Miro REST API as shell commands.
One verb per endpoint, JSON in and out, plus a local SQLite mirror of your
boards so you can search and query offline.

## Why use it

- **Shell-first.** Every endpoint is a flag-driven command. No hand-rolled
  JSON envelopes, no pagination boilerplate, no curl + jq pipelines.
- **Agent-friendly.** `--json`, `--dry-run`, `--select`, and `--agent` are
  built in so the output is predictable for scripts and LLMs.
- **Offline workflows.** `miro-cli sync` mirrors boards and items into a
  local SQLite store; `miro-cli query` runs SQL and FTS5 search against it
  without spending API quota.
- **Bulk verbs.** `items bulk-create`, `items bulk-update`,
  `items bulk-delete`, and `stickies create-grid` collapse N HTTP calls
  into one for workshop and migration flows.
- **Safe defaults.** Destructive verbs refuse to run without `--yes` (or
  `--agent`, which implies it). `--idempotent` makes create/delete retries
  safe.

## When to reach for what

| You want to... | Use |
| --- | --- |
| Script Miro from bash, CI, or a Makefile | `miro-cli` (this repo) |
| Drive Miro from Claude Code as a skill | `miro-cli` + the bundled `SKILL.md` |
| Drive Miro from a Claude Desktop / MCP-compatible agent | [miro-mcp-server](https://github.com/olgasafonova/miro-mcp-server) |
| Embed Miro into a TypeScript or Python app | The official Miro SDKs |
| Issue a one-off API call to test a payload | `miro-cli <verb> --dry-run`, or `curl` |

The CLI and the MCP server are complements, not alternatives: same author,
overlapping coverage, different runtimes. Use both if you want.

## Install

### From source

```bash
go install miro-cli/cmd/miro-cli@latest
```

This drops a `miro-cli` binary in `$GOPATH/bin` (typically `~/go/bin`). Make
sure that directory is on your `PATH`.

### Pre-built binary

Download the appropriate archive for your platform from the
[latest release](https://github.com/olga-safonova/miro-cli/releases/latest)
and extract the `miro-cli` binary into a directory on your `PATH`. On macOS,
clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine miro-cli`.
On Unix, mark it executable: `chmod +x miro-cli`.

### Homebrew

```bash
brew install olga-safonova/tap/miro-cli
```

## Quick Start

### 1. Set your access token

Get an access token from the
[Miro developer portal](https://miro.com/app/settings/user-profile/apps) and
export it:

```bash
export MIRO_ACCESS_TOKEN="your-token-here"
```

Or pass `--token` on every invocation.

### 2. Try your first command

```bash
miro-cli boards list
miro-cli boards get --board-id <id>
miro-cli stickies create --board-id <id> --content "Hello, Miro" --color yellow
```

Run `miro-cli --help` for the full command reference, or
`miro-cli <group> --help` for the verbs under any group.

## Commands

Twenty-two resource groups today. The tables below summarize each group at
a glance; run `miro-cli <group> --help` for the full verb list and flags.

### Board management

| Group | Purpose | Key verbs |
| --- | --- | --- |
| `boards` | Board lifecycle plus a few search / content helpers | `list`, `get`, `create`, `update`, `delete`, `copy`, `find`, `search`, `content`, `summary`, `picture`, `share`, `diagram`, `audit` |
| `members` | Board access control | `list`, `get`, `update`, `remove` |

### Item CRUD (one group per item type)

Each group below ships `create` / `get` / `update` / `delete` against its
own Miro REST resource. The table only calls out non-standard verbs.

| Group | Resource path | Extras |
| --- | --- | --- |
| `stickies` | `/v2/boards/{id}/sticky_notes` | `create-grid` (bulk row-major layout) |
| `shapes` | `/v2/boards/{id}/shapes` | `create-flowchart` (v2-experimental stencils) |
| `texts` | `/v2/boards/{id}/texts` | — |
| `frames` | `/v2/boards/{id}/frames` | — |
| `cards` | `/v2/boards/{id}/cards` | — |
| `app-cards` | `/v2/boards/{id}/app_cards` | — |
| `connectors` | `/v2/boards/{id}/connectors` | `list` |
| `embeds` | `/v2/boards/{id}/embeds` | — |
| `documents` | `/v2/boards/{id}/documents` and `/v2/boards/{id}/docs` | `upload` (multipart from disk), `update-from-file` (replace bytes), `create-doc` (Markdown rich-text) |
| `images` | `/v2/boards/{id}/images` | `upload`, `update-from-file` |
| `codewidgets` | `/v2-experimental/boards/{id}/code_widgets` | read-only `list` (experimental) |
| `mindmap` | `/v2-experimental/boards/{id}/mindmap_nodes` | `list` |
| `tables` | `/v2/boards/{id}/data_table_formats` | read-only `list` and `get` |

### Cross-type item operations

`items` is the catch-all for verbs that don't care about the underlying
type. Bulk verbs are here because Miro's bulk endpoint accepts a typed
array, not per-resource arrays.

| Verb | What it does |
| --- | --- |
| `items list` / `items list-all` | Cursor-paginated list, or fetch everything until the cursor exhausts |
| `items get` | Get any item by ID (returns the right shape based on `type`) |
| `items update` / `items delete` | Cross-type partial update + delete |
| `items bulk-create` | Create up to 20 items per call, mixed types |
| `items bulk-update` | PATCH many items in one logical call (serial under the hood) |
| `items bulk-delete` | DELETE many items in one logical call |
| `items get-within-frame` | List items inside a frame |
| `items get-by-tag` | List items carrying a tag |
| `items get-tags` / `attach-tag` / `detach-tag` | Tag membership for a single item |

### Tags and grouping

| Group | Purpose | Verbs |
| --- | --- | --- |
| `tags` | Tag definitions on a board | `list`, `get`, `create`, `update`, `delete` |
| `groups` | Group items so they move/resize as one | `list`, `get`, `get-items`, `create`, `update` (full PUT), `delete` (ungroup, items survive) |

### Local store (offline workflow)

| Command | What it does |
| --- | --- |
| `sync` | Mirror boards + items into a local SQLite store (incremental by default) |
| `query` | Run a read-only SQL or FTS5 query against the store |

See the [Local Store](#local-store) section below for the full workflow.

### Enterprise / audit

| Group | Purpose | Notes |
| --- | --- | --- |
| `audit` | Enterprise audit log (`list-logs`) | Last 90 days; CSV export for older data |
| `exports` | Org-scoped board export jobs (eDiscovery) | `create-job`, `list-job-tasks`, `get-job-status`, `get-job-results`, `get-task-link`, `update-job` |

Both require Enterprise-plan tokens with the matching scopes.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
miro-cli boards get

# JSON for scripting and agents
miro-cli boards get --json

# Filter to specific fields
miro-cli boards get --json --select id,name,status

# Dry run, show the request without sending
miro-cli boards get --dry-run

# Agent mode, JSON + compact + no prompts in one flag
miro-cli boards get --agent
```

## Local Store

`miro-cli` ships a local SQLite mirror of your boards and items so you can
run ad-hoc SQL and full-text search without burning API calls or waiting
for network round-trips. The default store path is
`$XDG_DATA_HOME/miro-cli/store.db` (or `~/.local/share/miro-cli/store.db`);
override it for any command with `--store-path`.

### `miro-cli sync`, populate the store

```bash
# Incremental: re-fetches a board's items only when modifiedAt has advanced
# past the stored watermark. First run does a full sweep.
miro-cli sync

# Force a full re-fetch (after a schema change, or when you suspect drift)
miro-cli sync --full

# Pin the watermark for one run, useful when a previous sync was interrupted
miro-cli sync --since 2026-05-14T00:00:00Z
```

Conflict policy is last-write-wins: the API is the source of truth, and
rows are upserted by id. The watermark is stamped at the start of a
successful run, so a mid-run failure leaves the previous watermark in
place and the next run resumes.

### `miro-cli query`, SQL passthrough

```bash
# Read-only SELECT against the store. Output is JSON when piped,
# tab-separated table when stdout is a terminal.
miro-cli query "SELECT id, name FROM boards ORDER BY modified_at DESC LIMIT 10"

# Items on a board, by type
miro-cli query "SELECT type, COUNT(*) FROM items WHERE board_id = 'b1' GROUP BY type"
```

The connection is opened with `mode=ro` and `PRAGMA query_only=ON`;
non-SELECT input is rejected before execution. `--limit` (default `1000`)
caps the rows returned per query, pass `--limit 0` to disable the cap.

### Full-text search

An `items_fts` FTS5 virtual table is kept in lockstep with `items` via
triggers. It indexes the textual fields Miro models as `data.content`,
`data.title`, and `data.description`: sticky notes, cards, text widgets,
shapes, app cards, and frame titles.

```bash
# Find every item mentioning "roadmap" across all synced boards
miro-cli query "SELECT item_id, board_id, item_type FROM items_fts WHERE items_fts MATCH 'roadmap'"

# Phrase match, adjacent tokens in any indexed field
miro-cli query 'SELECT item_id FROM items_fts WHERE items_fts MATCH "Q3 review"'

# Join back to items for richer columns
miro-cli query "SELECT i.id, i.type, i.position_x, i.position_y
  FROM items_fts f JOIN items i ON i.id = f.item_id
  WHERE items_fts MATCH 'design'"
```

The tokenizer is `unicode61` (FTS5's default) and does not stem, so `fox`
will not match `foxes`. Use the `*` suffix for prefix matching: `MATCH 'fox*'`.

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive**, never prompts, every input is a flag
- **Pipeable**, `--json` output to stdout, errors to stderr
- **Filterable**, `--select id,name` returns only fields you need
- **Previewable**, `--dry-run` shows the request without sending
- **Explicit retries**, add `--idempotent` to create retries and
  `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable**, `--yes` for explicit confirmation of destructive actions
- **Piped input**, write commands can accept structured input when their
  help lists `--stdin`
- **Offline-friendly**, `miro-cli sync` mirrors boards + items locally;
  `miro-cli query` runs SQL and FTS5 search against the mirror without
  network round-trips
- **Agent-safe by default**, no colors or formatting unless
  `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error,
`5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

This repo ships a `SKILL.md` that drives the CLI from Claude Code. Install
the binary first (see [Install](#install) above), then either drop
`SKILL.md` into your project's `.claude/skills/miro-cli/` directory or load
it via your skills-installer of choice. The skill verifies the binary is on
`$PATH` before running any command.

For Miro MCP tools (board operations, stickies, frames, diagrams), use the
separate [miro-mcp-server](https://github.com/olgasafonova/miro-mcp-server)
project instead. This CLI is shell + skill only.

## Health Check

```bash
miro-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/miro-cli/config.toml`

Static request headers can be configured under `headers`; per-command
header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `MIRO_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting

**Authentication errors (exit code 4)**

- Run `miro-cli doctor` to check credentials
- Verify the environment variable is set: `echo $MIRO_ACCESS_TOKEN`

**Not found errors (exit code 3)**

- Check the resource ID is correct
- Run the `list` command to see available items

## Further reading

Miro Developer Platform background, in case this is your first time touching
the REST API:

- [Platform introduction](https://beta.developers.miro.com/docs/introduction)
- [REST API quickstart (video)](https://beta.developers.miro.com/docs/try-out-the-rest-api-in-less-than-3-minutes)
- [REST API quickstart (article)](https://beta.developers.miro.com/docs/build-your-first-hello-world-app-1)
- [Getting started with OAuth 2.0](https://beta.developers.miro.com/docs/getting-started-with-oauth)
- [Miro App Examples](https://github.com/miroapp/app-examples)
