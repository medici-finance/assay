# deskpr — the PR link trailer

`deskpr create`, `deskpr update` and `deskpr edit` require the PR body to carry exactly
one link trailer, written at the moment the body is filled in:

- `Brief: <stream>/<NN>` — the brief this PR delivers (also accepted: `<stream>:<NN>`,
  `<repo>:<stream>:<NN>`, and the full `<cell>:<repo>:<stream>:<NN>`; the brief-v1
  `<stream>/<NN>` form stays accepted on read during the migration window), **or**
- `Issue: #<N>` — issue-only work with no brief.

Rules (derived-board/02):

- Exactly one link per body. A second `Brief:` line, a second `Issue:` line, or one of
  each, is a refusal naming the offending lines.
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
  trailer may gain one — that is exactly the `update` migration above.
- `Brief: <stream>/<NN>` must resolve to a brief file under `--root`
  (`docs/streams/<stream>/brief-<NN>-*.md`); a value that resolves to nothing refuses
  with the unresolved pattern.
- There is **no** bypass flag (`--no-brief`, env var, commit token). A worker-typeable
  bypass makes the edge asserted again.
- **One exempt body: the derived issue-loop scan carrier.** A body whose head carries the
  machine-written marker `<!-- desk-scanbody v1 -->` (from `deskscanbody emit`) is exempt
  from the trailer requirement. That body is regenerated from the
  branch diff on every push and reconciles a whole-scope scan spanning many issues, so no
  single `Issue: #N` can be both correct and stable across a re-push. The exemption keys off
  the emitter-written marker, not a worker-typeable flag, so it does not reopen the bypass
  the rule above closes — every human-authored body still faces the full gate.
