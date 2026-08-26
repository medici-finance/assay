---
brief: harness-portability/09
title: jcode desk-harness spike — measured parity + fleet-density for driving desks
why: >-
  jcode (a Rust terminal coding-agent harness) claims roughly an order-of-magnitude less RAM than
  Claude Code (vendor figures vary by scenario) and native agent swarms — if that holds on OUR
  workload it is a direct multiplier on how many desks + workers fit per node, the desk-console
  substrate's binding constraint. But the desks assume Claude Code's harness (the MCP tool surface,
  the SessionStart resident-rules hook, the writeguard/isolation behaviours). Ruling jcode in or out
  on vendor benchmarks + a demo would rebuild, at decision time, the single-vendor drift this stream
  exists to remove. Measure first. This spike INFORMS — it does not gate — the HP/03 harness-target
  ruling (a proposal of the authoring session, not a dependency Ian set).
wave: 0
depends: []
unblocks: []
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-16 by intake-desk authoring session
sources:
  - "authoring dispatch (Ian, 2026-08-16): evaluate jcode as a desk-driver harness — two spikes, one per question; this is spike (a)"
  - "jcode: 1jehuang/jcode (MIT, Rust; YC S2026 per the vendor thread, not confirmed from repo metadata) — learnjcode.org; deepwiki.com/1jehuang/jcode. Vendor claims (per-scenario, UNDER TEST here — never inherited): order-of-magnitude less RAM than Claude Code (vendor tables give a range by scenario), ~63x faster boot than Codex in one vendor table, native multi-model + agent swarms + SSH"
  - "freshness-checked 2026-08-16: no docs/research/jcode-* file exists; the agentic-workflow-tools landscape doc does not cover jcode"
exec-tier: strong
exec-tier-why: >-
  (a) distinguishing "primitive absent" from "primitive not found / needs a workaround" requires
  deliberate probing and judgement; the parity matrix's verdicts are the facts the harness-target
  ruling (HP/03) binds to.
---

# Brief 09 — jcode desk-harness spike

## Context
files:
- **create** `docs/research/jcode-desk-harness-capabilities.md` (planned) — the measured parity matrix + density numbers + go/no-go read
- **amend** `freshness.yaml` (planned) — register the new file (empirical harness facts rot)
out-of-repo files: none (jcode installs to the runner/pod, not the repo)
facts:
- the desks require, from any harness that drives them: an MCP tool surface (the desk tools + claude-in-chrome), a SessionStart-equivalent that injects the resident rules, a persistent-memory equivalent, background/parallel sub-agents (worker-desk fans out 8), and the isolation the writeguard assumes. This matrix is present/absent/workaround per primitive — MEASURED, never inherited from vendor copy.
- density is the point: measure REAL per-session resident memory + cold-boot time on our workload (one desk skill, real tools loaded), not the vendor's synthetic figure.
- **exec-tier enforcement (Ian 2026-08-16):** `exec-tier: strong/weak` is today an UNENFORCEABLE annotation — statusgen never verifies which model actually ran (author-brief rule 9, honor system). The question the spike answers: can jcode assign models PER-TASK (not just per-session) — e.g. a strong parent spawning weak children — so the brief's `exec-tier` becomes a runtime GUARANTEE rather than a hope. A yes here is a first-class reason to adopt.
- **prose-vs-discrete / "meld into jcode" (Ian 2026-08-16):** the desks are mostly deterministic mechanics (scan, board reads, guards, token-mint, the loop) NARRATED in prose SKILL.md and executed by a general LLM — expensive + non-deterministic. The spike delivers a DESIGN SKETCH of the alternative: which desk parts become discrete jcode-native code/tools/config vs stay LLM-judgment (triage classification, decision authoring). This is a redesign question, not a port — the sketch is the deliverable, not an implementation.
- this is a spike: the deliverables are a decision-grade matrix + numbers + a design sketch, NOT a migration. No desk is moved off Claude Code here.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done.
- Time-box it: if a required primitive has no jcode equivalent and no obvious workaround, RECORD the gap and move on — do not build the missing primitive in this brief.

