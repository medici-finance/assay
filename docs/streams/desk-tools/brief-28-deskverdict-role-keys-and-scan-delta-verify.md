---
brief: assay:assay:desk-tools:28
title: Role-keyed verdict signing (deskverdict --key) + the R-7 clause-4 cross-repo scan-delta verify path
why: >-
  A house-private brief (on the house's own toolkit repo) needed a second
  signing role in `deskverdict` — the cross-repo desk-batched scan-delta lane signs with the
  issue-loop App's key, distinct from the existing verdict-by-issue lane's verifier key — and a
  new cross-repo clause-4 verify path in `statusgen`'s R-7 transcriber. Both are house-lane
  CONSUMERS of this repo's tools, so the source lands here per the dist/12 desk-tools-source
  dehouse (`tools/desk/`, `statusgen/` are canonical in this repo); the house-lane planning,
  private verify scripts and testdata stay on the house-private board. This brief is the public
  half of that split — authored and implemented together, the same shape PR #242
  (desk-tools/04) used for a landed cross-repo-driven change.
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-19 by a worker-desk session dispatched from the house's own toolkit repo's
  house-private brief
sources:
  - "the house's own toolkit repo's house-private brief specifying this cross-repo scan-delta
    work — its Task 1 and Task 3 this brief carries out; its Task 2
    (consuming the emitted payload into the intake-scan flow) is deferred pending an unauthored
    public statusgen payload-emission brief and is NOT part of this brief."
  - "freshness-checked 2026-09-19 @ 951ca784 (origin/main) — tools/desk/internal/deskkit/verdict.go,
    tools/desk/cmd/deskverdict/{main,sign,verify,keygen,pubkey}.go, statusgen/{transcribeverdict,
    transcribescan,main}.go all re-read against the tree at that commit before this brief's Task
    text was written."
exec-tier: strong
exec-tier-why: "(b) cross-component: this is the signing primitive a separate cross-repo trust
  lane verifies against — a drift here (a role silently defaulting, a declared-role check skipped)
  fails silently at the consuming lane, not at this repo's own tests."
consumers:
  - "the house's own toolkit repo's house-private consuming brief: out-of-scope (the
    consuming brief — house-private verify scripts + committed fixtures exercising this brief's
    CLI/Go surface — lives on that repo's own board, which this repo's
    consumers-corroboration pass cannot read or verify; tracked there, not here)."
version: 1
id: 0350262d-2d1a-4a4d-af21-880ad708e0c8
---

# Brief 28 — Role-keyed verdict signing (`deskverdict --key`) + the R-7 clause-4 cross-repo scan-delta verify path

## Dependencies
None typed, and the reason is recorded rather than left to be rediscovered.

`verdict-lane/01` (the brief that landed the original single-role signing scheme this brief
generalises) is a HOUSE-PRIVATE brief on the house's own toolkit repo's own board, not this
board — there is no `verdict-lane/01` entry here for a typed `depends:` edge to resolve against,
and a typed id naming a brief that exists on no board this repo's `statusgen --lint` can see is
exactly the dangling-reference PROBLEM class the sibling house-private brief's own `sources:`
field independently ran into for its Task 2 deferral. The prerequisite is real (this brief reads
and extends `AssembleVerdictBody`/`VerifyVerdictBody`, both landed by verdict-lane/01) and is
recorded in prose in the Context `facts:` below instead of as an edge.

<!-- graph: not-a-gate -->

## Context
files: `tools/desk/internal/deskkit/verdict.go`, `tools/desk/cmd/deskverdict/main.go`, `tools/desk/cmd/deskverdict/sign.go`, `tools/desk/cmd/deskverdict/verify.go`, `tools/desk/cmd/deskverdict/keygen.go`, `statusgen/transcribeverdict.go`, `statusgen/transcribescan.go`, `statusgen/main.go`, `statusgen/testdata/scan-delta-payload.json`

single-point-of-failure: NOT a single control. The signature is the first layer; behind it sit
three that fail on DIFFERENT signals in DIFFERENT components — (1) the R-7 clause-4 container-issue
author check (an identity fact: the scan-delta issue must be authored by the issue-loop App's
own API-read identity, not just carry a valid signature), (2) the `--key`/declared-role match
refusal (`VerifyVerdictBodyForRole`), which rejects a verifier-signed artifact on the issue-loop
lane and vice versa even when its signature is perfectly valid, and (3) the per-entry API
re-check plus the same-repo-entry refusal plus the R-7 cl.6 flood tripwire. Key CUSTODY has no
single point either: the private PEM never leaves local config and the public key is a
repo/Actions VARIABLE, so a tree compromise yields no key material at all — proven by a negative
Verify row (row 5 below).

facts:
  - "Landed BEFORE this brief (verdict-lane/01) <!-- graph: not-a-gate -->: `deskverdict sign|verify` signs/verifies a single
    verifier-role payload; the public key resolves from `ASSAY_VERIFIER_PUBKEY`
    (`deskkit.VerifierPubkeyVar`), never a committed file. `AssembleVerdictBody`/`VerifyVerdictBody`
    carried no role concept at all — a signed body's trailer was `<!-- deskverdict-signature v1
    alg=RS256 sig=... -->`, no `role=` field."
  - "This brief GENERALISES that scheme to a second role (`issue-loop`) rather than replacing it:
    `VerdictRoleVerifier` stays the implicit default for `--key`, `AssembleVerdictBody` /
    `VerifyVerdictBody` are now thin shorthands over the new `…ForRole` functions, and a body with
    no `role=` field (every body signed before this brief) is read as declaring `verifier` —
    every pre-existing invocation and every pre-existing test is unaffected byte-for-byte in
    behaviour, though `AssembleVerdictBody`'s own OUTPUT now always carries `role=verifier`
    explicitly."
  - "The declared role travels IN the signed block (a `role=<role>` field in the same HTML-comment
    trailer the signature lives in) and is checked BEFORE any signature arithmetic runs
    (`VerifyVerdictBodyForRole`) — a verifier-signed artifact is refused on the issue-loop lane
    even with an otherwise-valid signature, and the refusal still fires when a key is
    mis-provisioned into the wrong role's Actions variable (the crypto alone cannot catch that
    case; only the declaration can)."
  - "`statusgen` is a SEPARATE Go module from `tools/desk` and shares no code with it
    (`transcribeverdict.go`'s own package doc states this) — `statusgen`'s verdict crypto
    (`verdictCanonicalizeJSON`, `verdictParseBody`, `verdictResolvePubkey`, …) is an independently
    duplicated implementation that must agree with `deskkit`'s byte-for-byte. The clause-4
    scan-delta verify path this brief adds to `statusgen/transcribescan.go` reuses those EXISTING
    statusgen-side primitives (the fenced-block markers, the canonicaliser, the RSA verify call)
    rather than re-duplicating them a third time; it adds only the role-declaration check
    (`scanDeltaDeclaredRole`) and the issue-loop pubkey resolution (`scanDeltaResolvePubkey`) as
    the SAME kind of duplicate-by-necessity the file already carries for the verifier role."
  - "The roster's role-binding vocabulary already names `issue-loop` (confirmed 2026-09-19: this
    house's own `~/.config/assay/roster.env` `role-bindings=` line already binds
    `issue-loop=assay-issue-loop-app`) — the clause-4 container-author check
    (`scanDeltaIssueLoopIdentity`) reads that SAME roster key, so no new roster vocabulary is
    introduced."

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity
  does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- **Never commit key material.** Row 5 is the standing check for this, not a formality.

## Task

### 1. Role-keyed signing in `deskverdict` — `--key verifier|issue-loop`
Add a `--key <role>` selector to both `deskverdict sign` and `deskverdict verify`. `--key`
selects WHICH role's key is used; it never changes WHERE a key comes from. `--key` absent
defaults to `verifier`. An unrecognized role is REFUSED — exit 5 on `sign`, exit 6
(could-not-check) on `verify` — and never falls back to `verifier`.

- **Private key (`sign`), by role**: `deskkit.VerdictRoleVerifier`/`VerdictRoleIssueLoop` name the
  two roles; `resolveSignerPEM(role, override)` generalises the existing three-step order
  (`--pem`, then the role's env override `VERIFIER_PEM`/`ISSUE_LOOP_PEM`, then
  `<role>-app.pem` via `deskkit.FindConfigFile`), failing closed and naming every path searched.
- **Public key (`verify`), by role**: `resolvePublicKeyPEM(role, flagPath)` generalises the
  existing order (`--pubkey`, then the role's variable via `deskkit.PubkeyVarForRole` — a NEW
  function, the single place the role→variable mapping lives) with the SAME `DecodePubkeyVar`
  normalisation (PEM string or base64-of-PEM). No key configured for the role is exit 6, never 0.
  Nothing is committed for either role: no `.github/verify/` directory, no `.pem`/`.key`/
  `issue-loop-pubkey` path of any kind in the tree (row 5).
- **Declared role travels in the signed block**: `AssembleVerdictBodyForRole` writes a
  `role=<role>` field into the trailer; `VerifyVerdictBodyForRole` refuses (exit 1) a block whose
  declared role differs from `--key`, BEFORE any signature arithmetic decides the outcome.
- **`keygen`/`pubkey` stay role-agnostic** by design: they generate/derive a plain keypair, and
  the OPERATOR decides which role's variable the public half goes into (keygen's stderr hint now
  names both variables rather than defaulting to the verifier's).
- Package doc + `usage` in `tools/desk/cmd/deskverdict/main.go` updated to describe both roles
  and both variables.

### 2. Clause-4 verify path in `statusgen transcribe-scan`
Extend `statusgen/transcribescan.go` with the R-7 clause-4 cross-repo scan-delta verify path,
gated behind the SAME R-7 enactment gate as the existing same-repo lane
(`transcribeEnactmentGate`) — this task adds NO second arming path:

- **New CLI mode `--transcribe-scan-delta`** (`runTranscribeScanDelta`), sweeping open issues on
  the home repo exactly the way `runTranscribeVerdict` sweeps verdict issues — a cheap body-shape
  test (`verdictHasPayloadBlock`) selects candidates, and an ordinary issue is skipped silently
  (no clause noise). `--dry-run` is the no-write `--check` surface.
- **Five independent layers** (`planScanDelta` + `verifyScanDeltaEntry`), each naming the clause
  it refuses under:
  1. `clause-4 (author)` — the CONTAINER issue must be authored by the issue-loop App's own
     API-read identity (`scanDeltaIssueLoopIdentity`, reading the roster's `issue-loop` role
     binding) — an identity fact, mirrors R-6 clause-1.
  2. `clause-4 (signature)` — role-declared (`scanDeltaDeclaredRole`) + RS256
     (`scanDeltaVerifyBody`), against `ASSAY_ISSUE_LOOP_PUBKEY` via `scanDeltaResolvePubkey`.
  3. `clause-4 (body-unedited)` — the container issue's `last_edited_at` (via the existing
     `verdictIssueResolver`) must show no edit since creation.
  4. `clause-4 (per-entry author …)` — for an entry whose repo this box CAN read via the API, the
     claimed author is re-checked; a contradiction refuses THAT entry. An entry whose repo is not
     readable rests on the signature + the producer's own check (R-7 cl.4's stated two-tier
     honesty) and is surfaced as a NOTICE, never silently dropped or silently promoted to
     "verified via API".
  5. `clause-4 (same-repo)` — an entry targeting the home repo itself is refused (same-repo issues
     have their own lane above; accepting one here would bypass that lane's re-derivation).
- The R-7 cl.6 flood tripwire (25 CREATEs) applies to one scan-delta issue's payload as a whole.
- `statusgen/testdata/scan-delta-payload.json` is a committed fixture (one entry, byte-identical
  to `renderPlaceholder`'s own output) standing in for the still-deferred producer emission path.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `cd tools/desk && go test -run Verdict ./internal/deskkit/ && go test -run Canonical ./internal/deskkit/ && go test -run WrongKey ./internal/deskkit/ && go test -run Reflow ./internal/deskkit/ && go test -run DeriveAndParse ./internal/deskkit/ && go test -run PubkeyVarForRole ./internal/deskkit/ && go test -run Role ./internal/deskkit/` | exit 0 — every pre-existing verdict test plus `TestValidVerdictRole`, `TestPubkeyVarForRole`, `TestIssueLoopRoleRoundtrip`, `TestRoleMismatchRefusedBeforeCrypto`, `TestUnrecognizedRoleNeverFallsBackToVerifier`, `TestNoRoleFieldDefaultsToVerifier` (chained single-pattern `-run` calls — `go test -run` compiles its argument as RE2, where a literal `\|` inside a markdown table cell is NOT alternation, so this avoids that trap rather than tripping it) |
| 2 | check | `cd tools/desk && go test ./cmd/deskverdict/... -v` | exit 0 — every pre-existing CLI test plus `TestCLIIssueLoopRoundtrip`, `TestCLIRoleMismatchExit1`, `TestCLISignUnknownKeyExit5`, `TestCLIVerifyUnknownKeyExit6`, `TestCLIIssueLoopEnvVarBase64`, `TestCLIIssueLoopNoPubkeyConfiguredExit6`, `TestCLISignIssueLoopPEMFromEnv` |
| 3 | check | `cd statusgen && go test -run ScanDelta . -v` | exit 0 — 14 tests: the roundtrip, the six negative-path fixtures (same-repo, contradicted-author, unreadable-author-accepted, container-not-issue-loop, wrong-role, wrong-key, edited-body, no-pubkey, unsupported-class) and the four `runTranscribeScanDelta` sweep/gate/apply/flood/skip tests |
| 4 | check | `cd statusgen && go test -run TranscribeScan . && go test -run Verdict . && go test -run PubkeyVar .` | exit 0 — the pre-existing same-repo R-7 lane and R-6 verdict-lane suites are unaffected by the shared-decoder generalisation (`verdictDecodePubkeyVar`'s new `varName` parameter) |
| 5 | check | `test ! -e .github/verify && test -z "$(git ls-files "*.pem" "*.key" "*issue-loop-pubkey*")"` | exit 0 — NEGATIVE PATH: no `.github/verify/` directory and no key-material path of any kind lands in this tree for either role |
| 6 | check | `cd tools/desk && gofmt -l cmd/deskverdict/ internal/deskkit/verdict.go internal/deskkit/verdict_test.go && cd ../statusgen && gofmt -l transcribescan.go transcribeverdict.go transcribeverdict_test.go transcribescandelta_test.go main.go` | exit 0, empty output (gofmt-clean) |
| 7 | check +mutation | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRoleMismatchRefusedBeforeCrypto$' -count=1 -v` | exit 0 baseline. MUTATION: in `VerifyVerdictBodyForRole` (`tools/desk/internal/deskkit/verdict.go`), delete the `if declared := extractDeclaredRole(body); declared != wantRole { return VerdictRefused, … }` block — the declared-role check this brief adds. The test then REDDENS: `role mismatch must be REFUSED even with a cryptographically valid signature, got 0 (verified: …)`. Restoring the block returns it to exit 0. This is the row that proves the SECOND trust layer (declaration, independent of the crypto) actually reddens when removed, not merely that it is documented. |

