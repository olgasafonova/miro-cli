#!/usr/bin/env bash
#
# Regenerate the Miro CLI from the curated spec using the printing-press
# generator. Source of truth: specs/miro-spec-curated.json.
#
# The generator is expected at $PP_REPO (defaults to ~/Projects/cli-printing-press).
# scripts/printing-press-version.txt pins the commit this artifact was built from;
# the script warns if the local generator is at a different commit.
#
# IMPORTANT: the printing-press --force flag is destructive. It will reset the
# output directory, including .git/, hand-authored helpers, and any directories
# the generator does not own (composites/, docs/, scripts/, specs/, HANDOFF.md).
# This script guards against that by refusing to run on a dirty working tree
# unless REGENERATE_ALLOW_DIRTY=1 is set explicitly. Even with that escape
# hatch, prefer to commit work first; recovery from a wipe requires re-cloning
# from origin and selectively replaying spec patches and doc edits.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PP_REPO="${PP_REPO:-$HOME/Projects/cli-printing-press}"
PINNED_VERSION="$(cat "$REPO_ROOT/scripts/printing-press-version.txt")"

# Safety guard: refuse to run on a dirty working tree.
# The generator's --force flag wiped .git and uncommitted spec patches once
# already (10-05-2026 incident; see HANDOFF.md "Phase 1 incident").
if [[ -d "$REPO_ROOT/.git" ]]; then
  cd "$REPO_ROOT"
  if ! git diff --quiet || ! git diff --cached --quiet || [[ -n "$(git ls-files --others --exclude-standard)" ]]; then
    echo "error: working tree is dirty. printing-press --force is destructive and" >&2
    echo "       will reset the output directory, wiping any uncommitted changes" >&2
    echo "       (including spec patches, hand-authored helpers, and untracked dirs)." >&2
    echo "" >&2
    echo "       Commit or stash your work before regenerating, or pass" >&2
    echo "       REGENERATE_ALLOW_DIRTY=1 to bypass this check." >&2
    if [[ "${REGENERATE_ALLOW_DIRTY:-0}" != "1" ]]; then
      exit 1
    fi
    echo "" >&2
    echo "warning: REGENERATE_ALLOW_DIRTY=1 set, proceeding anyway." >&2
  fi
fi

if [[ ! -d "$PP_REPO" ]]; then
  echo "error: printing-press generator not found at $PP_REPO" >&2
  echo "set PP_REPO env var or clone https://github.com/mvanhorn/cli-printing-press" >&2
  exit 1
fi

cd "$PP_REPO"
ACTUAL_VERSION="$(git rev-parse HEAD)"

if [[ "$ACTUAL_VERSION" != "$PINNED_VERSION" ]]; then
  echo "warning: printing-press at $PP_REPO is at $ACTUAL_VERSION" >&2
  echo "         pinned version (per scripts/printing-press-version.txt) is $PINNED_VERSION" >&2
  echo "         output may differ from what's committed." >&2
  echo "" >&2
  read -r -p "continue anyway? [y/N] " answer
  [[ "$answer" =~ ^[Yy]$ ]] || exit 1
fi

go build -o ./printing-press ./cmd/printing-press

./printing-press generate \
  --spec "$REPO_ROOT/specs/miro-spec-curated.json" \
  --output "$REPO_ROOT" \
  --force

cd "$REPO_ROOT"

# Strip MCP artifacts. printing-press emits them unconditionally; this CLI
# does not ship an MCP wrapper (see HANDOFF.md "Scope pivot"). Miro MCP tools
# live in the separate miro-mcp-server repo.
rm -rf "$REPO_ROOT/cmd/miro-developer-platform-pp-mcp"
rm -rf "$REPO_ROOT/internal/mcp"
rm -f  "$REPO_ROOT/manifest.json"

go build ./...

echo ""
echo "regeneration complete."
echo "  generator: $ACTUAL_VERSION"
echo "  spec:      specs/miro-spec-curated.json"
echo "  output:    $REPO_ROOT"
echo ""
echo "if you bumped the generator intentionally, update the pin:"
echo "  echo $ACTUAL_VERSION > scripts/printing-press-version.txt"
