#!/usr/bin/env bash
# Shared acquisition/discovery/report owner. Analyzers own analysis semantics.
set -euo pipefail
owner=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
exec python3 "$owner/tools/go-analysis/runner.py" "$@"
