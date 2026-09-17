# server-controls — scoping document

**Status:** proposed — this document reserves the `server-controls` namespace and states the
problem for a human ruling. It is not approved; the stream README ships `status: parked`, so
nothing here is pulled onto any board until a human approves this scope. Approval is stamped
by the merge of the pull request that flips this header to `approved` and the README to
`status: active`, per `spec/lifecycle-v1.md` §8.4.
**Routes-to:** docs/streams/server-controls/

## 0. Framing — the rejected ask, and the ask that replaces it

A recurring request has been to *"provision fine-grained privileges"* on the forge so that
the merge gate, the review gate, and every custody rule the desk depends on are enforced by
the platform rather than by client-tool restraint. **That ask is impossible as stated.** A
forge (GitHub, GitLab) does not offer arbitrary per-invariant privileges. It offers:

1. a **fixed permission menu** on the App/PAT (the *ceiling* — `contents: write`,
   `pull_requests: write`, and so on — coarse, and never merge-specific);
2. a **fixed ruleset menu** on the protected ref (required-approval-count,
   require-last-push-approval, dismiss-stale-on-push, restrict-who-can-push,
   restrict-who-can-dismiss, required signed commits, block-force-push, block-deletion,
   required status checks — GitLab adds *prevent approval by author* and *prevent approval by
   committers*); and
3. **required status checks** — the one place a ruleset defers to code the operator supplies.

Nothing between (2) and (3) exists. So the ask is reframed into three tractable things, and
this stream is the plan for them:

1. **Set the coarse ruleset menu UNIFORMLY across every repo the fleet runs.** The menu is
   fixed; the gap is that it is not set the *same* on every repo, so client tools compensate
   for the difference. Close the difference.
2. **Standardize on rulesets and retire classic branch protection, so the controls are
   READABLE (#1020).** #1020 is a *read* gap, not an enforcement gap: `deskflip` cannot read
   classic branch protection because its App lacks `administration: read`, so every draft PR
   on a classic-protection repo is stuck at could-not-check. One readable API (rulesets) on
   every repo, or the `administration: read` grant, closes it.
3. **Express any invariant FINER than the menu as a required STATUS CHECK reported by a runner
   the policed party cannot control.** This is the only mechanism the forge gives for a
   fine-grained rule, and this repo already runs one instance of it: the `leak-sweep` required
   status check. The critical caveat, stated up front because a check written without it is
   worse than no check: **a required check is only as trustworthy as the identity that reports
   it and the context it runs in.** It must run from the base repository (not the PR branch's
   own `pull_request` workflow, whose code the author controls), report under an identity the
   author does not hold, and have its own source on a protected path. A check that violates any
   of these degrades to *self-attestation* — a green lamp the policed party wired themselves.

## 1. What already exists, measured

Live read on `origin/main` 2026-09-16 (`gh api repos/medici-finance/assay/rulesets` and the
per-ruleset detail endpoint for each returned id):

| Ruleset (id) | What it enforces today |
|---|---|
| `protect-main` (20301257) | `pull_request` rule: `required_approving_review_count: 1`, `dismiss_stale_reviews_on_push: true`, `require_last_push_approval: true`, `require_extra_approval_for_unattributed_changes: true`, `dismissal_restriction.enabled: false`. Plus `deletion` and `non_fast_forward` (force-push blocked). The JSON carries NO `bypass_actors` key — no actor is configured to bypass. |
| `leak-sweep` (20872509) | `required_status_checks` rule with one context, `leak-sweep`; plus `deletion`, `non_fast_forward`, and a `pull_request` rule at count 0. This is a LIVE instance of primitive #3: a fine rule ("no withheld content") the ruleset cannot express, delegated to a required check reported by a base-repo workflow the PR author does not control. |
| `protect-release-tags` (20301270) | tag-target ruleset (out of this stream's scope; noted for completeness). |

**The consequence for the anti-collusion question (#997).** #997's original premise —
*"one approval from ANY identity, no server-side anti-collusion block"* — is **partly stale**
on this repo. Because `require_last_push_approval: true` and
`require_extra_approval_for_unattributed_changes: true` are both set, **author ≠ approver is
already enforced server-side**: the identity that pushed the head cannot be the one approval
that satisfies the count. What the menu genuinely *cannot* express, and what remains the real
residual, is the *finer* case:

- an **independent / second** approver requirement (more than one distinct approver, or an
  approver drawn from a specific set) beyond the single at-head approval; and
- **cross-operator collusion** — two role Apps, each a legitimate `pull_requests: write`
  holder, one pushing and a *different* one approving. The forge sees two distinct GitHub
  Apps and is satisfied; it has no setting that can see they may be driven by the same
  operator, because "operated by the same human" is not a property the platform models.

Only primitive #3 — a required check reported by a runner neither App controls — can close
that residual. This stream states that honestly; it does not round the `require_last_push`
enforcement down to "nothing stops self-merge," nor round the residual up to "solved."

## 2. Why a new stream, and not an existing one

Two nearby streams were considered and neither squarely fits:

- **`forge-neutral`** is about making the *desk verbs* the only sanctioned write path and
  giving them a GitHub/GitLab-neutral mapping — a *client-abstraction* concern. Its
  `brief-19-human-only-surfaces-server-side` documents the merge-approval gap surface-by-surface
  and proposes a `human-approved` check, and this stream builds directly on that finding. But
  forge-neutral's spine is neutrality of the tool surface, not the *cross-repo provisioning*
  of the server-side control menu. Folding provisioning into it would blur two concerns.
- **`contributor-trust`** is about the *inbound* bar for external people and agents (trust
  tiers, blessing, fork CI posture). It governs who and what is trusted, not how the merge
  gate itself is provisioned on the server. Adjacent, not the same.

The concern here — *the server-side control posture, set uniformly and made readable, with a
required-check pattern for everything finer than the menu* — has no existing home. Hence a new
stream, `server-controls`, parked pending approval.

## 3. Scope of the stream (what the briefs cover)

1. A **uniform-ruleset audit**: define the target menu, then read each configured repo's
   current ruleset state into a table against it. Reading is the brief; *applying* a changed
   setting is a human/admin act the brief ends in as a provisioning ask.
2. **Standardize on rulesets / retire classic protection (#1020)** so the controls are
   readable through one API, or record the `administration: read` grant as the alternative.
3. The **required-check enforcement pattern** — the reusable design (base-repo execution,
   non-author identity, protected source, and custody of any input data the rule reads beyond
   the triggering event) and its self-attestation caveat.
4. A **decision-dependency note** folding the credential/identity design decisions (#900,
   #903, #942) that gate the credential-contract parts — cited, not re-authored.
5. A **reference cross-operator / independent-approver check** — the #997 residual, expressed
   as a required status check per primitive #3.

## 4. Out of scope / explicit non-goals

- **Provisioning fine-grained per-invariant privileges** — impossible on the forge; this is
  the rejected framing §0 replaces.
- **Performing any admin act** — creating/editing rulesets, retiring classic protection,
  granting an App permission, or adding a required check to a ruleset are repo-admin acts,
  human-performed. Every brief here reads, designs, or specifies; none applies.
- **Re-deciding #900 / #903 / #942** — those are open human decisions; this stream maps the
  dependency and waits on their rulings.

## 5. The human decision this scope asks for

Approve `server-controls` as a stream (flip this header to `approved`, the README to
`active`), approve with changes, or reject. Until then the namespace is reserved and parked.
