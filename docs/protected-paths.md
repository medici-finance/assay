# Protected verifier paths

Brief: verify-integrity/01. A worker PR can edit the brief's own Verify table, a CI workflow,
a client-side guard or a golden fixture in the same diff as the code those things check —
and, absent this check, nothing notices. `deskpathguard check` is the reviewer-side control
that closes that gap; this page states the protected set, the exemptions and the boundary of
what the check does and does not do, once, so the check, `pr-review-desk` and `verify-desk`
can all cite it rather than restate it.

## The protected set (initial)

A diff touches a protected path when it touches any of:

- a brief's `## Verify` table (detected from the diff itself — the hunk that changed a line
  between the `## Verify` heading and the next `## ` heading; see
  `tools/desk/cmd/deskpathguard/verifysection.go`)
- `.github/workflows/**`
- `.claude/guardrails/**`
- `tools/skillslint/**`
- any path under a `verify.d/**` scripted-rows directory
- golden fixtures under `**/testdata/**`

## The label rule

`deskpathguard check <owner/repo> <number>` applies the `wrote-to-the-test` label — and
prints a `gate-forced: wrote-to-the-test` line the status transition reads — when ALL of the
following hold:

1. the PR author is **not** the desk or verifier identity (the roster's `RoleBotIdentity`
   for those two roles) — this is a full exemption regardless of what the diff touches, so a
   desk- or verifier-authored regen/re-baseline PR is never labelled no matter which paths it
   carries;
2. the diff touches at least one protected path (above);
3. the diff also touches at least one file that is **neither** a brief file
   (`docs/streams/<stream>/brief-*.md`) **nor** a fixture file (`**/testdata/**`).

Condition 3 is what makes a pure-authoring diff (brief file(s) only, or a testdata-only
re-baseline) exempt on its own, without needing a separate identity check: a diff that touches
only brief and/or fixture files can never satisfy it.

A PR carrying a `regen:`-prefixed label is a fourth, independent exemption — it short-circuits
before the three conditions above are evaluated at all.

## Three states, never two

`deskpathguard check` reports one of `checked-clean`, `checked-failed` or `could-not-check` —
matching the verify-integrity stream's own rule. A diff that could not be read (a deleted or
unreadable head) is **`could-not-check`** (exit 6): it is never rendered as clean, and the
label is never applied on a could-not-check verdict. `checked-clean` and `checked-failed` both
exit 0 when the check ran — so a caller reads the `state=` line, not the exit code alone, to tell
them apart (`checked-failed` also prints `gate-forced: wrote-to-the-test`). The exit code is still
a signal for the write: without `--dry-run`, a `checked-failed` check whose label write fails
after the state line exits 5 (refused) or 6 (could-not-check) and prints no `applied:` line — the
label is **not** on the PR, and a caller who read only the `state=` line would wrongly conclude
the gate was forced. A non-zero exit with no `state=` line means the check did not run, which is
`could-not-check`.

## What this check does NOT do

- It does not run the house ruleset / branch-protection half — that is a repo-admin change
  out of this brief's scope, tracked separately, and lands once applied.
- It does not itself change what `deskflip` requires. A `wrote-to-the-test`-labelled PR is
  forced to `gate: human` at the STATUS TRANSITION (the brief's own status-machine, not the
  ready-flip) — per the 2026-09-09 ruling that a human-gate block sits at the status
  transition, never at the ready-flip. `deskflip`'s own conditions are unchanged; see
  `tools/desk/cmd/deskflip`'s test suite, which this brief adds no new case to (there is
  nothing there to change).
