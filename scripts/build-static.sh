#!/usr/bin/env sh

set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
OUTPUT_DIR=${OUTPUT_DIR:-"$ROOT_DIR/dist"}
BINARY_NAME=${BINARY_NAME:-pbs-exporter-linux-amd64}
GO_BUILDER_IMAGE=${GO_BUILDER_IMAGE:-golang:1.25}
CONTAINER_OUTPUT_DIR=/out
BUILD_VERSION=${BUILD_VERSION:-"$("$SCRIPT_DIR/version.sh")"}
BUILD_REVISION=${BUILD_REVISION:-$(git -C "$ROOT_DIR" rev-parse --short=12 HEAD 2>/dev/null || printf 'unknown')}
BUILD_BRANCH=${BUILD_BRANCH:-$(git -C "$ROOT_DIR" symbolic-ref --short -q HEAD 2>/dev/null || printf 'detached')}
BUILD_DATE=${BUILD_DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}
BUILD_USER=${BUILD_USER:-$(id -un 2>/dev/null || printf 'unknown')}
LDFLAGS="-s -w -X pbs-exporter/internal/buildinfo.Version=$BUILD_VERSION -X pbs-exporter/internal/buildinfo.Revision=$BUILD_REVISION -X pbs-exporter/internal/buildinfo.Branch=$BUILD_BRANCH -X pbs-exporter/internal/buildinfo.BuildDate=$BUILD_DATE -X pbs-exporter/internal/buildinfo.BuildUser=$BUILD_USER"

mkdir -p "$OUTPUT_DIR"

docker run --rm \
  -u "$(id -u):$(id -g)" \
  -v "$ROOT_DIR":/src \
  -v "$OUTPUT_DIR":"$CONTAINER_OUTPUT_DIR" \
  -w /src \
  -e CGO_ENABLED=0 \
  -e GOOS=linux \
  -e GOARCH=amd64 \
  -e GOCACHE=/tmp/go-build \
  -e GOMODCACHE=/tmp/go-mod \
  -e LDFLAGS="$LDFLAGS" \
  "$GO_BUILDER_IMAGE" \
  sh -c "go mod download && go build -trimpath -ldflags \"\$LDFLAGS\" -o \"$CONTAINER_OUTPUT_DIR/$BINARY_NAME\" ."
