package main

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The reviewer App identity is READ FROM THE FIXTURE ROSTER, never spelled as a literal
// here: the verb reads it from the roster too, so a test carrying its own copy would keep
// passing after the binding changed.
func reviewerBot(t *testing.T) string {
	t.Helper()
	login, ok := deskkit.RoleAppLogin(reviewerRole)
	if !ok {
		t.Fatal("the fixture roster does not bind the reviewer role")
	}
	return login
}

type stub struct {
	calls   [][]string
	pr      prInfo
	reviews []reviewInfo
	// reviewsAfterFirstRead, when non-nil, is what EVERY read of the reviews endpoint
	// after the first returns. It models the race the pre-mutation re-read exists to
	// close: a verdict posted at the SAME head while the checks were running, which no
	// amount of head re-reading can see because the head never moves for it.
	reviewsAfterFirstRead []reviewInfo
	reviewReads           int
	head2                 string // what the TOCTOU re-read returns; "" means "unchanged"
	failPR                bool
	failGH                string // any argv containing this fragment exits non-zero

	// files is the COMPLETE changed-file list the fake forge will SERVE, independent of
	// what pr.ChangedFiles ASSERTS. Keeping the two independent is the point: the pair
	// (assert 163, serve 100) is a truncating forge, and the pair (assert 163, serve 163
	// across two pages) is the case the verb has to walk. nil means "the one-file diff
	// greenPR describes"; an explicitly empty slice serves nothing.
	files []string
	// filesPerPage is the fake forge's page size. Zero means the size the verb asks for,
	// so a 163-entry list lands as 100 + 63 without any test having to say so.
	filesPerPage int
	fileReads    int

	// labelEvents is the PR's `labeled` timeline — the dispatcher-attestation the model-
	// capability floor reads. nil serves an empty timeline, which the floor reads as
	// UNATTESTED (a NOTICE, not a refusal). Each event carries the applier login, so a
	// fixture can distinguish a dispatcher stamp from a self-applied one.
	labelEvents []deskkit.LabelEvent
	timelineErr bool // when true, the timeline read fails (could-not-check)
}

func (s *stub) install(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("DESK_SESSION", "deskflip-test")
	t.Setenv("CLAUDE_SESSION_ID", "deskflip-test")
	t.Setenv("DESK_LOOP", flipRole)

	// The App-token condition is exercised past its refusal here: every case below is
	// about a LATER condition, and a verb that could not authenticate would never reach
	// one. The refusal itself has its own tests (identity_test.go), which is what keeps
	// this stub from making the condition vacuous.
	oldMint := mintTokenFn
	mintTokenFn = func(role, repo string) (string, string, error) {
		return "stub-installation-token", filepath.Join(home, ".config", "assay", role+"-token-stub"), nil
	}
	oldTok := ghToken
	t.Cleanup(func() { mintTokenFn = oldMint; ghToken = oldTok })

	if s.files == nil {
		s.files = greenFiles()
	}

	old := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		joined := name + " " + strings.Join(args, " ")
		s.calls = append(s.calls, append([]string{name}, args...))
		switch {
		case s.failGH != "" && strings.Contains(joined, s.failGH):
			return exec.Command("/bin/sh", "-c", "echo stub-failure 1>&2; exit 1")
		case strings.Contains(joined, "-q .headRefOid"):
			h := s.head2
			if h == "" {
				h = s.pr.HeadRefOid
			}
			return echo(h)
		case strings.Contains(joined, "pr view") && strings.Contains(joined, "statusCheckRollup"):
			if s.failPR {
				return exec.Command("/bin/sh", "-c", "echo no such PR 1>&2; exit 1")
			}
			return echo(mustJSON(t, s.pr))
		case strings.Contains(joined, "issues/") && strings.Contains(joined, "/timeline"):
			if s.timelineErr {
				return exec.Command("/bin/sh", "-c", "echo timeline unreadable 1>&2; exit 1")
			}
			return echo(s.servedTimeline(t))
		case strings.Contains(joined, "pulls/") && strings.Contains(joined, "/files"):
			s.fileReads++
			return echo(s.servedFilePages(t))
		case strings.Contains(joined, "pulls/") && strings.Contains(joined, "/reviews"):
			s.reviewReads++
			if s.reviewReads > 1 && s.reviewsAfterFirstRead != nil {
				return echo(mustJSON(t, s.reviewsAfterFirstRead))
			}
			return echo(mustJSON(t, s.reviews))
		}
		return exec.Command("/bin/sh", "-c", "exit 0")
	}
	t.Cleanup(func() { execCommand = old })
	return root
}

