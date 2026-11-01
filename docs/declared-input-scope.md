# Declared verification input scope

Issue #2798 closes a shared parsing/expansion defect: declarations silently became
an empty or partial wake input scope. A single error-returning parser and tree
resolver now own the writer, coverage and Source-to-Evidence shape contract.

## Declaration grammar

Only an exact, column-zero `files:` inside one unfenced `## Context` declares inputs.
A field-shaped `files:` marker outside that section is refused. Ordinary prose
starting with “Files” and inline code references to `files:` remain prose outside
Context. Inside Context, wrapped prose starting with “files” or an inline
`file:line` reference still refuses conservatively as an unsupported marker;
these existing refusals are retained rather than guessing declaration intent. Inside
Context, unsupported capitalization or list, quote, backtick and emphasis prefixes
on a files marker are refused rather than treated as an absent declaration. A field can start
inline or continue as list items. Bare and backtick-quoted repository paths are
equivalent. Commas, semicolons, `+` and `and` separate declarations. Optional bold
action labels, parenthesized annotations and the existing `and tests`/`their tests`
annotations are prose; name actual test paths when those tests belong to scope.
An em-dash description, or a colon after a quoted path, ends the declaration part
of that line. Backticks inside that description, facts, other sections and fenced
examples do not add files. Structural headings or files markers inside a region
the parser would skip as fenced refuse conservatively, even in a genuine code
example; the parser does not infer list or HTML container boundaries. This includes
shell `## build` comments, YAML `files:` keys and Go `files := list()` lines
inside code examples; those structural-looking examples also refuse. A blank line, another Context field or section ends the
field. A section boundary ending Context MUST be at column zero; an indented
`## `-shaped line inside Context refuses, including while a files list is paused
after a blank. It cannot silently discard later list items.
HTML comments within a declaration (including its description) and raw HTML
blocks or comments while awaiting the next field also refuse: a hidden heading
cannot be trusted to end Context. Known metadata fields still end the scope.
After the blank, list markers (`-`, `*`, `+`, `N.`, `N)` followed by
whitespace), backtick-led lines, quote-led lines and indented continuations are
refused as ambiguous until a known Context field begins. A following field key
must be at column zero; trailing whitespace and CRLF do not add indentation.
Known boundary keys are `facts`, `layering`, `design-fit`, `single-point-of-failure`,
`risk-answers`, `layout` and the separate `out-of-repo files` metadata field. They
may carry a parenthesized annotation before the colon, such as `facts (date):`.
An unknown key while the declaration is open or awaiting its next field refuses;
it cannot silently discard later files. A quoted path ending in a
colon remains a declaration, while indented prose is an explicit unsupported-form
error. Hyphen-minus and en-dash separators are unsupported; only the documented
em dash introduces description text. Duplicate Context sections or files fields
are refused explicitly. Fence openers and closers allow at most three leading
ASCII spaces. A backtick opener's info string cannot contain a backtick. Inside
Context, every indented fence opener and every fence opener within a files
field or while awaiting its next field after a blank refuses. A fence-shaped
line with an invalid backtick info string also refuses there; outside Context,
invalid openers remain prose. These containment refusals deliberately narrow
the supported Markdown subset instead of approximating list/HTML block ends.
Fences close only with the
same character and at least the opener's length; an unclosed fence refuses the scope. External metadata is
not permission to read outside the admitted repository.

Supported path forms:

- Literal files and directories; a directory includes every nested file. A trailing
  slash is normalized. That normalisation applies to declarations only: the
  `scope:` input key re-adds the slash when the literal is observed as a directory
  (see the reader contract below). A literal not yet present has an absence scope
  digest and no content hash; that is distinct from an unreadable tree.
- Segment globs interpreted by Go `path.Match`: `*`, `?` and character classes.
  Matched directories include their descendants. A glob with no match has an empty
  membership digest. Patterns use slash separators on every host.
- Finite brace alternatives, such as `cmd/{one,two}/main.go`, expanded before tree
  resolution. At most 256 expansions per declaration; nested/empty alternatives
  are refused. This supports the existing desk-tools/37 and /38 declarations.

Placeholders (`<slug>`), ellipses (`...`, `…`), recursive `**`, malformed glob/brace
syntax, noncanonical paths, absolute/drive paths, traversal, backslashes, controls
and whitespace within paths are explicit refusals. There is no shell expansion,
abbreviation guessing or out-of-repository lookup. One path is at most 4096 bytes;
canonical declarations and selected files are bounded at 4096. Each resolution
also permits at most 32768 work units, counting declarations, listed members,
ancestor candidates, candidate matches and path-kind reads across all declarations.
This bounds overlapping work even when selected files deduplicate. Exceeding a bound
is reported, never truncated to a partial successful scope. Other historical prose
forms need an explicit declaration repair and fresh observation.

## Observation and reader contract

