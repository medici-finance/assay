---
brief: assay:assay:windows-port:04
title: Windows CI leg — statusgen --lint + a desk-verb smoke on Windows
why: >-
  "Runs on Windows" is a claim until a check corroborates it. Every existing CI leg runs on a
  self-hosted Linux runner and the toolchain is hand-installed linux-amd64 — nothing has ever
  executed on Windows. This brief adds a Windows runner leg that proves `statusgen --lint` exits
  0 and a desk-verb smoke passes on Windows, turning the end-state claim into a green check a
  reviewer and an adopter can trust.
wave: 2
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
schema: brief-v2
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
version: 1
id: 570ade9d-9904-423e-95fc-2c25344fe236
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
| 5 | **Dereferencing — statusgen genuinely lints clean on a windows-built binary** (proves the leg's assertion is real, run from any host via cross-build + a linux `--lint` as a proxy, plus the workflow's own windows run is the true check): `cd statusgen && go build -o /tmp/wp04-sg . && /tmp/wp04-sg --root .. --lint; echo "exit=$?"` | `exit=0` — statusgen lints the stream tree clean, resolving `docs/streams` under the repo `--root` (the windows leg runs the same command on the windows binary) |
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
| 1 | `grep -rlE 'runs-on: *windows-latest' .github/workflows/` | — | **satisfied on apply** — the staged `ci/staged-workflows/windows-ci-leg.yml` carries a `runs-on: windows-latest` job (`windows-smoke`); turns green when the maintainer promotes it into `.github/workflows/`. Proven against a copy of the staged file placed at `.github/workflows/`: one file listed. | 2026-09-06 | host (apply-gated) |
| 2 | `grep -rEA30 'runs-on: *windows-latest' .github/workflows/ \| grep -qE 'statusgen([.]exe)? +.*--lint'` | — | **satisfied on apply** — the leg runs `./statusgen.exe --root .. --lint`. Proven `0` against the staged file copied to `.github/workflows/`. Authoritative evidence is a green promoted-leg run. | 2026-09-06 | host (apply-gated) |
| 3 | `grep -rEA40 'runs-on: *windows-latest' .github/workflows/ \| grep -qiE -e 'smoke' -e '--version' …` | — | **satisfied on apply** — the offline smoke is `./statusgen.exe --version` in a step named "Desk-verb smoke". Proven `0` against the staged copy. | 2026-09-06 | host (apply-gated) |
| 4 | `grep -rEA40 'runs-on: *windows-latest' .github/workflows/ \| grep -qiE -e 'deskpost' -e 'deskpr' -e 'gh pr create' … -e 'git push'` | — | **satisfied on apply** — no mutating/forge verb appears in the leg's steps. Proven `1` (no match) against the staged copy. Offline envelope holds. | 2026-09-06 | host (apply-gated) |
| 5 | `cd statusgen && go build -o /tmp/wp04-sg . && /tmp/wp04-sg --root .. --lint; echo "exit=$?"` | `0` | `LINT: PASS` — statusgen lints the stream tree clean on a from-source build (the proxy for the windows leg, which runs the same command on `statusgen.exe`). **Correction:** the brief's literal command uses `--root ../docs/streams`, but statusgen resolves `<root>/docs/streams` under `--root` (it treats `--root` as the REPO ROOT), so `--root ../docs/streams` reads a nonexistent `../docs/streams/docs/streams` and exits 1. The correct repo-root invocation is `--root ..`, which the staged leg uses. | 2026-09-06 | host (macOS/darwin, linux-equivalent proxy) |
| 6 | `grep -qiE -e 'arm64.*BLOCKED' -e 'BLOCKED.*arm64' -e 'windows-11-arm' .github/workflows/*.yml <brief>` | `0` | The native-arm64 row is present and held BLOCKED: this brief's "Open question" section and Evidence carry `windows-11-arm` + `arm64 … BLOCKED`, and the staged yml carries an `arm64-native-smoke` job held BLOCKED (`if: false`). Passes now via the brief; also passes via the yml on apply. | 2026-09-06 | host |
| 6a | `grep -riE -e 'windows.?arm64 .*PASS' -e 'windows.?arm64 .*green' -e 'windows.?arm64 .*verified' .github/workflows/ <brief>` | `1` | No false-pass: arm64 is nowhere marked passing/green/verified. Proven `1` (no match) over the brief + the staged copy. | 2026-09-06 | host |
| 7 | `statusgen --root . --consumers windows-port/04` | — | **satisfied on apply** — the declared consumer is `.github/workflows/` (fixed-here); the branch diff only touches `ci/staged-workflows/` in the prep-only phase, so consumer routing is corroborated once the maintainer promotes the staged file into `.github/workflows/`. | 2026-09-06 | host (apply-gated) |

### Fail-first (Task 5) — the green is load-bearing

The leg's assertions run under `set -euo pipefail`, and statusgen exits non-zero on a broken
input, so a failure reddens the job rather than being swallowed. Measured on the host binary at
implementation time (2026-09-06):

- `statusgen --i-am-not-a-verb` → **exit 2** (a bogus verb is refused).
- `statusgen --root /nonexistent-tree --lint` → **exit 1** (a broken lint root fails).

The staged leg encodes an opt-in fail-first demonstration: `workflow_dispatch` input
`failfirst=true` runs a step that points the smoke at a nonexistent verb and INVERTS the
assertion (fails the job unless statusgen refuses), so a maintainer can show on demand that the
leg is not vacuously green.

### Native windows/arm64 smoke — BLOCKED

