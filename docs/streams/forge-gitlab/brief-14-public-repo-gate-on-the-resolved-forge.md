---
brief: assay:assay:forge-gitlab:14
title: The public-repo gate reads the forge that serves the repo — every verb, not one
why: >-
  The draft-change verb's public-repo gate was routed onto the resolved forge in #1060, and the
  reviewer who landed it found the identical defect still standing in two sibling verbs: each
  resolves the forge for every other operation on the change, then builds a separate hardcoded
  GitHub client for the visibility read the gate depends on. On a GitLab project that read is sent
  to a host that has never heard of the project, the gate fails closed on a foreign error, and the
  verb has no working path — the exact symptom #1054 reported for the draft-change verb, waiting in
  two more places. A review desk uses both: one edits its workpad, one lands Evidence. Fixing them
  one report at a time is how the third site gets discovered in the field instead of in the tree,
  so this brief closes the CLASS: every gate call site reads through the backend already resolved
  beside it, and the superseded single-forge adapter that made the second shape possible is deleted.
wave: 3
depends: ["forge-gitlab/02"]
unblocks: ["forge-gitlab/16", "forge-gitlab/17"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1066]
schema: brief-v2
authored: 2026-09-14 by forge-gitlab wave-plan authoring session
sources:
  - "#1066 — filed at discovery during the correctness review of PR #1060: the two named sites, the fix shape (swap the hardcoded fetcher for the generic already-resolved-forge adapter), the regression-test pattern to follow, and the note that the earlier single-forge adapter is now fully superseded and unused in production code"
  - "#1054 — the field report the class comes from: the gate's visibility read went to the wrong host, the refusal named a foreign error for a project that does not live there, and the sanctioned draft-change verb had no working path on the adopter"
  - "PR #1060 (MERGED 2026-09-14) — the fix for the draft-change verb and the adapter this brief reuses; its regression test asserts the fetcher handed to the gate wraps the SAME backend already resolved for that repo, which is the assertion shape each verb's test copies"
  - "tools/desk/internal/deskkit/repovis.go — the generic adapter and the gate. The gate treats any fetch error as could-not-check, which is correct given a failed read; the defect is upstream, in WHICH backend the read targets"
  - "tools/desk/cmd/deskpost/forgeclient.go — the counter-example already in the tree: its backend routes the visibility and reaction reads through its own resolved backend and does not carry this defect"
  - "docs/streams/forge-gitlab/spec.md §3 (parity per control) and §6 (freeze rule — no new operation; this brief adds none, it re-points call sites at an operation that already exists on both backends)"
  - "freshness-checked 2026-09-14 @ 2c67b34f — the two sites named in #1066 are live and unchanged by #1060; a THIRD site exists in the release verb, which #1066 does not name; the superseded single-forge adapter is still present and still has no production caller"
exec-tier: strong
exec-tier-why: "the deliverable is the routing of a security gate's input across several call sites, and a site left behind is invisible to every happy-path test on the forge that already worked (question b); the third, unreported site has to be RULED on rather than swept in, because the release verb's forge story is not the review desk's (question a)."
domain: complicated
tier: free
consumers:
  - "tools/desk/cmd/deskevidence/deskevidence.go: fixed-here (the gate's fetcher becomes the already-resolved-forge adapter wrapping the backend resolved above it)"
  - "tools/desk/cmd/deskreply/deskreply.go: fixed-here (same shape; the backend is already taken once and used for every read and the write)"
  - "tools/desk/cmd/deskrelease/cut.go: follow-up forge-gitlab/14 (this brief — RULE on it: either route it like the others, or record at the site why the release verb's gate legitimately stays single-forge. Silence is not one of the two answers)"
  - "tools/desk/internal/deskkit/forge_gitlab.go: fixed-here (delete the superseded single-forge visibility adapter once no caller remains — #1066's non-blocking half)"
  - "tools/desk/internal/deskkit/repovis.go: out-of-scope (the generic adapter and the gate are unchanged; this brief adds no type and relaxes no check there)"
  - "docs/streams/forge-gitlab/README.md: fixed-here (the status row)"
version: 1
id: 83ced764-c760-49c4-8dcf-f1fbc14e5493
---

# Brief 14 — The public-repo gate reads the forge that serves the repo

