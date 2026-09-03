---
brief: desk-tools/16
title: "`deskevidence` — an Evidence block equivalent to one already standing is a no-op, not a second block"
why: >-
  `deskevidence` is idempotent at the FILE level: the same merged content at the same head is
  a no-op. It is not idempotent at the BLOCK level: a fresh verifier run whose Evidence block
  is byte-equivalent to the block already standing in the brief is appended again, and the
  brief grows a duplicate. A 24-hour sweep of fifteen desk-role and worker session transcripts
  found this four times in one verify session — each duplicate then tracked as its own cleanup.
  A re-run that proves the same thing the last run proved has nothing to land; the verb should
  say so in one line, exit 0, and commit nothing.
wave: 1
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-02 by a worker-desk authoring session, from a 24-hour transcript sweep across
  fifteen desk-role and worker sessions (tallied per session)
sources:
  - "freshness-checked 2026-09-02 @ 547b708 — `tools/desk/cmd/deskevidence/deskevidence.go` § the idempotency check compares the WHOLE merged file's digest with the remote (`noop: … already has this content`); § mergeEvidence appends the new block after the existing Evidence section unconditionally, so a block already present is appended twice. The append-only shrink guard sits after the noop check and must keep doing so."
  - "The result vocabulary the no-op reports with: `tools/desk/internal/deskkit/audit.go` (`ResultNoop`) and the exit contract `exitcodes.go` (0 ok/noop)."
  - "Brief and Verify shape: `spec/brief-v1.md`; status semantics: `spec/lifecycle-v1.md`."
---

# Brief 16 — `deskevidence`: an Evidence block equivalent to one already standing is a no-op

## Dependencies
None.

## Context

files:
- `tools/desk/cmd/deskevidence/deskevidence.go` (`mergeEvidence` gains the containment check; the flow gains the second noop)
- `tools/desk/cmd/deskevidence/deskevidence_test.go`
- `tools/desk/README.md` (one sentence in `deskevidence`'s idempotency paragraph)

facts:
- today's flow: read local block → `mergeEvidence` (fetch remote brief, append block under
  `## Evidence`) → digest of merged vs remote → noop if equal → shrink guard → commit.
- **equivalence** is defined narrowly and stated in the README: after normalising line
  endings to `\n` and trimming trailing whitespace per line and trailing blank lines, the
  fresh block is a CONTIGUOUS substring of the existing Evidence section. Nothing looser (no
  "same commands", no "same runner") — a re-run on a different date with a different runner
  is new evidence and still lands.
- the no-op is reported as `noop: Evidence block already present in <path> on <branch> (sha
  <short>)`, audited as `ResultNoop`, exit 0, and NOTHING is fetched-then-put: the decision
  is taken on the content already fetched for the merge, so it adds no API call.
- ordering: the block-equivalence check runs BEFORE the shrink guard, like the file-level
  noop, so an idempotent re-run is never mistaken for a shrink.
- a fresh block that is a PREFIX of the standing section but not equal (a partial re-run) is
  NOT equivalent — it is new content and lands; a fresh block that CONTAINS the standing block
  plus new rows lands (the append then carries the whole fresh block, unchanged behaviour —
  the brief does not try to de-duplicate rows inside a block).
- `--force-append` is NOT added; a verifier who needs a second identical block has no such
  need.
- tests already stub the remote fetch (`deskevidence_test.go`); extend them.

## Ground rules
- No forge contact from any test or Verify row.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Task

1. **Containment check** in `mergeEvidence` (or a sibling `blockAlreadyPresent`) per the
   normalisation rule; return a distinguishable no-op result.
2. **Flow**: on block-equivalence, print the one line, audit `ResultNoop`, return 0 before the
   shrink guard and before any PUT.
3. **Tests**: identical block → noop, no PUT recorded, exit 0; identical block with CRLF and
   trailing spaces → noop; a block differing by one character → lands; a partial (prefix)
   block → lands; a superset block → lands; a brief with no Evidence section → lands (creates
   the section, unchanged); the existing file-level noop test unchanged.
4. **README** sentence.
5. **Nothing else.**

## Verify

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/desk && go build ./... && go vet ./...` | exit 0 |
| 2 | check:ci | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestEquivalentEvidenceBlockIsNoop$' -count=1` | exit 0 — stdout carries `noop: Evidence block already present`, the stub records no PUT, audit result is noop |
| 3 | check:ci | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestEquivalenceSurvivesLineEndingsAndTrailingSpace$' -count=1` | exit 0 |
| 4 | check:ci | `cd tools/desk && go test ./cmd/deskevidence/ -run '^TestNearEquivalentBlocksStillLand$' -count=1` | exit 0 — the NEGATIVE control: one-character difference, prefix, and superset each land (a PUT is recorded) |
| 5 | check:ci | `cd tools/desk && go test ./cmd/deskevidence/ -count=1` | exit 0 — the file-level noop and the shrink-guard tests unchanged |
| 6 | check:ci | `gofmt -l tools/desk/cmd/deskevidence > /tmp/de-fmt.out; test ! -s /tmp/de-fmt.out` | exit 0 |
| 7 | check:ci | `cd statusgen && go run . --root .. --lint; echo $?` | 0 |

Pre-mortem → detection map:

| Failure mode of the work | Caught by |
|---|---|
| Equivalence loosened to "same commands" and a genuinely new run is dropped | row 4 |
| The check runs after the shrink guard and a re-run is refused as a shrink | row 5 (shrink tests) + row 2 (exit 0) |
| The no-op still PUTs the unchanged file | row 2 (no PUT recorded) |
| CRLF from a Windows-authored block defeats the check | row 3 |

## Evidence
<!-- appended at implementation time: one witness row per Verify row —
     (command, exit code, output line(s), date, runner). -->

## Review

Gate: model (all four risk answers no). The reviewer confirms row 4 is present — the rows that
prove the check cannot swallow new evidence are the ones that matter.
