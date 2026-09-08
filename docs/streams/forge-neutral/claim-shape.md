# Claim shape — where a dispatch claim lives, and how it is released

**Brief:** [forge-neutral/05](brief-05-claim-layer-forge-shape.md) · **Decided:** 2026-09-07 ·
**Decision:** the dispatch claim moves into the branch namespace — `refs/heads/dispatch/<key>` —
on **both** forges.

The dispatch claim is the fleet's cross-machine mutual-exclusion primitive: a ref in the target
repository that says *this item is being worked, by someone, somewhere*. Taking one and reading
one have always been plain git and work on any host. Releasing one did not: `DeleteRef` mapped
only the `heads/` namespace on GitLab and answered could-not-check for anything else, in as many
words — such a claim is *"NOT reported released"*. A claim that can be taken and never given back
is a slot lost for good, so on GitLab the fleet's mutual exclusion degraded to a leak.

This file records the live reads the decision turns on, the decision itself, what it costs, and
what the rejected alternative would have required.

## 1. Live reads

Every row is an endpoint, a status code, and the date it was read. Nothing here is taken from a
documentation badge or an edition matrix — where an instrument could not look, the row says
**COULD-NOT-CHECK** and says why.

| # | Deployment | Question | Read | Result | Date |
|---|---|---|---|---|---|
| L1 | gitlab.com SaaS (anonymous, public project `278964`) | Does GitLab expose a general ref endpoint the way GitHub's git-data API does? | `GET /api/v4/projects/278964/repository/refs` | **HTTP 404**, body `{"error":"404 Not Found"}` — the *route-miss* shape, not a resource-miss: there is no such endpoint to authorize into | 2026-09-07 |
| L2 | gitlab.com SaaS (anonymous, public project `278964`) | Same question, under the other plausible spelling | `GET /api/v4/projects/278964/repository/git/refs` | **HTTP 404**, route-miss — no general ref surface under either name | 2026-09-07 |
| L3 | gitlab.com SaaS (anonymous, public project `278964`) | Does the Branches API exist and answer, at Free tier, unauthenticated? | `GET /api/v4/projects/278964/repository/branches?per_page=1` | **HTTP 200** | 2026-09-07 |
| L4 | gitlab.com SaaS (anonymous, public project `278964`) | Can the Branches API ADDRESS a branch whose name contains a separator — the shape `dispatch/<key>` takes? | `GET /api/v4/projects/278964/repository/branches/dispatch%2Fno-such-claim--key` | **HTTP 404**, body `{"message":"404 Branch Not Found"}` — the *resource-miss* shape: the route matched and the encoded name was accepted as one branch, the branch simply does not exist | 2026-09-07 |
| L5 | gitlab.com SaaS (anonymous, public project `278964`) | Is that distinction real, or does everything answer 404 alike? | `GET /api/v4/projects/278964/repository/branches/dispatch/no-such-claim--key` (separator NOT encoded) | **HTTP 404**, body `{"error":"404 Not Found"}` — route-miss, a different body from L4. The two failure shapes ARE distinguishable, which is what makes L1/L2 and L4 readable as evidence rather than as noise | 2026-09-07 |
| L6 | gitlab.com SaaS, GitLab `19.4.0-pre`, **Free**, project `86032201` | Can a Developer-role service account CREATE and DELETE a branch through the Branches API? | `POST /projects/86032201/repository/branches` then `DELETE …` | **HTTP 201** then **HTTP 204** — recorded live in [`../forge-gitlab/pilot-report.md`](../forge-gitlab/pilot-report.md) §3 row 12 | 2026-09-02 |
| L7 | github.com, `medici-finance/assay` (worker App installation token) | Does the git-data ref API address a ref BY REF PATH inside the `heads/` namespace? | `GET /repos/medici-finance/assay/git/ref/heads/main` | **HTTP 200** | 2026-09-07 |
| L8 | github.com, `medici-finance/assay` (worker App installation token) | Is a claim-shaped ref under that namespace addressable, and does absence report as not-found rather than as an error? | `GET /repos/medici-finance/assay/git/ref/heads/dispatch/no-such-claim--key` | **HTTP 404** — the shape `IsForgeNotFound` reads, which is what lets a release treat an already-released claim as a no-op | 2026-09-07 |
| L9 | github.com, `medici-finance/assay` (worker App installation token) | Is the chosen namespace listable as a namespace? | `GET /repos/medici-finance/assay/git/matching-refs/heads/dispatch/` | **HTTP 200** (empty list — no claims are live in this namespace yet) | 2026-09-07 |
| L10 | — | Can a ref OUTSIDE `refs/heads` and `refs/tags` be PUSHED to a GitLab project at all? | A `git push <sha>:refs/<other>/<key>` probe needs a write credential against a GitLab project | **COULD-NOT-CHECK** — no GitLab custody file exists on the implementing machine (no `gitlab-<role>.token` on the App-credential search path), so no authenticated write could be attempted. See §2 for why the decision does not turn on this | 2026-09-07 |
| L11 | — | Can such a ref be DELETED on GitLab? | — | **Answered by L1+L2 rather than by a probe**: there is no route. A DELETE cannot be authorized into an endpoint that does not exist | 2026-09-07 |

**What L1/L2 establish that documentation could not.** A 404 on an API path is ambiguous — it can
mean "you may not see this" as easily as "there is no such thing". L4 and L5 disambiguate it on
this deployment by exhibiting both bodies from the same host in the same minute: GitLab answers a
missing *resource* with `{"message":"404 …"}` and a missing *route* with `{"error":"404 Not
Found"}`. L1 and L2 return the route-miss body. So the absence of a general ref endpoint is a
measured property of the running deployment, not a claim read off a docs page.

