# loop-admin-runner-v1 — the shared runner protocol

**Status:** DRAFT, published for review. Part of the v1.0-draft specification set; breaking changes MAY be made without a major-version bump.
**Schema:** [`schemas/loop-admin-runner-v1.json`](../schemas/loop-admin-runner-v1.json)
**Reference implementation:** the `loopadmin/runner` Go package (`loopadmin/README.md`), with an in-memory fake adapter and an offline conformance kit.

## 1. Purpose and boundary

Every launch of a model-backed agent, by a standing desk or by a workflow stage, goes through one checked contract so that provider integrations, crash recovery and cost accounting are implemented once. The contract has two sides: a **caller API** (start, observe, cancel, reconcile, accept a result) and an **adapter API** that a concrete harness integration implements.

The contract is protocol and checking only. It does not select queue work, advance a workflow node, mint role credentials, approve a result, journal launches durably, or offer operator administration. It owns no work identity and no claim: the caller's own identifiers and authority are carried through it by reference. A conforming implementation MUST NOT depend on a workflow definition, an instance store or any graph type; a desk request carries none of them.

Key words MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119.

## 2. Four rules

1. **Unknown stays unknown.** A lost launch acknowledgment, a missing usage figure and an unconfirmed cancellation are reported as unknown, requested and absent respectively. They MUST NOT be rounded to failure, zero or stopped.
2. **Process exit is not acceptance.** An adapter reports a terminal observation. Only the caller API's result acceptance, after the generation fence and the result checks of section 7, turns it into an accepted result, and an accepted result is still not the caller's decision to take the work (section 8).
3. **No silent fallback.** The request pins the model and the complete tool list. An observation from another model, a tool request outside the list, or a resume the adapter cannot do, is refused. A different model or a fresh session is never substituted.
4. **Refuse what you do not understand.** A mandatory field or capability the consumer does not provide, or does not define, refuses the request instead of downgrading it.

## 3. Launch request

A launch request is a JSON object, decoded strictly: a field this version does not define is an error, because an unknown field might be mandatory and a v1 consumer cannot tell. Optional additions travel under `ext`.

| Field | Rule |
|---|---|
| `version` | MUST be exactly `loop-admin-runner/v1`. Any other value is refused. |
| `caller`, `id` | The stable request identity `(caller, id)`. The same pair always means the same request; reusing it with different content is refused. |
| `mode` | `standing-desk` or `workflow-stage`. |
| `desk` | `{binding_id}`. Required in standing-desk mode, forbidden in workflow-stage mode. |
| `work` | `{work_id, node_id, attempt_id}`: references to the existing canonical identities, never replacements for them. Required, all three, in workflow-stage mode; forbidden in standing-desk mode. |
| `packet` | `{role, ref, hash?, trust}`: the role packet by reference, with the trust of its source (`operator`, `caller` or `untrusted`). |
| `profile` | `{id, model, skill?, tools}`: the operator-approved pinned profile. `tools` is the complete list the invocation may request; an empty list pins none and an absent list is invalid. |
| `workspace` | A workspace reference. |
| `authority` | `{key, generation, validated_by}`: the externally validated claim or ownership generation (at least 1) the launch runs under. The contract neither mints nor validates it. |
| `budget` | `{id, scope, max_tokens?, max_cost_micros?}`: a reference to the caller's reservation. `scope` is `launch` or `request`; at least one limit MUST be positive. |
| `resume` | Optional `{session_id, role, profile_id}`, pinned to the request's own role and profile. |
| `require` | Mandatory capabilities and extensions, section 5. |
| `ext` | Optional extensions, ignored unless named in `require` as `ext:<name>`. |

**Mode exclusivity is strict.** A standing-desk request carrying a `work` object, and a workflow-stage request carrying a `desk` object, are both refused. This keeps a desk from being launched under an invented workflow instance and keeps a workflow stage from borrowing a desk binding.

**Credential exclusion.** No request field, extension key or extension value may carry credential material. A consumer refuses a request in which any identifier, reference or extension looks like a credential (a token, a private key, an authorization header), and refuses extension keys that name one. Credentials are the adapter's own concern, supplied out of band; they never travel in a packet, a request or a result.

## 4. Invocation states

| State | Meaning | Holds the authority |
|---|---|---|
| `unknown` | The outcome is not known (acknowledgment lost, adapter unreachable). | yes |
| `running` | The adapter reports it running. | yes |
| `cancel-requested` | A cancel was requested. It is not a confirmed stop. | yes |
| `finished` | The adapter reports a terminal result. This is not acceptance. | no |
| `failed` | The launch definitely did not start, or definitely ended without a result. | no |
| `stopped` | An observation confirmed it stopped. | no |
| `absent` | Adapter-side only: reconcile found no record of the identity. | n/a |

An invocation in a holding state owns its authority key: no other invocation MAY start under the same `authority.key` until it leaves the holding states. A state value the contract does not define is treated as `unknown`.

## 5. Capabilities

An adapter declares what it can do; the zero declaration declares nothing, so every optional facility is opt-in.

