---
brief: desk-tools/07
title: "`clusterguard` — exec-boundary shim for cluster CLIs, operator opt-in"
why: >-
  A permission rule that matches on command TEXT cannot see a cluster call made from inside a
  committed script, and a written policy only covers the paths somebody thought to write a line
  about. The premise is not that sessions ignore prose — an explicit refusal (a non-zero exit, a
  blocked write, a body-check) is respected. It is that uncovered surface has nothing behind it
  and leaves no trace: the only way anyone learns a probe happened is if the caller mentions it.
  A shim at the exec boundary survives both problems. The cluster CLIs on a session's PATH
  resolve to one guard that refuses by default, records the attempt, and passes through only in
  a shell that deliberately exported the operator opt-in. "Desk sessions are offline" stops
  being an instruction and becomes a mechanical property with a refusal log behind it — the
  detection surface a text-matching rule cannot provide at all.
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-02 by a re-home session; the control was tracked on a private board before the
  disposition review found nothing house-specific in it
sources:
  - "The guard family this joins and its shared contract: `tools/desk/README.md` § Tool reference and § Guard, the two meters, and repo scope — `writeguard`, `deskpushguard`, `desksourceguard`, `repohardenguard`."
  - "The exit-code contract every desk tool maps its refusals onto: `tools/desk/internal/deskkit/exitcodes.go` (0 ok · 3 disabled · 5 refused · 6 unverifiable) and the `Guard()` stop-flag precedence in `tools/desk/internal/deskkit/killswitch.go`."
  - "The opt-in-token shape this copies: the writeguard shared-checkout claim in `tools/desk/cmd/writeguard/main.go` — a token a guarded session must never be able to export for itself."
  - "The roster's unknown-`ASSAY_`-key refusal that the opt-in variable has to be declared against: `tools/desk/internal/deskkit/rosterconfig.go` § parseConfig key recognition."
  - "Brief and Verify shape: `spec/brief-v1.md`; status semantics: `spec/lifecycle-v1.md`."
  - "Install surface for the shim directory: `docs/adopting-assay.md` § PRIMITIVE: install-desk-tools."
exec-tier: strong
exec-tier-why: "safety plumbing. A subtle pass-through bug — the guard resolving itself, the
  invoked name lost, the opt-in check inverted, an unclassified verb defaulting to allowed —
  survives every happy-path test and leaves the control looking installed while it does nothing."
---

# Brief 07 — `clusterguard`: exec-boundary shim for cluster CLIs

## Dependencies
None. The guard family, the exit-code contract and the stop-flag precedence this builds on are
all present in `tools/desk` today, so no typed `depends:` edge remains.

## Context

single-point-of-failure: **PATH resolution** — a session that rebuilds its environment without
the shim directory, or a call made by absolute path, never reaches this layer at all. Behind it
must stand two independent layers, and this brief ships neither: a **credential quarantine**
(a probe that evades the shim still finds nothing to authenticate with) and, durably, **sandboxed
execution** (a session with no cluster reachability at the network layer). Independence: the shim
fails only on PATH manipulation, a quarantine only on a reconstructed credential, a sandbox only
on a container escape — different reasons, different components. This layer is worth shipping
because it is the one of the three that produces a **log**, not because it is sufficient.

risk note — this brief declares `tools/desk/internal/deskkit/rosterconfig.go`, which is on the
security-path trigger list, while answering all four risk questions "no". The answers stand and
here is why, so a reviewer checks the reasoning rather than re-deriving it: the edit to that file
is **additive and inert** — one exported constant naming the opt-in variable, and one entry adding
that name to the recognised-key set. It applies no value, reads no environment, and changes no
trust decision; its only effect is to stop a mis-placed opt-in in `roster.env` from refusing the
whole roster, which strictly WIDENS the set of roster files that load. Nothing in it can narrow
trust, and no reviewer-visible behaviour of any other tool changes. If a reviewer disagrees that
an additive recognised-key entry is non-sensitive, the correction is to flip `sensitive-data` to
yes and take the human gate — not to leave the answers as-is with the disagreement unrecorded.

files:
- `tools/desk/cmd/clusterguard/` (NEW — a Go command in the existing `tools/desk` module)
- `tools/desk/internal/deskkit/rosterconfig.go` (declare + recognise the opt-in variable name)
- `tools/desk/internal/forgeban/allowlist.go` (register the one pass-through exec site)
- `tools/desk/README.md` (contract, verdict table, stated limits)
- `docs/adopting-assay.md` (§ install-desk-tools — installing the shim directory, and the opt-in)

