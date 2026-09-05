#!/usr/bin/env bash
# pdfingest — first-pass document ingestion via a docling-serve endpoint.
#
# Public assay tooling (MIT). Talks to YOUR docling-serve instance (stock
# upstream container, no house layer); falls back to a local `docling` CLI if
# no endpoint is configured. Outputs LLM-ready markdown on stdout.
#
# Usage:
#   pdfingest.sh <file.pdf|docx|pptx|xlsx|html|epub>      # markdown to stdout
#   pdfingest.sh --json <file>                            # docling JSON (page/bbox provenance)
#   pdfingest.sh --pages 5-9 <file>                       # page-range slice
#   pdfingest.sh --health                                 # probe the endpoint
#
# Config:
#   ASSAY_DOCLING_URL  base URL of docling-serve (default http://localhost:5001).
#                      Set it to your in-cluster instance, e.g. via
#                      kubectl port-forward svc/docling-serve 5001:5001.
#                      NEVER point this at a third-party API for confidential
#                      documents — the document body crosses this wire whole.
set -euo pipefail

URL="${ASSAY_DOCLING_URL:-http://localhost:5001}"
FORMAT="md"
PAGES=""

die() { echo "pdfingest: $*" >&2; exit 1; }

while [ $# -gt 0 ]; do
  case "$1" in
    --json)   FORMAT="json"; shift ;;
    --pages)  PAGES="${2:?--pages needs a value like 5-9}"; shift 2 ;;
    --health)
      curl -sf -o /dev/null -w '%{http_code}\n' "$URL/docs" \
        || die "endpoint $URL not healthy (is docling-serve up? kubectl port-forward?)"
      exit 0 ;;
    -h|--help) sed -n '2,20p' "$0"; exit 0 ;;
    -*) die "unknown flag: $1" ;;
    *)  FILE="$1"; shift ;;
  esac
done

[ -n "${FILE:-}" ] || die "no input file"
[ -f "$FILE" ] || die "not a file: $FILE"

# Build the options payload docling-serve expects (verify field names against
# your server's /docs — upstream may evolve them between versions).
OPTS="{\"to_formats\":[\"$FORMAT\"]"
[ -n "$PAGES" ] && OPTS="$OPTS,\"page_range\":{\"page_start\":${PAGES%%-*},\"page_end\":${PAGES##*-}}"
OPTS="$OPTS}"

if curl -sf -o /dev/null "$URL/docs" 2>/dev/null; then
  # Endpoint mode: multipart upload to /v1/convert/file.
  curl -sf -X POST "$URL/v1/convert/file" \
    -F "files=@$FILE" \
    -F "options=$OPTS" \
    | if [ "$FORMAT" = "md" ]; then
        # Pull the markdown document out of the conversion envelope.
        python3 -c 'import json,sys; d=json.load(sys.stdin); sys.stdout.write(d["document"]["md_content"] or "")'
      else
        cat
      fi
else
  # Fallback: local docling CLI, same models, no network.
  command -v docling >/dev/null 2>&1 \
    || die "no endpoint at $URL and no local 'docling' CLI — pip install docling or start the service"
  TOARG=( --to md ); [ "$FORMAT" = "json" ] && TOARG=( --to json )
  PAGESARG=( ); [ -n "$PAGES" ] && PAGESARG=( --page-range "$PAGES" )
  docling "$FILE" "${TOARG[@]}" "${PAGESARG[@]}" --output /tmp/pdfingest-out >/dev/null 2>&1
  cat "/tmp/pdfingest-out/$(basename "${FILE%.*}").${FORMAT}" 2>/dev/null \
    || cat /tmp/pdfingest-out/*."$FORMAT"
fi
