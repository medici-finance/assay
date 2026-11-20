---
brief: assay:assay:desk-tools:62
title: "Name the file, line and side of a branch-diff secret-scan refusal, not its line in the stripped diff"
why: >-
  When deskpr's branch-diff scan refuses, the line it reports is the line's position inside the
  stripped diff text: a hunk-relative count that names no file and matches no line anyone can
  open. #3196 measured it pointing at line 20 when the run sat near line 667 of the file. The
  reader goes to the wrong place, and sometimes rewords text that was never the problem.
  deskevidence fixed the same defect for its one-file surface (#3283). This brief fixes the
  multi-file diff surface. It gives one owner the job of mapping a diff line to its file, line
  and side, the owner the outbound check already uses for added lines, and leaves what the scan
  refuses unchanged.
wave: 1
depends: []
unblocks: ["desk-tools/63", "desk-tools/64"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: >-
  The diff parser this brief extends is the one the outbound push check uses to place every
  added line before it scans it. A mistake there makes the outbound check skip or misplace an
  added line, and a credential on that line is pushed. The brief changes no verdict and proves
  it, but a change to a credential control's input path is reviewed by a human before the
  ready flip.
decision-trigger: start
decision-issue: 4284
ruling: "https://github.com/medici-finance/assay/issues/4284#issuecomment-6774271356"
issues: [3196]
outcome: none
design: DR-scan-diff-reader
schema: brief-v2
version: 1
authored: 2026-11-10 by a desk-dispatched authoring session (error-class design for issue 4284; the location half of issue 3196)
sources:
  - "medici-finance/assay#3196 part 2 — the refusal's line number is hunk-relative (reported 20, the run sat near file line 667)"
  - "medici-finance/assay#3283, closed by #3300 (418592cb5) — the same defect fixed for deskevidence by blanking unchanged lines so the scanner's line is the destination file's line; that technique needs one file, and a branch diff has many"
  - "tools/desk/internal/deskkit/outboundpush.go — AddedLinesByFile and hunkNewStart already parse hunks into new-file line numbers for the outbound check; the one diff-line owner this brief extends"
  - "tools/desk/cmd/deskpr/deskpr.go — scanWrite scans stripDiffMetaLines(DiffSymmetric(defaultRef, HEAD, 3)); the finding's Line is the line in that stripped text"
  - "freshness-checked 2026-11-10 @ 450ac4047 (origin/main)"
exec-tier: strong
exec-tier-why: "Diff parsing across added, removed and context sides, renames and deletions, with a proof that the scanned bytes are unchanged."
domain: complicated
consumers:
  - "tools/desk/cmd/deskpr/deskpr.go: fixed-here (scanWrite maps the branch-diff finding through the index before reporting)"
  - "tools/desk/README.md: fixed-here (the deskpr row states what the refusal location names)"
id: 69746756-158a-4edf-b128-117ea60e45ad
---

# Brief 62 — Name the file, line and side of a branch-diff refusal

## Dependencies

None. It is diagnostic only: no verdict changes, and nothing is refused or admitted that was
not before. desk-tools/63 reuses the index this brief adds, to anchor whole-file grammars on
diff lines. desk-tools/64 decides which diff lines are scanned, and this brief works for any
answer to it.

## Context

files:
- `tools/desk/internal/deskkit/outboundpush.go` — the hunk parser gains a whole-diff line index; `AddedLinesByFile` becomes a consumer of it or stays as is, but there is never a second hunk parser
- `tools/desk/internal/deskkit/difflines_test.go` (planned) — NEW
- `tools/desk/cmd/deskpr/deskpr.go` — `scanWrite`'s branch-diff surface; `stripDiffMetaLines`' meta-line rule moves to or is shared with the index, so the stripped text and its index come from one pass
- `tools/desk/cmd/deskpr/difffinding_test.go` (planned) — NEW
- `tools/desk/cmd/deskpr/difffinding-mutations.json` (planned) — NEW
- `tools/desk/README.md` — the deskpr row
- **add** — `changelog/<branch-slug>.md` — one fragment

facts:
- The branch-diff surface is the stripped text of `DiffSymmetric(defaultRef, "HEAD", 3)`. Git
  meta lines (`diff --git`, `index`, `---`, `+++`, rename and similarity lines) are removed, and
  every added, removed and context line stays, with its one-character marker. `ScanFinding.Line`
  is the 1-based line of the first offending span in that text.
- `BodyCheck` and `ScanSurfaceSecrets` are generic and carry no diff awareness by design. The
  comment in `scanWrite` says so. The mapping therefore lives with the diff owner and is applied
  to the finding after the scan returns. It is never pushed into the scanner.
- `deskkit.AddedLinesByFile` already reads `+++` paths, `@@` new-side starts (`hunkNewStart`),
  new and renamed files, and deleted files, and caps placement at `maxPlacedLine` (200000).
  Old-side starts are not parsed yet.
- A removed line has an old-side line number in the base file, and an added or context line has
  a new-side number in the head file. The reader needs the side to know which version to open.
- `--explain` prints `scan-explain: rule=… line=… length=… shape=…`, and the shape is redacted.
  `TestExplainNamesRuleAndLineNeverSpan` pins that it never prints the span.

layering: one function in the existing diff owner, `DiffLineIndex(diff) (stripped string, refs
[]DiffLineRef)`, where a `DiffLineRef` is `{Path, Side, FileLine}` with `Side` one of `added`,
`removed`, `context`, built in the same pass that strips the meta lines. deskpr looks the
finding's line up in `refs`. No new package and no change to the scanner.

design-fit:
  owner: tools/desk/internal/deskkit/outboundpush.go — the hunk parser the outbound check uses
  contract: S-publication-scan (diagnostic only; the refused set is unchanged)
  retires: [the hunk-relative line in deskpr's branch-diff refusal, deskpr's private meta-line stripping as a second reading of diff structure]
  weight: "verbs 0, flags 0, refusals 0; one exported function and one record type in the existing owner"
  why-add: "The refusal must name a location a reader can open. The index is the parser that already exists, extended to the old side and to context lines, so no second parser is added."

## Human decision

When the desk's code-diff secret scan refuses a pull request, it reports a line number counted
inside the stripped diff text. That number names no file and matches no line anyone can open,
so readers go to the wrong place. The fix reports the file, the line in that file, and whether
the line was added, removed or unchanged. To do that it extends the diff reader the outbound
push check already uses, so that only one diff reader exists. The scan refuses exactly what it
refused before, and the change proves it. The question is whether to extend the shared reader or
add a second one.

Options:
1. **Extend the shared diff reader** — one reader for both checks. A test proves that the scanned
   text is byte-identical to before, and the outbound check's own tests stay unchanged.
2. **Add a separate reader for the report only** — the outbound check's reader is not touched,
   but two readers of diff structure exist and can drift apart.

Default if no answer: option 1 after 2026-11-17. Neither option changes what is refused.

## Ground rules
- Ruled option A on the decision issue, which is option 1 above: https://github.com/medici-finance/assay/issues/4284#issuecomment-6774271356
  The ruling extends the shared diff reader, with a byte-identity test and the outbound check's
  tests left unedited. Option 2 is not taken. It names no record id, so it was restated under
  DR-scan-diff-reader: https://github.com/medici-finance/assay/issues/4284#issuecomment-6775569997
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- `AddedLinesByFile`'s results do not change for any input. If it is rebuilt on the index, its
  existing tests pass unedited.
- The bytes scanned do not change, and neither does the verdict. Do not touch `bodycheck.go`,
  `structured.go` or the scanner's tests.
- Never write a credential-shaped literal into a test. Generate the planted run from a fixed
  seed or use the corpus seam.

## Task
1. Add `DiffLineIndex` to `outboundpush.go`, parsing old-side and new-side hunk starts. For each
   kept line it records path, side and file line: new-side numbering for added and context
   lines, old-side for removed lines, and the old path for a deleted file. A line past
   `maxPlacedLine` or under a malformed hunk header gets `FileLine` 0, which is reported as an
   unknown line and never guessed. `TestDiffLineIndex` (planned) covers multi-file, multi-hunk, rename,
   new-file, deleted-file, no-newline-at-end and malformed-header diffs.
2. The stripped text `DiffLineIndex` returns must be byte-identical to what deskpr scans today.
   `TestDiffLineIndexStrips` (planned) runs both the old `stripDiffMetaLines` and the index over the same
   diffs, including the existing `stripDiffMetaLines` test cases, and asserts equal output. Then
   point deskpr at the index's text and retire the private copy.
   `TestAddedLinesParity` (planned) runs `AddedLinesByFile` and the index's added refs over every
   diff that `TestDiffLineIndex` (planned) covers, and asserts they name the same files and the same new-side
   lines. Its one direct test on main covers a single file with two hunks, so this differential
   test is what covers rename, new-file, deleted-file, multi-file and no-newline diffs for the
   outbound check. It is required whether or not `AddedLinesByFile` is rebuilt on the index.
3. In `scanWrite`, when the branch-diff scan returns a finding, report `<path>:<line> (<side>)`
   in the refusal and in `--explain`, alongside the rule. `--explain` gains `path=` and `side=`
   fields after `line=`, which now carries the file line. The redacted shape and the "never the
   span" rule stay. `TestDiffFindingFileLine` (planned) plants a generated run at a known file line in each
   of the three sides, in the second file of a three-file diff. It asserts the reported path,
   line and side, and that the old stripped-text line number appears nowhere in the output.
4. `TestDiffFindingVerdicts` (planned) runs the branch-diff scan over the same diffs with and without the
   index, and asserts the same refused-or-clean result, the same rule and the same override
   path.
5. Add `difffinding-mutations.json` with four mutants, each caught by the tests above:
   report the stripped-text line, label every line `added`, number removed lines on the new
   side, and drop the added lines of a renamed file from the index. The spec's `test` command
   runs both `./internal/deskkit` and `./cmd/deskpr`, because the index mutants live in deskkit.
6. State the location format in the deskpr row of `tools/desk/README.md`. Change both consumer
   entries to `fixed-here` in the implementation PR.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestDiffLineIndex' -count=1 -v > "${TMPDIR:-/tmp}/dt62-idx.log" 2>&1 && grep -F -e '--- PASS: TestDiffLineIndex ' "${TMPDIR:-/tmp}/dt62-idx.log" && grep -F -e '--- PASS: TestDiffLineIndexStrips ' "${TMPDIR:-/tmp}/dt62-idx.log" && go test ./internal/deskkit/ -run '^TestAddedLinesParity$' -count=1 -v > "${TMPDIR:-/tmp}/dt62-par.log" 2>&1 && grep -F -e '--- PASS: TestAddedLinesParity ' "${TMPDIR:-/tmp}/dt62-par.log"` | exit 0; all three named PASS lines; the index numbers every side correctly, its added lines match the outbound reader's, and its stripped text is byte-identical to what deskpr scanned before |
| 2 | check:ci +flow | `cd tools/desk && go test ./cmd/deskpr -run '^TestDiffFinding' -count=1 -v > "${TMPDIR:-/tmp}/dt62-find.log" 2>&1 && grep -F -e '--- PASS: TestDiffFindingFileLine ' "${TMPDIR:-/tmp}/dt62-find.log" && grep -F -e '--- PASS: TestDiffFindingVerdicts ' "${TMPDIR:-/tmp}/dt62-find.log"` | exit 0; both named PASS lines; the refusal names the file, line and side, and the verdict is unchanged |
| 3 | check:ci +mutation | `O=$(mktemp) && trap 'rm -f "$O"' 0 && cd tools/desk && go run ./cmd/muhar -spec cmd/deskpr/difffinding-mutations.json > "$O" 2>&1 && grep -F -e 'Harness healthy: baseline GREEN, positive control CAUGHT.' "$O" && grep -F -e 'Totals: 4 caught, 0 NOT CAUGHT, 0 could-not-mutate.' "$O"` | exit 0; all four mutants caught |
| 4 | check:ci +neighbour | `cd tools/desk && test -f internal/deskkit/difflines_test.go && go test ./internal/deskkit/ -count=1 && go test ./cmd/deskpr -count=1 && go test ./cmd/muhar -run '^TestSpecAnchorsLand$' -count=1 -v > "${TMPDIR:-/tmp}/dt62-anch.log" 2>&1 && grep -F -e '--- PASS: TestSpecAnchorsLand ' "${TMPDIR:-/tmp}/dt62-anch.log"` | exit 0; the outbound check's added-line tests, deskpr's remote-scan and override tests, and every mutation anchor still pass |
| 5 | check +flow +dereference | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && git diff --quiet "$DELIVERY_BASE" "$DELIVERY_HEAD" -- tools/desk/internal/deskkit/bodycheck.go tools/desk/internal/deskkit/structured.go tools/desk/internal/deskkit/testdata && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-62-diff-finding-file-line.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/62 --delivery "$DELIVERY_HEAD" --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 2 corroborated, 0 disproved, 0 unchecked' "$o"` | exit 0; the scanner and its fixtures are untouched, and deskpr and its README row are in the delivering diff |

## Evidence
<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This authoring PR claims no future implementation test executed. -->

### Post-merge verification — 2026-11-18

Independent post-merge verifier, not the implementer. Delivering PR #6766, merged as 6335d2d8e3cdaac8897d21f8603d99ff3a0edcae (first parent 41aaa64abad93ce902e5024346dc5c90a93e8f85). Verified at main source 10b5d50b7a4ff0ed604608455bab2e1dd2028389. Risk line has `sensitive-data: yes`, so every row was run `--fail-first` against the first parent of the delivering merge.

Rows 1–4 ran in one admitted container-netns bracket (network none, read-only root and source mount, release-pinned runner image). The closed witness bytes follow verbatim. `--bracket-digest` on them reports `verdict=match span-lines=tree-digest:1,container-image:1,attestation:1`.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestDiffLineIndex' -count=1 -v > "${TMPDIR:-/tmp}/dt62-idx.log" 2>&1 && grep -F -e '--- PASS: TestDiffLineIndex ' "${TMPDIR:-/tmp}/dt62-idx.log" && grep -F -e '--- PASS: TestDiffLineIndexStrips ' "${TMPDIR:-/tmp}/dt62-idx.log" && go test ./internal/deskkit/ -run '^TestAddedLinesParity$' -count=1 -v > "${TMPDIR:-/tmp}/dt62-par.log" 2>&1 && grep -F -e '--- PASS: TestAddedLinesParity ' "${TMPDIR:-/tmp}/dt62-par.log"` | pass exit=0 sandbox=container-netns | sha256:e8166d77ccd2 | 2026-11-18 | assay-verifier-app[bot] @ 10b5d50b7a4f (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=41aaa64abad9 red rc=1 · head=10b5d50b7a4f · delivering=6335d2d8e3cd |
| 2 | `cd tools/desk && go test ./cmd/deskpr -run '^TestDiffFinding' -count=1 -v > "${TMPDIR:-/tmp}/dt62-find.log" 2>&1 && grep -F -e '--- PASS: TestDiffFindingFileLine ' "${TMPDIR:-/tmp}/dt62-find.log" && grep -F -e '--- PASS: TestDiffFindingVerdicts ' "${TMPDIR:-/tmp}/dt62-find.log"` | pass exit=0 sandbox=container-netns | sha256:a8ad99f38ff6 | 2026-11-18 | assay-verifier-app[bot] @ 10b5d50b7a4f (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=41aaa64abad9 red rc=1 · head=10b5d50b7a4f · delivering=6335d2d8e3cd |
| 3 | `O=$(mktemp) && trap 'rm -f "$O"' 0 && cd tools/desk && go run ./cmd/muhar -spec cmd/deskpr/difffinding-mutations.json > "$O" 2>&1 && grep -F -e 'Harness healthy: baseline GREEN, positive control CAUGHT.' "$O" && grep -F -e 'Totals: 4 caught, 0 NOT CAUGHT, 0 could-not-mutate.' "$O"` | fail exit=1 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-18 | assay-verifier-app[bot] @ 10b5d50b7a4f (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=41aaa64abad9 red rc=1 · head=10b5d50b7a4f · delivering=6335d2d8e3cd |
| 4 | `cd tools/desk && test -f internal/deskkit/difflines_test.go && go test ./internal/deskkit/ -count=1 && go test ./cmd/deskpr -count=1 && go test ./cmd/muhar -run '^TestSpecAnchorsLand$' -count=1 -v > "${TMPDIR:-/tmp}/dt62-anch.log" 2>&1 && grep -F -e '--- PASS: TestSpecAnchorsLand ' "${TMPDIR:-/tmp}/dt62-anch.log"` | fail exit=1 sandbox=container-netns | sha256:e9df66579ca6 | 2026-11-18 | assay-verifier-app[bot] @ 10b5d50b7a4f (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=41aaa64abad9 red rc=1 · head=10b5d50b7a4f · delivering=6335d2d8e3cd |
Tree-Digest: sha256:770bbc0b6ff600a8be94b7502ecee59b1266bd1562c594b0b2dbbfb8d346b86a
Container-Image: sha256:c67baa7a3c8ba3d514128f31340489146525a50c29ebb641a68899c650d354a1
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6942 run=9c7790d77390e027f932ba7a7651705769ebbb6a2ae3aab49fb74c3077ebea95 source=10b5d50b7a4ff0ed604608455bab2e1dd2028389 brief=docs/streams/desk-tools/brief-62-diff-finding-file-line.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=9c7790d77390e027f932ba7a7651705769ebbb6a2ae3aab49fb74c3077ebea95 source=10b5d50b7a4ff0ed604608455bab2e1dd2028389 tree=sha256:770bbc0b6ff600a8be94b7502ecee59b1266bd1562c594b0b2dbbfb8d346b86a witness=sha256:2cf567efef79925bb390c2cee2c58d097f216eabe8ae3e14188808512c3bd22b opened=2026-11-18T16:51:30Z closed=2026-11-18T17:01:37Z before=receipt after=check-verifier

Row 5 is host-only (it reads git history). It ran no command. `statusgen verifyrun --rows 5 --fail-first --delivering-commit 6335d2d8e3cdaac8897d21f8603d99ff3a0edcae --dry-run`, in the declared verifier Git environment, exited 2 before running the row. Its output, verbatim:

```
statusgen verifyrun: --fail-first: --delivering-commit "6335d2d8e3cdaac8897d21f8603d99ff3a0edcae" is refused: it cannot be read through the library read path (native discovery/config/shallow environment profile): GIT_CONFIG_COUNT is set in the environment, which selects a native Git profile the library does not qualify; unset it (with any GIT_CONFIG_KEY_<n>/GIT_CONFIG_VALUE_<n> it carries) and re-run
```

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 5 | see Verify row 5 | could-not-run: tool refusal before the row ran (rc=2); the declared verifier Git environment sets GIT_CONFIG_COUNT=0, which the fail-first library read path refuses | none | 2026-11-18 | assay-verifier-app[bot] @ 10b5d50b7a4f (host) (on-behalf-of human:ian) | unproven: not run |

This is the open class #6851. The declared row environment was not changed to get past the refusal. Row 5's definition also has a separate defect, independent of the refusal. Its guard requires `HEAD = DELIVERY_HEAD`, so post-merge DELIVERY_HEAD has to be the verified source, not the delivering merge. Its `git diff --quiet DELIVERY_BASE DELIVERY_HEAD -- <scanner and fixtures>` then also covers later main commits that changed the scanner. Those commits are unrelated to this delivery. So the row cannot express "untouched by the delivering diff" after merge (compare the class #6099). Static reading aid, not evidence: the delivering diff 41aaa64..6335d2d8 does not list the scanner, the structured-body module or the deskkit fixtures.

### Rows held

- **Row 3** (mutation harness) failed at head inside the container (exit 1, empty matched output). The harness's default `-j 1` mode edits the source tree in place and restores it. The admitted bracket mounts the source tree read-only, so the row as declared may not be witnessable in this sandbox. This matches the open class #6444. A supplemental diagnostic was not run, so the cause is unconfirmed. The row is held, not judged.
- **Row 4** (neighbour packages) failed at head inside the container (exit 1). The full deskkit and deskpr test packages include tests that drive git and read files above the module root. The cause was not diagnosed, so whether this is the environment or the deliverable is unconfirmed. The row is held, not judged.
- **Row 5**: tool refusal plus a definition defect, as above.
- Rows 1 and 2 discriminate: red at the base, green at head.

### Risk values (sensitive-data)

ENUMERATE, then RANK by effect on the scanned bytes, then DERIVE:

1. RISK-VALUE: hunk-header range pattern `^(@@ -[0-9]+(?:,[0-9]+)? \+[0-9]+(?:,[0-9]+)? @@)` (outboundpush.go, moved byte-identical from deskpr.go) — DERIVED: the unified-diff hunk range grammar, with the count optional because git omits a count of 1. A line that does not match is kept whole, so the pattern can only add scanning, never remove it.
2. RISK-VALUE: maxPlacedLine = 200000 (outboundpush.go; existed before this delivery and is now reused as the file-line cap) — NAMED, NOT DERIVED: no derivation is recorded. Above the cap, only the reported location changes, to unknown. The scanned bytes do not.
3. RISK-VALUE: finding lookup bounds 1 <= line <= len(refs) — DERIVED: from the 1-based line number indexing the line-ref slice.
4. RISK-VALUE: side labels added/removed/context and the scan-explain format string — N/A: these are report labels and do not decide what is scanned.

**VERIFY: BLOCKED** — rows 1 and 2 pass and are red at the base. Rows 3 and 4 failed in the admitted container and their cause is undiagnosed (possibly a read-only sandbox fault, since row 3 mutates the tree in place), so both are held. Row 5 was refused by the tool before it ran, and its definition cannot express "untouched by the delivering diff" after merge. No row is a FAIL of the deliverable.

## Threat model and detection

| Failure mode | Caught by |
|---|---|
| The location is right for added lines but wrong for removed or context lines | rows 1 and 2 |
| The stripped text changes, so the scan sees different bytes | row 1 (`TestDiffLineIndexStrips` (planned)) |
| The outbound check's added-line placement changes, so it skips or misplaces a line | row 1 (`TestAddedLinesParity` (planned) over rename, new-file, deleted-file, multi-file and no-newline diffs); row 3 (the renamed-file mutant); row 4 (its existing tests pass unedited) |
| The mapping changes a verdict or the override path | row 2 (`TestDiffFindingVerdicts` (planned)) |
| The new report prints the offending span | row 4 (`TestExplainNamesRuleAndLineNeverSpan` stays green); row 2 (`TestDiffFindingFileLine` (planned) asserts the output) |
| A mutated mapping goes unnoticed | row 3 |
| The scanner is edited to carry diff awareness | row 5 |

## Review
Gate: human. Before it leaves `todo`, the design needs an approved `DR-<slug>` record cited by
`design:` (brief-rules 48). That record is DR-scan-diff-reader. The reviewer checks that the
reported line can be opened at the named side: head
for added and context lines, base for removed lines. They check that no output path prints the
span, and that the outbound check's tests are unedited. Every Verify row is risk-bearing,
because the brief answers `sensitive-data: yes`. Each row is written to fail on the delivery
base, and the verifier records it with `verifyrun --fail-first` against the parent of the
delivering merge.
