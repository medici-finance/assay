# Proposed forge-neutral/55 row-9 correction (held)

Issue: https://github.com/medici-finance/assay/issues/5752

This proposal supplies an exact command-only correction to Verify row 9 of
`docs/streams/forge-neutral/brief-55-operator-provisioning-of-known-read-only-refresh-profiles.md`.
The adjacent `row-9.patch` proposes version 4 to version 5. This change does not
apply that patch: the active brief, its table, version, Expect, Evidence and
lifecycle records remain byte-identical to the source snapshot below.

## Correction

The existing command runs vet and the complete workflow race suite. That suite
also passes before the provisioning tests exist, so success alone does not
identify the delivered contribution. The proposed command retains both checks,
captures verbose test output and requires a PASS line for each delivered test:

- `TestPGRefreshProvisionSchema`
- `TestPGRefreshProvision`
- `TestPGRefreshProvisionReplay`
- `TestPGRefreshProvisionProcesses`

It also rejects every `--- SKIP` line. A successful Go invocation selecting no
provisioning tests, or a skipped database test, cannot satisfy those assertions.
Each assertion identifies an actual provisioning test in `workflow/store`;
subprocess-only `TestPGRefreshProvisionChild` earns no separate credit.

The exact unchanged Expect is:

> Exit 0 on the actual disposable container; all foundation/Store/Gate/provisioning and owned-role cleanup checks pass without skipped integration credit.

No row is removed, made supplemental or renumbered. The class, shell, order,
remaining commands and existing obligations are unchanged. The new check retains
the full regression suite while requiring the delivered provisioning cases.
The assertions use shell `if` statements so the command introduces no literal
pipe delimiters into the Markdown table. The earlier proposal used shell OR
operators that split the row into extra cells; that patch must not be applied.

## Why the patch is held

The brief already uses the human-approved command correction from amendment
`85898e42b80ec4114f0a0621dc7166fefbf066e4` for rows 7 and 10.
Its existing approval remains valid for that exact scope:
https://github.com/medici-finance/assay/issues/5724#issuecomment-6715508823

The v1.0.83 resolver in `statusgen/acceptancetransition.go` supports one live
acceptance transition. It requires the current Verify table and acceptance
version to equal the approved amendment, and it requires the table before an
amendment to equal the delivered table. A second edit to row 9 would therefore
break the current transition without providing a usable successor. A second
human approval alone cannot add unsupported resolver behavior.

The `statusgen/40` transition-chain implementation merged in
https://github.com/medici-finance/assay/pull/6235
at merge `968f879b24836a3194194848692cae71d5325789`. Its design scope is
recorded on https://github.com/medici-finance/assay/issues/6064.
The latest published tools sampled on 2026-11-12 remain v1.0.83. Release and
consumer adoption of the merged support, followed by qualification by the
respective desks, remain prerequisites for applying this successor.
This proposal reuses that owner and adds no alternate resolver, descriptor,
approval bypass, binary or exception. The earlier rows 7 and 10 must retain their
approved effective commands when the successor is eventually admitted.

The resolver source is byte-identical at v1.0.83 and the immutable proposal source (Git
blob `b15fd59fb87646f0462cf9e65da06508295091be`). The worker's source check was:

```text
$ rg -n 'one literal live Acceptance-Transition|acceptance changed between delivery and amendment|current Verify differs from the approved amendment|current acceptance version differs from amendment' statusgen/acceptancetransition.go
48:		return fail("one literal live Acceptance-Transition marker is required")
78:		return fail("acceptance changed between delivery and amendment")
92:		return fail("current Verify differs from the approved amendment")
163:		return fail("current acceptance version differs from amendment")
```

These are resolver checks, not a verifier execution or a new acceptance result.

## Immutable source and acceptance coordinates

- Proposal source: `0665bfb771788b07473c281b8875fc1806dab36d`.
- Provisioning delivery: `ef8316b8814dc2625ba701089909ddd8a399e96a`.
- Existing protected-table base: `ae3e2b12ded2c8202264a90df9d666d202842aa7`.
- Existing original table: `e5783147c8e96af62779601af8aa71f31667e567`.
- Pre-delivery first parent used by the reported fail-first comparison:
  `33b78e4273a2c68395a7cff18d8d831bca9b882f`.

The protected-table base and the fail-first comparison revision serve different
purposes. This proposal retains the existing coordinates and does not widen or
replace either to manufacture a passing result.

## Worker checks and limits

Worker checks validate the proposed command and patch. They do not run the
verifier protocol, append Evidence, issue a review verdict or grant acceptance.
The independent verifier must rerun the actual protected plan after a supported
successor and its exact human approval land.

On 2026-11-12, the worker reran the corrected command against the pinned
PostgreSQL 18.6 disposable container from `workflow/testdata/compose.yaml`, using
an explicit local Unix-socket engine and a unique Compose project. Vet and the
complete workflow race suite exited 0; no skip was accepted. Its retained output
was:

```text
--- PASS: TestPGRefreshProvisionSchema (0.32s)
--- PASS: TestPGRefreshProvision (9.56s)
--- PASS: TestPGRefreshProvisionReplay (1.93s)
--- PASS: TestPGRefreshProvisionProcesses (1.79s)
```

The runtime source was `9a86fc2b90b71f0858a177d06bd6a6bd0dfca7e0`; its
`workflow` tree is identical to the immutable proposal source. No active brief
was changed or admitted by that worker run.

The worker removed only that fixture's labeled project resources after the run.
The output SHA256 is
`83c5880419ea7ca435272f17170f923949bee8265b1fec3efb02b124f1b071d2`.
The proposed command SHA256, including its final newline, is
`ad6d4bd1987267a3fb628239eaabff75e99cb4dceedf8eeee582413c58e7d476`.
The adjacent patch SHA256 is
`d1f4a81e9a2c1af96e94776763522198a3b910f3912218d455249ac8d3364f9f`.

Nine synthetic command controls each exited 1: successful Go output with no
selected tests; each of the four required names missing individually; a SKIP
beside all four PASS lines; a failing Go test; failing vet; and an absent DSN.
The matching four-PASS synthetic control exited 0. These exercise shell-command
acceptance only and earn no provisioning or authentication credit.

A source declaration search for `TestPGRefreshProvision` at the pre-delivery
first parent exited 1, with no match. The worker did not execute a baseline
verification run or reuse these checks as independent Evidence.

## Order of work

1. Release the merged transition-chain implementation and adopt its published
   assets through consumer pins. Leave qualification to the respective desks.
2. Refresh this patch against the then-current brief. Keep the existing approval
   and every historical Evidence byte intact; stop if its context changed.
3. Prepare the separate version-5 amendment and obtain the driver's exact
   immutable successor approval through the shared supported protocol.
4. The review desk handles review and readiness. The independent verification
   desk runs the protected plan and records fresh results.

Serving, production identities, policy values, deployment and tool cutover retain
their existing gates. This proposal supplies none of them.
