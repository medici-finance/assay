package main

// triage_test.go — the triage-close lane (issue #1207).
//
// Every refusal ships with a positive control beside it: "the lane refused" is proved by
// the absence of a write (assertNoWrites), and "the lane closed" by exactly [comment, close]
// on the one item, carrying the not_planned state reason (assertClosedWith). The two
// authorities are tested apart: not-planned authorizes on a RECORDED disposition (a marker
// comment or --tracker), human-decided on the SAME fetched-and-verified R-1 sign-off the
// ruled lanes use plus a named tracker.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const triageIssue = 120 // an open, undecided idea-issue — the triage subject

// triageWorld is baseWorld plus an open, unlabelled idea-issue at triageIssue. baseWorld's
// signed rulings register and planted R-1 sign-off (by the blessing authority) are what the
// human-decided disposition verifies against.
func triageWorld(t *testing.T) (*stubRemote, string) {
	t.Helper()
	s, rul := baseWorld(t)
	s.items[fmt.Sprintf("%s#%d", testRepo, triageIssue)] = issueJSON(triageIssue, "open", nil, "an idea worth triaging")
	return s, rul
}

// ---------------------------------------------------------------- (a) not-planned, recorded

// TestTriageNotPlannedCloses — #1207 test (a): not-planned closes when the triage
// disposition is already on the record (the marker comment) OR a --tracker names the
// residual work. Either signal → exactly [comment, close] as not planned.
func TestTriageNotPlannedCloses(t *testing.T) {
	t.Run("recorded triage-disposition marker comment", func(t *testing.T) {
		s, rul := triageWorld(t)
		s.plantComment(testRepo, triageIssue, blessLogin,
			triageDispositionMarker+"\nrejected — out of scope for this stream\n")
		code, out := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
			"--disposition", dispositionNotPlanned, "--rulings", rul)
		if code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d\n%s", code, out)
		}
		assertClosedWith(t, s, reasonNotPlanned)
		if !strings.Contains(strings.ToLower(commentBody(t, s)), "recorded triage disposition") {
			t.Fatalf("close comment should name the recorded disposition:\n%s", commentBody(t, s))
		}
	})

	t.Run("--tracker names the residual work", func(t *testing.T) {
		s, rul := triageWorld(t)
		code, out := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
			"--disposition", dispositionNotPlanned, "--tracker", "#40", "--rulings", rul)
		if code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d\n%s", code, out)
		}
		assertClosedWith(t, s, reasonNotPlanned)
		if !strings.Contains(commentBody(t, s), testRepo+"#40") {
			t.Fatalf("close comment should name the tracker:\n%s", commentBody(t, s))
		}
	})
}

// ---------------------------------------------------------------- (b) not-planned, unrecorded

// TestTriageNotPlannedRefusedWithoutRecord — #1207 test (b): not-planned with neither a
// disposition record nor a tracker is refused exit 5, and writes nothing.
func TestTriageNotPlannedRefusedWithoutRecord(t *testing.T) {
	s, rul := triageWorld(t)
	code, out := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
		"--disposition", dispositionNotPlanned, "--rulings", rul)
	if code != deskkit.ExitRefused {
		t.Fatalf("want exit 5 refused, got %d\n%s", code, out)
	}
	assertNoWrites(t, s)
}

// ---------------------------------------------------------------- (c) human-decided, no ruling

// TestTriageHumanDecidedRefusedWithoutRuling — #1207 test (c): human-decided is refused
// when the R-1 sign-off is not a VERIFIED human ruling. Two shapes, both refused, both write
// nothing: a sign-off authored by a trusted-but-not-blessing-authority account (the shared
// automation login that reports type=User exactly as a person does), and an unsigned register.
func TestTriageHumanDecidedRefusedWithoutRuling(t *testing.T) {
	t.Run("sign-off authored by a non-authority (trusted automation)", func(t *testing.T) {
		s, rul := triageWorld(t)
		// Replace the blessing-authority sign-off with one from the shared automation login.
		s.comment[signOffCID] = commentJSON(sharedLogin, sharedID, "User",
			"accepted.", "https://api.github.com/repos/"+testRepo+"/issues/297")
		code, out := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
			"--disposition", dispositionHumanDecided, "--tracker", "#40", "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5 refused, got %d\n%s", code, out)
		}
		assertNoWrites(t, s)
	})

	t.Run("unsigned register", func(t *testing.T) {
		s, _ := triageWorld(t)
		// A register present but with a non-URL sign-off is UNSIGNED — a positive "the human
		// has not granted this" determination, refused exit 5 (an empty PATH would instead be
		// could-not-check, exit 6, a different epistemic state).
		unsigned := signedRulings(t, "_(not yet signed)_")
		code, out := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
			"--disposition", dispositionHumanDecided, "--tracker", "#40", "--rulings", unsigned)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5 refused, got %d\n%s", code, out)
		}
		assertNoWrites(t, s)
	})
}

// TestTriageHumanDecidedCloses is the positive control for (c): a verified R-1 sign-off AND a
// named tracker close the item as not planned, citing the ruling.
func TestTriageHumanDecidedCloses(t *testing.T) {
	s, rul := triageWorld(t)
	code, out := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
		"--disposition", dispositionHumanDecided, "--tracker", "#40", "--rulings", rul)
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d\n%s", code, out)
	}
	assertClosedWith(t, s, reasonNotPlanned)
	body := commentBody(t, s)
	for _, want := range []string{rulingID, testRepo + "#40"} {
		if !strings.Contains(body, want) {
			t.Fatalf("close comment should cite %q:\n%s", want, body)
		}
	}
}

