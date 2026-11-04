---
brief: assay:assay:forge-neutral:55
title: Operator provisioning of known read-only refresh profiles
why: "The durable read owner deliberately holds when policy or usage is absent. A bounded operator path must install explicitly known profiles and accounting before read-only serving can use it, without teaching runtime startup to invent allowances or reset unknown work."
wave: 7
depends: ["forge-neutral/54"]
unblocks: ["forge-neutral/56"]
split-from: forge-neutral/40
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Review the demonstrated separation between operator profile initialization and restricted runtime SQL access. Accepting this library and synthetic role proof grants no operational role, credential, policy value or activation."
decision-trigger: spec
issues: []
schema: brief-v2
version: 2
authored: 2026-11-04 by statusgen newbrief
outcome: none
design: DR-forge-library
exec-tier: strong
exec-tier-why: "Known-accounting provenance, replay, cross-process initialization and actual SQL privileges must remain correct when runtime code bypasses the operator path."
domain: complicated
sources:
- docs/streams/forge-neutral/brief-40-authenticated-forge-serving-and-role-executor-custody.md
- docs/streams/forge-neutral/brief-54-canonical-postgresql-refresh-gate-and-admitted-read-through-integration.md
- spec/coordination-store-v1.md
- docs/proposals/non-ci-architecture/14-storage-forge-graph.md
- docs/proposals/non-ci-architecture/22-forge-service-and-source-queries.md
- docs/library-first.md
- docs/go-engineering.md
- "Inspected foundation36 1ddfb16e68ebb7be54d4dc5a606a07e74e2a7259, Store52 e7b084ba2f10e90feee8470ebab8c52942f05233 and Gate54 d114102a6e4c7de0d78c235dcbca2801676c4a4c; feature availability is not mainline delivery or acceptance."
- "Implementation genuinely consumes qualified local Gate54 30d838267e055397a35b07b0fe264a16302f8e41, containing published native53 622bec858f5ba7c7e9fc3ab91ff780e7d368e7d1 and corrected Store52 bfea138c74ae784ecc034f273076bd5866f4dddd. Gate54 publication and acceptance remain separately gated."
consumers:
- "workflow/store/refresh_provision.go: fixed-here (operator-only initialization and immutable replay)"
- "workflow/store/refresh_role_profile.go: fixed-here (declarative operator and restricted-runtime SQL profile)"
- "workflow/store/migrations/00004_refresh_initialization.sql: fixed-here (same Goose ledger, immutable provenance and bounded terminal eviction)"
- "workflow/internal/refreshcodec/codec.go: fixed-here (reuse canonical profile encoding)"
- "workflow/internal/pgtest/fixture.go: fixed-here (owned ephemeral synthetic role support only)"
- "workflow/pgtestfixture/fixture.go: fixed-here (exact production-declared manifest on owned test roles)"
- "tools/desk/internal/deskkit/mirror_joined_test.go: fixed-here (actual operator-provisioned restricted-login read-through fixture)"
- "workflow/README.md: fixed-here (typed provisioning and role boundaries)"
- "spec/coordination-store-v1.md: fixed-here (same canonical schema owner)"
- "docs/contracts.md: fixed-here (same semantic owner and explicit trust-boundary row)"
- ".github/workflows/assay-workflow.yml: fixed-here (dedicated native joined proof requires nonempty DSN and exact named PASS)"
- "authenticated caller mapping and read-only handlers: follow-up forge-neutral/56 (qualified R3 producer remains required)"
- "retention, complete mirror acceptance and tool migration: out-of-scope (already owned by full37; this contribution delivers none of them)"
- "workflow claims, execution, outbox and reconstruction: out-of-scope (no dependency on their delivery)"
---

# Brief 55 — Operator provisioning of known read-only refresh profiles

## Context

files:
- `workflow/store/refresh_provision.go` (planned)
- `workflow/store/refresh_provision_test.go` (planned)
- `workflow/store/refresh_provision_process_test.go` (planned)
- `workflow/store/refresh_role_profile.go` (planned)
- `workflow/store/testdata/provision_mutations.py` (planned)
- `workflow/store/migrations/` (planned)
- `workflow/store/refresh.go` (planned)
- `workflow/store/refresh_schema_test.go` (planned)
- `workflow/internal/refreshcodec/` (planned)
- `workflow/internal/pgtest/` (planned)
- `workflow/pgtestfixture/` (planned)
- `workflow/README.md` (planned)
- `spec/coordination-store-v1.md` (planned)
- `docs/contracts.md`
- `tools/desk/internal/deskkit/mirror_joined_test.go` (planned)
- `tools/desk/internal/deskkit/citrigger_test.go`
- `.github/workflows/assay-workflow.yml` (planned)
- `changelog/mirror-refresh-provisioning.md` (planned)

