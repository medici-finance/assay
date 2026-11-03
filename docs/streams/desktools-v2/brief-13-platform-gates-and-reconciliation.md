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
   and `gitcore/claimref.go` — the list is not exhaustive; the script's own
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

Result: FAIL — 9 of 13 rows. Rows 1, 11 and 13 fail by exit code; row 7 exits 0 but its requirement is unmet (see below), which is why the hand count is one lower than the witness. None of the four is a defect in the code that merged: each waits on an act held for a human, tracked in #2055. Status stays `implemented`. Routed to closed #2055: the hold was tracked there and resolved on 2026-10-03.

**Vacuity findings**

1. Row 7 — FALSE GREEN. 'tools/desk' matches a comment that #1578 added to windows-ci-leg.yml on 2026-10-02, an unrelated change that landed after this brief was authored (#1878) and implemented (#1996); on those trees the file had no 'tools/desk' match and the row exited 1. 'windows-latest' was already there. Since #1578 the row passes on a tree without the work. Deliverable 4 (tools/desk go test on windows-latest, proposed pull_request trigger) is present only in the staged patch. The brief says rows 1 and 7 "pass after the human-applied workflow commit"; row 7 already passes without it, so it cannot detect that commit. Suggested tightening: match the step itself, e.g. a 'cd tools/desk' line followed by 'go test ./...'.
2. Row 12 — vacuous on merged main: the consumers check diffs against HEAD, finds no brief files in the diff, and reports "nothing to corroborate". It can only bite pre-merge.
3. Row 6 — the command discards all output and ends in `echo rc=$?`, so the row's exit status is always 0 and the only oracle is the printed rc=0. It would print rc=0 on a tree without the lint. It shows "the tree still lints clean", not "the NOTICE fires"; the latter was confirmed separately by hand (183 notices) and by row 5.
4. Row 2 — `echo rc=$?` after a script that ends in an unconditional `exit 0` is always rc=0; the two greps carry the row. Not vacuous for the class lines.
5. Row 4 — the 02 line is 63 while the counter at this SHA prints `forge reach-around sites: 65`. Not this brief's doing (the 13 commit added only the second line, and an earlier change stopped rows comparing against it), but the recorded a–d baseline is stale by 2.

**Other findings**

- The class f baseline of 45 includes one self-count: the probe script's own heredoc line containing the /api/v4 literal. The counter excludes itself in shell but not the probe script, and the header declares no carve-out for it. It also counts 4 lines in the cellctl shell-oracle test fixture. Advisory only; the baseline is reproducible as recorded.
- Row 5's command was edited by the implementing PR itself (the mutation step and `+mutation` class were added). It is a strengthening, but the implementer changed its own oracle.
- No human-runsheet file for the workflow patch was found under the stream directory (the delivery mechanic names one). Not checked elsewhere — could-not-check, not absent. Routed to closed #2055: the hold was tracked there and resolved on 2026-10-03.
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

### 2026-10-03 desk dispatch — re-verify on merged main b3677d6da: 12 of 13 rows pass; row 11 blocked on act 3 of #2055

