#!/usr/bin/env sh

set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
VERSION_FALLBACK=${VERSION_FALLBACK:-dev}

if [ -n "${BUILD_VERSION:-}" ]; then
  printf '%s\n' "$BUILD_VERSION"
  exit 0
fi

if [ -n "${CI_COMMIT_TAG:-}" ]; then
  printf '%s\n' "$CI_COMMIT_TAG"
  exit 0
fi

if [ -n "${DRONE_TAG:-}" ]; then
  printf '%s\n' "$DRONE_TAG"
  exit 0
fi

if [ "${GITHUB_REF_TYPE:-}" = "tag" ] && [ -n "${GITHUB_REF_NAME:-}" ]; then
  printf '%s\n' "$GITHUB_REF_NAME"
  exit 0
fi

if ! command -v git >/dev/null 2>&1; then
  printf '%s\n' "$VERSION_FALLBACK"
  exit 0
fi

if ! git -C "$ROOT_DIR" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  printf '%s\n' "$VERSION_FALLBACK"
  exit 0
fi

EXACT_TAG=$(git -C "$ROOT_DIR" describe --tags --exact-match 2>/dev/null || true)
if [ -n "$EXACT_TAG" ]; then
  printf '%s\n' "$EXACT_TAG"
  exit 0
fi

DESCRIBED_VERSION=$(git -C "$ROOT_DIR" describe --tags --dirty --always 2>/dev/null || true)
if [ -n "$DESCRIBED_VERSION" ]; then
  printf '%s\n' "$DESCRIBED_VERSION"
  exit 0
fi

printf '%s\n' "$VERSION_FALLBACK"
