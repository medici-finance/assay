---
brief: assay:assay:statusgen:45
title: Prepare an offline fail-first cache for the selected delivery
why: "Offline verification can exercise the admitted head and still miss every baseline assertion because only head dependencies were fetched. A bounded delivery-specific cache lets verify-desk distinguish a real before/after result from a setup failure."
wave: 1
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "A human approves the bounded input to the networked preparation step: historical module paths from one verifier-selected merged delivery of this repository and its exact first parent, screened by the existing public-module and checksum rules. Egress of historical dependency requests is a scope decision, and an unavailable or ambiguous delivery binding must return could-not-check rather than widen it."
issues: [7036]
schema: brief-v2
version: 1
authored: 2026-11-19 by statusgen newbrief
sources:
  - "medici-finance/assay#7036 and its verifier report for desk-tools/50"
  - "docs/protected-paths.md, container-netns transition bracket; existing protected fail-first refusal"
  - "statusgen/failfirst.go, statusgen/containerbracket.go and statusgen/modcachefresh.go at d138a8f4f2"
  - "tools/desk/cmd/deskevidence/outcomejoin.go and the landed desk-tools/40, /41, /42 and /50 outcome records"
  - freshness-checked 2026-11-19 @ d138a8f4f2 (origin/main)
exec-tier: strong
exec-tier-why: "Delivery provenance, historical dependency screening and the offline receipt cross multiple existing ownership boundaries."
domain: complicated
outcome: none
consumers:
  - "statusgen verifyrun host open, ordinary container admission and fail-first execution, host close and bracket digest: fixed-here"
  - "deskevidence delivery PR corroboration in tools/desk and verifier pickup instructions in plugins/assay/skills/verify-desk: fixed-here"
  - "protected acceptance-transition execution and cross-repository version-2 admission: out-of-scope (those lanes have separate admission rules)"
---

# Brief 45 — Prepare an offline fail-first cache for the selected delivery

## Context

files: `deskcore/attestation/`, `forgeread/adapters/`, `tools/desk/cmd/deskevidence/outcomejoin.go`, `tools/desk/cmd/deskevidence/outcomejoin_test.go`, `statusgen/containerbracket.go`, `statusgen/containerhistory.go`, `statusgen/modcachefresh.go`, `statusgen/verifyrun.go`, `statusgen/failfirst.go`, `statusgen/basecache.go` (planned), `statusgen/basecache_test.go` (planned), `statusgen/testdata/basecache/` (planned), `statusgen/README.md`, `plugins/assay/skills/verify-desk/SKILL.md`, `changelog/verifyrun-base-cache.md` (planned).

facts:
- source: d138a8f4f2, 19 November 2026.
- owner: statusgen prepares and admits the cache, while the forge-backed delivery join is currently owned by deskevidence.
- existing behavior: the verifier's historical cache lacks some first-parent pins even when the head checks pass.
- evidence: the merged outcomes for desk-tools/40, /41, /42 and /50 and the six delivery PRs checked below.
- scope: ordinary same-repository host preparation, offline execution and close.

At source `d138a8f4f2`, the host downloads modules for the admitted head. The network-off container may later select a different tree for the fail-first rows. When the first parent needs a module version absent from the cache, the row never reaches its assertion. A green head and a matched receipt cannot supply that missing baseline.

These four earlier attempts identify concrete compatibility fixtures. The delivered PRs listed here were read back from the forge as merged. The four first parents predate `images/verify-runner/go-module-roots.txt`. Eleven of the fourteen module roots listed at the source above have both a go.mod and go.sum in each parent. Current-main comparisons found 35, 35, 36 and 35 changed module files respectively.

| Brief | Selected delivery PR | Merge | Actual first parent |
|---|---|---|---|
| desk-tools/40 | #4248 | c243d2d116fc | 3e24567bbc4e |
| desk-tools/41 | #4259 | 8a6d345ed3c3 | 1f579542d540 |
| desk-tools/42 | #3541 | a92fea2720a5 | 2f44050f8a70 |
| desk-tools/50 | #4392 | 5a195f3b5a1e | 3761878af174 |

The /42 outcome also cites #3584 and #3931. The verifier selects the delivery appropriate to the row group. Different delivery parents require different brackets. Input preparation is only one blocker. A row can still return green at both revisions, expose a defect or require a corrected acceptance command.

### Architecture pickup

- Source and constraints. Read `docs/go-engineering.md`, `docs/protected-paths.md`, the ordinary fail-first guidance in `plugins/assay/skills/verify-desk/SKILL.md`, `statusgen/containerhistory.go`, and `deskcore/attestation/attestation.go` at pickup.
- Existing ownership. The admitted host selects the brief and source. The forge adapter already reads the canonical repository, merged change and immutable merge SHA for the outcome writer. The fail-first resolver owns the first-parent and mainline checks. The module-cache builder owns egress screening, checksum enforcement and digesting.
- Continue with the ordinary same-repository lane. Extend the shared host delivery check as a library function available to both consumers. Preserve the current outcome writer's historical timestamp, visibility and redaction rules. The new path adds no binary, actor, cache service or row-time fetch.

## Human decision

Approve the bounded input to the networked preparation step. The recommended scope is one verifier-selected merged PR for this same repository and brief, corroborated through the existing admitted host-side forge reader, with its immutable merge and exact first parent checked in the source history. Historical dependency requests pass the existing public-module and checksum rules. An unavailable or ambiguous binding returns could-not-check. The alternative is continued host-based fail-first execution for these rows. A supplied raw revision, a mutable branch, an ambient module cache or an unexamined first PR in a list is insufficient.

## Task

