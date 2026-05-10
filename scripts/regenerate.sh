#!/usr/bin/env bash
#
# Regenerate the Miro CLI from the curated spec using the printing-press
# generator. Source of truth: specs/miro-spec-curated.json.
#
# The generator is expected at $PP_REPO (defaults to ~/Projects/cli-printing-press).
# scripts/printing-press-version.txt pins the commit this artifact was built from;
# the script warns if the local generator is at a different commit.
#
# Hand-authored files under internal/cli/*.go are preserved by the generator's
# --force flag.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PP_REPO="${PP_REPO:-$HOME/Projects/cli-printing-press}"
PINNED_VERSION="$(cat "$REPO_ROOT/scripts/printing-press-version.txt")"

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
go build ./...

echo ""
echo "regeneration complete."
echo "  generator: $ACTUAL_VERSION"
echo "  spec:      specs/miro-spec-curated.json"
echo "  output:    $REPO_ROOT"
echo ""
echo "if you bumped the generator intentionally, update the pin:"
echo "  echo $ACTUAL_VERSION > scripts/printing-press-version.txt"
