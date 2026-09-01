package comms

import (
	"strings"
	"testing"
	"time"
)

// envelope_test.go — cellmsg-v1 parse-or-refuse, plus the envelope-level
// verify-or-refuse (VerifyEnvelope). The parse refusals assert distinct typed
// errors so a caller can tell "too big" from "unknown verb" from "absent triple".

// TestParseRoundTrip is the positive control: a well-formed envelope parses.
func TestParseRoundTrip(t *testing.T) {
	raw := mustJSON(t, sampleEnvelopeMap(nil))
	e, err := ParseEnvelope(raw)
	if err != nil {
		t.Fatalf("a well-formed envelope did not parse: %v", err)
	}
	if e.Verb != "handoff" || e.From.Role != "worker-desk" || e.To.Role != "pr-review-desk" {
		t.Fatalf("parsed envelope has wrong fields: %+v", e)
	}
}

// TestRefuseAbsentTriple: an envelope missing part of the {from, to, verb} triple
// is refused with ErrAbsentTriple.
func TestRefuseAbsentTriple(t *testing.T) {
	cases := []struct {
		name string
		mut  func(map[string]any)
	}{
		{"no verb", func(m map[string]any) { delete(m, "verb") }},
		{"no from", func(m map[string]any) { delete(m, "from") }},
		{"no to.role", func(m map[string]any) { m["to"] = map[string]any{"cell": "cell-a"} }},
		{"no id", func(m map[string]any) { delete(m, "id") }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw := mustJSON(t, sampleEnvelopeMap(c.mut))
			_, err := ParseEnvelope(raw)
			if !isErr(err, ErrAbsentTriple) {
				t.Fatalf("want ErrAbsentTriple, got %v", err)
			}
		})
	}
}

// TestRefuseBadSchema: a payload declaring an unknown schema is refused.
func TestRefuseBadSchema(t *testing.T) {
	raw := mustJSON(t, sampleEnvelopeMap(func(m map[string]any) { m["schema"] = "cellmsg-v2" }))
	if _, err := ParseEnvelope(raw); !isErr(err, ErrBadSchema) {
		t.Fatalf("want ErrBadSchema, got %v", err)
	}
}

// TestRefuseMalformed: non-JSON and an unknown field are both refused as
// malformed (unknown-field rejection is what stops a silently-added surface).
func TestRefuseMalformed(t *testing.T) {
	if _, err := ParseEnvelope([]byte("{not json")); !isErr(err, ErrMalformed) {
		t.Fatalf("want ErrMalformed for bad JSON, got %v", err)
	}
	raw := mustJSON(t, sampleEnvelopeMap(func(m map[string]any) { m["surprise"] = "field" }))
	if _, err := ParseEnvelope(raw); !isErr(err, ErrMalformed) {
		t.Fatalf("want ErrMalformed for an unknown field, got %v", err)
	}
}

// TestRefuseOversize: a payload past the whole-envelope cap is refused before it
// is unmarshalled; an oversize payload body is refused too.
func TestRefuseOversize(t *testing.T) {
	big := make([]byte, MaxEnvelopeBytes+1)
	for i := range big {
		big[i] = 'x'
	}
	if _, err := ParseEnvelope(big); !isErr(err, ErrOversize) {
		t.Fatalf("want ErrOversize for an over-cap payload, got %v", err)
	}
	raw := mustJSON(t, sampleEnvelopeMap(func(m map[string]any) {
		m["payload"] = strings.Repeat("y", MaxPayloadBytes+10)
	}))
	if _, err := ParseEnvelope(raw); !isErr(err, ErrOversize) {
		t.Fatalf("want ErrOversize for an over-cap payload body, got %v", err)
	}
}

// TestRefuseUnknownClass: a class outside the known set is refused.
func TestRefuseUnknownClass(t *testing.T) {
	raw := mustJSON(t, sampleEnvelopeMap(func(m map[string]any) { m["class"] = "cataclysm" }))
	if _, err := ParseEnvelope(raw); !isErr(err, ErrUnknownClass) {
		t.Fatalf("want ErrUnknownClass, got %v", err)
	}
}

// TestVerifyEnvelopeRoundTrip: a well-formed, correctly-signed envelope passes
// VerifyEnvelope.
func TestVerifyEnvelopeRoundTrip(t *testing.T) {
	e, trust := signedEnvelope(t, "cell-a", "worker-desk", "msg-42", testNow)
	if err := VerifyEnvelope(e, testNow.Add(5*time.Second), trust, DefaultSkew, NewMemReplayGuard()); err != nil {
		t.Fatalf("a correctly-signed envelope did not verify: %v", err)
	}
}

// TestRefuseIdentityMismatch: an envelope whose declared From disagrees with the
// identity its assertion proves is refused — this is the check that stops a valid
// assertion for one message being lifted onto another.
func TestRefuseIdentityMismatch(t *testing.T) {
	e, trust := signedEnvelope(t, "cell-a", "worker-desk", "msg-42", testNow)
	// Same signed assertion, but the envelope now claims a different sender role.
	e.From.Role = "the-desk"
	err := VerifyEnvelope(e, testNow.Add(5*time.Second), trust, DefaultSkew, NewMemReplayGuard())
	if !isErr(err, ErrIdentityMismatch) {
		t.Fatalf("want ErrIdentityMismatch, got %v", err)
	}
}
