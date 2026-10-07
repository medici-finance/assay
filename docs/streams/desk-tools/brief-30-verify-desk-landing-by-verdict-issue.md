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
  This brief changes WHO may write the project's `main` branch for verify landings: the shipped
  doctrine moves the verify desk off its standing direct-push grant and onto a signed-issue lane that
  a workflow lands. That grant is a human authorization in the first place, so narrowing or
  re-routing it is a human ruling, not a model call. The human confirms (1) which of the options in
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
    tools/desk/internal/deskkit/{ratelimit,width}.go, tools/desk/cmd/verifyloop/verdictrun.go,
    statusgen/{transcribeverdict,transcribescan,main}.go and .github/workflows/ all re-read at that
    commit before the Task text was written; every line number below is at that commit."
  - "correction carried, not repeated: the house cutover brief's claim that the lane has been landing
    real verdicts since an earlier date is stale and is not repeated here — nothing in this tree
    shows a live lane (facts 4 and 6), and the claim was found stale downstream. Whether any repo's
    lane is live TODAY is exactly the precondition the human attests."
exec-tier: strong
exec-tier-why: "(b) cross-artifact doctrine: one shared policy sentence is rewritten identically in five
  skill bodies plus the desk meter's rationale, and a drift between them is invisible to every test."
consumers:
  - "plugins/assay/skills/verify-desk/SKILL.md: follow-up desk-tools/30 (this brief; Tasks 1-5)"
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
`plugins/assay/skills/pr-shepherd/SKILL.md`, `tools/desk/internal/deskkit/ratelimit.go`,
`changelog/desk-tools-30.md` (planned)

single-point-of-failure: NOT a single control. The lane's landing rests on four layers that fail
on different signals in different components: the enactment gate (a human sign-off resolved
through the API to the blessing authority, User-typed), the RS256 signature over the verdict body
against a public key held as a repo variable, the issue author and body-unedited timeline checks,
and the network-off re-execution of `check:ci` rows. Break-glass `deskevidence` keeps its own,
independent guards (main-push switch, repo allowlist, body scan, rate limit, attribution check).
This brief moves doctrine between those two paths; it removes no guard from either.

facts:
  - "verify-desk SKILL.md:435 — heading '## Landing — deskevidence is the SOLE main-push carve-out
    (narrow, dated)'; :441-446 the carve-out grant (Evidence rows + status flips commit
    straight to `main` as the verifier App); :448-456 the `deskevidence` Interface paragraph;
    :458-461 the guards paragraph; :492-601 `### Public repo (PR-required main) — Evidence lands by
    PR` (the same `deskevidence`, aimed at a branch plus a draft PR)."
  - "verify-desk SKILL.md:488-490 — the `Standing-doctrine pointer (2026-08-17)` paragraph: the
    lane 'would replace this path'; it cites the directory docs/streams/verdict-lane/, which does not exist in
    this tree (a dangling private path). :179 step 3 says 'Land each verdict as it returns via
    `deskevidence`'; :196-197 says filing the signed payload 'is the autonomous cutover, `gate:
    human`'. :815-818 (desk-specific) says the desk lands everything else 'via the push race loop
    (`commit → pull --rebase → push`, retry on race)'. Today the file contains 'break-glass' 0
    times and 'transcribe-verdict' 0 times."
  - "The inherited `Git push policy (ONE policy, role-keyed)` block carries one identical sentence —
    'The verify desk lands its own work: its Evidence + status flips commit straight to `main` as
    the project directs' — in five skills: the-desk SKILL.md:177-178, pr-shepherd :222-223,
    pr-review-desk :989-990, worker-desk :773-774, verify-desk :809-810. No lint checks the five
    copies agree; that is why this brief verifies all five (row 5)."
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
    (`deskevidence --outcome-record`, SKILL.md:255 and :398-414), Evidence for
    `risk.irreversible: yes` briefs (refused by cl.5; the human flips via the verify-gate issue,
    SKILL.md:641-676), and any landing while the lane is down or unarmed."

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

Three things the second path does not cover, whichever option is picked: results for work marked
irreversible (a human signs those off separately), the per-verdict outcome records, and any
landing while the second path is switched off or broken. Under options 1 and 2 those go through
the direct-write tool's pull-request form (a branch plus a draft pull request, never a direct
write to the main branch).

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

Recommendation: **Option 2.** It narrows the widest write permission wherever the safer path is
proven, and strands no installation that has not switched it on.

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
Written for option 2 (the recommendation). Under option 1, drop the "where the project has armed
the lane" qualifier and the "unarmed project" paragraph in step 1; everything else is the same.

