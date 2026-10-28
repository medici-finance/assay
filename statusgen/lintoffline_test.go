package main

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// lintoffline_test.go — forge-neutral/18 row 4 (TestLintOfflineMakesNoNetworkCall), and the
// class guard over every other plain-`--lint` code path.
//
// WHAT IS PROVEN, AND HOW. A full `--lint` with no `--forge` must start no forge process and
// make no network call (docs/statusgen-lint-reach.md). Removing `gh` from PATH does NOT prove
// that: a call site that looks `gh` up, fails, and degrades to a could-not-check passes such a
// test while still being a forge read in every environment that has `gh` — which is exactly
// how dead-claim decay's branch read (then `gh api graphql`, now an in-process GraphQL request), and
// the `git ls-remote --heads origin` claim read in front of it, ran on every plain --lint while the old version of this test stayed green (it
// asserted the failed lookup's NOTICE as its proof). Here every forge or network tool on PATH
// is PRESENT — a logging shim that records its argv and then fails — so a process that starts
// and then fails is caught at the start, not excused by the failure:
//
//   - gh, glab, deskread, curl, wget, ssh, nc, docker: any invocation at all fails the test.
//   - git: a logging wrapper that runs the real git, so the run behaves normally; any
//     invocation whose subcommand talks to a remote (ls-remote, fetch, pull, push, clone,
//     submodule, `remote` other than get-url) fails the test.
//   - net/http: http.DefaultTransport — which every HTTP client in this module resolves through
//     (claimdecay_gitlab.go, ghfetch.go, telemetry.go all leave Transport nil) — is replaced
//     by one that records the request and refuses it.
//
// The guard is not a list of the call sites known today: it watches the process table and the
// transport for the WHOLE run, so a forge read added to any other --lint path trips it too.
// The positive control runs the same fixture with forge reads opted in and requires the shims
// to RECORD the claim read and decay — proof the instrument can see what it says is absent.

// lintNetShims are the forge/network tools the guard puts on PATH as fail-on-call shims.
var lintNetShims = []string{"gh", "glab", "deskread", "curl", "wget", "ssh", "nc", "docker"}

// gitRemoteSubcmds are git subcommands that contact a remote.
var gitRemoteSubcmds = map[string]bool{
	"ls-remote": true, "fetch": true, "pull": true, "push": true, "clone": true, "submodule": true,
}

type netGuard struct {
	shimDir string
	log     string
	mu      sync.Mutex
	http    []string
}

// installNetGuard builds the shim PATH and the refusing transport for this test. With
// forgeShims false, PATH carries only the recording git wrapper: no gh, no deskread, nothing
// else — exec.LookPath fails closed for every forge tool.
func installNetGuard(t *testing.T, forgeShims bool) *netGuard {
	t.Helper()
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not on PATH — cannot build the fixture")
	}
	g := &netGuard{shimDir: t.TempDir()}
	g.log = filepath.Join(t.TempDir(), "exec.log")
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(g.shimDir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write("git", "#!/bin/sh\nprintf '%s\\n' \"git $*\" >> \""+g.log+"\"\nexec \""+gitBin+"\" \"$@\"\n")
	for _, n := range lintNetShims {
		if !forgeShims {
			break
		}
		write(n, "#!/bin/sh\nprintf '%s\\n' \""+n+" $*\" >> \""+g.log+"\"\nexit 1\n")
	}
	t.Setenv("PATH", g.shimDir)

	prev := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		g.mu.Lock()
		g.http = append(g.http, r.Method+" "+r.URL.String())
		g.mu.Unlock()
		return nil, errNetGuardRefused
	})
	t.Cleanup(func() { http.DefaultTransport = prev })
	return g
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

var errNetGuardRefused = &netGuardErr{}

type netGuardErr struct{}

func (*netGuardErr) Error() string { return "network call refused by the --lint reach guard" }

