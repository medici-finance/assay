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
