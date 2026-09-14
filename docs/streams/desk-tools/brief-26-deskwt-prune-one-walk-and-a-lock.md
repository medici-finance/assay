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
  - "`tools/desk/internal/gitcore/gitcore.go` § `AheadCount` (L717) — walks `Log(base)` into `baseAncestors` to exhaustion, then walks head. Two full history walks per call."
  - "`tools/desk/internal/gitcore/gitcore.go` § `IsAncestor` (L401) — resolves both ends, decodes both commits, and delegates to go-git's `object.Commit.IsAncestor`, which is unmemoized: a fresh preorder walk per call, nothing shared across calls."
  - "`tools/desk/internal/gitcore/gitcore.go` § `Open` (L87) — builds `cache.NewObjectLRUDefault()` fresh on every call, so nothing decoded by one Open is visible to the next."
  - "`tools/desk/internal/gitcore/gitcore.go` § `Log` (L348) — already returns the reachable hashes of a rev, newest first; the ancestor SET this brief needs is `Log` plus three lines, not a new primitive."
  - "`tools/desk/internal/gitcore/contains.go` § package header — the in-repo precedent for exactly this defect class, written up at length: `RefsContaining` once called go-git's unmemoized `IsAncestor` once per ref (~10 million commit visits, ~108 s where the git binary answered in ~15 ms) and was fixed by ONE shared walk plus a commit-graph generation-number cut-off. Same package, same cause, opposite direction of the question."
  - "`tools/desk/cmd/deskwt/deskwt.go` § `dirtyTracked` (L236) → `gitcore.DirtyTrackedPorcelain` (gitcore.go L815) → go-git `wt.Status()` — the ~25 % block, and the SINGLE implementation shared by `remove` and `prune`."
  - "`tools/desk/cmd/deskwt/deskwt.go` § `removeWorktreeDir` (L256) — `os.RemoveAll`, then `git worktree prune`, then a full `guard.worktreePaths(dir)` (a `git worktree list --porcelain` parse) to positively verify deregistration — all three PER removal. Shared with `remove`."
  - "`tools/desk/cmd/deskwt/deskwt.go` § `lockedWorktrees` (L74) — the porcelain lock read, already done ONCE per sweep; the shape every other per-sweep read in this brief follows."
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
  - "`tools/desk/cmd/deskwt/prune.go`: fixed-here (the sweep body — the ancestor set, the gate reorder, the batched removal, the read-only dry-run arm, and the singleton call site)."
  - "`tools/desk/cmd/deskwt/deskwt.go`: fixed-here (`removeWorktreeDir` is SPLIT so prune can batch the deregistration step; `remove`'s own call keeps the per-path prune-and-verify it has today, byte-identical in behaviour — Verify row 8 is the assertion)."
  - "`tools/desk/internal/gitcore/gitcore.go`: fixed-here (ONE added exported function, `OpenWith(dir, cache)`; `Open` becomes a one-line call to it with a fresh cache, so every existing caller is unchanged — Verify row 9)."
  - "`deskwt remove` (the human-named single-path verb): out-of-scope (it removes ONE operator-named path; it has no loop to hoist a walk out of, no second candidate to share a cache with, and no singleton to take. Its gates, its order and its per-path verification are untouched, and Verify row 8 asserts it)."
  - "`gitcore.AheadCount`: out-of-scope (the FUNCTION stays — `deskpreflight` and the push guard read a real count that a human reads. This brief removes prune's CALL to it, which is the only caller that discards the number into a string; the function's own tests are unchanged)."
  - "`gitcore.IsAncestor`: out-of-scope (unchanged and still correct; prune stops calling it per candidate, and every other caller keeps it. Making `IsAncestor` itself memoized is a `gitcore`-wide change with a cache-lifetime question this brief does not answer — it is named as follow-up, not attempted)."
  - "`gitcore.RefsContaining` / `contains.go`'s `reachIndex`: out-of-scope (read as the PRECEDENT, reused as a pattern, not called. It answers the dual question — which tips contain one commit — and its index is built for a fan-out this sweep does not have; wiring prune through it would build a parent→children adjacency over the whole ref set to answer N single-membership questions)."
  - "The `deskboot` worktree-prune step and the `--interval` supervisor: out-of-scope as POLICY (whether a boot step should sweep at all, versus leaving it to the supervisor, is a house guardrail decision recorded on the adopter side and not settled here). Both are affected MECHANICALLY by the singleton and neither is edited: the supervisor takes the lock per TICK rather than for its lifetime, precisely so it cannot lock out a manual sweep — Verify row 12."
  - "`deskkit.TryLockExclusive` / `UnlockFile` / `ErrLockBusy`: out-of-scope (consumed exactly as they are; no new locking primitive is written and neither platform arm is touched)."
