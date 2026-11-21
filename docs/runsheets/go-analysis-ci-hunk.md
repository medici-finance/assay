# Apply the Go lint CI job

This is the human-applied workflow leg of desk-tools/34. The worker supplies code,
fixtures and this exact hunk; a runsheet alone does not deploy CI. Apply after its
implementation PR merges (see below). Existing CI jobs and
permissions remain in place. The runner needs Python 3.12 or newer for checked archive
extraction; the job checks this before downloading tools. An older runner image needs
an explicit image update, not an unchecked extraction fallback.

## Trust and report delivery

The reusable composite action at `actions/go-analysis` owns tool acquisition,
trusted harness execution, the ratchet, job summary and sanitized artifact
publication. The workflow below only checks out the target and invokes that action
at the human-reviewed immutable SHA. This matches the subdirectory-action approach
being developed for Muhar in #2642; no new repository or Marketplace listing is needed.

The action takes its executable code, analyzer config and pins from its own pinned
ref, never from the PR. It reads the baseline from a separately checked-out main
revision. The initial bootstrap is pinned to the reviewed implementation commit;
after main has a baseline, the action automatically uses that instead. Tool caches
live outside the analyzed checkout. Failed scans still publish reports, while
missing reports fail explicitly. See [the action contract](../../actions/go-analysis/README.md).

Desk-tools/35 extends this action with the security scan and feeds its report into
qualgen. It must also deliver the explicit reviewed contract-migration lane:
this first action refuses changed tool/config/fingerprint contracts against the
old baseline, and bootstrap cannot be reused as an upgrade bypass. This first
hunk does not claim that ingestion or that migration lane already exists.

## When to apply: after merge

This branch re-measures the baseline under the current pins and moves the fingerprint
to `source-window/v3`. A maintainer ruled that this change is the reviewed migration:
https://github.com/medici-finance/assay/issues/7370#issuecomment-7066967298
Against main's baseline, the action at this branch's head refuses with
`tool/config migration requires reviewed baseline`. The refusal is by design and must
not be loosened. Apply the hunk after the implementation PR merges, on a new branch cut
from merged main. The pull request carrying that workflow change is the first one after
merge, and its run is where the gate is measured.

## Prepare in an isolated checkout

Set `REVIEWED_HEAD` to the full main commit you reviewed, at or after the merge of
the implementation PR. Run with your own permitted Git identity, from a checkout of
this repository. The ancestry check ties the pin to a commit that is on main and
already carries the migrated baseline.

```sh
: "${REVIEWED_HEAD:?Set the full implementation commit you reviewed}"
case "$REVIEWED_HEAD" in *[!0-9a-f]*|'') exit 1 ;; esac
test "${#REVIEWED_HEAD}" -eq 40
git fetch origin main
git merge-base --is-ancestor "$REVIEWED_HEAD" refs/remotes/origin/main
git show "$REVIEWED_HEAD:tools/go-analysis/runner.py" | grep -q "FINGERPRINT = 'source-window/v3'"
CI_REVIEW_TREE=$(mktemp -d "${TMPDIR:-/tmp}/go-lint-workflow.XXXXXX")
git worktree add --detach "$CI_REVIEW_TREE" "$REVIEWED_HEAD"
cd "$CI_REVIEW_TREE"
export REVIEWED_HEAD
python3 - <<'PY'
import os
from pathlib import Path
p = Path('.github/workflows/ci.yml')
current = p.read_text()
if '\n  go-lint:\n' in current:
    raise SystemExit('go-lint already exists; review the existing job instead of appending')
doc = Path('docs/runsheets/go-analysis-ci-hunk.md').read_text()
start = '<!-- go-lint-job:begin -->\n```yaml\n'
end = '\n```\n<!-- go-lint-job:end -->'
job = doc.split(start, 1)[1].split(end, 1)[0]
job = job.replace('BOOTSTRAP_IMPLEMENTATION_SHA', os.environ['REVIEWED_HEAD'])
p.write_text(current.rstrip() + '\n\n' + job + '\n')
PY
git diff --check
git diff -- .github/workflows/ci.yml
```

Inspect the diff and the bootstrap SHA, then commit and push that workflow-only
change using the human account's configured identity and credentials:

```sh
test "$(git diff --name-only)" = '.github/workflows/ci.yml'
git add .github/workflows/ci.yml
git commit -m 'ci: enable ratcheted Go lint reporting'
git push origin HEAD:refs/heads/ci/go-lint-enable
```

Open a pull request from `ci/go-lint-enable` to main. Do not dispatch a workflow.
Wait for the normal pull request run, record its exact head and `go-lint` job logs,
then repeat the proof on main after that pull request merges. Check the workflow
commit's history and forge attribution as brief 34's Verify table requires. If acquisition or analysis fails, retain the failure and fix its cause;
do not add a success fallback or bypass the ratchet.

## Exact appended job

The extraction command replaces both uses of `BOOTSTRAP_IMPLEMENTATION_SHA` with
the reviewed commit. The action and its internals are pinned; callers do not copy
the runner logic into their workflows.

<!-- go-lint-job:begin -->
```yaml
  go-lint:
    runs-on: medici-builder-public
    timeout-minutes: 45
    permissions:
      contents: read
    steps:
      - name: Check out the analyzed commit
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          ref: ${{ github.event.pull_request.head.sha || github.sha }}
          fetch-depth: 0
          persist-credentials: false
      - name: Run pinned ratcheted Go analysis
        uses: medici-finance/assay/actions/go-analysis@BOOTSTRAP_IMPLEMENTATION_SHA
        with:
          root: .
          base-ref: main
          bootstrap-revision: BOOTSTRAP_IMPLEMENTATION_SHA
          allow-downloads: 'true'
```
<!-- go-lint-job:end -->
