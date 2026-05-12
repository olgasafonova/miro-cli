# Miro Developer Platform CLI

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

## Install

The recommended path installs both the `miro-developer-platform-pp-cli` binary and the `pp-miro-developer-platform` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install miro-developer-platform
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install miro-developer-platform --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/miro-developer-platform-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-miro-developer-platform --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-miro-developer-platform --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-miro-developer-platform skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-miro-developer-platform. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your access token from your API provider's developer portal, then store it:

```bash
miro-developer-platform-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export MIRO_ACCESS_TOKEN="your-token-here"
```

### 3. Verify Setup

```bash
miro-developer-platform-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
miro-developer-platform-pp-cli boards get
```

## Usage

Run `miro-developer-platform-pp-cli --help` for the full command reference and flag list.

## Commands

### audit

Manage audit

- **`miro-developer-platform-pp-cli audit enterprise-get-logs`** - Retrieves a page of audit events from the last 90 days. If you want to retrieve data that is older than 90 days, you can use the <a target=_blank href="https://help.miro.com/hc/en-us/articles/360017571434-Audit-logs#h_01J7EY4E0F67EFTRQ7BT688HW0">CSV export feature</a>.<br/><h3>Required scope</h3> <a target=_blank href=https://developers.miro.com/reference/scopes>auditlogs:read</a> <br/><h3>Rate limiting</h3> <a target=_blank href="/reference/rate-limiting#rate-limit-tiers">Level 2</a>

### boards

Manage boards

- **`miro-developer-platform-pp-cli boards copy`** - Creates a copy of an existing board. You can also update the name, description, sharing policy, and permissions policy for the new board in the request body.<br/><h3>Required scope</h3> <a target=_blank href=https://developers.miro.com/reference/scopes>boards:write</a> <br/><h3>Rate limiting</h3> <a target=_blank href="/reference/rate-limiting#rate-limit-tiers">Level 4</a><br/>
- **`miro-developer-platform-pp-cli boards create`** - Creates a board with the specified name and sharing policies.<br/><h4>Note</h4> You can only create up to 3 team boards with the free plan.<br/><h3>Required scope</h3> <a target=_blank href=https://developers.miro.com/reference/scopes>boards:write</a> <br/><h3>Rate limiting</h3> <a target=_blank href="/reference/rate-limiting#rate-limit-tiers">Level 3</a><br/>
- **`miro-developer-platform-pp-cli boards delete`** - Deletes a board. Deleted boards go to Trash (on paid plans) and can be restored via UI within 90 days after deletion.<br/><h3>Required scope</h3> <a target=_blank href=https://developers.miro.com/reference/scopes>boards:write</a> <br/><h3>Rate limiting</h3> <a target=_blank href="/reference/rate-limiting#rate-limit-tiers">Level 3</a><br/>
- **`miro-developer-platform-pp-cli boards get`** - Retrieves a list of boards accessible to the user associated with the provided access token. This endpoint supports filtering and sorting through URL query parameters.
Customize the response by specifying `team_id`, `project_id`, or other query parameters. Filtering by `team_id` or `project_id` (or both) returns results instantly. For other filters, allow a few seconds for indexing of newly created boards.

