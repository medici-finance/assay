package deskkit

// verdict.go — the verdict payload trust primitive for the verdict-by-issue lane.
//
// A main-side workflow (the transcriber) has to act on an issue
// body it did not write. Trusting the author login alone is spoofable by anyone
// who can edit an issue body, so the payload is SIGNED: the drain engine signs a
// canonical serialisation of the payload with the verifier App's EXISTING RSA
// key (RS256), and the workflow verifies against the verifier PUBLIC key — read
// from the ASSAY_VERIFIER_PUBKEY repo/Actions variable, NOT a committed file —
// before it trusts a single row. Zero new secrets: the private key never leaves
// local custody, and the public half is public material distributed as a variable
// (a local adopter self-generates their own keypair; see payload.md).
//
// Two properties carry the whole scheme and both live in this file:
//
//   - CANONICALISATION IS BYTE-DETERMINISTIC. The signature is over the canonical
//     bytes, and verify RE-CANONICALISES the payload it extracted before it
//     checks — so a payload that was reflowed in transit (a markdown processor
//     pretty-printing the fenced block, a key reordered) still verifies, while
//     any change to the LOGICAL content (a PASS flipped to FAIL, a head SHA
//     edited) changes the canonical bytes and breaks the signature. A subtle
//     malleable serialiser is exactly the class of bug this brief is exec-tier
//     for, so the canonical form is defined here in full rather than delegated to
//     encoding/json's default (which HTML-escapes `<`/`>`/`&` and offers no
//     documented key-ordering contract for arbitrary input).
//
//   - VERIFY TAKES THE PUBLIC KEY ONLY. VerifyVerdictBody never sees a private
//     key; it is safe to run in a public CI job against the pubkey supplied in
//     the ASSAY_VERIFIER_PUBKEY variable.
//
// Three-state result, never two: Verified / Refused / CouldNotCheck. Refused is a
// definite cryptographic negative (the signature does not match the payload).
// CouldNotCheck is a structural surprise (no payload block, no signature trailer,
// unparseable JSON) — it is NOT a synonym for Refused and it is NOT a pass. A
// trust gate treats everything that is not Verified as "do not trust".

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// VerdictSchemaVersion is the payload schema this build speaks. It is carried in
// the payload's "schema" field; a consumer that reads a version it does not
// recognise must refuse rather than guess (see the verdict payload format spec).
const VerdictSchemaVersion = "verdict-v1"

// The issue-body block delimiters. The payload is a fenced code block tagged with
// verdictFenceTag; the signature is an HTML comment trailer so it does not render
// in the issue body a human reads. Both markers are literal and are what
// ParseVerdictBody keys on.
const (
	verdictFenceTag = "verdict-payload"
	// verdictSigPrefix opens the signature trailer; verdictSigAlg names the scheme.
	verdictSigMarker = "deskverdict-signature"
	verdictSigAlg    = "RS256"
)

// verdictSigRE extracts the base64 signature from the trailer comment. Standard
// base64 alphabet (A-Za-z0-9+/=) — the signature is a raw RSA blob, not a JWT, so
// it never carries the eyJ… shape a body scanner refuses.
var verdictSigRE = regexp.MustCompile(verdictSigMarker + `\b[^>]*\bsig=([A-Za-z0-9+/=]+)`)

