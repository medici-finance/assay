---
module: cmd/example
s-row: S-example
investigation: docs/investigations/2026-01-21-cmd-example.md
divergence: intent-changed
pinned-source: r10
candidate-source: r13
dependency-references: docs/streams/decisions/DR-example-token.md docs/contracts.md docs/example-write-policy.md internal/credhelper/rotate.go
tier: strong
date: "2026-01-22"
---

# Refactor oracle template

A **refactor oracle** is what a redesign is coded against. A brief records intent when it is
written and then drifts; an incident records what broke, not what must hold. Neither is enough
to refactor from. The oracle reconciles them into four parts and an acceptance rule: the
intent, the failure modes, the module's current behaviour (characterization tests, triaged
intent versus accident), and the invariants. The refactor lands only when all four hold.

**When.** Only when a brittle investigation (`docs/brittle-investigation-template.md`)
recommends `redesign`. The oracle is the act before the design brief; the design brief
carries `oracle: docs/investigations/<file>` beside its `design-fit:` and copies the five rows
of `## 5. Acceptance` into its Verify table. `reconcile`, `accept` and `clear` need no oracle,
and no ordinary task does.

**How to use this file.** Copy it to `docs/investigations/<yyyy-mm-dd>-<module-slug>-oracle.md`,
beside the investigation it reads. Keep the frontmatter keys, replace the values, and under
each section replace the instruction and the worked example with your own. Everything named
`example` (`cmd/example`, `S-example`, `DR-example-token`, briefs `example/01` to `example/03`,
`docs/example-write-policy.md`, `internal/credhelper`, class `#0`, revisions `r1`–`r13`) is
fictional and continues the investigation template's example: its superseding revision
(`2026-01-21`) read `intent-changed` under policy P2 and recommended `redesign`. A real oracle
writes full commit ids where the example writes `r<N>`.

**Bounds.** Strong tier. The oracle is assembled by whoever will code the redesign, and it
contains generated tests; the investigation stays read-only. Generating the tests is cheap.
The triage table is the judgement, and it is the record the PR carries.

**Frontmatter keys.**

- `module`, `s-row` — as the investigation names them.
- `investigation` — the investigation file this oracle reads; a superseded one is never read.
- `divergence` — copied from that file; the acceptance rule checks they agree.
- `pinned-source` — the revision every read below ran at.
- `candidate-source` — the revision the acceptance rows last ran at: the refactor PR's head.
  Final evidence names this one, never `pinned-source`.
- `dependency-references` — every path a conclusion here rests on that is not the module's own
  file (the investigation's list, plus any the oracle adds). Empty means unknown coverage.
- `tier` — always `strong`. `date` — the day this file was written.

## 1. Intent

Source: the investigation's `## Intent` (its quotes of the owning brief, the `DR-` record and
the `S-` row) and its `divergence:` and reconciled reading. Quote them; do not re-derive them.
Where the brief and the code disagree, the investigation decided which is right; record that
decision so the refactor codes against the reconciled reading, not either document. Reads:

- `sed -n '/^## Intent/,/^## What happened/p' <investigation>` and
  `grep -E '^(divergence|recommendation|supersedes):' <investigation>`.
