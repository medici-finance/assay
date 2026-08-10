---
brief: methodology-metrics/43
title: 'verify-gate-close becomes the SOLE writer of a human:<name> sign-off stamp — --lint rejects the stamp added anywhere else (arm 2 of oit:I-corroborate-lint)'
wave: 1
depends: []
unblocks: []
effort: M
gate: human
gate-why: >-
  This changes statusgen's anti-falsification / integrity-check surface — the machinery that decides
  whether a human sign-off is real. That category is human-gate by standing policy (memory
  `integrity-check-changes-are-human-gate`), independently of the four risk answers, which are all
  `no` (repo-internal Go tooling, revertible, no ledger/customer/data surface). Two specifics make
  human:<name>'s call load-bearing rather than ceremonial: a rule that is too strict reddens the board and
  gets switched off (the failure mode `#223` documents on itself), and a rule scoped one notch wider
  than specified would forbid the `authorized-by: human:<name>` anchor `#223`'s own gate depends on.
  human:<name> rules on the arming condition and the scope boundary; the merge is his.
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-31 by assay-desk-app authoring session (human:<name>'s arm-2 ruling on #223, 2026-07-31T13:17:37Z); relocated from oit#1580 on human:<name>'s instruction, cross-repo notes re-derived for this repo
sources: ["oit:I-corroborate-lint — the two-arm intake entry; human:<name> chose arm 2 (../oit/docs/streams/intake/2026-07-23-wire-corroborate-into-ci-lint.md)", "F-human-stamp-check — owning finding, resolved: no, now co-located in this repo's docs/streams/FINDINGS.md (moved from oit by assay-selfcontain/03, same brief as this stream's own relocation; oit's original entry is withdrawn-in-place, tombstoned, pointing back here)", "oit:methodology-metrics/15 (built `statusgen --corroborate`; `done`)", "statusgen/verifyissues.go `closeVerify`/`verifyReviewer`, statusgen/corroborate.go `humanStampRe`/`stampsInDiff`, statusgen/registers.go (`registerLandedBase`/`guttedRegisterFields` arrive with #223)", ".github/workflows/statusgen.yml (this repo's PR lint) + ../oit/.github/workflows/{statusgen.yml,verify-gate-close.yml,status-regen.yml}", "#223 (register-gutting gate — the colliding `authorized-by: human:<name>` anchor) and #230 (its overstated `--corroborate` claim)", "oit:assay-selfcontain/03 (relocates this stream here; this brief is seeded ahead of it)", "memory integrity-check-changes-are-human-gate", "freshness-checked 2026-07-31 @ assay-toolkit main + oit main fb410cf0"]
why: >-
  A `human:<name>` stamp is the strongest authorization token the board has — it is what closes an
  irreversible brief — and today any agent can type one into a Verified/Reviewed cell and CI passes.
  methodology-metrics/15 built the detector (`--corroborate`) but wired it to nothing, so
  F-human-stamp-check has sat open since 2026-07-17. human:<name> ruled arm 2 over arm 1: rather than
  verify each stamp online, reduce the set of writers to exactly one already-tamper-evident path —
  verify-gate-close.yml, which only runs when the allowlisted human `human:<name>` closes a verify-gate
  issue — and have `--lint` refuse the stamp from every other origin. Fewer writers is a stronger
  property than more checking.
---

# Brief 43 — verify-gate-close is the sole `human:<name>` writer

## The ruling this brief implements — verbatim

`human:<name>`, on `medici-finance/assay-toolkit#223` @ **2026-07-31T13:17:37Z**, choosing arm 2 of
`oit:I-corroborate-lint`:

> Stricter — make verify-gate-close.yml (the human:<name>-allowlist path) the sole permitted writer of
> human:ian, and have --lint reject that stamp added anywhere else.

Owning finding: `oit:F-human-stamp-check`, open since 2026-07-17, and it stays
`resolved: no` — see "What arm 2 does NOT close".

**Where the ruling is and is not recorded.** It is recorded here, and on `#223`. It is **not**
recorded in the register entry that scoped it: `oit:I-corroborate-lint`
(`../oit/docs/streams/intake/2026-07-23-wire-corroborate-into-ci-lint.md`, in
`oit`) still reads `disposition: scoped`, `scoped-to:
"methodology-metrics (or desk-apps) …"`, and closes with *"Needs human:<name>'s call on which arm"* — a
question that has now been answered. A one-file update to that entry was prepared as
`oit#1580` and **closed unmerged** when this brief was consolidated
here; the branch `feat/mm-43-sole-human-stamp-writer` (head `2fbb11e6`) is retained, so the fix is a
cherry-pick, not a rewrite. It cannot be landed from this repo — the file is in oit — so it is
carried as Task 8 below. Do not assume `oit:assay-selfcontain/03` fixes it at move
time: a relocation moves the entry as-is, stale content included.

## Context

files:
- `statusgen/checks.go` — where the new lint rule lands (the `--lint` check set)
- `statusgen/registers.go` — `registerLandedBase` (the merge-base convention to reuse verbatim; do
  NOT re-derive a base) and `guttedRegisterFields` (the working-tree-vs-base comparison shape to
  mirror). **Both arrive with `#223`** — see "Ordering against #223" below
- `statusgen/corroborate.go` — `humanStampRe` (`human:(\w+)`) and `HumanLoginMap`; reuse the regex,
  do not write a second one
- `statusgen/verifyissues.go` — `const verifyReviewer = "human:ian"` and `closeVerify`, which writes
  `now.Format("2006-01-02") + " " + verifyReviewer` into the Reviewed cell
- `statusgen/checks_test.go` and a new `statusgen/humanstamp_test.go` (planned) — the tests
- .github/workflows/statusgen.yml — **this repo's** PR lint (`go run . --root .. --lint`); the
  surface the rule arms on here
- `../oit/.github/workflows/verify-gate-close.yml` — the sole permitted writer.
  **Exists only in oit** (404 in this repo); read-only for this brief
- `../oit/.github/workflows/statusgen.yml` — oit's PR lint (runs the sha256-pinned
  release binary, not `go run`); `../oit/.assay-versions` — the pin

