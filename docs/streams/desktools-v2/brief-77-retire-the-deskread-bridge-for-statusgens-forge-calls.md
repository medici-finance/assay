---
brief: assay:assay:desktools-v2:77
title: "Retire the deskread bridge for statusgen's forge calls"
why: "After forge-neutral/18 and /35, every statusgen forge read arrives at one reader seam whose online implementation starts a deskread process. The driver's ruling has statusgen link the shared forge library instead, and statusgen still reads forge tokens from the environment in four files. This brief swaps the one reader and retires those token reads, so a credential exists in statusgen only inside the library's client."
wave: 7
depends: ["desktools-v2/75", "desktools-v2/58", "forge-neutral/18", "forge-neutral/35"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
design: DR-forge-library
issues: [2395]
schema: brief-v2
outcome: none
version: 1
id: 8ca0dffc-43e2-4ba1-911a-b26fab5b4d62
authored: 2026-10-25 by desktools-v2 authoring session
sources:
  - "docs/streams/decisions/DR-forge-library.md — point 2 of the driver's ruling (https://github.com/medici-finance/assay/issues/2395#issuecomment-6404972196)"
  - docs/streams/forge-neutral/brief-18-statusgen-off-gh-one-read-verb.md — the reader seam and rows 21 to 23
  - docs/streams/forge-neutral/brief-35-ruling-resolver-onto-the-read-verb.md
  - docs/streams/desktools-v2/brief-08-hold-statusgen-at-zero.md — rows 4, 6, 7 and 10
  - "freshness-checked 2026-10-25 @ d6a2ff244 (origin/main)"
exec-tier: strong
exec-tier-why: "This is the step that puts a credential into the statusgen process. A swap that keeps an environment-token fallback, or lets the pure evaluation package link the library, would pass every read test; only the negative controls on token reads and imports catch it."
domain: complicated
gate-why: "sensitive-data is yes: statusgen gains a minted credential for the first time. The human confirms it is held only by the library client, that the old environment-token and gh fallbacks are gone, and that statusgen/evaluation links no forge library."
decision-trigger: start
consumers:
  - "statusgen/forgeread.go: follow-up desktools-v2/77 (the online reader becomes library-backed; the offline and frozen readers are unchanged)"
  - "statusgen/claimdecay_github.go, statusgen/claimdecay_gitlab.go, statusgen/doratiming.go, statusgen/ghfetch.go: follow-up desktools-v2/77 (environment-token reads retire; the claim-decay `gh auth token` fallback is already gone, retired by forge-neutral/18, and row 3 holds it gone)"
  - "statusgen/decisionruling.go: follow-up forge-neutral/35 (its token reads retire there)"
  - "statusgen/go.mod: follow-up desktools-v2/77 (requires the library module by require plus relative replace)"
  - "statusgen/evaluation/: out-of-scope (stays pure; row 4 proves it links no forge library)"
  - "docs/streams/desktools-v2/brief-08-hold-statusgen-at-zero.md rows 4, 6, 7 and 10: follow-up desktools-v2/77 (rows 4, 6 and 7 re-pointed from a stub deskread to the library over a fake provider; row 10's allowed set widened to the library's caller package)"
  - "tools/desk/scripts/forge-ban.sh, tools/desk/internal/forgeban/allowlist.go: follow-up desktools-v2/77 (the combined gh/glab ratchet desktools-v2/75 adds is lowered to the ruled ceiling of 7)"
  - "docs/library-first.md, forgeread/README.md: follow-up desktools-v2/77"
---

# Brief 77 — Retire the deskread bridge for statusgen's forge calls

## Context

files:
- `statusgen/forgeread.go` and its tests, `statusgen/go.mod`
- `statusgen/claimdecay_github.go`, `statusgen/claimdecay_gitlab.go`, `statusgen/doratiming.go`,
  `statusgen/ghfetch.go`
- `statusgen/forgeclient_nogh_test.go` (planned)
- `tools/desk/scripts/forge-ban.sh`, `tools/desk/internal/forgeban/allowlist.go` (the combined
  gh/glab ratchet from desktools-v2/75)
- `docs/library-first.md`, `forgeread/README.md`
- `changelog/statusgen-forge-library.md` (planned)

single-point-of-failure: the library client's constructor and private credential field,
reached from statusgen's adapter layer only. Behind it: the forge's App installation scope;
row 2, which fails on any forge-token environment read left in statusgen; row 4, which
keeps the pure evaluation package off the library; and row 7, which fails when any statusgen
package other than the adapter layer imports the library.

facts (d6a2ff244, 2026-10-25):
- `statusgen/forgeread.go` defines the `forgeReader` seam with offline, frozen and `deskread`
  process implementations; the offline reader is the default.
- Forge-token environment reads in non-test statusgen code: `claimdecay_github.go` 2,
  `claimdecay_gitlab.go` 3, `decisionruling.go` 2, `doratiming.go` 1, `ghfetch.go` 1.
  `claimdecay_github.go` also falls back to `gh auth token --hostname <host>`; forge-neutral/18
  retires that fallback (its 2026-10-25 amendment), so it is gone before this brief starts.
- The ruling's gh/glab ceiling is 7 across desk-tools and statusgen. desktools-v2/75 adds the
  combined ratchet at its pickup count (36 at `d6a2ff244`, 53 with the 17 skill shell launches
  under `plugins/assay/`); statusgen's 30 reach 0 through forge-neutral/18 and /35, both before
  this brief.
- statusgen links `forgeread`'s root package only; forge-neutral/18 row 21 and desktools-v2/08
  row 10 hold the import graph.

design-fit:
  owner: statusgen forge adapter layer (consumer of the shared forge library)
  contract: S-identity (statusgen consumes the library's role-and-repo client; it holds no credential of its own)
  retires: [deskread process reader in statusgen, statusgen environment-token reads, the combined gh/glab ratchet's pickup ceiling (lowered to 7)]
  weight: "verbs 0, flags 0; one reader implementation replaced; one ratchet ceiling lowered"
  why-add: "Carries point 2 of the ruling with one swap behind an existing seam."

## Read first

`docs/streams/decisions/DR-forge-library.md`, desktools-v2/75, forge-neutral/18's 2026-10-25
amendment and rows 21 to 23, and desktools-v2/08.

## Human decision

Options:
1. **Accept the swap** — statusgen's online reader is the library client and statusgen holds no
   other credential path.
2. **Hold** — name a site that still reads a token or a package that links the library outside
   the adapter layer.

Default if no answer: none — blocks until answered.

## Ground rules

Feature branch and draft PR only; fake providers only. Touch no call site's mappings: only the
reader behind the seam changes. Leave `decisionruling.go` to forge-neutral/35. The offline
default stays: with no `--forge`, no client is built.

## Interface contract

statusgen builds one library client per role and repository from its adapter layer when
`--forge` is set, and passes it behind `forgeReader`. The adapter layer is statusgen's root
package (`github.com/medici-finance/assay/statusgen`, where `forgeread.go` lives); no other
statusgen package imports the library, directly or through a dependency (row 7). No statusgen code reads a forge-token environment variable or runs
`gh auth token`.

## Task

1. Add a library-backed `forgeReader` and select it under `--forge`; remove the `deskread`
   process reader.
2. Move `claimdecay_github.go`, `claimdecay_gitlab.go`, `doratiming.go` and `ghfetch.go` onto
   the library client, deleting their environment-token reads. The claim-decay `gh` fallback is
   already gone (forge-neutral/18); row 3 holds it gone.
3. Re-point desktools-v2/08 rows 4, 6 and 7 from a stub `deskread` to the library over a fake
   provider, and widen its row 10 allowed set to the library's caller package; update that
   brief's version.
4. Lower the combined gh/glab ratchet ceiling that desktools-v2/75 added from its pickup value
   to 7, the ruling's value, now that statusgen counts 0. The surface counted is the one /75's
   human gate settled. Row 8 takes it as a parameter, `SKILL_SCRIPTS`: `excluded` counts
   desk-tools and statusgen non-test code; `included` also counts the skill shell launches under
   `plugins/assay/`. If the settled surface counts more than 7, stop: the row cannot pass, and
   the reduction is named as a hold at this brief's gate rather than met by raising the ceiling.
5. Update the library-first table, `forgeread/README.md` and the fragment.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -timeout 300s -v -run '^TestLibraryReaderParity$' . > "$out" && grep -F -- '--- PASS: TestLibraryReaderParity ' "$out")` | exit 0; named PASS; over the forgeread parity fixtures the library reader returns the same three-state results the `deskread` reader did; one planted partial-as-complete fails |
| 2 | check +dereference | `test -d statusgen && { grep -rnE --include='*.go' --exclude='*_test.go' '"(GH_TOKEN\|GITHUB_TOKEN\|GH_ENTERPRISE_TOKEN\|GITHUB_ENTERPRISE_TOKEN\|GITLAB_TOKEN\|STATUSGEN_GITLAB_TOKEN\|CI_JOB_TOKEN)"' statusgen \|\| [ $? -eq 1 ]; } \| { grep -v -E '^[^:]+:[0-9]+:[[:space:]]*//' \|\| [ $? -eq 1 ]; } \| wc -l` | output `0` once forge-neutral/35 has landed; forge-neutral/18 row 22's ratchet at its end state |
| 3 | check:ci +flow +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -timeout 300s -v -run '^TestNoGhAuthTokenFallback$' . > "$out" && grep -F -- '--- PASS: TestNoGhAuthTokenFallback ' "$out")` | exit 0; named PASS; with a `gh` stub first on `PATH` that records any call and no minted token, every online site refuses and the stub records no call; planting the old fallback fails. A regression hold: forge-neutral/18 deletes the claim-decay fallback, so this row proves it stays gone with the library behind the seam |
| 4 | check +dereference | `(cd statusgen && GOWORK=off go list -deps ./evaluation/ \| { grep -E -e '/forgeclient(/.*)?$' -e '/forgeread(/.*)?$' \|\| [ $? -eq 1 ]; } \| wc -l)` | output `0`; the pure evaluation package links neither the library nor forgeread |
| 5 | check:ci +flow | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -timeout 300s -v -run '^TestMovedReadsUseReaderSeamOnly$' ./... > "$out" && grep -F -- '--- PASS: TestMovedReadsUseReaderSeamOnly ' "$out")` | exit 0; named PASS; forge-neutral/18 row 23 still holds with the library behind the seam |
| 6 | check +dereference | `statusgen --root . --consumers --brief desktools-v2/77 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | pre-merge, on the implementation branch: exit 0; the declared routing corroborates the actual diff; out-of-scope judgments remain UNCHECKED and reviewer-owned; a could-not-check is never recorded as a pass |
| 7 | check +dereference +mutation | `(cd statusgen && GOWORK=off go list -f '{{.ImportPath}}{{range .Deps}} {{.}}{{end}}' ./... \| { grep -E -e '/forgeclient$' -e '/forgeclient[/ ]' \|\| [ $? -eq 1 ]; } \| cut -d' ' -f1)` | output is exactly one line, `github.com/medici-finance/assay/statusgen`: of every statusgen package, only the root package (the forge adapter layer) depends on the library, directly or transitively. If /75's gate settled a different module path, substitute it in the pattern. A planted import of the library in `statusgen/workref` (or any other subpackage) adds a line and fails the row; an empty output means the swap did not happen and also fails |
| 8 | check +mutation | `sh tools/desk/scripts/forge-ban.sh` with `SKILL_SCRIPTS` set to the surface /75's gate settled (`excluded` or `included`) | exit 0 on the clean tree; the combined gh/glab line /75 row 6 added prints a count of at most 7 over that surface, and the ratchet's recorded ceiling is 7. A planted eighth launch site (`exec.Command("gh", …)` in a new non-test desk-tools file) makes the script exit non-zero |

## Pre-mortem and detection

- The library reader maps a partial read as complete: row 1.
- An old environment-token read survives the swap: row 2.
- The `gh auth token` fallback survives: row 3.
- The evaluation package starts linking the library: row 4.
- A call site bypasses the seam to use the client directly: row 5.
- A statusgen package other than the adapter layer links the library: row 7.
- The ruled ceiling of 7 is never asserted, so the ratchet stays at its pickup count: row 8.

## Evidence

<!-- Independent verifier records subject revision, environment, command, exit, named result and date. No implementation is claimed by this authoring change. -->

## Review

Gate: human. Review answers both: (1) is the library client the only credential path in the
statusgen process? (2) do rows 2 to 4 and 7 prove that with the reader seam bypassed? Row 8 asserts the ruled
ceiling of 7 over the surface /75's gate settled.
