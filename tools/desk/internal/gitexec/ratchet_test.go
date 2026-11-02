package gitexec

// ratchet_test.go — the CI enforcement of scripts/count-git-exec.sh (the git-exec
// ratchet) and of the go-git CVE floor. It lives in go test, not in a workflow step,
// so every build-test run of tools/desk enforces both gates.

import (
	"archive/tar"
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// deskRoot is tools/desk, relative to this package directory.
const deskRoot = "../.."

// baseRef is the PR base the ratchet re-measures against. Spelled in full: a bare
// origin/main resolves to a stray local branch of that name where one exists.
const baseRef = "refs/remotes/origin/main"

var countLine = regexp.MustCompile(`git-exec sites: (\d+) \(direct spawns: (\d+), seam call sites: (\d+)\)`)

// needSh returns the sh the counter runs under. On Linux and macOS a missing sh fails:
// the gate must not read as green because its interpreter is absent. The Windows leg
// skips; the Linux legs of the same run enforce the gate.
func needSh(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("could-not-check: the POSIX counter is enforced on the Linux legs, not Windows")
	}
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Fatalf("could-not-check: no sh on PATH to run the git-exec counter: %v", err)
	}
	return sh
}

// runCounter runs the counter script at script (cwd-independent: it cds to its own
// tools/desk) and returns its combined output and exit code.
func runCounter(t *testing.T, sh, script string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(sh, append([]string{script}, args...)...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return string(out), ee.ExitCode()
	}
	t.Fatalf("could not run %s: %v", script, err)
	return "", -1
}

