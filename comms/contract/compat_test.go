package contract

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// compat_test.go — TestContractCompatibility pins the extracted contract to the
// legacy cellmsg-v1 behaviour with golden fixtures that were authored against the
// pre-extraction implementation (tools/desk/internal/comms). The same fixture
// files are driven through the legacy-compatible Assay entrypoint by
// TestLegacyEntrypointMatchesContractFixtures in tools/desk, so both entrypoints
// answer to one set of bytes.
//
// The contract makes NO policy claim: the "structural" column below is what the
// contract alone decides. A message the Assay ACL forbids is structurally fine
// here — that refusal belongs to the policy adapter.

type fixtureExpect struct {
	Structural string `json:"structural"`
	Assay      string `json:"assay"`
	Lane       string `json:"lane"`
}

func loadExpect(t *testing.T) map[string]fixtureExpect {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "legacy", "expect.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]fixtureExpect
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if len(m) == 0 {
		t.Fatal("empty expectation table")
	}
	return m
}

func codeName(err error) string {
	if err == nil {
		return "ok"
	}
	return string(CodeOf(err))
}

func TestContractCompatibility(t *testing.T) {
	expect := loadExpect(t)
	names := make([]string, 0, len(expect))
	for n := range expect {
		names = append(names, n)
	}
	sort.Strings(names)

	files, _ := filepath.Glob(filepath.Join("testdata", "legacy", "*.json"))
	if len(files)-1 != len(expect) { // expect.json itself is not a fixture
		t.Fatalf("fixture/expectation count drift: %d fixture files, %d expectations", len(files)-1, len(expect))
	}

	for _, n := range names {
		t.Run("envelope/"+n, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("testdata", "legacy", n+".json"))
			if err != nil {
				t.Fatal(err)
			}
			e, err := ParseEnvelope(raw)
			if got := codeName(err); got != expect[n].Structural {
				t.Fatalf("structural parse = %s, want %s (err %v)", got, expect[n].Structural, err)
			}
			if err != nil && e != nil {
				t.Fatalf("a refusal returned a partially-populated envelope")
			}
			if err == nil {
				// Typed mapping round-trips every field the wire carries.
				back := MessageFromEnvelope(e).Envelope()
				a, _ := json.Marshal(e)
				b, _ := json.Marshal(back)
				if !bytes.Equal(a, b) {
					t.Fatalf("typed message round-trip changed the envelope:\n  %s\n  %s", a, b)
				}
			}
		})
	}

	t.Run("oversize", func(t *testing.T) {
		raw := bytes.Repeat([]byte(" "), MaxEnvelopeBytes+1)
		if _, err := ParseEnvelope(raw); !errors.Is(err, ErrOversize) {
			t.Fatalf("want ErrOversize, got %v", err)
		}
	})

	t.Run("vocabulary is consulted in legacy order", func(t *testing.T) {
		raw, _ := os.ReadFile(filepath.Join("testdata", "legacy", "unknown-verb.json"))
		if _, err := ParseEnvelopeWith(raw, denyAll{}); !errors.Is(err, ErrUnknownVerb) {
			t.Fatalf("want ErrUnknownVerb from a vocabulary that knows nothing, got %v", err)
		}
		raw, _ = os.ReadFile(filepath.Join("testdata", "legacy", "unknown-class.json"))
		if _, err := ParseEnvelopeWith(raw, verbsOnly{}); !errors.Is(err, ErrUnknownClass) {
			t.Fatalf("want ErrUnknownClass, got %v", err)
		}
		// Structural refusals win over the vocabulary: a bad-schema message with
		// an unknown verb is bad-schema, as it always was.
		raw, _ = os.ReadFile(filepath.Join("testdata", "legacy", "refused-bad-schema.json"))
		if _, err := ParseEnvelopeWith(raw, denyAll{}); !errors.Is(err, ErrBadSchema) {
			t.Fatalf("want ErrBadSchema before the vocabulary, got %v", err)
		}
	})
}

type denyAll struct{}

func (denyAll) KnownVerb(string) bool  { return false }
func (denyAll) KnownClass(string) bool { return false }

type verbsOnly struct{}

func (verbsOnly) KnownVerb(string) bool  { return true }
func (verbsOnly) KnownClass(string) bool { return false }

type assertionFixture struct {
	Cell        string `json:"cell"`
	Role        string `json:"role"`
	MsgID       string `json:"msgId"`
	Nonce       string `json:"nonce"`
	IssuedUnix  int64  `json:"issuedUnix"`
	ExpiresUnix int64  `json:"expiresUnix"`
	Segments    []struct {
		Name string `json:"name"`
		Hex  string `json:"hex"`
	} `json:"canonicalSegments"`
	SigHex       string `json:"sigHex"`
	PublicKeyHex string `json:"publicKeyHex"`
}