1. Bind the proposed baseline to the admitted source and brief **before** any historical dependency lookup. Resolve the selected change through the existing admitted forge transport. Validate the canonical repository, brief-to-delivery association, merged state and immutable SHA. Keep exact first-parent and ancestry checks in the existing fail-first resolver. Reuse prior outcome PR identifiers as pickup hints when present. The decision must also work on a first run without an earlier outcome. When multiple deliveries contribute, require a named, corroborated one. Keep existing head-only receipts readable under their current semantics.
2. Materialize the selected base's manifests from immutable Git objects without invoking hooks, executing historical code or following symlinks outside the selected tree. Use a bounded root policy for trees with the current manifest and for the actual legacy parents above. For the legacy form, start with the admitted head's provisioned roots, use their historical go.mod/go.sum pairs, record exclusions, and refuse partial pairs, unresolved local replacements or required roots outside that selection. Limit local replacements to regular files within the historical tree; handle required in-tree replacement modules from the same revision. Preserve existing egress restrictions, proof of public ownership, isolated configuration, version checks and digest semantics for both sets. No union of unscreened paths may reach the proxy.
3. Let the host fill a new invocation-owned cache with the two screened dependency sets and bind the complete materialization to the receipt. Include the selected repo/PR, delivery, first parent, historical root/input digest and final cache digest in its admitted plan. Validate the same selection on the offline runner and at close. Reject stale or mismatched inputs, unsupported receipt semantics and incomplete preparation. If caching both versions causes a checksum disagreement or changes source bytes, refuse. A different historical module set must never silently fall back to the image or host cache.
4. Run the two revisions with the same read-only, network-off environment. Preserve the distinction between an executed baseline assertion going red, an unproven baseline due to infrastructure, and a non-discriminating green/green result. Retain the protected-transition container baseline refusal in `docs/protected-paths.md`; cross-repository and cluster staging are separate capabilities. Update the host/runner reference and verify-desk invocation at the normal entry point. An older reader that cannot honor the new bound input must refuse the new operation.
5. Supply an offline differential fixture consumable by both binaries. Build real miniature Go modules at two versions with committed checksums. Use a fixture-only local file proxy for the host and close it before either row, with an empty invocation-owned cache and no ambient Go config. Drive the public open/run/close path. Provide a way for the independent verifier to run the **same fixture and assertion** against a baseline binary without copying implementation logic. The old binary must reach the known setup failure or reject the new operation; a missing test or missing command alone supplies no evidence of behavioral improvement. Keep tests, fixture and docs together. Record source and binary revisions in the developer results.

## Verify

Run offline fixtures from the repository root. Check each named test actually ran. Keep authenticated production evidence and lifecycle writes with the independent verifier.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow | `out=$(GOWORK=off GOPROXY=off go -C statusgen test -count=1 -v -run '^TestBaseCacheRoundTrip$' . 2>&1) && case "$out" in *'--- PASS: TestBaseCacheRoundTrip ('*) true;; *) false;; esac` | Exit 0. The real host prepares both pinned versions; the reference behavior passes at head, executes and goes red at the selected first parent, closes with a matching bound receipt and no row-time network. The same fixture with the prior binary cannot produce this witness. |
| 2 | check:ci +dereference | `out=$(GOWORK=off GOPROXY=off go -C statusgen test -count=1 -v -run '^TestBaseCacheDelivery$' . 2>&1) && case "$out" in *'--- PASS: TestBaseCacheDelivery ('*) true;; *) false;; esac` | Exit 0. Correct merged PR succeeds. An unrelated ancestor, a different repository or brief, an unmerged change, mismatched SHA, a missing explicit selection on a multi-delivery brief and a swapped receipt each refuse before new module egress or credit. |
| 3 | check:ci | `out=$(GOWORK=off GOPROXY=off go -C statusgen test -count=1 -v -run '^TestBaseCacheHistoricalRoots$' . 2>&1) && case "$out" in *'--- PASS: TestBaseCacheHistoricalRoots ('*) true;; *) false;; esac` | Exit 0. A missing historical manifest with the documented legacy shape succeeds. Missing required inputs, partial pairs, an out-of-tree replacement, unapproved historical dependency, changed pins or conflicting checksums refuse. The fixture records zero proxy requests when screening refuses. |
| 4 | check:ci | `out=$(GOWORK=off GOPROXY=off go -C statusgen test -count=1 -v -run '^TestBaseCacheRefusals$' . 2>&1) && case "$out" in *'--- PASS: TestBaseCacheRefusals ('*) true;; *) false;; esac` | Exit 0. A missing historical version yields unproven, green/green stays non-discriminating, an altered cache refuses, legacy head-only reads keep their meaning, and protected container fail-first still refuses before rows. |
| 5 | check:ci +flow | `delivery=$(git log --first-parent --diff-filter=A --format=%H HEAD -- statusgen/basecache.go) && test "${#delivery}" -eq 40 && base=$(git rev-parse "$delivery^1") && out=$(statusgen --root . --consumers --brief statusgen/45 --delivery "$delivery" --base "$base" 2>&1) && echo "$out" && case "$out" in *'summary: '[1-9]*' corroborated, 0 disproved, '*) true;; *) false;; esac` | Exit 0 and a `summary:` line reporting at least one corroborated and zero disproved routes. The routing report evaluates the single first-parent change introducing the shared baseline-cache implementation and that commit's actual first parent. Record each routed consumer and the unchecked count: prose entries, `out-of-scope` entries and entries unchanged since that first parent print UNCHECKED, which is the reviewer's call and not a pass, so the implementing change must route at least one entry to a path its diff touches. Missing or multiple introduction commits refuse. |

## Evidence

The independent verifier records the exact source, outcome and bound delivery after the implementation lands.

## Review

Gate: human. The review covers historical module-path egress and delivery selection. Independent verification is required before lifecycle acceptance.