// servedFilePages renders s.files the way `gh api --paginate` renders the changed-files
// endpoint: one top-level JSON array PER PAGE, concatenated. Reproducing the
// concatenation rather than one flat array is deliberate — a stub that returned a single
// array would let a reader that never joins pages pass, which is the very defect these
// tests exist to pin.
func (s *stub) servedFilePages(t *testing.T) string {
	t.Helper()
	size := s.filesPerPage
	if size == 0 {
		size = changedFilePerPage
	}
	var b strings.Builder
	for i := 0; i < len(s.files); i += size {
		end := i + size
		if end > len(s.files) {
			end = len(s.files)
		}
		page := make([]struct {
			Filename string `json:"filename"`
		}, 0, end-i)
		for _, f := range s.files[i:end] {
			page = append(page, struct {
				Filename string `json:"filename"`
			}{Filename: f})
		}
		b.WriteString(mustJSON(t, page))
		b.WriteString("\n")
	}
	if b.Len() == 0 {
		return "[]"
	}
	return b.String()
}

// servedTimeline renders s.labelEvents as the GitHub `labeled` timeline shape the floor
// reads: one JSON array of {event, label:{name}, actor:{login}} entries. A nil slice serves
// an empty array — a PR with no labels, which the floor reads as UNATTESTED.
func (s *stub) servedTimeline(t *testing.T) string {
	t.Helper()
	type tlEntry struct {
		Event string            `json:"event"`
		Label struct{ Name string } `json:"label"`
		Actor struct{ Login string } `json:"actor"`
	}
	out := make([]tlEntry, 0, len(s.labelEvents))
	for _, e := range s.labelEvents {
		var te tlEntry
		te.Event = "labeled"
		te.Label.Name = e.Name
		te.Actor.Login = e.AppliedBy
		out = append(out, te)
	}
	return mustJSON(t, out)
}

// dispatcherLogin is the roster's desk-App login — the ONLY applier whose dispatched-* stamp
// the floor trusts. Read from the fixture roster, never a literal, so a fixture stamp is
// applied by the same identity the verb vouches for.
func dispatcherLogin(t *testing.T) string {
	t.Helper()
	login, ok := deskkit.RoleAppLogin("desk")
	if !ok {
		t.Fatal("the fixture roster does not bind the desk (dispatcher) role")
	}
	return login
}

// strongStamp / cheapStamp build a complete dispatcher-applied tier attestation for a
// fixture PR. The applier is the roster dispatcher, so AttestedModelStampOf trusts it.
func strongStamp(t *testing.T) []deskkit.LabelEvent {
	d := dispatcherLogin(t)
	return []deskkit.LabelEvent{
		{Name: deskkit.DispatchedModelPrefix + "opus-4.8", AppliedBy: d},
		{Name: deskkit.DispatchedTierPrefix + "strong", AppliedBy: d},
	}
}

func cheapStamp(t *testing.T) []deskkit.LabelEvent {
	d := dispatcherLogin(t)
	return []deskkit.LabelEvent{
		{Name: deskkit.DispatchedModelPrefix + "haiku-3", AppliedBy: d},
		{Name: deskkit.DispatchedTierPrefix + "any", AppliedBy: d},
	}
}

