# The workflow App — scope, duties, and the sole-holder invariant

Status: the scope as ruled for desk-supervision/10, not yet confirmed against the live
installation (that is brief 10's Verify rows 5 and 6, which are still owed). Design record:
[`DR-workflow-app-landing`](../decisions/DR-workflow-app-landing.md), still `proposed` until the
ruling is recorded in it. Ruling: #1245, option 2 in brief 10 (provision, then confirm; the
ratified relay letters it "option A"), with a human merging the workflow-only PR.

The **workflow App** is the one identity in the fleet that may write `.github/workflows/**`. It is
a **capability that stands beside the desk-role Apps**, not one of the desk roles and not a tier:
it is no desk loop's default identity, and no desk role's token is ever minted from it.

## Permission set

Granted — exactly these four, and nothing else:

| Permission | Level | Why the duty needs it |
|---|---|---|
| `contents` | `contents: write` | push the workflow-only branch |
| `workflows` | `workflows: write` | create or update a `.github/workflows/*` file; GitHub refuses any App push touching that path without it |
| `pull_requests` | `pull_requests: write` | open the workflow-only PR — a forge act that `contents` and `workflows` do not cover |
| `metadata` | `metadata: read` | mandatory baseline for every GitHub App |

Webhook events: **none** subscribed.

Withheld — each checked as absent, not assumed:

- `administration` — the App cannot edit branch protection, rulesets, or their bypass lists, which
  is where the mixed-PR guard's required check lives (DR, accepted entry 5).
- `actions` — the App cannot dispatch, cancel, or delete workflow runs.
- `checks: write` and `statuses: write` — the App cannot post the verdicts that gate its own PR.
- `members` — the App cannot change who belongs to the organization or its teams.
- secrets and variables — the App cannot read or set repository or organization secrets/variables.

"Narrowly scoped" means this list: four grants, a named withheld set, zero events. Any other
permission appearing on the installation is drift.

**Open reading: installation scope.** The ruling fixes the permission set, not whether the App is
installed on selected repositories or on all of them. A `workflows: write` grant applies on every
repository the installation covers, so this is part of the concentration claim. It is read as the
installation's `repository_selection` (`selected` or `all`) in Verify row 5 and recorded with that
row's Evidence.

## Duties

1. **Author the workflow-only PR.** A workflow change travels as one PR whose diff touches
   `.github/workflows/**` (plus its own changelog fragment) and nothing else. The workflow App
   pushes that branch and opens that PR. The contract and the verb that does this are
   desk-supervision/11.
2. **Not merge it.** Merge authority for the first cutover is ruled **human-merges** (#1245): a
   human merges the workflow-only PR exactly as every other PR. The App holds
   `pull_requests: write` to OPEN the PR, not to land it. Widening this to identity-merges is a
   separate human decision, admissible only under the conditions the DR's final `accepted:` entry
   sets.
3. **Run only under a human-initiated invocation.** Its credentialled path runs only under an
   operator/desk run or an explicit `workflow_dispatch` (DR, accepted entry 6), never from a
   trigger an outside contributor can fire (`pull_request`, `pull_request_target`,
   `issue_comment`, `workflow_run`). A `workflow_dispatch` is not by itself proof that a human
   started the run; the DR's note on this says where that property has to be enforced.

## The sole-holder invariant

**No other App holds `workflows: write`.** Every other App — each desk-role App (worker, desk,
reviewer, verifier), any inbound-lane or loop App, and any single-purpose App — has no
`workflows` permission at all. The workflow App is the only identity whose grant includes it.

The invariant binds `workflows: write` alone. The other three granted permissions are ordinary
and other Apps may hold them; only the power to rewrite CI is concentrated.

Why it matters: an identity that can write workflow files can rewrite what every check asserts.
Spread across several role Apps, that power multiplies the attack surface and stops being
auditable; held by one App, there is a single actor of record for every CI change.

## How it is checked

GitHub has no `/installation/permissions` endpoint. An installation's granted set is read in
one of two places:

- `GET /app/installations/{installation_id}`, called under the App's JWT. Its `.permissions`
  is the accepted grant, `.events` the subscribed webhook events, and `.repository_selection`
  the installation scope.
- The `permissions` object in the response to `POST /app/installations/{installation_id}/access_tokens`,
  the call that mints an installation token. A token minter can record it beside the token.

The two checks:

- **The grant (brief 10, Verify row 5).** For the workflow App's installation, `.permissions` is
  exactly the four grants above with none of the withheld set, `.events` is empty, and
  `.repository_selection` is recorded. Only the first instrument returns `.events`, so the
  zero-events half of the scope needs it.
- **The negative (brief 10, Verify row 6).** For every other App installed on the repository
  (each desk-role App, and any inbound-lane, loop or single-purpose App), the granted set has no
  `workflows` key.
- The two checks fail in different components: row 5 catches a widened grant on the workflow App;
  row 6 catches a second holder appearing elsewhere in the fleet.

## Where it is recorded

The workflow App's row in the App inventory, [`docs/adopting-assay.md`](../../adopting-assay.md)
§2, carries the same scope for adopters.
