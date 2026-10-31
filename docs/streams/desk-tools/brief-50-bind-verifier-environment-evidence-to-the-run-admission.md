---
brief: assay:assay:desk-tools:50
title: "Bind verifier environment evidence to the run admission"
why: "Preflight and source admission already exist, yet a row can still fail because a required instrument or declared sibling is stale. Capture those prerequisites once so missing environment is not misreported as a deliverable defect."
wave: 1
depends: []
unblocks: ["desk-tools/51"]
effort: M
gate: human
risk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: no}
gate-why: "Changes verification scheduling/landing behavior relied on by adopters; human review must confirm the preserved provenance and authority boundaries."
decision-trigger: start
decision-issue: 4393
issues: []
schema: brief-v2
authored: "2026-10-17 maintainer-directed verify-flow refresh"
sources:
  - "docs/streams/desk-tools/README.md"
  - "freshness-checked 2026-10-17 @ 8f1206fe5eb4b4ab19f5f2f7266c515aa71f8eae"
exec-tier: strong
exec-tier-why: "Cross-component queue, input provenance and effect boundaries must agree; a subtle error can suppress needed verification or advance the wrong state."
domain: complicated
outcome: none
version: 2
id: b5cb55be-755f-4166-8f27-cfe3af63556c
consumers:
  - "tools/desk/internal/deskkit/flow_50_test.go: fixed-here (planned test contract)"
  - "tools/desk/cmd/verifyloop/flow_50_test.go: fixed-here (planned test contract)"
  - "tools/desk/internal/deskkit/preflight.go: fixed-here (this implementation brief)"
  - "tools/desk/cmd/verifyloop/preflight.go: fixed-here (this implementation brief)"
  - "tools/desk/cmd/verifyloop/dispatch.go: fixed-here (this implementation brief)"
  - "tools/desk/cmd/verifyloop/association.go: fixed-here (this implementation brief)"
  - "tools/desk/cmd/verifyloop/attestation.go: fixed-here (this implementation brief)"
  - "tools/desk/internal/deskkit/verifierattestation.go: fixed-here (this implementation brief)"
  - "changelog/verifier-world-evidence.md: fixed-here (this implementation brief)"
---

# Brief 50 — Bind verifier environment evidence to the run admission

## Context

files: tools/desk/internal/deskkit/preflight.go, tools/desk/cmd/verifyloop/preflight.go, tools/desk/cmd/verifyloop/dispatch.go, tools/desk/cmd/verifyloop/association.go, tools/desk/cmd/verifyloop/attestation.go, tools/desk/internal/deskkit/verifierattestation.go, changelog/verifier-world-evidence.md, tools/desk/cmd/verifyloop/flow_50_test.go, tools/desk/internal/deskkit/flow_50_test.go
facts:
- verifyloop preflight already checks the operating envelope and runner table, including per-root coverage. Verifier admission already binds the source, verifier home and run associations.
- Wake readers already distinguish unchanged, changed and unreadable declared inputs; reuse those boundaries.
- Reading a live cluster or production endpoint is not a prerequisite check this workflow may perform, even read-only.

layering: Extend the existing classifier, receipt, Evidence, budget or admission owner named in Task. Keep policy evaluation pure and fixture-test external effects through the existing typed adapter; do not add a daemon or duplicate authority.

design-fit:
  owner: "existing statusgen/desk verification owners"
  contract: none — this extends the existing queue/receipt and tool contracts; no new semantic owner.
  retires: ["the stale assumption or duplicated state inference described in why"]
  weight: "No new binary or limiter. CLI/field additions are limited to the specified existing owners; implementation records actual flag/refusal/text deltas."
  why-add: "A minimal prerequisite record in the existing admission owner; no additional service, production contact or credential scope."

single-point-of-failure: A stale or forged run could be reused. Run/input identity, existing independent acceptance/landing guards and end-to-end negative controls must each reject it independently.

## Human decision

Confirm that the reviewed implementation preserves the current human-only acceptance and source-provenance boundaries while providing the behavior described below. Approval of planning grants no workflow activation or additional write scope.

Options:
1. Approve the concrete implementation after independent verification.
2. Request corrections and keep the change pending.
3. Hold the change.

Default if no answer: none — blocks until answered.

Hold the change.

## Task

1. Extend the existing admission/preflight result with a run-bound prerequisite record: selected root/main revision, the declared sibling revisions actually needed, applicable pinned tool versions/digests, supported runtime and required App capability evidence.
2. Use local materialized revisions and existing authorized envelope observations by default. No token dumps, implicit live probes, blanket sweep of unrelated siblings or fresh per-row credential mint. If an external observation is unavailable, report it rather than trying a production endpoint.
3. Fail or hold before spending rows when a required prerequisite is missing. Keep environment could-not-check distinct from code FAIL and from a successful empty scan. Preserve healthy-root coverage and the exact reason for an unreadable/skipped root.
4. Invalidate the prerequisite record if source, tool or relevant sibling changes between admission and landing. Make landing consume the same run-bound record; a stale successful preflight cannot authorize a later tree.
5. Test instrument missing, pin drift, changed sibling, denied capability, zero scanned paths and a second root with unreadable prerequisites. Preserve the existing admission guard rather than replace it with a self-authored prose banner.

