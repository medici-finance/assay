---
brief: assay:assay:desk-containers:02
title: runtime credential contract (PEM + model env) + image layer-secret scan
wave: 1
depends: []
unblocks: ["desk-containers/03", "desk-containers/04", "desk-containers/05", "desk-containers/06"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: >-
  This brief designs the custody path for the bot App's signing PEM and the
  model-endpoint API keys inside containers. A wrong contract here (a path that tempts
  baking, a fallback to ambient host credentials, a scan with a blind spot) is a
  credential-exfiltration surface on a PUBLIC image. The human confirms: the mount/env
  contract keeps every credential out of image layers and out of image defaults, the
  fail-closed behaviour is right, and the layer scan's patterns cover the key material
  we actually hold.
issues: []
schema: brief-v2
authored: 2026-08-22 by desk-containers scoping session
sources:
  - "medici-finance/assay#63 — the request (pem for the bot; environmental variables to reach the models)"
  - "docs/streams/desk-containers/spec.md — runtime credential contract summary + non-goal 'no secret in any image layer'"
  - "freshness-checked 2026-08-22 @ b3a2067 — no containers/secrets.md or layer-scan script exists"
why: >-
  Four surfaces (desk images, launch script, compose, k8s) all inject the same
  credentials; without one contract each invents its own paths and env names, and the
  no-secret-in-layer rule is enforced only by hoping each reviewer notices. One
  human-reviewed contract plus one automated scan turns the rule into a check.
consumers:
  - "containers/<desk-name>/Dockerfile (env names, mount defaults): follow-up desk-containers/03"
  - "containers/desk-run.sh (mount + env-file flags): follow-up desk-containers/04"
  - "containers/compose.yaml (secrets: + env_file): follow-up desk-containers/05"
  - "containers/k8s/ (Secret volume + envFrom): follow-up desk-containers/06"
version: 1
id: 73c2a9fd-dcdf-44a3-b79f-8bfb88f2bcae
---

# Brief 02 — runtime credential contract + layer-secret scan

## Context

single-point-of-failure: without this brief the only control is per-reviewer vigilance
on each Dockerfile — one distracted review away from a baked PEM. This brief adds two
independent layers behind it: a normative contract every surface implements, and an
automated layer scan that fails a build carrying key-shaped material. The layers are
independent — the contract fails by being mis-designed, the scan by a pattern gap; the
mutation row proves the scan catches what a review misses.

files:
- `containers/secrets.md` (new) — the normative runtime credential contract.
- `containers/scripts/layer-secret-scan.sh` (new) — scans a built image's layers +
  config for key-shaped material; exit non-zero on any hit.
- `containers/scripts/layer-secret-scan.test.sh` (new) — the mutation test fixture.

facts:
- Contract (from spec, to be made normative here): PEM mounted read-only at
  `/run/secrets/assay/app.pem`; `ASSAY_APP_PEM_FILE` names the path (image may default
  the PATH VALUE — never the file). Model credentials/endpoints (`ANTHROPIC_API_KEY`,
  base-URL overrides, and the full passthrough list this brief enumerates) come from
  runtime env (`--env-file` / compose `env_file` / k8s `secretRef`).
- Fail-closed: a desk that starts without its PEM or model env reports what is missing
  and exits — no fallback to ambient host credentials, no anonymous degraded mode.
- Scan scope: every layer's filesystem AND the image config (env defaults, build-arg
  echoes in history) — patterns at minimum: `BEGIN … PRIVATE KEY`, `ghs_`, `ghp_`,
  `github_pat_`, `sk-ant-`. The scan runs against base + all five desk images in
  brief 03's matrix.
- The scan is one layer, not the whole defence: contract review (human) + scan
  (automated) + the leak-sweep already gating this repo's main are three independent
  controls.
