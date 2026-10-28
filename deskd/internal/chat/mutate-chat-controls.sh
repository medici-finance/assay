#!/usr/bin/env bash
# mutate-chat-controls.sh — fail-first evidence for the four controls the Ask
# Assay chat rests on (the service contract, worker-prompt kit §9).
#
# WHY MUTATION AND NOT A PRE-FIX RED RUN.
#
# Every test this brief adds is NEW, against code that is also new. There is no
# pre-fix commit at which they compile, let alone fail: internal/chat did not
# exist, so `git stash && go test ./internal/chat/` is not a red run, it is a
# build error. A build error is not evidence that a check can catch the thing
# it claims to catch.
#
# So the red is produced by MUTATION. Each case below removes or inverts
# exactly ONE control — the way a plausible future edit would — and asserts the
# matching test then FAILS. If a test still passes with its control removed,
# that test is not pinning what it claims to pin, and this script reports it.
#
# The four controls, each failing on a different signal in a different
# component (brief §single-point-of-failure):
#
#   1  the numbers rule       figure.go        a rendered-figure signal
#   2  the tool binding       binding.go       a tool-registry signal
#   3  the render             askassay.html    an output-encoding signal
#   4  the config gate        chat.go (schema) a boot-validation signal
#
# plus the credential redaction, which is the gate-why property (3).
#
# It never edits the working tree: the module is copied to a tempdir, the
# mutation is applied to the copy, and the copy is thrown away.
#
# Usage:  internal/chat/mutate-chat-controls.sh
# Exit:   0 every control proved load-bearing
#         1 a control is not load-bearing
#         2 usage / environment error
set -uo pipefail

here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../.." && pwd)

command -v go >/dev/null 2>&1 || { echo "go is not on PATH" >&2; exit 2; }
[ -f "$repo/go.mod" ] || { echo "no go.mod at $repo" >&2; exit 2; }

tmp=$(mktemp -d "${TMPDIR:-/tmp}/mutate-chat.XXXXXX") || exit 2
trap 'rm -rf "$tmp"' EXIT

fails=0
cases=0

# fresh_copy <name> — a pristine copy of the module, path echoed.
fresh_copy() {
  local d="$tmp/$1"
  rm -rf "$d"; mkdir -p "$d"
  # .git is deliberately excluded — see the sibling script's note.
  (cd "$repo" && tar --exclude='./.git' -cf - .) | (cd "$d" && tar -xf -)
  echo "$d"
}

# expect_red <label> <dir> <pkg> <test-regex>
# The mutated tree must FAIL the named test. A pass means the control the
# mutation removed was not what the test was pinning.
expect_red() {
  local label=$1 dir=$2 pkg=$3 re=$4
  cases=$((cases + 1))
  local out
  out=$(cd "$dir" && go test -count=1 -timeout 180s -run "$re" "$pkg" 2>&1)
  if [ $? -eq 0 ]; then
    fails=$((fails + 1))
    printf 'NOT LOAD-BEARING: %s\n  %s %s still PASSES with the control removed\n' "$label" "$pkg" "$re"
  else
    printf 'red as expected: %s\n' "$label"
    printf '%s\n' "$out" | grep -E '^\s+(---)?\s*(FAIL|[a-z_]+\.go:[0-9]+)' | head -4 | sed 's/^/    /'
  fi
}

echo "== control 2: the tool binding (binding.go) =="

d=$(fresh_copy allowlist)
# Remove the allow-list refusal: a name not on the list is bound anyway.
perl -0pi -e 's/\t\tif !allowed\[name\] \{\n(.*?\n)*?\t\t\}\n//' "$d/internal/chat/binding.go"
# ...and the forbidden-name refusal, which would otherwise still catch it.
perl -0pi -e 's/\t\tif why, forbidden := forbiddenChatTools\[name\]; forbidden \{\n(.*?\n)*?\t\t\}\n//' "$d/internal/chat/binding.go"
# The subtest targeted here is the one that isolates the ALLOW-LIST: it injects
# a genuine read tool (desk__claim and friends — no Mutates, no AuthorsAs), so
# no other refusal in bindNames can catch it. Targeting the whole test instead
# would pass for the wrong reason — which is exactly what this script caught on
# its first run, and why the test was split.
expect_red "a non-allow-listed READ tool is refused (isolates the allow-list check)" \
  "$d" ./internal/chat/ 'TestChatBindsOnlyReadTools/a_non-mutating'

d=$(fresh_copy mutates)
# Remove the "advertised as mutating" refusal only. The allow-list check stays,
# so this proves the two refusals are independently load-bearing.
perl -0pi -e 's/\t\tif t\.Mutates \{\n(.*?\n)*?\t\t\}\n//' "$d/internal/chat/binding.go"
expect_red "a mutating tool advertised under an allow-listed name is refused" \
  "$d" ./internal/chat/ 'TestChatBindsOnlyReadTools/a_mutating'

