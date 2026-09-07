### Fixed
- **A PR whose sensitivity is declared in its brief frontmatter (`gate: human`, or any
  `risk:` flag `yes`) is now risk-classed by both `deskboard` and `deskflip`, even when its
  changed paths hit no compiled trigger.** Previously the owning brief never resolved, so the
  board marked such a PR `FLIP` and `deskflip` required no `Security-Review: pass` — a
  human-gated, sensitive-data change was flippable with no security verdict. `deskflip`'s
  security lane now REFUSES the ready flip on such a PR until a reviewer App posts
  `Security-Review: pass` at the current head (absence is never a pass).

### Changed
- **`deskboard` resolves a PR's owning brief from the body's `Brief:` trailer** (both the
  `<stream>/<NN>` slash form and the `<…>:<stream>:<NN>` colon form), falling back to
  branch-as-claim only when the body names no brief. The board's `riskClassed` and
  `deskflip`'s risk classification now UNION a brief term over the existing visibility,
  security-surface-label, and changed-path terms. The brief term only ever WIDENS — it never
  waives a gate another term set. A body with **no** `Brief:` trailer leaves the term silent
  (the other terms decide); a body **with** a `Brief:` trailer that cannot be
  resolved/read/parsed is **UNVERIFIABLE, fail closed** — risk-classed, so the flip is
  refused until a security review passes at head. You cannot prove a declared brief you could
  not read is not `gate: human` / `risk: yes`, so a declared-but-unreadable brief is never
  treated as clean (the same "a short read is UNVERIFIABLE, not clean" rule the changed-file
  gate already uses).

### Added
- `deskkit.BriefRiskFromBody(repo, body)` resolves the `Brief:` trailer to a brief file under
  the configured stream roots and reads its `gate:`/`risk:` frontmatter, returning the owning
  brief id and whether the brief's own declaration risk-classes the PR. `PullRequest` grows a
  `Body` field to feed it. (A later change can consolidate the local trailer splitter onto the
  shared canonicaliser once that lands.)
