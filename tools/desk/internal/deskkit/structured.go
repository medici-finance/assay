package deskkit

import (
	"regexp"
	"strings"
)

// Structured-format awareness for the shared secret scan (ground-truth/06).
//
// WHY THIS FILE EXISTS AT ALL, AND WHY IT IS NOT A PILE OF EXEMPTION PATCHES.
//
// #781 catalogued four (later nine) structurally different false-positive
// shapes in a single session and argued — correctly — that patching each shape as its own
// content pattern would miss the fifth and sixth. The shapes it lists are not arbitrary:
// go.sum checksum lines, package-lock integrity fields and sops envelopes are all
// MACHINE-GENERATED FIELDS OF A KNOWN FILE FORMAT, in which a high-entropy run is not
// merely tolerable but REQUIRED by the format. That is the generalisation this file
// implements: recognise the FORMAT (its field marker, its digest algebra, its envelope
// grammar), and exempt the exact bytes the format accounts for — nothing else on the line,
// nothing else in the document.
//
// Three properties make that safe, and each is load-bearing:
//
//  1. EVERY exemption is ANCHORED on a field marker the format supplies (` h1:`,
//     `"integrity": "sha512-`, `ENC[AES256…,data:`). A bare digest with no marker in
//     front of it is refused exactly as before. So this cannot be used to launder a
//     credential unless the author also writes the surrounding format around it — at
//     which point the credential is not what the field would hold.
//  2. EVERY exemption is LENGTH-EXACT where the format fixes a length. A go.sum h1 digest
//     is base64 of sha256: 43 payload characters plus one pad, never 44 of payload. An
//     SRI digest is 44/64/88 by algorithm. A run one character off the algebra is not the
//     field, and refuses.
//  3. Exemptions apply ONLY to the high-entropy-run loop. The literal-pattern arms of
//     ScanSurface — GitHub token prefixes, AWS access-key IDs, JWT shapes, private-key PEM
//     headers — run over the WHOLE surface regardless of any span computed here. A
//     `ghp_…` token pasted inside a sops block still refuses. This is the containment that
//     makes the sops-block exemption below tolerable rather than a hole.
//
// See docs/desk-tools-gate-bar.md for the ship bar these rules are measured against, and
// testdata/corpus/ for the golden corpus (both directions) that pins them.

// span is a half-open byte range [lo,hi) of the scanned surface.
type span struct{ lo, hi int }

// armorDashes is the five-dash run of a PEM/armor delimiter, held as its own constant so
// that no source line in this package ever carries the contiguous five-dash-BEGIN literal.
// That is not style: the pre-fix scanner refuses any surface containing it, and it scans
// REMOVED diff lines too, so a file that spells the marker out cannot be edited through
// the tools it guards (#380). Composing it keeps this file maintainable by
// deskpr.
const armorDashes = "-----"

