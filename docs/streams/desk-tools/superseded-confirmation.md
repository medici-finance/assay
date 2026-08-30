# Superseded confirmation — the two-role lane, and what "superseded" means for a brief

**Stream:** desk-tools. This note lives here because `deskclose` and `deskdisposition` are desk
binaries under `tools/desk/`, this stream is their planning board, and the change is a desk verb's
contract plus the two desk skills that drive it. It is not a lint device (mistake-proofing) and not
a forge-seam change (forge-gitlab) — though §7 hands each of those streams one follow-up.

**Status:** first cut landed with this note (tool + tests + mutation sweep + skill text). The
brief-level question in §5 is answered with a recommendation; it is the driver's to accept.

---

## 1. The trigger

A worker recorded `disposition: SUPERSEDED` on its own 3-file PR, naming a 41-file PR for the same
brief as the thing that carried its scope, and closed it. The call was right. It was also unreviewed
*by design*: nothing in the tools required a second identity to look at "the other PR really does
carry my scope" before the close happened. The driver's ask (2026-08-30):

> When a worker marks something as `disposition: superseded`, it needs to get the reviewer desk to
> actually confirm this. If it disagrees it's a human event. If it doesn't, then the reviewer can
> close it. Still not sure what happens at the brief level for this.

## 2. What exists today (from source)

| Surface | Where | What it does |
|---|---|---|
| The disposition record | `tools/desk/internal/deskkit/disposition.go:47-68` (vocabulary), `:29-33` (who does what) | A worker's terminal verdict on a PR: `disposition:<verdict>` label + `<!-- desk-disposition v1 -->` marker with Evidence. **Records, never closes.** |
| `deskdisposition set/read/sweep` | `tools/desk/cmd/deskdisposition/verbs.go` | Writes/reads the record; the orphan sweep filters on the label. Runs under the caller's ambient token — no role check. |
| `deskclose superseded` (before this cut) | `tools/desk/cmd/deskclose/verbs.go` `cmdSuperseded` → `applyClose` | Read the record (`disposition.go` `requireTerminalDisposition`), require the target MERGED (`github.go` `requireMergedPR`), pass the R-1 ruling gate (`authority.go:20-41`), comment, close. **No check on who is calling** — the worker that wrote the record could execute it. |
| The duplicate lane's two-role rule | `tools/desk/cmd/deskclose/verbs.go` `cmdDuplicate` (the refusal naming "a strong-tier worker folds… a REVIEWER closes") | The precedent: a close that needs two roles is *refused in-tool* until both have acted. |
| The decision-label gate | `tools/desk/cmd/deskclose/github.go:15-28` `decisionLabels`, `refuseDecisionItem` | `needs-decision` / `human-decided` refuse every close, in every mode, manifest rows included. |
| Escalation vocabulary + human-only close | `plugins/assay/skills/intake-desk/SKILL.md:232`, `plugins/assay/skills/the-desk/SKILL.md:127-138`, `plugins/assay/skills/worker-desk/SKILL.md` §Guardrails | `question` / `help wanted` / `needs-decision`; a `needs-decision` issue is human-only-close, and a bare label is unanswerable — the reason must be on the record. |
| Role from the caller today | `tools/desk/cmd/deskflip/flip.go:311-340` `checkCallerRole` | The ready-flip reads its role from `$DESK_LOOP` — an *asserted* identity. `deskpost review` instead mints the reviewer token and posts as the App — an *attested* one. |
| Lifecycle spec | `spec/lifecycle-v1.md` §2 (five ordered states), §2.0 (`blocked`, the ONLY off-path state, MUST re-enter the sequence), §7.3.6 (any other status MUST be rejected) | **"Superseded" does not exist at the brief level.** It exists only as a PR disposition and, for INTAKE entries, as `rejected — <why>` (`spec/registers-v1.md` §5.2). Withdrawal of a register entry is a tombstone (§3.3): flip the disposition, keep the file. |
| Retired board rows | `statusgen/boardhonesty.go:49,106` ("re-homed: the stream README retired the row; its record merged elsewhere") | The board already recognises a *retired* row whose record moved — the closest existing thing to a superseded brief. |

