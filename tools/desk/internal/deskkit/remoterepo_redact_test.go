package deskkit

import (
	"path/filepath"
	"strings"
	"testing"
)

// redactFixtures are remote strings that FAIL to parse and carry a userinfo credential, one
// per remote shape the parser reads. The credential is the literal word "secret"; every
// error ParseRemoteRepo returns for them must omit it while still naming the host and path.
var redactFixtures = []struct{ name, in, keep string }{
	{"https single segment", "https://user:secret@example.invalid/repo.git", "example.invalid/repo.git"},
	{"https no path", "https://user:secret@example.invalid", "example.invalid"},
	{"ssh url, credential as user", "ssh://secret@github.com:22/repo.git", "github.com:22/repo.git"},
	{"scp-like", "user:secret@example.invalid:repo", "example.invalid:repo"},
	{"bare userinfo", "user:secret@example.invalid/repo", "example.invalid/repo"},
	{"hybrid owner slot", "https://example.invalid/user:secret@alias:repo.git", "alias:repo.git"},
	// A password holding ':' splits into the owner slot with no '@' left in that slot.
	{"password in owner slot", "user:secret:x@example.invalid", "example.invalid"},
}

// regression: #6621 (security review A2)
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
// text unchanged (it passes on the unfixed parser too), so the redaction touches only the
// credential-bearing span.
func TestParseRemoteRepoCleanText(t *testing.T) {
	_, _, err := ParseRemoteRepo("https://example.invalid/repo.git")
	want := `cannot parse owner/repo from remote "https://example.invalid/repo.git": expected a trailing <owner>/<repo> (`
	if err == nil || !strings.HasPrefix(err.Error(), want) || strings.Contains(err.Error(), "<redacted>") {
		t.Fatalf("clean-remote error text changed:\n got: %v\nwant prefix: %s", err, want)
	}
	_, _, err = ParseRemoteRepo("https://example.invalid/bad!owner/repo")
	want = `cannot parse owner/repo from remote "https://example.invalid/bad!owner/repo": the trailing pair "bad!owner"/"repo" is not a clean`
	if err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Fatalf("clean-remote error text changed:\n got: %v\nwant prefix: %s", err, want)
	}
}

// regression: #6696
func TestRedactSlashInPassword(t *testing.T) {
	cases := []struct{ in, want string }{
		// An unencoded '/' in the password must not split the span: no part of it survives.
		{"https://user:sec/ret-tail@example.invalid/o/r.git", "https://<redacted>@example.invalid/o/r.git"},
		{"user:sec/ret-tail@example.invalid:o/r.git", "<redacted>@example.invalid:o/r.git"},
		// A list joined by ", " or newlines is masked remote by remote.
		{"https://a:secret@github.com/o/r, ssh://git@github.com/o/r", "https://<redacted>@github.com/o/r, ssh://<redacted>@github.com/o/r"},
		{"origin is user:sec@example.invalid:o/r\nand https://b:secret@example.invalid/o/r", "origin is <redacted>@example.invalid:o/r\nand https://<redacted>@example.invalid/o/r"},
		// A quoted remote in git's own stderr keeps its quote and the prose before it.
		{"fatal: unable to access 'https://a:secret@example.invalid/o/r/': denied", "fatal: unable to access 'https://<redacted>@example.invalid/o/r/': denied"},
		{"https://example.invalid/o/r.git", "https://example.invalid/o/r.git"},
	}
	for _, c := range cases {
		if got := RedactRemoteUserinfo(c.in); got != c.want {
			t.Errorf("RedactRemoteUserinfo(%q)\n got %q\nwant %q", c.in, got, c.want)
		}
	}
}

