---
brief: assay:assay:contributor-trust:02
title: "Trust tiers + the contributor ledger — one vocabulary for how much automation an external identity gets"
why: >-
  The inbound author bar is binary: an identity is in the roster or it is a stranger, and an
  item is blessed or it is quarantined. So a contributor whose third change is landing is
  assessed exactly like an account nobody has seen before, and there is nowhere to write down
  "this identity has a record" or "this identity's last three submissions were closed
  unreviewed". Every other control this stream adds — how deep review runs, whether a fork's
  workflows need a manual approve, what an automated-sweep signal costs — needs a name for
  the middle of that range and a place to record which name applies. This brief supplies
  both, and grants nothing until a human records the first row.
wave: 0
depends: []
unblocks: ["contributor-trust/03", "contributor-trust/04", "contributor-trust/05", "contributor-trust/07", "contributor-trust/08", "contributor-trust/09"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: >-
  The ledger's rows are trust judgements about named external people, and the tiers they carry
  decide whether that person's pull request runs continuous integration without a fresh human
  act. Two things need a human, not a model reading a diff: that the ledger is read from
  operator-side configuration and never lands as a file in this public tree (a demotion row in
  a public append-only register is a permanent public mark on an individual), and that an
  absent, unreadable or malformed ledger resolves every identity to `unknown` rather than
  failing open. The human is confirming those two properties and the per-tier capability list,
  not the code.
issues: []
schema: brief-v2
authored: 2026-09-12 by a contributor-trust authoring session
sources:
  - "docs/streams/contributor-trust/spec.md §3, §4 — the three consequences of a binary bar, and the four-tier model with its three boundaries."
  - "tools/desk/internal/deskkit/rosterconfig.go and statusgen/rosterconfig.go — the existing per-class roster loader (env over roster file), the documented-duplicate pattern the tier reader must follow, and the unset-trusts-nobody posture it inherits."
  - "tools/desk/internal/deskkit/trust.go and statusgen/trustgate.go — the present binary author bar, including the strict-path refusal of a human login with no pinned numeric id."
  - "docs/streams/decisions/DR-trust-tiers-ledger.md — the design record this brief is authored against (PROPOSED)."
  - "freshness-checked 2026-09-12 @ e96b7f6d (origin/main) — no tier vocabulary exists anywhere in the tree; the roster carries trusted logins, a blessing authority, a human-login map and trusted bot slugs, and nothing between trusted and untrusted."
design: DR-trust-tiers-ledger
decision-trigger: creation
exec-tier: strong
exec-tier-why: >-
  (c) trust plumbing — a subtle error in the resolution order or the fail-closed default
  silently widens who gets unattended automation, and a happy-path test on a known identity
  passes either way; and (b) the reader is a documented duplicate across two modules that a
  coupling test must hold together.
consumers:
  - "tools/desk/internal/deskkit/trusttier.go: fixed-here (the Tier type, LedgerRow, the injected LedgerLoader, ResolveTier and the capabilityTable land in this file)"
  - "tools/desk/internal/deskkit/rosterconfig.go: fixed-here (the ASSAY_CONTRIBUTOR_LEDGER roster key is added by this brief's implementation)"
  - "statusgen/rosterconfig.go: fixed-here (the documented-duplicate twin gains the identical key, recognised-not-applied, in the same change)"
  - "docs/contributor-trust.md: fixed-here (the published tier model)"
  - "the identity predicate contributor-trust/09 asks 'is this author external?' through: follow-up contributor-trust/09 (the release-note credit resolver reuses this reader rather than re-deriving the answer from the raw roster)"
  - "tools/desk/internal/deskkit/trust.go (the existing binary bar): out-of-scope (this brief ADDS a tier resolver beside it and changes no existing predicate; every current caller keeps its current answer, which is what makes the change inert until a row is written)"
version: 1
---

# Brief 02 — Trust tiers and the contributor ledger

## Context

files:
- `tools/desk/internal/deskkit/trusttier.go` (new) — the tier type, the ledger reader, and
  `ResolveTier(repo, login string, id int64) (Tier, Provenance)`; pure over an injected
  loader so it tests offline.
- `tools/desk/internal/deskkit/trusttier_test.go` (new).
- `tools/desk/internal/deskkit/rosterconfig.go` — one new roster key naming the ledger's path;
  read exactly like every other roster key (environment over roster file, per class).
- `statusgen/rosterconfig.go` — the documented-duplicate twin gains the same key, and the
  existing cross-tree coupling test is extended to cover it.
- `docs/contributor-trust.md` (planned) (new) — the published tier model: the four names, what each
  unlocks, how an identity moves, and the explicit statement that the rows are not published.
- `changelog/contributor-trust-tiers.md` (new).

single-point-of-failure: the ledger read. Behind it, two independent layers — the resolver's
fail-closed default (absent, unreadable, or malformed resolves every identity to `unknown`,
which is today's bar, so a ledger failure can only ever narrow) and the unchanged existing
author bar, which every current caller keeps consulting; a wrong tier can therefore widen
automation but cannot by itself admit an item the present gate refuses.

facts:
- Four tiers, ordered: `unknown` < `blessed-once` < `contributor` < `maintainer`. `unknown` is
  every identity the ledger does not name. `maintainer` is existing roster membership and is
  resolved from the roster, not from the ledger.
- The ledger is operator-side configuration named by a roster key, on the same path discipline
  as the roster file itself. It is NOT a file in this repository and no tool writes it except
  the blessing verb (`contributor-trust/03`).
- Resolution order: roster membership first (a roster identity is `maintainer` and the ledger
  cannot lower it), then the ledger row for `repo` + identity, then `unknown`.
- Fail-closed, three-state: absent ledger is a legitimate empty and resolves `unknown`;
  unreadable or malformed is could-not-check and ALSO resolves `unknown`, with the anomaly
  announced on standard error. The two are distinguishable in the returned provenance and are
  never collapsed.
- A ledger row carries: repository, identity (login plus pinned numeric id), tier, the date,
  the human who recorded it, and a reason. A row for a human identity with no pinned numeric
  id resolves `unknown` on the strict path, matching the existing recycled-login defence.
- Per-tier capability set, and nothing else is in it: review-lane depth, whether fork
  continuous-integration runs need a manual approve, whether desk automation may act on the
  identity's items, and changelog-proxy eligibility. No tier grants merge authority.
- This brief is INERT on landing: with no ledger configured every identity resolves exactly as
  it does today, and no existing caller's answer changes.

## Human decision
<!-- gate: human — decision-trigger: creation. Lifted VERBATIM into the decision issue; self-contained. -->
This project's automation currently sorts the people who send it changes into two boxes:
identities the operator has configured as trusted, and everybody else. Everybody else is
quarantined — their items are shown to a maintainer but no automation acts on them until a
maintainer admits that one item by hand. There is nothing in between, so somebody who has had
three changes accepted is treated on their fourth exactly like somebody nobody has ever seen.

The proposal is four named levels instead of two, with a record of which level each external
identity holds for each repository. The levels are: unknown (the default, and what everybody
outside the operator's own roster is today), blessed-once (a maintainer admitted one specific
item and nothing more), contributor (a standing grant a maintainer recorded after seeing the
identity's work land), and maintainer (the operator's existing roster, unchanged). A level
decides only how much machinery runs without a human present — how deep the review lane goes,
whether the identity's pull requests need a manual click before continuous integration runs,
whether automation may act on their items, and whether a workaround for crediting changelog
entries applies to them. No level lets anybody merge anything; merging stays a human act
behind branch protection.

What needs deciding is where the record of who holds which level lives.

Options:
1. **Outside the repository, with the operator's existing configuration (recommended)** — the
   record sits next to the trusted-identity configuration already held on the operator's
   machine. Consequence: it is not reviewable by pull request and not visible to anyone but
   the operator, and its integrity rests on the same custody the existing configuration has.
   What it buys: a level is a judgement about a named person, and a demotion is an adverse
   one. Publishing those judgements in a public, append-only, never-deleted record would put a
   permanent public mark on an individual, out of all proportion to the workflow problem being
   solved. This option publishes the MODEL — the level names, what each unlocks, how one
   moves — and never the rows.
2. **Inside the repository, as a tracked file** — reviewable by pull request like every other
   record here, and visible in history. Consequence: every promotion and every demotion
   becomes a permanent public statement about a named external person, and because the record
   is append-only it can be annotated but never withdrawn.
3. **Do not adopt levels; keep the present two boxes** — nothing to record and nothing to
   publish. Consequence: the rest of the planned work loses the vocabulary it keys on, and the
   repeated stranger-grade assessment of known-good contributors continues.

Default if no answer: none — blocks until answered. Everything downstream keys on the answer.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Do not write, commit, or add to version control any file containing a real external
  identity. The ledger's contents never enter this tree; the tests use invented logins.

## Task

1. `trusttier.go`: the ordered `Tier` type with the four values and a total order, a
   `LedgerRow` shape carrying repository, login, pinned id, tier, date, recorder and reason,
   a loader over an injected reader, and `ResolveTier` implementing the resolution order in
   `facts:`. Return a provenance value that distinguishes roster / ledger-row / default-absent
   / default-unreadable, so a caller can tell an empty ledger from a broken one.
2. One new roster key naming the ledger path, added to BOTH `rosterconfig.go` copies with the
   existing per-class precedence, and covered by the existing cross-tree coupling test.
3. A capability table in code — tier to the four capabilities named in `facts:` — as data, not
   as scattered conditionals, so a reader can see the whole grant in one place. No caller is
   wired to it in this brief; wiring is `contributor-trust/04`, `/05` and `/08`.
4. `docs/contributor-trust.md` (planned): the published model. State plainly that rows are operator-side
   and are not published, that a tier grants automation and never merge authority, and that
   promotion and demotion are recorded human acts with reasons.
5. The changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'TrustTier' -count=1` | exit 0; output contains `ok` | check |
| 2 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'TrustTierAbsentLedger' -count=1 -v` | exit 0; output contains `default-absent`; output contains `unknown` | check |
| 3 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'TrustTierUnreadableLedger' -count=1 -v` | exit 0; output contains `default-unreadable`; output contains `unknown`; output does not contain `contributor` | check +mutation |
| 4 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'TrustTierRosterWins' -count=1 -v` | exit 0; output contains `maintainer` (negative control: a ledger row naming a roster identity at a lower tier does not lower it) | check |
| 5 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'TrustTierUnpinnedHuman' -count=1 -v` | exit 0; output contains `unknown` (a ledger row for a human login with no pinned numeric id grants nothing on the strict path) | check +mutation |
| 6 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'RosterCoupling' -count=1` | exit 0; output contains `ok` (the statusgen twin reads the new key identically) | check +flow +neighbour |
| 7 | `grep -n 'never published' docs/contributor-trust.md` | exit 0; at least one matching line | check |
| 8 | `git -C . grep -n -E 'unknown.*blessed-once.*contributor.*maintainer' -- docs/contributor-trust.md` | exit 0; at least one matching line (the four tiers are named in order in the published model) | check |
| 9 | `cd tools/desk && GOWORK=off go build ./... && GOWORK=off go vet ./internal/deskkit/` | exit 0 | check:ci |
| 10 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'TrustTierPublishedModelMatchesTable' -count=1 -v` | exit 0; output contains `PASS` (the test parses the tier and capability names out of `docs/contributor-trust.md` (planned) and compares them to the capability table in code, so a published model that claims a capability the code does not grant fails) | check +dereference |
| 11 | `statusgen --root . --consumers --brief assay:assay:contributor-trust:02` | exit 0; output does not contain `DISPROVED`; output does not contain `COULD-NOT-CHECK`; output contains `corroborated` (the fully-qualified key is required — the short `<stream>/<NN>` form answers `no brief-v1 file` and exits 2, so it can never corroborate anything) | check |

Pre-mortem to detection map. "The ledger is absent on a fresh machine and every external
identity silently becomes a contributor" is caught by row 2. "A corrupt or half-written ledger
is read as an empty one and the anomaly is invisible" is caught by row 3, which requires the
distinct provenance value as well as the safe tier. "A ledger row demotes a maintainer and the
desk locks itself out" is caught by row 4. "A row naming a login with no pinned id grants a
tier, re-opening the recycled-login hole the existing strict path closes" is caught by row 5.
"One of the two duplicate roster readers gains the key and the other does not, so the board
and the desk disagree about a tier" is caught by row 6. "The published model leaks the rows'
existence as something readers may request" is caught by row 7. "A capability is added to a
tier by an inline conditional somewhere rather than to the table" — no row; review-only, it is
a code-shape judgement the reviewer makes from the diff.

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

Implemented on branch `feat/contributor-trust-02`. Deliverables: `tools/desk/internal/deskkit/trusttier.go`
(new — the `Tier` type, `LedgerRow`, the injected `LedgerLoader`, `ResolveTier`, and the
`capabilityTable`), `tools/desk/internal/deskkit/trusttier_test.go` (new), the
`ASSAY_CONTRIBUTOR_LEDGER` roster key added to both `tools/desk/internal/deskkit/rosterconfig.go`
(fully consumed) and `statusgen/rosterconfig.go` (recognised, not applied — statusgen resolves no
tier), the shared `statusgen/testdata/roster_coupling.json` vector extended with the new key,
`docs/contributor-trust.md` (new, the published model), and a changelog fragment. Verify table run
locally against the branch tree (`go build`/`go test` from this repo's `tools/desk/` and `statusgen/`
sources, not an installed binary):

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'TrustTier' -count=1` | exit 0; `ok` | exit 0; `ok` | 2026-09-12 | assay-worker-app[bot] |
| 2 | `-run 'TrustTierAbsentLedger' -v` | `default-absent`; `unknown` | `tier=unknown provenance=default-absent` | 2026-09-12 | assay-worker-app[bot] |
| 3 | `-run 'TrustTierUnreadableLedger' -v` | `default-unreadable`; `unknown`; no `contributor` | `tier=unknown provenance=default-unreadable`; no `contributor` substring in output | 2026-09-12 | assay-worker-app[bot] |
| 4 | `-run 'TrustTierRosterWins' -v` | `maintainer` | `tier=maintainer provenance=roster` (the fixture ledger row named the same roster identity at `blessed-once` and was never consulted) | 2026-09-12 | assay-worker-app[bot] |
| 5 | `-run 'TrustTierUnpinnedHuman' -v` | `unknown` | `tier=unknown provenance=default-absent` (both the unpinned-row and the zero-supplied-id shapes) | 2026-09-12 | assay-worker-app[bot] |
| 6 | `-run 'RosterCoupling' -count=1` | exit 0; `ok` | exit 0; `ok` (new `TestContributorLedgerRosterCoupling`; also re-ran statusgen's own `TestRosterKeySchemaCoupling` / `TestRosterCouplingVectors` — both green) | 2026-09-12 | assay-worker-app[bot] |
| 7 | `grep -n 'never published' docs/contributor-trust.md` | ≥1 match | line 69 | 2026-09-12 | assay-worker-app[bot] |
| 8 | `git grep -n -E 'unknown.*blessed-once.*contributor.*maintainer' -- docs/contributor-trust.md` | ≥1 match | line 11 (only matches once the file is staged — `git grep` does not search untracked files) | 2026-09-12 | assay-worker-app[bot] |
| 9 | `go build ./... && go vet ./internal/deskkit/` | exit 0 | exit 0 | 2026-09-12 | assay-worker-app[bot] |
| 10 | `-run 'TrustTierPublishedModelMatchesTable' -v` | `PASS` | `PASS` | 2026-09-12 | assay-worker-app[bot] |
| 11 | `statusgen --root . --consumers --brief assay:assay:contributor-trust:02` | exit 0; `corroborated`; no `DISPROVED`/`COULD-NOT-CHECK` | run against the branch's own diff vs. `refs/remotes/origin/main` post-commit (see PR) | 2026-09-12 | assay-worker-app[bot] |

Full package suite also run: `go test ./internal/deskkit/...` — clean (46s), no regressions from the
new roster key or the two-Config-struct-field addition.

Also ran, out of caution given the corpus-leak guard's scope: `go test ./internal/deskkit/ -run
'TestCorpusHasNoWithheldStreamPaths'` — the first draft of the new comments named the stream's own
briefs by bare `contributor-trust/NN` form and the design record by its `docs/streams/decisions/`
path, both of which the guard correctly flagged (this repo's whole `docs/streams/` tree is
`do-not-copy` for the `tools/desk` copy set, regardless of whether the referenced stream is itself
withheld). Neutralised to prose with no bare slug/number or `docs/streams/` path; the guard is now
clean.

Independent verification — 2026-10-09.

Runner: assay-verifier-app[bot], on-behalf-of human:ian — model claude-opus-5-5 — a fresh
session that did not implement this brief. Run against merged main at
a65f270aa9c10e73bd68e68b6a29d253a9a8ac28 (the implementation landed in #971). Toolchain:
go1.27.1 darwin/arm64, git 2.56.0, GNU bash 5.3.20, statusgen v1.0.34. Every Go row ran in a
cleared environment with a throwaway home and temp directory and the module proxy disabled.

Verification-Attestation (dispatcher pre-work check, read before the first row and again before
this block was written; both reads returned the same binding): Run
fbf2d4b0ff94384f132dce92c73563e03666c2cbbd880a458619ed18766646c7 — Source
a65f270aa9c10e73bd68e68b6a29d253a9a8ac28 — BriefSHA256
81f353462db936b074bc762930062f36f8183852646129d5489b3264c70edc70 — PlanSHA256
7714fb0395e1249e60c71fc50c5d2ee9688d5590df8460b3c098a1ff1436667b — HomeSHA256
8ae1541160c639fc4168b3eb5293cd34d28bedbd5c8aa17be01657c6f71b207a — Model claude-opus-5-5 —
Tier any — decision issue #2480.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'TrustTier' -count=1` | exit 0; output contains `ok` | exit 0; the single output line reads: ok, the deskkit package import path, 0.279s; a second pass with -v listed 9 tests, all PASS. Meets Expect: yes | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |
| 2 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'TrustTierAbsentLedger' -count=1 -v` | exit 0; `default-absent`; `unknown` | exit 0; `ResolveTier = tier=unknown provenance=default-absent`. Meets Expect: yes | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |
| 3 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'TrustTierUnreadableLedger' -count=1 -v` | exit 0; `default-unreadable`; `unknown`; no `contributor` | exit 0; `ResolveTier = tier=unknown provenance=default-unreadable`; standard error carried `trust-ledger: malformed ledger fixture: not valid JSON — resolving medici-finance/example/outsider to unknown`; the real-file variant of the test also PASS; zero occurrences of `contributor` in the output. Meets Expect: yes | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |
| 4 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'TrustTierRosterWins' -count=1 -v` | exit 0; `maintainer` | exit 0; `ResolveTier = tier=maintainer provenance=roster`. Meets Expect: yes | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |
| 5 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'TrustTierUnpinnedHuman' -count=1 -v` | exit 0; `unknown` | exit 0; `ResolveTier = tier=unknown provenance=default-absent`. Meets Expect: yes | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |
| 6 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'RosterCoupling' -count=1` | exit 0; output contains `ok` | exit 0; the single output line reads: ok, the deskkit package import path, 0.244s; a second pass with -v showed exactly one test selected, `TestContributorLedgerRosterCoupling`, PASS — the `ok` is a real test result, never an empty selection. Meets Expect: yes | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |
| 7 | `grep -n 'never published' docs/contributor-trust.md` | exit 0; at least one matching line | exit 0; one match, line 84: `**The rows are never published.** The ledger — which identity holds which tier,` (the earlier Evidence above says line 69; the document has grown since). Meets Expect: yes | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |
| 8 | `git -C . grep -n -E 'unknown.*blessed-once.*contributor.*maintainer' -- docs/contributor-trust.md` | exit 0; at least one matching line | exit 0; one match, line 11: the sentence beginning "Four tiers, ordered:" names the four in order. Meets Expect: yes | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |
| 9 | `cd tools/desk && GOWORK=off go build ./... && GOWORK=off go vet ./internal/deskkit/` | exit 0 | exit 0; no output; the tree was still clean afterwards, ignored files included. Executed by hand with the module proxy disabled. The row's class asks for a network-off sandbox, which this host (darwin) cannot provide, so the hermetic condition is could-not-check here; the command itself meets Expect: yes | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |
| 10 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'TrustTierPublishedModelMatchesTable' -count=1 -v` | exit 0; output contains `PASS` | exit 0; `--- PASS: TestTrustTierPublishedModelMatchesTable`, with the log line "capability table matches capabilityTable for all four tiers". Meets Expect: yes | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |
| 11 | `statusgen --root . --consumers --brief assay:assay:contributor-trust:02` | exit 0; no `DISPROVED`; no `COULD-NOT-CHECK`; contains `corroborated` | exit 2; `statusgen: --consumers: COULD-NOT-CHECK: assay:assay:contributor-trust:02 is not in the diff against a65f270aa9c10e73bd68e68b6a29d253a9a8ac28, so this run carries no evidence about its claims — no entry was corroborated and none was disproved.` Meets Expect: no — could-not-check. On merged main the default comparison base is the checked-out commit itself, so the brief is never in the diff and the row as written cannot corroborate anything after the merge; this is a property of the row, and says nothing about the implementation | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |

Row 11, supplementary read (does not turn the row green). The same command with the base moved
to the parent of the commit that landed #971 exits 0 with `summary: 4 corroborated, 0 disproved,
2 unchecked, 0 brief(s) claiming nothing` — all four `fixed-here` entries corroborated, the
`follow-up` and `out-of-scope` entries unchecked by design. That diff spans everything merged
since, so it is weak on its own; the narrower fact is that the landing commit's own file list
contains all four `fixed-here` paths.

Execution witness (`statusgen verifyrun`): held back on this host, for every row. The runner
gives each row the caller's own home directory, with no way to substitute a throwaway one and
no way to select a subset of rows, and this verification's standing rule keeps Go test rows
under the desk tools away from a real home. Row 9 would additionally need the network-off
sandbox the tool provides on Linux only. Witness tally: 0 pass, 0 fail, 11 without a witness
result. No witness table accompanies this block.

Mutation checks (each made in a throwaway copy of the tracked tree, never in the verified
checkout; each reverted, and the five touched files compared byte-identical to the checkout
afterwards; the copy's baseline was green before and after):

- Row 2: the not-configured arm returns `contributor` → row 2 red (`tier = contributor, want
  unknown`). The final no-match fallthrough returning `contributor` SURVIVES row 2 (its fixture
  only reaches the not-configured arm) and is caught by row 1.
- Row 3: unreadable arm reports `default-absent` → red. Unreadable arm returns `contributor` →
  red, and the output then contains `contributor`. The file reader skipping a malformed line →
  red. The file reader reporting a malformed file as not-configured → red. Deleting the
  standard-error announcement SURVIVES row 3, row 1 and the neighbouring lane tests.
- Row 4: roster arm disabled → red (`tier = blessed-once, want maintainer`).
- Row 5: three weakenings of the pinned-id condition. Accepting a row with no pinned id → red.
  Accepting a caller with no id → red. Dropping the id-equality term SURVIVES row 5 and is
  caught by row 1 (`TestTrustTierLedgerRowGrants`, the mismatched-id case).
- Row 6: key dropped from the twin's recognised list → red. Twin's literal changed → red. Key
  dropped from the shared vector → red. Key dropped from the desk copy's recognised list → red.
  Parsed value never stored → red.
- Row 7: the sentence reworded to offer the rows on request → red (exit 1).
- Row 8: two tiers swapped in the ordered sentence → red (exit 1).
- Row 10: document claims an extra grant for `blessed-once` → red. Code grants `unknown` a
  capability → red. Code grants `blessed-once` a second capability → red. Two shapes SURVIVE:
  a fifth capability added to `contributor` in code that the document has no column for
  (survives row 1 as well), and a capability column deleted from the document.

Risk-bearing values (enumerated over the landing commit's diff, ranked, top entries derived;
line numbers are in trusttier.go under the desk tools' deskkit package unless stated):

- RISK-VALUE: NAMED, NOT DERIVED — capabilityTable[TierContributor] = {review-depth, fork-ci-auto-run, desk-automation, changelog-proxy} @ trusttier.go:297 — it equals the published table, but no source assigns capabilities to tiers: the facts above name the four capabilities, the scoping document names the dimensions, and this brief reserves the per-tier list for the human gate, whose design record is still PROPOSED. A verifier cannot derive a value the gate exists to decide.
- RISK-VALUE: NAMED, NOT DERIVED — capabilityTable[TierBlessedOnce] = {review-depth} @ trusttier.go:296 — same missing source; additionally the lane table that later shipped gives `blessed-once` the same lane set as `unknown`, so what this one grant means in practice is not established anywhere.
- RISK-VALUE: DERIVED — capabilityTable[TierUnknown] = {} @ trusttier.go:295 — `unknown` is every identity the ledger does not name and the brief is inert on landing, so the only set that leaves every existing answer unchanged is the empty one.
- RISK-VALUE: DERIVED — TierUnknown Tier = iota (0), then 1, 2, 3 @ trusttier.go:57-60, compared by `t >= min` @ trusttier.go:79 — the order is the one the facts state, and making the lowest tier the type's zero value means an unset tier satisfies no minimum above `unknown`.
- RISK-VALUE: DERIVED — a ledger row is passed over when `row.ID == 0 || id == 0 || row.ID != id` @ trusttier.go:256 — the facts require the strict pinned-id path, and the existing strict author check refuses the same three shapes (record never pinned, caller without an id, mismatch). The ledger path has no bot exemption, which only narrows.
- RISK-VALUE: DERIVED — failure arms return TierUnknown with "default-absent" @ trusttier.go:115 (returned at 239 and 265) and "default-unreadable" @ trusttier.go:119 (returned at 242) — the facts require both states to resolve `unknown` and to stay distinguishable. Observed beyond the fixtures: missing file, empty file and comment-only file resolve default-absent; a malformed line after a good row, a half-written last row, a directory, an unreadable file, a string-typed id, a JSON array, trailing text, a byte-order mark and an over-long line all resolve default-unreadable with the announcement.
- RISK-VALUE: NAMED, NOT DERIVED — parseTier accepts "maintainer" @ trusttier.go:92, so a ledger row can resolve the top tier with provenance "ledger-row" @ trusttier.go:111 — the facts above and the published model both say that tier is resolved from the roster and never from the ledger; no source supports accepting it from a row.

Ranked last, reversible by an edit, no derivation attempted: the line-buffer bounds 64*1024 and
1024*1024 (trusttier.go:186; an over-long line was observed to fail closed), the key literal
"ASSAY_CONTRIBUTOR_LEDGER" (one copy in each roster reader and one in the shared vector, held
equal by row 6), the path separator set (desk copy of the roster reader, line 1813), the row's
seven field names (trusttier.go:125-131), the comment prefix "#" (trusttier.go:191), and the
provenance and capability name strings. One further entry is carried in the desk hand-back.

Observations outside the table (handed to the desk; none changes a row result):

1. Row 11 cannot pass on merged main (above). The row needs an explicit base, or a different
   class, to be runnable after the merge.
2. A ledger row naming `maintainer` is honoured (last NAMED, NOT DERIVED line above).
3. Duplicate rows for one identity: the first matching row wins. A promotion followed by a
   later demotion row therefore still resolves the promotion. No test pins either order and
   the row shape carries no scope for the single item a `blessed-once` grant is meant to cover.
4. Two row shapes are passed over without an announcement and resolve default-absent: a tier
   name outside the four, and a row with no id field. Both stay `unknown`.
5. The published `review-depth` column and the lane table that shipped later point in opposite
   directions for `unknown` and `contributor`; no shipped caller reads the capability table.
6. Row 10's comparison walks only the columns the document has (the two surviving mutations),
   and no row pins the standard-error announcement.
7. Housekeeping: the `(planned)` markers on the published model's path are stale; the changelog
   fragment listed under files: has since been folded into the aggregated changelog, so that
   one path is gone at this commit while its content is present; every other listed path exists.
8. The design record this brief is authored against is still PROPOSED with no recorded ruling.

VERIFY: BLOCKED — check-definition: rows 1-10 meet Expect by hand; row 11 exits 2 with a
could-not-check that the row as written cannot avoid on merged main. Evidence only: this brief
is human-gated, and nothing here changes its status.

Desk note, 2026-10-10: the row 11 could-not-check above is an instance of #1915 (the consumers check cannot corroborate on merged main as written). #1915 is the blocker this pass's outcome record names.

## Review
Gate: human (from frontmatter). Reviewer records verdict + date in the stream README table.
Human gate is MANDATORY when any risk answer is yes.
