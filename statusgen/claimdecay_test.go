package main

import (
	"encoding/json"
	"errors"
	"fmt"
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

// stubGHPRListJSON substitutes the GitHub reader's raw output for one test. It
// takes pull-request ROWS (the shape the fork-guard fixtures below are written
// in) and serves them as the branch read returns them: one branch per distinct
// headRefName, its pull requests newest first in row order, and the branch's tip
// equal to the head commit of its first row (so a merged or closed PR still
// sitting at the tip is a corpse). A row may set headRefOid / tip explicitly.
func stubGHPRListJSON(t *testing.T, rows string, err error) {
	t.Helper()
	var body []byte
	if err == nil {
		var prs []map[string]any
		if jerr := json.Unmarshal([]byte(rows), &prs); jerr != nil {
			t.Fatalf("fixture rows: %v", jerr)
		}
		order := []string{}
		byName := map[string][]map[string]any{}
		tips := map[string]string{}
		for i, pr := range prs {
			name, _ := pr["headRefName"].(string)
			if _, ok := pr["headRefOid"]; !ok {
				pr["headRefOid"] = fmt.Sprintf("%040x", i+1)
			}
			if _, seen := byName[name]; !seen {
				order = append(order, name)
				tip, _ := pr["tip"].(string)
				if tip == "" {
					tip, _ = pr["headRefOid"].(string)
				}
				tips[name] = tip
			}
			delete(pr, "tip")
			byName[name] = append(byName[name], pr)
		}
		nodes := []map[string]any{}
		for _, name := range order {
			nodes = append(nodes, map[string]any{
				"name":                   name,
				"target":                 map[string]any{"oid": tips[name]},
				"associatedPullRequests": map[string]any{"nodes": byName[name]},
			})
		}
		body, _ = json.Marshal([]any{map[string]any{"data": map[string]any{"repository": map[string]any{"refs": map[string]any{"nodes": nodes}}}}})
	}
	prev := ghBranchPRJSON
	ghBranchPRJSON = func(string) ([]byte, error) { return body, err }
	t.Cleanup(func() { ghBranchPRJSON = prev })
}

// TestForkPRNameNeverDecaysLiveClaim is the end-to-end guard on the pass's
// load-bearing invariant, on the GitHub arm: decay may only ever shrink the
// claim set to what it VERIFIED is dead.
//
// A pull-request listing can return pull requests TARGETING the repository, forks
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

// TestGHBranchQueryCarriesAttributionAndHead pins the fields the branch read asks
// for. The fork guard is only as good as the fields it can see, and the corpse
// test needs the branch tip and the PR head to compare: trimmed, the parse would
// read every row as unattributable or every corpse as live.
func TestGHBranchQueryCarriesAttributionAndHead(t *testing.T) {
	for _, f := range []string{"headRefName", "headRefOid", "state", "isCrossRepository", "headRepository", "headRepositoryOwner", "target{oid}", "pageInfo{hasNextPage endCursor}", "direction:DESC"} {
		if !strings.Contains(ghBranchPRQuery, f) {
			t.Errorf("ghBranchPRQuery is missing %q", f)
		}
	}
}

// TestDecayReadsBranchesNotPRHistory pins the corpse rule the branch read
// exists for. A branch is dead only when NO same-repo pull request on it is open,
// its NEWEST one is merged or closed, AND its tip is still that PR's head; the old all-PRs read
// keyed on the name alone (any PR ever on the name made it dead) and stopped at
// 1,000 PRs (an old corpse never decayed).
func TestDecayReadsBranchesNotPRHistory(t *testing.T) {
	stubRemoteOriginURL(t, "https://github.com/example-org/board.git", nil)
	same := `"isCrossRepository":false,"headRepository":{"name":"board"},"headRepositoryOwner":{"login":"example-org"}`
	stubGHPRListJSON(t, `[
	  {"number":12,"headRefName":"fix/old-01-corpse","state":"CLOSED",`+same+`},
	  {"number":3001,"headRefName":"fix/reused-02","state":"OPEN","headRefOid":"`+strings.Repeat("c", 40)+`",`+same+`},
	  {"number":40,"headRefName":"fix/reused-02","state":"MERGED","headRefOid":"`+strings.Repeat("c", 40)+`",`+same+`},
	  {"number":70,"headRefName":"fix/older-open-05","state":"CLOSED","headRefOid":"`+strings.Repeat("d", 40)+`",`+same+`},
	  {"number":65,"headRefName":"fix/older-open-05","state":"OPEN","headRefOid":"`+strings.Repeat("d", 40)+`",`+same+`},
	  {"number":50,"headRefName":"fix/repushed-03","state":"CLOSED","headRefOid":"`+strings.Repeat("a", 40)+`","tip":"`+strings.Repeat("b", 40)+`",`+same+`},
	  {"number":60,"headRefName":"fix/merged-04","state":"MERGED",`+same+`}
	]`, nil)

	dead, err := listMergedClosedBranches("/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dead["fix/old-01-corpse"] {
		t.Error("a closed PR's branch still at the PR head is a corpse however old the PR is (the 1,000-PR cap hid these)")
	}
	if dead["fix/reused-02"] {
		t.Error("a branch whose NEWEST PR is open is a live claim, even though an older PR on the same name merged")
	}
	if dead["fix/older-open-05"] {
		t.Error("a branch with an OPEN PR is a live claim even when a newer PR from it closed at the same tip")
	}
	if dead["fix/repushed-03"] {
		t.Error("a branch pushed again after its PR closed (tip moved past the PR head) is new work, not the corpse")
	}
	if !dead["fix/merged-04"] {
		t.Error("a merged PR's branch still at the PR head must decay")
	}
}

// TestDecayIgnoresPRHeadedElsewhere pins the headRefName filter: a pull request
// the forge associates with the branch but whose head is a DIFFERENT branch name
// says nothing about this branch, even merged at the branch's own tip.
func TestDecayIgnoresPRHeadedElsewhere(t *testing.T) {
	tip := strings.Repeat("e", 40)
	no := false
	b := ghBranch{Name: "fix/x-06"}
	b.Target = &struct {
		Oid string `json:"oid"`
	}{Oid: tip}
	b.AssociatedPullRequests.Nodes = []ghPR{{Number: 80, HeadRefName: "fix/other-07", HeadRefOid: tip, State: "MERGED", IsCrossRepository: &no}}
	if dead, _, _ := branchIsDead(b, ""); dead {
		t.Fatal("a pull request headed by another branch must not decay this one")
	}
	b.AssociatedPullRequests.Nodes[0].HeadRefName = "fix/x-06"
	if dead, _, _ := branchIsDead(b, ""); !dead {
		t.Fatal("fixture drift: the same pull request headed by this branch must decay it")
	}
}

// TestDecayGraphQLErrorIsCouldNotCheck pins the fail direction for a listing
// that came back with a GraphQL error: an error, so decayDeadClaims keeps the
// whole superset, never a partial dead set read as complete.
func TestDecayGraphQLErrorIsCouldNotCheck(t *testing.T) {
	prev := ghBranchPRJSON
	ghBranchPRJSON = func(string) ([]byte, error) {
		return []byte(`[{"data":{"repository":null},"errors":[{"message":"rate limited"}]}]`), nil
	}
	t.Cleanup(func() { ghBranchPRJSON = prev })
	if _, err := listMergedClosedBranches("/repo"); err == nil || !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("a GraphQL error must be returned as an error naming it; got %v", err)
	}
	got, reason := decayDeadClaims("/repo", []string{"main", "fix/x-01"})
	if len(got) != 2 || reason == "" {
		t.Fatalf("a failed read must keep every branch and carry a could-not-check reason; got %v %q", got, reason)
	}
}

