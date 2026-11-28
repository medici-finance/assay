# prepared-environment-v1 — prepared-environment receipts and the workspace port

**Status:** DRAFT, published for review. A companion of [`loop-admin-runner-v1`](loop-admin-runner-v1.md) and [`loop-admin-execution-profile-v1`](loop-admin-execution-profile-v1.md). Part of the v1.0-draft specification set; breaking changes MAY be made without a major-version bump.
**Schema:** [`schemas/prepared-environment-v1.json`](../schemas/prepared-environment-v1.json)
**Reference implementation:** `loopadmin/workspace` (types, validator and port), `loopadmin/workspace/fake` (in-memory adapters), fixture `loopadmin/testdata/workspace/ready-receipt.json` (`loopadmin/README.md`).

## 1. Purpose and boundary

A healthy process started in the wrong repository, at the wrong revision or with the wrong tools wastes work and produces misleading evidence. A runner v1 request carries its workspace as a string reference, so nothing on the request says what that workspace holds.

This contract makes readiness an explicit, checked result. A **receipt** records every mandatory input of one preparation and what an observer reported the workspace actually is. A **port** prepares, reconciles and releases instance workspaces through adapters the caller supplies, and decides readiness with one validator.

The port runs no subprocess, opens no network connection, keeps no store and embeds no Git engine or scheduler. Its adapters do those things. It owns no claim, candidate, acceptance or credential: candidate and acceptance identities are opaque references that their existing owners validate. A ready receipt grants no scheduling, provider-write or acceptance authority. The port never deletes anything: release hands a clean allocation to the existing retirement owner (for checkouts, the [`cell-workspaces-v1`](cell-workspaces-v1.md) lifecycle) or holds.

Key words MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119.

## 2. Receipt

| Field | Meaning |
|---|---|
| `version` | `prepared-environment/v1`. |
| `inputs.invocation` | The runner request's `caller` and `id`. |
| `inputs.owner` | The authority the preparation is held under: key, generation, owner incarnation and the validator that confirmed it. |
| `inputs.instance` | Cell, role, instance id and generation. |
| `inputs.repositories` | Each repository at an exact revision: 40 or 64 lowercase hex. A branch or tag name is not a revision. Names are unique. |
| `inputs.environment` | Recipe id, version and digest; tool and skill versions; the harness-component bundle digest; an optional image digest. |
| `inputs.writable` | The approved writable paths, clean and relative, with no backslash. Always a list: an instance with no writable paths records an empty list, never a missing one. |
| `inputs.policy` | References to the effective tool and network policies, never their content. |
| `inputs.candidate`, `inputs.acceptance` | Optional opaque references. |
| `inputs.restore` | Optional: the source instance and the continuity dimensions (`runner.PreservationNeed`) the restoration needs. |
| `allocation` | The shared cell preparation and the instance's own workspace. Both empty in a receipt written before the first adapter call. |
| `outcome` | `unknown`, `partial`, `not-ready` or `ready` (section 4). |
| `validation` | The validator, the observer whose report it compared, and one `{input, result}` check per mandatory input. |
| `artifacts` | Optional references (name, ref, sha256 digest) to what preparation produced. |
| `restored` | Set after a restoration: the source subject and the dimensions carried over. |

A receipt MUST NOT carry credential material in any string at any depth. Decoding is strict: exact keys, no repeated key, no null for a defined field, nothing trailing, at most `runner.MaxDocumentBytes`. Every refusal names the field path, never the value. The port stores a receipt with its observed outcome only after that receipt survives this same strict decode, so the port never reports as `ready` a receipt a caller cannot decode and admit.

A `ready` receipt MUST name both its preparation and its workspace and MUST record a `match` for each mandatory input: `repositories`, `recipe`, `tools`, `skills`, `harness_bundle`, `writable`, `policy`, and `image` when one is requested.

A receipt is **admitted** only for the invocation, owner, instance and workspace it records, and only when `ready`. A receipt for another instance (a swapped readiness), another invocation, another owner generation or another allocation is refused.

## 3. Instances and bindings

Readiness is bound to one instance at one generation: the binding key is cell, role, instance id and generation. Two instances of one role are two bindings; a successor generation is a new binding with its own workspace.

Instances of one cell whose environments are equal MAY share one cell **preparation** (built tools, skills, harness bundle, image). Their writable **workspaces** MUST stay separate.

