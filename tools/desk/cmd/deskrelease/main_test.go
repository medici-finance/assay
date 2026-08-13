package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const goodTag = "desk-tools/v0.1.3"

// --- happy path -----------------------------------------------------------------

func TestCutCreatesExactlyOneRefAtMainHead(t *testing.T) {
	h := newHarness(t)

	if code := run([]string{"cut", goodTag}); code != deskkit.ExitOK {
		t.Fatalf("exit %d, want 0; stderr=%s", code, h.errb.String())
	}

	if got := h.gh.refs["tags/"+goodTag]; got != mainSHA {
		t.Fatalf("tag points at %q, want main HEAD %q", got, mainSHA)
	}
	if h.gh.posts != 1 {
		t.Fatalf("made %d POSTs, want exactly 1: %v", h.gh.posts, h.gh.calls())
	}
	// The commit is re-read from the remote at act time, not supplied by the caller.
	if !containsHit(h.gh.calls(), "GET /repos/"+testOwner+"/"+testRepo+"/git/ref/heads/main") {
		t.Fatalf("never re-read origin/main; calls=%v", h.gh.calls())
	}
	// …and the existence of the tag is checked before creating it.
	if !containsHit(h.gh.calls(), "GET /repos/"+testOwner+"/"+testRepo+"/git/ref/tags/"+goodTag) {
		t.Fatalf("never checked whether the tag exists; calls=%v", h.gh.calls())
	}

	entries := h.auditLines(t)
	if len(entries) != 1 {
		t.Fatalf("wrote %d audit lines, want exactly 1: %+v", len(entries), entries)
	}
	e := entries[0]
	if e.Tool != "deskrelease" || e.Result != deskkit.ResultOK || e.Verb != "cut:"+goodTag {
		t.Fatalf("audit line = %+v; want tool=deskrelease verb=cut:%s result=ok", e, goodTag)
	}
	if e.HeadSHA == nil || *e.HeadSHA != mainSHA {
		t.Fatalf("audit HeadSHA = %v; want %s", e.HeadSHA, mainSHA)
	}
	if e.Repo != repoSlug {
		t.Fatalf("audit repo = %q; want %q", e.Repo, repoSlug)
	}
}

// TestActsAsTheDeskAppAndRunsNoGit pins the identity and the "no git verb, ever"
// property: the only external program constructed on any path is desktoken.
func TestActsAsTheDeskAppAndRunsNoGit(t *testing.T) {
	h := newHarness(t)
	if code := run([]string{"cut", goodTag}); code != deskkit.ExitOK {
		t.Fatalf("exit %d, want 0", code)
	}
	argv := h.tok.all()
	if len(argv) != 1 {
		t.Fatalf("ran %d external commands, want exactly 1 (desktoken): %v", len(argv), argv)
	}
	want := []string{deskTokenPath, "desk", "--repo", repoSlug}
	if strings.Join(argv[0], " ") != strings.Join(want, " ") {
		t.Fatalf("external argv = %v; want %v", argv[0], want)
	}
	for _, a := range argv {
		if strings.Contains(a[0], "git") || strings.Contains(a[0], "gh") {
			t.Fatalf("constructed a git/gh command: %v", a)
		}
	}
}

// TestTokenNeverReachesTheAuditOrOutput — the secret stays in memory.
func TestTokenNeverReachesTheAuditOrOutput(t *testing.T) {
	h := newHarness(t)
	run([]string{"cut", goodTag})
	if strings.Contains(h.auditRaw(t), "ghs_FAKE_DESK_APP_TOKEN") {
		t.Fatal("token material landed in the audit log")
	}
	if strings.Contains(h.combined(), "ghs_FAKE_DESK_APP_TOKEN") {
		t.Fatal("token material was printed")
	}
}

// --- refusal paths --------------------------------------------------------------