- The `## Verify`-table detection (condition 1's first bullet) is a best-effort heuristic
  over the diff text, not a byte-exact section parse — see the doc comment on
  `verifySectionTouched` in `tools/desk/cmd/deskpathguard/verifysection.go` for its bound.
  The lower layer that does not share this approximation is verify-desk's merge-base
  re-derivation, next.

## The re-derivation step (verify-desk)

`deskpathguard rederive --root <checkout> --brief <path/to/brief.md>` is verify-desk's
pre-change re-read on a labelled brief: it resolves the merge-base of `HEAD` and
`origin/main`, reads the brief's `## Verify` table as it stood there, and compares it against
the table at `HEAD`. Every row present at `HEAD` whose `#` cell also exists at the merge-base
runs with the **merge-base row's own command and expect** — an edit to an existing row's
command at `HEAD` is bypassed, never trusted. A row whose `#` cell has no match at the
merge-base is reported **`author-added`**, never `pass` — the row the worker itself added is
never treated as a witness of anything.

This is the single-point-of-failure's second layer (see the brief's `facts:`): the reviewer
check above is the ONE control until the house ruleset lands; this re-derivation is what
catches an edit the check missed, because it reads git history rather than a label.

### Recording closure after re-derivation

The independent verifier puts one live marker in the current Evidence run, before
its final verdict (full immutable Git object IDs, not branch names):

```text
Verification-Table: delivery=<sha> base=<sha> original=<sha>
```

`delivery` is the landed implementation commit on the observed HEAD's first-parent
chain. Other reachable commits do not establish mainline delivery. For a squash delivery, `base` is
its exact parent; for a two-parent merge, it is the parents' unique merge-base.
Before using a two-parent merge baseline, the evaluator also requires the first-parent
main acceptance table to equal the divergence table; otherwise independent
re-derivation is required.
`original` is the actual pre-change re-derivation revision. It must be an ancestor
of `base`, and its complete parsed Verify table must equal the table at `base`.
If main changed that table while the delivery was in flight, closure refuses;
retain the independent re-derivation and resolve the discrepancy explicitly.
Missing objects, root/octopus deliveries and ambiguous merge bases refuse too.

The current Verify section must still equal the delivered section. Every original
row ID remains mandatory, with its original Command, Expect and class; removing or
renumbering one refuses. An edited command needs a witness of the original command,
and an edited Expect cannot change the required original process exit. As with
ordinary witnesses, output assertions require independent inspection of the recorded
output hash and primary output. The audit does not reconstruct output from a hash.

Each added row's latest matching witness reports `author-added; execution exit=0
sandbox=unshare; no acceptance credit` (with the actual observed exit and sandbox).
It is a supplemental disposition, never a passing acceptance witness. Retain the
normal witness columns and primary execution evidence. A later pass claim does not
override this accounting. The execution command does not automatically rewrite a
current-table run into a re-derived run: the verifier must retain the original
commands and results before producing the closure record.

`verifyrun --check`, `brief --check-verified`, `verifyclosure`, the verified flip,
the reconciliation fold and lint use the same protected-table evaluator. Lint's
fail-first and row-strength checks retain the original acceptance table. Human row
gates, verifier authorship, admission and full closure lint remain required. The
marker proves local immutable object contents and ancestry; it is not a forge
identity attestation, permission or a replacement for independent verification.
This local history proof is relative to the checked HEAD: the independent verifier
still identifies the actual delivery; it does not bind that identity to a forge PR.
Run protected closure from a landed-mainline checkout; a branch that imports it
only through a merge can fail closed.

Coverage observes the same original acceptance rows. Its typed
`AcceptanceRevision` records the validated original definition revision; the
evaluator reads that actual historical Expect and compares it to the selected
original Expect. The witness keeps its execution revision and all content and
dependency checks. Invalid provenance creates an explicit held claim, including
when a separate pattern supplies other passing claims.

Retain previous Evidence runs. Each protected run contains its own marker and
witnesses, followed by its final strict `**VERIFY: PASS**` verdict. Closure,
audit, coverage, flip, reconciliation and lint select that same latest run, never
borrowing an earlier marker or witness. Duplicate live markers within one run, a latest failed or
malformed verdict, and unfinished live content after the last PASS refuse
closure. A caller with an already-selected run retains full Evidence through
the shared context adapter: earlier protection still requires current provenance,
while ordinary unmarked records audit only their selected run. Passing the selected
text alone loses that context and is not a valid protected closure input.
