---
id: DR-cellctl-cobra
date: "2026-10-08"
title: "The compatibility report for cellctl's Cobra and Viper migration is accepted: the preserved source/admission boundaries and the explicitly listed help and parse changes, subject to independent review and all required validation passing"
consequence: major
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/2240#issuecomment-6065679126"
alternatives:
  - "Option 2 of the decision issue — 'Hold the migration and specify a compatibility change that must be revised before acceptance. Existing published CLI behavior remains in place until a reviewed merge.' (#2240 `Options:`, item 2). The issue put two options; this is the one not taken. Ruled out because the driver's answer is `1`. No rationale was recorded with the answer and none is supplied here."
accepted:
  - "Help, `-h`, `--version`, `version` and parse failures no longer print the effective-configuration echo (first item of the change list as put to the driver; desk relay on #2240)."
  - "An unknown verb and a missing flag value use Cobra's wording, still exit 3 (second item of the same list)."
  - "`scratch ... --max-age bogus` exits 3 (was 2) (third item of the same list)."
  - "`--flag=value` and a bare `--` are accepted by every non-raw verb; a single-dash token that is not a known long flag is an error; `cadence recover` without a role is refused (fourth item of the same list)."
  - "The acceptance is conditional, in option 1's own words: 'subject to independent review and all required validation passing' (#2240 `Options:`, item 1). This record does not say that condition is met."
---