func echo(s string) *exec.Cmd {
	return exec.Command("/bin/sh", "-c", "cat <<'STUBEOF'\n"+s+"\nSTUBEOF")
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func (s *stub) ran(fragment string) bool {
	for _, c := range s.calls {
		if strings.Contains(strings.Join(c, " "), fragment) {
			return true
		}
	}
	return false
}

// mutated reports whether ANY state-changing forge call was made. Every refusal test
// asserts on this: a gate that refuses AFTER mutating has not gated anything.
func (s *stub) mutated() []string {
	var out []string
	for _, c := range s.calls {
		j := strings.Join(c, " ")
		for _, verb := range []string{"pr ready", "pr edit", "pr merge", "pr close", "pr review"} {
			if strings.Contains(j, verb) {
				out = append(out, verb)
			}
		}
	}
	return out
}

const headSHA = "aaaaaaaabbbbbbbbccccccccdddddddd11111111"

// greenPR is a PR that satisfies every condition: open, draft, mergeable, one successful
// check, and no risky paths. Its ChangedFiles pairs with greenFiles — the envelope's
// asserted total and the list the forge serves are set in two places because the verb
// reads them from two endpoints, and their agreement is what a green run depends on.
func greenPR() prInfo {
	return prInfo{
		Number: 7, State: "OPEN", IsDraft: true, Mergeable: "MERGEABLE",
		HeadRefOid: headSHA, ChangedFiles: 1,
		StatusCheckRollup: []rollupEntry{{Name: "test", Status: "COMPLETED", Conclusion: "SUCCESS"}},
		Labels:            []labelInfo{{Name: labelBeforeFlip}},
	}
}

// greenFiles is the changed-file list greenPR's ChangedFiles asserts.
func greenFiles() []string { return []string{"README.md"} }

// riskyPath is a path in the compiled BASE trigger set, so it risk-classes a PR in any
// repo. It is read through the shared classifier in the tests below rather than asserted
// as a literal fact about the trigger list.
const riskyPath = ".github/workflows/ci.yml"

// paddingFiles builds n quiet paths — the docs-only bulk that a register PR is made of,
// and the reason such a PR crosses a page boundary at all.
func paddingFiles(n int) []string {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, "docs/streams/example/"+strconv.Itoa(i)+".md")
	}
	return out
}

func approvalAtHead(t *testing.T, head string) []reviewInfo {
	t.Helper()
	r := reviewInfo{State: "APPROVED", CommitID: head, Body: "looks correct", SubmittedAt: "2026-01-01T00:00:00Z"}
	r.User.Login = reviewerBot(t)
	return []reviewInfo{r}
}

// privateCIRepo and publicRepo are read from the roster so the tests exercise the same
// policy the verb reads, rather than an assumption about it.
const (
	privateCIRepo = "medici-finance/assay"
	publicRepo    = "example-org/example-k8s"
)

func TestFlipsWhenEveryConditionHolds(t *testing.T) {
	s := &stub{pr: greenPR()}
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitOK {
		t.Fatalf("flip rc = %d, want 0", rc)
	}
	if !s.ran("pr ready 7") {
		t.Error("the ready mutation never ran")
	}
	if !s.ran("--remove-label "+labelBeforeFlip) || !s.ran("--add-label "+labelAfterFlip) {
		t.Errorf("the queue labels were not swapped: %v", s.calls)
	}
	// The TOCTOU re-read must happen, and it must happen BEFORE the mutation.
	readIdx, flipIdx := -1, -1
	for i, c := range s.calls {
		j := strings.Join(c, " ")
		if readIdx < 0 && strings.Contains(j, "-q .headRefOid") {
			readIdx = i
		}
		if flipIdx < 0 && strings.Contains(j, "pr ready") {
			flipIdx = i
		}
	}
	if readIdx < 0 || flipIdx < 0 || readIdx > flipIdx {
		t.Errorf("head re-read at %d, flip at %d — the re-read is the LAST thing before the mutation",
			readIdx, flipIdx)
	}
}

