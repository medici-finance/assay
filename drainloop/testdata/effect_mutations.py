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
    ("contract-field-drift", "../spec/operator-incarnation-v1.md", "- `KeyRef` — `key_ref`; an authorized reference",
     "- `OldKeyRef` — `key_ref`; an authorized reference", "TestIncarnationContractFields"),
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
# GE/16 preparation, review round 2: strict decode and ruled contract text.
mutations += [
    ("checkpoint-json-bound-bypassed", "ownership.go", "if !json.Valid(body) {",
     "if false && !json.Valid(body) {", "TestCheckpointSyntaxBound"),
    ("checkpoint-null-accepted", "ownership.go", "if v == nil {",
     "if false && v == nil {", "TestCheckpointNullPositionsHeld"),
    ("checkpoint-duplicate-accepted", "ownership.go", "if _, seen := obj[key]; seen {",
     "if _, seen := obj[key]; false && seen {", "TestCheckpointObjectKeysHeld"),
    ("checkpoint-case-variant-accepted", "ownership.go", "if !names[key] {",
     "if false && !names[key] {", "TestCheckpointObjectKeysHeld"),
    ("custody-read-wire-drift", "incarnation.go", 'json:"read_signing"',
     'json:"read_key"', "TestCustodyContractFields"),
    ("custody-volume-wire-drift", "incarnation.go", 'json:"volume_ref"',
     'json:"volume"', "TestCustodyContractFields"),
    ("restore-snapshot-wire-drift", "incarnation.go", 'json:"snapshot_ref"',
     'json:"snapshot"', "TestCustodyContractFields"),
    ("custody-restriction-softened", "../spec/operator-incarnation-v1.md",
     "Role executors never read signing material", "Role executors may read signing material",
     "TestRuledCustodyRestrictions"),
    ("restore-restriction-softened", "../spec/operator-incarnation-v1.md",
     "A snapshot or backup restore never presents operator-control state",
     "A snapshot or backup restore may present operator-control state",
     "TestRuledCustodyRestrictions"),
]
# GE/16 store-neutral protocol: compiling mutations of actual admission/boundary
# and accounting code; no test source is mutated and compile failures are rejected.
mutations += [
 ("cell-outbox-unreserved", "ownership.go", "return j.Authority.Commit(ctx, r)", "return nil", "TestCellOwnership"),
 ("cell-capability-unbound", "ownership.go", 'p.AdmissionDigest == "" || p.AdmissionDigest != CellAdmissionDigest(r)', "false", "TestCellOwnership"),
 ("cell-authentication-ignored", "ownership.go", "return a.Capabilities.CheckCellCapability(ctx, p)", "return nil", "TestCellOwnership"),
 ("cell-no-intent-authorized", "ownership.go", 'return errors.New("effect has no admitted intent")', "return nil", "TestCellOwnership"),
 ("cell-claim-group-conflated", "ownership.go", "return generation, nil", "return s.Checkpoint.Ownership.Generation, nil", "TestCellOwnership"),
 ("cell-claim-other-item", "ownership.go", "claim.ItemID == item && claim.Owner == s.Checkpoint.Ownership", "claim.Owner == s.Checkpoint.Ownership", "TestCellItemClaimGeneration"),
 ("cell-stale-group-class", "ownership.go", [
  ("if err = MatchCellFence(current, s.Checkpoint.Activation, s.Checkpoint.Ownership, p.Ownership); err != nil {", "if err = MatchCellFence(current, s.Checkpoint.Activation, s.Checkpoint.Ownership, p.Ownership); false {"),
  ("claim.Owner != s.Checkpoint.Ownership ||", "false ||")
 ], None, "TestCellOwnershipStaleWriterDenied"),
 ("cell-stale-claim-class", "ownership.go", [
  ("claim.Generation != r.Permit.ClaimGeneration || claim.Generation != r.Intent.Event.Generation", "false")
 ], None, "TestCellEffectRestoredRecords"),
 ("cell-released-claim-authorized", "ownership.go", "if matches != 1 {", "if false {", "TestCellEffectRestoredRecords"),
 ("cell-outstanding-not-charged", "budget.go", "if old.BalanceRef == q.BalanceRef {", "if false {", "TestCellBudgetConcurrentReservations"),
 ("cell-unknown-handback-released", "ownership.go", "len(s.Pending) != 0 || len(s.Checkpoint.PendingEffects) != 0", "false", "TestOwnershipBackendHandbackUnknown"),
 ("cell-incarnation-ignored", "ownership.go", "if err = MatchCellFence(current, s.Checkpoint.Activation, s.Checkpoint.Ownership, p.Ownership); err != nil {", "if err = MatchCellFence(current, s.Checkpoint.Activation, s.Checkpoint.Ownership, p.Ownership); false {", "TestIncarnationStaleRefused"),
 ("cell-restored-sql-current", "ownership.go", "func (a CellAuthority) check(ctx context.Context, s CellState, p CellPermit) error {\n\tcurrent, err := a.Current.CurrentIncarnation(ctx, p.Ownership.CellID)", "func (a CellAuthority) check(ctx context.Context, s CellState, p CellPermit) error {\n\tcurrent, err := OperatorIncarnation{Schema:IncarnationSchema,CellID:s.Checkpoint.Ownership.CellID,ID:s.Checkpoint.Activation.IncarnationID,KeyRef:s.Checkpoint.Activation.KeyRef}, error(nil)", "TestIncarnationMismatchPaused"),
 ("cell-contract-field-drift", "../spec/operator-incarnation-v1.md", "- `KeyRef` — `key_ref`; an authorized reference", "- `OldKeyRef` — `key_ref`; an authorized reference", "TestIncarnationSpecMatchesFields"),
 ("cell-unknown-reservation-lost", "ownership.go", "s.Checkpoint.Budget.Reservations[index].Unknown = true", "s.Checkpoint.Budget.Reservations = append(s.Checkpoint.Budget.Reservations[:index], s.Checkpoint.Budget.Reservations[index+1:]...)", "TestCellBudgetRestoreAllAttempts"),
 ("cell-pacing-seed-zero", "budget.go", "n := *p.Seed", "n := uint64(0)", "TestCellPacingRestartNoMultiply"),
 ("cell-pacing-other-process-ignored", "budget.go", "r.PacingScope == scope && r.PacingWindow == window", "false && r.PacingScope == scope && r.PacingWindow == window", "TestCellPacingRestartNoMultiply"),
]
budget_source = (root / "budget.go").read_text()
unknown_brake = '''if found == nil || found.Consumed == nil {
			return errors.New("brake usage unavailable")
		}'''
