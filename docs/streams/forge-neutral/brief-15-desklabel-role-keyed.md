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
- `tools/desk/cmd/desklabel/` (new) — `main.go`, `verbs.go` (`add`/`rm`), `vocabulary.go` (the
  role-ownership table), `forge.go` (session-role custody wiring, the deskclose/deskfile
  precedent).
- `tools/desk/internal/deskkit/forge.go` — `ApplyIssueLabels`, the one op this brief adds.
- `tools/desk/internal/deskkit/forge_github.go`, `forge_gitlab.go` — both backends implement
  it.
- `docs/streams/forge-gitlab/inventory.md` — row 38.
- `tools/desk/README.md` — the Tool reference table, `desklabel`'s row.

**Why the risk answers are all `no`.** `desklabel` mints no new custody path: it reuses
`ForgeFor`/`SetGitHubCustodyMinter` exactly as `deskclose` does (brief 13's already-human-gated
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

## Review
Gate: **model** (from frontmatter — all four risk answers are `no`; see the note in
`## Context`). Reviewer records verdict + date in the stream README table, and confirms the
vocabulary table's role assignments against the code cited in Context (not against the series
comment's paraphrase, which this brief already corrects once).
