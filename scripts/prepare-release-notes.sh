#!/usr/bin/env bash
set -euo pipefail

tag="$1"
output="$2"
version="$(cat VERSION)"

if [[ "$tag" != "v$version" ]]; then
  echo "Release tag $tag does not match VERSION ($version)" >&2
  exit 1
fi

awk -v version="$version" '
  index($0, "## [" version "] - ") == 1 { found = 1; next }
  found && /^## \[/ { exit }
  found { print }
  END { if (!found) exit 1 }
' CHANGELOG.md > "$output"

if [[ ! -s "$output" ]]; then
  echo "No release notes found for $tag in CHANGELOG.md" >&2
  exit 1
fi
