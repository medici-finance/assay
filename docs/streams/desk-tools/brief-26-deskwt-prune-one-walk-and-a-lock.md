---
brief: assay:assay:desk-tools:26
title: "`deskwt prune` — one origin/main walk per sweep, the merge gate before `Status()`, batched removal, a read-only `--dry-run`, and a prune singleton"
why: >-
  `deskwt prune` is a BOOT STEP: every desk window runs it, so its cost is paid at the
  moment a human is waiting. On one operating checkout carrying ~657 registered worktrees
  over a 5,779-commit `origin/main`, five windows booting inside one minute each sat at
  ~100 % CPU for 8–12 minutes. Enumeration is not the cost — that is ~1.2 ms per worktree.
  The cost is three in-process go-git walks repeated PER CANDIDATE, and ~97 % of it is
  work no gate reads. `unmergedReason` walks the ENTIRE `origin/main` history into a map
  and then walks HEAD, purely to render a commit COUNT inside a skip string nothing parses
  (~36 %). `mergedToOriginMain` calls go-git's unmemoized `IsAncestor`, a preorder walk of
  `origin/main` that runs to exhaustion for the 19-in-20 candidates that are genuinely
  unmerged, zlib-inflating the same commits out of the pack again for every one of them
  (~36–39 %). And `dirtyTracked` runs a full `Status()` over ~4,100 files BEFORE the merge
  gate, so ~95 % of those walks are spent on worktrees that are held anyway (~25 %).
  `gitcore.Open` builds a fresh object cache on each of its 3–4 calls per worktree, so the
  same commits are re-inflated ~700× in one sweep. None of that is a heuristic to tune: the
  sweep asks ONE question of ONE history N times and shares nothing between the answers.
  This brief makes it ask once. It also closes three structural defects found alongside:
  prune takes no lock of any kind, so five boots are five identical full sweeps; each
  removal runs its own `git worktree prune` plus a full `list --porcelain` (an O(N²) term
  that cost nothing only because that sweep removed nothing); and `--dry-run` is not
  read-only — it runs `git worktree prune` before it ever reaches the dry-run check.
wave: 2
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1037]
schema: brief-v2
authored: >-
  2026-09-14 by a worker-desk authoring session, from issue #1037 and a same-day read of the
  prune, worktree-removal and gitcore surfaces named below
sources:
  - "Issue #1037 — the measurement this brief implements: ~657 registered worktrees (~348 prune candidates), 5,779 commits on origin/main, a ~920 MB pack; 0.77–1.28 s per worktree single-process, ~2.1 s at five-concurrent (1.64× contention, no serialisation), i.e. 4.5–7.4 min single and 7–12 min at ×5. Its three named hot spots and its F1–F7 asks are the scope of this brief."
  - "freshness-checked 2026-09-14 @ e428134c (origin/main) — every line reference and every claim below re-read against the source at that commit."
  - "`tools/desk/cmd/deskwt/prune.go` § `pruneSweep` (L292) — the sweep body and the exact gate ORDER this brief reorders: lock (L344) → shared-checkout/cwd/prefix (L349–359) → `dirtyTracked` (L362) → `headAtOriginMainTip` (L381) → `mergedToOriginMain` (L392) → `unmergedReason` (L398) → dry-run check (L404) → `removeWorktreeDir` (L419)."
  - "`tools/desk/cmd/deskwt/prune.go` § `unmergedReason` (L441) — the ~36 % block: `repo.UpstreamRef()` then `repo.AheadCount(upstream, \"HEAD\")` (L451) whose only use is the `%d` in a skip string. No gate reads the number."
  - "[pre-lift Git implementation](https://github.com/medici-finance/assay/blob/e428134c6c3668b56e44302720ab704d1ea6e5c3/tools/desk/internal/gitcore/gitcore.go) § `AheadCount` (L717) — walks `Log(base)` into `baseAncestors` to exhaustion, then walks head. Two full history walks per call."
  - "[pre-lift Git implementation](https://github.com/medici-finance/assay/blob/e428134c6c3668b56e44302720ab704d1ea6e5c3/tools/desk/internal/gitcore/gitcore.go) § `IsAncestor` (L401) — resolves both ends, decodes both commits, and delegates to go-git's `object.Commit.IsAncestor`, which is unmemoized: a fresh preorder walk per call, nothing shared across calls."
  - "[pre-lift Git implementation](https://github.com/medici-finance/assay/blob/e428134c6c3668b56e44302720ab704d1ea6e5c3/tools/desk/internal/gitcore/gitcore.go) § `Open` (L87) — builds `cache.NewObjectLRUDefault()` fresh on every call, so nothing decoded by one Open is visible to the next."
  - "[pre-lift Git implementation](https://github.com/medici-finance/assay/blob/e428134c6c3668b56e44302720ab704d1ea6e5c3/tools/desk/internal/gitcore/gitcore.go) § `Log` (L348) — already returns the reachable hashes of a rev, newest first; the ancestor SET this brief needs is `Log` plus three lines, not a new primitive."
  - "[pre-lift containment reader](https://github.com/medici-finance/assay/blob/e428134c6c3668b56e44302720ab704d1ea6e5c3/tools/desk/internal/gitcore/contains.go) § package header — the in-repo precedent for exactly this defect class, written up at length: `RefsContaining` once called go-git's unmemoized `IsAncestor` once per ref (~10 million commit visits, ~108 s where the git binary answered in ~15 ms) and was fixed by ONE shared walk plus a commit-graph generation-number cut-off. Same package, same cause, opposite direction of the question."
  - "`tools/desk/cmd/deskwt/deskwt.go` § `dirtyTracked` (L236) → `gitcore.DirtyTrackedPorcelain` (gitcore.go L815) → go-git `wt.Status()` — the ~25 % block, and the SINGLE implementation shared by `remove` and `prune`."
  - "`tools/desk/cmd/deskwt/deskwt.go` § `removeWorktreeDir` (L256) — `os.RemoveAll`, then `git worktree prune`, then a full `guard.worktreePaths(dir)` (a `git worktree list --porcelain` parse) to positively verify deregistration — all three PER removal. Shared with `remove`."
  - "`tools/desk/cmd/deskwt/deskwt.go` § `lockedWorktrees` (doc comment L204, signature L215) — the porcelain lock read, already done ONCE per sweep; the shape every other per-sweep read in this brief follows."
  - "`tools/desk/internal/deskkit/filelock_unix.go` / `filelock_windows.go` § `TryLockExclusive` / `UnlockFile` / `ErrLockBusy` — the house's non-blocking advisory lock, both platform arms already present, documented as FAILING CLOSED (never reports success when the lock was not taken). This is the singleton's concurrency mechanism; nothing new is written for it."
  - "`tools/desk/internal/deskkit/killswitch.go` § `deskDir` / `StateDir` (L57/L75) — `~/.config/assay`, the ONE state-directory resolution path, with the `dirOverride` white-box test hook, and its doc comment's explicit invitation for desk tools to place their own state beside the shared state."
  - "`tools/desk/cmd/deskwt/prune_test.go` § `TestPruneSkipsUnpushedCommit` (L189) and `TestPruneSkipsCleanUnmergedBranch` (L211) — the positive and NEGATIVE control that together pin the unpushed-vs-unmerged distinction. They are why F1 replaces the `AheadCount` block rather than deleting it; see the correction recorded in the facts."
  - "`tools/desk/cmd/deskwt/prune_test.go` § `TestPruneSkipsDirtyTracked` (L138) — the test whose asserted skip REASON the F3 gate reorder changes for a worktree that is both dirty and unmerged."
  - "PR #1045 (OPEN at authoring time) — the same fix applied to a different caller: `deskpushguard` walked `origin/main` once per candidate remote branch via the same unmemoized `IsAncestor` and now walks once per push into a hash set. Read, not depended on: see the Dependencies note."
exec-tier: strong
exec-tier-why: >-
  (b) this rewrites the ORDER and the SHARING of the gates on a DESTRUCTIVE verb. Every
  change here is a performance change whose failure mode is a wrong REMOVAL, and the
  distance between "600 worktrees swept in 20 s" and "an active worker's worktree deleted"
  is one inverted set-membership test. Three specific hazards need a strong hand: an
  ancestor set that is silently EMPTY (a `Log` that failed, a resolve that returned the
  wrong ref) makes every candidate read as unmerged — safe — while an ancestor set built
  from the WRONG ref makes unmerged candidates read as merged, which deletes live work and
  looks like a successful sweep; the gate reorder must move `Status()` without moving the
  SAFETY of any gate, and a green suite looks identical whether the reorder preserved the
  holds or merely stopped reaching them; and a singleton that fails CLOSED on a stale stamp
  wedges the boot step of every desk window on the machine. Each needs a row whose failure
  is a deletion the fixture can observe, not a timing number.
consumers:
  - "tools/desk/cmd/deskwt/prune.go: fixed-here (the sweep body — the ancestor set, the gate reorder, the batched removal, the read-only dry-run arm, and the singleton call site)."
  - "tools/desk/cmd/deskwt/deskwt.go: fixed-here (`removeWorktreeDir` is SPLIT so prune can batch the deregistration step; `remove`'s own call keeps the per-path prune-and-verify it has today, byte-identical in behaviour — Verify row 8 is the assertion)."
  - "tools/desk/internal/gitcore/gitcore.go: fixed-here (ONE added exported function, `OpenWith(dir, cache)`; `Open` becomes a one-line call to it with a fresh cache, so every existing caller is unchanged — Verify row 9)."
  - "`deskwt remove` (the human-named single-path verb): out-of-scope (it removes ONE operator-named path; it has no loop to hoist a walk out of, no second candidate to share a cache with, and no singleton to take. Its gates, its order and its per-path verification are untouched, and Verify row 8 asserts it)."
  - "`gitcore.AheadCount`: out-of-scope (the FUNCTION stays — `deskpreflight` and the push guard read a real count that a human reads. This brief removes prune's CALL to it, which is the only caller that discards the number into a string; the function's own tests are unchanged)."
  - "`gitcore.IsAncestor`: out-of-scope (unchanged and still correct; prune stops calling it per candidate, and every other caller keeps it. Making `IsAncestor` itself memoized is a `gitcore`-wide change with a cache-lifetime question this brief does not answer — it is named as follow-up, not attempted)."
  - "`gitcore.RefsContaining` / `contains.go`'s `reachIndex`: out-of-scope (read as the PRECEDENT, reused as a pattern, not called. It answers the dual question — which tips contain one commit — and its index is built for a fan-out this sweep does not have; wiring prune through it would build a parent→children adjacency over the whole ref set to answer N single-membership questions)."
  - "The `deskboot` worktree-prune step and the `--interval` supervisor: out-of-scope as POLICY (whether a boot step should sweep at all, versus leaving it to the supervisor, is a house guardrail decision recorded on the adopter side and not settled here). Both are affected MECHANICALLY by the singleton and neither is edited: the supervisor takes the lock per TICK rather than for its lifetime, precisely so it cannot lock out a manual sweep — Verify row 12."
  - "`deskkit.TryLockExclusive` / `UnlockFile` / `ErrLockBusy`: out-of-scope (consumed exactly as they are; no new locking primitive is written and neither platform arm is touched)."
version: 2
id: 77c06d5e-1772-433f-ad39-327f9603c466
---

# Brief 26 — `deskwt prune`: one history walk, the merge gate first, batched removal, a prune singleton

## Dependencies

None. `depends:` is empty and the brief is dispatchable as authored.

One adjacent PR is deliberately NOT a dependency. PR #1045 (open at authoring time) applies the
same technique — one `Log(origin/main)` walk hoisted into a hash set, replacing a per-candidate
`IsAncestor` — to `deskpushguard`, and its write-up describes the identical defect. It is read
here as corroboration and as a wording reference, not consumed: it lands in `cmd/deskpushguard`,
so it exports nothing this brief could call, and taking a dependency on an unmerged PR would
block a brief that has no need to wait. The consequence is that after both land, `deskpushguard`
and `deskwt` each build their own ancestor set from `gitcore.Repo.Log`. That is three lines
apiece over a shared primitive, not a duplicated algorithm; consolidating the pair into a
`gitcore` helper once BOTH have merged is named as follow-up in the Ground rules and attempted
in neither.

## Context

single-point-of-failure: **the merge gate — `HEAD` must be an ancestor of `origin/main` — is
the one control standing between the sweep and the deletion of live work, and this brief
replaces its entire implementation.** Behind it are three layers, each tripping on a different
signal in a different component, and none of them is touched by this change: (1) the LOCK gate
(`prune.go` L344), read once per sweep from `git worktree list --porcelain` and consulted FIRST,
which holds any worktree a session has locked whatever every content heuristic says — it is the
control that survived issue #264 and it runs before the merge question is even asked; (2) the
FRESH-WORKTREE guard (`headAtOriginMainTip`), a hash equality against `refs/remotes/origin/main`
that holds any worktree sitting exactly at the tip, catching the case the merge gate is
structurally blind to (a worktree at the tip IS an ancestor, and may hold untracked new work);
and (3) the TRACKED-CLEAN gate (`dirtyTracked`), which this brief MOVES but does not weaken — a
dirty worktree is still never removed, and moving it after the merge gate can only reduce the
population that reaches it, never widen what is deleted. The three are independent: the lock
reads git's admin metadata, the tip guard reads two ref hashes, the clean gate reads the
worktree's index and files, and the merge gate reads commit ancestry. A fault in the new
ancestor set is caught by the lock gate for every locked worktree, by the tip guard for every
fresh one, and by the clean gate for every dirty one — and by Verify rows 2, 3 and 4, which are
the ones that exist because the other three layers do not cover a clean, unlocked, below-tip,
genuinely-unmerged worktree. That case is the one this brief must not get wrong, and it is what
rows 2 and 4 plant.

The fail-closed direction is stated here because it is the design's load-bearing asymmetry: if
the per-sweep ancestor set CANNOT be built — `refs/remotes/origin/main` does not resolve, the
walk errors, the shared repo cannot be opened — the sweep does not fall back to a per-candidate
walk and does not proceed with an empty set. It holds EVERY candidate with a
could-not-check reason and returns 0 removals. An empty set read as data would mark every
worktree unmerged, which is accidentally safe; an empty set read as an ANSWER is a confident
wrong answer waiting for the day the ref resolves to something unexpected, and the three-state
rule says an instrument that did not look has cleared nothing.

risk note — all four risk answers are `no`, and the change is to a DESTRUCTIVE verb, so the
reasoning is stated rather than assumed. `irreversible: no` holds because nothing this brief
adds can delete anything the current code would not have deleted: every gate that holds a
worktree today still holds it, the removal set is proven identical to the pre-change set on a
mixed fixture (row 2), and the only NEW deletions in the diff are the singleton's own stamp
file. `customer: no` and `regulatory: no` hold because the verb is local-only — filesystem and
worktree state, no outward call, no forge write, no credential. `sensitive-data: no` holds
because the singleton stamp carries a pid, two timestamps and a hash of a repository path, and
no path, branch, ref or identity in clear text. A reviewer who finds that the reorder changed
WHICH worktrees are removed rather than only the order the gates run in, or that a failed
ancestor-set build can reach the removal step, flips `irreversible` to yes and takes the human
gate.

