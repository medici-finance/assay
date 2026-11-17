package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// regression: #6696
func TestLsRemoteFailRedacts(t *testing.T) {
	const secret = "example-secret-value"
	dir := t.TempDir()
	// A push destination whose spelling carries a userinfo-shaped credential and does not
	// exist, so `git ls-remote` fails offline and runCmd renders its argv and stderr.
	dest := filepath.Join(t.TempDir(), "example-user:"+secret+"@missing.git")
	for _, args := range [][]string{
		{"init", "-q", dir},
		{"-C", dir, "remote", "add", "origin", "https://example.invalid/example-org/example-repo.git"},
		{"-C", dir, "remote", "set-url", "--push", "origin", dest},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	err := requireRemoteHoldsHead(dir, "feature/x", "0123456789abcdef0123456789abcdef01234567", 1)
	if err == nil || !strings.Contains(err.Error(), "ls-remote") {
		t.Fatalf("want an ls-remote failure, got %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("ls-remote failure echoes the push URL's userinfo secret: %v", err)
	}
}