// regression: #6696
// A quote, bracket, backtick or "://" INSIDE a password must not cut the masked span short:
// no prefix of the password (here "pre") survives, while an opening quote that begins the
// token and a real scheme stay visible.
func TestRedactPunctInPassword(t *testing.T) {
	// Each input is a template: the literal carries the placeholder PW and the punctuated
	// password is spliced in at run time, so no source line reads as a real userinfo password.
	cases := []struct{ in, pw, want string }{
		{"https://user:PW@example.invalid/o/r", "pre'tail", "https://<redacted>@example.invalid/o/r"},
		{"https://user:PW@example.invalid/o/r", "pre(tail", "https://<redacted>@example.invalid/o/r"},
		{"https://user:PW@example.invalid/o/r", "pre[tail", "https://<redacted>@example.invalid/o/r"},
		{"https://user:PW@example.invalid/o/r", "pre`tail", "https://<redacted>@example.invalid/o/r"},
		{"https://user:PW@example.invalid/o/r", "pre<tail", "https://<redacted>@example.invalid/o/r"},
		{"https://user:PW@example.invalid/o/r", "pre://tail", "https://<redacted>@example.invalid/o/r"},
		{"user:PW@example.invalid:o/r", "pre://tail", "<redacted>@example.invalid:o/r"},
		{"user:PW@example.invalid:o/r", "pre'tail", "<redacted>@example.invalid:o/r"},
		{"'user:PW@example.invalid:o/r'", "pre(tail", "'<redacted>@example.invalid:o/r'"},
		{"remote=https://pretail@example.invalid/o/r", "", "remote=https://<redacted>@example.invalid/o/r"},
		{"(https://pretail@example.invalid/o/r)", "", "(https://<redacted>@example.invalid/o/r)"},
	}
	for _, c := range cases {
		in := strings.Replace(c.in, ":PW@", ":"+c.pw+"@", 1)
		got := RedactRemoteUserinfo(in)
		if got != c.want || strings.Contains(got, "pre") {
			t.Errorf("RedactRemoteUserinfo(%q)\n got %q\nwant %q", in, got, c.want)
		}
	}
	keys := map[string]string{
		"url.https://user:" + "pre'tail" + "@example.invalid/.insteadof": "url.https://<redacted>@example.invalid/.insteadof",
		"url.user:pretail@example.invalid:.pushinsteadof":                "url.<redacted>@example.invalid:.pushinsteadof",
		"url.ssh://git@github.com/.insteadof":                            "url.ssh://git@github.com/.insteadof",
		"url.ssh://pretail@github.com/.insteadof":                        "url.ssh://<redacted>@github.com/.insteadof",
		"remote.origin.pushurl":                                          "remote.origin.pushurl",
	}
	for in, want := range keys {
		if got := RedactRemoteConfigKey(in); got != want {
			t.Errorf("RedactRemoteConfigKey(%q) = %q, want %q", in, got, want)
		}
	}
}

// regression: #6696
func TestRedactArgvAndKeepGit(t *testing.T) {
	got := RedactRemoteArgv([]string{"ls-remote", "--refs", "https://u:secret@example.invalid/o/r.git", "refs/heads/x"})
	if want := "ls-remote --refs https://<redacted>@example.invalid/o/r.git refs/heads/x"; got != want {
		t.Errorf("RedactRemoteArgv = %q, want %q", got, want)
	}
	// The push gate's display keeps only the bare SSH account name "git".
	for in, want := range map[string]string{
		"ssh://git@github.com/o/r.git":      "ssh://git@github.com/o/r.git",
		"git@github.com:o/r.git":            "git@github.com:o/r.git",
		"ssh://git-sec@github.com/o/r.git":  "ssh://<redacted>@github.com/o/r.git",
		"https://u:secret@example.invalid/": "https://<redacted>@example.invalid/",
	} {
		if got := RedactRemoteURL(in); got != want {
			t.Errorf("RedactRemoteURL(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestVerifierGitRedacts runs git -C in a directory that does not exist, so the failure it
// expects never depends on what encloses the temporary root: run in an existing empty
// directory, git discovery walks up to any repository above it and the set-url writes
// there (once, a shared checkout's origin URL).
//
// regression: #6696
func TestVerifierGitRedacts(t *testing.T) {
	const secret = "example-secret-value"
	missing := t.TempDir() + "/example-user:" + secret + "@missing.git"
	_, err := verifierGit(filepath.Join(t.TempDir(), "absent"), "remote", "set-url", "origin", missing)
	if err == nil || !strings.Contains(err.Error(), "set-url origin") {
		t.Fatalf("want a verifier-source failure naming the command, got %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("verifier-source failure echoes the remote URL's userinfo secret: %v", err)
	}
}
