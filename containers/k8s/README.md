# Interactive Kubernetes desk package

This Kustomize base packages the **compatibility profile** of the five existing
role images. It is an offline-reviewed template, not a cluster installation or
an admitted managed cell. See [operator instructions](../../docs/docker.md#kubernetes-compatibility-profile)
and the [current architecture](../../docs/proposals/non-ci-architecture/28-interactive-kubernetes-desks.md).

Each role has one StatefulSet, one interactive container and one independently
provisioned `work-<role>-0` PVC. The base renders nine objects: one Namespace, one
unprivileged ServiceAccount, five StatefulSets and two NetworkPolicies. The five
PVC templates are inside their respective StatefulSets; Kubernetes creates the
claims. There is no Service, Ingress, operator workload or RBAC grant. The
StatefulSet `serviceName` supplies a stable identity field; no headless Service
is created, and this package does not rely on stable Pod DNS.

The image entrypoint still requires its App PEM and model credentials. Terminal
access therefore confers transitive custody of those credentials. This is not
untrusted-candidate isolation, and it cannot be attached through the future
managed operator. Human acceptance of desk-containers/02 is still required;
packaging review does not supply it. Agent-execution/09 owns managed boot and
isolation; desk-containers/28 owns the managed operator; /29 owns real-profile
qualification. A Running Pod does not acquire a role lease or authorize effects.

Each desk's container log is also a session transcript: `tty: true` sends terminal
output, including echoed input and anything a command prints, to the runtime log.
Treat logs as compatibility key-custody material even if a session normally prints
no secrets. On `assay-desks`, restrict `pods/log` together with `pods/attach`,
`pods/exec` and `pods/ephemeralcontainers` to principals authorized for the relevant
role's key custody; do not grant broader log-reader access. Protect node log files
and their readers on the same basis. Exclude this namespace from node log shipping,
or require every downstream collector, store, export/backup and reader to meet the
same custody boundary and retention/access controls. A generic shared log pipeline
that admits other readers is not an acceptable destination. NetworkPolicy and the
absence of a terminal Service do not restrict these log paths. Qualify the actual
logging configuration before installation; offline checks cannot establish it.

## Before installation

Supply an adopter-owned overlay and review its complete render.
[`overlays/compatibility-example/`](overlays/compatibility-example/) shows the
opt-in storage-class patch and endpoint policy shape using reserved documentation
addresses. It is never part of the base and is not a deployable configuration;
replace every placeholder in an adopter-owned copy before installation. The base's
external egress is **closed**. It permits only UDP/TCP 53 to selected kube-dns
Pods in kube-system. An operator must qualify the CNI and DNS selection (including
NodeLocal DNS if used), then add an explicit allowlist for **only** the intended
forge/model endpoints. Standard NetworkPolicy uses addresses, not DNS names;
maintain exact endpoint addresses, or use a separately qualified egress gateway
that enforces destination names. Do not replace this with an Internet-wide rule.
The base checks do not qualify an adopter's overlay or effective CNI behavior.

Other operator values are storage class, storage size, resource requests/limits,
namespace if customized, and the ten distinct credential Secret names. The
initial 20Gi, 250m/512Mi requests and 2 CPU/4Gi limits are packaging defaults,
not measured capacity promises. Supply an approved storage class with suitable
retention, backup and access controls. StatefulSet deletion and scale-down
retain PVCs; namespace deletion or explicit PVC deletion can still destroy data.

The process uses the pinned images' `desk` account at UID/GID 999 (pinned in the
base image build; see "Image account verification" below), with fsGroup 999
granting write access to `/work` and group read on the 0440 Secret volume. No privileged ownership-fix init
container is used. Validate CSI fsGroup handling and preflight readability before
admitting a profile. Kubernetes Secret files otherwise default to root ownership;
0400 alone would be unreadable to this non-root process. The mount remains
read-only, and only `app.pem` is projected.

`secrets.example.yaml` contains empty named skeletons for all ten Secrets, with
required keys listed only in comments; it is excluded from the build. These
skeletons are not deployable. Do not apply or populate that tracked file. Provision real Secrets
out of band using the [credential contract](../secrets.md); independently scoped
roles must not share a Secret reference **or actual credential value**. Static
checks can prove distinct references, not distinct key material inside live
Secrets. The model Secret carries the selected runtime model credential, endpoint
and model configuration. The generic examples use API-key mode; file-based WIF or
Vertex profiles need separate, reviewed runtime Secret mounts per that contract.

Pre-provision the retained work volume with the approved pinned agent CLI and
required work inputs. The legacy first-run npm installer is not run by these
manifests, and the forge/model/DNS network floor does not permit arbitrary package
registries. A retained shell does not install or start the agent automatically.

## Offline checks

From the repository root:

```sh
sh containers/k8s/check.sh --base
sh containers/k8s/check.sh --base --self-test
sh containers/k8s/check.sh --source-secrets --self-test
```

Required installed tools: Kustomize **v5.8.3**, kubeconform **v0.8.0**, gitleaks
**8.30.1** (the existing repository scanner pin), Python 3.9+ and the repository's
Go toolchain with its existing `gopkg.in/yaml.v3 v3.0.1` module cached. No tool is
installed by the checker. The small Go YAML-to-JSON adapter runs from the existing
desk module; Python's standard library performs the package assertions. No new
parser or package dependency is introduced. The scanner uses the repository's
existing `.gitleaks.toml`, redacts output, ignores inline allow directives and
also scans decoded Secret.data. Its staged positive controls prove that an
example omitted from Kustomize remains in the source inventory.

