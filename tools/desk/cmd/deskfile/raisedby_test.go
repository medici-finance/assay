package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// raisedby_test.go — the `raised-by:<role>` provenance stamp on `deskfile new`
// (methodology-metrics/29).
//
// The property under test is NOT "the label gets applied". It is the shape of the four
// outcomes: exactly one of them stamps, three of them file UNSTAMPED, all four are
// distinguished on the audit line, and none of the three non-stamping ones is allowed to
// stop the filing. A stamp that could block a filing would be a metric with veto power
// over the thing it measures.

// labelsJSON builds a FAKEGH_LABELS payload.
func labelsJSON(t *testing.T, names ...string) string {
	t.Helper()
	type label struct {
		Name string `json:"name"`
	}
	out := make([]label, 0, len(names))
	for _, n := range names {
		out = append(out, label{Name: n})
	}
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal labels: %v", err)
	}
	return string(b)
}

// createArgv returns the single `gh issue create` argv, or nil when none was made.
func createArgv(calls [][]string) []string {
	for _, c := range ghCalls(calls) {
		if len(c) >= 3 && c[1] == "issue" && c[2] == "create" {
			return c
		}
	}
	return nil
}

// labelArgs returns every value passed as `--label` in an argv.
func labelArgs(argv []string) []string {
	var out []string
	for i := 0; i+1 < len(argv); i++ {
		if argv[i] == "--label" {
			out = append(out, argv[i+1])
		}
	}
	return out
}

func hasString(hay []string, want string) bool {
	for _, h := range hay {
		if h == want {
			return true
		}
	}
	return false
}

// lastNewDetail returns the audit Detail of the most recent `new` entry.
func lastNewDetail(t *testing.T) string {
	t.Helper()
	entries := auditEntriesFor(t, "new")
	if len(entries) == 0 {
		t.Fatal("no `new` audit entry was written — every path must audit")
	}
	return entries[len(entries)-1].Detail
}

// --- outcome 1: stamped -----------------------------------------------------------

// TestRaisedByStampsWhenLabelExists — the happy path. The stamp rides alongside the
// caller's own labels rather than replacing them.
func TestRaisedByStampsWhenLabelExists(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, "bug", "raised-by:reviewer"))
	body := bodyFileWith(t, "an out-of-scope discovery found during review")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "reviewer found a stale retry budget", "--body-file", body,
		"--label", "bug", "--raised-by", "reviewer"})
	if rc != deskkit.ExitOK {
		t.Fatalf("stamped new rc = %d, want 0; out=%s", rc, out)
	}
	argv := createArgv(*calls)
	if argv == nil {
		t.Fatalf("no `gh issue create` was made; gh calls: %v", ghCalls(*calls))
	}
	got := labelArgs(argv)
	if !hasString(got, "raised-by:reviewer") {
		t.Fatalf("create argv carries no raised-by label: %v", got)
	}
	if !hasString(got, "bug") {
		t.Fatalf("the stamp displaced the caller's own label: %v", got)
	}
	if d := lastNewDetail(t); !strings.Contains(d, "raised-by=reviewer") {
		t.Fatalf("audit detail = %q, want it to carry raised-by=reviewer", d)
	}
	assertNoForbiddenGH(t, *calls)
}

// TestRaisedByProbesBeforeStamping — the stamp is only applied after the label is KNOWN
// to exist, because `gh issue create --label <missing>` fails the whole filing. A stamp
// applied on faith turns a missing metric label into a lost issue.
func TestRaisedByProbesBeforeStamping(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, "raised-by:worker"))
	body := bodyFileWith(t, "an insight worth routing")

	if rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "worker hit a stale pin during a fanout", "--body-file", body,
		"--raised-by", "worker"}); rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0; out=%s", rc, out)
	}
	if !anyCall(ghCalls(*calls), "label", "list") {
		t.Fatalf("no label-existence probe was made before stamping; gh calls: %v", ghCalls(*calls))
	}
}

