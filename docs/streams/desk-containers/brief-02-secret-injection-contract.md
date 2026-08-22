---
brief: desk-containers/02
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
schema: brief-v1
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

## Review
Gate: human (sensitive-data: yes — App-PEM custody design; see gate-why). Reviewer
answers both core-control questions: (1) the single control standing between a baked
credential and a public image is the contract+scan pair backed by the repo leak-sweep —
acceptable only if all three are independent; (2) the mutation row (Verify 4) proves
the lower layer fires with the upper (review) bypassed. Verdict + date in the stream
README table.
