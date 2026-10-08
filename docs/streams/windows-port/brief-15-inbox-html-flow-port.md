---
brief: assay:assay:windows-port:15
title: deskinbox html + flow — the self-contained page renderer and the pipeline-flow model (split from windows-port/13)
why: >-
  windows-port/13 ported the oracle's shared engine and its two most-used renderings
  (`table`, `walk` — the `ask-decision` skill's entry point) and split off the remaining two
  renderings as too large for one PR (dispatch's own pre-authorization: "if it's too big for
  one PR, STOP and split ... keeping only the piece you were mid-implementing"). Those two —
  `--html` (the self-contained decision-queue page, PLUS the Flow section) and `--flow`/
  `--flow --html` (the pipeline flow model: statusgen/deskboard readers, a terminal table, and
  an inline-SVG stage diagram) — are a materially different, larger piece of work: a new
  reader (statusgen --bottleneck/--intake-debt/--net-flow, deskboard throughput) and an
  inline-SVG diagram builder, neither of which table/walk touch. windows-port/14's Windows
  leg (`deskinbox flow medici-finance/assay`) depends on THIS brief's deliverable, not
  windows-port/13's.
wave: 5
depends: ["windows-port/13"]
unblocks: ["windows-port/14"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1435]
schema: brief-v2
authored: 2026-09-22 by a windows-port/13 worker session, on a STOP-and-split (dispatch's own pre-authorization) — driver ask 2026-09-21 (the original brief-13 authoring session)
sources:
  - "plugins/assay/scripts/assay-inbox.sh (assay-inbox.sh:594-1233 — write_html_program, the flow reader collect_flow/write_flow_program, render_flow_text) — the engine for these two modes; its `--help` and plugins/assay/commands/inbox.md are the behavioural spec"
  - "tools/desk/cmd/deskinbox/{query.go,forge.go,format.go,detail.go} (windows-port/13, done) — the shared engine and format builder this brief REUSES rather than re-implements; walk.go is the pattern a html.go follows for card rendering"
  - "tools/desk/cmd/deskinbox/testdata/spec.md (windows-port/13) — the mode/flag contract and the split rationale, written at 13's pickup"
  - "plugins/assay/commands/inbox.md, plugins/assay/skills/ask-decision/SKILL.md (windows-port/13 re-pointed the table/walk invocations; the --html and --flow invocations there still name the bash oracle — this brief re-points those)"
  - "docs/streams/windows-port/brief-14-windows-leg-proves-desk-role-paths.md:56 — the Windows leg step `deskinbox flow medici-finance/assay` this brief's deliverable must satisfy"
  - "freshness-checked 2026-09-22 (this worktree, mid windows-port/13): deskinbox table+walk implemented; no html/flow mode on the verb"
exec-tier: strong
exec-tier-why: >-
  Question (b): the flow model is DERIVED from four separate JSON readers
  (statusgen --bottleneck/--intake-debt/--net-flow, deskboard throughput) whose per-cell and
  fleet-aggregation rules (assay-inbox.sh:857-1231) must be reproduced exactly — including the
  could-not-check-vs-n/a distinction and the fleet-row "AT LEAST" partial-sum flag — and the
  html renderer must produce a self-contained page (no url(), no external asset) that the
  oracle's own test suite already asserts as a property, which this port must keep proving.
domain: complicated
parallel-streams:
  - {name: html, files: ["tools/desk/cmd/deskinbox/html.go", "tools/desk/cmd/deskinbox/html_test.go"]}
  - {name: flow, files: ["tools/desk/cmd/deskinbox/flow.go", "tools/desk/cmd/deskinbox/flow_test.go"]}
consumers:
  - "tools/desk/cmd/deskinbox/{html.go,flow.go} (new modes: html | flow | flow --html): follow-up windows-port/15 (this brief)"
  - "plugins/assay/scripts/assay-inbox.sh: follow-up windows-port/15 (this brief — kept as the parity oracle until windows-port/14 retires it)"
  - "plugins/assay/commands/inbox.md, plugins/assay/skills/ask-decision/SKILL.md (name the verb for html/flow; drop the 'not yet ported' notes windows-port/13 left): follow-up windows-port/15 (this brief)"
version: 1
---

# Brief 15 — `deskinbox html` + `deskinbox flow` (split from windows-port/13)

## Context
files: tools/desk/cmd/deskinbox/{html.go,flow.go,flow_parity_test.go,html_parity_test.go,testdata/**}, plugins/assay/commands/inbox.md, plugins/assay/skills/ask-decision/SKILL.md, changelog/windows-port-15-deskinbox-html-flow.md
facts:
- `html` reuses windows-port/13's fetchQueue + buildRendered for the cards (the SAME format builder walk.go already renders — the oracle shares write_format_program between its own `--walk` and `--html` and this port must keep that property, per windows-port/13's testdata/spec.md), then adds the self-contained-page wrapper (inline CSS, no `<script>`, no `src=`, no `@import`, no `url()`) plus the Flow section
- `flow` is a SEPARATE reader (assay-inbox.sh:857-1233): per-cell `statusgen --bottleneck/--intake-debt/--net-flow --json` (one invocation per root — statusgen refuses multi-root for these) plus ONE fleet-wide `deskboard throughput --json`; a reader that fails leaves its stage `could-not-check`, NEVER `0`
- the stage model (7 stages: intake/todo/in-progress/review/implemented/verified/done), the COUNT-vs-QUEUE distinction, and the fleet-row "AT LEAST" partial-sum flag are DERIVED rules from the oracle's write_flow_program jq (assay-inbox.sh:1017-1232) — port the rules, not just the shape
- `flow --html` renders the SAME flow model as an inline-SVG stage diagram (assay-inbox.sh:598-748) — an equivalent `<table>` must follow it with the same numbers (a diagram a screen reader cannot read is half the audience not served, per the oracle's own comment)
- readers are invoked via `os/exec` on `statusgen`/`deskboard` binaries (the oracle's own `ASSAY_STATUSGEN`/`ASSAY_DESKBOARD` env overrides, default: on PATH) — this is the ONE place in `deskinbox` that legitimately shells a subprocess, because the flow model's readers are OTHER desk binaries, not `gh`/`jq`/`make`; the no-shell-outs Verify row from windows-port/13 must be re-scoped here to name statusgen/deskboard specifically, not banned outright
single-point-of-failure: the flow-model parity test against the extracted oracle jq program (write_flow_program), run over canned reader-output fixtures (no live statusgen/deskboard call needed for that test, mirroring windows-port/13's format_parity_test.go approach); the independent layer is windows-port/14's live Windows-leg run of `deskinbox flow` against a real repo with real statusgen/deskboard binaries.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions.
- Stop at `implemented` — you do not set verified/done.
- Do not delete or edit the script (oracle); do not change the skill PROCEDURE, only the
  command it names.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Read assay-inbox.sh:594-1233 end to end; extend `tools/desk/cmd/deskinbox/testdata/spec.md`
   with the html/flow contract (page structure, the 7-stage model, the reader invocations) —
   checked by the reviewer against the script.
2. Port `html` (reuses windows-port/13's query+format engine; adds the page wrapper + Flow
   section).
3. Port `flow` and `flow --html` (the reader, the stage-model interpreter, the terminal table,
   the inline-SVG diagram).
4. Re-point `plugins/assay/commands/inbox.md` and `plugins/assay/skills/ask-decision/SKILL.md`'s
   remaining `--html`/`--flow` invocations to the verb; drop windows-port/13's "not yet ported"
   notes.
5. Changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go vet ./cmd/deskinbox/ && go test -count=1 ./cmd/deskinbox/` | exit 0 | `check` |
| 2 | **Parity flow**: `cd tools/desk && go test -count=1 -run 'TestParityFlow' -v ./cmd/deskinbox/ \| grep -c -- '--- PASS'` | `>= 1` (needs `jq` on the runner; could-not-check with reason otherwise) | `check +dereference` |
| 3 | **Parity html** (timestamp-normalised): `cd tools/desk && go test -count=1 -run 'TestParityHTML' ./cmd/deskinbox/` | PASS | `check +dereference` |
| 4 | Self-contained page: `deskinbox html /tmp/inbox-check.html medici-finance/assay && ! grep -q -e 'url(' -e '<script' -e ' src=' /tmp/inbox-check.html` | exit 0 (no external ref; any `url(`, `<script`, or ` src=` exits 1) | `check` |
| 5 | No unscoped shell-outs: `! grep -rn 'exec\.Command' tools/desk/cmd/deskinbox/*.go \| grep -v _test.go \| grep -qv -e statusgen -e deskboard` | exit 0 (every shipping `exec.Command` site names statusgen or deskboard; an unscoped site exits 1) | `check` |
| 6 | Windows build: `cd tools/desk && GOOS=windows GOARCH=amd64 go build ./cmd/deskinbox/` | exit 0 | `check` |
| 7 | Command + skill re-pointed for html/flow: `grep -c 'deskinbox' plugins/assay/commands/inbox.md plugins/assay/skills/ask-decision/SKILL.md` | `>=` windows-port/13's own counts (strictly more references, not fewer) | `check` |
| 8 | **Flow — the skill's own example runs**: `deskinbox flow medici-finance/assay; echo rc=$?` and `bash plugins/assay/scripts/assay-inbox.sh --flow medici-finance/assay; echo rc=$?` on the same instant | the bottleneck line and the per-stage counts agree, modulo the two tools' different exit-code taxonomies (windows-port/13's testdata/spec.md divergence 4); needs live statusgen/deskboard binaries and a live minted token — could-not-check with reason otherwise | `check +flow` |
| 9 | Consumers routing corroborated: `statusgen --root . --consumers windows-port/15; echo $?` | `0` | `check` |
| 10 | Board lint: `statusgen --root . --lint` | `0` PROBLEMs | `check:ci` |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->
### Non-implementer verifier run — VERIFY: PASS on behaviour, held — 8/10 witness-clear, rows 9–10 held — 2026-09-30 claude-opus-5-5-verifier

Runner is not the implementer. Isolated worktree at merged main `b89b3957225e227e69d5b5ec7949344f580d9966` (HEAD == the forge's `commits/main`, cross-checked), host darwin/arm64, go1.27.1, jq-1.8.2, statusgen v1.0.29; `deskinbox` built from main. Implementing change: #1684 (squash aefb94618), follow-up #1821. `gate: model`, all four risk answers `no`. Status stays `implemented`: row 10 is `check:ci` and its network-off witness cannot run on a darwin host (#1800); row 9's `--consumers` run on merged main corroborates nothing (#1281 class).

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd tools/desk && go vet ./cmd/deskinbox/ && go test -count=1 ./cmd/deskinbox/` | exit 0 | PASS — exit 0; `ok github.com/medici-finance/assay/tools/desk/cmd/deskinbox 5.566s` | 2026-09-30 | claude-opus-5-5-verifier |
| 2 | `cd tools/desk && go test -count=1 -run 'TestParityFlow' -v ./cmd/deskinbox/ \| grep -c -- '--- PASS'` | `>= 1` | PASS — exit 0, count = 8; both flow parity tests passed 3 subtests each, incl. the AT LEAST + could-not-check + n/a case; 0 SKIP. Mutation probe (CountPartial forced false, flow.go:784) turned both red; restored | 2026-09-30 | claude-opus-5-5-verifier |
| 3 | `cd tools/desk && go test -count=1 -run 'TestParityHTML' ./cmd/deskinbox/` | PASS | PASS — exit 0; with -v both the html parity test and its login-shapes variant report `--- PASS` (not vacuous) | 2026-09-30 | claude-opus-5-5-verifier |
| 4 | `deskinbox html /tmp/inbox-check.html medici-finance/assay && ! grep -q -e 'url(' -e '<script' -e ' src=' /tmp/inbox-check.html` | exit 0 | PASS — exit 0 (output path moved to a session scratch dir); `deskinbox: wrote 76 card(s)`; no `url(`, `<script`, ` src=` or `@import`; the only hrefs are the 76 issue links; Flow section present | 2026-09-30 | claude-opus-5-5-verifier |
| 5 | `! grep -rn 'exec\.Command' tools/desk/cmd/deskinbox/*.go \| grep -v _test.go \| grep -qv -e statusgen -e deskboard` | exit 0 | PASS — exit 0. The one real site (flow.go:194) passes the grep via its trailing comment, so the code was checked directly: `resolveFlowBin` (flow.go:66) refuses any other base name — an `ASSAY_STATUSGEN` override naming `gh` gave rc=5, a Windows-style `gh.EXE` likewise | 2026-09-30 | claude-opus-5-5-verifier |
| 6 | `cd tools/desk && GOOS=windows GOARCH=amd64 go build ./cmd/deskinbox/` | exit 0 | PASS — exit 0; cross-compile only (no Windows host run) | 2026-09-30 | claude-opus-5-5-verifier |
| 7 | `grep -c 'deskinbox' plugins/assay/commands/inbox.md plugins/assay/skills/ask-decision/SKILL.md` | `>=` windows-port/13's counts, strictly more | PASS — inbox.md 37, SKILL.md 10 (vs 11 / 5 at the windows-port/13 merge, 13 / 7 just before #1684); no "not yet ported" text remains | 2026-09-30 | claude-opus-5-5-verifier |
| 8 | `deskinbox flow medici-finance/assay; echo rc=$?` and `bash plugins/assay/scripts/assay-inbox.sh --flow medici-finance/assay; echo rc=$?` | bottleneck line and per-stage counts agree | PASS — rc=0 / rc=0, run 45 s apart with the same readers; byte-identical apart from asOf and the program name. Both name bottleneck `todo` (1125 waiting / 8 slots = 140.62); stage counts intake 0, todo 106, in-progress 3, review n/a, implemented 83, verified 5, done 89; review is could-not-check in both (never 0) | 2026-09-30 | claude-opus-5-5-verifier |
| 9 | `statusgen --root . --consumers windows-port/15; echo $?` | `0` | HELD (#1281) — exit 0 but `no brief files in the diff against b89b395… — nothing to corroborate`; at the implementing PR's base (`--base e70bc8647`) rc=2 could-not-check (#1684 did not touch the brief). The three consumer claims were checked by hand: html.go and flow.go exist; assay-inbox.sh untouched by #1684; inbox.md and ask-decision SKILL.md changed by #1684 | 2026-09-30 | claude-opus-5-5-verifier |
| 10 | `statusgen --root . --lint` | `0` PROBLEMs | HELD (#1800) — check:ci network-off witness needs a Linux runner. Direct non-hermetic run (supporting only): exit 0, `LINT: PASS`, 0 PROBLEM lines; two NOTICEs on this brief (gotest-run-vacuous on rows 2 and 3, both cleared by the -v runs above) | 2026-09-30 | claude-opus-5-5-verifier |

RISK-VALUE (enumerate → rank → derive; the diff touches the forgeban exec allowlist, so enumerated despite all-`no` risk):

- RISK-VALUE: DERIVED — `resolveFlowBin` want = "statusgen" / "deskboard" @ tools/desk/cmd/deskinbox/flow.go:1094,1099 (html.go:170,175) — the brief names these two desk binaries as the flow model's only readers; refusing any other base name (`.exe` stripped, flow.go:95) keeps a forge CLI out of argv[0]; confirmed live (rc=5).
- RISK-VALUE: DERIVED — `stagedefs` = [intake, todo, in-progress, review, implemented, verified, done] @ tools/desk/cmd/deskinbox/flow.go:465-473 — matches the oracle's stage list entry for entry.
- RISK-VALUE: DERIVED — page mode `0o600` @ tools/desk/cmd/deskinbox/html.go:189 — owner-only for decision-queue content; the brief pins no value; reversible.

Findings (no defect in scope): `deskinbox html` is documented as the no-bash fallback — `--walk`/`--html` stay on the bash oracle in inbox.md and ask-decision SKILL.md because the screen classifier (attention-budget/15) is not ported; row 5's grep passes on a comment and row 9 passes vacuously once merged (both checked independently above).

### 2026-10-02 desk dispatch — rows 1 to 9 pass by hand, row 10 passes only outside the hermetic witness, row 9 now corroborated at the authoring commit, status held on the row 10 check:ci witness (#1800)

Runner is not the implementer. Detached worktree at merged main `e1d99484ffd9`; host darwin/arm64, go1.27.1, jq-1.8.2, statusgen v1.0.31; `deskinbox` built from this main. Implementing change: #1684, follow-up #1821. `gate: model`, all four risk answers `no`. Go tests ran under a throwaway HOME. Since the 2026-09-30 pass, no input file has changed (brief, html.go, flow.go, both parity tests, the inbox command doc and the ask-decision skill hash identically); only the toolchain moved (statusgen v1.0.29 to v1.0.31). The changelog fragment named in Context was folded into the root CHANGELOG by the v1.0.28 aggregation, so it no longer exists as its own file.

| # | Command | Exit | Observed | Date | Runner |
| --- | --- | --- | --- | --- | --- |
| 1 | `cd tools/desk && go vet ./cmd/deskinbox/ && go test -count=1 ./cmd/deskinbox/` | 0 | PASS — `ok .../tools/desk/cmd/deskinbox 1.922s` | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test -count=1 -run 'TestParityFlow' -v ./cmd/deskinbox/ \| grep -c -- '--- PASS'` | 0 | PASS — count 8: TestParityFlow and TestParityFlowText each pass 3 subtests (single cell; two cells with one blind, exercising AT LEAST plus could-not-check plus n/a; throughput unread); 0 SKIP | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | `cd tools/desk && go test -count=1 -run 'TestParityHTML' ./cmd/deskinbox/` | 0 | PASS — `ok`; with -v, TestParityHTML (3 subtests) and TestParityHTMLLoginShapes each report `--- PASS` | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | `deskinbox html OUT.html medici-finance/assay && ! grep -q -e 'url(' -e '<script' -e ' src=' OUT.html` (authored OUT is a file under the system temp dir; written to a session scratch dir instead) | 0 | PASS — `deskinbox: wrote 86 card(s)`, `86 item(s) across 1 repo(s)`; zero matches for `url(`, `<script`, ` src=` and `@import`; the only hrefs are the 86 issue links; Flow section with one inline SVG present; page mode 0600 | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | `! grep -rn 'exec\.Command' tools/desk/cmd/deskinbox/*.go \| grep -v _test.go \| grep -qv -e statusgen -e deskboard` | 0 | PASS — the one real exec site (flow.go:194) is matched through its trailing comment, so the bound was also checked live: an ASSAY_STATUSGEN override naming `gh` and an ASSAY_DESKBOARD override naming a Windows-style `gh.EXE` were both refused, rc=5 | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | `cd tools/desk && GOOS=windows GOARCH=amd64 go build ./cmd/deskinbox/` | 0 | PASS — PE32+ x86-64 console executable produced (cross-compile only, no Windows host run); the git-ignored artifact was removed afterwards | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | `grep -c 'deskinbox'` over the inbox command doc and the ask-decision SKILL.md | 0 | PASS — inbox.md 37, SKILL.md 10, against 11 and 5 at the windows-port/13 merge (#1507) and 13 and 7 just before #1684; no "not yet ported" text remains | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | `deskinbox flow medici-finance/assay; echo rc=$?` and the bash oracle `assay-inbox.sh --flow medici-finance/assay; echo rc=$?` | 0 / 0 | PASS — run 51 s apart with the same readers; the two outputs differ only in asOf and the program name. Both name the bottleneck `todo` (1273 waiting / 8 slots); stage counts intake 0, todo 111, in-progress 3, review n/a, implemented 85, verified 3, done 104; review is could-not-check in both (never 0) | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 9 | `statusgen --root . --consumers windows-port/15; echo $?` | 0 | PASS on exit, but corroborates nothing on merged main: `no brief files in the diff ... nothing to corroborate` (#1281 class). In a scratch clone: at the implementing commit (base its parent) rc=2 could-not-check, because #1684 did not touch the brief; at the authoring commit (#1507, base its parent) rc=0 with 3 CORROBORATED, 0 disproved, 0 unchecked | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 10 | `statusgen --root . --lint` | 0 | HELD (#1800): the check:ci network-off witness needs a Linux runner. Direct non-hermetic run (supporting evidence only): exit 0, `LINT: PASS`, 0 PROBLEM lines; NOTICEs on this brief: gotest-run-vacuous on rows 2 and 3 (the -v runs above clear them), verify-row-portability on row 4 | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |

Execution witness (statusgen verifyrun, dry run, throwaway HOME): exit 2, 8 of 10 rows proven. Row 4 exited 6 because the throwaway HOME carries no roster (`could-not-check: assay/desk-tools inactive`), an environment limit; the same row passes by hand with the desk config home. Row 10 could not run because darwin lacks `unshare --net` (#1800). The witness counts row 8 as passed, but that pass proves nothing: the row's trailing `echo rc=$?` returns 0 even when `deskinbox` exits 6.

Vacuity (mutations in a scratch clone only; the merged-main worktree was not touched):
- Row 2 is weaker than its Expect suggests. Forcing the AT LEAST partial-sum flag off fails both TestParityFlow and TestParityFlowText, yet the row still exits 0 with count 4, because `grep -c '--- PASS'` also counts subtests that passed. Row 1 catches this mutation. Before #1684 the row gives count 0 and exit 1.
- Row 3 passes vacuously before #1684 (`[no tests to run]`, exit 0). On this main it does discriminate: changing the Flow section heading in the page makes both html parity tests fail. The page's own summary line is not under parity; changing its wording still passes.
- Row 5 passes vacuously before #1684, when the package has no exec site. On this main it depends on comment text: deleting the trailing comment at flow.go:194 turns it red with no change in behaviour. A planted unscoped `exec.Command("gh")` does turn it red. The real bound is resolveFlowBin, checked live in row 5.
- Rows 1 and 6 do not discriminate: both pass before #1684. Row 7 always exits 0, so its Expect is judged by hand from the counts. Row 9 does not discriminate on merged main.

Risk values (enumerated over the #1684 diff in the exec allowlist, the stage model and the file write):
- RISK-VALUE: DERIVED — resolveFlowBin want = "statusgen" / "deskboard" @ deskinbox flow.go:1094,1099 (and html.go:170,175) — the brief names these two desk binaries as the flow model's only readers. Rejecting every other base name (a trailing `.exe` is stripped first, flow.go:95) keeps any forge CLI out of argv[0]; confirmed live, rc=5.
- RISK-VALUE: DERIVED — stagedefs = [intake, todo, in-progress, review, implemented, verified, done] @ deskinbox flow.go:465-473 — this matches the oracle's stagedefs (assay-inbox.sh:1394-1402) entry for entry, including labels and throughput keys. The source comment's oracle line citation (1025-1033) is stale; that is cosmetic.
- RISK-VALUE: DERIVED — page mode 0o600 @ deskinbox html.go:189 and flow.go:1111 — decision-queue content is readable by its owner only. The brief pins no value, and the setting is reversible.

VERIFY: BLOCKED — rows 1 to 9 pass by hand, and row 9 is corroborated at the authoring commit. Row 10 is check:ci and its network-off witness cannot run on a darwin host (#1800), so the status stays implemented until a Linux runner produces that witness. The gate is model, all risk answers are no, and no defect was found in scope.
### Non-implementer verifier re-run on merged main 11228951d0a8 — 2026-10-06

Independent non-implementer pass. Forge main SHA cross-checked: git and the API agree at 11228951d0a8. Readers ran with real binaries and no ambient credential, using statusgen/desk-tools v1.0.32 and go1.27.1 darwin/arm64. Expectations were derived from the brief before the diff was read. No Context file has changed since the 2026-10-02 pass at e1d99484ffd9, except this brief's Evidence.

| # | Command | Exit | Observed output | Date | Runner |
| --- | --- | --- | --- | --- | --- |
| 1 | `cd tools/desk && go vet ./cmd/deskinbox/ && go test -count=1 ./cmd/deskinbox/` | 0 | PASS: `ok .../tools/desk/cmd/deskinbox 3.205s` | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test -count=1 -run 'TestParityFlow' -v ./cmd/deskinbox/ \| grep -c -- '--- PASS'` | 0 | PASS: count 8. The flow and flow-text parity tests each pass 3 subtests: single cell; two cells with one blind; throughput unread. 0 SKIP, 0 FAIL | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) |
| 3 | `cd tools/desk && go test -count=1 -run 'TestParityHTML' ./cmd/deskinbox/` | 0 | PASS: `ok`. With -v, the HTML parity test (3 subtests) and the login-shapes test each report `--- PASS` | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) |
| 4 | `deskinbox html OUT.html medici-finance/assay && ! grep -q -e 'url(' -e '<script' -e ' src=' OUT.html` (OUT in a scratch dir) | 0 | PASS: wrote 73 cards, 73 items across 1 repo. Zero matches for `url(`, `<script`, ` src=` and `@import`. Every href is an issue link. A Flow section has one inline SVG. Page mode 0600 | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) |
| 5 | `! grep -rn 'exec\.Command' tools/desk/cmd/deskinbox/*.go \| grep -v _test.go \| grep -qv -e statusgen -e deskboard` | 0 | PASS. The only real exec site (flow.go:194) matches through its trailing comment, so the bound was checked live: an ASSAY_STATUSGEN override naming `gh` is refused rc=5, and an ASSAY_DESKBOARD override naming `gh.EXE` is refused rc=5 | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) |
| 6 | `cd tools/desk && GOOS=windows GOARCH=amd64 go build ./cmd/deskinbox/` | 0 | PASS: PE32+ x86-64 console executable. Cross-compile only, not run on a Windows host. Artifact removed | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) |
| 7 | `grep -c 'deskinbox' plugins/assay/commands/inbox.md plugins/assay/skills/ask-decision/SKILL.md` | 0 | PASS: inbox.md 37, SKILL.md 10, up from 11 and 5 at the windows-port/13 merge. No "not yet ported" text remains | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) |
| 8 | `deskinbox flow medici-finance/assay; echo rc=$?` against the bash oracle `assay-inbox.sh --flow medici-finance/assay` | 0 / 0 | PASS: the runs were 43 s apart and differ only in asOf and the program name. Both name the bottleneck `todo` (1393 waiting / 12 slots = 116.08). Stage counts match. Review is could-not-check in both, never 0. Note: the trailing `echo rc=$?` makes this row exit 0 even if deskinbox fails, so the comparison carries it | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) |
| 9 | `statusgen --root . --consumers windows-port/15; echo $?` | 0 (vacuous) | Exit 0, but nothing is corroborated on merged main ("no brief files in the diff"). Row-authoring defect: the positional `windows-port/15` is silently ignored, and the selector is `--brief`. With `--consumers --brief windows-port/15` on merged main the result is rc=2 could-not-check. At the authoring commit (#1507) with `--base` set to its parent: rc=0, 3 CORROBORATED, 0 disproved, 0 unchecked | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) |
| 10 | `statusgen --root . --lint` | 0 (held) | HELD (#1800): this check:ci row needs a network-off witness on a Linux runner. A direct non-hermetic run is supporting evidence only: exit 0, `LINT: PASS`, 0 PROBLEM lines | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) |

RISK-VALUE: DERIVED. The resolveFlowBin allow-list, "statusgen" / "deskboard" at tools/desk/cmd/deskinbox/flow.go:1094,1099 and html.go:170,175, matches the brief's two named readers. Every other base name is refused after a case-insensitive `.exe` strip, and `gh` and `gh.EXE` both return rc=5 live. The stage model (flow.go:464-472) matches the oracle entry for entry. WriteFile mode 0o600 is owner-only for decision-queue content. Layout constants are reversible and identical to the oracle.

Execution witness, `statusgen verifyrun` (exit 2; rows 1–9 pass, row 10 could-not-run on darwin):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go vet ./cmd/deskinbox/ && go test -count=1 ./cmd/deskinbox/` | pass exit=0 | sha256:3d2930ea8bea | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test -count=1 -run 'TestParityFlow' -v ./cmd/deskinbox/ \| grep -c -- '--- PASS'` | pass exit=0 | sha256:aa67a169b0bb | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test -count=1 -run 'TestParityHTML' ./cmd/deskinbox/` | pass exit=0 | sha256:4b0686a72e27 | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) (forge-identity) |
| 4 | `deskinbox html /tmp/inbox-check.html medici-finance/assay && ! grep -q -e 'url(' -e '<script' -e ' src=' /tmp/inbox-check.html` | pass exit=0 | sha256:f3fcd015df09 | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) (forge-identity) |
| 5 | `! grep -rn 'exec\.Command' tools/desk/cmd/deskinbox/*.go \| grep -v _test.go \| grep -qv -e statusgen -e deskboard` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows GOARCH=amd64 go build ./cmd/deskinbox/` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -c 'deskinbox' plugins/assay/commands/inbox.md plugins/assay/skills/ask-decision/SKILL.md` | pass exit=0 | sha256:10ebd0e57c74 | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) (forge-identity) |
| 8 | `deskinbox flow medici-finance/assay; echo rc=$?` | pass exit=0 | sha256:039cd9543317 | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers windows-port/15; echo $?` | pass exit=0 | sha256:d3e0f9aed696 | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) (forge-identity) |
| 10 | `statusgen --root . --lint` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) (forge-identity) |


VERIFY: BLOCKED — rows 1–9 pass by hand; row 10 is check:ci, and its network-off witness cannot run on a darwin host (#1800); the execution witness exited 2 on row 10 alone. No deliverable defect found. Row 9 positional-argument authoring defect noted above.

## Review
Gate: **model**. Reviewer's questions: (1) does the flow-model port reproduce the
could-not-check-vs-n/a distinction and the fleet-row "AT LEAST" partial-sum flag, or does a
blind reader silently read as zero anywhere? (2) is the html page genuinely self-contained
(row 4), and does it render through the SAME format builder walk.go uses, or has a second copy
of the five-part logic drifted in? (3) is every `exec.Command` site in this package scoped to
`statusgen`/`deskboard` only (row 5), with nothing that reaches `gh`/`jq`/`make`?
