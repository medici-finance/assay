package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// regression: #2395
func TestRevisionPortProfiles(t *testing.T) {
	t.Run("ordinary-repository-locator", func(t *testing.T) {
		a, _ := revisionFixture(t)
		b, _ := revisionFixture(t)
		// Give the two repositories distinct immutable commits, even in one second.
		runGit(t, b, "commit", "--allow-empty", "-qm", "decoy is deliberately distinct")
		want := strings.TrimSpace(string(parityGitBytes(t, b, "rev-parse", "HEAD")))
		runGit(t, b, "update-ref", "refs/remotes/origin/main", want)
		t.Setenv("GIT_DIR", filepath.Join(b, ".git"))
		if got := exactRefCommit(a, "refs/remotes/origin/main"); got != want {
			t.Errorf("ordinary exact ref lost locator: %q != %q", got, want)
		}
		if got, err := integrationTip(a); err != nil || got != want {
			t.Errorf("ordinary integration lost locator: %q %v", got, err)
		}
	})
	t.Run("ordinary-prefix-profile-is-not-provenance", func(t *testing.T) {
		root, _ := revisionFixture(t)
		nested := filepath.Join(root, "sub")
		if err := os.Mkdir(nested, 0755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("GIT_WORK_TREE", nested)
		want, err := exec.Command("git", "-C", nested, "rev-parse", "--show-prefix").Output()
		if err != nil || strings.TrimSpace(string(want)) != "" {
			t.Fatalf("native prefix profile fixture: %q %v", want, err)
		}
		got, err := gitPrefixOutput(nested)
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("ordinary prefix=(%q,%v), native=%q", got, err, want)
		}
		if why := verifyrunRootBelowToplevel(nested); why != "" {
			t.Errorf("ordinary root profile changed: %s", why)
		}
		if got, err := provenanceRootPrefix(nested); err != nil || got != "sub/" {
			t.Errorf("provenance profile was weakened: %q %v", got, err)
		}
	})
	t.Run("real-read-error-does-not-retry-native", func(t *testing.T) {
		root := dhFixture(t, unruledBase())
		dhSeams(t, nil)
		marker := revisionNativeGuard(t)
		var code int
		out := captureStdout(t, func() { code = runDecisionGate(root, "", strings.Repeat("e", 40)) })
		if code == 0 || !strings.Contains(out, "does not resolve to a commit") {
			t.Errorf("missing object did not refuse: %d %s", code, out)
		}
		revisionNoStarts(t, marker)
	})
	t.Run("native-leading-option-argv-retained", func(t *testing.T) {
		root := dhFixture(t, unruledBase())
		dhSeams(t, nil)
		native, err := exec.LookPath("git")
		if err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		log := filepath.Join(dir, "argv")
		quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
		script := "#!/bin/sh\nfor arg do\nif [ \"$arg\" = rev-parse ]; then printf '%s\\0' \"$@\" >" + quote(log) + "; fi\ndone\nexec " + quote(native) + " \"$@\"\n"
		if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
		var code int
		captureStdout(t, func() { code = runDecisionGate(root, "", "--not-a-ref") })
		if code == 0 {
			t.Error("leading option form no longer refuses")
		}
		got, err := os.ReadFile(log)
		want := strings.Join([]string{"-C", root, "rev-parse", "--verify", "--quiet", "--not-a-ref^{commit}"}, "\x00") + "\x00"
		if err != nil || string(got) != want {
			t.Errorf("native argv changed: %q %v want %q", got, err, want)
		}
		if _, _, supported := commitReadOutput(ordinaryCommitRead(root, "HEAD~0"), "HEAD~0"); supported {
			t.Error("unqualified expression was enrolled in library resolver")
		}
	})
}
