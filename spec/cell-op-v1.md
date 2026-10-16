# cell-op-v1 — the cell-operation result and plan

**Status:** DRAFT, published for review. Part of the v1.0-draft specification set; breaking changes MAY be made without a major-version bump.
**Schema:** [`schemas/cell-op-v1.json`](../schemas/cell-op-v1.json)
**Reference implementation:** the `cellop` Go package (`tools/desk/internal/cellop`) and the cellctl adapter described in [`docs/cellctl-machine-seam.md`](../docs/cellctl-machine-seam.md).

## 1. Purpose and boundary

The cell-level operator verbs (`up`, `drain`, `pause`, `resume`, `restart`, `status`, `doctor`, `logs`, `gc`, `upgrade`) are driven from a terminal today and will later be served by a Cell API that the cockpit and other clients read. This contract defines, once, the two documents both surfaces use:

- the **result** (`cell-op-result`), what one invocation did;
- the **plan** (`cell-op-plan`), a sealed list of actions with the evidence each depends on.

The CLI emits them now. The Cell API serves the same documents later, and it is **read/plan-only**: it serves no apply. Neither adapter MAY add a field the other cannot carry.

Applying a plan is an **interactive-terminal confirmation only** (section 6). It is a guard against operator mistakes and concurrent races, **not an authorization control** (section 7). Approval-based, non-interactive and API apply are deferred (section 9) and nothing in this version implements any part of them.

Key words MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119.

## 2. Result

A result is one JSON object. Every member is always present; a value that was not measured is `null`, never a default such as `0`, `""` or `false`.

| Member | Rule |
|---|---|
| `schema` | Exactly `cell-op-v1`. |
| `kind` | Exactly `cell-op-result`. |
| `verb` | The verb as the CLI spells it. |
| `cell` | The cell the invocation named. |
| `started`, `finished` | RFC 3339 UTC times in whole seconds. |
| `outcome` | `ok`, `partial`, `refused`, `failed` or `could-not-check` (below). |
| `reason` | `null` when the outcome is `ok`; otherwise one word from the closed list below. |
| `detail` | A sentence for the operator, or `null`. |
| `roles` | One entry per role the verb looked at: `role`, `state` (string or `null`) and `fields`, an object of named readings whose values are strings or `null`. |
| `actions` | Every planned action with its `id`, `cell`, `outcome` (`ok`, `failed`, `not-run`) and `detail`. |
| `plan_id` | The plan this invocation made or applied, or `null`. |
| `next` | Commands the operator may run next, verbatim. |

**Outcomes and exit codes.**

| Outcome | Meaning | Exit |
|---|---|---|
| `ok` | Everything the verb set out to do is done. | 0 |
| `partial` | At least one action completed, and a later one failed or the sequence was stopped. | 1 |
| `failed` | An action failed before any action completed. | 1 |
| `refused` | Nothing was changed; `reason` says why. | 5 |
| `could-not-check` | A measurement the verb needed could not be taken; nothing was changed. | 6 |

**Read verbs** (`status`, `logs`) report per-field `null`s inside an `ok` result. They exit 6 only when the cell itself cannot be loaded.

**Reasons:** `not-interactive`, `desk-session`, `plan-not-found`, `plan-invalid`, `wrong-verb`, `foreign-cell`, `plan-expired`, `already-consumed`, `confirm-mismatch`, `plan-stale`, `unmeasured`, `action-failed`. A consumer that meets a reason it does not know MUST treat the result as not `ok`.

A successful apply:

```json
{
  "schema": "cell-op-v1",
  "kind": "cell-op-result",
  "verb": "drain",
  "cell": "example",
  "started": "2026-10-14T12:02:00Z",
  "finished": "2026-10-14T12:02:40Z",
  "outcome": "ok",
  "reason": null,
  "detail": null,
  "roles": [
    {
      "role": "worker-desk",
      "state": "drained",
      "fields": {}
    }
  ],
  "actions": [
    {
      "id": "drain-worker-desk",
      "cell": "example",
      "outcome": "ok",
      "detail": "worker-desk drained after its pass ended"
    }
  ],
  "plan_id": "4f1c2a9e7b3d5c8e0a6f2b1d",
  "next": []
}
```

