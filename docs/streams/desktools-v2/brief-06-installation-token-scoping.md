---
brief: assay:assay:desktools-v2:06
title: installation-token scoping — explicit repo-scoped custody across Go, cellctl and dispatch
why: >-
  Repository and roster resolution must determine identity on every operation, across GitHub
  and GitLab. Current cellctl Go-generated POSIX wrappers preserve an ambient credential before
  swapping HOME; current GitLab dispatch already has explicit role-custody refusal tests. This
  human-gated migration removes remaining ambient identity paths without losing those refusals.
wave: 4
depends: ["desktools-v2/02", "desktools-v2/03"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: [628, 1573, 1203, 655, 676, 677]
schema: brief-v2
authored: 2026-09-16 by desktools-v2 authoring session
sources:
  - "docs/streams/desktools-v2/spec.md §2 Principle 1 CUSTODY — explicit role credentials, per-repo identity, no ambient fallback"
  - "tools/desk/cmd/cellctl/shims.go — Go-generated POSIX wrappers resolve CELLCTL_GH_AMBIENT before HOME swap; observed 2026-10-01"
  - "tools/desk/cmd/deskdispatch/inheritedtokengitlab_test.go — custody PAT chosen over inherited non-role PAT; unreadable custody refuses before claim"
  - "tools/desk/internal/deskkit/forgeresolve.go and forge_github.go — explicit repo resolution and empty-token refusal are retained"
  - "#1573, #1203, #655, #676, #677 — GitLab identity-path reports assigned here by desktools-v2/13; ownership is not a resolution claim"
gate-why: >-
  Identity/auth changes require human acceptance. The human confirms repo + roster determines
  identity on every Go, shim and script path, with no ambient fallback, on both forge backends.
exec-tier: strong
exec-tier-why: >-
  Credential scoping across three surfaces and two forges requires independent negative-path
  proofs; a same-repo happy path cannot detect a token selected for the wrong repository.
domain: complicated
consumers:
  - "tools/desk/internal/deskkit: follow-up desktools-v2/06 (explicit repo-scoped native client token)"
  - "tools/desk/cmd/cellctl: follow-up desktools-v2/06 (Go wrapper generation hands explicit scoped credentials across HOME changes)"
  - "tools/desk/cmd/deskdispatch: follow-up desktools-v2/06 (script children retain explicit custody; existing GitLab refusals remain)"
  - "native Windows cellctl wrappers: out-of-scope (cellctl-windows owns native host wrappers; this brief pins credential selection semantics without claiming POSIX shims run on Windows)"
  - "token minting/authentication: out-of-scope (forge-neutral owns HOW credentials authenticate; this brief owns WHICH repository/role they target)"
  - "earlier repaired shim and dispatch regressions: follow-up desktools-v2/14 (regression floor; closed reports are not this brief's premise)"
version: 3
id: 2808921b-f587-499f-abe8-e87b63dddea6
---

# Brief 06 — installation-token scoping

## Context

files:
- `tools/desk/internal/deskkit/` — repo-derived native token scoping and fail-closed resolution.
- `tools/desk/cmd/cellctl/shims.go` — the current Go wrapper generator, not the retired shell oracle.
- `tools/desk/cmd/deskdispatch/` — explicit custody delivered to script children.
- New/extended Go tests listed below (planned until this human-gated brief is accepted).

Re-derived by desktools-v2/13 against the current tree. This diff authors the contract;
it changes no runtime credential selection and is not the human's acceptance of brief 06.
The older closed-report premises are retired to desktools-v2/14's regression floor.

single-point-of-failure: identity is decided by repo + roster, never ambient environment.
Two independent layers remain: the transport refuses absent explicit credentials, and the
identity resolver derives the target from the repository. The independent ban-lint records
remaining CLI reach-arounds. None of these controls may be weakened to complete this migration.

facts:
- Cellctl is now Go (`cmd/cellctl`). Its generated shims are still POSIX shell wrappers;
  `CELLCTL_GH_AMBIENT` is resolved before HOME is replaced. Preserving ambient identity is
  current behavior, not a proof of the explicit-only target contract.
- Native Windows wrapper implementation belongs to `cellctl-windows`; this brief's Windows
  row tests credential selection under Windows HOME/USERPROFILE semantics in Go, without Bash.
- GitLab identity reports #1573, #1203, #655, #676 and #677 are in this brief's scope. Explicit
  per-role PAT custody replaces GitHub App installation semantics on GitLab; do not invent an
  App-minting path there or accept an unrelated inherited token.
- Existing GitLab custody refusal tests remain regression requirements, including unreadable
  custody refusing before a claim and an inherited non-role PAT not selecting identity.
- The key-presence custody boundary and exactly one role credential per environment remain.

## Human decision

Confirm repo + roster determines identity on Go, cellctl wrappers and dispatch scripts,
for both forges, and every operation refuses without an explicit correctly scoped credential.

Options:
1. **Approve all three surfaces as scoped** — explicit per-repo/role credentials, no ambient
   fallback, preserving native-client and GitLab custody refusals.
2. **Approve Go + client only, defer wrapper/dispatch surfaces** — reroute those surfaces and
   their Verify rows to an explicit follow-up before implementing the reduced scope.
3. **Hold** — no identity-path implementation until the custody design is accepted.

Default if no answer: none. A credential-scoping change never proceeds on silence.

## Ground rules

Feature branch + draft PR only. No main push, ready flip, workflow dispatch or infrastructure
contact. Stop at implemented; independent verification owns verified/done. If completion
requires ambient fallback or weakens refuse-if-unminted, STOP and file needs-decision.

## Task

1. Derive the native client's credential from target repo + roster, not inherited GH_TOKEN.
2. Migrate Go cellctl wrapper generation to explicit scoped credentials across HOME changes.
3. Hand explicit custody to script children, independent of a child's executable name.
4. Remove every ambient identity fallback across all three surfaces; do not leave dormant paths.
5. Add planned negative-path tests `TestRepoTokenScope`, `TestShimTokenScope`,
   `TestScriptTokenScope`, `TestGitLabRepoCustody` and `TestShimWindowsScope`. Each must assert
   the identity actually chosen and refusal on absent/wrong-repo custody; Windows semantics
   use a hermetic Go seam, not a POSIX runtime. Keep existing GitLab refusal tests.
6. Add a class guard covering all credential-selection sites, with a planted second ambient
   fallback that must fail. Report migrated surfaces and the same-script ban count before/after.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./internal/deskkit/` | exit 0 |
| 2 | `cd tools/desk && go test -timeout 60s ./internal/deskkit/` | exit 0; scoping and negative paths pass |
| 3 | `d=$(mktemp -d) && cd tools/desk && go test -timeout 45s ./internal/deskkit/ -run '^TestRepoTokenScope$' -v > "$d/out" 2>&1 && grep -E '^--- PASS: TestRepoTokenScope \(' "$d/out"` | exit 0 and anchored PASS — inherited GH_TOKEN cannot choose installation; planned until accepted |
| 4 | `d=$(mktemp -d) && cd tools/desk && go test -timeout 45s ./cmd/cellctl/ -run '^TestShimTokenScope$' -v > "$d/out" 2>&1 && grep -E '^--- PASS: TestShimTokenScope \(' "$d/out"` | exit 0 and anchored PASS — current Go wrapper generator hands explicit scoped custody across HOME swap; planned until accepted |
| 5 | `d=$(mktemp -d) && cd tools/desk && go test -timeout 45s ./cmd/deskdispatch/ -run '^TestScriptTokenScope$' -v > "$d/out" 2>&1 && grep -E '^--- PASS: TestScriptTokenScope \(' "$d/out"` | exit 0 and anchored PASS — script child receives explicit custody irrespective of executable name; planned until accepted |
| 6 | `test -n "$D" && git rev-parse -q --verify "$D^1^{commit}" >/dev/null && git rev-parse -q --verify "$D^{commit}" >/dev/null && { n0=; n1=; for r in "$D^1" "$D"; do t=$(mktemp -d) && git archive -o "$t.tar" "$r" && tar -xf "$t.tar" -C "$t" && cp tools/desk/scripts/forge-ban.sh "$t/tools/desk/scripts/forge-ban.sh" && sh "$t/tools/desk/scripts/forge-ban.sh" > "$t.out" 2>&1; n=$(sed -n 's/.*reach-around sites: \([0-9][0-9]*\).*/\1/p' "$t.out"); rm -rf "$t" "$t.tar" "$t.out"; if [ -z "$n" ]; then echo "$r NO-COUNT"; exit 1; fi; echo "$r reach-around sites: $n"; if [ -z "$n0" ]; then n0=$n; else n1=$n; fi; done; if [ "$n1" -lt "$n0" ]; then echo "LOWER $n0 -> $n1"; else echo "NOT-LOWER $n0 -> $n1"; exit 1; fi; }` — run from a main checkout with `D` set to this brief's delivering commit on main | exit 0; prints three lines, `<D>^1 reach-around sites: N0`, `<D> reach-around sites: N1`, then `LOWER N0 -> N1`, with N1 STRICTLY LOWER than N0 (the ambient-credential reach-arounds are gone — the removal check). The command decides the verdict itself: it exits 1 on `NOT-LOWER`, on a tree that prints no count, and on a `D` that is unset or does not resolve to a commit with a parent. Both trees are measured with the SAME current script, and the reference is the count at the delivering commit's own merge parent, not the frozen `desktools-v2/02` line in `docs/streams/desktools-v2/forge-ban-baseline.txt`, so sites other PRs add or remove before or after cannot move the verdict (#1529). Evidence cites `D`, the parent sha, N0 and N1 |
| 7 | `d=$(mktemp -d) && cd tools/desk && go test -timeout 45s ./cmd/deskdispatch/ -run '^TestGitLabRepoCustody$' -v > "$d/out" 2>&1 && grep -E '^--- PASS: TestGitLabRepoCustody \(' "$d/out"` | exit 0 and anchored PASS — explicit GitLab role PAT is selected per target, absent/wrong-role custody refuses; planned until accepted (desktools-v2/13 GitLab row) |
| 8 | `d=$(mktemp -d) && cd tools/desk && go test -timeout 45s ./cmd/cellctl/ -run '^TestShimWindowsScope$' -v > "$d/out" 2>&1 && grep -E '^--- PASS: TestShimWindowsScope \(' "$d/out"` | exit 0 and anchored PASS — Go selection seam covers HOME/USERPROFILE and refuses ambient identity; no Windows runtime claim, planned until accepted (OS: Windows) (desktools-v2/13 Windows row) |

## Evidence
<!-- Independent implementation verification only; re-derivation is not execution evidence. -->

## Review

Gate: human. Rows 3–5 independently pin the three surfaces; rows 7–8 cover forge and Windows
credential semantics. Planned tests cannot pass by matching nothing: each row asserts its
anchored top-level PASS line. Row 6 retains its strict removal verdict unchanged. The human
must accept the identity scope before implementation; desktools-v2/13 cannot waive this gate.
