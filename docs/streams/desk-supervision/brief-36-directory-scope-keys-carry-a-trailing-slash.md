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
version: 2
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
| 6 | `a='select(.state != "CLOSED")' && v='select((.author.login \| sub("^app/";"") \| sub("\\[bot\\]$";"")) != "assay-verifier-app")' && b='select(.body \| test("(^\|\\n)Brief: desk-supervision/36\\r?(\\n\|$)"))' && f=".[] \| $a \| $v \| $b \| .number" && m=".[] \| $a \| $b \| .number" && k='[{"number":1,"state":"OPEN","author":{"login":"app/assay-worker-app"},"body":"x\nBrief: desk-supervision/36\n"},{"number":2,"state":"MERGED","author":{"login":"assay-verifier-app[bot]"},"body":"Brief: desk-supervision/36\n"},{"number":3,"state":"MERGED","author":{"login":"app/assay-verifier-app"},"body":"x\nBrief: desk-supervision/36\r\n"},{"number":4,"state":"CLOSED","author":{"login":"app/assay-worker-app"},"body":"Brief: desk-supervision/36\n"}]' && test "$(jq -nr "$k \| $f" \| paste -sd, -)" = 1 && test "$(jq -nr "$k \| $m" \| paste -sd, -)" = 1,2,3 && echo control-ok && M=65a2c1247eab4b5aae35b805900d45a679209e73 && git merge-base --is-ancestor "$M" HEAD && for n in 4628; do h="$(git rev-parse "$M^2")" && s="$(git rev-parse "$M^1")" && test "$h" = aa1e747a69b597a94b85fdf104b034a071d8461f && m="$(git merge-base "$s" "$h")" && test -z "$(git diff --no-renames --name-only "$m" "$h" \| grep -vxF -e tools/desk/internal/deskkit/declaredscope.go -e tools/desk/internal/deskkit/verifywake.go -e tools/desk/internal/deskkit/declaredscope_test.go -e tools/desk/internal/deskkit/scopekey_test.go -e tools/desk/internal/deskkit/scopekey-mutations.json -e docs/declared-input-scope.md -e docs/streams/desk-supervision/verify-wake-v1.md -e changelog/scope-key-directory-slash.md -e changelog/desk-supervision-36-scope-key-authoring.md -e docs/streams/desk-supervision/README.md -e docs/streams/desk-supervision/brief-36-directory-scope-keys-carry-a-trailing-slash.md)" && test -z "$(git diff --text --no-color --no-renames "$m" "$h" -- tools/ \| grep -e '^[-+].*EnvSweep' -e '^[-+].*Setenv' -e '^[-+].*os[.]Exit' -e '^[-+].*func TestMain' -e '^[-+].*func init[(]' -e '^[-+].*s2SweepExclusions' -e '^[-+].*corpusCopySetExempt' -e '^[-+].*corpusOperationalStreams' -e '^[-+].*leakShapeRules' -e '^[-+].*AllLeakShapes' -e '^[-+].*forgeSSHHosts')" && test -z "$(git diff --no-renames --name-only --text -G 'gitleaks[:]allow' "$m" "$h")" && echo "#$n untouched" \|\| { echo "#$n TOUCHED"; exit 1; }; done && echo untouched` | exit 0; prints `control-ok`, then one `#<N> untouched` line per delivery PR, then `untouched`. Scoped to the delivery, never to a pinned base, so later changes on main cannot turn it red: it finds every open or merged PR whose body carries the exact line `Brief: desk-supervision/36` and whose author is not the verifier App (none found is a failure, not a pass, so run it after the implementation PR exists, in a checkout whose `origin` is this repo). The verifier's Evidence PRs carry the same trailer but are not deliveries: they add Evidence and outcome records by design, so selecting them would turn the row red for the wrong reason. The selector is proven before the live search: `control-ok` means that on four fixed PR records (a worker-authored open one, a verifier-authored merged one in each login spelling, a closed one) the selector returns only the worker-authored open PR, and the same selector with the verifier-author clause removed (the mutant) returns the verifier-authored ones too, so the control is sensitive to that clause. For each selected PR it diffs the merge-base of its base and head against its head. Each delivery PR must (a) change only the paths named in this brief's `files:` plus this brief, the stream README and the authoring changelog fragment, so no sweep config, ignore file, `.gitattributes` at any depth, workflow, outcome record, other test file or other package can change at all; (b) add or remove no line under `tools/` naming a sweep environment knob, `Setenv`, `os.Exit`, `TestMain`, an `init` function or the sweep's package-level state (`--text --no-color`, so an attribute cannot hide a line); and (c) add or remove no inline gitleaks allow marker anywhere (the regex brackets the colon so this row's own text never matches). A red is fixed in the implementation, never passed by hand | check +mutation |
| 7 | `(cd tools/desk && o=$(mktemp) && go test ./internal/deskkit -run '^(TestScopeKeyLegacyReceiptReads\|TestScopeKeyLeakSweepFixture\|TestDeclaredReceiptForms\|TestDeclaredMembershipWakes\|TestEvaluateWake_UnchangedHolds\|TestEvaluateWake_RelevantChangeFires\|TestEvaluateWake_UnreadableIsCouldNotCheck\|TestEvaluateWake_LegacyAndIncompleteAreUnclassified\|TestRootRevisionReader_OfflineAndContained\|TestOutcomeRecordWritesReceipt\|TestUnchangedThreeState)$' -count=1 -timeout 240s -v > "$o" 2>&1 && go test ./cmd/deskevidence -run '^(TestOutcomeBindingRequiresCurrentLandedEvidence\|TestOutcomeBindingKeepsExistingIdenticalRecordIdempotent\|TestOutcomeInputShapeChangeRequiresFreshVerification\|TestOutcomeCannotLandWithoutCurrentEvidence\|TestReceiptInputsCoverDeliverables)$' -count=1 -timeout 240s -v >> "$o" 2>&1 && go test ./cmd/verifyloop -run '^(TestPlanWakeFixture\|TestWakeHoldTable\|TestWakeNoTokenIsCouldNotCheckNotClosed)$' -count=1 -timeout 240s -v >> "$o" 2>&1 && for t in TestScopeKeyLegacyReceiptReads TestScopeKeyLeakSweepFixture TestDeclaredReceiptForms TestDeclaredMembershipWakes TestEvaluateWake_UnchangedHolds TestEvaluateWake_RelevantChangeFires TestEvaluateWake_UnreadableIsCouldNotCheck TestEvaluateWake_LegacyAndIncompleteAreUnclassified TestRootRevisionReader_OfflineAndContained TestOutcomeRecordWritesReceipt TestUnchangedThreeState TestOutcomeBindingRequiresCurrentLandedEvidence TestOutcomeBindingKeepsExistingIdenticalRecordIdempotent TestOutcomeInputShapeChangeRequiresFreshVerification TestOutcomeCannotLandWithoutCurrentEvidence TestReceiptInputsCoverDeliverables TestPlanWakeFixture TestWakeHoldTable TestWakeNoTokenIsCouldNotCheckNotClosed; do grep -qF -- "--- PASS: $t (" "$o" \|\| { echo "no PASS: $t"; exit 1; }; done && ! grep -qF -- '--- FAIL' "$o")` | exit 0. Admission binding, receipt validation, the producer and the wake fixtures stay green (the flow across all consumers) | check:ci |
| 8 | `git merge-base --is-ancestor 65a2c1247eab4b5aae35b805900d45a679209e73 HEAD && out=$(statusgen --root . --lint 2>&1); printf '%s\n' "$out" \| grep -E '^LINT: ' && ! printf '%s\n' "$out" \| grep -E '^PROBLEM: .*desk-supervision/(brief-)?36([^0-9]\|$)' \| grep .` | exit 0; LINT PASS | check:ci |
| 9 | `git merge-base --is-ancestor 65a2c1247eab4b5aae35b805900d45a679209e73 HEAD && out=$(statusgen --root . --consumers --brief desk-supervision/36 --delivery 65a2c1247eab4b5aae35b805900d45a679209e73 --base 2e355f68913f0f253f1e206089ac394131106287 2>&1) && printf '%s\n' "$out" \| grep -E '^summary: ' && printf '%s\n' "$out" \| grep -qE '^summary: 4 corroborated, 0 disproved, '` | exit 0; the four follow-up rows are corroborated by the implementation diff | check:ci |
| 10 | `(cd tools/desk && o=$(mktemp) && go test ./internal/deskkit -run '^TestScopeKeyValueMustBeDigest$' -count=1 -timeout 120s -v > "$o" 2>&1; grep -F -- '--- PASS: TestScopeKeyValueMustBeDigest' "$o")` | exit 0. A 40-character base62 value under a slash-keyed directory key is refused as could-not-check; a well-formed digest is accepted | check +mutation |
| 11 | `T=$(mktemp -d) && (cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/scopekey-mutations.json > "$T/out" 2>&1); rc=$?; L=$(tail -1 "$T/out"); echo "$L"; [ $rc -eq 0 ] && [ "$L" = 'Totals: 3 caught, 0 NOT CAUGHT, 0 could-not-mutate.' ]` | exit 0 and prints exactly `Totals: 3 caught, 0 NOT CAUGHT, 0 could-not-mutate.` (the count is pinned; adding a mutant updates this literal in the same change). Baseline GREEN, positive control CAUGHT, and every mutation CAUGHT: the ambiguous-form refusal, the digest-format check and the slash-form glob refusal each have a test that fails without them | check +mutation |
| 12 | `git merge-base --is-ancestor 65a2c1247eab4b5aae35b805900d45a679209e73 HEAD && d=$(mktemp -d) && git archive HEAD \| tar -x -C "$d" && printf 'probe %s-%s\n' example withheld-channel > "$d/docs/zz-sweep-plant.md" && { (cd "$d/tools/desk" && go test ./internal/deskkit/ -run '^TestS2DisclosureSweepIsClean$' -count=1 -v -timeout 300s > "$d/planted.log" 2>&1); p=$?; (cd tools/desk && go test ./internal/deskkit/ -run '^TestS2DisclosureSweepIsClean$' -count=1 -v -timeout 300s > "$d/clean.log" 2>&1 && go test ./internal/deskkit/ -run '^TestS2ShapeRepoGate$' -count=1 -v -timeout 300s >> "$d/clean.log" 2>&1 && go test ./internal/deskkit/ -run '^TestCorpusHasNoWithheldStreamPaths$' -count=1 -v -timeout 300s >> "$d/clean.log" 2>&1); c=$?; echo "planted=$p clean=$c"; test "$p" != 0 && grep -qF -- '--- FAIL: TestS2DisclosureSweepIsClean' "$d/planted.log" && grep -qF -- 'docs/zz-sweep-plant.md' "$d/planted.log" && test "$c" = 0 && grep -qF -- '--- PASS: TestS2DisclosureSweepIsClean' "$d/clean.log" && grep -qF -- '--- PASS: TestS2ShapeRepoGate' "$d/clean.log" && grep -qF -- '--- PASS: TestCorpusHasNoWithheldStreamPaths' "$d/clean.log" && echo sweep-live; }` | exit 0; prints `planted=1 clean=0` then `sweep-live`. Behavioural: the control sweep's own S2 tests, run over a copy of this tree with the S2 test-only token (assembled from two halves, so this row never carries it) planted in a scratch `docs/` file, must FAIL; the same S2 tests over the tree as-is, followed by the corpus tests as a second single-pattern run (one token per `-run`, never an alternation), must pass and print PASS lines, so a neutered sweep and a test binary that exits before any test runs are both red. Catches what a path or keyword check cannot see | check |

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
### Independent verification on desk tools v1.0.65 — 2026-10-29, assay-verifier-app[bot] (claude-opus-5-5, strong)

