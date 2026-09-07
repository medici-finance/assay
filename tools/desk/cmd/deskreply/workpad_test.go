package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// captureStdout runs fn with os.Stdout redirected to an in-process pipe and returns
// everything fn wrote. Tests below use it to pin the exact WORKPAD: … lines --dry-run and
// the real upsert print, not just their exit codes.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stdout
	os.Stdout = w
	fn()
	if cerr := w.Close(); cerr != nil {
		t.Fatalf("close pipe writer: %v", cerr)
	}
	os.Stdout = old
	out, rerr := io.ReadAll(r)
	if rerr != nil {
		t.Fatalf("read pipe: %v", rerr)
	}
	return string(out)
}

// workpadBody is a small helper building a valid --workpad body via the real Render path,
// so every test here exercises the SAME body a real worker would produce.
func workpadBody(stamp, plan string) string {
	return deskkit.Render(deskkit.Workpad{Stamp: stamp, Plan: plan})
}

// swapWorkpadSeams stubs workpadFinder/workpadEditor for the duration of the test and
// restores the real (forge-backed) implementations on cleanup.
func swapWorkpadSeams(t *testing.T,
	finder func(fg deskkit.Forge, fr deskkit.ForgeRepo, pr int, workerLogin string) ([]workpadCandidate, error),
	editor func(fg deskkit.Forge, fr deskkit.ForgeRepo, commentID, body string) error) {
	t.Helper()
	origFinder, origEditor := workpadFinder, workpadEditor
	t.Cleanup(func() { workpadFinder, workpadEditor = origFinder, origEditor })
	if finder != nil {
		workpadFinder = finder
	}
	if editor != nil {
		workpadEditor = editor
	}
}

// TestWorkpadUpsertIsIdempotent is Verify row 2: two upserts against an unchanged PR must
// leave exactly ONE workpad comment — the first call CREATES (via `gh pr comment`, the
// only comment-creating verb deskreply has ever emitted), the second EDITS the same
// comment in place (via the updateIssueComment seam) rather than creating a second one.
// The stubbed finder models GitHub's own state transition: empty before the first call's
// create actually lands, then carrying exactly the comment that call created.
func TestWorkpadUpsertIsIdempotent(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)

	const fakeCommentID, fakeDatabaseID = "COMMENT_NODE_1", 555
	created := false
	editCount := 0

	swapWorkpadSeams(t,
		func(fg deskkit.Forge, fr deskkit.ForgeRepo, pr int, workerLogin string) ([]workpadCandidate, error) {
			if !created {
				return nil, nil
			}
			return []workpadCandidate{{CommentID: fakeCommentID, DatabaseID: fakeDatabaseID}}, nil
		},
		func(fg deskkit.Forge, fr deskkit.ForgeRepo, commentID, body string) error {
			editCount++
			if commentID != fakeCommentID {
				t.Fatalf("edit targeted comment %q, want %q", commentID, fakeCommentID)
			}
			return nil
		},
	)

	bf1 := bodyFileWith(t, workpadBody("w@abc1234", "- step one"))
	rc := run([]string{"example-org/tracker", "7", "--workpad", "--body-file", bf1})
	if rc != deskkit.ExitOK {
		t.Fatalf("first upsert rc = %d, want 0", rc)
	}
	if !forgeRec(t).posted() {
		t.Fatalf("first upsert (no existing candidate) should CREATE a comment; forge writes: %v",
			forgeRec(t).writes())
	}
	if editCount != 0 {
		t.Fatalf("first upsert should not edit anything; editCount = %d", editCount)
	}
	created = true

	*calls = nil
	forgeRec(t).requests = nil
	bf2 := bodyFileWith(t, workpadBody("w@def5678", "- step one\n- step two"))
	rc = run([]string{"example-org/tracker", "7", "--workpad", "--body-file", bf2})
	if rc != deskkit.ExitOK {
		t.Fatalf("second upsert rc = %d, want 0", rc)
	}
	if forgeRec(t).posted() {
		t.Fatalf("second upsert must EDIT the existing comment, not create a second one; forge writes: %v",
			forgeRec(t).writes())
	}
	if editCount != 1 {
		t.Fatalf("second upsert should edit exactly once; editCount = %d", editCount)
	}
}

