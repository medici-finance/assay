package main

// enact_test.go — the enactment gate brought up to the ruling's narrowed text: the acceptance
// sits on the ONE configured sign-off thread, is created after the latest change to R-8's
// text that the register's path history records, is the authority's newest acceptance on that
// thread, and opens with a bare `Enact: R-8` line. Every case here only NARROWS what
// enacts, or turns an enactment into could-not-check.
//
// Every fixture value is an example-org placeholder.

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// assertEnacted runs an admission recompute on a PR not yet in the lane and requires the
// admission label to be written — the gate held.
func assertEnacted(t *testing.T, e *env) string {
	t.Helper()
	notYetInLane(e)
	code, stdout, stderr := e.run(verbRecompute, "7")
	if code != deskkit.ExitOK || !strings.Contains(stdout, "admitted:") {
		t.Fatalf("exit %d stdout %q stderr %q — want the admission written", code, stdout, stderr)
	}
	if w := e.fg.writes(); len(w) != 1 || w[0].Op != "ApplyLabels" {
		t.Fatalf("writes %v — want exactly the admission label", w)
	}
	return stdout
}

// dryMerge runs `merge --dry-run`, which returns the enactment gate's own outcome.
func dryMerge(e *env) (int, string) {
	code, _, stderr := e.run(verbMerge, "7", "--dry-run", "--fpy-file", e.fpy(healthyFPY))
	return code, stderr
}

// signOnThread points R-8's Sign-off line at a comment on another thread of the register repo.
func signOnThread(e *env, thread string, commentID string) {
	url := "https://github.com/example-org/tracker/issues/" + thread + "#issuecomment-" + commentID
	e.fg.rulings = strings.Replace(rulingsSigned, fxSignURL, url, 1)
}

// --- (a) the sign-off thread is pinned --------------------------------------------------------

// TestAutoLane_EnactRefuses_OtherThread — the Sign-off names a genuine acceptance by the
// blessing authority, in the register's repo, but on a thread other than the configured one:
// refused before the thread is fetched. Fail-first: at 7cbc29f any thread in the register
// repo was followed, and this admitted.
func TestAutoLane_EnactRefuses_OtherThread(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	signOnThread(e, "4", "556")
	e.fg.comments[4] = []deskkit.Comment{{DatabaseID: 556, Author: deskkit.Account{Login: "ada", ID: 2001, Type: "User"},
		Body: fxEnactBody, CreatedAt: fxAccepted}}
	assertNotEnacted(t, e, "not on the configured sign-off thread #3")
	for _, c := range e.fg.calls {
		if c.Op == "ListCommentsTyped" {
			t.Fatalf("the gate fetched a thread other than the configured one: %v", c)
		}
	}
}

// TestAutoLane_UnsetThreadIs_CouldNotCheck — the lane is configured with no sign-off thread:
// the enactment gate is could-not-check (exit 6), never "any thread", and nothing is written.
// Fail-first: at 7cbc29f there was no thread key, and this enacted.
func TestAutoLane_UnsetThreadIs_CouldNotCheck(t *testing.T) {
	e := install(t, fixtureLaneKeysNoThread, rulingsSigned)
	code, stderr := dryMerge(e)
	if code != deskkit.ExitUnverifiable || !strings.Contains(stderr, "no sign-off thread is configured") {
		t.Fatalf("exit %d stderr %q — want 6 naming the unset thread", code, stderr)
	}
	e = install(t, fixtureLaneKeysNoThread, rulingsSigned)
	assertNotEnacted(t, e, "no sign-off thread is configured")
}

// --- (b) the acceptance postdates the ruling's text ---------------------------------------------

// TestAutoLane_EnactRefuses_AcceptanceBeforeText — the acceptance comment was created BEFORE
// the PR that last changed R-8's text merged: it accepted an earlier text, and enacts nothing.
// A comment created at the very merge instant is not after it either. Fail-first: at 7cbc29f
// the comment's time was never read, and both admitted.
func TestAutoLane_EnactRefuses_AcceptanceBeforeText(t *testing.T) {
	for _, at := range []string{"2025-12-31T23:59:59Z", fxTextMerged} {
		e := install(t, fixtureLaneKeys, rulingsSigned)
		e.fg.comments[3][0].CreatedAt = at
		assertNotEnacted(t, e, "not after R-8's current text merged (#21")
	}
}

