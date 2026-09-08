package loopengine

import (
	"errors"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// reconcile_test.go — offline unit tests for the eligibility reconciliation. Every read is an
// injected stub, so no forge and no git are touched: the Verify row is
// `go test ./internal/loopengine/ -run 'Eligib|Reconcile' -count=1`.

// stubReaders builds an EligibilityReaders whose three reads return the given values (and no
// error). A test that wants a read to fail (could-not-check) overrides that one field.
func stubReaders(holder string, st Status, pr PRState) EligibilityReaders {
	return EligibilityReaders{
		Claim:    func(string) (ClaimRecord, error) { return ClaimRecord{Holder: holder}, nil },
		BoardRow: func(string, string) (Status, error) { return st, nil },
		PR:       func(string, int) (PRState, error) { return pr, nil },
	}
}

// aliveClaim is a claim held by "owner", used as the reconciled record in every case.
func aliveClaim() ClaimRecord {
	return ClaimRecord{Key: "s--01", Item: "s/01", Root: ".", Repo: "o/r", PR: 7, Holder: "owner"}
}

func TestEligibility_EligibleLiveRun(t *testing.T) {
	v, err := Eligibility(aliveClaim(), stubReaders("owner", "in-progress", PRState{Kind: deskkit.PROpen}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !v.Eligible() {
		t.Fatalf("an open PR, active row, still-held claim must be Eligible, got %+v", v)
	}
}

func TestEligibility_PRMergedIsTerminal(t *testing.T) {
	v, err := Eligibility(aliveClaim(), stubReaders("owner", "in-progress", PRState{Kind: deskkit.PRMerged}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !v.Terminal() || v.Reason != "pr-merged" {
		t.Fatalf("a merged PR must be IneligibleTerminal(pr-merged), got %+v", v)
	}
}

func TestEligibility_PRClosedIsTerminal(t *testing.T) {
	v, err := Eligibility(aliveClaim(), stubReaders("owner", "in-progress", PRState{Kind: deskkit.PRClosed}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !v.Terminal() || v.Reason != "pr-closed" {
		t.Fatalf("a closed PR must be IneligibleTerminal(pr-closed), got %+v", v)
	}
}

func TestEligibility_BoardRowFinishedIsTerminal(t *testing.T) {
	// implemented/verified/done are FINISHED for this run — Terminal (stop + release).
	for _, st := range []Status{"implemented", "verified", "done"} {
		v, err := Eligibility(aliveClaim(), stubReaders("owner", st, PRState{Kind: deskkit.PROpen}))
		if err != nil {
			t.Fatalf("status %q: unexpected error: %v", st, err)
		}
		if !v.Terminal() || v.Reason != "board-row-"+string(st) {
			t.Fatalf("a %q board row must be IneligibleTerminal(board-row-%s), got %+v", st, st, v)
		}
	}
}

func TestEligibility_BoardRowBlockedIsHeld(t *testing.T) {
	// blocked is a human-hold state — a human owns the next move, so STOP without release.
	v, err := Eligibility(aliveClaim(), stubReaders("owner", "blocked", PRState{Kind: deskkit.PROpen}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !v.Held() || v.Reason != "board-row-blocked" {
		t.Fatalf("a blocked board row must be IneligibleHeld(board-row-blocked) (stop, do NOT release), got %+v", v)
	}
}

func TestEligibility_ClaimReleasedIsTerminal(t *testing.T) {
	v, err := Eligibility(aliveClaim(), stubReaders("", "in-progress", PRState{Kind: deskkit.PROpen}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !v.Terminal() || v.Reason != "claim-released" {
		t.Fatalf("a released claim must be IneligibleTerminal(claim-released), got %+v", v)
	}
}

func TestEligibility_ClaimReassignedIsHeld(t *testing.T) {
	// A claim moved to a DIFFERENT live holder must be HELD, not Terminal: releasing it would
	// delete the new holder's live ref and re-free an item they are working (double-dispatch).
	v, err := Eligibility(aliveClaim(), stubReaders("someone-else", "in-progress", PRState{Kind: deskkit.PROpen}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !v.Held() || v.Reason != "claim-reassigned" {
		t.Fatalf("a stolen claim must be IneligibleHeld(claim-reassigned) (stop, do NOT release), got %+v", v)
	}
}

func TestEligibility_DispositionIsHeld(t *testing.T) {
	cases := []struct {
		disp   deskkit.DispositionVerdict
		reason string
	}{
		{deskkit.DispositionSuperseded, "superseded"},
		{deskkit.DispositionResolvedElsewhere, "resolved-elsewhere"},
	}
	for _, tc := range cases {
		v, err := Eligibility(aliveClaim(), stubReaders("owner", "in-progress", PRState{Kind: deskkit.PROpen, Disposition: tc.disp}))
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.disp, err)
		}
		if !v.Held() || v.Reason != tc.reason {
			t.Fatalf("disposition %s must be IneligibleHeld(%s) (stop, do NOT release), got %+v", tc.disp, tc.reason, v)
		}
	}
}

func TestEligibility_HeldLabelsAreHeld(t *testing.T) {
	for _, label := range []string{"needs-decision", "question", "Needs-Decision"} {
		v, err := Eligibility(aliveClaim(), stubReaders("owner", "in-progress", PRState{Kind: deskkit.PROpen, Labels: []string{"area:desk", label}}))
		if err != nil {
			t.Fatalf("label %q: unexpected error: %v", label, err)
		}
		if !v.Held() {
			t.Fatalf("a %q label must be IneligibleHeld (stop, do NOT release), got %+v", label, v)
		}
	}
}

func TestEligibility_BlindClaimKeepsRun(t *testing.T) {
	r := stubReaders("owner", "in-progress", PRState{Kind: deskkit.PROpen})
	r.Claim = func(string) (ClaimRecord, error) { return ClaimRecord{}, errors.New("forge down") }
	v, err := Eligibility(aliveClaim(), r)
	if v.Ineligible() {
		t.Fatalf("a blind read must not yield an ineligible verdict, got %+v", v)
	}
	src, ok := BlindSource(err)
	if !ok || src != "claim" {
		t.Fatalf("expected BlindError(source=claim), got %v (ok=%v src=%q)", err, ok, src)
	}
}

func TestEligibility_BlindBoardKeepsRun(t *testing.T) {
	r := stubReaders("owner", "in-progress", PRState{Kind: deskkit.PROpen})
	r.BoardRow = func(string, string) (Status, error) { return "", errors.New("origin/main unreadable") }
	_, err := Eligibility(aliveClaim(), r)
	src, ok := BlindSource(err)
	if !ok || src != "board-row" {
		t.Fatalf("expected BlindError(source=board-row), got %v (ok=%v src=%q)", err, ok, src)
	}
}

func TestEligibility_BlindPRKeepsRun(t *testing.T) {
	r := stubReaders("owner", "in-progress", PRState{})
	r.PR = func(string, int) (PRState, error) { return PRState{}, errors.New("PR read failed") }
	_, err := Eligibility(aliveClaim(), r)
	src, ok := BlindSource(err)
	if !ok || src != "pr" {
		t.Fatalf("expected BlindError(source=pr), got %v (ok=%v src=%q)", err, ok, src)
	}
}

// TestReconcile_BoardCheckedBeforePR is the defense-in-depth ordering pin the SPOF note
// relies on: a brief that flipped is caught even when the PR read WOULD blind. If the reads
// were ordered PR-first, this would return BlindError(pr) instead of the terminal verdict.
func TestReconcile_BoardCheckedBeforePR(t *testing.T) {
	r := stubReaders("owner", "implemented", PRState{})
	r.PR = func(string, int) (PRState, error) {
		return PRState{}, errors.New("PR read failed — must never be reached")
	}
	v, err := Eligibility(aliveClaim(), r)
	if err != nil {
		t.Fatalf("a flipped board row must short-circuit BEFORE the failing PR read, got error %v", err)
	}
	if !v.Terminal() || v.Reason != "board-row-implemented" {
		t.Fatalf("expected IneligibleTerminal(board-row-implemented), got %+v", v)
	}
}

// TestReconcile_ClaimCheckedFirst pins the claim read as first: a stolen claim is terminal
// even when the board read WOULD blind.
func TestReconcile_ClaimCheckedFirst(t *testing.T) {
	r := stubReaders("someone-else", "in-progress", PRState{Kind: deskkit.PROpen})
	r.BoardRow = func(string, string) (Status, error) {
		return "", errors.New("board unreadable — must never be reached")
	}
	v, err := Eligibility(aliveClaim(), r)
	if err != nil {
		t.Fatalf("a stolen claim must short-circuit BEFORE the failing board read, got error %v", err)
	}
	if !v.Held() || v.Reason != "claim-reassigned" {
		t.Fatalf("expected IneligibleHeld(claim-reassigned), got %+v", v)
	}
}

func TestReconcileParseBriefRowStatus(t *testing.T) {
	readme := `# Stream

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [First](brief-01.md) | 0 | M | done | 2026-09-04 v | 2026-09-04 r |
| 03 | [Third](brief-03.md) | 2 | M | in-progress | — | — |
`
	got, err := ParseBriefRowStatus(readme, "example-stream/03")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "in-progress" {
		t.Fatalf("status = %q, want in-progress", got)
	}
	if _, err := ParseBriefRowStatus(readme, "example-stream/99"); err == nil {
		t.Fatal("a missing row must be an error (could-not-check), not a guessed status")
	}
}
