---
brief: assay:assay:desk-tools:66
title: Pin the verdict transcriber's trust anchors in a committed file
why: >-
  The verdict transcriber lands verified rows on main from a signed verdict issue. It decides
  whom to believe from three values it reads out of the repository's Actions configuration: the
  blessing login, the verifier role binding and the verifier public key. Anyone who can write
  that configuration can change all three in one step, and nothing in the tree or in review
  shows the change. A committed file on main that the transcriber reads first, plus a check
  that fails closed when the Actions copy differs, puts a reviewed change in front of every
  edit to those values. The environment read stays as the fallback for installations that have
  no file.
wave: 1
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
gate-why: >-
  It changes where a trust control reads its anchors from. Today the transcriber believes
  whatever the Actions variables hold. After this brief it believes the committed file when one
  exists, and refuses when the two differ. That moves who can change the anchors from "anyone
  with write access to Actions configuration" to "a merged pull request", and it adds a second
  source that can disagree with the first. The human confirms the file's path and shape, where
  the equality check runs, and what stays outside the pin.
decision-trigger: creation
issues: [2354]
outcome: none
schema: brief-v2
version: 1
authored: 2026-11-22 by a worker-desk authoring session (design proposal; no ruling recorded yet)
sources:
  - "medici-finance/assay#2354 (the issue this brief answers: pin the transcriber's trust anchors in a committed file with a CI equality check; its trusted commenter asked for this gate: human brief)"
  - "desk-tools/30 (records the accepted residual: the anchors are exactly as protected as write access to Actions configuration, and names this pin as the unauthored follow-up)"
  - "statusgen/transcribeverdict.go (verdictResolvePubkey, verdictVerifierIdentity, transcribeVerdictEnactmentGate) and statusgen/rosterconfig.go (scanReadRawConfig), read at 63e1f4e6a8"
  - "freshness-checked 2026-11-22 @ 63e1f4e6a8 (origin/main)"
exec-tier: strong
exec-tier-why: "It changes the input of a trust control, must fail closed on every missing, malformed or disagreeing anchor, and the file read, the equality check and the fallback must agree on one normal form."
domain: complicated
consumers:
  - "statusgen/rosterconfig.go: follow-up desk-tools/66 (this brief; scanReadRawConfig gains the committed-file read ahead of the Actions environment read for the CI class)"
  - "statusgen/transcribeverdict.go: follow-up desk-tools/66 (this brief; the transcriber's three anchor reads go through the one pinned reader and refuse on a disagreement)"
  - "statusgen/main.go: follow-up desk-tools/66 (this brief; the --pubkey flag is checked against the pinned key when the file exists)"
  - "docs/contracts.md: follow-up desk-tools/66 (this brief; the transcriber's configuration contract names the file and the precedence)"
  - "statusgen/evaluation/witness.go: out-of-scope (the witness record binds a verdict's outcome, not the transcriber's anchors)"
  - "the other roster entries (trusted logins, the remaining bot slugs): out-of-scope (they stay as Actions configuration, the residual this brief does not claim to close)"
---

# Brief 66 — Pin the verdict transcriber's trust anchors in a committed file

## Dependencies
- No typed edge. The transcriber, the roster reader and the CI class all exist on main.
- desk-tools/30 recorded the residual this brief narrows and named the pin as the follow-up.
  It does not need to change.

## Context

files:
- `statusgen/rosterconfig.go` — `scanReadRawConfig`, `scanRosterReadKeys`, the `ASSAY_BLESS_LOGIN` parse
- `statusgen/transcribeverdict.go` — `verdictPubkeyVar`, `verdictResolvePubkey`, `verdictVerifierIdentity`, `transcribeVerdictEnactmentGate`, `runTranscribeVerdict`
- `statusgen/main.go` — the `--pubkey` flag
- `docs/contracts.md` — the transcriber's configuration contract
- `docs/streams/desk-tools/brief-30-verify-desk-landing-by-verdict-issue.md` — the accepted residual
- **add** — `statusgen/trustanchors.go` (planned), `statusgen/trustanchors_test.go` (planned), `statusgen/trustanchors-mutations.json` (planned)
- **add** — the pinned anchors file at the path the answer names (planned)
- **add** — `changelog/<branch-slug>.md` — one fragment

