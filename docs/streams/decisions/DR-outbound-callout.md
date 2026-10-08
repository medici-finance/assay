---
id: DR-outbound-callout
date: "2026-10-08"
title: "The outbound-write check may consult a deployment-supplied executable: option 1 of #2375, as proposed"
consequence: major
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/2375#issuecomment-6065679413"
alternatives:
  - "Option 2 of #2375, 'No REQUIRED mode' — as the issue states it: 'simpler; a misconfigured deployment silently runs built-in checks only and finds out from its merge gate, a review round trip later.' Ruled out: the ratified answer is option 1; no rationale was recorded and none is supplied."
  - "Option 3 of #2375, 'REQUIRED by default for public targets' — as the issue states it: 'strongest, but every adopter with no executable is refused on their first public write, which makes publishing the tools a breaking change.' Ruled out: the ratified answer is option 1; no rationale was recorded and none is supplied."
  - "Option 4 of #2375, 'Hold' — as the issue states it: 'handing full write text to a deployment-supplied executable needs more design.' Ruled out: the ratified answer is option 1; no rationale was recorded and none is supplied."
accepted:
  - "With no executable configured, only the built-in checks run (#2375 option 1: 'unset means built-in checks only')."
  - "An executable that is configured but broken refuses the write (#2375 option 1: 'configured-but-broken refuses the write')."
  - "Under the REQUIRED mode a deployment may set, public-target writes refuse when no executable is configured (#2375 option 1: 'a deployment may set a REQUIRED mode under which public-target writes refuse when no executable is configured')."
  - "The deployment-supplied executable receives the text of the write (#2375 option 1: 'the executable gets the text but a scrubbed environment with no credential'). The proposal names what else it is handed with the text: 'the verb, the role, the target repository and its visibility, the kind of write'. Brief desktools-v2/11 `gate-why` says the same of the design it proposes: it 'hands that executable the full text of every outward write'."
---

**This record transcribes a recorded human ruling. It is not an agent asserting sign-off.**
The human act is the driver's own comment on the decision issue, linked in `ruling:` above. This
file copies that ruling, and the text the issue put in front of the driver, into the register so
the lifecycle's design-approval gate (`spec/lifecycle-v1.md` §4.4) has a record to dereference.
It adds no rationale and decides nothing.

## The recorded ruling

