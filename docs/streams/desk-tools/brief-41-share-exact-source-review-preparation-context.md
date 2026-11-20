---
brief: assay:assay:desk-tools:41
title: Share exact-source review preparation context
why: "Review round and packet builders repeat reads and can read a different head or the allocated worktree instead of the assigned change."
wave: 3
depends: ["desk-tools/38", "desk-tools/39"]
unblocks: ["desk-tools/43", "desk-tools/44"]
effort: M
gate: human
decision-issue: 4523
design: DR-desk-tools-41
ruling: "https://github.com/medici-finance/assay/issues/4523#issuecomment-6923696525"
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-17 by statusgen newbrief
sources:
  - docs/desk-workflow-efficiency-spec.md
  - docs/desk-workflow-efficiency-change-map.md
  - freshness-checked 2026-10-17 @ fb85924c039c27e7d2aa80e055bfce118e46125d (App-authenticated main fetch)
  - 'desk-tools: Share exact-source review preparation context'
id: 8e177504-b12b-4d23-a48b-81e7764a5558
exec-tier: strong
exec-tier-why: Semantic extraction, cost coverage and cross-caller failure behavior require independent contract judgment.
domain: complicated
gate-why: Review/issue observations can expose scoped operational facts; implemented coverage and authority boundaries require the existing human acceptance.
decision-trigger: spec
consumers:
  - "tools/desk/cmd/deskdispatch/: fixed-here"
  - "tools/desk/cmd/deskdispatch/references/review-prompt.md: fixed-here"
  - "plugins/assay/skills/pr-review-desk/SKILL.md: fixed-here"
  - ".cursor/skills/pr-review-desk/SKILL.md: fixed-here"
  - ".cursor/assay-managed.json: fixed-here"
---

# Brief 41 — Share exact-source review preparation context

## Context

files: `tools/desk/cmd/deskdispatch/reviewroundread.go` (existing), `tools/desk/cmd/deskdispatch/packet_review.go` (existing), `tools/desk/cmd/deskdispatch/main.go` (existing), `tools/desk/internal/reviewflow/` (planned), `tools/desk/internal/reviewtest/` (planned), `tools/desk/internal/gitcore/` (existing), `changelog/desk-tools-41.md` (planned)

facts: 17 October 2026 at the source pin above. Existing implementation is described in
approved spec §2. All named tests and paths marked planned below are deliverables, not
passing claims. desk-tools/23 remains implemented, /24–25 done, /27 retains its own
verification debt; desktools-v2/09 is verified and /45 implemented. Refresh actual source,
claims and open PRs at pickup. #3248 and #3302 are active CLI ports; #3325 is proposed
supervision guidance and #3305 proposed generic improvement planning. Neither draft is
assumed shipped. The initial blocker is the missing complete observation and attribution,
not the full future runtime.

layering: deskdispatch preparation adapters over shared reviewflow context and gitcore exact objects. Contracts and pure computation remain separated from
credentialed collection/effects; adapters consume values through legal module APIs.
No new shipped service, runtime store or credential owner is authorized. The flow and boundary
mutation cases below enforce this placement.

design-fit:
  owner: deskdispatch preparation adapters over shared reviewflow context and gitcore exact objects
  contract: approved spec PrepareReview/ReviewContext and A4; existing claim/effect preconditions
  retires: duplicate work only after the applicable caller and equivalence acceptance
  weight: verbs +0, flags +1 observation input, refusals +0 authority, rule-text +0 duplicate policy
  why-add: reuse existing contracts/runner/typed reads; a complete observation is required before removing the second read, and a new cache would preserve that duplication

## Read first

