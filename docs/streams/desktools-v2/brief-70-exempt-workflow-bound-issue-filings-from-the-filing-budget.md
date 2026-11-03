---
brief: assay:assay:desktools-v2:70
title: Exempt workflow-bound issue filings from the filing budget
why: "The new-issue budget counts every filing alike, so a blocker or question issue that a PR's own review requires waits behind a cap meant for the desk's process-error reports."
wave: 1
depends: []
unblocks: []
effort: M
gate: human
decision-issue: 4072
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
gate-why: "Relaxes a fail-closed brake (proposal 11 lists new-issue caps as brakes). The driver ruling, ratified by the driver's merge of this brief PR (#4069, merged 2026-10-22), names the compensating bound; its implemented values and refusals need human acceptance at done."
issues: []
schema: brief-v2
version: 1
authored: 2026-10-22 by statusgen newbrief
sources:
  - 'desktools-v2: Exempt workflow-bound issue filings from the filing budget'
  - "driver ruling, 2026-10-22, for ratification at merge (its public record is the driver's merge of this gate: human brief PR, #4069, merged 2026-10-22): issues that are part of the regular PR workflow are not counted; the budget bounds only the desk's own process-error reporting"
  - docs/proposals/non-ci-architecture/11-target-architecture.md (§Pacing counts are best effort; brakes fail closed)
  - deskcore/config/knobs.go (budget knobs default 0, counted per role)
  - freshness-checked 2026-10-22 @ 9a2c6ba2bc85558d77c4227984d93a728a7f7e94 (origin/main)
exec-tier: strong
exec-tier-why: "Changes a brake and its audit accounting; the counting, binding checks and refusals need adversarial judgment."
domain: complicated
outcome: none
consumers:
  - "tools/desk/cmd/deskfile/: follow-up desktools-v2/70"
  - "deskcore/config/knobs.go and its tests: follow-up desktools-v2/70"
  - "docs/contracts.md R-new-issue-budget row: follow-up desktools-v2/70"
  - "plugins/assay/skills/ filing-budget wording (verify-desk, pr-review-desk, worker-desk, the-desk, intake-desk, pr-shepherd) and the generated Cursor mirror: follow-up desktools-v2/70"
  - "unbound `deskfile new` budget: out-of-scope (keeps today's per-session rate; moving it onto the per-role issue.file.rate knob is a separate deskcore/config adoption)"
  - "perfrec work references: out-of-scope (perf-v4 has no PR reference kind; this brief records PR refs in deskfile's audit line only)"
---

# Brief 70 — Exempt workflow-bound issue filings from the filing budget

## Context

