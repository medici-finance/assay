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
