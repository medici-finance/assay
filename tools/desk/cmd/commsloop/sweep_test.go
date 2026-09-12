package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
	"github.com/medici-finance/assay/tools/desk/internal/commsqueue"
)

// sweep_test.go — the sweep's own negative controls: "a sweep that cannot
// fail is a green lamp". TestSweepCleanRun is
// the positive control (without it, a battery of refusals could all pass
// against a Sweep that flags everything); TestSweepSeededViolation plants
// one fixture per violation class and asserts each is DETECTED;
// TestSweepCorruptJournal proves could-not-check is never clean.

var sweepTestNow = time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)

func newSweepKeypair(t *testing.T) (ed25519.PublicKey, comms.Ed25519Signer) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey: %v", err)
	}
	return pub, comms.Ed25519Signer{Key: priv}
}

// writeJournalLines writes root/journal.log with one line per entry,
// verbatim (each entry already includes any trailing content needed).
func writeJournalLines(t *testing.T, root string, lines ...string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(root, "journal.log"), []byte(content), 0o600); err != nil {
		t.Fatalf("write journal.log: %v", err)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return string(raw)
}

// --- positive control -----------------------------------------------------

// TestSweepCleanRun proves a fully legal picture — a landed within-cell
// message with a validly-signed assertion, plus a spawn that traces to it
// through a legal assign-table row — reports checked-clean with zero
// findings. Without this, every other test in this file could be passing
// against a Sweep that ALWAYS reports checked-failed.
func TestSweepCleanRun(t *testing.T) {
	root := t.TempDir()
	pub, signer := newSweepKeypair(t)
	trust := comms.Ed25519TrustStore{"cell-a": pub}

	assertion, err := comms.Mint("cell-a", "worker-desk", "msg-1", "nonce-1", sweepTestNow, 2*time.Minute, signer)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	landed := SweepRecord{
		Time: sweepTestNow, Kind: SweepKindLanded, ID: "msg-1",
		From: comms.SenderID{Cell: "cell-a", Role: "worker-desk"},
		To:   comms.Lane{Cell: "cell-a", Role: "the-desk"},
		Verb: "notify", Class: "routine",
		Assertion: &assertion,
	}
	spawn := SweepRecord{
		Time: sweepTestNow, Kind: SweepKindSpawn, ID: "spawn-1", Cell: "cell-a",
		Action: "land-report", AssignClass: "routine", AssignRisk: false,
		MsgID: "msg-1", SessionID: "sess-1",
	}
	writeJournalLines(t, root, mustJSON(t, landed), mustJSON(t, spawn))

	report := Sweep(root, "cell-a", time.Time{}, SweepDeps{Trust: trust, Now: sweepTestNow.Add(30 * time.Second)})

	if report.State != SweepCheckedClean {
		t.Fatalf("state = %s, want %s — findings=%v couldNotCheck=%v", report.State, SweepCheckedClean, report.Findings, report.CouldNotCheckReasons)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("findings = %v, want none", report.Findings)
	}
	if len(report.CouldNotCheckReasons) != 0 {
		t.Fatalf("could-not-check reasons = %v, want none", report.CouldNotCheckReasons)
	}
	if report.Checked == 0 {
		t.Fatalf("Checked = 0, want at least the landed record + the spawn reconciliation counted")
	}
}

// --- seeded violations: the sweep's own negative controls -----------------

func TestSweepSeededViolation(t *testing.T) {
	t.Run("out-of-lane landing is detected", func(t *testing.T) {
		root := t.TempDir()
		// "status" is a CROSS-CELL-only verb (comms.Compiled().CrossVerbs);
		// within-cell it is not in WithinVerbs, so this landed message was
		// never legal — planting it directly as a journal entry stands in
		// for "legal under an older matrix, illegal now" without needing a
		// second, narrower ACL fixture.
		landed := SweepRecord{
			Time: sweepTestNow, Kind: SweepKindLanded, ID: "msg-bad-lane",
			From: comms.SenderID{Cell: "cell-a", Role: "worker-desk"},
			To:   comms.Lane{Cell: "cell-a", Role: "the-desk"},
			Verb: "status",
		}
		writeJournalLines(t, root, mustJSON(t, landed))

		report := Sweep(root, "cell-a", time.Time{}, SweepDeps{Now: sweepTestNow})

		if report.State != SweepCheckedFailed {
			t.Fatalf("state = %s, want %s — findings=%v couldNotCheck=%v", report.State, SweepCheckedFailed, report.Findings, report.CouldNotCheckReasons)
		}
		if !hasFinding(report.Findings, FindingLaneViolation, "msg-bad-lane") {
			t.Fatalf("findings = %v, want a %s finding for msg-bad-lane", report.Findings, FindingLaneViolation)
		}
	})

	t.Run("invalid-assertion acceptance is detected", func(t *testing.T) {
		root := t.TempDir()
		_, signer := newSweepKeypair(t)
		// The trust store the sweep re-checks against does NOT carry
		// cell-a's key (revoked/rotated/never-known) — a message that was
		// accepted with a signature valid THEN no longer verifies TODAY.
		emptyTrust := comms.Ed25519TrustStore{}

		assertion, err := comms.Mint("cell-a", "worker-desk", "msg-bad-sig", "nonce-2", sweepTestNow, 2*time.Minute, signer)
		if err != nil {
			t.Fatalf("Mint: %v", err)
		}
		landed := SweepRecord{
			Time: sweepTestNow, Kind: SweepKindLanded, ID: "msg-bad-sig",
			From:      comms.SenderID{Cell: "cell-a", Role: "worker-desk"},
			To:        comms.Lane{Cell: "cell-a", Role: "the-desk"},
			Verb:      "notify",
			Assertion: &assertion,
		}
		writeJournalLines(t, root, mustJSON(t, landed))

		report := Sweep(root, "cell-a", time.Time{}, SweepDeps{Trust: emptyTrust, Now: sweepTestNow.Add(time.Second)})

		if report.State != SweepCheckedFailed {
			t.Fatalf("state = %s, want %s — findings=%v couldNotCheck=%v", report.State, SweepCheckedFailed, report.Findings, report.CouldNotCheckReasons)
		}
		if !hasFinding(report.Findings, FindingInvalidAssertion, "msg-bad-sig") {
			t.Fatalf("findings = %v, want a %s finding for msg-bad-sig", report.Findings, FindingInvalidAssertion)
		}
	})

	t.Run("orphan spawn is detected", func(t *testing.T) {
		root := t.TempDir()
		// No landed/quarantined record for "msg-never-landed" exists
		// anywhere in the journal, and the action is outside the closed
		// vocabulary — a session with no accountable routing decision
		// behind it.
		spawn := SweepRecord{
			Time: sweepTestNow, Kind: SweepKindSpawn, ID: "spawn-orphan", Cell: "cell-a",
			Action: "not-a-real-action", AssignClass: "routine", AssignRisk: false,
			MsgID: "msg-never-landed", SessionID: "sess-orphan",
		}
		writeJournalLines(t, root, mustJSON(t, spawn))

		report := Sweep(root, "cell-a", time.Time{}, SweepDeps{Now: sweepTestNow})

		if report.State != SweepCheckedFailed {
			t.Fatalf("state = %s, want %s — findings=%v couldNotCheck=%v", report.State, SweepCheckedFailed, report.Findings, report.CouldNotCheckReasons)
		}
		if !hasFinding(report.Findings, FindingOrphanSpawn, "sess-orphan") {
			t.Fatalf("findings = %v, want a %s finding for sess-orphan", report.Findings, FindingOrphanSpawn)
		}
	})
}

