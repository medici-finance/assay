package main

// triage_test.go — the triage-close lane (issue #1207), authority model per the security
// review of PR #1211 (S1/S2/S3):
//
//   - not-planned authorizes ONLY on a triage-disposition marker comment on issue N that is
//     (a) authored by a roster-trusted account and (b) not minimized. A bare marker string
//     any commenter could paste, a minimized marker, or a marker by an untrusted author do
//     NOT authorize (S2). --tracker is never authority and, when given, must name a FETCHED
//     existing item (S3).
//   - human-decided authorizes ONLY on a FETCHED, author-verified, per-issue human artifact:
//     the human's own ruling comment ON issue N (--decision <url>), verified via fetchComment
//     + verifyHumanAuthor against the roster-pinned blessing authority (S1). A blanket ruling
//     grant and a caller-typed --tracker never stand in for it.
//
// Every refusal ships with a positive control beside it, and "the lane refused" is proved by
// the absence of a write (assertNoWrites).

import (
	"fmt"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const triageIssue = 120 // an open, undecided idea-issue — the triage subject

// triageWorld is baseWorld plus an open, unlabelled idea-issue at triageIssue.
func triageWorld(t *testing.T) (*stubRemote, string) {
	t.Helper()
	s, rul := baseWorld(t)
	s.items[fmt.Sprintf("%s#%d", testRepo, triageIssue)] = issueJSON(triageIssue, "open", nil, "an idea worth triaging")
	return s, rul
}

// plantCom= plant a comment on issue n via the id-bearing s.comment path, so its author
// login+id, minimized flag and permalink are all set the way a real ListComments read carries
// them (the threads path used by plantComment/plantProposal sets neither id nor minimized).
func plantMarkerComment(s *stubRemote, n int, cid, login string, id int64, typ string, minimized bool, body string) {
	issueURL := fmt.Sprintf("https://api.github.com/repos/%s/issues/%d", testRepo, n)
	html := fmt.Sprintf("https://github.com/%s/issues/%d#issuecomment-%s", testRepo, n, cid)
	s.comment[cid] = fmt.Sprintf(
		`{"id":%s,"html_url":%q,"issue_url":%q,"body":%q,"minimized":%t,"user":{"login":%q,"id":%d,"type":%q}}`,
		cid, html, issueURL, body, minimized, login, id, typ)
}

// a trusted disposition marker (shared-agent is a roster-trusted automation login, id-pinned).
func plantTrustedMarker(s *stubRemote, n int) {
	plantMarkerComment(s, n, "6001", sharedLogin, sharedID, "User", false,
		triageDispositionMarker+"\nrejected — out of scope for this stream\n")
}

// the human's ruling comment ON issue n, authored by the blessing authority.
func plantDecision(s *stubRemote, n int, cid string) string {
	plantMarkerComment(s, n, cid, blessLogin, blessID, "User", false,
		"Decision: not pursuing this; the residual work is tracked in #40. — human ruling\n")
	return fmt.Sprintf("https://github.com/%s/issues/%d#issuecomment-%s", testRepo, n, cid)
}

// ---------------------------------------------------------------- (a) not-planned, recorded

// TestTriageNotPlannedCloses — a trusted, non-minimized disposition marker authorizes the
// close; a --tracker naming an existing item is recorded alongside it (never as authority).
func TestTriageNotPlannedCloses(t *testing.T) {
	t.Run("trusted marker comment authorizes", func(t *testing.T) {
		s, rul := triageWorld(t)
		plantTrustedMarker(s, triageIssue)
		code, out := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
			"--disposition", dispositionNotPlanned, "--rulings", rul)
		if code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d\n%s", code, out)
		}
		assertClosedWith(t, s, reasonNotPlanned)
		if !strings.Contains(strings.ToLower(commentBody(t, s)), "recorded triage disposition") {
			t.Fatalf("close comment should cite the recorded disposition:\n%s", commentBody(t, s))
		}
	})

	t.Run("trusted marker + existing --tracker", func(t *testing.T) {
		s, rul := triageWorld(t)
		plantTrustedMarker(s, triageIssue)
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

// TestTriageNotPlannedRefusedWithoutRecord — no marker → refused, writes nothing.
func TestTriageNotPlannedRefusedWithoutRecord(t *testing.T) {
	s, rul := triageWorld(t)
	code, out := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
		"--disposition", dispositionNotPlanned, "--rulings", rul)
	if code != deskkit.ExitRefused {
		t.Fatalf("want exit 5 refused, got %d\n%s", code, out)
	}
	assertNoWrites(t, s)
}

// TestTriageNotPlannedMarkerAuthor — S2: a marker does not authorize unless its author is
// roster-trusted and the comment is not minimized.
func TestTriageNotPlannedMarkerAuthor(t *testing.T) {
	t.Run("untrusted author's marker does not authorize", func(t *testing.T) {
		s, rul := triageWorld(t)
		plantMarkerComment(s, triageIssue, "6002", "drive-by-nobody", 999, "User", false,
			triageDispositionMarker+"\nrejected\n")
		code, out := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
			"--disposition", dispositionNotPlanned, "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5 refused, got %d\n%s", code, out)
		}
		assertNoWrites(t, s)
	})

	t.Run("minimized marker by a trusted author does not authorize", func(t *testing.T) {
		s, rul := triageWorld(t)
		plantMarkerComment(s, triageIssue, "6003", sharedLogin, sharedID, "User", true,
			triageDispositionMarker+"\nrejected\n")
		code, out := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
			"--disposition", dispositionNotPlanned, "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5 refused, got %d\n%s", code, out)
		}
		assertNoWrites(t, s)
	})
}

