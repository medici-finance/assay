---
brief: assay:assay:measured-status:05
title: "attribution.go: a same-identity author/verifier pair in a multi-identity repo becomes a hard PROBLEM, not a NOTICE — the independence gate the check exists to establish"
why: >-
  The committer-identity cross-check reads a real second signal — the git identity of a brief's
  authoring commit versus the commit that most recently added its Evidence — but it only ever
  emits a NOTICE, so a same-identity author/verifier pair still passes the verified/done gate.
  The defense-in-depth property the check was built for (security-hardening/27) therefore does
  not actually gate anything. This flips the discriminating case to a hard PROBLEM so
  independence is enforced by the code, not left as a line a reader may skim past.
wave: 1
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1116]
schema: brief-v2
authored: 2026-09-16 by measured-status scoping session
design: DR-independence-gate
gate-why: >-
  This narrows the verified/done trust gate: a state that is a NOTICE today becomes a hard
  PROBLEM (exit 1), so briefs that pass now will red. A human confirms the hard-reject scope
  (multi-identity repo, same-identity author/verifier pair) and accepts that a legitimate
  same-identity re-touch will red until an independent commit or a recorded override cures it.
decision-trigger: creation
decision-issue: 3368
ruling: "https://github.com/medici-finance/assay/issues/3368#issuecomment-6417863766"
sources:
  - "#1116 — attribution.go same-identity author/verifier pair only NOTICEs, never hard-rejects (security-hardening/27 gap)"
  - "statusgen/attribution.go — attributionProblems: the committer-identity cross-check block calls only addNotice(...); add(...) is reachable only from the token-level checks"
  - "statusgen/attribution_test.go — TestAttributionIdentityCrossCheckMultiIdentity / ...SingleIdentityInconclusive assert the same-identity pair is NOT a problem today"
  - "freshness-checked 2026-09-16 @ e9fa19d3 — the len(sameIdentity)>0 branch is addNotice; no add(...) call exists in the committer-identity block"
exec-tier: strong
exec-tier-why: changes a trust-gate verdict from advisory to blocking on a live board; a mis-scoped reject reds every brief in a single-identity checkout
domain: complicated
value: high
version: 1
id: 2cb59a99-aca1-47f2-be84-60be1a432231
---

# Brief 05 — Same-identity author/verifier becomes a hard reject

## Context
files:
- `statusgen/attribution.go` — `attributionProblems`, the committer-identity cross-check block and its `sameIdentity` resolution (currently `addNotice`).
- `statusgen/attribution_test.go` — the two cross-check tests asserting today's NOTICE-only behaviour.
- `statusgen/testdata/attribution/` — the brief fixtures the tests read.
- `statusgen/brieffile.go` — parses the optional `independence-override:` frontmatter key (Option 3).
- `statusgen/gitinfo.go` — the git reads the override is judged on: the commit that last wrote the committed override line (through the isolated provenance read, so runner git configuration cannot move it), that commit's own author email and date, and the Evidence section's text at that commit (compared by content, not by commit ancestry).
- `statusgen/attribution_override_test.go` — the override path's tests, including that an override committed under the excused identity's own author email is refused, with or without a `.mailmap`.
- `schemas/brief-v1.json`, `schemas/brief-v2.json`, `schemas/brief-v3.json` (and their `statusgen/schemas/` copies) — type the `independence-override` key as a string.
- `statusgen/attribution-mutations.json` — the mutation spec for the override's refusal conditions (muhar).
- `spec/brief-v1.md` — §3.2 row for the `independence-override` key.
facts:
- three-state read MUST be preserved: one git identity behind every checked brief is
  INCONCLUSIVE (commit metadata cannot tell author from verifier) and stays a NOTICE; MULTIPLE
  identities with a same-identity author/verifier pair is the DISCRIMINATING case that becomes a
  hard PROBLEM; git-unreadable stays the loud could-not-check NOTICE it is today.
