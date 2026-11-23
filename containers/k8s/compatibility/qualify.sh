#!/bin/sh
# The runner creates its own disposable loopback fixture.
set -eu
exec python3 -I "$(dirname "$0")/qualify.py" "$@"
