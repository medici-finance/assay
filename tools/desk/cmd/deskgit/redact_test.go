package main

import (
	"strings"
	"testing"
)

// regression: #6621 (security review A2)
func TestParseRepoRedactsUserinfo(t *testing.T) {
	for _, in := range []string{
		"https://user:secret@example.invalid",
		"user:secret@example.invalid:repo",
		"file://user:secret@example.invalid/x",
	} {
		_, err := parseRepo(in)
		if err == nil {
			t.Fatalf("parseRepo(%q) = nil error; want a refusal", in)
		}
		if msg := err.Error(); strings.Contains(msg, "secret") || !strings.Contains(msg, "example.invalid") {
			t.Fatalf("parseRepo error must redact the credential and keep the host: %s", msg)
		}
	}
}

// Shapes a local redactor once passed through: a password in an scp-like or ssh remote.
//
// regression: #6606
func TestRedactURLAllShapes(t *testing.T) {
	for _, in := range []string{
		"https://user:secret@github.com/o/r",
		"ssh://secret@github.com/o/r",
		"user:secret@github.com:o/r",
	} {
		if got := redactURL(in); strings.Contains(got, "secret") || !strings.Contains(got, "<redacted>@github.com") {
			t.Errorf("redactURL(%q) = %q, want the userinfo redacted and the host kept", in, got)
		}
	}
}
