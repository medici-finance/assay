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
| 10 | `statusgen --root . --consumers --brief assay:assay:contributor-trust:01` | exit 0; output does not contain `DISPROVED`; output does not contain `COULD-NOT-CHECK`; output contains `corroborated` (the fully-qualified key is required — the short `<stream>/<NN>` form answers `no brief-v1 file` and exits 2, so it can never corroborate anything) | check |

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

Implemented on branch `feat/contributor-trust-01`. Deliverables: `tools/desk/internal/deskkit/provenance.go`
(new — the seven-signal registry, `Gather`, `Overall`, `Card`), `tools/desk/internal/deskkit/provenance_test.go`
(new), `tools/desk/cmd/deskprovenance/main.go` (new — the verb: `--fixture`/`--dry-run` gather-and-print,
or a live gather-and-upsert against a real pull request), four fixtures under
`tools/desk/cmd/deskprovenance/testdata/`, `docs/contributor-provenance.md` (new, the published
description), and a changelog fragment. **Known scope boundary**, stated in the docs and in
`tools/desk/cmd/deskprovenance/main.go`'s package comment: today's Forge interface exposes only a pull
request's own body and changed-file paths, so a *live* (non-fixture) run reports the other five
signals as could-not-check rather than approximating them — extending the Forge interface to
carry them is a follow-up, not part of this brief's declared file list. Fail-first: with
`tools/desk/internal/deskkit/provenance.go` moved aside, `go test ./internal/deskkit/ -run 'Provenance'`
fails to build (`undefined: signalRegistry`, `undefined: ProvenanceInput`, `undefined: Gather`,
`undefined: Card`, …); restoring the file and re-running is green (see row 1). Verify table run
locally against the branch tree (`go build`/`go test` from this repo's `tools/desk/`, not an
installed binary):

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'Provenance' -count=1` | exit 0; `ok` | exit 0; `ok` (13 subtests) | 2026-09-13 | assay-worker-app[bot] |
| 2 | `go build ./cmd/deskprovenance && ./deskprovenance --dry-run --fixture cmd/deskprovenance/testdata/ordinary-author.json; echo rc=$?` | `rc=0`; no `flagged` | `rc=0`; card state `clean`, no `flagged` substring anywhere | 2026-09-13 | assay-worker-app[bot] |
| 3 | `./deskprovenance --dry-run --fixture cmd/deskprovenance/testdata/bulk-sweep-author.json; echo rc=$?` | `rc=1`; no `score`/`verdict` | `rc=1`; card state `flagged`, 6 of 7 signals notable; no `score`/`verdict` | 2026-09-13 | assay-worker-app[bot] |
| 4 | `./deskprovenance --dry-run --fixture cmd/deskprovenance/testdata/unreadable-signals.json; echo rc=$?` | `rc=6`; `could-not-check`; no `rc=0` | `rc=6`; card state `could-not-check` (4 signals unreadable, 0 flagged); no `rc=0` | 2026-09-13 | assay-worker-app[bot] |
| 5 | `./deskprovenance --dry-run --fixture cmd/deskprovenance/testdata/ci-paths-touched.json` | exit 1; `continuous-integration` | exit 1; `build-and-dependency-paths-touched` line reads "touches continuous-integration configuration…" | 2026-09-13 | assay-worker-app[bot] |
| 6 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ProvenanceExcludedSignals' -count=1 -v` | exit 0; `PASS` | exit 0; `--- PASS: TestProvenanceExcludedSignals` | 2026-09-13 | assay-worker-app[bot] |
| 7 | `./deskprovenance --dry-run --fixture cmd/deskprovenance/testdata/bulk-sweep-author.json` | exit 1; `not a judgement of the change` | exit 1; abstention line present verbatim | 2026-09-13 | assay-worker-app[bot] |
| 8 | `git grep -n -e follower -e employer -e geograph -- tools/desk/internal/deskkit/provenance.go` | exit 1; no match | exit 1 (git-grep's no-match code); no line printed | 2026-09-13 | assay-worker-app[bot] |
| 9 | `grep -n 'deliberately' docs/contributor-provenance.md` | exit 0; ≥1 match | exit 0; 2 matches (lines 6 and 39) | 2026-09-13 | assay-worker-app[bot] |
| 10 | `statusgen --root . --consumers --brief assay:assay:contributor-trust:01` | exit 0; `corroborated`; no `DISPROVED`/`COULD-NOT-CHECK` | run against the branch's own diff vs. `refs/remotes/origin/main` post-commit (see PR) | 2026-09-13 | assay-worker-app[bot] |

Also ran, given the corpus-leak guard's known trip on a stream's own brief/decision paths quoted
in `tools/desk` source comments (precedent: contributor-trust/02's Evidence section): `go test
./internal/deskkit/ -run 'TestCorpusHasNoWithheldStreamPaths'`. The first draft named the brief and
decision-record paths, and the tool's own package comment, directly; the guard flagged all five
occurrences (`docs/streams/contributor-trust`, `docs/streams/decisions`, and the bare
`contributor-trust` slug form). Neutralised to prose pointing at the published
`docs/contributor-provenance.md` instead — the guard is now clean.

### Non-implementer verifier run on merged main a65f270aa9c1 — 2026-10-09

Runner is not the implementer: assay-verifier-app[bot], on-behalf-of human:ian; model claude-opus-5-5. Detached worktree at merged main `a65f270aa9c1`; host darwin/arm64, go1.27.1, git 2.56.0, GNU bash 5.3.20, statusgen v1.0.34. Implementing commit `2f3b5da4a33b`. This brief is `gate: human` with `sensitive-data: yes`, so this block is evidence for the human gate only and changes no status. Every Go invocation and every run of the built tool used a throwaway HOME and temp directory with the ordinary Go build caches and module fetching off. The tool ran only with `--dry-run` against fixtures: no forge call, no token, nothing posted. The built binary was removed afterwards.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'Provenance' -count=1` | exit 0; `ok` | exit 0; `ok  github.com/medici-finance/assay/tools/desk/internal/deskkit 0.788s`; with -v, 13 tests each report `--- PASS` — meets Expect | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |
| 2 | `cd tools/desk && GOWORK=off go build ./cmd/deskprovenance && ./deskprovenance --dry-run --fixture cmd/deskprovenance/testdata/ordinary-author.json; echo rc=$?` | `rc=0`; no `flagged` | build exit 0; `rc=0`; `State: clean`; zero occurrences of `flagged`; abstention line present — meets Expect | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |
| 3 | `cd tools/desk && ./deskprovenance --dry-run --fixture cmd/deskprovenance/testdata/bulk-sweep-author.json; echo rc=$?` | `rc=1`; no `score`; no `verdict` | `rc=1`; `State: flagged`; 6 of 7 signals notable, each with its ordinary explanation; zero occurrences of `score`, `verdict` or `rating` in any letter case — meets Expect | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |
| 4 | `cd tools/desk && ./deskprovenance --dry-run --fixture cmd/deskprovenance/testdata/unreadable-signals.json; echo rc=$?` | `rc=6`; `could-not-check`; no `rc=0` | `rc=6`; `State: could-not-check`; four signals rendered as could-not-check with a reason each; no `rc=0` — meets Expect | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |
| 5 | `cd tools/desk && ./deskprovenance --dry-run --fixture cmd/deskprovenance/testdata/ci-paths-touched.json` | exit 1; `continuous-integration` | exit 1; `State: flagged`; the build-and-dependency-paths-touched line reads "this diff touches continuous-integration configuration, a dependency lockfile, an install script or a container definition (.github/workflows/ci.yml, go.sum)" — meets Expect | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |
| 6 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ProvenanceExcludedSignals' -count=1 -v` | exit 0; `PASS` | exit 0; `--- PASS: TestProvenanceExcludedSignals (0.00s)`, `PASS`, `ok` — meets Expect; see the mutation notes for what this guard does and does not catch | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |
| 7 | `cd tools/desk && ./deskprovenance --dry-run --fixture cmd/deskprovenance/testdata/bulk-sweep-author.json` | exit 1; `not a judgement of the change` | exit 1; last line of the flagged card: "This card describes the shape of this submission; it is not a judgement of the change, and the change is reviewed on its merits." — meets Expect | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |
| 8 | `git -C . grep -n -e follower -e employer -e geograph -- tools/desk/internal/deskkit/provenance.go` | exit 1; no match | exit 1; no line printed — meets Expect; the search is case-sensitive, see the mutation notes | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |
| 9 | `grep -n 'deliberately' docs/contributor-provenance.md` | exit 0; at least one match naming the excluded signals | exit 0; two matches: line 6 ("what it deliberately does not") and line 39 (the heading "What it deliberately excludes"); the excluded categories themselves are named on lines 41 to 43 under that heading — meets Expect | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |
| 10 | `statusgen --root . --consumers --brief assay:assay:contributor-trust:01` | exit 0; no `DISPROVED`; no `COULD-NOT-CHECK`; `corroborated` | exit 2; `statusgen: --consumers: COULD-NOT-CHECK: assay:assay:contributor-trust:01 is not in the diff against a65f270aa9c1…, so this run carries no evidence about its claims` — does NOT meet Expect as written on merged main (class #1915). Same instrument with an explicit base: base `14daf9276135` (parent of the commit that authored this brief) gives exit 0 with 3 CORROBORATED, 0 disproved, 1 UNCHECKED (the out-of-scope workflow claim; read by hand, that workflow has no reference to the probe); base `468a3082b1f0` (parent of the implementing commit) gives exit 0 with 0 corroborated, 4 unchecked | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |

Execution witness (statusgen verifyrun): not executed for any row, so 0 of 10 rows carry a witness and rows 1 to 9 rest on the hand run above. The witness has no per-row selection and executes every row in the caller's environment; a desk rule for this dispatch forbids running the Go tests under tools/desk, or the tool built from it, against the operator's real home directory, and seven of the ten rows are exactly that.

Mutations (each applied to the merged-main sources in place, observed, then reverted; afterwards provenance.go hashes to blob `3071b1465bca` and main.go to blob `188a3169113d`, identical to merged main):
- Row 4, overall fold: with the could-not-check precedence removed from Overall, the unreadable fixture prints `State: clean` and `rc=0`. Row 4 goes red and row 1 goes red (two tests). The guard discriminates.
- Row 4, single signal: with the commit-signature gatherer changed to report clean ("this submission's commits carry a signature") when the value is absent, row 4 still prints `rc=6` and row 1 still passes. The mutation survives because the fixture leaves four signals unreadable and the Go test asks only for at least one could-not-check signal. On a scratch fixture where only the signature value is absent, the mutant prints `State: clean`, `rc=0` and the false statement; merged main prints `State: could-not-check`, `rc=6`. Merged main is correct here; the guard is coarser than the property.
- Rows 6 and 8, literal word: a registry entry named follower-count turns row 6 red (`signal "follower-count" reads as identity-adjacent`) and row 8 red (exit 0, two matching lines). Both guards discriminate for the listed words.
- Rows 6 and 8, reworded: input fields FollowerCount, Employer and Geography plus a registry entry named audience-workplace-and-home-region with the band text "fewer than 10 accounts subscribe to this author's updates" leave row 6 at PASS and row 8 at exit 1 with no match, and the mutant card prints a line giving the author's subscriber count, workplace and home region. Row 8 is case-sensitive, so capitalised Go identifiers pass it; row 6 compares three registry text fields against eleven substrings, so a synonym passes it. Neither guard tests the category "describes the person"; that judgement stays with review.
- Row 7: with the abstention written only on a clean card, row 7 exits 1 with zero abstention lines (red) and row 1 goes red (TestProvenanceCardAlwaysCarriesAbstention). The guard discriminates.
- Row 3, literal word: a card line "Risk score: 6/7" turns row 3 red and row 1 red (TestProvenanceRegistryHasNoAggregateField). The guard discriminates for the listed words.
- Row 3, reworded: a card line "Suspicion level: 6 of 7 (HIGH)" leaves row 3 green (`rc=1`, no `score`, no `verdict`) and row 1 green. The renderer computed the aggregate from the existing results with no type change, so the Context's statement that an aggregate cannot be added without changing the type holds for the result type only, and the remaining layer is a three-word list.
- Live posting: with the unconditional refusal after the `--post` gate replaced by a stub that neither posts nor refuses (never run against a live target), build and vet stay clean, row 1 passes and all four fixture rows keep their exit codes; the command package has no test file. No Verify row and no test guards the refusal. On merged main the refusal holds by reading: main.go:156-162 returns before any write unless `--post` is given without `--dry-run`, main.go:168-169 then refuses unconditionally with exit 2, and the function that would post (main.go:228) has no call site. The same reading means "replaced in place on re-run" has no executed evidence.

Dry-run probes on merged main with invented scratch fixtures (no Verify row covers these):
- An empty object gives `rc=6` with all seven signals could-not-check. Invalid JSON and a missing fixture give exit 2. `--fixture` without `--dry-run` gives exit 2, with or without `--post`. `--dry-run --post --fixture` prints the card and posts nothing.
- Timestamps in an impossible order (first activity before account creation, pull request opened before the fork) and a burst count of -4 give `State: clean`, `rc=0`, with the lines "first observed activity came 431d after it was created", "3d elapsed between the fork being created and this pull request being opened" and "opened -4 pull request(s)". The comment at provenance.go:295-296 calls a negative gap a could-not-check-worthy oddity; the gatherers at provenance.go:317-322 and 329-334 return clean with a false sentence.
- A diff touching only .gitlab-ci.yml, Cargo.lock, Jenkinsfile, setup.py, composer.lock, .circleci/config.yml, Containerfile and uv.lock gives `State: clean`, `rc=0` and the sentence "this diff does not touch continuous-integration configuration, dependency lockfiles, install scripts or container definitions". The marker list at provenance.go:207-221 has thirteen substrings and none matches those paths. A diff touching only docs/makefile-guide.md is reported as notable (substring match).
- A fork-to-pull-request gap of exactly five minutes is clean (strict less-than at provenance.go:331).

Context paths at this main: every path in `files:`, `consumers:` and `sources:` exists except the changelog fragment named in `files:`; its text is in the top-level changelog instead. The `(planned)` marker on the published description and the "flips to fixed-here" wording in `consumers:` are stale now that the implementation is merged.

Risk values (enumerated over the implementing commit: provenance.go, provenance_test.go and the deskprovenance main.go; ranked by how hard the consequence is to undo, a public comment about a named person being the irreversible act):
- RISK-VALUE: DERIVED — live-post gate = `if *dryRun || !*post { … return }` then unconditional `return 2` @ tools/desk/cmd/deskprovenance/main.go:156,169 — the Human decision section says nothing is posted publicly about anybody until the decision is made, and the design record is still PROPOSED with a placeholder in decided-by (DR-provenance-card lines 6 and 19). An unconditional refusal is the only value consistent with that. No test pins it.
- RISK-VALUE: NAMED, NOT DERIVED — role default = "worker" @ tools/desk/cmd/deskprovenance/main.go:102 — the brief says the card is posted "under a role identity" and names none; which identity a public card about a contributor is attributed to is part of the pending human decision. Inert while the refusal above holds.
- RISK-VALUE: NAMED, NOT DERIVED — excludedSignalDenylist = 11 substrings (profile, avatar, follower, following, employer, geograph, location, username, account name, real name, display name) @ tools/desk/internal/deskkit/provenance_test.go:16-28 — the list covers the six categories the brief names as words, but the brief's exclusion ends "anything else that describes the person", which no word list can derive; the reworded mutation above passes it.
- RISK-VALUE: DERIVED — abstentionText = "This card describes the shape of this submission; it is not a judgement of the change, and the change is reviewed on its merits." @ tools/desk/internal/deskkit/provenance.go:267 — carries the three clauses the brief's facts require, and is written on every state (provenance.go:288-290).
- RISK-VALUE: DERIVED — ExitCode = clean 0, flagged 1, anything else 6 @ tools/desk/internal/deskkit/provenance.go:73,75,77 — the three exit states the rows pin; the default arm returns 6, so an unknown state can never read as clean.
- RISK-VALUE: NAMED, NOT DERIVED — account-age band = `gap >= 0 && gap < 24*time.Hour` @ tools/desk/internal/deskkit/provenance.go:319 — neither the brief nor the stream spec gives a figure. The band measures first activity against creation, a fixed historical fact, so it stays notable for an old account that began quickly and stays quiet for the long-dormant-then-active pattern the stream spec section 1 describes.
- RISK-VALUE: DERIVED — fork-to-pull-request band = `gap >= 0 && gap < 5*time.Minute` @ tools/desk/internal/deskkit/provenance.go:331 — the stream spec section 1 records the observed pattern as "inside three to five minutes". Exactly five minutes falls outside the band.
- RISK-VALUE: NAMED, NOT DERIVED — burst band = `n >= 5` @ tools/desk/internal/deskkit/provenance.go:343 — the stream spec records about eleven pull requests in twelve hours; nothing sources five in twenty-four.
- RISK-VALUE: NAMED, NOT DERIVED — ratio band = `total >= 3 && ratio < 0.2` @ tools/desk/internal/deskkit/provenance.go:360 — no source for either figure.
- RISK-VALUE: NAMED, NOT DERIVED — riskyPathMarkers = 13 substrings @ tools/desk/internal/deskkit/provenance.go:207-221 — the brief names four file classes and no list; the probe above shows common members of those classes outside the list rendered as a negative statement.
- Ranked last, reversible presentation values with no derivation needed: the card marker comment @ main.go:51 and the duration rounding units @ provenance.go:302-308.

Verification-Attestation: medici-finance/assay#2482 run=8afef3bc97990e04168a69e256feba191fbc20e5aec44acc5a0bdf7b714d3834 source=a65f270aa9c10e73bd68e68b6a29d253a9a8ac28 brief=docs/streams/contributor-trust/brief-01-provenance-probe.md model=claude-opus-5-5 tier=any

VERIFY: BLOCKED — check-definition: rows 1 to 9 meet Expect by hand; row 10 cannot be satisfied as written on merged main (exit 2, could-not-check, class #1915) and corroborates 3 of 4 claims only with an explicit base at the authoring commit. No execution witness. The gate is human and a risk answer is yes, so this is evidence for the human gate and no status changes.

## Review
Gate: human (from frontmatter). Reviewer records verdict + date in the stream README table.
Human gate is MANDATORY when any risk answer is yes.
