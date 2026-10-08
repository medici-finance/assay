# ISO 9001 stream reactivation

**Status:** approved
**Date:** 2026-10-06
**Routes-to:** docs/streams/iso-9001/

## Decision and scope

The driver directed that the existing ISO 9001 stream return to `active` at
priority `P2`. This reverses the prioritization hold recorded on 2026-09-25.

The approved scope is reactivation of the existing workstream, as described in
[its README](README.md), [the clause mapping](../../iso9001-mapping.md) and
[the project assurance proposal](project-assurance-spec.md). It adds no work,
changes no implementation status, and removes no dependency or human gate.
The reactivated work is the stream's existing `todo` briefs: iso-9001/03,
iso-9001/06 and iso-9001/08 through iso-9001/11.
Approval of this prioritization does not approve project activation, data use,
release, deployment or a compliance claim; those retain their existing gates.

The owner-set priority remains P2. Subsequent reprioritization is the owner's
decision, not an implementation step in an individual brief.

Because no brief's `sources:` cites this document, the lifecycle lint reports
it as owing brief authoring. That notice is expected here: the work already
exists as briefs. The tooling gap is tracked in #2244.