A read verb with unmeasured fields, still `ok`:

```json
{
  "schema": "cell-op-v1",
  "kind": "cell-op-result",
  "verb": "status",
  "cell": "example",
  "started": "2026-10-14T12:00:00Z",
  "finished": "2026-10-14T12:00:00Z",
  "outcome": "ok",
  "reason": null,
  "detail": null,
  "roles": [
    {
      "role": "the-desk",
      "state": "running",
      "fields": {
        "last-pass": null,
        "pid": "41207"
      }
    },
    {
      "role": "worker-desk",
      "state": null,
      "fields": {
        "last-pass": null,
        "pid": null
      }
    }
  ],
  "actions": [],
  "plan_id": null,
  "next": []
}
```

## 3. Plan

| Member | Rule |
|---|---|
| `schema`, `kind` | Exactly `cell-op-v1` and `cell-op-plan`. |
| `id` | 24 lowercase hex characters, random. |
| `verb` | The verb that made the plan. A plan applies only through the same verb. |
| `cells` | Every cell the plan affects, sorted, without repeats, at least one. |
| `actions` | The actions in the order apply runs them. Each has an `id` unique in the plan, a `cell` from `cells`, a `summary`, and `preconditions`: the evidence it depends on, each a `key` and the `value` measured when the plan was made. Empty for a batch. |
| `children` | A batch's ordered child plans, each an `id` and a `digest` (section 4). Empty for any other plan. A plan carries actions or children, never both. |
| `created` | RFC 3339 UTC, whole seconds. |
| `expires` | Exactly 10 minutes after `created`. Fixed: no verb and no flag changes it. |
| `digest` | The sha256, lowercase hex, of the plan's RFC 8785 (JCS) canonical JSON with `digest` and `confirm_code` omitted. `confirm_code` is omitted with `digest` because it is derived from it. |
| `confirm_code` | The first 8 characters of `digest`, shown to the operator, who types it to apply. |

A plan carries no numbers and only fixed ASCII member names, so any RFC 8785 implementation and the reference canonicaliser produce the same bytes for it.

**Store.** A plan is stored at `<cell>/run/plans/<id>.json`, mode 0600, in a 0700 directory, by exclusive creation: a stored plan is never overwritten. A reader decodes it strictly (no unknown or case-variant members, no `null` lists, nothing after the object) and recomputes the digest; a plan that fails either is `plan-invalid`.

**Making a plan.** `<verb> --plan [--json]` measures, writes one plan and prints it (the document with `--json`, a readable rendering without), and changes nothing else. It works anywhere: off a terminal, from a desk, and through the Cell API. Making a plan is never state-changing.

```json
{
  "schema": "cell-op-v1",
  "kind": "cell-op-plan",
  "id": "4f1c2a9e7b3d5c8e0a6f2b1d",
  "verb": "drain",
  "cells": [
    "example"
  ],
  "actions": [
    {
      "id": "drain-worker-desk",
      "cell": "example",
      "summary": "pause worker-desk, then wait for its current pass to end",
      "preconditions": [
        {
          "key": "role/worker-desk/state",
          "value": "running"
        },
        {
          "key": "role/worker-desk/stop-flag",
          "value": "absent"
        }
      ]
    }
  ],
  "children": [],
  "created": "2026-10-14T12:00:00Z",
  "expires": "2026-10-14T12:10:00Z",
  "digest": "416ea0afb22adde60e51ccbc92d93a1d6ce2dc9087de681158c5936a9654f981",
  "confirm_code": "416ea0af"
}
```

## 4. Batch plans

A batch plan lists ordered child plans by `id` and `digest`; the children are ordinary stored plans. The batch's digest covers the children's digests, so the confirm code the operator types binds every step.

