package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// discover_test.go — the open scan PR is DISCOVERED on the forge, not only read from flags.
//
// The incident: `scanloop run --root <repo>` (the drain exactly as the desk skill invokes it)
// cut a second scan carrier PR eleven minutes after the first, with the same delta, while the
// first was still an open draft inside the 20-minute window. The pass had no --scan-pr flags, so
// the coalesce decision read "no open scan PR" and cut a fresh one — every tick, a duplicate.

// scanForge is an httptest GitHub serving the two reads a live pass makes against the scan
// target: the trust gate's issue read, and the open-change list the coalesce discovery reads.
type scanForge struct {
	mu        sync.Mutex
	changes   []map[string]any
	listCalls int
	failList  bool
}

func (f *scanForge) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/repos/medici-finance/assay/issues/"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"number": 161, "title": "an inbound issue", "state": "open",
				"user": map[string]any{"login": "ada", "id": 2001, "type": "User"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/graphql":
			f.listCalls++
			if f.failList {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"repository": map[string]any{"pullRequests": map[string]any{"nodes": f.changes}}},
			})
		default:
			t.Errorf("unexpected forge request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

// scanChange is one open-change node in the forge's own wire shape.
func scanChange(number int, head, author, typename string, draft bool, created time.Time) map[string]any {
	return map[string]any{
		"number": number, "title": "chore(intake): scan", "state": "OPEN", "isDraft": draft,
		"createdAt":   created.UTC().Format(time.RFC3339),
		"author":      map[string]any{"login": author, "__typename": typename},
		"headRefName": head, "baseRefName": "main",
	}
}

// inboundPoller writes a poller that prints one inbound event, so the pass has a mechanical item
// and reaches the scan dispatch.
func inboundPoller(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash on PATH — execMonitor runs the poller through it")
	}
	script := filepath.Join(t.TempDir(), "inbound-monitor.sh")
	body := "#!/usr/bin/env bash\necho 'INBOUND: medici-finance/assay#161 2026-08-24T11:58:00Z'\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return script
}

// livePass runs `scanloop run --dry-run` exactly as the desk skill invokes the drain — NO
// --scan-pr flags — against the httptest forge, and returns its output.
func livePass(t *testing.T, f *scanForge) (string, error) {
	t.Helper()
	srv := httptest.NewServer(f.handler(t))
	t.Cleanup(srv.Close)
	oldBase := forgeAPIBase
	forgeAPIBase = srv.URL
	t.Cleanup(func() { forgeAPIBase = oldBase })
	stubIdentity(t,
		func(string) (string, string, error) { return "issue-loop", "intake-desk", nil },
		func(string, string) (string, string, error) { return "x-test-token", "/cache/x-test-token", nil })
	withDryRunTempBase(t)

	var out strings.Builder
	err := cmdRun([]string{
		"--root", t.TempDir(),
		"--scan-target", "medici-finance/assay",
		"--worktree-base", t.TempDir(),
		"--state-dir", armedStateDir(t),
		"--monitor", inboundPoller(t),
		"--now", "2026-08-24T12:00:00Z",
		"--dry-run",
	}, &out)
	return out.String(), err
}

// regression: duplicate scan carrier PR per tick. A pass with no --scan-pr flags must still find
// the open draft scan PR another session of the same App opened inside the window, and push into it.
func TestRun_CoalescesAcrossSessions(t *testing.T) {
	f := &scanForge{changes: []map[string]any{
		// Newest first, as the forge returns them. An unrelated PR by the same App is not a scan
		// PR; a scan-shaped branch by ANOTHER account is not ours to push to.
		scanChange(905, "fix/something-else", "assay-issue-loop-app", "Bot", true, coalesceNow.Add(-2*time.Minute)),
		scanChange(904, "chore/intake-scan-2026-08-24-115500-aaaaaa", "someone-else", "User", true, coalesceNow.Add(-5*time.Minute)),
		scanChange(900, "chore/intake-scan-2026-08-24-114900-abcdef", "assay-issue-loop-app", "Bot", true, coalesceNow.Add(-11*time.Minute)),
	}}
	out, err := livePass(t, f)
	if err != nil {
		t.Fatalf("pass errored: %v\n%s", err, out)
	}
	if strings.Contains(out, "deskpr create") {
		t.Fatalf("the pass cut a DUPLICATE scan PR while #900 (same App, 11m old, draft) was open inside the window:\n%s", out)
	}
	for _, want := range []string{
		"deskpr update",
		"worktree add",
		"origin/chore/intake-scan-2026-08-24-114900-abcdef",
		"# coalesce: COALESCE — #900",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("the coalesced pass is missing %q:\n%s", want, out)
		}
	}
	if f.listCalls != 1 {
		t.Fatalf("open-change reads = %d, want exactly ONE per pass", f.listCalls)
	}
}

