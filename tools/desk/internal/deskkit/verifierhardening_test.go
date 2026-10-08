package deskkit

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeGitOnPath puts a `git` first on PATH that runs script (a POSIX shell
// fragment, with "$@" the git arguments) and otherwise hands every call to
// the real git. POSIX-only: the wrapper is a #!/bin/sh script.
func fakeGitOnPath(t *testing.T, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only: the fake git is a #!/bin/sh script on PATH")
	}
	real, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git")
	}
	bin := t.TempDir()
	body := "#!/bin/sh\n" + script + "\nexec '" + real + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// renderFixture is a prepared home with one tracked file, and that file's
// blob, for calling verifierRender directly.
func renderFixture(t *testing.T) (string, string) {
	t.Helper()
	root, _ := verifierFixture(t, "gpt-6-astra")
	return root, strings.TrimSpace(attestOut(t, root, "rev-parse", "HEAD:source.txt"))
}

// TestRenderNeedsAttrSource: a checkout with no attested attribute source
// cannot be rendered, and says so, rather than rendering nothing.
func TestRenderNeedsAttrSource(t *testing.T) {
	root, blob := renderFixture(t)
	got, err := verifierRender(root, "source.txt", blob, verifierCheckout{autocrlf: "false", eol: "native"})
	if err == nil || ExitCodeOf(err) != ExitUnverifiable || got != nil {
		t.Fatalf("render without an attribute source: got %q, %v", got, err)
	}
}

// TestOldGitProbeClass: only git rejecting --attr-source itself (exit 129,
// naming the option, as git before 2.41 does) is classed as an old git. Any
// other failure of the same probe is could-not-check, not a claim about the
// git.
func TestOldGitProbeClass(t *testing.T) {
	for _, tc := range []struct {
		name, script, want string
		code               int
	}{
		{"old-git", `case "$*" in *--attr-source=*) echo "unknown option: --attr-source=HEAD" >&2; exit 129;; esac`, "cannot be admitted on this git", ExitRefused},
		{"other-failure", `case "$*" in *--attr-source=*) echo "fatal: not a git repository" >&2; exit 128;; esac`, "cannot read attested attributes", ExitUnverifiable},
		{"other-usage", `case "$*" in *--attr-source=*) echo "unknown option: --other" >&2; exit 129;; esac`, "cannot read attested attributes", ExitUnverifiable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, blob := renderFixture(t)
			fakeGitOnPath(t, tc.script)
			_, err := verifierRender(root, "source.txt", blob, verifierCheckout{autocrlf: "false", eol: "native", source: "HEAD"})
			if err == nil || !strings.Contains(err.Error(), tc.want) || ExitCodeOf(err) != tc.code {
				t.Fatalf("want %q (exit %d), got %v", tc.want, tc.code, err)
			}
		})
	}
}

// TestWorkTreeConfigUnread: a work tree config that cannot be read is
// could-not-check, never taken as unset.
func TestWorkTreeConfigUnread(t *testing.T) {
	root, _ := verifierFixture(t, "gpt-6-astra")
	home, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	fakeGitOnPath(t, `case "$*" in *"config --get-all core.worktree"*) echo "fatal: bad config" >&2; exit 3;; esac`)
	if err := verifierOwnWorkTree(home); err == nil || ExitCodeOf(err) != ExitUnverifiable {
		t.Fatalf("unreadable work tree config admitted: %v", err)
	}
}

// TestRowReadProbeUnrunnable: a row read that cannot be run at all is
// could-not-check, not admitted.
func TestRowReadProbeUnrunnable(t *testing.T) {
	root, _ := verifierFixture(t, "gpt-6-astra")
	fakeGitOnPath(t, `case "$*" in *" grep "*) kill -9 $$;; esac`)
	if err := verifierRowGitReads(root); err == nil || ExitCodeOf(err) != ExitUnverifiable {
		t.Fatalf("unrunnable row read admitted: %v", err)
	}
}

// TestAttestSubdirHome: a home inside a repository but not at its top is
// refused by git's resolved view alone (nothing configures a work tree): a
// row's git reads the whole work tree, files admission never compared.
func TestAttestSubdirHome(t *testing.T) {
	root := t.TempDir()
	verifierFixtureRepo(t, root, map[string]string{"sub/brief.md": "# Sub\n\n## Verify\n\n| 1 | true | exit 0 |\n\n## Evidence\n\nPending.\n"})
	sub := filepath.Join(root, "sub")
	// The plant: a row run from the home reads a file outside it.
	if got := attestOut(t, sub, "grep", "-l", "attested source", "--", ":/"); !strings.Contains(got, "source.txt") {
		t.Fatalf("row git does not read outside the subdirectory home: %q", got)
	}
	if out, err := exec.Command("git", "-C", root, "config", "--get-all", "core.worktree").Output(); err == nil {
		t.Fatalf("fixture configures a work tree: %q", out)
	}
	err := PrepareVerifierAttestation(sub, "example-org/one", "brief.md", "gpt-6-astra", "strong")
	if err == nil || !strings.Contains(err.Error(), "git work tree is not the home itself") {
		t.Fatalf("subdirectory home: want refusal, got %v", err)
	}
}

// TestAttestCaseVariantRoot: on a case-insensitive file system (macOS,
// Windows) a home named in another letter case, or with forward slashes, is
// the same home: it prepares, and checks under either spelling.
func TestAttestCaseVariantRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Home")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	variant := filepath.Join(filepath.Dir(root), "HOME")
	a, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	if b, err := os.Stat(variant); err != nil || !os.SameFile(a, b) {
		t.Skip("case-sensitive file system: no second spelling of the home")
	}
	verifierFixtureRepo(t, root, nil)
	given := filepath.ToSlash(variant)
	if err := PrepareVerifierAttestation(given, "example-org/one", "brief.md", "gpt-6-astra", "strong"); err != nil {
		t.Fatalf("case-variant home refused: %v", err)
	}
	f := &verifierForge{actor: "example-desk[bot]"}
	if _, err := IssueVerifierAttestation(root, f); err != nil {
		t.Fatal(err)
	}
	for _, spelling := range []string{root, variant, given} {
		if _, err := CheckVerifierAttestationWithForge(spelling, "brief.md", f); err != nil {
			t.Fatalf("check from %s: %v", spelling, err)
		}
	}
}

// TestVerifierSameDir: one directory under two spellings is the same; two
// directories are not, whatever their names.
func TestVerifierSameDir(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	if !verifierSameDir(a, a+string(filepath.Separator)+".") || !verifierSameDir(a, filepath.ToSlash(a)) {
		t.Fatal("one directory under two spellings compared different")
	}
	if verifierSameDir(a, b) || verifierSameDir(a, filepath.Join(a, "absent")) {
		t.Fatal("different or absent directories compared the same")
	}
}