// TestRaisedByLabelMatchIsCaseInsensitive — GitHub treats label names case-insensitively
// for uniqueness, so a repo holding `Raised-By:Worker` already HAS the label and a create
// of the lowercase form would collide. The probe must agree with the API.
func TestRaisedByLabelMatchIsCaseInsensitive(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, "Raised-By:Worker"))
	body := bodyFileWith(t, "case folding on the probe")

	if rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "probe folds label case like the api does", "--body-file", body,
		"--raised-by", "worker"}); rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0; out=%s", rc, out)
	}
	if !hasString(labelArgs(createArgv(*calls)), "raised-by:worker") {
		t.Fatalf("an existing label in different case read as missing; labels: %v",
			labelArgs(createArgv(*calls)))
	}
}

// --- outcome 2: not-requested -----------------------------------------------------

// TestRaisedByOmittedFilesUnstampedAndSaysSo — no flag means UNKNOWN provenance, said out
// loud. It must not probe (nothing to probe for) and must not stamp anything.
func TestRaisedByOmittedFilesUnstampedAndSaysSo(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	body := bodyFileWith(t, "a filing with no declared origin")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "an unattributed observation about retries", "--body-file", body})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0 — omitting the stamp must never block a filing; out=%s", rc, out)
	}
	for _, l := range labelArgs(createArgv(*calls)) {
		if strings.HasPrefix(strings.ToLower(l), deskkit.RaisedByPrefix) {
			t.Fatalf("a stamp %q was applied with no --raised-by flag", l)
		}
	}
	if anyCall(ghCalls(*calls), "label", "list") {
		t.Fatalf("probed for a label with no stamp requested; gh calls: %v", ghCalls(*calls))
	}
	if !strings.Contains(out, "UNKNOWN provenance") {
		t.Fatalf("an unstamped filing must SAY it is unattributed (no silent caps); out=%s", out)
	}
	if !strings.Contains(out, "not 'human-raised'") && !strings.Contains(out, "NOT 'human-raised'") {
		t.Fatalf("the NOTICE must state that unknown is not human-raised, which is the whole "+
			"invariant the metric turns on; out=%s", out)
	}
	if d := lastNewDetail(t); !strings.Contains(d, "raised-by=UNSTAMPED:not-requested") {
		t.Fatalf("audit detail = %q, want raised-by=UNSTAMPED:not-requested", d)
	}
}

// --- outcome 3: label-missing -----------------------------------------------------

// TestRaisedByMissingLabelFilesAnywayWithARemedy is the mergedstatus.go precedent applied:
// the labels do not exist in ANY repo yet, so refusing here would have made the flag
// unusable on the day it shipped and taught the fleet to stop passing it. The filing goes
// through UNSTAMPED, and the NOTICE carries the one-off create command.
func TestRaisedByMissingLabelFilesAnywayWithARemedy(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, "bug", "question")) // no raised-by:* at all
	body := bodyFileWith(t, "a verification failure worth filing")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "verify gate failed on a stale evidence table", "--body-file", body,
		"--raised-by", "verifier"})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0 — a missing METRIC label must never stop a filing; out=%s", rc, out)
	}
	if argv := createArgv(*calls); argv == nil {
		t.Fatalf("the issue was not created; gh calls: %v", ghCalls(*calls))
	} else if hasString(labelArgs(argv), "raised-by:verifier") {
		t.Fatalf("a non-existent label was passed to `gh issue create`, which would have failed "+
			"the whole filing: %v", labelArgs(argv))
	}
	if !strings.Contains(out, "gh label create raised-by:verifier") {
		t.Fatalf("the NOTICE must carry the exact remedy command; out=%s", out)
	}
	if !strings.Contains(out, "UNKNOWN provenance") {
		t.Fatalf("the NOTICE must say what the issue now reads as; out=%s", out)
	}
	if d := lastNewDetail(t); !strings.Contains(d, "raised-by=UNSTAMPED:label-missing") {
		t.Fatalf("audit detail = %q, want raised-by=UNSTAMPED:label-missing", d)
	}
}

// --- outcome 4: could-not-check ---------------------------------------------------