mutations.append(("cell-unknown-brake-zero", "budget.go", unknown_brake,
 '''if found == nil { return errors.New("brake usage unavailable") }; if found.Consumed == nil { n := uint64(0); found.Consumed = &n }''', "TestCellBrakeCountUnavailableHolds"))
start = budget_source.index('case "effect":')
end = budget_source.index('case "read":', start)
mutations.append(("cell-pacing-cached-only", "budget.go", budget_source[start:end], 'case "effect": n = p.Cached\n\t', "TestCellPacingRestartNoMultiply"))
# GE/16 review: safe retirement is authoritative, not a free retry or usage reset.
mutations += [
 ("cell-resolution-not-retired", "ownership.go", "s.Pending = append(s.Pending[:index], s.Pending[index+1:]...)", "s.Pending = s.Pending", "TestCellJournalProvenAbsence"),
 ("cell-resolution-unfenced", "ownership.go", 'result.FenceRef == ""', "false", "TestCellJournalProvenAbsence"),
 ("cell-resolution-unknown-class", "ownership.go", [
  ("if held.ID == q.ID {", "if false && held.ID == q.ID {"),
  ('if !known {\n\t\t\t\treturn errors.New("complete usage proof unavailable")', 'if false && !known {\n\t\t\t\treturn errors.New("complete usage proof unavailable")')
 ], None, "TestCellJournalProvenAbsence"),
 ("cell-resolution-lineage-reused", "ownership.go", "old.Admission.Permit.AttemptID == p.AttemptID || old.Admission.Permit.RequestID == p.RequestID", "false", "TestCellJournalProvenAbsence"),
 ("cell-usage-phase-unbound", "ownership.go", "if u.Phase != phase {", "if false && u.Phase != phase {", "TestCellUsagePhaseBound"),
 ("cell-authority-clock-ignored", "ownership.go", "reserveBudget(&s.Checkpoint.Budget, r, a.Now())", "reserveBudget(&s.Checkpoint.Budget, r, e.Time)", "TestCellAuthorityWindow"),
 ("cell-lower-window-ignored", "ownership.go", "if err := balanceWindow(balance, a.Now()); err != nil {", "if err := balanceWindow(balance, a.Now()); false {", "TestCellAuthorityWindow"),
]
# Security lane: isolate each stored-binding/settlement/completion guard.
mutations += [
 ("cell-stored-policy-unbound", "ownership.go", "if CellAdmissionDigest(r) != p.AdmissionDigest {", "if false && CellAdmissionDigest(r) != p.AdmissionDigest {", "TestCellEffectRestoredRecords"),
 ("cell-usage-identity-ignored", "ownership.go", 'return errors.New("usage identity/policy mismatch")', "_ = u", "TestCellUsageRefusals"),
 ("cell-usage-unrecorded-empty", "ownership.go", "if !found {", "if false && !found {", "TestCellUsageRefusals"),
 ("cell-usage-predecessor-ignored", "ownership.go", "if !seenUsage && latestSource != x.PreviousSourceRef {", "if false && !seenUsage && latestSource != x.PreviousSourceRef {", "TestCellUsageRefusals"),
 ("cell-settlement-change-accepted", "ownership.go", "if !reflect.DeepEqual(old, x) {", "if false && !reflect.DeepEqual(old, x) {", "TestCellUsageRefusals"),
 ("cell-record-intent-discarded", "ownership.go", "return j.CommitIntent(context.Background(), Intent{Event: e})", "return nil", "TestCellCompletionGeneration"),
 ("cell-completion-receipt-dropped", "ownership.go", "s.Receipts = append(s.Receipts, e)", "s.Receipts = s.Receipts", "TestCellCompletionGeneration"),
 ("cell-completion-generation-ignored", "ownership.go", "r.Intent.Event.Generation == e.Generation &&", "true &&", "TestCellCompletionGeneration"),
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
    edits = old if isinstance(old, list) else [(old, new)]
    mutated = source
    for before, after in edits:
        if mutated.count(before) != 1: raise SystemExit(name + ": mutation site is not unique")
        mutated = mutated.replace(before, after)
    try:
        path.write_text(mutated)
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
