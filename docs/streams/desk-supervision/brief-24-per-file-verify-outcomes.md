---
brief: assay:assay:desk-supervision:24
title: One file per verify outcome — retire the shared appended outcomes log
why: >-
  Every verify Evidence PR appends one line to one shared file, and the forge merges pull requests
  server-side without the `merge=union` driver that file relies on, so each Evidence landing turns
  every sibling Evidence PR CONFLICTING (19 of 40 open on 2026-09-27, the oldest ten days old). Each
  conflict costs a merge of main, a head move past the approval, and a re-review. Writing each
  outcome as its own new file means concurrent Evidence PRs never touch the same path, which ends
  the conflict class, removes the size cap that single file keeps outgrowing, and makes "latest
  outcome per brief" a timestamp read instead of a line-position read. The same writer also starts
  refusing the three receipt defects reviewers keep bouncing Evidence PRs for, so they are fixed once
  at the source instead of once per PR.
wave: 0
depends: []
unblocks: ["desk-supervision/25", "desk-supervision/26"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [882, 1338]
schema: brief-v2
version: 1
id: 7dc7ad21-b2f0-450e-8969-20b007e92331
authored: 2026-09-27 by worker-desk authoring session (#882, the per-file outcomes option)
exec-tier: strong
exec-tier-why: >-
  (b) correctness spans one writer, two reader modules in separate Go modules, a register the board
  generator must skip, a data migration and a transition window where both layouts are live; (c) a
  reader that silently drops or double-counts a record survives a happy-path test.
domain: complicated
sources:
  - "medici-finance/assay#882 — the conflict class: every verify Evidence PR appends to one shared log and goes CONFLICTING when a sibling lands"
  - "https://github.com/medici-finance/assay/issues/882#issuecomment-5857907228 — the driver's acceptance (own login, 2026-09-27) of the plan on #882: briefs desk-supervision/24-26 plus the verify-desk and pr-review-desk skill edits"
  - "option name: this brief carries 'per-file outcomes' (one file per verify outcome); its siblings are 'approval carry' (desk-supervision/25) and 'batched landing' (desk-supervision/26) — three alternatives for the conflict class #882 tracks"
  - "medici-finance/assay#882, comment of 2026-09-27 — recurring verify-wake-v1 receipt defects found in review of #1706-#1713 (inputs omit deliverables, free-text blocker_ref, brief hash that does not match the brief as it lands); the comment asks that the redesign fold writer-side validation into this brief"
  - "medici-finance/assay#1338 — the per-file size cap class on the shared log (raised to 4 MiB, rotation still owed)"
  - "medici-finance/assay#588 — the `merge=union` attribute (landed 2026-09-07 in 7aa97b7db)"
  - "docs/streams/fresh-views/brief-04-shared-append-only-log-discipline-verify-outcomes-jsonl-merge-union-deskevidence-post-write-sha.md — the prior remedy for this class; lineage in facts"
  - "docs/streams/desk-supervision/brief-23-evidence-lander-gatekeeper.md — its scope rules name the appended log; see consumers"
  - "live read 2026-09-27 ~16:30Z: open verifier-App PRs and their mergeability, and a local merge-tree of each CONFLICTING head against main (facts)"
  - "freshness-checked 2026-09-27 @ b227b4076 (origin/main): the log is a single appended file, no per-record layout exists, the writer and both readers read it by path"
consumers:
  # Authoring PR: each path this brief's implementation edited is routed to this brief
  # (rule 6). Flipped to fixed-here below for every path this PR actually touched; two stay
  # follow-up because Task step 8 (the log's retirement) is conditional and did NOT run this
  # tick — 17 open PRs still touch the log (medici-finance/assay#1802 tracks the retirement).
  - "tools/desk/internal/deskkit/verifyoutcomes.go: fixed-here (desk-supervision/24) — the reader/name choke point (RecordName, ParseRecord, ReadVerifyOutcomes, LatestPerBrief)"
  - "tools/desk/cmd/verifyloop/briefscan.go: fixed-here (desk-supervision/24) — readOutcomeSidecar/readWakeReceipts rewired onto the choke point via readVerifyOutcomeRecords, latest-by-timestamp"
  - "tools/desk/cmd/deskevidence (writer, size-cap override, verified-outcome gates): fixed-here (desk-supervision/24) — --outcome-record, the class guard, receipt validation; the size-cap override is DELETED, not migrated"
  - "statusgen/verifyoutcomes.go, statusgen/load.go and the new `statusgen outcomes` subcommand: fixed-here (desk-supervision/24) — statusgen's own reader/name copy, reservedRegisterNames, statusgen/outcomessplit.go"
  - "statusgen/verifyoutcomes_union_test.go (asserts merge=union on the log): follow-up desk-supervision/24 (this brief; the assertion retires with the log at Task step 8, which is CONDITIONAL and did not run this tick — medici-finance/assay#1802 tracks it — flips to fixed-here then)"
  - ".gitattributes (line 23, merge=union for the log): follow-up desk-supervision/24 (this brief; removed at the retirement step, Task step 8 — conditional, not run this tick, medici-finance/assay#1802 — flips to fixed-here then)"
  - ".gitleaks.toml (SR-1803-1/CR-1803-1 — the sibling `[[allowlists]]` block for the `#1803`/`#1804` per-outcome-record false-positive class): fixed-here (desk-supervision/24) — landed directly by the driver, own login, per the ruling on medici-finance/assay#1804"
  - "docs/streams/verify-outcomes.jsonl: out-of-scope, by design (Task step 7 leaves the log itself byte-for-byte unchanged — the split only ADDS record files under docs/streams/verify-outcomes/; every legacy line has its record file, verified by `statusgen outcomes split --check` exiting 0 on the merged tree rather than by a pinned count, since the log keeps growing while this PR is open). The DELETE half (Task step 8) stays follow-up desk-supervision/24, conditional on medici-finance/assay#1802"
  - "docs/streams/desk-supervision/verify-wake-v1.md and repair-obligation-v1.md (name the appended log as the receipt's home): fixed-here (desk-supervision/24)"
  - "plugins/assay/skills/verify-desk/SKILL.md (the outcome-row paragraph and the Evidence-PR state table's premise): fixed-here (desk-supervision/24)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md (the Evidence-PR re-conflict bullet): fixed-here (desk-supervision/24)"
  - "tools/desk/README.md (deskevidence and verifyloop sections): fixed-here (desk-supervision/24)"
  - "migrations/ (a new adopter migration note for the split): fixed-here (desk-supervision/24) — migrations/0004-v1.0.27-to-v1.0.28-per-file-verify-outcomes.md"
  - "docs/streams/desk-supervision/brief-23-evidence-lander-gatekeeper.md (its scope core admits appended lines of the log and its remerge relies on merge=union): fixed-here (desk-supervision/24) — Task step 9's amendment, brief still todo, version bumped 1 -> 2"
  - ".github/workflows/evidence-automerge.yml: out-of-scope (it admits a verifier-App PR whose every file is under docs/streams/; the record directory sits under docs/streams/, so the lane admits a per-file Evidence PR unchanged — Verify row 10 observes a real one)"
  - "tools/desk/cmd/deskboard: out-of-scope (reads no outcome record; its MERGE-CURR versus RE-REVIEW split changes only as a consequence, because sibling Evidence PRs stop sharing a file)"
  - "tools/desk/cmd/fanoutloop/repair.go and tools/desk/cmd/deskdispatch/repairadmission.go (the repair-obligations.jsonl projection): out-of-scope (same defect class, but its real sink is not armed: verifyloop/repair.go writes a dry-run only. Task step 5's class guard refuses any write to an appended docs/streams/*.jsonl log, so arming that sink as a shared appended file fails red and forces this layout at that time)"
  - "docs/streams/fresh-views/brief-04-shared-append-only-log-discipline-verify-outcomes-jsonl-merge-union-deskevidence-post-write-sha.md: out-of-scope (its #806 half, reporting the forge-returned sha, is independent of the storage layout and stays with it)"
  - "medici-finance/assay#1420 (verifyloop re-lists when no outcome record exists): out-of-scope (a queue-derivation defect independent of layout; this brief keeps 'no record for a brief' meaning exactly what it means today, so that fix applies unchanged to the new reader)"
  - "adopter trees outside this repository that hold their own log: out-of-scope (reached through the migration note of Task step 7, run by each adopter)"
---

# Brief 24 — One file per verify outcome

## Context

files:
- **add** `tools/desk/internal/deskkit/verifyoutcomes.go` (planned) + `verifyoutcomes_test.go` (planned)
  — the desk-side reader choke point and the record-name function.
- **edit** `tools/desk/cmd/verifyloop/briefscan.go` — `readOutcomeSidecar` (line 240) and
  `readWakeReceipts` (line 384) read through the choke point.
- **edit** `tools/desk/cmd/deskevidence/deskevidence.go`, `main.go`, `outcome.go`,
  `verifiedgate.go` + tests — the per-record writer, the class guard, the cap override retired.
- **add** `tools/desk/cmd/deskevidence/concurrency_test.go` (planned) — the mergeability proxy test.
- **edit** `statusgen/verifyoutcomes.go`, `statusgen/load.go` (line 64, `reservedRegisterNames`),
  `statusgen/main.go` (subcommand dispatch); **add** `statusgen/outcomessplit.go` (planned) +
  `statusgen/outcomessplit_test.go` (planned).
- **add** `docs/streams/verify-outcomes/<stream>/*.json` (the split records) and
  `docs/streams/verify-outcomes/README.md` (planned) (declares "register, not a stream").
- **edit** `docs/streams/desk-supervision/verify-wake-v1.md`, `repair-obligation-v1.md`,
  `plugins/assay/skills/verify-desk/SKILL.md`, `plugins/assay/skills/pr-review-desk/SKILL.md`,
  `tools/desk/README.md`.
- **add** `migrations/0004-<from>-to-<to>-per-file-verify-outcomes.md` (planned; the version span is
  the release that carries this change).
- **edit at retirement (Task step 8)** `.gitattributes`, `statusgen/verifyoutcomes_union_test.go`;
  **delete** `docs/streams/verify-outcomes.jsonl`.
- **add** `changelog/<branch>.md` — the fragment this repo enforces.

facts (2026-09-27 @ b227b4076; re-establish from the named files and commands at pickup):
- **Why the union driver does not help.** `.gitattributes:23` sets
  `docs/streams/verify-outcomes.jsonl merge=union`. The forge computes a PR's mergeability and
  performs its merge server-side, and applies no `.gitattributes` merge driver there. Live read
  2026-09-27: 40 open verifier-App PRs, 19 `CONFLICTING`, the oldest opened 2026-09-17. For each of
  the 19 heads, `git merge-tree --write-tree` against main is clean (exit 0) with the repository's
  attributes, and conflicts (exit 1) on exactly one path, `docs/streams/verify-outcomes.jsonl`,
  with attributes disabled (`git -c core.attributesFile=/dev/null --attr-source=<empty tree>
  merge-tree --write-tree`). The attribute-free local merge is therefore a faithful proxy for the
  forge's verdict on this conflict.
- **Lineage.** `merge=union` landed on 2026-09-07 (7aa97b7db, #588). fresh-views/04 (authored
  2026-09-16, still `todo`) planned the same attribute for #882; its freshness line says the
  attribute was absent at e9fa19d3, but 7aa97b7db is an ancestor of e9fa19d3 and the line is
  present there. What it fixed: local merges and `git merge` by a desk. What it did NOT fix: the
  forge's mergeability and merge of a PR, which is the path every Evidence PR takes on a
  PR-required main.
- **Readers are position-dependent.** `verifyloop`'s `readOutcomeSidecar` (briefscan.go:240) and
  `readWakeReceipts` (briefscan.go:384) keep the LAST line per brief ("the log is append-only, so
  the last line for a brief is its current outcome"). A union merge orders lines by merge order,
  not by time: the committed log (46 rows) is out of `ts` order at 7 places. No brief's current
  outcome is wrong today, by luck of which lines interleaved.
- **The other reader is prepared but unused.** `statusgen/verifyoutcomes.go` holds a
  rotation-aware union reader (glob `verify-outcomes*.jsonl`, line 26) written for #1338; `git
  grep` finds no non-test caller of `readVerifyOutcomesUnion` or `verifyOutcomesShardPaths`.
- **The writer.** `deskevidence` commits one file per call through the forge's contents API.
  `maxBytes` (deskevidence.go:16) is 256 KiB; the log gets its own 4 MiB override
  (`verifyOutcomesMaxBytes`, line 43) through `isVerifyOutcomesSidecar` (line 63), because a
  fleet-wide log reached 291722 bytes and every append was refused (#1338). `.jsonl` targets get
  `--append-only` (refuse a shrink) automatically. `guardVerifiedOutcomes` (outcome.go:23) and
  `gateVerifiedSidecarLanding` (verifiedgate.go:152) parse the ADDED lines of the log to gate
  `"outcome":"verified"` records on a lint-valid closure.
- **Board generation.** statusgen walks `docs/streams/*/` as streams and complains about a
  directory with no README unless its name is in `reservedRegisterNames` (load.go:64) or its README
  declares "register, not a stream". `verifyloop` skips a directory with no README.
- **The Evidence auto-merge lane** (`.github/workflows/evidence-automerge.yml`) admits a verifier-App
  PR whose every changed file is under `docs/streams/` and is not `STATUS.md` or
  `docs/streams/FINDINGS.md`.
- **Record shape today.** Each row is one JSON object: `ts` (RFC 3339, second precision, `Z`),
  `brief` (`<stream>/<NN>`), `outcome`, `sha`, and optional wake-receipt fields
  (verify-wake-v1). 46 rows, 38 briefs, no duplicate lines, no duplicate (`brief`, `ts`) pair.
- **Receipts carry three recurring defects** (#882 comment, 2026-09-27; `verify-wake-v1.md` §Fields).
  (a) `inputs` names only the brief and `tool`, not the deliverable files the "declared input scope"
  requires, so a fix to a deliverable never wakes the hold (blocking on #1710 and #1712; log lines
  24, 35, 40, 41, 43 and 44 have the same shape). (b) `blocker_ref` is free text ("desk to file", or
  an `action: …` sentence) where the field requires an issue, PR or action reference. (c) the
  brief's `file:` revision does not match the brief as it lands. The wake reader
  (`deskkit.RootRevisionReader.Revision`, verifywake.go:355-373) takes the SHA-256 of the declared
  file in the root's working tree after the merge, and that copy includes the Evidence PR's own
  append to the brief. A receipt that hashed the pre-Evidence brief (for example the copy at the
  receipt's `sha`) therefore fires `relevant-input-changed` the moment it lands (reproduced on
  #1709; #1712 hit the same class through a later edit on main). The receipt already landed for
  desk-supervision/04 hashes the brief as it landed. The skill now states all three as procedure;
  nothing in the writer checks them.
- **The forge's directory listing is bounded.** The contents API lists at most 1,000 entries per
  directory (GitHub REST documentation; re-check at pickup), so one flat directory of records would
  stop being listable by a forge-side reader long before it stopped being a valid git tree.

layering: flat-tool change across existing components, no new boundary. The one new piece of
logic worth isolating is the pure record layer in deskkit (name a record from its bytes; parse a
record; union the legacy log with record files and dedupe by digest; pick the latest per brief by
timestamp), tested with no forge and no git. `statusgen` keeps its own copy of the read half because
it is a separate Go module (the same constraint `verifyOutcomesGlobPattern`'s comment records); a
structural test in each module pins that module's single reader (Task step 6).

**The record layout (decided here).** One outcome is one new file:

```
docs/streams/verify-outcomes/<stream>/<NN>-<YYYYMMDDTHHMMSSZ>-<digest12>.json
```

- `<stream>` and `<NN>` come from the record's `brief` field, validated as
  `^[a-z0-9][a-z0-9-]*$` and `^[0-9]+$`; any other shape is refused, never sanitised.
- `<YYYYMMDDTHHMMSSZ>` is the record's `ts`, compacted, so a directory listing reads in time order.
- `<digest12>` is the first 12 hex digits of the SHA-256 of the file's exact bytes (the record's one
  JSON line plus a trailing newline).
- The content is exactly today's row, one line, unchanged schema. A migrated record's bytes equal
  its old log line plus `\n`, so the digest of a migrated file and of its source line agree.

Why this naming. It is collision-free across concurrent PRs: two PRs create the same path only when
they carry byte-identical content for the same brief at the same second, which is the same outcome,
and two identical adds merge cleanly. The digest, not the verified commit sha, is the uniqueness
term, because one brief is legitimately recorded twice at one merged sha (a re-issued outcome after
an Evidence correction note is in the log today). The per-stream directory keeps each directory
small enough for a forge listing (facts) and makes "every record for one brief" a single directory
read with a name prefix. Records are immutable: a correction is a new record, never an edit.

risk cross-read: `tools/desk/internal/deskkit/` is a security-path trigger in this repository. The
answers stay no: the new deskkit file is a read-side parser of outcome records, it adds no identity,
token or permission, and the writer keeps its existing identity and every existing gate.

single-point-of-failure: not a core-system brief under the stream's definition (no money, auth or
identity surface). For the record: the ONE control keeping a record from being lost or double-read
is the reader choke point; behind it sit the digest dedupe (a record present in both layouts is read
once) and the per-module structural test that fails when any other code opens the outcomes path.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions; the deliverable is a draft PR opened by the desk verbs.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Record layer (deskkit).** Add `RecordName(recordBytes) (path, error)`,
   `ParseRecord`, and `ReadVerifyOutcomes(root) ([]OutcomeRecord, error)`. `ReadVerifyOutcomes`
   reads every record file under `docs/streams/verify-outcomes/` AND every line of any legacy
   `docs/streams/verify-outcomes*.jsonl`, dedupes by the digest of the canonical bytes (trimmed line
   plus `\n`), and returns records with their source. Add `LatestPerBrief(records)`: the newest `ts`
   wins, ties broken by the lexically greatest record name. Line position is never an input. An
   absent directory and an absent log together are an empty set, not an error; an unreadable file is
   an error (could-not-check), never a skipped record.
2. **Readers.** Rewire `readOutcomeSidecar` and `readWakeReceipts` onto `ReadVerifyOutcomes` +
   `LatestPerBrief`, keeping their return shapes. Rewire `statusgen/verifyoutcomes.go` to the same
   rules (its own copy; separate module). Add `verify-outcomes` to `reservedRegisterNames` and add
   `docs/streams/verify-outcomes/README.md` (planned) declaring "register, not a stream". Then grep every
   walker of `docs/streams` in `statusgen/` and `tools/desk/` (`os.ReadDir` / `filepath.Glob` /
   `WalkDir` on that path) and confirm each skips the register; record the list in the PR body.
3. **Writer.** Add `deskevidence <owner/repo> <branch> --outcome-record <local-file> [--root <dir>]`.
   It reads one JSON object, computes the record path with `RecordName`, and commits a NEW file at
   that path. An existing file with identical bytes is a noop; an existing file with different bytes
   is refused (exit 5: records are immutable). The verified-outcome gates (`guardVerifiedOutcomes`,
   `gateVerifiedSidecarLanding`) apply to a record whose `outcome` is `verified` exactly as they apply
   to an added log line today; a `verify-fail` or `blocked` record is not gated, as today.
   **Receipt validation.** For a record carrying `wake_schema: verify-wake-v1`, the writer also
   refuses (exit 5, naming the field) when:
   - **(a)** `inputs` omits a declared deliverable. The declared set is every backticked
     repo-relative path in the brief's `## Context` `files:` block, read from the brief AT the
     record's `sha`. A path that is a file at that sha needs its own `file:<path>` key; a path that
     is a directory needs at least one `file:` key under it; a path absent at that sha (a `(planned)`
     file not yet written) is not required.
   - **(b)** `blocker_ref` is not a reference: it must match `#<N>`, `<owner>/<repo>#<N>`, or a forge
     URL to an issue, pull request or workflow run. A URL is parsed into owner, repository and
     number, and never fetched as a URL. The writer then reads that issue, PR or run only through the
     configured forge API. A number that does not exist is refused; a read that fails is
     could-not-check (exit 6), never a pass.
   - **(c)** the brief's own `file:` revision is not the SHA-256 of the brief AS IT LANDS: the copy
     on the target branch at write time, which already carries this landing's Evidence append. The
     writer reads that copy from the forge and compares. The outcome record is therefore the last
     write of a landing, after every edit to the brief on that branch. A later edit to the brief on
     the branch makes the record stale, and a re-issued record is the fix.

   An unreadable brief (at the record's `sha` for (a), or on the target branch for (c)) is exit 6. A record without `wake_schema` (a legacy-shape
   `verified` or `verify-fail` row) is not receipt-validated, as today.
4. **Retire the size-cap class.** Delete `verifyOutcomesMaxBytes`, `verifyOutcomesGlobPattern` and
   `isVerifyOutcomesSidecar`'s cap use; a record is bounded by the general `maxBytes`, which a
   single outcome is nowhere near. #1338 is closed by this step.
5. **Class guard.** Name the defect class in the PR body: *a record that concurrent PRs each add is
   stored as lines appended to one shared file, so the forge's server-side merge (which applies no
   merge driver) conflicts every sibling.* `deskevidence` REFUSES (exit 5) any write to a file
   matching `docs/streams/*.jsonl`, naming this brief, so the class cannot reopen through the writer
   for this log or any sibling log (the repair-obligation projection included).
6. **Choke-point guard.** One structural test per module: walk the module's non-test Go files and fail
   naming any file other than the allow-listed reader (and, in tools/desk, the writer) that contains
   the literal `verify-outcomes`. Each test carries a committed positive-control fixture under
   `testdata/` with one planted direct reader it must flag.
7. **Migration.** Add `statusgen outcomes split --root <dir> [--check]`. It writes one record file per
   legacy log line (verbatim bytes plus `\n`), is idempotent (a second run writes nothing), leaves the
   log in place, and with `--check` exits 1 naming any line that has no record file. Run it on this
   repository and commit the records. The log stays, frozen: no writer appends to it (step 5), and
   the reader dedupes the copies. Add the adopter migration note under `migrations/` (an
   `ensure-line` into `docs/UPGRADING.txt` naming the command, in the shape of migration 0003):
   upgrade the desk tools, run the split, then retire the log once no open PR touches it.
8. **Retirement (conditional).** At merge time, read the open PRs touching the log
   (`gh pr list --state open --json number,files`). If none, run the split once more to pick up any
   late line, then in the same PR delete `docs/streams/verify-outcomes.jsonl`, delete
   `.gitattributes:23` and its comment block, and retire the merge=union assertion in
   `statusgen/verifyoutcomes_union_test.go`. If any open PR still touches the log, do NOT delete it
   (that would turn every such PR modify/delete CONFLICTING one more time); file a follow-up issue,
   `help wanted`, naming the step and the open PRs, and say so in the PR body. Either way record which
   branch was taken.
9. **Brief 23.** If desk-supervision/23 is not yet implemented, amend its outcome-log clauses in this
   PR: the admissible outcome change becomes "one NEW file under `docs/streams/verify-outcomes/`
   whose name `RecordName` reproduces from its bytes", and the `merge=union`-dependent remerge
   fixtures and live row 19 (c) are re-stated on that basis; bump its `version:`. If it is already
   implemented, make the same change in its scope core and audit instead.
10. **Docs and skills.** Update `verify-wake-v1.md`, including its `inputs` field wording ("revision
    observed at receipt time" becomes the brief's revision as it lands, Evidence append included, which
    is what the wake reader hashes), and `repair-obligation-v1.md` (where a receipt
    lives), `tools/desk/README.md` (deskevidence usage, verifyloop's read), and the skills:
    - `plugins/assay/skills/verify-desk/SKILL.md`: the outcome paragraph ("append one row to the append-only sidecar …")
      becomes "write one outcome record with `deskevidence --outcome-record`"; the description line
      naming the log names the record directory.
    - Both skills: the verify-desk Evidence-PR state table (PR lane step 4) and the pr-review-desk
      bullet "Evidence PRs re-conflict by design" say the shared log is why Evidence PRs re-conflict.
      Once this lands, rewrite both to say that sibling Evidence PRs no longer conflict on outcomes,
      keep the state table and its owners for any other conflict, and say that a CONFLICTING
      Evidence PR now means a real content conflict.
    - `plugins/assay/skills/verify-desk/SKILL.md`, "Writing a receipt: three fields reviewers keep bouncing": keep the
      three procedure rules and add that `deskevidence --outcome-record` now refuses a receipt that
      breaks any of them, naming the field.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestVerifyOutcomes$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b24-r1-1.out" 2>&1 && grep -F -e '--- PASS: TestVerifyOutcomes' "${TMPDIR:-/tmp}/b24-r1-1.out"` | exit 0; output contains `--- PASS: TestVerifyOutcomes`. Fixtures: records only; legacy log only; both, with every log line also present as a record (each outcome read once); a log whose last line for a brief carries an OLDER `ts` than an earlier line (`LatestPerBrief` returns the newer one; a last-line-wins reader fails this fixture); an unreadable record file (error, not a skip); neither layout present (empty, no error); a malformed `brief` (`../x/01`, `X/1`) refused by `RecordName` | check:ci +mutation |
| 2 | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestOutcomeRecords$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b24-r2-1.out" 2>&1 && grep -F -e '--- PASS: TestOutcomeRecords' "${TMPDIR:-/tmp}/b24-r2-1.out"` | exit 0; output contains `--- PASS: TestOutcomeRecords`. One fixture tree read three ways (log only, records only, both): identical stuck-flip bucket and identical wake state for every brief | check:ci +flow |
| 3 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecordWrite$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b24-r3-1.out" 2>&1 && grep -F -e '--- PASS: TestOutcomeRecordWrite' "${TMPDIR:-/tmp}/b24-r3-1.out"` | exit 0; output contains `--- PASS: TestOutcomeRecordWrite`. The record lands at the path `RecordName` gives; identical re-write is noop; a different-bytes write to an existing record path exits 5; a `verified` record whose closure is not landed exits 5 exactly as a log line did; a `verify-fail` record is not gated | check:ci +mutation |
| 4 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestAppendedLogWriteRefused$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b24-r4-1.out" 2>&1 && grep -F -e '--- PASS: TestAppendedLogWriteRefused' "${TMPDIR:-/tmp}/b24-r4-1.out"` | exit 0; output contains `--- PASS: TestAppendedLogWriteRefused`. A write to `docs/streams/verify-outcomes.jsonl` exits 5 naming #882; so does a write to a planted second log `docs/streams/example-log.jsonl` | check:ci +mutation |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestVerifyOutcomesSingleReader$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b24-r5-1.out" 2>&1 && grep -F -e '--- PASS: TestVerifyOutcomesSingleReader' "${TMPDIR:-/tmp}/b24-r5-1.out" && cd ../../statusgen && go test . -run '^TestVerifyOutcomesSingleReader$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b24-r5-2.out" 2>&1 && grep -F -e '--- PASS: TestVerifyOutcomesSingleReader' "${TMPDIR:-/tmp}/b24-r5-2.out"` | exit 0; output contains `--- PASS: TestVerifyOutcomesSingleReader`; output contains `--- PASS: TestVerifyOutcomesSingleReader`. Each test passes on the tree AND flags its committed positive-control fixture (one planted direct reader) | check:ci +mutation |
| 6 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecordsConcurrentLandingsMergeable$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b24-r6-1.out" 2>&1 && grep -F -e '--- PASS: TestOutcomeRecordsConcurrentLandingsMergeable' "${TMPDIR:-/tmp}/b24-r6-1.out"` | exit 0; output contains `--- PASS: TestOutcomeRecordsConcurrentLandingsMergeable`. In a scratch repository with NO `.gitattributes`: branches A and B are cut from one main; each adds one outcome record AND one Evidence row to a different brief file; A is merged into main; then `git -c core.attributesFile=/dev/null --attr-source=<empty tree> merge-tree --write-tree main B` exits 0 (B stays mergeable with no driver). Negative control in the same test: the same scenario where A and B each append a line to one shared log exits 1, so the proxy can fail | check:ci +mutation +flow |
| 7 | `cd statusgen && go test . -run '^TestOutcomesSplit$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b24-r7-1.out" 2>&1 && grep -F -e '--- PASS: TestOutcomesSplit' "${TMPDIR:-/tmp}/b24-r7-1.out"` | exit 0; output contains `--- PASS: TestOutcomesSplit`. Split of a fixture log writes one file per line with bytes equal to the line plus `\n`; a second run writes nothing; `--check` exits 1 naming a line whose record was deleted | check:ci +mutation |
| 8 | `(cd statusgen && go build -o /tmp/statusgen-24 .) && /tmp/statusgen-24 outcomes split --root . --check` | exit 0 on merged main: every legacy line (if the log still exists) has its record file; if step 8 retired the log, exit 0 with nothing to check | check +dereference |
| 9 | `(cd statusgen && go build -o /tmp/statusgen-24 .) && /tmp/statusgen-24 --root . --lint` | exit 0; output contains `LINT: PASS` and no line naming `verify-outcomes` as a stream without a README | check:ci |
| 10 | After the first two Evidence PRs written with `--outcome-record` were open together and one merged, with `SIBLING_PR` exported as the number of the one still open: `gh pr view "$SIBLING_PR" -R medici-finance/assay --json mergeable --jq .mergeable` | `MERGEABLE`. If no such pair has existed yet, record could-not-check with the date; never a pass | check +dereference +flow |
| 11 | `! grep -rn 'verifyOutcomesMaxBytes' tools/desk/cmd/deskevidence/` | exit 0 (the override is gone; a record rides the general cap) | check:ci |
| 12 | `statusgen --consumers --root .` | exit 0 — every routing token above corroborated against the branch diff | check:ci +dereference |
| 13 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestReceiptInputsCoverDeliverables$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b24-r13-1.out" 2>&1 && grep -F -e '--- PASS: TestReceiptInputsCoverDeliverables' "${TMPDIR:-/tmp}/b24-r13-1.out"` | exit 0; output contains `--- PASS: TestReceiptInputsCoverDeliverables`. A receipt whose `inputs` holds only the brief and `tool` for a brief whose `files:` names two existing files exits 5 naming the missing path; one key per file passes; a directory entry needs one `file:` key under it; a `(planned)` path absent at the record's `sha` is not required; the brief is read at the record's `sha`, not the working tree (a fixture whose working-tree brief lists a different file than the brief at `sha` is judged by the latter) | check:ci +mutation |
| 14 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestReceiptBlockerRef$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b24-r14-1.out" 2>&1 && grep -F -e '--- PASS: TestReceiptBlockerRef' "${TMPDIR:-/tmp}/b24-r14-1.out"` | exit 0; output contains `--- PASS: TestReceiptBlockerRef`. `desk to file`, an `action: …` sentence and an empty value each exit 5; `#1546`, `example-org/example#12` and an issue URL pass when the stubbed forge has them; a well-formed `#999999` the forge does not have exits 5; a forge read error exits 6 | check:ci +mutation |
| 15 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestReceiptBriefHashAsLanded$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b24-r15-1.out" 2>&1 && grep -F -e '--- PASS: TestReceiptBriefHashAsLanded' "${TMPDIR:-/tmp}/b24-r15-1.out"` | exit 0; output contains `--- PASS: TestReceiptBriefHashAsLanded`. A brief revision equal to the pre-Evidence copy (the brief at the record's `sha`) exits 5, naming the path and both hashes; the hash of the target branch's copy, which carries the Evidence append, passes; a branch copy that cannot be read exits 6. A second fixture pins the reader side: `RootRevisionReader.Revision` over the merged tree returns the as-landed hash, so a receipt the writer accepted holds (does not fire) right after landing | check:ci +mutation |

Pre-mortem (failure mode → row):

| Failure mode | Caught by |
|---|---|
| Per-file records still conflict on the forge (two PRs pick the same path, or another shared file is hit) | row 6 (driver-free proxy), row 10 (a real pair) |
| The proxy itself cannot fail, so row 6 is green by construction | row 6 negative control |
| A record present in both layouts is counted twice during the transition | row 1 (both-layouts fixture), row 2 |
| "Latest outcome" still depends on position | row 1 (older-last fixture) |
| Some reader still opens the log by path and goes blind after retirement | row 5 (both modules), plus the Task step 2 walker list for directory walkers |
| The board generator treats the new directory as a broken stream | row 9 |
| A writer or a new sink appends to a shared `.jsonl` again | row 4 (planted second log) |
| The migration loses or rewrites a row | row 7 (verbatim bytes), row 8 (every line has its record) |
| Retirement deletes the log under open Evidence PRs and re-conflicts them all | Task step 8's open-PR read; review-only (the PR body records which branch was taken) |
| Brief 23 lands with a scope core that rejects per-file records | Task step 9; review-only (a human-gated brief's amendment is read by its gate) |
| A receipt still lands with a narrow `inputs`, a free-text `blocker_ref`, or a brief hash that is not the brief as it lands | rows 13, 14, 15 |
| The deliverable check reads the brief from the wrong tree and passes a receipt it should refuse | row 13 (working-tree-versus-`sha` fixture) |
| The writer's as-landed hash and the wake reader's hash disagree, so an accepted receipt still fires on landing | row 15 (the reader-side fixture over the merged tree) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->
### Non-implementer verifier run — VERIFY: PASS on behaviour, held — 1/15 witness-clear, rows 1–7 and 9–15 held — 2026-09-30 claude-opus-5-5-verifier

Runner is not the implementer. Isolated worktree at merged main `b89b3957225e227e69d5b5ec7949344f580d9966` (HEAD == the forge's `commits/main`, cross-checked), host darwin/arm64, go1.27.1; tree clean before and after. Implementing change: #1803 (squash c248703b0). `gate: model`, all four risk answers `no`. Status stays `implemented`: 13 rows are `check:ci` and their network-off witness cannot run on a darwin host (#1800); row 10's literal command can no longer print `MERGEABLE` (every qualifying sibling pair has merged) and row 12's literal `--consumers` run corroborates nothing on merged main (#1281 class). Every row's direct run passed.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | Verify row 1 command verbatim (deskkit verify-outcomes reader test) | named PASS | HELD (#1800) — direct exit 0; `--- PASS: TestVerifyOutcomes`; 7 subtests pass | 2026-09-30 | claude-opus-5-5-verifier |
| 2 | Verify row 2 command verbatim (verifyloop outcome-records test) | named PASS | HELD (#1800) — direct exit 0; `--- PASS: TestOutcomeRecords` (log only / records only / both) | 2026-09-30 | claude-opus-5-5-verifier |
| 3 | Verify row 3 command verbatim (deskevidence outcome-record write test) | named PASS | HELD (#1800) — direct exit 0; `--- PASS: TestOutcomeRecordWrite`; 7 subtests incl. noop re-write, different-bytes refused, clock skew | 2026-09-30 | claude-opus-5-5-verifier |
| 4 | Verify row 4 command verbatim (appended-log write refused) | named PASS | HELD (#1800) — direct exit 0; `--- PASS` for Test Appended Log Write Refused; the legacy log write exits 5 naming #882 | 2026-09-30 | claude-opus-5-5-verifier |
| 5 | Verify row 5 command verbatim (single-reader chokepoint, deskkit then statusgen) | named PASS ×2 | HELD (#1800) — direct exit 0 in both modules; each carries a planted-reader positive control | 2026-09-30 | claude-opus-5-5-verifier |
| 6 | Verify row 6 command verbatim (concurrent landings mergeable) | named PASS | HELD (#1800) — direct exit 0; per-file records stay mergeable with no merge driver; negative control (shared log) conflicts | 2026-09-30 | claude-opus-5-5-verifier |
| 7 | Verify row 7 command verbatim (statusgen outcomes split test) | named PASS | HELD (#1800) — direct exit 0; `--- PASS: TestOutcomesSplit` | 2026-09-30 | claude-opus-5-5-verifier |
| 8 | `(cd statusgen && go build -o /tmp/statusgen-24 .) && /tmp/statusgen-24 outcomes split --root . --check` | exit 0 | PASS — exit 0 (output path in a scratch dir); `outcomes split --check: every legacy line has its record file` (log 96 lines, 110 record files) | 2026-09-30 | claude-opus-5-5-verifier |
| 9 | `/tmp/statusgen-24 --root . --lint` | `LINT: PASS`; verify-outcomes not a stream | HELD (#1800) — direct exit 0, `LINT: PASS`; no line names verify-outcomes as a stream; brief-24 NOTICEs only | 2026-09-30 | claude-opus-5-5-verifier |
| 10 | `SIBLING_PR=1715; gh pr view "$SIBLING_PR" -R medici-finance/assay --json mergeable --jq .mergeable` | `MERGEABLE` | HELD — literal now prints `UNKNOWN` (the PR merged). Dereferenced instead: #1742 and #1715 were both written with `--outcome-record` and open together; #1742 merged 16:17:52Z, #1715 merged 16:26:04Z with no new commit after 5 main commits under docs/streams/verify-outcomes/. Attribute-free `git merge-tree --write-tree` proxies for #1715, #1765 and #1812 against their pre-merge main all exit 0 | 2026-09-30 | claude-opus-5-5-verifier |
| 11 | `! grep -rn 'verifyOutcomesMaxBytes' tools/desk/cmd/deskevidence/` | exit 0 | HELD (#1800) — direct exit 0, no matches | 2026-09-30 | claude-opus-5-5-verifier |
| 12 | `statusgen --consumers --root .` | corroborated | HELD (#1281) — literal on merged main: `no brief files in the diff … nothing to corroborate`. At the implementing commit's parent (`--base b7899516cb45 --brief desk-supervision/24`): `13 corroborated, 0 disproved, 7 unchecked` (all 7 out-of-scope judgements) | 2026-09-30 | claude-opus-5-5-verifier |
| 13 | Verify row 13 command verbatim (receipt inputs cover deliverables) | named PASS | HELD (#1800) — direct exit 0; 5 subtests incl. directory entry, planned path absent, brief read at the record's sha | 2026-09-30 | claude-opus-5-5-verifier |
| 14 | Verify row 14 command verbatim (receipt blocker ref) | named PASS | HELD (#1800) — direct exit 0; free text / sentence / empty exit 5; `#N`, `owner/repo#N`, URL pass; `#999999` exits 5; forge read error exits 6 | 2026-09-30 | claude-opus-5-5-verifier |
| 15 | Verify row 15 command verbatim (receipt brief hash as landed) | named PASS | HELD (#1800) — direct exit 0; 4 subtests incl. the reader-side revision check returning the as-landed hash | 2026-09-30 | claude-opus-5-5-verifier |

RISK-VALUE (trigger fires on the tools/desk/internal/deskkit/ path; none irreversible):

- RISK-VALUE: DERIVED — `RecordDigest` truncation = 12 hex @ tools/desk/internal/deskkit/verifyoutcomes.go:111 — the brief pins "first 12 hex digits"; a path collision needs the same brief, the same second and a 48-bit match, and the writer refuses rather than overwrites. The reader dedupes on the same 48 bits across all briefs: birthday bound ~1.8e-7 at 10^4 records.
- RISK-VALUE: DERIVED — `outcomeStreamRe` / `outcomeNumRe` @ tools/desk/internal/deskkit/verifyoutcomes.go:54-55 — the brief's own patterns verbatim; no `/` or `.` admitted, so a record path cannot leave its stream dir; `UnderOutcomeRecordsDir` re-checks independently.
- RISK-VALUE: DERIVED — ts compaction `20060102T150405Z` @ tools/desk/internal/deskkit/verifyoutcomes.go:164 — lossless for second-precision RFC 3339 Z and lexically time-ordered, as the brief requires.

Findings: (a) `MaxClockSkew` / future-ts exclusion and `UnderOutcomeRecordsDir` exceed the Task list but trace to the #1803 security review — not a defect. (b) Advisory: `ReadVerifyOutcomes` dedupes on the 12-hex digest globally; keying on the full digest (or brief + digest) would close the negligible collision drop at no cost. (c) Task step 4 says #1338 closes with this step; #1338 is still open — the code half is satisfied (row 11, c248703b0). (d) Task step 8's retirement was not taken; the log and `.gitattributes` union line remain, tracked by #1802.

### 2026-10-02 desk dispatch — Non-implementer verifier re-run — 2026-10-02 23:00 UTC — VERIFY: BLOCKED (15/15 rows pass by hand; witness 1/15)

Run at: 2026-10-02T23:00:03Z (UTC). Merged main `e1d99484ffd9`, detached worktree, clean before and after. darwin/arm64, go1.27.1, git 2.56.0, throwaway HOME. The runner is not the implementer. Implementing change: #1803 (squash c248703b0). `gate: model`, all four risk answers `no`. Status stays `implemented`. The previous record (2026-09-30 at `b89b3957225e`) was `blocked` on #1800. Changes since then: row 10 now has a live qualifying pair and its literal command prints `MERGEABLE`. Rows 10 and 12 are now tracked as check-definition items on #1927. The witness image's git version (row 6) is tracked on #1926. The brief's code paths are unchanged apart from the outbound-check refactor at the deskevidence write seam (#1919, #2024). Every test row was re-run green.

| # | Command | Exit | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | Verify row 1 command verbatim (deskkit verify-outcomes reader test) | 0 | named PASS line printed. 7 subtests pass: records only; legacy log only; both layouts read once; older-ts-last returns the newer; unreadable file is an error; neither layout is empty; malformed brief refused | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | Verify row 2 command verbatim (verifyloop outcome-records test) | 0 | named PASS line printed. Subtests log only / records only / both all pass | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | Verify row 3 command verbatim (deskevidence outcome-record write test) | 0 | named PASS line printed. 7 subtests pass: lands at the RecordName path; identical re-write is a noop; different bytes refused; unlanded verified closure exits 5; verify-fail not gated; two clock-skew cases | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | Verify row 4 command verbatim (appended-log write refused) | 0 | named PASS line printed. The legacy log write exits 5 naming #882. A planted second log under docs/streams also refuses | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | Verify row 5 command verbatim (single-reader choke point, deskkit then statusgen) | 0 | named PASS line printed by both modules | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | Verify row 6 command verbatim (concurrent landings stay mergeable) | 0 | named PASS line printed. Per-file subtest passes. Negative-control subtest (shared log, no driver) passes on git 2.56 | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | Verify row 7 command verbatim (statusgen outcomes split test) | 0 | named PASS line printed. Subtests: bytes equal line plus newline; second run writes nothing; check names a deleted record | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | Verify row 8 command verbatim (build statusgen, then `outcomes split --root . --check`) | 0 | `outcomes split --check: every legacy line has its record file`. The log has 96 lines; 125 record files exist | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 9 | Verify row 9 command verbatim (build statusgen, then `--root . --lint`) | 0 | `LINT: PASS`. No line names verify-outcomes as a stream without a README. Brief-24 lines are NOTICEs only (risk-files cross-read, row portability, outcome-absent) | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 10 | `gh pr view "$SIBLING_PR" -R medici-finance/assay --json mergeable --jq .mergeable` with SIBLING_PR=2056, then 2052 | 0 | `MERGEABLE` for both. #2053 (outcome-record PR) merged at 21:50Z while #2056 (outcome-record PR, opened 11:32Z) and #2052 (opened 10:17Z) were open. Both remain MERGEABLE with no conflict on outcomes. The literal pair in the Expect cell ("the first two") merged earlier; this is a later qualifying pair | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 11 | `! grep -rn 'verifyOutcomesMaxBytes' tools/desk/cmd/deskevidence/` | 0 | no matches | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 12 | `statusgen --consumers --root .` | 0 | Merged main prints `consumers: no brief files in the diff against e1d99484ffd9 — nothing to corroborate`, which corroborates nothing (#1281, #1927). A scratch clone at the implementing commit with `--base` its parent and `--brief desk-supervision/24` gives `13 corroborated, 0 disproved, 7 unchecked`; every unchecked entry is an out-of-scope judgement. The authoring commit (613d930dc) against its parent gives `13 corroborated, 0 disproved, 6 unchecked` | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 13 | Verify row 13 command verbatim (receipt inputs cover deliverables) | 0 | named PASS line printed. 5 subtests pass: missing path exits 5; one key per file passes; planned path absent; directory entry needs a key under it; brief read at the record sha | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 14 | Verify row 14 command verbatim (receipt blocker ref) | 0 | named PASS line printed. Free text, action sentence and empty value each exit 5. `#N`, `owner/repo#N` and URL pass. `#999999` exits 5. A forge read error exits 6 | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 15 | Verify row 15 command verbatim (receipt brief hash as landed) | 0 | named PASS line printed. 4 subtests pass, including the reader-side check that the revision returns the as-landed hash | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |

**Execution witness** (`statusgen verifyrun --brief <brief> --dry-run`, exit 2): 1 of 15 rows proven (row 8 pass).
- Rows 1–7, 9 and 11–15 are check:ci rows. On a darwin host they record could-not-run because there is no network-off sandbox (#1800).
- Row 10 records fail with exit 2. The witness takes the first code span in the command cell, `--outcome-record`, as the command, because the cell opens with prose. This is a check-definition defect (#1927).
- On the Linux witness, row 6 also depends on the image's git version (#1926).

**Mutation check** (scratch clone, never the verify worktree). Each mutation below turned its row red:
- last-line-wins in the latest-per-brief reduction: row 1
- reader skips the record directory: row 2
- immutable refusal removed: row 3
- class guard narrowed to the one legacy log: row 4
- planted direct reader in each module: row 5
- split drops the trailing newline: row 7
- a legacy line's record deleted: row 8 exits 1
- register README and reserved name removed: row 9 lint FAIL
- deliverable and directory omission allowed: row 13
- not-found and read-error tolerated: row 14
- hash mismatch tolerated: row 15

Row 11 exits 1 at the implementing commit's parent (4 matches).

Non-discriminating: row 6. Its test writes hand-picked record paths and never calls RecordName. A RecordName that collapses every record onto one shared path keeps rows 1, 3 and 6 green. Row 10's live pair partly covers this. Row 12's literal command is vacuous on merged main.

RISK-VALUE (the trigger fires on the deskkit security path; no entry is irreversible):

- RISK-VALUE: DERIVED — record digest truncation = 12 hex @ verifyoutcomes.go:111 (desk module), mirrored at statusgen verifyoutcomes.go:156. The brief pins "first 12 hex digits". A path collision needs the same brief, the same second and a 48-bit match, and the writer refuses rather than overwrites. Global dedupe birthday bound is about 1.8e-7 at 10^4 records.
- RISK-VALUE: DERIVED — brief key patterns `^[a-z0-9][a-z0-9-]*$` and `^[0-9]+$` @ verifyoutcomes.go:54-55 (desk), statusgen verifyoutcomes.go:111-112. These are the brief's own patterns verbatim. Neither admits `/` or `.`, so a record path cannot leave its stream directory.
- RISK-VALUE: DERIVED — ts compaction layout `20060102T150405Z` @ verifyoutcomes.go:164 (desk), statusgen verifyoutcomes.go:200. It is lossless for second-precision RFC 3339 `Z` timestamps and sorts lexically in time order, as the brief requires.
- RISK-VALUE: NAMED, NOT DERIVED — MaxClockSkew = 5 * time.Minute @ verifyoutcomes.go:277 (desk), copied by hand at statusgen verifyoutcomes.go:297. It came from the #1803 security review (SR-1803-2), not from the brief. No source derives 5 minutes against expected writer clock skew. It is a reversible operational knob: if too small, legitimate records are refused; if too large, a future-dated record shadows the real outcome for up to that window. The two copies are equal today but nothing ties them together.
- Pre-existing, unchanged: the general write cap `maxBytes` = 256 KiB (deskevidence.go:16). The 4 MiB log override was deleted, which row 11 confirms.

Open items still outstanding: #1338 is still open (its code half is met at c248703b0). The Task step 8 retirement was not taken (#1802).

VERIFY: BLOCKED — all 15 rows pass by hand at e1d99484ffd9, but the execution witness proves 1/15: 13 check:ci rows cannot run on a darwin host (#1800), rows 10 and 12 cannot pass as written (#1927), and row 6 also needs a newer git in the witness image (#1926); status stays implemented

### Non-implementer verifier re-run — VERIFY: BLOCKED — 2026-10-04 claude-opus-5-5-verifier

Run at 2026-10-04 09:10–09:23 UTC against merged main `70deba75a5775d50574695d2fb24efeb757c552f`, in a detached worktree that was clean before the run. Host: darwin/arm64, go1.27.1, git 2.56.0. The runner is not the implementer. Implementing change: #1803 (squash c248703b0). `gate: model`, all four risk answers `no`. Status stays `implemented`. The previous record (`blocked` at `e1d99484ffd9` on #1800) went stale.

Why this re-run: three declared inputs changed on main after `e1d99484ffd9`: the verify-desk skill, the pr-review-desk skill and the desk-tools README (10 commits, for example #2062, #2117, #2126, #2138, #2141 and #2135). None of them touches the per-file outcome text this brief verifies. The verify-desk diff changes the deskack wording, the comms cutover wording and the inline-code status rule. The pr-review-desk diff changes the scan-refusal STOP, deskack and comms. The README diff changes deskpr, deskroster, the roster beacon and comms. The brief-24 text is still in place: verify-desk describes the receipt refusal and the outcome-record paragraph, and says an Evidence PR on the per-file layout "no longer conflicts on outcomes". pr-review-desk says a CONFLICTING per-file Evidence PR "now means a real content conflict". The README keeps its `--outcome-record` section and the verifyloop read. The brief's code paths are byte-identical between `e1d99484ffd9` and `70deba75a577`: the deskevidence command, the deskkit verify-outcomes and verify-wake files, the verifyloop command, statusgen's verify-outcomes, outcomes-split and load files. The input change is therefore wake noise for this brief.

What changed in this run: the execution witness ran in a Linux container, so the check:ci rows no longer depend on the darwin host (#1800). Row 6's negative control runs on git 2.47, which accepts `--attr-source`. The exit-129 shape tracked on #1926 therefore does not apply to this witness. The witness proves 13 of 15 rows. Row 12 passes only vacuously on merged main (nothing to corroborate), so it is held, not proven (#1281 / #1927). Row 10 fails, because of the check-definition defect tracked on #1927.

| # | Command | Exit | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | Verify row 1 command verbatim (deskkit verify-outcomes reader test) | 0 | `--- PASS: TestVerifyOutcomes` and package `ok`. Subtests include neither layout present (empty, no error) and malformed brief refused by RecordName | 2026-10-04 | claude-opus-5-5-verifier · assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | Verify row 2 command verbatim (verifyloop outcome-records test) | 0 | `--- PASS: TestOutcomeRecords`. The log-only, records-only and both subtests pass | 2026-10-04 | claude-opus-5-5-verifier · assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | Verify row 3 command verbatim (deskevidence outcome-record write test) | 0 | `--- PASS: TestOutcomeRecordWrite`, including both clock-skew subtests | 2026-10-04 | claude-opus-5-5-verifier · assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | Verify row 4 command verbatim (appended-log write refused) | 0 | `--- PASS: TestAppendedLogWriteRefused`. A write to the legacy log exits 5 naming #882. A planted second log under docs/streams also refuses | 2026-10-04 | claude-opus-5-5-verifier · assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | Verify row 5 command verbatim (single-reader choke point, deskkit then statusgen) | 0 | `--- PASS: TestVerifyOutcomesSingleReader` printed by both modules | 2026-10-04 | claude-opus-5-5-verifier · assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | Verify row 6 command verbatim (concurrent landings stay mergeable) | 0 | `--- PASS: TestOutcomeRecordsConcurrentLandingsMergeable`. The per-file subtest (B stays mergeable with no merge driver) and the negative-control subtest (a shared log does conflict) both pass on git 2.56 | 2026-10-04 | claude-opus-5-5-verifier · assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | Verify row 7 command verbatim (statusgen outcomes split test) | 0 | `--- PASS: TestOutcomesSplit`. A second run writes nothing, and the check names a deleted record | 2026-10-04 | claude-opus-5-5-verifier · assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | Verify row 8 command, with the build output sent to a file under the worktree instead of /tmp | 0 | `outcomes split --check: every legacy line has its record file`. The log has 96 lines and there are 167 record files | 2026-10-04 | claude-opus-5-5-verifier · assay-verifier-app[bot] (on-behalf-of human:ian) |
| 9 | Verify row 9 command, using the same binary from row 8 | 0 | `LINT: PASS`. No line names verify-outcomes as a stream without a README. Brief-24 lines are NOTICEs only (risk-files cross-read, row-8 and row-9 portability, outcome-absent) | 2026-10-04 | claude-opus-5-5-verifier · assay-verifier-app[bot] (on-behalf-of human:ian) |
| 10 | `gh pr view "$SIBLING_PR" -R medici-finance/assay --json mergeable --jq .mergeable` with SIBLING_PR=2153 | 0 | `MERGEABLE`. #2153 (outcome-record PR, opened 07:28:41Z) and #2152 (outcome-record PR) were open together, and #2152 merged at 08:03:41Z. An attribute-free merge-tree of #2153's original head 7d8ec8dbbaa2 against main at #2152's merge commit b71720886484 exits 0. #2153 then merged main twice; its 08:47 head 7d9fc2fdb498 against main at #2068's merge commit b71b96ba1d5a also exits 0 attribute-free, and so does its current head b767ec9ce715 against 70deba75a577. The literal "first two" pair merged long ago; this is a later qualifying pair | 2026-10-04 | claude-opus-5-5-verifier · assay-verifier-app[bot] (on-behalf-of human:ian) |
| 11 | `! grep -rn 'verifyOutcomesMaxBytes' tools/desk/cmd/deskevidence/` | 0 | no matches | 2026-10-04 | claude-opus-5-5-verifier · assay-verifier-app[bot] (on-behalf-of human:ian) |
| 12 | `statusgen --consumers --root .` | 0 | On merged main: `consumers: no brief files in the diff against 70deba75a577… — nothing to corroborate`, which corroborates nothing (#1281, #1927). Dereferenced with `--base c248703b0^1 --brief desk-supervision/24`: `13 corroborated, 0 disproved, 7 unchecked`. Every unchecked entry is an out-of-scope judgement | 2026-10-04 | claude-opus-5-5-verifier · assay-verifier-app[bot] (on-behalf-of human:ian) |
| 13 | Verify row 13 command verbatim (receipt inputs cover deliverables) | 0 | `--- PASS: TestReceiptInputsCoverDeliverables`, including the directory-entry subtest and the subtest that reads the brief at the record's sha | 2026-10-04 | claude-opus-5-5-verifier · assay-verifier-app[bot] (on-behalf-of human:ian) |
| 14 | Verify row 14 command verbatim (receipt blocker ref) | 0 | `--- PASS: TestReceiptBlockerRef`. `#999999` exits 5, and a forge read error exits 6 | 2026-10-04 | claude-opus-5-5-verifier · assay-verifier-app[bot] (on-behalf-of human:ian) |
| 15 | Verify row 15 command verbatim (receipt brief hash as landed) | 0 | `--- PASS: TestReceiptBriefHashAsLanded`, including the subtest where an unreadable branch copy exits 6 and the reader-side subtest where the revision returns the as-landed hash | 2026-10-04 | claude-opus-5-5-verifier · assay-verifier-app[bot] (on-behalf-of human:ian) |

**Execution witness (Linux).** Command: `statusgen verifyrun --brief docs/streams/desk-supervision/brief-24-per-file-verify-outcomes.md --timeout 300s`, then `statusgen verifyrun --check` on the same brief. Re-run 2026-10-04 09:52:53–09:54:47 UTC. The statusgen binary was built inside the container from the clone's own source at `70deba75a577` (linux/arm64). It ran in the locally cached image golang:1.25-trixie (digest 2c4c60ef415f…, go1.25.14, git 2.47.3) with `--network none`, GOFLAGS=-count=1, GOPROXY=off, GOSUMDB=off, GOTOOLCHAIN=local and the host module cache mounted read-only. No image was pulled. Seccomp was unconfined so that statusgen's own `unshare --net --map-root-user` sandbox could start. The checkout was a throwaway clone, detached at `70deba75a577`, with origin/main set to that sha, and clean before the run. The only other mount was the house roster file, read-only. No credentials, tokens or PEMs were mounted. The container's git identity was set to the verifier App's name and noreply email (container-global, discarded with the container). The `(on-behalf-of human:ian)` annotation in the Runner cells below was rendered by statusgen itself from the mounted roster. The pinned harness image (desk-tools v1.0.29) is not cached on this host, so `--in-container` was not used. The darwin host witness is not carried here: there, rows 1–7, 9 and 11–15 could not run (#1800), row 8 passed and row 10 failed.

Correction: an earlier copy of this witness had the on-behalf-of annotation inserted by hand; this table is the unedited output of a re-run with the roster mounted read-only.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestVerifyOutcomes$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b24-r1-1.out" 2>&1 && grep -F -e '--- PASS: TestVerifyOutcomes' "${TMPDIR:-/tmp}/b24-r1-1.out"` | pass exit=0 | sha256:20fece53bd96 | 2026-10-04 | assay-verifier-app[bot] @ 70deba75a577 (on-behalf-of human:ian) (git-config) |
| 2 | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestOutcomeRecords$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b24-r2-1.out" 2>&1 && grep -F -e '--- PASS: TestOutcomeRecords' "${TMPDIR:-/tmp}/b24-r2-1.out"` | pass exit=0 | sha256:3fd5f8344033 | 2026-10-04 | assay-verifier-app[bot] @ 70deba75a577 (on-behalf-of human:ian) (git-config) |
| 3 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecordWrite$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b24-r3-1.out" 2>&1 && grep -F -e '--- PASS: TestOutcomeRecordWrite' "${TMPDIR:-/tmp}/b24-r3-1.out"` | pass exit=0 | sha256:102b90c9d3c3 | 2026-10-04 | assay-verifier-app[bot] @ 70deba75a577 (on-behalf-of human:ian) (git-config) |
| 4 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestAppendedLogWriteRefused$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b24-r4-1.out" 2>&1 && grep -F -e '--- PASS: TestAppendedLogWriteRefused' "${TMPDIR:-/tmp}/b24-r4-1.out"` | pass exit=0 | sha256:f14493a3eb2e | 2026-10-04 | assay-verifier-app[bot] @ 70deba75a577 (on-behalf-of human:ian) (git-config) |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestVerifyOutcomesSingleReader$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b24-r5-1.out" 2>&1 && grep -F -e '--- PASS: TestVerifyOutcomesSingleReader' "${TMPDIR:-/tmp}/b24-r5-1.out" && cd ../../statusgen && go test . -run '^TestVerifyOutcomesSingleReader$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b24-r5-2.out" 2>&1 && grep -F -e '--- PASS: TestVerifyOutcomesSingleReader' "${TMPDIR:-/tmp}/b24-r5-2.out"` | pass exit=0 | sha256:e3be20512864 | 2026-10-04 | assay-verifier-app[bot] @ 70deba75a577 (on-behalf-of human:ian) (git-config) |
| 6 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestOutcomeRecordsConcurrentLandingsMergeable$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b24-r6-1.out" 2>&1 && grep -F -e '--- PASS: TestOutcomeRecordsConcurrentLandingsMergeable' "${TMPDIR:-/tmp}/b24-r6-1.out"` | pass exit=0 | sha256:b2f53c50565a | 2026-10-04 | assay-verifier-app[bot] @ 70deba75a577 (on-behalf-of human:ian) (git-config) |
| 7 | `cd statusgen && go test . -run '^TestOutcomesSplit$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b24-r7-1.out" 2>&1 && grep -F -e '--- PASS: TestOutcomesSplit' "${TMPDIR:-/tmp}/b24-r7-1.out"` | pass exit=0 | sha256:deb95f05316c | 2026-10-04 | assay-verifier-app[bot] @ 70deba75a577 (on-behalf-of human:ian) (git-config) |
| 8 | `(cd statusgen && go build -o /tmp/statusgen-24 .) && /tmp/statusgen-24 outcomes split --root . --check` | pass exit=0 | sha256:8573ee90721d | 2026-10-04 | assay-verifier-app[bot] @ 70deba75a577 (on-behalf-of human:ian) (git-config) |
| 9 | `(cd statusgen && go build -o /tmp/statusgen-24 .) && /tmp/statusgen-24 --root . --lint` | pass exit=0 | sha256:f9f9c9a71d46 | 2026-10-04 | assay-verifier-app[bot] @ 70deba75a577 (on-behalf-of human:ian) (git-config) |
| 10 | `--outcome-record` | fail exit=2 | sha256:e2aee07e1822 | 2026-10-04 | assay-verifier-app[bot] @ 70deba75a577 (on-behalf-of human:ian) (git-config) |
| 11 | `! grep -rn 'verifyOutcomesMaxBytes' tools/desk/cmd/deskevidence/` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ 70deba75a577 (on-behalf-of human:ian) (git-config) |
| 12 | `statusgen --consumers --root .` | pass exit=0 | sha256:87dbbdb16019 | 2026-10-04 | assay-verifier-app[bot] @ 70deba75a577 (on-behalf-of human:ian) (git-config) |
| 13 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestReceiptInputsCoverDeliverables$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b24-r13-1.out" 2>&1 && grep -F -e '--- PASS: TestReceiptInputsCoverDeliverables' "${TMPDIR:-/tmp}/b24-r13-1.out"` | pass exit=0 | sha256:76ad37343ae4 | 2026-10-04 | assay-verifier-app[bot] @ 70deba75a577 (on-behalf-of human:ian) (git-config) |
| 14 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestReceiptBlockerRef$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b24-r14-1.out" 2>&1 && grep -F -e '--- PASS: TestReceiptBlockerRef' "${TMPDIR:-/tmp}/b24-r14-1.out"` | pass exit=0 | sha256:0a772b34d5a7 | 2026-10-04 | assay-verifier-app[bot] @ 70deba75a577 (on-behalf-of human:ian) (git-config) |
| 15 | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestReceiptBriefHashAsLanded$' -count=1 -v -timeout 180s > "${TMPDIR:-/tmp}/b24-r15-1.out" 2>&1 && grep -F -e '--- PASS: TestReceiptBriefHashAsLanded' "${TMPDIR:-/tmp}/b24-r15-1.out"` | pass exit=0 | sha256:f6181ef9f4ed | 2026-10-04 | assay-verifier-app[bot] @ 70deba75a577 (on-behalf-of human:ian) (git-config) |

**verifyrun --check summary** (Linux witness above, same container): `docs/streams/desk-supervision/brief-24-per-file-verify-outcomes.md: 14 pass, 1 fail, 0 could-not-run/missing (of 15 Verify rows)`, exit 1. Row 10 fails with exit 2 because the witness takes the first code span in the command cell, `--outcome-record`, as the command, since the cell opens with prose. This is the check-definition defect tracked on #1927. Row 12's witness pass is the vacuous merged-main run described in row 12 above, which is also on #1927.

RISK-VALUE (the trigger fires on the deskkit security path; no entry is irreversible; the literals and their line numbers are unchanged at `70deba75a577`):

- RISK-VALUE: DERIVED — record digest truncation = 12 hex @ tools/desk/internal/deskkit/verifyoutcomes.go:111, mirrored at statusgen/verifyoutcomes.go:156. The brief pins "first 12 hex digits". A path collision needs the same brief, the same second and a 48-bit match, and the writer refuses rather than overwrites.
- RISK-VALUE: DERIVED — `outcomeStreamRe` = `^[a-z0-9][a-z0-9-]*$` and `outcomeNumRe` = `^[0-9]+$` @ tools/desk/internal/deskkit/verifyoutcomes.go:54-55 and statusgen/verifyoutcomes.go:111-112. These are the brief's own patterns verbatim. Neither admits `/` or `.`, so a record path cannot leave its stream directory.
- RISK-VALUE: DERIVED — ts compaction layout = `20060102T150405Z` @ tools/desk/internal/deskkit/verifyoutcomes.go:164 and statusgen/verifyoutcomes.go:200. It is lossless for second-precision RFC 3339 `Z` timestamps and sorts lexically in time order.
- RISK-VALUE: NAMED, NOT DERIVED — `MaxClockSkew` = `5 * time.Minute` @ tools/desk/internal/deskkit/verifyoutcomes.go:277, hand-copied as `maxClockSkew` at statusgen/verifyoutcomes.go:297. It comes from the #1803 security review, not from the brief. No source derives it. It is a reversible operational knob, and nothing ties the two copies together.
- Pre-existing and unchanged: `maxBytes` = `256 * 1024` @ tools/desk/cmd/deskevidence/deskevidence.go:16. Row 11 confirms the 4 MiB log override is gone.

Open items still outstanding: #1338 is open (its code half is met at c248703b0). The Task step 8 retirement was not taken (#1802). #1800 is open but no longer gates this brief's witness, because the Linux container clears it.

VERIFY: BLOCKED — all 15 rows pass by hand at `70deba75a577`, and the Linux execution witness proves 13 of 15. Row 10's witness fails on the check-definition defect, and row 12 passes only vacuously on merged main. Both are tracked on medici-finance/assay#1927, which is still OPEN. Status stays implemented.

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the
stream README table.
