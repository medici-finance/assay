package main

import (
	"strings"
	"testing"
)

// regression: #6696
func TestReconcileRedactsOrigin(t *testing.T) {
	const secret = "example-secret-value"
	root, wt := absWT(t)
	exec := func(dir, name string, args ...string) (string, error) {
		if name == "git" && len(args) >= 2 && args[0] == "config" && args[1] == "--get" {
			return "https://example-user:" + secret + "@example.invalid\n", nil
		}
		return "", nil
	}
	_, err := Run(Options{Root: root, Worktree: wt, Now: runNow, DryRun: true, exec: exec})
	if err == nil || !strings.Contains(err.Error(), "could not resolve owner/name") {
		t.Fatalf("want the origin refusal, got %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("refusal echoes the origin URL's userinfo secret: %v", err)
	}
}