// TestRun_ForeignScanPRIgnored — the control on the control: only scan PRs by THIS App
// coalesce. A scan-shaped branch from another account, or a young non-scan PR by this App, leaves
// the pass cutting a fresh PR, so the test above cannot be passing by coalescing into anything.
func TestRun_ForeignScanPRIgnored(t *testing.T) {
	f := &scanForge{changes: []map[string]any{
		scanChange(905, "fix/something-else", "assay-issue-loop-app", "Bot", true, coalesceNow.Add(-2*time.Minute)),
		scanChange(904, "chore/intake-scan-2026-08-24-115500-aaaaaa", "someone-else", "User", true, coalesceNow.Add(-5*time.Minute)),
	}}
	out, err := livePass(t, f)
	if err != nil {
		t.Fatalf("pass errored: %v\n%s", err, out)
	}
	if !strings.Contains(out, "deskpr create") || strings.Contains(out, "deskpr update") {
		t.Fatalf("with no scan PR of this App open, the pass must cut a fresh one:\n%s", out)
	}
	if !strings.Contains(out, "# coalesce: FRESH-PR — no open scan PR by this App") {
		t.Fatalf("the fresh decision does not say WHOSE scan PRs it looked for:\n%s", out)
	}
}

// TestRun_UnreadOpenPRsFailClosed — an open-change list that cannot be read is could-not-check
// on whether a scan PR is already open. Cutting a fresh PR on that reading is exactly the
// duplicate this discovery exists to prevent, so the dispatch refuses and the pass exits
// unverifiable; nothing is pushed and no PR is created.
func TestRun_UnreadOpenPRsFailClosed(t *testing.T) {
	f := &scanForge{failList: true}
	out, err := livePass(t, f)
	if err == nil {
		t.Fatalf("a pass that could not read the open scan PRs exited 0:\n%s", out)
	}
	if got := deskkit.ExitCodeOf(err); got != deskkit.ExitUnverifiable {
		t.Fatalf("a pass blind to the open scan PRs exited %d, want %d (unverifiable): %v", got, deskkit.ExitUnverifiable, err)
	}
	if strings.Contains(out, "deskpr create") || strings.Contains(out, "deskpr update") {
		t.Fatalf("a pass blind to the open scan PRs still carried the delta:\n%s", out)
	}
	if !strings.Contains(out, "dispatch-error") || !strings.Contains(out, "open scan PR") {
		t.Fatalf("the refusal does not name what could not be read:\n%s", out)
	}
}

// TestRun_ExplicitScanPRWins — the flags still win: an operator who names the open scan
// PR is not second-guessed, and the forge is not read for it.
func TestRun_ExplicitScanPRWins(t *testing.T) {
	f := &scanForge{}
	srv := httptest.NewServer(f.handler(t))
	t.Cleanup(srv.Close)
	oldBase := forgeAPIBase
	forgeAPIBase = srv.URL
	t.Cleanup(func() { forgeAPIBase = oldBase })
	stubIdentity(t,
		func(string) (string, string, error) { return "issue-loop", "intake-desk", nil },
		func(string, string) (string, string, error) { return "x-test-token", "/cache/x-test-token", nil })
	withDryRunTempBase(t)

	var out strings.Builder
	err := cmdRun([]string{
		"--root", t.TempDir(),
		"--scan-target", "medici-finance/assay",
		"--worktree-base", t.TempDir(),
		"--state-dir", armedStateDir(t),
		"--monitor", inboundPoller(t),
		"--now", "2026-08-24T12:00:00Z",
		"--scan-pr", "42", "--scan-branch", "chore/intake-scan-2026-08-24-1150",
		"--scan-pr-created", "2026-08-24T11:50:00Z", "--scan-pr-state", "draft",
		"--dry-run",
	}, &out)
	if err != nil {
		t.Fatalf("pass errored: %v\n%s", err, out.String())
	}
	if f.listCalls != 0 {
		t.Fatalf("the forge was read %d time(s) for an open scan PR the operator had already named", f.listCalls)
	}
	if !strings.Contains(out.String(), "# coalesce: COALESCE — #42") {
		t.Fatalf("the named scan PR was not coalesced into:\n%s", out.String())
	}
}

