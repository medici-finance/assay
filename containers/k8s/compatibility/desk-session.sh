#!/bin/sh
# Image-owned bounded PID-1 launcher. The existing image ENTRYPOINT performs
# credential preflight before invoking this command; it is not replaced here.
set -eu
exec python3 /usr/local/lib/assay/desk-session.py "$@"
