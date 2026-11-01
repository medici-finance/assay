#!/usr/bin/env python3
"""Bounded, compiling recovery mutations; always restores the tracked source.
Run only in an owned clean worktree with no concurrent writers.
"""
from pathlib import Path
import os
import subprocess
import sys

root = Path(__file__).resolve().parents[1]
mutations = [
    ("duplicate-receipt-bypassed", "effect.go", "r, found, err := effect.Receipt(ctx, key)",
     "r, found, err := Receipt{}, false, error(nil)", "TestDuplicateDeliveryAppliesOnce"),
    ("lost-event-receipt-bypassed", "effect.go", "r, found, err := effect.Receipt(ctx, key)",
     "r, found, err := Receipt{}, false, error(nil)", "TestLostEventTriggersReconcile"),
    ("multiple-receipt-bypassed", "effect.go", "r, found, err := effect.Receipt(ctx, key)",
     "r, found, err := Receipt{}, false, error(nil)", "TestMultipleEffectsResume"),
    ("receipt-error-ignored", "effect.go",
     'if err != nil {\n\t\t\treturn effectResult(it, VerdictHold, "reconcile could-not-check", err)\n\t\t}',
     'if err != nil { err = nil; found = false }', "TestReceiptCouldNotCheckHolds"),
    ("missing-journal-accepted", "engine.go", "len(plans[i]) > 0 && c.Journal == nil",
     "false && len(plans[i]) > 0 && c.Journal == nil", "TestJournalRequiredForEffects"),
    ("missing-run-accepted", "effect.go",
     'if strings.TrimSpace(it.RunID) == "" || strings.TrimSpace(it.AttemptID) == "" {',
     'if false && (strings.TrimSpace(it.RunID) == "" || strings.TrimSpace(it.AttemptID) == "") {', "TestEffectKeysAndIdentity"),
    ("failure-becomes-unknown", "effect.go",
     "if errors.As(err, &failure) || errors.As(err, &failurePtr) {",
     "if false && (errors.As(err, &failure) || errors.As(err, &failurePtr)) {", "TestEffectOutcomeAndDiscovery"),
    ("ordinary-journal-fatal", "engine.go",
     'c.log("JOURNAL %s %s — sink error (non-fatal): %v", kind, id, err)',
     "panic(err)", "TestNonEffectJournalStaysBestEffort"),
    ("receipt-bypassed", "effect.go", "r, found, err := effect.Receipt(ctx, key)",
     "r, found, err := Receipt{}, false, error(nil)", "TestLostAckIsReconciledNotRepeated"),
    ("unknown-treated-as-absence", "effect.go",
     'if err != nil {\n\t\t\treturn effectResult(it, VerdictHold, "reconcile could-not-check", err)\n\t\t}',
     'if err != nil { err = nil; found = false }', "TestEffectUnsupportedIdempotencyHolds"),
    ("intent-error-ignored", "effect.go",
     'if err != nil {\n\t\t\treturn effectResult(it, VerdictError, "effect intent journal", err)\n\t\t}',
     'if err != nil { err = nil }', "TestIntentRecordFailureAbortsEffect"),
    ("declared-artifact-bypass", "engine.go", "if len(plans[idx]) > 0 {",
     'if len(plans[idx]) > 0 && it.Payload["kind"] == "effect" {', "TestDeclaredEffectCannotBypassJournal"),
    ("stale-generation-unchecked", "effect.go", "old.Event.Generation != generation",
     'old.Event.Generation == "impossible-generation"', "TestEffectIntentOutboxHandoff"),
    ("stale-found-completion-dropped", "effect.go", "event.Generation = old.Event.Generation",
     "_ = old", "TestEffectIntentOutboxHandoff"),
    ("non-atomic-intent", "effect.go", "err = handoff.CommitIntent(ctx, Intent{Event: event})",
     "err = c.Journal.Record(event)", "TestEffectIntentOutboxHandoff"),
    ("outbox-falls-back", "engine.go", "if c.RequireHandoff {",
     "if false && c.RequireHandoff {", "TestOutboxRequiredNoFallback"),
]
# GE/16 preparation: pure record/schema assertions only, not runtime activation.
mutations += [
    ("incarnation-ignored", "incarnation.go", "owned.IncarnationID != current.ID",
     "false", "TestCellFenceBoundary"),
    ("activation-unchecked", "incarnation.go", "active.Paused || active.StopReceipt == \"\"",
     "false", "TestCellFenceBoundary"),
    ("owner-generation-ignored", "incarnation.go", "if presented != owned {",
     "if false {", "TestCellFenceBoundary"),
    ("contract-field-drift", "../spec/operator-incarnation-v1.md", "- `KeyRef` —",
     "- `OldKeyRef` —", "TestIncarnationContractFields"),
    ("checkpoint-stop-lost", "ownership.go", '`json:"stopped"`',
     '`json:"stop_discarded"`', "TestCellCheckpointRoundTrip"),
]
# GE/16 preparation, review round 1: one mutant per MatchCellFence guard, the
# wire names, the fixture's schema and windows, and the strict checkpoint decoder.
mutations += [
    ("schema-unchecked", "incarnation.go", "current.Schema != IncarnationSchema",
     "false", "TestCellFenceBoundary"),
    ("current-cell-empty-ok", "incarnation.go", 'current.CellID == ""',
     "false", "TestCellFenceBoundary"),
    ("current-id-empty-ok", "incarnation.go", 'current.ID == ""',
     "false", "TestCellFenceBoundary"),
    ("current-key-empty-ok", "incarnation.go", 'current.KeyRef == ""',
     "false", "TestCellFenceBoundary"),
    ("pause-ignored", "incarnation.go", "active.Paused ||",
     "false ||", "TestCellFenceBoundary"),
    ("stop-receipt-ignored", "incarnation.go", 'active.StopReceipt == ""',
     "false", "TestCellFenceBoundary"),
    ("activation-id-ignored", "incarnation.go", "active.IncarnationID != current.ID",
     "false", "TestCellFenceBoundary"),
    ("activation-key-ignored", "incarnation.go", "active.KeyRef != current.KeyRef",
     "false", "TestCellFenceBoundary"),
    ("owned-cell-ignored", "incarnation.go", "owned.CellID != current.CellID",
     "false", "TestCellFenceBoundary"),
    ("owner-empty-ok", "incarnation.go", 'owned.OwnerID == ""',
     "false", "TestCellFenceBoundary"),
    ("generation-empty-ok", "incarnation.go", 'owned.Generation == ""',
     "false", "TestCellFenceBoundary"),
    ("backend-empty-ok", "incarnation.go", 'owned.Backend == ""',
     "false", "TestCellFenceBoundary"),
    ("key-wire-name-drift", "incarnation.go", 'KeyRef string `json:"key_ref"`',
     'KeyRef string `json:"key"`', "TestIncarnationContractFields"),
    ("schema-wire-name-drift", "incarnation.go", 'Schema string `json:"schema"`',
     'Schema string `json:"kind"`', "TestIncarnationContractFields"),
    ("fixture-schema-drift", "testdata/cell-checkpoint.json", '"schema": "cell-checkpoint/v1"',
     '"schema": "cell-checkpoint/v0"', "TestCellCheckpointRoundTrip"),
    ("fixture-windows-merged", "testdata/cell-checkpoint.json",
     '"window_end": "2026-11-01T00:00:00Z"', '"window_end": "2026-10-01T01:00:00Z"',
     "TestCellCheckpointRoundTrip"),
    ("absent-field-accepted", "ownership.go",
     'return fmt.Errorf("%s.%s: field absent", path, name)', "continue",
     "TestCheckpointAbsentFieldsHeld"),
    ("checkpoint-schema-unchecked", "ownership.go", "state.Schema != CellCheckpointSchema",
     "false", "TestCheckpointAbsentFieldsHeld"),
]
# GE/16 ruled restrictions on custody 1A: executor key custody and snapshot restore.
mutations += [
    ("executor-signing-read-ok", "incarnation.go", "if g.ReadSigning {",
     "if false {", "TestExecutorKeyReadRefused"),
    ("executor-write-ok", "incarnation.go", "if g.Write {",
     "if false {", "TestExecutorKeyReadRefused"),
    ("grant-identity-empty-ok", "incarnation.go", 'g.Identity == ""',
     "false", "TestExecutorKeyReadRefused"),
    ("grant-volume-empty-ok", "incarnation.go", 'g.VolumeRef == ""',
     "false", "TestExecutorKeyReadRefused"),
    ("grant-role-unknown-ok", "incarnation.go",
     'return errors.New("control grant role unknown")', "return nil",
     "TestExecutorKeyReadRefused"),
    ("restored-id-current-ok", "incarnation.go", "now.ID == restored.IncarnationID",
     "false", "TestRestoredControlNotCurrent"),
    ("restored-key-current-ok", "incarnation.go", "now.KeyRef == restored.KeyRef",
     "false", "TestRestoredControlNotCurrent"),
    ("snapshot-ref-empty-ok", "incarnation.go", 'restored.SnapshotRef == ""',
     "false", "TestRestoredControlNotCurrent"),
    ("restored-id-empty-ok", "incarnation.go", 'restored.IncarnationID == ""',
     "false", "TestRestoredControlNotCurrent"),
    ("restored-key-empty-ok", "incarnation.go", 'restored.KeyRef == ""',
     "false", "TestRestoredControlNotCurrent"),
]
selected = sys.argv[1:]
if selected:
    unknown = set(selected) - {m[0] for m in mutations}
    if unknown: raise SystemExit("unknown mutation: " + repr(unknown))
    mutations = [m for m in mutations if m[0] in selected]
for name, filename, old, new, test in mutations:
    path = root / filename
    original = path.read_bytes()
    source = original.decode()
    if source.count(old) != 1: raise SystemExit(name + ": mutation site is not unique")
    try:
        path.write_text(source.replace(old, new))
        result = subprocess.run(
            ["go", "test", "-timeout", "30s", "-count=1", "-v", "-run", "^" + test + "$", "."],
            cwd=root, env={**os.environ, "GOWORK": "off"}, capture_output=True, text=True, timeout=45)
        output = result.stdout + result.stderr
        if result.returncode == 0 or "--- FAIL: " + test not in output or "[build failed]" in output:
            print(output)
            raise SystemExit(name + ": not killed by the named runtime assertion")
        print("KILLED " + name + " (runtime assertion)")
        for line in output.splitlines():
            if ".go:" in line: print("  " + line.strip())
    finally:
        path.write_bytes(original)
print("Totals: " + str(len(mutations)) + " killed, 0 survived, 0 compile-only")
