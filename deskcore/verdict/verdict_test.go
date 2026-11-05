package verdict

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

func TestCanonicalBytesAndRoles(t *testing.T) {
	raw := []byte(`{"z":2,"a":"<&>"}`)
	canonical, e := CanonicalizeJSON(raw)
	if e != nil || string(canonical) != `{"a":"<&>","z":2}` {
		t.Fatal(string(canonical), e)
	}
	again, e := CanonicalizeJSON(canonical)
	if e != nil || string(again) != string(canonical) {
		t.Fatal("non-idempotent canonicalization")
	}
	if _, e := CanonicalizeJSON([]byte(`{} {}`)); e == nil {
		t.Fatal("trailing JSON admitted")
	}
	key, e := rsa.GenerateKey(rand.Reader, 1024)
	if e != nil {
		t.Fatal(e)
	}
	h := sha256.Sum256(canonical)
	sig, e := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, h[:])
	if e != nil {
		t.Fatal(e)
	}
	body := fmt.Sprintf("```verdict-payload\n%s\n```\n<!-- deskverdict-signature v1 alg=RS256 role=verifier sig=%s -->\n", canonical, base64.StdEncoding.EncodeToString(sig))
	if state, why := VerifyBodyForRole(body, &key.PublicKey, "verifier"); state != VerdictVerified {
		t.Fatal(state, why)
	}
	if state, _ := VerifyBodyForRole(body, &key.PublicKey, "issue-loop"); state != VerdictRefused {
		t.Fatal("wrong role admitted")
	}
	if state, _ := VerifyBodyForRole(strings.Replace(body, `"z":2`, `"z":3`, 1), &key.PublicKey, "verifier"); state != VerdictRefused {
		t.Fatal("changed payload admitted")
	}
	if state, _ := VerifyBodyForRole(body, &key.PublicKey, "unknown"); state != VerdictCouldNotCheck {
		t.Fatal("unknown profile admitted")
	}
}
