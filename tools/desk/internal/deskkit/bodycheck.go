package deskkit

import (
	"fmt"
	"regexp"
	"strings"
)

// bodycheck is the shared secret scan. It lives in deskkit because
// deskpost, deskpr, and deskreply all run it over any text they would write to
// GitHub. It is a seatbelt plus friction against ACCIDENTAL credential paste,
// not a complete exfiltration defence — a deliberately encoding adversary can evade
// pattern filters. There is NO override flag anywhere.

var (
	reGitHubToken = regexp.MustCompile(`(ghp_|github_pat_|ghs_|gho_)[A-Za-z0-9_]+`)
	reAWSKeyID    = regexp.MustCompile(`AKIA[0-9A-Z]{16}`)
	reJWT         = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)
	reBase64ish   = regexp.MustCompile(`[A-Za-z0-9+/=]{32,}`)
	reLowerHex    = regexp.MustCompile(`^[0-9a-f]+$`)

	// rePEMBegin captures the LABEL of a PEM / ASCII-armor BEGIN line so the scanner can
	// tell an UNENCRYPTED key block (which it must refuse) from an ENCRYPTED one (which is
	// ciphertext, safe to commit at rest — see encryptedArmorLabels).
	rePEMBegin = regexp.MustCompile(`-----BEGIN ([A-Z0-9 ]+?)-----`)

	// reK8sSecretKind identifies a Kubernetes Secret manifest and reK8sSecretData the
	// `data:`/`stringData:` mapping that carries its values. A Secret whose values are
	// NOT sops ciphertext is a DECRYPTED Secret — k8s `data:` is base64 ENCODING, not
	// encryption, so those values are plaintext — and committing one is exactly the leak
	// the sanctioned sops-encrypt-then-commit flow exists to prevent (#778). A short
	// base64'd password slips the 32-char high-entropy rule entirely, so this positive
	// detection catches what entropy alone misses.
	//
	// The optional leading `[+-]` is load-bearing, not cosmetic. The surface that
	// actually carries a committed Secret is deskpr's BRANCH DIFF, where every line is
	// prefixed `+`/`-`; an anchor of `^\s*` alone would make this rule fire only on a
	// manifest pasted into a body and never on the commit that #778 is about. It also
	// covers the YAML sequence form (`- kind: Secret`).
	//
	// The optional quoting, trailing comma and trailing `#` comment are load-bearing for
	// the same reason: they are what the manifest looks like on the surfaces it actually
	// arrives on. A `kind: Secret # app creds` line and the PRETTY-PRINTED JSON that
	// `kubectl get secret -o json` emits — `"kind": "Secret",` with `"data": {` below it —
	// are the two commonest accidental-paste shapes, and an anchor of `\s*$` against a
	// bare unquoted key silently missed BOTH. Measured before this tolerance was added:
	// the trailing-comment form and both JSON forms carrying the same plaintext value
	// were ADMITTED while the bare YAML form refused.
	reK8sSecretKind = regexp.MustCompile(`(?m)^[+-]?\s*"?kind"?\s*:\s*["']?Secret["']?\s*,?\s*(#.*)?$`)
	reK8sSecretData = regexp.MustCompile(`(?m)^[+-]?(\s*)"?(data|stringData)"?\s*:\s*\{?\s*$`)
	// reK8sSecretEntry matches one `key: value` entry of such a mapping; group 1 is the
	// indent (used to tell an entry from the dedented line that ends the mapping),
	// group 2 the key, group 3 the scalar value (empty for a block scalar). The optional
	// quotes on the key cover the JSON form of the same mapping.
	reK8sSecretEntry = regexp.MustCompile(`^[+-]?(\s*)"?([A-Za-z0-9_.\-]+)"?\s*:[ \t]*(.*)$`)

	// The sops-DOCUMENT signature. These no longer drive a refusal ARM of ScanSurface —
	// #778 removed the blanket "contains a sops block" refusal, because ciphertext
	// committed at rest is the sanctioned flow. They remain because structured.go still
	// needs them to RECOGNISE a sanctioned document (sanctionedSopsDocument) and to
	// neutralise its markers on the marker surface.
	//
	// reSopsKey and reSopsField together form that signature. A real sops file carries a
	// line-anchored `sops:` mapping key AND, nested under it, a sops-metadata field (mac /
	// lastmodified / unencrypted_suffix / kms / gcp_kms / azure_kv / pgp / age). Prose that
	// merely says the word "sops" has neither shape, so requiring BOTH drops the bare-word
	// false positive.
	//
	// reSopsEncVal matches the STRUCTURAL start of a sops encrypted VALUE — the AES256-GCM
	// marker followed by the envelope's mandatory first field, `data:` — NOT the bare marker
	// substring. sops writes every encrypted value as `ENC[…,data:…,iv:…,tag:…,type:…]`, so a
	// real lone value pasted outside a sanctioned document (still refused by design) always
	// carries `data:` right after the marker, while PROSE that merely NAMES the marker while
	// describing the scanner — "keys on an ENC[…] marker" — has no `data:` after it. Anchoring
	// on that field is what drops the fifth #781 false positive: the refusal arm below keys on
	// this marker, so it now refuses structural ciphertext rather than the substring — exactly
	// the lone-value shape it refused before, and no longer a description of it. The same
	// anchored marker drives neutraliseSopsMarkers (structured.go), a length-preserving rename
	// over an already-recognised sanctioned document; its replacement carries the `data:` tail
	// so the rename keeps the surface byte-for-byte the same length.
	reSopsEncVal = regexp.MustCompile(`ENC\[AES256_GCM,data:`)
	reSopsKey    = regexp.MustCompile(`(?m)^\s*"?sops"?\s*:`)
	reSopsField  = regexp.MustCompile(`(?m)^\s*"?(mac|lastmodified|unencrypted_suffix|kms|gcp_kms|azure_kv|pgp|age)"?\s*:`)
)

// encryptedArmorLabels are the ASCII-armor BEGIN labels whose body is CIPHERTEXT rather
// than a plaintext secret. The scan exists to catch UNENCRYPTED secrets pasted in the
// clear; encrypting a secret at rest and committing it is the sanctioned pattern, so
// these blocks are not refused and their armored base64 body is exempt from the
// high-entropy scan (#778).
//
// Every OTHER `-----BEGIN` block is still refused: RSA/EC/OPENSSH/DSA/bare PRIVATE KEY,
// CERTIFICATE — and, deliberately, `ENCRYPTED PRIVATE KEY` as well. A PKCS#8
// passphrase-encrypted key is NOT in this list even though it is technically ciphertext:
// its confidentiality rests entirely on a passphrase that offline cracking attacks, the
// scan cannot see how strong that passphrase is or whether it was committed alongside,
// and #778's actual requirement is sops-encrypted Secrets (age/pgp/sops) — admitting
// PKCS#8 buys that requirement nothing and widens what may be committed in exchange.
// Keep this list to formats whose ciphertext is bound to a KEY the repo does not hold.
var encryptedArmorLabels = map[string]bool{
	"AGE ENCRYPTED FILE": true,
	"PGP MESSAGE":        true,
}

