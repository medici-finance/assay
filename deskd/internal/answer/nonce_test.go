package answer

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/internal/contract"
)

// randBytes returns n bytes from crypto/rand, the source a real client would
// mint a nonce from.
func randBytes(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return b
}

// TestNonceDerivation pins the derivation recorded on nonceRE (the authorization contract):
// the floor is the hex encoding of 64 random bits, the ceiling is the hex
// encoding of 512 bits, and every nonce this repo actually mints falls inside
// the range. A change to either bound must update the derivation with it.
//
// regression: #242
func TestNonceDerivation(t *testing.T) {
	accept := map[string]string{
		"64 random bits, hex (the floor)":        hex.EncodeToString(randBytes(t, 8)),
		"96 random bits, unpadded base64url":     base64.RawURLEncoding.EncodeToString(randBytes(t, 12)),
		"128 random bits, unpadded base64url":    base64.RawURLEncoding.EncodeToString(randBytes(t, 16)),
		"256 random bits, hex":                   hex.EncodeToString(randBytes(t, 32)),
		"512 random bits, hex (the ceiling)":     hex.EncodeToString(randBytes(t, 64)),
		"512 random bits, unpadded base64url":    base64.RawURLEncoding.EncodeToString(randBytes(t, 64)),
		"exactly 16 characters, base64url alpha": "abcdefghij-_0123",
	}
	for name, n := range accept {
		if !nonceRE.MatchString(n) {
			t.Errorf("%s: nonceRE refuses %q (len %d), but the derivation admits it", name, n, len(n))
		}
	}

	refuse := map[string]string{
		"one under the floor (15)":                  strings.Repeat("a", 15),
		"one over the ceiling (129)":                strings.Repeat("a", 129),
		"64 random bits, unpadded base64url (11)":   base64.RawURLEncoding.EncodeToString(randBytes(t, 8)),
		"a counter, not a nonce":                    "n-0001",
		"padded base64 ('=' breaks the audit line)": base64.URLEncoding.EncodeToString(randBytes(t, 16)),
		"a space breaks the audit line":             "abcdefgh ijklmnop",
	}
	for name, n := range refuse {
		if nonceRE.MatchString(n) {
			t.Errorf("%s: nonceRE admits %q (len %d), but the derivation refuses it", name, n, len(n))
		}
	}

	// The minted side: the fixture minter every signed-intent test in this
	// package uses is the only nonce mint the package has. Its output must sit
	// inside the range, and the verifier must accept it end to end.
	v, priv, _, t0 := verifierFixture(t)
	si := signed(priv, contract.ActionAnswer, "operator", "opt-a", "rev-1", t0.Add(time.Second), 5*time.Minute)
	if l := len(si.Nonce); l < 16 || l > 128 || !nonceRE.MatchString(si.Nonce) {
		t.Fatalf("minted nonce %q (len %d) is outside nonceRE's 16..128 range", si.Nonce, l)
	}
	if _, err := v.Accept(si, contract.ActionAnswer, testRef.URL); err != nil {
		t.Fatalf("verifier refused a minted nonce: %v", err)
	}
}
