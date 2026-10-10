package deskkit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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

// regression: F2-repo-grammar-drops-record
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
	// Positive controls for the identifier grammars: every shape the dispatcher really writes.
	for _, c := range []struct {
		name string
		mut  func(*DispatchRecord)
	}{
		{"brief v1 id", func(r *DispatchRecord) { r.Brief = strp("example-stream/02") }},
		{"brief v2 id", func(r *DispatchRecord) { r.Brief = strp("cell-a:proj:example-stream:28") }},
		{"aliased item", func(r *DispatchRecord) { r.Item = strp("proj:example-stream/28") }},
		{"pr item key", func(r *DispatchRecord) { r.Item = strp("assay--pr-2427") }},
		{"kit review", func(r *DispatchRecord) { r.Kit = strp("review") }},
		{"kit verifier", func(r *DispatchRecord) { r.Kit = strp("verifier") }},
		{"kit worker-objective", func(r *DispatchRecord) { r.Kit = strp("worker-objective") }},
		{"dotted session", func(r *DispatchRecord) { r.SessionTag = "worker-desk.2:night_1" }},
		{"unknown session", func(r *DispatchRecord) { r.SessionTag = "unknown" }},
		{"dotted repo", func(r *DispatchRecord) { r.Repo = "example.org/my_project-2" }},
		// A session UUID's shape (8-4-4-4-12 hex), built from zeros so no real session id is named.
		{"uuid session", func(r *DispatchRecord) {
			r.SessionTag = strings.Join([]string{strings.Repeat("0", 8), "0000", "4000", "8000", strings.Repeat("0", 12)}, "-")
		}},
		// Every slug a forge can host is a repo the roster can admit, so each is recorded.
		{"repo dot name", func(r *DispatchRecord) { r.Repo = "example-org/.github" }},
		{"repo underscore name", func(r *DispatchRecord) { r.Repo = "example-org/_template" }},
		{"repo dash name", func(r *DispatchRecord) { r.Repo = "example-org/-x" }},
		{"repo underscore owner", func(r *DispatchRecord) { r.Repo = "_owner/x" }},
		{"repo 101-char name", func(r *DispatchRecord) { r.Repo = "example-org/" + strings.Repeat("n", 101) }},
		{"branch 255 chars", func(r *DispatchRecord) { r.Branch = strp("f" + strings.Repeat("b", 254)) }},
	} {
		t.Run(c.name+" accepted", func(t *testing.T) {
			r := validDispatched()
			c.mut(&r)
			if err := ValidateDispatchRecord(r); err != nil {
				t.Fatalf("a record the dispatcher writes was refused (%s): %v", c.name, err)
			}
		})
	}
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
		// No string field is free text: each identifier field is refused prose, a link, a
		// mention, markup and the format characters that reorder or hide what a reader sees.
		{"brief is prose", func(r *DispatchRecord) { r.Brief = strp("vendor-model-9 ran this; see notes @someone") }},
		{"brief is a link", func(r *DispatchRecord) { r.Brief = strp("https://example.invalid/x") }},
		{"brief is markup", func(r *DispatchRecord) { r.Brief = strp("<b>x</b>/1") }},
		{"brief has RLO", func(r *DispatchRecord) { r.Brief = strp("x/‮1") }},
		{"brief has ZWSP", func(r *DispatchRecord) { r.Brief = strp("x​/1") }},
		{"brief has LSEP", func(r *DispatchRecord) { r.Brief = strp("x/1 ") }},
		{"brief v2 short", func(r *DispatchRecord) { r.Brief = strp("assay:x:1") }},
		{"item is prose", func(r *DispatchRecord) { r.Item = strp("any words at all") }},
		{"kit outside set", func(r *DispatchRecord) { r.Kit = strp("auditor") }},
		{"kit is prose", func(r *DispatchRecord) { r.Kit = strp("a worker, probably") }},
		{"kit not canonical", func(r *DispatchRecord) { r.Kit = strp("Worker") }},
		{"branch is prose", func(r *DispatchRecord) { r.Branch = strp("feat/any words") }},
		{"branch has dotdot", func(r *DispatchRecord) { r.Branch = strp("feat/../x") }},
		{"repo is prose", func(r *DispatchRecord) { r.Repo = "any sentence / with a slash" }},
		{"repo two slashes", func(r *DispatchRecord) { r.Repo = "a/b/c" }},
		{"repo empty name", func(r *DispatchRecord) { r.Repo = "example-org/" }},
		{"repo has @", func(r *DispatchRecord) { r.Repo = "example-org/a@b" }},
		{"repo has scheme", func(r *DispatchRecord) { r.Repo = "https://example.org/x" }},
		// The v2 grammar's end anchor: a valid v2 prefix followed by more is refused.
		{"brief v2 five parts", func(r *DispatchRecord) { r.Brief = strp("cell-a:proj:example-stream:28:9") }},
		// 64+64+64+62 characters and three colons: 257 bytes, inside the v2 shape, so only the
		// 256-byte cap refuses it.
		{"brief v2 257 bytes", func(r *DispatchRecord) {
			seg := strings.Repeat("s", 64)
			r.Brief = strp(seg + ":" + seg + ":" + seg + ":" + strings.Repeat("n", 62))
		}},
		{"brief v2 then slash", func(r *DispatchRecord) { r.Brief = strp("cell-a:proj:example-stream:28/x") }},
		{"session is prose", func(r *DispatchRecord) { r.SessionTag = "any words at all, a name included" }},
		{"session has PSEP", func(r *DispatchRecord) { r.SessionTag = "sess A" }},
		{"session empty", func(r *DispatchRecord) { r.SessionTag = "" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := validDispatched()
			c.mut(&r)
			if err := ValidateDispatchRecord(r); err == nil {
				line, _ := json.Marshal(r)
				t.Fatalf("ValidateDispatchRecord accepted an out-of-schema record (%s): %s", c.name, line)
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

// regression: F2-repo-grammar-drops-record (the walk now fails on a field kind it does not know)
// TestDispatchRecordEveryStringFieldHasAGrammar closes the free-text class for fields added
// later: every string field of DispatchRecord, found by reflection rather than by a list a new
// field could be left off, is set in turn to prose and must be refused. A new field missing from
// the validator's field list, or from checkRecordField's grammar switch, fails here.
func TestDispatchRecordEveryStringFieldHasAGrammar(t *testing.T) {
	const prose = "free text, a name and a <b>tag</b>"
	typ := reflect.TypeOf(DispatchRecord{})
	n := 0
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		isStr := f.Type.Kind() == reflect.String
		isPtrStr := f.Type.Kind() == reflect.Pointer && f.Type.Elem().Kind() == reflect.String
		isPtrInt := f.Type.Kind() == reflect.Pointer && f.Type.Elem().Kind() == reflect.Int
		if isPtrInt {
			continue // pr and attempt_local: numbers, bounded by ValidateDispatchRecord
		}
		if !isStr && !isPtrStr {
			// A slice, map, interface or any other kind is seen neither by this walk nor by the
			// validator's field list, so it could carry text unchecked: refuse it here.
			t.Errorf("field %s has kind %s, which neither this walk nor the validator checks", f.Name, f.Type)
			continue
		}
		n++
		t.Run(f.Name, func(t *testing.T) {
			r := validDispatched()
			fv := reflect.ValueOf(&r).Elem().Field(i)
			if isStr {
				fv.SetString(prose)
			} else {
				v := prose
				fv.Set(reflect.ValueOf(&v))
			}
			if err := ValidateDispatchRecord(r); err == nil {
				t.Fatalf("field %s accepted free text: no grammar holds it", f.Name)
			}
		})
	}
	if n == 0 {
		t.Fatal("found no string field in DispatchRecord: the reflection walk is broken")
	}
	if !ValidDispatchRecordField("kit", "worker") || ValidDispatchRecordField("no_such_field", "x") {
		t.Fatal("checkRecordField must accept a known field's value and refuse a field it has no grammar for")
	}
}

// regression: S2-claim-key-comment (advisory 4: the backstop was unpinned)
// TestCheckRecordStringBackstop pins the three generic checks every string field passes before
// its own grammar: the 256-byte cap, valid UTF-8 and no control character. Each field grammar
// refuses these values too, so only a direct call shows the backstop itself holds.
func TestCheckRecordStringBackstop(t *testing.T) {
	for name, v := range map[string]string{
		"257 bytes":    strings.Repeat("k", 257),
		"invalid UTF8": "a\xffb",
		"newline":      "a\nb",
		"escape":       "a\x1b[2Jb",
		"C1 control":   "a\u0085b",
		"DEL":          "a\x7fb",
	} {
		if err := checkRecordString("f", v); err == nil {
			t.Errorf("checkRecordString accepted %s (%q)", name, v)
		}
	}
	if err := checkRecordString("f", strings.Repeat("k", 256)); err != nil {
		t.Errorf("checkRecordString refused 256 bytes, the cap itself: %v", err)
	}
}
