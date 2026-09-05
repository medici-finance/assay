package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// status_test.go — the offline tests behind the brief's Verify rows 1 and 3, plus the guard
// tests that pin the two invariants a console depends on: tokens is could-not-check (never a
// zero a reader could mistake for "free"), and a blind claim renders timers n/a (never 0s) and
// makes the snapshot exit-6-worthy. All run against the committed testdata fixtures, no network.

// statusSchemaPath is the real published schema, four directories up from this package
// (tools/desk/cmd/desksupervise → repo root) at schemas/. The test reads the REAL file so a
// drift between the emitted JSON and the schema the console reads is caught here (Verify row 3).
const statusSchemaPath = "../../../../schemas/desksupervise-status-v1.json"

func mustLoadSnapshot(t *testing.T, claimsFile, obsFile, stopsFile string, stopsOnly bool, now string) (StatusSnapshot, bool) {
	t.Helper()
	claims, err := loadClaimsFixture(claimsFile)
	if err != nil {
		t.Fatalf("loadClaimsFixture(%s): %v", claimsFile, err)
	}
	byKey, err := loadObservationsFixture(obsFile)
	if err != nil {
		t.Fatalf("loadObservationsFixture(%s): %v", obsFile, err)
	}
	stopsByKey := map[string]*StatusStop{}
	if stopsFile != "" {
		stopsByKey, err = loadStopsFixture(stopsFile)
		if err != nil {
			t.Fatalf("loadStopsFixture(%s): %v", stopsFile, err)
		}
	}
	snap, anyBlind, err := buildSnapshot(claims, fixtureStatusObs(byKey), stopsByKey, stopsOnly, loopengine.DefaultLivenessPolicy(), mustParseTS(t, now))
	if err != nil {
		t.Fatalf("buildSnapshot: %v", err)
	}
	return snap, anyBlind
}

// TestStatusSnapshotMixed pins Verify row 2's shape in-process: three claims, none blind, and
// claims[0].tokens is could-not-check.
func TestStatusSnapshotMixed(t *testing.T) {
	snap, anyBlind := mustLoadSnapshot(t, "testdata/mixed.json", "testdata/mixed-obs.json", "", false, "2026-09-02T12:00:00Z")
	if anyBlind {
		t.Fatal("mixed-obs has no could-not-check claim; anyBlind must be false")
	}
	if snap.Schema != "desksupervise-status-v1" {
		t.Fatalf("schema = %q, want desksupervise-status-v1", snap.Schema)
	}
	if len(snap.Claims) != 3 {
		t.Fatalf("len(claims) = %d, want 3", len(snap.Claims))
	}
	if snap.Claims[0].Tokens != "could-not-check" {
		t.Fatalf("claims[0].tokens = %q, want could-not-check", snap.Claims[0].Tokens)
	}
	// Liveness classification is the SAME taxonomy tick runs.
	wantLiveness := map[string]string{
		"example-stream--alive-01": "ALIVE",
		"example-stream--never-01": "NEVER-STARTED",
		"example-stream--dead-01":  "HEARTBEAT-EXPIRED",
	}
	for _, c := range snap.Claims {
		if want := wantLiveness[c.Key]; c.Liveness != want {
			t.Errorf("%s liveness = %q, want %q", c.Key, c.Liveness, want)
		}
	}
}

// TestStatusTokensNeverZero is the guard: EVERY claim's tokens field is could-not-check, never
// a numeric zero. A mutation that filled tokens with a count (the "renders as 0 and someone
// reads it as free" pre-mortem, Verify row 2) trips this. See the PR body's Fail-first section.
func TestStatusTokensNeverZero(t *testing.T) {
	snap, _ := mustLoadSnapshot(t, "testdata/mixed.json", "testdata/mixed-obs.json", "", false, "2026-09-02T12:00:00Z")
	for _, c := range snap.Claims {
		if c.Tokens != "could-not-check" {
			t.Errorf("%s tokens = %q, want could-not-check (never a zero)", c.Key, c.Tokens)
		}
	}
}

