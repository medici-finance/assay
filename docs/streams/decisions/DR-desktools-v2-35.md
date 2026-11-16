---
id: DR-desktools-v2-35
date: "2026-11-16"
title: "Install path on the shared Cobra adapter (desktools-v2/35): option 1, approve the measured deskinstall compatibility report, is the proposed design; no driver ruling is recorded, so this record is placeholder-unratified"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Option 2 — reject or narrow: the listed change is removed and the report is re-submitted (the brief's `## Human decision` and decision issue #4552). Not ruled out: no driver ruling chooses between the options."
accepted:
  - "Proposed, not ruled. `deskinstall` parses its flags with Cobra through the shared desk CLI adapter. Kept: every legacy long spelling, the `--version` forms, exit codes 0, 1, 2 and 5, `unknown argument` refusals, help to stderr with exit 5 on no arguments, and every mode rule. The sha256 and build-attestation checks stay inside the installer and no flag skips them (the compatibility report on #4553, summarised on #4552)."
  - "Proposed, not ruled. Changed, as listed in that report: `--help`, `-h` and `help` print to stdout and exit 0; help text is generated from the command tree; refusals no longer reprint the usage screen; parse errors point to `--help`; a missing value reads `flag needs an argument`; a flag after a leading `help` is refused. Additive: `--flag=value`, explicit boolean values and the shared `--perf-*` flags. No environment variable or configuration key is newly bound."
  - "Under either option, approval covers only the listed changes, and silence never authorises a weaker check (the brief's `## Human decision`). Independent verification stays a separate act; this record supplies none."
---

## State and provenance

**Approval state: placeholder-unratified. This is not an approved design record.** No recorded
driver ruling exists for desktools-v2/35. Its decision issue,
[issue #4552](https://github.com/medici-finance/assay/issues/4552), is open. It holds a worker
comment pointing to the compatibility report; that comment is not a ruling, and no comment from
the driver's own account chooses an option.

The `decided-by` stamp above is the public placeholder and names nobody
(`spec/registers-v1.md` §7.5). This record carries no `ruling:` link, because there is no driver
comment to link. `statusgen --corroborate` is expected to report it `placeholder-unratified`
until the driver posts an unedited comment on #4552, from the driver's own account, naming
`desktools-v2/35` and `DR-desktools-v2-35` with the chosen option. The `ruling:` key is added by
a later push, only after that comment exists.

The brief's `design:` key is deliberately absent. The design-gate lint
(`spec/lifecycle-v1.md` §4.4) checks only that a cited record exists, never that it is approved,
so citing this record now would release the hold before the record is ratified. The `design:` key
is added in the same push that adds this record's `ruling:` link.

**Written after implementation.** The `deskinstall` half of desktools-v2/35 was delivered by
[#4553](https://github.com/medici-finance/assay/pull/4553), merged 2026-10-27, before this record
existed. This record was written afterwards to back-fill the design-approval gate for a brief
that other work depends on: desktools-v2/17 declares `desktools-v2/35` in `depends`. It did not
precede the implementation. It is written on demand under the ruling on #5145, which limits
back-filled records to held briefs that block other work.

This record does not mint a decision. The options are taken from the brief and #4552; the
compatibility report is taken from #4553. Nothing here adds rationale beyond what those record.

## The decision put to the driver

`docs/streams/desktools-v2/brief-35-migrate-the-install-path-to-cobra-and-viper.md` asks whether
the measured compatibility report preserves the existing refusals, credential sources and exit
codes exactly. The options are:

1. **Approve the report.** The migration lands with exactly the listed compatibility changes.
2. **Reject or narrow.** The listed change is removed and the report is re-submitted.

Default if no answer: none. The brief blocks until answered.

## What this record does not decide

It does not decide the four install-script rows the brief assigns to itself
(`assay-install.sh`, `bootstrap-windows.ps1`, `build-windows.ps1` and
`windows-bootstrap-hashcheck-smoke.ps1`). #4553 left them `pending` in
`docs/streams/desktools-v2/cli-migration.json` and raised their scope on #4552 with three paths:
mark them `exempt` under the CLI contract, narrow the brief to the Go installer, or port them.
That scope question is the driver's and is not settled here. This record also does not choose an
option, record a verification result, move the brief's board row, or decide desktools-v2/17 or
any other dependent.