- [Approved source](../../desk-workflow-efficiency-spec.md) and [consolidation](../../desk-workflow-efficiency-change-map.md).
- [Delivery waves](README.md#review-efficiency-follow-ons) and existing source files declared above.
- `docs/library-first.md`, `docs/brief-rules.md` and `docs/telemetry.md` for the applicable boundary/consent rules.

## Ground rules

- Preserve exact-head independent verdicts, claims, trust, budgets, audit and consequential rechecks.
- Use fake providers and disposable local Git for acceptance; live activation and collection are separate gates.
- Stop at implemented; independent verification supplies Evidence. Do not rewrite completed brief Evidence.

## Task

1. Add --observation FILE to existing deskdispatch --kit review; advisory review observations supply
   applicable facts without authorizing effects. Construct one typed ReviewContext and pass it to
   round and packet builders. Reuse immutable source facts within preparation before considering
   persistent caching.
2. Key source reuse by provider-qualified identity, full head SHA, path and content/adapter version;
   diff/merge-base keys include both immutable endpoints and relevant algorithm/version. A branch
   name, mtime or allocated worktree HEAD is not a key. Use existing gitcore exact-object reads; no
   shared object-store project.
3. Body/review/check state, permissions, claims and ready/publication inputs remain mutable with
   their existing freshness boundaries. Recheck before consequential operations; retain
   dispatch/model admission, independent review lanes, refusal/hold history and effect
   intent/receipt/unknown behavior. Respect existing file/history budgets and report partial packet
   coverage.
4. Measure complete dispatch plus agent preparation to detect work shifted downstream. Coordinate
   active #3302 parser port. Future desk-review-prepare integration remains the role executor
   owner's cutover; this bridge grants no new custody or writer activation.
5. Update affected command/operator contracts and the declared changelog; regenerate applicable
   documentation with the existing statusgen README generator. No generated global board is branch-
   committed. All planned tests must be wired to actual production paths, not a parallel test
   implementation.

## Verify

Run from the public repository root; each named test is a planned deliverable and its PASS
line is mandatory (zero selected tests is a failure). No authoring-time execution Evidence
is asserted. Each test covers production flow plus the stated wrong-but-plausible mutation.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReviewPrepareFlow$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestReviewPrepareFlow ' "$o")` | exit 0; named PASS. Real dispatch preparation uses one exact-source context for round and packet; an allocated worktree at another head still yields assigned-head bytes, and duplicate immutable reads are counted once. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReviewPrepareInvalidation$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestReviewPrepareInvalidation ' "$o")` | exit 0; named PASS. Head/base/body/review/check/access changes invalidate applicable inputs; a saved observation cannot bypass claim, attestation, authorization or immediate consequential freshness rechecks. |
| 3 | check:ci +dereference | `statusgen --root . --consumers --brief desk-tools/41` | exit 0; implementation consumer routing is corroborated against the actual diff. Out-of-scope judgments remain review obligations. |

## Pre-mortem and detection

| Failure mode | Caught by |
|---|---|
| Adapter produces plausible output while duplicating or omitting work | row 1, independent oracle and actual entrypoint/count instrumentation |
| A partial/unsupported observation or hidden consumer creates a false success | row 2, planted invalid input and explicit refusal/coverage |
| Operational activation or control deletion inferred from source qualification | review-only: review scope and separate existing adoption gates; no live effects in the test harness |

## Architecture pickup

Continue against main `d92df1b1ab898db5f537fab33dae3087114c9fb0` after #4248 merged.
The controlling spec and change map, library-first direction, brief rules, telemetry
contract and semantic-owner index were read at that revision. Spec SHA-256:
`c2aa79b9423289b3484eb81973ae92f5a538dfde015c770938eb250b8790e1ed`;
library-first SHA-256: `9272c94d8ff0fa2f02ddf6ee173e12e267527515c2e9ba88eb92fcb9bf3103af`.
The existing round, gate, packet, CLI and gitcore APIs are inspected; #3248 and
#3302 are merged, so this extends the delivered CLI parser rather than another port.
Reuse `reviewflow` for the pure context/key values, deskdispatch for credentialed
composition, gitcore for exact objects and deskkit for checked provider transport.
The process boundary remains at the existing CLI/custody owner. No SDK gains credentials.

Adapt the stale prompt consumer locator to the shipped embedded reference under
`tools/desk/cmd/deskdispatch/references/`; no parallel prompt owner is created.
Observation v1 contains resolved advisory classification, not raw reviews/checks or
an authenticated approved-policy receipt. Therefore preparation takes source
attribution from it and reads mutable raw inputs live; it cannot remove those reads
by trusting the file. Existing terminal checks are strengthened, not replaced.
The API/CLI/packet consumers are updated here; actual prose/discrete flag adoption
remains desk-tools/43 and full attributed downstream cost remains /44. Existing
readonly packet guidance is retained. No lifecycle/design approval or independent
Evidence is inferred from this implementation checkpoint.

## Evidence

<!-- Independent verifier appends one row per Verify command after implementation. -->

### Container verification — 2026-11-18 (source 1a259b85f7b5)

Observed source: merged main 1a259b85f7b5, in a detached home admitted by `deskdispatch --check-verifier` (attestation medici-finance/assay#6917).

- The delivery is merged PR #4259: two-parent merge 8a6d345ed3c3, first parent 1f579542d540. The first parent is the fail-first base.
- `deskpathguard rederive` at the observed source reports 3 rows and 0 author-added, so the Verify rows run here are the original rows.
- The frontmatter declares `sensitive-data: yes`. Every row is therefore risk-bearing and owes a fail-first witness at the base.

The table directly below is the execution witness. It was produced by `statusgen verifyrun` v1.0.87, non-dry, with `--sandbox=container-netns` and `--fail-first` against the delivering merge, inside a closed admission bracket:

- The bracket was opened on the host at 2026-11-18T15:49:25Z and closed on the host at 16:10:46Z.
- `statusgen verifyrun --bracket-digest` on the closed witness reports `verdict=match`.
- The witness bytes follow unchanged, including the `Admission-Bracket:` line and the tool-written Runner and Base cells.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReviewPrepareFlow$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestReviewPrepareFlow ' "$o")` | pass exit=0 sandbox=container-netns | sha256:93f10218f7d6 | 2026-11-18 | assay-verifier-app[bot] @ 1a259b85f7b5 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=1f579542d540 unproven · head=1a259b85f7b5 |
| 2 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReviewPrepareInvalidation$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestReviewPrepareInvalidation ' "$o")` | pass exit=0 sandbox=container-netns | sha256:a0aaf716d13b | 2026-11-18 | assay-verifier-app[bot] @ 1a259b85f7b5 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=1f579542d540 unproven · head=1a259b85f7b5 |
| 3 | `statusgen --root . --consumers --brief desk-tools/41` | fail exit=2 sandbox=container-netns | sha256:4321273847e4 | 2026-11-18 | assay-verifier-app[bot] @ 1a259b85f7b5 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=1f579542d540 red rc=2 · head=1a259b85f7b5 · delivering=8a6d345ed3c3 |
Tree-Digest: sha256:19179b2cba9b5a822c9cfdb867bd8fcbf7ae470db462e893cb8805b79cbe1453
Container-Image: sha256:c67baa7a3c8ba3d514128f31340489146525a50c29ebb641a68899c650d354a1
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6917 run=d97e495d3edb3a0f3eebe81241cef14a6aa4ed11372cafd1f0ec054203fb8004 source=1a259b85f7b59b39d2d6663c7965a9cc0b1c5e6c brief=docs/streams/desk-tools/brief-41-share-exact-source-review-preparation-context.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=d97e495d3edb3a0f3eebe81241cef14a6aa4ed11372cafd1f0ec054203fb8004 source=1a259b85f7b59b39d2d6663c7965a9cc0b1c5e6c tree=sha256:19179b2cba9b5a822c9cfdb867bd8fcbf7ae470db462e893cb8805b79cbe1453 witness=sha256:2d1040237625d3ced6a5ebf1e2d7d6693155cb3c92f3f0ef793ebc3ad9c96dd6 opened=2026-11-18T15:49:25Z closed=2026-11-18T16:10:46Z before=receipt after=check-verifier