// calls returns every recorded exec line.
func (g *netGuard) calls(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(g.log)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

// violations returns each recorded forge process, remote-contacting git, and HTTP request.
func (g *netGuard) violations(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, line := range g.calls(t) {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		if f[0] != "git" {
			out = append(out, "forge/network process started: "+line)
			continue
		}
		if sub, next := gitSubcommand(f[1:]); gitRemoteSubcmds[sub] || (sub == "remote" && next != "get-url") {
			out = append(out, "git contacted a remote: "+line)
		}
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, r := range g.http {
		out = append(out, "HTTP request: "+r)
	}
	return out
}

// gitSubcommand skips git's global options and returns the subcommand and the argument after it.
func gitSubcommand(args []string) (sub, next string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-C" || a == "-c" || a == "--git-dir" || a == "--work-tree" || a == "--namespace":
			i++
		case strings.HasPrefix(a, "-"):
		default:
			if i+1 < len(args) {
				next = args[i+1]
			}
			return a, next
		}
	}
	return "", ""
}

// lintReachFixture is testdata/goodrepo as a real git repo whose origin is a local bare repo
// carrying one claim-shaped branch — so the claim read returns a non-empty list and dead-claim
// decay is genuinely reached whenever claims are read at all. Without the branch, decay
// short-circuits on an empty list and its GraphQL branch read would never be attempted.
func lintReachFixture(t *testing.T) string {
	t.Helper()
	bareDir := t.TempDir()
	runGit(t, bareDir, "init", "--bare")
	seedDir := t.TempDir()
	gitInit(t, seedDir, "Seed Author", "seed@example.com")
	if err := os.WriteFile(filepath.Join(seedDir, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, seedDir, "add", "f.txt")
	runGit(t, seedDir, "commit", "-m", "seed")
	runGit(t, seedDir, "branch", "-M", "fix/alpha-02-inflight")
	runGit(t, seedDir, "remote", "add", "origin", bareDir)
	runGit(t, seedDir, "push", "origin", "fix/alpha-02-inflight")

	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	// An active drive with a spliced region, rendered with the branch's claim on alpha/02:
	// without one, the drive-snapshot lint is inert and this guard cannot see its claim read
	// (#3323). Callers pin the clock to driveTestNow so the drive is in its window.
	writeAlphaPlan(t, root, alphaDriveRegion(t, root, map[string]bool{"alpha/02": true}))
	gitInit(t, root, "Test Author", "test@example.com")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "initial")
	runGit(t, root, "remote", "add", "origin", bareDir)
	return root
}

// alphaDriveManifest is an active drive over goodrepo's stream alpha. Its todo brief
// alpha/02 renders under In-flight when claimed and under Frontier next when not, so
// the drive-snapshot region depends on the claim set.
const alphaDriveManifest = "declared-by: fixture\n" + liveWindow +
	"intensity: focus\nstate: active\nwhy: reach fixture\nitems:\n  - stream: alpha\n"

// alphaDriveRegion writes the alpha drive's manifest under root and returns its
// snapshot section rendered with the given claim set. It reads no claims itself.
func alphaDriveRegion(t *testing.T, root string, claimed map[string]bool) string {
	t.Helper()
	writeDrive(t, root, "d1", alphaDriveManifest)
	streams, _, err := loadHydratedStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	section, ok := driveSnapshotRender(loadDrives(root, streams, driveTestNow), streams, claimed, "d1", "", driveTestNow)
	if !ok {
		t.Fatal("fixture drive d1 must render")
	}
	return section
}

// writeAlphaPlan writes the alpha drive's plan file with region spliced between the markers.
func writeAlphaPlan(t *testing.T, root, region string) {
	t.Helper()
	writePlanMD(t, root, "d1", "# Drive d1\n\n"+driveSnapshotBeginPrefix+"d1 @deadbee 2026-08-15"+
		driveSnapshotMarkerSuffix+"\n"+region+driveSnapshotEndMarker+"\n")
}

