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
files: `tools/desk/internal/deskkit/verdict.go`, `tools/desk/cmd/deskverdict/{main,sign,verify,
keygen}.go`, `statusgen/{transcribeverdict,transcribescan,main}.go`,
`statusgen/testdata/scan-delta-payload.json`

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
### Verification — 2026-09-30 (assay-verifier-app[bot] @ e03f4f5c7c41 (claude-opus-5-5) (on-behalf-of human:ian)) — 2026-09-30 claude-opus-5-5-verifier

Non-implementer verification on merged main e03f4f5c7c412560a666d95383bee0444fb6d263, gate: model, all four risk answers no. First table: the `statusgen verifyrun --dry-run` execution witness, landed verbatim; it ran on the darwin host (no row needs a Linux-only facility), statusgen built from a `--no-hardlinks` clone pinned to this SHA. Second table: the hand run.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test -run Verdict ./internal/deskkit/ && go test -run Canonical ./internal/deskkit/ && go test -run WrongKey ./internal/deskkit/ && go test -run Reflow ./internal/deskkit/ && go test -run DeriveAndParse ./internal/deskkit/ && go test -run PubkeyVarForRole ./internal/deskkit/ && go test -run Role ./internal/deskkit/` | pass exit=0 | sha256:8510a4f534d9 | 2026-09-30 | human:ian @ e03f4f5c7c41 (git-config) |
| 2 | `cd tools/desk && go test ./cmd/deskverdict/... -v` | pass exit=0 | sha256:463b5e880390 | 2026-09-30 | human:ian @ e03f4f5c7c41 (git-config) |
| 3 | `cd statusgen && go test -run ScanDelta . -v` | pass exit=0 | sha256:8da947ddc2ea | 2026-09-30 | human:ian @ e03f4f5c7c41 (git-config) |
| 4 | `cd statusgen && go test -run TranscribeScan . && go test -run Verdict . && go test -run PubkeyVar .` | pass exit=0 | sha256:4fc6548dfff1 | 2026-09-30 | human:ian @ e03f4f5c7c41 (git-config) |
| 5 | `test ! -e .github/verify && test -z "$(git ls-files "*.pem" "*.key" "*issue-loop-pubkey*")"` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-30 | human:ian @ e03f4f5c7c41 (git-config) |
| 6 | `cd tools/desk && gofmt -l cmd/deskverdict/ internal/deskkit/verdict.go internal/deskkit/verdict_test.go && cd ../statusgen && gofmt -l transcribescan.go transcribeverdict.go transcribeverdict_test.go transcribescandelta_test.go main.go` | fail exit=1 | sha256:c5bc76e0d6b7 | 2026-09-30 | human:ian @ e03f4f5c7c41 (git-config) |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRoleMismatchRefusedBeforeCrypto$' -count=1 -v` | pass exit=0 | sha256:6c1bab7bc1ae | 2026-09-30 | human:ian @ e03f4f5c7c41 (git-config) |

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd tools/desk && go test -run Verdict ./internal/deskkit/ && go test -run Canonical ./internal/deskkit/ && go test -run WrongKey ./internal/deskkit/ && go test -run Reflow ./internal/deskkit/ && go test -run DeriveAndParse ./internal/deskkit/ && go test -run PubkeyVarForRole ./internal/deskkit/ && go test -run Role ./internal/deskkit/` | exit 0; pre-existing verdict tests plus the six new role tests | exit 0; seven `ok github.com/medici-finance/assay/tools/desk/internal/deskkit` lines. With -v, TestValidVerdictRole, TestPubkeyVarForRole, TestIssueLoopRoleRoundtrip, TestRoleMismatchRefusedBeforeCrypto, TestUnrecognizedRoleNeverFallsBackToVerifier and TestNoRoleFieldDefaultsToVerifier all PASS | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test ./cmd/deskverdict/... -v` | exit 0; pre-existing CLI tests plus the seven new TestCLI role tests | exit 0; 16 PASS lines, no FAIL; all seven named TestCLI role tests PASS; `ok github.com/medici-finance/assay/tools/desk/cmd/deskverdict` | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 3 | `cd statusgen && go test -run ScanDelta . -v` | exit 0; 14 tests | exit 0; exactly 14 PASS (10 TestScanDelta plus 4 TestRunTranscribeScanDelta); `ok github.com/medici-finance/assay/statusgen` | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 4 | `cd statusgen && go test -run TranscribeScan . && go test -run Verdict . && go test -run PubkeyVar .` | exit 0 | exit 0; three `ok github.com/medici-finance/assay/statusgen` lines | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 5 | `test ! -e .github/verify && test -z "$(git ls-files "*.pem" "*.key" "*issue-loop-pubkey*")"` | exit 0, no key material tracked | exit 0; empty output (no .github/verify directory; no tracked path matches the key-material patterns) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 6 | `cd tools/desk && gofmt -l cmd/deskverdict/ internal/deskkit/verdict.go internal/deskkit/verdict_test.go && cd ../statusgen && gofmt -l transcribescan.go transcribeverdict.go transcribeverdict_test.go transcribescandelta_test.go main.go` | exit 0, empty output | FAIL AS AUTHORED: exit 1; `cd: no such file or directory: ../statusgen` (from tools/desk that path is tools/statusgen, which does not exist; statusgen is at the repo root). Hand procedure with `cd ../../statusgen`: exit 0, empty output, so every named file IS gofmt-clean | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 7 | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRoleMismatchRefusedBeforeCrypto$' -count=1 -v` | exit 0 baseline; reddens under the mutation; exit 0 on restore | baseline exit 0 (PASS). MUTATION applied by hand (deleted the 3-line declared-role block at verdict.go lines 453-455 in VerifyVerdictBodyForRole): exit 1, `verdict_test.go:368: role mismatch must be REFUSED even with a cryptographically valid signature, got 0 (verified: signature matches the canonical verdict payload)`. Restored via git checkout: file sha256 prefix 106ccaa1913bff22 before and after, git status clean, re-run exit 0 | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |

RISK-VALUE: DERIVED — VerdictRoleIssueLoop = "issue-loop" @ tools/desk/internal/deskkit/verdict.go:93 (twin scanDeltaWantRole = "issue-loop" @ statusgen/transcribescan.go:671; roster key "issue-loop" @ statusgen/transcribescan.go:774) — it must equal the roster's existing role-binding name for the intake/issue-loop App, and deskkit already maps intake-desk to "issue-loop" @ tools/desk/internal/deskkit/roletoken.go:46. The signer, the verifier and the container-author check all use byte-identical copies, so all three agree on one role.
RISK-VALUE: DERIVED — IssueLoopPubkeyVar = "ASSAY_ISSUE_LOOP_PUBKEY" @ tools/desk/internal/deskkit/verdict.go:108 (twin scanDeltaPubkeyVar @ statusgen/transcribescan.go:667) — this follows the ASSAY_VERIFIER_PUBKEY convention (verdict.go:515 and statusgen/transcribeverdict.go:80) and is byte-identical across both modules. A drift would silently turn the consuming lane into could-not-check.
RISK-VALUE: DERIVED — trailer role field "role=" @ tools/desk/internal/deskkit/verdict.go:351, parsed by verdictRoleRE @ verdict.go:83 and scanDeltaRoleRE @ statusgen/transcribescan.go:678 — both regexes are the same text over the same marker verdictSigMarker = "deskverdict-signature" (verdict.go:68, statusgen/transcribeverdict.go:69), so what deskverdict writes is what statusgen reads. Absent role= reads as verifier, which keeps every body signed before this brief compatible (TestNoRoleFieldDefaultsToVerifier PASS).
RISK-VALUE: DERIVED — container author Type = "Bot" @ statusgen/transcribescan.go:871 — this mirrors the R-6 clause-1 check at statusgen/transcribeverdict.go:748 exactly (login EqualFold, ID nonzero and equal, Type Bot). An App-authored issue is always type Bot, so the value is right.
RISK-VALUE: NAMED, NOT DERIVED — privKeyEnvForRole "ISSUE_LOOP_PEM" and privKeyFileForRole "issue-loop-app.pem" @ tools/desk/cmd/deskverdict/sign.go:99,106 — these follow the VERIFIER_PEM / verifier-app.pem pattern, but the operator-side config they must match lives outside this repo and could not be checked from here. A wrong value fails closed: sign refuses and names every path it searched.

