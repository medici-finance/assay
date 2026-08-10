---
brief: methodology/33
title: 'Register references become links — author-brief convention + 94-brief backfill + lint (F-NN/I-NN → the per-entry file)'
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by Fable desk session (human:<name> direction)
sources: ["human:<name> 2026-07-10: fix the author briefs + scan the existing ones so Intake/Findings references link to the actual file", "author-brief skill rule 2 (typed IDs — the convention being extended, not replaced)", "methodology/23 (registers-as-directories — the migration the convention must survive)", "issue #221 (out-of-repo skill edit protocol)", "measurement 2026-07-10 (refreshed): F-refs 185 across 37 briefs (deep — load-bearing body text); I-refs 170 across 85 briefs (broad but shallow — mostly one provenance citation in sources:)", "freshness-checked 2026-07-10 @ authoring-debt-batch head"]
why: >-
  A bare F-NN/I-NN is a typed ID you must grep for — fine for scripts, friction for every
  human and agent reader, and (worse) unverifiable: a typo'd [F-17](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-40-confirmed-ws-streams-die-5-min-the-subscribe-time-caller-.md) that means [F-18](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-42-confirmed-dar-not-on-ledger-startup-warning-compares-the-.md) greps
  clean and lints clean today. Making references links gives readers one-click context and
  gives the linter something checkable: a link that names an entry that doesn't exist is a
  mechanical PROBLEM instead of a silent wrong pointer. 252 existing references across 94
  briefs make this worth one systematic pass rather than piecemeal drift.
---

# Brief 33 — Register references become links

