---
brief: windows-port/04
title: Windows CI leg — statusgen --lint + a desk-verb smoke on Windows
why: >-
  "Runs on Windows" is a claim until a check corroborates it. Every existing CI leg runs on a
  self-hosted Linux runner and the toolchain is hand-installed linux-amd64 — nothing has ever
  executed on Windows. This brief adds a Windows runner leg that proves `statusgen --lint` exits
  0 and a desk-verb smoke passes on Windows, turning the end-state claim into a green check a
  reviewer and an adopter can trust.
wave: 1
depends: ["windows-port/01", "windows-port/02"]
unblocks: ["windows-port/05"]
effort: M
gate: human
gate-why: >-
  Adds a job under .github/workflows/, which is a security-classified path in this repo:
  a workflow file decides what runs with the repo's CI credentials, and a workflow-file
  change cannot be pushed by an agent credential at all — it needs a workflow-scoped one,
  i.e. a human's hands. `irreversible: yes` records that, the same answer windows-port/01
  gives for the same reason; regulatory / customer / sensitive-data remain honestly "no"
  (a CI leg reads no regulated, customer or secret surface). The human gate is therefore
  risk-derived, not hand-set over four "no"s.
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-01 by windows-port authoring session
sources:
  - "Ian's direction (2026-09-01): a windows runner leg proving statusgen --lint + a desk-verb smoke passes on Windows"
  - "survey (2026-09-01, re-surveyed 2026-09-02 @ origin/main): every runs-on in .github/workflows is Linux — self-hosted medici-builder-public / medici-builder-release, plus one GitHub-hosted ubuntu-latest job in inbound-triage.yml (landed after the first survey); zero windows-latest; ci.yml build-test does go build+vet (not go test); assay-statusgen.yml lint job runs statusgen --root . --lint; toolchain hand-installed linux-amd64 by construction"
  - "windows-port/01: emits the statusgen-windows-<arch>.exe asset the smoke can install and run"
  - "windows-port/02: the portability triage — which desk verb is safe to smoke on Windows (a verb that does not shell out to a POSIX-only surface)"
  - "windows-port/03: the Windows install path — when landed, this leg's smoke installs and runs THAT release binary rather than a from-source build (the release-binary smoke that closes the CI-proven claim)"
  - "GitHub-hosted runners: windows-latest is free for public repos (medici-finance/assay is public); windows-11-arm hosted runners exist but availability/eligibility is the open question this brief resolves for native arm64 smoke"
  - "freshness-checked 2026-09-01 @ origin/main: no workflow runs on any Windows runner"
consumers:
  - ".github/workflows/ (a windows leg — a new job or a matrix OS axis): fixed-here"
  - "docs/adopting-assay.md: follow-up windows-port/05 (the doc points at the green Windows CI as the 'CI-proven' evidence)"
---

# Brief 04 — Windows CI leg: statusgen --lint + a desk-verb smoke

## Context

files:
- **amend or create** a Windows CI leg under `.github/workflows/` — either a new job or an
  added OS axis on an existing job. The leg runs on a Windows runner (see the runner decision
  below).

facts:
- **No CI has ever run on Windows.** Every `runs-on:` today is Linux — the self-hosted
  `medici-builder-public` / `medici-builder-release` labels, plus one GitHub-hosted
  `ubuntu-latest` job in the inbound-triage workflow — and the Go toolchain is hand-installed
  `go…linux-amd64.tar.gz`, so Windows is a genuinely new substrate, not a matrix tweak on an
  existing job. (Re-surveyed 2026-09-02 @ origin/main; the `ubuntu-latest` job landed after the
  brief was first authored.)
- **Runner choice — the substrate decision, resolved in this brief, not gated out:**
  `medici-finance/assay` is public, so **GitHub-hosted `windows-latest` is free** and needs no
  procurement. Use it. This is the key difference from the harness-portability stream, whose true
  head is a procured Codex environment — here there is no external-environment head for amd64.
- **The smoke's two required assertions** (Ian's direction): (1) `statusgen --lint` exits 0 on
  Windows; (2) a desk-verb smoke passes. For (2), pick a verb that brief 02's triage classes
  windows-runnable and that needs no live forge (offline envelope) — e.g. a `--help`/`--version`
  / a dry-run/validate verb that exercises the binary's real code path without a network write.
  A verb that shells out to a POSIX-only surface is the wrong choice; the triage says which.
- **What the leg should exercise — the RELEASE binary, not a from-source build, where feasible.**
  The stream's claim is that the *pinned release* runs on Windows. Prefer installing the
  windows asset via brief 03's install path and smoking THAT; if 03 has not landed when this
  leg is built, a `GOOS`-native `go build` on the windows runner + `--lint`/smoke is the interim
  form, with a note that the release-binary smoke follows 03.
- **The `.exe` matters in CI too:** the built/installed binary is `statusgen.exe`; the workflow
  invokes it by the suffixed name (or via a shim the runner resolves).

single-point-of-failure: none — this brief ADDS a corroborating check; it removes no control. If
the Windows leg is red, the stream is not done, which is the point.

## Open question — native windows/arm64 smoke (surfaced, not blocking)

