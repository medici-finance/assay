package main

// deskflip_test.go — the verb's behavioural suite, driven through the RECORDING HTTP
// TRANSPORT in harness_test.go.
//
// THE 1:1 MAP from the retired `gh`-argv assertions to their successors on the recorder.
// Every argv assertion this file used to make has a named successor below; none was dropped,
// and each successor checks at least as much as its predecessor did:
//
//	ran("pr ready 7")                → flipped()           — POST /graphql whose BODY carries
//	                                                         markPullRequestReadyForReview
//	                                                         (the argv form could not see a body)
//	ran("--remove-label X")          → saw(DELETE, "/labels/X")
//	ran("--add-label Y")             → sawBody(POST, "/labels", "Y")
//	ran("/reviews")                  → saw(GET, "/reviews")
//	ran("-q .headRefOid")            → count(GET, "/pulls/7") >= 2 (the re-read is a SECOND
//	                                                         change read on the seam)
//	ran("/files") + "--paginate"     → saw(GET, "/files") with per_page=100&page=N on the query
//	fileReads                        → count(GET, "/files")
//	mutated() over 5 argv verbs      → mutated() over EVERY non-GET request — broader, since it
//	                                                         needs no verb list to keep current
//	GH_TOKEN reaches the child       → every recorded request carries the minted token in its
//	                                                         Authorization header (identity_test.go)
//	gh refuses with no minted token  → the backend refuses to build a client without one, and
//	                                                         issues zero requests (identity_test.go)
//	flipPRGraphQL brace balance      → TestPRStateReadNeedsNoActionsRead (workflowrun_test.go):
//	                                                         the constant is gone, and what it
//	                                                         existed to protect — a PR-state read
//	                                                         that does not need `actions:read` —
//	                                                         is asserted directly

import (
	"net/http"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestFlipsWhenEveryConditionHolds(t *testing.T) {
	s := newStub()
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitOK {
		t.Fatalf("flip rc = %d, want 0", rc)
	}
	if !s.flipped() {
		t.Errorf("the ready mutation never ran: %v", s.requests)
	}
	if !s.saw(http.MethodDelete, "/labels/"+labelBeforeFlip) ||
		!s.sawBody(http.MethodPost, "/labels", labelAfterFlip) {
		t.Errorf("the queue labels were not swapped: %v", s.requests)
	}
	// The TOCTOU re-read must happen, and it must happen BEFORE the mutation. On the seam the
	// re-read is a SECOND change read, so the ordering assertion is "the last change read
	// precedes the mutation".
	readIdx := s.lastIndexOf(http.MethodGet, "/pulls/7")
	flipIdx := s.indexOf(http.MethodPost, "/graphql")
	if readIdx < 0 || flipIdx < 0 || readIdx > flipIdx {
		t.Errorf("last head re-read at %d, flip at %d — the re-read is the LAST thing before the mutation",
			readIdx, flipIdx)
	}
	if s.count(http.MethodGet, "/pulls/7") < 2 {
		t.Errorf("the change was read %d time(s) — the pre-mutation head re-read is a second read",
			s.count(http.MethodGet, "/pulls/7"))
	}
}

// The flip belongs to the role that watched the review.
func TestWrongCallerRoleRefusesBeforeReadingAnything(t *testing.T) {
	s := newStub()
	s.install(t)
	t.Setenv("DESK_LOOP", "worker-desk")

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("wrong role rc = %d, want %d", rc, deskkit.ExitRefused)
	}
	if len(s.requests) != 0 {
		t.Errorf("the role gate read forge state before refusing: %v", s.requests)
	}
}

func TestUnsetCallerRoleRefuses(t *testing.T) {
	s := newStub()
	s.install(t)
	t.Setenv("DESK_LOOP", "")
	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("unset role rc = %d, want %d", rc, deskkit.ExitRefused)
	}
}

// An unrecognised loop name is could-not-check, not "some other role".
func TestUnknownCallerRoleIsUnverifiable(t *testing.T) {
	s := newStub()
	s.install(t)
	t.Setenv("DESK_LOOP", "pr-reveiw-desk")
	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("unknown role rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
}

// An approval at a DIFFERENT head is STALE — a distinct answer from "no verdict", and the
// refusal must say so.
func TestStaleApprovalRefused(t *testing.T) {
	s := newStub()
	s.install(t)
	s.reviews = approvalAtHead(t, "0000000011111111222222223333333344444444")

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("stale approval rc = %d, want %d", rc, deskkit.ExitRefused)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("a stale approval still produced mutations: %v", m)
	}
}