Verified source: main 5729430698d44ffd874bb25275d056f5a5404dc6, in a detached verifier home. The delivery is PR #4628 (merged as two-parent merge 65a2c1247eab4b5aae35b805900d45a679209e73, first parent 2e355f68913f0f253f1e206089ac394131106287, merge-base 62a8a9915e3cdb452ed3e96eb6175f2a0666e8f8), which lies on the first-parent chain of the verified source. The authoring PR is #3824 (0d125de06f8f2223845f8d650702bcbec3b8a9ed). The Verify section is byte-identical at the authoring merge, the merge-base, the first parent, the delivery and the verified source, and `deskpathguard rederive` reports all 12 rows from-base with 0 author-added. Admission (`deskdispatch --check-verifier`, v1.0.65) exited 0 before any row ran and again before this Evidence was written. statusgen on PATH and in the runner image report v1.0.65. Environment: Darwin 27.0.0 host (go1.27.2, gitleaks 8.30.1), KUBECONFIG=/dev/null. No cluster, production or billed endpoint was contacted. Forge reads were read-only (PR state and commit objects of this repo). Logs are kept in scratch outside the home.

Rows 7, 8 and 9 are check:ci rows, which the darwin host cannot sandbox, so they ran in the closed admission bracket: host `statusgen verifyrun --open-bracket` (tree sha256:14caf0ea5681), then the release-pinned runner image ghcr.io/medici-finance/verify-runner:v1.0.65 (digest sha256:1af55e60c922; statusgen v1.0.65, git 2.56.0, go1.27.3 inside; linux/amd64 under emulation), then host `--close-bracket`. The container ran with `--network none --read-only --cap-drop ALL`, no-new-privileges and uid 501; only lo and tunl0 exist inside it. The run root was a no-hardlinks clone of the home at the verified source, mounted read-only at the home's own path, and its tree digest equals the receipt's. HOME was a read-only copy of the roster file, which holds no credentials, and no token entered the container. `statusgen verifyrun --bracket-digest` on the closed witness reports verdict=match with span-lines=tree-digest:1,container-image:1,attestation:1. The closed witness, verbatim:


| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 7 | `(cd tools/desk && go test ./internal/deskkit/... ./cmd/deskevidence/... ./cmd/verifyloop/... -count=1 -timeout 300s)` | fail exit=1 sandbox=container-netns | sha256:00d9da0a1c5b | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --root . --lint` | fail exit=1 sandbox=container-netns | sha256:94be0cc81716 | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers --brief desk-supervision/36` | fail exit=2 sandbox=container-netns | sha256:36fbf0e9cd6b | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:14caf0ea56811f25c67d30cc8f736f33e114ece3a6504c05fc4a241f9def8d9f
Container-Image: sha256:1af55e60c9222ad09dab8376b629725135c54912bb3583c13591833db90f2d8f
Verification-Attestation: medici-finance/assay#4815 run=463af40eb8a96cfba2455ef5f39b91c26c4b4250ea4d8b36f13209b74bc3a10b source=5729430698d44ffd874bb25275d056f5a5404dc6 brief=docs/streams/desk-supervision/brief-36-directory-scope-keys-carry-a-trailing-slash.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=463af40eb8a96cfba2455ef5f39b91c26c4b4250ea4d8b36f13209b74bc3a10b source=5729430698d44ffd874bb25275d056f5a5404dc6 tree=sha256:14caf0ea56811f25c67d30cc8f736f33e114ece3a6504c05fc4a241f9def8d9f witness=sha256:f04318352e195c64a21086dd323e6934cd805e957f8fff1fa72a82a7db532220 opened=2026-10-29T17:02:53Z closed=2026-10-29T17:23:00Z before=receipt after=check-verifier

Rows 1 to 6 and 10 to 12 are `check` rows, so they ran on the host. The host witness below is from `statusgen verifyrun --rows 1,2,3,4,5,6,10,11,12 --dry-run` in the home, after the bracket closed (17:26Z to 17:28Z). The table was printed, not written into the home. Its bytes, verbatim:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd tools/desk && o=$(mktemp) && go test ./internal/deskkit -run '^TestScopeKeyDirectoryForm$' -count=1 -timeout 120s -v > "$o" 2>&1; grep -F -- '--- PASS: TestScopeKeyDirectoryForm' "$o")` | pass exit=0 | sha256:a6e9b177a187 | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd tools/desk && o=$(mktemp) && go test ./internal/deskkit -run '^TestScopeKeyLegacyReceiptReads$' -count=1 -timeout 120s -v > "$o" 2>&1; grep -F -- '--- PASS: TestScopeKeyLegacyReceiptReads' "$o")` | pass exit=0 | sha256:a6a931b7e8bf | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd tools/desk && o=$(mktemp) && go test ./internal/deskkit -run '^TestScopeKeyAmbiguousForms$' -count=1 -timeout 120s -v > "$o" 2>&1; grep -F -- '--- PASS: TestScopeKeyAmbiguousForms' "$o")` | pass exit=0 | sha256:b5054abc5e54 | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd tools/desk && o=$(mktemp) && go test ./internal/deskkit -run '^TestScopeKeyLayersIndependent$' -count=1 -timeout 120s -v > "$o" 2>&1; grep -F -- '--- PASS: TestScopeKeyLayersIndependent' "$o")` | pass exit=0 | sha256:1fc426c29553 | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 5 | `d=$(mktemp -d) && (cd tools/desk && SCOPE_KEY_FIXTURE_DIR="$d" go test ./internal/deskkit -run '^TestScopeKeyLeakSweepFixture$' -count=1 -timeout 120s) && p=docs/streams/verify-outcomes/scope-fixture/01-20261021T000000Z-000000000000.json && test -f "$d/new/$p" && test -f "$d/legacy/$p" && test -f "$d/planted/$p" && gitleaks version \| grep -Fx 8.30.1 && cp .gitleaks.toml "$d/new/" && cp .gitleaks.toml "$d/legacy/" && cp .gitleaks.toml "$d/planted/" && { (cd "$d/new" && gitleaks dir . --config .gitleaks.toml --exit-code 9 --no-banner --redact >/dev/null 2>&1); a=$?; (cd "$d/legacy" && gitleaks dir . --config .gitleaks.toml --exit-code 9 --no-banner --redact >/dev/null 2>&1); b=$?; (cd "$d/planted" && gitleaks dir . --config .gitleaks.toml --exit-code 9 --no-banner --redact --report-format json --report-path "$d/planted.json" >/dev/null 2>&1); c=$?; echo "new=$a legacy=$b planted=$c"; test "$a" = 0 && test "$b" = 9 && test "$c" = 9 && jq -e 'length > 0 and all(.[]; .RuleID != "generic-api-key")' "$d/planted.json" >/dev/null && echo planted-rule-dedicated; }` | pass exit=0 | sha256:e6dc3834a635 | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 6 | `a='select(.state != "CLOSED")' && v='select((.author.login \| sub("^app/";"") \| sub("\\[bot\\]$";"")) != "assay-verifier-app")' && b='select(.body \| test("(^\|\\n)Brief: desk-supervision/36\\r?(\\n\|$)"))' && f=".[] \| $a \| $v \| $b \| .number" && m=".[] \| $a \| $b \| .number" && k='[{"number":1,"state":"OPEN","author":{"login":"app/assay-worker-app"},"body":"x\nBrief: desk-supervision/36\n"},{"number":2,"state":"MERGED","author":{"login":"assay-verifier-app[bot]"},"body":"Brief: desk-supervision/36\n"},{"number":3,"state":"MERGED","author":{"login":"app/assay-verifier-app"},"body":"x\nBrief: desk-supervision/36\r\n"},{"number":4,"state":"CLOSED","author":{"login":"app/assay-worker-app"},"body":"Brief: desk-supervision/36\n"}]' && test "$(jq -nr "$k \| $f" \| paste -sd, -)" = 1 && test "$(jq -nr "$k \| $m" \| paste -sd, -)" = 1,2,3 && echo control-ok && p="$(gh pr list -R medici-finance/assay --state all --limit 100 --search '"Brief: desk-supervision/36" in:body' --json number,state,body,author --jq "$f")" && test -n "$p" && printf '%s\n' "$p" \| while read -r n; do h="$(gh api "repos/medici-finance/assay/pulls/$n" --jq .head.sha)" && s="$(gh api "repos/medici-finance/assay/pulls/$n" --jq .base.sha)" && git fetch -q --no-tags origin "$h" "$s" && m="$(git merge-base "$s" "$h")" && test -z "$(git diff --no-renames --name-only "$m" "$h" \| grep -vxF -e tools/desk/internal/deskkit/declaredscope.go -e tools/desk/internal/deskkit/verifywake.go -e tools/desk/internal/deskkit/declaredscope_test.go -e tools/desk/internal/deskkit/scopekey_test.go -e tools/desk/internal/deskkit/scopekey-mutations.json -e docs/declared-input-scope.md -e docs/streams/desk-supervision/verify-wake-v1.md -e changelog/scope-key-directory-slash.md -e changelog/desk-supervision-36-scope-key-authoring.md -e docs/streams/desk-supervision/README.md -e docs/streams/desk-supervision/brief-36-directory-scope-keys-carry-a-trailing-slash.md)" && test -z "$(git diff --text --no-color --no-renames "$m" "$h" -- tools/ \| grep -e '^[-+].*EnvSweep' -e '^[-+].*Setenv' -e '^[-+].*os[.]Exit' -e '^[-+].*func TestMain' -e '^[-+].*func init[(]' -e '^[-+].*s2SweepExclusions' -e '^[-+].*corpusCopySetExempt' -e '^[-+].*corpusOperationalStreams' -e '^[-+].*leakShapeRules' -e '^[-+].*AllLeakShapes' -e '^[-+].*forgeSSHHosts')" && test -z "$(git diff --no-renames --name-only --text -G 'gitleaks[:]allow' "$m" "$h")" && echo "#$n untouched" \|\| { echo "#$n TOUCHED"; exit 1; }; done && echo untouched` | fail exit=1 | sha256:45fddd70bfb3 | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 10 | `(cd tools/desk && o=$(mktemp) && go test ./internal/deskkit -run '^TestScopeKeyValueMustBeDigest$' -count=1 -timeout 120s -v > "$o" 2>&1; grep -F -- '--- PASS: TestScopeKeyValueMustBeDigest' "$o")` | pass exit=0 | sha256:aa8b1b12b5f9 | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 11 | `T=$(mktemp -d) && (cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/scopekey-mutations.json > "$T/out" 2>&1); rc=$?; L=$(tail -1 "$T/out"); echo "$L"; [ $rc -eq 0 ] && [ "$L" = 'Totals: 3 caught, 0 NOT CAUGHT, 0 could-not-mutate.' ]` | pass exit=0 | sha256:0f8741a6cd7b | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 12 | `d=$(mktemp -d) && git archive HEAD \| tar -x -C "$d" && printf 'probe %s-%s\n' example withheld-channel > "$d/docs/zz-sweep-plant.md" && { (cd "$d/tools/desk" && go test ./internal/deskkit/ -run TestS2 -count=1 -v -timeout 300s > "$d/planted.log" 2>&1); p=$?; (cd tools/desk && go test ./internal/deskkit/ -run TestS2 -count=1 -v -timeout 300s > "$d/clean.log" 2>&1 && go test ./internal/deskkit/ -run TestCorpus -count=1 -v -timeout 300s >> "$d/clean.log" 2>&1); c=$?; echo "planted=$p clean=$c"; test "$p" != 0 && grep -qF -- '--- FAIL: TestS2DisclosureSweepIsClean' "$d/planted.log" && test "$c" = 0 && grep -qF -- '--- PASS: TestS2DisclosureSweepIsClean' "$d/clean.log" && grep -qF -- '--- PASS: TestS2ShapeRepoGate' "$d/clean.log" && grep -qF -- '--- PASS: TestCorpusHasNoWithheldStreamPaths' "$d/clean.log" && echo sweep-live; }` | pass exit=0 | sha256:dd39838811e6 | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4815 run=463af40eb8a96cfba2455ef5f39b91c26c4b4250ea4d8b36f13209b74bc3a10b source=5729430698d44ffd874bb25275d056f5a5404dc6 brief=docs/streams/desk-supervision/brief-36-directory-scope-keys-carry-a-trailing-slash.md model=claude-opus-5-5 tier=strong

