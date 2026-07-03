#!/usr/bin/env sh

set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
OUTPUT_DIR=${OUTPUT_DIR:-"$ROOT_DIR/dist"}
BINARY_NAME=${BINARY_NAME:-pbs-exporter-linux-amd64}
GO_BUILDER_IMAGE=${GO_BUILDER_IMAGE:-golang:1.25}
CONTAINER_OUTPUT_DIR=/out
LDFLAGS=${LDFLAGS:-"$("$SCRIPT_DIR/ldflags.sh")"}

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
