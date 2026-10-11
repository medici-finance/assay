# human-decision-v1 — the human decision record

When the driver rules on a decision issue, this record captures which options the ask
offered, which one the desk recommended, which one the ruling picked, and how long the
decision waited. It is what makes "how often is the recommended default taken?" and "how
long does a human gate hold work up, per kind of decision?" answerable without re-reading
free-text threads.

Implemented in `tools/desk/internal/deskkit/decisionrecord.go`. The writer is `deskclose`'s
`triage --disposition human-decided` lane (`tools/desk/cmd/deskclose/decisionrecord.go`).
Brief: `brief-32-human-decision-record.md`.

## Where it lives: two copies, one set of bytes

One record is written for each ruled close, and it is written twice by independent paths:

1. **Forge copy.** A hidden block appended to the close comment `deskclose` already posts,
   so it costs no extra write:

   ```
   <!-- human-decision-v1 {"schema":"human-decision-v1","repo":"…",…} -->
   ```

   The JSON sits on one line. `encoding/json` escapes `<`, `>` and `&`, so the payload can
   never close the HTML comment early.
2. **Local copy.** The same JSON bytes, as one line of `decision-records.jsonl` in the desk
   state directory (`~/.config/assay/`, beside `audit.jsonl`). The file is mode 0600 and is
   only ever appended to (`O_APPEND`), one `write` per line.

Either copy survives the loss of the other. The local line is appended only **after** the
close is confirmed; a close that fails or does not take leaves no local line. If the append
fails, the close still stands, the forge copy holds the record, and the audit line and
stderr both say `decision-record: local-unwritten`.

The record never stops the close:

- A record that fails validation is **never written**. The close proceeds with no block and
  no line, and the audit line says `decision-record: invalid (<field>)`.
- If the outbound check refuses the close comment **with** the block (for example, a `brief`
  that names a withheld identifier on a public repo), the check refused before anything was
  written. The lane posts the same comment **without** the block and closes. The local line
  is then the only copy.
- If the thread cannot be re-read, or the re-read no longer carries the ruling, no record is
  composed and the close proceeds.

The record adds no other failure. A comment write that fails for any other reason fails the
close exactly as it would with no record. A `--dry-run` prints the comment, block included,
and writes nothing. No other `deskclose` lane writes a record.

Every audit outcome is named:

- `decision-record: forge+local` — both copies were written.
- `decision-record: local-unwritten (…)` — only the forge copy was written.
- `decision-record: local only (the outbound check refused the block)` — only the local copy
  was written.
- `decision-record: unrecorded (the outbound check refused the block; local-unwritten) (…)` —
  neither copy was written.
- `decision-record: invalid (<field>)` — nothing was written.
- `decision-record: unrecorded (…)` — the thread could not be re-read, or no longer carries
  the ruling, so nothing was written.

**A re-run can write a second record.** If the comment posts and the close then fails, a
re-run of the lane posts a second close comment with its own block, and a second local line.
Records with the same `repo`, `issue` and `ruling_sha256` describe the same ruling. A reader
takes the one with the newest `recorded_at`.

## The fields

The table is in encoding order. That order is the struct's field order, which is why the two
copies are byte-identical.

| Field | Type | Meaning |
|---|---|---|
| `schema` | string | Always `human-decision-v1`. |
| `repo` | string | `owner/name` of the decision issue. |
| `issue` | int | The decision issue's number (> 0). |
| `tracker` | string | `owner/repo#N` (or `!N`): the `--tracker` ref the close names, where the decided work continues. |
| `brief` | string, optional | The brief id from the body's `<!-- needs-decision: <brief> -->` marker. Read only from a marker that is a whole line of its own (surrounding spaces allowed; not inside a sentence or a `>` quote). The id must match `<stream>/<NN>`: a lower-case stream of 1–64 characters (`a-z`, `0-9`, `-`, starting with a letter or digit), `/`, and 1–4 digits. Omitted when there is no such marker, when the id breaks that grammar, or when two markers name different ids. |
| `options` | list of `{id, text_sha256}` | At most four. `id` is the option's letter **as the anchor ask wrote it** (`A`–`D` or `1`–`4`, uppercased). `text_sha256` is the sha256 hex of its cleaned text, never the text itself (cleaning: below). Empty when the ask stated no options. |
| `options_source` | `body` \| `relay` | Which ask the options were read from (the **anchor**; see below). |
| `recommended` | id \| `none` | The id marked "recommended", or `none` when no option carried the mark. |
| `picked` | id \| `ambiguous` \| `unparsed` | The offered id the ruling names. See the pick rules below. |
| `picked_via` | `direct` \| `ratified-relay` | `direct`: the ruling's first line names a letter. `ratified-relay`: the ruling ratifies a relay's `Answer: <letter>`. |
| `picked_is_recommended` | bool | True only when `picked` is an id equal to `recommended`. |
| `edit_state` | `unedited` \| `edited` \| `unverifiable` | Whether any text the record was read from changed after the ruling. See "Edits after the ruling" below. |
| `ruler` | string | The literal role string `driver`. Never a login or account id. |
| `ruling_sha256` | string | The sha256 hex of the ruling comment's body. |
| `opened_at` | RFC3339 UTC | The decision issue's creation time, as the forge reports it. |
| `asked_at` | RFC3339 UTC | When the anchor ask was posted: equal to `opened_at` for a body anchor, the relay's creation time for a relay anchor. |
| `ruled_at` | RFC3339 UTC | The ruling comment's creation time. |
| `recorded_at` | RFC3339 UTC | When `deskclose` composed the record. |
| `opened_to_ruled_s` | int | `ruled_at − opened_at`, in seconds. Never negative. |
| `asked_to_ruled_s` | int | `ruled_at − asked_at`, in seconds. Never negative. |
| `tool_sha` | string | The build's source SHA (the value every audit entry carries; `unpinned` for an unstamped build). |

