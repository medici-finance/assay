# The enforcement model — what the identities enforce, and what they only attribute

This page states, in precise terms, what Assay's identity separation actually *enforces* versus
what it merely *attributes*. It is deliberately written in the same voice as
[`docs/adopting-assay.md`](adopting-assay.md) §1a: it refuses to overstate the gates. Read it before you describe the segregation-of-duties property to
anyone, and before you decide how few identities a given install can run with.

The short version, unchanged from §1a: **a review posted by the reviewer App is *attribution, not
authorization*.** It names a distinct identity that is answerable for the verdict and that a plain
implementer session cannot post as. That is strictly more than a self-written checkmark; it is not
proof the review was independent, thorough, or even happened.

## The identity set, and what breaks if each is merged into another

This enumeration is grounded in the code, not in prose: the desk boot preflight
(`tools/desk/internal/deskkit/preflight.go`) applies **one** required-duty list — `requiredDuties`:
`pull_requests: write`, `issues: write`, `contents: write` — identically to *every* role App, and
refuses to run a role whose App is missing any of the three. So the question a "minimal path" answers
is never "which App needs fewer permissions" (they all need the same three) — it is **how many
distinct App identities must exist at all.**

*(Reads are the exception to "the same three, identically". They are role-scoped and sit outside
`requiredDuties`, so the boot preflight does not check them: the roles that read CI need
`checks`/`statuses`/`actions: read`, and the **reviewer** — the identity that runs `deskflip` —
additionally needs **`administration: read`** to see the required status checks of a protected
branch whose required set is not expressed in a ruleset — classic protection, or a ruleset carrying
no `required_status_checks` rule. Missing, it costs no boot; it costs every ready-flip on such a
repo, which stays could-not-check forever. See `docs/adopting-assay.md` §3 `setup-reviewer-app`,
*Required checks a ruleset does not express*.)*

| Identity | What it does | If merged into another, what breaks |
|---|---|---|
| **implementer** (the machine account / worker App) | authors branches and commits; opens PRs | Merging it with the **reviewer** destroys the product. The implementer↔reviewer split is the one load-bearing separation: the identity that writes a change and the identity that certifies it must differ. Collapse it and there is no separation left to install. |
| **reviewer** (App) | posts the correctness verdict as a distinct login the author cannot post as | See above — merging with the implementer is the collapse the whole methodology exists to prevent. Merging the reviewer with the **verifier** loses a *distinct* non-author-verification claim (see below) but keeps the load-bearing implementer↔reviewer split intact. |
| **verifier** (App, today distinct) | runs the brief's Verify table on merged main and records Evidence | Both reviewer and verifier are *non-implementers*; sharing one identity between them is a real option with a real cost — it collapses "an independent identity re-verified the merged work" into "the reviewer also ran the checks." It does **not** touch the implementer↔reviewer floor. |
| **desk / loop role Apps** (worker-desk, pr-review-desk, verify-desk, intake, …) | drive the loops; act as their role in the audit trail | Merging these loses **per-role attribution** in the trail — valuable for the audit story, but a *different kind* of value than the implementer↔reviewer separation. Nothing structural is lost; the segregation-of-duties floor is unaffected. |

The consequence the table makes explicit: **there is exactly one merge that destroys the central
claim (implementer into reviewer), and every other merge trades attribution for a smaller identity
set without touching that claim.** Which of those attribution-for-cost trades an install may make —
in particular whether the verifier may share the reviewer's identity, and whether a single-person,
single-repo adopter is supported at all — is a human decision recorded on `#463`, not a call any
model may make.

> **A note on this enumeration versus the decision it feeds.** The install runbook already states
> (§2, *automation identity*) that the **minimum** is one machine account plus the reviewer App it
> owns, and that role-App splitting is *optional*. So the smallest identity set that keeps the
> load-bearing separation — implementer + reviewer — is, on the evidence of the code and the current
> guide, **already the documented minimum**, not a new topology to invent. What `#463` decides is
> which *further* collapses (verifier-into-reviewer; single-person single-repo) become *supported and
> tested* choices, and how the audit-trail cost of each is disclosed.

## The two layers behind the load-bearing separation

The implementer↔reviewer separation is not left to a single control. It has two independent layers,
so that a failure of one does not silently destroy the property:

1. **The forge's own refusal (first layer).** GitHub will not let a PR author approve their own PR.
   This is server-side and keyed on **PR authorship**. On the full-Apps path it is the primary guard.
   Its limits are the ones §1a records: it is *attribution* (whoever holds the reviewer credential
   holds the property), and on a setup without server-side branch protection the merge gate is
   advisory.

