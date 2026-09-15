---
brief: assay:assay:forge-neutral:16
title: deskclose widened lanes — author-App self-withdraw, verifier reopen+close on verify-gate, and manifest as the documented human-ruled batch lane
why: >-
  `deskclose`'s mode set is deliberately closed to the four lanes R-1 grants
  (`docs/streams/forge-neutral/brief-13-write-verbs-c-deskpr-deskfile-deskclose.md` re-seated
  it onto the forge resolver; nothing in that brief widened what it may DO). Three gaps have
  since shown up in real operation: an author-App with a superseded or abandoned draft has no
  way to withdraw its OWN proposal without a human closing it by hand; a verifier re-running a
  `verify-gate` cycle has no way to toggle the card's state as part of that cycle; and the
  `manifest` mode — already the batch lane, per `deskclose --help` — has never been written down
  as the SANCTIONED path for a human-ruled batch, so operators keep re-deriving whether it is safe
  to use for that. This brief specifies all three, each scoped to be no wider than the identity
  and control structure already in place: it grants no new AUTHORITY, only new WHENs for
  authority the roster and the forge already assign.
wave: 4
depends: ["forge-neutral/13"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
gate-why: >-
  All three lanes are widened WITHIN the identity model `forge-neutral/13` already put a human
  gate on, not past it. (a) an App closing its own draft PR is bounded by the same
  login-AND-numeric-id pin `authority.go` already uses for the blessing authority
  (`IsBlessAuthorityIDStrict`) — applied here to the App's OWN roster-bound identity, never a
  caller-supplied one — so the widening cannot let the App act on anyone else's item. (b) the
  verifier's reopen+close is scoped to the `verify-gate` label AND backed by a SERVER-SIDE,
  independent control this brief does not touch and is forbidden from touching
  (`.github/workflows/verify-gate-close.yml:113-125` unconditionally reopens any close whose
  `sender.type` is not `User` or whose login is off the human allowlist) — so even a defect in
  this lane's own gate cannot let a bot complete the human sign-off; the human-only surface stays
  human-only for the reason the design principle below names, not because deskclose declines to
  try. (c) is a documentation change to an ALREADY-SHIPPED lane whose authorization (a
  blessing-authority comment carrying the row set's digest, `authority.go:254-276`) is unchanged
  by this brief. No new identity is trusted, no existing refusal is loosened, and the one new
  server-reaching write (`ReopenIssue`) is reversible by construction (a close undoes it). A human
  reviewer confirms these boundaries hold; a model can verify the code proves them (rows 1-17).
domain: complicated
issues: [984, 992]
schema: brief-v2
authored: 2026-09-13 by forge-neutral authoring session
sources:
  - "docs/streams/forge-neutral/brief-13-write-verbs-c-deskpr-deskfile-deskclose.md — the resolver
    re-seat this brief builds on: `mintedRole`, `forgeForFn`, `RoleAppLogin`, and the `item` shape
    all come from it unchanged"
  - "tools/desk/cmd/deskclose/main.go:1-40 — the two-gate design (`THE RULING GATE`, `THE MANIFEST
    GATE`) this brief adds a THIRD kind of gate beside, never inside: self-withdraw and
    verify-gate-refire authorize on IDENTITY + STRUCTURE, not on a fetched human artifact, because
    neither closes another party's item"
  - "tools/desk/cmd/deskclose/github.go:11-13 — \"There is no merge verb, no reopen verb and no
    edit verb anywhere in this package\" — true as of `forge-neutral/13`; this brief is the first
    to add a reopen, and states why that line's invariant survives in spirit (the reopen is
    always paired with a close in the SAME narrow lane, never a free-standing capability)"
  - "#984 — a live confirm-path defect in the EXISTING two-role superseded lane (`--by` ref-form
    mismatch reads as a disagreement; `deskdisposition read`'s `gh` subprocess 401s under
    deskclose's ambient-isolated shell) that stalled a real reviewer confirm. A fix is proposed in
    #991 (open, not yet merged at authoring time). Cited as prior art for why this brief specifies
    each new lane's negative paths as carefully as its happy path: a lane that is right on the
    forge-neutral migration's transport but wrong on one string-comparison or one subprocess
    environment variable still blocks the human it exists to relieve."
  - "tools/desk/internal/deskkit/trust.go:151 — `VerifyGateLabel = \"verify-gate\"`; the label lane
    (b) is scoped to"
  - "tools/desk/internal/deskkit/roletoken.go:40 — `loopTokenRoles[\"verify-desk\"] = \"verifier\"`,
    the loop-to-role binding that gives a verify-desk session the `verifier` role this brief keys
    lane (b) on"
  - ".github/workflows/verify-gate-close.yml:85-130 — the human-allowlist close guard: ANY close of
    a `verify-gate`-labelled issue by a non-`User` sender or an off-allowlist login is reopened and
    commented on, unconditionally. This is the independent, server-side layer lane (b)'s gate-why
    cites and this brief must not weaken (a Ground Rule below forbids touching
    `.github/workflows/*` at all)"
  - "tools/desk/internal/deskkit/forgeidentity.go:37-52 — `BotIdentity{Forge, ForgeInferred, Slug,
    ID}`; `ID` is the roster-pinned numeric bot user id, 0 when unpinned — the same shape
    `authority.go`'s `IsBlessAuthorityIDStrict` pins a HUMAN by; lane (a) pins an APP by it"
  - "tools/desk/internal/deskkit/forge.go:864-865 — `CloseIssue(repo, number, stateReason)` is the
    only lifecycle-write op on the frozen `Forge` interface today; no `ReopenIssue` exists"
  - "docs/streams/forge-gitlab/inventory.md — the frozen op table, 37 rows on current main; this
    brief appends row 38"
  - "freshness-checked 2026-09-13 @ this branch's base — `grep -n Reopen tools/desk/internal/deskkit/forge.go
    tools/desk/internal/deskkit/forge_github.go tools/desk/internal/deskkit/forge_gitlab.go`
    returns nothing; `grep -n 'no reopen verb' tools/desk/cmd/deskclose/github.go` returns
    `github.go:13`; `.github/workflows/verify-gate-close.yml` lines 85-130 read as quoted above"
exec-tier: strong
exec-tier-why: "each lane adds a NEW way something may be closed, and the whole package's safety
  property is that every closure traces to an authority deskclose fetched and verified. A subtle
  error here — a login check with no id pin, a role check that admits an unresolved role, a label
  check matched case-sensitively against a forge that lowercases labels — reads as a working
  feature in every happy-path test and widens who may close what."
consumers:
  - "tools/desk/internal/deskkit/forge.go: follow-up forge-neutral/16 (`ReopenIssue` is added to the
    frozen seam, both backends, consumed by lane (b) — this is a docs-only authoring PR; the code
    lands in this brief's own implementation phase)"
  - "tools/desk/cmd/deskclose: follow-up forge-neutral/16 (two new modes — `self-withdraw`,
    `verify-gate-refire` — plus the manifest documentation; the existing
    `duplicate`/`superseded`/`review-request` behavior stays UNCHANGED when that implementation
    lands; this authoring PR carries no code)"
  - "docs/streams/forge-gitlab/inventory.md: follow-up forge-neutral/16 (row 38, appended when
    `ReopenIssue` ships, not by this authoring PR)"
  - ".github/workflows/verify-gate-close.yml: out-of-scope (MUST NOT CHANGE — it is the independent
    server-side layer lane (b)'s safety argument depends on; a Ground Rule below forbids touching
    any `.github/workflows/*` file)"
  - "docs/streams/forge-neutral/brief-13-write-verbs-c-deskpr-deskfile-deskclose.md: fixed-here
    (the `unblocks:` edge `newbrief` wrote there is graph housekeeping for this brief's `depends:`,
    and IS part of this authoring PR's diff; no other content in brief 13 changes)"
version: 1
id: 9abfee7a-f0af-48d7-89aa-e36cf5d67ddb
---

# Brief 16 — deskclose widened lanes

## Design principle (quoted verbatim, per the series tracking issue #992)

> every desk action a human does through the forge CLI today gets a desk verb with a
> forge-neutral mapping; the human-only ones stay human-only because SERVER-SIDE permissions
> make them so, not because the desk lacks a verb.

Lane (b) below is this principle in its most literal form yet in the series: the desk GAINS a
verb (verifier reopen+close on a `verify-gate` issue), and the human-only surface it looks like it
touches — completing the sign-off that flips a brief `verified → done` — stays human-only anyway,
because `.github/workflows/verify-gate-close.yml` enforces that server-side and this brief does
not touch that file. The verb is real; the authority it cannot grant itself is also real, and nailing
down which is which is most of this brief's content.

## Context

files:
- `tools/desk/cmd/deskclose/verbs.go` — the closed mode set (`modes()`, `rowModes()`), the
  `splitPositionals`/`common`/`closeReq` shapes, `applyClose`.
- `tools/desk/cmd/deskclose/authority.go` — `gateFor`/`authorize` (the R-1 ruling gate),
  `IsBlessAuthorityIDStrict`'s login+id pin pattern this brief mirrors for App identity.
- `tools/desk/cmd/deskclose/superseded.go` — `resolveCaller`'s three-answer shape
  (could-not-check / refused / a role), which lane (a) and (b) each reuse in their own narrower
  form; `roleWorker`/`roleReviewer` constants sit beside the new `roleVerifier` this brief adds.
- `tools/desk/cmd/deskclose/forge.go` — `mintedRole`, `forgeForFn`, `githubCustodyMint`: the
  already-minted session-role App token both new lanes act under. Neither lane changes custody.
- `tools/desk/cmd/deskclose/github.go` — `fetchItem`/`postComment`/`closeItem`, and the package
  doc's *"no merge verb, no reopen verb and no edit verb"* line this brief is the first to revise
  (a `reopenItem` helper is added beside `closeItem`, never a general one — it has exactly one
  caller, the verify-gate-refire mode).
- `tools/desk/cmd/deskclose/disposition.go` — `requireTerminalDisposition`, which lane (a)
  deliberately does NOT call (see facts below).
- `tools/desk/cmd/deskclose/manifest.go` — `authorizeManifest`, `manifestApprovalPhrase`,
  `Digest()`: unchanged code, the subject of lane (c)'s documentation.
- `tools/desk/cmd/deskclose/main.go` — the `usage` const; each new mode is added to it, and the
  manifest line gains the "documented human-ruled BATCH lane" phrasing lane (c) asks for.
- `tools/desk/internal/deskkit/forge.go`, `forge_github.go`, `forge_gitlab.go` — `ReopenIssue`
  added to the frozen seam (both backends).
- `docs/streams/forge-gitlab/inventory.md` — row 38 appended.

single-point-of-failure: for lane (a) the single control is the AUTHORSHIP PIN — get it wrong
and an App can close a draft it did not open. It is backed by a second, independent layer: the
backend's own write scope (an installation token can only write where its App is installed; a
close attempt against a repo the App has no access to 404s before the pin is even relevant) and
the unchanged decision-label absolute refusal (`refuseDecisionItem`, called from every mode
including these two). For lane (b) the single control is the ROLE+LABEL pin, and its independent
second layer is NOT in this codebase at all — it is the server-side allowlist guard in
`verify-gate-close.yml`, which this brief is forbidden from touching and which trips on a
completely different signal (`github.event.sender.type`, read by GitHub itself from the token
that made the API call) than anything deskclose's own gate computes. A defect in deskclose's role
check and a defect in the GitHub Actions guard would have to fail in the SAME direction, in two
different systems maintained by two different parties, for the human sign-off to be
completable by a bot.

facts:
- The closed mode set is enumerated in exactly one place (`verbs.go:17-26`, `modes()`) and
  echoed by the dispatcher's `default` arm (`main.go:163-168`, *"the mode set is CLOSED"*) and the
  `usage` const. A new mode must appear in all three or the dispatcher's refusal message
  (`modeList()`) would lie about what actually runs.
- `rowModes()` (`verbs.go:30-33`) is deliberately narrower than `modes()` — `manifest` cannot
  contain a manifest row, "a recursion with a human authorization at only one level of it." This
  brief's two new modes are ALSO excluded from `rowModes()`: neither is a human-ruled batch
  primitive, so neither belongs in a manifest row. `parseManifest`'s row-mode validator
  (`manifest.go:159-161`) enforces this from the same list, so the exclusion is structural, not a
  second list to keep in sync.
- `applyClose`'s `verifyLane` (`verbs.go:354-419`) calls `requireTerminalDisposition` for EVERY
  PR-mode close (`verbs.go:369-378`) — a worker's recorded finding that this PR is done with. That
  precondition is right for `superseded`/`review-request`, where deskclose closes SOMEONE ELSE'S
  work on a third party's finding. It is the wrong shape for lane (a): an author withdrawing its
  OWN unreviewed draft is not acting on a worker's finding about the PR's fate: it is exercising
  the same authority a human already has over their own PR, no finding required. Lane (a) is
  therefore NOT implemented as a new case inside `verifyLane`/`applyClose`; it is its own function
  (`applySelfWithdraw`) sharing `postComment`/`closeItem`/`refuseDecisionItem`/`allowWrite`, but
  running its OWN precondition chain, described in Task 2.
- `gateFor`/`authorize` (`authority.go:220-239`) is the R-1 ruling gate, called once per PROCESS
  (`cachedGrant`, `verbs.go:244-256`) from every existing mode. R-1 grants the desk authority to
  close OTHER people's items in narrow lanes. Lane (a) and lane (b) do not exercise that grant —
  neither closes an item belonging to someone other than the acting identity's own established
  authority over it (its own draft; a process card the desk's own CI files) — so NEITHER calls
  `gateFor`. This is a structural claim, not an oversight, and Task 2/3 each carry a negative-path
  Verify row proving R-1 being UNSIGNED does not block either lane.
- `Issue.Author` and `PullRequest.Author` are both `Account{Login, ID}` (`forge.go:62-65,84`).
  `PullRequest` already carries `Draft`, `Labels` and `State` (`forge.go:74,96-100`) in the SAME
  read `GetPullRequest` returns — lane (a) needs exactly one forge read, not two, and adds no new
  `Forge` operation.
- `RoleAppLogin(role)` returns the rendered login only (`trust.go:517-527`); the numeric id lives
  in the roster's `BotIdentity` (`forgeidentity.go:37-52`, `ID int64 // 0 when unpinned`), reached
  via `Config.RoleBotIdentity(role)` (`forgeidentity.go:203`). `RoleBotCommitIdentity` already
  reads BOTH fields together to build a commit address (`trust.go:551-569`); lane (a)'s authorship
  pin is the same two-field read used for a COMPARISON instead of a construction — login for the
  cheap check, id for the one that cannot be spoofed by a same-named account (the exact shape
  `IsBlessAuthorityIDStrict` already uses for the human blessing authority, `authority.go:209-216`).
- `SameActor` (`prstate.go:142-146`) normalizes the `[bot]`/`app/` renderings before comparing —
  the same function `superseded.go`'s two-role check already uses to keep a worker from proposing
  and confirming under renderings of the same App. Lane (a) reuses it rather than a second
  string-compare.
- `loopTokenRoles["verify-desk"] = "verifier"` (`roletoken.go:40`) already exists; a verify-desk
  session's minted App role IS `verifier` today, unused by deskclose (`resolveCaller`'s `switch
  mintedRole` in `superseded.go:124-136` only matches `roleReviewer`/`roleWorker`, refusing
  anything else — a verifier session hitting `deskclose superseded` today gets that refusal, and
  correctly so: the two-role lane is not this brief's business). Lane (b) is the first deskclose
  code path a `verifier` role identity may run.
- `VerifyGateLabel = "verify-gate"` (`trust.go:151`) is the ONE label lane (b) is scoped to.
  Nothing about this brief widens what counts as a `verify-gate` issue or how it gets that label —
  `verify-gate-open.yml` still owns filing it.
- `.github/workflows/verify-gate-close.yml:113-125` reopens and comments on ANY close of a
  `verify-gate`-labelled issue whose `sender.type != "User"` or whose login is off
  `ASSAY_BLESS_LOGIN`, and fails the job non-zero so no later step advances a brief to `done`. The
  job's `if:` (line ~46, `contains(github.event.issue.labels.*.name, 'verify-gate')`) fires on
  EVERY such issue regardless of who filed it, so this guard reaches whatever issue lane (b)'s
  close touches, not only the one card `verify-gate-open.yml` itself files. `RoleAppLogin`'s
  rendering (`<slug>[bot]`) is a bot-shaped login by construction, so `sender.type` for a
  deskclose-driven close is always `Bot` — this lane's close can therefore NEVER pass that guard.
  Lane (b) is written knowing this, not despite it: see the design-principle note above.
- Manifest's authorization (`authority.go:254-276`, `authorizeManifest`) already requires an
  `authorized-by:` comment authored by the roster-pinned blessing authority AND carrying the
  manifest's own content digest (`Digest()`, `manifest.go:45-51`, covering issue/mode/target/mined
  per row). When that comment IS the ruling's own sign-off — the ordinary case for "many stale or
  duplicate items closed under one ruling" — the ruling's URL is not a separate field to add: the
  `authorized-by:` URL already names the exact comment doing double duty as the ruling's sign-off
  AND the batch's authorization, and the digest binds it to this exact row set. Nothing here is
  under-specified; it has never been written down as the intended shape for that scenario, which
  is lane (c)'s whole deliverable.
- `TestNoForceEscapeHatch` (referenced in `main.go:36`, *"THERE IS NO --force, NO --yes, AND NO
  ENVIRONMENT OVERRIDE"*) pins the package-wide invariant. Both new modes add zero flags of that
  shape; Task 2 and Task 3 each name their full flag surface so this is checkable by inspection.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Do NOT touch any file under `.github/workflows/`. Lane (b)'s entire safety argument is that
  `verify-gate-close.yml` is untouched and independent; a diff that edits it invalidates this
  brief's gate-why and must be split into a separately-reviewed change.
- No new mode may add a `--force`, `--yes`, environment override, or any other flag that lets a
  refusal be bypassed rather than satisfied.
- `duplicate`, `superseded`, `review-request` and `manifest`'s existing behavior is UNCHANGED.
  Every existing test in the package stays green unmodified.

## Task

### 1. Add `ReopenIssue` to `Forge`, both backends, consumed by lane (b) in this change
`ReopenIssue(repo ForgeRepo, number int) error` — GitHub `PATCH /repos/{o}/{r}/issues/{n}`
`{"state":"open"}`; GitLab `PUT /projects/:id/issues/:iid` `state_event=reopen`. Mirrors
`CloseIssue`'s shape (`forge.go:864-865`) exactly, minus a state-reason parameter (GitHub's
`state_reason` is a close-time field only; reopening clears it). Record it in
`docs/streams/forge-gitlab/inventory.md` as row 38, and give it a both-backend golden contract
case beside `CloseIssue`'s. This is the only new operation this brief adds — lanes (a) and (c)
need none.

### 2. `self-withdraw` — author-App self-withdraw on its own superseded/abandoned draft
```
deskclose self-withdraw -R <owner/repo> <N> --because {superseded|abandoned} [--by <ref>]
```
- `--because superseded` requires `--by <ref>` (the item that replaces this one — recorded in the
  close comment, NOT verified merged: unlike `superseded`/`review-request`, this is the author's
  own claim about its own item, the same as a human closing their own PR needs no second party to
  confirm the reason). `--because abandoned` refuses a `--by`.
- `applySelfWithdraw`'s precondition chain, in order (mirroring `applyClose`'s fixed-order
  comment, `verbs.go:263-276`, but its OWN chain, not a branch inside `verifyLane`):
  1. `GetPullRequest` (not `GetIssue` — this lane only ever acts on a PR; an issue number is a
     could-not-check "not a pull request" refusal, since `GetPullRequest` on an issue number
     fails to resolve).
  2. Already closed → idempotent no-op (unchanged pattern).
  3. `refuseDecisionItem` against the read's own `Labels` — UNCHANGED, absolute, in this mode too.
  4. Not a draft (`!pr.Draft`) → refused: *"self-withdraw closes a DRAFT of your own authorship;
     a PR out for review is not this lane's business."*
  5. Authorship pin, BOTH conditions: `deskkit.SameActor(pr.Author.Login, RoleAppLogin(mintedRole))`
     AND `pr.Author.ID == RoleBotIdentity(mintedRole).ID` (id must be nonzero — an unpinned roster
     id is a could-not-check refusal, never a login-only pass). Fails either → refused, naming the
     PR's actual author and the acting role, never silently trying the other.
  6. Comment (states `--because`, `--by` if present, and explicitly: *"no ruling artifact is cited
     — this is the item's own author withdrawing its own proposal, the same authority a human
     already has over their own pull request"*) → close (`state_reason` "not planned").
- `gateFor`/`authorize` (R-1) is NEVER called by this mode. This is load-bearing, not an oversight
  — see facts above — and Verify row 6 proves it with R-1 unsigned.
- Add `self-withdraw` to `modes()`/the dispatcher/the usage text; do NOT add it to `rowModes()`.

### 3. `verify-gate-refire` — verifier reopen+close on `verify-gate`-labelled issues
```
deskclose verify-gate-refire -R <owner/repo> <N> --reason <text>
```
- Role gate: `mintedRole == "verifier"` (via `RoleAppLogin("verifier")`, same could-not-check /
  refused / accepted three-way shape `resolveCaller` already uses, `superseded.go:99-137`, but
  keyed on ONE role, not two — any other resolved role, including `roleWorker`/`roleReviewer`, is
  refused by name).
- Label gate: the item must carry `deskkit.VerifyGateLabel` (`"verify-gate"`) — absent is refused:
  *"verify-gate-refire is not a general reopen tool; it acts only on issues already carrying
  verify-gate."*
- State gate: the item must currently be CLOSED — an already-open item has nothing to re-fire
  (idempotent no-op, not a refusal: a retried cycle should not fail on the second call).
- `--reason` is mandatory free text, echoed into both the reopen comment and the re-close comment,
  so the audit trail states why THIS cycle ran without depending on the reader inferring it from
  context.
- Action, in order: `ReopenIssue` → `postComment` (states the reason, the acting verifier login,
  and explicitly: *"this reopen+close is a verify-gate re-fire cycle, not the human sign-off — only
  an allowlisted human's own close advances this brief to done; a bot's close of a verify-gate
  issue is unconditionally reopened by verify-gate-close.yml"*) → `closeItem` with NO state reason
  (mirrors `closeItem`'s existing `isPR` carve-out, `github.go:186-189` — an issue could still
  carry one, but this cycle states no disposition of its own, only that the marker needed to
  re-fire).
- `gateFor`/`authorize` (R-1) is NEVER called by this mode either, for the same structural reason
  as lane (a): a `verify-gate` card is a machine-filed process artifact, not another party's item
  whose retirement needs a human ruling — and the close leg cannot complete the human sign-off
  regardless of what deskclose decides, because `verify-gate-close.yml` decides that
  independently. Verify row 10 proves R-1-unsigned does not block this mode; verify row 8 proves
  the server-side guard is what actually keeps the sign-off human-only, by reading the workflow's
  own condition rather than by trusting this brief's prose.
- Add `verify-gate-refire` to `modes()`/the dispatcher/the usage text; do NOT add it to
  `rowModes()`.
- Add a `reopenItem` helper beside `closeItem` in `github.go` (one caller: this mode). Update the
  package doc's *"no merge verb, no reopen verb and no edit verb"* line
  (`github.go:11-13`) to state the one narrow exception and why it does not open a general
  capability: the reopen has exactly one caller, is always followed by a close in the same
  invocation, and is role+label scoped to a surface the desk cannot unilaterally complete a
  sign-off on regardless.

### 4. Document `manifest` as the sanctioned human-ruled BATCH lane
No behavior changes; `authorizeManifest`, `Digest()` and the row grammar are untouched.
- Add one sentence to the `manifest` line of `main.go`'s `usage` const naming it explicitly: *"the
  documented human-ruled BATCH lane — many items, one recorded ruling, one digest-bound
  authorization."*
- In this brief's own prose (this file) and, if the implementer judges it useful, a short addition
  to `docs/streams/forge-neutral/README.md` or a sibling doc: a GENERIC worked shape (no repo or
  incident names) —

  > A human ruling retires N stale-or-duplicate items in one sitting (a naming migration is
  > superseded by its final PR; a batch of watch-and-reject entries expires together). The human
  > states the ruling once, in a single forge comment, and that SAME comment becomes the
  > manifest's `authorized-by:` — its permalink names the ruling, and `deskclose manifest`'s own
  > digest check (`Digest()` over issue/mode/target/mined, `manifest.go:45-51`) binds it to
  > exactly the row set the human saw. There is no second "ruling URL" field to add: the
  > authorizing comment already IS the ruling's own artifact when the human writes it as one, and
  > `authorizeManifest` already refuses a batch whose digest does not match what that comment
  > carries — a row added, retargeted or re-moded after the human looked invalidates the
  > authorization outright (`authority.go:245-250`).
- No frontmatter change to this brief is warranted for (c) alone: the manifest's existing
  ruling-URL/digest requirement is the control, unchanged, and documenting an existing control
  introduces no new risk to gate.

## Verify (executable — no prose-only DoD items)

This brief follows brief-13's `Class`-column convention (statusgen v1.0.3 mistake-proofing/03):
`+dereference` resolves a claim rather than counting its presence; `+flow` exercises a
cross-component path end to end; `+mutation` proves a control reddens when broken. These names
below are this brief's planned test deliverables, created by the implementer:
`TestSelfWithdrawSucceedsOnOwnDraft` (planned),
`TestSelfWithdrawRefusesNonAuthor` (planned), `TestSelfWithdrawRefusesNonDraft` (planned),
`TestSelfWithdrawRefusesDecisionLabelled` (planned), `TestSelfWithdrawIDPinRejectsLoginOnlyMatch` (planned),
`TestSelfWithdrawIgnoresUnsignedRuling` (planned), `TestVerifyGateRefireSucceedsAsVerifier` (planned),
`TestVerifyGateRefireRefusesNonVerifier` (planned), `TestVerifyGateRefireRefusesUnlabelledItem` (planned),
`TestVerifyGateRefireIgnoresUnsignedRuling` (planned), `TestReopenIssueOpBothBackends` (planned).

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/desk && go build ./... && go test ./...` | exit 0 |
| 2 | check:ci | `cd tools/desk && go test ./cmd/deskclose/... -count=1` | exit 0 — `duplicate`/`superseded`/`review-request`/`manifest` unmodified AND green alongside the two new modes |
| 3 | check:ci +flow | `cd tools/desk && go test ./cmd/deskclose/... -run TestSelfWithdrawSucceedsOnOwnDraft -count=1 -v` | exit 0 — the authoring App closes its own draft PR (`--because abandoned`, and separately `--because superseded --by <ref>`); asserts exactly one `PostComment` then one `CloseIssue`, zero calls against any OTHER item |
| 4 | check:ci | `cd tools/desk && go test ./cmd/deskclose/... -run TestSelfWithdrawRefusesNonAuthor -count=1 -v` | **negative path**: the same PR, a DIFFERENT minted role attempting self-withdraw → refused, ZERO forge writes (recording fake shows no `PostComment`/`CloseIssue`/`ReopenIssue` calls) |
| 5 | check:ci | `cd tools/desk && go test ./cmd/deskclose/... -run TestSelfWithdrawRefusesNonDraft -count=1 -v` | **negative path**: the authoring App's OWN pull request, `Draft: false` → refused, zero writes |
| 6 | check:ci | `cd tools/desk && go test ./cmd/deskclose/... -run TestSelfWithdrawRefusesDecisionLabelled -count=1 -v` | **negative path**: the authoring App's own draft, but carrying `needs-decision` → refused by the SAME `refuseDecisionItem` every other mode uses, before the authorship check runs |
| 7 | check:ci +dereference | `cd tools/desk && go test ./cmd/deskclose/... -run TestSelfWithdrawIDPinRejectsLoginOnlyMatch -count=1 -v` | **negative path**: `pr.Author.Login` matches `RoleAppLogin(mintedRole)` but `pr.Author.ID` does NOT match the roster's pinned bot id (or the roster id is 0/unpinned) → refused. Proves the pin is login-AND-id, not login-only — the exact shape `IsBlessAuthorityIDStrict` already enforces for the human blessing authority, mirrored here for the App's own identity |
| 8 | check:ci | `cd tools/desk && go test ./cmd/deskclose/... -run TestSelfWithdrawIgnoresUnsignedRuling -count=1 -v` | **negative-path-of-a-different-kind**: with R-1 UNSIGNED (or the rulings file absent), a genuine self-withdraw on the author's own draft still SUCCEEDS — proving `gateFor`/`authorize` is never called by this mode, structurally, not by accident |
| 9 | check:ci +flow | `cd tools/desk && go test ./cmd/deskclose/... -run TestVerifyGateRefireSucceedsAsVerifier -count=1 -v` | exit 0 — a `verifier`-role session, a CLOSED item carrying `verify-gate`: asserts `ReopenIssue` then `PostComment` then `CloseIssue`, in that order, on the SAME item number, with `--reason` present in both comment bodies |
| 10 | check:ci | `cd tools/desk && go test ./cmd/deskclose/... -run TestVerifyGateRefireRefusesNonVerifier -count=1 -v` | **negative path**: `mintedRole` = `worker` or `reviewer` (or unresolved) on the same item → refused / could-not-check per role; zero `ReopenIssue`/`CloseIssue` calls |
| 11 | check:ci | `cd tools/desk && go test ./cmd/deskclose/... -run TestVerifyGateRefireRefusesUnlabelledItem -count=1 -v` | **negative path**: a verifier session, a closed item WITHOUT `verify-gate` → refused naming the missing label; zero writes |
| 12 | check:ci | `cd tools/desk && go test ./cmd/deskclose/... -run TestVerifyGateRefireIgnoresUnsignedRuling -count=1 -v` | with R-1 UNSIGNED, a genuine verifier re-fire on a verify-gate-labelled closed item still SUCCEEDS — `gateFor` is never called by this mode either |
| 13 | check | `grep -cE -e 'sender\.type' -e 'gh issue reopen' .github/workflows/verify-gate-close.yml` | ≥ 2 — the independent server-side guard this brief's gate-why depends on is present and UNCHANGED (this row also serves as the "did not touch `.github/workflows/*`" spot-check; the full check is `git diff origin/main -- .github/workflows` being empty) |
| 14 | check:ci +dereference | `cd tools/desk && go test ./internal/deskkit/ -run TestReopenIssueOpBothBackends -count=1 -v && go test ./internal/deskkit/ -run TestForgeGithubGolden -count=1 && go test ./internal/deskkit/ -run TestForgeGitlabGolden -count=1 && go test ./internal/deskkit/ -run TestForgeGitlabCoverage -count=1` | exit 0 on every chained command — `ReopenIssue` pinned on both backends' wire and reconciled against inventory row 38; no generic/passthrough method added (`TestForgeNoPassthrough` implicitly covered by the golden/coverage run) |
| 15 | check | `grep -c 'self-withdraw' docs/streams/forge-gitlab/inventory.md tools/desk/cmd/deskclose/main.go; grep -c 'verify-gate-refire' docs/streams/forge-gitlab/inventory.md tools/desk/cmd/deskclose/main.go` | each grep's total across both files ≥ 1 — both new modes are named in the usage text and (for the op they introduce) the inventory |
| 16 | check +mutation | **Mutation for the self-withdraw authorship pin.** In the id-comparison this brief adds (the line comparing `pr.Author.ID` against the roster's pinned bot id), disable the id half — change it to always pass (e.g. `id == id \|\| true`-shaped) — then run `go test ./cmd/deskclose/... -run TestSelfWithdraw -count=1`; restore and re-run | exit **1** on the mutant: `TestSelfWithdrawIDPinRejectsLoginOnlyMatch` fails (a login-only match now closes a draft it did not author). exit 0 restored. Proves the id half of the pin is a LIVE control, not a comment |
| 17 | check:ci +dereference | `statusgen --root . --consumers --brief forge-neutral/16` | exit 0 — every `consumers:` routing claim is corroborated against this branch's own diff (the `.github/workflows/verify-gate-close.yml` row is corroborated as UNCHANGED, not as touched) |

## Pre-mortem → detection map

| Failure mode of the work | Caught by |
|---|---|
| Self-withdraw closes a draft it did not author (login rendering collision, or the id check is dropped) | rows 4, 7, 16 |
| Self-withdraw fires on a non-draft PR, becoming a general self-close tool | row 5 |
| Self-withdraw skips the decision-label refusal because it is a new code path that forgot to call `refuseDecisionItem` | row 6 |
| Self-withdraw is wired through `applyClose`/`verifyLane` after all, silently requiring a `deskdisposition` record that has no business gating an author's own withdrawal | row 3 (asserts the ONLY two writes are the comment and the close — a `requireTerminalDisposition` call would surface as an extra read/refusal in the recording fake) |
| Self-withdraw or verify-gate-refire accidentally calls `gateFor`, so an unsigned R-1 blocks a lane that was never supposed to depend on it | rows 8, 12 |
| Verify-gate-refire runs under a non-verifier role because the role check falls through to a default-accept | row 10 |
| Verify-gate-refire acts on an issue with no `verify-gate` label, becoming the general reopen tool `github.go`'s doc comment says does not exist | row 11 |
| The reopen+close ORDER is wrong (close before reopen, or the two race) | row 9 asserts the exact call ORDER on a recording fake |
| This brief edits `.github/workflows/verify-gate-close.yml` to "help," quietly invalidating its own gate-why | row 13 + the Ground Rule forbidding the touch |
| `ReopenIssue` is added as a generic passthrough or grows a caller-supplied endpoint | row 14 (`TestForgeNoPassthrough` via the golden/coverage run) |
| The new modes leak into `rowModes()`, so a manifest row could invoke a self-withdraw or a verify-gate-refire under a human ruling that never named either as a granted lane | **no row** — structural: `rowModes()`'s literal (`verbs.go:33`) is reviewed by inspection; `parseManifest`'s row-mode validator already refuses any mode not in that list, so an omission here is a refusal, never a silent grant |
| Lane (c)'s documentation overstates what `manifest` does — implies a NEW ruling-URL field exists when none was added | **no row** — review-only: the Review gate reads the added prose against `authority.go:254-276` unchanged |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **model** (from frontmatter — all four risk answers `no`; see gate-why for the reasoning that
each widened lane stays inside the identity/authority model `forge-neutral/13` already put a human
gate on, rather than opening a new one).

Core-system reviewer questions, answered in the verdict:
1. What single control stands between each widened lane and a close it should not perform? (Lane
   (a): the login-AND-id authorship pin. Lane (b): the role+label pin, backed by an INDEPENDENT
   server-side layer this brief cannot touch.) Is the single control alone acceptable? (For (a),
   yes — bounded by the App's own established authority over its own item, the write-scope of its
   own installation token, and the unchanged decision-label absolute refusal. For (b), the
   server-side guard is what makes "no" the right answer to whether the desk-level control alone
   would be acceptable — and it is present precisely because it is NOT.)
2. Does any Verify row prove a LOWER layer catches the fault with the UPPER layer bypassed? (Row
   13 reads `verify-gate-close.yml`'s own guard rather than trusting this brief's prose that it
   exists; rows 7/16 prove the id half of lane (a)'s pin is load-bearing by disabling it and
   watching the suite redden.)
3. Does lane (c) actually change anything, or only describe what already ships? (It changes
   nothing behavioral — confirm the `usage` const edit and any README addition make no claim
   `authority.go:254-276` does not already back.)