const (
	// runThreshold mirrors reBase64ish's {32,}: the length at which a base64ish run is
	// long enough to hold a credential and is therefore scanned.
	runThreshold = 32
	// maxOpaqueInPath is how much NON-word-shaped material a path-like run may carry in
	// total before the exemption is denied. It is deliberately equal to runThreshold: if
	// the opaque segments of a run, concatenated, would themselves meet the run threshold,
	// the run could be a secret wearing slashes and must be scanned.
	maxOpaqueInPath = runThreshold
	// maxDigitRun bounds a digit group inside a word-shaped segment. Digits carry ~3.3
	// bits each, so short groups (ports, years, `python311`) are noise; a long unbroken
	// digit run is not name material (it could be an account or card number).
	maxDigitRun = 12
	// minLowerWord is the shortest lowercase run that reads as a word rather than as
	// base64 debris. `tools`, `api`, `src` qualify; the `b` of `bPxRfi…` does not.
	minLowerWord = 3
	// maxWordSegment caps how long a PATH SEGMENT may be and still be treated as name
	// material at all, however word-shaped it looks. Real segments are short —
	// `Widget` is 6, `ConfigurationManager` is 20 — so 48 is far
	// above any legitimate name while keeping a degenerate run (a 200-character
	// lowercase stretch, which is word-SHAPED but is not a word) on the opaque budget
	// where the length gate can refuse it. It is enforced by looksLikeWords (the path-segment
	// caller); wordDecomposition itself no longer caps length, so isIdentifierLike can apply
	// its own, wider cap — see maxIdentifierRun.
	maxWordSegment = 48
	// maxIdentifierRun caps how long a BARE run may be and still be treated as a single
	// identifier by isIdentifierLike. Real Go identifiers are long but bounded — the longest
	// test names in this repo run into the 50s of characters
	// (`TestLeadingZeroTagsDoNotCollideWithTheirCanonicalForm` is 53) — so 100 sits well above
	// any legitimate name while still refusing a random base64 run that happens to decompose.
	//
	// Putting the cap HERE, on the bare-identifier caller, rather than in wordDecomposition
	// (where it used to sit at maxWordSegment and refused those legitimate long test names,
	// #781 class) is deliberate and SAFE: for a bare identifier the security bar is the
	// decomposition itself (words>=2 && camel), and a LONGER cap is strictly HARDER for random
	// base64 to clear, not easier — every extra character is another chance for a lone capital,
	// an ALL-CAPS stretch, or short lowercase debris to disqualify the whole run. The tight
	// maxWordSegment cap stays on looksLikeWords, where a path SEGMENT that overruns it should
	// spend opaque budget rather than be admitted.
	maxIdentifierRun = 100
	// maxBareAssignKey bounds the variable-name half of a `key=value` shell assignment
	// that is NOT itself word-shaped (see isAssignmentLike). Shell scratch variables in
	// Verify/Evidence command text are one or two characters — `f=`, `d=`, `p=` — and a
	// key this short can contribute at most two characters of opaque material, far under
	// runThreshold. A longer key must earn its exemption by being word-shaped.
	maxBareAssignKey = 2
)

// twoLetterWords are the two-letter units that count as WORDS in a CamelCase
// decomposition even though they carry only one lowercase letter after the capital
// (see wordDecomposition's capital branch, which otherwise requires two).
//
// The membership rule is deliberately a CLOSED LIST OF ENGLISH WORDS rather than the
// broader "a capital followed by any single lowercase letter", and the difference is
// measurable rather than stylistic. Both spellings clear the reported false positives
// equally well, so the only thing that can choose between them is what each costs on real
// token material: against 2,000,000 uniformly random 32-char base64 runs the blanket
// relaxation exempts 35, while this list exempts 0 — the same as the stricter rule it
// replaces. The list buys back the whole false-positive class without measurably loosening
// the scanner's grip. TestTwoLetterWordsCostAlmostNothing pins that comparison against a
// fixed seed, so a future widening to "any capital plus one lowercase" — or a list grown
// long enough to amount to the same thing — cannot be waved through on intuition.
//
// The list is grounded in this repo's own identifiers, not guessed: the units that
// actually block real long CamelCase names here are, in frequency order, Is (124),
// No (48), On (41), At, By, In, To, As, An, Of, Up, Do, Or, It, So, Me, Be — every one an
// English function word. The debris further down that same frequency table (Ym, Jl, Vh,
// Zm, Qx, Px, Xo) comes from base64 test fixtures, and an English-word list leaves all of
// it on the opaque budget where it belongs.
//
// `Id` and `Ok` are the only two non-words admitted: both are universal Go identifier
// idioms (`UserIdIsStable`, `ReportsOkOnEmptyInput`) and both are English-lexicon enough
// to be indistinguishable in cost from the rest of the list. Do NOT extend this to
// general abbreviations (`Db`, `Ui`, `Io`, `Pr`, `Cd`, `Tx`) — each addition is another
// face on the die that random base64 gets to roll, and the frequency table above shows
// they buy back almost nothing: one or two identifiers each.
var twoLetterWords = map[string]bool{
	"Am": true, "An": true, "As": true, "At": true, "Be": true, "By": true,
	"Do": true, "Go": true, "He": true, "If": true, "In": true, "Is": true,
	"It": true, "Me": true, "My": true, "No": true, "Of": true, "On": true,
	"Or": true, "So": true, "To": true, "Up": true, "Us": true, "Vs": true,
	"We": true, "Id": true, "Ok": true,
}

// SurfaceBody is the default surface name — the PR/issue/comment body most callers scan.
const SurfaceBody = "body"

// BodyCheck REFUSES (Refused → exit 5) any body that looks like it carries a
// credential, and returns nil for a clean body. It refuses on:
//   - GitHub token prefixes ghp_ / github_pat_ / ghs_ / gho_
//   - AWS access-key IDs (AKIA + 16 upper-alnum)
//   - an UNENCRYPTED PEM key block ("-----BEGIN … PRIVATE KEY-----", CERTIFICATE, …).
//     An age/pgp ASCII-armor block is ciphertext, not a plaintext secret, and is NOT
//     refused (see hasPlaintextPEM and encryptedArmorLabels, #778).
//   - eyJ-prefixed 3-segment JWT shapes
//   - a DECRYPTED Kubernetes Secret (see decryptedK8sSecret, #778): a `kind: Secret`
//     manifest with a data/stringData mapping in which ANY value is not sops
//     ciphertext. k8s `data:` is base64 ENCODING, not encryption, so such a value is
//     plaintext, and a short one clears no entropy threshold. A Secret every one of
//     whose values is ENC[AES256_GCM…] is sops-encrypted and passes. The rule is
//     PER-VALUE: one encrypted field elsewhere in the document does not license a
//     plaintext field beside it. A bare `sops:` metadata mapping is not itself a
//     secret and is no longer refused (#585).
//   - any run of ≥32 base64ish chars, EXCEPT:
//   - an exactly-40- or exactly-64-char lowercase-hex run (git SHAs — the
//     methodology quotes them constantly; refusing them would brick every verdict).
//   - a run shaped like a filesystem or module path, absolute OR repo-relative,
//     built out of word-shaped segments (see isPathLike). This exempts
//     `/Users/operator/work/example/slides` (#1052), the mandated `file:line`
//     finding format `tools/desk/internal/deskkit/bodycheck.go:45`, and Go module
//     paths (#209) while still refusing opaque token material carried
//     between slashes — including an AWS secret access key, whose `/` characters
//     defeat a purely length-based segment gate.
//   - a run that is itself a single bare identifier built entirely out of words (see
//     isIdentifierLike). This exempts long CamelCase Go identifiers — test names like
//     `HonoredFilterLeavesTripwireSilent` routinely clear the 32-char run threshold
//     with no `/` to trigger the path exemption above (#253) — while
//     still refusing a bare random token. MIXED-case token material fails the
//     word-decomposition almost immediately (a lone capital, an ALL-CAPS stretch, or
//     1-2 char lowercase debris); LOWERCASE token material — hex or base36, whose
//     digit groups split it into several letter-word units — is kept out by the
//     requirement that at least one word unit be capital-led (#582
//     review; see isIdentifierLike). A two-letter English word inside such an
//     identifier (`…IsOn…`, `…VsScheduled…`) counts as a word rather than as debris —
//     see twoLetterWords, which is what unblocked the #781 class.
//   - a `key=<path>` shell assignment whose VALUE is itself exempt by one of the two
//     rules above (see isAssignmentLike). This is the Verify/Evidence command idiom
//     `f=docs/streams/…; test -f "$f" && …` (#775), which could not be
//     reworded without desyncing a recorded audit trail from what was actually run.
//   - a run composed ENTIRELY of `=` (see isAllEquals): a console-banner separator
//     line (`echo "===================="`, #781) is base64 padding with no
//     payload, so it cannot carry a credential. This does NOT exempt a `VAR=secret`
//     shell assignment, whose run carries the secret's own base64 after the `=`.
//   - the marker-anchored digest fields of a RECOGNISED structured format (see
//     structuredExemptSpans in structured.go, #778): go.sum `h1:` checksums, SRI
//     integrity digests, and — inside a SANCTIONED sops document only — the ciphertext
//     payloads of complete envelopes, the generated `sops:` metadata block, and armored
//     ciphertext bodies. The sops pass is earned by the FULL envelope grammar PLUS the
//     document signature, never by a bracket or a BEGIN line in isolation: a lone
//     envelope pasted into prose, a bare `sops:` footer, and an armored blob outside any
//     sops document all still refuse.
//
// Test vectors for BOTH directions (SHAs and paths pass, every credential pattern —
// including one wearing a path disguise — refuses) are exercised by this package's tests.
func BodyCheck(body []byte) error { return ScanSurface(SurfaceBody, body) }

