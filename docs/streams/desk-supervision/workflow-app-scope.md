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

**Open reading: key custody.** The ruling does not say who holds the App's private key, and
neither does the DR. Read that silence as an open question, not as permission: it neither allows
nor forbids a desk role holding the key. Who may mint the workflow App's token decides whether its
credentialled path is really human-initiated (duty 3 below), so the question is routed to
desk-supervision/11's gate, beside the enforcement question the DR's note raises.

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
   `issue_comment`, `workflow_run`). Neither a `workflow_dispatch` nor a desk run is by itself
   proof that a human started the run; the DR's note on this says where that property has to be
   enforced.

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
one of these places:

- `GET /app/installations/{installation_id}`, called under the App's JWT. Its `.permissions`
  is the accepted grant, `.events` the subscribed webhook events, and `.repository_selection`
  the installation scope. The App finds `{installation_id}` for a repository with
  `GET /repos/{owner}/{repo}/installation`, under the same JWT.
- `GET /orgs/{org}/installations`, read by an organization owner (it needs an org-owner read).
  Each entry carries the same `.permissions`, `.events` and `.repository_selection`, plus
  `.app_slug`, so one call both enumerates and reads every App installed in the organization,
  third-party Apps included, without holding any of their keys.
- The `permissions` object in the response to `POST /app/installations/{installation_id}/access_tokens`,
  the call that mints an installation token, but **only for an un-narrowed mint**: a request
  that carries no `permissions` or `repositories` body. A narrowed mint returns just the subset
  it asked for, so its `permissions` object can omit `workflows` while the installation holds
  `workflows: write`. A token minter that mints un-narrowed can record the object beside the
  token as a reading of the grant.

The two checks:

- **The grant (brief 10, Verify row 5).** For the workflow App's installation, `.permissions` is
  exactly the four grants above with none of the withheld set, `.events` is empty, and
  `.repository_selection` is recorded. The mint response carries no `.events`, so the
  zero-events half of the scope needs one of the two `GET` readings.
- **The negative (brief 10, Verify row 6).** For every other App installed on the repository
  (each desk-role App, and any inbound-lane, loop or single-purpose App), the granted set has no
  `workflows` key. The org-wide listing is the enumeration source; a reading from a narrowed mint
  does not count, and a set that cannot be enumerated is could-not-check, not a pass.
- The two checks fail in different components: row 5 catches a widened grant on the workflow App;
  row 6 catches a second holder appearing elsewhere in the fleet.

## Where it is recorded

The workflow App's row in the App inventory, [`docs/adopting-assay.md`](../../adopting-assay.md)
§2, carries the same scope for adopters.