2. **The run-time non-author verdict assertion (second layer).** Before the desk tool posts a
   verdict, it compares the **posting identity** against the **author of the head commit** and
   refuses to post on equality — naming both identities. This lives in a *different component* (the
   desk tool, `deskkit.AssertNonAuthorVerdict`, wired into `deskpost review` / `security-review`),
   fires at a *different time* (verdict-post time, not approve time), and keys on a *different signal*
   (identity equality against the certified head, not PR authorship). That independence is the point:
   it catches the case a collapsed identity path can create, where the forge's authorship-keyed
   refusal may not fire because the certifier and the code's author are one actor on a route the
   forge does not treat as self-approval.

   The assertion is **three-state**, per the project's three-state instrument rule: poster ≠ author
   is a permit, poster = author is a refusal, and an *unreadable* head-commit author is a
   could-not-check — it falls back to the PR author (always present) and warns, never a silent pass.
   A transient inability to read the commit author must not brick the reviewer loop, but it also must
   not be reported as a cleared separation.

Removing the second layer leaves a collapsed path with no verdict-time protection at all; that
property is pinned by a test whose mutation (deleting the identity comparison) reddens the
poster-equals-author case while leaving the distinct-author case green.

## What the minimal path enforces — and what it only attributes

The honest scorecard, so no handover note overclaims a smaller install:

- **What a minimal (implementer + reviewer) path still enforces:** the load-bearing separation —
  the certifier is a distinct identity the implementer cannot post as, backed by *both* layers above.
  The run-time non-author assertion applies on this path specifically because it is the path where
  the forge's own refusal is most likely not to fire.
- **What a minimal path gives up:** per-role **attribution** in the audit trail (who-did-what across
  the loop roles), and — if the verifier shares the reviewer's identity — the *distinctness* of the
  non-author-verification claim from the review claim.
- **What no path makes true:** none of this proves a review was independent, thorough, or unbiased.
  The gate is *attribution, not authorization*. A smaller identity set is **not** claimed to be as
  strong as the full one; it is claimed only to keep the one separation the product rests on, and to
  keep it behind two independent layers rather than one.

Nothing on this page provisions an identity or changes a permission: App creation and permission
grants are admin acts and are ESCALATE-only in the runbook. This page describes the model; the
runbook installs it, and `#463` decides how small the supported floor may be.

## A different layer: WHEN dispatch happens, not WHO may certify it

Everything above is identity enforcement — who may post which verdict. The eligibility
evaluator (`statusgen --eligibility`, `docs/dependency-graph-design.md` §3.7,
graph-execution/01) is a **separate, deterministic** enforcement layer that gates *when* a
brief may be dispatched at all, computed from the declared graph (`depends:`/`gates:`)
rather than from any identity: `eligible` (no hold), `held` (excluded from Next-up — a
`gates:` edge or an unsatisfied `depends:` blocks it), or `eligible-with-notice` (offered,
with a `feathers:` edge flagged but never excluding). It runs before any reviewer/verifier
identity is even in play, and it enforces the SAME verdict everywhere it is consulted
(Next-up, the drive frontier) — a brief the board hides can never be offered by the
frontier. It is a graph-shape gate, never an authorization: it says nothing about who
certified the work, only about whether its declared prerequisites are in force.

## A third layer: WHAT evidence must exist before the model lane may release

The eligibility evaluator above answers "may this brief be dispatched at all". The
**coverage rule** (`statusgen --coverage`, `docs/streams/graph-execution/brief-03-evidence-coverage-rule.md`,
graph-execution/03) answers a different, later question: once a brief reaches
`verified`, did every claim it is required to make actually resolve `pass`, at the
revision it is being credited for? Like eligibility it is **deterministic and
identity-free** — it says nothing about who ran a check (that stays evidenceactor.go's
advisory attribution) or who is authorised to certify it (the reviewer/verifier
separation above). It is a THIRD, independent enforcement axis: identity (who may
certify), graph shape (when work may start), and now evidence completeness (whether the
record a certification rests on actually supports it).

Coverage is the ONE place the model-lane auto-flip (`--auto-flip-model`) RE-CHECKS the
`verified` stamp it is about to promote to `done` — every other fact that flip consults
(the README row, the App's approval at the merged head) is taken as given, by design
(see the flip's own header comment). A brief whose mandatory claims are not ALL
`released` never reaches the flip's live review fetch at all: coverage is fully offline,
so it is the cheapest possible refusal, checked first. It never weakens the
implementer↔reviewer separation or the eligibility gate; it adds a THIRD, independent
question neither of those two asks: not "who", not "when", but "did what was required
actually happen, and can this exact revision still stand behind it".