facts:
- In CI the roster is read from the process environment only. `scanReadRawConfig` returns
  `fromEnv()` for the CI class, labelled "environment (repository/organization Actions
  variables)" (`statusgen/rosterconfig.go:777-778`). Nothing in the tree is consulted.
- The transcriber takes three anchors from that configuration:
  1. the blessing login, `ASSAY_BLESS_LOGIN` (`rosterconfig.go:105`, parsed at `:1001`), which
     `transcribeVerdictEnactmentGate` compares to the author of the R-6 sign-off
     (`transcribeverdict.go:264`);
  2. the verifier role binding, the `verifier=` entry of `ASSAY_TRUSTED_BOT_SLUGS`, which
     `verdictVerifierIdentity` turns into the login and bot user id every verdict's author must
     match (`transcribeverdict.go:435-447`);
  3. the verifier public key, `ASSAY_VERIFIER_PUBKEY` (`transcribeverdict.go:75`), resolved by
     `verdictResolvePubkey` from a `--pubkey` file first, then the variable, else an error
     (`:174-190`).
- The public key's comment says it is a variable, never a secret, and "NOT committed to the
  tree", so that adopters running locally generate their own keypair and set the variable to
  their own public half (`transcribeverdict.go:71-75`). This brief does not reverse that for
  adopters. It adds a committed file that an installation may have, and keeps the variable as
  the read for an installation that has none.
- All three values are as protected as write access to the repository's Actions configuration,
  and a single change there moves all three together. A signature check and an author check
  that both rest on the same anchor are two checks on one point, not two layers (desk-tools/30
  records this). The enactment gate arms the lane. It does not guard any single verdict.
- The transcriber already reads a committed file from the tree it runs on: the enactment gate
  reads `docs/streams/issue-flow/rulings.md` under `root` (`transcribeverdict.go:240`). A
  committed anchors file is the same kind of read.
- The workflow that runs the transcriber is not in this repository. The check therefore cannot
  live only in a workflow here. It has to be a property of the transcriber, so it holds wherever
  the transcriber runs.
- `runTranscribeVerdict` already refuses with exit 2 when the roster is unconfigured or no
  verifier is bound (`transcribeverdict.go:945-1000`). A disagreement between the file and the
  environment is the same class of refusal.

design-fit:
  owner: statusgen/trustanchors.go (planned) is the one reader the three anchor reads share
  contract: S-trust-roster
  retires: [the direct environment read of the three anchors inside the transcriber, once the pinned reader owns it]
  weight: "verbs 0, flags 0, refusals 4 (malformed file, disagreement, flag disagreement, file read outside the base tree), one committed file"
  why-add: "One reader that knows both sources replaces three separate environment reads. A disagreement has one named refusal instead of the lane quietly trusting whichever source it read first."

## Human decision

The verdict transcriber decides whom to believe from three values kept in the repository's
Actions settings: the login that may sign off the lane, the App that may sign verdicts, and that
App's public key. Whoever can edit those settings can change all three at once, and the change
does not appear in the tree or in any review. This brief proposes that the same three values are
also written in a file on main. The transcriber reads that file first and stops with a refusal
if the Actions copy says something different. An installation with no such file keeps working
from the Actions settings as it does today.

Options:
1. **Committed file authoritative, Actions copy checked against it, fallback kept** — the
   recommendation. When the file exists on the tree the transcriber runs on, it is the source of
   the three anchors. If the Actions copy of any of the three is set and differs, the transcriber
   refuses with exit 2 and names which anchor disagrees. With no file it reads the Actions
   settings as today and says on its source line that it did. The threat it opens: the file
   becomes a second place to keep three values, and a person who edits only one copy stops the
   lane until the other is fixed. That stop is the intended signal.