**This record transcribes a recorded human ruling. It is not an agent asserting sign-off.**
The human act is the driver's (`human:<name>`) two comments on decision issue
[#2240](https://github.com/medici-finance/assay/issues/2240), each posted under the driver's
own login and never a role App. This file copies that ruling into the register for the
lifecycle's design-approval gate (`spec/lifecycle-v1.md` §4.4). It mints no decision and
supplies no rationale; the ruling gave none.

## The recorded ruling

**The issue.** #2240, "Decision: assay:assay:desktools-v2:16 — Migrate cellctl to Cobra
commands and Viper configuration", was opened 2026-10-05T10:56:34Z and its body has not been
edited. The body carries `<!-- decision-gate: assay:assay:desktools-v2:16 -->`, the
decision-gate marker of brief desktools-v2/16
(`docs/streams/desktools-v2/brief-16-migrate-cellctl-to-cobra-commands-and-viper-configuration.md`),
which cites this record through `design:`. The issue was still open when this record was
written.

**What was asked.** The body says: "A human decision is needed on work item
`assay:assay:desktools-v2:16`" and "reply with the chosen option as a comment (the comment is
the decision record), then close the issue." It gives a compatibility report and then two
options, and no more than two:

> 1. Accept the preserved source/admission boundaries and the explicitly listed help and
>    parse changes, subject to independent review and all required validation passing.
> 2. Hold the migration and specify a compatibility change that must be revised before
>    acceptance. Existing published CLI behavior remains in place until a reviewed merge.

It ends: "Default if no answer: none — blocks until answered".

**The answer.** Two comments, both by the driver, account type User, neither edited (each
comment's last-updated time equals its creation time):

- [comment 6065583010](https://github.com/medici-finance/assay/issues/2240#issuecomment-6065583010),
  2026-10-08T17:37:42Z. Its whole text is `1`.
- [comment 6065679126](https://github.com/medici-finance/assay/issues/2240#issuecomment-6065679126),
  2026-10-08T17:43:28Z. Its whole text is `1 — DR-cellctl-cobra`.

The first comment is the answer. It does not name this record, and the register's check
requires the linked comment's own text to name the record's id (`README.md`, "How the approval
is checked"). The record's `ruling:` link therefore cites the second comment, which repeats
the same answer and names the record. It does not change the ruling. `DR-windows-port-08`
links a restating comment for the same reason. The record's `date:` is the date of both
comments (UTC).

## What was put to the driver

The desk App posted a relay on the issue
([comment 6065590187](https://github.com/medici-finance/assay/issues/2240#issuecomment-6065590187),
2026-10-08T17:38:07Z). It is a desk note, not the ruling. It records the question as "the two
options in this issue's body (1 accept, 2 hold)" and then says: "Because the body's list of
changes was written from an earlier build, the desk put the list from the draft PR #2391 at
head `597904c5` to the driver with the question". The list, as the relay gives it:

> - help, `-h`, `--version`, `version` and parse failures no longer print the effective-configuration echo;
> - an unknown verb and a missing flag value use Cobra's wording, still exit 3;
> - `scratch ... --max-age bogus` exits 3 (was 2);
> - `--flag=value` and a bare `--` are accepted by every non-raw verb; a single-dash token that is not a known long flag is an error; `cadence recover` without a role is refused.

The relay continues: "Unchanged, as put: credential sources, the desk-role model refusal and
the admission checks, tested with the parser bypassed. Where this list and the issue body
differ (the body says parse errors return exit 2), the PR's list is what was put." And:
"**Answer:** `1`, accept. No rationale was given with it."

**The two lists differ, and this record says so plainly.** The issue body's own report, under
"Deliberate parse changes", reads: "Unknown flags/commands, malformed typed values and wrong
positional counts return exit 2 before effects, replacing inconsistent exit 1/3 or ignored
extra arguments." The list as put keeps exit 3 for an unknown verb and a missing flag value,
and lists `scratch ... --max-age bogus` moving from 2 to 3.

The relay's four items are the "Deliberate differences" list in the body of
[#2391](https://github.com/medici-finance/assay/pull/2391), in shorter wording. That body was
last edited 2026-10-08T16:27:52Z, before the answer, and its list reads:

> - help, `-h`, `--version`, `version` and parse failures no longer print the effective-configuration (roster) echo;
> - an unknown verb and a missing flag value use Cobra's wording, still exit 3, with no echo;
> - `scratch ... --max-age bogus` exits 3 (was 2 from the Go flag package); the hook verb keeps 2;
> - `--flag=value` and a bare `--` are accepted by every non-raw verb; a single-dash token that is not a known long flag is an error; `cadence recover` without a role is refused.

Two limits on this section. The relay was posted after the first answer (17:38:07Z against
17:37:42Z) and before the second (17:43:28Z). And what was said outside the issue cannot be
checked from the issue: for the list that was put, this record relies on the relay.

## Reading, stated so it can be declined

The driver's comments say `1` and `1 — DR-cellctl-cobra`, and nothing else. This record reads
that answer as follows.

1. `1` is option 1 of #2240 as the issue wrote it: "Accept the preserved source/admission
   boundaries and the explicitly listed help and parse changes, subject to independent review
   and all required validation passing."
2. "the explicitly listed help and parse changes" is the change list as put to the driver,
   which the relay records and which is quoted above. Where that list and the issue body's
   report differ, the list as put governs on this reading, because the relay says that list
   "is what was put". The `accepted:` entries above carry that list and no other.

An item that appears only in the issue body's report is part of the issue the answer was given
on. This record neither adds such an item to what was accepted nor removes it.

If either half of this reading is wrong, the remedy is to decline the pull request that adds
this record. Nothing here lands until the driver merges it.

## The design that was approved

The brief is the authority for the design. This summary quotes it and the issue and does not
amend either.

- The issue's context: "the proposed migration replaces cellctl's manual flag loops and
  copied shell help with a Cobra command tree and command-local Viper resolution. The
  configuration format remains cell.env plus the existing JSON policy/provider catalogs; no
  persistence format migration, config search path, credential source or authority grant is
  proposed."
- The brief's `layering` line: "Cobra constructs the command tree and passes validated typed
  inputs into existing handlers; command-local Viper resolves only declared inputs through
  compatibility adapters. Launch planning, custody, policy and external execution retain
  their current owners."
- The brief's `single-point-of-failure` line: "parser-selected options must not grant
  authority; domain admission remains an independent check below the adapter, and fixture
  launch/custody tests exercise it with the parser bypassed."
- What the human acceptance covers, from the brief's `## Human decision`: "Human acceptance
  covers preserved credential-source restrictions, execution admission and any explicitly
  listed command/configuration compatibility changes; silence never authorizes a weaker
  gate."
- #2391's body: "No configuration-format transition is proposed."

## The order of events

The ruling came after the implementation was built and after the implementing pull request
was opened. This record does not describe a design approved before it was built.

- 2026-10-03: the brief was authored (`authored: "2026-10-03 by coordinator"`).
- 2026-10-05T10:56:34Z: #2240 was opened. Its compatibility report already compares against
  an implementation: "203 fake/DRY_RUN transcripts from pre-port Go at
  40d0063ef8a0c500d15a70b2db9cfbf6a6ce118b match the migrated binary."
- 2026-10-08T13:07:07Z: #2391 was opened as a draft. On its branch the brief's board row
  reads `implemented`; on `main` it read `todo` when this record was written.
- 2026-10-08T14:32:27Z and 14:49:30Z: both review lanes requested changes on #2391 at head
  `f19dd544`.
- 2026-10-08T17:37:42Z and 17:43:28Z: the driver's two comments.

## The consequence level

`consequence: major` is the transcriber's classification. It is not part of the ruling, which
states no level. It was taken from two things in the brief: its `gate-why` ("Cellctl
configuration selects executable paths, role identity and credential locations.") and its
`risk` answers (`sensitive-data` is `yes`; `irreversible`, `regulatory` and `customer` are
`no`). The driver can change the level by declining this record or asking for the edit.

## What this record does NOT decide

- **The review of #2391.** Both lanes requested changes at head `f19dd544` (the correctness
  lane at 2026-10-08T14:32:27Z, the security lane at 14:49:30Z). The head has since moved to
  `597904c5`, which had no review when this record was written. Option 1 is "subject to
  independent review and all required validation passing"; that review and that validation
  are not this record.
- **Any change to the listed behaviour made after 2026-10-08T17:37:42Z.** The answer was
  given on the list quoted above. A later change to that list is not covered by it.
- **The choice of Cobra and Viper.** The brief's `gate-why` says "choosing Cobra and Viper
  was already requested in issue 2111", and #2111 says: "This issue records the maintainer's
  direct request to use Cobra and Viper; library selection is settled." The brief's
  `## Human decision` says "Library choice is already settled."
- **The human sign-off on the brief's later `verified` and `done` flips.** The brief is
  `gate: human`; those sign-offs are separate acts with no recorded answer here.
- **The merge of #2391.** That stays the driver's.

And, per the register's own limits (`README.md`, "What a record does not establish"): a
checked `ruling:` link proves who ruled and where, not what the comment said. Whether this
record matches the ruling is the review gate's call on the pull request that adds it.