func TestNoVerdictRefused(t *testing.T) {
	s := newStub()
	s.install(t)
	s.reviews = nil
	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("no verdict rc = %d, want %d", rc, deskkit.ExitRefused)
	}
}

// THE FORGERY GUARD. An APPROVED that immediately follows a CHANGES_REQUESTED at the SAME
// head verifies nothing new — there is no new code to verify — so it must not clear the
// standing block. The forge's self-approval block only keys on the PR AUTHOR account and has
// nothing to say about a third-party App re-posting a verdict at an unchanged head.
func TestApprovalOverAStandingBlockAtTheSameHeadIsRefused(t *testing.T) {
	s := newStub()
	s.install(t)
	bot := reviewerBot(t)
	blocked := reviewInfo{State: "CHANGES_REQUESTED", CommitID: headSHA, Body: "findings", SubmittedAt: "2026-01-01T00:00:00Z"}
	blocked.User.Login = bot
	approved := reviewInfo{State: "APPROVED", CommitID: headSHA, Body: "ok now", SubmittedAt: "2026-01-01T00:00:30Z"}
	approved.User.Login = bot
	s.reviews = []reviewInfo{blocked, approved}

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("same-head re-approval rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("a same-head re-approval produced mutations: %v", m)
	}
}

// A SECURITY pass must never satisfy the CORRECTNESS gate. That confusion has exactly one
// fail-open direction, and this is it.
func TestSecurityPassDoesNotSatisfyTheCorrectnessGate(t *testing.T) {
	s := newStub()
	s.install(t)
	sec := reviewInfo{State: "APPROVED", CommitID: headSHA, Body: "Security-Review: pass", SubmittedAt: "2026-01-01T00:00:00Z"}
	sec.User.Login = reviewerBot(t)
	s.reviews = []reviewInfo{sec}

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("security-only verdict rc = %d, want %d — a security pass is not a correctness approval",
			rc, deskkit.ExitRefused)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("mutations on a security-only verdict: %v", m)
	}
}

func TestRedCheckRefusedAndNamed(t *testing.T) {
	s := newStub()
	s.rollup = []rollupEntry{
		{Name: "test", Status: "COMPLETED", Conclusion: "SUCCESS"},
		{Name: "lint", Status: "COMPLETED", Conclusion: "FAILURE"},
	}
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("red check rc = %d, want %d", rc, deskkit.ExitRefused)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("mutations over red CI: %v", m)
	}
}