## Context

The public-repo gate compares a LIVE visibility read against the configured value, so that a
stale roster claiming a project is private cannot authorize an outward write. That design is
correct and is not what this brief touches. What it touches is the client the read is made
through.

Three verbs resolve the forge backend that serves a repo, use it for every operation on the
change — opening it, reading it, writing to it — and then build a *second*, hardcoded,
single-forge HTTP client for the gate's visibility read alone. On the forge that client
happens to speak, nobody notices. On any other, the read is addressed to a host that has never
heard of the project, the gate fails closed on that foreign error, and the verb is unusable
while every other verb in the same session is healthy against the same project.

Two of the three are named in #1066; the third — the release verb — is not, and it is the one
that needs a decision rather than a sweep. The review desk needs the first two: one edits its
workpad on the change, one lands Evidence.

files:
- `tools/desk/cmd/deskevidence/deskevidence.go` — the gate's fetcher becomes the generic
  already-resolved-forge adapter wrapping the backend resolved earlier in the same function.
- `tools/desk/cmd/deskreply/deskreply.go` — same change; the backend is already taken once and
  reused for every read and the write below it, so the adapter is a two-line swap.
- `tools/desk/cmd/deskrelease/cut.go` — the third site. RULE on it in this brief: route it, or
  record at the site, in a comment, why a release cut's gate legitimately reads one forge.
- `tools/desk/internal/deskkit/forge_gitlab.go` — delete the superseded single-forge visibility
  adapter, which the generic one has replaced and which no production code calls.
- `tools/desk/cmd/deskevidence/*_test.go`, `tools/desk/cmd/deskreply/*_test.go` — one regression
  test per verb, asserting the fetcher handed to the gate wraps the SAME backend the verb
  already resolved, then exercising its visibility read against a fake backend.
- `docs/streams/forge-gitlab/README.md` — the status row.
- `changelog/forge-gitlab-14-public-repo-gate-on-the-resolved-forge.md` (planned).

single-point-of-failure: the per-verb swap is one control and repeating it is redundancy, not
depth — so the second layer is a different kind of check in a different component: a
package-level test that enumerates the gate's construction sites across the command tree and
fails on any site building a single-forge fetcher that is not the ruled exception. The swap can
be forgotten at a new verb; the enumeration cannot be, because it is the thing that goes red
when a new verb appears. Different signal, different component, and neither is satisfied by the
other going wrong.

facts:
- The already-resolved-forge adapter is generic and already exists; no new type is needed. It
  was introduced by PR #1060 for the draft-change verb.
- The gate's fail-closed posture on a read error is CORRECT and is preserved unchanged. This
  brief re-points the read, it does not relax the gate.
- The reference assertion shape is PR #1060's: the fetcher the verb hands the gate wraps the
  SAME backend instance already resolved for that repo — an identity assertion, not a
  behaviour assertion, because a second correctly-configured client would pass a behaviour
  test and still be a second client.
- The counter-example already in the tree is the verdict verb's backend, which routes the
  visibility and reaction reads through its own resolved backend.
- Freshness: verified 2026-09-14 @ `2c67b34f` — two sites from #1066 live, one further site in
  the release verb, superseded adapter still present with no production caller.

## Edition
Minimum GitLab tier: **free**. The visibility read is a Free-tier project read on every tier,
and the gate's comparison is local. No tier-gated field is touched and no degradation is
disclosed or removed.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity
  does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Do NOT weaken the gate to make a read succeed. A forge that genuinely cannot answer keeps
  failing closed; the refusal must name the forge that was asked, not the one that was not.

## Task
1. Swap the gate's fetcher at the two sites #1066 names for the generic already-resolved-forge
   adapter, reusing the backend already in scope at each site. No new type, no new resolution.
2. Rule on the third site in the release verb: route it the same way, or leave it and record
   the reason in a comment at the site naming what makes a release cut different. Say which you
   chose in the PR body.
3. Delete the superseded single-forge visibility adapter once no caller remains.
4. Add one regression test per swapped verb, following PR #1060's identity-assertion shape, and
   show each failing on the unfixed code before the swap.
