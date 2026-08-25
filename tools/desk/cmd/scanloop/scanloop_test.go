package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

func readSource(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("cannot read %s: %v", name, err)
	}
	return string(b)
}

func writeTemp(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

// ---------------------------------------------------------------------------
// CLI contract
// ---------------------------------------------------------------------------

// TestCLI_NoArgsRefuses_HelpSucceeds — the desk-tools convention: a bare invocation is a refusal
// (nothing was asked for), an explicit help request is a success.
func TestCLI_NoArgsRefuses_HelpSucceeds(t *testing.T) {
	if got := run(nil); got != deskkit.ExitRefused {
		t.Fatalf("no args = %d, want %d", got, deskkit.ExitRefused)
	}
	for _, flag := range []string{"-h", "--help", "help"} {
		if got := run([]string{flag}); got != deskkit.ExitOK {
			t.Fatalf("%s = %d, want %d", flag, got, deskkit.ExitOK)
		}
	}
}

func TestCLI_VersionSucceeds(t *testing.T) {
	if got := run([]string{"--version"}); got != deskkit.ExitOK {
		t.Fatalf("--version = %d", got)
	}
}

func TestCLI_UnknownSubcommandRefuses(t *testing.T) {
	if got := run([]string{"drain-everything"}); got != deskkit.ExitRefused {
		t.Fatalf("unknown subcommand = %d, want %d", got, deskkit.ExitRefused)
	}
}

// TestUsage_StatesTheFiveExitsAndTheJudgmentBoundary — --help is where an operator learns what this
// binary will and will not decide. Both are load-bearing.
func TestUsage_StatesTheFiveExitsAndTheJudgmentBoundary(t *testing.T) {
	for _, want := range append(exitNames(),
		"EMITTED for", "REGENERATED", "BEFORE anything is queued", "DISABLED > STOP > STOP."+LoopName) {
		if !strings.Contains(usage, want) {
			t.Fatalf("--help does not state %q", want)
		}
	}
}

// TestLoopName_IsAKnownStopFlagIdentity — an unrecognised DESK_LOOP is could-not-check inside the
// guard, so a drain presenting a name the roster does not know cannot be stopped by name.
func TestLoopName_IsAKnownStopFlagIdentity(t *testing.T) {
	if !deskkit.IsKnownLoopName(LoopName) {
		t.Fatalf("%q is not a registered loop name — its STOP.%s flag would halt nothing", LoopName, LoopName)
	}
}

// ---------------------------------------------------------------------------
// tier policy — the mechanical/judgment split
// ---------------------------------------------------------------------------

func TestTierPolicy_MechanicalIsLocal_JudgmentIsEmitted(t *testing.T) {
	s := &ScanLoop{Root: t.TempDir()}
	mech := loopengine.Item{ID: "a#1", Payload: map[string]string{"lane": string(LaneScanCarrierPR)}}
	if got, _ := s.TierPolicy(mech); got != loopengine.TierLocal {
		t.Fatalf("mechanical tier = %s, want local", got)
	}
	judgment := loopengine.Item{ID: "a#2", Payload: map[string]string{"lane": string(LaneRouting)}}
	if got, _ := s.TierPolicy(judgment); got != loopengine.TierSession {
		t.Fatalf("judgment tier = %s, want session", got)
	}
}

// TestTierPolicy_NeverRoutesToAHumanTier — this desk's human gate is downstream (the review of the
// PRs it opens, the decision queue it files into), not a lane inside the drain.
func TestTierPolicy_NeverRoutesToAHumanTier(t *testing.T) {
	s := &ScanLoop{Root: t.TempDir()}
	for _, lane := range []LaneName{LaneScanCarrierPR, LaneIssueFiling, LaneRouting, "unknown"} {
		it := loopengine.Item{ID: "a#1", Payload: map[string]string{"lane": string(lane)}}
		got, err := s.TierPolicy(it)
		if err != nil {
			t.Fatalf("lane %s: %v", lane, err)
		}
		if got == loopengine.TierHuman {
			t.Fatalf("lane %s routed to the human tier", lane)
		}
	}
	if got := tierNames(s.reachableTiers()); len(got) != 2 {
		t.Fatalf("reachable tiers = %v, want exactly the two this policy can emit", got)
	}
}

// TestClassify_UnreadableLocalStateRoutesToJudgment — the bounded direction. A mechanical lane must
// never act on state it could not read.
func TestClassify_UnreadableLocalStateRoutesToJudgment(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, deskkit.ScanDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Skip("cannot make a directory unreadable in this environment")
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	s := &ScanLoop{Root: root, ScanTarget: "medici-finance/assay"}
	lane, kind := s.classify(Admission{Item: inbound("example-org/tracker", 1), State: AdmissionAdmitted})
	if lane != LaneRouting || kind != "unreadable-placeholder-state" {
		t.Fatalf("lane = %s (%s), want the judgment lane when local state is unreadable", lane, kind)
	}
}

// TestClassify_ReadScopeOnlyRepoJoinsTheBatch — an issue on a repo that is in the intake READ scope
// but OUTSIDE the write boundary is ordinary work, not a dead end: the scan is whole-scope and its
// placeholder lands in the SCAN TARGET under a repo-stemmed name. Treating the inbound repo as the
// write destination is what made such an item refuse inside the lane on every pass forever.
func TestClassify_ReadScopeOnlyRepoJoinsTheBatch(t *testing.T) {
	readOnly := readScopeOnlyRepo(t)
	s := &ScanLoop{Root: t.TempDir(), ScanTarget: "medici-finance/assay"}
	lane, kind := s.classify(Admission{Item: inbound(readOnly, 4), State: AdmissionAdmitted})
	if lane != LaneScanCarrierPR || kind != "new-issue" {
		t.Fatalf("lane = %s (%s) for a read-scope-only repo, want the mechanical batch — its placeholder "+
			"lands in the writable scan target", lane, kind)
	}
}

// TestClassify_UnwritableScanTargetRoutesToJudgmentAtClassificationTime — the residual case, and it
// is decided at CLASSIFICATION time. Deciding it inside the lane makes every pass produce a dispatch
// error and a false leak flag for a condition retrying can never change.
func TestClassify_UnwritableScanTargetRoutesToJudgmentAtClassificationTime(t *testing.T) {
	unwritable := readScopeOnlyRepo(t)
	s := &ScanLoop{Root: t.TempDir(), ScanTarget: unwritable}
	lane, kind := s.classify(Admission{Item: inbound("medici-finance/assay", 5), State: AdmissionAdmitted})
	if lane != LaneRouting || kind != "scan-target-outside-write-boundary" {
		t.Fatalf("lane = %s (%s), want the judgment lane when the scan target cannot be written", lane, kind)
	}
	// And with no target resolved at all, likewise — never a mechanical lane pointed at nowhere.
	s = &ScanLoop{Root: t.TempDir()}
	if lane, kind := s.classify(Admission{Item: inbound("medici-finance/assay", 6), State: AdmissionAdmitted}); lane != LaneRouting || kind != "no-scan-target" {
		t.Fatalf("lane = %s (%s) with no scan target", lane, kind)
	}
}

// readScopeOnlyRepo finds a repo the fixture roster puts in the intake SCAN scope but NOT in the
// write boundary — the exact shape the two tests above are about. It is derived from the roster
// rather than hard-coded so a roster edit cannot leave the tests asserting a shape that no longer
// exists.
func readScopeOnlyRepo(t *testing.T) string {
	t.Helper()
	for _, r := range deskkit.ScanRepos() {
		if !deskkit.IsAllowedRepo(r) {
			return r
		}
	}
	t.Skip("the fixture roster has no repo in the scan scope that is outside the write boundary")
	return ""
}

func TestHasPlaceholder_BareAndRepoStemmedForms(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, deskkit.ScanDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"issue-7.md", "tracker-issue-9.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("---\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		repo string
		num  int
		want bool
	}{
		{"medici-finance/assay", 7, true},   // the bare form
		{"example-org/tracker", 9, true},    // this repo's own stem
		{"example-org/agents", 9, false},    // another repo's number 9 must NOT match
		{"medici-finance/assay", 11, false}, // no placeholder at all
	}
	for _, c := range cases {
		got, err := HasPlaceholder(root, c.repo, c.num)
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Fatalf("HasPlaceholder(%s, %d) = %v, want %v", c.repo, c.num, got, c.want)
		}
	}
}