// CanonicalizeJSON returns the byte-deterministic canonical JSON encoding of raw:
//
//   - object keys sorted lexicographically by UTF-8 code unit;
//   - no insignificant whitespace;
//   - strings escaped by the fixed rule in writeCanonicalString (only the two
//     mandatory escapes " and \, the five short control escapes, and \u00XX for
//     the remaining C0 controls — every other rune, ASCII or not, emitted as raw
//     UTF-8; `<`/`>`/`&` are NOT escaped, unlike encoding/json's default);
//   - integers normalised to their shortest decimal form; any number carrying a
//     fraction or exponent is emitted as its source literal unchanged (the schema
//     uses only integer numbers and strings — see payload.md).
//
// The transform is idempotent: CanonicalizeJSON(CanonicalizeJSON(x)) == CanonicalizeJSON(x),
// which is what lets verify re-canonicalise a payload block that already holds the
// canonical form.
func CanonicalizeJSON(raw []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("payload is not valid JSON: %w", err)
	}
	// Reject trailing tokens: "{}{}" or "{} garbage" is not one JSON value, and a
	// canonicaliser that silently signed only the first would be a malleability hole.
	if dec.More() {
		return nil, errors.New("payload has trailing data after the JSON value")
	}
	var buf bytes.Buffer
	if err := writeCanonical(&buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeCanonical(buf *bytes.Buffer, v interface{}) error {
	switch t := v.(type) {
	case nil:
		buf.WriteString("null")
	case bool:
		if t {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case json.Number:
		buf.WriteString(canonicalNumber(t))
	case string:
		writeCanonicalString(buf, t)
	case []interface{}:
		buf.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeCanonical(buf, e); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	case map[string]interface{}:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			writeCanonicalString(buf, k)
			buf.WriteByte(':')
			if err := writeCanonical(buf, t[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	default:
		return fmt.Errorf("unexpected JSON value of type %T during canonicalisation", v)
	}
	return nil
}

// canonicalNumber normalises an integer literal to its shortest decimal form and
// passes any non-integer literal through unchanged. json.Number preserves the
// exact source token, so passing a fractional/exponent literal through is still
// deterministic AND idempotent; the schema does not use such numbers.
func canonicalNumber(n json.Number) string {
	s := n.String()
	if isIntegerLiteral(s) {
		if i, err := n.Int64(); err == nil {
			return strconv.FormatInt(i, 10)
		}
	}
	return s
}

// isIntegerLiteral reports whether s is a plain optionally-signed integer with no
// fraction and no exponent (JSON already forbids leading zeros, so "01" never
// reaches here).
func isIntegerLiteral(s string) bool {
	if s == "" {
		return false
	}
	i := 0
	if s[0] == '-' {
		i = 1
	}
	if i == len(s) {
		return false
	}
	for ; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// writeCanonicalString writes s as a JSON string with the fixed canonical escape
// rule. It intentionally does NOT escape `<`, `>`, `&` (encoding/json's HTML
// escaping is a source of ambiguity for a signed form) and emits all non-control
// runes as raw UTF-8.
func writeCanonicalString(buf *bytes.Buffer, s string) {
	buf.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\b':
			buf.WriteString(`\b`)
		case '\f':
			buf.WriteString(`\f`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(buf, `\u%04x`, r)
			} else {
				buf.WriteRune(r)
			}
		}
	}
	buf.WriteByte('"')
}

// SignVerdictCanonical signs the SHA-256 digest of canonical with key using
// RSASSA-PKCS1-v1_5 (RS256) and returns the standard-base64 signature. canonical
// must already be the output of CanonicalizeJSON.
func SignVerdictCanonical(canonical []byte, key *rsa.PrivateKey) (string, error) {
	if key == nil {
		return "", errors.New("nil signing key")
	}
	sum := sha256.Sum256(canonical)
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", fmt.Errorf("cannot sign verdict payload: %w", err)
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// VerifyVerdictCanonical verifies sigB64 over canonical using the PUBLIC key. A
// nil error means the signature matches; a non-nil error means it does not (or the
// signature was not decodable). It performs NO canonicalisation itself — callers
// pass canonical bytes from CanonicalizeJSON.
func VerifyVerdictCanonical(canonical []byte, sigB64 string, pub *rsa.PublicKey) error {
	if pub == nil {
		return errors.New("nil public key")
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(sigB64))
	if err != nil {
		return fmt.Errorf("signature is not valid base64: %w", err)
	}
	sum := sha256.Sum256(canonical)
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], sig); err != nil {
		return fmt.Errorf("signature does not verify: %w", err)
	}
	return nil
}

// AssembleVerdictBody produces the issue-body block a signed payload lands as: the
// canonical payload in a fenced code block, then the signature as an HTML-comment
// trailer. canonical must be the CanonicalizeJSON output that sigB64 was computed
// over, so the block is internally consistent and re-verifiable.
func AssembleVerdictBody(canonical []byte, sigB64 string) string {
	var b strings.Builder
	b.WriteString("```")
	b.WriteString(verdictFenceTag)
	b.WriteByte('\n')
	b.Write(canonical)
	b.WriteByte('\n')
	b.WriteString("```\n\n")
	b.WriteString("<!-- ")
	b.WriteString(verdictSigMarker)
	b.WriteString(" v1 alg=")
	b.WriteString(verdictSigAlg)
	b.WriteString(" sig=")
	b.WriteString(sigB64)
	b.WriteString(" -->\n")
	return b.String()
}

// ParseVerdictBody extracts the payload JSON and the base64 signature from an
// issue body assembled by AssembleVerdictBody. It is tolerant of surrounding
// prose: it locates the FIRST fenced block tagged verdictFenceTag and the FIRST
// signature trailer. A missing block or trailer is a structural error the caller
// maps to CouldNotCheck.
func ParseVerdictBody(body string) (payload []byte, sigB64 string, err error) {
	payloadStr, perr := extractFencedPayload(body)
	if perr != nil {
		return nil, "", perr
	}
	m := verdictSigRE.FindStringSubmatch(body)
	if m == nil {
		return nil, "", errors.New("no verdict signature trailer found in body")
	}
	return []byte(payloadStr), m[1], nil
}

// extractFencedPayload returns the inner text of the first ```verdict-payload
// fenced block. The opening fence is a line whose trimmed text is "```" (or a
// longer backtick run) immediately followed by the tag; the closing fence is the
// next line whose trimmed text is a bare backtick run.
func extractFencedPayload(body string) (string, error) {
	lines := strings.Split(body, "\n")
	open := -1
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "```") && strings.TrimSpace(strings.TrimLeft(t, "`")) == verdictFenceTag {
			open = i
			break
		}
	}
	if open < 0 {
		return "", errors.New("no ```" + verdictFenceTag + " block found in body")
	}
	for j := open + 1; j < len(lines); j++ {
		t := strings.TrimSpace(lines[j])
		if strings.HasPrefix(t, "```") && strings.TrimLeft(t, "`") == "" {
			return strings.Join(lines[open+1:j], "\n"), nil
		}
	}
	return "", errors.New("unterminated ```" + verdictFenceTag + " block in body")
}

