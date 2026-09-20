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

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in
the stream README table. Review note: confirm (a) every pre-existing verdict/scan/verify test
still passes byte-for-byte unmodified in assertion, (b) the declared-role check really runs
BEFORE the crypto check in `VerifyVerdictBodyForRole` and `scanDeltaVerifyBody` (not just
documented as doing so), and (c) row 5 — no key material anywhere in the diff.
