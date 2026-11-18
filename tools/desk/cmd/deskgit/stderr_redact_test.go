package main

import (
	"os/exec"
	"strings"
	"testing"
)

// regression: #6696
// git's stderr quotes the argument it failed on, so a failed git call must mask a userinfo in
// its stderr as well as in its argv — otherwise the argv redaction is bypassed by git echoing
// the same remote back.
func TestGitStderrRedacted(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	const secret = "example-secret-value"
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	for _, arg := range []string{
		"example-user:" + secret + "@example.invalid:example-org/example-repo",
		"https://" + secret + "@example.invalid/example-org/example-repo",
	} {
		// checkout of an unknown pathspec fails with "pathspec '<arg>' did not match", so git's
		// stderr carries the argument verbatim.
		_, err := runGit(dir, "checkout", arg)
		if err == nil {
			t.Fatalf("checkout of %q succeeded; want a failure that quotes it", arg)
		}
		if !strings.Contains(err.Error(), "pathspec '") || !strings.Contains(err.Error(), "example.invalid") {
			t.Fatalf("failure lost git's stderr or the remote's shape: %v", err)
		}
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("failed git call echoes a userinfo secret through its stderr: %v", err)
		}
	}
}