// VerdictVerifyState is the three-state result of verifying an issue body.
type VerdictVerifyState int

const (
	// VerdictVerified — the signature matches the canonicalised payload.
	VerdictVerified VerdictVerifyState = iota
	// VerdictRefused — a definite cryptographic negative: the payload and
	// signature were both present and well-formed, and the signature does not
	// match. This is the tamper verdict.
	VerdictRefused
	// VerdictCouldNotCheck — a structural surprise that prevented a verdict from
	// being reached at all (no payload block, no signature trailer, unparseable
	// payload JSON). NOT a synonym for Refused, and NOT a pass.
	VerdictCouldNotCheck
)

// VerifyVerdictBody is the full body-level verify: extract, re-canonicalise, and
// check the signature with the PUBLIC key. It returns the three-state result and
// a human-readable message. The Refused message always contains the word
// "refused"; the CouldNotCheck message always contains "could not check".
func VerifyVerdictBody(body string, pub *rsa.PublicKey) (VerdictVerifyState, string) {
	rawPayload, sigB64, err := ParseVerdictBody(body)
	if err != nil {
		return VerdictCouldNotCheck, "could not check: " + err.Error()
	}
	canonical, cerr := CanonicalizeJSON(rawPayload)
	if cerr != nil {
		return VerdictCouldNotCheck, "could not check: " + cerr.Error()
	}
	if verr := VerifyVerdictCanonical(canonical, sigB64, pub); verr != nil {
		return VerdictRefused, "refused: " + verr.Error()
	}
	return VerdictVerified, "verified: signature matches the canonical verdict payload"
}