The native windows/arm64 smoke is **BLOCKED**: `windows-latest` is amd64, and a native
windows/arm64 smoke needs a `windows-11-arm` hosted runner or a self-hosted arm64 Windows runner.
Runner-eligibility finding: `windows-11-arm` GitHub-hosted runners exist but their availability to
this repo is unconfirmed at implementation time — the row is held BLOCKED and is NOT derived from
the amd64 result. windows/arm64 still ships cross-compiled + checksummed from windows-port/01
regardless; only the native smoke is held. The staged yml carries the held `arm64-native-smoke`
job (`if: false`) so the row can never be mistaken for a passing arm64 result.

### Prep-only note (workflow-scope maintainer-apply)

The LIVE workflow file is maintainer-pushed: no bot/App in this repo holds workflow-push
permission (GitHub hard-rejects any App push that creates or updates a `.github/workflows/*`
file). The complete leg is therefore staged at `ci/staged-workflows/windows-ci-leg.yml` (the exact
content that goes into `.github/workflows/`), with the promotion step in
`ci/staged-workflows/README.md`. Rows 1-4, 6 (yml half), and 7 turn green when the maintainer
promotes the staged file into `.github/workflows/`; they are proven here against a copy of the
staged file placed at `.github/workflows/`. Row 5 is proven now as a host-side proxy; row 6 also
passes now via this brief.

### Non-implementer verifier run — VERIFY: PASS (7/7; now against the PROMOTED leg, not a staged copy); HELD at `implemented` (gate: human) — 2026-09-07 assay-verifier (verify-desk dispatch), merged main `cd19bf8`

Runner ≠ implementer (fresh dispatched verifier, offline `KUBECONFIG=/dev/null`; `gh` used only for CI-run reads). The staged leg has been PROMOTED — the workflow now lives at `.github/workflows/windows-ci-leg.yml`, so the implementer's "satisfied on apply" rows resolve against the real live file.

| # | Command | Exit | Observed | Date | Runner |
|---|---------|------|----------|------|--------|
| 1 | `grep -rlE 'runs-on: *windows-latest' .github/workflows/` | 0 | `.github/workflows/windows-ci-leg.yml` (job `windows-smoke`) | 2026-09-07 | assay-verifier |
| 2 | leg runs `statusgen --lint` | 0 | leg runs `./statusgen.exe --root .. --lint` | 2026-09-07 | assay-verifier |
| 3 | leg runs an offline desk-verb smoke | 0 | offline smoke `./statusgen.exe --version` (step "Desk-verb smoke") | 2026-09-07 | assay-verifier |
| 4 | no mutating/forge verb in the leg | 1 (no match) | no deskpost/deskpr/gh-pr/git-push in the leg; offline envelope holds | 2026-09-07 | assay-verifier |
| 5 | `go build -o wp04-sg . && wp04-sg --root <repo> --lint` | 0 | `LINT: PASS` (host from-source proxy; NOTICEs data-quality only) | 2026-09-07 | assay-verifier |
| 6 | arm64 native smoke held BLOCKED | 0 | `arm64-native-smoke` job `if: false` + brief Open-question section | 2026-09-07 | assay-verifier |
| 6a | arm64 nowhere marked passing/green | 1 (no match) | no false-pass | 2026-09-07 | assay-verifier |
| 7 | `statusgen --root . --consumers windows-port/04` | 0 | "no brief files in the diff against cd19bf8 — nothing to corroborate" (vacuous post-merge) | 2026-09-07 | assay-verifier |

**Authoritative green run (rows 2–3):** `windows-ci-leg` run **34152585132**, event push, branch **main**, head **cd19bf8**, conclusion **success**; job `windows-smoke` on `windows-latest`; steps `statusgen --lint (assert exit 0 on Windows)` → success and `Desk-verb smoke (offline; --version)` → success; `Fail-first` skipped (opt-in), `arm64-native-smoke` skipped (`if: false`). https://github.com/medici-finance/assay/actions/runs/34152585132 — also green at #583's promotion head `d684440` (runs 34132073703 + 34132069915).

**RISK-VALUE: DERIVED** — `permissions.contents = read` @ `.github/workflows/windows-ci-leg.yml:71-72` (the leg only reads → builds → --lint → --version; writes to no contents/issue/PR/forge surface, so read is least-privilege). Action pins verified full-SHA against their tags via gh api: `actions/checkout@3d3c42e5…` = v7.0.1 @:80, `actions/setup-go@d35c59ab…` = v5.5.0 @:83 (correct supply-chain form, no floating ref). Reversible knobs (out of scope): `GO_VERSION="1.25.0"`, `if:false`, `failfirst=false`. SPOF: none — additive corroborating check, removes no control.

**VERIFY: PASS** — every row passes on the promoted leg; the authoritative Windows run is green. This brief is **gate: human** (`irreversible: yes` — workflow-file push is a human-only credential boundary): a model verifier CANNOT sign it off. Evidence complete; status stays `implemented`, routed to the human gate. Read-only, no flip.

## Review
Gate: **human** (from frontmatter, risk-derived: `irreversible: yes` — it adds a job under
`.github/workflows/`, a security-classified path that only a workflow-scoped credential can
push); regulatory / customer / sensitive-data are `no` — this adds a CI leg and
removes no control. The reviewer confirms the leg is not vacuously green (the fail-first
demonstration reddens it), the smoke stays offline (row 4), and the arm64 native-smoke is held
BLOCKED with its reason rather than greened from amd64 (rows 6/6a). The authoritative evidence is
a green `windows-latest` run, recorded in the Evidence section.
