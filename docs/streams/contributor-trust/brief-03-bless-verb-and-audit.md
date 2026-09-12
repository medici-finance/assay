---
brief: assay:assay:contributor-trust:03
title: "`deskbless` — a structured blessing act with a machine marker, a scope, a reason and an audit row"
why: >-
  The blessing is the one act that moves an inbound item from quarantined to actionable, and
  today it is recognised by WHO wrote a comment rather than by what the comment says. So
  "thanks, I'll take a look" admits an item into automation exactly as a considered approval
  does, the record afterwards cannot say what was decided or why, and nobody can list what has
  been admitted without reading every thread. Making the act explicit costs a maintainer one
  command and turns the highest-consequence control in the inbound path into something that
  can be audited, scoped and explained.
wave: 1
depends: ["contributor-trust/02"]
unblocks: ["contributor-trust/05"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: >-
  This brief changes the predicate that admits untrusted inbound content into automation.
  After it, a maintainer's free-text comment no longer blesses anything and only the marked
  form does — a deliberate fail-closed narrowing, but one that silently does nothing when a
  maintainer types a blessing by hand, and silence is the worst failure shape a gate can have.
  The human is confirming exactly that trade: that the narrowing is intended, that the
  bless-then-edit voiding rule survives the change unweakened (Verify rows 4 and 5), and that
  the verb grants one item and never a standing tier.
issues: []
schema: brief-v2
authored: 2026-09-12 by a contributor-trust authoring session
sources:
  - "docs/streams/contributor-trust/spec.md §2 — the measured per-item blessing surface, including its bless-then-edit awareness."
  - "docs/streams/decisions/DR-bless-verb-audit.md — the design record this brief is authored against (PROPOSED)."
  - "statusgen/trustgate.go — the blessing evaluator: a comment by the configured authority admits an issue, numeric-id verified through the richer query, voided by content added or edited after the latest blessing comment. Its deskkit twin is a documented duplicate and both readers are bound by a coupling test over a shared vector file."
  - "docs/streams/contributor-trust/brief-02-trust-tiers-and-ledger.md — the ledger this verb writes a `blessed-once` row into; the tier vocabulary is defined there, not here."
  - "freshness-checked 2026-09-12 @ e96b7f6d (origin/main) — the blessing is still recognised from authorship alone; no marker, no scope, no reason and no audit row exist."
design: DR-bless-verb-audit
decision-trigger: creation
exec-tier: strong
exec-tier-why: >-
  (c) this is the admission gate itself — an error that widens it admits untrusted content
  into automation, and an error that narrows it silently strands genuine items; and (b) the
  evaluator is a documented duplicate across two modules held together by a coupling test that
  must be extended in the same change.
consumers:
  - "tools/desk/cmd/deskbless/main.go: follow-up contributor-trust/03 (this brief; the new verb)"
  - "statusgen/trustgate.go: follow-up contributor-trust/03 (this brief; the evaluator gains the marker form)"
  - "tools/desk/internal/deskkit/trust.go: follow-up contributor-trust/03 (this brief; the documented-duplicate twin must gain the marker form in the same change or the two readers disagree about what is blessed)"
  - "docs/contributor-trust.md: follow-up contributor-trust/03 (this brief; the published model gains the sanctioned admission path)"
  - "CONTRIBUTING.md: follow-up contributor-trust/06 (the contributor-facing statement of how admission happens is authored there, against the wording decision recorded for that brief, not here)"
version: 1
---

# Brief 03 — The blessing verb and its audit row

## Context

files:
- `tools/desk/cmd/deskbless/main.go` (new) — the verb: post a marked blessing comment on one
  item, with a scope and a reason, and write the ledger row. `--dry-run` prints and posts
  nothing.
- `tools/desk/cmd/deskbless/testdata/` — fixtures for each evaluator case below.
- `statusgen/trustgate.go` and `tools/desk/internal/deskkit/trust.go` — the two documented-
  duplicate evaluators gain the marker form, in the same change, with the existing shared
  vector file extended to cover it.
- `docs/contributor-trust.md` (planned) — the sanctioned admission path, added to the published model.
- `changelog/contributor-trust-bless.md` (new).

single-point-of-failure: the marker match. Behind it, two independent layers that fail for
different reasons — the authority check (the comment's author must still be the configured
blessing authority, numeric-id verified, so a marker typed by anybody else admits nothing) and
the bless-then-edit voiding rule (content added or edited after the blessing comment voids it,
whatever the marker says). A forged or copied marker therefore has to also be posted by the
authority AND survive the edit rule.

facts:
- The blessing comment carries a fixed machine-readable marker, the scope (this item only),
  and a free-text reason. The marker is the gate's key; the rest is for the human reading the
  audit row.
- Only the marked form blesses. A free-text comment by the authority no longer admits an item.
  This is a narrowing and is the point of the brief.
- Every existing property of the evaluator is preserved: the author must be the configured
  blessing authority with a matching numeric id, and content added or edited after the latest
  blessing comment voids the blessing.
- A blessing writes one ledger row at tier `blessed-once`, scoped to the item. It never writes
  `contributor` and never writes `maintainer`; a standing promotion is a separate human act.
- The verb refuses, rather than guesses, when the ledger path is unconfigured, when the caller
  is not the blessing authority, or when the item cannot be read. Exit 0 ok, 5 refused,
  6 unverifiable.
- An audit row is appended for every invocation, including refusals, so the record answers
  "what was admitted, by whom, when, and why" without reading threads.
- The comment stays a comment: human-legible, in the thread, visible to the contributor.

## Human decision
<!-- gate: human — decision-trigger: creation. Lifted VERBATIM into the decision issue; self-contained. -->
Inbound issues and pull requests from people the project's automation does not recognise are
held back: they are shown to a maintainer, and no automation touches them until a maintainer
admits that one item. Today, admitting happens when the designated maintainer leaves any
comment at all on the item. The tooling cannot tell the difference between a considered
approval and "thanks, I'll look at this later" — both admit. Afterwards nothing records what
was decided, why, or by what reasoning, and there is no way to list everything that has been
admitted without reading every conversation.

The proposal is to make admitting an explicit act: a command the maintainer runs, which posts
a comment containing a fixed machine-readable marker, the scope (this one item), and a short
reason. The automation would then admit an item only when it finds that marked form. Existing
safeguards stay exactly as they are — the comment must still come from the designated
maintainer's verified account, and any content added to the item after the admitting comment
still cancels the admission.

The consequence to weigh is that after this change, a maintainer who types an approval by hand
without the marker does not admit the item. The automation does nothing, and says nothing. The
item simply stays held.

Options:
1. **Only the marked form admits (recommended)** — one command becomes the sanctioned way to
   admit, and the act carries a scope, a reason and an audit trail. The failure mode is
   "nothing happened", which is the safe direction: a held item is visible on the held queue
   and a maintainer re-runs the command. Consequence accepted: the muscle memory of "just
   comment" stops working, and the published contribution guidance has to say how admission
   now happens.
2. **Accept both the marked form and any comment by the maintainer** — nothing breaks and no
   habit changes. Consequence: the ambiguity the change exists to remove is preserved, so a
   casual remark still admits an item into automation and the audit trail is only as complete
   as the maintainer's discipline.
3. **Use a label instead of a comment as the admitting token** — quick to apply from the web
   interface. Consequence: a label carries no reason and no author in what a reader sees, and
   the cancel-on-later-edit rule has nothing to compare a label against, so that safeguard is
   lost.
4. **Change nothing** — the status quo, with its ambiguity and no audit trail.

Default if no answer: none — blocks until answered; this changes what admits untrusted content
into automation.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Do not weaken or remove either existing safeguard (authority identity, cancel-on-later-edit)
  to make a check pass. If greening requires it, stop and escalate.

## Task

1. Extend both documented-duplicate evaluators with the marker form, in one change, and extend
   the shared vector file that binds them so a divergence fails a test rather than a review.
2. `deskbless`: validate the caller is the blessing authority, post the marked comment with
   scope and reason, write the `blessed-once` ledger row, append the audit row. Kill switch
   first, one audit line per invocation, exits 0 / 5 / 6, fail closed.
3. Fixtures and tests: marked comment by the authority admits; marked comment by anybody else
   does not; unmarked comment by the authority does not; a marked blessing followed by an edit
   to the item does not; a marked blessing on a different item does not admit this one.
4. Add the sanctioned admission path to the published model, and the changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'Bless' -count=1` | exit 0; output contains `ok` | check |
| 2 | `cd statusgen && GOWORK=off go test . -run 'Bless' -count=1` | exit 0; output contains `ok` | check +neighbour |
| 3 | `cd tools/desk && GOWORK=off go build ./cmd/deskbless && ./deskbless --dry-run --repo example-org/example --item 1 --reason 'scoped test' --fixture cmd/deskbless/testdata/authority.json; echo rc=$?` | output contains `rc=0`; output contains `blessed-once` | check |
| 4 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'BlessUnmarkedCommentDoesNotAdmit' -count=1 -v` | exit 0; output contains `PASS` (the narrowing: a free-text comment by the authority admits nothing) | check +mutation |
| 5 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'BlessThenEditVoids' -count=1 -v` | exit 0; output contains `PASS` (the pre-existing safeguard survives the marker change) | check +mutation |
| 6 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'BlessMarkerByNonAuthority' -count=1 -v` | exit 0; output contains `PASS` (a copied marker posted by anybody else admits nothing) | check +mutation |
| 7 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'BlessScopedToOneItem' -count=1 -v` | exit 0; output contains `PASS` (a blessing on one item does not admit another) | check |
| 8 | `cd tools/desk && ./deskbless --dry-run --repo example-org/example --item 1 --reason 'x' --fixture cmd/deskbless/testdata/not-authority.json; echo rc=$?` | output contains `rc=5`; output does not contain `blessed-once` | check +mutation |
| 9 | `cd tools/desk && ./deskbless --dry-run --repo example-org/example --item 1 --reason 'x' --fixture cmd/deskbless/testdata/unreadable-item.json; echo rc=$?` | output contains `rc=6`; output does not contain `rc=0` | check |
| 10 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'TrustReaderCoupling' -count=1` | exit 0; output contains `ok` (the two duplicate evaluators agree on every vector, marker included) | check +flow +neighbour |
| 11 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'BlessPublishedMarkerMatchesEmitted' -count=1 -v` | exit 0; output contains `PASS` (the test reads the marker documented in `docs/contributor-trust.md` (planned) and compares it to the one the verb emits and the evaluator matches, so a documented admission path that does not admit fails) | check +dereference |
| 12 | `statusgen --root . --consumers --brief contributor-trust/03` | exit 0; output does not contain `DISPROVED` | check |

Pre-mortem to detection map. "The marker form lands but the old free-text path is left in as a
fallback, so nothing actually narrows" is caught by row 4. "Adding the marker check
accidentally short-circuits the cancel-on-later-edit rule" is caught by row 5. "A contributor
copies the marker text into their own comment and admits their own item" is caught by rows 6
and 8. "The marker matches too loosely and a blessing on one item admits a neighbouring one"
is caught by row 7. "One of the two duplicate evaluators gains the marker and the other does
not, so the board and the desk disagree about what is admitted" is caught by rows 2 and 10.
"A maintainer types an approval by hand and the item silently stays held" — no row; this is
the accepted consequence of the narrowing, mitigated by the held queue's visibility and by the
published guidance, and it is what the human gate is confirming.

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: human (from frontmatter). Reviewer records verdict + date in the stream README table.
Human gate is MANDATORY when any risk answer is yes.
