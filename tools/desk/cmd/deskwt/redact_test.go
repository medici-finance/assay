package main

import (
	"strings"
	"testing"
)

// regression: #6621 (security review A2)
func TestCurrentRepoRedactsOrigin(t *testing.T) {
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
//
// regression: #6606
func TestRedactURLAllShapes(t *testing.T) {
	for _, in := range []string{
		"https://user:secret@example.invalid/o/r",
		"ssh://user:secret@example.invalid/o/r",
		"user:secret@example.invalid:o/r",
		"git:secret@example.invalid:o/r",
		"ssh://git@x:secret@example.invalid/o/r",
	} {
		if got := redactURL(in); strings.Contains(got, "secret") || !strings.Contains(got, "<redacted>@example.invalid") {
			t.Errorf("redactURL(%q) = %q, want the userinfo redacted and the host kept", in, got)
		}
	}
	for _, in := range []string{"ssh://git@example.invalid/o/r", "git@example.invalid:o/r"} {
		if got := redactURL(in); got != in {
			t.Errorf("redactURL(%q) = %q, want the conventional git login kept", in, got)
		}
	}
}
