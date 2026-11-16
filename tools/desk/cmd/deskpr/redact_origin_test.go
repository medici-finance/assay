package main

import (
	"strings"
	"testing"
)

// regression: #6696
// The origin gate quotes the fetch URL when it does not parse; the userinfo secret a
// credential-bearing https origin carries must not reach that refusal.
func TestPreflightRedactsOrigin(t *testing.T) {
	hermeticGitConfig(t)
	work := newBaseFixture(t)
	const secret = "example-secret-value"
	mustGit(t, work, "remote", "set-url", "origin", "https://example-user:"+secret+"@example.invalid/only-one")
	_, err := preflightOpts(work, "", false, "detached", false)
	if err == nil {
		t.Fatal("want a refusal for an unparseable origin, got nil")
	}
	if !strings.Contains(err.Error(), "cannot parse origin repo") {
		t.Fatalf("refusal is not the origin-parse refusal under test: %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("refusal echoes the origin URL's userinfo secret: %v", err)
	}
}
