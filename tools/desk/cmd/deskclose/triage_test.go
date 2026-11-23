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
	"strconv"
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

	t.Run("unstructured reply cannot clear needs-decision", func(t *testing.T) {
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

const queuedAsk = "<!-- decision-gate: assay:demo:sample:02 -->\n\nOptions:\n\n1. Prepare the candidate.\n2. Return it.\n"

func queuedWorld(t *testing.T, body, reply string) (*stubRemote, string, string) {
	t.Helper()
	s, rul, _ := recWorld(t, recT0)
	key := fmt.Sprintf("%s#%d", testRepo, triageIssue)
	s.items[key] = strings.Replace(s.items[key], `{"name":"human-decided"}`, `{"name":"needs-decision"},{"name":"keep"}`, 1)
	s.items[key] = strings.Replace(s.items[key], strconv.Quote(recIssueBody()), strconv.Quote(body), 1)
	url := plantAtComment(s, triageIssue, "7007", blessLogin, blessID, "User", recT1, reply)
	return s, rul, url
}

func TestTriageQueuedDecision(t *testing.T) {
	s, rul, url := queuedWorld(t, queuedAsk, " sample/02: 1")
	code, out := recClose(t, rul, url, "--dry-run")
	if code != 0 || !strings.Contains(out, "needs-decision -> human-decided") {
		t.Fatalf("dry-run did not describe the verified transition: code %d: %s", code, out)
	}
	assertNoWrites(t, s)
	code, out = recClose(t, rul, url)
	if code != 0 {
		t.Fatalf("close: code %d: %s", code, out)
	}
	it, err := fetchItem(testRepo, triageIssue, deskkit.TargetIssue)
	if err != nil || !it.closed() || hasLabel(it, labelNeedsDecision) || !hasLabel(it, dispositionHumanDecided) || !hasLabel(it, "keep") {
		t.Fatalf("issue after close: %+v (%v)", it, err)
	}
	if rec := recDecode(t, s); rec.Picked != "1" || rec.Brief != "sample/02" {
		t.Fatalf("wrong recorded ruling: %+v", rec)
	}
	writes := len(s.writes())
	if code, _ := recClose(t, rul, url); code != 0 || len(s.writes()) != writes {
		t.Fatal("closed replay must be a no-op")
	}
}

func TestTriageQueueRefusals(t *testing.T) {
	for _, reply := range []string{"1", "sample/03: 1", "sample/02: 3", "sample/02: 1?", "> sample/02: 1", "    sample/02: 1", "sample/02: 1 or 2", "sample/02: 1 — if it passes"} {
		t.Run(reply, func(t *testing.T) {
			s, rul, url := queuedWorld(t, queuedAsk, reply)
			if code, _ := recClose(t, rul, url); code != deskkit.ExitRefused {
				t.Fatalf("want refusal, got %d", code)
			}
			assertNoWrites(t, s)
		})
	}
	for _, tc := range []struct {
		name  string
		code  int
		stage func(*stubRemote)
	}{
		{"edited", 5, func(s *stubRemote) {
			plantEditedComment(s, triageIssue, "7007", blessLogin, blessID, "User", recT1, "2026-10-04T00:00:00Z", "sample/02: 1")
		}},
		{"hidden", 5, func(s *stubRemote) {
			s.comment["7007"] = strings.Replace(s.comment["7007"], `"minimized":false`, `"minimized":true`, 1)
		}},
		{"another human reply", 5, func(s *stubRemote) {
			plantAtComment(s, triageIssue, "7008", blessLogin, blessID, "User", "2026-10-04T00:00:00Z", "Wait")
		}},
		{"reopened", 5, func(s *stubRemote) {
			s.queueStateEvents = []deskkit.IssueStateEvent{{Kind: deskkit.IssueEventReopened, CreatedAt: recT1}}
		}},
		{"requeued", 5, func(s *stubRemote) {
			s.queueLabelEvents = []deskkit.LabelEvent{{Name: labelNeedsDecision, CreatedAt: recT1}}
		}},
		{"incomplete history", 6, func(s *stubRemote) { s.queueLabelsIncomplete = true }},
		{"unknown body edits", 6, func(s *stubRemote) { s.bodyEditUnknown = true }},
		{"body changed", 5, func(s *stubRemote) {
			s.bodyEdited[fmt.Sprintf("%s#%d", testRepo, triageIssue)] = "2026-10-04T00:00:00Z"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, rul, url := queuedWorld(t, queuedAsk, "sample/02: 1")
			tc.stage(s)
			if code, _ := recClose(t, rul, url); code != tc.code {
				t.Fatalf("want %d, got %d", tc.code, code)
			}
			assertNoWrites(t, s)
		})
	}
}

func TestTriageQueuedLabelRetry(t *testing.T) {
	s, rul, url := queuedWorld(t, queuedAsk, "sample/02: 2")
	s.failApply = true
	if code, _ := recClose(t, rul, url); code == 0 {
		t.Fatal("failed add must hold the close")
	}
	it, _ := fetchItem(testRepo, triageIssue, deskkit.TargetIssue)
	if !hasLabel(it, labelNeedsDecision) || hasLabel(it, dispositionHumanDecided) || it.closed() {
		t.Fatalf("failed add: %+v", it)
	}
	s.failApply = false
	s.failRemove = true
	if code, _ := recClose(t, rul, url); code == 0 {
		t.Fatal("failed removal must hold the close")
	}
	it, _ = fetchItem(testRepo, triageIssue, deskkit.TargetIssue)
	if !hasLabel(it, labelNeedsDecision) || !hasLabel(it, dispositionHumanDecided) || it.closed() {
		t.Fatalf("partial transition: %+v", it)
	}
	s.failRemove = false
	if code, out := recClose(t, rul, url); code != 0 {
		t.Fatalf("retry %d: %s", code, out)
	}
	if rec := recDecode(t, s); rec.Picked != "2" {
		t.Fatalf("return decision recorded as %+v", rec)
	}
}

func TestTriageQueuedCloseRetryReadsNewReply(t *testing.T) {
	s, rul, url := queuedWorld(t, queuedAsk, "sample/02: 1")
	key := fmt.Sprintf("%s#%d", testRepo, triageIssue)
	s.failPost[key] = true
	if code, _ := recClose(t, rul, url); code == 0 {
		t.Fatal("failed receipt must leave the issue open")
	}
	it, _ := fetchItem(testRepo, triageIssue, deskkit.TargetIssue)
	if hasLabel(it, labelNeedsDecision) || !hasLabel(it, dispositionHumanDecided) || it.closed() {
		t.Fatalf("partial close: %+v", it)
	}
	s.failPost[key] = false
	plantAtComment(s, triageIssue, "7008", blessLogin, blessID, "User", "2026-10-04T00:00:00Z", "Hold")
	writes := len(s.writes())
	if code, _ := recClose(t, rul, url); code != deskkit.ExitRefused || len(s.writes()) != writes {
		t.Fatalf("stale ruling on retry: exit %d", code)
	}
}

func TestTriageQueueVisibleOffer(t *testing.T) {
	for _, body := range []string{"~~~\n## Options\nA. example\n~~~", "> ## Options\n> A. example", "    ## Options\n    A. example"} {
		s, rul, url := queuedWorld(t, body, "A")
		if code, _ := recClose(t, rul, url); code != deskkit.ExitRefused {
			t.Fatalf("example option in %q: exit %d", body, code)
		}
		assertNoWrites(t, s)
	}
	_, rul, url := queuedWorld(t, "## Options\nA. Ship\nB. Return", "B")
	if code, _ := recClose(t, rul, url); code != 0 {
		t.Fatalf("direct issue choice: exit %d", code)
	}
}

func TestTriageQueuedProtectedOrSameTracker(t *testing.T) {
	for _, label := range []string{deskkit.VerifyGateLabel, "human-only"} {
		s, rul, url := queuedWorld(t, queuedAsk, "sample/02: 1")
		key := fmt.Sprintf("%s#%d", testRepo, triageIssue)
		s.items[key] = strings.Replace(s.items[key], `"name":"keep"`, `"name":`+strconv.Quote(label), 1)
		if code, _ := recClose(t, rul, url); code != deskkit.ExitRefused {
			t.Fatalf("label %s: %d", label, code)
		}
		assertNoWrites(t, s)
	}
	s, rul, url := queuedWorld(t, queuedAsk, "sample/02: 1")
	if code, _ := recClose(t, rul, url, "--tracker", fmt.Sprint(triageIssue)); code != deskkit.ExitRefused {
		t.Fatalf("self tracker: %d", code)
	}
	assertNoWrites(t, s)
}

func TestTriageQueueGitLab(t *testing.T) {
	s, rul, _ := queuedWorld(t, queuedAsk, "sample/02: 1")
	s.forge = deskkit.ForgeGitLab
	s.host = glHost
	url := fmt.Sprintf("https://%s/%s/-/issues/%d#note_7007", glHost, testRepo, triageIssue)
	if code, out := recClose(t, rul, url); code != 0 {
		t.Fatalf("GitLab decision: %d %s", code, out)
	}
	it, err := fetchItem(testRepo, triageIssue, deskkit.TargetIssue)
	if err != nil || !it.closed() || hasLabel(it, labelNeedsDecision) || !hasLabel(it, dispositionHumanDecided) {
		t.Fatalf("issue: %+v (%v)", it, err)
	}
}
