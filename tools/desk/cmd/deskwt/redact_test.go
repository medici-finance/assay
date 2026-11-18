package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #6621 (security review A2)
func TestCurrentRepoRedactsOrigin(t *testing.T) {
	installWorktreeFixture(t)
	work := t.TempDir()
	mustGit(t, "", "init", "-b", "main", work)
	mustGit(t, work, "remote", "add", "origin", "https://user:secret@example.invalid/repo.git")
	_, err := currentRepo(work)
	if err == nil {
		t.Fatal("currentRepo on an unparseable origin = nil error; want a refusal")
	}
	if msg := err.Error(); strings.Contains(msg, "secret") || !strings.Contains(msg, "example.invalid/repo.git") {
		t.Fatalf("currentRepo error must redact the credential and keep the shape: %s", msg)
	}
}

// Shapes a local redactor once passed through: a password in an scp-like or ssh remote.
// The package's local redactor is gone; its messages render through deskkit.RedactRemoteURL,
// so these shapes are pinned on that renderer.
//
// regression: #6606
func TestRedactURLAllShapes(t *testing.T) {
	for _, in := range []string{
		"https://user:secret@github.com/o/r",
		"ssh://secret@github.com/o/r",
		"user:secret@github.com:o/r",
		"git:secret@github.com:o/r",
		"git@github.com:secret@github.com:o/r",
	} {
		if got := deskkit.RedactRemoteURL(in); strings.Contains(got, "secret") || !strings.Contains(got, "<redacted>@github.com") {
			t.Errorf("RedactRemoteURL(%q) = %q, want the userinfo redacted and the host kept", in, got)
		}
	}
	for _, in := range []string{"ssh://git@github.com/o/r", "git@github.com:o/r"} {
		if got := deskkit.RedactRemoteURL(in); got != in {
			t.Errorf("RedactRemoteURL(%q) = %q, want the conventional git login kept", in, got)
		}
	}
}
