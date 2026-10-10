# Board-writer Environment migration

This proposal uses the existing board-writer App with the `board-writer`
Environment, whose allowed deployment ref must be exactly **Branch `main`**.
The Environment must contain `BOARD_APP_ID` and `BOARD_APP_PRIVATE_KEY`.

The App that authors this PR cannot push `.github/workflows/**`. The live
workflow changes are therefore reviewable in `workflows.patch`; until a
maintainer applies and pushes that patch to this PR branch, the live workflows
remain unchanged. Do not remove the repository secrets before that promotion.

The patch modifies only these five live workflows:

| Workflow | Jobs using the Environment |
|---|---|
| `assay-statusgen.yml` | `regen`, `model-autoflip` |
| `assay-qualgen.yml` | `regen` |
| `verify-gate-close.yml` | `close` |
| `release.yml` | `changelog-roll`, restricted to a non-dry-run dispatch on `main` |
| `evidence-automerge.yml` | `enable`, after read-only discovery |

Evidence auto-merge now polls at minutes 7, 22, 37 and 52 each hour. GitHub may
delay scheduled runs. A manual dispatch on `main` is also available; other refs
skip both jobs. Discovery reads every open-PR page with the read-only default
token. Each ready verifier PR targeting `main` gets its own guarded matrix job.
That job re-reads current metadata, preserves the existing shared author/file
guard and draft check, and mints the App token only after eligibility passes.
It checks out only the decision-script directory at the exact `main` run SHA,
with no persisted credential. The existing auto-merge mutation and refusal
classifier remain unchanged.

The staged release and statusgen copies retain their earlier pending proposals;
the patch deliberately excludes those proposals. Staged and activation copies
also receive the Environment restriction so a future promotion cannot restore
an unfenced credential consumer. The staged reconcile proposal is fenced too;
this migration does not activate it or change the App's permissions.

## Add the live changes to this PR

Run as a maintainer in a checkout of the PR branch:

```sh
git apply --check ci/board-writer-migration/workflows.patch
git apply ci/board-writer-migration/workflows.patch
git add .github/workflows/assay-statusgen.yml .github/workflows/assay-qualgen.yml .github/workflows/verify-gate-close.yml .github/workflows/release.yml .github/workflows/evidence-automerge.yml
git commit -m "ci: activate board-writer Environment migration"
git -c remote.origin.pushurl=https://github.com/medici-finance/assay.git push origin HEAD:codex/board-writer-environment
```

If the patch no longer applies, stop and refresh it against the current live
files. Do not copy the staged release/statusgen proposals wholesale: that would
activate other work in addition to this migration. A merged staged-only PR does
not complete the migration.

## Offline checks

Before applying the patch:

```sh
python3 tools/evidence-automerge/automerge-poll_test.py
bash tools/evidence-automerge/automerge-refusal_test.sh
bash tools/evidence-automerge/automerge-step_test.sh
git apply --check ci/board-writer-migration/workflows.patch
```

The poll test applies the patch in a disposable directory and checks every
live key consumer, the main-only triggers and checkout, pagination, refreshed
eligibility, unreadable metadata/files and the unchanged shared Evidence guard.
The refusal and step tests retain their fail-first controls.

After promotion and merge, check legitimate main-triggered writer runs and a
scheduled Evidence poll. Once those succeed with the Environment credentials,
remove the repository-level secrets and retire the old App key, after checking
other users of that key. Environment protection trusts merged `main`; it does
not isolate self-hosted runners or restrict the App to particular file paths.