verifyrun records only an exit status and an output hash. Rows 1 to 5 and 10 to 12 each assert their own Expect inside the command (the PASS-line grep, the new/legacy/planted exit codes and rule check, the exact Totals line, the planted/clean exit codes and PASS lines), so exit 0 discharges the stated output. The row 11 output hash equals the sha256 of the exact line `Totals: 3 caught, 0 NOT CAUGHT, 0 could-not-mutate.` and the row 12 hash equals that of `planted=1 clean=0` followed by `sweep-live`. The key lines below come from separate captures of the same commands. Those captures are supplemental and earn no row credit.

| # | Command | Expected | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | Verify row 1 as written | exit 0; directory key carries the slash, file/absent/glob keys do not, digest equals the base-pinned golden | Host witness: pass exit=0. Supplemental base probe, no credit: the first-parent producer over the same fixture tree writes exactly the three golden digests pinned in the test (directory 3baaf58d5c0c…064c, file 7531a10303cc…12c0, absent 4524e523bad6…8e0f). | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | Verify row 2 as written | exit 0 | Host witness: pass exit=0. | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | Verify row 3 as written | exit 0 | Host witness: pass exit=0. Row 11 covers its mutation obligation. | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | Verify row 4 as written | exit 0 | Host witness: pass exit=0. | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | Verify row 5 as written | exit 0; new=0 legacy=9 planted=9, planted rule dedicated | Host witness: pass exit=0. Capture: `8.30.1`, `new=0 legacy=9 planted=9`; the planted report's only RuleID is github-pat. | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 6 | Verify row 6 as written | exit 0; `control-ok`, one `#N untouched` per delivery PR, `untouched` | Host witness: fail exit=1. COULD-NOT-CHECK, not a product failure: an admitted verifyrun removes GH_TOKEN and GITHUB_TOKEN from every row's environment, so the row's forge reads ran with no sanctioned credential. Supplemental probe, no credit: the identical command in the same home, with the same admitted git environment plus the verifier App's read token, exits 0 and prints `control-ok`, `#4628 untouched`, `untouched` (selector picks #4628 only; head aa1e747a69b5, base c5fcbfddf470, merge-base 62a8a9915e3c; no path outside the allowlist, no sweep-knob line, no allow marker). The same command with no credential prints `control-ok` and stops at the forge CLI's login prompt. | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 7 | Verify row 7 as written | exit 0 | Bracket witness: fail exit=1 sandbox=container-netns. COULD-NOT-CHECK, environment: a capture in an identical container shows `panic: test timed out after 5m0s` for the deskkit and deskevidence packages (each about 300s under amd64 emulation) and `ok` for verifyloop, with no failing test reported. A supplemental host run under heavy load also hit the 300s package budget in deskkit only, and deskevidence and verifyloop passed. Main CI at the verified source is green on the desk-1, desk-2 and desk-3 test shards. | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 8 | Verify row 8 as written | exit 0; LINT PASS | Bracket witness: fail exit=1 sandbox=container-netns. COULD-NOT-CHECK, outside this brief: the capture's single PROBLEM is `desktools-v2/65: protected table: acceptance transition: could-not-check human authority: --acceptance-forge is required`, then `LINT: FAIL 1 problem(s)`; nothing names this brief. Supplemental probe, no credit: `statusgen --root . --lint --acceptance-forge` on the host with a read-only forge token prints `LINT: PASS`, the form main CI runs. | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 9 | Verify row 9 as written | exit 0; the four follow-up rows corroborated | Bracket witness: fail exit=2 sandbox=container-netns. COULD-NOT-CHECK: `the brief's delivery could not be derived: the committed Evidence has no Verification-Table marker naming the delivery`. Supplemental probe, no credit: with explicit delivery 65a2c1247eab and first parent 2e355f68913f, exit 0 with 4 CORROBORATED (declaredscope.go, verifywake.go, the declared-input-scope contract, verify-wake-v1), 0 disproved and 6 UNCHECKED out-of-scope entries. In a scratch clone with this Evidence's marker committed, the row as written derives the same delivery range and prints the same 4 corroborated. | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 10 | Verify row 10 as written | exit 0 | Host witness: pass exit=0. Row 11 covers its mutation obligation. | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 11 | Verify row 11 as written | exit 0, exactly `Totals: 3 caught, 0 NOT CAUGHT, 0 could-not-mutate.` | Host witness: pass exit=0; the output hash matches that exact line. | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 12 | Verify row 12 as written | exit 0; `planted=1 clean=0`, `sweep-live` | Host witness: pass exit=0; the output hash matches those two lines. | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Verification-Table: delivery=65a2c1247eab4b5aae35b805900d45a679209e73 base=62a8a9915e3cdb452ed3e96eb6175f2a0666e8f8 original=0d125de06f8f2223845f8d650702bcbec3b8a9ed

