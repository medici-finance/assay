package main

// first-sweep-prereqs (#2986).
//
// The defect: deskboot printed BOOT COMPLETE (8/8) on a consumer whose roster named no forge
// for the repo and whose `.assay-versions` carried only statusgen lines. The role's very next
// commands failed — `desksupervise status --stops` could not resolve a forge endpoint (exit 6),
// and `deskboard`'s drift banner was STALE-UNKNOWN. Boot proved a narrower setup than the first
// sweep reads. These tests pin the step that closes that gap.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const statusgenOnlyPins = "statusgen v1.0.9 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\n" +
	"statusgen-source v1.0.9 0123456789abcdef0123456789abcdef01234567\n"

// plantRoster replaces the fixture roster with body (same mode, same place).
func plantRoster(t *testing.T, home, body string) {
	t.Helper()
	p := filepath.Join(home, ".config", "assay", "roster.env")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	deskkit.ReloadConfig()
}

// rosterWithForges is the fixture roster with its ASSAY_REPO_FORGES line replaced by line
// ("" drops the key entirely).
func rosterWithForges(t *testing.T, line string) string {
	t.Helper()
	var out []string
	replaced := false
	for _, l := range strings.Split(fixtureRoster, "\n") {
		if strings.HasPrefix(l, deskkit.EnvRepoForges+"=") {
			replaced = true
			if line != "" {
				out = append(out, line)
			}
			continue
		}
		out = append(out, l)
	}
	if !replaced {
		t.Fatal("fixture roster carries no ASSAY_REPO_FORGES line to replace")
	}
	return strings.Join(out, "\n")
}

// consumerStub is happyStub for a CONSUMER checkout: origin/main carries no in-tree
// tools/desk source, so only a pin line can prove desk-tools provenance.
func consumerStub(t *testing.T, tokenPath, pins string) []reply {
	t.Helper()
	return append([]reply{
		{match: "refs/remotes/origin/main:tools/desk", fail: true},
		{match: "show FETCH_HEAD:.assay-versions", stdout: pins},
	}, happyStub(t, tokenPath)...)
}

func writePins(t *testing.T, root, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, ".assay-versions"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// captureStdout redirects os.Stdout for the duration of fn and returns what was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var b strings.Builder
		buf := make([]byte, 4096)
		for {
			n, rerr := r.Read(buf)
			if n > 0 {
				b.Write(buf[:n])
			}
			if rerr != nil {
				break
			}
		}
		done <- b.String()
	}()
	fn()
	os.Stdout = old
	_ = w.Close()
	out := <-done
	_ = r.Close()
	return out
}

// bootOutput runs a boot and returns its rc, stdout and stderr.
func bootOutput(t *testing.T, root string) (rc int, stdout, stderr string) {
	t.Helper()
	stderr = captureStderr(t, func() int {
		stdout = captureStdout(t, func() { rc = run([]string{"the-desk", "--root", root}) })
		return 0
	})
	return rc, stdout, stderr
}

// wantRefusal asserts a first-sweep refusal: exit 6, the step named, every fragment
// present, and no BOOT COMPLETE.
func wantRefusal(t *testing.T, rc int, stdout, stderr string, fragments ...string) {
	t.Helper()
	if rc != deskkit.ExitUnverifiable {
		t.Errorf("boot rc = %d, want %d — a setup the first sweep cannot run on must not boot green",
			rc, deskkit.ExitUnverifiable)
	}
	if strings.Contains(stdout, "BOOT COMPLETE") {
		t.Errorf("boot printed BOOT COMPLETE on an incomplete setup:\n%s", stdout)
	}
	if !strings.Contains(stderr, "step first-sweep-prereqs") {
		t.Errorf("the refusal does not name step first-sweep-prereqs:\n%s", stderr)
	}
	for _, f := range fragments {
		if !strings.Contains(stderr, f) {
			t.Errorf("the refusal does not carry %q:\n%s", f, stderr)
		}
	}
}

// A roster with no forge mapping for the boot's repo is refused, naming the exact roster line.
// The origin remote IS github.com — the boot must not accept the origin host as the answer,
// because desksupervise's cross-repo endpoint deliberately does not.
// regression: #2986
func TestBootRefusesNoForgeMapping(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	plantRoster(t, home, rosterWithForges(t, ""))
	s.replies = happyStub(t, writeToken(t, home))

	rc, stdout, stderr := bootOutput(t, root)
	wantRefusal(t, rc, stdout, stderr, "ASSAY_REPO_FORGES=medici-finance/assay=github", "roster.env")
}

