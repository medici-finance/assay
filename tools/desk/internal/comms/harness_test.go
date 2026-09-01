package comms

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// harness_test.go — shared test helpers for the comms package. Every _test.go in
// this package is in package comms, so these are visible to all of them.

func isErr(err, target error) bool { return errors.Is(err, target) }

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshalling test fixture: %v", err)
	}
	return b
}

// sampleEnvelopeMap builds a well-formed cellmsg-v1 envelope as a map (so a test
// can mutate one field into an invalid shape and re-marshal). The signature is
// left absent: ParseEnvelope does not check it, and the tests that DO exercise
// verification build a signed envelope with signedEnvelope below.
func sampleEnvelopeMap(mut func(map[string]any)) map[string]any {
	m := map[string]any{
		"schema": Schema,
		"id":     "msg-1",
		"cell":   "cell-a",
		"from":   map[string]any{"cell": "cell-a", "role": "worker-desk"},
		"to":     map[string]any{"cell": "cell-a", "role": "pr-review-desk"},
		"verb":   "handoff",
		"class":  "routine",
		"sent":   "2026-09-01T00:00:00Z",
	}
	if mut != nil {
		mut(m)
	}
	return m
}

// newKeypair returns a fresh Ed25519 public key and a Signer over its private
// half. The key material never leaves the test.
func newKeypair(t *testing.T) (ed25519.PublicKey, Signer) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating ed25519 keypair: %v", err)
	}
	return pub, Ed25519Signer{Key: priv}
}

// signedEnvelope mints a valid assertion for cell/role/id and returns a parsed
// Envelope carrying it, plus a trust store that recognises the cell. now sets the
// assertion's issue time.
func signedEnvelope(t *testing.T, cell, role, id string, now time.Time) (*Envelope, TrustStore) {
	t.Helper()
	pub, signer := newKeypair(t)
	a, err := Mint(cell, role, id, "nonce-"+id, now, time.Minute, signer)
	if err != nil {
		t.Fatalf("minting assertion: %v", err)
	}
	m := sampleEnvelopeMap(func(m map[string]any) {
		m["id"] = id
		m["cell"] = cell
		m["from"] = map[string]any{"cell": cell, "role": role}
	})
	raw := mustJSON(t, m)
	e, err := ParseEnvelope(raw)
	if err != nil {
		t.Fatalf("parsing the sample envelope: %v", err)
	}
	e.Sig = a
	return e, Ed25519TrustStore{cell: pub}
}
