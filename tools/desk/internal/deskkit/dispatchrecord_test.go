package deskkit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func strp(s string) *string { return &s }
func intp(n int) *int       { return &n }

// validDispatched is a well-formed `dispatched` record every refusal subtest mutates one field of.
func validDispatched() DispatchRecord {
	return DispatchRecord{
		Schema: DispatchRecordSchema, Event: DispatchEventDispatched, TS: "2026-10-06T14:15:02Z",
		DispatchRef: strp("assay--x--1@20261006T141502Z.3fa9c01b7d2e"), ClaimKey: "assay--x--1",
		Repo: "example-org/project", Item: strp("x/1"), Brief: strp("assay:assay:x:1"), Kit: strp("worker"),
		Branch: strp("feat/x-1"), PR: nil, SessionTag: "sess-A", Tier: strp("strong"),
		BriefExec: strp("strong"), BriefEffort: strp("M"), ModelStamp: strp(ModelStampPending),
		AttemptLocal: intp(1),
	}
}

func validReleased() DispatchRecord {
	return DispatchRecord{
		Schema: DispatchRecordSchema, Event: DispatchEventReleased, TS: "2026-10-06T15:00:00Z",
		DispatchRef: strp("assay--x--1@20261006T141502Z.3fa9c01b7d2e"), ClaimKey: "assay--x--1",
		Repo: "example-org/project", SessionTag: "sess-A",
	}
}

// TestDispatchRecord_Refusals is Verify row 2: every out-of-schema value is refused, one field
// at a time, and a valid record of each event is accepted (the positive controls — a validator
// that refused everything would otherwise pass every refusal subtest).
func TestDispatchRecord_Refusals(t *testing.T) {
	t.Run("valid dispatched accepted", func(t *testing.T) {
		if err := ValidateDispatchRecord(validDispatched()); err != nil {
			t.Fatalf("a valid dispatched record was refused: %v", err)
		}
	})
	t.Run("valid released accepted", func(t *testing.T) {
		if err := ValidateDispatchRecord(validReleased()); err != nil {
			t.Fatalf("a valid released record was refused: %v", err)
		}
	})
	t.Run("released null ref accepted", func(t *testing.T) {
		r := validReleased()
		r.DispatchRef = nil
		if err := ValidateDispatchRecord(r); err != nil {
			t.Fatalf("a released record with a null dispatch_ref was refused: %v", err)
		}
	})

	cases := []struct {
		name string
		mut  func(*DispatchRecord)
	}{
		{"tier is a model name", func(r *DispatchRecord) { r.Tier = strp("opus-4.8") }},
		{"brief_exec_tier fast", func(r *DispatchRecord) { r.BriefExec = strp("fast") }},
		{"ref of another claim key", func(r *DispatchRecord) { r.DispatchRef = strp("other--x--1@20261006T141502Z.3fa9c01b7d2e") }},
		{"ref without @", func(r *DispatchRecord) { r.DispatchRef = strp("assay--x--1") }},
		{"ref with no nonce", func(r *DispatchRecord) { r.DispatchRef = strp("assay--x--1@20261006T141502Z") }},
		{"nonce too short", func(r *DispatchRecord) { r.DispatchRef = strp("assay--x--1@20261006T141502Z.3fa9c01b7d2") }},
		{"nonce too long", func(r *DispatchRecord) { r.DispatchRef = strp("assay--x--1@20261006T141502Z.3fa9c01b7d2e0") }},
		{"nonce uppercase hex", func(r *DispatchRecord) { r.DispatchRef = strp("assay--x--1@20261006T141502Z.3FA9C01B7D2E") }},
		{"nonce non-hex char", func(r *DispatchRecord) { r.DispatchRef = strp("assay--x--1@20261006T141502Z.3fa9c01b7d2g") }},
		{"effort XL", func(r *DispatchRecord) { r.BriefEffort = strp("XL") }},
		{"field holds a newline", func(r *DispatchRecord) { r.Branch = strp("feat/x\ninjected") }},
		{"unknown event", func(r *DispatchRecord) { r.Event = "launched" }},
		{"unknown schema", func(r *DispatchRecord) { r.Schema = "dispatch-record-v0" }},
		{"field over 256 bytes", func(r *DispatchRecord) { r.Kit = strp(strings.Repeat("k", 257)) }},
		{"model_stamp unknown", func(r *DispatchRecord) { r.ModelStamp = strp("maybe") }},
		{"ts not RFC3339", func(r *DispatchRecord) { r.TS = "yesterday" }},
		{"claim_key carries @", func(r *DispatchRecord) { r.ClaimKey = "a@b--1"; r.DispatchRef = nil }},
		{"no attempt_local", func(r *DispatchRecord) { r.AttemptLocal = nil }},
		{"released with a tier", func(r *DispatchRecord) { *r = validReleased(); r.Tier = strp("any") }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := validDispatched()
			c.mut(&r)
			if err := ValidateDispatchRecord(r); err == nil {
				t.Fatalf("ValidateDispatchRecord accepted an out-of-schema record (%s): %+v", c.name, r)
			}
		})
	}
}

