# Criterion-level evidence freshness: offline study

At Assay commit `2f56cf2892afb4ce78aab1636344aa400bf3e93c`, a requirement's
criteria can change while the citing brief's old execution witness remains reusable.
That does **not** make the requirement rollup a false execution claim: it explicitly
reports authored state. It does mean that joining a `satisfied` rollup to a released
brief is insufficient evidence of **current criterion satisfaction**.

The smallest justified response from these fixtures is review guidance using existing
owners and explicit governing input declarations. No criterion identity system or new
runtime gate is justified by this study alone. A conservative source-version binding is
useful as a report-only comparison, but over-invalidates cosmetic changes and cannot
judge semantic equivalence or test adequacy.

This is research for graph-execution/32. It changes no product contract, evaluator,
schema, lifecycle state, release pin, skill or admission authority. Results are synthetic
local observations, not a deployment report or independent verification of this brief.

## Reproduce

From the repository root, with Python 3.12+ and the source's Go toolchain/dependencies
already cached:

```sh
export KUBECONFIG=/dev/null
python3 docs/research/criterion-evidence-freshness/replay.py --check
python3 docs/research/criterion-evidence-freshness/replay.py --self-test
```

The driver uses only Python's standard library. It exports the **pinned source commit**
with `git archive` (statusgen, forgeread and perfrec), builds that source with
`go build -trimpath`, and invokes the real `statusgen verifyrun`, `--coverage --json`
and `--requirements-rollup --json`. `GOWORK=off`, `GOTOOLCHAIN=local`, `GOENV=off`,
`GOPROXY=off` and `GOSUMDB=off` prohibit dependency/toolchain acquisition. A missing
tool, source object or cached dependency rejects the replay; it is not a no-gap result.
No Loft executable or package is needed. Existing Go caches are used; build outputs and
synthetic repositories stay in runtime temporary storage outside tracked content.

Each synthetic repository has a function returning 1, a named check asserting a value
of at least 1 and a positive inherited limit, a REQ with those criteria, a citing brief
with one Verify row, and a deliberately authored `done` board row. The board row and
REQ satisfaction are fixture inputs, **not** claims that a real independent review
occurred. Process-local Git identity is synthetic; no persistent Git configuration is
changed. Credentials, desk custody and provider environment are excluded from child
processes. The original row is `check` and runs locally; it makes no network call and
claims no network-namespace sandbox or production execution attestation.

The captured coverage/rollup stderr reports that this empty fixture HOME has no roster
or bound roles, even while these read-only modes return their offline tree-derived JSON
and exit 0. That diagnostic is retained, not cleared or replaced with a configured host
roster. These observations establish no role-gated identity, admission or write authority.

The runner executes the original row on a committed fixture revision and appends its
actual witness. The driver commits that witness together with one prescribed change,
then reads coverage and the authored rollup at the new subject. It never edits the
historical witness to match a revised Command. A separate direct Python probe measures
the fixture's current numeric condition; it is explicitly **not** a new Verify witness,
a semantic-review result, or proof that the named test remains adequate.

`results.json` retains argv, cwd, exit, stdout/stderr observations, original witness, exact
fixture revisions, source archive hash, driver/case hashes and tool versions. For successful
CLI `--json` output, `stdout` is a structured decoded JSON value with explicit provenance,
not a JSON document nested inside an escaped string. For example, a command's conjunction
remains the literal `&&` in its decoded command field; no command or semantic value is changed.

The producer first writes the **original process stdout bytes** to a private runtime
artifact, before any decoding. Provenance records its portable artifact identifier, SHA-256,
byte length, UTF-8 encoding and the pinned CLI's compact/indented JSON layout, HTML escaping
and final newline. Captured object member order is retained when writing results: order
does not change JSON meaning, but sorting it away would lose the original byte layout.
It verifies the decoded value reconstructs those exact bytes using that
declared codec. Reconstruction is validation only: it never generates a purported raw
capture. JSON string contents are unchanged. These finite cases need no path redaction
inside JSON output; a future output requiring that redaction is rejected as unestablished
byte equivalence, rather than silently normalized. Other output remains captured text.

