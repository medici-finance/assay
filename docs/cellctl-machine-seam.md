# The cellctl machine seam

How `cellctl` speaks the cell-operation contract, [`cell-op-v1`](../spec/cell-op-v1.md): what a verb adopting it does, how a plan is made and applied, and what the tests check. The contract itself (documents, reasons, exit codes, the apply order and its stated limit) is the spec; the JSON Schema is [`schemas/cell-op-v1.json`](../schemas/cell-op-v1.json). Command-line forms are in [the `cellctl` command reference](cellctl-cli-reference.md).

`up` has adopted the contract (`--plan`, an annotated `--apply`, `--json`; see *The `up` verb* below). The other verbs adopt it in their own briefs, with their own rows. Until they do, the mechanics are exercised end to end by a fixture verb that exists only in a test build (below).

## The three forms

A verb that adopts the contract has up to three forms.

| Form | Off a terminal | In a desk session | Changes |
|---|---|---|---|
| `<verb> <cell> --plan [--json]` | works | works | writes one plan to `<cell>/run/plans/`, nothing else |
| `<verb> <cell> --apply <plan-id> [--json]` | refused `not-interactive` | refused `desk-session` | runs the plan after the typed confirm |
| a state-changing form without `--plan` (`drain`, `pause`, `resume`, `restart`, `gc --run`) | refused `not-interactive` | refused `desk-session` | makes a plan, then applies it as `--apply` does |

`--json` prints the plan document (`--plan`) or the result document (every other form) on stdout. Without it a plan prints as a short rendering ending in the apply command line, and a result prints as one line, on stdout when it is `ok` and on stderr otherwise, then one line per action and one per `next` command.

The exit code is the result's: 0 `ok`, 1 `partial` or `failed`, 5 `refused`, 6 `could-not-check` (spec §2).

A plan made off a terminal, by the fixture verb:

```json
{
  "schema": "cell-op-v1",
  "kind": "cell-op-plan",
  "id": "7c68e5630cdcab44d18df158",
  "verb": "cellop-fixture",
  "cells": [
    "example"
  ],
  "actions": [
    {
      "id": "cellop-fixture",
      "cell": "example",
      "summary": "fixture: cellop-fixture on example",
      "preconditions": [
        {
          "key": "fixture/state",
          "value": "absent"
        }
      ]
    }
  ],
  "children": [],
  "created": "2026-10-15T01:46:23Z",
  "expires": "2026-10-15T01:56:23Z",
  "digest": "5041f675752af6542783cb7abe12635bd3563c6b1fa9fecdc0be4cadb1931c03",
  "confirm_code": "5041f675"
}
```

And the result of a state-changing form run without a terminal (`drain example --json` with stdin from `/dev/null`). Nothing was loaded and nothing changed; `next` names the form that works there:

```json
{
  "schema": "cell-op-v1",
  "kind": "cell-op-result",
  "verb": "drain",
  "cell": "example",
  "started": "2026-10-15T01:46:23Z",
  "finished": "2026-10-15T01:46:23Z",
  "outcome": "refused",
  "reason": "not-interactive",
  "detail": "this invocation changes state and needs a terminal on stdin and stdout; the --plan form works anywhere",
  "roles": [],
  "actions": [],
  "plan_id": null,
  "next": [
    "cellctl drain example --plan"
  ]
}
```

## How apply runs

`--apply` hands the stored plan to `cellop.Session.Apply` (`tools/desk/internal/cellop/apply.go`), which keeps the spec's order (§6):

1. refuse `not-interactive` unless stdin and stdout are terminals, and `desk-session` when `ASSAY_CELL`, `DESK_LOOP` or `DESK_SESSION` is set;
2. load the plan strictly and recompute its digest; refuse `wrong-verb`, `foreign-cell`, `plan-expired` and `already-consumed`;
3. show the plan, its cells and its confirm code on the terminal, and read one typed line; anything but the code is `confirm-mismatch`;
4. re-measure every precondition: a changed value is `plan-stale`, a measurement that fails is `could-not-check`; nothing is consumed;
5. consume, by exclusive creation of `<cell>/run/plans/consumed/<plan-id>`: exactly one of any number of concurrent applies gets past this step;
6. run the actions. A failure after step 5 leaves the plan consumed, with a `failed` or `partial` result.

A batch plan is consumed at step 5 before its first child runs; each child is then re-measured and consumed just before its own step, and a child found already consumed, stale or past the batch's expiry stops the sequence (spec §4).

**What the confirm is.** It guards against mistakes and races. It is not an authorization control: a process under the operator's account can drive a terminal, and can delete a consumed marker, which allows one more interactive apply of the same plan before it expires, still subject to the re-measure. The desk-session refusal is a mistake guard too; a desk that drives a state-changing form through a pseudo-terminal is out of contract (spec §7). There is no flag that skips the confirm, the re-measure or the consume, and no approval mechanism (spec §8 and §9). Every annotated verb's `--help` ends with this limit.

## Adopting the contract in a verb

A verb brief does four things in `tools/desk/cmd/cellctl`; the helpers are in `cellop_cli.go`.

