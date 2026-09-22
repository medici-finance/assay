package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// vitals_test.go — example-stream/13's `deskroster set` vitals flags: the per-tick
// self-report a desk session makes about ITSELF (tokens/context-pct/session-age-seconds/
// subagents/model), the S-1 session-name shape check that gates every beacon write, and
// the regression that a vitals write must not drop what `set`'s own role/work-entry path
// owns on the same beacon file.

// TestSetRefusesMultiSegmentSessionName pins Verify row 11 / security review finding S-1:
// a --session value that does not resolve to a single path segment is refused (exit 5,
// i.e. a *deskkit.DeskError with Code == deskkit.ExitRefused) and never silently joined
// into the beacon path.
func TestSetRefusesMultiSegmentSessionName(t *testing.T) {
	rosterSetup(t)
	// --session must be exercised directly, not shadowed by an ambient/inherited
	// DESK_SESSION or CLAUDE_SESSION_ID (both take precedence over --session in
	// resolveSession) — clear both exactly as roster_test.go's own
	// TestUnresolvableSessionExit6 does.
	t.Setenv("DESK_SESSION", "")
	t.Setenv("CLAUDE_SESSION_ID", "")

	cases := []string{"../escape", "a/b", "..", "."}
	for _, name := range cases {
		args := []string{"--tokens", "5", "--session", name}
		err := cmdSet(args)
		if err == nil {
			t.Fatalf("cmdSet with --session %q: want a refusal, got nil", name)
		}
		de, ok := err.(*deskkit.DeskError)
		if !ok {
			t.Fatalf("cmdSet with --session %q: want a *deskkit.DeskError, got %T (%v)", name, err, err)
		}
		if de.Code != deskkit.ExitRefused {
			t.Fatalf("cmdSet with --session %q: exit code = %d, want %d (refused)", name, de.Code, deskkit.ExitRefused)
		}
		// The rejected name must never have been joined into a beacon path.
		if name != ".." && name != "." {
			path, _ := deskkit.AckBeaconPath(name)
			if _, statErr := os.Stat(path); statErr == nil {
				t.Fatalf("a beacon was written at %s for the refused session name %q", path, name)
			}
		}
	}
}

// TestSetVitalsWritesResourceBlock — the happy path: measured flags land in the beacon's
// resource block, an omitted flag stays null, and "unknown" becomes could-not-check.
func TestSetVitalsWritesResourceBlock(t *testing.T) {
	rosterSetup(t)
	t.Setenv("DESK_SESSION", "s-vitals-flags")

	if err := cmdSet([]string{
		"--tokens", "4200",
		"--context-pct", "55.5",
		"--subagents", "0",
		"--model", "example-model",
		// --session-age-seconds intentionally omitted — must render null, not 0.
		"--session", "s-vitals-flags",
	}); err != nil {
		t.Fatalf("cmdSet: %v", err)
	}

	path, _ := deskkit.AckBeaconPath("s-vitals-flags")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read beacon: %v", err)
	}
	var obj struct {
		Resource struct {
			Tokens            json.RawMessage `json:"tokens"`
			ContextPctUsed    json.RawMessage `json:"context_pct_used"`
			SessionAgeSeconds json.RawMessage `json:"session_age_seconds"`
			SubagentsSpawned  json.RawMessage `json:"subagents_spawned"`
			Model             json.RawMessage `json:"model"`
		} `json:"resource"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("parse beacon: %v", err)
	}
	if string(obj.Resource.Tokens) != "4200" {
		t.Errorf("tokens = %s, want 4200", obj.Resource.Tokens)
	}
	if string(obj.Resource.ContextPctUsed) != "55.5" {
		t.Errorf("context_pct_used = %s, want 55.5", obj.Resource.ContextPctUsed)
	}
	if string(obj.Resource.SubagentsSpawned) != "0" {
		t.Errorf("subagents_spawned = %s, want the measured value 0", obj.Resource.SubagentsSpawned)
	}
	if string(obj.Resource.Model) != `"example-model"` {
		t.Errorf("model = %s, want \"example-model\"", obj.Resource.Model)
	}
	if string(obj.Resource.SessionAgeSeconds) != "null" {
		t.Errorf("session_age_seconds = %s, want null (flag was omitted)", obj.Resource.SessionAgeSeconds)
	}
}

// TestSetVitalsUnknownSentinelIsCouldNotCheck — passing the literal "unknown" writes the
// string could-not-check, distinct from omitting the flag (null).
func TestSetVitalsUnknownSentinelIsCouldNotCheck(t *testing.T) {
	rosterSetup(t)
	t.Setenv("DESK_SESSION", "s-vitals-unknown")

	if err := cmdSet([]string{"--tokens", "unknown", "--session", "s-vitals-unknown"}); err != nil {
		t.Fatalf("cmdSet: %v", err)
	}
	path, _ := deskkit.AckBeaconPath("s-vitals-unknown")
	data, _ := os.ReadFile(path)
	var obj struct {
		Resource struct {
			Tokens json.RawMessage `json:"tokens"`
		} `json:"resource"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("parse beacon: %v", err)
	}
	if string(obj.Resource.Tokens) != `"could-not-check"` {
		t.Errorf("tokens = %s, want \"could-not-check\"", obj.Resource.Tokens)
	}
}

