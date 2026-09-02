---
brief: desk-tools/06
title: "Roster from deployment — resolve trust / role-binding config from the cell registry + mounted secrets, not a machine-local `roster.env` (design direction)"
why: >-
  The desk tools fail closed on a machine-local `roster.env` — the hand-kept file that carries the
  trusted logins, the role→App bindings, the scan/allowed repo set, and the bless login. That file
  is a laptop-era coupling, and it is now the source of recurring operational pain: helper tokens
  cached in it age to 401 mid-session, its contents drift out of parity with the CI variables that
  are supposed to mirror them, and a single unreadable repo can fail the whole trust roster closed.
  The cell-separation direction says the deployed environment should own this: in pod / container
  environments the machine-local `roster.env` dependency should go away, and laptops should get a
  GENERATED roster from the same source rather than a hand-maintained one.
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-30 by an authoring session (author-brief); design-direction brief
sources:
  - "The cell-separation ruling: in pod / container environments the machine-local roster.env dependency should be removed — the roster need goes away once containers power the desks, and the roster/trust config is resolved from the deployed environment instead."
  - "The concrete operational pain the machine-local roster.env produces today: cached helper tokens aging to 401 mid-session, CI-variable vs roster.env parity drift, and one unreadable repo fail-closing the entire trust roster."
  - "The container packaging direction (sibling desk-containers stream): no credential ever appears in an image layer; per-role / per-cell key material is injected at runtime via mounted sealed secrets — the natural neighbour of the pod resolution path this brief directs."
exec-tier: strong
exec-tier-why: "this is a design decision every desk role and every desk verb inherits — where the trust roster, the role→App bindings, the repo set and the bless login are resolved from is a boundary contract, not a local edit."
---

# Brief 06 — Roster from deployment (design direction)

**This is a DESIGN-DIRECTION brief.** It records the direction and names the deliverables a
follow-on implementation brief-set would produce. It implements none of them. A reviewer agrees or
disagrees with the DIRECTION and the resolution mapping; the implementation briefs are authored
separately once the direction stands.

## Dependencies
None as a typed edge. The pod resolution path this directs is a natural neighbour of the sibling
container-packaging stream's runtime-credential contract (no credential in an image layer;
per-role / per-cell keys injected via mounted sealed secrets) — the two share the mount contract,
but neither blocks the other's design.

## Context
files (design artifact): this brief is the design of record. The follow-on implementation
brief-set (named under **Deliverables** below) will land the render verb, the pod resolution path,
the sealed-secret mount contract, and the migration — each its own brief with its own executable
Verify table.

