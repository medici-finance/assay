package deskkit

import (
	"bytes"
	"strings"
	"testing"
)

// gateSecret is the credential half of a userinfo planted in the push gate's config fixtures.
// It is an obviously synthetic placeholder; the assertions only need a findable substring.
const gateSecret = "example-secret-value"

const gateHTTPS = "https://example-user:" + gateSecret + "@github.com/"

// gateNoSecret fails when got does not carry want or carries the planted secret.
func gateNoSecret(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("want gate output containing %q, got:\n%s", want, got)
	}
	if strings.Contains(got, gateSecret) {
		t.Fatalf("gate output echoes the remote URL's userinfo secret:\n%s", got)
	}
}

// regression: #6696
func TestGateNoticeRedacts(t *testing.T) {
	t.Setenv("DESK_LOOP", "worker-desk")
	var errb bytes.Buffer
	in := gateInput(t, configZ("remote.origin.url", gateHTTPS+"example-org/tracker.git"))
	in.Stderr = &errb
	if err := CheckPushTransport(in); err != nil {
		t.Fatalf("CheckPushTransport = %v, want nil (NOTICE)", err)
	}
	gateNoSecret(t, errb.String(), "NOTICE")
	gateNoSecret(t, errb.String(), "github.com/example-org/tracker.git")
}

// regression: #6696
func TestGateSSHRefusalRedacts(t *testing.T) {
	t.Setenv("DESK_LOOP", "worker-desk")
	in := gateInput(t, configZ("remote.origin.url",
		"ssh://"+gateSecret+"@github.com/example-org/tracker.git"))
	err := CheckPushTransport(in)
	if err == nil {
		t.Fatal("want a refusal for an SSH push url")
	}
	gateNoSecret(t, err.Error(), "over an SSH transport")
}

// regression: #6696
func TestGateUnattributedRedacts(t *testing.T) {
	t.Setenv("DESK_LOOP", "worker-desk")
	in := gateInput(t, configZ("remote.origin.url", gateHTTPS+"example-org/tracker.git"))
	in.PushURLs = func() (string, error) {
		return "ssh://" + gateSecret + "@ssh.github.com/example-org/tracker.git", nil
	}
	err := CheckPushTransport(in)
	if err == nil {
		t.Fatal("want a refusal for an SSH push url")
	}
	gateNoSecret(t, err.Error(), "could not attribute")
}

// regression: #6696
func TestGateRewriteRedacts(t *testing.T) {
	t.Setenv("DESK_LOOP", "worker-desk")
	in := gateInput(t, configZ(
		"remote.origin.url", gateHTTPS+"example-org/tracker.git",
		"url.ssh://git@github.com/.insteadof", gateHTTPS))
	in.PushURLs = func() (string, error) { return "ssh://git@github.com/example-org/tracker.git", nil }
	err := CheckPushTransport(in)
	if err == nil {
		t.Fatal("want a refusal for an insteadOf rewrite to SSH")
	}
	// The rule key, the rewritten url and the remedy all keep the SSH account name "git".
	gateNoSecret(t, err.Error(), "is rewritten by the rule url.ssh://git@github.com/.insteadOf")
	gateNoSecret(t, err.Error(), "remove or narrow the rule")
}

// regression: #6696
func TestGateSetURLRedacts(t *testing.T) {
	cfg := parseConfigZ(configZ(
		"remote.origin.pushurl", gateHTTPS+"example-org/tracker.git",
		"remote.origin.pushurl", "ssh://git@github.com/example-org/tracker.git"))
	got := setPushURLCommand(cfg, "/w", "origin", gateHTTPS+"example-org/tracker.git", gateHTTPS+"example-org/tracker.git")
	gateNoSecret(t, got, "remote set-url --push origin")
}