facts:
- statusgen is canonical here; oit's `tools/statusgen/` is a frozen consumer copy behind a CI
  tripwire that fails any PR editing a `.go` file under it.
- oit's stream READMEs carry **71** `human:<name>` occurrences on **56** rows across **13** files.
  This repo's stream READMEs carry **zero**.
- `--close-verify` (the writer's mechanism) is implemented here; the workflow that invokes it is
  not.

### What lands here, what stays in oit (this inverts the usual note)

The `--lint` rule is **local**: it is Go code in `statusgen/`, in this repo, gated by this repo's
own .github/workflows/statusgen.yml. Nothing about the rule needs a cross-repo PR.

What is **not** here is the *writer*. `verify-gate-close.yml` exists **only** in
`oit` — this repo has no such workflow, no `issues: [closed]`
trigger, and no `ALLOWED_CLOSERS` allowlist. `statusgen.yml` exists in **both** repos and runs
`--lint` on `pull_request` in both. So the shipped rule arms on PRs in two repos whose writer
situation is different:

| | oit | assay-toolkit (here) |
|---|---|---|
| `verify-gate-close.yml` (the sole permitted writer) | present | **absent** |
| `statusgen.yml` PR `--lint` gate (the rule's surface) | present (pinned binary) | present (`go run`) |
| existing `human:<name>` stamps in stream-README cells | 71 on 56 rows | **0** |
| effect of this rule | narrows two writers to one | **forbids the stamp outright** |

**State that second column plainly rather than discovering it later:** in this repo the rule has no
permitted-writer exception because there is no permitted writer, so its meaning here is "a
`human:<name>` sign-off cannot appear in an assay-toolkit stream-README cell at all". That is
correct and intended for now — this repo has never used one — but it means **`oit:assay-selfcontain/03`
must not be run after this lands without the relocation carve-out below**, and it means a future
decision to sign off assay-toolkit briefs the same way requires porting `verify-gate-close.yml`
here first. Neither is in scope; both are named so they are not surprises.

The two things that genuinely stay in oit and are **not** this brief's deliverable: the workflow
itself, and the `.assay-versions` pin bump that adopts the new binary (Task 4).

**Do NOT patch oit's `tools/statusgen/`.** A `.go` patch there runs nowhere and the tripwire
rejects it.

### Ordering against `#223`

`registerLandedBase` and `guttedRegisterFields` do **not** exist on this repo's `main` — they are
introduced by `#223` (OPEN, draft, human-gate, head `b425538d`). This brief reuses both. So either:

- **`#223` lands first** (preferred — no duplicated merge-base logic), and this brief calls
  `registerLandedBase` directly; or
- this brief lands first and must extract an equivalent merge-base helper, in which case `#223`'s
  reviewer has to reconcile two of them.

There is no `depends:` entry for this because a brief cannot depend on a PR. Treat "is `#223`
merged?" as the first question the implementer answers, and record the answer in Evidence.

### The rule, stated precisely

1. `../oit/.github/workflows/verify-gate-close.yml` is the **sole permitted
   writer** of a `human:<name>` sign-off stamp in a stream-README **Verified/Reviewed cell**. It
   already is the only tamper-evident one: it fires on `issues: [closed]`, and its first step rejects
   any closer that is not `type == User` AND in `ALLOWED_CLOSERS: "human:<name>"` (reopening + commenting
   otherwise).
2. `statusgen --lint`, running on a PR, emits a **hard PROBLEM** for any Verified/Reviewed cell that
   gains a `human:<name>` stamp it did not carry at the merge-base with `origin/main`.

### The discriminator — how `--lint` tells the two apart

This is the load-bearing question and it has a clean answer, but not the one the arms suggest.
**There is no identity discriminator available to `--lint`, and none is needed: the two writers are
already on disjoint surfaces.** Measured, not assumed:

- `verify-gate-close.yml` **never opens a PR**. It checks out `ref: main`, runs `--close-verify`,
  and `git push origin HEAD:main`. All **20** of its historical writes are direct commits to oit
  `main` authored by `github-actions[bot]`
  (`git log --grep='chore(verify):' --format='%an' | sort | uniq -c` → `20 github-actions[bot]`,
  @ oit main `fb410cf0`).
- `statusgen.yml` — the only workflow that runs `--lint` as a gate, in **either** repo — is
  `on: pull_request`.

So on the PR surface the rule can be **unconditional**: no allowlist, no identity check, no
exemption clause. Every `human:<name>` a PR adds is, by construction, not from the permitted writer.

**The one collision, and it is not solvable by identity.** `verify-gate-close.yml` calls `--lint`
*itself* — line 210, `/tmp/statusgen --root . --lint` — and it runs it on a working tree that
already contains the stamp `--close-verify` wrote at line 178 and has **not yet committed** (the
`git add` / `git commit` are at lines 217-218). Every existing register check compares the **working
tree** (`os.ReadFile(fsDir/…)`) against `git show <merge-base>:<rel>` (`statusgen/registers.go`), so
a check of that shape fires there — failing the sole permitted writer's own job under `bash -e`,
before its commit. At that moment there is no commit to attribute, so commit-author checks are
unavailable in principle. **The collision lives in oit and the guard ships from here**: it cannot be
reproduced in an assay-toolkit checkout, which is exactly why Verify row 2 proves it by mutation
against a fixture rather than by observation.

Discriminators evaluated against the actual code and **rejected**:

| candidate | why it fails |
|---|---|
| commit author / committer identity | The stamp is uncommitted at the only lint call where it legitimately exists (above). And the workflow sets it itself with `git config user.name "github-actions[bot]"` — two lines any agent can write locally. Forgeable, and unavailable when needed. |
| a workflow-set marker in the content (trailer, HTML comment, sentinel key) | The marker is repo content, so a PR author copies it verbatim. Self-defeating. |
| the gate issue's closer | Genuinely tamper-evident (`sender.type == User` + `human:<name>`), but it lives in the **workflow event context**, not in the repo. `--lint` on a PR has no access to it. Usable only via new machinery — see below. |
| the branch / actor the stamp arrived on | `--lint` runs inside a checkout with no reliable notion of who pushed; `GITHUB_ACTOR` is environment, forgeable locally and absent in a local run. |

**The only tamper-evident signal available to `--lint` is which surface it is running on**, and that
is sufficient. The rule must therefore be **armed**, and the arming condition is the design
decision.

### Arming condition — re-ranked on design merit, with the tooling objection removed

**The original ranking of these three was partly shaped by a constraint that no longer applies.**
Variant B was marked down because it needs a `workflows`-scoped push, which the desk App cannot
make. human:<name> has since said he can make that push, so that objection is **withdrawn**. What follows
ranks the variants on design merit alone. The top choice did not change — but the *reason* B sits
below A is now a design flaw, not a permissions problem, and that distinction is the point.

- **(A) git-derived — `merge-base(HEAD, origin/main) != HEAD`.** Pure git, no flag, no workflow
  edit. Correctly inert in `verify-gate-close` and `status-regen` (both run on `main`, where HEAD
  *is* the merge-base, so the rule is skipped outright regardless of the dirty tree); armed on every
  PR branch, in both repos. **Recommended as the primary arming condition.** Its merit is
  self-enforcement: in oit the arming logic is inside a sha256-pinned release binary, so a PR cannot
  disarm the rule without changing `../oit/.assay-versions` — a visible, reviewable
  diff line. (In this repo `--lint` runs from source, so a PR *can* edit the check; that is the
  ordinary code-review surface and applies to every statusgen check equally.)
- **(B) an explicit flag** (`--human-stamp-gate`) passed only by `statusgen.yml`. Most legible: the
  arming is visible in the workflow rather than inferred from git state. **Still ranks below (A),
  and now for a design reason:** for a same-repo `pull_request`, GitHub runs the workflow file
  **from the PR head**, so a PR can drop the flag from its own run. The edit shows in the diff, but
  the flag is not self-enforcing — it relies on a reviewer noticing, which is precisely the property
  arm 2 exists to stop relying on. Fully available now, and worth adding **on top of** (A) as a
  legibility/defence-in-depth layer if human:<name> wants the arming visible in the workflow; not as a
  replacement for it.
- **(C) arm on `--changed` presence.** Today `--changed` is passed by exactly one of oit's four
  statusgen CI call sites — the PR lint. Zero workflow edits. **Rejected as primary**: implicit
  coupling. A future workflow that adds `--changed` silently arms the rule; a PR lint that drops it
  silently disarms it. Acceptable only with a test pinning the coupling, and (A) is strictly better.
  Unchanged by the lifted constraint.

**human:<name>'s call**: (A) alone, or (A) + (B) together. (B) alone is not recommended.

**Residual hole, stated plainly: no arm of this closes a direct push to `main`.** All three arming
conditions are PR-surface-only, and neither repo's `main` has server-side branch protection
(`oit:deploy-hardening/02` — free tier, not purchased). An account with push rights
can commit a stamp straight to main and meet no gate. Arm 2 narrows the writer set on the *reviewed*
path; it does not create a server-side one. Do not describe the result as tamper-evident — describe it
as "tamper-evident on the PR path".

### What the check must compare — measured, not guessed

A naive "an added diff line contains `human:`" rule is **wrong and would be switched off in days**.
Measured over the 21 days to 2026-07-31 on oit `main` (@ `fb410cf0`), on commits touching
`docs/streams/*/README.md`:

| population | commits | note |
|---|---|---|
| add a line containing `human:` | 68 | the naive rule's firing set |
| …authored by `github-actions[bot]` (verify-gate-close) | 13 | legitimate; the sole writer |
| …**non-workflow, net-new** stamp (added > removed) | **33** | the genuine target (~1.6/day) |
| …**non-workflow, re-flow only** (added ≤ removed) | **22** | **pure false positives** (~1/day) |

(33 + 22 + 13 = 68. Within the 13 bot commits the split is 5 net-new / 8 re-flow; the re-flow class
is not exclusive to human authors, it is simply irrelevant for the permitted writer.)

The 22 are merge-conflict resolutions and row re-renders that rewrite a line already carrying a
landed stamp — 40% of the non-workflow firings, none of them a new authorization. So the check
**must compare per-brief-row cell content against the merge-base version of that same row** and fire
only when the row's Verified/Reviewed cell gains a `human:<name>` it did not have at the base. That
is a hard requirement, not a refinement.

### Migration cost — no sweep, no allowlist, no grandfathering needed

Existing stamps in **oit** stream READMEs @ `fb410cf0`: **71** `human:<name>` occurrences across
**13** stream READMEs, on **56** table rows — all `human:ian`. That is the population the rule
scopes to, and it is the number that matters. (Repo-wide, oit carries **214** occurrences across
**74** markdown files, most of them outside the rule's scope: `STATUS.md`, `docs/reports/`,
statusgen fixtures under `tools/statusgen/testdata/`, and prose. Four are `human:x`, all
test-fixture or prose tokens, not stamps. Reproduce with
`grep -rEno "human:[A-Za-z0-9_]+" --include="*.md" .`.) This repo carries **zero** in either
population.

Because the check is **merge-base-relative**, none of these newly fail: they are already at the base
of every branch cut from main. **Nothing needs grandfathering.** That property is contingent on the
row-cell comparison above — under the naive line rule it evaporates and the board reddens on day
one, which is precisely the switched-off-rather-than-fixed failure mode `#223`'s own false-positive
section warns about. Three carve-outs must be handled:

- **In-repo file moves.** A rename re-adds every stamp at a new path. Treat a rename/copy whose cell
  content is unchanged as not-an-addition (git rename detection, or compare against the pre-move
  path).
- **Cross-repo relocation — the carve-out that in-repo rename detection does NOT cover.**
  `oit:assay-selfcontain/03` moves this whole stream, and eight others, from oit
  into this repo. On the assay-toolkit side that is not a rename: the files are **new**, with no
  pre-image in this repo's history, so `registerLandedBase` finds nothing at the merge-base and
  every one of oit's 71 stamps reads as freshly added. Git rename detection cannot help — there is
  no source blob in this repo to detect a rename *from*. This is a real ordering hazard created by
  the two briefs existing at once, and it has three acceptable resolutions, in preference order:
  (i) land `assay-selfcontain/03` **before** this rule; (ii) give the relocation PR a one-shot,
  explicitly-reviewed escape recorded in the PR body (e.g. a `--allow-stamp-import` flag used once,
  or an import-manifest commit landed to main first so the stamps are at the base); (iii) re-derive
  the base across repos, which is not worth building. Pick one at implementation time and say which
  in Evidence. **Do not leave this to be discovered when the move PR goes red.**
- **Base-unresolvable.** Mirror `registerLandedBase`'s fallback: when `origin/main` does not
  resolve, emit a NOTICE and do not hard-fail (a local clone legitimately has no `origin/main`).

### Collision with `#223` — scope this narrowly or you break the other gate

`#223` (OPEN, human-gate, head `b425538d`) introduces `authorized-by: human:<name>` in a **finding
entry's YAML frontmatter** as the exemption anchor for the register-gutting gate. That anchor is
added **by a PR**, by design. A rule reading "reject a `human:<name>` stamp added anywhere else"
would forbid the very token `#223`'s gate depends on — the two controls would deadlock.

**Resolution: scope this rule to Verified/Reviewed cells in stream-README brief tables only.**
`authorized-by:` frontmatter is explicitly OUT of scope and stays governed by `#223` + `#230`.
Add a test that pins this boundary so a later widening cannot silently break `#223`.

Note for the fixture: this repo's findings register is the aggregate docs/streams/FINDINGS.md,
not the per-entry `docs/streams/findings/` directory oit uses, so the boundary test must be written
against a synthetic fixture root rather than against either repo's live tree.

### `--corroborate` is NOT dead code

methodology-metrics/15 shipped `--corroborate` and it is `done`. Arm 2 does not wire it, and the
natural reading is that it becomes dead. That reading is wrong, and the reason is the scope boundary
above: once this rule lands, the Verified/Reviewed-cell path no longer needs online corroboration
(only one writer can produce it), but **`#223`'s `authorized-by:` anchor still does** — `#230`
records that `--corroborate` "reads added diff lines and would pick up an added
`authorized-by: human:ian`", and that anchor remains self-issuable in CI. So `--corroborate` keeps a
live consumer and a live gap to close. **Verdict: keep it. Not retired, not merely manual — it is
arm 1 for the site arm 2 does not cover.** Recording that here is what stops a later "retire cold
`--lint` rules" pass (`oit:methodology-metrics/35`) from deleting it as unfired.

**Wording correction, a direct consequence of this ruling.** `#223` carries the claim that the
anchor is *"verified online by `statusgen --corroborate <pr>`"* at **two** sites it adds to
`statusgen/registers.go`: the `guttedRegisterFields` `HARD GATE` doc comment, and — the one with
teeth — the gate's **runtime error message**, the string an agent reads at the moment it decides
whether to self-issue a stamp. (Both are on `#223`'s branch, not on `main`; cite them by symbol, not
by line number, until it merges.) Arm 2 makes that claim wrong about the *mechanism*, not merely
unwired: the corroboration path for Verified/Reviewed cells is being replaced by sole-writer
enforcement, while the `authorized-by:` site it actually describes is still uncorroborated. The
correction belongs to whichever PR lands first and must not be silently dropped; it is named here so
it is not lost between the two.

### What arm 2 does NOT close

- **`oit:desk-apps/07` is not subsumed.** That brief (`todo`) backs a `verified`
  row by the Evidence commit's actor and *accepts* `human:<name>` as a valid actor without
  corroborating it. Arm 2 constrains where a stamp may be **written**; it says nothing about how a
  **reader** treats one it finds. A stamp that predates this rule, or one pushed directly to main,
  still satisfies `desk-apps/07` unexamined. **This gap stays open** and `desk-apps/07` must not
  cite this brief as its resolution.
- **`F-human-stamp-check` becomes partially, not fully, resolved.** Its recommendation had two arms;
  arm 2 closes the Verified/Reviewed-cell site on the PR path only. The finding stays `resolved: no`
  until `#230`'s anchor site is also closed. Do not flip it in this brief's PR.
- **Retroactive validity is untouched.** methodology-metrics/15's own verifier found that every
  historical `human:ian` stamp (oit PRs #405, #376, #357, #356) has zero platform action from
  `human:<name>`. Arm 2 does not re-examine any of them.

## Ground rules
- NEVER git push to main / trigger workflows / mutating kubectl. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit STATUS.md on a branch (single writer = main's CI).
- Implementation goes to `statusgen/` **in this repo**, NOT oit's frozen `tools/statusgen/`.
- Do NOT edit `.github/workflows/*` without the `workflows` scope; if arming variant (B) is chosen,
  hand the workflow line to human:<name> rather than pushing it — he has confirmed he can make that push.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Answer the ordering question first**: is `#223` merged? Record the answer and the chosen
   consequence (reuse `registerLandedBase`, or extract an equivalent) in Evidence.
2. **TDD — failing tests first.** Fixture pairs of a stream README (base blob vs working tree):
   (a) a row whose Reviewed cell gains `human:ian` where the base had none → PROBLEM;
   (b) a row whose cell already carried `human:ian` at the base and is re-rendered/re-flowed by a
   merge → clean (this is the 22-commit false-positive class);
   (c) a `human:<unknown>` name → PROBLEM (unknown names are not a loophole);
   (d) `authorized-by: human:ian` added to a findings-register entry → clean (the `#223` boundary);
   (e) armed-condition off (`merge-base(HEAD, origin/main) == HEAD`, i.e. on main) with a dirty tree
   carrying a fresh stamp → clean (this is verify-gate-close's own `--lint` call, which lives in oit
   and can only be reproduced here as a fixture);
   (f) a stream-README move/rename **within one repo** carrying existing stamps to a new path →
   clean.
3. Implement the check in `statusgen/checks.go`, reusing `registerLandedBase` for the base and
   `humanStampRe` for the token. Arming condition (A), plus (B) only if human:<name> rules for it. The
   PROBLEM message must name the sole permitted writer **and the repo it lives in**, name the remedy
   (close the oit verify-gate issue as `human:<name>`), and must NOT claim online verification.
4. Land the `#223` wording correction at both `registers.go` sites if `#223` has merged; if it has
   not, record on `#230` that it is now owed by this ruling.
5. Decide and record the cross-repo relocation carve-out for
   `oit:assay-selfcontain/03` (one of the three resolutions above), and comment it
   on that brief's tracking surface so the move does not go red unexpectedly.
6. Cut a statusgen release (.github/workflows/release-statusgen.yml) and bump oit's
   `../oit/.assay-versions` pin (tag an explicit SHA, per the standing recipe in
   that file) — tagging is human-gated, so stop at the request. The pin bump is a separate oit PR.
7. One line under this stream README's "Shared conventions" (already stubbed there, marked
   not-yet-implemented): remove the caveat once the rule ships.
8. **Land the unlanded register update** (separate one-file oit PR, not part of this repo's diff):
   update `../oit/docs/streams/intake/2026-07-23-wire-corroborate-into-ci-lint.md`
   to record the arm-2 ruling — `scoped-to:` re-pointed at this brief, the verbatim quote above, and
   the note that the entry stays OPEN because arm 2 closes only the Verified/Reviewed-cell site on
   the PR path. Content is ready on the closed `#1580` (branch `feat/mm-43-sole-human-stamp-writer`,
   head `2fbb11e6`); withdraw-in-place rules apply — **never delete the entry file**, `--lint` trips
   on a deleted register entry.

## Verify (executable — no prose-only DoD items)

Rows 1-4 run in this repo. Rows 5-6 run in an oit checkout, against the newly-built binary.

| # | Command | Expect |
|---|---------|--------|
| 1 | `cd statusgen && go test ./ -run HumanStamp -v` | exit 0; `TestHumanStamp*` covers all six Task-2 cases (a)-(f) by name |
| 2 | Mutation test: delete the arming condition so the rule fires on `main`, re-run row 1 | RED on case (e) — proves the verify-gate-close self-collision is actually guarded, not merely absent |
| 3 | Mutation test: replace the row-cell comparison with a raw added-line match, re-run row 1 | RED on case (b) — proves the 22-commit false-positive class is actually excluded |
| 4 | `cd statusgen && go test ./... && go vet ./... && gofmt -l .` | exit 0; `gofmt -l` prints nothing |
| 5 | `statusgen --root <oit> --lint; echo $?` on unmodified oit main with the new binary | 0 — the 71 in-scope stamps do not newly fail (the no-grandfathering claim, executed) |
| 6 | Forge a `human:ian` Reviewed cell on a scratch branch off oit main, run `statusgen --root <oit> --lint; echo $?` | 1, with a PROBLEM naming the row; revert the forgery and confirm exit 0 again |
| 7 | `statusgen --root <assay-toolkit> --lint; echo $?` on unmodified main with the new binary | 0 — this repo has zero in-scope stamps, so the rule is inert here until stamps arrive |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item.
     Also record: (a) whether #223 had merged and which ordering consequence was taken;
     (b) which cross-repo relocation carve-out was chosen for assay-selfcontain/03. -->

## Review
Gate: **human** — this modifies statusgen's anti-falsification / integrity-check surface, which is
human-gate class per memory `integrity-check-changes-are-human-gate` and per the standing policy
`#223` states in its own body. **The merge is human:<name>'s, not the desk's**; no desk `gh pr ready`, no
model-closed `done`. A model reviewer may judge correctness; the authorization to land is human:<name>'s
alone. Reviewer confirms: (a) the arming condition leaves verify-gate-close.yml's own `--lint` call
green on a dirty tree, proven by mutation (Verify row 2); (b) the re-flow class does not fire,
proven by mutation (Verify row 3); (c) `authorized-by:` frontmatter is out of scope and `#223` is
not broken; (d) no message in the diff claims online verification that is not wired; (e) the
cross-repo relocation carve-out is decided and recorded, not deferred.