Use `--raw-artifacts DIR` with a new private runtime directory to retain inspectable original
bytes at `DIR/<artifact identifier>`; with no flag the isolated runtime scratch holds them.
Artifact references in results never publish that directory's machine path. A missing
original artifact cannot be treated as proof it was inspected. Its digest is a byte
commitment, not execution authentication; independent fresh replay still has to run.

`<checkout>`,
`<scratch>` and `<fixture-home>` are portable path aliases substituted at capture to avoid
publishing local locators. The last names the deliberately empty synthetic HOME; its
configuration-file suffixes are preserved, consistently in argv/cwd/stdout/stderr.
The paths beneath the aliases and the argv are preserved. `--check` reruns every case and
compares saved observations with fresh ones. Every structured stdout's byte length/hash is
validated against its decoded value and declared codec before comparison. The check prints
how many original byte commitments match across runs, separately from semantic equality.
Known run-date/fixture-revision changes can change bytes without changing the normalized
semantic comparison; this is never reported as byte equality. Only temporary path aliases,
generated fixture commit identifiers, actual witness dates and reported tool versions are
normalized for comparison. Register dates, semantic output values, verdicts, reasons,
source/case/driver hashes and execution flags remain compared. Different tools may change observations;
such a difference must be investigated rather than accepted by updating a golden file.
`--record` explicitly writes fresh observations and is not part of the acceptance check.

## Frozen cases and independent expectations

`cases.json` was committed as `3770e9386` before the driver or any replay result was
committed or measured. Its Git blob is `7f0bdda125023bd5e807eba44b212dbc32085317`.
The expectations are authored judgments about obligations for **current acceptance**,
not expected statusgen outputs. They do not derive from either compared mechanism.
No production criterion IDs are introduced; case IDs name experiments only.

| Case | Independent obligation | Why |
|---|---|---|
| unchanged | neither | Same question and implementation. |
| tighten | review + execution | Value must now be at least 2; the old check accepted 1. |
| add | review + execution | Add an upper bound of 0; it conflicts with the old lower bound, which review must expose. |
| wording | neither | Stipulated equivalent numeric wording; this stipulation is a semantic judgment. |
| reorder | neither | Same conjunction, reordered. |
| split-duplicates | review | Establish multiplicity and identity after splitting/duplicating text; stipulated behavior remains equivalent. |
| retire | review | Authorize removal and retain history; no stronger behavior is created. |
| inherited | review + execution | The governing positive-limit input changes to zero. |
| command | review + execution | An additional file assertion changes what the row asks to run. |
| expect | review + execution | The row now requires an additional output token. |
| weaken-test | review + execution | Same function name, assertions removed; passing cannot restore adequacy. |
| unrelated | neither | A notes file is stipulated independent of the question and check. |
| unavailable | explicit unknown | The governing REQ is unavailable; no current acceptance claim is licensed. |
| declared-tighten | review + execution | Same tightening, with the governing REQ explicitly in existing `files:`. |

The unavailable case deliberately removes only a temporary fixture file. That is a
negative input, not a recommended retirement: the normative register requires a
withdrawn tombstone. The rollup returns no requirement for it, not a satisfied result.
The driver retains the known original reference and records `could-not-check` rather
than treating the empty rollup as current acceptance.

## Actual observations

All 14 original numeric checks actually execute and yield reusable baseline coverage.
After each prescribed change:

| Case | Existing coverage | Current numeric probe | Authored rollup |
|---|---|---|---|
| unchanged | released / pass | pass | satisfied |
| tighten | released / pass | fail | satisfied |
| add | released / pass | fail | satisfied |
| wording | released / pass | pass | satisfied |
| reorder | released / pass | pass | satisfied |
| split-duplicates | released / pass | pass | satisfied |
| retire | released / pass | pass | satisfied |
| inherited | released / pass | fail | satisfied |
| command | held / error | pass | satisfied |
| expect | held / error | pass | satisfied |
| weaken-test | held / wrong-revision | pass | satisfied |
| unrelated | held / wrong-revision | pass | satisfied |
| unavailable | released / pass | could-not-check; not executed | no requirement emitted |
| declared-tighten | held / wrong-revision | fail | satisfied |