1. Apply loads every child and refuses `plan-invalid` if a child is missing, its digest differs from the batch's record, it is itself a batch, or it affects a cell the batch does not list.
2. **The batch plan itself is consumed atomically before its first child runs.** Of any number of concurrent applies of one batch, exactly one proceeds.
3. Each child is re-measured and consumed immediately before its own step.
4. **A child found already consumed stops the sequence**, as does a stale child or a step that would start after the batch's expiry. A sequence never skips a step and continues.
5. The batch expiry bounds the **start** of each step.

A stopped sequence leaves the batch consumed. Its result is `partial` if any action completed, otherwise the stop's own outcome.

```json
{
  "schema": "cell-op-v1",
  "kind": "cell-op-plan",
  "id": "7e6d5c4b3a2f1e0d9c8b7a6f",
  "verb": "restart",
  "cells": [
    "example"
  ],
  "actions": [],
  "children": [
    {
      "id": "9a0b1c2d3e4f5a6b7c8d9e0f",
      "digest": "67c8ea19ce3028ae239c44c614187639ca21faf87fa7d933ec0400be25b05016"
    },
    {
      "id": "1b2c3d4e5f6a7b8c9d0e1f2a",
      "digest": "685f693f275d923d6ebfc6df6f18d6b158bf4646f436a76d71d5e10189080457"
    }
  ],
  "created": "2026-10-14T12:00:00Z",
  "expires": "2026-10-14T12:10:00Z",
  "digest": "90c4cf89cde72b8cb1837436af8cfedf754c7e079be6c7f2dc5a615fa8d1d04c",
  "confirm_code": "90c4cf89"
}
```

The two children it names:

```json
{
  "schema": "cell-op-v1",
  "kind": "cell-op-plan",
  "id": "9a0b1c2d3e4f5a6b7c8d9e0f",
  "verb": "drain",
  "cells": [
    "example"
  ],
  "actions": [
    {
      "id": "drain-worker-desk-now",
      "cell": "example",
      "summary": "pause worker-desk and abandon its in-flight pass, recording each item as an unknown outcome",
      "preconditions": [
        {
          "key": "role/worker-desk/state",
          "value": "running"
        }
      ]
    }
  ],
  "children": [],
  "created": "2026-10-14T12:00:00Z",
  "expires": "2026-10-14T12:10:00Z",
  "digest": "67c8ea19ce3028ae239c44c614187639ca21faf87fa7d933ec0400be25b05016",
  "confirm_code": "67c8ea19"
}
```

```json
{
  "schema": "cell-op-v1",
  "kind": "cell-op-plan",
  "id": "1b2c3d4e5f6a7b8c9d0e1f2a",
  "verb": "up",
  "cells": [
    "example"
  ],
  "actions": [
    {
      "id": "start-worker-desk",
      "cell": "example",
      "summary": "start worker-desk in a new window",
      "preconditions": [
        {
          "key": "role/worker-desk/model-pin",
          "value": "sonnet"
        }
      ]
    }
  ],
  "children": [],
  "created": "2026-10-14T12:00:00Z",
  "expires": "2026-10-14T12:10:00Z",
  "digest": "685f693f275d923d6ebfc6df6f18d6b158bf4646f436a76d71d5e10189080457",
  "confirm_code": "685f693f"
}
```

## 5. State-changing invocations

These invocations change state, and every one of them is refused off a terminal:

- `up --apply` (reconcile);
- `drain`, `pause`, `resume` and `restart`, in every form except `--plan`;
- `doctor --apply`;
- `gc --run` and `gc --apply`;
- `upgrade --apply`;
- any future `--apply`.

Each registers the annotation `cellop:state-changing`: on the command when every form except `--plan` changes state, on the flag otherwise. Apply is reachable **only** through these annotated invocations. `drain`, `pause`, `resume` and `restart` apply on a terminal after the typed confirm.

**Read-only:** `status`, `logs`, bare `doctor`, bare `gc`, bare `upgrade`, and any `--plan`. Bare `doctor` and bare `upgrade` never apply and never write: they print what they would do and write no plan. Their plans come only from `doctor --plan` and `upgrade --plan`, and apply only through `--apply`.

