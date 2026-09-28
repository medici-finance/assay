---
brief: assay:assay:forge-neutral:07
title: statusgen acting identity — Evidence-actor and verifyrun name the forge identity that acted
why: >-
  The Evidence-actor check is what stops an implementer attesting its own Evidence, and the
  verifyrun witness is what records who ran a Verify row. On the 2026-09-02 GitLab pilot both
  failed open: the check reported "0 row(s) are backed" for Evidence a distinct verifier
  account had genuinely committed, and the witness named a host-derived handle instead of the
  acting service account, so the witness table and the Evidence table disagreed by
  construction. A trust check that cannot recognise the acting identity is not a weaker check;
  it is an absent one wearing a green lamp.
wave: 3
depends: ["forge-neutral/01", "forge-neutral/02"]
unblocks: ["forge-neutral/08", "forge-neutral/10"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
authored: 2026-09-02 by forge-neutral authoring session
sources:
  - "docs/streams/forge-gitlab/pilot-report.md D-3 (Evidence-actor cannot recognise a GitLab verifier) and D-9 (the witness stamps a machine-derived runner, not the acting forge identity)"
  - "docs/streams/forge-neutral/brief-02-forge-qualified-identity.md — the forge-qualified roster grammar and the per-forge commit-address table this consumes"
  - "docs/streams/forge-neutral/brief-01-forge-resolution-contract.md — the resolution contract statusgen mirrors without importing deskkit"
  - "freshness-checked 2026-09-02 @ deae247 — evidenceactor.go:225-247 resolves the accepted verifier from `verifier=<slug>[:<id>]` and matches it against the GitHub noreply regex at :165; verifyrun.go:621-645 derives Runner from GITHUB_ACTIONS/GITHUB_ACTOR then git config, special-casing the GitHub noreply form; evidenceactor.go:128-131 records the check as dead in CI today"
exec-tier: strong
exec-tier-why: "this is the code that decides whether a verified row is believed; a subtle error (an actor comparison that matches too loosely, a could-not-check that reads as a pass) survives the check's own tests and is exactly the self-attestation the check exists to prevent (question c)."
gate-why: >-
  These two are the anti-self-attestation controls. Widening the accepted-actor comparison —
  or letting a forge the check does not understand resolve to "backed" instead of
  could-not-check — makes an implementer's own Evidence indistinguishable from a verifier's,
  which is the exact failure `F-verify-self-attest` was raised for. The human is confirming
  the per-forge actor-matching rule, that an unrecognised forge stays could-not-check rather
  than becoming a pass, and that the witness runner is derived from the acting forge identity
  rather than from anything the running session can set.
domain: complicated
consumers:
  - "statusgen/evidenceactor.go: fixed-here"
  - "statusgen/verifyrun.go: fixed-here"
  - "statusgen/rosterconfig.go: fixed-here (statusgen's own roster parser must accept the forge-qualified grammar forge-neutral/02 defines)"
  - "statusgen/autoflip.go: follow-up forge-neutral/08 (the auto-flip's reviewer-login composition is the same class of assumption, handled with the CI scaffold)"
  - "docs/streams/forge-neutral/identity.md: fixed-here (the per-forge commit-address table gains statusgen's two consumers)"
version: 1
id: 0128a67b-af4f-4d25-9dd2-1b53e603c99d
---

# Brief 07 — statusgen acting identity

## Context
files:
- `statusgen/evidenceactor.go` — the accepted-actor policy and the commit-address matcher.
- `statusgen/verifyrun.go` — `executingRunner` and the witness `Runner` cell.
- `statusgen/rosterconfig.go` — statusgen's own copy of the roster parser.
- `docs/streams/forge-neutral/identity.md` (planned) — created by `forge-neutral/02`; this
  brief adds statusgen's two consumers to its per-forge table.

single-point-of-failure: the accepted-actor comparison is the one control standing between a
self-attested Evidence row and a believed one. Two independent layers: the comparison itself
(id-pinned where the roster pins an id, in `evidenceactor.go`), and the witness row, which
records the acting identity from a different source at a different time — so an Evidence row
whose committer matches the roster but whose witness names a different actor is visibly
inconsistent even when the comparison passes. The pilot proved that pair matters: D-9's
disagreement between the two tables is precisely the signal a single control would not have
produced.

facts:
- The accepted verifier is resolved from `scanEffectiveConfig().RoleBots["verifier"]` with
  the numeric id pinned from `cfg.Bots[slug]` (`statusgen/evidenceactor.go:235,244`), and the
  entry grammar is stated as `verifier=<slug>[:<id>]` (`evidenceactor.go:24-25,238`).
- The id is the GitHub numeric USER id, matched against the noreply commit address by the
  regex `^(\d+)\+([^@]+)@users\.noreply\.github\.com$` (`statusgen/evidenceactor.go:165`,
  parsed at `:173-183`). An id-pinned entry wins over a login compare; an unpinned entry
  degrades to login-only (`evidenceactor.go:198-204`).
- Evidence ownership itself is read with `git blame --line-porcelain`
  (`statusgen/evidenceactor.go:443`) — already forge-neutral in mechanism. Only the identity
  comparison is GitHub-shaped.
- The could-not-check wrapper is *"could-not-check: Evidence-actor (desk-apps/07,
  F-verify-self-attest) did not run — %s. No `verified`/`done` row is reported clean or
  unbacked by this run."* (`statusgen/evidenceactor.go:527-529`), with two `Unavailable`
  reasons: an absent/invalid roster (`:229-233`) and no bound verifier (`:237-241`).