func TestLintOfflineMakesNoNetworkCall(t *testing.T) {
	pinNow(t, driveTestNow)
	root := lintReachFixture(t) // built before any guard: fixture setup is not the run under test

	// The positive control, first: with forge reads opted in, the SAME fixture's claim read and
	// decay must be RECORDED by the guard — the remote read, and the decay's GraphQL request
	// (refused by the guard's transport). A token and a repository are supplied so the decay
	// gets as far as the request. An instrument that cannot see them proves nothing by silence.
	var networkedCode int
	t.Run("control", func(t *testing.T) {
		g := installNetGuard(t, true)
		t.Setenv("GH_TOKEN", "control-token")
		t.Setenv("GITHUB_REPOSITORY", "acme/widgets")
		t.Setenv("GITHUB_GRAPHQL_URL", "")
		forgeReadsOptedIn = true
		t.Cleanup(func() { forgeReadsOptedIn = false })
		_ = captureStderr(t, func() { networkedCode = run(root, "lint", nil, nil, "") })
		joined := strings.Join(g.violations(t), "\n")
		for _, want := range []string{"ls-remote", "POST https://api.github.com/graphql"} {
			if !strings.Contains(joined, want) {
				t.Errorf("guard did not record %q with forge reads opted in — it cannot see what this "+
					"row says is absent; recorded:\n%s", want, joined)
			}
		}
	})

	// The run under test, twice: with every forge/network tool PRESENT as a recording shim
	// (catches a process that starts and fails), and with none of them on PATH at all (the
	// row's literal harness). Both must touch nothing and reach the networked run's verdict.
	for _, shims := range []bool{true, false} {
		name := "offline-shims"
		if !shims {
			name = "offline-bare"
		}
		t.Run(name, func(t *testing.T) {
			g := installNetGuard(t, shims)
			forgeReadsOptedIn = false
			var code int
			stderr := captureStderr(t, func() { code = run(root, "lint", nil, nil, "") })
			if v := g.violations(t); len(v) > 0 {
				t.Fatalf("a plain --lint (no --forge) reached the forge or the network:\n  %s\nstderr:\n%s",
					strings.Join(v, "\n  "), stderr)
			}
			if code != 0 || code != networkedCode {
				t.Fatalf("offline --lint exited %d; want 0, the networked run's verdict (%d):\n%s",
					code, networkedCode, stderr)
			}
			// The unread claim set is could-not-check, rendered as itself — never a clean read.
			if !strings.Contains(stderr, offlineLintClaimReason) {
				t.Errorf("offline --lint must say its claim set is could-not-check; stderr:\n%s", stderr)
			}
			// The run really went through the shims, so an empty violation list is a reading.
			if len(g.calls(t)) == 0 {
				t.Fatal("guard recorded no git process at all — the shim PATH was not in effect")
			}
		})
	}
}

// TestOfflineLintRequireClaims: an offline --lint has no claim set, so --require-claims must
// fail closed on it (exit 1, naming --forge) rather than pass on claims it never read; with
// forge reads opted in and a readable origin, the same run passes.
func TestOfflineLintRequireClaims(t *testing.T) {
	root := goodRepoRoot(t)
	// A readable origin with no branches: the claim read succeeds and decay has nothing to
	// read, so the opted-in half reaches no forge either.
	stubRemoteBranches(t, nil, nil)
	requireClaims = true
	t.Cleanup(func() { requireClaims = false; forgeReadsOptedIn = false })

	forgeReadsOptedIn = false
	var code int
	stderr := captureStderr(t, func() { code = run(root, "lint", nil, nil, "") })
	if code != 1 || !strings.Contains(stderr, "--require-claims") || !strings.Contains(stderr, "--forge") {
		t.Fatalf("offline --lint --require-claims exited %d, want 1 naming --require-claims and --forge:\n%s", code, stderr)
	}
	forgeReadsOptedIn = true
	if code := run(root, "lint", nil, nil, ""); code != 0 {
		t.Fatalf("--lint --forge --require-claims with a readable origin exited %d, want 0", code)
	}
}

