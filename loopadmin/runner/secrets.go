package runner

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"
)

// Credential exclusion. Packets, results and extensions never carry secrets:
// the runner's credentials are installed by the operator outside the model's
// reach, and nothing the caller or the model writes into a packet may smuggle
// one in. The check is deliberately conservative pattern matching; it is a
// floor, not a scanner replacement. It does not defeat deliberate obfuscation
// (a credential split across strings, or re-encoded); see spec section 3.
var credentialPatterns = []*regexp.Regexp{
	regexp.MustCompile(`-{5}BEGIN [A-Z ]*PRIVATE KEY-{5}`),
	regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}`),
	regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}`),
	regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
	regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}`),
	regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]{20,}`),
	// key=value and "key": "value" forms, including a quoted JSON key.
	regexp.MustCompile(`(?i)(api[_-]?key|secret|passw(or)?d|token|authorization|cookie)["']?\s*[:=]\s*\S{8,}`),
	regexp.MustCompile(`(?i)(auth|bearer|privkey|access[_-]?key(?:[_-]?id)?)["']?\s*[:=]\s*\S{8,}`),
	// A URL carrying a password in its userinfo: scheme://user:password@host.
	regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://[^/\s:@]+:[^/\s@]+@`),
}

// basicAuth finds an HTTP Basic credential: the scheme word followed by a
// base64 run. It counts only when the run decodes to a user:password pair, so
// prose such as "basic functionality" is not a credential.
var basicAuth = regexp.MustCompile(`(?i)\bbasic\s+([A-Za-z0-9+/]{4,}={0,2})`)

// LooksLikeCredential reports whether s contains credential-shaped content.
func LooksLikeCredential(s string) bool {
	for _, p := range credentialPatterns {
		if p.MatchString(s) {
			return true
		}
	}
	for _, m := range basicAuth.FindAllStringSubmatch(s, -1) {
		if b, err := base64.StdEncoding.DecodeString(m[1]); err == nil && bytes.IndexByte(b, ':') > 0 {
			return true
		}
	}
	return false
}

// credentialKeyWords are the substrings that make a key name a credential slot.
var credentialKeyWords = []string{
	"token", "secret", "password", "passwd", "passphrase", "credential",
	"apikey", "api_key", "api-key", "private_key", "privatekey", "private-key",
	"authorization", "cookie", "session",
	"bearer", "privkey", "access_key", "accesskey", "access-key",
}

// credentialKeyTokens name a credential slot only as a whole word of the key
// (split on anything but a letter or digit): "auth" is a slot in "x-auth" or
// "basic_auth", but "author" and "authority" are not credentials.
var credentialKeyTokens = []string{"auth", "oauth"}

// CredentialKey reports whether an extension key names a credential slot.
func CredentialKey(k string) bool {
	k = strings.ToLower(k)
	for _, w := range credentialKeyWords {
		if strings.Contains(k, w) {
			return true
		}
	}
	words := strings.FieldsFunc(k, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
	for _, w := range words {
		for _, tok := range credentialKeyTokens {
			if w == tok {
				return true
			}
		}
	}
	return false
}

// extensionCarriesCredential reports whether one extension, its key or its
// value, carries credential material. The value is DECODED and walked: every
// object key at every depth is checked as a key, and every decoded string as a
// value, so a nested slot or a JSON-escaped token is seen as written. A value
// that does not decode fails closed.
func extensionCarriesCredential(k string, v json.RawMessage) bool {
	if CredentialKey(k) || LooksLikeCredential(k) || LooksLikeCredential(string(v)) {
		return true
	}
	dec := json.NewDecoder(bytes.NewReader(v))
	dec.UseNumber()
	var x any
	if err := dec.Decode(&x); err != nil {
		return true
	}
	return valueCarriesCredential(x)
}

func valueCarriesCredential(x any) bool {
	switch t := x.(type) {
	case string:
		return LooksLikeCredential(t)
	case []any:
		for _, e := range t {
			if valueCarriesCredential(e) {
				return true
			}
		}
	case map[string]any:
		for k, e := range t {
			if CredentialKey(k) || LooksLikeCredential(k) || valueCarriesCredential(e) {
				return true
			}
		}
	}
	return false
}