facts:
- The actual /54 constructor `DB.Refresh` takes its existing MirrorStore, explicit RefreshConfig and CallerAuthority. It creates no profile or usage record. Every operation holds on missing, unreadable, mismatched or unknown admission/source/caller accounting.
- RefreshConfig binds SourceID, AdmissionID, access/configuration/policy epochs, refresh Limits, NativeLimits and PollInterval. Values are supplied by trusted composition. Shared profiles require kernel FailureBackoff zero; the Gate alone owns shared retry eligibility.
- Native budget usage counts charged/reserved attempt slots, not scan calls, measured HTTP entries or provider quota points. A known zero HTTP result can still retain a charged slot; operator baselines must preserve those distinct units.
- Source, admission and caller usage already share R2 PostgreSQL, the private pool, one Goose manifest and canonical refreshcodec profile bytes. Existing runtime window rollover is not permission to initialize or reconcile unknown accounting.
- The test-only SeedRefresh requires a testing capability and owned database marker. It is not an operator API and must not become runtime bootstrap.
- Typed service Scope and profile identity strings do not authenticate a caller or grant repository access. This contribution is independent of selecting a qualified caller issuer or R3 control profile.

layering: extend the existing R2 owner. An operator supplies an explicitly authorized database connection and known inputs; the restricted runtime connection consumes initialized state. No default endpoint, second store, quota owner, issuer, daemon or automatic initializer.

design-fit:
  owner: canonical R2 workflow/store
  contract: S-coordination-store; same profile codec, pool and migration ledger
  retires: []
  weight: "No new CLI verb, flag, backend, driver or ledger; one operator-only typed path and declarative role profile. Record actual implementation counts."
  why-add: "Runtime cannot infer a known baseline. One explicit operator path prevents each serving consumer from promoting the test seed or inventing its own initializer."

single-point-of-failure: confusing a missing accounting row with known zero would create new allowance after restart or uncertain work. Independent SQL privileges, immutable initialization provenance and actual runtime-bypass controls must hold even if an upper operator check is removed.

## Read first and pickup

Read actual /36, /52 and /54 source, the canonical schema, native attempt accounting, library-first and Go engineering. Genuinely consume the qualified source and record immutable revisions before coding. Recheck migration numbering and current claims; append to the same manifest. Missing canonical source is NEEDS_CONTEXT, never permission to copy SeedRefresh or create a second initializer. No full40,41 or workflow-execution prerequisite is introduced. A supported canonical
pickup may genuinely consume qualified immutable54 feature source before its mainline
merge or acceptance; it must contain the actual current migrations, codec, constructor
and guards, not an authoring-only port. Core R2 provisioning can then proceed locally.
Final joined delivery consumes the completed qualified native/SQL owners and retains all
publication, independent verification and activation gates.

The implementation reuses the canonical private DB pool, refreshcodec and Goose
ledger. Its additive initialization migration also guards the runtime's existing
eligible-terminal fill deletion below direct SQL. The role manifest is rendered
only; actual grants and ephemeral LOGIN credentials are confined to the marked
disposable fixture during qualification. The joined native fixture uses that
restricted login, the operator API and the existing admitted SDK/mapper/coordinator
path; its positive path uses no SeedRefresh initialization. The committed bounded
production controls cover known inputs, replay, atomicity, two-process source
fencing, declared runtime grants, effective role escapes, owned cleanup and the
dedicated joined CI proof. Author-side checks do not supply independent Evidence.

## Human decision

Accept or hold the demonstrated operator-only library and restricted-runtime SQL boundary after its implementation, real-container role proof and independent review are concrete. The decision concerns explicit known inputs, replay and containment; it does not select production identities, limits or credentials.

Options:
1. Accept the bounded contribution after its named proof, retaining separate operator provisioning, authenticated serving, retention and activation acts.
2. Hold for a named accounting, replay or SQL privilege correction.

Default if no answer: none — blocks until answered

This gates acceptance. Offline source and owned disposable-container qualification may proceed. Do not ask for acceptance before the implementation and evidence packet are reviewable. No database, role or credential is provisioned operationally by this brief.

## Ground rules

