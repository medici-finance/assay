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
authored: 2026-09-29 by the desk, scoping trusted issue #1836
sources:
  - "#1836 — §1a (CI never cross-compiles tools/desk), §1b (forge-ban is GitHub-asymmetric), §1c (Verify rows assume POSIX), §2 brief-06 row (stale), §3 (the unowned issue list), §4 T1/T8/T9 + the secondary windows-latest leg, §5 acceptance"
  - ".github/workflows/windows-ci-leg.yml — builds and lints statusgen only, per #1836 §1a; re-confirmed at implementation"
  - "tools/desk/scripts/forge-ban.sh + docs/streams/desktools-v2/forge-ban-baseline.txt — the GitHub-only counter and its baseline, per #1836 §1b"
  - "tools/desk/cmd/cellctl/shims.go — present at authoring 2026-09-29 @ b89b39572; the Go port brief 06's row 4 must target (desk-containers/10 retired the shell oracle it names)"
  - "#1145, #1146, #1223 — closed, per #1836 §2; brief 06 still cites them as its basis"
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
  - "tools/desk/scripts/forge-ban.sh, docs/streams/desktools-v2/forge-ban-baseline.txt: follow-up desktools-v2/13 (this brief)"
  - "statusgen (the Verify-row portability NOTICE): follow-up desktools-v2/13 (this brief)"
  - ".github/workflows/ci.yml, windows-ci-leg.yml: follow-up desktools-v2/13 (this brief — staged, human-pushed, see deliverable 1)"
  - "docs/streams/desktools-v2/brief-06-installation-token-scoping.md: follow-up desktools-v2/13 (this brief re-derives it)"
  - "the §3 platform issues on the tracker: follow-up desktools-v2/13 (this brief's triage table)"
version: 1
id: c3f52e25-c133-4e70-a242-c6392d215efb
---

# Brief 13 — platform gates and reconciliation

## Context