version: 1
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
- `tools/desk/cmd/deskwt/prune.go` (existing) — `pruneSweep` restructured around a per-sweep
  `sweepCtx` (the ancestor set, the shared object cache, the per-worktree `*gitcore.Repo`
  cache); the gate reorder; `unmergedReason`'s count removed; the batched removal tail; the
  read-only `--dry-run` arm; the singleton acquisition; two new flags.
- `tools/desk/cmd/deskwt/prunesingleton.go` (planned) (new) — the stamp file, its flock, the
  TTL debounce, the held/recent report strings, and the stamp's read/write/parse.
- `tools/desk/cmd/deskwt/deskwt.go` (existing) — `removeWorktreeDir` split into
  `removeWorktreeTree` (the `os.RemoveAll` alone) and the existing prune-and-verify tail;
  `remove`'s call site keeps both halves in the order it has today.
- `tools/desk/internal/gitcore/gitcore.go` (existing) — `OpenWith(dir string, objects
  cache.Object) (*Repo, error)`; `Open` becomes `OpenWith(dir, cache.NewObjectLRUDefault())`.
- `tools/desk/cmd/deskwt/prunesingleton_test.go` (planned) (new).
- `tools/desk/cmd/deskwt/pruneperf_test.go` (planned) (new) — the synthetic-repo fixture
  builder, the N-scaling test, the concurrency test and the per-candidate benchmark.
- `tools/desk/cmd/deskwt/prune_test.go` (existing) — new rows for the gate order, the
  read-only dry run and the fail-closed ancestor set; `TestPruneSkipsDirtyTracked`'s fixture
  made unambiguous (see the facts).
