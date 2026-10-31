# Desk-tools/50 — run prerequisite design proposed for ruling

**Status:** draft — proposed record DR-desk-tools-50; no approval or lifecycle transition.
**Decision issue:** https://github.com/medici-finance/assay/issues/4393
**Source checked:** main `125611d`, 26 October 2026; delivered implementation PR [#4392](https://github.com/medici-finance/assay/pull/4392).

This proposal prepares the design decision for the existing issue. It is not an
approved `decision-v1` register entry and the brief does not cite it as `design:`.
The existing implementation decision still requires independent verification;
approving this design does not assert that verification happened.

## Proposed design

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
supply only commands that the existing deterministic runner executes. Production
probes use local materialized inputs and existing authorized envelope evidence.
No live cluster or production probe, new service, cache, credential scope or
per-row token mint is proposed.

`checkVerifierRun` in `verifierattestation.go` refuses a marked run without its
record and rechecks a bound record before admitting rows or Evidence. Rebinding
a run to a different digest refuses. The verifyloop per-root preflight and item
admission preserve healthy roots alongside named unreadable/skipped roots; its
native dispatch and landing consume the bound record. These records supplement
the existing source and independent landing guards rather than granting acceptance.

## Alternatives and tradeoffs to decide

| Option | Why select or reject it | Consequence |
|---|---|---|
| A — retain the delivered local run-bound record in the existing owners (recommended for design review) | Shares derivation and detects relevant local drift without adding another owner, process or production contact. | Accepts the explicitly listed limits below; needs a human design ruling and independent implementation verification. |
| B — rely on boot preflight alone or merely record a prose environment banner | Does not bind the environment checked at boot to later row/Evidence admission. | Leaves stale successful preflight reusable after source/tool/sibling changes; does not satisfy Task 4. |
| C — create a fresh live probe or separate environment service for each row | May obtain fresher remote facts but duplicates custody, availability and policy boundaries. | Introduces external access and repeated costs outside this brief; requires separately authorized design. |
| D — require forge-anchored world metadata before accepting this design | Addresses the local-record tamper limit more strongly. | Changes the stamp format and compatibility path; requires a bounded amendment and re-review before proceeding. |

Proposed consequence is **major**: a wrong prerequisite decision can suppress
verification or admit stale evidence, with recovery by a fresh admitted run.
This severity is an author's proposal, not a human stamp.

## Limits requiring explicit consideration

- `World` and `WorldRequired` remain in the local attestation record rather than
  the forge stamp. Source identity has its existing forge binding, while the
  local prerequisite record has a different tamper boundary (delivery security
  advisory S2). Approval must not imply that all environment facts are forge anchored.
- An earlier run without `WorldRequired` retains the documented compatibility
  behavior: absence of a record alone does not impose the new requirement.
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
Verify table on the admitted source; no Evidence is added by this proposal.

For a changed environment, recover by admitting a new run; never overwrite the
old binding or reinterpret an unreadable prerequisite as a deliverable FAIL.
If option D is chosen, retain the hold while its bounded stamp-format amendment
is designed and reviewed.

## Human acts and subsequent binding

On the existing decision issue, the driver may post an unedited own-login
comment explicitly approving **DR-desk-tools-50**, requesting changes, or holding
it. A design-only approval must say that independent implementation verification
and lifecycle acceptance remain pending. The issue already marks this exact brief;
future `design: DR-desk-tools-50` binding would connect it to the record under
registers-v1 section 7.5 condition 6. Only after the own-login named approval exists
may a follow-up transcribe it into `docs/streams/decisions/DR-desk-tools-50.md` (planned)
with its real `ruling:` link and bind `design:`. A bot relay, PR merge, spec approval,
or bare option number supplies no such record approval.

The existing issue's option 1 separately asks for implementation approval after
independent verification. This proposal neither rewrites that question nor
claims its prerequisite is satisfied. No status, ready/draft state, approval,
release, activation or pin change follows from authoring this file.
