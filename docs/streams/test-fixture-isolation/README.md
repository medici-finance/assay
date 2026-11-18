---
stream: test-fixture-isolation
repo: medici-finance/assay
serves: assay
status: active
priority: P1
track: platform
issues: [6300, 2715]
board: generated
spec: docs/streams/test-fixture-isolation/README.md
---

# Test fixtures own their environment

**Status:** draft. This adopts the test-only outcomes recorded on issue #6300 and splits implementation into eleven contracts. The maintainer's merge approves this scoping document; no earlier approval, qualification or class closure is asserted. Implementation drafts may be prepared against this explicit draft contract under the driver's request to fan out; ordinary review, merge and independent verification stay with their respective desks.

**Routes-to:** docs/streams/test-fixture-isolation/

## Intent and evidence

Issue #6300 records four incident groups: injected Git configuration (#2715), absent fixture forge credentials (#5014), installed decision binaries winning PATH resolution (#6279/#6372), and a caller global Git configuration escaping a temporary HOME (#6389). #6298 combines incidents. The controlling triage calls for a strong-tier design deliverable and states that test-only isolation needs no human ruling. This is that deliverable; it does not resolve the incidents by declaration.

Freshness checked on 2026-11-17 at `dabede22b0be3abd3320a0a7016132a917192e86`: statusgen's `fixtureGitEnv` filters child configuration while `gitCoreFor` still observes process configuration; desk regression's helper is internal to its module; maintenance fixtures inherit explicit global configuration. Existing decision-child PATH and landed-origin repairs are retained regression consumers; this plan does not claim they still reproduce on the pinned head. Source contracts, tests and production rejection behavior remain unchanged by this documentation delivery.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Shared explicit fixture environment](brief-01-shared-explicit-fixture-environment.md) | 0 | M | todo | — | — |
| 02 | [Statusgen Git fixture migration](brief-02-statusgen-git-fixture-migration.md) | 1 | M | todo | — | — |
| 03 | [Desk fixture migration](brief-03-desk-fixture-migration.md) | 1 | M | todo | — | — |
| 04 | [Hostile environment CI coverage](brief-04-hostile-environment-ci-coverage.md) | 3 | M | todo | — | — |
| 05 | [Desk command fixture siblings](brief-05-desk-command-fixture-siblings.md) | 1 | M | todo | — | — |
| 06 | [Worktree fixture siblings](brief-06-worktree-fixture-siblings.md) | 2 | M | todo | — | — |
| 07 | [Dispatch and fleet fixture siblings](brief-07-dispatch-and-fleet-fixture-siblings.md) | 1 | M | todo | — | — |
| 08 | [Statusgen remaining fixture siblings](brief-08-statusgen-remaining-fixture-siblings.md) | 2 | L | todo | — | — |
| 09 | [Nonmutating child fixture environment](brief-09-nonmutating-child-environment.md) | 1 | M | todo | — | — |
| 10 | [Compiled cellctl and shim fixture environment](brief-10-compiled-cellctl-fixture-environment.md) | 2 | M | todo | — | — |
| 11 | [Deskkit origin fixture environment](brief-11-deskkit-origin-fixture-environment.md) | 2 | M | todo | — | — |
<!-- statusgen:briefs:end -->

## Critical path

`01 -> (02, 03, 05, 07 and 09 in parallel) -> (06 after 03; 08 after 02+09; 10 and 11 after 09) -> 04`. The real head is a shared, importable test-support contract, not a release or #5749. Existing `tools/desk/internal/regression` cannot be imported by statusgen; both already depend on gitcore. `gitcore/gittest` registers a global update flag and is unsuitable for this environment-only reuse. Use a separate `gitcore/testenv` package. #5749's publication exception, PostgreSQL work, and write/execution features are outside this graph.

## Dependency waves

| Wave | Briefs | Boundary |
|---|---|---|
| 0 | 01 | Shared helper contract and hostile controls |
| 1 | 02, 03, 05, 07, 09 | Separate statusgen and desk consumers |
| 2 | 06, 08, 10, 11 | Wider worktree and statusgen fixtures after bounded consumer migrations |
| 3 | 04 | All migrations exercised by mandatory existing CI aggregate |

Draft branches may be prepared concurrently against this contract; a consumer's dependency is satisfied by the real helper commit, not by an invented API or a prose completion claim. Integrate that commit without rebasing. Source qualification and lifecycle transitions require the independent desks.

## Whole-suite sibling follow-ups

