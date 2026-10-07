---
brief: assay:assay:desk-tools:29
title: "`verifyloop verdict --unsigned-out <file>` — compose the verdict-v1 payload with no key, so a fenced runner composes and the host signs"
why: >-
  Today `verifyloop verdict` runs a brief's Verify rows, composes the verdict-v1 payload and
  signs it in one process, and it resolves the verifier's private key before it reads the queue.
  So the process that executes arbitrary Verify-row shell commands has to be able to read the
  verifier key. A downstream fenced-runner adopter wants the rows to run in a sealed container
  (no network, no key mounted) and the signature to happen afterwards on the host that holds the
  key. The signing half already exists (`deskverdict sign --payload <file>`); the composing half
  does not, because no mode of `verifyloop verdict` composes without first resolving a key. This
  brief adds that mode, so the code that runs untrusted row commands never needs the key in
  reach, and the host signs the same bytes today's combined path would have signed.
wave: 1
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: >-
  The change sits on the verifier key's custody boundary and changes what a verdict signature
  attests. Today the signer is the process that ran the rows, so a signature says "the key
  holder witnessed these results". After this brief, a host can sign a payload a separate fenced
  process composed, so a signature on that lane says "the key holder signed what the fence
  reported" — `deskverdict sign` canonicalises any JSON and checks no schema, so a compromised
  fence could report results the rows never produced. The verifier key is the credential in
  question (sensitive-data: yes; the same call desk-containers/02 made for a key-custody
  design). The human confirms three things: (1) the attestation change is acceptable for the
  benefit that row commands never run in a process that can read the key; (2) the new mode
  must never resolve, open or read a key, and the rows that prove it are discriminating; (3)
  the unsigned payload is a file only, never a fenced issue body, so nothing downstream can
  mistake it for a verdict.
decision-trigger: creation
issues: []
schema: brief-v2
version: 1
outcome: none
id: 5b7aa5be-b580-4522-a9f1-056b65e8913b
domain: complicated
authored: 2026-10-08 by a desk authoring session (subagent), from a downstream fenced-runner adopter's request
sources:
  - "A downstream fenced-runner adopter's request: run check/check:ci rows inside a network-none container that never holds the verifier key, write the canonical UNSIGNED verdict-v1 payload to a file, and sign on the host afterwards with `deskverdict sign --payload <file>`"
  - "docs/streams/desk-tools/brief-04-runner-verdict-batching.md: the deterministic runner this brief extends (execute rows, batch ~5 min, sign, print the would-be verdict body)"
  - "docs/streams/desk-tools/brief-28-deskverdict-role-keys-and-scan-delta-verify.md: `deskverdict sign --key verifier`, the host-side signer this brief pairs with, unchanged here"
  - "tools/desk/cmd/verifyloop/verdictrun.go:136-142: `runVerdict` calls `resolveVerifierPEMPath(cfg.pem)` FIRST, before `scanAwaiting` (:162) and before any row runs; :216-235 `emitBatch` ALWAYS calls `composePayload` then `signPayload`, and `dryRun` changes only the trailing text; :269-285 `resolveVerifierPEMPath` returns an override or a non-empty `VERIFIER_PEM` as-is (no stat), else `deskkit.FindConfigFile(\"verifier-app.pem\")`, failing closed with exit 6; :32 the package comment's invariant 'an unsigned verdict is never emitted'"
  - "tools/desk/cmd/verifyloop/verdictpayload.go:84 `composePayload(repo, head, ts, meta, rows)`; :127-148 `signPayload`: `json.Marshal`, `deskkit.CanonicalizeJSON`, `os.ReadFile(pemPath)`, `ParseRSAPrivateKeyPEM`, `SignVerdictCanonical`, `AssembleVerdictBody`"
  - "tools/desk/cmd/verifyloop/main.go:281 (the `verdict` usage line) and :305-309 (the `verdict` prose: '--dry-run composes + signs + prints without filing … A missing verifier PEM is a loud envelope error and nothing is signed')"
  - "tools/desk/cmd/deskverdict/sign.go:22-60: `cmdSign` reads `--payload` as ANY JSON (no verdict-v1 schema check), canonicalises it, resolves the signer key (`--pem`, then the role's env override, then `<role>-app.pem`) and assembles the body for the `--key` role"
  - "tools/desk/internal/deskkit/verdict.go: `CanonicalizeJSON` (idempotent), `SignVerdictCanonical` (RS256 PKCS#1 v1.5 — deterministic for a given key and input), `AssembleVerdictBody` (role=verifier shorthand of `AssembleVerdictBodyForRole`)"
  - "statusgen/transcribeverdict.go:266-276 `verdictParseBody`: a body with no signature trailer is a structural CouldNotCheck, never trusted"
  - "freshness-checked 2026-10-08 @ 36113a1dd (origin/main) — verdictrun.go, verdictpayload.go, main.go and verdictrun_test.go under tools/desk/cmd/verifyloop/, sign.go and main.go under tools/desk/cmd/deskverdict/, tools/desk/internal/deskkit/verdict.go and confighome.go re-read at that commit, and the live checks under `facts:` run against binaries built from it"
