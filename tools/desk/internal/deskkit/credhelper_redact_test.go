package deskkit

import (
	"strings"
	"testing"
)

// regression: #6696
// A token-only userinfo (the user half IS the credential, no password) must not reach the
// credential-chain detail, for an http(s) or an ssh:// push URL, nor the failed preflight
// Check that renders that detail.
func TestCredDetailRedactsToken(t *testing.T) {
	const secret = "example-secret-value"
	r := newChainRepo(t)
	for _, u := range []string{
		"https://" + secret + "@example.invalid/example-org/example-repo.git",
		"ssh://" + secret + "@example.invalid/example-org/example-repo.git",
	} {
		ok, detail, err := credTransportMatchesApp(r.dir, u, r.token)
		if err != nil || ok {
			t.Fatalf("credTransportMatchesApp(%q) = %v, %q, %v; want a red verdict", u, ok, detail, err)
		}
		if !strings.Contains(detail, "example.invalid") {
			t.Fatalf("detail does not name the push URL's host: %q", detail)
		}
		if strings.Contains(detail, secret) {
			t.Fatalf("credential-chain detail echoes a token-only userinfo: %q", detail)
		}
	}

	r.git("-C", r.dir, "remote", "set-url", "origin", "https://"+secret+"@example.invalid/example-org/example-repo.git")
	p := PreflightProbes{CredHelperMatchesApp: credHelperMatchesAppProbe}
	c := checkAmbientTransport(p, Landing{Dir: r.dir, Remote: "origin"}, r.token, "the worker App", "")
	if c.State != CheckedFailed {
		t.Fatalf("ambient transport check = %v (%q); want checked-failed", c.State, c.Detail)
	}
	if strings.Contains(c.Detail+c.Remediation, secret) {
		t.Fatalf("failed preflight check echoes a token-only userinfo: %q", c.Detail)
	}
}