// windowRows returns n pull-request rows for one branch, newest first, all from
// this repository. Each carries its own head commit (stubGHPRListJSON's default),
// and the branch's tip is the first row's, so the newest row is a corpse at the
// tip. open, when >= 0, makes that row (0 = newest) OPEN instead of CLOSED.
func windowRows(branch string, n, open int) string {
	rows := make([]string, 0, n)
	for i := 0; i < n; i++ {
		state := "CLOSED"
		if i == open {
			state = "OPEN"
		}
		rows = append(rows, fmt.Sprintf(`{"number":%d,"headRefName":%q,"state":%q,"isCrossRepository":false,`+
			`"headRepository":{"name":"board"},"headRepositoryOwner":{"login":"example-org"}}`, 900-i, branch, state))
	}
	return "[" + strings.Join(rows, ",") + "]"
}

// TestGHPRWindowMatchesQuery keeps the window the corpse test treats as full equal
// to the window the query asks for: raise one without the other and either a full
// window reads as not-full (an open pull request behind it is missed again) or
// every branch reads as truncated.
// regression: #3023
func TestGHPRWindowMatchesQuery(t *testing.T) {
	want := fmt.Sprintf("associatedPullRequests(first:%d,", ghPRWindow)
	if !strings.Contains(ghBranchPRQuery, want) {
		t.Fatalf("ghBranchPRQuery must ask for %q (ghPRWindow = %d)", want, ghPRWindow)
	}
}