- Weakening this contract or the scan's patterns later is a human decision, never a
  model self-clear.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature
  branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on a branch (single writer = main's CI).
- Use only PLACEHOLDER key material in fixtures (clearly fake, e.g. a freshly generated
  throwaway test key labelled as such) — never a real credential, even in a test.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write `containers/secrets.md` (planned): the mount path + env-name table (PEM, GitHub token if
   any, each model-endpoint variable), per-surface injection recipe (docker run flags,
   compose `secrets:`/`env_file`, k8s Secret volume + `envFrom`), the fail-closed rule,
   and the explicit "no secret in any image layer" statement with what that forbids
   (COPY/ADD, build-args, ENV value defaults, layer history).
2. Implement `layer-secret-scan.sh <image>`: walks `docker history --no-trunc`, the
   image config env, and each layer's filesystem (`docker save` + tar scan) for the
   pattern set; prints hits; exit 1 on any hit, exit 0 clean.
3. Implement the mutation test: build a throwaway fixture image that COPYs a clearly
   fake private key, run the scan, assert it goes RED; run the scan on a clean fixture,
   assert green.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -c 'ASSAY_APP_PEM_FILE' containers/secrets.md` | exit 0; count ≥ 1 |
| 2 | `grep -c '/run/secrets/assay/app.pem' containers/secrets.md` | exit 0; count ≥ 1 |
| 3 | `grep -ci 'no secret in any image layer' containers/secrets.md` | exit 0; count ≥ 1 |
| 4 | `sh containers/scripts/layer-secret-scan.test.sh` | exit 0; prints `RED on baked-key fixture` and `GREEN on clean fixture` — the mutation test proves the scan detects a baked key (dereferencing row: the scan's central claim is exercised, not counted) |
| 5 | `docker build -t assay-desk-base:dev containers/base && sh containers/scripts/layer-secret-scan.sh assay-desk-base:dev` | exit 0 — the real base image scans clean (run once brief 01 has landed; before that, the clean fixture in row 4 stands in) |
| 6 | `statusgen --consumers --brief desk-containers/02 --root .` | exit 0; the four follow-up entries (03/04/05/06) listed for the reviewer to weigh |
| 7 | `shellcheck containers/scripts/layer-secret-scan.sh` | exit 0 |

## Definition of Done
- Verify rows green, recorded in Evidence by a non-implementer.
- **No secret in any image layer**: the contract states it normatively, the scan
  enforces it mechanically, and the mutation row proves the scan fires. No fixture or
  doc contains real key material.
- Every consumer surface (03/04/05/06) can implement injection from `secrets.md` alone,
  without inventing a path or env name.
- Human review recorded: contract + scan patterns + fail-closed behaviour confirmed by
  a human (see gate-why).

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->
Verified on merged main at `86c7d62c8081` by a NON-implementer (the implementing
commit is `6ccecda5`, merged via PR #67; this table was run by the verify-desk
runner named in the Runner column, which did not author any deliverable).

Runner tier: this brief is `risk.sensitive-data: yes`, so its Evidence was run by a
strong-tier runner per the verifier floor.

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `grep -c 'ASSAY_APP_PEM_FILE' containers/secrets.md` | 0 | `5` (≥ 1) | 2026-09-11 | opus-5[1m]-verifier |
| 2 | `grep -c '/run/secrets/assay/app.pem' containers/secrets.md` | 0 | `6` (≥ 1) | 2026-09-11 | opus-5[1m]-verifier |
| 3 | `grep -ci 'no secret in any image layer' containers/secrets.md` | 0 | `3` (≥ 1) | 2026-09-11 | opus-5[1m]-verifier |
| 4 | `sh containers/scripts/layer-secret-scan.test.sh` | 0 | `baked PEM-in-layer fixture -> scan exit 1` / `baked key-in-ENV fixture -> scan exit 1` / `clean fixture -> scan exit 0`, then both required tokens `RED on baked-key fixture` and `GREEN on clean fixture`. Docker 29.4.0. The mutation claim is exercised, not counted: the two baked fixtures are built at runtime and the scan returns non-zero on each; the clean fixture returns 0 | 2026-09-11 | opus-5[1m]-verifier |
| 5 | `docker build -t assay-desk-base:dev containers/base && sh containers/scripts/layer-secret-scan.sh assay-desk-base:dev` | 1 | COULD-NOT-CHECK (not a deliverable failure) — the build never reaches the scan. `failed to authorize: failed to fetch oauth token: denied: denied` on `FROM ${DESK_TOOLS_IMAGE}`: the base image's pinned `desk-tools` image is not pullable without a registry credential this verifier session does not hold. Independently, the row's command form cannot succeed as written even with that credential: the base `Dockerfile` does `COPY plugins/assay/`, which needs the REPO ROOT as build context, but the row passes `containers/base` (proved with the same COPY in the same context: `"/plugins/assay": not found`). Both are `desk-containers/01`/`03` concerns, not this brief's. The brief's own stated fallback governs this row — "run once brief 01 has landed; before that, the clean fixture in row 4 stands in" — and row 4's clean fixture passed, so a real built image was scanned clean | 2026-09-11 | opus-5[1m]-verifier |
| 6 | `statusgen --consumers --brief desk-containers/02 --root .` | 2 | COULD-NOT-CHECK (not a deliverable failure) — statusgen v1.0.6 (current latest release). Two independent causes, both external to this brief's deliverables. (a) The row's brief-ID form is STALE: it was authored when this brief was `schema: brief-v1` with `brief: desk-containers/02` (confirmed at the PR head), and the brief-v1 -> brief-v2 flag day migrated the id to `assay:assay:desk-containers:02`; the short form matches no brief and reports the misleading `COULD-NOT-CHECK: no brief-v1 file`. (b) With the CORRECT qualified id the tool answers precisely and the row is still uncorroborable: `assay:assay:desk-containers:02 is not in the diff` — `--consumers` corroborates only claims a branch itself made, and the implementing commit `6ccecda5` touched the three deliverables ONLY, never the brief file, so all four `consumers:` entries are inherited from the authoring commit and are UNCHECKED by the tool's own design. Re-running the tool's prescribed post-merge recipe at the PR head against its merge-base reproduces the same result. Corroborated BY HAND instead — see "Consumers corroboration" below | 2026-09-11 | opus-5[1m]-verifier |
| 7 | `shellcheck containers/scripts/layer-secret-scan.sh` | 0 | no findings (ShellCheck 0.11.0). The test script also passes clean | 2026-09-11 | opus-5[1m]-verifier |

Rows 1-7 were independently re-run by `statusgen verifyrun --dry-run`, whose
machine-derived verdicts match this table exactly (pass on 1/2/3/4/7, fail on
5/6). That witness table was deliberately NOT written back: `verifyrun` derives
the Runner from the worktree git identity, which here resolves to a human
account that did not run these rows, and a witness must not misattribute.

**Consumers corroboration (hand-done, standing in for row 6).** All four
`consumers:` entries are `follow-up` routings, so the correct evidence is that
each names a real brief and that this brief's diff did NOT touch the routed
path. Both hold: `desk-containers/03` (per-desk Dockerfiles), `04`
(`desk-run.sh`), `05` (`compose.yaml`), `06` (`k8s/`) all exist as briefs, and
`6ccecda5` touched only `containers/secrets.md` and the two scan scripts — no
consumer surface. The Definition-of-Done claim that every consumer surface can
implement injection from `secrets.md` alone is corroborated in the strongest
available way: `desk-containers/03` is already implemented, and the
`containers/entrypoint.sh` it produced implements this contract exactly —
defaults `ASSAY_APP_PEM_FILE` to the canonical path (the path VALUE only, which
§2 permits), checks the file exists and is readable, runs a model-credential
preflight over the §3 variable set, and exits 78 (EX_CONFIG) stating it does not
fall back to any ambient host credential.

**Substance checks (credential-custody brief — read, not grepped).**

- *Fail-closed.* §5 states it normatively and specifically: a desk missing its
  PEM or model credential names what is missing and exits non-zero, with **no
  fallback to ambient host credentials**. It goes further than the grep would
  show, enumerating the SDK's own resolution chain (API key -> auth token ->
  on-disk profile -> workload-identity -> default profile) and requiring the
  chain to terminate at the runtime-injected credential, calling out "started
  successfully on an unknown credential" as the exact forbidden outcome.
- *Full no-secret-in-any-layer scope.* §6 forbids all four vectors the brief
  names: `COPY`/`ADD`, credential build-args (noting they persist in layer
  history even when the final stage drops the file), credential `ENV` value
  defaults (while correctly permitting the PEM PATH value), and write-then-delete
  in an earlier layer.
- *Scanner pattern coverage vs the required minimum.* All five required patterns
  are present, plus one extra: `BEGIN[A-Z0-9 _-]*PRIVATE KEY` (covers RSA / EC /
  OPENSSH / plain PEM variants), `gh[ps]_[A-Za-z0-9]{20,}` (covers both the
  `ghp_` and `ghs_` prefixes), `github_pat_[A-Za-z0-9_]{20,}`,
  `sk-ant-[A-Za-z0-9_-]{10,}`, and a generic `sk-[A-Za-z0-9]{20,}`. **No gap
  against the minimum set.** Each alternative requires a run of body characters
  after the prefix, so the script's own documentation of its prefixes does not
  self-match, and the PEM alternative deliberately omits the dashed fence for
  the same reason.
- *Both surfaces genuinely scanned.* Not one or the other: the script collects
  `docker history --no-trunc` (build-arg echoes, ENV instructions), `docker
  inspect` (image-config env defaults and labels), AND every layer filesystem
  via `docker save` plus extraction of each nested layer tar, then greps all of
  them. Row 4 proves both halves independently — its two baked fixtures plant a
  secret in a LAYER FILE and in an `ENV` DEFAULT respectively, and each is
  caught, so a scan covering only one surface would fail that row. Layer
  whiteouts are not replayed, which is the conservative direction for a secret
  scan: a credential deleted by a later layer is still reported.
- *Fail-closed scanner.* A surface that cannot be read (no docker, no image,
  unreadable history/config/save) exits 2, never 0 — "could not scan" is never
  reported as clean.
- *No real credential material anywhere.* Confirmed by reading all three
  deliverables and by pattern sweep: no whole token and no PEM fence is
  committed in any of them. The fixtures assemble their synthetic values at
  RUNTIME from fragments, and the values are self-labelling —
  a PEM body reading `THIS-IS-NOT-A-REAL-KEY` + `SYNTHETIC-TEST-FIXTURE-ONLY`, a
  `TESTING FAKE PRIVATE KEY` header, and a model key whose body is `FAKE` +
  zeroes + `TESTONLY`. The fixture images are built `FROM scratch` (no pull) and
  removed on exit.

**VERIFY: HELD** — the deliverables are sound and no defect was found in them.
Five of seven rows pass outright; rows 5 and 6 are COULD-NOT-CHECK for reasons
external to this brief (a registry credential plus a wrong build context in row
5's command, both owned by the base-image brief; a stale brief-ID form plus a
structural limit of `--consumers` in row 6), and row 5's deferral is
pre-authorized by the brief itself. Status therefore stays `implemented` rather
than advancing to `verified`, for one further reason that no amount of re-running
can clear: this brief is `gate: human` with `risk.sensitive-data: yes`, and its
Definition of Done requires "human review recorded: contract + scan patterns +
fail-closed behaviour confirmed by a human". That review is still open and is not
a model's to self-clear — the contract's own §1 says weakening it "is a human
decision, never a model self-clear". A verifier recording green rows is not that
review.

HUMAN GATE: the contract, the scan's pattern set, and the fail-closed behaviour
need a human's confirmation before this brief can advance. The substance checks
above are offered as input to that review, not as a substitute for it.

## Review
Gate: human (sensitive-data: yes — App-PEM custody design; see gate-why). Reviewer
answers both core-control questions: (1) the single control standing between a baked
credential and a public image is the contract+scan pair backed by the repo leak-sweep —
acceptable only if all three are independent; (2) the mutation row (Verify 4) proves
the lower layer fires with the upper (review) bypassed. Verdict + date in the stream
README table.
