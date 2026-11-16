package main

import (
	"strings"
	"testing"
)

// regression: #6606
func TestPreflightRedactsOrigin(t *testing.T) {
	work := newBaseFixture(t)
	mustGit(t, work, "remote", "set-url", "origin", "https://user:secret@example.invalid/repo.git")
	_, err := preflight(work)
	if err == nil {
		t.Fatal("preflight on an unparseable origin = nil error; want a refusal")
	}
	if msg := err.Error(); strings.Contains(msg, "secret") || !strings.Contains(msg, "example.invalid/repo.git") {
		t.Fatalf("preflight error must redact the credential and keep the shape: %s", msg)
	}
}