| Declaration | Meaning |
|---|---|
| `resume` | Can continue a pinned session. |
| `cancel_ack` | Its cancel acknowledgment is meaningful. Even so, an acknowledgment is a request received, never a stop. |
| `budget_scopes` | Which of `launch` and `request` it can enforce. A request whose budget scope is not in the list is refused. |
| `telemetry` | `none`, `partial` or `complete`. With `none`, the consumer treats every usage figure as unknown. |
| `snapshot` | Can expose a workspace or session snapshot. |
| `launch_dedupe` | `start` is idempotent on `(caller, id)`, and a lookup of an unseen identity is authoritative. |
| `extensions` | The `ext:<name>` extensions it understands. |

A request names what it cannot do without in `require`: `resume`, `cancel-ack`, `budget-launch`, `budget-request`, `telemetry`, `snapshot`, `launch-dedupe`, or `ext:<name>`. A name the adapter does not declare, and a name this version does not define, both refuse the request. The same check runs for both modes. A resume the adapter cannot do is refused even when it was not listed in `require`; a fresh session is never started in its place.

## 6. Operations

**Start.** The consumer records the invocation as `unknown` before asking the adapter to start, so no launch exists that the caller does not know about. The adapter answers with a receipt, a definite failure, or anything else. Only an error the adapter explicitly marks as a definite failure moves the invocation to `failed` and frees the authority. Any other error, including a timeout or a lost acknowledgment, leaves it `unknown`, refuses every further launch under that authority, and requires reconcile. A retry of the same `(caller, id)` while `unknown` is refused the same way.

**Observe.** Reports the adapter's state. An adapter error leaves the recorded state unchanged: not being able to look is not an outcome, and the caller is told reconcile is required.

**Reconcile.** Looks an invocation up by its stable identity without needing a receipt. A report of `absent` settles an `unknown` launch as `failed` only when the adapter declares `launch_dedupe`; otherwise it stays `unknown` and held.

**Cancel.** Requests a stop. The invocation becomes `cancel-requested` and keeps holding its authority. Only an observation of `stopped` confirms the stop. An acknowledgment is reported only when the adapter declares `cancel_ack`. An adapter that acknowledges a cancel and keeps running is observed as still running, cannot have its result accepted, and cannot be replaced.

## 7. Results and acceptance

An observation carries `state`, `usage`, `actual_model`, and for a terminal state a `result`. A result carries the `generation` the attempt ran under, an `outcome` (`success` or `failure`), a summary, artifacts and tool requests.

- **Artifacts** are by reference and sha256. Their trust is always `untrusted`; an adapter cannot raise it, and model output stays untrusted until a caller's own check accepts it.
- **Tool requests** carry a name and an optional note. The note is the model's own text and carries no authority: authentication and role enforcement never depend on model text.
- **Usage** has `input_tokens`, `output_tokens` and `cost_micros`. An absent figure is unknown; a measured `0` is a real reading. A total over invocations is unknown for a figure unknown in any of them. Usage from an adapter that declares `telemetry: none` is treated as unknown whatever it reports.

Result acceptance is the one place an observation becomes an accepted result. It applies, in order:

1. The invocation MUST be `finished` and hold a result.
2. **The generation fence.** The caller's current generation for `authority.key` is read. If it is not the generation the attempt ran under, the result is refused however well formed it is. If it cannot be read, the result is refused: the fence fails closed.
3. The observed model MUST be the pinned model.
4. The result MUST be well formed (outcome, artifacts, hashes, trust) and carry no credential material.
5. The result's own `generation` MUST equal the attempt's generation.
6. Every tool request MUST name a tool in the pinned profile.

A process exit, a `finished` state or a `success` outcome is never acceptance by itself.

## 8. The caller's own check

The generation fence in section 7 is a single control. It depends on the contract implementation reading the caller's record correctly, and an adapter or a mistaken fence implementation can report a fenced attempt's result as current. A caller MUST therefore recheck its own authority record (a desk's claim generation, a controller's attempt generation) at the moment it acts on an accepted result, and refuse the result if that generation is no longer the one the attempt ran under, independently of the contract's fence. The two checks fail on different signals in different components; neither stands in for the other. The conformance kit ships a fixture that bypasses the fence to show the caller's check alone still refuses.

## 9. Compatibility

- A consumer MUST refuse a `version` it does not implement.
- Adding an optional field under `ext`, a new capability that no request marks mandatory, or a new `ext:<name>` extension is compatible. Adding a field outside `ext`, a new state, or a new mandatory rule is a new version.
- A producer that needs a new field to be honoured MUST list it in `require`; a v1 consumer then refuses the request rather than ignoring the field.
- Provider-specific behaviour belongs in an adapter, never in the contract. This version defines no real provider client and makes no provider call.

## 10. Conformance

An adapter conforms when the offline kit passes against it. The kit drives the caller API over an adapter and its fault control, and covers: lost launch acknowledgment; late output from a fenced attempt; malformed result; unauthorized tool request; credential in a request or a result; missing cost; cancelled but still running; unsupported resume; model fallback; and both modes, with no graph fields for a desk and refusal of a workflow stage missing a canonical reference. A mutation gate shows each named test fails when the behaviour it guards is removed.
