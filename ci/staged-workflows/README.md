# Staged workflows — pending human promotion

CI workflow files under `.github/workflows/` are added and changed by a **maintainer**, not
by automation: no bot or App in this project holds workflow-push permission, and GitHub
hard-rejects any App push that creates or updates a `.github/workflows/*` file. That is a
deliberate boundary — an identity that can rewrite CI is a supply-chain surface — so a
prepared workflow lands here first, in the ordinary pull-request flow, and a maintainer
**promotes** it into place.

## Promoting a staged workflow

```
git mv ci/staged-workflows/<name>.yml .github/workflows/<name>.yml
git commit -m "ci: activate <name> workflow"
git push
```

The push that lands the file under `.github/workflows/` must come from a maintainer's
credential (or a narrowly-scoped promote credential), which is what makes activation a
human's act. Until a file here is promoted it runs nowhere; the staged copy is the
reviewable artifact, not a run.

## Contents

- `evidence-automerge.yml` — the Evidence-PR auto-merge lane. Unlike the other files here
  this one is **already live**: it was promoted, and the copy in this directory is kept
  byte-identical to `.github/workflows/evidence-automerge.yml` as the reviewable edit
  surface, because no App may push a `.github/workflows/*` file. Every change to that lane
  therefore lands here first, and a maintainer re-promotes it by copying the file over the
  live one (`cp ci/staged-workflows/evidence-automerge.yml .github/workflows/evidence-automerge.yml`)
  in a separate maintainer-credentialled commit. Until that copy lands, the merged change
  is inert: the running workflow is still the old file.
