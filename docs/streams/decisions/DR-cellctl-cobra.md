---
id: DR-cellctl-cobra
date: "2026-10-08"
title: "Option 1 of the decision issue is taken for cellctl's Cobra and Viper migration: accept the preserved source/admission boundaries and the explicitly listed help and parse changes, subject to independent review and all required validation passing; by the driver's own answer the listed changes are the 'Deliberate differences' list of #2391; a second answer accepts seven further differences; a third answer, on a corrected before, sets exit 3 for a flag-shaped word after `desk`"
consequence: major  # the transcriber's classification, not part of the ruling; see "The consequence level"
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/2240#issuecomment-6065679126"
alternatives:
  - "Option 2 of the decision issue — 'Hold the migration and specify a compatibility change that must be revised before acceptance. Existing published CLI behavior remains in place until a reviewed merge.' (#2240 `Options:`, item 2). The issue put two options; this is the one not taken. Ruled out because the driver's answer is `1`. No rationale was recorded with the answer and none is supplied here."
accepted:
  - "The four entries below are the list the driver named in their own login: the answer `a` (#2240 comment 6066281586) to the question 'Which list does option 1 cover?' (#2240 comment 6066226070), whose option a is the 'Deliberate differences' list in #2391's body as it stood at 2026-10-08T16:27:52Z. Each entry is quoted from option a."
  - "help, `-h`, `--version`, `version` and parse failures no longer print the effective-configuration (roster) echo"
  - "an unknown verb and a missing flag value use Cobra's wording, still exit 3, with no echo"
  - "`scratch ... --max-age bogus` exits 3 (was 2 from the Go flag package); the hook verb keeps 2"
  - "`--flag=value` and a bare `--` are accepted by every non-raw verb; a single-dash token that is not a known long flag is an error; `cadence recover` without a role is refused"
  - "Not accepted: the issue body's own report of the parse changes ('Unknown flags/commands, malformed typed values and wrong positional counts return exit 2 before effects'). It was option b of the same question and was not taken."
  - "The acceptance is conditional, in option 1's own words: 'subject to independent review and all required validation passing' (#2240 `Options:`, item 1). This record does not say that condition is met."
  - "The seven entries below are the second list the driver accepted in their own login: the answer `1 — DR-cellctl-cobra` (https://github.com/medici-finance/assay/issues/2240#issuecomment-6078423478, 2026-10-09T09:42:59Z) to the question on the seven remaining differences in #2391 at `e69e02b15` (#2240 comment 6078394997), whose option 1 is 'Accept all seven (recommended). They join the record's accepted list.' Each entry quotes that question's numbered item, bold markup dropped, and cites the answer. Counting only the quoted differences, the four entries above are decision entries 1 to 4 and these seven are decision entries 5 to 11, in the question's order. Entry 10 also states the measured before and quotes the third answer (#2240 comment 6079591256), set out under 'The third answer' below."
  - "Unknown flag on `ls` / `status <cell>` (`ls --bogus`). Before: exit 0, the word ignored. Now: exit 3, `unknown flag`. — item 1 of #2240 comment 6078394997, accepted by https://github.com/medici-finance/assay/issues/2240#issuecomment-6078423478, whose whole text is `1 — DR-cellctl-cobra`"
  - "`ls --cells-root <abs>` and `ls --cells-root=<abs>`. Before: exit 0, lists the default registry and ignores the words. Now: exit 3, refused. — item 2 of #2240 comment 6078394997, accepted by https://github.com/medici-finance/assay/issues/2240#issuecomment-6078423478, whose whole text is `1 — DR-cellctl-cobra`"
  - "A refused or relative registry selector in front of `model-policy hook …`. Before: exit 3, which the hook's caller reads as non-blocking. Now: exit 2, the hook's blocking code. — item 3 of #2240 comment 6078394997, accepted by https://github.com/medici-finance/assay/issues/2240#issuecomment-6078423478, whose whole text is `1 — DR-cellctl-cobra`"
  - "`help` and `help <verb>`. Before: `unknown verb 'help'`, exit 3. Now: the usage or the verb's help, exit 0. — item 4 of #2240 comment 6078394997, accepted by https://github.com/medici-finance/assay/issues/2240#issuecomment-6078423478, whose whole text is `1 — DR-cellctl-cobra`"
  - "`<verb> --help` and `<verb> -h`. Before: the word was taken as a cell or role, or ignored. Now: the verb's help, exit 0. — item 5 of #2240 comment 6078394997, accepted by https://github.com/medici-finance/assay/issues/2240#issuecomment-6078423478, whose whole text is `1 — DR-cellctl-cobra`"
  - "`-help`, `desk -help` and similar refusals. Before: exit 3 with the tool's own wording after the roster echo. Now: exit 3 with the parser's wording and no echo. — item 6 of #2240 comment 6078394997, accepted by https://github.com/medici-finance/assay/issues/2240#issuecomment-6078423478, whose whole text is `1 — DR-cellctl-cobra`. Corrected before, measured: `desk -help`, `desk --bogus` and `desk -x` exited 1, printing the `desk` usage line after the roster echo; they now exit 3 with the parser's wording and no echo. That exit-code change is accepted by https://github.com/medici-finance/assay/issues/2240#issuecomment-6079591256, whose whole text is `1 — DR-cellctl-cobra`, answering #2240 comment 6079520751, whose option 1 reads: '3 (recommended). The same usage-refusal code as every other verb. Cost: a caller that tested for exactly 1 from `desk` on a bad flag now sees 3; a caller that tested for non-zero sees no change.'"
  - "`scratch <cell> <action> --nosuch` and other `scratch` flag-parse failures. Before: exit 2. Now: exit 3. — item 7 of #2240 comment 6078394997, accepted by https://github.com/medici-finance/assay/issues/2240#issuecomment-6078423478, whose whole text is `1 — DR-cellctl-cobra`"