files: tools/desk/cmd/deskfile/deskfile.go; tools/desk/internal/deskkit/ratelimit.go, tools/desk/internal/deskkit/audit.go, tools/desk/internal/deskkit/audittoolkey.go, tools/desk/internal/deskkit/trailer.go; deskcore/config/knobs.go; docs/contracts.md; plugins/assay/skills/*/SKILL.md (filing-budget passages); docs/proposals/non-ci-architecture/11-target-architecture.md

facts:
- The `new` budget is per session and per repository, not per role: `defaultNewRate = 3` at `tools/desk/cmd/deskfile/deskfile.go:452`, `defaultNewWindow = 24 * time.Hour` at `:455`, read through `ASSAY_DESKFILE_NEW_RATE` / `ASSAY_DESKFILE_NEW_WINDOW` by `newBudgetConfig` (`:465`-`:490`). `checkSessionBudget` (`:554`) counts audit entries with `Tool == "deskfile"`, `Verb == "new"`, the same repo and the same `SessionTag` (`:562`-`:568`). Nothing in the count records why an issue was filed.
- `--force-file` (`:811`) skips the budget check for one filing but the filing is still charged (`:1207`-`:1214`). After the budget, every `new` also passes `deskkit.AllowWriteRepoWide("deskfile", repo)` (`:1216`-`:1221`), the repo-wide outward-write ceiling held at the per-PR cap of 20 per rolling hour (`tools/desk/internal/deskkit/ratelimit.go:47`, `:387`).
- There is precedent for keeping a workflow lane off the daily cap while still auditing it: `VerdictIssueTool = "verifyloop-verdict"` (`ratelimit.go:400`, rationale `:392`-`:399`) records under its own tool key, so `checkSessionBudget` never counts it. `RequireCanonicalToolKey` (`tools/desk/internal/deskkit/audittoolkey.go:206`) refuses an unregistered tool key.
- An audit `Entry` already carries `Tool`, `Verb`, `Repo`, `PR *int`, `HeadSHA`, `Result`, `Detail` and `SessionTag` (`tools/desk/internal/deskkit/audit.go:69`-`:89`). `deskfile attach --kind issue|mr` (`deskfile.go:1378`) is the existing write that posts on a PR.
- `deskcore/config/knobs.go:89` defines `issue.file.rate` (class budget, default 0, per role, earlier hard-coded value 3) and `:91` `issue.file.window`. The table validator refuses a budget or rate knob that does not default to 0 or is not counted per role (`:202`-`:207`); the package doc says budgets "can only limit, they default to 0 (off), and they are counted per role" (`:15`-`:17`). Per-role overrides are `ASSAY_DESK_<KNOB>__<ROLE>`. `deskfile` does not resolve any knob through this package yet.
- Brief ids: `SplitBriefTrailer` accepts `<stream>/<NN>`, `<stream>:<NN>`, `<repo>:<stream>:<NN>` and the qualified `<cell>:<repo>:<stream>:<NN>` (`tools/desk/internal/deskkit/trailer.go:183`-`:188`), the same qualified form perf-v4 stores (`perfrec/context.go:63`-`:74`; e.g. `assay:assay:desktools-v2:70`). Perf-v4 work references have only `brief` and `issue` kinds; there is no PR kind (`perfrec/context.go:63`-`:74`, attribution spec `work` row at `docs/performance-attribution-spec.md:75`). This brief therefore spells a PR as `<owner>/<repo>#<N>` on the command line and records it in deskfile's audit line, mirroring the issue reference (repository identity plus number). It does not add a perf-v4 kind.
- Contract row `R-new-issue-budget` (`docs/contracts.md:257`) describes the per-session cap and names `TestNewRateUnparseableFallsBackWithNotice` and `TestForceFileDoesNotResetTheRateCount`.
- Skill wording: the shared comms block says "a spent filing budget never pushes a routine relay onto the tracker" (verify-desk `SKILL.md:1101`, pr-review-desk `:1239`, the-desk `:435`, pr-shepherd `:335`, intake-desk `:611`, worker-desk `:971`); worker-desk `:458` says `deskpr create` and `deskfile new` share one repo-wide ceiling of 20 per hour. Could not find a passage that explicitly tells a desk to defer an Evidence blocker behind the budget. The deferral happens because the tool refuses (exit 4), so the skill change is to name the bound form, not to remove a sentence.

Design-fit: proposal 11 classes new-issue caps as brakes and says relaxing one "is a separate human decision, recorded by the maintainer and naming its compensating bound" (`11-target-architecture.md:296`-`:311`).
That decision is recorded by the driver's merge of this gate: human brief PR, #4069, merged 2026-10-22, which ratifies the 2026-10-22 driver ruling with its compensating bound named in the merged text; no separate earlier record is claimed.
The compensating bound is three layers, and each stands without the others: (1) a per-PR, per-role cap on bound filings, as a knob shaped like the rest of `deskcore/config` (budget class, default 0, per role), where the counted role is the one whose App identity performs the write (`deskkit.SessionTokenRole`, the same custody that mints the write token), never a caller-chosen label; (2) bound filings audit as their own class, so the per-session count of unbound filings is unchanged and the bound ones remain visible; (3) the existing repo-wide outward-write ceiling still charges every bound filing. The bound is counted per PR, not per PR head: a head-keyed count would reset on every push, so a push loop could unlock more filings. The brief adds no verb (`new` gains flags), no new tool key (the ceiling keys on `deskfile`) and no perf-v4 schema change. It fits the verifier transaction of proposal 13 (step 5, "Publish through the authorized path"): a blocker the review requires is part of publishing the result, not a process-error report.

single-point-of-failure: the count limits, not the binding check — the per-role per-PR cap (when set), the per-PR outward-write tier and the repo-wide ceiling; the last two are always on and are the only bound while the cap is off. The binding check only ties a filing to an open PR and its brief; it does not tie the filer to that PR (see Human decision).

## Human decision

For ratification at merge: driver ruling, 2026-10-22, whose public record is the driver's merge of this gate: human brief PR, #4069, merged 2026-10-22. Workflow-bound filings are not counted against the
`new` budget. A bound filing names its work item; when the brief is known it passes both
`--for-pr <owner/repo#N>` and `--brief <brief id>`, never one without the other; `--for-pr`
alone only for a PR that has no brief. The tool verifies the binding and back-links the
issue on the PR. The compensating bound is a per-PR cap, its own audit class, and the
repo-wide ceiling. The done-time acceptance below still binds separately.

What the binding does not do: `--for-pr` does not bind the filer to the PR's author, its
reviewer or the holder of its dispatch claim. Any role may bind a filing to any open PR in
the repository, and to any open PR with no brief by `--for-pr` alone. The binding records
which work item a filing belongs to; it is not an authorization check. The real bound is
the counting: the per-role per-PR cap, the per-PR write tier and the repo-wide ceiling.
Requiring the filer to hold the PR's claim was considered and left out: claim custody is
not on deskfile's write path today, and adding it is a separate change.

Still for acceptance at done: the knob's off state. `deskcore/config` requires a budget
knob to default to 0 (off). Recommended: the bound cap knob defaults to 0 (off) like every
other budget knob. While it is off, the per-PR outward-write tier (`write.rate.per_pr_hour`,
20 per hour shipped) charged against the bound PR and the repo-wide ceiling remain the
always-on bound, and the driver's deployed configuration sets the cap per role. Weigh the
daily figure, not only the hourly one: today an unbound filer gets 3 per session per
repository per 24 hours; with the cap off, bound filings are limited only by the 20-an-hour
tiers, which allow up to 480 in 24 hours.
Alternative: refuse every bound filing while the cap is off. That keeps the brake fully
closed by default, but an adopter with no configuration gets today's behaviour and
nothing from the ruling.

Default if no answer: the recommended shape (bound-cap knob defaults to 0, cap set per role in deployed config; per-PR write tier and repo-wide ceiling always on) after 2026-10-24.

The default above fixes only the design the implementer builds to at dispatch. It never
substitutes for the `gate: human` acceptance at done, which still decides the off state.

## Ground rules

Normal worker admission, one isolated worktree and one draft PR. No workflow triggers,
releases or live infrastructure; tests use the existing audit-ledger and forge fakes.
Do not weaken the unbound budget, `--force-file` charging, the repo-wide ceiling or any
existing refusal. Stop at implemented; an independent verifier records Evidence.

## Task

1. Add a knob row to `deskcore/config/knobs.go`: `issue.file.bound_per_pr`, class budget,
   count of issues, default 0, per role, earlier hard-coded value 0, doc "workflow-bound
   issues one role may file against one change request per issue.file.window; 0 = off".
   It reuses `issue.file.window` for its window. Extend the table tests.
2. `deskfile new` accepts `--for-pr <owner>/<repo>#<N>` and `--brief <brief id>` (any form
   `SplitBriefTrailer` accepts, recorded in canonical qualified form). Refuse (exit 5):
   `--brief` without `--for-pr`; `--for-pr` without `--brief` when the PR has a brief;
   a malformed reference; and `--force-file` with `--for-pr` (a bound filing never needs
   the escape hatch).
3. Before any create, verify through the existing forge interface: the PR is open; the
   brief resolves to a brief file in the PR's tracking repository; and it is the PR's
   brief, meaning the PR body's `Brief:` or `Cross-repo-brief:` trailer names it, or the
   PR's diff touches that brief file (an Evidence PR). A could-not-read result holds
   (exit 6) and is never treated as bound. A refused binding is not charged.
4. A bound filing is audited with `Tool "deskfile"`, `Verb "new-bound"`, `PR` set to the
   bound PR number, and a `Detail` prefix `for-pr <owner/repo#N> brief <id> | `, using the
   same prefix convention as `createSentMarker`. `checkSessionBudget` stays unchanged and
   so does not count it. A new counter charges bound entries per role, per bound PR
   (repository plus number), over `issue.file.window`, using `chargedNewEntry`'s charge
   rules, and refuses (exit 4, with retry-after) at the resolved `issue.file.bound_per_pr`.
   The counted role is `deskkit.SessionTokenRole("deskfile")`, the role whose App token
   performs the write; it is recorded in the `Detail` prefix as
   `for-pr <owner/repo#N> brief <id> role <role> | ` and the counter reads it from there.
   `--raised-by`, `--to` and any other flag or variable never set the counted role. A role
   that cannot be resolved holds (exit 6) before any create. Changing the loop identity
   changes the App that writes, so the filing is charged to that role, and the number of
   roles is fixed by the roster. An unreadable ledger holds (exit 6). `AllowWriteRepoWide("deskfile", repo)` still runs,
   and `AllowWrite("deskfile", <PR repo>, N)` charges the bound PR's per-PR tier.
5. After a confirmed create, back-link the new issue on the PR through the existing
   `attach` path, with one comment naming the issue. If the back-link fails, report it
   as unverifiable and leave the issue in place. Never delete the issue or retry the create.
6. Rewrite contract row `R-new-issue-budget` to cover both classes, the bound cap knob,
   the always-on tiers and the new tests, and re-stamp it. Amend the skill passages named
   in Context: workflow filings use `--for-pr` with `--brief`, and a spent budget never
   defers a bound filing. Regenerate the Cursor mirror.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +mutation | `cd tools/desk && go test ./cmd/deskfile/ -run '^TestBoundFiling$' -count=1 -v` | exit 0 and named PASS; cases: bound filing with exhausted `new` budget succeeds; `--brief` alone refused; `--for-pr` alone refused when the PR has a brief and accepted when it has none; closed PR refused; brief not the PR's brief refused; unreadable PR holds exit 6; `--force-file` with `--for-pr` refused; mutation: make the "is the PR's brief" check always return true — the brief-mismatch case must fail |
| 2 | check:ci +flow | `cd tools/desk && go test ./cmd/deskfile/ -run '^TestBoundFilingCap$' -count=1 -v` | exit 0 and named PASS; with `ASSAY_DESK_ISSUE_FILE_BOUND_PER_PR__WORKER=2` a third bound filing on the same PR by the same role is refused exit 4 with retry-after, a filing on a different PR still succeeds, and a different role is counted separately; the `Detail` prefix records the session-token role; passing `--raised-by` with another role, or setting the per-role override for another role, does not change the counted role or reset the count; an unresolvable session role holds exit 6 with nothing created |
| 3 | check:ci | `cd tools/desk && go test ./cmd/deskfile/ -run '^TestBoundFilingOffFixedCeiling$' -count=1 -v` | exit 0 and named PASS; a bound filing is not refused by the fixed 20-per-hour repo-wide `deskfile` ceiling, and is refused (exit 4) by the PR-bound backstop `write.bound.repo_hour` at its default 60 per repository per hour, with the refusal naming that knob |
| 4 | check:ci | `cd tools/desk && go test ./cmd/deskfile/ -count=1` | exit 0; the unbound budget's existing regressions (`TestBudget*`, `TestNewRateUnparseableFallsBackWithNotice`, `TestForceFileDoesNotResetTheRateCount`) still pass unchanged |
| 5 | check:ci | `cd deskcore && go test ./config/ -count=1` | exit 0; the new knob passes the table validator (default 0, per role) |
| 6 | check:ci | `grep -n 'issue.file.bound_per_pr' deskcore/config/knobs.go docs/contracts.md` | both files match; the contract row names the knob and the bound-filing tests |
| 7 | check:ci | `cd tools/desk && go run ./cmd/deskinstall --harness cursor --forge github --repo ../.. --bundle ../../plugins/assay --check` | exit 0; no Cursor-mirror drift reported |
| 8 | check | `statusgen --root . --consumers --brief desktools-v2/70 --base $(git merge-base origin/main HEAD)` | pre-merge, on the implementation branch: exit 0; declared implementation sites and follow-up references corroborate; out-of-scope judgments remain UNCHECKED and reviewer-owned; completeness is not proved |
| 9 | check | `statusgen --root . --lint` | exit 0; no PROBLEM line names this brief |

Pre-mortem: an exemption that lets any filing through by naming any PR is caught by row 1
(binding refusals); a cap keyed on head SHA or on session instead of PR and role, or a
counted role taken from `--raised-by` instead of the session-token role, is caught by
row 2; dropping the repo-wide ceiling for the bound path is caught by row 3; regressing
the unbound budget while refactoring the counter is caught by row 4. Mutation check for
row 1: make the "is the PR's brief" check always return true and confirm the
brief-mismatch case goes red. Whether the binding check reads enough of the PR (body
trailer and diff) is for review to judge.

The implementation must create the exact named tests; an empty go-test match is not PASS.

## Evidence

<!-- Independent postmerge verifier records actual commands, source and outcomes here. -->

## Review

Gate: human (gate-why above). Independent correctness and security review of the exact head; the human accepts the compensating bound at done.
