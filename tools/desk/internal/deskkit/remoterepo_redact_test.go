package deskkit

import (
	"strings"
	"testing"
)

// redactFixtures are remote strings that FAIL to parse and carry a userinfo credential, one
// per remote shape the parser reads. The credential is the literal word "secret"; every
// error ParseRemoteRepo returns for them must omit it while still naming the host and path.
var redactFixtures = []struct{ name, in, keep string }{
	{"https single segment", "https://user:secret@example.invalid/repo.git", "example.invalid/repo.git"},
	{"https no path", "https://user:secret@example.invalid", "example.invalid"},
	{"ssh url with port", "ssh://user:secret@example.invalid:22/repo.git", "example.invalid:22/repo.git"},
	{"scp-like", "user:secret@example.invalid:repo.git", "example.invalid:repo.git"},
	{"bare userinfo", "user:secret@example.invalid/repo", "example.invalid/repo"},
	{"hybrid owner slot", "https://example.invalid/user:secret@alias:repo.git", "alias:repo.git"},
}

// regression: #6606
func TestParseRemoteRepoRedacts(t *testing.T) {
	for _, c := range redactFixtures {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := ParseRemoteRepo(c.in)
			if err == nil {
				t.Fatalf("ParseRemoteRepo(%q) = nil error; want a refusal", c.in)
			}
			msg := err.Error()
			if strings.Contains(msg, "secret") {
				t.Fatalf("error echoes the userinfo credential: %s", msg)
			}
			if !strings.Contains(msg, "<redacted>@") || !strings.Contains(msg, c.keep) {
				t.Fatalf("error lost the remote's shape (want <redacted>@ and %q): %s", c.keep, msg)
			}
		})
	}
	if _, err := RemoteRepoSlug(redactFixtures[0].in); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("RemoteRepoSlug error echoes the credential: %v", err)
	}
}

// TestParseRemoteRepoCleanText is the control: a remote with no userinfo keeps its error
// text byte-for-byte, so the redaction touches only the credential-bearing span.
func TestParseRemoteRepoCleanText(t *testing.T) {
	_, _, err := ParseRemoteRepo("https://example.invalid/repo.git")
	want := `cannot parse owner/repo from remote "https://example.invalid/repo.git": expected a ` +
		`trailing <owner>/<repo> (git@host:owner/repo, ssh://host/owner/repo, ` +
		`https://host/owner/repo, or host:owner/repo)`
	if err == nil || err.Error() != want {
		t.Fatalf("clean-remote error text changed:\n got: %v\nwant: %s", err, want)
	}
}
