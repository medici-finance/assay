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
- `tools/changelog/aggregate.py` — a new `credits` subcommand (fragment directory in, a
  fragment-to-pull-request map out, resolved from git alone), and an optional `--credits
  <file>` on `highlights` and `roll` that appends the credit suffix to every bullet of a
  credited fragment. The module stays pure over the filesystem and makes no network call.
- `tools/changelog/aggregate_test.sh` — three new cases, plus the `AGG_IMPL` fail-first
  evidence the harness already supports.
- `.github/workflows/release.yml` — the aggregate step resolves the credits map and passes it
  to `highlights`; the same map is passed to `roll` in the write step so the changelog section
  and the release body agree. The release job's checkout gains the history depth the git half
  of the resolution needs.
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

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
