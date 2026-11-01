#!/bin/sh
set -eu
script_dir=$(CDPATH= cd "$(dirname "$0")" && pwd)
source_root=$(CDPATH= cd "$script_dir/.." && pwd)
cd "$source_root"
GOWORK=off GOPROXY=off GOSUMDB=off go run -mod=readonly ./internal/gitlaunch "$source_root"