// ScanSurface is BodyCheck with the SURFACE named in the refusal message.
//
// A tool that scans more than one surface before it writes — deskpr scans the PR title,
// the branch name and the branch diff, not just the body — used to report every refusal
// as a "body" problem whichever surface actually tripped. #328 is the cost:
// the trigger was in the branch diff, two full diagnostic rounds went into finding that
// out, and the first two attempts were spent rewriting a body that was never the problem.
// A refusal that misidentifies its own surface converts a one-line fix into an
// investigation, so surface is a required argument of the scan rather than a decoration
// callers may add afterwards.
//
// surface is a short noun phrase used verbatim ("PR body", "PR title", "branch name",
// "branch diff vs origin/main"). The DETECTION is identical on every surface; only the
// message differs.
func ScanSurface(surface string, content []byte) error {
	return scanSurface(surface, content, true)
}

// ScanSurfaceSecrets is ScanSurface WITHOUT the impersonated-human-ruling guard: every
// credential arm and the high-entropy loop run exactly as in ScanSurface, in the same
// order, with the same refusal messages; only the ruling-claim check is skipped.
//
// It exists for one caller shape: a DIFF surface, where the two checks need DIFFERENT
// line directions. The secret arms deliberately read every line of a diff — added,
// removed and context — because a credential on a removed or context line is still
// sitting in the repository's history and working tree, and #778's committed-Secret
// class arrives on exactly that surface. The ruling-claim guard must NOT read removed
// lines: a deletion cannot introduce a forged ruling (only ADDED text can claim a
// human's voice), so scanning the removal direction false-positives on every
// retirement/cleanup branch that deletes an old attribution line — and, because the
// impersonation guard is non-overridable BY DESIGN (see scanoverride.go), that false
// positive is a hard stop with no audited way through, on a branch whose whole point
// is to REMOVE the attributions. A diff-scanning caller therefore runs this over the
// full diff and ScanSurfaceRulingClaim over the added lines only.
//
// Do not reach for this on a NON-diff surface: a body, title or branch name has no
// removal direction, and ScanSurface (which includes the ruling-claim guard) is the
// correct scan there.
func ScanSurfaceSecrets(surface string, content []byte) error {
	return scanSurface(surface, content, false)
}

// ScanSurfaceRulingClaim runs ONLY the impersonated-human-ruling guard
// (impersonation.go) over content — the other half of the diff-surface split described
// on ScanSurfaceSecrets. It reads the same marker surface ScanSurface hands the guard,
// so a claim's verdict is identical whichever entry point evaluates it.
//
// The caller owns the direction narrowing: hand this the ADDED lines of a diff (never
// the removed ones), with the leading `+` markers stripped so line-start positioning
// (rulingClaimPrefixRe) and per-line citation checks (citedRelay) see the lines as they
// exist in the file.
func ScanSurfaceRulingClaim(surface string, content []byte) error {
	if surface == "" {
		surface = SurfaceBody
	}
	if name, found := ImpersonatedRulingClaim(markerSurface(string(content))); found {
		return impersonationRefusal(surface, name)
	}
	return nil
}