// The flip belongs to the role that watched the review.
func TestWrongCallerRoleRefusesBeforeReadingAnything(t *testing.T) {
	s := &stub{pr: greenPR()}
	s.install(t)
	t.Setenv("DESK_LOOP", "worker-desk")

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("wrong role rc = %d, want %d", rc, deskkit.ExitRefused)
	}
	if len(s.calls) != 0 {
		t.Errorf("the role gate read forge state before refusing: %v", s.calls)
	}
}

func TestUnsetCallerRoleRefuses(t *testing.T) {
	s := &stub{pr: greenPR()}
	s.install(t)
	t.Setenv("DESK_LOOP", "")
	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("unset role rc = %d, want %d", rc, deskkit.ExitRefused)
	}
}

// An unrecognised loop name is could-not-check, not "some other role".
func TestUnknownCallerRoleIsUnverifiable(t *testing.T) {
	s := &stub{pr: greenPR()}
	s.install(t)
	t.Setenv("DESK_LOOP", "pr-reveiw-desk")
	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("unknown role rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
}

// An approval at a DIFFERENT head is STALE — a distinct answer from "no verdict", and the
// refusal must say so.
func TestStaleApprovalRefused(t *testing.T) {
	s := &stub{pr: greenPR()}
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
	s := &stub{pr: greenPR()}
	s.install(t)
	s.reviews = nil
	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("no verdict rc = %d, want %d", rc, deskkit.ExitRefused)
	}
}

