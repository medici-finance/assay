---
brief: assay:assay:contributor-trust:01
title: "Contributor provenance probe — mechanical signals about an unknown author, rendered as a neutral card"
why: >-
  When a pull request arrives from an identity the roster does not know, the only instrument
  a maintainer has is whatever they happen to notice in the thirty seconds before they decide
  whether to look. Every signal that distinguishes a considered contribution from an
  automated bulk sweep is already public metadata and none of it is collected, so the same
  judgement is re-made from scratch, worse, on every arrival — and the one thing that was
  provably wrong in the first such arrivals here was a body claim nobody had checked. A
  probe that gathers the signals and states them as facts costs the maintainer nothing and
  gives the contributor something to answer.
wave: 0
depends: []
unblocks: ["contributor-trust/07"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: >-
  This brief posts, on a public pull request, a card describing a named individual's account
  behaviour — how old the account is against when it first acted, how fast the fork became a
  pull request, how many pull requests they opened across repositories in a day, how their
  prior submissions were closed. That is a public statement about a person, and the human is
  confirming three things a model reading the diff cannot: that the signal list contains
  nothing identity-adjacent (no profile text, no follower counts, no employer, no geography),
  that the card states facts and renders no score and no verdict, and that a signal that
  could not be gathered appears as could-not-check rather than as an absence of concern.
issues: []
schema: brief-v2
authored: 2026-09-12 by a contributor-trust authoring session
sources:
  - "docs/streams/contributor-trust/spec.md §1 — the observed arrival pattern the signal list is designed against, and §4's 'signals are facts, verdicts are people's' boundary."
  - "docs/streams/decisions/DR-provenance-card.md — the design record this brief is authored against (PROPOSED): no score, no verdict, no identity-adjacent signals, public card, three-state."
  - ".github/workflows/inbound-triage.yml — the existing advisory commenter for unknown authors: pull_request_target, never checks out the pull request's code, comments and labels only, both enforcement switches default off. The card follows the same never-check-out-the-head posture."
  - "spec/brief-v1.md §8 — the three-state instrument invariant every signal in the card must satisfy."
  - "freshness-checked 2026-09-12 @ e96b7f6d (origin/main) — no provenance or signal-gathering code exists in the tree; the only unknown-author surface is the inbound-triage advisory."
design: DR-provenance-card
decision-trigger: creation
exec-tier: strong
exec-tier-why: >-
  (a) the signal set and its neutral rendering are design decisions the facts do not fully
  pre-specify; and (c) a card that quietly concludes, or that reports a failed measurement as
  a clean one, passes every happy-path test while being exactly the defect that matters.
consumers:
  - "tools/desk/internal/deskkit/provenance.go: follow-up contributor-trust/01 (this brief; flips to fixed-here when the implementation lands the signal gatherer)"
  - "tools/desk/cmd/deskprovenance/main.go: follow-up contributor-trust/01 (this brief; the verb that renders and posts the card)"
  - "docs/contributor-provenance.md: follow-up contributor-trust/01 (this brief; the published description of what the card measures and what it deliberately does not)"
  - ".github/workflows/inbound-triage.yml: out-of-scope (the card is posted by a desk verb under a role identity, not by a workflow; folding it into the pull_request_target job would put signal-gathering network reads next to a write-capable token, which that job's safety argument rests on not doing)"
version: 1
---

# Brief 01 — Contributor provenance probe

## Context

files:
- `tools/desk/internal/deskkit/provenance.go` (new) — one function per signal, each returning
  a measured value or a could-not-check, plus `Card(signals) string` rendering the neutral
  comment. Pure over injected readers; no network in the package under test.
- `tools/desk/internal/deskkit/provenance_test.go` (new).
- `tools/desk/cmd/deskprovenance/main.go` (new) — the verb: gather, render, post as a comment,
  replacing its own previous card rather than appending. `--dry-run` prints and posts nothing.
- `tools/desk/cmd/deskprovenance/testdata/` — fixtures: a plainly-ordinary author, an author
  matching every bulk-sweep signal, an author with several signals unreadable, an author whose
  diff touches continuous-integration and lockfile paths.
- `docs/contributor-provenance.md` (planned) (new) — what the card measures, the benign explanation
  carried alongside each signal, and the signals deliberately excluded and why.
- `changelog/contributor-provenance-card.md` (new).

single-point-of-failure: the card's wording is the only thing standing between a set of weak
correlations and a reader treating them as a verdict. Behind it, two independent layers — the
rendering function refuses to emit any aggregate (there is no score field to populate, so a
caller cannot add one without changing the type) and the excluded-signal list is asserted by a
test over the gatherer's own signal registry, so an identity-adjacent signal added later fails
the build rather than reaching a card.

facts:
- Signals, each measured from public metadata about the SUBMISSION and its timing, never about
  the person: account age against first observed activity; elapsed time from fork creation to
  pull-request open; count of pull requests the author opened across repositories in the
  preceding 24 hours; the author's prior merged-to-closed ratio; similarity of this body's
  shape to the author's other recent bodies; whether the commits carry a signature; and
  whether the diff touches continuous-integration configuration, dependency lockfiles, install
  scripts or container definitions.
- Excluded, by design and by test: profile text, avatar, follower or following counts, named
  employer, geography, account name shape, and anything else that describes the person rather
  than the submission.
- Every signal is a three-state measurement. The card's exit states are `clean` (every signal
  gathered, none in its notable band), `flagged` (at least one signal in its notable band) and
  `could-not-check` (at least one signal unreadable). Could-not-check is never rendered as
  clean, and `flagged` is never rendered as a conclusion about the change or the author.
