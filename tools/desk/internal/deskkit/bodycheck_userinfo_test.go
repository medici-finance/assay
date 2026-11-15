package deskkit

import (
	"errors"
	"math/rand"
	"strings"
	"testing"
	"time"
)

// The url-userinfo fixtures are built by concatenation, with a space on each side of every
// '+', so no line of this file holds a userinfo-password URL the arm would match.
const (
	uiPw     = "fixture-" + "pw"
	uiRepoPw = "fixture-" + "token" // the password of the remote-userinfo boundary row
)

// regression: url-userinfo arm
// TestBodyCheckURLUserinfo: a URL whose userinfo carries a password refuses under the
// url-userinfo rule; placeholders, username-only forms and matches that leave the URL's
// authority do not.
func TestBodyCheckURLUserinfo(t *testing.T) {
	refused := map[string]string{
		"remote userinfo": "https://fixture-user:" + uiRepoPw + "@github.com/example-org/project.git",
		"oauth2 user":     "git push https://oauth2:" + uiPw + "@gitlab.example.com/o/r.git main",
		"sftp scheme":     "sftp://deploy:" + uiPw + "@files.example.com/drop",
		"ftp scheme":      "fetch ftp://anon:" + uiPw + "@ftp.example.com/pub",
		"db dsn":          "postgres://app:" + uiPw + "@127.0.0.1:5432/app",
		"numeric pw":      "https://user:" + "4815" + "1623" + "@example.com/x",
		"second url":      "see https://example.com/a and https://u:" + uiPw + "@example.com/b",
		"later line":      "line one\nline two https://u:" + uiPw + "@example.com\n",
		"empty username":  "redis://:" + uiPw + "@cache.example.com:6379/0",
		// A URL nested in a longer one is judged on its own userinfo.
		"nested in path":    "https://example.com:8443/redir/https://u:" + uiPw + "@example.org/r",
		"nested in query":   "https://example.com:8443/cb?return=https://u:" + uiPw + "@example.org/r",
		"nested in frag":    "https://example.com:8443/cb#https://u:" + uiPw + "@example.org/r",
		"nested after port": "see https://example.com:8443/https://u:" + uiPw + "@example.org",
		"nested twice":      "https://a.example:1/https://b.example:2/https://u:" + uiPw + "@example.org",
		"nested no user":    "https://example.com:8443/cb?return=redis://:" + uiPw + "@cache.example.com",
		"nested later line": "https://example.com:8443/ok\nhttps://example.com:8443/cb?next=https://u:" + uiPw + "@h\n",
	}
	for name, text := range refused {
		err := BodyCheck([]byte(text))
		if !IsRefused(err) {
			t.Errorf("%s: BodyCheck = %v, want Refused", name, err)
			continue
		}
		var f *ScanFinding
		if !errors.As(err, &f) || f.Rule != "url-userinfo" {
			t.Errorf("%s: finding = %+v, want rule url-userinfo", name, f)
			continue
		}
		pw := uiPw
		if name == "remote userinfo" {
			pw = uiRepoPw
		}
		if strings.Contains(err.Error(), pw) || strings.Contains(f.Explain(), pw) {
			t.Errorf("%s: refusal or finding carries the password: %v / %s", name, err, f.Explain())
		}
	}
	if err := BodyCheck([]byte(refused["later line"])); err != nil {
		var f *ScanFinding
		if errors.As(err, &f) && f.Line != 2 {
			t.Errorf("later line: finding line = %d, want 2", f.Line)
		}
	}

	clean := map[string]string{
		"no password":       "https://user@example.com/o/r",
		"username only":     "https://x-access-token@github.com/o/r.git",
		"scp form":          "git@github.com:example-org/project.git",
		"angle placeholder": "https://x-access-token:<token>@github.com/o/r.git",
		"brace var":         "https://user:${URL_PW}@example.com/o/r",
		"dollar var":        "https://user:$URL_PW@example.com/o/r",
		"command subst":     "https://user:$(pass show x)@example.com/o/r",
		"template":          "https://user:{{ .Pw }}@example.com/o/r",
		"mask":              "https://user:***@example.com/o/r",
		"elision":           "https://x-access-token:…@github.com/o/r",
		"dots":              "https://user:...@example.com/o/r",
		"redacted":          "https://user:REDACTED@example.com/o/r",
		"field word":        "usage: scheme://user:password@host/path",
		"port and path":     "https://example.com:8443/team@example/r",
		"query":             "https://example.com?next=a:b@example.org",
		"fragment":          "https://example.com#a:b@example.org",
		"regex":             `^https://(?:[A-Za-z0-9._-]+:[^@]+@)?github\.com/`,
		"concatenation":     `s = reURL.ReplaceAllString(s, "://$1:"+marker+"@")`,
		"spaced concat":     `u := "https://user:" + pw + "@example.com"`,
		"empty user, angle": "redis://:<password>@cache.example.com:6379/0",
		"empty user, var":   "amqp://:${MQ_PW}@mq.example.com/vhost",
		"empty user, mask":  "redis://:***@cache.example.com:6379",
		// Nested URLs whose own userinfo is not a credential.
		"nested placeholder": "https://example.com:8443/cb?return=https://u:<token>@example.org/r",
		"nested mask":        "https://example.com:8443/cb?return=https://u:***@example.org/r",
		"nested shell var":   "https://example.com:8443/redir/https://u:${URL_PW}@example.org/r",
		"nested no password": "https://example.com:8443/cb?return=https://u@example.org/r",
		"nested no at":       "https://example.com:8443/cb?return=https://example.org/r",
		"nested path at":     "https://example.com:8443/cb?return=https://example.org/a:b/c@d",
	}
	for name, text := range clean {
		if err := BodyCheck([]byte(text)); err != nil {
			t.Errorf("%s: BodyCheck(%q) = %v, want nil", name, text, err)
		}
	}
}