// TestRaisedByProbeOutageIsCouldNotCheck — the third state. An unanswered probe is NOT
// "the label is absent": both drop the stamp, but they need different remedies, and a
// caller told to create a label during an API outage creates one that already exists and
// is still not stamped.
func TestRaisedByProbeOutageIsCouldNotCheck(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABEL_FAIL", "1")
	body := bodyFileWith(t, "filed during a label-api outage")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "desk observed a broken alerter during an outage", "--body-file", body,
		"--raised-by", "desk"})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0 — a metric probe outage must not block a filing; out=%s", rc, out)
	}
	if argv := createArgv(*calls); argv == nil {
		t.Fatalf("the issue was not created; gh calls: %v", ghCalls(*calls))
	} else if hasString(labelArgs(argv), "raised-by:desk") {
		t.Fatalf("stamped on an UNANSWERED probe — that is a guess: %v", labelArgs(argv))
	}
	if !strings.Contains(out, "could not check") {
		t.Fatalf("the NOTICE must name the could-not-check state; out=%s", out)
	}
	if strings.Contains(out, "does not exist") {
		t.Fatalf("an unanswered probe was reported as a MISSING label — the two states need "+
			"different remedies; out=%s", out)
	}
	if d := lastNewDetail(t); !strings.Contains(d, "raised-by=UNSTAMPED:could-not-check") {
		t.Fatalf("audit detail = %q, want raised-by=UNSTAMPED:could-not-check", d)
	}
}

// TestRaisedByEmptyProbeOutputIsCouldNotCheck — exit 0 with no stdout is an unanswered
// probe, not an empty label set. Reading it as "no labels exist" would report every repo
// as missing the label forever.
func TestRaisedByEmptyProbeOutputIsCouldNotCheck(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABEL_EMPTY", "1")
	body := bodyFileWith(t, "empty probe output")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "empty probe output is not an empty label set", "--body-file", body,
		"--raised-by", "desk"})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0; out=%s", rc, out)
	}
	if d := lastNewDetail(t); !strings.Contains(d, "raised-by=UNSTAMPED:could-not-check") {
		t.Fatalf("audit detail = %q, want could-not-check for empty probe output", d)
	}
}

// --- the ONE refusal --------------------------------------------------------------

// TestRaisedByUnknownRoleRefusesBeforeAnyWrite — the single raised-by condition that
// refuses. It is a caller error with a fix in hand, and stamping it would mint a metric
// category nothing will ever populate again. The refusal has to land BEFORE the dedupe
// search and before any write.
func TestRaisedByUnknownRoleRefusesBeforeAnyWrite(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	body := bodyFileWith(t, "a filing naming a role nobody bound")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "an invented desk name reaches the label", "--body-file", body,
		"--raised-by", "pr-review-desk"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("unknown-role rc = %d, want 5; out=%s", rc, out)
	}
	assertNoMutatingGH(t, *calls)
	if anyCall(ghCalls(*calls), "search") {
		t.Fatalf("the role was validated AFTER the dedupe search — a caller error should not "+
			"spend an API call; gh calls: %v", ghCalls(*calls))
	}
	for _, want := range []string{"reviewer", "verifier", "worker"} {
		if !strings.Contains(out, want) {
			t.Errorf("the refusal must enumerate the bound roles (missing %q); out=%s", want, out)
		}
	}
}

// TestRaisedByEmptyValueIsTreatedAsOmission — `--raised-by ""` is not a role and not a
// blank stamp; it is the same as not passing the flag. The alternative (refusing) would
// break a skill that interpolates an empty variable, for no gain.
func TestRaisedByEmptyValueIsTreatedAsOmission(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	body := bodyFileWith(t, "an interpolated empty role")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "an empty interpolated role variable", "--body-file", body,
		"--raised-by", "  "})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0; out=%s", rc, out)
	}
	for _, l := range labelArgs(createArgv(*calls)) {
		if strings.HasPrefix(strings.ToLower(l), deskkit.RaisedByPrefix) {
			t.Fatalf("a blank role produced a stamp %q", l)
		}
	}
	if d := lastNewDetail(t); !strings.Contains(d, "raised-by=UNSTAMPED:not-requested") {
		t.Fatalf("audit detail = %q, want raised-by=UNSTAMPED:not-requested", d)
	}
}

// --- the audit line is the corpus record ------------------------------------------

