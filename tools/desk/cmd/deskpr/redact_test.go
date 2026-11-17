package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #6621 (security review A2)
func TestPreflightRedactsOrigin(t *testing.T) {
	work := newBaseFixture(t)
	mustGit(t, work, "checkout", "-q", "-b", "feature/redact")
	mustGit(t, work, "remote", "set-url", "origin", "https://user:secret@example.invalid/repo.git")
	_, err := preflight(work, "")
	if err == nil {
		t.Fatal("preflight on an unparseable origin = nil error; want a refusal")
	}
	if msg := err.Error(); strings.Contains(msg, "secret") || !strings.Contains(msg, "example.invalid/repo.git") {
		t.Fatalf("preflight error must redact the credential and keep the shape: %s", msg)
	}
}

// Shapes a local redactor once passed through: a password in an scp-like or ssh remote.
// The package's local redactor is gone; its messages render through deskkit.RedactRemoteUserinfo,
// so these shapes are pinned on that renderer.
//
// regression: #6606
func TestRedactURLAllShapes(t *testing.T) {
	for _, in := range []string{
		"https://user:secret@github.com/o/r",
		"ssh://secret@github.com/o/r",
		"user:secret@github.com:o/r",
	} {
		if got := deskkit.RedactRemoteUserinfo(in); strings.Contains(got, "secret") || !strings.Contains(got, "<redacted>@github.com") {
			t.Errorf("RedactRemoteUserinfo(%q) = %q, want the userinfo redacted and the host kept", in, got)
		}
	}
}