func parseCount(t *testing.T, out string) int {
	t.Helper()
	m := countLine.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("counter printed no count line: %q", out)
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

// TestRatchetHoldsOnTree is the gate itself: the real counter on this tree, against the
// checked-in pin. Growth above the pin fails the build.
func TestRatchetHoldsOnTree(t *testing.T) {
	sh := needSh(t)
	out, code := runCounter(t, sh, filepath.Join(deskRoot, "scripts", "count-git-exec.sh"))
	if code != 0 {
		t.Fatalf("git-exec ratchet is red (exit %d):\n%s", code, out)
	}
	parseCount(t, out)
	t.Logf("%s", strings.TrimSpace(out))
}

// TestRatchetNotAboveBase re-measures the PR's base tree with THIS tree's counter, so
// base and head are counted by identical patterns, and fails if the head count exceeds
// the base count. A pin raised in the same change cannot admit a new site past it.
func TestRatchetNotAboveBase(t *testing.T) {
	sh := needSh(t)
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skipf("could-not-check: no git binary to read the base tree: %v", err)
	}
	repoRoot, err := filepath.Abs(filepath.Join(deskRoot, "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	gitOutput := func(args ...string) (string, error) {
		cmd := exec.Command(git, append([]string{"-C", repoRoot}, args...)...)
		out, err := cmd.Output()
		return strings.TrimSpace(string(out)), err
	}
	if _, err := gitOutput("rev-parse", "--verify", "--quiet", baseRef+"^{commit}"); err != nil {
		t.Skipf("could-not-check: %s does not resolve here (shallow or detached checkout)", baseRef)
	}
	mb, err := gitOutput("merge-base", "HEAD", baseRef)
	if err != nil || mb == "" {
		t.Skipf("could-not-check: no merge base between HEAD and %s: %v", baseRef, err)
	}

	base := t.TempDir()
	baseDesk := filepath.Join(base, "tools", "desk")
	if err := os.MkdirAll(filepath.Join(baseDesk, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	extractGoFiles(t, git, repoRoot, mb, base, "tools/desk/cmd", "tools/desk/internal")
	script, err := os.ReadFile(filepath.Join(deskRoot, "scripts", "count-git-exec.sh"))
	if err != nil {
		t.Fatal(err)
	}
	baseScript := filepath.Join(baseDesk, "scripts", "count-git-exec.sh")
	if err := os.WriteFile(baseScript, script, 0o644); err != nil {
		t.Fatal(err)
	}

	baseOut, code := runCounter(t, sh, baseScript, "--count")
	if code != 0 {
		t.Fatalf("re-measuring the base tree failed (exit %d): %s", code, baseOut)
	}
	headOut, code := runCounter(t, sh, filepath.Join(deskRoot, "scripts", "count-git-exec.sh"), "--count")
	if code != 0 {
		t.Fatalf("measuring the head tree failed (exit %d): %s", code, headOut)
	}
	baseN, headN := parseCount(t, baseOut), parseCount(t, headOut)
	if headN > baseN {
		t.Fatalf("git-exec sites grew against the PR base %.12s: head %d > base %d (+%d)\nbase: %s\nhead: %s",
			mb, headN, baseN, headN-baseN, strings.TrimSpace(baseOut), strings.TrimSpace(headOut))
	}
	t.Logf("base %.12s: %d, head: %d", mb, baseN, headN)
}

// extractGoFiles writes the non-test-relevant .go files under paths at rev into dst,
// read from git archive's tar stream (no tar binary needed).
func extractGoFiles(t *testing.T, git, repoRoot, rev, dst string, paths ...string) {
	t.Helper()
	cmd := exec.Command(git, append([]string{"-C", repoRoot, "archive", "--format=tar", rev, "--"}, paths...)...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(stdout)
	files := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("reading the base archive: %v", err)
		}
		if hdr.Typeflag != tar.TypeReg || !strings.HasSuffix(hdr.Name, ".go") {
			continue
		}
		clean := filepath.Clean(filepath.FromSlash(hdr.Name))
		if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
			t.Fatalf("unsafe path in the base archive: %q", hdr.Name)
		}
		p := filepath.Join(dst, clean)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(p)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		files++
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("git archive %.12s failed: %v: %s", rev, err, stderr.String())
	}
	if files == 0 {
		t.Fatalf("the base archive at %.12s held no .go files under %v", rev, paths)
	}
}

// fixture is a miniature tools/desk: the real counter script, a pin, and Go files.
type fixture struct {
	t      *testing.T
	sh     string
	root   string
	script string
}

func newFixture(t *testing.T, pin string) *fixture {
	t.Helper()
	sh := needSh(t)
	root := t.TempDir()
	body, err := os.ReadFile(filepath.Join(deskRoot, "scripts", "count-git-exec.sh"))
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, sh: sh, root: root, script: filepath.Join(root, "scripts", "count-git-exec.sh")}
	f.write("scripts/count-git-exec.sh", string(body))
	if pin != "" {
		f.write("scripts/git-exec-pin.txt", pin)
	}
	// Two counted sites: one direct spawn, one seam call.
	f.write("cmd/tool/main.go", "package main\n\nimport \"os/exec\"\n\nfunc a() { _ = exec.Command(\"git\", \"status\") }\n")
	f.write("internal/lib/lib.go", "package lib\n\nfunc b() { runGit(\"status\") }\n")
	return f
}

func (f *fixture) write(rel, body string) {
	f.t.Helper()
	p := filepath.Join(f.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) run(args ...string) (int, string, int) {
	f.t.Helper()
	out, code := runCounter(f.t, f.sh, f.script, args...)
	n := -1
	if m := countLine.FindStringSubmatch(out); m != nil {
		n, _ = strconv.Atoi(m[1])
	}
	return n, out, code
}

func TestRatchetGreenAtPin(t *testing.T) {
	f := newFixture(t, "# pin\n2\n")
	if n, out, code := f.run(); n != 2 || code != 0 {
		t.Fatalf("at the pin: want count 2 exit 0, got %d exit %d: %s", n, code, out)
	}
}

// TestRatchetRedOnGrowth is the brief's mutation row: one new site above the pin is red.
// regression: F-git-exec-ratchet
func TestRatchetRedOnGrowth(t *testing.T) {
	for name, src := range map[string]string{
		"direct":  "package x\n\nimport \"os/exec\"\n\nfunc c() { _ = exec.Command(\"git\", \"log\") }\n",
		"context": "package x\n\nimport (\n\t\"context\"\n\t\"os/exec\"\n)\n\nfunc c(ctx context.Context) { _ = exec.CommandContext(ctx, \"git\", \"log\") }\n",
		"seam":    "package x\n\nfunc c() { gitOut(\"log\") }\n",
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, "2\n")
			f.write("cmd/deskboard/stray.go", src)
			n, out, code := f.run()
			if n != 3 || code != 1 || !strings.Contains(out, "FAIL: git-exec sites grew: 3 > pinned 2") {
				t.Fatalf("one site above the pin: want count 3 exit 1 FAIL, got %d exit %d: %s", n, code, out)
			}
		})
	}
}

