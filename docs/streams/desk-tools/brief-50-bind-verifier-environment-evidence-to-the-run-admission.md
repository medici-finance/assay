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
ruling: "https://github.com/medici-finance/assay/issues/4393#issuecomment-6923694936"
design: DR-desk-tools-50
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

Ruled 16 November 2026 on decision issue #4393: option 1, approve the concrete
implementation delivered in #4392 after independent verification; the ruling grants no
workflow activation or additional write scope. Independent verification still owns
acceptance Evidence and every later lifecycle step.

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

### Post-merge verification, 2026-11-18 (non-implementer verifier)

Verification-Table: delivery=5a195f3b5a1e1825fdd688c415d9ea145672de48 base=c011fac2c224c0e871421d72ccd2a30d8de42c4e original=c011fac2c224c0e871421d72ccd2a30d8de42c4e

- Verified source: merged main 379852bcad72d258ef435b68e3b1e05ac3da0be0. Statusgen v1.0.87. Pre-work admission: Verification-Attestation medici-finance/assay#6938. The admitted world records capability=verifier-attestation (checked-clean), runtime=darwin-arm64, tool=v1.0.87 and Scanned 0. Risk line `customer: yes`, so every row owes a fail-first witness.
- Delivery: PR #4392, merge 5a195f3b5a1e1825fdd688c415d9ea145672de48. DELIVERY_HEAD (second parent) is 59c4b3d2dbeddb431c703e55c75a0fab71779755. DELIVERY_BASE (merge-base of the parents) is c011fac2c224c0e871421d72ccd2a30d8de42c4e. The Verify section is byte-identical at DELIVERY_BASE, DELIVERY_HEAD, the merge and the verified source. Re-derivation found 4 rows, 0 author-added, all from the base.
- Rows 1, 2 and 4 (check:ci) ran in one container-netns admission bracket with `--fail-first --delivering-commit 5a195f3b5a1e`. The fail-first base is the merge's first parent, 3761878af174. The history pack and module cache were mounted read-only, with GOPROXY=off. Bracket-digest verdict=match. The closed witness bytes follow verbatim.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `cd tools/desk && go test ./internal/deskkit -run "^TestFlowWorld$" -count=1 -v > "${TMPDIR:-/tmp}/flow-50-tools-desk-internal-deskkit.log" 2>&1 && grep -F -e "--- PASS: TestFlowWorld " "${TMPDIR:-/tmp}/flow-50-tools-desk-internal-deskkit.log"` | pass exit=0 sandbox=container-netns | sha256:32b936e48064 | 2026-11-18 | assay-verifier-app[bot] @ 379852bcad72 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=3761878af174 unproven · head=379852bcad72 |
| 2 | `cd tools/desk && go test ./cmd/verifyloop -run "^TestFlowWorld$" -count=1 -v > "${TMPDIR:-/tmp}/flow-50-tools-desk-cmd-verifyloop.log" 2>&1 && grep -F -e "--- PASS: TestFlowWorld " "${TMPDIR:-/tmp}/flow-50-tools-desk-cmd-verifyloop.log"` | pass exit=0 sandbox=container-netns | sha256:875fb83dc959 | 2026-11-18 | assay-verifier-app[bot] @ 379852bcad72 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=3761878af174 unproven · head=379852bcad72 |
| 4 | `cd statusgen && GOWORK=off go test . -run "^TestConsumersGateFiresOnFalseFixedHere$" -count=1 -v > "${TMPDIR:-/tmp}/flow-50-consumer-negative.log" 2>&1 && grep -F -e "--- PASS: TestConsumersGateFiresOnFalseFixedHere " "${TMPDIR:-/tmp}/flow-50-consumer-negative.log"` | pass exit=0 sandbox=container-netns | sha256:99a686106f74 | 2026-11-18 | assay-verifier-app[bot] @ 379852bcad72 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=3761878af174 unproven · head=379852bcad72 |
Tree-Digest: sha256:edebca9fff7917c484cbed034161c9d8ea1081921cd32bd7a3bc27fc7165e107
Container-Image: sha256:c67baa7a3c8ba3d514128f31340489146525a50c29ebb641a68899c650d354a1
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6938 run=1899ef04274fc64d3d9ea1eea6deca44a1b25da8de4431aa867d00ad2d45f9bc source=379852bcad72d258ef435b68e3b1e05ac3da0be0 brief=docs/streams/desk-tools/brief-50-bind-verifier-environment-evidence-to-the-run-admission.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=1899ef04274fc64d3d9ea1eea6deca44a1b25da8de4431aa867d00ad2d45f9bc source=379852bcad72d258ef435b68e3b1e05ac3da0be0 tree=sha256:edebca9fff7917c484cbed034161c9d8ea1081921cd32bd7a3bc27fc7165e107 witness=sha256:c984461088976fda039761df9e57495e31c11c59280adda0d636075b27cb4c19 opened=2026-11-18T16:44:49Z closed=2026-11-18T16:51:09Z before=receipt after=check-verifier

All three rows passed at head. Every Base cell reads `unproven`: the baseline checkout could not run any row offline, and no base log was retained. The admitted module cache is bound to the verified source's go.sum files, and the base pins different module versions. Across 36 module files there are 613 insertions and 46 deletions between 3761878af174 and 379852bcad72; for example, statusgen moves golang.org/x/sys from v0.46.0 to v0.49.0 and golang.org/x/text from v0.41.0 to v0.43.0. With GOPROXY=off the base cannot resolve them. This is a lane limit, not a deliverable result. Under `customer: yes`, an unproven base leaves each row HELD.

