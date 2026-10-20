package deskkit

import (
	"errors"
	"strings"
	"testing"
)

// ghPrefixFakeBody is a clearly-fake token body: 16 characters, so it sits under the
// 32-character run threshold and the entropy loop cannot refuse it. Only the prefix arm of
// reGitHubToken can, which is the arm these tests pin. The prefix is joined from fragments
// at each use, for the reason recorded on awsExampleSecretKey.
const ghPrefixFakeBody = "FAKEFAKE" + "00000000"

// checkGHPrefix asserts both halves of the shared pattern for one prefix: the outbound
// body scan refuses it as a github-token finding, and Scrub redacts it out of diagnostics.
func checkGHPrefix(t *testing.T, prefix string) {
	t.Helper()
	token := prefix + ghPrefixFakeBody
	text := "pasted " + token + " by mistake"

	err := BodyCheck([]byte(text))
	if !IsRefused(err) {
		t.Errorf("BodyCheck(%s...) = %v, want Refused", prefix, err)
	} else if de := (*DeskError)(nil); !errors.As(err, &de) || de.Finding == nil || de.Finding.Rule != "github-token" {
		t.Errorf("BodyCheck(%s...) refused by %+v, want rule github-token", prefix, de)
	}

	got := Scrub(text)
	if strings.Contains(got, ghPrefixFakeBody) {
		t.Errorf("Scrub left the %s token body in place: %q", prefix, got)
	}
	if !strings.Contains(got, redactedMarker) {
		t.Errorf("Scrub(%s...) = %q, want the redaction marker", prefix, got)
	}
}

// regression: #3560
func TestGHUPrefixScanAndScrub(t *testing.T) { checkGHPrefix(t, "ghu"+"_") }

// regression: #3560
func TestGHRPrefixScanAndScrub(t *testing.T) { checkGHPrefix(t, "ghr"+"_") }

// TestEveryDocumentedGHPrefix covers the whole documented set, so a seventh prefix missing
// from the alternation is a one-line addition here rather than a fresh finding. The set is
// the token-prefix table in GitHub's "About authentication to GitHub" documentation.
// regression: #3560
func TestEveryDocumentedGHPrefix(t *testing.T) {
	for _, p := range []string{"ghp" + "_", "github" + "_pat_", "gho" + "_", "ghu" + "_", "ghs" + "_", "ghr" + "_"} {
		t.Run(p, func(t *testing.T) { checkGHPrefix(t, p) })
	}
}
