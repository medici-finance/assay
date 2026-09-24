# deskpr — the PR link trailer

`deskpr create`, `deskpr update` and `deskpr edit` require the PR body to carry exactly
one link trailer, written at the moment the body is filled in:

- `Brief: <stream>/<NN>` — the brief this PR delivers (also accepted: `<stream>:<NN>`,
  `<repo>:<stream>:<NN>`, and the full `<cell>:<repo>:<stream>:<NN>`; the brief-v1
  `<stream>/<NN>` form stays accepted on read during the migration window), **or**
- `Authors: <stream>/<NN>[, <stream>/<NN> …]` — the brief(s) a briefs-AUTHORING PR writes and
  does not deliver (entries separated by commas and/or spaces; each accepts every form `Brief:`
  does), **or**
- `Issue: #<N>` — issue-only work with no brief.

`Brief:` asserts DELIVERY. The dispatcher's phantom check, `fanoutloop plan`'s
already-represented reconciliation and the derived board all read a `Brief:` PR as the
brief's delivery, so a docs-only PR that only WROTE a brief must carry `Authors:` (or
`Issue:`), never `Brief:` — otherwise the brief reads as delivered the moment the
authoring PR merges and is never dispatched (#1339). No reader keys on `Authors:`.

Rules (derived-board/02):

- Exactly one link per body. A second `Brief:`, `Issue:` or `Authors:` line, or two
  different kinds, is a refusal naming the offending lines.
- The line may sit anywhere in the body; a trailer inside a fenced code block is
  documentation and is ignored.
- `Closes #N` / `Refs #N` keep their GitHub meaning and are NOT the link — a PR may
  close an issue and deliver a brief.
- `create` checks before any network call; `update` reads the PR's existing body from
  GitHub and refuses (exit 5) with the line to add — the worker fixes the body with
  `deskpr edit` and re-runs.
- `edit` checks the REPLACEMENT body before any network call, and additionally refuses
  (exit 5) when the replacement's trailer differs from the one the PR's current body
  already carries: the link is not editable after the fact. A current body carrying NO
  trailer may gain one — that is exactly the `update` migration above. But a PR that opened
  with the WRONG form (e.g. a `Brief:` line where the work delivers no brief, or vice
  versa) cannot be corrected in place: `edit` refuses the differing trailer, so the fix is
  a fresh branch and a new PR carrying the right link trailer, not an edit to this one.
- `Brief: <stream>/<NN>`, and every entry of `Authors:`, must resolve to a brief file
  under `--root` (`docs/streams/<stream>/brief-<NN>-*.md`); a value that resolves to
  nothing refuses with the unresolved pattern. An `Authors:` list that is empty, holds a
  non-brief entry, or names one brief twice refuses.
- `create` refuses a `Brief:` line when the branch only AUTHORS that brief: its diff
  against the base adds the brief's own file and touches nothing but stream board
  READMEs (`docs/streams/<stream>/README.md`), brief files and changelog fragments. The
  refusal names the `Authors:` line to use. It is the same classification the
  dispatcher and the planner apply to already-merged PRs (`deskkit.BriefAuthoringOnly`),
  so a PR that authors a brief AND delivers anything else (code, or a document under
  `docs/streams/`) is a delivery and keeps `Brief:`. A diff with a rename is not judged
  (its old path is not visible to this local read) and is never refused on this ground.
  `update` and `edit` do not run this check: they act on an existing PR whose trailer is
  immutable.
- Already-merged authoring PRs that carry `Brief:` do not block their briefs: the
  dispatcher's phantom check and `fanoutloop plan` both read a representing PR's changed
  files and set aside one that only authored the brief (a file list that cannot be read
  or proven complete keeps the PR counted, never rounded to "authoring").
- There is **no** bypass flag (`--no-brief`, env var, commit token). A worker-typeable
  bypass makes the edge asserted again.
- **One exempt body: the derived issue-loop scan carrier.** A body whose head carries the
  machine-written marker `<!-- desk-scanbody v1 -->` (from `deskscanbody emit`) is exempt
  from the trailer requirement. That body is regenerated from the
  branch diff on every push and reconciles a whole-scope scan spanning many issues, so no
  single `Issue: #N` can be both correct and stable across a re-push. The exemption keys off
  the emitter-written marker, not a worker-typeable flag, so it does not reopen the bypass
  the rule above closes — every human-authored body still faces the full gate.
