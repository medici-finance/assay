---
brief: assay:assay:desktools-v2:13
title: platform gates — cross-compile CI leg, forge-ban GitLab symmetry, Verify-row portability lint, brief-06 re-derivation and platform-issue triage
why: >-
  The compatibility suite (desktools-v2/12) makes Windows and GitLab faults visible to tests,
  but nothing today makes them STAY visible: PR CI never compiles tools/desk for Windows, the
  forge-ban counter only counts GitHub facts so a GitLab reach-around grows silently, and a
  Verify row hardcoding /tmp is authored again the next week because no lint notices. This
  brief adds the three gates that hold the ground, re-derives brief 06 against the current
  tree (its cited issues are closed and its key row targets a retired shell oracle), and puts
  every known platform issue on an owner so the class stops resurfacing as unowned reports.
wave: 2
depends: ["desktools-v2/01"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1836, 641, 642, 678, 865, 1203, 655, 676, 677, 1411, 1412, 1415, 1418, 1424, 1435, 1477, 1569, 1573, 1604, 1621, 1644, 1667, 1693, 1794, 1805]
schema: brief-v2
outcome: none
authored: 2026-09-29 by the desk, scoping trusted issue #1836
sources:
  - "#1836 — §1a (CI never cross-compiles tools/desk), §1b (forge-ban is GitHub-asymmetric), §1c (Verify rows assume POSIX), §2 brief-06 row (stale), §3 (the unowned issue list), §4 T1/T8/T9 + the secondary windows-latest leg, §5 acceptance"
  - ".github/workflows/windows-ci-leg.yml — builds and lints statusgen only, per #1836 §1a; re-read 2026-09-30"
  - "tools/desk/scripts/forge-ban.sh — ADVISORY: prints four GitHub-fact class counts and exits 0 always; its --baseline mode upserts a single `desktools-v2/02 <N>` line in docs/streams/desktools-v2/forge-ban-baseline.txt — both re-read 2026-09-30"
  - "tools/desk/cmd/cellctl/shims.go — present at authoring 2026-09-29; the Go port brief 06's row 4 must target (desk-containers/10 retired the shell oracle it names)"
  - "#1145, #1146, #1223 — closed, per #1836 §2; brief 06 still cites them as its basis"
  - "docs/streams/desk-containers/README.md row 10 — reads `todo` for the cellctl Go port, re-read 2026-09-30"
exec-tier: strong
exec-tier-why: >-
  (a) the forge-ban symmetry change edits a COUNTING control — a careless port counts the
  wrong literals and the baseline rows start lying in the green direction; (b) the statusgen
  lint must NOTICE without false-firing on the hundred-plus legitimate POSIX rows already in
  the tree, which is a calibration act, not a grep; (c) the brief-06 re-derivation decides
  what a human-gated custody brief now means, with closed premises removed — authorship-grade
  judgment, evidence-bound.
domain: complicated
consumers:
  - "tools/desk/scripts/forge-ban.sh, NEW tools/desk/scripts/forge-ban-probe.sh (planned), docs/streams/desktools-v2/forge-ban-baseline.txt: follow-up desktools-v2/13 (this brief)"
  - "statusgen (the Verify-row portability NOTICE + TestVerifyRowPortability): follow-up desktools-v2/13 (this brief)"
  - ".github/workflows/ci.yml, windows-ci-leg.yml: follow-up desktools-v2/13 (this brief — staged, human-pushed, see the delivery mechanic)"
  - "docs/streams/desktools-v2/brief-06-installation-token-scoping.md: follow-up desktools-v2/13 (this brief is the single owner of its re-derivation, including its GitLab and Windows rows)"
  - "the §3 platform issues on the tracker: follow-up desktools-v2/13 (this brief's triage table)"
  - "docs/streams/desk-containers/README.md row 10: follow-up desktools-v2/13 (corrected in this brief's PR)"
  - "brief 06's GitLab and Windows rows, routed here from assay:assay:desktools-v2:12 (which does not edit brief 06): follow-up desktools-v2/13 (deliverable 5)"
  - "the behavior #1145, #1146 and #1223 fixed: follow-up desktools-v2/14 (pinned as regression tests there once dropped from brief 06's basis)"
  - "desktools-v2/11 (the house-callout brief) — no Windows or GitLab row today: out-of-scope (a gap #1836 did not catalog; raised as a follow-up issue at implementation, not silently absorbed here)"
version: 3
id: c3f52e25-c133-4e70-a242-c6392d215efb
---

# Brief 13 — platform gates and reconciliation

## Context

files:
- `tools/desk/scripts/forge-ban.sh` + NEW `tools/desk/scripts/forge-ban-probe.sh` (planned) — classes e and f, still advisory.
- `docs/streams/desktools-v2/forge-ban-baseline.txt` — the `desktools-v2/13` line.
- `statusgen/` — the Verify-row portability NOTICE and its test.
- `.github/workflows/ci.yml`, `.github/workflows/windows-ci-leg.yml` — staged as a patch for the human to apply; never pushed by this brief's implementer.
- `docs/streams/desktools-v2/brief-06-installation-token-scoping.md` — the re-derivation, offered to its human gate.
- `docs/streams/desk-containers/README.md` — row 10 corrected.
- `changelog/<branch>.md` — the per-PR fragment this repository requires.

