---
brief: assay:assay:forge-neutral:15
title: desklabel — a role-keyed label verb
why: >-
  Every label-carrying write in the fleet today is either bundled into a bigger verb's own
  fixed set (deskflip's `approval-needed` swap, deskclose's `superseded?` proposal) or reached
  by a raw, unscoped API call — nothing lets a role clear or correct ONE label on its own,
  and #992's own series names the resulting gap: a stale disposition-proposal label a
  desk role could only have cleared by reaching the forge directly, unscoped, because no
  verb existed to do it safely. `desklabel` is that verb: it applies the design principle
  the whole series shares — every desk action a human does through the forge CLI today gets
  a desk verb with a forge-neutral mapping — to the one write nobody had scoped: setting or
  clearing a label a role is entitled to touch, and refusing the ones it is not.
wave: 2
depends: ["forge-neutral/01"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [992]
schema: brief-v2
authored: 2026-09-13 by forge-neutral authoring session
sources:
  - "#992 — the tracking issue for this five-brief series; its own series comment states the
    design principle this brief quotes verbatim, and names `desklabel add|rm` as brief 15:
    role-keyed label verb, any role sets/clears the shared escalation vocabulary
    (question/help wanted/needs-decision) and the disposition markers its own role owns;
    anything else refused exit 5"
  - "docs/streams/forge-neutral/brief-01-forge-resolution-contract.md — ForgeFor, the custody
    binding and the refusal contract this verb's forge reach is built on"
  - "docs/streams/forge-neutral/brief-13-write-verbs-c-deskpr-deskfile-deskclose.md — the
    session-role identity precedent (`deskkit.SessionTokenRole`, no ambient fallback, no
    `--as <role>` flag because a flag is a claim and the session's minted role is a fact) this
    brief's caller resolution reuses verbatim"
  - "tools/desk/cmd/deskclose/superseded.go:13-48 — the two-role superseded lane this brief's
    Evidence gap sits inside: `worker PROPOSES — label \"superseded?\"`, `reviewer CONFIRMS or
    DISPUTES`, and `resolveCaller`'s role-from-session pattern (`:99-137`) this brief's identity
    check follows"
  - "tools/desk/cmd/deskclose/superseded.go:400-438 (confirm), 444-512 (disputeProposal) —
    freshness-read: a dispute posts the verdict and applies `needs-decision`, and neither
    `confirm` nor `disputeProposal` ever removes `labelProposed` (`superseded?`) — the label a
    worker applied stands, unaltered, on an item a dispute has already ruled stale. Clearing it
    today has no verb; only a raw, unscoped label write reaches it"
  - "tools/desk/internal/deskkit/disposition.go:47-95 — the `DispositionVerdict` closed
    vocabulary (SUPERSEDED / RESOLVED-ELSEWHERE / NEEDS-REBASE) and its `Label()` rendering
    (`disposition:<verdict>`), described at `:18` as \"what a sweep can filter on\";
    `tools/desk/cmd/deskdisposition/verbs.go`'s `cmdSet` doc: \"records a WORKER's finding\""
  - "tools/desk/cmd/deskclose/github.go:27 — `decisionLabels = []string{\"needs-decision\",
    \"human-decided\"}\", the human decision-queue pair `refuseDecisionItem` reads"
  - "tools/desk/cmd/deskflip/flip.go:64-65 — `labelBeforeFlip = \"authorization-needed\"`,
    `labelAfterFlip = \"approval-needed\"`, the reviewer-run ready-flip's queue-state pair"
  - "tools/desk/cmd/deskdispatch/dispatch.go:39,347-354 — `queueLabelAuthorizationNeeded` is
    applied \"under the reviewer role's own credential ... never the deskpost verdict-write
    path\" even though the dispatcher's own session runs it — the label's OWNING role and the
    session that happens to invoke the write are different things, which this brief's
    ownership model has to key on the former"
  - "tools/desk/internal/deskkit/forge.go:361-376,854-858 — `LabelChange`/`LabelOutcome` and the
    `ApplyLabels` doc comment: \"reconciles a change's labels\" (its scope is explicitly the
    CHANGE, not the issue)"
  - "tools/desk/internal/deskkit/forge_github.go:1157-1230 — GitHub's `ApplyLabels`, wired to
    `/repos/{o}/{r}/issues/{n}/labels` — GitHub's issues and PR share one number sequence and
    one labels endpoint, so this call already serves both kinds with no branch"
  - "tools/desk/internal/deskkit/forge_gitlab.go:2483-2574 — GitLab's `ApplyLabels`, wired ONLY
    to `PUT /projects/:id/merge_requests/:iid` (`add_labels`/`remove_labels`) — issues are a
    SEPARATE IID sequence and endpoint on GitLab, and nothing on the seam reconciles an issue's
    labels today"
  - "gitlab.com/gitlab-org/api/client-go@v1.46.0 issues.go:429-440 — `UpdateIssueOptions`
    carries the same `add_labels`/`remove_labels` fields `UpdateMergeRequestOptions` does
    (`PUT /projects/:id/issues/:iid`), confirming the GitLab issue-label mapping this brief
    specifies is a real, already-vendored surface, not a gap needing a client upgrade"
  - "docs/streams/forge-gitlab/inventory.md row 2 (`GetIssue`) — the existing ambiguity
    precedent this brief's target-kind resolution reuses: GitLab probes BOTH
    `/projects/:id/issues/:iid` and `.../merge_requests/:iid` and refuses a both-resolve,
    because the two are separate IID sequences and a bare number is ambiguous without it"
  - "freshness-checked 2026-09-13 @ cc4f9e22 (origin/main) — the `Forge` interface carries 37
    methods (`awk` count over `forge.go`'s interface block); `allowedInvocationCeiling = 7`
    (`tools/desk/internal/forgeban/allowlist.go:72`) — desklabel is a NEW verb with no prior
    `gh` call site, so this brief retires no permit row and does not move the ratchet;
    `grep -rn '\"human-decided\"' tools/desk --include='*.go' | grep -v _test.go` finds only
    `decisionLabels` in `tools/desk/cmd/deskclose/github.go` — no verb ever APPLIES `human-decided`, it is
    read-only everywhere in the tree today"
exec-tier: strong
exec-tier-why: "the vocabulary table this verb enforces is a security boundary disguised as a
  small lookup: a role-ownership check that is too permissive (a typo in the table, a role
  string compared case-sensitively against a case-insensitive forge label, a wrong role
  resolved from the session) grants a role the ability to forge another role's marker — the
  identical class of failure the two-role superseded lane exists to prevent — and every
  happy-path test still passes, because the happy path never exercises the refusal (question a)."
domain: complicated
consumers:
  - "tools/desk/cmd/desklabel: follow-up forge-neutral/15 (new verb — `add`/`rm`, the
    vocabulary table, the session-role caller resolution, the GitHub/GitLab target-kind
    dispatch — this PR is doc-only and does not add this path)"
  - "tools/desk/internal/deskkit/forge.go, forge_github.go, forge_gitlab.go: follow-up
    forge-neutral/15 (one op to add — `ApplyIssueLabels` — both backends, consumed by
    desklabel in the implementation change; `ApplyLabels` itself stays UNCHANGED, no
    signature edit, no existing caller touched — this PR is doc-only and does not edit these
    files)"
  - "docs/streams/forge-gitlab/inventory.md: follow-up forge-neutral/15 (row 38 — this PR is
    doc-only and does not edit this file)"
  - "tools/desk/README.md: follow-up forge-neutral/15 (desklabel's row in the Tool reference
    table — this repo's desk-verbs index — this PR is doc-only and does not edit this file)"
  - "tools/desk/internal/forgeban/allowlist.go: out-of-scope (no permit row exists for a
    verb that did not exist before this brief; the ratchet does not move)"
  - "tools/desk/cmd/deskclose, deskdisposition, deskflip, deskdispatch: out-of-scope (each
    keeps applying its OWN fixed labels exactly as it does today through its own already-wired
    forge call; desklabel is an ADDITIONAL, narrower verb for a role acting on a label outside
    those fixed flows — e.g. clearing a stale `superseded?` after a dispute — not a replacement
    for any of their internal label writes)"
version: 2
id: 6f2c9a7e-3b1d-4e5a-9c2f-8a7d5e1b4c93
---

# Brief 15 — `desklabel`: a role-keyed label verb

## Context