Row 3 is the fail-first-worthy row: every one of its 14 tests was run against the pre-fix tree
first (via `git stash` of the implementation files only) and confirmed to fail on `undefined:
scanDeltaPayload` / `scanDeltaEntry` / … before the implementation was restored — the clause-4
battery genuinely did not exist before this brief. Rows 1–2 have the same fail-first pairing for
the role-keyed selector (`undefined: ValidVerdictRole` / `flag provided but not defined: -key`).
Row 7 was executed for real at authoring time (the mutation applied, the red captured, the file
restored byte-identical — `git diff` clean afterward) — not merely described.

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test -run Verdict ./internal/deskkit/ && go test -run Canonical ./internal/deskkit/ && go test -run WrongKey ./internal/deskkit/ && go test -run Reflow ./internal/deskkit/ && go test -run DeriveAndParse ./internal/deskkit/ && go test -run PubkeyVarForRole ./internal/deskkit/ && go test -run Role ./internal/deskkit/` | pass exit=0 | sha256:5bfd5603074b | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskverdict/... -v` | pass exit=0 | sha256:d35aa731f441 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test -run ScanDelta . -v` | pass exit=0 | sha256:9e68a530a7f7 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go test -run TranscribeScan . && go test -run Verdict . && go test -run PubkeyVar .` | pass exit=0 | sha256:74f742bf4af8 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 5 | `test ! -e .github/verify && test -z "$(git ls-files "*.pem" "*.key" "*issue-loop-pubkey*")"` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && gofmt -l cmd/deskverdict/ internal/deskkit/verdict.go internal/deskkit/verdict_test.go && cd ../statusgen && gofmt -l transcribescan.go transcribeverdict.go transcribeverdict_test.go transcribescandelta_test.go main.go` | fail exit=1 | sha256:c5bc76e0d6b7 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRoleMismatchRefusedBeforeCrypto$' -count=1 -v` | pass exit=0 | sha256:51d2f385e277 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |

Verifier notes (dispatched non-implementer, claude-opus-5-5, merged main 9585b4b6cc2e, 2026-09-27). These expand the key observed output behind the witness rows above:

- Row 1: seven `ok` lines. With -v, TestValidVerdictRole, TestPubkeyVarForRole, TestIssueLoopRoleRoundtrip, TestRoleMismatchRefusedBeforeCrypto, TestUnrecognizedRoleNeverFallsBackToVerifier and TestNoRoleFieldDefaultsToVerifier all PASS.
- Row 2: 16/16 PASS, including all seven new TestCLI* role tests.
- Row 3: 14 PASS (10 TestScanDelta* + 4 TestRunTranscribeScanDelta*), matching the expected count of 14.
- Row 4: three `ok github.com/medici-finance/assay/statusgen` lines.
- Row 5: empty output. There is no .github/verify directory, and no tracked files match the key-material patterns.
- Row 6: `cd: ../statusgen: No such file or directory`. The row's command is wrong: from tools/desk, ../statusgen resolves to tools/statusgen, which has never existed in this repo (statusgen lives at the repo root). The same command with `cd ../../statusgen` exits 0 with empty output, so the code IS gofmt-clean. The defect is in the check definition, not the implementation.
- Row 7 MUTATION, applied by hand: deleting the 3-line declared-role block from VerifyVerdictBodyForRole makes the test exit 1 with `verdict_test.go:368: role mismatch must be REFUSED even with a cryptographically valid signature, got 0 (verified: signature matches the canonical verdict payload)`. I restored the file (clean git diff, file hash unchanged) and the re-run exits 0.
- Review note (b), checked in source: VerifyVerdictBodyForRole (tools/desk/internal/deskkit/verdict.go:453) and scanDeltaVerifyBody (statusgen/transcribescan.go:747) both evaluate the declared-role comparison before any canonicalisation or RSA verify call.

RISK-VALUE: DERIVED — VerdictRoleIssueLoop = "issue-loop" @ tools/desk/internal/deskkit/verdict.go:93 (twin scanDeltaWantRole = "issue-loop" @ statusgen/transcribescan.go:671; roster key read as RoleBots["issue-loop"] @ statusgen/transcribescan.go:774) — this must be the roster's existing role-binding name for the intake/issue-loop App. deskkit already maps intake-desk to "issue-loop" in its role-token table (tools/desk/internal/deskkit/roletoken.go:46). All three copies are byte-identical, so signer, verifier and container-author check agree on one role.
RISK-VALUE: DERIVED — IssueLoopPubkeyVar = "ASSAY_ISSUE_LOOP_PUBKEY" @ tools/desk/internal/deskkit/verdict.go:108 (twin scanDeltaPubkeyVar @ statusgen/transcribescan.go:667) — this follows the existing ASSAY_VERIFIER_PUBKEY convention (verdict.go:515 and statusgen/transcribeverdict.go:80) and is byte-identical across the two independent modules. If they drifted, the consuming lane would silently fall into could-not-check, which is the brief's stated exec-tier risk.
RISK-VALUE: NAMED, NOT DERIVED — privKeyEnvForRole "ISSUE_LOOP_PEM" / privKeyFileForRole "issue-loop-app.pem" @ tools/desk/cmd/deskverdict/sign.go:99,106 — these are local custody lookup names. They follow the VERIFIER_PEM / verifier-app.pem pattern, but the operator-side config that must match them lives outside this repo, so it could not be checked from here. A wrong value is reversible and fails closed, because sign refuses and names every path it searched.
Ranked last (reversible operational knobs, no derivation needed): transcribeFloodThreshold = 25 @ statusgen/transcribescan.go:59 (existing value, reused and unchanged); scanDeltaSchemaVersion = "scan-delta-v1" @ statusgen/transcribescan.go:658; CLI exit codes 5 (sign, unknown --key) and 6 (verify, unknown --key or no pubkey). The item's risk answers are all "no" and it is not irreversible, so every entry above can be fixed by an edit plus a redeploy.

VERIFY: FAIL — rows 1–5 and 7 PASS; row 6 FAILS as written (exit 1). The row 6 command cd's into a directory that does not exist; the property it checks holds (the gofmt-clean command with the corrected path exits 0).
FAIL filed: row 6 check-definition (bad relative path) — medici-finance/assay#1735

### Non-implementer verifier re-run: 2026-10-02T21:46:31Z (UTC), assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian), merged main cf31c32418ba49f93c679913813768542db1c072

| # | Command | Expect | Observed (exit + key output line) | Date / runner |
|---|---------|--------|-----------------------------------|---------------|
| 1 | `cd tools/desk && go test -run Verdict ./internal/deskkit/ && go test -run Canonical ./internal/deskkit/ && go test -run WrongKey ./internal/deskkit/ && go test -run Reflow ./internal/deskkit/ && go test -run DeriveAndParse ./internal/deskkit/ && go test -run PubkeyVarForRole ./internal/deskkit/ && go test -run Role ./internal/deskkit/` | exit 0, pre-existing verdict tests plus the six role tests | exit 0 — seven `ok github.com/medici-finance/assay/tools/desk/internal/deskkit` lines; with -v, TestValidVerdictRole, TestPubkeyVarForRole, TestIssueLoopRoleRoundtrip, TestRoleMismatchRefusedBeforeCrypto, TestUnrecognizedRoleNeverFallsBackToVerifier, TestNoRoleFieldDefaultsToVerifier each `--- PASS` | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test ./cmd/deskverdict/... -v` | exit 0, pre-existing CLI tests plus the seven role CLI tests | exit 0 — 16 `--- PASS`, 0 `--- FAIL`; `ok github.com/medici-finance/assay/tools/desk/cmd/deskverdict`; all seven named TestCLI* role tests present and passing | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | `cd statusgen && go test -run ScanDelta . -v` | exit 0, 14 tests | exit 0 — 14 `--- PASS` (10 TestScanDelta*, 4 TestRunTranscribeScanDelta*), 0 `--- FAIL`; `ok github.com/medici-finance/assay/statusgen` | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | `cd statusgen && go test -run TranscribeScan . && go test -run Verdict . && go test -run PubkeyVar .` | exit 0 | exit 0 — three `ok github.com/medici-finance/assay/statusgen` lines | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | `test ! -e .github/verify && test -z "$(git ls-files "*.pem" "*.key" "*issue-loop-pubkey*")"` | exit 0 (negative path: no key material in tree) | exit 0 — empty output | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | `cd tools/desk && gofmt -l cmd/deskverdict/ internal/deskkit/verdict.go internal/deskkit/verdict_test.go && cd ../statusgen && gofmt -l transcribescan.go transcribeverdict.go transcribeverdict_test.go transcribescandelta_test.go main.go` | exit 0, empty output | exit 1 — `cd: ../statusgen: No such file or directory` (first gofmt half printed nothing; the second half never executed because the cd failed) | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRoleMismatchRefusedBeforeCrypto$' -count=1 -v` | exit 0 baseline; mutation reddens; restore returns exit 0 | exit 0 baseline — `--- PASS: TestRoleMismatchRefusedBeforeCrypto`. Mutation (3-line declared-role block deleted): exit 1 — `verdict_test.go:368: role mismatch must be REFUSED even with a cryptographically valid signature, got 0 (verified: signature matches the canonical verdict payload)`. Restored: exit 0 | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |

Hand-run result: 6 of 7 rows pass; row 6 fails as written.

Execution witness (`statusgen verifyrun --brief <this brief> --dry-run`, tool v1.0.31, nothing written to the brief): 6 of 7 rows pass, witness exit 1.

- row 1: pass (exit=0, sha256:190b1881449c)
- row 2: pass (exit=0, sha256:670d530e6354)
- row 3: pass (exit=0, sha256:a7d1a1675fc0)
- row 4: pass (exit=0, sha256:81b12aa5fa0a)
- row 5: pass (exit=0, sha256:e3b0c44298fc)
- row 6: fail (exit=1, sha256:c5bc76e0d6b7) — exit 1, expected 0
- row 7: pass (exit=0, sha256:51ca871ca4e6)

Findings:

