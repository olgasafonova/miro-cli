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
   go install miro-cli/cmd/miro-cli@latest
   ```
2. Verify: `miro-cli --help`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

Alternatively, install via Homebrew (`brew install olga-safonova/tap/miro-cli`) or download a pre-built binary from the [latest release](https://github.com/olga-safonova/miro-cli/releases/latest).

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

**audit** — Manage audit

- `miro-cli audit` — Retrieves a page of audit events from the last 90 days. If you want to retrieve data that is older than 90 days, you...

**boards** — Manage boards

- `miro-cli boards copy` — Creates a copy of an existing board. You can also update the name, description, sharing policy, and permissions...
- `miro-cli boards create` — Creates a board with the specified name and sharing policies.<br/><h4>Note</h4> You can only create up to 3 team...
- `miro-cli boards delete` — Deletes a board. Deleted boards go to Trash (on paid plans) and can be restored via UI within 90 days after...
- `miro-cli boards get` — Retrieves a list of boards accessible to the user associated with the provided access token. This endpoint supports...
- `miro-cli boards get-specific` — Retrieves information about a board.<br/><h3>Required scope</h3> <a target=_blank...
- `miro-cli boards update` — Updates a specific board.<br/><h3>Required scope</h3> <a target=_blank...

**groups** — Manage groups

- `miro-cli groups get` — Retrieves a single Group resource.<br><b> Note</b>: Along with groups (teams), the users that are part of those...
- `miro-cli groups list` — Retrieves the list of groups (teams) in the organization.<br><br> Note: Along with groups (teams), the users that...
- `miro-cli groups patch` — Updates an existing group resource, i.e. a team, overwriting values for specified attributes. Patch operation for...

**oauth** — Manage oauth

- `miro-cli oauth revoke-token` — <p><b>Please use the new revoke endpoint <code>/v2/oauth/revoke</code>. This endpoint is considered vulnerable and...
- `miro-cli oauth revoke-token-v2` — Revoke the current access token. Revoking an access token means that the access token will no longer work. When an...

**oauth-token** — Manage oauth token

- `miro-cli oauth-token` — Get information about an access token, such as the token type, scopes, team, user, token creation date and time, and...

**orgs** — Manage orgs

- `miro-cli orgs <org_id>` — Retrieves organization information.<br/><h3>Required scope</h3> <a target=_blank...

**resource-types** — Manage resource types

- `miro-cli resource-types get` — Retrieve metadata for the available resource types (User and Group) that are supported.
- `miro-cli resource-types list` — Retrieve information about which SCIM resources are supported. <br><br> Currently, Miro supports Users and Groups as...

**schemas** — Manage schemas

- `miro-cli schemas get` — Retrieve information about how users, groups, and enterprise-user attributes URIs that are formatted.
- `miro-cli schemas list` — Retrieve metadata about Users, Groups, and extension attributes that are currently supported.

**service-provider-config** — Manage service provider config

- `miro-cli service-provider-config` — Retrieve supported operations and SCIM API basic configuration.

**sessions** — Manage sessions

- `miro-cli sessions` — Reset all sessions of a user. Admins can now take immediate action to restrict user access to company data in case...

**users** — Manage users

- `miro-cli users create` — Creates a new user in the organization. <br><br> <br>Note</b>: All newly provisioned users are added to the default...
- `miro-cli users delete` — Deletes a single user from the organization.<br><br> Note: A user who is the last admin in the team or the last...
- `miro-cli users get` — Retrieves a single user resource. <br><b> <br>Note</b>: Returns only users that are members in the organization. It...
- `miro-cli users list` — Retrieves the list of users in your organization. <br><b> <br>Note</b>: The API returns users that are members in...
- `miro-cli users patch` — Updates an existing user resource, overwriting values for specified attributes. Attributes that are not provided...
- `miro-cli users replace` — Updates an existing user resource. This is the easiest way to replace user information. <br><br> If the user is...

**v2-experimental** — Manage v2 experimental

- `miro-cli v2-experimental create-code-widget-item` — Adds a code widget item to a board.<br/><h3>Required scope</h3> <a target=_blank...
- `miro-cli v2-experimental create-mindmap-nodes-experimental` — Adds a mind map node to a board. A root node is the starting point of a mind map. A node that is created under a...
- `miro-cli v2-experimental create-shape-item-flowchart` — Adds a flowchart shape item to a board.<br/><h3>Required scope</h3> <a target=_blank...
- `miro-cli v2-experimental delete-code-widget-item` — Deletes a code widget item from the board.<br/><h3>Required scope</h3> <a target=_blank...
- `miro-cli v2-experimental delete-item-experimental` — Deletes an item from a board.<br/><h3>Required scope</h3> <a target=_blank...
- `miro-cli v2-experimental delete-mindmap-node-experimental` — Deletes a mind map node item and its child nodes from the board.<br/><h3>Required scope</h3> <a target=_blank...
- `miro-cli v2-experimental delete-shape-item-flowchart` — Deletes a flowchart shape item from the board.<br/><h3>Required scope</h3> <a target=_blank...
- `miro-cli v2-experimental get-code-widget-item` — Retrieves information for a specific code widget item on a board.<br/><h3>Required scope</h3> <a target=_blank...
- `miro-cli v2-experimental get-code-widget-items` — Retrieves a list of code widget items for a specific board. This method returns results using a cursor-based...
- `miro-cli v2-experimental get-items-experimental` — Retrieves a list of items for a specific board. You can retrieve all items on the board, a list of child items...
- `miro-cli v2-experimental get-metrics` — Returns a list of usage metrics for a specific app for a given time range, grouped by requested time period. This...
- `miro-cli v2-experimental get-metrics-total` — Returns total usage metrics for a specific app since the app was created. This endpoint requires an app management...
- `miro-cli v2-experimental get-mindmap-node-experimental` — Retrieves information for a specific mind map node on a board.<br/><h3>Required scope</h3> <a target=_blank...
- `miro-cli v2-experimental get-mindmap-nodes-experimental` — Retrieves a list of mind map nodes for a specific board. This method returns results using a cursor-based approach....
- `miro-cli v2-experimental get-shape-item-flowchart` — Retrieves information for a specific shape item on a board.<br/><h3>Required scope</h3> <a target=_blank...
- `miro-cli v2-experimental get-specific-item-experimental` — Retrieves information for a specific item on a board.<br/><h3>Required scope</h3> <a target=_blank...
- `miro-cli v2-experimental move-code-widget-item` — Updates the position of a code widget item on a board.<br/><h3>Required scope</h3> <a target=_blank...
- `miro-cli v2-experimental update-code-widget-item` — Updates a code widget item on a board based on the data properties provided in the request body.<br/><h3>Required...
- `miro-cli v2-experimental update-shape-item-flowchart` — Updates a flowchart shape item on a board based on the data and style properties provided in the request...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
miro-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Store your access token:

```bash
miro-cli auth set-token YOUR_TOKEN_HERE
```

Or set `MIRO_ACCESS_TOKEN` as an environment variable.

Run `miro-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  miro-cli boards get --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
miro-cli feedback "the --since flag is inclusive but docs say exclusive"
miro-cli feedback --stdin < notes.txt
miro-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.miro-cli/feedback.jsonl`. They are never POSTed unless `MIRO_DEVELOPER_PLATFORM_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `MIRO_DEVELOPER_PLATFORM_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
miro-cli profile save briefing --json
miro-cli --profile briefing boards get
miro-cli profile list --json
miro-cli profile show briefing
miro-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `miro-cli --help` output
2. **Starts with `install`** → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## Direct Use

1. Check if installed: `which miro-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   miro-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `miro-cli <command> --help`.
