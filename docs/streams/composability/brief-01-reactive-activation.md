---
brief: assay:assay:composability:01
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
schema: brief-v2
authored: 2026-09-08 by composability authoring session
sources:
  - "docs/streams/composability/component-model.md §6 (activation rule; trust surface stays fail-closed; extension keys per component)"
  - "arXiv 2608.25512 §3.2 (a component whose dependency is unavailable stays inactive until it appears, without erroring) and §5.3 (reconfiguring a provider reactivates only the dependents whose resolved dependency changed)"
  - "docs/adopting-assay.md (roster section) — the trust/extension split of the ASSAY_* keys; trust keys are fail-closed by design"
  - "house incidents, 2026-09 (two): one tool rejecting one roster key downed a whole desk lane; one unregistered ASSAY_* key fail-closed every verb — both cited by the stream README"
  - "composability/00 — the manifests and the trust-vs-extension inject split this brief reads"
version: 1
id: a7513cbd-6ab8-472f-8360-4b953b40e489
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
### Non-implementer verifier run — VERIFY: FAIL — 8/10 pass, rows 2 and 5 fail — 2026-09-30 claude-opus-5-5-verifier

Runner is not the implementer. Isolated worktree at merged main `b89b3957225e227e69d5b5ec7949344f580d9966` (HEAD == the forge's `commits/main`, cross-checked before and after). Implementing PR #957 (merge 743195b97). `gate: model`. Binaries built from main. **Grounding:** every desk verb is write-class and reads the roster only from the config-home roster file, so the Verify rows' env-prefixed forms reach no loader; each row was run literally and again through a roster file (synthetic HOME, synthetic keys `ada:2001`, `example-org/tracker`). Failures filed as #1843. Status stays `implemented`.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | roster file with `ASSAY_REPO_ALIASES=not=a=valid=shape`; `deskboard --help`; `deskreply --help` | exit 0; runs | PASS — exit 0 / 0; no `inactive` line, help printed (literal env form: exit 0, never reaches the loader) | 2026-09-30 | claude-opus-5-5-verifier |
| 2 | synthetic manifest tree (desk-tools requires `assay.roster.ext.repo-aliases`), invalid alias in the roster file; `deskreply --help` | exit 2; first line `could-not-check: assay/<component> inactive — assay.roster.ext.repo-aliases invalid` | FAIL — exit 6; `could-not-check: assay/assay/desk-tools inactive — assay.roster.ext.repo-aliases ASSAY_REPO_ALIASES: entry "not=a=valid=shape" has no ':' …`; the `assay-config:` echo block precedes it; component name doubled (activation.go:291). No real manifest requires the key, so the row runs only against a synthetic tree | 2026-09-30 | claude-opus-5-5-verifier |
| 3 | roster file: bless set + empty trusted logins; trust surface unset; malformed `ASSAY_TRUSTED_LOGINS=bob:notanid` | non-zero; refuses | PASS on intent — 0 / 6 / 6; the bless login counts as trusted (unchanged from pre-#957); unset or malformed: `… inactive — assay.roster.trust unset or malformed` | 2026-09-30 | claude-opus-5-5-verifier |
| 4 | roster file: trust unset + valid alias; then trust unset + invalid alias; `deskreply --help` | non-zero; refuses on the trust key | PASS — exit 6 / 6; both `… inactive — assay.roster.trust unset or malformed` (trust outranks extension) | 2026-09-30 | claude-opus-5-5-verifier |
| 5 | roster file with `ASSAY_UNKNOWN_KEY=1`; `deskboard --help` | exit 0; NOTICE mentions `ASSAY_UNKNOWN_KEY`; no refusal | FAIL — exit 6; `REFUSED — ASSAY_UNKNOWN_KEY: unknown key in the ASSAY_ namespace …`, no NOTICE. Declared deviation in #957; ruling open in #1261 | 2026-09-30 | claude-opus-5-5-verifier |
| 6 | mutation: any invalid `Ext` aborts `LoadConfig`; rebuild; row 1 (file form); restore | row 1 now fails | PASS — exit 6, `REFUSED — MUTANT abort: repo-aliases …`; deskkit suite fails 6 tests incl. the repo-alias deactivation test; restored | 2026-09-30 | claude-opus-5-5-verifier |
| 7 | mutations: (a) malformed trusted-login entries to a non-aborting accumulator; (b) trust gate disabled; row 3; restore | row 3 exits 0 and the suite fails | PASS — mutants exit 0 / 0; suite FAILs on the malformed-id test (a) and 4 trust-unset tests (b); restored | 2026-09-30 | claude-opus-5-5-verifier |
| 8 | `cd tools/desk && go test ./internal/deskkit/... ./cmd/...` | exit 0 | PASS — exit 0; 67 `ok` packages, no FAIL | 2026-09-30 | claude-opus-5-5-verifier |
| 9 | `deskmanifest lint --root .` | exit 0 | PASS — exit 0; `checked-clean: 26 manifest(s), every inject.required resolves in range, no cycles` | 2026-09-30 | claude-opus-5-5-verifier |
| 10 | valid roster: `deskboard --help`, `deskversion --version`, `deskreply --help`, `deskroster --help`, `deskfile --help` | all exit 0; no `inactive` line | PASS — exit 0 ×5, no `inactive` line. Caveat: every desk binary maps to the one `assay.desk.verbs` provider, so "three components" cannot hold as written | 2026-09-30 | claude-opus-5-5-verifier |

RISK-VALUE (fail-safe trigger fires: the diff touches the writeguard command's main.go, reviewer-labelled `surface:core`; all values reversible):

- RISK-VALUE: DERIVED — `extKeyNames` @ tools/desk/internal/deskkit/rosterconfig.go:864-874 — none of the brief's five trust keys is in the map; all eight listed extension keys are. A ninth, `run-credentials` (line 871), was added 2026-09-23 outside #957; it decides which credential a write runs as, so under the brief's "ambiguous → treat as trust" rule it deserves a reviewer look.
- RISK-VALUE: DERIVED — `"assay.roster.trust"` gate @ tools/desk/internal/deskkit/activation.go:191 — resolves only when `Configured()` holds, the pre-existing fail-closed definition (rows 3, 4, mutation 7b).
- RISK-VALUE: NAMED, NOT DERIVED — `ExitUnverifiable = 6` @ tools/desk/internal/deskkit/exitcodes.go:55 — the brief pins exit 2; 6 was chosen after review found 2 collides with usage errors, but the brief was never amended. Brief-owner call, carried on #1843.

Findings (filed in #1843): the doubled `assay/` prefix at activation.go:291; `DiscoverManifests` walks untracked nested clones/worktrees, and duplicate component names resolve last-wins after an unstable sort, so a nested worktree can change the parent checkout's activation; the env-prefixed Verify rows should be restated in roster-file form. Not re-checked this pass: whether the #957 security-review note (duplicate non-exclusive providers resolve by sort order) was filed.

### 2026-10-02 desk dispatch — Non-implementer verifier re-run — 2026-10-02T23:10Z (UTC)

Runner is not the implementer. Isolated detached worktree at merged main e1d99484ffd9 (equal to the forge's main head at run time). `gate: model`; risk answers all `no`. Binaries built from that tree; throwaway HOME for every row. Since the 2026-09-30 run (main b89b3957225e): #1929 names the inactive component verbatim (no more doubled prefix) and scopes manifest discovery to the checkout; #1872 moves ASSAY_RUN_CREDENTIALS to the trust surface. The brief text is unchanged, #1843 and #1261 are still open.

**Grounding.** Desk verbs are write-class: they read the roster only from the config-home roster file, never from the environment. So the env-prefixed forms in rows 1-5 never reach the loader. Each row was run literally and again in roster-file form, with a synthetic roster (login `ada:2001`, repo example-org/tracker, synthetic reviewer slug). No shipped manifest requires `assay.roster.ext.repo-aliases`: the-desk and worker-desk list it as optional only. So rows 2 and 4 ran in a scratch clone whose desk-tools manifest also requires that key.

| # | Command | Exit | Observed output | Date | Runner |
| --- | --- | --- | --- | --- | --- |
| 1 | roster file sets ASSAY_REPO_ALIASES to `not=a=valid=shape`; `deskboard --help`, then `deskreply --help` (env-prefixed form also run) | 0 / 0 (env form 0) | PASS — help printed, no `inactive` line. The env-prefixed form exits 0 even against a build where an invalid extension aborts the load, so only the file form is a real test | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | synthetic manifest (desk-tools requires `assay.roster.ext.repo-aliases`), roster file with the invalid alias; `deskreply --help` | 6 | FAIL — refusal reads `could-not-check: assay/desk-tools inactive — assay.roster.ext.repo-aliases ASSAY_REPO_ALIASES: entry "not=a=valid=shape" has no ':' …`. The component name is now correct (fixed by #1929). Two gaps remain: the exit is 6, not the pinned 2, and the refusal is the last of 18 stderr lines, after the `assay-config:` echo block, not the first line. Control with a well-formed alias: exit 0 | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | roster file with no bless and empty ASSAY_TRUSTED_LOGINS; roster file with `ASSAY_TRUSTED_LOGINS=bob:notanid` ; `deskboard --help` (env-prefixed form also run) | 6 / 6 | PASS — both print `could-not-check: assay/desk-tools inactive — assay.roster.trust unset or malformed`. Bless set with empty trusted logins exits 0, because the bless login counts as trusted (same as before #957). The env-prefixed form exits 0 when a valid roster file is present and 6 with no roster file, so it does not test this | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | synthetic manifest from row 2; roster file with trust unset and a well-formed alias, then trust unset and the invalid alias; `deskreply --help` | 6 / 6 | PASS — both print `… inactive — assay.roster.trust unset or malformed`. Note: when the extension key is declared before `assay.roster.trust` in inject.required and both are bad, the refusal names the extension key instead. It still refuses (exit 6) | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | roster file with `ASSAY_UNKNOWN_KEY=1`; `deskboard --help` | 6 | FAIL — `assay-config: REFUSED — ASSAY_UNKNOWN_KEY: unknown key in the ASSAY_ namespace …` then `could-not-check: assay/desk-tools inactive — assay.roster.trust unset or malformed`; no NOTICE. Deviation declared in #957; the ruling is still open in #1261 | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | mutation in a scratch clone: an invalid extension key also appends a roster problem (aborts the load); rebuild; row 1 file form; restore | 6 | PASS — mutant prints `REFUSED — MUTANT abort: ASSAY_REPO_ALIASES …` and the trust-inactive refusal; the deskkit suite fails 7 tests, including the repo-alias deactivation and extension-refusal tests; restored | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | mutations in a scratch clone: (a) malformed ASSAY_TRUSTED_LOGINS entries go to a non-aborting extension accumulator; (b) ASSAY_TRUSTED_LOGINS added to the extension catalogue; row 3 and deskkit suite; restore | 0 (a); suite 1 / 1 | PASS — mutant (a) exits 0 on the malformed-trust roster (the wrong result, as the row predicts), and the suite fails the malformed-id test. Mutant (b) fails TestExtCatalogueAllowList. Restored | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | `cd tools/desk && go test ./internal/deskkit/... ./cmd/...` | 0 | PASS — 68 `ok` packages, no FAIL, no deadline hit | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 9 | `deskmanifest lint --root .` | 0 | PASS — `checked-clean: 26 manifest(s), every inject.required resolves in range, no cycles` | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 10 | valid roster file: `deskboard --help`, `deskversion --version`, `deskreply --help`, `deskroster --help`, `deskfile --help`, `deskmanifest lint --root .` | 0 ×6 | PASS — no `inactive` line. Caveat: every desk binary maps to the single provider of `assay.desk.verbs`, so "three components" cannot hold as written | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |

Execution witness (statusgen v1.0.31, `verifyrun --dry-run`, throwaway HOME, no roster file): 2/10 rows proven (rows 8 and 9), exit 2.
- Rows 1, 3 and 5 exit 6, because there is no roster file, so the trust surface is unset. For row 3 the witness reads the expectation as exit 0, although the brief expects non-zero.
- Rows 2 and 4 carry unsubstituted placeholders.
- Rows 6, 7 and 10 are prose rather than commands.

Vacuity: the env-prefixed forms of rows 1, 3 and 4 do not exercise the loader for write-class verbs. Rows 9 and 10 would also pass without this brief's work (neighbour and regression rows). Row 2 can only run against a synthetic manifest.

RISK-VALUE (fail-safe trigger: the diff touches the writeguard command entrypoint; every value below is reversible by edit and redeploy):
- RISK-VALUE: NAMED, NOT DERIVED — activation refusal exit binding `ExitUnverifiable = 6` @ exitcodes.go:55 (deskkit; the constant predates #957, and #957 binds every verb's refusal to it, e.g. deskboard main.go:155-156). The brief pins exit 2. Moving to 6 avoids a clash with the usage-error code, but the brief was never amended, so the brief owner has to choose. Tracked on #1843.
- RISK-VALUE: DERIVED — `extKeyNames` @ rosterconfig.go:881-890 (deskkit) holds exactly the brief's eight extension keys and none of the five trust keys. ASSAY_RUN_CREDENTIALS is now a trust key (#1872). An allow-list test pins the catalogue (mutation 7b).
- RISK-VALUE: DERIVED — the `"assay.roster.trust"` gate @ activation.go:247 resolves only when `Configured()` @ rosterconfig.go:944-945 holds: no problems, bless login set, at least one trusted login. That is the existing fail-closed definition (rows 3 and 4, mutation 7a).

VERIFY: FAIL — 8/10 rows pass on merged main e1d99484ffd9, rows 2 and 5 fail. Row 2 refuses with exit 6, not the pinned 2, and its refusal is not the first line; that needs a brief amendment, carried on #1843. Row 5 refuses an unknown key where the brief expects a NOTICE, pending the #1261 ruling. Status stays implemented.
### 2026-10-07 desk dispatch — Non-implementer verifier re-run — VERIFY: FAIL — 8/10 pass, rows 2 and 5 fail

Runner is not the implementer. Isolated detached worktree at merged main 91f04b81ba06 (equal to the forge's main head by ls-remote at run time). `gate: model`; risk answers all `no`. Binaries built from that tree; throwaway HOME for every row. Since the 2026-10-02 run (main e1d99484ffd9) the deskkit loader, activation code, manifests and this brief are unchanged on the lines these rows exercise (ExitUnverifiable = 6, the unknown-key refusal, the refusal format, tools/desk component manifest); #1843 and #1261 are still open. The failure reproduces.

**Grounding.** Desk verbs are write-class and read the roster only from the config-home roster file, so the env-prefixed forms in rows 1-5 never reach the loader. Each row was run literally and again in roster-file form, with a synthetic roster (login `ada:2001`, repo example-org/tracker, synthetic reviewer slug). No shipped manifest requires `assay.roster.ext.repo-aliases` (the-desk and worker-desk list it as optional), so rows 2 and 4 ran in a scratch clone whose desk-tools manifest also requires that key.

| # | Command | Exit | Observed output | Date | Runner |
| --- | --- | --- | --- | --- | --- |
| 1 | roster file sets ASSAY_REPO_ALIASES to `not=a=valid=shape`; `deskboard --help`, then `deskreply --help` (env-prefixed form also run) | 0 / 0 (env form 0) | PASS — help printed, no `inactive` line; the echo shows ASSAY_REPO_ALIASES empty (rejected value falls back to the unset default) | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 2 | synthetic manifest (desk-tools requires `assay.roster.ext.repo-aliases`), roster file with the invalid alias; `deskreply --help` | 6 | FAIL — refusal reads `could-not-check: assay/desk-tools inactive — assay.roster.ext.repo-aliases ASSAY_REPO_ALIASES: entry "not=a=valid=shape" has no ':' separating short label from product …`. Component name and key are right; the exit is 6, not the pinned 2, and the refusal is line 18 of 18 on stderr, after the `assay-config:` echo block, not the first line. Control with a well-formed alias: exit 0. Env-prefixed form: exit 0 (never reaches the loader) | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 3 | roster file with no bless and empty ASSAY_TRUSTED_LOGINS; roster file with `ASSAY_TRUSTED_LOGINS=bob:notanid`; `deskboard --help` (env-prefixed form also run) | 6 / 6 | PASS — both print `could-not-check: assay/desk-tools inactive — assay.roster.trust unset or malformed`; the malformed one also prints `REFUSED — ASSAY_TRUSTED_LOGINS: cannot parse entry "bob:notanid"`. Bless set with empty trusted logins exits 0 (the bless login counts as trusted, unchanged). Env form exits 0 with a valid roster file and 6 with no roster file, so it does not test this | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 4 | synthetic manifest from row 2; roster file with trust unset and a well-formed alias, then trust unset and the invalid alias; `deskreply --help` | 6 / 6 | PASS — both print `could-not-check: assay/desk-tools inactive — assay.roster.trust unset or malformed` (trust outranks extension). Env form exits 0 with a valid roster file | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 5 | roster file with `ASSAY_UNKNOWN_KEY=1`; `deskboard --help` (env-prefixed form also run) | 6 (env form 0) | FAIL — `assay-config: REFUSED — ASSAY_UNKNOWN_KEY: unknown key in the ASSAY_ namespace …` then `could-not-check: assay/desk-tools inactive — assay.roster.trust unset or malformed`; no NOTICE. Env form exits 0 but prints no NOTICE naming the key either. Deviation declared in #957; the ruling is still open in #1261 | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 6 | mutation in a scratch clone: recordExt also appends `MUTANT abort` to the roster problems for any invalid extension key; rebuild; row 1 file form; deskkit suite; restore | 6 / 6; suite 1 | PASS — deskboard and deskreply print `REFUSED — MUTANT abort: ASSAY_REPO_ALIASES …` then the trust-inactive refusal (the old fleet-wide behaviour is back); the deskkit suite fails 7 tests, including TestRepoAliasMalformedDeactivatesOnlyItsDependents and TestVerbActivationRefusal_ExtensionKeyInvalid; clone discarded, main tree untouched | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 7 | mutation in a scratch clone: malformed ASSAY_TRUSTED_LOGINS entries go to a non-aborting extension accumulator, and ASSAY_TRUSTED_LOGINS is added to the extension catalogue as `trusted-logins`; row 3 malformed form; deskkit suite; restore | 0; suite 1 | PASS — the mutant exits 0 on the malformed-trust roster (the wrong result the row predicts); the suite fails TestExtCatalogueAllowList and TestMalformedIDNeverDegradesToLoginOnlyTrust; clone discarded, main tree untouched | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./internal/deskkit/... ./cmd/...` | 0 | PASS — 68 `ok` packages, no FAIL | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 9 | `deskmanifest lint --root .` | 0 | PASS — `checked-clean: 26 manifest(s), every inject.required resolves in range, no cycles` (same with no roster file) | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 10 | valid roster file: `deskboard --help` and `deskreply --help` (assay/desk-tools), `statusgen --version` (assay/statusgen), the assay/hooks inject-resident-rules hook script | 0 ×4 | PASS — no `inactive` line. Caveat: every desk binary maps to the single provider of `assay.desk.verbs`; the statusgen and hook entries do not pass through the desk activation check, so only one component here exercises activation | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |

Execution witness (statusgen v1.0.32, `verifyrun` non-dry, throwaway HOME with the synthetic valid roster): 5/10 rows proven (1, 3, 5, 8, 9), exit 2. Rows 2 and 4 carry unsubstituted placeholders; rows 6, 7 and 10 are prose, not commands. Rows 1, 3 and 5 pass on exit status only, in the env-prefixed form that never reaches the loader: row 3's witness pass is at exit 0 although the brief expects non-zero, and row 5's witness cannot see the missing NOTICE.

Vacuity: the env-prefixed forms of rows 1, 3, 4 and 5 do not exercise the loader for write-class verbs. Rows 9 and 10 would also pass without this brief's work. Row 2 can only run against a synthetic manifest.

RISK-VALUE (fail-safe trigger: the implementing diff touches the writeguard command entrypoint; every value below is reversible by edit and redeploy; enumeration over the deskkit loader, activation code and desk-tools manifest the brief names):
- RISK-VALUE: NAMED, NOT DERIVED — activation refusal exit binding ExitUnverifiable = 6 @ exitcodes.go:55 (deskkit; bound per verb, e.g. deskboard main.go:156). The brief pins exit 2. 6 avoids the usage-error clash, but the brief was never amended, so the brief owner has to choose. Tracked on #1843.
- RISK-VALUE: DERIVED — extKeyNames @ rosterconfig.go:881-890 (deskkit) holds exactly the brief's eight extension keys and none of the five trust keys; ASSAY_RUN_CREDENTIALS stays a trust key. TestExtCatalogueAllowList pins it (mutation 7).
- RISK-VALUE: DERIVED — the "assay.roster.trust" gate @ activation.go:247 resolves only when Configured() @ rosterconfig.go:944-945 holds: no problems, bless login set, at least one trusted login. That is the existing fail-closed definition (rows 3 and 4, mutation 7).

**VERIFY: FAIL** — 8/10 rows pass on merged main 91f04b81ba06, rows 2 and 5 fail, the same two as on 2026-10-02. Row 2 refuses with exit 6, not the pinned 2, and the refusal is not the first line (brief amendment needed, carried on #1843). Row 5 refuses an unknown key where the brief expects a NOTICE, pending the #1261 ruling. Status stays implemented.

Failure re-confirmed and attached: https://github.com/medici-finance/assay/issues/1843#issuecomment-6038031896. That comment records the NAMED, NOT DERIVED value ExitUnverifiable = 6 (tools/desk/internal/deskkit/exitcodes.go:55). Row 5 waits on the needs-decision ruling at https://github.com/medici-finance/assay/issues/1261.

## Review

gate: model — pending.