// TestWorkpadNeverEditsForeignMarker is Verify row 3: a human comment that happens to
// carry the marker line must never be treated as a candidate — filterWorkpadCandidates is
// the identity gate, and this pins it directly rather than through the full run() plumbing.
func TestWorkpadNeverEditsForeignMarker(t *testing.T) {
	const workerLogin = "assay-worker-app[bot]"

	var human, worker deskkit.Comment
	human.ID, human.DatabaseID = "HUMAN_NODE", 1
	human.Body = deskkit.WorkpadMarker + "\nada@abc1234\n\n## Plan\na human wrote this comment"
	human.Author.Login = "ada"

	worker.ID, worker.DatabaseID = "WORKER_NODE", 2
	worker.Body = deskkit.WorkpadMarker + "\nworktree@def5678\n\n## Plan\nthe worker's own workpad"
	worker.Author.Login = workerLogin

	cands := filterWorkpadCandidates([]deskkit.Comment{human, worker}, workerLogin)
	if len(cands) != 1 {
		t.Fatalf("filterWorkpadCandidates returned %d candidates, want exactly 1 (the human's marker-carrying "+
			"comment must never be a candidate): %+v", len(cands), cands)
	}
	if cands[0].CommentID != worker.ID {
		t.Fatalf("filterWorkpadCandidates selected comment %q, want the worker's own comment %q",
			cands[0].CommentID, worker.ID)
	}

	// The same guarantee the OTHER direction: a worker-authored login that merely LOOKS
	// similar (a human named literally "assay-worker-app", no [bot] suffix — i.e. NOT the
	// same actor per deskkit.SameActor's App-rendering fold) is not admitted either.
	var lookalike deskkit.Comment
	lookalike.ID, lookalike.DatabaseID = "LOOKALIKE_NODE", 3
	lookalike.Body = deskkit.WorkpadMarker + "\nx@abc1234\n\n## Plan\nlookalike"
	lookalike.Author.Login = "assay-worker-app"
	if got := filterWorkpadCandidates([]deskkit.Comment{lookalike}, workerLogin); len(got) != 0 {
		t.Fatalf("a login that is not the SAME actor as the App rendering must never be a candidate: %+v", got)
	}
}

// TestWorkpadFilterIgnoresMinimizedComments folds in Task item 4's "a resolved worker
// comment is skipped and a new one created": a minimised (hidden/resolved) worker comment
// must never survive the filter, so the newest-candidate search that follows correctly
// reports "not found" and cmdWorkpadUpsert takes the CREATE path — exactly the same path
// TestWorkpadUpsertIsIdempotent's first call already proves creates cleanly.
func TestWorkpadFilterIgnoresMinimizedComments(t *testing.T) {
	const workerLogin = "assay-worker-app[bot]"
	var resolved deskkit.Comment
	resolved.ID, resolved.DatabaseID = "OLD_RESOLVED_NODE", 9
	resolved.Body = deskkit.WorkpadMarker + "\nw@abc1234\n\n## Plan\nstale plan from a resolved thread"
	resolved.Minimized = true
	resolved.Author.Login = workerLogin

	cands := filterWorkpadCandidates([]deskkit.Comment{resolved}, workerLogin)
	if len(cands) != 0 {
		t.Fatalf("a minimised worker comment must never be a candidate: %+v", cands)
	}
	if _, found := newestWorkpadCandidate(cands); found {
		t.Fatal("newestWorkpadCandidate reported a candidate from an all-minimised input")
	}
}

// TestWorkpadBodycheckRefuses is Verify row 4: a --workpad body carrying a secret refuses
// exactly like a plain reply — the SAME bodycheck, applied before ANY workpad-specific
// preflight (mint/list/find-or-create) runs — with nothing posted.
func TestWorkpadBodycheckRefuses(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)

	body := workpadBody("w@abc1234", "here is the token ghp_"+strings.Repeat("a", 36))
	bf := bodyFileWith(t, body)

	rc := run([]string{"example-org/tracker", "7", "--workpad", "--body-file", bf})
	if rc != deskkit.ExitRefused {
		t.Fatalf("workpad body with a secret rc = %d, want 5 (refused)", rc)
	}
	if rec := forgeRec(t); len(rec.requests) != 0 {
		t.Fatalf("the verb reached the forge despite a secret in the workpad body: %v", rec.requests)
	}
}

// TestWorkpadWithoutMarkerRefuses: a --workpad body that does not carry the marker is a
// caller error (it belongs on the plain-reply path instead) and is refused before any
// write, same as the bodycheck case above.
func TestWorkpadWithoutMarkerRefuses(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)

	bf := bodyFileWith(t, "an ordinary reply body with no workpad marker")
	rc := run([]string{"example-org/tracker", "7", "--workpad", "--body-file", bf})
	if rc != deskkit.ExitRefused {
		t.Fatalf("--workpad body without the marker rc = %d, want 5 (refused)", rc)
	}
	if rec := forgeRec(t); len(rec.requests) != 0 {
		t.Fatalf("the verb reached the forge despite the missing workpad marker: %v", rec.requests)
	}
}

