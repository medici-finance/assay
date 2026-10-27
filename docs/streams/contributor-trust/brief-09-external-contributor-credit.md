---
brief: assay:assay:contributor-trust:09
title: "External-contributor credit in release notes — the aggregator names the author a fork change came from"
why: >-
  The release aggregator lifts fragment BULLETS and nothing else, so a fix merged from a fork
  is credited nowhere in the changelog or the release notes unless the contributor thought to
  write their own handle into the bullet — and on the fork path they often cannot, because the
  fragment is landed on their behalf by a maintainer. The project asks outside contributors
  for issue-first discipline, a claims checklist and a verification statement; giving nothing
  back in the one artifact anybody reads is the cheapest possible way to look ungrateful. The
  authorship is already recorded in git and on the pull request; this brief just stops
  throwing it away at aggregate time.
wave: 1
depends: ["contributor-trust/02"]
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-12 by a contributor-trust authoring session
sources:
  - "tools/changelog/aggregate.py (287 lines, measured 2026-09-12) — the release-time engine. Subcommands `unreleased-bullets`, `highlights`, `roll`; `_parse_bullets` reads a fragment into (bucket, entry) pairs; the module is pure over the filesystem and makes no network call, a property `aggregate_test.sh` relies on."
  - "tools/changelog/aggregate_test.sh (110 lines) — the offline, network-free harness, with the AGG_IMPL indirection (`AGG_IMPL=testdata/old-aggregate.py ./aggregate_test.sh` reds the fragment cases) that is this repository's established fail-first evidence pattern."
  - ".github/workflows/release.yml — the `CHANGELOG discipline — aggregate fragments and require highlights` step (`python3 tools/changelog/aggregate.py highlights changelog CHANGELOG.md`) and the later `write the aggregated section and clear fragments` step. Measured 2026-09-12: NO checkout in this workflow sets `fetch-depth`, so every job runs on the default shallow clone."
  - "changelog/README.md § 'Fragment by proxy (fork PRs)' and tools/changelog/README.md § the PR_NUMBER input — the existing proxy path for a fork pull request whose branch maintainers cannot push to."
  - "contributor-trust/02 = assay:assay:contributor-trust:02 (docs/streams/contributor-trust/brief-02-trust-tiers-and-ledger.md) — the identity resolver this brief asks 'is this author external?' through, rather than re-deriving the answer from the raw roster."
  - "freshness-checked 2026-09-12 @ 6d4d8f0a (origin/main plus this stream's authoring commit) — aggregate.py contains no author, credit, thanks or PR-number handling of any kind; the word 'credit' does not appear in tools/changelog/."
exec-tier: strong
exec-tier-why: >-
  (b) correctness is a cross-artifact argument — a resolver spanning the fragment file, git
  history whose depth the workflow controls, the forge's view of the pull request, and the
  identity predicate — and the failure that matters (crediting the wrong person, or a
  maintainer, in a published artifact) passes every happy-path test on a fixture.
consumers:
  - "tools/changelog/aggregate.py: follow-up contributor-trust/09 (this brief; the credits map input and the credits resolver subcommand)"
  - "tools/changelog/aggregate_test.sh: follow-up contributor-trust/09 (this brief; the three new cases and their fail-first evidence)"
  - ".github/workflows/release.yml: follow-up contributor-trust/09 (this brief; the aggregate step gains the credit resolution, and the release job's checkout gains the history depth the resolver needs)"
  - "changelog/README.md: follow-up contributor-trust/09 (this brief; one sentence on the proxy recipe, and the opt-out marker)"
  - "tools/changelog/README.md: follow-up contributor-trust/09 (this brief; the credits map's shape and its three states)"
  - "tools/changelog/check.sh: out-of-scope (the PR-gate check decides whether a fragment EXISTS for a pull request; credit is an aggregate-time concern and the gate's verdict is unchanged — a fragment with no credit is exactly as valid as one with)"
  - "CHANGELOG.md sections already published: out-of-scope (forward-only by construction; a released section is a record of what was said at the time and is never rewritten to add a credit)"
version: 1
---

# Brief 09 — External-contributor credit in release notes

## Context

files:
- `tools/changelog/aggregate.py` — a new `credits` subcommand (fragment directory in, a fragment-to-pull-request map out, resolved from git alone), and an optional `--credits <file>` on `highlights` and `roll` that appends the credit suffix to every bullet of a credited fragment. The module stays pure over the filesystem and makes no network call.
- `tools/changelog/aggregate_test.sh` — three new cases, plus the `AGG_IMPL` fail-first evidence the harness already supports.
- `.github/workflows/release.yml` — the aggregate step resolves the credits map and passes it to `highlights`; the same map is passed to `roll` in the write step so the changelog section and the release body agree. The release job's checkout gains the history depth the git half of the resolution needs.
- `changelog/README.md` — one sentence in the proxy recipe, and the opt-out marker.
- `tools/changelog/README.md` — the credits map's shape and its three states.
- `changelog/contributor-trust-credit.md` (new).

