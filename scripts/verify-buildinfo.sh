#!/usr/bin/env sh

set -eu

if [ "$#" -ne 1 ]; then
  printf 'usage: %s /path/to/pbs-exporter\n' "$0" >&2
  exit 2
fi

BINARY_PATH=$1
VERSION_OUTPUT=$("$BINARY_PATH" --version)

case "$VERSION_OUTPUT" in
  *"version dev"*|*"revision: unknown"*|*"branch: unknown"*|*"build date:       unknown"*)
    printf 'invalid build metadata:\n%s\n' "$VERSION_OUTPUT" >&2
    exit 1
    ;;
esac

printf '%s\n' "$VERSION_OUTPUT"