// TestRefusalPaths is the core security table: every refusal is proven to refuse, with
// the right exit code, and — where it matters — proven to have written NOTHING.
func TestRefusalPaths(t *testing.T) {
	cases := []struct {
		name string
		// setup mutates the fake before the run.
		setup      func(h *harness)
		argv       []string
		wantExit   int
		wantDetail string
		wantPosts  int
		// wantNoCalls asserts the tool refused BEFORE reaching the network at all.
		wantNoCalls bool
	}{
		{
			name:        "malformed tag — wrong namespace",
			argv:        []string{"cut", "tracker/v0.1.3"},
			wantExit:    deskkit.ExitRefused,
			wantDetail:  "does not match",
			wantNoCalls: true,
		},
		{
			name:        "malformed tag — no semver",
			argv:        []string{"cut", "desk-tools/vNEXT"},
			wantExit:    deskkit.ExitRefused,
			wantDetail:  "does not match",
			wantNoCalls: true,
		},
		{
			name:        "second refspec smuggled into the tag",
			argv:        []string{"cut", goodTag + " +HEAD:refs/heads/main"},
			wantExit:    deskkit.ExitRefused,
			wantDetail:  "whitespace",
			wantNoCalls: true,
		},
		{
			name:        "peel-to-commit destination smuggled into the tag",
			argv:        []string{"cut", goodTag + "^{}:refs/heads/main"},
			wantExit:    deskkit.ExitRefused,
			wantDetail:  "refspec metacharacter",
			wantNoCalls: true,
		},
		{
			name:        "extra argv operand",
			argv:        []string{"cut", goodTag, "+HEAD:refs/heads/main"},
			wantExit:    deskkit.ExitRefused,
			wantDetail:  "extra operand",
			wantNoCalls: true,
		},
		{
			name:        "unknown flag",
			argv:        []string{"cut", goodTag, "--force"},
			wantExit:    deskkit.ExitRefused,
			wantDetail:  "unrecognised flag",
			wantNoCalls: true,
		},
		{
			// The load-bearing half of the exists check: an existing tag at a DIFFERENT
			// commit is the case where honouring the request would MOVE a published tag.
			// It stays a hard refusal — the same-commit noop below must never widen to it.
			name: "tag already exists at another commit — never moved",
			setup: func(h *harness) {
				h.gh.refs["tags/"+goodTag] = otherSHA
			},
			argv:       []string{"cut", goodTag},
			wantExit:   deskkit.ExitRefused,
			wantDetail: "already exists",
			wantPosts:  0,
		},
		{
			// Fail closed: "the tag is already where we would put it" must be PROVEN
			// against a resolvable HEAD, never assumed when the read comes back empty.
			name: "tag exists but main cannot be resolved — sameness is never assumed",
			setup: func(h *harness) {
				h.gh.refs["tags/"+goodTag] = mainSHA
				delete(h.gh.refs, "heads/main")
			},
			argv:       []string{"cut", goodTag},
			wantExit:   deskkit.ExitRefused,
			wantDetail: "cannot be resolved",
			wantPosts:  0,
		},
		{
			name: "create races another cut — 422 is a refusal, not a move",
			setup: func(h *harness) {
				h.gh.createStatus = 422
			},
			argv:       []string{"cut", goodTag},
			wantExit:   deskkit.ExitRefused,
			wantDetail: "already exists and is never moved",
			wantPosts:  1,
		},
		{
			name: "main is missing — no tag at an unknown commit",
			setup: func(h *harness) {
				delete(h.gh.refs, "heads/main")
			},
			argv:       []string{"cut", goodTag},
			wantExit:   deskkit.ExitUnverifiable,
			wantDetail: "cannot resolve refs/heads/main",
			wantPosts:  0,
		},
		{
			name: "main does not point at a commit",
			setup: func(h *harness) {
				h.gh.mainType = "tree"
			},
			argv:       []string{"cut", goodTag},
			wantExit:   deskkit.ExitUnverifiable,
			wantDetail: `points at a "tree" object`,
			wantPosts:  0,
		},
		{
			name: "API error on the ref read — fails closed",
			setup: func(h *harness) {
				h.gh.getStatus = 500
			},
			argv:       []string{"cut", goodTag},
			wantExit:   deskkit.ExitUnverifiable,
			wantDetail: "returned HTTP 500",
			wantPosts:  0,
		},
		{
			name: "API error on create — fails closed",
			setup: func(h *harness) {
				h.gh.createStatus = 503
			},
			argv:       []string{"cut", goodTag},
			wantExit:   deskkit.ExitUnverifiable,
			wantDetail: "returned HTTP 503",
			wantPosts:  1,
		},
		{
			name: "created ref does not point where we asked — fails closed",
			setup: func(h *harness) {
				h.gh.createdSHA = otherSHA
			},
			argv:       []string{"cut", goodTag},
			wantExit:   deskkit.ExitUnverifiable,
			wantDetail: "GitHub reports it at",
			wantPosts:  1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			if tc.setup != nil {
				tc.setup(h)
			}
			code := run(tc.argv)
			if code != tc.wantExit {
				t.Fatalf("exit %d, want %d; stderr=%s", code, tc.wantExit, h.errb.String())
			}
			if !strings.Contains(h.errb.String(), tc.wantDetail) {
				t.Fatalf("stderr %q does not mention %q", h.errb.String(), tc.wantDetail)
			}
			if h.gh.posts != tc.wantPosts {
				t.Fatalf("made %d POSTs, want %d: %v", h.gh.posts, tc.wantPosts, h.gh.calls())
			}
			if tc.wantNoCalls && len(h.gh.calls()) != 0 {
				t.Fatalf("refused but still called the API: %v", h.gh.calls())
			}
			if tc.wantPosts == 0 {
				if got := h.gh.mutatingCalls(); len(got) != 0 {
					t.Fatalf("refusal made mutating calls: %v", got)
				}
			}
			// Every refusal is audited exactly once. That single line is what the
			// non-progress circuit breaker counts (#209): it is NOT what
			// the outward-write budget counts, because a refusal never reached GitHub.
			// Auditing zero lines would blind the breaker; auditing two would trip it
			// at half the intended run length.
			entries := h.auditLines(t)
			if len(entries) != 1 {
				t.Fatalf("wrote %d audit lines, want exactly 1: %+v", len(entries), entries)
			}
			if entries[0].Result == deskkit.ResultOK {
				t.Fatalf("audited a refusal as result=ok: %+v", entries[0])
			}
		})
	}
}

