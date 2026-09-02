package main

// edit_test.go — `deskpr edit`, the verb that corrects an OPEN PR's body/title text.
//
// FAIL-FIRST. Every assertion here was observed RED before edit.go existed: with no `edit`
// case in main.go's switch, `run([]string{"edit", …})` fell through to the unknown-
// subcommand arm and returned exit 5 for every one of them. That makes the four refusal
// subtests pass VACUOUSLY on the unfixed tree — a refusal for the wrong reason. Five went
// genuinely RED there and cannot be satisfied by any refusal at all:
// TestEditSuccessPostsTheReviewNotice, TestEditMayAddATrailerToAPreTrailerPR,
// TestEditNoopsWhenTheTextAlreadyMatches, TestEditReportsALandedEditWhoseNoticeFailed and
// TestEditGoesThroughThePublicRepoGate.
//
// Because the refusal cases cannot rest on the exit code alone, each asserts on the
// refusal's OWN evidence — zero `gh pr edit`/`gh pr comment` argv, and where two guards
// would both produce exit 5, on the message that says which one fired — and each names,
// in its comment, the mutation that re-reds it against the FIXED code. Those mutations
// were run: A (drop the trailer-immutability arm) reds TestEditRefusesATrailerChange;
// dropping the body scan reds TestEditRefusesASecretInTheBody; dropping the title scan
// reds TestEditRefusesASecretInTheTitle; dropping the no-open-PR arm panics
// TestEditRefusesWithNoOpenPR on a nil PR; dropping the noop reds
// TestEditNoopsWhenTheTextAlreadyMatches; dropping the comment post reds
// TestEditSuccessPostsTheReviewNotice; dropping the public-repo gate reds
// TestEditGoesThroughThePublicRepoGate.

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// writeTempFile stages a --body-file for a subtest and returns its path.
func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "body.md")
	writeFile(t, path, content)
	return path
}

// editCalls returns the recorded `gh pr edit` argvs — the writes this verb exists to make,
// and the thing every refusal test must show ZERO of.
func editCalls(calls [][]string) [][]string {
	var out [][]string
	for _, c := range ghCalls(calls) {
		if len(c) >= 3 && c[1] == "pr" && c[2] == "edit" {
			out = append(out, c)
		}
	}
	return out
}

// commentCalls returns the recorded `gh pr comment` argvs — the re-review notice.
func commentCalls(calls [][]string) [][]string {
	var out [][]string
	for _, c := range ghCalls(calls) {
		if len(c) >= 3 && c[1] == "pr" && c[2] == "comment" {
			out = append(out, c)
		}
	}
	return out
}

// argAfter returns the value following flag in an argv, or "" when the flag is absent.
func argAfter(c []string, flag string) string {
	for i, a := range c {
		if a == flag && i+1 < len(c) {
			return c[i+1]
		}
	}
	return ""
}