exec-tier: strong
exec-tier-why: >-
  (a) design decisions with a security consequence (what the new mode refuses, what it writes,
  what a signature now attests) and (c) key-custody plumbing whose dangerous failure — the
  unsigned branch quietly still resolving or opening the key — passes every happy-path test.
consumers:
  - "tools/desk/cmd/deskverdict/sign.go: out-of-scope (read-only consumer of the payload file, unchanged; row 3 proves it accepts the composed shape and produces the combined path's exact body)"
  - "statusgen/transcribeverdict.go: out-of-scope (never sees the unsigned payload: it reads issue bodies, and an unsigned payload is written only to a file; a body without a signature trailer is already CouldNotCheck there)"
---

# Brief 29 — `verifyloop verdict --unsigned-out <file>`: compose with no key, sign on the host

## Dependencies
None. `deskverdict sign --payload` (desk-tools/28) and the deterministic runner (desk-tools/04)
are both in the tree at the freshness stamp; this brief adds one mode to the runner and changes
neither the signer nor the verifier.

## Context
files:
- `tools/desk/cmd/verifyloop/verdictrun.go` — the new flag, the unsigned branch of `runVerdict`, the amended package comment
- `tools/desk/cmd/verifyloop/verdictpayload.go` — one shared canonical-bytes helper used by BOTH `signPayload` and the unsigned writer
- `tools/desk/cmd/verifyloop/main.go` — usage line and `verdict` prose
- `tools/desk/cmd/verifyloop/verdictunsigned_test.go` (planned) — rows 1, 3, 4
- `tools/desk/cmd/verifyloop/verdictunsigned_unix_test.go` (planned) — row 2 (FIFO canary, `//go:build unix`)
- `changelog/` — one fragment for the implementing PR

