#!/usr/bin/env bash
set -euo pipefail
owner=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
test "${1:-}" = lint
python3 "$owner/scripts/tests/go-analysis.py"
python3 "$owner/scripts/tests/go-analysis-mutations.py"