// TestEditRefusesWithNoOpenPR — the branch has no OPEN PR, so there is nothing to edit.
// The listing is --state open, so this ONE refusal is also how a merged or a closed PR is
// refused: neither appears in it. Nothing may be written on this path.
//
// Re-red against the fixed code: delete the `if pr == nil { return … }` arm in cmdEdit and
// this goes from exit 5 to a nil-pointer panic reading pr.Number — i.e. the guard is what
// the refusal rests on, not an accident of the fake.
func TestEditRefusesWithNoOpenPR(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	// FAKEGH_LIST_HAS_PR deliberately UNSET: `gh pr list --state open` returns [].
	bodyPath := writeTempFile(t, "corrected text\nBrief: fixture/01\n")

	rc := run([]string{"edit", "--body-file", bodyPath})
	if rc != deskkit.ExitRefused {
		t.Fatalf("edit with no open PR rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
	}
	if got := editCalls(*calls); len(got) != 0 {
		t.Fatalf("the refusal still edited the PR: %v", got)
	}
	if got := commentCalls(*calls); len(got) != 0 {
		t.Fatalf("the refusal still posted the re-review notice: %v", got)
	}
}

// TestEditRefusesATrailerChange — the body's link trailer is the derived board's edge from
// the PR to its work item. A body-rewrite verb that could re-point it (or drop it) would
// make that edge assertable exactly once, at create time, and silently revocable forever
// after. The PR's current body carries `Brief: fixture/01`; a replacement naming
// `Issue: #77` is refused, and nothing is written.
//
// Re-red against the fixed code: drop the `hadLink && oldLink != newLink` refusal in
// cmdEdit and this subtest alone goes green-to-red — the edit succeeds (exit 0) with the
// PR re-pointed at a different work item.
func TestEditRefusesATrailerChange(t *testing.T) {
	t.Run("changed", func(t *testing.T) {
		work := newBaseFixture(t)
		calls := withEnv(t, work)
		t.Setenv("FAKEGH_LIST_HAS_PR", "1")
		t.Setenv("FAKEGH_PR_BODY", "the original body\nBrief: fixture/01\n")
		bodyPath := writeTempFile(t, "corrected text\nIssue: #77\n")

		rc := run([]string{"edit", "--body-file", bodyPath})
		if rc != deskkit.ExitRefused {
			t.Fatalf("edit re-pointing the trailer rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
		}
		if got := editCalls(*calls); len(got) != 0 {
			t.Fatalf("the refusal still edited the PR: %v", got)
		}
	})

	// Removal is the same rule seen from the other side, and it is caught EARLIER — by the
	// same requireTrailer grammar create runs, before any network call. Asserting it here
	// pins that a replacement body cannot simply drop the link.
	t.Run("removed", func(t *testing.T) {
		work := newBaseFixture(t)
		calls := withEnv(t, work)
		t.Setenv("FAKEGH_LIST_HAS_PR", "1")
		bodyPath := writeTempFile(t, "corrected text with no link line at all\n")

		rc := run([]string{"edit", "--body-file", bodyPath})
		if rc != deskkit.ExitRefused {
			t.Fatalf("edit dropping the trailer rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
		}
		if got := editCalls(*calls); len(got) != 0 {
			t.Fatalf("the refusal still edited the PR: %v", got)
		}
		if got := ghCalls(*calls); len(got) != 0 {
			t.Fatalf("the trailer grammar must refuse BEFORE any forge call; made %d: %v", len(got), got)
		}
	})
}

// TestEditRefusesASecretInTheBody — the replacement body faces the same secret scan the
// create path runs over the body it publishes, with the same audited
// --force-scan-override. A body carrying a credential is exit 5 and writes nothing.
//
// Re-red against the fixed code: remove the HandleScanRefusal(…, ScanSurface("PR body", …))
// call at the top of cmdEdit and the credential lands on the PR at exit 0.
func TestEditRefusesASecretInTheBody(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1")
	bodyPath := writeTempFile(t, "token ghp_"+strings.Repeat("a", 30)+"\nBrief: fixture/01\n")

	rc := run([]string{"edit", "--body-file", bodyPath})
	if rc != deskkit.ExitRefused {
		t.Fatalf("edit with a credential in the body rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
	}
	if got := editCalls(*calls); len(got) != 0 {
		t.Fatalf("the refusal still edited the PR: %v", got)
	}
	if got := ghCalls(*calls); len(got) != 0 {
		t.Fatalf("the body scan must refuse BEFORE any forge call; made %d: %v", len(got), got)
	}
}

// TestEditRefusesASecretInTheTitle — the title is the OTHER surface this verb publishes,
// so it is scanned too. A create whose title carried a credential was already refused;
// an edit that could set the same title without the scan would be the bypass.
func TestEditRefusesASecretInTheTitle(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1")
	bodyPath := writeTempFile(t, "corrected text\nBrief: fixture/01\n")

	rc := run([]string{"edit", "--body-file", bodyPath, "--title", "ghp_" + strings.Repeat("b", 30)})
	if rc != deskkit.ExitRefused {
		t.Fatalf("edit with a credential in the title rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
	}
	if got := editCalls(*calls); len(got) != 0 {
		t.Fatalf("the refusal still edited the PR: %v", got)
	}
}

// TestEditSuccessPostsTheReviewNotice — the whole point of the verb, and the one thing
// no refusal can imitate.
//
// It asserts three things a naive "call gh pr edit" implementation would miss:
//   - the edit argv carries --body-file (a real file, staged by the tool) and --title, and
//     names the PR the branch actually owns (#42, from the listing);
//   - a `gh pr comment` follows it, naming both changed surfaces — because a body edit
//     moves NO head SHA, so a head-keyed review monitor sees nothing otherwise. This is the
//     cell-reported invisibility the verb exists to close;
//   - NOTHING is pushed. `deskpr edit` touches text, never commits.
func TestEditSuccessPostsTheReviewNotice(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1")
	t.Setenv("FAKEGH_PR_BODY", "the original body\nBrief: fixture/01\n")
	t.Setenv("FAKEGH_PR_TITLE", "the original title")
	bodyPath := writeTempFile(t, "the corrected body\nBrief: fixture/01\n")

	rc := run([]string{"edit", "--body-file", bodyPath, "--title", "the corrected title"})
	if rc != deskkit.ExitOK {
		t.Fatalf("edit rc = %d, want 0; calls: %v", rc, *calls)
	}

	edits := editCalls(*calls)
	if len(edits) != 1 {
		t.Fatalf("want exactly 1 `gh pr edit` call, got %d: %v", len(edits), edits)
	}
	e := edits[0]
	if !callContainsAll(e, "pr", "edit", "42", "-R", "example-org/tracker") {
		t.Fatalf("edit argv does not name the branch's own PR: %v", e)
	}
	if argAfter(e, "--body-file") == "" {
		t.Fatalf("edit argv carries no --body-file: %v", e)
	}
	if got := argAfter(e, "--title"); got != "the corrected title" {
		t.Fatalf("edit argv --title = %q, want %q", got, "the corrected title")
	}

	comments := commentCalls(*calls)
	if len(comments) != 1 {
		t.Fatalf("want exactly 1 `gh pr comment` re-review notice, got %d: %v — a body edit moves no head "+
			"SHA, so without this comment the change is invisible to a head-keyed review monitor",
			len(comments), comments)
	}
	if !callContainsAll(comments[0], "pr", "comment", "42", "-R", "example-org/tracker") {
		t.Fatalf("the notice was not posted on the branch's own PR: %v", comments[0])
	}

	if anyCall(gitCalls(*calls), "push") {
		t.Fatalf("edit pushed — it edits TEXT and must never touch commits; calls: %v", gitCalls(*calls))
	}
	if anyGitForce(*calls) {
		t.Fatal("edit emitted a git force flag")
	}

	// The notice must NAME what changed; "something changed" gives a reviewer nothing to
	// act on. Assert on the rendered text rather than on the file the fake consumed.
	notice := reviewNotice([]string{"body", "title"}, true)
	for _, want := range []string{"body and title", "re-review requested", "no head SHA"} {
		if !strings.Contains(notice, want) {
			t.Fatalf("the re-review notice does not mention %q:\n%s", want, notice)
		}
	}
}

// TestEditMayAddATrailerToAPreTrailerPR — the one asymmetry in the trailer rule, and it is
// deliberate. `deskpr update` refuses a PR whose body predates the trailer requirement and
// tells the worker to "edit the body, then re-run update"; before this verb, that
// instruction named no verb at all. A CURRENT body with no link may therefore GAIN one —
// which is not a bypass, because the replacement still faces the full exactly-one grammar.
func TestEditMayAddATrailerToAPreTrailerPR(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1")
	t.Setenv("FAKEGH_PR_BODY", "a body written before the trailer rule existed\n")
	bodyPath := writeTempFile(t, "a body written before the trailer rule existed\nBrief: fixture/01\n")

	rc := run([]string{"edit", "--body-file", bodyPath})
	if rc != deskkit.ExitOK {
		t.Fatalf("edit adding a trailer to a pre-trailer PR rc = %d, want 0; calls: %v", rc, *calls)
	}
	if got := editCalls(*calls); len(got) != 1 {
		t.Fatalf("want exactly 1 `gh pr edit` call, got %d: %v", len(got), got)
	}
}

// TestEditNoopsWhenTheTextAlreadyMatches — update keys its noop on the head SHA; an edit
// moves no head, so the only honest answer to "has this already been done" is the content
// already on the PR. A noop must in particular not post a SECOND re-review comment on a PR
// nothing changed on — repeat notices are how a monitor learns to ignore them.
func TestEditNoopsWhenTheTextAlreadyMatches(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1")
	t.Setenv("FAKEGH_PR_BODY", "already correct\nBrief: fixture/01\n")
	bodyPath := writeTempFile(t, "already correct\nBrief: fixture/01\n")

	rc := run([]string{"edit", "--body-file", bodyPath})
	if rc != deskkit.ExitOK {
		t.Fatalf("edit noop rc = %d, want 0; calls: %v", rc, *calls)
	}
	if got := editCalls(*calls); len(got) != 0 {
		t.Fatalf("the noop still edited the PR: %v", got)
	}
	if got := commentCalls(*calls); len(got) != 0 {
		t.Fatalf("the noop still posted a re-review notice: %v", got)
	}
}

// TestEditReportsALandedEditWhoseNoticeFailed — the notice is not decoration. When the edit
// LANDS but the comment does not post, exit 0 would tell the caller the review desk had
// been informed when it had not, and a bare failure would read as "the edit failed" and
// send them to re-run it. Exit 6 carrying BOTH facts is the only reading that leaves
// nothing silent.
func TestEditReportsALandedEditWhoseNoticeFailed(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1")
	t.Setenv("FAKEGH_PR_BODY", "the original body\nBrief: fixture/01\n")
	t.Setenv("FAKEGH_COMMENT_FAIL", "1")
	bodyPath := writeTempFile(t, "the corrected body\nBrief: fixture/01\n")

	rc := run([]string{"edit", "--body-file", bodyPath})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("edit whose notice failed rc = %d, want %d (unverifiable)", rc, deskkit.ExitUnverifiable)
	}
	if got := editCalls(*calls); len(got) != 1 {
		t.Fatalf("the edit itself should have landed exactly once, got %d: %v", len(got), got)
	}
}

// TestEditRefusesWithoutABodyFile — --body-file is the only way a replacement body reaches
// the gates, so its absence is a refusal before anything else happens.
//
// It asserts on the refusal's MESSAGE, not just its exit code, and that distinction is the
// whole test: with the --body-file check deleted, readBody("", "") hands back an EMPTY body
// which the trailer grammar then refuses for a different reason — same exit 5, same zero
// forge calls, an operator told to add a trailer to a body they never supplied. Pinning the
// text is what makes the mutation visible. cmdEdit is called directly (rather than through
// run) because run reports only the code; the message goes to stderr.
func TestEditRefusesWithoutABodyFile(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1")

	err := cmdEdit([]string{"--title", "a new title"})
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("edit without --body-file rc = %d, want %d (refused)", deskkit.ExitCodeOf(err), deskkit.ExitRefused)
	}
	if !strings.Contains(err.Error(), "--body-file") {
		t.Fatalf("the refusal does not name the missing flag: %v", err)
	}
	if got := ghCalls(*calls); len(got) != 0 {
		t.Fatalf("the refusal still made %d forge call(s): %v", len(got), got)
	}
}

// TestEditGoesThroughThePublicRepoGate — the gate is asked about the PR being edited,
// which is the reactions surface for this write. A +1 on any OTHER number must never
// authorize it, so the ARGUMENTS are pinned, not just the propagated refusal.
func TestEditGoesThroughThePublicRepoGate(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1")

	var gotOwner, gotRepo string
	var gotIssue, called int
	publicRepoGateFn = func(_ deskkit.RepoInfoFetcher, owner, repo string, issueNumber int) error {
		called++
		gotOwner, gotRepo, gotIssue = owner, repo, issueNumber
		return deskkit.Refused("public-repo gate: stub refusal")
	}
	bodyPath := writeTempFile(t, "the corrected body\nBrief: fixture/01\n")

	if rc := run([]string{"edit", "--body-file", bodyPath}); rc != deskkit.ExitRefused {
		t.Fatalf("edit rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
	}
	if called != 1 {
		t.Fatalf("gate called %d times, want exactly 1", called)
	}
	if gotOwner != "example-org" || gotRepo != "tracker" {
		t.Fatalf("gate asked about %s/%s, want example-org/tracker", gotOwner, gotRepo)
	}
	if gotIssue != 42 {
		t.Fatalf("gate asked about #%d, want #42 (the PR being edited) — a +1 on any OTHER number "+
			"must never authorize this write", gotIssue)
	}
	if got := editCalls(*calls); len(got) != 0 {
		t.Fatalf("the gate refused but the PR was still edited: %v", got)
	}
}

// TestEditRefusesWithNoLoopIdentity — edit is an OUTWARD verb, so it faces the same
// $DESK_LOOP check as create: with the variable unset a STOP.<loop> flag a human is
// holding has nothing to match and the halt silently fails.
func TestEditRefusesWithNoLoopIdentity(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	t.Setenv("DESK_LOOP", "")
	t.Setenv("FAKEGH_LIST_HAS_PR", "1")
	bodyPath := writeTempFile(t, "the corrected body\nBrief: fixture/01\n")

	if rc := run([]string{"edit", "--body-file", bodyPath}); rc != deskkit.ExitRefused {
		t.Fatalf("edit with $DESK_LOOP unset rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
	}
	if got := ghCalls(*calls); len(got) != 0 {
		t.Fatalf("the refusal still made %d forge call(s): %v", len(got), got)
	}
}

// TestTrailerLinkIsTotal pins trailerLink's contract directly: it must answer "this body
// asserts no link" for every shape requireTrailer REFUSES, rather than erroring. The
// immutability comparison rests on that totality — a helper that returned a partial value
// for a malformed body could report a spurious CHANGE and refuse a legitimate correction.
func TestTrailerLinkIsTotal(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want string
		ok   bool
	}{
		{"brief", "text\nBrief: fixture/01\n", "Brief: fixture/01", true},
		{"issue", "text\nIssue: #77\n", "Issue: #77", true},
		{"issue no hash", "text\nIssue: 77\n", "Issue: #77", true},
		{"none", "just prose\n", "", false},
		{"both kinds", "Brief: a/01\nIssue: #2\n", "", false},
		{"duplicate", "Brief: a/01\nBrief: b/02\n", "", false},
		{"fenced only", "```\nBrief: a/01\n```\n", "", false},
		{"scan carrier", deskkit.ScanBodyMarker + "\ngenerated\n", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := trailerLink([]byte(tc.body))
			if got != tc.want || ok != tc.ok {
				t.Fatalf("trailerLink = (%q, %v), want (%q, %v)", got, ok, tc.want, tc.ok)
			}
		})
	}
}

// TestEditActorNeverMisnamesItself — the notice says who edited the PR, and a comment that
// misnames its own author is worse than one that is vague. On the ambient-identity path it
// must not claim an App at all.
func TestEditActorNeverMisnamesItself(t *testing.T) {
	t.Setenv("DESK_LOOP", "worker-desk")
	if got := editActor(true); got != "the worker App" {
		t.Fatalf("editActor(as-app) = %q, want %q", got, "the worker App")
	}
	if got := editActor(false); strings.Contains(got, "App") {
		t.Fatalf("editActor(--as-app=false) = %q — the ambient-identity path must not claim an App", got)
	}
}