## 3. The flow

### 3.1 Two halves, one verb

`deskclose superseded -R <repo> <N> --by <target> [--dispute <reason>]` — the flags are the same
for both roles. **Which half runs is decided by the token in use, never by a flag.**

| Token role | Half | Preconditions (all fail-closed) | Writes | Closes |
|---|---|---|---|---|
| worker | **propose** | item open, not decision-labelled; a PR carries its disposition record naming `--by` (the finding first); target exists, is in an allowed repo, is not closed-unmerged / a closed issue | `superseded?` label, then a `<!-- desk-superseded-proposal v1 -->` marker comment (`Superseded-By`, `Proposed-By`, `Proposed-At`) | **never**; `--dispute` refused |
| reviewer | **confirm** | a standing proposal exists; its forge-attested author ≠ the caller; its target = `--by`; then the ordinary close gates — R-1 signed, disposition record, target genuinely MERGED | verdict comment on the item (`SUPERSEDED-CONFIRMED` + `<!-- desk-superseded-verdict v1 -->` block), a back-reference on the target, then the close | yes |
| reviewer + `--dispute` | **dispute** | a standing proposal exists; author ≠ caller; target = `--by`; item open and not yet on the decision queue | `SUPERSEDED-DISPUTED: <why>` comment (verdict block + `Reason:`), then the `needs-decision` label | **never** — and every later close is refused by the decision-label gate |
| any other token | — | — | none | refused (5); unreadable identity = could-not-check (6) |

Idempotency: a re-run over a standing proposal for the same target is a no-op; a re-dispute of an
item already on the decision queue is a no-op; a confirm of an already-closed item is a no-op.
Dry-run works for all three halves and writes nothing.

### 3.2 Identity from the token, mapped through the roster

