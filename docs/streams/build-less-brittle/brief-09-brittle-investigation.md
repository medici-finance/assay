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
version: 4
authored: "2026-09-24 by the build-less-brittle authoring session (read-only; author-brief format; SOTA amendment)"
sources:
  - "docs/streams/build-less-brittle/spec.md — 2026-09-30 pending scope amendment"
  - "freshness-checked 2026-09-30 @ 8485778515c041fc87966902a14eb9d195492be3: amend unfinished scope; no implementation claim"
  - "docs/streams/build-less-brittle/spec.md §3 row 11, §4.9, §11"
  - "docs/streams/build-less-brittle/spec.md §11 (Lewis et al. 2013 on actionability; Fowler on refactor-vs-rewrite and the strangler fig; Ousterhout on strategic investment; Foote & Yoder on reconstruction as last resort; SRE workbook on action items with an owner and a verifiable end state)"
  - "docs/brief-template.md (the template precedent this sits beside) and spec/registers-v1.md §7 (DR-<slug> decision records)"
  - "plugins/assay/skills/worker-desk/SKILL.md §Un-briefed issues (the strong-tier lane brief 04 routes design-owed classes through)"
  - "freshness-checked 2026-09-24 @ f7bde6bfa: no investigation template, no docs/investigations/, no procedure that reads a module's originating brief against its fix history"
exec-tier: strong
exec-tier-why: "(a) the template asks for judgement (which of three divergences holds) and must tell a strong-tier session precisely what evidence settles each; (b) it spans the brief spec, the registers, the class issues and git history."
domain: complicated
consumers:
  - "docs/brittle-investigation-template.md: fixed-here"
  - "plugins/assay/skills/worker-desk/SKILL.md §Un-briefed issues: fixed-here (3 lines beside build-less-brittle/04's design-owed line, offset in the same section)"
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

## Record compatibility amendment — 2026-09-28

Add spec §4.9's replayable-reading fields to the existing template and worked example,
without adding a seventh section. The example has an unavailable source and a later,
differently scoped success: it must retain the gap and must not infer recovery. Put the
competing explanation and discriminating check under Divergence, and the expected
observable under Next act. Preserve bounded reading, existing exits and net-zero skill lines.

## Work-input amendment — 2026-09-30

The template must state its code/source revisions, the relevant source/policy/dependency
references, reconciled assumptions, unresolved questions and next act. Each conclusion
names the evidence it depends on. Reuse the investigation at those inputs; if a later brief
changes a relevant assumption, preserve the old record and produce a superseding revision
or explicit affected-scope revalidation. An empty dependency list means unknown coverage.