**Join keys.** The record joins by `repo` + `issue`, by `tracker`, and by `brief` when present.
It cannot know a dispatch reference, and it does not carry one. `brief` comes from the issue
body, so its value is whoever wrote the body's marker; the grammar bounds its shape, not who
set it.

**Option text cleaning.** An option's text is the rest of its list line after the letter and
its `.` or `)`. Then, in order: `**` and backticks are removed; control characters are
removed; surrounding whitespace is trimmed; a trailing `(recommended)` / `— recommended`
mark and a leading `(recommended)` mark (either case of `R`) are removed; whitespace is
trimmed again. `text_sha256` is the sha256 of the UTF-8 bytes of that result. An option
whose cleaned text is empty is dropped.

## The anchor: which ask the options come from

"Before the ruling" means before the ruling's own position in the thread, located by its
comment id. If the thread does not carry the ruling, nothing comes before it: `deskclose`
writes no record, and a composed record reads `picked: unparsed`.

The anchor is the **newest** comment before the ruling that is authored by a roster-trusted
App, is not minimized, and states an Options section. That is a relay restating the ask. A
roster-trusted App is an App-shaped login (`…[bot]` or `app/…`) **and** a non-zero account
id that matches the roster's pin. A comment whose author id the forge did not report is not
a trusted App. If no such comment exists, the anchor is the issue body.

A comment by any other author is never an anchor, however it is formatted. That includes a
roster-trusted human who restates the options.

**A body anchor's author is not checked.** The issue body is read whoever wrote it. When the
anchor is the body, `options` and `recommended` are what the body states. `recommended` is
the desk's recommendation only when the desk wrote the body.

## The pick rules

The ruling's lines are prepared the way the inbox oracle's `isruling` prepares them: HTML
comment lines and blank lines are dropped, and Markdown and control characters are stripped.

The oracle only screens an item for a desk that then reads the comment. Nobody reads the
comment behind a record, so the record's reading is stricter. Whenever a person could read
the ruling as anything other than one definite option, the record says `unparsed`:

1. **A question, a refusal, a hold or a qualifier is `unparsed`.** This covers a `?`
   anywhere in the ruling, and any of these words anywhere in it: `no`, `not`, `none`,
   `neither`, `nor`, `don't`, `didn't`, `won't`, `hold`, `wait`, `later`, `undecided`,
   `unsure`, `pending`, `reject`/`rejects`/`rejected`/`rejecting`, `except`, `instead`,
   `unless`, `rather`, `but`, `however`, `actually`, `…n't`.
2. **Direct letter.** The first line is `<letter>` or `Option <letter>`, followed by the end
   of the line, `.`, `)`, `!`, `:`, `,` or a dash. If the ruling names any **other** offered
   id anywhere, the pick is `unparsed`. An id is named when it stands alone as a token, so
   `A, B`, `B` then `go with C` on a later line, and `2-3 days` all name more than one id.
   The ids counted are the anchor's, plus the body's when the anchor is a relay. Otherwise
   the letter is read in the **anchor's own lettering**.
   - Against a **body** anchor, the letter is the body's own. A ruling of `B` on a body that
     offers A/B/C picks `B`.
   - Against a **relay** anchor whose issue body also states options, two other letterings
     are alternatives the ruler could have been reading: the body's own lettering, and the
     inbox walk's re-lettering of the body. The inbox walk puts the recommended option first
     as `A`. If any alternative names a different option from the anchor, the pick is
     `ambiguous`.