An App installation token cannot read REST `/user` (403 "Resource not accessible by
integration") but GraphQL `viewer { login }` answers with the App's own login. The verb reads that
and maps it through the roster binding (`ASSAY_TRUSTED_BOT_SLUGS` `worker=` / `reviewer=`) with
`deskkit.SameActor`, which normalises the REST (`slug[bot]`) and gh-CLI (`app/slug`) renderings.
Three fail-closed refusals sit in front of the mapping: an unbound role (could-not-check — an
empty identity matches a deleted author), a roster binding both roles to **one** App (refused —
with one App the two-role property is vacuous, and the lane must say so rather than enforce
nothing), and a login bound to neither role (refused — a human closes on the forge directly).

Why not `$DESK_LOOP`, as `deskflip` does? `$DESK_LOOP` is a claim the caller makes about itself.
The forge's answer about the token is a fact about the credential that will author the write. For
a control whose whole content is "a *different* identity looked", the asserted form is exactly the
thing that cannot be trusted.

### 3.3 The same-actor check rests on the forge-attested author

The proposal marker carries a `Proposed-By:` line, and the verb **ignores it** for the same-actor
check: it reads the comment's `user.login` from the thread. A worker cannot write a value the forge
attests, so a proposal that *claims* to be someone else's still fails the check if the same
credential posted it (`TestSupersededReviewerConfirms/the forge-attested author decides`).

### 3.4 Why a comment, not a PR review

The task framed the confirm as "a real review comment/state from the reviewer App". The artifact
here is an **issue comment authored by the reviewer App**, for three reasons: (a) the App's
authorship is what GitHub attests, and that is the whole evidentiary value — a review *state* adds
nothing a closed PR can use; (b) issues and PRs take the same write, so the lane is target-agnostic;
(c) a `COMMENT`-state review from the reviewer App on a *disputed* PR (which stays open) would read
to `deskboard` as **CHECK** — "a bot review exists at head but is neither APPROVED nor
CHANGES_REQUESTED: re-dispatch for a decisive verdict" — and pull a fresh reviewer onto an item
that is already the human's. The verdict is therefore a marker-tagged comment, and the *state*
change is the close (confirm) or the `needs-decision` label (dispute).

### 3.5 Why the target must be MERGED to confirm — and why propose accepts an open one

The R-1 lane's existing invariant is that closed-unmerged never supersedes: work that did not land
cannot retire live work. The confirm keeps it. The proposal does **not** require it — the worker's
claim ("PR B carries this") is usually made while B is open — so a proposed PR sits open, labelled
`superseded?` and `disposition:superseded`, excluded from orphan dispatch by the record, until B
merges and the reviewer confirms. If B dies unmerged instead, the proposal is stale: the reviewer
disputes it (or the worker re-records `NEEDS-REBASE`) and the work resumes. The alternative — let
the reviewer confirm against an *open* target, certifying scope containment and accepting that B
may never land — is a legitimate design; it trades the parked-PR window for a close that can be
wrong for a reason nobody can see at confirm time. This cut takes the conservative side and names
the other.

### 3.6 The manifest lane is not role-keyed

A manifest row is authorized by a human whose comment carries the row set's digest
(`authority.go` `authorizeManifest`). That is a stronger authority than a reviewer's confirmation,
not a weaker one, so manifest rows still go straight to `applyClose` and never consult the token's
identity (`TestSupersededRoleFromTokenBinding/the manifest lane is NOT role-keyed`).

### 3.7 The record is bidirectional

The confirm posts a back-reference on the *target* ("Supersedes `<repo>#<N>` — SUPERSEDED-CONFIRMED
by `<reviewer>` (proposed by `<worker>`)") **before** the close, so a reader of the surviving PR
sees what it retired even if the close then fails; and the superseded PR's close comment carries the
verdict block naming the target, the proposer and the confirmer.

## 4. Pros and cons, honestly

**What it costs.**
- One more round for a class that is usually right. Every supersession now waits on a reviewer
  action, and the reviewer's action is only worth anything if it includes the comparison (both
  file lists against the brief's DoD). That is minutes per item; the cost is real and recurring.
- A parked window: a proposed PR stays open until its target merges (§3.5). It is visibly parked
  (`superseded?`), not orphaned, but it is one more open PR on every board read.
- One forge read per invocation (the `viewer` query) that the old verb did not make.
- Label provisioning: `superseded?` is created on first use; `needs-decision` is the decision
  queue's own label and is **not** created here — a repo without it gets a could-not-check on the
  dispute's label write, with the comment already posted, and the operator is told exactly what is
  missing.

**What it catches.**
- A worker closing the WRONG one — the proposal is against a target the reviewer must agree with,
  and a disagreement is a dispute, not a coin flip.
- A "superseded" that silently drops scope the target lacks — the reviewer's comparison is the
  only place that is looked at, and the dispute path gives it somewhere to go that is not "close
  anyway".
- Identity laundering via close — one credential recording, proposing and confirming is refused
  on the forge-attested author, and a roster that would make it structurally possible (both roles
  on one App) is refused outright.
- Could-not-check rounded to a role — an unreadable token identity does nothing, in either
  direction.

**What it does not catch.**
- A reviewer rubber-stamping. The lane can prove that *a different identity acted*; it cannot
  prove that identity compared anything. The skill text makes the comparison the reviewer's stated
  duty, and a confirm comment that names no comparison is a finding for the review of the reviewer
  — but that is process, not tooling.
- Any session that can read the reviewer App's key can mint its token. The two-role property is
  as strong as key custody — the same limit `pr-review-desk`'s "Reviewer identity" section already
  states for approvals.
- A worker closing its PR by hand with `gh pr close`. The tools refuse; the forge does not. Branch
  protection cannot gate a PR close. The skill text names the by-hand close as the error it is.

**Interactions.**
- *Ready-flip.* None on the flip path: `deskflip` gates on the reviewer's APPROVED at head; a
  `superseded?` PR is never flipped because nobody approves it. A confirmed close removes it from
  the queue; a dispute parks it under `needs-decision`, which the review desk's sweep already treats
  as waiting-on-human.
- *R-1's authority.* Unchanged for the close: the confirm still fetches and verifies the signed
  ruling. Propose and dispute are records, not closes, and sit behind the kill switch, the repo
  allowlist and the write meter but not behind R-1 — in a repo whose ruling register is unsigned,
  a worker can still propose and a reviewer can still dispute; only the close waits.
- *The duplicate lane.* Same shape, now enforced in-tool rather than by refusal-until-signed. If
  R-1's sign-off ever unlocks the duplicate lane, the role split here is the template.
- *The orphan sweep.* Unchanged — `disposition:superseded` already suppresses dispatch. The
  `superseded?` label is the queue-legibility index for the *review* desk; the sweep never needs
  it.

## 5. The brief level — the open question, three options, one recommendation

The question: a PR can be superseded. Can a **brief**?

### Option (i) — never

"Superseded" is a PR/issue disposition; briefs do not carry it. The brief stays `implemented` on
the surviving PR, and the superseded PR is a no-op in the register.

- *Cost:* zero tool or spec changes.
- *What it leaves open:* a brief that is **re-planned** (replaced by a differently-scoped brief) has
  no honest state. `done` lies (no Verify ran), `blocked` is "terminal-for-now" and pollutes the
  stall reports while carrying the wrong meaning, and deleting the file loses the record.

### Option (ii) — a brief may be superseded by another brief

A `superseded-by: <stream>/<NN>` field, a terminal off-path status, a board rendering, and the
lifecycle rule that a superseded brief is terminal (never `done`, never `todo`), its Verify rows
retired, with a human sign-off when the superseded brief was `gate: human`.

- *Cost:* `spec/lifecycle-v1.md` §2 (a second off-path state — and unlike `blocked`, terminal),
  §5 (Next-up exclusion), §7.1/§7.3 (conformance + linter rows); `spec/brief-v1.md` (the field);
  `statusgen` (status parser, the lint that `superseded-by` dereferences to an existing brief and
  that the successor names its predecessor, the `gate: human` sign-off rule, Evidence not required,
  bottleneck/roadmap/trend buckets, STATUS counts); every reader of the status vocabulary
  (`deskboard`, `fanoutloop`, board CI) — because §7.3.6 makes an unknown status a hard reject, the
  fleet's pinned versions must move in lockstep; plus a release and re-pins. Realistically one M
  brief in statusgen, one S in desk-tools, one spec PR, one release.
- *What it buys:* an honest terminal state for cross-brief re-plans, a bidirectional record, and
  tombstone preservation.
- *What it costs beyond the diff:* brief IDs are **typed handles** (`spec/registers-v1.md` §3.4 —
  `affects:`, `depends:`, PR trailers `Brief: <stream>/<NN>`). Replacing a brief with a new ID
  orphans every reference to the old one; `superseded-by:` is a redirect that exists to repair the
  churn this option creates.

### Option (iii) — a brief is superseded only by a revision of itself

A brief that must change scope is **re-baselined in place** under its own ID: a dated re-baseline
note, Verify rows re-baselined, and — when the old plan was invalidated by new knowledge — a
FINDING filed against it (`spec/registers-v1.md` §4.1: "new knowledge that makes an existing brief
wrong or stale"), resolved by the re-baseline. "Superseded" stays reserved for **artifacts** (PRs,
issues, intake entries), never for briefs. This is what happened in the re-plan the driver
mentions, and it cost zero tool changes.

- *Cost:* a documented convention (the re-baseline note's shape: dated, names the finding, states
  which Verify rows were re-baselined) — an S docs change to the authoring rules. Optionally, later,
  an S lint that flags Evidence dated before a brief's re-baseline date as stale.
- *What it keeps:* the ID and every reference to it; the FINDINGS register as the single mechanism
  for "the plan was wrong"; the lifecycle's one off-path state.
- *The residual it does not cover:* a brief retired with **no successor scope at all**. That is not
  "superseded" — there is nothing that supersedes it — and it is a smaller question than it looks:
  `statusgen`'s board-honesty check already recognises a stream README that has *retired* a row
  whose record merged elsewhere. If a no-successor retirement recurs, a `retired` status (terminal,
  no redirect field, no successor semantics) is a strictly smaller change than (ii), and it can be
  decided on its own evidence.

### Recommendation: (iii)

Adopt (iii) now. It is the only option whose cost is a paragraph, it matches what the house
actually did when it re-planned a brief, and it keeps brief IDs stable — which is the property
every other register leans on. Option (ii) is a redirect mechanism whose main job is to repair the
reference churn it introduces; if a second cross-brief re-plan ever makes the case for it, the
`superseded-by` field and its lint can be scoped then, against a real specimen, and the two-role
PR lane here is unaffected either way. Option (i) is (iii) without the rule, and the rule is the
part that was missing.

What this means for the PR-level flow: the brief's board row is **never** touched by a
supersession. The surviving PR carries the row flip; the superseded PR's close comment names the
target (and, by the disposition record's Evidence, the brief it served). A *dispute* at the brief
level is two PRs claiming one brief: the human decides which stands; if the disputed PR carried
scope the target lacks, that is a FINDING against the brief (`affects: [<stream>/<NN>]`), not a
new brief and not a superseded one.

## 6. Verify (what this cut proves, and how)

- `tools/desk/cmd/deskclose/superseded_test.go` — worker propose does not close (and label lands
  before comment); reviewer confirm closes with the verdict on the item and the back-reference on
  the target; reviewer dispute posts the reason then `needs-decision` and never closes; after a
  dispute a confirm is refused; role from the token's binding — neither-role refused, unreadable
  identity could-not-check, unbound role could-not-check, one-App roster refused, both App
  renderings resolve; the manifest lane never consults the identity.
- `tools/desk/cmd/deskclose/mutations.json` — eleven mutations, each disarming one arm of the lane
  (worker falls through to confirm; worker `--dispute` accepted; same-actor check off; no-proposal
  confirm; dispute without the label; proposal without the label; could-not-check rounded to
  reviewer; one-App roster accepted; fenced example read as live; back-reference dropped;
  closed-unmerged target accepted). Run from `tools/desk`:
  `go run ./cmd/muhar -spec cmd/deskclose/mutations.json` → `Totals: 11 caught, 0 NOT CAUGHT,
  0 could-not-mutate`.
- `TestDisputeLabelIsCloseRefused` pins that the label the dispute applies is one the close gate
  refuses — the single property the dispute path rests on.

## 7. Not in this cut — named follow-ups

1. **CI wiring of the mutation sweep.** `release.yml`'s `muhar-light` leg lists its specs
   explicitly; adding `cmd/deskclose/mutations.json` there is a workflow edit, which the worker
   App cannot push. One-line follow-up under a workflows-scoped identity.
2. **Forge parity.** The identity read is GitHub GraphQL. GitLab's equivalent is
   `currentUser { username }`; when the forge-gitlab stream's `Forge` interface lands, `deskclose`
   should read identity through it (a `WhoAmI` on the seam) rather than calling `gh` directly.
3. **Board rendering.** `deskboard` does not yet render `superseded?` as its own bucket
   ("awaiting the review desk's confirm/dispute"). Today it is a labelled open PR; a row would make
   the parked window (§3.5) legible at a glance.
4. **The re-baseline convention** (§5, option iii): a paragraph in the authoring rules stating the
   re-baseline note's shape, and the optional Evidence-predates-re-baseline lint.
5. **Skill parity.** The two skill paragraphs live in the plugin bundle only; adopters that keep a
   house copy of `pr-review-desk` / `worker-desk` re-sync on their next upgrade.