6. At implementation delivery, convert all 9 consumer entries that name this brief from `follow-up desk-tools/50` to `fixed-here`, preserving the exact sites and notes. These are deferred only in the authoring PR. Every owed path must occur in the delivering diff; a self-follow-up proves only that this brief exists. The consumer Verify row requires 9 corroborated, zero disproved and zero unchecked entries. Any proposed exclusion or reroute needs a reviewed brief amendment with its residual obligation, rather than silently lowering the count.

## Interface contract

A minimal prerequisite record in the existing admission owner; no additional service, production contact or credential scope.

## Ground rules

- Isolated branch and draft PR; no merge, main push, workflow activation or live infrastructure query in this brief.
- Existing narrow landing permissions apply at runtime; a refusal stops the operation and never licenses another identity or channel.
- Do not rewrite old Evidence, invent a witness, close real gate cards or fabricate a human stamp.
- Stop at implementation; independent verification owns acceptance Evidence.

## Verify

New test names below are planned contracts. The named PASS assertion must fail if a selector runs no test. Run commands from the repository root.

Consumer-diff provenance: obtain the full immutable `DELIVERY_HEAD` and `DELIVERY_BASE` from this brief's actual delivering PR; the base is its reviewed merge-base, not whichever main happens to be current. Record both SHAs and the command output in Evidence. After merge, use the delivery merge's second parent as the head and the merge-base of its two parents as the base, as described in `docs/brief-rules.md`. Run the consumer row in a separate checkout at that exact head, with those variables set; never move an already admitted verifier home to satisfy it. An absent delivery binding or empty/unrelated diff is could-not-check, never acceptance. The literal coverage summary accounts for every entry; no unchecked residue is accepted by this authored contract.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./internal/deskkit -run "^TestFlowWorld$" -count=1 -v > "${TMPDIR:-/tmp}/flow-50-tools-desk-internal-deskkit.log" 2>&1 && grep -F -e "--- PASS: TestFlowWorld " "${TMPDIR:-/tmp}/flow-50-tools-desk-internal-deskkit.log"` | exit 0; output contains the named PASS line. Cases: required prerequisite failure spends no row; environment unknown is distinct from FAIL; source/tool drift invalidates admission; a healthy root stays visible beside an unreadable root | check:ci +mutation +flow +dereference |
| 2 | `cd tools/desk && go test ./cmd/verifyloop -run "^TestFlowWorld$" -count=1 -v > "${TMPDIR:-/tmp}/flow-50-tools-desk-cmd-verifyloop.log" 2>&1 && grep -F -e "--- PASS: TestFlowWorld " "${TMPDIR:-/tmp}/flow-50-tools-desk-cmd-verifyloop.log"` | exit 0; output contains the named PASS line. Cases: required prerequisite failure spends no row; environment unknown is distinct from FAIL; source/tool drift invalidates admission; a healthy root stays visible beside an unreadable root | check:ci +mutation +flow +dereference |
| 3 | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-50-bind-verifier-environment-evidence-to-the-run-admission.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/50 --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 9 corroborated, 0 disproved, 0 unchecked' "$o"` | exit 0; exact delivering head/base, no deferred route, and all 9 consumer sites corroborated by the implementation diff. Zero unchecked residue; missing implementation, an empty diff or a routing-only authoring tree must not pass | check +flow +dereference |
| 4 | `cd statusgen && GOWORK=off go test . -run "^TestConsumersGateFiresOnFalseFixedHere$" -count=1 -v > "${TMPDIR:-/tmp}/flow-50-consumer-negative.log" 2>&1 && grep -F -e "--- PASS: TestConsumersGateFiresOnFalseFixedHere " "${TMPDIR:-/tmp}/flow-50-consumer-negative.log"` | exit 0; named PASS. Negative control: an owed fixed-here consumer exists but is absent from the implementation diff, so corroboration must exit 1; adding that path to the diff restores exit 0 | check:ci +mutation +dereference |


## Evidence

<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This authoring PR claims no future implementation test executed. -->

## Failure map

- Wrong queue or stale PASS suppresses real work: shared fixture and second changed-input controls must fail.
- A human/risk item becomes model acceptance: the existing lower landing guard must refuse with the planner bypassed.
- A missing root, instrument, ledger or forge response becomes clean: the unknown-state matrix must fail that result.
- Source changes but an old consumer claims rollout complete: the adoption brief requires published paired versions and a local UAT receipt.

## Review

Gate: human. Check semantic classification, provenance and actual permission boundaries; a passing shape/presence check is insufficient.
