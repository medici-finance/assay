---
brief: assay:assay:statusgen:18
title: '`--scan-issues`: print a notice when an open issue derives `gate: human` and its placeholder reads `gate: model` (detect only, no write)'
why: >-
  The issue scanner decides once, when it first writes a placeholder, whether the item needs a
  human sign-off. Triage sets the `risk:high` label and the scanner does not wait for triage, so
  on the ordinary path the label arrives after the placeholder and the item keeps `gate: model`.
  Nothing reports that. The only cover is an instruction to the triage session, added by
  statusgen/17. #2405 asked whether the scanner should ever revisit a stored gate, and the
  maintainer answered with its option c, detect only. This brief builds that: the scan prints
  one notice for each such item and changes nothing. A missed item is then printed on every
  scan until a person raises the gate by hand, and the first run lists the placeholders that
  already exist. The desk's scan drain discards that output, so the line is read only by
  running the scan directly; a `--dry-run` scan prints it and writes nothing.
wave: 2
depends: ["statusgen/17"]
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [2405]
design: DR-gate-rederive
outcome: none
schema: brief-v2
version: 1
authored: 2026-10-08 by the-desk session (on behalf of the driver)
sources:
  - "#2405 — the question and its three options (a keep derive-once, b raise on every scan, c detect only). The maintainer's answer, in their own login, is comment 6070714706 (2026-10-08T22:58:41Z), whole text `c — DR-gate-rederive`"
  - "docs/streams/decisions/DR-gate-rederive.md — the design-decision record that transcribes that answer; this brief cites it through `design:`"
  - "statusgen/scanissues.go — `planScan`: the existing-placeholder skip this brief adds the comparison to, and the `notices` slice it returns"
  - "statusgen/placeholder.go — `derivePlaceholderGate` (what an issue derives) and `parsePlaceholderFile` (what a placeholder reads as)"
  - "statusgen/main.go — `emitNotices`: the notice convention this brief reuses"
  - "statusgen/17 — adds `risk:high` to the label set that derives `gate: human`, and the intake-desk sentence this brief extends"
  - "freshness-checked 2026-10-08 @ 979e5453f (origin/main) — `planScan` skips an existing placeholder without reading its gate; no Go file compares a stored gate with the issue's labels; statusgen/17 is `todo`, so `risk:high` is not yet in `riskGateLabels` and its skill bullet is not yet in the tree; no open PR touches `statusgen/scanissues.go`"
consumers:
  - "statusgen/scanissues.go: fixed-here (this brief; the comparison at the existing-placeholder skip and the notice it appends)"
  - "plugins/assay/skills/intake-desk/SKILL.md: fixed-here (this brief; two sentences appended to the scored-triage bullet statusgen/17 adds)"
  - ".cursor/skills/intake-desk/SKILL.md: fixed-here (canonical Cursor delivery of the intake-desk notice guidance)"
  - ".cursor/assay-managed.json: fixed-here (canonical manifest digest for the regenerated intake-desk skill)"
  - "tools/desk/cmd/scanloop/lane.go: out-of-scope (the drain runs `statusgen --root . --scan-issues` and discards that command's output when it succeeds, so it does not show the notice; relaying it is not part of the answer on #2405 and is left open, see Context)"
  - "statusgen/transcribescan.go: out-of-scope (the same-repo transcriber has the same skip and prints its own notices under its own prefix; it ships inert and no workflow in this repo runs it, and the answer on #2405 names the scan)"
---

# Brief 18 — notice for a placeholder gate the issue has outgrown

## Context

files:
- `statusgen/scanissues.go` — in `planScan`, the comparison at the existing-placeholder skip and the notice it appends.
- `statusgen/scanissues_test.go` — the two new tests named in Verify.
- `plugins/assay/skills/intake-desk/SKILL.md` — two sentences appended to one bullet in the scored-triage section.
- `.cursor/skills/intake-desk/SKILL.md` and `.cursor/assay-managed.json` — canonical generated Cursor delivery of the changed skill (Task 8).
- `docs/streams/statusgen/brief-18-gate-mismatch-notice.md` — this brief's consumer routings (Task 6).
- `changelog/feat-assay--statusgen--18.md` — the per-PR fragment this repo enforces.