// scanSurface is the shared engine behind ScanSurface and ScanSurfaceSecrets.
// rulingClaim selects whether the impersonated-human-ruling guard runs; its position
// in the arm order — after the literal-marker arms, before the high-entropy loop — is
// unchanged from when it was inlined, so refusal precedence on a surface tripping more
// than one arm is exactly what it was.
func scanSurface(surface string, content []byte, rulingClaim bool) error {
	if surface == "" {
		surface = SurfaceBody
	}
	// TWO VIEWS OF THE SAME SURFACE.
	//
	//   raw  every byte, exactly as the caller handed it over. The high-entropy-run loop at
	//        the bottom reads this, minus whatever structuredExemptSpans accounts for.
	//   s    the MARKER SURFACE: raw with every marker that structured recognition has
	//        already accounted for neutralised. The literal-marker arms below read this.
	//
	// Normalising the SURFACE rather than editing the arms is deliberate, and it is what
	// makes this file maintainable through the tools it guards. Both markers below are
	// substrings of the detector's OWN SOURCE and of its own refusal messages, so any diff
	// touching them carries them on an added AND a removed line — and, because the scan reads
	// three lines of diff context, on lines the branch does not even change. That is
	// #380: four PRs went around deskpr rather than through it, and #328's
	// closing note is that a guard which fires on its own routine edit gets routed around
	// rather than fixed. Every arm below is therefore byte-for-byte what it was; what
	// changed is what they are handed. See markerSurface for what is neutralised and why
	// each neutralisation is safe.
	raw := string(content)
	s := markerSurface(raw)
	switch {
	case reGitHubToken.MatchString(s):
		return Refused("refused: " + surface + " contains a GitHub token prefix (ghp_/github_pat_/ghs_/gho_)")
	case reAWSKeyID.MatchString(s):
		return Refused("refused: " + surface + " contains an AWS access-key ID (AKIA…)")
	case hasPlaintextPEM(s):
		return Refused("refused: " + surface + " contains an unencrypted PEM key block (-----BEGIN …-----)")
	case reJWT.MatchString(s):
		return Refused("refused: " + surface + " contains a JWT-shaped token (eyJ….….…)")
	// decryptedK8sSecret reads `raw`, NOT the marker surface `s`, and it is the one arm
	// that must. Every other arm fires on a MARKER, so neutralising a marker that
	// structured recognition has already accounted for is exactly right for them. This arm
	// asks the opposite question — is this value ciphertext or is it plaintext? — and
	// neutraliseSopsMarkers rewrites `ENC[AES256_GCM` to lowercase on `s` for any
	// sanctioned document. Reading `s` therefore made a CORRECTLY sops-encrypted Secret
	// look decrypted and refused it: a false positive on the exact artifact #778 exists to
	// let through, caught by the corpus fixture neg-sops-encrypted-manifest.
	case decryptedK8sSecret(raw):
		return Refused("refused: " + surface + " contains a DECRYPTED Kubernetes Secret " +
			"(kind: Secret with a data/stringData value that is not ENC[AES256_GCM…]) — " +
			"sops-encrypt every value before committing")
	// The sops arm STAYS, and #778 is still satisfied. An earlier draft of this branch
	// deleted it outright on the reasoning that ciphertext at rest is sanctioned. Deleting
	// it opens false negatives the entropy loop structurally cannot cover: a sops IMITATION
	// and a bare `sops:` footer carry no base64 run over the 32-char threshold, so the loop
	// never looks at them and the arm was the only thing refusing them.
	//
	// The narrowing #778 actually needs is already done, one layer down: markerSurface
	// neutralises these markers on `s` for a document sanctionedSopsDocument has RECOGNISED
	// (full envelope grammar plus the document signature plus encrypted content outside the
	// metadata block). A genuine encrypted-at-rest manifest therefore never reaches this
	// arm, and everything wearing the shape without earning it still does.
	case reSopsEncVal.MatchString(s) || (reSopsKey.MatchString(s) && reSopsField.MatchString(s)):
		return Refused("refused: " + surface + " contains a sops-encrypted secret block or ENC[ marker")
	}
	// A sops-ENCRYPTED value (ENC[AES256_GCM,…]) is no longer refused on sight (#778):
	// this scan catches UNENCRYPTED secrets, and sops ciphertext committed at rest is
	// the sanctioned flow. What replaces the blanket refusal is NARROWER, not absent —
	// decryptedK8sSecret above refuses a Secret with any plaintext value, and the
	// high-entropy loop below exempts a run only when it lies WHOLLY INSIDE a span some
	// recogniser has anchored on a structural marker.
	//
	// WHAT THIS SCAN THEREFORE NO LONGER CATCHES, stated rather than capped silently:
	//   - a plaintext secret parked in a sops `unencrypted_suffix` field OUTSIDE a
	//     `kind: Secret` manifest has exactly the coverage any other plaintext has (the
	//     token / AWS-key / unencrypted-PEM / bare-base64 checks, all still active) and no
	//     more; a SHORT one, under the 32-char run threshold, is not caught. That is the
	//     same blind spot every short plaintext has, recorded here rather than claimed
	//     closed.
	//   - ciphertext-shaped material inside a complete ENC[…] bracket, a complete
	//     age/pgp armor block, or the marker-anchored digest fields of a recognised
	//     structured format is not inspected further. A secret smuggled INTO one of those
	//     exact grammars is out of scope by construction; each recogniser is anchored and
	//     length-exact so that "a marker somewhere nearby" never suffices.
	//   - decryptedK8sSecret is LINE-ORIENTED, so it sees a Secret only in the shapes a
	//     line-anchored regex can reach: block YAML and pretty-printed JSON, with or
	//     without diff markers, quoting, a trailing comma or a trailing comment. Two
	//     shapes are measured ADMITTED and are NOT covered: a Secret serialised as
	//     COMPACT single-line JSON (`{"kind":"Secret","data":{…}}`), and a YAML FLOW
	//     mapping (`data: {password: …}`). Both put the whole mapping on one line, which
	//     the entry/indent walk cannot decompose; covering them needs a real YAML/JSON
	//     parse rather than a wider regex, which is its own change. A LONG value in
	//     either shape still refuses on entropy; a SHORT one is admitted, which is the
	//     same blind spot recorded above for short plaintext generally.
	//   - The `kind:`/`data:` match is CASE-SENSITIVE, matching Kubernetes' own
	//     case-sensitivity. `kind: secret` or `Data:` is therefore admitted by this rule —
	//     measured — but such a manifest is not a Secret Kubernetes would accept either,
	//     and any plaintext in it has exactly the coverage plaintext in any other
	//     document has. This is a deliberate grammar match, not an oversight.

	// Impersonated human-ruling guard (impersonation.go). This write
	// path never posts as a human, so a body claiming a configured human's ruling BY
	// NAME — "Decision (Alex, ...)", "Ruling: ... — Alex", "I (Alex) have decided" — is
	// refused categorically, independent of whether the claim happens to be true.
	// Skipped when the caller scans a surface whose removal direction must not be read
	// (rulingClaim false — see ScanSurfaceSecrets); such a caller runs
	// ScanSurfaceRulingClaim over the added lines instead.
	if rulingClaim {
		if name, found := ImpersonatedRulingClaim(s); found {
			return impersonationRefusal(surface, name)
		}
	}
	// Structured-format spans are computed ONCE over the whole surface, before the loop, so
	// a recognised field is judged by its FORMAT rather than by the run's own shape. See
	// structured.go for why every span is anchored on a field marker and length-exact.
	//
	// The ciphertext exemption is structuredExemptSpans' ALONE. An earlier draft of #778
	// also exempted anything inside a complete `ENC[AES256_GCM,…]` bracket or a complete
	// armor block (isEncryptedMaterial). That is too wide, and the corpus proves it: a
	// lone envelope pasted into prose, a sops footer whose only envelope is its own `mac:`,
	// and an armored blob outside any sops document were all ADMITTED. The pass has to be
	// earned by the FULL envelope grammar PLUS the document signature — see
	// sanctionedSopsDocument — never by a bracket or a BEGIN line in isolation.
	exempt := structuredExemptSpans(raw)
	for _, loc := range reBase64ish.FindAllStringIndex(raw, -1) {
		run := raw[loc[0]:loc[1]]
		if coveredBy(exempt, loc[0], loc[1]) {
			continue
		}
		if isGitSHA(run) || isPathLike(run) || isIdentifierLike(run) || isAssignmentLike(run) || isAllEquals(run) {
			continue
		}
		return Refused(fmt.Sprintf(
			"refused: %s contains a %d-char high-entropy run (possible secret); "+
				"only git SHAs (40/64 lowercase hex), slash-separated paths built from "+
				"word-shaped segments, bare word-shaped identifiers, key=<path> shell "+
				"assignments, all-'=' banner separators, and the marker-anchored digest "+
				"fields of a recognised structured format (go.sum h1:, SRI integrity, a "+
				"complete sops envelope) are exempt", surface, len(run)))
	}
	return nil
}

// isGitSHA reports whether run is an exactly-40 (sha1) or exactly-64 (sha256)
// lowercase-hex string — the one exemption to the high-entropy-run rule.
func isGitSHA(run string) bool {
	return (len(run) == 40 || len(run) == 64) && reLowerHex.MatchString(run)
}

// isAllEquals reports whether run is composed ENTIRELY of '=' — the FOURTH exemption to
// the high-entropy-run rule (#781). '=' is in reBase64ish's
// charset (it is base64 padding), so a console-banner separator line —
// `echo "=================================="`, a 34-char run of '=' — clears the 32-char
// run threshold and, having no '/' and failing word-decomposition on its first character,
// is exempted by nothing above and refuses on ordinary shell scripts. deskpr scans deleted
// diff lines too, so a banner already on origin cannot be edited out of the
// refusal — only the scanner can distinguish it.
//
// This is safe because a run of only '=' carries ZERO data bytes: base64 '=' is padding,
// not payload, so an all-'=' run cannot encode a credential of any length. It is NOT the
// `VAR=secret` shell-assignment laundering shape (the AWS-secret-key guard in the tests):
// that run carries the secret's own base64 AFTER the '=', so it is never all-'='. The
// discriminator here is "the run has no non-padding character at all", which no real
// credential — and no assignment carrying one — can satisfy.
func isAllEquals(run string) bool {
	if run == "" {
		return false
	}
	for i := 0; i < len(run); i++ {
		if run[i] != '=' {
			return false
		}
	}
	return true
}

