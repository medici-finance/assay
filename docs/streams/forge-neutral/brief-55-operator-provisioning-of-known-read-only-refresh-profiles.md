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
decision-issue: 5424
ruling: "https://github.com/medici-finance/assay/issues/5424#issuecomment-6672660827"
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Review the demonstrated separation between operator profile initialization and restricted runtime SQL access. Accepting this library and synthetic role proof grants no operational role, credential, policy value or activation."
decision-trigger: spec
issues: []
schema: brief-v2
version: 4
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
- "Qualified staging pickup consumes canonical Gate54 36e150d39ea494742c256b56df9fe6706e313aaa, the helper-qualified descendant of 7e141264a28e928d9a74eb6d0bbcd05dfb7efc3d with identical runtime. Its explicit trusted refresh.Clock binds source/coordinator data evaluation only; database time independently owns leases, accounting windows and retry floors. Publication and acceptance remain separately held."
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
- The actual /54 constructor `DB.Refresh` takes its existing MirrorStore, explicit RefreshConfig, CallerAuthority and trusted refresh.Clock shared with the source/coordinator. That data clock does not enter profile encoding or operator provisioning; database time independently owns leases, accounting windows and retry floors. It creates no profile or usage record. Every operation holds on missing, unreadable, mismatched or unknown admission/source/caller accounting.
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
| 7 | check +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && python3 store/testdata/provision_mutations.py --delivery ef8316b8814dc2625ba701089909ddd8a399e96a --base 33b78e4273a2c68395a7cff18d8d831bca9b882f > "$o" && cat "$o" && grep -F -- 'PASS: provision production controls killed; exact bytes restored' "$o")` | In an owned clean linked feature worktree, every named known-state/replay/atomicity/role-profile fault compiles and causes the independently specified runtime negative. Setup-only failures are invalid; exact original bytes are restored. |
| 8 | check:ci +flow | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestWorkflowRefreshProvisionCI$' ./internal/deskkit > "$o" && grep -F -- '--- PASS: TestWorkflowRefreshProvisionCI ' "$o")` | Named PASS; actual dedicated workflow triggers cover every dependency read, use the admitted Docker-capable runner, require nonempty DSN and exact provisioned-join PASS. Ordinary missing-DSN skips cannot qualify this row. |
| 9 | check | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && GOWORK=off go vet ./... && GOWORK=off go test -count=1 -race ./...)` | Exit 0 on the actual disposable container; all foundation/Store/Gate/provisioning and owned-role cleanup checks pass without skipped integration credit. |
| 10 | check +dereference | `GOWORK=off go -C statusgen run . --root .. --consumers --brief forge-neutral/55 --delivery ef8316b8814dc2625ba701089909ddd8a399e96a --base 33b78e4273a2c68395a7cff18d8d831bca9b882f` | Before merge, actual delivery edits corroborate consumers and identify unchecked uses. Before post-merge verification, use independently established immutable delivery/base bounds. Source/role configuration, authorization, deployment and parent acceptance remain distinct. Any forge lookup uses the verifier's explicitly checked managed identity. |

## Evidence
<!-- A NON-implementer supplies independent evidence; no prospective row is passed by authoring. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshProvision$' ./store > "$o" && grep -F -- '--- PASS: TestPGRefreshProvision ' "$o")` | pass exit=0 | sha256:41226fd0eebf | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshProvisionReplay$' ./store > "$o" && grep -F -- '--- PASS: TestPGRefreshProvisionReplay ' "$o")` | pass exit=0 | sha256:3232d9ce6fef | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshProvisionProcesses$' ./store > "$o" && grep -F -- '--- PASS: TestPGRefreshProvisionProcesses ' "$o")` | pass exit=0 | sha256:e2fb6faa9429 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshRuntimePrivileges$' ./store > "$o" && grep -F -- '--- PASS: TestPGRefreshRuntimePrivileges ' "$o")` | pass exit=0 | sha256:d51523a7f07a | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) |
| 5 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshProvisionSchema$' ./store > "$o" && grep -F -- '--- PASS: TestPGRefreshProvisionSchema ' "$o")` | pass exit=0 | sha256:76ee5e692d61 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) |
| 6 | `(cd tools/desk && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -race -v -run '^TestMirrorProvisionedReadThrough$' ./internal/deskkit > "$o" && grep -F -- '--- PASS: TestMirrorProvisionedReadThrough ' "$o")` | pass exit=0 | sha256:ef8fcfca4221 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) |
| 7 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && python3 store/testdata/provision_mutations.py > "$o" && cat "$o" && grep -F -- 'PASS: provision production controls killed; exact bytes restored' "$o")` | fail exit=1 | sha256:215d940efeb7 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) |
| 9 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && GOWORK=off go vet ./... && GOWORK=off go test -count=1 -race ./...)` | pass exit=0 | sha256:2187f55839c5 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) |
| 10 | `GOWORK=off go -C statusgen run . --root .. --consumers --brief forge-neutral/55 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | fail exit=1 | sha256:2156f8e0c6a0 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5694 run=7da8d79e34a929b54245d4e70579da87fb0d06572033138883a4e8fc6be60fa8 source=5142af8c66f4671d7cb3c575d5b65d5eadd6275b brief=docs/streams/forge-neutral/brief-55-operator-provisioning-of-known-read-only-refresh-profiles.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshProvision$' ./store > "$o" && grep -F -- '--- PASS: TestPGRefreshProvision ' "$o")` | pass exit=0 | sha256:eafaeb336a8f | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) | base=33b78e4273a2 red rc=1 · head=5142af8c66f4 |
| 2 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshProvisionReplay$' ./store > "$o" && grep -F -- '--- PASS: TestPGRefreshProvisionReplay ' "$o")` | pass exit=0 | sha256:0e7912bef16d | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) | base=33b78e4273a2 red rc=1 · head=5142af8c66f4 |
| 3 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshProvisionProcesses$' ./store > "$o" && grep -F -- '--- PASS: TestPGRefreshProvisionProcesses ' "$o")` | pass exit=0 | sha256:e2fb6faa9429 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) | base=33b78e4273a2 red rc=1 · head=5142af8c66f4 |
| 4 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshRuntimePrivileges$' ./store > "$o" && grep -F -- '--- PASS: TestPGRefreshRuntimePrivileges ' "$o")` | pass exit=0 | sha256:115a031c8c01 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) | base=33b78e4273a2 red rc=1 · head=5142af8c66f4 |
| 5 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshProvisionSchema$' ./store > "$o" && grep -F -- '--- PASS: TestPGRefreshProvisionSchema ' "$o")` | pass exit=0 | sha256:3b9b881f472a | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) | base=33b78e4273a2 red rc=1 · head=5142af8c66f4 |
| 6 | `(cd tools/desk && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -race -v -run '^TestMirrorProvisionedReadThrough$' ./internal/deskkit > "$o" && grep -F -- '--- PASS: TestMirrorProvisionedReadThrough ' "$o")` | pass exit=0 | sha256:d448e0ab2cc5 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) | base=33b78e4273a2 red rc=1 · head=5142af8c66f4 |
| 7 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && python3 store/testdata/provision_mutations.py > "$o" && cat "$o" && grep -F -- 'PASS: provision production controls killed; exact bytes restored' "$o")` | fail exit=1 | sha256:c7b93fed52ad | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) | base=33b78e4273a2 red rc=2 · head=5142af8c66f4 |
| 9 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && GOWORK=off go vet ./... && GOWORK=off go test -count=1 -race ./...)` | pass exit=0 | sha256:160def4526f5 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) | base=33b78e4273a2 green rc=0 · head=5142af8c66f4 |
| 10 | `GOWORK=off go -C statusgen run . --root .. --consumers --brief forge-neutral/55 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | pass exit=0 | sha256:66715d2b4f3e | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) | base=33b78e4273a2 red rc=1 · head=5142af8c66f4 |
Verification-Attestation: medici-finance/assay#5694 run=7da8d79e34a929b54245d4e70579da87fb0d06572033138883a4e8fc6be60fa8 source=5142af8c66f4671d7cb3c575d5b65d5eadd6275b brief=docs/streams/forge-neutral/brief-55-operator-provisioning-of-known-read-only-refresh-profiles.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 8 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestWorkflowRefreshProvisionCI$' ./internal/deskkit > "$o" && grep -F -- '--- PASS: TestWorkflowRefreshProvisionCI ' "$o")` | could-not-run exit=1 sandbox=container-netns — image module cache does not match go.sum (github.com/jackc/pgx/v5 v5.12.0 vs none; github.com/mattn/go-isatty v0.0.24 vs v0.0.20; github.com/pressly/goose/v3 v3.29.0 vs none): the row stopped on module resolution before its check ran. Re-run in the verify-runner image of a release whose tools/desk go.mod and go.sum match the verified tree, normally the first release that contains the verified commit | sha256:f4991c0f250d | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:a45db11d4bffc3f40f29c060e503274a3912aafbbb7aaded88ea20c181f1c3cf
Container-Image: sha256:576cb2e49407447769b79a32af370a23e1c108d62842b8f1772a809d1d3b7617
Verification-Attestation: medici-finance/assay#5694 run=7da8d79e34a929b54245d4e70579da87fb0d06572033138883a4e8fc6be60fa8 source=5142af8c66f4671d7cb3c575d5b65d5eadd6275b brief=docs/streams/forge-neutral/brief-55-operator-provisioning-of-known-read-only-refresh-profiles.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=7da8d79e34a929b54245d4e70579da87fb0d06572033138883a4e8fc6be60fa8 source=5142af8c66f4671d7cb3c575d5b65d5eadd6275b tree=sha256:a45db11d4bffc3f40f29c060e503274a3912aafbbb7aaded88ea20c181f1c3cf witness=sha256:92cfd06bd651aeedbc3f94821d1761830c31e38603ad709471e03fe5f9b517f3 opened=2026-11-07T22:27:29Z closed=2026-11-07T22:27:49Z before=receipt after=check-verifier

