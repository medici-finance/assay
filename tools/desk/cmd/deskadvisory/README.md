# deskadvisory

Recompute-at-the-gate verification for security-advisory fixes.

```
deskadvisory check <base-repo> <ghsa-id>
```

Exit: `0` ok · `3` disabled · `5` refused · `6` unverifiable.

Spec: `docs/design/advisory-fix-pipeline.md`.

A security fix developed under a GitHub Security Advisory gets *less* verification than a
typo PR: the temporary private fork (TPF) has no Actions, and no App can reach it. This tool
is the last check before a merge nothing can stop, so it stores no verdict — it recomputes
one, live, at the moment the human is about to merge.

## What it will not tell you

**It cannot gate the merge.** The advisory merge is an admin action; this tool prints a
verdict and exits. Its output informs a human's decision; it never takes it.

**A PASS is not "this fix is correct".** It is "these checks, over this many files, at this
SHA, reported nothing". The tool prints the coverage figure precisely so PASS cannot be read
as more than it is:

```
deskadvisory: PASS GHSA-… @ <sha> -- 5 check(s), 431 file(s) examined [ck-bash-default-expansion: 6 file(s); …]
```

**The check list is narrower than the repo's CI.** See the omissions table below. Anything
absent from the shipped check list is not checked, and this tool says nothing about it.

## Check-list definition

The definition is read from a **trusted source only**: the base repo's default branch
(`.deskadvisory.json` via the contents API), else a per-repo file embedded in this binary
(`checkdefs/<owner>/<repo>.json`, via `go:embed` — so an installed binary carries it and the
load does not depend on the process CWD). **Never** from the fetched TPF tree: a TPF copies
`validate.yml` from the base, so reading the list from the tree would let untrusted content
declare the checks that judge it.

### Fields

| field | meaning |
|---|---|
| `name` | check identifier, reported in the verdict |
| `tool` | executable resolved via `PATH` — never from the fetched tree |
| `args` | fixed argv; the tree is `cmd.Dir`, strictly input data |
| `invertExit` | grep-shaped "forbidden pattern" check: pass on exit **1** only |
| `requireFiles` | paths that must exist in the tree; **mandatory** |
| `minFiles` | floor on regular files under `requireFiles` (default 1, `.git` excluded) |
| `requireOutputMatch` | regexp the output must match for exit 0 to be a pass; **mandatory** for non-inverted checks |
| `note` | documentation; ignored by the runner |

### How exit codes become a verdict

The failure mode this tool most has to avoid is reporting PASS having examined nothing, so
every way a tool can finish is classified explicitly and the default is refusal.

| condition | verdict |
|---|---|
| a `requireFiles` path is absent | unverifiable (6) |
| `requireFiles` paths hold fewer than `minFiles` files | unverifiable (6) — the vacuous-pass guard |
| `invertExit`, tool exits **1** | pass — pattern absent |
| `invertExit`, tool exits **0** | fail — forbidden pattern found |
| `invertExit`, tool exits **≥ 2** or dies on a signal | unverifiable (6) — the guard malfunctioned; a `grep` with a bad regex says nothing about the tree |
| plain check, exit 0, output matches `requireOutputMatch` | pass |
| plain check, exit 0, output does **not** match | unverifiable (6) — the tool found nothing to examine |
| plain check, non-zero exit | fail |

`minFiles` and `requireOutputMatch` exist for the same reason: `grep` over an empty directory
exits 1 ("no match") and `kubeconform` over one exits 0 ("0 resource found in 0 file"). Without
a floor on work done, a directory that was renamed away — or emptied — reads as a clean tree.

## Shipped check list — `example-org/example-k8s`

Derived from `example-k8s` `.github/workflows/validate.yml`.

| check | tool | validate.yml step |
|---|---|---|
| `ck-bash-default-expansion` | grep | Forbid bash-default expansions in ConfigMap-shipped scripts |
| `ck-secret-denylist` | grep | Secret / denylist scan (denylist pattern) |
| `ck-credential-shaped` | grep | Secret / denylist scan (credential-shape pattern) |
| `kubeconform-base` | kubeconform | Kustomize build … + kubeconform, restricted to `base/` |
| `kubeconform-admin` | kubeconform | Kustomize build … + kubeconform, restricted to `admin/` |

### Deliberate omissions

Stated boundaries, not oversights. `deskadvisory` says nothing about these.

| omitted | why |
|---|---|
| the kustomize render (`kustomize build`) | `kustomize` reads `kustomization.yaml` **from the tree** — untrusted content controlling execution and resource resolution, which brief step 5 forbids |
| kubeconform over `deploy/` | without the render, `deploy/` holds pre-substitution `${VAR}` templates, not manifests; validating them as manifests is a category error and fails on `main`. `deploy/` is covered only by the grep-shaped guards |
| shellcheck | takes an explicit file list from `git ls-files '*.sh'`; the runner's fixed-argv model cannot express it |
| the placeholder / resolution guards | both read the **rendered** artifact, so they depend on the omitted render |

`doc_parity_test.go` fails if this section drifts from the embedded checkdef.