// TestRatchetExemptsGitexecOnly: the exclusion was unanchored and never matched grep's
// relative paths, so the audited home's own spawn was counted.
// regression: F-gitexec-exempt-unanchored
func TestRatchetExemptsGitexecOnly(t *testing.T) {
	f := newFixture(t, "2\n")
	// The audited home: neither its spawn nor a seam-named call inside it is counted.
	f.write("internal/gitexec/gitexec.go", "package gitexec\n\nimport \"os/exec\"\n\nfunc r() { _ = exec.Command(\"git\", \"merge\"); execGit() }\n")
	if n, out, code := f.run(); n != 2 || code != 0 {
		t.Fatalf("internal/gitexec must be exempt: want count 2 exit 0, got %d exit %d: %s", n, code, out)
	}
	// Only the top-level home is exempt: a nested package of the same name is counted.
	f.write("cmd/other/internal/gitexec/x.go", "package gitexec\n\nimport \"os/exec\"\n\nfunc r() { _ = exec.Command(\"git\", \"log\") }\n")
	if n, out, code := f.run(); n != 3 || code != 1 {
		t.Fatalf("a nested gitexec package is not the audited home: want count 3 exit 1, got %d exit %d: %s", n, code, out)
	}
}

func TestRatchetSkipsTestFiles(t *testing.T) {
	f := newFixture(t, "2\n")
	f.write("cmd/tool/main_test.go", "package main\n\nimport \"os/exec\"\n\nfunc t2() { _ = exec.Command(\"git\", \"init\"); runGit() }\n")
	if n, out, code := f.run(); n != 2 || code != 0 {
		t.Fatalf("_test.go files are not counted: want count 2 exit 0, got %d exit %d: %s", n, code, out)
	}
}

func TestRatchetNoteBelowPin(t *testing.T) {
	f := newFixture(t, "5\r\n") // a CRLF pin still parses
	n, out, code := f.run()
	if n != 2 || code != 0 || !strings.Contains(out, "NOTE: git-exec sites fell below the pin: 2 < 5") {
		t.Fatalf("below the pin: want count 2 exit 0 with a NOTE, got %d exit %d: %s", n, code, out)
	}
}

// TestRatchetPinFailsClosed: a missing or garbled pin fails closed, never reads as a pass.
// regression: F-git-exec-ratchet
func TestRatchetPinFailsClosed(t *testing.T) {
	for name, pin := range map[string]string{
		"missing": "",
		"garbled": "2x\n",
		"empty":   "# only a comment\n",
		"two":     "2\n3\n",
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, pin)
			if pin == "" {
				// newFixture wrote no pin file at all.
				if _, err := os.Stat(filepath.Join(f.root, "scripts", "git-exec-pin.txt")); !os.IsNotExist(err) {
					t.Fatalf("fixture unexpectedly has a pin file: %v", err)
				}
			}
			if _, out, code := f.run(); code != 2 || !strings.Contains(out, "could-not-check") {
				t.Fatalf("pin %q: want exit 2 could-not-check, got exit %d: %s", pin, code, out)
			}
		})
	}
}

func TestRatchetCountModeNeedsNoPin(t *testing.T) {
	f := newFixture(t, "")
	if n, out, code := f.run("--count"); n != 2 || code != 0 {
		t.Fatalf("--count: want count 2 exit 0 without a pin, got %d exit %d: %s", n, code, out)
	}
	if _, out, code := f.run("--bogus"); code != 2 {
		t.Fatalf("unknown argument: want exit 2, got %d: %s", code, out)
	}
}

// goGitFloor is the lowest go-git v5 release carrying the fixes for CVE-2025-21613
// (argument injection) and CVE-2025-21614 (denial of service).
var goGitFloor = [3]int{5, 13, 0}

var goGitRequire = regexp.MustCompile(`^\s*(?:require\s+)?github\.com/go-git/go-git/v5\s+v(\d+)\.(\d+)\.(\d+)(\S*)`)

