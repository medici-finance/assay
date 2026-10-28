#!/bin/sh
# Offline only: no kubectl, discovery, downloads or implicit toolchain fetch.
set -eu
KUBECONFIG=/dev/null
GOPROXY=off
GOSUMDB=off
GOTOOLCHAIN=local
export KUBECONFIG GOPROXY GOSUMDB GOTOOLCHAIN
CHECK_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
exec python3 "$CHECK_DIR/check.py" "$@"