// TestSelect_DraftThenYoungest — the choice among several open scan PRs by this App: a draft
// beats a flipped one however young the flipped one is (a flipped PR is push-quiet), and among
// drafts the youngest is the one inside the window.
func TestSelect_DraftThenYoungest(t *testing.T) {
	const app = "assay-issue-loop-app[bot]"
	change := func(n int, draft bool, age time.Duration) deskkit.OpenChange {
		return deskkit.OpenChange{
			Number: n, Draft: draft, HeadRef: scanBranchPrefix + "x",
			CreatedAt: coalesceNow.Add(-age).Format(time.RFC3339), Author: deskkit.Account{Login: app},
		}
	}
	oc := &deskkit.OpenChanges{Changes: []deskkit.OpenChange{
		change(910, false, time.Minute),   // flipped, youngest
		change(909, true, 4*time.Minute),  // the youngest DRAFT
		change(903, true, 40*time.Minute), // an older, sealed draft
	}}
	pr, err := selectOpenScanPR(oc, app, coalesceNow, DefaultCoalesceWindow)
	if err != nil || pr == nil || pr.Number != 909 || pr.State != ScanPRDraft {
		t.Fatalf("picked %+v (err %v), want the youngest draft #909", pr, err)
	}

	// Only a flipped one open: it is still reported, so the decision names the flip boundary.
	pr, err = selectOpenScanPR(&deskkit.OpenChanges{Changes: oc.Changes[:1]}, app, coalesceNow, DefaultCoalesceWindow)
	if err != nil || pr == nil || pr.State != ScanPRReady {
		t.Fatalf("picked %+v (err %v), want the flipped #910 reported as ready", pr, err)
	}
	if d, _ := (CoalescePolicy{}).Decide(pr, coalesceNow); d != CoalesceFresh {
		t.Fatalf("a flipped scan PR coalesced: %s", d)
	}

	// An unparseable creation time is kept as ZERO — could-not-check, never young.
	bad := change(911, true, time.Minute)
	bad.CreatedAt = "not-a-time"
	pr, err = selectOpenScanPR(&deskkit.OpenChanges{Changes: []deskkit.OpenChange{bad}}, app, coalesceNow, DefaultCoalesceWindow)
	if err != nil || pr == nil {
		t.Fatalf("picked %+v (err %v)", pr, err)
	}
	if d, _ := (CoalescePolicy{}).Decide(pr, coalesceNow); d != CoalesceCouldNotCheck {
		t.Fatalf("an unreadable creation time decided %s, want COULD-NOT-CHECK", d)
	}
}

// TestSelect_TruncatedBounded — a list cut at the forge's cap with no scan PR in it is only
// "none open" when its oldest change is already past the window (the read is newest-first, so the
// unseen tail is older still). Inside the window the tail may hold one, so it is could-not-check.
func TestSelect_TruncatedBounded(t *testing.T) {
	other := func(age time.Duration) deskkit.OpenChange {
		return deskkit.OpenChange{Number: 1, HeadRef: "feat/x", CreatedAt: coalesceNow.Add(-age).Format(time.RFC3339)}
	}
	young := &deskkit.OpenChanges{Changes: []deskkit.OpenChange{other(time.Minute), other(5 * time.Minute)}, TruncatedAtCap: true, Cap: 2}
	if pr, err := selectOpenScanPR(young, "assay-issue-loop-app[bot]", coalesceNow, DefaultCoalesceWindow); err == nil {
		t.Fatalf("a truncated list whose tail starts inside the window read as %+v — the tail is unseen", pr)
	} else if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("exit = %d, want unverifiable: %v", deskkit.ExitCodeOf(err), err)
	}
	old := &deskkit.OpenChanges{Changes: []deskkit.OpenChange{other(time.Minute), other(time.Hour)}, TruncatedAtCap: true, Cap: 2}
	if pr, err := selectOpenScanPR(old, "assay-issue-loop-app[bot]", coalesceNow, DefaultCoalesceWindow); err != nil || pr != nil {
		t.Fatalf("a truncated list already past the window = %+v, %v; want none open", pr, err)
	}
}

// TestDiscover_UnboundRoleRefuses — an App role the roster binds no login to cannot recognise its
// own scan PR, so the answer is could-not-check, never "none open".
func TestDiscover_UnboundRoleRefuses(t *testing.T) {
	stubIdentity(t, func(string) (string, string, error) { return "no-such-role", "intake-desk", nil }, mintTokenFn)
	old := listOpenChangesFn
	listOpenChangesFn = func(string) (*deskkit.OpenChanges, error) {
		t.Fatal("the forge was read although the App login could not be established")
		return nil, nil
	}
	t.Cleanup(func() { listOpenChangesFn = old })
	pr, err := discoverOpenScanPR("medici-finance/assay", coalesceNow, DefaultCoalesceWindow)
	if err == nil {
		t.Fatalf("an unbound role read as %+v — that is a guess, not an answer", pr)
	}
	if got := deskkit.ExitCodeOf(err); got != deskkit.ExitUnverifiable {
		t.Fatalf("an unbound role exited %d, want %d (unverifiable): %v", got, deskkit.ExitUnverifiable, err)
	}
}
