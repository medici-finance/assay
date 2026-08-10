---
id: critical-review-2026-07
date: 2026-07-17
title: "First periodic critical-thinking review — anti-Rube-Goldberg methodology audit"
reviewer: strong-tier (original session tier)
---

# Critical-Review 2026-07 — Methodology surface, first periodic run

This is the first periodic zero-based read of the full methodology surface under
the stable-vs-greenfield lens. The methodology grew mechanism-by-mechanism under
greenfield conditions: each rule, check, and loop was added to stop a specific
incident, and nothing ever asked whether the accumulated pile still makes sense
as the system stabilizes. Stable systems need LESS process than the greenfield
that accreted it, and unreviewed accretion is how a working system becomes a
Rube Goldberg machine.

This run READS the whole surface but WRITES only this doc + register entries.
It never edits skills, statusgen, or CLAUDE.md directly; changes route through
findings/briefs. The next run is due 2026-08-17 (monthly cadence, re-clocked by
methodology/38 when landed).

---

## Previous run

None. This is the first periodic critical-thinking review. It establishes the
baseline: every mechanism enumerated, every verdict argued, every chain named.
Subsequent runs open by reviewing whether this run's retirements happened and
whether its keeps rotted.

---

## Verdicts

The surface is walked in the order listed in the brief's enumerated surface.
Each row applies the four questions: (a) what problem did it solve when added;
(b) does that problem still exist; (c) would we build it today; (d) what would
LESS look like.