**Design principle (quoted verbatim from #992, binding on all five briefs in this series):**
*"every desk action a human does through the forge CLI today gets a desk verb with a
forge-neutral mapping; the human-only ones stay human-only because SERVER-SIDE permissions
make them so, not because the desk lacks a verb."*

A human with forge write access can apply or remove any label on any issue or PR with one
`gh issue edit --add-label`/`--remove-label` call (or the GitLab equivalent). Every desk role
that reaches the forge only through the enumerated verbs has no such general write: each
verb that touches a label today bundles it into a bigger, fixed operation —
`deskflip`'s `authorization-needed`→`approval-needed` swap, `deskclose superseded`'s
`superseded?` proposal, `deskdisposition set`'s `disposition:*` record. None of them is a
general "set or clear one label" verb, and that is deliberate: a general label write is a
general forgery surface, and the whole point of routing writes through the seam is that a
session gets exactly the writes its role earns, never an unscoped one.

The gap this leaves is real, not hypothetical, and tracing the dispute path already shipped
in this repo's own `deskclose` surfaces it directly. `deskclose superseded --dispute` posts the verdict comment and applies
`needs-decision`, which puts the item on the human decision queue — but it never removes the
`superseded?` label the worker's `propose` step applied (`superseded.go:400-438` `confirm`,
`:444-512` `disputeProposal`: neither writes to `labelProposed`). An item a
reviewer has just ruled is NOT settled by the named target still carries the exact label a
sweep filters queue rows on as "awaiting a reviewer's verdict" — a stale disposition-proposal
label with no verb able to clear it safely, because no verb ever needed to touch that one
label in isolation before. Before this brief, correcting it took a raw, unscoped label write
— exactly the boundary-free path the forge-neutral stream exists to close off
(`identity.md`'s point 1: *"any forge the fleet runs on must be reachable THROUGH the verbs,
and the verbs must refuse rather than fall back to a raw call"*).

files:
- `tools/desk/cmd/desklabel/` (new) — `main.go`, `verbs.go` (`add`/`rm`), `vocabulary.go` (the role-ownership table), `forge.go` (session-role custody wiring, the deskclose/deskfile precedent).
- `tools/desk/internal/deskkit/forge.go` — `ApplyIssueLabels`, the one op this brief adds.
- `tools/desk/internal/deskkit/forge_github.go`, `tools/desk/internal/deskkit/forge_gitlab.go` — both backends implement it.
- `docs/streams/forge-gitlab/inventory.md` — row 38.
- `tools/desk/README.md` — the Tool reference table, `desklabel`'s row.

**Why the risk answers are all `no`.** `desklabel` mints no new custody path: it reuses `ForgeFor`/`SetGitHubCustodyMinter`
exactly as `deskclose` does (brief 13's already-human-gated
precedent), so no credential decision is made here. Every write is a label — idempotent,
reversible, and bounded to a closed, reviewed-in-this-PR vocabulary; there is no close, no
merge, no push, and no path to a "human-decided" forgery (see the vocabulary's explicit
exclusion below). The worst failure this verb can commit is a wrong label on an issue or PR,
which the next correct call — by any entitled role — reverses.

single-point-of-failure: the vocabulary table (which label belongs to which role, and which
labels belong to no role at all) is the one control standing between "a role writes a label"
and "a role forges another role's marker." Two independent layers stand behind it: `desklabel`
itself refuses (exit 5) on any label not in the table or owned by a different role BEFORE any
forge call is made (Verify rows 6-7), and the table is a small, reviewed, closed Go literal
with no runtime mutation path (no flag, no config file, no env var extends it) — a defect here
is a code-review question over a handful of lines, not a live surface an operator's
misconfiguration could widen.

facts:
- The `Forge` interface carries 37 enumerated operations
  (`tools/desk/internal/deskkit/forge.go`); it is FROZEN — an addition requires a consuming
  tool in the same change (`forge.go:10-11` states the rule; brief 13 amended it under the
  same precedent for four ops).
- `ApplyLabels(repo, number, change)` already exists and is explicitly scoped to "a change's
  labels" (`forge.go:854-858`). GitHub's implementation reaches
  `/repos/{o}/{r}/issues/{n}/labels`, which GitHub serves identically for issues and PRs
  (one number sequence, one endpoint) — so on GitHub, `ApplyLabels` already works on a plain
  issue with no code change. GitLab's implementation reaches ONLY
  `PUT /projects/:id/merge_requests/:iid` (`forge_gitlab.go:2483-2574`) — GitLab issues and
  merge requests are separate resources with separate IID sequences and separate endpoints, so
  today nothing on the seam can reconcile an ISSUE's labels on GitLab. `desklabel` is the
  first consumer that needs to.
- The GitLab client already vendors the issue-side mapping this brief specifies:
  `UpdateIssueOptions` (`gitlab.com/gitlab-org/api/client-go@v1.46.0` `issues.go:429-440`)
  carries the identical `add_labels`/`remove_labels` fields `UpdateMergeRequestOptions` does.
  No client upgrade, no new dependency — the mapping is a new call site, not a new capability.
- `GetIssue` (op 2) already resolves this exact ambiguity for READS: on GitLab it probes BOTH
  `/projects/:id/issues/:iid` and `.../merge_requests/:iid` and refuses a both-resolve
  (`inventory.md` row 2). `desklabel` reuses this read to learn which kind a number
  addresses — `IsPullRequest` on the returned `Issue` — rather than adding a second,
  parallel kind-resolution mechanism.
- The session-role identity model this brief's caller check reuses is already live in
  `deskclose`: `deskkit.SessionTokenRole(verb)` resolves the App role from `DESK_LOOP` with NO
  worker default and NO `--as <role>` override (`deskclose/forge.go:56-67`; the
  `superseded.go:38` header states the reasoning: *"A flag saying '--as reviewer' would be
  a claim; the session's minted role is a fact"*). `desklabel` takes the identical stance: the
  role that decides which labels a call may touch is read from the session, never supplied.
- The shared escalation vocabulary and its labels are already load-bearing in code, just never
  behind a scoped write verb: `needs-decision` and `human-decided` are `decisionLabels`
  (`deskclose/github.go:27`) that make an item human-only-close everywhere in `deskclose`.
  `question` and `help wanted` carry no code-level enforcement today (they are a filing
  convention passed to `deskfile --label`, unscoped) — `desklabel` is the first verb to give
  the whole trio a closed, checked write path.
- Two labels this brief's vocabulary table deliberately does NOT include, and why:
  - **`human-decided` is refused for every role, with no owner at all.** It asserts that a
    human ruled on an item (`decisionLabels`' human-only-close pair). A desk role applying it
    would be exactly the forgery class `deskclose`'s design already guards against elsewhere in
    this tree (a label standing in for a recorded human act must never be a thing a role can
    self-apply) — so `desklabel` refuses it unconditionally, the same as any label with no
    table entry, and the vocabulary table says so explicitly rather than by omission.
  - **`ready-for-human` is not a label at all.** It is prose describing the PR's draft-state
    transition (`MarkReadyForReview`, op 11) — `grep -rn '"ready-for-human"' tools/desk
    --include='*.go'` (excluding tests) returns nothing. `desklabel` is a labels-only verb; the
    draft-state flip stays `deskflip`'s alone.
- Role ownership, read from the code that already applies each marker (not assumed from the
  series comment, which names `superseded?` imprecisely — see the correction below):
  - **worker owns** `superseded?` and the `disposition:` label family
    (`disposition:superseded`, `disposition:resolved-elsewhere`, `disposition:needs-rebase`):
    both are worker-authored findings. `superseded.go:26` states plainly, *"worker PROPOSES —
    label `superseded?`"*; `tools/desk/cmd/deskdisposition/verbs.go`'s `cmdSet` doc calls its record "a
    WORKER's finding." **Correction to #992's series comment**, which describes `superseded?`
    as reviewer-owned: the code is unambiguous that the WORKER applies it (`propose`,
    `superseded.go:290-393`); the REVIEWER only confirms or disputes what a worker proposed,
    and never re-applies or removes the proposal marker itself (the Evidence gap this brief
    opens with). This brief follows the code, per its own instruction to read the actual
    vocabulary rather than the paraphrase.
  - **reviewer owns** `authorization-needed` and `approval-needed`: `deskflip`'s ready-flip
    queue-state pair (`flip.go:64-65`). `deskdispatch` also writes `authorization-needed`, but
    explicitly "under the reviewer role's own credential" (`dispatch.go:347-354`) — the
    dispatcher's own session is not the label's owner; the reviewer role is, regardless of
    which tool's session performs a given write of it.
  - **shared, no owner (any role may set or clear):** `question`, `help wanted`,
    `needs-decision` — the escalation vocabulary #992 names, matching `deskclose`'s existing
    `needs-decision` handling (only the human-only-CLOSE gate is absolute; the LABEL itself is
    not role-exclusive, because any role may need to escalate).
  - **everything else is refused, exit 5** — including every repo-specific label
    (`raised-by:*`, size/surface/priority families, etc.) that some OTHER verb provisions or
    reads at its own call site. `desklabel` adds no vocabulary entry for a label it did not
    verify a real owner for in this brief.
- Clearing `needs-decision` through `desklabel rm` removes only the LABEL. It does not close
  the item, does not touch `refuseDecisionItem`'s close gate (which reads the label fresh on
  its own next call), and does not overrule any human ruling recorded in the item's comments —
  a premature clear is a bookkeeping mistake any later sweep or human re-applies, not a bypass
  of anything irreversible. This is why the shared-vocabulary label answers are all "no
  role-check" rather than "human-only": the verb this brief specifies never closes, merges, or
  pushes anything.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Do not add a generic/passthrough method, a new `gh`/`glab` shell-out, or a caller-supplied
  forge name/flag. `.github/workflows/forge-surface-control.yml` must stay green unchanged.
- No `--as <role>` flag, no role argument of any kind: the acting role is read from the
  session (`deskkit.SessionTokenRole`), exactly as `deskclose` does. A flag that let a caller
  assert its own role would be the exact hole the vocabulary table exists to close.
- The vocabulary table (label → owning role, or "shared", or absent) is a closed Go literal.
  No flag, environment variable, or config file may extend it at runtime.

## Task
1. **Add one op: `ApplyIssueLabels(repo ForgeRepo, number int, change LabelChange)
   (*LabelOutcome, error)`.** Same input/output shapes `ApplyLabels` already uses — no new
   types. Doc comment states it is the ISSUE-scoped analog of `ApplyLabels`, needed because
   GitHub's issue/PR label endpoint already serves both kinds (so its GitHub implementation
   calls the SAME internal helper `ApplyLabels` uses, not a duplicate REST sequence — cite the
   shared function it delegates to), while GitLab's issues and merge requests are different
   resources with different endpoints and IID sequences (`PUT /projects/:id/issues/:iid`
   carrying `add_labels`/`remove_labels`, vs `ApplyLabels`'s `.../merge_requests/:iid`).
   `ApplyLabels` itself is UNCHANGED — no signature edit, no existing caller (`deskflip`,
   `deskpost`, `deskclose`) touched. Add the both-backend golden contract case and record the
   op as row 38 in `docs/streams/forge-gitlab/inventory.md`.
2. **Build the vocabulary table** (`tools/desk/cmd/desklabel/vocabulary.go` (planned)) exactly as
   specified in Context above: a shared set (`question`, `help wanted`, `needs-decision`), a
   per-role-owned set keyed by role name (`worker`: `superseded?` +
   `disposition:superseded`/`disposition:resolved-elsewhere`/`disposition:needs-rebase`;
   `reviewer`: `authorization-needed`, `approval-needed`), and an explicit refuse-always entry
   for `human-decided` (documented as refused for every role, not merely absent). A label
   matched case-insensitively against the table but written in its canonical case. Any label
   not appearing in the table at all is refused with the same message shape as an
   owned-by-another-role refusal, naming that the label has no desklabel vocabulary entry.
3. **Resolve the caller's role from the session**, following `deskclose`'s precedent exactly:
   `deskkit.SessionTokenRole("desklabel")`, no worker default, no override flag. Mint the
   session-role token via the same `desktoken <role> --repo <slug>` custody path
   (`SetGitHubCustodyMinter`); an unminted token is a hard refusal, never an ambient fallback.
4. **Implement `desklabel add <owner/repo> <issue-or-pr> <label>` and
   `desklabel rm <owner/repo> <issue-or-pr> <label>`.**
   - Check the label against the vocabulary table for the resolved role FIRST, before any
     forge read or write: not in the table, or owned by a different role → `deskkit.Refused`
     (exit 5) naming the label, the role that DOES own it (or "no role" for a table-absent or
     `human-decided` label), and the role the session resolved.
   - Only after the ownership check passes, call `GetIssue(repo, number)` to learn the target
     kind (`IsPullRequest`) — reusing op 2's existing ambiguity refusal rather than adding a
     second kind-resolution path — then call `ApplyLabels` (a change) or the new
     `ApplyIssueLabels` (a plain issue) with `Add: [{Name: label}]` (`add`) or
     `Remove: [label]` (`rm`). Removing an absent label or adding an already-present one is a
     no-op, inherited from `ApplyLabels`'/`ApplyIssueLabels`' own idempotency.
   - `--dry-run` validates the ownership check and the target-kind read, then stops before any
     write, printing what would happen — the `deskdisposition set --dry-run` shape.
5. **Add `desklabel`'s row to `tools/desk/README.md`'s Tool reference table** (this repo's
   desk-verbs index): verb(s) `add`, `rm`; class `outward write`; write budget/breaker `yes`
   (the same meter every other outward-write verb rides).

## Verify (executable — no prose-only DoD items)

Planned test deliverables: `TestDesklabelRefusesUnownedLabel` (planned),
`TestDesklabelAppliesOwnedLabelGitHub` (planned),
`TestDesklabelAppliesOwnedLabelGitLab` (planned),
`TestDesklabelSharedVocabularyAnyRole` (planned),
`TestDesklabelRefusesHumanDecidedForEveryRole` (planned),
`TestApplyIssueLabelsBothBackends` (planned).

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/desk && go build ./... && go test ./...` | exit 0 |
| 2 | check:ci | `cd tools/desk && go test ./cmd/desklabel/... -count=1` | exit 0 — the new verb's suite is green |
| 3 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -count=1 && go test ./internal/deskkit/ -run TestForgeNoPassthrough -count=1` | exit 0 — the seam grows one op and stays closed |
| 4 | check:ci +dereference | `cd tools/desk && go test ./internal/deskkit/ -run TestApplyIssueLabelsBothBackends -count=1 -v` | exit 0 — GitHub's `apply_issue_labels` golden case pins the shared-helper wire (identical request shape to `ApplyLabels`); GitLab's pins `PUT /projects/:id/issues/:iid` with `add_labels`/`remove_labels`, distinct from the MR path |
| 5 | check:ci +flow | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelAppliesOwnedLabelGitHub -count=1 -v && go test ./cmd/desklabel/... -run TestDesklabelAppliesOwnedLabelGitLab -count=1 -v` | exit 0 — POSITIVE, both forges: a role applies/clears a label it owns end to end (session role → vocabulary check → `GetIssue` kind resolution → the correct one of `ApplyLabels`/`ApplyIssueLabels`) |
| 6 | check:ci | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelRefusesUnownedLabel -count=1 -v` | **negative path**: a worker-role session calling `desklabel add … approval-needed` (reviewer-owned) REFUSES (exit 5) before any forge call — asserted on a recording fake forge showing zero calls; the error names both roles |
| 7 | check:ci | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelRefusesHumanDecidedForEveryRole -count=1 -v` | **negative path**: EVERY resolved role (worker, reviewer, and any other role the roster's loop→role map carries) is refused (exit 5) on `human-decided`, zero forge calls in every case |
| 8 | check:ci | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelSharedVocabularyAnyRole -count=1 -v` | exit 0 — `question`/`help wanted`/`needs-decision` succeed under BOTH a worker-role and a reviewer-role session (no role-check fires on the shared set) |
| 9 | check | `test -d tools/desk/cmd/desklabel && { grep -rn -e 'flag.String("as"' -e '"--as"' tools/desk/cmd/desklabel --include='*.go' \|\| [ $? -eq 1 ]; } \| { grep -v _test.go \|\| [ $? -eq 1 ]; } \| wc -l` | output is `0` — no role-override flag exists; the acting role is read from the session only. Re-written 2026-10-03 (#1862): every grep stage tolerates only the no-match status, so a missing path or a grep error fails the row instead of passing it. The `test -d` leg covers BSD grep, which stays silent on an absent directory under `--include`. |
| 10 | check:ci | `cd tools/desk && go test ./internal/forgeban/... -count=1` | exit 0 — the ratchet is unaffected (desklabel is a new verb; no permit row to retire) |
| 11 | check:ci +dereference | `statusgen --root . --consumers --brief forge-neutral/15` | exit 0 — every `consumers:` routing claim is corroborated against this branch's own diff |
| 12 | check | `grep -c 'desklabel' tools/desk/README.md` | prints a value ≥ 1 — the docs-index row (this repo's desk-verbs index) exists |
| 13 | check +mutation | **Mutation demonstration for the role-ownership guard.** In the vocabulary lookup (the function `desklabel` calls before any forge read/write), force it to always report "owned by the caller's own role" — e.g. change its return to `return true, callerRole` unconditionally — then `cd tools/desk && go test ./cmd/desklabel/... -count=1`; restore the file and re-run | exit **1** on the mutant: `TestDesklabelRefusesUnownedLabel` reddens (a worker-role session can now write a reviewer-owned label), exit **0** again after restoring. Proves the ownership check is a live control, not a decoration nothing exercises |

## Pre-mortem → detection map

| Failure mode of the work | Caught by |
|---|---|
| The vocabulary table is too permissive — a label lands in the shared set that should be role-owned, or vice versa | row 6 (unowned-label refusal) + row 8 (shared-set positive) exercise both directions; the table itself is reviewed in this PR's diff, not runtime-configurable (row 9's absent-flag check) |
| `human-decided` becomes settable by SOME role via a table entry a later edit adds | row 7 asserts the refusal holds for every resolved role, not just one |
| A `--as <role>` flag creeps in "for testing", letting a session assert its own role | row 9 |
| `ApplyLabels`'s existing callers (`deskflip`, `deskpost`, `deskclose`) break because the op's signature changed | row 1 (whole-module build+test) — this brief adds an op, it does not touch `ApplyLabels`'s signature |
| `ApplyIssueLabels` on GitHub duplicates REST logic instead of sharing `ApplyLabels`'s helper, and the two silently drift | row 4's golden case pins both to the identical request shape |
| The GitLab issue-vs-MR distinction is dropped and `desklabel` writes an MR endpoint for what is actually a plain issue (or vice versa) | row 5's GitLab case exercises the real kind-resolution → dispatch path end to end |
| The ownership check is present in code but never actually reached before a forge call (e.g., checked after the write) | row 13's mutation row — a check that ran too late or not at all reddens no test on its own; forcing it to always pass and watching row 6 flip to green-when-it-should-be-red is what proves it runs, and runs FIRST |
| The docs index is never updated, so an operator scanning `tools/desk/README.md` for available verbs never finds `desklabel` | row 12 |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->
### Non-implementer verifier run — 2026-09-17 sonnet-5-verifier (verify-desk dispatch) — **VERIFY: PARTIAL (12/13 PASS)** — HELD at `implemented`

Runner ≠ implementer. Own detached temp worktree off `medici-finance/assay` origin/main at `c67cc371f165a7b63e8b0a26d0a5afa40a95556b`. This brief's own file carries only its doc-only authoring commit (PR #998); the real implementation landed separately via PR #1180 with no Evidence ever appended — this is that missing non-implementer pass, verified against what actually shipped in `tools/desk/cmd/desklabel/`, not the (now partly stale) authoring prose.

| # | Command | Expect | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | `go build ./... && go test ./...` | exit 0 | exit 0 — ~75 packages ok, incl. `cmd/desklabel` and `internal/deskkit` | 2026-09-17 | sonnet-5-verifier |
| 2 | `go test ./cmd/desklabel/... -count=1` | exit 0 | exit 0 | 2026-09-17 | sonnet-5-verifier |
| 3 | `TestNoForgeCLIShellout` + `TestForgeNoPassthrough` | exit 0 | exit 0 both — ratchet ceiling 5, frozen surface 46 ops, all tabulated | 2026-09-17 | sonnet-5-verifier |
| 4 | `TestApplyIssueLabelsBothBackends -v` | exit 0 | exit 0 — both backends PASS (see scope deviation below) | 2026-09-17 | sonnet-5-verifier |
| 5 | `TestDesklabelAppliesOwnedLabelGitHub` + `...GitLab -v` | exit 0 | exit 0 both — GitHub + GitLab (issue-only, MR-only, both-resolve-refused) | 2026-09-17 | sonnet-5-verifier |
| 6 | `TestDesklabelRefusesUnownedLabel -v` | exit 5, zero forge calls, names both roles | exit 0 (test PASS) — 14 subtests, all refuse exit 5 with 0 calls via a panicking recording fake | 2026-09-17 | sonnet-5-verifier |
| 7 | `TestDesklabelRefusesHumanDecidedForEveryRole -v` | exit 5 for EVERY role | exit 0 (test PASS) — 20 subtests across 5 real roster roles × add/rm × case variants, all refused, all zero calls | 2026-09-17 | sonnet-5-verifier |
| 8 | `TestDesklabelSharedVocabularyAnyRole -v` | exit 0 | exit 0 — shared set is 4-wide (topology also carries `needs-human`), worker+reviewer both succeed | 2026-09-17 | sonnet-5-verifier |
| 9 | `grep -rn '"--as"' cmd/desklabel \| wc -l` | `0` | `0` | 2026-09-17 | sonnet-5-verifier |
| 10 | `go test ./internal/forgeban/... -count=1` | exit 0 | exit 0 | 2026-09-17 | sonnet-5-verifier |
| 11 | `statusgen --root . --consumers --brief forge-neutral/15` | exit 0, corroborated | **EXPLICITLY UNRUN (exit 2, could-not-check)** — structural: the brief's authoring PR (#998, doc-only) and its implementation PR (#1180) never share a single diff; traced both candidate bases, both refuse identically. Filed: `medici-finance/assay#1281` | 2026-09-17 | sonnet-5-verifier |
| 12 | `grep -c 'desklabel' tools/desk/README.md` | ≥1 | `1` | 2026-09-17 | sonnet-5-verifier |
| 13 | mutation: force the ownership check to always return true, re-run row 6, restore | mutant reddens row 6, restore green | **mutant exit 1** — `TestDesklabelRefusesUnownedLabel` reddened on 10/14 subtests (the role-mismatch cases; the 4 table-absent-pair cases correctly unaffected — separate code path); `TestDesklabelDryRunWritesNothing` also reddened. Restored, md5-verified byte-identical, green again. **Correction (pr-review-desk finding, PR #1282): count corrected from an earlier 8/14 to the reproduced 10/14** | 2026-09-17 | sonnet-5-verifier |

**Scope deviation (Task item 1 / row 4), not a defect.** The brief's Task specified a new op `ApplyIssueLabels`; the shipped code instead reuses the existing `ApplyLabels` with a `Target` field (`TargetIssue`/`TargetChange`, added by an earlier, unrelated PR #1095) — achieving the same functional outcome without growing the frozen `Forge` interface. Confirmed functionally equivalent: GitHub's `ApplyLabels` documents Target doesn't change its request (one endpoint serves both); GitLab's already switches `/issues/:iid` vs `/merge_requests/:iid` on `Target`. Superior design, but the brief's own `consumers:` claim (which names `ApplyIssueLabels` as the new op) is now factually stale — covered by #1281.

**RISK-VALUE: DERIVED — from `tools/desk/cmd/desklabel/vocabulary.go` source, not from the brief's prose:**
- `human-decided` refused for EVERY role, unconditionally — `Owner: ownerNone` (`:128-129`); `permits()`'s `case ownerNone: return false` (`:147-156`) has no role reference at all, so no code path can flip it. Confirmed live (not decorative) by row 7's 20-subtest sweep and independently by row 13's mutation, which left this branch untouched and still-refusing while the role-owned branch reddened — proving it's a genuinely separate, independent code path (matches the brief's "two independent layers" SPOF claim).
- `superseded?` + 3 `disposition:*` labels — `Owner: roleWorker` (`:112-119`).
- `authorization-needed`, `approval-needed` — `Owner: roleReviewer` (`:122-125`).
- Shared set (`question`, `needs-decision`, `needs-human`, `help wanted`) — derived live from `topology.Compiled()` (`:87-105`), not a hardcoded 3 as the brief's prose states — confirmed by row 8 observing 4, not 3.
- Case-insensitive match / canonical-case write: `lookup()` uses `strings.EqualFold` (`:134-142`); `authorize()` returns the table's canonical spelling (`:163-186`), which `verbs.go:142` uses for every subsequent read/write/audit line — confirmed functionally via row 7's `Human-Decided` mixed-case variant refusing identically to lowercase.

**VERIFY: PARTIAL** — 12/13 rows PASS, including all three security-critical rows (6, 7, 13) run with genuine rigor; row 13's mutation is the load-bearing proof and behaved exactly as predicted. Row 11 is EXPLICITLY UNRUN for a structural reason unrelated to the deliverable's correctness (filed #1281), not a defect. Held at `implemented` pending that row's resolution or an explicit waiver — the deliverable itself verifies clean.


### Verify pass 2026-09-22 (non-implementer, VERIFY: 12/13 PASS — row 11 could-not-check, tracked #1281)

Runner: `claude-opus-4-8[1m]` (non-implementer). Merged main `6204bb4f1eacc0229f2a86c8e0dce59edabdd22a`. Offline (`KUBECONFIG=/dev/null`).

| # | Command | Expect | Observed (exit + key line) | Date | Runner |
|---|---------|--------|----------------------------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | exit 0 | build 0; test 0 — ~75 pkgs ok incl cmd/desklabel, internal/deskkit, internal/forgeban | 2026-09-22 | opus-4.8-verifier |
| 2 | `go test ./cmd/desklabel/... -count=1` | exit 0 | 0 — `ok cmd/desklabel 0.389s` | 2026-09-22 | opus-4.8-verifier |
| 3 | `TestNoForgeCLIShellout` + `TestForgeNoPassthrough` | exit 0 | 0 both — seam stays closed | 2026-09-22 | opus-4.8-verifier |
| 4 | `TestApplyIssueLabelsBothBackends -v` | exit 0 | 0 — github + gitlab subtests PASS | 2026-09-22 | opus-4.8-verifier |
| 5 | `TestDesklabelAppliesOwnedLabelGitHub/GitLab -v` | exit 0 | 0 — GitLab issue-only/MR-only/both-refused-without-`--kind` PASS | 2026-09-22 | opus-4.8-verifier |
| 6 | `TestDesklabelRefusesUnownedLabel -v` | exit 5, 0 forge calls, names both roles | test PASS — refuses incl table-absent + size-family labels | 2026-09-22 | opus-4.8-verifier |
| 7 | `TestDesklabelRefusesHumanDecidedForEveryRole -v` | exit 5 every role | test PASS — worker/reviewer × add/rm × case variants all refuse | 2026-09-22 | opus-4.8-verifier |
| 8 | `TestDesklabelSharedVocabularyAnyRole -v` | exit 0 | 0 — shared set 4-wide (needs-decision, needs-human, question, help wanted); worker+reviewer succeed | 2026-09-22 | opus-4.8-verifier |
| 9 | `grep -rn 'flag.String("as"'/'"--as"' cmd/desklabel --include='*.go' \| grep -v _test \| wc -l` | `0` | `0` | 2026-09-22 | opus-4.8-verifier |
| 10 | `go test ./internal/forgeban/... -count=1` | exit 0 | 0 — ratchet unaffected | 2026-09-22 | opus-4.8-verifier |
| 11 | `statusgen --root . --consumers --brief forge-neutral/15` | exit 0, corroborated | **could-not-check (exit 2, explicitly unrun)** — "not in the diff against 6204bb4f"; structural: brief's doc-authoring & impl landed in separate PRs so it is never in the merged-main diff. Reported as itself, not rounded. Same condition as prior 2026-09-17 pass; tracked `#1281`. | 2026-09-22 | opus-4.8-verifier |
| 12 | `grep -c 'desklabel' tools/desk/README.md` | ≥1 | `1` — tool-reference row present | 2026-09-22 | opus-4.8-verifier |
| 13 | Mutation (repo `muhar` harness on cmd/desklabel/mutations.json): force ownership check caller-owned, re-run, restore | mutant reddens row 6, green after restore | **CAUGHT** — baseline GREEN, positive control CAUGHT; row-13 mutant CAUGHT; 11 caught / 0 not-caught / 0 could-not-mutate; vocabulary.go restored byte-identical (md5 a500b2c1…) | 2026-09-22 | opus-4.8-verifier |

Scope traceability: every Evidence row maps 1:1 to its Verify row; no invented scope.

RISK-VALUE: DERIVED — `human-decided` Owner=`ownerNone` @ `tools/desk/cmd/desklabel/vocabulary.go:128` — the human-only-close pair (deskclose decisionLabels {needs-decision, human-decided}); a role self-applying it forges a recorded human ruling, so NO role owns it (`permits()` ownerNone → false). Proven by row 7 (all roles refuse) + row-13 mutant CAUGHT.
RISK-VALUE: DERIVED — `superseded?` Owner=`roleWorker` @ `vocabulary.go:112`; `authorization-needed`/`approval-needed` Owner=`roleReviewer` @ `:122,124` — code-truth matching deskclose (worker proposes) and deskflip (reviewer ready-flip pair).
RISK-VALUE: DERIVED — shared set = {needs-decision, needs-human, question} + `help wanted` @ `vocabulary.go:92-105` — first three derived live from `topology.Compiled().DecisionOwedLabelNames()` (bound to topology.yaml by TestTopologyDriftRegistry); the brief prose naming it 3-wide is stale doc, not a code defect (set self-updates from loader).

**VERIFY: 12/13 PASS** — held pending flip. All three security-critical rows (6 refusal, 7 human-decided-refusal, 13 mutation-caught) PASS. Row 11 could-not-check is the structural `statusgen --consumers` merged-brief limitation tracked `#1281` — the identical accepted condition under which sibling briefs 03/05/06/08 landed `done` on this board. gate:model, risk all-no → advances implemented → verified.

### Verify pass 2026-09-27 (non-implementer, dispatched verifier) — execution witness

Runner: `claude-opus-5-5` (non-implementer, verify-desk dispatch). Merged main `b227b40768db08a0a91046899bc1877cf3c6d1ec`, own detached worktree off `refs/remotes/origin/main`. Offline (`KUBECONFIG=/dev/null`). Implementation under test: PR #1180 (squash `2d2966622`); brief authored doc-only in PR #998 (squash `74865cba6`).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/desklabel/... -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -count=1 && go test ./internal/deskkit/ -run TestForgeNoPassthrough -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestApplyIssueLabelsBothBackends -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelAppliesOwnedLabelGitHub -count=1 -v && go test ./cmd/desklabel/... -run TestDesklabelAppliesOwnedLabelGitLab -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelRefusesUnownedLabel -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelRefusesHumanDecidedForEveryRole -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelSharedVocabularyAnyRole -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 9 | `grep -rn -e 'flag.String("as"' -e '"--as"' tools/desk/cmd/desklabel --include='*.go' \| grep -v _test.go \| wc -l` | fail exit=1 | sha256:4eff2db4bada | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./internal/forgeban/... -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 11 | `statusgen --root . --consumers --brief forge-neutral/15` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 12 | `grep -c 'desklabel' tools/desk/README.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 13 | `desklabel` | fail exit=5 | sha256:893c5fa03f6f | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |

**How to read the witness table above.** The witness runs on a darwin host, so every `check:ci` row (1-8, 10, 11) is could-not-run by the tool's own design (network-off sandbox needs Linux `unshare --net`). Rows 9 and 13 show `fail`, and both are check-definition artifacts, not defects: row 9's pipeline prints `0` (the expected value) but the witness runs it under `pipefail`, and the first `grep` finding no match exits 1; row 13 is a prose mutation procedure, so the witness extracted the first backticked token (`desklabel`) and ran the bare binary, which refuses with exit 5 for missing arguments. Every row was also run by hand, with these results:

- Row 1: the local full-module run exited 1 three times, and none of the failures touched desklabel. On the host toolchain (go1.27.1), `internal/avatar` `TestGolden20px` failed its byte-for-byte PNG golden check; the same package passes on go1.25.0 and go1.26.7, so this is a toolchain encoder difference. On the CI-pinned go1.25.0 (`GOTOOLCHAIN=go1.25.0`, matching the workflows' `GO_VERSION` and go.mod), 84/85 packages were ok, and `internal/loopengine` `TestDrain` hit its 4-5 s engine-stop deadline twice. Host load average was about 26 at the time. That package passes in isolation (`-count=3` exit 0; exit 0 again on go1.27.1). The build step exited 0 every time, and `cmd/desklabel`, `internal/deskkit` and `internal/forgeban` were ok in every run. CI's `ci` workflow `build-test` job runs this row's exact `go build ./... && go test ./...` over this tree and completed `success` on this merged-main SHA. Row 1 is counted as PASS on that CI result, and the local failures are counted as host environment.
- Row 2: exit 0 — `ok cmd/desklabel`.
- Row 3: exit 0 for both tests (`TestNoForgeCLIShellout`, `TestForgeNoPassthrough`).
- Row 4: exit 0 — `TestApplyIssueLabelsBothBackends` github + gitlab PASS. The github case asserts that the issue-target and change-target requests are identical (one helper). The gitlab case routes the issue target to the issues endpoint.
- Row 5: exit 0 for both. GitLab subtests `issue_only → the issues endpoint`, `merge_request_only → the merge_requests endpoint` and `both_resolve → refused without --kind, routed with it` all PASS.
- Row 6: exit 0, 14/14 subtests PASS. Each role-mismatch case (worker on approval-needed/authorization-needed, reviewer on superseded?/disposition family, desk role on superseded?) and each table-absent case (`raised-by:worker`, `size:xl`) asserts exit 5, that both roles are named, and zero forge calls.
- Row 7: exit 0, 20/20 subtests PASS. That is 5 roster roles (desk, issue-loop, reviewer, verifier, worker) × add/rm × two case spellings (`human-decided`, `Human-Decided`).
- Row 8: exit 0, 16/16 subtests PASS. That is worker and reviewer × add/rm × the 4-wide shared set: `help wanted`, `needs-decision`, `needs-human`, `question`.
- Row 9: prints `0`, and the command exits 0 in a plain shell.
- Row 10: exit 0 — `ok internal/forgeban`.
- Row 11: the literal command against merged main exits 2 with COULD-NOT-CHECK (the brief is not in the diff against merged main), the same result as the earlier passes. statusgen v1.0.27's own guidance is to re-run a merged brief against the diff that made the claims. At the authoring squash (PR #998, `74865cba6`) with `--base` set to its parent, it exits 0: 4 corroborated, 0 disproved, 2 unchecked (the two `out-of-scope` entries, which the tool leaves to reviewer judgement). I checked those two against the implementation diff (PR #1180): it touches neither the forgeban allowlist nor deskclose, deskdisposition, deskflip or deskdispatch, so both claims hold. At the implementation squash the tool still reports could-not-check (exit 2), because the brief file is absent from that diff. Row 11 is counted as PASS through the tool-sanctioned merged-brief procedure. This resolves the structural hold tracked in assay#1281.
- Row 12: prints `1`.
- Row 13: I made the mutation by hand in the `permits()` default branch, replacing `return e.Owner == role` with `return true`. On the mutant, `go test ./cmd/desklabel/... -count=1` exited 1: `TestDesklabelRefusesUnownedLabel` reddened on 10/14 subtests (the role-mismatch cases; the 4 table-absent cases take the separate `lookup` path), and `TestDesklabelDryRunWritesNothing` also reddened. After a path-specific restore, the file's md5 matched the original (`a500b2c1…`) and the suite exited 0 again. **CAUGHT.**

Observations:
- The known scope deviation still holds. There is no `ApplyIssueLabels` op; desklabel reuses `ApplyLabels` with `LabelChange.Target`, and inventory row 38 is `GetIssueTyped`, which backs the `--kind` flag. `--kind` selects the target kind only. It is not a role input, so the no-role-flag rule (row 9) is unaffected. The role comes from `deskkit.SessionTokenRole` (`tools/desk/cmd/desklabel/verbs.go:31`), and `authorize()` runs before any forge read (`verbs.go:142`, ahead of `GetIssue`/`GetIssueTyped` at `:160-162` and `ApplyLabels` at `:208`).
- `statusgen --lint` raises a `risk-files-crossread` NOTICE on this brief: the prose declares `tools/desk/internal/deskkit/forge.go`, which falls under a security-path trigger, while every risk answer is `no`. PR #1180 never modified forge.go or either backend. Its only changes under the deskkit directory are a one-line `"desklabel": {}` audit-tool-key registration, a new label test file, and one test name added to the package's mutation-gate run list. The desklabel verb reaches the forge only through the existing `ApplyLabels`/`GetIssue` ops. The all-`no` risk answers therefore stand, and the NOTICE comes from the stale `ApplyIssueLabels` prose.
- Two local-suite instruments are environment-sensitive, and neither is related to this brief: the avatar golden test on toolchains newer than 1.26, and `loopengine` `TestDrain` under heavy host load. They are routed to the desk.

Risk-bearing value enumeration (the risk fields are all `no`, and this is recorded for completeness). Literals the implementation introduces, plus those named in Deliverables:
- `roleWorker = "worker"` @ `vocabulary.go:37`; `roleReviewer = "reviewer"` @ `:38`; `ownerShared = "shared"` @ `:48`; `ownerNone = "no role"` @ `:50`; `helpWantedLabel = "help wanted"` @ `:57`.
- Owned rows @ `vocabulary.go:112-129`: `superseded?`, `disposition:superseded`, `disposition:resolved-elsewhere`, `disposition:needs-rebase` → worker; `authorization-needed`, `approval-needed` → reviewer; `human-decided` → `ownerNone`.
- The shared set is derived from the root topology.yaml `labels.decision_owed` (`needs-decision`, `question`, `needs-human`), plus `help wanted`.
- `ExitRefused = 5` @ `tools/desk/internal/deskkit/exitcodes.go:27` is pre-existing and not changed by this item.

Ranked by irreversibility, none of these is irreversible. A wrong label is undone by the next entitled call, and a wrong table entry by an edit and a redeploy. The top-ranked entries are the ownership rows, because a mis-owned row lets a role forge another role's marker.

RISK-VALUE: DERIVED — `human-decided` Owner = `ownerNone` (`"no role"`) @ `tools/desk/cmd/desklabel/vocabulary.go:128` — deskclose's `decisionLabels` pair makes this label record a HUMAN ruling, so no role may self-apply it. `permits()` `case ownerNone: return false` (`:151-152`) has no role reference. Row 7 shows 20/20 refused, and the repo's own mutation list separately carries the ownerNone→true mutant.
RISK-VALUE: DERIVED — `superseded?` Owner = `roleWorker` (`"worker"`) @ `vocabulary.go:112`, and the three `disposition:*` rows @ `:114-118` — these match the code that applies them (deskclose superseded: the worker proposes and the reviewer only confirms or disputes; deskdisposition `set` records a worker's finding).
RISK-VALUE: DERIVED — `authorization-needed` / `approval-needed` Owner = `roleReviewer` (`"reviewer"`) @ `vocabulary.go:122,124` — deskflip's `labelBeforeFlip`/`labelAfterFlip` ready-flip pair. deskdispatch applies `authorization-needed` under the reviewer role's own credential, so the owner is the reviewer and not the invoking session.
RISK-VALUE: DERIVED — `helpWantedLabel = "help wanted"` @ `vocabulary.go:57` (shared) plus the topology-derived shared rows @ `:92-105` — these are the escalation vocabulary, which any role must be able to raise. Clearing one removes only the label, never closes the item, and never touches deskclose's human-only-close gate.

VERIFY: PASS — 13/13 rows PASS against merged main `b227b40768db`. Row 1 rests on CI's `build-test` success at this SHA, since the local full-suite failures came from host load and toolchain and not from desklabel. Row 11 passes through the tool-sanctioned merged-brief re-run against the authoring diff (exit 0, 4 corroborated / 0 disproved; the 2 out-of-scope entries were checked against the implementation diff and hold). Rows 6, 7 and 13, the security-critical refusal rows and the mutation proof, pass. gate:model and all risk answers are `no`.
### Verification — 2026-09-30 (assay-verifier-app[bot] @ b0088804294b (claude-opus-5-5) (on-behalf-of human:ian)) — 2026-09-30 claude-opus-5-5-verifier

Non-implementer verification on merged main b0088804294b8b68ad8d06f341e6f0fd9dd2637d, gate: model, all four risk answers no. First table: the `statusgen verifyrun` execution witness, landed verbatim; it ran on Linux (golang:1.25-bookworm pinned by digest, `--network none`, `unshare --net` available), statusgen built in-container from a clone pinned to this SHA. Second table: the hand run.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | fail exit=1 | sha256:ac61d3fe14b0 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b+dirty (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/desklabel/... -count=1` | fail exit=1 | sha256:0dacb780e4af | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b+dirty (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -count=1 && go test ./internal/deskkit/ -run TestForgeNoPassthrough -count=1` | pass exit=0 | sha256:d29c37f5b9f4 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b+dirty (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestApplyIssueLabelsBothBackends -count=1 -v` | fail exit=1 | sha256:f22ab7353b9a | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b+dirty (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelAppliesOwnedLabelGitHub -count=1 -v && go test ./cmd/desklabel/... -run TestDesklabelAppliesOwnedLabelGitLab -count=1 -v` | fail exit=1 | sha256:d2c495e65bcf | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b+dirty (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelRefusesUnownedLabel -count=1 -v` | fail exit=0 | sha256:c1da3242912e | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b+dirty (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelRefusesHumanDecidedForEveryRole -count=1 -v` | fail exit=0 | sha256:4b5a4132ae14 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b+dirty (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelSharedVocabularyAnyRole -count=1 -v` | pass exit=0 | sha256:357f19cd73c7 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b+dirty (on-behalf-of human:ian) (forge-identity) |
| 9 | `grep -rn -e 'flag.String("as"' -e '"--as"' tools/desk/cmd/desklabel --include='*.go' \| grep -v _test.go \| wc -l` | fail exit=1 | sha256:9a271f2a916b | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b+dirty (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./internal/forgeban/... -count=1` | pass exit=0 | sha256:58980f43252c | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b+dirty (on-behalf-of human:ian) (forge-identity) |
| 11 | `statusgen --root . --consumers --brief forge-neutral/15` | fail exit=2 | sha256:8557b6dc2097 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b+dirty (on-behalf-of human:ian) (forge-identity) |
| 12 | `grep -c 'desklabel' tools/desk/README.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b+dirty (on-behalf-of human:ian) (forge-identity) |
| 13 | `desklabel` | could-not-run exit=- — prose-led-command: not executed; the first span desklabel is a lone word mentioned ahead of the command span, not a command. Mark the command with a cmd: code span | sha256: | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b+dirty (on-behalf-of human:ian) (forge-identity) |

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | exit 0 | exit 0 on the go.mod-pinned toolchain (GOTOOLCHAIN=go1.25.0): build 0, 88 packages ok, no FAIL. On the host go1.27.1 at load avg ~34 it exited 1: writeguard callout tests hit their 5s callout deadline, internal/avatar TestGolden20px PNG golden differs (toolchain encoder), loopengine TestDrain; cmd/desklabel, internal/forgeban ok in that run. Host-environment, not desklabel | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test ./cmd/desklabel/... -count=1` | exit 0 | exit 0 — ok cmd/desklabel | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -count=1 && go test ./internal/deskkit/ -run TestForgeNoPassthrough -count=1` | exit 0 | exit 0 both; -v confirms --- PASS: TestNoForgeCLIShellout and --- PASS: TestForgeNoPassthrough (not a vacuous no-tests match) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestApplyIssueLabelsBothBackends -count=1 -v` | exit 0 | exit 0 — github + gitlab subtests PASS; github asserts issue-target and change-target requests identical (one helper); gitlab routes the issue target to /issues/7, never /merge_requests/7. Note: implemented via ApplyLabels with LabelChange.Target, not a new ApplyIssueLabels op (see Notes) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 5 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelAppliesOwnedLabelGitHub -count=1 -v && go test ./cmd/desklabel/... -run TestDesklabelAppliesOwnedLabelGitLab -count=1 -v` | exit 0 | exit 0 — GitHub PASS; GitLab issue_only, merge_request_only, both_resolve (refused without --kind, routed with it) PASS | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 6 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelRefusesUnownedLabel -count=1 -v` | refuses exit 5, zero forge calls, names both roles | command exit 0 (test PASS), 14/14 subtests PASS; each subtest asserts the verb returns exit 5, names label + both roles, and the recording fake saw 0 calls. The row's Expect (exit 5) does not match its command's exit (0 on pass) — check-definition mismatch, substance holds | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 7 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelRefusesHumanDecidedForEveryRole -count=1 -v` | every role refused exit 5, zero forge calls | command exit 0 (test PASS), 20/20 subtests PASS: 5 roles (desk, issue-loop, reviewer, verifier, worker) x add/rm x two case spellings, each exit 5, says no role, 0 calls. Same Expect-vs-command mismatch as row 6 | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 8 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelSharedVocabularyAnyRole -count=1 -v` | exit 0 | exit 0 — 16/16 subtests PASS: worker and reviewer x add/rm x shared set (help wanted, needs-decision, needs-human, question) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 9 | `grep -rn -e 'flag.String("as"' -e '"--as"' tools/desk/cmd/desklabel --include='*.go' \| grep -v _test.go \| wc -l` | prints 0 | prints 0, exit 0 in a plain shell; under bash -o pipefail (as the witness runs it) exit 1 because the first grep matches nothing. Only flags defined: --kind, --dry-run | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 10 | `cd tools/desk && go test ./internal/forgeban/... -count=1` | exit 0 | exit 0 — ok internal/forgeban | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 11 | `statusgen --root . --consumers --brief forge-neutral/15` | exit 0, every consumers claim corroborated | exit 2 COULD-NOT-CHECK — brief not in the diff against b0088804 (statusgen built from the clone at this SHA). Hand procedure per the tool's own guidance: at the authoring squash 74865cba6 with --base 08d14c91d, exit 0 — 4 corroborated, 0 disproved, 2 unchecked (the two out-of-scope entries; PR #1180 touches neither the forgeban allowlist nor deskclose/deskdisposition/deskflip/deskdispatch, so both hold). As authored the row cannot pass on merged main | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 12 | `grep -c 'desklabel' tools/desk/README.md` | value of 1 or more | prints 1 — Tool reference row: add, rm; outward write; yes | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 13 | mutation: force the ownership check to report caller-owned, run `cd tools/desk && go test ./cmd/desklabel/... -count=1`, restore, re-run | exit 1 on mutant, exit 0 after restore | permits() default branch return e.Owner == role replaced by return true: mutant exit 1 — TestDesklabelRefusesUnownedLabel red on 10/14 subtests (the role-mismatch cases; the 4 table-absent cases take the separate lookup path) and TestDesklabelDryRunWritesNothing red. Restored, md5 prefix a500b2c18eea byte-identical, suite exit 0. CAUGHT | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |

RISK-VALUE: DERIVED — human-decided Owner = ownerNone ("no role") @ tools/desk/cmd/desklabel/vocabulary.go:128 — deskclose's decisionLabels (needs-decision, human-decided @ tools/desk/cmd/deskclose/github.go:32) make this label a recorded human ruling, so no role may self-apply it; permits() case ownerNone returns false with no role reference (vocabulary.go:151-152). Row 7: 20/20 refused.
RISK-VALUE: DERIVED — superseded? Owner = roleWorker ("worker") @ vocabulary.go:112, disposition:* rows @ :114-118 — matches the code that applies them (deskclose superseded: worker proposes, reviewer confirms or disputes; deskdisposition set records a worker's finding).
RISK-VALUE: DERIVED — authorization-needed / approval-needed Owner = roleReviewer ("reviewer") @ vocabulary.go:122,124 — deskflip's labelBeforeFlip/labelAfterFlip pair; deskdispatch applies authorization-needed under the reviewer role's credential, so ownership keys on the reviewer, not the invoking session.
RISK-VALUE: DERIVED — helpWantedLabel = "help wanted" @ vocabulary.go:57 plus topology decision_owed rows (needs-decision @ topology.yaml:330, question @ :332, needs-human @ :336) as shared — the escalation vocabulary any role must be able to raise; clearing removes only the label, never closes an item, and deskclose's close gate reads only needs-decision/human-decided. needs-human is a widening beyond the brief's 3-wide set (see Notes).

Notes:
- BLOCKED (check-definition), not a product failure. All 13 rows hold when run by hand (row 1 under the go.mod-pinned go1.25.0 toolchain). The Linux witness passes rows 3, 8, 10 and 12; rows 1, 2, 4 and 5 fail only because loopback is down in the witness sandbox (#1925). Rows 6 and 7 expect exit 5 where a passing `go test` exits 0, row 9 exits 1 under pipefail, row 11 cannot pass on merged main as written (it passes at the authoring squash with a pinned base), and row 13 has no cmd: span. `statusgen brief --check-verified` with a hypothetical flip exits 1. Advancing needs rows 6, 7, 9, 11 and 13 re-authored, tracked with the other row re-authors at #1927. All four RISK-VALUE lines are DERIVED.
- Ground expectation written before reading diffs: new desklabel verb with add/rm and --dry-run, closed role-keyed vocabulary, SessionTokenRole role, no --as flag, ownership check before any forge call, new ApplyIssueLabels op, inventory row 38, README row. All held except the op, which shipped as ApplyLabels with LabelChange.Target (an earlier PR's addition); inventory row 38 is GetIssueTyped (backing --kind). Row 4's test pins the issue-target wire on both backends, so the Verify substance holds; the brief's Task 1 / consumers prose naming ApplyIssueLabels is stale.
- Ordering confirmed in source: roleFn at verbs.go:136, authorize at :142, first forge reach (forgeForFn) at :148, GetIssue/GetIssueTyped at :160-162, ApplyLabels at :208.
- Shared set is 4-wide (adds needs-human, derived from topology decision_owed), not the brief's 3-wide. needs-human gates no close path in tools/desk (only the topology tables reference it), so it is analogous to needs-decision; routed as an observation for the reviewer, not a defect.
- Check-definition failures (row fails as authored, substance passes by hand): rows 6 and 7 (Expect exit 5 vs go test exit 0), row 9 (pipefail), row 11 (merged-brief consumers structural limit), row 13 (prose mutation; no cmd: span). Per the dispatch rules these end BLOCKED, not PASS. A brief amendment would fix them: Expect exit 0 with the refusal asserted inside the test for 6/7; a pipefail-safe form for 9 (e.g. grep -c with an explicit exit-1-as-zero guard); a pinned --base form or waiver for 11; a cmd: span pointing at the repo's mutation harness for 13.
- Environment (known, not re-diagnosed): Linux witness rows 1, 2, 4, 5 fail only on the unshare loopback-down issue. Host go1.27.1 row 1 failures are toolchain and load (avatar golden, 5s callout deadlines, loopengine TestDrain); the go.mod-pinned go1.25.0 run is clean.
- Test runs leave an untracked tools/desk/cmd/commsloop/mailbox/ directory in the tree, which stamps a non-dry witness +dirty. Minor hygiene finding, unrelated to this brief.
- Stream README row 15 on main reads implemented, although history carries a verify commit titled implemented-to-verified (#1450); the status is currently implemented.

VERIFY: BLOCKED

### Non-implementer verifier run — VERIFY: BLOCKED — 2026-10-04 claude-opus-5-5-verifier

Non-implementer verification of forge-neutral/15 on merged main 3ad1ad83c871b5e2693702810f81e58d8ef085e8 (origin/main, confirmed by ls-remote at run time), own detached worktree. gate: model, all four risk answers no. Offline (KUBECONFIG=/dev/null). This pass runs the CURRENT Verify table, as re-written for row 9 by medici-finance/assay#2047 (medici-finance/assay#1862). Implementation under test: PR medici-finance/assay#1180 (squash 2d2966622), later touched by medici-finance/assay#1919 (4 lines in verbs.go registering the audited outbound-scan override flag; one test line). Brief authored doc-only in medici-finance/assay#998 (squash 74865cba6).

Ground expectation, written before reading any diff or test: a new desklabel verb with add/rm and --dry-run; a closed, role-keyed vocabulary as a Go literal (shared question / help wanted / needs-decision; worker superseded? and the three disposition labels; reviewer authorization-needed and approval-needed; human-decided refused for every role; anything else refused, exit 5); case-insensitive match with a canonical-case write; the role taken from deskkit.SessionTokenRole and no role flag; the ownership check before any forge read or write; a new ApplyIssueLabels op on both backends; inventory row 38; a README Tool reference row; the forgeban ratchet unchanged. All of this held except two points. There is no ApplyIssueLabels op: desklabel reuses ApplyLabels with LabelChange.Target, and row 4's test pins the issue-target wire on both backends. Inventory row 38 is GetIssueTyped, which backs --kind. Both are known deviations carried from earlier passes, and the brief's Task 1 and consumers prose is stale on them. The shared set is 4-wide because needs-human comes from topology decision_owed.

The witness table (verbatim at the end of this fragment) ran on a darwin host. Every check:ci row (1-8, 10, 11) is could-not-run because the network-off sandbox needs Linux unshare --net (medici-finance/assay#1800, still open). Row 9 passes under the witness now that medici-finance/assay#2047 has made it pipefail-safe. Row 12 passes. Row 13 is could-not-run: the cell is a prose mutation procedure with no cmd: span. The hand-run table below re-runs every row on this host. A hand run is not a witness pass.

| # | Command | Expect | Observed (exit + key output) | Date Runner |
|---|---------|--------|------------------------------|-------------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | exit 0 | exit 0 on host go1.27.1 (load avg ~8): build 0, 96 packages ok, no FAIL; cmd/desklabel, internal/deskkit, internal/forgeban ok. Discharges Verify row 1 | 2026-10-04 claude-opus-5-5-verifier (hand, darwin) |
| 2 | `cd tools/desk && go test ./cmd/desklabel/... -count=1` | exit 0 | exit 0 — ok cmd/desklabel. Discharges Verify row 2 | 2026-10-04 claude-opus-5-5-verifier (hand, darwin) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -count=1 && go test ./internal/deskkit/ -run TestForgeNoPassthrough -count=1` | exit 0 | exit 0 both; a -v re-run shows --- PASS: TestNoForgeCLIShellout and --- PASS: TestForgeNoPassthrough, so the run matched real tests. Discharges Verify row 3 | 2026-10-04 claude-opus-5-5-verifier (hand, darwin) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestApplyIssueLabelsBothBackends -count=1 -v` | exit 0 | exit 0 — TestApplyIssueLabelsBothBackends/github and /gitlab PASS. The wire is ApplyLabels with LabelChange.Target, not a new op (see above). Discharges Verify row 4 | 2026-10-04 claude-opus-5-5-verifier (hand, darwin) |
| 5 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelAppliesOwnedLabelGitHub -count=1 -v && go test ./cmd/desklabel/... -run TestDesklabelAppliesOwnedLabelGitLab -count=1 -v` | exit 0 | exit 0 — GitHub PASS. GitLab subtests issue_only to the issues endpoint, merge_request_only to the merge_requests endpoint, and both_resolve (refused without --kind, routed with it) all PASS. Discharges Verify row 5 | 2026-10-04 claude-opus-5-5-verifier (hand, darwin) |
| 6 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelRefusesUnownedLabel -count=1 -v` | refusal exit 5, zero forge calls, both roles named | command exit 0 (test PASS), 14/14 subtests PASS. Each subtest asserts code == ExitRefused (5), that the output names the roles, and that the recording fake saw zero calls. Discharges Verify row 6 | 2026-10-04 claude-opus-5-5-verifier (hand, darwin) |
| 7 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelRefusesHumanDecidedForEveryRole -count=1 -v` | every role refused exit 5, zero forge calls | command exit 0 (test PASS), 20/20 subtests PASS. The role set is worker, reviewer and every role in deskkit.LoopTokenRoles(), each with add/rm and two case spellings. Each asserts ExitRefused, "no role" in the output, and zero calls. Discharges Verify row 7 | 2026-10-04 claude-opus-5-5-verifier (hand, darwin) |
| 8 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelSharedVocabularyAnyRole -count=1 -v` | exit 0 | exit 0 — 16/16 subtests PASS: worker and reviewer, add/rm, across the shared set (help wanted, needs-decision, needs-human, question). Discharges Verify row 8 | 2026-10-04 claude-opus-5-5-verifier (hand, darwin) |
| 9 | the row 9 test -d / grep / wc -l pipeline | output is 0 | prints 0, exit 0 under bash -o pipefail. The only flags desklabel defines are --kind and --dry-run (verbs.go), plus the audited --force-scan-override that medici-finance/assay#1919 registers on every write verb. None of them is a role input. Discharges Verify row 9 (the witness passed too) | 2026-10-04 claude-opus-5-5-verifier (hand, darwin) |
| 10 | `cd tools/desk && go test ./internal/forgeban/... -count=1` | exit 0 | exit 0 — ok internal/forgeban. Discharges Verify row 10 | 2026-10-04 claude-opus-5-5-verifier (hand, darwin) |
| 11 | `statusgen --root . --consumers --brief forge-neutral/15` | exit 0, every consumers claim corroborated | On the clean merged-main tree: exit 2, COULD-NOT-CHECK, "not in the diff against 3ad1ad83c871". With the verifyrun witness edit present in the working tree: exit 0, but 0 corroborated / 6 unchecked, which is a vacuous exit 0 and not counted. Tool-sanctioned merged-brief re-run at the authoring squash 74865cba6 with --base 74865cba6^: exit 0, 4 corroborated, 0 disproved, 2 unchecked (the two out-of-scope entries). The implementation squash 2d2966622 touches none of the forgeban allowlist, deskclose, deskdisposition, deskflip or deskdispatch, so both out-of-scope claims hold. As written, this row cannot pass on merged main. Discharges Verify row 11 only through the authoring-diff procedure; the row stays held | 2026-10-04 claude-opus-5-5-verifier (hand, darwin; statusgen v1.0.31) |
| 12 | `grep -c 'desklabel' tools/desk/README.md` | value of 1 or more | prints 1, exit 0 — the Tool reference row (add, rm; outward write; yes). Discharges Verify row 12 (the witness passed too) | 2026-10-04 claude-opus-5-5-verifier (hand, darwin) |
| 13 | mutation: in permits() (vocabulary.go) replace the default branch's return e.Owner == role with return true, run `cd tools/desk && go test ./cmd/desklabel/... -count=1`, restore, re-run | exit 1 on the mutant, exit 0 after restore | Mutant: exit 1. TestDesklabelRefusesUnownedLabel failed 10/14 subtests (the role-mismatch cases; the 4 table-absent cases take the separate lookup path), and TestDesklabelDryRunWritesNothing also failed. Restored from a saved copy; md5 a500b2c18eea… matches the original byte for byte; the suite exits 0 again. CAUGHT. Discharges Verify row 13 by hand; the witness cannot run this row (no cmd: span) | 2026-10-04 claude-opus-5-5-verifier (hand, darwin) |

Risk-bearing value enumeration (fail-safe trigger: the risk fields are present and all no, and the brief prose names forge.go, a security-path file the implementation never edited). Scope: the implementation squash 2d2966622, the desklabel lines of medici-finance/assay#1919, and the brief's Deliverables.
- roleWorker = "worker" @ tools/desk/cmd/desklabel/vocabulary.go:37; roleReviewer = "reviewer" @ :38; ownerShared = "shared" @ :48; ownerNone = "no role" @ :50; helpWantedLabel = "help wanted" @ :57.
- Owned rows: superseded? @ :112, disposition:superseded @ :114, disposition:resolved-elsewhere @ :116 and disposition:needs-rebase @ :118 belong to worker; authorization-needed @ :122 and approval-needed @ :124 belong to reviewer; human-decided @ :128 belongs to ownerNone.
- The shared rows are derived from topology.yaml labels.decision_owed: needs-decision @ topology.yaml:330, question @ :332, needs-human @ :336.
- ExitRefused = 5 @ tools/desk/internal/deskkit/exitcodes.go:51 is pre-existing and was not changed by this item.
- Ranking: none of these is irreversible. A wrong label is undone by the next call from an entitled role, and a wrong table row by an edit and a redeploy. The top entries are the ownership rows, because a mis-owned row would let one role forge another role's marker.

RISK-VALUE: DERIVED — human-decided Owner = ownerNone ("no role") @ tools/desk/cmd/desklabel/vocabulary.go:128 — deskclose's decisionLabels (needs-decision, human-decided @ tools/desk/cmd/deskclose/github.go:32) make this label a record of a human ruling, so no role may self-apply it. permits() case ownerNone returns false with no reference to the role (vocabulary.go:151-152). Row 7: 20/20 refused.
RISK-VALUE: DERIVED — superseded? Owner = roleWorker ("worker") @ vocabulary.go:112, with the disposition rows @ :114-118 — this matches the code that applies them. deskclose labelProposed = "superseded?" @ tools/desk/cmd/deskclose/superseded.go:57 is the worker's proposal, which the reviewer only confirms or disputes, and deskdisposition set records a worker's finding.
RISK-VALUE: DERIVED — authorization-needed / approval-needed Owner = roleReviewer ("reviewer") @ vocabulary.go:122,124 — these are deskflip's labelBeforeFlip / labelAfterFlip @ tools/desk/cmd/deskflip/flip.go:115-116, the reviewer-run ready-flip pair. deskdispatch applies authorization-needed under the reviewer role's credential, so ownership follows the reviewer role, not the session that invokes the write.
RISK-VALUE: DERIVED — helpWantedLabel = "help wanted" @ vocabulary.go:57 and the topology decision_owed rows (topology.yaml:330, 332, 336) are shared — they are the escalation vocabulary, and any role must be able to raise it. A clear removes only the label. It never closes an item, and deskclose's close gate reads only needs-decision and human-decided. needs-human widens the brief's 3-wide set; it gates no close path in tools/desk.

Held rows:
- Witness rows 1-8 and 10 are could-not-run on darwin (medici-finance/assay#1800). All of them pass by hand, and a hand run is not a witness pass.
- Witness row 11 is could-not-run on darwin. By hand on merged main it exits 2 (could-not-check), and it passes only at the authoring squash. This is the structural limit for a merged brief (medici-finance/assay#1281); the row needs re-authoring with a pinned base, or a waiver. medici-finance/assay#1281 is still open.
- Witness row 13 is could-not-run (prose-led command, no cmd: span). It passes by hand (mutant CAUGHT). verifyrun --check reports row 13 as fail, apparently because it matched the bare desklabel command to the older 2026-09-27 witness row (fail exit=5). The row needs a cmd: span pointing at the repo's mutation harness (cmd/desklabel/mutations.json). The re-author is tracked in medici-finance/assay#1927: a 2026-10-04 comment there (issuecomment-5976638074) adds forge-neutral/15 row 13 with the mutation result and the cmd: span fix. The issue body itself does not list this brief, so that comment is where the item lives.
- verifyrun --check summary: 2 pass, 1 fail, 10 could-not-run/missing (of 13 Verify rows), exit 2.
- Correction to the 2026-09-30 block above. It says rows 6, 7, 9, 11 and 13 are tracked at medici-finance/assay#1927, but the #1927 issue body lists none of them. Where each one is tracked now: row 9 was re-authored by medici-finance/assay#2047 and passes the witness; row 11 is medici-finance/assay#1281; rows 6 and 7 (Expect says exit 5, but the passing `go test` exits 0) and row 13 are added to #1927 by 2026-10-04 comments there.

VERIFY: BLOCKED — no product failure. Every row holds by hand on merged main 3ad1ad83c871, with row 11 holding only through the authoring-diff procedure. The witness passes only rows 9 and 12. Rows 1-8 and 10-11 need a Linux witness (medici-finance/assay#1800). Row 11 cannot pass on merged main as written, and row 13 has no cmd: span. All four RISK-VALUE lines are DERIVED.

Execution witness for this run (statusgen verifyrun, non-dry, 2026-10-04):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ 3ad1ad83c871 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/desklabel/... -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ 3ad1ad83c871 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -count=1 && go test ./internal/deskkit/ -run TestForgeNoPassthrough -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ 3ad1ad83c871 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestApplyIssueLabelsBothBackends -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ 3ad1ad83c871 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelAppliesOwnedLabelGitHub -count=1 -v && go test ./cmd/desklabel/... -run TestDesklabelAppliesOwnedLabelGitLab -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ 3ad1ad83c871 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelRefusesUnownedLabel -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ 3ad1ad83c871 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelRefusesHumanDecidedForEveryRole -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ 3ad1ad83c871 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelSharedVocabularyAnyRole -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ 3ad1ad83c871 (on-behalf-of human:ian) (forge-identity) |
| 9 | `test -d tools/desk/cmd/desklabel && { grep -rn -e 'flag.String("as"' -e '"--as"' tools/desk/cmd/desklabel --include='*.go' \|\| [ $? -eq 1 ]; } \| { grep -v _test.go \|\| [ $? -eq 1 ]; } \| wc -l` | pass exit=0 | sha256:4eff2db4bada | 2026-10-04 | assay-verifier-app[bot] @ 3ad1ad83c871 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./internal/forgeban/... -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ 3ad1ad83c871 (on-behalf-of human:ian) (forge-identity) |
| 11 | `statusgen --root . --consumers --brief forge-neutral/15` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ 3ad1ad83c871 (on-behalf-of human:ian) (forge-identity) |
| 12 | `grep -c 'desklabel' tools/desk/README.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-04 | assay-verifier-app[bot] @ 3ad1ad83c871 (on-behalf-of human:ian) (forge-identity) |
| 13 | `desklabel` | could-not-run exit=- — prose-led-command: not executed; the first span desklabel is a lone word mentioned ahead of the command span, not a command. Mark the command with a cmd: code span | sha256: | 2026-10-04 | assay-verifier-app[bot] @ 3ad1ad83c871 (on-behalf-of human:ian) (forge-identity) |

### Non-implementer verifier re-run — VERIFY: BLOCKED — 2026-10-04 claude-opus-5-5-verifier

Non-implementer verification of forge-neutral/15 on merged main 5f5072d89b1120ad3d35e78a0ffadebe0a5f4bc8, own detached worktree, offline (KUBECONFIG=/dev/null). gate: model, all four risk answers no. Host darwin, go1.27.1, statusgen v1.0.31. This pass supersedes the unlanded pass at 4aac3271b3b4.

Why this re-run: since the last landed receipt (3ad1ad83c871), five medici-finance/assay PRs changed this brief's inputs. medici-finance/assay#2118 (deskboard re-review for own-file changes) and medici-finance/assay#2138 (deskpr resolves --pr from its head ref) changed tools/desk/internal/deskkit/forge.go and forge_github.go. medici-finance/assay#2117 (deskboard riskClassed), medici-finance/assay#2120 (deskroster --scope roots) and medici-finance/assay#2155 (roster beacon writes) changed tools/desk/README.md; #2138 also edits the README's deskpr prose. None of the five touches label, desklabel or label-ownership behaviour. Their added and removed lines in those files contain zero lines matching label or desklabel. The forge.go and forge_github.go hunks are in RepoCommit, ListCommitChanges, CompareRefs and the PullRequest struct (head-ref fields), not ApplyLabels or LabelChange. The README desklabel Tool reference row (line 43) is unchanged.

Ground expectation, written from the brief alone before reading any prior Evidence: a desklabel verb with add/rm and --dry-run; a closed, role-keyed Go-literal vocabulary (shared question / help wanted / needs-decision; worker superseded? and the three disposition labels; reviewer authorization-needed and approval-needed; human-decided refused for every role; anything else exit 5); case-insensitive match; role from deskkit.SessionTokenRole with no role flag; ownership check before any forge call; an issue-scoped label write on both backends; a README row; the forgeban ratchet unchanged. Everything held except the points earlier passes also recorded. No ApplyIssueLabels op exists anywhere in history (git log -S finds none). desklabel calls ApplyLabels with LabelChange.Target = TargetIssue or TargetChange, and row 4's test pins the issue-target wire on both backends. The shared set is 4-wide: it also includes needs-human, read from topology decision_owed. The brief's Task 1 and consumers prose naming ApplyIssueLabels is stale. Ordering in source: roleFn at verbs.go:140, authorize at :146, first forge reach (forgeForFn) at :152, GetIssueTyped/GetIssue at :164/:166, ApplyLabels at :212.

Hand run on this host (a hand run is not a witness pass):

| # | Command | Expect | Observed (exit + key output) | Date Runner |
|---|---------|--------|------------------------------|-------------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | exit 0 | exit 0 on host go1.27.1 at load avg 10-43: build 0, 98 packages ok, 0 FAIL lines; cmd/desklabel, internal/deskkit (152s), internal/forgeban ok | 2026-10-04 claude-opus-5-5-verifier (hand, darwin) |
| 2 | `cd tools/desk && go test ./cmd/desklabel/... -count=1` | exit 0 | exit 0 — ok cmd/desklabel 1.964s | 2026-10-04 claude-opus-5-5-verifier (hand, darwin) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -count=1 && go test ./internal/deskkit/ -run TestForgeNoPassthrough -count=1` | exit 0 | exit 0 both. A -v re-run shows --- PASS: TestNoForgeCLIShellout and --- PASS: TestForgeNoPassthrough with 5 subtests, including method_set_equals_the_committed_inventory, so real tests matched | 2026-10-04 claude-opus-5-5-verifier (hand, darwin) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestApplyIssueLabelsBothBackends -count=1 -v` | exit 0, GitHub shared-helper wire, GitLab issues endpoint distinct from MR | exit 0 — /github and /gitlab PASS. In the test source, GitHub asserts that issue-target and change-target requests are identical (one helper) on /issues/7/labels and never /pulls/7. GitLab routes to /issues/7, never /merge_requests/7, with an MR at the same iid served as a trap. The backend writes PUT /projects/:id/issues/:iid for TargetIssue (forge_gitlab.go ApplyLabels). The wire is ApplyLabels plus Target, not a new op | 2026-10-04 claude-opus-5-5-verifier (hand, darwin) |
| 5 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelAppliesOwnedLabelGitHub -count=1 -v && go test ./cmd/desklabel/... -run TestDesklabelAppliesOwnedLabelGitLab -count=1 -v` | exit 0 | exit 0 — GitHub PASS. GitLab issue_only → the issues endpoint, merge_request_only → the merge_requests endpoint, and both_resolve (refused without --kind, routed with it) all PASS | 2026-10-04 claude-opus-5-5-verifier (hand, darwin) |
| 6 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelRefusesUnownedLabel -count=1 -v` | refusal exit 5, zero forge calls, both roles named | command exit 0 (test PASS), 14/14 subtests PASS: 7 cases × add/rm. Each asserts code == ExitRefused (5), that the output names the label and both roles (or "no desklabel vocabulary entry"), and that fakeForge.calls() == 0. The Expect column's "exit 5" is the verb's refusal asserted inside the test; the go test command itself exits 0 | 2026-10-04 claude-opus-5-5-verifier (hand, darwin) |
| 7 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelRefusesHumanDecidedForEveryRole -count=1 -v` | every role refused exit 5, zero forge calls | command exit 0 (test PASS), 20/20 subtests PASS: roles desk, issue-loop, reviewer, verifier and worker × add/rm × the spellings human-decided and Human-Decided. Each asserts ExitRefused, "no role" in the output, and zero calls. Same Expect-versus-command note as row 6 | 2026-10-04 claude-opus-5-5-verifier (hand, darwin) |
| 8 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelSharedVocabularyAnyRole -count=1 -v` | exit 0 | exit 0 — 16/16 subtests PASS: worker and reviewer × add/rm × help wanted, needs-decision, needs-human, question | 2026-10-04 claude-opus-5-5-verifier (hand, darwin) |
| 9 | the row 9 test -d / grep / wc -l pipeline | output is 0 | prints 0, exit 0 under bash. The only "--as" text in the package is the forbidden-needle list in the test file (desklabel_test.go:641), which the pipeline excludes. The witness passed this row too | 2026-10-04 claude-opus-5-5-verifier (hand, darwin) |
| 10 | `cd tools/desk && go test ./internal/forgeban/... -count=1` | exit 0 | exit 0 — ok internal/forgeban 2.151s | 2026-10-04 claude-opus-5-5-verifier (hand, darwin) |
| 11 | `statusgen --root . --consumers --brief forge-neutral/15` | exit 0, every consumers claim corroborated | On the clean merged-main tree: exit 2, COULD-NOT-CHECK, "not in the diff against 5f5072d89b11…, so this run carries no evidence about its claims". The tool-sanctioned merged-brief procedure (checkout of the authoring squash 74865cba6 with --base 74865cba6^, then back to 5f5072d89): exit 0, 4 corroborated, 0 disproved, 2 unchecked (the two out-of-scope entries). The implementation squash 2d2966622 touches none of forgeban, deskclose, deskdisposition, deskflip or deskdispatch, so both out-of-scope claims hold. As written, the row cannot pass on merged main (medici-finance/assay#1281); it stays held | 2026-10-04 claude-opus-5-5-verifier (hand, darwin; statusgen v1.0.31) |
| 12 | `grep -c 'desklabel' tools/desk/README.md` | value of 1 or more | prints 1, exit 0 — Tool reference row at README line 43 (add, rm; outward write; yes). The witness passed this row too | 2026-10-04 claude-opus-5-5-verifier (hand, darwin) |
| 13 | mutation: in permits() (vocabulary.go) replace the default branch's return e.Owner == role with return true (the row-13 mutant in cmd/desklabel/mutations.json), run `cd tools/desk && go test ./cmd/desklabel/... -count=1`, restore, re-run | exit 1 on the mutant, exit 0 after restore | Mutant: exit 1. TestDesklabelRefusesUnownedLabel failed 10/14 subtests (the role-mismatch cases; the 4 table-absent cases take the separate lookup path), and TestDesklabelDryRunWritesNothing also failed. Restored from a saved copy: md5 a500b2c18eeafc86e48d02f796725936 before and after, and git status is clean apart from the verifier output dir. The suite exits 0 again. CAUGHT | 2026-10-04 claude-opus-5-5-verifier (hand, darwin) |

Risk-bearing value enumeration. The fail-safe trigger was checked: risk metadata is present and all no, and the item is not irreversible. Enumeration scope: implementation squash 2d2966622 as it reads on 5f5072d89, and the brief's Deliverables. None of the five wake PRs changes any of these literals.
- roleWorker = "worker" @ tools/desk/cmd/desklabel/vocabulary.go:37; roleReviewer = "reviewer" @ :38; ownerShared = "shared" @ :48; ownerNone = "no role" @ :50; helpWantedLabel = "help wanted" @ :57.
- Owned rows: superseded? @ :112, disposition:superseded @ :114, disposition:resolved-elsewhere @ :116 and disposition:needs-rebase @ :118 belong to worker; authorization-needed @ :122 and approval-needed @ :124 belong to reviewer; human-decided @ :128 belongs to ownerNone.
- Shared rows come from topology.yaml labels.decision_owed: needs-decision @ topology.yaml:330, question @ :332, needs-human @ :336.
- ExitRefused = 5 @ tools/desk/internal/deskkit/exitcodes.go:51 is pre-existing and was not changed by this item.
- Ranking: none is irreversible. A wrong label is reversed by the next call from an entitled role, and a wrong table row by an edit and a redeploy. The ownership rows rank top, because a mis-owned row lets one role forge another's marker.

RISK-VALUE: DERIVED — human-decided Owner = ownerNone ("no role") @ tools/desk/cmd/desklabel/vocabulary.go:128 — deskclose decisionLabels = {"needs-decision", "human-decided"} @ tools/desk/cmd/deskclose/github.go:32 make this label a record of a human ruling, so no role may self-apply it. permits() case ownerNone returns false regardless of role (vocabulary.go:151-152). Row 7: 20/20 refused.
RISK-VALUE: DERIVED — superseded? Owner = roleWorker ("worker") @ vocabulary.go:112, with disposition rows @ :114-118 — labelProposed = "superseded?" @ tools/desk/cmd/deskclose/superseded.go:57 is the worker's proposal, which the reviewer only confirms or disputes. deskdisposition set records a worker's finding.
RISK-VALUE: DERIVED — authorization-needed / approval-needed Owner = roleReviewer ("reviewer") @ vocabulary.go:122,124 — these are labelBeforeFlip / labelAfterFlip @ tools/desk/cmd/deskflip/flip.go:115-116, the reviewer-run ready-flip pair. deskdispatch applies authorization-needed under the reviewer credential, so ownership keys on the reviewer role.
RISK-VALUE: DERIVED — helpWantedLabel = "help wanted" @ vocabulary.go:57 and the decision_owed rows (topology.yaml:330, 332, 336) are shared, as the escalation vocabulary any role must be able to raise. A clear removes only the label and never closes an item; deskclose's close gate reads only decisionLabels. needs-human widens the brief's 3-wide set and gates no close path in tools/desk.

Held rows:
- Witness rows 1-8 and 10-11 are could-not-run: check:ci rows need the Linux unshare --net sandbox, and this host is darwin (medici-finance/assay#1800). All of them pass by hand, row 11 only through the authoring-diff procedure.
- Witness row 11: by hand on merged main it exits 2 (could-not-check), which is the structural limit for a merged brief (medici-finance/assay#1281). The row needs re-authoring with a pinned base, or a waiver.
- Witness row 13 is could-not-run in this witness (prose-led command, no cmd: span); by hand the mutant is CAUGHT. verifyrun --check scores row 13 as fail. The only fail-result witness for row 13 in the brief is the 2026-09-27 row (fail exit=5), so the check appears to bind row 13 to that older row rather than to this run's could-not-run. Re-authoring the row with a cmd: span pointing at the mutation harness is tracked in medici-finance/assay#1927, as are the rows 6 and 7 wording issues: Expect says exit 5 while the passing go test exits 0.
- The witness Runner stamp reads 5f5072d89b11+dirty. The only working-tree change at witness time was the untracked verifier output dir .v/ that this dispatch requires. git status showed no other modification, and the mutation file was restored byte-identical before the witness ran.

verifyrun --check summary: docs/streams/forge-neutral/brief-15-desklabel-role-keyed.md: 2 pass, 1 fail, 10 could-not-run/missing (of 13 Verify rows), exit 2.

VERIFY: BLOCKED — no product failure. Every row holds by hand on merged main 5f5072d89, with row 11 holding only through the authoring-diff procedure. None of the five wake PRs (medici-finance/assay#2117, #2118, #2120, #2138, #2155) touches label, desklabel or label-ownership behaviour, or the README desklabel row. The witness passes only rows 9 and 12. Rows 1-8 and 10 need a Linux witness (medici-finance/assay#1800), row 11 cannot pass on merged main as written (medici-finance/assay#1281), and row 13 has no cmd: span (medici-finance/assay#1927). All four RISK-VALUE lines are DERIVED.

Execution witness for this run (statusgen verifyrun, non-dry, 2026-10-04):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ 5f5072d89b11+dirty (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/desklabel/... -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ 5f5072d89b11+dirty (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -count=1 && go test ./internal/deskkit/ -run TestForgeNoPassthrough -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ 5f5072d89b11+dirty (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestApplyIssueLabelsBothBackends -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ 5f5072d89b11+dirty (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelAppliesOwnedLabelGitHub -count=1 -v && go test ./cmd/desklabel/... -run TestDesklabelAppliesOwnedLabelGitLab -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ 5f5072d89b11+dirty (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelRefusesUnownedLabel -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ 5f5072d89b11+dirty (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelRefusesHumanDecidedForEveryRole -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ 5f5072d89b11+dirty (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelSharedVocabularyAnyRole -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ 5f5072d89b11+dirty (on-behalf-of human:ian) (forge-identity) |
| 9 | `test -d tools/desk/cmd/desklabel && { grep -rn -e 'flag.String("as"' -e '"--as"' tools/desk/cmd/desklabel --include='*.go' \|\| [ $? -eq 1 ]; } \| { grep -v _test.go \|\| [ $? -eq 1 ]; } \| wc -l` | pass exit=0 | sha256:4eff2db4bada | 2026-10-04 | assay-verifier-app[bot] @ 5f5072d89b11+dirty (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./internal/forgeban/... -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ 5f5072d89b11+dirty (on-behalf-of human:ian) (forge-identity) |
| 11 | `statusgen --root . --consumers --brief forge-neutral/15` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ 5f5072d89b11+dirty (on-behalf-of human:ian) (forge-identity) |
| 12 | `grep -c 'desklabel' tools/desk/README.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-04 | assay-verifier-app[bot] @ 5f5072d89b11+dirty (on-behalf-of human:ian) (forge-identity) |
| 13 | `desklabel` | could-not-run exit=- — prose-led-command: not executed; the first span desklabel is a lone word mentioned ahead of the command span, not a command. Mark the command with a cmd: code span | sha256: | 2026-10-04 | assay-verifier-app[bot] @ 5f5072d89b11+dirty (on-behalf-of human:ian) (forge-identity) |

### Non-implementer verifier re-run — VERIFY: BLOCKED — 2026-10-22 claude-opus-5-5-verifier

Non-implementer verification of forge-neutral/15 on merged main 05dc9fbc88f2b1b23cd78bebc8634c564327eca4 (equal to the forge's main at run time), own detached admitted worktree, offline (KUBECONFIG=/dev/null). gate: model; all four risk answers no. Host darwin, go1.27.2, git 2.56.0, statusgen v1.0.54. Admission: `deskdispatch --check-verifier` rc=0 before any row, binding run=4c80fdcd481d29079a75eec8ca012bd0ac100213aabec0b089a042302942c8c2, source=05dc9fbc88f2, model claude-opus-5-5, tier strong.

Since the last pass (5f5072d89b11) the only change under the desklabel package is main.go (perf instrumentation, medici-finance/assay#3051); vocabulary.go, verbs.go, forge.go and the test file are byte-unchanged. The brief changed only by joining its wrapped `files:` lines. The README desklabel Tool reference row is present (now at line 80). Ordering in source is unchanged: roleFn at verbs.go:140, authorize at :146, first forge reach (forgeForFn) at :152, GetIssueTyped/GetIssue at :164/:166, ApplyLabels at :212. The only flag set desklabel builds is the per-verb FlagSet at verbs.go:65 (no role flag). The known scope deviation still holds: no ApplyIssueLabels op exists; desklabel calls ApplyLabels with LabelChange.Target, and row 4's test pins the issue-target wire on both backends.

The execution witness below was written first, by statusgen verifyrun in write mode, from the admitted home on a clean tree at the source sha. It passes rows 9 and 12 only. Rows 1-8, 10 and 11 are could-not-run on this darwin host (check:ci needs Linux unshare --net, medici-finance/assay#1800), and row 13 is could-not-run as a prose-led command (medici-finance/assay#1927).
| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/desklabel/... -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -count=1 && go test ./internal/deskkit/ -run TestForgeNoPassthrough -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestApplyIssueLabelsBothBackends -count=1 -v` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelAppliesOwnedLabelGitHub -count=1 -v && go test ./cmd/desklabel/... -run TestDesklabelAppliesOwnedLabelGitLab -count=1 -v` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelRefusesUnownedLabel -count=1 -v` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelRefusesHumanDecidedForEveryRole -count=1 -v` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelSharedVocabularyAnyRole -count=1 -v` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 9 | `test -d tools/desk/cmd/desklabel && { grep -rn -e 'flag.String("as"' -e '"--as"' tools/desk/cmd/desklabel --include='*.go' \|\| [ $? -eq 1 ]; } \| { grep -v _test.go \|\| [ $? -eq 1 ]; } \| wc -l` | pass exit=0 | sha256:4eff2db4bada | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./internal/forgeban/... -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 11 | `statusgen --root . --consumers --brief forge-neutral/15` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 12 | `grep -c 'desklabel' tools/desk/README.md` | pass exit=0 | sha256:53c234e5e847 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 13 | `desklabel` | could-not-run exit=- — prose-led-command: not executed; the first span desklabel is a lone word mentioned ahead of the command span, not a command. Mark the command with a cmd: code span | sha256: | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4034 run=4c80fdcd481d29079a75eec8ca012bd0ac100213aabec0b089a042302942c8c2 source=05dc9fbc88f2b1b23cd78bebc8634c564327eca4 brief=docs/streams/forge-neutral/brief-15-desklabel-role-keyed.md model=claude-opus-5-5 tier=strong

Hand runs (supplementary; a hand run is not a witness pass and these observations discharge no witness row). Rows 2-8, 10, 11 and 13 were run in a fresh scratch clone at 05dc9fbc88f2, inside a docker container started with --network none (only loopback and an inert tunnel interface present), image go1.27.2 linux/arm64, module cache mounted read-only with GOPROXY=off. Row 1 was attempted in two environments, and neither result can be credited.
- Row 1: could-not-check by hand. In a network-none linux/amd64 container (git 2.56.0, emulated on this arm host) it exited 1. Nine packages failed, among them cellctl, deskdispatch, deskwt and deskkit, through per-package 10-minute timeouts under emulation and process-control tests. A native darwin run at load average about 53 also exited 1, through unix-socket paths over the darwin length limit (bind: invalid argument), a scratch-clone origin that names no known forge, and 10-minute timeouts. cmd/desklabel and internal/forgeban were ok in both runs, and none of the failures is in a package this item delivered. CI's build-test check run at this sha (job 119579348844) concluded success; that is context, not a witness.
- Row 2: exit 0, ok cmd/desklabel.
- Row 3: exit 0 both, ok internal/deskkit for TestNoForgeCLIShellout and for TestForgeNoPassthrough.
- Row 4: exit 0. TestApplyIssueLabelsBothBackends, with subtests github and gitlab, PASS.
- Row 5: exit 0 both. TestDesklabelAppliesOwnedLabelGitHub PASS. TestDesklabelAppliesOwnedLabelGitLab PASS on issue_only to the issues endpoint, merge_request_only to the merge_requests endpoint, and both_resolve (refused without --kind, routed with it).
- Row 6: command exit 0 (test PASS), 14/14 subtests: 7 cases × add/rm. These are worker on approval-needed and authorization-needed, reviewer on superseded? and the disposition family, desk role on a worker-owned label, a label with no vocabulary entry, and a size-family label. The Expect cell's exit 5 is the verb's refusal asserted inside the test; the go test command itself exits 0 (medici-finance/assay#1927).
- Row 7: command exit 0 (test PASS), 20/20 subtests: roles desk, issue-loop, reviewer, verifier and worker × add/rm × human-decided and Human-Decided.
- Row 8: exit 0, 16/16 subtests: worker and reviewer × add/rm × help wanted, needs-decision, needs-human and question.
- Row 10: exit 0, ok internal/forgeban.
- Row 11: the pinned statusgen v1.0.54 on merged main exits 2 with COULD-NOT-CHECK ("not in the diff against 05dc9fbc88f2…"). A statusgen built from this tree inside the container exits 0, but reports 0 corroborated and 6 unchecked ("unchanged since the merge-base"), so the Expect cell's "every claim corroborated" is not met on merged main either way. Following the tool's merged-brief procedure at the authoring squash 74865cba6 with --base 74865cba6^, it exits 0 with 4 corroborated, 0 disproved and 2 unchecked (the two out-of-scope entries). As written, the row is structurally unable to pass on merged main (medici-finance/assay#1281).
- Row 13: in permits() (vocabulary.go) the default branch's return e.Owner == role was replaced with return true (the row-13 mutant in the package's mutations.json). The package suite then exited 1: TestDesklabelRefusesUnownedLabel and TestDesklabelDryRunWritesNothing failed. A second mutant, return true as the first statement of permits(), also exited 1, failing TestDesklabelRefusesUnownedLabel, TestDesklabelRefusesHumanDecidedForEveryRole and TestDesklabelDryRunWritesNothing. With the file restored from the saved copy (git status clean), the suite exits 0. CAUGHT.

Risk-bearing value enumeration. Fail-safe trigger checked: the risk metadata is present and all no, and the item is not irreversible. Enumeration scope: the desklabel package as it reads at 05dc9fbc88f2, plus the brief's Deliverables. Literals:
- roleWorker = "worker" @ tools/desk/cmd/desklabel/vocabulary.go:37; roleReviewer = "reviewer" @ :38; ownerShared = "shared" @ :48; ownerNone = "no role" @ :50; helpWantedLabel = "help wanted" @ :57.
- Owned rows: worker owns superseded? @ :112, disposition:superseded @ :114, disposition:resolved-elsewhere @ :116 and disposition:needs-rebase @ :118. Reviewer owns authorization-needed @ :122 and approval-needed @ :124. human-decided @ :128 is ownerNone.
- The shared rows come from topology.yaml labels.decision_owed: needs-decision @ topology.yaml:330, question @ :332, needs-human @ :336.
- ExitRefused = 5 @ tools/desk/internal/deskkit/exitcodes.go:51, which already existed and is unchanged.
- Ranking: none is irreversible. A wrong label is reversed by the next call from an entitled role; a wrong table row by an edit and a redeploy. The ownership rows rank top, because a mis-owned row lets one role forge another's marker.

RISK-VALUE: DERIVED — human-decided Owner = ownerNone ("no role") @ tools/desk/cmd/desklabel/vocabulary.go:128 — deskclose's decisionLabels = {"needs-decision", "human-decided"} @ tools/desk/cmd/deskclose/github.go:36 makes this label the record of a human ruling, so no role may self-apply it. permits() case ownerNone returns false regardless of role (vocabulary.go:151-152). Hand row 7: 20/20 refused.
RISK-VALUE: DERIVED — superseded? Owner = roleWorker ("worker") @ vocabulary.go:112, and the disposition rows @ :114-118 — labelProposed = "superseded?" @ tools/desk/cmd/deskclose/superseded.go:57 is the worker's proposal, which the reviewer only confirms or disputes. deskdisposition set records a worker's finding.
RISK-VALUE: DERIVED — authorization-needed / approval-needed Owner = roleReviewer ("reviewer") @ vocabulary.go:122,124 — these are labelBeforeFlip / labelAfterFlip @ tools/desk/cmd/deskflip/flip.go:115-116, the reviewer-run ready-flip pair. deskdispatch applies authorization-needed under the reviewer credential, so ownership keys on the reviewer role.
RISK-VALUE: DERIVED — helpWantedLabel = "help wanted" @ vocabulary.go:57 and the decision_owed rows (topology.yaml:330, 332, 336) are Owner = ownerShared ("shared") — they are the escalation vocabulary any role must be able to raise. A clear removes only the label and never closes an item; deskclose's close gate reads only decisionLabels.

Which rows rest on what:
- Witness: rows 9 and 12 pass. No other row has a witness pass.
- Hand runs only: rows 2-8 and 10 pass, and row 13 passes (mutant caught).
- Row 1: could-not-check both under the witness and by hand. The host and emulated environments failed in packages this item did not touch.
- Row 11: could-not-check on merged main as written. It holds only through the authoring-diff procedure.

VERIFY rows: 2 of 13 pass under the witness; 11 are could-not-run under the witness. By hand, 10 more hold (rows 2-8, 10 and 13), and rows 1 and 11 are could-not-check.

**VERIFY: BLOCKED** — no product failure observed. Every desklabel-specific obligation holds by hand on merged main 05dc9fbc88f2, but only rows 9 and 12 have a witness pass. Rows 1-8 and 10-11 need a Linux network-off witness (medici-finance/assay#1800, open). Row 11 cannot pass on merged main as written (medici-finance/assay#1281, open). Row 13 has no cmd: span, and rows 6-7 have an Expect-versus-command wording mismatch (medici-finance/assay#1927, open). All four RISK-VALUE lines are DERIVED.

### Evidence addendum — 2026-10-22 claude-opus-5-5-verifier (supersedes the tally and verdict of the 2026-10-22 re-run above)

This addendum answers review of the Evidence PR for this brief. The rule is that a prose hand run credits no row, so this addendum credits rows only from the tool-written witness table above. It supersedes the "Which rows rest on what" list, the "VERIFY rows" tally and the verdict of the 2026-10-22 re-run. Nothing above is edited.

Admission: `deskdispatch --check-verifier` exited 0 from the admitted home before this work and again after this edit, with the same binding: run=4c80fdcd481d29079a75eec8ca012bd0ac100213aabec0b089a042302942c8c2, source=05dc9fbc88f2b1b23cd78bebc8634c564327eca4, model claude-opus-5-5, tier strong (medici-finance/assay#4034).

Network-off witness attempt: one run of `statusgen verifyrun --sandbox container-netns` from the admitted home, inside a container started with `--network none` (loopback up; the only other interface, tunl0, down), using the pinned statusgen v1.0.54 linux-amd64 release binary, checksum-matched to the release checksums. It exited 2 with "pre-work verifier admission failed; no Verify rows executed", because the admission reader is not reachable inside the network-off container. No row ran, and the brief was not written. The check:ci rows therefore stay could-not-run exactly as the witness wrote them. The container route needs an admitted Linux runner, tracked in medici-finance/assay#2665 (open; its 2026-10-12 comment records that container-netns still needs admission for the exact source worktree) and medici-finance/assay#3216 (open, the same Linux network-off prerequisite).

Rows, credited only from the witness table:
- Rows 9 and 12: pass under the witness (sha256:4eff2db4bada and sha256:53c234e5e847).
- Rows 1-8 and 10: BLOCKED, could-not-run under the witness, because check:ci needs a Linux network-off sandbox (medici-finance/assay#1800, open; route: medici-finance/assay#2665, medici-finance/assay#3216, both open). Rows 6 and 7 also carry an Expect cell (exit 5) that a passing `go test` command cannot meet (medici-finance/assay#1927, open; its 2026-10-04 comment names forge-neutral/15 rows 6 and 7).
- Row 11: BLOCKED, could-not-run under the witness (medici-finance/assay#1800), and structurally unable to pass on merged main as written (medici-finance/assay#1281, open; its title names forge-neutral/15).
- Row 13: BLOCKED, could-not-run as a prose-led command with no cmd: span (medici-finance/assay#1927, open; its 2026-10-04 comment names forge-neutral/15 row 13).
- No row is FAIL.

Hand observations (supplemental, no credit). The hand notes in the 2026-10-22 re-run above were observed by hand outside an admitted witness. They are uncredited, they discharge no row, and the word "pass" in them is not a witness result. This addendum adds no hand runs.

Risk values, re-read at 05dc9fbc88f2 against the cited lines; each still matches:
RISK-VALUE: DERIVED — human-decided Owner = ownerNone ("no role") @ tools/desk/cmd/desklabel/vocabulary.go:128 — decisionLabels = {"needs-decision", "human-decided"} @ tools/desk/cmd/deskclose/github.go:36 makes it the record of a human ruling; permits() case ownerNone returns false for every role (vocabulary.go:151-152).
RISK-VALUE: DERIVED — superseded? and the disposition rows Owner = roleWorker ("worker") @ vocabulary.go:112-118 — labelProposed = "superseded?" @ tools/desk/cmd/deskclose/superseded.go:57 is the worker's proposal.
RISK-VALUE: DERIVED — authorization-needed / approval-needed Owner = roleReviewer ("reviewer") @ vocabulary.go:122,124 — labelBeforeFlip / labelAfterFlip @ tools/desk/cmd/deskflip/flip.go:115-116, the reviewer-run ready-flip pair.
RISK-VALUE: DERIVED — helpWantedLabel = "help wanted" @ vocabulary.go:57 and labels.decision_owed (topology.yaml:330, 332, 336) Owner = ownerShared ("shared") — the escalation vocabulary every role must be able to raise.
No risk value is NAMED, NOT DERIVED, so no question issue is owed.

VERIFY rows: 2 of 13 pass under the witness (rows 9 and 12); 11 are BLOCKED as could-not-run (rows 1-8, 10, 11 and 13); 0 FAIL; 0 rows credited from hand runs.

**VERIFY: BLOCKED** — no product failure observed; only rows 9 and 12 have a witness pass. Rows 1-8, 10 and 11 need an admitted Linux network-off witness (medici-finance/assay#1800, medici-finance/assay#2665, medici-finance/assay#3216, all open). Row 11 cannot pass on merged main as written (medici-finance/assay#1281, open). Row 13 has no cmd: span, and rows 6-7 have an Expect-versus-command mismatch (medici-finance/assay#1927, open). All four RISK-VALUE lines are DERIVED.

### Non-implementer verifier run — 2026-10-24 claude-opus-5-5-verifier — VERIFY: BLOCKED

Prior attempts on this brief (2026-09-17, 2026-09-22, 2026-09-27, 2026-09-30, two blocks on 2026-10-04, and the 2026-10-22 re-run plus addendum, all on earlier merged-main sources) ended held or BLOCKED; the 2026-10-22 addendum, the latest, credited only rows 9 and 12 under a witness. This block adds a new witness run at merged main b5f4fcb2bbb4 and edits nothing above.

Admission: `deskdispatch --check-verifier` exited 0 from the admitted home before any row, binding run=8dcef2228644de13e46f7f529e5874532d4666725423b04e851eced0f6d4b60f, source=b5f4fcb2bbb4c6d0af1db434ad32a47600ffd935, model claude-opus-5-5, tier strong (medici-finance/assay#4336). `tools/desk/cmd/desklabel` is byte-identical to 05dc9fbc88f2, the source of the previous attempts (`git diff --stat` empty).

Witness lanes. Rows 1-10 and 12 ran in one admission bracket (opened 2026-10-24T19:26:57Z, closed 2026-10-24T19:38:26Z) with `statusgen verifyrun --sandbox container-netns` in the pinned verify-runner v1.0.59 image (digest in the Container-Image line below; amd64, emulated on an arm64 darwin host). Rows 11 and 13 ran in a host `statusgen verifyrun --rows 11,13` run after the bracket closed, because row 11 reads git history, and the container cannot resolve the home's linked-worktree gitdir (medici-finance/assay#4327, open). Both runs had the home clean at b5f4fcb2bbb4. Host 1-minute load was 10.9 at start, 14.0 when the bracket opened, 35.5-37.2 during the container run, and 16.3-17.3 afterwards.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | could-not-run exit=- sandbox=container-netns — timed out after 10m0s — no verdict was produced | sha256:b66863aa80b0 | 2026-10-24 | assay-verifier-app[bot] @ b5f4fcb2bbb4 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/desklabel/... -count=1` | pass exit=0 sandbox=container-netns | sha256:84e7cac32555 | 2026-10-24 | assay-verifier-app[bot] @ b5f4fcb2bbb4 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -count=1 && go test ./internal/deskkit/ -run TestForgeNoPassthrough -count=1` | pass exit=0 sandbox=container-netns | sha256:41923d164f53 | 2026-10-24 | assay-verifier-app[bot] @ b5f4fcb2bbb4 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestApplyIssueLabelsBothBackends -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:69ed8754c472 | 2026-10-24 | assay-verifier-app[bot] @ b5f4fcb2bbb4 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelAppliesOwnedLabelGitHub -count=1 -v && go test ./cmd/desklabel/... -run TestDesklabelAppliesOwnedLabelGitLab -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:bded6ee7ba1c | 2026-10-24 | assay-verifier-app[bot] @ b5f4fcb2bbb4 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelRefusesUnownedLabel -count=1 -v` | fail exit=0 sandbox=container-netns | sha256:de468070268c | 2026-10-24 | assay-verifier-app[bot] @ b5f4fcb2bbb4 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelRefusesHumanDecidedForEveryRole -count=1 -v` | fail exit=0 sandbox=container-netns | sha256:b407b3b6e655 | 2026-10-24 | assay-verifier-app[bot] @ b5f4fcb2bbb4 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelSharedVocabularyAnyRole -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:458f4305601d | 2026-10-24 | assay-verifier-app[bot] @ b5f4fcb2bbb4 (on-behalf-of human:ian) (forge-identity) |
| 9 | `test -d tools/desk/cmd/desklabel && { grep -rn -e 'flag.String("as"' -e '"--as"' tools/desk/cmd/desklabel --include='*.go' \|\| [ $? -eq 1 ]; } \| { grep -v _test.go \|\| [ $? -eq 1 ]; } \| wc -l` | pass exit=0 sandbox=container-netns | sha256:9a271f2a916b | 2026-10-24 | assay-verifier-app[bot] @ b5f4fcb2bbb4 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./internal/forgeban/... -count=1` | pass exit=0 sandbox=container-netns | sha256:0338e2057521 | 2026-10-24 | assay-verifier-app[bot] @ b5f4fcb2bbb4 (on-behalf-of human:ian) (forge-identity) |
| 12 | `grep -c 'desklabel' tools/desk/README.md` | pass exit=0 sandbox=container-netns | sha256:53c234e5e847 | 2026-10-24 | assay-verifier-app[bot] @ b5f4fcb2bbb4 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:83acf2e9e54d56a42ac925ca03c966480802d027ba5f12160a25c9698569c0ac
Container-Image: sha256:c2e32c7ff99f67951174e8ff835d928d8999b1bb2ecdc7c63d3775528178640e
Verification-Attestation: medici-finance/assay#4336 run=8dcef2228644de13e46f7f529e5874532d4666725423b04e851eced0f6d4b60f source=b5f4fcb2bbb4c6d0af1db434ad32a47600ffd935 brief=docs/streams/forge-neutral/brief-15-desklabel-role-keyed.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=8dcef2228644de13e46f7f529e5874532d4666725423b04e851eced0f6d4b60f source=b5f4fcb2bbb4c6d0af1db434ad32a47600ffd935 tree=sha256:83acf2e9e54d56a42ac925ca03c966480802d027ba5f12160a25c9698569c0ac witness=sha256:173426149a8a3b5894f259405e4f49897bbcebaf261ec06b4b314e3180e59ff2 opened=2026-10-24T19:26:57Z closed=2026-10-24T19:38:26Z before=receipt after=check-verifier

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 11 | `statusgen --root . --consumers --brief forge-neutral/15` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-24 | assay-verifier-app[bot] @ b5f4fcb2bbb4 (on-behalf-of human:ian) (forge-identity) |
| 13 | `desklabel` | could-not-run exit=- — prose-led-command: not executed; the first span desklabel is a lone word mentioned ahead of the command span, not a command. Mark the command with a cmd: code span | sha256: | 2026-10-24 | assay-verifier-app[bot] @ b5f4fcb2bbb4 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4336 run=8dcef2228644de13e46f7f529e5874532d4666725423b04e851eced0f6d4b60f source=b5f4fcb2bbb4c6d0af1db434ad32a47600ffd935 brief=docs/streams/forge-neutral/brief-15-desklabel-role-keyed.md model=claude-opus-5-5 tier=strong

Rows, credited only from the two witness tables above:
- Rows 2, 3, 4, 5, 8, 9, 10 and 12: pass under the container witness.
- Row 1: could-not-run. The full `go build ./... && go test ./...` hit verifyrun's 10-minute row limit under amd64 emulation with host load above 35. This is an environment limit (could-not-check), not a product failure (medici-finance/assay#4323, open).
- Rows 6 and 7: the witness cells read `fail exit=0 sandbox=container-netns`, and verifyrun's console output for each adds `exit 0, expected 5`. The Expect cell asks for exit 5, the desklabel refusal code, but the row's command is `go test`, which exits 0 when the test that asserts exit 5 passes. A passing command cannot meet that Expect (medici-finance/assay#1927, open; its 2026-10-04 comment names forge-neutral/15 rows 6 and 7). Supplemental observation, no credit: I re-ran both commands by hand in a scratch copy of the source outside the home. Each exited 0, with 14 and 20 subtests PASS and none FAIL (the `-v` output also shows each parent test's own `--- PASS` line, which is not a subtest). Every subtest checks `code == deskkit.ExitRefused` and zero calls on the recording fake (`tools/desk/cmd/desklabel/desklabel_test.go:160,168` for row 6, `:200,206` for row 7). This is a defect in how the rows are written, not in the product.
- Row 11: could-not-run (host sandbox=unshare, darwin; medici-finance/assay#1800, open), and it cannot pass on merged main as written (medici-finance/assay#1281, open; its title names forge-neutral/15).
- Row 13: could-not-run as a prose-led command with no cmd: span (medici-finance/assay#1927, open). `statusgen verifyrun --check` still scores row 13 as fail. The likeliest cause is the 2026-09-27 witness row (`fail exit=5`, the only row-13 witness with an Output digest), because every later prose-led could-not-run row, this one included, has an empty Output digest. No row-13 run in this block produced a failure. Supplemental observation, no credit: in the same scratch copy, I replaced `return e.Owner == role` in permits()' default branch (`tools/desk/cmd/desklabel/vocabulary.go:154`) with `return true`. `go test ./cmd/desklabel/... -count=1` then exited 1, with TestDesklabelRefusesUnownedLabel and TestDesklabelDryRunWritesNothing failing. After the file was restored (byte-identical, checked with cmp), the same command exited 0.

Risk values, re-read at b5f4fcb2bbb4 against the cited lines; each still matches:
RISK-VALUE: DERIVED — human-decided Owner = ownerNone ("no role") @ tools/desk/cmd/desklabel/vocabulary.go:128 — decisionLabels = {"needs-decision", "human-decided"} @ tools/desk/cmd/deskclose/github.go:36 makes it the record of a human ruling; permits() case ownerNone returns false for every role (vocabulary.go:151-152).
RISK-VALUE: DERIVED — superseded? and the disposition rows Owner = roleWorker ("worker") @ vocabulary.go:112-118 — labelProposed = "superseded?" @ tools/desk/cmd/deskclose/superseded.go:57 is the worker's proposal.
RISK-VALUE: DERIVED — authorization-needed / approval-needed Owner = roleReviewer ("reviewer") @ vocabulary.go:122,124 — labelBeforeFlip / labelAfterFlip @ tools/desk/cmd/deskflip/flip.go:115-116, the reviewer-run ready-flip pair.
RISK-VALUE: DERIVED — helpWantedLabel = "help wanted" @ vocabulary.go:57 and labels.decision_owed (topology.yaml:330, 332, 336) Owner = ownerShared ("shared") — the escalation vocabulary every role must be able to raise.
No risk value is NAMED, NOT DERIVED, so no question issue is owed.

VERIFY rows: 8 of 13 pass under the witness (rows 2, 3, 4, 5, 8, 9, 10, 12). Rows 6 and 7 read fail only because their Expect cells cannot be met (medici-finance/assay#1927). Rows 1, 11 and 13 are could-not-run under this block's witness (`--check` tallies 8 pass, 3 fail, 2 could-not-run; its third fail is row 13, as explained above). No row is credited from a hand run.
Added by the verify desk after review: the issue links for rows 1 and 11, and the corrected rows 6 and 7 counts, quote and line cites.

**VERIFY: BLOCKED** — no product failure observed. 8 of 13 rows pass under an admitted network-off witness at b5f4fcb2bbb4. Row 1 timed out under container emulation (environment limit, medici-finance/assay#4323, open). Rows 6 and 7 have an Expect-versus-command mismatch, and row 13 has no cmd: span (medici-finance/assay#1927, open). Row 11 cannot pass on merged main as written (medici-finance/assay#1281, open).
### Non-implementer verifier run — 2026-10-27 claude-opus-5-5-verifier — VERIFY: BLOCKED

Prior attempts on this brief (2026-09-17 through the 2026-10-24 block above, all on earlier merged-main sources) ended held or BLOCKED. The latest, at b5f4fcb2bbb4, credited rows 2, 3, 4, 5, 8, 9, 10 and 12 under an admitted container witness. This block adds a fresh witness run at merged main 384d5eda2ca2 and edits nothing above.

Admission: `deskdispatch --check-verifier` exited 0 from the admitted home before any row, and again after the last row and before landing. Both runs bound run=e745d95cc5a0607b8de5f35f6343b73311e8d6d99faec43f669ccf5317e2f50d, source=384d5eda2ca230e7e748750fdaed6ba4d1d50ed8, model claude-opus-5-5, tier strong (medici-finance/assay#4709). Since b5f4fcb2bbb4, `tools/desk/cmd/desklabel` is byte-identical (`git diff --stat` empty). This brief changed only by the appended 2026-10-24 Evidence block, so the Verify table is unchanged. `tools/desk/README.md` changed, but it still names desklabel (row 12).

Witness lanes. Rows 1-10 and 12 ran in one admission bracket (opened 2026-10-27T23:16:48Z, closed 2026-10-27T23:28:11Z). They used `statusgen verifyrun --sandbox=container-netns` in the pinned verify-runner v1.0.63 image (digest in the Container-Image line below). The image is amd64, emulated on an arm64 darwin host. The container ran with `--network none`, a read-only root, all capabilities dropped and `no-new-privileges`. The home was mounted read-only, and a pre-populated Go module cache was mounted read-only, with `GOPROXY=off` and `GOTOOLCHAIN=local`. `statusgen verifyrun --bracket-digest` on the closed witness reported `verdict=match`.

Rows 11 and 13 ran on the host after the bracket closed, as `statusgen verifyrun --rows 11,13 --dry-run`. Row 11 reads git history, and the container cannot resolve the home's linked-worktree gitdir (medici-finance/assay#4327, open). The `--dry-run` mode kept the home unmodified. The host table below is that run's stdout, copied byte for byte. Both runs had the home clean at 384d5eda2ca2. Host 1-minute load was about 36 when the bracket opened and 46-53 during the container run.

Fail-first: no row is risk-bearing. Every frontmatter risk answer is `no`, and the row tags are `check:ci`, `check:ci +dereference`, `check:ci +flow`, `check` and `check +mutation`. None of these is a risk-bearing, live, mutating or end-to-end tag. So no `--fail-first` witness is owed, and none was run. For the record, the delivering merge's parent, read from the forge, is bfe06363fd73.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | could-not-run exit=- sandbox=container-netns — timed out after 10m0s — no verdict was produced | sha256:578a4f7a0df8 | 2026-10-27 | assay-verifier-app[bot] @ 384d5eda2ca2 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/desklabel/... -count=1` | pass exit=0 sandbox=container-netns | sha256:37270e1ebfde | 2026-10-27 | assay-verifier-app[bot] @ 384d5eda2ca2 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -count=1 && go test ./internal/deskkit/ -run TestForgeNoPassthrough -count=1` | pass exit=0 sandbox=container-netns | sha256:668fed6203c1 | 2026-10-27 | assay-verifier-app[bot] @ 384d5eda2ca2 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestApplyIssueLabelsBothBackends -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:daa78825daf6 | 2026-10-27 | assay-verifier-app[bot] @ 384d5eda2ca2 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelAppliesOwnedLabelGitHub -count=1 -v && go test ./cmd/desklabel/... -run TestDesklabelAppliesOwnedLabelGitLab -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:b1be571e560d | 2026-10-27 | assay-verifier-app[bot] @ 384d5eda2ca2 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelRefusesUnownedLabel -count=1 -v` | fail exit=0 sandbox=container-netns | sha256:9bd7924db95e | 2026-10-27 | assay-verifier-app[bot] @ 384d5eda2ca2 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelRefusesHumanDecidedForEveryRole -count=1 -v` | fail exit=0 sandbox=container-netns | sha256:d17507a305a4 | 2026-10-27 | assay-verifier-app[bot] @ 384d5eda2ca2 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelSharedVocabularyAnyRole -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:67e4cdbfd19f | 2026-10-27 | assay-verifier-app[bot] @ 384d5eda2ca2 (on-behalf-of human:ian) (forge-identity) |
| 9 | `test -d tools/desk/cmd/desklabel && { grep -rn -e 'flag.String("as"' -e '"--as"' tools/desk/cmd/desklabel --include='*.go' \|\| [ $? -eq 1 ]; } \| { grep -v _test.go \|\| [ $? -eq 1 ]; } \| wc -l` | pass exit=0 sandbox=container-netns | sha256:9a271f2a916b | 2026-10-27 | assay-verifier-app[bot] @ 384d5eda2ca2 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./internal/forgeban/... -count=1` | pass exit=0 sandbox=container-netns | sha256:aa3f9701103e | 2026-10-27 | assay-verifier-app[bot] @ 384d5eda2ca2 (on-behalf-of human:ian) (forge-identity) |
| 12 | `grep -c 'desklabel' tools/desk/README.md` | pass exit=0 sandbox=container-netns | sha256:1121cfccd591 | 2026-10-27 | assay-verifier-app[bot] @ 384d5eda2ca2 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:6963ad5593dc6a427eb55e64b98194a327862d2882567ed257475fd4810eb876
Container-Image: sha256:3a3cb3913f1311f1a9bf13ddd53c4f27361566f74ac90c89f78991e5452644b8
Verification-Attestation: medici-finance/assay#4709 run=e745d95cc5a0607b8de5f35f6343b73311e8d6d99faec43f669ccf5317e2f50d source=384d5eda2ca230e7e748750fdaed6ba4d1d50ed8 brief=docs/streams/forge-neutral/brief-15-desklabel-role-keyed.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=e745d95cc5a0607b8de5f35f6343b73311e8d6d99faec43f669ccf5317e2f50d source=384d5eda2ca230e7e748750fdaed6ba4d1d50ed8 tree=sha256:6963ad5593dc6a427eb55e64b98194a327862d2882567ed257475fd4810eb876 witness=sha256:17554ab6fdaf256630872193394d97ecb01f90441934ffb5ebf545d852228e3b opened=2026-10-27T23:16:48Z closed=2026-10-27T23:28:11Z before=receipt after=check-verifier

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 11 | `statusgen --root . --consumers --brief forge-neutral/15` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 384d5eda2ca2 (on-behalf-of human:ian) (forge-identity) |
| 13 | `desklabel` | could-not-run exit=- — prose-led-command: not executed; the first span desklabel is a lone word mentioned ahead of the command span, not a command. Mark the command with a cmd: code span | sha256: | 2026-10-27 | assay-verifier-app[bot] @ 384d5eda2ca2 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4709 run=e745d95cc5a0607b8de5f35f6343b73311e8d6d99faec43f669ccf5317e2f50d source=384d5eda2ca230e7e748750fdaed6ba4d1d50ed8 brief=docs/streams/forge-neutral/brief-15-desklabel-role-keyed.md model=claude-opus-5-5 tier=strong

Rows, credited only from the two witness tables above:
- Rows 2, 3, 4, 5, 8, 9, 10 and 12: pass under the container witness.
- Row 1: could-not-run. The full `go build ./... && go test ./...` hit verifyrun's 10-minute row limit under amd64 emulation, with host load around 50. This is an environment limit (could-not-check), not a product failure (medici-finance/assay#4323, open).
- Rows 6 and 7: the witness cells read `fail exit=0 sandbox=container-netns`, and for each row verifyrun's console adds `exit 0, expected 5`. The Expect cell asks for exit 5, the desklabel refusal code. The row's command is `go test`, which exits 0 when the test asserting exit 5 passes, so a passing command cannot meet that Expect (medici-finance/assay#1927, open; its 2026-10-04 comment names forge-neutral/15 rows 6 and 7). This is a defect in how the rows are written, not in the product. The 2026-10-24 block's hand observation is unchanged: every subtest asserts `deskkit.ExitRefused` and makes zero forge calls. That code is byte-identical here. Nothing was re-run by hand in this block, and nothing is credited from a hand run.
- Row 11: could-not-run (host sandbox=unshare, darwin; medici-finance/assay#1800, open). It also cannot pass on merged main as written (medici-finance/assay#1281, open; its title names forge-neutral/15).
- Row 13: could-not-run as a prose-led command with no cmd: span (medici-finance/assay#1927, open). The 2026-10-24 block's hand mutation result is unchanged and uncredited: the mutant is caught, and the code is byte-identical. Like earlier prose-led runs, this witness has an empty Output digest. So `--check` may again bind row 13 to the older 2026-09-27 `fail exit=5` witness (medici-finance/assay#4340, open).

Attempts: one container bracket and one host `--rows 11,13 --dry-run` run. None was discarded or repeated, and no hand run was made.

Risk values, re-read at 384d5eda2ca2 against the cited lines; each still matches:
RISK-VALUE: DERIVED — human-decided Owner = ownerNone ("no role") @ tools/desk/cmd/desklabel/vocabulary.go:128 — decisionLabels = {"needs-decision", "human-decided"} @ tools/desk/cmd/deskclose/github.go:36 makes it the record of a human ruling; permits() case ownerNone returns false for every role (vocabulary.go:151-152).
RISK-VALUE: DERIVED — superseded? and the disposition rows Owner = roleWorker ("worker") @ vocabulary.go:112-118 — labelProposed = "superseded?" @ tools/desk/cmd/deskclose/superseded.go:57 is the worker's proposal.
RISK-VALUE: DERIVED — authorization-needed / approval-needed Owner = roleReviewer ("reviewer") @ vocabulary.go:122,124 — labelBeforeFlip / labelAfterFlip @ tools/desk/cmd/deskflip/flip.go:115-116, the reviewer-run ready-flip pair.
RISK-VALUE: DERIVED — helpWantedLabel = "help wanted" @ vocabulary.go:57 and labels.decision_owed (topology.yaml:330, 332, 336) Owner = ownerShared ("shared") — the escalation vocabulary every role must be able to raise.
No risk value is NAMED, NOT DERIVED, so no question issue is owed.

VERIFY rows: 8 of 13 pass under the witness (rows 2, 3, 4, 5, 8, 9, 10, 12). Rows 6 and 7 read fail only because their Expect cells cannot be met. Rows 1, 11 and 13 are could-not-run.

**VERIFY: BLOCKED** — no product failure observed. 8 of 13 rows pass under an admitted network-off witness at 384d5eda2ca2. Row 1 timed out under container emulation (environment limit, medici-finance/assay#4323, open). Rows 6 and 7 have an Expect-versus-command mismatch, and row 13 has no cmd: span (medici-finance/assay#1927, open). Row 11 cannot pass on merged main as written (medici-finance/assay#1281, open), and it cannot run on this darwin host (medici-finance/assay#1800, open).
### Non-implementer verifier run — 2026-10-30 claude-opus-5-5-verifier — VERIFY: BLOCKED

The 2026-10-27 block above, at 384d5eda2ca2, credited rows 2, 3, 4, 5, 8, 9, 10 and 12. This block adds a fresh witness run at merged main 40925e8233c8 and edits nothing above.

Admission: `deskdispatch --check-verifier` exited 0 from the admitted home before any row, and again after the Evidence edit. Both runs bound run=fe71d3d185bfe737f597ff7c746a03663a96f70b1682e2d36d70fb757d534fc1, source=40925e8233c8a63d70682e77ca333e665f1f955e, model claude-opus-5-5, tier strong (medici-finance/assay#4970), with tool v1.0.66. `deskpathguard rederive` found 13 rows, 0 author-added, base 40925e8233c8, so the Verify table is the one the earlier blocks ran. `tools/desk/cmd/desklabel` has no commits since 384d5eda2ca2.

Witness lanes. Rows 1-12 ran in one admission bracket (opened 2026-10-30T03:38:01Z, closed 2026-10-30T03:44:42Z) as `statusgen verifyrun --sandbox=container-netns --rows 1,2,3,4,5,6,7,8,9,10,11,12`. The image was the pinned verify-runner v1.0.66 (digest in the Container-Image line below), native arm64 on an arm64 host, with its baked Go module cache. The container ran with `--network none`, a read-only root, all capabilities dropped, `no-new-privileges`, `GOPROXY=off`, `GOTOOLCHAIN=local` and `KUBECONFIG=/dev/null`, and with no forge token. The home and the repository's git directory were both mounted read-only, so row 11 could read history inside the container this time. `statusgen verifyrun --bracket-digest` on the closed witness reported `verdict=match`.

Row 13 ran on the host after the bracket closed, as `statusgen verifyrun --rows 13 --dry-run`. A prose-led row cannot sit in a container table: the close step refuses a table carrying a row that is not a witness row. The `--dry-run` mode kept the home unmodified. The host table below is that run's stdout, copied byte for byte. The home was clean at 40925e8233c8 for every run.

Attempts: an earlier bracket on the same binding (opened 2026-10-30T03:06:31Z) ran all 13 rows in the container without `--rows`. Its close step refused, nothing appended, because row 13 is prose-led. That witness was discarded unclosed, and nothing from it is credited. Its row results matched the bracket below row for row (rows 1, 6, 7 and 11 fail, row 13 could-not-run, the rest pass).

Fail-first: no row is risk-bearing. Every frontmatter risk answer is `no`, and the row tags are `check:ci`, `check:ci +dereference`, `check:ci +flow`, `check` and `check +mutation`. So no `--fail-first` witness is owed, and none was run.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | fail exit=1 sandbox=container-netns | sha256:5c5ca02ca179 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/desklabel/... -count=1` | pass exit=0 sandbox=container-netns | sha256:9473acf877ff | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -count=1 && go test ./internal/deskkit/ -run TestForgeNoPassthrough -count=1` | pass exit=0 sandbox=container-netns | sha256:47a7e20a8a6a | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestApplyIssueLabelsBothBackends -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:429cc29f7660 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelAppliesOwnedLabelGitHub -count=1 -v && go test ./cmd/desklabel/... -run TestDesklabelAppliesOwnedLabelGitLab -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:ef0092be04c4 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelRefusesUnownedLabel -count=1 -v` | fail exit=0 sandbox=container-netns | sha256:90636fb3ac8d | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelRefusesHumanDecidedForEveryRole -count=1 -v` | fail exit=0 sandbox=container-netns | sha256:99b3339fb952 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelSharedVocabularyAnyRole -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:357f19cd73c7 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 9 | `test -d tools/desk/cmd/desklabel && { grep -rn -e 'flag.String("as"' -e '"--as"' tools/desk/cmd/desklabel --include='*.go' \|\| [ $? -eq 1 ]; } \| { grep -v _test.go \|\| [ $? -eq 1 ]; } \| wc -l` | pass exit=0 sandbox=container-netns | sha256:9a271f2a916b | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./internal/forgeban/... -count=1` | pass exit=0 sandbox=container-netns | sha256:2b5555ed982d | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 11 | `statusgen --root . --consumers --brief forge-neutral/15` | fail exit=2 sandbox=container-netns | sha256:761271292c14 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 12 | `grep -c 'desklabel' tools/desk/README.md` | pass exit=0 sandbox=container-netns | sha256:1121cfccd591 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:a661369244ed8355ed5de43fd37a5f06b998c2d984bad8d9609bf6f80f234465
Container-Image: sha256:097db7d3e20415bc7a2c524250e2ad89158ed93a31a7d8cd861e6cd38fc3b3b4
Verification-Attestation: medici-finance/assay#4970 run=fe71d3d185bfe737f597ff7c746a03663a96f70b1682e2d36d70fb757d534fc1 source=40925e8233c8a63d70682e77ca333e665f1f955e brief=docs/streams/forge-neutral/brief-15-desklabel-role-keyed.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=fe71d3d185bfe737f597ff7c746a03663a96f70b1682e2d36d70fb757d534fc1 source=40925e8233c8a63d70682e77ca333e665f1f955e tree=sha256:a661369244ed8355ed5de43fd37a5f06b998c2d984bad8d9609bf6f80f234465 witness=sha256:4d352b4b9202c71c286b2df3a78c2762b0aa0e376485e9d36ab5979157cc334b opened=2026-10-30T03:38:01Z closed=2026-10-30T03:44:42Z before=receipt after=check-verifier

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 13 | `desklabel` | could-not-run exit=- — prose-led-command: not executed; the first span desklabel is a lone word mentioned ahead of the command span, not a command. Mark the command with a cmd: code span | sha256: | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4970 run=fe71d3d185bfe737f597ff7c746a03663a96f70b1682e2d36d70fb757d534fc1 source=40925e8233c8a63d70682e77ca333e665f1f955e brief=docs/streams/forge-neutral/brief-15-desklabel-role-keyed.md model=claude-opus-5-5 tier=strong

Rows, credited only from the two witness tables above:
- Rows 2, 3, 4, 5, 8, 9, 10 and 12: pass under the container witness.
- Row 1: fail exit=1 under the container witness. The failing tests are all outside desklabel and trace to the container lane, not the product. Forge CI on 40925e8233c8 is green for the same tree (build-test, every desk test shard, desk-platform-compile). The supplemental bullet below lists them. One of them, a `deskperf` sub-build that cannot resolve modules with `GOPROXY=off`, is the module-cache gap in medici-finance/assay#4683 (open). The rest (the image's jq lacks `scan/2`; the image derives a `linux-arm64` platform that desk-tools does not publish; the 8 GB `/tmp` sits under the cache floor of a cellctl fixture; no roster inside the container) have no open issue that a read-only search found.
- Rows 6 and 7: the witness cells read `fail exit=0 sandbox=container-netns`, and verifyrun's console adds `exit 0, expected 5`. The Expect cell asks for exit 5, the desklabel refusal code. The row's command is `go test`, which exits 0 when the test asserting exit 5 passes, so a passing command cannot meet that Expect (medici-finance/assay#1927, open). This is a defect in how the rows are written, not in the product.
- Row 11: fail exit=2 under the container witness. It ran with history readable, and `statusgen --consumers` reported COULD-NOT-CHECK: the brief is not in the diff against 40925e8233c8, and the committed Evidence carries no Verification-Table marker naming the delivery. The row cannot pass on merged main as written (medici-finance/assay#1281, open).
- Row 13: could-not-run as a prose-led command with no cmd: span (medici-finance/assay#1927, open). The host witness has an empty Output digest, so `--check` may bind row 13 to an older witness (medici-finance/assay#4340, open).

Supplemental probes (labelled, discharge no row, credit nothing):
- **Row 1** (2026-10-30): `cd tools/desk && go build ./... && go test ./...` in the same container configuration, outside the bracket — expected a failing-test list; observed exit 1 with failures only in `cmd/cellctl` (TestCLINonHelpParity, TestKubeCompatParity, TestResetRetentionUsesFilesystemIdentity), `cmd/deskinbox` (TestParityWalk, TestParityHTMLLoginShapes: `jq: error: scan/2 is not defined`), `cmd/deskperf` (TestV4ReadReportFlow: `module lookup disabled by GOPROXY=off`), `cmd/deskpreflight` (TestRealFixtures, TestRealRepo_GitStorageIsOutOfScope: `statusgen --lint` REFUSED configuration with no roster), `cmd/desksourceguard` (TestDefaultPlatform: `linux-arm64` is not a published platform), `internal/deskkit` (TestOverrideIdentityResolvesRosterLogin, nil dereference after `trust-ledger: ledger truncated mid-write`) and `internal/regression` (TestReleaseGoCache, killed at its 60 s limit). `cmd/desklabel` passed.
- **Row 11** (2026-10-30): `statusgen --root . --consumers --brief forge-neutral/15` in the same container configuration, outside the bracket — expected the reason for exit 2; observed exit 2 with `COULD-NOT-CHECK: assay:assay:forge-neutral:15 is not in the diff against 40925e8233c8…` and the hint to pass `--delivery <full-sha> --base <its first parent>`.
- **Row 13** (2026-10-30): mutation demonstration on a scratch archive of 40925e8233c8 (host, `GOPROXY=off`), inserting `return true` at the top of `permits` in `tools/desk/cmd/desklabel/vocabulary.go`, then `go test ./cmd/desklabel/... -count=1` — expected the mutant caught; observed baseline exit 0, mutant exit 1 (TestDesklabelRefusesUnownedLabel, TestDesklabelRefusesHumanDecidedForEveryRole and TestDesklabelDryRunWritesNothing fail, for example `want exit 5 (refused), got 0: added: approval-needed … as the worker role`), restored exit 0.

Risk values, re-read at 40925e8233c8 against the cited lines; each still matches:
RISK-VALUE: DERIVED — human-decided Owner = ownerNone ("no role") @ tools/desk/cmd/desklabel/vocabulary.go:128 — decisionLabels = {"needs-decision", "human-decided"} @ tools/desk/cmd/deskclose/github.go:36 makes it the record of a human ruling; permits() case ownerNone returns false for every role (vocabulary.go:151-152).
RISK-VALUE: DERIVED — superseded? and the disposition rows Owner = roleWorker ("worker") @ vocabulary.go:112-118 — labelProposed = "superseded?" @ tools/desk/cmd/deskclose/superseded.go:57 is the worker's proposal.
RISK-VALUE: DERIVED — authorization-needed / approval-needed Owner = roleReviewer ("reviewer") @ vocabulary.go:122,124 — labelBeforeFlip / labelAfterFlip @ tools/desk/cmd/deskflip/flip.go:115-116, the reviewer-run ready-flip pair.
RISK-VALUE: DERIVED — helpWantedLabel = "help wanted" @ vocabulary.go:57 and labels.decision_owed (topology.yaml:330, 332, 336) Owner = ownerShared ("shared") — the escalation vocabulary every role must be able to raise.
No risk value is NAMED, NOT DERIVED, so no question issue is owed.

VERIFY rows: 8 of 13 pass under the witness (rows 2, 3, 4, 5, 8, 9, 10, 12). Rows 6 and 7 read fail only because their Expect cells cannot be met. Row 1 fails on container-lane environment gaps outside desklabel. Row 11 fails as a check-definition gap. Row 13 is could-not-run.

**VERIFY: BLOCKED** — no product failure observed. 8 of 13 rows pass under an admitted network-off witness at 40925e8233c8. Rows 6 and 7 have an Expect-versus-command mismatch, and row 13 has no cmd: span (medici-finance/assay#1927, open). Row 11 cannot pass on merged main as written (medici-finance/assay#1281, open). Row 1 fails only in packages outside desklabel, on verify-runner image gaps (medici-finance/assay#4683, open, covers one of them); forge CI is green on the same source.

### Non-implementer verifier run — 2026-10-31 claude-opus-5-5-verifier — VERIFY: BLOCKED

The 2026-10-30 block above, at 40925e8233c8, credited rows 2, 3, 4, 5, 8, 9, 10 and 12. This block adds a fresh witness run at merged main 2a3b4be18948 and edits nothing above.

Admission: `deskdispatch --check-verifier` exited 0 from the admitted home before any row, and again after the Evidence edit. Both runs bound run=cc20f21a76342df407bb7620ec32e6d756bee2b959ca18f90609d7b528fddd98, source=2a3b4be189484335a8592a80d619011d350a74c7, model claude-opus-5-5, tier strong (medici-finance/assay#5101), with tool v1.0.67. `deskpathguard rederive` found 13 rows, 0 author-added, base 2a3b4be18948, so the Verify table is the one the earlier blocks ran. `tools/desk/cmd/desklabel` has no commits since 40925e8233c8. The only change to this brief in that range is the Evidence commit 1ef1ce69e.

Witness lanes. Rows 1-12 ran in one admission bracket (opened 2026-10-31T22:47:53Z, closed 2026-10-31T23:19:46Z) as `statusgen verifyrun --sandbox=container-netns --rows 1,2,3,4,5,6,7,8,9,10,11,12 --timeout 30m`. The image was the pinned verify-runner v1.0.67 (digest in the Container-Image line below, matching the release's verify-runner-image.txt), amd64 under emulation on an arm64 host. The container ran with `--network none`, a read-only root, all capabilities dropped, `no-new-privileges`, `GOPROXY=off`, `GOSUMDB=off`, `GOTOOLCHAIN=local` and `KUBECONFIG=/dev/null`. The home and its git directory were mounted read-only, with a read-only Go module cache and a read-only HOME holding a copy of the roster. That module cache is a mount, not the image's baked cache, so this lane differs from the 2026-10-30 one, which ran native arm64 on the baked cache.

Row 13 ran on the host after the bracket closed, as `statusgen verifyrun --rows 13 --dry-run`. A prose-led row cannot sit in a container table: the close step refuses a table carrying a row that is not a witness row. The `--dry-run` mode kept the home unmodified. The host table below is that run's stdout, copied byte for byte. The home was clean at 2a3b4be18948 for every run.

Attempts: the first container start under this bracket was refused at admission and ran no rows, because the verifier's own shell quoting left the home unmounted, so the container's root digested to the empty digest (sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855) instead of the opened tree. The same command then ran with correct mounts under the same bracket, and that run is the witness below. The refused start is disclosed for the record. Nothing from it is credited.

Fail-first: no row is risk-bearing. Every frontmatter risk answer is `no`, and the row tags are `check:ci`, `check:ci +dereference`, `check:ci +flow`, `check` and `check +mutation`. So no `--fail-first` witness is owed, and none was run.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | fail exit=1 sandbox=container-netns | sha256:0f71a6f1a18b | 2026-10-31 | assay-verifier-app[bot] @ 2a3b4be18948 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/desklabel/... -count=1` | pass exit=0 sandbox=container-netns | sha256:11ed584f3eb0 | 2026-10-31 | assay-verifier-app[bot] @ 2a3b4be18948 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -run TestNoForgeCLIShellout -count=1 && go test ./internal/deskkit/ -run TestForgeNoPassthrough -count=1` | pass exit=0 sandbox=container-netns | sha256:e9c702ccd3f8 | 2026-10-31 | assay-verifier-app[bot] @ 2a3b4be18948 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestApplyIssueLabelsBothBackends -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:57617e6f766a | 2026-10-31 | assay-verifier-app[bot] @ 2a3b4be18948 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelAppliesOwnedLabelGitHub -count=1 -v && go test ./cmd/desklabel/... -run TestDesklabelAppliesOwnedLabelGitLab -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:318b167170e5 | 2026-10-31 | assay-verifier-app[bot] @ 2a3b4be18948 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelRefusesUnownedLabel -count=1 -v` | fail exit=0 sandbox=container-netns | sha256:fa66663ac7aa | 2026-10-31 | assay-verifier-app[bot] @ 2a3b4be18948 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelRefusesHumanDecidedForEveryRole -count=1 -v` | fail exit=0 sandbox=container-netns | sha256:390ad7ee0691 | 2026-10-31 | assay-verifier-app[bot] @ 2a3b4be18948 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/desklabel/... -run TestDesklabelSharedVocabularyAnyRole -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:666a027a42e6 | 2026-10-31 | assay-verifier-app[bot] @ 2a3b4be18948 (on-behalf-of human:ian) (forge-identity) |
| 9 | `test -d tools/desk/cmd/desklabel && { grep -rn -e 'flag.String("as"' -e '"--as"' tools/desk/cmd/desklabel --include='*.go' \|\| [ $? -eq 1 ]; } \| { grep -v _test.go \|\| [ $? -eq 1 ]; } \| wc -l` | pass exit=0 sandbox=container-netns | sha256:9a271f2a916b | 2026-10-31 | assay-verifier-app[bot] @ 2a3b4be18948 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./internal/forgeban/... -count=1` | pass exit=0 sandbox=container-netns | sha256:1522054ff019 | 2026-10-31 | assay-verifier-app[bot] @ 2a3b4be18948 (on-behalf-of human:ian) (forge-identity) |
| 11 | `statusgen --root . --consumers --brief forge-neutral/15` | fail exit=2 sandbox=container-netns | sha256:c0e52183b9d7 | 2026-10-31 | assay-verifier-app[bot] @ 2a3b4be18948 (on-behalf-of human:ian) (forge-identity) |
| 12 | `grep -c 'desklabel' tools/desk/README.md` | pass exit=0 sandbox=container-netns | sha256:1121cfccd591 | 2026-10-31 | assay-verifier-app[bot] @ 2a3b4be18948 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:c09fa1c5ee2cb408a1192399fb9f6dd4c108843f9a2400453ef776287339bae5
Container-Image: sha256:f073d208c69e1e2f5ce5b680ad536d7a3cef3c80756823b9643cab4f7d1068aa
Verification-Attestation: medici-finance/assay#5101 run=cc20f21a76342df407bb7620ec32e6d756bee2b959ca18f90609d7b528fddd98 source=2a3b4be189484335a8592a80d619011d350a74c7 brief=docs/streams/forge-neutral/brief-15-desklabel-role-keyed.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=cc20f21a76342df407bb7620ec32e6d756bee2b959ca18f90609d7b528fddd98 source=2a3b4be189484335a8592a80d619011d350a74c7 tree=sha256:c09fa1c5ee2cb408a1192399fb9f6dd4c108843f9a2400453ef776287339bae5 witness=sha256:67d66c229e6781dcb23f0b9785b87406b2aba150802553202c49865656058466 opened=2026-10-31T22:47:53Z closed=2026-10-31T23:19:46Z before=receipt after=check-verifier

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 13 | `desklabel` | could-not-run exit=- — prose-led-command: not executed; the first span desklabel is a lone word mentioned ahead of the command span, not a command. Mark the command with a cmd: code span | sha256: | 2026-10-31 | assay-verifier-app[bot] @ 2a3b4be18948 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5101 run=cc20f21a76342df407bb7620ec32e6d756bee2b959ca18f90609d7b528fddd98 source=2a3b4be189484335a8592a80d619011d350a74c7 brief=docs/streams/forge-neutral/brief-15-desklabel-role-keyed.md model=claude-opus-5-5 tier=strong

Rows, credited only from the two witness tables above:
- Rows 2, 3, 4, 5, 8, 9, 10 and 12: pass under the container witness.
- Row 1: fail exit=1 under the container witness. The witness records only the digest, so the cause below comes from the supplemental re-run, which may not match the witnessed run test for test. In that re-run nothing under `cmd/desklabel` or `internal/forgeban` failed, and the failures are environmental: package timeouts under emulation, the image's jq, and this lane's read-only HOME. Forge CI on 2a3b4be18948 reports 18 successful and 10 skipped check runs and no failure, including build-test, desk-platform-compile and build-test-shard (desk-1, desk-2, desk-3, rest). That is context and credits no row.
- Rows 6 and 7: the witness cells read `fail exit=0 sandbox=container-netns`, and verifyrun's console adds `exit 0, expected 5`. The Expect cell asks for exit 5, the desklabel refusal code. The row's command is `go test`, which exits 0 when the test asserting exit 5 passes, so a passing command cannot meet that Expect (medici-finance/assay#1927, open). This is a defect in how the rows are written, and the product is not implicated.
- Row 11: fail exit=2 under the container witness. `statusgen --consumers` reported COULD-NOT-CHECK: the brief is not in the diff against 2a3b4be18948, and the committed Evidence carries no Verification-Table marker naming the delivery. The row cannot pass on merged main as written (medici-finance/assay#1281, open).
- Row 13: could-not-run as a prose-led command with no cmd: span (medici-finance/assay#1927, open). The host witness has an empty Output digest, so `--check` may bind row 13 to an older witness (medici-finance/assay#4340, open).

Supplemental probes (labelled, discharge no row, credit nothing):
- **Row 1** (2026-10-31): `cd tools/desk && go build ./... && go test ./...` in the same container configuration, with no bracket of its own, started as the bracket closed and read the same home read-only — expected a failing-test list; observed exit 1 with `ok` for `cmd/desklabel` and `internal/forgeban`. Seven packages hit the 10m test timeout: `cmd/cellctl`, `cmd/deskevidence`, `cmd/deskwt`, `cmd/muhar`, `internal/deskkit`, `internal/regression` and `internal/reviewtest`. Before its timeout, `cmd/cellctl` failed TestCLINonHelpParity and TestKubeCompatParity (`exit want 0 got 6`). `cmd/deskinbox` failed TestParityWalk and TestParityHTMLLoginShapes (`jq: error: scan/2 is not defined`). `cmd/deskperf` failed TestV4ReadReportFlow because the sub-build could not create a cache under this lane's read-only HOME. `internal/regression` also failed TestReleaseGoCache (`signal: killed` at 60s).
- **Row 11** (2026-10-31): `statusgen --root . --consumers --brief forge-neutral/15` in the same container configuration, outside the bracket — expected the reason for exit 2; observed exit 2 with `COULD-NOT-CHECK: assay:assay:forge-neutral:15 is not in the diff against 2a3b4be189484335a8592a80d619011d350a74c7` and the hint to pass `--delivery <full-sha> --base <its first parent>`.
- **Row 13** (2026-10-31): mutation demonstration on a scratch archive of 2a3b4be18948 (host, `GOPROXY=off`), inserting `return true` at the top of `permits` in `tools/desk/cmd/desklabel/vocabulary.go`, then `go test ./cmd/desklabel/... -count=1` — expected the mutant caught; observed baseline exit 0, mutant exit 1 (TestDesklabelRefusesUnownedLabel, TestDesklabelRefusesHumanDecidedForEveryRole and TestDesklabelDryRunWritesNothing fail, for example `want exit 5 (refused), got 0: added: approval-needed on example-org/tracker#12 (issue) as the worker role`), restored exit 0. The home was not touched.

Risk values, re-read at 2a3b4be18948 against the cited lines; each still matches:
RISK-VALUE: DERIVED — human-decided Owner = ownerNone ("no role") @ tools/desk/cmd/desklabel/vocabulary.go:128 — decisionLabels = {"needs-decision", "human-decided"} @ tools/desk/cmd/deskclose/github.go:36 makes it the record of a human ruling; permits() case ownerNone returns false for every role (vocabulary.go:147-156).
RISK-VALUE: DERIVED — superseded? and the disposition rows Owner = roleWorker ("worker") @ vocabulary.go:112-118 — labelProposed = "superseded?" @ tools/desk/cmd/deskclose/superseded.go:57 is the worker's proposal.
RISK-VALUE: DERIVED — authorization-needed / approval-needed Owner = roleReviewer ("reviewer") @ vocabulary.go:122,124 — labelBeforeFlip / labelAfterFlip @ tools/desk/cmd/deskflip/flip.go:115-116, the reviewer-run ready-flip pair.
RISK-VALUE: DERIVED — helpWantedLabel = "help wanted" @ vocabulary.go:57 and labels.decision_owed (topology.yaml:331 ff.) Owner = ownerShared ("shared") @ vocabulary.go:96,101 — the escalation vocabulary every role must be able to raise.
No risk value is NAMED, NOT DERIVED, so no question issue is owed.

VERIFY rows: 8 of 13 pass under the witness (rows 2, 3, 4, 5, 8, 9, 10, 12). Rows 6 and 7 read fail only because their Expect cells cannot be met. Row 1 fails under this lane, and the supplemental re-run traces it to environment gaps outside desklabel. Row 11 fails as a check-definition gap. Row 13 is could-not-run.

**VERIFY: BLOCKED** — no product failure observed. 8 of 13 rows pass under an admitted network-off witness at 2a3b4be18948. Rows 6 and 7 have an Expect-versus-command mismatch, and row 13 has no cmd: span (medici-finance/assay#1927, open). Row 11 cannot pass on merged main as written (medici-finance/assay#1281, open). Row 1 fails in this lane on environment gaps outside desklabel, per a supplemental re-run (medici-finance/assay#4683, open, tracks the lane's module-cache gap); forge CI is green on the same source.

## Review
Gate: **model** (from frontmatter — all four risk answers are `no`; see the note in
`## Context`). Reviewer records verdict + date in the stream README table, and confirms the
vocabulary table's role assignments against the code cited in Context (not against the series
comment's paraphrase, which this brief already corrects once).
