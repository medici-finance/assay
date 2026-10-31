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

`deskflip` enforces the check only where it is adopted: the base branch's
required status checks include `lifecycle-preflight`. It reads that set from the
server; a set it cannot read refuses the flip rather than reading as "not
adopted". The review kit uses the same predicate. In a repository that does not
require the check, a delivery flips as it did before this change.

Where adopted, a PR carrying `Brief:` or `Cross-repo-brief:` needs literal
success from a `lifecycle-preflight` check run created by the GitHub Actions
App at the current head. A skipped or neutral result is insufficient. A commit
status with that context and a check run from any other App are not accepted. A
PR that changes `.github/workflows/` or `.github/actions/` is refused, because
its own job could produce the run. Authors-only and issue-only PRs are exempt.
Statusgen alone decides whether a declared brief is authoring-only or a partial
delivery; deskflip requires the check for every declaration. Statusgen does not
evaluate a `Cross-repo-brief:` delivery, so it reports could-not-check for one;
that delivery's lifecycle is judged in the brief's own repository. The review
kit requires reviewers to inspect that result against the actual PR.

The checker judges the PR against its base. Run on the PR merge checkout, it
takes `HEAD^1` as the judged base after confirming `HEAD^2` is the expected
head; elsewhere `--base-rev` names it. The decision-gate base snapshot and the
repository alias registry are read at that base, so a change cannot relax its
own brief's gate or widen which markers rule on it.

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
   Select GitHub Actions as the expected check source; `deskflip` independently
   accepts only a check run created by that App. Do not replace a ruleset using
   an incomplete copied settings payload.
4. Release and install the desk-tools and plugin changes through the existing
   release/pin process. The released deskflip enforces the check only where step
   3's required check exists, so a consumer that has not provisioned the
   workflow keeps its previous flip behaviour.

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
