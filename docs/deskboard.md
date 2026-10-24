# Review queue observation bridge

`deskboard review-queue --out FILE` collects a complete `review-observation-v1`
artifact. With `--out`, stdout is a compact count/generation/cutoff/source-policy
summary; the file retains every row and its resolved facts. Without `--out`, stdout
is the complete JSON artifact. `--table`, `--delta` and `--quiet` are refused for
this verb so a summary cannot be mistaken for the planning artifact.

Artifacts are written atomically with mode 0600, checked close errors, and no following
of a final destination symlink. File custody is the integrity boundary: these unsigned
observations and their provenance digests do not authenticate a provider or policy
approval. Anyone able to replace a snapshot can fabricate internally consistent facts.
Live adoption must re-read authoritative source state before every effect; a saved
plan can never authorize a ready write. Store artifacts in caller-controlled private
directories and do not accept them from untrusted authors.

The collector uses the existing typed `ReviewQueueSnapshot`, bounded repo/PR pools,
conditional reads and per-change fact reuse. Gate scores are read once per cycle.
Heads and reviews share the queue read; the adapter captures canonical resolved
classification facts rather than running a second `prs` sweep. File metadata must
match the frozen queue head. The recorded base, when used, is the reviewed revision
used by the existing own-files comparison. This is an interval observation, not a
forge transaction or a ready grant.

A denied/truncated population, quarantined PR, degraded classification or missing
required fact fails with exit 6 before a complete artifact is emitted. The existing
schema cannot represent quarantined rows, so this bridge holds the whole planning
cycle rather than quietly removing them. Public-repo adoption must account for an
external actor opening a quarantined PR and thereby holding this whole-cycle bridge;
the compatibility path remains available until that existing adoption gate is met. The current qualified collector supports
GitHub; GitLab collection refuses until its complete host-qualified facts are
available. The legacy boards retain their existing degraded-row behavior.

## Offline selection

Use `reviewloop plan --snapshot FILE --selection JSON --now RFC3339`. The two new
flags are `--snapshot` and `--selection`; the latter is an invocation value, not a
new policy file or register. It contains these fields:

```json
{
  "Policy": {
    "Owner": "approved-policy-owner",
    "Revision": "approved-policy-revision",
    "Evaluator": "review-selection-v1",
    "Unmapped": "hold",
    "Priorities": []
  },
  "ObservationPolicy": "source-policy-revision-from-the-collector",
  "Scope": [{"provider": "github", "host": "github.com", "repository": "example-org/tracker"}],
  "Oldest": "2026-10-23T12:00:00Z",
  "Capacity": 1
}
```

The approved caller supplies the source revision and scope it expects, an inclusive
freshness cutoff, the policy owner/revision, and current advisory capacity. `--now`
is mandatory on this path. Scope order matches the recorded requested scope.
The source policy digest binds collector version, configuration, trust/reviewer
identity, repo CI/visibility policy and observed gate scores. Digests do not expose
configuration contents. An expected source/access revision change or expired cutoff
requires refresh or hold. Changing selection priorities or capacity alone reuses
the observation; it does not recollect it. `Oldest` is an absolute cutoff: the
caller must advance it according to its approved freshness requirement. Advancing
`Now` alone introduces no implicit TTL or new cutoff.

Each `Priorities` entry has `Subject` (`Repository` and `Number`) and `Score`.
Unmapped candidates are explicitly held in the example. `observed-score` is an
explicit compatibility fallback; owning-brief/branch inputs can affect those
scores, so it is not a substitute for an approved priority mapping. Missing policy
entries never create candidates. An owner string or digest does not authenticate
approval; the caller remains responsible for that approval.

The output is the complete shared `SelectionPlan` JSON: selected/deferred/held/watch
candidates, exact heads, independent correctness/security conclusions, missing
priority subjects, the uncapped depths, generation interval, cutoff and idle verdict.
Capacity limits proposals, never population/depths. This adapter makes zero network
calls and performs no effects. A roster/claim admission boundary still derives real
capacity and enforces trust, lanes, claims, budgets and consequential rechecks.
Malformed/stale/unsupported/incomplete observations exit 6. A plan whose idle gate
is could-not-check also exits 6; it cannot establish all-clear. Bad or mixed flags
exit 5. Snapshot mode excludes `--actions`, `--prs` and `--records`.

## Compatibility and adoption

The legacy `reviewloop plan --actions FILE [--prs FILE]` form and the `deskboard
 actions` / `prs` wire shapes remain supported through desk-tools/43's caller
migration and equivalence acceptance. There is no calendar sunset or live consumer
cutover in desk-tools/40. Existing role skills, monitor and throughput still use
their legacy paths. The future `desk-review-queue` spelling is a migration target,
not a delivered binary. PostgreSQL, a forge service and desk-simplification are not
prerequisites for this direct-reader bridge; drainloop still owns execution lifecycle.

