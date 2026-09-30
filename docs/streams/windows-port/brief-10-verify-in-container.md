---
brief: assay:assay:windows-port:10
title: "Verify in the harness container: the supported execution-witness runner on Windows"
why: >-
  windows-port/00-05 made a pinned Assay release installable and CI-proven on Windows, but they
  did NOT give Windows a way to RUN a brief's Verify table. `statusgen verifyrun` executes each row
  under `bash -o pipefail`, and on native Windows that shell is often the WSL launcher shim, which
  exits before the row's own command runs — #1418, the defect that made verifyrun record
  could-not-run for a whole table on a Windows host. #1418 correctly stopped calling those rows a
  `fail`; it did not make them runnable. This brief supplies the runner: the execution witness on
  Windows is produced not by a native shell but by the LINUX harness container, where a real
  pipefail bash exists. `statusgen verifyrun --in-container` is the thin, digest-pinned launcher
  that runs verifyrun inside that container against the checkout bind-mounted at /work, so a Windows
  adopter can produce a real, host-owned witness — turning "verify runs on Windows" from a
  could-not-run into a green a reviewer and an adopter can trust.
wave: 3
depends: ["windows-port/03", "windows-port/04"]
unblocks: []
effort: M
gate: human
gate-why: >-
  Adds a CI job under a security-classified workflow path: the leg is staged at
  ci/staged-workflows/windows-ci-leg.yml and a maintainer promotes it into .github/workflows/,
  which an agent credential cannot push at all — it needs a workflow-scoped one, i.e. a human's
  hands. `irreversible: yes` records that, the same answer windows-port/04 gives for the same
  reason. Regulatory / customer / sensitive-data remain honestly `no` — the leg reads no regulated,
  customer, or secret surface; the credential passthrough is a PATH only (containers/secrets.md),
  never a baked or logged secret. The human gate is therefore risk-derived, not hand-set. The human
  also owns two acts only they can perform: harvesting and pinning the harness image's real
  registry digest (the committed pin is a fail-closed placeholder), and — see `## Human decision` —
  choosing the landing mechanism given desk-supervision/12's in-flight retirement of the staged-copy
  pattern this leg rides.
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
design: DR-verify-in-container
issues: []
schema: brief-v2
authored: 2026-09-21 by windows-port authoring session
exec-tier: strong
exec-tier-why: >-
  (b) correctness is a cross-artifact argument — a launcher, a digest-pinned manifest entry, a
  staged CI leg, and adoption docs must agree on one contract (digest-pinned, host-owned,
  credential-by-path); (c) it is credential-passthrough plumbing where a mistake (baking or logging
  a token, or running an un-digest-pinned image) is a supply-chain gap a happy-path test would not
  show.
sources:
  - "The driver's direction (2026-09-21, driver-directed): the container is the supported execution-witness runner on Windows; a digest-pinned wrapper, bind-mount caveats, a Windows CI leg proving the witness lands, adoption-doc delta, and no change to the four `sh -c` sites."
  - "statusgen/verifyrun.go: verifyrun runs each Verify row under `bash -o pipefail`; resolveShellPlan probes the shell once and records could-not-run (never a false fail) when no pipefail-capable POSIX shell bootstraps — the #1418 Windows WSL-launcher defect this brief routes around by running verifyrun inside a Linux container."
  - "#1418: on native Windows `bash.exe` is often the WSL launcher; with no distro it exits 1 with `execvpe(/bin/bash) failed` BEFORE the row runs — verifyrun records could-not-run, which is honest but not runnable. The container supplies the missing runnable shell."
  - "#1410: parseBriefFile's memo cache keys on CONTENT HASH, not mtime, precisely so a coarse-mtime filesystem cannot serve a stale parse — the property the bind-mount NTFS-coarse-mtime caveat relies on."
  - "#1427 (merged): the per-row `Shell` marker lets a Verify row that genuinely needs native-Windows shell behaviour run under cmd/pwsh instead of bash — the narrow exception this brief points adopters at, NOT the default (the container is the default)."
  - "containers/secrets.md (normative runtime credential contract): the ONLY credential surface is the operator-supplied role env-file passed as `--env-file <path>`; no secret in any image layer, and the path is not a secret while its contents are."
  - "docs/docker.md + .github/workflows/docker-publish.yml: `ghcr.io/medici-finance/assay/desk-tools` is the ONE combined Linux image that ships statusgen + the desk binaries — the harness image this brief pins and runs. There is no separate `assay-harness` image."
  - "windows-port/04 (ci/staged-workflows/windows-ci-leg.yml): the existing staged, gate:human Windows CI-leg mechanism and the job shape this brief's leg matches."
  - "docs/streams/decisions/DR-workflow-app-landing.md + docs/streams/desk-supervision/brief-12-retire-staged-copy-landing.md: the OPEN, unratified decision and in-flight brief that rule OUT the staged-copy landing pattern this leg rides (drift #1187, stall #1175/#1185) — named in `## Human decision`, not silently followed."
  - "freshness-checked 2026-09-21 @ origin/main"