// failReader fails every read — the secure source being unavailable.
type failReader struct{}

func (failReader) Read([]byte) (int, error) { return 0, errors.New("entropy unavailable") }

func TestMintDispatchRefShape(t *testing.T) {
	at := time.Date(2026, 10, 6, 14, 15, 2, 0, time.UTC)
	ref, err := MintDispatchRef("assay--x--1", at, bytes.NewReader([]byte{0x3f, 0xa9, 0xc0, 0x1b, 0x7d, 0x2e}))
	if err != nil {
		t.Fatal(err)
	}
	if ref != "assay--x--1@20261006T141502Z.3fa9c01b7d2e" {
		t.Fatalf("minted %q", ref)
	}
	if _, err := MintDispatchRef("assay--x--1", at, bytes.NewReader([]byte{1, 2, 3})); err == nil {
		t.Error("a short entropy read minted a ref; it must be an error, never a weaker nonce")
	}
	if _, err := MintDispatchRef("assay--x--1", at, failReader{}); err == nil {
		t.Error("a failing entropy source minted a ref")
	}
	if _, err := MintDispatchRef(strings.Repeat("k", 227), at, bytes.NewReader(make([]byte, 6))); err == nil {
		t.Error("a claim key over 226 bytes minted a ref longer than the 256-byte cap")
	}
	long, err := MintDispatchRef(strings.Repeat("k", 226), at, bytes.NewReader(make([]byte, 6)))
	if err != nil || len(long) != 256 {
		t.Errorf("the longest accepted key gave len %d (err %v), want exactly 256", len(long), err)
	}
}

// intOrNull renders a nullable int for a failure message: the value, or "null".
func intOrNull(p *int) string {
	if p == nil {
		return "null"
	}
	return fmt.Sprint(*p)
}

// TestAppendDispatchRecordStore: the writer appends 0600 lines, counts attempt_local per claim
// key from THIS store, and never writes a record that fails validation.
func TestAppendDispatchRecordStore(t *testing.T) {
	dir := setup(t)
	for i := 0; i < 2; i++ {
		r := validDispatched()
		r.AttemptLocal = nil
		if err := AppendDispatchRecord(&r); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
		if r.AttemptLocal == nil || *r.AttemptLocal != i+1 {
			t.Fatalf("append %d: attempt_local = %s, want %d", i, intOrNull(r.AttemptLocal), i+1)
		}
	}
	other := validDispatched()
	other.ClaimKey, other.DispatchRef = "assay--y--2", nil
	if err := AppendDispatchRecord(&other); err != nil || other.AttemptLocal == nil || *other.AttemptLocal != 1 {
		t.Fatalf("another key's attempt_local = %s (err %v), want 1", intOrNull(other.AttemptLocal), err)
	}
	bad := validDispatched()
	bad.Tier = strp("opus-4.8")
	if err := AppendDispatchRecord(&bad); err == nil {
		t.Fatal("an invalid record was appended")
	}
	path := filepath.Join(dir, DispatchRecordsFile)
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Errorf("record store mode %v, want 0600", st.Mode().Perm())
	}
	raw, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 3 {
		t.Fatalf("store holds %d lines, want 3:\n%s", len(lines), raw)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &m); err != nil {
		t.Fatal(err)
	}
	if v, ok := m["pr"]; !ok || v != nil {
		t.Errorf("a null pr must serialize as JSON null, got %v (present=%v)", v, ok)
	}
}
