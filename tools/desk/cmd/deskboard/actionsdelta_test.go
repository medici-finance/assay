package main

// actionsdelta_test.go — the `actions` --delta/--quiet extractor (the review-desk
// responsiveness fix).
//
// The defect these tests pin: the desk's quiet cadence loop could not run the
// CLASSIFIED board. `--delta`/`--quiet` existed only on prs/queue/nextup — verbs with
// no review state (`prs` labels its own count "ci-red/conflicting … see `actions`") —
// and `actions` refused both flags at exit 5. So a desk observing the console noise
// floor had NO quiet sweep that could say "a PR needs a reviewer", and the first loud
// signal it got was the UNREVIEWED neglect banner at the threshold age: the 2h neglect
// alarm had become the de-facto review trigger. Fail-first evidence for the e2e test
// below: on the unfixed tree, `actions --quiet` exits 5 ("--delta/--quiet are
// supported on prs, queue, and nextup only") and the test is red.

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestActionsDeltaSetSummary pins the extractor's three claims: Actionable counts
// exactly NEEDS-REVIEW + RE-REVIEW (the dispatch gate — never a bare len(rows)),
// the label names that subset, and the summary restates the standing MERGE-NOW count
// every sweep (the standing surface-duty --delta alone cannot satisfy: an unchanged row is
// silent after its first sighting).
func TestActionsDeltaSetSummary(t *testing.T) {
	rep := actionsReport{Rows: []actionRow{
		{Repo: "x/y", Number: 1, Action: actNeedsReview},
		{Repo: "x/y", Number: 2, Action: actNeedsReview},
		{Repo: "x/y", Number: 3, Action: actReReview},
		{Repo: "x/y", Number: 4, Action: actMergeNow},
		{Repo: "x/y", Number: 5, Action: actReady},
	}, External: []externalRow{{Repo: "x/y", Number: 9, Author: "stranger", Title: "ext"}}}
	ds, ok := actionsDeltaSet(rep)
	if !ok {
		t.Fatal("actionsDeltaSet returned false")
	}
	if len(ds.Items) != 6 { // 5 rows + 1 quarantined
		t.Errorf("items = %d, want 6", len(ds.Items))
	}
	if ds.Actionable != 3 { // 2 NEEDS-REVIEW + 1 RE-REVIEW; MERGE-NOW/READY excluded
		t.Errorf("actionable = %d, want 3 (NEEDS-REVIEW + RE-REVIEW only)", ds.Actionable)
	}
	if !strings.Contains(ds.ActionableLabel, "NEEDS-REVIEW/RE-REVIEW") {
		t.Errorf("label must name the dispatch gate; got %q", ds.ActionableLabel)
	}
	for _, want := range []string{"2 NEEDS-REVIEW", "1 RE-REVIEW", "1 MERGE-NOW", "1 other"} {
		if !strings.Contains(ds.Summary, want) {
			t.Errorf("summary %q missing %q", ds.Summary, want)
		}
	}
}

// TestActionsDeltaSet_AlarmsRideTheSummary: the UNREVIEWED neglect alarm and the
// MERGE-NOW decay alarm are standing state, not transitions — they must appear in the
// quiet line's summary on EVERY sweep while active, because --delta silences an
// unchanged row after its first sighting.
func TestActionsDeltaSet_AlarmsRideTheSummary(t *testing.T) {
	rep := actionsReport{Rows: []actionRow{{Repo: "x/y", Number: 1, Action: actNeedsReview}}}
	rep.Header.UnreviewedCount = 2
	rep.Header.UnreviewedThreshold = "30m0s"
	rep.Header.UnreviewedAgeUnknownCount = 1
	rep.Header.MergeNowDecay = true
	rep.Header.MergeNowDecayPRs = []int{7}
	rep.Header.MergeNowThreshold = "20m0s"
	ds, ok := actionsDeltaSet(rep)
	if !ok {
		t.Fatal("actionsDeltaSet returned false")
	}
	for _, want := range []string{"UNREVIEWED 2", "30m0s", "UNREVIEWED-AGE-UNKNOWN 1", "DECAY 1"} {
		if !strings.Contains(ds.Summary, want) {
			t.Errorf("summary %q missing %q", ds.Summary, want)
		}
	}
	// And a clean board pays nothing: no alarm text when the counts are zero.
	clean, _ := actionsDeltaSet(actionsReport{Rows: []actionRow{{Repo: "x/y", Number: 1, Action: actReady}}})
	for _, absent := range []string{"UNREVIEWED", "DECAY"} {
		if strings.Contains(clean.Summary, absent) {
			t.Errorf("clean summary %q must not carry %q", clean.Summary, absent)
		}
	}
}

