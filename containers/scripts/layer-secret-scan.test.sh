#!/bin/sh
# Full contracts and real pinned-engine integration; no downloads during tests.
set -eu
HERE=$(cd "$(dirname "$0")" && pwd)
IMAGE_SCAN_INTEGRATION=1 exec python3 "$HERE/image_secret_scan_test.py" "$@"