// goGitFloorHeld reads a go.mod body and reports the go-git/v5 version it requires and
// whether that version is at or above the floor. A replace of go-git, or no go-git
// requirement at all, is an error: the floor cannot be read from this file then.
func goGitFloorHeld(gomod string) (string, bool, error) {
	var found string
	held := false
	sc := bufio.NewScanner(strings.NewReader(gomod))
	for sc.Scan() {
		line := sc.Text()
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "github.com/go-git/go-git/v5") && strings.Contains(trimmed, "=>") {
			return "", false, fmt.Errorf("go-git is replaced (%q); the pinned version is not what builds", trimmed)
		}
		m := goGitRequire.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if found != "" {
			return "", false, fmt.Errorf("go-git/v5 required twice")
		}
		var v [3]int
		for i := range v {
			v[i], _ = strconv.Atoi(m[i+1])
		}
		found = fmt.Sprintf("v%d.%d.%d%s", v[0], v[1], v[2], m[4])
		held = v[0] > goGitFloor[0] ||
			(v[0] == goGitFloor[0] && (v[1] > goGitFloor[1] || (v[1] == goGitFloor[1] && v[2] >= goGitFloor[2])))
		// Semver orders a prerelease below its release, so v5.13.0-rc1 (or a
		// pseudo-version based on it) does not carry the floor release's fixes. A
		// prerelease of a LATER release (v5.13.1-0.2025...) is still above v5.13.0.
		if v == goGitFloor && strings.HasPrefix(m[4], "-") {
			held = false
		}
	}
	if found == "" {
		return "", false, fmt.Errorf("no github.com/go-git/go-git/v5 requirement")
	}
	return found, held, nil
}

// TestGoGitCVEFloor asserts both modules that build go-git into the desk tools pin it at
// or above the CVE floor.
func TestGoGitCVEFloor(t *testing.T) {
	for _, mod := range []string{
		filepath.Join(deskRoot, "go.mod"),
		filepath.Join(deskRoot, "..", "..", "gitcore", "go.mod"),
	} {
		body, err := os.ReadFile(mod)
		if err != nil {
			t.Fatalf("could-not-check: %v", err)
		}
		v, held, err := goGitFloorHeld(string(body))
		if err != nil {
			t.Fatalf("%s: %v", mod, err)
		}
		if !held {
			t.Fatalf("%s: go-git %s is below the CVE floor v%d.%d.%d", mod, v, goGitFloor[0], goGitFloor[1], goGitFloor[2])
		}
		t.Logf("%s: go-git %s", mod, v)
	}
}

// TestGoGitFloorRefusesBelow: the floor check refuses a pin below v5.13.0 and a replace.
// regression: F-go-git-cve-floor
func TestGoGitFloorRefusesBelow(t *testing.T) {
	cases := []struct {
		gomod   string
		held    bool
		wantErr bool
	}{
		{"require (\n\tgithub.com/go-git/go-git/v5 v5.12.0\n)\n", false, false},
		{"require (\n\tgithub.com/go-git/go-git/v5 v5.4.2 // indirect\n)\n", false, false},
		{"require github.com/go-git/go-git/v5 v5.13.0\n", true, false},
		{"require github.com/go-git/go-git/v5 v5.13.0-rc1\n", false, false},
		{"require github.com/go-git/go-git/v5 v5.13.0-0.20241201000000-abcdef123456\n", false, false},
		{"require github.com/go-git/go-git/v5 v5.13.1-0.20250101000000-abcdef123456\n", true, false},
		{"require (\n\tgithub.com/go-git/go-git/v5 v5.19.3\n)\n", true, false},
		{"require (\n\tgithub.com/go-git/go-git/v5 v5.19.3\n)\nreplace github.com/go-git/go-git/v5 => ../fork\n", false, true},
		{"require (\n\tgithub.com/go-git/go-billy/v5 v5.9.2\n)\n", false, true},
	}
	for _, c := range cases {
		_, held, err := goGitFloorHeld(c.gomod)
		if (err != nil) != c.wantErr || held != c.held {
			t.Errorf("goGitFloorHeld(%q) = held %v err %v; want held %v err %v", c.gomod, held, err, c.held, c.wantErr)
		}
	}
}
