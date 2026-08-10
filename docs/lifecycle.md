# Lifecycle and the status board

How a brief moves from idea to done, and how the generated board (`STATUS.md`) stays
trustworthy. Each rule is stated with its reason.

## Brief lifecycle

```
todo → in-progress → implemented → verified → done
```

- **todo** — authored, unclaimed, dependencies known.
- **in-progress** — a session owns it and is implementing.
- **implemented** — the implementer finished and filled the Evidence section with its own
  run. Implementers STOP here. Reason: an implementer verifying their own work is the
  narrator grading their own exam.
- **verified** — a **non-implementer** re-ran the Verify table on merged main and filled
  the Evidence section (dated, with runner). Reason: independent re-execution is the check
  that "works on my machine / green in isolation" claims survive contact with main.
- **done** — additionally carries the recorded review verdict. A `gate: human` brief needs
  a review entry naming a human (`human:<name>`); a model sign-off does not close a
  risk-flagged brief.

Verified/Reviewed cells take dated entries (`2026-07-08 sonnet-verifier`, `human:ian`),
never a bare checkmark. Reason: an undated tick is unattributable and unauditable.

**Merging does not verify.** After a brief-PR merges it sits in the board's "awaiting
verification" queue until someone dispatches a non-implementer to run the Verify table.
Reason: an unwatched awaiting-queue is how briefs rot at `implemented` — verified is a
distinct, owned step, not a side effect of merge.

## Review gates

- Working diff (pre-PR) and open PRs get a review pass; the verdict is recorded in the
  stream README table.
- The **Verify table proves function** ("works?"); the **review proves quality**
  ("well-built?"). Neither substitutes for the other. Reason: a change can pass every check
  and still be badly built, and vice versa.

## STATUS.md — a single-writer generated artifact

`STATUS.md` at the repo root is **generated from the stream READMEs and registers** — never
hand-edited. It has exactly one writer: **main's CI**, which regenerates and commits it on
every push that touches a source.

- **Branches never commit STATUS.md.** PR CI runs the generator in `--lint` mode (all source
  checks, no STATUS.md read/write) and blocks any PR whose diff touches STATUS.md. Reason:
  a generated file committed on branches turns every concurrent PR into a merge conflict;
  one writer eliminates the conflict class entirely.
- On a local STATUS.md merge conflict, take either side and rerun the generator — never
  hand-merge a generated file.
- Regenerate locally any time to read the board (e.g. to see Next-up); just don't commit it
  on a branch.

## Next-up semantics

The generator computes a **Next-up** batch — the briefs to pick next — so a session doesn't
default to "the next brief in my stream" (the rabbit-hole reflex the system exists to
prevent). It weighs:

- **Priority + staleness** — higher-priority streams and streams that have aged rise.
- **A 2-per-stream cap** — no single stream floods the batch, so parallel work spreads.
- **Findings exclusion** — a brief with an unresolved finding against it is held out until
  the finding resolves (see `registers.md`).
- **Claim exclusion** — a brief with an open branch against it on `origin` is already in
  flight and is dropped from the batch, so two sessions don't pick the same work.

**When the claim read fails, the board says so.** Claim exclusion needs `git ls-remote
origin`; when that times out or errors, Next-up is an *unfiltered superset* — it lists briefs
another session already holds. The run then prints a `claim filtering UNAVAILABLE` NOTICE and
stamps a **DEGRADED** banner into STATUS.md's Next-up section naming the cause. It still
writes the board, because STATUS.md has a single writer and refusing to write leaves the
previous board on main — equally a superset, but unlabelled. A caller that dispatches work
from the board can demand the stronger guarantee with `--require-claims`, which exits 1 and
writes nothing instead of degrading. The read's deadline defaults to 10s and is overridable
with `STATUSGEN_REMOTE_TIMEOUT` (a Go duration) — but the timeout only sets how *often* the
degraded path is taken, never whether it is announced (assay-toolkit#305).

Known limitation, stated honestly: `score = priority + staleness` rewards neglect regardless
of *why* a stream aged, and any git touch resets a rival stream's staleness. A value/effort
term is a candidate knob for a retro to add — the board is a heuristic scheduler, not an
oracle.

## The honest claim about the board

The board is **derived from agent-authored artifacts with consistency linting**, not measured
from ground truth. The generator parses status tables, frontmatter, Verified/Reviewed cells,
and Evidence blocks — all markdown written by the same agents whose work it reports. It
checks the *internal consistency* of those documents (sequence gaps, missing evidence,
unresolved findings, malformed gates) and is backstopped by adversarial spot-verification;
it does not independently observe that the code does what a brief claims.

So the defensible statement is "status is derived from agent-authored artifacts with
consistency linting and independent re-verification" — **not** "status is measured, never
self-reported." The strong form is false: the sensors are agent-writable. Optional hardening
(machine-attributable, non-self-writable lifecycle transitions; a deny-hook layer that
mechanically blocks an implementer from writing its own gate cells) narrows the gap but does
not close it while a single identity can author both the work and its record. Claim the
weaker, true thing.

The same discipline applied to the *gates* rather than the board — which of them GitHub
enforces, which the house merely honours, and the sandboxed-execution target that would close
the difference — is enforcement-model.md.
