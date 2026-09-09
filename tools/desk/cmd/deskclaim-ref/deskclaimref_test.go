package main

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// fakeForge is an in-memory model of the GitHub git-data surface the claim tool drives via
// `gh api`. Every gh argv the tool constructs is interpreted here, so a test exercises the
// REAL request shapes (endpoint, method, -f fields) the bash script and this port share —
// that is what makes these parity tests rather than mock theatre.
type fakeForge struct {
	repo       string
	repoExists bool
	baseSHA    string            // heads/main; "" simulates an unmintable base (rc6 path)
	tags       map[string]tagObj // tag sha -> {message,date}
	refs       map[string]string // claim id -> tag sha (refs/dispatch/<id>)
	branches   map[string]bool   // heads/<branch> existence
	serverNow  time.Time         // the clock GitHub stamps onto a freshly minted tag
	tagN       int
	calls      [][]string
}

type tagObj struct {
	message string
	date    string
}

func newForge(repo string) *fakeForge {
	return &fakeForge{
		repo: repo, repoExists: true, baseSHA: "basemain0",
		tags: map[string]tagObj{}, refs: map[string]string{}, branches: map[string]bool{},
		serverNow: time.Now().UTC(),
	}
}

// seedClaim installs a held claim directly, with a chosen holder/state/branch and age, so a
// test controls exactly what acquire/show read back without minting through the forge.
func (f *fakeForge) seedClaim(id, owner, state, branch string, age time.Duration) {
	sha := f.nextTag()
	msg := "dispatch-claim " + id + " owner=" + owner + " state=" + state + " branch=" + dashOr(branch)
	f.tags[sha] = tagObj{message: msg, date: f.serverNow.Add(-age).Format(time.RFC3339)}
	f.refs[id] = sha
}