var (
	// reGoSumDigest matches ONE go.sum (or go.mod `h1:`) line and captures its digest.
	//
	// The grammar go.sum guarantees: `<module-path> <version>[/go.mod] h1:<digest>`, one
	// per line, digest = standard-base64 of a sha256, i.e. exactly 43 payload characters
	// and one `=` pad. Anchoring on ` h1:` AND on that exact length is what keeps this
	// from being "a 44-char base64 run is fine": a 44-char run with no `h1:` marker in
	// front of it, or a marker followed by 45 characters, is not a module checksum.
	//
	// A leading unified-diff marker is tolerated because the surface deskpr scans is a
	// diff. deskpr strips the marker itself (#812) — this is belt and braces
	// for any other caller that hands a raw diff to the scan.
	reGoSumDigest = regexp.MustCompile(`(?m)^[+\- ]?[ \t]*[^ \t\n]+[ \t]+v[^ \t\n]+[ \t]+h1:([A-Za-z0-9+/]{43}=)[ \t]*$`)

	// reSRIDigest matches a Subresource-Integrity digest field as npm/yarn/pnpm lockfiles
	// write it, and captures the algorithm and the payload separately so the payload's
	// length can be checked against the algorithm's digest size (see sriPayloadLen).
	//
	// Anchored on the `integrity` KEY, not on the `sha512-` prefix alone: `sha512-<blob>`
	// appearing anywhere in prose is not a lockfile field and stays scanned.
	reSRIDigest = regexp.MustCompile(`"?integrity"?[ \t]*:[ \t]*"?(sha256|sha384|sha512)-([A-Za-z0-9+/]+={0,2})"?`)

	// reSopsEnvelope matches a COMPLETE sops encrypted-value envelope and captures its
	// three ciphertext payloads. sops writes every encrypted value as
	// `ENC[AES256…,data:…,iv:…,tag:…,type:…]` — all four fields, in that order. A
	// partial marker (`ENC[AES256…,data:x]`) is NOT this shape and is refused by
	// ScanSurface's sops arm, which is what keeps #585's coincidental
	// substring and a hand-written imitation on the refusing side.
	reSopsEnvelope = regexp.MustCompile(
		`ENC\[AES256_GCM,data:([A-Za-z0-9+/=]*),iv:([A-Za-z0-9+/=]*),tag:([A-Za-z0-9+/=]*),type:([a-z]+)\]`)

	// reArmorBegin matches an armor/PEM BEGIN delimiter and captures the rest of its line,
	// which is what decides whether the delimiter is secret-bearing (see
	// armorBeginIsSecretBearing).
	reArmorBegin = regexp.MustCompile(`-{5}BEGIN([^\n]*)`)

	// reArmorLabel matches the LABEL half of a well-formed delimiter — a space, an
	// upper-alnum label, and the closing five dashes. A `-{5}BEGIN` occurrence that does
	// not match this is not a delimiter at all: it is the marker quoted in prose or in the
	// detector's own source (#380).
	reArmorLabel = regexp.MustCompile(`^ ([A-Z0-9][A-Z0-9 ]*)-{5}`)

	// reArmorBlock matches a complete armor block — BEGIN delimiter, body, END delimiter —
	// capturing the label and the body. Non-greedy so adjacent blocks do not merge.
	reArmorBlock = regexp.MustCompile(`(?s)-{5}BEGIN ([A-Z0-9][A-Z0-9 ]*)-{5}(.*?)-{5}END [A-Z0-9][A-Z0-9 ]*-{5}`)
)

// privateKeyLabel is the substring that makes an armor label secret-bearing whatever else
// the label says: `RSA PRIVATE KEY`, `OPENSSH PRIVATE KEY`, `ENCRYPTED PRIVATE KEY`, and
// the bare `PRIVATE KEY` all carry it.
const privateKeyLabel = "PRIVATE KEY"

// armorCiphertextOrPublicLabels are the armor labels whose payload is CIPHERTEXT or PUBLIC
// material — nothing usable without a key the payload does not contain. They are the only
// labels this scan does not treat as a leaked private key.
//
// The set is CLOSED and an unknown label REFUSES. That is the fail-closed half of
// replacing the old `strings.Contains(s, <five-dash BEGIN>)` blanket: a novel or
// misspelled label is still a refusal, so the change cannot be walked past by inventing a
// label. What it buys is #380 (the detector's own quoted marker, which is not
// a well-formed delimiter at all) and #778 (a sops age/pgp recipient block,
// whose payload is the data key encrypted TO a recipient).
var armorCiphertextOrPublicLabels = map[string]bool{
	"AGE ENCRYPTED FILE":       true,
	"PGP MESSAGE":              true,
	"PGP PUBLIC KEY BLOCK":     true,
	"PGP SIGNATURE":            true,
	"SSH SIGNATURE":            true,
	"CERTIFICATE":              true,
	"TRUSTED CERTIFICATE":      true,
	"X509 CRL":                 true,
	"CERTIFICATE REQUEST":      true,
	"NEW CERTIFICATE REQUEST":  true,
	"PUBLIC KEY":               true,
	"RSA PUBLIC KEY":           true,
	"DH PARAMETERS":            true,
	"EC PARAMETERS":            true,
	"OPENSSH ALLOWED SIGNERS":  true,
	"SIGNED MESSAGE SIGNATURE": true,
}

