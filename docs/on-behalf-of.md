# On-behalf-of: the composite-identity principal trailer

**Status: initial landing (multi-principal/01).** One trailer format, six writers, a
witness annotation, and a lint. This document is the format spec, the resolution
contract, and — as important — what the trailer does **not** prove.

## Why

Every desk role shares one GitHub App across every session that ever runs it. A review
posted by the reviewer App, an Evidence row committed by the verifier App, a PR opened by
the worker App — none of it says whose fleet, whose driver, whose session actually
produced the write. GitLab's shared-service-account audit model ships the fix to copy:
every audited action records "service account **on behalf of** `@human`". This is that
model, generalised to a fleet that has not yet grown a second human, so the plumbing is in
place before it needs to disambiguate between two.

## The trailer

```
On-behalf-of: human:<who>
On-behalf-of: human:<who> mode:unattended
```

- `human:<who>` — the human the write is on behalf of, in the form the TARGET REPO'S
  VISIBILITY calls for: the lowercased human **login** on a repo the roster states is
  `:private`, and the roster's neutral **name** for that same human — the
  `ASSAY_HUMAN_LOGIN_MAP` key, i.e. the given name the map already carries as that
  human's public form — on every other target. See "Which form" below.
- ` mode:unattended` — appended only when the resolving session carried no live
  interactive roster beacon (a cron loop: `scanloop`, `verifyloop`, …, as opposed to a
  desk/worker session with a registered role). Its absence means the session was
  attended.

It appears in three shapes, depending on the surface:

| Surface | Shape |
|---|---|
| A git commit (an Evidence row) | a trailing line in the commit message, blank-line separated |
| An issue or PR comment / body | a trailing line in the body, blank-line separated |
| A witness Runner cell (`## Evidence` table, `statusgen verifyrun`) | `on-behalf-of human:<who>` inside the cell's trailing parenthetical, e.g. `assay-verifier-app[bot] @ a1b2c3d (on-behalf-of human:ada) (ci-env)` |

The witness-cell form omits the `On-behalf-of:` key (it is a table cell, not a
line-oriented record) and is never suffixed with `mode:unattended` — `verifiedrunneragree.go`'s
`runnerKey` already drops everything from the first `(` onward, so any text placed inside
a trailing parenthetical never perturbs the runner-agreement comparison.

## Which form: the target repo's visibility decides

A trailer stamped onto a write to a **public** repo is world-readable for good — a posted
review cannot be edited afterwards, and a commit message cannot be taken back out of
history. So the form the trailer names is chosen from the target repo's **configured**
visibility (`ASSAY_ALLOWED_REPOS`' `:public` / `:private` token, read with no network call
— the same value `VisibilityRiskClassed` reads):

| Target | The trailer names |
|---|---|
| A repo the roster states is `:private` | the mapped login — unchanged |
| Everything else — `:public`, an unstated visibility, a repo admitted only by an `owner/*` pattern, a repo the roster does not carry, or no repo at all | the roster's neutral **name** for that human |

The rule is stated in the fail-closed direction on purpose, exactly as
`VisibilityRiskClassed` states its own: *everything except a repo declared private takes the
neutral form.* A wrong "public" costs audit precision; a wrong "private" is a disclosure
that cannot be withdrawn. Every entry point therefore takes the target repo as a
**mandatory** argument — a verb cannot resolve a principal without saying where the write
is going, and a package test (`TestEveryWriteVerbNamesItsTarget`) reads the verbs' source
to hold that.