// Not-yet-green is could-not-verify, never green.
func TestPendingCheckIsUnverifiable(t *testing.T) {
	s := newStub()
	s.rollup = []rollupEntry{{Name: "test", Status: "IN_PROGRESS"}}
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("pending check rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
}

// An EMPTY rollup on a CI-required repo means the checks have not reported — not green.
func TestEmptyRollupOnCIRequiredRepoIsUnverifiable(t *testing.T) {
	s := newStub()
	s.rollup = nil
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("empty rollup on a CI repo rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
}

// NEW, and a strengthening the transport swap made possible: each CI rollup now carries the
// forge's OWN asserted total, so a rollup that serves fewer entries than the head claims is a
// rollup nobody read in full. That is could-not-check, never green — the same fail-closed
// shape the changed-files reconcile has always had, which the `gh pr view` rollup could not
// express at all.
func TestRollupServingLessThanItAssertsIsUnverifiable(t *testing.T) {
	s := newStub()
	s.checkTotalOverride = 5 // asserts 5 check runs, serves 1
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("truncating rollup rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("mutations over a rollup that was not read in full: %v", m)
	}
}

// The two RFC3339 stamps every latest-run-per-name fixture below is built from. tOld sorts
// before tNew lexicographically, which is also chronologically — the property recencyKey
// relies on.
const (
	tOld = "2026-01-01T00:00:00Z"
	tNew = "2026-01-01T01:00:00Z"
)

// TestSupersededRunDoesNotJamACleanFlip: a check NAME carrying a superseded older run PLUS a
// green LATEST run must not jam the flip. Both live jams observed on this public repo are
// reproduced as table rows — a CANCELLED predecessor (a `changelog` run cancelled by the push
// + pull_request double-trigger) and a stale QUEUED orphan (a `control-sweep` run the same
// double-trigger left queued).
func TestSupersededRunDoesNotJamACleanFlip(t *testing.T) {
	cases := map[string][]rollupEntry{
		// A cancelled predecessor + the re-triggered success, same NAME. The success is newer
		// (later CompletedAt), so it is the latest run and the flip is clean.
		"cancelled-predecessor": {
			{Name: "changelog", Status: "COMPLETED", Conclusion: "CANCELLED", StartedAt: tOld, CompletedAt: tOld},
			{Name: "changelog", Status: "COMPLETED", Conclusion: "SUCCESS", StartedAt: tNew, CompletedAt: tNew},
		},
		// An orphaned queued run carries NO timestamps, so recencyKey sorts it oldest and the
		// completed success is the latest run.
		"stale-queued-orphan": {
			{Name: "control-sweep", Status: "QUEUED"},
			{Name: "control-sweep", Status: "COMPLETED", Conclusion: "SUCCESS", StartedAt: tNew, CompletedAt: tNew},
		},
	}
	for name, rollup := range cases {
		t.Run(name, func(t *testing.T) {
			s := newStub()
			s.rollup = rollup
			s.install(t)
			s.reviews = approvalAtHead(t, headSHA)

			if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitOK {
				t.Fatalf("superseded run jammed the flip: rc = %d, want 0 — only the LATEST run per name counts", rc)
			}
			if !s.flipped() {
				t.Errorf("the ready mutation never ran: %v", s.requests)
			}
		})
	}
}

// TestLatestRedRunStillRefuses: the reduction must NEVER relax the gate. When the LATEST run
// of a name is red — an older SUCCESS superseded by a newer FAILURE — the flip must still
// refuse.
func TestLatestRedRunStillRefuses(t *testing.T) {
	s := newStub()
	s.rollup = []rollupEntry{
		{Name: "build", Status: "COMPLETED", Conclusion: "SUCCESS", StartedAt: tOld, CompletedAt: tOld},
		{Name: "build", Status: "COMPLETED", Conclusion: "FAILURE", StartedAt: tNew, CompletedAt: tNew},
	}
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("a red LATEST run did not refuse: rc = %d, want %d — the reduction must not weaken the gate",
			rc, deskkit.ExitRefused)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("mutations over a red latest run: %v", m)
	}
}

// TestLatestPerRollupNameReducesByRecency exercises the reducer directly: newest-per-name
// wins, both forge shapes are keyed by their own identity, a stampless run loses, nameless
// entries are never collapsed, and a genuine multi-name set is preserved.
func TestLatestPerRollupNameReducesByRecency(t *testing.T) {
	in := []rollupEntry{
		{Name: "a", Status: "COMPLETED", Conclusion: "FAILURE", CompletedAt: tOld},
		{Name: "a", Status: "COMPLETED", Conclusion: "SUCCESS", CompletedAt: tNew},
		{Name: "b", Status: "QUEUED"}, // stampless
		{Name: "b", Status: "COMPLETED", Conclusion: "SUCCESS", CompletedAt: tNew},
		{Context: "legacy", State: "PENDING", CreatedAt: tOld},
		{Context: "legacy", State: "SUCCESS", CreatedAt: tNew},
		{Status: "COMPLETED", Conclusion: "SUCCESS"}, // nameless
		{Status: "COMPLETED", Conclusion: "FAILURE"}, // nameless — must NOT merge with the above
	}
	got := latestPerRollupName(in)
	// a, b, legacy reduced to one each; the two nameless entries kept standalone → 5.
	if len(got) != 5 {
		t.Fatalf("reduced to %d entries, want 5: %+v", len(got), got)
	}
	byName := map[string]rollupEntry{}
	nameless := 0
	for _, e := range got {
		if e.groupKey() == "" {
			nameless++
			continue
		}
		byName[e.groupKey()] = e
	}
	if byName["a"].Conclusion != "SUCCESS" {
		t.Errorf("name a kept %q, want the newer SUCCESS", byName["a"].Conclusion)
	}
	if byName["b"].Status != "COMPLETED" || byName["b"].Conclusion != "SUCCESS" {
		t.Errorf("name b kept %+v, want the completed SUCCESS over the stampless QUEUED", byName["b"])
	}
	if byName["legacy"].State != "SUCCESS" {
		t.Errorf("context legacy kept %q, want the newer SUCCESS", byName["legacy"].State)
	}
	if nameless != 2 {
		t.Errorf("nameless entries collapsed to %d, want 2 (no identity to reduce by)", nameless)
	}
	// The reduced set is what evalRollup then judges — and it evaluates it UNCHANGED. The set
	// here still carries the standalone nameless FAILURE, so the reduction has not swallowed a
	// real red: the judgement stays ciFail.
	if st := evalRollup(got); st != ciFail {
		t.Errorf("reduced set evaluated to %v, want ciFail — the standalone nameless FAILURE must survive", st)
	}
}

func TestConflictingPRRefused(t *testing.T) {
	s := newStub()
	s.pr.Mergeable = deskkit.MergeableConflicting
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("conflicting rc = %d, want %d", rc, deskkit.ExitRefused)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("mutations on a conflicting PR: %v", m)
	}
}