// TestDecayFullWindowIsNotDead pins the window rule (assay#3023): a branch whose
// per-branch window comes back full with no open pull request may have an open one
// behind the window, so it is NOT decayed and is reported as could-not-check. One
// row short of full, the same shape is a plain corpse; a full window that does hold
// the open pull request is live, with nothing to report.
// regression: #3023
func TestDecayFullWindowIsNotDead(t *testing.T) {
	stubRemoteOriginURL(t, "https://github.com/example-org/board.git", nil)
	run := func(t *testing.T, rows string) (map[string]bool, string) {
		t.Helper()
		stubGHPRListJSON(t, rows, nil)
		var dead map[string]bool
		var err error
		stderr := captureStderr(t, func() { dead, err = listMergedClosedBranches("/repo") })
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return dead, stderr
	}
	const branch = "fix/busy-01"

	dead, stderr := run(t, windowRows(branch, ghPRWindow, -1))
	if dead[branch] {
		t.Error("a full window with no open pull request may hide an open one behind it: the branch must not decay")
	}
	if !strings.Contains(stderr, fmt.Sprintf("could-not-check: dead-claim decay saw a full window of %d pull requests", ghPRWindow)) {
		t.Errorf("a truncated branch must be reported as could-not-check, not silent; got:\n%s", stderr)
	}

	dead, stderr = run(t, windowRows(branch, ghPRWindow-1, -1))
	if !dead[branch] {
		t.Error("fixture drift: a window one row short of full is the whole history and its newest closed PR at the tip is a corpse")
	}
	if strings.Contains(stderr, "could-not-check") {
		t.Errorf("a window that was not full has nothing to report; got:\n%s", stderr)
	}

	dead, stderr = run(t, windowRows(branch, ghPRWindow, ghPRWindow-1))
	if dead[branch] {
		t.Error("an open pull request inside the window — however old — keeps the branch live")
	}
	if strings.Contains(stderr, "could-not-check") {
		t.Errorf("a full window that holds the open pull request is fully decided and has nothing to report; got:\n%s", stderr)
	}
}

// TestDecayNewestClosedWins pins "the NEWEST counted pull request
// decides" against the one mutation the other fixtures cannot see: picking the
// OLDEST closed pull request. Branch a has an older closed PR (other head) behind
// a newer one that sits at the tip, so it is a corpse only if the newer one is
// read. Branch b is the inverse: the older closed PR's head is the tip but the
// newest closed PR's head is not, so the branch is not that newest PR's corpse.
// regression: #3023
func TestDecayNewestClosedWins(t *testing.T) {
	stubRemoteOriginURL(t, "https://github.com/example-org/board.git", nil)
	same := `"isCrossRepository":false,"headRepository":{"name":"board"},"headRepositoryOwner":{"login":"example-org"}`
	a, b, c, d := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40), strings.Repeat("d", 40)
	stubGHPRListJSON(t, `[
	  {"number":21,"headRefName":"fix/two-closed-a","state":"CLOSED","headRefOid":"`+a+`",`+same+`},
	  {"number":11,"headRefName":"fix/two-closed-a","state":"MERGED","headRefOid":"`+b+`",`+same+`},
	  {"number":22,"headRefName":"fix/two-closed-b","state":"CLOSED","headRefOid":"`+c+`","tip":"`+d+`",`+same+`},
	  {"number":12,"headRefName":"fix/two-closed-b","state":"MERGED","headRefOid":"`+d+`",`+same+`}
	]`, nil)

	dead, err := listMergedClosedBranches("/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dead["fix/two-closed-a"] {
		t.Error("the newest closed PR at the tip makes the branch a corpse, whatever an older closed PR's head was")
	}
	if dead["fix/two-closed-b"] {
		t.Error("only the NEWEST closed PR's head is compared to the tip; an older PR's head at the tip must not decay the branch")
	}
}

// TestDecayNullRepoNoErrors pins the refusal for a page that
// carries no `errors` yet no repository either (a null or missing repository,
// as GitHub answers for a repository the token cannot see). Reading it as "no
// branches, nothing dead" would be a clean-looking could-not-check; it must be an
// error, so decayDeadClaims keeps the whole superset and carries the reason.
// regression: #3023
func TestDecayNullRepoNoErrors(t *testing.T) {
	for name, page := range map[string]string{
		"null repository":    `[{"data":{"repository":null}}]`,
		"missing repository": `[{"data":{}}]`,
		"second page null":   `[{"data":{"repository":{"refs":{"nodes":[]}}}},{"data":{"repository":null}}]`,
	} {
		t.Run(name, func(t *testing.T) {
			prev := ghBranchPRJSON
			ghBranchPRJSON = func(string) ([]byte, error) { return []byte(page), nil }
			t.Cleanup(func() { ghBranchPRJSON = prev })

			if dead, err := listMergedClosedBranches("/repo"); err == nil || !strings.Contains(err.Error(), "no repository") {
				t.Fatalf("a page with no repository and no errors must be refused naming it; got dead=%v err=%v", dead, err)
			}
			var got []string
			var reason string
			captureStderr(t, func() { got, reason = decayDeadClaims("/repo", []string{"main", "fix/x-01"}) })
			if len(got) != 2 || !strings.Contains(reason, "no repository") {
				t.Fatalf("a refused read must keep every branch and carry the reason; got %v %q", got, reason)
			}
		})
	}
}
