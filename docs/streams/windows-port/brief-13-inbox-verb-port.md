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
- Windows: paths via filepath; no shell-outs in this brief's own shipping files (main, forge, repos, query, table, detail, format, walk, summary) — proven by grep in Verify row 3. The package's one sanctioned subprocess, `flow.go`'s statusgen/deskboard readers, arrived later with windows-port/15, whose Verify row 5 scopes it
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
| 3 | `d=tools/desk/cmd/deskinbox; grep -l -e '"os/exec"' -e '"make"' -e '"jq"' $d/main.go $d/forge.go $d/repos.go $d/query.go $d/table.go $d/detail.go $d/format.go $d/walk.go $d/summary.go; test $? -eq 1` — no shell-outs in this brief's own shipping files | exit 0 — none of the nine files imports `os/exec` or names a `"make"`/`"jq"` literal. A match prints the offending file and fails the row; so does a missing file (grep's error status is not 1). Scope is this brief's files only: `flow.go`'s statusgen/deskboard exec belongs to windows-port/15 and is scoped by that brief's Verify row 5. Re-scoped per #1833 | `check` |
| 4 | Windows build: `cd tools/desk && GOOS=windows GOARCH=amd64 go build ./cmd/deskinbox/` | exit 0 | `check` |
| 5 | Command + skill re-pointed for table/walk: `grep -c 'deskinbox' plugins/assay/commands/inbox.md plugins/assay/skills/ask-decision/SKILL.md` | `>= 1` each; `cd tools/skillslint && go test -count=1 ./...` exit 0 | `check` |
| 6 | **Flow — live walk parity with the oracle**: `a=$(deskinbox walk --item 1 medici-finance/assay) && b=$(bash plugins/assay/scripts/assay-inbox.sh --walk --no-screen --item 1 medici-finance/assay) && grep -qx 'Verification' <<<"$a" && diff <(sed 's/^deskinbox: /assay-inbox: /' <<<"$a") <(printf '%s\n' "$b") && echo PARITY` | exit 0 and output is `PARITY` — both tools exit 0 on the same instant, the verb printed a five-part block, and that block (Header/Context/Options/Reply shape/Verification, plus the trailing item-count line with only the tool-name prefix normalised) is byte-identical to the oracle's; `diff` prints any divergence and fails the row. The oracle runs `--no-screen` because the verb does not screen, so a screened oracle numbers a different queue and appends a screen tail line. Needs a live minted token for the verb and a gh identity for the oracle: without them the verb exits 6 (unverifiable) and the row FAILS on that exit, never passes — record that as could-not-check with the reason | `check +flow` |
| 7 | Consumers routing corroborated: `t=$(mktemp -d) && trap 'rm -rf "$t"' EXIT && git clone -q --shared --no-checkout . "$t" && git -C "$t" checkout -q --detach b3fe2a1c7900 && o=$(statusgen --root "$t" --consumers --brief windows-port/13 --base b3fe2a1c7900^) && grep -E '^summary: [1-9][0-9]* corroborated, 0 disproved,' <<<"$o" && echo CORROBORATED` | exit 0 and output is `CORROBORATED`. The gate judges this brief's `consumers:` claims against the diff that made them: the implementing squash commit b3fe2a1c7900 (#1507) against its parent, checked out in a throwaway shared clone. On merged main this brief is in no diff, so the gate would have nothing to judge. `CORROBORATED` prints only when the gate judged this brief and its summary line reads one or more corroborated and 0 disproved; a DISPROVED claim (`exit 1`) or a could-not-check (`exit 2`) fails the row. A checkout without that commit (a shallow clone) is could-not-check with that reason, not a fail. The UNCHECKED residue at that commit is the reviewer's call (brief-rule 9): the `deskinbox/**` entry, whose windows-port/15 follow-up does not reference back, and the unchanged `assay-inbox.sh` oracle entry | `check` |
| 8 | Board lint: `statusgen --root . --lint` | `0` PROBLEMs | `check:ci` |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

### Non-implementer verifier run — VERIFY: BLOCKED — 5/8 pass, 3 could-not-check, 0 fail — 2026-09-23 claude-opus-4-8-verifier

