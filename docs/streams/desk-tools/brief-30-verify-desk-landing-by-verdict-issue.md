---
brief: assay:assay:desk-tools:30
title: verify-desk lands verdicts through the verdict-transcription lane; `deskevidence` demoted to break-glass
why: >-
  The verify desk's skill still says its verdicts land by the desk itself committing Evidence and
  status flips straight to `main` through `deskevidence`, with a pointer saying a signed-issue lane
  "would replace" that path one day. That lane exists in this repo's code: `verifyloop verdict` composes a
  signed verdict and `statusgen --transcribe-verdict` lands it on main behind a signature, author and
  body-edit check. A downstream house has ruled to cut over, and the skill is the doctrine every
  adopter's verify desk reads. Until the skill names the lane as the landing path and demotes the
  direct push to break-glass, every verify desk keeps pushing to `main` from a session, which is the
  widest write grant in the fleet.
wave: 1
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
gate-why: >-
  This brief changes HOW the verify desk routinely writes the project's `main` branch for verify
  landings: the shipped doctrine moves its routine landings off the standing direct-push path and
  onto a signed-issue lane that a workflow lands. It narrows routine USE, not the grant. The
  direct write's switch (`VERIFIER_MAIN_OK`, tools/desk/cmd/deskevidence/deskevidence.go:157) is
  set by the session itself and is set routinely in the verify-desk window (the :234 comment says
  so). After this brief, break-glass is therefore enforced by doctrine only, and it is recorded by
  the same session it limits. Gating or removing the direct write is a separate follow-up that has
  not been authored (named in `## Context`). The grant is a human authorization in the first place,
  so re-routing its routine use is a human ruling, not a model call. The human confirms (1) which of the options in
  `## Human decision` the doctrine adopts; (2) the PRECONDITION: on at least one repo a verify desk
  serves, the lane is armed (its enactment sign-off resolves to the blessing authority), a verdict
  filer is bound, and that repo's remote `main` already carries at least one real verdict the lane
  transcribed end to end. That precondition is something only the human can attest from the
  forge, so it is recorded here and in row 10, never as a typed `depends:` edge.
