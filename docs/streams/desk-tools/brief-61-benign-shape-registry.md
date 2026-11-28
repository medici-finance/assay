---
brief: assay:assay:desk-tools:61
title: "Declare the secret scan's benign shapes in one registry, each entry bounded by a refusal it must keep"
why: >-
  The shared secret scan refuses every run of 32 or more characters from the base64 alphabet,
  then lets one through only when one of fourteen hand-ordered predicates or three structured
  spans recognises it. Each false positive so far was fixed by adding one more predicate or one
  more closed-list word, ten widenings since August, and three of them had to be narrowed again
  after a widening let real-shaped material through. This brief moves every existing admission
  into one declared registry. Each entry names its context, the issue that added it, the
  examples it admits and the refusals that bound it. A test then proves, entry by entry, that
  widening the entry flips a refusal it must keep. Behaviour does not change. The next benign
  shape becomes one reviewed entry with its paired refusal, instead of one more branch in a
  chain nobody can audit.
wave: 1
depends: []
unblocks: ["desk-tools/63"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: >-
  The scan is a trust-boundary control: it is what keeps a credential out of every body, branch
  diff and Evidence landing the desk verbs write. A refactor of its admission logic can un-refuse
  a credential shape without any test noticing, which is how one earlier widening admitted eight
  synthetic credentials at its head. The human confirms the registry design, and confirms from
  the evidence that the migration is verdict-for-verdict identical to the pre-migration scanner
  before any later brief widens an entry.
decision-trigger: creation
issues: [4284]
decision-issue: 6804
ruling: "https://github.com/medici-finance/assay/issues/6804#issuecomment-7102966257"
outcome: none
design: DR-scan-admission
schema: brief-v2
version: 1
authored: 2026-11-10 by a desk-dispatched authoring session (error-class design brief for issue 4284)
sources:
  - "medici-finance/assay#4284 — the error-class seed: the scan clears benign shapes by enumerated exception; hypothesised invariant: an enumerated, tested class list whose every entry carries a refused-before and admitted-after repro set"
  - "docs/streams/build-less-brittle/spec.md §4.1 (design-owed trigger: 3 counted incident groups or a 2nd merged fix; both measures are met here) and §4.2 rules 1 and 2 (consolidate meaning, keep independent enforcement; a retired trust-boundary control names the still-refusing layer)"
  - "docs/contracts.md rows S-publication-scan and R-body-secret-scan — invariant: loosening the entropy heuristic never un-refuses a token shape"
  - "widening commits on the scan, oldest first: 73209fbdc, 8d90074d3, cea8ebb75, 153a80b27 (#750), 2671dc78b (#883), 0f2b5b18f (#966), 6824fb83a (#1161), be4380ce5 (#1643), ccf05f412 (#2036), 0f17a7f3f (#4077)"
  - "narrowing commits that followed a widening: e9e4e2b40 (after #1643 admitted eight synthetic credentials at its head), a34665a1a (Go subtest tail cap), 36d24d520 (decrypted k8s secret owner proof)"
  - "credential arms added beside the chain, not exemptions: #5332, #5348 (URL userinfo); #5320 constrained that arm's email exemption"
  - "freshness-checked 2026-11-10 @ 450ac4047 (origin/main): 14 admission predicates in scanSurface's high-entropy loop, 3 structured span formats, 7 main closed word lists; corpus 17 negative and 21 positive fixtures; mutations.json 146 mutations, 28 on bodycheck.go and 5 on structured.go plus the control"
exec-tier: strong
exec-tier-why: "A behaviour-preserving refactor of a trust-boundary control, with an equivalence proof against the pre-migration tree and a data-driven mutation per entry."
domain: complicated
consumers:
  - "tools/desk/README.md: fixed-here (the BodyCheck row's exemption prose is replaced by the table the registry renders)"
  - "docs/contracts.md: fixed-here (R-body-secret-scan's tests column names the registry tests)"
  - "tools/desk/internal/deskkit/testdata/corpus/README.md: fixed-here (states that each negative fixture is named by a registry entry and each entry by a positive fixture)"
id: e7164824-71da-4834-a4b3-21496f1caaed
---

# Brief 61 — Declare the secret scan's benign shapes in one registry

## Dependencies

None typed. It builds on work already landed: desk-tools/14 (the golden corpus and
`--explain`), desktools-v2/10 (one outbound-write check, which moves existing scans and drops
none), and build-less-brittle/10 (one implementation per registered meaning;
`S-publication-scan` has a ceiling of 1 and its one marker sits on `scanSurface`). The worker
change in flight for #1642, #5825 and #5974 (the `KEY=<value>` exemption) is not duplicated.
Whatever form it lands in becomes one more registry entry during this migration. If it lands
after this brief, it lands as one entry.

desk-tools/63 consumes this registry for the new shapes. desk-tools/62 and desk-tools/64 are
independent of it.

## Context

files:
- `tools/desk/internal/deskkit/benignshapes.go` (planned) — NEW: the registry type and its entries
- `tools/desk/internal/deskkit/benignshapes_test.go` (planned) — NEW: registry, bound and parity tests
- `tools/desk/internal/deskkit/benigngolden_test.go` (planned) — NEW: the verdict-equivalence test
- `tools/desk/internal/deskkit/testdata/benign-golden.txt` (planned) — NEW: recorded verdicts
- `tools/desk/internal/deskkit/bodycheck.go` — `scanSurface`'s high-entropy loop walks the registry instead of the predicate chain; the predicates stay where they are as the entries' implementations; the high-entropy refusal message is rendered from the registry
- `tools/desk/internal/deskkit/structured.go` — `structuredExemptSpans`' three formats become artifact-context entries; their grammars do not change
- `tools/desk/internal/deskkit/corpus_test.go` — the hand-written `required` map in `TestBodycheckCorpusCoversEveryCataloguedShape` is derived from the registry
- `tools/desk/internal/deskkit/mutations.json` — anchors re-pointed where code moved; no mutation dropped
- `tools/desk/README.md`, `docs/contracts.md`, `tools/desk/internal/deskkit/testdata/corpus/README.md` — see consumers
- **add** — `changelog/<branch-slug>.md` — one fragment

facts:
- **What the heuristic decides.** After the literal credential arms that run first (GitHub and
  GitLab token prefixes, the AWS access-key-id shape, PEM blocks, JWTs, a decrypted Kubernetes
  Secret, a sops block, the ruling-claim guard), `scanSurface` takes every run of the class
  `[A-Za-z0-9+/=]{32,}` and refuses it. The URL-userinfo arm runs after that loop, last, so
  every refusal before it keeps its rule id and message. It computes no entropy. The verdict is length plus
  alphabet, and a run is admitted only when one of 14 predicates says yes or the run sits inside
  a span `structuredExemptSpans` admits. In chain order the predicates are: git SHA, path-like,
  identifier-like, assignment-like, all-equals, digest-key assignment, doc-path hex segment,
  issue-number list, enum slash list, k8s UID hex segment, PGP fingerprint, quantifier-glued
  path, Go subtest name, diffstat bar. The 3 structured formats are a go.sum line, an SRI
  integrity value, and a sanctioned sops document (complete envelope, metadata block, armor
  bodies). The second two formats count only when the whole scanned surface parses as such a
  document. Seven main closed lists feed the predicates: numeronyms, the OAuth term, two-letter
  words, digest keys, credential stems, armor labels, k8s UID prefixes. Smaller ones also exist,
  among them `goTestPrefixes` in `bodycheck.go` and a second armor-label list in
  `structured.go`. The registry absorbs every list a predicate reads, not only the seven.
- **Why it recurs.** Admission is an open list of special cases over a decision that knows
  only length and alphabet. Every benign shape the scan has not seen is refused, and the only
  remedy the code offers is one more case. The record: 10 widening commits since 2026-08-27,
  and 3 narrowings after widenings admitted real-shaped material. One of those, after #1643,
  admitted eight synthetic credentials at the PR's head. There are 8 open false-positive issues
  (#1642, #929, #2321, #3196, #4723, #5257, #5825, #5974), and #4076 was closed by the latest
  widening. No predicate is required to name the refusal that bounds it, so none of the
  narrowings was caught by a test before review.
- **The second mechanism** (handled by desk-tools/63 and desk-tools/64, not here): artifact
  grammars such as a sops document can only be recognised on a whole document. The branch-diff
  surface hands the scan fragments, so the same armor that is admitted in a whole file is
  refused in a diff hunk (#5257).
- **What does not change.** The literal credential arms are not registry entries and cannot be
  suppressed by one. They run first, exactly as today, and are the independent layer that still
  refuses a known credential shape even if every registry entry were wrong (§4.2 rule 1). The
  run class, the 32-character threshold and the refusal for an unknown opaque run all stay
  exactly as they are.
- **Proof surface already present.** Tests `TestTokenShapesStillRefused`,
  `TestNewShapesStillRefuse`, `TestBodyCheckURLUserinfo`, the corpus tests
  (`TestBodycheckCorpus`, `TestBodycheckPositives`, `TestBodycheckCorpusCanFail`, and the
  coverage test named above), `TestExplainNamesRuleAndLineNeverSpan`, and the 34 mutation
  entries on `bodycheck.go` and `structured.go` (the control included) in `mutations.json`.
  The corpus README's rule: "the positive corpus is never edited to pass".

single-point-of-failure: the registry decides only which high-entropy runs are admitted. If a
registry entry is wrong, the literal arms still refuse every known credential shape, and
`TestBenignKeepsLiteralArms` (planned) proves that with every entry forced open at once. What the literal
arms do not cover is an unknown opaque credential. For those the bound is each entry's paired
refusal, proven by `TestBenignEntriesBounded` (planned).

layering: one data table inside the owner (`deskkit`, beside `scanSurface`) and no new package.
An entry is a value with fields `ID`, `Context` (`run`, `line` or `artifact`), `Doc` (one line),
`Issue`, `Admitted` (examples or negative fixture names), `Bounds` (refusal examples or positive
fixture names) and one admission function: a run predicate, a run-in-line predicate given the raw
text and offsets, or a span finder over an artifact. `scanSurface` takes the registry as a
parameter internally so a test can run it with a modified registry. Production has one
registry value.

design-fit:
  owner: tools/desk/internal/deskkit — scanSurface, the one marked implementation of S-publication-scan
  contract: S-publication-scan (trust-boundary row R-body-secret-scan)
  retires: [the hand-ordered admission chain in scanSurface's high-entropy loop, the hand-written exemption list in the high-entropy refusal message, the hand-maintained required map in the corpus coverage test, the BodyCheck exemption prose in tools/desk/README.md]
  weight: "verbs 0, flags 0, refusals 0 (the refused set is unchanged); rule-text lines down (the message and README prose are rendered, not written); +1 data table, +6 tests, +1 recorded golden"
  why-add: "The registry replaces the chain inside its owner rather than sitting beside it. Removing the entropy loop and keeping only the literal arms was considered and rejected: it un-refuses unknown opaque runs, which R-body-secret-scan's invariant forbids."

## Human decision

The desk's shared secret scan refuses every long run of letters, digits and base64 symbols,
then lets one through only when one of fourteen special-case checks recognises it. Each false
alarm so far has been fixed by adding one more check. Ten such loosenings have landed, and
three had to be tightened again after a loosening let real-looking credentials through. The
proposal moves every existing check, unchanged, into one declared list. Each entry must name the
refusals it must never admit, and a test proves that widening any entry breaks one of them.
Nothing the scan refuses today changes. The decision is whether to adopt this design before any
new benign shape is added.

Options:
1. **Adopt the declared list, behaviour-preserving, in one change** — every existing check moves
   into the list with its bounding refusals. A recorded-verdict test, run against both the old
   and the new code, proves nothing changed. New shapes then arrive one reviewed entry at a time.
2. **Adopt the list for new shapes only** — the existing checks stay as code and only new
   shapes get entries. This is cheaper now, but leaves the unaudited part as it is and keeps
   two ways to admit a run.
3. **Keep fixing one case at a time** — no structural change. Each new false alarm is one more
   check, as today.

Default if no answer: none — blocks until answered. Option 1 is the recommendation. A change
to a credential control is never adopted by silence.

## Ground rules
- Ruled option A on the decision issue, which is option 1 above: https://github.com/medici-finance/assay/issues/4284#issuecomment-6774184501
  The ruling adopts the declared registry. Every existing admission moves into it unchanged,
  proven by a zero-verdict-change golden. Options 2 and 3 are not taken.
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- This brief changes no verdict. A test that needs a changed expectation is a STOP: report it,
  never edit the expectation.
- Never edit a fixture under `testdata/corpus/`. Its README is edited only as task 9 says,
  and row 5 allows exactly that one file. Never write a credential-shaped literal into a test
  or a fixture. Generate such strings at test time from a fixed seed, or split them with the
  corpus README's seam.
- Do not touch the literal credential arms, the run class or the threshold.

## Task
1. **Record the golden first, against the unmigrated scanner.** Add `benigngolden_test.go` (planned) and
   `testdata/benign-golden.txt` in the first commit, before any production code moves. The test
   uses only API present at the delivery base (`ScanSurfaceSecrets`, the `*ScanFinding` it returns, and
   `corpus_test.go`'s `loadCorpus`). Its inputs are derived, not written: every corpus fixture's payload whole and
   line by line, and from a fixed seed, at least 2,000 generated runs of length 30 to 90 over the
   base64, hex and alphanumeric alphabets. Each generated run is also placed into every negative
   fixture's payload lines, at line start, after `=`, after `/` and after `@sha256:`. The golden holds
   one line per input, `<id>` TAB `<verdict>`. The id is stable: `corpus:<file>` or
   `corpus:<file>:<line>` for fixture inputs, and
   `gen:<index>` or `gen:<index>:<file>:<line>:<position>` for generated ones. The verdict is
   `<rule>:<line>` or `clean`. Later briefs read this format. `TestBenignGoldenVerdicts` (planned)
   regenerates the inputs, asserts that every one matches the golden, and asserts that the golden
   holds at least 1,000 refused and at least 100 clean inputs. The migration commits never edit
   the golden.
2. **Introduce the registry.** In `benignshapes.go`, declare one entry per existing admission:
   the 14 predicates (context `run` or `line`, as their signatures already are) and the 3
   structured formats (context `artifact`, still evaluated over the whole scanned surface
   exactly as `sanctionedSopsDocument` does today). Fill every field from the record: `Issue`
   comes from the commit or issue that added the shape (the sources list above), `Admitted` from
   the negative fixtures or test cases that cover it, and `Bounds` from the positive fixtures or
   refusal cases that bound it. Where an entry has no bounding refusal today, add one to the
   registry's in-test bound set. Write it as a generated or seam-split string, never a literal
   and never a corpus edit.
3. **Walk the registry.** Replace the chain in `scanSurface`'s high-entropy loop with a walk of
   the registry. The literal arms keep their current positions on either side of it: the arms
   that run before the loop today still run before the walk, and the URL-userinfo arm still
   runs after it, last. `--explain` and
   `ScanFinding` keep their fields. For an admitted run `--explain` may also name the admitting
   entry's `ID`, as an addition only.
4. **Render the text.** The high-entropy refusal message is rendered from the registry's `Doc`
   lines. `TestBenignRefusalText` (planned) asserts that every entry's `Doc` appears in it and that it still
   names `--explain`. The BodyCheck row in `tools/desk/README.md` carries the table the registry
   renders, between `<!-- benign-shapes:begin -->` and `<!-- benign-shapes:end -->`.
   `TestBenignDocsParity` (planned) fails when the table and the registry differ.
5. **Prove every entry is bounded.** `TestBenignEntriesBounded` (planned) runs, for each entry in turn,
   the scanner with that entry's admission forced to `true`. It asserts that at least one of the
   entry's `Bounds` flips from refused to clean, and an entry whose widening flips nothing fails,
   named. This is the data-driven mutation: it covers each future entry without a new
   `mutations.json` line.
6. **Prove the literal arms are independent.** `TestBenignKeepsLiteralArms` (planned) forces every entry
   open at once and asserts that every positive fixture whose rule is a literal arm still refuses
   with the same rule.
7. **Check the registry's shape.** `TestBenignRegistryComplete` (planned) asserts unique IDs, a known
   `Context`, a non-empty `Doc` and `Issue`, at least one `Admitted` and one `Bounds` per entry,
   and that every named fixture exists. It also asserts that every negative fixture is named by
   some entry. The corpus coverage test's `required` map is derived from the registry. Its 27
   current names stay a subset of the derived set, so the test fails if a fixture drops out.
8. **Keep every mutant.** Re-point `mutations.json` anchors whose code moved, keeping each
   mutation's meaning. The scan's mutation lines (on `bodycheck.go`, `structured.go` and
   `benignshapes.go`) may not fall below 34, and at least one targets the registry walk in
   `benignshapes.go` (for example, admit a run when no entry matches). Add `TestBenign` (planned) to the
   spec's top-level `test` selector, or the registry tests never run under a mutant. `TestSpecAnchorsLand` stays green.
9. Name the new tests in R-body-secret-scan's tests column in `docs/contracts.md`, and state in
   the corpus README that the registry names its fixtures. Change every consumer entry to
   `fixed-here` in the implementation PR.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +mutation | `cd tools/desk && go test ./internal/deskkit/ -run '^TestBenign' -count=1 -v > "${TMPDIR:-/tmp}/dt61-reg.log" 2>&1 && grep -F -e '--- PASS: TestBenignRegistryComplete ' "${TMPDIR:-/tmp}/dt61-reg.log" && grep -F -e '--- PASS: TestBenignEntriesBounded ' "${TMPDIR:-/tmp}/dt61-reg.log" && grep -F -e '--- PASS: TestBenignKeepsLiteralArms ' "${TMPDIR:-/tmp}/dt61-reg.log" && grep -F -e '--- PASS: TestBenignGoldenVerdicts ' "${TMPDIR:-/tmp}/dt61-reg.log" && grep -F -e '--- PASS: TestBenignRefusalText ' "${TMPDIR:-/tmp}/dt61-reg.log" && grep -F -e '--- PASS: TestBenignDocsParity ' "${TMPDIR:-/tmp}/dt61-reg.log"` | exit 0; six named PASS lines: every entry is complete, bounded by a refusal its widening flips, unable to suppress a literal arm, and the verdicts, message and docs agree with the registry |
| 2 | check:ci +flow | `test -n "${DELIVERY_BASE:-}" && d=$(mktemp -d) && trap 'rm -rf "$d"' 0 && git archive --format=tar "$DELIVERY_BASE" > "$d/base.tar" && mkdir "$d/b" && tar -x -C "$d/b" -f "$d/base.tar" && cp tools/desk/internal/deskkit/benigngolden_test.go "$d/b/tools/desk/internal/deskkit/" && cp tools/desk/internal/deskkit/testdata/benign-golden.txt "$d/b/tools/desk/internal/deskkit/testdata/" && cd "$d/b/tools/desk" && go test ./internal/deskkit/ -run '^TestBenignGoldenVerdicts$' -count=1 -v > "$d/golden.log" 2>&1 && grep -F -e '--- PASS: TestBenignGoldenVerdicts ' "$d/golden.log"` | exit 0; `ok`. The golden test from head passes on the pre-migration tree, so the golden is the old scanner's behaviour and row 1 proves the new scanner gives the same verdict on every input |
| 3 | check:ci +mutation | `O=$(mktemp) && trap 'rm -f "$O"' 0 && cd tools/desk && go run ./cmd/muhar -j 2 -spec internal/deskkit/mutations.json > "$O" 2>&1 && grep -F -e 'Harness healthy: baseline GREEN, positive control CAUGHT.' "$O" && grep -F -e ' 0 NOT CAUGHT, 0 could-not-mutate.' "$O" && n=$(grep -c -e '"file": "internal/deskkit/bodycheck.go"' -e '"file": "internal/deskkit/structured.go"' -e '"file": "internal/deskkit/benignshapes.go"' internal/deskkit/mutations.json) && [ "$n" -ge 34 ] && m=$(grep -c -e '"file": "internal/deskkit/benignshapes.go"' internal/deskkit/mutations.json) && [ "$m" -ge 1 ]` | exit 0; harness healthy, every mutation caught, the scan's mutation lines did not fall below 34, and at least one targets the registry walk |
| 4 | check:ci +neighbour | `cd tools/desk && go test ./internal/deskkit/ -count=1 -v > "${TMPDIR:-/tmp}/dt61-all.log" 2>&1 && grep -F -e '--- PASS: TestBodycheckCorpus ' "${TMPDIR:-/tmp}/dt61-all.log" && grep -F -e '--- PASS: TestBenignRegistryComplete ' "${TMPDIR:-/tmp}/dt61-all.log" && ! grep -F -e '--- FAIL' "${TMPDIR:-/tmp}/dt61-all.log"` | exit 0; every existing scan, corpus and explain test passes unedited beside the registry tests |
| 5 | check +flow | `test -n "${DELIVERY_BASE:-}" && test -n "${DELIVERY_HEAD:-}" && test -f tools/desk/internal/deskkit/benignshapes.go && git diff --quiet "$DELIVERY_BASE" "$DELIVERY_HEAD" -- tools/desk/internal/deskkit/testdata/corpus ':(exclude)tools/desk/internal/deskkit/testdata/corpus/README.md' && o=$(mktemp) && trap 'rm -f "$o"' 0 && git diff --name-only "$DELIVERY_BASE" "$DELIVERY_HEAD" -- tools/desk/internal/deskkit > "$o" && ! grep -v -x -e 'tools/desk/internal/deskkit/corpus_test.go' -e 'tools/desk/internal/deskkit/benignshapes_test.go' -e 'tools/desk/internal/deskkit/benigngolden_test.go' -e 'tools/desk/internal/deskkit/testdata/benign-golden.txt' -e 'tools/desk/internal/deskkit/benignshapes.go' -e 'tools/desk/internal/deskkit/bodycheck.go' -e 'tools/desk/internal/deskkit/structured.go' -e 'tools/desk/internal/deskkit/explain.go' -e 'tools/desk/internal/deskkit/mutations.json' -e 'tools/desk/internal/deskkit/testdata/corpus/README.md' "$o"` | exit 0; no corpus fixture changed (the corpus README may, for task 9), and no other deskkit test or source file changed |
| 6 | check:ci +neighbour | `cd tools/desk && test -f internal/deskkit/benignshapes.go && go test ./internal/arch/ -run '^TestOneImplementationPerMeaning$' -count=1 -v > "${TMPDIR:-/tmp}/dt61-arch.log" 2>&1 && grep -F -e '--- PASS: TestOneImplementationPerMeaning ' "${TMPDIR:-/tmp}/dt61-arch.log" && grep -F -x -e 'S-publication-scan 1' internal/arch/markers.txt && go test ./cmd/muhar -run '^TestSpecAnchorsLand$' -count=1 -v > "${TMPDIR:-/tmp}/dt61-anch.log" 2>&1 && grep -F -e '--- PASS: TestSpecAnchorsLand ' "${TMPDIR:-/tmp}/dt61-anch.log"` | exit 0; the scan still has one implementation with a ceiling of 1, and every mutation anchor lands |
| 7 | check +flow +dereference | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-61-benign-shape-registry.md && grep -F -e 'TestBenignEntriesBounded' docs/contracts.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/61 --delivery "$DELIVERY_HEAD" --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 3 corroborated, 0 disproved, 0 unchecked' "$o"` | exit 0; the README table, the contract row and the corpus README are in the delivering diff |

## Threat model and detection

| Failure mode | Caught by |
|---|---|
| The migration un-refuses a run the old chain refused (a predicate dropped, inverted or run over the wrong text) | rows 1 and 2 (golden verdicts on the old tree and the new one) |
| The golden is recorded from the migrated scanner, so the equivalence proof is circular | row 2 (the head's golden must pass on the delivery base) |
| An entry admits more than its issue asked for, with nothing to bound it | row 1 (`TestBenignEntriesBounded` (planned), per entry) |
| A registry entry suppresses a literal credential arm | row 1 (`TestBenignKeepsLiteralArms` (planned)) |
| A mutation is dropped or re-anchored to a no-op during the move | rows 3 and 6 |
| A positive fixture is edited so it passes | row 5 |
| A second admission path appears beside the registry | rows 5 and 6 |
| The refusal message or README drifts from what the scan admits | row 1 (`TestBenignRefusalText` (planned), `TestBenignDocsParity` (planned)) |
| An entry's `Doc` or `Issue` is accurate but misleading, so a reader trusts an admission they should not | No row; review-only: the security reviewer reads each entry's `Doc` against its predicate |

## Evidence
<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This authoring PR claims no future implementation test executed. -->

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 5 | `test -n "${DELIVERY_BASE:-}" && test -n "${DELIVERY_HEAD:-}" && test -f tools/desk/internal/deskkit/benignshapes.go && git diff --quiet "$DELIVERY_BASE" "$DELIVERY_HEAD" -- tools/desk/internal/deskkit/testdata/corpus ':(exclude)tools/desk/internal/deskkit/testdata/corpus/README.md' && o=$(mktemp) && trap 'rm -f "$o"' 0 && git diff --name-only "$DELIVERY_BASE" "$DELIVERY_HEAD" -- tools/desk/internal/deskkit > "$o" && ! grep -v -x -e 'tools/desk/internal/deskkit/corpus_test.go' -e 'tools/desk/internal/deskkit/benignshapes_test.go' -e 'tools/desk/internal/deskkit/benigngolden_test.go' -e 'tools/desk/internal/deskkit/testdata/benign-golden.txt' -e 'tools/desk/internal/deskkit/benignshapes.go' -e 'tools/desk/internal/deskkit/bodycheck.go' -e 'tools/desk/internal/deskkit/structured.go' -e 'tools/desk/internal/deskkit/explain.go' -e 'tools/desk/internal/deskkit/mutations.json' -e 'tools/desk/internal/deskkit/testdata/corpus/README.md' "$o"` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-27 | assay-verifier-app[bot] @ 75cb7f34d193 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) | base=96cdf390cda8 red rc=1 · head=75cb7f34d193 · delivering=382ea1c3e5f4 output=sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 log=row-000001.log |
| 7 | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-61-benign-shape-registry.md && grep -F -e 'TestBenignEntriesBounded' docs/contracts.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/61 --delivery "$DELIVERY_HEAD" --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 3 corroborated, 0 disproved, 0 unchecked' "$o"` | fail exit=1 | sha256:e3b0c44298fc | 2026-11-27 | assay-verifier-app[bot] @ 75cb7f34d193 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) | base=96cdf390cda8 red rc=1 · head=75cb7f34d193 · delivering=382ea1c3e5f4 output=sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 log=row-000002.log |
Verification-Attestation: medici-finance/assay#7843 run=364bed2aa198ed18f85c331b9b571ab112323727e9efa245b8705663b6c908c1 source=75cb7f34d193ddb4f295d67c948ae8ce86b7016b brief=docs/streams/desk-tools/brief-61-benign-shape-registry.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestBenign' -count=1 -v > "${TMPDIR:-/tmp}/dt61-reg.log" 2>&1 && grep -F -e '--- PASS: TestBenignRegistryComplete ' "${TMPDIR:-/tmp}/dt61-reg.log" && grep -F -e '--- PASS: TestBenignEntriesBounded ' "${TMPDIR:-/tmp}/dt61-reg.log" && grep -F -e '--- PASS: TestBenignKeepsLiteralArms ' "${TMPDIR:-/tmp}/dt61-reg.log" && grep -F -e '--- PASS: TestBenignGoldenVerdicts ' "${TMPDIR:-/tmp}/dt61-reg.log" && grep -F -e '--- PASS: TestBenignRefusalText ' "${TMPDIR:-/tmp}/dt61-reg.log" && grep -F -e '--- PASS: TestBenignDocsParity ' "${TMPDIR:-/tmp}/dt61-reg.log"` | pass exit=0 sandbox=container-netns | sha256:192fb451952f | 2026-11-27 | assay-verifier-app[bot] @ 75cb7f34d193 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=96cdf390cda8 red rc=1 · head=75cb7f34d193 · delivering=382ea1c3e5f4 |
| 2 | `test -n "${DELIVERY_BASE:-}" && d=$(mktemp -d) && trap 'rm -rf "$d"' 0 && git archive --format=tar "$DELIVERY_BASE" > "$d/base.tar" && mkdir "$d/b" && tar -x -C "$d/b" -f "$d/base.tar" && cp tools/desk/internal/deskkit/benigngolden_test.go "$d/b/tools/desk/internal/deskkit/" && cp tools/desk/internal/deskkit/testdata/benign-golden.txt "$d/b/tools/desk/internal/deskkit/testdata/" && cd "$d/b/tools/desk" && go test ./internal/deskkit/ -run '^TestBenignGoldenVerdicts$' -count=1 -v > "$d/golden.log" 2>&1 && grep -F -e '--- PASS: TestBenignGoldenVerdicts ' "$d/golden.log"` | fail exit=128 sandbox=container-netns | sha256:c4797869dae7 | 2026-11-27 | assay-verifier-app[bot] @ 75cb7f34d193 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=96cdf390cda8 red rc=1 · head=75cb7f34d193 · delivering=382ea1c3e5f4 |
| 3 | `O=$(mktemp) && trap 'rm -f "$O"' 0 && cd tools/desk && go run ./cmd/muhar -j 2 -spec internal/deskkit/mutations.json > "$O" 2>&1 && grep -F -e 'Harness healthy: baseline GREEN, positive control CAUGHT.' "$O" && grep -F -e ' 0 NOT CAUGHT, 0 could-not-mutate.' "$O" && n=$(grep -c -e '"file": "internal/deskkit/bodycheck.go"' -e '"file": "internal/deskkit/structured.go"' -e '"file": "internal/deskkit/benignshapes.go"' internal/deskkit/mutations.json) && [ "$n" -ge 34 ] && m=$(grep -c -e '"file": "internal/deskkit/benignshapes.go"' internal/deskkit/mutations.json) && [ "$m" -ge 1 ]` | pass exit=0 sandbox=container-netns | sha256:ca4f608802d9 | 2026-11-27 | assay-verifier-app[bot] @ 75cb7f34d193 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=96cdf390cda8 red rc=1 · head=75cb7f34d193 · delivering=382ea1c3e5f4 |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -count=1 -v > "${TMPDIR:-/tmp}/dt61-all.log" 2>&1 && grep -F -e '--- PASS: TestBodycheckCorpus ' "${TMPDIR:-/tmp}/dt61-all.log" && grep -F -e '--- PASS: TestBenignRegistryComplete ' "${TMPDIR:-/tmp}/dt61-all.log" && ! grep -F -e '--- FAIL' "${TMPDIR:-/tmp}/dt61-all.log"` | fail exit=1 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-27 | assay-verifier-app[bot] @ 75cb7f34d193 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=96cdf390cda8 red rc=1 · head=75cb7f34d193 · delivering=382ea1c3e5f4 |
| 6 | `cd tools/desk && test -f internal/deskkit/benignshapes.go && go test ./internal/arch/ -run '^TestOneImplementationPerMeaning$' -count=1 -v > "${TMPDIR:-/tmp}/dt61-arch.log" 2>&1 && grep -F -e '--- PASS: TestOneImplementationPerMeaning ' "${TMPDIR:-/tmp}/dt61-arch.log" && grep -F -x -e 'S-publication-scan 1' internal/arch/markers.txt && go test ./cmd/muhar -run '^TestSpecAnchorsLand$' -count=1 -v > "${TMPDIR:-/tmp}/dt61-anch.log" 2>&1 && grep -F -e '--- PASS: TestSpecAnchorsLand ' "${TMPDIR:-/tmp}/dt61-anch.log"` | pass exit=0 sandbox=container-netns | sha256:a2f70adff64d | 2026-11-27 | assay-verifier-app[bot] @ 75cb7f34d193 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=96cdf390cda8 red rc=1 · head=75cb7f34d193 · delivering=382ea1c3e5f4 |
Tree-Digest: sha256:a543710dbb04b56875593cca8b5a63271be5e17d50f23dd35932870eddb8992e
Container-Image: sha256:79e23ec185b1d5afe40cd92a25d9f7b18fd824f5678bf6c945443753ee6f509a
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#7843 run=364bed2aa198ed18f85c331b9b571ab112323727e9efa245b8705663b6c908c1 source=75cb7f34d193ddb4f295d67c948ae8ce86b7016b brief=docs/streams/desk-tools/brief-61-benign-shape-registry.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=364bed2aa198ed18f85c331b9b571ab112323727e9efa245b8705663b6c908c1 source=75cb7f34d193ddb4f295d67c948ae8ce86b7016b tree=sha256:a543710dbb04b56875593cca8b5a63271be5e17d50f23dd35932870eddb8992e witness=sha256:04d8e45128d9cd998d587b72cad5b982b89f701919eba73baf5bb873964ba589 opened=2026-11-27T15:59:32Z closed=2026-11-27T16:39:44Z before=receipt after=check-verifier

### Verifier notes (assay-verifier-app[bot], non-implementer, 2026-11-27)

- Source: merged main 75cb7f34d193ddb4f295d67c948ae8ce86b7016b, the admitted home, checked by deskdispatch --check-verifier (exit 0) before the rows ran. Attestation medici-finance/assay#7843, model claude-opus-5-5, tier strong. Desk tools v1.0.94, darwin arm64, offline, KUBECONFIG=/dev/null, declared verifier Git environment.
- Delivery: PR #6808, merge 382ea1c3e5f4f4f5970ece363e25aad7b093d115. Its first parent 96cdf390cda823c874e601c286520c72563c07a4 is the base, per the Review section ("the parent of the delivering merge"). The second parent is 51b6fb71daad66eebede7c6e022e56416ea94ddc and the merge-base of the parents is b03d8dace34d491cf14b46e9d9d96243a7a3b752. bodycheck.go and structured.go are unchanged between b03d8dac and 96cdf390. The rows ran with DELIVERY_HEAD=382ea1c3e5f4 and DELIVERY_BASE=96cdf390cda8. The commits from 382ea1c3 to 75cb7f34 do not touch tools/desk/internal/deskkit.
- Rows 5 and 7 (check) ran on the host with verifyrun --rows 5,7 --fail-first. That table is above. Rows 1, 2, 3, 4 and 6 (check:ci) ran in the closed container-netns admission bracket that follows it. That table is the bracketed attempt. Its credit read is could-not-check: --bracket-digest --credit needs the witness inside a git work tree.
- The bracket opened at 2026-11-27T15:59:32Z and closed at 2026-11-27T16:39:44Z, with tree sha256:a543710dbb04, image verify-runner v1.0.94 sha256:79e23ec185b1 (--network none, --read-only), module cache sha256:7f8fa9800956 mounted read-only, and history pack imported for the base. statusgen verifyrun --bracket-digest reports verdict=match, witness sha256:04d8e45128d9. Every row ran red at base 96cdf390cda8.
- Rows 1, 3 and 6 pass at head in the container and are red at base, so they discriminate. Row 3 is the mutation harness over all 188 entries of mutations.json. Row 5 passes at head on the host and is red at base.

### Rows 2 and 4: container environment, held

- **Row 2** exited 128 at head in the container. The row runs git archive on DELIVERY_BASE in the head tree. The home is a linked worktree, and its .git gitfile points at a Git directory that the bracket deliberately does not mount. In the same image with the same read-only home mount, git archive of 96cdf390 prints "fatal: gitfile does not point to a valid repository: /work/.git" with rc=128. This is the class medici-finance/assay#4327, an environment cause, not the deliverable. The row is held as could-not-run.
- **Row 4** exited 1 at head in the container. The full deskkit package ran with "--- FAIL" lines only in tests unrelated to this delivery: TestBeaconStoreConcurrentProcessesPreserveFields, five TestLayaAdvisor tests, TestSharedReadLowerScope and TestAttestCaseVariantRoot. None of their files is in the delivering diff. The same command passes on the host with 0 "--- FAIL" (below), so the cause is the container sandbox, not the deliverable. The likely mechanism is the read-only, Git-less mount (the medici-finance/assay#4327 class): the shared-read test reports no origin remote and the attest test reports a different worktree. The laya and beacon-store causes were not traced further. The row is held as could-not-run.

Supplemental diagnostic with no acceptance credit. Both were run on the host on 2026-11-27 by assay-verifier-app[bot], outside the home, in a scratch clone at 75cb7f34d193. The clone was made with git clone --shared from the local repository, with origin set to the forge URL. Each row command was unchanged, under the declared verifier Git environment, with GOPROXY=off, a throwaway HOME and TMPDIR, and no network sandbox:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 2-supp-host | see Verify row 2, with DELIVERY_BASE=96cdf390cda8 | diagnostic only, no acceptance credit | rc=0; "--- PASS: TestBenignGoldenVerdicts (0.39s)" | 2026-11-27 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4-supp-host | see Verify row 4 | diagnostic only, no acceptance credit | rc=0; 2235 top-level PASS, 0 "--- FAIL"; "ok github.com/medici-finance/assay/tools/desk/internal/deskkit 343.173s" | 2026-11-27 | assay-verifier-app[bot] (on-behalf-of human:ian) |

### Row 7: definition defect, held

Row 7 failed at head (exit 1) on its first guard, before any obligation it checks. The guard requires the HEAD of the home to equal DELIVERY_HEAD. The admitted home is merged main 75cb7f34, not the delivering merge 382ea1c3, and the verifier may not move the admitted home. After merge the row cannot pass in an admitted home, so it is held as could-not-run (check-definition), not judged. This is the same class recorded on desk-tools/48, desk-tools/50 and desk-tools/62.

Supplemental diagnostic with no acceptance credit. These are the row's later clauses, run by hand on the host on 2026-11-27 by assay-verifier-app[bot], in the home at 75cb7f34d193, with the same DELIVERY_HEAD and DELIVERY_BASE:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 7-supp | row 7 with the HEAD guard omitted (is-ancestor, follow-up grep, contracts grep, statusgen consumers) | diagnostic only, no acceptance credit | is-ancestor rc=0; follow-up marker grep rc=1 (none); docs/contracts.md names TestBenignEntriesBounded once; statusgen --consumers rc=0 with "summary: 3 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing" (all 3 consumer sites CORROBORATED) | 2026-11-27 | assay-verifier-app[bot] (on-behalf-of human:ian) |

### Risk values (sensitive-data: yes)

ENUMERATE. These are the literals that the diff 96cdf390..382ea1c3 introduces, moves or changes in the scanner and its tests, with source lines at 75cb7f34:

1. reBase64ish = [A-Za-z0-9+/=]{32,} @ tools/desk/internal/deskkit/bodycheck.go:23 (base :24)
2. runThreshold = 32 @ tools/desk/internal/deskkit/bodycheck.go:149 (base :150)
3. digestKeys = {SHA, SHA1, SHA256, DIGEST, CHECKSUM, COMMIT} @ tools/desk/internal/deskkit/benignshapes.go:244 (base bodycheck.go:247)
4. credentialStems = {SECRET, TOKEN, PASS, PWD, KEY, AUTH, CRED, PRIV, SESSION, COOKIE, SIGN, HMAC, SALT, API, CERT, BEARER, JWT, SEED} @ tools/desk/internal/deskkit/benignshapes.go:250 (base bodycheck.go:253)
5. twoLetterWords = 27 entries (Am ... We, Id, Ok) @ tools/desk/internal/deskkit/benignshapes.go:283 (base bodycheck.go:286)
6. numeronyms = {K8s, I18n, L10n, A11y} @ tools/desk/internal/deskkit/benignshapes.go:226 (base bodycheck.go:229)
7. mixedCaseWords = {OAuth} @ tools/desk/internal/deskkit/benignshapes.go:236 (base bodycheck.go:239)
8. k8sUIDPrefixes = {pvc, pv, pod, rs, rc, ds, sts, job, cm, svc} @ tools/desk/internal/deskkit/benignshapes.go:304 (base bodycheck.go:1250)
9. godebugHeads = {DefaultGODEBUG, GODEBUG} @ tools/desk/internal/deskkit/benignshapes.go:312 (base bodycheck.go:1009)
10. goTestPrefixes = {Test, Benchmark, Fuzz, Example} @ tools/desk/internal/deskkit/benignshapes.go:293 (base bodycheck.go:902)
11. Doc text "at most 16 chars" @ tools/desk/internal/deskkit/benignshapes.go:131 and "up to 16 letters" @ tools/desk/internal/deskkit/benignshapes.go:155. These are refusal-message prose carried over from the base refusal text, not the predicates.
12. goldenRuns = 2000 @ tools/desk/internal/deskkit/benigngolden_test.go:47; goldenSeed = 61 @ :48; run length 30+r.Intn(61) (30 to 90) @ :82; floor refused >= 1000 and clean >= 100 @ :205; mutations.json 188 entries
13. The DR-scan-admission design record: the authority binding for the design.

RANK. Entries 1 and 2 rank first. A wrong class or threshold lets a credential reach a public forge body, and a publish cannot be undone by an edit. Entries 3 and 4 come next: they decide whether a 64-hex value behind an env key is treated as a digest or as a secret. Entries 5 to 10 widen admission of word-shaped runs. Entries 11 and 12 are prose and test-sample bounds. They can be reverted by an edit and need no derivation.

DERIVE. This brief is a behaviour-preserving move into a registry, and its Review section requires "the literal arms ... byte-identical to the base". I checked this mechanically. The sorted set of all 20 regexp literals in bodycheck.go and structured.go at 96cdf390 equals the set across bodycheck.go, structured.go and benignshapes.go at 75cb7f34. For each closed list in entries 3 to 10, the sha256 of its var block at base (bodycheck.go) equals the sha256 of its var block at source (benignshapes.go).

- RISK-VALUE: DERIVED — reBase64ish = [A-Za-z0-9+/=]{32,} @ tools/desk/internal/deskkit/bodycheck.go:23 — The brief's own constraint is that the literal arms are byte-identical to the base. The regexp literal set is identical at base and source, so this delivery changes nothing about which runs are scanned. The adequacy of the value predates this item and was not re-derived here.
- RISK-VALUE: DERIVED — runThreshold = 32 @ tools/desk/internal/deskkit/bodycheck.go:149 — It mirrors reBase64ish {32,}, as its comment at :147 says, and is identical at base :150. The registry reads it, and it is not redefined.
- RISK-VALUE: DERIVED — digestKeys = {SHA, SHA1, SHA256, DIGEST, CHECKSUM, COMMIT} @ tools/desk/internal/deskkit/benignshapes.go:244 — The var block is byte-identical to base bodycheck.go:247, so it was moved, not widened. Its comment gives the derivation: a closed tail list, because a shape rule would admit TOKEN=<64 hex>.
- RISK-VALUE: DERIVED — credentialStems = 18 stems @ tools/desk/internal/deskkit/benignshapes.go:250 — The var block is byte-identical to base bodycheck.go:253. It is a refusing fence, so moving it cannot widen admission.
- RISK-VALUE: DERIVED — twoLetterWords = 27 entries @ tools/desk/internal/deskkit/benignshapes.go:283 — The var block is byte-identical to base bodycheck.go:286. The cost measurement in its comment is pinned by TestTwoLetterWordsCostAlmostNothing.
- RISK-VALUE: DERIVED — numeronyms, mixedCaseWords, k8sUIDPrefixes, godebugHeads, goTestPrefixes @ tools/desk/internal/deskkit/benignshapes.go:226, :236, :304, :312, :293 — Each var block is byte-identical to its base definition in bodycheck.go (:229, :239, :1250, :1009, :902).
- RISK-VALUE: NAMED, NOT DERIVED — goldenRuns = 2000, goldenSeed = 61, refused >= 1000, clean >= 100 @ tools/desk/internal/deskkit/benigngolden_test.go:47, :48, :205 — These are sample-size bounds for the equivalence golden. No statistical derivation of 2000 runs as sufficient coverage is stated, and I did not compute one. They are test-only and reversible.

### Verdict

**VERIFY: BLOCKED** — 4 of 7 rows met (1, 3 and 6 in the container bracket, 5 on the host), and every one is red at base. 0 failed. 3 rows are held as could-not-run. Rows 2 and 4 hit the container environment (medici-finance/assay#4327); their host diagnostics pass with no credit. Row 7 has a definition defect: its HEAD = DELIVERY_HEAD guard cannot hold in a post-merge admitted home. Status stays implemented. This is a gate:human item, and the verifier flips nothing.


## Review
Gate: human. Before it leaves `todo`, the design needs an approved `DR-<slug>` record cited by
`design:` (brief-rules 48). That record is DR-scan-admission. The implementation PR needs a `Security-Review: pass` at its head
before any ready flip. The reviewer checks that rows 1 and 2 were run against the delivery
base, not only at head. They check that no `Bounds` example is a weakened copy of a positive
fixture, and that the literal arms are byte-identical to the base. Every Verify row is
risk-bearing (the brief answers `sensitive-data: yes`), so each one is written to fail on the
delivery base, and the verifier records it with `verifyrun --fail-first` against the parent of
the delivering merge.