facts:
- guard-family precedent: `tools/desk/cmd/{writeguard,deskpushguard,desksourceguard,repohardenguard}`;
  binaries are built by `make desk-build` and installed by the HUMAN-ONLY `make desk-install`
  (the sudo prompt IS the gate) — agents run `desk-build` and `desk-test` only.
- exit-code contract: **3** disabled · **5** refused · **6** unverifiable. **Exit 5 is never a
  fallback trigger** — a refusal must not route the caller onto a rawer path, and the refusal
  message says so in as many words.
- mechanism: a shim **directory** (installed alongside the desk binaries) holding `kubectl`,
  `flux`, `helm`, `talosctl`, `k9s` as symlinks to the one `clusterguard` binary. Sessions
  prepend that directory to PATH. The guard reads which CLI it is from `argv[0]`, and on
  pass-through resolves the real binary by scanning PATH for the first executable of that name
  that is **not itself** — compared with `os.SameFile` against the running executable, not by
  path text, because the shim is reached through a symlink and a deployment may have the shim
  directory on PATH more than once. Self-resolution is the classic way this control becomes a
  fork bomb.
- **two opt-in levels, not one.** Looking at a cluster and changing one are different acts, and
  an operator who wants the first should not have to grant themselves the second in the same
  export:

  | `ASSAY_ALLOW_CLUSTER` | verdict |
  |---|---|
  | absent | every shimmed CLI refused (exit 5) — the default posture |
  | `1` (or `ro` / `read-only`) | read-only verbs pass through; mutating verbs refused (exit 5) |
  | `mutate` (or `rw` / `write`) | every verb passes through |
  | any other value | refused (exit 5) — a typo in a safety opt-in is never read as yes or no |

- read-only classification is an **allowlist per CLI**, which is the fail-closed direction: a
  verb nobody classified is MUTATING, so a subcommand added upstream tomorrow is refused rather
  than waved through on the day it appears. `k9s` carries an EMPTY allowlist on purpose — it is
  an interactive TUI whose mutating operations are reachable from inside the session, so nothing
  in its argv can establish that a call is read-only.
- the verb scan skips flags, and skips the value of a global flag that consumes one (`-n ns get`
  classifies on `get`, not on `ns`). The value-flag table can only remove FALSE REFUSALS: an
  unrecognised token still lands on the mutating side.
- token discipline: `ASSAY_ALLOW_CLUSTER` is a per-SHELL export an operator makes deliberately —
  the same shape as writeguard's shared-checkout claim, a token the guarded population must never
  export for itself. It is **declared in `deskkit`** for one reason only: it sits in the `ASSAY_`
  namespace, and an unrecognised `ASSAY_` key in `roster.env` refuses the WHOLE configuration, so
  an operator who records the opt-in in the shared roster file would otherwise take every desk
  tool's trust roster down at once. Recognised is not applied: putting it in `roster.env` still
  does not grant it.
- **stop flags only tighten.** `deskkit.Guard()` runs before any verdict, and an armed flag is
  itself a refusal (exit 3). This guard deliberately does not participate in the kill switch the
  way an acting tool does: a refusal-guard that stopped intercepting when disabled would fail
  OPEN, handing every halted session the cluster CLIs back — the exact inversion of what arming a
  kill switch is for. The uninstall path is removing the shim directory from PATH, and the README
  records that decision rather than leaving the flag silently absent.
- **the log is the point.** One append-only line per invocation — timestamp, CLI, verdict, verb,
  read-only flag, cwd, argv — under the config home. Credential-bearing flag VALUES
  (`--token`, `--password`, …) are redacted in both the `--flag value` and `--flag=value` forms:
  the log is written on every call and must not become a credential file. A logging failure never
  changes a verdict, but it is reported on stderr so an unrecorded surface is visible.
- pass-through **spawns** rather than replacing the process image. The desk-tools build targets
  Windows as well as unix, exec-replacement has no portable form, and nothing in the contract
  needs the process image to be the CLI's. Stdio is inherited directly, so an interactive TUI
  behaves exactly as it does unshimmed.
- **stated limits, named rather than summarised as "PATH order":** a session that constructs its
  own environment without the shim directory evades this layer; a CLI invoked by ABSOLUTE PATH
  never consults PATH and is never intercepted; and the guard keys on five CLI NAMES, so HTTP or
  SDK access, a credentialed tool integration, and `ssh`/`docker` hops are outside it entirely.
  This narrows one exec path. It is not a network boundary.
- tests use FIXTURE binaries only — two-line `/bin/sh` scripts written into a temp directory. No
  test and no Verify row contacts a live CLI, a live cluster, or the network.
