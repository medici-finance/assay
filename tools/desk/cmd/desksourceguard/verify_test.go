package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// ---------------------------------------------------------------------------
// End-to-end: the #519 reproduction, run as a test.
//
// A real git origin, a real annotated release tag, and the SAME clone shape the
// consumer install action performs (`git clone --depth 1 --branch <tag>`). The
// tag is then repointed at the origin with no consumer commit and no pin edit —
// the exact manoeuvre the issue describes — and the guard must go from exit 0 to
// a refusal. Both directions are asserted in one test so neither can be shown
// without the other.
// ---------------------------------------------------------------------------

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// writePins writes a consumer .assay-versions carrying both halves of the pin.
func writePins(t *testing.T, root, tag, binSHA256, sourceCommit string) {
	t.Helper()
	body := fmt.Sprintf(""+
		"desk-tools-linux-amd64  %s %s  # CI runners\n"+
		"desk-tools-source       %s %s  # commit the binaries were built from\n",
		tag, binSHA256, tag, sourceCommit)
	if err := os.WriteFile(filepath.Join(root, ".assay-versions"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

const fakeBinDigest = "3da575a37bc4363dbccf30696424ff5d86ae91073cd8aa0596e99b1352aa9362"

// TestRepointedTagIsRefused_EndToEnd is the load-bearing test: GREEN on the
// reviewed commit, RED after the tag moves, with nothing else changed.
func TestRepointedTagIsRefused_EndToEnd(t *testing.T) {
	work := t.TempDir()
	origin := filepath.Join(work, "origin")
	if err := os.MkdirAll(filepath.Join(origin, "tools", "desk"), 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, origin, "init", "-q", "-b", "main", ".")

	gate := filepath.Join(origin, "tools", "desk", "gate.go")
	if err := os.WriteFile(gate, []byte("package main // REVIEWED\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, origin, "add", "-A")
	git(t, origin, "commit", "-qm", "reviewed")
	reviewed := git(t, origin, "rev-parse", "HEAD")
	const tag = "desk-tools/v0.9.9"
	git(t, origin, "tag", "-a", tag, "-m", "release")

	if err := os.WriteFile(gate, []byte("package main // SUBSTITUTED\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, origin, "add", "-A")
	git(t, origin, "commit", "-qm", "attacker")
	attacker := git(t, origin, "rev-parse", "HEAD")
	if reviewed == attacker {
		t.Fatal("fixture is broken: the two commits are identical")
	}

	// The consumer tree: pins reviewed, binaries stamped with reviewed's short SHA.
	consumer := filepath.Join(work, "consumer")
	if err := os.MkdirAll(consumer, 0o755); err != nil {
		t.Fatal(err)
	}
	writePins(t, consumer, tag, fakeBinDigest, reviewed)
	stamp := reviewed[:7]

	materialise := func(dest string) {
		t.Helper()
		git(t, work, "clone", "-q", "--depth", "1", "--branch", tag, "file://"+origin, dest)
	}

	opts := func(src string) options {
		return options{repoRoot: consumer, source: src, platform: "linux-amd64"}
	}

	// --- GREEN: install at review time -------------------------------------
	green := filepath.Join(work, "run-green")
	materialise(green)
	var out strings.Builder
	if err := verify(opts(green), stamp, &out); err != nil {
		t.Fatalf("expected the pinned tree to verify, got: %v", err)
	}
	if !strings.Contains(out.String(), reviewed) {
		t.Fatalf("success output should name the verified commit, got:\n%s", out.String())
	}

	// --- the manoeuvre: repoint the tag, change nothing in the consumer ----
	git(t, origin, "tag", "-f", "-a", tag, "-m", "release", attacker)

	// --- RED: the same consumer tree installs again ------------------------
	red := filepath.Join(work, "run-red")
	materialise(red)
	if got := git(t, red, "rev-parse", "HEAD"); got != attacker {
		t.Fatalf("fixture is broken: the repointed clone is at %s, want %s", got, attacker)
	}
	out.Reset()
	err := verify(opts(red), stamp, &out)
	if err == nil {
		t.Fatal("REPOINTED TAG WAS ACCEPTED — the guard does not guard")
	}
	if !deskkit.IsRefused(err) {
		t.Fatalf("want ExitRefused (5) for a repointed tag, got %d: %v", deskkit.ExitCodeOf(err), err)
	}
	for _, want := range []string{"SOURCE PIN MISMATCH", attacker, reviewed} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal must name %q so the operator can tell repoint from stale pin; got:\n%v", want, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Every refusal is load-bearing: one case per check, each differing from the
// green baseline in exactly one respect.
// ---------------------------------------------------------------------------

// fixture builds a consumer + a non-git "source" directory whose HEAD is faked
// through the execCommand seam, so the pin-parsing and binding checks are tested
// without a git world each time.
type fixture struct {
	root  string
	src   string
	stamp string
}

func newFixture(t *testing.T, pinFileBody, headSHA, stamp string) fixture {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".assay-versions"), []byte(pinFileBody), 0o644); err != nil {
		t.Fatal(err)
	}
	src := t.TempDir()
	old := execCommand
	t.Cleanup(func() { execCommand = old })
	execCommand = func(name string, arg ...string) *exec.Cmd {
		if headSHA == "" { // simulate "not a git repository"
			return exec.Command("false")
		}
		return exec.Command("printf", "%s\n", headSHA)
	}
	return fixture{root: root, src: src, stamp: stamp}
}

const goodCommit = "135feb2a9926afe2a57156660af78f1f6eee2d63"

func pinBody(binTag, sourceTag, sourceDigest string) string {
	return fmt.Sprintf("desk-tools-linux-amd64 %s %s\ndesk-tools-source %s %s\n",
		binTag, fakeBinDigest, sourceTag, sourceDigest)
}

func TestEachRefusalIsLoadBearing(t *testing.T) {
	const tag = "desk-tools/v0.2.2"

	cases := []struct {
		name     string
		pins     string
		head     string
		stamp    string
		wantCode int
		wantMsg  string
	}{
		{
			name:     "baseline agrees",
			pins:     pinBody(tag, tag, goodCommit),
			head:     goodCommit,
			stamp:    goodCommit[:7],
			wantCode: deskkit.ExitOK,
		},
		{
			name:     "no source pin at all — the pre-#519 state",
			pins:     fmt.Sprintf("desk-tools-linux-amd64 %s %s\n", tag, fakeBinDigest),
			head:     goodCommit,
			stamp:    goodCommit[:7],
			wantCode: deskkit.ExitUnverifiable,
			wantMsg:  "no desk-tools-source pin",
		},
		{
			name:     "no binary pin for this platform",
			pins:     fmt.Sprintf("desk-tools-source %s %s\n", tag, goodCommit),
			head:     goodCommit,
			stamp:    goodCommit[:7],
			wantCode: deskkit.ExitUnverifiable,
			wantMsg:  "no desk-tools-linux-amd64 pin",
		},
		{
			name:     "source pin names a different tag from the binaries",
			pins:     pinBody(tag, "desk-tools/v0.2.1", goodCommit),
			head:     goodCommit,
			stamp:    goodCommit[:7],
			wantCode: deskkit.ExitRefused,
			wantMsg:  "pin split",
		},
		{
			name:     "abbreviated commit pin is not an anchor",
			pins:     pinBody(tag, tag, goodCommit[:12]),
			head:     goodCommit,
			stamp:    goodCommit[:7],
			wantCode: deskkit.ExitUnverifiable,
			wantMsg:  "not a full 40-hex commit SHA",
		},
		{
			name:     "non-hex commit pin",
			pins:     pinBody(tag, tag, "HEAD"),
			head:     goodCommit,
			stamp:    goodCommit[:7],
			wantCode: deskkit.ExitUnverifiable,
			wantMsg:  "not a full 40-hex commit SHA",
		},
		{
			name:     "malformed pin line (two fields)",
			pins:     fmt.Sprintf("desk-tools-linux-amd64 %s %s\ndesk-tools-source %s\n", tag, fakeBinDigest, tag),
			head:     goodCommit,
			stamp:    goodCommit[:7],
			wantCode: deskkit.ExitUnverifiable,
			wantMsg:  "malformed",
		},
		{
			name:     "unstamped binary cannot bind the pin",
			pins:     pinBody(tag, tag, goodCommit),
			head:     goodCommit,
			stamp:    "",
			wantCode: deskkit.ExitUnverifiable,
			wantMsg:  "no SourceSHA stamp",
		},
		{
			name:     "stamp too short to bind",
			pins:     pinBody(tag, tag, goodCommit),
			head:     goodCommit,
			stamp:    "135f",
			wantCode: deskkit.ExitUnverifiable,
			wantMsg:  "too weak to bind",
		},
		{
			name:     "source pin is not the commit the binaries were built from",
			pins:     pinBody(tag, tag, "0000000000000000000000000000000000000000"),
			head:     "0000000000000000000000000000000000000000",
			stamp:    goodCommit[:7],
			wantCode: deskkit.ExitRefused,
			wantMsg:  "binary/source pin disagreement",
		},
		{
			name:     "materialised tree is a different commit (repointed tag)",
			pins:     pinBody(tag, tag, goodCommit),
			head:     "1e8177b00b24d5c50009e7fa3eaa0be45c4fbe7b",
			stamp:    goodCommit[:7],
			wantCode: deskkit.ExitRefused,
			wantMsg:  "SOURCE PIN MISMATCH",
		},
		{
			name:     "HEAD unresolvable — fail closed, never assume",
			pins:     pinBody(tag, tag, goodCommit),
			head:     "",
			stamp:    goodCommit[:7],
			wantCode: deskkit.ExitUnverifiable,
			wantMsg:  "cannot resolve HEAD",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, tc.pins, tc.head, tc.stamp)
			var out strings.Builder
			err := verify(options{repoRoot: f.root, source: f.src, platform: "linux-amd64"}, f.stamp, &out)
			if got := deskkit.ExitCodeOf(err); got != tc.wantCode {
				t.Fatalf("exit code %d, want %d (err: %v)", got, tc.wantCode, err)
			}
			if tc.wantMsg == "" {
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("refusal must explain itself with %q; got: %v", tc.wantMsg, err)
			}
		})
	}
}

// TestPrefixMatchIsExact guards the trailing-space selection: a longer artifact
// name must never satisfy the source pin lookup, or a stray line could stand in
// for the real one.
func TestPrefixMatchIsExact(t *testing.T) {
	body := fmt.Sprintf("desk-tools-linux-amd64 t %s\ndesk-tools-source-notes t %s\n", fakeBinDigest, goodCommit)
	f := newFixture(t, body, goodCommit, goodCommit[:7])
	var out strings.Builder
	err := verify(options{repoRoot: f.root, source: f.src, platform: "linux-amd64"}, f.stamp, &out)
	if !deskkit.IsUnverifiable(err) {
		t.Fatalf("desk-tools-source-notes must not satisfy the desk-tools-source pin; got %v", err)
	}
}

// TestMissingPinFileFailsClosed — a consumer with no pin file at all is not
// "unpinned by default", it is unverifiable.
func TestMissingPinFileFailsClosed(t *testing.T) {
	var out strings.Builder
	err := verify(options{repoRoot: t.TempDir(), source: t.TempDir(), platform: "linux-amd64"}, "abc1234", &out)
	if !deskkit.IsUnverifiable(err) {
		t.Fatalf("want ExitUnverifiable for a missing .assay-versions, got %v", err)
	}
}

// TestRunRequiresSource — the CLI refuses rather than verifying nothing.
func TestRunRequiresSource(t *testing.T) {
	var stdout, stderr strings.Builder
	if got := run([]string{"--repo-root", t.TempDir()}, &stdout, &stderr); got != deskkit.ExitUnverifiable {
		t.Fatalf("exit %d, want %d", got, deskkit.ExitUnverifiable)
	}
	if !strings.Contains(stderr.String(), "--source is required") {
		t.Fatalf("stderr should say why; got %q", stderr.String())
	}
}

// TestVersionFlag — every desk tool answers --version; the release stamp is what
// this guard binds against, so its own report must work.
func TestVersionFlag(t *testing.T) {
	var stdout, stderr strings.Builder
	if got := run([]string{"--version"}, &stdout, &stderr); got != deskkit.ExitOK {
		t.Fatalf("exit %d, want 0", got)
	}
	if !strings.Contains(stdout.String(), "desksourceguard sourceSHA=") {
		t.Fatalf("unexpected --version output: %q", stdout.String())
	}
}

// TestDefaultPlatform — the artifact suffix is derived from the running binary so
// an unexpected runner refuses instead of reading another platform's pin.
func TestDefaultPlatform(t *testing.T) {
	p, err := defaultPlatform()
	if err != nil {
		t.Fatalf("this platform should be supported: %v", err)
	}
	if p != "linux-amd64" && p != "darwin-arm64" && p != "darwin-amd64" {
		t.Fatalf("derived platform %q is not one desk-tools publishes", p)
	}
}
