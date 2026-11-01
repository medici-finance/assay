# Reader capability negotiation

Version: 1. Implementation contract for the pure API and the named preflights.
This contract does not implement milestone loading, assessment, authoring, dispatch,
acceptance, or installed-reader adoption.

## Ownership and authority

`statusgen/readercaps` owns deterministic interpretation of supplied advertisements
and requirements. `Assess` accepts explicit typed values and returns `supported`,
`unsupported`, or `could-not-check` plus a diagnostic. It reads no environment, clock,
filesystem, credential, network or process. A supported result grants no claim,
execution, current-subject, human/risk-gate or acceptance authority. Those independent
consumer checks retain their existing owners.

`readercaps/record` adapts existing fenced YAML using briefdoc and the existing YAML
codec. `readercaps/tree` is the explicit filesystem host adapter shared by the two
binaries; the pure core never imports it. Hosts retain transport, observation validity,
diagnostics and exit adaptation. No new module or process boundary is introduced.

## Wire envelopes

[The JSON Schema](../schemas/reader-capabilities-v1.json) describes both envelopes.
All named keys are required, objects are closed, and arrays must be explicit, including
empty arrays. Missing/null lists are unknown, not empty. The pure decoders also reject
duplicate JSON keys and noncanonical field spellings (including case-folded aliases),
duplicate operation names and duplicate features within a list.
The schema describes structure; these cross-entry uniqueness rules require the decoder.
Unknown envelope versions and malformed advertisements are could-not-check.

An advertisement has `schema: reader-capabilities-v1`, nonempty `provider` and `version`
strings, and `operations`. Each operation has a nonempty `operation` name plus `schemas`
and `semantics` arrays of exact, versioned capability identifiers. For example:

```json
{"schema":"reader-capabilities-v1","provider":"example-reader","version":"dev","operations":[{"operation":"syntax","schemas":["brief-v1"],"semantics":[]}]}
```

Schema identifiers include their version, such as `brief-v2`. Semantic identifiers
also name an exact contract version, such as `required-semantics-v1`. There are no
implicit ranges, latest-version substitutions, aliases or wildcard support. Provider
and build version identify the source; neither implies support. A parser being able to
ignore a field does not establish its required semantics. Advertisements for syntax,
assessment, dispatch, authoring and acceptance are separate operation entries.

A requirements envelope has `schema: reader-requirements-v1` and `required`, whose
entries have the same operation/schema/semantics shape. All entries and all listed
features are mandatory. Unknown required features and unadvertised operations return
unsupported; they never become optional data or inherit support from another operation.
An explicit empty `required: []` preserves legacy behavior without consulting a peer.

Opted-in future stream indices and brief records place the envelope under the top-level
fenced-YAML key `reader-requirements`. Its entries name every operation the record
requires; the current tree preflight conservatively requires the complete envelope,
rather than dropping another operation's obligation when producing a narrower view.
The key is allocated by this contract, not inserted into the operational corpus.
Field presence is determined after the shared YAML decoder, so equivalent encoded key
spellings cannot hide a mandatory envelope. Body prose and ordinary legacy records do
not opt in. Capability extraction leaves unrelated legacy risk metadata with its existing owner. The shared structural decoder
validates document count, duplicate keys and key characters before any presence decision,
including records with no requirements; malformed structure is could-not-check. Thus
previously tolerated multi-document or invisible-key records are deliberately held.
Opted-in envelopes retain the existing brief-field validation. Ordinary and differential lint both enforce capability
admission before reading records. Record/schema generators remain
with the subsequent milestone briefs; a legacy permissive parser is not a safe writer.

## Observation validity

`Observation` carries an explicit host-supplied state and advertisement. Only `current`
permits positive support assessment. Missing, unreadable, malformed, stale, absent or
unknown observation states return could-not-check for a required operation. There is
no default-fresh timestamp or fallback to installed plugin versions. The existing host
observation boundary owns authenticity, observation time, freshness policy, source/build
binding and any transport failure. This API does not fetch a peer or validate its claims
against source. Fixture advertisements prove negotiation behavior, never deployed support.

## Production integration and limits

`statusgen --version --json` and `statusgen version --json` add `readerCapabilities` to
the existing document. Its syntax list comes from the shared briefdoc schema registry used by the production parser. Bare version
output remains unchanged. `deskboard` adds the same JSON discovery forms without entering
startup, credentials, telemetry or execution. Its adapter lists the legacy syntax it
actually gates and its requirements preflight. Both advertise only legacy brief syntax
and `preflight` support for `reader-requirements-v1`/`required-semantics-v1`.
Neither advertises prefixed operational references, milestones, assessment, dispatch,
authoring or acceptance. A public workref parser is not an operational writer.

