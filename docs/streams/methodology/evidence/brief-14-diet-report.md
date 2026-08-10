# Brief-14 diet report — removed/moved item mapping (zero rule loss)

Implementer: Fable session, 2026-07-10. Baseline at implementation: **4919 words**
(grew from the brief's 2026-07-08 baseline of ~3800 during the 07-09/07-10 rule influx).
Result: **2847 words** (−42% vs actual baseline, −25% vs brief baseline; Verify cap ≤2850).
canton-auth-doctor description: 239 → **119 words** (cap ≤120). Also dieted: ship-daml
139→119, canton-query 129→118, debug-agent 122→108.

Every deleted or moved line maps below to its new home or compressed replacement.
**No rule was deleted.** Categories: MOVED (content now lives elsewhere, CLAUDE.md keeps
rule + pointer), DEDUPED (already stated in the named home; duplicate removed),
COMPRESSED (same rule, fewer words — rationale/narrative cut, incident reduced to a
pointer), DROPPED-NARRATIVE (no rule content; explanation or history recoverable from
the named source).

## MOVED

| Item | New home |
|---|---|
| Frontend hooks table (10 rows) | `README.md` § Frontend Hooks (new section, verbatim) |
| POST `/v2/state/active-contracts` curl recipe | `../oit/docs/debugging-guide.md` § Raw /v2 sharp edges (new) |
| `GET /v2/parties` broken (#26) + truncated-fingerprint warning | `../oit/docs/debugging-guide.md` § Raw /v2 sharp edges; one-line pointer kept in CLAUDE.md debug RULE |
| Admin pod manifest line (`../oit/k8s/dev/app/medici-admin-debug.yaml` + tool list) | `../oit/docs/debugging-guide.md` § Raw /v2 sharp edges |

## DEDUPED (single residence + cross-reference)

| Item | Where it already lives |
|---|---|
| canton-auth-doctor description NOTE (auth-setup Jobs/claim-mappers deleted; reconciler owns provisioning) | skill body § "Before you edit" states the same model; description now trigger-only |
| "statusgen CI validates the PR on open" (worktree recipe) | Status tracking section: `--lint # PR CI` |
| "PRs … blocked from touching STATUS.md" | Covered by the resident single-writer RULE + "NEVER commit it on a branch" |
| Auth rule 4 "god token green proves nothing about RS256" | Same rule's "masks every RS256 failure" + "never diagnose with it alone" |
| Tiering effort-key restated in two places (review-gate + completion protocol) | Stated once in Next-pick paragraph |

## COMPRESSED (rule kept resident; narrative/incident → pointer)

| Item | Compressed replacement / pointer |
|---|---|
| Template-ID content-hash incident ("broke every frontend Split", upgrade-resolution mechanics) | "A pinned content hash 404s … (issue #18)" |
| Worker-forged DESK-READY marker story (PR #125) | "tamper-evident … (methodology/brief-17)" |
| Orphaned post-merge push story (#106, 2ab453d, rescued as #108) | "(incident: #106→#108)" |
| Secret-rotation "confirmed wrong four ways" enumeration | "cut unwired (`../oit/docs/streams/reconciler-spinout/brief-17-wire-or-cut-dead-machinery.md`)" |
| Auth rule 5 `${!ADDITIONAL_CONFIG@}` mechanics + wrong-name example (`…_AUTH_JWT_JWKS`, A<D) | "a name sorting before D gets silently wiped"; incident doc pointer retained |
| Auth rule 4 `sub=ledger-api-user` / "Splice disableAuth artifact" / "real participant has no such validator" detail | Rule + smoke-test mandate kept; detail in `../oit/docs/agent-first-architecture.md` § auth rules + canton-auth-doctor body |
| Singleton price feed "so P/N settle against the same price" rationale | Rule (Prune same-tx, one active per ticker, delay < interval) kept; rationale in README/AUDIT-2 |
| `--check` advisory paragraph (~70 words) | One-line comment on the command itself |
| CircuitBreaker trading-choice enumeration (`Execute`, recombines, `SettleP/N`, `Deposit`, `Exercise`) | "every trading choice"; enumeration in `../oit/docs/contracts-reference.md` |
| Governance heading "(propose/accept/execute multi-sig, role-based admin)" | "concept: README → Governance" |
| Reconciler-enforced list brief numbers (Brief 07/09/11/15/18) + delegations-bug anecdote | Rules kept; history in `../oit/docs/agent-first-architecture.md` + issue #23 pointer kept |
| Phase 3 detail bullets (agent-source paths, k8s manifests, fleet.ts) | 2-line summary; full paths in `../oit/docs/archive/agent-first-phase3/README.md` (pointer kept) |
| Retired-bot destinations ("→ StrategyAgent Phase 3", "→ backtest Phase 5") | "Retired (Brief 8)" retained; destinations are history |
| yq/jq worked examples (realm parse-check target, packageIds jq) | Tool rule + parse-check form kept |
| Namespace-table prose ("agent-fleet Postgres (CNPG Cluster)" etc.) | Table kept, descriptors tightened; "Untouched by app-layer changes" RETAINED |
| Reflected-secrets failure list ("Ledger Service, oracle-bot, and frontend fail") | "the app layer fails to start" |
| Branch-protection-OFF flip semantics | Kept in compressed form (step 3) — initially cut, restored on loss-check |
| Dev-env manifest/Kustomization line (`../oit/k8s/dev/canton.yaml` → namespace) | "FluxCD-deployed from `k8s/dev/canton/`"; Flux wiring in `k8s/dev/` tree itself |
| Related-docs descriptors (paper-to-code audit, phase-3 archive row, sharp-edges list) | Trimmed; targets carry their own descriptions |
| mint-reviewer-token full path | "`mint-reviewer-token.go` (pr-review-desk skill)" |

## DROPPED-NARRATIVE (no rule content)

- "Canton (Digital Asset's privacy-preserving blockchain)" descriptor; "This form is
  upgrade-transparent — Canton 3.5 resolves it to whatever DAR version…" mechanism prose.
- "Developed against the deployed int environment by default" sentence (heading says it).
- "Full plan: docs/archive/agent-first-phase1-plan.md" pointer (agents section) — archive
  is indexed from `docs/archive/`; superseded by the phase-3 archive pointer kept inline.
- Step-1 "(drafts included)" (drafts are open PRs); step-3 "posting a wrap-up comment that
  lists any open issues still left, at the bottom" → "with a wrap-up comment".
- "medici-finance/org-slides" org qualifier (path `../decks` kept); "— never short names"
  (redundant with "FQDN only"); assorted connective prose.

## ADDED (net-new, required by the brief / #221)

- **Placement rule** section (Task step 1) — what earns residence where; new-learning
  default is NOT CLAUDE.md; rules stay resident, never pointers-only.
- **Out-of-repo files rule** (issue #221 interim remedy): declare `out-of-repo files:` in
  Context; declaration = claim; max one in flight; apply live edits last; commit in the
  `~/.claude` stopgap repo.

## Skill descriptions (Task step 3)

All four edits cut process/workflow summary only; every trigger phrase retained:

- **canton-auth-doctor** 239→119: cut the reconciler-model NOTE (deduped to body),
  "multi-hour debugging loops"/"bitten us before" rationale, "see auth rule 5" aside.
- **ship-daml** 139→119: deploy-rules list tightened; prepare-and-stop contract kept.
- **canton-query** 129→118: sharp-edges enumeration cut (body's whole subject).
- **debug-agent** 122→108: run-convention/crash-cause parentheticals cut (body §§).