// UNKNOWN mergeability is the forge saying "not computed yet". Unknown is not mergeable.
func TestUnknownMergeabilityIsUnverifiable(t *testing.T) {
	s := newStub()
	s.pr.Mergeable = deskkit.MergeableUnknown
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("unknown mergeability rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
}

// A PUBLIC repo is risk-classed unconditionally — no diff reading required, and no quiet path
// out of the security gate.
func TestPublicRepoNeedsASecurityPassAtHead(t *testing.T) {
	s := newStub()
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", publicRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("public repo without a security pass rc = %d, want %d", rc, deskkit.ExitRefused)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("mutations on a risk-classed PR with no security verdict: %v", m)
	}
}

func TestPublicRepoFlipsWithASecurityPassAtHead(t *testing.T) {
	s := newStub()
	s.install(t)
	bot := reviewerBot(t)
	sec := reviewInfo{State: "COMMENTED", CommitID: headSHA, Body: "Security-Review: pass", SubmittedAt: "2026-01-01T00:01:00Z"}
	sec.User.Login = bot
	s.reviews = append(approvalAtHead(t, headSHA), sec)

	if rc := run([]string{"7", "--repo", publicRepo}); rc != deskkit.ExitOK {
		t.Fatalf("public repo with both artifacts rc = %d, want 0", rc)
	}
	if !s.flipped() {
		t.Error("the flip did not happen with both artifacts at head")
	}
}

// A `Security-Review: fail` at head is a deliberate RETRACTION. It blocks EVERY flip,
// risk-classed or not — whether the diff happens to trigger risk classification has nothing to
// do with whether the retraction was made.
func TestSecurityFailAtHeadBlocksEvenOnANonRiskClassedPR(t *testing.T) {
	s := newStub()
	s.install(t)
	bot := reviewerBot(t)
	fail := reviewInfo{State: "COMMENTED", CommitID: headSHA, Body: "Security-Review: fail", SubmittedAt: "2026-01-01T00:01:00Z"}
	fail.User.Login = bot
	s.reviews = append(approvalAtHead(t, headSHA), fail)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("security fail rc = %d, want %d", rc, deskkit.ExitRefused)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("mutations over a standing security retraction: %v", m)
	}
}

