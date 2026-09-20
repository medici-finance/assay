package deskkit

import (
	"encoding/json"
	"os"
	"testing"
)

// vitals_test.go — MergeResourceVitals must preserve every field this binary does not own
// (mirroring ackbeacon_test.go's AppendAck coverage), and the three-state discipline (null
// vs "could-not-check" vs a measured value, INCLUDING a measured zero) must survive the
// round trip through JSON (example-stream/13).

// TestMergeResourceVitalsPreservesAcks — writing the resource block on a beacon that
// already carries a receipt (deskack's AppendAck) must not drop that receipt (Verify row 2).
func TestMergeResourceVitalsPreservesAcks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := AppendAck("s-vitals-acks", AckRecord{Role: "worker-desk", Restatement: "a receipt"}); err != nil {
		t.Fatalf("AppendAck: %v", err)
	}

	rv := ResourceVitals{
		Tokens:         MeasuredInt(1234),
		ContextPctUsed: MeasuredFloat(60),
		Model:          MeasuredString("example-model"),
	}
	if _, err := MergeResourceVitals("s-vitals-acks", rv); err != nil {
		t.Fatalf("MergeResourceVitals: %v", err)
	}

	path, _ := AckBeaconPath("s-vitals-acks")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read beacon: %v", err)
	}
	var obj struct {
		Acks     []AckRecord     `json:"acks"`
		Resource json.RawMessage `json:"resource"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("parse beacon: %v", err)
	}
	if len(obj.Acks) != 1 || obj.Acks[0].Restatement != "a receipt" {
		t.Fatalf("MergeResourceVitals dropped the acks: %s", data)
	}
	if len(obj.Resource) == 0 {
		t.Fatalf("resource block missing from the beacon: %s", data)
	}
}

// resourceRaw is the raw-field shape both round-trip tests below decode into, so each
// field's JSON representation (null / "could-not-check" / a number) is inspected directly
// rather than through a typed value that would itself hide a fabricated zero.
type resourceRaw struct {
	Tokens            json.RawMessage `json:"tokens"`
	ContextPctUsed    json.RawMessage `json:"context_pct_used"`
	SessionAgeSeconds json.RawMessage `json:"session_age_seconds"`
	SubagentsSpawned  json.RawMessage `json:"subagents_spawned"`
	Model             json.RawMessage `json:"model"`
}

func readResource(t *testing.T, session string) resourceRaw {
	t.Helper()
	path, _ := AckBeaconPath(session)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read beacon: %v", err)
	}
	var obj struct {
		Resource resourceRaw `json:"resource"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("parse beacon: %v", err)
	}
	return obj.Resource
}

// TestUnsetVitalIsNullNotZero — a field the caller never set (the Go zero value, nil)
// must serialise as JSON null, never as a number a consumer could mistake for a measured
// zero (Verify row 3; the pre-mortem this closes: "an unset vital renders 0 and a
// consumer reads it as plenty of headroom").
func TestUnsetVitalIsNullNotZero(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	rv := ResourceVitals{Tokens: MeasuredInt(500)} // everything else left unset
	if _, err := MergeResourceVitals("s-vitals-unset", rv); err != nil {
		t.Fatalf("MergeResourceVitals: %v", err)
	}

	res := readResource(t, "s-vitals-unset")
	if string(res.Tokens) != "500" {
		t.Errorf("tokens = %s, want the measured value 500", res.Tokens)
	}
	for name, raw := range map[string]json.RawMessage{
		"context_pct_used":    res.ContextPctUsed,
		"session_age_seconds": res.SessionAgeSeconds,
		"subagents_spawned":   res.SubagentsSpawned,
		"model":               res.Model,
	} {
		if string(raw) != "null" {
			t.Errorf("%s = %s, want null (an unset field must never be a fabricated zero)", name, raw)
		}
	}
}

// TestMeasuredZeroSubagentsRoundTrips — a REAL reading of zero (no subagents spawned) must
// round-trip as the JSON number 0, distinct from both null (unset) and "could-not-check"
// (unreadable) — Verify row 4.
func TestMeasuredZeroSubagentsRoundTrips(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	rv := ResourceVitals{SubagentsSpawned: MeasuredInt(0)}
	if _, err := MergeResourceVitals("s-vitals-zero", rv); err != nil {
		t.Fatalf("MergeResourceVitals: %v", err)
	}

	res := readResource(t, "s-vitals-zero")
	if string(res.SubagentsSpawned) != "0" {
		t.Fatalf("subagents_spawned = %s, want the measured value 0 (a real reading, not \"unknown\")", res.SubagentsSpawned)
	}
}

// TestCouldNotCheckVitalIsDistinctFromNullAndZero pins the third state explicitly: a field
// whose source read failed serialises as the STRING "could-not-check", never null and
// never a number.
func TestCouldNotCheckVitalIsDistinctFromNullAndZero(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	rv := ResourceVitals{ContextPctUsed: CouldNotCheckVital()}
	if _, err := MergeResourceVitals("s-vitals-cnc", rv); err != nil {
		t.Fatalf("MergeResourceVitals: %v", err)
	}

	res := readResource(t, "s-vitals-cnc")
	if string(res.ContextPctUsed) != `"could-not-check"` {
		t.Fatalf("context_pct_used = %s, want the string \"could-not-check\"", res.ContextPctUsed)
	}
}

// TestMergeResourceVitalsMalformedBeaconFailsClosed mirrors AppendAck's own fail-closed
// test — a corrupt beacon is never silently overwritten, which would lose whatever another
// writer (deskroster, deskack) put there.
func TestMergeResourceVitalsMalformedBeaconFailsClosed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path, _ := AckBeaconPath("s-vitals-corrupt")
	if err := os.MkdirAll(dirOf(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := MergeResourceVitals("s-vitals-corrupt", ResourceVitals{Tokens: MeasuredInt(1)}); err == nil {
		t.Fatal("MergeResourceVitals silently overwrote a malformed beacon — it must fail closed")
	}
}

// TestValidSessionSegmentRejectsTraversal pins the S-1 boundary check this package exposes
// for deskroster's `set` to call before it joins a session name into a beacon path.
func TestValidSessionSegmentRejectsTraversal(t *testing.T) {
	bad := []string{"", ".", "..", "a/b", "../escape", "a/../b", `a\b`}
	for _, name := range bad {
		if ValidSessionSegment(name) {
			t.Errorf("ValidSessionSegment(%q) = true, want false", name)
		}
	}
	good := []string{"s1", "worker-desk-20260917", "session_with_underscore"}
	for _, name := range good {
		if !ValidSessionSegment(name) {
			t.Errorf("ValidSessionSegment(%q) = false, want true", name)
		}
	}
}
