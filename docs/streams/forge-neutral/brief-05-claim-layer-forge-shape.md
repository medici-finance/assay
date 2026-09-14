---
brief: assay:assay:forge-neutral:05
title: Claim layer — the GitLab shape of refs/dispatch/* and its release
why: >-
  The cross-machine dispatch claim is what stops two desks on two machines working the same
  brief. Creating and reading it is plain git and already forge-neutral; releasing it is not —
  `DeleteRef` maps only the `heads/` namespace on GitLab and answers could-not-check for
  `dispatch/`, saying in as many words that such a claim is "NOT reported released". A claim
  that can be taken and never given back is a slot lost for good, so on GitLab today the
  fleet's mutual exclusion degrades to a leak. This brief settles the namespace that works on
  both forges and proves the release round-trips.
wave: 2
depends: ["forge-neutral/01"]
unblocks: ["forge-neutral/10"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-02 by forge-neutral authoring session
sources:
  - "docs/streams/forge-neutral/brief-01-forge-resolution-contract.md — the resolver the sink obtains its Forge from"
  - "docs/streams/forge-gitlab/inventory.md — op 15 (DeleteRef) and its recorded GitLab namespace limit"
  - "freshness-checked 2026-09-02 @ deae247 — forge_gitlab.go:1227-1240 refuses every namespace but heads/; fanoutloop/land.go:107 fixes the namespace constant to `dispatch`; land.go:35 says the sink is wired only at cutover and its only caller is fanoutloop_test.go:561"
exec-tier: strong
exec-tier-why: "correctness depends on cross-component reasoning about a distributed mutual-exclusion primitive whose failure mode (a claim taken and never released) is invisible to any single-component test (question b)."
domain: complicated
consumers:
  - "tools/desk/cmd/fanoutloop: fixed-here (the sink is constructed from the resolver instead of being nil)"
  - "tools/desk/internal/deskkit/forge_gitlab.go: fixed-here (the release path for whichever namespace is chosen)"
  - "tools/desk/internal/loopengine/writescope_io.go: fixed-here if the namespace changes (the local read must find the same refs the writer creates)"
  - "tools/desk/cmd/deskdispatch: fixed-here (the claim-key grammar and the delegated script's contract)"
  - "tools/desk/internal/deskkit/claimref.go: fixed-here — ADDED at implementation time: the single definition of the claim namespace, its ref-path builder and its key parser, which every writer and reader derives from. The brief's files list anticipated this as `a create/read counterpart` inside forge_gitlab.go; it is one file up instead, because both backends and both readers consume it and none of them is the right home for it."
  - "tools/desk/cmd/desksupervise: fixed-here — ADDED at implementation time: the staleness reclaim releases a dispatch claim (actions.go) and its live source lists them (live.go). It was not in the brief's consumers list and is the second in-tree writer of this ref; leaving it on the old namespace would have been exactly the writer/reader split rows 7 and 8 exist to catch."
  - "plugins/assay/skills/worker-desk/SKILL.md, plugins/assay/skills/worker-desk/references/dispatch-runbook.md, plugins/assay/skills/pr-shepherd/SKILL.md, plugins/assay/skills/the-desk/SKILL.md: fixed-here — CORRECTED at implementation time (2026-09-07) from `follow-up forge-neutral/10`. The routing and Verify row 8 disagreed: row 8 greps `plugins/assay/skills` and requires that every mention name the SAME namespace as the constant, `since the skills' prose and the code must agree`. Deferring the prose would leave a shipped skill telling its reader to list a namespace the writer no longer uses, which is the reader/writer drift the brief's own pre-mortem calls the worst failure of this primitive. The prose is therefore corrected in this change and the routing follows the diff."
  - "the consumer repo's dispatch-claim script: out-of-scope (it lives in the repo being worked, not here; this brief fixes the CONTRACT it is invoked under and the release path it depends on)"
version: 1
id: 4973f73c-f2da-46d0-8e13-edd79fc1e867
---

# Brief 05 — Claim layer: the GitLab shape

## Context
files:
- `tools/desk/internal/deskkit/forge_gitlab.go` — `DeleteRef` and, if the design needs it, a
  create/read counterpart.
- `tools/desk/internal/deskkit/forge_refpath.go` — `ValidateRefPath`, which fixes the legal
  namespace set.
- `tools/desk/cmd/fanoutloop/land.go` — the dispatch sink and the namespace constant.
- `tools/desk/internal/loopengine/writescope_io.go` — the local claim read.
- `tools/desk/cmd/deskdispatch/dispatch.go` — the claim-key grammar and the script contract.
- `docs/streams/forge-neutral/claim-shape.md` (planned) — the decision record: which namespace,
  why, and what the GitLab release path is.

**Why the risk answers are all `no` even though `tools/desk/internal/deskkit/` is a security
path.** This brief touches no credential, no permission and no trust decision: it changes the
namespace a mutual-exclusion ref lives in and implements its delete. The custody binding it
consumes was settled under the human gate in `forge-neutral/01`. What it CAN get wrong is a
lost slot, which is an availability fault, and rows 6 and 7 are the checks that make one loud.

single-point-of-failure: the claim ref is a single mutual-exclusion control, and its failure
mode is silent — a claim that cannot be released does not error, it just never frees. Two
independent layers: the ref itself (taken and released through the forge), and the existing
age-and-branch staleness reclaim in the claim reader, which frees a slot on a different signal
(elapsed time plus no live branch) in a different component. A release that fails still gets
reclaimed; a reclaim that misfires still leaves the ref readable.

facts:
- `GitLabForge.DeleteRef` refuses every namespace but `heads/`, returning
  `Unverifiable("could-not-check: GitLab exposes no general ref-delete endpoint, so DeleteRef
  cannot serve %q — only the \"heads/<branch>\" namespace maps (the Branches API); a claim
  held outside refs/heads has no CE equivalent and is NOT reported released")` —
  `tools/desk/internal/deskkit/forge_gitlab.go:1227-1240`.
- The namespace is a non-interpolated constant precisely so no caller can widen it:
  `const dispatchRefNamespace = "dispatch"` (`tools/desk/cmd/fanoutloop/land.go:107`).
- Local claim reads are plain git and explicitly offline — `git -C <root> for-each-ref
  --format=%(refname) refs/dispatch/` with the envelope *"never `git ls-remote`, never a
  fetch"* (`tools/desk/internal/loopengine/writescope_io.go:6-9,47`). Remote reads in the desk
  skills use `git ls-remote origin 'refs/dispatch/*'`. Both work on any git host.
- The release used to be a raw endpoint call: the header records it as
  `gh api -X DELETE repos/<owner/repo>/git/refs/dispatch/<key>` and notes that the reachable
  surface was *"not 'delete a dispatch ref' but 'any endpoint on the forge, by any method'"*
  (`tools/desk/cmd/fanoutloop/land.go:55-62`). That passthrough is already closed; only the
  GitLab half is missing.
- The sink is unwired: `newForgeDispatchSink`'s only caller is a test
  (`tools/desk/cmd/fanoutloop/fanoutloop_test.go:561`); in production the field is nil and the
  sink errors *"no forge wired into the sink"* (`land.go:93-94`).
- Claim keys are `<repo>--<stream>--<NN>` / `<repo>--issue-<NN>`
  (`tools/desk/cmd/deskdispatch/dispatch.go:667`), bounded by `itemKeyRe`
  (`dispatch.go:63`) so a key cannot escape its namespace.
- GitLab's Branches API is the only ref-delete surface the backend found; whether a
  non-`heads/` ref can be pushed and deleted at all on a given deployment is a question about
  that deployment, not about the tool. Answer it by measurement, not by reading docs.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Do NOT restore an arbitrary-endpoint call to solve the release. The passthrough was closed
  deliberately; re-opening it to delete a ref trades a leaked slot for an open surface.
- If neither design below round-trips on the measured deployment, that is a finding, not a
  reason to ship a release that silently no-ops. Report NEEDS_CONTEXT and file it.

## Task
1. **Measure first.** Against a GitLab deployment, establish by live read whether a ref
   outside `refs/heads/` can be created, listed and deleted at all, and by which API. Record
   endpoint, status code and date in `claim-shape.md`. This is the fact the design turns on
   and it must not be taken from documentation.
2. **Choose the shape and record why.** Two candidates, both legitimate:
   (a) keep `refs/dispatch/<key>` and implement its delete on whatever endpoint step 1 finds;
   (b) move the claim into a reserved branch namespace the Branches API can serve — a
   `heads/`-prefixed claim namespace — accepting that a claim then appears in the branch list.
   Record the decision, its cost, and the rejected alternative in `claim-shape.md`. Whichever
   is chosen must be the SAME on both forges: two namespaces is two mutual-exclusion systems.
3. **Implement the release** on both backends so `DeleteRef` (or its replacement op) serves
   the chosen namespace on GitHub and GitLab alike, keeping `ValidateRefPath`'s guarantee that
   the argument is a ref path in the repo's own namespace and never an API path.
4. **Wire the sink.** `fanoutloop` obtains its `Forge` from `deskkit.ForgeFor` rather than
   holding nil. The *"wired only at cutover"* note at `land.go:35` is discharged here; delete
   it. The nil-forge error path stays as a guard, but it must no longer be the production
   state.
5. **Keep the local read consistent.** If the namespace changed, `writescope_io.go` and the
   desk skills' `ls-remote` pattern must read the same namespace the writer creates. A reader
   pointed at the old namespace sees zero claims and reports every slot free — the worst
   possible failure of this primitive.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `cd tools/desk && go build ./... && go test ./...` | exit 0 |
| 2 | check | `grep -c '^[\|] ' docs/streams/forge-neutral/claim-shape.md` | ≥ 4 — the live-read table from task 1 and the decision table from task 2 are present, each row citing an endpoint and a status code |
| 3 | check | `cd tools/desk && go test ./internal/deskkit/ -run TestClaimRefRoundTripBothBackends -count=1 -v` | exit 0 — create, list and delete the chosen namespace against both backends' recorded fixtures under the same scenario names |
| 4 | check | `cd tools/desk && go test ./internal/deskkit/ -run TestRefPathStillRejectsAPIPaths -count=1 -v` | **negative path**: an argument shaped like an API path, an absolute URL, or a bare unnamespaced component is still refused by `ValidateRefPath`; widening the namespace did not widen the guard |
| 5 | check | `cd tools/desk && go test ./cmd/fanoutloop/... -run TestSinkResolvesForge -count=1 -v` | exit 0 — the sink is constructed from the resolver; the test fails if the production path can still yield a nil forge |
| 6 | check | `cd tools/desk && go test ./cmd/fanoutloop/... -run TestReleaseFailureIsNotReportedReleased -count=1 -v` | **negative path**: when the delete fails, the sink reports the claim as NOT released (non-zero, message naming the key) and does not print or record a release — a leaked slot must be loud |
| 7 | check | `cd tools/desk && go test ./internal/loopengine/... -run TestClaimReaderNamespaceMatchesWriter -count=1 -v` | exit 0 — the reader's namespace is derived from the same constant the writer uses, so the two cannot drift |
| 8 | check | `grep -rn -e 'refs/dispatch' -e 'dispatchRefNamespace' tools/desk plugins/assay/skills --include='*.go' --include='*.md' \| grep -v _test.go` | every hit names the SAME namespace as the constant in `tools/desk/cmd/fanoutloop/land.go` — reviewed as a list, not a count, since the skills' prose and the code must agree |
| 9 | check | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -count=1 -v && go test ./internal/deskkit/ -run TestForgeNoPassthrough -count=1 -v` | exit 0 — no shell-out and no arbitrary-endpoint method was reintroduced to serve the delete |
| 10 | check | `cd tools/desk && go test ./internal/forgeban/... -count=1` | exit 0 — the ratchet is unchanged by this brief (it retires no permit row); a change here would mean a shell-out was added or removed unnoticed |
| 11 | check | `statusgen --root . --consumers --brief forge-neutral/05` | exit 0 — every `consumers:` routing claim is corroborated against this branch's own diff |
| 12 | check +mutation | **Mutation demonstration for the release guard this brief adds.** In `tools/desk/cmd/fanoutloop/land.go`, inside `ReleaseDispatchClaim`, move the `released dispatch claim …` `Fprintf` to BEFORE the `DeleteRef` call and replace the delete's error branch with `return nil` — the pre-brief shape, in which a release that failed is indistinguishable from one that succeeded. Then run row 6, and restore the file and re-run it | exit **1** on the mutant: three subtests of the row-6 test fail with `a release that did not happen was reported as success — the slot leaks silently`, and exit 0 again after restoring. This is the row that proves row 6 is sensitive to the one property the claim layer rests on — the guard reddens when the guarded thing is broken, rather than passing because nothing exercises it |

## Pre-mortem → detection map

| Failure mode of the work | Caught by |
|---|---|
| The release is "implemented" as a call that returns success without deleting anything, so slots leak silently | rows 3 (round trip: delete then list, and the list must not show it) + 6 |
| The namespace is changed for the writer but not the reader, so every slot reads free and the fleet double-dispatches | rows 7 + 8 |
| An arbitrary-endpoint passthrough is restored to serve the delete | rows 4 + 9 |
| The sink is left nil in production and the whole brief is dead code — the seam's existing disease | row 5 |
| The design is chosen from GitLab documentation rather than a live read, and the endpoint does not exist on the deployment | row 2 requires an endpoint and a status code per row; a docs-derived table has neither |
| Two namespaces ship, one per forge, so mutual exclusion is per-forge and a cross-forge desk pair collides | row 8, read as a list — one namespace must appear |
| The claim-key grammar is widened so a key escapes its namespace | row 4 (a bare unnamespaced component is refused) |
| The chosen design works but costs something (a claim visible in the branch list) that nobody recorded | row 2's decision table names the cost and the rejected alternative; adequacy of the trade-off is **review-only** |

## Evidence

**SELF-ATTESTED by the implementer** — every row below was run by the session that wrote the
change, so none of it earns `verified`. The stream README's Verified column stays `—` until a
non-implementer re-runs this table on merged main.

Runner: `assay-worker-app[bot]` · Date: 2026-09-07 · Working directory for every `go` command:
`tools/desk`.

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go build ./... && go test ./...` | 0 | every package `ok`; no `FAIL` | 2026-09-07 | implementer |
| 2 | `grep -c '^[\|] ' docs/streams/forge-neutral/claim-shape.md` | 0 | `22` (≥ 4). §1 is the live-read table — 11 rows, each an endpoint plus a status code plus a date, 9 of them read live on 2026-09-07 and 1 quoted from the 2026-09-02 pilot; §2 is the decision table, 4 rows, naming the adopted option, the rejected ones and the reason each was rejected | 2026-09-07 | implementer |
| 3 | `go test ./internal/deskkit/ -run TestClaimRefRoundTripBothBackends -count=1 -v` | 0 | `--- PASS`; 6 subtests — 3 scenarios × the two backends, same scenario names on both: take/list/release/list, release frees only the named claim, release of an absent claim is a distinguishable not-found | 2026-09-07 | implementer |
| 4 | `go test ./internal/deskkit/ -run TestRefPathStillRejectsAPIPaths -count=1 -v` | 0 | `--- PASS`; 15 refused ref shapes (API-path traversal, absolute and scheme-relative URL, bare unnamespaced component, bare claim key, empty, bare namespace, leading separator, query, fragment, percent escape, shell metacharacter, option-shaped component, embedded newline) plus 7 refused claim-key shapes, plus the non-vacuity leg: the three real key shapes still resolve and round-trip through the key parser | 2026-09-07 | implementer |
| 5 | `go test ./cmd/fanoutloop/... -run TestSinkResolvesForge -count=1 -v` | 0 | `--- PASS`; the default sink is the releasing one and carries a resolver, the printing sink is reached only by asking for it, a nil resolver is refused at construction, and the resolver is asked for the item's own target repo | 2026-09-07 | implementer |
| 6 | `go test ./cmd/fanoutloop/... -run TestReleaseFailureIsNotReportedReleased -count=1 -v` | 0 | `--- PASS`; 5 subtests, one per way the release can fail (forge refuses, credential refused, backend cannot serve the namespace, resolver refuses, resolver yields nothing) — each returns non-nil, names the claim key, and emits no output at all; plus the mirror leg proving a real release IS recorded | 2026-09-07 | implementer |
| 7 | `go test ./internal/loopengine/... -run TestClaimReaderNamespaceMatchesWriter -count=1 -v` | 0 | `--- PASS`; the reader's prefix IS the shared constant, and in a real temporary git repo the reader finds exactly the ref the writer's own path builder named, while a ref in the old namespace and an ordinary branch are both correctly not claims | 2026-09-07 | implementer |
| 8 | `grep -rn -e 'refs/dispatch' -e 'dispatchRefNamespace' tools/desk plugins/assay/skills --include='*.go' --include='*.md' \| grep -v _test.go` | 0 | 2 hits, both in the sink's own file: the doc comment for the namespace constant and the constant itself, `const dispatchRefNamespace = deskkit.ClaimRefNamespace`. No other shipped file names the old namespace. The complementary read — the same grep for `refs/heads/dispatch` — returns hits across the tools tree and the four skill files, every one naming the same namespace as that constant | 2026-09-07 | implementer |
| 9 | `go test ./internal/deskkit/ -run TestNoForgeCLIShellout -count=1` then `-run TestForgeNoPassthrough -count=1` | 0, 0 | both `ok` — no shell-out and no arbitrary-endpoint method was introduced to serve the release; the release runs on the enumerated `DeleteRef` and nothing else | 2026-09-07 | implementer |
| 10 | `go test ./internal/forgeban/... -count=1` | 0 | `ok` — the permit register and its ratchet are untouched by this change, as expected: no forge-CLI call site was added or retired | 2026-09-07 | implementer |
| 11 | `statusgen --root <this worktree> --consumers --brief forge-neutral/05` | 2 | **COULD-NOT-CHECK.** The tool exits before reaching this brief on a pre-existing condition in an unrelated register: `--consumers: docs/streams/decisions/README.md: no frontmatter: first line must be ---`. That file is untouched by this change (last written by an earlier brief) and the condition is a property of the tree plus this locally installed statusgen (v0.27.0), not of this diff. The routing claims were instead reconciled against the diff BY HAND, and the two that were wrong were corrected in the frontmatter above, each with its correction stated inline. CI's own pinned statusgen is the authority | 2026-09-07 | implementer |
| 12 | the mutation described in Verify row 12, then row 6, then restore and re-run row 6 | 1, then 0 | On the mutant, 3 subtests of the row-6 test fail with `a release that did not happen was reported as success — the slot leaks silently`; after restoring, row 6 is `--- PASS` again. Recorded in full under Fail-first below | 2026-09-07 | implementer |

### Fail-first

Each new test was run against the pre-brief behaviour first, by mutating the implementation back
and observing the specific failure, then restoring:

| Row | Mutation | Observed failure |
|---|---|---|
| 3 | claim namespace set back to its own namespace under `refs/` | 3 of the 6 subtests fail, all on the GitLab side: `DeleteRef(...): could-not-check: GitLab exposes no general ref-delete endpoint ... — this backend cannot release a claim, which means a claim taken on it is a slot lost`. The GitHub subtests still pass, which is the whole asymmetry this brief exists to close |
| 4 | the claim-key path-separator guard removed | `ClaimRefPath("example/example-stream/05") produced "dispatch/example/example-stream/05" — a key carrying a path separator must be refused`, plus the key parser wrongly claiming a key from a ref one level deeper |
| 5 | the sink's default reverted to the printing sink | `the production sink is main.dryRunSink, not the releasing forgeDispatchSink — the claim would never be released in production` |
| 6 | the release line moved ahead of the delete and the delete error swallowed | 3 subtests fail with `a release that did not happen was reported as success — the slot leaks silently` |
| 7 | the reader's prefix replaced by a literal of the old namespace | `the reader lists "refs/dispatch/" but the claim namespace is "refs/heads/dispatch/" — a reader pointed at the wrong namespace reports every slot free` |

### What an implementer cannot attest

- **Row 11 is could-not-check**, for the reason in its row. A verifier on merged main, or CI,
  settles it.
- **Live-read row L10 in the decision record is could-not-check**: no GitLab custody file exists
  on the implementing machine, so whether a ref outside the branch and tag namespaces can be
  PUSHED to a GitLab project was not probed. §2 of that record states why the decision does not
  turn on it — the option that question would have rescued has no delete route either way, and
  that absence WAS measured live.
- **The GitLab DELETE verb was not exercised live in this session** (it needs a write credential).
  What is live here is the read half on 2026-09-07 — the general ref route is absent, the Branches
  route is present and accepts the encoded separator — plus the authenticated create-and-delete at
  `HTTP 201` / `HTTP 204` recorded on the 2026-09-02 pilot run.
### Verify run — 2026-09-10, non-implementer dispatched verifier (opus-4.8[1m]-verifier, local)

Target: merged `origin/main` @ `48b978bb08c468fec52c015d280285698fc362bd` (two-protocol confirmed). Offline (`KUBECONFIG=/dev/null`) in an isolated worktree; runner ≠ implementer (merge `036c4025`). gate: model, risk all=no.

Design chose option (b): the claim moved into a reserved branch namespace served by the Branches API — `ClaimRefNamespace = "heads/dispatch"` → fully-qualified `refs/heads/dispatch/<key>`; writer and reader both derive from that one constant.

| # | Command | Exit | Key observed output | Result |
|---|---------|------|---------------------|--------|
| 1 | `go build ./... && go test ./...` | 0 | every package `ok`, no FAIL | PASS |
| 2 | `grep -c '^[|] ' docs/streams/forge-neutral/claim-shape.md` | 0 | `22` (≥4): live-read + decision tables present | PASS |
| 3 | `TestClaimRefRoundTripBothBackends` | 0 | create/list/delete both backends, same scenario names | PASS |
| 4 | `TestRefPathStillRejectsAPIPaths` (negative) | 0 | 22 refused shapes (API-path traversal, abs/scheme-rel URL, bare-unnamespaced, separator, newline, option-shaped); guard not widened | PASS |
| 5 | `TestSinkResolvesForge` (fanoutloop) | 0 | default sink built from the resolver; nil resolver refused at construction | PASS |
| 6 | `TestReleaseFailureIsNotReportedReleased` (negative, crux) | 0 | 5 subtests (forge refuses / cred refused / backend can't serve ns / resolver refuses / resolver yields nothing) each non-nil, name the key, emit no release line | PASS |
| 7 | `TestClaimReaderNamespaceMatchesWriter` (loopengine) | 0 | reader prefix = `deskkit.ClaimRefsPrefix`, derived from the same constant as the writer | PASS |
| 8 | `grep -rn -e refs/dispatch -e dispatchRefNamespace tools/desk plugins/assay/skills` (list review) | 0 | this brief's namespace agrees everywhere (see scope note); the `refs/dispatch/<id>` hits belong to a distinct, documented second primitive | PASS |
| 9 | `TestNoForgeCLIShellout` + `TestForgeNoPassthrough` | 0 | both `ok` — no shell-out / arbitrary-endpoint method added for the delete | PASS |
| 10 | `go test ./internal/forgeban/...` | 0 | `ok`; ratchet unchanged (`allowedInvocationCeiling = 9` @ allowlist.go:64 — not this brief's to move) | PASS |
| 11 | `statusgen --root . --consumers --brief forge-neutral/05` | 2 | COULD-NOT-CHECK — local statusgen v1.0.6 brief-v2 gap (`no brief-v1 file`); consumers hand-corroborated against the merge diff (below) | COULD-NOT-CHECK |
| 12 | MUTATION → row 6 → restore → row 6 | 1, then 0 | mutant (release line before `DeleteRef` + delete error swallowed): 3 subtests FAIL `a release that did not happen was reported as success — the slot leaks silently`; restored → `ok`; worktree clean | PASS |

**Risk-bearing value (ENUMERATE → RANK → DERIVE):**
- `RISK-VALUE: DERIVED — ClaimRefNamespace = "heads/dispatch" @ tools/desk/internal/deskkit/claimref.go:53.` RANK #1 (a mismatch = silent double-dispatch / permanently-leaked slot, invisible to any single-component test). Single-sourced: writer `dispatchRefNamespace = deskkit.ClaimRefNamespace` @ `cmd/fanoutloop/land.go:154`; reader `claimRefPrefix = deskkit.ClaimRefsPrefix` @ `internal/loopengine/writescope_io.go:31`, where `ClaimRefsPrefix = "refs/" + ClaimRefNamespace + "/"` @ `claimref.go:57`. Reader and writer share the one constant — proven by rows 7 (derivation) + 8 (grep list).
- `RISK-VALUE: DERIVED — release/list status codes: create 201 / delete 204; re-release no-op keys on 404/422 (refAlreadyGone @ land.go:138-140) so re-releasing a missing ref is not a false failure.` Standard Branches-API semantics, recorded in claim-shape.md.

**Defense-in-depth (gate: model):** CONFIRMED. Row 12's mutant (release line moved before `DeleteRef` + delete error swallowed with `return nil`) reddens row 6 — exactly 3 subtests fail with the precise sentinel; restoring returns row 6 green. Row 6 is a live control, sensitive to the one property (loud failed-release) the claim layer rests on.

**Scope-traceability (row 8):** every mention of THIS brief's claim namespace agrees — writer (`land.go`), reader (`tools/desk/internal/loopengine/writescope_io.go`), `forge_gitlab.go:1830`, `tools/desk/cmd/desksupervise/live.go`, and all four skill files name `refs/heads/dispatch`. Merged main carries a SECOND, separately-namespaced claim primitive — the pure-Go `deskclaim-ref`/`deskdispatch` (`refs/dispatch/<id>`, keyed by dispatch id, from later briefs), explicitly documented as distinct (`cmd/deskclaim-ref/main.go:58`); worker-desk SKILL.md:112 tells operators to run BOTH reads. This is a documented two-primitive split, NOT writer/reader drift within brief-05's namespace — row 8's intent holds. The brief's merge touched exactly the declared consumers (hand-corroborating row 11's could-not-check). No work maps to no Verify row.

**VERDICT: PASS** — rows 1–10 + mutation 12 PASS; row 11 COULD-NOT-CHECK (local statusgen brief-v2 gap, settled by hand-corroboration against the merge diff). gate: model, risk all=no → flip-eligible.

## Review
Gate: **model** (from frontmatter; all four risk answers are `no` — this changes a
mutual-exclusion namespace, not a credential, a permission or an irreversible external
write). Reviewer records verdict + date in the stream README table, and confirms that
`claim-shape.md`'s live-read table cites endpoints and status codes rather than documentation.