Environment: the container used the published verify-runner image pinned by digest (sha256:c67baa7a3c8b, linux/arm64), which carries statusgen v1.0.87. It ran with no network and `GOPROXY=off`, against the module cache that the bracket open provisioned from the head tree (the Module-Cache digest above). The container run lasted from 15:53:32Z to 15:55:01Z.

Disclosure: verifyrun refused a first container invocation at 15:52:55Z before any row ran. The verifier had passed Git configuration environment variables into the container, and they select a native Git profile that the library read path refuses for the delivering commit. That was a verifier invocation error, not a deliverable or tool fault. It ran no rows, wrote no witness and is not counted as an attempt. The run witnessed above used only the container flags printed by the bracket open.

#### Per-row accounting

| # | At head (source 1a259b85f7b5) | Fail-first at base 1f579542d540 | Disposition | Date | Runner |
|---|---|---|---|---|---|
| 1 | pass, exit 0, named PASS line present | unproven: the row could not run at the base in the container | BLOCKED: the fail-first obligation is not witnessed (environment capability, see below). No deliverable obligation was observed failing. | 2026-11-18 | verifier App, claude-opus-5-5 (strong) |
| 2 | pass, exit 0, named PASS line present | unproven: the row could not run at the base in the container | BLOCKED: same as row 1 | 2026-11-18 | verifier App, claude-opus-5-5 (strong) |
| 3 | exit 2: `--consumers: COULD-NOT-CHECK`. The brief is not in the diff against the observed source, and the delivery could not be derived: the committed Evidence has no Verification-Table marker naming it, and the history cannot be searched through the library read path. The tool asks for `--delivery` and `--base`. No routing claim was corroborated or disproved. | red, rc 2 | could-not-check: a Verify-command definition defect (a consumers row with no pinned delivery or base, the class tracked in #4560). It is not a deliverable failure. | 2026-11-18 | verifier App, claude-opus-5-5 (strong) |

Why rows 1 and 2 are unproven at the base: the bracket open provisions the module cache only for the modules named by the head tree's go.sum files. The base tree's Go module names modules the head no longer requires, for example golang.org/x/crypto v0.56.0 and golang.org/x/mod v0.38.0. With `GOPROXY=off`, the base build cannot resolve them, so the base row cannot run. This is a gap in the admitted lane's capability. It says nothing about the delivered code.

#### Supplemental observations (no acceptance credit)

These were run on the host outside the admission bracket. They inform the dispositions above and earn no acceptance credit.

| Observation | Command | Result | Date | Runner |
|---|---|---|---|---|
| S1: row 3 with the delivery pinned, at the observed source | `statusgen --root . --consumers --brief desk-tools/41 --delivery 8a6d345ed3c38c0c0efe9d7a10e56b6d8e8b9a88 --base 1f579542d540633425d6fd4fe647d5805f1c9cfb` | exit 0; 5 corroborated, 0 disproved, 0 unchecked. The corroborated consumers are the deskdispatch command, its embedded review prompt, the pr-review-desk skill in both harness skill trees, and the Cursor managed-files manifest. | 2026-11-18 | verifier App, claude-opus-5-5 (strong); host, no credit |
| S2: rows 1 and 2 at the base, in a scratch checkout of 1f579542d540 with modules available | the row 1 and row 2 commands, verbatim | both red: `go test` exits 0 with `[no tests to run]`, and the named-PASS grep exits 1. The rows discriminate between base and delivery, but no admitted lane witnessed it. | 2026-11-18 | verifier App, claude-opus-5-5 (strong); host, no credit |

#### Risk values

RISK-VALUE: DERIVED — the exact commit id is 40 lowercase hex characters that must round-trip as a canonical object id @ gitcore/filebounded.go:125. The repository object format is SHA-1, so 40 characters is the full id. This refuses ref names and abbreviations as keys, which the brief requires: a branch name or a worktree HEAD is not the assigned source.
RISK-VALUE: DERIVED — prepared-subject reuse compares head SHA, base SHA, base ref, body, state and the decision label @ tools/desk/cmd/deskdispatch/reviewcontext.go:69. The brief requires head, base and body changes to invalidate. Row 2's mutations exercise this, and it passes at head.
RISK-VALUE: DERIVED — a saved observation whose head or base differs from the live subject is refused ("observed head/base differs from current subject") @ tools/desk/cmd/deskdispatch/reviewcontext.go:158. This keeps an observation file from bypassing the live freshness recheck.
RISK-VALUE: DERIVED — the preparation key version is "review-prepare-v2" @ tools/desk/cmd/deskdispatch/reviewcontext.go:49. It is a key component, so any format change invalidates reuse. Reuse is in-process only.
RISK-VALUE: NAMED, NOT DERIVED — the diff input cap is 4 << 20 (4 MiB) @ tools/desk/cmd/deskdispatch/packet_review.go:50, and the read budget is 60 * time.Second @ packet_review.go:55, also used as the stage budget @ reviewcontext.go:51. Both are reversible tuning values that bound work. Exceeding the diff cap records a stated refusal of the local diff, and running out of the read budget omits the file with a stated reason. Neither yields a silent success.
RISK-VALUE: NAMED, NOT DERIVED — the observation file cap is 16<<20 (16 MiB) @ tools/desk/cmd/deskdispatch/reviewcontext.go:138 and :142, and an oversized file is refused. The diff context is 3 lines @ reviewcontext.go:240, Git's default unified context. Both are reversible. None of the values is irreversible.

**VERIFY: BLOCKED** — rows 1 and 2 pass at source 1a259b85f7b5 in the network-off container witness, inside a closed admission bracket (`verdict=match`), and no deliverable obligation was observed failing. Closure is blocked on two counts. Row 3 is could-not-check because the command does not pin the delivery and base (#4560 class); with them pinned, a host probe outside the bracket corroborates all 5 consumers. Rows 1 and 2 have no admitted fail-first witness because the container module cache cannot build the base. This brief is `sensitive-data: yes`, so every row owes that witness.

## Review

Retain the derived frontmatter gate. Approval of the source spec is recorded; this brief
adds implementation acceptance rather than reopening that decision. Existing adopter
writer/custody/equivalence and collection consent gates remain separate.
