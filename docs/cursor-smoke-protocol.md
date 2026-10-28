# Cursor smoke protocol — the live-harness acceptance checklist

**Brief**: [harness-portability/13](streams/harness-portability/brief-13-cursor-live-desk-smoke.md)
**Judged against**: the ruled degradation matrix (brief 12) as carried verbatim into the
binding file [`../plugins/assay/references/cursor.md`](../plugins/assay/references/cursor.md).
**Precedent**: [`codex-smoke-protocol.md`](codex-smoke-protocol.md) (brief 07), one harness
earlier. This protocol keeps its shape and adds the one step that document does not have:
a full desk loop (Step 8).

## Why this document exists

CI cannot run Cursor. The verification split for this stream is deliberate:

- **Structural truth is proven in CI** — the neutrality lint, the generated resident-rules
  byte-compare, the `harnessgen cursor` packaging check and the `deskinstall --harness cursor`
  placement check. Those run on every PR and need no live harness.
- **Behavioural truth is proven only on a live Cursor session** — that the resident rules
  actually arrive without pasting, that each skill body actually loads on invocation, that
  each skill actually `runs` / `degrades` / `refuses` exactly as the matrix rules, and that a
  whole desk loop (dispatch, isolation, a draft PR, a review cycle) completes. **No in-repo
  command can corroborate that.**

This protocol makes the live run **repeatable** and its evidence **comparable across
re-runs** — each `cursor-agent` build, and each bundle version bump. It is executed on a real
Cursor install on an environment the maintainer provides or sanctions (never self-procured),
and its evidence is the run log the human gate signs.

## Scope + preconditions

- **Two surfaces, headless first.** The headless `cursor-agent -p` CLI is the **primary**
  surface and carries every step. The in-editor IDE agent is the **secondary** surface and is
  exercised where the matrix distinguishes it (Steps 2, 4 and 5). Every transcript excerpt
  names the surface it came from.
- **Run parameters (recorded on `#619`, 2026-09-07):**
  - `cursor-agent` build **2026.09.02**, headless `-p` surface. A different build is allowed
    but is recorded as such in the run log header; the run is then comparable only to runs of
    the same build.
  - Model **`auto`** — Cursor picks the model per request. The run log records the model
    Cursor **actually served for each step**, read from `--output-format stream-json` where
    the stream reports it. Where it does not, the step is marked `model: unrecorded`; a step
    is never given a guessed model, so runs stay comparable.
  - **Credentials.** The API key lives only in a local credentials file on the runner
    machine. It never enters a repository, a run log, a transcript excerpt or a PR body;
    redact it from any excerpt before pasting.
- **Scratch adopter repository.** Steps 1 and 8 target a scratch adopter repository, named in
  the run log header as `Adopter repo under test`. **It is not named yet** — the maintainer
  names it before the run, and the run log carries the placeholder
  `<scratch-adopter-repo: not yet named>` until then. Step 8 opens its draft PR against that
  repository and no other; a run that cannot name it is `BLOCKED`, never pointed at a
  repository the maintainer did not name.
- **The protocol never hot-fixes.** A failed step routes to a GitHub issue against the owning
  brief's surface (the brief whose artifact the step exercises), recorded in the run log — it
  does not edit the bundle to make the step pass.
- **No fabricated runs.** A step with no transcript excerpt is not evidence, and a `Result:
  PASS` with nothing pasted is not a result. A step the environment cannot reach is
  `BLOCKED`, not skipped and not passed.

## How to run it

1. Copy [`cursor-smoke-runs/TEMPLATE.md`](cursor-smoke-runs/TEMPLATE.md) to
   `docs/cursor-smoke-runs/<date>-<cursor-version>.md` (e.g.
   `docs/cursor-smoke-runs/2026-09-14-cursor-agent-2026.09.02.md`) and fill the header.
2. Work the steps in order. Each step has a literal **Action** and an **`Expect:`** line —
   the observable that decides PASS/FAIL.
3. For every step, paste a **transcript excerpt** from the live session into the run log as
   the step's evidence, record the surface and the served model, and record `Result: PASS` /
   `Result: FAIL` / `Result: BLOCKED`.
