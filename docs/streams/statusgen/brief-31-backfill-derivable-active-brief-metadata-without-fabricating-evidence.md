---
brief: assay:assay:statusgen:31
title: "Backfill derivable active brief metadata without fabricating evidence"
why: "Missing file declarations and one-way routing claims produce persistent debt. Historical provenance can supply some answers; uncertain answers must remain owned work rather than plausible invented metadata."
wave: 2
depends: ["statusgen/28", "statusgen/36"]
unblocks: ["statusgen/35"]
effort: M
gate: human
risk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: "2026-10-17 by the maintainer-directed lint-debt refresh"
sources:
  - "docs/proposals/fixed-delivery-verification/README.md — fixed-delivery completion amendment, 2026-10-27; planning only"
  - "freshness-checked 2026-10-27 @ 9a58a792c; existing PR 4579 and source/caller work map"
  - "docs/streams/statusgen/README.md"
  - "freshness-checked 2026-10-17 @ 8f1206fe5eb4b4ab19f5f2f7266c515aa71f8eae"
exec-tier: strong
exec-tier-why: "Cross-artifact classification, rewrite semantics and consumer verification must agree."
domain: complicated
outcome: none
version: 2
id: 949bf322-b918-466c-834f-a24009819c3c
gate-why: "Changes tooling behavior relied on by adopters. Human sign-off confirms the new authoring or repair behavior preserves historical evidence and the existing closure gates."
decision-trigger: start
consumers:
  - "statusgen/fixfrontmatter.go: follow-up statusgen/31 (this brief; change is owed at implementation)"
  - "statusgen/fixfrontmatter_test.go: follow-up statusgen/31 (this brief; change is owed at implementation)"
  - "statusgen/main.go: follow-up statusgen/31 (this brief; change is owed at implementation)"
  - "statusgen/consumers.go: follow-up statusgen/31 (this brief; change is owed at implementation)"
  - "statusgen/lifecyclelint.go: follow-up statusgen/31 (this brief; change is owed at implementation)"
  - "changelog/fix-frontmatter.md: follow-up statusgen/31 (this brief; change is owed at implementation)"
---

# Brief 31 — Backfill derivable active brief metadata without fabricating evidence

## Context