// A pass RETRACTED by a later fail at the same head is not green. The reduction is
// order-sensitive on purpose.
func TestLaterFailRetractsAnEarlierPassAtTheSameHead(t *testing.T) {
	s := newStub()
	s.install(t)
	bot := reviewerBot(t)
	pass := reviewInfo{State: "COMMENTED", CommitID: headSHA, Body: "Security-Review: pass", SubmittedAt: "2026-01-01T00:01:00Z"}
	pass.User.Login = bot
	fail := reviewInfo{State: "COMMENTED", CommitID: headSHA, Body: "Security-Review: fail", SubmittedAt: "2026-01-01T00:02:00Z"}
	fail.User.Login = bot
	s.reviews = append(approvalAtHead(t, headSHA), pass, fail)

	if rc := run([]string{"7", "--repo", publicRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("retracted pass rc = %d, want %d", rc, deskkit.ExitRefused)
	}
}

// A short files read makes the risk-class determination unverifiable, not clean: pad a PR with
// enough files ahead of the risky one and a gate that trusted a truncated walk would waive
// itself.
func TestShortFilesReadIsUnverifiable(t *testing.T) {
	s := newStub()
	s.pr.ChangedFiles = 300
	s.files = greenFiles()
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("short files read rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
}

// The regression this file exists for. A 163-file PR is served across two pages, and the gate
// has to reach its risk-class determination on all 163 — not refuse at the page boundary.
func TestWalksEveryPageOfTheDiff(t *testing.T) {
	const total = 163
	s := newStub()
	s.pr.ChangedFiles = total
	s.files = paddingFiles(total)
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitOK {
		t.Fatalf("163-file risk-clear PR rc = %d, want 0 — the walk did not read past page one", rc)
	}
	if !s.flipped() {
		t.Error("the ready mutation never ran")
	}
	// The list really did span pages: the backend asks for 100 per page, so 163 cannot fit in
	// one, and the recorder must show more than one changed-files request.
	if s.count(http.MethodGet, "/files") < 2 {
		t.Fatalf("the changed-files endpoint was read %d time(s) — a 163-entry list spans pages",
			s.count(http.MethodGet, "/files"))
	}
}

// The other half, and the one that matters more: a risky path sitting PAST the page boundary
// must still risk-class the PR. A reader that stops at page one sees only quiet docs paths —
// so with the reconcile removed it would have called this diff clean and flipped a workflow
// change on a correctness review alone.
func TestRiskPathPastPageOneIsSeen(t *testing.T) {
	const total = 163
	files := append(paddingFiles(total-1), riskyPath)
	if !deskkit.RiskPathTriggered(privateCIRepo, []string{riskyPath}) {
		t.Fatalf("%s is no longer a risk-classed path — this test's premise is gone", riskyPath)
	}
	s := newStub()
	s.pr.ChangedFiles = total
	s.files = files
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("risk path on page two rc = %d, want %d (refused for want of a security pass)",
			rc, deskkit.ExitRefused)
	}
	if m := s.mutated(); len(m) > 0 {
		t.Errorf("a refused flip still mutated: %v", m)
	}
}

// Walking the pages does NOT relax the fail-closed branch. A forge that asserts 163 and serves
// 100 with no second page is a diff nobody read in full, and the refusal keeps its existing
// shape: read N but the forge reports M.
func TestForgeServesLessThanItAsserts(t *testing.T) {
	s := newStub()
	s.pr.ChangedFiles = 163
	s.files = paddingFiles(100)
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("truncating forge rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
	if m := s.mutated(); len(m) > 0 {
		t.Errorf("an unverifiable flip still mutated: %v", m)
	}
}

// The changed-file list comes from the PAGINATED changed-files endpoint, asking for the
// maximum page size — never from a single unpaginated field on the change document, which was
// the truncating source this walk replaced.
func TestChangedFilesReadPaginates(t *testing.T) {
	s := newStub()
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitOK {
		t.Fatalf("flip rc = %d, want 0", rc)
	}
	if s.fileReads == 0 {
		t.Fatal("the changed-files endpoint was never read")
	}
	// The paging parameters must be on the CHANGED-FILES call. Asserting they appear somewhere
	// in the log would be satisfied by the reviews read, which also paginates — the assertion
	// has to name the call it is about.
	var fileCalls int
	for _, r := range s.requests {
		if !strings.Contains(r.Path, "/files") {
			continue
		}
		fileCalls++
		if !strings.Contains(r.Query, "per_page=100") || !strings.Contains(r.Query, "page=") {
			t.Errorf("the changed-files read does not paginate: %s", r)
		}
	}
	if fileCalls == 0 {
		t.Error("no call to the changed-files endpoint was logged")
	}
}

// An unreadable changed-file list is could-not-check, never a clean classification, and the
// read is what must say so. Asserting only the exit code would not pin this: an empty list
// reaches the same fail-closed exit through the reconcile below, so a read that swallowed the
// error and returned nothing would look identical from outside.
func TestUnreadableChangedFilesRefuses(t *testing.T) {
	s := newStub()
	s.failPath = "/files"
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	fg := &deskkit.GitHubForge{Token: stubToken, BaseURL: s.srv.URL, Client: s.srv.Client()}
	fr := deskkit.ForgeRepo{Owner: "medici-finance", Name: "assay"}
	if _, err := readChangedFiles(flipOpts{pr: 7}, fg, fr); err == nil {
		t.Fatal("a failed changed-files read returned no error — could-not-check was rounded to an empty diff")
	} else if !strings.Contains(err.Error(), "changed files") {
		t.Errorf("the refusal does not name the read that failed: %v", err)
	}

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("unreadable changed files rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
	if m := s.mutated(); len(m) > 0 {
		t.Errorf("an unverifiable flip still mutated: %v", m)
	}
}

// TOCTOU: a head that moved during the checks means every verdict above was read against code
// that is no longer what would flip.
func TestHeadMovedDuringChecksRefuses(t *testing.T) {
	s := newStub()
	s.head2 = "9999999988888888777777776666666655555555"
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("moved head rc = %d, want %d", rc, deskkit.ExitRefused)
	}
	if s.flipped() {
		t.Fatal("the flip happened after the head moved")
	}
}

// An already-ready PR whose label is ALREADY correct is a pure no-op: exit 0 and not a single
// write. This is the common re-run case — a loop re-running its Land step over a landed item —
// and it must stay cheap and non-failing.
func TestAlreadyReadyWithCorrectLabelWritesNothing(t *testing.T) {
	s := newStub()
	s.pr.IsDraft = false
	s.pr.Labels = []string{labelAfterFlip}
	s.install(t)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitOK {
		t.Fatalf("already-ready, label correct: rc = %d, want 0 (idempotent no-op)", rc)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("a no-op re-run wrote: %v", m)
	}
	// It is also CHEAP: the no-op returns on the change read alone, without reading the
	// verdicts, the diff or the rollups.
	if s.saw(http.MethodGet, "/reviews") || s.saw(http.MethodGet, "/files") {
		t.Errorf("the no-op path read past the change document: %v", s.requests)
	}
}

// An already-ready PR whose label is STALE needs a WRITE, and the write asserts that the review
// lane is done — so it runs the same gate. When the conditions hold, the label is reconciled
// and the ready mutation is NOT re-issued.
func TestAlreadyReadyWithStaleLabelRelabelsAfterGating(t *testing.T) {
	s := newStub()
	s.pr.IsDraft = false // labels still carry the pre-flip label, from greenPR()
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitOK {
		t.Fatalf("already-ready, stale label, conditions hold: rc = %d, want 0", rc)
	}
	if s.flipped() {
		t.Error("the ready mutation was re-issued on a PR that is not a draft")
	}
	if !s.sawBody(http.MethodPost, "/labels", labelAfterFlip) {
		t.Error("the queue label was not reconciled")
	}
	// The re-gate really ran: the reviews were read.
	if !s.saw(http.MethodGet, "/reviews") {
		t.Error("the label was written without re-reading the verdicts — that is the ungated relabel")
	}
}

// THE FINDING. An already-ready PR whose label is stale and whose conditions DO NOT hold must
// not be relabelled. Writing `approval-needed` there tells every human reading the queue that
// the review lane is finished when it is not — and nobody re-checks a PR that claims to be
// done. The refusal names the condition and leaves the labels alone.
func TestAlreadyReadyWithStaleLabelRefusesWhenTheGateFails(t *testing.T) {
	s := newStub()
	s.pr.IsDraft = false
	s.install(t)
	s.reviews = nil // no approval at head: the gate must fail

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("already-ready, stale label, gate fails: rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("the label was written over a failed gate: %v — a queue that misreports who is blocked "+
			"is worse than one that says nothing", m)
	}
}

// A STABLE HEAD IS NOT A STABLE VERDICT.
//
// A `Security-Review: fail` is a retraction posted at the SAME head — no commit, so the head
// does not move. A head-only re-read therefore reports "still current" and the flip proceeds
// over a live withdrawal. This drives exactly that race: the first reviews read is clean, and a
// retraction lands before the mutation.
func TestSecurityRetractionPostedDuringTheChecksIsCaught(t *testing.T) {
	s := newStub()
	s.install(t)
	bot := reviewerBot(t)
	s.reviews = approvalAtHead(t, headSHA)

	// After the FIRST reviews read, a retraction at the same head appears.
	fail := reviewInfo{State: "COMMENTED", CommitID: headSHA, Body: "Security-Review: fail",
		SubmittedAt: "2026-01-01T00:05:00Z"}
	fail.User.Login = bot
	s.reviewsAfterFirstRead = append(append([]reviewInfo{}, s.reviews...), fail)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("retraction during the checks: rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("the PR was flipped over a retraction posted at the same head: %v — the head never moved, "+
			"so only a verdict re-read can see this", m)
	}
	// The re-read really happened: the reviews endpoint was hit more than once.
	if reads := s.count(http.MethodGet, "/reviews"); reads < 2 {
		t.Errorf("the reviews were read %d time(s) — the pre-mutation re-read is what closes this race", reads)
	}
}