// TestActionsSignature_AgeTickDoesNotChurn: ages, score and note re-derive or tick on
// every sweep; if any of them entered the signature, every row would be "changed"
// every run and the delta channel would become the full board again. A CLASSIFICATION
// change (the action verb) must still change the signature.
func TestActionsSignature_AgeTickDoesNotChurn(t *testing.T) {
	base := actionRow{Repo: "x/y", Number: 1, Action: actMergeNow, CIPass: 1, ApprovedAge: "5m", OpenAge: "1h", Score: 100, Note: "n1"}
	aged := base
	aged.ApprovedAge, aged.OpenAge, aged.Score, aged.Note = "45m", "2h", 200, "n2"
	sigOf := func(r actionRow) string {
		ds, ok := actionsDeltaSet(actionsReport{Rows: []actionRow{r}})
		if !ok {
			t.Fatal("actionsDeltaSet returned false")
		}
		return ds.Items[0].Signature
	}
	if sigOf(base) != sigOf(aged) {
		t.Errorf("age/score/note tick changed the signature: %q vs %q", sigOf(base), sigOf(aged))
	}
	reclassified := base
	reclassified.Action = actReReview
	if sigOf(base) == sigOf(reclassified) {
		t.Errorf("an action change must change the signature; both %q", sigOf(base))
	}
}

// TestActionsQuiet_FreshPRSurfacesAsActionable is the end-to-end pin of the
// responsiveness claim: a PR opened MINUTES ago, with no reviewer verdict, surfaces in
// `actions --quiet` as an actionable NEEDS-REVIEW count — at exit 0, with no
// UNREVIEWED alarm (it is fresh; the alarm is for neglect, not a gate on surfacing).
// Fail-first: before `actions` joined deltaExtractors this invocation exited 5.
func TestActionsQuiet_FreshPRSurfacesAsActionable(t *testing.T) {
	installFakeGH(t)
	t.Setenv("DESKBOARD_GH_PR_REPO", "example-org/tracker")
	t.Setenv("DESKBOARD_GH_PRLIST_JSON", fmt.Sprintf(
		`[{"number":1,"title":"fresh","state":"OPEN","isDraft":true,"author":{"login":"shared-agent"},"createdAt":%q,"headRefOid":"abc123","baseRefName":"main","mergeStateStatus":"CLEAN","statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]}]`,
		time.Now().UTC().Add(-3*time.Minute).Format(time.RFC3339)))

	var out, errb bytes.Buffer
	code := run([]string{"actions", "--quiet"}, &out, &errb)
	if code != deskkit.ExitOK {
		t.Fatalf("exit=%d, want 0 — stderr: %s", code, errb.String())
	}
	line := out.String()
	if !strings.HasPrefix(line, "actions: ") {
		t.Fatalf("quiet line missing; got %q", line)
	}
	if !strings.Contains(line, "1 NEEDS-REVIEW") {
		t.Errorf("a 3-minute-old PR must surface as NEEDS-REVIEW in the quiet sweep; got %q", line)
	}
	if !strings.Contains(line, "NEEDS-REVIEW/RE-REVIEW") {
		t.Errorf("the actionable segment must carry the dispatch-gate label; got %q", line)
	}
	if strings.Contains(line, "UNREVIEWED ") {
		t.Errorf("a fresh PR is under the neglect threshold — no UNREVIEWED alarm expected; got %q", line)
	}
}