func hasFinding(findings []Finding, kind FindingKind, id string) bool {
	for _, f := range findings {
		if f.Kind == kind && f.ID == id {
			return true
		}
	}
	return false
}

// --- fail-closed on unreadable input ---------------------------------------

// TestSweepCorruptJournal proves a truncated/corrupt journal.log line is
// NEVER read as clean: the run reports could-not-check with a non-empty
// reason naming the bad line, even though every other input source is
// perfectly readable.
func TestSweepCorruptJournal(t *testing.T) {
	root := t.TempDir()
	// A line that opens like a structured record but is truncated
	// mid-object — the realistic shape of "the writer crashed mid-append".
	writeJournalLines(t, root, `{"time":"2026-09-12T12:00:00Z","kind":"landed","id":"msg-1"`+
		`,"from":{"cell":"cell-a"`)

	report := Sweep(root, "cell-a", time.Time{}, SweepDeps{Now: sweepTestNow})

	if report.State != SweepCouldNotCheck {
		t.Fatalf("state = %s, want %s — a corrupt journal line must never read as clean", report.State, SweepCouldNotCheck)
	}
	if len(report.CouldNotCheckReasons) == 0 {
		t.Fatalf("CouldNotCheckReasons is empty, want it to name the corrupt line")
	}
}

// TestSweepCorruptJournalOutranksAClean SIDE also proves precedence: a
// corrupt line alongside an otherwise-legal record still resolves to
// could-not-check, never checked-clean and never a bare checked-failed that
// would hide the read failure.
func TestSweepCorruptJournalOutranksClean(t *testing.T) {
	root := t.TempDir()
	legal := SweepRecord{
		Time: sweepTestNow, Kind: SweepKindLanded, ID: "msg-ok",
		From: comms.SenderID{Cell: "cell-a", Role: "worker-desk"},
		To:   comms.Lane{Cell: "cell-a", Role: "the-desk"},
		Verb: "notify",
	}
	writeJournalLines(t, root, mustJSON(t, legal), "not json, not the legacy shape either")

	report := Sweep(root, "cell-a", time.Time{}, SweepDeps{Now: sweepTestNow})

	if report.State != SweepCouldNotCheck {
		t.Fatalf("state = %s, want %s", report.State, SweepCouldNotCheck)
	}
}

// TestSweepHeldMailboxIsReconciled proves the held/*.json source (real
// today, independent of any journal.log format) is itself swept: a
// quarantined message whose recorded assertion no longer verifies is a
// finding even though it never reached journal.log at all.
func TestSweepHeldMailboxIsReconciled(t *testing.T) {
	root := t.TempDir()
	_, signer := newSweepKeypair(t)
	assertion, err := comms.Mint("cell-a", "worker-desk", "msg-held", "nonce-3", sweepTestNow, 2*time.Minute, signer)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	env := comms.Envelope{
		Schema: comms.Schema, ID: "msg-held",
		From: comms.SenderID{Cell: "cell-a", Role: "worker-desk"},
		To:   comms.Lane{Cell: "cell-a", Role: "the-desk"},
		Verb: "notify", Sent: sweepTestNow, Sig: assertion,
	}
	if err := commsqueue.WriteHeld(root, env, "quarantined for test", sweepTestNow); err != nil {
		t.Fatalf("WriteHeld: %v", err)
	}

	// Empty trust store: the held message's assertion cannot be re-verified.
	report := Sweep(root, "cell-a", time.Time{}, SweepDeps{Trust: comms.Ed25519TrustStore{}, Now: sweepTestNow.Add(time.Second)})

	if !hasFinding(report.Findings, FindingInvalidAssertion, "msg-held") {
		t.Fatalf("findings = %v, want a %s finding for the held msg-held", report.Findings, FindingInvalidAssertion)
	}
}