// TestAutoLane_EnactIgnores_CommitDate — the commit that changed R-8's text carries its own
// committed date, and the gate must never read it: only the merging PR's merged_at anchors.
// Backdated: the commit claims a date before the acceptance, but its PR merged AFTER the
// acceptance was created, so the acceptance is of an earlier text and is refused. Forward-dated:
// the commit claims a date after the acceptance, but its PR merged before it, so the lane
// enacts. Fail-first: the mutation "the commit's own date anchors" admits the first and refuses
// the second.
func TestAutoLane_EnactIgnores_CommitDate(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.history = []histEntry{{sha: fxTextSHA, date: "2025-06-01T00:00:00Z"}}
	e.fg.merged[fxTextPR] = mergedPR(fxTextPR, "2026-01-03T00:00:00Z")
	assertNotEnacted(t, e, "not after R-8's current text merged (#21, merged 2026-01-03T00:00:00Z)")

	e = install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.history = []histEntry{{sha: fxTextSHA, date: "2026-02-01T00:00:00Z"}}
	assertEnacted(t, e)
}

// TestAutoLane_EnactAnchors_LatestMergedPR — the commit that changed the text is behind two
// merged PRs; the anchor is the LATER merge, never the earlier one and never a commit date.
func TestAutoLane_EnactAnchors_LatestMergedPR(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.commitPRs[fxTextSHA] = []int{fxTextPR, 23}
	e.fg.merged[23] = mergedPR(23, "2026-01-05T00:00:00Z")
	assertNotEnacted(t, e, "(#23, merged 2026-01-05T00:00:00Z)")
}

// TestAutoLane_EnactRefuses_TextWithoutMergedPR — the latest change to R-8's text has no PR
// merged into the default branch behind it (a direct push, an open PR, or a PR merged into
// another branch): refused.
func TestAutoLane_EnactRefuses_TextWithoutMergedPR(t *testing.T) {
	cases := map[string]func(e *env){
		"no change behind the commit": func(e *env) { e.fg.commitPRs[fxTextSHA] = nil },
		"an open change": func(e *env) {
			e.fg.merged[fxTextPR] = deskkit.PullRequest{Number: fxTextPR, State: "open", BaseRef: "main"}
		},
		"merged into another branch": func(e *env) {
			pr := mergedPR(fxTextPR, fxTextMerged)
			pr.BaseRef = "side-branch"
			e.fg.merged[fxTextPR] = pr
		},
	}
	for name, mut := range cases {
		e := install(t, fixtureLaneKeys, rulingsSigned)
		mut(e)
		code, stderr := dryMerge(e)
		if code != deskkit.ExitRefused || !strings.Contains(stderr, "has no change merged into main behind it") {
			t.Fatalf("%s: exit %d stderr %q — want refused, no merged change", name, code, stderr)
		}
	}
}

// TestAutoLane_TimeCheckUnreadable_CouldNotCheck — every read the time check needs, failing or
// unusable, is could-not-check (exit 6), never an enactment and never an "unsigned".
func TestAutoLane_TimeCheckUnreadable_CouldNotCheck(t *testing.T) {
	cases := map[string]func(e *env){
		"history unreadable": func(e *env) { e.fg.fail["ListFileCommits"] = true },
		"changes unreadable": func(e *env) { e.fg.fail["ListCommitChanges"] = true },
		"no creation time":   func(e *env) { e.fg.comments[3][0].CreatedAt = "" },
		"merge time missing": func(e *env) {
			e.fg.merged[fxTextPR] = deskkit.PullRequest{Number: fxTextPR, State: "closed", Merged: true, BaseRef: "main"}
		},
		"history is not the default branch's": func(e *env) {
			e.fg.history = []histEntry{{sha: fxTextSHA, content: strings.Replace(rulingsSigned, "Text.", "Older text.", 1)}}
		},
		"history page ends before the change": func(e *env) {
			e.fg.history = nil
			for i := 0; i < rulingHistoryLimit; i++ {
				e.fg.history = append(e.fg.history, histEntry{sha: fxTextSHA + string(rune('a'+i%26)) + string(rune('a'+i/26))})
			}
		},
	}
	for name, mut := range cases {
		e := install(t, fixtureLaneKeys, rulingsSigned)
		mut(e)
		code, stderr := dryMerge(e)
		if code != deskkit.ExitUnverifiable || !strings.Contains(stderr, "could-not-check: "+condRulingSigned) {
			t.Fatalf("%s: exit %d stderr %q — want 6 could-not-check", name, code, stderr)
		}
		if w := e.fg.writes(); len(w) != 0 {
			t.Fatalf("%s: writes %v", name, w)
		}
	}
}

