package deskkit

import (
	"errors"
	"strings"
	"testing"
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
		"ssh scheme":      "ssh://deploy:" + uiPw + "@github.com/o/r",
		"ftp scheme":      "fetch ftp://anon:" + uiPw + "@ftp.example.com/pub",
		"db dsn":          "postgres://app:" + uiPw + "@127.0.0.1:5432/app",
		"numeric pw":      "https://user:" + "4815" + "1623" + "@example.com/x",
		"second url":      "see https://example.com/a and https://u:" + uiPw + "@example.com/b",
		"later line":      "line one\nline two https://u:" + uiPw + "@example.com\n",
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
