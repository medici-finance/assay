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
| 3 | No shell-outs in the verb: `! grep -rl -e 'exec\.Command' -e '"make"' -e '"jq"' tools/desk/cmd/deskinbox/*.go \| grep -qv _test.go` | exit 0 (no shipping-file match; a shell-out in a non-`_test.go` file exits 1) | `check` |
| 4 | Windows build: `cd tools/desk && GOOS=windows GOARCH=amd64 go build ./cmd/deskinbox/` | exit 0 | `check` |
| 5 | Command + skill re-pointed for table/walk: `grep -c 'deskinbox' plugins/assay/commands/inbox.md plugins/assay/skills/ask-decision/SKILL.md` | `>= 1` each; `cd tools/skillslint && go test -count=1 ./...` exit 0 | `check` |
| 6 | **Flow — the skill's own example runs**: `deskinbox walk --item 1 medici-finance/assay; echo rc=$?` | `rc=0` and the block's Header/Context/Options/Reply/Verification match the script's on the same instant (`bash plugins/assay/scripts/assay-inbox.sh --walk --item 1 medici-finance/assay`), modulo the two tools' different exit-code taxonomies (testdata/spec.md divergence 4); needs a live minted token — could-not-check with reason otherwise | `check +flow` |
| 7 | Consumers routing corroborated: `statusgen --root . --consumers windows-port/13; echo $?` | `0` | `check` |
| 8 | Board lint: `statusgen --root . --lint` | `0` PROBLEMs | `check:ci` |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

### Non-implementer verifier run — VERIFY: PASS — 2026-09-23 opus-5.5-verifier

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|------|--------|
| 1 | `cd tools/desk && go vet ./cmd/deskinbox/ && go test -count=1 ./cmd/deskinbox/` | exit 0 | exit 0 — `ok github.com/medici-finance/assay/tools/desk/cmd/deskinbox 0.758s` | 2026-09-23 | opus-5.5-verifier |
| 2 | `cd tools/desk && go test -count=1 -run 'TestParityWalk' -v ./cmd/deskinbox/ \| grep -c -- '--- PASS'` | >= 1 (needs jq) | exit 0, count = 5 — jq-1.8.2 present; TestParityWalk + 4 subtests all `--- PASS`, 0 SKIP (typical, no-headings fallback, blind detail, recommended-reorder) | 2026-09-23 | opus-5.5-verifier |
| 3 | `! grep -rl -e 'exec\.Command' -e '"make"' -e '"jq"' tools/desk/cmd/deskinbox/*.go \| grep -qv _test.go` | exit 0 (no shipping-file match) | exit 0 — only match is the test file format parity underscore-test.go (allowed); no shipping file shells out | 2026-09-23 | opus-5.5-verifier |
| 4 | `cd tools/desk && GOOS=windows GOARCH=amd64 go build ./cmd/deskinbox/` | exit 0 | exit 0 — Windows amd64 build succeeded | 2026-09-23 | opus-5.5-verifier |
| 5 | `grep -c 'deskinbox' plugins/assay/commands/inbox.md plugins/assay/skills/ask-decision/SKILL.md` then `cd tools/skillslint && go test -count=1 ./...` | >= 1 each; skillslint exit 0 | inbox.md = 11, ask-decision SKILL.md = 5 (both >= 1); skillslint exit 0 — `ok github.com/medici-finance/assay/tools/skillslint 1.523s` | 2026-09-23 | opus-5.5-verifier |
| 6 | `deskinbox walk --item 1 medici-finance/assay; echo rc=$?` vs `bash plugins/assay/scripts/assay-inbox.sh --walk --item 1 medici-finance/assay` | rc=0 and five-part block matches the oracle on the same instant; needs a live minted token — could-not-check with reason otherwise | could-not-check — needs a live minted App token and a live GitHub read; offline envelope (KUBECONFIG=/dev/null, no live infra) forbids the probe. verifyrun's exit=0 reflects only the trailing echo, not parity. The five-part format-builder output is proven byte-for-byte offline by row 2 (TestParityWalk against the oracle's own extracted jq program); the un-checked residue is the live gh->builder wiring, covered structurally by rows 1/2/4 plus query/table/main unit tests against a fake Forge | 2026-09-23 | opus-5.5-verifier |
| 7 | `statusgen --consumers windows-port/13` (with absolute --root) | exit 0 | exit 0 — `consumers: no brief files in the diff against 39866201ce48... — nothing to corroborate` (run at merged HEAD, empty diff; rc=0 = the expectation) | 2026-09-23 | opus-5.5-verifier |
| 8 | `statusgen --lint` (with absolute --root) | 0 PROBLEMs | exit 0, 0 PROBLEM lines, `LINT: PASS` (NOTICEs only, none naming windows-port/13) via direct non-hermetic run. The class:ci hermetic network-off witness is could-not-check on darwin (verifyrun row 8: needs Linux `unshare --net`); re-executed network-off by CI on a Linux runner | 2026-09-23 | opus-5.5-verifier |

