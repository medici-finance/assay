---
brief: forge-neutral/05
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
schema: brief-v1
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
  - "plugins/assay/skills/worker-desk/SKILL.md, plugins/assay/skills/pr-shepherd/SKILL.md: follow-up forge-neutral/10 (both name `git ls-remote origin 'refs/dispatch/*'` in their text; the skill wording changes only once the round trip proves the shape)"
  - "the consumer repo's dispatch-claim script: out-of-scope (it lives in the repo being worked, not here; this brief fixes the CONTRACT it is invoked under and the release path it depends on)"
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
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | exit 0 |
| 2 | `grep -c '^[\|] ' docs/streams/forge-neutral/claim-shape.md` | ≥ 4 — the live-read table from task 1 and the decision table from task 2 are present, each row citing an endpoint and a status code |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestClaimRefRoundTripBothBackends -count=1 -v` | exit 0 — create, list and delete the chosen namespace against both backends' recorded fixtures under the same scenario names |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestRefPathStillRejectsAPIPaths -count=1 -v` | **negative path**: an argument shaped like an API path, an absolute URL, or a bare unnamespaced component is still refused by `ValidateRefPath`; widening the namespace did not widen the guard |
| 5 | `cd tools/desk && go test ./cmd/fanoutloop/... -run TestSinkResolvesForge -count=1 -v` | exit 0 — the sink is constructed from the resolver; the test fails if the production path can still yield a nil forge |
| 6 | `cd tools/desk && go test ./cmd/fanoutloop/... -run TestReleaseFailureIsNotReportedReleased -count=1 -v` | **negative path**: when the delete fails, the sink reports the claim as NOT released (non-zero, message naming the key) and does not print or record a release — a leaked slot must be loud |
| 7 | `cd tools/desk && go test ./internal/loopengine/... -run TestClaimReaderNamespaceMatchesWriter -count=1 -v` | exit 0 — the reader's namespace is derived from the same constant the writer uses, so the two cannot drift |
| 8 | `grep -rn -e 'refs/dispatch' -e 'dispatchRefNamespace' tools/desk plugins/assay/skills --include='*.go' --include='*.md' \| grep -v _test.go` | every hit names the SAME namespace as the constant in `tools/desk/cmd/fanoutloop/land.go` — reviewed as a list, not a count, since the skills' prose and the code must agree |
| 9 | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -count=1 -v && go test ./internal/deskkit/ -run TestForgeNoPassthrough -count=1 -v` | exit 0 — no shell-out and no arbitrary-endpoint method was reintroduced to serve the delete |
| 10 | `cd tools/desk && go test ./internal/forgeban/... -count=1` | exit 0 — the ratchet is unchanged by this brief (it retires no permit row); a change here would mean a shell-out was added or removed unnoticed |
| 11 | `statusgen --root . --consumers --brief forge-neutral/05` | exit 0 — every `consumers:` routing claim is corroborated against this branch's own diff |

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
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **model** (from frontmatter; all four risk answers are `no` — this changes a
mutual-exclusion namespace, not a credential, a permission or an irreversible external
write). Reviewer records verdict + date in the stream README table, and confirms that
`claim-shape.md`'s live-read table cites endpoints and status codes rather than documentation.