**One renderer per module, held by a class guard.** The visibility split only protects a
public repo if nothing ELSE renders the trailer. Two structural tests parse their module's
non-test source and fail on any second route: `TestOnBehalfOfRenderedOnlyThroughTheResolver`
(`tools/desk/internal/deskkit/principal_class_test.go`) and its statusgen twin
`TestWitnessOnBehalfOfRenderedOnlyThroughTheRenderer` (`statusgen/principal_class_test.go`).
Each reports a string literal that spells the trailer or the witness marker outside the one
renderer file, a use of `OnBehalfOfPrefix` to compose one, and a literal repo handed to the
resolver, and each holds every resolver call site to a reviewed allow-list keyed by file,
function and repo argument. A new writer is a new allow-list line, added after confirming its
repo argument is the repo the write lands in. Both guards carry a control test that plants
one instance of each shape and must see it.

**When the roster carries no neutral name** for the blessing authority, a public-target
write **refuses** (exit 5) naming the `ASSAY_HUMAN_LOGIN_MAP` entry to add. The two
alternatives were both rejected: stamping the login is the disclosure this split exists to
prevent, and writing with no trailer at all would retire the presence guarantee every
check downstream is built on. The refusal text does not quote the login it is declining to
disclose.

**The witness annotation follows the same split.** The `statusgen` witness cell (below)
lands in a brief's `## Evidence` table, a file in the repository, so it names the neutral
form too on any repo the roster does not state is `:private`. Its target is the `repo:`
frontmatter of the brief's own stream README; a stream that states no repo takes the
neutral form (fail-closed, the same direction as above). A git remote never selects the
form, because a worktree's origin can name the checkout it was cut from rather than the repo
the brief lands in. It can only veto the login form. Suppose the README states a `:private`
repo but the checkout's origin resolves to a repo the roster does not state is `:private`,
such as a stream moved between repos with its frontmatter left behind. The witness then
takes the neutral form. An unreadable remote leaves the README's answer as it is. With no
neutral name configured the witness writes no annotation rather than the login
(statusgen never blocks a witness write; the lint then reports the unannotated row).

## Resolution

Implemented once, in `tools/desk/internal/deskkit/principal.go` (`ResolvePrincipal`), and
mirrored read-only in `statusgen/principal.go` for the witness/lint side (statusgen and
the desk tools are separate Go modules that deliberately share no code — the
documented-duplicate pattern `rosterconfig.go` already uses).

**Order:** the calling session's own roster beacon (`deskroster`'s
`<StateDir>/roster/<session>.json`) decides ATTENDED vs UNATTENDED; `ASSAY_BLESS_LOGIN`,
read *exclusively* through the roster's config-home file (never an environment variable —
see `rosterconfig.go`'s `ClassWrite` rule, which this resolver reuses rather than opening
a second route), supplies the login itself.

**An environment variable is never consulted for the login.** A session could otherwise
set its own principal by exporting one — exactly the self-report failure mode the
roster's file-only-for-write-tools rule already closes for every other trust decision.
`DESK_SESSION` / `CLAUDE_SESSION_ID` name *which* beacon file to open; they never supply
the login.

**Three states** (`PrincipalState` in `principal.go`):

| State | Meaning |
|---|---|
| `PrincipalUnresolved` (zero value) | No roster is configured — or the target needs the neutral form and the roster states none. Every write verb **refuses** (exit 5) rather than write without a principal. |
| `PrincipalAttended` | Resolved to the bless login; the calling session has a live roster beacon. |
| `PrincipalUnattended` | Resolved to the **same** bless login; no live session beacon was found. |

**Why both resolved states name the same login today.** Per the 2026-09-12 driver ruling
this brief relays: until a second human joins the roster, the
principal resolves to the roster's single blessing authority (`ASSAY_BLESS_LOGIN`)
regardless of who or what is driving the session. The session-beacon read exists so that
when a second human's attach-time handshake lands (deferred to roster v2), attended
sessions already have the plumbing to resolve to the session's own human rather than the
bless login — this landing's job is only to tell attended and unattended runs apart, not
yet to pick between two humans.

## The writers

Six verbs stamp the trailer, or refuse (exit 5) rather than write without one:

| Verb | Where it lands |
|---|---|
| `deskpost comment` / `review` / `security-review` | a trailing line on the posted comment/review body |
| `deskreply` (plain and `--workpad`) | a trailing line on the posted/edited comment body |
| `deskpr create` / `edit` | a trailing line on the PR body |
| `deskfile new` / `attach` | a trailing line on the issue body / comment |
| `deskevidence` | a trailing git-trailer line on the commit message (both the direct-write and the side-branch/draft-change lanes), and the Runner-cell annotation on each App-run Evidence row the landing adds without one (see below) |
| `deskflip` | see below — this verb has no body to stamp |

**Idempotency is unaffected.** The trailer is appended to the body/message sent to the
forge, never to the bytes an idempotency/dedup key is computed from. Two sessions posting
byte-identical semantic content — one attended, one not, or on different days — still
dedupe: the trailer text (which can legitimately differ between the two calls) never
enters the comparison. `deskpr edit`'s noop compare strips a live body's prior trailer
(`deskkit.StripOnBehalfOfSuffix`) before comparing against the caller's replacement, for
the same reason.

**`deskflip` is the deliberate exception.** Its one mutation
(`deskkit.ReadyFlip` — a draft→ready GraphQL/REST transition) carries no body or commit
message at all; there is no text surface to stamp a trailer onto. Its write is: resolve
the principal as a *precondition* of the mutation (refusing exit 5 rather than flip
without one), and name it in the printed success/dry-run line, so the record of who a
flip was on behalf of survives in the one place every other `deskflip` decision does —
stdout, which the desk's audit capture already retains.

`--dry-run` on every verb runs every other check, resolves the principal (refusing if
unresolvable), and prints the trailer it would have written before stopping. **This
changes the dry-run contract**: a dry run can no longer be used to check a write verb's
*other* preconditions in an environment with no roster configured — `deskflip --dry-run`
in particular now exits 5 on an unresolvable principal even when every other precondition
it checks would have passed. That is deliberate (the principal is a real precondition of
the mutation, not a preview-only concern), but it is a behavior change worth knowing about
before scripting against `--dry-run` in an unconfigured environment.

## The witness annotation

`statusgen verifyrun`'s witness `row()` renders the on-behalf-of suffix into the Runner
cell whenever the base runner names a shared App/bot identity (the `<slug>[bot]`
convention `executingRunner` itself already uses for a CI-actor runner) — never for a
`human:<name>` runner, which already names the acting human directly. Unlike the write
verbs above, this annotation is **best-effort**: if no principal resolves, the cell is
left unannotated rather than refusing the whole witness write. The write-side refusal
(the six verbs, above) is the actual enforcement point; the witness cell is a
*record* of what already landed.

**`deskevidence` stamps the cell at ingest.** A row that reaches `deskevidence` without
the annotation (one transcribed or assembled from several witnesses rather than written by
`verifyrun` directly) is stamped by the tool, not by hand: for every `## Evidence` data row
the landing ADDS whose Runner cell names an App identity and carries no `on-behalf-of`
text, it inserts ` (on-behalf-of human:<who>)` before a trailing `(forge-identity)` /
`(ci-env)` / `(git-config)` tag, or at the end of the cell when there is none — the order
`verifyrun` renders. `<who>` comes from the same roster resolution as the commit trailer
(the neutral name on any target not stated `:private`, the login on one that is), never
from a flag, an environment variable or the landed content; when it cannot be resolved
the landing refuses (exit 5) and nothing is written. A row already on the branch and a
`human:<name>` row are left byte-identical.

An added App cell that already carries `on-behalf-of` text is read the way the lint reads
it (the last `on-behalf-of human:<p>`, case-insensitive) and checked against `<who>` and the
same roster's human set:

- exactly one annotation, written exactly `(on-behalf-of human:<who>)` byte for byte, is
  left byte-identical;