// TestSetVitalsRejectsUnparseableNumber — a numeric vitals flag that is neither a number
// nor the "unknown" sentinel is refused rather than silently treated as could-not-check
// (a typo must never masquerade as a real blind reading).
func TestSetVitalsRejectsUnparseableNumber(t *testing.T) {
	rosterSetup(t)
	t.Setenv("DESK_SESSION", "s-vitals-bad")

	err := cmdSet([]string{"--tokens", "not-a-number", "--session", "s-vitals-bad"})
	if err == nil {
		t.Fatal("cmdSet with --tokens not-a-number: want a refusal, got nil")
	}
	de, ok := err.(*deskkit.DeskError)
	if !ok || de.Code != deskkit.ExitRefused {
		t.Fatalf("cmdSet with --tokens not-a-number: want a refused *deskkit.DeskError, got %v", err)
	}
}

// TestSetVitalsPreservesRoleAndOpenWork — a vitals-only call after role/work-entry calls
// (and vice versa) must not clobber what the OTHER call wrote; both round through the same
// beacon file via different write paths (typed loadBeacon/saveBeacon vs. the raw
// deskkit.MergeResourceVitals merge).
func TestSetVitalsPreservesRoleAndOpenWork(t *testing.T) {
	rosterSetup(t)
	t.Setenv("DESK_SESSION", "s-vitals-combo")

	if err := cmdSet([]string{"--repo", "tracker", "--pr", "9", "--what", "brief z", "--session", "s-vitals-combo"}); err != nil {
		t.Fatalf("cmdSet (work entry): %v", err)
	}
	if err := cmdSet([]string{"--tokens", "10", "--session", "s-vitals-combo"}); err != nil {
		t.Fatalf("cmdSet (vitals): %v", err)
	}

	path, _ := deskkit.AckBeaconPath("s-vitals-combo")
	data, _ := os.ReadFile(path)
	var obj struct {
		OpenWork []WorkEntry     `json:"open_work"`
		Resource json.RawMessage `json:"resource"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("parse beacon: %v", err)
	}
	if len(obj.OpenWork) != 1 {
		t.Fatalf("the vitals write dropped the open_work entry: %s", data)
	}
	if len(obj.Resource) == 0 {
		t.Fatalf("resource block missing after the vitals write: %s", data)
	}

	// Now the reverse order: a role/work-entry write after vitals must preserve resource
	// (the Beacon struct's Resource RawMessage field is what makes this round-trip).
	if err := cmdSet([]string{"--role", "worker-desk", "--session", "s-vitals-combo"}); err != nil {
		t.Fatalf("cmdSet (role): %v", err)
	}
	data2, _ := os.ReadFile(path)
	var obj2 struct {
		Role     string          `json:"role"`
		Resource json.RawMessage `json:"resource"`
	}
	if err := json.Unmarshal(data2, &obj2); err != nil {
		t.Fatalf("parse beacon: %v", err)
	}
	if obj2.Role != "worker-desk" {
		t.Errorf("role = %q, want worker-desk", obj2.Role)
	}
	if len(obj2.Resource) == 0 {
		t.Fatalf("a role-only set dropped the resource block: %s", data2)
	}
}