- Reuse is valid only at the investigation's recorded inputs: run
  `git diff --name-only <as-of> <pinned-source> -- <module> <dependency-references>` (the
  investigation template's §Reuse and revalidation). A printed path makes each conclusion that
  rests on it `revalidate`: supersede the investigation, or record the revalidation here.

Record each revalidation as one line, `revalidated: <path> @ <merge-base> — <basis>` (the
revision acceptance row 1 diffs to; a merge of main moves it, so rewrite these lines after one), naming the conclusions re-checked and the result. Unknown applicability is not a basis, and cannot satisfy
a `keep` row that rests on the conclusion.

**Worked example.** At `pinned-source` r10. The investigation `2026-01-21-cmd-example.md`
(as-of r10, `supersedes:` the r7 file) reads `divergence: intent-changed`: under policy P2 a
per-write token read costs ten units of a 5000-per-hour budget, so a peak of 900 writes needs
9000. `DR-example-token` is amended at r10: *"decided: one token copy per process, dropped on
the credential helper's rotation signal; never refreshed on 401, which hides a rotation failure
behind a retry."* Reconciled intent: write as the current identity, keep at most one copy, drop
it when rotation is signalled (A3: the signal precedes revocation, rests on
`internal/credhelper/rotate.go` @ r10). The design brief is `example/03`.

### Intervening-change example

Two commits land on main between assembly and the candidate: r11, an API change in
`internal/credhelper/rotate.go` (`OnRotate` now fires after the old token is revoked, not
before); r12, an edit to `cmd/example/help.go` that rewords the help text. The refactor PR
branches from r12 (its `<merge-base>`), and its own commit r13 is the `candidate-source`.
Acceptance row 1 printed only `SOURCES-READ` at r10. At r13 its diff, r10 to the merge-base
r12, names both paths, and before any revalidation is recorded it prints:

```
UNREVALIDATED cmd/example/help.go
UNREVALIDATED internal/credhelper/rotate.go
SOURCES-READ
```

The refactor's own edits to `cmd/example` in r13 are not in that diff: they are the change
rows 3 to 5 judge, not an input to revalidate.

Nothing that ran at r10 is carried forward: evidence names r13 and is re-run there. r11 is
relevant: A3 fails, so the reconciled intent no longer settles when the copy is dropped, and
every row of §3 that rests on A3 is `revalidate` until a superseding investigation (or a DR
amendment) reads the new signal order. No `revalidated:` line can be written for it, so row 1
keeps refusing on `internal/credhelper/rotate.go` until then. r12 is reused, with its basis
recorded as one line in this section:

revalidated: cmd/example/help.go @ r12 — help text only; no conclusion and no §3 row asserts the help output (checked: every row's test name and reason); semantic relevance is the review's judgement, no command proves it.

That last clause is the rule: the diff proves which paths changed; whether a change matters is
judged at review, and a reviewer may reject the basis.

## 2. Failure modes

Source: every incident in the class issue and every findings entry whose `affects:` names the
module, each mapped to the regression test that pins it. Reads:

- `gh issue view <class-issue> --comments` (or the forge's equivalent): the counted
  `incident-group` blocks, as the investigation's `## What happened` lists them.
- `grep -lE '^affects:.*"<stream>/<NN>"' docs/streams/findings/*.md` for each brief id the
  investigation's Intent read 4 resolved.
- For each instance, its test: `git grep -n 'regression: .*#<N>' -- '*_test.go'`, and for a
  class or a finding `git grep -n 'regression: .*class #<N>' -- '*_test.go'` or
  `git grep -n 'regression: .*F-<slug>' -- '*_test.go'`.

One row per failure mode: `| ref | what broke | test | package |`. A failure mode with no
tagged test keeps an **empty test cell**; acceptance row 2 refuses it, so write the test (with
its `// regression:` tag) before the refactor, never after.

**Worked example.**

| ref | what broke | test | package |
|---|---|---|---|
| class #0 g1 | a refreshed token is not re-read: `post` wrote as the old identity | TestPostAfterRotation | cmd/example |
| class #0 g2 | the stale-cache refusal fired on a valid token | TestStaleCacheRefusal | cmd/example |

## 3. Current behaviour

Source: the owner package the `S-` row names. Generate characterization tests over its public
surface, one test per observable behaviour, asserting what the code does today, not what it
should (Feathers). For a library package the surface is every exported function and method.
A command (`cmd/<verb>`, package `main`) exports nothing; its surface is its CLI: every flag,
exit code and output line, and every forge or file write, driven through its entry function.
Reads, from the repository root: `grep -n '^| S-' docs/contracts.md | grep -F '<module>'` for
the owner; then for a library `go -C <owner package> doc -all .`, and for a command
`go -C <module> doc -cmd -u -all .` (its usage, exit codes and entry functions) and
`git grep -nE '\.(Bool|Duration|Float64|Int|Int64|String|Uint|Uint64|BoolFunc|Func)\(("[^"]*"|[A-Za-z_][A-Za-z_.]*),|\.(Bool|Duration|Float64|Int|Int64|String|Text|Uint|Uint64)?Var\([^,]+, *("[^"]*"|[A-Za-z_][A-Za-z_.]*),|(^|[^A-Za-z_])Flag: *("|[A-Za-z_])' -- '<module>/*.go' ':!<module>/*_test.go'`
(its flags). The read covers the `flag`/`pflag` call forms with a literal or a named-constant flag
name, and the `Flag:` key of a binding table. A `Flag: k` line inside a helper stands for the flags
the helper builds, and a flag declared another way is outside the read: run the command with `-h` and add every flag it prints that the
read missed to the triage table as its own row, so the surface is not the grep's shape. Each test carries, directly above its declaration,
`// characterization: <module> <oracle-file>`, two fields, so the test ledger can list it.
A replay corpus for the module, where one exists, may be cited here as a source.

**The triage table.** One row per characterization test: `| test | verdict | reason | maps to |`.
The test cell is the bare function name. The verdict is one of exactly three words:

- `keep` — intent. Maps to an `S-` row, a `DR-` sentence, a brief fact or an incident.
- `drop` — an accident or a bug. Maps to the failure mode or the divergence line that says so.
  The test leaves in the refactor with the trailer `Retires-test: <name> — accident per oracle §3`.
- `unknown` — the record does not settle it. A `drop` whose reason names no source is an
  `unknown`. An `unknown` at landing is a refusal: `NEEDS_CONTEXT` to the design brief's review.

Nothing else is a verdict, and `unknown` is never rewritten to land: an oracle with an
`unknown` row cannot pass row 3.

**Worked example.** Four characterization tests over the `cmd/example` CLI at r10:

| test | verdict | reason | maps to |
|---|---|---|---|
| TestCharPostWritesStatusNote | keep | the verb's purpose: `example/01` why: "writes a status note for a change" | S-example |
| TestCharPostRefusesRevokedToken | keep | never write as a revoked identity; the failure the DR predicted | DR-example-token; class #0 g1 |
| TestCharRetryOn401Refreshes | drop | the 401 refresh-and-retry branch (r4); the amended DR rules it out | DR-example-token; history row r4 |
| TestCharNoCacheFlagBypasses | unknown | `--no-cache` (r6) was a workaround; whether any caller still passes it is unrecorded | none: no source settles it |

## 4. Invariants

Source: the module's `S-` row and its contracts entry, the rule-register rows that serve it,
the architecture rules and the weight line. The refactor must leave every one green, with the
owner still declared by its `// semantic:` marker. Reads:

- `grep -n '^| S-' docs/contracts.md | grep -F '<module>'` and
  `grep -n '^| R-' docs/contracts.md | grep -F '<s-row>'`.
- `git grep -n '// semantic: <s-row>'`: the marker above the owner's declaration.
- `go -C tools/desk test ./internal/arch/ -count=1`: the architecture rules.
- `go -C tools/desk test ./internal/weight/ -run TestPrintWeight -count=1 -v`: the weight
  line, at the merge-base and at the candidate.

**Worked example.** `S-example` (owner `internal/example/token.go`, marker present at r10);
register row `R-example-token-owner`; arch green at r10; weight unchanged is not required, but
every added verb, flag or refusal is argued in the design brief's `design-fit:`.

## 5. Acceptance

The refactor lands only when all five rows hold at `candidate-source`. The design brief copies
them into its Verify table with the placeholders filled; `<merge-base>` is the refactor PR's
merge-base and `<N>` its number. Every row runs from the repository root and none changes the
shell's directory: Go commands name their directory with `go -C`.

| # | Command | Expect |
|---|---------|--------|
| 1 | `a=$(grep -m1 '^divergence:' <investigation>) && b=$(grep -m1 '^divergence:' <oracle-file>) && test "$a" = "$b" && d=$(git diff --name-only <pinned-source> <merge-base> -- <module> <dependency-references>) && printf '%s\n' "$d" \| while read -r p; do test -z "$p" \|\| grep -qF "revalidated: $p @ <merge-base>" <oracle-file> \|\| echo "UNREVALIDATED $p"; done && echo SOURCES-READ` | exactly one line, `SOURCES-READ`: both files carry the same `divergence:` line, both revisions resolve, and every path that changed on main between assembly and the candidate's merge-base (never the refactor's own edits) has a `revalidated:` line in §1 at `<merge-base>`. An `UNREVALIDATED` line refuses (the intent is not current); so does a missing `SOURCES-READ` (a `divergence:` line missing or differing, or a source that does not resolve) |
| 2 | `sed -n '/^## 2\. Failure modes/,/^## 3\./p' <oracle-file> \| grep -E '^[\|] *(#\|F-\|class #)' \| awk -F'\|' '{t=$4; gsub(/[ ]/,"",t); print (t == "" ? "EMPTY-TEST " $2 : t)}'` | no `EMPTY-TEST` line; each printed test is tagged and passes in row 4 |
| 3 | `sed -n '/^## 3\. Current behaviour/,/^## 4\./p' <oracle-file> \| grep -E '^[\|] *Test' \| awk -F'\|' '{gsub(/ /,"",$3); print $3}' \| sort \| uniq -c && o=$(sed -n '/^## 3\. Current behaviour/,/^## 4\./p' <oracle-file> \| grep -E '^[\|] *Test') && p=$(gh pr view <N> --json body --jq .body \| tr -d '\r' \| sed -n '/^## Oracle$/,/^## /p' \| grep -E '^[\|] *Test') && test "$o" = "$p" && echo ORACLE-VERBATIM` | only `keep` and `drop` counted, then `ORACLE-VERBATIM` (the PR body's `## Oracle` section carries the §3 triage rows byte for byte); any `unknown`, any other word, or a missing or differing table refuses |
| 4 | `go -C <module> test ./... -count=1 -v 2>&1 \| awk '/^[[:space:]]*--- (FAIL\|SKIP)\|^FAIL\|^go: /{b++} /^[[:space:]]*--- PASS/{p++} END{print b+0, p+0}'; go -C tools/desk test ./internal/testledger/ -run TestReportTestLedger -count=1 -v -args -base=<merge-base> -head=<candidate-source> 2>&1 \| grep -E 'untrailed\|characterization-untriaged\|could-not-check\|retired-trailed\|^[[:space:]]*--- (FAIL\|SKIP)\|^FAIL'` | `0 N` with N ≥ the number of tests named in §2 and §3 (a `go:` error line, such as a missing module directory, counts as a failure, and a module where nothing ran prints `0 0`), then only `retired-trailed` lines, each `drop` test named in one whose quoted reason is `accident per oracle §3`: every `keep` and failure-mode test passes, and every `drop` left with the trailer. An `untrailed`, `characterization-untriaged`, `could-not-check`, `SKIP` or `FAIL` line refuses (a skipped or unreadable report is never clean) |
| 5 | `go -C tools/desk test ./internal/arch/ -count=1 && git grep -c '// semantic: <s-row>' <candidate-source> -- <owner path>` | `ok`, then ≥ `1` (the invariants hold and the owner is still declared) |

**Worked example.** At r10, before the intervening change:

- Row 1: `SOURCES-READ` only.
- Row 2: `TestPostAfterRotation`, `TestStaleCacheRefusal`; no `EMPTY-TEST`.
- Row 3: `2 keep`, `1 drop`, `1 unknown`: **refused** on the `unknown` row
  (`TestCharNoCacheFlagBypasses`). `example/03` goes to review as `NEEDS_CONTEXT`, naming the
  question: does any caller still pass `--no-cache`? It cannot land until the row is answered
  `keep` or `drop` with a source.
- Rows 4 and 5 are not reached; a refusal on any row is a refusal of the whole rule.