- publication hygiene: no real cluster, context, node or host name appears in code, tests, README
  or log examples.

## Ground rules
- **Implemented entirely offline.** Every test and every Verify row runs against fixture
  binaries; no row may invoke a real cluster CLI against any context.
- Never push, never trigger workflows, never run a mutating infrastructure command.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Task

1. **Implement `tools/desk/cmd/clusterguard`** per the facts above: `argv[0]`-keyed dispatch;
   refuse by default (exit 5, a stderr line naming the guard, the CLI and the policy, plus a log
   line); the two opt-in levels with the per-CLI read-only allowlist; pass-through that resolves
   the next binary on PATH excluding itself, with argv and env unchanged; exit 6 when the opt-in
   is present but no real binary exists past the guard; exit 6 on a shim name the guard does not
   recognise; `deskkit.Guard()` first, able only to tighten.
2. **Declare the opt-in variable in `deskkit`** and add it to the roster's recognised-key set,
   with the reason written down: recognised so that recording it in `roster.env` cannot collapse
   the roster, never applied from there.
3. **Register the one pass-through exec site** in the forge-surface ledger
   (`tools/desk/internal/forgeban/allowlist.go`), stating what it launches and why no forge CLI
   can reach it.
4. **Tests** (fixture-only) covering: refusal without the opt-in (exit 5, message names the
   policy); the script-wrapped call refused identically; pass-through with the opt-in reaching a
   fixture with args intact; PATH resolution skipping every copy of the shim directory;
   `argv[0]` dispatch across all five names; the read-only/mutating table per CLI; the read-only
   tier refusing a mutating verb; an unrecognised opt-in value refused; the log carrying both
   verdicts; credential redaction; a missing real binary as a distinct exit 6; an armed stop flag
   refusing even with the opt-in present; and the ABSOLUTE-PATH BYPASS as a negative control.
5. **README contract section** in `tools/desk/README.md`: the verdict table, the token rule, the
   kill-switch decision with its fail-open rationale, the log format and the redaction, and every
   stated limit named individually.
6. **One paragraph in `docs/adopting-assay.md`** § install-desk-tools: installing the shim
   directory on PATH and where the opt-in belongs.
7. **Nothing else.** No hook changes, no new flags on other guards, no operator-shell edits
   anywhere (the export is documented, not applied).

## Verify

Every row is a `go test` invocation, a `gofmt` check, or a run of the BUILT binary under its own
name. That is deliberate and worth stating, because it is this brief's subject matter showing up
in its own verification: an agent verifier runs under command-text permission rules, and a Verify
row whose shell command spells `kubectl` is refused by those rules before it can run — even when
every binary it would reach is a fixture in a temp directory. Driving the same scenarios from Go,
where the CLI names are fixture data rather than command text, is what makes these rows runnable
by the verifier who has to run them. The behavioural coverage is identical: each test below
executes the built shim as a real subprocess reached through a symlink, which is the only way to
observe `argv[0]` keying and PATH resolution at all.

