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
version: 3
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
  - "statusgen/claimdecay_github.go, statusgen/claimdecay_gitlab.go, statusgen/doratiming.go, statusgen/ghfetch.go: follow-up desktools-v2/77 (environment-token reads and the four `api.github.com` host literals retire; the claim-decay `gh auth token` fallback is already gone, retired by forge-neutral/18, and row 3 holds it gone)"
  - "statusgen/forge.go: follow-up desktools-v2/77 (forge classification resolves through the library's forge resolver; statusgen's own `origin` read in `remoteOriginURL` retires, Task 6)"
  - "statusgen/decisionruling.go: follow-up forge-neutral/35 (its token reads retire there)"
  - "statusgen/go.mod: follow-up desktools-v2/77 (requires the library module by require plus relative replace)"
  - "statusgen/evaluation/: out-of-scope (stays pure; row 4 proves it links no forge library)"
  - "docs/streams/desktools-v2/brief-08-hold-statusgen-at-zero.md rows 4, 6, 7 and 10: follow-up desktools-v2/77 (rows 4, 6 and 7 re-pointed from a stub deskread to the library over a fake provider; row 10's allowed set widened to the library's caller package)"
  - "tools/desk/scripts/forge-ban.sh, tools/desk/internal/forgeban/allowlist.go: follow-up desktools-v2/77 (the combined gh/glab ratchet desktools-v2/75 adds is lowered to its clean count, which the ruling caps at 7; and the script's statusgen gate, which desktools-v2/08 limits to class a, widens to the whole statusgen total plus a statusgen API-base count, Task 7)"
  - ".github/workflows/forge-surface-control.yml: follow-up desktools-v2/77 (the step desktools-v2/08 makes a gate for the class a column gates the widened statusgen condition, Task 7)"
  - "docs/library-first.md, forgeread/README.md: follow-up desktools-v2/77"
---

# Brief 77 — Retire the deskread bridge for statusgen's forge calls

## Context

files:
- `statusgen/forgeread.go` and its tests, `statusgen/go.mod`
- `statusgen/claimdecay_github.go`, `statusgen/claimdecay_gitlab.go`, `statusgen/doratiming.go`,
  `statusgen/ghfetch.go`, `statusgen/forge.go`
- `statusgen/forgeclient_nogh_test.go` (planned), `statusgen/forge_offline_test.go` (planned)
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
  under `plugins/assay/`); statusgen's 30 gh launches reach 0 through forge-neutral/18 and /35,
  both before this brief.
- `forge-ban.sh` also counts three non-launch classes in statusgen, summed into its
  `statusgen sites:` total (38 at `e07782de2`, unchanged at `8169a8281`: class a 30, b 1, c 3,
  d 4). Class c (3) is forge-neutral/18's (its row 24). Class b is `statusgen/forge.go:113`, the
  `origin` remote read in `remoteOriginURL` behind `detectForge` and `detectForgeForScaffold`.
  Class d is the API host in `claimdecay_github.go:116` and `:140` and `ghfetch.go:42` and `:47`.
  This brief owns classes b and d (Tasks 2 and 6); row 9 asserts the total at `0`, and Task 7
  turns that total into a failing gate (row 11 proves the gate exits non-zero).
- Class f (the GitLab API literal) is a separate advisory column that `forge-ban.sh` does not
  sum: 8 statusgen lines at `8169a8281`. One is a code site, the derived API base
  `api.base = "https://" + host + "/api/v4"` in `claimdecay_gitlab.go:129` (`resolveGitLabAPI`);
  the other seven are comments, a field comment and a `users.noreply.gitlab.com` address in a
  scaffold string. Task 2 deletes the base with the file's move onto the library; Task 7 counts
  quoted `/api/v4` literals as their own gated line, and row 10 asserts it at `0`.
- statusgen links `forgeread`'s root package only; forge-neutral/18 row 21 and desktools-v2/08
  row 10 hold the import graph.