// TestArgvValidationPrecedesTheTokenMint — a malformed argument never even causes an
// identity to be minted, so a caller cannot use this tool as a token oracle.
func TestArgvValidationPrecedesTheTokenMint(t *testing.T) {
	h := newHarness(t)
	if code := run([]string{"cut", goodTag, "extra"}); code != deskkit.ExitRefused {
		t.Fatalf("exit %d, want 5", code)
	}
	if n := len(h.tok.all()); n != 0 {
		t.Fatalf("minted a token on a refused invocation (%d external commands)", n)
	}
}

// TestTokenMintFailureFailsClosed — no identity, no act.
func TestTokenMintFailureFailsClosed(t *testing.T) {
	h := newHarness(t)
	fakeDesktoken(t, true) // rebind execCommand to a failing desktoken

	if code := run([]string{"cut", goodTag}); code != deskkit.ExitUnverifiable {
		t.Fatalf("exit %d, want 6", code)
	}
	if len(h.gh.calls()) != 0 {
		t.Fatalf("called the API without a minted identity: %v", h.gh.calls())
	}
}

// --- idempotent re-run ------------------------------------------------------------

// TestReCutAtTheCurrentHeadIsANoopNotABurntVersion pins the one path where a hard
// refusal would COST something. If the create POST errors in transit AFTER GitHub has
// created the ref, the first run exits 6 having done the thing; a retry that refused
// outright would burn a version number, and a burnt number is unrecoverable (a deleted
// tag does not free it — cut-release/SKILL.md says so outright).
//
// The property being pinned is narrow on purpose: the tag already points at the commit
// this invocation would tag anyway, so the requested end state IS the actual state and
// no move is required. Nothing is written. The refusal-table cases above pin the other
// side — a tag at a different commit, or an unresolvable HEAD, still refuses — so this
// noop cannot widen into "an existing tag is fine".
func TestReCutAtTheCurrentHeadIsANoopNotABurntVersion(t *testing.T) {
	h := newHarness(t)
	h.gh.refs["tags/"+goodTag] = mainSHA

	if code := run([]string{"cut", goodTag}); code != deskkit.ExitOK {
		t.Fatalf("exit %d, want 0; stderr=%s", code, h.errb.String())
	}
	if h.gh.posts != 0 {
		t.Fatalf("noop made %d POSTs, want 0: %v", h.gh.posts, h.gh.calls())
	}
	if got := h.gh.mutatingCalls(); len(got) != 0 {
		t.Fatalf("noop made mutating calls: %v", got)
	}
	if got := h.gh.refs["tags/"+goodTag]; got != mainSHA {
		t.Fatalf("the existing tag moved to %q, want it left at %q", got, mainSHA)
	}
	if out := h.out.String(); !strings.Contains(out, "noop") || !strings.Contains(out, "nothing was written") {
		t.Fatalf("noop output does not say nothing was written: %q", out)
	}
	entries := h.auditLines(t)
	if len(entries) != 1 || entries[0].Result != deskkit.ResultNoop {
		t.Fatalf("audit = %+v; want exactly one result=noop line", entries)
	}
}

