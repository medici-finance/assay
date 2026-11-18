package main

import (
	"strings"
	"testing"
)

// regression: #6696
// The fail-open line for an unparseable remote quotes the URL; it must not quote the
// userinfo secret a credential-bearing remote carries.
func TestUnparsedRemoteRedacted(t *testing.T) {
	const secret = "example-secret-value"
	stdin := stdinString("refs/heads/example-branch x refs/heads/example-branch y\n")
	var stderr strings.Builder
	run([]string{"origin", "https://example-user:" + secret + "@example.invalid"}, stdin, &stderr)
	if !strings.Contains(stderr.String(), "cannot derive repo from remote") {
		t.Fatalf("expected the fail-open parse line, got: %s", stderr.String())
	}
	if strings.Contains(stderr.String(), secret) {
		t.Fatalf("stderr echoes the remote URL's userinfo secret: %s", stderr.String())
	}
}