// armorBeginIsSecretBearing reports whether a `BEGIN` armor delimiter announces PRIVATE
// KEY material — the thing the PEM arm of the scan exists to refuse. rest is everything on
// the line AFTER the delimiter keyword.
//
// Three states, deliberately:
//
//   - the label names private-key material           → secret-bearing (refuse)
//   - the label is a well-formed CLOSED-LIST member  → not secret-bearing (ciphertext/public)
//   - the label is well-formed but UNKNOWN           → secret-bearing (fail closed)
//   - there is no well-formed label at all           → not a delimiter (#380)
//
// The last state is the one that lets the detector's own source through. `bodycheck.go`
// carries the marker as a quoted Go string and inside a refusal message that elides the
// label; neither is followed by an upper-alnum label and closing dashes, so neither is a
// PEM header — which is what #380 has been saying since it was filed, and why
// four PRs were pushed around the tool rather than through it.
func armorBeginIsSecretBearing(rest string) bool {
	if strings.Contains(rest, privateKeyLabel) {
		return true
	}
	m := reArmorLabel.FindStringSubmatch(rest)
	if m == nil {
		return false
	}
	return !armorCiphertextOrPublicLabels[strings.TrimSpace(m[1])]
}

// stripBenignArmorDelimiters returns s with every NON-secret-bearing `BEGIN` armor
// delimiter neutralised (the keyword lowercased), so that the literal-marker arm of
// ScanSurface no longer fires on it. Secret-bearing delimiters are returned untouched and
// still refuse.
//
// Only the DELIMITER is rewritten, and only for the marker arm's copy of the surface: the
// body between delimiters is untouched and still reaches the high-entropy-run loop, where
// it refuses unless a structured exemption accounts for it. So a certificate or a stray
// armored blob pasted into a PR body is still refused — with an entropy message rather
// than a PEM one — while the detector's own source, and a sops recipient block, are not.
func stripBenignArmorDelimiters(s string) string {
	if !strings.Contains(s, armorDashes+"BEGIN") {
		return s
	}
	return reArmorBegin.ReplaceAllStringFunc(s, func(m string) string {
		rest := strings.TrimPrefix(m, armorDashes+"BEGIN")
		if armorBeginIsSecretBearing(rest) {
			return m
		}
		return armorDashes + "begin" + rest
	})
}

// markerSurface is the view of raw that ScanSurface's literal-marker arms read: raw with
// every marker that STRUCTURED RECOGNITION has already accounted for neutralised, and
// nothing else touched.
//
// Two neutralisations, each narrow and each with its own safety argument:
//
//   - a BENIGN armor delimiter (stripBenignArmorDelimiters). Only the delimiter keyword is
//     rewritten, only when the label is not private-key material, and the body between the
//     delimiters still reaches the high-entropy-run loop — so a stray armored blob is still
//     refused, just with an entropy message rather than a PEM one.
//   - the sops markers of a SANCTIONED encrypted-at-rest manifest
//     (neutraliseSopsMarkers), and only of one: the document signature, a complete envelope
//     grammar, and encrypted content outside the metadata block must ALL be present. An
//     imitation, a truncated paste, or a bare footer keeps its markers and refuses.
//
// Both are decided on `raw`, never on the partially-neutralised copy, so one neutralisation
// can never enable another.
//
// EVERY OTHER ARM of the scan still runs over the whole surface: a GitHub token, an AWS
// access-key ID, a JWT or a private key inside a sanctioned sops manifest refuses exactly
// as it would anywhere else. That containment is what makes the sops policy a recognition
// of ciphertext rather than a hole under a heading.
func markerSurface(raw string) string {
	s := stripBenignArmorDelimiters(raw)
	if sanctionedSopsDocument(raw) {
		s = neutraliseSopsMarkers(s)
	}
	return s
}