// TestHasPlaceholder_MissingScanDirIsNotAnError — a checkout that has never been scanned genuinely
// has no placeholders.
func TestHasPlaceholder_MissingScanDirIsNotAnError(t *testing.T) {
	got, err := HasPlaceholder(t.TempDir(), "example-org/tracker", 1)
	if err != nil || got {
		t.Fatalf("HasPlaceholder = %v, %v", got, err)
	}
}

// ---------------------------------------------------------------------------
// SelectQueue — the gate runs at the QUEUEING boundary
// ---------------------------------------------------------------------------

func TestSelectQueue_OnlyAdmittedItemsAreQueued(t *testing.T) {
	s := &ScanLoop{
		Root: t.TempDir(),
		Monitor: func() (*MonitorReport, error) {
			return ParseMonitorOutput(
				"INBOUND: example-org/tracker#1 2026-08-24T09:00:00Z\n" +
					"INBOUND: example-org/tracker#2 2026-08-24T09:01:00Z\n"), nil
		},
		Probe: func(_ string, n int) (string, time.Time, []deskkit.ContentEvent, bool, error) {
			if n == 1 {
				return "ada", time.Time{}, nil, true, nil
			}
			return "outsider", time.Time{}, nil, true, nil
		},
	}
	items, err := s.SelectQueue()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "example-org/tracker#1" {
		t.Fatalf("queue = %v, want only the admitted item", items)
	}
	// The quarantined item is still VISIBLE in the gate pass — listed and counted, never routed.
	if got := len(s.Admissions()); got != 2 {
		t.Fatalf("admissions = %d, want both items visible", got)
	}
	_, q, _ := AdmissionCounts(s.Admissions())
	if q != 1 {
		t.Fatalf("quarantined = %d, want 1", q)
	}
}