// --- dry run --------------------------------------------------------------------

func TestDryRunValidatesButWritesNothing(t *testing.T) {
	h := newHarness(t)

	if code := run([]string{"cut", goodTag, "--dry-run"}); code != deskkit.ExitOK {
		t.Fatalf("exit %d, want 0; stderr=%s", code, h.errb.String())
	}
	if h.gh.posts != 0 {
		t.Fatalf("dry run made %d POSTs, want 0: %v", h.gh.posts, h.gh.calls())
	}
	if _, ok := h.gh.refs["tags/"+goodTag]; ok {
		t.Fatal("dry run created the tag")
	}
	// It still did the real reads, so the report is about the real world.
	if !containsHit(h.gh.calls(), "GET /repos/"+testOwner+"/"+testRepo+"/git/ref/heads/main") {
		t.Fatalf("dry run did not read origin/main: %v", h.gh.calls())
	}
	if !strings.Contains(h.out.String(), "dry-run") || !strings.Contains(h.out.String(), "nothing was written") {
		t.Fatalf("dry-run output does not say nothing was written: %q", h.out.String())
	}
	entries := h.auditLines(t)
	if len(entries) != 1 || entries[0].Result != deskkit.ResultDryRun {
		t.Fatalf("dry run audit = %+v; want exactly one result=dryrun line", entries)
	}
}