- the token-level checks (`selfVerificationReason`, `evidenceHasIndependentRow`) already
  `add(...)` hard problems; this brief brings the committer-identity signal up to the same force
  for its discriminating case only — it does not touch the token checks.
- the two existing tests assert the NOTICE-only behaviour and MUST be updated to assert the hard
  PROBLEM for the multi-identity same-pair case (and the still-NOTICE for the inconclusive case).
- `statusgen` is its own Go module (`statusgen/go.mod`, no root `go.mod` in this repo); tests
  run from `statusgen/` (`cd statusgen && go test . …`), not from the repo root.
- this brief is inherently fail-first: `TestAttributionIdentityCrossCheckMultiIdentity` asserts
  NOTICE-only today and is being changed to assert a hard PROBLEM instead, so simply running the
  updated test against the unflipped `addNotice(...)` branch (before Task step 1 is applied)
  already demonstrates the red-before state — no separate mutation is needed.
- freshness-checked 2026-10-25: the Option 1 hard PROBLEM is already on main (the escalated,
  confirmed same-identity pair calls `add(...)`; the single-identity and git-unreadable cases stay
  NOTICEs). The ruling chose Option 3, so the remaining delta is the recorded override path
  (an override naming a human): an `independence-override: "human:<name> YYYY-MM-DD <reason>"` frontmatter value,
  honored only when the name is a CURRENT human who is not the author, the brief is outside any
  declared fixture corpus, the committed line was last written by a commit whose git author email
  differs from the identity it excuses, the date is no more than one day after that commit's
  committer date, and the Evidence
  section has not changed since that commit. Anything else keeps the hard PROBLEM and says why the
  override was refused.
- the offline check reads git metadata, which the committer sets. It refuses an override committed
  under the excused identity's own author email; it does not prove the named human authorized it.
  The control that binds the name to the human is `--corroborate` on the PR that adds the override,
  and it binds only where that lane runs.

## Human decision
A verification is supposed to be independent: the person or agent who checks a piece of work
should not be the same one who did it. The tool can already read a second, mechanical signal
for this — it compares who committed the original work against who most recently committed the
record that says the work was verified. Right now, when those are the same identity, the tool
only prints a soft notice, and the work can still be marked verified and done. This asks
whether that soft notice should become a hard failure that blocks the verified and done
status, in the one case where the signal is trustworthy: a repository that has several distinct
committer identities, where the original work and its verification record were nonetheless
committed under one and the same identity. A repository with only a single identity cannot tell
the two roles apart at all and would stay a soft notice.

Options:
1. **Make it a hard failure (recommended)** — in a multi-identity repository, a same-identity
   author-and-verifier pair blocks verified/done until an independent commit, or a recorded
   human override, cures it. Consequence: some items that pass today will fail until a genuinely
   independent act is recorded. This is the property the check was built to enforce.
2. **Leave it a soft notice** — nothing that passes today starts failing, but independence
   remains unenforced and a same-identity pair can still reach verified/done. Consequence: the
   known weakness this addresses stays open.
3. **Hard failure, but also require an explicit override path** — same as option 1 plus a
   recorded, human-authorized override for the legitimate same-identity re-touch case, so a
   genuine post-verification fix has a sanctioned way through.

Default if no answer: none — blocks until answered (this narrows a trust gate and must not be defaulted).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. In `attributionProblems`, change the `len(sameIdentity) > 0` resolution for a MULTI-identity
   repo from `addNotice(...)` to `add(...)` (a hard PROBLEM), keeping the message content
   (which briefs, which identity). Leave the single-identity inconclusive branch and the
   git-unreadable branch as NOTICEs.
2. Update `TestAttributionIdentityCrossCheckMultiIdentity` to assert the same-identity pair now
   produces a PROBLEM, and keep `...SingleIdentityInconclusive` asserting a NOTICE (no problem).
3. Add/confirm a fixture proving the multi-identity same-pair case reds and an independent-pair
   case stays green.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `cd statusgen && go test . -run TestAttributionIdentityCrossCheck -count=1` | exit 0; output contains "ok" |
