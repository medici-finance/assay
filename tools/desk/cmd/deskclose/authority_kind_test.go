package main

// authority_kind_test.go — the residual half of #1019 (desktools-v2/04).
//
// #1019 reported that deskclose could not close on an issue because the comment read assumed a
// pull request. Most of that was already fixed at head — the superseded lane and the triage
// lane state the target kind. What remained was the AUTHORIZATION read itself: the ruling gate
// (authorize) and the manifest gate (authorizeManifest) still fetched their authorizing comment
// through the kind-less fetchComment, which reads a CHANGE's thread on both backends. A human
// ruling recorded on an ISSUE therefore came back as could-not-check every time.
//
// fetchComment now derives the kind from the permalink itself (see its doc comment in
// authority.go) instead of defaulting: `/pull/` reads TargetChange, `/issues/` tries
// TargetIssue first and falls back to TargetChange only on a could-not-check.

import (
	"fmt"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestRulingCommentOnAnIssueAuthorizes is the #1019 residual, restated as a check. The sign-off
// comment lives on a genuine ISSUE thread (not a pull request) — exactly the shape #1019
// reported. On the unfixed code (kind-less ListComments, the change/pullRequest selection) this
// is could-not-check every time, because the untyped stub reproduces the SAME defect production
// has (deskclose_forgestub_test.go's ListComments comment). The fix must reach it.
func TestRulingCommentOnAnIssueAuthorizes(t *testing.T) {
	s, _ := baseWorld(t)
	const (
		issueNum = 298
		cid      = "5160298001"
	)
	s.items[testRepo+"#"+fmt.Sprint(issueNum)] = issueJSON(issueNum, "open", nil, "tracking issue, not a PR")
	s.comment[cid] = commentJSON(blessLogin, blessID, "User",
		"accepted. lanes B and C as written.",
		"https://api.github.com/repos/"+testRepo+"/issues/"+fmt.Sprint(issueNum))
	url := fmt.Sprintf("https://github.com/%s/issues/%d#issuecomment-%s", testRepo, issueNum, cid)
	rul := signedRulings(t, url)

	code, out := execCLI(modeSuperseded, "-R", testRepo, fmt.Sprint(subjectIssue),
		"--by", mergedPRRef, "--rulings", rul)
	if code != deskkit.ExitOK {
		t.Fatalf("a ruling sign-off comment on a genuine ISSUE thread must authorize (exit 0), got %d\n%s",
			code, out)
	}
	assertConfirmedClose(t, s, reasonNotPlanned)
}

// TestIssuesPathNamingAChangeStillResolves is the other half of the derivation: GitHub renders
// a pull request's OWN comment permalink under `/issues/<N>` too (the forge redirects it), so
// `/issues/` alone never proves the object is an issue. The sign-off comment here lives on a
// genuine PULL REQUEST thread but is linked under `/issues/<N>` — the TargetIssue try must come
// back could-not-check (the stub's kind-mismatch check) and the retry as TargetChange must still
// resolve it.
func TestIssuesPathNamingAChangeStillResolves(t *testing.T) {
	s, _ := baseWorld(t)
	const (
		prNum = 299
		cid   = "5160299001"
	)
	s.items[testRepo+"#"+fmt.Sprint(prNum)] = prIssueJSON(prNum, "open")
	s.comment[cid] = commentJSON(blessLogin, blessID, "User",
		"accepted. lanes B and C as written.",
		"https://api.github.com/repos/"+testRepo+"/issues/"+fmt.Sprint(prNum))
	url := fmt.Sprintf("https://github.com/%s/issues/%d#issuecomment-%s", testRepo, prNum, cid)
	rul := signedRulings(t, url)

	code, out := execCLI(modeSuperseded, "-R", testRepo, fmt.Sprint(subjectIssue),
		"--by", mergedPRRef, "--rulings", rul)
	if code != deskkit.ExitOK {
		t.Fatalf("an /issues/ permalink naming a change must still resolve (exit 0), got %d\n%s", code, out)
	}
	assertConfirmedClose(t, s, reasonNotPlanned)
}

// TestCommentIdFromAnotherItemIsStillRefused is the negative-path row: widening the read to try
// both kinds must never widen WHAT authorizes. The sign-off URL names a comment id that is real
// — but lives on a DIFFERENT item's thread, not on the one the permalink names. The TargetIssue
// try comes back could-not-check (the named number is a pull request, same as the positive case
// above), the retry as TargetChange DOES reach that item's own thread, but the id the permalink
// claims is not on it — so the fetch must still be refused, not silently authorize off whichever
// thread happened to answer.
func TestCommentIdFromAnotherItemIsStillRefused(t *testing.T) {
	s, _ := baseWorld(t)
	const (
		prNum      = 299
		realCID    = "5160299001" // the comment actually on prNum's own thread
		decoyCID   = "5160299002" // a real comment id, but on a DIFFERENT item
		decoyOnNum = 250
	)
	s.items[testRepo+"#"+fmt.Sprint(prNum)] = prIssueJSON(prNum, "open")
	s.comment[realCID] = commentJSON(blessLogin, blessID, "User", "not the ruling",
		"https://api.github.com/repos/"+testRepo+"/issues/"+fmt.Sprint(prNum))
	s.comment[decoyCID] = commentJSON(blessLogin, blessID, "User", "belongs elsewhere",
		"https://api.github.com/repos/"+testRepo+"/issues/"+fmt.Sprint(decoyOnNum))
	// The permalink CLAIMS decoyCID is on prNum — it is not.
	url := fmt.Sprintf("https://github.com/%s/issues/%d#issuecomment-%s", testRepo, prNum, decoyCID)
	rul := signedRulings(t, url)

	err := execErr(modeSuperseded, "-R", testRepo, fmt.Sprint(subjectIssue),
		"--by", mergedPRRef, "--rulings", rul)
	if err == nil {
		t.Fatal("a comment id that is not on the permalink's own item must be refused, whichever " +
			"kind's thread the retry finally reads")
	}
	if !deskkit.IsRefused(err) {
		t.Fatalf("want refused (exit 5) — the id match failed, not an unreadable fetch — got %v (exit %d)",
			err, deskkit.ExitCodeOf(err))
	}
	if got := s.writes(); len(got) != 0 {
		t.Fatalf("a refusal wrote to the forge: %v", got)
	}
}