decision-trigger: creation
issues: []
outcome: none
schema: brief-v2
authored: 2026-10-08 by a desk-dispatched authoring session (public half of a downstream house's ruled cutover)
sources:
  - "the house cutover brief (a downstream house's private brief, ruled): its public half, Task 4a,
    is this brief. Not cited by path or number: a public brief cannot anchor on a private one."
  - "freshness-checked 2026-10-08 @ 36113a1dd — plugins/assay/skills/verify-desk/SKILL.md,
    plugins/assay/skills/{the-desk,pr-review-desk,worker-desk,pr-shepherd}/SKILL.md,
    .claude/guardrails/GUARDRAILS.md, tools/skillslint/{main,guardrail}.go, Makefile,
    tools/desk/internal/deskkit/{ratelimit,width}.go, tools/desk/cmd/verifyloop/verdictrun.go,
    statusgen/{transcribeverdict,transcribescan,consumers,main}.go and .github/workflows/ all
    re-read at that commit before the Task text was written; every line number below is at that
    commit."
  - "correction carried, not repeated: the house cutover brief's claim that the lane has been landing
    real verdicts since an earlier date is stale and is not repeated here — nothing in this tree
    shows a live lane (facts 4 and 6), and the claim was found stale downstream. Whether any repo's
    lane is live TODAY is exactly the precondition the human attests."
exec-tier: strong
exec-tier-why: "(b) cross-artifact doctrine: one shared policy sentence is rewritten at its declared
  guardrail source and regenerated into five skill bodies, and §Landing, two other verify-desk
  sections and the desk meter's rationale must agree with it. Only the five copies are lint-checked
  against each other; the rest agree only if the implementer makes them."
consumers:
  - "plugins/assay/skills/verify-desk/SKILL.md: follow-up desk-tools/30 (this brief; Tasks 1-5)"
  - ".claude/guardrails/GUARDRAILS.md: follow-up desk-tools/30 (this brief; Task 5, the declared source of the shared push-policy block)"
  - "plugins/assay/skills/the-desk/SKILL.md: follow-up desk-tools/30 (this brief; Task 5, the shared push-policy sentence)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md: follow-up desk-tools/30 (this brief; Task 5)"
  - "plugins/assay/skills/worker-desk/SKILL.md: follow-up desk-tools/30 (this brief; Task 5)"
  - "plugins/assay/skills/pr-shepherd/SKILL.md: follow-up desk-tools/30 (this brief; Task 5)"
  - "tools/desk/internal/deskkit/ratelimit.go: follow-up desk-tools/30 (this brief; Task 6, comment only, cap value unchanged)"
  - "tools/desk/internal/deskkit/width.go: out-of-scope (the verify-desk width arm keeps metering the deskevidence bucket; re-deriving it onto the verdict-issue bucket is deferred to an unauthored brief, and the declared max of 6 binds under either meter)"
  - "tools/desk/cmd/verifyloop/verdictrun.go: out-of-scope (wiring the signed-issue FILING into a public tool is a separate unauthored brief; this brief changes doctrine only and names the filing step as a project-bound capability)"
version: 1
id: 4fd2d0b1-082c-4516-a093-92e587ce2e72
---

# Brief 30 — verify-desk lands verdicts through the verdict-transcription lane; `deskevidence` demoted to break-glass

## Dependencies
None typed, and the reason is recorded rather than left to be rediscovered.

The real precondition is not a brief: it is a FORGE FACT — a lane armed on a real repo with at
least one real verdict transcribed onto its `main`. No brief on this board delivers that, and the
house cutover brief that does track it lives on a private board this repo's lint cannot see. So
the precondition is the human's attestation (gate-why item 2, Verify row 10), not a `depends:`
edge.

There is a second, separate gap this brief does NOT close and does not wait on: no public tool
FILES the signed verdict issue today (fact 5). The doctrine is written so that it holds whether
the filer is a project-local tool or a future public one; wiring a public filer is a separate,
unauthored brief.

<!-- graph: not-a-gate -->

## Context
files: `plugins/assay/skills/verify-desk/SKILL.md`, `plugins/assay/skills/the-desk/SKILL.md`,
`plugins/assay/skills/pr-review-desk/SKILL.md`, `plugins/assay/skills/worker-desk/SKILL.md`,
`plugins/assay/skills/pr-shepherd/SKILL.md`, `.claude/guardrails/GUARDRAILS.md`,
`tools/desk/internal/deskkit/ratelimit.go`, `changelog/desk-tools-30.md` (planned)

single-point-of-failure: custody of the verifier App credential. The author check (clause 1) and
the RS256 signature (clause 2) both rest on it (statusgen/transcribeverdict.go:22-26, :746-758),
so they are two checks on one anchor, not two layers. The enactment gate does not guard a single
verdict; it only arms the lane. The one independent layer is the network-off re-execution of
`check:ci` rows (:842-843), and `check` rows do not get it. Behind that, a forged verdict is
bounded by the write class the lane admits:
- at most 2048 bytes per Evidence entry;
- no Markdown section headings;
- no `human:` runner stamp;
- irreversible briefs refused;
- status flips for `gate: model` briefs only.
The new custody point this brief adds is the push credential of the transcription workflow, which
writes `main` on the lane's behalf. Break-glass `deskevidence` keeps its own guards (main-push
switch, repo allowlist, body scan, rate limit, attribution check), but its switch is set by the
session (gate-why). This brief moves doctrine between the two paths and removes no guard from
either.
Accepted residual (named for the driver's decision at this brief's gate): in CI the transcriber reads its trust configuration (the
blessed login, the verifier role binding and the verifier public key) from the repository's
Actions configuration: the login and role binding through the CI branch of
statusgen/rosterconfig.go (:728-729), the public key from `--pubkey` or `ASSAY_VERIFIER_PUBKEY`
(statusgen/transcribeverdict.go `verdictResolvePubkey`, :339-358), so all three anchors are only as
protected as write access to that configuration, and one change there moves them together.
Write access to it is normally an administrator's, who can already bypass `main`'s protection,
so this adds no new path for that identity; pinning the anchors is a follow-up below.

follow-ups (not authored by this brief):
- pin the transcriber's trust anchors in a committed, protected file on `main`, with a CI
  check that the configuration copy equals the committed one, so changing an anchor takes a
  reviewed commit (human-gated: it changes a security control);
- gate or remove `deskevidence`'s direct `main` write, so that break-glass is enforced by a
  control the session cannot set for itself;
- an audit signal that does not depend on the session: in a project where the lane is armed, any
  verifier-authored commit on `main` that the transcription workflow did not write counts as a
  break-glass use;
- check the verdict payload's head and timestamp against the landing tree, so that the verifier
  identity cannot replay a stale verdict.

