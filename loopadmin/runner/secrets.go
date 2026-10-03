package runner

import (
	"regexp"
	"strings"
)

// Credential exclusion. Packets, results and extensions never carry secrets:
// the runner's credentials are installed by the operator outside the model's
// reach, and nothing the caller or the model writes into a packet may smuggle
// one in. The check is deliberately conservative pattern matching; it is a
// floor, not a scanner replacement.
var credentialPatterns = []*regexp.Regexp{
	regexp.MustCompile(`-{5}BEGIN [A-Z ]*PRIVATE KEY-{5}`),
	regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}`),
	regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}`),
	regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
	regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}`),
	regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]{20,}`),
	regexp.MustCompile(`(?i)(api[_-]?key|secret|passw(or)?d|token)\s*[:=]\s*\S{8,}`),
}

// LooksLikeCredential reports whether s contains credential-shaped content.
func LooksLikeCredential(s string) bool {
	for _, p := range credentialPatterns {
		if p.MatchString(s) {
			return true
		}
	}
	return false
}

// CredentialKey reports whether an extension key names a credential slot.
func CredentialKey(k string) bool {
	k = strings.ToLower(k)
	for _, w := range []string{"token", "secret", "password", "passwd", "credential", "apikey", "api_key", "api-key", "private_key"} {
		if strings.Contains(k, w) {
			return true
		}
	}
	return false
}
