# Evaluation context v1

`evaluation-context-v1` extends the pure `statusgen/domain` owner; it adds no
cache, evaluator, scheduler, source access, wire adapter or admission authority.
The normative shape is `schemas/evaluation-context-v1.json`. Wire consumers must
validate that closed schema before decoding, then call `EvaluationContext.Validate`.
Unknown fields, versions and mandatory capabilities cannot be silently ignored.

The required field inventory is:

- `schema`, `requires`: version and exactly one capability (`["context@1"]`).
  Empty, repeated and unsupported requirements are refused; the v1 set has one member.
- `work`, `candidate`, `acceptance`: the existing verification-domain-v1 identities
  and exact accepted definition, including its candidate and required facts. The
  qualified work names cell/repository; the candidate names the forge repository.
  Their alias correspondence and authenticity remain trusted adapter obligations.
- `sources`, `facts`: nonempty exact observation frontiers. Each unique public
  semantic name has a SHA-256 digest of normalized immutable observations, including
  absence. Digests commit the complete evaluator input closure, not file overlap.
- `evaluator`: existing evaluation.Identity version and policy digest.
- `query-digest`, `options-digest`: digests of normalized query and options. Do not
  put arbitrary query payloads, bearer tokens or raw identity secrets in records.
- `dependencies`, `dependency-digest`: the entire graph inventory, including known
  absent qualified members, presence and observation digests. DependencySetDigest
  sorts the inventory by qualified work and hashes canonical JSON. Removal,
  discovery and changed presence all invalidate reuse. Adapters must observe the
  whole inventory; a self-consistent digest does not authenticate completeness.
- `coverage`: present means the adapter has complete dependency coverage. Missing
  and could-not-check remain retainable records but cannot produce a reusable
  fingerprint. They require conservative reevaluation through the existing
  evaluation.DependencyManifest semantics, never invented selective applicability.
- `policy-digest`, `environment-digest`: explicit policy and environment versions.
  Include model, tool, prompt and relevant nondeterministic environment identity in
  the normalized environment manifest. Attribution does not prove reproducibility.
- `access-epoch`, `generation`: positive access-policy epoch and reevaluation
  generation. These are supplied by current trusted composition, not renewed here.
- `observed-cutoff`: UTC RFC3339 whole-second cutoff of the frozen observations.
- `bounds`: unique lease and access expiry observations are mandatory; optional
  evaluation expiry may further shorten validity. Every bound is strictly after
  the cutoff. No omitted or unavailable authority implies an infinite expiry.

Fingerprint validates the complete receipt, then hashes only the immutable
projection: schema, requires, work, candidate, acceptance, sources, facts, evaluator,
query-digest, options-digest, dependencies, dependency-digest, coverage, policy-digest
and environment-digest. It excludes observed-cutoff, bounds, access-epoch and
generation: those are mandatory currency observations, separately checked at use.
Canonical JSON has recursively sorted object keys, exact decimal integers, standard
encoding/json escaping and sorted named sets (requires, acceptance requirements,
frontiers and dependencies). Duplicate frontiers, dependencies and bounds refuse;
requires is the exact singleton above. Caller slices are not mutated. A digest proves
content identity only, never authenticity, current validity or caller authorization.

CheckApplicability(cached, current, observedNow) consumes independently observed
current context and explicit trusted time, reading no clock itself. Invalid or
incomplete contexts/time yield held; changed immutable inputs yield reevaluate.
Access-policy epoch and reevaluation generation must separately match exactly, even
when the immutable fingerprint is unchanged. A later re-observation cutoff or renewed
bound alone does not change immutable derivation identity and can retain a hit.

Matching derivations are reusable only at/after both cutoffs and strictly before
every bound in both receipts: the effective window is their intersection. A renewed
current grant never extends the cached receipt's expiry; a shortened current lease,
access grant or evaluation bound takes effect immediately. At or after any required
expiry, unchanged immutable data yields reevaluate. Results have stable state/reason
and omit input values. Held or reevaluate never mean eligible or authorized.

Reuse is advisory computation. Admission must independently obtain current caller
permission and lease/access observations even if the upper validator is bypassed;
a fingerprint or supported Assess result cannot supply that authority. Reuse never
retargets an old exact-revision PASS: Assessment.CheckCurrent and existing coverage
still bind work/candidate/acceptance/generation and the actual witness tree.

Offline evaluation retains its cutoff and missing coverage and cannot assert current
unspent authority. Restoring a receipt does not renew its cutoff, epoch, generation
or expiry. Planned graph-execution/14 obtains fresh authority and uses this contract
at dispatch/result/effect admission; /19 persists and reloads the complete receipt
without replacing expiry with restore time. /21 consumes admission through /14.
Forge source/index cache owners remain separate and gain no permissions here.

Conformance: TestContextInputs, TestContextExpiry, TestContextAdmissionFlow,
TestContextMembership and TestContextSchema exercise fixed fixtures, existing Assess
and evaluation, exact revision binding and an independent synthetic admission check.
An independently loaded current fixture has a later cutoff and renewed bounds;
its immutable hit is compared with fresh Assess. Explicit bypass of the synthetic
upper context check still reaches the independent lower authorization check. The
committed authority and membership mutations respectively treat a hit as authority
and compare only the surviving dependency intersection. The schema supplies shape;
the pure package adds calendar validity, set uniqueness,
required lease/access kinds and cross-field/digest consistency. The contract does
not authenticate observations or implement a production gate cache.
