package deskkit

import "regexp"

// scrub.go — the redaction pass every DIAGNOSTIC surface runs before it prints.
//
// Diagnostics are the one place a desk tool deliberately widens what it says: a trace block
// prints the argv it ran, the environment overrides it set, and the child's stderr verbatim.
// Each of those three is a place a credential has actually travelled — `GH_TOKEN=` in a
// child env, an `x-access-token:<token>@github.com` push URL in git's own error text, an
// `Authorization:` header echoed by a verbose HTTP client. A diagnostic that printed one
// would turn the debugging aid into the leak, so redaction is applied at the SINGLE choke
// point every diagnostic passes through rather than at each print site.
//
// This is the same posture bodycheck.go takes on outward writes, and it reuses bodycheck's
// own token pattern (reGitHubToken, reJWT, reAWSKeyID) rather than restating it: one pattern
// set, so a prefix added there is redacted here on the same commit. What it adds are the
// three TRANSPORT shapes that never appear in a body but always appear in a command line —
// URL userinfo, an Authorization header, and a secret-shaped environment assignment.
//
// It is a seatbelt against accidental disclosure in a diagnostic, not an exfiltration
// defence, and it is deliberately BIASED TO OVER-REDACT: a redacted value the operator has
// to look up by hand is a nuisance, a printed token is an incident.

var (
	// reURLUserinfo matches the `user:secret@` userinfo of a URL — the shape a
	// token-authenticated git remote carries (`https://x-access-token:<token>@github.com/…`,
	// `https://oauth2:<pat>@gitlab.com/…`). The password half is what is redacted; the
	// username half is kept because it is the part that identifies the transport
	// ("x-access-token" is diagnostic, the token is not).
	reURLUserinfo = regexp.MustCompile(`://([^:/@\s]+):([^@\s]+)@`)

	// reAuthHeader matches an `Authorization: token …` / `Authorization: Bearer …` header in
	// any of the spellings a client or a curl trace emits. The scheme word survives; the
	// credential after it does not.
	reAuthHeader = regexp.MustCompile(`(?i)(authorization\s*[:=]\s*)(token |bearer |basic )?\S+`)

	// reSecretEnvAssign matches a `NAME=value` assignment whose NAME reads as a credential.
	// This is the shape a trace of a child process's environment overrides carries — the
	// `GH_TOKEN=` deskdispatch sets on every `gh` child being the case that motivated it.
	// The name is kept (knowing WHICH variable was set is the diagnostic); the value is not.
	reSecretEnvAssign = regexp.MustCompile(
		`(?i)\b([A-Za-z0-9_-]*(?:TOKEN|SECRET|PASSWORD|PASSWD|APIKEY|API_KEY|PRIVATE_KEY|CREDENTIAL|PAT)[A-Za-z0-9_-]*\s*=)\s*\S+`)
)

// redactedMarker is what replaces a matched credential. It is a fixed, obviously-synthetic
// string: an operator reading a trace can tell "this was redacted" from "this was empty",
// which a silent elision would not.
const redactedMarker = "<redacted>"

// Scrub returns s with every credential shape this package can recognise replaced by
// redactedMarker, and with control/ANSI sequences stripped (StripControl keeps tab and
// newline, so multi-line child stderr stays readable).
//
// Control stripping runs FIRST and is load-bearing for the redaction, not merely cosmetic:
// an ANSI sequence embedded mid-token would split a run the patterns below match as one, so
// a colourised child's output could otherwise carry a token past them.
//
// Scrub is applied to TRACE and DIAGNOSTIC output only. It is deliberately NOT applied to
// the existing one-line refusal messages: those are byte-stable surfaces other tools and
// transcripts read, and widening the redaction to them would change output that no
// credential has been observed to reach.
func Scrub(s string) string {
	if s == "" {
		return s
	}
	s = StripControl(s)
	// Transport shapes first: a URL's userinfo and an Authorization header are matched as
	// whole spans, and doing them before the generic token patterns keeps the surrounding
	// structure (which host, which scheme) legible in the redacted result.
	s = reURLUserinfo.ReplaceAllString(s, "://$1:"+redactedMarker+"@")
	s = reAuthHeader.ReplaceAllString(s, "${1}${2}"+redactedMarker)
	s = reSecretEnvAssign.ReplaceAllString(s, "${1}"+redactedMarker)
	// Then the value shapes, reused verbatim from the body scanner.
	s = reGitHubToken.ReplaceAllString(s, redactedMarker)
	s = reJWT.ReplaceAllString(s, redactedMarker)
	s = reAWSKeyID.ReplaceAllString(s, redactedMarker)
	return s
}
