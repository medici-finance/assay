package deskkit

import (
	"strings"
	"testing"
)

// regression: #6696
// hostOfRemote's refusals quote the remote; neither may quote its userinfo credential.
func TestHostOfRemoteRedacts(t *testing.T) {
	const secret = "example-secret-value"
	for _, in := range []string{
		"https://example-user:" + secret + "@/o/r", // empty host
		"example-user:" + secret + "@",             // no host at all
	} {
		_, err := HostOfRemote(in)
		if err == nil {
			t.Fatalf("HostOfRemote(%q) = nil error; want a refusal", in)
		}
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("HostOfRemote(%q) refusal echoes the userinfo secret: %v", in, err)
		}
	}
}