1. **Annotate.** Every state-changing invocation registers the cobra annotation `cellop:state-changing`: `markStateChanging(cmd)` when every form except `--plan` changes state (`drain`, `pause`, `resume`, `restart`), `markStateChangingFlag(cmd, "<flag>")` when only the form with that flag does (`up --apply`, `doctor --apply`, `gc --run`, `gc --apply`, `upgrade --apply`). Both append the confirm's stated limit to the verb's help. A flag named `--apply`, `--run`, `--now` or `--foreign` is always annotated, on the flag or on its command.
2. **Plan.** `--plan` measures and calls `cellopPlan(cmd, cell, verb, actions)`, giving each action the preconditions it depends on as key and value pairs.
3. **Apply.** `--apply <id>` (and an annotated form without `--plan`, after `cellopNewPlan`) calls `cellopApply(cmd, cell, cellopVerb{Verb, Measure, Run, After}, id)`. `Measure` re-takes an action's preconditions; `Run` performs it. `After`, when set, runs once on the result before it is printed (`up` launches the windows there and adds one entry per role). `cellopApply` is the only place a `cellop.Session` is built, and it refuses an invocation that is not annotated.
4. **Read forms stay read-only.** Bare `doctor`, bare `gc` and bare `upgrade` print what they would do and write no plan.

The root command's pre-run gate (`gateStateChanging`) refuses an annotated invocation off a terminal or in a desk session before the verb loads anything, and `Session.Apply` checks both again.

When a verb lands, its stand-in in the fixture build (below) steps aside for it, and the flag-walk test then checks the real verb's annotations.

## The `up` verb

`cellctl up <cell>` converges the cell (see [the command reference](cellctl.md#cellctl-desk-up-down)). Its three forms:

- **bare**: starts the roles that are not running, one window at a time, and prints a table with one row per targeted role (`up`, `already-up`, `reconciled+up`, `failed`). A running role is never relaunched; a stale one is a `failed` row naming `up --plan`. Allowed off a terminal. With `--json` the same result is a `cell-op-result` with one entry per role and nothing else on stdout.
- **`--plan`**: writes ONE plan for the whole cell, one `reconcile-<role>` action per stale role (and `reconcile-comms` for stale comms state). Each action's preconditions are the role's measured disposition and stale class. A cell with nothing stale writes no plan and returns an ok result. Works off a terminal.
- **`--apply <plan-id>`** (annotated, terminal only): shows the plan, takes the typed confirm, re-measures every action and runs it, then launches the targeted roles. A role that came alive since the plan is refused `plan-stale` (exit 5) naming the role, and nothing is launched. Alive and could-not-check records are never planned.

## The fixture build

`cellop_fixture.go` is compiled only with `-tags cellopfixture`. It adds:

- `cellctl cellop-fixture <cell> [--plan [--json] | --apply <plan-id>]`, whose one precondition is the content of `<cell>/run/cellop-fixture/state` and whose one effect appends a line to `<cell>/run/cellop-fixture/effects`;
- a stand-in for each verb of the enumerated list that is not registered yet (`drain`, `pause`, `resume`, `restart`, `doctor`, `gc`, `upgrade`), annotated as step 1 says.

With `CELLOP_FIXTURE_TRACE` set, every arrival at the apply path appends the command path and the flags given to that file. The release build has none of this: `cellctl cellop-fixture` is an unknown command there.

## Tests

| Test | Build | What it holds |
|---|---|---|
| `go -C tools/desk test ./internal/cellop/ -count=1` | release | exit codes, JCS digests, store modes and strict decode, the apply order, 16 concurrent applies running once, re-measure, expiry and foreign cell, the consumed-marker bound, batch consume-first-and-stop, and every JSON example in the spec and this page against the schema |
| `TestNoApprovalSurface`, `TestOneApplyPath`, `TestConsentTreeChecks` in `cmd/cellctl` | release | no `approve`, `--approval`, `--yes`, `-y`, `--assume-yes` or `--skip-verify` anywhere in the tree; `cellop.Session` built only in `cellop_cli.go`; the tree checks catch planted gaps |
| `TestEveryStateChangingInvocationRequiresTTY` | `cellopfixture` | the enumerated list is registered and annotated; every annotated invocation is refused off a terminal (stdin, stdout or both not a terminal) without reaching apply; every `--plan` works off a terminal; every form of every adopting verb, run on a pseudo-terminal, reaches apply only when annotated; exempting any one invocation is caught |
| `TestApplyRefusesDeskSession` | `cellopfixture` | each annotated invocation with each desk-session marker is refused on a terminal |
| `TestPlanApplyRoundTrip` | `cellopfixture` | the built binary: `--plan --json` off a terminal, `--apply` with the typed code on a pseudo-terminal, a second apply refused `already-consumed`, a wrong code and a stale plan each refused without consuming |

The tagged rows need a pseudo-terminal (macOS or Linux):

```sh
go -C tools/desk test -tags cellopfixture ./cmd/cellctl/ -run 'TestEveryStateChangingInvocationRequiresTTY|TestApplyRefusesDeskSession|TestPlanApplyRoundTrip' -count=1
```

The mutations that each must fail the suite are in `tools/desk/internal/cellop/mutations.json`, for `go -C tools/desk run ./cmd/muhar -spec internal/cellop/mutations.json`.
