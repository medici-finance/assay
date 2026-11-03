---
brief: assay:assay:forge-neutral:48
title: Bound native mirror response allocation before decoding
why: "One native response can allocate before the existing aggregate 8 MiB guard runs. Bound admitted mirror responses before buffering or SDK decoding, once for both providers."
wave: 3
depends: ["forge-neutral/47"]
unblocks: ["forge-neutral/37", "forge-neutral/50", "forge-neutral/53"]
split-from: forge-neutral/37
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Admitted private forge reads cross this native response boundary. Human acceptance must demonstrate early bounds preserve scope, credential custody, redirects and uncertainty on both success and SDK error paths."
decision-trigger: spec
decision-issue: 5384
issues: []
schema: brief-v2
version: 2
outcome: none
id: 894ac034-2261-4f1c-945e-7d18d83ff68b
authored: 2026-11-03 by statusgen newbrief
sources:
  - docs/streams/forge-neutral/brief-37-scoped-forge-mirror-with-persisted-freshness-coverage-and-polling-repair.md
  - docs/streams/forge-neutral/brief-47-admitted-incremental-forge-readers-for-mirror-ingestion.md
  - docs/streams/decisions/DR-forge-library.md
  - docs/go-engineering.md
  - "freshness-checked 2026-11-03 against App-fetched main 678d7ea5feccd434daddc65741aec34f7d258679"
exec-tier: strong
exec-tier-why: "SDK success and error decoding may allocate before an apparent caller guard; only native-path evidence with aggregate accounting bypassed distinguishes the two."
domain: complicated
consumers:
  - "tools/desk/internal/deskkit/forge_response_bounds.go, forge_response_bounds_test.go: fixed-here (shared early native bound and both provider controls)"
  - "tools/desk/internal/deskkit/mirrorsource.go: fixed-here (exactly two enumerated operations consume private bounded native access)"
  - "tools/desk/internal/deskkit/forge_github.go, forge_gitlab.go, sharedread.go: out-of-scope (reuse existing SDK page readers and lower scope checks unchanged; bound their private upstream transports)"
  - "forgeread/README.md: fixed-here"
  - "docs/contracts.md: fixed-here (register the native response bound and its weight)"
  - "workflow/store/, mirroringest.go and persisted watermark failure flow: follow-up forge-neutral/37"
  - "forgeclient/: follow-up desktools-v2/75 (relocate this implementation if present)"
  - "statusgen/: out-of-scope (no reader cutover or import change)"
---

# Brief 48 — Bound native mirror response allocation before decoding

## Context
files:
- `tools/desk/internal/deskkit/forge_response_bounds.go`
- `tools/desk/internal/deskkit/forge_response_bounds_test.go`
- `tools/desk/internal/deskkit/forge_github.go`
- `tools/desk/internal/deskkit/forge_gitlab.go`
- `tools/desk/internal/deskkit/sharedread.go`
- `tools/desk/internal/deskkit/mirrorsource.go`
- `tools/desk/internal/deskkit/mirrorsource_test.go`
- `forgeread/README.md`
- `docs/contracts.md`
- `changelog/native-mirror-response-bound.md`

facts:
- At main 678d7ea5, /47's admitted incremental readers exist; their aggregate 8 MiB payload budget runs after native response allocation.
- GitHub sharedAccess uses doJSONHeader; GitLab uses SDK response decoding. Both success and error paths need the bound.
- WithForgeReadContext privately copies backends and preserves request accounting. Reuse that pattern without changing unrelated operations.
- workflow/store and the joined mapper are absent. These transport tests cannot qualify persisted coverage, coalescing or cutover.

Read first: /37's full current Task and Verify, /47, docs/go-engineering.md, docs/library-first.md and DR-forge-library. Follow implementation-pickup before effects; record source, contracts and continue/adapt/hold on the PR.

## Human decision
Accept the demonstrated mirror-only native response boundary, or hold it for a named correction. Evidence covers both providers, decoded size, error bodies, scope and neighboring operations. Acceptance grants no private-data ingestion, persistent store, listener, credential or activation.

Default if no answer: none — blocks until answered

This gate covers acceptance and activation. Offline implementation and synthetic evidence preparation may proceed before acceptance; no private data is ingested or served.