The existing statusgen tree/version gate and deskkit's schema gate call the same core
through the shared tree adapter before their consumers proceed. Canonical stream indices and
brief records under docs/streams and docs/archive are examined: `<stream>/README.md`,
`<stream>/brief-*.md` and `<stream>/done/brief-*.md`. Prose/example subdirectories are not
operational records. Reserved register directories and self-declaring register indices
use the shared briefdoc recognition owner and their indices are excluded. This never
excludes brief records beside those indices. Briefs are checked even where the legacy
loader skips a self-declared register or a malformed briefs table; a directory cannot
hide a requirement-bearing brief by making its index unreadable. Stream discovery ignores
depth-one symlinks; this adapter does too. Symlinked configured roots and canonical
record paths remain could-not-check. Only the exact decoded `reader-requirements` key opts in; an ordinary
case-variant key is not an alias, while structurally unreadable keys refuse before lookup.
Unknown required semantics map to refusal (5); unreadable/malformed requirements map to could-not-check
(6). The historic no-new-requirements release gate retains its exits and diagnostics. An
unterminated actual record now stops at capability preflight with could-not-check (6),
before the Evidence caller reaches its existing stamp refusal (5). The stamp guard is
unchanged and independently tested below preflight; both paths permit no forge writes.
Evidence startup preflight runs inside the existing audit lock before landing reads,
credentials or effects. Both malformed and unsupported refusals append one existing audit
row with result, record path and reason when the sink is writable. An append failure is
reported explicitly on stderr; it cannot be reported as a recorded row.

Discovery scope never substitutes for admission of a record named by a caller. After
existing credential/receipt admission, Evidence reads repo-relative source bytes through
`tree.ReadRecord` using `receipt.EvidenceRoot`, including when no `--root` was supplied.
A missing version-2 admitted root is could-not-check. Named records are checked regardless
of filename or discovery scope; observed symlink components refuse. The opened root
contains the read and root/file identity changes while opening refuse. Absolute Evidence
fragments keep their separate existing rules and must remain outside the admitted root.
The fetched target is checked before merging, and the exact final bytes are checked
independently immediately before either forge write path. A late mutation cannot reuse
an earlier capability verdict.

`Result.Reason` and host capability diagnostics are single-line printable ASCII, at most
2048 bytes. Control/non-ASCII runes are escaped; truncation is visible. This changes only
display text: capability identifiers remain exact during assessment. Record-derived
refusals cannot inject raw terminal controls or exceed the audit reader's line cap; the
CLI regression requires a following ordinary landing to succeed.

Canonical archived indices and done/archive briefs deliberately retain structural
admission even where the legacy board loader would skip malformed frozen content.
Unterminated fences, invalid YAML, multiple documents and invisible keys hold the
whole tree until repaired; the /14 rollout must measure adopter archives before re-pinning.
The existing deskboard, deskpr, deskclaim and deskevidence calls retain their own
execution/identity/subject checks. The deskboard consuming-boundary test bypasses the
outer startup/guard and still observes zero dispatch/output effects on incompatibility.
The actual configured-root consumer also checks every resolved root before reader
process/pin resolution, so a clean working directory cannot clear an incompatible sibling.

This delivery does not retrofit older binaries. A genuinely old binary may ignore an
unknown YAML key. Negotiation-aware consumers assess that peer's explicit supported set,
or hold a missing advertisement, before allowing effects. Operational declarations remain
held until coordinated readers and rollout are qualified by the later briefs. The current
schema gate does not claim complete dispatcher/pickup eligibility (/10), negotiated
newbrief authoring (/04), milestone acceptance, or release/adoption (/14).

## Conformance

`readercaps/testdata/conformance.json` is shared positive/negative wire data. The named
production tests are TestReaderCapsCompat, TestStatusgenCapsTruth and
TestDeskCapsPreflight (deskkit, deskboard, deskevidence, deskclaim and deskpr). TestReaderCapsSchema checks the shape
artifact with the existing validator; decoder tests cover stricter duplicate rules.
TestReaderCapsBoundary checks direct production imports and the compiled dependency closure.
The single-reader guard uses module-relative paths and has an explicit basename-collision
control. Evidence CLI cases cover oversized/control-bearing diagnostics, a following write,
named/skipped locations, admitted-root selection, remote requirements, and final bytes
changed after upper admission.
The executable controls live in `statusgen/testdata/milestone-support/02-mutations.json`.
Each named behavior must pass unmodified and fail under its plausible mutation.
These tests are author validation, not independent post-merge Evidence or acceptance.
