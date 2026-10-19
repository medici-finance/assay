# Prune sweep read-reuse regression evidence

This record addresses S1 (security review 5559691890) and F-fail-first (correctness review 5559702948) on PR #3189. All nine added tests have an observed behavioral red and a restored-source green. The production implementation and test assertions are unchanged by this evidence repair.

## Reproduce

From `tools/desk`, run each of the nine existing-harness specifications below. Each specification selects one existing test with `-count=1 -v -timeout 30s`; the exact test name and old/new source bytes are in its JSON. Every specification has a positive control. The identity specification also has two mutants, covering all six ambiguous/unreadable/changed-pointer subcases. No new runner, CLI, scanner exemption, lifecycle policy or persistent cache is introduced.

```sh
for name in reap detached common-store unreadable-head refresh scale identity native-git decode; do
  go run ./cmd/muhar -spec "cmd/deskwt/testdata/sweepread-mutations/$name.json" || exit $?
done
```

The existing harness temporarily edits and restores source. Run it only in an owned disposable checkout, with no concurrent source editor. Inspect each report: every control/mutant must be CAUGHT, with zero NOT CAUGHT and zero could-not-mutate. The harness treats any nonzero test exit as caught; therefore the observed results below additionally require the named test to fail at its behavioral assertion, rejecting build failures, timeouts and missing-test matches.

## Observed controls

Measured on the clean main merge of the reviewed `a420e4b` implementation, with only regression comments added to the tests. For each row, the exact committed edit was applied through a Go source overlay, that one test was run with the JSON command, and the same command then passed without the overlay. Original production source bytes were checked unchanged after every pair. Every red exited 1; every restored run exited 0 and named the intended test as PASS.

| Specification | Control or mutation | Observed behavioral failure | Restored |
|---|---|---|---|
| [reap](../../tools/desk/cmd/deskwt/testdata/sweepread-mutations/reap.json) | 0 | `40 candidates: mainline walks=1 non-mainline queries=20, want 1 each` | exit 0 |
| [detached](../../tools/desk/cmd/deskwt/testdata/sweepread-mutations/detached.json) | 0 | `detached candidates used different reachability indexes` | exit 0 |
| [common-store](../../tools/desk/cmd/deskwt/testdata/sweepread-mutations/common-store.json) | 0 | `a cached candidate handle survived redirection to another common store` | exit 0 |
| [unreadable-head](../../tools/desk/cmd/deskwt/testdata/sweepread-mutations/unreadable-head.json) | 0 | `missing HEAD object must be uncertainty: pushed=false err=<nil>` | exit 0 |
| [refresh](../../tools/desk/cmd/deskwt/testdata/sweepread-mutations/refresh.json) | 0 | `next sweep retained an obsolete error: cannot read the complete worktree admin registry (fixture path omitted)` | exit 0 |
| [scale](../../tools/desk/cmd/deskwt/testdata/sweepread-mutations/scale.json) | 0 | `80 candidates: registry reads=80 pointers=6480, want 1 registry and at most 160 pointers` | exit 0 |
| [identity](../../tools/desk/cmd/deskwt/testdata/sweepread-mutations/identity.json) | 0 | `uncertain admin identity was accepted` | exit 0 |
| [identity](../../tools/desk/cmd/deskwt/testdata/sweepread-mutations/identity.json) | 1 | `uncertain admin identity was accepted` | exit 0 |
| [identity](../../tools/desk/cmd/deskwt/testdata/sweepread-mutations/identity.json) | 2 | `uncertain admin identity was accepted` | exit 0 |
| [native-git](../../tools/desk/cmd/deskwt/testdata/sweepread-mutations/native-git.json) | 0 | `cached ancestry disagrees with native Git in all three graph modes (iteration order varies)` | exit 0 |
| [decode](../../tools/desk/cmd/deskwt/testdata/sweepread-mutations/decode.json) | 0 | `decoded 3320 commits for 80 distinct ancestry queries, want 1..81` | exit 0 |

The identity controls fail the duplicate, unreadable, malformed, moved, changed-malformed and removed subcases respectively. A second planted instance is therefore covered by the malformed/incomplete inventory mutation in addition to the duplicate control. Independent native Git remains the ancestry oracle; work counters measure real inventory/decoding calls rather than elapsed time.

## Instrument limits and negative result

An initial private overlay invocation placed the Go build flag before the `test` subcommand and returned usage exit 2. It was discarded, not counted as a red. After correcting the argument position, all reported controls compiled and failed behaviorally.

A proposed mutation bypassing only the mainline HEAD-object check survived the unreadable-HEAD test, because an earlier upstream ancestry read still reported uncertainty. That mutation was discarded. The committed unreadable-head control instead demonstrates the test detecting conversion of the upstream ancestry error into a non-error negative answer. No assertion or fixture was weakened to manufacture a red.

These local fixtures prove the named assertions discriminate. They do not certify a production engine, live infrastructure, independent review, or any unrun check. Full-module and cross-platform checks remain CI work. Raw private test captures are retained by the author; the refresh diagnostic above omits machine-specific fixture paths.
