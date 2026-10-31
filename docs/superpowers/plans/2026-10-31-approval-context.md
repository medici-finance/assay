# Closure Approval Context Implementation Plan

> **For executors:** Follow the approved Assay issue and the repository execution and review procedure. Steps use checkbox (`- [x]`) syntax for tracking.

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
- [x] Run bounded targeted tests, security diff review, weight/test ledger, then publish one draft issue PR (publication pending).

## Validation record

Pre-fix source: 6152bbf. The shared-checkout planted caller and both landed outcome destinations failed with `remote not found`; the actual protected snapshot closure failed with `approval requires a github.com origin`. After repair, all three regressions pass. The existing protected transition matrix passes; the snapshot matrix covers explicit HTTPS port and missing origin and passes approved, offline, absent, bot, edited, foreign, missing and other-repository cases. All four child permission routes pass. Immutable history, same/cross-repository landing routes and refusal tests pass.

Security review: inspected the changed source and authority consumers. Only URL metadata is persisted in the new repository; URL credentials/query/fragment, source headers/helpers/hooks/identity are excluded. No production approval logic or forge-read permission changes. All provenance checks continue in their existing owner; offline explicitly asserts the approval reader cannot run. Live infrastructure and independent Verify acceptance were not exercised.