// NEGATIVE CONTROL on the identity filter: an APPROVED at the current head from a login that is
// NOT the reviewer App must not satisfy the correctness gate. Human reviews do not govern this
// loop, and neither does any other bot: if the filter regressed to "somebody approved", every
// gate below it would be reachable by anyone who can press the button.
func TestNonReviewerApprovalAtHeadDoesNotSatisfyTheGate(t *testing.T) {
	for _, login := range []string{
		"a-human",             // a person
		"some-other-app[bot]", // a different App
		"app/some-other-app",  // the same other App in the CLI rendering
		"",                    // a review whose author the forge did not state
		"reviewer-typo",       // a near-miss on the reviewer's own name
	} {
		s := newStub()
		s.install(t)
		r := reviewInfo{State: "APPROVED", CommitID: headSHA, Body: "looks fine",
			SubmittedAt: "2026-01-01T00:00:00Z"}
		if login == "reviewer-typo" {
			login = reviewerBot(t) + "-typo"
		}
		r.User.Login = login
		s.reviews = []reviewInfo{r}

		if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
			t.Errorf("an APPROVED from %q gave rc = %d, want %d — only the reviewer App's verdict counts",
				login, rc, deskkit.ExitRefused)
		}
		if m := s.mutated(); len(m) != 0 {
			t.Errorf("an APPROVED from %q produced mutations: %v", login, m)
		}
	}
}

