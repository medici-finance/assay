package main

import (
	"context"
	"encoding/json"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func maintenanceFixture(t *testing.T) (string, *[][]string) {
	t.Helper()
	work := newRepo(t)
	withEnv(t, work)
	old := maintenanceCommand
	calls := [][]string{}
	maintenanceCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		calls = append(calls, append([]string(nil), args...))
		return exec.CommandContext(ctx, name, args...)
	}
	t.Cleanup(func() { maintenanceCommand = old })
	return work, &calls
}
func maintenanceRun(t *testing.T, args ...string) (maintenanceReport, error) {
	t.Helper()
	var err error
	out := runCapOut(t, func() { err = cmdMaintain(args) })
	var report maintenanceReport
	if out != "" {
		if e := json.Unmarshal([]byte(out), &report); e != nil {
			t.Fatalf("decode report: %v: %s", e, out)
		}
	}
	return report, err
}
func maintenanceWrites(calls [][]string) int {
	n := 0
	for _, c := range calls {
		for _, a := range c {
			if a == "pack-refs" || a == "commit-graph" {
				n++
				break
			}
		}
	}
	return n
}
func TestMaintenancePlanAndPreservation(t *testing.T) {
	work, calls := maintenanceFixture(t)
	sha := mustGit(t, work, "rev-parse", "HEAD")
	mustGit(t, work, "update-ref", "refs/dispatch/live", sha)
	mustGit(t, work, "config", "gc.reflogExpire", "60.days")
	writeFile(t, filepath.Join(work, "README.md"), "uncommitted work\n")
	before := mustGit(t, work, "show-ref")
	reflog := mustGit(t, work, "reflog", "--all")
	plan, err := maintenanceRun(t, "--repo", work)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != "plan" || plan.Before.RefNamespaces["refs/dispatch"] != 1 || plan.Before.Worktrees != 1 {
		t.Fatalf("bad plan: %+v", plan)
	}
	if !strings.Contains(plan.Before.ReflogRetention, "60.days") {
		t.Fatal("missing retention override")
	}
	if _, err := os.Stat(filepath.Join(plan.CommonDir, "assay-maintenance.lock")); !os.IsNotExist(err) {
		t.Fatal("plan wrote lock")
	}
	if maintenanceWrites(*calls) != 0 {
		t.Fatal("plan mutated Git")
	}
	report, err := maintenanceRun(t, "--repo", work, "--apply")
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "completed" || report.After == nil || maintenanceWrites(*calls) != 2 {
		t.Fatalf("unexpected apply %+v %v", report, *calls)
	}
	if got := mustGit(t, work, "show-ref"); got != before {
		t.Fatal("refs changed")
	}
	if got := mustGit(t, work, "reflog", "--all"); got != reflog {
		t.Fatal("recovery history changed")
	}
	if b, _ := os.ReadFile(filepath.Join(work, "README.md")); string(b) != "uncommitted work\n" {
		t.Fatal("dirty contents changed")
	}
	mustGit(t, work, "commit-graph", "verify")
	for _, c := range *calls {
		for _, a := range c {
			switch a {
			case "gc", "fetch", "prefetch", "repack", "prune", "reflog", "update-ref":
				t.Fatalf("forbidden task %v", c)
			}
		}
	}
}
func TestMaintenanceCommonDirCadenceAndHeldLock(t *testing.T) {
	work, calls := maintenanceFixture(t)
	linked := filepath.Join(t.TempDir(), "linked")
	mustGit(t, work, "worktree", "add", "--detach", linked)
	first, err := maintenanceRun(t, "--repo", work, "--apply")
	if err != nil {
		t.Fatal(err)
	}
	again, err := maintenanceRun(t, "--repo", linked, "--apply")
	if err != nil {
		t.Fatal(err)
	}
	if again.CommonDir != first.CommonDir || again.Status != "recent" || maintenanceWrites(*calls) != 2 {
		t.Fatalf("not coalesced: %+v", again)
	}
	f, err := os.OpenFile(filepath.Join(first.CommonDir, "assay-maintenance.lock"), os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = deskkit.TryLockExclusive(f); err != nil {
		t.Fatal(err)
	}
	defer deskkit.UnlockFile(f)
	held, err := maintenanceRun(t, "--repo", linked, "--apply")
	if err != nil || held.Status != "held" || maintenanceWrites(*calls) != 2 {
		t.Fatalf("lock not respected: %+v %v", held, err)
	}
}
func TestMaintenanceBorrowerSurvivesUnreachableSourceCommit(t *testing.T) {
	work, _ := maintenanceFixture(t)
	mustGit(t, work, "switch", "-c", "unique")
	writeFile(t, filepath.Join(work, "borrowed.txt"), "borrowed content\n")
	mustGit(t, work, "add", "borrowed.txt")
	mustGit(t, work, "commit", "-m", "borrower only")
	sha := mustGit(t, work, "rev-parse", "HEAD")
	borrower := filepath.Join(t.TempDir(), "borrower")
	mustGit(t, "", "clone", "--shared", work, borrower)
	mustGit(t, work, "switch", "main")
	mustGit(t, work, "branch", "-D", "unique")
	report, err := maintenanceRun(t, "--repo", work, "--apply")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(report.Reclamation, "held:") {
		t.Fatal("borrower uncertainty not reported")
	}
	if got := mustGit(t, borrower, "show", sha+":borrowed.txt"); got != "borrowed content" {
		t.Fatal("borrowed object lost")
	}
}
func TestMaintenanceIgnoresAmbientGitRedirection(t *testing.T) {
	work, _ := maintenanceFixture(t)
	victim := newRepo(t)
	t.Setenv("GIT_DIR", filepath.Join(victim, ".git"))
	t.Setenv("GIT_OBJECT_DIRECTORY", filepath.Join(victim, ".git", "objects"))
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "gc.reflogExpire")
	t.Setenv("GIT_CONFIG_VALUE_0", "now")
	report, err := maintenanceRun(t, "--repo", work, "--apply")
	if err != nil {
		t.Fatal(err)
	}
	common, _ := filepath.EvalSymlinks(filepath.Join(work, ".git"))
	if report.CommonDir != common {
		t.Fatal("ambient GIT_DIR redirected operation")
	}
	if strings.Contains(report.Before.ReflogRetention, "now") {
		t.Fatal("ambient config admitted")
	}
	if _, err := os.Stat(filepath.Join(victim, ".git", "assay-maintenance.lock")); !os.IsNotExist(err) {
		t.Fatal("victim mutated")
	}
}
func TestMaintenanceBadStampAndFailedInventoryHold(t *testing.T) {
	work, calls := maintenanceFixture(t)
	stamp := filepath.Join(work, ".git", "assay-maintenance.lock")
	writeFile(t, stamp, "malformed\n")
	_, err := maintenanceRun(t, "--repo", work, "--apply")
	if err == nil || maintenanceWrites(*calls) != 0 {
		t.Fatal("bad stamp did not hold")
	}
	writeFile(t, stamp, "")
	old := maintenanceCommand
	maintenanceCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if strings.Contains(strings.Join(args, " "), "for-each-ref") {
			return exec.CommandContext(ctx, "git", "invalid-fixture-subcommand")
		}
		return old(ctx, name, args...)
	}
	_, err = maintenanceRun(t, "--repo", work, "--apply")
	if err == nil || maintenanceWrites(*calls) != 0 {
		t.Fatal("failed inventory did not hold")
	}
	maintenanceCommand = old
	report, err := maintenanceRun(t, "--repo", work, "--apply")
	if err != nil || report.Status != "recent" {
		t.Fatal("failed attempt was not debounced")
	}
}
func TestMaintenanceSlowHelper(t *testing.T) {
	if os.Getenv("ASSAY_MAINTENANCE_SLOW_HELPER") == "1" {
		time.Sleep(time.Minute)
		os.Exit(0)
	}
}
func TestMaintenanceDeadline(t *testing.T) {
	work, _ := maintenanceFixture(t)
	old := maintenanceCommand
	t.Setenv("ASSAY_MAINTENANCE_SLOW_HELPER", "1")
	maintenanceCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if strings.Contains(strings.Join(args, " "), "pack-refs") {
			return exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMaintenanceSlowHelper$")
		}
		return old(ctx, name, args...)
	}
	start := time.Now()
	report, err := maintenanceRun(t, "--repo", work, "--apply", "--timeout", "1s")
	if err == nil || report.Status != "could-not-check" {
		t.Fatalf("deadline not reported: %+v %v", report, err)
	}
	if time.Since(start) > 4*time.Second {
		t.Fatal("deadline did not terminate builtin")
	}
}