- **Decision issue:** [#2375](https://github.com/medici-finance/assay/issues/2375), opened
  2026-10-08T12:15:55Z. Its body carries `<!-- decision-gate: assay:assay:desktools-v2:11 -->`,
  which names the work-item id of brief desktools-v2/11
  (`docs/streams/desktools-v2/brief-11-outbound-house-callout.md`).
- **The ruling comment:**
  [issue comment 6065679413](https://github.com/medici-finance/assay/issues/2375#issuecomment-6065679413),
  posted 2026-10-08T17:43:29Z under the driver's own login (a user account, not a role App)
  and not edited since (its last-updated time equals its creation time). Its whole text is:
  `1 — DR-outbound-callout`
- **What was asked:** the issue body. After its opening lines (the marker and a note on how to
  reply), it is the same text as the brief's `## Human decision` section: one paragraph
  describing the proposal, then four numbered options (1 "As proposed", 2 "No REQUIRED mode",
  3 "REQUIRED by default for public targets", 4 "Hold"), then the line
  `Default if no answer: none — blocks until answered.` The desk App's note on the issue
  ([issue comment 6065685697](https://github.com/medici-finance/assay/issues/2375#issuecomment-6065685697),
  2026-10-08T17:43:52Z) records how it was put: "the four options in this issue's body,
  unchanged: 1 as proposed, 2 no REQUIRED mode, 3 REQUIRED by default for public targets,
  4 hold. The driver was also told that the draft PR #2383 is already built on option 1." That
  note is the desk's account, posted after the ruling; the ruling comment itself says nothing
  about what the driver was told.

## Reading, stated so it can be declined

The ruling comment says `1` and names this record. This record reads `1` as option 1 of #2375
as written, in full. Option 1, verbatim from the issue body:

> 1. **As proposed** — unset means built-in checks only; configured-but-broken refuses the write;
>    a deployment may set a REQUIRED mode under which public-target writes refuse when no
>    executable is configured; 5 second default timeout, configurable to 60; the executable gets
>    the text but a scrubbed environment with no credential.

No rationale was given with the answer, and this record supplies none. If this reading is wrong,
the remedy is to decline the pull request that adds this record.

## The design the ruling's text describes

"As proposed" refers to the proposal in the issue body. The paragraph that carries it reads, in
full:

> The outbound-write check in the desk tools is generic: it cannot know the names one deployment
> withholds, and those names must never be compiled into or shipped with public tools. The
> proposal lets a deployment point the tools at its own executable. Before any write leaves the
> machine, after the built-in checks have passed it, the tools hand that executable one JSON
> object — the verb, the role, the target repository and its visibility, the kind of write, and
> the text being written — and it answers allow, or block with a reason. It can add a block; it
> can never clear one the built-in checks raised. The reason is shown locally and is never
> written to the forge or the audit log.

So, in the issue's words: the executable is consulted "after the built-in checks have passed"
the write; it "can add a block" and "can never clear one the built-in checks raised"; and the
reason "is shown locally and is never written to the forge or the audit log". Option 1 adds five
specifics:

- "unset means built-in checks only";
- "configured-but-broken refuses the write";
- "a deployment may set a REQUIRED mode under which public-target writes refuse when no
  executable is configured";
- "5 second default timeout, configurable to 60";
- "the executable gets the text but a scrubbed environment with no credential".

The brief holds the design in more detail than #2375 does. The ruling's text is the issue's
text. This record transcribes approval of what #2375's body states; where the brief says more
than the issue, this record does not say it was ruled, and it does not amend the brief. Whether
the ruling reaches the whole brief is the driver's to say, in the driver's own login on #2375.

## The consequence level

`consequence: major` is the transcriber's classification. It is not part of the ruling: the
ruling comment states no level. It was taken from two things in brief desktools-v2/11, read
against the register's definition of `major` (`spec/registers-v1.md` §7.2: "Getting it wrong
blocks a named user or forces an expensive rework, but a route back exists."):

- the brief's `gate-why`: "A wrong default either blocks every desk in a deployment (required,
  but the pod never mounted it) or silently runs compiled-only while the operator believes their
  sweep is in force.";
- the brief's `risk` answers: `regulatory: no`, `customer: no`, `irreversible: no`,
  `sensitive-data: yes`.

A reviewer of the pull request that adds this record may ask for a different level without
touching the ruling.

## The order of events

The implementation was written before the design was approved. These are the recorded times:

- 2026-10-08T12:15:55Z: #2375 opened. The brief's `decision-trigger` is `start`.
- 2026-10-08T12:43:28Z: the implementing pull request
  [#2383](https://github.com/medici-finance/assay/pull/2383) opened as a draft. Its first commit
  (`27a0b4e16`) was committed at 12:42:29Z. Its body says:

  > Implements Option 1 "as proposed" of the brief; the brief is gate:human and the decision is
  > #2375, so this stops at `implemented` and pre-empts nothing.

- 2026-10-08T16:45:05Z: the commit that is #2383's head as this record is written
  (`e73d79825`).
- 2026-10-08T17:43:29Z: the ruling.

`spec/lifecycle-v1.md` §4.4 places the design-approval gate before implementation begins. Here
the pull request was opened and built first, and the ruling came about five hours later. This
record does not change that order and should not be read as saying the design was approved
before it was built. On the main branch this record was written against, the brief's board row
reads `todo`.

## What this record does NOT decide

- **The review of #2383.** As this record is written, #2383 is an open draft with one review:
  changes requested by the reviewer App, submitted 2026-10-08T14:35:52Z at commit `fe453836a`.
  The head has since moved to `e73d79825`, and there is no review at that head. That review
  continues on #2383. This record is not a verdict on the diff.
- **Implementation details in #2383 that option 1 does not name.** #2383's body states the
  following, and none of them appears in the text of #2375. They stand on the pull request
  under review, not on this record:
  - the three roster key names (`ASSAY_OUTBOUND_CALLOUT`, `ASSAY_OUTBOUND_CALLOUT_REQUIRED`,
    `ASSAY_OUTBOUND_CALLOUT_TIMEOUT`) and their startup echo;
  - that the REQUIRED mode also refuses targets of unknown visibility (option 1 says
    "public-target writes");
  - that the REQUIRED setting "is also read from the environment";
  - that the refusal "is not overridable by `--force-scan-override`";
  - that a push is put to the executable "N+2" times for N commits (the branch name, each
    commit message, the added lines);
  - that the executable "runs with exactly PATH, HOME, TMPDIR, LANG" (option 1 says "a scrubbed
    environment with no credential" and names no variables);
  - how the answer is parsed ("the first whitespace-delimited word is exactly `allow` or
    `block`"), the 64 KiB limit on its output, and that the request text is sent "without HTML
    escaping";
  - what the audit row holds ("the rule id, outcome, kind and a digest"); the issue says only
    what the audit log does not hold;
  - which outcomes refuse the write: "missing file, group/world-writable, non-zero exit,
    timeout, empty output, more than 64 KiB, an answer that is neither word" (option 1 says
    "configured-but-broken refuses the write" and gives a timeout; it does not say a timeout
    counts as broken);
  - that "A key that is set but malformed refuses outward writes instead of reading as
    unconfigured";
  - that a write "with no text" is still put to the executable;
  - that what the executable printed is shown "with control characters removed".

  This list is not complete. It names the differences found when this record was written;
  #2383's body is the full statement.

  Some of these are stated in the brief. Whether #2383 matches the brief is the review's
  question, not this record's.
- **Which executable any deployment configures.** The brief puts that out of scope ("any
  deployment's executable or token map"); nothing of the kind is decided or named here.
- **The human sign-off on the brief's later status changes.** The brief's `## Review` section
  reads "MANDATORY human sign-off". The ruling answers the options in `## Human decision`; it
  is not a sign-off on finished work, and none is recorded here.
- **The merge of #2383.** The desk App's note on #2375 says the ruling does not cover "the
  merge, which stays the driver's".
- **Whether the design is correct.** Per the register's own limits (`README.md`, "What a record
  does not establish"), a checked `ruling:` link proves who ruled and where. Whether this record
  matches the ruling is the review gate's call on the pull request that adds it.
