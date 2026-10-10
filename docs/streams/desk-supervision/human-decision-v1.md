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
close succeeds. If that append fails, the close still stands, the forge copy holds the
record, and the audit line and stderr both say `decision-record: local-unwritten`.

A record that fails validation is **never written**. In that case the close proceeds with no
block and no line, and the audit line says `decision-record: invalid (<field>)`. A
`--dry-run` prints the comment, block included, and writes nothing. No other `deskclose`
lane writes a record.

Every audit outcome is named:

- `decision-record: forge+local` — both copies were written.
- `decision-record: local-unwritten (…)` — only the forge copy was written.
- `decision-record: invalid (<field>)` — nothing was written.
- `decision-record: unrecorded (…)` — the thread could not be read, so nothing was written.

## The fields

The table is in encoding order. That order is the struct's field order, which is why the two
copies are byte-identical.

| Field | Type | Meaning |
|---|---|---|
| `schema` | string | Always `human-decision-v1`. |
| `repo` | string | `owner/name` of the decision issue. |
| `issue` | int | The decision issue's number (> 0). |
| `tracker` | string | `owner/repo#N` (or `!N`): the `--tracker` ref the close names, where the decided work continues. |
| `brief` | string, optional | The brief id from the body's `<!-- needs-decision: <brief> -->` marker. Omitted when the body has no marker. |
| `options` | list of `{id, text_sha256}` | At most four. `id` is the option's letter **as the anchor ask wrote it** (`A`–`D` or `1`–`4`, uppercased). `text_sha256` is the sha256 hex of its cleaned text, never the text itself. Empty when the ask stated no options. |
| `options_source` | `body` \| `relay` | Which ask the options were read from (the **anchor**; see below). |
| `recommended` | id \| `none` | The id marked "recommended", or `none` when no option carried the mark. |
| `picked` | id \| `ambiguous` \| `unparsed` | The offered id the ruling names. See the pick rules below. |
| `picked_via` | `direct` \| `ratified-relay` | `direct`: the ruling's first line names a letter. `ratified-relay`: the ruling ratifies a relay's `Answer: <letter>`. |
| `picked_is_recommended` | bool | True only when `picked` is an id equal to `recommended`. |
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
It cannot know a dispatch reference, and it does not carry one.

## The anchor: which ask the options come from

The anchor is the **newest** comment before the ruling that is authored by a roster-trusted
App (login **and** account id pinned in the roster, `deskkit.TrustedAuthorID`), is not
minimized, and states an Options section. That is a relay restating the ask. If no such
comment exists, the anchor is the issue body.

A comment by any other author is never an anchor, however it is formatted.

## The pick rules

The ruling's lines are prepared the way the inbox oracle's `isruling` prepares them: HTML
comment lines and blank lines are dropped, and Markdown and control characters are stripped.
Then:

1. **A question or a hold is `unparsed`.** This covers a `?` anywhere in the ruling, and any
   of `no`, `not`, `don't`, `didn't`, `won't`, `hold`, `wait`, `later`, `undecided`, `unsure`,
   `pending`, `…n't`.
2. **Direct letter.** The first line is `<letter>` or `Option <letter>`, followed by the end
   of the line, `.`, `)`, `!`, `:`, `,` or a dash. The letter is read in the **anchor's own
   lettering**.
   - Against a **body** anchor, the letter is the body's own. A ruling of `B` on a body that
     offers A/B/C picks `B`.
   - Against a **relay** anchor whose issue body also states options, two other letterings
     are alternatives the ruler could have been reading: the body's own lettering, and the
     inbox walk's re-lettering of the body. The inbox walk puts the recommended option first
     as `A`. If any alternative names a different option from the anchor, the pick is
     `ambiguous`.
3. **Ratification.** The first line starts with `ratify`/`ratified`/`approve`/`approved`
   (optionally preceded by `I`). The ratification resolves through the **newest
   roster-trusted App comment** before the ruling that carries `Answer: <letter>`.
   - If no such relay exists, the pick is `unparsed`. An untrusted, minimized or wrong-id
     author's `Answer:` line is never read.
   - If that relay is itself the anchor, its letter is read in the relay's own lettering.
   - Otherwise, the letter may be the walk's re-lettering of the anchor (and, for a relay
     anchor, the body's own lettering or its walk). Any disagreement is `ambiguous`.
4. **Anything else is `unparsed`.** That includes a letter that no lettering offers.

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

`ValidateDecisionRecord` refuses a record that breaks the closed sets, so one that has
picked up a login or a text cannot be written. It refuses, naming the field:

- `ruler` ≠ `driver`;
- `picked` outside the offered ids ∪ {`ambiguous`, `unparsed`};
- an unknown `options_source` or `picked_via`;
- more than four options, or a duplicate or malformed id;
- a digest that is not 64 lowercase hex;
- a timestamp that is not RFC3339 UTC;
- latencies that disagree with the timestamps, or are negative;
- `picked_is_recommended` that disagrees with `picked`/`recommended`;
- any field containing a newline.

## Visibility

The forge copy lands on the close comment's own repo, and that repo's visibility can differ
from the visibility of the `tracker` item's repo.

`tracker` is the **only** repo-bearing field besides `repo` itself, and `repo` is the repo the
comment is posted on. `tracker` is already rendered in the close comment's prose ("tracker …
(verified present)"), and the outbound check scans the whole comment body, block included.
So the block discloses nothing the comment did not already carry, and `tracker` is exactly
as visible as the close comment itself.

The record holds no dispatch reference, no option or ruling text and no login.

## Authenticity

Any commenter can post a lookalike `human-decision-v1` block. A reader honours a block
**only** in the close comment authored by the closing role's own App identity, the
roster-trusted App that ran `deskclose`. A block in a comment by any other author is never
honoured.

When both copies exist, a reader prefers the local `decision-records.jsonl` line.

## What is not covered

- A decision issue closed by hand, outside `deskclose`, has no record, because no writer
  runs. The ask-decision skill (§Recording a ruling) routes ruled closes through the lane for
  this reason.
- `deskclose`'s other lanes (manifest, duplicate, superseded, self-withdraw, not-planned) close
  no single ruled decision and write no record.