func TestMaintenanceLockSymlinkDoesNotTruncate(t *testing.T) {
	work, calls := maintenanceFixture(t)
	victim := filepath.Join(t.TempDir(), "precious")
	// A malformed stamp would be refused even if no-follow were removed, masking
	// that regression. An expired, valid stamp would allow a write through the link.
	stamp, err := json.Marshal(time.Now().Add(-24 * time.Hour).UTC())
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, victim, string(stamp))
	if err := os.Symlink(victim, filepath.Join(work, ".git", "assay-maintenance.lock")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err = maintenanceRun(t, "--repo", work, "--apply")
	if err == nil || maintenanceWrites(*calls) != 0 {
		t.Fatal("symlink lock was accepted")
	}
	b, err := os.ReadFile(victim)
	if err != nil || string(b) != string(stamp) {
		t.Fatal("lock target changed")
	}
}

func TestMaintenanceDeniesTransportDespiteProtocolOverride(t *testing.T) {
	work, _ := maintenanceFixture(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Error(w, "fixture transport must never be contacted", http.StatusNotFound)
	}))
	defer server.Close()
	// Per-protocol configuration takes precedence over protocol.allow=never.
	// The child must still refuse transport, including a hostile ambient allowlist.
	mustGit(t, work, "config", "protocol.http.allow", "always")
	t.Setenv("GIT_ALLOW_PROTOCOL", "http")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result := maintenanceGit(ctx, work, "ls-remote", server.URL+"/fixture.git")
	if !result.Failed() || !strings.Contains(result.Stderr, "transport 'http' not allowed") {
		t.Fatalf("expected Git's transport refusal, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if requests.Load() != 0 {
		t.Fatalf("maintenance contacted HTTP transport %d times", requests.Load())
	}
}

func TestMaintenancePreservesRefMovedAfterInventory(t *testing.T) {
	work, _ := maintenanceFixture(t)
	previous := mustGit(t, work, "rev-parse", "HEAD")
	mustGit(t, work, "update-ref", "refs/heads/moving", previous)
	writeFile(t, filepath.Join(work, "next.txt"), "next\n")
	mustGit(t, work, "add", "next.txt")
	mustGit(t, work, "commit", "-m", "next")
	next := mustGit(t, work, "rev-parse", "HEAD")
	old := maintenanceCommand
	moved := false
	maintenanceCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if !moved && strings.Contains(strings.Join(args, " "), "pack-refs") {
			mustGit(t, work, "update-ref", "refs/heads/moving", next, previous)
			moved = true
		}
		return old(ctx, name, args...)
	}
	_, err := maintenanceRun(t, "--repo", work, "--apply")
	if err != nil {
		t.Fatal(err)
	}
	if !moved || mustGit(t, work, "rev-parse", "refs/heads/moving") != next {
		t.Fatal("maintenance clobbered moved ref")
	}
}