// TestRaisedByNoteDoesNotUnchargeTheBudget is the interaction bug this ordering exists to
// prevent: chargedNewEntry discriminates on createSentMarker being the PREFIX of Detail,
// so a raised-by note written in FRONT of it would silently stop a sent-but-unconfirmed
// create from charging session budget — a metric annotation reopening a filing budget.
func TestRaisedByNoteDoesNotUnchargeTheBudget(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_CREATE_FAIL", "1")
	body := bodyFileWith(t, "a create that was sent and could not be confirmed")

	rc, _ := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "an unconfirmable create carrying a stamp note", "--body-file", body,
		"--raised-by", "worker"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("rc = %d, want 6", rc)
	}
	entries := auditEntriesFor(t, "new")
	e := entries[len(entries)-1]
	if !strings.HasPrefix(e.Detail, createSentMarker) {
		t.Fatalf("Detail = %q — the create-sent marker must stay the PREFIX, or the budget "+
			"stops charging sent creates", e.Detail)
	}
	if !chargedNewEntry(e) {
		t.Fatal("a SENT create stopped charging session budget once a raised-by note was added — " +
			"the stamp must not be able to reopen the filing budget")
	}
	if !strings.Contains(e.Detail, "raised-by=") {
		t.Fatalf("Detail = %q — the stamp outcome must be recorded on the failure path too", e.Detail)
	}
}

// TestRaisedByEveryOutcomeIsDistinguishableInTheAudit — the four outcomes must be four
// distinct tokens. Collapsing the three unstamped ones into a single "unstamped" would
// hide WHICH remedy the corpus needs, which is the question the backfill decision turns
// on.
func TestRaisedByEveryOutcomeIsDistinguishableInTheAudit(t *testing.T) {
	seen := map[string]bool{}
	cases := []struct {
		name   string
		labels string
		fail   string
		role   string
		want   string
	}{
		{"stamped", labelsJSON(t, "raised-by:worker"), "", "worker", "raised-by=worker"},
		{"omitted", "[]", "", "", stampOutcomeOmitted},
		{"missing", labelsJSON(t, "bug"), "", "worker", stampOutcomeNoLabel},
		{"outage", "[]", "1", "worker", stampOutcomeUnchecked},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withEnv(t)
			t.Setenv("FAKEGH_SEARCH_HITS", "[]")
			t.Setenv("FAKEGH_LABELS", tc.labels)
			t.Setenv("FAKEGH_LABEL_FAIL", tc.fail)
			body := bodyFileWith(t, "distinguishable outcomes "+tc.name)
			args := []string{"new", "-R", allowedRepo,
				"--title", "distinguishable stamp outcome " + tc.name, "--body-file", body}
			if tc.role != "" {
				args = append(args, "--raised-by", tc.role)
			}
			if rc, out := runCapture(args); rc != deskkit.ExitOK {
				t.Fatalf("rc = %d, want 0; out=%s", rc, out)
			}
			d := lastNewDetail(t)
			if !strings.Contains(d, tc.want) {
				t.Fatalf("audit detail = %q, want it to carry %q", d, tc.want)
			}
			if seen[tc.want] {
				t.Fatalf("outcome token %q is not unique across the four outcomes", tc.want)
			}
			seen[tc.want] = true
		})
	}
	if len(seen) != 4 {
		t.Fatalf("recorded %d distinct outcome tokens, want 4", len(seen))
	}
}

// TestRaisedByStampNeverReachesAttachOrCheck — the stamp belongs to `new` alone. `attach`
// posts a comment on an issue somebody else's filing already attributed, and `check`
// writes nothing at all.
func TestRaisedByStampNeverReachesAttachOrCheck(t *testing.T) {
	calls := withEnv(t)
	body := bodyFileWith(t, "an observation attached to a class issue")
	if rc, _ := runCapture([]string{"attach", "-R", allowedRepo, "--to", "11",
		"--body-file", body, "--raised-by", "worker"}); rc != deskkit.ExitRefused {
		t.Fatalf("attach accepted --raised-by; the stamp is a property of the FILING, not of a "+
			"comment on somebody else's; gh calls: %v", ghCalls(*calls))
	}
	calls2 := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	if rc, _ := runCapture([]string{"check", "-R", allowedRepo,
		"--title", "a title being checked", "--raised-by", "worker"}); rc != deskkit.ExitRefused {
		t.Fatalf("check accepted --raised-by; gh calls: %v", ghCalls(*calls2))
	}
}
