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
// restores the real (gh-backed) implementations on cleanup.
func swapWorkpadSeams(t *testing.T, finder func(dir, repo string, pr int, workerLogin string) ([]workpadCandidate, error), editor func(dir, nodeID, bodyPath string) error) {
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

	const fakeNodeID, fakeDatabaseID = "COMMENT_NODE_1", 555
	created := false
	editCount := 0

	swapWorkpadSeams(t,
		func(dir, repo string, pr int, workerLogin string) ([]workpadCandidate, error) {
			if !created {
				return nil, nil
			}
			return []workpadCandidate{{NodeID: fakeNodeID, DatabaseID: fakeDatabaseID}}, nil
		},
		func(dir, nodeID, bodyPath string) error {
			editCount++
			if nodeID != fakeNodeID {
				t.Fatalf("edit targeted node %q, want %q", nodeID, fakeNodeID)
			}
			return nil
		},
	)

	bf1 := bodyFileWith(t, workpadBody("w@abc1234", "- step one"))
	rc := run([]string{"example-org/tracker", "7", "--workpad", "--body-file", bf1})
	if rc != deskkit.ExitOK {
		t.Fatalf("first upsert rc = %d, want 0", rc)
	}
	if !anyCall(ghCalls(*calls), "pr", "comment") {
		t.Fatalf("first upsert (no existing candidate) should CREATE via `gh pr comment`; calls: %v", ghCalls(*calls))
	}
	if editCount != 0 {
		t.Fatalf("first upsert should not edit anything; editCount = %d", editCount)
	}
	created = true

	*calls = nil
	bf2 := bodyFileWith(t, workpadBody("w@def5678", "- step one\n- step two"))
	rc = run([]string{"example-org/tracker", "7", "--workpad", "--body-file", bf2})
	if rc != deskkit.ExitOK {
		t.Fatalf("second upsert rc = %d, want 0", rc)
	}
	if anyCall(ghCalls(*calls), "pr", "comment") {
		t.Fatalf("second upsert must EDIT the existing comment, not create a second one; calls: %v", ghCalls(*calls))
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

	var human, worker workpadNode
	human.ID, human.DatabaseID = "HUMAN_NODE", 1
	human.Body = deskkit.WorkpadMarker + "\nada@abc1234\n\n## Plan\na human wrote this comment"
	human.Author.Login = "ada"

	worker.ID, worker.DatabaseID = "WORKER_NODE", 2
	worker.Body = deskkit.WorkpadMarker + "\nworktree@def5678\n\n## Plan\nthe worker's own workpad"
	worker.Author.Login = workerLogin

	cands := filterWorkpadCandidates([]workpadNode{human, worker}, workerLogin)
	if len(cands) != 1 {
		t.Fatalf("filterWorkpadCandidates returned %d candidates, want exactly 1 (the human's marker-carrying "+
			"comment must never be a candidate): %+v", len(cands), cands)
	}
	if cands[0].NodeID != worker.ID {
		t.Fatalf("filterWorkpadCandidates selected node %q, want the worker's own comment %q", cands[0].NodeID, worker.ID)
	}

	// The same guarantee the OTHER direction: a worker-authored login that merely LOOKS
	// similar (a human named literally "assay-worker-app", no [bot] suffix — i.e. NOT the
	// same actor per deskkit.SameActor's App-rendering fold) is not admitted either.
	var lookalike workpadNode
	lookalike.ID, lookalike.DatabaseID = "LOOKALIKE_NODE", 3
	lookalike.Body = deskkit.WorkpadMarker + "\nx@abc1234\n\n## Plan\nlookalike"
	lookalike.Author.Login = "assay-worker-app"
	if got := filterWorkpadCandidates([]workpadNode{lookalike}, workerLogin); len(got) != 0 {
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
	var resolved workpadNode
	resolved.ID, resolved.DatabaseID = "OLD_RESOLVED_NODE", 9
	resolved.Body = deskkit.WorkpadMarker + "\nw@abc1234\n\n## Plan\nstale plan from a resolved thread"
	resolved.IsMinimized = true
	resolved.Author.Login = workerLogin

	cands := filterWorkpadCandidates([]workpadNode{resolved}, workerLogin)
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
	calls := withEnv(t, work)

	body := workpadBody("w@abc1234", "here is the token ghp_"+strings.Repeat("a", 36))
	bf := bodyFileWith(t, body)

	rc := run([]string{"example-org/tracker", "7", "--workpad", "--body-file", bf})
	if rc != deskkit.ExitRefused {
		t.Fatalf("workpad body with a secret rc = %d, want 5 (refused)", rc)
	}
	if len(ghCalls(*calls)) != 0 {
		t.Fatalf("a gh call was made despite a secret in the workpad body: %v", ghCalls(*calls))
	}
}

// TestWorkpadWithoutMarkerRefuses: a --workpad body that does not carry the marker is a
// caller error (it belongs on the plain-reply path instead) and is refused before any
// write, same as the bodycheck case above.
func TestWorkpadWithoutMarkerRefuses(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)

	bf := bodyFileWith(t, "an ordinary reply body with no workpad marker")
	rc := run([]string{"example-org/tracker", "7", "--workpad", "--body-file", bf})
	if rc != deskkit.ExitRefused {
		t.Fatalf("--workpad body without the marker rc = %d, want 5 (refused)", rc)
	}
	if len(ghCalls(*calls)) != 0 {
		t.Fatalf("a gh call was made despite the missing workpad marker: %v", ghCalls(*calls))
	}
}

// TestWorkpadDryRunRequiresWorkpadFlag: --dry-run only means something alongside
// --workpad; on its own it must refuse rather than silently behave like a live plain reply.
func TestWorkpadDryRunRequiresWorkpadFlag(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)

	bf := bodyFileWith(t, "a plain reply body")
	rc := run([]string{"example-org/tracker", "7", "--dry-run", "--body-file", bf})
	if rc != deskkit.ExitRefused {
		t.Fatalf("--dry-run without --workpad rc = %d, want 5 (refused)", rc)
	}
	if len(ghCalls(*calls)) != 0 {
		t.Fatalf("a gh call was made on a refused --dry-run-without---workpad invocation: %v", ghCalls(*calls))
	}
}

