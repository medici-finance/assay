// Package verdict owns canonical signed payload bytes and public-key verification.
// It reads no configuration, credentials, files or network.
package verdict

import (
	"bytes"
	"crypto"
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

const (
	verdictFenceTag      = "verdict-payload"
	verdictSigMarker     = "deskverdict-signature"
	VerdictRoleVerifier  = "verifier"
	VerdictRoleIssueLoop = "issue-loop"
)

var verdictSigRE = regexp.MustCompile(verdictSigMarker + `\b[^>]*\bsig=([A-Za-z0-9+/=]+)`)
var verdictRoleRE = regexp.MustCompile(verdictSigMarker + `\b[^>]*\brole=([a-z-]+)`)

type VerdictVerifyState int

const (
	VerdictVerified VerdictVerifyState = iota
	VerdictRefused
	VerdictCouldNotCheck
)

func ValidVerdictRole(role string) bool {
	return role == VerdictRoleVerifier || role == VerdictRoleIssueLoop
}
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

func canonicalNumber(n json.Number) string {
	s := n.String()
	if isIntegerLiteral(s) {
		if i, err := n.Int64(); err == nil {
			return strconv.FormatInt(i, 10)
		}
	}
	return s
}

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

func DeclaredRole(body string) string {
	if m := verdictRoleRE.FindStringSubmatch(body); m != nil {
		return m[1]
	}
	return VerdictRoleVerifier
}

func VerifyCanonical(canonical []byte, sigB64 string, pub *rsa.PublicKey) error {
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

func ParseBody(body string) (payload []byte, sigB64 string, err error) {
	payloadStr, perr := ExtractFencedPayload(body)
	if perr != nil {
		return nil, "", perr
	}
	m := verdictSigRE.FindStringSubmatch(body)
	if m == nil {
		return nil, "", errors.New("no verdict signature trailer found in body")
	}
	return []byte(payloadStr), m[1], nil
}

func ExtractFencedPayload(body string) (string, error) {
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

func VerifyBodyForRole(body string, pub *rsa.PublicKey, wantRole string) (VerdictVerifyState, string) {
	if !ValidVerdictRole(wantRole) {
		return VerdictCouldNotCheck, fmt.Sprintf("could not check: unrecognized verdict role %q (want %q or %q)", wantRole, VerdictRoleVerifier, VerdictRoleIssueLoop)
	}
	rawPayload, sigB64, err := ParseBody(body)
	if err != nil {
		return VerdictCouldNotCheck, "could not check: " + err.Error()
	}
	if declared := DeclaredRole(body); declared != wantRole {
		return VerdictRefused, fmt.Sprintf("refused: signed block declares role %q, but verify was asked to check role %q", declared, wantRole)
	}
	canonical, cerr := CanonicalizeJSON(rawPayload)
	if cerr != nil {
		return VerdictCouldNotCheck, "could not check: " + cerr.Error()
	}
	if verr := VerifyCanonical(canonical, sigB64, pub); verr != nil {
		return VerdictRefused, "refused: " + verr.Error()
	}
	return VerdictVerified, "verified: signature matches the canonical verdict payload"
}

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