## Ground rules
- Own isolated worktree and draft PR. No main push, merge, release, workflow dispatch or live infrastructure contact. Fake provider transports and synthetic data only.
- Reuse admitted native clients. No new credential owner, generic request/URL/token interface, environment discovery, CLI, flag, retry loop or network default.
- Keep all 23 /37 Verify Class/Command pairs and obligations. /37 row 23 still requires the joined mapper and persisted-watermark failure path.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Enforce at most 8 MiB of JSON bytes per native mirror-page response before full-body buffering or SDK decoding, using one shared implementation for GitHub and GitLab. Count bytes after supported content decoding. Read at most limit plus one detection byte from the decoded body. Content-Length alone is insufficient; include oversized non-2xx/error bodies.
2. Apply only to enumerated mirror page reads, including later conditional variants. Preserve unrelated logs, archives, ordinary reads and writes. Reuse private backend copies, cancellation, scope, no-redirect custody and request accounting.
3. Oversize produces a typed limit/could-not-check result: no successful population, no nonzero ObservedUpTo, no valid truncated prefix and no complete-empty success. Preserve /47's independent aggregate budget.
4. Through both actual native provider paths test below-limit and exact-limit valid populations; oversized valid JSON with known, absent or false lengths, chunking, supported compression expansion and error bodies. Count consumed bytes, close the body and prove cancellation. Direct native-access tests bypass aggregate accounting.
5. Plant and restore a production-path bypass independently for each provider; the named test must fail. Preserve admitted scope, readonly and frozen-surface controls and neighboring non-mirror behavior. Register the guard and its weight in docs/contracts.md; update the README and fragment. Retain the joined row-23 obligation and shared-library relocation owner.

design-fit:
  owner: trusted deskkit native mirror response composition
  contract: "8 MiB decoded JSON bytes per native mirror page before buffering/SDK decoding"
  retires: []
  weight: "verbs 0; flags 0; stores 0; one shared response-bound implementation"
  why-add: "Aggregate accounting cannot prevent one response allocating before its check."

single-point-of-failure: the native body wrapper. Behind it /47 independently refuses aggregate overflow and clears incomplete ObservedUpTo, while /37's future mapper and Store refuse watermark advance on failed scans. These later protections do not substitute for the native bound; test it with aggregate accounting bypassed.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorNativeResponseBounds$' ./internal/deskkit/ > "$o" && grep -F -- '--- PASS: TestMirrorNativeResponseBounds ' "$o")` | Both actual native providers cover Task 4 before decode, including errors and compression. Exact-limit valid data succeeds. Oversize has typed limit failure, zero population/ObservedUpTo, at most limit+one decoded bytes consumed and closed body. Independently bypassing either provider's production bound fails. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorResponseBoundIsolation$' ./internal/deskkit/ > "$o" && grep -F -- '--- PASS: TestMirrorResponseBoundIsolation ' "$o")` | Direct native access with aggregate accounting bypassed refuses oversized data. Scope refusal makes zero requests, readonly/redirect custody remains, cancellation terminates, attempts count once and neighboring non-mirror operations retain behavior. Removing mirror-only selection fails the neighbor control. |
| 3 | check:ci +flow | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestForgeSurfaceUnchangedByDeskread$' ./internal/deskkit/ > "$o" && grep -F -- '--- PASS: TestForgeSurfaceUnchangedByDeskread ' "$o" && GOWORK=off go test -count=1 -v -run '^TestForgeGitlabCoverage$' ./internal/deskkit/ > "$o" && grep -F -- '--- PASS: TestForgeGitlabCoverage ' "$o")` | Frozen operation set and GitLab inventory remain unchanged. |
| 4 | check:ci | `(cd forgeread && GOWORK=off go test -count=1 ./...)` | Existing offline root, adapters and pure kernel suites pass without new module dependencies. |
| 5 | check +dereference | `statusgen --root . --consumers --brief forge-neutral/48 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | Pre-merge, on the implementing branch: actual fixed-here edits corroborate; follow-ups prove routing only, out-of-scope judgments remain unchecked, could-not-check is never pass. Before post-merge verification, bind this row to the independently established immutable mainline delivery and its first parent using --delivery and --base; do not treat running the branch-relative command on merged main as a pass. This routing check never establishes producer identity by itself. |
## Pre-mortem and detection

- SDK errors allocate before success wrapping: row 1 error-body cases.
- False lengths or compressed wire size evade the bound: row 1 length and expansion cases.
- Aggregate accounting masks an absent native wrapper: row 2 bypass and row 1 provider mutations.
- Global capping changes unrelated operations or duplicates attempts: row 2 neighbor and accounting controls.
- Transport fixtures get called persistent ingestion: review-only boundary; /37 retains its joined row 23 unchanged.

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: human. Record the formal review on the PR. Independent Evidence and lifecycle updates use the supported verifier tools; main CI owns generated board rows. Confirm the shared native boundary is early and limited to mirror pages. Persistence, selected retention, SQL constraints, scheduling, coalescing and tool cutover remain /37 and downstream owners.
