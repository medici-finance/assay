package deskkit

import (
	"regexp"
	"strings"
	"testing"
)

// This file holds the PURE half of the census live check: the code that decides whether
// a workflow file declares a pull_request trigger, and its hermetic coverage. It is a
// _test.go file, so nothing here ships in a binary — but unlike cicensus_live_test.go it
// is NOT env-gated, so every test below runs in ordinary CI with no network.
//
// The split is the point (R1). The previous arrangement had the
// matcher living inside the env-gated live file with no reference anywhere else, so
// `grep` found no coverage of it at all: the thing written to catch a stale census row
// was itself unpinned, and a refactor of it could not fail any check. The live run is
// the only part that genuinely needs network; the parsing does not, and it is where the
// bugs are.

// onPullRequest matches a pull_request / pull_request_target token inside a workflow's
// `on:` BLOCK. It is applied to the extracted block only (see extractOnBlock), never to
// the whole file, so a `pull_request` appearing in a job, a step, an `if:` expression or
// a path filter cannot be mistaken for a trigger.
//
// Both the opener and the terminator classes are deliberately wide, because GitHub
// accepts the event in every YAML shape and each shape puts a different character next
// to the token:
//
//	on:                      → "\n  pull_request:"      mapping key      (space, colon)
//	on:                      → "\n  - pull_request"     block sequence   (dash, EOL)
//	on: [push, pull_request] → inline sequence                           (comma, bracket)
//	on: pull_request         → bare scalar                               (space, EOL)
//	on: {pull_request: {}}   → flow mapping                              (brace, colon)
//	on: ['push','pull_request'] → quoted sequence items                  (quote, quote)
//
// The suffix is `(?:_target)?` and the terminator is REQUIRED, so `pull_request_review`
// and `pull_requests` do not match: neither can reach a terminator after the token.
var onPullRequest = regexp.MustCompile(`(?:^|[\s\-\[\{,:'"])pull_request(?:_target)?(?:[\s:,\]\}'"]|$)`)

// onKeyLine matches the workflow's top-level `on:` key at column 0.
//
// `true` is in the alternation on purpose and is not a typo. YAML 1.1 resolves the
// unquoted plain scalar `on` to the BOOLEAN true, so any tool that round-trips a
// workflow through a YAML 1.1 library (yq, ruby-psych, older PyYAML) writes the key back
// out as `true:`. GitHub still accepts such a file, so a workflow on disk can genuinely
// have `true:` where a reader expects `on:` — and a scanner that looks only for the
// literal `on:` silently finds no trigger block at all. That is the classic form of this
// bug and it fails OPEN, so it is handled rather than assumed away.
var onKeyLine = regexp.MustCompile(`^(?:on|"on"|'on'|true)\s*:(.*)$`)

// extractOnBlock returns the text of a workflow's top-level `on:` block — the remainder
// of the `on:` line plus every following line that is blank or indented — and whether it
// found one at all.
//
// Scoping the match to this block is what lets onPullRequest be permissive without
// becoming wrong. Applied to a whole file, a wide pattern matches a `pull_request:`
// input under `workflow_call`, a job step that echoes the string, or an
// `if: github.event_name == pull_request` expression.
func extractOnBlock(src string) (string, bool) {
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		m := onKeyLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		var b strings.Builder
		b.WriteString(m[1]) // inline remainder: `on: pull_request`, `on: [push, pull_request]`
		b.WriteByte('\n')
		for _, next := range lines[i+1:] {
			if strings.TrimSpace(next) == "" {
				b.WriteByte('\n')
				continue
			}
			if next[0] != ' ' && next[0] != '\t' {
				break // next top-level key ends the block
			}
			b.WriteString(next)
			b.WriteByte('\n')
		}
		return b.String(), true
	}
	return "", false
}

// workflowTriggersPullRequest reports whether one workflow file declares a pull_request
// (or pull_request_target) trigger.
//
// It FAILS CLOSED: a file with no locatable top-level `on:` key returns true. The census
// consumes this to decide whether a `CIRequired: false` row is still honest, and a false
// NEGATIVE there is the fail-open direction — it confirms a `false` row on a repo that
// does run PR CI, which is precisely the R1 defect this check exists to catch. A false
// POSITIVE only pushes a row to `true`, which refuses a flip rather than granting one.
//
// The remaining over-match is a `pull_request` KEY nested inside the on: block that is
// not itself the trigger — a `workflow_call` input of that name. That is the safe
// direction and is left in deliberately.
func workflowTriggersPullRequest(src string) bool {
	block, ok := extractOnBlock(stripYAMLComments(src))
	if !ok {
		return true
	}
	return onPullRequest.MatchString(block)
}

