# Forge service contract

The typed protocol and separately imported client are offline implementation work under forge-neutral/39. Serving, credential custody, source refresh ownership and activation remain their existing owners. This document records no operational adoption.

## Architecture pickup

Continue against source main `214b4c8308b3cc57beed91feccf45e6a03f2eecd`, carrying the published pickup clarification from PR 5413 through genuine merge `3fd78f3b7288f087fb66b320bffb2b8381a59d7a`. Review observation transport maps the existing reviewflow definitions and review-observation-v1 schema; it does not compute classifier rows, candidate selection, eligibility, ready authority or policy approval. The predecessor's design approval is distinct from pending independent verification and this contract's own acceptance issue 5414.

Reuse the forgeread root and mirror identity/observation meanings. Existing operation keys are opaque values from their canonical effect owner; the transport neither mints a competing key nor implements reconciliation. Stdlib encoding/json handles protocol coding and net/http is confined to the separate client. No third-party dependency is added to the stdlib-only module.

The client requires an already-admitted transport and an explicitly supplied trusted endpoint and scope binding. It has no default transport, credential lookup or client-side provider fallback. Redirects, unknown required protocol values and scope widening are refused. Fixture HTTP handlers qualify the wire/client boundary only; they are not a production listener or proof of current provider entitlement.


## Mapping and evidence limits

| Source owner | Transport mapping | Qualification limit |
|---|---|---|
| Existing root OpenIssues envelope | Optional complete legacy issue payload for a bounded issue query | Original parser/frozen behavior is unchanged; no service adoption |
| Actual mirror model at published mapper head `2677323c9bdf81aad1a1e3b4ad4407e0703ee25c`, merged through `442a1145a5a04cb2b80b8764106bb77f80904ca3` | Imported Snapshot/Observation types; existing ScanUpdate structural validation; BodyKnown and nil/empty preservation | No source scan, persistence, entitlement or freshness grant from transport |
| Main-pinned reviewflow and review-observation-v1 | Data-only field/tag mapping with scalar pointers; captured original producer results pass the unchanged classifier oracle | No classifier, lane, policy, candidate or current-input evaluation moves |
| Existing issueboard projection rows | Scope/population provenance, every existing action, quarantine and explicit history degradation | Literal mapping only; listing, trust, closure, history reads and selection remain the producer's responsibility |
| Canonical R2/R4 effect owner | Opaque existing operation key/digest, succeeded/failed/unknown provider outcome, confirmed object reference and separate projection availability | No effect is executed or reconciled; a changed payload is a typed conflict |

`forgeread` cannot import `tools/desk/internal`: the compiler refusal is qualified in a disposable fixture. The public DTO layer maps the existing wire/data contract; it neither imports a giant internal interface nor copies evaluation. The separately imported HTTP client makes one RoundTrip and follows no redirect. Server-handler, logical-client-call and transport-attempt measurements retain different units, and diagnostic links require trusted consent at both ends.

A later read-only handler must derive authenticated caller/admission/epochs independently of these request claims and compose the canonical refresh/store/source owners. No client-side source fallback is supplied. The current base mirror source does not establish complete review/check populations; review-dependent serving and full tool-cutover parity remain held until their actual qualified enrichment and existing gates. Root OpenIssues consumers also require CreatedAt/URL values absent from the present mirror model; no zero values are manufactured to claim parity.

## Read-through metadata

Unfulfilled reads and receipt lookups can carry explicit nullable retry advice from
the shared owner's qualified floor. The SDK preserves that timestamp and makes no
automatic retry or provider call. Null is unknown advice, not immediate eligibility.
Create effects, fulfilled results and permission holds carry no retry/cache advice.

An authorized original mirror snapshot can supply data-free cached coverage, its
evaluation instant, elapsed whole milliseconds since LastComplete (or explicit null
when unknown/unrepresentable), and original source provenance when known. The
pure mirror evaluator from the genuinely consumed published coordinator head
`836f586e72e2d161a83d94d52d2845b08d4c2bbd` checks coverage/freshness consistency;
transport neither stamps a new observation time nor grants current authorization.
Minimum-token failure remains independent of age, so older progress can have fresh
cached coverage. A permission reason requires the stripped permission state.

Loopback fixtures cover a healthy cold fill, shared fixture cooldown across two
clients, original cached age/provenance, cold source failure, unavailable service,
permission stripping and recovery. These are wire/client proofs. Actual shared
gate ownership, SQL persistence, provider pacing and authenticated serving remain
with their existing owners.