design-fit:
  owner: statusgen forge adapter layer (consumer of the shared forge library)
  contract: S-identity (statusgen consumes the library's role-and-repo client; it holds no credential of its own)
  retires: [deskread process reader in statusgen, statusgen environment-token reads, statusgen's own forge host literals and origin-based forge classification, the combined gh/glab ratchet's pickup ceiling (lowered to its clean count, at most 7), the advisory status of statusgen's classes b to d in the forge-ban counter]
  weight: "verbs 0, flags 0; one reader implementation replaced; one ratchet ceiling lowered; one counter gate widened"
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
`gh auth token`, names a GitHub API host (row 9) or builds a GitLab API base from a
host (row 10), or reads the `origin` remote to pick a forge (row 9). Class f stays an advisory
column, so row 10 holds the one code site in it, not every `gitlab.com` mention. Resolving a forge for a checkout builds no client, requests no credential and opens no
connection (row 12).

## Task

1. Add a library-backed `forgeReader` and select it under `--forge`; remove the `deskread`
   process reader.
2. Move `claimdecay_github.go`, `claimdecay_gitlab.go`, `doratiming.go` and `ghfetch.go` onto
   the library client, deleting their environment-token reads and the four `api.github.com`
   literals (`claimdecay_github.go:116`, `:140`, `ghfetch.go:42`, `:47` at `e07782de2`) and the
   derived GitLab API base (`claimdecay_gitlab.go:129`, `resolveGitLabAPI`): the API host and
   base are the library's. The claim-decay `gh` fallback is already gone (forge-neutral/18);
   row 3 holds it gone.
3. Re-point desktools-v2/08 rows 4, 6 and 7 from a stub `deskread` to the library over a fake
   provider, and widen its row 10 allowed set to the library's caller package; update that
   brief's version.
4. Lower the combined gh/glab ratchet ceiling that desktools-v2/75 added from its pickup value
   to the clean count, now that statusgen's gh launches count 0. The ruling caps it at 7 and
   says the ratchet only goes down, so a clean count below 7 is the ceiling, not 7. The surface
   counted is the one /75's human gate settled, and /75 builds that answer into the ratchet:
   the script takes no surface input, and row 8 reads the combined line /75 row 6 added. At
   `e07782de2`, with statusgen at 0, the surface without the skill scripts counts 6 (desk-tools
   Go 5, cellctl shell 1) and 23 with the 17 skill shell launches under `plugins/assay/`. If the
   settled surface counts more than 7, stop: the row cannot pass, and the reduction is named as
   a hold at this brief's gate rather than met by raising the ceiling.
5. Update the library-first table, `forgeread/README.md` and the fragment.
6. Resolve statusgen's forge classification (`detectForge`, `detectForgeForScaffold` in
   `statusgen/forge.go`) through the library's forge resolver, the one /75 moves out of
   `tools/desk/internal/deskkit/forgeresolve.go`, asked about the tracking root, and delete statusgen's own
   `origin` read in `remoteOriginURL` (`forge.go:113`). The three-state answer and `init`'s two
   `forgeUnknown` cases keep their meaning, and resolving builds no client and reads no
   credential, so the offline default holds. The resolver /75 moves out has a pure entry that
   takes an origin URL and constructs nothing, and entries that resolve and then reach
   custody; statusgen must call the pure one, because its no-`--forge` callers
   (`init`'s scaffold, the verify runner's role lookup, the claim-decay reader choice) are
   offline. Row 12 holds that. If the resolver /75's gate settled cannot be asked about a given
   root, stop and name it as a hold at this brief's gate.
7. Make the zero durable. Row 9 is read once at landing; this task is what keeps it. Do it
   after Tasks 2 and 6, once row 9 reads `statusgen sites: 0` and row 10 would read
   `statusgen api base: 0`. Then change
   `tools/desk/scripts/forge-ban.sh` so it exits non-zero when the whole `STATUSGEN_TOTAL`
   (classes a to d) or the new statusgen API-base count is above zero. This widens the class a
   gate that desktools-v2/08 Task 2 adds, which gates that column only. The new
   `statusgen api base: N` line counts non-test statusgen lines carrying a quoted string that
   holds `/api/v4`; it is its own line because class f also counts comments and an e-mail
   address that are not forge API reads. Make the workflow step /08 makes a gate cover the
   widened condition. Update /08's Task 2 and consumers line to say /77 widens it (and its
   version). The gh/glab launch ratchet is separate and unchanged by this task: statusgen
   already counts 0 launches, so the combined line row 8 reads is N, the desk-side clean
   count, at most 7, and the new gate only adds a failing exit for statusgen. It never raises a
   ceiling. If row 9 does not read `0` after Tasks 2 and 6, stop: the gate would land red, and the
   missing site is named as a hold at this brief's gate.

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
| 8 | check +mutation | `sh tools/desk/scripts/forge-ban.sh` | exit 0 on the clean tree; the combined gh/glab line /75 row 6 added prints a count N of at most 7 over the surface /75's gate settled (built into the ratchet; the script takes no surface input), and the ratchet's recorded ceiling equals N. Planting one launch site (`exec.Command("gh", …)` in a new non-test desk-tools file) raises the count to N + 1, above the ceiling, and makes the script exit non-zero. A recorded ceiling above N fails the row, because that mutation would then pass |
| 9 | check +dereference | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && sh tools/desk/scripts/forge-ban.sh > "$o" && grep -x -F 'statusgen sites: 0' "$o")` | exit 0 and the line `statusgen sites: 0`: every class `forge-ban.sh` sums for statusgen is at `0`, a and c through forge-neutral/18 and /35, b and d through Tasks 6 and 2. Measured at `e07782de2` and at `8169a8281`: the line reads `statusgen sites: 38` (a 30, b 1, c 3, d 4), so the row is red there. A landing check: a planted `var _ = "https://api.github.com"` in a new non-test statusgen file prints `statusgen sites: 1` and fails the row. The row proves the count was `0` when read; it cannot prove the script would refuse a later site, which row 11 does |
| 10 | check +dereference +mutation | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && sh tools/desk/scripts/forge-ban.sh > "$o" && grep -x -F 'statusgen api base: 0' "$o")` | exit 0 and the line `statusgen api base: 0`: no non-test statusgen line carries a quoted string holding `/api/v4`. Before Task 7 the line does not exist, so the row is red; the equivalent count `grep -rEn '"[^"]*/api/v4' statusgen --include='*.go'` over non-test files is `1` at `8169a8281` (`claimdecay_gitlab.go:129`). A planted `var _ = "/api/v4"` in a new non-test statusgen file prints `statusgen api base: 1` and fails the row. A base assembled from separate string pieces is not seen; the library's client builds its own base, and row 2 and row 7 keep any other credential-bearing path out |
| 11 | check +flow +mutation | `sh -c 'b=statusgen/zz_banprobe.go; f=statusgen/zz_forgeprobe.go; g=statusgen/zz_gqlprobe.go; trap "rm -f $b $f $g" 0; sh tools/desk/scripts/forge-ban.sh >/dev/null 2>&1; r0=$?; printf "package main\nvar _ = \"https://api.github.com\"\n" > $b; sh tools/desk/scripts/forge-ban.sh >/dev/null 2>&1; r1=$?; rm -f $b; printf "package main\nvar _ = \"origin\"\n" > $f; sh tools/desk/scripts/forge-ban.sh >/dev/null 2>&1; r2=$?; rm -f $f; printf "package main\nvar _ = \"pullRequest(number:1)\"\n" > $g; sh tools/desk/scripts/forge-ban.sh >/dev/null 2>&1; r3=$?; rm -f $g; echo "clean-rc=$r0 class-d-rc=$r1 class-b-rc=$r2 class-c-rc=$r3"; test "$r0" -eq 0 && test "$r1" -ne 0 && test "$r2" -ne 0 && test "$r3" -ne 0'` | exit 0; prints `clean-rc=0` and a non-zero `rc` for each planted class (d: an API host literal; b: an `"origin"` string in a file whose name holds `forge`, the only place the class b matcher looks; c: a `pullRequest(` block). The exit-status leg for row 9: it fails if the script merely prints a count and exits 0, so row 9 cannot pass vacuously. **Fail-first**: run at `8169a8281`, before Task 7, it prints `clean-rc=0 class-d-rc=0 class-b-rc=0 class-c-rc=0` and exits 1. The clean-tree leg also fails while any statusgen site remains, so the row reads green only with row 9. The probe files are removed whatever the result. Class a is the same leg in desktools-v2/08 row 3 |
| 12 | check:ci +flow +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -timeout 300s -v -run '^TestForgeDetectionIsOffline$' . > "$out" && grep -F -- '--- PASS: TestForgeDetectionIsOffline ' "$out")` | exit 0; named PASS. `TestForgeDetectionIsOffline` in `statusgen/forge_offline_test.go` (planned) runs `detectForge`, `detectForgeForScaffold` and their three offline callers (`runInit`, `forgeRoleRunner`, `decayReader`) in temp checkouts whose origin names GitHub, GitLab, a host naming neither, and no remote at all, with no `--forge`. The harness gives the library a fake credential provider that fails the test on any credential request, and uses the network dial hook of forge-neutral/18 row 4 (`TestLintOfflineMakesNoNetworkCall`) that fails on any connection. The three-state answers match the pre-swap answers. **+mutation**: swapping Task 6's call for a resolver entry that takes a credential provider records a request and fails the test; so does an entry that dials. **Fail-first**: run against the custody-reaching entry first and quote the red line under `## Fail-first` in the PR |