facts:
- Today `aggregate.py` lifts bullet TEXT and nothing else. It has no notion of an author, a
  pull request or a credit, and `aggregate_test.sh` is offline and network-free — a property
  the design must not break, because it is what makes the engine testable at all.
- The split that preserves that: the git half (fragment path → the commit that added it → its
  merge commit → the pull-request number from the merge-commit subject) is pure git and lives
  in `aggregate.py credits`. The forge half (pull-request number → author login, whether that
  identity is external, whether the body carries the opt-out marker) needs the forge and lives
  in the workflow step, which writes the resulting map to a file. `highlights` and `roll` only
  ever READ the map.
- "External" is answered by the identity resolver `contributor-trust/02` supplies — the same
  single answer to the same question the rest of the stream uses. An identity the roster knows
  (a maintainer, a role automation account) is never credited: the credit exists to name
  somebody the project does not already list.
- **Measured constraint:** no checkout in `release.yml` sets `fetch-depth`, so the release job
  runs on a default shallow clone, where `git log` over a fragment path cannot reach the
  commit that added it. The resolver is therefore inert until the checkout is deepened, and
  deepening it is part of this brief rather than an assumption about the runner.
- Three states, and the third is the safe one: a fragment whose pull request cannot be
  resolved, whose author cannot be read, or whose identity cannot be classified, gets NO
  credit and a named could-not-check line in the step log. Silence is the fail-closed
  direction here — a wrong name in a published release is worse than a missing one, and no
  release is ever refused for want of a credit.
- The credit suffix is a configured form, defaulting to ` — thanks @<login>`, appended to each
  of that fragment's bullets. Appended, never rewritten into the bullet's middle, so a
  multi-line highlight's continuation lines are untouched.
- **Opt-out:** a documented marker on its own line in the pull-request body suppresses the
  credit for that pull request. The forge half already reads the body to find the author, so
  the marker costs no extra call. An opted-out fragment is credited to nobody and is otherwise
  aggregated exactly as before.
- **Forward-only:** only the cut being prepared is affected. A `## <tag>` section already
  written to `CHANGELOG.md`, and any release already published, are never rewritten to add a
  credit.
- The forge half reads a pull request's author and body, so the changelog jobs need
  `pull-requests: read`. Measured 2026-09-12: `release.yml` carries NO `pull-requests` scope
  today at any level, so this is a new scope and not a widening of an existing one. It is
  READ-only, it adds no trigger, and it changes no existing permission — the release job's
  standing `contents: write` (it pushes the tag) is untouched and is a separate concern.
- Risk answers against the declared paths: `.github/workflows/release.yml` is a release-path
  trigger and all four answers are nevertheless `no`. The change adds reads (history depth, a
  read-only pull-requests scope, an author lookup) and appends a string to text the step
  already emits. It grants nothing, authorizes nothing, changes no trigger and adds no write
  scope or artifact. Its worst failure is a wrong or missing name in release prose, which the
  forward-only rule bounds to one cut.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Keep `aggregate.py` pure: no network call from the module, so `aggregate_test.sh` stays
  offline. Anything needing the forge belongs to the workflow step.
- Never rewrite a published `CHANGELOG.md` section or an existing release body.
- Name no real external account anywhere — in the code, the fixtures, the tests or the
  documentation. Every example uses an invented login.
- A credit must never be able to fail a release. If resolution fails for every fragment, the
  cut proceeds uncredited.

## Task

1. `aggregate.py credits <fragment-dir>`: for each fragment, resolve the adding commit, its
   merge commit and the pull-request number from the merge-commit subject; emit a map, with an
   explicit unresolved marker per fragment rather than an omission, so a reader can tell "no
   pull request found" from "not looked at".
2. `--credits <file>` on `highlights` and `roll`: append the configured suffix to each bullet
   of a credited fragment. Unknown or unresolved entries append nothing.
3. `release.yml`: resolve the map (git half, then the forge half with the identity predicate
   and the opt-out marker), pass it to both the aggregate step and the write step, deepen the
   release job's checkout enough for the git half to reach the fragment's adding commit, and
   add `pull-requests: read` to the changelog jobs so the forge half can read a pull request's
   author and body. Read-only: add no `pull-requests: write`, and change no existing
   permission. Log a named could-not-check line per unresolved fragment.