// TestTriageNotPlannedBareTrackerRefused — reviewer Probe C: --tracker alone (no marker) must
// NOT authorize. Non-existent tracker AND existing tracker both refuse when no marker is on
// the issue.
func TestTriageNotPlannedBareTrackerRefused(t *testing.T) {
	t.Run("non-existent tracker, no marker", func(t *testing.T) {
		s, rul := triageWorld(t)
		code, out := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
			"--disposition", dispositionNotPlanned, "--tracker", "#999999", "--rulings", rul)
		if code == deskkit.ExitOK {
			t.Fatalf("a bare --tracker must not authorize a close; got exit 0\n%s", out)
		}
		assertNoWrites(t, s)
	})
	t.Run("existing tracker, no marker", func(t *testing.T) {
		s, rul := triageWorld(t)
		code, out := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
			"--disposition", dispositionNotPlanned, "--tracker", "#40", "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5 refused (no disposition record), got %d\n%s", code, out)
		}
		assertNoWrites(t, s)
	})
}

// TestTriageNotPlannedTrackerMustExist — S3: when a marker DOES authorize, a --tracker that
// names a non-existent item is still refused; a tracker is a fetched existing item.
func TestTriageNotPlannedTrackerMustExist(t *testing.T) {
	s, rul := triageWorld(t)
	plantTrustedMarker(s, triageIssue)
	code, out := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
		"--disposition", dispositionNotPlanned, "--tracker", "#999999", "--rulings", rul)
	if code == deskkit.ExitOK {
		t.Fatalf("a non-existent --tracker must be refused; got exit 0\n%s", out)
	}
	assertNoWrites(t, s)
}

// ---------------------------------------------------------------- (c) human-decided

// TestTriageHumanDecidedNoArtifactRefused — reviewer Probe B: human-decided with no per-issue
// human artifact (no --decision) is refused, even with a signed rulings register and a valid
// --tracker. A blanket grant is not a decision about issue N.
func TestTriageHumanDecidedNoArtifactRefused(t *testing.T) {
	s, rul := triageWorld(t)
	code, out := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
		"--disposition", dispositionHumanDecided, "--tracker", "#40", "--rulings", rul)
	if code != deskkit.ExitRefused {
		t.Fatalf("want exit 5 refused (no per-issue human artifact), got %d\n%s", code, out)
	}
	assertNoWrites(t, s)
}

