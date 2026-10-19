package deskkit

import "testing"

// regression: #3281
func TestLandingPushRewrite(t *testing.T) {
	dir := rewriteRepo(t)
	const target = "https://github.com/example/project.git"
	rewriteGit(t, "-C", dir, "config", "remote.upstream.url", target)
	rewriteGit(t, "-C", dir, "config", "url.https://other.invalid/.pushInsteadOf", "unmatched:")
	if got := landingRemoteURL(Landing{Dir: dir, Remote: "upstream"}); got != target {
		t.Fatalf("landingRemoteURL = %q; want %q", got, target)
	}
}
