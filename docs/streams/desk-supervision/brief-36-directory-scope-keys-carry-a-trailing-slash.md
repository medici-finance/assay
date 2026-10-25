---
brief: assay:assay:desk-supervision:36
title: Directory scope keys in outcome records carry a trailing slash; readers accept both forms
why: >-
  An outcome record that declares a directory such as tools/desk/cmd/desktoken writes the key
  `scope:tools/desk/cmd/desktoken` beside a 64-hex digest, and the secret pattern sweep reads
  that as a token assignment, so honest Evidence PRs go red on four false positives. The driver
  ratified fixing the record's key shape rather than widening the scanner's allowlist: directory
  keys end in `/`, which the generic rule cannot match. Old records stay byte-identical and must
  keep reading, or every historical receipt would fall to could-not-check.
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [3807]
schema: brief-v2
version: 1
authored: 2026-10-21 by the-desk (coordinator session)
exec-tier: strong
exec-tier-why: >-
  (b) one key form is shared by the producer, the completeness check, the offline wake reader,
  the Source/Evidence admission binding and two contract docs; (c) a reader that silently
  returns an empty digest for a legacy key passes naive tests and wakes every held receipt.
domain: complicated
outcome: none
sources:
  - https://github.com/medici-finance/assay/issues/3807#issuecomment-6306541776
  - https://github.com/medici-finance/assay/issues/3807#issuecomment-6305484072
  - https://github.com/medici-finance/assay/pull/3816
  - https://github.com/medici-finance/assay/commit/c5faab976
  - docs/declared-input-scope.md
  - docs/streams/desk-supervision/verify-wake-v1.md
  - "fail-first probe 2026-10-21, gitleaks 8.30.1 + this repo's .gitleaks.toml @ 8121208a6: record keyed `scope:tools/desk/cmd/desktoken` exits 9; same record keyed `scope:tools/desk/cmd/desktoken/` exits 0"
  - "corpus count 2026-10-21 @ 8121208a6: 397 records under docs/streams/verify-outcomes/, 12 carry scope keys, 9 distinct literal-directory scope keys (11 occurrences), 0 ending in a credential keyword; 20 extensionless file:/scope: keys, 0 ending in a credential keyword"
  - "freshness-checked 2026-10-21 @ 8121208a6 (main): producer, completeness check and wake reader unchanged since 73530dcbf; no open PR touches declaredscope.go, verifywake.go, verifyoutcomes.go, outcomebinding.go, declared-input-scope.md or verify-wake-v1.md"
consumers:
  - "tools/desk/internal/deskkit/declaredscope.go (ResolveDeclaredScope key emission; CheckReceiptScope lookup): follow-up desk-supervision/36"
  - "tools/desk/internal/deskkit/verifywake.go (RootRevisionReader scope: case): follow-up desk-supervision/36"
  - "tools/desk/internal/deskkit/verifyoutcomes.go (BuildOutcomeRecord, inputs := scope.Shapes): out-of-scope (inherits the new keys from ResolveDeclaredScope unchanged)"
  - "tools/desk/cmd/deskevidence/outcomebinding.go (maps.Equal of Source and Evidence Shapes; maps.Equal of outcome.Inputs against the re-derived binding near line 128): out-of-scope (both sides use the same resolver, and a file/directory kind change between them stays refused; a record produced by a pre-change tool and bound by a post-change tool re-derives slash keys, so the inputs check refuses it, which fails closed and is remedied by re-producing the record with the current tool)"
  - "tools/desk/cmd/deskevidence/receiptvalidate.go (file: coverage only): out-of-scope (reads no scope: key)"
  - "tools/desk/cmd/verifyloop/testdata/wake-fixture-repo/ (scope keys are file literals only): out-of-scope (unchanged keys)"
  - "docs/declared-input-scope.md (Observation and reader contract): follow-up desk-supervision/36"
  - "docs/streams/desk-supervision/verify-wake-v1.md (input-key grammar and older-reader note): follow-up desk-supervision/36"
  - "docs/streams/verify-outcomes/**/*.json (landed records): out-of-scope (immutable; never rewritten)"
  - ".gitleaks.toml: out-of-scope (the ruling keeps it unchanged)"