// THE FORGERY GUARD. An APPROVED that immediately follows a CHANGES_REQUESTED at the SAME
// head verifies nothing new — there is no new code to verify — so it must not clear the
// standing block. The forge's self-approval block only keys on the PR AUTHOR account and
// has nothing to say about a third-party App re-posting a verdict at an unchanged head.
func TestApprovalOverAStandingBlockAtTheSameHeadIsRefused(t *testing.T) {
	s := &stub{pr: greenPR()}
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
	s := &stub{pr: greenPR()}
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
	pr := greenPR()
	pr.StatusCheckRollup = []rollupEntry{
		{Name: "test", Status: "COMPLETED", Conclusion: "SUCCESS"},
		{Name: "lint", Status: "COMPLETED", Conclusion: "FAILURE"},
	}
	s := &stub{pr: pr}
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
	pr := greenPR()
	pr.StatusCheckRollup = []rollupEntry{{Name: "test", Status: "IN_PROGRESS"}}
	s := &stub{pr: pr}
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("pending check rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
}

// An EMPTY rollup on a CI-required repo means the checks have not reported — not green.
func TestEmptyRollupOnCIRequiredRepoIsUnverifiable(t *testing.T) {
	pr := greenPR()
	pr.StatusCheckRollup = nil
	s := &stub{pr: pr}
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("empty rollup on a CI repo rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
}

// The two RFC3339 stamps every latest-run-per-name fixture below is built from. tOld sorts
// before tNew lexicographically, which is also chronologically — the property recencyKey
// relies on.
const (
	tOld = "2026-01-01T00:00:00Z"
	tNew = "2026-01-01T01:00:00Z"
)

// TestSupersededRunDoesNotJamACleanFlip is the fix's fail-first case (clause 9, direction a):
// a check NAME carrying a superseded older run PLUS a green LATEST run must not jam the flip.
// Both live jams observed on this public repo are reproduced as table rows — a CANCELLED
// predecessor (#289: a `changelog` run cancelled by the push + pull_request
// double-trigger) and a stale QUEUED orphan (#282: a `control-sweep` run the same
// double-trigger left queued). On the UNFIXED code each row refuses (the CANCELLED reddens
// the whole rollup; the QUEUED reads as forever-pending), so this test is red pre-fix.
func TestSupersededRunDoesNotJamACleanFlip(t *testing.T) {
	cases := map[string][]rollupEntry{
		// A cancelled predecessor + the re-triggered success, same NAME. The success is
		// newer (later CompletedAt), so it is the latest run and the flip is clean.
		"cancelled-predecessor": {
			{Name: "changelog", Status: "COMPLETED", Conclusion: "CANCELLED", StartedAt: tOld, CompletedAt: tOld},
			{Name: "changelog", Status: "COMPLETED", Conclusion: "SUCCESS", StartedAt: tNew, CompletedAt: tNew},
		},
		// An orphaned queued run carries NO timestamps, so recencyKey sorts it oldest and
		// the completed success is the latest run.
		"stale-queued-orphan": {
			{Name: "control-sweep", Status: "QUEUED"},
			{Name: "control-sweep", Status: "COMPLETED", Conclusion: "SUCCESS", StartedAt: tNew, CompletedAt: tNew},
		},
	}
	for name, rollup := range cases {
		t.Run(name, func(t *testing.T) {
			pr := greenPR()
			pr.StatusCheckRollup = rollup
			s := &stub{pr: pr}
			s.install(t)
			s.reviews = approvalAtHead(t, headSHA)

			if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitOK {
				t.Fatalf("superseded run jammed the flip: rc = %d, want 0 — only the LATEST run per name counts", rc)
			}
			if !s.ran("pr ready 7") {
				t.Errorf("the ready mutation never ran: %v", s.calls)
			}
		})
	}
}

// TestLatestRedRunStillRefuses is the fix's fail-first case (clause 9, direction b): the
// reduction must NEVER relax the gate. When the LATEST run of a name is red — an older
// SUCCESS superseded by a newer FAILURE — the flip must still refuse. On the correct code
// this passes; it is designed to REDDEN under a wrong reduction (keeping the older run
// instead of the newer), which mutations.json pins as a re-runnable mutation.
func TestLatestRedRunStillRefuses(t *testing.T) {
	pr := greenPR()
	pr.StatusCheckRollup = []rollupEntry{
		{Name: "build", Status: "COMPLETED", Conclusion: "SUCCESS", StartedAt: tOld, CompletedAt: tOld},
		{Name: "build", Status: "COMPLETED", Conclusion: "FAILURE", StartedAt: tNew, CompletedAt: tNew},
	}
	s := &stub{pr: pr}
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
	// The reduced set is what evalRollup then judges — and it evaluates it UNCHANGED. The
	// set here still carries the standalone nameless FAILURE, so the reduction has not
	// swallowed a real red: the judgement stays ciFail.
	if st := evalRollup(got); st != ciFail {
		t.Errorf("reduced set evaluated to %v, want ciFail — the standalone nameless FAILURE must survive", st)
	}
}

func TestConflictingPRRefused(t *testing.T) {
	pr := greenPR()
	pr.Mergeable = "CONFLICTING"
	s := &stub{pr: pr}
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
	pr := greenPR()
	pr.Mergeable = "UNKNOWN"
	s := &stub{pr: pr}
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("unknown mergeability rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
}

// A PUBLIC repo is risk-classed unconditionally — no diff reading required, and no quiet
// path out of the security gate.
func TestPublicRepoNeedsASecurityPassAtHead(t *testing.T) {
	s := &stub{pr: greenPR()}
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
	s := &stub{pr: greenPR()}
	s.install(t)
	bot := reviewerBot(t)
	sec := reviewInfo{State: "COMMENTED", CommitID: headSHA, Body: "Security-Review: pass", SubmittedAt: "2026-01-01T00:01:00Z"}
	sec.User.Login = bot
	s.reviews = append(approvalAtHead(t, headSHA), sec)

	if rc := run([]string{"7", "--repo", publicRepo}); rc != deskkit.ExitOK {
		t.Fatalf("public repo with both artifacts rc = %d, want 0", rc)
	}
	if !s.ran("pr ready 7") {
		t.Error("the flip did not happen with both artifacts at head")
	}
}

// A `Security-Review: fail` at head is a deliberate RETRACTION. It blocks EVERY flip,
// risk-classed or not — whether the diff happens to trigger risk classification has
// nothing to do with whether the retraction was made.
func TestSecurityFailAtHeadBlocksEvenOnANonRiskClassedPR(t *testing.T) {
	s := &stub{pr: greenPR()}
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
	s := &stub{pr: greenPR()}
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

// A short files read makes the risk-class determination unverifiable, not clean: pad a PR
// with enough files ahead of the risky one and a gate that trusted a truncated walk would
// waive itself.
func TestShortFilesReadIsUnverifiable(t *testing.T) {
	pr := greenPR()
	pr.ChangedFiles = 300
	s := &stub{pr: pr, files: greenFiles()}
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("short files read rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
}

// The regression this file exists for. A 163-file PR is served across two pages, and the
// gate has to reach its risk-class determination on all 163 — not refuse at the page
// boundary. Before the paginated read the changed-file list came from `gh pr view --json
// files`, one unpaginated page, so this flip refused as could-not-verify on a diff the
// forge was perfectly willing to serve.
func TestWalksEveryPageOfTheDiff(t *testing.T) {
	const total = 163
	pr := greenPR()
	pr.ChangedFiles = total
	s := &stub{pr: pr, files: paddingFiles(total)}
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitOK {
		t.Fatalf("163-file risk-clear PR rc = %d, want 0 — the walk did not read past page one", rc)
	}
	if !s.ran("pr ready 7") {
		t.Error("the ready mutation never ran")
	}
	// The list really did span pages: one page of changedFilePerPage cannot hold 163.
	if total <= changedFilePerPage {
		t.Fatalf("fixture no longer crosses a page boundary (%d <= %d)", total, changedFilePerPage)
	}
}

// The other half, and the one that matters more: a risky path sitting PAST the page
// boundary must still risk-class the PR. A reader that stops at page one sees only quiet
// docs paths — so with the reconcile removed it would have called this diff clean and
// flipped a workflow change on a correctness review alone.
func TestRiskPathPastPageOneIsSeen(t *testing.T) {
	const total = 163
	files := append(paddingFiles(total-1), riskyPath)
	if !deskkit.RiskPathTriggered(privateCIRepo, []string{riskyPath}) {
		t.Fatalf("%s is no longer a risk-classed path — this test's premise is gone", riskyPath)
	}
	if len(files) <= changedFilePerPage {
		t.Fatalf("the risky path is not past the page boundary (%d <= %d)", len(files), changedFilePerPage)
	}
	pr := greenPR()
	pr.ChangedFiles = total
	s := &stub{pr: pr, files: files}
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

// Walking the pages does NOT relax the fail-closed branch. A forge that asserts 163 and
// serves 100 with no second page is a diff nobody read in full, and the refusal keeps its
// existing shape: read N but the forge reports M.
func TestForgeServesLessThanItAsserts(t *testing.T) {
	pr := greenPR()
	pr.ChangedFiles = 163
	s := &stub{pr: pr, files: paddingFiles(100)}
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("truncating forge rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
	if m := s.mutated(); len(m) > 0 {
		t.Errorf("an unverifiable flip still mutated: %v", m)
	}
}

// The changed-file list comes from the PAGINATED endpoint, not from a `gh pr view` field.
// This pins the read itself: `pr view --json ... files` was the truncating source, so
// asking for it again would silently reintroduce the defect even with the walk in place.
func TestChangedFilesReadPaginates(t *testing.T) {
	s := &stub{pr: greenPR()}
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitOK {
		t.Fatalf("flip rc = %d, want 0", rc)
	}
	if s.fileReads == 0 {
		t.Fatal("the changed-files endpoint was never read")
	}
	// `--paginate` must be on the CHANGED-FILES call. Asserting it appears somewhere in
	// the call log would be satisfied by the reviews read, which also paginates — the
	// assertion has to name the call it is about.
	var fileCalls int
	for _, c := range s.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, "pr view") && strings.Contains(j, "files") {
			t.Errorf("the truncating `pr view --json files` read is back: %s", j)
		}
		if !strings.Contains(j, "/files") {
			continue
		}
		fileCalls++
		if !strings.Contains(j, "--paginate") {
			t.Errorf("the changed-files read does not paginate: %s", j)
		}
	}
	if fileCalls == 0 {
		t.Error("no call to the changed-files endpoint was logged")
	}
}

// An unreadable changed-file list is could-not-check, never a clean classification, and
// the read is what must say so. Asserting only the exit code would not pin this: an empty
// list reaches the same fail-closed exit through the reconcile below, so a read that
// swallowed the error and returned nothing would look identical from outside.
func TestUnreadableChangedFilesRefuses(t *testing.T) {
	s := &stub{pr: greenPR(), failGH: "/files"}
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if _, err := readChangedFiles(flipOpts{pr: 7}, privateCIRepo); err == nil {
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

// TOCTOU: a head that moved during the checks means every verdict above was read against
// code that is no longer what would flip.
func TestHeadMovedDuringChecksRefuses(t *testing.T) {
	s := &stub{pr: greenPR(), head2: "9999999988888888777777776666666655555555"}
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("moved head rc = %d, want %d", rc, deskkit.ExitRefused)
	}
	if s.ran("pr ready") {
		t.Fatal("the flip happened after the head moved")
	}
}

// An ALREADY-ready PR is the idempotent no-op: a loop re-running its Land step must not
// turn a completed flip into a failure. The label swap is still reconciled.
// An already-ready PR whose label is ALREADY correct is a pure no-op: exit 0 and not a
// single write. This is the common re-run case — a loop re-running its Land step over a
// landed item — and it must stay cheap and non-failing.
func TestAlreadyReadyWithCorrectLabelIsAPureNoOp(t *testing.T) {
	pr := greenPR()
	pr.IsDraft = false
	pr.Labels = []labelInfo{{Name: labelAfterFlip}}
	s := &stub{pr: pr}
	s.install(t)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitOK {
		t.Fatalf("already-ready, label correct: rc = %d, want 0 (idempotent no-op)", rc)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("a no-op re-run wrote: %v", m)
	}
}

// An already-ready PR whose label is STALE needs a WRITE, and the write asserts that the
// review lane is done — so it runs the same gate. When the conditions hold, the label is
// reconciled and the ready mutation is NOT re-issued.
func TestAlreadyReadyWithStaleLabelRelabelsAfterAFullReGate(t *testing.T) {
	pr := greenPR()
	pr.IsDraft = false // labels still carry the pre-flip label, from greenPR()
	s := &stub{pr: pr}
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitOK {
		t.Fatalf("already-ready, stale label, conditions hold: rc = %d, want 0", rc)
	}
	if s.ran("pr ready") {
		t.Error("the ready mutation was re-issued on a PR that is not a draft")
	}
	if !s.ran("--add-label " + labelAfterFlip) {
		t.Error("the queue label was not reconciled")
	}
	// The re-gate really ran: the reviews were read.
	if !s.ran("/reviews") {
		t.Error("the label was written without re-reading the verdicts — that is the ungated relabel")
	}
}

// THE FINDING. An already-ready PR whose label is stale and whose conditions DO NOT hold
// must not be relabelled. Writing `approval-needed` there tells every human reading the
// queue that the review lane is finished when it is not — and nobody re-checks a PR that
// claims to be done. The refusal names the condition and leaves the labels alone.
func TestAlreadyReadyWithStaleLabelRefusesWhenTheGateFails(t *testing.T) {
	pr := greenPR()
	pr.IsDraft = false
	s := &stub{pr: pr}
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
// A `Security-Review: fail` is a retraction posted at the SAME head — no commit, so the
// head does not move. A head-only re-read therefore reports "still current" and the flip
// proceeds over a live withdrawal. This drives exactly that race: the first reviews read
// is clean, and a retraction lands before the mutation.
func TestSecurityRetractionPostedDuringTheChecksIsCaught(t *testing.T) {
	s := &stub{pr: greenPR()}
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
	reads := 0
	for _, c := range s.calls {
		if strings.Contains(strings.Join(c, " "), "/reviews") {
			reads++
		}
	}
	if reads < 2 {
		t.Errorf("the reviews were read %d time(s) — the pre-mutation re-read is what closes this race", reads)
	}
}

// NEGATIVE CONTROL on the identity filter: an APPROVED at the current head from a login
// that is NOT the reviewer App must not satisfy the correctness gate. Human reviews do not
// govern this loop, and neither does any other bot: if the filter regressed to "somebody
// approved", every gate below it would be reachable by anyone who can press the button.
func TestNonReviewerApprovalAtHeadDoesNotSatisfyTheGate(t *testing.T) {
	s := &stub{pr: greenPR()}
	s.install(t)
	for _, login := range []string{
		"a-human",                // a person
		"some-other-app[bot]",    // a different App
		"app/some-other-app",     // the same other App in the CLI rendering
		"",                       // a review whose author the forge did not state
		reviewerBot(t) + "-typo", // a near-miss on the reviewer's own name
	} {
		r := reviewInfo{State: "APPROVED", CommitID: headSHA, Body: "looks fine",
			SubmittedAt: "2026-01-01T00:00:00Z"}
		r.User.Login = login
		s.calls = nil
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
	pr := greenPR()
	pr.State = "CLOSED"
	s := &stub{pr: pr}
	s.install(t)
	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("closed PR rc = %d, want %d", rc, deskkit.ExitRefused)
	}
}

// An unreadable PR is could-not-check. A PR whose state could not be read is not a PR that
// may be flipped.
func TestUnreadablePRIsUnverifiable(t *testing.T) {
	s := &stub{pr: greenPR(), failPR: true}
	s.install(t)
	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("unreadable PR rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
}

// An unreadable review list is could-not-check, and could-not-check is never an approval.
func TestUnreadableReviewsIsUnverifiable(t *testing.T) {
	s := &stub{pr: greenPR(), failGH: "/reviews"}
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
	s := &stub{pr: greenPR()}
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo, "--dry-run"}); rc != deskkit.ExitOK {
		t.Fatalf("dry-run rc = %d, want 0", rc)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("--dry-run mutated: %v", m)
	}
	if !s.ran("-q .headRefOid") {
		t.Error("--dry-run skipped the head re-read — it must exercise every condition")
	}
}

// There is deliberately no override, no un-ready, and no merge verb: a gate a caller can
// wave past is not a gate.
func TestThereIsNoOverrideOrMergeVerb(t *testing.T) {
	// Read the USAGE block itself rather than the whole help text: the prose deliberately
	// SAYS "the merge is the human's", and a naive substring search over the prose would
	// flag the very sentence that states the boundary.
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
	s := &stub{pr: greenPR()}
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	if rc := run([]string{"7", "--repo", privateCIRepo, "--force"}); rc != deskkit.ExitRefused {
		t.Fatalf("--force rc = %d, want %d (an unknown flag is refused)", rc, deskkit.ExitRefused)
	}
}

// A repo outside the desk set is not one this verb flips in.
func TestForeignRepoRefused(t *testing.T) {
	s := &stub{pr: greenPR()}
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
	s := &stub{pr: greenPR()}
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
	s := &stub{pr: greenPR()}
	s.install(t)
	t.Setenv("DESK_TOOLS_DISABLED", "1")
	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitDisabled {
		t.Fatalf("disabled rc = %d, want %d", rc, deskkit.ExitDisabled)
	}
	if len(s.calls) != 0 {
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

func TestSplitJSONArraysHandlesConcatenatedPages(t *testing.T) {
	got := splitJSONArrays(`[{"a":"]["}][{"b":2}]`)
	if len(got) != 2 {
		t.Fatalf("split %d pages, want 2 (a bracket inside a STRING is not a page boundary): %v", len(got), got)
	}
}
