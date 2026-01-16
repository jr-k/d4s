#!/usr/bin/env bash
set -e

TAG="$1"

if [ -z "$TAG" ]; then
  echo "Usage: $0 <tag>"
  echo "Example: $0 v0.42.9"
  exit 1
fi

echo "==> Removing local tag: $TAG"
git tag -d "$TAG" 2>/dev/null || echo "Local tag does not exist, ok"

echo "==> Removing remote tag: $TAG"
git push origin ":refs/tags/$TAG" 2>/dev/null || echo "Remote tag does not exist, ok"

echo "==> Creating tag: $TAG"
git tag -a "$TAG" -m "Release $TAG"

echo "==> Pushing tag: $TAG"
git push origin "$TAG"

echo "==> Done. The GitHub Actions workflow should trigger."