Issue #1836's other half: the gates. Today a Windows compile break in `tools/desk` is first
seen at release time (PR CI builds it on Linux only; the cross-build loop runs in
`release.yml`), and the seam contract's ban counter enforces only its GitHub half — class (a)
counts `gh` subprocesses but not `glab`, class (d) counts the `api.github.com` literal but not
`/api/v4` or `gitlab.com`, so the "count STRICTLY LOWER than baseline" rows can go green while
GitLab reach-arounds grow. Separately, brief 06 has gone stale: the issues it cites are
closed, and its row 4 tests a shell oracle the Go cellctl port retired. And the known Windows
and GitLab issues (#1836 §3) sit unowned — none is claimed by any brief row, so each
resurfaces as a fresh report.

**Delivery mechanic — workflow files.** App tokens cannot push ` .github/workflows/*` (the
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
   not the release.
2. **T8 — forge-ban symmetry.** `forge-ban.sh` gains a `glab` subprocess class and a
   GitLab-API-literal class (`/api/v4`, `gitlab.com` outside the two backend files), each with
   a probe-file negative row in the same style as brief 08's row 3; `forge-ban-baseline.txt`
   records both new columns so the STRICTLY-LOWER rows now bind GitLab reach-arounds too
   (today's non-backend literals in `tools/desk/cmd/cellctl/cell.go`, `tools/desk/cmd/cellctl/new.go`, `tools/desk/cmd/deskpost/forgeclient.go`,
   `tools/desk/cmd/deskflip/flip.go`, `tools/desk/internal/gitcore/claimref.go` and `tools/desk/internal/deskkit/*` are the
   initial count). If any class proves impractical to count, the seam contract is amended in
   the same commit to state the ban is GitHub-only for that class — never a silent gap.
3. **T9 — Verify-row portability lint.** `statusgen --lint` NOTICEs (not fails) a Verify row
   that hardcodes `/tmp`, `sh -c`/`bash -c` or `findstr` unless the row carries an explicit OS
   marker, nudging authors to a Go test or `mktemp`/`${TMPDIR:-/tmp}`. Calibrated against the
   existing tree: the NOTICE list it produces on main today is recorded in Evidence as the
   baseline, and the notice text names the portability rule.
4. **Windows test leg** (staged for human push with deliverable 1). `windows-ci-leg.yml`
   gains `cd tools/desk && go test ./...` on `windows-latest` (free on a public repo). Expected
   green once desktools-v2/12's `privateTempDir` lands; if it lands first, the leg is
   expected red on the custody tests and the PR says so — that is the leg working, and 12 is
   the fix.
5. **Brief 06 re-derived against the current tree.** The closed issues #1145, #1146 and #1223
   are dropped from its basis or re-pointed at what actually replaced them; row 4 targets the
   Go cellctl (`cmd/cellctl`, whose `shims.go` resolves `CELLCTL_GH_AMBIENT` before the HOME
   swap) instead of `tools/cellctl/testdata/cellctl-shell-oracle.sh`; the POSIX-only generated
   shims get an explicit statement — a Windows shim story, or out of scope with a named owner;
   and the GitLab identity-path issues (#1573, #1203, #655, #676, #677) are either taken into
   its scope or assigned an owner. Its gate stays `human` — the re-derivation is offered to
   that gate, not waved through.
6. **Platform-issue triage, on the record.** Every issue in #1836 §3 (Windows: #641, #642,
   #1604, #1621, #1418, #1424, #1805, #1435, #1644, #678, #1569, #1693; GitLab: #1573, #1203,
   #655, #676, #677, #1477, #1411, #1412, #1415, #865, #1667, #1794) gets a comment naming its
   owner — a brief row that claims it (desktools-v2/12 for the custody, env, conformance and
   tier-gap classes) or an explicit out-of-scope with the stream that owns it — and the same
   mapping lands as a table in this brief's Evidence. The two newly-observed gaps are already
   owned: the cellctl non-POSIX-path gap by desktools-v2/12 deliverable 1, the deskfleet
   CE-approvals-404 gap by desktools-v2/12 deliverable 7. The stale `desk-containers` README
   row 10 is corrected in this brief's PR.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -q 'GOOS=windows' .github/workflows/ci.yml && grep -q 'go test -c' .github/workflows/ci.yml` | exit 0 — the cross-compile leg is in PR CI (passes once the human-applied workflow commit lands, per the delivery mechanic) |
| 2 | `sh tools/desk/scripts/forge-ban.sh; echo rc=$?` | prints the new `glab` and GitLab-literal class counts alongside the existing classes, and `rc=0` on the clean tree |
| 3 | `sh -c 'f=tools/desk/internal/deskkit/zz_banprobe.go; printf "package deskkit\nimport \"os/exec\"\nvar _ = exec.Command(\"glab\", \"api\")\n" > "$f"; sh tools/desk/scripts/forge-ban.sh >/dev/null 2>&1; rc=$?; rm -f "$f"; echo "rc=$rc"; test "$rc" -ne 0'` | exit 0; prints a non-zero `rc=` — a planted `glab` shell-out fails the counter, and the probe file is removed whatever the result |
| 4 | `grep -q 'glab' docs/streams/desktools-v2/forge-ban-baseline.txt` | exit 0 — the baseline records the GitLab columns |
| 5 | `f=/tmp/zz-portability-probe.md; printf '%s\n' '---' 'brief: x' 'title: t' '---' '## Verify' '| # | Command | Expect |' '|---|---|---|' '| 1 | sh -c "echo hi > /tmp/x" | ok |' > $f && statusgen --lint --brief $f 2>&1 | grep -i portab; rc=$?; rm -f $f; test $rc -eq 0` | exit 0 — the lint prints the portability NOTICE naming the rule (a NOTICE, not a failure; flag spelling per the lint's implementation) |
| 6 | `statusgen --root . --lint >/dev/null 2>&1; echo rc=$?` | `rc=0` — the new lint is NOTICE-level: the existing tree, POSIX rows included, still lints clean, and the NOTICE baseline is recorded in Evidence |
| 7 | `grep -q 'windows-latest' .github/workflows/windows-ci-leg.yml && grep -q 'tools/desk' .github/workflows/windows-ci-leg.yml` | exit 0 — the windows leg runs the tools/desk suite (passes once the human-applied workflow commit lands) |
| 8 | `grep -E '1145|1146|1223' docs/streams/desktools-v2/brief-06-*.md; test $? -eq 1` | exit 0 — the closed premises are out of brief 06 |
| 9 | `grep -q 'cmd/cellctl' docs/streams/desktools-v2/brief-06-*.md && grep -q 'gate: human' docs/streams/desktools-v2/brief-06-*.md` | exit 0 — row 4 targets the Go port and the gate is untouched |
| 10 | `grep -c 'desktools-v2/1[23]\|out-of-scope' docs/streams/desktools-v2/brief-13-*.md` | prints a count ≥ 22 — every §3 issue appears in the Evidence triage table with an owner |
| 11 | `grep -q 'implemented\|verified\|done' docs/streams/desk-containers/README.md` | exit 0 — row 10 no longer reads `todo` |

## DoD

- All eleven Verify rows pass; rows 1 and 7 pass after the human-applied workflow commit and
  the PR tracks that commit explicitly.
- No gate weakened: the forge-ban change only ADDS classes, the lint only NOTICEs, and brief
  06 keeps `gate: human`.
- Every §3 issue carries its triage comment on the tracker, matching the Evidence table.