- The verdict has NOT changed since the 2026-09-27 pass. Row 6 fails for the same reason and with the same output hash (sha256:c5bc76e0d6b7): from tools/desk, the relative path ../statusgen resolves to tools/statusgen, which does not exist (statusgen lives at the repo root). The brief file is byte-identical to the one the 2026-09-27 outcome record hashed (sha256 e13d207c98d7…), so the row's command has not been corrected. #1735, which tracks this, is still OPEN.
- What did change among the previously declared inputs: the witness tool (v1.0.27 to v1.0.31) and, in the implementation, statusgen/main.go and statusgen/transcribeverdict.go (other verdict/scan files under this brief are unchanged between 9585b4b6cc2e and cf31c32418ba). Neither change touches row 6's path; rows 3 and 4, which exercise those two files, still pass.
- The property row 6 is meant to check still holds: the same command with the path corrected to `cd ../../statusgen` exits 0 with empty output (gofmt-clean on both halves). The defect is in the check definition, not the implementation. This corrected run is supplementary and does not stand in for the row as authored.
- Row 7 mutation disclosure: the mutation was applied to a scratch copy of the tools/desk module outside the checkout, not to the verification worktree itself; the worktree stayed clean throughout (empty `git status`). After the red run the scratch file was restored byte-identical to the tree's copy (matching sha256) and the test re-run exited 0.
- Run conditions: all `go test` invocations (hand-run and witness) ran with a throwaway HOME and a scratch temp directory rather than the system temp directory, with the Go build and module caches left at their normal locations. No row needed a cluster, a live service or another OS; no row is unrun.
- Review note (b), re-checked in source at this head: the declared-role comparison in VerifyVerdictBodyForRole (tools/desk/internal/deskkit/verdict.go:453) precedes the CanonicalizeJSON call at line 456 and any signature check.

Risk-bearing values (item risk answers are all "no", not irreversible; enumeration re-checked at this head, line numbers unchanged from the prior pass):

RISK-VALUE: DERIVED — VerdictRoleIssueLoop = "issue-loop" @ tools/desk/internal/deskkit/verdict.go:93 (twin scanDeltaWantRole = "issue-loop" @ statusgen/transcribescan.go:671; roster key RoleBots["issue-loop"] @ statusgen/transcribescan.go:774) — must equal the existing role-binding name for the intake App; deskkit's own role-token table maps intake-desk to "issue-loop" (tools/desk/internal/deskkit/roletoken.go:46). All three copies are byte-identical, so signer, verifier and container-author check agree on one role.
RISK-VALUE: DERIVED — IssueLoopPubkeyVar = "ASSAY_ISSUE_LOOP_PUBKEY" @ tools/desk/internal/deskkit/verdict.go:108 (twin scanDeltaPubkeyVar @ statusgen/transcribescan.go:667) — follows the existing VerifierPubkeyVar = "ASSAY_VERIFIER_PUBKEY" convention (verdict.go:515) and is byte-identical across the two independent modules; a drift would put the consuming lane silently into could-not-check.
RISK-VALUE: NAMED, NOT DERIVED — privKeyEnvForRole "ISSUE_LOOP_PEM" / privKeyFileForRole "issue-loop-app.pem" @ tools/desk/cmd/deskverdict/sign.go:99,106 — local custody lookup names following the VERIFIER_PEM / verifier-app.pem pattern; the operator-side config they must match lives outside this repo and could not be checked from here. A wrong value is reversible and fails closed.
Ranked last (reversible operational knobs, no derivation needed): transcribeFloodThreshold = 25 @ statusgen/transcribescan.go:59; scanDeltaSchemaVersion = "scan-delta-v1" @ statusgen/transcribescan.go:658; CLI exit codes 5 (sign, unknown --key) and 6 (verify, unknown --key or no pubkey).

VERIFY: FAIL — rows 1–5 and 7 pass (row 7 mutation killed); row 6 fails as written (exit 1) because its command changes into a directory that does not exist. Check-definition defect, unchanged since 2026-09-27, tracked by #1735 (open).
### Non-implementer verifier re-run: 2026-10-07, assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian), merged main dde4fbeaead9c173e8355869e06d86e8e9eee446

| # | Command | Expect | Observed (exit + key output line) | Date | Runner |
|---|---------|--------|-----------------------------------|------|--------|
| 1 | `cd tools/desk && go test -run Verdict ./internal/deskkit/ && go test -run Canonical ./internal/deskkit/ && go test -run WrongKey ./internal/deskkit/ && go test -run Reflow ./internal/deskkit/ && go test -run DeriveAndParse ./internal/deskkit/ && go test -run PubkeyVarForRole ./internal/deskkit/ && go test -run Role ./internal/deskkit/` | exit 0, pre-existing verdict tests plus the six role tests | exit 0 — seven `ok github.com/medici-finance/assay/tools/desk/internal/deskkit` lines; with -v, TestValidVerdictRole, TestPubkeyVarForRole, TestIssueLoopRoleRoundtrip, TestRoleMismatchRefusedBeforeCrypto, TestUnrecognizedRoleNeverFallsBackToVerifier, TestNoRoleFieldDefaultsToVerifier each `--- PASS` | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskverdict/... -v` | exit 0, pre-existing CLI tests plus the seven role CLI tests | exit 0 — 16 `--- PASS`, 0 `--- FAIL`; `ok github.com/medici-finance/assay/tools/desk/cmd/deskverdict`; all seven named TestCLI* role tests present and passing | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test -run ScanDelta . -v` | exit 0, 14 tests | exit 0 — 14 `--- PASS` (10 TestScanDelta*, 4 TestRunTranscribeScanDelta*), 0 `--- FAIL`; `ok github.com/medici-finance/assay/statusgen` | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go test -run TranscribeScan . && go test -run Verdict . && go test -run PubkeyVar .` | exit 0 | exit 0 — three `ok github.com/medici-finance/assay/statusgen` lines | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 5 | `test ! -e .github/verify && test -z "$(git ls-files "*.pem" "*.key" "*issue-loop-pubkey*")"` | exit 0 (negative path: no key material in tree) | exit 0 — empty output | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && gofmt -l cmd/deskverdict/ internal/deskkit/verdict.go internal/deskkit/verdict_test.go && cd ../statusgen && gofmt -l transcribescan.go transcribeverdict.go transcribeverdict_test.go transcribescandelta_test.go main.go` | exit 0, empty output | exit 1 — `cd: ../statusgen: No such file or directory` (first gofmt half printed nothing; the second half never executed because the cd failed) | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRoleMismatchRefusedBeforeCrypto$' -count=1 -v` | exit 0 baseline; mutation reddens; restore returns exit 0 | exit 0 baseline — `--- PASS: TestRoleMismatchRefusedBeforeCrypto`. Mutation (3-line declared-role block, verdict.go lines 453-455, deleted in a scratch copy of the module): exit 1 — `verdict_test.go:368: role mismatch must be REFUSED even with a cryptographically valid signature, got 0 (verified: signature matches the canonical verdict payload)`. Restored (sha256 matches the tree copy): exit 0 | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |

Hand-run result: 6 of 7 rows pass; row 6 fails as written.

Execution witness (`statusgen verifyrun`, tool v1.0.32, non-dry, run in the verification worktree and then reverted there): 6 of 7 rows pass, witness exit 1; row 6 fail (exit=1, sha256:c5bc76e0d6b7), the same output hash as the 2026-09-27 and 2026-10-02 passes.

Findings:

- The failure still reproduces on merged main dde4fbeaead9. Row 6's command has not changed: the brief has been touched since #1325 only by Evidence commits. From tools/desk, the relative path ../statusgen resolves to tools/statusgen, which does not exist on main (statusgen lives at the repo root). #1735 is still OPEN.
- The property row 6 checks still holds. The same command with the path corrected to `cd ../../statusgen` exits 0 with empty output, so both halves are gofmt-clean. The defect is in the check definition, not the implementation. This corrected run is supplementary and does not replace the row as authored.
- Since cf31c32418ba (the 2026-10-02 head), the only changes under this brief's declared files are statusgen/main.go (#2206, #2281). Rows 3 and 4, which exercise that file, still pass.
- Row 7: the mutation was applied to a scratch copy of the tools/desk module outside the checkout. The verification worktree stayed clean throughout.
- Run conditions: every `go test` (hand-run and witness) ran with a throwaway HOME and a scratch temp directory, with the Go build and module caches at their normal locations. No row needed a cluster, a live service or another OS, and every row executed.
- Review note (b), re-checked at this head: the declared-role comparison in VerifyVerdictBodyForRole (tools/desk/internal/deskkit/verdict.go:453) comes before CanonicalizeJSON at line 456 and before any signature check.

