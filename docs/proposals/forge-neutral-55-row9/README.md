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

The existing `statusgen/40` brief owns transition-chain support. Its scope is
recorded on https://github.com/medici-finance/assay/issues/6064.
This proposal reuses that owner and adds no alternate resolver, descriptor,
approval bypass, binary or exception. The earlier rows 7 and 10 must retain their
approved effective commands when the successor is eventually admitted.

The resolver source is byte-identical at v1.0.83 and the proposal source (Git
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

On 2026-11-12, the worker ran the exact proposed command against the pinned
PostgreSQL 18.6 disposable container from `workflow/testdata/compose.yaml`, using
an explicit local Unix-socket engine and a unique Compose project. Vet and the
complete workflow race suite exited 0; no skip was accepted. Its retained output
was:

```text
--- PASS: TestPGRefreshProvisionSchema (0.18s)
--- PASS: TestPGRefreshProvision (3.69s)
--- PASS: TestPGRefreshProvisionReplay (0.79s)
--- PASS: TestPGRefreshProvisionProcesses (1.28s)
```

The worker removed only that fixture's labeled project resources after the run.
The output SHA256 is
`ab29ca0498ccf8a0deb6493891882cdd44c9a289801b026979806e45ba479db4`.
The proposed command SHA256, including its final newline, is
`a051d980b314572e4a23f3dc43c6e94c76a5d9606e72ad6795251749203fddf5`.
The adjacent patch SHA256 is
`17bcb2ca24a4e1af5ec8d1937c4d3769f78e75f7268dada704922b15a89754b1`.

Nine synthetic command controls each exited 1: successful Go output with no
selected tests; each of the four required names missing individually; a SKIP
beside all four PASS lines; a failing Go test; failing vet; and an absent DSN.
The matching four-PASS synthetic control exited 0. These exercise shell-command
acceptance only and earn no provisioning or authentication credit.

A source declaration search for `TestPGRefreshProvision` at the pre-delivery
first parent exited 1, with no match. The worker did not execute a baseline
verification run or reuse these checks as independent Evidence.

## Order of work

1. The existing owner delivers transition-chain support; consumers acquire its
   reviewed release and pins.
2. Refresh this patch against the then-current brief. Keep the existing approval
   and every historical Evidence byte intact; stop if its context changed.
3. Prepare the separate version-5 amendment and obtain the driver's exact
   immutable successor approval through the shared supported protocol.
4. The review desk handles review and readiness. The independent verification
   desk runs the protected plan and records fresh results.

Serving, production identities, policy values, deployment and tool cutover retain
their existing gates. This proposal supplies none of them.