// --- (c) the Enact line opens the body, typed bare ---------------------------------------------

// TestAutoLane_EnactRefuses_QuotedOrLateLine — the authority's comment carries `Enact: R-8`
// only quoted, fenced, backticked, indented or after other text: a mention, not an act.
// Fail-first: at 7cbc29f the matcher took the line anywhere, and the late, fenced and indented
// bodies admitted. The indented body pins the first-column rule, which is stricter than a
// ruling text that ignores leading whitespace (see AutoLaneAcceptance).
func TestAutoLane_EnactRefuses_QuotedOrLateLine(t *testing.T) {
	for _, body := range []string{
		"> Enact: R-8",
		"`Enact: R-8`",
		"```\nEnact: R-8\n```",
		"  Enact: R-8",
		"Looks right to me.\n\nEnact: R-8",
		"To arm the lane a human would reply:\n\n```\nEnact: R-8\n```\n\nI am holding off for now.",
	} {
		e := install(t, fixtureLaneKeys, rulingsSigned)
		e.fg.comments[3][0].Body = body
		assertNotEnacted(t, e, "first non-empty line is not `Enact: R-8` typed bare")
	}
}

// --- (d) a Sign-off-only change does not move the anchor ---------------------------------------

// TestAutoLane_SignOffOnlyChange_KeepsAnchor — the newest commit to the register only fills the
// Sign-off line, and the PR carrying it merged AFTER the acceptance was created. It does not
// move the anchor: the text last changed at #21, before the acceptance, so the lane enacts.
// A change to ANOTHER ruling merged after the acceptance does not move it either. Fail-first:
// a time check that counted the Sign-off line (or the whole file) refused this.
func TestAutoLane_SignOffOnlyChange_KeepsAnchor(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.history = []histEntry{{sha: "bbbb0002"}, {sha: fxTextSHA, content: rulingsUnsigned}}
	e.fg.commitPRs["bbbb0002"] = []int{22}
	e.fg.merged[22] = mergedPR(22, "2026-01-03T00:00:00Z")
	assertEnacted(t, e)

	other := strings.Replace(rulingsSigned, "# Rulings\n", "# Rulings\n\n## R-9 — other\n\nNine.\n\n**Sign-off:**\n", 1)
	e = install(t, fixtureLaneKeys, other)
	e.fg.history = []histEntry{{sha: "cccc0003"}, {sha: "bbbb0002", content: rulingsSigned}, {sha: fxTextSHA, content: rulingsUnsigned}}
	e.fg.commitPRs["cccc0003"] = []int{24}
	e.fg.merged[24] = mergedPR(24, "2026-01-04T00:00:00Z")
	assertEnacted(t, e)
}

// --- (e) the real signing order admits ---------------------------------------------------------

// TestAutoLane_RealSigningOrder_Admits — the order a human actually follows: the narrowed text
// merges (#21, over an older text from #20), THEN the acceptance comment is posted on the
// sign-off thread, THEN a PR fills the Sign-off line with its permalink (#22). The gate walks
// past the Sign-off fill to #21, finds the acceptance after it, and enacts. The same history
// with the acceptance posted between #20 and #21 — an acceptance of the OLD text — does not.
func TestAutoLane_RealSigningOrder_Admits(t *testing.T) {
	older := strings.Replace(rulingsUnsigned, "Text.", "Older, wider text.", 1)
	order := func(e *env) {
		e.fg.history = []histEntry{
			{sha: "bbbb0002"},                          // Sign-off fill (#22), merged after the acceptance
			{sha: fxTextSHA, content: rulingsUnsigned}, // the narrowed text (#21)
			{sha: "0000aaaa", content: older},          // the older text (#20)
		}
		e.fg.commitPRs["bbbb0002"] = []int{22}
		e.fg.commitPRs["0000aaaa"] = []int{20}
		e.fg.merged[22] = mergedPR(22, "2026-01-03T00:00:00Z")
		e.fg.merged[20] = mergedPR(20, "2025-12-01T00:00:00Z")
	}
	e := install(t, fixtureLaneKeys, rulingsSigned)
	order(e)
	code, stdout, stderr := e.run(verbCheck, "--fpy-file", e.fpy(healthyFPY))
	if code != deskkit.ExitOK || !strings.Contains(stdout, "(#21 merged "+fxTextMerged+")") {
		t.Fatalf("check: exit %d stdout %q stderr %q — want enacted, anchored at #21", code, stdout, stderr)
	}
	e = install(t, fixtureLaneKeys, rulingsSigned)
	order(e)
	assertEnacted(t, e)

	e = install(t, fixtureLaneKeys, rulingsSigned)
	order(e)
	e.fg.comments[3][0].CreatedAt = "2025-12-15T00:00:00Z"
	assertNotEnacted(t, e, "not after R-8's current text merged (#21")
}