func TestClosedPRRefused(t *testing.T) {
	s := newStub()
	s.pr.State = "CLOSED"
	s.install(t)
	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("closed PR rc = %d, want %d", rc, deskkit.ExitRefused)
	}
}

// An unreadable PR is could-not-check. A PR whose state could not be read is not a PR that may
// be flipped.
func TestUnreadablePRIsUnverifiable(t *testing.T) {
	s := newStub()
	s.failPR = true
	s.install(t)
	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("unreadable PR rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
}

// An unreadable review list is could-not-check, and could-not-check is never an approval.
func TestUnreadableReviewsIsUnverifiable(t *testing.T) {
	s := newStub()
	s.failPath = "/reviews"
	s.install(t)
	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("unreadable reviews rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("mutations with an unreadable review list: %v", m)
	}
}

// --dry-run checks everything and stops before the mutation.
func TestDryRunChecksButNeverMutates(t *testing.T) {
	s := newStub()
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo, "--dry-run"}); rc != deskkit.ExitOK {
		t.Fatalf("dry-run rc = %d, want 0", rc)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("--dry-run mutated: %v", m)
	}
	if s.count(http.MethodGet, "/pulls/7") < 2 {
		t.Error("--dry-run skipped the head re-read — it must exercise every condition")
	}
}