Verifier accounting, one line per original row. The supplemental observations below get no acceptance credit.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | row 1 as in the Verify table | exit 0 with the named PASS line, red at base | Pass at head with exit 0 and the named PASS line; container-netns witness above. Fail-first: unproven at base 3761878af174, because the row could not run there offline (base module set not in the admitted cache). HELD: could-not-run for the fail-first obligation. | 2026-11-18 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | row 2 as in the Verify table | exit 0 with the named PASS line, red at base | Pass at head with exit 0 and the named PASS line; container-netns witness above. Fail-first: unproven at base 3761878af174, for the same reason. HELD: could-not-run for the fail-first obligation. | 2026-11-18 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | row 3 as in the Verify table, with DELIVERY_HEAD=59c4b3d2dbed and DELIVERY_BASE=c011fac2c224 | exit 0 at the exact delivery head; summary 9 corroborated, 0 disproved, 0 unchecked | could-not-run. The row requires HEAD equal to DELIVERY_HEAD, and the brief forbids moving the admitted home, which sits at 379852bcad72. In a separate checkout at DELIVERY_HEAD, host verifyrun refused before any row: "pre-work verifier admission failed; no Verify rows executed ... missing original pre-work verifier record". This is a check-definition / lane gap, not a deliverable failure. No fail-first witness is possible. | 2026-11-18 | assay-verifier-app[bot] (observed the refusal) (on-behalf-of human:ian) |
| 4 | row 4 as in the Verify table | exit 0 with the named PASS line, red at base | Pass at head with exit 0 and the named PASS line; container-netns witness above. Fail-first: unproven at base 3761878af174, for the same reason as rows 1 and 2. HELD: could-not-run for the fail-first obligation. Advisory, not observed: the named test already exists at the base (statusgen/consumers_test.go:362), and the delivery does not touch statusgen, so a proven base is expected to be green, which would make this row non-discriminating. | 2026-11-18 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3-supp-head | the row 3 command by hand, in a separate checkout at DELIVERY_HEAD (no acceptance credit) | diagnostic only | rc=0; 0 follow-up markers; "summary: 9 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing" (all 9 consumer sites CORROBORATED) | 2026-11-18 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3-supp-base | the row 3 command by hand, in a separate checkout at DELIVERY_BASE (no acceptance credit) | diagnostic only | rc=1; 9 follow-up markers remain in the brief at base; the consumers gate reports COULD-NOT-CHECK (brief not in the diff) | 2026-11-18 | assay-verifier-app[bot] (on-behalf-of human:ian) |

Risk-bearing values: the enumeration covered the delivering diff c011fac2c224..59c4b3d2dbed (deskkit preflight, worldbrief and verifierattestation; verifyloop attestation; deskdispatch verifier attestation; the desk-tools pin). Every value is reversible.

- RISK-VALUE: DERIVED — runWorldVersion = 1 @ tools/desk/internal/deskkit/preflight.go:2082. The run-world record is new in this delivery, so 1 is its first schema. Recheck treats any other version, or an empty input set, as unreadable could-not-check (preflight.go:2490), which fails closed and never re-mints.
- RISK-VALUE: DERIVED — verifierWorldCapability = "verifier-attestation" @ tools/desk/internal/deskkit/worldbrief.go:33. It names the pre-work attestation stamp that the world carries and Recheck never re-reads (preflight.go:2517). This run's own admission recorded `capability=verifier-attestation=checked-clean`.
- RISK-VALUE: DERIVED — fullSHA = `^[0-9a-f]{40}$` @ tools/desk/cmd/verifyloop/attestation.go:229, used at attestation.go:189. A full lowercase SHA-1 object id. The repository object format is sha1, so 40 hex is exact, and abbreviated or symbolic revisions are refused.
- RISK-VALUE: DERIVED — world digest = SHA-256 via verifierDigest @ tools/desk/internal/deskkit/verifierattestation.go:74, applied in RunWorld.Digest (preflight.go:2123). Instrument inputs carry a `sha256:` prefix (preflight.go:2368, 2523). This is the same algorithm as the attestation run id (64 hex) and the witness Tree-Digest and Module-Cache lines.

**VERIFY: BLOCKED** — no obligation failed. Rows 1, 2 and 4 pass at head, but their fail-first base 3761878af174 is unproven: the container lane's head-bound module cache cannot build the base offline. Row 3 could-not-run: its separate-checkout lane has no admission. Total: 0 of 4 rows passed with full obligations. Status not flipped; gate is human.

## Failure map

- Wrong queue or stale PASS suppresses real work: shared fixture and second changed-input controls must fail.
- A human/risk item becomes model acceptance: the existing lower landing guard must refuse with the planner bypassed.
- A missing root, instrument, ledger or forge response becomes clean: the unknown-state matrix must fail that result.
- Source changes but an old consumer claims rollout complete: the adoption brief requires published paired versions and a local UAT receipt.

## Review

Gate: human. Check semantic classification, provenance and actual permission boundaries; a passing shape/presence check is insufficient.