// TestStatusSnapshotBlind pins Verify row 4: a could-not-check claim renders liveness
// COULD-NOT-CHECK, all three timers n/a (NEVER 0s), lists a blind source, and makes anyBlind
// true (the caller maps that to exit 6). The "a blind tick prints a clean-looking table with
// exit 0" pre-mortem is what this and the exit-code path close.
func TestStatusSnapshotBlind(t *testing.T) {
	snap, anyBlind := mustLoadSnapshot(t, "testdata/mixed.json", "testdata/blind-obs.json", "", false, "2026-09-02T12:00:00Z")
	if !anyBlind {
		t.Fatal("blind-obs marks example-stream--dead-01 could-not-check; anyBlind must be true")
	}
	if len(snap.Aggregates.BlindSources) == 0 {
		t.Fatal("a blind claim must be listed in aggregates.blind_sources")
	}
	var dead *StatusClaim
	for i := range snap.Claims {
		if snap.Claims[i].Key == "example-stream--dead-01" {
			dead = &snap.Claims[i]
		}
	}
	if dead == nil {
		t.Fatal("example-stream--dead-01 missing from claims")
	}
	if dead.Liveness != "COULD-NOT-CHECK" {
		t.Errorf("blind claim liveness = %q, want COULD-NOT-CHECK", dead.Liveness)
	}
	for name, v := range map[string]string{
		"schedule_to_start": dead.Timers.ScheduleToStartRemaining,
		"heartbeat":         dead.Timers.HeartbeatRemaining,
		"wall_cap":          dead.Timers.WallCapRemaining,
	} {
		if v != "n/a" {
			t.Errorf("blind claim %s timer = %q, want n/a (a blind claim has no computed remaining)", name, v)
		}
	}
	// The rendered table must never show a bare " 0s" remaining anywhere (the blind claim's
	// timers are n/a, and no other claim's remaining lands on exactly zero in this fixture).
	var buf bytes.Buffer
	renderStatusTable(&buf, snap)
	if strings.Contains(buf.String(), " 0s") {
		t.Errorf("table contains a bare \" 0s\" remaining:\n%s", buf.String())
	}
}

// TestStatusStopsFilter pins Verify row 5: --stops renders ONLY claims carrying an armed stop.
// The "only the worker desk reads stops" pre-mortem is addressed by the three skill edits; this
// pins the mechanism they call.
func TestStatusStopsFilter(t *testing.T) {
	snap, _ := mustLoadSnapshot(t, "testdata/mixed.json", "testdata/mixed-obs.json", "testdata/stops.json", true, "2026-09-02T12:00:00Z")
	if len(snap.Claims) != 1 {
		t.Fatalf("--stops rendered %d claims, want exactly 1 (only the stopped one)", len(snap.Claims))
	}
	if snap.Claims[0].Key != "example-stream--dead-01" {
		t.Fatalf("--stops rendered %q, want example-stream--dead-01", snap.Claims[0].Key)
	}
	if snap.Claims[0].Stop == nil {
		t.Fatal("the rendered claim must carry its armed stop")
	}
	var buf bytes.Buffer
	renderStatusTable(&buf, snap)
	for _, absent := range []string{"example-stream--alive-01", "example-stream--never-01"} {
		if strings.Contains(buf.String(), absent) {
			t.Errorf("--stops output must not contain non-stopped claim %q:\n%s", absent, buf.String())
		}
	}
}

// TestStatusJSONValidatesAgainstSchema pins Verify row 3: the emitted JSON validates against the
// real published schema. The "JSON drifts from the schema the console reads" pre-mortem is what
// this closes — editing either side without the other reddens here.
func TestStatusJSONValidatesAgainstSchema(t *testing.T) {
	snap, _ := mustLoadSnapshot(t, "testdata/mixed.json", "testdata/mixed-obs.json", "testdata/stops.json", false, "2026-09-02T12:00:00Z")

	var buf bytes.Buffer
	if err := renderStatusJSON(&buf, snap); err != nil {
		t.Fatalf("renderStatusJSON: %v", err)
	}
	var doc interface{}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("emitted JSON does not parse: %v", err)
	}

	schemaBytes, err := os.ReadFile(statusSchemaPath)
	if err != nil {
		t.Fatalf("cannot read schema %s: %v", statusSchemaPath, err)
	}
	var schema map[string]interface{}
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatalf("schema %s does not parse: %v", statusSchemaPath, err)
	}

	if errs := validateAgainstSchema(schema, doc, "$"); len(errs) > 0 {
		t.Fatalf("emitted JSON does not validate against %s:\n  %s", statusSchemaPath, strings.Join(errs, "\n  "))
	}
}

