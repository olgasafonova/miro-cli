---
name: miro-cli
description: "Hand-authored Cobra CLI for the Miro REST API."
author: "Olga Safonova"
license: "Apache-2.0"
argument-hint: "<command> [args]"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - miro-cli
---

# Miro Developer Platform — miro-cli

## Prerequisites: Install the CLI

This skill drives the `miro-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install from source:
   ```bash
   go install github.com/olgasafonova/miro-cli/cmd/miro-cli@latest
   ```
2. Verify: `miro-cli --help` (and `miro-cli --version` to record the build; a source/`go install` build reports `dev`, a release reports its tag).
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

Alternatively, install via Homebrew (`brew install olgasafonova/tap/miro-cli`) or download a pre-built binary from the [latest release](https://github.com/olgasafonova/miro-cli/releases/latest).

If `--help` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

<img src="https://content.pstmn.io/47449ea6-0ef7-4af2-bac1-e58a70e61c58/aW1hZ2UucG5n" width="1685" height="593">

### Miro Developer Platform concepts

- New to the Miro Developer Platform? Interested in learning more about platform concepts??
[Read our introduction page](https://beta.developers.miro.com/docs/introduction) and familiarize yourself with the Miro Developer Platform capabilities in a few minutes.


### Getting started with the Miro REST API

- [Quickstart (video):](https://beta.developers.miro.com/docs/try-out-the-rest-api-in-less-than-3-minutes) try the REST API in less than 3 minutes.
- [Quickstart (article):](https://beta.developers.miro.com/docs/build-your-first-hello-world-app-1) get started and try the REST API in less than 3 minutes.


### Miro REST API tutorials

Check out our how-to articles with step-by-step instructions and code examples so you can:

- [Get started with OAuth 2.0 and Miro](https://beta.developers.miro.com/docs/getting-started-with-oauth)


### Miro App Examples

Clone our [Miro App Examples repository](https://github.com/miroapp/app-examples) to get inspiration, customize, and explore apps built on top of Miro's Developer Platform 2.0.

## Command Reference

`miro-cli` is organized into resource groups. Each group is a subcommand tree;
run `miro-cli <group> --help` for the exact verbs and flags of that group, and
`miro-cli <group> <verb> --help` for a single command. The groups are:

| Group | What it covers |
|-------|----------------|
| `boards` | Create, copy, get, update, delete boards; also `boards diagram` (render a sequence/flowchart from text) |
| `items` | Generic board items: get, list, update, delete, and bulk update/delete via `--ids-file` / `--patches-file` (pass `-` to read the JSON payload from stdin, e.g. `... \| miro items bulk-delete --ids-file -`) |
| `stickies` | Sticky notes |
| `shapes` | Shapes |
| `texts` | Text items |
| `cards` | Card items |
| `appcards` | App card items |
| `frames` | Frames |
| `images` | Image items |
| `embeds` | Embedded URLs |
| `documents` | Document items |
| `connectors` | Connectors (lines/arrows between items) |
| `tags` | Tags and tag assignment |
| `groups` | Item groups |
| `mindmap` | Mind-map nodes |
| `codewidgets` | Code widget items |
| `members` | Board members and sharing roles |
| `boards share` | Invite members to a board (gated by the share allowlist — see Security) |
| `exports` | Board export jobs |
| `audit` | Organization audit events (last 90 days; Enterprise) |
| `sync` | Mirror boards/items into a local SQLite store for offline use |
| `query` | Run read-only SQL / FTS5 search against the local store built by `sync` |

The authoritative command list is the binary itself. When unsure whether a verb
exists, run `miro-cli <group> --help` rather than guessing.

### Finding the right command

- Reading/searching offline or repeatedly → `miro-cli sync` once, then
  `miro-cli query "<SQL>"` (FTS5 full-text search is available).
- One-off live read → the matching resource group's `get`/`list` verb.
- Mutating a board → the resource group's `create`/`update`/`delete` verb;
  destructive verbs require `--yes` (or `--agent`).

## Auth Setup

`miro-cli` authenticates with a Miro access token, resolved in this order:

1. `--token <value>` flag
2. `$MIRO_ACCESS_TOKEN` environment variable

If neither is set, commands that hit the API exit with a config error (exit
code 10). There is no interactive login.

Check the setup directly with `miro-cli auth status`:

```bash
miro-cli auth status --json           # {token_present, source, verified, status}, no network
miro-cli auth status --verify --json  # also confirms the token works right now
```

Exit codes: `0` token present (and valid, with `--verify`), `10` no token,
`4` token rejected. Under `--verify`, `status` reports `ok`,
`invalid_or_expired`, or `insufficient_scope` so you can tell a bad token from
a scope problem. Prefer this over probing with a read command.

## Agent Mode and agent-facing flags

These global flags are available on every command:

- `--agent` — agent mode. Expands to `--json` and `--yes` (forces JSON output
  and skips destructive-operation confirmations). Nothing else.
- `--json` — force JSON output (the default when stdout is piped).
- `--dry-run` — print the request the command would send, then exit without
  calling the API.
- `--yes` — skip confirmation prompts on destructive operations.
- `--idempotent` — treat already-exists as success on create, and already-gone
  as success on delete.
- `--select <fields>` — comma-separated list of **top-level** field names to keep
  in JSON output. It filters the top-level object; when the output is a JSON
  array, the same top-level filter is applied to each element. It does **not**
  descend into nested objects via dotted paths.
- `--rate-limit`, `--concurrency`, `--cache-ttl`, `--no-cache`, `--store-path` —
  tuning for throughput, the GET response cache, and the local store location.

Example:

```bash
miro-cli boards get --agent --select id,name
```

### Output shape

Commands print the API result as JSON directly to stdout — there is no `meta`/
`results` envelope to unwrap; parse the value itself. When stdout is a terminal,
a short human-readable summary may be written to **stderr**, so piped and
`--agent` consumers receive clean JSON on stdout.

## Security: board sharing is allowlist-gated

`miro-cli boards share` grants a third party access to a board, so it is gated
by a fail-closed allowlist. With no allowlist configured, every share attempt is
refused.

- `MIRO_SHARE_ALLOWED_DOMAINS` — comma-separated list of recipient email domains
  permitted to receive invitations.
- `MIRO_SHARE_ALLOWED_EMAILS` — comma-separated list of exact addresses. When set
  (even to an empty value), it is authoritative: only the listed addresses pass
  and the domain allowlist is ignored entirely. This is a strict tightening, not
  a widening — a permitted domain cannot rescue an unlisted address.

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error |
| 3 | Not found (HTTP 404) |
| 4 | Auth error (HTTP 401 / 403) |
| 5 | Other API error |
| 7 | Rate limited (HTTP 429) |
| 10 | Config error (e.g. missing token) |
