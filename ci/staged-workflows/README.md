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
  fail-first demonstration (the leg must redden on a bogus verb).

  **Already live, like `evidence-automerge.yml` above** — this copy is kept as the reviewable
  edit surface for `.github/workflows/windows-ci-leg.yml`, no App may push a workflow-file
  change. `windows-port/06` simplified the `windows-bootstrap-smoke` job's first step (the
  bootstrap script now resolves its own tag+sha from `plugins/assay/paired-versions.yaml`, so
  the step passes only the tag) and added a second step exercising the PATH write + bare-name
  `statusgen --version` invocation. **Pending promotion** — a maintainer re-promotes by copying
  this file over the live one:
  ```
  cp ci/staged-workflows/windows-ci-leg.yml .github/workflows/windows-ci-leg.yml
  git commit -m "ci: promote windows-ci-leg.yml (windows-port/06)"
  git push
  ```

### `windows-ci-leg.yml` status

Already activated (`windows-port/04`) — see "Already live" above for how a later change to
this file is promoted now that it exists at `.github/workflows/windows-ci-leg.yml`.

The leg runs on `push`/`pull_request`; a green `windows-smoke` job is the
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
the brief's row 8 (record its run URL). Since `windows-port/06`, the job's second step also
proves rows 6 and 14: the real bootstrap run (no operator-supplied sha) writes the user PATH,
and a fresh process invokes the installed binary by its bare `statusgen` name.