`windows-latest` is `amd64`. A NATIVE `windows/arm64` smoke needs a `windows-11-arm` hosted
runner (recently available) or a self-hosted arm64 Windows runner. **This does NOT gate the wave
structure:** windows/arm64 ships cross-compiled + checksummed from brief 01 regardless, and this
leg's amd64 smoke proves the Go/Windows path works. The arm64 native-smoke row is authored
**BLOCKED (needs windows-arm64 runner)** and resolved when a runner is available — treated exactly
like harness-portability's "blocked is a state, not a failure." Do not green it from the amd64
result, and do not block the amd64 leg waiting on it. Record the runner-eligibility finding
(available / not-yet / needs-self-hosted) in the PR so the desk can route the arm64 follow-up.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands (you may add the workflow file;
  you do not dispatch it). Commit only per the task instructions.
- **Additive-only on `.github/workflows/`.** This brief ADDS a Windows CI leg — a new job / OS
  axis that runs read-only checks (`--lint`, an offline smoke). It weakens NO existing control:
  it does not touch the leak-sweep or forge-surface control workflows, the release guard, or any
  required-check assertion. The four risk answers are `no` on that basis; per the security-gate
  rule, if a change would weaken any control, STOP and escalate — this does not.
- Stop at `implemented` — you do not set verified/done.
- The smoke stays OFFLINE — no verb that contacts a live forge/cluster. `--lint`, `--version`, a
  dry-run/validate verb only.
- A vacuously-green leg (a smoke that runs nothing, or `|| true`-swallows a failure) is a rejected
  design — the leg must fail when the binary fails.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Add a Windows CI leg on `windows-latest` (a new job or an OS-axis matrix entry). Install Go on
   the windows runner (GitHub-hosted `windows-latest` ships Go / `actions/setup-go` works there,
   unlike the self-hosted Linux pool) or install the brief-03 release binary.
2. Run `statusgen --lint` (against a small fixture tree or the repo's own `docs/streams`) and
   assert exit 0.
3. Run the chosen desk-verb smoke (offline; the triage-approved verb) and assert it passes.
4. Author the native-arm64 smoke row as BLOCKED with its reason, and record the
   `windows-11-arm` runner-eligibility finding in the PR body.
5. Confirm the leg actually reddens on failure (fail-first: point the smoke at a deliberately
   broken input / a nonexistent verb and show the leg goes red, so the green is load-bearing).

## Verify (executable — no prose-only DoD items)

| # | Command | Expect |
|---|---------|--------|
| 1 | A Windows leg exists in a workflow: `grep -rlE 'runs-on: *windows-latest' .github/workflows/` | at least one file listed |
| 2 | The leg runs `statusgen --lint`: `grep -rEA30 'runs-on: *windows-latest' .github/workflows/ \| grep -qE 'statusgen([.]exe)? +.*--lint'; echo $?` | `0` |
| 3 | The leg runs a desk-verb smoke: `grep -rEA40 'runs-on: *windows-latest' .github/workflows/ \| grep -qiE -e 'smoke' -e '--version' -e '--help' -e 'dry-run' -e 'validate'; echo $?` | `0` |
| 4 | **Offline envelope** — the smoke names no live-forge verb: `grep -rEA40 'runs-on: *windows-latest' .github/workflows/ \| grep -qiE -e 'deskpost' -e 'deskpr' -e 'gh pr create' -e 'gh pr comment' -e 'gh issue create' -e 'gh issue comment' -e 'git push'; echo $?` | `1` (no mutating/network verb in the smoke) |
| 5 | **Dereferencing — statusgen genuinely lints clean on a windows-built binary** (proves the leg's assertion is real, run from any host via cross-build + a linux `--lint` as a proxy, plus the workflow's own windows run is the true check): `cd statusgen && go build -o /tmp/wp04-sg . && /tmp/wp04-sg --root ../docs/streams --lint; echo "exit=$?"` | `exit=0` — statusgen lints the stream tree clean (the windows leg runs the same command on the windows binary) |
| 6 | The native-arm64 row is present and BLOCKED, not greened: `grep -qiE -e 'arm64.*BLOCKED' -e 'BLOCKED.*arm64' -e 'windows-11-arm' .github/workflows/*.yml docs/streams/windows-port/brief-04-windows-ci-leg.md; echo $?` | `0` — the arm64 native smoke is explicitly held with its reason |
| 6a | **Positive control for row 6** — arm64 is NOT falsely marked passing: `grep -riE -e 'windows.?arm64 .*PASS' -e 'windows.?arm64 .*green' -e 'windows.?arm64 .*verified' .github/workflows/ docs/streams/windows-port/brief-04-windows-ci-leg.md; echo $?` | `1` |
| 7 | **Consumers routing corroborated by the diff** (run on the implementer's branch): `statusgen --root . --consumers windows-port/04; echo $?` | `0` — the Windows CI leg (fixed-here) is proved by the branch diff |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     The AUTHORITATIVE evidence for rows 2-3 is a green run of the windows-latest leg itself
     (its run URL + the statusgen --lint exit + the smoke result). Row 5 is a host-side proxy.
     The arm64 native-smoke row is BLOCKED until a windows-arm64 runner exists.
     "verified" requires a non-implementer. -->

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|

## Review
Gate: **human** (from frontmatter, risk-derived: `irreversible: yes` — it adds a job under
`.github/workflows/`, a security-classified path that only a workflow-scoped credential can
push); regulatory / customer / sensitive-data are `no` — this adds a CI leg and
removes no control. The reviewer confirms the leg is not vacuously green (the fail-first
demonstration reddens it), the smoke stays offline (row 4), and the arm64 native-smoke is held
BLOCKED with its reason rather than greened from amd64 (rows 6/6a). The authoritative evidence is
a green `windows-latest` run, recorded in the Evidence section.
