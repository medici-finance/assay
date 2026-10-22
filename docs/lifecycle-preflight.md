# Prospective lifecycle admission

An implementation PR can leave its brief's generated row at `todo`. Ordinary
lint validates that stored row; it does not prove that recording the delivered
implementation would pass the lifecycle rules. The hourly reconciliation then
finds the merge but holds `implemented` because the design or decision metadata
is missing. Public issue [4132](https://github.com/medici-finance/assay/issues/4132)
tracks this gap.

`statusgen lifecycle-preflight` evaluates the proposed transition without editing
the board. It owns the lifecycle policy; `deskflip` and the review kit consume its
result rather than implementing another design or decision gate. The checker
uses the authoritative PR declaration and complete file listing, binds the head,
and applies the same authoring and partial-delivery classifications as
reconciliation. A held transition and an unreadable fact both prevent admission.
This does not approve a design, verify a delivery, or claim a release is qualified.

For a delivering `Brief:` PR, `deskflip` requires literal success from the
`lifecycle-preflight` check at the current head. A skipped or neutral result is
insufficient. Authors-only and issue-only PRs keep their existing classification;
a partial-delivery declaration still goes through statusgen's classification.
The review kit requires reviewers to inspect that result against the actual PR.

## Activation is a separate maintainer act

The source implementation and the staged workflow are reviewable on an ordinary
worker draft PR. The worker App cannot push live workflow files or change branch
protection. Until the workflow is promoted and required, the repository has no
live merge protection for this gap. Until consumers install the released desk
binary and review kit, their desks retain their previous behavior.

Roll out in this order:

1. Merge the reviewed checker, deskflip, review kit and staged workflow source.
2. Promote the workflow with a maintainer credential using the runsheet below.
3. Observe the normal PR run at its exact head. Require the `lifecycle-preflight`
   check in the main branch ruleset, retaining every existing required check.
   Select GitHub Actions as the expected check source where available. Do not
   replace a ruleset using an incomplete copied settings payload.
4. Release and install the desk-tools and plugin changes through the existing
   release/pin process. Do not deploy a deskflip that requires this check before
   that repository can emit it.

The staged job runs on all PR paths and on body/base edits. It builds the checker
from the trusted base and reads the PR merge tree as data, following the existing
corroboration guard's isolation model. Its credential has contents, PR and issue
read access only. It fails if the base binary does not implement the command;
there is no bootstrap success fallback. The event head and base are checked,
and a mid-run base retarget fails. The workflow file itself remains subject to
human review; it is not a server-side guarantee against a maintainer changing CI.

## Maintainer promotion runsheet

Run from a human-owned checkout with its normal permitted Git identity, after
the source PR merges. Do not dispatch a workflow. These commands create an
isolated branch and copy exactly one reviewed staged workflow. They do not push
main, merge, or change rulesets.

```sh
set -eu
git fetch origin main
LIFECYCLE_MAIN=$(git rev-parse refs/remotes/origin/main)
LIFECYCLE_TREE=$(mktemp -d "${TMPDIR:-/tmp}/lifecycle-workflow.XXXXXX")
git worktree add -b codex/activate-lifecycle-preflight "$LIFECYCLE_TREE" "$LIFECYCLE_MAIN"
git worktree lock "$LIFECYCLE_TREE"
cd "$LIFECYCLE_TREE"
test -f statusgen/lifecyclepreflight.go
test ! -e .github/workflows/lifecycle-preflight.yml
cp ci/staged-workflows/lifecycle-preflight.yml .github/workflows/lifecycle-preflight.yml
python3 - <<'PY'
import json
from pathlib import Path
p = Path('ci/staged-workflows/declared-changes.json')
data = json.loads(p.read_text())
assert data['lifecycle-preflight.yml']['mode'] == 'pending'
data['lifecycle-preflight.yml'] = {'mode': 'identical'}
p.write_text(json.dumps(data, indent=2) + '\n')
PY
git diff --check
cmp ci/staged-workflows/lifecycle-preflight.yml .github/workflows/lifecycle-preflight.yml
git diff -- ci/staged-workflows/declared-changes.json
cat .github/workflows/lifecycle-preflight.yml
```

After reviewing that exact workflow and diff, commit only these paths and push
the feature branch under the human credential:

```sh
git add .github/workflows/lifecycle-preflight.yml ci/staged-workflows/declared-changes.json
git commit -m 'ci: activate prospective lifecycle preflight'
git push origin HEAD:refs/heads/codex/activate-lifecycle-preflight
```

Open a draft PR from that branch through the normal human PR flow. Inspect its
`lifecycle-preflight` result before adding that required check to main's ruleset.
After the promotion PR merges, the staged and live twins must stay byte-identical.
The source workflow tests execute held/unreadable cases and reject a wrong event
head. Statusgen's tests separately exercise the actual lifecycle policy.

## Existing reconciliation holds

The prospective check prevents newly admitted deliveries from adding this
backlog; it does not rewrite existing rows. For each held row, locate a genuinely
approved record and repair its missing reference, obtain the missing human
decision, or correct an authoring/partial-delivery witness. A merged PR alone
never supplies approval. Keep independent verification and done closure separate.