## Pre-mortem and detection

- The library reader maps a partial read as complete: row 1.
- An old environment-token read survives the swap: row 2.
- The `gh auth token` fallback survives: row 3.
- The evaluation package starts linking the library: row 4.
- A call site bypasses the seam to use the client directly: row 5.
- A statusgen package other than the adapter layer links the library: row 7.
- The ruled ceiling is never asserted, so the ratchet stays at its pickup count, or is set
  above the clean count so one new launch passes: row 8.
- statusgen keeps a forge fact other than a launch (a GitHub API host, or an `origin` read that
  picks the forge) at landing: row 9. A GitLab API base survives: row 10.
- A later change re-adds one of those after landing, and the script only prints a count: Task 7
  makes it exit non-zero, and row 11 proves it does.
- Task 6 reaches the credential-taking resolver entry, so `init` or the verify runner asks for a
  credential or dials with no `--forge`: row 12.

## Evidence

<!-- Independent verifier records subject revision, environment, command, exit, named result and date. No implementation is claimed by this authoring change. -->

## Review

Gate: human. Review answers both: (1) is the library client the only credential path in the
statusgen process? (2) do rows 2 to 4 and 7 prove that with the reader seam bypassed? Row 8 asserts the ruled
ceiling (the clean count, at most 7) over the surface /75's gate settled; row 9 asserts
statusgen's forge-ban total at `0` and row 10 its API-base count; row 11 proves the script exits
non-zero when either returns (the durable gate, Task 7). Row 12 proves that forge detection
reaches no credential and no network.
