package main

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// crossCellFixture is like testFixture but signs the-desk@cell-a messages
// addressed cross-cell to the-desk@cell-b.
func crossCellFixture(t *testing.T) (*testFixture, func(id, verb string) []byte) {
	t.Helper()
	f := newFixture(t)
	mk := func(id, verb string) []byte {
		we := wireEnvelope{
			Schema: comms.Schema, ID: id, Cell: "cell-a",
			From: comms.SenderID{Cell: "cell-a", Role: "the-desk"},
			To:   comms.Lane{Cell: "cell-b", Role: "the-desk"},
			Verb: verb, Class: "routine", Sent: f.now,
		}
		a, err := comms.Mint(we.From.Cell, we.From.Role, id, id+"-nonce", f.now, 0, comms.Ed25519Signer{Key: f.priv})
		if err != nil {
			t.Fatalf("Mint: %v", err)
		}
		we.Sig = a
		raw, _ := json.Marshal(we)
		return raw
	}
	return f, mk
}

// --- Verify row 11: CrossCellVerbSet -----------------------------------------

func TestCrossCellVerbSet(t *testing.T) {
	acl := comms.Compiled()
	if len(acl.CrossVerbs) != 4 {
		t.Fatalf("compiled cross-cell verb set has length %d, want exactly 4: %v", len(acl.CrossVerbs), acl.CrossVerbs)
	}

	f, mk := crossCellFixture(t)
	for _, verb := range []string{"status", "metrics", "help-offered", "focus-on"} {
		verb := verb
		t.Run(verb, func(t *testing.T) {
			// Fresh claims dir per verb: each is a distinct message id, so a
			// shared dir would work too, but isolating keeps failures legible.
			f.deps.ClaimConfig.ClaimsDir = filepath.Join(t.TempDir(), "claims")
			raw := mk("xc-"+verb, verb)
			env, err := PreCheck(PreCheckInput{PeerAuthenticated: true, Raw: raw, Now: f.now}, f.deps)
			if err != nil {
				t.Fatalf("verb %q on the-desk->the-desk should be ACCEPTED, got %v", verb, err)
			}
			if env.Verb != verb {
				t.Fatalf("accepted envelope verb = %q, want %q", env.Verb, verb)
			}
		})
	}
}

// --- Verify row 12: CrossCellVerbRefused ------------------------------------

func TestCrossCellVerbRefused(t *testing.T) {
	f, _ := crossCellFixture(t)

	// A near-miss verb ("status-report") and a reserved (human-gate) verb are
	// both refused, and neither is confused with the DIFFERENT pair-set
	// refusal (ErrCrossCellPair) — checkLane only reaches ErrCrossCellVerb once
	// the pair itself (the-desk<->the-desk) is already legal.
	//
	// Both candidate verbs must clear ParseEnvelope's own KnownVerb() gate to
	// even reach the ACL stage, so this test exercises PreCheck at the
	// envelope-shape level directly rather than via ParseEnvelope (which would
	// itself refuse an unknown verb earlier, with ErrEnvelopeParse — a REAL and
	// earlier refusal, but not the one this row pins).
	for _, verb := range []string{"status-report", "reserved-approve-fake"} {
		verb := verb
		t.Run(verb, func(t *testing.T) {
			f.deps.ClaimConfig.ClaimsDir = filepath.Join(t.TempDir(), "claims")
			env := &comms.Envelope{
				Schema: comms.Schema, ID: "xc-bad-" + verb, Cell: "cell-a",
				From: comms.SenderID{Cell: "cell-a", Role: "the-desk"},
				To:   comms.Lane{Cell: "cell-b", Role: "the-desk"},
				Verb: verb,
			}
			err := checkLane(env, f.deps.ACL)
			if err == nil {
				t.Fatalf("verb %q must be refused at the cross-cell ACL stage", verb)
			}
			if !errors.Is(err, ErrCrossCellVerb) {
				t.Fatalf("want ErrCrossCellVerb, got %v", err)
			}
			if errors.Is(err, ErrCrossCellPair) {
				t.Fatalf("an out-of-verb refusal must NOT also read as ErrCrossCellPair — they are distinct refusal codes")
			}
		})
	}

	// Sanity: the same message with an OUT-OF-PAIR role is refused with the
	// DIFFERENT ErrCrossCellPair code, proving the two really are distinct.
	t.Run("out-of-pair is a different code", func(t *testing.T) {
		env := &comms.Envelope{
			Schema: comms.Schema, ID: "xc-bad-pair", Cell: "cell-a",
			From: comms.SenderID{Cell: "cell-a", Role: "worker-desk"},
			To:   comms.Lane{Cell: "cell-b", Role: "the-desk"},
			Verb: "status",
		}
		err := checkLane(env, f.deps.ACL)
		if !errors.Is(err, ErrCrossCellPair) {
			t.Fatalf("want ErrCrossCellPair, got %v", err)
		}
		if errors.Is(err, ErrCrossCellVerb) {
			t.Fatalf("an out-of-pair refusal must NOT also read as ErrCrossCellVerb")
		}
	})

	// Reserved-verb protection: deskkit.ReservedMember already refuses a
	// human-gate verb at ACL LOAD time (laneacl.go), so the compiled ACL this
	// gateway consults can never even contain one — confirm that invariant
	// here rather than re-deriving deskkit's deny-list.
	t.Run("reserved verbs cannot appear in the compiled ACL at all", func(t *testing.T) {
		for _, v := range f.deps.ACL.CrossVerbs {
			if _, bad := deskkit.ReservedMember(v); bad {
				t.Fatalf("compiled cross-cell verb %q names a human-gate action — laneacl.go's load-time refusal has regressed", v)
			}
		}
	})
}