The mechanism is visible in the raw coverage reasons: a Python script's read set is
underivable, so the conservative scope is every path outside `docs/streams/**` and
`STATUS.md`, plus applicable explicitly declared inputs. The undeclared REQ and inherited
input sit under that bookkeeping prefix. Their edits do not change Command or Expect.
This is a concrete residual in a **particular input layout**, not evidence that every
requirement edit bypasses freshness. The declared-tighten control credits the existing
`files:` mechanism: a declared non-brief REQ remains guarded even under that prefix.

The Command and Expect cases are held by existing acceptance-definition guards. The
weakened test is held because its script changes outside the bookkeeping prefix; keeping
its name does not evade that check. Conversely, the unrelated notes edit also holds:
the conservative script scope intentionally cannot infer the independence stipulated by
the study's oracle. A complete dependency manifest is a separate existing evaluator API
capability; the current CLI adapter supplies none. The study does not quietly activate
selective reuse or claim to measure its benefit.

The numeric probe passes after weakening the named test because the product still returns
1. It does not discover lost test assertions. Likewise it does not establish the new
Expect token. Those are separate adequacy/review obligations, and the report does not
collapse them into a numeric pass.

## Source inventory and the actual chain

All local source references below resolve at the pinned Assay commit. These are file/line
locations for review, not a claim that source inspection itself executes a check.

| Link | Owner and location | What is established |
|---|---|---|
| Ask → criterion | `spec/registers-v1.md:315`, `statusgen/requirements.go:59` | Authored REQ identity, nonempty string criteria and satisfaction claim. No independent criterion identities or criterion-to-row coverage. |
| Requirement → brief | `statusgen/traceability.go:69`, `:219` | `satisfies:` join and existence check. The orphan/untraced notices and dangling-reference problem are traceability, not semantic coverage. |
| Requirement → rollup | `statusgen/rollup.go:82`, `:160`, `:209` | Authored board statuses determine satisfied/partial/unknown. The printed note explicitly disclaims remeasurement. |
| Brief → Verify row → witness | `statusgen/coverage.go:164`, `statusgen/verifyrun.go:1303`, `:1890` | Parser supplies authored row text and recorded execution witnesses; real verifyrun executes the row and records its own result/hash/revision. |
| Witness → current row | `statusgen/evaluation/coverage.go:339` | Command equality, historical Expect comparison, revision applicability and dependency fingerprints. |
| Input applicability | `statusgen/evaluation/coverage.go:753`, `:798`; `coverage_inputs.go:1` | Bookkeeping exclusion, declared inputs, closed read-set derivation and conservative fallback. A script is underivable. |
| Public evaluator seam | `statusgen/evaluation/coverage_api.go:100`, `:148`; `statusgen/coverage.go:117` | Explicit observations/dependency manifests; CLI currently supplies no manifest. Missing knowledge is not automatic selective applicability. |
| Lifecycle meaning | `spec/lifecycle-v1.md:67`, `:94`; `spec/brief-v1.md:444` | Independent re-execution and revision-bound coverage remain owners; a changed Verify table requires amendment/demotion. No lifecycle is changed here. |

`spec/registers-v1.md:388` already says a citation is not criterion coverage. This study
does not turn that declared limitation into an assertion that the register violated its
contract. It demonstrates why a consumer must retain the difference between an authored
satisfaction claim, a row's applicable execution, and a judgment that the row answers the
current ask. The unavailable-source case further shows that a consumer keeping only an
empty rollup loses the fact that an expected governing source is missing.

Existing regression evidence is credited explicitly:

- `statusgen/coverage_test.go:319` rejects a changed Command; `:483` rejects a changed
  Expect even when only the brief file changed. These are the brief's row-3 controls.
- `:448` holds an external source change, `:1111` retains the old receipt revision,
  `:1152` checks changed policy input and `:1197` retains unknown environment input.
- `statusgen/coverage_inputs_test.go` covers derived input tracking, absent/unsupported
  paths and conservative fallback. These existing fixtures model supplied observations;
  they are not fabricated execution witnesses for the research's numeric cases.
- `statusgen/traceability_test.go:135` pins dangling references;
  `statusgen/rollup_test.go:16` pins the rollup's three-state authored-state semantics.

