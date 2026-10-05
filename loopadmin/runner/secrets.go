package runner

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"
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
}

// userinfoURL finds scheme://user:password@. The user ends at the first '/',
// '?' or '#'; the password may carry '?' or '#' (a password can), so
// passwordInURL decides whether the run is a password or a port.
var userinfoURL = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://[^/\s:@?#]+:([^/\s@]+)@`)

// passwordInURL reports whether the userinfo run after the user's colon is a
// password. A run that is only digits up to its first '?' or '#' is a port with
// an '@' in the query or fragment ("https://host:8080?owner=a@b.example"), not
// a password; any other run is one, '?' and '#' included.
func passwordInURL(run string) bool {
	i := strings.IndexAny(run, "?#")
	if i <= 0 {
		return true
	}
	for _, r := range run[:i] {
		if r < '0' || r > '9' {
			return true
		}
	}
	return false
}

// slotPrefix finds a slot word and its separator: auth, bearer, privkey and
// access-key (with or without an id), wherever they occur in the text, so a
// slot glued to a prefix ("sshauth=", "awsaccesskey=", "userbearer:") is still
// a slot. The value is read separately, from the end of the match, so two slots
// written back to back each carry their own value. Group 1 is the slot word.
var slotPrefix = regexp.MustCompile(`(?i)(auth|bearer|privkey|access[_-]?key(?:[_-]?id)?)["']?\s*[:=]\s*`)

// compoundBearer are the English words that end in "bearer": "pallbearer=..."
// names a person, not a slot. Only these exact compounds are excused.
var compoundBearer = map[string]bool{
	"pall": true, "cup": true, "standard": true, "torch": true,
	"flag": true, "sword": true, "ring": true,
}

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

// slotValueOf returns the value that follows the slot word at s[start:end]'s
// separator: the run up to the next whitespace, and whether it is long enough
// to be a secret.
func slotValueOf(s string, end int) (string, bool) {
	v := s[end:]
	if i := strings.IndexAny(v, " \t\n\f\r"); i >= 0 {
		v = v[:i]
	}
	return v, utf8.RuneCountInString(v) >= 8
}

// benignSlotValue reports whether a slot's value is a bare setting word and
// nothing else. A quoted value is the word between its quotes and may be
// followed only by a separator ("disabled", then a comma or a closing brace),
// so the next field of a JSON object is read as its own slot, never as part of
// this value. An unquoted value may be followed only by closing punctuation.
func benignSlotValue(v string) bool {
	quote := byte(0)
	if v != "" && (v[0] == '"' || v[0] == '\'') {
		quote, v = v[0], v[1:]
	}
	end := 0
	for end < len(v) && (v[end] >= 'a' && v[end] <= 'z' || v[end] >= 'A' && v[end] <= 'Z' || v[end] >= '0' && v[end] <= '9' || v[end] == '_' || v[end] == '-') {
		end++
	}
	if !benignSlotValues[strings.ToLower(v[:end])] {
		return false
	}
	rest := v[end:]
	if quote != 0 {
		if rest == "" || rest[0] != quote {
			return false
		}
		rest = rest[1:]
		return rest == "" || strings.IndexByte(",;.)}]", rest[0]) >= 0
	}
	if rest != "" && (rest[0] == '"' || rest[0] == '\'') {
		// The word ends a string that opened before the slot (a JSON string
		// that says "auth: disabled"): the quote closes it, so a separator or
		// the end must follow, exactly as for a quoted value.
		rest = rest[1:]
		return rest == "" || strings.IndexByte(",;.)}]", rest[0]) >= 0
	}
	return strings.Trim(rest, ",;.)}]") == ""
}

// compoundBearerAt reports whether the slot word at s[start:] is the tail of
// one of the excused compounds, the letters before it being exactly that word.
func compoundBearerAt(s string, start int) bool {
	i := start
	for i > 0 && (s[i-1] >= 'a' && s[i-1] <= 'z' || s[i-1] >= 'A' && s[i-1] <= 'Z') {
		i--
	}
	return i < start && compoundBearer[strings.ToLower(s[i:start])]
}