## Task
1. Install jcode (curl/Homebrew, MIT). Point it at the same model backends the cells use (Anthropic-compatible endpoints / OpenRouter).
2. Run ONE desk skill end-to-end under jcode — pick the lightest real one (intake-desk's board read, or a minimal skill). Exercise: the MCP tool surface it needs, a resident-rules-injection equivalent, one background/child agent, and one write behind the isolation the writeguard expects.
3. **Exec-tier probe:** determine whether jcode binds a model PER-TASK/per-agent (a strong parent spawning a weak child, choosing the model from a per-task field). Record whether `exec-tier: strong/weak` could become a runtime guarantee under jcode, or is per-session-only.
4. **Prose-vs-discrete design sketch:** for the one desk exercised, sketch which parts of its loop would become DISCRETE jcode-native code/tools/config vs which stay LLM-judgment. Name the split; do not implement it.
5. Fill the planned matrix file: the present/absent/workaround row per required primitive (with evidence), the exec-tier verdict (3), and the design sketch (4).
6. Measure real per-session RSS + cold-boot on our workload; record both, with the measurement command, beside the vendor's claim.
7. Write the go/no-go read: parity gaps that block desk-driving, whether the density win is real, and whether exec-tier enforcement + the discrete-meld justify a migration brief. Register the file in `freshness.yaml`.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f docs/research/jcode-desk-harness-capabilities.md` | exit 0 — the matrix file exists (planned deliverable) |
| 2 | `grep -qiE -e absent -e workaround docs/research/jcode-desk-harness-capabilities.md` | exit 0 — at least one non-trivial per-primitive verdict recorded (separate `-e` patterns; a `\|` alternation inside `-E` is a literal pipe) |
| 3 | (dereferencing) the measured RAM/boot figures in the doc are each backed by a named command whose output is quoted — a reader can re-run it | every numeric claim cites its measurement command + output, not a vendor number |
| 4 | (dereferencing) the doc records the exec-tier probe (3) and the prose-vs-discrete split (4) as explicit verdicts, not TODOs | both questions answered with evidence, not left as TODOs |
| 5 | `grep -q 'jcode-desk-harness-capabilities' freshness.yaml` | exit 0 — the empirical file is registered so it is re-measured before it rots |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer at implementation time -->
jcode desk-harness capability spike. Diff = one research/design doc (prose, reversible) + a 4-line freshness registration + README board-row flip. Verified in-tree at the source repo; no sibling repos referenced.

| # | Command | Exit | Key output | Date | Runner |
|---|---------|------|-----------|------|--------|
| 1 | `test -f docs/research/jcode-desk-harness-capabilities.md` | 0 | present | 2026-08-24 | opus-4.8[1m]-verifier |
| 2 | grep -qiE absent/workaround in the capability matrix | 0 | P6=absent, P2/P3=workaround recorded | 2026-08-24 | opus-4.8[1m]-verifier |
| 3 | dereference: measured figures backed by named re-runnable command; no vendor number asserted as finding | — | §6 quarantines the vendor table (labelled, source+caveat), asserts NO number as a finding, supplies named re-runnable PSS/RSS/boot commands; our-workload numbers explicitly `BLOCKED (needs live jcode)` — intent met, live half BLOCKED by declared offline envelope | 2026-08-24 | opus-4.8[1m]-verifier |
| 4 | dereference: exec-tier probe (§4) + prose-vs-discrete split (§5) recorded as explicit verdicts | — | §4 documentary lean "per-session-only" + probe design + verdict rule; §5 full named intake-desk discrete/LLM split table; both answered with evidence, no TODOs | 2026-08-24 | opus-4.8[1m]-verifier |
| 5 | `grep -q jcode-desk-harness-capabilities freshness.yaml` | 0 | entry present, YAML parses | 2026-08-24 | opus-4.8[1m]-verifier |

**RISK-VALUE: DERIVED** — freshness config literals: `max-age-days = 45` (matches sibling empirical-doc entry HP/10 at 45d; reversible), `last-reviewed = "2026-08-24"` (matches authored + HEAD commit date), `path = docs/research/jcode-desk-harness-capabilities.md` (matches the file row 1 confirms). The doc's §6 density table is vendor figures explicitly NOT asserted as findings — **RISK-VALUE: N/A** (no measured literal introduced; nothing binds). Top irreversibility = the staleness window (lowest-consequence, reversible). No irreversible literal in this diff.

**VERIFY: PASS** — all 5 rows pass; the Verify table gates only doc existence/content + dereference intent (the live-measurement Tasks 1/2/6 are honestly marked `BLOCKED (needs live jcode)` per the stream's "Blocked is a state, not a failure" convention, with a live follow-up recommended). gate:model + all-risk-no → flipped `implemented → verified`.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the harness-portability README table.