// There is deliberately no override, no un-ready, and no merge verb: a gate a caller can wave
// past is not a gate.
func TestThereIsNoOverrideOrMergeVerb(t *testing.T) {
	// Read the USAGE block itself rather than the whole help text: the prose deliberately SAYS
	// "the merge is the human's", and a naive substring search over the prose would flag the
	// very sentence that states the boundary.
	for _, line := range strings.Split(usage, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "deskflip ") {
			continue
		}
		for _, forbidden := range []string{"--force", "--override", "--no-verify", "unready", "merge", "draft"} {
			if strings.Contains(trimmed, forbidden) {
				t.Errorf("the USAGE block advertises %q in %q — this verb has no override, no un-ready, "+
					"and no merge path", forbidden, trimmed)
			}
		}
	}
	s := newStub()
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	if rc := run([]string{"7", "--repo", privateCIRepo, "--force"}); rc != deskkit.ExitRefused {
		t.Fatalf("--force rc = %d, want %d (an unknown flag is refused)", rc, deskkit.ExitRefused)
	}
}

// A repo outside the desk set is not one this verb flips in.
func TestForeignRepoRefused(t *testing.T) {
	s := newStub()
	s.install(t)
	if rc := run([]string{"7", "--repo", "someone-else/thing"}); rc != deskkit.ExitRefused {
		t.Fatalf("foreign repo rc = %d, want %d", rc, deskkit.ExitRefused)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("mutations in a repo outside the desk set: %v", m)
	}
}

func TestConditionListIsTheDocumentedContract(t *testing.T) {
	want := []string{"caller-role", "app-token", "pr-open-draft", "model-floor", "reviewer-approved",
		"checks-green", "mergeable", "security-verdict", "head-stable"}
	if len(flipConditions) != len(want) {
		t.Fatalf("flipConditions has %d entries, want %d", len(flipConditions), len(want))
	}
	for i := range want {
		if flipConditions[i] != want[i] {
			t.Errorf("flipConditions[%d] = %q, want %q", i, flipConditions[i], want[i])
		}
		if !strings.Contains(usage, want[i]) {
			t.Errorf("condition %q is not documented in --help", want[i])
		}
	}
	// head-stable is LAST: its whole purpose is to be the final check before the mutation.
	if flipConditions[len(flipConditions)-1] != condHeadStable {
		t.Error("head-stable is not the last condition — the TOCTOU re-read must be the final one")
	}
}

func TestHelpNamesTheEngineSeam(t *testing.T) {
	if !strings.Contains(usage, "engine seam: LAND") {
		t.Error("deskflip --help does not name its engine seam")
	}
}

func TestVersionAndHelpAreUnguardedReads(t *testing.T) {
	s := newStub()
	s.install(t)
	if rc := run([]string{"--version"}); rc != deskkit.ExitOK {
		t.Fatalf("--version rc = %d, want 0", rc)
	}
	if rc := run([]string{"help"}); rc != deskkit.ExitOK {
		t.Fatalf("help rc = %d, want 0", rc)
	}
	if rc := run(nil); rc != deskkit.ExitRefused {
		t.Fatalf("no-args rc = %d, want %d", rc, deskkit.ExitRefused)
	}
}

func TestKillSwitchIsHonoured(t *testing.T) {
	s := newStub()
	s.install(t)
	t.Setenv("DESK_TOOLS_DISABLED", "1")
	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitDisabled {
		t.Fatalf("disabled rc = %d, want %d", rc, deskkit.ExitDisabled)
	}
	if len(s.requests) != 0 {
		t.Error("a disabled deskflip still read forge state")
	}
}

func TestEvalRollupTreatsAnUnknownConclusionAsNotGreen(t *testing.T) {
	if got := evalRollup([]rollupEntry{{Name: "x", Status: "COMPLETED", Conclusion: "ACTION_REQUIRED"}}); got != ciFail {
		t.Errorf("an unrecognised conclusion reduced to %v, want ciFail — an unverified check is never green", got)
	}
	if got := evalRollup(nil); got != ciEmpty {
		t.Errorf("an empty rollup reduced to %v, want ciEmpty", got)
	}
	if got := evalRollup([]rollupEntry{{Context: "legacy", State: "SUCCESS"}}); got != ciGreen {
		t.Errorf("a status-context rollup reduced to %v, want ciGreen — both forge shapes must be read", got)
	}
	if got := evalRollup([]rollupEntry{{Context: "legacy", State: "PENDING"}}); got != ciPending {
		t.Errorf("a pending status context reduced to %v, want ciPending", got)
	}
}
