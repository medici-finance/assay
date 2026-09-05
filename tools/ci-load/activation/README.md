# Activation — per-push CI fan-out

The five `*.yml` files in this directory are **post-change copies** of workflows that
already live in `.github/workflows/`. They are **staged, not active**: GitHub only runs
workflows from `.github/workflows/`, so nothing here executes until a human copies it into
place.

They are staged rather than landed because the identity that authored the change does not
hold the `workflows` permission — it cannot write `.github/workflows/*` at all. That is a
deliberate scope boundary, not an oversight, so the files are delivered where they can be
reviewed in the same diff as the brief that specifies them.

`ci-load.diff` in this directory is the unified diff of all five against the versions
currently in `.github/workflows/`, for reading. It is a reading aid, not the artifact to
apply — apply the whole files.

## What the change is

The pool that runs this repository's self-hosted jobs is two runners wide. Every push to a
pull-request branch fanned out into roughly a dozen jobs on it, several of which could not
have changed their verdict for the change that triggered them. These edits cut the fan-out
by selecting triggers more precisely. They change **which** workflows run, never **what** a
workflow checks: not one assertion, guard, gate or step is altered in any of the five files.

| File | Change |
|---|---|
| `ci.yml` | `push:` scoped to `branches: [main]` (it fired on every branch and tag, duplicating the `pull_request` leg 1:1); `paths-ignore` on both legs for `docs/**`, `changelog/**`, `CHANGELOG.md`, `STATUS.md`; a `concurrency` group added (it had none), cancelling superseded **pull-request** runs only |
| `plugin-drift.yml` | `paths-ignore` on the `pull_request` leg only, same four paths. The `push: main` leg keeps **no** filter — that is the leg the file's own `WHY NO paths: FILTER` note is defending, and it is untouched |
| `assay-statusgen.yml` | `cancel-in-progress` goes from the constant `false` to `${{ github.event_name == 'pull_request' }}`: the `main` regen job (STATUS.md's single writer) stays uncancellable; superseded PR **lint** runs are cancelled |
| `assay-qualgen.yml` | the same conditional-cancel change: the `main` regen job (QUALITY.md's single writer) stays uncancellable; superseded PR **render** runs — which discard their output by design — are cancelled |
| `evidence-automerge.yml` | a job-level `if:` on the pull-request author, in front of the unchanged in-job guard step, so the job no longer starts on pull requests the guard would decline anyway |

## The rule these edits were held to

**No security or leak workflow gains a filter of any kind.** `leaksweep-control.yml` and
`leaksweep-pattern.yml` keep `paths: "**"` on both legs, keep their runner, keep their
existing `concurrency` groups byte-for-byte, and are not in this directory at all. They read
a projection of the whole tree, so a docs-only diff is exactly the change a narrower trigger
would let through in silence. The check that this held is a diff, not a grep:
`git diff --stat <before>..<after> -- .github/workflows/leaksweep-*.yml` must be empty.

**The required check is untouched.** The only status check this repository's branch rulesets
require on the default branch is `leak-sweep`, posted by a gate that is not one of these
workflows and is not reachable from this directory. It runs on every pull request
unconditionally, before and after this change.

**Every writer keeps `cancel-in-progress: false` on `main`.** Both conditional-cancel edits
are conditioned on `github.event_name == 'pull_request'` for that reason; a cancelled regen
would throw away a push's board or view rather than merely delaying it.

## Landing it — a human act

```sh
cp tools/ci-load/activation/ci.yml                 .github/workflows/ci.yml
cp tools/ci-load/activation/plugin-drift.yml       .github/workflows/plugin-drift.yml
cp tools/ci-load/activation/assay-statusgen.yml    .github/workflows/assay-statusgen.yml
cp tools/ci-load/activation/assay-qualgen.yml      .github/workflows/assay-qualgen.yml
cp tools/ci-load/activation/evidence-automerge.yml .github/workflows/evidence-automerge.yml
git add .github/workflows/
git commit -m "ci: cut per-push fan-out on the self-hosted pool"
```

Copy them **verbatim**. Each file carries its rationale in a header comment written against
this repository's existing CI conventions — the `medici-builder-public` runner label, the
pinned `actions/checkout` sha, and the by-hand Go install from `go.dev/dl`. None of those is
changed by these edits.

The five are independent: any subset can be landed on its own, and any one can be reverted
on its own. `evidence-automerge.yml` is the most separable — it is a single `if:` line plus
its comment, and it is the only edit here that is not a `paths`/`concurrency` change.

## Verifying it after the copy

```sh
# 1. All five still parse, and the `on:` blocks are what was intended.
for f in ci plugin-drift assay-statusgen assay-qualgen evidence-automerge; do
  python3 -c "import yaml,sys; d=yaml.safe_load(open('.github/workflows/$f.yml')); \
    print('$f', d.get(True) or d.get('on'), d.get('concurrency'))"
done

# 2. No security or leak workflow changed AT ALL.
git diff --stat <sha-before-landing>..HEAD -- \
  .github/workflows/leaksweep-control.yml .github/workflows/leaksweep-pattern.yml
#    Expect: no output.

# 3. Every cancel-in-progress on a main-push WRITER is still false, literally or
#    conditioned on the event. Exactly two become conditional (assay-statusgen,
#    assay-qualgen); nothing else changes.
grep -n 'cancel-in-progress' .github/workflows/*.yml

# 4. The paths-ignore semantics the whole filter argument rests on, as a negative
#    control — three of its five cases must NOT skip.
python3 tools/ci-load/pathsemantics.py
```

Then, on the next pull request that changes only files under `docs/` (an Evidence pull
request is the standard case), confirm the fan-out with:

```sh
gh run list --repo medici-finance/assay --branch "<that branch>" \
  --json workflowName,event --jq '[.[]|"\(.workflowName)/\(.event)"]|sort|unique'
```

`ci/push`, `ci/pull_request`, `plugin-drift/pull_request` and
`evidence-automerge/pull_request` should be absent for a non-verifier author, and the
`leak-sweep` status must still be present on the pull request.

## Rollback

```sh
git revert <the landing commit>
```

Or restore any single file from before the landing commit:

```sh
git checkout <sha-before-landing> -- .github/workflows/<file>.yml
```

Nothing here is stateful and nothing here writes anything, so a revert is complete the
moment it merges: the next push fans out exactly as it did before. There is no cache to
clear, no pin to unwind, and no artifact whose contents depend on which version ran.

## What was deliberately NOT done

Two further reductions were identified and left out, because neither could be proven safe
from the evidence available at authoring time. They are recorded here so the next reader does
not have to rediscover them.

1. **Moving pure-lint jobs to `ubuntu-latest`.** `changelog-check.yml` (git + `python3` +
   `gh`, no secrets, no house network) is the obvious candidate, worth one self-hosted job on
   every pull-request push; the `statusgen-board` lint job is the next. But there is **no
   observed evidence that a GitHub-hosted job has ever executed in this repository**: the one
   workflow that requests `ubuntu-latest` (`inbound-triage.yml`) was skipped at its own `if:`
   gate in 60 of its last 60 runs, so its runner was never allocated. Hosted-runner
   availability here is **could-not-check**, not "available" — and a runner move made on that
   assumption converts a green gate into an unexplained red. The prerequisite is a one-job
   probe that actually executes on `ubuntu-latest` in this repository.

2. **A `paths:` filter on `verify-gate-open.yml`.** It runs on every push to `main` with no
   filter, and its input is `statusgen --root . --verify-issues` over `docs/streams/**` plus
   the existing issue markers — so a `main` push touching only Go code cannot make a brief
   newly eligible. The filter is very likely correct, but this is the lane that opens the
   human sign-off issue for a verified brief, and a missed issue is a brief that never
   reaches its gate. That belongs to whoever owns the verify lane, not to a CI-load change.
