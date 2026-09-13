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
  - "tools/desk/cmd/desklabel: fixed-here (new verb — `add`/`rm`, the vocabulary table, the
    session-role caller resolution, the GitHub/GitLab target-kind dispatch)"
  - "tools/desk/internal/deskkit/forge.go, forge_github.go, forge_gitlab.go: fixed-here (one op
    added — `ApplyIssueLabels` — both backends, consumed by desklabel in this same change;
    `ApplyLabels` itself is UNCHANGED, no signature edit, no existing caller touched)"
  - "docs/streams/forge-gitlab/inventory.md: fixed-here (row 38)"
  - "tools/desk/README.md: fixed-here (desklabel's row in the Tool reference table — this
    repo's desk-verbs index)"
  - "tools/desk/internal/forgeban/allowlist.go: out-of-scope (no permit row exists for a
    verb that did not exist before this brief; the ratchet does not move)"
  - "tools/desk/cmd/deskclose, deskdisposition, deskflip, deskdispatch: out-of-scope (each
    keeps applying its OWN fixed labels exactly as it does today through its own already-wired
    forge call; desklabel is an ADDITIONAL, narrower verb for a role acting on a label outside
    those fixed flows — e.g. clearing a stale `superseded?` after a dispute — not a replacement
    for any of their internal label writes)"
version: 1
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
| 9 | check | `grep -rn -e 'flag.String("as"' -e '"--as"' tools/desk/cmd/desklabel --include='*.go' \| grep -v _test.go \| wc -l` | prints `0` — no role-override flag exists; the acting role is read from the session only |
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

## Review
Gate: **model** (from frontmatter — all four risk answers are `no`; see the note in
`## Context`). Reviewer records verdict + date in the stream README table, and confirms the
vocabulary table's role assignments against the code cited in Context (not against the series
comment's paraphrase, which this brief already corrects once).
