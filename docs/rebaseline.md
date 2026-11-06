# `deskrebaseline` — the stale-artifact re-baseline lane

A large share of verify FAILs are not Change Failures. They are the same shape: a `## Verify`
row pins a **path**, a **count**, or a **tool idiom** the tree moved out from under it, so the
table fails *as written* while the work it checks is intact. The verify desk correctly refuses
to flip and files an issue — and the next drain re-runs the same brief and re-files the same
class of issue. `deskrebaseline` ends that loop by turning a *provably-intact-but-stale* row
into a one-row re-baseline PR instead of the Nth duplicate issue.

## The single point of failure — the "provably intact" predicate

The safe/refusal boundary is the whole design. **If it is loose, a real regression is
re-baselined into a green** — a laundered fail, the worst outcome the verify desk exists to
prevent. So the classifier (`tools/desk/cmd/deskrebaseline/classify.go`) fails **closed**: a row
reaches the safe set *only* by a positive proof from git history, and everything it cannot prove
intact is refused and filed.

The layer behind the predicate: the re-baseline is a **PR**, not a main landing. The verb never
merges and never pushes to `main`. The PR is reviewed by the reviewer App under the
`wrote-to-the-test` rule (it edits a Verify table) and merged by a human. The verb is one control;
the review and the human merge are the independent layers behind it.

## The safe set — the verb MAY propose a re-baseline

| Class | Proof required | Status |
|-------|----------------|--------|
| `safe:rename` | The row pins a path that no longer exists, and git records a **single** rename/move hop from it to a path that exists now (`git log --follow`). The row is then **re-run** with the old path rewritten to the new one in its Command and Expect, and it must pass that Expect. Only whole literal path tokens are rewritten; similarly named paths keep their meaning. The work is intact; only the pinned path moved. | **live** |
| `safe:count` | The row's Expect pins **one** count as its `output is` line, and the command now runs with the pinned exit and prints a **larger** bare integer, so the Expect rewritten to the new count passes. Git history must then prove the growth: the row's Evidence commit is the commit that wrote its latest Evidence line, which must record a `pass` of the same command, with the Verify row unchanged since. `git diff` from that commit to `HEAD` over every path the command names must show **only added lines**: zero deletions, no rename, delete or mode change, no binary change, every target present at both ends and clean in the tree. A shallow clone, a dirty target or any git error refuses. | **live** |
| `safe:idiom` | The row's command uses a tool idiom (one flag or command word, matched as a whole shell word) that an entry of the checked-in list [`retired-idioms.json`](../tools/desk/cmd/deskrebaseline/retired-idioms.json) retires. Each entry names the idiom, its replacement and the **ruling reference** (`#N`, `owner/repo#N` or an issue/pull URL) that retired it; an entry without one is ignored. The row is re-run with the idiom replaced and must pass its own Expect. Nothing is inferred from a pattern match alone. | **live** (the list ships empty) |

> **The retired-idiom list ships empty.** An idiom becomes re-baselineable only when a reviewed
> change adds its entry with the ruling that retired it. Until then no row is `safe:idiom`, and
> such a row is filed exactly as today. Both classes fail **closed**: a count drift the history
> cannot prove, or an idiom the list does not name, refuses; neither launders a change.
>
> Both are decided **before** the behaviour probe's refusal, because the drifted count or the
> retired idiom is exactly what fails the row's Expect. A count drift the history proof rejects
> still lands in `refused:behaviour-changed`, as before.

## The refusal set — the verb MUST file, not re-baseline

| Class | Signal |
|-------|--------|
| `refused:behaviour-changed` | The command still **runs** and its result fails the row's own Expect cell, or the rename-rewritten row runs and fails it. That is a real behaviour change, not a stale oracle. |
| `refused:gone` | The pinned deliverable path is gone with **no** single rename hop — the verb cannot prove the work survived. |
| `refused:risk-bearing` | The row's owning brief is risk-bearing (`gate: human`, or any `risk:` flag `yes`). **No shape** makes such a row re-baselineable by the verb, regardless of its stale shape. |
| `refused:unclassified` | The facts do not positively place the row in any safe class, including a rename-rewritten row that could not run or whose Expect could not be decided. The fail-closed default: unproven is never re-baselined. |

`refused:risk-bearing` is read from the owning brief's own frontmatter via the same audited
predicate the ready-flip risk gate uses, so the verb's notion of "risk-bearing" cannot drift
from the rest of the fleet's.

## Usage

```
deskrebaseline <brief> --row K [--root DIR] [--repo owner/name] [--open]
```