single-point-of-failure: the resolved config's SOURCE OF TRUTH. Today that is a hand-kept
`roster.env`; the direction moves it to `cells.yaml` (the cell registry, which already declares the
cell's repo set and its App bindings) plus mounted sealed secrets for key material. Behind that one
source: per-repo / per-cell resolution is INDEPENDENT — an unreadable repo or a missing key degrades
THAT cell's binding, it never fails the whole roster closed; and the render is DETERMINISTIC — the
laptop roster is regenerated from the registry, never curated, so laptop and pod cannot drift.
Independent because they fail on different signals (a declared-registry lookup vs a mounted-secret
read) in different places (the config resolver vs the secret mount).

The machine-local `roster.env` carries four distinct config concerns (plus the key material). The
direction resolves each from the deployed environment in a pod, and from a GENERATED (never
hand-edited) roster on a laptop. **This mapping is the core of the brief:**

| Config concern | Pod / container resolution | Laptop / interactive resolution |
|---|---|---|
| Trusted logins + trusted bot slugs (the trust roster) | derived from the cell registry (`cells.yaml`) + env override | rendered from `cells.yaml` by the render verb; never hand-edited |
| Role → App bindings (which App backs each desk role) | derived from `cells.yaml`, keyed to the mounted keys | rendered from `cells.yaml`; local key material from the developer's config dir |
| Scan / allowed repo set | the cell registry declares the repo set — no probing to discover it | rendered from `cells.yaml` |
| Bless login (the maintainer whose comment blesses untrusted content) | derived from `cells.yaml` + env override | rendered from `cells.yaml` |
| Per-role / per-cell KEY MATERIAL (App PEMs, tokens) | mounted **sealed secrets** at known paths (the custody ladder below) — never an image layer, never `roster.env` | the developer's local secret store / config dir (unchanged) |

facts:
- **The secret-custody ladder (the "mounted sealed secrets" layer, made explicit).** The pod's
  key material is not one fixed mechanism but a staged custody path, and the design names the whole
  ladder:
  - TODAY — role / App PEMs live in the developer's local config dir for laptop sessions, and as
    SOPS-sealed Kubernetes Secrets for pods.
  - TARGET — a self-hosted secret store (OpenBao) is the source of record; pods read their role keys
    from it (via a vault-config-operator-style CRD), and the SOPS seals RETIRE per-key as the store
    takes each one over.
  - MIGRATION DIRECTION — SOPS-sealed Secrets → the secret store, **per key**, not big-bang.
  The pod resolution path this brief directs reads whichever rung is current for a given key; the
  laptop path keeps local-config-dir PEMs until the store is the source. Standing up the secret
  store is explicitly OUT OF SCOPE here — it is the OpenBao secret-store track, gated on its own
  install; this brief only names the store as where the key material ends up.
- The rendered roster carries **no long-lived helper token**. Tokens are resolved at use time from
  the key material (a mounted sealed secret in a pod, a PEM minted fresh on a laptop) — never cached
  in the roster file. This is what removes the "cached helper token ages to 401 mid-session" pain:
  there is no cached token to age.
- **Parity is structural, not maintained.** CI variables and the local roster both DERIVE from the
  one `cells.yaml`, so they cannot drift out of parity by hand-edit — the drift class is designed
  out, not policed.
- **Per-repo resolution is independent and fails closed per-cell, not per-fleet.** Because the repo
  set is DECLARED in the registry rather than discovered by probing each repo, one unreadable repo
  degrades that repo's binding only; it never fail-closes the whole trust roster. This is the
  explicit fix for today's whole-roster fail-closed pain.
- **Determinism is the contract of the render verb.** `deskroster render --from cells.yaml`
  produces the roster deterministically; a `--check` / `--diff` mode asserts a checked-in or live
  roster matches what the registry would render, so drift is detectable in CI. The rendered roster
  is a build output, never a hand-edited file.
- `roster.env`, `cells.yaml`, and `deskroster` are desk-tool concepts; nothing in this direction
  names or depends on any particular deployment's repositories, issues, or paths.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Author the brief only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- DESIGN DIRECTION only: name the follow-on deliverables, do not implement any of them.

## Deliverables (the follow-on implementation brief-set — NAMED here, authored separately)
1. **The render verb** — `deskroster render --from cells.yaml`: deterministic roster generation from
   the cell registry, plus a `--check` / `--diff` mode that fails when a checked-in or live roster
   no longer matches the registry. Output is a build artifact; the hand-edited `roster.env` is
   retired as a source.
2. **The pod config-resolution path** — deskroster and the desk verbs resolve the four config
   concerns from `cells.yaml` + mounted sealed secrets + env when running in a container, with **no
   `roster.env` required**. Includes the resolution PRECEDENCE (registry → mounted secret → env
   override) and the fail-closed-per-cell semantics.
3. **The sealed-secret mount contract + custody ladder** — the per-role / per-cell key material
   mounted at known paths, never in an image layer; the path / naming convention, the
   fail-closed-on-missing-key behaviour, its relationship to the sibling container stream's
   runtime-credential contract, AND the custody ladder (SOPS-sealed Kubernetes Secrets today → a
   self-hosted secret store as end-state, per-key). Standing up the store is NOT in this
   brief-set — it is the OpenBao secret-store track, gated on its own install; the resolution path
   only reads whichever rung is current.
4. **The migration off `roster.env`** — a staged path: (a) render produces a roster at parity with
   the hand-kept file; (b) pods resolve directly from the registry + mounts; (c) a compatibility
   window accepts either source; (d) the hand-kept `roster.env` is deprecated and removed. Each
   stage is independently verifiable.

## DoD
- The design direction is recorded: the four config concerns are each mapped for BOTH pod and
  laptop resolution (the Context table).
- Each of today's three named pain points — cached helper token aging to 401, CI-vs-roster parity
  drift, one unreadable repo fail-closing the whole roster — is tied to the specific property of the
  direction that removes it.
- The four follow-on deliverables are named with scope (render verb, pod resolution path,
  sealed-secret mount contract, migration), and the secret layer is stated as the custody ladder
  (SOPS-sealed today → self-hosted store as end-state, per-key) with the store install scoped OUT.
- The brief is self-contained: no deployment-specific repository names, issue references, or paths;
  the ruling is cited neutrally.
- `statusgen --lint --root .` → LINT: PASS.

## Verify
These rows check the DESIGN's own properties — that the direction is completely and self-containedly
recorded — not any implementation (there is none).

Each command is hermetic (network-off, no state) and asserts on EXIT STATUS. Commands carry no `|`
character, per the stream convention (a bare pipe in a cell reads as a column separator); the
mapping-count row stops at the `## Verify` heading with awk so it cannot match its own patterns.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `awk '/^## Verify/{exit} /Trusted logins \+ trusted bot slugs/{n++} /Scan \/ allowed repo set/{n++} /Bless login \(the maintainer/{n++} /Per-role \/ per-cell KEY MATERIAL/{n++} /Role → App bindings \(which App/{n++} END{exit(n==5?0:1)}' docs/streams/desk-tools/brief-06-roster-from-deployment.md; echo $?` | 0 — the resolution table maps exactly five concerns (four config + key material) for pod and laptop |
| 2 | check:ci | `grep -q 'deskroster render --from cells.yaml' docs/streams/desk-tools/brief-06-roster-from-deployment.md; echo $?` | 0 — the render verb is named |
| 3 | check:ci | `grep -q 'render verb' docs/streams/desk-tools/brief-06-roster-from-deployment.md && grep -q 'pod config-resolution path' docs/streams/desk-tools/brief-06-roster-from-deployment.md && grep -q 'sealed-secret mount contract' docs/streams/desk-tools/brief-06-roster-from-deployment.md && grep -q 'migration off' docs/streams/desk-tools/brief-06-roster-from-deployment.md; echo $?` | 0 — all four follow-on deliverables named |
| 4 | check:ci | `grep -q '401' docs/streams/desk-tools/brief-06-roster-from-deployment.md && grep -q 'parity' docs/streams/desk-tools/brief-06-roster-from-deployment.md && grep -q 'fail-clos' docs/streams/desk-tools/brief-06-roster-from-deployment.md; echo $?` | 0 — each named pain point (401 token age, parity drift, whole-roster fail-closed) is addressed |
| 5 | check:ci | `grep -q 'secret-custody ladder' docs/streams/desk-tools/brief-06-roster-from-deployment.md && grep -q 'OpenBao secret-store track' docs/streams/desk-tools/brief-06-roster-from-deployment.md; echo $?` | 0 — the secret-custody ladder is stated and the store install is scoped OUT (its own gated track) |
| 6 | check:ci | `if grep -qE '[^/A-Za-z0-9_-]#[0-9]+' docs/streams/desk-tools/brief-06-roster-from-deployment.md; then echo 1; else echo 0; fi` | 0 — no bare `#NNN` issue reference (self-containment proxy: neutral citation only) |
| 7 | check:ci | `statusgen --lint --root . >/dev/null 2>&1; echo $?` | 0 — LINT: PASS |

Row 6 is the negative-path row: a bare cross-repo issue reference in a public brief would fail it.

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner). -->