- an annotation not in the `human:` form (a desk role, a bare word), one naming no
  recognised roster human, more than one annotation, the same human in the other form
  (the login on a public target, the neutral name on a private one), or `<who>` in any other
  spelling (other case, extra words after the name) has every `on-behalf-of` fragment
  stripped and `<who>` stamped — the trailer's strip-then-append rule, so a planted
  annotation never survives and a login never stays on a public target. An unclosed
  fragment (`(on-behalf-of desk` with no `)`) ends before the next `(`, so a source tag after
  it is kept;
- any `human:` annotation naming a *different* recognised roster human refuses the landing
  (exit 5), naming the row; nothing is written. This is deliberately stricter than the lint,
  which accepts any recognised human: one human's attribution is never overwritten with
  another's automatically;
- a Runner cell the tool cannot read the way the lint reads it refuses the landing (exit 5),
  naming the row and the reason; nothing is written. That is an HTML comment inside the
  cell, or one crossing a line break on the row's line (the lint removes comments, which
  joins the lines around one into a single row); invalid UTF-8; an invisible format
  character such as a zero-width space; a character whose lowercase form has a different
  byte length (the lint matches in a lowercased copy and slices the original at the same
  offsets); or any other non-ASCII character, such as a combining mark or a homoglyph in the
  marker, which renders an annotation the lint does not read. An App Runner cell is an App
  login, a commit and ASCII tags, so nothing legitimate is refused. Any line the landing adds
  to the body's Evidence section is refused the same way when it holds a control character
  other than a tab (C0, DEL or C1: a lone carriage return or a next-line character, say). That
  covers every kind of line, App row or not: a data row, a header, a separator, or a line that
  is not a table row at all. The lint splits lines on line feeds only, so a renderer could show
  a row the lint never reads. A line is skipped only when the base's own Evidence section
  already holds that exact line; a line copied in from elsewhere in the base is checked. The
  check covers control characters only, not other characters a renderer may break a line on
  (U+2028, U+2029). As a last check, the landed section is read the way the lint reads it,
  and every added App row must carry exactly one annotation, in the exact written form,
  naming `<who>`. A row counts as already on the branch there only when the base has the same
  line with the same Runner cell, so a base line re-landed under a header that moves the
  Runner column is checked like an added row. The heading line that ends the section is
  checked for control characters too, when the landing adds it;