Verifier notes (non-implementer, claude-opus-5-5, tier strong, statusgen/desk-tools v1.0.77, darwin arm64, KUBECONFIG=/dev/null, no forge identity exported, normal HOME). Source 5142af8c66f4 on main; delivering change #5485, merge ef8316b8814d, whose first parent 33b78e4273a2 is the fail-first base. Database rows ran against a disposable local PostgreSQL container created for this run only; no live infrastructure was touched.

Attempts. (1) 22:27:26Z to 22:27:49Z: row 8 (check:ci) through the container-netns admission bracket, pinned verify-runner v1.0.75 image, linux/arm64 manifest named in the Container-Image line; the container was given `GOFLAGS=-mod=mod`, disclosed here, which did not change the outcome. Row 8 was not run to a verdict: the image module cache predates the tools/desk go.sum at this source (verifyrun's own reason is in the row-8 witness above), the class tracked by #4683. (2) 22:28:12Z to 22:29:13Z: host verifyrun of rows 1-7, 9, 10, superseded by attempt 3. (3) 22:30:19Z to 22:31:38Z: host verifyrun `--fail-first --base 33b78e4273a2` of rows 1-7, 9, 10, the credited witness (the table with the Base column). `verifyrun --check`: 8 pass, 1 fail (row 7), 1 could-not-run (row 8), exit 2. `--bracket-digest`: verdict=match on the row-8 bracket.

Fail-first at base 33b78e4273a2: rows 1-6 and 10 red at base and passing at head; row 7 red at base and failing at head; row 9 green at both (non-discriminating). Row 9 is the whole-module vet and race suite, a regression gate whose discriminating coverage is carried by rows 1-6, so its green base is expected and is not a finding against the delivery.

Row 7 not run to its claim in this home: `provision_mutations.py` requires an owned linked worktree on a `codex/` branch with a clean tree, and a post-merge verifier home is a detached checkout of main, so the script stops at its branch precondition before any fault is planted (exit 1). This is a check-definition gap in the row, not an observed failed control. Supplemental, no acceptance credit: the same script, unmodified at this source, run in a scratch linked worktree on a throwaway `codex/` branch at 5142af8c66f4 against the same disposable database, printed 19 `KILLED` lines (each compiled, each failing at an independent runtime assertion, including lower-eviction, provenance-immutability, immutable-replay, cross-process-source-fence and dedicated-native-ci) and `PASS: provision production controls killed; exact bytes restored`, exit 0, 22:36:21Z to 22:37:10Z.

Row 10: the row command resolves its base to `git merge-base refs/remotes/origin/main HEAD`, which after the merge is HEAD itself. Attempt 2 shows the honest result (the brief is not in the diff, exit 1). In attempt 3 the row exits 0 only because this verifier's own Evidence edit put the brief into the working-tree diff; that run judged 0 corroborated, 0 disproved, 14 unchecked, so its exit 0 carries no corroboration on its own. The Expect cell asks for independently established immutable delivery/base bounds after the merge. Supplemental, no acceptance credit: `statusgen --consumers --brief forge-neutral/55 --delivery ef8316b8814dc2625ba701089909ddd8a399e96a --base 33b78e4273a2c68395a7cff18d8d831bca9b882f` (local history only, no forge reads) reported 12 corroborated, 0 disproved, 2 unchecked (both out-of-scope entries, a reviewer judgement), exit 0, with its producer-delivery limit line stating that producer identity and completeness are not evaluated.

Risk-bearing values enumerated over the delivered diff and ranked: identifier length bound, digest size, timestamp resolution, source fills bound, runtime delete grant, runtime budget update grant; lower: advisory lock key width (first 8 bytes of sha256 of the source id, refresh_provision.go:136-137) and the 64 KiB manifest cap (refresh_role_profile.go:105, an operational limit). Irreversible: no.
RISK-VALUE: DERIVED — `len(s) <= 63` at workflow/store/refresh_role_profile.go:43 is PostgreSQL NAMEDATALEN 64 less the terminator; a longer role name would be truncated and could alias another role.
RISK-VALUE: DERIVED — `octet_length(identity_hash)=32` at workflow/store/migrations/00004_refresh_initialization.sql:4 is the sha256 digest size produced by `sha256.Sum256(r.identity)` at workflow/store/refresh_provision.go:161.
RISK-VALUE: DERIVED — `t.Nanosecond()%1000 == 0` at workflow/store/refresh_provision.go:62 matches timestamptz microsecond resolution, so an accepted instant replays exactly.
RISK-VALUE: DERIVED — the source fills bound `0` in `initialUsage(p.Source.Usage, 0, ...)` at workflow/store/refresh_provision.go:71: refresh_budgets declares no fills column (workflow/store/refresh_role_profile.go:33).
RISK-VALUE: DERIVED — runtime `delete: true` on refresh_fills at workflow/store/refresh_role_profile.go:36 is bounded below SQL by the refresh_eviction_guard trigger (workflow/store/migrations/00004_refresh_initialization.sql:16-28); its mutation was killed in the supplemental row-7 run.
RISK-VALUE: DERIVED — runtime update of `unknown_hold` on refresh_budgets at workflow/store/refresh_role_profile.go:33 cannot clear it: workflow/store/migrations/00003_refresh.sql:111 raises 'unknown budget reset'.

#### Correction — 2026-11-07 rework

- Blockers: rows 7 and 10 are check-definition defects in the Verify table, filed as #5698 (row 10 is the class #4560 records for other briefs). Row 7 cannot pass in any detached post-merge home, and row 10's merge-base base equals HEAD after the merge, so its exit 0 carries no corroboration. Row 8 stays an environment blocker under #4683 (the pinned verify-runner image's module cache does not match this source's tools/desk go.sum). The verdict stays BLOCKED: row 7 stopped at a setup precondition before its claim was exercised, so no acceptance obligation was executed and failed. The check-definition repair for rows 7 and 10 is worker or brief-author rework tracked in #5698; the row-8 re-run waits on #4683.
- Row 7's stopping point: the notes above say the script stops at its branch precondition. That is more specific than the record supports. `provision_mutations.py` checks a clean `git status --porcelain` before it checks for a `codex/` branch, and this verifier's Evidence edit was in the working tree during attempt 3, so the credited run stopped at one of those two preconditions (clean tree or `codex/` branch); only the output digest was recorded, not the refusal text.
- Credited table: the credited witness is attempt 3, the table with the Base column. The first table without a Base column that carries row 10 `fail exit=1` is attempt 2; it is superseded and uncredited.

**VERIFY: BLOCKED** — rows 7 and 10 are check-definition defects in the Verify table (#5698): row 7 stops at a setup precondition a detached post-merge home cannot meet, and row 10's merge-base base equals HEAD after the merge; row 8 (check:ci) not run to a verdict because the pinned verify-runner image's module cache does not match this source's tools/desk go.sum (#4683); rows 1-6 and 9 pass with rows 1-6 red at the pre-delivery base.

Acceptance-Changes: 7=Command,10=Command
Verification-Table: delivery=ef8316b8814dc2625ba701089909ddd8a399e96a base=ae3e2b12ded2c8202264a90df9d666d202842aa7 original=e5783147c8e96af62779601af8aa71f31667e567
Acceptance-Transition: amendment=85898e42b80ec4114f0a0621dc7166fefbf066e4 approval=https://github.com/medici-finance/assay/issues/5724#issuecomment-6715508823

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshProvision$' ./store > "$o" && grep -F -- '--- PASS: TestPGRefreshProvision ' "$o")` | pass exit=0 | sha256:447af6a7bf7a | 2026-11-08 | assay-verifier-app[bot] @ 010c13c16ff9b531242982a02bf86cc8e1f3ab5d (on-behalf-of human:ian) (forge-identity) | base=33b78e4273a2 red rc=1 · head=010c13c16ff9b531242982a02bf86cc8e1f3ab5d |
| 2 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshProvisionReplay$' ./store > "$o" && grep -F -- '--- PASS: TestPGRefreshProvisionReplay ' "$o")` | pass exit=0 | sha256:82a839d41d48 | 2026-11-08 | assay-verifier-app[bot] @ 010c13c16ff9b531242982a02bf86cc8e1f3ab5d (on-behalf-of human:ian) (forge-identity) | base=33b78e4273a2 red rc=1 · head=010c13c16ff9b531242982a02bf86cc8e1f3ab5d |
| 3 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshProvisionProcesses$' ./store > "$o" && grep -F -- '--- PASS: TestPGRefreshProvisionProcesses ' "$o")` | pass exit=0 | sha256:1523b82321ea | 2026-11-08 | assay-verifier-app[bot] @ 010c13c16ff9b531242982a02bf86cc8e1f3ab5d (on-behalf-of human:ian) (forge-identity) | base=33b78e4273a2 red rc=1 · head=010c13c16ff9b531242982a02bf86cc8e1f3ab5d |
| 4 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshRuntimePrivileges$' ./store > "$o" && grep -F -- '--- PASS: TestPGRefreshRuntimePrivileges ' "$o")` | pass exit=0 | sha256:22c87e65de60 | 2026-11-08 | assay-verifier-app[bot] @ 010c13c16ff9b531242982a02bf86cc8e1f3ab5d (on-behalf-of human:ian) (forge-identity) | base=33b78e4273a2 red rc=1 · head=010c13c16ff9b531242982a02bf86cc8e1f3ab5d |
| 5 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshProvisionSchema$' ./store > "$o" && grep -F -- '--- PASS: TestPGRefreshProvisionSchema ' "$o")` | pass exit=0 | sha256:1de408836efe | 2026-11-08 | assay-verifier-app[bot] @ 010c13c16ff9b531242982a02bf86cc8e1f3ab5d (on-behalf-of human:ian) (forge-identity) | base=33b78e4273a2 red rc=1 · head=010c13c16ff9b531242982a02bf86cc8e1f3ab5d |
| 6 | `(cd tools/desk && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -race -v -run '^TestMirrorProvisionedReadThrough$' ./internal/deskkit > "$o" && grep -F -- '--- PASS: TestMirrorProvisionedReadThrough ' "$o")` | pass exit=0 | sha256:c8e2b258a223 | 2026-11-08 | assay-verifier-app[bot] @ 010c13c16ff9b531242982a02bf86cc8e1f3ab5d (on-behalf-of human:ian) (forge-identity) | base=33b78e4273a2 red rc=1 · head=010c13c16ff9b531242982a02bf86cc8e1f3ab5d |
| 7 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && python3 store/testdata/provision_mutations.py --delivery ef8316b8814dc2625ba701089909ddd8a399e96a --base 33b78e4273a2c68395a7cff18d8d831bca9b882f > "$o" && cat "$o" && grep -F -- 'PASS: provision production controls killed; exact bytes restored' "$o")` | pass exit=0 | sha256:a0bed627f9c4 | 2026-11-08 | assay-verifier-app[bot] @ 010c13c16ff9b531242982a02bf86cc8e1f3ab5d (on-behalf-of human:ian) (forge-identity) | base=33b78e4273a2 red rc=2 · head=010c13c16ff9b531242982a02bf86cc8e1f3ab5d |
| 9 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && GOWORK=off go vet ./... && GOWORK=off go test -count=1 -race ./...)` | pass exit=0 | sha256:195da1377d07 | 2026-11-08 | assay-verifier-app[bot] @ 010c13c16ff9b531242982a02bf86cc8e1f3ab5d (on-behalf-of human:ian) (forge-identity) | base=33b78e4273a2 green rc=0 · head=010c13c16ff9b531242982a02bf86cc8e1f3ab5d |
| 10 | `GOWORK=off go -C statusgen run . --root .. --consumers --brief forge-neutral/55 --delivery ef8316b8814dc2625ba701089909ddd8a399e96a --base 33b78e4273a2c68395a7cff18d8d831bca9b882f` | pass exit=0 | sha256:392829b413c0 | 2026-11-08 | assay-verifier-app[bot] @ 010c13c16ff9b531242982a02bf86cc8e1f3ab5d (on-behalf-of human:ian) (forge-identity) | base=33b78e4273a2 red rc=1 · head=010c13c16ff9b531242982a02bf86cc8e1f3ab5d |
Verification-Attestation: medici-finance/assay#5739 run=0eb01102d5d5baed8975f122f7fceae90411908b11fce8794453a3b35345fe87 source=010c13c16ff9b531242982a02bf86cc8e1f3ab5d brief=docs/streams/forge-neutral/brief-55-operator-provisioning-of-known-read-only-refresh-profiles.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 8 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestWorkflowRefreshProvisionCI$' ./internal/deskkit > "$o" && grep -F -- '--- PASS: TestWorkflowRefreshProvisionCI ' "$o")` | could-not-run exit=1 sandbox=container-netns — image module cache does not match go.sum (github.com/jackc/pgx/v5 v5.12.0 vs none; github.com/mattn/go-isatty v0.0.24 vs v0.0.20; github.com/pressly/goose/v3 v3.29.0 vs none): the row stopped on module resolution before its check ran. Re-run in the verify-runner image of a release whose tools/desk go.mod and go.sum match the verified tree, normally the first release that contains the verified commit | sha256:2b5e70b31771 | 2026-11-08 | assay-verifier-app[bot] @ 010c13c16ff9 (on-behalf-of human:ian) (forge-identity) | base=33b78e4273a2 red rc=1 · head=010c13c16ff9 |
Tree-Digest: sha256:8915e2f3720611c806e5863dea9bcdba311ab9590a50c4ef09315b09cffd6f79
Container-Image: sha256:576cb2e49407447769b79a32af370a23e1c108d62842b8f1772a809d1d3b7617
Verification-Attestation: medici-finance/assay#5739 run=0eb01102d5d5baed8975f122f7fceae90411908b11fce8794453a3b35345fe87 source=010c13c16ff9b531242982a02bf86cc8e1f3ab5d brief=docs/streams/forge-neutral/brief-55-operator-provisioning-of-known-read-only-refresh-profiles.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=0eb01102d5d5baed8975f122f7fceae90411908b11fce8794453a3b35345fe87 source=010c13c16ff9b531242982a02bf86cc8e1f3ab5d tree=sha256:8915e2f3720611c806e5863dea9bcdba311ab9590a50c4ef09315b09cffd6f79 witness=sha256:85a4332acff56bf14eae6306c65d89bb2750c8e06fbe35cb0476b4fa97a4e505 opened=2026-11-08T01:44:17Z closed=2026-11-08T01:47:55Z before=receipt after=check-verifier

#### Re-verification notes — 2026-11-08 acceptance transition

Verifier notes: non-implementer, claude-opus-5-5, tier strong, statusgen/desk-tools v1.0.78 on the host, darwin arm64, KUBECONFIG=/dev/null, normal HOME, and no forge identity exported by the verifier.

- **Source and delivery.** The source is 010c13c16ff9 on main. The delivering change is #5485, merge ef8316b8814d. Its first parent, 33b78e4273a2, is the fail-first base.
- **Verify-table amendment.** The amendment is merge 85898e42b80e (#5706), which moved the brief from version 3 to 4. It changed only the Command cells of rows 7 and 10.
- **Approval.** The approval of that transition is https://github.com/medici-finance/assay/issues/5724#issuecomment-6715508823.
- **Re-derivation.** `deskpathguard rederive` reported 10 rows, 0 author-added. The original table is e5783147c8e9 at merge-base ae3e2b12ded2.
- **Database.** Database rows ran against a disposable local PostgreSQL container created for this run only. No live infrastructure was touched.
- **Earlier records.** Outcome records from earlier attempts are void and superseded by this run.

Attempts:

1. **Host run, 01:37:04Z to 01:40:10Z.** One verifyrun of rows 1-7, 9 and 10 ran with the Acceptance-Transition record above. It used `--acceptance-forge` for the approval read and `--fail-first --base 33b78e4273a2`. This is the credited host witness: the table above with the Base column and `Acceptance-Changes: 7=Command,10=Command`.
2. **Row 8 container run, 01:44:17Z to 01:47:55Z.** Row 8 (check:ci) ran inside the container-netns admission bracket on the same source. It used the pinned verify-runner v1.0.75 image, linux/arm64 manifest, named in its Container-Image line. The baseline history came from `--open-bracket --history-out` and was used for the container fail-first. `--bracket-digest` reported verdict=match. The container got no extra environment beyond the git safe.directory setting and GOCACHE.

Fail-first at base 33b78e4273a2:

- Rows 1-7 and 10 are red at base and pass at head.
- Row 8 is red at base and was not run to a verdict at head (see below).
- Row 9 is green at both, so it does not discriminate. It is the whole-module vet and race suite, and its Expect cell is a whole-suite exit-0 regression gate: exit 0 on the disposable container with no skipped integration credit. That cell carries no fail-first obligation. Rows 1-7 discriminate the delivered controls, and row 9 is the regression gate around them. A green base is therefore expected for this row, and row 9 is credited as a regression gate with this disclosure. verifyrun folds the host run's exit code to 1 because of this row.

Row 7 under the amended command:

- `provision_mutations.py --delivery ef8316b8814d --base 33b78e4273a2` exited 0 in the detached post-merge home and printed its success line. Its base run is red (rc=2).
- Supplemental check, no acceptance credit: the same command was re-run on the same source and database from 01:40:35Z to 01:41:57Z, with output kept. It printed 19 `KILLED` lines, one per named fault. Each fault compiled and failed at its named runtime assertion. The faults include known-baseline, immutable-replay, atomic-initialization, cross-process-source-fence, effective-membership, lower-eviction, owned-role-binding, provenance-immutability and dedicated-native-ci.
- The script restored the exact original bytes. The brief's Evidence edit was the only change left in the home tree.

Row 10 under the amended command:

- The command now uses the explicit immutable bounds `--delivery ef8316b8814d --base 33b78e4273a2`, so its result no longer depends on the working-tree diff.
- Supplemental check, no acceptance credit: the same command was re-run with output kept, using local history only and no forge reads. It reported 12 corroborated, 0 disproved and 2 unchecked, exit 0. The 2 unchecked entries are both out-of-scope entries, which is a reviewer judgement.
- Its limit line says the gate judges only the listed entries, so an omitted consumer would not be seen.

Row 8 was not run to a verdict:

- Inside the pinned image, verifyrun stopped the row at module resolution, before its check ran. The reason it gave is that the image module cache does not match this source's tools/desk go.sum. For example, github.com/jackc/pgx/v5 v5.12.0 and github.com/pressly/goose/v3 v3.29.0 are absent from the cache.
- This is the known runner-image gap #4683, and row 8 is the only row without acceptance credit in this run.

Risk-bearing values were enumerated over the delivered diff and ranked:

- Ranked first: the identifier length bound, the digest size, the timestamp resolution, the source fills bound, the runtime delete grant and the runtime budget update grant.
- Ranked lower:
  - the advisory lock key width, which is the first 8 bytes of sha256 of the source id (refresh_provision.go:137);
  - the 64 KiB manifest cap (refresh_role_profile.go:105), which is an operational limit.
- The amendment adds no literal. Irreversible: no. All cited lines were re-read at source 010c13c16ff9.

RISK-VALUE: DERIVED — `len(s) <= 63` at workflow/store/refresh_role_profile.go:43 is PostgreSQL NAMEDATALEN 64 less the terminator. A longer role name would be truncated and could alias another role.
RISK-VALUE: DERIVED — `octet_length(identity_hash)=32` at workflow/store/migrations/00004_refresh_initialization.sql:4 is the sha256 digest size produced by `sha256.Sum256(r.identity)` at workflow/store/refresh_provision.go:161.
RISK-VALUE: DERIVED — `t.Nanosecond()%1000 == 0` at workflow/store/refresh_provision.go:62 matches timestamptz microsecond resolution, so an accepted instant replays exactly.
RISK-VALUE: DERIVED — the source fills bound `0` in `initialUsage(p.Source.Usage, 0, ...)` at workflow/store/refresh_provision.go:71 is correct because refresh_budgets declares no fills column (workflow/store/refresh_role_profile.go:33).
RISK-VALUE: DERIVED — runtime `delete: true` on refresh_fills at workflow/store/refresh_role_profile.go:36 is bounded below SQL by the refresh_eviction_guard trigger (workflow/store/migrations/00004_refresh_initialization.sql:16-28). Its mutation was killed in the row-7 run.
RISK-VALUE: DERIVED — runtime update of `unknown_hold` on refresh_budgets at workflow/store/refresh_role_profile.go:33 cannot clear it, because workflow/store/migrations/00003_refresh.sql:111 raises 'unknown budget reset'.

**VERIFY: BLOCKED** — row 8 (check:ci) was not run to a verdict because the pinned verify-runner image's module cache does not match this source's tools/desk go.sum (#4683). Under the approved acceptance transition, rows 1-7, 9 and 10 pass. Rows 1-7 and 10 are red at the pre-delivery base. Row 9, the whole-suite regression gate, is green at base, as disclosed above.

### Corrective note on the 2026-11-08 run (row 9 credit withdrawn) — 2026-11-08 claude-opus-5-5-verifier

- Row 9 is risk-bearing. The brief's risk line answers `sensitive-data: yes`, which makes every Verify row risk-bearing and gives every row a fail-first obligation.
- Row 9 was green at base 33b78e4273a2 and green at head 010c13c16ff9. It therefore does not discriminate, and it is uncredited.
- The regression-gate credit given to row 9 in this run's notes and in this run's verdict line above is withdrawn. The row-strength defect is filed as #5752.
- No row was re-run for this note. The witness tables above are unchanged.
- Row 8 stays not run: the pinned verify-runner image's module cache does not match this source's tools/desk go.sum (#4683).
- Rows 1-7 and 10 stay credited, which is 8 of 10 rows. Each is red at base 33b78e4273a2 and passes at head.
- The outcome record landed on 2026-11-07 (55-20261107T223922Z) also credited row 9. That record and this run's earlier blocked record are both superseded by a new record.
- The credited host invocation used `--acceptance-record <descriptor> --acceptance-forge` together with `--fail-first --base 33b78e4273a2`.

**VERIFY: FAIL** — row 9 is a risk-bearing row that does not discriminate: it is green at base 33b78e4273a2 and green at head (#5752). Row 8 (check:ci) was not run because the pinned verify-runner image's module cache does not match this source's tools/desk go.sum (#4683). Rows 1-7 and 10 pass and are red at base (8 of 10 credited).

## Review

Gate: human. Independent verification and acceptance follow the concrete implementation and role proof; operator provisioning, credential selection, serving and activation remain separate acts.