files: statusgen/fixfrontmatter.go, statusgen/fixfrontmatter_test.go, statusgen/main.go, statusgen/consumers.go, statusgen/lifecyclelint.go, changelog/fix-frontmatter.md
facts:
- Rule membership, exclusions and derivable/author-owed dispositions are defined by the [public rule-family inventory](README.md#public-rule-family-inventory); use that shared inventory for this brief's Task and Verify acceptance.
- No fix-frontmatter command exists at the checked source revision.
- Consumers coverage requires a genuine back-reference, and routing tokens record whether work is still owed.
- Git diff files are candidates for a declaration, not proof that every file belongs to the brief.

design-fit:
  owner: statusgen
  contract: none — extend existing reporting, reconciliation and pin contracts; no new semantic owner.
  retires: ["the replaced per-site notice behavior or unresolved corpus debt named in Task"]
  weight: "No additional daemon or independent rule table. Exact flags and production code delta are measured by the implementation PR."
  why-add: "Reuse the existing owner; any new CLI entry point is limited to the reviewed repair/reporting operation."

layering: Keep parsing/classification pure and reuse the existing command owner for filesystem, git and forge effects. Fixtures exercise the boundary without live infrastructure.

single-point-of-failure: A mislabeled finding could evade the authoring gate; generator refusal, differential comparison and closure/witness checks are exercised independently.

## Human decision

Confirm the implemented behavior rejects new actionable defects while preserving historical evidence, existing closure checks and explicit unknown states. Review the concrete implementation and its independent verification before approval.

Options:
1. Approve the implementation as verified.
2. Request the identified corrections before approval.
3. Hold the change.

Default if no answer: none — blocks until answered.

Hold the change; no sign-off is inferred.

## Ground rules

- Use an isolated feature branch and a draft PR; merge and workflow activation keep their existing human gates.
- Do not rewrite recorded Evidence or frozen closed commands. Do not query clusters or production endpoints.
- Stop at implementation; an independent verifier records the execution evidence.

## Task

### Fixed-delivery completion amendment — 2026-10-27

This is a planning amendment under [fixed-delivery
verification](../../proposals/fixed-delivery-verification/README.md), not runtime implementation or
activation. Preserve the original Verify table and Evidence. The changed fixed-subject,
historical-subject and row-reuse policy waits for the precise existing authority decisions; an old
decision token grants none of these changes. Existing approved work retains its original contract.
At pickup, pin this brief/version, the controlling contracts, actual producer/consumer versions and
open PRs; adapt or hold only the affected slice. No new evaluator, record store, desk, tracer or
qualification stage is authorized.

**Owner: historical active metadata disposition.** Reuse /36's admitted subject/criteria meanings in
the already-planned metadata repair owner. Do not mint a trusted delivery binding: /37 owns that
operation. `files:` remains the implementation-touch declaration; never infer a transitive test read
set from it.

- For open historical briefs, derive only corroborated source/scope/consumer metadata, preserve
  original acceptance and Evidence, and classify missing or ambiguous facts as author-owed with the
  existing review/issue owner and next action. Wrap/prose normalization must retain the same scope;
  a genuinely changed scope is not bookkeeping. - Resolve parser compatibility through /37's
  existing deskkit owner, not a new grammar or repair pass. Produce the reviewed
  metadata/disposition inputs consumed by /35; do not invent a witness, change a routing obligation,
  or auto-close a missing-context brief. - Extend `TestFixFrontmatter`'s existing fixture matrix
  with a historical wrapped declaration, an ambiguous delivery, omitted consumers and unchanged
  closed-record hashes. Keep the old command name/test selector only where it actually matches the
  existing planned Verify rows; all newly added cases must be exercised by that brief's package-test
  row.

### Existing task obligations

1. Add fix-frontmatter with a default reviewed dry run and an explicit write mode. Restrict writes to active briefs and preserve Evidence and closed records.
2. Derive candidate files from immutable carrying-PR history only when the provenance is available and unambiguous. Do not stamp an empty-looking sentinel as a truthful files list. List missing or ambiguous provenance as author-owed.
3. Append a genuine source back-reference at an existing active follow-up target; retain routing commitments. Suggest typed ordering edges for review rather than silently deciding dependencies. Do not generate why text or security-review claims.
4. Honor the pre-2026-08-15 security-review grandfather policy in lint based on corroborated closure dates, without mutating a closed record. Missing dates stay could-not-check. Keep archive lifecycle exclusion in the reporting owner from statusgen/28.
5. Atomic writes must re-lint and be idempotent. Report each unresolved item with a path, kind and needed evidence.

## Verify

Commands run from this repository root. Named new test contracts below are planned deliverables; they must be added by the implementer and the PASS assertion must fail if the selector finds nothing.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd statusgen && go test ./... -run "^TestFixFrontmatter$" -count=1 -v > "${TMPDIR:-/tmp}/notice-31.log" 2>&1 && grep -F -e "--- PASS: TestFixFrontmatter " "${TMPDIR:-/tmp}/notice-31.log"` | exit 0; output contains the named PASS line. Matrix: unique and ambiguous carrying PRs; missing git history; truthful reciprocal sources; closed follow-up target is listed rather than edited; no changed risk/why/Evidence; grandfather boundary and unknown date; malformed candidate rolls back; second run is empty | check:ci +mutation +flow +dereference |
| 2 | `cd statusgen && go test ./... -count=1` | exit 0; existing closure, witness and differential tests still pass | check:ci +neighbour |
| 3 | `statusgen --root . --consumers` | exit 0; the implementation diff corroborates the routing commitments | check +flow |

## Evidence

<!-- Independent verification records command, exit/output, revision, binary digest, date and runner here. This authoring PR executes no future implementation check. -->

## Review

Gate: human. Review the fixture matrix and frozen-history boundary, not just the presence of new tests.

## Failure map

- A selector runs zero tests: the named PASS assertion in the test row fails.
- Historical data is changed to reduce the count: the required pre/post hash comparison and independent review fail.
- Only the first reported example is repaired: the matrix plants a second defect at another site.
- Source ships but consumer stays stale: adoption briefs require the published version and digest receipt.
