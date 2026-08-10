---
brief: issue-loop/13
title: 'Reconceive the issue-desk as the generic intake-desk — rename issue-loop → intake-desk + broaden the skill to the five-exit front door'
why: >-
  The inbound desk already has its own window (brief-11), but it is named and framed around GitHub
  issues while its real job is generic: take in ANY inbound — issues, intake-register entries, incoming
  requests — and convert each into a spec/brief, a bug, a finding, a decision, or a reasoned rejection.
  Naming it "issue-loop" hides that and mis-sorts it in the loop taxonomy. Renaming to intake-desk and
  reframing the skill around the five exits makes the front door legible and lets the intake lane
  (I-INCOMING → tracked work) be a first-class part of the desk, not a bolt-on.
wave: 5
depends: ["issue-loop/08", "issue-loop/11"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-17 by Opus desk session (human:<name> directive)
sources:
  - "human:<name> 2026-07-17: 'we have the issue-desk which probably should be looked at as a generic intake desk … taking in I-INCOMING things and converting them to specs/briefs/bugs etc … we need a spec/briefs for this'"
  - "[F-issue-desk-intake](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-17-issue-desk-becomes-generic-intake-desk.md) — the reconception this implements; resolves the one-vs-two-desk fork to one generic desk"
  - "intake-desk-scoping.md (this stream) — the design + brief map"
  - "assay-toolkit docs/skill-naming.md — intake-desk = the front-of-pipeline automation loop; the naming convention this rename follows"
  - "brief-11 (the desk skill being broadened) + briefs 07/08/09 (the intake lane)"
  - "freshness-checked 2026-07-17 @ origin/main: .claude/skills/issue-loop/SKILL.md exists (brief-11); briefs 08/09 still todo"

---

# Brief 13 — Generic intake-desk reconception + rename

## Context
files: `.claude/skills/issue-loop/` → **rename to** `.claude/skills/intake-desk/` (dir + SKILL.md
`name: intake-desk`); `docs/streams/issue-loop/` → the stream reframed as the intake-desk (README
mission + this brief); by-name refs to `issue-loop`/`issue-desk` in `CLAUDE.md`, `pr-review-desk`/
`the-desk`/`batch-fanout` skills, and the loops reference. This is a SHARED-VALUE rename (a skill/stream
name many things reference) — enumerate consumers (grep `issue-loop`) and update each.

facts:
- The desk skill (brief-11) already drops issue placeholders, triages intake, files decision issues,
  and closes out — but is named/framed around issues. This brief makes the framing **generic**.
- **The five exits** (from the scoping doc / F-issue-desk-intake) are the desk's conversion vocabulary; the skill must
  state them as the desk's job: spec/brief · bug/issue · finding · decision-needed · rejected/watching.
- **Rename, not re-mechanism.** The mechanics (scanner, triage verbs, decision queue, self-dispatch,
  claims lock) are unchanged; only the name, the framing, and the intake-lane wiring change.
- **The rename is a coordinated change** — sequence it AFTER in-flight PRs that touch the issue-loop
  skill/stream path land, to avoid churn (check open PRs first; NEEDS_CONTEXT if any are open on that
  path).

## Task
1. **Rename** `.claude/skills/issue-loop/` → `.claude/skills/intake-desk/`; set SKILL.md
   `name: intake-desk`; keep the invocation triggers (add "intake"/"triage the front door"/"work the
   incoming" alongside the existing issue triggers).
2. **Broaden the skill's mission** to the generic front door: ingest issues + intake `disposition:new`
   + any incoming; state the **five conversion exits** explicitly; keep the routing test and the
   two-lanes framing.
3. **Reframe the stream**: `docs/streams/issue-loop/README.md` mission → the intake-desk (generic front
   door); note the rename and cite F-issue-desk-intake + the scoping doc. (Renaming the stream DIRECTORY is optional
   here — if deferred, state why; the mission reframe is required.)
4. **Update by-name consumers**: grep `issue-loop`/`issue-desk` across `.claude/skills/**` and
   `CLAUDE.md`; re-point each to `intake-desk` (the loops reference, pr-review-desk step 4 host, etc.).
5. **Resolve F-issue-desk-intake** in the same PR (record the rename landed).

## Ground rules
- NEVER push to main / trigger workflows / merge. Branch + draft PR; stop at `implemented`.
- In-repo skill (`.claude/skills/`), no out-of-repo declaration. Do NOT edit any `~/.claude` copy.
- NEEDS_CONTEXT over guessing if an open PR is mid-edit on the issue-loop skill/stream path (rename
  would collide) — coordinate/sequence, don't force it.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f .claude/skills/intake-desk/SKILL.md && ! test -d .claude/skills/issue-loop` | dir renamed |
| 2 | `grep -m1 '^name: intake-desk' .claude/skills/intake-desk/SKILL.md` | 1 — skill name set |
| 3 | `grep -ciE -e 'spec' -e 'brief' -e 'bug' -e 'finding' -e 'decision' -e 'reject' .claude/skills/intake-desk/SKILL.md` | ≥1 for each — the five exits are stated |
| 4 | `test -d ../oit/.claude/skills && test -f ../oit/CLAUDE.md && grep -rniE 'issue-loop' ../oit/.claude/skills/ ../oit/CLAUDE.md > /tmp/il13r4.txt && test -s /tmp/il13r4.txt && { grep -viE -e 'docs/streams/issue-loop' -e 'intake-desk-scoping' -e 'F-4[12]' /tmp/il13r4.txt \|\| true; }` | exit 0, and the printed residue empty (or each remaining hit justified in Evidence) — no stale skill refs to the old name. Re-anchored 2026-08-03. The corpus is guarded and named explicitly: the skills this brief renames live in the **sibling** repo (`../oit/.claude/skills/`), and this repo has no `CLAUDE.md` at all, so the bare `.claude/skills/ CLAUDE.md` form scanned assay-toolkit's own one-skill directory and a nonexistent file — "empty" was satisfied by an almost-empty corpus rather than by the rename being complete. `test -d`/`test -f` (the guard 20 other swept rows carry) plus `test -s` on the raw capture make a missing or unreadable corpus exit **1 before any grep runs**, which is otherwise indistinguishable from the clean state; the residue is printed for judgement, so `\|\| true` keeps the exit status meaning "the corpus was really scanned" rather than "no hits" |
| 5 | `statusgen --root . --lint` | exit 0 (F-issue-desk-intake resolved; stream reframe lints) |

## Evidence
<!-- filled by a non-implementer at verify time -->

### Non-implementer verifier run — VERIFY: PASS — glm-5.2-verifier, merged main `700e1c9e`, 2026-07-20

| # | Command | Expect | Observed | Result |
|---|---------|--------|----------|--------|
| 1 | `test -f .claude/skills/intake-desk/SKILL.md && ! test -d .claude/skills/issue-loop` | dir renamed | `.claude/skills/intake-desk/` exists, `issue-loop/` gone | PASS |
| 2 | `grep -m1 '^name: intake-desk' .../intake-desk/SKILL.md` | name set | `name: intake-desk` | PASS |
| 3 | `grep -ciE -e spec -e brief -e bug -e finding -e decision -e reject SKILL.md` | ≥1 each | spec 8, brief 28, bug 8, finding 3, decision 25, reject 4 — all ≥1 | PASS |
| 4 | `grep -rniE 'issue-loop' .claude/skills/ CLAUDE.md \| grep -viE 'docs/streams/issue-loop\|intake-desk-scoping\|F-4[12]'` | empty OR each hit justified | non-empty but all hits are `issue-loop/NN` brief-ID refs, claims-key/placeholder strings, or `chore/issue-loop-scan` branch names; `grep skills/issue-loop` → none | PASS (justified) |
| 5 | `go run ./tools/statusgen --root . --lint` | exit 0 | exit 0 (NOTICEs only, 0 ERROR) | PASS |

Supporting: finding `../oit/docs/streams/findings/2026-07-17-issue-desk-becomes-generic-intake-desk.md` `resolved: yes` (by issue-loop/13, 2026-07-19); README reframed to the generic five-exit intake-desk with the stream-dir-rename deferral stated.

**VERIFY: PASS** — the rename landed cleanly (dir + `name:`, five exits stated, README reframed, finding resolved, lint exit 0); row-4's non-empty output is fully accounted for by the brief's own scope (stream dir kept for history, mechanics unchanged), no residual old-skill-path refs.

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