// ParseRSAPrivateKeyPEM parses an RSA private key from PEM bytes, accepting both
// PKCS#1 ("RSA PRIVATE KEY") and PKCS#8 ("PRIVATE KEY") encodings — the same two
// forms the verifier App JWT minter accepts.
func ParseRSAPrivateKeyPEM(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("no PEM block found")
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	k8, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("cannot parse RSA private key (tried PKCS#1 and PKCS#8)")
	}
	rk, ok := k8.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("PKCS#8 key is %T, not RSA", k8)
	}
	return rk, nil
}

// ParseRSAPublicKeyPEM parses an RSA public key from a PKIX ("PUBLIC KEY") PEM —
// the form DeriveRSAPublicKeyPEM emits and that openssl `pkey -pubin` reads.
func ParseRSAPublicKeyPEM(pemBytes []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("no PEM block found")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		// Tolerate the bare PKCS#1 public-key form too ("RSA PUBLIC KEY").
		if rk, e2 := x509.ParsePKCS1PublicKey(block.Bytes); e2 == nil {
			return rk, nil
		}
		return nil, fmt.Errorf("cannot parse public key: %w", err)
	}
	rk, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is %T, not RSA", pub)
	}
	return rk, nil
}

// VerifierPubkeyVar is the environment / repo (Actions) variable that carries the
// verifier PUBLIC key. It is a repo/Actions VARIABLE, never a secret — the public
// half is public material by construction — and it is NOT committed to the tree.
// A verify step reads it as ${{ vars.ASSAY_VERIFIER_PUBKEY }}; a local adopter
// exports their own self-generated public key into it.
const VerifierPubkeyVar = "ASSAY_VERIFIER_PUBKEY"

// DecodePubkeyVar normalises the value of ASSAY_VERIFIER_PUBKEY into PKIX
// public-key PEM bytes. The variable may hold EITHER form, so both are accepted:
//
//   - a literal PEM string (begins "-----BEGIN … KEY-----"), which is what a
//     shell `export ASSAY_VERIFIER_PUBKEY="$(cat pub.pem)"` produces; or
//   - standard base64 of the PEM bytes, which survives an Actions/repo variable
//     round-trip without newline mangling — the more robust form for CI, so it is
//     the one payload.md tells adopters to store.
//
// It returns the PEM bytes ready for ParseRSAPublicKeyPEM. An empty/whitespace
// value, or a base64 blob that does not decode to a PEM, is an error — never a
// silent pass.
func DecodePubkeyVar(val string) ([]byte, error) {
	s := strings.TrimSpace(val)
	if s == "" {
		return nil, errors.New("empty verifier pubkey value")
	}
	if strings.HasPrefix(s, "-----BEGIN") {
		return []byte(val), nil
	}
	// Not a literal PEM — treat as base64-of-PEM. Tolerate embedded whitespace
	// (a wrapped variable) by stripping it before decoding.
	compact := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, s)
	der, err := base64.StdEncoding.DecodeString(compact)
	if err != nil {
		return nil, fmt.Errorf("%s is neither a PEM string nor valid base64-of-PEM: %w", VerifierPubkeyVar, err)
	}
	if !bytes.Contains(der, []byte("-----BEGIN")) {
		return nil, fmt.Errorf("%s base64-decoded but is not a PEM (no BEGIN marker)", VerifierPubkeyVar)
	}
	return der, nil
}

// DeriveRSAPublicKeyPEM derives the PKIX public-key PEM ("-----BEGIN PUBLIC
// KEY-----") from an RSA private-key PEM. The output is PUBLIC material by
// construction and is exactly what VerifyVerdictBody consumes, what a local
// adopter stores into ASSAY_VERIFIER_PUBKEY, and what `openssl pkey -pubin`
// validates.
func DeriveRSAPublicKeyPEM(privPEM []byte) ([]byte, error) {
	key, err := ParseRSAPrivateKeyPEM(privPEM)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal public key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), nil
}