// regression: url-userinfo arm
// TestBodyCheckUserinfoArmOrder: the arm runs after every other arm, so a token-prefixed
// password keeps its github-token rule id, and ScanSurfaceSecrets (the outbound check's
// secret pass) reports the arm under its own rule id.
func TestBodyCheckUserinfoArmOrder(t *testing.T) {
	var f *ScanFinding
	err := BodyCheck([]byte("https://x-access-token:" + scanGHToken + "@github.com/o/r"))
	if !errors.As(err, &f) || f.Rule != "github-token" {
		t.Errorf("token-prefixed password: finding = %+v, want github-token", f)
	}
	f = nil
	err = ScanSurfaceSecrets("title", []byte("mirror https://u:"+uiPw+"@example.com/r"))
	if !IsRefused(err) || !errors.As(err, &f) || f.Rule != "url-userinfo" {
		t.Errorf("ScanSurfaceSecrets = %v (finding %+v), want url-userinfo refusal", err, f)
	}
	if err != nil && !strings.Contains(err.Error(), "refused: title contains a URL") {
		t.Errorf("refusal does not name its surface: %v", err)
	}
}

// naiveUserinfoAt is the reference urlUserinfoAt is held to: after a skipped match it
// resumes just after that match's "://", one "://" at a time.
func naiveUserinfoAt(s string) (int, string) {
	for pos := 0; pos < len(s); {
		m := reURLUserinfo.FindStringSubmatchIndex(s[pos:])
		if m == nil {
			break
		}
		for i := range m {
			m[i] += pos
		}
		user, pw := s[m[2]:m[3]], s[m[4]:m[5]]
		if !strings.ContainsAny(user+pw, "/?#[]") && !isUserinfoPlaceholder(pw) {
			return m[0], pw
		}
		pos = m[0] + len("://")
	}
	return -1, ""
}

// regression: url-userinfo arm
// TestBodyCheckUserinfoResumeEquiv: the one-step resume after a skipped match reaches the
// same verdict and offset as resuming at every "://" in turn, over random strings built from
// the characters the pattern and its skip rules turn on.
func TestBodyCheckUserinfoResumeEquiv(t *testing.T) {
	pieces := []string{"://", "://", ":", "@", "/", "?", "#", "[", "]", "a", "bb", "8080",
		" ", "\n", "***", "<t>", "${V}", "pw1", "-", "."}
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 20000; i++ {
		var b strings.Builder
		for n := rng.Intn(14); n >= 0; n-- {
			b.WriteString(pieces[rng.Intn(len(pieces))])
		}
		in := b.String()
		wantAt, wantPw := naiveUserinfoAt(in)
		gotAt, gotPw := urlUserinfoAt(in)
		if gotAt != wantAt || gotPw != wantPw {
			t.Fatalf("urlUserinfoAt(%q) = (%d, %q), reference = (%d, %q)", in, gotAt, gotPw, wantAt, wantPw)
		}
	}
}

// regression: url-userinfo arm
// TestBodyCheckUserinfoRepeatedColon: a long run of repeated "://a:" fragments, with and
// without a closing '@', ends in time linear in its length.
func TestBodyCheckUserinfoRepeatedColon(t *testing.T) {
	frag := strings.Repeat("://a:", 60000)
	for name, in := range map[string]string{
		"no at":        "https" + frag,
		"closing at":   "https" + frag + "@h",
		"credential":   "https" + frag + "/https://u:" + uiPw + "@h",
		"placeholder":  "https" + frag + "/https://u:***@h",
		"many at":      "https" + strings.Repeat("://a:@", 60000),
		"many slashes": "https" + strings.Repeat("://a:/", 60000) + "@",
	} {
		start := time.Now()
		at, pw := urlUserinfoAt(in)
		if d := time.Since(start); d > 10*time.Second {
			t.Errorf("%s: urlUserinfoAt took %v on %d bytes", name, d, len(in))
		}
		wantCred := name == "credential"
		if (at >= 0) != wantCred || (wantCred && pw != uiPw) {
			t.Errorf("%s: urlUserinfoAt = (%d, %q), want credential=%v", name, at, pw, wantCred)
		}
	}
}