facts:
  - "verify-desk SKILL.md:435 — heading '## Landing — deskevidence is the SOLE main-push carve-out
    (narrow, dated)'; :442-446 the carve-out grant (Evidence rows + status flips commit
    straight to `main` as the verifier App); :448-456 the `deskevidence` Interface paragraph;
    :458-462 the guards paragraph; :492-602 `### Public repo (PR-required main) — Evidence lands by
    PR` (the same `deskevidence`, aimed at a branch plus a draft PR). Its :500-502 precondition: the
    PR form is available for a repo only when a recorded human ruling names that repo and that
    landing shape."
  - "verify-desk SKILL.md:488-490 — the `Standing-doctrine pointer (2026-08-17)` paragraph: the
    lane 'would replace this path'; it cites the directory docs/streams/verdict-lane/, which does not exist in
    this tree (a dangling private path). The same path also appears at :698 in §Cluster rows, which
    this brief does not rewrite, so a check for its absence must be scoped to §Landing. :179 step 3
    says 'Land each verdict as it returns via `deskevidence`'; :197-198 says filing the signed
    payload 'is the autonomous cutover, `gate: human`'. :815-818 (desk-specific) says the desk lands everything else 'via the push race loop
    (`commit → pull --rebase → push`, retry on race)'. Today the file contains 'break-glass' 0
    times and 'transcribe-verdict' 0 times."
  - "The inherited `Git push policy (ONE policy, role-keyed)` block carries one identical sentence —
    'The verify desk lands its own work: its Evidence + status flips commit straight to `main` as
    the project directs' — in five skills: the-desk SKILL.md:177-178, pr-shepherd :222-223,
    pr-review-desk :989-990, worker-desk :773-774, verify-desk :809-810. The five are generated
    copies, not independent text. Their declared source is `.claude/guardrails/GUARDRAILS.md`
    (§guardrail: git-push-policy, :101; the five sites :112-116; the block :119-127; the sentence
    :122-124; the prose that names the grant, :107). `tools/skillslint` byte-compares every copy
    against that source (`CheckGuardrails`; a gating job in .github/workflows/ci.yml runs
    `cd tools/skillslint && go run . --root ../..`), and `make guardrail-sync` (Makefile:173-174,
    `go run . --root ../.. --sync`) rewrites the copies from the source, locating each copy by the
    block's unchanged first line. So the sentence is changed in the source and regenerated; a
    hand-edit of the five copies alone fails skillslint."
  - "The lane in this repo's code: `statusgen --transcribe-verdict` (statusgen/main.go:1586, with
    `--pubkey` at :1587) is the transcriber; its header (statusgen/transcribeverdict.go:1-34) says it
    ships INERT. Its enactment gate (:423-436) reads the file docs/streams/issue-flow/rulings.md and is
    INERT when that file is absent — and it is absent in this tree, so the lane cannot arm on THIS
    repo as it stands. It consumes PASS verdicts only (cl.8, :790-800), refuses any
    `risk.irreversible: yes` brief and any `human:` stamp (cl.5, :808-815), and flips status only
    for `gate: model` briefs (implemented→verified, :894-910); `gate: human` briefs get Evidence
    only."
  - "The filing gap: `verifyloop verdict` composes and signs the payload but does not file it
    (tools/desk/cmd/verifyloop/verdictrun.go:23-29; :227-229 prints 'filing a verify-verdict issue is
    the autonomous cutover (gate: human, BLOCKED-ON-HUMAN) — not filed'). The verdict-issue meter
    `deskkit.VerdictIssueTool = \"verifyloop-verdict\"` and `AllowVerdictIssueWrite`
    (tools/desk/internal/deskkit/ratelimit.go:389-410) have no non-test caller."
  - "No workflow under `.github/workflows/` in this repo invokes `--transcribe-verdict` (only
    `verify-gate-open.yml` / `verify-gate-close.yml` exist on the verify side); the transcription
    workflow is project-supplied glue around the statusgen mode."
  - "The rate-limit carve-out: ratelimit.go:74-76 `deskevidenceUnnumberedCap = 30`; :110-127 its
    rationale and `unnumberedBucketCap = {\"deskevidence\": …}`. Consumers: width.go:238-252 (the
    verify-desk width arm charges `UnnumberedCapFor(\"deskevidence\")`, 2 writes per item, default
    and declared max 6), width_test.go:47 `TestMaxWidth_IsBoundedByTheEnforcedBudget`,
    ratelimit_test.go:771 `TestUnnumberedBucketOverridePerTool`. Lowering or deleting the cap
    changes the width arm's arithmetic, so this brief ANNOTATES it and keeps the value."
  - "Classes the lane does not carry, which therefore need a named non-lane route: outcome records
    (`deskevidence --outcome-record`, SKILL.md:255 and :398-416), Evidence for
    `risk.irreversible: yes` briefs (refused by cl.5; the human flips via the verify-gate issue,
    SKILL.md:641-675), Evidence that is not PASS (cl.8 refuses the whole verdict on any non-PASS
    entry, transcribeverdict.go:796-800), and any landing while the lane is down or unarmed. Two
    verify-desk sections outside §Landing assume the direct write today: :644 ('Because this desk
    lands status straight to main') and :704 (§Cluster rows step 1, 'Lands the passing-row Evidence
    via `deskevidence`', with the step-2 could-not-check rows beside it)."

## Human decision
<!-- gate: human, decision-trigger: creation — self-contained; no links, paths or brief refs. -->
The verify desk is the automation role that re-runs each finished work item's checks and records
the result on the project's main branch. Today its instructions let it write the main branch
directly from a working session, the widest write permission any automated role holds. The
project's tooling now has a second path: the desk files a signed verdict as a tracker issue, and a
separate workflow checks the signature, the issue's author and that the issue was never edited,
then writes the result to the main branch itself. A downstream installation has already ruled to
move its own verify desk onto that second path. This decision is what the shared instructions,
which every installation inherits, should say.

Four things the second path does not cover, whichever option is picked: results that are not a
pass (a failed check, or one that could not be run), results for work marked irreversible (a human
signs those off separately), the per-verdict outcome records, and any landing while the second
path is switched off or broken. Under options 1 and 2 those go through the direct-write tool's
pull-request form (a branch plus a draft pull request, never a direct write to the main branch),
which needs its own recorded ruling for that repository. The one exception is a landing while the
second path is down: it may use the direct write as break-glass, on a human's say-so for that one
landing.

Options:
1. **Signed-issue path everywhere.** Every installation's verify desk lands through the signed-issue
   path; the direct write becomes break-glass, used only on a human's say-so for one landing at a
   time. Consequence: an installation that has not switched the second path on has no routine way
   to land results except break-glass or pull requests, so its verify desk effectively stalls until
   it switches on.
2. **Signed-issue path where it is switched on (recommended).** Where an installation has switched
   the second path on and has a filer for the signed issues, its verify desk lands only that way,
   and the direct write is break-glass. Where it has not, the direct write stays the standing path,
   unchanged. Consequence: no installation stalls, and each one moves when it switches on.
3. **No change.** The instructions keep the direct write as the verify desk's landing path and keep
   the second path as a future pointer. Consequence: the downstream installation's ruling and the
   shared instructions disagree, and its desks follow the shared text.

Before choosing 1 or 2, confirm from the tracker itself that on at least one repository the second
path is switched on, a filer exists, and that repository's main branch already holds at least one
real verdict the second path wrote end to end. Without that, no installation has proven the path.

Recommendation: **Option 2.** Wherever the safer path is proven, it moves routine landings off the
widest write path, and it strands no installation that has not switched that path on. It narrows
routine use, not the permission. The direct write stays available to any session that sets its
switch, so until a follow-up gates or removes that direct write, break-glass is a rule the desk
follows, not a control that stops it.

Default if no answer: none — blocks until answered.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity
  does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Implement the option the recorded ruling names. If the ruling is option 3, implement nothing:
  report that the brief closes as ruled-no-change.
- Doctrine only. Do NOT change any cap VALUE, any width-table number, `VerdictIssueTool`,
  `AllowVerdictIssueWrite`, any statusgen code, or any workflow.
- Never claim in the skill that `verifyloop verdict` files the issue: it does not (fact 5). Name
  the filing step as the project's **verdict filer**, a capability the project binds.
- Write nothing in any skill body that names a private repo, path or issue.

## Task
Written for option 2 (the recommendation). Under option 1, drop the "unarmed project" paragraph in
step 1 and every armed/unarmed qualifier in the replacement text of steps 1, 4 and 5 ("where the
project has armed the lane", "the verdict lane where armed, `deskevidence` otherwise", "where the
lane is not armed it lands per its own skill's §Landing"); in step 2, drop "in an armed project"
from the break-glass introduction and the residual-route bullet, and drop "plus the
unarmed-project standing path" from the rewritten authorization sentence; in step 6, drop "in a
project that has armed the verdict-transcription lane" and "it still serves unarmed projects and"
from the comment (the width-arm reason stays). The lane is then the only routine route and
`deskevidence` is break-glass everywhere. Rows that grep those qualifiers change to match.

### 1. Rewrite `## Landing` in `plugins/assay/skills/verify-desk/SKILL.md` (:435-490)
- Retitle the heading to: ## Landing — a verdict lands by signed verdict issue; `deskevidence` is
  break-glass.
- Open with the landing path, in order: the verifier session's `verifyrun` rows are composed and
  signed by `verifyloop verdict` (if desk-tools/29 lands first, the body may instead be composed
  unsigned and signed on the host; either way it is signed by the time it is filed, and this brief
  needs nothing from desk-tools/29); the project's **verdict filer** files that signed body as a
  verify-verdict issue; the project's transcription workflow runs `statusgen --transcribe-verdict`,
  which re-checks author, signature and body-unedited, re-runs `check:ci` rows network-off, and is
  the sole writer of the resulting Evidence and `gate: model` flip on `main`. No session pushes
  `main` on this path.
- State the lane's limits plainly: PASS only. Evidence that is not PASS (a FAIL row, or a
  could-not-check or HELD row such as a cluster row's) is never put in a verdict, because one
  non-PASS entry makes the lane refuse the whole verdict; in an armed project it takes the residual
  route below, and the FAIL route in §On VERIFY: FAIL (bug plus outcome record) is otherwise
  unchanged. A `gate: human` brief gets Evidence only; an `risk.irreversible: yes` brief and any
  `human:` stamp are refused, so irreversible-brief Evidence and outcome records take the residual
  route too.
- Keep the "witness lands WITH the Evidence" paragraph and the "PASS is a flip signal only when its
  own Evidence agrees" paragraph; they bind the payload the lane carries as much as a direct landing.
- Add the unarmed-project paragraph: where the project has not armed the lane (its enactment
  sign-off does not resolve, or no verdict filer is bound), `deskevidence` stays this desk's
  standing landing path exactly as before. A read of that state that is could-not-check is
  neither "armed" nor "unarmed": the landing HOLDS and is surfaced as could-not-check, and never
  falls through to the direct `deskevidence` write.
- Keep "Land as each verdict arrives" and the desk-verbs sentence.

### 2. Demote `deskevidence` to break-glass — documented, not deleted
- Add a subsection headed ### Break-glass — `deskevidence`, holding the existing Interface paragraph
  (:448-456) and guards paragraph (:458-462) verbatim, introduced by: in an armed project,
  `deskevidence` lands to `main` only when the lane is down, on the driver's explicit say-so for
  that landing, recorded on the brief's Evidence as break-glass.
- Keep `### Public repo (PR-required main) — Evidence lands by PR` (:492-602) and name it as the
  **residual route**: in an armed project, non-PASS Evidence, outcome records and
  irreversible-brief Evidence land through it (branch plus draft PR), never a direct `main` push.
  Its recorded-ruling precondition (:500-502) stays as written; where no ruling names the repo,
  the item is surfaced as awaiting a decision, as that subsection already says.
- Rewrite the dated authorization sentence (:442-446) so it states the grant as break-glass plus
  the unarmed-project standing path, still "nothing else, nobody else".

### 3. Delete the stale pointers
- Delete the `Standing-doctrine pointer (2026-08-17)` paragraph (:488-490), including its
  docs/streams/verdict-lane/ path, which resolves nowhere in this repo. Leave the same path in
  §Cluster rows (:698) alone; it is outside this brief.
- In the desk-specific bullet (:815-818) replace "via the push race loop (`commit → pull --rebase →
  push`, retry on race)" with a pointer to §Landing. Keep the two landings the desk does NOT do.

### 4. Bring the rest of the verify-desk skill into line
- §The loop, :179 step 3 → "Land each verdict as it returns, by §Landing (the verdict lane where
  armed, `deskevidence` otherwise)".
- §The loop, :197-198 → "`verifyloop verdict` composes and signs the verdict; the project's verdict
  filer files it (§Landing)". Delete "the autonomous cutover, `gate: human`".
- §Irreversible briefs, :644-645 → replace "Because this desk lands status straight to main,
  flipping one on a model verify would fail `--lint` and redden main CI directly." with "A model
  flip of one would fail `--lint` and redden main CI, and the verdict lane refuses an irreversible
  brief outright." In step 1 (:648), after "Write the Evidence rows", add "(in an armed project, by
  the residual route in §Landing)".
- §Cluster rows, :704-705 step 1 → "Lands the passing-row Evidence by §Landing (the verdict lane
  where armed, `deskevidence` otherwise). In an armed project the step-2 could-not-check rows are
  not PASS, so they land by the residual route." Steps 2 and 3 are unchanged.

### 5. The shared push-policy sentence: edit the source, regenerate the five copies
The sentence quoted in fact 3 is a generated copy. Edit it ONLY in its declared source,
`.claude/guardrails/GUARDRAILS.md`, then run `make guardrail-sync` from the repo root. That
rewrites the copy in each of `plugins/assay/skills/{the-desk,pr-review-desk,worker-desk,pr-shepherd,verify-desk}/SKILL.md`.
Never hand-edit a copy.

- In the `text` block (:119-127), replace everything from the line that begins
  `` `gh pr create --draft`). **The verify desk lands its own work** `` to the end of the block
  with these lines, exactly (two-space indent, as now):

  ```text
    `gh pr create --draft`).
    **The verify desk lands verdicts through the project's verdict-transcription lane** where the
    project has armed it — a signed verdict issue that a workflow lands on `main`, never a session
    push; where the lane is not armed it lands per its own skill's §Landing. Those two routes need
    no push-go and none should be waited for; a break-glass `main` write in a project where the
    lane is armed is NOT one of them: it is for when the lane is down, and it waits for the
    driver's go. Any `main` push not covered by a standing authorization is
    gated on the driver's explicit go; committing local work is always fine. A guard/hook-BLOCKED
    push is a STOP signal — never route the same write through another tool. Each desk's own
    grants and denials (what it may flip, file, close, or land) stay in its skill, directly below
    this block.
  ```

  The first three lines of the block are unchanged, so the sync still finds each copy by its
  first line. Only the verify-desk sentence changes in substance; the rest is re-wrapped.
- In the prose above the sites, :107, replace "the verify-desk `main` grant" with "the verify
  desk's landing rule". Leave :105 alone: it quotes what the old copies said. Do not put the new
  sentence's opening words in this prose (row 6 counts them).
- Run `make guardrail-sync`, then confirm `cd tools/skillslint && go run . --root ../..` exits 0
  (row 14). Leave every other line of each skill's block, and each skill's own bullets below it,
  untouched.

### 6. Annotate the rate-limit carve-out (`tools/desk/internal/deskkit/ratelimit.go:110-127`)
Append to the `unnumberedBucketCap` comment: in a project that has armed the verdict-transcription
lane, `deskevidence` is break-glass plus the residual route (non-PASS Evidence, outcome records,
irreversible-brief Evidence), and routine verdict landings meter under `VerdictIssueTool`; the cap
stays at its value because it still serves unarmed projects and the width arm in `width.go` reads
it. The word `break-glass` appears in the comment. No code change.

### 7. Changelog
Add `changelog/desk-tools-30.md` (planned) with a `### Changed` bullet naming the doctrine change.

### 8. Flip this brief's consumer routings
In this brief's frontmatter, change each of the seven consumer entries that route to this brief
(the five skills, `GUARDRAILS.md` and `ratelimit.go`) from its `follow-up` routing to
`fixed-here`, keeping the site and the parenthesised note. `statusgen --consumers` then checks
each one against the implementation diff: a `fixed-here` entry is corroborated only when its path
is in the diff (statusgen/consumers.go:733-760). A `follow-up` that names this brief is
corroborated by the brief's own existence (:761-775), so before this step row 12 proves nothing.
Leave the two `out-of-scope` entries as they are.

## Verify (executable — no prose-only DoD items)
Rows 1-7, 12 and 13 are discriminating: each FAILS against today's tree (at 36113a1dd) and passes
only once the Task lands. Rows 8, 9 and 14 pass today and must still pass. Run every row from the repo root.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `grep -q '^## Landing' plugins/assay/skills/verify-desk/SKILL.md && ! grep -q 'SOLE main-push carve-out' plugins/assay/skills/verify-desk/SKILL.md && echo ok` | prints `ok`. FAILS today: the heading still reads 'SOLE main-push carve-out'. |
| 2 | check +dereference | `rm -f "${TMPDIR:-/tmp}/dt30-help.txt" && cd statusgen && go build -o "${TMPDIR:-/tmp}/dt30-sg" . && "${TMPDIR:-/tmp}/dt30-sg" --help > "${TMPDIR:-/tmp}/dt30-help.txt" 2>&1; grep -q -e '-transcribe-verdict' "${TMPDIR:-/tmp}/dt30-help.txt" && grep -q -e '--transcribe-verdict' ../plugins/assay/skills/verify-desk/SKILL.md && grep -q 'verdict filer' ../plugins/assay/skills/verify-desk/SKILL.md && grep -q 'verifyloop verdict' ../plugins/assay/skills/verify-desk/SKILL.md && echo ok` | prints `ok`: the skill names the transcriber mode AND that mode exists in the built binary's flag set (not a dangling name), plus the filer and the signer. FAILS today: the skill names `--transcribe-verdict` 0 times. |
| 3 | check | `grep -q -i 'break-glass' plugins/assay/skills/verify-desk/SKILL.md && grep -q 'deskevidence --help' plugins/assay/skills/verify-desk/SKILL.md && grep -q 'VERIFIER_MAIN_OK' plugins/assay/skills/verify-desk/SKILL.md && grep -q 'PR-required main' plugins/assay/skills/verify-desk/SKILL.md && echo ok` | prints `ok`: demoted, NOT deleted — the Interface, the main-push switch and the PR form survive. FAILS today: 'break-glass' appears 0 times. |
| 4 | check | `! grep -q -i 'push race' plugins/assay/skills/verify-desk/SKILL.md && ! grep -q 'Standing-doctrine pointer' plugins/assay/skills/verify-desk/SKILL.md && ! grep -q 'the autonomous cutover' plugins/assay/skills/verify-desk/SKILL.md && awk '/^## /{f=/^## Landing/} f' plugins/assay/skills/verify-desk/SKILL.md > "${TMPDIR:-/tmp}/dt30-landing.txt" && test -s "${TMPDIR:-/tmp}/dt30-landing.txt" && ! grep -q 'docs/streams/verdict-lane/' "${TMPDIR:-/tmp}/dt30-landing.txt" && echo ok` | prints `ok`. The dangling path is checked inside §Landing only (from its heading to the next `## ` heading, subsections included), because §Cluster rows keeps its own copy of the path and no Task touches it. FAILS today: 'push race', the pointer paragraph and 'the autonomous cutover' are present, and §Landing carries the path at :488. |
| 5 | check +neighbour | `rm -f "${TMPDIR:-/tmp}/dt30-lane.txt" && grep -L -F 'verdict-transcription lane' plugins/assay/skills/the-desk/SKILL.md plugins/assay/skills/pr-review-desk/SKILL.md plugins/assay/skills/worker-desk/SKILL.md plugins/assay/skills/pr-shepherd/SKILL.md plugins/assay/skills/verify-desk/SKILL.md .claude/guardrails/GUARDRAILS.md > "${TMPDIR:-/tmp}/dt30-lane.txt"; ! grep -r -q 'straight to .main. as the project directs' plugins/assay/skills .claude/guardrails && test -f "${TMPDIR:-/tmp}/dt30-lane.txt" && [ ! -s "${TMPDIR:-/tmp}/dt30-lane.txt" ] && grep -l 'Git push policy (ONE policy, role-keyed)' plugins/assay/skills/the-desk/SKILL.md plugins/assay/skills/pr-review-desk/SKILL.md plugins/assay/skills/worker-desk/SKILL.md plugins/assay/skills/pr-shepherd/SKILL.md plugins/assay/skills/verify-desk/SKILL.md > "${TMPDIR:-/tmp}/dt30-pol.txt" && [ $(wc -l < "${TMPDIR:-/tmp}/dt30-pol.txt") -eq 5 ] && echo ok` | prints `ok`: the old sentence is gone from the guardrail source and every skill, the source and all five copies carry the new one, and all five skills still carry the policy block. FAILS today: the old sentence is in the source and all five. |
| 6 | check | `grep -c -F 'The verify desk lands verdicts through the project' plugins/assay/skills/the-desk/SKILL.md plugins/assay/skills/pr-review-desk/SKILL.md plugins/assay/skills/worker-desk/SKILL.md plugins/assay/skills/pr-shepherd/SKILL.md plugins/assay/skills/verify-desk/SKILL.md .claude/guardrails/GUARDRAILS.md > "${TMPDIR:-/tmp}/dt30-same.txt"; ! grep -q -v ':1$' "${TMPDIR:-/tmp}/dt30-same.txt" && echo ok` | prints `ok`: exactly one copy of the new sentence's opening in the guardrail source and in each of the five skills. FAILS today: every count is 0. |
| 7 | check | `grep -q -i 'break-glass' tools/desk/internal/deskkit/ratelimit.go && grep -q 'deskevidenceUnnumberedCap = 30' tools/desk/internal/deskkit/ratelimit.go && grep -q 'const VerdictIssueTool = "verifyloop-verdict"' tools/desk/internal/deskkit/ratelimit.go && echo ok` | prints `ok`: the carve-out is annotated, its value and the verdict-issue meter are unchanged. FAILS today: 'break-glass' appears 0 times in the file. |
| 8 | check +neighbour | `cd tools/desk && go test -count=1 ./internal/deskkit/ ./cmd/deskevidence/ ./cmd/verifyloop/ ./cmd/deskverdict/` | exit 0: the cap, the width arm (`TestMaxWidth_IsBoundedByTheEnforcedBudget`), the per-tool override test and the break-glass tool itself are unaffected. Passes today and must still pass. |
| 9 | check +flow | `cd tools/desk && go test -count=1 -v -run '^TestCLIRoundtrip$' ./cmd/deskverdict/ > "${TMPDIR:-/tmp}/dt30-flow.txt" 2>&1 && cd ../../statusgen && go test -count=1 -v -run '^TestTranscribeVerdictValidSignatureConsumed$' . >> "${TMPDIR:-/tmp}/dt30-flow.txt" 2>&1 && go test -count=1 -v -run '^TestTranscribeVerdictInertEvaluatesNoClause$' . >> "${TMPDIR:-/tmp}/dt30-flow.txt" 2>&1 && go test -count=1 -v -run '^TestTranscribeVerdictHumanGateEvidenceOnlyNoFlip$' . >> "${TMPDIR:-/tmp}/dt30-flow.txt" 2>&1 && go test -count=1 -v -run '^TestTranscribeVerdictNegativeBattery$' . >> "${TMPDIR:-/tmp}/dt30-flow.txt" 2>&1 && grep -q -F -e '--- PASS: TestCLIRoundtrip' "${TMPDIR:-/tmp}/dt30-flow.txt" && grep -q -F -e '--- PASS: TestTranscribeVerdictValidSignatureConsumed' "${TMPDIR:-/tmp}/dt30-flow.txt" && grep -q -F -e '--- PASS: TestTranscribeVerdictInertEvaluatesNoClause' "${TMPDIR:-/tmp}/dt30-flow.txt" && grep -q -F -e '--- PASS: TestTranscribeVerdictHumanGateEvidenceOnlyNoFlip' "${TMPDIR:-/tmp}/dt30-flow.txt" && grep -q -F -e '--- PASS: TestTranscribeVerdictNegativeBattery' "${TMPDIR:-/tmp}/dt30-flow.txt" && echo ok` | prints `ok` (each test PASSES by name, never vacuously): the path §Landing now names exists end to end in this repo — sign and verify round-trip; a signed verdict is consumed, its Evidence appended and the `gate: model` row flipped; an unarmed lane evaluates nothing; a `gate: human` brief gets Evidence and no flip (the limits Task 1 states). The negative battery is the lower-layer proof: each clause refuses its own forged input with the layers above it bypassed (forged author with a valid signature, tampered signature, and so on). |
| 10 | gate:human +flow | READ FROM THE FORGE, not from this tree: on a repo a verify desk serves, the lane's enactment sign-off resolves to the blessing authority, a verdict filer is bound, and that repo's remote `main` carries at least one commit written by the transcription workflow from a real (non-test) signed verdict issue. The human records the repo, the verdict issue and the landing commit. | Attested before option 1 or 2 is implemented; without it the brief stays at `todo`. Could-not-check at authoring: no such landing was observed from this tree (facts 4 and 6). |
| 11 | check | `rm -f "${TMPDIR:-/tmp}/dt30-lint.txt" && cd statusgen && go build -o "${TMPDIR:-/tmp}/dt30-sg" . && "${TMPDIR:-/tmp}/dt30-sg" --root .. --lint > "${TMPDIR:-/tmp}/dt30-lint.txt" 2>&1; grep -q 'LINT: PASS' "${TMPDIR:-/tmp}/dt30-lint.txt" && test -f ../changelog/desk-tools-30.md && echo ok` | prints `ok`: the tree lints clean and the changelog fragment exists. |
| 12 | check | `! grep -q -e '[:] follow-up desk-tools/30' docs/streams/desk-tools/brief-30-verify-desk-landing-by-verdict-issue.md && cd statusgen && go build -o "${TMPDIR:-/tmp}/dt30-sg" . && "${TMPDIR:-/tmp}/dt30-sg" --root .. --consumers --brief desk-tools/30 > "${TMPDIR:-/tmp}/dt30-cons.txt" 2>&1 && grep -q -F 'summary: 7 corroborated, 0 disproved, 2 unchecked' "${TMPDIR:-/tmp}/dt30-cons.txt" && echo ok` | prints `ok`: no consumer entry still routes to this brief as a follow-up (Task 8; the bracket keeps the pattern from matching this row's own text), and all seven `fixed-here` entries are corroborated by paths in the implementation diff. Run it on the implementation branch (default base `origin/main`). After merge, run it from the delivering change's head with `--base` set to that change's parent, as the instrument's COULD-NOT-CHECK message says. The two `out-of-scope` entries (`width.go`, `verdictrun.go`) report UNCHECKED by design: the reviewer confirms neither file changed and both stated reasons still hold. FAILS today: seven entries still route as follow-ups. |
| 13 | check | `awk '/^## /{f=/^## Cluster rows/} f' plugins/assay/skills/verify-desk/SKILL.md > "${TMPDIR:-/tmp}/dt30-cluster.txt" && awk '/^## /{f=/^## Irreversible briefs/} f' plugins/assay/skills/verify-desk/SKILL.md > "${TMPDIR:-/tmp}/dt30-irrev.txt" && awk '/^## /{f=/^## Landing/} f' plugins/assay/skills/verify-desk/SKILL.md > "${TMPDIR:-/tmp}/dt30-landing.txt" && grep -q 'residual route' "${TMPDIR:-/tmp}/dt30-cluster.txt" && grep -q 'residual route' "${TMPDIR:-/tmp}/dt30-irrev.txt" && grep -q 'not PASS' "${TMPDIR:-/tmp}/dt30-landing.txt" && ! grep -q 'Because this desk lands status straight to main' plugins/assay/skills/verify-desk/SKILL.md && echo ok` | prints `ok`: non-PASS Evidence has a named route. §Landing says Evidence that is not PASS stays out of the verdict (the phrase 'not PASS' on one line), and §Irreversible briefs and §Cluster rows both send their non-lane Evidence by the residual route. FAILS today: 'residual route' appears 0 times in the file and :644 still says the desk lands status straight to main. |
| 14 | check +neighbour | `cd tools/skillslint && go run . --root ../.. > "${TMPDIR:-/tmp}/dt30-sl.txt" 2>&1 && grep -q 'GUARDRAILS: PASS' "${TMPDIR:-/tmp}/dt30-sl.txt" && echo ok` | prints `ok`: skillslint exits 0 and every shared-guardrail copy byte-matches `.claude/guardrails/GUARDRAILS.md`, so the five copies came from the edited source by `make guardrail-sync` and none was hand-edited. Passes today and must still pass; editing the copies without the source (or the source without the sync) makes it exit 1. |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|

## Review
Gate: human (from frontmatter). The reviewer answers, in the verdict: (1) does the rewritten
§Landing anywhere imply that a public tool files the verdict issue (it must not, fact 5); (2) does
any landing class lose its route — outcome records, irreversible-brief Evidence, FAIL verdicts,
non-PASS Evidence rows (row 13) and lane-down landings each have a named path; (3) are the five
copies of the shared sentence byte-identical to their `GUARDRAILS.md` source (rows 6 and 14).