`BriefDeclaredFiles` returns declarations or an error. `ResolveDeclaredScope`
returns the exact existing files and one `scope:` digest per canonical declaration.
Membership digests hash sorted, NUL-separated kind/path terms. Globs include
matched directory membership; content hashes are separate `file:` keys (one per file,
or one `file:<dir>/` key per observed directory; see below). Successful
absence, empty membership, unsupported syntax and unreadable state remain distinct.

`BuildOutcomeRecord` reads declarations and deliverable bytes from the admitted
observation tree. Coverage requires every selected existing file. The admission
binding independently compares the exact derived input set, preserves the supplied
tool version, and refuses declaration/kind/membership changes before Evidence
landing. The landed brief is the only file hashed from landed Evidence. A content
repair can wake the original failure; a new membership shape requires fresh
independent verification rather than relabelling the old observation.

The root reader checks required scope-key coverage before claiming unchangedness.
Historical partial receipts therefore remain intact and become could-not-check;
they do not acquire new hashes from the current branch. Known observed changes
still fire. No declared files is a legitimate empty declaration set; an unsupported
declaration is an error. Generic/custom input readers retain opaque semantics
unless they implement `CheckReceiptScope`.

Scope keys have two forms, decided by the observed path kind and never by the
declaration, which is always canonical without a trailing slash:

- `scope:<decl>/` for a literal declaration observed as a directory;
- `scope:<decl>` for a literal file, an absent literal and every glob, and for every
  directory key written before the slash form existed.

The digest is the same in both forms. Readers accept both: the completeness check
counts a declaration as covered when exactly one of the two forms is present with a
non-empty value (for a glob only `scope:<decl>` counts), and the root reader parses
the key back to its declaration and returns the one digest that resolution
produces, whichever form the key is in. A directory that becomes a file, or a file
that becomes a directory, therefore changes the digest and wakes; it never reads as
missing. "Ambiguous" means both forms of one declaration are present in a receipt;
that is refused and reads as could-not-check. A key with an empty declaration, a
double slash, a non-canonical declaration or a slash after a glob (never emitted)
is unreadable, so could-not-check. Every `scope:` value, in either form, must be
exactly the producer's digest format (64 lowercase hex characters); any other value
is refused, so it never reads as unchanged: when the declaration still resolves, the
recomputed digest differs from that value and the observed change outranks the
refusal, so the held work wakes; otherwise the read is could-not-check.

Measured residual of the slash form (the secret pattern sweep reads every landed
record). At a slash-keyed position the sweep's generic pattern rule matches no
value, so it no longer inspects that value; dedicated provider rules still match
there. Readers refuse to read any non-digest value, but that refusal happens after
publication, not before: it is not a pre-publication control. Keys that keep the
no-slash form can still trip the generic rule when they end in a credential
keyword: an extensionless file literal, an absent literal, and a glob whose last
segment ends in such a keyword. The landed corpus held none of these when the slash
form was introduced; they are recorded here, not fixed.

### Directory content keys

A directory observed as a tree (a literal directory declaration, or a directory a
glob matches) contributes ONE content key, `file:<dir>/`, instead of one
`file:<member>` key per file under it. Its value is the sha256 of the sorted,
NUL-joined `<member-path>\0<member-sha256>` terms over every file the directory
holds at the observed source, each member hashed exactly as a `file:` key would
hash it (a final symlink hashes its link text). An edit, addition, removal or rename
of any member therefore changes the value and wakes the held work. The `scope:`
key for the same declaration is unchanged: it still records path kinds and
membership only, and the Source-to-Evidence shape check still compares it.

Coverage counts a file as covered by its own `file:<path>` key or by the
`file:<dir>/` key of a declared directory that contains it. A key for one member
never covers its directory, and a `file:<dir>/` key for an undeclared directory
covers nothing. A `file:<dir>/` value must be exactly the producer's digest format
(64 lowercase hex characters); any other value is refused, so it never reads as
unchanged. When the directory still reads, its recomputed digest differs from such
a value and the observed change outranks the refusal, so the held work wakes;
otherwise the read is could-not-check. The root reader recomputes the value from the directory's current
members; a path that is no longer a directory, or an unreadable member, is
could-not-check.

One exception keeps per-member keys: a directory that holds the brief itself. The
brief's own `file:` key is hashed as its Evidence lands, which a directory digest
could not express, so that directory's files stay individually keyed.

This form exists because the secret pattern sweep reads every landed record. A
per-member key such as `file:<dir>/secret-mount.fixture` puts a credential keyword
immediately before a 64-hex value, which the generic pattern rule matches when the
file extension is longer than its allowlist's extension bound. At a slash-terminated
key the generic rule matches no value, so it no longer inspects that value; dedicated
provider rules still match there, and the writer computes the value itself. As for
the slash scope form, the reader's digest-format refusal happens after publication,
not before. The scanner configuration and its allowlists are unchanged. Residual: a literal file declaration, or a file a glob matches
directly, keeps its own `file:<path>` key, so a file whose name ends in a
credential keyword with a long extension can still trip the generic rule there, as
can a member of a directory that holds the brief.