- `tools/desk/internal/gitcore/gitcore_test.go` (existing) — the shared-cache row.
- `tools/desk/cmd/deskwt/prune-mutations.json` (planned) (new) — the `muhar` spec for row 11.
- `tools/desk/README.md` (existing) — the prune singleton, its two flags, and the statement
  that `--dry-run` is read-only.
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
  `tools/desk/internal/gitcore/contains.go`'s header records `RefsContaining` calling the same unmemoized
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
| 2 | check +mutation | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneRemovalSetUnchanged$' -count=1 -timeout 300s` | exit 0 — the SPOF row: a synthetic repo carrying one worktree of each class (locked, dirty, fresh-at-tip, clean-unmerged-below-tip, clean-merged-below-tip, outside-prefix, current, shared checkout) is swept, and the SET of removed paths equals the set the pre-change gate order produced, recorded as an explicit literal in the test. Mutation: inverting the ancestor-set membership test REDDENS this row by deleting the clean-unmerged worktree |
| 3 | check:ci | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneHoldsAll' -count=1` | exit 0 — the fail-closed row: with `refs/remotes/origin/main` deleted from the synthetic repo, a sweep over candidates that are otherwise fully removable removes ZERO, reports each as a sweep-level `unverifiable merge status`, emits one warning naming the cause, and exits 0. It must not fall back to per-candidate walking and must not proceed on an empty set |
| 4 | check:ci | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestMergeGateBeforeStatus$' -count=1` | exit 0 — the F3 row, asserted on BEHAVIOUR not on timing: a clean-unmerged worktree is held with an `unmerged` reason and its `Status()` is never computed (counted through a test seam on the tracked-clean gate, which must record 0 calls for it), while a merged-clean worktree is removed — so the reorder moved the gate without moving any hold |
| 5 | check:ci | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneSkips' -count=1 && go test ./cmd/deskwt/ -run '^TestPruneRefusesSharedCheckout$' -count=1` | exit 0 — every existing hold, including the unpushed POSITIVE and the pushed-but-unmerged NEGATIVE control that together pin what F1 must not collapse. `TestPruneSkipsDirtyTracked`'s fixture is made single-gate; its assertion is not loosened |
| 6 | check:ci | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneDryRunIsReadOnly$' -count=1` | exit 0 — the F6 row: on a fixture carrying a DANGLING admin entry and a STALE lock, `prune --dry-run --reclaim-stale-locks --lock-ttl 1s` leaves a recursive hash of `.git/worktrees/` unchanged, unlocks nothing, writes no singleton stamp, and still REPORTS both the bookkeeping entry it would drop and the lock it would reclaim |
| 7 | check:ci | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneBatch' -count=1` | exit 0 — the F5 row: a sweep removing 5 worktrees invokes `git worktree prune` exactly ONCE and `git worktree list --porcelain` a bounded number of times independent of the removal count (asserted against the recorded git argv list), AND a path that is still registered after the batched prune is DEMOTED — not counted in `removed`, reported as a warning naming it |
| 8 | check:ci | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestRemove' -count=1` | exit 0 — `remove`'s own tests, unchanged: the human-named single-path verb still deletes, prunes and positively verifies per path. The F5 split must be invisible to it |
| 9 | check:ci | `cd tools/desk && go test ./internal/gitcore/ -run '^TestOpenWithShares' -count=1 && go test ./internal/gitcore/ -run '^TestOpenStillBuilds' -count=1` | exit 0 — the F4 row: two `OpenWith` calls against two worktrees of one repository, given the same cache, both resolve and read correctly, and the cache instance they were handed is the one both used (asserted through a counting `cache.Object` wrapper); and plain `Open` still constructs its own, so no existing caller silently shares state |
| 10 | check:ci | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneSingleton' -count=1 -timeout 120s` | exit 0 — the F7 rows: (a) with the lock held, a second sweep skips with the held message, exits 0, and removes nothing; (b) a sweep whose stamp records a `finishedAt` inside the TTL skips with the recent message; (c) `--singleton-ttl 0` and `--no-singleton` sweep anyway but STILL take the lock; (d) a MISSING, truncated, non-JSON, and future-dated stamp each cause the sweep to PROCEED (fail-open), not to skip; (e) `--dry-run` writes no stamp |
| 11 | check +mutation | `cd tools/desk && go run ./cmd/muhar -spec cmd/deskwt/prune-mutations.json` | exit 0 — baseline GREEN, positive control CAUGHT, and every mutation CAUGHT: the ancestor-set membership test inverted; the failed-set build allowed to continue with an empty set; the tracked-clean gate moved back ahead of the merge gate AND its hold dropped; the batched verification's demotion removed so an unverified path counts as removed; `--dry-run` allowed to reach the real `git worktree prune`; the singleton's TTL parse made to fail CLOSED (a corrupt stamp wedging the sweep); and `TryLockExclusive`'s `ErrLockBusy` treated as free |
| 12 | check:ci | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestPruneInterval' -count=1 -timeout 120s` | exit 0 — the supervisor releases the singleton between ticks, so a one-shot sweep between two ticks is NOT locked out; the two existing interval tests are unchanged |
| 13 | check:ci | `cd tools/desk && DESKWT_PERF_N=50 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 600s` | exit 0 — the synthetic fixture (a `git fast-import`-built 5,000-commit `origin/main`, N fabricated registered worktrees verified against `git worktree list --porcelain`) sweeps at N=50, GATES on the measured wall time against the budget the test states for N, and asserts `origin/main` was walked EXACTLY ONCE (counted through the sweep's own walk seam) |
| 14 | check:ci | `cd tools/desk && DESKWT_PERF_N=200 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 900s && DESKWT_PERF_N=600 go test ./cmd/deskwt/ -run '^TestPruneSweepScalesAtN$' -count=1 -timeout 900s` | exit 0 — the same row at N=200 and N=600; at N=600 the test GATES on a sweep wall under 30 s, which is #1037's stated target, and the walk count stays 1. A row that merely printed a duration would assert nothing |
| 15 | check:ci | `cd tools/desk && DESKWT_PERF_N=200 go test ./cmd/deskwt/ -run '^TestConcurrentPrunes' -count=1 -timeout 900s` | exit 0 — the F7 measurement: five sweeps launched together against one synthetic repo, each holding its own open file descriptor on the stamp; exactly ONE performs a sweep, four report held-or-recent, all five exit 0, and the total wall is gated under 1.5× a single measured sweep on the same fixture |
| 16 | check:ci | `cd tools/desk && go test -bench '^BenchmarkPruneSweepPer' -benchtime 10x -run '^$' ./cmd/deskwt/ > /tmp/dt26-bench.out 2>&1; grep -q 'BenchmarkPruneSweepPerCandidate' /tmp/dt26-bench.out` | exit 0 — the benchmark exists and runs, pinning the per-candidate cost so a later regression has a number to be compared against. It reports; row 14 is what gates |
| 16b | check:ci | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestLegacyPerCandidate' -count=1 -timeout 600s` | exit 0 — the BASELINE, on the same synthetic fixture: what the sweep used to do per candidate (a fresh open with its own object cache, the unmemoized ancestor walk, and the two ahead-count walks for the unmerged majority), timed alongside a whole current sweep. It is not a gate; it exists so the improvement is a measurement with a reproducible baseline rather than a number in a PR body |
| 17 | check:ci | `cd tools/desk && go test -timeout 600s ./cmd/deskwt/... ./internal/gitcore/... -count=1` | exit 0 — every existing test of both touched packages |
| 18 | check:ci | `cd tools/desk && gofmt -l cmd/deskwt internal/gitcore > /tmp/dt26-fmt.out; test ! -s /tmp/dt26-fmt.out` | exit 0 |
| 19 | check:ci | `cd statusgen && go run . --root .. --lint; echo $?` | 0 |
| 20 | check:ci +dereference | `cd statusgen && go run . --root .. --consumers --brief assay:assay:desk-tools:26; echo $?` | 0 — the routing claims in `consumers:` are RESOLVED against this branch's own diff: every `fixed-here` entry is touched by the diff and every `out-of-scope` one is not. Exit 2 is COULD-NOT-CHECK (no diff to take — a fully merged tree) and is reported AS ITSELF, never as a pass |

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