Runner is not the implementer. Isolated detached worktree off `origin/main` at the merged head (HEAD == origin/main == `39866201ce48acdce1f9b14d1cae38eb2b7eff38`), offline envelope (`KUBECONFIG=/dev/null`), read-only. `gate: model`, all four risk answers `no`. Rows 1-5 pass offline; row 6 needs a live minted App token (offline-barred); row 7's consumers run on the fully merged tree corroborates nothing; row 8's `check:ci` hermetic witness is could-not-run on darwin (needs Linux `unshare --net`), the direct non-hermetic run passed.

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|------|--------|
| 1 | `cd tools/desk && go vet ./cmd/deskinbox/ && go test -count=1 ./cmd/deskinbox/` | exit 0 | PASS — exit 0; `ok github.com/medici-finance/assay/tools/desk/cmd/deskinbox 0.758s` | 2026-09-23 | claude-opus-4-8-verifier |
| 2 | `cd tools/desk && go test -count=1 -run 'TestParityWalk' -v ./cmd/deskinbox/ \| grep -c -- '--- PASS'` | >= 1 (needs jq) | PASS — exit 0, count = 5; jq-1.8.2 present; TestParityWalk + 4 subtests all `--- PASS`, 0 SKIP (typical, no-headings fallback, blind detail, recommended-reorder) | 2026-09-23 | claude-opus-4-8-verifier |
| 3 | `! grep -rl -e 'exec\.Command' -e '"make"' -e '"jq"' tools/desk/cmd/deskinbox/*.go \| grep -qv _test.go` | exit 0 (no shipping-file match) | PASS — exit 0; the only match is the test file `format_parity_test.go` (allowed); no shipping file shells out | 2026-09-23 | claude-opus-4-8-verifier |
| 4 | `cd tools/desk && GOOS=windows GOARCH=amd64 go build ./cmd/deskinbox/` | exit 0 | PASS — exit 0; Windows amd64 build succeeded | 2026-09-23 | claude-opus-4-8-verifier |
| 5 | grep -c 'deskinbox' plugins/assay/commands/inbox.md plugins/assay/skills/ask-decision/SKILL.md; cd tools/skillslint && go test -count=1 ./... | >= 1 each; skillslint exit 0 | PASS — inbox.md = 11, ask-decision SKILL.md = 5 (both >= 1); skillslint exit 0: `ok github.com/medici-finance/assay/tools/skillslint 1.523s` | 2026-09-23 | claude-opus-4-8-verifier |
| 6 | deskinbox walk --item 1 medici-finance/assay; echo rc=$? | rc=0 and the five-part block matches the oracle (`bash plugins/assay/scripts/assay-inbox.sh --walk --item 1 medici-finance/assay`) on the same instant; needs a live minted token — could-not-check with reason otherwise | COULD-NOT-CHECK — needs a live minted App token and a live GitHub read; the offline envelope (KUBECONFIG=/dev/null, no live infra) forbids the probe. verifyrun's exit=0 reflects only the trailing echo, not parity. The five-part format-builder output is proven byte-for-byte offline by row 2 (TestParityWalk against the oracle's own extracted jq program); the un-checked residue is the live gh->builder wiring, covered structurally by rows 1/2/4 plus query/table/main unit tests against a fake Forge | 2026-09-23 | claude-opus-4-8-verifier |
| 7 | statusgen --root . --consumers windows-port/13; echo $? | exit 0 | COULD-NOT-CHECK — merged tree; run at merged HEAD gives an empty diff so nothing is corroborated. Re-run at 39866201: exit 0, `consumers: no brief files in the diff against 39866201ce48... — nothing to corroborate` (nothing corroborated, nothing disproved) | 2026-09-23 | claude-opus-4-8-verifier |
| 8 | statusgen --root . --lint | 0 PROBLEMs | COULD-NOT-CHECK — hermetic witness owed (darwin); the check:ci hermetic network-off witness could-not-run on darwin (verifyrun row 8: needs Linux `unshare --net`), re-executed network-off by CI on a Linux runner. Direct non-hermetic run (supporting only): exit 0, 0 PROBLEM lines, `LINT: PASS` (NOTICEs only, none naming windows-port/13) | 2026-09-23 | claude-opus-4-8-verifier |