Migration: the key is additive, so `wake_schema` stays `verify-wake-v1`. Records
written before this form keep their per-member keys, are never rewritten, and are
read exactly as before. A reader older than this form cannot resolve a
`file:<dir>/` key and reports could-not-check, never unchanged. A pending
(not yet landed) record whose inputs were derived before this form, and which
therefore carries per-member keys for a directory this form folds, no longer equals
the writer's re-derivation. The verify loop refuses that pending Evidence with an
error, so the item is could-not-check and is not dispatched: it is neither held as
awaiting merge nor re-verified. It stays could-not-check until the pending Evidence
lands (the landed record then reads through its per-member keys as before) or its
record is re-issued with the current writer. Landed records are unaffected.

Local enumeration never follows ancestor directory symlinks. A final symlink is a
Git-style blob whose content hash is its link text. Read/enumeration errors are
could-not-check. Local extra files can conservatively wake a directory/glob scope;
the reader does not infer that untracked files were in the admitted Git snapshot.

## Consumer inventory

| Consumer | Disposition |
|---|---|
| deskkit receipt derivation | Shared parser, expansion, file hashes, `file:<dir>/` directory digests and membership digests |
| deskevidence coverage | Same expansion, every selected file required (its own key or its declared directory's `file:<dir>/` key) |
| deskevidence observation/shape binding | Exact original Source inputs; fresh admission on shape changes |
| verifyloop failed/blocked scheduling | Shared `Unchanged` and `EvaluateWake`; root completeness plus scope and `file:<dir>/` revisions; pending records re-derive with the same writer |
| statusgen outcome-register reader | Reads scheduling history; does not evaluate wake input revisions |

The Go engineering catalogue selects no generic Markdown parser for this semantic
field. This reuses the existing deskkit owner and standard-library `path.Match`,
filesystem and Git-tree adapters; it adds no dependency, CLI or second writer.
The small field grammar is intentionally not a general Markdown implementation.

## Corpus observations

Inventory results below describe declarations at the named immutable source, not
a new Verify result or a migration of historical Evidence/outcomes. Source manifests
of an entire execution snapshot are provenance, not a declaration that every tracked
file belongs to the receipt scope.

The independently observed desk-tools/37 and /38 failure proposals cover 12 and 44
implementation files respectively, plus their brief and tool. The shared resolver
retains those exact file sets. Their real failed acceptance rows remain owed;
this parser fix grants no lifecycle promotion and rewrites none of those receipts.


Against the 485 tracked `docs/streams/**/brief-*.md` corpus files from
`9019807975730ac678760b0be3ac6a626dceac7a`, this repair produces 183 supported
scopes and 302 explicit refusals, with no successful empty scopes. Every accepted
scope resolves successfully. Comparing with the parser at `fbba4b315c1e` over the
same files: 36 scopes recover, no previously supported scope begins refusing, and
no retained successful declaration list changes. The earlier 147/338 count
included valid annotated fields and prose incorrectly classified as authoring
defects; that interpretation is withdrawn. These are first diagnostics per brief,
not counts of every bad token or historical verification results. The in-module
regression excerpts retain the five genuine declaration-position refusals.

| Named example | Canonical declarations | Existing implementation files |
|---|---:|---:|
| cellctl-windows/00 (bare paths) | 4 | 6 |
| forge-neutral/36 (inline and continued paths) | 20 | 52 |
| fresh-views/02 (globs and descriptions) | 3 | 59 |
| desk-tools/37 (finite braces and directories) | 9 | 12 |
| desk-tools/38 (finite braces and directories) | 40 | 44 |

The 12/44 sets were compared exactly with the independently observed failure
proposals, excluding each proposal's brief/tool keys; equality held. This checks
scope preservation, not the historical execution result. Both existing failed
verdicts and their original admission/source bindings remain authoritative.

Representative unsupported authoring is explicit: `changelog/<branch-slug>.md`
(forge-neutral briefs), `tools/desk/**` recursive patterns, and abbreviated prose
such as `files (created/extended by this phase):` in statusgen/05. Preserve existing
Evidence while replacing such declarations with exact paths/segment globs for a
fresh admitted observation. The files fields under Task in desk-tools/31, /33 and
/36 now produce explicit out-of-Context refusals. The wrapped prose in derived-board/06
and composability/04 also refuses explicitly; their earlier partial parses never
established complete scope. These briefs need declaration repair and fresh admitted
observation; this inventory grants no historical acceptance credit.

## The `inputs:` field (statusgen/36)

A second, narrower Context field, `inputs:`, declares the literal paths a Verify row's
Commands read, for the content-bound witness reader. It reuses the `files:` tokenizer's
shape (inline or `- ` list items, backticks optional, an em dash ends the entries) but reads
only literal repo-relative files and directories: a glob, brace, `**`, backslash, absolute
path or `..` makes the declaration unreadable, and an unreadable or absent field means the
witness scope is the whole tree. It is parsed by `evaluation.ParseInputsDeclaration` and
bounded by `evaluation.InputsBoundViolations`; `files:` keeps its grammar and readers
unchanged. See `docs/contracts.md` R-witness-schema.