---

# Brief 36 — Directory scope keys in outcome records carry a trailing slash; readers accept both forms

## Context

files:
- `tools/desk/internal/deskkit/declaredscope.go`
- `tools/desk/internal/deskkit/verifywake.go`
- `tools/desk/internal/deskkit/declaredscope_test.go`
- `tools/desk/internal/deskkit/scopekey_test.go` (planned)
- `tools/desk/internal/deskkit/scopekey-mutations.json` (planned)
- `docs/declared-input-scope.md`
- `docs/streams/desk-supervision/verify-wake-v1.md`
- `changelog/scope-key-directory-slash.md` (planned)

facts:
- ruling (2026-10-21, driver's own comment, sources line 1): option A — the outcome-record producer
  writes literal-directory `scope:` keys with a trailing `/`; `.gitleaks.toml` stays unchanged.
- producer @ 8121208a6: `declaredscope.go:18` `const InputKeyScopePrefix = "scope:"`;
  `declaredscope.go:519` `result.Shapes[InputKeyScopePrefix+decl] = Sha256Hex(...)`, where `decl`
  is the canonical declaration. Canonicalisation strips a trailing slash
  (`docs/declared-input-scope.md` "A trailing slash is normalized") and `ResolveDeclaredScope`
  refuses a non-canonical declaration (`canonical[0] != decl`, about line 438). So the slash is a
  property of the KEY, decided by the observed path kind, never of the declaration.
- kind source @ 8121208a6: inside `ResolveDeclaredScope`, `tree.PathKind(p)` returns `""` (absent),
  `"blob"` or `"tree"`. A literal declaration (no `*?[`) whose kind is `"tree"` gets the slash key.
  Literal files, absent literals and globs keep today's key exactly.
- readers @ 8121208a6: `CheckReceiptScope` (`declaredscope.go:573`) requires
  `receipt.Inputs[InputKeyScopePrefix+decl] != ""`; the offline wake reader (`verifywake.go:521-527`)
  strips the prefix, resolves `[]string{decl}` and returns `scope.Shapes[input]`. With a slash key
  `decl` would be non-canonical and refused; with a legacy directory key the lookup would miss and
  return `""`. Each is a silent failure: could-not-check, or a spurious wake.
- gitleaks: the pattern sweep (`.github/workflows/leaksweep-pattern.yml`, `GITLEAKS_VERSION: "v8.30.1"`)
  runs rule `generic-api-key` over the whole tree. The record allowlist (`.gitleaks.toml`, block
  "#1804 A′") only admits keys ending in a file extension (`\.[a-z]{1,5}":"[0-9a-f]{64}`), so an
  extensionless key ending in a credential keyword trips. A `/` after the keyword breaks the rule's
  keyword-then-operator match, so the slash form passes without any allowlist.
- compatibility: a new record read by a reader older than this change hits the non-canonical
  refusal or the missing key, so it reports could-not-check, never unchanged. Fail-closed, and the
  same release carries both producer and reader. `verify-wake-v1.md` already says older readers
  report could-not-check on unfamiliar `scope:` keys.
- residual, measured not widened: keys that keep the no-slash form still trip the sweep when they
  end in a credential keyword — an extensionless FILE literal (`scope:x/token` naming a file), an
  ABSENT literal, and a glob whose last segment ends in a credential keyword. Count on main @ 8121208a6 is 0; record
  it in the contract doc, do not fix it here.

single-point-of-failure: the producer's key form is the only thing keeping directory keys clear of
`generic-api-key`. Behind it, the pattern sweep still scans every record and fails closed: a
regression goes red, never silently green. On the read side, the completeness check and the wake
reader are separate functions that fail on different inputs (key presence vs digest
recomputation), and each is tested with the other bypassed.

## Read first

- `docs/declared-input-scope.md` §"Supported path forms" and §"Observation and reader contract".
- `docs/streams/desk-supervision/verify-wake-v1.md` lines 45-85 (inputs grammar) and the
  older-reader note near line 217.
- `tools/desk/internal/deskkit/declaredscope.go` `ResolveDeclaredScope` and `CheckReceiptScope`;
  `tools/desk/internal/deskkit/verifywake.go` `RootRevisionReader` (the `scope:` case).

## Ground rules

- NEVER git push to main, trigger workflows or run mutating infrastructure commands. Branch + draft PR only.
- Stop at `implemented`; a different identity verifies.
- Never rewrite a landed record under `docs/streams/verify-outcomes/`, and never commit a file
  that holds a legacy directory key beside a 64-hex value: the pattern sweep scans the whole tree.
  Negative-control fixtures are generated under a temp dir at test time.
- `.gitleaks.toml` and the leak-sweep workflows are untouched. If greening anything seems to
  need them changed, stop: NEEDS_CONTEXT.
- If anything contradicts repo state, report NEEDS_CONTEXT; don't guess.

## Task

1. **One key helper.** Add `ScopeInputKey(decl, kind string) string` (slash form exactly when `decl`
   is a literal and `kind == "tree"`) and `ParseScopeInputKey(key string) (decl string, dirForm bool, ok bool)`.
   `ok` is false for an empty declaration, a double slash, a non-`scope:` key, or a slash form whose
   declaration is a glob (`scope:<glob>/` is never emitted, so it is refused, never normalised). The parser strips
   at most one trailing `/` and returns the canonical declaration. Use the helper at
   `declaredscope.go:519`. The digest value and its shape terms are unchanged byte for byte.
2. **Completeness check accepts both forms.** `CheckReceiptScope` treats a declaration as covered
   when exactly one of `scope:<decl>` or `scope:<decl>/` is present and non-empty. The `scope:<decl>/`
   alternative exists only for a literal declaration; for a glob, only `scope:<decl>` counts. If both are
   present, refuse with an "ambiguous scope key" error, which reads as could-not-check. Also refuse
   (could-not-check) the value of every `scope:` key, in either form, that is not exactly the digest
   format the producer writes at `declaredscope.go:519`. Once a directory key ends in `/`, the pattern
   sweep's generic rule no longer inspects that value position. This check only REFUSES TO READ such a
   record; it runs at read time and cannot stop a non-digest value from being published. A lint over
   landed records would be a separate, later layer and is not part of this brief.
3. **Wake reader keys on the declaration, not the key string.** In `verifywake.go`'s `scope:` case,
   parse the key with `ParseScopeInputKey`, resolve the canonical declaration, and return the one
   digest that resolution produced, whichever key form it emitted. Then:
   - an unchanged directory read through a legacy key returns the original digest (unchanged);
   - a member added or removed returns a different digest (wake);
   - a path that changed kind returns a different digest (wake);
   - an unparseable key returns `("", false)` (could-not-check).
4. **Contract docs.** In `declared-input-scope.md`, state the two key forms and when each is
   emitted, that readers accept both, that "ambiguous" means both forms present, and the measured
   residual: at a slash-keyed position the generic pattern rule matches no value, dedicated provider
   rules still fire, and readers refuse to read any non-digest value (after publication, not before); and the no-slash keys that can still
   trip the sweep (extensionless file literals, absent literals, globs ending in a credential keyword). Add one sentence after the "A trailing slash is normalized" line saying that
   normalisation applies to declarations and that keys re-add the slash for directories. In
   `verify-wake-v1.md`, extend the input-key grammar to `scope:<canonical-declaration>[/]` and keep
   the older-reader note true: older readers report could-not-check on slash keys.
5. **Tests** in `scopekey_test.go` (planned) with hermetic tree fixtures: `TestScopeKeyDirectoryForm`,
   `TestScopeKeyLegacyReceiptReads`, `TestScopeKeyAmbiguousForms`, `TestScopeKeyLayersIndependent`,
   `TestScopeKeyValueMustBeDigest`, `TestScopeKeyLeakSweepFixture`. `TestScopeKeyValueMustBeDigest`
   asserts a 40-character base62 value under a slash-keyed directory key is refused, and a well-formed
   digest is accepted. No test source commits a token-shaped literal: the base62 value is generated
   at test time, and the golden digest of row 1 is pinned in a form the pinned gitleaks config does not
   flag: one constant whose identifier carries no credential keyword (e.g. `goldenDirDigest`; never split
   a value across constants, which is the shape of a real evasion), checked by
   running the pre-push sweep over the changed files. `TestScopeKeyLeakSweepFixture` builds a real record through `BuildOutcomeRecord` over
   a fixture tree that declares the directory `tools/desk/cmd/desktoken`. When
   `SCOPE_KEY_FIXTURE_DIR` is set, it writes that record to
   `$SCOPE_KEY_FIXTURE_DIR/new/docs/streams/verify-outcomes/scope-fixture/01-20261021T000000Z-000000000000.json`,
   and the same record with the directory key's slash removed to the same path under `legacy/`.
   It also writes, under `planted/` at the same path, the new-form record with the value of the slash-form
   directory key `scope:tools/desk/cmd/desktoken/` replaced by a value that a dedicated provider rule of the pinned gitleaks config matches. That value
   is generated at test time and never committed.
   When the variable is unset it still asserts the producer emitted `scope:tools/desk/cmd/desktoken/`.
   Add a mutation spec, `scopekey-mutations.json` (planned), beside the code in the house format
   (`"test"`, a `"control"` positive control, and `"mutations"`, each `{name,file,old,new}`), whose
   `"test"` runs the rows 3 and 10 tests. It carries exactly these three mutants of `declaredscope.go` (row 11 pins the count): the
   both-forms refusal removed, the digest-format check removed, and the slash-form glob refusal removed.
   Test code calls no `Setenv` or `os.Exit`, defines no `TestMain` or `init`, and reads or writes none of the
   leak sweeps' package-level state: row 6 refuses any such line, and row 12 proves the control sweep still bites.
6. **Changelog.** `changelog/scope-key-directory-slash.md` (planned) under `### Fixed`: outcome records key
   declared directories with a trailing slash so the secret sweep stops flagging them, and old
   records still read.

## Verify (executable — no prose-only DoD items)

Run from the repository root; Go rows run inside the `tools/desk` module.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `(cd tools/desk && o=$(mktemp) && go test ./internal/deskkit -run '^TestScopeKeyDirectoryForm$' -count=1 -timeout 120s -v > "$o" 2>&1; grep -F -- '--- PASS: TestScopeKeyDirectoryForm' "$o")` | exit 0. A literal directory gets `scope:<decl>/`; a literal file, an absent literal and a glob keep `scope:<decl>`; the digest equals a golden value pinned from the base for the same fixture tree | check |
| 2 | `(cd tools/desk && o=$(mktemp) && go test ./internal/deskkit -run '^TestScopeKeyLegacyReceiptReads$' -count=1 -timeout 120s -v > "$o" 2>&1; grep -F -- '--- PASS: TestScopeKeyLegacyReceiptReads' "$o")` | exit 0. A receipt with a legacy directory key passes `CheckReceiptScope`, reads unchanged on an unchanged tree, and wakes when a member is added | check |
| 3 | `(cd tools/desk && o=$(mktemp) && go test ./internal/deskkit -run '^TestScopeKeyAmbiguousForms$' -count=1 -timeout 120s -v > "$o" 2>&1; grep -F -- '--- PASS: TestScopeKeyAmbiguousForms' "$o")` | exit 0. Both forms present is refused; `scope:x//`, `scope:/` and a slash-form glob key (`scope:<glob>/`) read could-not-check; a slash key whose path is now a file wakes | check +mutation |
| 4 | `(cd tools/desk && o=$(mktemp) && go test ./internal/deskkit -run '^TestScopeKeyLayersIndependent$' -count=1 -timeout 120s -v > "$o" 2>&1; grep -F -- '--- PASS: TestScopeKeyLayersIndependent' "$o")` | exit 0. With `CheckReceiptScope` bypassed, the wake reader alone still returns the original digest for a legacy key, and an empty digest never compares equal. With the wake reader stubbed, the completeness check alone still refuses a receipt that lacks the declaration in both forms | check |
| 5 | `d=$(mktemp -d) && (cd tools/desk && SCOPE_KEY_FIXTURE_DIR="$d" go test ./internal/deskkit -run '^TestScopeKeyLeakSweepFixture$' -count=1 -timeout 120s) && p=docs/streams/verify-outcomes/scope-fixture/01-20261021T000000Z-000000000000.json && test -f "$d/new/$p" && test -f "$d/legacy/$p" && test -f "$d/planted/$p" && gitleaks version \| grep -Fx 8.30.1 && cp .gitleaks.toml "$d/new/" && cp .gitleaks.toml "$d/legacy/" && cp .gitleaks.toml "$d/planted/" && { (cd "$d/new" && gitleaks dir . --config .gitleaks.toml --exit-code 9 --no-banner --redact >/dev/null 2>&1); a=$?; (cd "$d/legacy" && gitleaks dir . --config .gitleaks.toml --exit-code 9 --no-banner --redact >/dev/null 2>&1); b=$?; (cd "$d/planted" && gitleaks dir . --config .gitleaks.toml --exit-code 9 --no-banner --redact --report-format json --report-path "$d/planted.json" >/dev/null 2>&1); c=$?; echo "new=$a legacy=$b planted=$c"; test "$a" = 0 && test "$b" = 9 && test "$c" = 9 && jq -e 'length > 0 and all(.[]; .RuleID != "generic-api-key")' "$d/planted.json" >/dev/null && echo planted-rule-dedicated; }` | exit 0 and prints `new=0 legacy=9 planted=9` then `planted-rule-dedicated`. The producer-built record is clean under pinned gitleaks with the unchanged repo config; its legacy twin is red (fail-first control); a dedicated-rule value as the value of the slash-form key `scope:tools/desk/cmd/desktoken/` is still caught, by a rule other than `generic-api-key` (pins the residual) | check |
| 6 | `a='select(.state != "CLOSED")' && v='select((.author.login \| sub("^app/";"") \| sub("\\[bot\\]$";"")) != "assay-verifier-app")' && b='select(.body \| test("(^\|\\n)Brief: desk-supervision/36\\r?(\\n\|$)"))' && f=".[] \| $a \| $v \| $b \| .number" && m=".[] \| $a \| $b \| .number" && k='[{"number":1,"state":"OPEN","author":{"login":"app/assay-worker-app"},"body":"x\nBrief: desk-supervision/36\n"},{"number":2,"state":"MERGED","author":{"login":"assay-verifier-app[bot]"},"body":"Brief: desk-supervision/36\n"},{"number":3,"state":"MERGED","author":{"login":"app/assay-verifier-app"},"body":"x\nBrief: desk-supervision/36\r\n"},{"number":4,"state":"CLOSED","author":{"login":"app/assay-worker-app"},"body":"Brief: desk-supervision/36\n"}]' && test "$(jq -nr "$k \| $f" \| paste -sd, -)" = 1 && test "$(jq -nr "$k \| $m" \| paste -sd, -)" = 1,2,3 && echo control-ok && p="$(gh pr list -R medici-finance/assay --state all --limit 100 --search '"Brief: desk-supervision/36" in:body' --json number,state,body,author --jq "$f")" && test -n "$p" && printf '%s\n' "$p" \| while read -r n; do h="$(gh api "repos/medici-finance/assay/pulls/$n" --jq .head.sha)" && s="$(gh api "repos/medici-finance/assay/pulls/$n" --jq .base.sha)" && git fetch -q --no-tags origin "$h" "$s" && m="$(git merge-base "$s" "$h")" && test -z "$(git diff --no-renames --name-only "$m" "$h" \| grep -vxF -e tools/desk/internal/deskkit/declaredscope.go -e tools/desk/internal/deskkit/verifywake.go -e tools/desk/internal/deskkit/declaredscope_test.go -e tools/desk/internal/deskkit/scopekey_test.go -e tools/desk/internal/deskkit/scopekey-mutations.json -e docs/declared-input-scope.md -e docs/streams/desk-supervision/verify-wake-v1.md -e changelog/scope-key-directory-slash.md -e changelog/desk-supervision-36-scope-key-authoring.md -e docs/streams/desk-supervision/README.md -e docs/streams/desk-supervision/brief-36-directory-scope-keys-carry-a-trailing-slash.md)" && test -z "$(git diff --text --no-color --no-renames "$m" "$h" -- tools/ \| grep -e '^[-+].*EnvSweep' -e '^[-+].*Setenv' -e '^[-+].*os[.]Exit' -e '^[-+].*func TestMain' -e '^[-+].*func init[(]' -e '^[-+].*s2SweepExclusions' -e '^[-+].*corpusCopySetExempt' -e '^[-+].*corpusOperationalStreams' -e '^[-+].*leakShapeRules' -e '^[-+].*AllLeakShapes' -e '^[-+].*forgeSSHHosts')" && test -z "$(git diff --no-renames --name-only --text -G 'gitleaks[:]allow' "$m" "$h")" && echo "#$n untouched" \|\| { echo "#$n TOUCHED"; exit 1; }; done && echo untouched` | exit 0; prints `control-ok`, then one `#<N> untouched` line per delivery PR, then `untouched`. Scoped to the delivery, never to a pinned base, so later changes on main cannot turn it red: it finds every open or merged PR whose body carries the exact line `Brief: desk-supervision/36` and whose author is not the verifier App (none found is a failure, not a pass, so run it after the implementation PR exists, in a checkout whose `origin` is this repo). The verifier's Evidence PRs carry the same trailer but are not deliveries: they add Evidence and outcome records by design, so selecting them would turn the row red for the wrong reason. The selector is proven before the live search: `control-ok` means that on four fixed PR records (a worker-authored open one, a verifier-authored merged one in each login spelling, a closed one) the selector returns only the worker-authored open PR, and the same selector with the verifier-author clause removed (the mutant) returns the verifier-authored ones too, so the control is sensitive to that clause. For each selected PR it diffs the merge-base of its base and head against its head. Each delivery PR must (a) change only the paths named in this brief's `files:` plus this brief, the stream README and the authoring changelog fragment, so no sweep config, ignore file, `.gitattributes` at any depth, workflow, outcome record, other test file or other package can change at all; (b) add or remove no line under `tools/` naming a sweep environment knob, `Setenv`, `os.Exit`, `TestMain`, an `init` function or the sweep's package-level state (`--text --no-color`, so an attribute cannot hide a line); and (c) add or remove no inline gitleaks allow marker anywhere (the regex brackets the colon so this row's own text never matches). A red is fixed in the implementation, never passed by hand | check +mutation |
| 7 | `(cd tools/desk && go test ./internal/deskkit/... ./cmd/deskevidence/... ./cmd/verifyloop/... -count=1 -timeout 300s)` | exit 0. Admission binding, receipt validation, the producer and the wake fixtures stay green (the flow across all consumers) | check:ci |
| 8 | `statusgen --root . --lint` | exit 0; LINT PASS | check:ci |
| 9 | `statusgen --root . --consumers --brief desk-supervision/36` | exit 0; the four follow-up rows are corroborated by the implementation diff | check:ci |
| 10 | `(cd tools/desk && o=$(mktemp) && go test ./internal/deskkit -run '^TestScopeKeyValueMustBeDigest$' -count=1 -timeout 120s -v > "$o" 2>&1; grep -F -- '--- PASS: TestScopeKeyValueMustBeDigest' "$o")` | exit 0. A 40-character base62 value under a slash-keyed directory key is refused as could-not-check; a well-formed digest is accepted | check +mutation |
| 11 | `T=$(mktemp -d) && (cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/scopekey-mutations.json > "$T/out" 2>&1); rc=$?; L=$(tail -1 "$T/out"); echo "$L"; [ $rc -eq 0 ] && [ "$L" = 'Totals: 3 caught, 0 NOT CAUGHT, 0 could-not-mutate.' ]` | exit 0 and prints exactly `Totals: 3 caught, 0 NOT CAUGHT, 0 could-not-mutate.` (the count is pinned; adding a mutant updates this literal in the same change). Baseline GREEN, positive control CAUGHT, and every mutation CAUGHT: the ambiguous-form refusal, the digest-format check and the slash-form glob refusal each have a test that fails without them | check +mutation |
| 12 | `d=$(mktemp -d) && git archive HEAD \| tar -x -C "$d" && printf 'probe %s-%s\n' example withheld-channel > "$d/docs/zz-sweep-plant.md" && { (cd "$d/tools/desk" && go test ./internal/deskkit/ -run TestS2 -count=1 -v -timeout 300s > "$d/planted.log" 2>&1); p=$?; (cd tools/desk && go test ./internal/deskkit/ -run TestS2 -count=1 -v -timeout 300s > "$d/clean.log" 2>&1 && go test ./internal/deskkit/ -run TestCorpus -count=1 -v -timeout 300s >> "$d/clean.log" 2>&1); c=$?; echo "planted=$p clean=$c"; test "$p" != 0 && grep -qF -- '--- FAIL: TestS2DisclosureSweepIsClean' "$d/planted.log" && test "$c" = 0 && grep -qF -- '--- PASS: TestS2DisclosureSweepIsClean' "$d/clean.log" && grep -qF -- '--- PASS: TestS2ShapeRepoGate' "$d/clean.log" && grep -qF -- '--- PASS: TestCorpusHasNoWithheldStreamPaths' "$d/clean.log" && echo sweep-live; }` | exit 0; prints `planted=1 clean=0` then `sweep-live`. Behavioural: the control sweep's own S2 tests, run over a copy of this tree with the S2 test-only token (assembled from two halves, so this row never carries it) planted in a scratch `docs/` file, must FAIL; the same S2 tests over the tree as-is, followed by the corpus tests as a second single-pattern run (one token per `-run`, never an alternation), must pass and print PASS lines, so a neutered sweep and a test binary that exits before any test runs are both red. Catches what a path or keyword check cannot see | check |

