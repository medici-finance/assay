package comms

// policyadapter_test.go — the Assay policy adapter over the public contract.
// The contract (comms/contract) decides STRUCTURE; this package decides POLICY.
// These tests hold the seam: a message the contract accepts structurally must be
// refused here when the lane ACL forbids it, and the legacy entrypoint must
// answer every shared fixture exactly as the extracted contract does.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/medici-finance/assay/comms/contract"
)

// fixtureDir is the contract module's shared fixture set — one set of bytes for
// both entrypoints.
var fixtureDir = filepath.Join("..", "..", "..", "..", "comms", "contract", "testdata", "legacy")

type fixtureExpect struct {
	Structural string `json:"structural"`
	Assay      string `json:"assay"`
	Lane       string `json:"lane"`
}

func loadFixtures(t *testing.T) map[string]fixtureExpect {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(fixtureDir, "expect.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]fixtureExpect
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if len(m) == 0 {
		t.Fatal("empty fixture expectation table")
	}
	return m
}

func code(err error) string {
	if err == nil {
		return "ok"
	}
	return string(contract.CodeOf(err))
}

// regression: F-comms-policy-adapter
//
// TestPolicyAdapterRefusesForbiddenMessage — a structurally valid cellmsg-v1
// message whose lane the compiled ACL does not permit is ACCEPTED by the contract
// parser (which makes no policy claim) and REFUSED by the Assay policy adapter.
// Removing the policy call from Admit makes this test fail: the adapter would
// return the envelope the ACL forbids.
func TestPolicyAdapterRefusesForbiddenMessage(t *testing.T) {
	acl := Compiled()
	for name, exp := range loadFixtures(t) {
		if exp.Lane != "forbid" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(fixtureDir, name+".json"))
			if err != nil {
				t.Fatal(err)
			}
			// The contract alone accepts it: this is NOT a structural defect.
			if _, err := contract.ParseEnvelope(raw); err != nil {
				t.Fatalf("contract must accept %s structurally, got %v", name, err)
			}
			// The Assay vocabulary parse accepts it too (verb and class are known)...
			if _, err := ParseEnvelope(raw); err != nil {
				t.Fatalf("vocabulary parse must accept %s, got %v", name, err)
			}
			// ...and the policy adapter is what refuses it.
			e, err := Admit(raw, &acl)
			if !errors.Is(err, ErrLaneForbidden) {
				t.Fatalf("Admit(%s) = (%v, %v), want ErrLaneForbidden", name, e, err)
			}
			if e != nil {
				t.Fatalf("Admit returned an envelope alongside a refusal")
			}
		})
	}
}

// TestPolicyAdapterAdmitsPermittedMessage — the adapter is not a blanket refusal:
// every fixture marked allow is admitted, so the refusal test above bites the
// policy call and not a broken adapter.
func TestPolicyAdapterAdmitsPermittedMessage(t *testing.T) {
	acl := Compiled()
	seen := 0
	for name, exp := range loadFixtures(t) {
		if exp.Lane != "allow" {
			continue
		}
		seen++
		raw, err := os.ReadFile(filepath.Join(fixtureDir, name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		e, err := Admit(raw, &acl)
		if err != nil || e == nil {
			t.Fatalf("Admit(%s) = (%v, %v), want admitted", name, e, err)
		}
	}
	if seen == 0 {
		t.Fatal("no allow fixtures — the admit check saw nothing")
	}
}

// TestAdmitNilACLRefuses — an absent ACL is the empty matrix: absent is refused.
func TestAdmitNilACLRefuses(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(fixtureDir, "valid-within-notify.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Admit(raw, nil); !errors.Is(err, ErrLaneForbidden) {
		t.Fatalf("Admit with a nil ACL = %v, want ErrLaneForbidden", err)
	}
}

// TestLegacyEntrypointMatchesContractFixtures — the old-compatible entrypoint
// (this package's ParseEnvelope / ACL.Allow) and the extracted contract answer
// every shared fixture identically, and the Assay vocabulary column of the
// fixture table is what the legacy parser actually returns.
func TestLegacyEntrypointMatchesContractFixtures(t *testing.T) {
	fixtures := loadFixtures(t)
	names := make([]string, 0, len(fixtures))
	for n := range fixtures {
		names = append(names, n)
	}
	sort.Strings(names)
	acl := Compiled()
	for _, n := range names {
		exp := fixtures[n]
		t.Run(n, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(fixtureDir, n+".json"))
			if err != nil {
				t.Fatal(err)
			}
			// extracted contract, structural only
			_, cerr := contract.ParseEnvelope(raw)
			if got := code(cerr); got != exp.Structural {
				t.Errorf("contract structural = %s, want %s", got, exp.Structural)
			}
			// legacy entrypoint, Assay vocabulary applied
			le, lerr := ParseEnvelope(raw)
			if got := code(lerr); got != exp.Assay {
				t.Errorf("legacy ParseEnvelope = %s, want %s (err %v)", got, exp.Assay, lerr)
			}
			// the same parse via the contract with the Assay vocabulary: one parser
			ce, cwerr := contract.ParseEnvelopeWith(raw, Vocabulary())
			if code(cwerr) != code(lerr) {
				t.Errorf("contract+vocabulary = %s, legacy = %s", code(cwerr), code(lerr))
			}
			if lerr == nil {
				a, _ := json.Marshal(le)
				b, _ := json.Marshal(ce)
				if string(a) != string(b) {
					t.Errorf("entrypoints parsed different envelopes:\n %s\n %s", a, b)
				}
				allowed := acl.AllowEnvelope(le)
				if want := exp.Lane == "allow"; allowed != want {
					t.Errorf("AllowEnvelope = %v, want %v", allowed, want)
				}
				if direct := acl.Allow(le.From.Cell, le.From.Role, le.Verb, le.To.Cell, le.To.Role); direct != allowed {
					t.Errorf("AllowEnvelope and Allow disagree")
				}
			}
		})
	}
}

// TestAdapterAliasesAreTheContractTypes — the legacy names are aliases, not
// copies: one type, one implementation.
func TestAdapterAliasesAreTheContractTypes(t *testing.T) {
	var _ contract.Envelope = Envelope{}
	var _ contract.SenderID = SenderID{}
	var _ contract.Lane = Lane{}
	var _ contract.Assertion = Assertion{}
	if !errors.Is(ErrUnknownVerb, contract.ErrUnknownVerb) || ErrReplay != contract.ErrReplay {
		t.Fatal("legacy sentinels are not the contract's")
	}
	if Schema != contract.Schema || MaxEnvelopeBytes != contract.MaxEnvelopeBytes {
		t.Fatal("legacy constants drifted from the contract")
	}
}