// TestWorkpadDryRunReportsWithoutWriting exercises both dry-run messages
// ("WORKPAD: would create" / "WORKPAD: would edit #<id>") and asserts NEITHER produces any
// gh call — dry-run's whole point is a report with no write.
func TestWorkpadDryRunReportsWithoutWriting(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	swapWorkpadSeams(t,
		func(dir, repo string, pr int, workerLogin string) ([]workpadCandidate, error) { return nil, nil },
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
	assertNoPrComment(t, *calls)

	*calls = nil
	swapWorkpadSeams(t,
		func(dir, repo string, pr int, workerLogin string) ([]workpadCandidate, error) {
			return []workpadCandidate{{NodeID: "N1", DatabaseID: 42}}, nil
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
	assertNoPrComment(t, *calls)
}

// TestParseWorkpadCommentsResponseSurfacesGraphQLErrors pins the "errors" half of the
// GraphQL response contract: a non-empty top-level errors array must be reported as an
// error, never silently ignored in favour of whatever (possibly empty/partial) "data" came
// back alongside it.
func TestParseWorkpadCommentsResponseSurfacesGraphQLErrors(t *testing.T) {
	raw := `{"data":null,"errors":[{"message":"Could not resolve to a PullRequest"}]}`
	if _, err := parseWorkpadCommentsResponse(raw); err == nil {
		t.Fatal("parseWorkpadCommentsResponse returned no error for a response carrying a GraphQL errors array")
	}
}

// TestParseWorkpadCommentsResponseDecodesNodes is a small direct pin of the wire shape
// listWorkpadCandidatesGH depends on, independent of any gh subprocess.
func TestParseWorkpadCommentsResponseDecodesNodes(t *testing.T) {
	raw := `{"data":{"repository":{"pullRequest":{"comments":{"nodes":[` +
		`{"id":"N1","databaseId":11,"body":"hello","isMinimized":false,"author":{"login":"ada"}}` +
		`]}}}}}`
	nodes, err := parseWorkpadCommentsResponse(raw)
	if err != nil {
		t.Fatalf("parseWorkpadCommentsResponse: %v", err)
	}
	if len(nodes) != 1 || nodes[0].ID != "N1" || nodes[0].DatabaseID != 11 || nodes[0].Author.Login != "ada" {
		t.Fatalf("parseWorkpadCommentsResponse decoded %+v, want one node N1/11/ada", nodes)
	}
}

// TestCommentIDFromURL pins the `#issuecomment-<id>` suffix parse the create path uses to
// record the worktree-local hint.
func TestCommentIDFromURL(t *testing.T) {
	cases := map[string]int{
		"https://github.com/example-org/tracker/pull/7#issuecomment-123": 123,
		"https://github.com/example-org/tracker/pull/7":                  0,
		"":          0,
		"not a url": 0,
	}
	for url, want := range cases {
		if got := commentIDFromURL(url); got != want {
			t.Errorf("commentIDFromURL(%q) = %d, want %d", url, got, want)
		}
	}
}