The full hostile desk worker census found additional current consumers beyond the narrow maintenance/decision slice. [05 — Desk command fixture siblings](brief-05-desk-command-fixture-siblings.md) covers deskperf, deskpushguard, deskreply and verifyloop. [06 — Worktree fixture siblings](brief-06-worktree-fixture-siblings.md) covers the wider deskwt newRepo/withEnv families after the maintenance migration. [07 — Dispatch and fleet fixture siblings](brief-07-dispatch-and-fleet-fixture-siblings.md) covers confirmed real-allocator and fleet child-environment consumers. [08 — Remaining statusgen fixtures](brief-08-statusgen-remaining-fixture-siblings.md) covers the additional history/decay/repair groups exposed by the partial full-module run. The integrated census adds [09 — child-only constructor](brief-09-nonmutating-child-environment.md), [10 — compiled cellctl/shim fixtures](brief-10-compiled-cellctl-fixture-environment.md) and [11 — deskkit origin fixtures](brief-11-deskkit-origin-fixture-environment.md). These follow-ups reuse the same helper and preserve authority controls; required CI acceptance waits for these actual consumers rather than omitting their packages. No #5749 dependency or human ruling is added.

## Shared interface contract

Package: `github.com/medici-finance/assay/gitcore/testenv`, test support only, never imported by production tools.

```go
type Options struct {
    Tools map[string]string // bare command name -> absolute approved executable
    Vars  map[string]string // explicit fixture overlays, including Go cache paths
}
type Fixture struct {
    Home string
    Bin  string
    // explicit immutable child environment, not exported
}
func Install(t testing.TB, opts Options) *Fixture
// Added by09 for compiler/shim children; unlike Install, never changes process state.
func New(t testing.TB, opts Options) *Fixture
func (f *Fixture) Env(extra ...string) []string
```

`Install` creates private HOME/config and bin directories, an empty writable global Git config, and an owned `gitquiet.TemplateDir(t)`. Its process environment and child base contain only declared entries: all undeclared ambient variables are removed, including `GIT_*`, forge credentials, shell startup inputs and caller DESK/ASSAY configuration. Explicit `Vars` carry required synthetic roster inputs and already-resolved Go caches. Its explicit base sets PATH to Bin only, sets HOME/USERPROFILE and configuration homes to owned paths, disables system Git configuration, and supplies the owned template. Tools are links to caller-approved absolute executables, with a regular-file copy where symlinks are unavailable; invalid, absent or unapproved mappings fail, with no caller-PATH fallback. Resolve essential Git/Go/shell executables before installation, preserving explicit Go cache paths before HOME changes. The installed tool map is the declared dependency set, not a general virtual filesystem.

`Vars` and `Env` extras are intentional overlays; the final value per key wins, with platform-appropriate case handling. Reject overlays for owned PATH, HOME, USERPROFILE, XDG_CONFIG_HOME and temporary-directory keys; a test intentionally exercising their rejection paths uses `t.Setenv` after installation. Explicit overlays may plant synthetic credentials or hostile Git controls. `Env` returns a fresh slice from the stored base, never `os.Environ()`. Process changes restore original presence and value on cleanup. Installation cannot run in parallel tests. Tests exercising authority rejection install first, then plant the rejected input; a nested fixture constructor must not erase a deliberate hostile input. No package-wide blanket erasure replaces those negative controls.

The standard CI indicator is a declared test-runtime input preserved by the shared owner, so existing missing-tool failures do not become skips. Other undeclared caller inputs remain removed.

Runtime-only Windows values (SystemRoot, WINDIR, COMSPEC and PATHEXT) may be captured explicitly; temporary paths are owned. These values do not authorize inherited PATH entries, credentials or repository selectors. The tests cover Windows key matching and missing executable mappings.

The executable leaf commands and required04 whole-suite lane bind the current producer marker (`default`/`false`). Earlier basic-profile diagnostic receipts remain labelled as historical reproducers; they are not relabelled as current-marker executions. The lane preserves CI tool-requirement strictness and installed declared capabilities.

## Acceptance and limits

m-01: helper plus both consumer groups pass under an injected environment, including unchanged production rejection controls. m-02: the hostile lane participates in the already-required `build-test` aggregate and a failure cannot disappear behind a skipped job. No new branch-protection permission is assumed; if wiring requires an operator-only act, file that act without declaring #6300 complete.

Existing verifier tables, Evidence, row environments, production Git authority, stored-login guards, credential custody and activation remain outside scope. The helpers remove accidental fixture inheritance; production consumers that intentionally receive admitted Git configuration remain a separately diagnosed runtime concern. #6300 closes only after its named migrations, required CI and issue dispositions are complete; #2715 does not close merely because statusgen's transition tests pass.

## Alternatives and proof

Per-file unsets retain the recurring hazard; weakening production refusals changes security behavior; blanket TestMain cleanup can erase the very hostile inputs negative controls must test. The shared scoped helper removes accidental inheritance, while independent hostile parent/victim tests detect incomplete adoption. The helper's own unit tests and a separate CI launch environment fail for different reasons. Review judges scope adequacy; passing local source tests grants no independent verification credit.
