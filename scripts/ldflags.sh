#!/usr/bin/env sh

set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)

BUILD_VERSION=${BUILD_VERSION:-"$("$SCRIPT_DIR/version.sh")"}
BUILD_REVISION=${BUILD_REVISION:-}
BUILD_BRANCH=${BUILD_BRANCH:-}
BUILD_DATE=${BUILD_DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}
BUILD_USER=${BUILD_USER:-matzmz}

if [ -z "$BUILD_REVISION" ]; then
  if [ -n "${CI_COMMIT_SHA:-}" ]; then
    BUILD_REVISION=$(printf '%s' "$CI_COMMIT_SHA" | cut -c1-12)
  elif [ -n "${GITHUB_SHA:-}" ]; then
    BUILD_REVISION=$(printf '%s' "$GITHUB_SHA" | cut -c1-12)
  elif [ -n "${DRONE_COMMIT_SHA:-}" ]; then
    BUILD_REVISION=$(printf '%s' "$DRONE_COMMIT_SHA" | cut -c1-12)
  else
    BUILD_REVISION=$(git -C "$ROOT_DIR" rev-parse --short=12 HEAD 2>/dev/null || printf 'unknown')
  fi
fi

if [ -z "$BUILD_BRANCH" ]; then
  if [ -n "${CI_COMMIT_REF_NAME:-}" ]; then
    BUILD_BRANCH=$CI_COMMIT_REF_NAME
  elif [ "${GITHUB_REF_TYPE:-}" = "branch" ] && [ -n "${GITHUB_REF_NAME:-}" ]; then
    BUILD_BRANCH=$GITHUB_REF_NAME
  elif [ -n "${GITHUB_HEAD_REF:-}" ]; then
    BUILD_BRANCH=$GITHUB_HEAD_REF
  elif [ -n "${DRONE_BRANCH:-}" ]; then
    BUILD_BRANCH=$DRONE_BRANCH
  else
    BUILD_BRANCH=$(git -C "$ROOT_DIR" symbolic-ref --short -q HEAD 2>/dev/null || printf 'unknown')
  fi
fi

printf '%s' "-s -w"
printf '%s' " -X pbs-exporter/internal/buildinfo.Version=$BUILD_VERSION"
printf '%s' " -X pbs-exporter/internal/buildinfo.Revision=$BUILD_REVISION"
printf '%s' " -X pbs-exporter/internal/buildinfo.Branch=$BUILD_BRANCH"
printf '%s' " -X pbs-exporter/internal/buildinfo.BuildDate=$BUILD_DATE"
printf '%s' " -X pbs-exporter/internal/buildinfo.BuildUser=$BUILD_USER"
printf '\n'