Risk-bearing values (all of the item's risk answers are "no" and it is not irreversible; enumeration re-checked at this head, line numbers unchanged):

RISK-VALUE: DERIVED — VerdictRoleIssueLoop = "issue-loop" @ tools/desk/internal/deskkit/verdict.go:93 (twin scanDeltaWantRole = "issue-loop" @ statusgen/transcribescan.go:671; roster key RoleBots["issue-loop"] @ statusgen/transcribescan.go:774) — this must equal the existing role-binding name for the intake App. deskkit's role-token table maps intake-desk to "issue-loop" (tools/desk/internal/deskkit/roletoken.go:46). All three copies are byte-identical.
RISK-VALUE: DERIVED — IssueLoopPubkeyVar = "ASSAY_ISSUE_LOOP_PUBKEY" @ tools/desk/internal/deskkit/verdict.go:108 (twin scanDeltaPubkeyVar @ statusgen/transcribescan.go:667) — this follows the VerifierPubkeyVar = "ASSAY_VERIFIER_PUBKEY" convention (verdict.go:515) and is byte-identical across the two independent modules.
RISK-VALUE: NAMED, NOT DERIVED — privKeyEnvForRole "ISSUE_LOOP_PEM" / privKeyFileForRole "issue-loop-app.pem" @ tools/desk/cmd/deskverdict/sign.go:99,106 — local custody lookup names. The operator-side config they must match lives outside this repo and could not be checked from here. A wrong value is reversible and fails closed.
Ranked last (reversible operational knobs, no derivation needed): transcribeFloodThreshold = 25 @ statusgen/transcribescan.go:59; scanDeltaSchemaVersion = "scan-delta-v1" @ statusgen/transcribescan.go:658; CLI exit codes 5 (sign, unknown --key) and 6 (verify, unknown --key or no pubkey).

VERIFY: FAIL — rows 1–5 and 7 pass (the row 7 mutation was killed). Row 6 fails as written (exit 1) because its command changes into a directory that does not exist. This is a check-definition defect, unchanged since 2026-09-27 and tracked by #1735 (open).

Attached to #1735: https://github.com/medici-finance/assay/issues/1735#issuecomment-6038139388

### Non-implementer verifier re-run: 2026-10-11T10:30:11Z (UTC), assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian), merged main e9e48aa37bf9886983e8bee3103aea9815c6e73f

Admission: `deskdispatch --check-verifier` exit 0 before any row ran (binding run ce1b6b2fc996…, source e9e48aa37bf9, model claude-opus-5-5, tier any). The attestation line is carried under the witness table below.

| # | Command | Expect | Observed (exit + key output line) | Date | Runner |
|---|---------|--------|-----------------------------------|------|--------|
| 1 | `cd tools/desk && go test -run Verdict ./internal/deskkit/ && go test -run Canonical ./internal/deskkit/ && go test -run WrongKey ./internal/deskkit/ && go test -run Reflow ./internal/deskkit/ && go test -run DeriveAndParse ./internal/deskkit/ && go test -run PubkeyVarForRole ./internal/deskkit/ && go test -run Role ./internal/deskkit/` | exit 0, pre-existing verdict tests plus the six role tests | exit 0 — seven `ok github.com/medici-finance/assay/tools/desk/internal/deskkit` lines; with -v, TestValidVerdictRole, TestPubkeyVarForRole, TestIssueLoopRoleRoundtrip, TestRoleMismatchRefusedBeforeCrypto, TestUnrecognizedRoleNeverFallsBackToVerifier, TestNoRoleFieldDefaultsToVerifier each `--- PASS` | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test ./cmd/deskverdict/... -v` | exit 0, pre-existing CLI tests plus the seven role CLI tests | exit 0 — 19 top-level `--- PASS` (plus 16 subtest PASS lines), 0 `--- FAIL`; `ok github.com/medici-finance/assay/tools/desk/cmd/deskverdict`; all seven named TestCLI* role tests present and passing | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | `cd statusgen && go test -run ScanDelta . -v` | exit 0, 14 tests | exit 0 — 14 `--- PASS` (10 TestScanDelta*, 4 TestRunTranscribeScanDelta*), 0 `--- FAIL`; `ok github.com/medici-finance/assay/statusgen` | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | `cd statusgen && go test -run TranscribeScan . && go test -run Verdict . && go test -run PubkeyVar .` | exit 0 | exit 0 — three `ok github.com/medici-finance/assay/statusgen` lines | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | `test ! -e .github/verify && test -z "$(git ls-files "*.pem" "*.key" "*issue-loop-pubkey*")"` | exit 0 (negative path: no key material in tree) | exit 0 — empty output | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 6 | `cd tools/desk && gofmt -l cmd/deskverdict/ internal/deskkit/verdict.go internal/deskkit/verdict_test.go && cd ../statusgen && gofmt -l transcribescan.go transcribeverdict.go transcribeverdict_test.go transcribescandelta_test.go main.go` | exit 0, empty output | exit 1 — `cd: ../statusgen: No such file or directory` (the first gofmt half printed nothing; the second half never executed because the cd failed) | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRoleMismatchRefusedBeforeCrypto$' -count=1 -v` | exit 0 baseline; mutation reddens; restore returns exit 0 | exit 0 baseline — `--- PASS: TestRoleMismatchRefusedBeforeCrypto`. Mutation (the 3-line declared-role block, verdict.go lines 453-455, deleted in a scratch copy of the module): exit 1 — `verdict_test.go:368: role mismatch must be REFUSED even with a cryptographically valid signature, got 0 (verified: signature matches the canonical verdict payload)`. Restored (sha256 106ccaa1913b matches the tree copy): exit 0 | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

Hand-run result: 6 of 7 rows pass; row 6 fails as written.

Execution witness (`statusgen verifyrun --brief <this brief>`, tool v1.0.37, written by the tool below): 6 of 7 rows pass, witness exit 1; row 6 fail (exit=1, sha256:c5bc76e0d6b7), the same output hash as the 2026-09-27, 2026-10-02 and 2026-10-07 passes. Every row is class check (none is check:ci), so no row needed the network-off sandbox and every row executed on this host.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test -run Verdict ./internal/deskkit/ && go test -run Canonical ./internal/deskkit/ && go test -run WrongKey ./internal/deskkit/ && go test -run Reflow ./internal/deskkit/ && go test -run DeriveAndParse ./internal/deskkit/ && go test -run PubkeyVarForRole ./internal/deskkit/ && go test -run Role ./internal/deskkit/` | pass exit=0 | sha256:5bfd5603074b | 2026-10-11 | assay-verifier-app[bot] @ e9e48aa37bf9 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskverdict/... -v` | pass exit=0 | sha256:2e20bbfd691b | 2026-10-11 | assay-verifier-app[bot] @ e9e48aa37bf9 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test -run ScanDelta . -v` | pass exit=0 | sha256:01d1a6d2da0c | 2026-10-11 | assay-verifier-app[bot] @ e9e48aa37bf9 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go test -run TranscribeScan . && go test -run Verdict . && go test -run PubkeyVar .` | pass exit=0 | sha256:74f742bf4af8 | 2026-10-11 | assay-verifier-app[bot] @ e9e48aa37bf9 (on-behalf-of human:ian) (forge-identity) |
| 5 | `test ! -e .github/verify && test -z "$(git ls-files "*.pem" "*.key" "*issue-loop-pubkey*")"` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ e9e48aa37bf9 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && gofmt -l cmd/deskverdict/ internal/deskkit/verdict.go internal/deskkit/verdict_test.go && cd ../statusgen && gofmt -l transcribescan.go transcribeverdict.go transcribeverdict_test.go transcribescandelta_test.go main.go` | fail exit=1 | sha256:c5bc76e0d6b7 | 2026-10-11 | assay-verifier-app[bot] @ e9e48aa37bf9 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRoleMismatchRefusedBeforeCrypto$' -count=1 -v` | pass exit=0 | sha256:f37001900344 | 2026-10-11 | assay-verifier-app[bot] @ e9e48aa37bf9 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#2576 run=ce1b6b2fc99648bbe9b6825418876b1407ecdbfffe31853a0f00aca93e135892 source=e9e48aa37bf9886983e8bee3103aea9815c6e73f brief=docs/streams/desk-tools/brief-28-deskverdict-role-keys-and-scan-delta-verify.md model=claude-opus-5-5 tier=any

Findings:

- The failure still reproduces on merged main e9e48aa37bf9. Row 6's command has not changed since the brief landed; from tools/desk, the relative path ../statusgen resolves to tools/statusgen, which does not exist (statusgen lives at the repo root). #1735 is still OPEN (read 2026-10-11) and no fix PR references it.
- The property row 6 checks still holds. The same command with the path corrected to `cd ../../statusgen` exits 0 with empty output, so both halves are gofmt-clean. The defect is in the check definition, not the implementation. This corrected run is supplementary and does not replace the row as authored.
- Since dde4fbeaead9 (the 2026-10-07 head), commits touching this brief's declared files are #2362 (deskverdict sign: unsigned-out composition and sign binding to digest, repo, head and time bounds), #2095, #2374, #2457 and #2424. Rows 1-4 and 7 still pass over them. Row 2 now reports 19 top-level tests (16 at the 2026-10-07 pass); the extra tests come from #2362, and all seven role CLI tests this brief names are still present and passing.
- Row 7: the mutation was applied to a scratch copy of the tools/desk module (plus forgeread, which its go.mod replace directive needs), taken with git archive outside the checkout. The verification worktree stayed clean throughout.
- Run conditions: every hand-run `go test` used a throwaway HOME and a scratch temp directory, with the Go build and module caches at their normal locations. No row needed a cluster, a live service or another OS, and every row executed.
- Review note (b), re-checked at this head: the declared-role comparison in VerifyVerdictBodyForRole (tools/desk/internal/deskkit/verdict.go:453) comes before CanonicalizeJSON at line 456. In scanDeltaVerifyBody (statusgen/transcribescan.go:742) the comparison at line 747 comes before verdictCanonicalizeJSON at line 753.

Risk-bearing values (all of the item's risk answers are "no" and it is not irreversible; enumeration over this item's diff re-checked at this head):

RISK-VALUE: DERIVED — VerdictRoleIssueLoop = "issue-loop" @ tools/desk/internal/deskkit/verdict.go:93 (twin scanDeltaWantRole = "issue-loop" @ statusgen/transcribescan.go:671; roster key RoleBots["issue-loop"] @ statusgen/transcribescan.go:774) — this must equal the existing role-binding name for the issue-loop App. deskkit's role-token table maps intake-desk to "issue-loop" (tools/desk/internal/deskkit/roletoken.go:46), and the house roster's role-bindings line binds issue-loop. All three copies are byte-identical.
RISK-VALUE: DERIVED — IssueLoopPubkeyVar = "ASSAY_ISSUE_LOOP_PUBKEY" @ tools/desk/internal/deskkit/verdict.go:108 (twin scanDeltaPubkeyVar @ statusgen/transcribescan.go:667) — this follows the VerifierPubkeyVar = "ASSAY_VERIFIER_PUBKEY" convention (verdict.go:515) and is byte-identical across the two independent modules.
RISK-VALUE: NAMED, NOT DERIVED — privKeyEnvForRole "ISSUE_LOOP_PEM" / privKeyFileForRole "issue-loop-app.pem" @ tools/desk/cmd/deskverdict/sign.go:329,336 (moved from lines 99 and 106 by #2362; values unchanged) — local custody lookup names. The operator-side config they must match lives outside this repo and could not be checked from here. A wrong value is reversible and fails closed.
| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test -run Verdict ./internal/deskkit/ && go test -run Canonical ./internal/deskkit/ && go test -run WrongKey ./internal/deskkit/ && go test -run Reflow ./internal/deskkit/ && go test -run DeriveAndParse ./internal/deskkit/ && go test -run PubkeyVarForRole ./internal/deskkit/ && go test -run Role ./internal/deskkit/` | pass exit=0 | sha256:5bfd5603074b | 2026-10-15 | assay-verifier-app[bot] @ bbf8e3516245 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskverdict/... -v` | pass exit=0 | sha256:1f53c368bdcd | 2026-10-15 | assay-verifier-app[bot] @ bbf8e3516245 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test -run ScanDelta . -v` | pass exit=0 | sha256:d25e78db5dd4 | 2026-10-15 | assay-verifier-app[bot] @ bbf8e3516245 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go test -run TranscribeScan . && go test -run Verdict . && go test -run PubkeyVar .` | pass exit=0 | sha256:74f742bf4af8 | 2026-10-15 | assay-verifier-app[bot] @ bbf8e3516245 (on-behalf-of human:ian) (forge-identity) |
| 5 | `test ! -e .github/verify && test -z "$(git ls-files "*.pem" "*.key" "*issue-loop-pubkey*")"` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-15 | assay-verifier-app[bot] @ bbf8e3516245 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && gofmt -l cmd/deskverdict/ internal/deskkit/verdict.go internal/deskkit/verdict_test.go && cd ../statusgen && gofmt -l transcribescan.go transcribeverdict.go transcribeverdict_test.go transcribescandelta_test.go main.go` | fail exit=1 | sha256:c5bc76e0d6b7 | 2026-10-15 | assay-verifier-app[bot] @ bbf8e3516245 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRoleMismatchRefusedBeforeCrypto$' -count=1 -v` | pass exit=0 | sha256:317bb3d42edc | 2026-10-15 | assay-verifier-app[bot] @ bbf8e3516245 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#3033 run=c58dacf9400b798b31bc84eea956e12d0a4373f4feaddbd0a965b6c7ab3607d9 source=bbf8e351624538c2a03159cd5a58116fb97fc293 brief=docs/streams/desk-tools/brief-28-deskverdict-role-keys-and-scan-delta-verify.md model=claude-opus-5-5 tier=any

### Non-implementer verifier re-run: 2026-10-15, assay-verifier-app[bot] (claude-opus-5-5), merged main bbf8e351624538c2a03159cd5a58116fb97fc293

Verification-Attestation binding (pre-work check exit 0, re-checked before return): medici-finance/assay#3033 run=c58dacf9400b798b31bc84eea956e12d0a4373f4feaddbd0a965b6c7ab3607d9 source=bbf8e351624538c2a03159cd5a58116fb97fc293 model=claude-opus-5-5 tier=any.

| # | Command | Expect | Observed (exit + key output line) | Date | Runner |
|---|---------|--------|-----------------------------------|------|--------|
| 1 | `cd tools/desk && go test -run Verdict ./internal/deskkit/ && go test -run Canonical ./internal/deskkit/ && go test -run WrongKey ./internal/deskkit/ && go test -run Reflow ./internal/deskkit/ && go test -run DeriveAndParse ./internal/deskkit/ && go test -run PubkeyVarForRole ./internal/deskkit/ && go test -run Role ./internal/deskkit/` | exit 0, pre-existing verdict tests plus the six role tests | exit 0 — seven `ok github.com/medici-finance/assay/tools/desk/internal/deskkit` lines; with -v, TestValidVerdictRole, TestPubkeyVarForRole, TestIssueLoopRoleRoundtrip, TestRoleMismatchRefusedBeforeCrypto, TestUnrecognizedRoleNeverFallsBackToVerifier, TestNoRoleFieldDefaultsToVerifier each `--- PASS` | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test ./cmd/deskverdict/... -v` | exit 0, pre-existing CLI tests plus the seven role CLI tests | exit 0 — 19 `--- PASS`, 0 `--- FAIL`; `ok github.com/medici-finance/assay/tools/desk/cmd/deskverdict`; all seven named TestCLI* role tests present and passing (the three extra tests since the last pass are TestSignBindingRefusesMismatchBeforeKey, TestSignPayloadDirSwappedAfterCheckIsRefused, TestSignPathHandlingNeverFollowsLinks) | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | `cd statusgen && go test -run ScanDelta . -v` | exit 0, 14 tests | exit 0 — 14 `--- PASS` (10 TestScanDelta*, 4 TestRunTranscribeScanDelta*), 0 `--- FAIL`; `ok github.com/medici-finance/assay/statusgen` | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | `cd statusgen && go test -run TranscribeScan . && go test -run Verdict . && go test -run PubkeyVar .` | exit 0 | exit 0 — three `ok github.com/medici-finance/assay/statusgen` lines | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | `test ! -e .github/verify && test -z "$(git ls-files "*.pem" "*.key" "*issue-loop-pubkey*")"` | exit 0 (negative path: no key material in tree) | exit 0 — empty output | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 6 | `cd tools/desk && gofmt -l cmd/deskverdict/ internal/deskkit/verdict.go internal/deskkit/verdict_test.go && cd ../statusgen && gofmt -l transcribescan.go transcribeverdict.go transcribeverdict_test.go transcribescandelta_test.go main.go` | exit 0, empty output | exit 1 — `cd: ../statusgen: No such file or directory` (the first gofmt half printed nothing; the second half never executed because the cd failed) | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRoleMismatchRefusedBeforeCrypto$' -count=1 -v` | exit 0 baseline; mutation reddens; restore returns exit 0 | exit 0 baseline — `--- PASS: TestRoleMismatchRefusedBeforeCrypto`. Mutation (3-line declared-role block, verdict.go lines 453-455, deleted in a scratch copy of the module): exit 1 — `verdict_test.go:368: role mismatch must be REFUSED even with a cryptographically valid signature, got 0 (verified: signature matches the canonical verdict payload)`. Restored (sha256 and cmp match the tree copy): exit 0 | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

Hand-run result: 6 of 7 rows pass; row 6 fails as written. The execution witness table above (statusgen verifyrun v1.0.42, non-dry, on the clean tree before this hand section was added) agrees: 6 of 7 pass, witness exit 1, row 6 fail exit=1 with output hash sha256:c5bc76e0d6b7, the same hash as every earlier pass.

Findings:

- The failure still reproduces on merged main bbf8e3516245. Row 6's command is unchanged: from tools/desk, the relative path ../statusgen resolves to tools/statusgen, which does not exist (statusgen lives at the repo root). #1735, which tracks this, is still OPEN.
- The property row 6 checks still holds. The same command with the path corrected to `cd ../../statusgen` exits 0 with empty output, so both halves are gofmt-clean. The defect is in the check definition, not the implementation. This corrected run is supplementary and does not replace the row as authored.
- Since dde4fbeaead9 (the 2026-10-07 head), the declared files that changed are statusgen/main.go, statusgen/transcribescan.go, statusgen/transcribeverdict.go and the deskverdict sign path (sign.go, main.go, plus new sign-path hardening files and tests). Rows 1-4 and 7, which exercise them, still pass; verdict.go is unchanged.
- Review note (b), re-checked at this head: the declared-role comparison in VerifyVerdictBodyForRole (tools/desk/internal/deskkit/verdict.go:453) comes before CanonicalizeJSON at line 456; in scanDeltaVerifyBody (statusgen/transcribescan.go:742) the role check at line 747 comes before the canonicaliser at 753 and the RSA verify at 762.
- Run conditions: every `go test` (hand-run and witness) ran with a throwaway HOME and a scratch temp directory, with the Go build and module caches at their normal locations. No row needed a cluster, a live service or another OS, and every row executed.

Risk-bearing values (all four risk answers are "no" and the item is not irreversible; enumeration over the delivering change's literals, re-read at this head):

RISK-VALUE: DERIVED — VerdictRoleIssueLoop = "issue-loop" @ tools/desk/internal/deskkit/verdict.go:93 (twin scanDeltaWantRole = "issue-loop" @ statusgen/transcribescan.go:671; roster key RoleBots["issue-loop"] @ statusgen/transcribescan.go:774) — this must equal the existing role-binding name for the intake App; deskkit's role-token table maps intake-desk to "issue-loop" (tools/desk/internal/deskkit/roletoken.go:46). All three copies are byte-identical.
RISK-VALUE: DERIVED — IssueLoopPubkeyVar = "ASSAY_ISSUE_LOOP_PUBKEY" @ tools/desk/internal/deskkit/verdict.go:108 (twin scanDeltaPubkeyVar @ statusgen/transcribescan.go:667) — this follows the VerifierPubkeyVar = "ASSAY_VERIFIER_PUBKEY" convention (verdict.go:515, statusgen/transcribeverdict.go:80) and is byte-identical across the two independent modules; a drift would put the consuming lane silently into could-not-check.
RISK-VALUE: NAMED, NOT DERIVED — privKeyEnvForRole "ISSUE_LOOP_PEM" / privKeyFileForRole "issue-loop-app.pem" @ tools/desk/cmd/deskverdict/sign.go:329,336 (moved from lines 99,106 by the sign-path hardening since the last pass; values unchanged) — local custody lookup names. The operator-side config they must match lives outside this repo and could not be checked from here. A wrong value is reversible and fails closed.
Ranked last (reversible operational knobs, no derivation needed): transcribeFloodThreshold = 25 @ statusgen/transcribescan.go:59; scanDeltaSchemaVersion = "scan-delta-v1" @ statusgen/transcribescan.go:658; CLI exit codes 5 (sign, unknown --key) and 6 (verify, unknown --key or no pubkey).

**VERIFY: FAIL** — rows 1–5 and 7 pass (the row 7 mutation was killed). Row 6 fails as written (exit 1) because its command changes into a directory that does not exist. This is a check-definition defect, unchanged since 2026-09-27 and tracked by #1735 (open).

### Non-implementer verifier re-run: 2026-10-21, assay-verifier-app[bot] (claude-opus-5-5), merged main 65a6418cd90c921153db509370212b1cdcd4f5e1

Verification-Attestation: medici-finance/assay#3887 run=79c7f17a993bedd31accda7c24cd9fdbb0617e6a0303fde0d7ff9e4a531bf27d source=65a6418cd90c921153db509370212b1cdcd4f5e1 brief=docs/streams/desk-tools/brief-28-deskverdict-role-keys-and-scan-delta-verify.md model=claude-opus-5-5 tier=strong

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd tools/desk && go test -run Verdict ./internal/deskkit/ && go test -run Canonical ./internal/deskkit/ && go test -run WrongKey ./internal/deskkit/ && go test -run Reflow ./internal/deskkit/ && go test -run DeriveAndParse ./internal/deskkit/ && go test -run PubkeyVarForRole ./internal/deskkit/ && go test -run Role ./internal/deskkit/` | exit 0, pre-existing verdict tests plus the six role tests | exit 0 — seven `ok github.com/medici-finance/assay/tools/desk/internal/deskkit` lines; a -v run of the six named role tests shows each `--- PASS` (TestValidVerdictRole, TestPubkeyVarForRole, TestIssueLoopRoleRoundtrip, TestRoleMismatchRefusedBeforeCrypto, TestUnrecognizedRoleNeverFallsBackToVerifier, TestNoRoleFieldDefaultsToVerifier) | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test ./cmd/deskverdict/... -v` | exit 0, pre-existing CLI tests plus the seven role CLI tests | exit 0 — 19 top-level `--- PASS`, 0 `--- FAIL`; `ok github.com/medici-finance/assay/tools/desk/cmd/deskverdict`; all seven named TestCLI* role tests present and passing | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | `cd statusgen && go test -run ScanDelta . -v` | exit 0, 14 tests | exit 0 — 14 `--- PASS` (10 TestScanDelta*, 4 TestRunTranscribeScanDelta*), 0 `--- FAIL`; `ok github.com/medici-finance/assay/statusgen` | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | `cd statusgen && go test -run TranscribeScan . && go test -run Verdict . && go test -run PubkeyVar .` | exit 0 | exit 0 — three `ok github.com/medici-finance/assay/statusgen` lines | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | `test ! -e .github/verify && test -z "$(git ls-files "*.pem" "*.key" "*issue-loop-pubkey*")"` | exit 0 (negative path: no key material in tree) | exit 0 — empty output | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 6 | `cd tools/desk && gofmt -l cmd/deskverdict/ internal/deskkit/verdict.go internal/deskkit/verdict_test.go && cd ../statusgen && gofmt -l transcribescan.go transcribeverdict.go transcribeverdict_test.go transcribescandelta_test.go main.go` | exit 0, empty output | exit 1 — `cd: no such file or directory: ../statusgen` (the first gofmt half printed nothing; the second half never ran because the cd failed) | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRoleMismatchRefusedBeforeCrypto$' -count=1 -v` | exit 0 baseline; mutation reddens; restore returns exit 0 | exit 0 baseline — `--- PASS: TestRoleMismatchRefusedBeforeCrypto`. Mutation (the 3-line declared-role block, verdict.go lines 453-455, deleted in a scratch copy of the module outside this tree): exit 1 — `verdict_test.go:368: role mismatch must be REFUSED even with a cryptographically valid signature, got 0 (verified: signature matches the canonical verdict payload)`. Restored (cmp identical to the tree copy): exit 0 | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Result: 6 of 7 rows pass; row 6 fails as written.

Findings:

- Row 6 still fails on merged main 65a6418cd90c. Its command is unchanged: from tools/desk, the relative path ../statusgen resolves to tools/statusgen, which does not exist (statusgen lives at the repo root). #1735, which tracks this check-definition defect, is still OPEN.
- The property row 6 checks still holds. A supplemental probe with the path corrected to `cd ../../statusgen` exits 0 with empty output, so both halves are gofmt-clean. This probe is diagnostic only and discharges no original row.
- Since bbf8e3516245 (the 2026-10-15 head), the declared files that changed are statusgen/main.go, statusgen/transcribescan.go, statusgen/transcribeverdict.go and tools/desk/cmd/deskverdict/main.go. Rows 1-4 and 7, which exercise them, still pass; verdict.go is unchanged.
- Review note (b), re-checked at this head: in VerifyVerdictBodyForRole (tools/desk/internal/deskkit/verdict.go:453) the declared-role comparison comes before CanonicalizeJSON at line 456; in scanDeltaVerifyBody (statusgen/transcribescan.go) the role check at line 777 comes before the canonicaliser at 783 and the RSA verify at 792.
- Run conditions: every go test ran with a throwaway HOME and scratch temp directory outside this tree, with -count=1; no row needed a cluster, a live service or another OS, and every row executed.

Risk-bearing values (all four risk answers are "no" and the item is not irreversible; enumeration over the delivering change's literals, re-read at this head):

RISK-VALUE: DERIVED — VerdictRoleIssueLoop = "issue-loop" @ tools/desk/internal/deskkit/verdict.go:93 (twin scanDeltaWantRole = "issue-loop" @ statusgen/transcribescan.go:701; roster key RoleBots["issue-loop"] @ statusgen/transcribescan.go:804) — must equal the existing role-binding name for the intake App; deskkit's role-token table maps intake-desk to "issue-loop" (tools/desk/internal/deskkit/roletoken.go:46). All three copies are byte-identical.
RISK-VALUE: DERIVED — IssueLoopPubkeyVar = "ASSAY_ISSUE_LOOP_PUBKEY" @ tools/desk/internal/deskkit/verdict.go:108 (twin scanDeltaPubkeyVar @ statusgen/transcribescan.go:697) — follows the VerifierPubkeyVar = "ASSAY_VERIFIER_PUBKEY" convention (verdict.go:515) and is byte-identical across the two independent modules; a drift would silently put the consuming lane into could-not-check.
RISK-VALUE: NAMED, NOT DERIVED — privKeyEnvForRole "ISSUE_LOOP_PEM" / privKeyFileForRole "issue-loop-app.pem" @ tools/desk/cmd/deskverdict/sign.go:329,336 — local custody lookup names; the operator-side config they must match lives outside this repo and could not be checked from here. A wrong value is reversible and fails closed.
Ranked last (reversible operational knobs, no derivation needed): transcribeFloodThreshold = 25 @ statusgen/transcribescan.go:59; scanDeltaSchemaVersion = "scan-delta-v1" @ statusgen/transcribescan.go:688; CLI exit codes 5 (sign, unknown --key) and 6 (verify, unknown --key or no pubkey).

**VERIFY: FAIL** — rows 1–5 and 7 pass (the row 7 mutation was killed). Row 6 fails as written (exit 1) because its command changes into a directory that does not exist. This is a check-definition defect, unchanged since 2026-09-27 and tracked by #1735 (open).

### Non-implementer verifier re-run: 2026-10-21 (second pass), assay-verifier-app[bot] (claude-opus-5-5), merged main af92fe827146129bc50af0df2ddb1cbc82fe0bfe

Verification-Attestation: medici-finance/assay#3983 run=ed4ebebbad98d8e1e6a76f5817b71bbb72fa81c97b65fb28c05739ad5f59f819 source=af92fe827146129bc50af0df2ddb1cbc82fe0bfe brief=docs/streams/desk-tools/brief-28-deskverdict-role-keys-and-scan-delta-verify.md model=claude-opus-5-5 tier=strong

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd tools/desk && go test -run Verdict ./internal/deskkit/ && go test -run Canonical ./internal/deskkit/ && go test -run WrongKey ./internal/deskkit/ && go test -run Reflow ./internal/deskkit/ && go test -run DeriveAndParse ./internal/deskkit/ && go test -run PubkeyVarForRole ./internal/deskkit/ && go test -run Role ./internal/deskkit/` | exit 0, pre-existing verdict tests plus the six role tests | exit 0 — seven `ok github.com/medici-finance/assay/tools/desk/internal/deskkit` lines; a -v run of the six named role tests shows each `--- PASS` (TestValidVerdictRole, TestPubkeyVarForRole, TestIssueLoopRoleRoundtrip, TestRoleMismatchRefusedBeforeCrypto, TestUnrecognizedRoleNeverFallsBackToVerifier, TestNoRoleFieldDefaultsToVerifier) | 2026-10-21 | assay-verifier-app[bot] @ af92fe827146 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test ./cmd/deskverdict/... -v` | exit 0, pre-existing CLI tests plus the seven role CLI tests | exit 0 — 19 top-level `--- PASS`, 0 `--- FAIL`; `ok github.com/medici-finance/assay/tools/desk/cmd/deskverdict`; all seven named TestCLI* role tests present and passing | 2026-10-21 | assay-verifier-app[bot] @ af92fe827146 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | `cd statusgen && go test -run ScanDelta . -v` | exit 0, 14 tests | exit 0 — 14 `--- PASS` (10 TestScanDelta*, 4 TestRunTranscribeScanDelta*), 0 `--- FAIL`; `ok github.com/medici-finance/assay/statusgen` | 2026-10-21 | assay-verifier-app[bot] @ af92fe827146 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | `cd statusgen && go test -run TranscribeScan . && go test -run Verdict . && go test -run PubkeyVar .` | exit 0 | exit 0 — three `ok github.com/medici-finance/assay/statusgen` lines | 2026-10-21 | assay-verifier-app[bot] @ af92fe827146 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | `test ! -e .github/verify && test -z "$(git ls-files "*.pem" "*.key" "*issue-loop-pubkey*")"` | exit 0 (negative path: no key material in tree) | exit 0 — empty output | 2026-10-21 | assay-verifier-app[bot] @ af92fe827146 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 6 | `cd tools/desk && gofmt -l cmd/deskverdict/ internal/deskkit/verdict.go internal/deskkit/verdict_test.go && cd ../statusgen && gofmt -l transcribescan.go transcribeverdict.go transcribeverdict_test.go transcribescandelta_test.go main.go` | exit 0, empty output | exit 1 — `cd: ../statusgen: No such file or directory` (the first gofmt half printed nothing; the second half never ran because the cd failed) | 2026-10-21 | assay-verifier-app[bot] @ af92fe827146 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRoleMismatchRefusedBeforeCrypto$' -count=1 -v` | exit 0 baseline; mutation reddens; restore returns exit 0 | exit 0 baseline — `--- PASS: TestRoleMismatchRefusedBeforeCrypto`. Mutation (the 3-line declared-role block, verdict.go lines 453-455, deleted in a throwaway copy of the tree inside the container): exit 1 — `verdict_test.go:368: role mismatch must be REFUSED even with a cryptographically valid signature, got 0 (verified: signature matches the canonical verdict payload)`. Restored (cmp identical to the tree copy): exit 0 | 2026-10-21 | assay-verifier-app[bot] @ af92fe827146 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Result: 6 of 7 rows pass; row 6 fails as written.

Findings:

- Row 6 still fails on merged main af92fe827146 with the same unchanged command: from tools/desk, ../statusgen resolves to tools/statusgen, which does not exist (statusgen is at the repo root). #1735, which tracks this check-definition defect, is still OPEN.
- The property row 6 checks still holds. A supplemental probe with the path corrected to `cd ../../statusgen` exits 0 with empty output. The probe is diagnostic only and discharges no original row.
- No declared file (verdict.go, verdict_test.go, cmd/deskverdict, statusgen transcribescan.go, transcribeverdict.go, transcribeverdict_test.go, transcribescandelta_test.go, main.go, testdata/scan-delta-payload.json) changed between 65a6418cd90c (the previous pass) and af92fe827146.
- Review note (b), re-checked at this head: in VerifyVerdictBodyForRole (tools/desk/internal/deskkit/verdict.go:445) the declared-role comparison at line 453 comes before CanonicalizeJSON at 456 and the signature verify at 460; in scanDeltaVerifyBody (statusgen/transcribescan.go:772) the role check at 777 comes before the canonicaliser at 783 and the RSA verify at 792.
- Run conditions: every row ran in the assay-verify:go1.27-py image with `--network none`, as uid 65534 (nobody, passwd entry present), with GOPROXY=off and a read-only module cache mount, against a full-history local clone at af92fe827146 whose tree hash (dfd9dc58bdfc) and file contents match the admitted home exactly. A throwaway HOME, GOCACHE and temp directory lived inside the container; tests used -count=1. No row needed a cluster or live service, and every row executed.

Risk-bearing values (all four risk answers are "no" and the item is not irreversible; enumeration covers the delivering change's literals, re-read at this head):

RISK-VALUE: DERIVED — VerdictRoleIssueLoop = "issue-loop" @ tools/desk/internal/deskkit/verdict.go:93 (twin scanDeltaWantRole = "issue-loop" @ statusgen/transcribescan.go:701; roster key RoleBots["issue-loop"] @ statusgen/transcribescan.go:804) — must equal the existing role-binding name for the intake App; deskkit's role-token table maps intake-desk to "issue-loop" (tools/desk/internal/deskkit/roletoken.go:46). All three copies are byte-identical.
RISK-VALUE: DERIVED — IssueLoopPubkeyVar = "ASSAY_ISSUE_LOOP_PUBKEY" @ tools/desk/internal/deskkit/verdict.go:108 (twin scanDeltaPubkeyVar @ statusgen/transcribescan.go:697) — follows the VerifierPubkeyVar = "ASSAY_VERIFIER_PUBKEY" convention (verdict.go:515) and is byte-identical across the two independent modules; a drift would silently put the consuming lane into could-not-check.
RISK-VALUE: NAMED, NOT DERIVED — privKeyEnvForRole "ISSUE_LOOP_PEM" / privKeyFileForRole "issue-loop-app.pem" @ tools/desk/cmd/deskverdict/sign.go:329,336 — local custody lookup names; the operator-side config they must match lives outside this repo and could not be checked from here. A wrong value is reversible and fails closed.
Ranked last (reversible operational knobs, no derivation needed): transcribeFloodThreshold = 25 @ statusgen/transcribescan.go:59; scanDeltaSchemaVersion = "scan-delta-v1" @ statusgen/transcribescan.go:688; CLI exit codes 5 (sign, unknown --key) and 6 (verify, unknown --key or no pubkey).

**VERIFY: FAIL** — rows 1–5 and 7 pass (the row 7 mutation was killed). Row 6 fails as written (exit 1) because its command changes into a directory that does not exist. This is a check-definition defect, unchanged since 2026-09-27 and tracked by #1735 (open).

### Non-implementer verifier re-run: 2026-10-22, merged main 05dc9fbc88f2b1b23cd78bebc8634c564327eca4

Execution witness (statusgen verifyrun, write mode, run from the admitted home on a clean tree at the source commit):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test -run Verdict ./internal/deskkit/ && go test -run Canonical ./internal/deskkit/ && go test -run WrongKey ./internal/deskkit/ && go test -run Reflow ./internal/deskkit/ && go test -run DeriveAndParse ./internal/deskkit/ && go test -run PubkeyVarForRole ./internal/deskkit/ && go test -run Role ./internal/deskkit/` | pass exit=0 | sha256:191a86c08a6f | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskverdict/... -v` | pass exit=0 | sha256:da92949500f6 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test -run ScanDelta . -v` | pass exit=0 | sha256:f33aac4e97ab | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go test -run TranscribeScan . && go test -run Verdict . && go test -run PubkeyVar .` | pass exit=0 | sha256:c3b83da96575 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 5 | `test ! -e .github/verify && test -z "$(git ls-files "*.pem" "*.key" "*issue-loop-pubkey*")"` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && gofmt -l cmd/deskverdict/ internal/deskkit/verdict.go internal/deskkit/verdict_test.go && cd ../statusgen && gofmt -l transcribescan.go transcribeverdict.go transcribeverdict_test.go transcribescandelta_test.go main.go` | fail exit=1 | sha256:c5bc76e0d6b7 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRoleMismatchRefusedBeforeCrypto$' -count=1 -v` | pass exit=0 | sha256:22143e437f06 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4046 run=4ad44be38f1212fd8d86b5ff04406c5b7e230bff5bc3d7e5c87f9c356eb6f1c6 source=05dc9fbc88f2b1b23cd78bebc8634c564327eca4 brief=docs/streams/desk-tools/brief-28-deskverdict-role-keys-and-scan-delta-verify.md model=claude-opus-5-5 tier=strong

Result: 6 of 7 rows pass; row 6 fails as written. All seven rows rest on the witness table above. The hand runs below add detail and the row 7 mutation; they are not a second acceptance record. The witness ran on the host with GOFLAGS=-count=1, so no test result came from the cache, and with its temp, build cache and module directories outside this tree. No row needed a cluster or a live service.

Hand runs (prose only): a fresh scratch clone at 05dc9fbc88f2 (tree 2320a5cb5fcb, identical to the admitted home) ran inside the assay-verify:go1.27-py image with `--network none`, as uid 65534, with GOPROXY=off, a read-only module cache, and a throwaway HOME, GOCACHE and temp directory inside the container. Tests used -count=1.

- Row 1: a -v run of the six named role tests shows each `--- PASS` (TestValidVerdictRole, TestPubkeyVarForRole, TestIssueLoopRoleRoundtrip, TestRoleMismatchRefusedBeforeCrypto, TestUnrecognizedRoleNeverFallsBackToVerifier, TestNoRoleFieldDefaultsToVerifier).
- Row 2: exit 0, 19 top-level `--- PASS`, 0 `--- FAIL`, and `ok github.com/medici-finance/assay/tools/desk/cmd/deskverdict`.
- Row 3: exit 0, 14 `--- PASS`, 0 `--- FAIL`, and `ok github.com/medici-finance/assay/statusgen`. That matches the 14 tests the Expect names.
- Row 6 as written: exit 1, `cd: ../statusgen: No such file or directory`. The first gofmt half printed nothing; the second half never ran. From tools/desk, ../statusgen resolves to tools/statusgen, which does not exist, because statusgen sits at the repo root. #1735, which tracks this check-definition defect, is still OPEN.
- Row 6 supplemental probe, with the path corrected to `cd ../../statusgen`: exit 0 and empty output, so both halves are gofmt-clean. This probe only diagnoses the failure and discharges no original row.
- Row 7 mutation: the baseline exits 0 with `--- PASS: TestRoleMismatchRefusedBeforeCrypto`. The mutation deleted the 3-line declared-role block (verdict.go lines 453-455) in a throwaway copy inside the container. The test then exits 1 with `verdict_test.go:368: role mismatch must be REFUSED even with a cryptographically valid signature, got 0 (verified: signature matches the canonical verdict payload)`. After the file was restored (cmp identical to the clone), the test exits 0 again, so the mutation was killed.

Findings:

- Between af92fe827146 (the previous pass) and 05dc9fbc88f2, the only declared file that changed is statusgen/main.go. The change adds a `context` subcommand dispatch and its name in the known-subcommands list, and does not touch the transcribe-scan or verdict paths. Rows 3 and 4 still pass.
- Review note (b), re-checked at this head: in VerifyVerdictBodyForRole (tools/desk/internal/deskkit/verdict.go:445), the declared-role comparison at line 453 comes before CanonicalizeJSON at line 456. In scanDeltaVerifyBody (statusgen/transcribescan.go:772), the role check at line 777 comes before the canonicaliser at line 783 and the RSA verify at line 792.
- Review note (c): row 5 exits 0 with empty output, so no key material is in the tree.

Risk-bearing values: all four risk answers are "no" and the item is not irreversible. The enumeration covers the literals in the delivering change, re-read at this head.

RISK-VALUE: DERIVED — VerdictRoleIssueLoop = "issue-loop" @ tools/desk/internal/deskkit/verdict.go:93 (twin scanDeltaWantRole = "issue-loop" @ statusgen/transcribescan.go:701; roster key RoleBots["issue-loop"] @ statusgen/transcribescan.go:804) — it must equal the existing role-binding name for the intake App, and deskkit's role-token table maps intake-desk to "issue-loop" (tools/desk/internal/deskkit/roletoken.go:46). All three copies are byte-identical.
RISK-VALUE: DERIVED — IssueLoopPubkeyVar = "ASSAY_ISSUE_LOOP_PUBKEY" @ tools/desk/internal/deskkit/verdict.go:108 (twin scanDeltaPubkeyVar @ statusgen/transcribescan.go:697) — it follows the VerifierPubkeyVar = "ASSAY_VERIFIER_PUBKEY" convention (verdict.go:515) and is byte-identical across the two independent modules. If the two drifted, the consuming lane would silently fall into could-not-check.
RISK-VALUE: NAMED, NOT DERIVED — privKeyEnvForRole "ISSUE_LOOP_PEM" / privKeyFileForRole "issue-loop-app.pem" @ tools/desk/cmd/deskverdict/sign.go:329,336 — these are local custody lookup names. The operator-side config they must match lives outside this repo, so it could not be checked from here. A wrong value is reversible and fails closed. Question filed: #1928.
Ranked last (reversible operational knobs, no derivation needed): transcribeFloodThreshold = 25 @ statusgen/transcribescan.go:59; scanDeltaSchemaVersion = "scan-delta-v1" @ statusgen/transcribescan.go:688; CLI exit codes 5 (sign, unknown --key) and 6 (verify, unknown --key or no pubkey).

**VERIFY: FAIL** — rows 1–5 and 7 pass (the row 7 mutation was killed). Row 6 fails as written (exit 1) because its command changes into a directory that does not exist. This is a check-definition defect, unchanged since 2026-09-27 and tracked by #1735 (open).
### Non-implementer verifier re-run: 2026-10-27, merged main 0f2b208cebc30af31269e7482b8b9159b602867d

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test -run Verdict ./internal/deskkit/ && go test -run Canonical ./internal/deskkit/ && go test -run WrongKey ./internal/deskkit/ && go test -run Reflow ./internal/deskkit/ && go test -run DeriveAndParse ./internal/deskkit/ && go test -run PubkeyVarForRole ./internal/deskkit/ && go test -run Role ./internal/deskkit/` | pass exit=0 | sha256:58b850dce031 | 2026-10-27 | assay-verifier-app[bot] @ 0f2b208cebc3 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskverdict/... -v` | pass exit=0 | sha256:0e49a33fd731 | 2026-10-27 | assay-verifier-app[bot] @ 0f2b208cebc3 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test -run ScanDelta . -v` | pass exit=0 | sha256:cf92859670cf | 2026-10-27 | assay-verifier-app[bot] @ 0f2b208cebc3 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go test -run TranscribeScan . && go test -run Verdict . && go test -run PubkeyVar .` | pass exit=0 | sha256:e98067d2bc35 | 2026-10-27 | assay-verifier-app[bot] @ 0f2b208cebc3 (on-behalf-of human:ian) (forge-identity) |
| 5 | `test ! -e .github/verify && test -z "$(git ls-files "*.pem" "*.key" "*issue-loop-pubkey*")"` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 0f2b208cebc3 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && gofmt -l cmd/deskverdict/ internal/deskkit/verdict.go internal/deskkit/verdict_test.go && cd ../statusgen && gofmt -l transcribescan.go transcribeverdict.go transcribeverdict_test.go transcribescandelta_test.go main.go` | fail exit=1 | sha256:c5bc76e0d6b7 | 2026-10-27 | assay-verifier-app[bot] @ 0f2b208cebc3 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRoleMismatchRefusedBeforeCrypto$' -count=1 -v` | pass exit=0 | sha256:4265621bb902 | 2026-10-27 | assay-verifier-app[bot] @ 0f2b208cebc3 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4673 run=fe08fdeeaf4b82f9a09ba9ab1c5a087405ee4aae65f997e26d291012224ae9f7 source=0f2b208cebc30af31269e7482b8b9159b602867d brief=docs/streams/desk-tools/brief-28-deskverdict-role-keys-and-scan-delta-verify.md model=claude-opus-5-5 tier=strong

Witness: the table and attestation line above are the verbatim output of `statusgen verifyrun --dry-run` (v1.0.63) at merged main 0f2b208cebc3, run by a dispatched non-implementer verifier (claude-opus-5-5). Fail-first: no row is risk-bearing (all four brief-wide risk answers are "no", and no row's Command or Expect carries a risk-bearing, live, mutating or end-to-end tag), so no Base cell is owed. The delivering change is the squash merge daed69a89fa5 (#1325); its parent f645d2fcb84d does not hold this brief, and verifyrun refuses it as a base for that reason.

Run conditions: the witness run used the operator's normal HOME, because the installed verifyrun's pre-work admission step mints an App token and a credential-free HOME cannot. Every sign call in the deskverdict CLI tests passes an explicit temporary key (--pem or ISSUE_LOOP_PEM), so the config-directory key fallback is never reached there. The supplemental probes below ran in a scratch copy of the tree at this commit, with a throwaway HOME and the real Go build and module caches. No row needed a cluster or a live service.

Supplemental observations (diagnostic only, no acceptance credit):

- Row 1: a -v run of the six named role tests shows each `--- PASS` (TestValidVerdictRole, TestPubkeyVarForRole, TestIssueLoopRoleRoundtrip, TestRoleMismatchRefusedBeforeCrypto, TestUnrecognizedRoleNeverFallsBackToVerifier, TestNoRoleFieldDefaultsToVerifier).
- Row 2: exit 0, 19 top-level `--- PASS`, 0 `--- FAIL`. All seven named TestCLI* role tests are present and pass.
- Row 3: exit 0, 14 `--- PASS` (10 TestScanDelta*, 4 TestRunTranscribeScanDelta*), 0 `--- FAIL`, which matches the 14 the Expect names.
- Row 6 as written: exit 1, `cd: ../statusgen: No such file or directory`. The first gofmt half printed nothing and the second half never ran. From tools/desk, ../statusgen resolves to tools/statusgen, which does not exist; statusgen sits at the repo root. With the path corrected to ../../statusgen the same command exits 0 with empty output, so both halves are gofmt-clean. This is the check-definition defect tracked by #1735, which is still open.
- Row 7 mutation: the Expect names an `extractDeclaredRole(body)` block inside VerifyVerdictBodyForRole in tools/desk/internal/deskkit/verdict.go. At this commit that function is a two-line wrapper over the shared deskcore/verdict package (moved by #4445), and the equivalent block is `if declared := DeclaredRole(body); declared != wantRole { return VerdictRefused, … }` at deskcore/verdict/verdict.go:233-235. In a scratch copy, the baseline exits 0. Deleting those three lines makes the test exit 1 with `verdict_test.go:368: role mismatch must be REFUSED even with a cryptographically valid signature, got 0 (verified: signature matches the canonical verdict payload)`. After the file was restored (cmp identical), the test exits 0 again, so the mutation was killed. The row's mutation text now points at a file that no longer holds the block.

Findings:

- Review note (b), re-checked at this head: in deskcore/verdict/verdict.go VerifyBodyForRole (line 225), the declared-role comparison at line 233 comes before CanonicalizeJSON at line 236 and VerifyCanonical at line 240. In statusgen/transcribescan.go scanDeltaVerifyBody (line 763), the role check at line 767 comes before the call into the shared VerifyBodyForRole at line 773.
- The brief's fact that statusgen's verdict crypto is an independently duplicated copy no longer holds. Since #4445, both modules call the one deskcore/verdict implementation. This removes the drift risk that fact described; it does not change any row.
- Review note (c): row 5 exits 0 with empty output, so no key material is tracked in the tree.

Risk-bearing values: all four risk answers are "no" and the item is not irreversible. The enumeration covers the literals in the delivering change, re-read at this head.

RISK-VALUE: DERIVED — VerdictRoleIssueLoop = "issue-loop" @ deskcore/verdict/verdict.go:26 (re-exported at tools/desk/internal/deskkit/verdict.go:89; twin scanDeltaWantRole = "issue-loop" @ statusgen/transcribescan.go:698; roster key RoleBots["issue-loop"] @ statusgen/transcribescan.go:783) — it must equal the existing role-binding name for the intake App, and deskkit's role-token table maps intake-desk to "issue-loop" (tools/desk/internal/deskkit/roletoken.go:46). All copies are byte-identical.
RISK-VALUE: DERIVED — IssueLoopPubkeyVar = "ASSAY_ISSUE_LOOP_PUBKEY" @ tools/desk/internal/deskkit/verdict.go:104 (twin scanDeltaPubkeyVar @ statusgen/transcribescan.go:694) — it follows the VerifierPubkeyVar = "ASSAY_VERIFIER_PUBKEY" convention (tools/desk/internal/deskkit/verdict.go:310, statusgen/transcribeverdict.go:74) and is byte-identical across the two modules. If the two drifted, the consuming lane would silently fall into could-not-check.
RISK-VALUE: NAMED, NOT DERIVED — privKeyEnvForRole "ISSUE_LOOP_PEM" / privKeyFileForRole "issue-loop-app.pem" @ tools/desk/cmd/deskverdict/sign.go:329,336 — these are local custody lookup names. The operator-side config they must match lives outside this repo, so it could not be checked from here. A wrong value is reversible and fails closed. Question filed earlier: #1928.
Ranked last (reversible operational knobs, no derivation needed): transcribeFloodThreshold = 25 @ statusgen/transcribescan.go:56; scanDeltaSchemaVersion = "scan-delta-v1" @ statusgen/transcribescan.go:685; CLI exit codes 5 (sign, unknown --key) and 6 (verify, unknown --key or no pubkey).

**VERIFY: FAIL** — rows 1–5 and 7 pass (the row 7 mutation was killed at its moved location). Row 6 fails as written (exit 1) because its command changes into a directory that does not exist. This is a check-definition defect, unchanged since 2026-09-27 and tracked by #1735 (open).
### Non-implementer verifier re-run: 2026-10-29, merged main 1304d40f5c32cae5136e87f6b1f0cbe5a008c72a

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test -run Verdict ./internal/deskkit/ && go test -run Canonical ./internal/deskkit/ && go test -run WrongKey ./internal/deskkit/ && go test -run Reflow ./internal/deskkit/ && go test -run DeriveAndParse ./internal/deskkit/ && go test -run PubkeyVarForRole ./internal/deskkit/ && go test -run Role ./internal/deskkit/` | pass exit=0 | sha256:da919e3ce930 | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskverdict/... -v` | pass exit=0 | sha256:841bf286b766 | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test -run ScanDelta . -v` | pass exit=0 | sha256:423bc8924b2d | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go test -run TranscribeScan . && go test -run Verdict . && go test -run PubkeyVar .` | pass exit=0 | sha256:da2d2692cbc0 | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 5 | `test ! -e .github/verify && test -z "$(git ls-files "*.pem" "*.key" "*issue-loop-pubkey*")"` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && gofmt -l cmd/deskverdict/ internal/deskkit/verdict.go internal/deskkit/verdict_test.go && cd ../statusgen && gofmt -l transcribescan.go transcribeverdict.go transcribeverdict_test.go transcribescandelta_test.go main.go` | fail exit=1 | sha256:c5bc76e0d6b7 | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRoleMismatchRefusedBeforeCrypto$' -count=1 -v` | pass exit=0 | sha256:ffea44c42fa4 | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4895 run=686facfd5be06b424bfd9bc241ee95a010cddd7df8cdc69f3c2fc59ee5c21c8c source=1304d40f5c32cae5136e87f6b1f0cbe5a008c72a brief=docs/streams/desk-tools/brief-28-deskverdict-role-keys-and-scan-delta-verify.md model=claude-opus-5-5 tier=strong

Witness: the table and attestation line above are the verbatim output of `statusgen verifyrun` (v1.0.65, write mode) at merged main 1304d40f5c32, run from the admitted home on a clean tree by a dispatched non-implementer verifier (claude-opus-5-5). The run used GOFLAGS=-count=1, so no test result came from the cache, and its temp directory sat outside this tree. No row needed a cluster or a live service. Fail-first: no row is risk-bearing (all four brief-wide risk answers are "no", and no row's Command or Expect carries a risk-bearing, live, mutating or end-to-end tag), so no Base cell is owed.

Supplemental observations (diagnostic only, no acceptance credit), from an archive of the tree at 1304d40f5c32 extracted to scratch, with GOPROXY=off and -count=1:

- Row 1: a -v run of the six named role tests shows each `--- PASS` (TestValidVerdictRole, TestPubkeyVarForRole, TestIssueLoopRoleRoundtrip, TestRoleMismatchRefusedBeforeCrypto, TestUnrecognizedRoleNeverFallsBackToVerifier, TestNoRoleFieldDefaultsToVerifier).
- Row 2: exit 0, 19 top-level `--- PASS`, 0 `--- FAIL`. All seven named TestCLI* role tests are present and pass.
- Row 3: exit 0, 14 `--- PASS` (10 TestScanDelta*, 4 TestRunTranscribeScanDelta*), 0 `--- FAIL`, which matches the 14 the Expect names.
- Row 6 as written: exit 1, `cd: ../statusgen: No such file or directory`. The first gofmt half printed nothing and the second half never ran. From tools/desk, ../statusgen resolves to tools/statusgen, which does not exist; statusgen sits at the repo root. With the path corrected to ../../statusgen the same command exits 0 with empty output, so both halves are gofmt-clean. This is the check-definition defect tracked by #1735, which is still open.
- Row 7 mutation: VerifyVerdictBodyForRole in tools/desk/internal/deskkit/verdict.go is still a thin wrapper over the shared deskcore/verdict package, and the declared-role block the Expect names is `if declared := DeclaredRole(body); declared != wantRole { return VerdictRefused, … }` at deskcore/verdict/verdict.go:233-235. In the scratch copy the baseline exits 0. Deleting those three lines makes the test exit 1 with `verdict_test.go:368: role mismatch must be REFUSED even with a cryptographically valid signature, got 0 (verified: signature matches the canonical verdict payload)`. After the file was restored (cmp identical to the commit's blob) the test exits 0 again, so the mutation was killed.

Findings:

- Between 0f2b208cebc3 (the previous pass) and 1304d40f5c32, the only declared file that changed is statusgen/main.go. The change adds a witness-inputs lint call and a `witnessdigest` subcommand dispatch, and does not touch the transcribe-scan or verdict paths. deskcore/verdict is unchanged. Rows 3 and 4 still pass.
- Review note (b), re-checked at this head: in deskcore/verdict/verdict.go VerifyBodyForRole (line 225), the declared-role comparison at line 233 comes before CanonicalizeJSON at line 236 and VerifyCanonical at line 240. In statusgen/transcribescan.go scanDeltaVerifyBody (line 763), the role check at line 767 comes before the call into the shared VerifyBodyForRole at line 773.
- Review note (c): row 5 exits 0 with empty output, so no key material is tracked in the tree.

Risk-bearing values: all four risk answers are "no" and the item is not irreversible. The enumeration covers the literals in the delivering change, re-read at this head; every line number is unchanged from the previous pass.

RISK-VALUE: DERIVED — VerdictRoleIssueLoop = "issue-loop" @ deskcore/verdict/verdict.go:26 (re-exported at tools/desk/internal/deskkit/verdict.go:89; twin scanDeltaWantRole = "issue-loop" @ statusgen/transcribescan.go:698; roster key RoleBots["issue-loop"] @ statusgen/transcribescan.go:783) — it must equal the existing role-binding name for the intake App, and deskkit's role-token table maps intake-desk to "issue-loop" (tools/desk/internal/deskkit/roletoken.go:46). All copies are byte-identical.
RISK-VALUE: DERIVED — IssueLoopPubkeyVar = "ASSAY_ISSUE_LOOP_PUBKEY" @ tools/desk/internal/deskkit/verdict.go:104 (twin scanDeltaPubkeyVar @ statusgen/transcribescan.go:694) — it follows the VerifierPubkeyVar = "ASSAY_VERIFIER_PUBKEY" convention (tools/desk/internal/deskkit/verdict.go:310, statusgen/transcribeverdict.go:74) and is byte-identical across the two modules. If the two drifted, the consuming lane would silently fall into could-not-check.
RISK-VALUE: NAMED, NOT DERIVED — privKeyEnvForRole "ISSUE_LOOP_PEM" / privKeyFileForRole "issue-loop-app.pem" @ tools/desk/cmd/deskverdict/sign.go:329,336 — these are local custody lookup names. The operator-side config they must match lives outside this repo, so it could not be checked from here. A wrong value is reversible and fails closed. Question filed earlier: #1928 (still open).
Ranked last (reversible operational knobs, no derivation needed): transcribeFloodThreshold = 25 @ statusgen/transcribescan.go:56; scanDeltaSchemaVersion = "scan-delta-v1" @ statusgen/transcribescan.go:685; CLI exit codes 5 (sign, unknown --key) and 6 (verify, unknown --key or no pubkey).

**VERIFY: FAIL** — rows 1–5 and 7 pass (the row 7 mutation was killed). Row 6 fails as written (exit 1) because its command changes into a directory that does not exist. This is a check-definition defect, unchanged since 2026-09-27 and tracked by #1735 (open).
**VERIFY: FAIL** — rows 1–5 and 7 pass (the row 7 mutation was killed at its moved location). Row 6 fails as written (exit 1) because its command changes into a directory that does not exist. This is a check-definition defect, unchanged since 2026-09-27 and tracked by #1735 (open).

### Non-implementer verifier re-run: 2026-10-30, merged main 40925e8233c8a63d70682e77ca333e665f1f955e

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test -run Verdict ./internal/deskkit/ && go test -run Canonical ./internal/deskkit/ && go test -run WrongKey ./internal/deskkit/ && go test -run Reflow ./internal/deskkit/ && go test -run DeriveAndParse ./internal/deskkit/ && go test -run PubkeyVarForRole ./internal/deskkit/ && go test -run Role ./internal/deskkit/` | pass exit=0 | sha256:2a3966a73416 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskverdict/... -v` | pass exit=0 | sha256:315bcfed837c | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test -run ScanDelta . -v` | pass exit=0 | sha256:372e8f2e67a4 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go test -run TranscribeScan . && go test -run Verdict . && go test -run PubkeyVar .` | pass exit=0 | sha256:373ac6fad7ae | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 5 | `test ! -e .github/verify && test -z "$(git ls-files "*.pem" "*.key" "*issue-loop-pubkey*")"` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && gofmt -l cmd/deskverdict/ internal/deskkit/verdict.go internal/deskkit/verdict_test.go && cd ../statusgen && gofmt -l transcribescan.go transcribeverdict.go transcribeverdict_test.go transcribescandelta_test.go main.go` | fail exit=1 | sha256:c5bc76e0d6b7 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRoleMismatchRefusedBeforeCrypto$' -count=1 -v` | pass exit=0 | sha256:cf139e4616ca | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4956 run=5ef1a50d5c36c4adf762c77da251b0362df09b8b7538710d857f2c1ab690b7e8 source=40925e8233c8a63d70682e77ca333e665f1f955e brief=docs/streams/desk-tools/brief-28-deskverdict-role-keys-and-scan-delta-verify.md model=claude-opus-5-5 tier=strong

Witness: the table and attestation line above are the verbatim output of `statusgen verifyrun` (v1.0.66, write mode) at merged main 40925e8233c8, run on the host from the admitted home on a clean tree by a dispatched non-implementer verifier (claude-opus-5-5), after `deskdispatch --check-verifier` admitted the run (attestation #4956). The run used GOFLAGS=-count=1, so no test result came from the cache, with Go's temp directory outside the tree and no GH_TOKEN in its environment. No row needed a cluster or a live service. Fail-first: no row is risk-bearing (all four risk answers are "no", and no row's Command or Expect carries a risk-bearing, live, mutating or end-to-end tag), so no Base cell is owed.

Supplemental observations (diagnostic only, no acceptance credit), taken in a scratch extract of the tree at 40925e8233c8 with -count=1:

- **Row 1** (2026-10-30): a -v run of the six named role tests — expected each to pass; observed `--- PASS` for TestValidVerdictRole, TestPubkeyVarForRole, TestIssueLoopRoleRoundtrip, TestRoleMismatchRefusedBeforeCrypto, TestUnrecognizedRoleNeverFallsBackToVerifier and TestNoRoleFieldDefaultsToVerifier.
- **Row 2** (2026-10-30): the row's command with its -v output counted — expected the seven named TestCLI* role tests to pass; observed exit 0, 19 top-level `--- PASS`, 0 `--- FAIL`, all seven named tests among them.
- **Row 3** (2026-10-30): the row's command with its -v output counted — expected 14 tests; observed exit 0, 14 `--- PASS` (10 TestScanDelta*, 4 TestRunTranscribeScanDelta*), 0 `--- FAIL`.
- **Row 6** (2026-10-30): the row's command as written — expected exit 0 and empty output; observed exit 1, `cd: no such file or directory: ../statusgen`. The first gofmt half printed nothing and the second half never ran. From tools/desk the relative path resolves to tools/statusgen, which does not exist; statusgen sits at the repo root. With the path corrected to ../../statusgen the same command exits 0 with empty output, so both halves are gofmt-clean. This corrected-path probe only diagnoses the failure and discharges no original row. The check-definition defect is tracked by #1735, still OPEN.
- **Row 7 mutation** (2026-10-30): the Expect names a block inside VerifyVerdictBodyForRole in the deskkit verdict.go; at this commit that function is a wrapper over the shared deskcore/verdict package, and the equivalent block is `if declared := DeclaredRole(body); declared != wantRole { return VerdictRefused, … }` at deskcore/verdict/verdict.go:233-235 — expected the test to redden with that block deleted; observed baseline exit 0, mutated exit 1 with `verdict_test.go:368: role mismatch must be REFUSED even with a cryptographically valid signature, got 0 (verified: signature matches the canonical verdict payload)`, and exit 0 again after the file was restored (cmp identical to the commit's blob). The mutation was killed. The row's mutation text still points at a file that no longer holds the block.

Findings:

- Between 0f2b208cebc3 (the last landed pass) and 40925e8233c8, the only declared file that changed is statusgen/main.go: a `witnessdigest` subcommand dispatch, a witness-inputs lint hook, and the queue row type moved into a verifyqueue package. None of it touches the transcribe-scan or verdict paths; rows 3 and 4 still pass.
- Review note (b), re-checked at this head: in deskcore/verdict/verdict.go VerifyBodyForRole (line 225), the declared-role comparison at line 233 comes before CanonicalizeJSON at line 236 and VerifyCanonical at line 240. In statusgen/transcribescan.go scanDeltaVerifyBody (line 763), the role check at line 767 comes before the call into the shared VerifyBodyForRole at line 773.
- Review note (c): row 5 exits 0 with empty output, so no key material is tracked in the tree.

Risk-bearing values: all four risk answers are "no" and the item is not irreversible. The enumeration covers the literals in the delivering change (#1325), re-read at this head.

RISK-VALUE: DERIVED — VerdictRoleIssueLoop = "issue-loop" @ deskcore/verdict/verdict.go:26 (re-exported at tools/desk/internal/deskkit/verdict.go:89; twin scanDeltaWantRole = "issue-loop" @ statusgen/transcribescan.go:698; roster key RoleBots["issue-loop"] @ statusgen/transcribescan.go:783) — it must equal the existing role-binding name for the intake App, and deskkit's role-token table maps intake-desk to "issue-loop" (tools/desk/internal/deskkit/roletoken.go:46). All copies are byte-identical.
RISK-VALUE: DERIVED — IssueLoopPubkeyVar = "ASSAY_ISSUE_LOOP_PUBKEY" @ tools/desk/internal/deskkit/verdict.go:104 (twin scanDeltaPubkeyVar @ statusgen/transcribescan.go:694) — it follows the VerifierPubkeyVar = "ASSAY_VERIFIER_PUBKEY" convention (tools/desk/internal/deskkit/verdict.go:310, statusgen/transcribeverdict.go:74) and is byte-identical across the two modules. If the two drifted, the consuming lane would silently fall into could-not-check.
RISK-VALUE: NAMED, NOT DERIVED — privKeyEnvForRole "ISSUE_LOOP_PEM" / privKeyFileForRole "issue-loop-app.pem" @ tools/desk/cmd/deskverdict/sign.go:329,336 — these are local custody lookup names. The operator-side config they must match lives outside this repo, so it could not be checked from here. A wrong value is reversible and fails closed. Question filed earlier: #1928 (open).
Ranked last (reversible operational knobs, no derivation needed): transcribeFloodThreshold = 25 @ statusgen/transcribescan.go:56; scanDeltaSchemaVersion = "scan-delta-v1" @ statusgen/transcribescan.go:685; CLI exit codes 5 (sign, unknown --key) and 6 (verify, unknown --key or no pubkey).

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in
the stream README table. Review note: confirm (a) every pre-existing verdict/scan/verify test
still passes byte-for-byte unmodified in assertion, (b) the declared-role check really runs
BEFORE the crypto check in `VerifyVerdictBodyForRole` and `scanDeltaVerifyBody` (not just
documented as doing so), and (c) row 5 — no key material anywhere in the diff.