// --- (f) the named acceptance is the authority's newest ----------------------------------------

// laterComment is a comment on the sign-off thread, created after fxAccepted.
func laterComment(login string, id int64, typ, body, at string) deskkit.Comment {
	return deskkit.Comment{DatabaseID: 557, Author: deskkit.Account{Login: login, ID: id, Type: typ}, Body: body, CreatedAt: at}
}

// TestAutoLane_EnactRefuses_Superseded — the Sign-off names an acceptance the blessing
// authority has since replaced with a later acceptance on the sign-off thread: refused. The
// second case is the restore the simplified path history hides: the text changed after the
// first acceptance and the authority accepted it again, then a merge restored the old
// register (old text, old Sign-off). The history the forge lists shows only the old text, so
// the time check passes; the thread's later acceptance refuses it. Fail-first: before this
// step both admitted.
func TestAutoLane_EnactRefuses_Superseded(t *testing.T) {
	later := laterComment("ada", 2001, "User", fxEnactBody, "2026-01-05T00:00:00Z")
	e := install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.comments[3] = append(e.fg.comments[3], later)
	assertNotEnacted(t, e, "the sign-off artifact is superseded")

	e = install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.history = []histEntry{{sha: "bbbb0002"}, {sha: fxTextSHA, content: rulingsUnsigned}}
	e.fg.commitPRs["bbbb0002"] = []int{22}
	e.fg.merged[22] = mergedPR(22, "2026-01-09T00:00:00Z")
	e.fg.comments[3] = append(e.fg.comments[3], later)
	assertNotEnacted(t, e, "the sign-off artifact is superseded")
}

// TestAutoLane_EnactKeeps_NotSuperseded — later comments on the thread that are NOT a later
// acceptance by the blessing authority supersede nothing: a later non-acceptance by the
// authority, a later acceptance line from another User, one from an App carrying the
// authority's login, and an EARLIER acceptance by the authority all leave the lane enacted.
// Nobody but the authority can refuse the lane by posting on the thread.
func TestAutoLane_EnactKeeps_NotSuperseded(t *testing.T) {
	for name, c := range map[string]deskkit.Comment{
		"authority, not an acceptance": laterComment("ada", 2001, "User", "Noted, thanks.", "2026-01-05T00:00:00Z"),
		"another user's acceptance":    laterComment("shared-agent", 2002, "User", fxEnactBody, "2026-01-05T00:00:00Z"),
		"an app with the login":        laterComment("ada", 2001, "Bot", fxEnactBody, "2026-01-05T00:00:00Z"),
		"an earlier acceptance":        laterComment("ada", 2001, "User", fxEnactBody, "2026-01-01T12:00:00Z"),
	} {
		t.Run(name, func(t *testing.T) {
			e := install(t, fixtureLaneKeys, rulingsSigned)
			e.fg.comments[3] = append(e.fg.comments[3], c)
			assertEnacted(t, e)
		})
	}
}

// TestAutoLane_SupersedeUnreadable — a later acceptance by the authority with no readable
// creation time: the named one cannot be shown to be the newest, so could-not-check (exit 6).
func TestAutoLane_SupersedeUnreadable(t *testing.T) {
	e := install(t, fixtureLaneKeys, rulingsSigned)
	e.fg.comments[3] = append(e.fg.comments[3], laterComment("ada", 2001, "User", fxEnactBody, ""))
	code, stderr := dryMerge(e)
	if code != deskkit.ExitUnverifiable || !strings.Contains(stderr, "cannot be shown to be the newest") {
		t.Fatalf("exit %d stderr %q — want 6 could-not-check", code, stderr)
	}
	if w := e.fg.writes(); len(w) != 0 {
		t.Fatalf("writes %v", w)
	}
}
