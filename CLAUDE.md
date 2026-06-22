# Project Instructions for AI Agents

`miro-cli` is a single-binary Cobra CLI wrapping the Miro REST API. One verb per endpoint, JSON in and out, with a local SQLite mirror (`miro-cli sync` + `miro-cli query`) for offline search.

Module path: `github.com/olgasafonova/miro-cli`.

## Build & Test

```bash
make build    # binary to ./bin/miro-cli
make test     # go test -race -failfast ./...
make lint     # golangci-lint run
make install  # go install ./cmd/miro-cli
```

CI (`.github/workflows/ci.yml`) additionally runs `go mod verify`, `go mod tidy` drift check, `govulncheck`, `go vet`, race-enabled tests with coverage, `gosec`, and a goreleaser cross-platform build matrix.

## Architecture Overview

- `cmd/miro-cli/` — Cobra root + verb registration. Each resource group is a subcommand tree assembled in `root.go`.
- `internal/miro/` — REST client. Rate limiting (`ratelimit.go`), transient-failure retry with backoff + `Retry-After` (`client.go`), panic recovery (`recover.go`), share allowlist (`shareallowlist.go`, gates `boards share` per HG-3 in `~/Projects/claude-code-config/rules/code-review-prompts.md`), config (`config.go`).
- `internal/tools/<resource>/` — one package per Miro resource (boards, items, stickies, shapes, frames, tags, etc.). Each defines its Cobra subcommands and calls into `internal/miro`.
- `internal/store/` — SQLite mirror + FTS5 search backing `miro-cli sync` / `miro-cli query`.
- `internal/diagrams/` — sequence / flowchart rendering helpers for `boards diagram`.
- `spec.json` — Miro Developer Platform OpenAPI spec, bundled verbatim from [miroapp/api-clients](https://github.com/miroapp/api-clients) under MIT (see `NOTICE`). Used at design time for parameter discovery; not embedded in the binary. Refresh with `make sync-spec`.

## Conventions & Patterns

- **Destructive verbs refuse to run without `--yes` (or `--agent`, which implies it).** New destructive verbs MUST follow this gate.
- **`--idempotent` makes create/delete retries safe** by treating duplicate-resource / already-deleted as success.
- **`--json`, `--dry-run`, `--select`, `--agent`** are the four agent-facing flags. Preserve their semantics across new commands.
- **Share allowlist is fail-closed: unset = deny-all. Do not relax this.** `MIRO_SHARE_ALLOWED_DOMAINS` gates by recipient domain; `MIRO_SHARE_ALLOWED_EMAILS`, when set, is authoritative (exact-address match, domain layer ignored — a strict tightening, never an OR-widening).
- **No new `replace` directives in `go.mod`** — they block `pkg.go.dev` indexing.
- **Tests live next to source** (`*_test.go`) and use the table-driven style established in `internal/miro/` and `internal/tools/boards/`.

## Related repos

- [miro-mcp-server](https://github.com/olgasafonova/miro-mcp-server) — same author, MCP runtime, overlapping coverage. CLI and MCP are complements, not alternatives (per the dual-path pattern in `mediawiki-mcp-server`).
