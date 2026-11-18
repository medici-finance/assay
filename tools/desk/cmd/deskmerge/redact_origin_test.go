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
	if !strings.Contains(err.Error(), "does not name") {
		t.Fatalf("refusal is not the origin refusal under test: %v", err)
	}
	if strings.Contains(err.Error(), plantedOriginSecret) {
		t.Fatalf("refusal echoes the origin URL's userinfo secret: %v", err)
	}
}

// regression: #6696
func TestFetchOriginRedactsOrigin(t *testing.T) {
	root := redactFixtureRepo(t, "https://example-user:"+plantedOriginSecret+"@example.invalid/other-org/other-repo.git")
	assertNoOriginSecret(t, fetchFromOrigin(root, "example-org/example-repo", nil))
}

// assertRedacted fails when err is nil, does not carry want, or carries the planted secret.
func assertRedacted(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("want a refusal containing %q, got %v", want, err)
	}
	if strings.Contains(err.Error(), plantedOriginSecret) {
		t.Fatalf("refusal echoes the origin URL's userinfo secret: %v", err)
	}
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

const plantedHTTPS = "https://example-user:" + plantedOriginSecret + "@example.invalid/"

// regression: #6696
func TestRepoRootRedactsOrigin(t *testing.T) {
	root := redactFixtureRepo(t, plantedHTTPS+"other-org/other-repo.git")
	_, err := resolveRepoRoot("example-org/example-repo", root)
	assertRedacted(t, err, "does not name")
}

// regression: #6696
func TestRepoRootRedactsMultiURL(t *testing.T) {
	root := redactFixtureRepo(t, plantedHTTPS+"example-org/example-repo.git")
	gitIn(t, root, "config", "--add", "remote.origin.url", plantedHTTPS+"other-org/other-repo.git")
	_, err := resolveRepoRoot("example-org/example-repo", root)
	assertRedacted(t, err, "fetch URLs")
}

// regression: #6696
func TestFetchRedactsNonHTTPS(t *testing.T) {
	root := redactFixtureRepo(t, "example-user:"+plantedOriginSecret+"@example.invalid:example-org/example-repo.git")
	assertRedacted(t, fetchFromOrigin(root, "example-org/example-repo", nil), "not an https URL")
}

// regression: #6696
func TestPushGateRedactsMulti(t *testing.T) {
	root := redactFixtureRepo(t, plantedHTTPS+"example-org/example-repo.git")
	gitIn(t, root, "config", "--add", "remote.origin.pushurl", plantedHTTPS+"example-org/example-repo.git")
	gitIn(t, root, "config", "--add", "remote.origin.pushurl", plantedHTTPS+"other-org/other-repo.git")
	assertRedacted(t, gatePushDestinations(root, "example-org/example-repo"), "destinations")
}

// regression: #6696
func TestPushGateRedactsOther(t *testing.T) {
	root := redactFixtureRepo(t, plantedHTTPS+"example-org/example-repo.git")
	gitIn(t, root, "config", "remote.origin.pushurl", plantedHTTPS+"other-org/other-repo.git")
	assertRedacted(t, gatePushDestinations(root, "example-org/example-repo"), "does not name")
}