Independent re-run by a non-implementer at merged main b3677d6da (forge main was one commit ahead at 8291853224, a STATUS.md regeneration only; no file this brief touches differs). Host darwin/arm64, rows run from the repository root. Since the prior verdict (#2056): #2064 applied the staged workflow patch, #2066 re-pointed row 7 at the test step, and the twelve missing triage comments were posted on 2026-10-02. Commands are abbreviated below where a path or argument is long; the full text is the Verify table above.

| # | command | expected | observed: exit code + key output | date / runner |
|---|---|---|---|---|
| 1 | grep for GOOS=windows and go test -c in the ci workflow | exit 0 | exit 0. ci.yml now has a desk-platform-compile job (lines 25-46) whose run step does cd tools/desk, the windows/amd64 go vet, the per-package windows go test -c loop and the darwin/arm64 go vet; the workflow triggers on push and pull_request. Control: the same grep on the ci.yml before #2064 exits 1 | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | sh forge-ban.sh (tools/desk scripts), then grep both new class lines | rc=0 and both class lines | exit 0. rc=0 / class e (glab subprocess): desk=0 statusgen=0 / class f (GitLab API literal): desk=45 statusgen=8 | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | sh forge-ban-probe.sh (tools/desk scripts) | PROBE PASS, exit 0, tree clean | exit 0. PROBE PASS; git status --porcelain tools/desk empty afterwards | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | grep the desktools-v2/02 and desktools-v2/13 lines in the forge-ban baseline file | both lines, exit 0 | exit 0. desktools-v2/02 63 / desktools-v2/13 glab=0 gitlab-literal=45 | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | go test -run '^TestVerifyRowPortability$' -v in statusgen, anchored PASS grep, then sh testdata/portability-mutation.sh | top-level PASS and MUTATION PASS | exit 0. --- PASS: TestVerifyRowPortability (0.09s) / MUTATION PASS: disabled detector was rejected; git status --porcelain empty afterwards | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | build statusgen from the tree, run --lint with output discarded, echo rc | rc=0 | exit 0, rc=0. Re-run with output kept: LINT: PASS, 183 NOTICE lines tagged verify-row-portability (same count as the prior pass) | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | awk for a cd tools/desk line followed by go test ./... in the windows leg workflow, plus grep windows-latest | exit 0 | exit 0. windows-ci-leg.yml job desk-windows-tests runs on windows-latest; lines 78-79 are cd tools/desk then go test ./... (pwsh step). Control: the same awk on the file before #2064 exits 1, so the row now detects the step. The leg keeps its v-tag and workflow_dispatch triggers only (no pull_request), which #2064 states, so the Windows proof is release-time only, as deliverable 4 allows | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | grep -n for 1145, 1146, 1223 in brief 06; test status is 1 | exit 0, nothing printed | exit 0, nothing printed | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 9 | brief 06 carries cmd/cellctl, gate: human, and both trace markers | exit 0 | exit 0, no output | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 10 | 24-issue loop over this brief's triage table | exit 0, nothing printed | exit 0, nothing printed; all 24 rows present | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 11 | grep desk-containers README row 10 for implemented, verified or done | exit 0 | exit 1, no match. Row 10 still reads todo. BLOCKED, not a code defect: act 3 of #2055, the desk-containers README row 10 flip, has not landed. (Correction: the design record DR-cellctl-go-port that desk-containers/10 needs had already landed in #2069, so the flip was lint-clean at this SHA; the hold was the board flip alone.) | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 12 | statusgen --consumers --root . | exit 0, nothing disproved | exit 0. consumers: no brief files in the diff against b3677d6da — nothing to corroborate (installed v1.0.31 and a tree-built binary agree). Supplemental, with --base set to the parent of the implementing commit (#1996): exit 0, 79 briefs, 36 corroborated, 0 disproved, 287 unchecked | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 13 | 24-issue loop reading each issue's comments for the triage marker (read-only GET) | exit 0, nothing printed | exit 0, nothing printed; no READ FAILED and no NO TRIAGE COMMENT lines. Spot check: the marker comments on #641 and #1805 are by the maintainer (2026-10-02 21:28Z), on #1604 by the worker App (2026-10-01) | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |

Execution witness: statusgen verifyrun --dry-run (v1.0.31) on this brief, throwaway HOME, at b3677d6da: exit 1; rows 1-10, 12 and 13 pass, row 11 fail (exit=1). This matches the hand reading above.

Vacuity: rows 1 and 7 are no longer vacuous; both exit 1 on the workflow files as they were before #2064, so each now detects the applied commit. Row 12 stays vacuous on merged main (empty diff); the supplemental run above is the meaningful reading. Row 6 ends in echo rc, so its exit status is always 0 and the printed rc=0 is the only oracle; the NOTICE firing is shown by row 5 and the 183-line count. Row 2's rc=0 is likewise unconditional; the two greps carry it.

Finding: per #2064, with a pull_request trigger the native Windows desk suite failed 57 of 96 packages on the hosted runner, so the desk-windows-tests job will be red at the next v-tag run until desktools-v2/12 lands. Deliverable 4 anticipates this; recorded so a red release-tag leg is not read as a new regression.

Risk-bearing value (file references abbreviated to base names; the baseline file is in the desktools-v2 stream directory, the workflows under .github/workflows). Risk metadata present, all four answers no; gate model; not irreversible. Enumeration over this brief's diff (#1996 as enumerated in the prior pass) plus the changes since (#2064, #2066):
- desktools-v2/13 glab=0 gitlab-literal=45 @ forge-ban-baseline.txt:2 — recorded baseline; reversible by re-running the baseline mode.
- class f pattern /api/v4 or gitlab.com @ forge-ban.sh:168 and :176 (tools/desk scripts) — reversible.
- GOOS=windows GOARCH=amd64 and GOOS=darwin GOARCH=arm64 @ ci.yml:44-46 — the cross-compile targets; reversible.
- runs-on: windows-latest @ windows-ci-leg.yml:69 — reversible.
- triggers push tags v* and workflow_dispatch @ windows-ci-leg.yml:50-54 — reversible; the human chose to leave out pull_request.
- detector regexes @ verifyportability.go:11-14; -timeout 45s @ portability-mutation.sh:17 — advisory and operational, reversible.

None is irreversible. Top-ranked entry:

RISK-VALUE: DERIVED — desktools-v2/13 glab=0 gitlab-literal=45 @ forge-ban-baseline.txt:2 — an independent grep (not the counter) of lines matching /api/v4 or gitlab.com in .go and .sh files under tools/desk, tools/cellctl and plugins/assay, excluding the two forge backends, test files and the counter itself, gives 45, and exec.Command("glab" in non-test Go gives 0; the counter at this SHA prints desk=45. The value matches the brief's definition of classes e and f. As noted in the prior pass, 1 of the 45 is the probe script's own literal.

Context files: all present at this SHA (forge-ban.sh, forge-ban-probe.sh, the baseline file, statusgen with verifyportability.go and its test, both workflows, brief 06, the desk-containers README, cellctl shims.go, the changelog fragment dtv2-13-platform-workflows.md).

VERIFY: BLOCKED — 12 of 13 rows pass at b3677d6da; row 11 waits on act 3 of #2055 (the desk-containers README row 10 flip), status stays implemented

### 2026-10-03 desk dispatch (second pass) — desktools-v2/13 re-verified on merged main 03065900ca21: 13 of 13 rows pass, requirements met

Independent non-implementer pass on merged main 03065900ca21 (forge main confirmed equal at run time; later than the 0dbe21e42 named in the dispatch). Host darwin/arm64; every row run under bash from the repository root, rows 1 to 12 under a throwaway HOME, row 13 as read-only forge GETs. Commands are abbreviated below; the full text is the brief's Verify table. Changes since the prior pass (12 of 13, open Evidence PR #2076): #2075 flipped desk-containers board row 10 from todo to implemented, which is what row 11 was waiting on. #2064 (workflow patch applied by the human) and #2066 (row 7 tightened to match the step, not a comment) were already in the prior pass's tree.

| # | Command | Expected | Observed (exit code + key output) | Date / Runner |
|---|---|---|---|---|
| 1 | grep -q -F GOOS=windows and grep -q -F 'go test -c' on .github/workflows/ci.yml | exit 0 | exit 0. Both strings are real steps in job desk-platform-compile of ci.yml (lines 44 to 46: windows/amd64 vet, per-package windows test-binary compile, darwin/arm64 vet); ci.yml triggers on push and pull_request | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | sh tools/desk/scripts/forge-ban.sh into a mktemp file; echo rc; grep the class e and class f lines | rc=0 and both class lines | exit 0. rc=0 / class e (glab subprocess): desk=0 statusgen=0 / class f (GitLab API literal): desk=45 statusgen=8 | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | sh (tools/desk/scripts) forge-ban-probe.sh | PROBE PASS, exit 0, tools/desk clean afterwards | exit 0. PROBE PASS; git status --porcelain tools/desk printed nothing afterwards | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | grep -E the desktools-v2/02 line and the desktools-v2/13 glab/gitlab-literal line in forge-ban-baseline.txt | both lines, exit 0 | exit 0. desktools-v2/02 63 / desktools-v2/13 glab=0 gitlab-literal=45 | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | cd statusgen; go test -run TestVerifyRowPortability -v into a mktemp file, grep the anchored top-level PASS line, then sh testdata/portability-mutation.sh | top-level PASS and MUTATION PASS, exit 0 | exit 0. --- PASS: TestVerifyRowPortability (0.00s); the mutation run shows the notice-flow, tmp, sh, bash, findstr and tmpdir-plus-hardcode subtests FAIL with the detector disabled, then MUTATION PASS: disabled detector was rejected; tree clean afterwards | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | cd statusgen; go build into a mktemp dir; run it with --root . --lint, output discarded; echo rc | rc=0 | exit 0, rc=0. Re-run with output kept: LINT: PASS with 183 NOTICE lines tagged verify-row-portability (the NOTICE baseline at this SHA; unchanged from the 2026-10-02 pass) | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | awk for a line 'cd tools/desk' followed on the next line by 'go test ./...' in windows-ci-leg.yml, and grep -q -F windows-latest | exit 0 | exit 0. Matched lines 78 and 79 of the step Native Windows desk suite in job desk-windows-tests (runs-on windows-latest, pwsh); a real test step, not a comment | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | grep -n 1145 / 1146 / 1223 in brief-06; test $? -eq 1 | exit 0, nothing printed | exit 0, nothing printed | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 9 | brief-06 contains cmd/cellctl, a gate: human line, and both desktools-v2/13 trace markers | exit 0 | exit 0, no output | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 10 | loop over the 24 section-3 issues, grep each for an owner row in brief-13 | exit 0, nothing printed | exit 0, nothing printed; 24 owner rows present | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 11 | grep -q -E board row 10 of desk-containers README reading implemented, verified or done | exit 0 | exit 0. Row 10 (cellctl in Go) reads implemented, set by #2075 | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 12 | statusgen --consumers --root . | exit 0 | exit 0. consumers: no brief files in the diff against 03065900ca21 — nothing to corroborate (installed v1.0.31) | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 13 | loop over the 24 issues, gh api --paginate comments, select bodies containing the desktools-v2/13 triage marker | exit 0, nothing printed | exit 0, nothing printed; no READ FAILED and no NO TRIAGE COMMENT lines. Hand check: every one of the 24 carries a marked comment (12 by the worker App on 2026-10-01, 12 by the human driver on 2026-10-02 about 21:28Z), and each comment's named owner matches this brief's triage table, 24 of 24 | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |

Execution witness: statusgen verifyrun --dry-run (v1.0.31, throwaway HOME) exit 0, 13 of 13 rows pass exit=0 at 03065900ca21; nothing written back, tree clean.

Vacuity and findings

- Row 2 always prints rc=0 (the counter ends in an unconditional exit 0); the two greps carry the row. Row 6 always exits 0 and prints rc=0 on any tree; that the NOTICE fires is shown by row 5 and the 183-line count above, not by row 6.
- Row 12 is vacuous on merged main: the diff has no brief files, so there is nothing to corroborate. It can only bite pre-merge.
- Rows 1 and 7 are no longer vacuous: each matches the live workflow step, not a comment.
- Deliverable 4 trigger: the human-applied workflow commit (#2064) kept windows-ci-leg.yml on tag push and workflow_dispatch only, with no pull_request trigger. The brief leaves that decision to the human. Consequence, as the brief states: the Windows proof (here and for desktools-v2/12 rows 1 to 3 and 8) is release-time only. The cross-compile gate in ci.yml does run on pull_request.
- Row 4 side note: the desktools-v2/02 baseline reads 63, while the counter now prints forge reach-around sites: 69. Classes e and f are excluded from that total. The drift comes from class a to d growth elsewhere, not from this brief. It is advisory.
- Class f self-count: 1 of the 45 is the probe script's own planted literal (heredoc). It has no carve-out in the header. Advisory, and reproducible as recorded.

Risk-bearing value

Risk metadata is present: regulatory, customer, irreversible and sensitive-data are all no, and the gate is model. The diff touches .github/workflows (via #2064), so the enumeration was done anyway. It covered the implementing change #1996 (the forge-ban counter, the probe, the baseline, the statusgen portability detector and its mutation script), the workflow commit #2064, and the row 7 edit #2066.

- desktools-v2/13 glab=0 gitlab-literal=45 @ the stream forge-ban-baseline.txt:2. A recorded baseline. If it were wrong in the low direction it would hide growth. Reversible with --baseline.
- GLAB_SUBCMDS = issue, mr, api, auth, repo, release, ci, label, schedule, snippet, variable @ forge-ban.sh:172. Reversible.
- class f pattern = /api/v4 or gitlab.com @ forge-ban.sh:178 and :186. Reversible.
- BACKEND_EXCLUDE = forge_github, forge_gitlab @ forge-ban.sh:66. Reversible.
- detector regexes verifyOSMarker, verifyShellC, verifyFindstr and verifyTmpPath @ statusgen/verifyportability.go:11 to 14. Advisory NOTICE only. Reversible.
- cross-compile targets GOOS=windows GOARCH=amd64 and GOOS=darwin GOARCH=arm64 @ ci.yml:44 to 46. Reversible.
- GO_VERSION = "1.25.0" @ windows-ci-leg.yml:65 (pre-existing env, now also consumed by the new job). Operational.
- action pins: actions/checkout v7.0.1 and actions/setup-go v5.5.0 (full commit SHAs, abbreviated here) @ windows-ci-leg.yml:71 and :72. Reversible.

None of these is irreversible. The top-ranked entry is derived below.

RISK-VALUE: DERIVED — desktools-v2/13 glab=0 gitlab-literal=45 @ the stream forge-ban-baseline.txt:2 — an independent recount, done with a separate script and not with the counter, gives 45. It counts lines matching /api/v4 or gitlab.com in .go and .sh files under tools/desk, tools/cellctl and plugins/assay, excluding the two forge backends, _test.go, .test.sh and the counter itself, and those lines span 21 files. The same recount finds 0 exec.Command("glab" in non-test Go and 8 statusgen literals. All three match the counter's output and the brief's definitions of classes e and f. Caveat: 1 of the 45 is the probe's own literal.

Context files: all present at this SHA. That covers forge-ban.sh, forge-ban-probe.sh, forge-ban-baseline.txt, statusgen (with verifyportability.go), ci.yml, windows-ci-leg.yml, brief-06 and the desk-containers README. The changelog fragments dtv2-13-platform-workflows.md and feat--assay--issue-2055.md are both present.

VERIFY: PASS — all 13 rows pass by exit code and by hand reading on merged main 03065900ca21, rows 1, 7 and 13 now non-vacuously; gate model, risk all no; the Windows test leg is release-time only by the human's recorded trigger choice

### Non-implementer verifier re-run — VERIFY: BLOCKED (#1281) — 2026-10-04 claude-opus-5-5-verifier

Independent non-implementer pass on merged main 4f6bb6d59 (host darwin/arm64, go 1.27.1, installed statusgen v1.0.31). Rows 1 to 12 were run by hand under bash from the repository root with a throwaway HOME and KUBECONFIG=/dev/null; row 13 ran as read-only forge GETs (no writes). The execution witness below comes from a separate `statusgen verifyrun` run on the same tree. Expectations were taken from the brief's Task, DoD and Verify text before the earlier Evidence, the outcome record and #2055 were read. Commands are abbreviated; the full text is in the Verify table.

Why this re-run: the last landed outcome record (verify-fail at 6e4197744, 9 of 13) named blocker #2055. That issue closed on 2026-10-03 at 03:04Z after all three held human acts landed: #2064 applied the workflow patch, #2075 flipped desk-containers board row 10, and the twelve triage comments were posted on 2026-10-02 at about 21:28Z. #2066 also tightened row 7. All three held rows now pass on main: row 1 and row 7 match real workflow steps, and row 11 matches board row 10, which reads implemented. Between 03065900ca21 (the last Evidence-only PASS, #2076) and this SHA, the only changes to files these rows read are brief 13's own Evidence (#2076) and a Cobra/Viper section in brief 06 (#2112). Neither affects rows 8 or 9. Routed to closed #2055: the hold was tracked there and resolved on 2026-10-03.

| # | Command | Expected | Observed (exit code + key output) | Date / Runner |
|---|---|---|---|---|
| 1 | grep -q -F GOOS=windows and 'go test -c' in the ci workflow | exit 0, cross-compile leg in PR CI | exit 0. ci.yml lines 44 to 46 are real run lines: windows/amd64 go vet, a per-package windows go test -c loop, and darwin/arm64 go vet. The workflow triggers on pull_request (line 16). Control: the same greps on ci.yml at the parent of #2064 exit 1, so the row detects the applied commit and is not vacuous | 2026-10-04 claude-opus-5-5-verifier |
| 2 | sh forge-ban.sh (tools/desk scripts) into a mktemp file; echo rc; grep the class e and class f lines | rc=0 and both class lines with desk and statusgen counts | exit 0. rc=0 / class e (glab subprocess): desk=0 statusgen=0 / class f (GitLab API literal): desk=45 statusgen=8 | 2026-10-04 claude-opus-5-5-verifier |
| 3 | sh forge-ban-probe.sh (tools/desk scripts) | PROBE PASS, exit 0, tools/desk clean afterwards | exit 0. PROBE PASS; git status --porcelain tools/desk printed nothing afterwards | 2026-10-04 claude-opus-5-5-verifier |
| 4 | grep -E for the desktools-v2/02 line and the desktools-v2/13 glab/gitlab-literal line in the stream forge-ban-baseline.txt | both lines, exit 0 | exit 0. desktools-v2/02 63 / desktools-v2/13 glab=0 gitlab-literal=45 | 2026-10-04 claude-opus-5-5-verifier |
| 5 | cd statusgen; go test -run '^TestVerifyRowPortability$' -v into a mktemp file; anchored grep for the top-level PASS line; then sh testdata/portability-mutation.sh | top-level PASS and MUTATION PASS, exit 0 | exit 0. --- PASS: TestVerifyRowPortability (0.00s). With the detector disabled, the mutation run shows 7 FAIL lines (top level plus notice-flow, tmp, sh, bash, findstr and tmpdir-plus-hardcode), then MUTATION PASS: disabled detector was rejected. git status --porcelain statusgen was empty afterwards | 2026-10-04 claude-opus-5-5-verifier |
| 6 | cd statusgen; go build into a mktemp dir; run it with --root . --lint, output discarded; echo rc | rc=0 | exit 0, rc=0. Re-run with output kept: LINT: PASS with 183 NOTICE lines tagged verify-row-portability. This is the NOTICE baseline at this SHA, unchanged from the 2026-10-02 and 2026-10-03 passes | 2026-10-04 claude-opus-5-5-verifier |
| 7 | awk for a 'cd tools/desk' line followed on the next line by 'go test ./...' in windows-ci-leg.yml, plus grep -q -F windows-latest | exit 0, a real tools/desk test step | exit 0. Lines 78 and 79 are the pwsh run block of the step named Native Windows desk suite in job desk-windows-tests, which runs on windows-latest (line 69). The row passes on the substance of the step, not on a comment. Control: the same awk on the file at the parent of #2064 exits 1. Triggers are still v-tag push and workflow_dispatch, with no pull_request trigger, so the Windows proof is release-time only (deliverable 4 allows this choice) | 2026-10-04 claude-opus-5-5-verifier |
| 8 | grep -n for 1145, 1146 and 1223 in brief-06; test $? -eq 1 | exit 0, nothing printed | exit 0, nothing printed | 2026-10-04 claude-opus-5-5-verifier |
| 9 | brief-06 contains cmd/cellctl, a gate: human line and both desktools-v2/13 trace markers | exit 0 | exit 0, no output | 2026-10-04 claude-opus-5-5-verifier |
| 10 | loop over the 24 section-3 issues, grep brief-13 for an owner row for each | exit 0, nothing printed | exit 0, nothing printed; all 24 owner rows present | 2026-10-04 claude-opus-5-5-verifier |
| 11 | grep -q -E for board row 10 of the desk-containers README reading implemented, verified or done | exit 0, row 10 itself, never todo | exit 0. Board row 10 (cellctl in Go) reads implemented (set by #2075) | 2026-10-04 claude-opus-5-5-verifier |
| 12 | statusgen --consumers --root . | exit 0, no routing claim disproved | exit 0. consumers: no brief files in the diff against 4f6bb6d59a755bbd6cf530667e1072e9cd9d75cf — nothing to corroborate. VACUOUS on merged main because the diff is empty. Supplemental run with --base 75fc02522: exit 0, 527 corroborated, 0 disproved, 426 unchecked. Correction: 75fc02522 is a post-merge status-regen commit, not the parent of #1996's merge (that parent is 3a06803b579a); this verifier did not run the true parent, and the row-12 conclusion rests on the re-witness at #1996's head below | 2026-10-04 claude-opus-5-5-verifier |
| 13 | loop over the 24 issues: gh api --paginate on each issue's comments, selecting bodies that contain the desktools-v2/13 triage marker (read-only GET) | exit 0, nothing printed | exit 0, nothing printed; no READ FAILED and no NO TRIAGE COMMENT lines. Hand check: every one of the 24 carries a marked comment (12 by the worker App on 2026-10-01, 12 by the human driver on 2026-10-02 at about 21:28Z), and the owner each comment names matches this brief's triage table in all 24 cases | 2026-10-04 claude-opus-5-5-verifier |

Execution witness: statusgen verifyrun --brief (v1.0.31) exit 0, 13 of 13 pass. The check run gives the following summary:

docs/streams/desktools-v2/brief-13-platform-gates-and-reconciliation.md: 13 pass, 0 fail, 0 could-not-run/missing (of 13 Verify rows)

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -q -F -e 'GOOS=windows' .github/workflows/ci.yml && grep -q -F -e 'go test -c' .github/workflows/ci.yml` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ 4f6bb6d59a75+dirty (on-behalf-of human:ian) (forge-identity) |
| 2 | `d=$(mktemp -d) && sh tools/desk/scripts/forge-ban.sh > "$d/r2.out"; echo rc=$?; grep -F -e 'class e (glab subprocess):' "$d/r2.out" && grep -F -e 'class f (GitLab API literal):' "$d/r2.out"` | pass exit=0 | sha256:abf4e9ac77f9 | 2026-10-04 | assay-verifier-app[bot] @ 4f6bb6d59a75+dirty (on-behalf-of human:ian) (forge-identity) |
| 3 | `sh tools/desk/scripts/forge-ban-probe.sh` | pass exit=0 | sha256:406d6e54ba92 | 2026-10-04 | assay-verifier-app[bot] @ 4f6bb6d59a75+dirty (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -E -e '^desktools-v2/02 [0-9]+$' docs/streams/desktools-v2/forge-ban-baseline.txt && grep -E -e '^desktools-v2/13 glab=[0-9]+ gitlab-literal=[0-9]+$' docs/streams/desktools-v2/forge-ban-baseline.txt` | pass exit=0 | sha256:df26954dfed8 | 2026-10-04 | assay-verifier-app[bot] @ 4f6bb6d59a75+dirty (on-behalf-of human:ian) (forge-identity) |
| 5 | `d=$(mktemp -d) && cd statusgen && go test -run '^TestVerifyRowPortability$' -v . > "$d/r5.out" 2>&1 && grep -E -e '^--- PASS: TestVerifyRowPortability \(' "$d/r5.out" && sh testdata/portability-mutation.sh` | pass exit=0 | sha256:df8ab9dca119 | 2026-10-04 | assay-verifier-app[bot] @ 4f6bb6d59a75+dirty (on-behalf-of human:ian) (forge-identity) |
| 6 | `d=$(mktemp -d) && cd statusgen && go build -o "$d/statusgen" . && cd .. && "$d/statusgen" --root . --lint >/dev/null 2>&1; echo rc=$?` | pass exit=0 | sha256:93ff7811a209 | 2026-10-04 | assay-verifier-app[bot] @ 4f6bb6d59a75+dirty (on-behalf-of human:ian) (forge-identity) |
| 7 | `awk 'prev ~ /^[ ]+cd tools\/desk[ ]*$/ && $0 ~ /^[ ]+go test \.\/\.\.\.[ ]*$/ {f=1} {prev=$0} END {exit !f}' .github/workflows/windows-ci-leg.yml && grep -q -F -e 'windows-latest' .github/workflows/windows-ci-leg.yml` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ 4f6bb6d59a75+dirty (on-behalf-of human:ian) (forge-identity) |
| 8 | `grep -n -e 1145 -e 1146 -e 1223 docs/streams/desktools-v2/brief-06-*.md; test $? -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ 4f6bb6d59a75+dirty (on-behalf-of human:ian) (forge-identity) |
| 9 | `f=$(ls docs/streams/desktools-v2/brief-06-*.md); grep -q -F -e 'cmd/cellctl' "$f" && grep -q -E -e '^gate: human$' "$f" && grep -q -F -e '(desktools-v2/13 GitLab row)' "$f" && grep -q -F -e '(desktools-v2/13 Windows row)' "$f"` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ 4f6bb6d59a75+dirty (on-behalf-of human:ian) (forge-identity) |
| 10 | `miss=0; for n in 641 642 1604 1621 1418 1424 1805 1435 1644 678 1569 1693 1573 1203 655 676 677 1477 1411 1412 1415 865 1667 1794; do grep -q -E -e "^[\|] #$n [\|] desktools-v2/[0-9]+" -e "^[\|] #$n [\|] out-of-scope" docs/streams/desktools-v2/brief-13-platform-gates-and-reconciliation.md \|\| { echo "UNOWNED #$n"; miss=1; }; done; test $miss -eq 0` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ 4f6bb6d59a75+dirty (on-behalf-of human:ian) (forge-identity) |
| 11 | `grep -q -E -e '^[\|] 10 [\|].*[\|] implemented [\|]' -e '^[\|] 10 [\|].*[\|] verified [\|]' -e '^[\|] 10 [\|].*[\|] done [\|]' docs/streams/desk-containers/README.md` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ 4f6bb6d59a75+dirty (on-behalf-of human:ian) (forge-identity) |
| 12 | `statusgen --consumers --root .` | pass exit=0 | sha256:bab953f01294 | 2026-10-04 | assay-verifier-app[bot] @ 4f6bb6d59a75+dirty (on-behalf-of human:ian) (forge-identity) |
| 13 | `miss=0; for n in 641 642 1604 1621 1418 1424 1805 1435 1644 678 1569 1693 1573 1203 655 676 677 1477 1411 1412 1415 865 1667 1794; do c=$(gh api --paginate "repos/{owner}/{repo}/issues/$n/comments" --jq '.[] \| select(.body \| contains("(desktools-v2/13 triage)")) \| .id') \|\| { echo "READ FAILED #$n"; miss=1; continue; }; test -n "$c" \|\| { echo "NO TRIAGE COMMENT #$n"; miss=1; }; done; test $miss -eq 0` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ 4f6bb6d59a75+dirty (on-behalf-of human:ian) (forge-identity) |

The witness Runner cell reads "4f6bb6d59a75+dirty". The only untracked path at run time was the verifier's own scratch directory (.v/); no tracked file differed from 4f6bb6d59.

Vacuity and findings

- Row 12 is VACUOUS on merged main because the diff is empty. The supplemental run with --base disproves nothing.
- Row 2 always prints rc=0, because the counter ends in an unconditional exit 0, so the two greps carry the row. Row 6 prints rc=0 when the lint is clean (a failing lint prints rc=1). Row 5 and the 183-line count show that the NOTICE fires; row 6 does not.
- Rows 1, 7 and 11 now pass on substance, and the controls for rows 1 and 7 exit 1 on the tree before #2064.
- Row 4 drift (advisory): the desktools-v2/02 baseline reads 63, and the counter now prints forge reach-around sites: 70 (desk 34, statusgen 36). It printed 69 at the 2026-10-03 pass. Classes e and f are excluded from that total, so the growth comes from classes a to d elsewhere, not from this brief.
- Class f self-count: 1 of the 45 is the probe script's own planted literal (in its heredoc), and the script header declares no carve-out for it. This is advisory and reproduces as recorded.
- Record gap: the 13 of 13 PASS on 03065900ca21 (#2076) landed as Evidence prose only. The only outcome record under the stream's verify-outcomes directory is still the verify-fail at 6e4197744 naming #2055, and the board row still reads implemented.

Risk-bearing value

Risk metadata is present and all four answers are no (regulatory, customer, irreversible, sensitive-data); gate model. The work touches .github/workflows (through #2064), so the enumeration was done anyway. It covered the implementing change #1996, the workflow commit #2064 and the row 7 edit #2066:
- desktools-v2/13 glab=0 gitlab-literal=45 @ forge-ban-baseline.txt:2 (desktools-v2 stream). A recorded baseline that would hide growth if wrong in the low direction. Reversible with --baseline.
- GLAB_SUBCMDS = issue, mr, api, auth, repo, release, ci, label, schedule, snippet, variable @ forge-ban.sh:172. Reversible.
- class f pattern = /api/v4 or gitlab.com @ forge-ban.sh:178. Reversible.
- BACKEND_EXCLUDE = forge_github, forge_gitlab @ forge-ban.sh:66. Reversible.
- detector regexes verifyOSMarker, verifyShellC, verifyFindstr and verifyTmpPath @ statusgen verifyportability.go:11 to 14. Advisory NOTICE only, reversible.
- cross-compile targets GOOS=windows GOARCH=amd64 and GOOS=darwin GOARCH=arm64 @ ci.yml:44 to 46. Reversible.
- -timeout 45s @ portability-mutation.sh:17, and GO_VERSION 1.25.0 @ windows-ci-leg.yml:65. Operational.

None of these is irreversible. Top-ranked entry:

RISK-VALUE: DERIVED — desktools-v2/13 glab=0 gitlab-literal=45 @ forge-ban-baseline.txt:2 — an independent recount with find and grep (not the counter) counted the lines matching /api/v4 or gitlab.com in .go and .sh files under tools/desk, tools/cellctl and plugins/assay. It excluded the two forge backends, _test.go, .test.sh and the counter itself. The recount gives 45, finds 0 exec.Command("glab" in non-test Go, and finds 8 such lines in non-test statusgen Go. All three agree with the counter's output and with the brief's definitions of classes e and f. Caveat: 1 of the 45 is the probe's own literal.

VERIFY: BLOCKED (#1281). On merged main 4f6bb6d59 all 13 rows pass by exit code and witness. By hand reading, 12 pass on substance; row 12 does not.
- Row 12 is vacuous on merged main: the diff is empty, so there is nothing to corroborate.
- A one-row re-witness at the implementing PR's own head (fdab117fac80, medici-finance/assay#1996; clean clone, origin/main at 4f6bb6d59, statusgen v1.0.31, verifier git identity) was not empty. Against merge-base f0c655a7929f it corroborated 3, disproved 0 and left 12 unchecked. But all 3 corroborated entries belong to desktools-v2/06. Every one of this brief's own consumer entries prints UNCHECKED ("this branch did not make this claim"), because the brief was authored earlier by medici-finance/assay#1878.
- Unchecked is not corroborated, so row 12 discharges on no tree. Fixing that needs a fixed base or a different row shape, which is the medici-finance/assay#1281 class and the brief owner's change.
- Rows 1, 7 and 11 now pass on substance, since #2055 has closed.
- Gate model, every risk answer no. Status stays `implemented`.
### Non-implementer verifier re-run on merged main 11228951d0a8 — 2026-10-06

Independent non-implementer pass on merged main 11228951d0a8. The forge main SHA matched at run time. Host darwin/arm64, go 1.27.1, installed statusgen v1.0.32. Rows 1 to 12 ran by hand under bash from the repository root, with a throwaway HOME and KUBECONFIG=/dev/null. Expectations were written from the brief before the diff was read. Commands are abbreviated; the full text is in the Verify table above.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | grep -q -F GOOS=windows and 'go test -c' in the ci workflow | pass exit=0 | ci.yml lines 44 to 46 are real run lines in the cross-compile job: windows/amd64 go vet, a per-package windows go test -c loop, and darwin/arm64 go vet. Control: the same greps on ci.yml at the parent of #2064 exit 1 | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) |
| 2 | sh forge-ban.sh (tools/desk scripts) into a mktemp file; echo rc; grep the class e and class f lines | pass exit=0 | rc=0 / class e (glab subprocess): desk=0 statusgen=0 / class f (GitLab API literal): desk=45 statusgen=8 | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) |
| 3 | sh forge-ban-probe.sh (tools/desk scripts) | pass exit=0 | PROBE PASS; git status --porcelain tools/desk printed nothing afterwards | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) |
| 4 | grep -E for the desktools-v2/02 line and the desktools-v2/13 glab/gitlab-literal line in the stream forge-ban-baseline.txt | pass exit=0 | desktools-v2/02 63 / desktools-v2/13 glab=0 gitlab-literal=45 | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) |
| 5 | cd statusgen; go test -run on the verify-row portability test with -v into a mktemp file; anchored top-level PASS grep; then sh testdata/portability-mutation.sh | pass exit=0 | top-level PASS (0.00s). A -v re-run lists 11 subtests that ran and passed: notice-flow, tmp, sh, bash, findstr, tmpdir, marked, marked-command, go, word and tmpdir-plus-hardcode. In the mutation run, with the detector disabled, the top level fails along with notice-flow, tmp, sh, bash, findstr and tmpdir-plus-hardcode; it then prints MUTATION PASS: disabled detector was rejected. Tree clean afterwards | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) |
| 6 | cd statusgen; go build into a mktemp dir; run it with --root . --lint, output discarded; echo rc | pass exit=0 | rc=0. Re-run with output kept: LINT: PASS with 183 NOTICE lines tagged verify-row-portability. This is the NOTICE baseline at this SHA, unchanged from the 10-02, 10-03 and 10-04 passes | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) |
| 7 | awk for a 'cd tools/desk' line followed by 'go test ./...' in windows-ci-leg.yml, plus grep -q -F windows-latest | pass exit=0 | lines 78 and 79 are the pwsh step in the job that runs on windows-latest (line 69). Control: the same awk at the parent of #2064 exits 1. Triggers are still v-tag push and workflow_dispatch only, so the Windows proof is release-time only, as deliverable 4 allows | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) |
| 8 | grep -n for 1145, 1146 and 1223 in brief-06; test $? -eq 1 | pass exit=0 | nothing printed | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) |
| 9 | brief-06 contains cmd/cellctl, a gate: human line and both desktools-v2/13 trace markers | pass exit=0 | no output | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) |
| 10 | loop over the 24 section-3 issues, grep brief-13 for an owner row for each | pass exit=0 | nothing printed; all 24 owner rows present | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) |
| 11 | grep -q -E for board row 10 of the desk-containers README reading implemented, verified or done | pass exit=0 | row 10 (cellctl in Go) reads implemented. Control: the same grep at the parent of #2075 exits 1 | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) |
| 12 | statusgen --consumers --root . | pass exit=0 (vacuous) | consumers: no brief files in the diff against 11228951d0a8 — nothing to corroborate. The installed v1.0.32 binary and a tree-built binary agree. VACUOUS on merged main because the diff is empty; the tool-side gap is #1281, still open. Supplemental tool run at the merge of #1996, with --base set to its first parent 3a06803b579a and --brief desktools-v2/13: exit 0, 0 corroborated, 0 disproved, 9 unchecked. All 9 were checked by hand and none is disproved: the forge-ban scripts and baseline, statusgen and brief-06 are in the #1996 diff; both workflows are in the #2064 diff; brief 06's GitLab and Windows rows are present (row 9); brief-14 pins #1145, #1146 and #1223 as regression rows; the desktools-v2/11 follow-up is open as #1841. One detail is inaccurate: board row 10 was corrected by #2075, a held act tracked by #2055, not by this brief's own PR | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) |
| 13 | loop over the 24 issues: gh api --paginate on each issue's comments, selecting bodies with the desktools-v2/13 triage marker | could-not-check | could-not-check. The verifier App credential returned 401. The only reachable read went through an ambient operator credential that the gh wrapper on PATH injected. That read is not the verifier's, so it is not counted. Re-run this row under a working verifier credential | 2026-10-06 | assay-verifier-app[bot] @ 11228951d0a8 (on-behalf-of human:ian) |

Vacuity and findings:
- Row 2 always prints rc=0, so its two greps carry it.
- Row 6's rc=0 shows that the tree still lints clean. Row 5 and the 183-line count show that the NOTICE fires.
- Row 12 is vacuous on merged main (#1281).
- Row 4 drift (advisory): the desktools-v2/02 baseline reads 63, and the counter now prints forge reach-around sites: 70 (desk 34, statusgen 36). Classes e and f are excluded from that total, so the growth comes from classes a to d elsewhere.
- Class f self-count: 1 of the 45 is the probe script's own planted literal, and the header declares no carve-out for it. Advisory.
- The per-PR changelog fragments for this brief were aggregated into CHANGELOG.md by the v1.0.32 roll-up and are no longer present under changelog/. This is not a Verify row.

Risk-bearing value:

Risk metadata is present, all four answers are no, and the gate is model. The work touches .github/workflows (via #2064), so the enumeration was done anyway. It covered #1996, #2064 and #2066:
- desktools-v2/13 glab=0 gitlab-literal=45 @ forge-ban-baseline.txt:2 (desktools-v2 stream). A recorded baseline; wrong in the low direction it would hide growth. Reversible with --baseline.
- GLAB_SUBCMDS = issue, mr, api, auth, repo, release, ci, label, schedule, snippet, variable @ forge-ban.sh:172. Reversible.
- class f pattern = /api/v4 or gitlab.com @ forge-ban.sh:178 and :186. Reversible.
- BACKEND_EXCLUDE = forge_github, forge_gitlab @ forge-ban.sh:66. Reversible.
- detector regexes verifyOSMarker, verifyShellC, verifyFindstr and verifyTmpPath @ statusgen verifyportability.go:11 to 14. Advisory NOTICE (StatusAdvisory @ statusgen enforcementstatus.go:111). Reversible.
- cross-compile targets GOOS=windows GOARCH=amd64 and GOOS=darwin GOARCH=arm64 @ ci.yml:44 to 46. Reversible.
- runs-on windows-latest @ windows-ci-leg.yml:69, and -timeout 45s @ portability-mutation.sh:17. Operational.

None of these is irreversible. Top-ranked entry:

RISK-VALUE: DERIVED — desktools-v2/13 glab=0 gitlab-literal=45 @ forge-ban-baseline.txt:2 — a separate recount (a different tool, not the counter) counted lines matching /api/v4 or gitlab.com in .go and .sh files under tools/desk, tools/cellctl and plugins/assay. It excluded the two forge backends, _test.go, .test.sh and the counter itself, which is the brief's class f definition. The recount gives 45 across 21 files. It also finds 0 exec.Command("glab" in non-test Go (class e) and 8 statusgen literals. All three agree with the counter at this SHA. Caveat: 1 of the 45 is the probe's own literal.

VERIFY: BLOCKED — 12/13 rows pass by hand; row 13 could-not-check (credential); no clean execution witness (row 13 witness ran under an injected ambient credential and is not landed).
### Non-implementer verifier run — 2026-10-14 claude-opus-5-5 verifier at 0cb90058470f (13 of 13 rows pass; row 12 vacuous on merged main, medici-finance/assay#1281)

Verification-Attestation: medici-finance/assay#2894 run=b894832148ba82d615881bc5257fb70a60102b70597e1934d042e2305884d285 source=0cb90058470f9608f76b96e2055e7327d9c5c178 brief=docs/streams/desktools-v2/brief-13-platform-gates-and-reconciliation.md model=claude-opus-5-5 tier=strong

- Target: merged main @ 0cb90058470f9608f76b96e2055e7327d9c5c178, the admitted source. The forge main ref matched at run time, and the dispatcher's check-verifier receipt was re-checked after the run.
- Host and tools: darwin/arm64, go 1.27.2, installed statusgen v1.0.41, KUBECONFIG=/dev/null.
- How the rows ran: every row ran by hand under bash from the repository root, in a no-hardlinks scratch clone detached at the target SHA. The home stayed untouched.
- Row 13 credential: a verifier App installation token (/installation/repositories answers; /user returns 403), with an isolated gh config, GITHUB_TOKEN unset, and the system gh called directly, so no ambient operator credential could serve the read.
- Expectations were taken from the brief's Verify and DoD text before earlier Evidence was read.
- Prior hold: the last outcome record (2026-10-06, blocked, environment, medici-finance/assay#1631) held row 13 on a verifier-credential 401. Row 13 now reads cleanly under the verifier App credential, so that hold is cleared for this run. medici-finance/assay#1631 itself stays open.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -q -F -e 'GOOS=windows' .github/workflows/ci.yml && grep -q -F -e 'go test -c' .github/workflows/ci.yml` | pass exit=0 | exit 0. ci.yml lines 44 to 46 are real run lines: windows/amd64 go vet, the per-package windows go test -c loop, and darwin/arm64 go vet | 2026-10-14 | assay-verifier-app[bot] @ 0cb90058470f (on-behalf-of human:ian) (git-config) |
| 2 | `d=$(mktemp -d) && sh tools/desk/scripts/forge-ban.sh > "$d/r2.out"; echo rc=$?; grep -F -e 'class e (glab subprocess):' "$d/r2.out" && grep -F -e 'class f (GitLab API literal):' "$d/r2.out"` | pass exit=0 | rc=0 / class e (glab subprocess): desk=0 statusgen=0 / class f (GitLab API literal): desk=47 statusgen=8. Also printed: forge reach-around sites: 71 (desk: 35, statusgen: 36), statusgen sites: 36, forgeread sites: 0 | 2026-10-14 | assay-verifier-app[bot] @ 0cb90058470f (on-behalf-of human:ian) (git-config) |
| 3 | `sh tools/desk/scripts/forge-ban-probe.sh` | pass exit=0 | PROBE PASS, exit 0; git status --porcelain tools/desk printed nothing afterwards | 2026-10-14 | assay-verifier-app[bot] @ 0cb90058470f (on-behalf-of human:ian) (git-config) |
| 4 | `grep -E -e '^desktools-v2/02 [0-9]+$' docs/streams/desktools-v2/forge-ban-baseline.txt && grep -E -e '^desktools-v2/13 glab=[0-9]+ gitlab-literal=[0-9]+$' docs/streams/desktools-v2/forge-ban-baseline.txt` | pass exit=0 | desktools-v2/02 63 / desktools-v2/13 glab=0 gitlab-literal=45 | 2026-10-14 | assay-verifier-app[bot] @ 0cb90058470f (on-behalf-of human:ian) (git-config) |
| 5 | `d=$(mktemp -d) && cd statusgen && go test -run '^TestVerifyRowPortability$' -v . > "$d/r5.out" 2>&1 && grep -E -e '^--- PASS: TestVerifyRowPortability \(' "$d/r5.out" && sh testdata/portability-mutation.sh` | pass exit=0 | top-level PASS line (0.00s); 11 subtests passed. The mutation run with the detector disabled fails the top level plus notice-flow, tmp, sh, bash, findstr and tmpdir-plus-hardcode, then prints MUTATION PASS: disabled detector was rejected. git status --porcelain statusgen was empty afterwards | 2026-10-14 | assay-verifier-app[bot] @ 0cb90058470f (on-behalf-of human:ian) (git-config) |
| 6 | `d=$(mktemp -d) && cd statusgen && go build -o "$d/statusgen" . && cd .. && "$d/statusgen" --root . --lint >/dev/null 2>&1; echo rc=$?` | pass exit=0 | rc=0. Re-run with output kept: LINT: PASS with 184 NOTICE lines tagged verify-row-portability, which is the NOTICE baseline at this SHA (183 at the 10-02 through 10-06 passes). Environment note: the first attempt printed rc=1 with 9 verify-obligation problems on other briefs, because the scratch clone's origin/main was a stale local branch (40b45182b3c0). With the clone's origin/main set to the target SHA, which matches forge main and the checkout base, it printed rc=0. No problem named brief 13 | 2026-10-14 | assay-verifier-app[bot] @ 0cb90058470f (on-behalf-of human:ian) (git-config) |
| 7 | `awk 'prev ~ /^[ ]+cd tools\/desk[ ]*$/ && $0 ~ /^[ ]+go test \.\/\.\.\.[ ]*$/ {f=1} {prev=$0} END {exit !f}' .github/workflows/windows-ci-leg.yml && grep -q -F -e 'windows-latest' .github/workflows/windows-ci-leg.yml` | pass exit=0 | exit 0. Lines 78 and 79 are the pwsh run block of a job on windows-latest (line 69). Triggers are still the v-tag push and workflow_dispatch only, so the Windows proof is release-time only, as deliverable 4 allows | 2026-10-14 | assay-verifier-app[bot] @ 0cb90058470f (on-behalf-of human:ian) (git-config) |
| 8 | `grep -n -e 1145 -e 1146 -e 1223 docs/streams/desktools-v2/brief-06-*.md; test $? -eq 1` | pass exit=0 | exit 0, nothing printed | 2026-10-14 | assay-verifier-app[bot] @ 0cb90058470f (on-behalf-of human:ian) (git-config) |
| 9 | `f=$(ls docs/streams/desktools-v2/brief-06-*.md); grep -q -F -e 'cmd/cellctl' "$f" && grep -q -E -e '^gate: human$' "$f" && grep -q -F -e '(desktools-v2/13 GitLab row)' "$f" && grep -q -F -e '(desktools-v2/13 Windows row)' "$f"` | pass exit=0 | exit 0, no output | 2026-10-14 | assay-verifier-app[bot] @ 0cb90058470f (on-behalf-of human:ian) (git-config) |
| 10 | `miss=0; for n in 641 642 1604 1621 1418 1424 1805 1435 1644 678 1569 1693 1573 1203 655 676 677 1477 1411 1412 1415 865 1667 1794; do grep -q -E -e "^[\|] #$n [\|] desktools-v2/[0-9]+" -e "^[\|] #$n [\|] out-of-scope" docs/streams/desktools-v2/brief-13-platform-gates-and-reconciliation.md \|\| { echo "UNOWNED #$n"; miss=1; }; done; test $miss -eq 0` | pass exit=0 | exit 0, nothing printed; all 24 owner rows present | 2026-10-14 | assay-verifier-app[bot] @ 0cb90058470f (on-behalf-of human:ian) (git-config) |
| 11 | `grep -q -E -e '^[\|] 10 [\|].*[\|] implemented [\|]' -e '^[\|] 10 [\|].*[\|] verified [\|]' -e '^[\|] 10 [\|].*[\|] done [\|]' docs/streams/desk-containers/README.md` | pass exit=0 | exit 0. Board row 10 (cellctl in Go) itself reads implemented | 2026-10-14 | assay-verifier-app[bot] @ 0cb90058470f (on-behalf-of human:ian) (git-config) |
| 12 | `statusgen --consumers --root .` | pass exit=0 | exit 0. consumers: no brief files in the diff against 0cb90058470f — nothing to corroborate. VACUOUS on merged main because the diff is empty (tool-side gap medici-finance/assay#1281, still open). Supplemental run at the implementing commit 85aeeb8e1f8d (medici-finance/assay#1996) with --base set to its parent 3a06803b579a and --brief desktools-v2/13: exit 0, 0 corroborated, 0 disproved, 9 unchecked. All 9 were checked by hand and none is disproved: the forge-ban scripts and baseline (rows 2 to 4), the statusgen notice and test (rows 5 and 6), both workflows (rows 1 and 7), the brief-06 re-derivation and its GitLab and Windows rows (rows 8 and 9), the triage comments (row 13), and board row 10 (row 11, corrected by a held act rather than this brief's own PR). Brief 14 pins 1145, 1146 and 1223, and the desktools-v2/11 follow-up is open as medici-finance/assay#1841 | 2026-10-14 | assay-verifier-app[bot] @ 0cb90058470f (on-behalf-of human:ian) (git-config) |
| 13 | `miss=0; for n in 641 642 1604 1621 1418 1424 1805 1435 1644 678 1569 1693 1573 1203 655 676 677 1477 1411 1412 1415 865 1667 1794; do c=$(gh api --paginate "repos/{owner}/{repo}/issues/$n/comments" --jq '.[] \| select(.body \| contains("(desktools-v2/13 triage)")) \| .id') \|\| { echo "READ FAILED #$n"; miss=1; continue; }; test -n "$c" \|\| { echo "NO TRIAGE COMMENT #$n"; miss=1; }; done; test $miss -eq 0` | pass exit=0 | exit 0, nothing printed: no READ FAILED and no NO TRIAGE COMMENT lines. The read ran under the verifier App installation credential, with GH_REPO set to the target repo for the owner and repo placeholders. Control: the same select on medici-finance/assay#1836 returns 0 ids, so the filter discriminates. Hand check: in all 24 cases, the owner each marked comment names matches this brief's triage table | 2026-10-14 | assay-verifier-app[bot] @ 0cb90058470f (on-behalf-of human:ian) (git-config) |

Findings (advisory; none is a Verify-row failure):

- **Row 2 rc depends on forgeread.** Row 2's rc=0 no longer holds unconditionally. medici-finance/assay#2536 added a forgeread column to the counter, and the counter now exits 1 when that column is above 0. It reads 0 at this SHA. The desk and statusgen columns, and classes e and f, stay advisory.
- **Class f baseline drift.** The live class f desk count is 47 against the recorded gitlab-literal=45. An independent git grep recount, a different tool from the counter, gives 45 at the implementing commit 85aeeb8e1f8d and 47 at this SHA. The +2 is one new line in cellctl envkeys.go (GITLAB_API_BASE help text) and one more in deskkit forgegit.go, from later PRs. Classes e and f feed no total and no gate, and the drift is corrected with --baseline.
- **Class a to d drift.** The desktools-v2/02 baseline reads 63, and the reach-around total is now 71. The 10-06 pass recorded 70. Classes e and f are excluded from that total.
- **Probe self-count.** 1 of the class f desk lines is still the probe script's own planted literal, and the script header declares no carve-out for it.
- **Row 12 vacuity.** Row 12 is vacuous on merged main (medici-finance/assay#1281). Its Expect holds only by the supplemental run and the hand check above.

Risk-bearing value. Risk metadata is present: regulatory, customer, irreversible and sensitive-data all read no, and the gate is model. The diff touches .github/workflows (via medici-finance/assay#2064), so the enumeration was done anyway. It covered the implementing commit (#1996), the workflow commit (#2064), the row 7 edit (#2066), and the later counter change (#2536):

- desktools-v2/13 glab=0 gitlab-literal=45 @ forge-ban-baseline.txt:2 (desktools-v2 stream). A recorded baseline: wrong in the low direction, it hides growth. Reversible with --baseline.
- desktools-v2/02 63 @ forge-ban-baseline.txt:1. Not this brief's value, and untouched by it. Reversible.
- GLAB_SUBCMDS = issue, mr, api, auth, repo, release, ci, label, schedule, snippet, variable @ forge-ban.sh:181. Reversible.
- class f pattern = /api/v4 or gitlab.com @ forge-ban.sh:187 and :195. Reversible.
- BACKEND_EXCLUDE = forge_github, forge_gitlab @ forge-ban.sh:75. Reversible.
- the 4 detector regexes verifyOSMarker, verifyShellC, verifyFindstr and verifyTmpPath @ statusgen verifyportability.go:11 to 14. Advisory only: the row is StatusAdvisory @ statusgen enforcementstatus.go:113. Reversible.
- cross-compile targets GOOS=windows GOARCH=amd64 and GOOS=darwin GOARCH=arm64 @ ci.yml:44 to 46. Reversible.
- runs-on windows-latest @ windows-ci-leg.yml:69, and -timeout 45s @ portability-mutation.sh:17. Operational.

None of these is irreversible. Top-ranked entry:

RISK-VALUE: DERIVED — desktools-v2/13 glab=0 gitlab-literal=45 @ docs/streams/desktools-v2/forge-ban-baseline.txt:2 — an independent git grep recount (not the counter) used the brief's class e and class f definitions: lines matching /api/v4 or gitlab.com in .go and .sh files under tools/desk, tools/cellctl and plugins/assay, excluding the two forge backends, _test.go, .test.sh and the counter itself. The recount gives 45 at the implementing commit 85aeeb8e1f8d, so the recorded value was right for the tree it baselined. It also finds 0 non-test exec.Command("glab" sites (glab=0) and 8 statusgen literals, which agree with the counter. Caveat: the live count at 0cb90058470f is 47, a +2 drift from later PRs. The value is advisory and feeds no gate.

**VERIFY: PASS** — 13 of 13 rows pass at merged main 0cb90058470f. Row 12 is vacuous on merged main (medici-finance/assay#1281, open), with its Expect upheld by the supplemental run at the implementing commit (0 disproved) and a hand check of all 9 unchecked claims. The prior row 13 credential hold (medici-finance/assay#1631) is cleared for this run. Gate model, every risk answer no.

### Non-implementer verifier run — 2026-10-16 claude-opus-5-5 verifier at 1948dc087370 (13 of 13 rows pass; row 12 vacuous on merged main, medici-finance/assay#1281)

Verification-Attestation: medici-finance/assay#3135 run=d2e4a9a07ab9fed0628c23d117fb77ed54a533a9f641e86b6306529437cefe86 source=1948dc0873705930b202f0e747047df844010d1a brief=docs/streams/desktools-v2/brief-13-platform-gates-and-reconciliation.md model=claude-opus-5-5 tier=any

- Target: merged main @ 1948dc0873705930b202f0e747047df844010d1a, the admitted source; the forge main ref matched at run time. The dispatcher's check-verifier receipt admitted the run before any row and was re-checked after the Evidence edit.
- Host and tools: darwin/arm64, go 1.27.2, installed statusgen v1.0.45 for row 12, KUBECONFIG=/dev/null.
- How the rows ran: every row ran by hand under bash from the repository root of a no-hardlinks scratch clone detached at the target SHA (its origin/main set to that SHA), with a throwaway HOME and TMPDIR and the real Go caches. The home stayed untouched apart from this Evidence block.
- Expectations were written from the brief's Verify and DoD text before earlier Evidence was read.

| # | Command | Expected | Observed | Runner |
|---|---------|----------|----------|--------|
| 1 | `grep -q -F -e 'GOOS=windows' .github/workflows/ci.yml && grep -q -F -e 'go test -c' .github/workflows/ci.yml` | exit 0 — the cross-compile leg is in PR CI | exit 0. ci.yml lines 44 to 46 are run lines of a step in a workflow triggered on push and pull_request: windows/amd64 go vet, the per-package windows go test -c loop, and darwin/arm64 go vet | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 2 | `d=$(mktemp -d) && sh tools/desk/scripts/forge-ban.sh > "$d/r2.out"; echo rc=$?; grep -F -e 'class e (glab subprocess):' "$d/r2.out" && grep -F -e 'class f (GitLab API literal):' "$d/r2.out"` | prints rc=0 and both new class lines with their desk and statusgen counts | exit 0: rc=0 / class e (glab subprocess): desk=0 statusgen=0 / class f (GitLab API literal): desk=47 statusgen=8. Also printed: forge reach-around sites: 72 (desk: 36, statusgen: 36), statusgen sites: 36, forgeread sites: 0 | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 3 | `sh tools/desk/scripts/forge-ban-probe.sh` | PROBE PASS, exit 0, tools/desk clean afterwards | PROBE PASS, exit 0; git status --porcelain tools/desk printed nothing afterwards | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 4 | `grep -E -e '^desktools-v2/02 [0-9]+$' docs/streams/desktools-v2/forge-ban-baseline.txt && grep -E -e '^desktools-v2/13 glab=[0-9]+ gitlab-literal=[0-9]+$' docs/streams/desktools-v2/forge-ban-baseline.txt` | prints the desktools-v2/02 line and the desktools-v2/13 line (exit 0) | exit 0: desktools-v2/02 63 / desktools-v2/13 glab=0 gitlab-literal=45 | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 5 | `d=$(mktemp -d) && cd statusgen && go test -run '^TestVerifyRowPortability$' -v . > "$d/r5.out" 2>&1 && grep -E -e '^--- PASS: TestVerifyRowPortability \(' "$d/r5.out" && sh testdata/portability-mutation.sh` | top-level PASS line and MUTATION PASS: disabled detector was rejected (exit 0) | exit 0: --- PASS: TestVerifyRowPortability (0.00s), 11 subtests passed. The mutation run with the detector disabled failed the top level plus notice-flow, tmp, sh, bash, findstr and tmpdir-plus-hardcode, then printed MUTATION PASS: disabled detector was rejected. git status --porcelain statusgen was empty afterwards | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 6 | `d=$(mktemp -d) && cd statusgen && go build -o "$d/statusgen" . && cd .. && "$d/statusgen" --root . --lint >/dev/null 2>&1; echo rc=$?` | rc=0, the existing tree still lints clean, NOTICE baseline recorded | rc=0. Re-run with output kept: rc=0 with 187 NOTICE lines tagged verify-row-portability, the NOTICE baseline at this SHA (184 at the 2026-10-14 pass) | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 7 | `awk 'prev ~ /^[ ]+cd tools\/desk[ ]*$/ && $0 ~ /^[ ]+go test \.\/\.\.\.[ ]*$/ {f=1} {prev=$0} END {exit !f}' .github/workflows/windows-ci-leg.yml && grep -q -F -e 'windows-latest' .github/workflows/windows-ci-leg.yml` | exit 0 — a step does cd tools/desk then go test ./... on its next line, on windows-latest | exit 0. Lines 78 and 79 are the pwsh run block of job desk-windows-tests on windows-latest (line 69). Triggers are still the v-tag push and workflow_dispatch only (lines 50 to 54), so the Windows proof is release-time only, as deliverable 4 allows | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 8 | `grep -n -e 1145 -e 1146 -e 1223 docs/streams/desktools-v2/brief-06-*.md; test $? -eq 1` | exit 0 and nothing printed | exit 0, nothing printed | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 9 | `f=$(ls docs/streams/desktools-v2/brief-06-*.md); grep -q -F -e 'cmd/cellctl' "$f" && grep -q -E -e '^gate: human$' "$f" && grep -q -F -e '(desktools-v2/13 GitLab row)' "$f" && grep -q -F -e '(desktools-v2/13 Windows row)' "$f"` | exit 0 — brief 06 targets cmd/cellctl, keeps gate human, carries both trace-marked rows | exit 0, no output (brief-06-installation-token-scoping.md) | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 10 | `miss=0; for n in 641 642 1604 1621 1418 1424 1805 1435 1644 678 1569 1693 1573 1203 655 676 677 1477 1411 1412 1415 865 1667 1794; do grep -q -E -e "^[\|] #$n [\|] desktools-v2/[0-9]+" -e "^[\|] #$n [\|] out-of-scope" docs/streams/desktools-v2/brief-13-platform-gates-and-reconciliation.md \|\| { echo "UNOWNED #$n"; miss=1; }; done; test $miss -eq 0` | exit 0 and nothing printed — all 24 issues have a triage row | exit 0, nothing printed; all 24 owner rows present | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 11 | `grep -q -E -e '^[\|] 10 [\|].*[\|] implemented [\|]' -e '^[\|] 10 [\|].*[\|] verified [\|]' -e '^[\|] 10 [\|].*[\|] done [\|]' docs/streams/desk-containers/README.md` | exit 0 — board row 10 reads implemented, verified or done | exit 0. desk-containers board row 10 (cellctl in Go) itself reads implemented | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 12 | `statusgen --consumers --root .` | exit 0; no routing claim disproved by the diff | exit 0. consumers: no brief files in the diff against 1948dc087370 — vacuous on merged main (tool-side gap medici-finance/assay#1281). Supplemental run at the implementing commit 85aeeb8e1f8d with --base set to its parent 3a06803b579a and --brief desktools-v2/13: exit 0, 0 corroborated, 0 disproved, 9 unchecked. The 9 were checked by hand against rows 1 to 11 and 13 and none is disproved; brief 14 cites 1145, 1146 and 1223 in 13 lines | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 13 | `miss=0; for n in 641 642 1604 1621 1418 1424 1805 1435 1644 678 1569 1693 1573 1203 655 676 677 1477 1411 1412 1415 865 1667 1794; do c=$(gh api --paginate "repos/{owner}/{repo}/issues/$n/comments" --jq '.[] \| select(.body \| contains("(desktools-v2/13 triage)")) \| .id') \|\| { echo "READ FAILED #$n"; miss=1; continue; }; test -n "$c" \|\| { echo "NO TRIAGE COMMENT #$n"; miss=1; }; done; test $miss -eq 0` | exit 0 and nothing printed — all 24 issues carry a triage comment | exit 0, nothing printed: no READ FAILED and no NO TRIAGE COMMENT lines. Read under the verifier App installation credential (the user endpoint answers 403) with GH_REPO set to the target repo. Control: the same select on medici-finance/assay#1836 returns 0 ids. Hand check: for all 24 issues the owner the marked comment names matches this brief's triage table | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |

Findings (advisory; none is a Verify-row failure):

- **Class a to d drift.** The forge reach-around total reads 72 against the recorded desktools-v2/02 baseline of 63 (71 at the 2026-10-14 pass). Classes e and f stay excluded from that total; row 3 proves the exclusion.
- **Class f baseline drift.** The live class f desk count is 47 against the recorded gitlab-literal=45, unchanged since the 2026-10-14 pass. Classes e and f feed no total and no gate; the drift is corrected with --baseline.
- **Probe self-count.** One class f desk line is still the probe script's own planted literal (forge-ban-probe.sh line 14), with no carve-out declared in the counter header.
- **Row 12 vacuity.** Row 12 is vacuous on merged main (medici-finance/assay#1281); its Expect holds by the supplemental run at the implementing commit and the hand check.

Risk-bearing value. Risk metadata is present (regulatory, customer, irreversible and sensitive-data all no; gate model). The delivered work touches .github/workflows, so the enumeration was done anyway over the counter script, the baseline file, the statusgen portability detector, the mutation script and both workflow files at this SHA:

- desktools-v2/13 glab=0 gitlab-literal=45 @ forge-ban-baseline.txt:2. A recorded baseline: wrong low, it hides growth. Reversible with --baseline.
- desktools-v2/02 63 @ forge-ban-baseline.txt:1. Not this brief's value and untouched by it. Reversible.
- GLAB_SUBCMDS = issue, mr, api, auth, repo, release, ci, label, schedule, snippet, variable @ forge-ban.sh:181. Reversible.
- class f pattern = /api/v4 or gitlab.com @ forge-ban.sh:187 and :195. Reversible.
- BACKEND_EXCLUDE = forge_github, forge_gitlab @ forge-ban.sh:75. Reversible.
- detector regexes verifyOSMarker, verifyShellC, verifyFindstr, verifyTmpPath @ statusgen verifyportability.go:11 to 14. NOTICE-level only. Reversible.
- cross-compile targets GOOS=windows GOARCH=amd64 and GOOS=darwin GOARCH=arm64 @ ci.yml:44 to 46. Reversible.
- runs-on windows-latest @ windows-ci-leg.yml:69 and -timeout 45s @ portability-mutation.sh:17. Operational.

None is irreversible. Top-ranked entry:

RISK-VALUE: DERIVED — desktools-v2/13 glab=0 gitlab-literal=45 @ docs/streams/desktools-v2/forge-ban-baseline.txt:2 — an independent git grep recount (not the counter) applying the brief's class e and f definitions (lines matching /api/v4 or gitlab.com in .go and .sh files under tools/desk, tools/cellctl and plugins/assay, excluding the two forge backends, _test.go, .test.sh and the counter itself; non-test exec.Command("glab" sites) gives gitlab-literal 45 and glab 0 at the implementing commit 85aeeb8e1f8d, the commit that last wrote the baseline file, so the recorded value is right for the tree it baselined. The same recount gives 47 at 1948dc087370, matching the counter's live desk count, and 8 statusgen literals, matching the counter. Advisory value; it feeds no gate.

**VERIFY: PASS** — 13 of 13 rows pass at merged main 1948dc087370. Row 12 is vacuous on merged main (medici-finance/assay#1281, open), with its Expect upheld by the supplemental run at the implementing commit (0 disproved) and a hand check of the 9 unchecked claims. Gate model, every risk answer no.

### Non-implementer verifier run — 2026-10-22 claude-opus-5-5 verifier at 05dc9fbc88f2 (13 of 13 rows pass; row 12 vacuous on merged main, medici-finance/assay#1281)

- Target: merged main @ 05dc9fbc88f2b1b23cd78bebc8634c564327eca4, the admitted source; the forge main ref matched at run time. The dispatcher's check-verifier receipt admitted the run before any row and was re-checked after this Evidence edit.
- Host and tools: darwin/arm64, go 1.27.2, installed statusgen v1.0.54, KUBECONFIG=/dev/null, with TMPDIR and the Go build cache pointed at a scratch directory outside the home.
- Execution witness: the table below was written by statusgen verifyrun in write mode, from the admitted home on a clean tree at the source SHA. Its stamps and its Verification-Attestation line are as the tool wrote them. Every row is plain check class, so no row needed the network-off sandbox. For row 13, the run used a verifier App installation credential (the user endpoint answers 403), with an isolated gh config, GITHUB_TOKEN unset, and GH_REPO set to the target repo for the owner and repo placeholders.
- Hand reading: every row was then re-run by hand from the root of a no-hardlinks scratch clone detached at the source SHA, with its origin/main set to that SHA. The observations follow the table. The hand runs add no rows.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -q -F -e 'GOOS=windows' .github/workflows/ci.yml && grep -q -F -e 'go test -c' .github/workflows/ci.yml` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 2 | `d=$(mktemp -d) && sh tools/desk/scripts/forge-ban.sh > "$d/r2.out"; echo rc=$?; grep -F -e 'class e (glab subprocess):' "$d/r2.out" && grep -F -e 'class f (GitLab API literal):' "$d/r2.out"` | pass exit=0 | sha256:55fbc15862fa | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 3 | `sh tools/desk/scripts/forge-ban-probe.sh` | pass exit=0 | sha256:406d6e54ba92 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -E -e '^desktools-v2/02 [0-9]+$' docs/streams/desktools-v2/forge-ban-baseline.txt && grep -E -e '^desktools-v2/13 glab=[0-9]+ gitlab-literal=[0-9]+$' docs/streams/desktools-v2/forge-ban-baseline.txt` | pass exit=0 | sha256:df26954dfed8 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 5 | `d=$(mktemp -d) && cd statusgen && go test -run '^TestVerifyRowPortability$' -v . > "$d/r5.out" 2>&1 && grep -E -e '^--- PASS: TestVerifyRowPortability \(' "$d/r5.out" && sh testdata/portability-mutation.sh` | pass exit=0 | sha256:9308c209d31f | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 6 | `d=$(mktemp -d) && cd statusgen && go build -o "$d/statusgen" . && cd .. && "$d/statusgen" --root . --lint >/dev/null 2>&1; echo rc=$?` | pass exit=0 | sha256:93ff7811a209 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 7 | `awk 'prev ~ /^[ ]+cd tools\/desk[ ]*$/ && $0 ~ /^[ ]+go test \.\/\.\.\.[ ]*$/ {f=1} {prev=$0} END {exit !f}' .github/workflows/windows-ci-leg.yml && grep -q -F -e 'windows-latest' .github/workflows/windows-ci-leg.yml` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 8 | `grep -n -e 1145 -e 1146 -e 1223 docs/streams/desktools-v2/brief-06-*.md; test $? -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 9 | `f=$(ls docs/streams/desktools-v2/brief-06-*.md); grep -q -F -e 'cmd/cellctl' "$f" && grep -q -E -e '^gate: human$' "$f" && grep -q -F -e '(desktools-v2/13 GitLab row)' "$f" && grep -q -F -e '(desktools-v2/13 Windows row)' "$f"` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 10 | `miss=0; for n in 641 642 1604 1621 1418 1424 1805 1435 1644 678 1569 1693 1573 1203 655 676 677 1477 1411 1412 1415 865 1667 1794; do grep -q -E -e "^[\|] #$n [\|] desktools-v2/[0-9]+" -e "^[\|] #$n [\|] out-of-scope" docs/streams/desktools-v2/brief-13-platform-gates-and-reconciliation.md \|\| { echo "UNOWNED #$n"; miss=1; }; done; test $miss -eq 0` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 11 | `grep -q -E -e '^[\|] 10 [\|].*[\|] implemented [\|]' -e '^[\|] 10 [\|].*[\|] verified [\|]' -e '^[\|] 10 [\|].*[\|] done [\|]' docs/streams/desk-containers/README.md` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 12 | `statusgen --consumers --root .` | pass exit=0 | sha256:194ac591934f | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 13 | `miss=0; for n in 641 642 1604 1621 1418 1424 1805 1435 1644 678 1569 1693 1573 1203 655 676 677 1477 1411 1412 1415 865 1667 1794; do c=$(gh api --paginate "repos/{owner}/{repo}/issues/$n/comments" --jq '.[] \| select(.body \| contains("(desktools-v2/13 triage)")) \| .id') \|\| { echo "READ FAILED #$n"; miss=1; continue; }; test -n "$c" \|\| { echo "NO TRIAGE COMMENT #$n"; miss=1; }; done; test $miss -eq 0` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4027 run=8dd0be2a631c56c724af29f59d9e8720bb5c396577379a85a58ba73fe92bf379 source=05dc9fbc88f2b1b23cd78bebc8634c564327eca4 brief=docs/streams/desktools-v2/brief-13-platform-gates-and-reconciliation.md model=claude-opus-5-5 tier=strong

Hand reading at 05dc9fbc88f2 (scratch clone, same SHA):

- Row 1: ci.yml lines 155 to 157 are run lines of the step "Compile desk platform code and test binaries": windows/amd64 go vet, the per-package windows go test -c loop, and darwin/arm64 go vet. The workflow triggers on push to main and on pull_request (lines 18 to 20).
- Row 2: rc=0 / class e (glab subprocess): desk=0 statusgen=0 / class f (GitLab API literal): desk=56 statusgen=8. Also printed: forge reach-around sites: 74 (desk: 34, statusgen: 40), statusgen sites: 40, forgeread sites: 0.
- Row 3: PROBE PASS, exit 0; git status --porcelain tools/desk printed nothing afterwards.
- Row 4: desktools-v2/02 63 / desktools-v2/13 glab=0 gitlab-literal=45.
- Row 5: --- PASS: TestVerifyRowPortability (0.00s), 11 subtests passed. The mutation run with the detector disabled failed the top level plus notice-flow, tmp, sh, bash, findstr and tmpdir-plus-hardcode, then printed MUTATION PASS: disabled detector was rejected. git status --porcelain statusgen was empty afterwards.
- Row 6: rc=0. Re-run with output kept: rc=0 with 187 lines tagged verify-row-portability, the NOTICE baseline at this SHA (187 at the 2026-10-16 pass).
- Row 7: windows-ci-leg.yml lines 84 and 85 are the run block (cd tools/desk, then go test ./...) of a job on windows-latest (line 75). Triggers are the v-tag push, a nightly schedule (cron 41 3 * * *) and workflow_dispatch, with no pull_request trigger. So the Windows proof runs nightly and at release, not on a PR, which deliverable 4 allows.
- Row 8: exit 0, nothing printed.
- Row 9: exit 0 against brief-06-installation-token-scoping.md. It keeps gate: human (line 13), and its rows 7 and 8 carry the GitLab and Windows trace markers.
- Row 10: exit 0, nothing printed; all 24 owner rows are present.
- Row 11: exit 0. The desk-containers board row 10 (cellctl in Go) itself reads implemented.
- Row 12: exit 0. consumers: no brief files in the diff against 05dc9fbc88f2 — nothing to corroborate. This is vacuous on merged main, the tool-side gap medici-finance/assay#1281 (still open). A supplemental run at the implementing commit 85aeeb8e1f8d, with --brief desktools-v2/13, --base set to its first parent 3a06803b579a and --delivery set to that commit, gave exit 0 with 8 corroborated, 0 disproved and 1 unchecked. The unchecked entry is the desktools-v2/11 out-of-scope claim, and its follow-up issue medici-finance/assay#1841 is open, so the claim holds. The supplemental run earns no row credit on its own.
- Row 13: exit 0, nothing printed. Each of the 24 issues has exactly one comment carrying the triage marker. Control: the same select on medici-finance/assay#1836 returns 0 ids, so the filter discriminates.

Findings (advisory; none is a Verify-row failure):

- **Class f baseline drift.** The live class f desk count is 56 against the recorded gitlab-literal=45 (47 at the 2026-10-16 pass). An independent git grep recount, a different tool from the counter, gives 45 at the implementing commit 85aeeb8e1f8d and 56 at this SHA. Classes e and f feed no total and no gate; the drift is corrected with --baseline.
- **Class a to d drift.** The forge reach-around total reads 74 against the recorded desktools-v2/02 baseline of 63 (72 at the 2026-10-16 pass). Classes e and f stay excluded from that total; row 3 proves the exclusion.
- **Probe self-count.** One class f desk line is still the probe script's own planted literal (forge-ban-probe.sh line 14), and the counter header declares no carve-out for it.
- **Row 12 vacuity.** Row 12 is vacuous on merged main (medici-finance/assay#1281). Its Expect holds by the supplemental run at the implementing commit.

Risk-bearing value. Risk metadata is present: regulatory, customer, irreversible and sensitive-data all read no, and the gate is model. The delivered work touches .github/workflows, so the enumeration was done anyway, over the counter script, the baseline file, the statusgen portability detector, the mutation script and both workflow files at this SHA:

- desktools-v2/13 glab=0 gitlab-literal=45 @ forge-ban-baseline.txt:2. A recorded baseline: if it is wrong in the low direction, it hides growth. Reversible with --baseline.
- desktools-v2/02 63 @ forge-ban-baseline.txt:1. Not this brief's value, and untouched by it. Reversible.
- GLAB_SUBCMDS = issue, mr, api, auth, repo, release, ci, label, schedule, snippet, variable @ forge-ban.sh:181. Reversible.
- class f pattern = /api/v4 or gitlab.com @ forge-ban.sh:187 and :195. Reversible.
- BACKEND_EXCLUDE = forge_github, forge_gitlab @ forge-ban.sh:75. Reversible.
- detector regexes verifyOSMarker, verifyShellC, verifyFindstr and verifyTmpPath @ statusgen verifyportability.go:11 to 14. NOTICE-level only. Reversible.
- cross-compile targets GOOS=windows GOARCH=amd64 and GOOS=darwin GOARCH=arm64 @ ci.yml:155 to 157. Reversible.
- runs-on windows-latest @ windows-ci-leg.yml:75, and -timeout 45s @ portability-mutation.sh:17. Operational.

None of these is irreversible. Top-ranked entry:

RISK-VALUE: DERIVED — desktools-v2/13 glab=0 gitlab-literal=45 @ docs/streams/desktools-v2/forge-ban-baseline.txt:2 — an independent git grep recount (not the counter) applied the brief's class e and f definitions: lines matching /api/v4 or gitlab.com in .go and .sh files under tools/desk, tools/cellctl and plugins/assay, excluding the two forge backends, _test.go, .test.sh and the counter itself. It gives gitlab-literal 45 at the implementing commit 85aeeb8e1f8d, the commit that last wrote the baseline line, and 0 non-test exec.Command("glab" sites. So the recorded value is right for the tree it baselined. The same recount gives 56 at 05dc9fbc88f2, matching the counter's live desk count. Advisory value; it feeds no gate.

**VERIFY: PASS** — 13 of 13 rows pass at merged main 05dc9fbc88f2 by the execution witness and by hand reading. Row 12 is vacuous on merged main (medici-finance/assay#1281, open); its Expect is upheld by the supplemental run at the implementing commit (0 disproved). Gate model, every risk answer no.
### Non-implementer verifier run — 2026-10-27 claude-opus-5-5 verifier at bd47e88aa9e6 (rows 1 to 12 witnessed and passing; row 13 blocked on its forge credential)

- Target: merged main @ bd47e88aa9e6983267c85c2a0ed3f3589366fba5, the admitted source. The forge main ref had moved on to 7fcd9d284220 by run time; this run verifies the admitted source. The dispatcher's check-verifier receipt admitted the run before any row and was re-checked before landing.
- Host and tools: darwin/arm64, go 1.27.2, installed statusgen v1.0.63, KUBECONFIG=/dev/null, GH_TOKEN and GITHUB_TOKEN unset, TMPDIR pointed at a scratch directory outside the home, real Go module and build caches.
- Execution witness: the table below was written by statusgen verifyrun in write mode with --rows 1,2,3,4,5,6,7,8,9,10,11,12, from the admitted home on a clean tree at the source SHA. Its stamps and its Verification-Attestation line are as the tool wrote them.
- Row 13 is outside this witness. Its command reads issue comments with the forge CLI, and statusgen v1.0.63 strips the forge token variables from an admitted row's environment, so the row can only authenticate through a forge-CLI config directory. Writing the verifier App's installation token into an isolated forge-CLI config for the row was refused by the runner's permission layer ("Permission for this action was denied ... Reason: [Credential Materialization]"). That refusal is a stop, and running the row through an ambient human CLI login is outside this run's identity rules. Row 13 therefore has no witness at this source; its latest witness is the 2026-10-22 run at 05dc9fbc88f2.
- Fail-first: none of the 13 rows is risk-bearing under statusgen's definition (no risk: yes in the brief, and no risk-bearing, live, mutating or end-to-end tag in any row's Command or Expect), so no fail-first witness is owed. The delivering change is the single-parent squash commit 85aeeb8e1f8d (#1996), first parent 3a06803b579a.
- Hand reading: every witnessed row was re-run by hand from the root of a no-hardlinks scratch clone detached at the source SHA, with its origin/main set to that SHA. The observations follow the table. The hand runs add no rows.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -q -F -e 'GOOS=windows' .github/workflows/ci.yml && grep -q -F -e 'go test -c' .github/workflows/ci.yml` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ bd47e88aa9e6 (on-behalf-of human:ian) (forge-identity) |
| 2 | `d=$(mktemp -d) && sh tools/desk/scripts/forge-ban.sh > "$d/r2.out"; echo rc=$?; grep -F -e 'class e (glab subprocess):' "$d/r2.out" && grep -F -e 'class f (GitLab API literal):' "$d/r2.out"` | pass exit=0 | sha256:da80c370e2d1 | 2026-10-27 | assay-verifier-app[bot] @ bd47e88aa9e6 (on-behalf-of human:ian) (forge-identity) |
| 3 | `sh tools/desk/scripts/forge-ban-probe.sh` | pass exit=0 | sha256:406d6e54ba92 | 2026-10-27 | assay-verifier-app[bot] @ bd47e88aa9e6 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -E -e '^desktools-v2/02 [0-9]+$' docs/streams/desktools-v2/forge-ban-baseline.txt && grep -E -e '^desktools-v2/13 glab=[0-9]+ gitlab-literal=[0-9]+$' docs/streams/desktools-v2/forge-ban-baseline.txt` | pass exit=0 | sha256:df26954dfed8 | 2026-10-27 | assay-verifier-app[bot] @ bd47e88aa9e6 (on-behalf-of human:ian) (forge-identity) |
| 5 | `d=$(mktemp -d) && cd statusgen && go test -run '^TestVerifyRowPortability$' -v . > "$d/r5.out" 2>&1 && grep -E -e '^--- PASS: TestVerifyRowPortability \(' "$d/r5.out" && sh testdata/portability-mutation.sh` | pass exit=0 | sha256:1bd7ed2ce960 | 2026-10-27 | assay-verifier-app[bot] @ bd47e88aa9e6 (on-behalf-of human:ian) (forge-identity) |
| 6 | `d=$(mktemp -d) && cd statusgen && go build -o "$d/statusgen" . && cd .. && "$d/statusgen" --root . --lint >/dev/null 2>&1; echo rc=$?` | pass exit=0 | sha256:93ff7811a209 | 2026-10-27 | assay-verifier-app[bot] @ bd47e88aa9e6 (on-behalf-of human:ian) (forge-identity) |
| 7 | `awk 'prev ~ /^[ ]+cd tools\/desk[ ]*$/ && $0 ~ /^[ ]+go test \.\/\.\.\.[ ]*$/ {f=1} {prev=$0} END {exit !f}' .github/workflows/windows-ci-leg.yml && grep -q -F -e 'windows-latest' .github/workflows/windows-ci-leg.yml` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ bd47e88aa9e6 (on-behalf-of human:ian) (forge-identity) |
| 8 | `grep -n -e 1145 -e 1146 -e 1223 docs/streams/desktools-v2/brief-06-*.md; test $? -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ bd47e88aa9e6 (on-behalf-of human:ian) (forge-identity) |
| 9 | `f=$(ls docs/streams/desktools-v2/brief-06-*.md); grep -q -F -e 'cmd/cellctl' "$f" && grep -q -E -e '^gate: human$' "$f" && grep -q -F -e '(desktools-v2/13 GitLab row)' "$f" && grep -q -F -e '(desktools-v2/13 Windows row)' "$f"` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ bd47e88aa9e6 (on-behalf-of human:ian) (forge-identity) |
| 10 | `miss=0; for n in 641 642 1604 1621 1418 1424 1805 1435 1644 678 1569 1693 1573 1203 655 676 677 1477 1411 1412 1415 865 1667 1794; do grep -q -E -e "^[\|] #$n [\|] desktools-v2/[0-9]+" -e "^[\|] #$n [\|] out-of-scope" docs/streams/desktools-v2/brief-13-platform-gates-and-reconciliation.md \|\| { echo "UNOWNED #$n"; miss=1; }; done; test $miss -eq 0` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ bd47e88aa9e6 (on-behalf-of human:ian) (forge-identity) |
| 11 | `grep -q -E -e '^[\|] 10 [\|].*[\|] implemented [\|]' -e '^[\|] 10 [\|].*[\|] verified [\|]' -e '^[\|] 10 [\|].*[\|] done [\|]' docs/streams/desk-containers/README.md` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ bd47e88aa9e6 (on-behalf-of human:ian) (forge-identity) |
| 12 | `statusgen --consumers --root .` | pass exit=0 | sha256:35704801ded0 | 2026-10-27 | assay-verifier-app[bot] @ bd47e88aa9e6 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4688 run=5424bab89f7a96444164def235915bd1b5d782285d67dde79ef4fe90f39176b0 source=bd47e88aa9e6983267c85c2a0ed3f3589366fba5 brief=docs/streams/desktools-v2/brief-13-platform-gates-and-reconciliation.md model=claude-opus-5-5 tier=strong

Hand reading at bd47e88aa9e6 (scratch clone, same SHA):

- Row 1: ci.yml lines 155 to 157 run windows/amd64 go vet, the per-package windows go test -c loop, and darwin/arm64 go vet. The workflow triggers on push to main and on pull_request.
- Row 2: rc=0 / class e (glab subprocess): desk=0 statusgen=0 / class f (GitLab API literal): desk=55 statusgen=8. Also printed: forge reach-around sites: 74 (desk: 36, statusgen: 38), statusgen sites: 38, forgeread sites: 0.
- Row 3: PROBE PASS, exit 0; git status --porcelain tools/desk printed nothing afterwards.
- Row 4: desktools-v2/02 63 / desktools-v2/13 glab=0 gitlab-literal=45.
- Row 5: go test exit 0, --- PASS: TestVerifyRowPortability (0.00s) with 12 PASS lines (the top level and 11 subtests). The mutation script disabled the detector, the run failed subtests including bash, findstr and tmpdir-plus-hardcode, and the script printed MUTATION PASS: disabled detector was rejected. git status --porcelain statusgen was empty afterwards.
- Row 6: the witnessed output hash sha256:93ff7811a209 is the hash of the single line rc=0. Re-run with output kept: rc=0, LINT: PASS, with 114 lines tagged verify-row-portability. That is the NOTICE baseline at this SHA (187 at the 2026-10-22 pass).
- Row 7: windows-ci-leg.yml lines 84 and 85 are the run block (cd tools/desk, then go test ./...) of a job on windows-latest (line 75). Triggers are the v-tag push, a nightly schedule (cron 41 3 * * *) and workflow_dispatch, with no pull_request trigger. So the Windows proof runs nightly and at release, not on a PR, which deliverable 4 allows.
- Row 8: exit 0, nothing printed.
- Row 9: exit 0 against brief-06-installation-token-scoping.md. It keeps gate: human (line 13), and its rows 7 and 8 carry the GitLab and Windows trace markers.
- Row 10: exit 0, nothing printed; all 24 owner rows are present.
- Row 11: exit 0. The desk-containers board row 10 (cellctl in Go) itself reads implemented.
- Row 12: exit 0. In the clone: consumers: no brief files in the diff against bd47e88aa9e6 — nothing to corroborate. This is vacuous on merged main, the tool-side gap medici-finance/assay#1281 (still open). A supplemental run at the delivering commit 85aeeb8e1f8d, with --brief desktools-v2/13, --base set to its first parent 3a06803b579a and --delivery set to that commit, gave exit 0 with 8 corroborated, 0 disproved and 1 unchecked. The unchecked entry is the desktools-v2/11 out-of-scope claim. The supplemental run earns no row credit.
- Row 13 (supplemental probe, no acceptance credit): the row's loop, run outside statusgen with the verifier App installation token scoped to each read command and the repository named explicitly, found the (desktools-v2/13 triage) marker on all 24 issues (24 comment ids, no READ FAILED, no NO TRIAGE COMMENT). Control: the same select on medici-finance/assay#1836 returns 0. This shows the obligation's state on the tracker today, but it is not a statusgen witness and discharges nothing.

Findings (advisory; none is a Verify-row failure):

- **Class f baseline drift.** The live class f desk count is 55 against the recorded gitlab-literal=45 (56 at the 2026-10-22 pass). An independent git grep recount, a different tool from the counter, gives 45 at the delivering commit 85aeeb8e1f8d and 55 at this SHA. Classes e and f feed no total and no gate; --baseline corrects the drift.
- **Class a to d drift.** The forge reach-around total reads 74 against the recorded desktools-v2/02 baseline of 63. Classes e and f stay excluded from that total; row 3 proves the exclusion.
- **Probe self-count.** One class f desk line is still the probe script's own planted literal (forge-ban-probe.sh line 14), and the counter header declares no carve-out for it.
- **Row 13 has no credential path under the admitted runner.** statusgen v1.0.63 removes the forge token variables from an admitted row, so a row that calls the forge CLI can authenticate only through a CLI config directory or an ambient login. Until a sanctioned credential path exists for such rows, row 13 cannot be witnessed by an admitted run.

Risk-bearing value. Risk metadata is present: regulatory, customer, irreversible and sensitive-data all read no, and the gate is model. The delivered work touches .github/workflows, so the enumeration was done anyway, over the counter script, the probe, the baseline file, the statusgen portability detector, the mutation script and both workflow files at this SHA:

- desktools-v2/13 glab=0 gitlab-literal=45 @ forge-ban-baseline.txt:2. A recorded baseline: if it is wrong in the low direction, it hides growth. Reversible with --baseline.
- desktools-v2/02 63 @ forge-ban-baseline.txt:1. Not this brief's value, and untouched by it. Reversible.
- GLAB_SUBCMDS = issue, mr, api, auth, repo, release, ci, label, schedule, snippet, variable @ forge-ban.sh:181. Reversible.
- class f pattern = /api/v4 or gitlab.com @ forge-ban.sh:187 and :195. Reversible.
- BACKEND_EXCLUDE = forge_github, forge_gitlab @ forge-ban.sh:75. Reversible.
- probe plants exec.Command("glab", "api") and "/api/v4" @ forge-ban-probe.sh:13 and :14. Reversible.
- detector regexes verifyOSMarker, verifyShellC, verifyFindstr and verifyTmpPath @ statusgen verifyportability.go:11 to 14. NOTICE-level only. Reversible.
- cross-compile targets GOOS=windows GOARCH=amd64 and GOOS=darwin GOARCH=arm64 @ ci.yml:155 to 157. Reversible.
- runs-on windows-latest @ windows-ci-leg.yml:75, and -timeout 45s @ portability-mutation.sh:17. Operational.

None of these is irreversible. Top-ranked entry:

RISK-VALUE: DERIVED — desktools-v2/13 glab=0 gitlab-literal=45 @ docs/streams/desktools-v2/forge-ban-baseline.txt:2 — an independent git grep recount (not the counter) applied the brief's class e and f definitions: lines matching /api/v4 or gitlab.com in .go and .sh files under tools/desk, tools/cellctl and plugins/assay, excluding the two forge backends, _test.go, .test.sh and the counter itself. It gives gitlab-literal 45 at the delivering commit 85aeeb8e1f8d, the commit that last wrote the baseline line, and 0 non-test exec.Command("glab" sites. So the recorded value is right for the tree it baselined. The same recount gives 55 at bd47e88aa9e6, matching the counter's live desk count. Advisory value; it feeds no gate.

**VERIFY: BLOCKED** — rows 1 to 12 pass at merged main bd47e88aa9e6 by the execution witness and by hand reading; row 13 has no execution witness at this source because the admitted runner has no sanctioned forge credential path for it (the permission refusal is quoted above). No obligation failed. Row 12 is vacuous on merged main (medici-finance/assay#1281, open); its Expect is upheld by the supplemental run at the delivering commit (0 disproved). Gate model, every risk answer no.
### Non-implementer verifier run — 2026-10-29 claude-opus-5-5 verifier at 1304d40f5c32 (11 of 13 rows pass; row 6 and row 13 blocked)

- Target: merged main @ 1304d40f5c32cae5136e87f6b1f0cbe5a008c72a, the admitted source; the forge main ref matched it at run time. The dispatcher's check-verifier receipt admitted the run before any row and was re-checked after this Evidence edit.
- Host and tools: darwin/arm64, go 1.27.2, git 2.56.0, installed statusgen v1.0.65, KUBECONFIG=/dev/null, GH_TOKEN and GITHUB_TOKEN unset, TMPDIR pointed at a scratch directory outside the home.
- Execution witness: the table below was written by statusgen verifyrun in write mode with --rows 1,2,3,4,5,6,7,8,9,10,11,12, from the admitted home on a clean tree at the source SHA. Its stamps and its Verification-Attestation line are as the tool wrote them. Every row is plain check class, so no row needed the network-off sandbox.
- Row 13 is outside the witness for the reason recorded on medici-finance/assay#4700 (open): an admitted verifyrun row has no sanctioned forge credential path, and this run sets no forge token in the verifyrun environment.
- Fail-first: no row is risk-bearing under statusgen's definition (no risk: yes in the brief, no risk-bearing, live, mutating or end-to-end tag in any row), so no fail-first witness is owed. The delivering change is the single-parent squash commit 85aeeb8e1f8d (#1996), first parent 3a06803b579a.
- Hand reading: every witnessed row was re-run by hand from the root of a no-hardlinks scratch clone detached at the source SHA, with its origin/main set to that SHA. The observations follow the table. The hand runs add no rows.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -q -F -e 'GOOS=windows' .github/workflows/ci.yml && grep -q -F -e 'go test -c' .github/workflows/ci.yml` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 2 | `d=$(mktemp -d) && sh tools/desk/scripts/forge-ban.sh > "$d/r2.out"; echo rc=$?; grep -F -e 'class e (glab subprocess):' "$d/r2.out" && grep -F -e 'class f (GitLab API literal):' "$d/r2.out"` | pass exit=0 | sha256:55fbc15862fa | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 3 | `sh tools/desk/scripts/forge-ban-probe.sh` | pass exit=0 | sha256:406d6e54ba92 | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -E -e '^desktools-v2/02 [0-9]+$' docs/streams/desktools-v2/forge-ban-baseline.txt && grep -E -e '^desktools-v2/13 glab=[0-9]+ gitlab-literal=[0-9]+$' docs/streams/desktools-v2/forge-ban-baseline.txt` | pass exit=0 | sha256:df26954dfed8 | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 5 | `d=$(mktemp -d) && cd statusgen && go test -run '^TestVerifyRowPortability$' -v . > "$d/r5.out" 2>&1 && grep -E -e '^--- PASS: TestVerifyRowPortability \(' "$d/r5.out" && sh testdata/portability-mutation.sh` | pass exit=0 | sha256:7e61df5e3840 | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 6 | `d=$(mktemp -d) && cd statusgen && go build -o "$d/statusgen" . && cd .. && "$d/statusgen" --root . --lint >/dev/null 2>&1; echo rc=$?` | pass exit=0 | sha256:91d957f8f274 | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 7 | `awk 'prev ~ /^[ ]+cd tools\/desk[ ]*$/ && $0 ~ /^[ ]+go test \.\/\.\.\.[ ]*$/ {f=1} {prev=$0} END {exit !f}' .github/workflows/windows-ci-leg.yml && grep -q -F -e 'windows-latest' .github/workflows/windows-ci-leg.yml` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 8 | `grep -n -e 1145 -e 1146 -e 1223 docs/streams/desktools-v2/brief-06-*.md; test $? -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 9 | `f=$(ls docs/streams/desktools-v2/brief-06-*.md); grep -q -F -e 'cmd/cellctl' "$f" && grep -q -E -e '^gate: human$' "$f" && grep -q -F -e '(desktools-v2/13 GitLab row)' "$f" && grep -q -F -e '(desktools-v2/13 Windows row)' "$f"` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 10 | `miss=0; for n in 641 642 1604 1621 1418 1424 1805 1435 1644 678 1569 1693 1573 1203 655 676 677 1477 1411 1412 1415 865 1667 1794; do grep -q -E -e "^[\|] #$n [\|] desktools-v2/[0-9]+" -e "^[\|] #$n [\|] out-of-scope" docs/streams/desktools-v2/brief-13-platform-gates-and-reconciliation.md \|\| { echo "UNOWNED #$n"; miss=1; }; done; test $miss -eq 0` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 11 | `grep -q -E -e '^[\|] 10 [\|].*[\|] implemented [\|]' -e '^[\|] 10 [\|].*[\|] verified [\|]' -e '^[\|] 10 [\|].*[\|] done [\|]' docs/streams/desk-containers/README.md` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 12 | `statusgen --consumers --root .` | pass exit=0 | sha256:ec256dd0157f | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4909 run=0a651737abc54fbaa2258435a3f8ebee062687474840e651d921114720f544a5 source=1304d40f5c32cae5136e87f6b1f0cbe5a008c72a brief=docs/streams/desktools-v2/brief-13-platform-gates-and-reconciliation.md model=claude-opus-5-5 tier=strong

Verifier disposition of the two rows this run cannot credit (the witness above stays as the tool wrote it; both observed 2026-10-29 by the assay-verifier-app[bot] verifier at 1304d40f5c32):

- Row 6 — could-not-run. Its command exits 0 by its trailing echo, but the witnessed output hash sha256:91d957f8f274 is the hash of the single line rc=1, so this run does not meet the Expect rc=0. The lint's one PROBLEM is a could-not-check on a neighbour brief, not on this brief or its portability lint: desktools-v2/65 protected table acceptance transition, could-not-check human authority, --acceptance-forge is required (medici-finance/assay#4823, open). With --acceptance-forge the same lint gives LINT: PASS, rc 0.
- Row 13 — could-not-run. An admitted verifyrun row has no sanctioned forge credential path (medici-finance/assay#4700, open), and this run sets no forge token in the verifyrun environment, so the row has no execution witness at this source.

Hand reading at 1304d40f5c32 (scratch clone, same SHA):

- Row 1: ci.yml lines 155 to 157 run windows/amd64 go vet, the per-package windows go test -c loop, and darwin/arm64 go vet. The workflow triggers on push to main and on pull_request (lines 18 to 20).
- Row 2: rc=0 / class e (glab subprocess): desk=0 statusgen=0 / class f (GitLab API literal): desk=56 statusgen=8. Also printed: forge reach-around sites: 74 (desk: 36, statusgen: 38), statusgen sites: 38, forgeread sites: 0.
- Row 3: PROBE PASS, exit 0; git status --porcelain tools/desk printed nothing afterwards.
- Row 4: desktools-v2/02 63 / desktools-v2/13 glab=0 gitlab-literal=45.
- Row 5: go test exit 0, --- PASS: TestVerifyRowPortability (0.00s) with 12 PASS lines (the top level and 11 subtests). The mutation script disabled the detector; the run failed the top level plus notice-flow, tmp, sh, bash, findstr and tmpdir-plus-hardcode, and the script printed MUTATION PASS: disabled detector was rejected. git status --porcelain statusgen was empty afterwards.
- Row 6: the witnessed output hash sha256:91d957f8f274 equals the hash of the line rc=1 (rc=0 hashes to sha256:93ff7811a209, the value prior passes recorded). Re-run with output kept, in the clone with its origin set to the forge URL: rc=1, LINT: FAIL 1 problem(s), the sole PROBLEM the desktools-v2/65 acceptance-transition could-not-check quoted in the disposition table, the same with the installed pinned statusgen. 113 lines are tagged verify-row-portability and every one is a NOTICE, none a PROBLEM: that is the NOTICE baseline at this SHA (114 at the 2026-10-27 pass). Supplemental, no row credit: the locally built binary and the pinned one each give LINT: PASS, rc 0, with --acceptance-forge.
- Row 7: windows-ci-leg.yml lines 84 and 85 are the run block (cd tools/desk, then go test ./...) of a job on windows-latest (line 75). Triggers are the v-tag push, a nightly schedule (cron 41 3 * * *) and workflow_dispatch, with no pull_request trigger, so the Windows proof runs nightly and at release, which deliverable 4 allows.
- Row 8: exit 0, nothing printed.
- Row 9: exit 0 against brief-06-installation-token-scoping.md. It keeps gate: human (line 13), names cmd/cellctl, and carries the GitLab and Windows trace markers (lines 126 and 127).
- Row 10: exit 0, nothing printed; all 24 owner rows are present.
- Row 11: exit 0. The desk-containers board row 10 (cellctl in Go) itself reads implemented.
- Row 12: exit 0. consumers: no brief files in the diff against 1304d40f5c32 — nothing to corroborate. This is vacuous on merged main, the tool-side gap medici-finance/assay#1281 (open). A supplemental run at the delivering commit 85aeeb8e1f8d, with --brief desktools-v2/13, --base its first parent 3a06803b579a and --delivery that commit (full SHAs), gave exit 0 with 8 corroborated, 0 disproved and 1 unchecked; the unchecked entry is the desktools-v2/11 out-of-scope claim. The supplemental run earns no row credit.
- Row 13 (supplemental probe, no acceptance credit): the row's loop, run outside statusgen with the verifier App installation token scoped to each read command and the repository named explicitly, found the (desktools-v2/13 triage) marker on all 24 issues (24 comment ids, no READ FAILED, no NO TRIAGE COMMENT). Control: the same select on medici-finance/assay#1836 returns 0, so the filter discriminates. This shows the obligation's state on the tracker today but is not a statusgen witness and discharges nothing.

Findings (advisory; none is a failure of this brief's delivered work):

- **Row 6 lint red from a neighbour.** The offline lint is red on main from the desktools-v2/65 acceptance transition alone (medici-finance/assay#4823). The portability lint this brief added stays NOTICE-only. Row 6 can pass again once that transition is resolved or offline lint reports the could-not-check as could-not-check.
- **Class f baseline drift.** The live class f desk count is 56 against the recorded gitlab-literal=45 (55 at the 2026-10-27 pass). An independent git grep recount gives 45 at the delivering commit 85aeeb8e1f8d and 56 at this SHA. Classes e and f feed no total and no gate; --baseline corrects the drift.
- **Class a to d drift.** The forge reach-around total reads 74 against the recorded desktools-v2/02 baseline of 63. Classes e and f stay excluded from that total; row 3 proves the exclusion.
- **Probe self-count.** One class f desk line is still the probe script's own planted literal (forge-ban-probe.sh line 14), and the counter header declares no carve-out for it.

Risk-bearing value. Risk metadata is present: regulatory, customer, irreversible and sensitive-data all read no, and the gate is model. The delivered work touches .github/workflows, so the enumeration was done anyway, over the counter script, the probe, the baseline file, the statusgen portability detector, the mutation script and both workflow files at this SHA:

- desktools-v2/13 glab=0 gitlab-literal=45 @ forge-ban-baseline.txt:2. A recorded baseline: if it is wrong in the low direction, it hides growth. Reversible with --baseline.
- desktools-v2/02 63 @ forge-ban-baseline.txt:1. Not this brief's value, and untouched by it. Reversible.
- GLAB_SUBCMDS = issue, mr, api, auth, repo, release, ci, label, schedule, snippet, variable @ forge-ban.sh:181. Reversible.
- class f pattern = /api/v4 or gitlab.com @ forge-ban.sh:187 and :195. Reversible.
- BACKEND_EXCLUDE = forge_github, forge_gitlab @ forge-ban.sh:75. Reversible.
- probe plants exec.Command("glab", "api") and "/api/v4" @ forge-ban-probe.sh:13 and :14. Reversible.
- detector regexes verifyOSMarker, verifyShellC, verifyFindstr and verifyTmpPath @ statusgen verifyportability.go:11 to 14. NOTICE-level only. Reversible.
- cross-compile targets GOOS=windows GOARCH=amd64 and GOOS=darwin GOARCH=arm64 @ ci.yml:155 to 157. Reversible.
- runs-on windows-latest @ windows-ci-leg.yml:75, and -timeout 45s @ portability-mutation.sh:17. Operational.

None of these is irreversible. Top-ranked entry:

RISK-VALUE: DERIVED — desktools-v2/13 glab=0 gitlab-literal=45 @ docs/streams/desktools-v2/forge-ban-baseline.txt:2 — an independent git grep recount (not the counter) applied the brief's class e and f definitions: lines matching /api/v4 or gitlab.com in .go and .sh files under tools/desk, tools/cellctl and plugins/assay, excluding the two forge backends, _test.go, .test.sh and the counter itself. It gives gitlab-literal 45 at the delivering commit 85aeeb8e1f8d, the commit that last wrote the baseline line, and 0 non-test exec.Command("glab" sites. So the recorded value is right for the tree it baselined. The same recount gives 56 at 1304d40f5c32, matching the counter's live desk count. Advisory value; it feeds no gate.

**VERIFY: BLOCKED** — rows 1 to 5 and 7 to 12 pass at merged main 1304d40f5c32 by the execution witness and by hand reading. Row 6 is could-not-run: its lint is red only on a neighbour brief's could-not-check human authority (medici-finance/assay#4823, open) and passes with --acceptance-forge. Row 13 is could-not-run: an admitted row has no sanctioned forge credential path (medici-finance/assay#4700, open); its supplemental probe found all 24 triage comments. No obligation of this brief failed. Row 12 is vacuous on merged main (medici-finance/assay#1281, open); its Expect is upheld by the supplemental run at the delivering commit (0 disproved). Gate model, every risk answer no.
### Non-implementer verifier run — 2026-10-30 claude-opus-5-5 verifier at 40925e8233c8 (rows 1 to 5 and 7 to 12 pass; rows 6 and 13 could-not-run)

- Target: merged main @ 40925e8233c8a63d70682e77ca333e665f1f955e, the admitted source. The dispatcher's check-verifier receipt (medici-finance/assay#4958) admitted the run before any row and was re-checked after this Evidence edit.
- Host and tools: darwin/arm64, go 1.27.2, installed statusgen v1.0.66, KUBECONFIG=/dev/null, GH_TOKEN and GITHUB_TOKEN unset, TMPDIR pointed at a scratch directory outside the home.
- Execution witness: the table below was written by statusgen verifyrun in write mode with --rows 1,2,3,4,5,6,7,8,9,10,11,12, from the admitted home on a clean tree at the source SHA. Its stamps and its Verification-Attestation line are as the tool wrote them.
- Fail-first: none of the 13 rows is risk-bearing under statusgen's definition (no risk: yes in the brief, and no risk-bearing, live, mutating or end-to-end tag in any row's Command or Expect), so no fail-first witness is owed. The delivering change is the single-parent squash commit 85aeeb8e1f8d (#1996), first parent 3a06803b579a.
- Hand reading: every witnessed row was re-run by hand from the root of a no-hardlinks scratch clone detached at the source SHA, with its origin/main set to that SHA. The observations follow the table. The hand runs add no rows.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -q -F -e 'GOOS=windows' .github/workflows/ci.yml && grep -q -F -e 'go test -c' .github/workflows/ci.yml` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 2 | `d=$(mktemp -d) && sh tools/desk/scripts/forge-ban.sh > "$d/r2.out"; echo rc=$?; grep -F -e 'class e (glab subprocess):' "$d/r2.out" && grep -F -e 'class f (GitLab API literal):' "$d/r2.out"` | pass exit=0 | sha256:55fbc15862fa | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 3 | `sh tools/desk/scripts/forge-ban-probe.sh` | pass exit=0 | sha256:406d6e54ba92 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -E -e '^desktools-v2/02 [0-9]+$' docs/streams/desktools-v2/forge-ban-baseline.txt && grep -E -e '^desktools-v2/13 glab=[0-9]+ gitlab-literal=[0-9]+$' docs/streams/desktools-v2/forge-ban-baseline.txt` | pass exit=0 | sha256:df26954dfed8 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 5 | `d=$(mktemp -d) && cd statusgen && go test -run '^TestVerifyRowPortability$' -v . > "$d/r5.out" 2>&1 && grep -E -e '^--- PASS: TestVerifyRowPortability \(' "$d/r5.out" && sh testdata/portability-mutation.sh` | pass exit=0 | sha256:1262896a09c9 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 6 | `d=$(mktemp -d) && cd statusgen && go build -o "$d/statusgen" . && cd .. && "$d/statusgen" --root . --lint >/dev/null 2>&1; echo rc=$?` | pass exit=0 | sha256:91d957f8f274 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 7 | `awk 'prev ~ /^[ ]+cd tools\/desk[ ]*$/ && $0 ~ /^[ ]+go test \.\/\.\.\.[ ]*$/ {f=1} {prev=$0} END {exit !f}' .github/workflows/windows-ci-leg.yml && grep -q -F -e 'windows-latest' .github/workflows/windows-ci-leg.yml` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 8 | `grep -n -e 1145 -e 1146 -e 1223 docs/streams/desktools-v2/brief-06-*.md; test $? -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 9 | `f=$(ls docs/streams/desktools-v2/brief-06-*.md); grep -q -F -e 'cmd/cellctl' "$f" && grep -q -E -e '^gate: human$' "$f" && grep -q -F -e '(desktools-v2/13 GitLab row)' "$f" && grep -q -F -e '(desktools-v2/13 Windows row)' "$f"` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 10 | `miss=0; for n in 641 642 1604 1621 1418 1424 1805 1435 1644 678 1569 1693 1573 1203 655 676 677 1477 1411 1412 1415 865 1667 1794; do grep -q -E -e "^[\|] #$n [\|] desktools-v2/[0-9]+" -e "^[\|] #$n [\|] out-of-scope" docs/streams/desktools-v2/brief-13-platform-gates-and-reconciliation.md \|\| { echo "UNOWNED #$n"; miss=1; }; done; test $miss -eq 0` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 11 | `grep -q -E -e '^[\|] 10 [\|].*[\|] implemented [\|]' -e '^[\|] 10 [\|].*[\|] verified [\|]' -e '^[\|] 10 [\|].*[\|] done [\|]' docs/streams/desk-containers/README.md` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 12 | `statusgen --consumers --root .` | pass exit=0 | sha256:ca38fbc4dc1b | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4958 run=c85b10806ae1134bf9de56b53266352370e4e512ec44f2868ce79a4e3340bd16 source=40925e8233c8a63d70682e77ca333e665f1f955e brief=docs/streams/desktools-v2/brief-13-platform-gates-and-reconciliation.md model=claude-opus-5-5 tier=strong

Hand reading at 40925e8233c8 (scratch clone, same SHA):

- Row 1: ci.yml lines 155 to 157 run windows/amd64 go vet, the per-package windows go test -c loop, and darwin/arm64 go vet. The workflow triggers on push to main and on pull_request.
- Row 2: rc=0 / class e (glab subprocess): desk=0 statusgen=0 / class f (GitLab API literal): desk=56 statusgen=8. Also printed: forge reach-around sites: 74 (desk: 36, statusgen: 38), statusgen sites: 38, forgeread sites: 0.
- Row 3: PROBE PASS, exit 0; git status --porcelain tools/desk printed nothing afterwards.
- Row 4: desktools-v2/02 63 / desktools-v2/13 glab=0 gitlab-literal=45.
- Row 5: go test exit 0, --- PASS: TestVerifyRowPortability (0.00s), 12 PASS lines (the top level and 11 subtests). The mutation script disabled the detector, the run failed subtests tmp, sh, bash, findstr, tmpdir-plus-hardcode and notice-flow, and the script printed MUTATION PASS: disabled detector was rejected. git status --porcelain statusgen was empty afterwards.
- Row 6 — could-not-run. The witnessed exit 0 comes from the row's trailing echo; the witnessed output hash sha256:91d957f8f274 is the hash of the single line rc=1, so this run does not meet the Expect rc=0. Re-run with output kept (origin set to the forge URL, as in the home): rc=1, LINT: FAIL 1 problem(s). The one PROBLEM is a could-not-check on a neighbour brief, not on this brief or its portability lint: desktools-v2/65: protected table: acceptance transition: could-not-check human authority: --acceptance-forge is required (medici-finance/assay#4823, open). Supplemental probe, no acceptance credit: the same lint with --acceptance-forge and the verifier App installation token gives LINT: PASS, rc 0. 113 lines are tagged verify-row-portability in both runs; that is the NOTICE baseline at this SHA (114 at the 2026-10-27 pass).
- Row 7: windows-ci-leg.yml lines 84 and 85 are the run block (cd tools/desk, then go test ./...) of a job on windows-latest (line 75). Triggers are the v-tag push, a nightly schedule (cron 41 3 * * *) and workflow_dispatch, with no pull_request trigger. So the Windows proof runs nightly and at release, not on a PR, which deliverable 4 allows.
- Row 8: grep exit 1, nothing printed, so the row's test exits 0.
- Row 9: exit 0 against brief-06-installation-token-scoping.md. It keeps gate: human (line 13), and its rows 7 and 8 carry the GitLab and Windows trace markers.
- Row 10: exit 0, nothing printed; all 24 owner rows are present.
- Row 11: exit 0. The desk-containers board row 10 (cellctl in Go) itself reads implemented.
- Row 12: exit 0. In the clone: consumers: no brief files in the diff against 40925e8233c8 — nothing to corroborate. This is vacuous on merged main, the tool-side gap medici-finance/assay#1281 (still open). A supplemental run at the delivering commit 85aeeb8e1f8d, with --brief desktools-v2/13, --base set to its first parent 3a06803b579a and --delivery set to that commit, gave exit 0 with 8 corroborated, 0 disproved and 1 unchecked. The unchecked entry is the desktools-v2/11 out-of-scope claim. The supplemental run earns no row credit.
- **Row 13** (2026-10-30): `miss=0; for n in 641 642 1604 1621 1418 1424 1805 1435 1644 678 1569 1693 1573 1203 655 676 677 1477 1411 1412 1415 865 1667 1794; do c=$(gh api --paginate "repos/{owner}/{repo}/issues/$n/comments" --jq '.[] | select(.body | contains("(desktools-v2/13 triage)")) | .id') || { echo "READ FAILED #$n"; miss=1; continue; }; test -n "$c" || { echo "NO TRIAGE COMMENT #$n"; miss=1; }; done; test $miss -eq 0` — expected exit 0 and nothing printed; observed could-not-run: an admitted verifyrun row has no sanctioned forge credential path (medici-finance/assay#4700, open), and this run sets no forge token in the verifyrun environment, so the row has no execution witness at this source. Supplemental probe, no acceptance credit: the row's loop, run outside statusgen with the verifier App installation token scoped to each read and the repository named explicitly, found the (desktools-v2/13 triage) marker on all 24 issues (24 comment ids, no READ FAILED, no NO TRIAGE COMMENT). Control: the same select on medici-finance/assay#1836 returns 0.

Findings (advisory; none is a Verify-row failure):

- **Class f baseline drift.** The live class f desk count is 56 against the recorded gitlab-literal=45. An independent git grep recount, a different tool from the counter, gives 45 at the delivering commit 85aeeb8e1f8d and 56 at this SHA. Classes e and f feed no total and no gate; --baseline corrects the drift.
- **Class a to d drift.** The forge reach-around total reads 74 against the recorded desktools-v2/02 baseline of 63. Classes e and f stay excluded from that total; row 3 proves the exclusion.
- **Row 6 depends on the whole tree linting clean.** Its Expect is now held hostage by a neighbour brief's could-not-check (medici-finance/assay#4823); nothing in this brief's delivered lint contributes a PROBLEM.

Risk-bearing value. Risk metadata is present: regulatory, customer, irreversible and sensitive-data all read no, and the gate is model. The delivered work touches .github/workflows, so the enumeration was done anyway, over the counter script, the probe, the baseline file, the statusgen portability detector, the mutation script and both workflow files at this SHA (the only change to them since the 2026-10-27 pass is a notice-format refactor in verifyportability.go that names the rule through a constant):

- desktools-v2/13 glab=0 gitlab-literal=45 @ forge-ban-baseline.txt:2. A recorded baseline: if it is wrong in the low direction, it hides growth. Reversible with --baseline.
- desktools-v2/02 63 @ forge-ban-baseline.txt:1. Not this brief's value, and untouched by it. Reversible.
- GLAB_SUBCMDS = issue, mr, api, auth, repo, release, ci, label, schedule, snippet, variable @ forge-ban.sh:181. Reversible.
- class f pattern = /api/v4 or gitlab.com @ forge-ban.sh:187 and :195. Reversible.
- BACKEND_EXCLUDE = forge_github, forge_gitlab @ forge-ban.sh:75. Reversible.
- probe plants exec.Command("glab", "api") and "/api/v4" @ forge-ban-probe.sh:13 and :14. Reversible.
- detector regexes verifyOSMarker, verifyShellC, verifyFindstr and verifyTmpPath @ statusgen verifyportability.go:11 to 14. NOTICE-level only. Reversible.
- cross-compile targets GOOS=windows GOARCH=amd64 and GOOS=darwin GOARCH=arm64 @ ci.yml:155 to 157. Reversible.
- runs-on windows-latest @ windows-ci-leg.yml:75, and -timeout 45s @ portability-mutation.sh:17. Operational.

None of these is irreversible. Top-ranked entry:

RISK-VALUE: DERIVED — desktools-v2/13 glab=0 gitlab-literal=45 @ docs/streams/desktools-v2/forge-ban-baseline.txt:2 — an independent git grep recount (not the counter) applied the brief's class e and f definitions: lines matching /api/v4 or gitlab.com in .go and .sh files under tools/desk, tools/cellctl and plugins/assay, excluding the two forge backends, _test.go, .test.sh and the counter itself. It gives gitlab-literal 45 at the delivering commit 85aeeb8e1f8d, the commit that last wrote the baseline line, and 0 non-test exec.Command("glab" sites. So the recorded value is right for the tree it baselined. The same recount gives 56 at 40925e8233c8, matching the counter's live desk count. Advisory value; it feeds no gate.

**VERIFY: BLOCKED** — rows 1 to 5 and 7 to 12 pass at merged main 40925e8233c8 by the execution witness and by hand reading. Row 6 could-not-run: the whole-tree lint is red only on a neighbour brief's could-not-check (medici-finance/assay#4823, open), and passes with --acceptance-forge. Row 13 could-not-run: an admitted row has no forge credential path (medici-finance/assay#4700, open). No obligation failed. Row 12 is vacuous on merged main (medici-finance/assay#1281, open); its Expect is upheld by the supplemental run at the delivering commit (0 disproved). Gate model, every risk answer no.

### Non-implementer verifier run — 2026-10-31 claude-opus-5-5 verifier at 2a3b4be18948 (rows 1 to 5 and 7 to 12 pass; rows 6 and 13 could-not-run)

- Target: merged main @ 2a3b4be189484335a8592a80d619011d350a74c7, the admitted source. The dispatcher's check-verifier receipt (medici-finance/assay#5102) admitted the run before any row and was re-checked after this Evidence edit.
- Host and tools: darwin/arm64, installed statusgen v1.0.67, KUBECONFIG=/dev/null, no forge token set in the verifyrun environment, TMPDIR pointed at a scratch directory outside the home.
- Execution witness: the table below was written by statusgen verifyrun in write mode with --rows 1,2,3,4,5,6,7,8,9,10,11,12, from the admitted home on a clean tree at the source SHA. Its stamps and its Verification-Attestation line are as the tool wrote them. Its pass on row 6 does not meet that row's Expect; see the table after it.
- Fail-first: the brief's risk fields all read no and no row carries a risk-bearing, live, mutating or end-to-end tag, so no fail-first witness is owed. The delivering change is the single-parent squash commit 85aeeb8e1f8d (#1996), first parent 3a06803b579a.
- Hand reading: every witnessed row was re-run by hand from the root of a no-hardlinks scratch clone detached at the source SHA, with its origin set to the forge URL and origin/main at that SHA. The observations follow the table. The hand runs add no rows.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -q -F -e 'GOOS=windows' .github/workflows/ci.yml && grep -q -F -e 'go test -c' .github/workflows/ci.yml` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-31 | assay-verifier-app[bot] @ 2a3b4be18948 (on-behalf-of human:ian) (forge-identity) |
| 2 | `d=$(mktemp -d) && sh tools/desk/scripts/forge-ban.sh > "$d/r2.out"; echo rc=$?; grep -F -e 'class e (glab subprocess):' "$d/r2.out" && grep -F -e 'class f (GitLab API literal):' "$d/r2.out"` | pass exit=0 | sha256:fb9cee1c1f50 | 2026-10-31 | assay-verifier-app[bot] @ 2a3b4be18948 (on-behalf-of human:ian) (forge-identity) |
| 3 | `sh tools/desk/scripts/forge-ban-probe.sh` | pass exit=0 | sha256:406d6e54ba92 | 2026-10-31 | assay-verifier-app[bot] @ 2a3b4be18948 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -E -e '^desktools-v2/02 [0-9]+$' docs/streams/desktools-v2/forge-ban-baseline.txt && grep -E -e '^desktools-v2/13 glab=[0-9]+ gitlab-literal=[0-9]+$' docs/streams/desktools-v2/forge-ban-baseline.txt` | pass exit=0 | sha256:df26954dfed8 | 2026-10-31 | assay-verifier-app[bot] @ 2a3b4be18948 (on-behalf-of human:ian) (forge-identity) |
| 5 | `d=$(mktemp -d) && cd statusgen && go test -run '^TestVerifyRowPortability$' -v . > "$d/r5.out" 2>&1 && grep -E -e '^--- PASS: TestVerifyRowPortability \(' "$d/r5.out" && sh testdata/portability-mutation.sh` | pass exit=0 | sha256:677bf55dea8e | 2026-10-31 | assay-verifier-app[bot] @ 2a3b4be18948 (on-behalf-of human:ian) (forge-identity) |
| 6 | `d=$(mktemp -d) && cd statusgen && go build -o "$d/statusgen" . && cd .. && "$d/statusgen" --root . --lint >/dev/null 2>&1; echo rc=$?` | pass exit=0 | sha256:91d957f8f274 | 2026-10-31 | assay-verifier-app[bot] @ 2a3b4be18948 (on-behalf-of human:ian) (forge-identity) |
| 7 | `awk 'prev ~ /^[ ]+cd tools\/desk[ ]*$/ && $0 ~ /^[ ]+go test \.\/\.\.\.[ ]*$/ {f=1} {prev=$0} END {exit !f}' .github/workflows/windows-ci-leg.yml && grep -q -F -e 'windows-latest' .github/workflows/windows-ci-leg.yml` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-31 | assay-verifier-app[bot] @ 2a3b4be18948 (on-behalf-of human:ian) (forge-identity) |
| 8 | `grep -n -e 1145 -e 1146 -e 1223 docs/streams/desktools-v2/brief-06-*.md; test $? -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-31 | assay-verifier-app[bot] @ 2a3b4be18948 (on-behalf-of human:ian) (forge-identity) |
| 9 | `f=$(ls docs/streams/desktools-v2/brief-06-*.md); grep -q -F -e 'cmd/cellctl' "$f" && grep -q -E -e '^gate: human$' "$f" && grep -q -F -e '(desktools-v2/13 GitLab row)' "$f" && grep -q -F -e '(desktools-v2/13 Windows row)' "$f"` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-31 | assay-verifier-app[bot] @ 2a3b4be18948 (on-behalf-of human:ian) (forge-identity) |
| 10 | `miss=0; for n in 641 642 1604 1621 1418 1424 1805 1435 1644 678 1569 1693 1573 1203 655 676 677 1477 1411 1412 1415 865 1667 1794; do grep -q -E -e "^[\|] #$n [\|] desktools-v2/[0-9]+" -e "^[\|] #$n [\|] out-of-scope" docs/streams/desktools-v2/brief-13-platform-gates-and-reconciliation.md \|\| { echo "UNOWNED #$n"; miss=1; }; done; test $miss -eq 0` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-31 | assay-verifier-app[bot] @ 2a3b4be18948 (on-behalf-of human:ian) (forge-identity) |
| 11 | `grep -q -E -e '^[\|] 10 [\|].*[\|] implemented [\|]' -e '^[\|] 10 [\|].*[\|] verified [\|]' -e '^[\|] 10 [\|].*[\|] done [\|]' docs/streams/desk-containers/README.md` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-31 | assay-verifier-app[bot] @ 2a3b4be18948 (on-behalf-of human:ian) (forge-identity) |
| 12 | `statusgen --consumers --root .` | pass exit=0 | sha256:b850da2b1e16 | 2026-10-31 | assay-verifier-app[bot] @ 2a3b4be18948 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5102 run=b1c23fe038e96eb144b0588b8b8150948ce078423e0947362dc5d17fc3056957 source=2a3b4be189484335a8592a80d619011d350a74c7 brief=docs/streams/desktools-v2/brief-13-platform-gates-and-reconciliation.md model=claude-opus-5-5 tier=strong

Rows with no acceptance credit from the witness above:

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 6 | `d=$(mktemp -d) && cd statusgen && go build -o "$d/statusgen" . && cd .. && "$d/statusgen" --root . --lint >/dev/null 2>&1; echo rc=$?` | rc=0 | could-not-run as an obligation: the witnessed exit 0 comes from the row's trailing echo, and the witnessed output hash sha256:91d957f8f274 is the hash of the single line rc=1, not rc=0 (sha256:93ff7811a209). The lint stops on one could-not-check on a neighbour brief: desktools-v2/65: protected table: acceptance transition: could-not-check human authority: --acceptance-forge is required (medici-finance/assay#4823, open). | 2026-10-31 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 13 | `miss=0; for n in 641 642 1604 1621 1418 1424 1805 1435 1644 678 1569 1693 1573 1203 655 676 677 1477 1411 1412 1415 865 1667 1794; do c=$(gh api --paginate "repos/{owner}/{repo}/issues/$n/comments" --jq '.[] \| select(.body \| contains("(desktools-v2/13 triage)")) \| .id') \|\| { echo "READ FAILED #$n"; miss=1; continue; }; test -n "$c" \|\| { echo "NO TRIAGE COMMENT #$n"; miss=1; }; done; test $miss -eq 0` | exit 0 and nothing printed | could-not-run: excluded from the witness by --rows. An admitted verifyrun row has no sanctioned forge credential path (medici-finance/assay#4700, open): verifyrun strips forge tokens from the row, and an ambient login is outside the verifier's identity rules. | 2026-10-31 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6-supplemental | same lint with --acceptance-forge and the verifier App installation token, in the scratch clone | no acceptance credit | exit 0, LINT: PASS, no PROBLEM line | 2026-10-31 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 13-supplemental | the row's loop, outside statusgen, with the verifier App installation token scoped to each read and the repository named explicitly | no acceptance credit | miss=0, 24 comment ids, no READ FAILED, no NO TRIAGE COMMENT. Control: the same select on medici-finance/assay#1836 returns 0. | 2026-10-31 | assay-verifier-app[bot] (on-behalf-of human:ian) |

Hand reading at 2a3b4be18948 (scratch clone, same SHA):

- Row 1: ci.yml lines 155 to 157 run windows/amd64 go vet, the per-package windows go test -c loop and darwin/arm64 go vet. The only ci.yml change since the 2026-10-30 run is in the release image contract step, outside this leg.
- Row 2: rc=0 / class e (glab subprocess): desk=0 statusgen=0 / class f (GitLab API literal): desk=61 statusgen=8. Also printed: forge reach-around sites: 74 (desk: 36, statusgen: 38), statusgen sites: 38, forgeread sites: 0.
- Row 3: PROBE PASS; git status --porcelain tools/desk printed nothing afterwards.
- Row 4: desktools-v2/02 63 / desktools-v2/13 glab=0 gitlab-literal=45.
- Row 5: go test exit 0, --- PASS: TestVerifyRowPortability (0.00s), 12 PASS lines (the top level and 11 subtests). The mutation script printed MUTATION PASS: disabled detector was rejected. git status --porcelain statusgen was empty afterwards.
- Row 6: rc=1, LINT: FAIL 1 problem(s), the single PROBLEM recorded above. 112 lines are tagged verify-row-portability in both the plain and the --acceptance-forge lint; that is the NOTICE baseline at this SHA.
- Row 7: windows-ci-leg.yml lines 84 and 85 are the run block (cd tools/desk, then go test ./...) of a job on windows-latest (line 75). Triggers include a nightly schedule (cron 41 3 * * *), so the Windows proof runs nightly and at release, not on a PR, which deliverable 4 allows.
- Row 8: grep exit 1, nothing printed, so the row's test exits 0.
- Row 9: exit 0 against brief-06-installation-token-scoping.md.
- Row 10: exit 0, nothing printed; all 24 owner rows are present.
- Row 11: exit 0. The desk-containers board row 10 itself reads implemented.
- Row 12: exit 0: consumers: no brief files in the diff against 2a3b4be18948 — nothing to corroborate. This is vacuous on merged main, the tool-side gap medici-finance/assay#1281 (still open). A supplemental run at the delivering commit, with --brief desktools-v2/13 and full SHAs for --base and --delivery, gave exit 0 with summary: 8 corroborated, 0 disproved, 1 unchecked. No acceptance credit.

Findings (advisory; none is a Verify-row failure):

- **Row 6's witness reads pass while its Expect is unmet.** The row ends in echo, so its exit is always 0, and statusgen verifyrun --check counts it as pass. The hash is the only witness of rc=1.
- **Row 13's --check pass is old.** It rests on the last row 13 witness, from 2026-10-22 at 05dc9fbc88f2, since no later run could witness that row.
- **Class f baseline drift.** The live class f desk count is 61 against the recorded gitlab-literal=45. Classes e and f feed no total and no gate; --baseline corrects the drift.
- **Class a to d drift.** The forge reach-around total reads 74 against the recorded desktools-v2/02 baseline of 63. Classes e and f stay excluded from that total; row 3 proves the exclusion.

Risk-bearing value. Risk metadata is present: regulatory, customer, irreversible and sensitive-data all read no, and the gate is model. The delivered work touches .github/workflows, so the enumeration was done anyway, over the counter script, the probe, the baseline file, the statusgen portability detector, the mutation script and both workflow files at this SHA.

- desktools-v2/13 glab=0 gitlab-literal=45 @ forge-ban-baseline.txt:2. A recorded baseline: if it is wrong in the low direction, it hides growth. Reversible with --baseline.
- desktools-v2/02 63 @ forge-ban-baseline.txt:1. Not this brief's value, and untouched by it. Reversible.
- GLAB_SUBCMDS = issue, mr, api, auth, repo, release, ci, label, schedule, snippet, variable @ forge-ban.sh:181. Reversible.
- class f pattern = /api/v4 or gitlab.com @ forge-ban.sh:187 and :195. Reversible.
- BACKEND_EXCLUDE = forge_github, forge_gitlab @ forge-ban.sh:75. Reversible.
- probe plants exec.Command("glab", "api") and "/api/v4" @ forge-ban-probe.sh:13 and :14. Reversible.
- detector regexes verifyOSMarker, verifyShellC, verifyFindstr and verifyTmpPath @ statusgen verifyportability.go:11 to 14. NOTICE-level only. Reversible.
- cross-compile targets GOOS=windows GOARCH=amd64 and GOOS=darwin GOARCH=arm64 @ ci.yml:155 to 157. Reversible.
- runs-on windows-latest @ windows-ci-leg.yml:75, and -timeout 45s @ portability-mutation.sh:17. Operational.

None of these is irreversible. Top-ranked entry:

RISK-VALUE: DERIVED — desktools-v2/13 glab=0 gitlab-literal=45 @ docs/streams/desktools-v2/forge-ban-baseline.txt:2 — an independent git grep recount (not the counter) applied the brief's class f definition: lines matching /api/v4 or gitlab.com in .go and .sh files under tools/desk, tools/cellctl and plugins/assay, excluding the two forge backends, _test.go, .test.sh and the counter itself. It gives 45 at the delivering commit 85aeeb8e1f8d and 61 at this SHA, matching the recorded baseline and the live count.

**VERIFY: BLOCKED** — rows 1 to 5 and 7 to 12 pass at merged main 2a3b4be18948 by the execution witness and by hand reading. Row 6 could-not-run: the whole-tree lint is red only on a neighbour brief's could-not-check (medici-finance/assay#4823, open), and passes with --acceptance-forge. Row 13 could-not-run: an admitted row has no forge credential path (medici-finance/assay#4700, open). No obligation was observed failing.