`-run` compiles an RE2 pattern, so each row names ONE test rather than an alternation, and rows
are chained with `&&`.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/desk && go build ./... && go vet ./...` | exit 0 |
| 2 | check:ci | `cd tools/desk && go test ./cmd/clusterguard/... -count=1` | exit 0 |
| 3 | check:ci | `cd tools/desk && go test ./cmd/clusterguard/ -run '^TestRefusesWithoutOptIn$' -count=1 -v` | exit 0 — a shimmed CLI with no opt-in exits 5, the stderr names `clusterguard`, `offline-by-default` and the opt-in variable, and the fixture never runs |
| 4 | check:ci | `cd tools/desk && go test ./cmd/clusterguard/ -run '^TestRefusesScriptWrappedCall$' -count=1` | exit 0 — the MUTATION row: the same call wrapped in a shell script is refused identically. This is what a command-text rule cannot catch and what the exec boundary catches for free |
| 5 | check:ci | `cd tools/desk && go test ./cmd/clusterguard/ -run '^TestPassesThroughWithOptIn$' -count=1 && go test ./cmd/clusterguard/ -run '^TestPassThroughSkipsTheShimDirectory$' -count=1` | exit 0 — pass-through reaches the fixture with args intact, and still does so with the shim directory listed TWICE on PATH ahead of it (the self-exec loop) |
| 6 | check:ci | `cd tools/desk && go test ./cmd/clusterguard/ -run '^TestSymlinkNameDispatch$' -count=1 -v` | exit 0, five subtests — one binary invoked under five names reaches the fixture of its OWN name |
| 7 | check:ci | `cd tools/desk && go test ./cmd/clusterguard/ -run '^TestVerbClassification$' -count=1 && go test ./cmd/clusterguard/ -run '^TestReadOnlyTierRefusesMutatingVerbs$' -count=1` | exit 0 — the per-CLI read-only table holds (including `k9s` having no read-only lane and `delete -f get` not reclassifying on an argument), and the read-only tier refuses a mutating verb with exit 5 without reaching the CLI |
| 8 | check:ci | `cd tools/desk && go test ./cmd/clusterguard/ -run '^TestUnrecognisedOptInValueRefuses$' -count=1 && go test ./cmd/clusterguard/ -run '^TestNoRealBinaryIsUnverifiable$' -count=1 && go test ./cmd/clusterguard/ -run '^TestUnknownInvocationNameIsUnverifiable$' -count=1` | exit 0 — the three fail-closed edges: a typo'd opt-in refuses (5), a missing binary is a DISTINCT 6, an unrecognised shim name is 6. None of them is 0 |
| 9 | check:ci | `cd tools/desk && go test ./cmd/clusterguard/ -run '^TestStopFlagRefusesEvenWithOptIn$' -count=1` | exit 0 — an armed kill switch exits 3 and the fixture does not run. The fail-OPEN inversion is the thing this row exists to make impossible |
| 10 | check:ci | `cd tools/desk && go test ./cmd/clusterguard/ -run '^TestAbsolutePathInvocationBypassesTheShim$' -count=1 && go test ./cmd/clusterguard/ -run '^TestLogRecordsBothVerdicts$' -count=1 && go test ./cmd/clusterguard/ -run '^TestLogRedactsCredentialArguments$' -count=1` | exit 0 — the NEGATIVE CONTROL (an absolute-path call reaches the CLI and the guard never sees it: the stated limit, proven rather than asserted), plus both verdicts on the log and credential values redacted off it |
| 11 | check:ci | `cd tools/desk && d=$(mktemp -d) && go build -o "$d/clusterguard" ./cmd/clusterguard && "$d/clusterguard" > "$d/usage.out" 2>&1; rc=$?; grep -q 'ASSAY_ALLOW_CLUSTER' "$d/usage.out"; g1=$?; grep -q 'talosctl' "$d/usage.out"; g2=$?; [ "$rc" -eq 0 ] && [ "$g1" -eq 0 ] && [ "$g2" -eq 0 ]` | exit 0 — the BUILT binary, run under its OWN name (`clusterguard`, not a `cg-`-prefixed basename), states its opt-in variable and prints the shimmed set. The one row that exercises the real artifact end to end without naming a cluster CLI on a command line |
| 12 | check:ci | `cd tools/desk && go test ./... -count=1` | exit 0 — the whole module, including the forge-surface ledger check that the new pass-through exec site is registered |
| 13 | check:ci | `gofmt -l tools/desk/cmd/clusterguard > /tmp/cg-fmt.out; test ! -s /tmp/cg-fmt.out` | exit 0 — no unformatted file |
| 14 | check:ci | `cd statusgen && go run . --root .. --lint; echo $?` | 0 |

Rows 3, 4, 7, 8 and 9 are the negative-path rows. They were written and run against a
do-nothing stub before the implementation existed: every one of them failed, which is what
establishes that a green result here measures the guard rather than the harness. Row 10's
absolute-path control is the exception and deliberately so — it PASSES against the stub, because
what it asserts is the ABSENCE of interception.

## Evidence
<!-- appended at implementation time: one witness row per Verify row —
     (command, exit code, output line(s), date, runner). -->

## Review

Gate: model (all four risk answers no). The reviewer records verdict + date in the stream README
table and answers, for a control whose whole value is that it fails closed:

1. **What single control stands between the fault and the damage, and is that acceptable?** Here
   it is PATH resolution. It is acceptable only as one layer of three: the brief ships neither the
   credential quarantine nor the sandbox that stand behind it, and says so. A reviewer who reads
   this brief as sufficient containment has read it wrong.
2. **Does any row prove the control catches the fault with the layer above it bypassed?** Rows 4
   (script-wrapped) and 5 (shim directory doubled on PATH) are that shape. Row 10's absolute-path
   control proves the complementary negative — where this layer does NOT reach — so that the limit
   is a tested property rather than a sentence in a README.

The reviewer also confirms: no test or Verify row can contact a live CLI or cluster; the README
records the kill-switch fail-open decision rather than silently omitting the flag; and the
read-only allowlist is genuinely fail-closed (an unclassified verb is mutating), since an
allowlist inverted into a denylist would make every new upstream subcommand a pass-through.
