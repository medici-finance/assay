---
brief: assay:assay:windows-port:13
title: assay-inbox.sh → a Go `deskinbox` verb — table + walk (the inbox engine's shared core; html + flow split to windows-port/15)
why: >-
  The `assay:inbox` command is how a human or a desk walks the decision queue (`--walk`,
  `--flow`, `--html`) and the ask-decision skill calls it by name. It is the heaviest POSIX
  script in the plugin — 1,403 lines of bash calling gh twenty times, jq twenty-nine times and
  mktemp twenty-two times — and none of those exist on a native-Windows adopter's box. Every
  other desk surface now has a Windows path (binaries, install, Verify witness, and after brief
  11 the pollers); this is the one skill whose engine is still a shell script, so it is the one
  skill a Windows adopter cannot invoke at all.

  SPLIT AT PICKUP (dispatch's own pre-authorization: "if it's too big for one PR, STOP and
  split per the author-brief rules, keeping only the piece you were mid-implementing"). This
  brief now covers the shared engine (repo resolution, label-filtered issue query,
  dedupe/rank/sort, the five-part format builder) plus the `table` and `walk` renderings —
  `walk` is the `ask-decision` skill's actual entry point. `--html` and `--flow` (a wholly
  separate reader — statusgen/deskboard flow orchestration — plus an inline-SVG diagram
  builder) move to the new windows-port/15, which depends on this brief for the shared engine
  and format builder it reuses. See `tools/desk/cmd/deskinbox/testdata/spec.md` for the full
  split rationale and the mechanism divergences from the oracle.
wave: 4
depends: ["windows-port/00", "windows-port/01"]
unblocks: ["windows-port/15"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1435]
schema: brief-v2
authored: 2026-09-21 by the-desk (Bob) — windows-port authoring session, driver ask 2026-09-21
sources:
  - "plugins/assay/scripts/assay-inbox.sh (1,403 lines, #!/usr/bin/env bash; gh×20, jq×29, mktemp×22, find/date/awk/sed; ~79 bashism sites) — the engine; its `--help` and the ask-decision skill are the behavioural spec. CORRECTION (verified at pickup): the script invokes make zero times — every prior 'make times four' count named in this brief's own earlier draft was a false positive off the bare word 'make' appearing in comments/prose, never a make-target shell-out. There is no make-target-to-verb-call mapping to port."
  - "plugins/assay/skills/ask-decision/SKILL.md:50,54,189 (line numbers as of the earlier draft; re-pointed at pickup) — `bash <bundle>/scripts/assay-inbox.sh --walk --item 1 …`, `--flow`, `--html /path/to/inbox.html` — the three entry points a Windows adopter must be able to call"
  - "plugins/assay/commands/inbox.md (the command body that names the script; CORRECTION — the earlier draft cited a nonexistent path under plugins/assay/skills/inbox/, verified absent at pickup) — every invocation site to re-point for table/walk; html/flow stay pointed at the oracle until windows-port/15"
  - "docs/streams/windows-port/portability-audit.md — the script is ABSENT from brief 02's table (windows-port/11 Task 1 adds the row; this brief resolves it)"
  - "tools/desk/cmd/deskboard, tools/desk/cmd/issueboard — the resolved-Forge read pattern (deskkit.ForgeFor/SessionTokenRole) and ListOpenIssues op the engine reuses rather than re-implementing a gh client; cmd/deskpost's ghClient — the package-local, non-interface-op REST-reader pattern reused for comment bodies (detail.go)"
  - "freshness-checked 2026-09-21 @ 56491ce (origin/main): no verb named deskinbox; the script present at 1,403 lines"
exec-tier: strong
exec-tier-why: >-
  Question (b): the port must reproduce the script's format-builder output byte-for-byte as
  consumed by the ask-decision skill (format_parity_test.go extracts the oracle's own jq
  program and runs it through the system `jq` binary rather than a hand-copied second
  expectation, so the two sides cannot silently drift).
domain: complicated
parallel-streams:
  - {name: engine, files: ["tools/desk/cmd/deskinbox/**"]}
  - {name: skills, files: ["plugins/assay/commands/inbox.md", "plugins/assay/skills/ask-decision/**"]}
consumers:
  - "tools/desk/cmd/deskinbox/** (new verb: table (default) and walk): follow-up windows-port/13 (this brief). html and flow: follow-up windows-port/15"
  - "plugins/assay/scripts/assay-inbox.sh: follow-up windows-port/13 (this brief — kept as the parity oracle until windows-port/14 retires it)"
  - "plugins/assay/commands/inbox.md, plugins/assay/skills/ask-decision/SKILL.md (name the verb for table/walk; the script as fallback and as the current renderer for html/flow): follow-up windows-port/13 (this brief)"
version: 2
id: 9f77c761-e720-401f-b96c-f3dbf45001df
---

# Brief 13 — assay-inbox.sh → `deskinbox` (table + walk; the shared engine)

## Context
files: tools/desk/cmd/deskinbox/{main.go,forge.go,repos.go,query.go,table.go,detail.go,format.go,walk.go,summary.go,format_parity_test.go,query_test.go,table_test.go,repos_test.go,main_test.go,testdata/spec.md}, plugins/assay/commands/inbox.md, plugins/assay/skills/ask-decision/SKILL.md, changelog/windows-port-13-deskinbox.md
facts:
- modes and their contracts (from the script's --help, re-read at pickup): `walk --item N <repos…>` (one decision item at a time, oldest first, prints the same block the script prints) — PORTED; `flow` and `html <out> <repos…>` — split to windows-port/15
- there are no make invocations to port (see the sources CORRECTION above) — the earlier draft's Task 1 line calling for a make-target mapping is dropped
- parity: `format_parity_test.go`'s `TestParityWalk` extracts the oracle's own `write_format_program()` jq heredoc verbatim at test time and runs it through the system `jq` binary on identical fixture input, asserting the Go format builder byte-for-byte against the real jq program — no recorded gh fixtures needed for this half, since the format builder's input (item + issue detail) is independent of gh's wire shape
- identity: the verb reads through the resolved Forge (deskkit.ForgeFor/SessionTokenRole), the SAME seam cmd/deskboard and cmd/issueboard already use — a DELIBERATE improvement over the oracle's ambient-gh-keyring identity (see testdata/spec.md divergence 1); comment bodies (walk's "latest desk note") read through a small package-local GitHub-only REST client (detail.go), matching cmd/deskpost's ghClient pattern for reads with no typed Forge op
- Windows: paths via filepath; no shell-outs anywhere in the package — proven by grep in Verify row 3
single-point-of-failure: the jq-program-extraction parity test (format_parity_test.go); the independent layer is windows-port/14's live Windows-leg smoke of `deskinbox walk` against a real repo (pending that brief's own re-scope — it currently names `deskinbox flow`, which is windows-port/15's deliverable, not this brief's; see windows-port/15).

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions.
- Stop at `implemented` — you do not set verified/done.
- Do not delete or edit the script (oracle); do not change the skill PROCEDURE, only the
  command it names.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Read the script end to end; write `tools/desk/cmd/deskinbox/testdata/spec.md` listing every
   mode, flag, the table/walk-vs-html/flow split rationale, and the mechanism divergences from
   the oracle — the implementer's contract, checked by the reviewer against the script.