**Lifecycle primitives `up` and `down` are allowed off a terminal.** `up` only starts roles that are not running; reconciling stale state needs `up --apply`, a state-changing invocation. `down` stops the cell (cadence roles stop at a pass boundary; interactive sessions are ended with the cockpit session), and `down <cell> <role...>` is the per-role stop that refuses unless each role is already paused or drained. Off a terminal, `down` followed by `up` approximates a restart, so the off-terminal refusals of `restart` and `drain --now` are mistake guards against an accidental composite or abrupt stop. They are **not** a security boundary.

## 6. Apply

Every state-changing invocation requires stdin and stdout to be terminals. Off a terminal it is refused `not-interactive` (exit 5), and the `--plan` form still works. On a terminal, apply:

1. shows the plan, its cells and its `confirm_code`;
2. requires the operator to type that code, refusing `confirm-mismatch` otherwise;
3. re-measures every precondition, refusing `plan-stale` (exit 5) if any value changed, or `could-not-check` (exit 6) if one cannot be measured, and in both cases **does not consume** the plan;
4. **consumes atomically**, by exclusive creation of `run/plans/consumed/<plan-id>`, refusing `already-consumed`;
5. only then runs the actions.

Before the confirm, apply also refuses a plan that is missing (`plan-not-found`), invalid (`plan-invalid`), made by another verb (`wrong-verb`), does not list the invoking cell (`foreign-cell`), is past its expiry (`plan-expired`, checked again after the confirm) or is already consumed. A failure after the consume leaves the plan consumed, with a `failed` or `partial` result.

## 7. What the confirm is, honestly

The confirm, the re-measure and the atomic consume are a guard against operator mistakes and concurrent races. **They are not an authorization control.** A process running under the operator's own account can drive a terminal, and it can delete the consumed marker. A deleted marker allows another interactive apply of the same plan within its remaining expiry, still subject to the re-measure; after the expiry the plan is refused. Every state-changing verb's `--help` states this limit.

**Single point of failure, today:** none behind the operator's own account. A state change that a desk starts has no second authorization layer. Isolating desks from the operator's account is the runtime-isolation owner's job.

**Desk-driven apply is out of contract.** A desk driving a state-changing invocation through a pseudo-terminal (for example `script` or `expect`) is outside this contract. As a mistake guard only, never as a control, every state-changing invocation refuses `desk-session` when any desk-session marker is set in its environment: `ASSAY_CELL`, `DESK_LOOP` or `DESK_SESSION`. A determined process can unset them. An operator shell that has inherited a cell's environment is refused too, and must clear those variables first.

## 8. No blanket consent

There is no `--yes` and no flag that skips the confirm, the re-measure or the consume. No command is named `approve`, and no flag is named `--approval`, `--yes`, `-y`, `--assume-yes` or `--skip-verify`.

## 9. Deferred

Not built in this version, and not to be built in part: an operator key, a key registry, enrollment, a signed approval payload, `cellctl approve`, `--apply --approval`, and any non-interactive or API apply. The future approval design and the five properties that gate ever enabling it are recorded in [`docs/proposals/non-ci-architecture/21-cell-operator-surface.md`](../docs/proposals/non-ci-architecture/21-cell-operator-surface.md) §Approvals: deferred.

## 10. Conformance

An implementation conforms when:

- every result and plan it emits validates against [`schemas/cell-op-v1.json`](../schemas/cell-op-v1.json), and every plan's digest recomputes from its canonical form;
- of any number of concurrent applies of one plan, at most one runs its actions;
- a plan whose precondition changed is refused `plan-stale` and stays unconsumed;
- an expired plan and another cell's plan are refused;
- a batch is consumed before its first child, and a consumed child stops the sequence;
- every state-changing invocation is refused off a terminal and with a desk-session marker set, while its `--plan` form succeeds off a terminal;
- no approval or blanket-consent surface exists.

The reference implementation's tests for each item are listed in [`docs/cellctl-machine-seam.md`](../docs/cellctl-machine-seam.md).
