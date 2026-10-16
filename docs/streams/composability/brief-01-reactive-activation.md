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
### Non-implementer verifier run — 2026-10-14 claude-opus-5-5 verifier at c690023a864b (FAIL 7/10: rows 2 and 5 fail as on 2026-10-07, row 8 could-not-check on environment)

Verification-Attestation: medici-finance/assay#2925 run=b73e24dd1027607b6101cf2c4dcff147e0b2faf2106172214e0240c8b682504a source=c690023a864b1c1cfdd55c0a28e19d22e572048d brief=docs/streams/composability/brief-01-reactive-activation.md model=claude-opus-5-5 tier=strong

Runner is not the implementer. Detached worktree at merged main c690023a864b, admitted by the dispatcher's verifier check before any row ran. `gate: model`; risk answers all `no`. Delivering change: #957 (merge 743195b970b6). Binaries were built from that tree into a scratch directory. Every row used a throwaway HOME holding a synthetic roster (login `ada:2001`, no real deployment values). Since the 2026-10-07 run (main 91f04b81ba06), the activation code, the extension catalogue, the unknown-key refusal and the desk-tools manifest have not changed on the lines these rows exercise. The one visible change is the config echo, which is now a single digest line by default (#2719). Because of it, the row 2 refusal is on line 2 of stderr, where it was line 18. #1843 and the needs-decision ruling #1261 are both still open.

**Grounding (derived from the brief and main before reading the diff).** The desk verbs are write-class, and write-class tools read the roster only from the config-home roster file, never from the environment. The env-prefixed forms in rows 1-5 therefore never reach the loader. Each of those rows was run literally and again in roster-file form. No shipped manifest lists `assay.roster.ext.repo-aliases` as required (the-desk and worker-desk list it as optional). Activation is also checked only for the single `assay.desk.verbs` provider, assay/desk-tools. Rows 2 and 4 therefore ran in a scratch clone whose desk-tools manifest adds that key to inject.required.