4. `aggregate_test.sh`: the three cases in the Verify table, written to run against
   `AGG_IMPL=testdata/old-aggregate.py` as the committed fail-first evidence.
5. The sentence in the proxy recipe (the aggregator credits an external author automatically,
   so a proxy fragment need not carry a hand-written credit), the opt-out marker's
   documentation, and the credits-map shape in the tools README. Plus the changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/changelog && ./aggregate_test.sh; echo rc=$?` | output contains `rc=0`; output does not contain `FAIL` | check |
| 2 | `cd tools/changelog && AGG_IMPL=testdata/old-aggregate.py ./aggregate_test.sh; echo rc=$?` | output does not contain `rc=0`; output contains `FAIL` (the committed fail-first evidence: the new credit cases red against the retired engine) | check +mutation |
| 3 | `cd tools/changelog && ./aggregate_test.sh 2>&1` | exit 0; output contains `C1` and `thanks` (an external-authored fragment's bullets carry the credit) | check |
| 4 | `cd tools/changelog && ./aggregate_test.sh 2>&1` | exit 0; output contains `C2` (a roster-authored fragment gets no credit: the negative control) | check +mutation |
| 5 | `cd tools/changelog && ./aggregate_test.sh 2>&1` | exit 0; output contains `C3` (the opt-out marker suppresses the credit) | check +mutation |
| 6 | `cd tools/changelog && ./aggregate_test.sh 2>&1` | exit 0; output contains `C4` (an unresolvable fragment aggregates uncredited and the run still exits 0: a credit never fails a cut) | check +mutation |
| 7 | `cd tools/changelog && python3 aggregate.py highlights testdata-none CHANGELOG-none 2>&1; echo rc=$?` | output contains `rc=2` (positive control: the existing empty-fragments refusal is unchanged) | check +neighbour |
| 8 | `cd tools/changelog && git -C ../.. grep -n -c 'import ' -- tools/changelog/aggregate.py` | exit 0; output is `3` or fewer (the module gained no network or forge import; it still stands on os, re and sys) | check +dereference |
| 9 | `git -C . grep -n 'fetch-depth' -- .github/workflows/release.yml` | exit 0; at least one matching line (the release job's checkout is deep enough for the git half to resolve a fragment's adding commit) | check +dereference |
| 10 | `grep -n -i 'thanks' changelog/README.md` | exit 0; at least one matching line (the proxy recipe says the aggregator credits an external author automatically) | check |
| 11 | `grep -n -i 'opt-out' changelog/README.md tools/changelog/README.md` | exit 0; at least one matching line from each | check |
| 12 | `cd tools/changelog && ./check_test.sh; echo rc=$?` | output contains `rc=0` (neighbour: the PR-gate check's own suite is untouched by the aggregate-time change) | check +neighbour |
| 13 | `cd tools/changelog && python3 aggregate.py credits testdata/credits-fixture-repo/changelog 2>&1` | exit 0; output contains `unresolved` (a fragment with no merge commit reports an explicit unresolved marker, not an omission) | check +flow |
| 14 | `git -C . grep -n 'pull-requests: read' -- .github/workflows/release.yml` | exit 0; at least one matching line (the forge half can read a pull request's author and body) | check +dereference |
| 15 | `git -C . grep -n 'pull-requests: write' -- .github/workflows/release.yml` | exit 1; no matching line (negative control: the pull-requests scope added is READ-only. The release job already carries `contents: write` to push the tag; that is unchanged and is not what this row is about) | check +mutation |
| 16 | `statusgen --root . --consumers --brief assay:assay:contributor-trust:09` | exit 0; output does not contain `DISPROVED`; output does not contain `COULD-NOT-CHECK`; output contains `corroborated` (the fully-qualified key is required — the short `<stream>/<NN>` form answers `no brief-v1 file` and exits 2, so it can never corroborate anything) | check |

Pre-mortem to detection map. "The credit lands on the wrong person because the merge-commit
subject's pull-request number is parsed loosely" is caught by row 13's explicit unresolved
marker plus row 4's negative control — a resolution that cannot be made confidently produces
no name at all. "A maintainer or a role automation account is thanked in the release notes" is
caught by row 4. "The opt-out is documented but not wired" is caught by rows 5 and 11. "A
failed resolution fails the release, so an unrelated forge outage blocks a cut" is caught by
rows 6 and 7. "Somebody adds a forge API call inside `aggregate.py` and the offline test
silently starts needing the network" is caught by row 8. "The resolver ships but is inert on
the shallow clone the release job actually runs on, so nobody is ever credited and nothing
reports it" is caught by row 9 — the exact failure the measured `fetch-depth` fact exists to
prevent. "The pull-requests scope is widened from read to write while nobody is looking, so a release-note nicety carries a credential that can alter pull requests" is caught by rows 14 and 15, which assert the scope that is there and the scope that must not be. "Published releases are rewritten to backfill credits" — no row; forward-only is a
property of what the change does not do, and a check cannot prove a negative here. The
reviewer confirms `roll` writes only the section for the tag being cut, as it does today.
"The credit wording reads as grudging or as over-familiar" — no row; wording is the review
gate's judgement, and the form is configurable precisely so an adopter can change it.

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

### Non-implementer verifier run — 2026-10-21 claude-opus-5-5 verifier at 3cea6aa621ec (rows 1 to 15 pass; row 16 could-not-check as written)

Verification-Attestation: medici-finance/assay#3843 run=6f1180d3dfc608c1fc2145ed195e0f6bd8f8135eb24a7ac100cb8b8a1e3c20d0 source=3cea6aa621ec0364fb7a6cbd1196141829ba6c4c brief=docs/streams/contributor-trust/brief-09-external-contributor-credit.md model=claude-opus-5-5 tier=strong

Verification-Table: delivery=c856fec8136b2ca3ff22ec8f63c2c27ed10b4cc8 base=a906f6200ed11b573bc7c8f6ae2a506bb0b29f1d original=a906f6200ed11b573bc7c8f6ae2a506bb0b29f1d

Fresh non-implementer pass on merged main 3cea6aa621ec, admitted by the dispatcher check (exit 0, binding above). The re-derivation reports 16 rows, 0 author-added, and the Verify table is unchanged between the delivery's parent a906f6200ed1 and this commit (only the Context files list was re-wrapped). The work landed in three first-parent commits: c856fec8136b (#941, engine, tests, docs, identity helper), cc4f9e22 (#949, the release workflow wiring) and 95898d14d90c (#980, the resolver follow-up). The marker above names #941 as the delivery. All rows ran on darwin with KUBECONFIG=/dev/null, statusgen v1.0.54, and each run left the home tree clean. Logs were kept in a scratch directory outside the home.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | `cd tools/changelog && ./aggregate_test.sh; echo rc=$?` | rc=0, no FAIL | pass: `aggregate_test: 14 passed, 0 failed` then `rc=0`; zero FAIL lines | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | `cd tools/changelog && AGG_IMPL=testdata/old-aggregate.py ./aggregate_test.sh; echo rc=$?` | no rc=0, contains FAIL | pass: `aggregate_test: 3 passed, 11 failed` then `rc=1`; 11 FAIL lines, with C3, C4, C5, C6 and C7 among the failing cases | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | `cd tools/changelog && ./aggregate_test.sh 2>&1` | exit 0, contains C1 and thanks | pass: exit 0; `ok   - C1 external-authored fragment: bullets carry the 'thanks @<login>' credit` | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | `cd tools/changelog && ./aggregate_test.sh 2>&1` | exit 0, contains C2 | pass: exit 0; `ok   - C2 roster-authored fragment is NOT credited (negative control)` and `ok   - C2b an unlisted [bot] login is never credited`. Mutation: the same case reds against the retired engine (row 2) | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | `cd tools/changelog && ./aggregate_test.sh 2>&1` | exit 0, contains C3 | pass: exit 0; `ok   - C3 the opt-out marker suppresses the credit`. Mutation: C3 is FAIL against the retired engine (row 2) | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 6 | `cd tools/changelog && ./aggregate_test.sh 2>&1` | exit 0, contains C4 | pass: exit 0; `ok   - C4 unresolvable fragment aggregates uncredited, exit 0`. Mutation: C4 is FAIL against the retired engine (row 2) | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 7 | `cd tools/changelog && python3 aggregate.py highlights testdata-none CHANGELOG-none 2>&1; echo rc=$?` | contains rc=2 | pass: `changelog: nothing to aggregate — no fragment files under the changelog/ directory and no residual '## Unreleased' bullets.` then `rc=2` | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 8 | `cd tools/changelog && git -C ../.. grep -n -c 'import ' -- tools/changelog/aggregate.py` | exit 0, count 3 or fewer | pass: exit 0; `tools/changelog/aggregate.py:3`. Dereferenced by hand: the three lines are `import os`, `import re`, `import sys` (lines 75 to 77); there is no from-import, importlib or dunder-import. Git is reached through os.popen with every interpolated value single-quoted by the module's own quoting helper | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 9 | `git -C . grep -n 'fetch-depth' -- .github/workflows/release.yml` | exit 0, at least one line | pass: exit 0; three lines, release.yml lines 886, 1509 and 1824, each `fetch-depth: 0`. Line 886 carries the contributor-trust/09 comment explaining the shallow-clone failure it prevents | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 10 | `grep -n -i 'thanks' changelog/README.md` | exit 0, at least one line | pass: exit 0; line 89 names the appended ` — thanks @<login>` suffix for an external author | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 11 | `grep -n -i 'opt-out' changelog/README.md tools/changelog/README.md` | exit 0, a line from each file | pass: exit 0; changelog README line 110 (`**Opt-out.** A contributor who would rather not be named ...`) and tools/changelog README lines 266, 271, 281, 284 and 296 | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 12 | `cd tools/changelog && ./check_test.sh; echo rc=$?` | contains rc=0 | pass: `check_test: 23 passed, 0 failed` then `rc=0` | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 13 | `cd tools/changelog && python3 aggregate.py credits testdata/credits-fixture-repo/changelog 2>&1` | exit 0, contains unresolved | pass: exit 0; four lines: merged-via-merge-commit.md 4242, merged-via-squash.md 4343, never-merged.md `unresolved`, squash-then-unrelated-merge.md 5151 | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 14 | `git -C . grep -n 'pull-requests: read' -- .github/workflows/release.yml` | exit 0, at least one line | pass: exit 0; permission lines 133, 868 and 1814 plus two comment lines (958, 1897). Line 868 sits under the credit job's permissions with a READ-ONLY comment naming contributor-trust/09 | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 15 | `git -C . grep -n 'pull-requests: write' -- .github/workflows/release.yml` | exit 1, no line | pass: exit 1, no output | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 16 | `statusgen --root . --consumers --brief assay:assay:contributor-trust:09` | exit 0, no DISPROVED, no COULD-NOT-CHECK, contains corroborated | could-not-check exit=2: `--consumers: COULD-NOT-CHECK: assay:assay:contributor-trust:09 is not in the diff against 3cea6aa621ec..., and the brief's delivery could not be derived: the committed Evidence has no Verification-Table marker naming the delivery.` The command derives its pin only from a marker committed at HEAD, and before this Evidence lands none exists. It can be re-run as written once the marker above is on main | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Supplemental probes for row 16. These are diagnostics only and earn no acceptance credit for the row.
- (a) `statusgen --root . --consumers --brief assay:assay:contributor-trust:09 --delivery c856fec8136b2ca3ff22ec8f63c2c27ed10b4cc8 --base a906f6200ed11b573bc7c8f6ae2a506bb0b29f1d` exits 0 with `summary: 5 corroborated, 0 disproved, 2 unchecked`. The two unchecked entries are the out-of-scope claims (check.sh and published CHANGELOG sections), which stay the reviewer's call.
- (b) The same pinning against 95898d14d90c and its parent gives the same summary.
- (c) In a throwaway clone outside the home, the row's exact command ran with HEAD equal to the clone's origin/main and the Verification-Table line above committed into this section. It exits 0 with `5 corroborated, 0 disproved, 2 unchecked`. So row 16 as written is expected to pass after this Evidence lands, but this run has not observed that.

