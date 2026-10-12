# Board reconciliation and release notes

The defect class is conflating routine delivery-state recording with a notable
change. The changelog gate's one shared engine handles both the active workflow
and its historical staging copy. The board exception is deliberately narrower
than general README changes: it recognizes the scheduled board-writer's exact
identity and branch custody, then compares immutable Git trees and erases only
recognized Status cells for comparison. This does not validate the truth of a
status, grant a merge permission, or bypass the board's other checks.

`check.sh` loads the classifier library by its exact adjacent path after the
absolute Unreleased guard. The library has no independent operator entrypoint;
the CLI inventory records it and the two offline test scripts by exact path. It uses
`GITHUB_EVENT_NAME`, `GITHUB_EVENT_PATH` and `GITHUB_REPOSITORY`, supplied by the
Actions runner, plus the same explicit `BASE_SHA` and `HEAD_SHA` the existing
workflow already passes. Neither the active `.github/workflows/changelog-check.yml`
nor the historical `tools/changelog/changelog-check.yml` needs an edit. The active
workflow's separate Evidence identity and classification stay unchanged; its
maintainer label path also stays unchanged. An absent or invalid event retains
the ordinary requirement, including local runs outside Actions.

The scheduled live/staged `assay-statusgen.yml` producers need no fragment writer,
label write or permissions change. The `deskreconcile` sibling publishes through
the worker role and therefore does not inherit the board-writer exception; its
PRs retain the ordinary fragment/maintainer-label path. An unsupported generated
table layout or broader regeneration also retains that path rather than guessing
which changes are non-notable.

Run `bash tools/changelog/reconcile_test.sh` for the real-Git classification
fixtures and the existing changelog suite. The existing unconditional build-test
job runs this through `TestChangelogReconcile`, registered as a cross-module
reader. Run `python3 tools/changelog/reconcile_mutation.py` for isolated guard
mutations, including the whole-diff/content check's planted second instance.
No test calls the network or writes the producer's branch.