// TestContractCompatibilityAssertionBytes pins the signed byte layout: the
// canonical bytes, and the Ed25519 signature over them (deterministic), minted
// by the pre-extraction implementation. A drift here is a wire break.
func TestContractCompatibilityAssertionBytes(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "assertion.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f assertionFixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	issued := time.Unix(f.IssuedUnix, 0).UTC()

	a, err := Mint(f.Cell, f.Role, f.MsgID, f.Nonce, issued, 2*time.Minute, Ed25519Signer{Key: priv})
	if err != nil {
		t.Fatal(err)
	}
	var want []byte
	for _, seg := range f.Segments {
		b, err := hex.DecodeString(seg.Hex)
		if err != nil {
			t.Fatalf("segment %s: %v", seg.Name, err)
		}
		want = append(want, b...)
	}
	if len(f.Segments) == 0 {
		t.Fatal("assertion fixture carries no canonical segments")
	}
	if got := CanonicalAssertionBytes(a); !bytes.Equal(got, want) {
		t.Fatalf("canonical assertion bytes drifted:\n got %x\nwant %x", got, want)
	}
	if got := hex.EncodeToString(a.Sig); got != f.SigHex {
		t.Fatalf("assertion signature drifted:\n got %s\nwant %s", got, f.SigHex)
	}
	if a.ExpiresAt.Unix() != f.ExpiresUnix {
		t.Fatalf("expiry = %d, want %d", a.ExpiresAt.Unix(), f.ExpiresUnix)
	}
	pub, _ := hex.DecodeString(f.PublicKeyHex)
	trust := Ed25519TrustStore{f.Cell: ed25519.PublicKey(pub)}

	// The signature carried by the valid fixture envelope verifies end to end.
	env, err := os.ReadFile(filepath.Join("testdata", "legacy", "valid-within-notify.json"))
	if err != nil {
		t.Fatal(err)
	}
	e, err := ParseEnvelope(env)
	if err != nil {
		t.Fatal(err)
	}
	guard := NewMemReplayGuardWithClock(func() time.Time { return issued })
	if err := VerifyEnvelope(e, issued.Add(10*time.Second), trust, DefaultSkew, guard); err != nil {
		t.Fatalf("fixture envelope should verify: %v", err)
	}
	if err := VerifyEnvelope(e, issued.Add(11*time.Second), trust, DefaultSkew, guard); !errors.Is(err, ErrReplay) {
		t.Fatalf("second use must be ErrReplay, got %v", err)
	}
	// A tampered id breaks the binding.
	e2, _ := ParseEnvelope(env)
	e2.ID = "msg-0002"
	if err := VerifyEnvelope(e2, issued.Add(10*time.Second), trust, DefaultSkew, NewMemReplayGuard()); !errors.Is(err, ErrIdentityMismatch) {
		t.Fatalf("want ErrIdentityMismatch, got %v", err)
	}
}

func TestErrorCodesAreStable(t *testing.T) {
	want := map[error]string{
		ErrOversize: "oversize", ErrMalformed: "malformed", ErrBadSchema: "bad_schema",
		ErrAbsentTriple: "absent_triple", ErrUnknownVerb: "unknown_verb", ErrUnknownClass: "unknown_class",
		ErrUnknownCell: "unknown_cell", ErrBadSignature: "bad_signature", ErrExpired: "expired",
		ErrNotYetValid: "not_yet_valid", ErrReplay: "replay", ErrIdentityMismatch: "identity_mismatch",
		ErrMintInput: "mint_input",
	}
	for err, code := range want {
		if got := string(CodeOf(err)); got != code {
			t.Errorf("CodeOf(%v) = %q, want %q", err, got, code)
		}
	}
	if CodeOf(nil) != CodeNone || CodeOf(errors.New("x")) != CodeUnclassified {
		t.Fatal("nil / foreign errors misclassified")
	}
	if e := AsError(ErrOversize); !errors.Is(e, ErrOversize) || e.Code != CodeOversize {
		t.Fatalf("AsError lost its sentinel: %+v", e)
	}
}

func TestCapabilitiesAreDeclaredNotImplemented(t *testing.T) {
	caps := Capabilities()
	if len(caps) != 2 {
		t.Fatalf("capabilities = %v", caps)
	}
	for _, c := range caps {
		if Implemented(c) {
			t.Errorf("%s reports implemented — cellmsg-v1 does not provide it", c)
		}
		if !strings.Contains(string(c), "/v1") {
			t.Errorf("%s is not a versioned capability name", c)
		}
	}
}

func TestReceiptLegacyMapping(t *testing.T) {
	for _, l := range []LegacyReceipt{{ID: "m", Accepted: true}, {ID: "m", Accepted: false, Detail: "refused"}} {
		r := ReceiptFromLegacy(l, time.Time{})
		if r.Legacy() != l {
			t.Errorf("round trip %+v -> %+v", l, r.Legacy())
		}
		if r.Accepted() != l.Accepted {
			t.Errorf("Accepted mismatch for %+v", l)
		}
	}
}