A restoration names a source instance and the continuity it needs. The port checks each needed dimension against the environment's declared `runner.Preservation` with the execution-profile rules: `unknown` refuses with `ErrCapabilityUnknown`, anything else not `supported` with `ErrUnsupportedMandatory`. Neither falls back to a fresh workspace. The source MUST have a ready receipt for itself; files are restored into the new instance's own workspace, never the source's, and the receipt records the source subject and the preserved dimensions. A recorded restoration carries forward across a rebind only while the request asks for no other one; a changed source or changed needs restores again, and restoring into a workspace an earlier attempt allocated holds (`HoldUnpublished`) while it has unpublished or unknown files. The port does not check that the source shares the requester's cell or role, nor the source's own authority: the store and the supervisor that calls the port MUST fence which sources a requester may name.

## 4. Prepare and reconcile

1. Validate the request (shape and credential exclusion) before any adapter call.
2. Confirm with the authority adapter that the owner still holds the instance. A refusal prepares and changes nothing.
3. Check restoration continuity (section 3).
4. Read the binding's prior receipt from the caller-owned store. A stored receipt for another instance is refused.
5. **Reuse:** a `ready` prior receipt whose recorded inputs equal the request, invocation and current policy included, is re-observed (step 9) without reconfiguring. Anything else rebinds.
6. **Intent:** write an `unknown` receipt before the first adapter call, so an interruption leaves a record to reconcile.
7. Ask for the cell preparation and the binding's workspace; record the allocation as `partial`. A workspace other than the one the prior receipt recorded holds (`HoldUnknownAllocation`). A prior `partial` or `unknown` attempt is reconciled first: if its allocation cannot be observed, hold.
8. If the workspace's actual revisions differ from the request, check for unpublished files. Present or unknown: hold (`HoldUnpublished`) and keep the `partial` receipt. The port MUST NOT check out or re-clone over them. Otherwise check out, then apply the writable paths and policy.
9. **Observe:** read the effective configuration and actual revisions from the adapters and compare every mandatory input. `ready` only when all match; otherwise `not-ready` with the mismatched checks recorded, returned as `ErrNotReady`. An observation that cannot be made records `unknown` and holds; if the store cannot record that, the call returns `ErrAdapter`, since the store may still read an earlier outcome.

A changed environment within one binding asks for a new cell preparation but keeps the binding's workspace. Re-seeding that workspace from the new preparation is the resources adapter's job; the port only observes. A workspace that still reports the earlier environment stays `not-ready`, and the call fails closed rather than falling back to a fresh workspace.

An adapter failure is returned as `ErrAdapter`, whose text is the sentinel alone. It unwraps to the adapter's cause for `errors.Is` and `errors.As`, so callers MUST log only `Error()`, never an unwrapped cause.

Fake adapters report state and never decide readiness: the same validator compares their reports.

## 5. Release

Release never deletes. It holds with a typed `Hold` (`errors.Is(err, ErrHold)`) when there is no receipt for the binding (`HoldNoReceipt`), when the allocation expired (`HoldExpired`: an expired TTL never authorizes deletion), when the allocation is unknown or cannot be observed (`HoldUnknownAllocation`), or when the workspace holds unpublished or unknown files (`HoldUnpublished`). Only a clean, observed allocation is handed to the retirement owner, whatever outcome its receipt records. Release checks no authority of its own: the retirement owner MUST fence which caller may retire which instance. An uncertain retirement outcome is returned as `ErrAdapter` and not retried by any other path; there is no second pruner.

## 6. Compatibility

The runner v1 request is unchanged: `LaunchRequest.Workspace` stays a string reference, and a caller MAY pass a ready receipt's `allocation.workspace` there after admitting it. Callers that do not prepare through this port are unaffected. The execution-profile extension is unchanged; this contract reuses its preservation vocabulary and refusal rules. Two runner exports were added for reuse: `runner.DecodeStrict` (the request's strict decoder for a layered document) and `runner.CheckPreservation` (the profile's continuity check).

## 7. Conformance

The named tests in `loopadmin/workspace/workspace_test.go` and `loopadmin/runner/workspacebinding_test.go` are the conformance suite; each has an executable mutation in `loopadmin/testdata/mutations.json`. The schema test compares every object in the schema with its Go type and catches planted drift.