4. A `FAIL` step cites the GitHub issue number filed against the owning brief's surface.
5. The completed run log is the artifact the **human gate** signs: a real Cursor install, the
   named build, an excerpt per step, a genuine draft-PR reference for Step 8, and the
   degradations observed matching the ruled matrix.

---

## The steps

Minimum coverage is the eight steps below. Step 3's list is the one skill roster in this
file; it names every skill under `plugins/assay/skills/*/SKILL.md`, and a skill added to or
removed from the bundle needs the same change here.

#### Step 1 — Fresh install per the adopt path

Action: In a clean checkout of the scratch adopter repository with no prior Assay install,
place the Cursor harness exactly as [`adopting-assay.md`](adopting-assay.md)
§ **Running Assay on Cursor** prescribes — `deskinstall --harness cursor --release vX.Y.Z
--repo <repo> --forge github --fetch`, preceded by `--dry-run` and followed by `--check`. Do
**not** deviate from the documented commands; if a documented command fails, that is the
finding.

Expect: the documented commands complete without error and `--check` exits 0; `.cursor/skills/`
holds every skill in Step 3's list, with sibling `.cursor/references/`, `.cursor/rules/assay.mdc`,
`.cursor/assay-managed.json` and the bounded `AGENTS.md` bindings block present; a fresh
`cursor-agent` session discovers the placed skills. A deviation from the runbook needed to
make install succeed is a FAIL routed to harness-portability/12 (packaging/install path).

#### Step 2 — Resident rules present WITHOUT manual pasting, on BOTH surfaces

Action: Start a fresh session on each surface in the adopter repo **without pasting any rule
text into the prompt** — headless `cursor-agent -p` (which reads `AGENTS.md` natively) and the
IDE agent (which reads the generated always-apply `.cursor/rules/assay.mdc`). Ask each session
to state, in its own words, the resident rule on neutral-dispatch wording (rule 3). Probe
wording: `State resident operating rule 3 (neutral-dispatch wording) as you received it.`

Expect: on both surfaces the session reproduces rule 3's substance without having been given
it in-prompt — that dispatches must "describe the work in plain correctness language (wrong
values, forked state, fails-to-fire) ... framed by the observable defect, not by speculative
intent." Silence, "no such rule", or a request to be given the rules on either surface is a
FAIL routed to harness-portability/12 (resident-rules delivery for the Cursor column).

#### Step 3 — Invoke each skill by name (headless) → body loads

Action: On the headless surface, invoke **each** bundled skill by its namespaced name
(`assay:adopt`, `assay:ask-decision`, `assay:author-brief`, `assay:author-spec`,
`assay:cut-release`, `assay:human-runsheet`, `assay:install`, `assay:intake-desk`,
`assay:pdfingest`, `assay:pr-review-desk`, `assay:pr-shepherd`, `assay:session-coach`,
`assay:system-demo`, `assay:the-desk`, `assay:upgrade-assay`, `assay:verify-desk`,
`assay:worker-desk`) and confirm the full SKILL.md body loads (not merely the description).
Paste one identifying line from each loaded body.

Expect: every listed body loads on by-name invocation (invoke-by-name is the availability
floor, `references/cursor.md` § `capability:invoke-skill`). Any skill whose body does not load
is a FAIL routed to harness-portability/12 (packaging coverage).

#### Step 4 — Auto-trigger probe (IDE surface; headless observed)

Action: On the **IDE** surface, issue a natural-language request that matches a skill's
`description` **without naming the skill** — e.g. "fan out the next batch of briefs in
parallel" (should draw `worker-desk`). Confirm the matching body auto-loads via description
matching. Then issue the same request on the headless surface and record what happens.

Expect: on the IDE surface the description-driven match loads the skill body without the name
being spoken (description-driven auto-trigger is documented for Cursor). The headless result
is an **observation, not a pass condition** — the matrix marks headless auto-trigger as a
live-install confirmation — and is recorded either way; a headless run that does not
auto-trigger still has the by-name floor (Step 3) holding, which is a config posture to state,
not a FAIL. An IDE surface that does not auto-trigger is a FAIL routed to harness-portability/12.