// TestWorkpadDryRunRequiresWorkpadFlag: --dry-run only means something alongside
// --workpad; on its own it must refuse rather than silently behave like a live plain reply.
func TestWorkpadDryRunRequiresWorkpadFlag(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)

	bf := bodyFileWith(t, "a plain reply body")
	rc := run([]string{"example-org/tracker", "7", "--dry-run", "--body-file", bf})
	if rc != deskkit.ExitRefused {
		t.Fatalf("--dry-run without --workpad rc = %d, want 5 (refused)", rc)
	}
	if rec := forgeRec(t); len(rec.requests) != 0 {
		t.Fatalf("the verb reached the forge on a refused --dry-run-without---workpad invocation: %v", rec.requests)
	}
}

// TestWorkpadDryRunReportsWithoutWriting exercises both dry-run messages
// ("WORKPAD: would create" / "WORKPAD: would edit #<id>") and asserts NEITHER produces any
// gh call — dry-run's whole point is a report with no write.
func TestWorkpadDryRunReportsWithoutWriting(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	swapWorkpadSeams(t,
		func(fg deskkit.Forge, fr deskkit.ForgeRepo, pr int, workerLogin string) ([]workpadCandidate, error) {
			return nil, nil
		},
		nil,
	)

	bf := bodyFileWith(t, workpadBody("w@abc1234", "- step one"))
	out := captureStdout(t, func() {
		rc := run([]string{"example-org/tracker", "7", "--workpad", "--dry-run", "--body-file", bf})
		if rc != deskkit.ExitOK {
			t.Fatalf("dry-run (no candidate) rc = %d, want 0", rc)
		}
	})
	if !strings.Contains(out, "WORKPAD: would create") {
		t.Fatalf("dry-run with no existing candidate did not report %q; got %q", "WORKPAD: would create", out)
	}
	assertNoComment(t, forgeRec(t))

	*calls = nil
	swapWorkpadSeams(t,
		func(fg deskkit.Forge, fr deskkit.ForgeRepo, pr int, workerLogin string) ([]workpadCandidate, error) {
			return []workpadCandidate{{CommentID: "N1", DatabaseID: 42}}, nil
		},
		nil,
	)
	out = captureStdout(t, func() {
		rc := run([]string{"example-org/tracker", "7", "--workpad", "--dry-run", "--body-file", bf})
		if rc != deskkit.ExitOK {
			t.Fatalf("dry-run (existing candidate) rc = %d, want 0", rc)
		}
	})
	if !strings.Contains(out, "WORKPAD: would edit #42") {
		t.Fatalf("dry-run with an existing candidate did not report %q; got %q", "WORKPAD: would edit #42", out)
	}
	assertNoComment(t, forgeRec(t))
}

// THE THREE RETIRED TESTS, and where each one's assertion now lives. All three were about a
// WIRE SHAPE this package no longer owns:
//
//	TestParseWorkpadCommentsResponseSurfacesGraphQLErrors → deskkit's GitHubForge.ListComments
//	   reports a non-empty top-level `errors` array as an error rather than reading a partial
//	   list as a complete one; the golden case `list_comments` pins the read, and the check
//	   itself is one branch of that method.
//	TestParseWorkpadCommentsResponseDecodesNodes         → the `list_comments` golden pins the
//	   decoded nodes for BOTH backends, which is strictly more than this package's own parse
//	   was ever checked against.
//	TestCommentIDFromURL                                 → the create path no longer parses an
//	   id out of a printed URL. It records the id the FORGE reported for the comment it just
//	   created, which TestWorkpadCreateRecordsTheForgeReportedID below asserts directly — and
//	   which works on a forge whose URLs have a different shape, where the parse silently
//	   yielded 0.

// TestWorkpadCreateRecordsTheForgeReportedID is the successor to TestCommentIDFromURL: the
// worktree-local hint recorded after a CREATE is the numeric id the forge reported for the
// comment it created, not a number recovered from display text.
func TestWorkpadCreateRecordsTheForgeReportedID(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	swapWorkpadSeams(t,
		func(fg deskkit.Forge, fr deskkit.ForgeRepo, pr int, workerLogin string) ([]workpadCandidate, error) {
			return nil, nil // no existing candidate: the CREATE path
		},
		nil,
	)

	bf := bodyFileWith(t, workpadBody("w@abc1234", "- step one"))
	if rc := run([]string{"example-org/tracker", "7", "--workpad", "--body-file", bf}); rc != deskkit.ExitOK {
		t.Fatalf("workpad create rc = %d, want 0", rc)
	}
	if !forgeRec(t).posted() {
		t.Fatalf("the create path posted nothing: %v", forgeRec(t).writes())
	}
	// The fake forge reports id 123 for the comment it creates; the hint must carry it.
	got, err := git(work, "config", "--worktree", "--get", workpadConfigKey)
	if err != nil {
		t.Fatalf("the worktree-local workpad hint was not recorded: %v", err)
	}
	if strings.TrimSpace(got) != "123" {
		t.Fatalf("recorded workpad id %q, want the forge-reported 123", got)
	}
}
