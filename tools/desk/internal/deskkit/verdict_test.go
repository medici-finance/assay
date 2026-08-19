package deskkit

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"
	"testing"
)

// marshalTestPrivPEM renders an in-memory RSA key as a PKCS#1 private-key PEM,
// the same shape ParseRSAPrivateKeyPEM and DeriveRSAPublicKeyPEM consume.
func marshalTestPrivPEM(t *testing.T, key *rsa.PrivateKey) []byte {
	t.Helper()
	der := x509.MarshalPKCS1PrivateKey(key)
	return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der})
}

func testKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	return k
}

func TestCanonicalizeDeterministicAndIdempotent(t *testing.T) {
	// Same logical value, three different textual encodings: reordered keys,
	// added whitespace, and integer-with-exponent-free form. All must canonicalise
	// to identical bytes.
	inputs := []string{
		`{"b":2,"a":1,"z":{"y":[3,2,1],"x":"hi"}}`,
		"{ \"a\" : 1 ,\n \"z\":{\"x\":\"hi\",\"y\":[3,2,1]},\t\"b\":2 }",
	}
	var canon string
	for i, in := range inputs {
		out, err := CanonicalizeJSON([]byte(in))
		if err != nil {
			t.Fatalf("input %d: %v", i, err)
		}
		if i == 0 {
			canon = string(out)
		} else if string(out) != canon {
			t.Fatalf("input %d canonicalised differently:\n got %s\nwant %s", i, out, canon)
		}
	}
	// Idempotent: canonicalising the canonical form is a no-op.
	again, err := CanonicalizeJSON([]byte(canon))
	if err != nil {
		t.Fatalf("re-canonicalise: %v", err)
	}
	if string(again) != canon {
		t.Fatalf("not idempotent:\n got %s\nwant %s", again, canon)
	}
	// Keys are sorted, no whitespace.
	if canon != `{"a":1,"b":2,"z":{"x":"hi","y":[3,2,1]}}` {
		t.Fatalf("unexpected canonical form: %s", canon)
	}
}

func TestCanonicalizeStringEscaping(t *testing.T) {
	// `<`,`>`,`&` are NOT escaped; control chars and quotes ARE.
	out, err := CanonicalizeJSON([]byte(`{"k":"a<b>&c \"q\" \n\ttab"}`))
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, `a<b>&c`) {
		t.Fatalf("angle/amp were escaped: %s", got)
	}
	if !strings.Contains(got, `\"q\"`) || !strings.Contains(got, `\n\t`) {
		t.Fatalf("quotes/controls not escaped: %s", got)
	}
}

func TestCanonicalizeRejectsTrailingData(t *testing.T) {
	if _, err := CanonicalizeJSON([]byte(`{}{}`)); err == nil {
		t.Fatal("expected trailing-data rejection")
	}
	if _, err := CanonicalizeJSON([]byte(`{`)); err == nil {
		t.Fatal("expected invalid-JSON rejection")
	}
}

