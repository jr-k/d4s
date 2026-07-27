#!/usr/bin/env bash
set -euo pipefail

TAG="${1:-}"

if [ -z "$TAG" ]; then
  echo "Usage: $0 <tag>"
  echo "Example: $0 v0.42.9"
  exit 1
fi

if [[ ! "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
  echo "Invalid tag: $TAG (expected format: v1.2.3)"
  exit 1
fi

if [ -n "$(git status --porcelain)" ]; then
  echo "The working tree must be clean before creating a release."
  git status --short
  exit 1
fi

if git rev-parse --verify --quiet "refs/tags/$TAG" >/dev/null; then
  echo "Tag already exists locally: $TAG"
  exit 1
fi

VERSION="${TAG#v}"
FLAKE_FILE="flake.nix"

if [ ! -f "$FLAKE_FILE" ]; then
  echo "Missing $FLAKE_FILE"
  exit 1
fi

replace_flake_value() {
  local key="$1"
  local value="$2"

  KEY="$key" VALUE="$value" perl -0pi -e '
    $key = $ENV{"KEY"};
    $value = $ENV{"VALUE"};
    $count = s/(\b\Q$key\E\s*=\s*")[^"]*(";)/$1$value$2/g;
    die "Expected exactly one $key entry in flake.nix, found $count\n" unless $count == 1;
  ' "$FLAKE_FILE"
}

run_nix_build() {
  if command -v nix >/dev/null 2>&1; then
    nix --extra-experimental-features "nix-command flakes" build .#default --no-link
  elif command -v docker >/dev/null 2>&1; then
    docker run --rm \
      -v "$PWD:/src" \
      -w /src \
      nixos/nix:latest \
      nix --extra-experimental-features "nix-command flakes" \
      build path:/src#default --no-link
  else
    echo "Nix or Docker is required to validate the flake."
    return 1
  fi
}

echo "==> Updating Nix package version to $VERSION"
replace_flake_value "version" "$VERSION"

echo "==> Validating Nix package"
set +e
BUILD_OUTPUT="$(run_nix_build 2>&1)"
BUILD_STATUS=$?
set -e
printf '%s\n' "$BUILD_OUTPUT"

if [ "$BUILD_STATUS" -ne 0 ]; then
  VENDOR_HASH="$(printf '%s\n' "$BUILD_OUTPUT" | awk '/got:[[:space:]]+sha256-/ { print $2; exit }')"
  if [ -z "$VENDOR_HASH" ]; then
    echo "Nix build failed."
    exit "$BUILD_STATUS"
  fi

  echo "==> Updating Go vendor hash"
  replace_flake_value "vendorHash" "$VENDOR_HASH"

  echo "==> Re-validating Nix package"
  run_nix_build
fi

if ! git diff --quiet -- "$FLAKE_FILE" flake.lock; then
  echo "==> Committing Nix release metadata"
  git add "$FLAKE_FILE" flake.lock
  git commit -m "chore(prepare $TAG)"
fi

echo "==> Creating tag: $TAG"
git tag -a "$TAG" -m "Release $TAG"

echo "==> Pushing release commit"
git push origin HEAD

echo "==> Pushing tag: $TAG"
git push origin "$TAG"

echo "==> Done. The GitHub Actions workflow should trigger."