- The card carries, per notable signal, the ordinary explanation for it — a new account is how
  everybody starts; a fast fork-to-pull-request is what a prepared patch looks like; a batch
  across repositories is what a dependency-bump sweep looks like.
- The card ends with a stated abstention: this is a description of the submission's shape, it
  is not a judgement of the change, and the change is reviewed on its merits.
- The verb NEVER fetches, checks out, builds or executes the pull request's head. It reads
  metadata only. This is the same posture the existing inbound advisory keeps.
- The card is replaced in place on re-run, so one pull request carries at most one card.

## Human decision
<!-- gate: human — decision-trigger: creation. Lifted VERBATIM into the decision issue; self-contained. -->
When a pull request arrives from somebody the project's automation does not recognise, the
proposal is for a tool to gather a fixed list of mechanical signals about the submission and
post them, as plain facts, in a comment on that pull request. The signals are all derived from
public metadata: how old the account is compared with when it first did anything, how long
passed between the fork being created and the pull request being opened, how many pull
requests the author opened across all repositories in the previous day, what proportion of
their earlier submissions were merged rather than closed, how similar this description is in
shape to their other recent ones, whether the commits are signed, and whether the change
touches the files that control what the build system executes.

The comment renders no score, no rating and no conclusion. It says what was measured, says the
ordinary innocent explanation for anything that stands out, and states explicitly that none of
it is a judgement of the change. A signal that could not be measured is shown as "could not
check", never as nothing to report. Nothing about the person is measured — no profile text, no
follower counts, no employer, no location.

The thing to weigh is that this is a public comment about a named individual's behaviour, and
the person it describes will read it.

