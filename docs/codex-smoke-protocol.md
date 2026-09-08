# Codex smoke protocol — the live-harness acceptance checklist

**Brief**: [harness-portability/07](streams/harness-portability/brief-07-adoption-live-smoke.md)
**Judged against**: the ruled degradation matrix (Decision C) as carried
verbatim into the binding file [`../plugins/assay/references/codex.md`](../plugins/assay/references/codex.md).

## Why this document exists

CI cannot run Codex. The verification split for this stream is deliberate:

- **Structural truth is proven in CI** — harness-portability/04's neutrality lint,
  /05's resident-rules generation byte-compare, /06's Codex packaging coverage +
  manifest byte-compare. Those checks run on every PR and need no live harness.
- **Behavioural truth is proven only on a live Codex session** — that the resident
  rules actually arrive without pasting, that each skill body actually loads on
  invocation, and that each skill actually `runs` / `degrades` / `refuses` exactly as
  the matrix rules. **No in-repo command can corroborate that.**

This protocol makes the live run **repeatable** and its evidence **comparable across
re-runs** — each Codex release, and each bundle version bump. It is executed on a real
Codex CLI on an environment Ian provides or sanctions (never self-procured), and its
evidence is the run log the human gate signs.

## Scope + preconditions

- **Target: Codex CLI only** (Decision A1 — the hosted App is HP/08, out of scope here).
- The runner has a working `codex` CLI, an OpenAI account, and network access — an
  environment **only Ian provides or sanctions**.
- Two sandbox postures are exercised where the matrix distinguishes them:
  `workspace-write` (the default) and `--sandbox danger-full-access`. Several skills'
  ruled posture differs between the two (`worker-desk` refuses under the former, runs
  under the latter); a step that names a posture MUST be run under that posture.
- **The protocol never hot-fixes.** A failed step routes to a GitHub issue against the
  owning brief's surface (the brief whose artifact the step exercises), recorded in the
  run log — it does not edit the bundle to make the step pass.

## How to run it

1. Copy the **run-log skeleton** at the bottom of this file to
   `docs/codex-smoke-runs/<date>-<codex-version>.md` (e.g.
   `docs/codex-smoke-runs/2026-09-01-codex-0.44.0.md`).
2. Work the steps in order. Each step has a literal **Action** and an **`Expect:`**
   line — the observable that decides PASS/FAIL.
3. For every step, paste a **transcript excerpt** from the live session into the run
   log as the step's evidence. *A run-log entry with no transcript excerpt is not
   evidence.* Record `Result: PASS` / `Result: FAIL` / `Result: BLOCKED` per step.
4. A `FAIL` step cites the GitHub issue number filed against the owning brief's surface.
5. The completed run log is the artifact the **human gate** signs: a real Codex, the
   named version, an excerpt per step, and the degradations observed matching the ruled
   matrix.

---

## The steps

Minimum coverage is the seven steps below. (The bundle currently ships **nine** skills —
`adopt`, `author-brief`, `dailies`, `intake-desk`, `market-intelligence`,
`pr-review-desk`, `the-desk`, `verify-desk`, `worker-desk`; Step 3 iterates over all of
them, and Steps 5/6 probe the specific skills whose matrix cell is not a bare `runs`.)

#### Step 1 — Fresh install per the adopt path

Action: On a clean machine (or a clean checkout with no prior Assay install), install
the Assay Codex bundle exactly as the adoption path in
[`adopting-assay.md`](adopting-assay.md) prescribes — the git/`/plugin install` of the
`.codex-plugin` bundle, then place the `AGENTS.md` resident-rules fragment. Do **not**
deviate from the documented commands; if a documented command fails, that is the finding.

Expect: the documented install commands complete without error; `codex` reports the
`assay` plugin present and the nine skills discoverable (a `skills`/plugin listing shows
each of the nine names). A deviation from the runbook needed to make install succeed is
a FAIL routed to harness-portability/06 (packaging/install path).

#### Step 2 — Resident rules present WITHOUT manual pasting

Action: Start a fresh Codex session in the adopter repo with the fragment placed but
**without pasting any rule text into the prompt**. Ask the session to state, in its own
words, the resident rule on neutral-dispatch wording (rule 3). Probe wording:
`State resident operating rule 3 (neutral-dispatch wording) as you received it.`

Expect: the session reproduces rule 3's substance without having been given it in-prompt
— that dispatches must "describe the work in plain correctness language (wrong values,
forked state, fails-to-fire) ... framed by the observable defect, not by speculative
intent." Silence, "no such rule", or a request to be given the rules is a FAIL routed to
harness-portability/05 (resident-rules delivery).

#### Step 3 — Invoke each skill by name → body loads

Action: For **each** of the nine bundled skills, invoke it by its namespaced name
(`assay:adopt`, `assay:author-brief`, `assay:dailies`, `assay:intake-desk`,
`assay:market-intelligence`, `assay:pr-review-desk`, `assay:the-desk`,
`assay:verify-desk`, `assay:worker-desk`) and confirm the full SKILL.md body loads (not
merely the description). Paste one identifying line from each loaded body.

Expect: all nine bodies load on by-name invocation (invoke-by-name is the availability
floor, `references/codex.md` §`capability:invoke-skill`). Any skill whose body does not
load is a FAIL routed to harness-portability/06 (packaging coverage).

#### Step 4 — Auto-trigger probe

Action: With `allow_implicit_invocation` at its default (`true`,
`agents/openai.yaml`), issue a natural-language request that matches a skill's
`description` **without naming the skill** — e.g. "fan out the next batch of briefs in
parallel" (should draw `worker-desk`). Confirm the matching body auto-loads via
description matching.