2. **Committed file only, no environment fallback** — the transcriber refuses when the file is
   absent. It closes the fallback, but it breaks every installation that has not added a file,
   including adopters who generate their own keypair. The source issue asked for the fallback to
   stay.
3. **Check without a pin** — keep reading the Actions settings, and add a CI check that the
   settings equal a value recorded in the tree. It is the same file with the transcriber still
   believing the settings, so a change made between two runs of the check is trusted until the
   next one.

Questions the answer must also settle, with the recommended default first:
- **Path and format.** A single small file in the repository's config area, one `KEY=value` line
  per anchor, using the Actions variables' own names (`ASSAY_BLESS_LOGIN`,
  `ASSAY_VERIFIER_PUBKEY`, and the `verifier=` entry of `ASSAY_TRUSTED_BOT_SLUGS`). Using the
  same names makes the equality check a comparison of like with like. The driver names the path.
- **Where the equality check runs.** In the transcriber, so it holds wherever the lane runs, and
  in a dedicated CI job here in the shape of `pin-consistency.yml`, so a pull request that edits
  the file is exercised. Transcriber only is the smaller change, and leaves a bad edit to the
  file undetected until the lane next runs.
- **Review of the file itself.** Pinning moves the trust boundary from Actions configuration to
  whoever may merge a change to the file. Whether that file also needs an owner rule or a
  branch-protection path rule is a repository-settings change this brief does not make. The
  answer says whether to file it separately.
- **Scope of the pin.** Only the three anchors. The trusted-login list and the other bot slugs
  stay Actions configuration, and the brief says so rather than implying the roster is pinned.

Default if no answer: none. The brief blocks until answered, because it changes where a trust
control reads from and a default must not do that. Option 1 is the recommendation.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Implement only the recorded answer. If the answer is option 2 or 3, this brief's Task does not
  apply; report NEEDS_CONTEXT so the desk re-authors.
- Fail closed. A file that exists but cannot be read, parsed or normalised refuses. A file that
  names an anchor the Actions copy contradicts refuses. A missing anchor is could-not-check,
  never trust.
- The file is read from the tree the transcriber lands onto, never from a pull-request head, an
  issue body or a verdict payload. A file path outside `root` refuses.
- The pinned values are public material: a login, a bot slug with its numeric id, and a PEM
  public key. The file carries no secret and the check never prints one.
- The signature check, the author check, the enactment gate's other clauses, the network-off
  re-execution of `check:ci` rows and the write-class bounds are not touched.

## Task
1. Add `statusgen/trustanchors.go` (planned) with one reader, `readTrustAnchors` (planned). It
   returns, for each of the three anchors, a value and its source (`file` or `environment`), or a
   named refusal. It normalises both sources to the same form before comparing: logins are
   compared case-insensitively with their ids, the bot entry as slug and numeric id, and the key
   as the parsed public key's DER bytes, so a PEM string and a base64-of-PEM value that name the
   same key are equal.
2. Read the committed file from `root` at the path the answer names. Absent: return the
   environment values, each with source `environment`. Present: the file's values are the
   anchors. Present but unreadable, malformed, or naming an unknown key: refuse.
3. When the file is present and an Actions copy of the same anchor is set and differs after
   normalisation, refuse and name the anchor. An Actions copy that is unset is not a
   disagreement. A set copy that equals the file is not a disagreement.
4. Route the three reads through the reader: the blessing login that
   `transcribeVerdictEnactmentGate` compares, the identity `verdictVerifierIdentity` builds, and
   the key `verdictResolvePubkey` returns. In `scanReadRawConfig` the CI class gains the file read
   ahead of the environment read, so the source line says which one was used.
5. When `--pubkey` is given and the file exists, the flag's key must equal the pinned key or the
   transcriber refuses. The flag stays usable for a local run with no file.