| 2 | check | `cd statusgen && go test . -run 'TestAttributionIdentity' -count=1 -v 2>&1 \| grep -q 'PASS'` | exit 0 (both the hard-reject and the inconclusive-NOTICE cases ran and passed) |
| 3 | check | `cd statusgen && go vet ./...` | exit 0 (the flipped branch compiles and vets clean) |
| 4 | check | `cd statusgen && go test . -count=1` | exit 0 (the whole statusgen suite is green with the flip) |
| 5 | check +flow | `cd statusgen && go test . -run 'TestAttributionIdentityOverride\|TestIndependenceOverride\|TestAttributionIdentityNoOverrideStaysHard' -count=1 -v 2>&1 \| grep -c -- '--- PASS'` | ≥ 10 — the honored override, every refusal (self-serve with and without a `.mailmap`, recorder read independent of runner git configuration, self-edit, stale, uncommitted, working-tree edit, fixture corpus, postdated, former/unknown human, author, malformed) and the no-override default all ran and passed |
| 6 | check +mutation | `m=$(mktemp -d)/muhar && (cd tools/desk && go build -o "$m" ./cmd/muhar) && cd statusgen && "$m" -spec attribution-mutations.json` | exit 0; "Harness healthy: baseline GREEN, positive control CAUGHT."; every override refusal mutation CAUGHT, "0 NOT CAUGHT, 0 could-not-mutate" |

## Evidence
<!-- appended at implementation time by a non-implementer -->
### Verification — VERIFY: FAIL (row 2, check definition) — 2026-10-27

