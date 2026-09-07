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

- `truth-suite.yml` — the standing truth suite (`docs/test-policy.md` § "Standing truth
  suite"): the test corpus plus the release mutation gate, on push to the default branch and
  on a daily schedule, reporting three-state. Promote it to `.github/workflows/truth-suite.yml`
  to activate.
- `windows-ci-leg.yml` — the Windows CI leg (`windows-port/04`): the first check in this repo
  to run on a Windows runner. On GitHub-hosted `windows-latest` it installs Go
  (`actions/setup-go`, which works there — unlike the self-hosted Linux pool), builds
  `statusgen.exe` from source, asserts `statusgen.exe --root .. --lint` exits 0, and runs an
  OFFLINE desk-verb smoke (`statusgen.exe --version` — pure introspection, no forge/network, no
  POSIX shell-out). It runs NO mutating/forge verb. The native windows/arm64 smoke is held
  BLOCKED (`arm64-native-smoke`, `if: false`) pending a `windows-11-arm` runner and is never
  inferred from the amd64 result. A `workflow_dispatch` input `failfirst=true` runs the
  fail-first demonstration (the leg must redden on a bogus verb). Promote it to
  `.github/workflows/windows-ci-leg.yml` to activate.

### Activating `windows-ci-leg.yml`

```
git mv ci/staged-workflows/windows-ci-leg.yml .github/workflows/windows-ci-leg.yml
git commit -m "ci: activate windows-ci-leg workflow"
git push
```

Once promoted, the leg runs on `push`/`pull_request`; a green `windows-smoke` job is the
authoritative evidence for the brief's rows 2-3 (record its run URL, the `--lint` exit, and the
`--version` smoke result on the brief). The `failfirst` fail-first demo is run on demand via
`workflow_dispatch`. The `arm64-native-smoke` row stays held until a `windows-11-arm` runner is
available; flip it to that runner then — do not derive it from the amd64 leg. The offline
constraint is a design invariant: the smoke uses only `--lint`/`--version`, and any change that
adds a live-forge or mutating verb to this leg breaks the offline envelope the brief's row 4
asserts.
