---
brief: composability/01
title: Reactive activation — a missing extension key downs one component, not the fleet
why: >-
  Twice in one month a single config key one tool rejected took every desk verb down at once,
  because the tools share one config loader that fail-closes on anything it does not like. The
  trust surface must fail closed; a repo-alias typo must not. With the manifests from brief 00
  the loader can tell which components actually inject a key and deactivate only those, with a
  report, while everything else keeps working. This is the smallest brief in the stream and the
  one that would already have prevented both outages.
wave: 1
depends: ["composability/00"]
unblocks: ["composability/05"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-08 by composability authoring session
sources:
  - "docs/streams/composability/component-model.md §6 (activation rule; trust surface stays fail-closed; extension keys per component)"
  - "arXiv 2608.25512 §3.2 (a component whose dependency is unavailable stays inactive until it appears, without erroring) and §5.3 (reconfiguring a provider reactivates only the dependents whose resolved dependency changed)"
  - "docs/adopting-assay.md (roster section) — the trust/extension split of the ASSAY_* keys; trust keys are fail-closed by design"
  - "house incidents, 2026-09 (two): one tool rejecting one roster key downed a whole desk lane; one unregistered ASSAY_* key fail-closed every verb — both cited by the stream README"
  - "composability/00 — the manifests and the trust-vs-extension inject split this brief reads"
---

# Brief 01 — Reactive activation: a missing extension key downs one component, not the fleet

## Context

files:
- **edit** `tools/desk/internal/deskkit/` — the shared config loader (the roster-config and
  app-config readers): split the single fail-closed path into (a) trust-surface validation,
  unchanged and fail-closed, and (b) per-key extension validation whose result is recorded
  per key rather than thrown.
- **create** `tools/desk/internal/deskkit/activation.go` (planned) — given the parsed manifests (brief
  00's parser) and the per-key validation results, compute ACTIVE / INACTIVE per component and
  the reason for each INACTIVE one.
- **edit** the verb entrypoint shared by the desk commands so that a verb belonging to an
  INACTIVE component refuses with `could-not-check: assay/<component> inactive — <key> <reason>`
  before doing any work.
- **edit** `tools/desk/component.yaml` (planned) and the role-skill manifests only if the inventory shows
  a verb-to-component mapping the manifests do not yet carry.
- **edit** `docs/adopting-assay.md` (roster section) — one paragraph: what an invalid extension key now
  does (deactivates its dependents, reported) vs. what an invalid trust key still does (refuses).

facts:
- **The trust surface does not change.** `ASSAY_BLESS_LOGIN`, `ASSAY_TRUSTED_LOGINS`,
  `ASSAY_TRUSTED_BOT_SLUGS`, `ASSAY_ALLOWED_REPOS`, `ASSAY_HUMAN_LOGIN_MAP` — unset or malformed
  → every verb that injects `assay.roster.trust` refuses, exactly as today. If greening any
  check would require weakening that, STOP and file `needs-decision`.
- **Extension keys become per-component.** `ASSAY_REPO_ALIASES`, `ASSAY_REPO_FORGES`,
  `ASSAY_RISK_CALLOUT`, `ASSAY_WRITEGUARD_CALLOUT`, `ASSAY_RELEASE_REPO`, `ASSAY_SCAN_REPOS`,
  `ASSAY_CHANNEL_DRIFT_TARGET`, `ASSAY_HOME_REPO` — each maps to `assay.roster.ext.<name>`; a
  rejected value deactivates the components whose manifest lists that key as `required` and
  no others. An unknown `ASSAY_*` name is a NOTICE, never a refusal.
- **Which verb belongs to which component comes from the manifests**, not from a table in
  code. `tools/desk/component.yaml` (planned) provides `assay.desk.verbs`; the role skills provide
  `assay.desk.role.<role>` and inject the extension keys their procedure reads. If a verb
  has no owning component the loader treats it as injecting `assay.roster.trust` only.
- **Three-state output is mandatory** (`spec/brief-v1.md` §8): the refusal is
  `could-not-check`, not a silent no-op and not a generic error.
- **A provider that arrives later activates its dependents on the next invocation** — there is
  no daemon state to refresh; every verb recomputes activation at start.

## Ground rules

- Do not `git push`, trigger workflows, or run mutating infrastructure commands unless
  explicitly instructed. The deliverable is a draft PR opened by the desk verbs.
- Stop at `implemented`. Do not set `verified` or `done`.
- If a key's classification (trust vs extension) is ambiguous in `docs/adopting-assay.md` (roster section),
  treat it as trust (fail-closed) and raise `NEEDS_CONTEXT` on the PR — never downgrade a key
  to extension to make a test pass.
- No house value in tests or fixtures; test with synthetic keys and synthetic manifests.

## Task

1. In the shared loader, separate trust-surface validation (unchanged) from extension-key
   validation. Extension validation returns a per-key result (`ok` / `invalid <why>` /
   `unset`) instead of aborting the load.
2. Implement activation over the manifests: a component is ACTIVE iff every
   `inject.required` key is provided by an ACTIVE component (transitively) and, for roster
   keys, the key's validation result is `ok` (or `unset` for `optional`).
3. At verb entry, look up the verb's owning component; if INACTIVE, print the three-state
   refusal naming the component, the key, and the reason, and exit 2.
4. Add tests: synthetic manifests A (injects `ext.x`) and B (does not); `ext.x` invalid →
   A's verb refuses with the message, B's verb runs; trust key unset → both refuse.
5. Document the behaviour change in `docs/adopting-assay.md` (roster section).

## Verify

| # | Command | Expect |
|---|---------|--------|
| 1 | `ASSAY_REPO_ALIASES='not=a=valid=shape' deskboard --help` (a verb whose component does not inject `repo-aliases`) | exit 0; runs |
| 2 | `ASSAY_REPO_ALIASES='not=a=valid=shape' <a verb whose manifest requires assay.roster.ext.repo-aliases>` | exit 2; first line `could-not-check: assay/<component> inactive — assay.roster.ext.repo-aliases invalid` |
| 3 | `ASSAY_TRUSTED_LOGINS= deskboard --help` | non-zero; refuses (fail-closed trust surface unchanged) |
| 4 | `ASSAY_TRUSTED_LOGINS= <the verb from row 2, with a valid ASSAY_REPO_ALIASES>` | non-zero; refuses on the trust key (trust outranks extension) |
| 5 | `ASSAY_UNKNOWN_KEY=1 deskboard --help` | exit 0; a NOTICE line mentions `ASSAY_UNKNOWN_KEY`; no refusal |
| 6 | mutation: revert `activation.go`'s per-key result to an abort; run row 1 | row 1 now fails (the old fleet-wide behaviour is back); restore |
| 7 | mutation: reclassify one trust key as extension in the loader; run row 3 | row 3 now exits 0 — which is WRONG; the test suite must fail on this; restore |
| 8 | `cd tools/desk && go test ./internal/deskkit/... ./cmd/...` | exit 0 |
| 9 | neighbour: `deskmanifest lint --root .` | exit 0 (brief 00's lint still clean after any manifest edits) |
| 10 | flow: with a valid roster, run one verb from each of three components in sequence | all exit 0; no `inactive` line |

## Evidence

Pending — the implementer records its run here on reaching `implemented`; an independent
runner records a second run on merged main before `verified`.

## Review

gate: model — pending.