## Pre-mortem

| Failure mode | Caught by |
|---|---|
| Slash is added to the declaration, so the resolver refuses it as non-canonical | 1, 3 |
| Producer changes the digest value as well as the key, so every legacy receipt wakes | 1 (pinned golden digest), 2 |
| Wake reader still indexes `Shapes[input]`, so legacy directory keys return `""` and wake forever | 2, 4 |
| Completeness check only knows the new form, so historical receipts go could-not-check | 2 |
| Both forms in one receipt accepted silently | 3, 11 (the refusal is load-bearing) |
| Fix proven only on a hand-written key, not on what the producer actually emits | 5 (record built through BuildOutcomeRecord) |
| Fixture is green because gitleaks never ran or ran a different version | 5 (file existence, version pin, legacy twin must be red) |
| Allowlist, ignore-fingerprint file, inline allow marker, `.gitattributes`, control-sweep exclusion, sweep environment knob, early exit or test file, or leak-sweep workflow or job quietly widened or added to get green, by any commit of the delivery | 6 (every delivery PR changes only the declared paths, and adds no knob, exit, initializer or sweep-state line); 12 (the control sweep still fails on a planted token and still runs its tests) |
| Mutant quietly dropped from the spec, so row 11 stays green on fewer mutants | 11 (Totals line pinned to a literal count) |
| Slash key hides a token from the generic pattern rule | 5 (`planted=9`, dedicated rule id: dedicated rules still fire before publication); 10 (readers refuse to read a non-digest value after publication; not a pre-publication control) |
| Glob or file keys change shape by accident | 1 |
| Contract docs describe the wrong grammar | review-only: reviewer reads the two doc edits against rows 1-3 |

## Evidence

<!-- appended at implementation time -->

## Review

Gate: model. All four risks are no. The change is reversible, touches source and docs only,
leaves the secret-scanner configuration untouched (row 6), and keeps landed records
byte-identical. The reviewer answers two questions. First, does any reader path treat a missing
or empty digest as unchanged? Row 4 must prove that it does not. Second, does any committed
fixture contain a legacy directory key beside a 64-hex value?
