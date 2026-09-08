package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// deskack_test.go — the receipt verb: the fixed line, the twelve-word cap, the role from
// $DESK_LOOP, and the beacon append (Verify row 5a).

// withEnv gives a test a clean HOME (fresh beacon store + roster), a session, and a desk
// loop, and returns nothing — run() takes its I/O as buffers.
func withEnv(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("DESK_LOOP", "worker-desk")
	t.Setenv("DESK_SESSION", "sess-1")
	t.Setenv("CLAUDE_SESSION_ID", "")
}

func runAck(args []string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := run(args, &out, &errb)
	return rc, out.String(), errb.String()
}

func loadBeacon(t *testing.T, session string) map[string]json.RawMessage {
	t.Helper()
	path, err := deskkit.AckBeaconPath(session)
	if err != nil {
		t.Fatalf("AckBeaconPath: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read beacon %s: %v", path, err)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("parse beacon: %v", err)
	}
	return obj
}

// TestAckPrintsFixedLine — the receipt is `ack <role>@<repo-short>: <restatement>`, with
// role from $DESK_LOOP and repo-short resolved through the roster alias.
func TestAckPrintsFixedLine(t *testing.T) {
	withEnv(t)
	rc, out, errb := runAck([]string{"--repo", "example-reconciler", "re-run the verify row on merged main"})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0; stderr=%s", rc, errb)
	}
	want := "ack worker-desk@recon: re-run the verify row on merged main"
	if strings.TrimSpace(out) != want {
		t.Fatalf("receipt line = %q, want %q", strings.TrimSpace(out), want)
	}
}

// TestAckRoleFromDeskLoop — the role in the line is exactly $DESK_LOOP.
func TestAckRoleFromDeskLoop(t *testing.T) {
	withEnv(t)
	t.Setenv("DESK_LOOP", "verify-desk")
	rc, out, _ := runAck([]string{"drain the awaiting queue now"})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if !strings.HasPrefix(strings.TrimSpace(out), "ack verify-desk:") {
		t.Fatalf("role must come from $DESK_LOOP; got %q", strings.TrimSpace(out))
	}
}

// TestAckNoRepoOmitsAtSegment — with no --repo the line is `ack <role>: <restatement>`,
// not a placeholder.
func TestAckNoRepoOmitsAtSegment(t *testing.T) {
	withEnv(t)
	rc, out, _ := runAck([]string{"pick the next batch of briefs"})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0", rc)
	}
	got := strings.TrimSpace(out)
	if strings.Contains(got, "@") {
		t.Fatalf("no --repo must omit the @<repo-short> segment; got %q", got)
	}
	if got != "ack worker-desk: pick the next batch of briefs" {
		t.Fatalf("line = %q", got)
	}
}

// TestAckRefusesOverTwelveWords — a restatement over twelve words is refused (exit 5) and
// writes NO beacon record (a refused receipt is not a receipt).
func TestAckRefusesOverTwelveWords(t *testing.T) {
	withEnv(t)
	thirteen := "one two three four five six seven eight nine ten eleven twelve thirteen"
	rc, _, errb := runAck([]string{thirteen})
	if rc != deskkit.ExitRefused {
		t.Fatalf("13-word restatement rc = %d, want 5; stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "12") {
		t.Errorf("the refusal must name the twelve-word cap; stderr=%s", errb)
	}
	// No beacon must have been written.
	if path, _ := deskkit.AckBeaconPath("sess-1"); path != "" {
		if _, err := os.Stat(path); err == nil {
			t.Errorf("a refused over-length receipt wrote a beacon at %s — it must not", path)
		}
	}
}

// TestAckTwelveWordsExactlyOK — exactly twelve words is at the boundary and allowed.
func TestAckTwelveWordsExactlyOK(t *testing.T) {
	withEnv(t)
	twelve := "one two three four five six seven eight nine ten eleven twelve"
	if rc, _, errb := runAck([]string{twelve}); rc != deskkit.ExitOK {
		t.Fatalf("exactly-12-word restatement rc = %d, want 0; stderr=%s", rc, errb)
	}
}

// TestAckAppendsBeacon — a successful ack appends a {ts, role, repo, restatement} record
// to this session's roster beacon, and a second ack appends rather than replaces.
func TestAckAppendsBeacon(t *testing.T) {
	withEnv(t)
	if rc, _, errb := runAck([]string{"--repo", "example-reconciler", "first receipt here"}); rc != deskkit.ExitOK {
		t.Fatalf("first ack rc = %d; stderr=%s", rc, errb)
	}
	if rc, _, errb := runAck([]string{"second receipt here now"}); rc != deskkit.ExitOK {
		t.Fatalf("second ack rc = %d; stderr=%s", rc, errb)
	}

	obj := loadBeacon(t, "sess-1")
	raw, ok := obj["acks"]
	if !ok {
		t.Fatalf("beacon carries no acks array: %v", obj)
	}
	var acks []deskkit.AckRecord
	if err := json.Unmarshal(raw, &acks); err != nil {
		t.Fatalf("parse acks: %v", err)
	}
	if len(acks) != 2 {
		t.Fatalf("acks = %d, want 2 (append, not replace)", len(acks))
	}
	first := acks[0]
	if first.Role != "worker-desk" || first.Repo != "recon" || first.Restatement != "first receipt here" || first.TS == "" {
		t.Fatalf("first ack record wrong: %+v", first)
	}
	if acks[1].Restatement != "second receipt here now" {
		t.Fatalf("second ack record wrong: %+v", acks[1])
	}
	// The session field must be present so deskroster reads the file as a well-formed
	// beacon.
	if _, ok := obj["session"]; !ok {
		t.Errorf("beacon deskack created is missing the session field: %v", obj)
	}
}

// TestAckRequiresDeskLoop — $DESK_LOOP unset is a refusal, not a blank role.
func TestAckRequiresDeskLoop(t *testing.T) {
	withEnv(t)
	t.Setenv("DESK_LOOP", "")
	if rc, _, errb := runAck([]string{"a message with no loop"}); rc != deskkit.ExitRefused {
		t.Fatalf("unset DESK_LOOP rc = %d, want 5; stderr=%s", rc, errb)
	}
}

// TestAckEmptyRestatementRefuses — an empty restatement is a refusal (a receipt with
// nothing to confirm is not a receipt).
func TestAckEmptyRestatementRefuses(t *testing.T) {
	withEnv(t)
	if rc, _, _ := runAck([]string{}); rc != deskkit.ExitRefused {
		t.Fatalf("empty restatement rc = %d, want 5", rc)
	}
}

// TestAckKillSwitchDisabled — a disabled desk-tools env halts the receipt (exit 3): a
// halted desk does not acknowledge.
func TestAckKillSwitchDisabled(t *testing.T) {
	withEnv(t)
	t.Setenv("DESK_TOOLS_DISABLED", "1")
	if rc, _, _ := runAck([]string{"should not ack"}); rc != deskkit.ExitDisabled {
		t.Fatalf("kill switch rc = %d, want 3", rc)
	}
}
