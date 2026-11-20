package main

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

var printedClaimCmd = regexp.MustCompile("`([^`]*deskclaim-ref (progress|release) [^`]*)`")

// The Go claim tool resolves a work-reference key in the directory it runs in, and the
// dispatch's acquire ran in --root. The printed progress and release commands therefore name
// that directory, or a reader in another checkout gets "no published owning root" for the
// claim this dispatch just took.
// regression: #6896
func TestNotLaunchedPinsClaimDir(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	withGoClaimOnPath(t)
	t.Setenv("DESK_LOOP", "worker-desk")
	s.replies = happyReplies(filepath.Join(t.TempDir(), "worker-home"))
	rc, stderr := runCapturingStderr(t, []string{"example-stream--07", "--root", root, "--repo", allowedRepo,
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d, want 0\n%s", rc, stderr)
	}
	line := lineWith(stderr, "NOT LAUNCHED")
	got := printedClaimCmd.FindAllStringSubmatch(line, -1)
	if len(got) != 2 {
		t.Fatalf("NOT LAUNCHED line carries %d claim command(s), want progress and release:\n%s", len(got), line)
	}
	want := "cd " + shellWord(root) + " && " + goClaimBinary + " "
	for _, m := range got {
		if len(m[1]) < len(want) || m[1][:len(want)] != want {
			t.Errorf("printed %s command does not run in the acquire's directory %q:\n%s", m[2], root, m[1])
		}
	}
}

// shellWord must hand the shell back exactly the path it was given.
// regression: #6896
func TestShellWordRoundTrips(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh on PATH")
	}
	for _, s := range []string{"/plain/path-1.2_x", "/with space/x", "/it's/here", "/$HOME/`x`", ""} {
		out, err := exec.Command(sh, "-c", "printf %s "+shellWord(s)).Output()
		if err != nil || string(out) != s {
			t.Errorf("shellWord(%q) = %s; shell read back %q (%v)", s, shellWord(s), out, err)
		}
	}
}
