#!/usr/bin/env bash
# Exercise the shipped inbox caller with an offline producer and a failing receiver.
set -euo pipefail
launcher="${ASSAY_INBOX_TEST_LAUNCHER:-$(dirname "$0")/assay-inbox.sh}"
case "$launcher" in /*) ;; *) launcher="$PWD/$launcher" ;; esac
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cd "$work"
export ASSAY_DESKINBOX="$work/deskinbox"
cat > "$ASSAY_DESKINBOX" <<'STUB'
#!/usr/bin/env bash
set -eu
verb=$1; shift
if [[ "$1" == --help ]]; then
  if [[ -e old ]]; then echo 'Usage: deskinbox [flags]'; exit 0; fi
  printf 'Usage:\n  deskinbox %s SUBJECT [flags]\n' "$verb"; exit 0
fi
printf '%s\n' "$verb $*" >> calls
case "$verb" in
  show)
    [[ $# == 2 && "$1" == -- && "$2" == 'example-org/app#42' ]]
    printf 'State: ready\nOwner: human\n' ;;
  changes)
    case "$1" in --consumer=chat|--consumer=assay-inbox) ;; *) exit 5;; esac
    shift
    case "$1" in
      --ack=*)
        [[ "$*" == '--ack=receipt-42 -- example-org/app' ]] || exit 5
        touch acked ;;
      *)
        [[ -e refused ]] && { cat batch; echo 'producer precondition' >&2; exit 5; }
        [[ -e partial ]] && { cat batch; echo 'incomplete discovery' >&2; exit 6; }
        if [[ -e acked ]]; then echo '{"events":[]}'; else cat batch; fi ;;
    esac ;;
  *) exit 5 ;;
esac
STUB
chmod +x "$ASSAY_DESKINBOX"
# An old binary or incomplete read must never trigger ambient gh discovery.
printf '#!/bin/sh\ntouch ambient-read\nexit 90\n' > gh
chmod +x gh
export PATH="$work:$PATH"
poll() { bash "$launcher" --changes --consumer chat "$@" example-org/app; }
fail() { echo "FAIL: $*" >&2; exit 1; }
expect_failure() {
  local expected=$1 code=0; shift
  "$@" > out 2> err || code=$?
  [[ "$code" == "$expected" && ! -s out ]] || fail "expected exit $expected and empty stdout: $* (got $code)"
}

# First delivery, failed receiver, full replay, withdrawal and stable lookup.
cat > batch <<'BATCH'
{"receipt":"receipt-42","events":[{"id":"example-org/app#7","kind":"withdrawn"},{"id":"example-org/app#42","kind":"ready","action":{"owner":"human","act":"Choose."}}]}
BATCH
poll > first
cmp -s batch first || fail 'complete poll lost events, order or receipt'
if poll | (cat >/dev/null; exit 1); then fail 'receiver failure was hidden'; fi
[[ ! -e acked ]] || fail 'reading or failed delivery acknowledged the batch'
poll > replay
cmp -s first replay || fail 'undelivered events did not replay'
bash "$launcher" --show 'example-org/app#42' > shown
grep -Fx 'Owner: human' shown >/dev/null || fail 'stable issue lookup'
[[ ! -e acked ]] || fail 'looking up an act acknowledged it'
expect_failure 5 poll --ack superseded
[[ ! -e acked ]] || fail 'superseded receipt acknowledged'
poll --ack receipt-42 > out
[[ -e acked && ! -s out ]] || fail 'explicit scoped acknowledgement'
poll > out
[[ ! -s out ]] || fail 'unchanged poll produced a notification'
rm acked
touch partial
expect_failure 6 poll
grep -F 'incomplete discovery' err >/dev/null || fail 'original read diagnostic lost'
rm partial
touch refused
expect_failure 5 poll
grep -F 'producer precondition' err >/dev/null || fail 'producer refusal diagnostic lost'
rm refused
for kind in changed reminder; do
  jq -c --arg kind "$kind" '.events[1].kind=$kind' first > batch
  poll --remind-after 24h > out
  cmp -s batch out || fail "lost $kind event or receipt"
done
grep -Fx 'changes --consumer=chat --remind-after=24h -- example-org/app' calls >/dev/null || fail 'reminder policy lost'
jq -c 'del(.receipt)' first > batch
expect_failure 6 poll
grep -F 'missing inbox receipt' err >/dev/null || fail 'valid events without receipt were accepted'
for invalid in '' '{"events":[]' '{"events":null}' '{"events":[{}]}' '{"events":[{"id":"example-org/app#42","kind":"withdrawn","action":{"act":"Stale."}}],"receipt":"r"}' '{"events":[]} {"events":[]}'; do
  printf '%s' "$invalid" > batch
  expect_failure 6 poll
done
expect_failure 5 bash "$launcher" --show '--walk'
expect_failure 5 poll --remind-after 12h --ack receipt-42
for args in '--consumer' '--ack'; do
  count=$(wc -l < calls)
  expect_failure 5 poll "$args" 'two words'
  [[ $(wc -l < calls) == "$count" ]] || fail "invalid $args reached the producer"
done
touch old
expect_failure 6 poll
expect_failure 6 bash "$launcher" --show 'example-org/app#42'
[[ ! -e ambient-read && ! -e acked ]] || fail 'failure path touched another reader or acknowledged'
echo 'inbox delivery caller passed'