d=$(fresh_copy authority)
# Widen the shipped binding itself with a platform-writing tool. The §11
# authority test must catch it even though binding.go is untouched.
perl -pi -e 's/^var boundToolNames = \[6\]string\{$/var boundToolNames = [7]string{\n\t"desk__escalate",/' \
  "$d/internal/chat/binding.go"
expect_red "the authority test catches a write tool in the SHIPPED binding" \
  "$d" ./internal/mcp/ TestAuthority

echo
echo "== control 1: the numbers rule (figure.go) =="

d=$(fresh_copy modeltext)
# The watermelon: build a figure out of the model's own words. This is the
# single edit the whole package exists to make impossible, so it is the one
# the fail-first evidence must show being caught.
perl -0pi -e 's/(\t\tif len\(reply\.ToolCalls\) == 0 \{\n)/\t\tif n := firstNumberIn(reply.Text); n >= 0 {\n\t\t\tq := questionFromProbe("model_stated", "portfolio", probeJSON{Source: "the model said so", Method: "narration", Window: "n\/a", Limit: 0})\n\t\t\tans.Figures = append(ans.Figures, Figure{answer: askassay.Computed(q, n, askassay.ClockOnly(c.now(), "model text")), queryKey: "model_stated"})\n\t\t}\n$1/' \
  "$d/internal/chat/chat.go"
cat >>"$d/internal/chat/chat.go" <<'MUTEOF'

// injected by mutate-chat-controls.sh
func firstNumberIn(s string) int64 {
	cur, seen := int64(-1), false
	for _, r := range s {
		if r >= '0' && r <= '9' {
			if !seen {
				cur, seen = 0, true
			}
			cur = cur*10 + int64(r-'0')
		} else if seen {
			return cur
		}
	}
	return cur
}
MUTEOF
perl -pi -e 's{^import \(\n}{import (\n\t"github.com/example-org/assay/tools/desk/askassay"\n}' "$d/internal/chat/chat.go"
expect_red "a model-stated number never renders" \
  "$d" ./internal/chat/ TestModelStatedNumberNeverRenders

d=$(fresh_copy cleanonly)
# Ignore the tool's OWN assessment: treat a blind/throttled probe as a clean
# measurement. This is the "could-not-check decays into a zero" failure.
perl -pi -e 's/^\tif res\.Assessment\.Confidence != confidenceCheckedClean \{$/\tif false {/' \
  "$d/internal/chat/figure.go"
expect_red "a throttled or blind result renders could-not-check, never 0" \
  "$d" ./internal/chat/ TestFigureComesFromToolResultOnly

echo
echo "== control 3: the render (web/templates/askassay.html) =="

d=$(fresh_copy render)
# Render the model's words as trusted markup — the §6.6 failure exactly.
perl -pi -e 's{<p class="turn-text">\{\{\.Text\}\}</p>}{<p class="turn-text">\{\{.TextHTML\}\}</p>}' \
  "$d/web/templates/askassay.html"
perl -0pi -e 's/(type turnVM struct \{\n)/$1\tTextHTML template.HTML\n/' "$d/web/panes.go"
perl -0pi -e 's/(\t"context"\n)/$1\t"html\/template"\n/' "$d/web/panes.go"
perl -pi -e 's/\{Role: "assay", Text: ans\.Text,/{Role: "assay", Text: ans.Text, TextHTML: template.HTML(ans.Text),/' \
  "$d/web/panes.go"
expect_red "chat output renders as escaped TEXT, never markup" \
  "$d" ./web/ TestChatOutputIsTextNotMarkup

echo
echo "== control 4: the config gate (config/schema/chat.go) =="

d=$(fresh_copy ladder)
# Accept any model name — the "someone configures a metered backend to make it
# smarter" failure.
perl -pi -e 's/^\t\trank, ok := chatLadderRank\(m\)$/\t\trank, ok := i, true/' "$d/config/schema/chat.go"
expect_red "a model outside the ladder is refused at boot" \
  "$d" ./cmd/cellsvalidate/ TestAnswerRecordsBackendConfigRefusedOutsideLadder

echo
echo "== gate-why property 3: the key cannot be logged or rendered =="

d=$(fresh_copy redaction)
# The single most likely accident: someone "helpfully" makes Credential print
# its value while debugging.
perl -pi -e 's/^func \(c Credential\) String\(\) string \{ return redaction \}$/func (c Credential) String() string { return c.v }/' \
  "$d/internal/chat/backend.go"
expect_red "the credential cannot reach a log line or a rendered page" \
  "$d" ./internal/chat/ TestCredentialCannotBeLoggedOrRendered

echo
if [ "$fails" -ne 0 ]; then
  echo "FAIL: $fails of $cases control(s) are not load-bearing"
  exit 1
fi
echo "OK: all $cases controls proved load-bearing (each test went red with its control removed)"
