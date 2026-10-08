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
version: 2
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
| 5 | `docker build --platform linux/amd64 -f containers/base/Dockerfile -t assay-desk-base:dev . && sh containers/scripts/layer-secret-scan.sh assay-desk-base:dev` | exit 0; output is `clean: no key-shaped material found in image 'assay-desk-base:dev'` (the real base image builds and scans clean). Run from the repository root: the base Dockerfile COPYs `plugins/assay/` from the build context, so the context must be the repo root (`.`); with `containers/base` as the context the build stops at `"/plugins/assay": not found` and the scan never runs. `--platform linux/amd64` builds the image the Dockerfile's `TARGETARCH=amd64` default targets, so an arm64 host (under emulation) builds the same image as an amd64 host. A build that cannot pull its pinned parent image is COULD-NOT-CHECK, never a pass. The scan does not yet catch a key written in one layer and overwritten at the same path by a later layer (#2256), so a clean result does not cover that case. Re-authored per #2257 |
| 6 | `statusgen --consumers --brief desk-containers/02 --root . --base 709c223eff6d~1` | exit 0; output is `summary: 4 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing`, with a `CORROBORATED` line for each follow-up entry (03/04/05/06). `--consumers` judges only briefs that sit inside the diff it reads, and `709c223eff6d` is the commit that authored this brief and its `consumers:` list, so pinning the base to its parent keeps the brief in scope on merged main. The diff only decides scope here: all four entries are `follow-up` routings, which are judged against the CURRENT tree (each target must be a brief listed in its stream README, and must reference this brief back). So the row reads main as it stands: a `consumers:` entry retargeted to a brief that does not exist, or a target dropped from its stream README, is DISPROVED (exit 1), and a target that stops referencing this brief is UNCHECKED, which the summary line no longer matches. The diff runs from that parent to the working tree, uncommitted edits included, so run the row on a clean checkout of merged main. Run without `--base` on merged main, the check reports COULD-NOT-CHECK because the brief is not in the diff. Needs full history: a clone that cannot resolve `709c223eff6d` is COULD-NOT-CHECK. Re-authored per #2257 |
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
### CORRECTION to row 5 — revised from COULD-NOT-CHECK to **FAIL** (same verifier, same day)

The row-5 entry above reported COULD-NOT-CHECK because `docker build` could not
pull the base image's pinned parent. That reason stands for the *build* half, but
it led me to stop one step short. A genuine build of this very
`containers/base/Dockerfile` already existed on the verification host
(`assay-desk-base:dev`, built 2026-08-22, 24 layers), so **row 5's scan half was
run for real** against a real multi-layer desk base image:

```
$ sh containers/scripts/layer-secret-scan.sh assay-desk-base:dev
FAIL: key-shaped material detected in image 'assay-desk-base:dev'
  ... 16 hits ...
exit 1
```

Provenance of that image is established, not assumed: its OCI label description
matches this Dockerfile's `LABEL` verbatim (including the string "No credentials
in any layer."), and its history carries `GO_VERSION=1.25.0`, the
`/opt/assay/plugin` COPY, the `install-agent-cli` heredoc, and this Dockerfile's
exact `useradd` line. It was built with the repository root as context — which is
independent confirmation of the build-context defect noted in the original row-5
entry.

**Row 5 therefore FAILS, and every one of the 16 hits is a FALSE POSITIVE.** None
is a credential belonging to this project:

| Hit source | What it actually is |
|---|---|
under `/usr/local/go/src/`: `crypto/x509/platform_root_key.pem`, `crypto/tls/testdata/example-key.pem`, crypto/tls/example_test.go (Go stdlib path, not a repo path) | Go standard-library test fixtures, shipped in every Go source tree |
| `/usr/local/lib/node_modules/npm/...` (4 hits: `man7/config.7`, `definitions.js`, `using-npm/config.html`, using-npm/config.md (npm docs path, not a repo path)) | npm's own documentation, which *describes* PEM key config options |
| `/usr/lib/x86_64-linux-gnu/libssh2.so.1.0.1`, `/usr/lib/x86_64-linux-gnu/libgnutls.so.30.34.3`, `/usr/bin/gpgv` | PEM header format strings compiled into distro binaries |
| `/usr/local/bin/gh` | the gh CLI binary, matched by the loose generic `sk-[A-Za-z0-9]{20,}` alternative against an arbitrary base64-ish run |
| 5 `blobs/sha256/…` hits | the SAME layer content re-reported: the scanner greps both the extracted rootfs and the raw layer tars, so each filesystem hit is double-counted |

**Why this is a real defect and not an environment quirk.** The scanner has no
exclusion for toolchain-provided material, no distinction between "a PEM file"
and "a string that mentions PEM", and one pattern (`sk-` generic) loose enough to
match compiled binaries. Every one of these files comes from a toolchain the
Dockerfile itself installs — Go, Node/npm, gh, and the debian base. The
consequence is that row 5's stated expectation ("exit 0 — the real base image
scans clean") is **unachievable as the scanner is currently written**, for this
image or any realistic successor. Wired into CI as a fail-closed gate it would
red every image build, and a control that always reds is a control that gets
switched off — which would leave the contract's third layer (the repo leak-sweep)
carrying the rule alone.

