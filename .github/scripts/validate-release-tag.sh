#!/usr/bin/env bash
set -euo pipefail

tag=${1:?release tag is required}
if [[ ! $tag =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Invalid release tag: $tag (expected vMAJOR.MINOR.PATCH)" >&2
  exit 1
fi
