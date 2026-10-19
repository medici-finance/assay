#!/bin/sh
# Compatibility entry point. Detection is stock Gitleaks; Python supplies image
# traversal, raw inputs and fail-closed accounting.
command -v python3 >/dev/null 2>&1 || { echo 'image-secret-scan: python3 unavailable' >&2; exit 2; }
exec python3 -I -c 'import runpy, sys; sys.exit(runpy.run_path(sys.argv.pop(1))["main"]())' "$(dirname "$0")/image_secret_scan.py" "$@"
