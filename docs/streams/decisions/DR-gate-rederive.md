---
id: DR-gate-rederive
date: "2026-10-08"
title: "Option c of #2405 is taken for a placeholder whose stored gate the issue's labels no longer match: detect only. The issue scan prints a notice and never changes a stored gate; the edit stays a hand edit"
consequence: minor  # the transcriber's classification, not part of the ruling; see "The consequence level"
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/2405#issuecomment-6070714706"
alternatives:
  - "Option a of #2405 — 'keep derive-once and add nothing; the triage instruction from brief 17 is the only cover and existing placeholders are left alone' (#2405 `### Fork test`, first `option:` line). It was the issue's stated default (`default: a`). Ruled out: the recorded answer is c; no rationale was recorded."
  - "Option b of #2405 — 'on every scan, raise a stored `gate: model` to `human` when the issue's current labels and title derive `human`; never lower a stored gate' (#2405 `### Fork test`, second `option:` line). Ruled out: the recorded answer is c; no rationale was recorded."
accepted:
  - "The consequence #2405 states for option c, quoted whole: 'a miss has a mechanical signal and the first run lists the existing placeholders as a back-fill worklist; the gate still changes only when someone acts on the notice' (#2405 `### Fork test`, third `option:` line)."
---

**This record transcribes a recorded human ruling. It is not an agent asserting sign-off.**
The human act is the maintainer's (`human:<name>`) comment on issue
[#2405](https://github.com/medici-finance/assay/issues/2405), posted under the maintainer's
own login and never a role App. This file copies that ruling into the register so that brief
statusgen/18 can cite it through `design:`. It mints no decision and supplies no rationale;
the ruling gave none.

## The recorded ruling

**The issue.** #2405, "scan-issues: is a placeholder gate ever re-derived after first write,
and are existing `risk:high` placeholders back-filled?", was opened by the desk App on
2026-10-08T17:40:39Z and its body has not been edited. It was still open when this record was
written. Its body carries no `decision-gate` marker; see "What could not be checked" below.

**What was asked.** The body puts two questions:

> 1. **Re-derivation.** The scanner derives a placeholder's gate once, when it first writes
>    the file. Should a label change after that ever update the stored gate? The cases are a
>    triage session scoring the item `risk:high`, an issue being reopened, and an excluded
>    label being removed.
> 2. **Back-fill.** Should placeholders that already exist for issues labelled `risk:high` be
>    rewritten to `gate: human`?

It then gives three options under `### Fork test`, and no more than three. Each is one line
with three parts. Verbatim:

> option: a — keep derive-once and add nothing; the triage instruction from brief 17 is the
> only cover and existing placeholders are left alone | works-because: it is today's behaviour
> plus brief 17, so no further change is needed and a stored gate is never rewritten by a tool
> | consequence: an item scored `risk:high` after its placeholder exists keeps `gate: model`
> unless the triage session follows the instruction and a person edits the file; nothing
> reports a miss

> option: b — on every scan, raise a stored `gate: model` to `human` when the issue's current
> labels and title derive `human`; never lower a stored gate | works-because: the scanner
> already holds the open issue's labels at the line where it skips an existing placeholder,
> and the scan pass already rewrites placeholder frontmatter when it retires or reactivates
> one | consequence: a late `risk:high` label takes effect on the next scan with no session
> involved, and the same pass back-fills every existing placeholder; it also raises
> placeholders whose labels or title carry the other gate words, and a `gate: model` set
> deliberately by a person on such an item is raised again unless a way to pin it is added

> option: c — detect only: the scan prints one notice per open issue whose labels and title
> derive `human` while its placeholder stores `gate: model`; the edit stays a hand edit |
> works-because: it needs the same read as option b and no write | consequence: a miss has a
> mechanical signal and the first run lists the existing placeholders as a back-fill worklist;
> the gate still changes only when someone acts on the notice

The same block says `default: a`. The body also records one constraint from review: "a
re-derivation should only ever raise a stored gate. A `gate: human` written by hand must
survive a pass that computes `model`."

**The answer.** Two comments, both by the maintainer, account type User, neither edited (each
comment's last-updated time equals its creation time):

- [comment 6070714706](https://github.com/medici-finance/assay/issues/2405#issuecomment-6070714706),
  2026-10-08T22:58:41Z. Its whole text is `c — DR-gate-rederive`.
- [comment 6070738723](https://github.com/medici-finance/assay/issues/2405#issuecomment-6070738723),
  2026-10-08T23:00:41Z. Its whole text is the same.

The first comment is the answer and names this record, so the record's `ruling:` link cites
it. The record's `date:` is the date of both comments (UTC).

The desk App then posted a note on the issue
([comment 6070790898](https://github.com/medici-finance/assay/issues/2405#issuecomment-6070790898),
2026-10-08T23:04:58Z). It is a desk note, not the ruling. It says: "**Answer:** c. No
rationale was given and none is supplied here."

## Reading, stated so it can be declined

The maintainer's comments say `c — DR-gate-rederive` and nothing else. This record reads them
as follows.

1. `c` is option c of #2405 as the issue wrote it: "detect only: the scan prints one notice
   per open issue whose labels and title derive `human` while its placeholder stores
   `gate: model`; the edit stays a hand edit".
2. Options a and b were offered in the same block and not taken. The issue's stated default,
   option a, did not apply because an answer was given.
3. Question 1 is answered by option c's own words: a label change after first write never
   updates the stored gate through the scan; the scan reports it.
4. Question 2 is answered by option c's stated consequence: no placeholder is rewritten by a
   tool, and "the first run lists the existing placeholders as a back-fill worklist".
5. The constraint from review holds without further work, because under option c the scan
   writes no gate at all.

What remains a reading is small: that a one-letter reply names the option carrying that letter
in the issue body, in full and as written. The body had not been edited when this record was
written.

If any part of this reading is wrong, the remedy is to decline the pull request that adds this
record. Nothing here lands until the maintainer merges it.

## What the brief builds from it

Brief statusgen/18
(`docs/streams/statusgen/brief-18-gate-mismatch-notice.md`) is the work. The brief is the
authority for the design; this section only says which of its choices go beyond option c's
words, so they can be seen and declined with the rest. None of them is part of the ruling.

- The notice reuses the scan's existing notice path: a `NOTICE:` line on stderr, printed with
  and without `--dry-run`, which never changes the exit code.
- "Stores `gate: model`" is read as the gate the placeholder parses to. A file with no
  `gate:` line whose stored labels read as `model` is reported.
- A placeholder that is retired when the scan reads it is not reported in that scan, even
  when the same scan reactivates it. The next scan reports it.
- The notice names the repository, the issue number and the placeholder file. It prints
  nothing an issue author wrote.

## The consequence level

`consequence: minor` is the transcriber's classification. It is not part of the ruling, which
states no level. The reason: the design adds a printed line and writes nothing, the brief's
four risk answers are all `no`, and if the design is wrong the worst case is the behaviour
before it, an item that keeps `gate: model` with no report. A reader who ranks by the cost of
a missed human gate, not by what the change itself can break, would say `major`. The
maintainer can change the level by declining this record or asking for the edit.

## What could not be checked

The register's online check (`README.md`, "How the approval is checked") passes a `ruling:`
link only when the linked issue is the record's decision issue: its body carries a
`decision-gate` marker for the record, or for a brief whose `design:` cites it. #2405's body
carries neither. As the issue stands, that check would be expected to refuse this link as
`unrelated-issue`. It was not run for this record. Every other condition the check names was
read by hand and holds: the comment exists, is on #2405 in this repository, was never edited,
was written by a User account and names this record's id. Whether to add the marker to the
issue body is not this record's call.

## What this record does NOT decide

- **Whether any existing placeholder is back-filled.** The notice's first run lists the
  candidates. Each change of a stored gate stays a hand edit, one item at a time, and no such
  edit is recorded here.
- **Any later move to option b.** Having the scan raise a stored gate was offered and not
  taken. Taking it later needs its own ruling.
- **A way to silence the notice** for an item a person deliberately keeps at `gate: model`.
  Option b's consequence mentions a pin; option c does not, and none is built.
- **How the notice reaches a reader.** Option c says the scan prints it. Whether a desk lane
  that runs the scan relays its output is not addressed by the ruling.
- **The same-repo transcriber lane**, which skips an existing placeholder the same way. The
  ruling names the scan.
- **Who may set or remove the `risk:high` label, and the gate vocabulary.** Both are as
  statusgen/17 left them.
- **The review and merge of the implementing change.** Those are separate acts.

And, per the register's own limits (`README.md`, "What a record does not establish"): a
checked `ruling:` link proves who ruled and where, not what the comment said. Whether this
record matches the ruling is the review gate's call on the pull request that adds it.
