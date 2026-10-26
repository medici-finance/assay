---
brief: assay:assay:statusgen:23
title: Product target binding and context drift assessment
why: "Architecture-only context cannot detect a brief serving an obsolete product target. Bind both kinds of target and route mismatches through the existing reconciliation owner."
wave: 3
depends: ["statusgen/22", "statusgen/20"]
unblocks: ["statusgen/25", "statusgen/21"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-13 by statusgen newbrief
sources:
  - docs/proposals/non-ci-architecture/26-product-intent-and-portfolio.md
  - docs/proposals/non-ci-architecture/11-target-architecture.md
  - freshness-checked 2026-10-13 @ d484dd5df24f2aa8d557dafce41687b38173f227
exec-tier: strong
exec-tier-why: Cross-contract identity, compatibility and evidence semantics require design and consumer reasoning.
domain: complicated
outcome: none
id: cc77a185-fe5b-42a4-9476-6d4ee4f7c0f8
consumers:
  - "spec/brief-v1.md, brief schemas, statusgen/brieffile.go, statusgen/contexthandoff: follow-up statusgen/23"
  - "plugins/assay/skills/author-spec, plugins/assay/skills/author-brief: follow-up statusgen/21"
  - "statusgen/portfolio: follow-up statusgen/25"
---

# Brief 23 — Product target binding and context drift assessment

## Context

files: `spec/brief-v1.md`, `spec/context-handoff-v1.md`, `schemas/context-handoff-v1.json`, `schemas/amendment-manifest-v1.json`, brief schemas/readers, `statusgen/contexthandoff/`, `statusgen/brieffile.go`, `docs/context-handoff.md` (planned), `docs/contracts.md`; `changelog/statusgen-23-portfolio.md` (planned).

facts (d484dd5df, 13 October 2026; the statusgen/19 line refreshed against main after PR #2763):
- Statusgen/19 owns the initial handoff contract and has merged (PR #2763; contract files exist on main); /20 owns discovery/recheck and is still todo.
- outcome accepts an existing REQ reference or none; appending @revision is not its grammar.
- The optional binding is not shipped at the source pin; this is an additive successor, not a rewrite of /19.

layering: pure typed records/evaluators in `statusgen/contexthandoff`; filesystem and CLI adapters remain outside the package. No network, credentials, scheduler or runtime database. The flow and boundary fixtures below enforce this separation.

single-point-of-failure: an incorrect record interpretation could create a false alignment or completion claim; schema/record validation and independently supplied source/approval/coverage fixtures exercise different failure inputs, while existing admission and publication controls remain outside this read-only capability.

design-fit:
  owner: statusgen/contexthandoff
  contract: product-target binding extension of context-handoff-v1 (planned extension; register its semantic owner in docs/contracts.md)
  retires: []
  weight: "one bounded contract/evaluation capability; no new service, scheduler or dispatch authority"
  why-add: "Existing records do not carry this meaning; extend their owning contracts rather than adding an independent product-state system."

## Read first

- `docs/proposals/non-ci-architecture/26-product-intent-and-portfolio.md` — exact record semantics, boundaries and owner mapping.
- `spec/registers-v1.md`, `spec/brief-v1.md` and the declared dependency briefs.
- `docs/streams/continuing-operations/spec.md` for operational identity and impact ownership.

## Ground rules

- Isolated branch and draft PR; no merge, release or activation.
- Synthetic offline inputs only; no collection from real transcripts or production systems.
- Preserve existing records and populated Evidence; stop at implemented for independent verification.
- Named tests and planned interfaces below must be delivered; they are not current commands claimed to pass.

## Task

1. Add optional outcome-revision qualified by outcome and separate from the architecture baseline. Publish a versioned compatible handoff extension after inspecting /19 and /20; readers reject unsupported mandatory semantics rather than silently dropping them. Preserve legacy unbound briefs.
2. Extend existing discovery/recheck records with declared product bindings, exact mismatch evaluation, coverage and materiality/disposition inputs. Reuse one reconciliation engine. Preserve original selection context and append authorized continue/adapt/stop records with expected revision and rationale; refuse stale concurrent disposition.
3. Expose unapproved rivals, unavailable approvals and undiscovered secondary relationships distinctly. Compatible work may continue under recorded disposition; no automatic cancellation or status change. Extend CLI output through /20’s adapter and publish consumer migration/version tests.

## Interface contract

Input: qualified product target from /22, architecture baseline and /20 snapshot/receipt. Output: separate bindings, mismatch/coverage results and revision-bound amendment/disposition obligations. stale-context is a projection, not a lifecycle status or execution veto.

## Verify

Run from the repository root after implementation. Each named test asserts the stated
behavior and includes the specified failing mutation. A missing test/package or absent
named PASS fails the row; do not substitute a skipped or zero-test run.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestProductBindingIndependentTargets$" ./contexthandoff/ > "$check_output" && grep -F -- "--- PASS: TestProductBindingIndependentTargets " "$check_output")` | exit 0; named PASS; product target change preserves applicable architecture baseline and original binding; mutation: use one revision field for both |
| 2 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestProductBindingApprovalCoverage$" ./contexthandoff/ > "$check_output" && grep -F -- "--- PASS: TestProductBindingApprovalCoverage " "$check_output")` | exit 0; named PASS; unapproved rival does not stale approved work; missing binding/secondary relationship is coverage unknown; mutation: treat absent binding as aligned |
| 3 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestProductBindingCheckpointFlow$" . > "$check_output" && grep -F -- "--- PASS: TestProductBindingCheckpointFlow " "$check_output")` | exit 0; named PASS; real brief reader through resolver/CLI and disposition round trip retains historical evidence and rejects stale rebinding; mutation: replace original selection revision |

## Evidence

<!-- Independent verifier records subject revision, command, exit, named result and environment. No implementation or execution evidence is asserted here. -->

### Independent verification — 2026-10-26 (run 1, source 125611dfb, container bracket)

Runner: assay-verifier-app[bot], model claude-opus-5-5, admitted tier strong; independent non-implementer, fresh context. Observed source: merged main 125611dfbdfb950cf09c003e941c48902803fb1f, detached home admitted by `deskdispatch --check-verifier` (medici-finance/assay#4515).

Delivery: the work landed in the merge of medici-finance/assay#4502 (c2127697bfbee2dad0181347358021ebebf8afd0; first parent 8a22d6b5a16bc3f41e80c4c757376ff195659bb2, branch head 1bbff40ece607785b623b62bd1fe8fe3ea891059), an ancestor of the observed source.

Runner class: local container, `--network none`, default seccomp, image ghcr.io/medici-finance/verify-runner by digest sha256:c704b01c271a8b76c04afbcff6396f3bd88ea91d33a9cad56fccb9fadc12b219 (statusgen v1.0.62, go1.27.2, module cache baked into the image; no extra mount). Every row is check:ci and the darwin host has no `unshare --net`, so rows 1–3 ran inside one closed admission bracket: `statusgen verifyrun --open-bracket` on the host, `--sandbox=container-netns` in the container with the home bind-mounted read-only, `--close-bracket` on the host; `--bracket-digest` reports verdict=match. The Output digest is of each row's stdout (the grep's matched `--- PASS:` line); exit 0 means the named PASS line was present.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestProductBindingIndependentTargets$" ./contexthandoff/ > "$check_output" && grep -F -- "--- PASS: TestProductBindingIndependentTargets " "$check_output")` | pass exit=0 sandbox=container-netns | sha256:72adf0c79364 | 2026-10-26 | assay-verifier-app[bot] @ 125611dfbdfb (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestProductBindingApprovalCoverage$" ./contexthandoff/ > "$check_output" && grep -F -- "--- PASS: TestProductBindingApprovalCoverage " "$check_output")` | pass exit=0 sandbox=container-netns | sha256:d05c3a9a82f5 | 2026-10-26 | assay-verifier-app[bot] @ 125611dfbdfb (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestProductBindingCheckpointFlow$" . > "$check_output" && grep -F -- "--- PASS: TestProductBindingCheckpointFlow " "$check_output")` | pass exit=0 sandbox=container-netns | sha256:65da3e74f566 | 2026-10-26 | assay-verifier-app[bot] @ 125611dfbdfb (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:cbd1c5b5351629d37796916b6201ff92a07962078688c46fb7e7ecf60292dd0e
Container-Image: sha256:c704b01c271a8b76c04afbcff6396f3bd88ea91d33a9cad56fccb9fadc12b219
Verification-Attestation: medici-finance/assay#4515 run=df0b409c9b2e030dc7cfa929f38258bd2106078cc3c38daf388d8c5591eba5cc source=125611dfbdfb950cf09c003e941c48902803fb1f brief=docs/streams/statusgen/brief-23-product-target-binding-and-context-drift-assessment.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=df0b409c9b2e030dc7cfa929f38258bd2106078cc3c38daf388d8c5591eba5cc source=125611dfbdfb950cf09c003e941c48902803fb1f tree=sha256:cbd1c5b5351629d37796916b6201ff92a07962078688c46fb7e7ecf60292dd0e witness=sha256:58a9f2e2d3451ad743099abb7314372220c2ef790f70c5fd9d3673a150bdbd2b opened=2026-10-26T16:53:40Z closed=2026-10-26T16:54:46Z before=receipt after=check-verifier

Supplemental observations — diagnostics only, no acceptance credit for any row. The mutation probes ran in the same image under `--network none`, on scratch copies of the observed source tree outside the home, each applying one entry of the delivery's own mutation catalogue (statusgen/contexthandoff-mutations.json) and then running that row's exact command shape.

| Probe | Command or mutation | Observed | Date | Runner |
|---|---|---|---|---|
| S1 | rows 1–3 command shapes on the unmutated scratch copy | each exit 0 with its named PASS line | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M1 | row 1, catalogue entry row23-1-one-revision-field: the current product target takes the architecture snapshot's target revision instead of the approved product target's revision (one revision field for both) | killed: row 1 command exit 1, `--- FAIL: TestProductBindingIndependentTargets`; the resolver's receipt is refused because the current revision is not the last approved product target of the outcome | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M2 | row 2, catalogue entry row23-2-absent-binding-aligned: an unbound brief is assessed aligned and covered | killed: row 2 command exit 1, `--- FAIL: TestProductBindingApprovalCoverage`; the receipt is refused because an aligned result needs covered coverage, an outcome and both revisions | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| M3 | row 3, catalogue entry row23-3-replace-selection: on recheck the original selection is replaced by the newly declared revision | killed: row 3 command exit 1, `--- FAIL: TestProductBindingCheckpointFlow`, "rebound binding declared r2 selected r2, want declared r2 with the original selection r1 retained" | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |

M1 and M2 are killed by the contract validator the resolver output passes through inside the named test, not by a later field assertion; the row still discriminates the named mutation.

Risk-bearing values. The brief's risk answers are all "no", gate model, not irreversible, and no risk-classed path is touched. Enumeration over the delivery diff's non-test Go source and the new schema found these literals:

- RISK-VALUE: DERIVED — HandoffSchemaV3 = "context-handoff-v3" @ statusgen/contexthandoff/records.go:8 — the brief's Task 1 asks for a versioned extension that readers reject rather than silently drop; a new major schema identifier beside v1/v2 is that, and v1/v2 records carry no product section and refuse one.
- RISK-VALUE: DERIVED — REQ slug bound {8,18} in outcomeRefRe / inRepoReqRe / aliasReqRe @ statusgen/contexthandoff/product.go:87-89 — one leading and one trailing [a-z0-9] plus 8–18 middle characters gives 10–20 characters, exactly the REQ-<slug> grammar spec/registers-v1.md fixes ("10–20 characters of [a-z0-9-], starting and ending with [a-z0-9]"), and the same bound the existing DR-/DEPLOY-/F-/I- identifier patterns use.

Verdict accounting: 3 required rows; 3 pass, witnessed in one closed bracket; 0 fail; 0 could-not-run.

**VERIFY: PASS** — 3/3 rows, each named mutation killed (supplemental).

## Review

Gate: model; all risk answers no because this slice processes synthetic/explicit offline
snapshots and makes no live policy, publication or deployment change. Review semantic
compatibility and the real consumer flow; later live integration derives its own risk gate.