### 1. Rewrite `## Landing` in `plugins/assay/skills/verify-desk/SKILL.md` (:435-490)
- Retitle the heading to: ## Landing — a verdict lands by signed verdict issue; `deskevidence` is
  break-glass.
- Open with the landing path, in order: the verifier session's `verifyrun` rows are composed and
  signed by `verifyloop verdict`; the project's **verdict filer** files that signed body as a
  verify-verdict issue; the project's transcription workflow runs `statusgen --transcribe-verdict`,
  which re-checks author, signature and body-unedited, re-runs `check:ci` rows network-off, and is
  the sole writer of the resulting Evidence and `gate: model` flip on `main`. No session pushes
  `main` on this path.
- State the lane's limits plainly: PASS only (a FAIL verdict transcribes nothing — the FAIL route
  in §On VERIFY: FAIL is unchanged); a `gate: human` brief gets Evidence only; an
  `risk.irreversible: yes` brief and any `human:` stamp are refused, so irreversible-brief Evidence
  and outcome records take the residual route below.
- Keep the "witness lands WITH the Evidence" paragraph and the "PASS is a flip signal only when its
  own Evidence agrees" paragraph; they bind the payload the lane carries as much as a direct landing.
- Add the unarmed-project paragraph: where the project has not armed the lane (its enactment
  sign-off does not resolve, or no verdict filer is bound), `deskevidence` stays this desk's
  standing landing path exactly as before; reading that state is could-not-check, never "armed".
- Keep "Land as each verdict arrives" and the desk-verbs sentence.

### 2. Demote `deskevidence` to break-glass — documented, not deleted
- Add a subsection headed ### Break-glass — `deskevidence`, holding the existing Interface paragraph
  (:448-456) and guards paragraph (:458-461) verbatim, introduced by: in an armed project,
  `deskevidence` lands to `main` only when the lane is down, on the driver's explicit say-so for
  that landing, recorded on the brief's Evidence as break-glass.
- Keep `### Public repo (PR-required main) — Evidence lands by PR` (:492-601) and name it as the
  **residual route**: outcome records and irreversible-brief Evidence land through it (branch plus
  draft PR), never a direct `main` push, in an armed project.
- Rewrite the dated authorization sentence (:441-446) so it states the grant as break-glass plus
  the unarmed-project standing path, still "nothing else, nobody else".

### 3. Delete the stale pointers
- Delete the `Standing-doctrine pointer (2026-08-17)` paragraph (:488-490), including its
  docs/streams/verdict-lane/ path, which resolves nowhere in this repo.
- In the desk-specific bullet (:815-818) replace "via the push race loop (`commit → pull --rebase →
  push`, retry on race)" with a pointer to §Landing. Keep the two landings the desk does NOT do.

### 4. Bring §The loop into line
- :179 step 3 → "Land each verdict as it returns, by §Landing (the verdict lane where armed,
  `deskevidence` otherwise)".
- :196-197 → "`verifyloop verdict` composes and signs the verdict; the project's verdict filer
  files it (§Landing)". Delete "the autonomous cutover, `gate: human`".

### 5. The shared push-policy sentence, identical in all five skills
In `plugins/assay/skills/{the-desk,pr-review-desk,worker-desk,pr-shepherd,verify-desk}/SKILL.md`,
replace the sentence quoted in fact 3 with this text, byte-identical in all five:

> **The verify desk lands verdicts through the project's verdict-transcription lane** where the
> project has armed it — a signed verdict issue that a workflow lands on `main`, never a session
> push; where the lane is not armed it lands per its own skill's §Landing. No push-go is needed and
> none should be waited for.

Leave every other line of each block untouched.

### 6. Annotate the rate-limit carve-out (`tools/desk/internal/deskkit/ratelimit.go:110-127`)
Append to the `unnumberedBucketCap` comment: in a project that has armed the verdict-transcription
lane, `deskevidence` is break-glass plus the residual route (outcome records, irreversible-brief
Evidence), and routine verdict landings meter under `VerdictIssueTool`; the cap stays at its
value because it still serves unarmed projects and the width arm in `width.go` reads it. The word
`break-glass` appears in the comment. No code change.

### 7. Changelog
Add `changelog/desk-tools-30.md` (planned) with a `### Changed` bullet naming the doctrine change.