// TestRehearsingAReleaseCannotLockOutTheRealOne is #214's fourth item, end to end.
//
// A dry run used to be audited `noop`, which the non-progress circuit breaker counts, so
// BreakerTrip consecutive rehearsals opened a 15-minute breaker and the real `cut` that
// followed exited 4. Checking whether a release was safe was enough to prevent it.
//
// The loop runs past BOTH trip points — more than BreakerTrip attempts AND more than
// RateLimitPerPRPerHour of them — so this fails if either meter starts counting rehearsals
// again. It is the end-to-end companion to deskkit's TestDryRunsNeverOpenTheBreaker:
// that one pins the classification, this one pins that the binary emits it.
func TestRehearsingAReleaseCannotLockOutTheRealOne(t *testing.T) {
	h := newHarness(t)

	rehearsals := deskkit.RateLimitPerPRPerHour + deskkit.BreakerTrip
	for i := 0; i < rehearsals; i++ {
		if code := run([]string{"cut", goodTag, "--dry-run"}); code != deskkit.ExitOK {
			t.Fatalf("dry run %d/%d exited %d, want 0 — a rehearsal must never be refused "+
				"by a meter it does not feed; stderr=%s", i+1, rehearsals, code, h.errb.String())
		}
	}
	if h.gh.posts != 0 {
		t.Fatalf("%d dry runs made %d POSTs, want 0: %v", rehearsals, h.gh.posts, h.gh.calls())
	}
	if got := h.gh.mutatingCalls(); len(got) != 0 {
		t.Fatalf("dry runs made mutating calls: %v", got)
	}

	// Every rehearsal is audited (the line is not suppressed, it is CLASSIFIED),
	// and every one carries the class both meters ignore.
	entries := h.auditLines(t)
	if len(entries) != rehearsals {
		t.Fatalf("wrote %d audit lines for %d dry runs, want one each: %+v", len(entries), rehearsals, entries)
	}
	for i, e := range entries {
		if e.Result != deskkit.ResultDryRun {
			t.Fatalf("dry-run audit line %d has result %q, want %q — `noop` is counted by the "+
				"breaker and is what locked the real cut out (#214)", i, e.Result, deskkit.ResultDryRun)
		}
	}

	// The real cut now runs, and lands. This is the assertion the issue is about.
	if code := run([]string{"cut", goodTag}); code != deskkit.ExitOK {
		t.Fatalf("the real cut after %d rehearsals exited %d, want 0 — rehearsing a release "+
			"must not be able to lock out the release; stderr=%s", rehearsals, code, h.errb.String())
	}
	if got := h.gh.refs["tags/"+goodTag]; got != mainSHA {
		t.Fatalf("tag points at %q, want main HEAD %q", got, mainSHA)
	}
	if h.gh.posts != 1 {
		t.Fatalf("made %d POSTs after the real cut, want exactly 1: %v", h.gh.posts, h.gh.calls())
	}
}

// TestDryRunResultIsOnlyReachableWithoutAWrite pins the property that makes the meter
// exemption safe rather than a laundering route: the ONLY invocation shape that produces
// a `dryrun` audit line is one that creates no ref. If a writing path could emit it, a
// caller would get a real write for free on both meters.
//
// It is asserted from the outside — over the binary's real argv surface — because that
// is where a laundering route would have to exist.
func TestDryRunResultIsOnlyReachableWithoutAWrite(t *testing.T) {
	cases := []struct {
		name string
		argv []string
	}{
		{"the real cut", []string{"cut", goodTag}},
		{"a refused tag", []string{"cut", "bogus/v0.0.1"}},
		{"a refused tag with the flag", []string{"cut", "bogus/v0.0.1", "--dry-run"}},
		{"the flag given twice", []string{"cut", goodTag, "--dry-run", "--dry-run"}},
		{"an extra operand alongside the flag", []string{"cut", goodTag, "--dry-run", "extra"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			run(c.argv)
			for _, e := range h.auditLines(t) {
				if e.Result != deskkit.ResultDryRun {
					continue
				}
				if h.gh.posts > 0 {
					t.Fatalf("argv %v created a ref AND audited result=dryrun — that shape "+
						"launders a real write past both rate-limit meters", c.argv)
				}
				// Nothing was written, but a REFUSAL must still be audited `refused`:
				// misclassifying one as `dryrun` would hide it from the breaker and
				// re-open the refusal loop the breaker exists to stop.
				t.Fatalf("argv %v audited result=dryrun, but only a successful `--dry-run` "+
					"may emit that class — every other outcome must keep the class its own "+
					"meter reads (refused/noop/ok)", c.argv)
			}
		})
	}
}

func TestDryRunStillRefusesAMalformedTag(t *testing.T) {
	h := newHarness(t)
	if code := run([]string{"cut", "desk-tools/v1.2.3:main", "--dry-run"}); code != deskkit.ExitRefused {
		t.Fatalf("exit %d, want 5", code)
	}
	if len(h.gh.calls()) != 0 {
		t.Fatalf("--dry-run reached the API with a malformed tag: %v", h.gh.calls())
	}
}

// --- scope ----------------------------------------------------------------------

