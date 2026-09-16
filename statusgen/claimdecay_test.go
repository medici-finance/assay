package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// stubMergedClosedBranches substitutes the PR-state lister for one test, so the
// dead-claim decay is exercised offline — the same injection pattern
// stubRemoteBranches uses for listRemoteBranches.
func stubMergedClosedBranches(t *testing.T, dead map[string]bool, err error) {
	t.Helper()
	prev := listMergedClosedBranches
	listMergedClosedBranches = func(string) (map[string]bool, error) { return dead, err }
	t.Cleanup(func() { listMergedClosedBranches = prev })
}

// stubMergedClosedBranchesGitLab is the GitLab twin of stubMergedClosedBranches:
// it substitutes the merge-request-state reader so the GitLab arm of the decay is
// exercised with no network call (#1111).
func stubMergedClosedBranchesGitLab(t *testing.T, dead map[string]bool, err error) {
	t.Helper()
	prev := listMergedClosedBranchesGitLab
	listMergedClosedBranchesGitLab = func(string) (map[string]bool, error) { return dead, err }
	t.Cleanup(func() { listMergedClosedBranchesGitLab = prev })
}

// TestDecayDeadClaimsDropsMergedAndClosed is the unit-level property: a branch
// whose PR merged or closed is a corpse and must be dropped before it becomes a
// claim; a branch with an OPEN PR — or no PR at all (a worker that pushed but has
// not opened one) — is a live claim and is kept.
func TestDecayDeadClaimsDropsMergedAndClosed(t *testing.T) {
	branches := []string{
		"main",
		"fix/issue-loop-01-live",   // OPEN PR — keep
		"fix/issue-loop-02-merged", // MERGED PR — drop
		"fix/issue-loop-03-closed", // CLOSED PR — drop
		"fix/issue-loop-04-nopr",   // no PR yet — keep
	}
	stubMergedClosedBranches(t, map[string]bool{
		"fix/issue-loop-02-merged": true,
		"fix/issue-loop-03-closed": true,
	}, nil)

	got, reason := decayDeadClaims("/repo", branches)
	want := []string{"main", "fix/issue-loop-01-live", "fix/issue-loop-04-nopr"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decayDeadClaims = %v, want %v", got, want)
	}
	if reason != "" {
		t.Errorf("a decay that RAN must report no could-not-check reason; got %q", reason)
	}
}

// TestDecayDeadClaimsFailsToTheSuperset pins the fail direction: a PR-state read
// error must NOT drop any branch (that would risk dropping a live open-PR claim
// and double-dispatching). It falls back to the full open-branch set — the
// pre-decay behaviour — and says so on stderr.
func TestDecayDeadClaimsFailsToTheSuperset(t *testing.T) {
	branches := []string{"main", "fix/issue-loop-02-merged"}
	stubMergedClosedBranches(t, nil, errors.New("gh: not authenticated"))

	var got []string
	var reason string
	stderr := captureStderr(t, func() { got, reason = decayDeadClaims("/repo", branches) })
	if !reflect.DeepEqual(got, branches) {
		t.Fatalf("failed decay dropped branches: got %v, want the full set %v", got, branches)
	}
	if !strings.Contains(stderr, "could-not-check: claims not decayed") {
		t.Errorf("a failed decay must announce itself on stderr as a could-not-check; got:\n%s", stderr)
	}
	if !strings.Contains(reason, "gh: not authenticated") {
		t.Errorf("a failed decay must hand back the reason so the board can wear it; got %q", reason)
	}
}