Options:
1. **Post the card publicly, facts only, as described (recommended)** — the contributor can see
   exactly what was measured and can answer it in one sentence ("new account, these are
   genuine small fixes I had queued"). That sentence is worth more to the reviewer than any
   signal in the list. Consequence accepted: some genuine first-time contributors will find a
   card unwelcoming on arrival, and the mitigation is wording rather than suppression.
2. **Gather the same signals but show them only to the maintainer** — nothing public, so
   nobody feels examined. Consequence: the contributor cannot correct a misleading signal
   because they cannot see it, and the audit trail that would later justify a promotion or a
   demotion is gone.
3. **Add an overall rating (a number, or a red/amber/green mark)** — faster for a maintainer to
   act on. Consequence: a rating is a verdict, it compresses signals with different meanings
   and different error rates into one figure a reader will treat as authoritative, and it is
   the form most likely to be experienced as an accusation.
4. **Gather nothing; keep the present arrangement** — a maintainer decides from whatever they
   notice. Consequence: the status quo, which is what the recent arrivals overran.

Default if no answer: none — blocks until answered. Nothing is posted publicly about anybody
until this is decided.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Never fetch, check out, build or run a pull request's head in this brief, including in a
  test fixture. Metadata reads only.
- Every fixture, test and document uses invented identities. Do not commit a real external
  login anywhere in this change.

## Task

1. `provenance.go`: a signal registry — one entry per signal with its name, its gatherer, its
   notable band, and its one-line ordinary explanation — and a `Gather` that runs them over
   injected readers, returning a three-state result per signal. No aggregate field exists on
   the result type.
2. `Card`: render the gathered signals as a comment. Measured facts with their explanations,
   unreadable signals as could-not-check, and the closing abstention. Deterministic output so
   a test can pin it.
3. A test asserting the registry contains no identity-adjacent signal, driven by a denylist of
   the excluded categories, so adding one fails the build.
4. `deskprovenance`: gather, render, upsert the comment on the pull request, exit 0 clean, 1
   flagged, 6 could-not-check. `--dry-run` prints and posts nothing.
5. `docs/contributor-provenance.md` (planned), and the changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'Provenance' -count=1` | exit 0; output contains `ok` | check |
| 2 | `cd tools/desk && GOWORK=off go build ./cmd/deskprovenance && ./deskprovenance --dry-run --fixture cmd/deskprovenance/testdata/ordinary-author.json; echo rc=$?` | output contains `rc=0`; output does not contain `flagged` | check +flow |
| 3 | `cd tools/desk && ./deskprovenance --dry-run --fixture cmd/deskprovenance/testdata/bulk-sweep-author.json; echo rc=$?` | output contains `rc=1`; output does not contain `score`; output does not contain `verdict` | check +dereference |
| 4 | `cd tools/desk && ./deskprovenance --dry-run --fixture cmd/deskprovenance/testdata/unreadable-signals.json; echo rc=$?` | output contains `rc=6`; output contains `could-not-check`; output does not contain `rc=0` | check +mutation |
| 5 | `cd tools/desk && ./deskprovenance --dry-run --fixture cmd/deskprovenance/testdata/ci-paths-touched.json` | exit 1; output contains `continuous-integration` | check |
| 6 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ProvenanceExcludedSignals' -count=1 -v` | exit 0; output contains `PASS` (the denylist test: an identity-adjacent signal in the registry fails the build) | check +mutation |
| 7 | `cd tools/desk && ./deskprovenance --dry-run --fixture cmd/deskprovenance/testdata/bulk-sweep-author.json` | exit 1; output contains `not a judgement of the change` (the abstention is present on the flagged path, which is the path where it matters) | check |
| 8 | `git -C . grep -n -e follower -e employer -e geograph -- tools/desk/internal/deskkit/provenance.go` | exit 1; no matching line (the excluded categories appear nowhere in the gatherer) | check |
| 9 | `grep -n 'deliberately' docs/contributor-provenance.md` | exit 0; at least one matching line naming the excluded signals | check |
| 10 | `statusgen --root . --consumers --brief contributor-trust/01` | exit 0; output does not contain `DISPROVED` | check |

Pre-mortem to detection map. "A signal fails to read and the card says everything is fine" is
caught by row 4, which requires both the third exit state and the visible token. "The card
grows a score or a rating" is caught by row 3 and by the absence of any aggregate field on the
result type. "Somebody adds a follower-count or employer signal later" is caught by rows 6 and
8, which fail on the registry and on the source text independently. "The flagged card reads as
an accusation because the abstention only renders on the clean path" is caught by row 7. "The
verb fetches the head to compute a diff-touches-continuous-integration signal" — no row that
can prove a negative mechanically; the ground rule forbids it and the reviewer reads the
gatherer for a fetch, which is review-only. "The wording is technically neutral and still
lands badly" — no row; review-only by design, since tone is the review gate's judgement.

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: human (from frontmatter). Reviewer records verdict + date in the stream README table.
Human gate is MANDATORY when any risk answer is yes.
