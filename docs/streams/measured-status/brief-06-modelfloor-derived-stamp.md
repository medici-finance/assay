---
brief: assay:assay:measured-status:06
title: "model-floor: derive dispatch authority from the dispatch stamp, and give a first-class re-stamp path — stop refusing verdicts by guessing at a label actor's login"
why: >-
  The model-floor decides whether a PR's dispatched-* label counts as attestation by looking at
  who APPLIED the label. A label applied by a trusted LOGIN rather than a trusted dispatcher
  slug reads Indeterminate and the floor refuses to post a verdict, silently stranding real,
  legitimate draft PRs. Authority should be DERIVED from a verifiable stamp the dispatcher
  applies as its own bound identity — and where a legacy label lacks that stamp, there should be
  a sanctioned re-stamp path, not a reader's guess about a login.
wave: 1
depends: []
unblocks: []
effort: M
gate: human
decision-issue: 3369
ruling: "https://github.com/medici-finance/assay/issues/3369#issuecomment-6923924153"
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [336]
schema: brief-v2
authored: 2026-09-16 by measured-status scoping session
design: DR-independence-gate
gate-why: >-
  This changes what the model-floor accepts as dispatch authority — a trust-boundary control. A
  human confirms the derivation (authority read from the stamp, could-not-derive fails closed)
  and rules on whether a trusted-login label is honored at all or only re-stamped through the
  sanctioned path, since widening accepted authority is a security-relevant decision.
decision-trigger: creation
sources:
  - "#336 — model-floor refuses verdicts on PRs whose dispatched-* labels were applied by a trusted login, not a trusted dispatcher slug"
  - "tools/desk/internal/deskkit/modelstamp.go — DispatcherRoles / IsDispatcherLogin: the roster-derived set of actors whose stamp counts; an unbound actor reads Indeterminate (fail-closed)"
  - "tools/desk/internal/deskkit/modelstampactor_test.go — the actor-check tests"
  - "freshness-checked 2026-09-16 @ e9fa19d3 — IsDispatcherLogin returns false for a trusted login that is not a dispatcher slug, so a legacy label reads Indeterminate->refuse"
exec-tier: strong
exec-tier-why: changes the trust-boundary the reviewer verdict lane depends on; too loose admits an unattested tier, too strict keeps the backlog stranded
domain: complicated
value: high
version: 1
id: 59e98b33-3177-47e6-94e9-70ca852738a4
---

# Brief 06 — Derive dispatch authority from the stamp, plus a re-stamp path

## Context
files:
- `tools/desk/internal/deskkit/modelstamp.go` — `DispatcherRoles`, `IsDispatcherLogin`, `AttestedModelStampOf` and the Indeterminate->refuse actor check.
- `tools/desk/internal/deskkit/modelstampactor_test.go` — the actor-check tests.
facts:
- today the floor trusts a stamp only when the label ACTOR's login is a bound dispatcher slug
  (`example-worker-app`, `example-reviewer-app`); a trusted human LOGIN (`example-dispatch-login`)
  applying the same label reads Indeterminate and the verdict is refused.