// hasPlaintextPEM reports whether s carries an UNENCRYPTED PEM / armor block — the only
// PEM material the scan refuses (#778). An age/pgp ASCII-armor block
// (encryptedArmorLabels) is ciphertext, not a plaintext secret, so it is allowed; every
// other WELL-FORMED `-----BEGIN …-----` naming private-key material — RSA/EC/OPENSSH/DSA/
// bare PRIVATE KEY, and PKCS#8 ENCRYPTED PRIVATE KEY — is refused by THIS arm.
//
// Two limits of this arm are stated rather than assumed, because both were measured and
// neither is what an earlier draft of this comment claimed:
//
//   - The truncated-header clause is WHOLE-SURFACE, not per-occurrence: it fires only
//     when the surface carries a `-----BEGIN` AND no well-formed header anywhere on it.
//     What that actually admits and refuses was measured rather than assumed, because an
//     earlier draft of this comment claimed a flat "a truncated paste always refuses"
//     that is not what the code does:
//     `-----BEGIN` alone, no label            ADMITTED (neutralised as a benign
//     delimiter by stripBenignArmorDelimiters,
//     which is what keeps the detector's own
//     prose and quoted markers from tripping
//     it, #380)
//     `-----BEGIN RSA PRIVATE KEY`, no
//     closing dashes                          REFUSED
//     `-----BEGIN …-----` (prose ellipsis)    ADMITTED
//     The whole-surface shape was probed for the laundering it suggests — pairing a
//     truncated key paste with a well-formed benign block to suppress the clause — and
//     it does NOT launder: both orderings still refuse, because a non-allow-listed
//     well-formed label refuses on clause (a) in its own right.
//   - A public/ciphertext label (CERTIFICATE, PUBLIC KEY, and the rest of
//     armorCiphertextOrPublicLabels) is likewise not refused HERE. A realistic one still
//     refuses — measured — via the high-entropy loop on its payload, with an entropy
//     message rather than a PEM one.
func hasPlaintextPEM(s string) bool {
	for _, m := range rePEMBegin.FindAllStringSubmatch(s, -1) {
		if !encryptedArmorLabels[strings.TrimSpace(m[1])] {
			return true
		}
	}
	return strings.Contains(s, "-----BEGIN") && !rePEMBegin.MatchString(s)
}

// decryptedK8sSecret reports whether s carries a Kubernetes Secret manifest with at
// least one data/stringData value that is NOT sops ciphertext — a DECRYPTED Secret,
// which is the leak #778 is about. k8s `data:` is base64 ENCODING, not encryption, so
// such a value is plaintext to anyone who can read the file, and a short one (a
// base64'd password of a dozen characters) never reaches the 32-char high-entropy rule.
// That is why this is a POSITIVE detection rather than something entropy could cover.
//
// The test is PER VALUE. The first version of this rule asked only whether the marker
// `ENC[AES256_GCM` occurred anywhere in the whole document, which meant a single
// encrypted field switched the rule off for every other field beside it: a `kind:
// Secret` carrying one ENC[…] value and one SHORT plaintext value — precisely the
// sops `unencrypted_suffix` shape — passed clean, with the short value under the
// entropy threshold as well. Partial encryption now protects only the fields that are
// actually encrypted.
//
// It errs toward refusal in both directions where it cannot be sure. A Secret whose
// data mapping yields no readable entries (values elided, a comment, an unusual layout)
// is refused rather than passed: an unreadable mapping is a could-not-check, and a
// could-not-check must never report clean. Diff markers are tolerated on every line
// because the surface that carries a committed Secret is deskpr's branch diff.
func decryptedK8sSecret(s string) bool {
	if !reK8sSecretKind.MatchString(s) {
		return false
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		m := reK8sSecretData.FindStringSubmatch(strings.TrimRight(line, " \t\r"))
		if m == nil {
			continue
		}
		mapIndent, sawEntry := len(m[1]), false
		for j := i + 1; j < len(lines); j++ {
			body := strings.TrimRight(lines[j], " \t\r")
			if strings.TrimSpace(strings.TrimLeft(body, "+- ")) == "" {
				continue // a blank (or bare-marker) line does not end the mapping
			}
			e := reK8sSecretEntry.FindStringSubmatch(body)
			if e == nil || len(e[1]) <= mapIndent {
				break // not an entry, or dedented back out of the mapping
			}
			sawEntry = true
			val := strings.TrimSpace(e[3])
			if strings.Contains(val, "ENC[AES256_GCM") {
				continue
			}
			if isBlockScalarIndicator(val) && blockScalarEncrypted(lines, j+1, len(e[1])) {
				continue
			}
			return true
		}
		if !sawEntry {
			return true // could-not-check: a Secret data mapping we could not read
		}
	}
	return false
}

// isBlockScalarIndicator reports whether a YAML scalar value is empty or one of the
// block-scalar introducers, i.e. the value lives on the following indented lines.
func isBlockScalarIndicator(val string) bool {
	switch val {
	case "", "|", "|-", "|+", ">", ">-", ">+":
		return true
	}
	return false
}

// blockScalarEncrypted reports whether the block-scalar body starting at lines[from],
// indented deeper than keyIndent, is ciphertext — a sops ENC[…] value or an age/pgp
// armor block. Anything else counts as plaintext and refuses.
func blockScalarEncrypted(lines []string, from, keyIndent int) bool {
	encrypted := false
	for j := from; j < len(lines); j++ {
		body := strings.TrimRight(lines[j], " \t\r")
		if strings.TrimSpace(strings.TrimLeft(body, "+- ")) == "" {
			continue
		}
		indent := len(body) - len(strings.TrimLeft(strings.TrimLeft(body, "+-"), " \t"))
		if strings.HasPrefix(body, "+") || strings.HasPrefix(body, "-") {
			indent--
		}
		if indent <= keyIndent {
			break
		}
		if strings.Contains(body, "ENC[AES256_GCM") {
			encrypted = true
			continue
		}
		if m := rePEMBegin.FindStringSubmatch(body); m != nil && encryptedArmorLabels[strings.TrimSpace(m[1])] {
			encrypted = true
			continue
		}
		if !encrypted {
			return false
		}
	}
	return encrypted
}

