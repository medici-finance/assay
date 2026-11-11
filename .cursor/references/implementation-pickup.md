<!-- GENERATED from plugins/assay/references/implementation-pickup.md by deskinstall --harness cursor. Do not edit. -->
# Implementation pickup: current architecture and guidelines

<!-- assay:harnesslint non-matrix-reference — harness-neutral implementation-pickup procedure read by the author and worker skills and dispatch kits, not a per-harness capability binding; it names no harness capability and carries no per-skill degradation cells -->

Use before the first implementation effect, on resume, and after a material scope or
approved-target change. This is an affected-context assessment, not permission to
redesign the system or expand the brief. Preserve the original brief/source history.

1. Read the current applicable project instructions, the brief's controlling spec and
   selected approved architecture/transition, semantic-owner contracts and engineering
   guidelines. Follow project pointers to their canonical homes; do not assume one
   universal architecture filename. Record revisions/digests and missing sources.
2. Inspect the real implementation, module/import boundary and available APIs; discover
   actual consumers, planned replacements and affected unfinished work. For a refactor,
   state which owner is reused/extracted, why any process boundary remains, forbidden
   capabilities, and what duplicate path is retired. A linked future design is not a
   shipped API. Separate observed, approved, transition and proposed state.
3. Record an `Architecture pickup` assessment in the implementation plan or PR: brief
   identity/revision; target/transition and guideline/contract revisions; inspected
   implementation/producer versions; affected consumers/dependencies; coverage/unknowns;
   and a reasoned **continue**, **adapt**, or **hold** disposition. Reuse the existing
   context receipt when supported; do not create a second register or invent fields.
4. Continue when compatible. Adapt only within the authorized scope and update Task,
   consumer/dependency routing and Verify before implementing a changed contract; obtain
   the existing owner/checkpoint acknowledgment for active work. Hold only the affected
   work for a material unresolved ownership, acceptance, custody or compatibility
   conflict, or missing required evidence. Route the existing decision/context process.
   A draft rival or unrelated edit never automatically blocks approved work.
5. Recheck affected sources before publication/dispatch and on resume/material change.
   Preserve unaffected conclusions with their non-intersection rationale. Carry the
   assessment into review; check the actual diff retires the named duplicate and calls
   the chosen owner. A declaration or import alone is not conformance evidence.

**Shipped operation.** Where the installed version supports the receipt, use
`statusgen context recheck --snapshot DIR receipt.json` for steps 1, 2 and 5 (Assay v1.0.55
and later for `context-handoff-v1`, v1.0.63 and later for the product-bound
`context-handoff-v3`), and `statusgen context check` for a brief amendment. A desk dispatching
the work may run the same recheck before the claim with `deskdispatch --context-snapshot DIR
--context-receipt FILE` (the release after v1.0.81), which holds a receipt to reconcile or that
cannot be confirmed before any claim exists. The `author-spec`, `author-brief` and
`worker-desk` skills point here; this is the single procedure.

Otherwise, and for an install that lacks the verb or the receipt's schema, this
source-pinned assessment is the **manual bootstrap**; disclose its coverage and obtain
any bootstrap approval required by the controlling architecture. It does not pretend
a planned command exists, grant a new permission or relax an existing human gate.