// Planted second instance of the class: the key EXISTS but names only other repos. Presence
// of the key is not the required entry.
// regression: #2986
func TestBootRefusesForeignForgeOnly(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	plantRoster(t, home, rosterWithForges(t, "ASSAY_REPO_FORGES=example-org/tracker=github"))
	s.replies = happyStub(t, writeToken(t, home))

	rc, stdout, stderr := bootOutput(t, root)
	wantRefusal(t, rc, stdout, stderr, "ASSAY_REPO_FORGES=medici-finance/assay=github")
}

// Statusgen-only pin files that MATCH origin/main's are refused: worktree-current proves the
// two files are equal, never that the entries the board reads exist.
// regression: #2986
func TestBootRefusesStatusgenPins(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	writePins(t, root, statusgenOnlyPins)
	s.replies = consumerStub(t, writeToken(t, home), statusgenOnlyPins)

	rc, stdout, stderr := bootOutput(t, root)
	wantRefusal(t, rc, stdout, stderr, "desk-tools", "assay-install.sh pin --kind desk-tools")
}

// Planted second instance: a desk-tools line for ANOTHER platform only. The board's reader
// never matches it on this host, so neither may the boot's.
// regression: #2986
func TestBootRefusesForeignPlatPin(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	pins := statusgenOnlyPins +
		"desk-tools-plan9-386 v1.0.9 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\n"
	writePins(t, root, pins)
	s.replies = consumerStub(t, writeToken(t, home), pins)

	rc, stdout, stderr := bootOutput(t, root)
	wantRefusal(t, rc, stdout, stderr, "desk-tools-"+runtime.GOOS+"-"+runtime.GOARCH)
}

// A desk-tools-source line with a short commit cannot bind the stamped binary, so it is
// not provenance either (the board falls through it to could-not-check).
func TestBootRefusesShortSourcePin(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	pins := statusgenOnlyPins + "desk-tools-source v1.0.9 0123abc\n"
	writePins(t, root, pins)
	s.replies = consumerStub(t, writeToken(t, home), pins)

	rc, stdout, stderr := bootOutput(t, root)
	wantRefusal(t, rc, stdout, stderr, "desk-tools-source")
}

// Both prerequisites missing: ONE refusal names both remedies, so an operator fixes the
// setup in one pass instead of rebooting once per gap.
func TestBootNamesBothMissingPrereqs(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	plantRoster(t, home, rosterWithForges(t, ""))
	writePins(t, root, statusgenOnlyPins)
	s.replies = consumerStub(t, writeToken(t, home), statusgenOnlyPins)

	rc, stdout, stderr := bootOutput(t, root)
	wantRefusal(t, rc, stdout, stderr, "ASSAY_REPO_FORGES=medici-finance/assay=github",
		"assay-install.sh pin --kind desk-tools")
}

// A complete consumer setup — forge mapping plus a desk-tools pin for this host's platform,
// statusgen lines kept beside it — boots, and the step reports what it proved.
// regression: #2986
func TestBootPassesCompleteSetup(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	pins := statusgenOnlyPins + "desk-tools-" + runtime.GOOS + "-" + runtime.GOARCH +
		" v1.0.9 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\n"
	writePins(t, root, pins)
	s.replies = consumerStub(t, writeToken(t, home), pins)

	rc, stdout, stderr := bootOutput(t, root)
	if rc != deskkit.ExitOK {
		t.Fatalf("complete setup rc = %d, want 0:\n%s", rc, stderr)
	}
	for _, want := range []string{"first-sweep-prereqs OK", "forge github", "v1.0.9", "BOOT COMPLETE", "9/9"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("boot output does not carry %q:\n%s", want, stdout)
		}
	}
}

// The source tree (no pin file, origin/main carries tools/desk) is provenance on its own:
// the board's in-tree fallback reads exactly that ref.
func TestBootPassesInTreeSource(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	s.replies = happyStub(t, writeToken(t, home))

	rc, stdout, stderr := bootOutput(t, root)
	if rc != deskkit.ExitOK {
		t.Fatalf("source-tree boot rc = %d, want 0:\n%s", rc, stderr)
	}
	if !strings.Contains(stdout, "first-sweep-prereqs OK") || !strings.Contains(stdout, "refs/remotes/origin/main:tools/desk") {
		t.Errorf("boot output does not report the in-tree provenance:\n%s", stdout)
	}
	if !s.ran("rev-parse --verify --quiet refs/remotes/origin/main:tools/desk") {
		t.Error("the in-tree provenance was reported without reading the fully-qualified origin/main ref")
	}
}