## Context
files: `../assay-toolkit/statusgen/` (reference lint), `../oit/.claude/skills/author-brief/SKILL.md` (in-repo
wrapper — convention text), all 94 briefs under `docs/streams/*/brief-*.md` (backfill scan)
out-of-repo files: `~/.claude/skills/author-brief/SKILL.md` (core template rule 2
amendment — per issue #221's protocol)
facts:
- **Convention (the deliverable's heart — registers are per-entry files since
  methodology/23):** a register reference is the typed ID *as the link text* pointing at
  the entry's OWN file: `[F-15](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-operatorexecute-s-oracle-vaultadmin-oracle-depositor-asserti.md)` /
  `[I-23](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-desk-tools-zero-prompt-workflow-plumbing-purpose-built-binar.md)` — relative from the brief's directory, no
  anchors needed (stable file paths). The typed ID remains the primary key (author-brief
  rule 2 unchanged — scripts still grep the ID; the link is FOR readers; the ID→file
  mapping is each entry file's frontmatter `id:`). In YAML frontmatter `sources:`/prose
  alike, the markdown-link form is legal string content. The generated FINDINGS.md/
  INTAKE.md views are NEVER link targets (single-writer artifacts; linking them re-couples
  readers to a file that regenerates).
- **Lint (statusgen):** for every markdown link whose text matches `^[FI]-\d+$`: (a) the
  target file must exist, (b) the target file's frontmatter `id:` must EQUAL the link
  text (PROBLEM if not — catches both the typo'd-reference class and stale links after
  any future renumber), (c) a link whose target is the generated view (FINDINGS.md/
  INTAKE.md) is a NOTICE steering to the entry file. BARE (unlinked) F-NN/I-NN references
  stay legal — no retroactive hard requirement; the author-brief convention makes links
  the default going forward (same phasing pattern as gate-why).
- **methodology/23 is LIVE (2026-07-10):** registers are already directories; this brief
  is directory-native from birth. The backfill maps each bare ID to its entry file by
  reading every entry's frontmatter `id:` (never by guessing slugs).
- **Ambiguous-ID set (human:<name>, 2026-07-10) — the backfill must NEVER auto-link these; resolve
  per citing brief:** renumber cascades and parallel filings left bare tokens whose meaning
  depends on WHEN the citing text was written. Known members at authoring:
  - **[I-30](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-dogfood-the-methodology-via-the-assay-marketplace-new-initia.md)**: before the 2026-07-10 chain renumber, "[I-30](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-dogfood-the-methodology-via-the-assay-marketplace-new-initia.md)" meant the model-mix entry (now
    `I-31`); after it, `I-30` is the assay-dogfood entry. Resolve by the citing text's
    authoring date vs the renumber commits, and by topic (model-mix language vs dogfood/
    marketplace language).
  - **[F-22](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-10-security-review-mandate-is-prose-only-desk-tools-02-03-specs.md) / F-22-a**: double-filed in parallel — one [F-22](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-10-security-review-mandate-is-prose-only-desk-tools-02-03-specs.md) = the security-review mandate
    finding (#216, cited by methodology/30), one = the registers-pre-23-conflict finding
    (history: 09ae0841 vs 4ca2de89/122160c7). RESOLUTION (human:<name>, 2026-07-10): the sibling
    renames to the suffix form **F-22-a** rather than renumbering. The implementer
    confirms which entry holds which ID once both land, then links each citation by topic
    (#216/security language vs register-conversion language) to [F-22](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-10-security-review-mandate-is-prose-only-desk-tools-02-03-specs.md) or F-22-a
    accordingly. The link-text pattern therefore tolerates a single-letter collision
    suffix: `^[FI]-\d+(-[a-z])?$` alongside the slug form.
  Procedure: build the ambiguity set at implementation time (these two + any ID rewritten
  in a register renumber commit — `git log -S` the register paths); for members, the
  script links only via an explicit per-citation resolution table committed in the PR
  description; a citation the implementer cannot confidently resolve stays BARE and lands
  on a flagged list for the desk — a wrong link is worse than no link (C-10 spirit).
- **Prioritization from the measured split (2026-07-10):** I-refs are broad-but-shallow
  (170 refs / 85 briefs, mostly single provenance citations in `sources:` — mechanical,
  low ambiguity); F-refs are narrow-but-deep (185 refs / 37 briefs, load-bearing body
  text — where Affects semantics and the [F-22](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-10-security-review-mandate-is-prose-only-desk-tools-02-03-specs.md)-class ambiguity live). The script treats
  both; the implementer's ATTENTION goes to the 37 F-bearing briefs, and the spot-diffs
  in the PR description come from that set.
- **Backfill is scripted, not hand-edited:** a small committed tool (`tools/statusgen`
  subcommand or `tools/desk` script — implementer picks, records why) that rewrites bare
  `F-NN`/`I-NN` tokens in brief files to the linked form, idempotently (already-linked
  refs untouched; IDs inside code fences and command Verify rows untouched — a grep
  pattern in a Verify row must not become a link and break the command). 252 references
  across 94 briefs is machine work; the script IS the deliverable, the rewritten briefs
  are its output, and methodology/23 reuses it.
- **Author-brief edits (both homes):** rule 2 gains the link form as the standard
  reference style with one example; the template's `sources:` comment shows it. Core =
  out-of-repo per #221 (declared above, apply-last, diff in PR body); wrapper = in-repo,
  rides this PR.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- Out-of-repo skill edit per issue #221's protocol (declared above).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Implement the lint per facts (three checks, PROBLEM/NOTICE severities as specified);
   tests: valid link passes; nonexistent ID → PROBLEM; stale anchor → NOTICE; bare ID →
   silent; linked ID inside a code fence ignored.
2. Implement the backfill script per facts (idempotent, fence/Verify-row-safe); run it;
   commit script + rewritten briefs in the SAME PR (reviewable together); spot-diff 3
   briefs in the PR description showing before/after.
3. Amend both author-brief homes (rule 2 + template example); methodology/23 Context
   one-liner noting the migration re-runs the script.
4. Run the full lint on the rewritten tree — zero new PROBLEMs (any pre-existing broken
   reference the backfill EXPOSES gets fixed or filed, listed in the PR description).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/... -count=1` | exit 0; includes every Task-1 case |
| 2 | backfill script run twice; `git status --porcelain \| wc -l` after second run | 0 (idempotent) |
| 3 | `grep -roE -e "\[F-[A-Za-z0-9][A-Za-z0-9-]*\]\(" -e "\[I-[A-Za-z0-9][A-Za-z0-9-]*\]\(" docs/streams/*/brief-*.md \| wc -l` | ≥480 (true ref count — counts refs, not lines; 497 at head. A backfill regression that dropped a chunk of rewrites turns this red, unlike the old line-count `≥200`.) **RED at 452 on 2026-08-02, re-measured 457 on 2026-08-03** (the +5 is merge drift from `origin/main`, not a change this sweep made; still short of `≥480`) and left red deliberately. Two amendments, neither of which retunes the threshold: the `(F\|I)` alternation was a literal pipe under ERE, so the row was counting ~0 and could never have been green; and the ID grammar became a letter slug in methodology/35, so `[0-9]+` no longer matches a live ID. With both corrected the corpus yields 452, short of 480 because briefs split across repos in the assay-selfcontain move — a real staleness that belongs to a follow-up re-baseline, NOT to this sweep silently lowering the number to make the row pass |
| 4 | `grep -c "](../findings/\|](../intake/" .claude/skills/author-brief/SKILL.md` | ≥1 (wrapper carries the convention example) |
| 5 | PR body contains the out-of-repo core diff (#221) + the 3 before/after spot-diffs | present |
| 5b | PR body contains the per-citation resolution table for every ambiguous-set member (≥ F-22, I-30), and every dangling reference found (e.g. F-22 cited on main with no entry) is fixed or flag-listed | present |
| 6 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

### Non-implementer verifier run — VERIFY: PASS after backfill completion (glm-5.2-verifier, in-repo main `abc64aec`, impl PR #497 `66ed5365`, 2026-07-18)

Verifier ran isolated (own worktree). Row 2 (backfill idempotency) initially FAILED: the backfill was
**incomplete on main** — 23 stale bare refs across 6 files, all post-dating PR #497's one-shot backfill
(F-41 dated 2026-07-16; several re-verify Evidence rows). Completing the backfill
(`go run ./tools/statusgen --register-links`) linkified the 23 refs; re-run then links 0 → idempotent → row 2 PASS.

| # | Command | Exit | Key output |
|---|---------|------|------------|
| 1 | `go test ./tools/statusgen/... -count=1` | 0 | `ok …/statusgen`; `registerrefs_test.go` covers ValidLink/DeadLink/WrongID/ViewLinkNotice/BareRefIsSilent/FencedLinkIgnored/PathTraversal/ReplaceBareRefs/StripFences |
| 2 | backfill run twice; porcelain after 2nd | — | Run 1 = 23 refs / 6 files; **post-completion re-run = 0 bare refs linked** → idempotent |
| 3 | link count `grep -roE "\[(F\|I)-[0-9]+\]\("` | 0 | 499 pre → 522 post (+23 = the linkified refs; ≥480) |
| 4 | author-brief skill carries the convention example | 0 | 1 hit (≥1) |
| 5/5b | PR #497 body: out-of-repo diff (#221) + spot-diffs + ambiguity table (F-22/I-30) + dangling-ref handling | — | present |
| 6 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | exit 0 (advisory NOTICEs only) |

**Completeness gap fixed in this run** — the 6 backfilled files (committed alongside this Evidence):
desk-tools/brief-03 (`F-29`×3, `F-30`×3), issue-loop/brief-04 (`F-31`/`F-37`/`F-38`), issue-loop/brief-09 (`F-41`×6),
issue-loop/brief-11 (`F-41`×4), methodology-metrics/brief-19 (`F-12`×2), example-poc/brief-10 (`F-37`×2). All 23
linkified refs resolve to real entries whose `id:` matches; none dangling. Verify-row safety (Review point a)
checked: in every rewrite the ref sits in a prose/output cell while shell commands stay in backticks, untouched.
`gate: model`, all four risks `no` → model flip permitted → `implemented → verified`.

## Review
Gate: model. Reviewer confirms (a) the backfill script is fence/Verify-row-safe (spot-check
a brief whose Verify row greps for an F-NN token — it must be untouched), (b) the lint's
ID-existence check is register-shape-agnostic (survives methodology/23), (c) typo'd-ID
detection actually fires (the PROBLEM path has a test), (d) both author-brief homes updated.
