# loop-admin-execution-profile-v1 — qualified harness, model and backend profiles

**Status:** DRAFT, published for review. An extension of [`loop-admin-runner-v1`](loop-admin-runner-v1.md), negotiated through its `ext` and `require`. Part of the v1.0-draft specification set; breaking changes MAY be made without a major-version bump.
**Schema:** [`schemas/loop-admin-execution-profile-v1.json`](../schemas/loop-admin-execution-profile-v1.json)
**Reference implementation:** `loopadmin/runner/executionprofile.go`, the fake adapter's `Qualify`, and the conformance kit's profile builders, forged-declaration fixture and reference profile caller (`loopadmin/README.md`).

## 1. Purpose and boundary

A runner v1 adapter declares `resume` and `snapshot` as booleans. A caller cannot tell from them whether a resumed launch gets its files back, its conversation, its running processes, or only a fresh session in the old workspace. It also cannot pin the harness, model and backend a launch runs on, or learn that an adapter ran it somewhere else.

This extension makes those promises explicit before work is launched. An adapter declares **qualified profiles**: a harness, model and backend tuple, each bound to the qualification evidence it rests on, with every preservation, cancellation, budget and telemetry promise stated separately. A request pins one profile by digest and names what it needs from it. Anything the profile does not promise refuses the launch before Start. The adapter reports what it actually ran, and acceptance checks that report.

It defines no launcher, provider client, credential or installed profile. Qualification evidence and limits are opaque references, which the caller validates. A qualified profile, a receipt, a restored session or a healthy environment grants no scheduling, provider-write or acceptance authority.

## 2. Negotiation

The extension's name is `execution-profile/v1`.

- A request that needs it carries a **requirement** (section 4) under `ext["execution-profile/v1"]` and lists `ext:execution-profile/v1` in `require`. An adapter that does not declare the extension then refuses the request under runner v1 section 5. A request that needs stronger preservation than the v1 booleans MUST require the extension, so it can never be served by an old adapter.
- An adapter that knows the extension lists `execution-profile/v1` in its `extensions` and declares its qualified profiles (section 3).
- A request without the extension is a v1 request and runs exactly as before, on old and new adapters alike. It gets no execution report.
- A value carried but not required is a v1 optional extension: an adapter that does not declare the extension ignores it. An adapter that declares it honours it: the requirement is checked, and the launch is governed as if it were required.
- The v1 `resume` and `snapshot` capabilities keep their v1 meanings and imply nothing about preservation. No preservation dimension is inferred from another or from them.

## 3. Qualified profile

What an adapter declares it is qualified to run. Every field is required.

| Field | Meaning |
|---|---|
| `digest` | `sha256:` and the hex sha256 of the profile's compact JSON encoding (UTF-8, no insignificant whitespace, object members in the order this section and the schema list them) with `digest` itself empty. The reference `ProfileDigest` fixes the exact byte sequence. A declaration whose content no longer hashes to its digest is refused, so content cannot drift under an old digest. |
| `runner_profile` | The runner v1 `profile.id` this tuple serves. |
| `roles` | The role packets it is qualified for. |
| `harness` | `implementation`, `version`, the immutable harness-component `bundle_digest` and its `bundle_provenance` reference. |
| `model` | The model `profile` and the `model` it pins. This is a requested identity: what a run actually used is only ever the observation's `actual_model`. |
| `backend` | The execution backend `profile` and `platform`. |
| `preservation` | Per operation, what survives (section 5). |
| `cancellation` | Whether a cancel is confirmed by an observed stop. |
| `budget_scopes` | Which of `launch` and `request` it enforces. |
| `telemetry` | `none`, `partial`, `complete` or `unknown`. |
| `qualification` | `evidence`: references to the qualification it rests on (at least one). `limits`: the versions, platforms or conditions that qualification holds within. |

A declaration, a requirement and a report are each decoded as strictly as a runner v1 request (runner v1 section 3), the 1 MiB size limit included. A declaration with no `qualification.evidence` reference is refused.

**Support values.** Every promise is `supported`, `unsupported` (checked, and it does not) or `unknown` (could not check). Unknown and unsupported stay distinct. Both refuse a need: unsupported as an unsupported mandatory capability, unknown as an unknown capability. Neither is ever read as the other, or as supported.

## 4. Requirement

The request's value under `ext["execution-profile/v1"]`. It is decoded as strictly as a runner v1 request: exact keys, no repeated key, no null for a defined field, nothing trailing.

| Field | Meaning |
|---|---|
| `digest` | The pinned qualified profile. |
| `harness`, `model`, `backend` | The identities the caller pinned with it. |
| `needs` | What the launch cannot do without: `resume` and `restart`, each a set of preservation dimensions (section 5); `cancel_confirmation`; `telemetry`, `partial` or `complete`. The budget scope needed is the request's own `budget.scope`. |

**Admission.** Before Start, in order:

1. The requirement decodes and is well formed. A required extension with no value is refused.
2. The adapter declares a profile with the pinned digest; otherwise the request is refused as unsupported.
3. That declaration is well formed and its digest is the digest of its content.
4. The pinned harness, model binding and backend are the declared ones.
5. The request's `profile.id` is the profile's `runner_profile`, and its pinned `profile.model` is the profile's model.
6. The request's role is one the profile is qualified for.
7. The profile enforces the request's budget scope.
8. Each need is declared `supported`: cancellation confirmation, telemetry coverage, and every needed preservation dimension of each named operation.

A substituted harness, backend, model or runner profile, or a role it is not qualified for, refuses as a profile mismatch. A drifted declaration refuses the same way. The client runs admission before it records the invocation, so a refused launch leaves no invocation and starts nothing. An adapter that knows the extension MUST run admission again inside its own Start against its own declaration. A client that skipped or was misled about admission then still launches nothing unqualified.

## 5. Preservation

Preservation is declared per lifecycle operation, in the cell operator vocabulary:

- `resume`: paused to running, and runner v1's continuation of a pinned session (`resume` in the request).
- `restart`: drain, stop, start.

Each operation states, separately, whether these dimensions survive:

| Dimension | What survives |
|---|---|
| `files` | The workspace files. |
| `conversation` | The conversation history. |
| `process_memory` | The memory of the running harness process. |
| `descendants` | The processes the harness started. |
| `pending_interactions` | Questions, approvals or tool calls awaiting an answer. |

Restoring files is not restoring the conversation, and a rebound conversation is not continued processes. A launch that needs the conversation or process memory cannot be served by a profile that preserves files only, whatever its v1 `resume` and `snapshot` say. None of these observations renews authority, lifts a pause, or creates a new operation or a fresh budget.

## 6. Execution report

An adapter that ran a governed launch reports what it ran under `ext["execution-profile/v1"]` of each receipt and observation:

| Field | Meaning |
|---|---|
| `digest`, `harness`, `backend` | The profile, harness and backend it observed running. |
| `restored` | For a resumed launch, the five dimensions as observed restored, each `supported`, `unsupported` or `unknown`. |

The report is adapter-reported and untrusted. The model is not repeated here: the actual model is the observation's `actual_model`, checked by runner v1 acceptance.

**Acceptance** of a governed invocation adds to runner v1 section 7, after the model check:

1. A receipt or observation that reports another digest, harness or backend than the pinned one is refused, and the invocation can no longer be accepted.
2. The finished observation MUST carry a report. A missing report is refused as unreported, never read as the pinned tuple. A report that does not decode is a malformed result. A report that carries credential material is refused as credential material before it is read, under runner v1 section 7, and the invocation can no longer be accepted.
3. For a resumed launch whose requirement names `needs.resume`, the report MUST carry `restored`, and every needed dimension MUST be observed `supported`. A profile may declare more than one run restores; a run that restored only files is refused for a need of conversation or process memory, and an unreported restoration is unknown.

The accepted result carries the checked report.

## 7. The caller's own check

Admission and acceptance are the contract's checks. They do not know whether the profile is still the one the caller wants: a caller rotates a role to a new qualified profile, or retires one, after the launch was admitted. A caller MUST therefore check, when it acts on an accepted result, that the result's execution report names the qualified profile it currently holds for that role, and refuse a result with no report or another profile. That check reads the caller's own profile record, not anything the adapter or the contract reports, so it is independent of a skipped client check and a misreporting adapter. It is not independent of the caller's own record being wrong. The conformance kit's reference profile caller shows the check alone refusing a rotated-out or substituted profile with the contract bypassed. Each real caller MUST prove its own check in its own conformance.

## 8. Compatibility

- Old v1 requests are unchanged and accepted by old and new adapters.
- An old adapter refuses a request that requires the extension before Start; it never runs the launch on whatever it has.
- The receipt and observation `ext` maps are the adapter-side twin of the request's `ext`. An adapter emits a value there only for a request whose extension value it declares that it understands.
- Adding a preservation dimension, a support value or a field is a new version of the extension (`execution-profile/v2`), negotiated the same way.

## 9. Conformance

The conformance kit's `ProfileRefusedBeforeStart` case runs against any adapter: a request requiring a profile the adapter does not declare is refused before Start in both modes, and the same launch without the extension runs. Its `ProfileAdmittedByAdapter` case runs against any adapter that declares qualified profiles: with the client's check bypassed, the adapter itself refuses a request pinning a profile it does not declare, in both modes, and runs one pinning its own. The reference implementation's tests cover:

- v1 compatibility on old and new adapters;
- preservation per dimension in both modes, unknown kept distinct from unsupported, and partial restoration refused at acceptance;
- substitution of each bound identity and a drifted declaration;
- a substituted backend at run time and a report naming another profile;
- the adapter's own admission with the client check bypassed;
- the caller's own check with the contract bypassed;
- a report carrying credential material, on a receipt or an observation, refused, and the invocation never accepted afterwards;
- a declaration with no qualification evidence refused;
- the size limit on a declaration and a requirement;
- schema-to-source coverage, with planted drift detected;
- the malformed fixtures under `loopadmin/testdata/executionprofile/`.

Each named test has executable mutations in `loopadmin/testdata/mutations.json`.