2. Port the shared engine (repo resolution, label-filtered query, dedupe/rank/sort) and the
   `table` and `walk` modes, sharing ONE format builder the way the oracle shares
   write_format_program between its own `--walk` and `--html`; parity test against the
   extracted oracle jq program.
3. Re-point `plugins/assay/commands/inbox.md` and `plugins/assay/skills/ask-decision/SKILL.md`
   to the verb for table/walk; script stays the renderer (with a note, not silently) for
   html/flow until windows-port/15.
4. Changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go vet ./cmd/deskinbox/ && go test -count=1 ./cmd/deskinbox/` | exit 0 | `check` |
| 2 | **Parity walk**: `cd tools/desk && go test -count=1 -run 'TestParityWalk' -v ./cmd/deskinbox/ \| grep -c -- '--- PASS'` | `>= 1` (needs `jq` on the runner; could-not-check with reason otherwise) | `check +dereference` |
| 3 | No shell-outs in the verb: `grep -rn -e 'exec\.Command' -e '"make"' -e '"jq"' tools/desk/cmd/deskinbox/*.go \| grep -vc _test.go` | `0` | `check` |
| 4 | Windows build: `cd tools/desk && GOOS=windows GOARCH=amd64 go build ./cmd/deskinbox/` | exit 0 | `check` |
| 5 | Command + skill re-pointed for table/walk: `grep -c 'deskinbox' plugins/assay/commands/inbox.md plugins/assay/skills/ask-decision/SKILL.md` | `>= 1` each; `cd tools/skillslint && go test -count=1 ./...` exit 0 | `check` |
| 6 | **Flow — the skill's own example runs**: `deskinbox walk --item 1 medici-finance/assay; echo rc=$?` | `rc=0` and the block's Header/Context/Options/Reply/Verification match the script's on the same instant (`bash plugins/assay/scripts/assay-inbox.sh --walk --item 1 medici-finance/assay`), modulo the two tools' different exit-code taxonomies (testdata/spec.md divergence 4); needs a live minted token — could-not-check with reason otherwise | `check +flow` |
| 7 | Consumers routing corroborated: `statusgen --root . --consumers windows-port/13; echo $?` | `0` | `check` |
| 8 | Board lint: `statusgen --root . --lint` | `0` PROBLEMs | `check:ci` |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: **model**. Reviewer's questions: (1) does `tools/desk/cmd/deskinbox/testdata/spec.md`
account for every flag the script's `--help` prints, and for each of table/walk vs html/flow,
say which side of the split it is on? (2) does `format_parity_test.go` actually exercise the
REAL oracle jq program (not a hand-copied second expectation) across a representative set of
Context/Options shapes (heading found vs fallback, recommended reordering, blind detail,
unblocks-line dedupe)? (3) is any divergence from the oracle (identity, query mechanism,
comment-read scope, exit codes) recorded in testdata/spec.md rather than left implicit in the
diff?