// TestResolveClaimsDecaysDeadClaims wires the decay through resolveClaims: the
// merged/closed-PR corpses must not appear as claims, while the live open-PR
// branch still does — and the read is still Known (ls-remote succeeded; the
// board is filtered, not a degraded superset).
func TestResolveClaimsDecaysDeadClaims(t *testing.T) {
	streams := []*Stream{mkStream("issue-loop", "active", "P0",
		Brief{Num: "01", Status: "todo"},
		Brief{Num: "02", Status: "todo"},
		Brief{Num: "03", Status: "todo"},
	)}
	stubRemoteBranches(t, []string{
		"fix/issue-loop-01-live",
		"fix/issue-loop-02-merged",
		"fix/issue-loop-03-closed",
	}, nil)
	stubMergedClosedBranches(t, map[string]bool{
		"fix/issue-loop-02-merged": true,
		"fix/issue-loop-03-closed": true,
	}, nil)

	claimed, src := resolveClaims("/repo", streams)
	if !src.Known {
		t.Error("ls-remote succeeded — the claim source must be Known even after decay")
	}
	if !claimed["issue-loop/01"] {
		t.Error("a live open-PR branch must still claim its brief")
	}
	if claimed["issue-loop/02"] {
		t.Error("a merged-PR corpse must not claim (dead-claim decay)")
	}
	if claimed["issue-loop/03"] {
		t.Error("a closed-PR corpse must not claim (dead-claim decay)")
	}
}

// TestDeadClaimsNoLongerSuppressStream is the board-level regression for the live
// bug: a stream whose entire perStreamCap budget is spent on merged/closed-PR
// corpses shows ZERO rows before decay (its real backlog is silently held), and
// its rows come back once the corpses are decayed. The contrast case proves the
// suppression is real — four LIVE claims at perStreamCap=4 still hold the stream
// to zero, so the decay is not simply disabling the cap.
func TestDeadClaimsNoLongerSuppressStream(t *testing.T) {
	newStream := func() *Stream {
		return mkStream("issue-loop", "active", "P0",
			Brief{Num: "10", Title: "Ten", Status: "todo"},
			Brief{Num: "11", Title: "Eleven", Status: "todo"},
			Brief{Num: "12", Title: "Twelve", Status: "todo"},
			Brief{Num: "13", Title: "Thirteen", Status: "todo"},
			Brief{Num: "14", Title: "Fourteen", Status: "todo"},
			Brief{Num: "15", Title: "Fifteen", Status: "todo"},
		)
	}
	corpses := []string{
		"fix/issue-loop-01-corpse",
		"fix/issue-loop-02-corpse",
		"fix/issue-loop-03-corpse",
		"fix/issue-loop-04-corpse",
	}
	stubRemoteBranches(t, corpses, nil)

	// All four claiming branches are merged/closed corpses → decayed → cap restored.
	deadAll := map[string]bool{}
	for _, b := range corpses {
		deadAll[b] = true
	}
	stubMergedClosedBranches(t, deadAll, nil)
	claimed, src := resolveClaims("/repo", []*Stream{newStream()})
	nuDecayed := nextUp([]*Stream{newStream()}, ClaimView{Claimed: claimed, Source: src}, nil)
	if len(nuDecayed.Picks) == 0 {
		t.Fatalf("decaying dead claims must restore the stream's board rows; got 0 picks (HeldByStreamCap=%d)", nuDecayed.HeldByStreamCap)
	}
	if len(nuDecayed.Picks) != perStreamCap {
		t.Errorf("want %d picks after decay (full perStreamCap budget), got %d", perStreamCap, len(nuDecayed.Picks))
	}

	// Contrast: none of the four PRs are dead (all still OPEN) → all four remain
	// live claims → perStreamCap=4 is fully consumed → the stream is suppressed to
	// zero, exactly as it should be for genuine in-flight work.
	stubMergedClosedBranches(t, map[string]bool{}, nil)
	claimedLive, srcLive := resolveClaims("/repo", []*Stream{newStream()})
	nuLive := nextUp([]*Stream{newStream()}, ClaimView{Claimed: claimedLive, Source: srcLive}, nil)
	if len(nuLive.Picks) != 0 {
		t.Fatalf("four LIVE claims at perStreamCap=%d must still suppress the stream; got %d picks", perStreamCap, len(nuLive.Picks))
	}
	if nuLive.HeldByStreamCap != 6 {
		t.Errorf("HeldByStreamCap = %d, want 6 (all eligible briefs held by the consumed cap)", nuLive.HeldByStreamCap)
	}
}

// ---- the GitHub arm's fork guard (#1147) -----------------------------------