// neutraliseSopsMarkers lowercases the encrypted-value marker and renames the `sops:`
// mapping key on the MARKER SURFACE only, so the sops arm of ScanSurface does not fire on a
// document that has already been recognised as a sanctioned manifest. It is a rename, not a
// deletion: the bytes stay, their length stays, and the run loop still reads `raw`.
func neutraliseSopsMarkers(s string) string {
	s = reSopsEncVal.ReplaceAllString(s, "enc[AES256_GCM")
	return reSopsKey.ReplaceAllStringFunc(s, func(m string) string {
		return strings.Replace(m, "sops", "sops-recognised", 1)
	})
}

// sriPayloadLen is the exact base64 length (padding included) of a Subresource-Integrity
// digest for each algorithm SRI defines: base64 of 32, 48 and 64 raw bytes.
var sriPayloadLen = map[string]int{"sha256": 44, "sha384": 64, "sha512": 88}

// sopsDocument reports whether s carries the sops DOCUMENT signature — a line-anchored
// `sops:` mapping key AND, under it, a sops-metadata field. Both are required: prose that
// merely says "sops" has neither, and a lone `sops:` line in prose has only the first.
func sopsDocument(s string) bool {
	return reSopsKey.MatchString(s) && reSopsField.MatchString(s)
}

// sanctionedSopsDocument reports whether s is a genuine encrypted-at-rest sops MANIFEST.
// Three independent structures must all be present, and each excludes a different
// imitation:
//
//  1. the document signature — a line-anchored `sops:` mapping key AND a sops-metadata
//     field under it. Prose that says "sops" has neither.
//  2. at least one COMPLETE `ENC[AES256…,data:…,iv:…,tag:…,type:…]` envelope — all four
//     fields, in order. A truncated paste, a hand-written imitation, or #585's
//     coincidental substring has no such grammar.
//  3. at least one of those envelopes OUTSIDE the `sops:` metadata block — i.e. the
//     document actually has ENCRYPTED CONTENT, not merely a footer.
//
// This is the discriminator the sops policy turns on (#778). The house's
// sanctioned Flux path is `sops encrypt → commit → push → reconcile`, so an encrypted
// manifest is content that is SUPPOSED to be committed; refusing it unconditionally made
// the sanctioned path unusable and pushed desk-console/04 into a human-executed manual
// step the tooling was meant to handle.
//
// (3) is the narrow one and it is worth being explicit about, because the obvious spelling
// stops one step short. Every sops document ends with a `mac:` envelope inside its own
// metadata block — that envelope is the integrity tag over the ciphertext, not ciphertext
// of anything the author wrote. A rule satisfied by (1) and (2) alone therefore admits a
// BARE FOOTER: a `sops:` block with a `mac:` and a `version:` and no encrypted values at
// all. That is not an encrypted-at-rest manifest, it is a fragment, and the entire
// justification for admitting sops content — "the secrets in this file are ciphertext" —
// says nothing about it. Requiring an envelope outside the metadata block is what makes the
// exemption apply to the artifact #778 could not commit and to nothing else.
func sanctionedSopsDocument(s string) bool {
	if !sopsDocument(s) {
		return false
	}
	block, ok := sopsMetadataBlock(s)
	for _, m := range reSopsEnvelope.FindAllStringIndex(s, -1) {
		if !ok || m[0] < block.lo || m[1] > block.hi {
			return true
		}
	}
	return false
}