Risk-bearing values: every risk field is `no` and the gate is model, but the delivery touches tools/desk/internal/deskkit/, so the procedure applies. The enumeration covered the delivery diff 2e355f68913f..65a2c1247eab, which has 7 files: declaredscope.go, verifywake.go, scopekey_test.go, scopekey-mutations.json, two contract docs and a changelog fragment. These are the literals it found, ranked by what they gate:
1. the scope digest format `^[0-9a-f]{64}$` at declaredscope.go:21, which gates receipt acceptance;
2. the three golden digests at scopekey_test.go:17, :20 and :21;
3. the ambiguity and shape counts `len(canonical) != 1` at declaredscope.go:53 and `len(scope.Shapes) != 1` at verifywake.go:530;
4. the glob metacharacters `*?[` at declaredscope.go:23 and the directory kind `tree` with its `/` suffix at declaredscope.go:29-30;
5. the planted-token shapes at scopekey_test.go:299 and :350;
6. the 120s test timeouts in the mutation spec, which are operational and rank last.

- RISK-VALUE: DERIVED — scope digest format = `^[0-9a-f]{64}$` @ tools/desk/internal/deskkit/declaredscope.go:21 — the only writer of a scope value (declaredscope.go:648) is Sha256Hex (audit.go:287), the lowercase hex encoding of a 32-byte sha256 sum, which is exactly 64 characters of [0-9a-f].
- RISK-VALUE: DERIVED — golden digests 3baaf58d5c0c…064c / 7531a10303cc…12c0 / 4524e523bad6…8e0f @ tools/desk/internal/deskkit/scopekey_test.go:17,20,21 — the first-parent producer, run over the same fixture tree, writes exactly these values, so the pins come from the base and not from the changed code.
- RISK-VALUE: DERIVED — single-form counts (`!= 1`) @ declaredscope.go:53 and verifywake.go:530 — a declaration must resolve to exactly one canonical path and one shape. That is the brief's both-forms-refused rule, and row 3 together with its row 11 mutant (`if plain && dir {`) shows the check is live.
- RISK-VALUE: DERIVED — glob metacharacters `*?[` and directory kind `tree` @ declaredscope.go:23,30 — both are the same values the first-parent file already used for glob detection (line 443) and for the directory kind (lines 492 and 621), so the keying rule does not change what counts as a glob or a directory.
- RISK-VALUE: DERIVED — planted token `ghp_` + 36 base62 @ scopekey_test.go:350 — this is the classic GitHub personal-token shape. The row 5 capture shows gitleaks attributes it to github-pat alone. The 40-character base62 value at :299 is the generic-api-key shape the brief keeps out of digest positions, and row 10 shows it is refused.
- RISK-VALUE: NAMED, NOT DERIVED — mutation and test timeouts 120s @ scopekey-mutations.json — an operational time budget chosen by the author. It bounds runtime only and gates no acceptance value.

**VERIFY: BLOCKED** 8/12 @ 5729430698d4 — rows 1-5 and 10-12 pass, and the closed bracket reports verdict=match. Rows 6, 7, 8 and 9 are COULD-NOT-CHECK, with no obligation observed failing. Row 6: the admitted run strips the forge token the row needs. Row 7: the package test budget of 300s is exceeded under emulation. Row 8: lint red on desktools-v2/65 without `--acceptance-forge`, outside this brief. Row 9: delivery not derivable until this Evidence's Verification-Table marker lands. Leave the lifecycle at implemented.

### Independent verification on desk tools v1.0.67 — 2026-10-30, assay-verifier-app[bot] (claude-opus-5-5, strong)

Verified source: main 011f14965ef2cf173ee6be4ec3d4f319bb55ccae, in a detached verifier home. The delivery is PR #4628 (merged as two-parent merge 65a2c1247eab4b5aae35b805900d45a679209e73, first parent 2e355f68913f0f253f1e206089ac394131106287, merge-base 62a8a9915e3cdb452ed3e96eb6175f2a0666e8f8). The authoring PR is #3824. `deskpathguard rederive` over this home reports 12 rows, 0 author-added. The verifier did not implement this brief. Host: darwin-arm64, go1.27.2, git 2.56.0, gitleaks 8.30.1.