Issue #1836's other half: the gates. Today a Windows compile break in `tools/desk` is first
seen at release time (PR CI builds it on Linux only; the cross-build loop runs in
`release.yml`), and the seam contract's ban counter covers only its GitHub half — class (a)
counts `gh` subprocesses but not `glab`, class (d) counts the `api.github.com` literal but not
`/api/v4` or `gitlab.com`, so a GitLab reach-around never moves the count. The counter is
advisory (it prints counts and exits 0 always; brief 08 flips only its statusgen half to
failing), so this brief proves the new classes COUNT — it does not claim they fail a build.
Separately, brief 06 has gone stale: the issues it cites are closed, and its row 4 tests a
shell oracle the Go cellctl port retired. And the known Windows and GitLab issues (#1836 §3)
sit unowned — none is claimed by any brief row, so each resurfaces as a fresh report.

**Delivery mechanic — workflow files.** App tokens cannot push `.github/workflows/*` (the
`workflows` scope is deliberately absent). Deliverables 1 and 4 therefore land as the
non-workflow half on the branch (the scripts, the lint, the triage) plus a ready-to-apply
workflow patch file and a `human-runsheet` entry; the human applies the workflow commit
themselves at merge time. The Verify rows for those deliverables are written against the
applied state and are expected to pass after the human's commit lands.

## Deliverables

1. **T1 — cross-compile gate in PR CI** (staged for human push, see above). A job in `ci.yml`
   that runs, on the Linux runner:
   ```sh
   cd tools/desk
   GOOS=windows GOARCH=amd64 go vet ./...
   for p in $(go list ./...); do GOOS=windows GOARCH=amd64 go test -c -o /dev/null "$p" || exit 1; done
   GOOS=darwin GOARCH=arm64 go vet ./...
   ```
   Compiling the test binaries too, so Windows-only build-tag and syscall breaks fail the PR,
   not the release. This is also the gate that holds the cross-compile class behind #322.
2. **T8 — forge-ban symmetry.** `forge-ban.sh` gains two classes, each reported per tree like
   the existing four: **class e** (`glab` subprocess — `exec.Command("glab"` in Go, a `glab`
   subcommand in shell) and **class f** (GitLab API literal — `/api/v4` or `gitlab.com` outside
   the two backend files). Output lines begin `class e (glab subprocess):` and
   `class f (GitLab API literal):`. The initial class-f count includes at least today's
   non-backend literals in `tools/desk/cmd/cellctl/{cell,new}.go`,
   `tools/desk/cmd/deskfleet/{client,main}.go`, `tools/desk/cmd/deskflip/flip.go`,
   `tools/desk/cmd/deskpost/{forgeclient,github}.go`, `tools/desk/cmd/desktoken/{gitlab,main}.go`,
   `tools/desk/internal/deskkit/{forge,forgegit,forgeidentity,forgeresolve,preflight,scrub,transporthost,trustliveness_gitlab}.go`
   and `tools/desk/internal/gitcore/claimref.go` — the list is not exhaustive; the script's own
   count is authoritative. Any carve-out (a file whose literal is the documented canonical home,
   as `forge.go` is for class d) is declared in the script header with its reason — never a
   silent exclusion. `--baseline` additionally upserts a line
   `desktools-v2/13 glab=<N> gitlab-literal=<N>` beside the existing `desktools-v2/02` line.
   **Classes e and f feed NEITHER total.** They are reported on their own lines and in the
   `desktools-v2/13` baseline line only; they are excluded from `forge reach-around sites:`
   (the class a–d `TOTAL` behind the `desktools-v2/02` baseline line, which therefore does not
   move) and from `statusgen sites:` (the count brief 08 turns into a failing gate, so today's
   statusgen `/api/v4` and `gitlab.com` literals cannot turn that gate red). Folding either
   class into a total is a later brief's decision, made with its own baseline.
   A committed probe script `tools/desk/scripts/forge-ban-probe.sh` (planned) proves the classes
   count: it records the class e and f desk counts, plants one `glab` shell-out and one
   `/api/v4` literal in a probe `.go` file under `tools/desk/internal/deskkit/`, re-runs the
   counter, asserts each class rose by exactly 1 and that the `forge reach-around sites:` and
   `statusgen sites:` lines did NOT change (the exclusion above, proven), removes the plant
   under a `trap` whatever the result, and prints `PROBE PASS` (exit 0) or
   `PROBE FAIL: <class or total>` (exit 1). The plant's first line is `//go:build ignore`, so a
   `go build`, `go vet` or `go test` running in the same tree while the probe runs never
   compiles it. The counter is grep-based and skips only `_test.go`, so a build-tagged file
   still counts; the rise-by-exactly-1 assertion is itself that check — a later counter that
   skipped build-ignored files would make the probe print `PROBE FAIL`, never pass vacuously. If any class
   proves impractical to count, the seam contract is amended in the same commit to state the
   ban is GitHub-only for that class — never a silent gap.
3. **T9 — Verify-row portability lint.** `statusgen --lint` NOTICEs (not fails) a Verify row
   that hardcodes `/tmp`, `sh -c`/`bash -c` or `findstr` unless the row carries an explicit OS
   marker, nudging authors to a Go test or `mktemp`/`${TMPDIR:-/tmp}`; the `${TMPDIR:-/tmp}`
   form is exempt. The notice text names the portability rule. A table test
   `TestVerifyRowPortability` (planned) in `statusgen` holds the positive controls (a `/tmp` row, an
   `sh -c` row, a `findstr` row each NOTICE) and the negative controls (a `${TMPDIR:-/tmp}` row,
   an OS-marked row, a plain `go test` row each stay silent). Calibrated against the existing
   tree: the NOTICE list it produces on main today is recorded in Evidence as the baseline.
4. **Windows test leg** (staged for human push with deliverable 1). `windows-ci-leg.yml`
   gains `cd tools/desk && go test ./...` on `windows-latest` (free on a public repo). Expected
   green once desktools-v2/12's `custodytest.PrivateTempDir` lands; if this leg lands first, it
   is expected red on the custody tests and the PR says so — that is the leg working, and 12 is
   the fix.
   **NOTE — trigger.** Today `windows-ci-leg.yml` runs only on a `v*` tag push and on
   `workflow_dispatch`, not on `pull_request`, so without a trigger change the leg proves
   Windows behavior at release time, not on a PR. The staged patch proposes adding a
   `pull_request` trigger — safe under the workflow's existing `permissions: contents: read`,
   and free on a public repo — and the human applying it decides. If the trigger is not added,
   the implementing PR says the Windows proof (here and for desktools-v2/12 rows 1–3 and 8) is
   release-time only.
5. **Brief 06 re-derived against the current tree — this brief is its single owner.** The
   closed issues #1145, #1146 and #1223 are dropped from its basis (their fixed behavior is
   pinned by desktools-v2/14's regression floor instead); row 4 targets the Go cellctl
   (`cmd/cellctl`, whose `shims.go` resolves `CELLCTL_GH_AMBIENT` before the HOME swap) instead
   of the retired shell oracle; the POSIX-only generated shims get an explicit statement — a
   Windows shim story, or out of scope with a named owner; the GitLab identity-path issues
   (#1573, #1203, #655, #676, #677) are either taken into its scope or assigned an owner; and
   brief 06 gains one GitLab row and one Windows-semantics row, each Expect cell ending with
   the trace marker `(desktools-v2/13 GitLab row)` or `(desktools-v2/13 Windows row)`. Its gate
   stays `human` — the re-derivation is offered to that gate as a diff for the human to
   accept, not waved through.
6. **Platform-issue triage, on the record.** Every issue in #1836 §3 (Windows: #641, #642,
   #1604, #1621, #1418, #1424, #1805, #1435, #1644, #678, #1569, #1693; GitLab: #1573, #1203,
   #655, #676, #677, #1477, #1411, #1412, #1415, #865, #1667, #1794) gets a comment naming its
   owner — a brief row that claims it (desktools-v2/12 for the custody, env, conformance and
   tier-gap classes) or an explicit out-of-scope with the stream that owns it — and the same
   mapping lands as a table in this brief's Evidence, one row per issue in the form
   `| #<N> | <owner> | <reason> |`, where `<owner>` begins either `desktools-v2/<NN>` or
   `out-of-scope`. The two newly-observed gaps are already owned: the cellctl non-POSIX-path
   gap by desktools-v2/12 deliverable 1, the deskfleet CE-approvals-404 gap by desktools-v2/12
   deliverable 7. Each triage comment carries the marker `(desktools-v2/13 triage)` so row 13
   can find it. The stale `desk-containers` README row 10 is corrected in this brief's PR to
   `implemented` — the most the implementer may set; `verified` and `done` are set only by the
   verifier or CI, so row 11 also accepts them for when that later flip lands.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `grep -q -F -e 'GOOS=windows' .github/workflows/ci.yml && grep -q -F -e 'go test -c' .github/workflows/ci.yml` | exit 0 — the cross-compile leg is in PR CI (passes once the human-applied workflow commit lands, per the delivery mechanic) |
| 2 | check | `d=$(mktemp -d) && sh tools/desk/scripts/forge-ban.sh > "$d/r2.out"; echo rc=$?; grep -F -e 'class e (glab subprocess):' "$d/r2.out" && grep -F -e 'class f (GitLab API literal):' "$d/r2.out"` | prints `rc=0` (the counter stays advisory) and both new class lines with their desk and statusgen counts |
| 3 | check +flow +dereference | `sh tools/desk/scripts/forge-ban-probe.sh` | prints `PROBE PASS` and exits 0 — a planted `glab` shell-out raises class e by exactly 1 and a planted `/api/v4` literal raises class f by exactly 1, in a `//go:build ignore` probe file (so the rise also proves a build-tagged file still counts), while the `forge reach-around sites:` and `statusgen sites:` totals stay unchanged; the probe file is removed whatever the result (the script's `trap`), so `git status --porcelain tools/desk` is empty afterwards |
| 4 | check | `grep -E -e '^desktools-v2/02 [0-9]+$' docs/streams/desktools-v2/forge-ban-baseline.txt && grep -E -e '^desktools-v2/13 glab=[0-9]+ gitlab-literal=[0-9]+$' docs/streams/desktools-v2/forge-ban-baseline.txt` | prints the `desktools-v2/02` line and the new `desktools-v2/13` line (exit 0) — the GitLab columns are recorded, machine-readably, on their own line; the `desktools-v2/02` total is untouched by classes e and f (row 3 proves the exclusion) |
| 5 | check +mutation | `d=$(mktemp -d) && cd statusgen && go test -run '^TestVerifyRowPortability$' -v . > "$d/r5.out" 2>&1 && grep -E -e '^--- PASS: TestVerifyRowPortability \(' "$d/r5.out" && sh testdata/portability-mutation.sh` | prints the top-level PASS line and `MUTATION PASS: disabled detector was rejected` (exit 0; the `&&` keeps `go test`'s status and the grep is anchored at column 0, so a failing control subtest cannot hide behind a passing one) — the positive controls NOTICE naming the portability rule and the negative controls, `${TMPDIR:-/tmp}` included, stay silent |
| 6 | check | `d=$(mktemp -d) && cd statusgen && go build -o "$d/statusgen" . && cd .. && "$d/statusgen" --root . --lint >/dev/null 2>&1; echo rc=$?` | `rc=0` — the new lint is NOTICE-level: the existing tree, POSIX rows included, still lints clean, and the NOTICE baseline is recorded in Evidence |
| 7 | check | `awk 'prev ~ /^[ ]+cd tools\/desk[ ]*$/ && $0 ~ /^[ ]+go test \.\/\.\.\.[ ]*$/ {f=1} {prev=$0} END {exit !f}' .github/workflows/windows-ci-leg.yml && grep -q -F -e 'windows-latest' .github/workflows/windows-ci-leg.yml` | exit 0 — the windows leg has a step that does `cd tools/desk` and then runs `go test ./...` on its very next line, i.e. the tools/desk suite itself rather than a mention of the path (passes once the human-applied workflow commit lands; exits 1 today, where the only `tools/desk` text in that file is a comment and no step runs the suite) |
| 8 | check | `grep -n -e 1145 -e 1146 -e 1223 docs/streams/desktools-v2/brief-06-*.md; test $? -eq 1` | exit 0 and nothing printed — the closed premises are out of brief 06 |
| 9 | check | `f=$(ls docs/streams/desktools-v2/brief-06-*.md); grep -q -F -e 'cmd/cellctl' "$f" && grep -q -E -e '^gate: human$' "$f" && grep -q -F -e '(desktools-v2/13 GitLab row)' "$f" && grep -q -F -e '(desktools-v2/13 Windows row)' "$f"` | exit 0 — row 4 targets the Go port, the gate is untouched, and brief 06 carries its GitLab and Windows rows |
| 10 | check | `miss=0; for n in 641 642 1604 1621 1418 1424 1805 1435 1644 678 1569 1693 1573 1203 655 676 677 1477 1411 1412 1415 865 1667 1794; do grep -q -E -e "^[\|] #$n [\|] desktools-v2/[0-9]+" -e "^[\|] #$n [\|] out-of-scope" docs/streams/desktools-v2/brief-13-platform-gates-and-reconciliation.md \|\| { echo "UNOWNED #$n"; miss=1; }; done; test $miss -eq 0` | exit 0 and nothing printed — each of the 24 §3 issues has its own triage row naming an owner (today it prints 24 `UNOWNED` lines and exits 1) |
| 11 | check | `grep -q -E -e '^[\|] 10 [\|].*[\|] implemented [\|]' -e '^[\|] 10 [\|].*[\|] verified [\|]' -e '^[\|] 10 [\|].*[\|] done [\|]' docs/streams/desk-containers/README.md` | exit 0 — the desk-containers board row 10 itself (not any other row) reads `implemented` (this brief's correction), or `verified`/`done` once the verifier or CI has flipped it; never `todo` |
| 12 | check | `statusgen --consumers --root .` | exit 0; no routing claim in this brief is disproved by the diff |
| 13 | check | `miss=0; for n in 641 642 1604 1621 1418 1424 1805 1435 1644 678 1569 1693 1573 1203 655 676 677 1477 1411 1412 1415 865 1667 1794; do c=$(gh api --paginate "repos/{owner}/{repo}/issues/$n/comments" --jq '.[] \| select(.body \| contains("(desktools-v2/13 triage)")) \| .id') \|\| { echo "READ FAILED #$n"; miss=1; continue; }; test -n "$c" \|\| { echo "NO TRIAGE COMMENT #$n"; miss=1; }; done; test $miss -eq 0` | exit 0 and nothing printed — each of the 24 §3 issues carries its triage comment on the tracker (the DoD bullet below); a failed read prints `READ FAILED` and fails the row rather than reading as empty. Today it prints 24 `NO TRIAGE COMMENT` lines and exits 1 |

## DoD

- All thirteen Verify rows pass; rows 1 and 7 pass after the human-applied workflow commit and
  the PR tracks that commit explicitly.
- No gate weakened: the forge-ban change only ADDS classes and stays advisory, the lint only
  NOTICEs, and brief 06 keeps `gate: human`.
- Every §3 issue carries its triage comment on the tracker, matching the Evidence table
  (row 13 checks the comments, row 10 the table).

## Evidence

The following is an author routing inventory, not independent Verify evidence. Read on
2026-10-01: all 24 comments lacked the triage marker; 12 issues were closed. Closed-item
comments cannot be posted by the worker attach verb and remain human acts. Workflow rows
1/7 and derived-board row 11 remain unfulfilled until the respective human acts land.

| Issue | Owner | Reason |
|---|---|---|
| #641 | desktools-v2/12 | Windows ACL custody parity |
| #642 | desktools-v2/12 | Cold-child USERPROFILE/custody environment propagation |
| #1604 | desktools-v2/12 | Windows custody parity and credential-chain environment conformance; owner mapping does not claim resolution |
| #1621 | out-of-scope (server-controls) | Credential check/use race and filesystem hardening require dedicated custody boundary work, not a platform fixture waiver |
| #1418 | out-of-scope (statusgen) | Verify runner startup classification on Windows |
| #1424 | out-of-scope (statusgen) | Explicit Verify row shell selection for native Windows commands |
| #1805 | out-of-scope (statusgen) | Verify command lifting and witness classification regression |
| #1435 | out-of-scope (windows-port) | Native Windows inbound monitor launcher replaces Bash dependency |
| #1644 | out-of-scope (desk-supervision) | Monitor state-directory ownership and shared temporary-directory custody |
| #678 | out-of-scope (windows-port) | Windows PowerShell parsing regression floor |
| #1569 | out-of-scope (windows-port) | PowerShell ASCII/parser class guard |
| #1693 | out-of-scope (windows-port) | First deskinstall bootstrap on clean Windows |
| #1573 | desktools-v2/06 | GitLab role-init must select explicit role PAT custody rather than GitHub App minting |
| #1203 | desktools-v2/06 | GitLab dispatch claim identity path and no App-minting fallback regression |
| #655 | desktools-v2/06 | GitLab cold preflight and explicit role PAT custody |
| #676 | desktools-v2/06 | GitLab boot credential backend selection |
| #677 | desktools-v2/06 | GitLab per-role commit identity scoping |
| #1477 | desktools-v2/12 | GitLab evidence-actor identity conformance |
| #1411 | desktools-v2/12 | Required check-context conformance when last_pipeline is empty |
| #1412 | desktools-v2/12 | GitLab existing-file update conformance |
| #1415 | desktools-v2/12 | GitLab draft-change error and response conformance |
| #865 | desktools-v2/12 | Issue/change comment-kind conformance on both backends |
| #1667 | out-of-scope (contributor-trust) | GitLab account-liveness at trust-roster boundary |
| #1794 | out-of-scope (verify-outcomes) | Live-credential Verify row execution and offline witness policy |

Local portability calibration is in [staged/portability-baseline.md](staged/portability-baseline.md); it is an author observation, not an independent witness.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -q -F -e 'GOOS=windows' .github/workflows/ci.yml && grep -q -F -e 'go test -c' .github/workflows/ci.yml` | fail exit=1 | sha256:e3b0c44298fc | 2026-10-02 | assay-verifier-app[bot] @ 6e419774479f (on-behalf-of human:ian) (forge-identity) |
| 2 | `d=$(mktemp -d) && sh tools/desk/scripts/forge-ban.sh > "$d/r2.out"; echo rc=$?; grep -F -e 'class e (glab subprocess):' "$d/r2.out" && grep -F -e 'class f (GitLab API literal):' "$d/r2.out"` | pass exit=0 | sha256:abf4e9ac77f9 | 2026-10-02 | assay-verifier-app[bot] @ 6e419774479f (on-behalf-of human:ian) (forge-identity) |
| 3 | `sh tools/desk/scripts/forge-ban-probe.sh` | pass exit=0 | sha256:406d6e54ba92 | 2026-10-02 | assay-verifier-app[bot] @ 6e419774479f (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -E -e '^desktools-v2/02 [0-9]+$' docs/streams/desktools-v2/forge-ban-baseline.txt && grep -E -e '^desktools-v2/13 glab=[0-9]+ gitlab-literal=[0-9]+$' docs/streams/desktools-v2/forge-ban-baseline.txt` | pass exit=0 | sha256:df26954dfed8 | 2026-10-02 | assay-verifier-app[bot] @ 6e419774479f (on-behalf-of human:ian) (forge-identity) |
| 5 | `d=$(mktemp -d) && cd statusgen && go test -run '^TestVerifyRowPortability$' -v . > "$d/r5.out" 2>&1 && grep -E -e '^--- PASS: TestVerifyRowPortability \(' "$d/r5.out" && sh testdata/portability-mutation.sh` | pass exit=0 | sha256:8577ff00fb88 | 2026-10-02 | assay-verifier-app[bot] @ 6e419774479f (on-behalf-of human:ian) (forge-identity) |
| 6 | `d=$(mktemp -d) && cd statusgen && go build -o "$d/statusgen" . && cd .. && "$d/statusgen" --root . --lint >/dev/null 2>&1; echo rc=$?` | pass exit=0 | sha256:93ff7811a209 | 2026-10-02 | assay-verifier-app[bot] @ 6e419774479f (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -q -F -e 'windows-latest' .github/workflows/windows-ci-leg.yml && grep -q -F -e 'tools/desk' .github/workflows/windows-ci-leg.yml` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-02 | assay-verifier-app[bot] @ 6e419774479f (on-behalf-of human:ian) (forge-identity) |
| 8 | `grep -n -e 1145 -e 1146 -e 1223 docs/streams/desktools-v2/brief-06-*.md; test $? -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-02 | assay-verifier-app[bot] @ 6e419774479f (on-behalf-of human:ian) (forge-identity) |
| 9 | `f=$(ls docs/streams/desktools-v2/brief-06-*.md); grep -q -F -e 'cmd/cellctl' "$f" && grep -q -E -e '^gate: human$' "$f" && grep -q -F -e '(desktools-v2/13 GitLab row)' "$f" && grep -q -F -e '(desktools-v2/13 Windows row)' "$f"` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-02 | assay-verifier-app[bot] @ 6e419774479f (on-behalf-of human:ian) (forge-identity) |
| 10 | `miss=0; for n in 641 642 1604 1621 1418 1424 1805 1435 1644 678 1569 1693 1573 1203 655 676 677 1477 1411 1412 1415 865 1667 1794; do grep -q -E -e "^[\|] #$n [\|] desktools-v2/[0-9]+" -e "^[\|] #$n [\|] out-of-scope" docs/streams/desktools-v2/brief-13-platform-gates-and-reconciliation.md \|\| { echo "UNOWNED #$n"; miss=1; }; done; test $miss -eq 0` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-02 | assay-verifier-app[bot] @ 6e419774479f (on-behalf-of human:ian) (forge-identity) |
| 11 | `grep -q -E -e '^[\|] 10 [\|].*[\|] implemented [\|]' -e '^[\|] 10 [\|].*[\|] verified [\|]' -e '^[\|] 10 [\|].*[\|] done [\|]' docs/streams/desk-containers/README.md` | fail exit=1 | sha256:e3b0c44298fc | 2026-10-02 | assay-verifier-app[bot] @ 6e419774479f (on-behalf-of human:ian) (forge-identity) |
| 12 | `statusgen --consumers --root .` | pass exit=0 | sha256:7d36278df6bf | 2026-10-02 | assay-verifier-app[bot] @ 6e419774479f (on-behalf-of human:ian) (forge-identity) |
| 13 | `miss=0; for n in 641 642 1604 1621 1418 1424 1805 1435 1644 678 1569 1693 1573 1203 655 676 677 1477 1411 1412 1415 865 1667 1794; do c=$(gh api --paginate "repos/{owner}/{repo}/issues/$n/comments" --jq '.[] \| select(.body \| contains("(desktools-v2/13 triage)")) \| .id') \|\| { echo "READ FAILED #$n"; miss=1; continue; }; test -n "$c" \|\| { echo "NO TRIAGE COMMENT #$n"; miss=1; }; done; test $miss -eq 0` | fail exit=1 | sha256:75ca79222361 | 2026-10-02 | assay-verifier-app[bot] @ 6e419774479f (on-behalf-of human:ian) (forge-identity) |

### Independent verification — 2026-10-02 (non-implementer, merged main 6e419774479f)

Runner for every row below: assay-verifier-app[bot] @ 6e419774479f (claude-opus-5-5, on-behalf-of human:ian). Host darwin/arm64; rows run under bash from the repository root. The table above is the execution witness from the same run (10 pass, 3 fail by exit status); this block is the hand reading of each row. The implementing change (#1996) was read only after the rows ran.

| # | command | exit | observed output | result |
|---|---------|------|-----------------|--------|
| 1 | grep -q -F -e 'GOOS=windows' .github/workflows/ci.yml && grep -q -F -e 'go test -c' .github/workflows/ci.yml | 1 | no output; ci.yml contains neither string. The cross-compile job exists only as a staged patch (staged/platform-workflows.patch, lines 25-28), not applied to the workflow | FAIL |
| 2 | d=$(mktemp -d) && sh tools/desk/scripts/forge-ban.sh > "$d/r2.out"; echo rc=$?; grep -F -e 'class e (glab subprocess):' "$d/r2.out" && grep -F -e 'class f (GitLab API literal):' "$d/r2.out" | 0 | rc=0 / class e (glab subprocess): desk=0 statusgen=0 / class f (GitLab API literal): desk=45 statusgen=8 | PASS |
| 3 | sh tools/desk/scripts/forge-ban-probe.sh | 0 | PROBE PASS; git status --porcelain tools/desk empty afterwards | PASS |
| 4 | grep -E -e '^desktools-v2/02 [0-9]+$' …forge-ban-baseline.txt && grep -E -e '^desktools-v2/13 glab=[0-9]+ gitlab-literal=[0-9]+$' …forge-ban-baseline.txt | 0 | desktools-v2/02 63 / desktools-v2/13 glab=0 gitlab-literal=45 | PASS |
| 5 | d=$(mktemp -d) && cd statusgen && go test -run '^TestVerifyRowPortability$' -v . > "$d/r5.out" 2>&1 && grep -E -e '^--- PASS: TestVerifyRowPortability \(' "$d/r5.out" && sh testdata/portability-mutation.sh | 0 | --- PASS: TestVerifyRowPortability (0.00s), 11 subtests PASS; mutation run shows tmp/sh/bash/findstr/tmpdir-plus-hardcode subtests FAIL with the detector disabled, then MUTATION PASS: disabled detector was rejected; detector source restored (tree clean) | PASS |
| 6 | d=$(mktemp -d) && cd statusgen && go build -o "$d/statusgen" . && cd .. && "$d/statusgen" --root . --lint >/dev/null 2>&1; echo rc=$? | 0 | rc=0. Re-run with output kept: LINT: PASS, 183 NOTICE lines tagged [verify-row-portability] at this SHA (author calibration file records 184 at an earlier main) | PASS |
| 7 | grep -q -F -e 'windows-latest' .github/workflows/windows-ci-leg.yml && grep -q -F -e 'tools/desk' .github/workflows/windows-ci-leg.yml | 0 | exit 0, but the only 'tools/desk' match is a comment (line 79, naming a Linux byte-guard test) added by #1578 on 2026-10-02, after this brief was implemented (#1996); before #1578 the row exited 1. The workflow has no tools/desk go test step and no pull_request trigger. Requirement NOT met — vacuous pass | FAIL (requirement unmet; row exit 0 is vacuous) |
| 8 | grep -n -e 1145 -e 1146 -e 1223 docs/streams/desktools-v2/brief-06-*.md; test $? -eq 1 | 0 | nothing printed | PASS |
| 9 | f=$(ls docs/streams/desktools-v2/brief-06-*.md); grep -q -F -e 'cmd/cellctl' "$f" && grep -q -E -e '^gate: human$' "$f" && grep -q -F -e '(desktools-v2/13 GitLab row)' "$f" && grep -q -F -e '(desktools-v2/13 Windows row)' "$f" | 0 | no output; brief 06 line 13 reads gate: human, rows 7 and 8 carry the two trace markers | PASS |
| 10 | miss=0; for n in <24 issues>; do grep -q -E … brief-13 … done; test $miss -eq 0 | 0 | nothing printed; 24 triage rows present | PASS |
| 11 | grep -q -E -e '^[|] 10 [|].*[|] implemented [|]' -e '… verified …' -e '… done …' docs/streams/desk-containers/README.md | 1 | no match; row 10 of that board still reads todo | FAIL |
| 12 | statusgen --consumers --root . | 0 | consumers: no brief files in the diff against 6e419774479f — nothing to corroborate (same result from the installed v1.0.31 binary and a tree-built binary) | PASS (vacuous on merged main, see findings) |
| 13 | miss=0; for n in <24 issues>; do c=$(gh api --paginate "repos/{owner}/{repo}/issues/$n/comments" --jq …) … done; test $miss -eq 0 | 1 | 12 lines NO TRIAGE COMMENT for #641 #642 #655 #676 #677 #678 #865 #1203 #1411 #1415 #1418 #1805; no READ FAILED lines (all 24 reads succeeded; read-only forge GET) | FAIL |

Result: FAIL — 9 of 13 rows. Rows 1, 11 and 13 fail by exit code; row 7 exits 0 but its requirement is unmet (see below), which is why the hand count is one lower than the witness. None of the four is a defect in the code that merged: each waits on an act held for a human, tracked in #2055. Status stays `implemented`.

**Vacuity findings**

1. Row 7 — FALSE GREEN. 'tools/desk' matches a comment that #1578 added to windows-ci-leg.yml on 2026-10-02, an unrelated change that landed after this brief was authored (#1878) and implemented (#1996); on those trees the file had no 'tools/desk' match and the row exited 1. 'windows-latest' was already there. Since #1578 the row passes on a tree without the work. Deliverable 4 (tools/desk go test on windows-latest, proposed pull_request trigger) is present only in the staged patch. The brief says rows 1 and 7 "pass after the human-applied workflow commit"; row 7 already passes without it, so it cannot detect that commit. Suggested tightening: match the step itself, e.g. a 'cd tools/desk' line followed by 'go test ./...'.
2. Row 12 — vacuous on merged main: the consumers check diffs against HEAD, finds no brief files in the diff, and reports "nothing to corroborate". It can only bite pre-merge.
3. Row 6 — the command discards all output and ends in `echo rc=$?`, so the row's exit status is always 0 and the only oracle is the printed rc=0. It would print rc=0 on a tree without the lint. It shows "the tree still lints clean", not "the NOTICE fires"; the latter was confirmed separately by hand (183 notices) and by row 5.
4. Row 2 — `echo rc=$?` after a script that ends in an unconditional `exit 0` is always rc=0; the two greps carry the row. Not vacuous for the class lines.
5. Row 4 — the 02 line is 63 while the counter at this SHA prints `forge reach-around sites: 65`. Not this brief's doing (the 13 commit added only the second line, and an earlier change stopped rows comparing against it), but the recorded a–d baseline is stale by 2.

**Other findings**

- The class f baseline of 45 includes one self-count: the probe script's own heredoc line containing the /api/v4 literal. The counter excludes itself in shell but not the probe script, and the header declares no carve-out for it. It also counts 4 lines in the cellctl shell-oracle test fixture. Advisory only; the baseline is reproducible as recorded.
- Row 5's command was edited by the implementing PR itself (the mutation step and `+mutation` class were added). It is a strengthening, but the implementer changed its own oracle.
- No human-runsheet file for the workflow patch was found under the stream directory (the delivery mechanic names one). Not checked elsewhere — could-not-check, not absent.
- The changelog fragment added by the implementing commit is no longer present at this SHA under its original name (presumably rolled up by a later release); not a Verify row.

**Risk-bearing value**

Brief risk metadata is present and all four answers are "no"; gate is model. Enumeration over the implementing commit (forge-ban.sh, forge-ban-probe.sh, forge-ban-baseline.txt, verifyportability.go, enforcementstatus.go, main.go, portability-mutation.sh, the staged patch):

- `desktools-v2/13 glab=0 gitlab-literal=45` @ docs/streams/desktools-v2/forge-ban-baseline.txt:2 — a recorded baseline a later brief compares against; wrong in the low direction would hide growth. Reversible by re-running --baseline.
- class f pattern `/api/v4|gitlab\.com` @ tools/desk/scripts/forge-ban.sh:168 and :176 — reversible.
- `GLAB_SUBCMDS='issue|mr|api|auth|repo|release|ci|label|schedule|snippet|variable'` @ forge-ban.sh:162 — reversible; a glab subcommand outside the list is not counted in shell.
- detector regexes verifyOSMarker / verifyShellC / verifyFindstr / verifyTmpPath @ statusgen/verifyportability.go:11-14 — advisory NOTICE, reversible.
- `"verify-row-portability" … StatusAdvisory` @ statusgen/enforcementstatus.go (added line) — reversible.
- `-timeout 45s` @ statusgen/testdata/portability-mutation.sh:17 — operational knob.

None is irreversible. Top-ranked entry derived:

RISK-VALUE: DERIVED — desktools-v2/13 glab=0 gitlab-literal=45 @ docs/streams/desktools-v2/forge-ban-baseline.txt:2 — an independent line count (separate script, not the counter) of `/api/v4` or `gitlab.com` matching lines in .go/.sh files under tools/desk, tools/cellctl and plugins/assay, excluding the two forge backends, _test.go, .test.sh and the counter itself, gives 45; `exec.Command("glab"` occurrences in non-test Go give 0. The value matches the brief's definition of classes e and f. Caveat: 1 of the 45 is the probe script's own literal.

**Correction, 2026-10-02 (after review of this record).** The row 7 reading and vacuity finding 1 first said the matching comment predated the brief. Repository history shows otherwise and both now say so: the comment arrived with #1578, after the implementation merged, which is when row 7 turned from exit 1 to a false green. The row 7 verdict is unchanged. Row 13 is recorded as of this run (2026-10-02T11:32Z); the reviewer's later re-run reports that the twelve missing triage comments were posted at about 21:28Z the same day and that the row then exits 0. This run did not re-check that. Rows 1, 7 and 11 still hold the brief.
