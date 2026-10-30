# git-ai: reusable cache and source-control concepts

Research date: 22 October 2026. Disposition: reference implementation; retain Assay's current
architecture. Scope: source/document inspection, not a benchmark, deployment or security audit.

Upstream snapshot: [`feder-cr/git-ai` at ecf3ed5](https://github.com/feder-cr/git-ai/tree/ecf3ed5e31b2bb148c501451bd09a32d080f2af1),
resolved from main on this date. All upstream links below pin that snapshot. Assay baseline:
`49d6511daca94a978fff49c00bceea3a0d8af9e6`, checked against upstream main on this date. Briefs
42 and 44 were version 4 before this amendment; their new obligations are version 5. Their
implementation/evidence gates remain separate from this research.

Search terms: git-ai, Graft, Scion, Gitaly, immutable cache, single-flight, miss coalescing,
object batching, sparse workspace, exact SHA, derived data, compare-and-swap, changeset log,
shared Git store, fetch reuse, cache authorization, quarantine, performance attribution.

## Decision and placement

Use the project to mine mechanisms and failure cases. It is a Gitaly fork with additional gRPC
services, a `graft` workspace client and optional `graft-cache` tiers. Normal Git repositories
remain underneath. It independently adopts publicly described Scion ideas; it is not Anthropic's
implementation. It does not provide a complete forge product. See the pinned
[architecture](https://github.com/feder-cr/git-ai/blob/ecf3ed5e31b2bb148c501451bd09a32d080f2af1/doc/graft/ARCHITECTURE.md)
and [limitations](https://github.com/feder-cr/git-ai/blob/ecf3ed5e31b2bb148c501451bd09a32d080f2af1/doc/graft/LIMITATIONS.md).

`graft-cache` consumes its ObjectService API. It cannot simply intercept our ordinary GitHub/GitLab
Git fetches or provider API requests. An adapter or a different server/client path would be needed.
The potential reuse seam is the existing trusted source-query composition, underneath the typed
FileAt/Diff/MergeBase contract. GitHub/GitLab still own forge records; Assay's existing owners retain
orchestration, custody, workspaces, evidence and publication. No dependency on Graft is selected.

## Concept inventory

The observations in this table describe upstream code/design; the Assay column is our assessment.

| Concept | Observed upstream mechanism | Assay application or boundary |
|---|---|---|
| Sparse, online source access | ObjectService serves objects and trees; the client materializes selected paths | Exact-source inspection can avoid creating a workspace; builds and tools needing local history still need the existing workspace path |
| Immutable data versus moving refs | Object cache keys include storage/repository and OID; tree keys include recursion/size options; revision-name tree requests bypass caching | Retain repository/instance, both diff endpoints, path/options and derivation version; resolve mutable heads separately |
| Coalesced misses | In-flight loads share object work, with one batched upstream fill for owned misses | Compare against our already shared object cache; bound cancellation, retries, waiters and credential handling |
| Bounded tiers | Per-node memory LRU; optional regional disk store; request byte limits and pass-through on oversize | First measure an in-process improvement; another service or disk tier must earn its operating cost |
| Integrity | Recompute Git OID from object type, size and content; disk objects are verified when read | Integrity is independent of current access and object membership; corrupt data cannot become a successful source answer |
| Current authorization | Before cached reads, ask upstream RepositoryExists with forwarded caller credentials; absent credentials may use a configured tier token | Preserve our current admission and membership rules; include access-check cost and dependency failure in warm-hit measurements |
| Private temporary views | Requests carrying alternate/quarantine object-directory overrides bypass cache | Isolate unpublished views while preserving our existing admitted alternate-store reader |
| Preseeded cache | Snapshot seed records preload verified trees/blobs | Optional future experiment; charge construction, distribution, verification, startup and disk costs |
| Derived answers | Versioned content-addressed history indexes, path summaries, last-modification and blame reuse | Reuse exact-source facts; mutable approval/check/eligibility dependencies remain under their existing owners |
| Concurrent landing | Rebase/merge onto current tip and compare-and-swap; batch multiple lands into a ref update | Separate source-control publication design; no adoption in the cache work |
| Changeset stream | List/watch accepted reference updates with actor and operation | Useful event/recovery study; does not observe all external forge review/check state or replace Assay's effect ledger |
| Rebuildable storage nodes | Optional authoritative database refs/changesets plus immutable packs in object storage; local repos can be reconstructed | A source-storage architecture, distinct from Assay's operational coordination database; no migration selected |
| Conflict proposals and verification | External resolver protocol and optional verification; post-land tester bisects/reverts | Candidate tree binding is worth studying; a test pass or revert loop does not establish independent acceptance |

Sources for the cache rows:
[server and trust model](https://github.com/feder-cr/git-ai/blob/ecf3ed5e31b2bb148c501451bd09a32d080f2af1/internal/graft/cachetier/server.go),
[in-flight loading and keys](https://github.com/feder-cr/git-ai/blob/ecf3ed5e31b2bb148c501451bd09a32d080f2af1/internal/graft/cachetier/cache.go),
[verification and disk store](https://github.com/feder-cr/git-ai/blob/ecf3ed5e31b2bb148c501451bd09a32d080f2af1/internal/graft/cachetier/object.go),
[seed format](https://github.com/feder-cr/git-ai/blob/ecf3ed5e31b2bb148c501451bd09a32d080f2af1/internal/graft/cachetier/seed.go).
The alternate bypass refers to explicit request directory overrides, not proof that every repository
whose on-disk store uses alternates is uncacheable.

## Mechanisms worth mining

**Share work without sharing authority.** Each caller is admitted before participating in cached
reads. A successful shared fill can satisfy multiple readers, but another client's failed fill does
not establish the current client's result: upstream's loader retries failed shared work using the
current request's context. Cancellation and retry bounds need their own qualification. The cache
keys scope physical reuse by repository, rather than treating a globally known SHA as permission.
Our source-admission contract adds requirements beyond copying that implementation.

**Batch after deduplication.** Group the remaining misses, instead of fetching each object separately.
Measure object decoding/index work independently from Git transport fetches. Batching object RPCs is
not equivalent to coalescing pack negotiation or coordinating local-store maintenance. Our existing
`gitcore.OpenWith` already shares a decoded-object cache across sequential worktree reads, and our
alternate reader retains pack indexes for an Open lifetime. Those are baseline features, not benefits
to attribute to a new cache. See [pre-lift Git owner](https://github.com/medici-finance/assay/blob/ee2b42102c5f57e7737379bc88dc5902f0624f83/tools/desk/internal/gitcore/gitcore.go) and
[pre-lift alternate reader](https://github.com/medici-finance/assay/blob/ee2b42102c5f57e7737379bc88dc5902f0624f83/tools/desk/internal/gitcore/alternate_reader.go).

**Bound the whole request path.** Upstream bounds LRU storage and per-request buffered misses,
including disk hits and data received from other fills. Oversize requests fall back to upstream
streaming. Per-request bounds alone do not establish a total process-memory ceiling at arbitrary
concurrency; our experiment must measure and bound aggregate in-flight memory as well.

**Compute expensive facts once.** Upstream's [derived store](https://github.com/feder-cr/git-ai/blob/ecf3ed5e31b2bb148c501451bd09a32d080f2af1/doc/graft/DERIVED.md)
uses versioned immutable keys and atomic replacement. It includes graph/changed-path summaries and
last-modification/blame reuse. Repeated source-history questions may benefit, but no immutable cache
entry establishes that a review remains approved or that current access remains valid. Broader
incremental invalidation stays with forge-neutral/43.

**Separate durability from caches.** The [storage core](https://github.com/feder-cr/git-ai/blob/ecf3ed5e31b2bb148c501451bd09a32d080f2af1/doc/graft/STORAGE.md)
uploads immutable packs before committing database changes; reference updates use compare-and-swap.
Its checkpoints, review roots and GC are useful retention/recovery references. This snapshot documents
GC mechanisms: do not repeat the older blanket claim that pack GC is missing. Cloud conditional-write
behavior is explicitly marked untested in that document. Our evidence-retention and maintenance owners
remain authoritative; cache eviction must never become deletion of required evidence.

## Evidence limits and adoption risks

The [benchmark](https://github.com/feder-cr/git-ai/blob/ecf3ed5e31b2bb148c501451bd09a32d080f2af1/doc/graft/BENCH.md)
compares concurrent landing paths, including batching on versus off on the same server. It does not
measure our GitHub/GitLab cache, source-query workload or complete review throughput. Its short,
contended runs and documented variance cannot establish an Assay performance target.

A cache hit still performs upstream authorization in this design. It can save object work while
remaining dependent on upstream latency and availability. The configured fallback token expands
what unauthenticated reachable clients can access; it is not a custody pattern to import into Assay.
The limitations document also distinguishes test coverage by subsystem; its general operations section
says multi-node behavior is untested. Do not treat a broad README claim as qualification of every
production path. No upstream suite or Assay experiment was run for this report.

Source-control landing and test-after-land are architectural choices. Our cache amendments preserve
merge/review policy, exact-candidate evidence, independent verification and provider publication
ownership. Neither the source changeset stream nor a successful test substitutes for those controls.

## Experiment routing and decision rule

[Brief 42](../streams/forge-neutral/brief-42-sha-bound-source-queries-and-immutable-result-caching.md)
carries correctness, admission, immutable keys, bounded reads, integrity and source-query measurement.
[Brief 44](../streams/forge-neutral/brief-44-shared-git-object-and-fetch-reuse-qualification-trial.md)
carries measurement and the keep/reject/uncertain decision. Their Task and Verify sections are the
execution contracts; this report adds no delivery or activation authority.

Compare current reads, existing optimized shared readers, coalescing alone, batching/fetch reuse
alone and their justified combination. Use matched cold/warm/concurrent review workloads, independent
source-answer oracles and explicit failed/canceled/revoked/corrupt/oversized cases. Predeclare sample
sizes, improvement thresholds and resource/tail-latency ceilings before collecting results. Report
actual sends separately from simulated provider equivalents and unknown quota data.

Require per-technique deltas against the optimized baseline, including authorization, retained disk,
maintenance, startup and recovery costs. Prefer the existing process/store owner when sufficient.
Only measured residual cross-process/host demand can justify proposing a separate service brief;
unmeasured benefit remains uncertain. A no-go result completes the trial. No result here activates
another service, changes forge authority or certifies a production deployment.

## Future mining questions

- Which repeated source questions still dominate after one review preparation context and existing
  object/index reuse? Are the remaining duplicates within one process, across processes or across hosts?
- Does coalescing reduce actual object work, or mostly add waiting and lock contention at our concurrency?
- Does batching improve full preparation latency once admission, decoding, bounds and maintenance count?
- Would seeds or a persistent tier pay back construction/startup/verification costs within real task lifetimes?
- Can source-history summaries help our exact-source queries without introducing mutable readiness caches?
- What failure/retention semantics would a separate service need that the best local arm cannot provide?

Append later findings with source/subject revisions, workload, raw sample links and limitations. Keep
this inspection snapshot separate from the future trial's measured results.