// stubGHPRListJSON substitutes the raw `gh pr list` output for one test, so the
// GitHub reader's PARSE — not just its result — is exercised offline. It is the
// GitHub twin of the GitLab arm's stubGitLabDoer: stubMergedClosedBranches
// replaces the whole reader and so cannot see what the reader does with a row.
func stubGHPRListJSON(t *testing.T, body string, err error) {
	t.Helper()
	prev := ghPRListJSON
	ghPRListJSON = func(string) ([]byte, error) { return []byte(body), err }
	t.Cleanup(func() { ghPRListJSON = prev })
}

// TestForkPRNameNeverDecaysLiveClaim is the end-to-end guard on the pass's
// load-bearing invariant, on the GitHub arm: decay may only ever shrink the
// claim set to what it VERIFIED is dead.
//
// `gh pr list` returns every pull request TARGETING the repository, forks
// included, and a fork's headRefName is a name chosen inside the fork — it names
// nothing in the tracked repository. Matching dead claims by bare headRefName
// therefore let anyone who can fork and open a pull request (the ordinary
// contribution bar — no write access here) open a throwaway PR named after a
// live claim branch, close it, and have the decay drop that live claim: the brief
// goes back on the board and a second worker is dispatched onto work already in
// flight. The GitLab arm closed this in #1135 (sameProjectMR); this is the same
// walk of the whole pass on GitHub, because the claim set is what the invariant
// is about.
func TestForkPRNameNeverDecaysLiveClaim(t *testing.T) {
	stubRemoteOriginURL(t, "https://github.com/example-org/board.git", nil)
	// The tracked repository's own "fix/issue-loop-02-merged" PR really did merge
	// and SHOULD decay. A fork opened and closed a pull request whose headRefName
	// collides with the tracked repository's live claim "fix/issue-loop-01-live";
	// that one must survive untouched.
	stubGHPRListJSON(t, `[
	  {"headRefName":"fix/issue-loop-01-live","state":"CLOSED","isCrossRepository":true,
	   "headRepository":{"name":"board"},"headRepositoryOwner":{"login":"example-forker"}},
	  {"headRefName":"fix/issue-loop-02-merged","state":"MERGED","isCrossRepository":false,
	   "headRepository":{"name":"board"},"headRepositoryOwner":{"login":"example-org"}}
	]`, nil)

	branches := []string{"main", "fix/issue-loop-01-live", "fix/issue-loop-02-merged"}
	var got []string
	var reason string
	stderr := captureStderr(t, func() { got, reason = decayDeadClaims("/repo", branches) })

	want := []string{"main", "fix/issue-loop-01-live"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decay = %v, want %v — the fork PR must not decay the live claim it collides with, and the repository's own merged PR must still decay", got, want)
	}
	if reason != "" {
		t.Errorf("a decay that ran must report no could-not-check reason; got %q", reason)
	}
	if strings.Contains(stderr, "could-not-check") {
		t.Errorf("a fully-attributed listing must report no could-not-check; got:\n%s", stderr)
	}
}

// TestDecaySkipsUnattributedPR pins the fail DIRECTION of the fork check. A pull
// request whose head repository cannot be read (an older `gh` that did not
// answer the field, a fork since deleted) is one this reader could not attribute
// — and an unattributable pull request is indistinguishable from a fork's, whose
// headRefName does not name a branch of this repository at all. Treating "no
// head repository" as "same repository" would put the decay back on the wrong
// side of its own invariant for exactly the rows we understand least. It is
// skipped, counted, and reported — under-decay, never over-decay.
func TestDecaySkipsUnattributedPR(t *testing.T) {
	stubRemoteOriginURL(t, "https://github.com/example-org/board.git", nil)
	stubGHPRListJSON(t, `[{"headRefName":"stream/07","state":"MERGED"}]`, nil)

	var dead map[string]bool
	var err error
	stderr := captureStderr(t, func() { dead, err = listMergedClosedBranches("/repo") })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dead["stream/07"] {
		t.Error("a pull request whose head repository could not be read must NOT decay a claim — it was never attributed to this repository")
	}
	if !strings.Contains(stderr, "could-not-check: dead-claim decay skipped 1 pull request(s)") {
		t.Errorf("skipping an unattributable pull request must be reported, not silent; got:\n%s", stderr)
	}
}