// TestTriageHumanDecidedArtifactChecks — the per-issue artifact must be authored by the
// blessing authority and be ON the issue being closed.
func TestTriageHumanDecidedArtifactChecks(t *testing.T) {
	t.Run("decision by a non-authority (trusted automation) is refused", func(t *testing.T) {
		s, rul := triageWorld(t)
		plantMarkerComment(s, triageIssue, "7001", sharedLogin, sharedID, "User", false,
			"Decision: skip. — not the blessing authority\n")
		url := fmt.Sprintf("https://github.com/%s/issues/%d#issuecomment-7001", testRepo, triageIssue)
		code, out := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
			"--disposition", dispositionHumanDecided, "--decision", url, "--tracker", "#40", "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5 refused, got %d\n%s", code, out)
		}
		assertNoWrites(t, s)
	})

	t.Run("decision on a DIFFERENT issue is refused", func(t *testing.T) {
		s, rul := triageWorld(t)
		// A genuine blessing-authority ruling, but on issue subjectIssue (55), not triageIssue.
		url := plantDecision(s, subjectIssue, "7002")
		code, out := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
			"--disposition", dispositionHumanDecided, "--decision", url, "--tracker", "#40", "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5 refused, got %d\n%s", code, out)
		}
		assertNoWrites(t, s)
	})
}

// TestTriageHumanDecidedCloses — positive control: a fetched, author-verified decision ON the
// issue plus a named existing tracker closes as not planned, citing the decision (not R-1).
func TestTriageHumanDecidedCloses(t *testing.T) {
	s, rul := triageWorld(t)
	url := plantDecision(s, triageIssue, "7003")
	code, out := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
		"--disposition", dispositionHumanDecided, "--decision", url, "--tracker", "#40", "--rulings", rul)
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d\n%s", code, out)
	}
	assertClosedWith(t, s, reasonNotPlanned)
	body := commentBody(t, s)
	for _, want := range []string{"issuecomment-7003", testRepo + "#40", "recorded human decision"} {
		if !strings.Contains(strings.ToLower(body), strings.ToLower(want)) {
			t.Fatalf("close comment should cite %q:\n%s", want, body)
		}
	}
}

// TestTriageHumanDecidedRequiresTracker — human-decided with a verified decision but no
// tracker is refused: the close must NAME the continuing work.
func TestTriageHumanDecidedRequiresTracker(t *testing.T) {
	s, rul := triageWorld(t)
	url := plantDecision(s, triageIssue, "7004")
	code, out := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
		"--disposition", dispositionHumanDecided, "--decision", url, "--rulings", rul)
	if code != deskkit.ExitRefused {
		t.Fatalf("want exit 5 refused, got %d\n%s", code, out)
	}
	assertNoWrites(t, s)
}

// ---------------------------------------------------------------- (d) already closed

func TestTriageAlreadyClosedIsNoop(t *testing.T) {
	s, rul := triageWorld(t)
	plantTrustedMarker(s, triageIssue)
	s.items[fmt.Sprintf("%s#%d", testRepo, triageIssue)] = issueJSON(triageIssue, "closed", nil, "")
	code, out := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
		"--disposition", dispositionNotPlanned, "--rulings", rul)
	if code != deskkit.ExitOK || !strings.Contains(out, "noop") {
		t.Fatalf("want exit 0 noop, got %d\n%s", code, out)
	}
	assertNoWrites(t, s)
}

// ---------------------------------------------------------------- (e) PR target

func TestTriagePRTargetRefused(t *testing.T) {
	s, rul := triageWorld(t)
	// mergedPRNum (40) is registered by baseWorld as a pull request.
	code, out := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(mergedPRNum),
		"--disposition", dispositionNotPlanned, "--rulings", rul)
	if code != deskkit.ExitRefused {
		t.Fatalf("want exit 5 refused, got %d\n%s", code, out)
	}
	assertNoWrites(t, s)
}