single-point-of-failure: a person reading the notice and editing the placeholder. The notice
changes no gate, by the answer on #2405. One layer sits behind it and does not depend on the
notice being read: the instruction statusgen/17 adds, by which a triage session that scores an
item `risk:high` and finds `gate: model` labels the issue `help wanted` and names the file. It
fires once, at triage, on a session's reading; the notice fires on every scan, from the labels.
The PR-diff risk gate is not a layer for this fault. It keys on the changed paths and reads
neither the label nor the placeholder, so it stops such an item only when its paths happen to
trip it. One known hole is recorded, not closed: the desk's scan drain discards the scan's
output when the scan succeeds (see facts), so in that lane the notice is printed and not
shown.

facts (all read on main @ 979e5453f, 2026-10-08):
- **The decision.** #2405 put three options. The maintainer answered `c — DR-gate-rederive`
  (comment 6070714706). Option c as the issue wrote it: "detect only: the scan prints one notice
  per open issue whose labels and title derive `human` while its placeholder stores
  `gate: model`; the edit stays a hand edit". The record is
  `docs/streams/decisions/DR-gate-rederive.md`.
- **Where the scanner skips.** `planScan` in `statusgen/scanissues.go` walks each scanned
  repo's OPEN issues. For each one it computes `labels := labelNames(iss.Labels)`, skips the
  issue if a label is excluded, and then skips it if `existing[repo+"#"+number]` is set. That
  second skip is the site. At it the scanner holds the issue's labels and title and has not
  read the placeholder's gate.
- **What `existing` holds.** A set of `repo#issue` keys and nothing else. It is built by
  `existingPlaceholderIssues` from every stream's root placeholders (`s.Placeholders`, each a
  parsed `*Placeholder` with `Repo`, `Issue`, `Path`, `Status`, `Gate`), and then seeded with
  the keys of placeholders in each stream's `done/` archive. The skip site therefore needs a
  way to reach the root placeholder for a key; the archive seed can stay a bare key.
- **What an issue derives.** `derivePlaceholderGate(labels, title)` in
  `statusgen/placeholder.go` returns `human` or `model`. It is the same call, with the same
  arguments, that `planScan` makes a few lines later when it first writes a placeholder. Use
  that function; do not add a second rule. Once statusgen/17 lands it returns `human` for a
  `risk:high` label. It already returns `human` for a `funds` or `security` label and for the
  words `auth`, `funds` or `security` in the title or a label, plus any deployment additions.
  The notice follows all of them, as option c says ("labels and title").
- **What a placeholder reads as.** `parsePlaceholderFile` sets `Gate` from the file's `gate:`
  line when there is one. With no `gate:` line it derives the gate from the file's stored
  labels and an empty title. The scanner always writes the line, so the second case reaches
  only hand-written or older files. This brief compares against the parsed `Gate`, so a file
  with no `gate:` line that reads as `model` is reported and one that reads as `human` is not.
- **The notice convention.** `planScan` returns `notices []string`. `runScanIssues` appends
  them to the un-block notices and calls `emitNotices` (`statusgen/main.go`), which sorts them
  and prints each to stderr as `NOTICE: <text>`. Every scan notice text starts `scan-issues: `.
  `emitNotices` runs before the dry-run branch, so a notice prints with and without
  `--dry-run`. A notice never changes the exit code: `runScanIssues` returns non-zero only for
  a refused scan, a tree it cannot load, a partly unread scan or a failed write. This brief
  reuses that convention unchanged and adds no flag, no stdout line and no exit code.
- **Closed and excluded issues never reach the site.** The loop is over open issues, so a
  closed issue is not visited. An open issue carrying an excluded label takes the earlier
  `continue`. The same scan retires both placeholders in its close-out sweep.
- **A retired placeholder can belong to an open issue.** A placeholder with `status: done`
  whose issue is open and not excluded is reactivated by the same scan: rewritten to `todo`
  in place, or moved from `done/` back to the stream root. Its path can change during the run.
- **The desk drain does not show the notice.** `tools/desk/cmd/scanloop/lane.go` runs
  `statusgen --root . --scan-issues` through a step that discards the command's output, so
  after a successful scan nothing the scan printed is shown. No workflow in this repo runs
  `--scan-issues`. A person or session sees the notice by running the command directly;
  `--dry-run` prints it and writes nothing.
- **The notice carries no text from the issue.** It names the repo, the issue number and the
  placeholder's repo-relative path. It does not print the title or a label, so nothing an issue
  author wrote reaches a reader through it. An issue's author can edit its title and so cause
  a notice; the notice only ever asks for a stricter gate.

Out of scope. None of these is decided by this brief, by #2405's answer or by the record:
1. Changing a stored gate from the scan, in either direction. That is option b of #2405, which
   was not taken.