// TestTriageHumanDecidedRequiresTracker — human-decided with a verified ruling but no tracker
// is refused: the recorded-decision close must NAME the remaining work, never assert it.
func TestTriageHumanDecidedRequiresTracker(t *testing.T) {
	s, rul := triageWorld(t)
	code, out := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
		"--disposition", dispositionHumanDecided, "--rulings", rul)
	if code != deskkit.ExitRefused {
		t.Fatalf("want exit 5 refused, got %d\n%s", code, out)
	}
	assertNoWrites(t, s)
}

// ---------------------------------------------------------------- (d) already closed

// TestTriageAlreadyClosedIsNoop — #1207 test (d): an already-closed issue is an idempotent
// no-op success, and writes nothing.
func TestTriageAlreadyClosedIsNoop(t *testing.T) {
	s, rul := triageWorld(t)
	s.items[fmt.Sprintf("%s#%d", testRepo, triageIssue)] = issueJSON(triageIssue, "closed", nil, "")
	code, out := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
		"--disposition", dispositionNotPlanned, "--tracker", "#40", "--rulings", rul)
	if code != deskkit.ExitOK || !strings.Contains(out, "noop") {
		t.Fatalf("want exit 0 noop, got %d\n%s", code, out)
	}
	assertNoWrites(t, s)
}

// ---------------------------------------------------------------- (e) PR target

// TestTriagePRTargetRefused — #1207 test (e): a pull-request target is refused. A PR is
// retired through the finding-based lanes, never as a plain triage skip.
func TestTriagePRTargetRefused(t *testing.T) {
	s, rul := triageWorld(t)
	// mergedPRNum (40) is registered by baseWorld as a pull request.
	code, out := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(mergedPRNum),
		"--disposition", dispositionNotPlanned, "--tracker", "#55", "--rulings", rul)
	if code != deskkit.ExitRefused {
		t.Fatalf("want exit 5 refused, got %d\n%s", code, out)
	}
	assertNoWrites(t, s)
}

// ---------------------------------------------------------------- decision-label control

// TestTriageDecisionLabelControl is the worker §2 control: triage never closes a
// needs-decision issue (both dispositions), and not-planned also refuses a human-decided
// item; human-decided PERMITS a human-decided-labelled item (that is what it closes).
func TestTriageDecisionLabelControl(t *testing.T) {
	t.Run("needs-decision refused under not-planned", func(t *testing.T) {
		s, rul := triageWorld(t)
		s.items[fmt.Sprintf("%s#%d", testRepo, triageIssue)] =
			issueJSON(triageIssue, "open", []string{"needs-decision"}, "")
		code, _ := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
			"--disposition", dispositionNotPlanned, "--tracker", "#40", "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5 refused, got %d", code)
		}
		assertNoWrites(t, s)
	})

	t.Run("needs-decision refused under human-decided", func(t *testing.T) {
		s, rul := triageWorld(t)
		s.items[fmt.Sprintf("%s#%d", testRepo, triageIssue)] =
			issueJSON(triageIssue, "open", []string{"needs-decision"}, "")
		code, _ := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
			"--disposition", dispositionHumanDecided, "--tracker", "#40", "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5 refused, got %d", code)
		}
		assertNoWrites(t, s)
	})

	t.Run("human-decided label refused under not-planned", func(t *testing.T) {
		s, rul := triageWorld(t)
		s.items[fmt.Sprintf("%s#%d", testRepo, triageIssue)] =
			issueJSON(triageIssue, "open", []string{"human-decided"}, "")
		code, _ := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
			"--disposition", dispositionNotPlanned, "--tracker", "#40", "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5 refused, got %d", code)
		}
		assertNoWrites(t, s)
	})

	t.Run("human-decided label PERMITTED under human-decided", func(t *testing.T) {
		s, rul := triageWorld(t)
		s.items[fmt.Sprintf("%s#%d", testRepo, triageIssue)] =
			issueJSON(triageIssue, "open", []string{"human-decided"}, "")
		code, out := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
			"--disposition", dispositionHumanDecided, "--tracker", "#40", "--rulings", rul)
		if code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d\n%s", code, out)
		}
		assertClosedWith(t, s, reasonNotPlanned)
	})
}

// ---------------------------------------------------------------- arg surface + dry-run

// TestTriageArgSurface pins the closed disposition set and the required flag.
func TestTriageArgSurface(t *testing.T) {
	s, rul := triageWorld(t)
	t.Run("missing --disposition refused", func(t *testing.T) {
		code, _ := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue), "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d", code)
		}
		assertNoWrites(t, s)
	})
	t.Run("unknown --disposition refused", func(t *testing.T) {
		code, _ := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
			"--disposition", "maybe-later", "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d", code)
		}
		assertNoWrites(t, s)
	})
	t.Run("stated change kind refused", func(t *testing.T) {
		code, _ := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
			"--disposition", dispositionNotPlanned, "--kind", "mr", "--tracker", "#40", "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d", code)
		}
		assertNoWrites(t, s)
	})
}

// TestTriageDryRunWritesNothing — --dry-run validates and reads, writes nothing.
func TestTriageDryRunWritesNothing(t *testing.T) {
	s, rul := triageWorld(t)
	code, out := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
		"--disposition", dispositionNotPlanned, "--tracker", "#40", "--dry-run", "--rulings", rul)
	if code != deskkit.ExitOK || !strings.Contains(out, "dry-run") {
		t.Fatalf("want exit 0 dry-run, got %d\n%s", code, out)
	}
	assertNoWrites(t, s)
}