single-point-of-failure: the code path of the unsigned branch (it must never reach `resolveVerifierPEMPath`, `signPayload` or any key read) — behind it, the operator's fence (no key mounted, no network; outside this repo), and the consumers' refusal of an unsigned body (`deskverdict verify` exits 6 on a bare payload; statusgen's transcriber treats a body with no signature trailer as CouldNotCheck).

facts:
- Key resolution is the FIRST thing `runVerdict` does (`verdictrun.go:139`). Checked live at the stamp: with no key anywhere (empty `HOME`, `VERIFIER_PEM` and `ASSAY_CONFIG_HOME` unset) `verifyloop verdict --dry-run` exits 6 with "cannot find the verifier private key" even when the queue is empty — before the queue is read. No existing mode composes without a key.
- `--dry-run` DOES sign. Checked live: it printed the fenced canonical payload plus the `deskverdict-signature v1 alg=RS256 role=verifier` trailer and "signed verdict for 2 row(s) … — dry-run: not filed". Neither `--dry-run` nor the default mode files anything today; the only difference is the trailing text.
- With `VERIFIER_PEM` set, resolution does not stat the file; the key is OPENED only at sign time (`signPayload`'s `os.ReadFile`). Checked live: `VERIFIER_PEM` naming a nonexistent file runs every row and then exits 6 with "cannot read verifier key at …"; with an empty queue it exits 0 having opened nothing. So "VERIFIER_PEM set to a missing file" is NOT a discriminating negative for this brief — a run with no rows passes it whatever the code does. Rows 1 and 2 are built around that.
- `deskverdict sign --payload <file>` accepts exactly the composed payload shape — and any other JSON: it checks no schema. It prints the signed body on stdout and, when the payload path ends `.json`, also writes a sibling `<name>.out`. Checked live: the fenced payload extracted from a `--dry-run` body, signed by `deskverdict sign --pem <same key>`, was BYTE-IDENTICAL (`cmp`) to the body `--dry-run` printed, and `deskverdict verify` on it exited 0. RS256 PKCS#1 v1.5 is deterministic, which is why byte identity is achievable at all.
- `deskverdict verify --body <bare payload file>` exits 6 ("no ```verdict-payload block found"): an unsigned payload cannot pass as a verdict.
- The payload's `ts` is `cfg.nowFn()`; the CLI has no clock flag. So byte identity across two RUNS is provable only in a Go test that injects `now`, `exec`, `repo`, `head`, `session` and `runner` through `verdictRunConfig` (the existing tests already do: `TestRunVerdictDryRunEndToEnd`, helpers `writeTestKey`, `fakeExec`, `demoRoot`).
- The combined path may flush more than one payload per run (window `defaultBatchWindow` = 5m). The new mode composes ONE payload per run; equivalence is stated for a run the combined path also flushes once.
- Precedent for a test that builds a sibling binary: `tools/desk/cmd/deskpr/deskpr_test.go` and `tools/desk/cmd/deskdispatch/verifierattestation_test.go` (`exec.Command("go", "build", "-o", …)`).
- Exit codes (`verifyloop` usage): 0 ok · 3 disabled · 5 refused · 6 unverifiable · 7 author==runner. `deskkit.Refused` → 5, `deskkit.Unverifiable` → 6.

## Human decision
An automated verification tool runs each work item's check commands and then signs the results
with a private key, so a downstream reader can trust that the results came from the verifier.
Today one process does both: it loads the key before it runs anything, so the check commands —
which are arbitrary shell — run in a process that can read the key.

An adopter wants to split this in two. The check commands run inside a sealed container with no
network and no key; that container writes the results to a file, unsigned. Afterwards, on the
host that holds the key, a separate existing signing command signs that file. For the same
results, the signed output is byte-for-byte what today's combined process produces, so nothing
downstream changes.

What changes is what a signature means on that path. Today it means "the key holder ran the
checks and saw these results". On the split path it means "the key holder signed what the
container reported". The signing command does not check the file's structure, so a container
that was tampered with could report results the checks never produced, and the host would sign
them. In exchange, the check commands can no longer read or steal the key, which on the combined
path they could.

The new mode is built to never look for, open or read a key, even when the key's location is
configured, and it writes the unsigned results only to a file it creates fresh, never in the form
a signed result takes.

Options:
1. **Approve as briefed.** Add the split mode exactly as above. No change to the signed format;
   signed output stays byte-identical to the combined path. The split path is opt-in per run.
2. **Approve, and add a host-side structure check first.** Same mode, but the signing command
   must also refuse a file that is not a well-formed results record before it signs. This is a
   change to the signer and is authored as its own follow-up before this one ships.
3. **Approve, but mark split-path signatures.** Record in the signed results that they were
   composed elsewhere. This changes the signed format, breaks byte identity with the combined
   path, and needs a format version bump — a separate, larger change.
4. **Reject.** Keep the combined process only; adopters who want the checks fenced from the key
   have no supported path.

Default if no answer: none — blocks until answered.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- Do not change `tools/desk/cmd/deskverdict/`, `tools/desk/internal/deskkit/verdict.go` or the
  existing tests in `tools/desk/cmd/verifyloop/verdictrun_test.go` (row 8 checks this). The
  existing signed path is the baseline the new mode is proven equal to.
- No key material is committed: tests generate keys with `writeTestKey` into `t.TempDir()`.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Flag.** In `cmdVerdict` add `--unsigned-out <file>`: "compose the verdict-v1 payload over
   every executed row and write its canonical bytes to <file>, UNSIGNED; resolves and reads no
   key — sign on the key-holding host with `deskverdict sign --payload <file>`". The name says
   the custody fact (`unsigned`) and the shape (`-out` a file), and pairs with the signer's
   `--payload`. Carry it in `verdictRunConfig` as `unsignedOut string`.
2. **Refusals, all exit 5 (`deskkit.Refused`), all before any key lookup, queue read or row
   run:**
   - `--unsigned-out` with `--pem` (a key path in an unsigned run is a caller error, never
     silently ignored);
   - `--unsigned-out` with `--dry-run` or with an explicitly passed `--window` (detect explicit
     flags with `fs.Visit`) — neither has a meaning in this mode, and a silent ignore would hide
     a caller who expected batching;
   - an `--unsigned-out` path that already exists (`os.Lstat` succeeds), naming the path.
3. **The unsigned branch of `runVerdict`.** When `cfg.unsignedOut != ""`, branch BEFORE the
   `resolveVerifierPEMPath` call: derive repo/head/session/runner exactly as the signed path
   does, `scanAwaiting`, run every item's rows with `runBriefRows`, and compose ONE payload with
   `composePayload(repo, head, cfg.nowFn(), meta, rows)` over all rows (the batching window does
   not apply: cadence is the host's). The branch must not call `resolveVerifierPEMPath`,
   `signPayload`, `deskkit.FindConfigFile`, or read `VERIFIER_PEM` / `ASSAY_CONFIG_HOME`.
   Factor it as its own function (for example `runVerdictUnsigned`) so the separation is visible
   in review.
4. **One canonical form.** Add `canonicalPayloadBytes(p verdictPayload) ([]byte, error)`
   (`json.Marshal` then `deskkit.CanonicalizeJSON`) in `verdictpayload.go`, and make
   `signPayload` call it instead of its own two lines. The unsigned writer uses the same helper,
   so the bytes written are the bytes the combined path signs.
5. **Write.** Open the path with `os.O_WRONLY|os.O_CREATE|os.O_EXCL`, mode `0644`, write the
   canonical bytes plus one `"\n"`, close and check the close error. An `O_EXCL` failure (a row
   created the path during the run) is exit 5 naming the path, and the existing file is left
   untouched. No temp-file-then-rename: the fence's output directory is the operator's, and
   `O_EXCL` is the binding check.
6. **Stdout.** One line: `unsigned verdict payload for N row(s) across M brief(s) written to
   <file> — NOT signed; sign on the key-holding host: deskverdict sign --payload <file>`. Stdout
   never carries a ```` ```verdict-payload ```` fence or a `deskverdict-signature` trailer in
   this mode, so nothing that scrapes stdout for a verdict body can pick up an unsigned one.
7. **Empty queue.** No runner-executed rows: write NO file, print `verdict runner: no
   runner-executed (check/check:ci) rows in the Awaiting queue — nothing to compose`, exit 0.
   (The host's `deskverdict sign` then fails on the missing file, which is the correct signal.)
8. **Docs.** Amend the package comment's invariant at `verdictrun.go:32` to "an unsigned verdict
   BODY is never emitted; an unsigned PAYLOAD is written only to an explicit `--unsigned-out`
   file". Add `[--unsigned-out <file>]` to the `verdict` usage line in `main.go` and one sentence
   to the `verdict` prose naming the host step and that the mode reads no key.
9. **Tests** — the planned files under `files:`, one per Verify row below, anchored names as
   written there. Row 2's FIFO canary lives in the `//go:build unix` file.
10. **Changelog fragment** under `changelog/` (`### Added`): the new flag and that it reads no
    key.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci +mutation | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestUnsignedOutNeverResolvesVerifierKey$' -count=1 -v > "${TMPDIR:-/tmp}/b29-r1.out" 2>&1 && grep -F -e '--- PASS: TestUnsignedOutNeverResolvesVerifierKey' "${TMPDIR:-/tmp}/b29-r1.out"` | exit 0, `--- PASS:` printed. **Negative path**: `HOME` is an empty temp dir, `VERIFIER_PEM` and `ASSAY_CONFIG_HOME` are set empty, `demoRoot` holds runner rows. Arm A: `runVerdict` with `unsignedOut` returns nil, the file exists and parses as a `verdictPayload` with every executed row. CONTROL arm in the same test, same environment: the signed path (`dryRun: true`) returns an error whose `deskkit` exit code is 6 and whose text contains `cannot find the verifier private key` — so the environment really has no key, and arm A's success is not vacuous. Arm B: `VERIFIER_PEM` names a REAL valid key from `writeTestKey`; the unsigned run's stdout and file contain neither ```` ```verdict-payload ```` nor `deskverdict-signature`. Mutation M1 turns arm A red |
| 2 | check:ci +mutation | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestUnsignedOutNeverOpensVerifierPEMCanary$' -count=1 -timeout 120s -v > "${TMPDIR:-/tmp}/b29-r2.out" 2>&1 && grep -F -e '--- PASS: TestUnsignedOutNeverOpensVerifierPEMCanary' "${TMPDIR:-/tmp}/b29-r2.out"` | exit 0, `--- PASS:` printed. **Negative path, discriminating**: `VERIFIER_PEM` names a FIFO made with `syscall.Mkfifo` in `t.TempDir()`. A detector goroutine repeatedly opens the FIFO write-only and non-blocking (`os.O_WRONLY` combined with `syscall.O_NONBLOCK`); on a FIFO that open succeeds only while some reader has it open, and fails with `ENXIO` otherwise. Unsigned arm: the detector never succeeds for the whole run, and the payload file is written. CONTROL arm (signed path, `dryRun: true`, same FIFO, `demoRoot` rows): the detector DOES succeed (proving it can see a key open), then closes, the reader gets EOF and the run returns an error. A test whose control arm never fires fails — so the row cannot pass on a broken detector. Mutation M2 turns the unsigned arm red |
| 3 | check:ci +flow +mutation | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestUnsignedComposeSignedByDeskverdictMatchesCombined$' -count=1 -v > "${TMPDIR:-/tmp}/b29-r3.out" 2>&1 && grep -F -e '--- PASS: TestUnsignedComposeSignedByDeskverdictMatchesCombined' "${TMPDIR:-/tmp}/b29-r3.out"` | exit 0, `--- PASS:` printed. Fixed `now`, `repo`, `head`, `session`, `runner`, `fakeExec`, one `demoRoot` (one flush). (a) Combined path, `pem` = a `writeTestKey` key, `dryRun: true`: capture the printed body up to and including the signature trailer line. (b) Unsigned path, same inputs, writes the file. (c) The file's bytes equal the fenced payload text inside (a)'s body plus `"\n"` — the file IS the combined path's canonical form. (d) The test builds `deskverdict` from `../deskverdict` into `t.TempDir()` and runs `sign --payload <file> --pem <same key>`: exit 0, and its stdout is BYTE-IDENTICAL to (a)'s body (`bytes.Equal`, diff printed on failure). (e) `deskverdict verify --body <file>` (the unsigned file) exits 6. (f) `deskverdict verify` on the (d) output with the matching public key exits 0. Mutation M3 turns (c) red |
| 4 | check:ci +mutation | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestUnsignedOutRefusals$' -count=1 -v > "${TMPDIR:-/tmp}/b29-r4.out" 2>&1 && grep -F -e '--- PASS: TestUnsignedOutRefusals' "${TMPDIR:-/tmp}/b29-r4.out"` | exit 0, `--- PASS:` printed. Table cases through `cmdVerdict`, each with an `exec` that records calls: `--unsigned-out f --pem k`, `--unsigned-out f --dry-run`, `--unsigned-out f --window 1m`, and an `--unsigned-out` path that already exists each return 5 with ZERO rows executed and the pre-existing file's bytes unchanged. A case whose `exec` CREATES the output path with planted bytes while a row runs returns 5 and the planted bytes are unchanged afterwards. An empty-queue case returns 0, prints `nothing to compose`, and the path does not exist. Mutation M4 turns the planted-during-run case red |
| 5 | check:ci | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestRunVerdictMissingPEMFailsClosedAndFilesNothing$' -count=1 -v > "${TMPDIR:-/tmp}/b29-r5a.out" 2>&1 && grep -F -e '--- PASS: TestRunVerdictMissingPEMFailsClosedAndFilesNothing' "${TMPDIR:-/tmp}/b29-r5a.out" && go test ./cmd/verifyloop/ -run '^TestDryRunSignVerify$' -count=1 -v > "${TMPDIR:-/tmp}/b29-r5b.out" 2>&1 && grep -F -e '--- PASS: TestDryRunSignVerify' "${TMPDIR:-/tmp}/b29-r5b.out" && go test ./cmd/verifyloop/ -run '^TestRunVerdictDryRunEndToEnd$' -count=1 -v > "${TMPDIR:-/tmp}/b29-r5c.out" 2>&1 && grep -F -e '--- PASS: TestRunVerdictDryRunEndToEnd' "${TMPDIR:-/tmp}/b29-r5c.out" && go test ./cmd/verifyloop/ -run '^TestSignedBodyVerifiesAndTamperRefuses$' -count=1 -v > "${TMPDIR:-/tmp}/b29-r5d.out" 2>&1 && grep -F -e '--- PASS: TestSignedBodyVerifiesAndTamperRefuses' "${TMPDIR:-/tmp}/b29-r5d.out"` | exit 0, four `--- PASS:` lines. The existing signed path is unchanged: a missing key still fails closed with exit 6 and files nothing, `--dry-run` still signs and verifies, and a tampered body is still refused. Row 8 proves these tests were not edited to pass |
| 6 | check:ci | `cd tools/desk && test -z "$(gofmt -l cmd/verifyloop/)" && go vet ./cmd/verifyloop/ && go test ./cmd/verifyloop/ -count=1` | exit 0 — gofmt-clean, vet-clean, and the whole package (new and existing tests) passes |
| 7 | check | `cd tools/desk && go run ./cmd/verifyloop verdict --help > "${TMPDIR:-/tmp}/b29-r7.out" 2>&1; grep -F -e '--unsigned-out' "${TMPDIR:-/tmp}/b29-r7.out" && grep -F -e 'unsigned PAYLOAD is written only to an explicit' cmd/verifyloop/verdictrun.go && grep -F -e '--unsigned-out' cmd/verifyloop/main.go` | exit 0: the flag appears in the `verdict` help output and in the usage text, and the package comment's invariant is amended (the help command's own exit code is not the assertion; the greps are) |
| 8 | check | `git fetch -q origin main && b="$(git merge-base FETCH_HEAD HEAD)" && git diff --quiet "$b" HEAD -- tools/desk/cmd/deskverdict tools/desk/internal/deskkit/verdict.go && git diff --output-indicator-old='<' "$b" HEAD -- tools/desk/cmd/verifyloop/verdictrun_test.go > "${TMPDIR:-/tmp}/b29-r8.diff" && ! grep -q -e '^<' "${TMPDIR:-/tmp}/b29-r8.diff" && test -z "$(git ls-files '*.pem' '*.key')" && echo ok` | prints `ok`. The signer, the verdict primitives and every existing line of the existing test file are unchanged against a freshly fetched `main` (additions to the test file are allowed; a removed or edited line is not — removed lines are marked `<` so the diff's own `---` header cannot match), and no key material is tracked |
| 9 | check:ci | `cd statusgen && go run . --root .. --lint` | exit 0, `LINT: PASS` |
| 10 | check:ci | `cd statusgen && go build -o "${TMPDIR:-/tmp}/b29-statusgen" . && cd .. && git fetch -q origin main && "${TMPDIR:-/tmp}/b29-statusgen" --root . --consumers --brief desk-tools/29 --base "$(git merge-base FETCH_HEAD HEAD)"` | exit 0; the summary line reads `0 disproved`. Both entries are `out-of-scope`, which the gate reports as UNCHECKED, not passed — their truth is the reviewer's call, backed by row 8 (neither file changed) and row 3 (the signer accepts the composed shape) |

### Named mutations
Each was chosen so the row it names is the ONLY thing standing between it and a merge.
- **M1** — at the top of the unsigned branch, add `if _, err := resolveVerifierPEMPath(""); err != nil { return err }`. Row 1 arm A reddens (exit 6, key not found).
- **M2** — in the unsigned branch, add `if p := os.Getenv("VERIFIER_PEM"); p != "" { _, _ = os.ReadFile(p) }`. Row 1 stays green (arm A has `VERIFIER_PEM` empty; arm B's read succeeds silently); row 2's detector fires on the unsigned arm and it reddens. This is why row 2 exists beside row 1.
- **M3** — in the unsigned writer, replace `canonicalPayloadBytes(p)` with `json.MarshalIndent(p, "", "  ")`. Row 3 (d) stays green — `deskverdict sign` re-canonicalises — so only (c) catches a non-canonical file. This is why (c) is asserted separately.
- **M4** — drop `os.O_EXCL` from the open flags. Row 4's planted-during-run case reddens (the planted bytes are overwritten).

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: human (from frontmatter; sensitive-data: yes). The human decision above is ratified
before dispatch; the option chosen is recorded on the brief's decision issue and, if it is
option 2 or 3, the brief is revised before it moves to in-progress.

A reviewer answers both questions in the verdict:
1. What is the single control between the new mode and a key read, and is it acceptable?
   (Expected: the unsigned branch's code path. Behind it, the operator's fence holds no key —
   outside this repo, so this brief cannot prove it — and an unsigned payload is refused by
   every consumer that reads a verdict body. Acceptable because the mode is opt-in, the fence is
   the point of the request, and rows 1 and 2 test the code path from two independent angles.)
2. Does any Verify row prove a lower layer catches the fault with the upper layer bypassed?
   (Expected: row 2 catches a key OPEN by any route — not only through `resolveVerifierPEMPath`
   — so it holds when the structural separation of row 1 is bypassed (M2); row 3 (e) proves the
   consumer layer refuses the unsigned file on its own, independent of the fence.)

The reviewer also confirms from the diff that `signPayload` now calls `canonicalPayloadBytes`
and nothing else changed in the signed path's behaviour, and that no stdout line in the unsigned
mode can contain a payload fence.
