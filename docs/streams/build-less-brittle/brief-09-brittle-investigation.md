---
brief: assay:assay:build-less-brittle:09
title: "Brittle investigation: a strong-tier task template that reads the original intent and the issues found, and recommends reconcile, redesign or accept"
why: >-
  A hotspot list that nobody acts on changes nothing: Google shelved its bug predictor because
  developers found it correct but not actionable (Lewis et al., ICSE 2013). A brittle mark (08)
  therefore binds to one next act, an investigation at strong tier that reads what the module
  was for (its brief, decision record and first commits), what has happened to it since (the
  class instances, the fix commits, the findings), and says which of three things is true: the
  code drifted from the intent, the intent stopped matching the need, or the intent is right and
  the implementation wrong. Its output is a recommendation with a deletion bundle, not a patch.
wave: 3
depends: ["build-less-brittle/04", "build-less-brittle/08"]
unblocks: ["build-less-brittle/12", "build-less-brittle/13"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: "2026-09-24 by the build-less-brittle authoring session (read-only; author-brief format; SOTA amendment)"
sources:
  - "docs/streams/build-less-brittle/spec.md §3 row 11, §4.9, §11"
  - "docs/streams/build-less-brittle/spec.md §11 (Lewis et al. 2013 on actionability; Fowler on refactor-vs-rewrite and the strangler fig; Ousterhout on strategic investment; Foote & Yoder on reconstruction as last resort; SRE workbook on action items with an owner and a verifiable end state)"
  - "docs/brief-template.md (the template precedent this sits beside) and spec/registers-v1.md §7 (DR-<slug> decision records)"
  - "plugins/assay/skills/worker-desk/SKILL.md §Un-briefed issues (the strong-tier lane brief 04 routes design-owed classes through)"
  - "freshness-checked 2026-09-24 @ f7bde6bfa: no investigation template, no docs/investigations/, no procedure that reads a module's originating brief against its fix history"
exec-tier: strong
exec-tier-why: "(a) the template asks for judgement (which of three divergences holds) and must tell a strong-tier session precisely what evidence settles each; (b) it spans the brief spec, the registers, the class issues and git history."
domain: complicated
consumers:
  - "docs/brittle-investigation-template.md: follow-up build-less-brittle/09 (this brief)"
  - "plugins/assay/skills/worker-desk/SKILL.md §Un-briefed issues: follow-up build-less-brittle/09 (this brief; ≤ 3 lines beside build-less-brittle/04's design-owed line)"
  - "docs/contracts.md §Brittle marks (investigation column): follow-up build-less-brittle/08 (the column exists; this brief fills it)"
  - "installed deskdispatch binaries: out-of-scope (no kit text changes; the template is read from the tree at dispatch time)"
---

# Brief 09 — The brittle investigation

## Context

files:
- `docs/brittle-investigation-template.md` (planned): NEW. The task template, with its frontmatter, its six sections and the evidence each section must cite.
- `docs/investigations/` (planned): NEW directory. One file per investigation, `<yyyy-mm-dd>-<module-slug>.md`, produced by the dispatched session, never by this brief.
- `plugins/assay/skills/worker-desk/SKILL.md`: §"Un-briefed issues", ≤ 3 lines.
- `changelog/build-less-brittle-09.md` (planned)

facts:
- **Trigger.** A class issue that carries both `design-owed` (04) and `brittle` (08). It rides
  the un-briefed-issue lane at **strong** tier that 04 already routes design-owed classes
  through. The difference 09 adds: with `brittle` present, the deliverable is the
  investigation file **first**, and the design brief second and only if the recommendation is
  `redesign`. No new verb, flag, label-reading code or reply. The mark is made at the monthly
  pass, so the trigger is that pass's output, never a CI event.
- **Inputs the template requires, each with its read command.**
  1. *Original intent:* the module's originating brief and last redesign brief (`git log
     --follow --reverse --format='%H %s' -- <path> | head -3`, then the PR's `Brief:` trailer);
     the `DR-<slug>` record if the S- row names one; the S- row itself (`docs/contracts.md`);
     the brief's `## Context` and `why:` quoted, never paraphrased.
  2. *Issues found:* the class issue's instance table (kind, incident-group, introduced-by);
     the window's fix commits from 08's report; findings entries whose `affects:` names a
     brief that touched the module; chain arrows landing in the module (the project's baseline).
  3. *History reading:* for each fix commit, one row: date, commit, issue, what it added
     (verb / flag / refusal / branch / layer), and whether it is inside the owner the S- row
     names. The 08 coupling partners are read here: a partner outside the owner is a seam.
- **The three divergences** (exactly one is chosen; "none" is a legal answer that clears the
  mark): `drifted` (the intent holds; the fixes moved the code off it), `intent-changed` (the
  need moved; the intent as written is no longer what the module must do), `intent-right,
  implementation-wrong` (the intent holds and the original implementation never met it).
- **The three recommendations**, each with its next act and its owner:
  - `reconcile`: bring the code back to the intent. Next act: one fix brief whose `retires:`
    lists every layer the fixes added that the intent does not need (deletion bundling, spec
    §3). Fowler's refactor-first default.
  - `redesign`: the intent must change. Next act: a DR amendment and a design brief (title
    given), preferring a strangler seam over a rewrite; a rewrite is proposed only when the
    investigation shows the seam cannot be cut (Fowler; Foote & Yoder's "Reconstruction" is
    the last resort).
  - `accept`: the drift is the better design. Next act: amend the DR and the S- row so the
    record matches the code, and clear the mark. Nothing is coded.
  Every recommendation carries `single-point-of-failure:` when the module is on a core
  surface (the project layer's definition), and a `verifiable end state` line: what the next
  monthly pass must show for the mark to clear (the SRE workbook's action-item rule).
- **Tier and size.** Strong tier, read-only on the code, one file out. It is a reading task,
  bounded: the template caps the history table at the window's fix commits plus the
  originating commits, and says "report NEEDS_CONTEXT" when the originating brief cannot be
  found rather than inventing an intent.
- **Output shape** (frontmatter): `module`, `s-row`, `class-issue`, `mark-date`, `divergence`
  (one of three or `none`), `recommendation` (one of three or `clear`), `next-act` (a brief
  id, a DR id, or `clear`), `end-state` (one line), `tier: strong`, `date`.
- Line count at f7bde6bfa (for scale; the net ≤ 0 row derives its own base): worker-desk 868 (04 edits the
  same file; each brief offsets its own lines).

design-fit:
  owner: docs/brittle-investigation-template.md (a template beside docs/brief-template.md)
  contract: none — a template; the mark table it fills is 08's
  retires: []
  weight: verbs 0, flags 0, refusals 0, rule-text lines ≤ 0 (worker-desk)
  why-add: n/a (no ratcheted growth). The alternative, letting a design-owed class go straight to a design brief, skips the reading that says whether a redesign is needed at all; two of the three outcomes here code nothing.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- This brief writes the template and the skill line. It runs no investigation.
- Public tree: mechanisms and public issue numbers only.

## Task

1. Write `docs/brittle-investigation-template.md` (planned): the frontmatter keys above; sections
   `## Intent`, `## What happened`, `## Divergence`, `## Options`, `## Recommendation`,
   `## Next act`; under each, the evidence it must cite and the read command. Include the
   three divergences and three recommendations verbatim, the `none`/`clear` outcome, the
   NEEDS_CONTEXT rule, and a worked example over a fictional `cmd/example` module. Placement
   is fixed, because rows 1 and 5 read it: the file opens with ONE frontmatter block, and that
   block is the worked example's, filled in (the key descriptions live in the body, not in a
   second frontmatter block). Each of the six headings appears exactly once; under it comes
   the instruction, then the worked example's text for that section. The example never
   repeats a heading.
2. `docs/investigations/README.md` (planned): three lines. What lands here, the filename rule, and that
   a file's `recommendation:` is what the mark table's `investigation` column links to.
3. worker-desk §"Un-briefed issues" (≤ 3 lines, offset): a class issue labelled `brittle`
   dispatches at strong tier with the template as the deliverable's shape; the design brief
   follows only on `redesign`; `reconcile` yields a fix brief whose `retires:` is the
   investigation's deletion bundle.
4. Changelog fragment.

## Verify (executable — no prose-only DoD items)

Rows run from the root of `medici-finance/assay`. The deliverable is prose. Rows 1–4 gate the
template's shape, row 5 proves the worked example parses as an investigation would, row 6
dereferences the command the template tells sessions to run, rows 7–8 are the wiring and net
≤ 0 rows.

| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -c -e '^## Intent$' -e '^## What happened$' -e '^## Divergence$' -e '^## Options$' -e '^## Recommendation$' -e '^## Next act$' docs/brittle-investigation-template.md` | `6` |
| 2 | `grep -c -e '^divergence:' -e '^recommendation:' -e '^next-act:' -e '^end-state:' -e '^module:' -e '^class-issue:' docs/brittle-investigation-template.md` | ≥ `6` |
| 3 | `grep -o -e 'drifted' -e 'intent-changed' -e 'implementation-wrong' -e 'reconcile' -e 'redesign' -e 'accept' docs/brittle-investigation-template.md \| sort -u \| wc -l \| tr -d ' '` | `6` (all three divergences and all three recommendations are named) |
| 4 | `grep -c 'NEEDS_CONTEXT' docs/brittle-investigation-template.md` | ≥ `1` (an unfindable intent is reported, never invented) |
| 5 | `printf '%s\n' 'divergence: drifted' 'divergence: intent-changed' 'divergence: intent-right, implementation-wrong' 'divergence: none' 'recommendation: reconcile' 'recommendation: redesign' 'recommendation: accept' 'recommendation: clear' > /tmp/bl09-vocab.txt && awk '/^---$/{n++; next} n==1' docs/brittle-investigation-template.md \| grep -xF -f /tmp/bl09-vocab.txt \| cut -d: -f1 \| sort -u \| grep -c .` | `2` (the worked example's frontmatter carries both keys, each with a value from its closed vocabulary; a value outside it matches no line) |
| 6 | `cmd=$(grep -oE 'git log --follow --reverse[^<]*<path>' docs/brittle-investigation-template.md \| head -1); test -n "$cmd" && eval "${cmd/<path>/tools/desk/internal/forgeban/allowlist.go}" \| head -1 \| grep -cE '^[0-9a-f]{40} '` | `1` (the intent-read command the template gives runs and yields an originating commit) |
| 7 | `grep -c 'brittle-investigation-template' plugins/assay/skills/worker-desk/SKILL.md` | ≥ `1` |
| 8 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/09$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/worker-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/worker-desk/SKILL.md" \| wc -l)" && echo NET-OK` | `NET-OK` |
| 9 | `test -f docs/investigations/README.md && grep -c 'recommendation' docs/investigations/README.md` | ≥ `1` |
| 10 | `statusgen --consumers --root . --brief build-less-brittle/09; echo "exit=$?"` | `exit=0` at the PR head (no `consumers:` routing claim is disproved by the diff; the implementer replaces each self-routed entry with `fixed-here` in the same change). Exit 1 names the disproved claim |

## Evidence
<!-- appended at implementation time: one row per Verify item — (command, exit code,
     output line(s) or hash, date, runner). "verified" requires a NON-implementer. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|

## Review
Gate: model (from frontmatter). The reviewer runs the template's `## Intent` reads against one
real hotspot from 08's sample (`tools/desk/internal/deskkit/forge_gitlab.go` at f7bde6bfa) and checks that
the commands find an originating brief. A template whose reads work only on the worked example
is a finding. The reviewer also checks that `accept` and `none` are real outcomes and not
buried: an investigation that can only recommend work is a patch generator with extra steps.
