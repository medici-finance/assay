---
brief: issue-loop/15
title: 'Intake directory split by disposition — `ls` is the triage board; a move is a transition, identity is the entry id'
wave: 6
depends: ["issue-loop/08"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-22 by strong-tier authoring session (I-intake-dir-split)
sources: ["[I-intake-dir-split](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-12-intake-directory-split-by-disposition.md) (human:<name> 2026-07-12, verbatim: 'the intake directory should be split up. those completed should go in intake/completed, new in intake/new and so on.')", "issue-loop/08 (triage verbs — DONE 2026-07-16; this brief retrofits the layout under 08's existing verbs, superseding the entry's 'fold into 08' sequencing)", "issue #1023 + medici-finance/assay-toolkit#117 (2026-07-22: verify-desk owns findings resolution — findings/ symmetry scoped OUT, recorded as follow-on)", ".assay-versions + .github/workflows/statusgen.yml (assay-dogfood/03: tools/statusgen is a FROZEN copy, canonical source = medici-finance/assay-toolkit/statusgen, consumed as pinned release binary statusgen/v0.1.0)", "freshness-checked 2026-07-22 @ ef1de62a (docs/streams/intake/ is flat: 88 root-level entries; parseIntakeDir skips dirs; deletedRegisterFiles is path-identity — nothing of this brief is landed)"]
why: >-
  88 intake entries sit in one flat directory, every triage state mixed — the board is invisible
  without parsing frontmatter or the generated view. Splitting by disposition makes `ls` the triage
  board: the untriaged set is one glob (intake/new/*.md), the humans-blocking set is one dir, and a
  triage verb becomes a visible file move instead of an invisible frontmatter flip.
gate-why: >-
  This brief changes statusgen's tombstone-not-delete tamper wire (deletedRegisterFiles) from
  path-identity to id-identity so a git mv between subdirs stops reading as a deletion. Integrity/
  anti-falsification check changes are human-gate by standing decision (PR #255 precedent). The human
  confirms: (a) the relaxation cannot disguise a real delete as a move (the id must still resolve to
  a file under the SAME register dir, and the duplicate-id check closes the copy-without-delete
  hole), and (b) the assay-toolkit-release-first sequencing keeps main CI green through the
  migration.
exec-tier: strong
exec-tier-why: cross-repo/cross-component reasoning (statusgen release ↔ this repo's migration, question b) and an integrity-check change where a subtle bug survives the brief's own tests (question c).
consumers: ["medici-finance/assay-toolkit statusgen/ (canonical source): fixed-here — sibling draft PR + release statusgen/v0.1.1; record the sibling-repo SHA in this repo's PR (#272 pairing)", "tools/statusgen/ (frozen copy): out-of-scope (frozen per assay-dogfood/03; statusgen.yml tripwire blocks .go edits; all consumers use the pinned release binary)", ".assay-versions: fixed-here (bump tag+sha256 in the SAME commit as the migration so CI never runs an old binary against the new layout)", ".claude/skills/intake-desk/SKILL.md (triage lane): fixed-here (verb = frontmatter update + git mv, one commit)", "docs/streams/issue-loop/README.md 'Triage verbs' block: fixed-here (each verb gains its move; new entries filed under intake/new/)", ".claude/skills/author-brief/SKILL.md closing-step 2 (intake disposition flip): fixed-here (flip now includes the move)", "CLAUDE.md INTAKE line: out-of-scope (its statements stay true under the split — per-entry files still live under docs/streams/intake/, deletion still trips --lint; single-writer rule — any wording polish rides a future in-scope CLAUDE.md PR)", "docs/streams/findings/ layout symmetry: out-of-scope (verify-desk owns findings resolution per issue #1023 / assay-toolkit#117 — explicit follow-on candidate, authored in that remit if adopted; see Task 6)"]
---

# Brief 15 — Intake directory split by disposition

**CROSS-REPO**: the statusgen code change lands in `medici-finance/assay-toolkit` (canonical
statusgen source) as a sibling draft PR + release `statusgen/v0.1.1`; this repo's PR carries the
`.assay-versions` bump, the one-commit migration, and the convention/skill updates. Manifest-style
rule applies: nothing here merges before the release exists (its sha256 goes into
`.assay-versions`). `../assay-toolkit/statusgen/` is frozen — do not touch its `.go` files.

## Context

files: `../assay-toolkit/statusgen/{registerentries.go,registers.go,registerrefs.go,viewlinks.go}` + tests
+ testdata (sibling repo); this repo: `.assay-versions`, `docs/streams/intake/**` (migration),
`docs/streams/issue-loop/README.md`, `../oit/.claude/skills/intake-desk/SKILL.md`,
`../oit/.claude/skills/author-brief/SKILL.md`

facts:

- **Layout (the design decision, settled here):** five subdirs under `docs/streams/intake/`,
  one per coarse triage state — **`new/`** (untriaged; `disposition: new` or missing),
  **`decision-needed/`** (waiting on a human), **`watching/`**, **`completed/`** (routed out:
  `scoped → <stream>`, `scoped → issue #NN`, legacy `adopted`), **`rejected/`** (tombstones).
  Why this set and not dir-per-raw-disposition-value: human:<name>'s verbatim named `completed`; the three
  routed-out disposition spellings collapse into one done-dir; the dir is the coarse STATE, the
  frontmatter `disposition:` stays the authoritative detail (`scoped → <stream>`,
  `decision-issue: NN`). `ls intake/new/` IS the triage board; `intake/new/*.md` IS the untriaged
  glob (brief-07's alarm semantics unchanged — it keys on frontmatter, and the new consistency
  lint makes glob == frontmatter set).
- **Identity = entry id, never path** (design point 1 of the intake entry). A `git mv` between
  subdirs is a disposition TRANSITION. statusgen touchpoints (all in assay-toolkit's `statusgen/`):
  1. `registerentries.go parseIntakeDir` — currently `os.ReadDir` root-only, skips dirs; must also read
     one level into the five known subdirs, recording each entry's subdir on `intakeEntry` (new
     field). Root-level `.md` files stay supported (flat-layout compat for adopter repos).
  2. `registers.go registerIntegrityProblems` — the direct `os.ReadDir(intakeDir)` field-quality
     loop (missing-disposition check, L40–75) walks the subdirs too. NEW checks: dir↔disposition
     mismatch = PROBLEM (e.g. a `disposition: new` file in `completed/`); unknown subdir name
     under `intake/` = PROBLEM; root-level entry file while subdirs exist = advisory NOTICE
     ("file under intake/<state>/").
  3. `registers.go deletedRegisterFiles` (the tombstone-not-delete tamper wire, L137–213) — for
     each merge-base-landed path absent from the working tree, parse the merge-base blob
     (`git show <merge-base>:<path>`) for its frontmatter `id:`; if that id resolves to a file
     currently under the SAME register dir (any subdir), it is a move, not a delete — no
     violation. Id unresolvable, or resolvable only OUTSIDE the register dir → still fires. The
     existing duplicate-id PROBLEM closes the copy-without-delete hole.
  4. `registers.go generateIntakeView` — grouping/sections/sort unchanged (disposition-driven,
     date-then-id, byte-deterministic); the body-link rebase must use the entry's real depth:
     `rebaseEntryBodyLinks(e.Body, "intake/"+e.Subdir)` (viewlinks.go's rebase is
     depth-sensitive; root entries keep `"intake"`).
  5. `registerrefs.go buildRegisterMap` — walk the subdirs so the id→path map covers moved
     entries (a typed `I-NN` link whose target path vanished is already a hard PROBLEM — the
     migration rewrites them; the map is what makes the new paths resolvable).
  6. No change: `intake_alarm.go` (keys on frontmatter disposition), `idvalidate.go`/`load.go`
     (route through parseIntakeDir), findings-side code.
- **Repo routing:** canonical statusgen = `medici-finance/assay-toolkit/statusgen` (verified
  current on GitHub 2026-07-22 — carries all brief-07/08 files; single release
  `statusgen/v0.1.0`). `../assay-toolkit/statusgen/` here is a frozen copy; `statusgen.yml` hard-fails any
  PR diffing its `.go` files. CI (status-regen.yml etc.) downloads the pinned binary per
  `.assay-versions` (tag + sha256). NOTE: the sibling checkout `../assay-toolkit` can be stale
  (silent fetch failure) — `git fetch` and confirm against GitHub before branching.
- **Sequencing (the real head of this brief's path):** the assay-toolkit change + release is the
  head. Main's CI regenerates views with the PINNED binary: if the migration merged first, v0.1.0
  would drop every moved entry from the view AND fire tombstone PROBLEMs on all 88 paths. Order:
  sibling PR → review → release `statusgen/v0.1.1` (human cuts it; CI-built linux-amd64 sha) →
  this repo's PR with `.assay-versions` bump + migration in the same commit.
- **Migration (design point 4):** ONE commit: `git mv` every entry to its dir per the mapping
  (new/missing→`new/`, decision-needed→`decision-needed/`, watching→`watching/`,
  scoped|adopted→`completed/`, rejected→`rejected/`), plus rewrite every `intake/2026` path
  reference under `docs/**` (markdown links AND bare path mentions; Verify 6 sweeps the class).
  Bare `[[id]]`/typed-ID text survives untouched (id is the key). Tombstone rules unchanged:
  `rejected/` files are still never deleted.
- **Triage-verb retrofit (08 is DONE):** a verb's disposition write becomes frontmatter update +
  `git mv` to the matching subdir in ONE commit. Conventions live in this stream README's
  "Triage verbs" block + `../oit/.claude/skills/intake-desk/SKILL.md`; new entries are filed under
  `intake/new/`.

## Ground rules

- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Do NOT edit `../assay-toolkit/statusgen/**.go` (frozen; CI tripwire). Do NOT commit regenerated views
  (`STATUS.md`, docs/streams/INTAKE.md, docs/streams/FINDINGS.md) on a branch — restore them
  after local regeneration.
- The sibling assay-toolkit PR stays DRAFT; the release cut is human-gated. This repo's PR marks
  itself blocked until the release sha is in `.assay-versions`.

## Task

1. **assay-toolkit sibling PR** (statusgen): implement Context touchpoints 1–5 with tests —
   subdir parse + `Subdir` field; integrity walk + dir↔disposition mismatch PROBLEM +
   unknown-subdir PROBLEM + root-file NOTICE; id-identity `deletedRegisterFiles` (move ≠ delete,
   cross-register move still fires); register-map walk; view rebase depth. Fixtures cover: split
   layout, flat layout (compat), mixed, a moved entry, a truly deleted entry, a copy-without-delete
   (duplicate id).
2. **Release**: hand the sibling PR through its review; a human cuts `statusgen/v0.1.1`
   (CI-built linux-amd64 + sha256).
3. **This repo — one migration commit**: bump `.assay-versions` (tag + sha256) AND `git mv` all
   intake entries per the mapping AND rewrite every `intake/2026` path reference under `docs/**`
   (markdown links and bare path mentions alike) — same commit, so CI never sees old-binary/new-layout or new-binary/half-moved.
4. **Conventions**: amend this README's "Triage verbs" block (each verb's move; new entries →
   `intake/new/`; the one-commit rule); same retrofit in `../oit/.claude/skills/intake-desk/SKILL.md`
   (triage lane) and `../oit/.claude/skills/author-brief/SKILL.md` closing-step 2 (disposition flip now
   includes the move).
5. **Intake entry**: [I-intake-dir-split](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-12-intake-directory-split-by-disposition.md)
   moves to `completed/` with the migration (it is `scoped`); its frontmatter/body edits landed at
   authoring time — do not re-edit beyond the move.
6. **Findings symmetry — explicitly OUT** (design point 3): `docs/streams/findings/` has the same
   flat shape, but the verify-desk owns findings resolution (issue #1023, assay-toolkit#117,
   2026-07-22). Record nothing here beyond this note; if the split proves out, the follow-on brief
   is authored in that remit.

## Verify (executable — no prose-only DoD items)

Run in this repo at the implementation head, with the pinned `statusgen` binary installed per
`../assay-toolkit/statusgen/README.md` (fallback: `go run` in a FRESH `../assay-toolkit/statusgen` checkout at
the released tag). Rows 3–5 regenerate views locally — restore them afterwards, never commit.

| # | Command | Expect |
|---|---------|--------|
| 1 | `statusgen --root . --lint` | exit 0 on the migrated layout; no `tombstone-not-delete` PROBLEM for any moved entry |
| 2 | `cd statusgen && git fetch origin --tags && git checkout statusgen/v0.1.1 -- . && go test ./... -count=1` | exit 0; includes the Task-1 subdir/move/delete/duplicate cases |
| 3 | `statusgen --root . && grep -n '^## I-intake-dir-split' docs/streams/INTAKE.md; ec=$?; git checkout -- docs/streams/INTAKE.md docs/streams/FINDINGS.md STATUS.md; exit $ec` | exit 0 — a MOVED entry's id resolves in the regenerated view |
| 4 | `statusgen --root . && test "$(ls docs/streams/intake/new/*.md \| wc -l)" -eq "$(grep -c '^Disposition: new$' docs/streams/INTAKE.md)"; ec=$?; git checkout -- docs/streams/INTAKE.md docs/streams/FINDINGS.md STATUS.md; exit $ec` | exit 0 — the untriaged glob `intake/new/*.md` equals the view's untriaged set |
| 5 | `f=$(ls docs/streams/intake/rejected/*.md \| head -1); mv "$f" "${TMPDIR:-/tmp}/tomb.md"; statusgen --root . --lint > "${TMPDIR:-/tmp}/lint.out" 2>&1; ec=$?; mv "${TMPDIR:-/tmp}/tomb.md" "$f"; test $ec -ne 0 && grep -q tombstone-not-delete "${TMPDIR:-/tmp}/lint.out"` | exit 0 — a REAL removal (file leaves the register dir) still trips the tamper wire |
| 6 | `grep -rEn 'intake/2026' docs --include='*.md' \| grep -vE -e 'intake/new/' -e 'intake/completed/' -e 'intake/decision-needed/' -e 'intake/watching/' -e 'intake/rejected/' -e 'docs/streams/INTAKE.md' -e 'docs/streams/FINDINGS.md' \| wc -l` | `0` — no stale `intake/2026` path reference (markdown link or bare prose) survives the migration under `docs/` (class sweep, excluding the five subdir prefixes and generated views) |
| 7 | `! (git diff --name-only origin/main...HEAD \| grep -qE '^tools/statusgen/.*[.]go$') && grep -n 'statusgen statusgen/v0[.]1[.][1-9]' .assay-versions` | exit 0 — frozen copy untouched AND pinned tag bumped past v0.1.0 with its sha |

## Evidence

<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review

Gate: human (from frontmatter — integrity-check change, see gate-why). Reviewer confirms
(a) id-identity cannot disguise a delete: the moved-id lookup is scoped to the SAME register dir
and the duplicate-id PROBLEM still fires on copy-without-delete; (b) flat layout still parses
(adopter compat); (c) the `.assay-versions` bump and the migration are ONE commit; (d) the view
stays byte-deterministic and its sections/sort are unchanged; (e) findings/ symmetry stayed out of
scope per issue #1023. Human sign-off recorded as `human:<name>` in the README row.