func TestSignVerifyRoundtrip(t *testing.T) {
	key := testKey(t)
	pub := &key.PublicKey

	payload := `{"schema":"verdict-v1","repo":"medici-finance/assay","entries":[{"pass":true,"row":1}]}`
	canon, err := CanonicalizeJSON([]byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	sig, err := SignVerdictCanonical(canon, key)
	if err != nil {
		t.Fatal(err)
	}
	body := AssembleVerdictBody(canon, sig)

	state, msg := VerifyVerdictBody(body, pub)
	if state != VerdictVerified {
		t.Fatalf("expected Verified, got %v (%s)", state, msg)
	}
}

// TestVerdictTamper is the negative-path row (Verify #2). A signed body with one
// flipped CONTENT byte must REFUSE — a definite cryptographic negative, not a
// could-not-check. It prints the refusal to stdout so `go test` (without -v)
// still surfaces the word "refused".
func TestVerdictTamper(t *testing.T) {
	key := testKey(t)
	pub := &key.PublicKey

	payload := `{"schema":"verdict-v1","repo":"medici-finance/assay","entries":[{"pass":true,"row":1,"sha":"abcdef0"}]}`
	canon, err := CanonicalizeJSON([]byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	sig, err := SignVerdictCanonical(canon, key)
	if err != nil {
		t.Fatal(err)
	}
	body := AssembleVerdictBody(canon, sig)

	// Baseline: the untampered body verifies.
	if state, msg := VerifyVerdictBody(body, pub); state != VerdictVerified {
		t.Fatalf("baseline should verify, got %v (%s)", state, msg)
	}

	// Flip the PASS verdict true -> false: a logical-content change that keeps the
	// payload valid JSON, so the failure is a SIGNATURE mismatch (Refused), not a
	// parse error (CouldNotCheck).
	tampered := strings.Replace(body, `"pass":true`, `"pass":false`, 1)
	if tampered == body {
		t.Fatal("tamper substitution did not apply")
	}
	state, msg := VerifyVerdictBody(tampered, pub)
	if state != VerdictRefused {
		t.Fatalf("tampered body must be REFUSED, got %v (%s)", state, msg)
	}
	if !strings.Contains(msg, "refused") {
		t.Fatalf("refusal message must contain \"refused\": %s", msg)
	}
	fmt.Printf("TestVerdictTamper: tampered verdict body correctly %s\n", msg)

	// A single flipped byte inside the base64 signature is also refused.
	sigTampered := strings.Replace(body, "sig="+sig, "sig="+flipB64(sig), 1)
	if s2, m2 := VerifyVerdictBody(sigTampered, pub); s2 != VerdictRefused {
		t.Fatalf("flipped signature must be REFUSED, got %v (%s)", s2, m2)
	}
}

func TestVerifyCouldNotCheck(t *testing.T) {
	key := testKey(t)
	pub := &key.PublicKey

	// No payload block / no signature.
	if s, _ := VerifyVerdictBody("just some prose, no block", pub); s != VerdictCouldNotCheck {
		t.Fatalf("missing block should be CouldNotCheck, got %v", s)
	}
	// Payload block present but unparseable JSON, signature present.
	canon, _ := CanonicalizeJSON([]byte(`{"a":1}`))
	sig, _ := SignVerdictCanonical(canon, key)
	body := AssembleVerdictBody([]byte(`{not json`), sig)
	if s, m := VerifyVerdictBody(body, pub); s != VerdictCouldNotCheck {
		t.Fatalf("unparseable payload should be CouldNotCheck, got %v (%s)", s, m)
	}
}

func TestWrongKeyRefused(t *testing.T) {
	signer := testKey(t)
	other := testKey(t)

	canon, _ := CanonicalizeJSON([]byte(`{"a":1}`))
	sig, _ := SignVerdictCanonical(canon, signer)
	body := AssembleVerdictBody(canon, sig)

	if s, m := VerifyVerdictBody(body, &other.PublicKey); s != VerdictRefused {
		t.Fatalf("wrong pubkey should Refuse, got %v (%s)", s, m)
	}
}

func TestReflowToleratedByVerify(t *testing.T) {
	// A payload block whose JSON was reflowed (whitespace + key reorder) after
	// signing still verifies, because verify re-canonicalises. The signature is
	// over LOGICAL content, not the block's exact text.
	key := testKey(t)
	canon, _ := CanonicalizeJSON([]byte(`{"a":1,"b":2}`))
	sig, _ := SignVerdictCanonical(canon, key)

	// Hand-build a body whose fenced payload is the SAME logical value, reflowed.
	body := "```" + verdictFenceTag + "\n{ \"b\": 2, \"a\": 1 }\n```\n\n" +
		"<!-- " + verdictSigMarker + " v1 alg=" + verdictSigAlg + " sig=" + sig + " -->\n"

	if s, m := VerifyVerdictBody(body, &key.PublicKey); s != VerdictVerified {
		t.Fatalf("reflowed-but-equivalent payload should verify, got %v (%s)", s, m)
	}
}

func TestDeriveAndParsePublicKey(t *testing.T) {
	key := testKey(t)
	privPEM := marshalTestPrivPEM(t, key)

	pubPEM, err := DeriveRSAPublicKeyPEM(privPEM)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(pubPEM), "BEGIN PUBLIC KEY") {
		t.Fatalf("expected PKIX PUBLIC KEY PEM, got:\n%s", pubPEM)
	}
	pub, err := ParseRSAPublicKeyPEM(pubPEM)
	if err != nil {
		t.Fatal(err)
	}
	if pub.N.Cmp(key.PublicKey.N) != 0 {
		t.Fatal("derived public key modulus does not match")
	}
}

// TestDecodePubkeyVar covers the ASSAY_VERIFIER_PUBKEY value normalisation: a
// literal PEM string, base64-of-PEM (with and without embedded newlines), and the
// fail-closed cases (empty, non-PEM base64, garbage). Both good forms must parse
// back to the same public key.
func TestDecodePubkeyVar(t *testing.T) {
	key := testKey(t)
	pubPEM, err := DeriveRSAPublicKeyPEM(marshalTestPrivPEM(t, key))
	if err != nil {
		t.Fatal(err)
	}

	// Literal PEM string.
	got, err := DecodePubkeyVar(string(pubPEM))
	if err != nil {
		t.Fatalf("literal PEM: %v", err)
	}
	if pub, err := ParseRSAPublicKeyPEM(got); err != nil || pub.N.Cmp(key.PublicKey.N) != 0 {
		t.Fatalf("literal PEM did not round-trip: %v", err)
	}

	// Compact base64-of-PEM.
	b64 := base64.StdEncoding.EncodeToString(pubPEM)
	got, err = DecodePubkeyVar(b64)
	if err != nil {
		t.Fatalf("base64 PEM: %v", err)
	}
	if pub, err := ParseRSAPublicKeyPEM(got); err != nil || pub.N.Cmp(key.PublicKey.N) != 0 {
		t.Fatalf("base64 PEM did not round-trip: %v", err)
	}

	// base64 wrapped across lines (as a pasted variable might arrive).
	wrapped := b64[:20] + "\n" + b64[20:]
	if _, err := DecodePubkeyVar(wrapped); err != nil {
		t.Fatalf("wrapped base64 should decode: %v", err)
	}

	// Fail-closed: empty, valid base64 that is not a PEM, and outright garbage.
	if _, err := DecodePubkeyVar("   "); err == nil {
		t.Fatal("empty value should error")
	}
	if _, err := DecodePubkeyVar(base64.StdEncoding.EncodeToString([]byte("not a pem"))); err == nil {
		t.Fatal("base64 of non-PEM should error")
	}
	if _, err := DecodePubkeyVar("%%%not base64 not pem%%%"); err == nil {
		t.Fatal("garbage should error")
	}
}

// flipB64 returns sig with one character changed, staying inside the base64
// alphabet so the trailer still parses (the failure is a signature mismatch).
func flipB64(sig string) string {
	if len(sig) == 0 {
		return "A"
	}
	b := []byte(sig)
	// Change a character near the middle, avoiding trailing '=' padding.
	i := len(b) / 2
	if b[i] == 'A' {
		b[i] = 'B'
	} else {
		b[i] = 'A'
	}
	return string(b)
}