- Owned branch and draft PR only; no main push/merge, release, deployment, workflow dispatch or live infrastructure contact. KUBECONFIG=/dev/null; explicit verified disposable Docker PostgreSQL only.
- No new quota, lease, identity, privilege or default threshold is inferred. Inputs come from the operator's already authorized profile; strings and an API call do not supply that authorization.
- Never convert unknown to known, reset usage or remove holds through initialization, replay, epoch change, restart or source/admission/caller enrollment.
- No production role creation, GRANT execution or role upgrade from the library, runtime startup, Open, Mirror, Refresh or query path. Synthetic role creation is confined to the owned fixture and its exact cleanup capability.
- Keep all parent40 Tasks and seven Verify rows, full /37's 23 rows and GE19's 13 rows intact. This contribution accepts none of those parents.

## Task

1. Add one narrow typed operator provisioning API under workflow/store. Its finite, bounded request names exact existing source/admission/caller identities, current configuration/epochs, canonical profile values and explicitly known initial accounting/provenance. Reject missing, unknown, contradictory, out-of-window or malformed inputs before writes. Use the existing charged fill/attempt accounting units; never reconstruct allowance from client/scan totals or infer that zero measured HTTP means zero charged slots. Do not generate identity or policy values, accept request text as authority, discover credentials or expose arbitrary SQL/transaction callbacks.
2. Atomically create only the absent records justified by those known inputs. An existing source domain is shared, not a fresh allowance for a new admission or caller. Preserve every existing counter, active debit, unresolved reservation, retry floor and unknown hold. Never reconcile existing unknown records here. Bind an immutable initialization receipt to the canonical supplied profile and baseline so exact retries remain idempotent after legitimate runtime usage advances. A conflicting replay refuses; partial failure leaves no subset initialized. An uncertain acknowledgment permits only readback/exact idempotent reconciliation, never a guessed reset.
3. Reuse the canonical private pool, refreshcodec and next numbered Goose migration. Persist the minimal provenance needed to distinguish an exact initialization replay from a new baseline; no second migration ledger, execution journal or recovery subsystem. Use canonical database time for window checks and preserve independent SQL guards. A current profile remains explicit and requires FailureBackoff zero.
4. Define one canonical declarative SQL role/column privilege profile for operator initialization versus runtime consumption, with caller-supplied role identifiers and no production defaults. It may render a bounded grant manifest but must execute no grants or create roles in production. Give runtime only the existing Gate/Store operations it needs: it cannot insert or known-upgrade profiles/accounting, alter initialization provenance or perform administrative DDL, TRUNCATE protected tables, delete/reset accounting or obtain those privileges through ownership, inherited membership or SECURITY DEFINER paths. Exercise the actual declared profile using unique synthetic roles in the owned container; do not copy a different permissive fixture recipe. Restrict SECURITY DEFINER or equivalent escape paths; possession of the runtime connection cannot invoke operator initialization successfully.
5. Extend the single owned fixture capability to support that role proof, if needed. Require testing context plus the actual owned database/role marker before setup and cleanup. Affect only identities/resources it created; uncertain ownership is retained and reported. Do not promote SeedRefresh or export an arbitrary privileged pool/initializer. Test healthy runtime Gate/Store use and direct SQL/operator-API bypass refusal with the restricted role.
6. Run an actual provisioned source → mapper → coordinator → canonical SQL fixture inside deskkit. Its positive path uses the production operator provisioning API, then the restricted runtime role, not SeedRefresh. Reuse the existing admitted native implementation and joined fixture; no new provider walker or client fallback. Missing/uninitialized/unknown profiles still cause zero provider calls. This proves synthetic composition, not authenticated serving or real policy activation.
7. Prove exact replay, concurrent initialization by two independent processes, rollback and restart/readback; retain unknown accounting and provider retry floors. Plant production-path faults for known-state checks, conflicting replay, atomic initialization and the actual declared SQL privilege profile. Compile/setup-only failures do not count. Commit a bounded rerunnable mutation script that restores exact bytes and refuses unowned worktrees/fixtures.
8. Update the schema/semantic owner, README and changelog. Extend actual CI readers and the dedicated PostgreSQL workflow so the new role/provisioned-join proof runs with a nonempty explicit DSN and exact named PASS. Ordinary no-DSN desk skips are explicitly unqualified. Preserve existing foundation/Store/Gate checks and every human publication/acceptance boundary.

## Interface contract

The operator API receives an already authorized explicit R2 connection and typed known input; it returns a bounded immutable initialization receipt or a sanitized typed refusal. It neither creates authorization nor repairs unknown usage. Runtime APIs consume the same records through the existing restricted connection and remain unable to initialize them. The role manifest is configuration for an explicit later operator act, not an installer. No wire endpoint, caller credential, provider operation, claim, outbox or activation is exposed.