- the landed Evidence section must be one both readings agree on: what a Markdown (GFM)
  renderer shows as Evidence rows is what the lint reads. A landing that adds anything else
  refuses (exit 5), naming the line and the reason; nothing is written. The checks look at
  what the landing adds (and at the table or heading it adds lines under), so a brief that
  already carries an older shape is not blocked from landings that leave it alone. The tool
  carries no Markdown parser; it holds the rules below, which are narrower than "every
  shape a renderer could show", and the lint-way read of the landed section remains the
  last check.
  - **Lines in the section.** Every added line is a blank line, a `|`-led table row, or a
    note line. A table is a `|`-led header, then a delimiter row a renderer accepts (cells of
    dashes with optional edge colons, one per header cell), then `|`-led rows. In the middle
    of a table, an added line the lint reads as a separator, or as a new header (a `|`-led
    line with a separator line under it), is refused unless it repeats the table's own header
    exactly; so is an added row read under such a header. A `|`-led line under no header is
    read by the lint with its last cell as Runner, so it lands only when no other cell shows
    as an App-run Runner cell; a separator line under no header is refused.
  - **Rows.** Only the cell the lint reads as Runner may show as an App-run Runner cell. An
    added row wider than its header is refused when the header has no exact `Runner` cell
    (the lint then reads the row's last cell, which a renderer drops).
  - **Lines without a leading `|`.** Inside a table, a line holding a `|` that does not begin
    with one renders as a row the lint never reads, and is refused. A pipe-less line that
    closes a table renders as a one-cell row: it lands when it shows no `[bot]` or
    on-behalf-of text and no word in it (at its start or after a space, a tab or a `(`) shows a
    login and tag (below). A line after a blank
    line or a heading starts no table and is not read for that text. After it the lint has dropped the table's header, so a `|`-led row
    added later in that table is refused, unless a repeat of the table's own header (with its
    separator line) comes first, or the row has exactly as many cells as the table's header
    and that header's Runner column is its last cell (or it has none): both readings then take
    the row's last cell as its Runner cell.
    A table with a different header needs a blank line above it. The `--brief-path` merge
    puts one there whenever the appended block opens with a header, so a witness table
    appended after an attestation line lands as its own table.
  - **Markup.** An added line may not hold a raw HTML table or heading tag, share its line
    with an HTML comment that crosses a line break, or be a table line inside a block quote.
    A Runner cell the lint does not read as App-run is refused when it shows as one once HTML
    tags and comment markers are removed, entities decoded, and emphasis, strikethrough,
    code-span and escape characters (`*`, `_`, `~`, a backtick, `\`) stripped; that covers
    `<span>…[bot]</span>`, `**…[bot]**`, `_…[bot]_` and a login in a code span. Link, image
    and autolink syntax is read as the text a renderer shows for it (a link's text, an image's
    alt text both shown and not shown, an autolink's address), and a closed HTML comment both
    hidden and shown; a bracketed `[text]` that opens no link is also read as a shortcut
    reference link (its text shown, since a definition elsewhere in the brief can resolve it).
    A cell or line shows App-run text when any of these readings does, and every check of what
    a cell, line or heading shows uses them. A reader also takes a cell that opens with a login
    and a tag as App-run, whatever follows the tag and whatever characters the login holds, so
    the tool reads that by structure rather than by folding characters it knows. Each reading
    is also read with every character that renders as nothing dropped (a nonspacing or
    enclosing mark, a default-ignorable code point such as a filler letter, the blank braille
    pattern, and a control, format, private-use, noncharacter or unassigned code point, with a
    numeric reference to a C1 control read both as a browser maps it and as the control
    itself), and with every word that leads it and holds no ASCII letter or digit skipped
    (a word ends at white space), since such a word cannot be a login and a reader takes the
    word after it for the name, whatever its characters are and whether or not they render.
    That reading is taken once more with its leading words read as a renderer shows them: a
    word a renderer replaces with a glyph (`:name:`) is read as one symbol with no ASCII letter
    or digit. Each leading word still ends at white space and is read by itself, and the rest
    of the text stays as written, so a glyph word or a clock time later in a cell can add a
    reading but never removes one. A text holding an HTML tag with a quoted attribute value is
    also read with every such value emptied, beside the text as written, since a `>` inside
    quotes ends no tag. When a text holds two or more runs of `$` a renderer can take as math
    delimiters (an escaped dollar sign, one written as an entity and one inside a code span are
    not, since a renderer shows each as text, and the runs are found before entities are
    decoded), it is read beside the text as written, always with the text before the first run
    as written: once for each run, with everything from the first run up to and including it
    dropped, as when the spans there show only a symbol or blank space; once with every run
    dropped and the text between them read as math; and once for each two adjacent runs and
    each place in the text between them where a control word, a brace or white space ends, with
    everything from the first run up to that place dropped, the text from there to the second
    run read as math and the text after it as written. Where code spans stand is read two ways,
    each giving its own set of runs: by a loose match that pairs backtick runs of any lengths,
    and as CommonMark reads them (a backtick run no backslash escapes opens a code span, which
    only the next run of the same length closes). Beside both, the runs are also paired from
    the left, inside code spans too, for a renderer that typesets math there, each span read
    from each place a control word, a brace or white space ends. Text read as math has its
    markup (control words, braces, backticks and sub- or superscript signs) removed, a control
    word for a bracket, brace, bar or escaped sign read as that character, a character code
    (`\unicode` with optional arguments and a braced decimal or hex code, `\char` with a hex,
    octal, decimal or backtick code, `\mathchar` by the low byte of its code) read as the
    character it names, and is read both with its white space and with it dropped, as math
    typesets the symbols in it without the white space between them. It is read once more for
    each control word or symbol the tool does not name, read alone as an opening and alone as a
    closing bracket, and for each two of them, in order, read as an opening and a closing
    bracket, since a renderer can typeset any of them as one; a word read as a bracket is read
    both with the argument after it (optional arguments, then a brace group or a number) shown
    and with that argument taken into the bracket, since a renderer can typeset a word and its
    argument as one character. A text holding a character code is read all of those ways once
    more with each code's control word read as one the tool does not name, for a renderer that
    does not typeset the code, so reading codes adds readings and removes none. An escaped bar
    (`\|`, as a table cell writes a shell pipe) is read as a bar, never as a bracket. No reading
    depends on which runs before a span a renderer pairs. A
    later backtick run can still close a code span an earlier one opens, and so take the runs
    of `$` inside it out of the CommonMark reading, as it takes them out of what a renderer
    typesets; the loose and left-paired readings keep them. The text after each raw line break
    or block element tag (a paragraph, list, table or disclosure element and the like) is also
    read as a text of its own, since a renderer starts a new line there. Every reading is taken
    both with and without the text of a raw HTML element a sanitizer can drop with its text
    (`<script>`, `<style>` and the like), and apart from that, both with and without the text
    of the elements a renderer hides by default (a ruby fallback `<rp>`, a `<datalist>` or
    `<dialog>`, or any element with a `hidden` attribute, which may follow white space, a slash
    or a closing quote). Those are read dropped several ways: once with every ruby fallback,
    data list or dialog dropped to the next closing tag of its own kind and every element whose
    `hidden` attribute follows white space dropped to the first closing tag after it; and, with
    the hidden elements taken in order, once without the first, once without the first two and
    so on to all of them, each dropped both to the first closing tag after it and to the
    closing tag that matches it, counting the elements of the same name opened inside it, since
    a renderer can hide one element and show the next. These readings are rules the tool holds,
    not a renderer's own behaviour; apart from the code-span case above each only adds a
    reading, so they can refuse a cell a renderer shows plainly, and a renderer can pair,
    typeset or hide text in a way none of them names. A text reached by more than one reading
    is read once. The number of readings, each about as long as the text, grows with the line
    breaks and hidden elements a text holds, times the places between two runs of `$` a span's
    text can start at, times the control words and the pairs of control words in a span (up to
    four readings for a pair whose words both take an argument, and twice that when the span
    holds a character code), so a short text can ask for far more work than its length. The
    readings of one cell or line may read at most 128 MiB of text in all, each hidden-element
    reading charged before the next is built and each hidden element's end found by reading
    only as far as that end; past that cap the text is taken as showing
    App-run text and refused, never read with fewer readings. The heaviest text in the briefs the
    cap was checked against reads about 5.3 MiB.
    Every trim takes any Unicode white space, a non-breaking or wide space included.
    A cell's text so starts at the first word a reader can take for a name. A cell's
    identity region is its text up to the first ASCII space or `(`, where the lint cuts it,
    extended over spaces inside a tag that closes before any `(` or other tag bracket. A tag
    is an ASCII square or angle bracket, or a non-ASCII opening or closing bracket or quotation
    mark, opened directly after a letter, digit, `.`, `_`, `-` or non-ASCII character, and
    closed later in the region. A Runner cell the lint does not read as App-run is refused when
    its region holds a tag of exactly three letters or digits, as `[bot]` does, or a tag of
    three or more letters, digits or symbols while any character in the region is outside the
    ASCII login alphabet (letters, digits, `.`, `_`, `-`, `[`, `]`). A character in the region
    that no rule names therefore fails closed: `name[bot]\u00a0@ …`, `name[bo\u200bt]`,
    `name[bot]:`, `[name\[bot\]]` and an ASCII-only `name[abc]` are refused. A region whose
    letters are all
    CJK text (`東京【東京都】`) cannot spell a login and lands, as do `[bot]` later in a cell
    (`human:operator (via name[bot] review)`), a model tag (`[1m]`), brackets that open no tag
    (`[21:24Z][ok-all]`), a tag of only login characters (`release[draft]`) and a human
    name with a non-ASCII letter land. An App Runner cell holding
    `<`, `>`, `&`, `~` or `\`, or link, image or autolink syntax, is refused as unreadable
    (above); brackets that open no link, such as a model tag, are left alone.
  - **Headers.** A header the landing adds, or adds rows under, is refused when a reader can
    take it as naming a different Runner column than the lint reads: an invisible format
    character such as a bidi isolate, a non-ASCII letter or mark, a cell whose letters spell
    `Runner` but that the lint does not read as exactly `runner` (`**Runner**`), or more than
    one `Runner` column (the lint reads the last).
  - **Headings.** The lint reads only the first `## Evidence` section. A heading a renderer
    shows as Evidence anywhere else (ATX, setext or HTML, at any level or case, with entities
    decoded, tags removed and link syntax read as its text, or put on its own rendered line by
    a lone carriage return) may
    not be added, and no line may be added under one that is already on the branch. An added
    heading may not hold a non-ASCII letter or mark. Headings inside fenced or indented code
    are code, not headings, and are not counted.
  - **Outside the section.** An added line outside the section the lint reads (or anywhere
    in the body, when there is no such section) may not show an App-run Runner cell in a
    `|`-separated row, or hold a raw HTML table or heading tag outside a code span, with code
    spans read both by the loose match and as CommonMark reads them. Fenced
    code is left alone.

Like the lint, the tool looks for `## Evidence` only in the brief's body, after the
frontmatter block: a `## Evidence` line inside the frontmatter is never taken as the section.
The `--brief-path` merge places the new rows with the same rule: it takes the first body line
that reads `## Evidence` once surrounding spaces are trimmed, so a heading with a trailing
space is that section, not a reason to append a second one. A landing whose frontmatter opens but never closes has no body the lint can read, so if it
carries an `## Evidence` heading anywhere it is refused (exit 5) and nothing is written.

The stamp is always one `(on-behalf-of human:<who>)` parenthetical in the Runner cell. A
restamp writes it where the last stripped fragment stood, with the whitespace byte that
fragment carried (none, if it was glued to the text before it), and a strip removes each
old fragment with that one byte, so the rest of the row is byte-identical. Fragments sitting
back to back form one run, and the stamp takes the separator the run's first fragment carried.
A fresh stamp goes before a trailing source tag (matched in any case), ahead of any spaces or
tab between the text and the tag (a tag glued to the text gets the stamp glued too), or at
the end of the cell. The written stamp is exactly `(on-behalf-of human:<who>)` and nothing
broader, the one form a container witness bracket's digest drops from a Runner cell, so
landing it or restamping one exact-form annotation to another never moves that digest. A
non-exact annotation (a desk role, an empty principal) was inside the hash, so restamping it
does move the digest and that bracket fails closed.

So a verifier leaves Runner cells unannotated and pre-checks with `deskevidence … --dry-run`,
which runs every refusing gate on the stamped content and prints each row it stamped; a
lint of the verifier's own home reads App rows before the stamp, so it is not the pre-check.

## The lint

`statusgen --lint` (`principalAttributionProblems`, `statusgen/attribution.go`) reads
every brief's `## Evidence` section and checks two DISTINCT shapes, which are gated
differently — read both before assuming either governs the other:

- **Missing annotation.** An Evidence row run by an App/bot identity with **no**
  on-behalf-of annotation at all. This check is **cutover-gated, not roster-gated**: a
  row dated at or after `principalAttributionCutoverDate` (the date this write path
  landed) is a hard **PROBLEM** — the write path could have stamped it and did not. A
  row dated *before* the cutover is a **NOTICE** — no write path existed yet to stamp
  it, so it is grandfathered rather than condemned. This check runs **regardless of
  whether a roster is configured**: an unconfigured repo still gets the
  missing-annotation signal (as PROBLEM or NOTICE per the row's own date), because
  detecting the *absence* of an annotation needs no human map to check against.
- **Unrecognised annotation.** An Evidence row whose on-behalf-of annotation names a
  principal the roster does not recognise: neither a login in its human set (the bless
  login's own entry and the trusted humans) nor a neutral name `ASSAY_HUMAN_LOGIN_MAP` maps
  onto such a login. Both forms are accepted because the writers stamp either, by target
  visibility; a name is accepted only through the map, never as a free token. The
  corroboration lanes strip the on-behalf-of marker before looking for sign-offs under the
  same rule (a configured login or a configured name, whole and login-shaped). This check IS roster-gated: with **no roster configured at all**,
  it has no human map to validate against and says nothing (a repo that has not adopted
  the roster gets no signal from a check it cannot answer, rather than a manufactured
  problem). It carries no cutover — an annotation that exists at all was written by a
  stamping path, so the retroactive-history problem the cutover solves does not apply
  here.

A human-run row (`human:<name>`) is never checked by either shape — it already names its
principal directly.

## A relay is never a human's authority

The corroboration lanes strip the on-behalf-of marker, so a relay needs nothing on the PR
to corroborate. The offline readers of a human-authority key therefore remove the whole
relay, marker and principal together, before they look for a `human:` token. Those keys are
a finding's `authorized-by:`, its `parked-by:` (both the register gate and the alarm
layer's shelving check), a deploy record's `authority:` and `rollback-approver:`, and a
decision record's `decided-by:` (the design-approval authority). A value such as
`authorized-by: on-behalf-of human:<name>` authorizes nothing, whichever form of the human
it names. Without this removal, an App could write that line and pass both lanes without
the human ever acting. A `human:<name>` written outside the relay authorizes offline and
is gated online, the same as before.

A decision record's online lane re-reads `decided-by:` from the record on disk for every
record a PR adds or edits. That lane covers the same file set the register lint and the
design gate read: every `.md` under `docs/streams/decisions/` except the README, whatever
its file name. The design gate resolves a brief's `design:` by the record's frontmatter
`id:`, so a record whose file name is not `DR-<slug>.md` is still one the gate accepts,
and the online lane must see it.

A README's Verified/Reviewed cells keep their existing reader. The online stamp lane
strips a relay there too, so it is not what catches one. The branch lint's human-stamp
gain guard is: it reads the raw cell and refuses any `human:<x>` a branch adds, relay
included, so a relay in a cell still needs the human's own act.

The tests are in `statusgen/principal_authority_test.go`.
`TestOnlineExemptLineNeverAuthorizesOffline` pins the invariant that joins the two
lanes: an added line the online lane records no stamp for never grants authority offline.
It covers `decided-by:` in a record named `DR-<slug>.md` and in one that is not.
`TestReadmeCellRelayCaughtByGainGuard` pins the README-cell dependency on the gain guard.

## What this does *not* prove

The witness file comment already states the residual for `executingRunner`, and it
applies here without modification: **the environment is not a cryptographic anchor.**
Whoever controls the process controls the roster's config-home file and the session
beacon that decide attended/unattended. The on-behalf-of trailer is evidence for a
reviewer reading a diff — a composite-identity record analogous to GitLab's, and the
precondition the later author-never-flipper check, the decision packet, and the
segregation-of-duties lint (multi-principal/03, /04, /09) build on — not an unforgeable
attestation. Its real strength, like the witness record it rides alongside, is that it
lands next to the tree SHA and commit it names, where a second person (or a later,
stricter check) can corroborate it.
