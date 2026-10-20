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
// It is pure. It reads no file, git repository, forge, environment variable or
// clock, and runs no process; the caller hands it bytes. Producing a receipt
// from a real tree, and checking that a bound patch applies to its base, are
// the resolver's adapter work (statusgen/20), not this package's.
//
// A valid record is not a complete one. Validation proves the record is
// well-formed and internally consistent, including that every coverage gap is
// stated; it does not prove discovery found every consumer.
package contexthandoff
