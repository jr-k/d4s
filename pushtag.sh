#!/usr/bin/env bash
set -e

TAG="$1"

if [ -z "$TAG" ]; then
  echo "Usage: $0 <tag>"
  echo "Example: $0 v0.42.9"
  exit 1
fi

echo "==> Creating tag: $TAG"
git tag -a "$TAG" -m "Release $TAG"

echo "==> Pushing tag: $TAG"
git push origin "$TAG"

echo "==> Done. The GitHub Actions workflow should trigger."