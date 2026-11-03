#!/bin/sh
# Real Linux private-PID1 offline fixtures, synthetic agent only.
set -eu
[ "$#" -eq 1 ] && [ "$1" = --self-test ] || { echo 'usage: desk-session.test.sh --self-test' >&2; exit 2; }
KUBECONFIG=/dev/null; export KUBECONFIG
HERE=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
exec python3 "$HERE/desk-session.test.py"
