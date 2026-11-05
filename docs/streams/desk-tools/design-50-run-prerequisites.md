# Desk-tools/50 — run prerequisite design and recorded ruling

**Status:** design approved on 29 October 2026; independent implementation verification and lifecycle acceptance remain pending.
**Decision issue:** https://github.com/medici-finance/assay/issues/4393
**Source checked:** main `7f61cff`, 5 November 2026 UTC; delivered implementation PR [#4392](https://github.com/medici-finance/assay/pull/4392).

The [decision record](../decisions/DR-desk-tools-50.md) transcribes the existing
[human ruling](https://github.com/medici-finance/assay/issues/4393#issuecomment-6505589048),
and the brief now cites it as `design:`. The ruling approves the design described
in PR #4517, including the local-record tamper boundary and earlier-unmarked-run
compatibility. It explicitly leaves independent implementation verification and
lifecycle acceptance pending. The approval-time snapshot and later clarifications
are distinguished in the record; this reconciliation adds no human ruling.

## Design

Keep prerequisite admission in the existing deskkit/preflight and verifier
attestation owners. `RunWorld` in `tools/desk/internal/deskkit/preflight.go`
records the selected source, declared siblings, pinned instrument byte digests,
desk-tools version, runtime and capability evidence. Admission distinguishes a
contradiction (hold) from an unreadable input (could-not-check), and records a
successful empty scan separately. A missing input must spend no Verify row.

Both deskdispatch prepare and attestation recovery call
`RequireVerifierWorld` before completing the stamp, then
`AdmitAndBindVerifierWorld` to derive and persist the same record from the bound
brief. `WorldSpecFromBrief` in `worldbrief.go` is the common derivation; callers
supply only commands that the existing deterministic runner executes. Shipped
(non-test) probes read local materialized inputs. Verifyloop uses existing authorized
envelope observations; deskdispatch prepare uses the completed attestation round
trip as its capability evidence, rather than a new App-scopes query.
No live cluster or production probe, new service, cache, credential scope or
per-row token mint is proposed.

`checkVerifierRun` in `verifierattestation.go` refuses a marked run without its
record and rechecks a bound record before admitting rows or Evidence. Rebinding
a run to a different digest refuses while the local binding remains intact; this
is not protection against a writer clearing that binding. The verifyloop per-root preflight and item
admission preserve healthy roots alongside named unreadable/skipped roots; its
native dispatch and landing consume the bound record. These records supplement
the existing source and independent landing guards rather than granting acceptance.

## Alternatives presented for design review

| Option | Why select or reject it | Consequence |
|---|---|---|
| A — retain the delivered local run-bound record in the existing owners (approved design) | Shares derivation and detects relevant local drift without adding another owner, process or production contact. | The ruling expressly includes the local-record boundary and earlier-run compatibility; independent implementation verification remains pending. Later review clarifications below are not a new human acceptance. |
| B — rely on boot preflight alone or merely record a prose environment banner | Does not bind the environment checked at boot to later row/Evidence admission. | Leaves stale successful preflight reusable after source/tool/sibling changes; does not satisfy Task 4. |
| C — create a fresh live probe or separate environment service for each row | May obtain fresher remote facts but duplicates custody, availability and policy boundaries. | Introduces external access and repeated costs outside this brief; requires separately authorized design. |
| D — require forge-anchored world metadata before accepting this design | Addresses the local-record tamper limit more strongly. | Changes the stamp format and compatibility path; requires a bounded amendment and re-review before proceeding. |

The record retains **major** as a declared desk default: a wrong prerequisite
decision can suppress verification or admit stale evidence, with recovery by a
fresh admitted run. The ruling states no severity; this is not a human stamp.

## Limits requiring explicit consideration

- `World` and `WorldRequired` remain local-only, outside the forge stamp. A writer
  of the verifier home's local attestation record can remove both fields and make
  any run appear to be an earlier unmarked run. Admission then takes the compatibility
  path without tool, sibling, instrument or runtime prerequisite rechecks. The
  forge-anchored source binding and dispatcher-stamp checks still apply. This is
  the concrete downgrade consequence of delivery security advisory
  [S2](https://github.com/medici-finance/assay/pull/4392#pullrequestreview-5645899147).
- The different-digest rebinding refusal shares that local boundary: clearing
  `World` permits a new local binding. It is not an independent tamper defence.
- Earlier unmarked runs with no record keep the documented compatibility behavior;
  absence alone does not impose the new requirement. This legitimate compatibility
  path also enables the local-record downgrade above.
- Delivery advisory [S3](https://github.com/medici-finance/assay/pull/4392#pullrequestreview-5645899147)
  records a latent sibling path: verifyloop's native `attestRun` prepares and
  recovers without `RequireVerifierWorld`. If `bindDispatchWorld` refuses, the
  retained home has a closed stamp but no mark or record; later admission of that
  home can take the same compatibility path. The shipped binary exposes only
  `verdict` and `plan`, neither reaching native dispatch. Before wiring dispatch
  to a shipped verb, mark gated runs before recovery and independently verify
  refusal after failed binding. This was omitted from the approval-time proposal;
  the ruling is not represented as explicit human acceptance of S3.
- Instrument lookup and desk-tools tag discovery use host state. Instrument
  bytes are bound by digest, but this is not isolated execution or a new custody guarantee.
- Prepared-run capability evidence is the completed attestation round trip,
  not a new App-scopes query. No additional permission is requested.
- Unstamped development builds or hosts lacking a compatible pinned instrument
  can now report could-not-check or skip a root rather than admit it.

## Proof and recovery

The unchanged brief version 2 owns the implementation checks: both
`TestFlowWorld` suites, exact delivering-head/base consumer corroboration for
all nine consumers, and the false-fixed-here negative control. The delivery also
contains `TestFlowWorldPreparePath` for prepare and recovery binding, moved
siblings and held/could-not-check runs. Their prior delivery results are not a
new independent verification receipt. A non-implementer must run the brief's
Verify table on the admitted source; no Evidence is added by this reconciliation.

For a changed environment, recover by admitting a new run; never overwrite the
old binding or reinterpret an unreadable prerequisite as a deliverable FAIL.
If option D is chosen, retain the hold while its bounded stamp-format amendment
is designed and reviewed.

## Recorded authority and remaining acceptance

The unedited own-login ruling on issue #4393 names **DR-desk-tools-50** and is
transcribed in the linked decision record. The issue's marker names this brief;
its `design:` binding connects that marker to the record under registers-v1
section 7.5 condition 6. No brief `ruling:` is added: this is a design-only
approval, not the separate implementation/lifecycle decision.

The existing issue's option 1 separately asks for implementation approval after
independent verification. That verification and lifecycle acceptance remain
pending exactly as the ruling says. The brief UUID, version, Task, Verify and
Evidence, and its board status, are preserved. No ready/draft change, release,
activation or pin change follows from this reconciliation.