consumers:
  - "statusgen/ (the `verifyrun --in-container` launcher): fixed-here"
  - "plugins/assay/paired-versions.yaml (the `harness:` image pin): fixed-here (digest is a fail-closed placeholder until a maintainer harvests the real registry digest)"
  - "ci/staged-workflows/windows-ci-leg.yml (the execution-witness CI leg): fixed-here (staged; a maintainer promotes it — gate:human)"
  - "docs/adopting-assay.md (the Windows execution-witness statement): fixed-here"
version: 1
id: dc58ba3f-4326-430a-b183-2a37d3843290
---

# Brief 10 — Verify in the harness container: the supported execution-witness runner on Windows

## Context

files:
- **add** `statusgen/verifyrun.go` flags `--in-container` and `--env-file`, and a new
  `statusgen/verifyincontainer.go` — the launcher that composes a `docker run` argv and runs
  `statusgen verifyrun` inside the pinned harness container against the checkout bind-mounted at
  `/work`. New tests in `statusgen/verifyincontainer_test.go` (hermetic: a fake `docker` on PATH).
- **add** a `harness:` block to `plugins/assay/paired-versions.yaml` — the witness-runner container
  image, pinned by **tag AND digest**. The image is the SAME combined `desk-tools` Linux image the
  desk containers run (statusgen ships in it — docs/docker.md); there is no separate `assay-harness`
  image. The committed `digest:` is a fail-closed **placeholder** until a maintainer harvests the
  real registry digest.
- **add** two jobs to the staged `ci/staged-workflows/windows-ci-leg.yml` (a maintainer promotes it
  into `.github/workflows/` — gate:human, same as windows-port/04): `verify-in-container` (the live
  witness-lands proof) and a held `windows-verify-in-container` (native-Windows-Docker, BLOCKED).