If you're an Enterprise customer with Company Admin permissions:
- Enable **Content Admin** permissions to retrieve all boards, including private boards (those not explicitly shared with you). For details, see the [Content Admin Permissions for Company Admins](https://help.miro.com/hc/en-us/articles/360012777280-Content-Admin-permissions-for-Company-Admins).
- Note that **Private board contents remain inaccessible**. The API allows you to verify their existence but prevents viewing their contents to uphold security best practices. Unauthorized access attempts will return an error.
<h3>Required scope</h3> <a target=_blank href=https://developers.miro.com/reference/scopes>boards:read</a> <br/><h3>Rate limiting</h3> <a target=_blank href="/reference/rate-limiting#rate-limit-tiers">Level 1</a><br/>
- **`miro-developer-platform-pp-cli boards get-specific`** - Retrieves information about a board.<br/><h3>Required scope</h3> <a target=_blank href=https://developers.miro.com/reference/scopes>boards:read</a> <br/><h3>Rate limiting</h3> <a target=_blank href="/reference/rate-limiting#rate-limit-tiers">Level 1</a><br/>
- **`miro-developer-platform-pp-cli boards update`** - Updates a specific board.<br/><h3>Required scope</h3> <a target=_blank href=https://developers.miro.com/reference/scopes>boards:write</a> <br/><h3>Rate limiting</h3> <a target=_blank href="/reference/rate-limiting#rate-limit-tiers">Level 2</a><br/>

### groups

Manage groups

- **`miro-developer-platform-pp-cli groups get`** - Retrieves a single Group resource.<br><b> Note</b>: Along with groups (teams), the users that are part of those groups (teams) are also retrieved. Only users that have member role in the organization are fetched.
- **`miro-developer-platform-pp-cli groups list`** - Retrieves the list of groups (teams) in the organization.<br><br> Note: Along with groups (teams), the users that are part of those groups (teams) are also retrieved. Only users that have member role in the organization are fetched.
- **`miro-developer-platform-pp-cli groups patch`** - Updates an existing group resource, i.e. a team, overwriting values for specified attributes. Patch operation for group can be used to add, remove, or replace team members and to update the display name of the group (team). <br><br> To add a user to the group (team), use add operation. <br> To remove a user from a group (team), use remove operation. <br> To update a user resource, use the replace operation. <br> The last team admin cannot be removed from the team. <br><br> Note: Attributes that are not provided will remain unchanged. PATCH operation only updates the fields provided. <br><br> Team members removal specifics: <br> For remove or replace operations, the team member is removed from the team and from all team boards. The ownership of boards that belong to the removed team member is transferred to the oldest team member who currently has an admin role. After you remove a team member, adding the team member again to the team does not automatically restore their previous ownership of the boards. If the user is not registered fully in Miro and is not assigned to any other team, the user is also removed from the organization. <br><br> Add team members specifics: <br> All added team members are reactivated or recreated if they were deactivated or deleted earlier. <br><br> External users specifics: <br> When adding existing users with the role ORGANIZATION_EXTERNAL_USER or ORGANIZATION_TEAM_GUEST_USER to a team, we set FULL license and ORGANIZATION_INTERNAL_USER roles.

### oauth

Manage oauth

- **`miro-developer-platform-pp-cli oauth revoke-token`** - <p><b>Please use the new revoke endpoint <code>/v2/oauth/revoke</code>. This endpoint is considered vulnerable and deprecated due to access token passed publicly in the URL.</b></p> Revoke the current access token. Revoking an access token means that the access token will no longer work. When an access token is revoked, the refresh token is also revoked and no longer valid. This does not uninstall the application for the user.
- **`miro-developer-platform-pp-cli oauth revoke-token-v2`** - Revoke the current access token. Revoking an access token means that the access token will no longer work. When an access token is revoked, the refresh token is also revoked and no longer valid. This does not uninstall the application for the user.

### oauth-token

Manage oauth token

- **`miro-developer-platform-pp-cli oauth-token token-info`** - Get information about an access token, such as the token type, scopes, team, user, token creation date and time, and the user who created the token.

### orgs

Manage orgs

- **`miro-developer-platform-pp-cli orgs enterprise-get-organization`** - Retrieves organization information.<br/><h3>Required scope</h3> <a target=_blank href=https://developers.miro.com/reference/scopes>organizations:read</a> <br/><h3>Rate limiting</h3> <a target=_blank href="/reference/rate-limiting#rate-limit-tiers">Level 3</a> <br/><h3>Enterprise only</h3> <p>This API is available only for <a target=_blank href="/reference/api-reference#enterprise-plan">Enterprise plan</a> users. You can only use this endpoint if you have the role of a Company Admin. You can request temporary access to Enterprise APIs using <a target=_blank href="https://q2oeb0jrhgi.typeform.com/to/BVPTNWJ9">this form</a>.</p>

### resource-types

Manage resource types

- **`miro-developer-platform-pp-cli resource-types get`** - Retrieve metadata for the available resource types (User and Group) that are supported.
- **`miro-developer-platform-pp-cli resource-types list`** - Retrieve information about which SCIM resources are supported. <br><br> Currently, Miro supports Users and Groups as Resource Types.

### schemas

Manage schemas

- **`miro-developer-platform-pp-cli schemas get`** - Retrieve information about how users, groups, and enterprise-user attributes URIs that are formatted.
- **`miro-developer-platform-pp-cli schemas list`** - Retrieve metadata about Users, Groups, and extension attributes that are currently supported.

### service-provider-config

Manage service provider config

- **`miro-developer-platform-pp-cli service-provider-config list`** - Retrieve supported operations and SCIM API basic configuration.

### sessions

Manage sessions

- **`miro-developer-platform-pp-cli sessions enterprise-post-user-reset`** - Reset all sessions of a user.  Admins can now take immediate action to restrict user access to company data in case of security concerns. Calling this API ends all active Miro sessions across devices for a particular user, requiring the user to sign in again. This is useful in situations where a user leaves the company, their credentials are compromised, or there's suspicious activity on their account.<br/><h3>Required scope</h3> <a target=_blank href=https://developers.miro.com/reference/scopes>sessions:delete</a> <br/><h3>Rate limiting</h3> <a target=_blank href="/reference/rate-limiting#rate-limit-tiers">Level 3</a> <br/><h3>Enterprise only</h3> <p>This API is available only for <a target=_blank href="/reference/api-reference#enterprise-plan">Enterprise plan</a> users. You can only use this endpoint if you have the role of a Company Admin. You can request temporary access to Enterprise APIs using <a target=_blank href="https://q2oeb0jrhgi.typeform.com/to/BVPTNWJ9">this form</a>.</p>

### users

Manage users

- **`miro-developer-platform-pp-cli users create`** - Creates a new user in the organization. <br><br> <br>Note</b>: All newly provisioned users are added to the default team.
- **`miro-developer-platform-pp-cli users delete`** - Deletes a single user from the organization.<br><br> Note: A user who is the last admin in the team or the last admin in the organization cannot be deleted. User must be a member in the organization to be deleted. Users that have guest role in the organization cannot be deleted. <br><br> After a user is deleted, the ownership of all the boards that belong to the deleted user is transferred to the oldest team member who currently has an admin role.
- **`miro-developer-platform-pp-cli users get`** - Retrieves a single user resource. <br><b> <br>Note</b>: Returns only users that are members in the organization. It does not return users that are added in the organization as guests.
- **`miro-developer-platform-pp-cli users list`** - Retrieves the list of users in your organization. <br><b> <br>Note</b>: The API returns users that are members in the organization, it does not return users that are added in the organization as guests.
- **`miro-developer-platform-pp-cli users patch`** - Updates an existing user resource, overwriting values for specified attributes. Attributes that are not provided will remain unchanged. PATCH operation only updates the fields provided. <br><br> Note: If  the user is not a member in the organization, they cannot be updated. Additionally, users with guest role in the organization cannot be updated.
- **`miro-developer-platform-pp-cli users replace`** - Updates an existing user resource. This is the easiest way to replace user information. <br><br> If the user is deactivated, <br> userName, userType, and roles.value cannot be updated. <br> emails.value, emails.display, emails.primary get ignored and do not return any error. <br><br> Note: If the user is not a member in the organization, they cannot be updated. Additionally, users with guest role in the organization cannot be updated.

### v2-experimental

Manage v2 experimental

- **`miro-developer-platform-pp-cli v2-experimental create-code-widget-item`** - Adds a code widget item to a board.<br/><h3>Required scope</h3> <a target=_blank href=https://developers.miro.com/reference/scopes>boards:write</a> <br/><h3>Rate limiting</h3> <a target=_blank href="/reference/rate-limiting#rate-limit-tiers">Level 2</a><br/>
- **`miro-developer-platform-pp-cli v2-experimental create-mindmap-nodes-experimental`** - Adds a mind map node to a board. A root node is the starting point of a mind map. A node that is created under a root node is a child node. For information on mind maps, use cases, mind map structure, and more, see the <a href="https://developers.miro.com/docs/mind-maps" target=_blank>Mind Map Overview</a> page. <br/><h3>Required scope</h3> <a target=_blank href=https://developers.miro.com/reference/scopes>boards:write</a> <br/><h3>Rate limiting</h3> <a target=_blank href="/reference/rate-limiting#rate-limit-tiers">Level 2</a><br/><br/> <b>Known limitations on node placement: </b> Currently, the create API supports explicit positions for nodes. This means that users can only place nodes based on the x, y coordinates provided in the position parameters. If the position is not provided in the request, nodes default to coordinates x=0, y=0, effectively placing them at the center of the board. <br /><br /><b>Upcoming changes:</b> We understand the importance of flexibility in node placement. We are actively working on implementing changes to support positioning nodes relative to their parent node as well. This enhancement offers a more dynamic and intuitive mind mapping experience. <br /><br />Additionally, we are actively working on providing the update API, further enhancing the functionality of mind map APIs.
- **`miro-developer-platform-pp-cli v2-experimental create-shape-item-flowchart`** - Adds a flowchart shape item to a board.<br/><h3>Required scope</h3> <a target=_blank href=https://developers.miro.com/reference/scopes>boards:write</a> <br/><h3>Rate limiting</h3> <a target=_blank href="/reference/rate-limiting#rate-limit-tiers">Level 2</a><br/>
- **`miro-developer-platform-pp-cli v2-experimental delete-code-widget-item`** - Deletes a code widget item from the board.<br/><h3>Required scope</h3> <a target=_blank href=https://developers.miro.com/reference/scopes>boards:write</a> <br/><h3>Rate limiting</h3> <a target=_blank href="/reference/rate-limiting#rate-limit-tiers">Level 3</a><br/>
- **`miro-developer-platform-pp-cli v2-experimental delete-item-experimental`** - Deletes an item from a board.<br/><h3>Required scope</h3> <a target=_blank href=https://developers.miro.com/reference/scopes>boards:write</a> <br/><h3>Rate limiting</h3> <a target=_blank href="/reference/rate-limiting#rate-limit-tiers">Level 3</a><br/>
- **`miro-developer-platform-pp-cli v2-experimental delete-mindmap-node-experimental`** - Deletes a mind map node item and its child nodes from the board.<br/><h3>Required scope</h3> <a target=_blank href=https://developers.miro.com/reference/scopes>boards:write</a> <br/><h3>Rate limiting</h3> <a target=_blank href="/reference/rate-limiting#rate-limit-tiers">Level 3</a><br/>
- **`miro-developer-platform-pp-cli v2-experimental delete-shape-item-flowchart`** - Deletes a flowchart shape item from the board.<br/><h3>Required scope</h3> <a target=_blank href=https://developers.miro.com/reference/scopes>boards:write</a> <br/><h3>Rate limiting</h3> <a target=_blank href="/reference/rate-limiting#rate-limit-tiers">Level 3</a><br/>
- **`miro-developer-platform-pp-cli v2-experimental get-code-widget-item`** - Retrieves information for a specific code widget item on a board.<br/><h3>Required scope</h3> <a target=_blank href=https://developers.miro.com/reference/scopes>boards:read</a> <br/><h3>Rate limiting</h3> <a target=_blank href="/reference/rate-limiting#rate-limit-tiers">Level 1</a><br/>
- **`miro-developer-platform-pp-cli v2-experimental get-code-widget-items`** - Retrieves a list of code widget items for a specific board.

This method returns results using a cursor-based approach. A cursor-paginated method returns a portion of the total set of results based on the limit specified and a cursor that points to the next portion of the results. To retrieve the next portion of the collection, on your next call to the same method, set the `cursor` parameter equal to the `cursor` value you received in the response of the previous request.<br/><h3>Required scope</h3> <a target=_blank href=https://developers.miro.com/reference/scopes>boards:read</a> <br/><h3>Rate limiting</h3> <a target=_blank href="/reference/rate-limiting#rate-limit-tiers">Level 2</a><br/>
- **`miro-developer-platform-pp-cli v2-experimental get-items-experimental`** - Retrieves a list of items for a specific board. You can retrieve all items on the board, a list of child items inside a parent item, or a list of specific types of items by specifying URL query parameter values.

This method returns results using a cursor-based approach. A cursor-paginated method returns a portion of the total set of results based on the limit specified and a cursor that points to the next portion of the results. To retrieve the next portion of the collection, on your next call to the same method, set the `cursor` parameter equal to the `cursor` value you received in the response of the previous request. For example, if you set the `limit` query parameter to `10` and the board contains 20 objects, the first call will return information about the first 10 objects in the response along with a cursor parameter and value. In this example, let's say the cursor parameter value returned in the response is `foo`. If you want to retrieve the next set of objects, on your next call to the same method, set the cursor parameter value to `foo`.<br/><h3>Required scope</h3> <a target=_blank href=https://developers.miro.com/reference/scopes>boards:read</a> <br/><h3>Rate limiting</h3> <a target=_blank href="/reference/rate-limiting#rate-limit-tiers">Level 2</a><br/>
- **`miro-developer-platform-pp-cli v2-experimental get-metrics`** - Returns a list of usage metrics for a specific app for a given time range, grouped by requested time period.

This endpoint requires an app management API token. It can be generated in the <a href="https://developers.miro.com/?features=appMetricsToken#your-apps">Your Apps</a> section of Developer Hub.<br/>
<h3>Required scope</h3> <a target=_blank href=https://developers.miro.com/reference/scopes>boards:read</a><br/>
<h3>Rate limiting</h3> <a target=_blank href="/reference/rate-limiting#rate-limit-tiers">Level 1</a><br/>
- **`miro-developer-platform-pp-cli v2-experimental get-metrics-total`** - Returns total usage metrics for a specific app since the app was created.

This endpoint requires an app management API token. It can be generated in <a href="https://developers.miro.com/?features=appMetricsToken#your-apps">your apps</a> section of Developer Hub.<br/>
<h3>Required scope</h3> <a target=_blank href=https://developers.miro.com/reference/scopes>boards:read</a><br/>
<h3>Rate limiting</h3> <a target=_blank href="/reference/rate-limiting#rate-limit-tiers">Level 1</a><br/>
- **`miro-developer-platform-pp-cli v2-experimental get-mindmap-node-experimental`** - Retrieves information for a specific mind map node on a board.<br/><h3>Required scope</h3> <a target=_blank href=https://developers.miro.com/reference/scopes>boards:read</a> <br/><h3>Rate limiting</h3> <a target=_blank href="/reference/rate-limiting#rate-limit-tiers">Level 1</a><br/>
- **`miro-developer-platform-pp-cli v2-experimental get-mindmap-nodes-experimental`** - Retrieves a list of mind map nodes for a specific board.

This method returns results using a cursor-based approach. A cursor-paginated method returns a portion of the total set of results based on the limit specified and a cursor that points to the next portion of the results. To retrieve the next portion of the collection, on your next call to the same method, set the `cursor` parameter equal to the `cursor` value you received in the response of the previous request. For example, if you set the `limit` query parameter to `10` and the board contains 20 objects, the first call will return information about the first 10 objects in the response along with a cursor parameter and value. In this example, let's say the cursor parameter value returned in the response is `foo`. If you want to retrieve the next set of objects, on your next call to the same method, set the cursor parameter value to `foo`.<br/><h3>Required scope</h3> <a target=_blank href=https://developers.miro.com/reference/scopes>boards:read</a> <br/><h3>Rate limiting</h3> <a target=_blank href="/reference/rate-limiting#rate-limit-tiers">Level 2</a><br/>
- **`miro-developer-platform-pp-cli v2-experimental get-shape-item-flowchart`** - Retrieves information for a specific shape item on a board.<br/><h3>Required scope</h3> <a target=_blank href=https://developers.miro.com/reference/scopes>boards:read</a> <br/><h3>Rate limiting</h3> <a target=_blank href="/reference/rate-limiting#rate-limit-tiers">Level 1</a><br/>
- **`miro-developer-platform-pp-cli v2-experimental get-specific-item-experimental`** - Retrieves information for a specific item on a board.<br/><h3>Required scope</h3> <a target=_blank href=https://developers.miro.com/reference/scopes>boards:read</a> <br/><h3>Rate limiting</h3> <a target=_blank href="/reference/rate-limiting#rate-limit-tiers">Level 1</a><br/>
- **`miro-developer-platform-pp-cli v2-experimental move-code-widget-item`** - Updates the position of a code widget item on a board.<br/><h3>Required scope</h3> <a target=_blank href=https://developers.miro.com/reference/scopes>boards:write</a> <br/><h3>Rate limiting</h3> <a target=_blank href="/reference/rate-limiting#rate-limit-tiers">Level 2</a><br/>
- **`miro-developer-platform-pp-cli v2-experimental update-code-widget-item`** - Updates a code widget item on a board based on the data properties provided in the request body.<br/><h3>Required scope</h3> <a target=_blank href=https://developers.miro.com/reference/scopes>boards:write</a> <br/><h3>Rate limiting</h3> <a target=_blank href="/reference/rate-limiting#rate-limit-tiers">Level 2</a><br/>
- **`miro-developer-platform-pp-cli v2-experimental update-shape-item-flowchart`** - Updates a flowchart shape item on a board based on the data and style properties provided in the request body.<br/><h3>Required scope</h3> <a target=_blank href=https://developers.miro.com/reference/scopes>boards:write</a> <br/><h3>Rate limiting</h3> <a target=_blank href="/reference/rate-limiting#rate-limit-tiers">Level 2</a><br/>


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
miro-developer-platform-pp-cli boards get

# JSON for scripting and agents
miro-developer-platform-pp-cli boards get --json

# Filter to specific fields
miro-developer-platform-pp-cli boards get --json --select id,name,status

# Dry run — show the request without sending
miro-developer-platform-pp-cli boards get --dry-run

# Agent mode — JSON + compact + no prompts in one flag
miro-developer-platform-pp-cli boards get --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-miro-developer-platform -g
```

Then invoke `/pp-miro-developer-platform <query>` in Claude Code. The skill drives the CLI directly.

For Miro MCP tools (board operations, stickies, frames, diagrams), use the separate [miro-mcp-server](https://github.com/olgasafonova/miro-mcp-server) project instead. This CLI is shell + skill only.

## Health Check

```bash
miro-developer-platform-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/miro-developer-platform-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `MIRO_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `miro-developer-platform-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $MIRO_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