2. Editing any existing placeholder. The first run of the notice is the worklist for #2405's
   second question; each edit stays a hand edit, one item at a time.
3. A way to silence the notice for an item a person deliberately keeps at `gate: model`. Such
   an item is reported on every scan.
4. Showing the notice in the desk's scan drain, or anywhere other than the scan's stderr.
5. The same-repo transcriber (`--transcribe-scan`), which has the same skip.
6. Who may set or remove `risk:high`, and the keyword list. Both are as statusgen/17 left them.

design-fit:
  owner: `statusgen/scanissues.go` — `planScan`, which owns what a scan reports about an issue whose placeholder already exists
  contract: none — what a scan reports about an existing placeholder has no row in the semantic-owner index; one function owns it and the derivation it calls is `derivePlaceholderGate`, unchanged
  retires: []
  weight: verbs 0, flags 0, refusals 0, notices +1, rule-text lines +5 (two sentences in the intake-desk skill, as wrapped in Task 5)
  why-add: >-
    The notice goes into the owner, at the line that already holds both inputs, and reuses the
    existing notice path; no second mechanism is added. Adding nothing was option a of #2405
    and the answer ruled it out. Retiring the triage instruction from statusgen/17 to pay for
    the notice was considered and rejected: the instruction puts the item in front of a named
    person through a `help wanted` label, and the notice only prints a line that the desk
    drain does not show. The skill sentences are added because the bullet they extend is where a
    triage session reads what to do about a late `risk:high`.

## Ground rules
- NEVER push to main or trigger workflows by hand. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- Detect only. This change writes no file, plans no write and changes no exit code. Do not add
  a `scanPlan` or `closeOutPlan` for a mismatch, and do not touch `renderPlaceholder`,
  `applyCloseOut` or `parsePlaceholderFile`.
- Call `derivePlaceholderGate` with the issue's labels and title, exactly as the first-write
  path does. Do not add a `risk:high` check of your own and do not change the vocabulary.
- The notice text carries the repo, the issue number and the placeholder path. Never the title
  and never a label.
- Never suggest lowering a gate. A placeholder that reads `gate: human` produces no notice,
  whatever the issue derives.
- Add tests as new functions. Do not edit or loosen an assertion in an existing test. If an
  existing test goes red because its fixture now produces this notice, report it in the PR.
- This brief needs statusgen/17's change in the tree. If `"risk:high"` is not in
  `riskGateLabels`, or the skill bullet Task 5 extends is absent, report NEEDS_CONTEXT.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Fail-first: write the tests in steps 3 and 4 before step 2, run Verify row 2, and paste the
   red output (zero mismatch notices, want three) into the PR body under `## Fail-first`.
2. In `planScan`, at the existing-placeholder skip, before the `continue`: for each ROOT
   placeholder that carries this `repo#issue` key, append one notice when all three hold:
   - its `Status` is not `done`;
   - its parsed `Gate` is `model`;
   - `derivePlaceholderGate(labels, iss.Title)` is `human`.

   The notice text is exactly this, with the placeholder's path relative to the scan root and
   written with forward slashes (the same form `scanPlan.Rel` uses):

   ```
   scan-issues: <repo>#<N> derives gate: human but placeholder <rel> reads gate: model — the scan does not change it; set gate: human in the file by hand
   ```

   Give the skip site a lookup from the key to the root placeholders (a map built beside
   `existing` from `s.Placeholders` is enough). Leave the archive seed as a bare key: a
   placeholder in `done/` is never compared. A placeholder that is retired when the scan reads
   it is not reported in that scan, including when the same scan reactivates it; after the
   reactivation it is an active root placeholder and the next scan reports it. A `--dry-run`
   scan plans the reactivation and writes nothing, so it does not report such an item until a
   writing scan's reactivation is in the tree it reads. One line is
   appended per mismatching placeholder file. The scanner never creates a second placeholder
   for an issue, so in a tree it maintains that is one line per issue.