// isPathLike reports whether run is a path rather than a credential — the SECOND
// exemption to the high-entropy-run rule. `/`, letters and digits are all valid base64
// characters, so `/Users/operator/work/example/slides`,
// `tools/desk/internal/deskkit/bodycheck` and a base64 blob are indistinguishable by
// charset alone. A run is exempted when ALL of:
//
//  1. it contains at least one `/`. A credential is normally one unbroken token, so a
//     40-char GitHub/hex/base64 secret with no slash is refused by this alone.
//  2. it contains no `+` or `=` — the base64 special and padding characters, which a
//     genuine path segment never carries.
//  3. the slash-separated segments that are NOT word-shaped (see looksLikeWords), taken
//     together, total fewer than maxOpaqueInPath characters. A git-SHA segment counts as
//     non-opaque, since a bare SHA is already exempt above.
//
// (3) is the security-bearing rule and it is what makes this narrower than a length-only
// segment gate. HISTORY, because both previous versions were wrong in opposite
// directions and the next maintainer needs to see the shape of the trap:
//
//   - The ORIGINAL rule required the run to be anchored at `^/(Users|home|private|tmp|opt)/`
//     and every segment to be <32 chars. The anchor refused every repo-relative
//     `file:line` reference — which is the finding format desk review bodies are REQUIRED
//     to use — and every Go module path. Review bodies dense with evidence could not
//     post; the refusals then consumed the outward-write budget, starving the queue
//     (#209, #1255). It also refused this repo's own `go test` output,
//     which the methodology mandates as Evidence.
//   - The FIRST fix (#1261) dropped the anchor and kept only "every segment <32
//     chars". That is too wide: it no longer requires anything path-SHAPED, so a token
//     whose slashes merely fall in convenient places is exempted. Its reviewer
//     demonstrated five regressions against the anchored rule, the sharpest being an AWS
//     **secret access key** — 40 base64 chars that routinely contain `/`, whose segments
//     are all under 32. `AKIA…` (the access key ID) is caught by reAWSKeyID above; the
//     secret access key is a different string and was NOT caught. That reviewer also
//     showed the structural discriminators it had considered — case-transition density,
//     mean segment length — genuinely cannot separate `Documentation/Backend/ConfigurationPanel`
//     from that key: both are 40 chars, 3 segments, mean segment 12.7, mixed case.
//
// Rule (3) separates them on a different axis, and that is the point: not "how random do
// these characters look" (they look identical) but "are the segments built out of
// WORDS". `Documentation`, `Backend` and `ConfigurationPanel` decompose into word units;
// `wJalrXUtnFEMI`, `K7MDENG` and `bPxRfiCYEXAMPLEKEY` do not, and their lengths sum past
// the run threshold.
//
// Empty segments (`//` in a URL run, a trailing `/` on a directory reference) are skipped
// rather than rejected: they carry no material, and rejecting them was an early draft of
// the #1261 fix that re-created the bug on `//localhost/api/v2/state/deployments`
// and on `pkg/service/internal/api/handlers/`. Both are pass cases below.
//
// RESIDUAL GAP — MEASURED, NOT ASSERTED. This paragraph used to end "…which no real
// credential format does". #410 measured that clause and it is false: under
// rule (3) alone, 3.07% of runs in the published AWS example key's own slash layout were
// admitted. Rule (4) below is the response. The residual that remains after it is stated
// with its sample size by TestPathRuleDifferential, and is a number in this file's tests
// rather than a sentence in its comments — precisely because a maintainer reasons from
// what a comment like the old one asserts.
//  4. once ANY opaque material is present, the word-shaped segments must OUTNUMBER the
//     opaque ones. This is the #410 rule and it closes the residual the
//     paragraph above used to call closed.
//
// RULE (4), and why the paragraph above was wrong. #410 measured the residual instead of
// asserting it: over 2,000,000 runs forced into the published AWS example key's own 13/7/18
// slash layout, rule (3) alone ADMITTED 3.07% — one in thirty-three — because a random
// seven-character segment reads as word-shaped often enough to drop the opaque total to 31,
// one under the budget. The claim "which no real credential format does" was false of the
// single credential format this rule names.
//
// The mechanism is a LONE lucky segment rescuing a run that is otherwise wholly opaque, so
// rule (4) is written against that mechanism rather than against the layout: a genuine path
// is mostly words with the occasional `v2` or hash directory among them, while a credential
// wearing slashes is mostly opaque with the occasional lucky draw among them. Counting
// segments — not characters — separates the two without touching the character budget that
// rule (3) already spends on real paths.
//
// It is deliberately the SEGMENT-COUNT axis and not a character RATIO. A ratio
// (`opaque*2 < len(run)`) also closes the #410 class, but it newly refuses ordinary paths
// that carry one long opaque component — a build-output directory named after a content
// hash, a cache path — and the false-positive direction is the expensive one here (#209,
// #1255: refusing `file:line` references starved the desk's write budget). The differential
// behind both statements, in both directions with its sample sizes, is
// TestPathRuleDifferential.
func isPathLike(run string) bool {
	if !strings.Contains(run, "/") {
		return false
	}
	if strings.ContainsAny(run, "+=") {
		return false
	}
	opaque, wordSegs, opaqueSegs := 0, 0, 0
	for _, seg := range strings.Split(run, "/") {
		if seg == "" || isGitSHA(seg) {
			continue
		}
		if looksLikeWords(seg) {
			wordSegs++
			continue
		}
		opaque += len(seg)
		if opaque >= maxOpaqueInPath {
			return false
		}
		// A short all-DIGIT segment counts toward the character budget but not toward the
		// segment COUNT. `docs/reports/2026/07/30/…` is three such segments in a row, and
		// counting them as opaque made rule (4) refuse every dated report path in this
		// repo's own docs tree — measured, in the false-positive half of
		// TestPathRuleDifferential, which is why this carve-out exists at all. Digits are
		// path STRUCTURE (dates, versions, ports, ids), they are bounded by maxDigitRun,
		// and a credential's opaque spans are not all-digit: over the #410 population the
		// chance a rescuing segment is pure digits is (10/62)^7, which is noise.
		if !isShortDigitRun(seg) {
			opaqueSegs++
		}
	}
	if opaqueSegs > 0 && wordSegs <= opaqueSegs {
		return false
	}
	return true
}

// isShortDigitRun reports whether seg is entirely digits and short enough to be path
// structure rather than payload — see isPathLike's rule (4) for why the distinction is
// counted rather than budgeted.
func isShortDigitRun(seg string) bool {
	if seg == "" || len(seg) > maxDigitRun {
		return false
	}
	for i := 0; i < len(seg); i++ {
		if seg[i] < '0' || seg[i] > '9' {
			return false
		}
	}
	return true
}

// isIdentifierLike reports whether run reads as a single bare identifier built entirely
// out of words — the THIRD exemption to the high-entropy-run rule (#253).
//
// Go test names routinely clear the 32-char run threshold in CamelCase —
// `HonoredFilterLeavesTripwireSilent` is 33 — with no `/` anywhere to trigger
// isPathLike's exemption above; a review body naming the tests it exercises trips the
// high-entropy refusal on ordinary prose. This reuses wordDecomposition — the same
// word-decomposition that already separates a path segment like `Widget`
// from an AWS secret access key's `bPxRfiCYEXAMPLEKEY` does the identical job on a whole
// run instead of a slash-delimited piece of one, for the same structural reason —
// the scan rejects on the FIRST lone capital, ALL-CAPS stretch, or 1-2 char lowercase
// debris (a capital plus ONE lowercase is debris too, unless the pair is one of the
// two-letter English words in twoLetterWords), which is what uniformly random MIXED-case
// base64 material produces almost immediately. (LOWERCASE random material does not
// produce those shapes — that is the hole the capital-led requirement below closes.)
// A genuine identifier decomposes cleanly into CamelCase word units.
//
// UNLIKE looksLikeWords (one word unit is enough for a path SEGMENT — `tools`,
// `internal` — because a path supplies its own word boundaries via `/`), this requires
// AT LEAST TWO word units. A bare run has no such external boundary, and without this a
// single unbroken lowercase span of up to maxWordSegment characters — exactly the shape
// of a lowercased hex/hash string, e.g. a 32-char run one character short of the git-SHA
// exemption — decomposes as ONE long "word" and would be wrongly exempted. Requiring two
// units is also true to the actual bug report: every real CamelCase Go test name is a
// compound of several words, never one.
//
// It ALSO requires at least one word unit to be capital-led (a CamelCase word, not an
// all-lowercase run). The two-word floor alone stops only an UNBROKEN lowercase span:
// realistic lowercase token material — a hex hash, a base36 token — carries short digit
// groups, and every digit group ends one letter-word unit and starts another, so a
// 32-char lowercase md5-shaped value decomposes into FIVE word units and slipped the
// floor (demonstrated end-to-end through BodyCheck in the #582 review:
// both a bare lowercase hex value and a lowercase base36 token in prose were admitted).
// The pre-existing refuse case "32-char lowercase hex is too short to be an exempt sha"
// did not catch this because its fixture is one repeated letter pair — an unbroken span,
// ONE word — while real hex always carries digits. The capital-led requirement keeps the
// exemption on its actual bug class: every #253 repro identifier is CamelCase. No
// legitimate name shape is lost: a genuine multi-word lowercase name carries `_`/`-`
// separators, which are outside the base64 charset, so it never forms one 32+ char run
// in the first place — a separator-free lowercase run of that length is token-shaped,
// not name-shaped, and refusing it is the seatbelt doing its job.
//
// This does not widen the path exemption: a run containing `/`, `+` or `=` already fails
// the decomposition in its default case, so isIdentifierLike never overlaps with — or
// substitutes a weaker test for — isPathLike's slash-aware opaque-budget accounting.
//
// A short ALL-CAPS acronym that CLOSES the identifier — the `OK` of `…SilenceAsNotOK`, the
// `HTTP` of `…RequestSpeaksHTTP` — is admitted: wordDecomposition's capital branch takes a
// trailing run of 2-4 uppercase letters as one word unit (see endsInShortAcronym), so the
// run rides on the strength of its word-shaped body and the maxIdentifierRun cap governs its
// length. Only the CLOSING position is relaxed and only for 2-4 letters, because that is the
// bound that keeps the relaxation off real token material: everything before the tail must
// still decompose into word units under the normal rules, and a lone trailing capital, a
// 5+ letter ALL-CAPS tail, or an acronym anywhere but the end stays refused.
//
// KNOWN RESIDUAL GAP, documented rather than claimed closed: an ALL-CAPS acronym in the
// MIDDLE of an identifier — `TestHandlesHTTPSProxy…UpstreamHealth`, elided here because the
// unelided name trips the very gap this paragraph describes — is still refused once the run
// clears 32 characters, and so is a trailing acronym of 5+ letters. looksLikeWords' comment
// calls that acceptable, and for a path SEGMENT it is: failing there only spends opaque
// budget. Here it is a refusal, so the cost is real. It is left open on purpose, because a
// mid-run capital with no lowercase behind it is the same shape that keeps a Slack webhook
// token's `XXXXXXXX…` tail and an AWS key's `bPxRfiCYEXAMPLEKEY` opaque, and — away from the
// closing position that bounds it — no list of known acronyms separates the two the way
// twoLetterWords separates English words from base64 debris. Such a name over the threshold
// must be reworded or broken across the line, which — unlike a recorded Evidence command
// (#775) or a shipped test identifier (#663) — is always available to the author.
func isIdentifierLike(run string) bool {
	if len(run) > maxIdentifierRun {
		return false
	}
	ok, words, camel := wordDecomposition(run, true /* allowTrailingAcronym */)
	return ok && words >= 2 && camel
}

