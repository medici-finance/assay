---
brief: assay:assay:desk-containers:15
title: "The cell-operation contract — one versioned result, plan and plan-approval schema shared by the future Cell API and the CLI, with operator-key custody outside the desks' reach and the CLI as the first adapter"
why: >-
  The cell-level operator verbs (up, drain, restart, status, doctor, logs, gc, upgrade) will
  be driven both from a terminal and, later, from the Cell API that the cockpit and other
  clients use. If the CLI invents its own output and its own confirmation, the API will
  either copy it or diverge, and the cockpit will screen-scrape or fake a TTY, which removes
  the human from the confirmation. This brief defines the operation result, the plan, and
  the plan approval once, as a versioned contract that both surfaces consume. The approval
  is a human decision record in the architecture's existing decision model. It is signed by
  an operator key that desk sessions cannot read or use, over exactly pinned bytes, and it is
  consumed before any effect runs. The CLI is the first adapter.
wave: 7
depends: []
unblocks: ["desk-containers/17", "desk-containers/18", "desk-containers/20", "desk-containers/21", "desk-containers/22", "desk-containers/24"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
decision-trigger: creation
issues: []
schema: brief-v2
version: 1
authored: 2026-10-14 by a worker-desk fan-out session (assay:author-brief, opus-5.5; requested by the driver)
sources:
  - "docs/proposals/non-ci-architecture/11-target-architecture.md §Public Assay includes the control plane and cockpit — 'Use one versioned HTTP/JSON contract for UI and remote clients … CLI and MCP are adapters over those capabilities … Keep the existing human-action absences on agent-facing surfaces'"
  - "docs/proposals/non-ci-architecture/11-target-architecture.md §Runtime durability — 'A human decision binds a human actor, subject, revision and permitted act; agent/MCP surfaces cannot fabricate that act. Exact custody deployment remains a design decision before activation'"
  - "docs/proposals/non-ci-architecture/19-progressive-adoption-and-oversight.md §Sampling and batching — a batch decision binds explicit subject/revision/action/policy tuples, the authorized actor, expiry, revocation and consumption semantics, revalidated before each applicable effect"
  - "docs/proposals/non-ci-architecture/21-cell-operator-surface.md (proposed) (added by this PR) — the contract is target-owned; the R7 Cell API serves it with the same server-side checks; the approval record belongs to the evidence-and-decisions owner"
  - "docs/cellctl-comms.md — the comms trust format has one public key per cell, and desks running as the same OS account share that trust boundary; this is why the operator key cannot live in comms custody"
  - "Security review 5521033873 (F1–F5) and correctness review 5521049854 (F1–F4) at b022723a — custody, signed payload, consume order, API path, cross-cell authority, gate, unrunnable row; house-specific custody detail per the private note"
  - "Driver answers 2026-10-14 (in session, relayed by the coordinator): plan TTL 10 minutes; approvals are signed by an operator key; define the types once for the API and the CLI; consume before (or atomically with) the effect"
  - "freshness-checked 2026-10-14 @ 7bb318197 (origin/main) — no cross-verb result schema, plan record, plan-approval record or operator-key registry exists; tools/desk/internal/comms/identity.go maps one untagged key per cell"
gate-why: >-
  This brief introduces an authentication credential (the operator approval key) and the
  check that is the single control between an approved plan and its effects. What the human
  confirms: where the operator private key lives, and that a desk session (same OS account,
  cell HOME, comms key) cannot read or use it; that approval needs human presence, not a
  keystroke in a terminal a desk can also drive; the exact signed bytes; that consumption is
  atomic and happens before the first effect; and the stated interim threat boundary (the
  approval gates the cellctl and API paths, not a same-account process acting on files
  directly).
exec-tier: strong
exec-tier-why: (a) custody, payload and consumption are design decisions the API and later briefs build on; (b) two consumers (CLI now, API later) must read identical bytes and meanings; (c) signature verification, atomic consumption and re-verification are safety plumbing
domain: complicated
consumers:
  - "spec/cell-op-v1.md and schemas/cell-op-v1.json (planned; the shared contract): follow-up desk-containers/15 (this brief)"
  - "tools/desk/internal/cellop (planned; types, plan store, payload, approval verification, consume, apply harness): follow-up desk-containers/15 (this brief)"
  - "operator-key registry (planned; a new public-key-only file, separate from the comms trust store, referenced from cell configuration): follow-up desk-containers/15 (this brief; the comms trust format itself is NOT changed)"
  - "tools/desk/cmd/cellctl (CLI adapter mechanics; `approve`; the build-tagged fixture verb): follow-up desk-containers/15 (this brief), then each verb brief adopts the contract with its own rows (17–24)"
  - "the R7 Cell API (serves the same documents; runs the same checks server-side; read/plan-only until qualified): out-of-scope (no owner yet; requirements in docs/proposals/non-ci-architecture/21-cell-operator-surface.md (proposed))"
  - "the evidence-and-decisions human-decision record: out-of-scope (architecture owner; the approval becomes a profile of it when that contract ships)"
  - "the console cockpit (renders results; produces approvals client-side under operator presence; never holds the operator key server-side): out-of-scope (the console cockpit lives in a separate repository; follow-up PR there)"
---

# Brief 15 — The cell-operation contract

## Context
files:
- `spec/cell-op-v1.md` (planned) — the contract: result, plan, approval, signed payload, consumption, exit codes
- `schemas/cell-op-v1.json` (planned) — JSON Schema for the documents
- `tools/desk/internal/cellop/` (planned) — `result.go`, `plan.go`, `payload.go` (signed bytes), `approval.go`, `consume.go`, `apply.go`, `registry.go` (operator-key registry), tests
- `tools/desk/internal/cellop/testdata/approval-golden-v1/` (planned) — golden vectors: inputs, expected bytes, signature
- `tools/desk/cmd/cellctl/approve.go` (planned) — `cellctl approve`, presence-proving
- `tools/desk/cmd/cellctl/cellop_fixture.go` (planned) — a fixture verb compiled only under the `cellopfixture` build tag, so this brief's end-to-end row runs before any real verb adopts the contract
- `tools/desk/cmd/cellctl/consent_flag_test.go` (planned), `plan_roundtrip_test.go` (planned)
- `docs/cellctl-machine-seam.md` (planned)
- `changelog/desk-containers-15-cell-op-contract.md` (planned)

single-point-of-failure: the approval check at apply — behind it, apply-time re-measurement of every precondition, atomic consumption before the first effect, the 10-minute expiry, and key custody that keeps the signing key outside the desks' account.

facts:
- **One contract, two adapters.** `cell-op-v1` defines the result, the plan and the approval. The CLI emits and consumes them now; the R7 Cell API serves the same documents later. Neither adapter may add a field the other cannot carry. **The API path is read/plan-only until R7 proves the server-side checks below**; no apply is served before that qualification.
- **Result** (`cell-op-result`): `verb`, `cell`, `started`/`finished`, `outcome` (`ok` | `partial` | `refused` | `failed` | `could-not-check`), `roles[]`, `actions[]`, `plan_id`, `next[]`. Unmeasured values are `null`. Exit codes: 0 ok, 1 failed/partial, 5 refused, 6 could-not-check. Read verbs (status, logs) report per-field nulls inside an `ok` result and exit 6 only when the cell itself cannot be loaded.
- **Plan** (`cell-op-plan`): verb, `cells[]` (every cell the plan affects, sorted), the action list, each action's precondition evidence, created time, a fixed **10-minute** expiry, and `digest` = sha256 of the plan's **RFC 8785 (JCS) canonical JSON**, with the `digest` field itself omitted. Stored 0600 under the cell's `run/plans/`.
- **Batch plans.** A plan may instead list ordered child plans (id + digest); its digest therefore binds every child. One approval of the batch authorizes the children in order. Each child is re-measured and consumed atomically immediately before its own step. The batch expiry bounds the start of each step, and a stale child stops the sequence. This is the decision model's batch form (explicit subjects, revisions, expiry, consumption); `restart --now` (desk-containers/19) is the first user.
- **Operator-key custody (the human decision below).** The operator signing key is never in the comms trust store, never under a cell's home, and never readable or usable by a process running as the desks' OS account with a cell's environment. House-specific placement detail is per the private note.
  - The registry that verifies approvals holds **public keys only**: a new operator-key registry file, separate from the comms trust store, listing key id, public key, and the cells it may approve for.
  - **Desks may read the registry but cannot change it.** Two independent layers:
    - **Ownership:** the registry file is owned by an account other than the desks' (or a root-owned path), and verification refuses (`could-not-check`, exit 6) if the registry or its directory is writable by the verifying process's uid.
    - **Chain of enrollment:** every entry after the first is countersigned, over the `cell-op-operator-key/v1` domain, by an already-enrolled operator key, which needs presence. An entry without a valid countersignature is ignored and reported. The first key is enrolled at install with presence, and its fingerprint is printed for the operator to record.
  - `cellctl approve` signs only with a key that requires **human presence** (an OS keychain or hardware key with a user-presence prompt). A typed `y` in a terminal is never an approval, because desk panes are terminals too.
- **Signed bytes (pinned).** The signature is Ed25519 over:
  1. the ASCII domain tag `cell-op-approval/v1`;
  2. then, each as a 4-byte big-endian length followed by its UTF-8 bytes, in this order: `key_id`, `verb`, `plan_id`, `plan_digest` (lowercase hex), `cells` (the sorted cell names joined with `\n`), `issued` (RFC 3339 UTC, second precision), `expiry` (same format).

  The domain tag makes a comms assertion signature, or any other signature by the same key, invalid as an approval. Golden vectors are part of the contract, so the out-of-repo cockpit must reproduce the exact bytes.
- **Approval** (`cell-op-approval`) is the human-decision record:
  - **actor:** `key_id`;
  - **subject:** `cells` + `plan_id`;
  - **revision:** `plan_digest`;
  - **act:** `verb`;
  - **expiry:** `expiry`;
  - **consumption:** once.

  When the evidence-and-decisions owner ships its decision contract, this becomes a profile of it.
- **Apply.** In order, all **server-side** for the API and in-process for the CLI:
  1. verify the approval: domain tag, every field of the payload, the signature, the key against the registry, and the key authorized for **every** cell in `cells`;
  2. re-measure every precondition;
  3. **consume atomically**, by exclusive creation of `run/plans/consumed/<plan-id>`; if it already exists, refuse `already-consumed`;
  4. only then run the actions, recording the result.

  A failed approval check is refused `approval-invalid`; a precondition that changed is refused `plan-stale`; a key not authorized for an included cell is refused `cell-not-authorized`. All are exit 5 and run nothing. A failure after consumption leaves the plan consumed with a failed result, never re-runnable.
- **Who can write the consumed set.** Today it is the operator's OS account, which is also the desks' account. That is accepted explicitly, and bounded: a reset consumed marker still needs a fresh human-presence approval within the 10-minute expiry and still passes the re-measure. The R7 server keeps its consumed set in its coordination store.
- **Interim threat boundary.** The approval gates the `cellctl` and Cell API paths. It does not stop a same-account process from acting on files or processes directly; that boundary belongs to the runtime-isolation owner (R3).
- **No blanket consent.** There is no `--yes`, no flag that skips a check, and no apply without an approval record.

## Human decision
Where should the operator's approval key live, so that desk sessions on the same machine cannot use it to approve their own operations?

Plans for risky cell operations (abandoning in-flight work, killing orphaned processes, upgrading shared binaries) only run with an operator's signed approval. That approval only means something if the AI desks cannot produce it. The desks run as the same user account as the operator and can read the cell's configuration folder, so the key must be somewhere they cannot read or use.

Options:
1. **OS keychain or hardware key with a presence prompt** (recommended) — the key stays in the operator's login keychain or on a hardware token, and every signature needs a fingerprint, touch or password prompt that a background desk cannot answer. Little setup; ties approval to a human at the machine.
2. **A separate OS account for signing** — the key lives under another user account the desks cannot read, with approval run through that account. Stronger isolation from file access, more setup, and still no proof of presence on its own.
3. **No non-interactive approvals yet** — ship only read/plan verbs and keep risky operations behind manual commands until R7 provides server-side custody.

Default if no answer: none — blocks until answered.

## Ground rules
- NEVER git push / trigger workflows. Stop at `implemented`.
- This brief ships the contract, the registry, the package, `approve`, the fixture verb and the CLI mechanics. Each verb's adoption is that verb's brief, with its own rows. No HTTP server is built here.
- Never place, generate or copy a private key under a cell directory or in any path the desks' environment names.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write `spec/cell-op-v1.md` (planned) and `schemas/cell-op-v1.json` (planned), including the pinned signed bytes, JCS digest rule, consume order and threat boundary.
2. Implement `cellop` (types, plan store, payload, approval verification, registry, atomic consume, apply harness) and the golden vectors.
3. Implement `cellctl approve` against a presence-requiring key provider, the build-tagged fixture verb, and the CLI mechanics.
4. Docs and changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `go -C tools/desk test ./internal/cellop/ -count=1 -v` | exit 0; output contains `--- PASS` for every named test below |
| 2 | check +dereference | `go -C tools/desk test ./internal/cellop/ -run 'TestApprovalGoldenVectors' -count=1 -v` | `--- PASS` — the payload builder reproduces the committed golden bytes and signature exactly; `spec/cell-op-v1.md` (planned)'s worked example uses the same vector |
| 3 | check +mutation | `go -C tools/desk test ./internal/cellop/ -run 'TestApprovalEveryFieldBound' -count=1 -v` | `--- PASS` — editing any one of key id, verb, plan id, digest, cells, issued or expiry after signing is refused `approval-invalid`; a mutant omitting any field from the payload fails |
| 4 | check | `go -C tools/desk test ./internal/cellop/ -run 'TestApprovalDomainSeparation' -count=1 -v` | `--- PASS` — a valid comms assertion signature by the same key, presented as an approval, is refused |
| 5 | check | `go -C tools/desk test ./internal/cellop/ -run 'TestApplyRefusesUnsignedOrUnknownKey' -count=1 -v` | `--- PASS` — no approval, an empty one, a key absent from the operator registry, and the cell's comms key are each refused before any probe runs |
| 6 | check | `go -C tools/desk test ./internal/cellop/ -run 'TestApplyCrossCellAuthority' -count=1 -v` | `--- PASS` — a plan whose `cells` includes a cell the approving key is not registered for is refused `cell-not-authorized`; a plan acting on a cell absent from `cells` is refused |
| 7 | check +mutation | `go -C tools/desk test ./internal/cellop/ -run 'TestConcurrentApplyRunsOnce' -count=1 -v` | `--- PASS` — 16 concurrent applies of one approved plan: exactly one runs its actions and the rest are refused `already-consumed`; a mutant that consumes after the effect fails |
| 8 | check | `go -C tools/desk test ./internal/cellop/ -run 'TestValidApprovalStillReverifies' -count=1 -v` | `--- PASS` — a correct approval for a plan whose precondition changed is refused `plan-stale` and is not consumed |
| 9 | check | `go -C tools/desk test ./internal/cellop/ -run 'TestPlanTTLAndForeignCell' -count=1 -v` | `--- PASS` — apply after 10 minutes, and apply of another cell's plan, are refused |
| 10 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestDeskEnvCannotApprove' -count=1 -v` | `--- PASS` — run with a desk's environment (cell HOME, the comms key variables, the cell marker) and no presence-capable key provider, `cellctl approve` fails, and no file readable from that environment yields an approval that apply accepts |
| 10b | check | `go -C tools/desk test ./internal/cellop/ -run 'TestRegistryWritableByVerifierRefuses' -count=1 -v` | `--- PASS` — when the registry file or its directory is writable by the verifying uid, verification refuses `could-not-check` and no approval is accepted |
| 10c | check +mutation | `go -C tools/desk test ./internal/cellop/ -run 'TestRegistryUncountersignedEntryIgnored' -count=1 -v` | `--- PASS` — a new operator public key appended from the desk account without a countersignature from an enrolled key is ignored and reported, and approvals signed by it are refused `approval-invalid`; a mutant that trusts every entry fails |
| 11 | check +flow | `go -C tools/desk test -tags cellopfixture ./cmd/cellctl/ -run 'TestPlanApplyRoundTrip' -count=1 -v` | `--- PASS` — through the binary built with the fixture verb: `--plan --json`, `approve` with a fixture presence provider, `--apply` in a second process, a third apply refused `already-consumed` |
| 11b | check | `go -C tools/desk test ./internal/cellop/ -run 'TestBatchPlanChildren' -count=1 -v` | `--- PASS` — a batch approval binds each child by digest; an edited child is refused; children are consumed one at a time before their step; a child not started before expiry is not run |
| 12 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestNoBlanketConsentFlag' -count=1 -v` | `--- PASS` — no cellctl command defines `yes`, `y`, `assume-yes` or `skip-verify` |
| 13 | check | `! go -C tools/desk run ./cmd/cellctl/ cellop-fixture` | exit 0 — the fixture verb is absent from a default build |
| 14 | check | `statusgen --root . --lint` | exit 0 |
| 14a | check | `statusgen --root . --consumers --brief desk-containers/15` | exit 0 — the `consumers:` routing holds against the branch diff |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: human (see gate-why). The reviewer answers both core-system questions: what single control stands between a desk and an approved effect (the presence-bound key, backed by re-measure and consume-before-effect), and which rows prove a lower layer holds with the upper bypassed (rows 8 and 10).
