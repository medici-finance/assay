#!/usr/bin/env bash
# Job-local transport client. Never falls back to an ambient installation.
set -euo pipefail
readonly version=2.37.8
readonly digest=6a2f98fee0901ff623ff9abb5c80f4d67c40e5c0ac5e2ad7afa2ba756a7d5088
: "${RUNNER_TEMP:?a job-owned temporary directory is required}"
[ "$#" -eq 1 ] && [ "$1" = "$RUNNER_TEMP/release-aws" ] || {
  echo 'release object client: explicit job-local destination required' >&2; exit 2;
}
[ ! -e "$1" ] && [ ! -L "$1" ] || {
  echo 'release object client: refusing to replace existing client files' >&2; exit 2;
}
mkdir -m 700 "$1"
curl -fsSL "https://awscli.amazonaws.com/awscli-exe-linux-x86_64-${version}.zip" -o "$1/client.zip"
echo "$digest  $1/client.zip" | sha256sum -c -
unzip -q "$1/client.zip" -d "$1/unpacked"
"$1/unpacked/aws/install" --install-dir "$1/install" --bin-dir "$1/bin"
case "$("$1/bin/aws" --version)" in
  "aws-cli/$version "*) ;;
  *) echo 'release object client: installed version mismatch' >&2; exit 2 ;;
esac
