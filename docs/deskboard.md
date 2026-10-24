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