RISK-VALUE (kit §4 — enumerate → rank → derive). This is a byte-for-byte PARITY port: the brief's contract (exec-tier-why) is that the Go builder reproduce the oracle's output exactly, so the authoritative source for every literal is the oracle's own line, and TestParityWalk enforces equality by extracting the oracle's real jq program. Enumeration over the diff scope (tools/desk/cmd/deskinbox shipping files) found only display/ordering literals; none is irreversible or a money/auth/settlement value.

- RISK-VALUE: DERIVED — labels = ["urgent","needs-decision","question","help wanted"] @ tools/desk/cmd/deskinbox/query.go:29 — the escalation-contract rank order (most-urgent-first); mirrors the oracle's `--argjson rankorder '["urgent","needs-decision","question","help wanted"]'` at assay-inbox.sh:380, which sets which decision surfaces first. Reversible ordering knob (re-run shows a different order; nothing irreversible).
- RISK-VALUE: DERIVED — rank fallback = 99 @ tools/desk/cmd/deskinbox/query.go:45 — the "carries none of the labels" sentinel; mirrors the oracle's `... | min // 99` at assay-inbox.sh:383. Reversible.
- RISK-VALUE: DERIVED — title truncation = 57 (runes) + "..." @ tools/desk/cmd/deskinbox/table.go:35-36 — mirrors the oracle's `if length > 57 then .[:57] + "..." else . end` at assay-inbox.sh:392. Reversible display knob.
- RISK-VALUE: DERIVED — context snippet truncation = 180 @ tools/desk/cmd/deskinbox/format.go:255 — mirrors the oracle's `... [0:180]` at assay-inbox.sh:477. Reversible display knob.
- RISK-VALUE: DERIVED — default walk item = 1 @ tools/desk/cmd/deskinbox/main.go:81 — the oracle prints item 1 by default; option lettering letterFor = ["A","B","C","D"] @ format.go:335 mirrors the oracle's per-option index. Reversible.
- Exit-code mapping (deskkit.ExitRefused=5, ExitUnverifiable=6 @ main.go via shared deskkit) and the issue cap 10,000 (forgeMaxIssuePages*forgeIssuePerPage, deskkit) are shared-package constants, not literals introduced by this diff, and are documented divergences (testdata/spec.md divergence 2 and 4); reversible.

Summary: RISK-VALUE all DERIVED; every value is a parity mirror of a named oracle source line, enforced by the extracted-jq-program parity test (row 2). No irreversible or hard-pinned-constraint value in scope.

Row 6 needs a live minted token and a live forge read; its parity content is proven offline by row 2. Row 8's hermetic Linux witness is still owed.

### Non-implementer verifier re-run — VERIFY: FAIL — 6/8 pass, 1 fail (row 6), 1 could-not-run (row 8) — 2026-09-27 claude-opus-5-5-verifier