- The check is noted as dead in CI today: *"IT IS DEAD IN CI TODAY … every CI run reports
  could-not-check"* (`statusgen/evidenceactor.go:128-131`). Fixing that is out of scope here
  and stated so, but the brief must not make it worse.
- `verifyrun`'s witness row shape is `| # | Command | Result | Output | Date | Runner |`
  (`statusgen/verifyrun.go:155`), rendered at `:199`, with `Runner` set at `:794` from the
  value derived at `:1152`. `executingRunner` (`:621`) takes `GITHUB_ACTOR` under
  `GITHUB_ACTIONS` (`:622-626`), else `git config user.name`/`user.email` (`:627-628`),
  records a `[bot]` identity verbatim (`:629-637`), special-cases the GitHub noreply form
  (`:641-645`) and otherwise emits `human:<slug>` (`:654`).
- Supplying the runner on the command line is already a usage error
  (`statusgen/verifyrun.go:1079,1095`), and with no identity available the tool refuses
  rather than writing a witness with no runner (`:1154`). Both properties must survive.
- statusgen does not import `deskkit`; its network code is one client with
  `const githubAPIBase = "https://api.github.com"` (`statusgen/ghfetch.go:3,45`). This brief
  changes no network code — both surfaces here are local (git blame, git config, env).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Never make an unrecognised forge resolve to a PASS. The three-state rule is the point of
  this check: could-not-check is a legitimate answer, a false clean is not.
- Do not hand-correct a generated witness row anywhere in this work. A hand-written witness
  is exactly the manufactured evidence the witness exists to replace.

## Task
1. **Roster parity.** Teach `statusgen/rosterconfig.go` the forge-qualified grammar
   `forge-neutral/02` defines, with the same unqualified-means-`github` rule, so one roster
   file loads identically in both binaries.
2. **Per-forge actor matching.** `evidenceactor.go` resolves the accepted verifier's forge
   from its roster entry and matches the Evidence committer using that forge's commit-address
   form — the GitHub noreply regex as today, the GitLab service-account noreply form for a
   `gitlab:` entry. Id-pinned matching stays preferred; login-only degradation stays the
   fallback and stays recorded as weaker.
3. **Unknown forge is could-not-check.** An entry naming a forge this build does not
   understand, or an Evidence commit whose address matches no configured forge's form, yields
   the existing could-not-check wrapper with a message naming the forge — never `backed`,
   never `unbacked`. Extend the two `Unavailable` reasons with a third for this case.
4. **The witness names the acting forge identity.** `executingRunner` gains a first source
   ahead of the CI-env and git-config paths: the acting role identity resolved from the
   roster for the repo's forge, when one is bound. The CI-env and git-config paths stay as
   fallbacks in that order. The no-identity refusal and the forbidden-runner-flag refusal
   both stay exactly as they are — the runner must remain underivable from anything the
   session can simply set.