// isAssignmentLike reports whether run is a `key=value` shell assignment whose VALUE is
// itself already exempt — the FOURTH exemption to the high-entropy-run rule (#775).
//
// The Verify/Evidence rows this methodology mandates quote the exact command that was
// run, and the recurring idiom is a scratch variable holding a path:
//
//	f=docs/streams/…/version-scheme.md; test -f "$f" && …
//
// The base64ish run there is `f=docs/streams/…/version` — 35 characters in full, over
// the threshold. (Long spans are elided with `…` throughout this comment so the comment
// itself does not trip the PRE-fix scanner on the diff that introduces the fix. It is the
// same bootstrap the split fixtures in this package's tests work around, recorded at
// scanSecret40 there.) isPathLike refuses it at its `+=` gate, because `=` is base64 padding
// and a genuine path segment never carries one; isIdentifierLike refuses it in the
// default case for the same character. So the row could not be posted at all, and it
// could not be reworded either: rewriting a recorded Evidence command after the fact to
// dodge the scanner desyncs the audit trail from what was actually run (#775).
//
// SECURITY SHAPE — this is a prefix STRIP, not a new exemption. The value half must
// independently satisfy isGitSHA, isPathLike or isIdentifierLike — exactly the bar it
// would have to clear standing alone, no weaker and no stronger; all this function adds
// is permission to ignore a short or word-shaped `key=` in front of it. The git-SHA arm
// is there for that "no stronger" half: a bare SHA is exempt, and the methodology quotes
// SHAs constantly, so `base=<sha>` must not refuse what `<sha>` alone permits.
// The concrete guard-strength case is an AWS secret
// access key wearing the same idiom — `f=wJalrXUtnFEMI/…/bPxRfiCYEXAMPLEKEY`. The
// key half is fine, but the value is the bare AWS key, which isPathLike already refuses
// on opaque budget, so the run still refuses. That row is in the tests by name, because
// #1261's table once asserted a shell-assigned AWS key was "still caught" when in truth
// only the stray `=` was catching it.
//
// Three narrow conditions, each load-bearing:
//
//  1. The split is at the FIRST `=`, and the value may not contain another. This is what
//     keeps base64 PADDING out: `data=ZGVhZGJlZWZkZWFkYmVlZg==` has `=` in its value and
//     is refused, as is any `a=b=<secret>` nesting.
//  2. The key is either at most maxBareAssignKey characters (`f`, `d`, `p` — real shell
//     scratch variables, and at most two characters of opaque material) or word-shaped
//     under looksLikeWords (`file`, `manifest` — name material, not credential material).
//     `AWS_SECRET_ACCESS_KEY=<key>` reduces to a run keyed `KEY`, which is three
//     characters of ALL-CAPS: too long to be bare, not word-shaped, so refused here too.
//  3. The value carries no `+` either. isPathLike and isIdentifierLike both already
//     refuse `+`; the check is restated here on purpose, so that a future loosening of
//     either predicate cannot quietly open a padding-bearing route through this one.
func isAssignmentLike(run string) bool {
	eq := strings.IndexByte(run, '=')
	if eq <= 0 || eq == len(run)-1 {
		return false // no `=`, nothing before it, or nothing after it
	}
	key, val := run[:eq], run[eq+1:]
	if len(key) > maxBareAssignKey && !looksLikeWords(key) {
		return false
	}
	if strings.ContainsAny(val, "+=") {
		return false
	}
	return isGitSHA(val) || isPathLike(val) || isIdentifierLike(val)
}

// looksLikeWords reports whether seg reads as human-written name material — what real
// path, package and file-name segments are made of — rather than as opaque token
// material. seg qualifies when it decomposes ENTIRELY into:
//
//   - lowercase runs of at least minLowerWord letters (`tools`, `internal`, `deskkit`);
//   - CamelCase words: one uppercase letter followed by at least two lowercase
//     (`Registry`, `Cache`, `Container`, the `Users` of `/Users/…`), or one of the
//     two-letter English words in twoLetterWords (`Is`, `On`, `Vs`, `An`);
//   - digit groups of at most maxDigitRun (`python311`, `v2` — note `v2` fails on the
//     one-letter `v`, which is fine: it contributes 2 characters of opaque budget);
//
// …and contains at least one word unit (so a bare number is not "words") and is at most
// maxWordSegment long (so a degenerate 200-character lowercase stretch is not either).
//
// Base64 material fails this almost immediately and for a reason that is structural
// rather than tuned: uniformly random characters produce one- and two-letter lowercase
// runs, lone capitals, and ALL-CAPS stretches constantly, and any ONE of those
// disqualifies the segment. An acronym-heavy real name (`HTTPSProxy`) also fails — that
// is acceptable, because failing only spends opaque budget; it does not refuse the body
// unless the run carries 32+ characters of such material.
//
// A single word unit is enough HERE because a path segment inherits its boundary from the
// `/` around it. isIdentifierLike, which has no such boundary, requires two — and that at
// least one be capital-led. The capital-led requirement is deliberately NOT applied here:
// a path segment like `tools` or `deployments` is legitimately all-lowercase and must
// stay name material (it spends no opaque budget), while a bare lowercase run carrying
// digit groups is exactly the token shape isIdentifierLike exists to refuse.
func looksLikeWords(seg string) bool {
	// The maxWordSegment cap lives HERE, on the path-segment caller, since
	// wordDecomposition no longer applies it (isIdentifierLike wants the wider
	// maxIdentifierRun cap instead). A path segment past this length is a degenerate
	// run — a 200-char lowercase stretch is word-SHAPED but is not a word — and belongs
	// on the opaque budget, where isPathLike's length gate can refuse it.
	if len(seg) > maxWordSegment {
		return false
	}
	// allowTrailingAcronym is left OFF here: the trailing-acronym relaxation exists to keep a
	// bare IDENTIFIER (words>=2 && camel) from the high-entropy refusal, and admitting it for a
	// path SEGMENT would loosen the opaque-budget accounting isPathLike/isAssignmentLike rest
	// on. A path segment that ends in an acronym still fails here and spends opaque budget,
	// exactly as before — this predicate's verdict is unchanged by the relaxation.
	ok, words, _ := wordDecomposition(seg, false /* allowTrailingAcronym */)
	return ok && words > 0
}