Add a worked example where a policy changes outside the module's touched files between
investigation and oracle assembly. Show which conclusion needs revalidation and why file
non-overlap is insufficient. This is a template/review obligation, not automated dependency
inference. Keep the six sections, net line limits and stream independence. The reviewer
must reject a template that presents the old conclusion as current in this example.
Use `source-revisions:`, `dependency-references:` and `unresolved-questions:` labels in the
Intent body, and put `### Intervening-change example` under Divergence. These are body
fields/subheadings, not another frontmatter block or a seventh top-level section.

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
| 6 | `cmd=$(grep -oE 'git log --follow --reverse[^<]*<path>' docs/brittle-investigation-template.md \| sed -n 1p); test -n "$cmd" && eval "${cmd/<path>/tools/desk/internal/forgeban/allowlist.go}" \| sed -n 1p \| grep -cE '^[0-9a-f]{40} '` | `1` (the intent-read command the template gives runs and yields an originating commit). Row re-authored 2026-10-06 (#1915): `sed -n 1p` replaces `head -1`, which closed the pipe after the first commit and failed the row with SIGPIPE under `pipefail`; `sed` reads its whole input. |
| 7 | `grep -c 'brittle-investigation-template' plugins/assay/skills/worker-desk/SKILL.md` | ≥ `1` |
| 8 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/09$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/worker-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/worker-desk/SKILL.md" \| wc -l)" && echo NET-OK` | `NET-OK` |
| 9 | `test -f docs/investigations/README.md && grep -c 'recommendation' docs/investigations/README.md` | ≥ `1` |
| 10 | `d=; impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/09$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; b=$(git rev-parse "$base") && t=$(git rev-parse "$tip") && test "$b" != "$t" && d=$(mktemp -d "$PWD/.bl09-consumers.XXXXXX") && git clone -q --shared --no-checkout . "$d" && git -C "$d" checkout -q --detach "$t" && statusgen --consumers --root "$d" --brief build-less-brittle/09 --base "$b"; s=$?; test -n "$d" && rm -rf "$d"; exit $s` | exit 0; output is `summary: 2 corroborated, 0 disproved, 2 unchecked, 0 brief(s) claiming nothing` (the `consumers:` claims are judged against the delivering change itself, in a throwaway clone: after the merge, the first-parent commit on main carrying the `Brief: build-less-brittle/09` trailer (the net-weight rows resolve the same commit) diffed against its parent; before the merge, the PR head diffed against its merge-base; the two `fixed-here` entries are corroborated; the UNCHECKED ones are the build-less-brittle/08 follow-up column and the out-of-scope installed-binaries entry, which the delivering diff does not touch and which stay the reviewer's call per brief-rule 9). A disproved claim fails the row and names the claim; an unresolvable delivering change fails the row rather than passing on an empty diff. Row re-authored 2026-10-06 (#1915): the unpinned form ran against merged main, where the brief is no longer in the diff, and reported COULD-NOT-CHECK; this form runs the same way before and after the merge (the #1908 pattern). Expect first re-written 2026-10-03 (#1862). |
| 11 | `f=docs/brittle-investigation-template.md; for key in source-revisions dependency-references unresolved-questions; do grep -qF "$key:" "$f" \|\| exit 1; done; awk '/^## / {p=($0 == "## Divergence")} p && /^### Intervening-change example$/ {found=1} END {exit !found}' "$f" && echo SOURCE-CHANGE-EXAMPLE` | `SOURCE-CHANGE-EXAMPLE` (presence/placement only; the review walks the policy-change case and checks revalidation) |

## Evidence
<!-- appended at implementation time: one row per Verify item — (command, exit code,
     output line(s) or hash, date, runner). "verified" requires a NON-implementer. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | Verify row 1 as written | pass exit=0 | `6` | 2026-10-06 | assay-worker-app[bot] @ f479471da001 (on-behalf-of human:ian) (implementer, self-run) |
| 2 | Verify row 2 as written | pass exit=0 | `6` (≥ 6) | 2026-10-06 | assay-worker-app[bot] @ f479471da001 (on-behalf-of human:ian) (implementer, self-run) |
| 3 | Verify row 3 as written | pass exit=0 | `6` | 2026-10-06 | assay-worker-app[bot] @ f479471da001 (on-behalf-of human:ian) (implementer, self-run) |
| 4 | Verify row 4 as written | pass exit=0 | `3` (≥ 1) | 2026-10-06 | assay-worker-app[bot] @ f479471da001 (on-behalf-of human:ian) (implementer, self-run) |
| 5 | Verify row 5 as written | pass exit=0 | `2` | 2026-10-06 | assay-worker-app[bot] @ f479471da001 (on-behalf-of human:ian) (implementer, self-run) |
| 6 | Verify row 6 as written | pass exit=0 | `1` (the extracted `git log --follow --reverse` read on `tools/desk/internal/forgeban/allowlist.go` yields a 40-hex originating commit) | 2026-10-06 | assay-worker-app[bot] @ f479471da001 (on-behalf-of human:ian) (implementer, self-run) |
| 7 | Verify row 7 as written | pass exit=0 | `1` (≥ 1) | 2026-10-06 | assay-worker-app[bot] @ f479471da001 (on-behalf-of human:ian) (implementer, self-run) |
| 8 | Verify row 8 as written (pre-merge: base = merge-base with `refs/remotes/origin/main`, tip = HEAD) | pass exit=0 | `NET-OK` (worker-desk SKILL.md 923 → 923 lines) | 2026-10-06 | assay-worker-app[bot] @ f479471da001 (on-behalf-of human:ian) (implementer, self-run) |
| 9 | Verify row 9 as written | pass exit=0 | `1` (≥ 1) | 2026-10-06 | assay-worker-app[bot] @ f479471da001 (on-behalf-of human:ian) (implementer, self-run) |
| 10 | Verify row 10 as written | pass | `summary: 2 corroborated, 0 disproved, 2 unchecked, 0 brief(s) claiming nothing`; `exit=0` | 2026-10-06 | assay-worker-app[bot] @ f479471da001 (on-behalf-of human:ian) (implementer, self-run) |
| 11 | Verify row 11 as written | pass exit=0 | `SOURCE-CHANGE-EXAMPLE` | 2026-10-06 | assay-worker-app[bot] @ f479471da001 (on-behalf-of human:ian) (implementer, self-run) |

The reviewer also walks the amendment's worked case through the existing deliverables and
records the source links, gap handling and outcome interpretation in the review. These are
semantic acceptance checks; presence of field names alone does not satisfy them.
### 2026-10-06 non-implementer verification on merged main (56140a33d)

Run at merged main 56140a33d0b4cde167f9e8269e624769fc3013ca, which matched the forge's main at run time (main had moved past the dispatched head 3012e2bed, so the worktree was re-pointed before any row ran). The delivering change is squash commit 4329640e7 (#2249), whose parent is e77868565. The PR head was 8f64ec2fb. statusgen was built from this tree's own statusgen module, with a throwaway HOME. Rows were run by hand in an interactive shell (no pipefail), then again through `statusgen verifyrun`, which runs every row under `bash -o pipefail`. Its witness table follows this fragment.

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | Verify row 1 as written | 0 | `6`. Each of the six headings counted separately gives 1 (Intent, What happened, Divergence, Options, Recommendation, Next act). PASS | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | Verify row 2 as written | 0 | `6` (≥ 6). PASS | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | Verify row 3 as written | 0 | `6`. PASS | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | Verify row 4 as written | 0 | `3` (≥ 1). PASS | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | Verify row 5 as written | 0 | `2`. The file holds exactly one frontmatter block (two `---` lines). PASS | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | Verify row 6 as written | 0 in a shell without pipefail; **141 under the witness's pipefail** (3/3 reproductions) | `1` in both shells. The extracted command is `git log --follow --reverse --format='%H %s' -- <path>`. It lists 27 commits for the allowlist file, and `head -1` closes the pipe after the first, so `git log` dies of SIGPIPE. Under pipefail that makes the row's exit 141. The checked property holds (the template's intent-read command yields an originating 40-hex commit), but the row as written cannot record a clean exit in the repo's own execution witness. **FAIL (as executed by the witness)** | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | Verify row 7 as written | 0 | `1` (≥ 1). PASS | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | Verify row 8 as written (post-merge) | 0 | `NET-OK`. Post-merge the `Brief: build-less-brittle/09` grep resolves impl to squash commit 4329640e7, so base is its parent e77868565 and tip is 4329640e7, not the pre-merge merge-base and HEAD. worker-desk SKILL.md has 921 lines at base and 921 at tip (921 at HEAD too). The implementer's pre-merge 923 → 923 was measured against an older base. Three lines were added in §Un-briefed issues and offset by rewrapping rules above them in the same section. PASS | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 9 | Verify row 9 as written | 0 | `1` (≥ 1). PASS | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 10 | Verify row 10 as written: on merged main, then at the PR head (the scope the Expect names) | on main: 2; at PR head 8f64ec2fb: 0 | On merged main: `COULD-NOT-CHECK: assay:assay:build-less-brittle:09 is not in the diff against 56140a33d…`, then `exit=2`. That is a could-not-check, not a pass. At the PR head (merge-base fc5afe37f): `summary: 2 corroborated, 0 disproved, 2 unchecked`, then `exit=0`. The squash commit with its parent as base gives the same summary and `exit=0`. The two unchecked entries are the contracts.md §Brittle marks follow-up and the out-of-scope deskdispatch binaries line. Neither is disproved. PASS at the PR head; the witness records a fail because it runs on merged main | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 11 | Verify row 11 as written, run in a subshell so that its `exit 1` arm cannot end the session | 0 | `SOURCE-CHANGE-EXAMPLE`. PASS | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |

Execution witness: `statusgen verifyrun` exited 1, with 9 pass and 2 fail (rows 6 and 10). `statusgen verifyrun --check` agrees: 9 pass, 2 fail, 0 could-not-run or missing, exit 1. The witness's Output hash for row 6 (sha256:4355a46b19d3) is the hash of `1` plus a newline, the same hash as rows 7 and 9. So row 6 printed the expected value and failed on its exit status alone.

Risk metadata is present and every field is `no` (irreversible: no). The diff touches prose and skill text only, so the fail-safe risk-bearing trigger does not fire. The enumeration is recorded anyway. Every entry is reversible by an edit, so the ranking puts the two entries that decide whether a mark clears first.

- RISK-VALUE: DERIVED — counted instance kinds = {confirmed-defect, false-positive}, deduped by incident-group @ docs/brittle-investigation-template.md:190 — this is the build-less-brittle spec's counting rule word for word (spec.md:110–114). Counting `intended-control` would let a working refusal hold a mark open.
- RISK-VALUE: DERIVED — fix-window = 90 days before mark-date @ docs/brittle-investigation-template.md:201 — matches spec.md:94 and :264 ("trailing 90 days") and the hotspot report's default (hotspot_test.go:22, "default 90 days before -until"), so the investigation reads the same window that nominated the mark.
- RISK-VALUE: DERIVED — fix-subject pattern = `\bfix(es|ed)?\b|revert`, case-insensitive @ docs/brittle-investigation-template.md:200 — byte-identical to `FixPattern` at tools/desk/internal/hotspot/hotspot.go:48 (`(?i)` there is `grep -i` here), so the history table lists exactly the commits the metric counted.
- RISK-VALUE: DERIVED — originating-commit read depth = `head -3` @ docs/brittle-investigation-template.md:117 — the brief's own input 1 fixes it (facts, "Inputs the template requires", 1).
- RISK-VALUE: DERIVED — tier = strong @ docs/brittle-investigation-template.md:10 and plugins/assay/skills/worker-desk/SKILL.md:312 — the brief's `exec-tier: strong`, and facts "Tier and size".
- RISK-VALUE: DERIVED — worker-desk lines added = 3, net 921 → 921 @ plugins/assay/skills/worker-desk/SKILL.md:312–314 — the brief's task 3 caps the addition at ≤ 3 lines and design-fit caps the net at ≤ 0. Both hold.
- RISK-VALUE: NAMED, NOT DERIVED — deletion-bundle security-control entry is human-gated (`needs-decision`, never retired on the owner's lane alone) @ docs/brittle-investigation-template.md:72–73 — neither the brief nor the build-less-brittle spec states this binding, so there is no source to derive it from. It only adds a gate and never removes one.

Review-support findings (semantic checks the brief's Review section asks for; these are not Verify rows):

- Intent reads run against the real hotspot tools/desk/internal/deskkit/forge_gitlab.go, at f7bde6bfa and at main (the same three commits both times). Read 1 yields f6cf09e73 (forge-gitlab/02), a3720cfc0 (forge-gitlab/08) and cd9d9fa86 (desk-supervision/01). In read 2, the first two commits carry a `Brief:` trailer directly. The third has no trailer. The template's offline merge-commit fallback (`git log --first-parent --ancestry-path --merges <sha>..refs/remotes/origin/main`) returned nothing for it, so its offline arm does not cover every commit that landed through a merge. The template's online arm (the commit-to-PR lookup) resolved it to #369, whose body reads `Brief: desk-supervision/01`. Read 3 finds both forge-gitlab brief files. Read 4 works: 42 of 46 commits come back with no trailer and must go through read 2, as the template says. Read 5 finds no `S-` row naming the file, and `s-row: none` is a legal value. So the reads find an originating brief on a real hotspot, not only on the worked example. The offline-fallback gap degrades to an `evidence-gaps:` entry, never to a guess.
- `accept` and `none`/`clear` are real outcomes. Each has a next act and an owner (template lines 79–83). A paragraph states they "code nothing… are real outcomes, not consolation prizes" (85–90). The settling evidence for `none` is under Divergence (254–255), and the worker-desk line repeats "`accept` and `clear` code nothing". The worked example picks `reconcile` and argues against `accept` and `clear` rather than leaving them out.
- Amendment walk: the worked example keeps r2's unavailable PR as an `evidence-gaps:` entry. It treats g1's later success on a different known-scope as another scope, not recovery. The competing explanation and the discriminating check sit under Divergence, and the expected observable under Next act. The intervening-change example marks C2–C4 `revalidate`, "not current", after a policy change outside the module's files. It explains why file non-overlap is not enough and requires a superseding revision, so the old conclusion is never presented as current.
- Verify-table defects behind the FAIL (the deliverables themselves check out). Row 6's `| head -1` fails under pipefail, and reading the full output instead (for example `sed -n 1p`) would avoid that. Row 10, like row 8, needs a post-merge re-derivation, such as running at the delivering commit with `--base` set to its parent. The template's own read 1 (`… | head -3`) has the same SIGPIPE exposure for any session that runs it under pipefail.

**VERIFY: FAIL — row 6 exits 141 under the repo's execution witness (bash -o pipefail SIGPIPE on `git log … | head -1`; output `1` matches the Expect, but the row as written cannot record a clean exit); row 10 is COULD-NOT-CHECK (exit=2) on merged main 56140a33d and reaches `exit=0` only at the PR head 8f64ec2fb, so the witness records it failed. Rows 1–5, 7–9 and 11 pass on merged main 56140a33d. The deliverables match the brief; the fix belongs in the Verify table.**

Execution witness: `statusgen verifyrun` ran for real on merged main 56140a33d (9 pass; row 6 fail, exit 141 under pipefail; row 10 fail, the at-main COULD-NOT-CHECK above; `verifyrun --check` agrees, exit 1). The witness table is not landed here because that run stamped no on-behalf-of principal. It is re-run under the stamped write path once the rows are re-authored. Status stays `implemented` with no flip. Row re-authoring is routed to #1915 (https://github.com/medici-finance/assay/issues/1915#issuecomment-6004541583).
2026-10-06 non-implementer re-verification on merged main 11228951d (confirmed by rev-parse and the commits API; delivered by #2249, squash 4329640e7) — VERIFY: FAIL, check-definition. Same two rows as the earlier pass (#2261); re-authoring tracked in #1915 (open). The deliverables are correct.

| # | Command | Exit | Observed output | Date | Runner |
| --- | --- | --- | --- | --- | --- |
| 1 | Verify row 1 as written | 0 | 6. PASS | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | Verify row 2 as written | 0 | 6 (at least 6). PASS | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | Verify row 3 as written | 0 | 6. PASS | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | Verify row 4 as written | 0 | 3 (at least 1). PASS | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | Verify row 5 as written | 0 | 2. PASS | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | Verify row 6 as written, in plain bash and under bash -o pipefail | 0 plain; 141 under pipefail and in the verifyrun witness | prints 1 in both shells, matching the Expect, but head -1 closes the pipe while git log --follow --reverse is still writing its 27-commit list, so git dies of SIGPIPE. Under pipefail the exit is 141 and the witness records fail. FAIL (as the witness runs it) | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | Verify row 7 as written | 0 | 1 (at least 1). PASS | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | Verify row 8 as written | 0 | NET-OK. Impl resolves to 4329640e7 with base at its parent e77868565; worker-desk SKILL.md is 921 lines at base and at tip (925 on main today from the later #2278, which this row does not measure). PASS | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 9 | Verify row 9 as written | 0 | 1 (at least 1). PASS | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 10 | Verify row 10 as written on merged main; supplement at the delivering commit with its parent as base | exit=2 on main; 0 at 4329640e7 | On main: "COULD-NOT-CHECK: assay:assay:build-less-brittle:09 is not in the diff against 11228951d", exit=2, which the witness records as fail. At the delivering commit: "summary: 2 corroborated, 0 disproved, 2 unchecked", exit 0. The row as written cannot decide on merged main (#1915) | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 11 | Verify row 11 as written, in a subshell | 0 | SOURCE-CHANGE-EXAMPLE. PASS | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |

Execution witness: statusgen verifyrun --dry-run, built from this tree's statusgen module, exits 1 with 9 pass and 2 fail (rows 6 and 10); nothing written back.

RISK-VALUE: DERIVED — counted instance kinds = confirmed-defect and false-positive, deduped by incident group @ docs/brittle-investigation-template.md:190-191; matches the spec's counting rule at spec.md:114.
RISK-VALUE: DERIVED — fix-window = 90 days before mark-date @ docs/brittle-investigation-template.md:201; matches spec.md:94 and :264 and the hotspot report's default window.
RISK-VALUE: DERIVED — fix-subject pattern @ docs/brittle-investigation-template.md:200; byte-identical to FixPattern at tools/desk/internal/hotspot/hotspot.go:48 (case-insensitive in both).
RISK-VALUE: DERIVED — originating-commit read depth = head -3 @ docs/brittle-investigation-template.md:117; fixed by the brief's facts, input 1.
RISK-VALUE: DERIVED — tier = strong @ docs/brittle-investigation-template.md:10 and plugins/assay/skills/worker-desk/SKILL.md:315; from the brief's exec-tier.
RISK-VALUE: NAMED, NOT DERIVED — a deletion-bundle entry that is a security control is human-gated (needs-decision) @ docs/brittle-investigation-template.md:72-73; neither brief nor spec states this binding. It only adds a gate. Reversible.

**VERIFY: FAIL** — check-definition. Row 6 exits 141 under the witness's pipefail although its output matches; row 10 is could-not-check (exit=2) on merged main and passes only at the delivering commit. Rows 1-5, 7-9 and 11 pass. Status stays implemented. Both rows need re-authoring: #1915.
### 2026-10-07 desk dispatch — VERIFY: PASS (build-less-brittle/09 @ fe2217521989, 11/11 rows)

Re-verification on merged main fe2217521989c9925a83e57c083fc11028990824. The commits API confirmed this was the forge's main at run time, and the worktree was detached at it. This run follows #2300 (merge 24886d98ad2a, an ancestor of this head), which re-authored rows 6 and 10. The delivering change is still squash 4329640e7 (#2249), and its parent is e77868565. Execution witness: `statusgen verifyrun --dry-run` (installed statusgen v1.0.32) exited 0, with 11 pass and 0 fail. Row 6's output hash sha256:4355a46b19d3 is the hash of `1` plus a newline. Every row was then run by hand with `bash -o pipefail`, and the results matched. Nothing was written back, and the worktree stayed clean (the row-10 throwaway clone was removed).

| Row | Command | Exit | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | Verify row 1 as written | 0 | `6`. PASS | 2026-10-07 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | Verify row 2 as written | 0 | `6` (at least 6). PASS | 2026-10-07 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | Verify row 3 as written | 0 | `6`. PASS | 2026-10-07 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | Verify row 4 as written | 0 | `3` (at least 1). PASS | 2026-10-07 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | Verify row 5 as written | 0 | `2`. PASS | 2026-10-07 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | Verify row 6 as written (re-authored by #2300: sed -n 1p in place of head -1), under pipefail and in the witness | 0 | `1`. The template's intent-read command, run on the forgeban allowlist file, yields a 40-hex originating commit. The earlier exit 141 under pipefail no longer occurs, in either the witness or the hand run. PASS | 2026-10-07 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | Verify row 7 as written | 0 | `1` (at least 1). PASS | 2026-10-07 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | Verify row 8 as written | 0 | `NET-OK`. impl resolves to 4329640e7, so base is e77868565 and tip is 4329640e7. PASS | 2026-10-07 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 9 | Verify row 9 as written | 0 | `1` (at least 1). PASS | 2026-10-07 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 10 | Verify row 10 as written (re-authored by #2300: pinned to the delivering commit with base at its parent, in a throwaway clone) | 0 | `consumers corroboration — base e77868565585…`, then `summary: 2 corroborated, 0 disproved, 2 unchecked, 0 brief(s) claiming nothing`. The CORROBORATED entries are the template and worker-desk SKILL.md §Un-briefed issues (both fixed-here). The UNCHECKED entries are contracts.md §Brittle marks (follow-up build-less-brittle/08) and installed deskdispatch binaries (out-of-scope). The Expect names exactly these. On merged main the earlier COULD-NOT-CHECK with exit=2 no longer occurs. PASS | 2026-10-07 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 11 | Verify row 11 as written, in a subshell | 0 | `SOURCE-CHANGE-EXAMPLE`. PASS | 2026-10-07 | assay-verifier-app[bot] (on-behalf-of human:ian) |

#2300 fixed both rows that failed earlier. Row 6 now exits 0 under the witness's pipefail, and row 10 decides on merged main (exit 0) instead of reporting could-not-check.

Risk metadata is present and every field is `no` (irreversible: no). The diff in scope (#2249 deliverables plus #2300's Verify-table edits) touches prose, skill text and Verify rows only, so the fail-safe trigger does not fire. #2300 changes no literal value in the deliverables. Enumerated over the deliverables, the top entries are re-checked at this head and unchanged: counted kinds confirmed-defect and false-positive at docs/brittle-investigation-template.md:190 (spec.md:114); fix-window of 90 days at :201 (spec.md:94, :264); the fix-subject pattern at :200, byte-identical to FixPattern at tools/desk/internal/hotspot/hotspot.go:48; read depth head -3 at :117; tier strong at :10 and plugins/assay/skills/worker-desk/SKILL.md:315. All of them are reversible by an edit.

RISK-VALUE: DERIVED — fix-window = 90 days before mark-date @ docs/brittle-investigation-template.md:201 — matches the spec's trailing-90-day hotspot window (spec.md:94 and :264) and the hotspot report's default, so the investigation reads the same window that nominated the mark.

**VERIFY: PASS**
| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -c -e '^## Intent$' -e '^## What happened$' -e '^## Divergence$' -e '^## Options$' -e '^## Recommendation$' -e '^## Next act$' docs/brittle-investigation-template.md` | pass exit=0 | sha256:06e9d52c1720 | 2026-10-07 | assay-verifier-app[bot] @ fe2217521989 (on-behalf-of human:ian) (forge-identity) |
| 2 | `grep -c -e '^divergence:' -e '^recommendation:' -e '^next-act:' -e '^end-state:' -e '^module:' -e '^class-issue:' docs/brittle-investigation-template.md` | pass exit=0 | sha256:06e9d52c1720 | 2026-10-07 | assay-verifier-app[bot] @ fe2217521989 (on-behalf-of human:ian) (forge-identity) |
| 3 | `grep -o -e 'drifted' -e 'intent-changed' -e 'implementation-wrong' -e 'reconcile' -e 'redesign' -e 'accept' docs/brittle-investigation-template.md \| sort -u \| wc -l \| tr -d ' '` | pass exit=0 | sha256:06e9d52c1720 | 2026-10-07 | assay-verifier-app[bot] @ fe2217521989 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -c 'NEEDS_CONTEXT' docs/brittle-investigation-template.md` | pass exit=0 | sha256:1121cfccd591 | 2026-10-07 | assay-verifier-app[bot] @ fe2217521989 (on-behalf-of human:ian) (forge-identity) |
| 5 | `printf '%s\n' 'divergence: drifted' 'divergence: intent-changed' 'divergence: intent-right, implementation-wrong' 'divergence: none' 'recommendation: reconcile' 'recommendation: redesign' 'recommendation: accept' 'recommendation: clear' > /tmp/bl09-vocab.txt && awk '/^---$/{n++; next} n==1' docs/brittle-investigation-template.md \| grep -xF -f /tmp/bl09-vocab.txt \| cut -d: -f1 \| sort -u \| grep -c .` | pass exit=0 | sha256:53c234e5e847 | 2026-10-07 | assay-verifier-app[bot] @ fe2217521989 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cmd=$(grep -oE 'git log --follow --reverse[^<]*<path>' docs/brittle-investigation-template.md \| sed -n 1p); test -n "$cmd" && eval "${cmd/<path>/tools/desk/internal/forgeban/allowlist.go}" \| sed -n 1p \| grep -cE '^[0-9a-f]{40} '` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-07 | assay-verifier-app[bot] @ fe2217521989 (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -c 'brittle-investigation-template' plugins/assay/skills/worker-desk/SKILL.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-07 | assay-verifier-app[bot] @ fe2217521989 (on-behalf-of human:ian) (forge-identity) |
| 8 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/09$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/worker-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/worker-desk/SKILL.md" \| wc -l)" && echo NET-OK` | pass exit=0 | sha256:458c4e39effe | 2026-10-07 | assay-verifier-app[bot] @ fe2217521989 (on-behalf-of human:ian) (forge-identity) |
| 9 | `test -f docs/investigations/README.md && grep -c 'recommendation' docs/investigations/README.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-07 | assay-verifier-app[bot] @ fe2217521989 (on-behalf-of human:ian) (forge-identity) |
| 10 | `d=; impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/09$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; b=$(git rev-parse "$base") && t=$(git rev-parse "$tip") && test "$b" != "$t" && d=$(mktemp -d "$PWD/.bl09-consumers.XXXXXX") && git clone -q --shared --no-checkout . "$d" && git -C "$d" checkout -q --detach "$t" && statusgen --consumers --root "$d" --brief build-less-brittle/09 --base "$b"; s=$?; test -n "$d" && rm -rf "$d"; exit $s` | pass exit=0 | sha256:6d6298a471e7 | 2026-10-07 | assay-verifier-app[bot] @ fe2217521989 (on-behalf-of human:ian) (forge-identity) |
| 11 | `f=docs/brittle-investigation-template.md; for key in source-revisions dependency-references unresolved-questions; do grep -qF "$key:" "$f" \|\| exit 1; done; awk '/^## / {p=($0 == "## Divergence")} p && /^### Intervening-change example$/ {found=1} END {exit !found}' "$f" && echo SOURCE-CHANGE-EXAMPLE` | pass exit=0 | sha256:50ea47cc4bad | 2026-10-07 | assay-verifier-app[bot] @ fe2217521989 (on-behalf-of human:ian) (forge-identity) |

## Review
Gate: model (from frontmatter). The reviewer runs the template's `## Intent` reads against one
real hotspot from 08's sample (`tools/desk/internal/deskkit/forge_gitlab.go` at f7bde6bfa) and checks that
the commands find an originating brief. A template whose reads work only on the worked example
is a finding. The reviewer also checks that `accept` and `none` are real outcomes and not
buried: an investigation that can only recommend work is a patch generator with extra steps.