// TestRegenStartsNoForgeProcess is the class guard for #3372: the board regen
// (`statusgen --root .`, mode "write") runs on a runner that carries git and the
// statusgen binary and nothing else, so a read on that path that starts `gh` (or
// any other forge tool) cannot run there — and fails open, so the board quietly
// loses whatever the read was for. Every forge tool is on PATH as a recording
// shim; the regen must start none of them. The positive control is in the same
// run: the decay's GraphQL request must be recorded by the guard's transport, so
// the claim path was really reached.
//
// regression: #3372
func TestRegenStartsNoForgeProcess(t *testing.T) {
	pinNow(t, driveTestNow)
	root := lintReachFixture(t)
	g := installNetGuard(t, true)
	t.Setenv("GH_TOKEN", "regen-token")
	t.Setenv("GITHUB_REPOSITORY", "acme/widgets")
	t.Setenv("GITHUB_GRAPHQL_URL", "")
	stderr := captureStderr(t, func() { _ = run(root, "write", nil, nil, "") })
	var procs []string
	for _, line := range g.calls(t) {
		if f := strings.Fields(line); len(f) > 0 && f[0] != "git" {
			procs = append(procs, line)
		}
	}
	if len(procs) > 0 {
		t.Fatalf("the board regen started a forge process the regen runner does not have:\n  %s\nstderr:\n%s",
			strings.Join(procs, "\n  "), stderr)
	}
	g.mu.Lock()
	reached := strings.Contains(strings.Join(g.http, "\n"), "POST https://api.github.com/graphql")
	g.mu.Unlock()
	if !reached {
		t.Fatalf("control: the regen made no decay GraphQL request, so the claim path was not exercised; stderr:\n%s", stderr)
	}
}

// TestDriveLintOfflineNoClaims pins the drive-snapshot half of the reach contract: a plain
// --lint over a drive region reads no claims (no remote branch list, no dead-claim decay),
// and a region whose In-flight list only a claim can explain reads could-not-check — never
// a drift PROBLEM decided on claims nobody read. A region an unread claim could make stale
// reads could-not-check too — never a clean pass. A hand edit no claim set can explain is
// still drift offline, and with --forge the live claim read decides as before.
//
// regression: #3323
func TestDriveLintOfflineNoClaims(t *testing.T) {
	pinNow(t, driveTestNow)
	root := goodRepoRoot(t)
	claimedRegion := alphaDriveRegion(t, root, map[string]bool{"alpha/02": true})
	freeRegion := alphaDriveRegion(t, root, nil)
	if normalizeDriveSnapshot(claimedRegion) == normalizeDriveSnapshot(freeRegion) {
		t.Fatal("fixture: the claim on alpha/02 must change the region, or nothing here depends on claims")
	}

	reads, decays := 0, 0
	prevList, prevDecay := listRemoteBranches, listMergedClosedBranches
	listRemoteBranches = func(string) ([]string, error) { reads++; return []string{"fix/alpha-02-inflight"}, nil }
	listMergedClosedBranches = func(string) (map[string]bool, error) { decays++; return nil, nil }
	t.Cleanup(func() {
		listRemoteBranches, listMergedClosedBranches = prevList, prevDecay
		forgeReadsOptedIn = false
	})
	lint := func(forge bool, region string) (int, string) {
		t.Helper()
		writeAlphaPlan(t, root, region)
		forgeReadsOptedIn = forge
		reads, decays = 0, 0
		var code int
		stderr := captureStderr(t, func() { code = run(root, "lint", nil, nil, "") })
		return code, stderr
	}
	const drift = "PROBLEM: drive-region-drift: docs/roadmap/drives/d1.md"
	const regionNotice = "NOTICE: drive-region-drift: docs/roadmap/drives/d1.md"

	// Offline, a region spliced while alpha/02 was claimed: no claim read, no drift
	// PROBLEM, and a could-not-check naming the claim the comparison had to assume.
	code, stderr := lint(false, claimedRegion)
	if reads != 0 || decays != 0 {
		t.Fatalf("plain --lint read claims for the drive region: %d branch reads, %d decay reads", reads, decays)
	}
	if code != 0 || strings.Contains(stderr, drift) {
		t.Fatalf("offline --lint decided drift on claims it did not read (exit %d):\n%s", code, stderr)
	}
	if !strings.Contains(stderr, regionNotice) || !strings.Contains(stderr, "alpha/02 is claimed") ||
		!strings.Contains(stderr, "could-not-check") {
		t.Fatalf("offline --lint must report the claim-dependent region as could-not-check naming alpha/02:\n%s", stderr)
	}

	// Offline, a region matching the unclaimed render: no drift PROBLEM, but never a clean
	// pass either. A live claim on the todo brief alpha/02 would move it to In-flight and
	// make this region stale, and this run did not read the claim set, so it is
	// could-not-check naming alpha/02 as assumed unclaimed. --forge decides it (below).
	code, stderr = lint(false, freeRegion)
	if code != 0 || strings.Contains(stderr, drift) || reads != 0 {
		t.Fatalf("offline --lint decided drift on claims it did not read (exit %d, reads %d):\n%s", code, reads, stderr)
	}
	if !strings.Contains(stderr, regionNotice) || !strings.Contains(stderr, "alpha/02 is not claimed") ||
		!strings.Contains(stderr, "could-not-check") {
		t.Fatalf("offline --lint must not pass a region an unread claim could make stale; want a could-not-check naming alpha/02:\n%s", stderr)
	}

	// Offline strength: a hand edit no claim set explains is still drift.
	if code, stderr := lint(false, claimedRegion+"hand edit\n"); code == 0 || !strings.Contains(stderr, drift) || reads != 0 {
		t.Fatalf("a hand-edited region must stay a drift PROBLEM offline (exit %d, reads %d):\n%s", code, reads, stderr)
	}

	// Control, --forge: the claim set is read, and it decides. The claimed region is clean
	// with no could-not-check; the unclaimed one is drift, because alpha/02 is in flight.
	if code, stderr := lint(true, claimedRegion); code != 0 || strings.Contains(stderr, "drive-region-drift") || reads == 0 {
		t.Fatalf("--lint --forge must read claims and find the claimed region clean (exit %d, reads %d):\n%s", code, reads, stderr)
	}
	if code, stderr := lint(true, freeRegion); code == 0 || !strings.Contains(stderr, drift) {
		t.Fatalf("--lint --forge must find the unclaimed region drifted from the live claim (exit %d):\n%s", code, stderr)
	}
}