5. Add the enumeration test: every public-repo-gate construction site in the command tree uses
   the generic adapter, except the sites this brief explicitly ruled out, which the test names.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go build ./... && go test ./cmd/deskevidence/... ./cmd/deskreply/... ./internal/deskkit/... -timeout 420s` | exit 0 | check:ci |
| 2 | `cd tools/desk && go test ./cmd/deskevidence/ -run TestPublicRepoGateFetcherRoutesThroughResolvedForge -v -timeout 120s` | exit 0; output contains `PASS` — the fetcher handed to the gate wraps the same backend the verb resolved, and the subtest then EXERCISES its visibility read against the fake backend, so the claim "the gate reads the resolved forge" is resolved rather than asserted (`TestPublicRepoGateFetcherRoutesThroughResolvedForge` (planned)) | check +dereference |
| 3 | `cd tools/desk && go test ./cmd/deskreply/ -run TestPublicRepoGateFetcherRoutesThroughResolvedForge -v -timeout 120s` | exit 0; output contains `PASS` — same assertion for the reply verb (`TestPublicRepoGateFetcherRoutesThroughResolvedForge` (planned)) | check |
| 4 | `cd tools/desk && go test ./cmd/... -run TestPublicRepoGateSitesUseResolvedForge -v -timeout 300s` | exit 0; output contains `PASS` — no gate site across the whole command tree builds a single-forge fetcher outside the named exceptions; this is the cross-command row, walking every verb's gate construction rather than the two this brief edits (`TestPublicRepoGateSitesUseResolvedForge` (planned)) | check +flow |
| 5 | `cd tools/desk && go test ./cmd/deskevidence/ -run TestPublicRepoGateFetcherRoutesThroughResolvedForge -count=1 -timeout 120s` run against the pre-swap tree (stash the swap, or check out the parent commit) | exit non-zero at the type assertion, BEFORE any network call — proving the test observes the defect it pins | check +mutation |
| 6 | `cd tools/desk && grep -rn 'GitLabRepoInfoFetcher' --include='*.go' . \| grep -v '_test.go' \| wc -l \| tr -d ' '` | `0` — the superseded single-forge adapter is gone from production code | check |
| 7 | `statusgen --root . --consumers` | exit 0 | check |
| 8 | `statusgen --root . --lint` | exit 0; output contains `LINT: PASS` | check |

### Pre-mortem → detection map
| Failure mode of the work | Caught by |
|---|---|
| One verb is swapped, the other is missed, and the next field report finds it | rows 2 and 3 (one per verb) + row 4 (the enumeration) |
| A NEW verb added later builds a hardcoded fetcher again | row 4 — this is the reason row 4 exists rather than two per-verb rows being enough |
| The swap compiles and passes because the test constructs its own adapter rather than reading the one the verb built | row 5: on the unfixed tree the assertion must fail at the type, before any network call |
| The release-verb site is neither routed nor ruled, and the silence reads as a decision | no row — Task step 2 requires the answer in the PR body; adequacy of the reason is review-only |
| The gate is relaxed to make a read succeed on a forge that cannot answer | no row — Ground rules forbid it; a diff touching the gate's error handling is a review bounce |

### Dispatch checklist
```
[x] 1. Rows discriminate — row 4 goes red on a fourth hardcoded site; row 5 goes red on a test that never saw the defect.
[x] 2. Facts dated — every fact carries the 2026-09-14 @ 2c67b34f freshness check, including the third site #1066 does not name.
[x] 3. Self-contained — the sites, the adapter, the assertion shape and the counter-example are named here; the implementer never has to open #1066 to work.
[x] 4. Risk answers match files: — command call sites plus a deletion; no credential, no new identity, no write semantics change: all four stay no. The gate itself is untouched.
[x] 5. gate-why n/a (gate: model, all four no).
[x] 6. Effort honest — two swaps is S, but the ruling on the third site, the enumeration test and the adapter deletion make it M.
[x] 7. Shared value: the gate's fetcher contract is read by several verbs, so consumers: enumerates every site incl. the ruled one, and row 4 is the cross-site flow row.
[x] 8. Pre-mortem run; the two review-only items are named.
```

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
Reviewer answers: does the enumeration test actually enumerate — would it go red on a new verb
that built its own fetcher — or does it only assert the sites this brief already changed?