// TestRepoOutsideTheFixedSetIsRefused exercises the repo-scope gate. repoSlug is a package var
// ONLY so this test can move it; no flag or env reaches it in production.
func TestRepoOutsideTheFixedSetIsRefused(t *testing.T) {
	h := newHarness(t)
	old := repoSlug
	repoSlug = "attacker/elsewhere"
	t.Cleanup(func() { repoSlug = old })

	if code := run([]string{"cut", goodTag}); code != deskkit.ExitRefused {
		t.Fatalf("exit %d, want 5", code)
	}
	if !strings.Contains(h.errb.String(), "outside the fixed desk repo set") {
		t.Fatalf("stderr %q does not name the repo-scope refusal", h.errb.String())
	}
	if len(h.gh.calls()) != 0 {
		t.Fatalf("acted on an out-of-set repo: %v", h.gh.calls())
	}
}

// TestNoVerbBeyondCut — the tool has exactly one act. Anything else is a usage error,
// never a silently-accepted alias.
func TestNoVerbBeyondCut(t *testing.T) {
	for _, verb := range []string{"delete", "move", "force", "retag", "release", "push", "tag", "override"} {
		t.Run(verb, func(t *testing.T) {
			h := newHarness(t)
			if code := run([]string{verb, goodTag}); code != 2 {
				t.Fatalf("subcommand %q exited %d, want 2 (usage)", verb, code)
			}
			if len(h.gh.calls()) != 0 || h.gh.posts != 0 {
				t.Fatalf("subcommand %q reached the API: %v", verb, h.gh.calls())
			}
		})
	}
}

// --- argv is the only input -----------------------------------------------------

// TestTagIsNeverTakenFromTheEnvironmentOrStdin — a caller who controls the environment
// or the process's stdin still cannot name a tag.
func TestTagIsNeverTakenFromTheEnvironmentOrStdin(t *testing.T) {
	h := newHarness(t)
	for _, k := range []string{"TAG", "RELEASE_TAG", "DESKRELEASE_TAG", "GITHUB_REF_NAME"} {
		t.Setenv(k, goodTag)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.WriteString(goodTag + "\n")
	w.Close()
	oldStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = oldStdin })

	if code := run([]string{"cut"}); code != 2 {
		t.Fatalf("exit %d, want 2 (usage) — a tag was picked up from outside argv", code)
	}
	if _, ok := h.gh.refs["tags/"+goodTag]; ok {
		t.Fatal("created a tag named by the environment/stdin")
	}
	if len(h.gh.calls()) != 0 {
		t.Fatalf("reached the API with no argv tag: %v", h.gh.calls())
	}
	if len(h.auditLines(t)) != 0 {
		t.Fatal("a bare usage error consumed rate-limit budget")
	}
}

// --- kill switch ----------------------------------------------------------------

func TestKillSwitchRunsBeforeAnythingElse(t *testing.T) {
	h := newHarness(t)
	t.Setenv("DESK_TOOLS_DISABLED", "1")

	if code := run([]string{"cut", goodTag}); code != deskkit.ExitDisabled {
		t.Fatalf("exit %d, want 3", code)
	}
	if len(h.gh.calls()) != 0 || len(h.tok.all()) != 0 {
		t.Fatalf("acted while disabled: api=%v exec=%v", h.gh.calls(), h.tok.all())
	}
}

// --- rate limit -----------------------------------------------------------------