Rows 7, 8 and 9 are check:ci rows, so they ran in the closed admission bracket, in the arm64 image of ghcr.io/medici-finance/verify-runner:v1.0.67 (digest sha256:5fa242b9d02a). The bracket opened at 23:08:05Z and closed at 23:14:40Z. `statusgen verifyrun --bracket-digest` reports `verdict=match span-lines=tree-digest:1,container-image:1,attestation:1`. Its bytes, verbatim:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 7 | `(cd tools/desk && go test ./internal/deskkit/... ./cmd/deskevidence/... ./cmd/verifyloop/... -count=1 -timeout 300s)` | fail exit=1 sandbox=container-netns | sha256:6438b5b28583 | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --root . --lint` | fail exit=1 sandbox=container-netns | sha256:b6ed4f3e3c85 | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers --brief desk-supervision/36` | pass exit=0 sandbox=container-netns | sha256:5177b4b602af | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:8efeda67ba2e48ea5955b3b46beac792fba116f581ab6029ab97c0c654ca294d
Container-Image: sha256:5fa242b9d02a5f769fe9d541f5a8ad33ae291e418a735617faf4812ef21e8629
Verification-Attestation: medici-finance/assay#5062 run=9fbf4e41b2ec1fa6e45eff4016c74f41b26c218270568173094b49e6f33dfd61 source=011f14965ef2cf173ee6be4ec3d4f319bb55ccae brief=docs/streams/desk-supervision/brief-36-directory-scope-keys-carry-a-trailing-slash.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=9fbf4e41b2ec1fa6e45eff4016c74f41b26c218270568173094b49e6f33dfd61 source=011f14965ef2cf173ee6be4ec3d4f319bb55ccae tree=sha256:8efeda67ba2e48ea5955b3b46beac792fba116f581ab6029ab97c0c654ca294d witness=sha256:31cd7e1ae28e2ff25417cdcfc4adfdf2544eb2ddb81ab2e963f0b20ee8404f7a opened=2026-10-30T23:08:05Z closed=2026-10-30T23:14:40Z before=receipt after=check-verifier

Rows 1 to 6 and 10 to 12 are `check` rows, so they ran on the host. The host witness below is from `statusgen verifyrun --rows 1,2,3,4,5,6,10,11,12 --dry-run` in the home, after the bracket closed. The table was printed, not written into the home. Its bytes, verbatim:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd tools/desk && o=$(mktemp) && go test ./internal/deskkit -run '^TestScopeKeyDirectoryForm$' -count=1 -timeout 120s -v > "$o" 2>&1; grep -F -- '--- PASS: TestScopeKeyDirectoryForm' "$o")` | pass exit=0 | sha256:a6e9b177a187 | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd tools/desk && o=$(mktemp) && go test ./internal/deskkit -run '^TestScopeKeyLegacyReceiptReads$' -count=1 -timeout 120s -v > "$o" 2>&1; grep -F -- '--- PASS: TestScopeKeyLegacyReceiptReads' "$o")` | pass exit=0 | sha256:bf30df504c2d | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd tools/desk && o=$(mktemp) && go test ./internal/deskkit -run '^TestScopeKeyAmbiguousForms$' -count=1 -timeout 120s -v > "$o" 2>&1; grep -F -- '--- PASS: TestScopeKeyAmbiguousForms' "$o")` | pass exit=0 | sha256:9a25c3884a55 | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd tools/desk && o=$(mktemp) && go test ./internal/deskkit -run '^TestScopeKeyLayersIndependent$' -count=1 -timeout 120s -v > "$o" 2>&1; grep -F -- '--- PASS: TestScopeKeyLayersIndependent' "$o")` | pass exit=0 | sha256:4cd625ea567f | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 5 | `d=$(mktemp -d) && (cd tools/desk && SCOPE_KEY_FIXTURE_DIR="$d" go test ./internal/deskkit -run '^TestScopeKeyLeakSweepFixture$' -count=1 -timeout 120s) && p=docs/streams/verify-outcomes/scope-fixture/01-20261021T000000Z-000000000000.json && test -f "$d/new/$p" && test -f "$d/legacy/$p" && test -f "$d/planted/$p" && gitleaks version \| grep -Fx 8.30.1 && cp .gitleaks.toml "$d/new/" && cp .gitleaks.toml "$d/legacy/" && cp .gitleaks.toml "$d/planted/" && { (cd "$d/new" && gitleaks dir . --config .gitleaks.toml --exit-code 9 --no-banner --redact >/dev/null 2>&1); a=$?; (cd "$d/legacy" && gitleaks dir . --config .gitleaks.toml --exit-code 9 --no-banner --redact >/dev/null 2>&1); b=$?; (cd "$d/planted" && gitleaks dir . --config .gitleaks.toml --exit-code 9 --no-banner --redact --report-format json --report-path "$d/planted.json" >/dev/null 2>&1); c=$?; echo "new=$a legacy=$b planted=$c"; test "$a" = 0 && test "$b" = 9 && test "$c" = 9 && jq -e 'length > 0 and all(.[]; .RuleID != "generic-api-key")' "$d/planted.json" >/dev/null && echo planted-rule-dedicated; }` | pass exit=0 | sha256:5ab2e12505af | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 6 | `a='select(.state != "CLOSED")' && v='select((.author.login \| sub("^app/";"") \| sub("\\[bot\\]$";"")) != "assay-verifier-app")' && b='select(.body \| test("(^\|\\n)Brief: desk-supervision/36\\r?(\\n\|$)"))' && f=".[] \| $a \| $v \| $b \| .number" && m=".[] \| $a \| $b \| .number" && k='[{"number":1,"state":"OPEN","author":{"login":"app/assay-worker-app"},"body":"x\nBrief: desk-supervision/36\n"},{"number":2,"state":"MERGED","author":{"login":"assay-verifier-app[bot]"},"body":"Brief: desk-supervision/36\n"},{"number":3,"state":"MERGED","author":{"login":"app/assay-verifier-app"},"body":"x\nBrief: desk-supervision/36\r\n"},{"number":4,"state":"CLOSED","author":{"login":"app/assay-worker-app"},"body":"Brief: desk-supervision/36\n"}]' && test "$(jq -nr "$k \| $f" \| paste -sd, -)" = 1 && test "$(jq -nr "$k \| $m" \| paste -sd, -)" = 1,2,3 && echo control-ok && p="$(gh pr list -R medici-finance/assay --state all --limit 100 --search '"Brief: desk-supervision/36" in:body' --json number,state,body,author --jq "$f")" && test -n "$p" && printf '%s\n' "$p" \| while read -r n; do h="$(gh api "repos/medici-finance/assay/pulls/$n" --jq .head.sha)" && s="$(gh api "repos/medici-finance/assay/pulls/$n" --jq .base.sha)" && git fetch -q --no-tags origin "$h" "$s" && m="$(git merge-base "$s" "$h")" && test -z "$(git diff --no-renames --name-only "$m" "$h" \| grep -vxF -e tools/desk/internal/deskkit/declaredscope.go -e tools/desk/internal/deskkit/verifywake.go -e tools/desk/internal/deskkit/declaredscope_test.go -e tools/desk/internal/deskkit/scopekey_test.go -e tools/desk/internal/deskkit/scopekey-mutations.json -e docs/declared-input-scope.md -e docs/streams/desk-supervision/verify-wake-v1.md -e changelog/scope-key-directory-slash.md -e changelog/desk-supervision-36-scope-key-authoring.md -e docs/streams/desk-supervision/README.md -e docs/streams/desk-supervision/brief-36-directory-scope-keys-carry-a-trailing-slash.md)" && test -z "$(git diff --text --no-color --no-renames "$m" "$h" -- tools/ \| grep -e '^[-+].*EnvSweep' -e '^[-+].*Setenv' -e '^[-+].*os[.]Exit' -e '^[-+].*func TestMain' -e '^[-+].*func init[(]' -e '^[-+].*s2SweepExclusions' -e '^[-+].*corpusCopySetExempt' -e '^[-+].*corpusOperationalStreams' -e '^[-+].*leakShapeRules' -e '^[-+].*AllLeakShapes' -e '^[-+].*forgeSSHHosts')" && test -z "$(git diff --no-renames --name-only --text -G 'gitleaks[:]allow' "$m" "$h")" && echo "#$n untouched" \|\| { echo "#$n TOUCHED"; exit 1; }; done && echo untouched` | pass exit=0 | sha256:8b192511d657 | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 10 | `(cd tools/desk && o=$(mktemp) && go test ./internal/deskkit -run '^TestScopeKeyValueMustBeDigest$' -count=1 -timeout 120s -v > "$o" 2>&1; grep -F -- '--- PASS: TestScopeKeyValueMustBeDigest' "$o")` | pass exit=0 | sha256:c0999d835880 | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 11 | `T=$(mktemp -d) && (cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/scopekey-mutations.json > "$T/out" 2>&1); rc=$?; L=$(tail -1 "$T/out"); echo "$L"; [ $rc -eq 0 ] && [ "$L" = 'Totals: 3 caught, 0 NOT CAUGHT, 0 could-not-mutate.' ]` | pass exit=0 | sha256:0f8741a6cd7b | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 12 | `d=$(mktemp -d) && git archive HEAD \| tar -x -C "$d" && printf 'probe %s-%s\n' example withheld-channel > "$d/docs/zz-sweep-plant.md" && { (cd "$d/tools/desk" && go test ./internal/deskkit/ -run TestS2 -count=1 -v -timeout 300s > "$d/planted.log" 2>&1); p=$?; (cd tools/desk && go test ./internal/deskkit/ -run TestS2 -count=1 -v -timeout 300s > "$d/clean.log" 2>&1 && go test ./internal/deskkit/ -run TestCorpus -count=1 -v -timeout 300s >> "$d/clean.log" 2>&1); c=$?; echo "planted=$p clean=$c"; test "$p" != 0 && grep -qF -- '--- FAIL: TestS2DisclosureSweepIsClean' "$d/planted.log" && test "$c" = 0 && grep -qF -- '--- PASS: TestS2DisclosureSweepIsClean' "$d/clean.log" && grep -qF -- '--- PASS: TestS2ShapeRepoGate' "$d/clean.log" && grep -qF -- '--- PASS: TestCorpusHasNoWithheldStreamPaths' "$d/clean.log" && echo sweep-live; }` | pass exit=0 | sha256:dd39838811e6 | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5062 run=9fbf4e41b2ec1fa6e45eff4016c74f41b26c218270568173094b49e6f33dfd61 source=011f14965ef2cf173ee6be4ec3d4f319bb55ccae brief=docs/streams/desk-supervision/brief-36-directory-scope-keys-carry-a-trailing-slash.md model=claude-opus-5-5 tier=strong