// TestDriveLintOfflineOwnInFlight pins the other half of the offline drive-region check:
// a region no claim could change is checked clean. Here alpha/02 is in flight on the
// board's own status (in-progress), so it is listed under In-flight with or without a
// claim: it is never named as an assumed claim, and with no todo brief left in the drive
// there is nothing an unread claim could move. The offline verdict is clean, with no
// region notice.
//
// regression: #3323
func TestDriveLintOfflineOwnInFlight(t *testing.T) {
	pinNow(t, driveTestNow)
	root := goodRepoRoot(t)
	readme := filepath.Join(root, "docs", "streams", "alpha", "README.md")
	b, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	const todoRow = "| 02 | [Second](brief-02.md) | 1 | todo |"
	if !strings.Contains(string(b), todoRow) {
		t.Fatal("fixture: alpha/02's todo row not found")
	}
	inProgress := strings.Replace(string(b), todoRow, "| 02 | [Second](brief-02.md) | 1 | in-progress |", 1)
	if err := os.WriteFile(readme, []byte(inProgress), 0o644); err != nil {
		t.Fatal(err)
	}
	region := alphaDriveRegion(t, root, nil)
	if !strings.Contains(region, "- alpha/02") {
		t.Fatalf("fixture: an in-progress alpha/02 must be listed under In-flight:\n%s", region)
	}
	writeAlphaPlan(t, root, region)

	prevList, prevDecay := listRemoteBranches, listMergedClosedBranches
	listRemoteBranches = func(string) ([]string, error) { t.Error("plain --lint listed remote branches"); return nil, nil }
	listMergedClosedBranches = func(string) (map[string]bool, error) { t.Error("plain --lint decayed claims"); return nil, nil }
	t.Cleanup(func() { listRemoteBranches, listMergedClosedBranches = prevList, prevDecay })
	forgeReadsOptedIn = false

	stderr := captureStderr(t, func() { run(root, "lint", nil, nil, "") })
	if strings.Contains(stderr, "drive-region-drift") {
		t.Fatalf("a region no claim could change must be clean offline, with no region notice:\n%s", stderr)
	}
}
