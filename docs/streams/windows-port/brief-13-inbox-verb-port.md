---
brief: assay:assay:windows-port:13
title: assay-inbox.sh → a Go `deskinbox` verb (the inbox skill's 1,403-line bash + jq + make engine)
why: >-
  The `assay:inbox` skill is how a human or a desk walks the decision queue (`--walk`, `--flow`,
  `--html`) and the ask-decision skill calls it by name. It is the heaviest POSIX script in the
  plugin — 1,403 lines of bash calling gh twenty times, jq twenty-nine times, make four times and
  mktemp twenty-two times — and none of those exist on a native-Windows adopter's box. Every
  other desk surface now has a Windows path (binaries, install, Verify witness, and after brief
  11 the pollers); this is the one skill whose engine is still a shell script, so it is the one
  skill a Windows adopter cannot invoke at all.
wave: 4
depends: ["windows-port/00", "windows-port/01"]
unblocks: ["windows-port/14"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1435]
schema: brief-v2
authored: 2026-09-21 by the-desk (Bob) — windows-port authoring session, driver ask 2026-09-21
sources:
  - "plugins/assay/scripts/assay-inbox.sh (1,403 lines, #!/usr/bin/env bash; gh×20, jq×29, make×4, mktemp×22, find/date/awk/sed; ~79 bashism sites) — the engine; its `--help` and the ask-decision skill are the behavioural spec"
  - "plugins/assay/skills/ask-decision/SKILL.md:50,54,189 — `bash <bundle>/scripts/assay-inbox.sh --walk --item 1 …`, `--flow`, `--html /path/to/inbox.html` — the three entry points a Windows adopter must be able to call"
  - "plugins/assay/skills/inbox/SKILL.md (the skill body that names the script) — every invocation site to re-point"
  - "docs/streams/windows-port/portability-audit.md — the script is ABSENT from brief 02's table (windows-port/11 Task 1 adds the row; this brief resolves it)"
  - "tools/desk/cmd/deskboard — the forge read client and the JSON/table renderers the verb reuses rather than re-implementing"
  - "freshness-checked 2026-09-21 @ 56491ce (origin/main): no verb named deskinbox; the script present at 1,403 lines"
exec-tier: strong
exec-tier-why: >-
  Question (b): the port must reproduce the script's three modes byte-for-byte as consumed by
  the ask-decision skill and by humans reading the HTML; the `make` calls it makes are the
  cross-artifact part (which targets, with what cwd) and must be replaced by direct verb calls
  without changing what the human sees.
domain: complicated
parallel-streams:
  - {name: engine, files: ["tools/desk/cmd/deskinbox/**"]}
  - {name: skills, files: ["plugins/assay/skills/inbox/**", "plugins/assay/skills/ask-decision/**"]}
consumers:
  - "tools/desk/cmd/deskinbox/** (new verb: walk | flow | html): follow-up windows-port/13 (this brief)"
  - "plugins/assay/scripts/assay-inbox.sh: follow-up windows-port/13 (this brief — kept as the parity oracle until windows-port/14 retires it)"
  - "plugins/assay/skills/inbox/SKILL.md, plugins/assay/skills/ask-decision/SKILL.md:50,54,189 (name the verb; the script as fallback): follow-up windows-port/13 (this brief)"
  - "Makefile targets the script calls (`make` ×4 — list them at pickup): out-of-scope (unchanged; the verb calls the underlying tools directly)"
version: 1
id: 9f77c761-e720-401f-b96c-f3dbf45001df
---

# Brief 13 — assay-inbox.sh → `deskinbox`

## Context
files: tools/desk/cmd/deskinbox/{main.go,walk.go,flow.go,html.go,parity_test.go,testdata/**}, plugins/assay/skills/inbox/SKILL.md, plugins/assay/skills/ask-decision/SKILL.md, changelog/windows-port-13-deskinbox.md
facts:
- modes and their contracts (from the script's --help, re-read at pickup): `walk --item N <repos…>` (one decision item at a time, oldest first, prints the same block the script prints), `flow` (the pipeline-stage view), `html <out> <repos…>` (the static page)
- the four `make` invocations: identify each target at pickup and call the tool it wraps (a desk verb or statusgen) directly with the same arguments — the verb never shells to make
- parity: recorded fixtures of the gh responses the script consumes (httptest replay); `walk` and `flow` stdout diffed byte-for-byte; `html` diffed after normalising the generated timestamp line
- identity: the verb reads with the same identity rule the script uses (check its top: if it unsets role tokens like the monitors do, the verb does the same — test it)
- Windows: paths via filepath; scratch via os.CreateTemp; no `find`/`date` shell-outs
single-point-of-failure: the parity test on recorded fixtures; the independent layer is windows-port/14's live Windows-leg smoke of `deskinbox flow` against a real repo.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions.
- Stop at `implemented` — you do not set verified/done.
- Do not delete or edit the script (oracle); do not change the skill PROCEDURE, only the
  command it names.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Read the script end to end; write `tools/desk/cmd/deskinbox/testdata/spec.md` (planned) listing every mode, flag, and the four
   make targets with what they resolve to — the implementer's contract, checked by the
   reviewer against the script.
2. Port the three modes; parity tests on recorded fixtures.
3. Re-point the two skills to the verb, script as fallback.
4. Changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go vet ./cmd/deskinbox/ && go test -count=1 ./cmd/deskinbox/` | exit 0 | `check` |
| 2 | **Parity walk/flow**: `cd tools/desk && go test -count=1 -run 'TestParityWalk\|TestParityFlow' -v ./cmd/deskinbox/ \| grep -c -- '--- PASS'` | `>= 2` (needs bash + jq on the runner; could-not-check with reason otherwise) | `check +dereference` |
| 3 | **Parity html** (timestamp-normalised): `cd tools/desk && go test -count=1 -run 'TestParityHTML' ./cmd/deskinbox/` | PASS | `check +dereference` |
| 4 | No shell-outs in the verb: `git grep -n -e 'exec.Command' -e '"make"' -e '"jq"' HEAD -- tools/desk/cmd/deskinbox/ \| grep -vc _test.go` | `0` | `check` |
| 5 | Windows build: `cd tools/desk && GOOS=windows GOARCH=amd64 go build ./cmd/deskinbox/` | exit 0 | `check` |
| 6 | Skills re-pointed: `grep -c 'deskinbox' plugins/assay/skills/inbox/SKILL.md plugins/assay/skills/ask-decision/SKILL.md` | `>= 1` each; `cd tools/skillslint && go test -count=1 ./...` exit 0 | `check` |
| 7 | **Flow — the skill's own example runs**: `deskinbox walk --item 1 medici-finance/assay; echo rc=$?` | `rc=0` and the first block matches the script's on the same instant (`bash plugins/assay/scripts/assay-inbox.sh --walk --item 1 medici-finance/assay`) | `check +flow` |
| 8 | Consumers routing corroborated: `statusgen --root . --consumers windows-port/13; echo $?` | `0` | `check` |
| 9 | Board lint: `statusgen --root . --lint` | `0` PROBLEMs | `check:ci` |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: **model**. Reviewer's questions: (1) does `tools/desk/cmd/deskinbox/testdata/spec.md` (planned) account for every flag the
script's `--help` prints? (2) do the four make targets resolve to the same underlying tool
calls, with the same cwd semantics? (3) is any mode's output "close enough" rather than
byte-identical — and if so, is that recorded as a fixture change, not a silent drift?
