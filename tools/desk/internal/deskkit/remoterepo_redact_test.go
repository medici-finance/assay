package deskkit

import (
	"strings"
	"testing"
)

// plantedSecret is the credential half every userinfo fixture below carries. It is an
// obviously synthetic placeholder; the assertions only need it to be a findable substring.
const plantedSecret = "example-secret-value"

// regression: #6696
func TestRedactURLUserinfo(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"https user:pw", "https://example-user:" + plantedSecret + "@example.invalid/example-org/example-repo.git",
			"https://***@example.invalid/example-org/example-repo.git"},
		{"https bare secret as user", "https://" + plantedSecret + "@example.invalid/o/r",
			"https://***@example.invalid/o/r"},
		{"password holding an at sign", "https://u:" + plantedSecret + "@x@example.invalid/o/r",
			"https://***@example.invalid/o/r"},
		{"hybrid embedded pair", "https://example.invalid/u:" + plantedSecret + "@alias:o/r",
			"https://example.invalid/***@alias:o/r"},
		{"scp user:pw", "u:" + plantedSecret + "@example.invalid:o/r", "***@example.invalid:o/r"},
		{"two URLs in one line", "a https://u:" + plantedSecret + "@h1/o/r, https://v:" + plantedSecret + "@h2/o/r",
			"a https://***@h1/o/r, https://***@h2/o/r"},
		// Kept: no credential to hide.
		{"scp login kept", "git@github.com:example-org/example-repo.git", "git@github.com:example-org/example-repo.git"},
		{"hybrid login kept", "https://github.com/git@github.com:o/r", "https://github.com/git@github.com:o/r"},
		{"path at sign kept", "https://example.invalid/o/r@v1", "https://example.invalid/o/r@v1"},
		{"no userinfo", "https://example.invalid/o/r", "https://example.invalid/o/r"},
		{"empty", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := RedactURLUserinfo(c.in); got != c.want {
				t.Fatalf("RedactURLUserinfo(%q) = %q; want %q", c.in, got, c.want)
			}
		})
	}
}

// regression: #6696
// Every refusal ParseRemoteRepo returns quotes the URL; none may quote the userinfo secret.
// Both refusal arms are covered: too few segments, and an unclean trailing pair (where the
// ':' split leaves the password alone in the owner slot).
func TestParseRemoteRedactsSecret(t *testing.T) {
	for _, in := range []string{
		"https://example-user:" + plantedSecret + "@example.invalid/only-one",
		"https://example-user:" + plantedSecret + "@example.invalid",
		"https://example.invalid/x/u:" + plantedSecret + "@alias:repo",
		"u:" + plantedSecret + "@example.invalid:only-one",
	} {
		_, _, err := ParseRemoteRepo(in)
		if err == nil {
			t.Fatalf("ParseRemoteRepo(%q) = nil error; want a refusal", in)
		}
		if strings.Contains(err.Error(), plantedSecret) {
			t.Errorf("ParseRemoteRepo(%q) refusal leaks the userinfo secret: %v", in, err)
		}
		if _, err := RemoteRepoSlug(in); err == nil || strings.Contains(err.Error(), plantedSecret) {
			t.Errorf("RemoteRepoSlug(%q) refusal leaks the userinfo secret or did not refuse: %v", in, err)
		}
	}
}

// regression: #6696
func TestHostOfRemoteRedacts(t *testing.T) {
	_, err := HostOfRemote("https://example-user:" + plantedSecret + "@/o/r")
	if err == nil || strings.Contains(err.Error(), plantedSecret) {
		t.Fatalf("HostOfRemote refusal leaks the userinfo secret or did not refuse: %v", err)
	}
}
