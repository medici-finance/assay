package main

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
)

func TestGeneratedCorrelationID(t *testing.T) {
	t.Setenv(envCell, "cell-a")
	t.Setenv(envDesk, "worker-desk")
	d, err := buildDeps(strings.NewReader(""), &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	id := d.newID()
	if len(id) != 36 || id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' {
		t.Fatalf("not a hyphenated UUID: %q", id)
	}
	raw, err := hex.DecodeString(strings.ReplaceAll(id, "-", ""))
	if err != nil || len(raw) != 16 || raw[6]>>4 != 4 || raw[8]>>6 != 2 {
		t.Fatalf("not UUID v4: %q", id)
	}
	payload := "Reply for correlation " + id
	sender, gw, pub, _ := testDeps(t, payload)
	sender.newID = d.newID
	if _, err := cmdSend(sender, []string{"--to", "intake-desk", "--verb", "notify"}); err != nil {
		t.Fatalf("quoting a generated ID was refused: %v", err)
	}
	if len(gw.submitted) != 1 {
		t.Fatalf("want one submission, got %d", len(gw.submitted))
	}
	env, err := comms.ParseEnvelope(gw.submitted[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := comms.VerifyEnvelope(env, sender.now(), comms.Ed25519TrustStore{"cell-a": pub}, time.Minute, comms.NewMemReplayGuard()); err != nil {
		t.Fatalf("generated ID did not authenticate: %v", err)
	}
	if !strings.Contains(string(env.Payload), id) {
		t.Fatal("correlation was not preserved")
	}
	// Nonces are authentication material, not public correlation identifiers.
	nonce := d.newNonce()
	if len(nonce) != 32 || strings.Contains(nonce, "-") {
		t.Fatalf("nonce format changed: %q", nonce)
	}
}

func TestCorrelationLabelDoesNotExemptSecret(t *testing.T) {
	for _, payload := range []string{
		`{"correlation_id":"` + strings.Repeat("a1", 16) + `"}`,
		`{"correlation_id":"ghp_` + strings.Repeat("x", 36) + `"}`,
	} {
		d, gw, _, _ := testDeps(t, payload)
		_, err := cmdSend(d, []string{"--to", "intake-desk", "--verb", "notify"})
		if err == nil || !strings.Contains(err.Error(), "bodycheck") {
			t.Fatalf("want content refusal: %v", err)
		}
		if len(gw.submitted) != 0 {
			t.Fatal("refused content reached gateway")
		}
	}
	// Legacy IDs are opaque mailbox keys, even when they are not suitable prose.
	id := strings.Repeat("a1", 16)
	d, gw, _, _ := testDeps(t, "")
	if _, err := cmdAck(d, []string{id}); err != nil {
		t.Fatalf("legacy ack refused: %v", err)
	}
	if len(gw.acked) != 1 || gw.acked[0] != id {
		t.Fatal("legacy mailbox ID changed")
	}
}