3. `TestScanGateMismatchNotice` (planned) in `statusgen/scanissues_test.go`, through `planScan`
   with `fixtureLister` and `blessAll` on a `scanFixtureRepo` root. Count the notices that
   contain `derives gate: human but placeholder`. Placeholders are at the stream root with
   `status: todo` unless stated, and titles are neutral unless stated:
   - stored `gate: model`, issue open with `[bug, risk:high]` → one notice, equal to the full
     text above with this repo, number and `docs/streams/alpha/<file>`;
   - stored `gate: model`, issue open with `[bug]` and the title `rotate the auth token cache`
     → one notice (the title derives `human`), and the notice does not contain the title;
   - no `gate:` line and stored labels `[bug]`, issue open with `[bug, risk:high]` → one notice;
   - stored `gate: human`, issue open with `[bug, risk:high]` → none;
   - stored `gate: human`, issue open with `[bug]` (a gate raised by hand on an issue that
     derives `model`) → none;
   - no `gate:` line and stored labels `[risk:high]`, issue open with `[risk:high]` → none;
   - stored `gate: model`, issue open with `[bug]` → none;
   - stored `gate: model` and stored labels `[risk:high]`, issue absent from the open list
     (closed) → none, and the close-out plan for it is still `retire`;
   - stored `gate: model`, issue open with `[risk:high, verify-gate]` (an excluded label) →
     none, and the close-out plan is still `retire-label`;
   - `status: done` and stored `gate: model` at the stream root, issue open with `[risk:high]`
     → none, and the close-out plan is still `reactivate`;
   - the same, with the file in the stream's `done/` archive → none, and the plan is still
     `reactivate`;
   - a placeholder for a second scanned repo with the same issue number as the first case,
     stored `gate: human`, its issue open with `[risk:high]` → none: the lookup is keyed by
     repo and number, not number alone. Write this file's frontmatter in the test, because
     the `placeholderFile` helper fixes the repo.

   Then assert the total is exactly three, that no creation plan exists for any of these
   issues, and that a second `planScan` call over the same streams returns the same three.
4. `TestScanGateNoticeWritesNothing` (planned) in the same file, through `runScanIssues` on a
   temp root shaped like `TestScanIssuesDryRunWritesNothing`'s, holding one placeholder with
   stored `gate: model` whose issue is open with `[bug, risk:high]` and nothing else to do.
   Run it with `dryRun` true and again with `dryRun` false, capturing stderr with
   `captureStderr`. For each run assert: the exit code is 0; stderr holds exactly one line
   that starts `NOTICE: scan-issues: ` and contains `derives gate: human but placeholder`; the
   placeholder's bytes are identical before and after; and the sorted list of file paths
   under the scanned stream directory is identical before and after.
