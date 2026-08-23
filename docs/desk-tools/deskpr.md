# deskpr — the PR link trailer

`deskpr create` and `deskpr update` require the PR body to carry exactly one link
trailer, written at the moment the body is filled in:

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
  GitHub and refuses (exit 5) with the line to add — the worker edits the body and
  re-runs.
- `Brief: <stream>/<NN>` must resolve to a brief file under `--root`
  (`docs/streams/<stream>/brief-<NN>-*.md`); a value that resolves to nothing refuses
  with the unresolved pattern.
- There is **no** bypass flag (`--no-brief`, env var, commit token). A worker-typeable
  bypass makes the edge asserted again.