Expect: the description-driven match loads the skill body without the name being spoken,
per HP/01 §3.3 (`auto-trigger` = `supported` on Codex CLI). If auto-trigger is disabled
in the environment, record that posture and confirm the by-name floor (Step 3) still
holds — a disabled auto-trigger is a config posture, not a FAIL, but MUST be stated.

#### Step 5 — Dispatch probe → runs / degrades / refuses per the matrix

Action: Exercise a dispatch-bearing skill (`the-desk`, `worker-desk`, or a `dailies`
fan-out) with `[features] multi_agent` **off** in `~/.codex/config.toml`. Observe
whether the fan-out runs in parallel, runs serially, or refuses.

Expect: the ruled posture from `references/codex.md` is observed AND **stated in-session**
— with `multi_agent` off, the fan-out **degrades: runs serially, one item at a time,
with an explicit in-session statement of the degradation** (the one permitted
convenience degradation). A silent serial run (degradation not stated) is a FAIL routed
to harness-portability/04 (the skill body's neutral-core degradation text); the
guarantees themselves are never degraded.

#### Step 6 — Isolation probe → refusal fires where ruled

Action: Invoke `worker-desk` under the **default `workspace-write` sandbox** (worktree
creation blocked, HP/01 §3.8/§3.9) and ask it to implement a brief. Then, separately,
re-run under `--sandbox danger-full-access`.

Expect: under `workspace-write` the skill **refuses rather than implement in the shared
checkout** (the non-negotiable isolation floor — isolation never silently degrades);
under `danger-full-access` it creates its own worktree and runs. A skill that proceeds to
implement in the shared checkout under `workspace-write` is a **floor breach** — a FAIL
routed to harness-portability/04 and flagged as a safety-floor violation, not a mere
degradation.

#### Step 7 — Evidence-discipline probe → a Verify row executed and recorded

Action: Invoke `verify-desk` and have it execute one real Verify row from any brief's
Verify table, recording the command, its exit code, and output.

Expect: the skill records **real command evidence** (command + exit code + output), never
a bare claim of success — the `evidence-not-claims` guarantee is intact (never degrades).
A verdict asserted without command evidence is a FAIL routed to harness-portability/04
and flagged as a safety-floor violation.

---

## Run-log skeleton

Copy the block below into `docs/codex-smoke-runs/<date>-<codex-version>.md` and fill it.
Every step needs a `Result:` line and a transcript excerpt; a step with no excerpt is not
evidence. `Result:` is one of `PASS` / `FAIL` / `BLOCKED`. A `FAIL` cites the issue filed
against the owning brief's surface.

```
# Codex smoke run — <date> — codex <version>

Runner: <account/name — must be Ian or a session Ian sanctioned>
Codex version: <codex --version output>
Sandbox postures exercised: workspace-write, danger-full-access
Bundle version under test: <plugins/assay/.claude-plugin/plugin.json version>

Step 1: Fresh install per the adopt path
  Action taken: ...
  Transcript excerpt:
    <paste>
  Result: PASS | FAIL | BLOCKED
  Issue (if FAIL): owner/repo#<n> (harness-portability/06)

Step 2: Resident rules present without manual pasting
  Action taken: ...
  Transcript excerpt:
    <paste — the session stating rule 3 unprompted>
  Result: PASS | FAIL | BLOCKED
  Issue (if FAIL): owner/repo#<n> (harness-portability/05)

Step 3: Invoke each skill by name -> body loads
  Action taken: invoked all nine assay:* skills by name
  Transcript excerpt:
    <paste one identifying line per loaded body>
  Result: PASS | FAIL | BLOCKED
  Issue (if FAIL): owner/repo#<n> (harness-portability/06)

Step 4: Auto-trigger probe
  Action taken: ...
  Transcript excerpt:
    <paste>
  Auto-trigger posture: enabled | disabled (state it)
  Result: PASS | FAIL | BLOCKED
  Issue (if FAIL): owner/repo#<n> (harness-portability/06)

Step 5: Dispatch probe (multi_agent off) -> degrades: serial-with-statement
  Action taken: ...
  Transcript excerpt:
    <paste — the in-session degradation statement>
  Result: PASS | FAIL | BLOCKED
  Issue (if FAIL): owner/repo#<n> (harness-portability/04)

Step 6: Isolation probe -> refuses under workspace-write, runs under danger-full-access
  Action taken: ...
  Transcript excerpt:
    <paste — the refusal under workspace-write>
  Result: PASS | FAIL | BLOCKED
  Issue (if FAIL, floor breach): owner/repo#<n> (harness-portability/04)

Step 7: Evidence-discipline probe -> real command evidence recorded
  Action taken: ...
  Transcript excerpt:
    <paste — command + exit code + output>
  Result: PASS | FAIL | BLOCKED
  Issue (if FAIL, floor breach): owner/repo#<n> (harness-portability/04)

Human gate sign-off: <account> confirms — real Codex, version as listed, an excerpt per
step, degradations observed match the ruled matrix, failures routed to issues.
```

## Status of the first run

**BLOCKED — no live Codex environment.** This document is the protocol; the first
executed run log (`docs/codex-smoke-runs/<date>-<codex-version>.md`) is authored when Ian
provides or sanctions a Codex-capable runner. Rows 1–6 of the brief's Verify table can
green from the artifacts in this PR; the brief's acceptance row (the run log) stays
BLOCKED until that environment exists, and the stream is **not done** until it is signed —
"ready for the live run" is the honest state, and calling it done from the protocol text
alone is the vacuous-green failure this stream forbids.