Enumeration (risk-bearing values) over the delivery diff (#941), the workflow wiring (#949), the follow-up (#980) and the Deliverables:
- CREDIT_SUFFIX = " — thanks %s" @ tools/changelog/aggregate.py:91
- UNRESOLVED = "unresolved" @ tools/changelog/aggregate.py:95
- _MERGE_PR_RE = r"^Merge pull request #(\d+) " @ tools/changelog/aggregate.py:100
- _SQUASH_PR_RE = r"\(#(\d+)\)\s*$" @ tools/changelog/aggregate.py:101
- _LOGIN_RE repeat bound {0,38} @ tools/changelog/aggregate.py:106
- OPT_OUT_MARKER = '<!-- changelog-credit: no -->' @ tools/changelog/credit-identity.sh:46
- the bot-suffix and app-prefix patterns that never credit an App login @ tools/changelog/credit-identity.sh:116 and :117
- fetch-depth: 0 @ .github/workflows/release.yml:886, :1509 and :1824
- pull-requests: read @ .github/workflows/release.yml:868 and :1814
- curl --retry 3 @ .github/workflows/release.yml:963 and :1902 (the credit steps)

The brief's frontmatter says irreversible: no. Ranked by what breaks: the PR-number regexes and the roster/bot predicate decide WHO is named in published release prose. A wrong name can be corrected in the release body by an edit, but the changelog section is forward-only by policy, so these rank first. The login regex and the opt-out marker rank next. fetch-depth, the read scope, the suffix wording and the retry count are reversible operational knobs and rank last.

RISK-VALUE: DERIVED — _MERGE_PR_RE = r"^Merge pull request #(\d+) " @ tools/changelog/aggregate.py:100 — this is GitHub's fixed merge-commit subject (`Merge pull request #N from owner/branch`). The match is anchored at the start and needs a space after the digits, so the number is the whole token. The squash form _SQUASH_PR_RE = r"\(#(\d+)\)\s*$" @ tools/changelog/aggregate.py:101 is anchored at the end, matching GitHub's default squash title `<title> (#N)`. Any other subject is unresolved and credits nobody, which is the brief's fail-closed direction. Fixture cases C5 to C7 and row 13 confirm the merge, squash, never-merged and later-unrelated-merge shapes.

RISK-VALUE: DERIVED — _LOGIN_RE bound {0,38} @ tools/changelog/aggregate.py:106 — one leading alphanumeric plus up to 38 more characters gives at most 39, which is GitHub's maximum login length. Hyphens must be single and internal, matching GitHub's login rules. A value outside that form is never rendered as a credit.

RISK-VALUE: NAMED, NOT DERIVED — OPT_OUT_MARKER = '<!-- changelog-credit: no -->' @ tools/changelog/credit-identity.sh:46 — an arbitrary documented token. It is correct as long as it matches the documentation (tools/changelog README line 296 documents it) and is matched as a whole line (`grep -qxF`, line 83). There is no first-principles value to derive. The roster/bot predicate at credit-identity.sh lines 94 to 123 depends on the operator's configured roster, so this run cannot evaluate it against a live roster. Test cases C2 and C2b cover its logic offline.

**VERIFY: BLOCKED** — 15 of 16 rows pass. Row 16 is could-not-check as written: the bare post-merge `--consumers --brief` derives its delivery only from a committed Verification-Table marker, and none was committed at 3cea6aa621ec. Supplemental probes show 0 disproved and 5 corroborated once it is pinned. Next actor: the desk lands this Evidence (with the marker), then a fresh verifier re-runs row 16 as written on main. Status stays `implemented`.

### Non-implementer verifier re-run — 2026-10-23, merged main 4b7568de01248b514f48493efea7734ce0149823

Run by a fresh, non-implementer verifier (claude-opus-5-5, strong tier) under attestation medici-finance/assay#4187. Admission: `deskdispatch --check-verifier` admitted the run at the source above, with the home clean, before any row ran. The first check after the run refused, because the rows leave behind the suite's git-ignored fixture repository ("unattested worktree file" under tools/changelog/testdata/credits-fixture-repo). After that generated fixture was removed, the check admitted again with the same binding. The witness below comes from the installed `statusgen verifyrun` (release binary v1.0.56, not a build from this tree) on the host (darwin), offline (`KUBECONFIG=/dev/null`). Every row ran against this commit. This section supersedes the earlier hand-run section on this PR as the execution record; that section stays as it is.

Attempts, all disclosed: one `statusgen verifyrun --dry-run` invocation produced the witness table below (the table was printed, not written back, and is pasted here unchanged). The verifier then ran rows 1-3 and 7-16 once more by hand, at the same commit, to read their output against each Expect cell (rows 4-6 share row 3's command and its output); that pass earns no witness credit. Two supplemental probes, one for row 13 and one for row 16, are described below; neither discharges a row. Nothing was retried and no table was discarded.

- Rows 1-7 and 9-15 meet their Expect cells. Row 1: `aggregate_test: 14 passed, 0 failed`, then `rc=0`, and no `FAIL` line. Row 2 (fail-first, retired engine): `aggregate_test: 3 passed, 11 failed`, then `rc=1`, with `FAIL - C1` through `FAIL - C7` lines. Rows 3-6: exit 0, with `ok - C1 external-authored fragment: bullets carry the 'thanks @<login>' credit`, `ok - C2 roster-authored fragment is NOT credited (negative control)`, `ok - C3 the opt-out marker suppresses the credit` and `ok - C4 unresolvable fragment aggregates uncredited, exit 0`. Row 7: the empty-fragments refusal, then `rc=2`. Row 9: three `fetch-depth: 0` lines in the release workflow (lines 886, 1509 and 1824). Row 10: changelog README line 89 names the `thanks @<login>` credit. Row 11: changelog README line 110 and tools/changelog README lines 266, 271, 281, 284 and 296. Row 12: `check_test: 23 passed, 0 failed`, then `rc=0`. Row 13: exit 0, with the line `never-merged.md` then a tab then `unresolved`. Row 14: `pull-requests: read` at release workflow lines 133, 868 and 1814 (plus two comment lines). Row 15: exit 1, no output.
- Row 13 depends on row order. Its fixture repository is built by the suite in rows 1-6 and is git-ignored. The witness hash for row 13 (9d69237515e7) matches the four-line output the verifier read by hand on the built fixture, so the witnessed run read the real fixture. On a clean tree, with the fixture removed, the same command alone prints only `testdata/credits-fixture-repo/changelog` then `unresolved` and exits 0 (output hash 69bfacbd63d1). That meets the Expect cell's words without testing anything, so a standalone run of row 13 is not a discriminating witness. This is advisory: row 13's credit here rests on the in-order witness. The fixture the rows built was removed from the home after the run, so the home is back at its attested state for landing.
- Row 8 is a check-definition mismatch, not a code failure. `git grep -n -c` on a named path prints `tools/changelog/aggregate.py:3`. The import count is 3 (os, re, sys at aggregate.py lines 75-77), so the stated purpose holds: no network or forge import. But no output line is the bare `3` the Expect cell asks for, and the installed witness records `fail exit=0` ("no output line equals 3"). The row cannot pass its witness as written. It needs a reviewed definition repair, for example `git grep -h -c`, which prints the count alone. This pass does not replace the command. The row gets no acceptance credit, and no implementation bug is claimed.
- Row 16 exits 2. The output is `COULD-NOT-CHECK: assay:assay:contributor-trust:09 is not in the diff against 4b7568de01248b514f48493efea7734ce0149823, and the brief's delivery could not be derived: the committed Evidence has no Verification-Table marker naming the delivery`. This is the post-merge `--consumers --brief` class with no `--base` (#1915). Supplemental probe, no credit: `statusgen --root . --consumers --brief assay:assay:contributor-trust:09 --delivery c856fec8136b2ca3ff22ec8f63c2c27ed10b4cc8 --base a906f6200ed11b573bc7c8f6ae2a506bb0b29f1d` (the #941 merge and its first parent) exits 0 with `summary: 5 corroborated, 0 disproved, 2 unchecked, 0 brief(s) claiming nothing`.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/changelog && ./aggregate_test.sh; echo rc=$?` | pass exit=0 | sha256:a08856c967fe | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de0124 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/changelog && AGG_IMPL=testdata/old-aggregate.py ./aggregate_test.sh; echo rc=$?` | pass exit=0 | sha256:0d6c992bf00b | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de0124 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/changelog && ./aggregate_test.sh 2>&1` | pass exit=0 | sha256:f7a549113b38 | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de0124 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/changelog && ./aggregate_test.sh 2>&1` | pass exit=0 | sha256:f7a549113b38 | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de0124 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/changelog && ./aggregate_test.sh 2>&1` | pass exit=0 | sha256:f7a549113b38 | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de0124 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/changelog && ./aggregate_test.sh 2>&1` | pass exit=0 | sha256:f7a549113b38 | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de0124 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/changelog && python3 aggregate.py highlights testdata-none CHANGELOG-none 2>&1; echo rc=$?` | pass exit=0 | sha256:4b99d8c85889 | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de0124 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/changelog && git -C ../.. grep -n -c 'import ' -- tools/changelog/aggregate.py` | fail exit=0 | sha256:8cd12913a8fe | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de0124 (on-behalf-of human:ian) (forge-identity) |
| 9 | `git -C . grep -n 'fetch-depth' -- .github/workflows/release.yml` | pass exit=0 | sha256:b0b7da998f02 | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de0124 (on-behalf-of human:ian) (forge-identity) |
| 10 | `grep -n -i 'thanks' changelog/README.md` | pass exit=0 | sha256:bba28cf1d3ae | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de0124 (on-behalf-of human:ian) (forge-identity) |
| 11 | `grep -n -i 'opt-out' changelog/README.md tools/changelog/README.md` | pass exit=0 | sha256:872ea634c095 | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de0124 (on-behalf-of human:ian) (forge-identity) |
| 12 | `cd tools/changelog && ./check_test.sh; echo rc=$?` | pass exit=0 | sha256:35cf6117b62a | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de0124 (on-behalf-of human:ian) (forge-identity) |
| 13 | `cd tools/changelog && python3 aggregate.py credits testdata/credits-fixture-repo/changelog 2>&1` | pass exit=0 | sha256:9d69237515e7 | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de0124 (on-behalf-of human:ian) (forge-identity) |
| 14 | `git -C . grep -n 'pull-requests: read' -- .github/workflows/release.yml` | pass exit=0 | sha256:378e0da216e4 | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de0124 (on-behalf-of human:ian) (forge-identity) |
| 15 | `git -C . grep -n 'pull-requests: write' -- .github/workflows/release.yml` | pass exit=1 | sha256:e3b0c44298fc | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de0124 (on-behalf-of human:ian) (forge-identity) |
| 16 | `statusgen --root . --consumers --brief assay:assay:contributor-trust:09` | fail exit=2 | sha256:0bd2e16412c7 | 2026-10-23 | assay-verifier-app[bot] @ 4b7568de0124 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4187 run=7571a06362b9cf9592ddf85bacdd78df39d8a09e20a7481b71bc592d16574f22 source=4b7568de01248b514f48493efea7734ce0149823 brief=docs/streams/contributor-trust/brief-09-external-contributor-credit.md model=claude-opus-5-5 tier=strong

Risk-value enumeration, read at 4b7568de0124. The frontmatter answers `no` to every risk question, including irreversible. The work arrived in #941 (aggregator, resolver, docs) and #949 (release workflow). Each literal those changes introduced that is still on main:

1. pull-requests = read @ .github/workflows/release.yml:868 (release job) and :1814 (CHANGELOG roll job).
2. _MERGE_PR_RE = `^Merge pull request #(\d+) ` @ tools/changelog/aggregate.py:100.
3. _SQUASH_PR_RE = `\(#(\d+)\)\s*$` @ tools/changelog/aggregate.py:101.
4. _LOGIN_RE = `^@[A-Za-z0-9](?:[A-Za-z0-9]|-(?=[A-Za-z0-9])){0,38}$` @ tools/changelog/aggregate.py:106.
5. fetch-depth = 0 @ .github/workflows/release.yml:886 and :1824.
6. OPT_OUT_MARKER = `<!-- changelog-credit: no -->` @ tools/changelog/credit-identity.sh:46.
7. UNRESOLVED = "unresolved" @ tools/changelog/aggregate.py:95.
8. CREDIT_SUFFIX = " — thanks %s" @ tools/changelog/aggregate.py:91.

Ranking. A published release body that thanks the wrong person is the hardest of these to undo. It ranks entries 2-4 first, then entry 1 (a widened scope is a credential exposure while it lasts), then 5 and 6. Entries 7 and 8 are a sentinel and wording, both reversible with an edit, so they rank last.

RISK-VALUE: DERIVED — pull-requests = read @ .github/workflows/release.yml:868 — the forge half only reads a pull request's author and body, and the read scope covers that. Nothing in the credit path writes to a pull request. Row 15 confirms no `pull-requests: write` anywhere in the workflow.
RISK-VALUE: DERIVED — _MERGE_PR_RE = `^Merge pull request #(\d+) ` @ tools/changelog/aggregate.py:100 — this is the forge's fixed subject for a merge-commit landing ("Merge pull request #N from owner/branch"). It is anchored at the start and needs a trailing space, so a PR number cannot be read from elsewhere in a subject.
RISK-VALUE: DERIVED — _SQUASH_PR_RE = `\(#(\d+)\)\s*$` @ tools/changelog/aggregate.py:101 — a squash landing's default subject ends "<title> (#N)". The end anchor stops a `(#M)` inside the title from being taken. Test C6 covers the squash-then-unrelated-merge case.
RISK-VALUE: DERIVED — _LOGIN_RE = `^@[A-Za-z0-9](?:[A-Za-z0-9]|-(?=[A-Za-z0-9])){0,38}$` @ tools/changelog/aggregate.py:106 — forge logins are 1 to 39 characters: alphanumerics and single hyphens, never a leading, trailing or doubled hyphen. One leading alphanumeric plus up to 38 more is a maximum of 39, and the lookahead rules out a hyphen not followed by an alphanumeric.
RISK-VALUE: DERIVED — fetch-depth = 0 @ .github/workflows/release.yml:886 — 0 is the checkout action's full-history setting. The resolver walks back to each fragment's adding commit and its merge, which a shallow clone cannot reach. The same value is at :1824 for the roll job.
RISK-VALUE: DERIVED — OPT_OUT_MARKER = `<!-- changelog-credit: no -->` @ tools/changelog/credit-identity.sh:46 — byte-identical to the marker the contributor docs tell people to use (changelog README line 114, tools/changelog README line 300). A contributor who follows the docs is matched.
RISK-VALUE: reversible knob, ranked last, no derivation owed — UNRESOLVED = "unresolved" @ tools/changelog/aggregate.py:95; CREDIT_SUFFIX = " — thanks %s" @ tools/changelog/aggregate.py:91 (wording, configurable by design).

**VERIFY: BLOCKED** — 14 of 16 rows witnessed passing, and no acceptance obligation was observed to fail. Two rows hold the item. Row 8's witness fails on its wording, not the code: `git grep -c` prints `path:3`, never a bare count, so it needs a definition repair. Row 16 is COULD-NOT-CHECK: an unpinned post-merge `--consumers --brief`, class #1915. Status stays where it is.

Blockers: row 8 definition repair is filed as #4193. Row 16 is held on #1915.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