- `truth-suite.yml` — the standing truth suite (`docs/test-policy.md` § "Standing truth
  suite"): the test corpus plus the release mutation gate, on push to the default branch and
  on a daily schedule, reporting three-state. **Already live** — promoted to
  `.github/workflows/truth-suite.yml` on 2026-09-10; this directory no longer carries a copy,
  so a change to it is authored here first and re-promoted by a maintainer commit.
- `winparity.yml` — the Windows-build ↔ Makefile target-parity gate (#665). Runs
  `cd tools/winparity && go run . --root ../..` on the self-hosted `medici-builder-public` runner
  (hand-installed Go, no `make`), asserting that `scripts/build-windows.ps1`'s declared target set
  equals the root `Makefile`'s `.PHONY` set — so a target added on one side and not the other
  reddens rather than shipping a Windows build that fell behind the Unix target set. The Windows
  script runs the same `tools/winparity` guard as a preflight; this leg is the Linux-CI half, so a
  Makefile-only edit (the change that never runs the Windows script) is still caught. Promote it to
  `.github/workflows/winparity.yml` to activate.
- `windows-ci-leg.yml` — the Windows CI leg (`windows-port/04`): the first check in this repo
  to run on a Windows runner. On GitHub-hosted `windows-latest` it installs Go
  (`actions/setup-go`, which works there — unlike the self-hosted Linux pool), builds
  `statusgen.exe` from source, asserts `statusgen.exe --root .. --lint` exits 0, and runs an
  OFFLINE desk-verb smoke (`statusgen.exe --version` — pure introspection, no forge/network, no
  POSIX shell-out). It runs NO mutating/forge verb. The native windows/arm64 smoke is held
  BLOCKED (`arm64-native-smoke`, `if: false`) pending a `windows-11-arm` runner and is never
  inferred from the amd64 result. A `workflow_dispatch` input `failfirst=true` runs the
  fail-first demonstration (the leg must redden on a bogus verb). The staged copy also adds a
  **Windows PowerShell 5.1 parse check** to `windows-smoke` (#1569, pending promotion): under
  `shell: powershell` it runs `[System.Management.Automation.Language.Parser]::ParseFile` over
  every tracked `*.ps1` (case-insensitive pathspec) and fails on any parse error, with a
  `failfirst` twin that plants a BOM-less em dash and a PowerShell 7 `? :` ternary and
  requires 5.1 to reject each. The PR-time half of the ENCODING class needs
  no promotion: `TestPS1EncodingIs51Safe` in `tools/desk/internal/deskkit`
  (run by `ci.yml`'s build-test job) fails any `*.ps1` carrying a byte above 0x7F without a
  UTF-8 BOM. It is a byte scan, not a parser: any other 5.1-only parse error (a PowerShell 7
  operator such as the `? :` ternary, for example) is caught only by this staged step, so
  until it is promoted nothing gates that class.

  **Already live, like `evidence-automerge.yml` above** — this copy is kept as the reviewable
  edit surface for `.github/workflows/windows-ci-leg.yml`, no App may push a workflow-file
  change. The `windows-bootstrap-smoke` job resolves the pinned `windows-amd64` tag **and** its
  sha256 from `plugins/assay/paired-versions.yaml` and hands both to
  `scripts/windows-bootstrap-hashcheck-smoke.ps1` (`-Tag`/`-RealSha256`), exercising the sha256
  hash-verify at Windows runtime. **This staged copy has been re-based onto the live file so a
  promotion is a byte-for-byte copy that only ADDS** — the live file's later changes (the
  version-tag trigger and the lint job's `fetch-depth: 0`) are already present here, so promoting
  no longer reverts them (the #1187-class drift the caveat below warns about). **Pending
  promotion** — a maintainer re-promotes by copying this file over the live one:
  ```
  cp ci/staged-workflows/windows-ci-leg.yml .github/workflows/windows-ci-leg.yml
  git commit -m "ci: promote windows-ci-leg.yml"
  git push
  ```

- `release-on-merge.yml` — the "release by merge" merge-detector (iso-9001/07). **New,
  pending promotion.** On `push: branches: [main]` it detects that the pushed commit is the
  merge of a release PR (title `release: vX.Y.Z` + a `RELEASE: vX.Y.Z` body line, merged with
  `merge_commit_sha` == the pushed commit), mints a release-cutter App token, and creates
  `refs/tags/assay/vX.Y.Z` then `refs/tags/vX.Y.Z` at the merge commit via the git-data API. The
  plain `vX.Y.Z` tag — created by an App installation token, a distinct identity — triggers
  `release.yml`'s `push: tags: ['v*']` build; a GITHUB_TOKEN-created tag would NOT (GitHub's
  recursion guard). This job's own GITHUB_TOKEN stays read-only. **Prerequisite (repo-admin
  act):** a release-cutter GitHub App (contents:write; no workflows/actions/administration
  write) with its key wired as the Actions secrets `RELEASE_APP_ID` / `RELEASE_APP_PRIVATE_KEY`
  (a human may instead point these at the existing board-writer App's `BOARD_APP_*`). Promote it:
  ```
  git mv ci/staged-workflows/release-on-merge.yml .github/workflows/release-on-merge.yml
  git commit -m "ci: activate release-on-merge.yml"
  git push
  ```
- `release.yml` — a STAGED TWIN of the live `.github/workflows/release.yml`, carrying ONLY the
  release-by-merge addition to the `resolve` job's tag-push path (iso-9001/07): it resolves the
  authorizer from the merged release PR's `merged_by.login` and REFUSES to build a release for
  any `v*` tag whose commit is not a merged release PR (title `release: vX.Y.Z` + `RELEASE:
  vX.Y.Z`). Additive; no step reordered. Like `evidence-automerge.yml` above, no App may push a
  `.github/workflows/*` change, so this is the reviewable edit surface; a maintainer re-promotes
  by copying it over the live file in a maintainer-credentialled commit:
  ```
  cp ci/staged-workflows/release.yml .github/workflows/release.yml
  git commit -m "ci: promote release.yml (release-by-merge tag-push authorizer)"
  git push
  ```
  **Landing-mechanism caveat** applies exactly as for `windows-ci-leg.yml` below (see
  `docs/streams/decisions/DR-workflow-app-landing.md` and desk-supervision/11-12): if the
  workflow-App PR path has landed, prefer it over a verbatim hand-copy. Re-base this twin on the
  live file at promotion time (three-way) so promoting only ADDS.

### `windows-ci-leg.yml` status

Already activated (`windows-port/04`) — see "Already live" above for how a later change to
this file is promoted now that it exists at `.github/workflows/windows-ci-leg.yml`.

**`windows-port/10` adds two jobs to this file** (pending re-promotion by a maintainer copying
this staged copy over the live one, exactly as `windows-port/06` was):
- `verify-in-container` — the execution-witness runner leg. On `ubuntu-latest` it builds
  `statusgen`, asserts the wrapper REFUSES an un-digest-pinned harness image (the fail-closed pin
  control, green on promotion), and — once a maintainer harvests and pins the harness image's real
  registry digest — runs a fixture Verify table THROUGH the pinned container and asserts the
  witness lands in Evidence, host-owned. It runs on `ubuntu-latest` because the harness image is a
  Linux image whichever host launches it; the Windows value is that Windows adopters USE the
  container because a native pipefail `bash` is unreliable there (#1418).
- `windows-verify-in-container` — the native-Windows-host proof, HELD BLOCKED (`if: false`):
  `windows-latest` has no Linux-container Docker backend to run the Linux harness image, so this
  awaits a Windows runner configured with one. Never inferred from the ubuntu result — the same
  "blocked is a state" contract `arm64-native-smoke` uses.

**Landing-mechanism caveat — read before promoting.** This staged CI-leg addition rides the same
staged-copy → maintainer-hand-copy pattern `windows-port/04`/`/06` used. That pattern is the
subject of an OPEN, unratified decision record (`docs/streams/decisions/DR-workflow-app-landing.md`)
and an in-flight retirement brief (`docs/streams/desk-supervision/brief-12-...`), which cite real
drift (a staged copy authored against one base silently reverts intervening fixes when the live
file moves — #1187) and indefinite stalls (a hand-copy step sitting `BLOCKED-ON-HUMAN` 9+ days —
#1175/#1185). Whoever promotes this addition should check `desk-supervision/12`'s current state
first: if the workflow-App PR path has landed, this change should travel through THAT path (a
single workflow-only PR the workflow App authors) rather than a verbatim hand-copy — and this
staged copy may itself be reduced to a pointer by that brief.

The leg runs only on version tags (`push` with `tags: ['v*']`, #1215) plus on-demand
`workflow_dispatch`; a green `windows-smoke` job is the
authoritative evidence for the brief's rows 2-3 (record its run URL, the `--lint` exit, and the
`--version` smoke result on the brief). The `failfirst` fail-first demo is run on demand via
`workflow_dispatch`. The `arm64-native-smoke` row stays held until a `windows-11-arm` runner is
available; flip it to that runner then — do not derive it from the amd64 leg. The offline
constraint is a design invariant: the smoke uses only `--lint`/`--version`, and any change that
adds a live-forge or mutating verb to the `windows-smoke` job breaks the offline envelope the
brief's row 4 asserts.

The file also carries a SECOND `windows-latest` job, `windows-bootstrap-smoke` — the sanctioned
ONLINE exception (windows-port/03, decision #508). It exercises `scripts/bootstrap-windows.ps1`'s
sha256 hash-verify at Windows runtime via `scripts/windows-bootstrap-hashcheck-smoke.ps1`: a
tampered checksum must REFUSE (nothing installed), the pinned checksum installs, and a
check-removed copy installs the tampered asset (so the refusal is non-vacuous). The bootstrap
downloads a release asset (`Invoke-WebRequest` to the GitHub release CDN), so this job is
deliberately kept SEPARATE from `windows-smoke` — the download is decision #508's sanctioned
live-forge exception and does not weaken the offline invariant of the `windows-smoke` job. It is
promoted with the rest of this file; a green `windows-bootstrap-smoke` run is the evidence for
the brief's row 8 (record its run URL). The job hands the pinned tag **and** sha256 to
`scripts/windows-bootstrap-hashcheck-smoke.ps1` (`-Tag`/`-RealSha256`), so the untampered path
downloads the real published asset while a tampered checksum still REFUSES.