5. In the intake-desk skill's scored-triage section, append these TWO sentences to the end of
   the bullet statusgen/17 added (the bullet that begins "`risk:high` gates the placeholder, at
   first write only" and ends "in it by hand."). Keep the wording: Verify row 5 pins four of
   its clauses and that it directly follows statusgen/17's sentence. Re-wrap the bullet if the
   file needs that. Edit only that bullet and leave every generated block alone.

   ```
   `statusgen --scan-issues` prints one `NOTICE` for each such item on every pass, with or
   without `--dry-run`, naming the issue and the placeholder file, until the file reads
   `gate: human`: that line is the mechanical signal that this edit is still owed. The desk's
   scan drain does not show it; read it by running `statusgen --root . --scan-issues --dry-run`,
   which writes nothing.
   ```

   The text says where the signal is and how to read it. It says the desk drain does not
   show it, because it does not, so a triage session does not take the line for a reminder
   someone else is already reading.
6. In this brief's frontmatter, change the two `consumers:` entries that read `follow-up
   statusgen/18` to `fixed-here`, keeping each site and its parenthesised note. Leave the two
   `out-of-scope` entries as they are. Verify row 7 refuses a leftover `follow-up`.
7. Add the changelog fragment.
8. Regenerate the Cursor skill twin and its tracked manifest with the canonical
   `deskinstall --harness cursor --forge github --repo . --bundle plugins/assay` placement.
   Keep the generated delivery diff to the intake-desk skill and its manifest digest, and
   declare both as `fixed-here` consumers. Do not edit the generated files by hand.

## Verify (executable — no prose-only DoD items)
Every row that runs a named test anchors its selector, writes the output to a file and asserts
that test's `--- PASS:` line, so a missing or renamed test fails the row. Rows 1–6, 8 and 9 read
the tree and run the same before and after merge. Row 7 reads the implementing diff and is run
on the implementing branch before merge; its Expect says what it does on merged main.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd statusgen && GOWORK=off go build ./... && GOWORK=off go vet ./...` | exit 0 | check:ci |
| 2 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestScanGateMismatchNotice$' -v . > "${TMPDIR:-/tmp}/sg18-r2.out" 2>&1 && grep -F -e '--- PASS: TestScanGateMismatchNotice' "${TMPDIR:-/tmp}/sg18-r2.out"` | exit 0; through `planScan`, the three mismatching placeholders (a late `risk:high` label, a gate word in the title, a file with no `gate:` line that reads `model`) each get exactly one notice with the pinned text, and the nine others get none: stored `human` beside a `human` issue, a hand-raised `human` beside a `model` issue, a gate-less file that reads `human`, `model` beside `model`, a closed issue, an excluded label, a retired placeholder at the root and in the archive, and a same-numbered issue in a second repo. The total is three on the first and on a second pass. Mutation: with the appended notice removed the row exits 1 (zero notices, want three); with the `Gate` condition dropped it exits 1 on the stored-`human` cases; with the `Status` condition dropped it exits 1 on the retired case | check:ci +mutation |
| 3 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestScanGateNoticeWritesNothing$' -v . > "${TMPDIR:-/tmp}/sg18-r3.out" 2>&1 && grep -F -e '--- PASS: TestScanGateNoticeWritesNothing' "${TMPDIR:-/tmp}/sg18-r3.out"` | exit 0; through the `--scan-issues` entrypoint, with and without `--dry-run`: the run exits 0, stderr carries exactly one `NOTICE: scan-issues: ` line for the mismatch, the placeholder's bytes are unchanged and no file under the stream directory was created, moved or removed. Red when the change writes the placeholder, plans a write for it, or changes the exit code | check:ci +flow |
| 4 | `cd statusgen && GOWORK=off go test -count=1 -timeout 600s .` | exit 0; the whole package passes, so no existing scan test depended on a skipped placeholder producing no notice | check:ci |
| 5 | `awk '/^## /{f=/^## Scored triage/} f{$1=$1; printf "%s ", $0}' plugins/assay/skills/intake-desk/SKILL.md > "${TMPDIR:-/tmp}/sg18-r5.txt" && grep -q -e 'in it by hand\. .statusgen --scan-issues. prints one .NOTICE. for each such item on every pass' "${TMPDIR:-/tmp}/sg18-r5.txt" && grep -q -e 'naming the issue and the placeholder file, until the file reads .gate: human.' "${TMPDIR:-/tmp}/sg18-r5.txt" && grep -q -e 'mechanical signal that this edit is still owed' "${TMPDIR:-/tmp}/sg18-r5.txt" && grep -q -e 'scan drain does not show it; read it by running .statusgen --root \. --scan-issues --dry-run., which writes nothing\.' "${TMPDIR:-/tmp}/sg18-r5.txt"` | exit 0; the skill's scored-triage section, read with its lines joined so that re-wrapping does not matter, carries the new text directly after the sentence statusgen/17 added (the first pattern spans the two), and it says what is printed, what it names, when it stops, that the desk drain does not show it and how to read it. A `.` in a pattern stands for a backtick. Red when the text is absent, does not directly follow statusgen/17's sentence, or lacks a clause. The row does not pin that nothing follows the text in the bullet. The row reads the skill's text. Whether the scan prints what the text says is rows 2 and 3 | check:ci +dereference |
| 6 | `cd tools/skillslint && go build -o "${TMPDIR:-/tmp}/sg18-skl" . && "${TMPDIR:-/tmp}/sg18-skl" --root ../..` | exit 0; the intake-desk skill still passes every skill check after the two-sentence edit, and no generated block drifted | check:ci |
| 7 | `! grep -q -e '[:] follow-up statusgen/18' docs/streams/statusgen/brief-18-gate-mismatch-notice.md && cd statusgen && GOWORK=off go build -o "${TMPDIR:-/tmp}/sg18c" . && cd .. && "${TMPDIR:-/tmp}/sg18c" --root . --consumers --brief statusgen/18 --base "$(git merge-base refs/remotes/origin/main HEAD)" > "${TMPDIR:-/tmp}/sg18-r7.out" 2>&1 && grep -q -F -e 'summary: 4 corroborated, 0 disproved' "${TMPDIR:-/tmp}/sg18-r7.out"` | exit 0, run on the implementing branch before merge: no `consumers:` entry still routes to this brief as a follow-up, and with the two source entries rewritten to `fixed-here` and both generated delivery entries declared the instrument corroborates all four and disproves none. The two `out-of-scope` entries are unchanged from the base and are listed as unchecked. Red when the routings are left as authored (the first grep), and red when any `fixed-here` path is missing from the diff (the instrument reports `DISPROVED` and exits 1). On merged main the merge-base is HEAD and the brief is not in the diff, so the instrument exits 2, could-not-check, and the row does not pass there; a post-merge run checks out the delivering commit and passes its parent as `--base` | check |
| 8 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestScanRiskHighPlaceholderGate$' -v . > "${TMPDIR:-/tmp}/sg18-r8.out" 2>&1 && grep -F -e '--- PASS: TestScanRiskHighPlaceholderGate' "${TMPDIR:-/tmp}/sg18-r8.out"` | exit 0; the scan-level test statusgen/17 adds still passes: an existing `gate: model` placeholder for a `risk:high` issue is neither re-planned nor re-read as `human`. The notice was added beside the derive-once rule and did not change it. Red when that test is absent, which means statusgen/17 has not landed | check:ci +neighbour |
| 9 | `cd tools/desk && go run ./cmd/deskinstall --harness cursor --forge github --repo ../.. --bundle ../../plugins/assay --check` | exit 0; the generated Cursor skill contains the same notice guidance and its manifest digest matches. Before canonical regeneration this check reports both paths as `content-differs` | check:ci +dereference |

Pre-mortem — "this shipped and was wrong; what went wrong?":

| Failure mode | Caught by |
|---|---|
| The notice also fires for a placeholder that reads `human`, or reads as advice to lower a gate | row 2 (the stored-`human` and hand-raised cases) |
| The notice never fires: the lookup misses, or is keyed by issue number alone | row 2 (three expected, and the second-repo case) |
| An issue gets two notices in one scan | row 2 (the count is exact) |
| The change writes the placeholder or changes the exit code | row 3 |
| The notice prints the issue's title or a label | row 2 (the title case) and row 3 (one pinned line) |
| The derive-once rule was changed along the way | rows 4 and 8 |
| The skill sentences are missing, misplaced or say something else | row 5 |
| statusgen/17 has not landed, so `risk:high` derives `model` | rows 2, 5 and 8 go red |
| Nobody sees the notice, because the desk drain drops the scan's output | no row. Review-only: it is outside the answer on #2405, routed `out-of-scope` in `consumers:` and listed under Out of scope |
| An item a person deliberately keeps at `gate: model` is reported on every scan | no row. Review-only: a known limit, listed under Out of scope |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->
### Non-implementer verifier run — 2026-10-29 — claude-opus-5-5 (strong), source 9e61aee3f8e9

Verified on merged main at source 9e61aee3f8e946198214101938c0420ca37d7f6a (delivered by PR #4539, merge commit c49e6877db03). Brief frontmatter: gate model, every risk answer no, not irreversible.

The check:ci rows 1-6, 8 and 9 ran inside the closed admission bracket: host open-bracket, then the verify-runner v1.0.65 image (linux/amd64) under docker run with network none, read-only root, all capabilities dropped and the receipt mounted read-only, then host close-bracket; the bracket digest reports verdict=match. Row 7 is classed check (it needs git history and the forge-free consumers instrument), so it ran on the host through verifyrun with only that row selected. Both witness blocks below are the tool's own bytes.

#### Container-lane witness (rows 1-6, 8, 9)

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && GOWORK=off go build ./... && GOWORK=off go vet ./...` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 9e61aee3f8e9 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestScanGateMismatchNotice$' -v . > "${TMPDIR:-/tmp}/sg18-r2.out" 2>&1 && grep -F -e '--- PASS: TestScanGateMismatchNotice' "${TMPDIR:-/tmp}/sg18-r2.out"` | pass exit=0 sandbox=container-netns | sha256:2b7f5c595340 | 2026-10-29 | assay-verifier-app[bot] @ 9e61aee3f8e9 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestScanGateNoticeWritesNothing$' -v . > "${TMPDIR:-/tmp}/sg18-r3.out" 2>&1 && grep -F -e '--- PASS: TestScanGateNoticeWritesNothing' "${TMPDIR:-/tmp}/sg18-r3.out"` | pass exit=0 sandbox=container-netns | sha256:93fd5e2962cc | 2026-10-29 | assay-verifier-app[bot] @ 9e61aee3f8e9 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && GOWORK=off go test -count=1 -timeout 600s .` | fail exit=1 sandbox=container-netns | sha256:0e25d1bbcf30 | 2026-10-29 | assay-verifier-app[bot] @ 9e61aee3f8e9 (on-behalf-of human:ian) (forge-identity) |
| 5 | `awk '/^## /{f=/^## Scored triage/} f{$1=$1; printf "%s ", $0}' plugins/assay/skills/intake-desk/SKILL.md > "${TMPDIR:-/tmp}/sg18-r5.txt" && grep -q -e 'in it by hand\. .statusgen --scan-issues. prints one .NOTICE. for each such item on every pass' "${TMPDIR:-/tmp}/sg18-r5.txt" && grep -q -e 'naming the issue and the placeholder file, until the file reads .gate: human.' "${TMPDIR:-/tmp}/sg18-r5.txt" && grep -q -e 'mechanical signal that this edit is still owed' "${TMPDIR:-/tmp}/sg18-r5.txt" && grep -q -e 'scan drain does not show it; read it by running .statusgen --root \. --scan-issues --dry-run., which writes nothing\.' "${TMPDIR:-/tmp}/sg18-r5.txt"` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 9e61aee3f8e9 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/skillslint && go build -o "${TMPDIR:-/tmp}/sg18-skl" . && "${TMPDIR:-/tmp}/sg18-skl" --root ../..` | pass exit=0 sandbox=container-netns | sha256:aa57cc340c0a | 2026-10-29 | assay-verifier-app[bot] @ 9e61aee3f8e9 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestScanRiskHighPlaceholderGate$' -v . > "${TMPDIR:-/tmp}/sg18-r8.out" 2>&1 && grep -F -e '--- PASS: TestScanRiskHighPlaceholderGate' "${TMPDIR:-/tmp}/sg18-r8.out"` | pass exit=0 sandbox=container-netns | sha256:d972e8243ca7 | 2026-10-29 | assay-verifier-app[bot] @ 9e61aee3f8e9 (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd tools/desk && go run ./cmd/deskinstall --harness cursor --forge github --repo ../.. --bundle ../../plugins/assay --check` | pass exit=0 sandbox=container-netns | sha256:54208d57d196 | 2026-10-29 | assay-verifier-app[bot] @ 9e61aee3f8e9 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:45a7cf09742c66449d44c13be973ed4b287736e750b4de6f320474fc5a2c821b
Container-Image: sha256:1af55e60c9222ad09dab8376b629725135c54912bb3583c13591833db90f2d8f
Verification-Attestation: medici-finance/assay#4836 run=15fd02d3a7a5083f2c4828d21ffa67644535b5d76c5d0b9a53922e19b3487728 source=9e61aee3f8e946198214101938c0420ca37d7f6a brief=docs/streams/statusgen/brief-18-gate-mismatch-notice.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=15fd02d3a7a5083f2c4828d21ffa67644535b5d76c5d0b9a53922e19b3487728 source=9e61aee3f8e946198214101938c0420ca37d7f6a tree=sha256:45a7cf09742c66449d44c13be973ed4b287736e750b4de6f320474fc5a2c821b witness=sha256:8a67081672870e0cc86f11a13dd48fb798e2c8ad54dc1f3168f3c4c4eb14fb51 opened=2026-10-29T17:51:18Z closed=2026-10-29T18:07:27Z before=receipt after=check-verifier

#### Host witness (row 7)

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 7 | `! grep -q -e '[:] follow-up statusgen/18' docs/streams/statusgen/brief-18-gate-mismatch-notice.md && cd statusgen && GOWORK=off go build -o "${TMPDIR:-/tmp}/sg18c" . && cd .. && "${TMPDIR:-/tmp}/sg18c" --root . --consumers --brief statusgen/18 --base "$(git merge-base refs/remotes/origin/main HEAD)" > "${TMPDIR:-/tmp}/sg18-r7.out" 2>&1 && grep -q -F -e 'summary: 4 corroborated, 0 disproved' "${TMPDIR:-/tmp}/sg18-r7.out"` | fail exit=2 | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 9e61aee3f8e9 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4836 run=15fd02d3a7a5083f2c4828d21ffa67644535b5d76c5d0b9a53922e19b3487728 source=9e61aee3f8e946198214101938c0420ca37d7f6a brief=docs/streams/statusgen/brief-18-gate-mismatch-notice.md model=claude-opus-5-5 tier=strong

#### Row accounting

- Rows 1, 2, 3, 5, 6, 8 and 9 pass inside the container lane (7 of 9).
- Row 4 is could-not-run in this environment: no test failed. The container run of the package suite stopped at the row's own 600s budget (`panic: test timed out after 10m0s`, no failing test reported; the tests still running at the deadline were unrelated to this brief). Two separate causes stand in the way, neither in this brief's delivery: (a) the host was under heavy load from other verifier containers (load average 37-58 on 16 cores), and the emulated linux/amd64 package suite needs well over 600s like that; (b) since this brief merged, the package also contains the witness skew tests TestWitnessSkewOldReader and TestWitnessV1Unchanged (statusgen/36). They fail by design when SG_OLD_BIN is unset, and CI sets it through statusgen/fetch-baseline.sh before go test. Row 4's literal command does not provide it. So at the current source, row 4 as written cannot exit 0 outside CI on any host.
- Row 7 exits 2 on merged main. That is the consumers instrument's COULD-NOT-CHECK: once merged, the merge-base of main and HEAD is HEAD itself, so the diff is empty and the brief is "not in the diff". Nothing was disproved. The row is pinned to the pre-merge form and cannot pass on merged main.

#### Supplemental host observations — no acceptance credit

All of these were recorded on 2026-10-29 by the same verifier session that produced the witnesses above, at the same source unless a different commit is named.

- Row 4 diagnosis: the full package suite on the darwin-arm64 host, with the budget raised to 40m, finished in 845.8s. 2959 top-level tests passed and exactly two failed: TestWitnessSkewOldReader and TestWitnessV1Unchanged, both reporting "SG_OLD_BIN is unset". Then statusgen/fetch-baseline.sh fetched the v1.0.52 baseline and checked it against that release's checksums.txt (sha256 01aed27b2b90…). With SG_OLD_BIN pointing at it, both tests pass (`--- PASS: TestWitnessSkewOldReader`, `--- PASS: TestWitnessV1Unchanged`, ok 5.254s). Every test in the package therefore passes once CI's precondition is supplied. The only remaining gap is the 600s budget under this host's load.
- Row 7 post-merge replay, following the instrument's own recipe for a merge commit (check out M^2 and use its own merge-base). It used a scratch clone at PR head b7668886e24f, with refs/remotes/origin/main set to f2e232039c99 (the merge's first parent), so the merge-base resolves to the PR base 8663d6ae0579. Row 7's exact command was run there and exited 0: 4 corroborated, 0 disproved, 2 unchecked as out of scope (two files outside this brief's claims), 0 briefs claiming nothing.
- Row 2 discrimination (mutations in a scratch clone at the source, reverted afterwards): removing the appended notice gives exit 1, "got 0 mismatch notices, want 3". Dropping the placeholder-gate condition gives exit 1, "got 6 … want 3". Dropping the done-status condition gives exit 1, "got 4 … want 3". The test pins each of the three conditions independently.

#### Risk-bearing values

Enumerated over the delivery diff 8663d6ae0579..b7668886e24f (7 paths). The literals that decide behaviour: the derived-gate comparison == "human" (statusgen/scanissues.go:499), the skip conditions ph.Status == "done" and ph.Gate != "model" (statusgen/scanissues.go:501), the notice text (statusgen/scanissues.go:508), the placeholder key repo + "#" + issue number (statusgen/scanissues.go:439 and :497), the scored-triage sentences (plugins/assay/skills/intake-desk/SKILL.md:207-211), the regenerated cursor manifest digest for intake-desk (.cursor/assay-managed.json:24), and the test's want-count 3 (statusgen/scanissues_test.go:2069). No value is irreversible: the change only detects and writes nothing, which row 3 pins. The top-ranked values are the three gate and status conditions.

RISK-VALUE: "human" / "model" (statusgen/scanissues.go:499, :501) — derived: derivePlaceholderGate returns only these two values (statusgen/placeholder.go:169-176), and Task 2 names the mismatch as derived human against a file reading model. Row 2's mutations show the test fails when the gate condition is dropped.
RISK-VALUE: "done" (statusgen/scanissues.go:501) — derived: done is the retired-placeholder status the brief excludes from the notice. Row 2's mutation shows the test fails (4 notices, not 3) when the condition is dropped.
RISK-VALUE: placeholder key repo + "#" + number (statusgen/scanissues.go:439, :497) — derived: the key includes the repository, so the same issue number on a second repository is a different placeholder. Row 2's fixture covers the second-repository case.

**VERIFY: BLOCKED** — 7 of 9 rows pass at source 9e61aee3f8e9. Row 4 is could-not-run here (600s budget exceeded under host load, plus the SG_OLD_BIN precondition added after delivery that only CI supplies; with it supplied, every package test passes). Row 7 is COULD-NOT-CHECK on merged main (pre-merge-pinned row, exit 2; the post-merge replay corroborates 4 of 4 with 0 disproved). No obligation was observed to fail.

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