Notes:
- BLOCKED (check-definition), not a product failure, and unchanged since the 2026-09-27 pass (#1735). Rows 1-5 and 7 pass; row 6's `cd ../statusgen` resolves to a path that does not exist from tools/desk, and the same gofmt command with `../../statusgen` exits 0 with empty output. `statusgen brief --check-verified` with a hypothetical flip exits 1 on row 6 alone. Advancing needs row 6 re-pointed, then a fresh witness.
- Grounding: before reading any PR, diff or test, I wrote the expected surface down from the brief text and main. Every expected symbol exists on main: the role consts, ValidVerdictRole, PubkeyVarForRole, the ForRole assemble/verify functions, the --key flag on sign and verify, resolveSignerPEM, resolvePublicKeyPEM, the --transcribe-scan-delta mode, runTranscribeScanDelta, planScanDelta, verifyScanDeltaEntry, the scanDelta helpers and the scan-delta-payload.json fixture. That write-up also predicted row 6's authored path failure, because no tools/statusgen directory exists at e03f4f5c.
- Row 6 is a CHECK-DEFINITION failure, not an implementation failure. The command as authored exits 1 because `cd ../statusgen` from tools/desk does not resolve. The property it checks holds: with `cd ../../statusgen` the same gofmt invocation exits 0 with empty output. Under the kit rule this ends BLOCKED, not PASS. The defect is unchanged since the 2026-09-27 pass: same witness hash, and the brief text is unmodified on main. That pass already filed it as medici-finance/assay#1735. This pass filed nothing new. Unblocking takes a brief edit correcting row 6's path to `../../statusgen`, followed by a fresh verifyrun; verifyrun appends, so the red run stays on record.
- Row 7's mutation was run for real. A first attempt with in-place sed did not modify the file, because GNU sed is on PATH and rejected the BSD `-i ''` form. Its green result is void and not counted. The mutation was then applied with perl, confirmed by git diff (3 deletions), reddened as the brief predicts, and was restored byte-identical.
- Review note (b), checked in source: VerifyVerdictBodyForRole (tools/desk/internal/deskkit/verdict.go:453) and scanDeltaVerifyBody (statusgen/transcribescan.go:747) both compare the declared role after parsing the body and before canonicalisation or any RSA call. Row 7's mutation proves the deskkit side reddens without that compare.
- Witness: darwin host witness. No row is check:ci and none needs Linux-only facilities, so the docker recipe was not used. The runner stamp came from the clone's inherited git config (human:ian, git-config). At landing, the house procedure should re-stamp it with the verifier App identity on behalf of the driver.
- check-verified: in a separate throwaway `--no-hardlinks` clone at e03f4f5c, I flipped the README row 28 Status cell to verified by hand. `statusgen brief --root <scratch>/flip --check-verified desk-tools/28` then gave exit code 1: `verified outcome requires passing execution witnesses: row 6: fail — the witness records a failure`. `statusgen --lint` on that flipped clone gives LINT: FAIL, with 1 PROBLEM of the same cause. On the unflipped clone, lint gives LINT: PASS with 0 PROBLEMs. The only lint NOTICE of note is risk-files-crossread: the brief touches tools/desk/internal/deskkit/ with all four risk answers no. That is advisory, and the gate derivation of gate:model stands as authored.

VERIFY: BLOCKED

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in
the stream README table. Review note: confirm (a) every pre-existing verdict/scan/verify test
still passes byte-for-byte unmodified in assertion, (b) the declared-role check really runs
BEFORE the crypto check in `VerifyVerdictBodyForRole` and `scanDeltaVerifyBody` (not just
documented as doing so), and (c) row 5 — no key material anywhere in the diff.