// ---------------------------------------------------------------- decision-label control

func TestTriageDecisionLabelControl(t *testing.T) {
	t.Run("needs-decision refused under not-planned", func(t *testing.T) {
		s, rul := triageWorld(t)
		plantTrustedMarker(s, triageIssue)
		s.items[fmt.Sprintf("%s#%d", testRepo, triageIssue)] =
			issueJSON(triageIssue, "open", []string{"needs-decision"}, "")
		code, _ := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
			"--disposition", dispositionNotPlanned, "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5 refused, got %d", code)
		}
		assertNoWrites(t, s)
	})

	t.Run("needs-decision refused under human-decided", func(t *testing.T) {
		s, rul := triageWorld(t)
		url := plantDecision(s, triageIssue, "7005")
		s.items[fmt.Sprintf("%s#%d", testRepo, triageIssue)] =
			issueJSON(triageIssue, "open", []string{"needs-decision"}, "")
		code, _ := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
			"--disposition", dispositionHumanDecided, "--decision", url, "--tracker", "#40", "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5 refused, got %d", code)
		}
		assertNoWrites(t, s)
	})

	t.Run("human-decided label refused under not-planned", func(t *testing.T) {
		s, rul := triageWorld(t)
		plantTrustedMarker(s, triageIssue)
		s.items[fmt.Sprintf("%s#%d", testRepo, triageIssue)] =
			issueJSON(triageIssue, "open", []string{"human-decided"}, "")
		code, _ := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
			"--disposition", dispositionNotPlanned, "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5 refused, got %d", code)
		}
		assertNoWrites(t, s)
	})

	t.Run("human-decided label PERMITTED under human-decided", func(t *testing.T) {
		s, rul := triageWorld(t)
		url := plantDecision(s, triageIssue, "7006")
		s.items[fmt.Sprintf("%s#%d", testRepo, triageIssue)] =
			issueJSON(triageIssue, "open", []string{"human-decided"}, "")
		code, out := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
			"--disposition", dispositionHumanDecided, "--decision", url, "--tracker", "#40", "--rulings", rul)
		if code != deskkit.ExitOK {
			t.Fatalf("want exit 0, got %d\n%s", code, out)
		}
		assertClosedWith(t, s, reasonNotPlanned)
	})
}

// ---------------------------------------------------------------- arg surface + dry-run

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
			"--disposition", dispositionNotPlanned, "--kind", "mr", "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d", code)
		}
		assertNoWrites(t, s)
	})
	t.Run("--decision with not-planned refused", func(t *testing.T) {
		code, _ := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
			"--disposition", dispositionNotPlanned, "--decision",
			fmt.Sprintf("https://github.com/%s/issues/%d#issuecomment-1", testRepo, triageIssue), "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d", code)
		}
		assertNoWrites(t, s)
	})
	t.Run("human-decided without --decision refused", func(t *testing.T) {
		code, _ := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
			"--disposition", dispositionHumanDecided, "--tracker", "#40", "--rulings", rul)
		if code != deskkit.ExitRefused {
			t.Fatalf("want exit 5, got %d", code)
		}
		assertNoWrites(t, s)
	})
}

// TestTriageDryRunWritesNothing — --dry-run validates and reads (authority included), writes
// nothing.
func TestTriageDryRunWritesNothing(t *testing.T) {
	s, rul := triageWorld(t)
	plantTrustedMarker(s, triageIssue)
	code, out := execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
		"--disposition", dispositionNotPlanned, "--dry-run", "--rulings", rul)
	if code != deskkit.ExitOK || !strings.Contains(out, "dry-run") {
		t.Fatalf("want exit 0 dry-run, got %d\n%s", code, out)
	}
	assertNoWrites(t, s)
}