5. **Record the resolution source.** The witness carries which source produced the runner
   (forge identity / CI env / git config), so a reader can tell a stamped acting identity
   from a host-derived one. This is what makes the D-9 disagreement visible next time instead
   of requiring a pilot to notice it.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd statusgen && go build ./... && go test ./... -count=1` | exit 0 |
| 2 | `cd statusgen && go test ./... -run TestEvidenceActorGitLabVerifier -count=1 -v` | exit 0; Evidence committed by a `gitlab:`-qualified verifier account is reported BACKED — the row the pilot reported as unbacked |
| 3 | `cd statusgen && go test ./... -run TestEvidenceActorGitHubVerifierUnchanged -count=1 -v` | exit 0; the existing GitHub id-pinned path behaves exactly as before — a regression here would silently retire the control on the forge it already worked on |
| 4 | `cd statusgen && go test ./... -run TestEvidenceActorSelfAttestedStillUnbacked -count=1 -v` | **negative path**: Evidence committed by the IMPLEMENTER's identity is reported unbacked on BOTH forges. Without this row the brief could ship a check that reports everything backed and pass rows 2 and 3 |
| 5 | `cd statusgen && go test ./... -run TestEvidenceActorUnknownForgeIsCouldNotCheck -count=1 -v` | **negative path**: an entry naming an unrecognised forge yields could-not-check naming the forge — asserted on the message text, not merely on a non-zero result, so a "clean" answer fails the row |
| 6 | `cd statusgen && go test ./... -run TestVerifyrunRunnerFromForgeIdentity -count=1 -v` | exit 0; with a role identity bound for the repo's forge, the witness `Runner` is that identity and the recorded source says so |
| 7 | `cd statusgen && go test ./... -run TestVerifyrunFallbackOrder -count=1 -v` | exit 0; with no bound identity the CI-env path is used, and with neither the git-config path — each recording its own source |
| 8 | `cd statusgen && go test ./... -run TestVerifyrunStillRefusesSuppliedRunner -count=1 -v` | **negative path**: a runner supplied on the command line is still a usage error, and with no identity available at all the tool still refuses to write a witness — both pre-existing properties survive |
| 9 | `cd statusgen && go test ./... -run TestRosterGrammarParity -count=1 -v` | exit 0; the same roster fixture used by the desk-tools suite loads identically here, including legacy unqualified entries |
| 10 | `grep -c '^[\|] ' docs/streams/forge-neutral/identity.md` | ≥ 2 — statusgen's two consumers appear in the per-forge table rather than being re-derived here |
| 11 | `statusgen --root . --lint` | `LINT: PASS`, exit 0 — this repo's own board still lints with the changed checks |
| 12 | `statusgen --root . --consumers --brief forge-neutral/07` | exit 0 — every `consumers:` routing claim is corroborated against this branch's own diff |

## Pre-mortem → detection map

| Failure mode of the work | Caught by |
|---|---|
| The check is "fixed" by making the GitLab case report backed unconditionally — everything passes, nothing is checked | row 4, the discriminating negative control: a self-attested row must still read unbacked on both forges |
| An unrecognised forge resolves to a pass rather than could-not-check | row 5, asserted on message text |
| The GitHub path regresses while the GitLab path is added, retiring the control where it already worked | row 3 |
| The witness runner becomes settable by the session (an env var the runner honours) | row 8 |
| The witness stamps the forge identity but the Evidence table still names someone else, and nothing surfaces the disagreement | row 6's recorded-source assertion — the pair is comparable rather than merely present |
| statusgen and the desk tools disagree on the roster grammar, so one binary trusts an entry the other refuses | row 9, loading the SAME fixture in both suites |
| `identity.md` is duplicated rather than extended, so the two copies drift | row 10 |
| The changed checks redden this repo's own board | row 11 |
| The check remains dead in CI, so none of this runs where it matters | **no row** — explicitly out of scope (`facts:` cites `evidenceactor.go:128-131`); the Review gate confirms the brief did not make it worse, and the CI wiring is `forge-neutral/08`'s |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->
### Verify run — 2026-09-11, non-implementer dispatched verifier (opus-4.8[1m]-verifier, local) — gate: human, HELD at `implemented`

Target: merged `origin/main` @ `fc9001a7ab48ee9c859dd7e52f7543dec5f86c50` (two-protocol confirmed). Offline (`KUBECONFIG=/dev/null`) in an isolated worktree; runner ≠ implementer (delivering commit `b93380bb`, PR #825). Rows 1–9 from the nested `statusgen` module; rows 11–12 via a worktree-built binary (`--root <worktree>`) to sidestep the shared-home writeguard. gate: human + sensitive-data — table RUN for Evidence; no model sign-off.

| # | Command | Exit | Key observed output | Result |
|---|---------|------|---------------------|--------|
| 1 | `cd statusgen && go build ./... && go test ./... -count=1` | 0 | `ok … 28.6s` | PASS |
| 2 | `-run TestEvidenceActorGitLabVerifier -v` | 0 | gitlab: verifier service-account Evidence BACKED; worker-committed twin still flagged | PASS |
| 3 | `-run TestEvidenceActorGitHubVerifierUnchanged -v` | 0 | GitHub id-pin unchanged incl. impostor (wrong id + right login → `actorImpostor`) | PASS |
| 4 (neg) | `-run TestEvidenceActorSelfAttestedStillUnbacked -v` | 0 | implementer identity → `actorRejected` on BOTH forges; verifier's own commit → `actorVerifier`; non-SA addr under verifier name → rejected | PASS |
| 5 (neg) | `-run TestEvidenceActorUnknownForgeIsCouldNotCheck -v` | 0 | `bitbucket:` → policy Unavailable; message asserted to contain "bitbucket" AND "does not understand"; exactly one could-not-check notice | PASS |
| 6 | `-run TestVerifyrunRunnerFromForgeIdentity -v` | 0 | Runner = bound identity, source `runnerSourceForge`, rendered `(forge-…)`; both forges | PASS |
| 7 | `-run TestVerifyrunFallbackOrder -v` | 0 | no bound id → CI-env (ci-env source); neither → git-config source | PASS |
| 8 (neg) | `-run TestVerifyrunStillRefusesSuppliedRunner -v` | 0 | `--runner/--as/--identity/--who` → usage error; no identity → refuses to write a witness | PASS |
| 9 | `-run TestRosterGrammarParity -v` | 0 | shared roster loads identically (legacy unqualified → github/inferred); bare slug never accepted | PASS |
| 10 | `grep -c '^[|] ' docs/streams/forge-neutral/identity.md` | 0 | `9` (≥2); both statusgen consumers (`evidenceactor.go`, `verifyrun.go`) in the per-forge table (rows 132-133) | PASS |
| 11 | `statusgen --root <wt> --lint` | 0 | `LINT: PASS` (advisory NOTICEs only) | PASS |
| 12 | `statusgen --root . --consumers --brief forge-neutral/07` | 2 | COULD-NOT-CHECK — `no brief-v1 file` (statusgen v1.0.6 brief-v2 gap). Manual corroboration (`git show --stat b93380bb`): touches `evidenceactor.go`(+140), `verifyrun.go`(+139), `rosterconfig.go`(+37), `identity.md`(+13) — all `fixed-here`; `autoflip.go` NOT touched (matches its `follow-up forge-neutral/08` routing) | COULD-NOT-CHECK |

**Risk-bearing value (sensitive-data: yes — ENUMERATE → RANK → DERIVE):**
- Enumerated literals are TEST-FIXTURE ids only (gitlab 41987969/66, github 300000004/5/6) — not risk-bearing bindings; grep of shipped source (`evidenceactor.go`, `verifyrun.go`, `rosterconfig.go`, `forgeidentity.go`) for baked numeric ids is EMPTY (no real house bot USER id compiled in).
- Per-forge id-pin FORMS (the risk-bearing bindings): GitHub `^(\d+)\+([^@]+)@users\.noreply\.github\.com$` (id-pinned on the numeric USER id); GitLab `^service_account_group_[0-9]+_[0-9a-z]+@noreply\.[a-z0-9.-]+$` (address-SHAPE + git-author username, login-only — a GitLab commit address carries no verifiable USER id).
- `RISK-VALUE: DERIVED (top-ranked) — the GitLab login-only degradation.` GitLab cannot id-pin, so a gitlab verifier is matched by SA-address-shape + author username. Residual: a commit whose author username == the bound verifier username AND whose email is any valid-shaped `service_account_group_*_*@noreply.*` backs the row — the F-verify-self-attest surface on gitlab. DERIVED and bounded: identity.md:132 records it verbatim as "login-only … the weaker form", and row 4's gitlab branch bounds it (a non-SA address under the verifier name → rejected; the worker's own distinct SA → rejected). No `NAMED, NOT DERIVED` literal remains.

**Sensitive-data defense (gate: human), per-forge (row 4):** GitHub — `classify()` → `actorRejected` for the implementer/worker noreply because its numeric id ≠ the verifier's pinned id; only the verifier's id-pinned commit → `actorVerifier`. GitLab — `actorRejected` for the worker's distinct SA address and any non-SA address under the verifier name; only the bound verifier username + a SA-shaped address → `actorVerifier`. On both forges the committer must match the bound VERIFIER, not the implementer. Row 4 is genuinely discriminating (a rubber-stamp widening passes 2/3 but fails 4).

**Human open question (matches gate-why):** confirm the per-forge actor-matching rule is acceptable given GitLab offers no numeric user-id to pin — SA-address-shape + author-username as the weaker gitlab binding vs GitHub's id-pin; and confirm the witness runner derives only from the acting forge identity / CI-env / git-config, never from a session-settable value (row 8 proves the refusals survive).

**Scope-traceability:** one new supporting file `statusgen/forgeidentity.go` (+143, per-forge resolution) exercised by rows 2/4/6 (not orphan); no hand-written witness rows; no work maps to no Verify row concerningly.

**VERDICT: PASS** on rows 1–11; row 12 COULD-NOT-CHECK (brief-v2/`--consumers` gap, hand-corroborated) — **HELD at `implemented` (human sign-off owed via the verify-gate).**

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && go build ./... && go test ./... -count=1` | pass exit=0 | sha256:7b398161e40a | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd statusgen && go test ./... -run TestEvidenceActorGitLabVerifier -count=1 -v` | pass exit=0 | sha256:99996ef7f95a | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test ./... -run TestEvidenceActorGitHubVerifierUnchanged -count=1 -v` | pass exit=0 | sha256:8be5f2bbcb32 | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go test ./... -run TestEvidenceActorSelfAttestedStillUnbacked -count=1 -v` | pass exit=0 | sha256:db14edcfd3aa | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd statusgen && go test ./... -run TestEvidenceActorUnknownForgeIsCouldNotCheck -count=1 -v` | pass exit=0 | sha256:ba9516ca83af | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd statusgen && go test ./... -run TestVerifyrunRunnerFromForgeIdentity -count=1 -v` | pass exit=0 | sha256:155a1f224187 | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd statusgen && go test ./... -run TestVerifyrunFallbackOrder -count=1 -v` | pass exit=0 | sha256:2629b993dcbb | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd statusgen && go test ./... -run TestVerifyrunStillRefusesSuppliedRunner -count=1 -v` | pass exit=0 | sha256:92cf499adf04 | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd statusgen && go test ./... -run TestRosterGrammarParity -count=1 -v` | pass exit=0 | sha256:b7bf0a23f23e | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 10 | `grep -c '^[\|] ' docs/streams/forge-neutral/identity.md` | pass exit=0 | sha256:2e6d31a5983a | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 11 | `statusgen --root . --lint` | pass exit=0 | sha256:ae47fe3dbc48 | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 12 | `statusgen --root . --consumers --brief forge-neutral/07` | fail exit=2 | sha256:75fd6f72e40d | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |

### Re-verify notes — 2026-09-27, non-implementer dispatched verifier (claude-opus-5-5, local) — gate: human, HELD at `implemented`

The witness table directly above is this run's record. Target: merged main @ d034d39fe1c8f0a0a93ba1177f208c48b068a1d0 (worktree HEAD equal to the forge's main at run time, read with the verifier App token). Implementing commit b93380bb5 (PR #825); runner is not the implementer. Witness executed by the pinned statusgen v1.0.27 binary (sha256 matching the release pin) called directly, not via a wrapper, under `sandbox-exec` with network denied except loopback, a minimal PATH (system dirs + go + the pinned statusgen; no gh), `KUBECONFIG=/dev/null`, `GOFLAGS=-count=1`, `GOPROXY=off` over a pre-populated module cache — no row could reach a forge or proxy.

Per-row key output:
- Row 1: statusgen module builds; full package suite `ok` (about 56s). A first witness attempt recorded row 1 `fail exit=1` because three tests that `go build` a child binary (TestVersionFlag, TestBackCompatFlagsParse, the unknown-subcommand test) tried a module download inside the network-denied sandbox (`lookup proxy.golang.org: no such host`); with the module cache pinned by env and `GOPROXY=off` they pass. Environment artifact of the sandbox, not a code defect; the brief was restored and the witness re-run — the table above is the second run.
- Rows 2–9: each `--- PASS` / `ok`, run once more by hand as targeted package tests (`-run <name> -count=1 -v -timeout 240s`) with the same result.
- Row 4 discrimination checked by mutation: a rubber-stamp mutant (GitLab classify returns verifier for everything) FAILS row 4 ("implementer's own service-account commit must be unbacked"; "non-service-account address must be unbacked") and row 2; source restored with a path-specific checkout, tree clean before the witness.
- Row 5 asserts the could-not-check message text names the forge and says "does not understand"; exactly one notice, no backed/unbacked wording.
- Row 6 corroborated live: this run's own witness Runner cells carry `(forge-identity)` — the bound verifier identity resolved through the roster, ahead of the CI-env and git-config fallbacks.
- Row 10: output `9` (at least 2); statusgen/evidenceactor.go and statusgen/verifyrun.go rows present in the identity.md per-forge table.
- Row 11: `LINT: PASS`, exit 0.
- Row 12: exit 2, `--consumers: COULD-NOT-CHECK: assay:assay:forge-neutral:07 is not in the diff against d034d39f…` — on merged main there is no diff to corroborate, so the row is structurally vacuous (check-definition class, tracked by #1281). Hand corroboration: `git show --stat b93380bb5` touches evidenceactor.go, verifyrun.go, rosterconfig.go and identity.md (all routed fixed-here) and does NOT touch autoflip.go (routed follow-up forge-neutral/08). Row 12 is could-not-check, not a pass.

**Risk-bearing value (sensitive-data: yes) — ENUMERATE → RANK → DERIVE.** Scope: the b93380bb5 diff to the four consumer files plus forgeidentity.go, read at merged main line numbers.
1. scanGitlabServiceAccountRe = `^service_account_group_[0-9]+_[0-9a-z]+@noreply\.[a-z0-9.-]+$` @ statusgen/forgeidentity.go:65
2. GitLab accept-1 name compare = `strings.EqualFold(trimmedName, p.Verifier.Login)` @ statusgen/evidenceactor.go:403
3. noreplyEmailRe = `^(\d+)\+([^@]+)@users\.noreply\.github\.com$` @ statusgen/evidenceactor.go:185 (unchanged by the diff; named in facts)
4. idPinned = `p.VerifierForge != forgeGitLab && p.Verifier.ID != 0` @ statusgen/evidenceactor.go:261, with `verifierID := int64(0)` for GitLab @ statusgen/evidenceactor.go:330
5. recognised forge set = `"github"`, `"gitlab"` @ statusgen/forge.go:33,35; any other forge returns Unavailable @ statusgen/evidenceactor.go:317
6. explicit-forge discriminator = `strings.Contains(rest, ":")` @ statusgen/forgeidentity.go:124
7. acceptedLogins = GitLab `[]string{b.Slug}` @ statusgen/forgeidentity.go:176; GitHub `b.Slug + "[bot]", "app/" + b.Slug` @ statusgen/forgeidentity.go:178
8. forge runner renderings = `b.Slug + "[bot]"` @ statusgen/verifyrun.go:1017; `b.Slug` @ statusgen/verifyrun.go:1024
9. runner-source tokens = `"forge-identity"`, `"ci-env"`, `"git-config"` @ statusgen/verifyrun.go:926-928

Rank (irreversibility): 1+2 highest — together they are the whole GitLab acceptance rule; wrong means implementer Evidence is believed on GitLab, and a `verified` written on it is only undone by a human re-review, not by redeploying a fix. 3 and 4 next (the GitHub id-pin, and keeping a GitLab user id out of the GitHub id namespace). 5 and 6 (unknown forge must stay could-not-check). 7 and 8 (login renderings; a wrong one fails closed as unbacked). 9 last — display tokens, reversible, no trust decision.

- RISK-VALUE: NAMED, NOT DERIVED — scanGitlabServiceAccountRe = `^service_account_group_[0-9]+_[0-9a-z]+@noreply\.[a-z0-9.-]+$` @ statusgen/forgeidentity.go:65 — the shape is taken from a GitLab pilot observation, not from a cited GitLab format specification; the host is unpinned (any instance matches) and it matches every service account on every group, so it is a shape gate, not an identity binding. I could not confirm GitLab's documented service-account address format offline, and nothing in the repo derives the suffix class or the unpinned host.
- RISK-VALUE: NAMED, NOT DERIVED — GitLab accept-1 = `strings.EqualFold(trimmedName, p.Verifier.Login)` @ statusgen/evidenceactor.go:403 — with item 1 this is the only discriminator between the verifier and any other GitLab service account, and it compares a git author name the committer sets freely. The code and identity.md record it as the weaker login-only form; whether that strength is acceptable for an anti-self-attestation control is exactly the gate-why question and is a ruling, not a derivation.
- RISK-VALUE: DERIVED — noreplyEmailRe = `^(\d+)\+([^@]+)@users\.noreply\.github\.com$` @ statusgen/evidenceactor.go:185 — GitHub's documented noreply form is `<id>+<login>@users.noreply.github.com`, the id is the permanent account handle and the host is pinned; unchanged by this diff and row 3 shows the id-pinned path (including the wrong-id impostor case) behaves as before.
- RISK-VALUE: DERIVED — idPinned / verifierID = `p.VerifierForge != forgeGitLab && p.Verifier.ID != 0`, `int64(0)` @ statusgen/evidenceactor.go:261,330 — a GitLab service-account address carries a group id and a per-account suffix, never the user id, so no GitLab id can be pinned from commit metadata; zeroing it also stops a GitLab user id entering the map the GitHub id paths read (rosterconfig adds ids to Bots for github entries only).
- RISK-VALUE: DERIVED — recognised forge set = `"github"`, `"gitlab"` @ statusgen/forge.go:33,35 → Unavailable @ statusgen/evidenceactor.go:317 — the brief's three-state ground rule requires an unrecognised forge to be could-not-check; the code returns the Unavailable policy before classify can run, and row 5 asserts the message text.
- RISK-VALUE: DERIVED — explicit-forge discriminator = `strings.Contains(rest, ":")` @ statusgen/forgeidentity.go:124 — a legacy entry is at most `slug:id`, a GitHub App slug or GitLab username never contains a colon, and a numeric head is rejected, so a third segment can only be a forge qualifier; row 9 shows legacy unqualified entries still load as github (inferred).

Observations for the human gate:
- Since the 2026-09-11 verification, PR #1486 (issue #1477, still open) widened the same control: a second GitLab accept path on a declared display name (`ASSAY_GITLAB_DISPLAY_NAMES`, statusgen/evidenceactor.go:418) and an id-pinned GitLab human noreply arm (scanGitlabHumanNoreplyRe, statusgen/forgeidentity.go:87). Rows 2–5 still pass at merged main. Those literals are outside this item's diff and were not derived here.
- verifyrun's GitLab forge-identity source keys on git `user.name` equal to the roster username (statusgen/verifyrun.go:1023) and does not consult the display-name map. A GitLab session whose git name is the account's display name would fall through to the git-config source and stamp `human:<name>`, which is the D-9 shape again. No Verify row covers that case.
- The check is still dead in CI (evidenceactor.go header, "IT IS DEAD IN CI TODAY"); out of scope per the brief and not made worse.

**Open questions for the human (carried verbatim from the NAMED, NOT DERIVED lines):** (a) scanGitlabServiceAccountRe = `^service_account_group_[0-9]+_[0-9a-z]+@noreply\.[a-z0-9.-]+$` @ statusgen/forgeidentity.go:65 — confirm the address shape against GitLab's own format, and whether the host should stay unpinned. (b) GitLab accept-1 = `strings.EqualFold(trimmedName, p.Verifier.Login)` @ statusgen/evidenceactor.go:403 — confirm that a login-only match on the git author name (plus the shape gate) is an acceptable strength for the GitLab verifier, given GitHub's is id-pinned.

VERIFY: PASS — rows 1–11 pass on merged main d034d39f; row 12 could-not-check (merged-main `--consumers` has nothing to corroborate, #1281; hand-corroborated). gate: human + sensitive-data — Evidence only, status stays `implemented`; sign-off is the human's.

## Review
Gate: **human** (from frontmatter — `sensitive-data: yes`). Reviewer records verdict + date in
the stream README table.

Core-system reviewer questions, answered in the verdict:
1. What single control stands between a self-attested Evidence row and a believed one? (The
   accepted-actor comparison.) Is it acceptable alone? (No — the witness row's independent
   record of the acting identity is the second layer, and task 5 is what makes the two
   comparable.)
2. Does any Verify row prove a LOWER layer catches the fault with the UPPER bypassed? (Row 4
   is the discriminating control: it fails on a check that has been widened into a rubber
   stamp, which every other row would pass.)