// wordDecomposition is the scan looksLikeWords and isIdentifierLike are both built on: it
// reports whether seg decomposes ENTIRELY into word/digit units (ok) and, if so, how many
// word units (letter runs, not digit groups) it contains and whether any of them is
// capital-led — a CamelCase word rather than an all-lowercase run (camel). Split out so
// isIdentifierLike can require MORE than one word unit, at least one of them capital-led,
// without hand-duplicating this loop — see its doc comment for why the two callers need
// different thresholds, and looksLikeWords' for why the capital-led flag is ignored there.
//
// It applies NO length cap of its own: each caller caps length for its own security bar —
// looksLikeWords at maxWordSegment (a path segment), isIdentifierLike at the wider
// maxIdentifierRun (a bare identifier). See those two functions.
//
// allowTrailingAcronym lets the FINAL word unit be a short (2-4 letter) ALL-CAPS acronym
// closing the segment (see endsInShortAcronym). isIdentifierLike passes true so a real
// identifier ending in an acronym (`…SilenceAsNotOK`) is admitted rather than refused as a
// high-entropy run; looksLikeWords passes false so the path-segment opaque-budget accounting
// it feeds is left exactly as it was. The relaxation touches only the CLOSING position and
// only 2-4 letters, so the run is still admitted on the strength of its word-shaped body.
func wordDecomposition(seg string, allowTrailingAcronym bool) (ok bool, words int, camel bool) {
	for i := 0; i < len(seg); {
		c := seg[i]
		switch {
		case c >= 'a' && c <= 'z':
			j := i
			for j < len(seg) && seg[j] >= 'a' && seg[j] <= 'z' {
				j++
			}
			if j-i < minLowerWord {
				return false, 0, false // `b`, `zz` — base64 debris, not a word
			}
			i, words = j, words+1
		case c >= 'A' && c <= 'Z':
			j := i + 1
			for j < len(seg) && seg[j] >= 'a' && seg[j] <= 'z' {
				j++
			}
			// A capital normally needs two lowercase letters behind it to read as a
			// CamelCase word. Three exceptions, in decreasing looseness:
			//
			//   n == 1 and the pair is a two-letter English word from twoLetterWords
			//   (`…IsOn…`, `…OnDemandVsScheduled…`, `…HandlesAnEmpty…`). Without it a single
			//   `Is` anywhere in a 32+ char Go test name sent the whole identifier to the
			//   high-entropy refusal (#781); `Is` alone blocks 124 real identifiers here.
			//
			//   n == 0 and the capital is the one-letter English word `A` or `I`, AND the
			//   NEXT unit is itself a capital-led CamelCase word (see startsCapitalLedWord):
			//   the `A` of `…WithoutAWrite`, the `I` of `…WhenIReturn`. It counts as a word
			//   but does NOT set camel — a lone capital is not itself CamelCase; the word that
			//   follows supplies that. The capital-led-FOLLOWER requirement is load-bearing:
			//   it is what keeps `AAAA…`, ALL-CAPS stretches (`K7MDENG`) and `XXXX…` refusing,
			//   because their lone capital is followed by another bare capital, never a word.
			//
			//   n == 0, allowTrailingAcronym is set, and this capital opens a TRAILING run of
			//   2-4 uppercase letters closing the segment (see endsInShortAcronym): the `OK`
			//   of `…SilenceAsNotOK`, the `HTTP` of `…RequestSpeaksHTTP`. It counts as a
			//   capital-led word (camel), consuming the rest of the segment. This is the ONLY
			//   place a capital with no lowercase behind it is admitted for its own sake, and
			//   it is bounded to the closing position and to 2-4 letters precisely so it does
			//   NOT admit a mid-run ALL-CAPS stretch (`…HTTPSProxy…`, an AWS key's EXAMPLEKEY
			//   sitting inside a longer run) or a 5+ letter tail — those still refuse below.
			//
			// A capital with ZERO lowercase behind it and no qualifying follower or trailing
			// acronym — a lone capital or an ALL-CAPS stretch — is still refused outright.
			n := j - (i + 1)
			switch {
			case n >= 2:
				i, words, camel = j, words+1, true
			case n == 1 && twoLetterWords[seg[i:j]]:
				i, words, camel = j, words+1, true
			case n == 0 && (c == 'A' || c == 'I') && startsCapitalLedWord(seg, j):
				i, words = j, words+1 // a one-letter word; camel comes from the follower
			case n == 0 && allowTrailingAcronym && endsInShortAcronym(seg, i):
				i, words, camel = len(seg), words+1, true // a short trailing acronym closes the run
			default:
				return false, 0, false
			}
		case c >= '0' && c <= '9':
			j := i
			for j < len(seg) && seg[j] >= '0' && seg[j] <= '9' {
				j++
			}
			if j-i > maxDigitRun {
				return false, 0, false
			}
			i = j
		default:
			return false, 0, false // '+', '=' and anything else non-alphanumeric
		}
	}
	return true, words, camel
}

// startsCapitalLedWord reports whether the unit beginning at index k is a capital-led
// CamelCase word — a capital followed by at least two lowercase letters. It is the lookahead
// the lone-`A`/`I` one-letter-word rule uses in wordDecomposition's capital branch: `A`/`I`
// count as an English word only when a real word follows them (`…WithoutAWrite`), never when
// the next character is itself a lone capital (the ALL-CAPS / random-base64 shape that must
// keep refusing). Two lowercase are required, not one, so the follower is a genuine word and
// not a second piece of two-letter debris.
func startsCapitalLedWord(seg string, k int) bool {
	return k+2 < len(seg) &&
		seg[k] >= 'A' && seg[k] <= 'Z' &&
		seg[k+1] >= 'a' && seg[k+1] <= 'z' &&
		seg[k+2] >= 'a' && seg[k+2] <= 'z'
}

// endsInShortAcronym reports whether seg[i:] is a TRAILING all-caps acronym — 2 to 4
// uppercase letters running to the end of seg with nothing after them. It is the lookahead
// wordDecomposition's capital branch uses (only when its caller opts in via
// allowTrailingAcronym) to admit a short acronym that closes an otherwise word-shaped
// identifier — the `OK` of `…SilenceAsNotOK`, the `HTTP` of `…RequestSpeaksHTTP` — so
// isIdentifierLike accepts the whole run and the maxIdentifierRun length cap governs it,
// rather than the run being refused as a high-entropy secret once it clears 32 characters.
//
// SECURITY SHAPE — three bounds keep this from opening a route for real token material, and
// each is load-bearing:
//
//   - TRAILING only: the acronym must reach the end of seg. Everything BEFORE it must still
//     decompose into word units under the normal rules, so the run is admitted on the
//     strength of its word-shaped body, not this tail. A mid-run ALL-CAPS stretch
//     (`…HTTPSProxy…`, an AWS key's `…EXAMPLEKEY…` inside a longer run) does not reach the
//     end and still refuses at the capital branch's default case.
//   - SHORT: 2 to 4 letters. A lone trailing capital stays debris — a 1-char tail carries no
//     acronym meaning and is the shape a truncated token ends on — and a 5+ letter ALL-CAPS
//     tail (a Slack webhook's `XXXX…`, an AWS key's 10-char `EXAMPLEKEY`) still refuses.
//   - ALL-CAPS: every byte A-Z. A lowercase letter or digit in the tail means it is not an
//     acronym, and the normal rules apply.
//
// The bar the RUN still has to clear is unchanged: isIdentifierLike requires words>=2 and a
// capital-led word, so a bare `OK` (one word, whole run) is still refused, and the random-
// base64 guard stays at zero admits because a 2-4 caps tail cannot help a 90+ char body that
// must still decompose.
func endsInShortAcronym(seg string, i int) bool {
	tail := len(seg) - i
	if tail < 2 || tail > 4 {
		return false
	}
	for k := i; k < len(seg); k++ {
		if seg[k] < 'A' || seg[k] > 'Z' {
			return false
		}
	}
	return true
}