RISK-VALUE (kit §4 — enumerate → rank → derive). This is a byte-for-byte PARITY port: the brief's contract (exec-tier-why) is that the Go builder reproduce the oracle's output exactly, so the authoritative source for every literal is the oracle's own line, and TestParityWalk enforces equality by extracting the oracle's real jq program. Enumeration over the diff scope (tools/desk/cmd/deskinbox shipping files) found only display/ordering literals; none is irreversible or a money/auth/settlement value.

- RISK-VALUE: DERIVED — labels = ["urgent","needs-decision","question","help wanted"] @ tools/desk/cmd/deskinbox/query.go:29 — the escalation-contract rank order (most-urgent-first); mirrors the oracle's `--argjson rankorder '["urgent","needs-decision","question","help wanted"]'` at assay-inbox.sh:380, which sets which decision surfaces first. Reversible ordering knob (re-run shows a different order; nothing irreversible).
- RISK-VALUE: DERIVED — rank fallback = 99 @ tools/desk/cmd/deskinbox/query.go:45 — the "carries none of the labels" sentinel; mirrors the oracle's `... | min // 99` at assay-inbox.sh:383. Reversible.
- RISK-VALUE: DERIVED — title truncation = 57 (runes) + "..." @ tools/desk/cmd/deskinbox/table.go:35-36 — mirrors the oracle's `if length > 57 then .[:57] + "..." else . end` at assay-inbox.sh:392. Reversible display knob.
- RISK-VALUE: DERIVED — context snippet truncation = 180 @ tools/desk/cmd/deskinbox/format.go:255 — mirrors the oracle's `... [0:180]` at assay-inbox.sh:477. Reversible display knob.
- RISK-VALUE: DERIVED — default walk item = 1 @ tools/desk/cmd/deskinbox/main.go:81 — the oracle prints item 1 by default; option lettering letterFor = ["A","B","C","D"] @ format.go:335 mirrors the oracle's per-option index. Reversible.
- Exit-code mapping (deskkit.ExitRefused=5, ExitUnverifiable=6 @ main.go via shared deskkit) and the issue cap 10,000 (forgeMaxIssuePages*forgeIssuePerPage, deskkit) are shared-package constants, not literals introduced by this diff, and are documented divergences (testdata/spec.md divergence 2 and 4); reversible.

Summary: RISK-VALUE all DERIVED; every value is a parity mirror of a named oracle source line, enforced by the extracted-jq-program parity test (row 2). No irreversible or hard-pinned-constraint value in scope.

Row 6 needs a live minted token and a live forge read; its parity content is proven offline by row 2. Row 8's hermetic Linux witness is still owed.


## Review
Gate: **model**. Reviewer's questions: (1) does `tools/desk/cmd/deskinbox/testdata/spec.md`
account for every flag the script's `--help` prints, and for each of table/walk vs html/flow,
say which side of the split it is on? (2) does `format_parity_test.go` actually exercise the
REAL oracle jq program (not a hand-copied second expectation) across a representative set of
Context/Options shapes (heading found vs fallback, recommended reordering, blind detail,
unblocks-line dedupe)? (3) is any divergence from the oracle (identity, query mechanism,
comment-read scope, exit codes) recorded in testdata/spec.md rather than left implicit in the
diff?
