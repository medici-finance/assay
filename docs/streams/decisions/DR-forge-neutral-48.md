---
id: DR-forge-neutral-48
date: "2026-11-16"
title: "Native mirror response bound (forge-neutral/48): one shared early bound of 8 MiB decoded JSON bytes per enumerated mirror-page response, applied before buffering or SDK decoding on both providers; proposed, not yet ratified as a design record"
consequence: major # The transcriber's classification.
decided-by: "human:<name>"
alternatives:
  - "Hold the change for a named correction (the second path the brief's `## Human decision` and decision issue #5384 offer). Not taken: the driver's acceptance comment on #5384 accepts the demonstrated boundary. No rationale beyond that comment is recorded, and none is supplied here."
  - "Rely on the existing aggregate 8 MiB payload budget from forge-neutral/47 alone. Ruled out by the brief's `why:` and design-fit `why-add:`: the aggregate check runs after one native response has already allocated, so it cannot prevent that allocation."
  - "Enforce the limit from Content-Length alone. Ruled out by the brief's Task 1 (\"Content-Length alone is insufficient\"): a false or absent length, chunking or compression expansion evades it."
  - "A global cap on every native read. Ruled out by the brief's Task 2: the bound applies only to enumerated mirror page reads, and unrelated logs, archives, ordinary reads and writes keep their behavior."
  - "A new credential owner, generic request/URL/token interface, retry loop or network default. Ruled out by the brief's ground rules: the admitted native clients and their private backend copies are reused."
accepted:
  - "Oversize is a typed limit/could-not-check result: no successful population, no nonzero ObservedUpTo, no valid truncated prefix and no complete-empty success (the brief's Task 3). The aggregate budget from forge-neutral/47 stays as an independent second boundary."
  - "Unsupported content encoding refuses rather than supplying decoded observations (the delivering pull request's weight section)."
  - "The bound covers transport only. Persisted coverage, the joined mapper and watermark failure flow, retention, scheduling, coalescing and tool cutover stay with forge-neutral/37, /50 and downstream owners (the brief's consumers and Review)."
  - "Acceptance grants no credential, private-data ingestion, persistent store, listener or activation (the brief's `## Human decision` and the driver's acceptance comment on #5384)."
---

## State and provenance

**Approval state: proposed, not ratified as a design record.** The `decided-by` stamp above is
the public placeholder and names nobody. This record carries no `ruling:` link, so
`statusgen --corroborate` reports it `placeholder-unratified` until the driver posts, on the
decision issue, an unedited comment from the driver's own login that names `DR-forge-neutral-48`.
The `ruling:` link to that comment, and the brief's `design:` key, are added only after it exists.

**What the driver has already ruled.** The brief's decision issue,
[#5384](https://github.com/medici-finance/assay/issues/5384), carries the decision-gate marker for
`assay:assay:forge-neutral:48`. On 2026-11-16 the driver's own login commented, unedited:
"forge-neutral/48: accept. I accept the demonstrated mirror-only native response boundary in
#5387. This acceptance grants no credential, private-data ingestion, persistent store, listener or
activation." The brief already cites that comment as its `ruling:`. That comment names the brief,
not this record, so it serves the brief's decision-gate
hold (`spec/lifecycle-v1.md` §4.5) and cannot serve as this record's `ruling:` link
(`spec/registers-v1.md` §7.5, `record-not-named`).

**Written after implementation.** forge-neutral/48 was delivered by
[#5387](https://github.com/medici-finance/assay/pull/5387), merged 2026-11-03, before this record
existed. It is written on demand under the ruling on #5145, which limits back-filled design
records to held briefs that block other work: forge-neutral/37, forge-neutral/50 and
forge-neutral/53 each declare `depends:` on forge-neutral/48, and all three are `todo`.

This record does not mint a decision. It is taken from the brief, the delivering pull request and
the code on main; nothing here adds rationale beyond what those record.

## The design

As the brief asks and #5387 delivered:

1. One shared implementation in `tools/desk/internal/deskkit/forge_response_bounds.go` bounds each
   enumerated mirror-page response for both GitHub and GitLab, success and error bodies alike,
   before full-body buffering or SDK decoding. It counts bytes after supported content decoding and
   reads at most the limit plus one detection byte; the limit is the existing
   `adapters.MaxUpdateBytes`.
2. Only the two enumerated incremental mirror operations in `mirrorsource.go` use it, through
   private admitted backend copies that keep cancellation, scope, no-redirect custody and request
   accounting. Ordinary forge operations are unchanged.
3. The guard is registered in `docs/contracts.md` as `R-mirror-native-response`; its weight is
   in the brief's design-fit and #5387's Weight section.

The brief and its delivering diff are the authority; this summary does not amend either.

## What this record does not decide

It does not record a verification result: the brief's Evidence is empty and its Verify table stays
the independent verifier's. It does not move the brief's board row. It does not decide
forge-neutral/37, /50 or /53.