For R repositories, P PRs and E necessary per-change enrichment requests, the former
review planner preparation collected `actions + prs`: the queue snapshot plus a
second PR-list read per repository, or approximately `2R + E` GitHub calls. This
bridge uses `R + E`: savings R, both O(R + E). The qualified small fake-provider
cohort observes `1 + 2P` cold calls (one queue, P metadata and P files reads), and
zero calls for each local reselection. Recollecting for K capacity/policy-only
changes would cost `(K+1)(R+E)`; reuse costs `R+E`, saving `K(R+E)` and removing K
from network complexity. Cache hits can lower E; this is not a claim that enrichment
has disappeared or that live desks already use the bridge. Desk-tools/44 owns wider
cost qualification; /43 owns adoption and retirement of duplicate live reads.


## Exact-source review preparation

`deskdispatch <claim-key> --kit review --repo <owner/repo> --pr N
--observation FILE ...` accepts the existing closed review-observation-v1 artifact
(up to 16 MiB). The optional flag is valid only for an open-change review kit.
The existing dispatch arguments, model/role admission, canonical claim, attestation,
workspace and stamp checks still apply. Dry-run does not read the file or the forge.
The bridge validates complete observation structure/results and the assigned
provider/instance/repository/number/head (and base when used), before claiming.
Unknown schemas, partial populations and mismatched subjects refuse with exit 6.

The file supplies advisory source attribution, not approval or a cached raw review
history. Its policy revision is untrusted attribution, never an approved selection
policy. The adapter reads current metadata, review bodies and checks from the live
provider; it does not substitute observation classification or grants for those
reads. Consequently no age cutoff in the file can authorize stale mutable facts.
The planner's caller-owned policy/cutoff remains the separate selection contract.

One invocation-scoped context supplies the round and packet. Successful assembly
reads are shared; terminal reads bypass the memo. A head/base/body/base-branch/open-state/decision-label,
review/check or required-check-policy change, or failed access at that boundary,
aborts the assignment and releases the claim. Unrelated title, mergeability,
update-time and routine label changes do not invalidate source applicability. Packet construction remains optional;
if it fails before its terminal check, dispatch performs that check itself. It never
silently relabels old source facts with a new head. Publisher and reviewer-owned
freshness checks remain necessary after dispatch.

Immutable source keys include provider/host/repository, full commit, path and
adapter version. Bounded local APIs accept canonical commit object IDs directly;
missing objects fall back to the provider without revision-expression interpretation. Comparisons additionally key both full endpoint SHAs and the
comparison adapter. Branch-named comparisons are not memoized. Source bytes come
from gitcore at the assigned head, even when the allocated checkout has another
HEAD; missing objects fall back to exact-head provider reads. Local regular blobs
are size-checked before reading and limited to the packet's per-item byte ceiling;
symlinks are never followed. When both base and head objects exist, gitcore builds
the three-dot diff from the exact endpoints. A provider without an exact base
endpoint gets full-pass scope and an explicit diff omission, not a claimed frozen diff. Packet file,
history, time and output caps and untrusted-content escaping remain in force.
Round reads, packet assembly and terminal rechecks each have their own bounded
60-second context through both provider transports, including blocked requests and
response bodies. The packet clock starts at packet construction, after claim/workspace
latency; an expired optional packet reports omissions and does not poison the fresh
terminal check. A terminal read failure is reported as could-not-recheck, distinct
from an observed input change. Omitted/unused requests still count as requests.
GitLab maps `diff_refs.base_sha` to the exact base so its existing patch fallback and
delta path remain available. Local diff inputs are header-checked against a separate 4 MiB input budget before
rename detection or patch construction can inflate blobs; the rendered diff keeps
its 192 KiB output cap. Over-budget or non-regular local inputs use the already-
collected per-file patches under the existing packet caps, with explicit omission
only where a patch is unavailable. This adds no whole-provider diff request.

The qualified one-file delta fixture measures **17 -> 14 HTTP attempts** for round
plus packet with local exact objects (18% fewer), while adding terminal mutable
review/check/protection checks. By endpoint: PR metadata 3 -> 2, changed-files
2 -> 1, diff 1 -> 0, contents 1 -> 0, required-policy 1 -> 2; reviews remain 2,
check/status attempts 4, comparisons 2 and commit detail 1. This is one cold scenario,
not a fleet average. With missing local objects diff/content provider reads remain;
with F packet files available locally, those F content calls become zero. For D
review preparations, this fixture is 17D -> 14D, both O(D); generally mutable and
history reads remain O(D(1+H)), and content requests fall O(DF) -> 0 when exact local
objects cover them. No cross-dispatch or cross-lane cache is introduced.

The reviewer packet contract already requires reading supplied immutable material
without fetching it again, while independently checking the head and current checks.
Tests qualify the dispatcher and packet handoff; live agent compliance and complete
attributed workflow cost remain desk-tools/44 acceptance, not a fabricated zero-cost
measurement. Desk-tools/43 owns adoption of the observation handoff in both prose and
discrete callers. Desktools-v2 retains CLI/custody ownership, and drainloop retains
execution/claim lifecycle; future role-executor cutover remains separately gated.

### Reusing a complete review observation

`deskboard throughput --review-observation FILE` consumes an applicable complete
`review-observation-v1` artifact instead of another review actions sweep. Its review
stage reports the observation cutoff, with current configured scope and local policy
checked and a five-minute freshness limit. Incompatible or unreadable inputs blind
that stage, preserving the other independently measured stages. There is no silent
provider fallback. See [compatibility and rollback](review-efficiency-compatibility.md).