3. **Ratification.** The first line starts with `ratify`/`ratified`/`approve`/`approved`
   (optionally preceded by `I`). A ratification that names any offered id itself
   (`Approved — B`) is `unparsed`. It resolves through the **newest roster-trusted App
   comment** that is at or after the anchor, is before the ruling, and carries an `Answer:`
   line of its own.
   - Only the relay's own lines count: an `Answer:` inside a `>` quote is someone else's
     text and is never read.
   - That relay's answer must be one offered letter. If it is a word (`Answer: yes, go
     ahead`), or the relay gives two different answers, the pick is `unparsed`. The search
     never skips a newer relay to reach an older one.
   - The search never reaches a relay older than the anchor. A ratification after a new ask
     with no answer of its own is `unparsed`.
   - If no such relay exists, the pick is `unparsed`. An untrusted, minimized or wrong-id
     author's `Answer:` line is never read.
   - If that relay is itself the anchor, its letter is read in the relay's own lettering.
   - Otherwise, the letter may be the walk's re-lettering of the anchor (and, for a relay
     anchor, the body's own lettering or its walk). Any disagreement is `ambiguous`.
4. **Anything else is `unparsed`.** That includes a letter that no lettering offers.

## Edits after the ruling

The record states what stood when the driver ruled, or says it cannot tell. `edit_state`
compares these edit times with the ruling's creation time:

- the issue body's last edit (always read: the options, the alternative letterings, the
  brief);
- the anchor relay and the relay a ratification resolved through, when either was read;
- the ruling itself (edited at all after it was posted).

If any of them was edited after the ruling, `edit_state` is `edited`, and the record carries
`picked: unparsed` and `recommended: none`. The values read now may not be the ones the
ruler saw. Otherwise, if the forge did not report any of those edit times, `edit_state` is
`unverifiable`, and the values stand as read. Otherwise it is `unedited`.

A forge can move an update time for reasons other than a text edit, so `edited` errs toward
saying less, never more.

## Lettering

A direct letter against a body anchor is read as the body's own letter, not flagged
`ambiguous`. The brief's Verify row 2 fixes this: options A/B/C with B recommended, ruled
`B`, records `picked: B`. The ask-decision format puts the recommended option first (§The
format). Under that format the walk's lettering and the body's are the same, so the reading
is unambiguous by construction. Only a relay, which the ruler may have read in walk order,
introduces a second lettering.

## Never recorded

The record carries digests, closed-set values, timestamps and the join keys. It never
carries:

- any option text, even though the options are recorded by digest;
- the ruling text, or the text of any other comment;
- any login or account id, of the ruler, the desk App, a relay author or anyone else
  (`ruler` is the role `driver`);
- any per-person aggregate. Analysis groups by decision class, brief or stream. The records
  are never a target for ranking the driver, the desk or any agent;
- any model name.

`ValidateDecisionRecord` refuses a record that breaks the closed sets or the grammars. Every
string field is a closed-set value, a digest, a timestamp, the `owner/name` refs the close
comment already names, or `brief`. `brief` is held to the brief-id grammar and 64 characters,
which bounds its shape but not its meaning: a short word can satisfy it. The validator refuses,
naming the field:

- `ruler` ≠ `driver`;
- `picked` outside the offered ids ∪ {`ambiguous`, `unparsed`};
- an unknown `options_source`, `picked_via` or `edit_state`;
- `edit_state: edited` with a `picked` other than `unparsed` or a `recommended` other than
  `none`;
- a `brief` outside the brief-id grammar;
- more than four options, or a duplicate id, or an id other than one of `A`–`D` / `1`–`4`;
- a digest that is not 64 lowercase hex;
- a timestamp that is not RFC3339 UTC;
- latencies that disagree with the timestamps, or are negative;
- `picked_is_recommended` that disagrees with `picked`/`recommended`;
- any field containing a newline.

## Visibility

The forge copy lands on the close comment's own repo, and that repo's visibility can differ
from the visibility of the `tracker` item's repo.

`repo` is the repo the comment is posted on. `tracker` is already rendered in the close
comment's prose ("tracker … (verified present)"). `brief` can name a stream, and a stream
name can be an identifier that must not appear on a public repo.

The outbound check scans the whole comment body, block included. If it refuses the block,
the lane posts the comment without it, and the local line is the only copy (see "Where it
lives"). The forge copy therefore never carries anything the outbound check refuses.

The record holds no dispatch reference, no option or ruling text and no login.

## Authenticity

Any commenter can post a lookalike `human-decision-v1` block. A reader honours a block
**only** in the close comment authored by the closing role's own App identity, the
roster-trusted App that ran `deskclose`. A block in a comment by any other author is never
honoured.

When both copies exist, a reader prefers the local `decision-records.jsonl` line.

The local file is refused, not written through, when the path is a symlink or any other
non-regular file, including one swapped in while the file was opened. An existing file is
set back to 0600 on each append. Each line is one short `O_APPEND` write; there is no lock.

## What is not covered

- A decision issue closed by hand, outside `deskclose`, has no record, because no writer
  runs. The ask-decision skill (§Recording a ruling) routes ruled closes through the lane for
  this reason.
- `deskclose`'s other lanes (manifest, duplicate, superseded, self-withdraw, not-planned) close
  no single ruled decision and write no record.
- `edit_state: unverifiable` keeps the values as read. A forge that reports no edit times
  gives a record that cannot say whether the body or a relay changed after the ruling.
- A body anchor's options and recommended mark are the body author's. The record does not
  check who wrote the body.
- The pick reading is conservative, not complete. A ruling in a form the grammar does not
  know is `unparsed` even when a person would read one option from it.
