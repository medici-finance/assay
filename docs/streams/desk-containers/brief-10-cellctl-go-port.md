---
brief: assay:assay:desk-containers:10
title: "cellctl in Go: `tools/desk/cmd/cellctl`, bash kept as the oracle until parity"
wave: 1
depends: ["desk-containers/09"]
unblocks: ["desk-containers/11"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-16 by the-desk dispatch (issue #1193)
sources:
  - "#1193 — 'Why a Go port rather than more bash': ~2,100 lines of bash already running as three divergent copies on one laptop plus a fourth implementation in a fourth language; `deskkit` already carries the forge resolver, role-token custody, roster parsing and the writeguard that the bash re-implements slices of; the windows-port stream needs a native-Windows cellctl that bash cannot reach"
  - "#1193 — the operator's own ruling comment, `ratified - go port`, 2026-09-16 15:00Z, recorded on the issue: the Go port proceeds. That comment is the decision that opens this brief; the human gate below is the cutover sign-off, not a re-ask of the port itself"
  - "tools/cellctl/cellctl — 2,179 lines at 872ac03e; verbs `ls check deskd desk up down new set` (dispatch table :2168-2176); kinds `k8s|house|container` (:224-236) plus `scrubbed` after desk-containers/09; harnesses `claude|codex` (:264-267); cockpits `auto|tmux|herdr|orca` (`resolve_cockpit`, :376-405); `DRY_RUN=1` plan output on `desk` (:1311-1315), `up`, and `container_run` (:297-300)"
  - "tools/cellctl/cellctl:173-179 — `DESK_TOOLS_BIN`, `REAL_CONFIG_HOME`, `HOUSE_VERBS`: the three host-layout constants the port carries over unchanged"
  - "tools/cellctl/cellctl:181-198 — `CELLCTL_VERSION=\"dev\"` and the `--version` contract; the Go binary takes `-ldflags -X` like every other `tools/desk/cmd/*` program"
  - "tools/cellctl/tests/*.test.sh — fifteen bash suites at 872ac03e (cell-set, cockpit, container-cell, desk-worktree-merge, down, gitlab-fetch-cred, harness, herdr-orca-launch, house-cell, model-namespace, model-override, model-pin, provider, root-drift, version-stamp); each resolves the binary under test from `$HERE/../cellctl` — the port needs them to honour a `CELLCTL` override so the SAME suites run against both implementations"
  - ".github/workflows/release.yml:1120-1152 — the desk-tools packaging step: every `cmd/*/` under tools/desk is cross-compiled per platform; cellctl is the one exception, `sed`-stamped and copied as a script into the tarball root; the port removes the exception"
  - "Makefile:35, :90 — `CELLCTL_SRC := tools/cellctl/cellctl` installed by `make desk-install`; the port re-points it at the built binary"
  - "docs/cellctl.md §Install — 'It is a shell script, not a Go build … It does not need a Go toolchain'; both sentences retire"
  - "tools/desk/internal/deskkit/forgeresolve.go:193 `ForgeKindForRepoRemote`, :425 `ResolveForge`, :476 `ForgeKindFor` — forge resolution the bash re-implements as `CELL_FORGE` + `FORGE_API_BASE` (:237-256)"
  - "tools/desk/internal/deskkit/roletoken.go:291 `RoleTokenForOwner`, :353 `RoleTokenForRepo` — role-token custody the bash's `deskd_mint_github` (:969) re-implements with curl + openssl"
  - "tools/desk/internal/deskkit/rosterconfig.go:91 `EnvAllowedRepos`, :823 `LoadConfig` — roster parsing; the bash shells out to `deskroster repos --scope scan` (check_house :925) to prove the roster parses"
  - "tools/desk/cmd/writeguard — the write guard; the bash has no equivalent and relies on the harness's own hook wiring"
  - "tools/desk/cmd/deskinstall/install.go:52-80 — the sha256 pin format `<asset> <tag> <sha256>` every shipped verb is checked against; the Go cellctl is pinned the same way, by being in the tarball"
  - "docs/streams/windows-port/README.md — brief 00 (unix/windows build-tag split) names the eight unix-only syscall sites in `internal/deskkit` that block `GOOS=windows`; a Go cellctl inherits that dependency and that stream's delivery"
  - "freshness-checked 2026-09-16 @ 872ac03e — `tools/desk/cmd/` holds 52 programs, none named cellctl; no parity harness exists; the release packaging exception for the shell script is live"
why: >-
  One laptop already runs three divergent copies of a 2,100-line shell launcher, and the first
  cell it could not express grew a fourth implementation in a fourth language. Every other desk
  verb is a Go binary in one tarball, pinned by sha256, built on the same `deskkit` the launcher
  re-implements slices of by hand. Porting the launcher onto that shape ends the drift class
  (one binary, one pin), reuses the custody and forge code instead of re-deriving it in bash,
  and makes a native-Windows launcher possible by construction — the shell script never can.
version: 1
id: ab4443e0-f127-41d0-9679-121a699dbdc0
gate-why: >-
  irreversible: yes — this replaces the operator's launcher. Once a release ships the Go binary
  under the tarball's `cellctl` entry, every pinned install that upgrades runs the port; the bash
  script is no longer what `cellctl` on PATH means, and a boot that goes wrong goes wrong on
  every cell at once. The port itself is decided (#1193, the operator's `ratified - go port`
  comment of 2026-09-16 15:00Z); what the human confirms at sign-off is the CUTOVER — that the
  parity matrix (row 3) and the fifteen re-pointed suites (row 4) were green at the SHA that
  shipped, and that the divergence-detection row (row 5) was exercised, not skipped.
exec-tier: strong
exec-tier-why: (a) the deskkit-reuse seams are named but not pre-specified line by line; (b) correctness is the cross-artifact equivalence of two implementations across kind × harness × cockpit; (c) a subtle env-composition or lock error survives unit tests and breaks a boot
domain: complicated
consumers:
  - "tools/desk/cmd/cellctl/: follow-up desk-containers/10 (this brief; new package, flips to fixed-here when it lands)"
  - "tools/cellctl/tests/parity.test.sh: follow-up desk-containers/10 (this brief; new file)"
  - "tools/cellctl/tests/*.test.sh: follow-up desk-containers/10 (this brief; each gains the `CELLCTL` override, one line)"
  - ".github/workflows/release.yml: follow-up desk-containers/10 (this brief; the packaging exception at :1133-1152 is removed — the implementer's App token has no `workflows` scope, so this hunk lands as a separate commit a human pushes, noted on the PR)"
  - "Makefile: follow-up desk-containers/10 (this brief; `CELLCTL_SRC` re-pointed)"
  - "docs/cellctl.md: follow-up desk-containers/10 (this brief; §Install rewritten, plan-grammar section cross-referenced)"
  - "tools/cellctl/cellctl: follow-up desk-containers/10 (this brief, NARROWLY — the bash stays in the tree as the oracle and its removal is a later brief this stream has not authored. Per the desk ruling recorded on the decision issue and the AMENDMENT under §The oracle, the ONLY edit this brief makes to it is dropping four withheld stream cites from its emitted scaffold text, shipped with the parity proof; nothing else in the script changes)"
  - "docs/streams/windows-port/: out-of-scope (a native-Windows cellctl is a consequence this brief NAMES for that stream and does not deliver; the build-tag split its brief 00 owns is the precondition)"
---

# Brief 10 — cellctl in Go: `tools/desk/cmd/cellctl`, bash kept as the oracle until parity

## Context

files:
- `tools/desk/cmd/cellctl/` (planned) — `main.go` (verb dispatch, `--version` via `-ldflags
  -X`), `cell.go` (cell.env load + kind/harness/forge validation), `env.go` (the scrubbed
  allowlist from desk-containers/09, stated once), `plan.go` (the `[dry-run]`/`[plan]`
  grammar), `check.go`, `desk.go`, `up.go`, `down.go`, `new.go`, `set.go`, `smoke.go`,
  `status.go`, `cockpit.go`, `lock.go` (mkdir lock; a build-tagged `flock` is NOT introduced),
  `deskd.go` (the App-JWT mint, ported ONTO `deskkit.RoleTokenForRepo` — never a re-derivation
  of the bash's hand-built RS256 flow; see the deskd-exclusion fact and row 15), any
  `CELLCTL_PARITY_MUTATE` code carrying `//go:build parity` so a release build omits it,
  `*_test.go` beside each.
- `tools/cellctl/tests/parity.test.sh` (planned) — the parity harness; §Parity below.
- `tools/cellctl/tests/*.test.sh` — each gains `CELLCTL="${CELLCTL:-$HERE/../cellctl}"` so the
  suite runs against either implementation.
- `.github/workflows/release.yml` — remove the packaging exception (:1133-1152): cellctl is now
  one of the `cmd/*/` builds. **This file needs the `workflows` scope the worker App lacks** —
  stage the hunk as its own commit, name it in the PR body as `BLOCKED-ON-HUMAN: push
  release.yml hunk`, and leave the rest of the PR reviewable without it.
- `Makefile` — `CELLCTL_SRC` → the built binary; `make desk-install` installs it like the
  others.
- `docs/cellctl.md` — §Install rewritten (Go binary, per-platform, `--version` from ldflags);
  a §*Parity with the shell oracle* subsection naming the harness and the matrix.
- `changelog/desk-containers-10-cellctl-go.md` (planned) — one `### Changed` bullet.

facts:
- **Surface to port (2026-09-16 @ 872ac03e):** verbs `ls check deskd desk up down new set`
  (`tools/cellctl/cellctl:2168-2176`) plus `smoke status` from desk-containers/09; kinds
  `k8s house container scrubbed`; harnesses `claude codex`; cockpits `auto tmux herdr orca`
  (`resolve_cockpit`, :376-405, with the herdr/orca fall-through rules docs §Cockpits
  states); forges `github gitlab` (:237-256). Every one of these is a parity-matrix axis
  EXCEPT `deskd` — see the deskd-exclusion fact below, which states why and how it is covered
  instead.
- **deskkit reuse is a requirement, not a preference** (#1193): forge resolution →
  `deskkit.ResolveForge` / `ForgeKindForRepoRemote` (forgeresolve.go:425 / :193) replaces the
  `CELL_FORGE`/`FORGE_API_BASE` derivation; role-token custody → `deskkit.RoleTokenForRepo`
  (roletoken.go:353) replaces `deskd_mint_github`'s curl+openssl JWT (:969-1011); roster →
  `deskkit.LoadConfig` + `EnvAllowedRepos` (rosterconfig.go:823 / :91) replaces the
  `deskroster repos --scope scan` shell-out AND gives `check_scrubbed`'s exact-scope row a typed
  read; the writeguard → the `tools/desk/cmd/writeguard` package's guard is what the Go
  `desk` verb consults before composing a launch into a worktree. A port that copies the bash
  logic for any of these four is a review finding, not a style nit.
- **The oracle.** `tools/cellctl/cellctl` stays in the tree, unmodified by this brief except
  where desk-containers/11 later touches it, and is what the parity harness runs the Go
  binary AGAINST. It is not shipped in the tarball once the Go binary is (release.yml hunk),
  and it is not deleted here — its removal is a separate brief this stream has not authored.
  AMENDMENT (desk ruling on #1193, 2026-09-20): the oracle may be edited only to remove text
  the leak gate refuses, and the edit ships with the parity proof. The rule exists to keep
  parity HONEST, not to preserve text the corpus gate classifies as withheld: a scaffold that
  writes private stream identifiers into every adopter's README and roster is a defect in the
  oracle, and both implementations are corrected together in one PR so the matrix stays whole.
- **Parity harness (`tools/cellctl/tests/parity.test.sh` (planned)):** for each cell fixture in the
  matrix below, runs `DRY_RUN=1 <impl> <verb> <args>` under both implementations with the
  same `CELLS_ROOT`, `DESK_TOOLS_BIN`, stub `PATH` and env, normalises (the implementation's
  own path replaced by `<cellctl>`; nothing else), and `diff`s stdout+stderr and the exit
  code. Any difference → exit 1 naming `<kind>/<harness>/<cockpit>/<verb>`. `new` has no dry
  run: parity there is the byte-diff of the `cell.env` and `home/` tree each writes into its
  own temp `CELLS_ROOT`, mode bits included. Matrix: kinds {k8s, house, container, scrubbed}
  × harness {claude, codex} × cockpit {tmux, herdr, orca} × verbs {check, desk, up, down, set,
  ls, smoke, status} plus `new` per kind × forge {github, gitlab}. Cockpit fixtures use stub
  `herdr`/`orca` binaries answering `--help` the way `resolve_cockpit`'s `help_has` probes
  (:366-374). The harness honours `PARITY_ONLY=<kind>/<harness>/<cockpit>/<verb>` for a
  single cell.
- **`deskd` is excluded from the parity matrix DELIBERATELY, and covered by a dedicated row
  instead — not an oversight.** The matrix verb axis is `{check, desk, up, down, set, ls, smoke,
  status}`; `deskd` is absent because it has no `DRY_RUN` plan path in the bash oracle (sources:
  dry-run output exists on `desk`, `up` and `container_run` only), so the plan-diff harness has
  nothing to compare — and giving `deskd` a dry-run path would mean editing the bash oracle,
  which this brief forbids (the oracle stays unmodified). `deskd` is also the single most
  credential-sensitive verb: it signs an RS256 App JWT with the PEM and exchanges it for per-org
  installation tokens it exports into the process env. Its port is therefore proven by **row
  15**, a source-level assertion that the Go `deskd` mints only through
  `deskkit.RoleTokenForRepo` (row 10) and contains no JWT-signing or `openssl` shellout of its
  own — i.e. it does not re-derive the bash `deskd_mint_github` flow. A live-mint behavioural
  row is out of scope for an offline authoring PR and named as the online-lane hand-off in the
  DoD, not stretched to fit an offline stub here.
- **Divergence detection must itself be proven** (row 5): the Go binary honours an
  undocumented, test-only `CELLCTL_PARITY_MUTATE=<key>` that drops the named `[plan] env`
  line; the harness run under it MUST go red naming the cell. This is the negative control on
  the oracle diff — without it a harness that diffs nothing (a normalisation bug, an empty
  matrix) is a green lamp wired to nothing. The variable is read only when the binary was
  built with the `parity` build tag, so a release build cannot carry it — and that claim is
  itself PROVEN, not merely asserted, by two rows a fail-open guard demands: **row 13**
  (behavioural) builds the binary with NO `-tags parity` and shows `CELLCTL_PARITY_MUTATE` is
  inert there — the mutated and unmutated dry-run plans are byte-identical AND the
  `KUBECONFIG=/dev/null` isolation line the mutation would have dropped is still present; **row
  14** (source) shows every file naming `CELLCTL_PARITY_MUTATE` carries `//go:build parity`, so
  a tagless compile contains none of that code. Row 5 proves the mutation WORKS (a `parity`
  build); rows 13-14 prove it is absent from what ships. (Row 5 deliberately drops
  `KUBECONFIG=/dev/null`, the cluster-isolation control, as its canary line precisely because
  that is the most damaging line to lose silently — which is why row 13 re-checks that exact
  line is intact in a release build.)
- **Fifteen suites re-pointed (row 4):** `CELLCTL=<go binary> bash tools/cellctl/tests/
  <suite>.test.sh` for every suite; these assert BEHAVIOUR (files written, env recorded by
  stubs, exit codes), so they fail for different reasons than the parity diff and are the
  second independent layer.
- **Pinning:** the binary is one more `cmd/*/` build in the desk-tools tarball, so it is
  covered by the umbrella `checksums.txt` line consumers pin in `.assay-versions`
  (deskinstall/install.go:52-80, `<asset> <tag> <sha256>`). No new pin line; the `cellctl`
  filename in the tarball is unchanged, so `deskinstall` and docs §Install's from-tarball line
  keep working.
- **Native Windows is a NAMED consequence, not a deliverable.** `deskkit` does not compile
  for `GOOS=windows` at 872ac03e (windows-port/00: eight unix-only syscall sites); this
  brief's `lock.go` adds none (mkdir lock, `os.Mkdir` is portable) and its process handling
  goes through `os/exec` only. The windows-port stream owns building and proving a Windows
  cellctl; this brief's DoD carries one line stating that hand-off and nothing more.
- **The plan grammar is frozen by desk-containers/09** (`[plan] env|argv|cwd|lock`, sorted
  KEYs) — the port emits it byte-identically; a change to the grammar is a change to BOTH
  implementations in one PR or it is a parity failure.
- **Effort L, deliberately not split:** a parity harness over a partial port proves nothing
  (every missing verb is a matrix hole a reviewer cannot tell from a divergence). The unit is
  whole because the proof is whole; the Task's ordering keeps every step reviewable.

single-point-of-failure: the parity harness's dry-run diff (row 3) — behind it: the fifteen
behavioural suites re-pointed at the Go binary (row 4; fail in a different component, on
recorded stub behaviour, not on a textual diff), the divergence-detection negative control (row
5; proves the diff can go red at all), and the human cutover gate (the tarball entry does not
flip until a human reads rows 3-5 at the shipped SHA). The `deskd` credential-mint path sits
OUTSIDE this matrix by construction (no dry-run plan to diff); its single control is row 15's
source assertion that the mint runs only through `deskkit.RoleTokenForRepo` with no self-baked
JWT, plus the online-lane live-mint check named in the DoD — the fail-open guard
(`CELLCTL_PARITY_MUTATE`) is held closed in a release build by rows 13-14.

## Human decision
The shell launcher every cell on the operator's machines boots through is being replaced by a
Go program that ships under the same name in the same release tarball. The port itself is
already ruled: proceed. What remains is when the tarball's `cellctl` entry switches from the
script to the binary, because once a release ships the binary, every install that upgrades
boots through it and a wrong port breaks every cell at once.

Options:
1. **Cut over in the release that lands this brief, gated on three green proofs** — the
   parity matrix over every kind, harness and cockpit shows no divergence; the fifteen
   existing behavioural suites pass against the binary; and a deliberately injected
   divergence is caught by the matrix. The script stays in the source tree as the oracle
   and is no longer shipped. Next: the cutover is signed at close; the following brief retires
   the out-of-tree bridge.
2. **Ship both for one release** — the binary as `cellctl`, the script as `cellctl-bash`, same
   three proofs required, with a documented one-line fallback. Next: a follow-up removes
   `cellctl-bash` one release later; costs one more release of two-copy drift, the very class
   this work retires.
3. **Do not cut over; keep the binary out of the tarball until a later ruling** — the port
   lands as source only. Next: nothing changes for any install; the drift and the bridge
   persist.

Default if no answer: none — blocks until answered (the cutover is the irreversible step).

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature branch +
  draft PR only. The `release.yml` hunk is staged as its own commit and named on the PR for a
  human to push — never worked around with another token.
- Stop at `implemented` — you do not set verified/done; the cutover is human-signed.
- NEVER commit `STATUS.md` on a branch (single writer = main's CI).
- Never run either implementation against a live harness, cluster or forge in a test or a
  Verify row; stubs, `DRY_RUN=1`, `KUBECONFIG=/dev/null`.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Add the `CELLCTL` override to every `tools/cellctl/tests/*.test.sh` (one line each;
   default unchanged) and confirm all fifteen still pass against the bash — the baseline.
2. Write `tools/cellctl/tests/parity.test.sh` (planned) FIRST, against the bash on both sides
   (`CELLCTL_A=… CELLCTL_B=…`); it must pass trivially and must go red when `CELLCTL_B` is a
   copy of the script with one `[plan]` line removed — the harness is proven before the port
   exists.
3. Port `cell.go` + `plan.go` + `env.go` and the `desk` verb's DRY_RUN path; run the parity
   harness with `PARITY_ONLY` narrowed to `desk`; widen verb by verb: `check`, `up`, `down`,
   `set`, `ls`, `smoke`, `status`, then `new`. Each widening is its own commit with the
   harness green at that width.
4. Replace the four re-implemented seams with deskkit calls (facts) — as the verb that owns
   each is ported, never as a later sweep.
5. Live (non-dry-run) paths: `exec` through `os/exec` with the composed env; tmux on the
   private socket; the mkdir lock; `deskd` via `deskkit.RoleTokenForRepo` for the GitHub mint.
6. Re-point the fifteen suites (`CELLCTL=<binary>`); fix behavioural divergences in the Go
   side only — a change to the bash oracle in this brief is a NEEDS_CONTEXT report, not an
   edit.
7. `release.yml` hunk (own commit, human push), `Makefile`, `docs/cellctl.md`, changelog.
8. Write the one-line Windows hand-off into the DoD evidence and the docs §Install (the
   binary is unix-only until windows-port delivers; the tarball's windows legs carry no
   cellctl until then, which is what they effectively carried before — a shell script no
   Windows shell runs).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go build ./cmd/cellctl && go vet ./cmd/cellctl` | exit 0. Red on the merge-base: `no Go files`/package not found | check:ci |
| 2 | `cd tools/desk && go test ./cmd/cellctl/... -count=1` | exit 0; ≥ 1 test per verb file listed in `files:` (unit level: cell.env parsing refusals, the allowlist, the plan encoder, the lock). Red on the merge-base: no such package | check:ci |
| 3 | `CELLCTL_A=tools/cellctl/cellctl CELLCTL_B=tools/desk/cellctl bash tools/cellctl/tests/parity.test.sh` | exit 0; prints one `ok` line per matrix cell and a final `parity: <n> cells, 0 divergent`; `<n>` ≥ 4 kinds × 2 harnesses × 3 cockpits × 8 verbs + `new` cells. Red on the merge-base: the harness does not exist (exit 127) | check:ci +flow |
| 4 | `for s in tools/cellctl/tests/*.test.sh; do case "$s" in *parity*) continue;; esac; CELLCTL=tools/desk/cellctl bash "$s" \|\| { echo "suite red: $s"; exit 1; }; done` | exit 0 — all fifteen behavioural suites pass against the Go binary. Red on the merge-base: the suites ignore `CELLCTL` and test the bash, so the row cannot fail there — that is why row 3 and row 5 exist beside it | check:ci +neighbour |
| 5 | `cd tools/desk && go build -tags parity -o /tmp/cellctl-parity ./cmd/cellctl && cd ../.. && CELLCTL_PARITY_MUTATE=KUBECONFIG CELLCTL_A=tools/cellctl/cellctl CELLCTL_B=/tmp/cellctl-parity bash tools/cellctl/tests/parity.test.sh; test $? -eq 1` | exit 0 — the harness goes RED (exit 1) and its last line names at least one `scrubbed/…/desk` cell as divergent when the Go side drops one plan line. This is the row that proves the oracle diff catches a divergence | check:ci +mutation |
| 6 | `cd tools/desk && go build -ldflags '-X main.cellctlVersion=v9.9.9-test' -o /tmp/cellctl-v ./cmd/cellctl && /tmp/cellctl-v --version` | exit 0; prints `v9.9.9-test` — the same `--version` contract the script carries (`tools/cellctl/cellctl:189-197`) | check +dereference |
| 7 | `grep -n 'tools/cellctl/cellctl' .github/workflows/release.yml Makefile; test $? -eq 1` | exit 0 — no packaging or install step names the script any more. Red on the merge-base: two hits (release.yml:1149, Makefile:35) | check:ci |
| 8 | `cd tools/desk && GOOS=windows GOARCH=amd64 go build -o /dev/null ./cmd/cellctl; echo "windows build rc=$?"` | prints `windows build rc=` followed by a number; the value is RECORDED in Evidence, not asserted — non-zero is the expected state until windows-port/00 lands, and this row exists so the hand-off is measured, not assumed | check |
| 9 | `grep -rn --exclude='*_test.go' 'syscall\.' tools/desk/cmd/cellctl/; test $? -eq 1` | exit 0 — the package adds no direct syscall use (the Windows consequence is not made worse here) | check:ci |
| 10 | `for f in ResolveForge RoleTokenForRepo LoadConfig; do grep -rq --exclude='*_test.go' "deskkit\.$f(" tools/desk/cmd/cellctl/ \|\| { echo "seam not called: $f"; exit 1; }; done` | exit 0 — each of the three deskkit seams is CALLED from a non-test file, not re-implemented (dereferencing: a port that copies the bash JWT/roster logic exits 1 at the first missing call) | check +dereference |
| 11 | `grep -c '^## .*Parity' docs/cellctl.md` | exit 0; prints `1` | check |
| 12 | `statusgen --consumers --root . --base $(git merge-base origin/main HEAD)` | exit 0 | check:ci |
| 13 | `d=$(mktemp -d); mkdir -p "$d/s/home/.config/assay"; printf 'CELL_KIND=scrubbed\nCELL_REPO=%s\nCELL_REPO_SLUG=example-org/example-repo\nCELL_HARNESS=codex\n' "$PWD" > "$d/s/cell.env"; ( cd tools/desk && go build -o "$d/cellctl-rel" ./cmd/cellctl ); a=$(CELLS_ROOT="$d" DRY_RUN=1 "$d/cellctl-rel" desk s the-desk 2>&1); b=$(CELLS_ROOT="$d" CELLCTL_PARITY_MUTATE=KUBECONFIG DRY_RUN=1 "$d/cellctl-rel" desk s the-desk 2>&1); test "$a" = "$b" && case "$a" in *KUBECONFIG=/dev/null*) true;; *) false;; esac` | exit 0 — a binary built with NO `-tags parity` IGNORES `CELLCTL_PARITY_MUTATE` entirely (mutated and unmutated dry-run plans byte-identical) and still emits the `KUBECONFIG=/dev/null` isolation line the mutation would have dropped. Proves the fail-open guard is inert in what ships. Red on the merge-base: the package does not build (non-zero) | check:ci +mutation |
| 14 | `for f in $(grep -rl 'CELLCTL_PARITY_MUTATE' tools/desk/cmd/cellctl/); do grep -q '//go:build parity' "$f" \|\| { echo "unguarded: $f"; exit 1; }; done` | exit 0 — every source file naming `CELLCTL_PARITY_MUTATE` carries the `//go:build parity` constraint, so a release compile (no tag) contains none of that code. Red on the merge-base: the package does not exist, the `for` iterates nothing, exit 0 — so this row is paired with row 13, which fails to build on the merge-base | check:ci +dereference |
| 15 | `! grep -rnE --exclude='*_test.go' -e 'crypto/rsa' -e 'crypto/x509' -e '[Jj][Ww][Tt]' -e 'openssl' tools/desk/cmd/cellctl/` | exit 0 — no file in the Go `cellctl` package signs an App JWT or shells to `openssl`; the `deskd` mint runs only through `deskkit.RoleTokenForRepo` (row 10), never a re-derivation of the bash `deskd_mint_github` RS256 flow. Red on the merge-base: the package does not exist, grep matches nothing, `!` makes it exit 0 — paired with row 10, which fails on the merge-base (no such package) | check:ci +dereference |

## Definition of Done
- Verify rows green, recorded in Evidence by a non-implementer; row 8's value recorded.
- Parity matrix green across every kind × harness × cockpit × verb, and `new` per kind ×
  forge; the bash oracle untouched by this brief.
- The four deskkit seams called (row 10); no bash logic for JWT minting, forge derivation,
  roster parsing or write guarding survives in the Go package.
- `deskd` mint proven at the source level (row 15) offline; the live-mint behavioural check
  (a real stub-forge token exchange) is named here as the ONLINE-LANE hand-off, since this repo
  runs no live mint offline — it is not stretched into a fake offline row.
- The `CELLCTL_PARITY_MUTATE` fail-open guard is proven inert in a release build (rows 13-14),
  not merely asserted.
- `release.yml` hunk pushed by a human (named on the PR until it is), the script no longer
  in the tarball, the binary under the same `cellctl` filename.
- Windows hand-off line present in `docs/cellctl.md` §Install and in Evidence (row 8).
- `docs/cellctl.md` regenerated: §Install, §Parity (docs-regen item).

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: human (irreversible: yes — see `gate-why`; the port is ruled on #1193, the cutover is what
is signed). The reviewer of a core-tool replacement answers BOTH, in the verdict:
1. **What single control stands between a wrong port and a broken boot, and is it
   acceptable?** — the parity harness's dry-run diff (row 3), backed by the behavioural suites
   (row 4) and the negative control (row 5). Acceptable only if row 5 was RUN at the reviewed
   head (its Evidence row carries the red harness output), not merely present in the table.
2. **Which Verify row proves the oracle diff catches a divergence?** — row 5, and only row 5.
   If the Evidence for row 5 shows a green harness, the answer is "none" and the verdict is
   a bounce.
The reviewer also confirms row 10's count is made of calls, not comments, by reading the
three call sites.
