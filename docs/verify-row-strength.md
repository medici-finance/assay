# Verify-row strength — the thirteen rules

A Verify row is evidence only if it can fail. Each rule below names one way a row passes whatever
the tree holds, the stable tag `statusgen --lint` reports it under, and the incident that earned the
rule. Rules are cited by **tag** in code and tests. The R-number is this page's ordering only.

**Severity.**

- **R1–R10** are *unfailable* rows. On a brief that **this branch closes** (the stream README says
  `verified`/`done`, and it did not at the merge-base of HEAD and `origin/main`), a finding is a
  lint **PROBLEM**: an unfailable row cannot be the basis of a new closure. On every other brief
  they stay a **NOTICE**. That includes the closures already on main, whose Verify tables are never
  rewritten to green a gate. When `origin/main` cannot be resolved, the lint cannot tell a new
  closure from an old one, so it holds nothing and says it is running degraded.
- **R11–R13** are *strength* heuristics. They flag weak rows rather than provably unfailable ones,
  so they are a **NOTICE** on every brief.

Separately from the shape rules, a **risk-bearing** row must be shown to fail. A row is
risk-bearing when the brief has any `risk:` answer `yes`, or the row is tagged risk-bearing, live,
mutating or end-to-end. See "Fail-first" at the end.

## R1 — `\|` inside `grep -E` (`ere-literal-pipe`)

In an extended regex `|` is alternation. Escaping it as `\|` makes it a literal pipe, so the
pattern matches almost nothing. A row gated on a count or a non-match then passes blind.
**Fix:** write `grep -E 'a|b'`. *Earned by #509.*

## R2 — `grep -c` gated on zero (`grep-zero-count`)

A row whose pass bar a zero count satisfies, such as `grep -c X file` with Expect `0` or `≥0`,
measures nothing: the file being absent or the pattern being misspelled pass the same way. When
zero IS the claim, assert it on a file the row first proves exists. `grep -c … || true` is the
sanctioned way to keep a zero count from exiting 1, and R11 exempts it. *Earned by #509.*

## R3 — a pipeline whose exit status is sunk (`pipeline-exit-sunk`)

`cmd | tail -1`, `cmd | tee log`: the shell reports the LAST stage's status. The command under test
can fail and the row still exits 0. **Fix:** assert on the output, or `set -o pipefail` where the
shell has it. *Earned by #509.*

## R4 — `\|` inside a `go test -run` / `-bench` selector (`rE2-literal-pipe`)

Go's selectors are RE2, where `\|` is a literal pipe. `-run 'TestA\|TestB'` matches no test, and
`go test` exits 0 with "no tests to run". **Fix:** `-run 'TestA|TestB'` plus a `--- PASS` assertion.
*Earned by #374.*

## R5 — an unsubstituted placeholder (`unsubstituted-metavar`)

A `<metavar>` left in the Command cell, such as `<path>` or `<sha>`, means the row cannot be run
as written. Whatever a runner substitutes is the runner's row, not the brief's. *Earned by #509.*

## R6 — `go run` under a specific non-zero exit (`gorun-exit`)

`go run` does not propagate the program's status. It prints `exit status N` and exits 1 itself, so a
row expecting exit 3 cannot tell 3 from any other failure. **Fix:** build the binary, then run it,
or assert on the printed `exit status N`. *Earned by #493.*

## R7 — a pipe in a basic-regex `grep` pattern (`bre-alternation`)

Without `-E`/`-P`, a portable `grep` compiles a basic regex, where `|` is an ordinary character. The
pattern looks for one long literal, and in a brief that quotes its own command the row counts its
own line. **Fix:** `grep -E 'a|b'`. *Earned by #262.*

## R8 — a raw `|` in the Command cell (`shredded-cell`)

The table splitter reads an unescaped `|` as a cell boundary. The command is truncated and every
later column shifts, so the row runs something other than what it shows. **Fix:** escape it as `\|`
inside the code span. *Earned by #374.*

## R9 — a diff base pinned to a moving ref (`moving-ref`)

`git diff main…` or `origin/main..HEAD` resolves differently tomorrow. The row's result drifts with
the branch, not with the change. **Fix:** compute the base once (`$(git merge-base HEAD origin/main)`)
or pin a SHA. *Earned by #639.*

## R10 — a GNU-only construct (`gnu-only`)

A construct the BSD/macOS userland reads differently. For example, process substitution `<(…)`
reaches BSD `grep` as an empty file, so a row returns 0 matches with the content plainly present.
**Fix:** a pipe, or an explicit OS marker on the row. *Earned by #650.*

## R11 — a trivially-green command (`trivially-green`)

The command exits 0 whatever the tree holds:

- `true`, `:`, `exit 0`, or a lone `echo` or `printf`;
- a trailing `|| true` or `|| echo …`, unless the row is R2's sanctioned `grep -c … || true`;
- `git log --grep <text>` with no count, which exits 0 whether or not anything matched;
- an existence test (`test -e`, `-f`, `-d`, `-s`, or `[ … ]`) on a path the brief's own `files:`
  declares, which the change itself creates.