## Verify (executable — no prose-only DoD items)
Rows 1-7 are discriminating: each FAILS against today's tree (at 36113a1dd) and passes only once
the Task lands. Run every row from the repo root.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `grep -q '^## Landing' plugins/assay/skills/verify-desk/SKILL.md && ! grep -q 'SOLE main-push carve-out' plugins/assay/skills/verify-desk/SKILL.md && echo ok` | prints `ok`. FAILS today: the heading still reads 'SOLE main-push carve-out'. |
| 2 | check +dereference | `rm -f "${TMPDIR:-/tmp}/dt30-help.txt" && cd statusgen && go build -o "${TMPDIR:-/tmp}/dt30-sg" . && "${TMPDIR:-/tmp}/dt30-sg" --help > "${TMPDIR:-/tmp}/dt30-help.txt" 2>&1; grep -q -e '-transcribe-verdict' "${TMPDIR:-/tmp}/dt30-help.txt" && grep -q -e '--transcribe-verdict' ../plugins/assay/skills/verify-desk/SKILL.md && grep -q 'verdict filer' ../plugins/assay/skills/verify-desk/SKILL.md && grep -q 'verifyloop verdict' ../plugins/assay/skills/verify-desk/SKILL.md && echo ok` | prints `ok`: the skill names the transcriber mode AND that mode exists in the built binary's flag set (not a dangling name), plus the filer and the signer. FAILS today: the skill names `--transcribe-verdict` 0 times. |
| 3 | check | `grep -q -i 'break-glass' plugins/assay/skills/verify-desk/SKILL.md && grep -q 'deskevidence --help' plugins/assay/skills/verify-desk/SKILL.md && grep -q 'VERIFIER_MAIN_OK' plugins/assay/skills/verify-desk/SKILL.md && grep -q 'PR-required main' plugins/assay/skills/verify-desk/SKILL.md && echo ok` | prints `ok`: demoted, NOT deleted — the Interface, the main-push switch and the PR form survive. FAILS today: 'break-glass' appears 0 times. |
| 4 | check | `! grep -q -i 'push race' plugins/assay/skills/verify-desk/SKILL.md && ! grep -q 'Standing-doctrine pointer' plugins/assay/skills/verify-desk/SKILL.md && ! grep -q 'docs/streams/verdict-lane/' plugins/assay/skills/verify-desk/SKILL.md && ! grep -q 'the autonomous cutover' plugins/assay/skills/verify-desk/SKILL.md && echo ok` | prints `ok`. FAILS today: all four strings are present. |
| 5 | check +neighbour | `rm -f "${TMPDIR:-/tmp}/dt30-lane.txt" && grep -L -F 'verdict-transcription lane' plugins/assay/skills/the-desk/SKILL.md plugins/assay/skills/pr-review-desk/SKILL.md plugins/assay/skills/worker-desk/SKILL.md plugins/assay/skills/pr-shepherd/SKILL.md plugins/assay/skills/verify-desk/SKILL.md > "${TMPDIR:-/tmp}/dt30-lane.txt"; ! grep -r -q 'straight to .main. as the project directs' plugins/assay/skills && test -f "${TMPDIR:-/tmp}/dt30-lane.txt" && [ ! -s "${TMPDIR:-/tmp}/dt30-lane.txt" ] && grep -l 'Git push policy (ONE policy, role-keyed)' plugins/assay/skills/the-desk/SKILL.md plugins/assay/skills/pr-review-desk/SKILL.md plugins/assay/skills/worker-desk/SKILL.md plugins/assay/skills/pr-shepherd/SKILL.md plugins/assay/skills/verify-desk/SKILL.md > "${TMPDIR:-/tmp}/dt30-pol.txt" && [ $(wc -l < "${TMPDIR:-/tmp}/dt30-pol.txt") -eq 5 ] && echo ok` | prints `ok`: the old sentence is gone from every skill, all five carry the new one, and all five still carry the policy block. FAILS today: the old sentence is in all five. |
| 6 | check | `grep -c -F 'The verify desk lands verdicts through the project' plugins/assay/skills/the-desk/SKILL.md plugins/assay/skills/pr-review-desk/SKILL.md plugins/assay/skills/worker-desk/SKILL.md plugins/assay/skills/pr-shepherd/SKILL.md plugins/assay/skills/verify-desk/SKILL.md > "${TMPDIR:-/tmp}/dt30-same.txt"; ! grep -q -v ':1$' "${TMPDIR:-/tmp}/dt30-same.txt" && echo ok` | prints `ok`: exactly one copy of the new sentence's opening in each of the five. FAILS today: every count is 0. |
| 7 | check | `grep -q -i 'break-glass' tools/desk/internal/deskkit/ratelimit.go && grep -q 'deskevidenceUnnumberedCap = 30' tools/desk/internal/deskkit/ratelimit.go && grep -q 'const VerdictIssueTool = "verifyloop-verdict"' tools/desk/internal/deskkit/ratelimit.go && echo ok` | prints `ok`: the carve-out is annotated, its value and the verdict-issue meter are unchanged. FAILS today: 'break-glass' appears 0 times in the file. |
| 8 | check +neighbour | `cd tools/desk && go test -count=1 ./internal/deskkit/ ./cmd/deskevidence/ ./cmd/verifyloop/ ./cmd/deskverdict/` | exit 0: the cap, the width arm (`TestMaxWidth_IsBoundedByTheEnforcedBudget`), the per-tool override test and the break-glass tool itself are unaffected. Passes today and must still pass. |
| 9 | check +flow | `cd tools/desk && go test -count=1 -v -run '^TestCLIRoundtrip$' ./cmd/deskverdict/ > "${TMPDIR:-/tmp}/dt30-flow.txt" 2>&1 && cd ../../statusgen && go test -count=1 -v -run '^TestTranscribeVerdictValidSignatureConsumed$' . >> "${TMPDIR:-/tmp}/dt30-flow.txt" 2>&1 && go test -count=1 -v -run '^TestTranscribeVerdictInertEvaluatesNoClause$' . >> "${TMPDIR:-/tmp}/dt30-flow.txt" 2>&1 && go test -count=1 -v -run '^TestTranscribeVerdictHumanGateEvidenceOnlyNoFlip$' . >> "${TMPDIR:-/tmp}/dt30-flow.txt" 2>&1 && grep -q -F -e '--- PASS: TestCLIRoundtrip' "${TMPDIR:-/tmp}/dt30-flow.txt" && grep -q -F -e '--- PASS: TestTranscribeVerdictValidSignatureConsumed' "${TMPDIR:-/tmp}/dt30-flow.txt" && grep -q -F -e '--- PASS: TestTranscribeVerdictInertEvaluatesNoClause' "${TMPDIR:-/tmp}/dt30-flow.txt" && grep -q -F -e '--- PASS: TestTranscribeVerdictHumanGateEvidenceOnlyNoFlip' "${TMPDIR:-/tmp}/dt30-flow.txt" && echo ok` | prints `ok` (each test PASSES by name, never vacuously): the path §Landing now names exists end to end in this repo — sign and verify round-trip; a signed verdict is consumed, its Evidence appended and the `gate: model` row flipped; an unarmed lane evaluates nothing; a `gate: human` brief gets Evidence and no flip (the limits Task 1 states). |
| 10 | gate:human +flow | READ FROM THE FORGE, not from this tree: on a repo a verify desk serves, the lane's enactment sign-off resolves to the blessing authority, a verdict filer is bound, and that repo's remote `main` carries at least one commit written by the transcription workflow from a real (non-test) signed verdict issue. The human records the repo, the verdict issue and the landing commit. | Attested before option 1 or 2 is implemented; without it the brief stays at `todo`. Could-not-check at authoring: no such landing was observed from this tree (facts 4 and 6). |
| 11 | check | `rm -f "${TMPDIR:-/tmp}/dt30-lint.txt" && cd statusgen && go build -o "${TMPDIR:-/tmp}/dt30-sg" . && "${TMPDIR:-/tmp}/dt30-sg" --root .. --lint > "${TMPDIR:-/tmp}/dt30-lint.txt" 2>&1; grep -q 'LINT: PASS' "${TMPDIR:-/tmp}/dt30-lint.txt" && test -f ../changelog/desk-tools-30.md && echo ok` | prints `ok`: the tree lints clean and the changelog fragment exists. |
| 12 | check | `cd statusgen && go build -o "${TMPDIR:-/tmp}/dt30-sg" . && "${TMPDIR:-/tmp}/dt30-sg" --root .. --consumers --brief desk-tools/30 > "${TMPDIR:-/tmp}/dt30-cons.txt" 2>&1 && grep -q -F ' 0 disproved' "${TMPDIR:-/tmp}/dt30-cons.txt" && echo ok` | prints `ok`: every `follow-up desk-tools/30` routing is corroborated by the implementation diff and none is disproved. The two `out-of-scope` entries (`width.go`, `verdictrun.go`) report UNCHECKED by design: the reviewer confirms neither file changed and both stated reasons still hold. |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|

## Review
Gate: human (from frontmatter). The reviewer answers, in the verdict: (1) does the rewritten
§Landing anywhere imply that a public tool files the verdict issue (it must not, fact 5); (2) does
any landing class lose its route — outcome records, irreversible-brief Evidence, FAIL verdicts,
and lane-down landings each have a named path; (3) are the five copies of the shared sentence
byte-identical (row 6).
