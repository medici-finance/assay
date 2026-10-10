# Board-writer Environment migration

This migration uses the existing board-writer App with the `board-writer`
Environment, whose allowed deployment ref must be exactly **Branch `main`**.
The Environment must contain `BOARD_APP_ID` and `BOARD_APP_PRIVATE_KEY`.

The five live workflow changes are included in this PR in a maintainer-authored
commit. `workflows.patch` records the exact migration for review and for any
checkout that still needs promotion. Do not remove the repository secrets
before merge and successful Environment-backed writer runs.

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

## Applying the migration patch

The patch is already applied on this PR branch. For a checkout where the five
workflow changes are absent, run as a maintainer:

```sh
git apply --check ci/board-writer-migration/workflows.patch
git apply ci/board-writer-migration/workflows.patch
git add .github/workflows/assay-statusgen.yml .github/workflows/assay-qualgen.yml .github/workflows/verify-gate-close.yml .github/workflows/release.yml .github/workflows/evidence-automerge.yml
git commit -m "ci: activate board-writer Environment migration"
git push https://github.com:443/medici-finance/assay HEAD:codex/board-writer-environment
```

If the patch no longer applies, first check whether it is already present with
`git apply --reverse --check ci/board-writer-migration/workflows.patch`. Otherwise,
stop and refresh it against the current live files. Do not copy the staged release/statusgen proposals wholesale: that would
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
