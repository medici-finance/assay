#!/bin/sh
# Explicit local benchmark; no download, provider, persistent cache or upload.
set -eu
module=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
  echo 'usage: perfworkload.sh REPORT.md [SAMPLES=20]' >&2
  exit 2
fi
case "$1" in /*) report=$1 ;; *) report="$PWD/$1" ;; esac
revision=$(git -C "$module" rev-parse HEAD)
export GOWORK=off GOPROXY=off GOTOOLCHAIN=local KUBECONFIG=/dev/null
export ASSAY_PERF_WORKLOAD_REPORT="$report"
export ASSAY_PERF_WORKLOAD_SAMPLES="${2:-20}"
export ASSAY_PERF_WORKLOAD_SOURCE="$revision"
cd "$module"
exec go test -count=1 -timeout=240s -v -run '^TestPerfWorkloadSamples$' .
