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
	// A URL carrying a password in its userinfo: scheme://user:password@host.
	// Userinfo ends at the first '/', '?' or '#' (RFC 3986), so a port and an
	// '@' in a query or fragment is not a password.
	regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://[^/\s:@?#]+:[^/\s@?#]+@`),
}

// slotValue is the key=value form for the slot words that are only slots as a
// whole word: auth, oauth, bearer, privkey and access-key, left-bounded by
// the start or a non-alphanumeric ("x-auth", "basic_auth", but not
// "pallbearer"). Group 1 is the value.
var slotValue = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(?:o?auth|bearer|privkey|access[_-]?key(?:[_-]?id)?)["']?\s*[:=]\s*(\S{8,})`)

// benignSlotValues are values that state a setting, never a secret, for the
// slot words above ("auth: disabled").
var benignSlotValues = map[string]bool{
	"disabled": true, "required": true, "optional": true, "external": true,
	"internal": true, "enabled": true, "inherit": true,
}

// basicAuth finds an HTTP Basic credential: the scheme word followed by a
// base64 run. It counts only when the run decodes to a user:password pair, so
// prose such as "basic functionality" is not a credential.
var basicAuth = regexp.MustCompile(`(?i)\bbasic\s+([A-Za-z0-9+/]{4,}={0,2})`)

// benignSlotValue reports whether the text after a slot word is a bare setting
// word. In JSON text the value runs to its closing quote, so the word is read
// up to the first double quote.
func benignSlotValue(v string) bool {
	v = strings.TrimLeft(v, `"'`)
	if i := strings.IndexByte(v, '"'); i >= 0 {
		v = v[:i]
	}
	return benignSlotValues[strings.ToLower(strings.TrimRight(v, `,;.)}]'`))]
}

// LooksLikeCredential reports whether s contains credential-shaped content.
func LooksLikeCredential(s string) bool {
	for _, p := range credentialPatterns {
		if p.MatchString(s) {
			return true
		}
	}
	for _, m := range slotValue.FindAllStringSubmatch(s, -1) {
		if !benignSlotValue(m[1]) {
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
}

// credentialKeySlots name a credential slot only as whole words of the key (the
// key is split on anything but a letter or digit, and at a lower-to-upper
// camelCase step): "auth" is a slot in "x-auth" or "basic_auth", but "author"
// and "authority" are not credentials, and "access_key" is a slot in
// "aws_access_key_id" or "accessKeyId" but not in "access_key_rotation_days".
var credentialKeySlots = [][]string{
	{"auth"}, {"oauth"}, {"bearer"}, {"privkey"},
	{"accesskey"}, {"accesskeyid"}, {"access", "key"},
}

// benignKeyQualifiers are the words that, trailing a slot word, make the key
// describe the slot rather than hold it: "auth_method", "oauth_scopes",
// "bearer_format", "access_key_rotation_days". A trailing word outside this
// set ("auth_header", "access_key_id") leaves the key a slot.
var benignKeyQualifiers = map[string]bool{
	"method": true, "methods": true, "required": true, "scope": true,
	"scopes": true, "format": true, "type": true, "mode": true,
	"enabled": true, "rotation": true, "days": true, "ttl": true,
	"expiry": true, "timeout": true, "provider": true, "realm": true,
	"version": true, "policy": true,
}

// keyWords splits a key into lower-case words.
func keyWords(k string) []string {
	var b strings.Builder
	var prev rune
	for _, r := range k {
		if r >= 'A' && r <= 'Z' && prev >= 'a' && prev <= 'z' || r >= 'A' && r <= 'Z' && prev >= '0' && prev <= '9' {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
		prev = r
	}
	return strings.FieldsFunc(strings.ToLower(b.String()), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
}

// CredentialKey reports whether an extension key names a credential slot.
func CredentialKey(k string) bool {
	lk := strings.ToLower(k)
	for _, w := range credentialKeyWords {
		if strings.Contains(lk, w) {
			return true
		}
	}
	words := keyWords(k)
	for _, slot := range credentialKeySlots {
	scan:
		for i := 0; i+len(slot) <= len(words); i++ {
			for j, w := range slot {
				if words[i+j] != w {
					continue scan
				}
			}
			for _, rest := range words[i+len(slot):] {
				if !benignKeyQualifiers[rest] {
					return true
				}
			}
			if i+len(slot) == len(words) {
				return true
			}
			// Every trailing word is a qualifier: the key describes the slot.
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
