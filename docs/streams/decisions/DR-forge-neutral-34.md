---
id: DR-forge-neutral-34
date: "2026-10-08"
title: "Option 1 of the decision issue is taken for deskread's CI workflow-token transport: approve as briefed, an explicit, CI-only, read-only opt-in beside the App custody default"
consequence: major  # the transcriber's classification, not part of the ruling; see "The consequence level"
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/2315#issuecomment-6070714987"
alternatives:
  - "Option 2 of the decision issue — 'Approve with changes. Name the change (for example: require a second CI marker, drop the token-shape check, widen to other repositories). The brief is revised before dispatch.' (#2315 `Options:`, item 2). Ruled out: the recorded answer is option 1; no rationale was recorded beyond the answer's own text."
  - "Option 3 of the decision issue — 'Hold. The CI checks keep using the separate command-line client; the follow-on change cannot finish its last step.' (#2315 `Options:`, item 3). Ruled out: the recorded answer is option 1; no rationale was recorded beyond the answer's own text."
accepted:
  - "Each entry is a cost the decision issue itself states; none is added by this record. The issue's own words on what the layers are worth: 'The CI markers, the \"own repository\" and the run id all come from the same environment, so anyone running the tool on their own machine can set them.' (#2315 comment 6014966229)"
  - "'Against a forger, the single control is the read-only client together with the closed kind list. Nothing stands behind it that a forger cannot also satisfy.' (#2315 comment 6014966229)"
  - "'The shape check ... Every app installation token passes it, including one minted on a workstation and the board-writer token minted inside the auto-flip job.' (#2315 comment 6014966229)"
  - "'The identity record is not proof.' The repository and run id it carries 'are copied from the environment and are not verified.' (#2315 comment 6014966229)"
  - "'Under `--ci-workflow-token`, `deskread` reads its trusted-account roster from the job environment. That roster is forgeable input in the same way as the CI markers, the repository binding and the run id.' (#2315 comment 6015793839, which adds: 'Option 1 (approve as briefed) accepts both as residuals.')"
  - "'The scaffolded workflow narrows `permissions:` only on jobs that hand the token to `deskread`. The regen/board job keeps `contents: write` because it commits, so the server-side read-only layer does not cover that job.' (#2315 comment 6015793839)"
  - "'The ambient-token ratchet rises from 4 to 5. The register of functions allowed to read a token from the environment grows from 4 permits to 5, against a stated target of 0.' (#2315 comment 6014966229)"
  - "'Without the flag, nothing changes.' (#2315 issue body)"
---

**This record transcribes a recorded human ruling. It is not an agent asserting sign-off.**
The human act is the driver's (`human:<name>`) comments on decision issue
[#2315](https://github.com/medici-finance/assay/issues/2315), each posted under the driver's
own login and never a role App. This file copies that ruling into the register for the
lifecycle's design-approval gate (`spec/lifecycle-v1.md` §4.4). It mints no decision and
supplies no rationale; the ruling gave none.

## The recorded ruling

**The issue.** #2315, "Decision: assay:assay:forge-neutral:34 — deskread CI workflow-token
transport", was opened 2026-10-06T10:45:52Z by the reviewer App. Its body carries
`<!-- decision-gate: assay:assay:forge-neutral:34 -->`, the decision-gate marker of brief
forge-neutral/34
(`docs/streams/forge-neutral/brief-34-deskread-ci-workflow-token-transport.md`), which cites
this record through `design:`. The forge reports no edit to the issue body (its last-edited
time is empty). The issue is closed; it was closed by the desk App's comment of
2026-10-07T03:00:11Z, which says "Closing on the driver's own ruling above (option 1, approve
as briefed)".

**What was asked.** The body says: "A human decision is needed on work item
`assay:assay:forge-neutral:34`" and "reply with the chosen option as a comment (the comment
is the decision record), then close the issue." It describes the exception and then puts
three options, and no more than three:

> 1. **Approve as briefed.** The transport is built exactly as above; the CI checks then move
>    onto the tool in a follow-on change.
> 2. **Approve with changes.** Name the change (for example: require a second CI marker, drop
>    the token-shape check, widen to other repositories). The brief is revised before dispatch.
> 3. **Hold.** The CI checks keep using the separate command-line client; the follow-on change
>    cannot finish its last step.

It ends: "Default if no answer: none — blocks until answered".