// TestRefusalLoopIsStopped pins that a looping caller must be cut off. Since
// #209 the instrument that cuts it off is the non-progress CIRCUIT BREAKER,
// not the outward-write budget — a `refused` never reached the remote, so charging it to
// a budget denominated in remote amplification starved every other writer of the same
// tool for an hour (a full window of refusals, none of them writes).
//
// The guarantee this test pins is therefore UNCHANGED IN INTENT AND STRONGER IN EFFECT:
// the loop is stopped at BreakerTrip attempts instead of RateLimitPerPRPerHour, i.e. sooner.
// The bound is RateLimitPerPRPerHour rather than BreakerTrip on purpose: expressing it in
// terms of the constant under test would make the assertion self-referential — raising
// BreakerTrip to 500 would move the goalposts with it and the test could not fail. Stated
// this way it pins the property that matters: whatever the breaker is tuned to, a loop is
// cut off within the number of attempts the OLD budget would have allowed, so this change
// never lets a runaway run longer than it used to.
func TestRefusalLoopIsStopped(t *testing.T) {
	h := newHarness(t)
	stoppedAt := 0
	for i := 1; i <= deskkit.RateLimitPerPRPerHour; i++ {
		code := run([]string{"cut", "bogus/v0.0.1"})
		if code == deskkit.ExitRateLimited {
			stoppedAt = i
			break
		}
		if code != deskkit.ExitRefused {
			t.Fatalf("attempt %d exited %d, want 5 (refused) or 4 (stopped)", i, code)
		}
	}
	if stoppedAt == 0 {
		t.Fatalf("a refusal loop ran %d times without ever being stopped — a looping "+
			"caller must be cut off", deskkit.RateLimitPerPRPerHour)
	}
	if h.gh.posts != 0 {
		t.Fatalf("rate-limited run still POSTed: %v", h.gh.calls())
	}
}

// NOTE: the companion property — "local refusals do not spend the outward-write budget"
// — is deliberately NOT asserted here. An end-to-end version of it cannot fail: the
// breaker caps a refusal run at 5, which is under RateLimitPerPRPerHour, so the subsequent cut
// succeeds under BOTH the old and the new charging rule and the test would be decorative.
// It is pinned where it is observable instead, in
// deskkit's TestRefusalsDoNotConsumeTheWriteBudget, which is mutation-checked.

// --- misc -----------------------------------------------------------------------

func TestVersionAndHelpDoNotAct(t *testing.T) {
	for _, argv := range [][]string{{"version"}, {"--version"}, {"help"}, {"--help"}} {
		h := newHarness(t)
		if code := run(argv); code != 0 {
			t.Fatalf("%v exited %d, want 0", argv, code)
		}
		if len(h.gh.calls()) != 0 {
			t.Fatalf("%v reached the API", argv)
		}
	}
}

func TestNoArgsIsUsage(t *testing.T) {
	h := newHarness(t)
	if code := run(nil); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if len(h.gh.calls()) != 0 {
		t.Fatal("no-args reached the API")
	}
}

// TestCreateTagRefRefusesAMalformedSHA pins the precondition on the ONE mutating call.
//
// It is reachable only from planCut, which today can pass it nothing but a sha already
// validated by getRef — so the check is redundant against the current call graph, and a
// mutation removing it left the suite green. It is kept rather than deleted, and pinned
// here, for one reason: it is the last thing standing between a future edit to planCut
// and a POST body carrying something that is not an object id. A precondition on the only
// write is exactly where a redundant belt earns its place — but a guard nothing exercises
// is a claim, not an enforcement, and this PR's framing says every guard is mutation-
// checked. This test makes that true.
func TestCreateTagRefRefusesAMalformedSHA(t *testing.T) {
	h := newHarness(t)
	c, err := newGHClient(repoSlug)
	if err != nil {
		t.Fatalf("newGHClient: %v", err)
	}

	bad := []string{
		"",
		"not-a-sha",
		strings.Repeat("a", 39),                  // one short
		strings.Repeat("a", 41),                  // one long
		strings.Repeat("A", 40),                  // uppercase hex is not a git object id
		mainSHA + " +refs/heads/main",            // a second refspec
		"refs/heads/main",                        // a ref, not an object id
		mainSHA[:39] + "z",                       // non-hex
		mainSHA + "\n" + strings.Repeat("b", 40), // embedded newline (RE2 $ is \z)
	}
	for _, s := range bad {
		t.Run(fmt.Sprintf("%q", s), func(t *testing.T) {
			got, cerr := c.createTagRef(goodTag, s)
			if cerr == nil {
				t.Fatalf("createTagRef accepted the malformed sha %q (returned %q)", s, got)
			}
			if !deskkit.IsUnverifiable(cerr) {
				t.Fatalf("createTagRef(%q) = %v; want unverifiable (exit 6)", s, cerr)
			}
			if !strings.Contains(cerr.Error(), "malformed sha") {
				t.Fatalf("createTagRef(%q) error %q does not name the malformed sha", s, cerr)
			}
		})
	}

	// The point is not the error — it is that no write ever left the process.
	if h.gh.posts != 0 {
		t.Fatalf("made %d POSTs with a malformed sha, want 0: %v", h.gh.posts, h.gh.calls())
	}
	if got := h.gh.mutatingCalls(); len(got) != 0 {
		t.Fatalf("malformed sha still produced mutating calls: %v", got)
	}
}

