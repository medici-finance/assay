# contributor-trust — scoping document

**Status:** approved — commissioned 2026-09-12; the approval is stamped by the merge of the
pull request that lands this document, per `spec/lifecycle-v1.md` §8.4.
**Routes-to:** docs/streams/contributor-trust/

## 1. The problem, stated from observation

This repository is public and its automation is real. Until now every inbound item came from
an identity the roster already knew, so "who is this author?" was answered by a set-membership
test and nothing downstream ever had to ask a second question.

That assumption broke on 2026-09-12, when the first unsolicited pull requests from unknown
accounts arrived. The shape is worth recording precisely, because it is the shape the controls
in this stream are designed against — and because none of it is, on its own, evidence of bad
intent:

- a long-dormant account resumed activity and opened roughly eleven pull requests across
  roughly eleven unrelated repositories inside twelve hours;
- each one fork → branch → pull request inside three to five minutes;
- the bodies were near-identical in shape, all asserting some variant of "fixed, all tests
  pass";
- at least one such assertion was provably false against the diff it accompanied;
- several were closed by the receiving maintainers within hours;
- the changes themselves were small and benign.

Read one at a time, every item in that list is ordinary. Read together they describe an
automated sweep operating faster than any review queue can absorb, with body claims that were
not checked before they were written. The failure mode is not malice; it is **an unverified
claim entering a review queue at machine speed**, and a maintainer's attention being the only
thing standing between it and a merge.

## 2. What already exists, measured

Measured on `origin/main` at authoring (2026-09-12):

| Surface | What it does today |
|---|---|
| Roster author bar | An operator-configured set of trusted logins, a blessing authority, a human-login map and a set of trusted bot slugs. Unset trusts nobody; a human login with no pinned numeric id is refused on the strict path. |
| Per-item blessing | A comment by the configured blessing authority admits one issue or pull request into the automation. It is bless-then-edit aware: content added after the latest blessing comment voids it. |
| Quarantine | Unblessed inbound content is surfaced, never executed — it appears on the external/unblessed lane of the board rather than becoming a work item. |
| Inbound triage workflow | `.github/workflows/inbound-triage.yml` comments and labels on issue-first (`needs-issue-link`) and per-author concurrency (`too-many-open-prs`). Both enforcement switches default off. It uses `pull_request_target` and deliberately never checks out the pull request's code. |
| Fork CI | First-time fork contributors require a maintainer's manual "approve and run" before workflows execute. |
| Resident rule | Never build or test an unblessed fork head. |
| Changelog proxy | A fork pull request whose branch maintainers cannot push to is credited by a `changelog/pr-<N>-*.md` fragment landed on the base branch on its behalf. |
| Published policy | `CONTRIBUTING.md` states issue-first, the concurrency guideline, maintainer-merge, and that automation ignores outside items until a maintainer engages. |

## 3. What is missing

The existing bar is **binary and per-item**: an author is trusted or not, and an item is
blessed or not. Three consequences follow.

1. **No signal is gathered.** The decision to bless is made from whatever the maintainer
   happens to notice. Every signal in §1 is mechanically derivable from public metadata and
   none of it is collected, so the same judgement is re-made from scratch, worse, every time.
2. **Nothing accumulates.** A contributor whose third change lands is re-assessed exactly like
   a stranger. There is no state between "unknown" and "maintainer", so there is no way to
   reward a good record and no way to record a bad one except in a maintainer's memory.
3. **Review depth does not vary.** An unknown author's body claim is read the same way a
   maintainer's is — which is precisely the assumption §1 disproves.

## 4. What this stream decides

A four-tier trust model (`unknown → blessed-once → contributor → maintainer`), an
operator-side per-repository ledger recording which tier each external identity holds and why,
a structured blessing act that writes that ledger, review depth and continuous-integration
posture keyed on the tier, and a published contributor-facing statement of the bar.

Three boundaries, set here so no brief re-opens them:

- **The tiers gate automation, never a human maintainer's judgement.** A tier decides how much
  machinery runs unattended. It never decides whether a change is good, and it never merges.
- **Promotion and demotion are recorded human acts with reasons.** No signal, and no
  accumulation of signals, ever moves an identity between tiers by itself.
- **Signals are facts, verdicts are people's.** Every mechanical output of this stream states
  what was measured and abstains from concluding. An instrument that could not measure says so
  as a third state; it never reports "clean".

## 5. What this stream does not decide

Merge authority (unchanged: a human maintainer merges), the published contribution guidelines'
enforcement switches (a separate, recorded maintainer ruling), and anything about the identity
or intent of any specific account. Nothing in this stream names an external contributor.
