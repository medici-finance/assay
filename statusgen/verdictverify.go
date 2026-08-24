package main

// verdictverify.go — the VERIFY-side verdict-payload trust primitives for the
// R-6 verify-transcription lane (verdict-lane/03).
//
// This is a DOCUMENTED DUPLICATE of the verify half of
// `../tools/desk/internal/deskkit/verdict.go`, following the same
// documented-duplicate pattern this tree already uses for the trust roster
// (rosterconfig.go duplicates deskkit's roster loader; trustgate.go duplicates
// its blessing rule). statusgen is its own Go module and deliberately does NOT
// import deskkit, so the transcriber cannot call VerifyVerdictBody directly. The
// canonicalisation, the RS256 verify, the fenced-block extraction and the
// pubkey-variable decoding are reproduced here byte-for-byte so the transcriber
// verifies exactly what `deskverdict sign` produced.
//
// KEEP IN SYNC with deskkit/verdict.go. A change to the canonical form, the
// signature scheme, the fence tag, the signature-trailer regex, or the pubkey
// variable name must be made in BOTH copies, or a body signed by one will not
// verify under the other. The two are bound by intent, not by a shared import.
//
// Sign primitives are included too, but ONLY the test fixtures use them: the
// production transcriber never signs (it holds no private key — the verifier PEM
// never enters this process). They live here so the tests can build a genuine
// RS256-signed fixture and prove the bad-signature clause refuses independently,
// with no committed key material and no `deskverdict` binary dependency.

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

// vvSchemaVersion is the payload schema this build speaks. A payload naming a
// version this binary does not recognise is refused, never guessed.
const vvSchemaVersion = "verdict-v1"

const (
	vvFenceTag  = "verdict-payload"
	vvSigMarker = "deskverdict-signature"
	vvSigAlg    = "RS256"
	vvPubkeyVar = "ASSAY_VERIFIER_PUBKEY"
)

// vvSigRE extracts the base64 signature from the trailer comment. Standard
// base64 alphabet — the signature is a raw RSA blob, never a JWT.
var vvSigRE = regexp.MustCompile(vvSigMarker + `\b[^>]*\bsig=([A-Za-z0-9+/=]+)`)

// vvVerifyState is the three-state result of verifying an issue body. Refused is
// a definite cryptographic negative; CouldNotCheck is a structural surprise. A
// trust gate treats everything that is not Verified as "do not trust".
type vvVerifyState int

const (
	vvVerified vvVerifyState = iota
	vvRefused
	vvCouldNotCheck
)

