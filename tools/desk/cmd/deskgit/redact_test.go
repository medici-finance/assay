package main

import (
	"strings"
	"testing"
)

// regression: #6606
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
