# Work references

`workref.Parse` and `workref.Resolve` are public, pure stdlib-only library calls.
They interpret supplied values; they never read a file, environment, token, clock or
network. Hosts adapt the existing graph alias registry to `Root` values and existing
UUID/rename observations to `Binding` values. `NewRegistry` copies them and refuses
ambiguous roots, conflicting keys and aliases that change kind. The registry has no
mutators and is safe to share between readers.

Use `TypedBrief` or `TypedMilestone` when a boundary requires one kind; `Typed`
accepts either. The final segment alone selects the kind. Colon elisions require the
actual declaring cell/root, never a slug search. Typed resolution refuses unknown roots.
All legacy aliases listed in brief §3.8 and the specified earlier draft slash forms
normalize through the same core. Unprefixed compatibility inputs have no revision suffix.
Milestone dependency references require a positive exact version; this API parses
references, not unversioned milestone definition keys. Versions use `uint64` and
refuse overflow. Brief number spelling retains the existing two-to-four-digit and
optional lowercase suffix grammar, including historical zero values. Milestone
numbers are positive decimal with at least two digits and no letter suffix.

`Result.Identity` is canonical logical identity. `LogicalKey` is the supplied stable
key/UUID when present, otherwise the canonical typed identity key. `Spelling` retains
the input, `Compatibility` is the old unprefixed brief fullId (empty for milestones),
and `Revision` is an exact selector, with zero denoting no supplied selector.
A selector never enters the logical key. The resolver establishes neither record
existence nor revision availability: hosts must bind unversioned reads to their selected
source snapshot and retain observed version/commit/path separately. Missing requested
revisions remain unknown at the record-loading boundary; a newer revision cannot
substitute. No source revision or approval is inferred here.

Existing production entrypoints select explicit profiles:

- `contexthandoff.ResolveQualifiedID` calls `Resolve` with `LegacyQualified`.
  It preserves strict colon-only qualified IDs, optional letter suffixes, exact old
  diagnostics and the existing absence of graph-registry validation. Its declaration
  context remains copied as before. The remaining name/number regexes in that package
  validate snapshot root fields and board-row labels, rather than reinterpreting references.
- `deskkit.SplitBriefTrailer` and `SplitCrossRepoBriefTrailer` call `Parse` with
  `LegacyTrailer` and `LegacyCrossRepo`. They preserve their different projection rules,
  numeric lengths, trimming and case behavior; `CanonicalBriefID` still lowercases.
- `deskkit.ResolveWorkTrailer` explicitly selects `TypedBrief` and retains the exact
  revision separately. Operational writers do not call this adapter in this delivery.

Neither legacy entrypoint gains `b-`, `m-` or revision support. `statusgen/briefv2.go`
and verification-domain's strict boundary remain unchanged. Capability negotiation,
record loading, claims and ruling/subject binding belong to subsequent milestone briefs.
No operational prefixed writes or milestone capability advertisement is enabled here.

`tools/desk` requires the existing statusgen module at the repository's placeholder
pseudo-version and replaces it with `../../statusgen` for source builds. No new module
or third-party dependency is introduced. The new package's entire production closure is
stdlib; the desk module keeps its existing effectful closure and authority boundaries.
The external consumer test serves the actual statusgen manifest and required pure
package source through a local file proxy, with GOWORK off and no consumer replace.
It proves this API can be consumed; it is not proof that placeholder sibling versions
or a statusgen release have been published. A release/pin update remains separate work.

Run the four named Verify tests in the brief. After committing, run
`python3 statusgen/testdata/milestone-support/01-mutate.py` from the repository root.
The manifest uses the explicit `workref-mutations-v1` schema: `cases` hold
repo-root-relative `target` paths. It is a Python runner input, separate from
the native muhar spec format, whose edits are relative to a Go module and cannot
create these planted source files. The runner first requires each pristine
named test to pass, then requires every case to fail its named test; build or
setup failures do not count. All ten cases remain covered, including three
that plant a second production source with an effect import. The runner restores
each plant. The repository's native muhar anchor check remains unchanged.