6. Print the source of each anchor on the transcriber's report, for example
   `anchors: blessing=file verifier=file pubkey=file`, and `anchors: environment (no pinned
   file)` when there is none, so a run that fell back says so in its log.
7. Add the dedicated CI job, if the answer includes it, as `.github/workflows/trust-anchors.yml` (planned)
   in the shape of `pin-consistency.yml`, triggered on the pinned file, the transcriber
   sources and the workflow itself, running the `TestTrustAnchors` tests with `-count=1`.
8. Document the file, its precedence and the fallback in `docs/contracts.md`, and add one
   changelog fragment.
9. Flip this brief's consumer routings to the delivering PR's files, as the `--consumers` check
   requires.

## Why this does not weaken the gate
- No check is removed. The signature check, the author check and the enactment gate keep their
  clauses and read the same three values through one reader.
- Where there is no file, the lane reads exactly what it reads today. Nothing that refuses today
  passes, and nothing that passes today refuses, until an installation adds a file.
- With a file, there are two sources and the transcriber trusts neither unless they agree where
  both are set. A stale Actions copy stops the lane. It never lets a different value through.
- What is given up is narrow: an installation that adds a file can no longer change an anchor in
  Actions settings alone. That is the point of the pin.
- What stays residual, and is said so: write access to the pinned file's path through a merged
  pull request, the other roster entries, and the host that runs the transcriber. This brief
  narrows one residual. It does not claim to remove the trust boundary.

