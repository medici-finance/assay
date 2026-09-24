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
  `update` and `edit` do not run this check: they act on an existing PR and have no local
  branch diff to read. A PR's trailer, once set, is immutable, but a PR whose body carries
  NO trailer may gain one through `edit` (the pre-trailer migration), and that path runs
  neither diff check. The security side of that gap is covered at flip time by `deskflip`
  (below); the board-edge side is not, so a trailer added by `edit` is only as right as
  the session that wrote it.
- `create` also refuses the MIRROR case (#1641): an
  `Authors:` line is refused unless the branch's diff is authoring-only, by the same
  `deskkit.BriefAuthoringOnly` classification, for EVERY listed id. `Authors:` asserts no
  delivery, so nothing that reads `Brief:` as delivery matches it — including the
  security lane's brief-declared risk term (`deskkit.BriefRiskFromBody` reads `Brief:`
  only). A PR that actually delivers code or a `docs/streams/` document for a `gate:
  human` / `risk: yes` brief could otherwise carry `Authors:` and switch that term off.
  Unlike the `Brief:` direction, a rename here is NOT left alone: an unprovable diff is
  refused rather than trusted, because the whole point of `Authors:` is to switch off a
  risk term the diff must actually back. `deskflip`'s `checkSecurityVerdict` carries the
  BINDING half of this same check (`deskkit.AuthorsRiskFromBody`, read again from the
  PR's forge-served diff at flip time): an `Authors:` PR whose complete changed files are
  not authoring-only for every listed id is risk-classed there too, fail closed, so a PR
  that reached the flip gate some other way than this `create`-time gate (opened before
  the gate existed, or a body edited around it) still cannot switch off the security
  lane by writing `Authors:` instead of `Brief:`.
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