## Verify

All named tests below are prospective deliverables and require the explicit owned disposable container. A skipped or absent qualifying test is failure. Independent post-merge verification binds actual delivery/base revisions; authoring supplies no Evidence.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshProvision$' ./store > "$o" && grep -F -- '--- PASS: TestPGRefreshProvision ' "$o")` | Named PASS; explicit known profiles/accounting initialize atomically, canonical bytes match and the restricted runtime Gate consumes them. Missing/unknown/invalid inputs refuse; removing the production known-state guard fails. |
| 2 | check +flow +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshProvisionReplay$' ./store > "$o" && grep -F -- '--- PASS: TestPGRefreshProvisionReplay ' "$o")` | Named PASS; exact replay survives legitimate usage advancement without resetting any counter/floor/hold; conflicting baseline, unknown existing usage and epoch-based resets refuse. Positive charged slots with measured zero HTTP remain charged. Removing immutable replay binding fails. |
| 3 | check +flow +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshProvisionProcesses$' ./store > "$o" && grep -F -- '--- PASS: TestPGRefreshProvisionProcesses ' "$o")` | Named PASS; two independent processes initialize once through deterministic barriers. A failure after staging leaves the old complete state; reopen and exact retry retain one receipt and all shared source accounting. Removing atomicity/fencing fails at runtime. |
| 4 | check +flow +mutation +neighbour | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshRuntimePrivileges$' ./store > "$o" && grep -F -- '--- PASS: TestPGRefreshRuntimePrivileges ' "$o")` | Named PASS against actual declared role grants. Runtime Gate/Store works; direct SQL and operator API cannot initialize/known-upgrade profiles, alter provenance, delete/reset accounting, TRUNCATE protected tables or perform admin DDL, including effective ownership/membership paths. The operator fixture can initialize only its owned test resources. Widening the actual production role profile makes a negative witness fail. |
| 5 | check +dereference +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshProvisionSchema$' ./store > "$o" && grep -F -- '--- PASS: TestPGRefreshProvisionSchema ' "$o")` | Named PASS; actual catalog and implemented schema/privilege contract agree in both directions. Missing provenance, widened runtime privilege and unexpected bootstrap behavior fail. Unseeded/unreadable/unknown profiles independently hold without provider allowance. |
| 6 | check +flow +mutation | `(cd tools/desk && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -race -v -run '^TestMirrorProvisionedReadThrough$' ./internal/deskkit > "$o" && grep -F -- '--- PASS: TestMirrorProvisionedReadThrough ' "$o")` | Named PASS; actual operator API then restricted runtime role drives admitted native SDK → mapper → coordinator → SQL → guarded query. Unknown/missing policy has zero provider calls; bypassing the role/operator boundary fails. No SeedRefresh positive or client fallback supplies evidence. |
| 7 | check +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && python3 store/testdata/provision_mutations.py > "$o" && cat "$o" && grep -F -- 'PASS: provision production controls killed; exact bytes restored' "$o")` | In an owned clean linked feature worktree, every named known-state/replay/atomicity/role-profile fault compiles and causes the independently specified runtime negative. Setup-only failures are invalid; exact original bytes are restored. |
| 8 | check:ci +flow | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestWorkflowRefreshProvisionCI$' ./internal/deskkit > "$o" && grep -F -- '--- PASS: TestWorkflowRefreshProvisionCI ' "$o")` | Named PASS; actual dedicated workflow triggers cover every dependency read, use the admitted Docker-capable runner, require nonempty DSN and exact provisioned-join PASS. Ordinary missing-DSN skips cannot qualify this row. |
| 9 | check | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && GOWORK=off go vet ./... && GOWORK=off go test -count=1 -race ./...)` | Exit 0 on the actual disposable container; all foundation/Store/Gate/provisioning and owned-role cleanup checks pass without skipped integration credit. |
| 10 | check +dereference | `GOWORK=off go -C statusgen run . --root .. --consumers --brief forge-neutral/55 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | Before merge, actual delivery edits corroborate consumers and identify unchecked uses. Before post-merge verification, use independently established immutable delivery/base bounds. Source/role configuration, authorization, deployment and parent acceptance remain distinct. Any forge lookup uses the verifier's explicitly checked managed identity. |

## Evidence
<!-- A NON-implementer supplies independent evidence; no prospective row is passed by authoring. -->

## Review

Gate: human. Independent verification and acceptance follow the concrete implementation and role proof; operator provisioning, credential selection, serving and activation remain separate acts.