| # | Command | Result | Output | Date | Runner |
| --- | --- | --- | --- | --- | --- |
| 1 | roster file sets ASSAY_REPO_ALIASES to `not=a=valid=shape`; `deskboard --help`, then `deskreply --help`; literal env-prefixed form also run against a valid roster | pass exit=0 | deskboard 0, deskreply 0, literal form 0. Help is printed with no `inactive` line, and the echo shows ASSAY_REPO_ALIASES empty because the rejected value falls back to the unset default | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 2 | scratch clone whose desk-tools manifest requires `assay.roster.ext.repo-aliases`; roster file holds the invalid alias; `deskreply --help` (also `deskboard --help`) | fail exit=6 | Both exit 6 with `could-not-check: assay/desk-tools inactive — assay.roster.ext.repo-aliases ASSAY_REPO_ALIASES: entry "not=a=valid=shape" has no ':' separating short label from product …`. The component and key are right. The exit is 6, where the brief pins 2. The refusal is line 2 of 2 on stderr, after the one-line config digest, so it is not the first line. Control with a well-formed alias: exit 0. Literal env-prefixed form: exit 0, because it never reaches the loader | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 3 | roster file with no bless login and empty ASSAY_TRUSTED_LOGINS; roster file with `ASSAY_TRUSTED_LOGINS=bob:notanid`; `deskboard --help`; literal env-prefixed form also run | pass exit=6 | Both roster-file forms exit 6 with `could-not-check: assay/desk-tools inactive — assay.roster.trust unset or malformed`. The malformed form also prints `REFUSED — ASSAY_TRUSTED_LOGINS: cannot parse entry "bob:notanid"`. The literal form exits 0 against a valid roster file, because the environment is not read, and exits 6 with no roster file present. The literal form therefore does not exercise this row | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 4 | row 2's scratch clone; roster file with trust unset and a well-formed alias, then trust unset and the invalid alias; `deskreply --help` | pass exit=6 | Both runs exit 6 with `could-not-check: assay/desk-tools inactive — assay.roster.trust unset or malformed`: trust outranks the extension key | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 5 | roster file with `ASSAY_UNKNOWN_KEY=1` added to a valid roster; `deskboard --help`; literal env-prefixed form also run | fail exit=6 | Roster-file form: `assay-config: REFUSED — ASSAY_UNKNOWN_KEY: unknown key in the ASSAY_ namespace …`, then `could-not-check: assay/desk-tools inactive — assay.roster.trust unset or malformed`. No NOTICE is printed. The literal form exits 0, but its only stderr line is the config digest and no NOTICE names the key. The loader still refuses an unknown key (rosterconfig.go:1371-1373); the ruling is pending on #1261 | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 6 | mutation in a scratch clone: parseConfig appends a `MUTANT abort` problem for every extension key recorded invalid, which turns the per-key result back into a whole-config abort (the per-key result is produced in rosterconfig.go, not activation.go); rebuild; row 1 roster-file form; deskkit suite; clone discarded | pass exit=6 | Mutant deskboard and deskreply both exit 6 with `REFUSED — MUTANT abort: ASSAY_REPO_ALIASES …` and then the trust-inactive refusal, so the fleet-wide behaviour is back. The mutant control with a valid roster exits 0. The deskkit suite fails 10 tests. 8 of them are caused by the mutant, including Test Repo Alias Malformed Deactivates Only Its Dependents and Test Verb Activation Refusal Extension Key Invalid. The other 2 also fail in an unmutated control clone | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 7 | mutation in a scratch clone: a malformed ASSAY_TRUSTED_LOGINS entry goes to a non-aborting extension accumulator, and the key is added to the extension catalogue as `trusted-logins`; row 3 malformed form; deskkit suite; clone discarded | pass exit=0 | The mutant exits 0 on the malformed-trust roster, which is the wrong result the row predicts. The deskkit suite fails, and the failures caused by the mutant are Test Ext Catalogue Allow List and Test Malformed ID Never Degrades To Login Only Trust. Both pass in the unmutated control clone | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 8 | `cd tools/desk && go test ./internal/deskkit/... ./cmd/...` | could-not-check exit=1 | 67 packages `ok`; 2 packages FAIL, both on environment. deskkit fails only the heartbeat-boundary subtest of Test Guard Stop Flags, the one-second scheduling-margin flake tracked on #2776, which passes 5 of 5 when run alone. cellctl fails Test Two Sessions Comms Hello Round Trip and the herdr case of Test Comms Foreign Surface: the comms gateway socket cannot bind because the session temp path pushes the unix socket path past the platform length limit. Every activation, catalogue and trust test passes. With a symlinked short temp dir, unrelated callout and worktree tests fail on path resolution instead, so the row cannot be made clean on this host | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 9 | `deskmanifest lint --root .` | pass exit=0 | `checked-clean: 26 manifest(s), every inject.required resolves in range, no cycles` | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 10 | valid roster file: `deskboard --help`, `deskreply --help` and `deskversion --version` (assay/desk-tools), `statusgen --version` (assay/statusgen), and the inject-resident-rules hook script (assay/hooks), in sequence | pass exit=0 | All five exit 0 and none prints an `inactive` line. Caveat: every desk binary maps to the single `assay.desk.verbs` provider, and the statusgen and hook entries do not pass through the desk activation check. Only one of the three components exercises activation, so "three components" cannot hold as the row is written | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |

Vacuity: the env-prefixed forms of rows 1, 3, 4 and 5 do not exercise the loader for write-class verbs. Rows 9 and 10 would also pass without this brief's work. Row 2 can only run against a synthetic manifest.

RISK-VALUE. The fail-safe trigger fires because the implementing diff touches the writeguard command entrypoint. Enumeration covered the #957 diff over the deskkit loader, activation.go, the per-verb entrypoint bindings and the desk-tools manifest. It found: the refusal exit binding; the extension catalogue (9 entries); the `assay.roster.trust`, `assay.roster.ext.` and `assay.desk.verbs` key literals; the trust predicate; and the refusal format string. All of them can be reversed by an edit and a redeploy. Ranked by blast radius:
- RISK-VALUE: DERIVED — extKeyNames @ tools/desk/internal/deskkit/rosterconfig.go:953-967. It holds exactly the brief's eight extension keys plus os-user-login-map, which was added after #957. It holds none of the brief's five trust keys, and ASSAY_RUN_CREDENTIALS is absent. That is right because a trust key in this map would stop failing closed. os-user-login-map only labels a forensic audit row and authorises nothing. The docs list it with the extension surfaces (docs/adopting-assay.md:684-687), so the brief's "ambiguous means trust" rule does not apply. Test Ext Catalogue Allow List pins the map, and mutation 7 reddens it.
- RISK-VALUE: DERIVED — the `"assay.roster.trust"` gate @ tools/desk/internal/deskkit/activation.go:247 resolves only when Configured() @ tools/desk/internal/deskkit/rosterconfig.go:1022 holds: `len(c.Problems) == 0 && c.Bless.Login != "" && len(c.Logins) > 0`. That is the fail-closed definition that predates #957, unchanged, so the trust surface is no weaker than before. Rows 3 and 4 and mutation 7 exercise it.
- RISK-VALUE: DERIVED — ExitUnverifiable = 6 @ tools/desk/internal/deskkit/exitcodes.go:55, bound per verb (for example tools/desk/cmd/deskboard/main.go:167). The repo's own exit-code derivation (exitcodes.go:15-30, pinned by its exit-code table test) reserves 2 for shell usage errors. It starts refusal classes at 3, with 6 for "could not positively verify". A could-not-check refusal is that class, so 6 is the right value. The brief's pinned 2 would collide with usage errors. Row 2 fails on the brief's text, not on the code; the fix is a brief amendment, tracked on #1843.
- RISK-VALUE: DERIVED — the `assay.roster.ext.` prefix @ activation.go:253 and `assay.desk.verbs` @ activation.go:304 match the catalogue in components/KEYS.md (lines 32-47) and the providers in components/roster and tools/desk component.yaml. The refusal format @ activation.go:353 is display text and ranks last.