// vvCanonicalize returns the byte-deterministic canonical JSON encoding of raw:
// object keys sorted by UTF-8 code unit, no insignificant whitespace, the fixed
// string-escape rule (no HTML escaping of <, >, &), integers shortest-decimal.
// The transform is idempotent, which is what lets verify re-canonicalise a block
// that already holds the canonical form.
func vvCanonicalize(raw []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("payload is not valid JSON: %w", err)
	}
	if dec.More() {
		return nil, errors.New("payload has trailing data after the JSON value")
	}
	var buf bytes.Buffer
	if err := vvWriteCanonical(&buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func vvWriteCanonical(buf *bytes.Buffer, v interface{}) error {
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
		buf.WriteString(vvCanonicalNumber(t))
	case string:
		vvWriteCanonicalString(buf, t)
	case []interface{}:
		buf.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := vvWriteCanonical(buf, e); err != nil {
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
			vvWriteCanonicalString(buf, k)
			buf.WriteByte(':')
			if err := vvWriteCanonical(buf, t[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	default:
		return fmt.Errorf("unexpected JSON value of type %T during canonicalisation", v)
	}
	return nil
}

func vvCanonicalNumber(n json.Number) string {
	s := n.String()
	if vvIsIntegerLiteral(s) {
		if i, err := n.Int64(); err == nil {
			return strconv.FormatInt(i, 10)
		}
	}
	return s
}

func vvIsIntegerLiteral(s string) bool {
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

func vvWriteCanonicalString(buf *bytes.Buffer, s string) {
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

// vvExtractPayload returns the inner text of the FIRST ```verdict-payload fenced
// block — the same "first block" rule deskkit's ParseVerdictBody uses, so the
// transcriber parses exactly the bytes the signature was verified over.
func vvExtractPayload(body string) (string, error) {
	lines := strings.Split(body, "\n")
	open := -1
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "```") && strings.TrimSpace(strings.TrimLeft(t, "`")) == vvFenceTag {
			open = i
			break
		}
	}
	if open < 0 {
		return "", errors.New("no ```" + vvFenceTag + " block found in body")
	}
	for j := open + 1; j < len(lines); j++ {
		t := strings.TrimSpace(lines[j])
		if strings.HasPrefix(t, "```") && strings.TrimLeft(t, "`") == "" {
			return strings.Join(lines[open+1:j], "\n"), nil
		}
	}
	return "", errors.New("unterminated ```" + vvFenceTag + " block in body")
}

// vvParseBody extracts the payload JSON and the base64 signature from a signed
// issue body. A missing block or trailer is a structural error the caller maps
// to CouldNotCheck.
func vvParseBody(body string) (payload []byte, sigB64 string, err error) {
	payloadStr, perr := vvExtractPayload(body)
	if perr != nil {
		return nil, "", perr
	}
	m := vvSigRE.FindStringSubmatch(body)
	if m == nil {
		return nil, "", errors.New("no verdict signature trailer found in body")
	}
	return []byte(payloadStr), m[1], nil
}

// vvVerifyCanonical verifies sigB64 over canonical using the PUBLIC key.
func vvVerifyCanonical(canonical []byte, sigB64 string, pub *rsa.PublicKey) error {
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

// vvVerifyBody is the full body-level verify: extract, re-canonicalise, and check
// the signature with the PUBLIC key. Three-state; the Refused message contains
// "refused", the CouldNotCheck message "could not check".
func vvVerifyBody(body string, pub *rsa.PublicKey) (vvVerifyState, string) {
	rawPayload, sigB64, err := vvParseBody(body)
	if err != nil {
		return vvCouldNotCheck, "could not check: " + err.Error()
	}
	canonical, cerr := vvCanonicalize(rawPayload)
	if cerr != nil {
		return vvCouldNotCheck, "could not check: " + cerr.Error()
	}
	if verr := vvVerifyCanonical(canonical, sigB64, pub); verr != nil {
		return vvRefused, "refused: " + verr.Error()
	}
	return vvVerified, "verified: signature matches the canonical verdict payload"
}

// vvParsePublicKeyPEM parses an RSA public key from a PKIX ("PUBLIC KEY") PEM,
// tolerating the bare PKCS#1 form too.
func vvParsePublicKeyPEM(pemBytes []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("no PEM block found")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
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

// vvDecodePubkeyVar normalises the value of ASSAY_VERIFIER_PUBKEY into PKIX PEM
// bytes, accepting either a literal PEM or standard base64-of-PEM. An empty
// value, or base64 that does not decode to a PEM, is an error — never a pass.
func vvDecodePubkeyVar(val string) ([]byte, error) {
	s := strings.TrimSpace(val)
	if s == "" {
		return nil, errors.New("empty verifier pubkey value")
	}
	if strings.HasPrefix(s, "-----BEGIN") {
		return []byte(val), nil
	}
	compact := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, s)
	der, err := base64.StdEncoding.DecodeString(compact)
	if err != nil {
		return nil, fmt.Errorf("%s is neither a PEM string nor valid base64-of-PEM: %w", vvPubkeyVar, err)
	}
	if !bytes.Contains(der, []byte("-----BEGIN")) {
		return nil, fmt.Errorf("%s base64-decoded but is not a PEM (no BEGIN marker)", vvPubkeyVar)
	}
	return der, nil
}

// ---------------------------------------------------------------------------
// Sign side — TEST FIXTURE USE ONLY (the transcriber never signs).
// ---------------------------------------------------------------------------

// vvSignCanonical signs the SHA-256 digest of canonical with key (RS256) and
// returns the standard-base64 signature. canonical must be vvCanonicalize output.
func vvSignCanonical(canonical []byte, key *rsa.PrivateKey) (string, error) {
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

// vvAssembleBody produces the issue-body block a signed payload lands as: the
// canonical payload in a fenced code block, then the signature as an HTML-comment
// trailer.
func vvAssembleBody(canonical []byte, sigB64 string) string {
	var b strings.Builder
	b.WriteString("```")
	b.WriteString(vvFenceTag)
	b.WriteByte('\n')
	b.Write(canonical)
	b.WriteByte('\n')
	b.WriteString("```\n\n")
	b.WriteString("<!-- ")
	b.WriteString(vvSigMarker)
	b.WriteString(" v1 alg=")
	b.WriteString(vvSigAlg)
	b.WriteString(" sig=")
	b.WriteString(sigB64)
	b.WriteString(" -->\n")
	return b.String()
}

// vvDerivePublicKeyPEM derives the PKIX public-key PEM from an RSA private key —
// used by tests to populate ASSAY_VERIFIER_PUBKEY from a generated keypair.
func vvDerivePublicKeyPEM(key *rsa.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal public key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), nil
}
