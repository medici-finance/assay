# Context handoff v2

`context-handoff-v2` is the seven-record [v1 contract](context-handoff-v1.md) with one
versioned vocabulary addition: `impacts[].lifecycle` also accepts `withdrawn`.
Its schema is [context-handoff-v2.json](../schemas/context-handoff-v2.json).
Every other shape, refusal, approval, coverage, history and active-work rule is unchanged.
The pure `statusgen/contexthandoff` decoder accepts both explicit versions; v1 still
rejects `withdrawn`, and unknown versions fail closed. `HandoffSchema` remains v1;
callers opt into `HandoffSchemaV2`. Canonical digests include the schema identifier.

A withdrawn brief can still be impacted by a new target through surviving code,
evidence, dependencies or an open claim. Include it in discovery and record the
appropriate disposition; absence from dispatch is not absence from the impact graph.
Retaining a withdrawn artifact does not revive it or satisfy a dependency on it.

`amendment-manifest-v1` continues to bind the selected handoff's canonical digest;
no manifest shape or meaning changes. Consumers that only accept v1 must be upgraded
before publishing a v2 handoff. Existing v1 handoffs and their digests remain unchanged.
