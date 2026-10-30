# Proposed write-policy acceptance amendment

Issue [#4794](https://github.com/medici-finance/assay/issues/4794) concerns the
acceptance definition of desktools-v2/66, not an established runtime defect.
This proposal changes only row 5's Command and increments the brief from version 1
to version 2. Every original row ID, Class and Expect is retained. Historical
Evidence, including the independent FAIL, is unchanged. The Context declaration
repair in [#4810](https://github.com/medici-finance/assay/pull/4810) remains separate.

## Why this command discriminates

Row 5 first runs its exact original full inherited regression command. It then
reruns the existing `TestCheck*` cases with Go coverage restricted to the shared
`deskcore/writepolicy` package, and requires nonzero execution of `Evaluate`.
These file and shell cases predate extraction and assert both refusals and allows.
The second run therefore checks that inherited behavior exercises the extracted
owner, rather than merely passing against a retained command-local evaluator.
It introduces no new test name, package, test-count threshold or convenient base.

The complete suite remains the regression floor, including inherited exemption,
sentinel and lexical cases outside the focused coverage run. A nonzero coverage
assertion is only a connection check; it does not establish parity or purity by
itself. Rows 1–4 retain those independent obligations and their mutation controls.
No risk classification, fail-first requirement, human gate or process exit is
relaxed. Production evaluator and adapter bytes are unchanged.

## Architecture pickup

At source `9e26cedb8cd35505cc0d4eb11105159176c8ecc3`, the existing
`docs/write-policy.md`, `docs/write-policy-consumers.md`, `docs/contracts.md`,
`docs/library-first.md`, `docs/go-engineering.md` and brief Task still select
`deskcore/writepolicy` as policy owner and writeguard as observation/protocol
adapter. Continue that ownership; adapt the row's connection proof only. The
original tests' compatibility helpers call the production adapter's `evaluate`,
which calls the imported production `writepolicy.Evaluate`. Coverage observes
that path; it is not an additional evaluator or production guard.

The authoring/provenance rules are `spec/brief-v1.md` §§4.1, 7,
`spec/registers-v1.md` §3 and `docs/protected-paths.md`'s explicit human-approved
acceptance transitions. No existing register entry or Evidence is rewritten.
The implementation pickup ruling on issue #3752 approves the extraction; it
provides no approval of this later acceptance correction.

## Author validation (not independent verification)

Offline local validation of the exact proposed Command:

| Source | Original full suite | Existing focused cases | Shared-owner assertion | Row exit |
|---|---|---|---|---|
| Actual pre-extraction first parent `4395628b9ff5575e2c5d7d62f8ded2902d1c9ee7` | pass | pass | fails: no shared-policy statements are executed | 1 |
| Source `9e26cedb8cd35505cc0d4eb11105159176c8ecc3` | pass | pass; 51.0% shared-package statement coverage | pass: `Evaluate` executes | 0 |

The baseline is a successful behavioral regression run followed by a failed
**extraction connection assertion**. Its coverage warning that no tested package
uses the shared owner is the observed architectural difference. It is not a
missing-test failure, a compilation failure, or evidence of a runtime policy bug.

Two Go build overlays changed only the body of the actual shared
`deskcore/writepolicy/policy.go` `Evaluate` function, leaving production files
unchanged. Prepending `return Outcome{Decision: Allow, Reason: NoObjection}`
compiled and made existing shared-path and shell refusal assertions fail.
Prepending `return Outcome{Decision: Deny, Reason: SharedDenied}` compiled and
made existing owned-path and allowed-shell assertions fail. Both focused runs
exited 1 on behavioral assertions. These controls establish that the inherited
cases respond to shared-owner decision changes, beyond merely observing a
covered symbol. They do not replace rows 1–4's more specific controls.

Pre-mortem: keeping a functioning local evaluator would pass the inherited suite
but fail the shared execution assertion; always-allow/always-deny shared decisions
fail inherited expectations; a call made only by new extraction tests cannot
satisfy a run restricted to the longstanding `TestCheck*` cases. A redundant
shared call alongside a duplicate evaluator remains a source-review and row 4
consumer-census concern. Coverage alone cannot exclude that shortcut.

## Required acceptance transition after landing

This draft does not supply human approval or independent verification. Keep
issue #4794 open and the brief at implemented. The human must approve the exact
landed amendment through the existing acceptance-transition protocol, after
review and merge. Merging this draft alone does not approve the new table.

The protocol's immutable delivery is
`86eec0f73399f679d5ea9b8ed81687e186441012` (#3772).
Its two-parent merge has unique merge-base
`ce25be49efeb8a345019c6a663002edfd705da27`; the complete acceptance table there
matches its first-parent main table and delivery. These are table-provenance
coordinates, not an alternative fail-first baseline. The actual pre-extraction
fail-first source remains first parent
`4395628b9ff5575e2c5d7d62f8ded2902d1c9ee7`.

The transition owner must create a same-repository decision issue containing:

```text
<!-- acceptance-transition: docs/streams/desktools-v2/brief-66-extract-shared-write-policy-and-make-writeguard-a-hook-adapter.md -->
```

After landing, an externally mapped human posts this exact payload with
`<landed-amendment-sha>` replaced by the full immutable amendment commit:

```text
Approve acceptance-transition v1
repo=medici-finance/assay
brief=docs/streams/desktools-v2/brief-66-extract-shared-write-policy-and-make-writeguard-a-hook-adapter.md
delivery=86eec0f73399f679d5ea9b8ed81687e186441012
base=ce25be49efeb8a345019c6a663002edfd705da27
original=ce25be49efeb8a345019c6a663002edfd705da27
amendment=<landed-amendment-sha>
rows=5
```

The owner must recheck those immutable coordinates and retained table definitions
against the landed history. The resulting descriptor binds that amendment and
the unedited human comment URL. Fresh independent admission and verification
then rerun every original obligation and controls, using the approved correction
only for row 5. Final gate:human acceptance is separate. The host-only transition
runner's limitations remain in force; this document authorizes no alternate
sandbox or approval bypass. The repository builds statusgen from source and its
authoritative workflow already opts into `--acceptance-forge`; the verifier must
recheck every authoritative consumer and corroborate the actual ruling before
committing any final transition verdict.
