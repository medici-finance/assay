---
brief: assay:assay:desk-supervision:26
title: Land one verify tick's Evidence-only outcomes in one Evidence PR
why: >-
  On a repository whose main requires a pull request, every verify outcome today opens its own
  Evidence PR, and each one costs two review lanes, a CI run and a merge. On 2026-09-27, 35 of 40
  open verifier PRs were Evidence-only landings of a failed or blocked verify. Landing one tick's
  outcomes in one PR cuts the PR count, and with it the review count, CI runs and forge writes, by
  the batch size. A refused entry is left out of the batch instead of stopping it.
wave: 1
depends: ["desk-supervision/24"]
unblocks: []
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1568]
schema: brief-v2
version: 1
id: 8bf9453a-31c7-411d-aba0-de7f11735c80
authored: 2026-09-27 by worker-desk authoring session (#882 ruling, option 3)
exec-tier: strong
exec-tier-why: >-
  (b) one verb gates many entries, writes them atomically through two forge backends and must keep a
  refused entry from blocking or leaking into the rest; (c) a batch that silently drops an entry's
  outcome survives a happy-path test.
domain: complicated
sources:
  - "medici-finance/assay#882 — the 2026-09-27 driver ruling that selects this option (option 3 of 3, with desk-supervision/24 and desk-supervision/25)"
  - "medici-finance/assay#1568 — the driver's direction for batching (one PR per window, 15 minutes by default as an adopter value, never append to a batch under review, flips stay single-brief in the interim); this brief carries it"
  - "docs/streams/desk-supervision/brief-23-evidence-lander-gatekeeper.md — names the batch Evidence PR as its fallback path"
  - "live read 2026-09-27: open verifier-App PRs and titles; batch PRs #1576 (merged 2026-09-24), #1574 and #1586 (open)"
  - "freshness-checked 2026-09-27 @ b227b4076 (origin/main): deskevidence writes one file per call; no batch verb exists; the skill still says one brief = one branch = one draft PR"
consumers:
  # Authoring PR: routed to this brief (rule 6); each flips to fixed-here in the implementation
  # commit that edits the path.
  - "tools/desk/cmd/deskevidence (the batch verb): follow-up desk-supervision/26 (this brief; flips to fixed-here when the implementation edits it)"
  - "tools/desk/internal/deskkit (a multi-file commit on the Forge interface, GitHub and GitLab backends): follow-up desk-supervision/26 (this brief; flips to fixed-here when the implementation edits it)"
  - "plugins/assay/skills/verify-desk/SKILL.md (the PR-required-main lane): follow-up desk-supervision/26 (this brief; flips to fixed-here when the implementation edits it)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md (reviewing a batch; asking for an entry to be dropped): follow-up desk-supervision/26 (this brief; flips to fixed-here when the implementation edits it)"
  - "tools/desk/README.md (deskevidence section): follow-up desk-supervision/26 (this brief; flips to fixed-here when the implementation edits it)"
  - "statusgen PR-link derivation (singleBriefTrailer, statusgen/ghfetch.go:256): out-of-scope (a batch carries no `Brief:` trailer and no status flip, so derivation is untouched; a multi-brief trailer for flips is deferred to an unauthored brief, because flips were 5 of 40 landings in the 2026-09-27 queue and the trailer is a shared value every PR-link reader consumes)"
  - ".github/workflows/evidence-automerge.yml: out-of-scope (a batch is a verifier-App PR touching only docs/streams/; the lane admits it unchanged, and #1576 merged through it)"
---

# Brief 26 — One Evidence PR per verify tick

## Context

files:
- **edit** `tools/desk/cmd/deskevidence/` — add `batch.go` (planned) + `batch_test.go` (planned); the
  usage text in `main.go`.
- **edit** `tools/desk/internal/deskkit/` — a multi-file commit on the `Forge` interface and its
  GitHub and GitLab implementations, with golden tests beside the existing forge goldens.
- **edit** `plugins/assay/skills/verify-desk/SKILL.md`, `plugins/assay/skills/pr-review-desk/SKILL.md`,
  `tools/desk/README.md`.
- **add** `changelog/<branch>.md` — the fragment this repo enforces.

facts (2026-09-27 @ b227b4076; re-establish from the named files and commands at pickup):
- **The skill today.** `plugins/assay/skills/verify-desk/SKILL.md`, "Public repo (PR-required main)": one Evidence PR per
  brief, cut server-side from main, one `deskevidence` call per file, a single `Brief:` trailer, and
  "Land-as-each-verdict-arrives … One brief = one branch = one draft PR".
- **The ruled direction (#1568, open).** Batch Evidence landings into one PR per time window, 15
  minutes by default as an adopter value and never hard-coded in the skill body; never append to a
  batch PR that is already under review, but start the next window instead. Interim rule: a landing that
  flips a brief's status stays in its own single-brief PR; only Evidence-only landings (status stays
  `implemented`) are batched.
- **Batches exist by hand.** #1576 (merged 2026-09-24) landed four Evidence-only outcomes with a
  body table of brief, verdict and rows; #1574 (5 briefs) and #1586 (14 briefs) are open and
  CONFLICTING, on the shared outcomes log like every other Evidence PR.
- **The queue.** 40 open verifier-App PRs on 2026-09-27; 5 are PASS landings that flip
  `implemented → verified`; the other 35 are Evidence-only (FAIL or BLOCKED), all batchable under
  the interim rule.
- **The writer.** `deskevidence` commits ONE file per call through the forge's contents API, each
  call a separate write against the outward-write budget. A one-brief landing is two or more
  commits; an N-brief batch done by hand is 2N or more.
- **Board derivation.** `singleBriefTrailer` (statusgen/ghfetch.go:256) derives a brief from a PR
  body with exactly one `Brief:` trailer; a body with more than one is `multi-linked` and derives
  nothing.
- **With desk-supervision/24 landed** each outcome is a new file, so a batch PR conflicts with no
  sibling on outcomes. Batching is then about PR, review and CI count, not conflicts. Without it a
  batch still goes CONFLICTING whenever any other Evidence PR lands. This brief therefore depends on
  24 and builds no batch path for the appended log.

layering: flat-tool extension of `deskevidence` plus one new `Forge` capability. The batch
decision (which entries are admitted, in what order, and why each excluded one was excluded) is a
pure function over the per-entry gate results and the combined-tree gate result, tested without a
forge. The forge write is a thin adapter; its atomicity is the backend's (one commit object, one
compare-and-swap ref update), proven by goldens.

