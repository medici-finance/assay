package main

import (
	"encoding/json"
	"os/exec"
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
// check, and no risky paths.
func greenPR() prInfo {
	return prInfo{
		Number: 7, State: "OPEN", IsDraft: true, Mergeable: "MERGEABLE",
		HeadRefOid: headSHA, ChangedFiles: 1,
		Files:             []fileInfo{{Path: "README.md"}},
		StatusCheckRollup: []rollupEntry{{Name: "test", Status: "COMPLETED", Conclusion: "SUCCESS"}},
		Labels:            []labelInfo{{Name: labelBeforeFlip}},
	}
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
	pr.Files = []fileInfo{{Path: "README.md"}}
	s := &stub{pr: pr}
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("short files read rc = %d, want %d", rc, deskkit.ExitUnverifiable)
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
	want := []string{"caller-role", "pr-open-draft", "reviewer-approved", "checks-green",
		"mergeable", "security-verdict", "head-stable"}
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