// TestDecaySkipsOtherRepoPR is the belt-and-braces arm: when the tracked
// repository's name is known from `origin`, a row whose head names some OTHER
// repository is one we do not understand and must draw no conclusion from, even
// when the forge's own isCrossRepository flag says false. It is attributed (to
// the wrong place), so it is not counted as unattributable.
func TestDecaySkipsOtherRepoPR(t *testing.T) {
	stubRemoteOriginURL(t, "https://github.com/example-org/board.git", nil)
	stubGHPRListJSON(t, `[
	  {"headRefName":"stream/07","state":"MERGED","isCrossRepository":false,
	   "headRepository":{"name":"board"},"headRepositoryOwner":{"login":"example-other"}},
	  {"headRefName":"stream/08","state":"MERGED","isCrossRepository":false,
	   "headRepository":{"name":"board"},"headRepositoryOwner":{"login":"EXAMPLE-ORG"}}
	]`, nil)

	var dead map[string]bool
	var err error
	stderr := captureStderr(t, func() { dead, err = listMergedClosedBranches("/repo") })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dead["stream/07"] {
		t.Error("a pull request whose head names another repository must not decay this repository's branch")
	}
	if !dead["stream/08"] {
		t.Error("a merged PR whose head IS the tracked repository (case-insensitively) must still decay")
	}
	if strings.Contains(stderr, "could-not-check") {
		t.Errorf("a row attributed to another repository is not UNattributable and must not be reported as such; got:\n%s", stderr)
	}
}

// TestDecayUnknownOriginFallsBackToForgeFlag is the GitHub twin of the GitLab
// arm's path-addressed case: when the tracked repository's name cannot be read
// from `origin`, the forge's own isCrossRepository flag is the whole test — a
// same-repo row still decays, a fork row still does not, and a row carrying only
// a head name (nothing to compare it to) is unattributable and is reported.
func TestDecayUnknownOriginFallsBackToForgeFlag(t *testing.T) {
	stubRemoteOriginURL(t, "", errors.New("no origin"))
	stubGHPRListJSON(t, `[
	  {"headRefName":"stream/07","state":"MERGED","isCrossRepository":false,
	   "headRepository":{"name":"board"},"headRepositoryOwner":{"login":"example-org"}},
	  {"headRefName":"stream/08","state":"CLOSED","isCrossRepository":true,
	   "headRepository":{"name":"board"},"headRepositoryOwner":{"login":"example-forker"}},
	  {"headRefName":"stream/09","state":"MERGED",
	   "headRepository":{"name":"board"},"headRepositoryOwner":{"login":"example-org"}}
	]`, nil)

	var dead map[string]bool
	var err error
	stderr := captureStderr(t, func() { dead, err = listMergedClosedBranches("/repo") })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dead["stream/07"] {
		t.Error("with no tracked name to compare, isCrossRepository=false is the whole test and a merged same-repo PR must decay")
	}
	if dead["stream/08"] {
		t.Error("a fork-sourced pull request must not decay, tracked name known or not")
	}
	if dead["stream/09"] {
		t.Error("a head name with no tracked name to compare it to and no forge flag is unattributable and must not decay")
	}
	if !strings.Contains(stderr, "skipped 1 pull request(s)") {
		t.Errorf("the one unattributable row must be counted and reported; got:\n%s", stderr)
	}
}

// TestGHPRListFieldSetCarriesHeadRepository pins the `--json` field set the
// reader asks `gh` for. The fork guard is only as good as the fields it can see:
// trimmed back to headRefName,state the parse would read every row as
// unattributable and decay nothing (loud, but useless), so the three attribution
// fields are part of the contract.
func TestGHPRListFieldSetCarriesHeadRepository(t *testing.T) {
	for _, f := range []string{"headRefName", "state", "isCrossRepository", "headRepository", "headRepositoryOwner"} {
		if !strings.Contains(","+ghPRListJSONFields+",", ","+f+",") {
			t.Errorf("ghPRListJSONFields = %q, missing %q", ghPRListJSONFields, f)
		}
	}
}