// structuredExemptSpans returns the byte ranges of s that a RECOGNISED STRUCTURED FORMAT
// accounts for, and which the high-entropy-run loop therefore skips. Every span is
// anchored on a field marker of its format; nothing here exempts a bare run.
//
// The spans are: module checksum digests (`h1:`), SRI integrity digests, the ciphertext
// payloads of complete sops envelopes, and — inside a well-formed sops document only — the
// generated `sops:` metadata block and the bodies of armored ciphertext blocks.
func structuredExemptSpans(s string) []span {
	var out []span

	for _, m := range reGoSumDigest.FindAllStringSubmatchIndex(s, -1) {
		out = append(out, span{m[2], m[3]})
	}

	for _, m := range reSRIDigest.FindAllStringSubmatchIndex(s, -1) {
		alg, payload := s[m[2]:m[3]], s[m[4]:m[5]]
		if want, ok := sriPayloadLen[alg]; ok && len(payload) == want {
			out = append(out, span{m[4], m[5]})
		}
	}

	if !sanctionedSopsDocument(s) {
		return out
	}
	// Complete sops envelopes: data, iv and tag payloads (groups 1..3). The `type:` field
	// is a bare word and never forms a run.
	for _, m := range reSopsEnvelope.FindAllStringSubmatchIndex(s, -1) {
		for g := 1; g <= 3; g++ {
			out = append(out, span{m[2*g], m[2*g+1]})
		}
	}
	// The generated `sops:` metadata block: age/pgp recipients, key fingerprints, the
	// encrypted data keys and the MAC. This is machine-written key-management material,
	// none of it plaintext, and it is the half of #778 an envelope-only
	// exemption misses. Containment: the literal-pattern arms still scan this region, so a
	// token or private key hidden under `sops:` is refused by those and not by this.
	if b, ok := sopsMetadataBlock(s); ok {
		out = append(out, b)
		for _, m := range reArmorBlock.FindAllStringSubmatchIndex(s, -1) {
			label := strings.TrimSpace(s[m[2]:m[3]])
			if armorCiphertextOrPublicLabels[label] && m[4] >= b.lo && m[5] <= b.hi {
				out = append(out, span{m[4], m[5]})
			}
		}
	}
	return out
}

// sopsMetadataBlock returns the byte range of the `sops:` mapping and everything nested
// under it, using YAML/JSON indentation to find the end: the block runs to the first
// subsequent NON-BLANK line indented no deeper than the `sops:` line itself. An
// unterminated block runs to the end of the surface, which is the common case (sops writes
// its metadata last).
func sopsMetadataBlock(s string) (span, bool) {
	loc := reSopsKey.FindStringIndex(s)
	if loc == nil {
		return span{}, false
	}
	// reSopsKey's leading `\s*` can span newlines, so the match may START on an earlier
	// blank line than the one carrying the key. Re-anchor on the physical line the key sits
	// on before measuring its indentation, or a blank line above the block silently sets
	// the indent to 0 and the block ends at the first line it should have contained.
	start := strings.LastIndexByte(s[:loc[1]], '\n') + 1
	indent := lineIndent(s[start:])
	pos := loc[1]
	for pos < len(s) {
		nl := strings.IndexByte(s[pos:], '\n')
		if nl < 0 {
			return span{start, len(s)}, true
		}
		lineStart := pos + nl + 1
		if lineStart >= len(s) {
			return span{start, len(s)}, true
		}
		lineEnd := lineStart
		if n := strings.IndexByte(s[lineStart:], '\n'); n >= 0 {
			lineEnd = lineStart + n
		} else {
			lineEnd = len(s)
		}
		line := s[lineStart:lineEnd]
		if strings.TrimSpace(line) != "" && lineIndent(line) <= indent {
			return span{start, lineStart}, true
		}
		pos = lineEnd
		if lineEnd == len(s) {
			return span{start, len(s)}, true
		}
	}
	return span{start, len(s)}, true
}

// lineIndent counts the leading spaces/tabs of a line (a tab counts as one, which is all
// the relative comparison above needs).
func lineIndent(line string) int {
	n := 0
	for n < len(line) && (line[n] == ' ' || line[n] == '\t') {
		n++
	}
	return n
}

// coveredBy reports whether [lo,hi) lies ENTIRELY inside one of the exempt spans. Partial
// overlap is not enough: a run that starts inside a recognised field and continues past it
// is not that field, and must still be judged on its own.
func coveredBy(spans []span, lo, hi int) bool {
	for _, sp := range spans {
		if lo >= sp.lo && hi <= sp.hi {
			return true
		}
	}
	return false
}
