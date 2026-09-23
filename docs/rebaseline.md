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
| `safe:rename` | The row pins a path that no longer exists, and git records a **single** rename/move hop from it to a path that exists now (`git log --follow`). The work is intact; only the pinned path moved. | **live** |
| `safe:count` | The row's Expect is a count whose current value differs, and the target files changed **only by additions** since the Evidence date — the higher count is growth, not a regression. | *not yet implemented* |
| `safe:idiom` | The row pins a tool idiom **retired by a recorded ruling** (e.g. a renamed flag). The command idiom moved; the checked behaviour did not. | *not yet implemented* |

> **Only `safe:rename` fires in the shipped verb today.** The classifier and the row-rewrite
> code carry the `safe:count` and `safe:idiom` arms, but the fact-gatherer
> (`tools/desk/cmd/deskrebaseline/facts.go`) does not yet populate the count/idiom facts, so
> those two verdicts are never produced in production — a count-drift or retired-idiom row
> falls through to `refused:unclassified` and is filed exactly as today. Wiring the count/idiom
> facts (and reconciling the behaviour probe, which currently pre-empts a count drift written
> as `test $(...) -eq N` before `safe:count` could fire) is a tracked follow-up. The two rows
> above are marked *not yet implemented* so no reader expects re-baselines the verb cannot
> produce. This fails **closed** — the not-yet-wired classes refuse, never launder.

## The refusal set — the verb MUST file, not re-baseline

| Class | Signal |
|-------|--------|
| `refused:behaviour-changed` | The command still **runs** and returns a different result than the row pinned. That is a real behaviour change, not a stale oracle. |
| `refused:gone` | The pinned deliverable path is gone with **no** single rename hop — the verb cannot prove the work survived. |
| `refused:risk-bearing` | The row's owning brief is risk-bearing (`gate: human`, or any `risk:` flag `yes`). **No shape** makes such a row re-baselineable by the verb, regardless of its stale shape. |
| `refused:unclassified` | The facts do not positively place the row in any safe class. The fail-closed default: unproven is never re-baselined. |

`refused:risk-bearing` is read from the owning brief's own frontmatter via the same audited
predicate the ready-flip risk gate uses, so the verb's notion of "risk-bearing" cannot drift
from the rest of the fleet's.

## Usage

```
deskrebaseline <brief> --row K [--root DIR] [--repo owner/name] [--open]
```

- `<brief>` — a brief file path, or a `<stream>/<NN>` id resolved under `--repo`'s stream root.
- `--row K` — the 1-based row of the brief's `## Verify` table to classify.
- **Without `--open` (dry-run)** — prints the classification, the git intactness evidence, and
  the plan. It creates **no branch** and always exits `0`: a classification report is not a
  refusal. Read the printed `verdict:` line.
  - **Dry-run executes the row's command.** The behaviour probe (does the command still run,
    and does it return a different result?) *is* one of the classification signals, so
    classifying a row runs that row's shell command — even in dry-run, and even though the plan
    is printed only afterwards. Verify-row commands are trusted-authored and are exactly what
    the verifier runs anyway, inside the verifier's offline envelope; as defence in depth the
    probe additionally forces `KUBECONFIG=/dev/null` in the command's environment, so a row
    command cannot reach a cluster whatever the ambient `KUBECONFIG`.
- **With `--open`** — on a `safe:*` verdict it rewrites that one row, commits it on a
  `rebaseline/<stream>-<NN>-row-<K>` branch, and opens a **draft** PR via `deskpr create` (as the
  loop identity — the verifier App under `DESK_LOOP=verify-desk`). On a `refused:*` verdict it
  opens nothing and exits `5`: file the issue as today, with the verb's reason.

## The PR shape

One row per commit. The PR body names the original Evidence date, the old and the new row, and
the git evidence for intactness. The branch is `rebaseline/<stream>-<NN>-row-<K>`. Because it
edits a Verify table, the reviewer App reviews it under `wrote-to-the-test`; the verifier
identity authored it, so that rule's own-author exemption applies.

## Where the verify desk runs it

On a FAIL whose row matches the stale shape, the verify-desk skill runs `deskrebaseline` **before
filing**. A `safe:*` classification opens the re-baseline PR; a `refused:*` classification files
the issue exactly as today, carrying the verb's reason. See
`plugins/assay/skills/verify-desk/SKILL.md`, "On VERIFY: FAIL".