// TestSelectQueue_NoInboundSurfaceIsUnverifiable — an unread surface is COULD-NOT-CHECK, never an
// empty queue.
func TestSelectQueue_NoInboundSurfaceIsUnverifiable(t *testing.T) {
	s := &ScanLoop{Root: t.TempDir()}
	_, err := s.SelectQueue()
	if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("err = %v, want unverifiable", err)
	}
}

// TestSelectQueue_ItemIDsCarryTheRepo — two repos routinely own issue numbers of the same value,
// and the claim key is derived from the item ID.
func TestSelectQueue_ItemIDsCarryTheRepo(t *testing.T) {
	if a, b := inbound("example-org/tracker", 5).ID(), inbound("example-org/agents", 5).ID(); a == b {
		t.Fatalf("two repos' issue #5 share the claim key %q — they would over-lock each other", a)
	}
}

// ---------------------------------------------------------------------------
// Land
// ---------------------------------------------------------------------------

func TestLand_RecordsExactlyOneExitPerItem(t *testing.T) {
	s := &ScanLoop{Root: t.TempDir(), DryRun: true, Emit: io_Discard{}}
	s.outcomes = map[string]LaneOutcome{
		"a#1": {Lane: LaneScanCarrierPR, Exit: ExitPlaceholder, Artifact: "pr"},
	}
	if err := s.Land(loopengine.Result{Item: loopengine.Item{ID: "a#1"}, Verdict: loopengine.VerdictPass}); err != nil {
		t.Fatal(err)
	}
	recs := s.Ledger().Records()
	if len(recs) != 1 || recs[0].Exit != ExitPlaceholder {
		t.Fatalf("ledger = %v", recs)
	}
}

// TestLand_RefusesAnItemThatLandsWithNoExit — the leak, caught at the only place it can be.
func TestLand_RefusesAnItemThatLandsWithNoExit(t *testing.T) {
	s := &ScanLoop{Root: t.TempDir(), DryRun: true, Emit: io_Discard{}}
	err := s.Land(loopengine.Result{Item: loopengine.Item{ID: "a#1"}, Verdict: loopengine.VerdictPass})
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("err = %v, want a refusal", err)
	}
}

// TestDispatch_JudgmentWithNoFeederRefuses — with no result path for the routing decision the drain
// refuses rather than inventing an exit.
func TestDispatch_JudgmentWithNoFeederRefuses(t *testing.T) {
	s := &ScanLoop{Root: t.TempDir(), DryRun: true, Emit: io_Discard{}}
	it := loopengine.Item{ID: "a#1", Payload: map[string]string{"repo": "medici-finance/assay", "lane": string(LaneRouting)}}
	_, err := s.Dispatch(it, loopengine.TierSession)
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("err = %v, want a refusal", err)
	}
}

// ---------------------------------------------------------------------------
// plan
// ---------------------------------------------------------------------------

const planPoll = `INBOUND: example-org/tracker#101 2026-08-24T10:00:00Z
`

func armedStateDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, slug := range deskkit.ScanRepos() {
		if err := os.WriteFile(filepath.Join(dir, stateFileName(slug)), []byte("x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestPlan_PrintsTheQueueAndItsBounds(t *testing.T) {
	var sb strings.Builder
	err := cmdPlan([]string{
		"--root", t.TempDir(),
		"--scan-target", "medici-finance/assay",
		"--state-dir", armedStateDir(t),
		"--inbound", writeTemp(t, "poll.txt", planPoll),
		"--now", "2026-08-24T12:00:00Z",
	}, &sb)
	if err != nil {
		t.Fatalf("plan errored: %v\n%s", err, sb.String())
	}
	out := sb.String()
	for _, want := range []string{
		"scan scope:",
		"MONITOR:",
		"TRUST GATE (applied BEFORE queueing)",
		"QUEUE —",
		"example-org/tracker#101",
		"COALESCE (window",
		"REGENERATED on every push",
		"INBOUND SURFACES THIS DRAIN DOES NOT READ",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("plan output does not carry %q:\n%s", want, out)
		}
	}
}

// TestPlan_UnarmedMonitorIsUnverifiable — nothing being watched is COULD-NOT-CHECK, not a quiet
// queue. rc=0 here would be read as "no inbound".
func TestPlan_UnarmedMonitorIsUnverifiable(t *testing.T) {
	err := cmdPlan([]string{
		"--root", t.TempDir(),
		"--scan-target", "medici-finance/assay",
		"--state-dir", filepath.Join(t.TempDir(), "never-armed"),
		"--now", "2026-08-24T12:00:00Z",
	}, &strings.Builder{})
	if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("err = %v, want unverifiable", err)
	}
}

// TestPlan_BlindPassIsUnverifiable — a degraded repo or a suppressed burst means the pass could not
// see the whole surface.
func TestPlan_BlindPassIsUnverifiable(t *testing.T) {
	err := cmdPlan([]string{
		"--root", t.TempDir(),
		"--scan-target", "medici-finance/assay",
		"--state-dir", armedStateDir(t),
		"--inbound", writeTemp(t, "poll.txt", busyPoll),
		"--now", "2026-08-24T12:00:00Z",
	}, &strings.Builder{})
	if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("err = %v, want unverifiable for a partially-read surface", err)
	}
}

// TestPlan_BadFlagsRefuse
func TestPlan_BadFlagsRefuse(t *testing.T) {
	if got := deskkit.ExitCodeOf(cmdPlan([]string{"--nope"}, &strings.Builder{})); got != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d", got, deskkit.ExitRefused)
	}
	if got := deskkit.ExitCodeOf(cmdPlan([]string{"--now", "yesterday"}, &strings.Builder{})); got != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d", got, deskkit.ExitRefused)
	}
}

// TestPlan_DoesNotRunThePoller — a plan that polled would ADVANCE the per-repo baselines and
// swallow the events it was asked to report. The state dir must be byte-identical afterwards.
func TestPlan_DoesNotRunThePoller(t *testing.T) {
	dir := armedStateDir(t)
	before, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	stamps := map[string]time.Time{}
	for _, e := range before {
		info, _ := e.Info()
		stamps[e.Name()] = info.ModTime()
	}
	_ = cmdPlan([]string{
		"--root", t.TempDir(),
		"--scan-target", "medici-finance/assay",
		"--state-dir", dir,
		"--inbound", writeTemp(t, "poll.txt", planPoll),
		"--now", "2026-08-24T12:00:00Z",
	}, &strings.Builder{})
	after, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("the state dir changed size: %d -> %d", len(before), len(after))
	}
	for _, e := range after {
		info, _ := e.Info()
		if !info.ModTime().Equal(stamps[e.Name()]) {
			t.Fatalf("plan advanced the baseline for %s — it consumed the events it was asked to report", e.Name())
		}
	}
}

// TestRun_OfflineWithNoCapturedPollRefuses — an offline drain with no events in hand is BLIND, and
// blind is not idle.
func TestRun_OfflineWithNoCapturedPollRefuses(t *testing.T) {
	err := cmdRun([]string{
		"--root", t.TempDir(),
		"--scan-target", "medici-finance/assay",
		"--worktree-base", t.TempDir(),
		"--state-dir", armedStateDir(t),
		"--offline",
		"--now", "2026-08-24T12:00:00Z",
	}, &strings.Builder{})
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("err = %v, want a refusal", err)
	}
}

// TestRun_RelativeWorktreeBaseRefuses — isolation depends on an absolute path.
func TestRun_RelativeWorktreeBaseRefuses(t *testing.T) {
	err := cmdRun([]string{
		"--root", t.TempDir(),
		"--scan-target", "medici-finance/assay",
		"--worktree-base", "scan-worktrees",
		"--offline",
		"--inbound", writeTemp(t, "poll.txt", planPoll),
	}, &strings.Builder{})
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("err = %v, want a refusal", err)
	}
}