- the durable fix is a first-class re-stamp/migration verb that re-applies the label under a
  bound dispatcher slug (keeping the floor's derivation intact), NOT relaxing the floor to trust
  any login — honoring a login is at most an interim, human-authorized operator action.
- fail-closed is the invariant: an authority that cannot be derived from a verifiable stamp
  reads could-not-verify and is refused, never rounded up to trust (an unconfigured roster
  vouches for nobody).
- `tools/desk` is its own Go module; tests run from `tools/desk/`.

## Human decision
When work is dispatched to an automated reviewer, the system attaches a label recording which
model tier was chosen. A safety check later refuses to accept a review verdict unless it can
confirm that label came through the sanctioned dispatch path — and today it confirms this by
checking whether the account that attached the label is one of the system's own dispatcher
identities. Some real, legitimate items in the backlog had their label attached by a trusted
human account instead, which the check cannot recognize, so their verdicts are refused and they
cannot progress — even though the work is genuine. This asks how to let those items through
without weakening the check for everyone.

Options:
1. **Add a sanctioned re-stamp path (recommended)** — provide a first-class way to re-attach the
   label under a proper dispatcher identity, so the safety check keeps deriving authority from a
   verifiable stamp exactly as before, and the backlog is cleared by re-stamping rather than by
   trusting a human account. Consequence: a one-time operation on the affected items; the check
   itself is unchanged and stays strict.
2. **Also accept labels attached by a trusted human account** — widen what the check counts as
   authority to include trusted human accounts, not only dispatcher identities. Consequence:
   the backlog clears with no per-item action, but the set of accounts that can vouch for a tier
   grows, which is a security-relevant loosening. **`design: DR-independence-gate`'s second
   alternative already weighs and rules this out as the primary control** — the record calls
   honoring a trusted login "at most an interim operator action a human authorizes, recorded,
   not a code default." Choosing this option as a standing code default AMENDS that record and
   must update `DR-independence-gate` in the same motion, not leave the two disagreeing.
3. **Both** — clear the current backlog with a one-time, explicitly human-authorized acceptance,
   and add the re-stamp path as the durable mechanism so the loosening is not permanent. **This
   is the shape `DR-independence-gate` itself describes as sanctioned** — an interim,
   human-authorized, RECORDED operator action, never a silent code default — so choosing it is
   consistent with the record as written; no amendment needed as long as the acceptance is a
   recorded, one-time act rather than a standing widened check.

Default if no answer: none — blocks until answered (this changes a trust boundary and must not be defaulted).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Implement the ruling. For the recommended path: add a re-stamp verb/function that re-applies
   a `dispatched-*` label under a bound dispatcher slug, and keep `IsDispatcherLogin` /
   the actor check deriving authority from the stamp unchanged. If the ruling widens accepted
   authority, gate that behind an explicit, roster-configured allowance — never a silent default.
2. Preserve fail-closed: an authority that cannot be derived reads could-not-verify and is
   refused; an unconfigured roster vouches for nobody.
3. Cover both cases in `modelstampactor_test.go`: a properly-stamped label is accepted, and an
   un-derivable actor is refused (and, if a re-stamp verb is added, a re-stamped label is then accepted).

## Verify (executable — no prose-only DoD items)
| # | Class | Env | Command | Expect |
|---|-------|-----|---------|--------|
| 1 | check:ci | tree | `cd tools/desk && go test ./internal/deskkit/ -run 'TestModelStamp' -count=1` | exit 0; output contains "ok" |
| 2 | check:ci | tree | `cd tools/desk && go test ./internal/deskkit/ -run 'TestModelStamp.*Actor' -count=1 -v 2>&1 \| grep -q 'PASS'` | exit 0 (the accepted-stamp and refused-actor cases both ran and passed) |
| 3 | check:ci | tree | `cd tools/desk && go test ./internal/deskkit/ -count=1` | exit 0 (the whole deskkit suite is green) |
| 4 | check:ci | tree | `cd tools/desk && go vet ./internal/deskkit/` | exit 0 |
| 5 | check | host | `bash docs/streams/measured-status/verify.d/06-allowance-backlog.sh` | exit 0, and every output line is `examined <N> PRs in <owner>/<repo>`: the allowance backlog has cleared. The script reads the repository set from the deployment's `ASSAY_ALLOWED_REPOS`, the write set where the floor admits verdicts (read from the environment, so the set stays out of this tree; an `owner/*` entry is enumerated from the forge). It prints one `examined` line per repository, and a `PR <owner>/<repo>#<n> <label>` line for each open PR whose `dispatched-*` label (matched case-insensitively, as the floor matches it) still stands under a non-App account. Any `PR` line makes it exit 1 after every repository is examined: the backlog remains, and the row fails. `ASSAY_ALLOWED_REPOS` unset, empty, naming no repository, or holding a malformed entry exits 2 with a `could-not-check:` line; that, and any other non-zero exit with no `PR` line (a forge or tool failure), mean could-not-check, never a pass. The row needs forge read access to every repository in the set, so it is classed `check` / `host`: a witness runs it on a machine holding that access and that configuration, and the scheduled CI re-run skips it. Row 6 tests this script offline. When it passes, the allowance can expire (see DR-independence-gate, "Expiry of the allowance"), and removing the roster entry is the driver's act. |
| 6 | check:ci | tree | `bash docs/streams/measured-status/verify.d/06-allowance-backlog.test.sh` | exit 0; output is `ok: allowance-backlog check` (row 5's script, run against a stub `gh` serving planted label timelines: a user-standing stamp exits 1 and names its PR, a cleared set exits 0 with only `examined` lines, a case-variant stamp counts, an `owner/*` entry is enumerated, and a malformed set or a forge failure never exits 0) |

## Evidence
<!-- appended at implementation time by a non-implementer -->

Non-implementer verification on merged main bec90193ce916b2d5b11433f25970867a1b7ef95 (delivered by #1727, merge a5bacb1237f8, and #6688, merge 210c60dd23ed, which added the row 5 and 6 scripts). Gate: human; all four risk answers no. Tool: installed statusgen v1.0.87. Rows 1, 2, 3, 4 and 6 are `check:ci` rows. The host is darwin and cannot sandbox them with `unshare --net`, so they ran in a closed container-netns admission bracket (`--network none`, read-only tree, read-only module cache, `GOPROXY=off`). `--bracket-digest` reports `verdict=match`. The witness file's bytes follow verbatim.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestModelStamp' -count=1` | pass exit=0 sandbox=container-netns | sha256:a3edb8d75eb5 | 2026-11-18 | assay-verifier-app[bot] @ bec90193ce91 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run 'TestModelStamp.*Actor' -count=1 -v 2>&1 \| grep -q 'PASS'` | fail exit=141 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-18 | assay-verifier-app[bot] @ bec90193ce91 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./internal/deskkit/ -count=1` | fail exit=1 sandbox=container-netns | sha256:bf5b6d6c10d0 | 2026-11-18 | assay-verifier-app[bot] @ bec90193ce91 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go vet ./internal/deskkit/` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-18 | assay-verifier-app[bot] @ bec90193ce91 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 6 | `bash docs/streams/measured-status/verify.d/06-allowance-backlog.test.sh` | pass exit=0 sandbox=container-netns | sha256:ff3d913b868a | 2026-11-18 | assay-verifier-app[bot] @ bec90193ce91 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:1e8f3139667c5b2860658d062ab306b15095bbf06895404673ef6a551831135d
Container-Image: sha256:3cfd931fc8fb2277df25eafbc68eccfe84ce7d7b92379b927b8c1841d99854d1
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6910 run=196e84e577c82d259c1ac67f6d35922c063cd3aec0b43fdd1461b1cd20464887 source=bec90193ce916b2d5b11433f25970867a1b7ef95 brief=docs/streams/measured-status/brief-06-modelfloor-derived-stamp.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=196e84e577c82d259c1ac67f6d35922c063cd3aec0b43fdd1461b1cd20464887 source=bec90193ce916b2d5b11433f25970867a1b7ef95 tree=sha256:1e8f3139667c5b2860658d062ab306b15095bbf06895404673ef6a551831135d witness=sha256:e21ab575324f31e3925d56cb0266d42b1bf3b017bf63840cc6fee16553eac6db opened=2026-11-18T15:23:51Z closed=2026-11-18T15:30:25Z before=receipt after=check-verifier

Row 5 is `check/host`, so it ran on the host from this admitted home, with a read credential for the verifier App and the configured repository set.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 5 | `bash docs/streams/measured-status/verify.d/06-allowance-backlog.sh` | exit 0, only `examined <N> PRs in <owner>/<repo>` lines | could-not-run as admitted. `statusgen verifyrun --rows 5 --dry-run` gave `fail exit=2`, output `sha256:ecbf62737f46`. That digest is exactly the script's line `could-not-check: ASSAY_ALLOWED_REPOS is unset or empty`, because an admitted run removes ASSAY_* inputs from every row's environment. Under the row's own Expect, exit 2 is could-not-check. | 2026-11-18 | assay-verifier-app[bot] @ bec90193ce91 (claude-opus-5-5) (on-behalf-of human:ian) |

Supplemental host observations. These carry no acceptance credit and discharge no row.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 5 (supplemental) | the row 5 command run directly in `bash` outside admission, with only ASSAY_ALLOWED_REPOS and the read credential supplied, after the script was reviewed as read-only (`gh repo list`, `gh pr list`, `gh api .../issues/<n>/events`) | exit 0, only `examined` lines | exit 0. Ten `examined <N> PRs in <owner>/<repo>` lines, one for each of the 10 configured repositories, covering 45 open PRs with a `dispatched-*` label in total (28, 11, 6 and seven zeros). No `PR <owner>/<repo>#<n> <label>` line and empty stderr: in the run's read, the backlog has cleared. Every configured repository was readable. | 2026-11-18 | assay-verifier-app[bot] @ bec90193ce91 (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 (diagnostic) | in the same network-off container and mounts, `cd tools/desk && go test ./internal/deskkit/ -run 'TestModelStamp.*Actor' -count=1 -v` with no pipe; then on the host the row 2 pipeline under `bash -o pipefail -c`, and the `grep -c '^--- PASS'` form | — | the unpiped run exits 0 with six `--- PASS: TestModelStampActor…` lines (HonorsAllowanceLogin, RefusesUnvouchedLogin, AllowanceIsExplicitNeverSilent, RestampRestoresStrictDerivation, AllowanceParseRules, RefusalNamesTheAllowance) and `ok`. The row 2 pipeline exits 141 on the host too. The `grep -c` form prints `6` and exits 0. The tests pass. The row fails because `grep -q` exits at its first match and `go test` then takes SIGPIPE under pipefail. This is the #4170 / #2241 class. | 2026-11-18 | assay-verifier-app[bot] @ bec90193ce91 (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 (diagnostic) | in the same container, `cd tools/desk && go test ./internal/deskkit/ -count=1` with full output kept; then the same command on the host with `GOPROXY=off` and the same module cache | — | in the container, exit 1 from environment-bound tests only. TestCredHelperChainRealGit, TestCredChainGitParity and TestCredChainHostlessMatchesGit fail with `fatal: gitfile does not point to a valid repository: /work/.git`, because the linked-worktree home's `.git` is not visible in the container (#4327). TestReviewCallerFlow and TestReviewCallerRollback fail with `could-not-check: ... the working tree's .git is not a usable repository` (#4322). TestOverrideIdentityResolvesRosterLogin panics with a nil dereference because `user.Current()` fails for the mapped container uid (#4329). No model-stamp test failed. On the host the whole suite gives `ok ... deskkit 337.277s` and exits 0. | 2026-11-18 | assay-verifier-app[bot] @ bec90193ce91 (claude-opus-5-5) (on-behalf-of human:ian) |

Row accounting, 6 required rows: rows 1, 4 and 6 pass in the bracket. No obligation on the delivered behaviour was observed failing. Row 2: the witness cell `fail exit=141` stays as recorded, but it measures the row's command rather than the deliverable. `grep -q` exits on its first match, and `go test` then takes SIGPIPE under pipefail. The obligation the row tests, that the TestModelStamp*Actor tests pass, is observed present: six `--- PASS` lines and `ok` in the unpiped diagnostic, plus row 1's pass over a superset of those tests. So row 2 is a check-definition defect of the #4170 SIGPIPE class. It is not an obligation failure. It needs a pipefail-safe amendment and a fresh verify. Row 3: the bracket's `fail exit=1` comes only from tests bound to the container lane (#4327, #4322, #4329). No model-stamp test failed, and the suite is green on the host (uncredited), so row 3 counts as could-not-run (environment). Row 5 could not run as admitted (ASSAY_* scrubbed), so it is could-not-check; its supplemental direct run is clean, uncredited. The table is protected, but the delivering commit a5bacb12 does not touch this brief, and re-derivation reports every row from base. No Verification-Table marker applies.

Risk-bearing values. Risk metadata is present and all four answers are no. The enumeration covered the non-test Go and roster files in #1727's diff (tools/desk/internal/deskkit/{trust,modelstamp,modelfloor,rosterconfig}.go, tools/desk/cmd/deskrestamp/*.go, statusgen/rosterconfig.go) plus the dispatcher-role constants that the new stamp-authority predicate reads. Ranked by irreversibility:
1. RestampDriverCutoffRFC3339 = "2026-09-27T00:00:00Z" @ tools/desk/internal/deskkit/trust.go:208. If it is wrong, re-stamps vouch for labels the ruling excludes, and verdicts already posted on that trust cannot be withdrawn by a redeploy.
2. EnvStampTrustedLogins = "ASSAY_STAMP_TRUSTED_LOGINS" @ tools/desk/internal/deskkit/rosterconfig.go:577, paired with scanEnvStampTrustedLogins = "ASSAY_STAMP_TRUSTED_LOGINS" @ statusgen/rosterconfig.go:249. A drift between them leaves the allowance unread on one side, which fails closed to the dispatcher arm. Reversible.
3. DispatcherRole = "desk" @ tools/desk/internal/deskkit/modelstamp.go:816 and ReviewDispatcherRole = "reviewer" @ tools/desk/internal/deskkit/modelstamp.go:838. These are the roles whose bound Apps are stamp authority. They predate this delivery.
4. stampLabelColorHex = "ededed" @ tools/desk/cmd/deskrestamp/forge.go:29. Cosmetic. Reversible; no derivation needed.

RISK-VALUE: DERIVED — RestampDriverCutoffRFC3339 = "2026-09-27T00:00:00Z" @ tools/desk/internal/deskkit/trust.go:208 — the driver's ruling (https://github.com/medici-finance/assay/pull/1727#issuecomment-5860170351) names this exact instant: re-stamps may vouch only for dispatched-* labels applied by the bless login "before 2026-09-27T00:00:00Z". The constant is byte-identical to it. The comparison is strict `at.Before(cutoff)` and a parse failure refuses (trust.go:266-267), which matches "before". The ruling was posted at 2026-09-27T21:54:19Z, after the cutoff, so no label applied after the ruling can qualify. It is compiled, not a roster key, so an operator cannot move it.
RISK-VALUE: DERIVED — EnvStampTrustedLogins = "ASSAY_STAMP_TRUSTED_LOGINS" @ tools/desk/internal/deskkit/rosterconfig.go:577 — the desk loader and the statusgen loader read one roster file, so each key must be spelled identically in both. statusgen/rosterconfig.go:249 carries the same literal, and #1727 adds the key to statusgen/testdata/roster_coupling.json, which pins the pair. The ASSAY_ prefix follows the roster's key namespace. An unset or empty key vouches only for the dispatcher arm (modelstamp.go:886), so an absent allowance fails closed.
RISK-VALUE: NAMED, NOT DERIVED — DispatcherRole = "desk" @ tools/desk/internal/deskkit/modelstamp.go:816; ReviewDispatcherRole = "reviewer" @ tools/desk/internal/deskkit/modelstamp.go:838 — the correct values are the roster role names bound to the two dispatching Apps. The code comment gives the reason for admitting the second role (dispatcher equals applier for the review lane), but I found no spec or ruling that fixes the role vocabulary to derive these literals from. They predate this delivery.

**VERIFY: BLOCKED** — 3/6 rows pass (1, 4, 6), and no obligation on the delivered behaviour was observed failing. Row 2 is check-definition: `grep -q` SIGPIPE under pipefail, the #4170 class; the six Actor tests pass. Row 3 is could-not-run (environment): its container-lane faults are #4327, #4322 and #4329, and the host run is green, uncredited. Row 5 is could-not-check (environment): admission scrubs ASSAY_ALLOWED_REPOS, and the supplemental direct run is clean, uncredited.

## Review
Gate: human (from frontmatter). Human gate is MANDATORY — this changes a trust boundary.
Reviewer records verdict + date in the stream README table.