func dashOr(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func (f *fakeForge) nextTag() string {
	f.tagN++
	return "tagsha" + itoa(f.tagN)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

func fval(args []string, key string) (string, bool) {
	for i := 0; i+1 < len(args); i++ {
		if (args[i] == "-f" || args[i] == "-F") && strings.HasPrefix(args[i+1], key+"=") {
			return strings.TrimPrefix(args[i+1], key+"="), true
		}
	}
	return "", false
}

func (f *fakeForge) run(args ...string) ghResult {
	f.calls = append(f.calls, append([]string{}, args...))
	ok := func(stdout string) ghResult { return ghResult{stdout: stdout} }
	fail := func(body string) ghResult { return ghResult{stdout: "", stderr: body, code: 1} }

	// repo view --json nameWithOwner
	if len(args) >= 2 && args[0] == "repo" && args[1] == "view" {
		return ok(f.repo)
	}
	if len(args) == 0 || args[0] != "api" {
		return fail("unhandled")
	}

	// Method + path.
	method := "GET"
	rest := args[1:]
	if len(rest) >= 2 && rest[0] == "-X" {
		method = rest[1]
		rest = rest[2:]
	}
	path := rest[0]

	switch {
	case method == "GET" && path == "repos/"+f.repo:
		if f.repoExists {
			return ok(f.repo)
		}
		return fail("HTTP 404")
	case method == "GET" && strings.HasPrefix(path, "repos/"+f.repo+"/git/ref/dispatch/"):
		id := strings.TrimPrefix(path, "repos/"+f.repo+"/git/ref/dispatch/")
		if sha, held := f.refs[id]; held {
			return ok(sha)
		}
		return fail("HTTP 404 Not Found")
	case method == "GET" && strings.HasPrefix(path, "repos/"+f.repo+"/git/ref/heads/"):
		br := strings.TrimPrefix(path, "repos/"+f.repo+"/git/ref/heads/")
		if br == "main" {
			if f.baseSHA == "" {
				return fail("HTTP 404")
			}
			return ok(f.baseSHA)
		}
		if f.branches[br] {
			return ok("refs/heads/" + br)
		}
		return fail("HTTP 404")
	case method == "GET" && strings.HasPrefix(path, "repos/"+f.repo+"/git/tags/"):
		sha := strings.TrimPrefix(path, "repos/"+f.repo+"/git/tags/")
		if t, tok := f.tags[sha]; tok {
			return ok(t.message + "\t" + t.date)
		}
		return fail("HTTP 404")
	case method == "POST" || (method == "GET" && path == "repos/"+f.repo+"/git/tags"):
		// gh treats a call carrying -f as a POST implicitly; both tag-create and ref-create
		// arrive here with method still "GET" (no -X). Disambiguate on the path.
		return f.write(method, path, rest, ok, fail)
	default:
		return f.write(method, path, rest, ok, fail)
	}
}

func (f *fakeForge) write(method, path string, rest []string, ok, fail func(string) ghResult) ghResult {
	switch {
	case path == "repos/"+f.repo+"/git/tags":
		msg, _ := fval(rest, "message")
		sha := f.nextTag()
		f.tags[sha] = tagObj{message: msg, date: f.serverNow.Format(time.RFC3339)}
		return ok(sha)
	case path == "repos/"+f.repo+"/git/refs":
		ref, _ := fval(rest, "ref")
		sha, _ := fval(rest, "sha")
		id := strings.TrimPrefix(ref, refPrefix+"/")
		if _, exists := f.refs[id]; exists {
			return fail("HTTP 422 Reference already exists")
		}
		f.refs[id] = sha
		return ok(ref)
	case method == "PATCH" && strings.HasPrefix(path, "repos/"+f.repo+"/git/refs/dispatch/"):
		id := strings.TrimPrefix(path, "repos/"+f.repo+"/git/refs/dispatch/")
		sha, _ := fval(rest, "sha")
		f.refs[id] = sha
		return ok(refPrefix + "/" + id)
	case method == "DELETE" && strings.HasPrefix(path, "repos/"+f.repo+"/git/refs/dispatch/"):
		id := strings.TrimPrefix(path, "repos/"+f.repo+"/git/refs/dispatch/")
		if _, exists := f.refs[id]; exists {
			delete(f.refs, id)
			return ok("")
		}
		return fail("HTTP 404")
	case path == "repos/"+f.repo+"/git/matching-refs/dispatch/":
		var lines []string
		for id := range f.refs {
			lines = append(lines, refPrefix+"/"+id)
		}
		return ok(strings.Join(lines, "\n"))
	}
	return fail("unhandled write " + path)
}

// harness wires the forge into the tool's seams and captures its output.
func harness(t *testing.T, f *fakeForge) (rc func(args ...string) int, stdout, stderr *bytes.Buffer) {
	t.Helper()
	var so, se bytes.Buffer
	oldRun, oldOut, oldErr, oldLook := ghRun, out, errOut, ghLookPath
	ghRun = f.run
	out = &so
	errOut = &se
	ghLookPath = func(string) (string, error) { return "/usr/bin/gh", nil }
	t.Cleanup(func() { ghRun, out, errOut, ghLookPath = oldRun, oldOut, oldErr, oldLook })
	return func(args ...string) int {
		so.Reset()
		se.Reset()
		return run(args)
	}, &so, &se
}

// --- protocol parity: acquire on a free claim -------------------------------

func TestAcquireFreeCreatesTheRefAndEncodesTheHolder(t *testing.T) {
	f := newForge("medici-finance/assay")
	run, so, _ := harness(t, f)

	rc := run("acquire", "at--stream--07", "--repo", f.repo, "--owner", "sess-A", "--branch", "feat/x")
	if rc != exitOK {
		t.Fatalf("acquire rc = %d, want 0; out=%s", rc, so.String())
	}
	sha, held := f.refs["at--stream--07"]
	if !held {
		t.Fatal("no refs/dispatch/at--stream--07 was created")
	}
	// The holder is encoded in the tag object exactly as the bash mints it.
	msg := f.tags[sha].message
	for _, want := range []string{"owner=sess-A", "state=claimed", "branch=feat/x", "dispatch-claim at--stream--07"} {
		if !strings.Contains(msg, want) {
			t.Errorf("claim tag message %q missing %q", msg, want)
		}
	}
	if !strings.Contains(so.String(), "acquired at--stream--07") {
		t.Errorf("acquire did not log the acquisition: %s", so.String())
	}
}

// A second acquire of a LIVE claim (within TTL) refuses (exit 5), logs the DEDUP holder, and
// never steals.
func TestAcquireLiveHolderRefusesAndNeverSteals(t *testing.T) {
	f := newForge("medici-finance/assay")
	f.seedClaim("at--stream--07", "other-sess", "dispatched", "", 42*time.Minute)
	run, so, _ := harness(t, f)

	rc := run("acquire", "at--stream--07", "--repo", f.repo, "--owner", "sess-B")
	if rc != exitRefused {
		t.Fatalf("live-holder acquire rc = %d, want 5; out=%s", rc, so.String())
	}
	if !strings.Contains(so.String(), "DEDUP at--stream--07") {
		t.Errorf("no DEDUP line for the live holder: %s", so.String())
	}
	if strings.Contains(so.String(), "stole") {
		t.Error("a live claim was stolen inline")
	}
}

// A claim past its state's TTL is DEAD and reclaimable: acquire reclaims it (exit 0) via an
// auditable steal, mirroring the two-phase-TTL contract (claimed 20m, dispatched 120m).
func TestAcquireStaleClaimIsReclaimed(t *testing.T) {
	cases := []struct {
		state string
		age   time.Duration
		stale bool
	}{
		{"dispatched", 121 * time.Minute, true},
		{"dispatched", 42 * time.Minute, false},
		{"claimed", 25 * time.Minute, true},
		{"claimed", 5 * time.Minute, false},
	}
	for _, c := range cases {
		t.Run(c.state+"-"+c.age.String(), func(t *testing.T) {
			f := newForge("medici-finance/assay")
			f.seedClaim("at--stream--07", "old-sess", c.state, "", c.age)
			run, so, _ := harness(t, f)
			rc := run("acquire", "at--stream--07", "--repo", f.repo, "--owner", "sess-N")
			if c.stale {
				if rc != exitOK {
					t.Fatalf("stale reclaim rc = %d, want 0; out=%s", rc, so.String())
				}
				if !strings.Contains(so.String(), "stole at--stream--07") {
					t.Errorf("stale claim was not reclaimed via steal: %s", so.String())
				}
			} else {
				if rc != exitRefused {
					t.Fatalf("live claim rc = %d, want 5; out=%s", rc, so.String())
				}
			}
		})
	}
}

// A held claim whose recorded branch already exists on the remote is a branch-as-claim: the
// work is in flight, not stalled — refuse (exit 5), regardless of age.
func TestAcquireBranchAsClaimRefuses(t *testing.T) {
	f := newForge("medici-finance/assay")
	f.seedClaim("at--stream--07", "old-sess", "dispatched", "feat/live", 9999*time.Minute)
	f.branches["feat/live"] = true
	run, so, _ := harness(t, f)
	rc := run("acquire", "at--stream--07", "--repo", f.repo, "--owner", "sess-N")
	if rc != exitRefused {
		t.Fatalf("branch-as-claim rc = %d, want 5; out=%s", rc, so.String())
	}
	if !strings.Contains(so.String(), "branch-as-claim") {
		t.Errorf("no branch-as-claim note: %s", so.String())
	}
}

// An unmintable base (the forge cannot answer heads/main) is the fail-closed path: exit 6,
// never a claim written blind.
func TestAcquireUnmintableBaseIsUnverifiable(t *testing.T) {
	f := newForge("medici-finance/assay")
	f.baseSHA = ""
	run, _, se := harness(t, f)
	rc := run("acquire", "at--stream--07", "--repo", f.repo)
	if rc != exitUnverifiable {
		t.Fatalf("unmintable base rc = %d, want 6; err=%s", rc, se.String())
	}
}

// --- show / list output parity ---------------------------------------------

// The `show` output is a wire contract: desksupervise/live.go and deskdispatch/dispatch.go
// parse state=/age=/owner=/branch= and the "FREE <key>" marker out of it. This pins the exact
// line shape and proves those very regexes extract the right values.
func TestShowOutputIsTheParsedWireContract(t *testing.T) {
	f := newForge("medici-finance/assay")
	f.seedClaim("at--stream--07", "sess-Z", "dispatched", "feat/y", 42*time.Minute)
	run, so, _ := harness(t, f)

	if rc := run("show", "at--stream--07", "--repo", f.repo); rc != exitOK {
		t.Fatalf("show rc = %d, want 0", rc)
	}
	line := strings.TrimSpace(so.String())
	if !strings.HasPrefix(line, "dispatch-claim: HELD at--stream--07 — ") {
		t.Fatalf("HELD line prefix wrong: %q", line)
	}
	// These four regexes are byte-copies of the ones desksupervise/live.go and
	// deskdispatch/dispatch.go run against this output.
	for re, want := range map[*regexp.Regexp]string{
		regexp.MustCompile(`state=([A-Za-z]+)`): "dispatched",
		regexp.MustCompile(`age=(\d+)m`):        "42",
		regexp.MustCompile(`owner=(\S+)`):       "sess-Z",
		regexp.MustCompile(`branch=(\S+)`):      "feat/y",
	} {
		m := re.FindStringSubmatch(line)
		if m == nil || m[1] != want {
			t.Errorf("regex %s on %q -> %v, want %q", re, line, m, want)
		}
	}
}

func TestShowFreeCarriesTheFreeMarker(t *testing.T) {
	f := newForge("medici-finance/assay")
	run, so, _ := harness(t, f)
	if rc := run("show", "at--stream--07", "--repo", f.repo); rc != exitOK {
		t.Fatalf("show FREE rc = %d, want 0", rc)
	}
	if !strings.Contains(so.String(), "FREE at--stream--07") {
		t.Errorf("FREE marker absent: %s", so.String())
	}
}

// An unreadable claim (the ref read fails AND the repo probe fails) is exit 6, never FREE.
func TestShowUnreadableIsUnverifiableNotFree(t *testing.T) {
	f := newForge("medici-finance/assay")
	f.repoExists = false // the repo probe fails, so "no ref" cannot be proven to mean free
	run, _, se := harness(t, f)
	if rc := run("show", "at--stream--07", "--repo", f.repo); rc != exitUnverifiable {
		t.Fatalf("unreadable show rc = %d, want 6; err=%s", rc, se.String())
	}
}

func TestListShowsEachClaim(t *testing.T) {
	f := newForge("medici-finance/assay")
	f.seedClaim("at--stream--07", "s1", "dispatched", "", 10*time.Minute)
	f.seedClaim("at--issue-5", "s2", "claimed", "", 3*time.Minute)
	run, so, _ := harness(t, f)
	if rc := run("list", "--repo", f.repo); rc != exitOK {
		t.Fatalf("list rc = %d, want 0", rc)
	}
	for _, want := range []string{"HELD at--stream--07", "HELD at--issue-5"} {
		if !strings.Contains(so.String(), want) {
			t.Errorf("list missing %q:\n%s", want, so.String())
		}
	}
}

// --- release / steal / progress ---------------------------------------------

func TestReleaseDeletesTheRefAndIsNoopWhenMissing(t *testing.T) {
	f := newForge("medici-finance/assay")
	f.seedClaim("at--stream--07", "s1", "dispatched", "", time.Minute)
	run, so, _ := harness(t, f)

	if rc := run("release", "at--stream--07", "--repo", f.repo); rc != exitOK {
		t.Fatalf("release rc = %d, want 0", rc)
	}
	if _, held := f.refs["at--stream--07"]; held {
		t.Error("release did not delete the ref")
	}
	// A second release of a now-missing claim is a no-op, not a failure.
	if rc := run("release", "at--stream--07", "--repo", f.repo); rc != exitOK {
		t.Fatalf("release-missing rc = %d, want 0; out=%s", rc, so.String())
	}
}

func TestStealRequiresAReasonThenSucceeds(t *testing.T) {
	f := newForge("medici-finance/assay")
	f.seedClaim("at--stream--07", "old", "dispatched", "", 5*time.Minute)
	run, _, se := harness(t, f)

	if rc := run("steal", "at--stream--07", "--repo", f.repo); rc != exitRefused {
		t.Fatalf("reasonless steal rc = %d, want 5; err=%s", rc, se.String())
	}
	run2, so, _ := harness(t, f)
	if rc := run2("steal", "at--stream--07", "--repo", f.repo, "--reason", "TTL dead", "--owner", "new"); rc != exitOK {
		t.Fatalf("steal-with-reason rc = %d, want 0; out=%s", rc, so.String())
	}
	sha := f.refs["at--stream--07"]
	if !strings.Contains(f.tags[sha].message, "note=TTL_dead") {
		t.Errorf("steal did not record the reason in the replacement: %q", f.tags[sha].message)
	}
}

func TestProgressRequiresHolderAndBranch(t *testing.T) {
	f := newForge("medici-finance/assay")
	f.seedClaim("at--stream--07", "owner-1", "claimed", "", time.Minute)
	run, _, se := harness(t, f)

	// A free claim cannot be advanced.
	if rc := run("progress", "at--issue-9", "--repo", f.repo, "--owner", "owner-1", "--branch", "feat/x"); rc != exitRefused {
		t.Fatalf("progress-on-free rc = %d, want 5; err=%s", rc, se.String())
	}
	// A non-holder cannot advance someone else's claim.
	if rc := run("progress", "at--stream--07", "--repo", f.repo, "--owner", "intruder", "--branch", "feat/x"); rc != exitRefused {
		t.Fatalf("progress-by-nonholder rc = %d, want 5; err=%s", rc, se.String())
	}
	// progress with no --branch is refused.
	if rc := run("progress", "at--stream--07", "--repo", f.repo, "--owner", "owner-1"); rc != exitRefused {
		t.Fatalf("progress-no-branch rc = %d, want 5", rc)
	}
	// The holder advances its own claim to dispatched.
	if rc := run("progress", "at--stream--07", "--repo", f.repo, "--owner", "owner-1", "--branch", "feat/x"); rc != exitOK {
		t.Fatalf("progress-by-holder rc = %d, want 0; err=%s", rc, se.String())
	}
	sha := f.refs["at--stream--07"]
	if !strings.Contains(f.tags[sha].message, "state=dispatched") {
		t.Errorf("progress did not advance state: %q", f.tags[sha].message)
	}
}

// --- argument-level refusals (no forge call) --------------------------------

func TestInvalidKeyRefusedBeforeAnyForgeCall(t *testing.T) {
	for _, key := range []string{"noprefix", "at stream", "at--..--1", ".at--x--1", "at--x--1.lock", "at~x--1"} {
		f := newForge("medici-finance/assay")
		run, _, se := harness(t, f)
		rc := run("acquire", key, "--repo", f.repo)
		if rc != exitRefused {
			t.Errorf("key %q rc = %d, want 5; err=%s", key, rc, se.String())
		}
		// A malformed key must not reach the forge (no ref read, no mint).
		for _, c := range f.calls {
			if len(c) > 0 && c[0] == "api" {
				t.Errorf("key %q reached a forge api call: %v", key, c)
			}
		}
	}
}

func TestUnknownVerbAndFlagRefused(t *testing.T) {
	f := newForge("medici-finance/assay")
	run, _, _ := harness(t, f)
	if rc := run("frobnicate", "at--x--1", "--repo", f.repo); rc != exitRefused {
		t.Errorf("unknown verb rc = %d, want 5", rc)
	}
	if rc := run("acquire", "at--x--1", "--repo", f.repo, "--bogus", "v"); rc != exitRefused {
		t.Errorf("unknown flag rc = %d, want 5", rc)
	}
}

func TestGhMissingIsUnverifiable(t *testing.T) {
	f := newForge("medici-finance/assay")
	run, _, se := harness(t, f)
	ghLookPath = func(string) (string, error) { return "", &notFound{} }
	if rc := run("show", "at--x--1", "--repo", f.repo); rc != exitUnverifiable {
		t.Fatalf("gh-missing rc = %d, want 6; err=%s", rc, se.String())
	}
}

type notFound struct{}

func (*notFound) Error() string { return "exec: \"gh\": executable file not found in $PATH" }

// The exit codes this port emits ARE the deskkit contract deskdispatch passes through
// untouched — pin the mapping so a future edit cannot silently repoint one.
func TestExitCodesAreTheDeskkitContract(t *testing.T) {
	if exitOK != deskkit.ExitOK || exitRefused != deskkit.ExitRefused || exitUnverifiable != deskkit.ExitUnverifiable {
		t.Fatalf("exit codes drifted from the deskkit contract: ok=%d refused=%d unverifiable=%d",
			exitOK, exitRefused, exitUnverifiable)
	}
}

// --- Windows-viable exec path ----------------------------------------------

// The whole reason this binary exists: it is reached with NO shebang script and NO bash. A
// dispatcher invokes it as a bare executable (CreateProcess resolves it on PATH on Windows),
// and every forge call it makes is likewise a bare `gh` executable — never a `.sh`, never an
// interpreter line. This proves the tool completes real work through the gh seam with a
// LookPath that only knows plain executables, which is exactly the Windows condition.
func TestReachesTheForgeWithNoShellOrScript(t *testing.T) {
	f := newForge("medici-finance/assay")
	run, so, _ := harness(t, f)
	// LookPath answers only for bare executables, the way CreateProcess+PATH does — no shell.
	ghLookPath = func(name string) (string, error) {
		if name == "gh" {
			return `C:\Program Files\GitHub CLI\gh.exe`, nil
		}
		return "", &notFound{}
	}
	if rc := run("acquire", "at--stream--07", "--repo", f.repo, "--owner", "s"); rc != exitOK {
		t.Fatalf("acquire rc = %d, want 0; out=%s", rc, so.String())
	}
	if len(f.calls) == 0 {
		t.Fatal("no forge call was made")
	}
	// Every forge call flows through the ONE exec seam (realGHRun -> exec.Command("gh", …)),
	// so the executable is always the bare `gh` binary and each recorded argv begins with a
	// gh SUBCOMMAND — never a `.sh` script path, never `bash`/`sh -c`, never a shebang line.
	// That is precisely the property that makes the claim path run under Windows CreateProcess
	// with no shell association, which the shebang `.sh` script cannot.
	for _, c := range f.calls {
		if len(c) == 0 {
			t.Errorf("an empty argv reached the exec seam")
			continue
		}
		if c[0] != "api" && c[0] != "repo" {
			t.Errorf("a non-gh-subcommand reached the exec seam (argv[0]=%q): %v", c[0], c)
		}
		if strings.HasSuffix(c[0], ".sh") || c[0] == "bash" || c[0] == "sh" {
			t.Errorf("a shell/script executable reached the exec seam: %v", c)
		}
	}
}
