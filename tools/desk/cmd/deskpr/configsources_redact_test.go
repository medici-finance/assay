package main

import (
	"os/exec"
	"strings"
	"testing"
)

// cfgSecret is the credential half of a userinfo planted in the push-destination fixtures. It
// is an obviously synthetic placeholder; the assertions only need a findable substring.
const cfgSecret = "example-secret-value"

func cfgRepo(t *testing.T, kv ...string) string {
	t.Helper()
	dir := t.TempDir()
	cmds := [][]string{{"init", "-q", dir}}
	for i := 0; i+1 < len(kv); i += 2 {
		cmds = append(cmds, []string{"-C", dir, "config", "--add", kv[i], kv[i+1]})
	}
	for _, args := range cmds {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func cfgNoSecret(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("want a refusal containing %q, got %v", want, err)
	}
	if !strings.Contains(err.Error(), "where git reads it from") {
		t.Fatalf("refusal lacks the config-sources block under test: %v", err)
	}
	if strings.Contains(err.Error(), cfgSecret) {
		t.Fatalf("refusal echoes a planted userinfo secret:\n%v", err)
	}
}

// regression: #6696
// Two push URLs (one https, one scp-like, both carrying userinfo) plus insteadOf rules whose
// BASE carries userinfo: the refusal lists every value AND every rule key, and none may print
// the secret.
func TestPushDestKeysRedacted(t *testing.T) {
	dir := cfgRepo(t,
		"remote.origin.url", "https://example.invalid/example-org/example-repo.git",
		"remote.origin.pushurl", "https://example-user:"+cfgSecret+"@example.invalid/example-org/example-repo.git",
		"remote.origin.pushurl", "example-user:"+cfgSecret+"@example.invalid:example-org/example-repo.git",
		"url.https://example-user:"+cfgSecret+"@example.invalid/.insteadOf", "https://example.org/",
		"url.example-user:"+cfgSecret+"@example.invalid:.insteadOf", "example-alias:")
	err := pushDestinationGate(dir, "update", "example-org/example-repo",
		"https://example.invalid/example-org/example-repo.git")
	cfgNoSecret(t, err, "url.")
}

// regression: #6696
// The multi-fetch-URL refusal appends the same config-sources listing.
func TestMultiFetchKeysRedacted(t *testing.T) {
	dir := cfgRepo(t,
		"remote.origin.url", "https://example-user:"+cfgSecret+"@example.invalid/example-org/example-repo.git",
		"remote.origin.url", "example-user:"+cfgSecret+"@example.invalid:other-org/other-repo.git",
		"url.https://example-user:"+cfgSecret+"@example.invalid/.insteadOf", "https://example.org/")
	_, err := effectiveOriginURL(dir)
	cfgNoSecret(t, err, "fetch URLs")
}
