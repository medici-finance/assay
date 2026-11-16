package main

import (
	"os/exec"
	"strings"
	"testing"
)

// plantedOriginSecret is the credential half of the userinfo planted in the origin URL. It is
// an obviously synthetic placeholder; the assertion only needs a findable substring.
const plantedOriginSecret = "example-secret-value"

// redactFixtureRepo makes a git repo with one commit whose origin is url.
func redactFixtureRepo(t *testing.T, url string) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main", dir},
		{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "--allow-empty", "-m", "seed"},
		{"-C", dir, "remote", "add", "origin", url},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

// assertNoOriginSecret fails when err is nil or its text carries the planted secret.
func assertNoOriginSecret(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("want a refusal for an unparseable origin, got nil")
	}
	if !strings.Contains(err.Error(), "cannot parse origin repo") {
		t.Fatalf("refusal is not the origin refusal under test: %v", err)
	}
	if strings.Contains(err.Error(), plantedOriginSecret) {
		t.Fatalf("refusal echoes the origin URL's userinfo secret: %v", err)
	}
}

// regression: #6696
func TestCurrentRepoRedactsOrigin(t *testing.T) {
	_, err := currentRepo(redactFixtureRepo(t, "https://example-user:"+plantedOriginSecret+"@example.invalid/only-one"))
	assertNoOriginSecret(t, err)
}