- **edit** `docs/adopting-assay.md` §Windows adopters — state that the container is THE supported
  execution-witness runner on Windows and that native-Windows Verify rows use the `Shell` column
  (#1427) only when a brief genuinely needs Windows-native shell behaviour, never as a default.

facts:
- **verifyrun needs a POSIX pipefail shell; native Windows often lacks one.** verifyrun executes
  each row under `bash -o pipefail` and never falls back to PowerShell/cmd (their quoting/exit
  semantics would silently reinterpret a row). On native Windows `bash.exe` is frequently the WSL
  launcher shim, which exits before the row runs (#1418). verifyrun records that honestly as
  could-not-run — but the row still did not run. The container is the runnable POSIX shell.
- **The harness image is Linux whichever host launches it.** Running the container on any host with
  a Linux-container Docker backend produces an identical run. The Windows value is that Windows
  adopters USE it because their native shell is unreliable — not that the image is Windows-specific.
  GitHub-hosted `windows-latest` has NO Linux-container backend, so the native-Windows-host proof is
  a held row; the mechanism proof runs on a Docker-capable Linux runner.
- **Pinned by tag AND digest, never `latest`.** The launcher resolves the image by its sha256
  DIGEST and REFUSES fail-closed on a `latest` tag, an absent digest, or a placeholder — the same
  "never a floating ref, never a hand-invented hash" contract paired-versions.yaml already holds for
  the release binaries. A container image digest is a REGISTRY digest (`docker manifest inspect`),
  not the release checksums.txt sha256, so it is harvested online and must never be invented.
- **Credentials by PATH only.** Per containers/secrets.md the only credential surface is the
  operator role env-file passed as `--env-file <path>`. The launcher forwards the PATH; it never
  reads, echoes, logs, or bakes the file's contents. No credential appears on the argv.
- **Host-owned Evidence.** The container writes the witness back into the brief on the bind mount.
  On a POSIX host `--user <uid>:<gid>` maps writes to the invoking user so Evidence lands host-owned,
  not container-root. On Windows there is no POSIX uid (`os.Getuid()` == -1); `--user` is omitted and
  Docker Desktop maps ownership to the host user itself.

single-point-of-failure: the digest pin control (`harnessPin.validate`) — the ONE control standing
between an adopter and running an un-pinned/tampered image. The layer behind it: the run itself is
read-then-witness with NO forge or funds surface, and the credential is a PATH the launcher never
dereferences — so even if a wrong image were somehow run, it executes offline Verify rows and writes
a witness a human reads, not a privileged action. The control fails closed (refuse), not open, and
the CI leg's fail-closed step reddens if it ever stops refusing.

## Human decision
<!-- decision-trigger: creation — the fork is nameable now; filed as the brief lands. Self-contained. -->

This leg touches a workflow-file path (`ci/staged-workflows/windows-ci-leg.yml` → promoted into
`.github/workflows/`), which is gate:human territory: an agent credential cannot push a workflow
file, and promotion needs a workflow-scoped human credential. Two human acts are owed:

1. **Harvest and pin the harness image's real registry digest.** The committed `harness.digest:` is
   a fail-closed placeholder; the launcher REFUSES to run until a maintainer with registry access
   runs `docker manifest inspect ghcr.io/medici-finance/assay/desk-tools:v1.0.6`, takes the sha256
   digest, and pins it. This is a human act (registry credential) and is a `could-not-check` until
   performed.

2. **Choose the landing mechanism — and know the tension before you do.** This leg rides the SAME
   staged-copy → maintainer-hand-copy pattern windows-port/04 and /06 used. That pattern is the
   subject of an OPEN, unratified decision record (`../decisions/DR-workflow-app-landing.md`) and an
   in-flight retirement brief (`../desk-supervision/brief-12-retire-staged-copy-landing.md`), which cite REAL failure modes: a staged copy
   authored against one base silently REVERTS intervening fixes when the live file moves (#1187),
   and the hand-copy step has left briefs `BLOCKED-ON-HUMAN` 9+ days (#1175, #1185). This brief does
   NOT resolve that tension and was directed to proceed on the established, known-working precedent —
   but it names it here rather than following it silently. **Whoever promotes this leg should check
   desk-supervision/12's current state first:** if the workflow-App PR path has landed, this
   change should travel through THAT path (a single workflow-only PR the workflow App authors), and
   this staged addition may itself be reduced to a pointer by that brief.

Default if no answer: none — blocks until answered. The leg stays staged and the digest stays a
placeholder — the launcher refuses (fail-closed), so nothing runs an un-pinned image in the
meantime. Status stays `implemented`.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. You may ADD the staged workflow
  file; you do not promote or dispatch it. Stop at `implemented` — you do not set verified/done.
- **NEVER run a real `docker run` / `docker build` against the live registry or harness image.** The
  launcher's tests are hermetic (a fake `docker` on PATH captures the composed argv). The CI leg's
  witness-lands step is the LIVE proof and is apply-gated (a real Docker + Linux-container runner and
  the harvested digest); it is not, and must not be, faked here.
- **Digest-pinned or refuse.** The launcher runs the image ONLY by a full `sha256:<64hex>` digest and
  refuses `latest`, an absent digest, or a placeholder. Never widen this to a floating tag.
- **Credentials by PATH only, never baked, never logged.** The env-file is forwarded as a path; its
  contents are never read or rendered. No credential on the argv.
- **Do NOT touch the four existing `sh -c` execution sites** — `statusgen/mergecheck.go`,
  `tools/desk/cmd/muhar/main.go`, `tools/desk/cmd/verifyloop/verdictrun.go`, and
  `tools/desk/internal/deskkit/hooks.go`. This brief adds a container launcher around `verifyrun`
  (which uses `bash -o pipefail`, not `sh -c`); it changes none of those sites. Verify row 8 proves
  the diff leaves them untouched.
- **Additive on the workflow file.** The leg ADDS two jobs that run read-only/offline checks; it
  weakens no existing control (leak-sweep, forge-surface, required checks all untouched). If a change
  would weaken any control, STOP and escalate — this does not.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Add `statusgen verifyrun --in-container [--env-file <path>]`: read the digest-pinned `harness:`
   image from `plugins/assay/paired-versions.yaml`, REFUSE fail-closed on `latest`/absent/placeholder
   digest, compose `docker run --rm -v <root>:/work -w /work [--user uid:gid] [--env-file <path>]
   <image>@<digest> statusgen verifyrun --brief <briefRel> …`, and exec it, propagating the inner
   exit code. `--user` on POSIX only; omit it where there is no POSIX uid (Windows).
2. Add the `harness:` block to `paired-versions.yaml` with a fail-closed placeholder digest, keeping
   `tag:` equal to the single release tag `check-paired-versions.sh` enforces.
3. Document the bind-mount caveats (NTFS coarse mtime — the #1410 content-hash property; the exec bit
   may not survive the mount — verifyrun records 126/127 as could-not-run with the reason, never a
   silent skip or a forced pass) in the launcher and the adoption doc.
4. Add the `verify-in-container` CI job (mechanism proof, on a Docker-capable Linux runner) and the
   held `windows-verify-in-container` job (native-Windows-Docker, BLOCKED — no Linux-container
   backend on `windows-latest`), matching windows-port/04's job shape; keep it STAGED, gate:human.
5. State in `docs/adopting-assay.md` that the container is THE supported execution-witness runner on
   Windows and that native-Windows Verify rows use the `Shell` column (#1427) only for a genuine
   Windows-native-shell need, not as a default.
6. Prove the diff does not touch the four `sh -c` sites (Verify row 8).

## Verify

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd statusgen && go build -o /tmp/wp10-sg . && /tmp/wp10-sg verifyrun --help \| grep -c -e '--in-container' -e '--env-file'` | `≥ 2` — the two flags are documented | check |
| 2 | `cd statusgen && go test -run TestVIC -count=1 .` | exit 0 — the launcher's hermetic suite passes (fake `docker` on PATH; no real container): argv composition, digest-pin validation, and the fake-docker end-to-end are all under this one prefix | check +dereference |
| 3 | `(cd statusgen && go build -o /tmp/wp10-sg .) && /tmp/wp10-sg verifyrun --in-container --root . --brief docs/streams/windows-port/brief-10-verify-in-container.md; echo "exit=$?"` | exit non-zero (`exit=2`) — the launcher REFUSES the committed placeholder digest (fail-closed pin control), never runs an un-digest-pinned image | check +dereference |
| 4 | `bash plugins/assay/scripts/check-paired-versions.sh` | exit 0 — the new `harness:` block keeps the manifest's single-tag + hash-shape invariants | check |
| 5 | `grep -qE '^[[:space:]]*image:[[:space:]]+ghcr\.io/.+/desk-tools$' plugins/assay/paired-versions.yaml && grep -qE '^[[:space:]]*digest:' plugins/assay/paired-versions.yaml; echo $?` | `0` — the harness image is pinned (image + digest fields present) | check |
| 6 | The CI leg drives the full path (statusgen → docker → verifyrun → Evidence): `grep -rlE 'verifyrun --in-container' ci/staged-workflows/windows-ci-leg.yml` | at least one file listed — the `verify-in-container` leg exercises the wrapper end to end; its LIVE discharge (the witness landing host-owned) is the promoted-leg run on a Docker+Linux-container runner once the real digest is pinned (apply-gated, not fakeable here) | check:ci +flow |
| 7 | **Offline envelope** — the CI leg's mechanism job names no live-forge verb: `grep -A60 'verify-in-container:' ci/staged-workflows/windows-ci-leg.yml \| grep -qiE -e 'deskpost' -e 'deskpr' -e 'gh pr create' -e 'gh issue create' -e 'git push'; echo $?` | `1` (no mutating/forge verb) | check |
| 8 | **Boundary** — the diff touches none of the four `sh -c` sites: `git diff $(git merge-base refs/remotes/origin/main HEAD)..HEAD -- statusgen/mergecheck.go tools/desk/cmd/muhar/main.go tools/desk/cmd/verifyloop/verdictrun.go tools/desk/internal/deskkit/hooks.go \| wc -l` | `0` — the four `sh -c` execution sites are untouched | check |
| 9 | The adoption doc names the container as the Windows execution-witness runner: `grep -qi 'supported execution-witness runner on Windows' docs/adopting-assay.md; echo $?` | `0` | check |
| 10 | Consumers routing corroborated by the diff (run on the implementer's branch): `statusgen --root . --consumers windows-port/10; echo $?` | `0` — the four declared consumers (launcher, pin, CI leg, adoption doc) are proved by the branch diff | check |
| 11 | **Mutation** — the digest-pin control reddens when broken: `cp statusgen/verifyincontainer.go /tmp/wp10-vic.bak && perl -0pi -e 's/if !pinDigestRe\.MatchString/if false \&\& !pinDigestRe.MatchString/' statusgen/verifyincontainer.go && (cd statusgen && go test -run TestVICRunRefusePlaceholder -count=1 . >/dev/null 2>&1); rc=$?; cp /tmp/wp10-vic.bak statusgen/verifyincontainer.go; echo "mutated-rc=$rc"` | output is `mutated-rc=1` — bypassing the digest check makes the fail-closed refusal test FAIL, proving the control is load-bearing (the file is restored) | check +mutation |

## Evidence
<!-- appended at implementation time: one row per Verify item (command, exit, output hash, date,
     runner). Rows 1-9 are host-runnable and proven at implementation. The AUTHORITATIVE evidence
     for the LIVE witness-lands proof (CI leg's second step) is a green promoted-leg run on a
     Docker+Linux-container runner AFTER a maintainer harvests the real harness digest — apply-gated,
     not fakeable here. The native-Windows-Docker row is BLOCKED until a Linux-container-capable
     Windows runner exists. "verified" requires a non-implementer; this brief is gate:human. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && go build -o /tmp/wp10-sg . && /tmp/wp10-sg verifyrun --help \| grep -c -e '--in-container' -e '--env-file'` | pass exit=0 | sha256:1121cfccd591 | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd statusgen && go test -run TestVIC -count=1 .` | pass exit=0 | sha256:da02f54b70ef | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && go build -o /tmp/wp10-sg .) && /tmp/wp10-sg verifyrun --in-container --root . --brief docs/streams/windows-port/brief-10-verify-in-container.md; echo "exit=$?"` | pass exit=0 | sha256:4bd199fcb53e | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 4 | `bash plugins/assay/scripts/check-paired-versions.sh` | pass exit=0 | sha256:955321a16be7 | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 5 | `grep -qE '^[[:space:]]*image:[[:space:]]+ghcr\.io/.+/desk-tools$' plugins/assay/paired-versions.yaml && grep -qE '^[[:space:]]*digest:' plugins/assay/paired-versions.yaml; echo $?` | pass exit=0 | sha256:9a271f2a916b | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 6 | `grep -rlE 'verifyrun --in-container' ci/staged-workflows/windows-ci-leg.yml` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -A60 'verify-in-container:' ci/staged-workflows/windows-ci-leg.yml \| grep -qiE -e 'deskpost' -e 'deskpr' -e 'gh pr create' -e 'gh issue create' -e 'git push'; echo $?` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 8 | `sh -c` | fail exit=2 | sha256:93710cfbdcc6 | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 9 | `grep -qi 'supported execution-witness runner on Windows' docs/adopting-assay.md; echo $?` | pass exit=0 | sha256:9a271f2a916b | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 10 | `statusgen --root . --consumers windows-port/10; echo $?` | pass exit=0 | sha256:37de28eccd46 | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 11 | `cp statusgen/verifyincontainer.go /tmp/wp10-vic.bak && perl -0pi -e 's/if !pinDigestRe\.MatchString/if false \&\& !pinDigestRe.MatchString/' statusgen/verifyincontainer.go && (cd statusgen && go test -run TestVICRunRefusePlaceholder -count=1 . >/dev/null 2>&1); rc=$?; cp /tmp/wp10-vic.bak statusgen/verifyincontainer.go; echo "mutated-rc=$rc"` | pass exit=0 | sha256:5e67acbcf39d | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |

**Post-merge verify notes — 2026-09-27, non-implementer verifier (assay-verifier-app[bot], model claude-opus-5-5), merged main b227b40768db, implementing PR #1432 (merge f2c158769) plus follow-ups #1468 (d84ddab64) and #1478 (9ebb8125a, harness digest pin).**

Witness environment: darwin host. The witness above was produced with DOCKER_HOST pointed at a
socket that does not exist, so no row could reach a Docker daemon or pull from the registry (offline
envelope; this brief's own ground rule forbids a real container run here). This matters for row 3
only — see below. The result column reads `pass` for rows 3, 5, 7, 9, 10, 11 because each command
ends in `echo`; the decisive value is the printed output, recorded per row here.

Per-row key output (each row re-run by hand at the same SHA to read the output the hash stands for):

- Row 1 — printed `3` (both flags documented; expect ≥ 2). PASS.
- Row 2 — `ok` for the statusgen package; 12 TestVIC tests PASS (pin validate, ref digest, compose
  POSIX / Windows / safe-directory / no-env-file, inner flags, read pin, read pin missing, fake-docker
  run, refuse placeholder, refuse brief outside root). PASS.
- Row 3 — printed `exit=1`, expect `exit=2`. **FAIL as written — stale-shaped, not a launcher
  defect.** The row's premise is "the committed placeholder digest"; human act 1 has since been
  performed (#1478 pinned a real `sha256:` digest), so the launcher correctly ACCEPTS the pin, prints
  the digest-pinned reference and the composed docker argv, and then fails only because the daemon was
  unreachable (`failed to connect to the docker API`). The fail-closed control itself was re-proven
  out of tree: a scratch root carrying the brief plus a copy of the manifest with the digest set back
  to the original `PENDING-HARVEST-…` placeholder printed `refusing to run — harness digest … is not a
  full sha256:<64 hex>` and `exit=2`; the same scratch root with the image set to `…/desk-tools:latest`
  printed `refusing to run — … pinned to latest` and `exit=2`. Re-baseline suggestion: run row 3
  against a scratch root with a placeholder pin, not the committed manifest.
- Row 4 — `check-paired-versions: OK` (single tag v1.0.24 across 10 pin lines; 10 sha256 values are
  64 lowercase hex). PASS.
- Row 5 — printed `0`. PASS.
- Row 6 — witness could-not-run: check:ci needs a network-off sandbox (Linux `unshare --net`), host is
  darwin. By hand (network on, not a witness) the grep lists the staged leg file, exit 0. The row's
  LIVE discharge (witness landing host-owned through the promoted leg) has NOT happened: the live
  windows-ci-leg workflow on main carries no `verify-in-container` job — human act 2 (promotion) is
  still owed. Native-Windows-Docker proof remains held (`if: false`). UNRUN in witness; live leg
  owed.
- Row 7 — printed `1` (no forge verb). PASS. Coverage note: `-A60` reaches 60 of the job's 79 lines;
  the whole job (to the next job header) was also scanned by hand — 0 forge-verb matches.
- Row 8 — witness **FAIL exit=2, check-definition defect**: the runner lifted the FIRST backticked span
  of the cell (`sh -c`, which is prose naming the four sites) as the command; `sh -c` alone exits 2
  (`option requires an argument`). The intended command, run by hand, prints `0`, but on merged main
  its base is self-referential (merge-base of main and HEAD is HEAD) so it is vacuous — the class
  tracked in #1657, which lists this brief. Substantive check: the four sites show 0 diff lines in
  each implementing commit (f2c158769 and d84ddab64). Substance PASS; row as authored FAIL.
- Row 9 — printed `0`. PASS.
- Row 10 — printed `0`, but every consumer is UNCHECKED ("unchanged since the merge-base"): this row
  is authored to run on the implementer's branch, and merged main has no branch diff. The implementing
  commit f2c158769 does change all four declared consumer paths (the statusgen launcher and verifyrun
  flags, the manifest harness block, the staged CI leg, the adoption doc). Exit matches; corroboration
  not dischargeable on main.
- Row 11 — printed `mutated-rc=1`; the source file was restored (worktree clean apart from this
  brief). PASS.

**Finding — the staged CI leg's fail-closed step is stale since the digest pin and would report a
false green on promotion.** In ci/staged-workflows/windows-ci-leg.yml, the step "Fail-closed control —
the wrapper REFUSES an un-digest-pinned image" (line 256) runs the wrapper against the COMMITTED pin
and treats ANY non-zero exit as "fail-closed OK — the wrapper refused the placeholder digest" (line
270). With the real digest now pinned, the wrapper no longer refuses: on a Docker runner it would pull
the image and run this brief's whole Verify table inside it, and any inner failure or could-not-run
would print the refusal message although nothing was refused. The step no longer exercises the pin
control at all, so the Context's claim that "the CI leg's fail-closed step reddens if it ever stops
refusing" no longer holds. Same root as row 3 (placeholder premise overtaken by act 1), but this one
sits in a deliverable a maintainer is about to promote. It should be re-baselined (for example,
against a scratch copy of the manifest carrying a placeholder digest) BEFORE act 2's promotion.

**Risk-bearing values** (enumerated over the diffs of f2c158769, d84ddab64 and 9ebb8125a, plus the
Deliverables), ranked by irreversibility:

1. `harness.digest = sha256:600e548c0e2a574009033fc03a3763246e6bf87d5f5f1c0407a1eb2613e5a9b9` @
   plugins/assay/paired-versions.yaml:100 — selects the image every in-container witness executes;
   a witness already produced under a wrong image cannot be un-produced by a later edit.
2. `harness.image = ghcr.io/medici-finance/assay/desk-tools` @ plugins/assay/paired-versions.yaml:98 —
   registry namespace binding.
3. `pinDigestRe = ^sha256:[0-9a-f]{64}$` @ statusgen/verifyincontainer.go:104 — the single control.
4. `":latest"` refusal @ statusgen/verifyincontainer.go:125.
5. `GIT_CONFIG_VALUE_0 = /work` @ statusgen/verifyincontainer.go:200 (with `containerWorkDir = "/work"`
   @ :95, `GIT_CONFIG_COUNT=1` @ :198, `GIT_CONFIG_KEY_0=safe.directory` @ :199) — trust scope.
6. `--user` threshold `inv.uid >= 0 && inv.gid >= 0` @ statusgen/verifyincontainer.go:205.
7. `harness.tag = v1.0.24` @ plugins/assay/paired-versions.yaml:99 — label only; `ref()` (:115) never
   uses it.
8. Staged-leg supply-chain pins `actions/checkout@3d3c42e5…` (line 242), `actions/setup-go@d35c59ab…`
   (line 245), `runs-on: ubuntu-latest` (240), `if: false` hold (319) in the staged CI leg —
   reversible; the two action SHAs match the ones the live workflows already use (checkout in 17 live workflows, setup-go in the live Windows leg).
9. `inContainerEnvFileVar = "ASSAY_VERIFY_ENV_FILE"` @ :91; refusal exit `verifyrunExitCouldNot = 2`
   @ statusgen/verifyrun.go:138 (pre-existing, reused) — reversible, rank last.

- `RISK-VALUE: NAMED, NOT DERIVED — harness.digest = sha256:600e548c0e2a574009033fc03a3763246e6bf87d5f5f1c0407a1eb2613e5a9b9 @ plugins/assay/paired-versions.yaml:100 — the forge's container package-versions record shows this digest carries the tags v1.0.24, latest and sha-ded08e5 (one read of the forge's package metadata, so the pin was not invented). Not derived: that the image at this digest is the right witness runner. It was built by a manual publish dispatch from main at ded08e5a4, which is 12 first-parent commits BEFORE the v1.0.24 release tag commit (a9fa171e9). So the image labelled v1.0.24 is not built from the v1.0.24 source: its Dockerfile has bash but not the file utility that main now installs (#1491 blocker 2). The registry manifest itself was not read and the image was not pulled (offline envelope). OPEN QUESTION FOR THE HUMAN: accept an image built from ded08e5a4 and labelled v1.0.24 as the pinned witness runner, or re-publish from the release tag and re-pin?`
- `RISK-VALUE: DERIVED — harness.image = ghcr.io/medici-finance/assay/desk-tools @ plugins/assay/paired-versions.yaml:98 — equals the IMAGE the publish workflow pushes (ghcr.io/<repository>/desk-tools, docker-publish.yml line 57) and the combined image docs/docker.md names; no separate harness image exists.`
- `RISK-VALUE: DERIVED — pinDigestRe = ^sha256:[0-9a-f]{64}$ @ statusgen/verifyincontainer.go:104 — a sha256 digest is 32 bytes = 64 hex characters, and the OCI image-spec requires the sha256 encoded portion to be lowercase hex; so the regex accepts exactly a well-formed sha256 content address and refuses a placeholder, truncated or upper-cased value (re-proven: placeholder refused exit 2 out of tree; mutation row 11 shows the check is load-bearing).`
- `RISK-VALUE: DERIVED — ":latest" refusal @ statusgen/verifyincontainer.go:125 — defence in depth only: ref() (:115) always appends @digest, and docker resolves image@digest by digest. Observation: only harness.image is checked; harness.tag: latest would not be refused, which is harmless because the tag never enters the reference.`
- `RISK-VALUE: DERIVED — GIT_CONFIG_VALUE_0 = /work @ statusgen/verifyincontainer.go:200 — equals the bind-mount target containerWorkDir (:95), and git's GIT_CONFIG_COUNT / GIT_CONFIG_KEY_n / GIT_CONFIG_VALUE_n mechanism scopes safe.directory to that one tree for that one run, never *.`
- `RISK-VALUE: DERIVED — --user threshold uid >= 0 && gid >= 0 @ statusgen/verifyincontainer.go:205 — Go's os.Getuid/os.Getgid return -1 on Windows and a real id (0 included) on POSIX, so the threshold omits --user exactly where no POSIX id exists (TestVICComposeWin covers the -1 branch).`

Open questions carried to the human gate (the item stays `implemented`):
(a) the harness.digest provenance above; (b) act 2, landing the staged leg — ruled "hand-land now" on
#1433 but not yet performed (the live workflow lacks the job); (c) the stale fail-closed CI step,
which should be re-baselined before (b); (d) rows 3, 8 and 10 need re-baselining for merged main
(row 8's class is tracked in #1657).

VERIFY: FAIL — rows 3 and 8 fail as written, and neither is a launcher defect: both are check-definition /
stale-shaped. Row 6's live discharge is owed. One real latent defect was found in a deliverable: the
staged CI leg's fail-closed step no longer tests the pin control. The launcher mechanism itself checked
clean (rows 1, 2, 4, 5, 7, 9, 11, plus out-of-tree placeholder and latest refusals).

## Review
Gate: **human** (from frontmatter, risk-derived: `irreversible: yes` — the leg adds a job under a
`.github/workflows/` path only a workflow-scoped credential can promote); regulatory / customer /
sensitive-data are `no` (offline read-then-witness, credential-by-path). The reviewer confirms: the
launcher runs ONLY a digest-pinned image and refuses fail-closed on `latest`/placeholder (rows 2-3);
the manifest invariants hold (row 4); the CI leg is offline and additive (rows 6-7); the four `sh -c`
sites are untouched (row 8); and the credential passthrough is a PATH the launcher never
dereferences. The `## Human decision` names the two human acts (digest harvest, landing mechanism)
and the desk-supervision/12 tension rather than assuming approval. The authoritative witness-lands
evidence is a green promoted-leg run once the real digest is pinned; the native-Windows-Docker row is
held BLOCKED with its reason, never inferred from the Linux run.
