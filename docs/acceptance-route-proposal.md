# Separate acceptance adjudication route — proposal for #4993

Status: **proposed, not ratified or implemented**. This document supplies no
verification credit, acceptance authority or scan exemption. It does not make
verification-handoff/03 ready for final acceptance.

## Recorded authority and remaining boundary

The [human ruling on #4995](https://github.com/medici-finance/assay/pull/4995#issuecomment-6530834192)
declined the scan exception, retained the full hold scan, required a separate
route for the verification-handoff/03 card, and prohibited rewording Evidence
to pass the scan. The closed implementation is terminal. This proposal preserves
that ruling and the merged Evidence in [#4608](https://github.com/medici-finance/assay/pull/4608)
and [#4621](https://github.com/medici-finance/assay/pull/4621).

The current source has two independent refusals. `verifyIssues` declines to
emit an implemented brief's acceptance card when
`verifyPassHeldContradiction` finds an unresolved signal. `closeVerifyFlipPlan`
calls `closeVerifyHeldRefusal` before either a verified or implemented brief
can become done. Both use `unroutedHeldLine`; changing only card emission
would offer a close that still refuses. Changing the close to ignore the same
signal would recreate the declined exception.

The real record contains an original-obligation trace mentioning held work,
a retained advisory saying could-not-check, a quoted mutation diagnostic,
risk-value prose and an exit-code mapping. A worker cannot establish that all
of these are discharged obligations by checking that the two original command
rows passed. In particular, an advisory's nonblocking review classification
does not by itself establish its disposition for final human acceptance.

The missing contract is **who may adjudicate a positive full-scan signal, on
which exact source, and with what consequence**. Existing `gate-card/v1`
metadata explicitly grants no authority. A new verifier-authored text file,
App label, attestation or passing CI run must not silently acquire it.

## Proposed contract

Separate preparation, adjudication and final acceptance. A preparation is
read-only advice. Adjudication resolves the meaning and disposition of exact
scan signals; it is not a verification stamp. Final acceptance remains the
existing distinct human act after the independent verification prerequisites.

1. An authorized independent verifier prepares a manifest through a native
   command for one canonical repository and brief. The manifest binds the full
   immutable source revision, brief identity and acceptance version, brief and
   stream-README blob IDs, protected original-table coordinates and selected
   verification revisions. It cannot be supplied by the implementing worker
   as an independent result.
2. Preparation calls the existing full scan without new exclusions. It records
   the complete ordered set of refusals and a digest of that set. The scanner
   presently exposes its first refusal; complete enumeration must be added to
   its existing semantic owner, preserving the existing boolean/first-reason
   behavior for every current consumer. Do not implement another regex scan.
3. Each signal has an exact source span and content digest, plus one proposed
   disposition: `diagnostic-only`, `resolved` or `unresolved`. A diagnostic
   proposal identifies the original acceptance obligation, or explains why
   the span is not one. A resolved proposal cites immutable evidence and the
   existing accountable owner. Unknown, unverifiable or could-not-check stays
   unresolved. A receipt's presence is never evidence that its claims hold.
4. The verifier may propose dispositions, but cannot authorize them. A separate
   human adjudication comment approves the exact manifest digest and source
   revision. It must be corroborated through the existing mapped-human,
   unedited-comment, same-repository and exact-record binding rules. It is a
   new structured authority type; existing design approval, a generic bless,
   a PR approval or the ruling on #4995 cannot substitute for it.
5. Only after that corroboration may the authorized emitter prepare a canonical
   final acceptance card. It reruns all existing verification prerequisites,
   original-row coverage and floor checks, authenticates independent writer
   provenance, and confirms that the manifest covers every current scan signal.
   Any unresolved signal holds the route. The card contains the full scan
   result, proposed dispositions, adjudication link, immutable source binding
   and surviving advisories. It must state that the generic scan still holds
   and that only the separately corroborated adjudication supports this route.
6. Final human close re-reads all inputs and corroborates the adjudication
   again. It refuses missing, edited, foreign, stale or incomplete authority,
   changed Evidence or acceptance rows, and new scan findings. Only a current
   complete adjudication can discharge this route's scan prerequisite; it
   cannot discharge a failing original Verify row, missing provenance,
   insufficient independent stamps, a decision gate or final human close.
   The ordinary emitter, autoflip and done-close paths retain their full scan.

This is deliberately a proposed **new authority**, rather than a new spelling
that makes existing Evidence evade the scan. The implementation must remain
held until the human rules on that authority contract. Approval of this
proposal would authorize implementing the contract; it would not adjudicate
the real /03 signals or accept /03.

## Reuse and implementation boundaries

Architecture pickup used source revision
`7b56c62e2c2dc6a597a6b41ccdd43c72c70fe0ca`. The semantic owners are
`unroutedHeldLine` for the scan, `protectedAcceptance`/`closureWitnesses` for
original-row provenance, `flipLatestPass` for run selection,
`closeVerifyFloorRefusal` for floor policy, and S-decision-acceptance in
`docs/contracts.md` for human authority. `gate-card/v1` remains informational.
Selected closure evaluation stays in `statusgen/evaluation`; forge and actor
authentication stay outside that pure package.

Disposition: **hold the new effect authority; continue design preparation**.
No new library, dependency, credential, service or deployed tool is required
by this proposal. Existing source-pinned acceptance-transition records provide
a precedent for exact structured human corroboration, but do not already
authorize scan adjudication. They must not be repurposed as an implicit waiver.

Implementation consumers to cover together are the explicit preparation verb,
the authorized canonical emitter, the human close adapter, typed manifest
validation and any lint that validates resulting lifecycle state. The current
workflow reads only the verify-gate marker; a new card type must not rely on
informational metadata being enforced there. Workflow promotion remains with
its authorized maintainer. No hand-edited generated board or manual card can
replace the native route.

The existing error class [#4276](https://github.com/medici-finance/assay/issues/4276)
owns free-text human sign-off corroboration. This proposal adds no recognition
rule to it. A future implementation changing that owner must perform the
strike-two audit before coding; it cannot use this proposal as a bleed ruling.

## Required discriminating tests before implementation lands

| Input or change | Required outcome |
|---|---|
| Real /03 Evidence, unchanged | Full scan still reports its current signals; preparation grants no acceptance. |
| Same record with valid independent preparation and exact human adjudication | Separate route can offer a final acceptance card; ordinary routes still hold. |
| Original Verify result is HELD, FAIL, missing or could-not-check | Refuse, even with a purported human-approved manifest. |
| Unkeyed unresolved signal or omitted signal | Hold; complete passing command rows do not hide it. |
| Missing authority, bot-authority, wrong human, edited comment, foreign record or generic approval | Refuse; never fall back to free text or card metadata. |
| Forge authority cannot be read | Could-not-check; never accepted or a fabricated refusal observation. |
| Wrong repository/brief/version, stale source, changed blob or selected run | Refuse without a lifecycle write or usable final card. |
| New scan hit after preparation | Hold; recompute and require new exact adjudication. |
| Worker writes the receipt or independent actor cannot be corroborated | Refuse or could-not-check according to the actual observation. |
| Final card closed by a bot, or a human adjudicates but has not finally accepted | No done transition. |

Plant a second caller trying to turn a preparation or informational card into
authority. The class guard must fail at that site, not just at a fixture naming
/03. Retain mutation controls for source binding, actor corroboration, complete
signal coverage, unresolved dispositions and final-close separation.

## Exact decision proposed

On [issue #4993](https://github.com/medici-finance/assay/issues/4993), a human may
approve or decline the following contract after independent review:

> Approve implementing the separate acceptance adjudication contract in
> docs/acceptance-route-proposal.md at the reviewed immutable PR head. Independent
> verifiers may prepare exact source-bound dispositions; only an exact,
> corroborated human adjudication may discharge the separately reported scan
> signals for that route. Unresolved results and every other verification,
> provenance, decision and final-human-acceptance gate remain blocking. This
> approves the contract, not the adjudication or acceptance of /03.

The recorded ruling must identify the actual reviewed commit, rather than
literally saying “the reviewed immutable PR head”. Until it exists, #4993
remains open and /03 has no newly authorized final acceptance route.
