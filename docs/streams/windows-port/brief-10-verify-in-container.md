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

Default if no answer: the leg stays staged and the digest stays a placeholder — the launcher refuses
(fail-closed), so nothing runs an un-pinned image in the meantime. Status stays `implemented`.

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