// TestWorktreeFor_IsOutsideTheTargetCheckout — the lane's guard is the backstop; the default path
// must not need it.
func TestWorktreeFor_IsOutsideTheTargetCheckout(t *testing.T) {
	root := filepath.Join(t.TempDir(), "target")
	s := &ScanLoop{Root: root}
	wt := s.worktreeFor(loopengine.Item{ID: "example-org/tracker#5"})
	if !filepath.IsAbs(wt) {
		t.Fatalf("worktree path %q is not absolute", wt)
	}
	if strings.HasPrefix(wt, root+string(filepath.Separator)) {
		t.Fatalf("the default scan worktree %q is nested inside the target checkout %q", wt, root)
	}
	if strings.ContainsAny(filepath.Base(wt), "/#") {
		t.Fatalf("the worktree segment %q is not a single path component", filepath.Base(wt))
	}
}

// TestBranchFor_IsPerPassAndSecondGranular — the collision this pins is the one that made a second
// item in a pass fail `worktree add -b` and then read as a leak. The branch is now computed ONCE per
// pass, is second-granular, and carries a random suffix, so neither two items in a pass nor two
// passes in a second can produce the same name.
func TestBranchFor_IsPerPassAndSecondGranular(t *testing.T) {
	s := &ScanLoop{Now: func() time.Time { return coalesceNow }}
	first := s.branchFor(loopengine.Item{ID: "a#1"})
	second := s.branchFor(loopengine.Item{ID: "a#2"})
	if first != second {
		t.Fatalf("two items in ONE pass produced two branches (%q, %q) — the pass has one scan branch", first, second)
	}
	if !strings.Contains(first, "2026-08-24-120000") {
		t.Fatalf("branch = %q, want a second-granular stamp", first)
	}
	// A DIFFERENT pass on the same frozen clock must still not collide.
	other := (&ScanLoop{Now: func() time.Time { return coalesceNow }}).branchFor(loopengine.Item{ID: "a#1"})
	if other == first {
		t.Fatalf("two passes at the same instant produced the SAME branch %q — `worktree remove` does not "+
			"delete a branch, so the second pass would fail `worktree add -b`", first)
	}
	// The worktree path is derived from the same pass name, so it cannot disagree with the branch.
	if !strings.Contains(s.worktreeFor(loopengine.Item{ID: "a#1"}), strings.TrimPrefix(first, "chore/intake-scan-")) {
		t.Fatalf("the scan worktree path is not derived from the pass name: %q vs %q", s.worktreeFor(loopengine.Item{ID: "a#1"}), first)
	}
}

func TestParseRepoSlug(t *testing.T) {
	cases := map[string]string{
		"git@github.com:medici-finance/assay.git\n":    "medici-finance/assay",
		"https://github.com/medici-finance/assay":      "medici-finance/assay",
		"https://github.com/medici-finance/assay.git ": "medici-finance/assay",
		"ssh://git@github.com/medici-finance/assay":    "medici-finance/assay",
	}
	for in, want := range cases {
		got, ok := parseRepoSlug(in)
		if !ok || got != want {
			t.Fatalf("parseRepoSlug(%q) = %q %v, want %q", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "not a url", "https://github.com/", "/leading"} {
		if got, ok := parseRepoSlug(bad); ok {
			t.Fatalf("parseRepoSlug(%q) = %q, want a refusal — a half-parsed remote writes the delta to the wrong repo", bad, got)
		}
	}
}

// TestResolveScanTarget_FailsClosed — a scan whose destination could not be established is
// could-not-check. Guessing it writes a placeholder delta to the wrong repository.
func TestResolveScanTarget_FailsClosed(t *testing.T) {
	o := &planOptions{root: t.TempDir()}
	_, err := o.resolveScanTarget(func(string, string, ...string) (string, error) {
		return "", os.ErrNotExist
	})
	if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("err = %v, want unverifiable", err)
	}
	o = &planOptions{root: t.TempDir(), scanTarget: "not-a-slug"}
	if _, err := o.resolveScanTarget(nil); deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("err = %v, want a refusal for a malformed --scan-target", err)
	}
	o = &planOptions{root: t.TempDir()}
	got, err := o.resolveScanTarget(func(string, string, ...string) (string, error) {
		return "git@github.com:medici-finance/assay.git\n", nil
	})
	if err != nil || got != "medici-finance/assay" {
		t.Fatalf("resolveScanTarget = %q, %v", got, err)
	}
}

// io_Discard is a tiny writer sink; the plan/run paths take an io.Writer and the adapter defaults to
// stdout, which a test must not spray.
type io_Discard struct{}

func (io_Discard) Write(p []byte) (int, error) { return len(p), nil }