| # | Mechanism | Built-for (incident/brief) | Still needed? | Verdict | Evidence |
|---|-----------|---------------------------|---------------|---------|----------|
| 1 | **the-desk skill** (coordinator persona, ~200 lines) | methodology/01 — single arbiter across streams; the role split to four windows (2026-07-09) when PR-monitor churn fragmented deep coordination | Yes — arbitration across streams still needs one owner | **keep** | The split solved a real problem (the coordinator grinding through per-minute PR monitor cycles), but the skill has accreted operating rules to ~200 lines. Candidate for a diet: the "boot sequence" is now blind — it references `../oit/docs/needs-fixing.md` which was retired 2026-07-10. The extraction note (project-agnostic half → user-level core) is still unimplemented. |
| 2 | **batch-fanout skill** (~205 lines) | methodology/02 — parallel dispatch; serial implementation was too slow. The 8-worker pool replaced single-wave dispatch (human:<name>, 2026-07-16) | Yes — dispatch is the throughput engine | **simplify** | The mechanism is needed but the skill body has grown 205 lines of which ~40% is guardrail restatements (insight-routing, escalation labels, git push policy, no-attribution) that also live in the-desk, pr-review-desk, and verify-desk. These rules should live ONCE (in CLAUDE.md or a shared guardrails include), not restated per skill. The claims-lock mechanism (step 2 in procedure) has three layers: claims file, branch-as-claim, PR-scan clone detection; two would suffice. |
| 3 | **pr-review-desk skill** (~283 lines) | methodology/03 — pre-merge review loop; reviewer App identity added by methodology/brief-17 to make reviews attributable | Yes — pre-merge review is a structural gate | **keep** | The longest skill body; the review loop itself is sound. The deskboard.go tool is well-scoped. The "reviewer identity" section now carries a long self-correction about advisory-vs-enforcement (assay-toolkit#37/#38) — the correction is honest and needed, but it also reveals the mechanism is more complex than its value warrants: attribution without enforcement is commentary with a bot avatar. Still, the App review signal is the desk's flip trigger and it works. |
| 4 | **verify-desk skill** (~258 lines) | methodology/04 — merged briefs rotted at `implemented` with no owner (2026-07-08 late observation: "the verified-flip had no owner") | Yes — the Awaiting queue is a real bottleneck | **simplify** | The autonomy rules (FILE, don't ask; DRAIN, don't wait; WRITE FIRST) were added after the #541 incident (an afternoon of verification lost to parked questions). They are necessary but their repetition across three sections (Autonomy, Loop, Rules) is bloated. The reversible-brief carve-out (irreversible = checkpoint PR for human:<name>) is well-designed but the text explaining it spans ~60 lines across two sections. The "land-as-you-go" push-race-loop pattern is correct but is described three times (Boot step 2, Loop step 4, Rules). |
| 5 | **issue-loop skill** | F-41 (2026-07-16) — the inbound half that was distributed across pr-review-desk and the-desk got its own window | Yes, but the justification is thin | **keep (watch)** | This is the newest mechanism (4 days old). The original design deliberately chose "no fourth standing window" (README, 2026-07-12); human:<name> revised that on 2026-07-16. The model-tier requirement (strong/smart tier only) is correct for judgment work but adds cost. The board tool doesn't exist yet (MVP is two hand-run statusgen commands). First candidate for retirement in a future run if the distributed model was actually sufficient. |
| 6 | **author-brief skill** (project layer) | methodology/05 — split brief-authoring conventions across user-level core + project thin-wrapper | Yes — the split is correct | **keep** | The user-level/project-layer split is clean and the thin-wrapper pattern is exactly the right amount of mechanism. The model-tier gate and effort-keyed execution rules are load-bearing. The `value:` and `exec-tier:` fields were added incrementally (methodology-metrics/14, methodology/29) and the accretion is visible but each field earns its keep. |
| 7 | **Findings register** (per-entry files, `docs/streams/findings/`) | F-01 — knowledge invalidating briefs needed tracking; enforced by statusgen flagging affected briefs | Yes — unresolved findings blocking briefs is a real control | **simplify** | The register itself is sound, but three aspects are overbuilt: (a) the per-file format (YAML frontmatter + prose body) duplicates what a single FINDINGS.md table could do with less ceremony; (b) the sequence-number requirement (F-NN, no gaps) caused the F-05 deletion incident — the fix (sequence-gap detection in statusgen) is a mechanism added to police a mechanism; (c) the `affects:` field is free-text brief IDs with no validation that the brief actually exists or is still active. A single-table register with auto-assigned IDs and validated `affects:` references would be simpler and equally functional. |
| 8 | **Intake register** (per-entry files, `docs/streams/intake/`) | I-01 — brainstorming needed structure; four exits (brief-shaped, finding, parked, rejected) | Yes — the front door is needed | **keep** | The intake format is lighter than findings (no prose body requirement), and the 3-day triage alarm (issue-loop/07) makes it self-policing. The four-exit routing is well-designed. Minor bloat: the `disposition:` field enum and statusgen's intake-alarm reporting could be consolidated into a simpler triage-state model, but this is working well. |
| 9 | **RETRO.md** (cadence retrospective) | methodology/08 — weekly incremental review; one-process-change budget; DORA metrics | Yes, but the "one change" budget is a paper tiger | **simplify** | The retro serves a real purpose (banking process-change candidates, DORA reporting), but the "one process change max per retro" guard was routed around from day one via the "correction-class doesn't consume the budget" exemption — ≥6 rules landed 2026-07-08 under it. R-01's decision was "NONE" + track the rate. The guard is now explicitly advisory: the retro records the cycle's rule-change count and watches the trend decrease. This is honest, but it means the "one change" budget is a convention, not a constraint. Either admit that and drop the "max" language, or close the correction-class loophole. Also: the DORA section is substantial (~25 lines of the ~150-line RETRO.md) but the metrics are largely `unknown` or `needs: verify-desk|manual` — only deployment frequency and lead time are automated. |
| 10 | **statusgen tool** (STATUS.md generator + lint engine) | methodology/07 — board visibility; single-writer rule (main's CI) after methodogy/15 | Yes — the board is load-bearing | **keep** | The most complex mechanism in the methodology (~2000+ lines of Go across ~20 files). Its lint rules have grown incrementally: brief format checking, link checking, register-reference checking, DAR sync, attribution, Verify sections, unfailable rows, freshness checks, standing alarms, verification debt, intake triage alarms, CLAUDE.md budget checks, register integrity, placeholder checking, point-quality notices. Each rule solved a real problem, but the accretion is visible in the `run()` function which now chains ~12 check functions. The tool is well-structured (each check is a separate function) but a newcomer reading `main.go`'s `run()` would see a wall of `problems = append(problems, ...)`. A periodic check-function audit (are all 12 still firing on live problems?) would be a healthy companion to this review. |
| 11 | **Brief-v1 format** (frontmatter schema) | methodology/06 — standardize what a brief looks like; frontmatter fields drive statusgen parsing | Yes — the schema is load-bearing | **keep** | The frontmatter has grown from ~8 fields to ~14 (`brief`, `title`, `wave`, `depends`, `unblocks`, `effort`, `gate`, `risk`, `issues`, `schema`, `authored`, `sources`, `exec-tier`, `value`). Each field addition was justified (exec-tier for methodology/29, value for methodology-metrics/14), but the cumulative frontmatter ceremony is notable. A `brief-v2` that consolidates risk into a single structured object and drops `unblocks` (redundant with reverse-depends walks) would be lighter. Not urgent. |
| 12 | **CLAUDE.md resident rules** (~205 lines) | The placement rule itself: every session and subagent loads CLAUDE.md, so load-bearing rules live here | Yes — CLAUDE.md is the cheapest residence | **simplify** | CLAUDE.md has grown from a dev-commands cheatsheet to a 205-line operational guide. Several rules could move to cheaper residences per the placement rule: the "Close-PR flow" (11 lines, issue-loop/10) is needed only by bug-fixing workers, not every session; the "Red check is your work item" section (7 lines, assay-toolkit#57) is a worker/PR-author rule; the "Next pick" section sync/origin/main advice is skimmable on every session boot. The auth rules (5 human-actionable + reconciler-enforced) are genuinely load-bearing and correctly resident. The "Insight-routing" rule is restated in at least 4 skills but absent from CLAUDE.md — it should be here once, not there four times. |
| 13 | **Main-commit backstop** (F-13, `.githooks` hook) | F-05 deletion incident (2026-07-09) — a session deleted a finding to silence a checker; the hook prevents main commits without explicit authorization | Yes — the guard has never fired spuriously | **keep** | Simple, mechanical, correct. The list of sanctioned main-writers (desk, verify-desk, human:<name>, Flux exception) is clear. The hook is the right granularity: it gates the act, not the intent. |
| 14 | **Worktree isolation rule** | 2026-07-08 incident — `git restore`/`clean` in the shared checkout wiped another session's uncommitted work | Yes — parallel sessions still share a checkout | **keep** | The rule is sound and has prevented recurrence. The F-35 incident (a fanout session `cd`'d to the shared checkout for a boot step, then propagated shared paths into worker prompts) showed the rule needed hardening (never `cd` to shared, even for reads), which was added. The isolation is now well-specified. |
| 15 | **Reviewer App identity** (reviewer-app[bot], mint-reviewer-token.go) | methodology/brief-17 — tamper-evident review verdict; the "DESK-READY" text marker was forgeable (PR #125 incident) | Partially — attribution works, enforcement doesn't | **simplify** | The original claim was "tamper-evident verdict" — this was walked back to "attribution with an auditable trail" (assay-toolkit#37/#38) after it was demonstrated that any session with the App PEM can mint the token and the App can self-approve. The current state is: the App review is the desk's flip signal (advisory), enforcement is still human:<name>'s merge, and the "real enforcement" is pending the desk-apps stream. The mechanism is honest about its limits, but it's a heavyweight solution (PEM key, token minting Go tool, per-org installation, deskboard.go reading App state, the cross-install 404-vs-403 gotcha) for what is currently advisory attribution. A simpler mechanism: a `gh pr review` from a distinct bot account (no App, no PEM, no installation) would achieve the same attribution with half the machinery. The App investment may pay off when branch protection gates on it, but that day hasn't arrived. |
| 16 | **Deskboard.go** (PR review board tool) | pr-review-desk boot — one instrument instead of hand-polling `gh pr list` + `gh pr view` per PR | Yes — the board is the desk's worklist | **keep** | Well-scoped: stdlib-only, `go run`, read-only. The MERGE-CURR classifier (own-files intersection changed-since-review, minus shared registers) is genuine value. The liveness heartbeat (`swept <ISO8601>`) and idle gate (`actionable: N NEEDS-REVIEW, N RE-REVIEW`) are correct. Gap: doesn't yet read mergeStateStatus (oit#603), so FLIP actions need a manual mergeability check. |
| 17 | **DORA metrics** (statusgen --dora, emitted per retro) | methodology/18 — five metrics as a system, diagnostic not target | Yes, but half the metrics are `unknown` | **simplify** | The DORA framework is genuinely useful (the 2026-07-10 retro's 137 commits/day + 21.7h lead time told a real story), but the reporting apparatus overstates the instrumentation: only deployment frequency, commit→merge time, and the bug-issue slice of change-failure are automated. Failed-deploy recovery time, full change-failure rate, and rework rate are all `needs: verify-desk|manual` — and have been since the first retro. The Goodhart anti-gaming guardrails are verbose (~10 lines) but correct. The real simplification: drop the three `unknown` metrics from the emitted report until they're automated, and run only what's instrumented. A half-empty dashboard is noise. |
| 18 | **Close-PR flow for bugs** (bugs/<N>.md → PR with Closes #<N> → merge closes → bugs-gc prunes) | issue-loop/10 — "agents + coordinator NEVER close issues directly" | Marginal — the flow adds ceremony to what `gh issue close` does in one step | **simplify** | This is a 4-step chain for the outcome "close a bug issue when the fix lands": (1) create bugs/<N>.md with resolution claim + evidence, (2) include Closes #<N> in the PR body, (3) reviewer judges the claim, (4) bugs-gc prunes the transient .md file. The durable record is "issue + merged PR" — which a `gh issue close <N> --comment "fixed in #<M>"` also produces, without the intermediate file or the GC tool. The original motivation (reviewer judges the close claim) is served equally by the reviewer reviewing the fix PR itself. The bugs/<N>.md file is described as "transient" in the very rule that mandates it — a mechanism that exists to be garbage-collected. |
| 19 | **Claims lock mechanism** (~/.claude/desk-tools/claims/) | batch-fanout dispatch race (human:<name>, 2026-07-10) — between board-read and first push, two dispatchers see the same brief free | Yes, but three layers is one too many | **simplify** | The dispatch-race prevention has three layers: (1) noclobber claim file creation, (2) branch-exists check (authoritative), (3) PR-scan clone detection. Layer 2 (branch check) is sufficient for all cases except the race window between board-read and first push — which layer 1 closes. Layer 3 (scanning for existing PRs) is belt-and-suspenders on top of the branch check since a pushed branch implies a PR (workers open PRs immediately). Two layers (claim file + branch check) would serve the same outcome. |
| 20 | **Shared guardrails (insight-routing, escalation labels, git push policy, no-attribution)** | Added incrementally across multiple briefs as each skill grew | Yes — but restated 4-5 times across skills | **simplify** | The same four guardrails appear in the-desk (~30 lines), batch-fanout (~20 lines), pr-review-desk (~15 lines), verify-desk (~20 lines), and issue-loop (inherited by reference). That's ~85 lines of duplicated rules that could live once in CLAUDE.md with a single pointer from each skill. The duplication creates drift risk (verify-desk's "land-as-you-go" push policy conflicts with the-desk's "push is gated unless standing-authorized" framing — they're describing the same policy from different angles). |
| 21 | **Unfailable-row detection** (statusgen NOTICE) | #509 — a Verify row whose command is structurally incapable of failing manufactures evidence | Yes — catch `grep -c` as a "test" | **keep** | Currently a NOTICE (not a hard problem) because existing briefs on main already carry such rows. The plan to flip to hard-problem once active streams are clean is correct. This is a good example of a mechanism that accreted incrementally and earns its keep. |
| 22 | **Stream per-tiering policy** (`tiering:` frontmatter override) | methodology/05 — no single universal tiering rule works for every stream | Marginal — has any stream actually used it? | **keep (watch)** | The mechanism is well-designed (free-text field, rendered in STATUS.md Notes, never a Next-up score input). But after searching the active stream READMEs, I found zero streams with a `tiering:` override. The mechanism exists for a future that hasn't arrived. It costs little (one field parse in statusgen) but if no stream uses it by the next review, it's accretion. |
| 23 | **`exec-tier:` field** (methodology/29) | methodology/29 — derived from three complexity questions; signals "strong implementer only" to dispatchers | Yes — prevents cheap-tier models on complex briefs | **keep** | Simple, derived (not chosen), correct. The "pickup-STOP" text added to worker prompts ("If you are a fast/cheap-tier model, STOP") is the enforcement rail. |
| 24 | **Evidence convention** (`docs/streams/<stream>/evidence/`) | PR #77 CI failure — committed brief-12 docs cited a gitignored `.superpowers/` artifact, green locally, red in clean checkout | Yes — environment-dependent evidence is a real hazard | **keep** | Simple, mechanical. The correction-class landing (didn't consume R-01 budget) was appropriate. No bloat. |
| 25 | **Freshness-check notices** (statusgen) | methodology/37 — briefs age out; the `freshness-checked:` frontmatter field + statusgen NOTICE | Yes — catches stale briefs | **keep** | The mechanism is lightweight: a frontmatter field, a statusgen NOTICE, no hard gate. Correct granularity. |

The count is 25 mechanisms, 25 verdicts: 15 **keep**, 9 **simplify**, 1 **keep (watch)**, 0 **retire**. No mechanism received a clean `retire` verdict on this first run — the system is young enough (13 days since the first retro) that nothing has fully outlived its purpose. But 9 `simplify` verdicts across 25 mechanisms (36%) is a strong signal: the methodology has accreted duplication, ceremony, and layered defenses that could be lighter. This is the expected profile for a greenfield-built system hitting its first zero-based review — lots of "yes, but less would do."

---

## Chains

Three Rube Goldberg chains were identified — three or more mechanisms serving one outcome where a single simpler mechanism would serve:

### Chain 1: Bug-close flow (4 mechanisms)

**Mechanisms**: (1) `bugs/<N>.md` resolution-claim file, (2) `Closes #<N>` in PR body, (3) reviewer judges the claim, (4) `tools/bugs-gc` prunes the transient file.

**Outcome**: close a bug issue when its fix lands.

**Why it's a chain**: A `gh issue close <N> --comment "fixed in #<M>"` achieves the same durable record (issue body + merged PR) without the intermediate file, the Closes keyword dependency, or the GC tool. The intermediate `bugs/<N>.md` file is described in the rule itself as "transient" — a mechanism whose explicit purpose is to be garbage-collected is a mechanism that shouldn't exist. The reviewer already judges the fix PR; asking them to also judge a separate close-claim file is ceremony.

**Recommendation**: Collapse to a single step: the fix PR's reviewer (or the worker) closes the issue with a comment linking the merged PR. Drop `bugs/<N>.md` and `tools/bugs-gc`. If a close is disputed, re-open the issue — GitHub supports this. File as finding F-bug-close-four-step.

### Chain 2: Dispatch-race prevention (3 mechanisms)

**Mechanisms**: (1) noclobber claim file in `~/.claude/desk-tools/claims/`, (2) branch-exists check (git remote), (3) PR-scan clone detection (gh pr list).

**Outcome**: prevent two dispatchers from assigning the same brief to two workers.

**Why it's a chain**: Layer 2 (branch-exists check) is authoritative — a branch means a worker has pushed. Layer 1 (claim file) closes the race window between board-read and first push. Layer 3 (PR-scan) is belt-and-suspenders: a pushed branch implies a PR because workers open PRs immediately after push. Two layers (claim file + branch check) would serve the same outcome.

**Recommendation**: Drop the PR-scan clone detection from the dispatch eligibility check. The branch check already answers "is this brief claimed?" and the claim file already answers "is someone about to push?". A PR without a branch is impossible (workers push branch then open PR), and a branch without a PR is a worker that crashed mid-push (the claim file staleness timeout handles this). File as simplification note on batch-fanout.

### Chain 3: Shared guardrails duplication (5 restatements)

**Mechanisms**: The same four guardrails (insight-routing, escalation labels, git push policy, no-attribution) are restated in the-desk (RO), batch-fanout (GR), pr-review-desk (GP), verify-desk (Rules), and issue-loop (by inheritance reference). Each restatement is ~15-30 lines, totaling ~85 lines of duplicated rules.

**Outcome**: every desk/loop skill knows the cross-cutting operating rules.

**Why it's a chain**: The placement rule (CLAUDE.md, § "Placement rule") already specifies that rules load-bearing for every session live in CLAUDE.md. These four guardrails are exactly that class — every desk/loop session needs them. Restating them in each skill body creates drift risk (verify-desk's "the desk lands its own work" framing vs the-desk's "push is gated unless standing-authorized" — same policy, different words) and maintenance cost (a rule change needs N file edits).

**Recommendation**: Move the four shared guardrails to CLAUDE.md once, under a "Desk/loop guardrails" section. Each skill body replaces its guardrails block with a one-line pointer: "Cross-cutting guardrails (insight-routing, escalation labels, git push policy): CLAUDE.md § Desk/loop guardrails." This follows the placement rule's own advice (rules must stay resident; compress wording; pointer from skill body to CLAUDE.md is allowed for rules "every session needs"). File as finding F-guardrails-dup.

### Chains verdict

Three chains found. None are fatal — each mechanism in each chain is individually defensible — but the chains are exactly the Rube Goldberg pattern the review exists to catch: defensible pieces, indefensible whole. The single-simpler-mechanism test passes for all three: a `gh issue close` comment, a two-layer dispatch guard, and a once-in-CLAUDE.md guardrails section would each replace their chain.

---

## All-keep argument

This run found 0 `retire` verdicts and 9 `simplify` verdicts. The absence of clean retirements is not suspicious at this system's age — the methodology is 13 days post-first-retro, and every mechanism was added to stop a specific, recent incident. The earliest mechanisms (the-desk skill, statusgen, brief-v1) are ~19 days old. At this age, "nothing is ready to retire" is the expected answer. What IS notable is the 36% simplify rate — if the next review (2026-08-17) still shows 0 retirements with sustained simplify volume, that signals the simplification recommendations aren't being acted on, not that the system is static.

---

## Routed outputs

### Findings entries

1. **F-bug-close-four-step** — Bug-close flow (bugs/<N>.md + bugs-gc) is a 4-step chain for outcome "close a bug." Single `gh issue close` comment achieves the same durable record. Affects: issue-loop/10.
2. **F-guardrails-dup** — Shared guardrails (insight-routing, escalation labels, git push policy, no-attribution) restated 4-5 times across skills; should live once in CLAUDE.md. Affects: methodology, issue-loop.

### Intake entries

1. **I-claude-md-diet** — CLAUDE.md diet: move Close-PR flow, Red-check rule, and Next-pick advice to cheaper residences (skill bodies or docs/). The shared guardrails consolidation (F-guardrails-dup) is the first move.
2. **I-simplify-retro-dora** — Simplify RETRO.md DORA section: drop the three `unknown` metrics from the emitted report until automated; report only what's instrumented.
3. **I-statusgen-audit** — Statusgen check-function audit: are all ~12 lint rules in `run()` still firing on live problems, or have some gone cold?

### Brief-shaped simplification work

- **Author a brief for F-bug-close-four-step** (bug-close flow simplification) — effort S, in the issue-loop stream.
- **Author a brief for F-guardrails-dup** (guardrails consolidation) — effort S, in the methodology stream (affects multiple skills).
- The deskboard.go mergeStateStatus gate (oit#603) is already tracked; this review adds no new brief for it.

---

## Recursion guard

This document itself is a mechanism. The next run (due 2026-08-17) must:
1. Check whether F-bug-close-four-step and F-guardrails-dup were resolved (did the bug-close flow simplify? did guardrails consolidate?).
2. Check whether the `simplify` verdicts produced action or sat idle — a 36% simplify rate that produces zero follow-through is a process smell.
3. Re-evaluate the issue-loop skill (verdict: keep/watch) — by then it will have a month of operating history and the "fourth standing window" decision can be assessed on data, not design intent.
4. Apply the same four questions to this document: does the review itself justify its cost? If 0 retirements and 0 simplifications acted upon, the review is ceremony.
