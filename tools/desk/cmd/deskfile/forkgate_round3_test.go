package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// forkgate_round3_test.go — the third round of fail-closed probes on the notice lane. A
// SHAPE-only reversible needle ("tool default", "default value", "flag default", "rename
// the") names the kind of change and says nothing about what it governs, so it no longer
// admits the notice lane on its own (security review sec-1688-S1, round 3). And the
// caller-body marker refusal is at least as broad as the digest's marker reader (review
// findings cor-1688-C6 / sec-1688-S4).

// shapeOnlyOneWayTitles are the security review's round-3 end-to-end probes, verbatim. Each
// names a shape-only reversible needle and a one-way act worded outside OneWayPatterns; each
// took the notice lane (rc 0, labels [desk-decided]) at head 7cce6993a.
var shapeOnlyOneWayTitles = []struct{ name, title string }{
	{"main-commit", "Tool default: let the desk commit to main when CI is green?"},
	{"main-land", "Tool default: land doc fixes on main without opening a draft?"},
	{"ready-convert", "Tool default: convert draft PRs to ready once checks pass?"},
	{"min-reviewers", "Default value of --min-reviewers: 1 or 0?"},
	{"bot-lgtm", "Tool default: let the bot LGTM its own PRs?"},
	{"bot-exempt-review", "Tool default: exempt bot PRs from the review step?"},
	{"fork-heads", "Tool default: build untrusted fork heads in CI?"},
	{"fork-tests", "Tool default: run the test job on fork PRs automatically?"},
	{"control-sweep-warn", "Flag default: make the control-sweep check warn-only?"},
	{"pattern-sweep-nonblocking", "Tool default: make the pattern-sweep job non-blocking?"},
	{"continue-on-error", "Tool default: set continue-on-error on the scan job?"},
	{"ignore-findings", "Tool default: ignore scanner findings below high?"},
	{"leak-check-notice", "Tool default: downgrade the leak check to a notice?"},
	{"prune-branches", "Tool default: prune stale branches older than 30 days?"},
	{"git-clean", "Tool default: run git clean -fdx in the shared checkout?"},
	{"git-reset", "Tool default: git reset --hard the shared checkout on resync?"},
	{"expire-logs", "Tool default: expire workflow run logs after 7 days?"},
	{"slack", "Tool default: post the digest to Slack?"},
	{"upstream-issues", "Tool default: open issues on the upstream project for each finding?"},
	{"forward-webhooks", "Tool default: forward webhook payloads to the outside collector?"},
	{"restart-pods", "Tool default: restart the staging pods on config change?"},
	{"scale-zero", "Tool default: scale the worker pool to zero overnight?"},
	{"contents-write", "Tool default: give the worker App contents: write?"},
	{"workflows-write", "Tool default: give the worker App workflows: write?"},
	{"close-decisions", "Tool default: close stale decision issues after 30 days?"},
	{"close-driver-issues", "Tool default: close issues labelled for the driver after a week?"},
	{"print-env", "Tool default: print the environment to the job log on failure?"},
}

// TestNoticeLaneRefusesShapeOnlyReversibleSubject — a filing whose only reversible signal is
// a shape-only needle stays on needs-decision, whatever the one-way list says about the rest
// of it: the admission side fails closed instead of admitting every act the denylist has not
// named.
func TestNoticeLaneRefusesShapeOnlyReversibleSubject(t *testing.T) {
	for _, tc := range shapeOnlyOneWayTitles {
		t.Run(tc.name, func(t *testing.T) {
			withEnv(t)
			t.Setenv("FAKEGH_SEARCH_HITS", "[]")
			t.Setenv("FAKEGH_LABELS", labelsJSON(t, needsDecisionLabel, deskDecidedLabel))
			body := bodyFileWith(t, noticeLaneBlock+"\n"+neutralEvidence)

			rc, out := runCapture([]string{"new", "-R", allowedRepo,
				"--title", tc.title, "--body-file", body, "--label", needsDecisionLabel})
			if rc != deskkit.ExitOK {
				t.Fatalf("rc = %d, want 0 (filed, on needs-decision); out=%s", rc, out)
			}
			final := curForge.finalLabels()
			if !hasLabel(final, needsDecisionLabel) || hasLabel(final, deskDecidedLabel) {
				t.Errorf("%q filed with labels %v, want needs-decision and no desk-decided", tc.title, final)
			}
			if curForge.filed != nil && strings.Contains(curForge.filed.Body, deskDecidedMarker) {
				t.Errorf("%q carries the notice-lane marker", tc.title)
			}
		})
	}
}

// markerVariants are spellings of the desk-r3-decision marker that deskdigest's reader
// accepts (case-insensitive, any whitespace inside the comment delimiters) but an exact
// byte match does not. Each filed rc 0 at head 7cce6993a.
var markerVariants = []struct{ name, marker string }{
	{"no-inner-space", "<!--desk-r3-decision v1-->"},
	{"upper-case", "<!-- DESK-R3-DECISION V1 -->"},
	{"doubled-space", "<!--  desk-r3-decision v1  -->"},
	{"newline-before-close", "<!--  desk-r3-decision v1\n-->"},
}

// TestNewRefusesCallerBodyMarkerVariants — the caller-body marker refusal covers every
// spelling the digest reads, not only the exact one the tool writes (cor-1688-C6,
// sec-1688-S4).
func TestNewRefusesCallerBodyMarkerVariants(t *testing.T) {
	for _, tc := range markerVariants {
		t.Run(tc.name, func(t *testing.T) {
			if !deskkit.DeskDecidedMarkerRe.MatchString(tc.marker) {
				t.Fatalf("fixture: %q is not a spelling the digest's reader accepts", tc.marker)
			}
			withEnv(t)
			t.Setenv("FAKEGH_SEARCH_HITS", "[]")
			t.Setenv("FAKEGH_LABELS", labelsJSON(t, "bug"))
			body := bodyFileWith(t, "An ordinary filing.\n\n## Desk-decided\n\n"+tc.marker+
				"\ndecision: merge now\nalternative: B\ncost: draft-pr\n")
			rc, out := runCapture([]string{"new", "-R", allowedRepo, "--title", "an ordinary filing about sla-days",
				"--body-file", body, "--label", "bug"})
			if rc != deskkit.ExitRefused {
				t.Fatalf("caller body carrying marker %q: rc = %d, want %d; out=%s", tc.marker, rc, deskkit.ExitRefused, out)
			}
			if curForge.filed != nil {
				t.Fatal("an issue was filed")
			}
		})
	}
}