#### Step 5 — Dispatch probe → observed `runs` / `degrades` / `refuses` matches the ruled matrix

Action: Invoke `worker-desk` on each surface with a request that would fan out **N ≥ 2** small
items (capture the headless run with `--output-format stream-json`). Record whether dispatch
ran in parallel, ran serially, or refused, and what the session itself said about it.

Expect: the observed outcome matches the matrix cell for that surface exactly — on the IDE
surface `worker-desk` **runs** (cloud background agents in isolated worktrees); headless it
**runs or refuses** depending on whether `git worktree add` is permitted, and where headless
parallel dispatch is not reached it **degrades to serial, one item at a time, with the
degradation stated by the session**. Every one of the N items completes (none silently
skipped), and each still receives its own isolated worktree, command evidence and review. A
serial run that does not state it is a FAIL routed to harness-portability/12; a run that drops
an item or weakens isolation, evidence or the review gate to keep up is a floor breach — a
FAIL routed to harness-portability/04 and flagged as a safety-floor violation, not a mere
degradation.

#### Step 6 — Isolation probe → refusal fires where ruled

Action: On the headless surface, invoke `worker-desk` under a permission posture that does
**not** permit `git worktree add` (the default headless posture, if it blocks that ref-write)
and ask it to implement a brief. Then, separately, re-run under the posture the maintainer
sanctions for worktree creation, recording the exact flags used for each.

Expect: under the blocking posture the skill **refuses rather than implement in the shared
checkout** (the non-negotiable isolation floor — isolation never silently degrades); under the
permitting posture it creates its own worktree and runs. Which posture permits the ref-write is
itself a live-install confirmation; record the observation. A skill that proceeds to implement
in the shared checkout when it cannot isolate is a **floor breach** — a FAIL routed to
harness-portability/04 and flagged as a safety-floor violation.

#### Step 7 — Evidence-discipline probe → a Verify row executed and recorded

Action: Invoke `verify-desk` and have it execute one real Verify row from any brief's Verify
table, recording the command, its exit code, and output.

Expect: the skill records **real command evidence** (command + exit code + output), never a
bare claim of success — the `evidence-not-claims` guarantee is intact (it never degrades). A
verdict asserted without command evidence is a FAIL routed to harness-portability/04 and
flagged as a safety-floor violation.

#### Step 8 — Full desk loop: one worker-desk item to a draft PR, then one pr-review-desk review cycle

Action: On the **headless** surface, against the scratch adopter repository named in the run
log header (real repository content, not a toy), dispatch **one small** `worker-desk` item end
to end — the implementer works in its own isolated worktree, commits, and opens a **draft PR**
— then run **one** `pr-review-desk` cycle against that PR. The forge writes need a
`gh`/`glab` credential scoped to the scratch repository only and sanctioned by the maintainer;
that credential is also kept out of every excerpt. Paste the dispatch transcript, the PR URL
and the reviewer's verdict.

Expect: a draft PR URL/number exists on the scratch adopter repository, carries a real diff
(not an empty or placeholder change), and received exactly one reviewer verdict from the
`pr-review-desk` cycle; the isolation and evidence floors from Steps 5–7 hold throughout and
are not re-relaxed for this step; the review gate is upheld (`pr-review-desk` owns the
draft-to-ready flip and only after its own approval; a human merges; no skill merges or closes
the PR, and the worker never flips it ready). A loop that stops before a PR exists, or reaches a PR with no
review cycle, is a FAIL (or `BLOCKED` where the environment, not the bundle, is the cause)
routed to the owning surface of the skill that stopped — and a PR number invented for the
run log is the vacuous-green failure this stream forbids.

---

## Status of the runs

- **No run exists yet.** The first Cursor live smoke is set by the maintainer's ruling on
  `#3699`; its run parameters are recorded on `#619` (above). Until the run log is signed,
  Step 8's acceptance row is **BLOCKED on the live run**: the honest state is "ready for the
  live run" — calling the stream done from this protocol text alone is the vacuous-green
  failure this stream forbids.
- **Still owed before the run:** the maintainer names the scratch adopter repository for
  Steps 1 and 8. This document and the run-log scaffold deliberately leave it a placeholder.