The [pinned Loft reconciliation reference](https://github.com/Ask-Loft/loft/blob/9bed930a32cd6bdc09a7274f45a4c665daf305de/crates/loft-cli/skill/loft-spec/references/reconciling.md)
provides a comparison of reviewed node-to-test links. It explicitly finds test names
without executing them and marks changed nodes as needing review. That is a useful
separation of mapping review from execution, not an adopted dependency or proof that
test-name presence establishes adequate current acceptance. This study runs no Loft tool.

## Report-only comparison and decision

For the demonstrated gap, the experiment uses existing `REQ-numeric-condition`, its
source bytes plus the inherited source, and existing Verify row 1. A source-byte revision
change predicts **review binding owed**; missing governing content predicts **unknown**;
otherwise it defers to baseline. These are explicitly labelled `experimental_prediction`
in results, separate from actual statusgen output. The comparator grants no acceptance,
chooses no production criterion identities and never modifies either evaluator or witness.

It flags the tightening, addition and inherited-input cases, but also flags wording and
reorder, where the independent oracle says neither obligation changed. Split/duplicate
and retirement cases need review rather than an automatic execution demand. It cannot
infer which test is adequate, and it adds nothing to the already guarded Command/Expect
or changed-test cases. Hash equality is therefore a bounded change detector, not a
semantic oracle. A hash match also says nothing about an unavailable external source,
environment, new inherited obligation or changing membership that was never included.

Prefer the following **guidance-only** response now, subject to independent review:

1. Preserve historical evidence and its original question/revision. Before treating a
   satisfied REQ as currently met, read the current criterion against the cited Verify
   rows and execution. Record missing coverage as unknown or owed work, not alignment.
2. Make governing REQ and inherited source inputs explicit through the existing input
   owners. The declared-tighten control demonstrates an existing path for this file
   layout. Do not assume a `satisfies:` citation automatically creates that dependency.
3. On a semantic acceptance change, amend the owning Verify row and execute the adequate
   revised check. On cosmetic equivalence, retain the semantic review rationale rather
   than rewriting the old witness or forcing a new production criterion ID.

No source change or new register is enacted by that recommendation. If measured adopter
failures or unacceptable review cost later justify automation, scope a separate reviewed
follow-on in the existing requirement/evaluation owners: bind a reviewed requirement
source revision to existing rows, retain explicit unknowns and a rationale for compatible
edits, and prove it against these and independent holdout cases. Questions owed before
that implementation include who approves mappings/retirements, which sources are complete,
what invalidates acceptance, migration of legacy unbound records, and where a missing
source must hold. This report grants none of that authority.

Criterion-level identities could make additions, split/merge and retirement more precise,
but require durable identity/history, mapping review, duplicate semantics and ongoing
author/reviewer effort. This small corpus supplies no evidence that those costs are needed
over conservative requirement-version binding plus review. Existing criteria remain valid
strings and legacy records remain unbound; no satisfaction history is demoted or rewritten.

Ownership overlap stays explicit: graph-execution/28 plans candidate/acceptance identities
and fact completeness; /29 plans the old/new regression floor; /30 plans applicability
context and unknown/membership/expiry handling; /31 plans corroborated provenance. All are
prospective at this source revision. A follow-on must extend those owners and the existing
coverage API rather than invent a parallel acceptance, principal or freshness authority.

## Controls, limits and verification

`--self-test` first proves identical genuine observations are accepted. It then runs
child comparisons that must exit 1 on a tampered observation, omitted case, duplicate
case, fabricated execution flag, unavailable-source-as-pass, prediction-as-measured,
wrong raw-byte commitment and a changed decoded value. The latter two protect only the
research serialization's provenance consistency; they add no product acceptance control.
`--check` independently executes the actual baseline again; self-test's fixtures alone
would establish only comparator behavior. Neither mechanism authenticates an execution
against a malicious author who rewrites both the driver and results: independent source,
case-oracle and code review remain required. A local execution table is not cryptographic
proof or a claim of independent human acceptance.

The saved observation set is nine current releases and five holds. The corpus is bounded
and synthetic, uses one numeric implementation and one input layout, and does not sample
real projects, provider nondeterminism, live authority, deployment or human review cost.
The numeric oracle, cosmetic equivalence and independence judgments need independent
review. Changed-test coverage shows file freshness, not detection of semantically weak
tests after they are freshly executed. No conclusion here waives that review.

The brief's Verify rows remain unchanged. Its Evidence section belongs to the independent
verifier; worker validation and CI receipts belong to the draft PR workpad.