This does NOT retract row 4: the mutation test is still valid and the scan does
fire on planted secrets. The defect is precision, not sensitivity — the scan
catches real baked keys AND everything else, and its fail-closed design converts
that imprecision into an unusable gate. Filed as a bug; status stays
`implemented`.
### 2026-10-06 re-verify on merged main 3012e2bed680 (non-implementer)
**VERIFY: FAIL — row 5 as written exits 1 (build-context defect in the row's command), row 6 could not check, and an independent probe found a recall regression in the shipped scanner: a key file in an earlier layer, overwritten at the same path by a later layer, is missed (5 of 6 fixtures)**

Run on merged main `3012e2bed680aa63ee9d378e99b199043f3d3f99`, the forge's `main` head at run time, by a verifier that wrote none of this brief's deliverables. The run covers the scanner as amended by the false-positive fix in #1011. Host: Docker 29.4.0 (daemon available), ShellCheck 0.11.0, statusgen v1.0.32.

| # | Command | Expected | Observed | Date / Runner |
|---|---------|----------|----------|---------------|
| 1 | `grep -c 'ASSAY_APP_PEM_FILE' containers/secrets.md` | exit 0; count ≥ 1 | exit 0; `5` | 2026-10-06 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | `grep -c '/run/secrets/assay/app.pem' containers/secrets.md` | exit 0; count ≥ 1 | exit 0; `6` | 2026-10-06 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | `grep -ci 'no secret in any image layer' containers/secrets.md` | exit 0; count ≥ 1 | exit 0; `3` | 2026-10-06 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | `sh containers/scripts/layer-secret-scan.test.sh` | exit 0; prints `RED on baked-key fixture` and `GREEN on clean fixture` | exit 0. Fixture exits: baked PEM-in-layer 1, baked key-in-ENV 1, clean 0, false-positive fixture 1. Printed `RED on baked-key fixture`, `GREEN on clean fixture`, `RED on false-positive fixture's real secret (/opt/app/leaked.pem) — no bypass`, `GREEN on all five allowlisted-path mimics (Go src, npm docs, gpgv, libssh2, gh) — none reported`, `RED on the generic sk--in-binary regression fixture (/opt/app/vendored-tool)` | 2026-10-06 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | `docker build -t assay-desk-base:dev containers/base && sh containers/scripts/layer-secret-scan.sh assay-desk-base:dev` | exit 0: the real base image scans clean | **FAIL, exit 1.** The pinned desk-tools parent now pulls, but the build stops at `COPY plugins/assay/ /opt/assay/plugin/` with `"/plugins/assay": not found`. The base Dockerfile needs the repository root as build context, and this row passes the containers/base directory. The scan never ran. **Supplementary run, outside the row's literal command:** the documented form from containers/README.md, `docker build --platform linux/amd64 -f containers/base/Dockerfile -t <tag> .`, built with exit 0. `--platform linux/amd64` was needed because this host is arm64 and the Dockerfile defaults TARGETARCH to amd64; without it the amd64 Node tarball fails to execute (exit 255). Scanning that real 24-layer base image exited 0: `clean: no key-shaped material found`. So the #1011 fix removed all 16 false positives recorded on 2026-09-11. The row's command form still needs correcting | 2026-10-06 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | `statusgen --consumers --brief desk-containers/02 --root .` | exit 0; the four follow-up entries (03/04/05/06) listed | **COULD-NOT-CHECK, exit 2.** `COULD-NOT-CHECK: assay:assay:desk-containers:02 is not in the diff against 3012e2bed680…`. By design, `--consumers` judges only claims made inside the diff under test. The qualified id gives the same result. **Supplementary run, outside the row's literal command:** pointing `--base` at the parent of the commit that authored the brief (`statusgen --consumers --brief assay:assay:desk-containers:02 --base 709c223ef^ --root .`) exited 0 with `4 corroborated, 0 disproved, 0 unchecked`, covering all four follow-ups (03/04/05/06) | 2026-10-06 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | `shellcheck containers/scripts/layer-secret-scan.sh` | exit 0 | exit 0, no findings. The test script is also clean | 2026-10-06 assay-verifier-app[bot] (on-behalf-of human:ian) |

**Execution witness.** The `statusgen verifyrun` witness table that follows this fragment was written by a statusgen binary built from this SHA's own statusgen source tree, run in a clean throwaway tree at this SHA under a throwaway HOME. Its verdicts match the table above: rows 1, 2, 3, 4 and 7 pass (exit 0); row 5 fails (exit 1); row 6 fails (exit 2). Its cells are the tool's own, with no hand edits. It stamps the UTC date 2026-10-05, which is the same run as the 2026-10-06 local date above. The output hashes for rows 5 and 6 differ between runs because the docker build and statusgen output carry per-run detail; rows 1-4 and 7 reproduced byte-for-byte in an earlier `--dry-run` pass.

**Independent probe: scanner recall regression, write-then-overwrite across layers.** Contract §6 forbids leaving a credential in an earlier layer that a later step removes, because the earlier layer still contains it. The scanner extracts every layer tar into one shared rootfs. Since #1011 it no longer greps the raw layer tars, a change made to stop double-counting hits. The result: when a later layer overwrites the same path with benign content, whichever layer extracts last decides what gets scanned, and the extraction order follows blob-digest order, which has nothing to do with layer order. Fixtures used a synthetic, self-labelled fake key: COPY the key to one path, then overwrite that path in a later layer.
- `FROM scratch`, COPY key, then COPY a benign file to the same path: current scanner exit 0 (`clean`), three runs out of three. The scanner from the implementing commit exit 1. A manual `docker save` confirmed the fake key is still present in the earlier layer blob.
- `FROM debian:bookworm-slim`, COPY key, then `RUN echo scrubbed-N > <same path>`, six variants: current scanner exit 0 on 5 of 6, exit 1 on 1 of 6. The scanner from the implementing commit exit 1 on 6 of 6.
- Same base, COPY key, then `RUN rm <path>`: both scanners exit 1. The whiteout leaves the original file in place.

A key-bearing image therefore passes the gate depending on content hashes. That breaks the Definition of Done's claim that "the scan enforces it mechanically". Row 4's mutation test does not cover this vector. The fix is to restore raw scanning of the earlier layers, or to extract each layer into its own directory, and to add an overwrite fixture to row 4. The fix is reversible, but until it lands this layer of the three-control design has a blind spot.

**Risk-bearing values.** The trigger fires: `risk.sensitive-data: yes`, and the change under review is a security scanner plus a credential contract. The enumeration covers the three deliverables at the verified SHA, including the #1011 changes. File:line references point to that SHA.

| Literal | Location | Rank / reversibility |
|---|---|---|
| `PEM = 'BEGIN[A-Z0-9 _-]*PRIVATE KEY'` | containers/scripts/layer-secret-scan.sh:91 | top: a gap lets a baked key ship in a public image, and a published layer cannot be recalled |
| `GHTOK = 'gh[ps]_[A-Za-z0-9]{20,}\|github_pat_[A-Za-z0-9_]{20,}'` | layer-secret-scan.sh:92 | top: same |
| `MODELKEY_ANT = 'sk-ant-[A-Za-z0-9_-]{10,}'` | layer-secret-scan.sh:93 | top: same |
| `MODELKEY_GENERIC = 'sk-[A-Za-z0-9]{20,}'` | layer-secret-scan.sh:94 | high: same exposure; precision trade-off |
| allowlist `"$WORK"/rootfs/usr/local/go/src/*` | layer-secret-scan.sh:117 | high: an exemption is a recall hole at that prefix |
| allowlist `"$WORK"/rootfs/usr/local/lib/node_modules/npm/*` | layer-secret-scan.sh:124 | high: same |
| allowlist `"$WORK"/rootfs/usr/bin/gpgv` | layer-secret-scan.sh:137 | medium: single exact path |
| allowlist `"$WORK"/rootfs/usr/lib/*/libssh2.so*` | layer-secret-scan.sh:138 | medium: narrow glob |
| allowlist `"$WORK"/rootfs/usr/lib/*/libgnutls.so*` | layer-secret-scan.sh:139 | medium: narrow glob |
| allowlist `"$WORK"/rootfs/usr/local/bin/gh` | layer-secret-scan.sh:148 | medium: single exact path |
| hit exit `exit 1` | layer-secret-scan.sh:252 | medium: gate semantics, reversible |
| could-not-scan exit `exit 2` | layer-secret-scan.sh:60, :65, :162, :168, :174 | medium: fail-closed semantics, reversible |
| mask width `'%.4s'` | layer-secret-scan.sh:227 | low: prints at most 4 leading chars of a hit (a public prefix) |
| PEM mount path `/run/secrets/assay/app.pem` | containers/secrets.md:39, :55 | medium: a contract constant, reversible by edit |
| PEM path env name `ASSAY_APP_PEM_FILE` | containers/secrets.md:57 | medium: a contract constant, reversible |
| PEM file mode `mode: 0400` / `defaultMode: 0400` | containers/secrets.md:144, :171 | low: operational; fails closed (unreadable PEM ⇒ exit), reversible |

- RISK-VALUE: DERIVED — PEM = `BEGIN[A-Z0-9 _-]*PRIVATE KEY` @ containers/scripts/layer-secret-scan.sh:91. Every PEM private-key armor header (PKCS#1 `RSA`, SEC1 `EC`, `OPENSSH`, PKCS#8 plain and `ENCRYPTED`, and the PGP private-key block) has the form BEGIN, then an uppercase/space/hyphen label, then `PRIVATE KEY`, and the character class accepts all of them. Leaving out the dashed fence costs no recall, because every real key carries the header.
- RISK-VALUE: DERIVED — GHTOK = `gh[ps]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}` @ layer-secret-scan.sh:92. It covers the brief's required minimum (`ghp_`, `ghs_`, `github_pat_`). Real token bodies are well over 20 characters (36 for classic and installation tokens, longer for fine-grained), so `{20,}` is a safe lower bound that catches every real token and skips bare prefix mentions. NAMED, NOT DERIVED part: the `gho_`, `ghu_` and `ghr_` prefixes (OAuth, user-to-server and refresh tokens) are not covered. Whether a desk container can ever hold one depends on how `gh` is authenticated at runtime, and confirming that for every desk is outside this pass. It is above the brief's minimum and a candidate for the human pattern review.
- RISK-VALUE: DERIVED — MODELKEY_ANT = `sk-ant-[A-Za-z0-9_-]{10,}` @ layer-secret-scan.sh:93. Anthropic API keys carry the `sk-ant-` prefix followed by a long URL-safe body (letters, digits, `_`, `-`). The class matches that alphabet, and `{10,}` is far below the real body length, so recall is complete for this key family.
- RISK-VALUE: NAMED, NOT DERIVED — MODELKEY_GENERIC = `sk-[A-Za-z0-9]{20,}` @ layer-secret-scan.sh:94. It is a deliberately loose catch-all, and its precision/recall trade-off has no first-principles bound. Its one proven false positive is handled by a single exact-path exemption (line 148), not by weakening the pattern. Row 4's binary-regression fixture confirms it still fires inside binaries.
- RISK-VALUE: NAMED, NOT DERIVED — allowlist prefixes @ layer-secret-scan.sh:117 and :124. These are justified as wholly upstream-populated trees, and this pass confirmed the real base image is clean with them in place. Proving that no later desk image step ever writes a credential under those prefixes would require auditing every per-desk Dockerfile, which this pass did not do.

**Human review of record:** the driver's ratification at https://github.com/medici-finance/assay/issues/900#issuecomment-6004016520 approves the contract, the fail-closed behaviour and the #1011-amended pattern set. This verifier did not reverse that ruling. The overwrite-blind-spot finding above is new input for that gate.

**Status:** stays `implemented`. Row 5's command form and the scanner's overwrite blind spot both need a fix before this brief can advance.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -c 'ASSAY_APP_PEM_FILE' containers/secrets.md` | pass exit=0 | sha256:f0b5c2c2211c | 2026-10-05 | assay-verifier-app[bot] @ 3012e2bed680 (on-behalf-of human:ian) (forge-identity) |
| 2 | `grep -c '/run/secrets/assay/app.pem' containers/secrets.md` | pass exit=0 | sha256:06e9d52c1720 | 2026-10-05 | assay-verifier-app[bot] @ 3012e2bed680 (on-behalf-of human:ian) (forge-identity) |
| 3 | `grep -ci 'no secret in any image layer' containers/secrets.md` | pass exit=0 | sha256:1121cfccd591 | 2026-10-05 | assay-verifier-app[bot] @ 3012e2bed680 (on-behalf-of human:ian) (forge-identity) |
| 4 | `sh containers/scripts/layer-secret-scan.test.sh` | pass exit=0 | sha256:ab6e61478cee | 2026-10-05 | assay-verifier-app[bot] @ 3012e2bed680 (on-behalf-of human:ian) (forge-identity) |
| 5 | `docker build -t assay-desk-base:dev containers/base && sh containers/scripts/layer-secret-scan.sh assay-desk-base:dev` | fail exit=1 | sha256:b02d91ad75a6 | 2026-10-05 | assay-verifier-app[bot] @ 3012e2bed680 (on-behalf-of human:ian) (forge-identity) |
| 6 | `statusgen --consumers --brief desk-containers/02 --root .` | fail exit=2 | sha256:d556ff5a51a3 | 2026-10-05 | assay-verifier-app[bot] @ 3012e2bed680 (on-behalf-of human:ian) (forge-identity) |
| 7 | `shellcheck containers/scripts/layer-secret-scan.sh` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-05 | assay-verifier-app[bot] @ 3012e2bed680 (on-behalf-of human:ian) (forge-identity) |

## Review
Gate: human (sensitive-data: yes — App-PEM custody design; see gate-why). Reviewer
answers both core-control questions: (1) the single control standing between a baked
credential and a public image is the contract+scan pair backed by the repo leak-sweep —
acceptable only if all three are independent; (2) the mutation row (Verify 4) proves
the lower layer fires with the upper (review) bypassed. Verdict + date in the stream
README table.
