---
brief: apps-installer/08
title: Solo identity mode — spec and decision for running the desk verbs on the operator's own token
why: >-
  The pilot tier promises "experience the workflow with zero Apps", but every desk verb today
  mints a role token from a role App's key and the trust gate accepts posts only from rostered bot
  logins. Whether the verbs may run on the operator's own login, with the role as a label and
  GitHub's own refusal of self-approval as the restored control, changes the identity model the
  tools enforce. That is a decision for the driver, made on a written spec, not a default an
  implementer picks.
wave: 0
depends: []
unblocks: []
effort: S
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
gate-why: >-
  No risk boolean trips, but the brief changes the identity model the desk tools enforce: a mode in
  which a role token is replaced by the operator's user token weakens bot-attribution (every action
  shows one login) and must not be introduced by an implementer's judgement. The human confirms the
  mode's boundaries — which verbs may run under it, what the roster records, that it is refused on
  a repo whose ruleset expects a bot identity — before any code is authored.
decision-trigger: spec
issues: []
schema: brief-v1
authored: 2026-09-05 by apps-installer authoring session
sources:
  - "./design.md §7 — Solo creates no App and exits after Screen 0; the identity question is a decision."
  - "./README.md — Solo row: everything runs as the operator's login; verdicts land as comments because GitHub refuses a login approving its own PR; merging and every decision stay the operator's."
  - "`tools/desk/internal/deskkit/trust.go` — role Apps are trusted by `<slug>[bot]` login from `ASSAY_TRUSTED_BOT_SLUGS`; humans by `ASSAY_TRUSTED_LOGINS`; `ASSAY_BLESS_LOGIN` blesses content."
  - "`tools/desk/cmd/desktoken/desktoken.go` — `<ROLE>_TOKEN` env overrides the mint for one role; today this is the only non-App token path and it is undocumented as a mode."
  - "freshness-checked 2026-09-05 @ 38e96f7 (origin/main) — no 'solo' or user-token mode exists; the preflight refuses a role without a minted grant."
exec-tier: strong
exec-tier-why: >-
  Question (a) — the spec must decide the trust-model boundaries the facts leave open (which verbs,
  which surfaces, what a ruleset expecting a bot does), and question (c) — a wrong boundary here is
  an authorization change that survives every happy-path test.
---

# Brief 08 — Solo identity mode: spec and decision

## Context
files:
- `docs/streams/apps-installer/solo-identity.md` (new) — the spec.
- `docs/streams/apps-installer/README.md` — status row and the note that the implementation
  brief is authored after the ruling.

single-point-of-failure: in Solo the ONE identity is the operator's, so the separation the tools
otherwise provide by bot login does not exist. The spec must name what replaces it — GitHub's own
refusal of self-approval under one login, and the human merge gate — and where that is NOT enough
(any ruleset or CI check that keys on a bot login), so that the mode refuses rather than degrades.

facts:
- Today's identity surfaces in the tools: `desktoken` (mint per role), `tools/desk/internal/deskkit/trust.go` (who is
  trusted, by login class), `tools/desk/internal/deskkit/preflight.go` (`app-scopes-vs-duties`, cold mint),
  `deskpost`/`deskpr`/`deskfile`/`deskevidence` (`--as-app`, role provenance stamps).
- The `<ROLE>_TOKEN` env override exists and would let all six roles share one user token today;
  the preflight's scopes check cannot read a grant for a user token (no `.perms` sidecar) and
  reports could-not-check.
- GitHub refuses a user approving their own pull request; a review by the PR author can only be
  COMMENT or, on others' PRs, APPROVE / REQUEST_CHANGES.
- The roster today has no representation for "the operator IS the reviewer".

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl.
- Stop at `implemented` (the spec written and the decision issue filed).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Author no code. The implementation brief is written after the ruling and cites it.

## Task
1. Write `solo-identity.md`: (a) the mode's definition and the exact env/roster shape that turns
   it on (`ASSAY_SOLO_LOGIN=<login>` is the candidate; state alternatives); (b) per desk verb, the
   behaviour under Solo — runs as the user token, runs with a role LABEL in the body, or refuses —
   in a table; (c) the trust gate: which login classes are accepted, what `ASSAY_TRUSTED_BOT_SLUGS`
   holds (nothing), how `deskfile --raised-by` stamps; (d) the preflight: which checks are
   could-not-check by construction and how the boot line says so; (e) refusals: a repo whose
   ruleset requires a bot identity, any attempt to APPROVE own PR (the verb downgrades to COMMENT
   and says so); (f) exit criteria: what an operator sees that tells them they have outgrown Solo.
2. `## Human decision` section: the options (adopt the spec as written / adopt with named
   changes / reject Solo as a supported mode) with the consequence of each for the README's tier
   table.
3. File the decision issue per the repo's decision procedure; record its number in `issues:`.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f docs/streams/apps-installer/solo-identity.md && grep -cE -e '^## Human decision' docs/streams/apps-installer/solo-identity.md` | 1 |
| 2 | `grep -cE -e 'desktoken' -e 'deskpost' -e 'deskpr' -e 'deskfile' -e 'deskevidence' -e 'deskflip' docs/streams/apps-installer/solo-identity.md` | ≥ 6 (every write verb has a row) |
| 3 | `grep -cE -e 'could-not-check' docs/streams/apps-installer/solo-identity.md` | ≥ 1 |
| 4 | `grep -cE -e 'COMMENT' -e 'own pull request' docs/streams/apps-installer/solo-identity.md` | ≥ 2 |
| 5 | `grep -cE -e 'ruleset' docs/streams/apps-installer/solo-identity.md` | ≥ 1 |
| 6 | `statusgen --root . --lint` | exit 0 |

## Evidence
<!-- appended at implementation time -->

## Review
Gate: human (decision-trigger: spec). The driver's ruling is recorded on the decision issue and
cited in the stream README; the implementation brief is authored only after it.