Runner is not the implementer. Isolated detached worktree at merged main `b227b40768db08a0a91046899bc1877cf3c6d1ec` (HEAD == origin/main == the forge's main), `KUBECONFIG=/dev/null`, host darwin. Execution witness below is `statusgen verifyrun` v1.0.27 (non-dry). Re-run of the 2026-09-23 BLOCKED pass (held rows 6, 7, 8): since then the brief and every shipping `deskinbox` file except `forge.go` are byte-identical (the `forge.go` delta is the deskkit credential-routing change of #1587, a one-line seam rename); the two skill surfaces changed under attention-budget/15 (#1689, the screen). This pass ran row 6 live with a minted read token instead of holding it.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go vet ./cmd/deskinbox/ && go test -count=1 ./cmd/deskinbox/` | pass exit=0 | sha256:0beb8cea3135 | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test -count=1 -run 'TestParityWalk' -v ./cmd/deskinbox/ \| grep -c -- '--- PASS'` | pass exit=0 | sha256:f0b5c2c2211c | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 3 | `! grep -rl -e 'exec\.Command' -e '"make"' -e '"jq"' tools/desk/cmd/deskinbox/*.go \| grep -qv _test.go` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && GOOS=windows GOARCH=amd64 go build ./cmd/deskinbox/` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 5 | `grep -c 'deskinbox' plugins/assay/commands/inbox.md plugins/assay/skills/ask-decision/SKILL.md` | pass exit=0 | sha256:680b1617cf1a | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 6 | `deskinbox walk --item 1 medici-finance/assay; echo rc=$?` | pass exit=0 | sha256:983af9c7d089 | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 7 | `statusgen --root . --consumers windows-port/13; echo $?` | pass exit=0 | sha256:37de28eccd46 | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --root . --lint` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |

Per-row key output (runner claude-opus-5-5-verifier, 2026-09-27; the witness table's Result for rows 6 and 7 is exit-status only):

- Row 1: PASS — exit 0; `ok github.com/medici-finance/assay/tools/desk/cmd/deskinbox 4.917s`
- Row 2: PASS — exit 0, count = 5 (jq-1.8.2); parent + 4 subtests PASS, 0 SKIP. The extracted oracle program still matches after #1689 reshaped the script
- Row 3: PASS — exit 0; only match is format_parity_test.go (a test file, allowed)
- Row 4: PASS — exit 0; artifact is `PE32+ executable (console) x86-64, for MS Windows` (cross-compiled on darwin; a native-Windows run of the binary is out of reach on this host — that is windows-port/14's leg)
- Row 5: PASS — ask-decision SKILL.md = 7, inbox.md = 13; skillslint exit 0 `ok github.com/medici-finance/assay/tools/skillslint 4.576s`
- Row 6: FAIL — rc=0 on both tools (verb built from merged main, `DESK_LOOP=verify-desk`, minted verifier read token; oracle run with `--no-screen` so both number the same unscreened queue; both reported 73 items). Header, Options, Reply shape and Verification are identical; Context differs: the verb prints `latest desk note (assay-desk-app[bot])`, the oracle `latest desk note (assay-desk-app)`. Over items 1-20: 12 byte-identical, 8 differ in Context, and on one (item 10, #1193) the two tools pick a DIFFERENT comment as the desk note (verb: a worker-App comment; oracle: a desk-App comment)
- Row 7: PASS — exit 0; at merged main `nothing to corroborate` (empty diff). Supporting run at the implementing squash commit b3fe2a1c7 against its parent (flags before the positional, `--consumers --base <parent>`): windows-port/13 = 1 CORROBORATED, 2 UNCHECKED, 0 DISPROVED, exit 0
- Row 8: COULD-NOT-RUN (hermetic) — check:ci network-off witness needs Linux `unshare --net`; host is darwin. Direct non-hermetic run (supporting only): exit 0, 0 PROBLEM lines, `LINT: PASS`; NOTICEs naming this brief are the consumers one-way-coverage note and an ordering-gate prose note, both advisory

**Row 6 — why it fails, and that it is a real defect, not a stale row.** The oracle selects the desk note with `test("\\[bot\\]$|desk"; "i")` over `gh issue view --json comments`, whose `author.login` for a GitHub App carries NO `[bot]` suffix; the verb reads comments over REST (detail.go), whose `user.login` for the same App is `<slug>[bot]`. Same regex (`desknoteAuthorRe` at format.go:76), different wire shape, so (a) the author label renders differently whenever the note is an App's, and (b) any App whose slug lacks `desk` (the worker, reviewer, verifier Apps) is a desk note to the verb but not to the oracle, so the two tools can quote different comments. TestParityWalk cannot see this by construction — it feeds identical fixture input to both sides and is "independent of gh's wire format entirely" (testdata/spec.md) — and spec.md's divergence 3 (comment bodies over REST) says the reader change is "not a narrowing in practice" without recording the login-shape difference. This is exactly the live gh-to-builder residue the 2026-09-23 pass named as un-checked. Fix direction is the implementer's/reviewer's call: normalise the REST login (strip `[bot]`) for parity, or keep it, fix the oracle's selection, and record it as divergence 5. windows-port/15 (open PR #1684) reuses the same detail reader for `--html`.

**Observation — part of Task 3 was later reversed on purpose.** attention-budget/15 (#1689) re-pointed `--walk` in inbox.md and the ask-decision skill back to the bash oracle as the primary renderer (only the oracle runs the screen), with `deskinbox walk` named as the fallback where the oracle cannot run. Row 5 still passes (the verb is still named) and this is a later brief's deliberate change, not a defect of this one; but the "skill's own example" row 6 names is now the oracle's, and under the screen `--item K` numbers a different queue than `deskinbox walk --item K`.

**Risk-bearing values** (kit §4; gate model, all four risk answers `no`, no risk-classed path in this brief's diff scope — enumeration carried forward from the 2026-09-23 pass and re-pointed at current lines; the oracle's lines moved under #1689). Enumerated over the shipping files of tools/desk/cmd/deskinbox: the rank label list, the no-label rank sentinel, the title truncation, the snippet truncation, the default walk item, the option letters, the desk-note author regex, and the comment page size. All are reversible display/ordering knobs; none moves money, identity or authority.

- RISK-VALUE: DERIVED — labels = ["urgent","needs-decision","question","help wanted"] @ tools/desk/cmd/deskinbox/query.go:29 — mirrors the oracle's `rankorder` at assay-inbox.sh:552, the escalation vocabulary's most-urgent-first order. Reversible.
- RISK-VALUE: DERIVED — best = 99 @ tools/desk/cmd/deskinbox/query.go:45 — the no-escalation-label sentinel, mirrors `min // 99` at assay-inbox.sh:555. Reversible.
- RISK-VALUE: DERIVED — title cut = 57 runes + "..." @ tools/desk/cmd/deskinbox/table.go:35-36 — mirrors `length > 57` at assay-inbox.sh:564. Reversible.
- RISK-VALUE: DERIVED — snippet cut = 180 @ tools/desk/cmd/deskinbox/format.go:255 — mirrors `[0:180]` at assay-inbox.sh:684. Reversible.
- RISK-VALUE: DERIVED — walkItem = 1 @ tools/desk/cmd/deskinbox/main.go:81; letterFor = ["A","B","C","D"] @ format.go:335 — the oracle's default item and at-most-four lettering. Reversible.
- RISK-VALUE: NAMED, NOT DERIVED — desknoteAuthorRe = `(?i)\[bot\]$|desk` @ tools/desk/cmd/deskinbox/format.go:76 — textually identical to the oracle's regex at assay-inbox.sh:681, but a literal copy is not parity when the two inputs differ in shape (row 6). Whether the right literal is this one over a normalised login, or a changed one with a recorded divergence, is open — routed with the row 6 failure.
- per_page = 100 @ tools/desk/cmd/deskinbox/detail.go:83 — pagination size of the comment reader; operational knob, ranks last, no derivation owed.

VERIFY: FAIL — row 6 (live parity: Context's desk-note author and, on some items, the chosen desk-note comment diverge from the oracle); row 8 hermetic witness still owed on a Linux runner. Rows 1-5 and 7 pass.
### Non-implementer verifier re-run — VERIFY: PASS — 8/8 rows pass by hand; witness row 8 could-not-run on darwin (hermetic lane owed to a Linux runner) — 2026-09-30 claude-opus-5-5

**What moved since the last run (2026-09-27, VERIFY: FAIL on row 6).** The row 6 failure was a code defect and has been fixed: #1821 (for #1797) makes the REST comment reader strip the trailing `[bot]` from App logins (`ghLogin` in tools/desk/cmd/deskinbox/detail.go). The desk-note selector and its label now receive the same login shape the oracle gets from gh. The divergence is recorded in testdata/spec.md, and TestParityWalk now feeds both wire shapes (row 2 now shows 6 subtests, up from 4). Three Verify rows were re-authored after that run: row 3 was scoped to the brief's nine shipping files (#1858), row 7 now judges consumers against the implementing squash commit b3fe2a1c7900 (#1903), and row 6 now runs a real diff against the oracle instead of ending in a bare echo (#1887). The two earlier Evidence blocks ran different commands for rows 3, 6 and 7.

Runner is not the implementer. The worktree is detached at merged main `2adf73c2a791bad8d37be1c7ffe9792731506d5b`, which was HEAD, origin/main and the forge's main when these rows ran. It sits one STATUS.md-only regen commit after `b5e53a6a2dbc`, the merge of #1887. Environment: `KUBECONFIG=/dev/null`, host darwin/arm64, go1.27.1, jq-1.8.2. `statusgen` and `deskinbox` were built from this merged main's source and put first on PATH. The only network use was row 6's read-only forge reads: the verb used the verifier role's read token and the oracle used an existing read-only gh credential. Nothing was written to the forge.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd tools/desk && go vet ./cmd/deskinbox/ && go test -count=1 ./cmd/deskinbox/` | exit 0 | Row 1: PASS. exit 0; `ok github.com/medici-finance/assay/tools/desk/cmd/deskinbox 6.521s` | 2026-09-30 | assay-verifier-app[bot] @ 2adf73c2a791 (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test -count=1 -run 'TestParityWalk' -v ./cmd/deskinbox/ \| grep -c -- '--- PASS'` | >= 1 (needs jq) | Row 2: PASS. exit 0, count = 7 (jq-1.8.2 present). TestParityWalk and all 6 subtests `--- PASS`, 0 SKIP, 0 FAIL: typical, desk-App note before a non-desk App comment (both wire shapes), desk-App note latest with gh login label, no-headings fallback, blind detail, recommended reorder | 2026-09-30 | assay-verifier-app[bot] @ 2adf73c2a791 (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | `d=tools/desk/cmd/deskinbox; grep -l -e '"os/exec"' -e '"make"' -e '"jq"' $d/main.go $d/forge.go $d/repos.go $d/query.go $d/table.go $d/detail.go $d/format.go $d/walk.go $d/summary.go; test $? -eq 1` | exit 0, no file printed | Row 3: PASS. exit 0; grep printed nothing, so none of the nine shipping files imports os/exec or names a make/jq literal, and none is missing | 2026-09-30 | assay-verifier-app[bot] @ 2adf73c2a791 (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | `cd tools/desk && GOOS=windows GOARCH=amd64 go build ./cmd/deskinbox/` | exit 0 | Row 4: PASS. exit 0; artifact is `PE32+ executable (console) x86-64, for MS Windows`. It was cross-compiled on darwin; running the binary natively on Windows is the Windows-leg job of windows-port/14 | 2026-09-30 | assay-verifier-app[bot] @ 2adf73c2a791 (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | `grep -c 'deskinbox' plugins/assay/commands/inbox.md plugins/assay/skills/ask-decision/SKILL.md` then `cd tools/skillslint && go test -count=1 ./...` | >= 1 each; skillslint exit 0 | Row 5: PASS. ask-decision SKILL.md = 10, inbox.md = 37 (both >= 1); skillslint exit 0, `ok github.com/medici-finance/assay/tools/skillslint 2.545s` | 2026-09-30 | assay-verifier-app[bot] @ 2adf73c2a791 (claude-opus-5-5) (on-behalf-of human:ian) |
| 6 | `a=$(deskinbox walk --item 1 medici-finance/assay) && b=$(bash plugins/assay/scripts/assay-inbox.sh --walk --no-screen --item 1 medici-finance/assay) && grep -qx 'Verification' <<<"$a" && diff <(sed 's/^deskinbox: /assay-inbox: /' <<<"$a") <(printf '%s\n' "$b") && echo PARITY` | exit 0, output `PARITY` | Row 6: PASS. exit 0, output `PARITY` (run under bash as authored; `deskinbox` on PATH is the merged-main build under the verifier role, and the oracle reads with an existing gh read credential). diff printed nothing. The verb's trailing line reads `deskinbox: 82 item(s) across 1 repo(s)`. Supporting sweep over the same row: the same diff over items 1 to 20 gave 20 byte-identical, 0 differing, and both tools exited 0 on every item. The last run had 8 of 20 differing | 2026-09-30 | assay-verifier-app[bot] @ 2adf73c2a791 (claude-opus-5-5) (on-behalf-of human:ian) |
| 7 | `t=$(mktemp -d) && trap 'rm -rf "$t"' EXIT && git clone -q --shared --no-checkout . "$t" && git -C "$t" checkout -q --detach b3fe2a1c7900 && o=$(statusgen --root "$t" --consumers --brief windows-port/13 --base b3fe2a1c7900^) && grep -E '^summary: [1-9][0-9]* corroborated, 0 disproved,' <<<"$o" && echo CORROBORATED` | exit 0, output `CORROBORATED` | Row 7: PASS. exit 0; `summary: 1 corroborated, 0 disproved, 2 unchecked, 0 brief(s) claiming nothing` then `CORROBORATED`. The corroborated entry is the inbox.md / ask-decision SKILL.md claim. The two unchecked entries are the deskinbox glob (its windows-port/15 follow-up does not reference back) and the unchanged oracle script, exactly the residue the Expect column hands to the reviewer | 2026-09-30 | assay-verifier-app[bot] @ 2adf73c2a791 (claude-opus-5-5) (on-behalf-of human:ian) |
| 8 | `(cd statusgen && go build -o ../.vbin/statusgen .) && PATH="$PWD/.vbin:$PATH" statusgen --root "$(git rev-parse --show-toplevel)" --lint` | 0 PROBLEMs | Row 8: PASS. exit 0, 0 PROBLEM lines, `LINT: PASS`. The NOTICEs naming this brief are advisory: a one-way consumers note (windows-port/15 does not reference back) and ordering-gate prose notes | 2026-09-30 | assay-verifier-app[bot] @ 2adf73c2a791 (claude-opus-5-5) (on-behalf-of human:ian) |
| 8b | `(cd statusgen && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o ../.vbin/statusgen-linux .) && docker run --rm --network none -v "$PWD":/work -w /work alpine:3.21 /work/.vbin/statusgen-linux --root /work --lint` | 0 PROBLEMs, network off | Row 8, network-off form in a local Linux container: PASS. exit 0, 0 PROBLEM lines, `LINT: PASS`. The image has no git, so the done-gate for missing witnesses ran degraded (it printed a NOTICE that origin/main could not be resolved). The darwin run above has git and also found 0 PROBLEMs. This supports row 8 only; it is not the verifyrun hermetic witness | 2026-09-30 | assay-verifier-app[bot] @ 2adf73c2a791 (claude-opus-5-5) (on-behalf-of human:ian) |

Execution witness below: `statusgen verifyrun --dry-run`, built from this merged main's source, run on a clean tree. The table is exactly as emitted.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go vet ./cmd/deskinbox/ && go test -count=1 ./cmd/deskinbox/` | pass exit=0 | sha256:91587450a3d5 | 2026-09-30 | assay-verifier-app[bot] @ 2adf73c2a791 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test -count=1 -run 'TestParityWalk' -v ./cmd/deskinbox/ \| grep -c -- '--- PASS'` | pass exit=0 | sha256:10159baf262b | 2026-09-30 | assay-verifier-app[bot] @ 2adf73c2a791 (on-behalf-of human:ian) (forge-identity) |
| 3 | `d=tools/desk/cmd/deskinbox; grep -l -e '"os/exec"' -e '"make"' -e '"jq"' $d/main.go $d/forge.go $d/repos.go $d/query.go $d/table.go $d/detail.go $d/format.go $d/walk.go $d/summary.go; test $? -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-30 | assay-verifier-app[bot] @ 2adf73c2a791 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && GOOS=windows GOARCH=amd64 go build ./cmd/deskinbox/` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-30 | assay-verifier-app[bot] @ 2adf73c2a791 (on-behalf-of human:ian) (forge-identity) |
| 5 | `grep -c 'deskinbox' plugins/assay/commands/inbox.md plugins/assay/skills/ask-decision/SKILL.md` | pass exit=0 | sha256:10ebd0e57c74 | 2026-09-30 | assay-verifier-app[bot] @ 2adf73c2a791 (on-behalf-of human:ian) (forge-identity) |
| 6 | `a=$(deskinbox walk --item 1 medici-finance/assay) && b=$(bash plugins/assay/scripts/assay-inbox.sh --walk --no-screen --item 1 medici-finance/assay) && grep -qx 'Verification' <<<"$a" && diff <(sed 's/^deskinbox: /assay-inbox: /' <<<"$a") <(printf '%s\n' "$b") && echo PARITY` | pass exit=0 | sha256:e944104f9aca | 2026-09-30 | assay-verifier-app[bot] @ 2adf73c2a791 (on-behalf-of human:ian) (forge-identity) |
| 7 | `t=$(mktemp -d) && trap 'rm -rf "$t"' EXIT && git clone -q --shared --no-checkout . "$t" && git -C "$t" checkout -q --detach b3fe2a1c7900 && o=$(statusgen --root "$t" --consumers --brief windows-port/13 --base b3fe2a1c7900^) && grep -E '^summary: [1-9][0-9]* corroborated, 0 disproved,' <<<"$o" && echo CORROBORATED` | pass exit=0 | sha256:beb6fe86eee8 | 2026-09-30 | assay-verifier-app[bot] @ 2adf73c2a791 (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --root . --lint` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-30 | assay-verifier-app[bot] @ 2adf73c2a791 (on-behalf-of human:ian) (forge-identity) |

**Scope traceability.** Hand rows 1 to 8 each discharge the Verify row with the same number. Row 8b and the item 1 to 20 sweep in row 6 are supporting runs of rows 8 and 6. No verified work falls outside the Verify table.

**Risk-bearing values** (kit §4). The item is gate model; all four risk answers are `no`, and irreversible is `no`. No risk-classed path is in the diff scope. Enumeration covered the shipping files of tools/desk/cmd/deskinbox at this head, together with the #1821 fix diff (which adds no literal: it adds a `TrimSuffix` of the fixed marker `[bot]` and leaves the shared regex unchanged). Every entry is a display or ordering knob that an edit and a redeploy can undo. The top-ranked entries mirror oracle source lines, which is this brief's parity contract (exec-tier-why).

- RISK-VALUE: DERIVED — labels = ["urgent","needs-decision","question","help wanted"] @ tools/desk/cmd/deskinbox/query.go:29. Mirrors the oracle's rankorder at plugins/assay/scripts/assay-inbox.sh:552, the escalation vocabulary's most-urgent-first order.
- RISK-VALUE: DERIVED — best = 99 @ tools/desk/cmd/deskinbox/query.go:45. The rank sentinel for an item with no escalation label; mirrors `min // 99` at assay-inbox.sh:555.
- RISK-VALUE: DERIVED — title cut = 57 runes + "..." @ tools/desk/cmd/deskinbox/table.go:35-36. Mirrors `length > 57` at assay-inbox.sh:564.
- RISK-VALUE: DERIVED — snippet cut = 180 @ tools/desk/cmd/deskinbox/format.go:255. Mirrors `[0:180]` at assay-inbox.sh:684.
- RISK-VALUE: DERIVED — walkItem = 1 @ tools/desk/cmd/deskinbox/main.go:92; letterFor = ["A","B","C","D"] @ tools/desk/cmd/deskinbox/format.go:335. The oracle's default item and its at-most-four lettering.
- RISK-VALUE: DERIVED — desknoteAuthorRe = `(?i)\[bot\]$|desk` @ tools/desk/cmd/deskinbox/format.go:76, with ghLogin stripping the `[bot]` suffix @ tools/desk/cmd/deskinbox/detail.go:130. The last run left this NAMED, NOT DERIVED because the literal matched the oracle's regex (assay-inbox.sh:681) while the two sides received logins in different shapes. With the REST login normalised to gh's bare-slug shape, both sides now apply the same regex to the same input. That is proven offline by TestParityWalk's two wire-shape subtests (row 2) and live by row 6 (20 of 20 items byte-identical).
- per_page = 100 @ tools/desk/cmd/deskinbox/detail.go:84. Page size of the comment reader; an operational knob that ranks last and owes no derivation.

**Note for the desk.** All eight Verify rows pass as run by hand at this head. In the `statusgen verifyrun` witness, row 8 (`check:ci`) is could-not-run, because the host is darwin and lacks `unshare --net`. The same lint ran network-off in a Linux container (row 8b) and passed, but that is not the witness's own sandbox. A Linux runner still owes the hermetic witness if the verified flip requires it.

rows_passed=8 rows_total=8
RISK-VALUE: DERIVED — desknoteAuthorRe = `(?i)\[bot\]$|desk` @ tools/desk/cmd/deskinbox/format.go:76 (+ ghLogin @ detail.go:130) — both sides now see one login shape; the rest are DERIVED parity mirrors listed above

VERIFY: PASS

## Review
Gate: **model**. Reviewer's questions: (1) does `tools/desk/cmd/deskinbox/testdata/spec.md`
account for every flag the script's `--help` prints, and for each of table/walk vs html/flow,
say which side of the split it is on? (2) does `format_parity_test.go` actually exercise the
REAL oracle jq program (not a hand-copied second expectation) across a representative set of
Context/Options shapes (heading found vs fallback, recommended reordering, blind detail,
unblocks-line dedupe)? (3) is any divergence from the oracle (identity, query mechanism,
comment-read scope, exit codes) recorded in testdata/spec.md rather than left implicit in the
diff?