Non-implementer verification on merged main 47ef38f961aa1006ff296034d118630efdc177cb, admitted by the dispatcher attestation on #4578. Delivery: squash merge of #4480 (718a1b6f781a), an ancestor of the observed source; its parent 5ec829c1fc1e is the fail-first base. Gate: human; all four risk answers no, and the witness tool selects no risk-bearing row, so every row is recorded not-selected at base. Rows ran on the host (darwin-arm64) under the witness's default pipefail row shell, offline (KUBECONFIG=/dev/null), statusgen v1.0.63; no row is check:ci, so the container lane does not apply.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `cd statusgen && go test . -run TestAttributionIdentityCrossCheck -count=1` | pass exit=0 | sha256:24dc52478461 | 2026-10-27 | assay-verifier-app[bot] @ 47ef38f961aa (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=47ef38f961aa |
| 2 | `cd statusgen && go test . -run 'TestAttributionIdentity' -count=1 -v 2>&1 \| grep -q 'PASS'` | fail exit=141 | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 47ef38f961aa (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=47ef38f961aa |
| 3 | `cd statusgen && go vet ./...` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 47ef38f961aa (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=47ef38f961aa |
| 4 | `cd statusgen && go test . -count=1` | pass exit=0 | sha256:c76bdc1d7799 | 2026-10-27 | assay-verifier-app[bot] @ 47ef38f961aa (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=47ef38f961aa |
| 5 | `cd statusgen && go test . -run 'TestAttributionIdentityOverride\|TestIndependenceOverride\|TestAttributionIdentityNoOverrideStaysHard' -count=1 -v 2>&1 \| grep -c -- '--- PASS'` | pass exit=0 | sha256:9961d158a7e0 | 2026-10-27 | assay-verifier-app[bot] @ 47ef38f961aa (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=47ef38f961aa |
| 6 | `m=$(mktemp -d)/muhar && (cd tools/desk && go build -o "$m" ./cmd/muhar) && cd statusgen && "$m" -spec attribution-mutations.json` | pass exit=0 | sha256:d84f39b97b7a | 2026-10-27 | assay-verifier-app[bot] @ 47ef38f961aa (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=47ef38f961aa |
Verification-Attestation: medici-finance/assay#4578 run=0df8c037094cb7ce9eff3b5f1dc754e622cd53b31206b2470231e7525f01896f source=47ef38f961aa1006ff296034d118630efdc177cb brief=docs/streams/measured-status/brief-05-attribution-hard-reject.md model=claude-opus-5-5 tier=strong

**VERIFY: FAIL** — 5 of 6 rows pass; row 2 fails as written (exit 141, a check-definition defect, not an implementation defect).

Observed key lines (the witness above records only an output hash; these lines come from the verifier's own reruns of the exact commands under bash -o pipefail on the same source, 2026-10-27):
- Row 1: exit 0; "ok github.com/medici-finance/assay/statusgen".
- Row 2: exit 141 in both witness runs. grep -q exits at its first match and go test then takes SIGPIPE writing the rest of its -v output; pipefail turns the pipeline status into 141. Supplemental, no acceptance credit: plain bash exits 0 on 2 of 2 runs and pipefail bash exits 141 on 2 of 2 runs; the same -v output carries 22 "--- PASS" lines and no "--- FAIL", including TestAttributionIdentityCrossCheckMultiIdentity and TestAttributionIdentitySingleIdentityInconclusive. The tests pass; the row as written cannot. Same class as #4170 (measured-status/01 row 4) and #2241 (measured-status/04); class tracker #3038.
- Row 3: exit 0, no output.
- Row 4: exit 0 (whole statusgen suite green).
- Row 5: exit 0; prints 28 (Expect at least 10).
- Row 6: exit 0; "Harness healthy: baseline GREEN, positive control CAUGHT."; "Totals: 21 caught, 0 NOT CAUGHT, 0 could-not-mutate."

What must change: the brief's owner amends row 2 to a pipefail-safe reader, for example `cd statusgen && go test . -run 'TestAttributionIdentity' -count=1 -v 2>&1 | grep -c -- '--- PASS'` with Expect "prints a count of at least 1, exit 0"; then a non-implementer re-verifies. The lifecycle row is left unchanged here; the brief is gate: human.

Risk-bearing values. Risk metadata is present and all four answers are no, including irreversible: no. Enumerated over the 718a1b6f781a diff to the statusgen Go sources (attribution.go, gitinfo.go, brieffile.go), re-located at 47ef38f961aa:
- override postdate slack = recordedOn.AddDate(0, 0, 1) (one day) @ statusgen/attribution.go:825
- override grammar = `^human:([0-9A-Za-z_]+)\s+(\d{4}-\d{2}-\d{2})\s+(.*)$` @ statusgen/attribution.go:721
- override date layout = "2006-01-02" @ statusgen/attribution.go:738, 820, 821
- recorder identity binding = git log -1 --no-use-mailmap --format=%ae%x00%cs (author email, mailmap ignored) @ statusgen/gitinfo.go:468-469

Ranked: the recorder identity binding first (a wrong binding would let the excused identity self-serve its own override and reopen the trust gate this brief closes), the postdate slack second, the grammar and date layout last. All are reversible by an edit and a release; every refusal fails closed to the hard PROBLEM.

RISK-VALUE: DERIVED — recorder identity = %ae with --no-use-mailmap @ statusgen/gitinfo.go:468 — the brief's Context facts require the line to be last written by a commit whose git AUTHOR email differs from the excused identity; author email (not committer, which a squash merge sets to the forge) is that field, and ignoring .mailmap stops a committed mailmap entry from aliasing the excused identity to another name.
RISK-VALUE: DERIVED — postdate slack = AddDate(0, 0, 1) @ statusgen/attribution.go:825 — the brief's facts pin "no more than one day after that commit's committer date"; the override date is a human's local calendar date and %cs is the committer's local calendar date, which for one instant differ by at most one day whenever the two offsets are within 24 hours of each other. Only a pair at the opposite extremes of the offset range (a 26-hour span) can differ by two days, and that case fails closed.

The verifier ran no code change and edited only this Evidence section.

## Review
Gate: human (from frontmatter). Human gate is MANDATORY — this narrows a trust gate. Reviewer
records verdict + date in the stream README table.
