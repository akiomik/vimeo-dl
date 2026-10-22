#!/usr/bin/env bash
set -euo pipefail

validator="$(dirname "$0")/validate-release-tag.sh"
for tag in v0.2.0 v1.20.3; do
  bash "$validator" "$tag"
done
for tag in draft v1draft v1.2.3draft v1.2 v1.2.3.4 v1.2.3-rc.1 1.2.3; do
  if bash "$validator" "$tag" >/dev/null 2>&1; then
    echo "Invalid tag was accepted: $tag" >&2
    exit 1
  fi
done