**The answer.** Three comments by the driver, each with account type User, none edited (each
comment's last-updated time equals its creation time).

- [comment 6029982445](https://github.com/medici-finance/assay/issues/2315#issuecomment-6029982445),
  2026-10-07T02:59:26Z. Its whole text is `Option 1: approve as briefed.` (the comment body
  begins with one space).
- [comment 6070714987](https://github.com/medici-finance/assay/issues/2315#issuecomment-6070714987),
  2026-10-08T22:58:42Z. Its whole text is `1 — DR-forge-neutral-34`.
- [comment 6070738945](https://github.com/medici-finance/assay/issues/2315#issuecomment-6070738945),
  2026-10-08T23:00:42Z. Its whole text is `1 — DR-forge-neutral-34`, identical to the
  previous one.

The first comment is the answer. It does not name this record, and the register's check
requires the linked comment's own text to name the record's id (`README.md`, "How the approval
is checked"). The record's `ruling:` link therefore cites the second comment, which gives the
same answer and names the record. The third repeats the second and changes nothing. It is
the second, comment 6070714987, that this record names as the ruling that names the record id.
The record's `date:` is the date of the second and third comments (UTC).

**Two desk notes on the issue are not the ruling.** On 2026-10-06 the desk App posted the
issue's "Correction" and a further residuals update (comments 6014966229 and 6015793839);
they are the issue's own account of the layers and of the residuals, which `accepted:` above
quotes. On 2026-10-07 it posted a relay (comment 6029957615) that begins "Ruling relayed from
the driver" and says of itself: "this relay is the record only. It does not satisfy the human
gate, which needs the driver's own comment on this issue." This record does not rely on the
relay. The relay's statement that both residuals were accepted is not the driver's own text.
What the driver's own text says is option 1, "approve as briefed"; the issue's comment 6015793839
says option 1 accepts both residuals.

## The design the brief and the issue describe

The brief is the authority for the design. This summary quotes the issue and does not amend
either.

- The issue's own statement of what the decision ratifies: "This decision ratifies the exact
  shape of that exception before it is built."
- The exception, as the issue states it: the caller passes an explicit flag; the token arrives
  in a dedicated variable, never the usual token variables; the process sees the CI
  environment markers; the token has the shape of an app installation token, not a personal or
  OAuth one; the repository read is the one the environment names as the job's own; the job
  was not started by a `pull_request_target` event; and the read is one of the read-only kinds
  on a fixed, closed list. "The forge client it builds refuses every write, and it refuses by
  default any method nobody has classified yet."
- The threat model, as the issue states it: "The threat model is accidental or ambient use in
  an honest CI job. It is not a local caller who deliberately forges the CI environment."
- Two further relaxations the issue names so they are ratified and not inferred: under the
  flag inside CI the tool reads its trusted-account roster from the job's environment instead
  of from a file in the home directory; and the ambient-token ratchet rises from 4 to 5.

## The order of events

The ruling came before the implementing pull request was opened. The record that names it
came after.

- 2026-10-06T10:44:07Z: the brief was proposed in #2314, which merged 2026-10-07T01:45:44Z.
- 2026-10-06T10:45:52Z: #2315 was opened.
- 2026-10-07T02:59:26Z: the driver's first comment, option 1.
- 2026-10-08T12:30:14Z: #2377, the implementing pull request, was opened as a draft; it is
  still a draft when this record is written.
- 2026-10-08T22:58:42Z and 23:00:42Z: the driver's two comments naming this record.

## The consequence level

`consequence: major` is the transcriber's classification. It is not part of the ruling, which
states no level. It was taken from two things in the brief: its `gate-why` ("This brief
relaxes a security posture on purpose.") and its `risk` answers (`sensitive-data` is `yes`;
`regulatory`, `customer` and `irreversible` are `no`). The driver can change the level by
declining this record or asking for the edit.

## What this record does NOT decide

- **The human sign-off on the brief's `verified` and `done` flips.** The brief is
  `gate: human`; those sign-offs are separate acts with no recorded answer here.
- **Anything in the implementing pull request, #2377, beyond the brief.** The ruling is on the
  shape the brief describes ("as briefed"). A difference between that pull request and the
  brief, and the pull request's review and merge, are not covered by it.
- **The follow-on change** that moves the CI checks onto the tool, and any change to the
  scaffolded workflow. The issue says they come after this one.
- **Whether the design is correct.** That is the review gate's judgement and then the
  change's own validation.

And, per the register's own limits (`README.md`, "What a record does not establish"): it does
not by itself prove the approver differs from the brief's author, and a checked `ruling:` link
proves who ruled and where, not what the comment said. Whether this record matches the ruling
is the review gate's call on the pull request that adds it. Nothing here lands until the
driver merges it.