---

**This record transcribes a recorded human ruling. It is not an agent asserting sign-off.**
The human act is the driver's (`human:<name>`) five comments on decision issue
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
comment's last-updated time equals its creation time). A third comment by the driver, on
which list the answer covers, is set out under "Which list the answer covers" below.

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
checked from the issue. This record therefore does not rely on the relay for which list was
accepted. It relies on the driver's own later answer, set out next.

## Which list the answer covers

The driver's first two comments name no list, and the two lists above disagree on exit values.
Both reviews of the pull request that adds this record found that a desk relay cannot settle
which one option 1 covers. The desk App therefore put the question on the issue
([comment 6066226070](https://github.com/medici-finance/assay/issues/2240#issuecomment-6066226070),
2026-10-08T18:15:51Z, not edited). It is a desk note, not a ruling. It asks "**Which list does
option 1 cover?**" and gives two lettered options. Option a, verbatim:

> a. **(recommended) The "Deliberate differences" list in #2391's body, as it stood at 2026-10-08T16:27:52Z:**
>    - help, `-h`, `--version`, `version` and parse failures no longer print the effective-configuration (roster) echo;
>    - an unknown verb and a missing flag value use Cobra's wording, still exit 3, with no echo;
>    - `scratch ... --max-age bogus` exits 3 (was 2 from the Go flag package); the hook verb keeps 2;
>    - `--flag=value` and a bare `--` are accepted by every non-raw verb; a single-dash token that is not a known long flag is an error; `cadence recover` without a role is refused.

Option b was the issue body's own report, quoted in that comment as: "Unknown flags/commands,
malformed typed values and wrong positional counts return exit 2 before effects, replacing
inconsistent exit 1/3 or ignored extra arguments." The comment asks for the reply as "`a` or
`b`, in the driver's own login on this issue".

The driver's answer is
[comment 6066281586](https://github.com/medici-finance/assay/issues/2240#issuecomment-6066281586)
(login of the driver, account type User, 2026-10-08T18:19:07Z, not edited). Its whole text is
`a`.

The four list items in option a are the same text as the four items in #2391's body quoted in
the section above. That body's last edit is still 2026-10-08T16:27:52Z, so the list has not
changed since before the first answer.

None of these four items changes the hook verb's exit value. The third item says "the hook verb
keeps 2", and no other of the four names the hook verb. The second answer, set out under "The
second answer" below, accepts one change that does: decision entry 7, a refused or relative
selector in front of `model-policy hook` now exits 2, the hook's blocking code, where it exited 3.

## Reading, stated so it can be declined

The driver's first three comments say `1`, `1 — DR-cellctl-cobra` and `a`, and nothing else.
This record reads them as follows; the fourth is read under "The second answer" below and the
fifth under "The third answer".

1. `1` is option 1 of #2240 as the issue wrote it: "Accept the preserved source/admission
   boundaries and the explicitly listed help and parse changes, subject to independent review
   and all required validation passing."
2. `a` is option a of the question comment as that comment wrote it: "the explicitly listed
   help and parse changes" are the four items of the "Deliberate differences" list quoted
   under option a. Decision entries 1 to 4 of `accepted:` quote those four items and no others.
3. Option b, the issue body's own report (exit 2 for unknown flags and commands, malformed
   typed values and wrong positional counts), was offered in the same question and not taken.
   The record lists it as not accepted.

What remains a reading is small: that a one-letter reply names the option carrying that letter
in the comment directly above it, in full and as written. The question comment had not been
edited when this record was written, and the answer follows it by a little over three minutes
with nothing from the driver in between.

`ruling:` still links the second comment (`1 — DR-cellctl-cobra`), because the register's
check needs the linked comment's own text to name the record. The third comment does not name
it. It is cited here, by id, for which list the ruling covers.

If any part of this reading is wrong, the remedy is to decline the pull request that adds this
record. Nothing here lands until the driver merges it.

## The second answer: seven further differences

Review of #2391 found differences from the pre-migration binary that the four entries do not
name. Those a refusal could restore were restored at head `e69e02b15`. Seven could not be, and the
desk App put them to the driver
([comment 6078394997](https://github.com/medici-finance/assay/issues/2240#issuecomment-6078394997),
2026-10-09T09:41:03Z, not edited). It is a desk note, not a ruling. It lists the seven as items 1
to 7, each with its behaviour before and now, and gives three options: "1. **Accept all seven
(recommended).** They join the record's accepted list.", "2. **Mixed.** Name the item numbers to
restore; the rest are accepted." and "3. **Restore all seven.**" It asks for "one line naming the
record, for example `1 — DR-cellctl-cobra`".

The driver's answer is
[comment 6078423478](https://github.com/medici-finance/assay/issues/2240#issuecomment-6078423478)
(login of the driver, account type User, 2026-10-09T09:42:59Z, not edited). Its whole text is
`1 — DR-cellctl-cobra`.

This record reads `1` as option 1 of that question as it was written: all seven items, and
nothing beyond them. Decision entries 5 to 11 of `accepted:` quote items 1 to 7 in order, each
citing the answer. The question's own summary of what they change: "Items 1, 2 and 7 change an
exit code on input that was already wrong. Item 3 turns a malformed hook line from non-blocking
into blocking. Items 4 to 6 are help output." Option 1 of that question states no condition; the
condition on option 1 of #2240 is unchanged by it.

`ruling:` still links the comment it linked before. The answer above names the record too, and
each of the seven entries cites it by its full link. The record's `date:` stays the date of the
first ruling.

## The third answer: a corrected before for entry 10

Item 6 of the second question, quoted as decision entry 10, gave the before of `desk -help` as
exit 3. Review of #2391 at `c8b55034` ran the pre-migration binary and measured exit **1**:
`desk -help`, `desk --bogus` and `desk -x` printed the `desk` usage line after the roster echo
and exited 1. They now exit 3, with the parser's wording and no echo. The second answer was given
on the wrong before for that item, so the 1 → 3 change was accepted nowhere.

The desk App put the correction to the driver
([comment 6079520751](https://github.com/medici-finance/assay/issues/2240#issuecomment-6079520751),
2026-10-09T10:58:19Z, not edited). It is a desk note, not a ruling. It asks "for an unknown or
single-dash flag after `desk`, which exit code?" and gives two options:

> 1. **3 (recommended).** The same usage-refusal code as every other verb. Cost: a caller that
>    tested for exactly 1 from `desk` on a bad flag now sees 3; a caller that tested for
>    non-zero sees no change.
> 2. **1.** #2391 restores the old code for `desk`. Cost: one verb keeps its own usage-refusal
>    code, with parser code written to hold it there and a review round for it.

It adds: "Either way the changed wording and the missing roster echo stay as already accepted."
Under "Not asked" it names six further differences the same review measured and says: "The desk's
default is that #2391 restores the old behaviour for each, so nothing is owed on them unless that
proves impossible."

The driver's answer is [comment 6079591256](https://github.com/medici-finance/assay/issues/2240#issuecomment-6079591256)
(login of the driver, account type User, 2026-10-09T11:03:03Z, not edited). Its whole text is
`1 — DR-cellctl-cobra`. The desk App read it back
([comment 6079600045](https://github.com/medici-finance/assay/issues/2240#issuecomment-6079600045),
2026-10-09T11:03:39Z), a record, not a ruling.

This record reads `1` as option 1 of that question as written: a flag-shaped word after `desk`
exits 3, and nothing beyond that. Decision entry 10 of `accepted:` keeps item 6's quote, states
the measured before and quotes this answer. The six differences under "Not asked" are not
accepted by it; #2391 restores the old behaviour for each, and they are not entries of this record.

## The design the brief and the issue describe

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
- 2026-10-08T17:37:42Z and 17:43:28Z: the driver's two comments answering `1`.
- 2026-10-08T18:15:51Z: the desk App's question on which list option 1 covers.
- 2026-10-08T18:19:07Z: the driver's answer, `a`.
- 2026-10-09T09:41:03Z: the desk App's question on the seven remaining differences, at head
  `e69e02b15`.
- 2026-10-09T09:42:59Z: the driver's answer, `1 — DR-cellctl-cobra`.
- 2026-10-09T10:58:19Z: the desk App's correction of item 6 and its question on the exit code
  of a flag-shaped word after `desk`, at head `c8b55034`.
- 2026-10-09T11:03:03Z: the driver's answer, `1 — DR-cellctl-cobra`.

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
- **Any change to the listed behaviour that is not in the four items quoted under option a
  or the seven items of the second answer, as corrected by the third.** The driver's answers
  name those lists as the question comments quote them. A later change to #2391's list, or a difference from the
  pre-migration binary that neither list names, is not covered by them.
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