## 2. The decision

| # | Option | Release path on GitHub | Release path on GitLab | Verdict |
|---|---|---|---|---|
| D1 | **CHOSEN — `refs/heads/dispatch/<key>`**: the claim is a branch under a reserved prefix | `DELETE /repos/{o}/{r}/git/refs/heads/dispatch/{key}` — the git-data ref API, which serves every namespace (L7/L8/L9 address the same resource by the same ref path) | `DELETE /projects/{id}/repository/branches/dispatch%2F{key}` — the Branches API, **Free tier**, proved live at **HTTP 204** (L6) and proved to accept the encoded separator (L4) | **Adopted.** Both halves of the round trip have a live status code on a live deployment, and the namespace is the SAME on both forges |
| D2 | REJECTED — `refs/dispatch/<key>`: the claim keeps its own namespace, and the GitLab release is implemented against whatever endpoint serves it | Same git-data ref API — GitHub serves this namespace and always did | **There is no endpoint.** L1 and L2 return the route-miss body, so there is nothing to implement against at any tier; and L10 could not even establish that such a ref can be pushed in the first place | **Rejected.** Not on the balance of costs — on the absence of a release path at all |
| D3 | REJECTED — `refs/tags/dispatch/<key>`: the claim is a tag | Same git-data ref API | The Tags API exists and answers (`GET …/repository/tags` → HTTP 200, 2026-09-07), so a release would be expressible | **Rejected on cost, not capability.** Tags are auto-followed by a plain `git fetch` when they point at fetched objects, so every clone in the fleet would accumulate claim tags and keep them after the remote ref was deleted — a locally-stale claim universe on every machine. Branches are not auto-fetched, so D1 has no equivalent |
| D4 | REJECTED — release the claim over the git protocol (`git push origin :<ref>`), symmetric with the plain-git create | n/a — no forge API involved | n/a | **Rejected.** It is the most elegant answer and it is not available: it still depends on L10, which could not be established, and it would put a second write transport beside the enumerated forge surface the stream exists to make the only one. If L10 is ever answered YES, this is the option to revisit — see §4 |

**Why the COULD-NOT-CHECK at L10 does not decide anything.** D2 needs BOTH a create and a delete.
Its delete has no route (L1, L2), measured. So L10 — whether the create would have worked — can
only move D2 from *unimplementable* to *implementable-at-the-taking-end-only*, which is the same
verdict. The decision runs the safe direction of the unknown: the option that was adopted is the
one whose every step has a live status code, and the option left unmeasured is the one that was
rejected anyway.

## 3. What this costs

Recorded because a design whose costs nobody wrote down is a design that will be re-litigated by
whoever trips over them.

| # | Cost | Who pays it, and the mitigation |
|---|---|---|
| C1 | **A live claim is visible in the branch list.** `git branch -r`, the forge's branch picker, and any "stale branches" report now show one entry per in-flight item | Accepted, and partly a feature: a claim that shows up where humans already look is more legible than a ref nothing lists. The reserved `dispatch/` prefix keeps them sorted together and greppable |
| C2 | **A push-triggered CI configuration will see claim branches.** A workflow whose trigger is a bare `on: push` runs once per claim taken | **A cutover precondition, not a consequence to absorb.** Before the create side moves, every push-triggered pipeline in a repo the fleet dispatches into must exclude the claim prefix (`branches-ignore: ['dispatch/**']`, or the equivalent). This repository's own `ci.yml` triggers on a bare `push` and is therefore in scope; the edit is a workflow-file change, which is human-gated here, so it is named as a precondition rather than made in this change |
| C3 | **The namespace move is a cutover, and the two namespaces must not be live at once.** A reader on the new namespace does not see a claim held at the old one, and would report the slot free | Claims are short-lived (one dispatch), so the cutover is a drain rather than a migration: quiesce dispatch, let live claims release, then move the create side (the consumer repo's `dispatch-claim` helper). The second layer holds regardless — `desksupervise`'s staleness reclaim frees a slot on elapsed time plus no live branch, a different signal in a different component |
| C4 | **Branch-protection and deletion rules apply to the claim now that it is a branch.** A ruleset that forbids branch deletion under a pattern covering `dispatch/**` would block the release | Nothing in the fleet protects that prefix today, and the failure is LOUD by construction (the release reports a non-nil error naming the claim key — Verify row 6), not silent. Named here so a future ruleset author knows the prefix is load-bearing |

## 4. Remainders, named rather than left implicit

- **L10 is open.** Whether a non-`heads`/`tags` ref can be pushed to a GitLab project is still
  unmeasured, and answering it is the only thing that would reopen D4. It needs a GitLab custody
  file on the measuring machine; it does not need a new brief until someone wants D4.
- **`desksupervise`'s live claim listing still composes a `github.com` clone URL.** It reads the
  claim namespace through the shared constant now, so it cannot drift on the namespace — but the
  HOST it lists against is still hardcoded. That is a read-verb/host-resolution defect and belongs
  to [`forge-neutral/06`](brief-06-read-verbs-on-the-seam.md), not here.
- **The consumer repo's `dispatch-claim` helper is out of scope by the brief's own consumers
  routing.** This change fixes the CONTRACT it is invoked under (the ref path it must create is
  `deskkit.ClaimRefPath`'s answer) and the release path it depends on. Moving its create side is
  the cutover C3 describes.