## Verify (executable — no prose-only DoD items)
Every row is risk-bearing, and rows 1 and 3 red at the delivery base, where the named tests and
the mutation spec do not exist. The verifier records them with `verifyrun --fail-first` against
the parent of the delivering merge.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow | `test -f statusgen/trustanchors_test.go && L=$(mktemp) && cd statusgen && go test . -run '^TestTrustAnchors' -count=1 -v > "$L" 2>&1 && grep -F -e '--- PASS: TestTrustAnchorsFileWins ' "$L" && grep -F -e '--- PASS: TestTrustAnchorsEnvFallback ' "$L" && grep -F -e '--- PASS: TestTrustAnchorsDisagreeRefuses ' "$L" && grep -F -e '--- PASS: TestTrustAnchorsMalformedRefuses ' "$L" && grep -F -e '--- PASS: TestTrustAnchorsUnsetCopyAgrees ' "$L" && grep -F -e '--- PASS: TestTrustAnchorsKeyFormsEqual ' "$L" && grep -F -e '--- PASS: TestTrustAnchorsFlagDisagreeRefuses ' "$L" && grep -F -e '--- PASS: TestTrustAnchorsSourceLine ' "$L" && ! grep -F -e '--- FAIL' "$L"` | exit 0; risk-bearing; the file is the source when present, the environment is read when it is absent and the source line says so, a differing Actions copy refuses and names the anchor, an unset copy and an equal copy do not, a PEM string and a base64-of-PEM value for one key compare equal, and a differing `--pubkey` refuses. The log is a fresh `mktemp` file and every grep runs only after the suite passed, so a red suite or a stale log fails the row. Red at the delivery base (no such test file) |
| 2 | check:ci +neighbour | `L=$(mktemp) && cd statusgen && go test . -run 'TranscribeVerdict' -count=1 && go test . -run 'Roster' -count=1 && go test . -run 'Verdict' -count=1 -v > "$L" 2>&1 && ! grep -F -e '--- FAIL' "$L"` | exit 0; risk-bearing; the existing transcriber, roster and verdict suites pass unchanged beside the new reader, so a repository with no pinned file behaves as before. Every link is chained with `&&` into a fresh `mktemp` log |
| 3 | check:ci +mutation | `T=$(mktemp -d) && (cd tools/desk && go build -o "$T/muhar" ./cmd/muhar) && cd statusgen && "$T/muhar" -spec trustanchors-mutations.json -j 4 > "$T/out" 2>&1; rc=$?; L=$(tail -1 "$T/out"); echo "$L"; [ $rc -eq 0 ] && grep -F -e 'positive control CAUGHT' "$T/out" && [ "$L" = 'Totals: 5 caught, 0 NOT CAUGHT, 0 could-not-mutate.' ]` | exit 0; risk-bearing; the five mutants (file ignored, disagreement skipped, environment copy preferred over the file, key forms compared as text, flag skipped) and the positive control are caught, so each fail-closed rule has a test that fires. Red at the delivery base (no spec) |
| 4 | check +dereference | `test -n "${PINNED_FILE:-}" && test -f "$PINNED_FILE" && [ "$(grep -c -e '^ASSAY_BLESS_LOGIN=' -e '^ASSAY_VERIFIER_PUBKEY=' -e '^ASSAY_TRUSTED_BOT_SLUGS=' "$PINNED_FILE")" = 3 ] && ! grep -q -e 'PRIVATE KEY' "$PINNED_FILE"` | exit 0; risk-bearing; the committed file carries exactly the three anchors and no private key. `PINNED_FILE` is the path the answer names, set by the verifier from the recorded answer |
| 5 | check +flow +dereference | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-66-pin-the-verdict-transcriber-s-trust-anchors-in-a-committed-file.md && o=$(mktemp) && statusgen --root . --consumers --brief desk-tools/66 --delivery "$DELIVERY_HEAD" --base "$DELIVERY_BASE" > "$o" 2>&1; grep -F -e ' 0 disproved, 0 unchecked' "$o"` | exit 0; risk-bearing; every consumer is in the delivering diff |

## Threat model and detection

| Failure mode | Caught by |
|---|---|
| The committed file is ignored and the Actions copy wins | row 1 (`TestTrustAnchorsFileWins` (planned)); row 3 (file-ignored mutant) |
| A differing Actions copy is trusted instead of refused | row 1 (`TestTrustAnchorsDisagreeRefuses` (planned)); row 3 (disagreement-skipped mutant) |
| The Actions copy is preferred when both are set | row 3 (environment-preferred mutant) |
| A malformed or unreadable file is read as absent and the lane falls back | row 1 (`TestTrustAnchorsMalformedRefuses` (planned)) |
| The same key in two encodings reads as a disagreement, or two keys read as equal | row 1 (`TestTrustAnchorsKeyFormsEqual` (planned)); row 3 (text-compare mutant) |
| A `--pubkey` file overrides the pinned key | row 1 (`TestTrustAnchorsFlagDisagreeRefuses` (planned)); row 3 (flag-skipped mutant) |
| A run that fell back to the environment does not say so | row 1 (`TestTrustAnchorsSourceLine` (planned)) |
| A repository with no file behaves differently from today | row 2 (the existing suites pass unchanged) |
| The file carries a secret | row 4 (no private key marker; the file holds public values only) |
| A consumer is left reading the environment directly | row 5 (every routed consumer is in the delivering diff) |
| The pinned file is changed by someone who should not | No row; it is the boundary this brief moves. Review-only: the PR that edits the file is the control, and whether it needs an owner rule is the question the answer settles |
| The other roster entries are changed in Actions settings | No row; they are out of scope and stay the residual desk-tools/30 recorded |

**RISK-VALUE:** none of the four risk answers is yes; the brief is `gate: human` because it
changes where a trust control reads its anchors from.

## Evidence
<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This authoring PR claims no future implementation test executed. -->

## Review
Gate: human, decision at creation, not yet answered. The implementation PR needs a recorded
answer cited by `design:`, and a `Security-Review: pass` at its head before any ready flip,
because it changes a trust control's input. Every Verify row is tagged risk-bearing. Rows 1 and 3
are written to fail at the delivery base, and the verifier records them with `verifyrun
--fail-first` against the parent of the delivering merge.