Findings: (1) As written, rows 2 and 4 need a placeholder filled and a synthetic manifest, and rows 6, 7 and 10 are prose. The env-prefixed rows should be restated in roster-file form. This is the same check-definition gap already on #1843. (2) cellctl comms tests fail wherever the temp dir path is long, because the unix socket path goes over the platform limit. No issue tracks this. (3) The Test Guard Stop Flags boundary flake reproduced under full-suite load (#2776).

**VERIFY: FAIL** — check-definition, 7/10 rows pass on merged main c690023a864b. Row 2 refuses with exit 6, not the pinned 2, and the refusal is line 2, after the config digest. That needs a brief amendment, carried on #1843. Row 5 refuses an unknown key where the brief expects a NOTICE, which waits on the needs-decision ruling #1261. Row 8 is could-not-check on the host environment (socket path limit, and the #2776 flake). Status stays implemented.

### Non-implementer verifier run — 2026-10-16 claude-opus-5-5 verifier at 1948dc087370 (FAIL 8/10: rows 2 and 5 fail as on 2026-10-14; row 8 passes)

Verification-Attestation: medici-finance/assay#3141 run=d1c6b99361e505dd8e41e7f8601b59d796579f2e7a649ab49e53fd457fd91d47 source=1948dc0873705930b202f0e747047df844010d1a brief=docs/streams/composability/brief-01-reactive-activation.md model=claude-opus-5-5 tier=any

