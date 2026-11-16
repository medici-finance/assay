package main

import (
	"strings"
	"testing"
)

// regression: #6696
// Every parseRepo refusal that quotes the remote URL quotes it with the userinfo masked.
func TestParseRepoRedactsUserinfo(t *testing.T) {
	const secret = "example-secret-value"
	for _, in := range []string{
		"ext::https://example-user:" + secret + "@example.invalid/o/r", // remote-helper form
		"https://example-user:" + secret + "@example.invalid",          // no path
		"file://example-user:" + secret + "@example.invalid/o/r",       // file:// not a desk root
	} {
		_, err := parseRepo(in)
		if err == nil {
			t.Fatalf("parseRepo(%q) = nil error; want a refusal", in)
		}
		if strings.Contains(err.Error(), secret) {
			t.Errorf("parseRepo(%q) refusal echoes the userinfo secret: %v", in, err)
		}
	}
}