files:
- `tools/desk/cmd/deskwt/prune.go` (existing) — `pruneSweep` restructured around a per-sweep `sweepCtx` (the ancestor set, the shared object cache, the per-worktree `*gitcore.Repo` cache); the gate reorder; `unmergedReason`'s count removed; the batched removal tail; the read-only `--dry-run` arm; the singleton acquisition; two new flags.
- `tools/desk/cmd/deskwt/prunesingleton.go` (planned) (new) — the stamp file, its flock, the TTL debounce, the held/recent report strings, and the stamp's read/write/parse.
- `tools/desk/cmd/deskwt/deskwt.go` (existing) — `removeWorktreeDir` split into `removeWorktreeTree` (the `os.RemoveAll` alone) and the existing prune-and-verify tail; `remove`'s call site keeps both halves in the order it has today.
- **edit** — [pre-lift Git implementation](https://github.com/medici-finance/assay/blob/ee2b42102c5f57e7737379bc88dc5902f0624f83/tools/desk/internal/gitcore/gitcore.go) (existing) — `OpenWith(dir string, objects cache.Object) (*Repo, error)`; `Open` becomes `OpenWith(dir, cache.NewObjectLRUDefault())`.
- `tools/desk/cmd/deskwt/prunesingleton_test.go` (planned) (new).
- `tools/desk/cmd/deskwt/pruneperf_test.go` (planned) (new) — the synthetic-repo fixture builder, the N-scaling test, the concurrency test and the per-candidate benchmark.
- `tools/desk/cmd/deskwt/prune_test.go` (existing) — new rows for the gate order, the read-only dry run and the fail-closed ancestor set; `TestPruneSkipsDirtyTracked`'s fixture made unambiguous (see the facts).
- **edit** — [pre-lift Git tests](https://github.com/medici-finance/assay/blob/ee2b42102c5f57e7737379bc88dc5902f0624f83/tools/desk/internal/gitcore/gitcore_test.go) (existing) — the shared-cache row.
- `tools/desk/cmd/deskwt/prune-mutations.json` (planned) (new) — the `muhar` spec for row 11.
- `tools/desk/README.md` (existing) — the prune singleton, its two flags, and the statement that `--dry-run` is read-only.
- `changelog/desk-tools-26-prune-perf.md` (planned) (new).

facts (read at `e428134c`, 2026-09-14):

- **The sweep asks one question of one history, N times, and shares nothing.** `pruneSweep`
  (prune.go L292) reads the lock map ONCE (L336) and then, per candidate, calls
  `dirtyTracked` (L362), `headAtOriginMainTip` (L381), `mergedToOriginMain` (L392) and — for
  the 19-in-20 that are unmerged — `unmergedReason` (L398). Each of those four opens the
  worktree with its own `gitcore.Open`, and each `Open` builds a fresh
  `cache.NewObjectLRUDefault()` (gitcore.go L87), so no object decoded by one survives into
  the next. At ~348 candidates that is ~700–1,400 Opens and ~700–1,400 independent object
  caches in one sweep.
- **The ~36 % block renders a number nothing reads.** `unmergedReason` (L441) exists to refine
  a SKIP STRING. Its expensive half is `repo.AheadCount(upstream, "HEAD")` (L451), and
  `AheadCount` (gitcore.go L717) walks `Log(base)` into a map to exhaustion and then walks
  head. The result is interpolated into `"unpushed (%d commit(s) ahead of upstream, not on
  origin/main)"` and nothing — no gate, no test, no caller — parses the digits back out.
- **Correction to the ask, recorded rather than dropped** (worker-kit clause 7). Issue #1037's
  F1 says to DELETE the `AheadCount` block and "keep the skip string without the count".
  Deleting the block outright would also delete the unpushed-vs-unmerged DISTINCTION, which is
  pinned by two tests that were read before this was decided:
  `TestPruneSkipsUnpushedCommit` (prune_test.go L189) asserts the word `unpushed` appears, and
  `TestPruneSkipsCleanUnmergedBranch` (L211) asserts as a NEGATIVE control that a clean,
  pushed-but-unmerged worktree is NOT labelled unpushed. So the block is REPLACED, not
  removed: `UpstreamRef()` (a config read, no walk) plus `Resolve(upstream)` and
  `Resolve("HEAD")` (two ref reads, no walk) answer "is HEAD somewhere other than its
  upstream" for two map lookups' worth of work. What goes is the COUNT, exactly as the ask
  intends; what stays is the distinction the ask did not mention and the tests already own.
  The skip string becomes `unpushed (HEAD is not at its upstream, and not on origin/main)` —
  a claim the comparison actually proves, where `(N commit(s) ahead …)` asserted a direction
  the replacement does not measure.
- **The ~36–39 % block re-walks the same history per candidate.** `mergedToOriginMain` (L474)
  calls `gitcore.IsAncestor` (gitcore.go L401), which delegates to go-git's
  `object.Commit.IsAncestor` — an unmemoized preorder walk. For a candidate that IS merged the
  walk short-circuits; for one that is NOT it runs to the roots. #1037 measured 19 of 20
  candidates unmerged, so the common case is the exhaustive one, repeated per candidate,
  inflating the same 5,779 commits out of a ~920 MB pack every time.
- **This exact defect has been diagnosed and fixed once already in this package.**
  [pre-lift containment reader](https://github.com/medici-finance/assay/blob/ee2b42102c5f57e7737379bc88dc5902f0624f83/tools/desk/internal/gitcore/contains.go)'s header records `RefsContaining` calling the same unmemoized
  `IsAncestor` once per ref — ~10 million commit visits, ~108 s where the git binary answered
  in ~15 ms — and its fix is the same shape this brief takes: one shared walk instead of N.
  The direction of the question differs (which tips contain one commit, versus is this one
  commit contained in one tip, asked N times), which is why that index is the PRECEDENT and
  not the implementation.
- **The gates run in the wrong order for cost.** `dirtyTracked` (L362) is third, ahead of the
  tip guard and the merge gate. It runs a full go-git `Status()` (gitcore.go L815) over the
  worktree — ~4,100 files on the measured checkout — and its answer is then discarded for the
  ~95 % of candidates that go on to be held by the merge gate anyway. The two gates ahead of
  it are a hash comparison and a set lookup once F2 lands.
- **The gate order is observable in the skip REASON, and one existing test sits on the
  boundary.** A worktree that is BOTH dirty AND unmerged is reported `dirty (uncommitted
  tracked changes)` today and will be reported `unmerged (…)` after the reorder. Both are
  holds, so nothing about what is DELETED changes — but `TestPruneSkipsDirtyTracked`
  (prune_test.go L138) asserts a reason, and its fixture must be made unambiguous rather than
  its assertion loosened: the dirty worktree it builds is committed onto `origin/main` first
  so that dirtiness is the ONLY gate that can hold it. A test that would accept either reason
  stops pinning the gate it exists to pin.
- **Removal is O(N²) in the worktree count, and it cost nothing only by luck.**
  `removeWorktreeDir` (deskwt.go L256) does `os.RemoveAll`, then `git worktree prune`, then
  `guard.worktreePaths(dir)` — a full `git worktree list --porcelain` parse — PER removal. At
  N≈675 that listing is ~1 s. The measured sweep removed zero worktrees, so the term never
  fired; a sweep that actually drains 300 worktrees pays it 300 times.
- **`--dry-run` is not read-only.** Step A's `bookkeepingPrune` runs `git worktree prune
  --verbose` at prune.go L297, unconditionally, and the dry-run check is at L404 — more than a
  hundred lines later, inside the candidate loop. A dry run therefore mutates
  `.git/worktrees/` before it reports anything. The opt-in lock-reclaim pass (L308) is on the
  same side of the check and UNLOCKS worktrees during a dry run too.
- **git's own prune has a dry-run mode.** `git worktree prune` accepts `-n`/`--dry-run`
  alongside `--verbose`, and prints the same per-entry report to stderr that
  `bookkeepingPrune` already counts. The read-only arm is a flag, not a reimplementation.
- **prune takes no lock of any kind.** There is no flock, no stamp, no debounce and no
  coordination anywhere in prune.go: five windows booting inside one minute run five
  identical full sweeps concurrently, which #1037 measured at 1.64× contention and no
  serialisation. Four of those five sweeps are pure waste even when the first succeeds.
- **The house already owns the locking primitive, on both platforms.**
  `deskkit.TryLockExclusive` / `UnlockFile` / `ErrLockBusy` (filelock_unix.go,
  filelock_windows.go) are a non-blocking exclusive advisory lock whose doc comment states it
  FAILS CLOSED and that `ErrLockBusy` must be read as "another op is live", never as "free".
  Nothing new needs to be written for the singleton's concurrency half.
- **The state directory is resolved in exactly one place.** `deskkit.StateDir()`
  (killswitch.go L75) returns `~/.config/assay`, carries the `dirOverride` white-box test hook
  every state test in the suite uses, and its doc comment explicitly invites desk tools to
  place their own state there. The singleton's stamp goes under a `prune/` subdirectory of it
  — never inside the target repository's `.git`, which is what keeps the read-only dry-run
  claim (row 6) true for the singleton as well as for the sweep.

facts — the design:

- **F1 — the count goes, the distinction stays.** `unmergedReason` no longer calls
  `AheadCount`. It resolves the upstream ref and `HEAD` and compares the two hashes: unequal →
  `unpushed (HEAD is not at its upstream, and not on origin/main)`; equal, or no upstream
  configured, or either resolve fails → the existing `unmerged (branch <B> not an ancestor of
  origin/main — active work)`. Two ref reads replace two history walks. The function is called
  only for worktrees the merge gate has already held, so it can never influence a removal.
- **F2 — one walk per sweep, hoisted above the loop.** Before the candidate loop, the sweep
  opens the SHARED checkout once and calls `repo.Log("refs/remotes/origin/main")`, collecting
  the hashes into a `map[string]struct{}`. Per candidate, the merge test becomes
  `Resolve("HEAD")` in that worktree plus one lookup in the set. This is EXACT, not an
  approximation: `IsAncestor(HEAD, origin/main)` is true precisely when HEAD is in the set of
  commits reachable from `origin/main`, and `Log` returns that set including `origin/main`
  itself. Every linked worktree shares the main checkout's object store and its
  remote-tracking refs (which is what `gitcore.Open`'s `commondir` routing exists to honour),
  so a set built once at the shared checkout is the same set every worktree would have built
  for itself.
- **F2, fail-closed.** If the shared repo cannot be opened, `refs/remotes/origin/main` does not
  resolve, or the walk errors, the sweep records ONE warning naming the cause and holds every
  candidate with `unverifiable merge status (sweep-level): <cause>` — it does not fall back to
  a per-candidate walk and does not continue with a partial or empty set. Removals for that
  sweep are 0. The base is spelled `refs/remotes/origin/main` in full, for the reason
  `mergedToOriginMain`'s existing comment gives at length (issue #885: `refs/heads/` wins
  disambiguation, so a stray local `origin/main` would silently become the baseline).
- **Memory.** The set is one 40-byte hash string per commit reachable from `origin/main`: at
  the measured 5,779 commits ≈ 0.5 MB with map overhead, and ≈ 8 MB at 100,000 commits. It is
  built once, read N times and dropped when the sweep returns. It replaces N independent walks
  each of which decoded the same commits.
- **F3 — the merge gate moves ahead of `Status()`.** The new order is: lock → shared-checkout
  / cwd / prefix → resolve HEAD → fresh-at-tip guard → merge gate → tracked-clean gate →
  remove. Every gate is preserved and none is weakened; the three ahead of the clean gate are
  a map lookup and two hash comparisons, so the expensive `Status()` runs only for candidates
  that have already passed the merge gate. On the measured checkout that is ~5 % of them. The
  only observable change is the skip REASON reported for a worktree that would fail more than
  one gate — it now names the first gate that holds it in the new order.
- **F4 — one object cache per sweep, one Open per worktree.** `gitcore.OpenWith(dir,
  objects cache.Object)` is added: it is today's `Open` with the cache supplied by the caller
  rather than constructed inside, and `Open(dir)` becomes one line calling it with a fresh
  `cache.NewObjectLRUDefault()` — so every existing caller in the tree is behaviourally
  unchanged. The sweep constructs ONE cache and passes it to every `OpenWith`, and holds a
  `map[string]*gitcore.Repo` so each worktree is opened at most once per sweep and its
  `Resolve`/`Status`/upstream reads share that one handle. This also LOWERS the memory
  ceiling rather than raising it: one LRU with go-git's default cap, instead of one per Open.
  The sweep is sequential, so the shared cache is never used concurrently and this brief makes
  no claim about the cache type's own thread-safety.
- **F5 — one deregistration pass, with the verification kept.** `removeWorktreeDir` is split:
  `removeWorktreeTree(rt)` does the `os.RemoveAll` alone, and the `git worktree prune` +
  `worktreePaths` verification tail stays where it is. `remove` calls both, in that order, and
  is byte-identical in behaviour. `prune` calls only the first inside the loop, collecting each
  successfully-deleted path, and after the loop runs ONE `git worktree prune` and ONE
  `guard.worktreePaths(dir)`. The positive verification is NOT dropped, only batched: every
  collected path is checked against that single listing, and a path still registered is
  DEMOTED — it does not count as removed, and it is reported as a warning naming the path. So
  `res.removed` continues to count only deregistrations this sweep actually observed. Two
  `git` invocations replace 2N.
- **F6 — `--dry-run` touches nothing.** Step A becomes `git worktree prune --dry-run
  --verbose` when `opts.dryRun` is set, which reports the same count it would have dropped
  without dropping it. The lock-reclaim pass is likewise made reporting-only under `--dry-run`
  — it prints each lock it WOULD reclaim with the same evidence line, and unlocks nothing —
  because a dry run that silently unlocks is a worse surprise than one that silently prunes
  bookkeeping. The second bookkeeping prune (prune.go L316) is skipped for the same reason. A
  dry run also does not take or write the singleton stamp: it reports whether a sweep would
  have been held, and touches no state.
- **F7 — the singleton is a lock plus a debounce, in one stamp file.** Path:
  `<StateDir()>/prune/<sha256 of the resolved shared-checkout path>.stamp`, mode 0600 in a
  directory created 0700. The key is a hash so no path character can escape into a filename,
  and it is per-REPO so two different checkouts never block each other. The file carries a
  small JSON object: `{schema, pid, startedAt, finishedAt, repoHash}`. Two mechanisms, and
  they answer different questions:
  - **Concurrency — `deskkit.TryLockExclusive` on that file.** A sweep that cannot take the
    lock does not sweep. It prints `deskwt prune: held by pid <pid>, running <age> — skipping
    this sweep` (reading pid and `startedAt` from the stamp's CONTENT, purely to name the
    holder) and exits 0 with a `noop` audit result. An advisory flock is released by the
    KERNEL when its holder dies or exits, however it dies, so there is no liveness question to
    answer and nothing to time out.
  - **Recency — the TTL debounce.** A sweep that DOES take the lock reads `finishedAt`: if a
    sweep completed less than `--singleton-ttl` ago (default 10m), it skips with `deskwt
    prune: swept <age> ago (under the <ttl> singleton TTL) — skipping this sweep`. This is
    what the five-windows-in-one-minute case actually needs: window 1 sweeps, windows 2–5 that
    arrive while it runs are held by the lock, and a window arriving ten seconds AFTER it
    finishes is held by the TTL. `--singleton-ttl 0` disables the debounce; the LOCK is never
    disableable, and there is no `--force` — prune has none anywhere and gains none here.
  - **Why this does not recreate the stale-lock class**, stated explicitly because it is the
    failure mode the design is chosen to avoid. The stamp file is NEVER consulted for
    liveness. Liveness is the flock alone, and a killed, crashed or `SIGKILL`ed prune releases
    it at process exit with no cleanup path to forget — so a leftover stamp cannot hold
    anything. The stamp's only powers are to name a holder in a message and to carry one
    timestamp, and the TTL half fails OPEN: a stamp that is missing, unreadable, truncated,
    not JSON, or carrying a future or unparseable `finishedAt` is treated as NO STAMP and the
    sweep PROCEEDS. The worst a corrupt or abandoned stamp can cost is one delayed sweep, for
    at most the TTL, and the sweep it delays is a boot-time convenience, never a safety
    control. This is the opposite posture to every other gate in this verb, and deliberately
    so: the singleton is an efficiency mechanism, and an efficiency mechanism that fails
    closed can wedge every desk window on the machine.
  - **The `--interval` supervisor takes the lock per TICK, not for its lifetime** (prune.go
    L246's loop), and releases it before it sleeps. A long-lived supervisor holding the lock
    forever would lock out every manual and boot-time sweep on the machine, turning the
    singleton into an outage. It also honours the TTL, so a supervisor ticking more often than
    the TTL simply skips the ticks that are too close together.
- **Combined expectation, from #1037's measurement.** F1 + F2 + F3 remove ~97 % of the
  per-candidate cost, taking the measured 4.5–7.4 min single-process sweep to under 30 s; F4
  compounds it by removing the repeated inflation; F5 removes an O(N²) term that the measured
  sweep never paid; F7 takes the five-concurrent case from 7–12 min to about one sweep. The
  Verify rows measure the synthetic equivalents rather than restating these numbers.

## Ground rules
- **Nothing that is removed today may stop being removed, and nothing that is held today may
  start being removed.** Every gate survives the reorder; the only permitted observable change
  to a HELD worktree is which reason it is reported with. Row 2 is the assertion, over a mixed
  fixture, and it compares SETS, not counts.
- **The ancestor set fails closed.** A set that cannot be built holds every candidate. There is
  no fallback to per-candidate walking, no partial set, and no empty set treated as an answer.
- **No `--force`, anywhere.** prune's doc comment states it has none; this brief adds none,
  including any flag that would bypass the singleton's lock.
- **The singleton's TTL fails OPEN and its lock fails CLOSED.** An unreadable stamp means
  sweep; a lock that cannot be taken means do not sweep. Neither may be inverted.
- **`--dry-run` writes nothing at all** — not the bookkeeping prune, not a lock reclaim, not
  the singleton stamp, not a byte inside the target repository.
- **`deskwt remove` is not changed.** The split of `removeWorktreeDir` is a refactor whose only
  consumer-visible obligation is that `remove` still prunes and verifies per path, in the order
  it does today.
- **`gitcore.Open`'s signature and behaviour are unchanged** for every existing caller. The
  cache-sharing is an ADDITIVE second entry point.
- **Consolidating the ancestor-set helper into `gitcore`** once PR #1045 has merged is named
  follow-up and is not attempted here. Neither is memoizing `gitcore.IsAncestor` itself.
- Measurement fixtures are SYNTHETIC repositories created under the test's own `t.TempDir()`.
  No Verify row in this brief runs against a real checkout, and none runs `git worktree
  prune`, `deskwt prune` or any removal against a directory it did not create.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Deliverables

1. **F1 — `unmergedReason` without the walk** (`prune.go`). The `AheadCount` call is replaced
   by an upstream/HEAD hash comparison; the skip string loses the count and gains wording the
   comparison proves. The unpushed-vs-unmerged distinction is preserved, and both existing
   tests that pin it pass unchanged.
2. **F2 — one `origin/main` walk per sweep** (`prune.go`). A `sweepCtx` built before the
   candidate loop carrying `mainAncestors map[string]struct{}` from
   `repo.Log("refs/remotes/origin/main")` on the shared checkout. `mergedToOriginMain` becomes
   a `Resolve("HEAD")` plus a set lookup. A set that cannot be built holds every candidate with
   a sweep-level could-not-check reason and one warning naming the cause.
3. **F3 — the gate reorder** (`prune.go`). Merge gate and fresh-at-tip guard ahead of
   `dirtyTracked`. `TestPruneSkipsDirtyTracked`'s fixture is made single-gate so it still pins
   the gate it names.
4. **F4 — one object cache and one Open per worktree per sweep**
   (`gitcore.go`, `prune.go`). `OpenWith(dir, objects cache.Object)` added; `Open` delegates to
   it. The sweep holds one cache and a `map[string]*gitcore.Repo`.
5. **F5 — batched removal** (`deskwt.go`, `prune.go`). `removeWorktreeTree` split out; prune
   deletes inside the loop and runs ONE `git worktree prune` + ONE `worktreePaths` listing
   after it, demoting any path still registered from removed to a warned skip. `remove` is
   unchanged in behaviour.
6. **F6 — a read-only `--dry-run`** (`prune.go`). `git worktree prune --dry-run --verbose` for
   the bookkeeping count; the lock-reclaim pass reporting-only; no second bookkeeping prune; no
   singleton stamp written.
7. **F7 — the prune singleton** (`prunesingleton.go`, `prune.go`, `README.md`). The stamp
   under `<StateDir()>/prune/`, `deskkit.TryLockExclusive` for concurrency, the
   `--singleton-ttl` debounce (default 10m, `0` disables), `--no-singleton` for a caller that
   must sweep regardless of the TTL **and still takes the lock** (it disables the DEBOUNCE
   only — it is not a force flag and cannot bypass a live holder), the two skip messages, the
   fail-open stamp parse, and the per-tick acquisition in the `--interval` loop.
8. **Tests** — `prunesingleton_test.go`, `pruneperf_test.go` (the synthetic fixture builder,
   the N-scaling test, the five-concurrent test and `BenchmarkPruneSweepPerCandidate`), the new
   rows in `prune_test.go`, and the shared-cache row in `gitcore_test.go`.
9. **`prune-mutations.json`** — the `muhar` spec for row 11.
10. **Docs** — a `tools/desk/README.md` section covering the singleton, its two flags, the
    read-only dry run, and the statement that a held sweep exits 0.
11. **Changelog fragment** under `changelog/`.
12. **Nothing else.** No new locking primitive, no `gitcore.IsAncestor` memoization, no
    consolidation with PR #1045, no change to `remove`'s behaviour, no change to which
    worktrees are eligible for removal, and no `--force` of any spelling.

## Definition of done

- A mixed synthetic fixture (locked / dirty / fresh-at-tip / unmerged-clean / merged-clean /
  outside-prefix / current / shared) yields the IDENTICAL set of removed paths before and after
  the change.
- A sweep over a synthetic repo with a 5,000-commit `origin/main` and 600 registered worktrees
  completes in under 30 s single-process, and `origin/main` is walked exactly once.
- Five concurrent sweeps on one synthetic repo complete in about the time of one, with exactly
  one sweeping and four reporting held-or-recent; all five exit 0.
- `deskwt prune --dry-run` (including with `--reclaim-stale-locks`) leaves `.git/worktrees/`
  byte-identical on a fixture carrying both a dangling admin entry and a stale lock.
- A sweep whose `origin/main` does not resolve removes nothing and reports a sweep-level
  could-not-check, rather than removing everything or falling back to per-candidate walks.
- `deskwt remove` still prunes and verifies per path, and its tests pass unchanged.
- `gitcore.Open`'s existing callers are unchanged; `OpenWith` shares one cache across many
  worktrees of one repository.
- A benchmark pins the per-candidate cost, and `go build ./...`, `go vet ./...` and `gofmt -l`
  are clean, and `statusgen --root .. --lint` reports `LINT: PASS`.
- The brief's board row is `implemented` and the changelog fragment is present.

## Verify

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/desk && go build ./... && go vet ./...` | exit 0 — the restructured sweep, the new singleton file and the added `gitcore` entry point compile with no call-site churn beyond the files named under `files:` |
| 2 | check +mutation +flow | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneRemovalSetUnchanged$' -count=1 -timeout 300s` | exit 0 — the SPOF row, and the CROSS-COMPONENT row: a synthetic repo carrying one worktree of each class (locked, dirty, fresh-at-tip, clean-unmerged-below-tip, clean-merged-below-tip, outside-prefix, current, shared checkout) is swept, and the SET of removed paths equals the set the pre-change gate order produced, recorded as an explicit literal in the test. Mutation: inverting the ancestor-set membership test REDDENS this row by deleting the clean-unmerged worktree. It is the `+flow` row because one sweep drives BOTH components the brief's declared paths cross, end to end and in one pass: `tools/desk/cmd/deskwt/`'s reordered gate chain and batched deregistration, over `tools/desk/internal/gitcore/`'s shared-cache `OpenWith` and the single `Log` walk the merge gate answers from. Nothing is stubbed on either side of that boundary |
| 3 | check:ci | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneHoldsAll' -count=1` | exit 0 — the fail-closed row: with `refs/remotes/origin/main` deleted from the synthetic repo, a sweep over candidates that are otherwise fully removable removes ZERO, reports each as a sweep-level `unverifiable merge status`, emits one warning naming the cause, and exits 0. It must not fall back to per-candidate walking and must not proceed on an empty set |
| 4 | check:ci | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestMergeGateBeforeStatus$' -count=1` | exit 0 — the F3 row, asserted on BEHAVIOUR not on timing: a clean-unmerged worktree is held with an `unmerged` reason and its `Status()` is never computed (counted through a test seam on the tracked-clean gate, which must record 0 calls for it), while a merged-clean worktree is removed — so the reorder moved the gate without moving any hold |
| 5 | check:ci | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneSkips' -count=1 && go test ./cmd/deskwt/ -run '^TestPruneRefusesSharedCheckout$' -count=1` | exit 0 — every existing hold, including the unpushed POSITIVE and the pushed-but-unmerged NEGATIVE control that together pin what F1 must not collapse. `TestPruneSkipsDirtyTracked`'s fixture is made single-gate; its assertion is not loosened |
| 6 | check:ci | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneDryRunIsReadOnly$' -count=1` | exit 0 — the F6 row: on a fixture carrying a DANGLING admin entry and a STALE lock, `prune --dry-run --reclaim-stale-locks --lock-ttl 1s` leaves a recursive hash of `.git/worktrees/` unchanged, unlocks nothing, writes no singleton stamp, and still REPORTS both the bookkeeping entry it would drop and the lock it would reclaim |
| 7 | check:ci | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneBatch' -count=1` | exit 0 — the F5 row: a sweep removing 5 worktrees invokes `git worktree prune` exactly ONCE and `git worktree list --porcelain` a bounded number of times independent of the removal count (asserted against the recorded git argv list), AND a path that is still registered after the batched prune is DEMOTED — not counted in `removed`, reported as a warning naming it |
| 8 | check:ci | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestRemove' -count=1` | exit 0 — `remove`'s own tests, unchanged: the human-named single-path verb still deletes, prunes and positively verifies per path. The F5 split must be invisible to it |
| 9 | check:ci | `cd tools/desk && go test ./internal/gitcore/ -run '^TestOpenWithShares' -count=1 && go test ./internal/gitcore/ -run '^TestOpenStillBuilds' -count=1` | exit 0 — the F4 row: two `OpenWith` calls against two worktrees of one repository, given the same cache, both resolve and read correctly, and the cache instance they were handed is the one both used (asserted through a counting `cache.Object` wrapper); and plain `Open` still constructs its own, so no existing caller silently shares state |
| 10 | check:ci | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneSingleton' -count=1 -timeout 120s` | exit 0 — the F7 rows: (a) with the lock held, a second sweep skips with the held message, exits 0, and removes nothing; (b) a sweep whose stamp records a `finishedAt` inside the TTL skips with the recent message; (c) `--singleton-ttl 0` and `--no-singleton` sweep anyway but STILL take the lock; (d) a MISSING, truncated, non-JSON, and future-dated stamp each cause the sweep to PROCEED (fail-open), not to skip; (e) `--dry-run` writes no stamp |
| 11 | check +mutation | `cd tools/desk && report=$(go run ./cmd/muhar -spec cmd/deskwt/prune-mutations.json) && printf '%s\n' "$report" && case "$report" in *'Totals: 55 caught, 0 NOT CAUGHT, 0 could-not-mutate.'*) ;; *) exit 1;; esac` | exit 0 — baseline GREEN, positive control CAUGHT, and exactly 55 mutations CAUGHT with zero NOT CAUGHT and zero could-not-mutate: the ancestor-set membership test inverted; the failed-set build allowed to continue with an empty set; the tracked-clean gate moved back ahead of the merge gate AND its hold dropped; the batched verification's demotion removed so an unverified path counts as removed; `--dry-run` allowed to reach the real `git worktree prune`; the singleton's TTL parse made to fail CLOSED (a corrupt stamp wedging the sweep); and `TryLockExclusive`'s `ErrLockBusy` treated as free |
| 12 | check:ci | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneInterval' -count=1 -timeout 120s` | exit 0 — the supervisor releases the singleton between ticks, so a one-shot sweep between two ticks is NOT locked out; the two existing interval tests are unchanged |
| 13 | check:ci | `cd tools/desk && DESKWT_PERF_N=50 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 600s` | exit 0 — the synthetic fixture (a `git fast-import`-built 5,000-commit `origin/main`, N fabricated registered worktrees verified against `git worktree list --porcelain`) sweeps at N=50, GATES on the measured wall time against the budget the test states for N, and asserts `origin/main` was walked EXACTLY ONCE (counted through the sweep's own walk seam) |
| 14 | check:ci | `cd tools/desk && DESKWT_PERF_N=200 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 900s && DESKWT_PERF_N=600 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 900s` | exit 0 — the same row at N=200 and N=600; at N=600 the test GATES on a sweep wall under 30 s, which is #1037's stated target, and the walk count stays 1. A row that merely printed a duration would assert nothing |
| 15 | check:ci | `cd tools/desk && DESKWT_PERF_N=200 go test ./cmd/deskwt/ -run '^TestConcurrentPrunes' -count=1 -timeout 900s` | exit 0 — the F7 measurement: five sweeps launched together against one synthetic repo, each holding its own open file descriptor on the stamp; exactly ONE performs a sweep, four report held-or-recent, all five exit 0, and the total wall is gated under 1.5× a single measured sweep on the same fixture |
| 16 | check:ci | `cd tools/desk && go test -bench '^BenchmarkPruneSweepPer' -benchtime 10x -run '^$' ./cmd/deskwt/ > /tmp/dt26-bench.out 2>&1; grep -q 'BenchmarkPruneSweepPerCandidate' /tmp/dt26-bench.out` | exit 0 — the benchmark exists and runs, pinning the per-candidate cost so a later regression has a number to be compared against. It reports; row 14 is what gates |
| 16b | check:ci | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestLegacyPerCandidate' -count=1 -timeout 600s` | exit 0 — the BASELINE, on the same synthetic fixture: what the sweep used to do per candidate (a fresh open with its own object cache, the unmemoized ancestor walk, and the two ahead-count walks for the unmerged majority), timed alongside a whole current sweep. It is not a gate; it exists so the improvement is a measurement with a reproducible baseline rather than a number in a PR body |
| 17 | check:ci | `cd tools/desk && go test -timeout 600s ./cmd/deskwt/... ./internal/gitcore/... -count=1` | exit 0 — every existing test of both touched packages |
| 18 | check:ci | `cd tools/desk && gofmt -l cmd/deskwt internal/gitcore > /tmp/dt26-fmt.out; test ! -s /tmp/dt26-fmt.out` | exit 0 |
| 19 | check:ci | `cd statusgen && go run . --root .. --lint; echo $?` | 0 |
| 20 | check:ci +dereference | `o=$(statusgen --root . --consumers --brief desk-tools/26 --delivery b02ea2b35e9f74b8331478dd59490bb7a8e906dd --base 86e3a434e5f17c4260efda925404b214df48ba74); s=$?; printf '%s\n' "$o"; test "$s" -eq 0 || exit "$s"; case "$o" in *'summary: 3 corroborated, 0 disproved, 6 unchecked, 0 brief(s) claiming nothing'*) ;; *) exit 1;; esac` | exit 0; exact `summary: 3 corroborated, 0 disproved, 6 unchecked, 0 brief(s) claiming nothing` — the routing claims in `consumers:` are RESOLVED against the delivering diff: every `fixed-here` entry is touched by it and every `out-of-scope` one is not. Claims are read from the immutable delivered snapshot of b02ea2b35, the squash that delivered this brief in #1064, and compared only with its first-parent diff, so the row reproduces on any later main. The 6 UNCHECKED entries are the brief's out-of-scope routes; they remain individual review obligations and this row does not discharge their truth. Exit 2 is COULD-NOT-CHECK (missing history or no diff to take) and is reported AS ITSELF, never as a pass |

Pre-mortem → detection map:

| Failure mode of the work | Caught by |
|---|---|
| The ancestor set is built from the wrong ref, or from a stray local `origin/main`, and unmerged worktrees read as merged — a sweep that deletes live work and looks successful | row 2 (set equality over a mixed fixture, with a clean-unmerged worktree in it) + row 11's inversion mutation |
| The set cannot be built and the sweep continues with an empty one, or silently falls back to per-candidate walks | row 3 + row 11's empty-set mutation |
| The gate reorder drops a hold instead of moving it — `Status()` is skipped AND the dirty worktree is removed | rows 2 and 5 + row 11's moved-gate mutation, which also drops the hold |
| `Status()` still runs for every candidate, so the ~25 % is not actually saved | row 4 (the call is counted, not timed) |
| F1 collapses the unpushed/unmerged distinction, or invents a direction the comparison does not prove | row 5 (the positive AND the negative control, both pre-existing) |
| Batched removal loses the positive deregistration check, so a path that is still registered counts as removed | row 7 (the demotion is asserted) + row 11's demotion mutation |
| The batch tail leaves `remove` prune-less or verify-less | row 8 |
| `--dry-run` still mutates — the bookkeeping prune, an unlock, or the singleton stamp | row 6 + row 11's dry-run mutation |
| A leftover stamp wedges prune on every window — the stale-lock class, recreated | row 10(d), which asserts four kinds of bad stamp all PROCEED, + row 11's fail-closed-TTL mutation |
| The lock is treated as free when it cannot be taken, so five windows sweep anyway | rows 10(a) and 15 + row 11's `ErrLockBusy` mutation |
| The `--interval` supervisor holds the lock for its lifetime and locks out every other sweep on the machine | row 12 |
| The shared object cache is not actually shared, so F4 saves nothing | row 9 (the counting cache wrapper) |
| `Open`'s existing callers silently start sharing state | row 9's second test |
| It is fast on a toy repo and still slow at scale — the walk was hoisted but something else stayed per-candidate | rows 13 and 14 (walk count asserted EXACTLY 1 at N=600, and the wall gated) |
| The fabricated synthetic worktrees are not faithful, so the measurement measures nothing | row 13 (the fixture asserts `git worktree list --porcelain` reports exactly N before it measures) |
| A measurement row is run against a real checkout and removes somebody's work | Ground rules (synthetic, `t.TempDir()`-owned fixtures only) — a diff-shape check at review |
| The per-candidate cost regresses later with no number to compare against | row 16 |

## Evidence
<!-- appended at implementation time: one witness row per Verify row —
     (command, exit code, output line(s), date, runner). -->

| # | Exit | Key observed output |
|---|------|---------------------|
| — | — | not yet run — this brief is authored, not implemented |
### Non-implementer verifier run — 2026-09-17 sonnet-5-verifier (verify-desk dispatch) — **VERIFY: PARTIAL** — HELD at `implemented`

Runner ≠ implementer. Brief's own Evidence section was never filled in despite `implemented` status — nothing self-reported to void. Deliverable squash-merge `b02ea2b35` (PR #1064) confirmed an ancestor of `origin/main` `e5f2b89dbab1e4be255fe15ca73ba00dc8764995`.

Methodology note: an initial concurrent-execution mistake (running the row-11 mutation harness in the background while running other rows in the foreground against the same mutable worktree) produced 3 spurious transient FAILs; re-run sequentially after the harness finished (tree confirmed clean) and got consistent, reproducible results below.

| # | Command | Expect | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | `go build ./... && go vet ./...` | exit 0 | exit 0 | 2026-09-17 | sonnet-5-verifier |
| 2 | `TestPruneRemovalSetUnchanged` | exit 0, removal set identical | exit 0 — literal expected set `{mergedClean}` asserted, only that path removed | 2026-09-17 | sonnet-5-verifier |
| 3 | `TestPruneHoldsAll*` | exit 0, fail-closed | exit 0 both subtests — 0 removed, could-not-check/unverifiable correctly reported | 2026-09-17 | sonnet-5-verifier |
| 4 | `TestMergeGateBeforeStatus` | exit 0 | exit 0 — `Status()` called 0 times for the merge-held candidate, 1 for the merged one | 2026-09-17 | sonnet-5-verifier |
| 5 | `TestPruneSkips*` | exit 0, positive+negative controls | exit 0 both | 2026-09-17 | sonnet-5-verifier |
| 6 | `TestPruneDryRunIsReadOnly` | exit 0, byte-identical tree | exit 0 — hash before/after equal, lock not retired, both findings still reported | 2026-09-17 | sonnet-5-verifier |
| 7 | `TestPruneBatch*` | exit 0 | exit 0 both subtests | 2026-09-17 | sonnet-5-verifier |
| 8 | `TestRemove*` | exit 0 | exit 0, all 7 subtests | 2026-09-17 | sonnet-5-verifier |
| 9 | `TestOpenWithShares`/`TestOpenStillBuilds` | exit 0 | exit 0 both — the literal un-anchored patterns `-run '^TestOpenWithShares'`/`-run '^TestOpenStillBuilds'` (no trailing `$` in the table) prefix-match `TestOpenWithSharesCache`/`TestOpenStillBuildsItsOwnCache` and genuinely PASS AS WRITTEN (corrected 2026-09-18 — see correction note below) | 2026-09-17 | sonnet-5-verifier |
| 10 | `TestPruneSingleton*` | exit 0, (a)-(e) | exit 0 — 5 subtests (held/recent/ttl-zero/4 bad-stamp shapes/missing-stamp) | 2026-09-17 | sonnet-5-verifier |
| 11 | `muhar -spec prune-mutations.json` | baseline GREEN, all mutations CAUGHT | exit 0 — **10/10 caught** (1 control + 9 mutations incl. ancestor-set inversion, dry-run-reaches-real-prune, TTL fail-closed, lock fail-open) | 2026-09-17 | sonnet-5-verifier |
| 12 | `TestPruneInterval*` | exit 0 | exit 0, 3 subtests | 2026-09-17 | sonnet-5-verifier |
| 13 | `DESKWT_PERF_N=50` scaling test | exit 0, walk==1 | exit 0 — N=50, sweep=444ms, walks=1 | 2026-09-17 | sonnet-5-verifier |
| 14 | N=200/600 scaling | exit 0, N=600 <30s walk==1 | exit 0 both — N=600 sweep=2.12s, well under budget | 2026-09-17 | sonnet-5-verifier |
| 15 | `TestConcurrentPrunes` | exit 0 | exit 0 — the literal un-anchored pattern `-run '^TestConcurrentPrunes'` prefix-matches `TestConcurrentPrunesCostOne` and genuinely PASSes AS WRITTEN, ratio 1.37x < 1.5x gate (corrected 2026-09-18 — see correction note below) | 2026-09-17 | sonnet-5-verifier |
| 16 | benchmark | exit 0, exists | exit 0 — 10.63ms/candidate | 2026-09-17 | sonnet-5-verifier |
| 16b | `TestLegacyPerCandidate` | exit 0 | exit 0 — the literal un-anchored pattern `-run '^TestLegacyPerCandidate'` prefix-matches `TestLegacyPerCandidateWalkCost` and genuinely PASSes AS WRITTEN, legacy 334.99ms/candidate vs whole-sweep 381ms (corrected 2026-09-18 — see correction note below) | 2026-09-17 | sonnet-5-verifier |
| 17 | full package test | exit 0 | exit 0 on clean sequential re-run (both packages) | 2026-09-17 | sonnet-5-verifier |
| 18 | `gofmt -l` | exit 0 | exit 0, empty | 2026-09-17 | sonnet-5-verifier |
| 19 | `statusgen --lint` | exit 0 | exit 0, LINT: PASS | 2026-09-17 | sonnet-5-verifier |
| 20 | `statusgen --consumers` via `go run` | exit 0 or 2 could-not-check | substance correct (genuine COULD-NOT-CHECK reported), but the literal `echo $?` shows 1 not 2 — `go run` never propagates a wrapped program's real exit code (independently verified with a trivial `os.Exit(2)` test program); a pre-existing Go-tooling property, not a desk-tools defect, but the row's literal assertion is unreachable via `go run` as written | 2026-09-17 | sonnet-5-verifier |

**Correction (2026-09-18, per pr-review-desk request-changes on PR #1300):** the text originally here (and at rows 9, 15, 16b and the closing VERIFY line below) claimed the Verify table's `-run` anchors for rows 9, 15, 16b were `-run '^TestX$'`, matched no test in the shipped code, and silently false-passed via "no tests to run" / exit 0. Both halves of that claim were wrong: the table's actual patterns carry no trailing `$`, and Go's `-run` match is an unanchored-at-the-end regex, so the un-`$`-terminated `^TestOpenWithShares`/`^TestOpenStillBuilds`/`^TestConcurrentPrunes`/`^TestLegacyPerCandidate` prefixes directly match `TestOpenWithSharesCache`, `TestOpenStillBuildsItsOwnCache`, `TestConcurrentPrunesCostOne`, and `TestLegacyPerCandidateWalkCost` respectively. Independently reran all four literal commands against this PR's head and confirmed each runs and passes exactly as written. There is no Verify-table defect in rows 9, 15, or 16b, and no fixing-PR is needed against them.

**RISK-VALUE: DERIVED** — the ancestor-set correctness (`mainAncestors` from `refs/remotes/origin/main`, per issue #885's disambiguation) is the SPOF this brief names; independently verified via row 2 (real removal-set equality), row 3 (fail-closed on missing ref), and row 11's control mutation (ancestor-set inversion caught). `defaultSingletonTTL=10min` (`prunesingleton.go:57`) — any malformed/missing/future stamp treated as "no stamp" (fail-open to a delayed sweep, never data loss), confirmed by row 10(d) and row 11's TTL-fail-closed mutation. Lock file mode `0600`/dir `0700` — cosmetic, state lives outside the target repo. The tip-guard/ancestor-set independence design (a documented "DO NOT hoist" comment against correlated-failure of two gates) confirmed present exactly as described.

**Invented-scope note (benign):** `lockreclaim.go` modified but not in the brief's `files:` list — legitimate, required for deliverable F6 (dry-run must not retire locks), not scope creep, just a bookkeeping omission. Two test files shipped as new rather than edits to existing ones (functionally equivalent). `deskwt_test.go` helper signatures generalized to `testing.TB` (benign, needed for benchmark reuse).

**VERIFY: PARTIAL** — all safety-critical rows (2,3,4,6,7,10,11,12,17) — the ones guarding the worst-case failure mode of a wrong prune/reclaim rule deleting live work — pass genuinely and rigorously, including a 10/10 mutation-catch rate. Rows 9, 15, 16b genuinely PASS as literally written (the earlier "false-pass" claim was wrong — retracted; see correction above) and carry no Verify-table defect. Row 20's exit-code assertion remains unreachable via `go run` (a pre-existing Go-tooling property, not a desk-tools defect) — that is the sole remaining open note. The implementation is sound. Held at `implemented` pending a verify-desk re-adjudication of the overall verdict in light of this correction.

### Non-implementer verifier run — VERIFY: BLOCKED — 2/21 pass, 19 could-not-check, 0 fail — 2026-09-23 claude-opus-4-8-verifier

Runner is a NON-implementer, dispatched against merged main
7e8e79ed43f30fcf43dc2fdbf735606cce2a8323. Deliverable landed by PR #1064 (squash
b02ea2b35), an ancestor of origin/main. Every row re-run fresh from the verifier's own
worktree. Go 1.26.5 darwin/arm64. Every row the brief's `## Verify` table classes `check:ci` (rows 1, 3-10,
12-19 and 16b/20) is COULD-NOT-CHECK on this darwin desk: the hermetic `statusgen verifyrun`
witness needs a Linux `unshare --net` sandbox (#1491), so each go-test row's direct run is recorded
supporting-only, never as a pass. Only rows 2 (`check +mutation +flow`) and 11 (`check +mutation`,
the muhar harness) are plain `check` rows that pass offline. Go test rows carry -v and the
--- PASS / --- SKIP lines were read (no row passed on "no tests to run" or a skip). This pass supersedes the
2026-09-17 PARTIAL: rows 9, 15, 16b run and pass as literally written (the earlier
false-pass claim was already retracted 2026-09-18), and row 20's exit-code note is a
fully-merged-tree could-not-check, not a defect.

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | exit 0 | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; builds and vets clean, no call-site churn | 2026-09-23 | claude-opus-4-8-verifier |
| 2 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneRemovalSetUnchanged$' -count=1 -v -timeout 300s` | exit 0, removed SET == recorded literal over the mixed fixture | PASS — exit 0; --- PASS TestPruneRemovalSetUnchanged (1.21s); mixed-class fixture swept, removal set matches the recorded literal | 2026-09-23 | claude-opus-4-8-verifier |
| 3 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneHoldsAll' -count=1 -v` | exit 0, fail-closed: 0 removed, sweep-level unverifiable, 1 warning | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; both --- PASS (the no-ancestor-set and the mainline-walk-fails subtests); both hold all, remove 0 | 2026-09-23 | claude-opus-4-8-verifier |
| 4 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestMergeGateBeforeStatus$' -count=1 -v` | exit 0, Status() 0 calls for the merge-held candidate, merged-clean removed | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; --- PASS TestMergeGateBeforeStatus (0.58s) | 2026-09-23 | claude-opus-4-8-verifier |
| 5 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneSkips' -count=1 -v && go test ./cmd/deskwt/ -run '^TestPruneRefusesSharedCheckout$' -count=1 -v` | exit 0, every existing hold incl. unpushed POSITIVE + pushed-but-unmerged NEGATIVE control | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; 7 skip subtests --- PASS incl. the unpushed-commit (positive) and clean-unmerged-branch (negative) subtests; TestPruneRefusesSharedCheckout --- PASS | 2026-09-23 | claude-opus-4-8-verifier |
| 6 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneDryRunIsReadOnly$' -count=1 -v` | exit 0, tree byte-identical, unlocks nothing, no stamp, still reports | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; --- PASS TestPruneDryRunIsReadOnly (0.69s) | 2026-09-23 | claude-opus-4-8-verifier |
| 7 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneBatch' -count=1 -v` | exit 0, ONE prune + bounded listings; stranded path DEMOTED not counted | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; --- PASS for the batches-removal subtest and the batch-demotes-stranded-path subtest | 2026-09-23 | claude-opus-4-8-verifier |
| 8 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestRemove' -count=1 -v` | exit 0, remove still deletes/prunes/verifies per path, split invisible to it | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; 11 TestRemove subtests --- PASS (incl. the no-force-flag, unpushed-commits-refuses and detached-head cases) | 2026-09-23 | claude-opus-4-8-verifier |
| 9 | `cd tools/desk && go test ./internal/gitcore/ -run '^TestOpenWithShares' -count=1 -v && go test ./internal/gitcore/ -run '^TestOpenStillBuilds' -count=1 -v` | exit 0, shared cache is the handed-in instance; plain Open builds its own | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; unanchored patterns prefix-match: --- PASS TestOpenWithSharesCache + --- PASS TestOpenStillBuildsItsOwnCache | 2026-09-23 | claude-opus-4-8-verifier |
| 10 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneSingleton' -count=1 -v -timeout 120s` | exit 0, (a) held skips (b) recent skips (c) ttl-0/no-singleton still take lock (d) 4 bad stamps PROCEED (e) missing stamp proceeds | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; 5 subtests --- PASS (the held-by-a-live-sweep, recent-sweep, ttl-zero-and-no-singleton, bad-stamp-always-sweeps and missing-stamp-sweeps subtests) | 2026-09-23 | claude-opus-4-8-verifier |
| 11 | `cd tools/desk && go run ./cmd/muhar -spec cmd/deskwt/prune-mutations.json` | exit 0 — baseline GREEN, positive control CAUGHT, every mutation CAUGHT | PASS — exit 0; "Harness healthy: baseline GREEN, positive control CAUGHT"; Totals: 16 caught, 0 NOT CAUGHT, 0 could-not-mutate. Caught incl. ancestor-set inversion, empty-set-continue, moved-gate-drops-hold, demotion-removed, dry-run-reaches-real-prune, dry-run-unlocks, TTL-fail-closed, future-finishedAt, lock-fail-open (ErrLockBusy read as free), supervisor-holds-for-lifetime, walk-back-in-loop, and 5 reap-arm mutations | 2026-09-23 | claude-opus-4-8-verifier |
| 12 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneInterval' -count=1 -v -timeout 120s` | exit 0, supervisor releases the lock between ticks; existing interval tests unchanged | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; 3 subtests --- PASS (the single-tick-then-stop, halts-on-stop-flag and releases-lock-per-tick subtests) | 2026-09-23 | claude-opus-4-8-verifier |
| 13 | `cd tools/desk && DESKWT_PERF_N=50 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -v -timeout 600s` | exit 0, walk count == 1, wall within budget | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; N=50 commits=5000 sweep=435ms walks=1 removed=0 held=51 | 2026-09-23 | claude-opus-4-8-verifier |
| 14 | `cd tools/desk && DESKWT_PERF_N=200 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -v -timeout 900s && DESKWT_PERF_N=600 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -v -timeout 900s` | exit 0, N=600 sweep < 30s, walk count == 1 | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0 both; N=200 sweep=852ms walks=1; N=600 sweep=2.128s walks=1 (well under 30s) | 2026-09-23 | claude-opus-4-8-verifier |
| 15 | `cd tools/desk && DESKWT_PERF_N=200 go test ./cmd/deskwt/ -run '^TestConcurrentPrunes' -count=1 -v -timeout 900s` | exit 0, exactly 1 sweeps + 4 held, all exit 0, total wall < 1.5x a single sweep | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; unanchored pattern matches the concurrent-prunes-cost-one test; single=827ms five-concurrent=1.166s (ratio ~1.41x < 1.5x), 1 swept, 4 held | 2026-09-23 | claude-opus-4-8-verifier |
| 16 | `cd tools/desk && go test -bench '^BenchmarkPruneSweepPer' -benchtime 10x -run '^$' ./cmd/deskwt/ > /tmp/dt26-bench.out 2>&1; grep -q 'BenchmarkPruneSweepPerCandidate' /tmp/dt26-bench.out` | exit 0, benchmark exists and runs | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; BenchmarkPruneSweepPerCandidate-16, 514909117 ns/op, 10.30 ms/candidate | 2026-09-23 | claude-opus-4-8-verifier |
| 16b | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestLegacyPerCandidate' -count=1 -v -timeout 600s` | exit 0, baseline measured alongside a whole current sweep | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; unanchored pattern matches the legacy-per-candidate-walk-cost test; legacy=6.311s (315.564ms/candidate) vs whole current sweep=399ms | 2026-09-23 | claude-opus-4-8-verifier |
| 17 | `cd tools/desk && go test -timeout 600s ./cmd/deskwt/... ./internal/gitcore/... -count=1` | exit 0, every existing test of both touched packages | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; ok cmd/deskwt 66.647s; ok internal/gitcore 24.301s | 2026-09-23 | claude-opus-4-8-verifier |
| 18 | `cd tools/desk && gofmt -l cmd/deskwt internal/gitcore > /tmp/dt26-fmt.out; test ! -s /tmp/dt26-fmt.out` | exit 0 | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; gofmt list empty | 2026-09-23 | claude-opus-4-8-verifier |
| 19 | `cd statusgen && go run . --root .. --lint; echo $?` | 0 | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; LINT: PASS; printed 0 | 2026-09-23 | claude-opus-4-8-verifier |
| 20 | `cd statusgen && go run . --root .. --consumers --brief assay:assay:desk-tools:26; echo $?` | 0 — routing claims RESOLVED against this branch's diff; exit 2 = could-not-check on a fully merged tree, reported as itself | COULD-NOT-CHECK — printed 0 but decides nothing: on this fully-merged detached-at-origin/main tree there is NO branch diff, so all 9 consumer entries report UNCHECKED (0 corroborated / 0 disproved / 9 unchecked). The row's own "fully merged tree" carve-out, reported as itself, NOT a pass and NOT a fail | 2026-09-23 | claude-opus-4-8-verifier |

RISK-VALUE lines (kit §4 — enumerate → rank → derive; every entry a literal at file:line):

RISK-VALUE: DERIVED — the merge-gate authority binding, base = "refs/remotes/origin/main" @ tools/desk/cmd/deskwt/prune.go:484 (logOriginMain; also the Resolve at prune.go:991) — this is the item's single-point-of-failure. It is right because IsAncestor(HEAD, origin/main) is TRUE exactly when HEAD is in the set of commits reachable from origin/main, and Log returns that exact set (including the tip); spelling it in FULL (not the bare "origin/main") is load-bearing because git resolves refs/heads/ ahead of refs/remotes/, so a stray local branch literally named origin/main would silently become the baseline and let unmerged worktrees read as merged — the exact deletes-live-work failure (issue #885). Independently corroborated by row 2 (removal-set equality over a fixture containing a clean-unmerged worktree), row 3 (fail-closed when the ref is absent), and row 11's ancestor-set-inversion mutation being CAUGHT.

RISK-VALUE: DERIVED (reversible, ranks last, no further derivation owed) — defaultSingletonTTL = 10 * time.Minute @ tools/desk/cmd/deskwt/prunesingleton.go:57. What breaks if wrong: a boot-time convenience sweep is delayed or duplicated by at most one TTL. Fully reversible by an edit-and-redeploy, and the mechanism fails OPEN (a bad/missing stamp proceeds), so a wrong value can never delete work or wedge a window — it is an efficiency knob by construction. Confirmed by row 10(d) (bad stamps proceed) and row 11's TTL-fail-closed mutation being CAUGHT.

RISK-VALUE: DERIVED (fail-safe, ranks last) — stamp file mode 0o600 @ prunesingleton.go:182, stamp dir mode 0o700 @ prunesingleton.go:177; lock-ttl flag default "0" @ prune.go:141 (disabled unless --reclaim-stale-locks/--reap-dead-sessions). All restrictive/disabled-by-default; the stamp lives under StateDir()/prune, outside the target repository's .git, so none of them can affect a removal or the read-only dry-run guarantee.

The check:ci go-test rows are all could-not-check (Linux hermetic witness owed, #1491) with their direct --- PASS lines recorded supporting-only; the two clean passes are row 2 and row 11 (mutation harness, caught 16/16, 0 not caught); row 20 (consumers) is the merged-tree carve-out.

### Non-implementer verifier run — hermetic witness — VERIFY: BLOCKED — 19/21 pass, 2 witness fails, neither an implementation defect — 2026-09-27 claude-opus-5-5-verifier

Runner is a NON-implementer, dispatched against merged main 874d56de38a73228dad62ed25482d2af55692849 (deliverable squash b02ea2b35, PR #1064, an ancestor). What changed since the 2026-09-23 BLOCKED pass: that pass had no Linux network-off sandbox, so every check:ci row was could-not-check. This pass supplies one — `statusgen verifyrun` (built from this same main commit) run inside a Linux container (Go 1.25.14, the go.mod floor; network disabled at the container level, module cache mounted read-only with GOPROXY=off) where `unshare --net --map-root-user` works, so every check:ci row was re-executed network-off by the tool itself. The table below is that witness, written non-dry at a clean tree (stamp carries no +dirty), with the roster configuration available to the tool so each row carries its on-behalf-of annotation. No prune-path source changed since the prior pass: the one later diff in the touched files (deskwt.go, the `add --role` transport, PR #1634) is outside the prune sweep.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneRemovalSetUnchanged$' -count=1 -timeout 300s` | pass exit=0 | sha256:bbf5b956cace | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneHoldsAll' -count=1` | pass exit=0 | sha256:463aefafb81c | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestMergeGateBeforeStatus$' -count=1` | pass exit=0 | sha256:f58572f9817f | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneSkips' -count=1 && go test ./cmd/deskwt/ -run '^TestPruneRefusesSharedCheckout$' -count=1` | pass exit=0 | sha256:633995271f7d | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneDryRunIsReadOnly$' -count=1` | pass exit=0 | sha256:721298980f2e | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneBatch' -count=1` | pass exit=0 | sha256:7c61ac42ed29 | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestRemove' -count=1` | pass exit=0 | sha256:4b29fd1955e7 | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd tools/desk && go test ./internal/gitcore/ -run '^TestOpenWithShares' -count=1 && go test ./internal/gitcore/ -run '^TestOpenStillBuilds' -count=1` | pass exit=0 | sha256:33855308a78d | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneSingleton' -count=1 -timeout 120s` | pass exit=0 | sha256:cd8a3cb7bc42 | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 11 | `cd tools/desk && go run ./cmd/muhar -spec cmd/deskwt/prune-mutations.json` | pass exit=0 | sha256:8af0b5c75032 | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 12 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneInterval' -count=1 -timeout 120s` | pass exit=0 | sha256:3ba261aea614 | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 13 | `cd tools/desk && DESKWT_PERF_N=50 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 600s` | pass exit=0 | sha256:c9337b26adf2 | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 14 | `cd tools/desk && DESKWT_PERF_N=200 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 900s && DESKWT_PERF_N=600 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 900s` | pass exit=0 | sha256:6f0c49af49e9 | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 15 | `cd tools/desk && DESKWT_PERF_N=200 go test ./cmd/deskwt/ -run '^TestConcurrentPrunes' -count=1 -timeout 900s` | pass exit=0 | sha256:1924d5cb7df2 | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 16 | `cd tools/desk && go test -bench '^BenchmarkPruneSweepPer' -benchtime 10x -run '^$' ./cmd/deskwt/ > /tmp/dt26-bench.out 2>&1; grep -q 'BenchmarkPruneSweepPerCandidate' /tmp/dt26-bench.out` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 16b | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestLegacyPerCandidate' -count=1 -timeout 600s` | pass exit=0 | sha256:c451445d452f | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 17 | `cd tools/desk && go test -timeout 600s ./cmd/deskwt/... ./internal/gitcore/... -count=1` | fail exit=1 | sha256:38e5135dd0e0 | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 18 | `cd tools/desk && gofmt -l cmd/deskwt internal/gitcore > /tmp/dt26-fmt.out; test ! -s /tmp/dt26-fmt.out` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 19 | `cd statusgen && go run . --root .. --lint; echo $?` | pass exit=0 | sha256:c99399159acd | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 20 | `cd statusgen && go run . --root .. --consumers --brief assay:assay:desk-tools:26; echo $?` | fail exit=0 | sha256:e1a03c81798e | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |

Key real output behind the witness (same container, same commit, network-off, -v re-runs for the lines):

- Row 2: --- PASS TestPruneRemovalSetUnchanged — the mixed-class fixture's removed set equals the recorded literal.
- Row 3: --- PASS TestPruneHoldsAllNoAncestorSet and --- PASS TestPruneHoldsAllWhenTheMainlineWalkFails — zero removed, sweep-level could-not-check warning.
- Row 4: --- PASS TestMergeGateBeforeStatus.
- Row 11: "Harness healthy: baseline GREEN, positive control CAUGHT." then "Totals: 16 caught, 0 NOT CAUGHT, 0 could-not-mutate." — every mutation the row names is among the 16 (ancestor-set inversion, empty-set continue, moved gate drops the hold, demotion removed, dry-run reaches the real prune, TTL fail-closed, future finishedAt, ErrLockBusy read as free, supervisor holds for its lifetime, walk back in the loop, plus the reap-arm set). The tree was clean after the harness restored it.
- Row 14: N=600 commits=5000 sweep=1.017s walks=1 removed=0 held=601 (gate is 30 s; walk count exactly 1).
- Row 15: N=200 single=387ms five-concurrent=372ms (1 swept, 4 held); inside the 1.5x gate.
- Row 17 FAIL (exit 1): gitcore ok; the deskwt package fails on exactly three tests — TestAddRoleWiresAppTransportOverSSHOriginAndSentinel, TestAddRoleResolvesSSHHostAlias, TestAddRoleReplacesInheritedSSHPushUnderBotLoop — each with the refusal text "an empty entry resets the list only on git 2.46 or later". The container's git is 2.39.5. These tests belong to the `deskwt add --role` transport work (PR #1634, landed after this brief), not to anything this brief ships; no prune, singleton, batch or gitcore test failed. The same row command run directly on a host with git 2.55.0 exits 0 (ok deskwt, ok gitcore). Assessment: ENVIRONMENT-shaped for this brief (the hermetic runner's git is older than the floor those tests silently assume), and separately a latent test-portability gap in the add --role tests, which assume git 2.46 or later without a version skip. That gap is a finding for the add --role work's owner, not a desk-tools/26 defect.
- Row 19: LINT exit 0.
- Row 20 FAIL as witnessed (exit 0, the tool read "exit 2" from the Expect cell): on this fully merged tree there is no branch diff, so the consumers gate prints "summary: 0 corroborated, 0 disproved, 9 unchecked" and exits 0; the trailing echo also forces the row's own exit to 0. This is the row's own merged-tree carve-out — could-not-check, reported as itself, and it can never reach a pass or fail on merged main as authored. Assessment: CHECK-DEFINITION (stale-shaped: the row can only decide on the implementing branch, which is gone). Independent spot check instead: the implementing diff touches all three fixed-here files (prune.go, deskwt.go, gitcore.go) and none of the out-of-scope sites (no deskkit lock file, no deskboot, no contains.go; AheadCount, IsAncestor and RefsContaining are unchanged in gitcore.go — the only gitcore hunk adds NewObjectCache and OpenWith and re-points Open).

Risk-bearing value — enumeration over the non-test files of squash b02ea2b35 (prune.go, prunesingleton.go, deskwt.go, lockreclaim.go, gitcore.go), read at main 874d56de3. Literals found:

- merge-gate baseline ref = "refs/remotes/origin/main" @ tools/desk/cmd/deskwt/prune.go:484 (the one walk), also @ prune.go:991 (the tip guard's resolve)
- defaultSingletonTTL = 10 * time.Minute @ tools/desk/cmd/deskwt/prunesingleton.go:57
- pruneStampSchema = "deskwt-prune-singleton-v1" @ prunesingleton.go:51 (an unknown schema reads as "no stamp", fail-open)
- stamp dir mode 0o700 @ prunesingleton.go:177; stamp file mode 0o600 @ prunesingleton.go:182
- lock-ttl flag default "0" @ prune.go:141 (pre-existing flag; disabled by default)

Ranked by irreversibility: the baseline ref is the only entry whose error deletes work (an unmerged worktree reading as merged, removed irreversibly); the rest are reversible efficiency or hygiene knobs.

RISK-VALUE: DERIVED — merge-gate baseline = "refs/remotes/origin/main" @ tools/desk/cmd/deskwt/prune.go:484 — IsAncestor(HEAD, origin/main) is true exactly when HEAD is in the set of commits reachable from origin/main; gitcore Log walks go-git's default preorder over ALL parents from the resolved tip (not first-parent), so the set is the full reachable set including the tip, and set membership is therefore equal to the ancestor test it replaces. Spelling the ref in full is required because git resolves refs/heads/ before refs/remotes/, so a stray local branch named origin/main would otherwise become the baseline (issue #885). A failed walk leaves the set nil and every candidate unverifiable (prune.go:513 and prune.go:596); a partial set can only shrink membership, which holds rather than removes. Corroborated by rows 2 and 3 and row 11's inversion and empty-set mutations, both CAUGHT.

RISK-VALUE: DERIVED (reversible, ranks last) — defaultSingletonTTL = 10 * time.Minute @ tools/desk/cmd/deskwt/prunesingleton.go:57 — a debounce only: a wrong value delays or duplicates a sweep by at most one TTL, never removes anything, and every bad, missing, future-dated or unknown-schema stamp proceeds (row 10, row 11's fail-closed-TTL and future-finishedAt mutations CAUGHT). Undone by an edit and a redeploy.

RISK-VALUE: DERIVED (reversible, ranks last) — pruneStampSchema = "deskwt-prune-singleton-v1" @ prunesingleton.go:51, stamp modes 0o700/0o600 @ prunesingleton.go:177/182, lock-ttl default "0" @ prune.go:141 — all fail open or disabled by default; the stamp lives under the desk state directory, outside any repository, so none can affect a removal.

Observation: a singleton that cannot be taken for a reason other than a busy lock (state directory unresolvable or uncreatable) sweeps anyway with a warning (prune.go:410-411). That matches the brief's stated design — the singleton deduplicates work and never gates it, and every removal gate sits in the sweep itself — so it is noted, not raised.

VERIFY: BLOCKED — every row that bears on what this brief ships passes under the hermetic witness (19 of 21, including the SPOF row 2, the fail-closed row 3 and 16/16 mutations CAUGHT). The two witness fails are not implementation defects: row 17 is environment-shaped (three add --role transport tests from later work need git 2.46 or later; the row passes on a newer git), and row 20 is check-definition (a merged-tree could-not-check by the row's own carve-out). Held at implemented pending a desk ruling on those two rows.

### Non-implementer verifier re-run: 2026-10-02T22:28:52Z (UTC), assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian), merged main cf31c32418ba49f93c679913813768542db1c072

Runner is a NON-implementer. Deliverable squash b02ea2b35 (PR #1064) is an ancestor of the commit under test. Every row was run by hand, sequentially, from a detached worktree at that commit, on darwin/arm64 with go1.27.1 and git 2.56.0. Go commands ran under a throwaway HOME with the test temp directory pointed at the runner's own scratch directory. Rows 16 and 18 write an output file to the system temp directory as authored; for the hand run that one file path was redirected into the runner's scratch directory, the command otherwise unchanged. Row 11 mutates and restores the source tree; the tree was clean after it.

What changed since the 2026-09-27 BLOCKED pass: of the inputs that pass declared, the desk README, deskwt.go (#1930, #1995), gitcore.go and its test file (#1919, which adds CommitMessage) changed; prune.go, prunesingleton.go, the prune tests and the mutation spec are byte-identical. None of the changes is in the prune sweep.

| # | Command | Expect | Observed (exit + key output line) | Date / runner |
|---|---------|--------|-----------------------------------|---------------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | exit 0 | exit 0; no output (builds and vets clean) | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneRemovalSetUnchanged$' -count=1 -timeout 300s` | exit 0, removed set equals the recorded literal | exit 0; ok cmd/deskwt 2.673s; verbose re-run: --- PASS: TestPruneRemovalSetUnchanged (1.31s) | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneHoldsAll' -count=1` | exit 0, fail-closed: zero removed | exit 0; ok cmd/deskwt 1.712s; verbose re-run: --- PASS: TestPruneHoldsAllNoAncestorSet and --- PASS: TestPruneHoldsAllWhenTheMainlineWalkFails | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestMergeGateBeforeStatus$' -count=1` | exit 0 | exit 0; ok cmd/deskwt 0.957s; verbose re-run: --- PASS: TestMergeGateBeforeStatus (0.82s) | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneSkips' -count=1 && go test ./cmd/deskwt/ -run '^TestPruneRefusesSharedCheckout$' -count=1` | exit 0, every existing hold | exit 0; ok cmd/deskwt 4.738s then ok 0.416s; verbose re-run: 7 TestPruneSkips tests --- PASS (incl. TestPruneSkipsUnpushedCommit and TestPruneSkipsCleanUnmergedBranch), --- PASS: TestPruneRefusesSharedCheckout | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneDryRunIsReadOnly$' -count=1` | exit 0 | exit 0; ok cmd/deskwt 1.282s; verbose re-run: --- PASS: TestPruneDryRunIsReadOnly (1.07s) | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneBatch' -count=1` | exit 0 | exit 0; ok cmd/deskwt 1.899s; verbose re-run: --- PASS: TestPruneBatchesRemoval and --- PASS: TestPruneBatchDemotesStrandedPath | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestRemove' -count=1` | exit 0 | exit 0; ok cmd/deskwt 3.863s; verbose re-run: 11 TestRemove tests --- PASS | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 9 | `cd tools/desk && go test ./internal/gitcore/ -run '^TestOpenWithShares' -count=1 && go test ./internal/gitcore/ -run '^TestOpenStillBuilds' -count=1` | exit 0 | exit 0; ok internal/gitcore 0.472s then ok 0.291s; verbose re-run: --- PASS: TestOpenWithSharesCache and --- PASS: TestOpenStillBuildsItsOwnCache | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 10 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneSingleton' -count=1 -timeout 120s` | exit 0 | exit 0; ok cmd/deskwt 13.754s; verbose re-run: 5 TestPruneSingleton tests --- PASS (held-by-live-sweep, recent-sweep, TTL-zero-and-no-singleton, bad-stamp, missing-stamp) | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 11 | `cd tools/desk && go run ./cmd/muhar -spec cmd/deskwt/prune-mutations.json` | exit 0, baseline GREEN, control CAUGHT, every mutation CAUGHT | exit 0 (1143s); "Harness healthy: baseline GREEN, positive control CAUGHT." and "Totals: 16 caught, 0 NOT CAUGHT, 0 could-not-mutate." | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 12 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneInterval' -count=1 -timeout 120s` | exit 0 | exit 0; ok cmd/deskwt 6.028s; verbose re-run: 3 TestPruneInterval tests --- PASS incl. TestPruneIntervalReleasesLockPerTick | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 13 | `cd tools/desk && DESKWT_PERF_N=50 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 600s` | exit 0, walk count 1, wall inside budget | exit 0; ok cmd/deskwt 4.495s | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 14 | `cd tools/desk && DESKWT_PERF_N=200 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 900s && DESKWT_PERF_N=600 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 900s` | exit 0, N=600 sweep under 30 s, walk count 1 | exit 0; ok 9.305s then ok 29.066s (package wall incl. fixture build); verbose N=600 re-run: "N=600 commits=5000 sweep=3.915s walks=1 removed=0 held=601" | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 15 | `cd tools/desk && DESKWT_PERF_N=200 go test ./cmd/deskwt/ -run '^TestConcurrentPrunes' -count=1 -timeout 900s` | exit 0, one sweeps, four held, total wall under 1.5x a single sweep | exit 0; ok cmd/deskwt 9.936s; verbose re-run: "N=200 single=1.091s five-concurrent=1.019s (1 swept, 4 held)", --- PASS: TestConcurrentPrunesCostOne (ratio 0.93x) | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 16 | `cd tools/desk && go test -bench '^BenchmarkPruneSweepPer' -benchtime 10x -run '^$' ./cmd/deskwt/ > /tmp/dt26-bench.out 2>&1; grep -q 'BenchmarkPruneSweepPerCandidate' /tmp/dt26-bench.out` | exit 0, benchmark exists and runs | exit 0 (output file redirected to runner scratch); "BenchmarkPruneSweepPerCandidate-16 10 2079992854 ns/op 41.60 ms/candidate" | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 16b | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestLegacyPerCandidate' -count=1 -timeout 600s` | exit 0, baseline measured | exit 0; ok cmd/deskwt 26.595s; verbose re-run: "legacy=7.457s (372.828ms/candidate); WHOLE current sweep (all gates, same fixture)=683ms", --- PASS: TestLegacyPerCandidateWalkCost | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 17 | `cd tools/desk && go test -timeout 600s ./cmd/deskwt/... ./internal/gitcore/... -count=1` | exit 0 | exit 0; ok cmd/deskwt 171.496s; ok internal/gitcore 69.049s | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 18 | `cd tools/desk && gofmt -l cmd/deskwt internal/gitcore > /tmp/dt26-fmt.out; test ! -s /tmp/dt26-fmt.out` | exit 0 | exit 0 (output file redirected to runner scratch); gofmt list is 0 bytes | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 19 | `cd statusgen && go run . --root .. --lint; echo $?` | 0 | exit 0; "LINT: PASS", printed 0 | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 20 | `cd statusgen && go run . --root .. --consumers --brief assay:assay:desk-tools:26; echo $?` | 0 (exit 2 is COULD-NOT-CHECK, reported as itself) | COULD-NOT-CHECK, does not meet Expect: printed 1, not 0. Tool output: "--consumers: COULD-NOT-CHECK: assay:assay:desk-tools:26 is not in the diff against cf31c32418ba49f93c679913813768542db1c072, so this run carries no evidence about its claims — no entry was corroborated and none was disproved." then "exit status 2" (go run reports a child's exit 2 as its own 1). Same result with and without a roster present | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |

By hand: 20 of 21 rows meet their Expect; row 20 is could-not-check.

Execution witness (statusgen verifyrun, dry-run, tool v1.0.31, per-row timeout raised to 40m so the mutation harness could finish; nothing written to the brief): exit 2. 2 of 21 rows witnessed pass — row 2 (pass exit=0, sha256:8caf29e6c79c) and row 11 (pass exit=0, sha256:86891ac67ae1). The other 19 rows (1, 3-10, 12-20, 16b) are could-not-run with the tool's own reason: "check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses unshare --net, a Linux facility, and this host is darwin." That is an environment limit of this runner, UNRUN as a witness, and is recorded as itself; the hand results above are direct runs on the host, with the network available.

Previously failing rows:

- Row 17 (witness fail on 2026-09-27, exit 1 inside a Linux container with git 2.39.5): the same command exits 0 by hand here on git 2.56.0, both packages ok. The three add --role transport tests still depend on git 2.46 or later (no version skip was added; the 2.46 floor is still stated in the transport source), so this is the host meeting the floor, not a code change. Whether the row passes under the hermetic Linux witness could not be checked on this host.
- Row 20 (witness fail on 2026-09-27): still cannot decide on merged main. Its behaviour changed — it then printed 0 with "0 corroborated, 0 disproved, 9 unchecked"; it now refuses outright with COULD-NOT-CHECK and exit 2, so the row prints 1. The consumers gate was last touched by #1966 in this window. The cause is unchanged: the brief file is absent from the diff the tool takes, and the implementing squash (PR #1064) itself touches the stream README but not the brief file, so no single diff carries both the brief and its consumer targets. That is the structural case #1281 describes; #1281 is OPEN. Supporting spot check, unchanged from the prior pass: the squash touches all three fixed-here files (prune.go, deskwt.go, gitcore.go) and none of the out-of-scope sites.

Findings:

- Row 15's Expect states the concurrent wall is gated under 1.5x a single sweep. The test's actual gate is 2*single + 5s (pruneperf_test.go:332). The observed ratio (0.93x) satisfies both, so the row passes, but the Expect prose is tighter than what the test enforces. Check-definition note, not an implementation defect.
- Row 16's benchmark reported 41.60 ms/candidate against 10.30 ms on 2026-09-23. The row only reports (row 14 gates, and passed with N=600 sweep=3.915s against 30 s). This run used a long scratch path for the fixture and shared the host with other work, so the figure is not a like-for-like comparison; noted, not raised.
- The hermetic witness for the check:ci rows needs a Linux runner; this host cannot supply it.

Risk-bearing value — enumeration over the non-test files of squash b02ea2b35 (prune.go, prunesingleton.go, deskwt.go, lockreclaim.go, gitcore.go) plus the test gates the Deliverables name, read at the commit under test:

- merge-gate baseline ref = "refs/remotes/origin/main" @ tools/desk/cmd/deskwt/prune.go:484 (the one walk), also @ prune.go:991 (the tip guard's resolve)
- defaultSingletonTTL = 10 * time.Minute @ tools/desk/cmd/deskwt/prunesingleton.go:57
- pruneStampSchema = "deskwt-prune-singleton-v1" @ prunesingleton.go:51
- stamp dir mode 0o700 @ prunesingleton.go:177; stamp file mode 0o600 @ prunesingleton.go:182
- lock-ttl flag default "0" @ prune.go:141
- perf budget at N>=600 = 30 * time.Second @ tools/desk/cmd/deskwt/pruneperf_test.go:58; concurrency budget = 2*single + 5*time.Second @ pruneperf_test.go:332 (test gates only)

Ranked by irreversibility: the baseline ref is the only entry whose error deletes work (an unmerged worktree read as merged and removed). Every other entry is a reversible efficiency, hygiene or test-budget knob, undone by an edit and a redeploy.

RISK-VALUE: DERIVED — merge-gate baseline = "refs/remotes/origin/main" @ tools/desk/cmd/deskwt/prune.go:484 — a HEAD is an ancestor of origin/main exactly when it is in the set of commits reachable from origin/main, which is the set the single Log walk returns (tip included), so set membership equals the ancestor test it replaces. The ref must be spelled in full because git resolves refs/heads/ ahead of refs/remotes/, so a stray local branch named origin/main would otherwise become the baseline (#885). A walk that fails leaves no set and holds every candidate; a partial set can only shrink membership, which holds rather than removes. Corroborated this run by rows 2 and 3 and by row 11's inversion and empty-set mutations, both CAUGHT.

RISK-VALUE: DERIVED (reversible, ranks last) — defaultSingletonTTL = 10 * time.Minute @ tools/desk/cmd/deskwt/prunesingleton.go:57 — a debounce only: a wrong value delays or duplicates a sweep by at most one TTL and never removes anything; bad, missing and future-dated stamps proceed (row 10; row 11's fail-closed-TTL mutation CAUGHT).

The brief is gate: model with all four risk answers no. Status is left where it is.

VERIFY: BLOCKED — 20 of 21 rows pass by hand, including the SPOF row 2, the fail-closed row 3 and 16/16 mutations CAUGHT; row 20 is could-not-check on merged main by construction (check-definition, #1281, open), and the hermetic witness for the 19 check:ci rows could not run on this darwin host (2 of 21 witnessed).

### Non-implementer verifier re-run — VERIFY: BLOCKED — 2026-10-04 claude-opus-5-5-verifier

Runner is a NON-implementer: assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian), dispatched by verify-desk against merged main fe12ee0c90b9e39e78afc291e5b17111844356cb. Deliverable squash b02ea2b35 (PR #1064) is an ancestor of the commit under test.

Why this re-run: the 2026-10-02 receipt (blocked at cf31c3241, 20/21) went stale because three declared inputs changed on main. Between cf31c3241 and fe12ee0c9: prune.go and deskwt.go each gained the same 4-line early return in their flag-parse error branch (#2051, "tier-two --help writes no audit row"). It returns deskkit.ErrHelpRequested when the parse error is a help request, before any sweep, gate, singleton or removal code runs. The rest of prune.go is unchanged, and line numbers below the hunk shift by 4. The desk README hunks (#2051, #2059, #2117, #2120, #2135, #2138, #2155) cover role-init fetch credentials, cellctl, deskpr, deskboard and roster; none touches the prune singleton, its flags or the read-only dry-run text. prunesingleton.go, lockreclaim.go, the prune, singleton and perf tests, prune-mutations.json and every gitcore file are byte-identical to cf31c3241. Conclusion: no change touches the prune behaviour this brief verifies.

How the rows were run:

- **Hand runs.** Every row ran sequentially from a detached worktree at fe12ee0c9, on darwin/arm64 with go1.27.1 and git 2.56.0. HOME was a throwaway (a copy of the roster only), TMPDIR and GOCACHE pointed into the runner's own scratch directory, and GOPROXY was off. Rows 16 and 18 write their output file into that scratch directory rather than the system temp directory; the commands are otherwise unchanged. Row 11 mutates and restores the source tree; the tree was clean before and after it.
- **Execution witness.** `statusgen verifyrun` (built from this same commit, reporting version "dev") ran non-dry inside a throwaway golang:1.25-trixie container that was already cached locally. Its settings:
  - Go 1.25.14 (the go.mod floor) and git 2.47.3.
  - `--network none`, with GOPROXY=off and GOSUMDB=off.
  - The host module cache mounted read-only.
  - The target was a clone pinned to fe12ee0c9, with that clone's refs/remotes/origin/main also pinned to fe12ee0c9 and a clean tree, so the stamp carries no +dirty.
  - Per-row timeout raised to 40m.

  **Disclosure:** the container ran with `--security-opt seccomp=unconfined`, which statusgen's own `unshare --net --map-root-user` sandbox needs inside Docker. The desk authorized this for this throwaway, network-off container only. No image was pulled.

| # | Command | Expect | Observed (exit + key output line) | Date / runner |
|---|---------|--------|-----------------------------------|---------------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | exit 0 | exit 0; no output | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneRemovalSetUnchanged$' -count=1 -timeout 300s` | exit 0, removed set equals the recorded literal | exit 0; ok cmd/deskwt 12.490s; verbose re-run: --- PASS: TestPruneRemovalSetUnchanged (1.95s) | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneHoldsAll' -count=1` | exit 0, fail-closed: zero removed | exit 0; ok cmd/deskwt 8.856s; verbose re-run: --- PASS: TestPruneHoldsAllNoAncestorSet and --- PASS: TestPruneHoldsAllWhenTheMainlineWalkFails | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestMergeGateBeforeStatus$' -count=1` | exit 0 | exit 0; ok cmd/deskwt 4.744s; verbose re-run: --- PASS: TestMergeGateBeforeStatus (0.79s) | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneSkips' -count=1 && go test ./cmd/deskwt/ -run '^TestPruneRefusesSharedCheckout$' -count=1` | exit 0, every existing hold | exit 0; ok 18.050s then ok 2.973s; verbose re-run: 7 TestPruneSkips tests --- PASS (incl. TestPruneSkipsUnpushedCommit and TestPruneSkipsCleanUnmergedBranch), --- PASS: TestPruneRefusesSharedCheckout | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneDryRunIsReadOnly$' -count=1` | exit 0 | exit 0; ok cmd/deskwt 5.511s; verbose re-run: --- PASS: TestPruneDryRunIsReadOnly (0.98s) | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneBatch' -count=1` | exit 0 | exit 0; ok cmd/deskwt 14.016s; verbose re-run: --- PASS: TestPruneBatchesRemoval and --- PASS: TestPruneBatchDemotesStrandedPath | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestRemove' -count=1` | exit 0 | exit 0; ok cmd/deskwt 20.939s; verbose re-run: 11 TestRemove tests --- PASS | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 9 | `cd tools/desk && go test ./internal/gitcore/ -run '^TestOpenWithShares' -count=1 && go test ./internal/gitcore/ -run '^TestOpenStillBuilds' -count=1` | exit 0 | exit 0; ok 1.009s then ok 0.959s; verbose re-run: --- PASS: TestOpenWithSharesCache and --- PASS: TestOpenStillBuildsItsOwnCache | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 10 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneSingleton' -count=1 -timeout 120s` | exit 0 | exit 0; ok cmd/deskwt 29.019s; verbose re-run: 5 TestPruneSingleton tests --- PASS (held-by-live-sweep, recent-sweep, TTL-zero-and-no-singleton, bad-stamp, missing-stamp) | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 11 | `cd tools/desk && go run ./cmd/muhar -spec cmd/deskwt/prune-mutations.json` | exit 0, baseline GREEN, control CAUGHT, every mutation CAUGHT | exit 0 (1279s); "Harness healthy: baseline GREEN, positive control CAUGHT." and "Totals: 16 caught, 0 NOT CAUGHT, 0 could-not-mutate."; tree clean after | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 12 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneInterval' -count=1 -timeout 120s` | exit 0 | exit 0; ok cmd/deskwt 6.458s; verbose re-run: 3 TestPruneInterval tests --- PASS incl. TestPruneIntervalReleasesLockPerTick | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 13 | `cd tools/desk && DESKWT_PERF_N=50 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 600s` | exit 0, walk count 1, wall inside budget | exit 0; ok cmd/deskwt 3.079s | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 14 | `cd tools/desk && DESKWT_PERF_N=200 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 900s && DESKWT_PERF_N=600 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 900s` | exit 0, N=600 sweep under 30 s, walk count 1 | exit 0; ok 5.168s then ok 9.970s; verbose N=600 re-run: "N=600 commits=5000 sweep=1.956s walks=1 removed=0 held=601" (Linux container: sweep=2.037s walks=1) | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 15 | `cd tools/desk && DESKWT_PERF_N=200 go test ./cmd/deskwt/ -run '^TestConcurrentPrunes' -count=1 -timeout 900s` | exit 0, one sweeps, four held, total wall under 1.5x a single sweep | exit 0; ok cmd/deskwt 10.586s; verbose re-run: "N=200 single=837ms five-concurrent=879ms (1 swept, 4 held)", --- PASS: TestConcurrentPrunesCostOne (ratio 1.05x; Linux container: 713ms vs 788ms, 1.11x) | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 16 | `cd tools/desk && go test -bench '^BenchmarkPruneSweepPer' -benchtime 10x -run '^$' ./cmd/deskwt/ > /tmp/dt26-bench.out 2>&1; grep -q 'BenchmarkPruneSweepPerCandidate' /tmp/dt26-bench.out` | exit 0, benchmark exists and runs | exit 0 (output file redirected to runner scratch); "BenchmarkPruneSweepPerCandidate-16 10 1650833162 ns/op 33.02 ms/candidate" | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 16b | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestLegacyPerCandidate' -count=1 -timeout 600s` | exit 0, baseline measured | exit 0; ok cmd/deskwt 23.199s; verbose re-run: "legacy=6.658s (332.909ms/candidate); WHOLE current sweep (all gates, same fixture)=414ms", --- PASS: TestLegacyPerCandidateWalkCost | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 17 | `cd tools/desk && go test -timeout 600s ./cmd/deskwt/... ./internal/gitcore/... -count=1` | exit 0 | exit 0 on the second hand run (ok cmd/deskwt 123.031s; ok internal/gitcore 29.277s) and exit 0 in the Linux witness (ok 50.900s, ok 12.955s). The first hand run exited 1 on one test, TestBranchHoldersPropagatesAGitError ("branchHolders outside a repo returned no error"). Cause: the runner had pointed TMPDIR inside its own git worktree, so the test's temp directory was not outside a repo. Re-run with git discovery capped at that temp root (GIT_CEILING_DIRECTORIES): that test passes alone, and the whole row exits 0. Runner-environment artifact, not a code fault | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 18 | `cd tools/desk && gofmt -l cmd/deskwt internal/gitcore > /tmp/dt26-fmt.out; test ! -s /tmp/dt26-fmt.out` | exit 0 | exit 0 (output file redirected to runner scratch); gofmt list is 0 bytes | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 19 | `cd statusgen && go run . --root .. --lint; echo $?` | 0 | exit 0; "LINT: PASS", printed 0 | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 20 | `cd statusgen && go run . --root .. --consumers --brief assay:assay:desk-tools:26; echo $?` | 0 (exit 2 is COULD-NOT-CHECK, reported as itself) | COULD-NOT-CHECK; does not meet Expect: printed 1, not 0. Tool output: "--consumers: COULD-NOT-CHECK: assay:assay:desk-tools:26 is not in the diff against fe12ee0c90b9e39e78afc291e5b17111844356cb, so this run carries no evidence about its claims — no entry was corroborated and none was disproved." then "exit status 2" (go run reports the child's exit 2 as its own 1) | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |

By hand: 20 of 21 rows meet their Expect. Row 20 is could-not-check.

Execution witness: `statusgen verifyrun` in the Linux container described above, network-off, non-dry, written at a clean tree pinned to fe12ee0c9. It exited 1 because row 20 records a failure.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |
| 2 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneRemovalSetUnchanged$' -count=1 -timeout 300s` | pass exit=0 | sha256:94072c530011 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |
| 3 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneHoldsAll' -count=1` | pass exit=0 | sha256:0a523a0a67bc | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |
| 4 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestMergeGateBeforeStatus$' -count=1` | pass exit=0 | sha256:18179e892f6b | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |
| 5 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneSkips' -count=1 && go test ./cmd/deskwt/ -run '^TestPruneRefusesSharedCheckout$' -count=1` | pass exit=0 | sha256:a234f312da39 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |
| 6 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneDryRunIsReadOnly$' -count=1` | pass exit=0 | sha256:3e91a89f24f8 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |
| 7 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneBatch' -count=1` | pass exit=0 | sha256:ef1630928be1 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |
| 8 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestRemove' -count=1` | pass exit=0 | sha256:38d180c59859 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |
| 9 | `cd tools/desk && go test ./internal/gitcore/ -run '^TestOpenWithShares' -count=1 && go test ./internal/gitcore/ -run '^TestOpenStillBuilds' -count=1` | pass exit=0 | sha256:9683b10b69de | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |
| 10 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneSingleton' -count=1 -timeout 120s` | pass exit=0 | sha256:c03ced4addfd | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |
| 11 | `cd tools/desk && go run ./cmd/muhar -spec cmd/deskwt/prune-mutations.json` | pass exit=0 | sha256:2cef7e8079e3 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |
| 12 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneInterval' -count=1 -timeout 120s` | pass exit=0 | sha256:f5461c4784b0 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |
| 13 | `cd tools/desk && DESKWT_PERF_N=50 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 600s` | pass exit=0 | sha256:290ac98f13dc | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |
| 14 | `cd tools/desk && DESKWT_PERF_N=200 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 900s && DESKWT_PERF_N=600 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 900s` | pass exit=0 | sha256:2f2187a18645 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |
| 15 | `cd tools/desk && DESKWT_PERF_N=200 go test ./cmd/deskwt/ -run '^TestConcurrentPrunes' -count=1 -timeout 900s` | pass exit=0 | sha256:1b878346a2b5 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |
| 16 | `cd tools/desk && go test -bench '^BenchmarkPruneSweepPer' -benchtime 10x -run '^$' ./cmd/deskwt/ > /tmp/dt26-bench.out 2>&1; grep -q 'BenchmarkPruneSweepPerCandidate' /tmp/dt26-bench.out` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |
| 16b | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestLegacyPerCandidate' -count=1 -timeout 600s` | pass exit=0 | sha256:fb4d17b4bf76 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |
| 17 | `cd tools/desk && go test -timeout 600s ./cmd/deskwt/... ./internal/gitcore/... -count=1` | pass exit=0 | sha256:02cde06ebc99 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |
| 18 | `cd tools/desk && gofmt -l cmd/deskwt internal/gitcore > /tmp/dt26-fmt.out; test ! -s /tmp/dt26-fmt.out` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |
| 19 | `cd statusgen && go run . --root .. --lint; echo $?` | pass exit=0 | sha256:d772997bb1b9 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |
| 20 | `cd statusgen && go run . --root .. --consumers --brief assay:assay:desk-tools:26; echo $?` | fail exit=0 | sha256:aee00550ba47 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |

`statusgen verifyrun --check` over that witness (same container) exited 1 and printed: "docs/streams/desk-tools/brief-26-deskwt-prune-one-walk-and-a-lock.md: 20 pass, 1 fail, 0 could-not-run/missing (of 21 Verify rows)". The one fail is row 20: "the witness records a failure".

Previously failing rows:

- **Row 17** failed on 2026-09-27 under a bookworm container with git 2.39.5. It now passes in the hermetic witness, on trixie with git 2.47.3, which meets the 2.46 floor the add --role transport tests assume; the verbose re-run in the container shows TestAddRoleReplacesInheritedSSHPushUnderBotLoop and its siblings --- PASS. This is the first network-off witness of row 17 passing. No source change was needed.
- **Row 20** still cannot decide on merged main, for the same structural reason as before. The brief file is absent from the diff the consumers gate takes, and the implementing squash (PR #1064) does not touch the brief file, so no single diff carries both the brief and its consumer targets. The witness records "fail exit=0" because the trailing echo forces the row's exit to 0 while the tool reads "exit 2" from the Expect cell. The hand run shows the real state: COULD-NOT-CHECK. This is check-definition, the case #1281 describes, and #1281 is OPEN (checked 2026-10-04). Supporting spot check, unchanged from the prior pass: the squash touches all three fixed-here files (prune.go, deskwt.go, gitcore.go) and none of the out-of-scope sites.

Findings:

- **Row 15's Expect is tighter than its test.** The Expect says the concurrent wall is gated under 1.5x a single sweep, but the test's actual gate is 2*single + 5s (pruneperf_test.go:332). The observed ratios (1.05x on darwin, 1.11x in the Linux container) satisfy both. Check-definition note, carried over.
- **One test assumes TMPDIR is not inside a git repo.** TestBranchHoldersPropagatesAGitError (branchcollision_test.go) assumes a t.TempDir() is not inside any git repository, and fails when TMPDIR sits under a checkout. That is a test-portability gap in the branch-collision tests, not in anything this brief ships; noted, not raised.

Risk-bearing value. The enumeration covers the non-test files of squash b02ea2b35 (prune.go, prunesingleton.go, deskwt.go, lockreclaim.go, gitcore.go) plus the test gates the Deliverables name, read at fe12ee0c9. Line numbers in prune.go are shifted by +4 from the prior pass by the #2051 help-request hunk.

- merge-gate baseline ref = "refs/remotes/origin/main" @ tools/desk/cmd/deskwt/prune.go:488 (the one walk), also @ prune.go:995 (the tip guard's resolve)
- defaultSingletonTTL = 10 * time.Minute @ tools/desk/cmd/deskwt/prunesingleton.go:57
- pruneStampSchema = "deskwt-prune-singleton-v1" @ prunesingleton.go:51
- stamp dir mode 0o700 @ prunesingleton.go:177; stamp file mode 0o600 @ prunesingleton.go:182
- lock-ttl flag default "0" @ prune.go:141
- perf budget at N>=600 = 30 * time.Second @ tools/desk/cmd/deskwt/pruneperf_test.go:58; concurrency budget = 2*single + 5*time.Second @ pruneperf_test.go:332 (test gates only)

Ranked by irreversibility, the baseline ref is the only entry whose error deletes work: an unmerged worktree read as merged gets removed. Every other entry is a reversible efficiency, hygiene or test-budget knob, undone by an edit and a redeploy.

RISK-VALUE: DERIVED — merge-gate baseline = "refs/remotes/origin/main" @ tools/desk/cmd/deskwt/prune.go:488 — HEAD is an ancestor of origin/main exactly when it is in the set of commits reachable from origin/main, and the single Log walk returns that set, tip included. Set membership therefore equals the ancestor test it replaces. The ref must be spelled in full because git resolves refs/heads/ ahead of refs/remotes/, so a stray local branch named origin/main would otherwise become the baseline (#885). A walk that fails leaves the set nil, and every candidate is then held as unverifiable (prune.go:517 and prune.go:600). A partial set can only shrink membership, which holds worktrees rather than removing them. Corroborated this run by rows 2 and 3 and by row 11's inversion and empty-set mutations, both CAUGHT.

RISK-VALUE: DERIVED (reversible, ranks last) — defaultSingletonTTL = 10 * time.Minute @ tools/desk/cmd/deskwt/prunesingleton.go:57 — this is a debounce only. A wrong value delays or duplicates a sweep by at most one TTL and never removes anything. Bad, missing and future-dated stamps all proceed (row 10; row 11's fail-closed-TTL mutation CAUGHT).

The brief is gate: model, with all four risk answers no. Status is left where it is.

VERIFY: BLOCKED — 20 of 21 rows pass by hand and 20 of 21 pass in the network-off Linux witness. That includes the SPOF row 2, the fail-closed row 3, 16/16 mutations CAUGHT, and row 17 green under the hermetic witness for the first time. Row 20 is could-not-check on merged main by construction (check-definition, blocker #1281, open).
### Non-implementer verifier re-run — VERIFY: BLOCKED — 20/21 — 2026-10-07 claude-opus-5-5-verifier

Runner is a NON-implementer: assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian), dispatched by verify-desk against merged main dde4fbeaead9c173e8355869e06d86e8e9eee446. Deliverable squash b02ea2b35 (PR #1064) is an ancestor of that commit.

Why this re-run: inputs changed on main after the 2026-10-04 pass (fe12ee0c9). Between fe12ee0c9 and dde4fbea, prune.go, prunesingleton.go, lockreclaim.go and gitcore.go are byte-identical. deskwt.go gained `deskwt add --upstream` (a pinned-commit worktree with a tracked upstream, about 25 lines in cmdAdd), which is outside the prune sweep and outside `remove`. gitcore gained write.go and test additions. statusgen changed (coverage inputs, read memo, stale-fail, verifyrun admission), but none of the changes is in the consumers gate. medici-finance/assay#1281 is still OPEN (checked 2026-10-07).

How the rows were run: by hand, one after another, from a detached worktree at dde4fbea on darwin/arm64 with go1.27.1 and git 2.56.0. HOME was an empty throwaway with no roster. TMPDIR was a scratch directory outside any git repo, with GIT_CEILING_DIRECTORIES set to its parent. GOPROXY was off and KUBECONFIG was /dev/null. The go test rows had -v added so each test's PASS line could be read; their exit codes are the same with or without it. Row 11 mutates the source tree and then restores it; the tree was clean afterwards.

| # | Command | Expect | Observed (exit + key output line) | Date | Runner |
|---|---------|--------|-----------------------------------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | exit 0 | exit 0; no output | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneRemovalSetUnchanged$' -count=1 -timeout 300s` | exit 0, removed set equals the recorded literal | exit 0; --- PASS: TestPruneRemovalSetUnchanged (4.51s); ok cmd/deskwt 6.147s | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneHoldsAll' -count=1` | exit 0, fail-closed: zero removed | exit 0; --- PASS: TestPruneHoldsAllNoAncestorSet and --- PASS: TestPruneHoldsAllWhenTheMainlineWalkFails; ok 3.963s | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestMergeGateBeforeStatus$' -count=1` | exit 0 | exit 0; --- PASS: TestMergeGateBeforeStatus (1.18s) | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneSkips' -count=1 && go test ./cmd/deskwt/ -run '^TestPruneRefusesSharedCheckout$' -count=1` | exit 0, every existing hold | exit 0; 7 TestPruneSkips tests --- PASS (incl. TestPruneSkipsUnpushedCommit, TestPruneSkipsCleanUnmergedBranch, TestPruneSkipsDirtyTracked), then --- PASS: TestPruneRefusesSharedCheckout | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneDryRunIsReadOnly$' -count=1` | exit 0 | exit 0; --- PASS: TestPruneDryRunIsReadOnly (1.21s) | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneBatch' -count=1` | exit 0 | exit 0; --- PASS: TestPruneBatchesRemoval and --- PASS: TestPruneBatchDemotesStrandedPath | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestRemove' -count=1` | exit 0 | exit 0; 11 TestRemove tests --- PASS; ok 9.127s | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd tools/desk && go test ./internal/gitcore/ -run '^TestOpenWithShares' -count=1 && go test ./internal/gitcore/ -run '^TestOpenStillBuilds' -count=1` | exit 0 | exit 0; --- PASS: TestOpenWithSharesCache, then --- PASS: TestOpenStillBuildsItsOwnCache | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneSingleton' -count=1 -timeout 120s` | exit 0 | exit 0; 5 TestPruneSingleton tests --- PASS (held-by-live-sweep, recent-sweep, TTL-zero-and-no-singleton, bad-stamp, missing-stamp); ok 14.899s | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 11 | `cd tools/desk && go run ./cmd/muhar -spec cmd/deskwt/prune-mutations.json` | exit 0, baseline GREEN, control CAUGHT, every mutation CAUGHT | exit 0 (773s); "Harness healthy: baseline GREEN, positive control CAUGHT." and "Totals: 16 caught, 0 NOT CAUGHT, 0 could-not-mutate."; tree clean after | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 12 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneInterval' -count=1 -timeout 120s` | exit 0 | exit 0; 3 TestPruneInterval tests --- PASS incl. TestPruneIntervalReleasesLockPerTick | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 13 | `cd tools/desk && DESKWT_PERF_N=50 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 600s` | exit 0, walk count 1, wall inside budget | exit 0; "N=50 commits=5000 sweep=2.117s walks=1 removed=0 held=51" | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 14 | `cd tools/desk && DESKWT_PERF_N=200 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 900s && DESKWT_PERF_N=600 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 900s` | exit 0, N=600 sweep under 30 s, walk count 1 | exit 0; "N=200 commits=5000 sweep=4.885s walks=1" then "N=600 commits=5000 sweep=7.508s walks=1 removed=0 held=601" | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 15 | `cd tools/desk && DESKWT_PERF_N=200 go test ./cmd/deskwt/ -run '^TestConcurrentPrunes' -count=1 -timeout 900s` | exit 0, one sweeps, four held | exit 0; "N=200 single=3.44s five-concurrent=3.42s (1 swept, 4 held)"; --- PASS: TestConcurrentPrunesCostOne | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 16 | `cd tools/desk && go test -bench '^BenchmarkPruneSweepPer' -benchtime 10x -run '^$' ./cmd/deskwt/ > /tmp/dt26-bench.out 2>&1; grep -q 'BenchmarkPruneSweepPerCandidate' /tmp/dt26-bench.out` | exit 0, benchmark exists and runs | exit 0; "BenchmarkPruneSweepPerCandidate-16 10 1638951762 ns/op 32.78 ms/candidate" | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 16b | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestLegacyPerCandidate' -count=1 -timeout 600s` | exit 0, baseline measured | exit 0; "legacy=19.641s (982.041ms/candidate); WHOLE current sweep (all gates, same fixture)=975ms"; --- PASS: TestLegacyPerCandidateWalkCost | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 17 | `cd tools/desk && go test -timeout 600s ./cmd/deskwt/... ./internal/gitcore/... -count=1` | exit 0 | exit 0 on the first attempt; ok cmd/deskwt 136.768s; ok internal/gitcore 50.200s | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 18 | `cd tools/desk && gofmt -l cmd/deskwt internal/gitcore > /tmp/dt26-fmt.out; test ! -s /tmp/dt26-fmt.out` | exit 0 | exit 0; gofmt list is 0 bytes | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 19 | `cd statusgen && go run . --root .. --lint; echo $?` | 0 | exit 0; "LINT: PASS", printed 0 | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 20 | `cd statusgen && go run . --root .. --consumers --brief assay:assay:desk-tools:26; echo $?` | 0 (exit 2 is COULD-NOT-CHECK, reported as itself) | COULD-NOT-CHECK; does not meet Expect: printed 1, not 0. Tool output: "--consumers: COULD-NOT-CHECK: assay:assay:desk-tools:26 is not in the diff against dde4fbeaead9c173e8355869e06d86e8e9eee446, so this run carries no evidence about its claims" then "exit status 2" (go run turns the child's exit 2 into its own exit 1) | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |

By hand: 20 of 21 rows meet their Expect. Row 20 is could-not-check, the same result as the 2026-10-04 pass.

Row 20 still cannot be decided, for the same structural reason as before. The consumers gate has no diff that contains both the brief file and the files it names as consumers. A diagnostic run against the pre-implementation base (`--base b02ea2b35^`) does not help either: it reports all 9 entries UNCHECKED ("0 corroborated, 0 disproved, 9 unchecked"). That is no evidence either way, so it does not count as a pass. Blocker: check-definition, medici-finance/assay#1281, OPEN.

Execution witness: `/opt/desk-tools/bin/statusgen verifyrun` (v1.0.32) ran non-dry on this darwin host at a clean tree pinned to dde4fbea, with --timeout 40m, and exited 2. Rows 2 and 11 are not classed check:ci, so verifyrun ran them on the host, and both passed (exit 0). The other 19 rows are check:ci, and verifyrun recorded them as could-not-run: check:ci rows need the `unshare --net` network-off sandbox, and that only exists on Linux. No container was used on this pass. The runner stamp is "(git-config)" without on-behalf-of, because the tool was given no roster.

Risk-bearing value. The enumeration covers the non-test files of squash b02ea2b35 (prune.go, prunesingleton.go, deskwt.go, lockreclaim.go, gitcore.go) plus the test gates the Deliverables name, all re-read at dde4fbea. The line numbers match the 2026-10-04 pass because these files did not change.

- merge-gate baseline ref = "refs/remotes/origin/main" @ tools/desk/cmd/deskwt/prune.go:488 (the one walk)
- defaultSingletonTTL = 10 * time.Minute @ tools/desk/cmd/deskwt/prunesingleton.go:57
- pruneStampSchema = "deskwt-prune-singleton-v1" @ prunesingleton.go:51
- stamp dir mode 0o700 @ prunesingleton.go:177; stamp file mode 0o600 @ prunesingleton.go:182
- lock-ttl flag default "0" @ prune.go:141
- perf budget at N>=600 = 30 * time.Second @ tools/desk/cmd/deskwt/pruneperf_test.go:58; concurrency budget = 2*single + 5*time.Second @ pruneperf_test.go:332 (test gates only)

Ranked by irreversibility, the baseline ref is the only entry whose error deletes work: if it is wrong, an unmerged worktree reads as merged and is removed. Every other entry is a reversible knob for efficiency, hygiene or a test budget.

RISK-VALUE: DERIVED — merge-gate baseline = "refs/remotes/origin/main" @ tools/desk/cmd/deskwt/prune.go:488. Why it is right:
- HEAD is an ancestor of origin/main exactly when HEAD is in the set of commits reachable from origin/main. newSweepCtx builds that set with one Log walk on the shared checkout, tip included, and the membership test is at prune.go:530–531.
- The ref is spelled in full. Git resolves refs/heads/ ahead of refs/remotes/, so a stray local branch named origin/main would otherwise become the baseline (#885).
- A walk that fails leaves the set nil, and then every candidate is held as unverifiable (prune.go:517–520 and prune.go:600–602). A partial set can only shrink membership, which holds worktrees instead of removing them.
- This run corroborates it: rows 2 and 3 pass, and row 11's inversion and empty-set mutations are both CAUGHT.

RISK-VALUE: DERIVED (reversible, ranks last) — defaultSingletonTTL = 10 * time.Minute @ tools/desk/cmd/deskwt/prunesingleton.go:57. It is a debounce only. A wrong value delays or repeats a sweep by at most one TTL and never removes anything.

Finding carried over: row 15's Expect says the concurrent wall is gated under 1.5x a single sweep, but the test gates on 2*single + 5s (pruneperf_test.go:332). The observed ratio this run, about 0.99x, satisfies both.

The brief is gate: model, with all four risk answers no. Its status is left at implemented.

VERIFY: BLOCKED — 20 of 21 rows pass by hand on merged main dde4fbea, including the SPOF row 2, the fail-closed row 3 and 16/16 mutations CAUGHT. Row 20 is could-not-check by construction (check-definition, blocker medici-finance/assay#1281, open).

**VERIFY: BLOCKED**

### Non-implementer verifier re-run — VERIFY: BLOCKED — 20/21 — 2026-10-11 claude-opus-5-5-verifier

Runner is a NON-implementer: assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian), dispatched by verify-desk against merged main 08d194b1d25d8e44c33e1a46e41b1cb9ad6e4059. Deliverable squash b02ea2b35 (PR #1064) is an ancestor of that commit. Admission `deskdispatch --check-verifier` exited 0 at the start and again before this Evidence was written: Run f1adaf4e869f6d602646b7c4a4b680872a697768c955214c57d539ae9c6fc8f9, BriefSHA256 c61cd16ec46ef82394d1215fd7a298a680fd0ae9139a27d7d5b63dd81433597a, PlanSHA256 f52ea7b419cec0024487ab67487def67c2d973590ab5a5db99713ea8bb2c1dc6, Model claude-opus-5-5, Tier any, attestation issue medici-finance/assay#2589.

Why this re-run: inputs changed on main after the 2026-10-07 pass (dde4fbea). prune.go grew by about 214 lines (branch GC #2508, reaping detached review worktrees #2518, cache pressure #2520; each belongs to another brief), prune-mutations.json now carries 21 mutations instead of 16, the muhar workspace changed (#2550, #2506), and gitcore.go changed by 10 lines (alternate pack indexes, bd0e29bc7). This is a fresh dispatch: an earlier home for this run was pruned mid-run, and none of its rows count. Every row below was re-run at 08d194b1.

Execution witness: `/opt/desk-tools/bin/statusgen verifyrun` (v1.0.39, installed at 12:51Z, before the witness started) ran non-dry on this darwin/arm64 host at the clean home pinned to 08d194b1, with --timeout 60m, from 13:03:48Z to 14:07:11Z. It exited 2. Row 2 is not classed check:ci, so verifyrun ran it on the host, and it passed (exit 0). Row 11 is also not check:ci, and verifyrun ran it on the host, but it hit the 60-minute timeout and produced no verdict. The host was shared with another verifier's mutation run, and the hand run of row 11 below took 50 minutes on its own, so this is a capacity limit, not a defect. The other 19 rows are check:ci, and verifyrun recorded them as could-not-run: check:ci rows need the `unshare --net` network-off sandbox, and that only exists on Linux. No container was used. verifyrun appended its table below:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 08d194b1d25d (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneRemovalSetUnchanged$' -count=1 -timeout 300s` | pass exit=0 | sha256:872844c1608c | 2026-10-11 | assay-verifier-app[bot] @ 08d194b1d25d (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneHoldsAll' -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 08d194b1d25d (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestMergeGateBeforeStatus$' -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 08d194b1d25d (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneSkips' -count=1 && go test ./cmd/deskwt/ -run '^TestPruneRefusesSharedCheckout$' -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 08d194b1d25d (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneDryRunIsReadOnly$' -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 08d194b1d25d (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneBatch' -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 08d194b1d25d (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestRemove' -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 08d194b1d25d (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd tools/desk && go test ./internal/gitcore/ -run '^TestOpenWithShares' -count=1 && go test ./internal/gitcore/ -run '^TestOpenStillBuilds' -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 08d194b1d25d (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneSingleton' -count=1 -timeout 120s` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 08d194b1d25d (on-behalf-of human:ian) (forge-identity) |
| 11 | `cd tools/desk && go run ./cmd/muhar -spec cmd/deskwt/prune-mutations.json` | could-not-run exit=- — timed out after 1h0m0s — no verdict was produced | sha256:838f17bdb1d9 | 2026-10-11 | assay-verifier-app[bot] @ 08d194b1d25d (on-behalf-of human:ian) (forge-identity) |
| 12 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneInterval' -count=1 -timeout 120s` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 08d194b1d25d (on-behalf-of human:ian) (forge-identity) |
| 13 | `cd tools/desk && DESKWT_PERF_N=50 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 600s` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 08d194b1d25d (on-behalf-of human:ian) (forge-identity) |
| 14 | `cd tools/desk && DESKWT_PERF_N=200 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 900s && DESKWT_PERF_N=600 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 900s` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 08d194b1d25d (on-behalf-of human:ian) (forge-identity) |
| 15 | `cd tools/desk && DESKWT_PERF_N=200 go test ./cmd/deskwt/ -run '^TestConcurrentPrunes' -count=1 -timeout 900s` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 08d194b1d25d (on-behalf-of human:ian) (forge-identity) |
| 16 | `cd tools/desk && go test -bench '^BenchmarkPruneSweepPer' -benchtime 10x -run '^$' ./cmd/deskwt/ > /tmp/dt26-bench.out 2>&1; grep -q 'BenchmarkPruneSweepPerCandidate' /tmp/dt26-bench.out` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 08d194b1d25d (on-behalf-of human:ian) (forge-identity) |
| 16b | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestLegacyPerCandidate' -count=1 -timeout 600s` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 08d194b1d25d (on-behalf-of human:ian) (forge-identity) |
| 17 | `cd tools/desk && go test -timeout 600s ./cmd/deskwt/... ./internal/gitcore/... -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 08d194b1d25d (on-behalf-of human:ian) (forge-identity) |
| 18 | `cd tools/desk && gofmt -l cmd/deskwt internal/gitcore > /tmp/dt26-fmt.out; test ! -s /tmp/dt26-fmt.out` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 08d194b1d25d (on-behalf-of human:ian) (forge-identity) |
| 19 | `cd statusgen && go run . --root .. --lint; echo $?` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 08d194b1d25d (on-behalf-of human:ian) (forge-identity) |
| 20 | `cd statusgen && go run . --root .. --consumers --brief assay:assay:desk-tools:26; echo $?` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 08d194b1d25d (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#2589 run=f1adaf4e869f6d602646b7c4a4b680872a697768c955214c57d539ae9c6fc8f9 source=08d194b1d25d8e44c33e1a46e41b1cb9ad6e4059 brief=docs/streams/desk-tools/brief-26-deskwt-prune-one-walk-and-a-lock.md model=claude-opus-5-5 tier=any

Hand-run rows, because the witness could not decide 20 of them on this host. How the rows were run: by hand, one after another, from the same detached home at 08d194b1 on darwin/arm64 with go1.27.1 and git 2.56.0. HOME was an empty throwaway with no roster. TMPDIR was a scratch directory outside any git repo, with GIT_CEILING_DIRECTORIES set to its parent. GOPROXY was off, GOTOOLCHAIN was local, GOFLAGS was unset, and KUBECONFIG was /dev/null. The go test rows had -v added so each test's PASS line could be read; their exit codes are the same with or without it. Row 11 mutates the source tree and then restores it; the tree was clean afterwards.

| # | Command | Expect | Observed (exit + key output line) | Date | Runner |
|---|---------|--------|-----------------------------------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | exit 0 | exit 0; no output (19s) | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneRemovalSetUnchanged$' -count=1 -timeout 300s` | exit 0, removed set equals the recorded literal | exit 0; --- PASS: TestPruneRemovalSetUnchanged (15.25s) | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneHoldsAll' -count=1` | exit 0, fail-closed: zero removed | exit 0; --- PASS: TestPruneHoldsAllNoAncestorSet and --- PASS: TestPruneHoldsAllWhenTheMainlineWalkFails | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestMergeGateBeforeStatus$' -count=1` | exit 0 | exit 0; --- PASS: TestMergeGateBeforeStatus (6.33s) | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneSkips' -count=1 && go test ./cmd/deskwt/ -run '^TestPruneRefusesSharedCheckout$' -count=1` | exit 0, every existing hold | exit 0; 7 TestPruneSkips tests --- PASS (DirtyTracked, FreshAtMainWorktreeWithUntrackedFile, UnpushedCommit, CleanUnmergedBranch, LockedWorktree, LockedWorktreeNoReason, CurrentWorktree), then --- PASS: TestPruneRefusesSharedCheckout | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 6 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneDryRunIsReadOnly$' -count=1` | exit 0 | exit 0; --- PASS: TestPruneDryRunIsReadOnly (7.86s) | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 7 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneBatch' -count=1` | exit 0 | exit 0; --- PASS: TestPruneBatchesRemoval and --- PASS: TestPruneBatchDemotesStrandedPath | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 8 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestRemove' -count=1` | exit 0 | exit 0; 11 TestRemove tests --- PASS; ok 49.570s | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 9 | `cd tools/desk && go test ./internal/gitcore/ -run '^TestOpenWithShares' -count=1 && go test ./internal/gitcore/ -run '^TestOpenStillBuilds' -count=1` | exit 0 | exit 0; --- PASS: TestOpenWithSharesCache, then --- PASS: TestOpenStillBuildsItsOwnCache | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 10 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneSingleton' -count=1 -timeout 120s` | exit 0 | exit 0; 5 TestPruneSingleton tests --- PASS (HeldByALiveSweepSkips, RecentSweepSkips, TTLZeroAndNoSingletonStillSweep, BadStampAlwaysSweeps, MissingStampSweeps); ok 48.102s | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 11 | `cd tools/desk && go run ./cmd/muhar -spec cmd/deskwt/prune-mutations.json` | exit 0, baseline GREEN, control CAUGHT, every mutation CAUGHT | exit 0 (3014s, run in place with -j 1); "Harness healthy: baseline GREEN, positive control CAUGHT." and "Totals: 21 caught, 0 NOT CAUGHT, 0 could-not-mutate."; tree clean after | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 12 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneInterval' -count=1 -timeout 120s` | exit 0 | exit 0; 3 TestPruneInterval tests --- PASS incl. TestPruneIntervalReleasesLockPerTick | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 13 | `cd tools/desk && DESKWT_PERF_N=50 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 600s` | exit 0, walk count 1, wall inside budget | exit 0; "N=50 commits=5000 sweep=762ms walks=1 removed=0 held=51" | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 14 | `cd tools/desk && DESKWT_PERF_N=200 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 900s && DESKWT_PERF_N=600 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 900s` | exit 0, N=600 sweep under 30 s, walk count 1 | exit 0; "N=200 commits=5000 sweep=1.328s walks=1 removed=0 held=201" then "N=600 commits=5000 sweep=2.511s walks=1 removed=0 held=601" | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 15 | `cd tools/desk && DESKWT_PERF_N=200 go test ./cmd/deskwt/ -run '^TestConcurrentPrunes' -count=1 -timeout 900s` | exit 0, one sweeps, four held | exit 0; "N=200 single=1.46s five-concurrent=1.425s (1 swept, 4 held)"; --- PASS: TestConcurrentPrunesCostOne | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 16 | `cd tools/desk && go test -bench '^BenchmarkPruneSweepPer' -benchtime 10x -run '^$' ./cmd/deskwt/ > /tmp/dt26-bench.out 2>&1; grep -q 'BenchmarkPruneSweepPerCandidate' /tmp/dt26-bench.out` | exit 0, benchmark exists and runs | exit 0; "BenchmarkPruneSweepPerCandidate-16 10 790822796 ns/op 15.82 ms/candidate" | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 16b | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestLegacyPerCandidate' -count=1 -timeout 600s` | exit 0, baseline measured | exit 0; "legacy=7.549s (377.461ms/candidate); WHOLE current sweep (all gates, same fixture)=576ms"; --- PASS: TestLegacyPerCandidateWalkCost | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 17 | `cd tools/desk && go test -timeout 600s ./cmd/deskwt/... ./internal/gitcore/... -count=1` | exit 0 | exit 0 on the first attempt; ok cmd/deskwt 228.508s; ok internal/gitcore 32.190s | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 18 | `cd tools/desk && gofmt -l cmd/deskwt internal/gitcore > /tmp/dt26-fmt.out; test ! -s /tmp/dt26-fmt.out` | exit 0 | exit 0; gofmt list is 0 bytes | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 19 | `cd statusgen && go run . --root .. --lint; echo $?` | 0 | exit 0; "LINT: PASS", printed 0 (no PROBLEM lines) | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 20 | `cd statusgen && go run . --root .. --consumers --brief assay:assay:desk-tools:26; echo $?` | 0 (exit 2 is COULD-NOT-CHECK, reported as itself) | COULD-NOT-CHECK; does not meet Expect: printed 1, not 0. Tool output: "--consumers: COULD-NOT-CHECK: assay:assay:desk-tools:26 is not in the diff against 08d194b1d25d8e44c33e1a46e41b1cb9ad6e4059, so this run carries no evidence about its claims — no entry was corroborated and none was disproved." then "exit status 2" (go run turns the child's exit 2 into its own exit 1) | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

By hand: 20 of 21 rows meet their Expect. Row 20 is could-not-check, the same result as the 2026-10-04 and 2026-10-07 passes. Row 11 passed by hand with all 21 mutations CAUGHT, including the 5 added since the last pass.

Row 20 still cannot be decided, for the same structural reason as before. The consumers gate has no diff that contains both the brief file and the files it names as consumers. Blocker: check-definition, medici-finance/assay#1281, still OPEN (checked 2026-10-11).

Risk-bearing value. The enumeration covers the non-test files of squash b02ea2b35 (prune.go, prunesingleton.go, deskwt.go, lockreclaim.go, gitcore.go) plus the test gates the Deliverables name, all re-read at 08d194b1. The prune.go line numbers moved because of the additions listed above; the values themselves did not change.

- merge-gate baseline ref = "refs/remotes/origin/main" @ tools/desk/cmd/deskwt/prune.go:648 (the one walk)
- defaultSingletonTTL = 10 * time.Minute @ tools/desk/cmd/deskwt/prunesingleton.go:57
- pruneStampSchema = "deskwt-prune-singleton-v1" @ prunesingleton.go:51
- stamp dir mode 0o700 @ prunesingleton.go:177; stamp file mode 0o600 @ prunesingleton.go:182
- lock-ttl flag default "0" @ prune.go:188; singleton-ttl flag default defaultSingletonTTL @ prune.go:200
- perf budget at N>=600 = 30 * time.Second @ tools/desk/cmd/deskwt/pruneperf_test.go:58; concurrency budget = 2*single + 5*time.Second @ pruneperf_test.go:332 (test gates only)

Ranked by irreversibility, the baseline ref is the only entry whose error deletes work: if it is wrong, an unmerged worktree reads as merged and is removed. Every other entry is a reversible knob for efficiency, hygiene or a test budget.

RISK-VALUE: DERIVED — merge-gate baseline = "refs/remotes/origin/main" @ tools/desk/cmd/deskwt/prune.go:648 — HEAD is an ancestor of origin/main exactly when HEAD is in the set of commits reachable from origin/main, and that set is built by one walk from this ref. Why it is right:
- newSweepCtx builds the set with one Log walk on the shared checkout, tip included, and the membership test is at prune.go:690.
- The ref is spelled in full. Git resolves refs/heads/ ahead of refs/remotes/, so a stray local branch named origin/main would otherwise become the baseline (#885).
- A walk that fails leaves the set nil, and then every candidate is held as unverifiable (prune.go:677–680 and the sweep warning at prune.go:771–774). A partial set can only shrink membership, which holds worktrees instead of removing them.
- This run corroborates it: rows 2 and 3 pass, and row 11's inversion and empty-set mutations are both CAUGHT.

RISK-VALUE: DERIVED (reversible, ranks last) — defaultSingletonTTL = 10 * time.Minute @ tools/desk/cmd/deskwt/prunesingleton.go:57 — it is a debounce only. A wrong value delays or repeats a sweep by at most one TTL and never removes anything.

Finding carried over: row 15's Expect says the concurrent wall is gated under 1.5x a single sweep, but the test gates on 2*single + 5s (pruneperf_test.go:332). The observed ratio this run, about 0.98x, satisfies both.

The brief is gate: model, with all four risk answers no. Its status is left at implemented.

VERIFY: BLOCKED — 20 of 21 rows pass by hand on merged main 08d194b1, including the SPOF row 2 (also passed by the witness), the fail-closed row 3 and 21/21 mutations CAUGHT. Row 20 is could-not-check by construction (check-definition, blocker medici-finance/assay#1281, open). No row fails.

**VERIFY: BLOCKED**

### Non-implementer verifier re-run — VERIFY: BLOCKED — 19/21 — 2026-10-21 claude-opus-5-5-verifier

The runner did not implement this brief: assay-verifier-app[bot] (claude-opus-5-5), dispatched by verify-desk against merged main 65a6418cd90c921153db509370212b1cdcd4f5e1. That commit has the deliverable squash b02ea2b35 (PR #1064) as an ancestor. `deskdispatch --check-verifier` exited 0 before any row ran.

Verification-Attestation: medici-finance/assay#3880 run=84ecaf31f93ea25f773753b689f1dfd42bcbed3acee1e526083ada23b259560f source=65a6418cd90c921153db509370212b1cdcd4f5e1 brief=docs/streams/desk-tools/brief-26-deskwt-prune-one-walk-and-a-lock.md model=claude-opus-5-5 tier=strong

Why this re-run: row 20's command was redefined. It is now pinned to the delivered squash b02ea2b35 and its base 86e3a434, so the consumers gate has a diff to read. This run is the first one under that definition.

How the rows were run: by hand, one after another, from the detached home at 65a6418c. Host: darwin/arm64, 16 cores, go1.27.2, git 2.56.0. The rows ran with GOPROXY=off, GOTOOLCHAIN=local, GOFLAGS empty and KUBECONFIG=/dev/null. TMPDIR and GOCACHE were a scratch directory outside any repo, with GIT_CEILING_DIRECTORIES set. HOME was an empty throwaway for every row except the row-19 attempt recorded below. The host was heavily shared: load average was 40–60 for the whole run, and other verifiers' mutation runs and containers were running at the same time. Every command below is exactly what ran.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | exit 0 | exit 0; no output (176s with a cold build cache) | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneRemovalSetUnchanged$' -count=1 -timeout 300s` | exit 0, removed set equals the recorded literal | exit 0; ok deskwt 4.623s | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneHoldsAll' -count=1` | exit 0, fail-closed: zero removed | exit 0; ok deskwt 4.351s | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestMergeGateBeforeStatus$' -count=1` | exit 0 | exit 0; ok deskwt 2.927s | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneSkips' -count=1 && go test ./cmd/deskwt/ -run '^TestPruneRefusesSharedCheckout$' -count=1` | exit 0, every existing hold | exit 0; ok deskwt 15.648s, then ok deskwt 1.929s | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 6 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneDryRunIsReadOnly$' -count=1` | exit 0 | exit 0; ok deskwt 2.010s | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 7 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneBatch' -count=1` | exit 0 | exit 0; ok deskwt 5.292s | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 8 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestRemove' -count=1` | exit 0 | exit 0; ok deskwt 21.830s | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 9 | `cd tools/desk && go test ./internal/gitcore/ -run '^TestOpenWithShares' -count=1 && go test ./internal/gitcore/ -run '^TestOpenStillBuilds' -count=1` | exit 0 | exit 0; ok gitcore 0.711s, then ok gitcore 0.572s | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 10 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneSingleton' -count=1 -timeout 120s` | exit 0 | exit 0; ok deskwt 17.730s | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 11 | `cd tools/desk && go run ./cmd/muhar -spec cmd/deskwt/prune-mutations.json` | exit 0, baseline GREEN, control CAUGHT, every mutation CAUGHT | could-not-check: no complete verdict. The spec now holds 55 mutations; the 2026-10-11 pass had 21. The run went for 2h11m on the loaded host and was interrupted at 20:01Z. Up to that point: "Harness healthy: baseline GREEN, positive control CAUGHT."; 35 CAUGHT and 0 NOT CAUGHT, including every mutation the brief names (empty-set continue, dropped clean-gate hold, demotion removed, dry-run reaching the real prune, TTL failing closed, ErrLockBusy read as free, lifetime supervisor lock). Two mutations did not compile against 65a6418c, before the interrupt. Both anchors still match, so neither is drift: each entry is mis-specified: "the ancestor walk goes back inside the candidate loop" (prune.go:836: undefined: roots; the anchor matches two sites and the harness replaces the first, ahead of where roots is declared; blocker: #2672, which also makes the 2026-10-11 CAUGHT for this entry suspect, since that runner counted any non-zero suite status as caught) and "the reap arm's pushed gate stops holding an unpushed worktree" (deadsession.go:139: declared and not used: pushed; the replacement removes the only read of pushed, so the mutant cannot compile; blocker: #4177). The other 18 could-not-mutate entries are the ones cut off by the interrupt. That is a second, independent cause: repairing the two entries does not give the row a complete run (blocker: #1927, row-11 run length). Afterwards the tree was clean | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 12 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneInterval' -count=1 -timeout 120s` | exit 0 | exit 0; ok deskwt 3.241s | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 13 | `cd tools/desk && DESKWT_PERF_N=50 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 600s` | exit 0, walk count 1, wall inside budget | exit 0; ok deskwt 9.351s | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 14 | `cd tools/desk && DESKWT_PERF_N=200 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 900s && DESKWT_PERF_N=600 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 900s` | exit 0, N=600 sweep under 30 s, walk count 1 | exit 0 on the second attempt (20:02Z): ok deskwt 9.033s, then ok deskwt 24.071s. The first attempt (17:37Z, load average about 48) exited 1: N=200 passed, then N=600 failed with "sweep over 600 worktrees took 31.537s, over the 30s budget (held 601, removed 0)". A supplemental -v probe of N=600 right after the second attempt printed "N=600 commits=5000 sweep=18.353s walks=1 removed=0 held=601". The 2026-10-11 pass measured 2.511s | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 15 | `cd tools/desk && DESKWT_PERF_N=200 go test ./cmd/deskwt/ -run '^TestConcurrentPrunes' -count=1 -timeout 900s` | exit 0, one sweeps, four held | exit 0; ok deskwt 25.294s | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 16 | `cd tools/desk && go test -bench '^BenchmarkPruneSweepPer' -benchtime 10x -run '^$' ./cmd/deskwt/ > /tmp/dt26-bench.out 2>&1; grep -q 'BenchmarkPruneSweepPerCandidate' /tmp/dt26-bench.out` | exit 0, benchmark exists and runs | exit 0; "BenchmarkPruneSweepPerCandidate-16 10 4850180933 ns/op 97.00 ms/candidate" (15.82 ms/candidate on 2026-10-11) | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 16b | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestLegacyPerCandidate' -count=1 -timeout 600s` | exit 0, baseline measured | exit 0; ok deskwt 51.920s | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 17 | `cd tools/desk && go test -timeout 600s ./cmd/deskwt/... ./internal/gitcore/... -count=1` | exit 0 | could-not-check: exit 1 because the run hit the row's own 600s timeout under host load average 50–60. gitcore passed (ok 99.295s). The deskwt package was stopped by "panic: test timed out after 10m0s" while TestSweepReapReusesAncestry was running, and no test had reported a failure before that (0 FAIL lines from tests). The 2026-10-11 pass finished deskwt in 228.508s (blocker: #4179) | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 18 | `cd tools/desk && gofmt -l cmd/deskwt internal/gitcore > /tmp/dt26-fmt.out; test ! -s /tmp/dt26-fmt.out` | exit 0 | exit 0; gofmt list is 0 bytes | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 19 | `cd statusgen && go run . --root .. --lint; echo $?` | 0 | printed 0; "LINT: PASS", with the operator HOME and its roster. The first attempt used the empty throwaway HOME and printed 1 ("configuration REFUSED; trust-dependent checks could-not-check", exit status 5). That came from the test environment, not from the tree | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 20 | `o=$(statusgen --root . --consumers --brief desk-tools/26 --delivery b02ea2b35e9f74b8331478dd59490bb7a8e906dd --base 86e3a434e5f17c4260efda925404b214df48ba74); s=$?; printf '%s\n' "$o"; test "$s" -eq 0 \|\| exit "$s"; case "$o" in *'summary: 3 corroborated, 0 disproved, 6 unchecked, 0 brief(s) claiming nothing'*) ;; *) exit 1;; esac` | exit 0; exact summary line | exit 0; "summary: 3 corroborated, 0 disproved, 6 unchecked, 0 brief(s) claiming nothing"; prune.go, deskwt.go and gitcore.go CORROBORATED fixed-here; the 6 out-of-scope routes UNCHECKED (statusgen v1.0.54) | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Totals: 19 of 21 rows meet their Expect, 2 are could-not-check (11 and 17), and 0 fail. Under its new definition, row 20 passes for the first time. Rows 2 (the SPOF set equality) and 3 (fail-closed) both pass.

Observation, not counted as a failure (question #4178): the timing-sensitive numbers are 6–12× worse than the 2026-10-11 pass. N=600 sweep: 18–31s against 2.5s. Benchmark: 97 against 15.8 ms per candidate. deskwt package: over 600s against 228s. This host ran at load average 40–60 on 16 cores throughout, so contention alone may explain it. But prune.go grew substantially since 08d194b1 (reap arm, admission hold, activity gate, cache pressure, bounded maintenance #2525), and this run cannot separate the two causes. The row-14 budget of 30s held on the second attempt with about 6s of margin. Two mutation-spec entries are mis-specified (not anchor drift: both anchors match), so the guards they target (the one-walk hoist under #2672 and the reap arm's pushed gate under #4177) are not being tested by row 11 at all.

Risk-bearing value. Enumeration over the non-test files of squash b02ea2b35 (prune.go, prunesingleton.go, deskwt.go, lockreclaim.go, gitcore.go) plus the test gates the Deliverables name, re-read at 65a6418c:

- merge-gate baseline ref = "refs/remotes/origin/main" @ tools/desk/cmd/deskwt/prune.go:765 (the one walk, in logOriginMain)
- defaultSingletonTTL = 10 * time.Minute @ tools/desk/cmd/deskwt/prunesingleton.go:57
- pruneStampSchema = "deskwt-prune-singleton-v1" @ prunesingleton.go:51
- stamp dir mode 0o700 @ prunesingleton.go:177; stamp file mode 0o600 @ prunesingleton.go:182
- lock-ttl flag default "0" @ prune.go:236; singleton-ttl flag default defaultSingletonTTL @ prune.go:248
- perf budget at N>=600 = 30 * time.Second @ tools/desk/cmd/deskwt/pruneperf_test.go:58; concurrency budget = 2*single + 5*time.Second @ pruneperf_test.go:332 (test gates only)

Ranked by irreversibility, the baseline ref is the only entry whose error deletes work. If it is wrong, an unmerged worktree reads as merged and is removed. Every other entry is a reversible knob for efficiency, hygiene or a test budget.

RISK-VALUE: DERIVED — merge-gate baseline = "refs/remotes/origin/main" @ tools/desk/cmd/deskwt/prune.go:765 — HEAD is an ancestor of origin/main exactly when HEAD is in the set of commits reachable from origin/main. logOriginMain resolves this ref once on the shared checkout and Logs from its tip, which includes the tip itself. The membership test is at prune.go:820. The ref is spelled in full because git resolves refs/heads/ ahead of refs/remotes/, so a stray local origin/main would otherwise become the baseline (#885). If the walk fails, the set is nil and every candidate is held as unverifiable (prune.go:805–809, sweep warning prune.go:880–883). Rows 2 and 3 pass at this commit, and row 11 CAUGHT the empty-set mutation before it was interrupted.

RISK-VALUE: DERIVED (reversible, ranks last) — defaultSingletonTTL = 10 * time.Minute @ tools/desk/cmd/deskwt/prunesingleton.go:57 — this is a debounce only. A wrong value delays or repeats a sweep by at most one TTL and never removes anything.

The brief is gate: model with all four risk answers no. Its status is left where it is.

**VERIFY: BLOCKED** — 19/21 rows pass on merged main 65a6418c, 0 fail. Row 11 is could-not-check: the 55-mutation run was interrupted after 2h11m with 35 CAUGHT and 0 NOT CAUGHT, and 2 spec entries are mis-specified and do not compile (blockers: #2672, #4177); the 18 unrun entries are a separate capacity cause (blocker: #1927). Row 17 is could-not-check: the package timed out at 600s under host load, with no test failure observed (blocker: #4179). Timing regression or host contention is open as question #4178.

### Non-implementer verifier re-run — VERIFY: BLOCKED — 17/21 — 2026-10-24 claude-opus-5-5-verifier

The runner did not implement this brief: assay-verifier-app[bot] (claude-opus-5-5), dispatched by verify-desk against merged main 385cb29e5682dcb9a886b298076f722b4823ec1b. That commit has the deliverable squash b02ea2b35 (PR #1064) as an ancestor. `deskdispatch --check-verifier` exited 0 before any row ran.

Verification-Attestation: medici-finance/assay#4319 run=beb8995d97923b53a5b401b85c3d242412fc99d8cabcc677d170a45aa17b6ad8 source=385cb29e5682dcb9a886b298076f722b4823ec1b brief=docs/streams/desk-tools/brief-26-deskwt-prune-one-walk-and-a-lock.md model=claude-opus-5-5 tier=strong

Prior attempts: 2026-10-21 on 65a6418c, BLOCKED 19/21 with 0 fail (rows 11 and 17 could-not-check). Why this re-run: since then, the two mis-specified mutation entries were fixed in commit 02ea06710 (PR #4267, blockers #2672 and #4177), and the `verifyrun --rows` selector shipped in v1.0.59. That lets this table be witnessed by the tools rather than by hand. There were no uncredited or discarded witness attempts: each lane ran once.

How the rows were run. Both tables below are the tools' own output, with nothing hand-edited. They ran in this order, with the home clean throughout:
1. Host `statusgen verifyrun --open-bracket` at 18:56:45Z.
2. Container `--sandbox=container-netns --rows 1,3,4,5,6,7,8,9,10,12,13,14,15,16,16b,17,18`, from 18:56Z to 19:15Z.
3. Host `--rows 2,11,19,20 --dry-run --timeout 8h`, captured outside the home, from 19:29Z to 21:52Z.
4. Host `--close-bracket` at 21:52:22Z.

The two selections are disjoint and together cover all 21 rows. The container is the release-pinned verify-runner v1.0.59 image (digest in the Container-Image line). It ran linux/amd64 under emulation on an arm64 darwin host, with `--network none --read-only --cap-drop ALL` and the home mounted read-only. Host load average (1-minute) was 32 at the bracket open, 11 at the container end, and 21 to 5 during the host lane.

Row 11 (55 mutations, including the two entries repaired under #2672 and #4177) ran to completion on the host in about 2h22m, uninterrupted, and exited 0. That exit code is the only retained witness. The harness output, including its Totals line, was not kept, and the harness exits 0 even when a mutation is NOT CAUGHT or could-not-mutate (#4359). So this run does not show that every mutation was caught. After this run, #4361 changed row 11's command to require the literal `Totals: 55 caught, 0 NOT CAUGHT, 0 could-not-mutate.` line. This run predates that check and is not evidence under it; a re-run under the new command is owed. The witness tool scores exit codes only, so the `pass exit=0` it records for row 11 below is not a pass under the row's Expect, and this block counts row 11 as could-not-check.

Container lane (closed bracket):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneHoldsAll' -count=1` | pass exit=0 sandbox=container-netns | sha256:7b0717e8fc95 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestMergeGateBeforeStatus$' -count=1` | pass exit=0 sandbox=container-netns | sha256:9e1c8b37ceea | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneSkips' -count=1 && go test ./cmd/deskwt/ -run '^TestPruneRefusesSharedCheckout$' -count=1` | pass exit=0 sandbox=container-netns | sha256:70aa73d6524a | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneDryRunIsReadOnly$' -count=1` | pass exit=0 sandbox=container-netns | sha256:b1f5c491f3f3 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneBatch' -count=1` | pass exit=0 sandbox=container-netns | sha256:0bfaefeb39f6 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestRemove' -count=1` | pass exit=0 sandbox=container-netns | sha256:880cd9ed8a66 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd tools/desk && go test ./internal/gitcore/ -run '^TestOpenWithShares' -count=1 && go test ./internal/gitcore/ -run '^TestOpenStillBuilds' -count=1` | pass exit=0 sandbox=container-netns | sha256:7048a9676fa8 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneSingleton' -count=1 -timeout 120s` | pass exit=0 sandbox=container-netns | sha256:8f1556dbce1f | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 12 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneInterval' -count=1 -timeout 120s` | pass exit=0 sandbox=container-netns | sha256:fac75b7bf7fd | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 13 | `cd tools/desk && DESKWT_PERF_N=50 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 600s` | pass exit=0 sandbox=container-netns | sha256:b9c9b9c6bce3 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 14 | `cd tools/desk && DESKWT_PERF_N=200 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 900s && DESKWT_PERF_N=600 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 900s` | pass exit=0 sandbox=container-netns | sha256:921bdb775c2f | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 15 | `cd tools/desk && DESKWT_PERF_N=200 go test ./cmd/deskwt/ -run '^TestConcurrentPrunes' -count=1 -timeout 900s` | pass exit=0 sandbox=container-netns | sha256:c6e7cbbf0893 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 16 | `cd tools/desk && go test -bench '^BenchmarkPruneSweepPer' -benchtime 10x -run '^$' ./cmd/deskwt/ > /tmp/dt26-bench.out 2>&1; grep -q 'BenchmarkPruneSweepPerCandidate' /tmp/dt26-bench.out` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 16b | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestLegacyPerCandidate' -count=1 -timeout 600s` | pass exit=0 sandbox=container-netns | sha256:d5c294f74520 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 17 | `cd tools/desk && go test -timeout 600s ./cmd/deskwt/... ./internal/gitcore/... -count=1` | fail exit=1 sandbox=container-netns | sha256:ffede75200b6 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 18 | `cd tools/desk && gofmt -l cmd/deskwt internal/gitcore > /tmp/dt26-fmt.out; test ! -s /tmp/dt26-fmt.out` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:98685d6717c495aecd71bc3b8cb782aa9cc7dc0ac1332b029f60eb2c3a1c1104
Container-Image: sha256:c2e32c7ff99f67951174e8ff835d928d8999b1bb2ecdc7c63d3775528178640e
Verification-Attestation: medici-finance/assay#4319 run=beb8995d97923b53a5b401b85c3d242412fc99d8cabcc677d170a45aa17b6ad8 source=385cb29e5682dcb9a886b298076f722b4823ec1b brief=docs/streams/desk-tools/brief-26-deskwt-prune-one-walk-and-a-lock.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=beb8995d97923b53a5b401b85c3d242412fc99d8cabcc677d170a45aa17b6ad8 source=385cb29e5682dcb9a886b298076f722b4823ec1b tree=sha256:98685d6717c495aecd71bc3b8cb782aa9cc7dc0ac1332b029f60eb2c3a1c1104 witness=sha256:ed5257ea779c2b29fdaac5becd52cd9b00f5e02c622c6c052de247a48d10af71 opened=2026-10-24T18:56:45Z closed=2026-10-24T21:52:22Z before=receipt after=check-verifier

Host lane (rows 2, 11, 19 and 20, same source and Runner). Rows 2 and 11 are class `check`. Rows 19 and 20 are `check:ci` and need this repository's git history, which the container does not have, because a linked worktree's git directory is not mounted:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 2 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneRemovalSetUnchanged$' -count=1 -timeout 300s` | pass exit=0 | sha256:acb930734411 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 11 | `cd tools/desk && go run ./cmd/muhar -spec cmd/deskwt/prune-mutations.json` | pass exit=0 | sha256:582f64ed3f0a | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 19 | `cd statusgen && go run . --root .. --lint; echo $?` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 20 | `o=$(statusgen --root . --consumers --brief desk-tools/26 --delivery b02ea2b35e9f74b8331478dd59490bb7a8e906dd --base 86e3a434e5f17c4260efda925404b214df48ba74); s=$?; printf '%s\n' "$o"; test "$s" -eq 0` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4319 run=beb8995d97923b53a5b401b85c3d242412fc99d8cabcc677d170a45aa17b6ad8 source=385cb29e5682dcb9a886b298076f722b4823ec1b brief=docs/streams/desk-tools/brief-26-deskwt-prune-one-walk-and-a-lock.md model=claude-opus-5-5 tier=strong

Totals: 17 of 21 rows meet their Expect: 16 in the container and row 2 on the host, including the SPOF row 2 and the fail-closed row 3. 0 rows fail on the deliverable. Four rows have no pass witness. Row 11 is could-not-check, and rows 17, 19 and 20 are instrument limits:

- **Row 11** (host witness `pass exit=0`, counted could-not-check). The tool scored the exit code only. The harness output and its Totals line were not retained, and the harness exits 0 even when a mutation is NOT CAUGHT or could-not-mutate (#4359), so the run shows no per-mutation result. #4361 later changed the row to require the literal Totals line; a re-run under that command is owed.

- **Row 17** (container witness `fail exit=1`). This is could-not-check, not a deliverable failure. A diagnostic re-run of the same command in the same image, with the home read-only, from 19:17Z to 19:27:40Z (1-minute load 12 to 34), printed `ok  github.com/medici-finance/assay/tools/desk/internal/gitcore 168.599s`. In the same run, deskwt stopped at `panic: test timed out after 10m0s` while `TestRoleInitNonGitHubOriginHostRefusedBeforeMint/hostref2` was running. It had four FAIL lines, all in role-init and identity tests from later work, none in this brief's files:
  - `TestAddWithoutRoleClearsIdentitySoItNeverInherits`: "effective user.name in a no-role worktree = "assay-verifier-app[bot]", want empty". The runner injects the commit identity through `GIT_CONFIG_*` environment, which outranks any worktree config.
  - `TestRoleInitProvisionsLockedWorktreeWithScopedIdentity`, `TestRoleInitLeavesSharedCheckoutConfigAndIndexUntouched` and `TestRoleInitWiresHostScopedCredentialHelper`: each reported "fatal: gitfile does not point to a valid repository: /work/.git". The mounted home's `.git` file points at an unmounted git directory.

  The 600s timeout under emulation is the capacity cause already open as #4179.
- **Row 19** (host `could-not-run`). The host has no `unshare --net`. A read-only container probe of the same command printed `LINT: FAIL 10 problem(s)`, every one of the form "this repo's git history was unreadable, so the consumed-release-fragment exemption (#722) could NOT be evaluated", which is could-not-check by the tool's own words. The row passed on 2026-10-21 with history available.
- **Row 20** (host `could-not-run`). The host has no `unshare --net`, and the row needs the delivery and base commits, which the container cannot read. The row passed on 2026-10-21 on 65a6418c, and the delivered snapshot it reads is immutable.

Risk-bearing value. Enumeration over the non-test files of squash b02ea2b35 (prune.go, prunesingleton.go, deskwt.go, lockreclaim.go, gitcore.go) plus the test gates the Deliverables name, re-read at 385cb29e:

- merge-gate baseline ref = "refs/remotes/origin/main" @ tools/desk/cmd/deskwt/prune.go:765 (the one walk, in logOriginMain)
- fresh-at-tip guard ref = "refs/remotes/origin/main" @ tools/desk/cmd/deskwt/prune.go:1498 (headAtOriginMainTip, deliberately independent of the walk)
- defaultSingletonTTL = 10 * time.Minute @ tools/desk/cmd/deskwt/prunesingleton.go:57
- pruneStampSchema = "deskwt-prune-singleton-v1" @ prunesingleton.go:51
- stamp dir mode 0o700 @ prunesingleton.go:177; stamp file mode 0o600 @ prunesingleton.go:182
- lock-ttl flag default "0" @ prune.go:236; singleton-ttl flag default defaultSingletonTTL @ prune.go:248
- perf budget at N>=600 = 30 * time.Second @ tools/desk/cmd/deskwt/pruneperf_test.go:58; concurrency budget = 2*single + 5*time.Second @ pruneperf_test.go:332 (test gates only)

Ranked by irreversibility:
1. The merge-gate baseline ref ranks first. If it is wrong, an unmerged worktree reads as merged and is removed.
2. The fresh-at-tip guard ref ranks second. It is a hold, so an error there removes a worktree only if the merge gate also errs.
3. Every other entry ranks below these: each is a reversible knob for efficiency, hygiene or a test budget.

RISK-VALUE: DERIVED — merge-gate baseline = "refs/remotes/origin/main" @ tools/desk/cmd/deskwt/prune.go:765 — HEAD is an ancestor of origin/main exactly when HEAD is in the set of commits reachable from origin/main. logOriginMain resolves this ref once on the shared checkout and Logs from its tip, which includes the tip itself (prune.go:760–771). The set is built at prune.go:745–749, and the membership test is at prune.go:819. The ref is spelled in full because git resolves refs/heads/ ahead of refs/remotes/, so a stray local origin/main would otherwise become the baseline (#885). If the walk fails, the set is nil and every candidate is held as unverifiable (prune.go:806–810, sweep warning prune.go:880–883). At this commit, rows 2 and 3 pass. Row 11 exited 0, but with no retained Totals line it does not show that the inversion and empty-set mutations were caught.

RISK-VALUE: DERIVED (second) — fresh-at-tip guard = "refs/remotes/origin/main" @ tools/desk/cmd/deskwt/prune.go:1498 — this is the same full spelling, read per candidate from the worktree's own refs. The comment at prune.go:1478–1487 records why it must not be fed from the walk: two layers that fail on one signal are one layer. A resolve failure returns Unverifiable, which holds the worktree (prune.go:1499–1500).

RISK-VALUE: DERIVED (reversible, ranks last) — defaultSingletonTTL = 10 * time.Minute @ tools/desk/cmd/deskwt/prunesingleton.go:57 — this is a debounce only. A wrong value delays or repeats a sweep by at most one TTL and never removes anything.

The brief is gate: model with all four risk answers no. Its status is left where it is.

**VERIFY: BLOCKED** — 17/21 rows pass on merged main 385cb29e, 0 fail on the deliverable. Row 11 is could-not-check: its 55-mutation run exited 0, but its Totals line was not retained, so it does not show every mutation caught, and it predates the stricter row 11 command from #4361. Rows 17, 19 and 20 are instrument limits. Row 17: the container lane leaks the runner identity through `GIT_CONFIG_*`, the git directory is unmounted, and the package hit the 600s timeout under emulation (blocker: #4179). Rows 19 and 20: the darwin host has no `unshare --net`, and the container has no git history.

### Non-implementer verifier re-run — VERIFY: BLOCKED — 20/21 — 2026-10-25 claude-opus-5-5-verifier

The runner did not implement this brief: assay-verifier-app[bot] (claude-opus-5-5), dispatched by verify-desk against merged main 4aeecf71626f485c4938dec3165e887a2f74e20d. That commit has the deliverable squash b02ea2b35 (PR #1064) as an ancestor. `deskdispatch --check-verifier` exited 0 before any row ran.

Verification-Attestation: medici-finance/assay#4433 run=c5acf6df1fa430b6d3b64451c4f0e61c63d957eee4427e49a1b32de6d47dd087 source=4aeecf71626f485c4938dec3165e887a2f74e20d brief=docs/streams/desk-tools/brief-26-deskwt-prune-one-walk-and-a-lock.md model=claude-opus-5-5 tier=strong

Why this re-run: the 2026-10-24 pass ended blocked at 17/21. Its row 11 run exited 0, but the Totals line was not retained, so it did not show every mutation caught. PR #4361 (commit d0f322021, merged 2026-10-25) then repaired row 11: the harness now fails closed and fails fast, and the row's command requires the literal line `Totals: 55 caught, 0 NOT CAUGHT, 0 could-not-mutate.`. This pass witnesses row 11 under that repaired command and witnesses every other row fresh. Row 11 was amended after delivery by an ordinary repair PR, not a `wrote-to-the-test` change. The amended row is stricter than the original (the original exited 0 on the harness's exit code alone), so a pass under it also meets the original row.

How the rows were run. Every table below is the tools' own output, with nothing hand-edited. The home stayed clean throughout (`git status --short` empty before and after each lane, including after row 11's in-place mutations). The order was:
1. Bracket 1: host `--open-bracket` at 18:46:41Z, then container `--sandbox=container-netns --rows 1,2,3,4,5,6,7,8,9,10,12,13,14,15,16,16b,17,18,19,20` from 18:46Z to 19:05Z, then host `--close-bracket`.
2. Bracket 2: host `--open-bracket` at 19:09:04Z, then container `--rows 19` with the operator's roster file mounted read-only at the container's `$HOME/.config/assay/roster.env`, then host `--close-bracket`.
3. Host `--rows 11 --dry-run --timeout 8h`, captured outside the home, from 19:17Z to 20:57Z, exit 0.
4. Bracket 3: host `--open-bracket` at 20:58:23Z, then container `--rows 17` (a retry at lower host load), then host `--close-bracket`.

All three container lanes used the release-pinned verify-runner v1.0.60 image (digest in each Container-Image line). It ran as linux/amd64 under emulation on an arm64 darwin host, with `--network none --cap-drop ALL`, default seccomp, the home mounted read-only, and this repository's git directory mounted read-only at its own path, so rows that read history could run. Go modules came from a pre-filled cache mounted read-only, with `GOPROXY=off`. `--bracket-digest` reports `verdict=match` for all three Admission-Bracket lines. Host 1-minute load average was 22 to 97 during bracket 1, 20 to 60 during row 11, and 9 to 18 during bracket 3.

Container lane, bracket 1:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneRemovalSetUnchanged$' -count=1 -timeout 300s` | pass exit=0 sandbox=container-netns | sha256:88b09ad3f215 | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneHoldsAll' -count=1` | pass exit=0 sandbox=container-netns | sha256:5e002a8b1529 | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestMergeGateBeforeStatus$' -count=1` | pass exit=0 sandbox=container-netns | sha256:f40687b70a92 | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneSkips' -count=1 && go test ./cmd/deskwt/ -run '^TestPruneRefusesSharedCheckout$' -count=1` | pass exit=0 sandbox=container-netns | sha256:1a942e1c9dc5 | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneDryRunIsReadOnly$' -count=1` | pass exit=0 sandbox=container-netns | sha256:5e5f3f34e776 | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneBatch' -count=1` | pass exit=0 sandbox=container-netns | sha256:8617130617cd | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestRemove' -count=1` | pass exit=0 sandbox=container-netns | sha256:48e21edec359 | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd tools/desk && go test ./internal/gitcore/ -run '^TestOpenWithShares' -count=1 && go test ./internal/gitcore/ -run '^TestOpenStillBuilds' -count=1` | pass exit=0 sandbox=container-netns | sha256:9e72cd6ae17c | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneSingleton' -count=1 -timeout 120s` | pass exit=0 sandbox=container-netns | sha256:06e1de5816a7 | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) |
| 12 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneInterval' -count=1 -timeout 120s` | pass exit=0 sandbox=container-netns | sha256:d8233c5796b9 | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) |
| 13 | `cd tools/desk && DESKWT_PERF_N=50 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 600s` | pass exit=0 sandbox=container-netns | sha256:addb49e70be1 | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) |
| 14 | `cd tools/desk && DESKWT_PERF_N=200 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 900s && DESKWT_PERF_N=600 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 900s` | pass exit=0 sandbox=container-netns | sha256:12a94807fc29 | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) |
| 15 | `cd tools/desk && DESKWT_PERF_N=200 go test ./cmd/deskwt/ -run '^TestConcurrentPrunes' -count=1 -timeout 900s` | pass exit=0 sandbox=container-netns | sha256:0595dc9a8973 | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) |
| 16 | `cd tools/desk && go test -bench '^BenchmarkPruneSweepPer' -benchtime 10x -run '^$' ./cmd/deskwt/ > /tmp/dt26-bench.out 2>&1; grep -q 'BenchmarkPruneSweepPerCandidate' /tmp/dt26-bench.out` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) |
| 16b | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestLegacyPerCandidate' -count=1 -timeout 600s` | pass exit=0 sandbox=container-netns | sha256:637d004be0db | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) |
| 17 | `cd tools/desk && go test -timeout 600s ./cmd/deskwt/... ./internal/gitcore/... -count=1` | fail exit=1 sandbox=container-netns | sha256:fa0cfbdbddae | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) |
| 18 | `cd tools/desk && gofmt -l cmd/deskwt internal/gitcore > /tmp/dt26-fmt.out; test ! -s /tmp/dt26-fmt.out` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) |
| 19 | `cd statusgen && go run . --root .. --lint; echo $?` | pass exit=0 sandbox=container-netns | sha256:9883240f29ee | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) |
| 20 | `o=$(statusgen --root . --consumers --brief desk-tools/26 --delivery b02ea2b35e9f74b8331478dd59490bb7a8e906dd --base 86e3a434e5f17c4260efda925404b214df48ba74); s=$?; printf '%s\n' "$o"; test "$s" -eq 0` | pass exit=0 sandbox=container-netns | sha256:12e196106941 | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:a07e75f04765056587c4d1b5228d8647eb4d6d748c59de45455e49d7f64f4e9b
Container-Image: sha256:4cb3f129e71ff220b62995c587fd4716e2c73c81267bc583df3f544f6fdd5ae3
Verification-Attestation: medici-finance/assay#4433 run=c5acf6df1fa430b6d3b64451c4f0e61c63d957eee4427e49a1b32de6d47dd087 source=4aeecf71626f485c4938dec3165e887a2f74e20d brief=docs/streams/desk-tools/brief-26-deskwt-prune-one-walk-and-a-lock.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=c5acf6df1fa430b6d3b64451c4f0e61c63d957eee4427e49a1b32de6d47dd087 source=4aeecf71626f485c4938dec3165e887a2f74e20d tree=sha256:a07e75f04765056587c4d1b5228d8647eb4d6d748c59de45455e49d7f64f4e9b witness=sha256:2336617118718fa8709834e97691abb7481ca24c4220ce6adde2dd022bfc4ae5 opened=2026-10-25T18:46:41Z closed=2026-10-25T19:06:00Z before=receipt after=check-verifier

Container lane, bracket 2 (row 19 with the operator roster present; this witness supersedes row 19 in bracket 1, see below):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 19 | `cd statusgen && go run . --root .. --lint; echo $?` | pass exit=0 sandbox=container-netns | sha256:9acb66b081d4 | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:a07e75f04765056587c4d1b5228d8647eb4d6d748c59de45455e49d7f64f4e9b
Container-Image: sha256:4cb3f129e71ff220b62995c587fd4716e2c73c81267bc583df3f544f6fdd5ae3
Verification-Attestation: medici-finance/assay#4433 run=c5acf6df1fa430b6d3b64451c4f0e61c63d957eee4427e49a1b32de6d47dd087 source=4aeecf71626f485c4938dec3165e887a2f74e20d brief=docs/streams/desk-tools/brief-26-deskwt-prune-one-walk-and-a-lock.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=c5acf6df1fa430b6d3b64451c4f0e61c63d957eee4427e49a1b32de6d47dd087 source=4aeecf71626f485c4938dec3165e887a2f74e20d tree=sha256:a07e75f04765056587c4d1b5228d8647eb4d6d748c59de45455e49d7f64f4e9b witness=sha256:922c10fb81e1f0ce4d2c2285d18cd0f421d3458c6b377efec99033802b6e6b34 opened=2026-10-25T19:09:04Z closed=2026-10-25T19:11:16Z before=receipt after=check-verifier

Container lane, bracket 3 (row 17 retry):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 17 | `cd tools/desk && go test -timeout 600s ./cmd/deskwt/... ./internal/gitcore/... -count=1` | fail exit=1 sandbox=container-netns | sha256:dbc292c7bade | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:a07e75f04765056587c4d1b5228d8647eb4d6d748c59de45455e49d7f64f4e9b
Container-Image: sha256:4cb3f129e71ff220b62995c587fd4716e2c73c81267bc583df3f544f6fdd5ae3
Verification-Attestation: medici-finance/assay#4433 run=c5acf6df1fa430b6d3b64451c4f0e61c63d957eee4427e49a1b32de6d47dd087 source=4aeecf71626f485c4938dec3165e887a2f74e20d brief=docs/streams/desk-tools/brief-26-deskwt-prune-one-walk-and-a-lock.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=c5acf6df1fa430b6d3b64451c4f0e61c63d957eee4427e49a1b32de6d47dd087 source=4aeecf71626f485c4938dec3165e887a2f74e20d tree=sha256:a07e75f04765056587c4d1b5228d8647eb4d6d748c59de45455e49d7f64f4e9b witness=sha256:26ab4ab553405e1ad4af1aa8474c187f85a618c08b9dacd85f1346c70ef4f79d opened=2026-10-25T20:58:23Z closed=2026-10-25T21:07:36Z before=receipt after=check-verifier

Host lane (row 11, class `check +mutation`; the harness mutates the tree in place, so it cannot run on the read-only container mount):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 11 | `cd tools/desk && report=$(go run ./cmd/muhar -spec cmd/deskwt/prune-mutations.json) && printf '%s\n' "$report" && case "$report" in *'Totals: 55 caught, 0 NOT CAUGHT, 0 could-not-mutate.'*) ;; *) exit 1;; esac` | pass exit=0 | sha256:22d7325d6a33 | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4433 run=c5acf6df1fa430b6d3b64451c4f0e61c63d957eee4427e49a1b32de6d47dd087 source=4aeecf71626f485c4938dec3165e887a2f74e20d brief=docs/streams/desk-tools/brief-26-deskwt-prune-one-walk-and-a-lock.md model=claude-opus-5-5 tier=strong

Totals: 20 of 21 rows meet their Expect, including the SPOF row 2, the fail-closed row 3 and the mutation row 11. 0 rows fail on the deliverable. Row 17 has no pass witness and is could-not-check.

- **Row 11** (host witness `pass exit=0`, counted pass). The witnessed command is the repaired one. It exits 0 only if the harness succeeds AND its report contains the literal `Totals: 55 caught, 0 NOT CAUGHT, 0 could-not-mutate.`. Otherwise the `case` arm runs `exit 1`. So exit 0 under this command shows that the line was printed: all 55 mutations caught, none NOT CAUGHT, none could-not-mutate. The run took about 1h40m.
- **Row 17** (container witness `fail exit=1` in brackets 1 and 3, counted could-not-check). This is not a deliverable failure:
  - A same-image diagnostic of bracket 1's run printed `panic: test timed out after 10m0s` in deskwt while `TestMaintenanceWarningsFailClosed` was running. gitcore passed in 239.080s. No test reported a failure.
  - Bracket 3 ran row 17 alone, at load average 9 to 18, and hit the same 600s budget.
  - The 2026-10-24 git-identity and unmounted-gitdir failures did not recur.
  - The cause is capacity: under amd64 emulation the deskwt package does not fit the row's fixed `-timeout 600s` (open as #4179, with this pass's data attached).
  - A supplemental native host run of the same command passed: `ok .../cmd/deskwt 445.602s`, `ok .../internal/gitcore 89.181s`, exit 0, at load average 50 to 65. It is not credited: it ran outside any network-off sandbox, and check:ci requires one.
- **Row 19** (counted pass on bracket 2). Bracket 1's row-19 witness reads `pass exit=0`, but the row always exits 0 because of its trailing `echo $?`, so only its output can decide it. A same-image capture of that output hashes to the witnessed `sha256:9883240f29ee`. It ends `LINT: REFUSED configuration; trust-dependent checks could-not-check` and prints `1`, because the container had no roster file. Bracket 1's row 19 is therefore not counted. Bracket 2 re-ran row 19 with the operator's roster mounted read-only. A same-image capture of that configuration hashes to the witnessed `sha256:9acb66b081d4`. It ends `LINT: PASS` and prints `0`, which meets the Expect.
- **Row 20** (container witness `pass exit=0`, counted pass with a hand binding). The row's Command cell contains unescaped `||`, so table readers end the cell at the first `||`. The witnessed command is truncated before the exact-summary check, and `deskpathguard rederive` reports could-not-check for this table ("row width differs from header"). Filed as #4453. The full command, run separately in the same image and home, exited 0 and printed `summary: 3 corroborated, 0 disproved, 6 unchecked, 0 brief(s) claiming nothing`. Its output hashes to the witnessed `sha256:12e196106941`, so the witnessed run printed that exact summary.
- **Rows 1, 16 and 18** print nothing on success (`sha256:e3b0c44298fc` is the empty-output hash), which is what their Expect asks for.

Risk-bearing value. Enumeration over the non-test files of squash b02ea2b35 (prune.go, prunesingleton.go, deskwt.go, lockreclaim.go, gitcore.go) plus the test gates the Deliverables name. Re-read at 4aeecf71: the deskwt changes since the previous pass do not touch these lines.

- merge-gate baseline ref = "refs/remotes/origin/main" @ tools/desk/cmd/deskwt/prune.go:765 (the one walk, in logOriginMain)
- fresh-at-tip guard ref = "refs/remotes/origin/main" @ tools/desk/cmd/deskwt/prune.go:1498 (headAtOriginMainTip, deliberately independent of the walk)
- defaultSingletonTTL = 10 * time.Minute @ tools/desk/cmd/deskwt/prunesingleton.go:57
- pruneStampSchema = "deskwt-prune-singleton-v1" @ prunesingleton.go:51
- stamp dir mode 0o700 @ prunesingleton.go:177; stamp file mode 0o600 @ prunesingleton.go:182
- lock-ttl flag default "0" @ prune.go:236; singleton-ttl flag default defaultSingletonTTL @ prune.go:248
- perf budget at N>=600 = 30 * time.Second @ tools/desk/cmd/deskwt/pruneperf_test.go:58; concurrency budget = 2*single + 5*time.Second @ pruneperf_test.go:332 (test gates only)

Ranked by irreversibility:
1. The merge-gate baseline ref ranks first. If it is wrong, an unmerged worktree reads as merged and is removed.
2. The fresh-at-tip guard ref ranks second. It is a hold, so an error there removes a worktree only if the merge gate also errs.
3. Every other entry ranks below these: each is a reversible knob for efficiency, hygiene or a test budget.

RISK-VALUE: DERIVED — merge-gate baseline = "refs/remotes/origin/main" @ tools/desk/cmd/deskwt/prune.go:765 — HEAD is an ancestor of origin/main exactly when HEAD is in the set of commits reachable from origin/main. logOriginMain resolves this ref once and Logs from its tip, which includes the tip itself. The set is built at prune.go:745–749 and the membership test is at prune.go:819. A nil set returns Unverifiable, which holds every candidate (prune.go:806–810). The full spelling avoids a stray local branch named `origin/main` (#885). Row 2 (set equality over a mixed fixture) and row 11's inversion and empty-set mutations bind it.

RISK-VALUE: DERIVED (second) — fresh-at-tip guard = "refs/remotes/origin/main" @ tools/desk/cmd/deskwt/prune.go:1498 — the same full spelling, read per candidate from the worktree's own refs, not from the walk. A resolve failure returns Unverifiable, which holds the worktree (prune.go:1500).

RISK-VALUE: DERIVED (reversible, ranks last) — defaultSingletonTTL = 10 * time.Minute @ tools/desk/cmd/deskwt/prunesingleton.go:57 — this is a debounce only. A wrong value delays or repeats a sweep by at most one TTL and never removes anything.

The brief is gate: model with all four risk answers no. Its status is left where it is.

**VERIFY: BLOCKED** — 20/21 rows pass on merged main 4aeecf71, 0 fail on the deliverable. Row 11 passes under the repaired command: all 55 mutations caught. Row 17 is could-not-check: the deskwt package exceeds the row's fixed 600s budget under amd64 emulation (#4179). A native supplemental run passed in 445.602s but cannot be credited for a check:ci row. Row 20 passes, with its summary check bound by output hash because of the unescaped-pipe defect #4453.
### Non-implementer verifier re-run — VERIFY: BLOCKED — 20/21 — 2026-10-27 claude-opus-5-5-verifier

The runner did not implement this brief: assay-verifier-app[bot] (claude-opus-5-5), dispatched by verify-desk against merged main f955b97c11baa28bc9a0b8c1b378111311e5c2a4. That commit has the deliverable squash b02ea2b35 (PR #1064) as an ancestor. `deskdispatch --check-verifier` exited 0 before any row ran and again before this Evidence landed.

Verification-Attestation: medici-finance/assay#4668 run=3778a789cf8dae210025eeac7ad0f245b83d557d1553b6b4b73be4bd2f14e370 source=f955b97c11baa28bc9a0b8c1b378111311e5c2a4 brief=docs/streams/desk-tools/brief-26-deskwt-prune-one-walk-and-a-lock.md model=claude-opus-5-5 tier=strong

Why this re-run: the 2026-10-25 pass ended blocked at 20/21 on row 17 alone. Since then deskwt moved to Cobra and declared bindings (#4460, commit 0d9cb2522), and the flag declarations moved out of prune.go into cli.go. This pass witnesses every row fresh at f955b97 and re-reads the risk-bearing literals there.

How the rows were run. Every table below is the tools' own output, with nothing hand-edited. The home stayed clean throughout (`git status --porcelain` empty before and after each lane, including after row 11's in-place mutations). The order was:
1. Bracket A: host `--open-bracket` at 21:48:24Z, then container `--sandbox=container-netns --rows 1,2,3,4,5,6,7,8,9,10,12,13,14,15,16,16b,17,18,19,20` from 21:48Z to 22:10Z, then host `--close-bracket` at 22:10:35Z. The operator's roster file was mounted read-only at the container's `$HOME/.config/assay/roster.env` from the start, so row 19 needed no second bracket.
2. Host `--rows 11 --dry-run --timeout 8h`, captured outside the home, from 22:24:54Z to 23:33:10Z, exit 0 (an earlier attempt under a throwaway HOME refused at admission before any row ran; see below).

The container lane used the release-pinned verify-runner v1.0.63 image (digest in the Container-Image line; the release's image-reference asset matched its published checksum). It ran as linux/amd64 under emulation on an arm64 darwin host, with `--network none --cap-drop ALL`, default seccomp, the home mounted read-only, and this repository's git directory mounted read-only at its own path, so rows that read history could run. Go modules came from a pre-filled cache mounted read-only, with `GOPROXY=off`. `--bracket-digest` reports `verdict=match` for the Admission-Bracket line. Host 1-minute load average was 8 to 12 during bracket A.

The host row-11 lane ran with the operator's normal HOME. The host verifyrun's pre-work admission mints the verifier App installation token, which a throwaway HOME holding only the roster cannot do; the first attempt under a throwaway HOME refused before executing any row ("no App ID for App \"verifier-app\""). The deskwt test package's own TestMain re-points HOME to a temporary fixture roster for every test, and the Go build and module caches were the real ones, set explicitly.

Container lane, bracket A:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ f955b97c11ba (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneRemovalSetUnchanged$' -count=1 -timeout 300s` | pass exit=0 sandbox=container-netns | sha256:4f7f308455ef | 2026-10-27 | assay-verifier-app[bot] @ f955b97c11ba (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneHoldsAll' -count=1` | pass exit=0 sandbox=container-netns | sha256:8a093a18bdac | 2026-10-27 | assay-verifier-app[bot] @ f955b97c11ba (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestMergeGateBeforeStatus$' -count=1` | pass exit=0 sandbox=container-netns | sha256:67a96b8980e0 | 2026-10-27 | assay-verifier-app[bot] @ f955b97c11ba (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneSkips' -count=1 && go test ./cmd/deskwt/ -run '^TestPruneRefusesSharedCheckout$' -count=1` | pass exit=0 sandbox=container-netns | sha256:54619fb38bdf | 2026-10-27 | assay-verifier-app[bot] @ f955b97c11ba (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneDryRunIsReadOnly$' -count=1` | pass exit=0 sandbox=container-netns | sha256:77d752377ed3 | 2026-10-27 | assay-verifier-app[bot] @ f955b97c11ba (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneBatch' -count=1` | pass exit=0 sandbox=container-netns | sha256:31eaf63673c1 | 2026-10-27 | assay-verifier-app[bot] @ f955b97c11ba (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestRemove' -count=1` | pass exit=0 sandbox=container-netns | sha256:56f1c785520d | 2026-10-27 | assay-verifier-app[bot] @ f955b97c11ba (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd tools/desk && go test ./internal/gitcore/ -run '^TestOpenWithShares' -count=1 && go test ./internal/gitcore/ -run '^TestOpenStillBuilds' -count=1` | pass exit=0 sandbox=container-netns | sha256:6fa07efaa61e | 2026-10-27 | assay-verifier-app[bot] @ f955b97c11ba (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneSingleton' -count=1 -timeout 120s` | pass exit=0 sandbox=container-netns | sha256:a09b47e05f28 | 2026-10-27 | assay-verifier-app[bot] @ f955b97c11ba (on-behalf-of human:ian) (forge-identity) |
| 12 | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneInterval' -count=1 -timeout 120s` | pass exit=0 sandbox=container-netns | sha256:91731a834dd6 | 2026-10-27 | assay-verifier-app[bot] @ f955b97c11ba (on-behalf-of human:ian) (forge-identity) |
| 13 | `cd tools/desk && DESKWT_PERF_N=50 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 600s` | pass exit=0 sandbox=container-netns | sha256:4bffa1301e93 | 2026-10-27 | assay-verifier-app[bot] @ f955b97c11ba (on-behalf-of human:ian) (forge-identity) |
| 14 | `cd tools/desk && DESKWT_PERF_N=200 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 900s && DESKWT_PERF_N=600 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 900s` | pass exit=0 sandbox=container-netns | sha256:d223472d13a4 | 2026-10-27 | assay-verifier-app[bot] @ f955b97c11ba (on-behalf-of human:ian) (forge-identity) |
| 15 | `cd tools/desk && DESKWT_PERF_N=200 go test ./cmd/deskwt/ -run '^TestConcurrentPrunes' -count=1 -timeout 900s` | pass exit=0 sandbox=container-netns | sha256:f98988806c7d | 2026-10-27 | assay-verifier-app[bot] @ f955b97c11ba (on-behalf-of human:ian) (forge-identity) |
| 16 | `cd tools/desk && go test -bench '^BenchmarkPruneSweepPer' -benchtime 10x -run '^$' ./cmd/deskwt/ > /tmp/dt26-bench.out 2>&1; grep -q 'BenchmarkPruneSweepPerCandidate' /tmp/dt26-bench.out` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ f955b97c11ba (on-behalf-of human:ian) (forge-identity) |
| 16b | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestLegacyPerCandidate' -count=1 -timeout 600s` | pass exit=0 sandbox=container-netns | sha256:b2c19d6d3df4 | 2026-10-27 | assay-verifier-app[bot] @ f955b97c11ba (on-behalf-of human:ian) (forge-identity) |
| 17 | `cd tools/desk && go test -timeout 600s ./cmd/deskwt/... ./internal/gitcore/... -count=1` | fail exit=1 sandbox=container-netns | sha256:3ae647dbcc87 | 2026-10-27 | assay-verifier-app[bot] @ f955b97c11ba (on-behalf-of human:ian) (forge-identity) |
| 18 | `cd tools/desk && gofmt -l cmd/deskwt internal/gitcore > /tmp/dt26-fmt.out; test ! -s /tmp/dt26-fmt.out` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ f955b97c11ba (on-behalf-of human:ian) (forge-identity) |
| 19 | `cd statusgen && go run . --root .. --lint; echo $?` | pass exit=0 sandbox=container-netns | sha256:8a98c18c7f17 | 2026-10-27 | assay-verifier-app[bot] @ f955b97c11ba (on-behalf-of human:ian) (forge-identity) |
| 20 | `o=$(statusgen --root . --consumers --brief desk-tools/26 --delivery b02ea2b35e9f74b8331478dd59490bb7a8e906dd --base 86e3a434e5f17c4260efda925404b214df48ba74); s=$?; printf '%s\n' "$o"; test "$s" -eq 0` | pass exit=0 sandbox=container-netns | sha256:5fd23df1e032 | 2026-10-27 | assay-verifier-app[bot] @ f955b97c11ba (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:e246e2605cc9a34e8ac832e904102d03825ea85faaad825f6eb2eb0f8dc761bc
Container-Image: sha256:3a3cb3913f1311f1a9bf13ddd53c4f27361566f74ac90c89f78991e5452644b8
Verification-Attestation: medici-finance/assay#4668 run=3778a789cf8dae210025eeac7ad0f245b83d557d1553b6b4b73be4bd2f14e370 source=f955b97c11baa28bc9a0b8c1b378111311e5c2a4 brief=docs/streams/desk-tools/brief-26-deskwt-prune-one-walk-and-a-lock.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=3778a789cf8dae210025eeac7ad0f245b83d557d1553b6b4b73be4bd2f14e370 source=f955b97c11baa28bc9a0b8c1b378111311e5c2a4 tree=sha256:e246e2605cc9a34e8ac832e904102d03825ea85faaad825f6eb2eb0f8dc761bc witness=sha256:f38f2c38a3f5983959674b0b86a888fa66ae20a5e9011748dc0f79839585a5bb opened=2026-10-27T21:48:24Z closed=2026-10-27T22:10:35Z before=receipt after=check-verifier

Host lane (row 11, class `check +mutation`; the harness mutates the tree in place, so it cannot run on the read-only container mount):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 11 | `cd tools/desk && report=$(go run ./cmd/muhar -spec cmd/deskwt/prune-mutations.json) && printf '%s\n' "$report" && case "$report" in *'Totals: 55 caught, 0 NOT CAUGHT, 0 could-not-mutate.'*) ;; *) exit 1;; esac` | pass exit=0 | sha256:48fdaee4d96a | 2026-10-27 | assay-verifier-app[bot] @ f955b97c11ba (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4668 run=3778a789cf8dae210025eeac7ad0f245b83d557d1553b6b4b73be4bd2f14e370 source=f955b97c11baa28bc9a0b8c1b378111311e5c2a4 brief=docs/streams/desk-tools/brief-26-deskwt-prune-one-walk-and-a-lock.md model=claude-opus-5-5 tier=strong

Totals: 20 of 21 rows meet their Expect, including the SPOF row 2, the fail-closed row 3 and the mutation row 11. 0 rows fail on the deliverable. Row 17 has no pass witness and is could-not-check.

- **Row 11** (host witness `pass exit=0`, counted pass). The witnessed command exits 0 only if the harness succeeds AND its report contains the literal `Totals: 55 caught, 0 NOT CAUGHT, 0 could-not-mutate.`; otherwise the `case` arm runs `exit 1`. So exit 0 under this command shows that the line was printed: all 55 mutations caught, none NOT CAUGHT, none could-not-mutate. The run took about 1h08m, and `git status --porcelain` in the home was empty afterwards.
- **Row 17** (container witness `fail exit=1`, counted could-not-check). This is not a deliverable failure:
  - A bounded same-image diagnostic of the same command at the same tree, uncredited, printed `panic: test timed out after 10m0s` in deskwt, while `TestRoleInitLeavesSharedCheckoutConfigAndIndexUntouched` had just started. `FAIL .../cmd/deskwt 600.088s`; gitcore passed (`ok .../internal/gitcore 141.541s`). No test reported a failure (zero `--- FAIL` lines).
  - The cause is capacity, the same as the 2026-10-25 pass: under amd64 emulation the deskwt package does not fit the row's fixed `-timeout 600s` (open as #4179, with this pass's data attached).
- **Row 19** (container witness `pass exit=0`, counted pass). The row always exits 0 because of its trailing `echo $?`, so its output decides it. The roster was mounted for this bracket, and a bounded same-image diagnostic of the same command at the same tree ended `LINT: PASS` then `0`, which meets the Expect. That diagnostic captured under a plain shell, not verifyrun's sandbox, so its byte hash is not compared with the witnessed `sha256:8a98c18c7f17`.
- **Row 20** (container witness `pass exit=0`, counted pass with a diagnostic binding). The row's Command cell contains unescaped `||`, so table readers end the cell at the first `||` and the witnessed command is truncated before the exact-summary check (#4453). The full command, including the `case` check, run in the same image at the same tree, exited 0 and printed `summary: 3 corroborated, 0 disproved, 6 unchecked, 0 brief(s) claiming nothing`. As with row 19, that capture's byte hash is not compared with the witnessed `sha256:5fd23df1e032`.
- **Rows 1, 16 and 18** print nothing on success (`sha256:e3b0c44298fc` is the empty-output hash), which is what their Expect asks for.

Fail-first: no Verify row is risk-bearing. The brief's four risk answers are all no, and no row's tag carries risk-bearing, live, mutating or end-to-end. So no `--fail-first` witness is owed.

Risk-bearing value. ENUMERATE over the non-test files of squash b02ea2b35 (prune.go, prunesingleton.go, deskwt.go, lockreclaim.go, gitcore.go) plus the test gates the Deliverables name, re-read at f955b97. Since the 2026-10-25 pass the Cobra migration moved the flag declarations into cli.go and shifted prune.go's line numbers; the values are unchanged.

- merge-gate baseline ref = "refs/remotes/origin/main" @ tools/desk/cmd/deskwt/prune.go:705 (the one walk, in logOriginMain)
- fresh-at-tip guard ref = "refs/remotes/origin/main" @ tools/desk/cmd/deskwt/prune.go:1438 (headAtOriginMainTip, deliberately independent of the walk)
- defaultSingletonTTL = 10 * time.Minute @ tools/desk/cmd/deskwt/prunesingleton.go:57
- pruneStampSchema = "deskwt-prune-singleton-v1" @ prunesingleton.go:51
- stamp dir mode 0o700 @ prunesingleton.go:177; stamp file mode 0o600 @ prunesingleton.go:182
- lock-ttl flag default "0" @ tools/desk/cmd/deskwt/cli.go:110; singleton-ttl flag default defaultSingletonTTL @ cli.go:113
- perf budget at N>=600 = 30 * time.Second @ tools/desk/cmd/deskwt/pruneperf_test.go:58; concurrency budget = 2*single + 5*time.Second @ pruneperf_test.go:332 (test gates only)

RANK by irreversibility:
1. The merge-gate baseline ref ranks first. If it is wrong, an unmerged worktree reads as merged and is removed.
2. The fresh-at-tip guard ref ranks second. It is a hold, so an error there removes a worktree only if the merge gate also errs.
3. Every other entry ranks below these: each is a reversible knob for efficiency, hygiene or a test budget.

RISK-VALUE: DERIVED — merge-gate baseline = "refs/remotes/origin/main" @ tools/desk/cmd/deskwt/prune.go:705 — HEAD is an ancestor of origin/main exactly when HEAD is in the set of commits reachable from origin/main. logOriginMain resolves this ref once and Logs from its tip, which includes the tip itself. The set is assigned at prune.go:689 and the membership test is at prune.go:759. A nil set returns Unverifiable, which holds every candidate (prune.go:746–750). The full spelling avoids a stray local branch named `origin/main` (#885). Row 2 (set equality over a mixed fixture) and row 11's inversion and empty-set mutations bind it.

RISK-VALUE: DERIVED (second) — fresh-at-tip guard = "refs/remotes/origin/main" @ tools/desk/cmd/deskwt/prune.go:1438 — the same full spelling, read per candidate from the worktree's own refs, not from the walk. A resolve failure returns Unverifiable, which holds the worktree (prune.go:1439–1441).

RISK-VALUE: DERIVED (reversible, ranks last) — defaultSingletonTTL = 10 * time.Minute @ tools/desk/cmd/deskwt/prunesingleton.go:57 — this is a debounce only. A wrong value delays or repeats a sweep by at most one TTL and never removes anything.

The brief is gate: model with all four risk answers no. Its status is left where it is.

**VERIFY: BLOCKED** — 20/21 rows pass on merged main f955b97, 0 fail on the deliverable. Row 11 passes: all 55 mutations caught. Row 17 is could-not-check: the deskwt package exceeds the row's fixed 600s budget under amd64 emulation (#4179). Row 20 passes, with its summary check shown by a same-image diagnostic because of the unescaped-pipe defect #4453.

## Review

Gate: model (all four risk answers no). Model-gated because the destructive hazard is
mechanically bounded: row 2 compares the REMOVED SET against a recorded literal over a fixture
carrying one worktree of every class, row 3 pins the fail-closed direction, row 11's mutations
break each gate in turn, and rows 5 and 8 hold the pre-existing behaviour of every other hold
and of `remove`. The reviewer confirms in the verdict: (1) that the set the merge gate consults
is built from `refs/remotes/origin/main` spelled in FULL and from the shared checkout, and that
a failure to build it can reach no removal; (2) that every gate present before the reorder is
present after it, and that the only difference for a held worktree is the reason string; (3)
that the batched removal still verifies each deregistration against the post-loop listing and
demotes a path it cannot verify; and (4) that the singleton's LOCK fails closed while its TTL
fails open, so no leftover stamp can hold a sweep for longer than the TTL — the stale-lock
class this brief is required not to recreate.