The checker sets `GOPROXY=off`, `GOSUMDB=off`, `GOTOOLCHAIN=local` and
`KUBECONFIG=/dev/null`. It preflights the base to admit only the seven local
resource files before calling Kustomize; remote bases, plugins, generators and
Helm are outside this base. Kubeconform receives only the committed, checksummed
local schemas, without a default/HTTP fallback. Missing tools/caches report
`could-not-check` and exit 2; rejected manifests or leaked credentials exit 1;
only completed clean checks exit 0. No kubectl or discovery call runs. These
checks prove packaging; runtime behavior, endpoint reachability, effective policy,
model usage and recovery remain for qualification.

The package source scan is independent of the repository-wide credential scan.
It scans the whole tracked/unignored package, including unrendered examples and
nonstandard filenames. It does not rely on a successful render. `testdata/mutations.py`
is the committed fail-first recipe: every planted invariant defect must be
rejected by the structured check, a wrong Kubernetes field type must be rejected
by kubeconform, and excluded synthetic credentials must fail the real scanner.

Tool/schema dependency assessment: [issue #3725](https://github.com/medici-finance/assay/issues/3725).
Schema source and upgrade procedure: [schemas/README.md](schemas/README.md).

## Image provenance

The five manifests consume the `linux/amd64` immutable role references from
[release v1.0.52](https://github.com/medici-finance/assay/releases/tag/v1.0.52),
`release-images.json`, SHA-256
`ff9237178e5bbf9523012a83e2366b99d2ff3cd0092c6116ad4c0dee08afdd64`.
Published source revision: `2718fc978b7454f0f2aa422e1236fb6ad543e0d2`.
The node selector matches that published platform.

[`release-images.json`](release-images.json) in this directory is that release
asset, kept **byte for byte**: it is the record the release build itself wrote
(schema `release-images-v1`, see [release images](../../docs/release-images.md)),
naming the umbrella version, the source revision it was built from and the
registry manifest digest of every image it pushed. Its SHA-256 is the one listed
for it in that release's `checksums.txt`. Each manifest's `image:` line cites it,
and `check.sh --base` binds them: it requires the record's bytes to hash to the
SHA-256 pinned in `check.py` (`RECORD_SHA256`, the value stated above, which
this README must also state), validates the record with the release's own
contract validator (`tools/release/images_contract.py`) and rejects any role
reference that is not exactly the one the record holds for that role (a digest no
release recorded, another role's digest, or a recorded digest under another
tag). `testdata/mutations.py` plants each of those at every role, plus a
fabricated digest written into both the record and one manifest, which only the
pin rejects. The check is offline and never contacts the release: the pin makes
a record edit change `check.py` and this README too, and a reviewer confirms that
value against the release's `checksums.txt`.

To update, from one selected release: download its `checksums.txt` and
`release-images.json`, confirm `sha256sum -c` passes for the record, replace this
file with the downloaded bytes unchanged, copy the five role references from it
into the manifests, update the release, SHA-256 and source revision above and
`RECORD_SHA256` in `check.py`, and repeat both offline checks. Never hand-edit the record: a record that is not the
release's own bytes no longer binds anything. No registry lookup occurs inside
`check.sh`; a well-formed digest is not a fresh image availability proof, and the
record states integrity and version alignment, not a signed build attestation.

### Image account verification

On 20 October 2026, all five published v1.0.52 digests named in these manifests
were checked in disposable local containers: the default account and explicit
`999:999` both resolved to `desk` in passwd/group, `/work` was owned by `999:999`,
and Node `os.userInfo()` and Python `pwd.getpwuid()` succeeded. A `1000:1000`
negative control failed Node user lookup with `ENOENT` in every image. No model
entrypoint, credentials, host mounts or container network were used. This verifies
the pinned image account; it does not qualify CSI or live Secret permissions.

`containers/base/Dockerfile` now pins the account: `groupadd --gid 999` and
`useradd --uid 999`, followed by a build step that fails unless `desk` resolves to
`999:999`. `tools/desk/internal/deskkit/deskuid_test.go` holds that Dockerfile
line and all five manifests' `runAsUser`, `runAsGroup` and `fsGroup` to the same
value on every PR, so 999 holds by construction for images built from that
Dockerfile. The v1.0.52 images above were built before the pin, when system-ID
allocation happened to yield 999; the check recorded above is what establishes
their account. Until the manifests move to a release built with the pin, and as a
cheap confirmation afterwards, acquire the exact reviewed digests explicitly and
run this local check for **every** role before adopting those references.
`--pull never` makes a missing local artifact a failure, not an implicit download:

```sh
for manifest in containers/k8s/base/*-desk.yaml; do
  image=$(awk '/^          image:/ { print $2 }' "$manifest")
  test -n "$image" || exit 1
  docker run --rm --pull never --platform linux/amd64 --network none \
    --read-only --cap-drop ALL --security-opt no-new-privileges \
    --user 999:999 --entrypoint node "$image" -e '
      const u = require("node:os").userInfo();
      const fs = require("node:fs");
      const work = fs.statSync("/work");
      if (u.username !== "desk" || u.uid !== 999 || u.gid !== 999 ||
          work.uid !== 999 || work.gid !== 999 || process.env.HOME !== "/work")
        process.exit(1);
      fs.accessSync("/work", fs.constants.W_OK);
      fs.accessSync("/usr/local/bin/desk-entrypoint", fs.constants.X_OK);
      console.log("checked-clean: " + process.env.ASSAY_DESK + " desk=999:999");
    ' || exit 1
done
```

Record each digest and the result. Missing artifacts or an account mismatch hold
adoption. Update the Dockerfile pin, `deskuid_test.go`, the manifests, the
checker's identity invariant, fixtures and this procedure together if the image
account changes. The static checker rejects
a different UID, GID or fsGroup at every role; it does not inspect registry images.