Runner is not the implementer. Fresh full run: every row below was run in this pass, at merged main 1948dc087370 (detached, equal to the forge's main head at dispatch). The dispatcher's verifier check admitted the run before any row and again before return. `gate: model`; risk answers all `no`. Delivering change: #957 (merge 743195b970b6). Desk binaries and statusgen were built from this tree into a scratch directory outside the checkout. Every row used a throwaway HOME. The roster in it was synthetic (login `ada:2001`, repo example-org/tracker, synthetic reviewer slug), with no real deployment values. Since the 2026-10-14 run (c690023a864b), activation.go, rosterconfig.go, exitcodes.go, the desk-tools manifest and every component manifest are byte-identical. Neither #1843 nor #1261 has a recorded ruling yet.

**Grounding (derived from the brief and main before reading the diff or earlier Evidence).** Every desk verb declares itself write-class (for example deskboard main.go:158). Write-class tools read the roster only from the config-home roster file, never from the environment (rosterconfig.go readRawConfig, write branch). So the env-prefixed forms in rows 1-5 never reach the loader. Each of those rows was run literally and again in roster-file form. No shipped manifest lists `assay.roster.ext.repo-aliases` under `inject.required`: the-desk and worker-desk carry it as optional, and no manifest requires any extension key. Row 2 and row 4 therefore ran in a scratch clone of this commit whose desk-tools manifest adds that key to `inject.required`. Rows 6 and 7 mutated their own scratch clones, which were then discarded.

| # | Command | Result | Output | Date | Runner |
| --- | --- | --- | --- | --- | --- |
| 1 | roster file sets ASSAY_REPO_ALIASES to `not=a=valid=shape`; `deskboard --help`, then `deskreply --help`; literal env-prefixed form also run against a valid roster | pass exit=0 | deskboard 0, deskreply 0, literal form 0. Help printed with no `inactive` line; the full echo shows ASSAY_REPO_ALIASES empty, because the rejected value falls back to the unset default | 2026-10-16 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | scratch clone whose desk-tools manifest requires `assay.roster.ext.repo-aliases`; roster file holds the invalid alias; `deskreply --help`, then `deskboard --help`; literal env form also run | fail exit=6 | Both exit 6. Line 1 is the config digest (`assay-config: … configured=true …`). Line 2 is `could-not-check: assay/desk-tools inactive — assay.roster.ext.repo-aliases ASSAY_REPO_ALIASES: entry "not=a=valid=shape" has no ':' separating short label from product …`. The component and key are right. Three things miss the pinned Expect: exit 6, not 2; the refusal is line 2, not line 1; the reason is the parser's text, not the word `invalid`. The literal env form exits 0 with no `inactive` line, because the env value never reaches the loader | 2026-10-16 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | roster file with no bless login and empty ASSAY_TRUSTED_LOGINS; roster file with `ASSAY_TRUSTED_LOGINS=bob:notanid`; `deskboard --help`; literal env-prefixed form also run | pass exit=6 | Both roster-file forms exit 6 with `could-not-check: assay/desk-tools inactive — assay.roster.trust unset or malformed`. The malformed form also prints `REFUSED — ASSAY_TRUSTED_LOGINS: cannot parse entry "bob:notanid" …`. With the bless login still set and ASSAY_TRUSTED_LOGINS empty, the result is exit 0, because the bless login counts as trusted; that predates #957. The literal form exits 0 against a valid roster file and exits 6 when no roster file exists | 2026-10-16 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | row 2's scratch clone; roster file with trust unset and a well-formed alias, then trust unset and the invalid alias; `deskreply --help`; literal env form also run | pass exit=6 | Both roster-file runs exit 6 with `could-not-check: assay/desk-tools inactive — assay.roster.trust unset or malformed`, so trust outranks the extension key. Control: a valid roster plus a well-formed alias in the same clone exits 0. The literal form exits 0 against a valid roster file | 2026-10-16 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | roster file with `ASSAY_UNKNOWN_KEY=1` added to a valid roster; `deskboard --help`; literal env-prefixed form also run | fail exit=6 | Roster-file form: `assay-config: REFUSED — ASSAY_UNKNOWN_KEY: unknown key in the ASSAY_ namespace …`, then `could-not-check: assay/desk-tools inactive — assay.roster.trust unset or malformed`. No NOTICE line. The literal form exits 0, but no output line mentions ASSAY_UNKNOWN_KEY. The brief expects a NOTICE and no refusal; the ruling is still open on #1261 | 2026-10-16 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 6 | mutation in a scratch clone: parseConfig appends a `MUTANT abort` problem for every extension key recorded invalid. That turns the per-key result back into a whole-config abort; the per-key result lives in rosterconfig.go, not activation.go. Rebuild; row 1 roster-file form; deskkit suite; clone discarded | pass exit=6 | The mutant deskboard and deskreply both exit 6 with `REFUSED — MUTANT abort: repo-aliases: ASSAY_REPO_ALIASES: …`, then the trust-inactive refusal, so the old fleet-wide behaviour is back. The deskkit suite exits 1 with 9 failures. Eight come from the mutation, including TestRepoAliasMalformedDeactivatesOnlyItsDependents and TestVerbActivationRefusal_ExtensionKeyInvalid; the ninth, TestSharedReadLowerScope, is environmental (see row 7) | 2026-10-16 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 7 | mutation in a scratch clone: a malformed ASSAY_TRUSTED_LOGINS entry goes to a non-aborting extension accumulator, and the key joins the extension catalogue as `trusted-logins`; row 3 malformed form; deskkit suite; clone discarded | pass exit=0 | The mutant exits 0 with `configured=true` on the malformed-trust roster, which is the wrong result the row predicts. The deskkit suite exits 1, failing TestExtCatalogueAllowList and TestMalformedIDNeverDegradesToLoginOnlyTrust. A third failure, TestSharedReadLowerScope, also appeared in both mutation clones. It comes from the clone's local-path origin remote, not from either mutation: it passes on the unmutated tree once origin names the forge (row 8) | 2026-10-16 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 8 | `cd tools/desk && go test ./internal/deskkit/... ./cmd/...` | pass exit=0 | 69 `ok` packages, no FAIL. Run in a pristine clone of this commit whose origin remote names the forge as the checkout's does, with Homebrew bash first on PATH. A first attempt exited 1 because of how this runner set things up: the clone's origin was a local path, so forge resolution failed in deskkit, deskdispatch and scanloop tests, and macOS bash 3.2 sat first on PATH, which fails the cellctl bash-quote test. Those four packages, then the full command, pass once both are corrected | 2026-10-16 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 9 | `deskmanifest lint --root .` | pass exit=0 | `checked-clean: 26 manifest(s), every inject.required resolves in range, no cycles`; same result with no roster file | 2026-10-16 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 10 | valid roster file, in sequence: `deskboard --help`, `deskreply --help` and `deskversion --version` (assay/desk-tools), `statusgen --version` (assay/statusgen), the inject-resident-rules hook script (assay/hooks) | pass exit=0 | All five exit 0 and none prints an `inactive` line. Caveat: every desk binary maps to the single `assay.desk.verbs` provider, and the statusgen and hook entries do not pass through the desk activation check. So only one component here exercises activation, and "three components" cannot hold as written | 2026-10-16 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Vacuity: the env-prefixed forms of rows 1, 3, 4 and 5 do not exercise the loader for write-class verbs. Rows 9 and 10 would also pass without this brief's work. Row 2 can only run against a synthetic manifest.

RISK-VALUE. The fail-safe trigger fires because #957's diff touches the writeguard command entrypoint. Enumeration covered #957's diff over the deskkit loader, activation.go, the per-verb entrypoint bindings and the desk-tools manifest, read at this commit. It found: the refusal exit binding; the extension catalogue (9 entries); the `assay.roster.trust`, `assay.roster.ext.` and `assay.desk.verbs` key literals; the Configured() predicate; and the refusal format string. All of them can be undone by an edit and a redeploy.
- RISK-VALUE: DERIVED — extKeyNames @ tools/desk/internal/deskkit/rosterconfig.go:953-967. It holds the brief's eight extension keys plus os-user-login-map, which only labels a forensic audit row and authorises nothing. It holds none of the brief's five trust keys, and ASSAY_RUN_CREDENTIALS is absent. That is right because a trust key in this map would stop failing closed: mutation 7 shows exactly that, and TestExtCatalogueAllowList catches it.
- RISK-VALUE: DERIVED — the `"assay.roster.trust"` gate @ tools/desk/internal/deskkit/activation.go:247 resolves only when Configured() @ tools/desk/internal/deskkit/rosterconfig.go:1022 holds: `len(c.Problems) == 0 && c.Bless.Login != "" && len(c.Logins) > 0`. That is the fail-closed definition that predates #957, so the trust surface is no weaker than before (rows 3 and 4, mutation 7).
- RISK-VALUE: DERIVED — ExitUnverifiable = 6 @ tools/desk/internal/deskkit/exitcodes.go:55, bound per verb (for example tools/desk/cmd/deskboard/main.go:167). The repo's own derivation at exitcodes.go:16-35 reserves 2 for shell usage errors and starts refusal classes at 3, with 6 meaning a precondition could not be positively verified. A could-not-check refusal belongs to that class, so 6 is the right code and the brief's pinned 2 is the stale value. Amending the brief is the owner's call, carried on #1843.
- RISK-VALUE: DERIVED — the `assay.roster.ext.` prefix @ activation.go:253 and `assay.desk.verbs` @ activation.go:304 match the catalogue in components/KEYS.md and the providers declared in the components/roster and tools/desk manifests. The refusal format @ activation.go:353 is display text and ranks last.

Findings: (1) Rows 2 and 4 need a placeholder filled and a synthetic manifest, and rows 6, 7 and 10 are prose. The env-prefixed rows should be restated in roster-file form. This is the same check-definition gap already carried on #1843. (2) Some row 8 tests depend on the host. They need the checkout's origin remote to name a known forge, and the cellctl bash-quote test needs a modern bash first on PATH. A clone with a local-path origin, or macOS's stock bash 3.2, fails them for reasons outside the code under test.

**VERIFY: FAIL** — check-definition, 8/10 rows pass on merged main 1948dc087370. Row 2 refuses with exit 6, not the pinned 2, and the refusal is line 2, after the config digest. That needs a brief amendment, carried on #1843. Row 5 refuses an unknown key where the brief expects a NOTICE, which waits on the needs-decision ruling #1261. Status stays implemented.

## Review

gate: model — pending.