verifyrun records only an exit status and an output hash, so each Observed cell below says how far that goes for its row.

| # | Command | Expected | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | Verify row 1 as written | exit 0; directory key carries the slash, file/absent/glob keys do not, digest equals the base-pinned golden | Host witness: pass exit=0. The golden digests were derived independently; see the risk-bearing values below. | 2026-10-30 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | Verify row 2 as written | exit 0 | Host witness: pass exit=0. | 2026-10-30 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | Verify row 3 as written | exit 0 | Host witness: pass exit=0. Row 11 covers its mutation obligation. | 2026-10-30 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | Verify row 4 as written | exit 0 | Host witness: pass exit=0. | 2026-10-30 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | Verify row 5 as written | exit 0; new=0 legacy=9 planted=9, planted rule dedicated | Host witness: pass exit=0. The command itself requires new=0, legacy=9, planted=9 and every planted finding to carry a rule other than generic-api-key, so exit 0 discharges them. The output hash cannot confirm the lines, because the output includes `go test` timing, and no separate capture was taken this run. | 2026-10-30 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 6 | Verify row 6 as written | exit 0; `control-ok`, one `#N untouched` per delivery PR, `untouched` | Host witness: pass exit=0. The output hash equals the sha256 of exactly those three lines, with #4628 as the one delivery PR. The credential was not the verifier App; see the row 6 disclosure below. | 2026-10-30 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 7 | Verify row 7 as written | exit 0 | Bracket witness: fail exit=1 sandbox=container-netns. COULD-NOT-CHECK, environment: the 300s package budget ran out under host load, and no assertion failure was observed. Supplemental probes, no credit, are below. | 2026-10-30 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 8 | Verify row 8 as written | exit 0; LINT PASS | Bracket witness: fail exit=1 sandbox=container-netns. COULD-NOT-CHECK, outside this brief: the single PROBLEM names desktools-v2/65 and needs `--acceptance-forge`, tracked at #4823. Nothing in the capture names this brief. | 2026-10-30 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 9 | Verify row 9 as written | exit 0; the four follow-up rows corroborated | Bracket witness: pass exit=0 sandbox=container-netns. Host preflight, no credit: 4 corroborated, 0 disproved, 6 unchecked. The container run recorded only its exit and hash. | 2026-10-30 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 10 | Verify row 10 as written | exit 0 | Host witness: pass exit=0. Row 11 covers its mutation obligation. | 2026-10-30 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 11 | Verify row 11 as written | exit 0, exactly `Totals: 3 caught, 0 NOT CAUGHT, 0 could-not-mutate.` | Host witness: pass exit=0; the output hash matches that exact line. | 2026-10-30 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 12 | Verify row 12 as written | exit 0; `planted=1 clean=0`, `sweep-live` | Host witness: pass exit=0; the output hash matches those two lines. | 2026-10-30 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Disclosures:

- **Refused first bracket.** The first bracket attempt was refused before any row ran, with this message: `statusgen verifyrun: container admission refused; no Verify rows executed: the mounted root <verifier home> digests to sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855, not the sha256:8efeda67ba2e48ea5955b3b46beac792fba116f581ab6029ab97c0c654ca294d the host recorded at the open step. The container is not running the admitted home`. The home was mounted at its own host path, which came up empty under this host's container runtime; e3b0c442 is the sha256 of empty input. Its close and digest steps then refused on the missing witness. The second attempt ran the same check unchanged, with the home mounted at another path, and the digest matched the true tree. The coordinating desk's reading is that a refused empty-mount digest, followed by the same unchanged check passing on the true tree, is not a bypass. The reader should weigh that reading; the verifier did not decide it alone.
- **Mount.** The admitted bracket ran with `--network none --read-only --cap-drop ALL --security-opt no-new-privileges` as a non-root user, with a tmpfs at /tmp. The home was mounted read-only at /work, a gitfile read-only over /work/.git pointing at /gitc, and the git common dir read-only at /gitc. The module cache, the receipt and a roster copy were read-only. `GOPROXY=off`, `GOSUMDB=off`, `GOFLAGS=-mod=readonly`, `--root /work`.
- **Sibling scratch.** The verifier took this mount recipe from a sibling verifier's scratch directory and read other sibling verifiers' scratch and task output in the same search. Those verifiers were working other briefs in the same session. The verifier also ran read-only probe containers to find the mount that works. None of this is credited as evidence for any row.
- **Early report.** The verifier sent the desk an early report before this run finished, at the desk's request. The desk then told it to finish under the same dispatch. The rows above come from the finished run.
- **Row 6 credential.** The host invocation set no forge token variable, and an admitted verifyrun removes those variables from each row. The row's `gh` still authenticated. statusgen's own source documents this path: a row that runs the forge CLI itself still finds the stored login, and the cell shim also supplies an ambient copy of it. So row 6 read the forge with a stored login, not a verifier App token; the witness does not record which login. The row's reads are read-only. Its pass is recorded as the run shows it. No token supplement was run for this row. On 2026-10-29 the same row exited 1 under a different credential environment, so its result depends on the runner's credentials, and a desk that requires the verifier's own identity for forge reads may wish to re-run it.

Supplemental probes, no credit (rows 7 and 8, run in the same image and mounts, outside the bracket):

- Row 7 as written, under host load near 40 on 6 CPUs: exit 1, `panic: test timed out after 5m0s` in deskkit at 300.053s with TestS2SweepControlIsLive running, 0 `--- FAIL` lines. untrustcorpus, deskevidence (164.355s) and verifyloop (24.120s) were `ok`.
- Deskkit package alone with `-timeout 1500s`, once the load had fallen: exit 0, `ok` in 200.974s. That is inside the row's 300s budget, so a re-run on a quieter host may discharge row 7 as written. This probe does not discharge it.
- Row 8 as written: exit 1, and its output hash equals the witness hash sha256:b6ed4f3e3c85. The single PROBLEM is `desktools-v2/65: protected table: acceptance transition: could-not-check human authority: --acceptance-forge is required`, followed by `LINT: FAIL 1 problem(s)`. The string desk-supervision/36 appears nowhere in the capture.

Risk-bearing values: every risk field is `no` and the gate is model, but the delivery touches tools/desk/internal/deskkit/, so the procedure applies. The literals were enumerated at the verified source and ranked by what they gate.

- RISK-VALUE: DERIVED — golden digests 3baaf58d5c0c…064c / 7531a10303cc…12c0 / 4524e523bad6…8e0f @ tools/desk/internal/deskkit/scopekey_test.go:17,20,21 — recomputed outside the toolchain from the first-parent producer's shape format (declaredscope.go lines 487, 509 and 519 at 2e355f68913f: sorted `kind:path` entries, NUL-joined, sha256 hex) over the test's fixture, and all three match the pins.
- RISK-VALUE: DERIVED — scope digest format = `^[0-9a-f]{64}$` @ tools/desk/internal/deskkit/declaredscope.go:21 — the only writer of a scope value (declaredscope.go:730) is Sha256Hex (audit.go:287), the lowercase hex of a 32-byte sum, which is exactly 64 characters of [0-9a-f].
- RISK-VALUE: DERIVED — glob metacharacters `*?[` and directory kind `tree` @ declaredscope.go:23,30 — the first-parent file already uses the same values (lines 443 and 446 for globs, lines 492 and 621 for the directory kind), so the keying rule does not change what counts as either.
- RISK-VALUE: DERIVED — single-form counts (`!= 1`) @ declaredscope.go:53 and verifywake.go:535 — a declaration must resolve to exactly one canonical path and one shape, which is the brief's both-forms-refused rule. Rows 3 and 11 pass in this run.
- RISK-VALUE: DERIVED — planted token `ghp_` + 36 base62 @ scopekey_test.go:350 — this is the classic GitHub personal-token shape, and row 5 requires that no planted finding is attributed to generic-api-key. The 40-character base62 value at :299 is the shape row 10 shows refused.
- RISK-VALUE: NAMED, NOT DERIVED — mutation and test timeouts 120s @ scopekey-mutations.json:2 — an operational budget chosen by the author. It bounds runtime only and gates no acceptance value.

**VERIFY: BLOCKED** 10/12 @ 011f14965ef2 — rows 1-6 and 9-12 pass, and the closed bracket reports verdict=match. Rows 7 and 8 are could-not-run, with no acceptance obligation observed failing.

Uncovered rows:

- Row 7: `(cd tools/desk && go test ./internal/deskkit/... ./cmd/deskevidence/... ./cmd/verifyloop/... -count=1 -timeout 300s)`. Could-not-run reason: the deskkit package exhausted its 300s budget under concurrent host load. Blocker: environment time budget, the same class as #4179. Needed capability: a fresh bracket on a host with spare capacity. Next actor: the desk, by re-dispatching when load is lower.
- Row 8: `statusgen --root . --lint`. Could-not-run reason: lint is red on desktools-v2/65 because the network-off lane cannot read human authority without `--acceptance-forge`. Blocker: #4823. Needed capability: a resolution of #4823 or a row definition that can run in this lane. Next actor: the driver or desk owning #4823.

## Review

Gate: model. All four risks are no. The change is reversible, touches source and docs only,
leaves the secret-scanner configuration untouched (row 6), and keeps landed records
byte-identical. The reviewer answers two questions. First, does any reader path treat a missing
or empty digest as unchanged? Row 4 must prove that it does not. Second, does any committed
fixture contain a legacy directory key beside a 64-hex value?