// stripYAMLComments removes whole-line and trailing `#` comments so prose mentioning
// pull_request in a workflow header does not read as a trigger. A `#` inside a quoted
// scalar is not handled; truncating such a line can only DROP a match, and a dropped
// match inside the on: block leaves workflowTriggersPullRequest's fail-closed default
// only for a file whose `on:` key vanished entirely — which a `#` cannot cause, since
// the key is at column 0 before any `#`.
func stripYAMLComments(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// TestWorkflowTriggersPullRequestSpellings is the hermetic pin R1 asked for.
//
// Three of these cases were BROKEN before this test existed, all in the fail-open
// direction: `on: pull_request`, `on: {pull_request: {}}` and `on: pull_request_target`
// all returned false, so a census row saying `CIRequired: false` for such a repo passed
// the live check while the repo ran PR CI on every PR.
func TestWorkflowTriggersPullRequestSpellings(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want bool
	}{
		// --- Positive: every spelling GitHub accepts ---
		{"mapping_key", "name: X\non:\n  pull_request:\n    branches: [main]\n", true},
		{"mapping_key_null", "name: X\non:\n  pull_request:\n", true},
		{"block_sequence", "name: X\non:\n  - push\n  - pull_request\n", true},
		{"inline_sequence", "name: X\non: [push, pull_request]\n", true},
		{"inline_sequence_single", "name: X\non: [pull_request]\n", true},
		{"inline_sequence_quoted", "name: X\non: ['push', 'pull_request']\n", true},
		// R1: the bare scalar form. Missed before.
		{"bare_scalar", "name: X\non: pull_request\n", true},
		// R1: the flow-mapping form. Missed before.
		{"flow_mapping", "name: X\non: {pull_request: {}}\n", true},
		{"flow_mapping_with_types", "name: X\non: {pull_request: {types: [opened]}}\n", true},
		// R1: pull_request_target in the scalar form. Missed before; not named in the
		// review, found by applying the same reasoning to the other event.
		{"pr_target_scalar", "name: X\non: pull_request_target\n", true},
		{"pr_target_mapping", "name: X\non:\n  pull_request_target:\n    types: [opened]\n", true},
		{"quoted_on_key", "name: X\n\"on\":\n  pull_request:\n", true},
		{"single_quoted_on_key", "name: X\n'on':\n  pull_request:\n", true},
		// YAML 1.1 resolves a bare `on` key to the boolean true; a round-tripped
		// workflow really can carry `true:` on disk.
		{"yaml11_true_key", "name: X\ntrue:\n  pull_request:\n", true},
		{"no_space_after_colon", "name: X\non:pull_request\n", true},
		{"trailing_whitespace", "name: X\non: pull_request   \n", true},
		{"mixed_with_push", "name: X\non:\n  push:\n    branches: [main]\n  pull_request:\n", true},
		{"crlf_mapping", "name: X\r\non:\r\n  pull_request:\r\n", true},

		// --- Negative: near-misses that must NOT read as a PR trigger ---
		{"neg_push_only", "name: X\non:\n  push:\n    branches: [main]\n", false},
		{"neg_pull_request_review", "name: X\non:\n  pull_request_review:\n    types: [submitted]\n", false},
		{"neg_pull_requests_plural", "name: X\non:\n  pull_requests:\n", false},
		{"neg_schedule_only", "name: X\non:\n  schedule:\n    - cron: \"0 6 * * *\"\n", false},
		{"neg_commented_out", "name: X\non:\n  push:\n# on:\n#   pull_request:\n", false},
		{"neg_trailing_comment", "name: X\non:\n  push:  # not pull_request\n", false},
		// Outside the on: block entirely — this is what block-scoping buys.
		{"neg_event_name_expression", "name: X\non:\n  push:\njobs:\n  a:\n    if: github.event_name == 'pull_request'\n", false},
		{"neg_step_echoes_it", "name: X\non:\n  push:\njobs:\n  a:\n    steps:\n      - run: echo pull_request\n", false},
		{"neg_job_named_pull_request", "name: X\non:\n  push:\njobs:\n  pull_request:\n    runs-on: ubuntu-latest\n", false},
		{"neg_path_filter_filename", "name: X\non:\n  push:\n    paths:\n      - '.github/workflows/pull_request.yml'\n", false},

		// --- Fail-closed: an unreadable/absent on: block reads as a trigger ---
		{"failclosed_no_on_key", "name: X\njobs:\n  a:\n    runs-on: ubuntu-latest\n", true},
		{"failclosed_empty_file", "", true},
		{"failclosed_indented_on_key", "name: X\n  on:\n    pull_request:\n", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := workflowTriggersPullRequest(c.yaml); got != c.want {
				t.Fatalf("workflowTriggersPullRequest(%q) = %v, want %v", c.yaml, got, c.want)
			}
		})
	}
}

// TestExtractOnBlockStopsAtNextTopLevelKey pins the scoping itself rather than the
// matcher, so a change that widens the block back to the whole file fails here even if
// every spelling above still resolves correctly.
func TestExtractOnBlockStopsAtNextTopLevelKey(t *testing.T) {
	const src = "name: X\non:\n  push:\n    branches: [main]\n\njobs:\n  pull_request:\n    runs-on: ubuntu-latest\n"
	block, ok := extractOnBlock(src)
	if !ok {
		t.Fatal("extractOnBlock found no on: block")
	}
	if strings.Contains(block, "jobs:") || strings.Contains(block, "runs-on") {
		t.Fatalf("on: block leaked past the next top-level key:\n%s", block)
	}
	if !strings.Contains(block, "push:") {
		t.Fatalf("on: block dropped its own content:\n%s", block)
	}
}

// TestStripYAMLComments pins the comment stripper independently: a header paragraph
// mentioning pull_request must not survive into the matcher's input.
func TestStripYAMLComments(t *testing.T) {
	got := stripYAMLComments("on:\n  # runs on pull_request too\n  push:  # and not pull_request\n")
	if strings.Contains(got, "pull_request") {
		t.Fatalf("comment text survived stripping: %q", got)
	}
	if !strings.Contains(got, "push:") {
		t.Fatalf("stripper ate non-comment content: %q", got)
	}
}
