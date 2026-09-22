package deskkit

import "testing"

// TestParseRemoteRepo pins the parser against every remote shape git accepts plus the
// rewritten/hybrid forms an `insteadOf` config or a bad URL-composition bakes into
// remote.origin.url. The hybrid case is issue 1470's field failure: go-git's RemoteURL
// returns the RAW configured value (no insteadOf expansion), so a checkout whose origin was
// composed as `https://github.com/` + an scp string handed the old verb-local parser a
// string it split on the first '@' and first '/', yielding a single "example-repo" segment
// and the "cannot parse owner/repo" refusal misreported upstream as "branch already exists".
//
// DECISION (documented, per the brief): the hybrid parses LENIENTLY to owner/repo when the
// trailing pair is unambiguous — the rewrite is the operator's own config and the tail is
// not in doubt. A hybrid whose owner slot still carries a `user@host` token IS refused.
func TestParseRemoteRepo(t *testing.T) {
	ok := []struct {
		name, in, want string
	}{
		{"https .git", "https://github.com/example-org/example-repo.git", "example-org/example-repo"},
		{"https no suffix", "https://github.com/example-org/example-repo", "example-org/example-repo"},
		{"https trailing slash", "https://github.com/example-org/example-repo/", "example-org/example-repo"},
		{"scp user@host", "git@github.com:example-org/example-repo.git", "example-org/example-repo"},
		{"scp alias host", "git@github-alias:example-org/example-repo.git", "example-org/example-repo"},
		{"scp alias no user", "github-alias:example-org/example-repo.git", "example-org/example-repo"},
		{"ssh url", "ssh://git@github.com/example-org/example-repo.git", "example-org/example-repo"},
		{"ssh url with port", "ssh://git@github.com:22/example-org/example-repo.git", "example-org/example-repo"},
		{"ssh alias url", "ssh://git@github-alias/example-org/example-repo.git", "example-org/example-repo"},
		{"bare path", "example-org/example-repo", "example-org/example-repo"},
		// The issue-1470 hybrid: base URL prepended onto an scp string. Lenient parse.
		{"hybrid https+scp alias", "https://github.com/git@github-alias:example-org/example-repo.git", "example-org/example-repo"},
		{"hybrid ssh-rewrite+scp alias", "ssh://git@github.com/git@github-alias:example-org/example-repo.git", "example-org/example-repo"},
	}
	for _, c := range ok {
		t.Run(c.name, func(t *testing.T) {
			owner, repo, err := ParseRemoteRepo(c.in)
			if err != nil {
				t.Fatalf("ParseRemoteRepo(%q) errored: %v; want %q", c.in, err, c.want)
			}
			if got := owner + "/" + repo; got != c.want {
				t.Fatalf("ParseRemoteRepo(%q) = %q; want %q", c.in, got, c.want)
			}
		})
	}

	bad := []struct{ name, in string }{
		{"empty", ""},
		{"blank", "   "},
		{"single segment", "example-repo"},
		{"single segment url", "https://github.com/example-repo"},
		// A hybrid whose tail carries NO clean owner — the owner slot still holds
		// `user@host`, so the trailing pair is ambiguous and we refuse (naming the string).
		{"hybrid missing owner", "https://github.com/git@github-alias:example-repo.git"},
	}
	for _, c := range bad {
		t.Run("refuse/"+c.name, func(t *testing.T) {
			owner, repo, err := ParseRemoteRepo(c.in)
			if err == nil {
				t.Fatalf("ParseRemoteRepo(%q) = %q/%q, nil; want a refusal", c.in, owner, repo)
			}
		})
	}
}

// TestRemoteRepoSlug is the combined-slug wrapper the verb call sites use.
func TestRemoteRepoSlug(t *testing.T) {
	got, err := RemoteRepoSlug("https://github.com/git@github-alias:example-org/example-repo.git")
	if err != nil {
		t.Fatalf("RemoteRepoSlug hybrid errored: %v", err)
	}
	if got != "example-org/example-repo" {
		t.Fatalf("RemoteRepoSlug hybrid = %q; want example-org/example-repo", got)
	}
	if _, err := RemoteRepoSlug("nope"); err == nil {
		t.Fatalf("RemoteRepoSlug(\"nope\") = nil error; want a refusal")
	}
}
