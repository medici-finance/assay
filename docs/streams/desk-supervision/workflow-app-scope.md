# The workflow App — scope, duties, and the sole-holder invariant

Status: implemented (desk-supervision/10). Design record:
[`DR-workflow-app-landing`](../decisions/DR-workflow-app-landing.md). Ruling: #1245 (option A —
provision, then confirm; a human merges the workflow-only PR).

The **workflow App** is the one identity in the fleet that may write `.github/workflows/**`. It is
a **capability that stands beside the desk-role Apps**, not one of the desk roles and not a tier:
no desk loop runs as it, and no desk role's token is ever minted from it.

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
3. **Run only under a human-initiated invocation.** Its credential is minted only inside an
   operator run or an explicit `workflow_dispatch`, never from a trigger an outside contributor can
   fire (`pull_request`, `pull_request_target`, `issue_comment`, `workflow_run`).

## The sole-holder invariant

**No other App holds `workflows: write`.** Every desk-role App — worker, desk, reviewer, verifier,
and any inbound-lane or loop App — has no `workflows` permission at all. The workflow App is the
only identity whose grant includes it.

The invariant binds `workflows: write` alone. The other three granted permissions are ordinary
and other Apps may hold them; only the power to rewrite CI is concentrated.

Why it matters: an identity that can write workflow files can rewrite what every check asserts.
Spread across several role Apps, that power multiplies the attack surface and stops being
auditable; held by one App, there is a single actor of record for every CI change.

## How it is checked

- **The grant (brief 10, Verify row 5).** With the workflow App's own installation token,
  `gh api /installation/permissions` returns exactly the four grants above and none of the
  withheld set. Only a human operator mints this token; no desk role reads its key.
- **The negative (brief 10, Verify row 6).** With each desk-role App's installation token,
  `gh api /installation/permissions` shows no `workflows` key.
- The two checks fail in different components: row 5 catches a widened grant on the workflow App;
  row 6 catches a second holder appearing elsewhere in the fleet.

## Where it is recorded

The workflow App's row in the App inventory, [`docs/adopting-assay.md`](../../adopting-assay.md)
§2, carries the same scope for adopters.
