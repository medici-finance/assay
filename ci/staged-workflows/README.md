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
