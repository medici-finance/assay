package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// replayFixture makes a working tree and, outside it, a replay file. It
// returns the tree, the outside directory and the replay file's resolved path.
func replayFixture(t *testing.T) (string, string, string) {
	t.Helper()
	root, outside := t.TempDir(), t.TempDir()
	file := filepath.Join(outside, "replay.json")
	if err := os.WriteFile(file, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	real, err := filepath.EvalSymlinks(file)
	if err != nil {
		t.Fatal(err)
	}
	return root, outside, real
}

// regression: #7454
// TestAdmittedReplayInput: the one declared replay path reaches an admitted
// row only by its exact name, made absolute and symlink-resolved, naming an
// existing regular file outside the working tree. A path inside the tree is
// refused, also when a symlink outside the tree leads into it.
func TestAdmittedReplayInput(t *testing.T) {
	root, outside, real := replayFixture(t)
	inside := filepath.Join(root, "replay.json")
	if err := os.WriteFile(inside, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(outside, "link.json")
	intoFile := filepath.Join(outside, "into-file.json")
	intoDir := filepath.Join(outside, "into-tree")
	for target, l := range map[string]string{real: link, inside: intoFile, root: intoDir} {
		if err := os.Symlink(target, l); err != nil {
			t.Skipf("no symlinks here: %v", err)
		}
	}
	want := admittedReplayInputVar + "=" + real
	accept := map[string]string{"absolute": real, "symlink": link, "unclean": outside + "/./replay.json"}
	for name, value := range accept {
		kv, why := admittedReplayInput([]string{"PATH=/bin", admittedReplayInputVar + "=" + value}, root)
		if why != "" || kv != want {
			t.Errorf("%s %q: got (%q, %q), want (%q, \"\")", name, value, kv, why, want)
		}
	}
	t.Run("relative", func(t *testing.T) {
		t.Chdir(outside)
		if kv, why := admittedReplayInput([]string{admittedReplayInputVar + "=replay.json"}, root); why != "" || kv != want {
			t.Errorf("relative path: got (%q, %q), want (%q, \"\")", kv, why, want)
		}
	})
	refuse := map[string]string{"inside": inside, "symlink into tree": intoFile,
		"through linked dir": filepath.Join(intoDir, "replay.json"), "missing": filepath.Join(outside, "absent.json"),
		"directory": outside, "empty": "", "tree itself": root}
	for name, value := range refuse {
		kv, why := admittedReplayInput([]string{admittedReplayInputVar + "=" + value}, root)
		if kv != "" || why == "" {
			t.Errorf("%s %q: got (%q, %q), want a refusal and no input", name, value, kv, why)
		}
	}
	for _, env := range [][]string{nil, {"assay_cohort_replay=" + real}, {"ASSAY_COHORT_REPLAY_OUT=" + real}} {
		if kv, why := admittedReplayInput(env, root); kv != "" || why != "" {
			t.Errorf("%q: got (%q, %q), want no input and no refusal", env, kv, why)
		}
	}
}

// regression: #7454
// TestAdmittedRowSeesReplay: end to end through verifyrun, an admitted row
// reads the replay path by the guard derived-board/10 row 14 uses, and gets
// the resolved path. A replay inside the working tree is refused, and the row
// that reads it is could-not-run, never run without its input.
func TestAdmittedRowSeesReplay(t *testing.T) {
	root, _ := admittedFixture(t, true)
	_, _, real := replayFixture(t)
	t.Setenv(admittedReplayInputVar, real)
	row := `test -n "$ASSAY_COHORT_REPLAY" && test -f "$ASSAY_COHORT_REPLAY" && test "$ASSAY_COHORT_REPLAY" = '` + real + `'`
	if code := verifyrunRow(t, root, row); code != verifyrunExitPass {
		t.Fatalf("admitted row did not receive the replay path (exit %d)", code)
	}
	inside := filepath.Join(root, "replay.json")
	if err := os.WriteFile(inside, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(admittedReplayInputVar, inside)
	tick := string(rune(96))
	brief := filepath.Join(t.TempDir(), "brief.md")
	body := "# Fixture\n\n## Verify\n\n| # | Command | Expect |\n|---|---|---|\n| 1 | " + tick + `test -f "$ASSAY_COHORT_REPLAY"` + tick + " | exit 0 |\n\n## Evidence\n\n"
	if err := os.WriteFile(brief, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, out := captureVerifyrunOut(t, []string{"--brief", brief, "--root", root, "--dry-run"})
	if got.code != verifyrunExitCouldNot {
		t.Fatalf("admitted row given a replay inside the tree gave exit %d, want could-not-run (exit %d):\n%s", got.code, verifyrunExitCouldNot, out)
	}
	if !strings.Contains(out, "resolves inside the working tree") {
		t.Fatalf("the refusal does not say the replay resolves inside the working tree:\n%s", out)
	}
}
