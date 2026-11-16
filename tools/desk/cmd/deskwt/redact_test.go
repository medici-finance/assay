package main

import (
	"strings"
	"testing"
)

// regression: #6606
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