- `<brief>` — a brief file path, or a `<stream>/<NN>` id resolved under `--root` first, then under `--repo`'s configured stream root.
- `--row K` — the 1-based row of the brief's `## Verify` table to classify.
- `deskrebaseline --help` (also `-h`, `help`, `--version`) answers before anything is read. A
  command line it cannot use is refused with exit 5 before the brief is read: a missing
  `--row`, a row that is not a positive integer, a second `<brief>`, or an unknown flag.
- **Without `--open` (dry-run)** — prints the classification, the git intactness evidence, and
  the plan. It creates **no branch** and always exits `0`: a classification report is not a
  refusal. Read the printed `verdict:` line.
  - **Dry-run executes the row's command.** The behaviour probe (does the command still run,
    and does it return a different result?) *is* one of the classification signals, so
    classifying a row runs that row's shell command — even in dry-run, and even though the plan
    is printed only afterwards. Verify-row commands are trusted-authored and are exactly what
    the verifier runs anyway, inside the verifier's offline envelope; as defence in depth the
    probe additionally forces `KUBECONFIG=/dev/null` in the command's environment, so a row
    command cannot reach a cluster whatever the ambient `KUBECONFIG`. For a rename hop it runs
    the rewritten row instead, under the same envelope.
  - **The result is judged against the row's Expect cell**, read the way `statusgen`'s
    verify runner reads it: the exit code it names (`exit 1`; default `0`; a backticked
    `exit=N` is printed output, not the required exit), an `output is` line, and a `≥ N`
    floor. A row whose Expect is `exit 1` and whose command exits 1 has not changed.
    An Expect that names none of those is **undecided**, not a pass: prose only, a backticked
    token only, or an exit given as "non-zero" with no number. An explicit numeric exit wins over explanatory "non-zero" prose; an overflowed numeric constraint is undecided. `statusgen` would pass such a
    row on its exit status alone. Here it is never proof, so a renamed row with an undecided
    Expect is `refused:unclassified`, and so is an undecided behaviour probe.
  - **Comment-led and comment-only rows are never executed**, by either the behaviour probe or the rename re-run. A comment's no-op exit is never proof of `safe:rename`.
  - **An escaped pipe `\|` in the Command cell is part of the command.** Rows are split the way
    `statusgen` splits them, and the probe runs the un-escaped pipeline.
- **With `--open`** — on a `safe:*` verdict it rewrites that one row, commits it on a
  `rebaseline/<stream>-<NN>-row-<K>` branch, and opens a **draft** PR via `deskpr create` (as the
  loop identity — the verifier App under `DESK_LOOP=verify-desk`). On a `refused:*` verdict it
  opens nothing and exits `5`: file the issue as today, with the verb's reason.
  - **`--open` writes only the checkout it was pointed at, and only from a clean base.** Before
    it creates a branch or writes a file, it refuses unless all three of these hold:
    1. the brief resolves **inside `--root`**, with symlinks resolved. A `<stream>/<NN>` id
       resolves under `--root` first and falls back to the configured stream root only when
       `--root` has no such brief. A brief that lands outside `--root` either way is refused,
       never written.
    2. `HEAD` **is the fetched base** `refs/remotes/origin/main`. The one-row branch is cut from
       `HEAD`, so any other commit would ride along. The verb never fetches: fetch first, then
       check out `refs/remotes/origin/main` detached.
    3. the checkout is **clean**, with nothing staged, modified or untracked.

    The commit also names its one path explicitly, so it can never sweep in the rest of the
    index. A refusal leaves every tree exactly as it was.
  - The rename-hop proof reads `HEAD`'s history only (never `--all`), so a rename recorded only
    on an unmerged or unrelated ref is not taken as evidence.

## The PR shape

One row per commit: the commit touches only the brief, and its parent is the fetched base.
The PR body names the original Evidence date, the old and the new row, and the git evidence
for intactness. The branch is `rebaseline/<stream>-<NN>-row-<K>`. Because it
edits a Verify table, the reviewer App reviews it under `wrote-to-the-test`; the verifier
identity authored it, so that rule's own-author exemption applies.

## Where the verify desk runs it

On a FAIL whose row matches the stale shape, the verify-desk skill runs `deskrebaseline` **before
filing**. A `safe:*` classification opens the re-baseline PR; a `refused:*` classification files
the issue exactly as today, carrying the verb's reason. See
`plugins/assay/skills/verify-desk/SKILL.md`, "On VERIFY: FAIL".