// TestSnapshotFromSweepAtomicWrite pins Task 3: `run --interval` builds the snapshot from its
// own tick's sweepResults (no re-probe) and writes it atomically. The written document must
// validate against the schema, and no .tmp file may survive a successful write (the rename is
// atomic, and a reader never sees a half-written file).
func TestSnapshotFromSweepAtomicWrite(t *testing.T) {
	now := mustParseTS(t, "2026-09-02T12:00:00Z")
	results := []sweepResult{
		{
			Claim: claimRecord{Key: "s--01", Item: "s/01", Owner: "o", Repo: "medici-finance/assay", Branch: "b", Tier: "cheap", State: "dispatched", DispatchedAt: "2026-09-02T11:58:00Z"},
			Disp:  loopengine.Alive, Last: mustParseTS(t, "2026-09-02T11:59:00Z"), Via: "audit line", Action: "none",
		},
		{
			Claim: claimRecord{Key: "s--02", Item: "s/02", Owner: "o", Repo: "medici-finance/assay", Branch: "b", Tier: "cheap", State: "dispatched", DispatchedAt: "2026-09-02T09:00:00Z"},
			Blind: true, Action: "BLIND",
		},
	}
	snap := snapshotFromSweep(results, loopengine.DefaultLivenessPolicy(), now)
	if len(snap.Claims) != 2 {
		t.Fatalf("len(claims) = %d, want 2", len(snap.Claims))
	}
	if len(snap.Aggregates.BlindSources) != 1 {
		t.Fatalf("blind_sources = %v, want exactly one (s--02)", snap.Aggregates.BlindSources)
	}
	for _, c := range snap.Claims {
		if c.Tokens != "could-not-check" {
			t.Errorf("%s tokens = %q, want could-not-check", c.Key, c.Tokens)
		}
	}

	dir := t.TempDir()
	path := dir + "/supervise/status.json"
	if err := writeStatusJSON(path, snap); err != nil {
		t.Fatalf("writeStatusJSON: %v", err)
	}

	// The written file validates against the schema.
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back status.json: %v", err)
	}
	var doc interface{}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("status.json does not parse: %v", err)
	}
	schemaBytes, _ := os.ReadFile(statusSchemaPath)
	var schema map[string]interface{}
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatalf("schema does not parse: %v", err)
	}
	if errs := validateAgainstSchema(schema, doc, "$"); len(errs) > 0 {
		t.Fatalf("written status.json does not validate:\n  %s", strings.Join(errs, "\n  "))
	}

	// No temp file survived the atomic rename.
	entries, err := os.ReadDir(dir + "/supervise")
	if err != nil {
		t.Fatalf("read supervise dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("a .tmp file survived the atomic write: %s", e.Name())
		}
	}
}

// --- a minimal, self-contained JSON Schema validator ---
//
// The repo vendors no JSON-schema library and the offline envelope forbids adding one, so this
// validates the SUBSET of draft-2020-12 the desksupervise-status-v1 schema uses: type (string
// or array of strings), const, enum, required, properties, items, and additionalProperties
// (bool). It is deliberately strict on additionalProperties:false — an emitted field the schema
// does not name is a violation, which is exactly the drift Verify row 3 exists to catch.

func validateAgainstSchema(schema map[string]interface{}, value interface{}, path string) []string {
	var errs []string

	if c, ok := schema["const"]; ok {
		if !jsonEqual(c, value) {
			errs = append(errs, fmt.Sprintf("%s: const mismatch (want %v, got %v)", path, c, value))
		}
	}
	if e, ok := schema["enum"].([]interface{}); ok {
		matched := false
		for _, cand := range e {
			if jsonEqual(cand, value) {
				matched = true
				break
			}
		}
		if !matched {
			errs = append(errs, fmt.Sprintf("%s: %v is not one of the enum values %v", path, value, e))
		}
	}

	if ty, ok := schema["type"]; ok && !typeMatches(ty, value) {
		errs = append(errs, fmt.Sprintf("%s: type mismatch (want %v, got %T)", path, ty, value))
		return errs // don't recurse into a value of the wrong type
	}

	switch v := value.(type) {
	case map[string]interface{}:
		props, _ := schema["properties"].(map[string]interface{})
		if req, ok := schema["required"].([]interface{}); ok {
			for _, r := range req {
				key, _ := r.(string)
				if _, present := v[key]; !present {
					errs = append(errs, fmt.Sprintf("%s: missing required property %q", path, key))
				}
			}
		}
		allowAdditional := true
		if ap, ok := schema["additionalProperties"].(bool); ok {
			allowAdditional = ap
		}
		for key, sub := range v {
			propSchema, named := props[key]
			if !named {
				if !allowAdditional {
					errs = append(errs, fmt.Sprintf("%s: additional property %q not allowed", path, key))
				}
				continue
			}
			ps, _ := propSchema.(map[string]interface{})
			errs = append(errs, validateAgainstSchema(ps, sub, path+"."+key)...)
		}
	case []interface{}:
		if items, ok := schema["items"].(map[string]interface{}); ok {
			for i, elem := range v {
				errs = append(errs, validateAgainstSchema(items, elem, fmt.Sprintf("%s[%d]", path, i))...)
			}
		}
	}
	return errs
}

// typeMatches reports whether value satisfies a JSON Schema "type" (a string, or an array of
// acceptable type names — the null-or-object union the stop field uses).
func typeMatches(ty interface{}, value interface{}) bool {
	switch t := ty.(type) {
	case string:
		return oneTypeMatches(t, value)
	case []interface{}:
		for _, cand := range t {
			if s, ok := cand.(string); ok && oneTypeMatches(s, value) {
				return true
			}
		}
		return false
	default:
		return true // unknown type declaration — do not fail on it
	}
}

func oneTypeMatches(name string, value interface{}) bool {
	switch name {
	case "object":
		_, ok := value.(map[string]interface{})
		return ok
	case "array":
		_, ok := value.([]interface{})
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "integer":
		f, ok := value.(float64)
		return ok && f == float64(int64(f))
	case "number":
		_, ok := value.(float64)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "null":
		return value == nil
	default:
		return true
	}
}

func jsonEqual(a, b interface{}) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}