An existence test followed by a content check (`test -e f && grep -c X f`) is not trivially green.
*Earned by the verify-row strength audit behind this page: rows of exactly these shapes stood in
closed Verify tables as the evidence for the closure.*

## R12 — `exit 0` with no output assertion (`no-output-assertion`)

The Expect cell is exactly `exit 0`, and the command produces output that nobody reads. The status
of `go test`, `make`, or a linter says little on its own (R4's "no tests to run" exits 0 too).
Quiet assertions are exempt: `test`, `[`, `grep -q`, `cmp -s`, `git diff --quiet`, and a pipeline
ending in one. **Sanctioned Expect forms:** `exit N; output contains "<literal>"`,
`exit N; <N> lines`, or a hash of the output. *Earned by the same audit as R11.*

## R13 — the table touches no declared path (`table-touches-no-files`)

None of the table's commands reference any path the brief's `## Context` `files:` declares. The
reference can be exact, an ancestor or descendant directory, a glob, or a Go package pattern. A
table that never reads what the change touched may prove nothing about the change. When `files:`
is present but yields no path, the result is **COULD-NOT-CHECK**, never a finding. An absent
`files:` is silent. *Earned by the same audit as R11.*

## Fail-first

`statusgen verifyrun --brief <path> --fail-first` runs each risk-bearing row twice. The first run
is on the base, checked out into a temporary worktree, with the same sandbox and `--ci` mode as the
head run. The second is the ordinary run at head. The default base is the merge-base of HEAD and
`origin/main`. `--base <rev>` may name that merge-base or an ancestor of it that already holds the
brief file, and nothing else: a branch commit, HEAD, an unknown object or a base older than the
brief is refused. When the merge-base does not hold the brief at its current path (the brief was
created, renamed or moved on the branch, or its path does not resolve under the root), no ancestor
can be shown to hold it, so only the merge-base itself is accepted and the check fails closed. The witness table gains a `Base` column,
appended to the right of `Runner` (readers of Date and Runner skip it):

    base=<sha> red rc=1 · head=<sha>

The base states are:

- **`red`**: the row failed at base. This is what a row that discriminates looks like.
- **`green`**: the row passed at base. If it also passes at head, it is **non-discriminating**:
  it passes with or without the change.
- **`unproven`**: the row could not run at base.
- **`not-selected`**: the row is not risk-bearing, or `--ci` skips it, and it was not run at base.

**After the change has merged**, the merge-base of HEAD and `origin/main` is HEAD itself or a
commit that already contains the change, so every row is green. The verifier passes
`--base <the parent of the delivering merge>`. Only the latest witness of a row counts, and it
must carry the row's current command; a later plain run or an edited command drops the record.
One parser (`briefVerifyRows`) enumerates the rows for the gate, the risk selection and the
runner, and Verify-shaped text it cannot place on a closure is a PROBLEM (see "Verify text the
runner cannot place" below). A live, mutating or
end-to-end row runs a second time at base, so check it is safe to run twice.

The lint gate on a closure **this branch makes** treats each risk-bearing row as follows. A row is
a **PROBLEM** when it has no fail-first witness, its witness is `unproven`, its witness is
non-discriminating, its witness is for an earlier command or has been superseded by a later plain
run, or its witness names a base that is not the merge-base or an ancestor of it, or names a base
that predates the brief (the merge-base holds the brief file at its current path and the base
does not), or, when the merge-base does not hold the brief at its current path (created or renamed
on the branch), names any base other than the merge-base itself. The last cases cover a red run on some other tree, on the branch itself, or on a
tree from before the brief existed (the root commit, where every row reds because the files are
missing), which proves nothing about this change. A non-risk-bearing row
that is non-discriminating is reported as a NOTICE and never blocks. `verifyrun --fail-first`
exits 1 when a risk-bearing row is non-discriminating and 2 when one could not run at base.

### What the base check does not prove

The gate proves that the base is the merge-base, or an older commit that held the brief file at
its current path. A brief created or renamed on the branch has no older ancestor that holds it at
that path, so for such a brief only the merge-base itself passes (rename history is not followed;
pass the default base). It does not prove that the base is the tree without THIS change. Any ancestor from the brief's
creation up to the merge-base passes, so which one counts as "without this change" (in a
post-merge verify, the parent of the delivering merge) is the verifier's attestation, and the
Base cell is self-attested Evidence text, like every witness cell. Binding the base to the
delivering change in code is a design decision tracked on the issue linked from the pull request
that introduced the gate, not something the lint decides today.

### Verify text the runner cannot place

`verifyrun` runs the rows of a table whose header names a `Command` column. On a closure the
branch makes, `## Verify` text that looks like Verify rows and that the runner cannot place is a
PROBLEM, because it is never run, witnessed or held to a fail-first record. Three shapes count:

- a numbered row that lost its table header, usually because a prose paragraph split the table;
- a table headed `Expect` (or `Expected`) with no `Command` column;
- a row too short to reach the `Command` cell.

Other tables are not reported. The `Failure mode of the work | Caught by` map the author-brief
skill allows beside the Verify table, a `Row | The mutation that must redden it` map and a legend
(`Class | Meaning`) are not Verify-shaped, so they stay where the skill tells authors to put them.
Registry entry: `verify-unplaced-rows`.