All seven rows are pure text/lint checks over this brief's own committed content (no network
call, no mutating action). They were run directly in the implementer's worktree — `statusgen
verifyrun` (the execution-witness tool) could not itself produce a witness here because its
`check:ci` class re-executes hermetically inside a network-off `unshare --net` sandbox, a Linux
facility unavailable on this darwin host; it reported `could-not-run` for all seven rows rather
than a fabricated pass (three-state: could-not-check is never rounded up to a pass). The rows
below are the direct, non-hermetic run — genuinely executed, honestly labelled as such rather
than as a sandboxed witness. A Linux runner (CI, or `verify-desk` at verify time) can produce the
formal `statusgen verifyrun` witness by re-running the same command.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `awk '/^## Verify/{exit} /Trusted logins \+ trusted bot slugs/{n++} /Scan \/ allowed repo set/{n++} /Bless login \(the maintainer/{n++} /Per-role \/ per-cell KEY MATERIAL/{n++} /Role → App bindings \(which App/{n++} END{exit(n==5?0:1)}' docs/streams/desk-tools/brief-06-roster-from-deployment.md; echo $?` | manual-run exit=0 | `0` (sha256:9a271f2a916b) | 2026-09-01 | assay-worker-app[bot] (darwin, non-hermetic) |
| 2 | `grep -q 'deskroster render --from cells.yaml' docs/streams/desk-tools/brief-06-roster-from-deployment.md; echo $?` | manual-run exit=0 | `0` (sha256:9a271f2a916b) | 2026-09-01 | assay-worker-app[bot] (darwin, non-hermetic) |
| 3 | `grep -q 'render verb' ... && grep -q 'pod config-resolution path' ... && grep -q 'sealed-secret mount contract' ... && grep -q 'migration off' ...; echo $?` | manual-run exit=0 | `0` (sha256:9a271f2a916b) | 2026-09-01 | assay-worker-app[bot] (darwin, non-hermetic) |
| 4 | `grep -q '401' ... && grep -q 'parity' ... && grep -q 'fail-clos' ...; echo $?` | manual-run exit=0 | `0` (sha256:9a271f2a916b) | 2026-09-01 | assay-worker-app[bot] (darwin, non-hermetic) |
| 5 | `grep -q 'secret-custody ladder' ... && grep -q 'OpenBao secret-store track' ...; echo $?` | manual-run exit=0 | `0` (sha256:9a271f2a916b) | 2026-09-01 | assay-worker-app[bot] (darwin, non-hermetic) |
| 6 | `if grep -qE '[^/A-Za-z0-9_-]#[0-9]+' docs/streams/desk-tools/brief-06-roster-from-deployment.md; then echo 1; else echo 0; fi` | manual-run exit=0 | `0` (sha256:9a271f2a916b) | 2026-09-01 | assay-worker-app[bot] (darwin, non-hermetic) |
| 7 | `statusgen --lint --root . >/dev/null 2>&1; echo $?` | manual-run exit=0 | `0` — `LINT: PASS` (sha256:9a271f2a916b) | 2026-09-01 | assay-worker-app[bot] (darwin, non-hermetic) |

### Non-implementer verifier run — VERIFY: PASS — 2026-09-01 opus-4.8[1m]-verifier (verify-desk dispatch), merged main `5927efe`

Runner ≠ implementer (the rows above are the implementer's; this is the independent non-implementer re-run). Own temp worktree off `origin/main`, offline (`KUBECONFIG=/dev/null`). Design-direction brief — all seven Verify rows are text/grep/lint checks over the brief's own committed content.

| # | Command | Exit | Result |
|---|---------|------|--------|
| 1 | `awk '…5 resolution-table concerns… END{exit(n==5?0:1)}' <brief>` | 0 | resolution table maps exactly 5 concerns (4 config + key material) |
| 2 | `grep -q 'deskroster render --from cells.yaml' <brief>` | 0 | render verb named |
| 3 | grep for `render verb` + `pod config-resolution path` + `sealed-secret mount contract` + `migration off` | 0 | all four follow-on deliverables named |
| 4 | grep for `401` + `parity` + `fail-clos` | 0 | three named pain points addressed |
| 5 | grep for `secret-custody ladder` + `OpenBao secret-store track` | 0 | custody ladder stated, store install scoped OUT |
| 6 | `grep -qE '[^/A-Za-z0-9_-]#[0-9]+' <brief>` (negative) | 0 | no bare `#NNN` issue ref (self-containment proxy) |
| 7 | `statusgen --lint --root <worktree>` | 0 | `LINT: PASS` (only non-fatal NOTICEs) |

`RISK-VALUE: N/A` — design-direction brief; implements no code, names no literal constant in any guard (config concepts `roster.env`/`cells.yaml`/`deskroster` are named neutrally, not as source values).

**VERIFY: PASS** — all seven Verify rows checked-clean by a non-implementer. Advancing `implemented → verified`.

## Review
Gate: model (from frontmatter). A reviewer answers, in the verdict: (1) is `cells.yaml` + mounted
sealed secrets + env the right source of truth to replace the hand-kept `roster.env`, and is the
per-cell (not per-fleet) fail-closed boundary acceptable? (2) does the resolution mapping cover
every concern the machine-local `roster.env` carries today, with a named pod AND laptop answer for
each? Reviewer records verdict + date in the stream README table.
