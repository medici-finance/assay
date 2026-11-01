# Closure Approval Context Implementation Plan

> **For executors:** Follow the approved Assay issue and the repository execution and review procedure. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve repository authority metadata in immutable closure checkouts without granting forge-read permission.

**Architecture:** The existing gitcore checkout owner preserves only origin metadata, stripping URL credentials and excluding all other configuration. Existing statusgen acceptance-transition corroboration and deskevidence explicit opt-in remain the only approval-read path.

**Tech Stack:** Go, existing go-git, standard library URL parser.

**Spec:** Issue #4823 and its retained verifier reports.

## Global Constraints

- No approval bypass, lint downgrade, credential widening, lifecycle changes or independent Verify outcomes.
- Offline checks remain held without explicit --acceptance-forge.
- Every immutable approval coordinate and human provenance check remains unchanged.

## Review Focus

- Missing origin remains missing authority rather than a fabricated repository.
- Foreign origin cannot borrow a GitHub approval.
- Credential-bearing origins cannot copy credentials or configuration.
- Both outcome-record and sidecar closures retain metadata.
- Explicit forge permission still reaches every child only when requested.

## Architecture pickup

Continue at source 6152bbf. Read AGENTS.md, implementation-pickup.md, go-engineering.md and library-first.md; inspected gitcore checkout, statusgen acceptance-transition owner and deskevidence landed/child adapters. The packet matches source. Only production checkout caller is landedClosureTree; both immutable and legacy outcome routes share it. Existing Hostname parsing supports :443, and acceptanceArgs already propagates explicit opt-in. No dependency changes or rival implementation; generic checkout loses origin despite retaining history. Source repository identity must survive without credentials, helpers, hooks or arbitrary config. No active intersecting PR found by issue search; native claim owns this issue.

### Task 1: Preserve metadata at the checkout owner

**Files:** gitcore/checkout.go; gitcore/checkout_context_test.go.
**Interfaces:** CheckoutLocalCommit remains the only local immutable-checkout API.

- [x] Add a planted second checkout caller asserting origin identity and secret/config exclusion; run against pre-fix source and retain the red.
- [x] Preserve a credential-free origin remote in the disposable checkout using existing go-git and stdlib URL parsing; never copy source config.
- [x] Run targeted origin, missing-origin and history checks.

### Task 2: Qualify consumers and unchanged approval boundary

**Files:** tools/desk/cmd/deskevidence/landed_test.go; statusgen/acceptancetransition_test.go; changelog/issue-4823.md.
**Interfaces:** Existing landedClosureTree, acceptanceArgs and statusgen approval resolver.

- [x] Reproduce origin loss in two closure destinations before implementation.
- [x] Assert preserved origin and source immutability through the shared closure owner.
- [x] Exercise :443, foreign, missing, offline, edited, bot and unapproved approval cases plus all four child opt-in routes.
- [x] Run bounded targeted tests, security diff review, weight/test ledger, then publish one draft issue PR #5109.

## Validation record

Pre-fix source: 6152bbf. The shared-checkout planted caller and both landed outcome destinations failed with `remote not found`; the actual protected snapshot closure failed with `approval requires a github.com origin`. After repair, all three regressions pass. The existing protected transition matrix passes; the snapshot matrix covers explicit HTTPS port and missing origin and passes approved, offline, absent, bot, edited, foreign, missing and other-repository cases. All four child permission routes pass. Immutable history, same/cross-repository landing routes and refusal tests pass.

Security review: inspected the changed source and authority consumers. Only URL metadata is persisted in the new repository; URL credentials/query/fragment, source headers/helpers/hooks/identity are excluded. No production approval logic or forge-read permission changes. All provenance checks continue in their existing owner; offline explicitly asserts the approval reader cannot run. Live infrastructure and independent Verify acceptance were not exercised.


## Review pickup at 22413f2a9

Continue on the same PR for review findings C1 (effective-origin scope/list semantics) and C2 (optional metadata failure). Confirmed against the owner: repository Config excludes worktree/global scopes; fetchConfig already corrects pushInsteadOf-only decoding. The prior direct Config call is retired, using the existing config codec and normalization owner. Supported ordered scopes include system/global repository and enabled worktree config. Include directives, dynamic overrides, ambiguous rewrite declarations, multiple fetch URLs and unreadable/invalid origin metadata are withheld; the approval consumer retains could-not-check. No native process is added to the library.

- [x] Reproduce C1 with linked homes whose shared origin names another host/repository, list reset, push-only and multi-valued cases.
- [x] Reproduce C2 with pushInsteadOf-only and malformed/unreadable metadata at the checkout and both closure destinations.
- [x] Share fetch normalization, qualify supported scope/reset/rewrite semantics, preserve sanitization and demonstrate approval consumers hold unavailable context.
- [x] Push a follow-up, respond by finding ID, refresh the PR body/workpad and own bounded CI watch.


Review validation: the new config matrix failed at the exact reviewed source 22413f2a9 for both differing linked origins, local reset, push-only, multiple URLs, push-only rewrite corruption, invalid/unreadable metadata and global scope cases. Snapshot consumer tests failed when malformed metadata aborted history or push-only/multiple origins reached approval. Both unrelated closure destinations failed on malformed metadata. After the scope/normalization repair those groups pass.

The URL reset alone was insufficient for command-priority injection: the positive control proves native Git replaces the reset under both GIT_CONFIG_COUNT and GIT_CONFIG_PARAMETERS, and each actual closure consumer reached the forbidden approval-reader seam before the added refusal. The shared OriginConfigInjected profile now refuses at acceptanceTransition before its origin read; both controls pass. This additional trust-boundary guard is recorded as R-approval-origin-config in docs/contracts.md. why-add: higher-priority injected config cannot be removed by writing a local URL reset; explicit consumer refusal is required. No native process is added to the canonical library path.


CI at 4b4fa4744 found four synthetic SSH fixture strings in the unchanged disclosure gate. Reserved .example spellings still require reviewed exceptions in that matcher. Use canonical public-forge SSH spellings for the differing shared host and SCP rewrite prefix, and SSH username/query coverage alongside the shared-parser HTTP password control. The same scope/reset/sanitization assertions remain; no scanner, allowlist, literal splitting or check changed. The unchanged disclosure/corpus gate and the affected origin fixture groups pass before the follow-up push.



Further C1 pickup at 00eb46178: an isolated parent diagnostic and the actual consumer's red positive control proved an ordinary global insteadOf=origin rule can reinterpret the missing-remote fallback even without injected configuration. Retire the native authority lookup in acceptanceTransition and consume gitcore.OriginAuthority directly, sharing the same bounded observation owner as the snapshot producer. Missing/ambiguous URLs can no longer become a native remote-name fallback. The explicit injected-config refusal remains; no new native process or exception is introduced.



Actual cb09 CI context: checkout-v7 persists authentication through conditional includeIf.gitdir entries (host and container paths). The new native-equivalence fixture reproduces the real profile and fails with unavailable authority before support is added. Adapt the bounded owner to observe only authority-neutral HTTP/credential include files, including neutral nested files (relative paths require a canonical declaring directory), at proven absolute ASCII exact/single-component gitdir(/i) matches. Proven nonmatches are skipped before file access; remote/rewrite-bearing includes and unsupported conditions remain unavailable. Cycle/depth limits apply, and included headers/helper values are never merged, copied, logged or used for transport. This avoids a general conditional/inline Git config interpreter and preserves the existing codec owner.