// LooksLikeCredential reports whether s contains credential-shaped content.
func LooksLikeCredential(s string) bool {
	for _, p := range credentialPatterns {
		if p.MatchString(s) {
			return true
		}
	}
	for _, m := range slotPrefix.FindAllStringSubmatchIndex(s, -1) {
		v, long := slotValueOf(s, m[1])
		if !long || benignSlotValue(v) {
			continue
		}
		if strings.EqualFold(s[m[2]:m[3]], "bearer") && compoundBearerAt(s, m[2]) {
			continue
		}
		return true
	}
	for _, m := range userinfoURL.FindAllStringSubmatch(s, -1) {
		if passwordInURL(m[1]) {
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

// slotWords name a credential slot only as whole words of the key (the key is
// split on anything but a letter or digit, at a lower-to-upper camelCase step
// and after an acronym run, and again on the lower-cased key split only on
// anything but a letter or digit, so a case change inside the word cannot hide
// it): "auth" is a slot in "x-auth", "basic_auth", "AWSAuthKey" or "aUth_header",
// but "author" and "authority" are not credentials.
var slotWords = map[string]bool{"auth": true, "oauth": true}

// slotRuns name a credential slot wherever they occur in the key's words run
// together, so a slot glued to a prefix or split by case or punctuation is
// still a slot: "bearer", "userbearer"; "privkey", "privKey", "sshPrivKey",
// "priv_key"; "accesskey", "access_key", "AWSAccessKeyId", "myaccesskey".
var slotRuns = []string{"bearer", "privkey", "accesskey"}

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

// describesOnly reports whether every word after a slot is a qualifier. A slot
// with no word after it is the slot itself.
func describesOnly(rest []string) bool {
	if len(rest) == 0 {
		return false
	}
	for _, w := range rest {
		if !benignKeyQualifiers[w] {
			return false
		}
	}
	return true
}

// keyWords splits a key into lower-case words: at anything but a letter or
// digit, where a lower-case letter or digit meets an upper-case letter
// ("accessKey"), and where an acronym run meets a capitalised word ("AWSAccess"
// is "aws" and "access").
func keyWords(k string) []string {
	rs := []rune(k)
	var b strings.Builder
	for i, r := range rs {
		if i > 0 && r >= 'A' && r <= 'Z' {
			prev := rs[i-1]
			lowerOrDigit := prev >= 'a' && prev <= 'z' || prev >= '0' && prev <= '9'
			acronymEnd := prev >= 'A' && prev <= 'Z' && i+1 < len(rs) && rs[i+1] >= 'a' && rs[i+1] <= 'z'
			if lowerOrDigit || acronymEnd {
				b.WriteByte(' ')
			}
		}
		b.WriteRune(r)
	}
	return strings.FieldsFunc(strings.ToLower(b.String()), notAlnum)
}

// notAlnum is true for anything but an ASCII letter or digit: where a key splits.
func notAlnum(r rune) bool {
	return (r < 'a' || r > 'z') && (r < '0' || r > '9')
}

// slotWordIn reports whether a slot word stands in words with no qualifier-only
// tail after it: a slot with a trailing word that is not a qualifier
// ("auth_header") is a slot, "auth_method" only describes one.
func slotWordIn(words []string) bool {
	for i, w := range words {
		if slotWords[w] && !describesOnly(words[i+1:]) {
			return true
		}
	}
	return false
}

// plainWords splits a key into lower-case words at anything but a letter or
// digit, and nowhere else.
func plainWords(k string) []string {
	return strings.FieldsFunc(strings.ToLower(k), notAlnum)
}

// CredentialKey reports whether an extension key names a credential slot.
func CredentialKey(k string) bool {
	lk := strings.ToLower(k)
	for _, w := range credentialKeyWords {
		if strings.Contains(lk, w) {
			return true
		}
	}
	// The slot word is looked for in two splits of the key and either one
	// counts: the camelCase-aware split (so "AWSAuthKey" has "auth") and the
	// plain split, that cuts only at anything but a letter or digit and ignores
	// case. A case boundary inside the slot word itself ("aUth", "OAUth") cuts
	// it in the first split; the plain one still holds it whole, and the key
	// stays a slot whatever its spelling.
	if slotWordIn(keyWords(k)) || slotWordIn(plainWords(k)) {
		return true
	}
	words := keyWords(k)
	// ends[i] is where word i ends in the words run together.
	ends := make([]int, len(words))
	run := 0
	for i, w := range words {
		run += len(w)
		ends[i] = run
	}
	joined := strings.Join(words, "")
	for _, slot := range slotRuns {
		for from := 0; from < len(joined); from++ {
			j := strings.Index(joined[from:], slot)
			if j < 0 {
				break
			}
			from += j
			end := from + len(slot)
			last, first := -1, 0
			for i, e := range ends {
				if e == end {
					last = i
				}
				if e <= from {
					first = i + 1
				}
			}
			if slot == "bearer" && last == first && from > ends[first]-len(words[first]) &&
				compoundBearer[words[first][:from-(ends[first]-len(words[first]))]] {
				continue // "pallbearer": the one word is an excused compound
			}
			// A slot that ends inside a longer word ("accesskeyid") is a slot.
			if last < 0 || !describesOnly(words[last+1:]) {
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