risk cross-read: `tools/desk/internal/deskkit/` is a security-path trigger in this repository. The
answers stay no: `CommitFiles` writes with the same verifier identity and token the one-file path
uses, to a branch that still reaches main only through a reviewed PR; it adds no identity, scope or
bypass, and every existing per-file gate runs on every entry before it.

single-point-of-failure: not a core-system brief under the stream's definition. For the record: the
ONE control keeping a refused entry out of a batch is the per-entry preflight; behind it, the
combined-tree gate re-checks the admitted set as a whole, and the review of the batch PR reads every
entry.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. The deliverable is a draft PR
  opened by the desk verbs.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **`Forge.CommitFiles`.** Add a multi-file commit to the `Forge` interface: a list of path/content
   pairs, a message, the branch and the expected parent commit. GitHub: blobs, one tree, one commit,
   and a non-forced ref update that fails when the branch moved. GitLab: one commits-API call with one
   action per file and the expected parent. Either the whole commit lands or nothing does; a moved
   branch is a refusal, never a silent rebase.
2. **The batch verb.** `deskevidence batch <owner/repo> --manifest <file> [--root <dir>] [--dry-run]
   [--body-out <file>]`. The manifest lists entries, one per brief: its Evidence snippet and brief
   path, and its outcome record (desk-supervision/24's `--outcome-record` input). The verb:
   - (a) creates the batch branch `verify-desk/batch-<window-id>` server-side from the current main,
     and refuses (exit 5) when that branch already has an open PR: never append to a batch under
     review;
   - (b) preflights every entry alone against main with every gate the single-file path runs today
     (BodyCheck, scope, the statusgen PROBLEM diff, the verified-outcome gates, desk-supervision/24's
     receipt validation);
   - (c) excludes any entry that flips a status cell, with the reason "status flip: single-brief PR
     required";
   - (d) re-runs the PROBLEM diff over the combined post-image of the admitted set. If the set
     introduces a problem no single entry did, it admits greedily in manifest order, re-checking
     cumulatively, and excludes the entry that tips it, with the interaction named;
   - (e) writes every admitted entry in ONE `CommitFiles` commit;
   - (f) prints a result manifest (each entry `landed`, `refused: <reason>` or
     `could-not-check: <reason>`) and, with `--body-out`, a PR body: a table of brief, verdict, rows
     and record path per landed entry, a `Verify-Batch: <window-id>` line, and NO `Brief:` trailer.

   Exit 0 when at least one entry landed and every exclusion is reported; exit 5 when every entry was
   refused; exit 6 when the write itself could not be made (nothing landed).
3. **Refused entries never block.** An excluded entry is not lost and does not hold the batch. It is
   reported, and it stays in the verify desk's own queue for the next tick. A content refusal is also
   filed as a single refusal is today.
4. **Drop after review.** `deskevidence batch --drop <brief> --branch <batch branch>` restores that
   entry's files to main's blobs in one commit and leaves the others. A reviewer finding on one entry
   then costs that entry a window, not the whole batch its approval.
5. **Adopter values.** The window length (default 15 minutes) and the maximum entries per batch
   (default 10) are read from the desk's configuration, never hard-coded in the skill body. Entries
   over the cap are deferred to the next window and reported, never dropped.
6. **Skills.**
   - `plugins/assay/skills/verify-desk/SKILL.md`, the PR-required-main lane: Evidence-only outcomes land per tick
     through `deskevidence batch`; flips keep the single-brief lane. "Land-as-each-verdict-arrives"
     becomes "land within the tick", and a PASS held past its tick is still phantom verification
     debt.
   - `plugins/assay/skills/pr-review-desk/SKILL.md`: review a batch per entry against the body table, and ask for a
     `--drop` for an entry with a finding instead of blocking the batch.
   - Remove the just-in-time merge step's per-PR framing where it no longer applies.
   - Keep every skill body neutral: no values, no names.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestBatchIsolatesRefusedEntry$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b26-r1-1.out" 2>&1 && grep -F -e '--- PASS: TestBatchIsolatesRefusedEntry' "${TMPDIR:-/tmp}/b26-r1-1.out"` | exit 0; output contains `--- PASS: TestBatchIsolatesRefusedEntry`. Three entries, one refused by a gate: two land, the result manifest names the refused one and its gate, exit 0. Every entry refused: exit 5 and nothing written. One entry whose brief cannot be read: it is `could-not-check` and the other two land | check:ci +mutation |
| 2 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestBatchSingleAtomicCommit$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b26-r2-1.out" 2>&1 && grep -F -e '--- PASS: TestBatchSingleAtomicCommit' "${TMPDIR:-/tmp}/b26-r2-1.out"` | exit 0; output contains `--- PASS: TestBatchSingleAtomicCommit`. The stub forge sees exactly one commit carrying every admitted file; a branch that moved before the ref update gives exit 6 and no commit | check:ci +mutation |
| 3 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestBatchCombinedGate$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b26-r3-1.out" 2>&1 && grep -F -e '--- PASS: TestBatchCombinedGate' "${TMPDIR:-/tmp}/b26-r3-1.out"` | exit 0; output contains `--- PASS: TestBatchCombinedGate`. Two entries clean alone but introducing a lint problem together: the later one is excluded with the interaction named; the earlier one lands | check:ci +mutation |
| 4 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestBatchNoAppendUnderReview$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b26-r4-1.out" 2>&1 && grep -F -e '--- PASS: TestBatchNoAppendUnderReview' "${TMPDIR:-/tmp}/b26-r4-1.out"` | exit 0; output contains `--- PASS: TestBatchNoAppendUnderReview`. A batch branch with an open PR gives exit 5 and nothing written | check:ci +mutation |
| 5 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestBatchRefusesFlipEntry$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b26-r5-1.out" 2>&1 && grep -F -e '--- PASS: TestBatchRefusesFlipEntry' "${TMPDIR:-/tmp}/b26-r5-1.out"` | exit 0; output contains `--- PASS: TestBatchRefusesFlipEntry`. An entry that flips a status cell is excluded with "single-brief PR required"; the rest land | check:ci +mutation |
| 6 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestBatchDropEntry$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b26-r6-1.out" 2>&1 && grep -F -e '--- PASS: TestBatchDropEntry' "${TMPDIR:-/tmp}/b26-r6-1.out"` | exit 0; output contains `--- PASS: TestBatchDropEntry`. `--drop` restores exactly that entry's files to main's blobs in one commit; the other entries' files are unchanged | check:ci +mutation |
| 7 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestBatchWindowConfig$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b26-r7-1.out" 2>&1 && grep -F -e '--- PASS: TestBatchWindowConfig' "${TMPDIR:-/tmp}/b26-r7-1.out"` | exit 0; output contains `--- PASS: TestBatchWindowConfig`. Window and cap come from configuration; entries over the cap are reported deferred, not dropped | check:ci |
| 8 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestBatchBody$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b26-r8-1.out" 2>&1 && grep -F -e '--- PASS: TestBatchBody' "${TMPDIR:-/tmp}/b26-r8-1.out"` | exit 0; output contains `--- PASS: TestBatchBody`. The body lists every landed entry with its record path, carries the `Verify-Batch:` line, and no line of it starts with `Brief:` | check:ci |
| 9 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestCommitFiles$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b26-r9-1.out" 2>&1 && grep -F -e '--- PASS: TestCommitFiles' "${TMPDIR:-/tmp}/b26-r9-1.out"` | exit 0; output contains `--- PASS: TestCommitFiles`. GitHub and GitLab goldens: one commit with every file; a moved branch is refused with nothing written | check:ci +mutation |
| 10 | After the first real batch PR merges, with `BATCH_PR` exported as its number: `gh pr view "$BATCH_PR" -R medici-finance/assay --json files,reviews --jq '{files: (.files \| length), reviews: (.reviews \| length)}'` | one PR carrying k ≥ 2 entries' files, and a review count per lane that does not grow with k. If no batch has merged yet, record could-not-check with the date; never a pass | check +dereference +flow |
| 11 | `cd tools/skillslint && go run . --root ../..` | exit 0 | check:ci |
| 12 | `statusgen --consumers --root .` | exit 0 — every routing token above corroborated against the branch diff | check:ci +dereference |

Pre-mortem (failure mode → row):

| Failure mode | Caught by |
|---|---|
| One refused entry blocks the whole batch | row 1 |
| A refused entry lands anyway, or disappears without a report | row 1 (result manifest), Task step 3 |
| A half-written batch (some files landed, some not) | rows 2 and 9 (one commit or nothing) |
| Entries clean alone but broken together | row 3 |
| A batch grows under review and the approval covers less than lands | row 4 |
| A status flip rides a batch and the board cannot derive it | row 5 |
| One bad entry costs the whole batch its approval | row 6 (drop) |
| The window or cap is hard-coded | row 7 |
| Batching does not actually cut the review count | row 10 |
| Batches still conflict with each other | depends on desk-supervision/24 (its concurrency row); review-only here |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the
stream README table.
