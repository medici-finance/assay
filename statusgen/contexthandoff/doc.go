// Package contexthandoff holds context-handoff-v1/v2 and amendment-manifest-v1
// records (spec/context-handoff-v1.md; contract S-context-handoff): the typed
// handoff a spec author gives a brief author when a change moves the plan, and
// the manifest that binds each amendment obligation to a patch or a follow-up.
//
// The package does three things and nothing else:
//
//   - Decode: a strict reader. Unknown fields, duplicate keys, missing fields,
//     nulls where a value is required, invalid UTF-8 and trailing data are all
//     refused. Every field is mandatory to understand, so a field this version
//     does not know is never silently dropped.
//   - Validate: the per-value shape the JSON schemas also state, plus the
//     cross-field rules the schemas cannot express (coverage membership, the
//     approval revision, impact-to-amendment binding, readiness).
//   - Canonicalize: brief references are expanded to the four-segment
//     <cell>:<repo>:<stream>:<NN> form, and the record is serialized with sorted
//     keys and no insignificant whitespace, so the sha256 digest is stable
//     across field order and formatting.
//
// The offline resolver (statusgen/20) works over an explicit Snapshot:
//
//   - Discover: enumerate owners and consumers from declared edges, plan
//     frontmatter, consumers: fields, independent directory enumeration and
//     deterministic prose candidates, and record the limits and coverage of
//     the run as a context-handoff receipt.
//   - Recheck: compare a prior receipt with a new snapshot and classify each
//     change as preserved, reconcile or hold.
//   - Check: bind each amendment obligation's patch to its base revision and
//     refuse one that does not apply, rewrites protected history, silently
//     rebinds an active claim, patches a delivered brief, or contradicts an
//     approved rival.
//
// It is pure. It reads no git repository, forge, environment variable or
// clock, and runs no process. LoadSnapshot reads only the fs.FS the caller
// hands it; the statusgen context adapter chooses and opens that root.
//
// A valid record is not a complete one. Validation proves the record is
// well-formed and internally consistent, including that every coverage gap is
// stated; it does not prove discovery found every consumer. A receipt covers
// architecture only: product alignment is unchecked until product binding is
// supported.
package contexthandoff
