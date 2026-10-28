#!/bin/sh
set -eu
HERE=$(cd "$(dirname "$0")" && pwd)
exec python3 "$HERE/image_secret_scan_mutate.py"