func containsHit(hits []string, want string) bool {
	for _, h := range hits {
		if h == want {
			return true
		}
	}
	return false
}

// --- the public-repo gate on the tag-cut path ---
//
// `deskrelease cut` creates a tag ref via POST /repos/{owner}/{repo}/git/refs — an
// outward write, and one with no associated issue/PR, so there is no reactions surface
// on which a ada +1 could live. On a PUBLIC repo the gate can therefore never be
// satisfied and must refuse before the POST.
//
// The gate is INERT against the shipped default, whose visibility the fixture
// reports as private, so these tests drive the fake rather than the slug.
// They exist because the gate shipped untested — an early review removed the
// PublicRepoGate call and `go test ./cmd/deskrelease/` still passed. The whole value
// of an inert-but-future-proofing gate is that it still works when the slug widens,
// and only a test can hold that.

// TestPublicRepoGateRefusesTagCutOnPublicRepo — public repo, no reactions surface:
// exit 6, and crucially ZERO POSTs. The exit code alone would not distinguish a
// refusal from a tag that was created and then reported as failed.
func TestPublicRepoGateRefusesTagCutOnPublicRepo(t *testing.T) {
	h := newHarness(t)
	h.gh.visibility = "public"

	if code := run([]string{"cut", goodTag}); code != deskkit.ExitUnverifiable {
		t.Fatalf("cut on a PUBLIC repo: exit = %d, want %d (unverifiable) — the public-repo "+
			"gate is not wired into deskrelease", code, deskkit.ExitUnverifiable)
	}
	if h.gh.posts != 0 {
		t.Fatalf("POSTs = %d, want 0 — a tag was cut on a public repo behind the gate", h.gh.posts)
	}
	if _, exists := h.gh.refs["tags/"+goodTag]; exists {
		t.Fatalf("tag %q exists after a refusal", goodTag)
	}
}

// TestPublicRepoGateFailsClosedWhenVisibilityUnreadable — the liveness cost of the
// gate, pinned deliberately rather than discovered in production: a 500 / rate-limit /
// token hiccup on GET /repos now turns a release cut into exit 6. That is the correct
// direction (never guess "private"), but it IS a new failure mode on the release path
// and this test is where that trade is recorded.
func TestPublicRepoGateFailsClosedWhenVisibilityUnreadable(t *testing.T) {
	h := newHarness(t)
	h.gh.visStatus = http.StatusInternalServerError

	if code := run([]string{"cut", goodTag}); code != deskkit.ExitUnverifiable {
		t.Fatalf("unreadable visibility: exit = %d, want %d (unverifiable) — never guess private",
			code, deskkit.ExitUnverifiable)
	}
	if h.gh.posts != 0 {
		t.Fatalf("POSTs = %d, want 0 — cut a tag without establishing visibility", h.gh.posts)
	}
}

// TestPublicRepoGatePassesPrivateRepo — the counterweight: the gate must not break the
// real path. Without this, "always refuse" would satisfy the two tests above.
func TestPublicRepoGatePassesPrivateRepo(t *testing.T) {
	h := newHarness(t)
	h.gh.visibility = "private"

	if code := run([]string{"cut", goodTag}); code != deskkit.ExitOK {
		t.Fatalf("cut on a PRIVATE repo: exit = %d, want 0; stderr=%s", code, h.errb.String())
	}
	if h.gh.posts != 1 {
		t.Fatalf("POSTs = %d, want 1", h.gh.posts)
	}
}
