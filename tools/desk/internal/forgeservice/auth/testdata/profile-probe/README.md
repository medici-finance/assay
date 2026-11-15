# Local profile primitive probe

Task1 environment qualification only. This helper contains no signing key, grant,
caller authority, current registry or production launcher profile. Its success
cannot satisfy authentication or custody acceptance.

Declared local build: existing Go1.27.2 darwin/arm64 toolchain, CGO disabled,
GOOS=linux, GOARCH=arm64, local toolchain/module cache only, trimpath and no VCS
metadata. Record the actual Go executable, source, output binary, Dockerfile and
local image digests. This qualifies only that exact build/platform. The separate
Ubuntu qualification must build/qualify its actual target independently.

The declared recipe is the adjacent Dockerfile. FROM scratch uses no registry or
third-party base. It fixes UID/GID501:501 and the entrypoint and declares no ENV or
VOLUME. Owned context contains only the compiled helper and explicit seed files
in private/channel directories. Those directory seeds deliberately test actual
named-volume initialization rather than assuming its ownership behavior.

Use an explicitly validated owned local engine, ignoring ambient endpoint/builder
overrides. No pull, push, engine-socket mount, privilege, provider credential,
production endpoint or unrelated resource contact is permitted. Fixed fixture
runtime: read-only rootfs, all capabilities dropped, no-new-privileges, no network,
separate namespaces and only the exact named-volume views. The producer gets
private/channel RW; client gets channel RO; sibling gets neither. Each receives
only a bounded tmpfs and explicit finite fixture CPU/memory/process limits.

The serve mode proves its marker exists/readbacks before readiness and creates an
AF_UNIX endpoint. The client proves the private marker is absent in its namespace,
channel file writes fail and socket roundtrip works through the RO directory view.
The sibling proves both resources are excluded. Separate actual containers are
required. No permission shape, same-UID file mode or mocked socket is a pass.

The fixed 45-second server and 5-second peer windows bound this disposable probe;
they are fixture timings, not proposed authority lease/revocation defaults. Any
unavailable image, architecture, volume ownership, channel or resource observation
is retained as could-not-check/failure. Do not infer auth/custody from this helper.


Observed prerequisite result: the exact source-bound linux/arm64 probe succeeded
through the explicitly selected same-daemon BuildKit docker driver. Two classic
builder attempts timed out and remain unqualified. A third attempt correctly
held on an image environment assertion: although the Dockerfile has no ENV, the
engine inserts its standard PATH. The successful primitive run pinned exactly
that observed baseline, not arbitrary image environment, and changed no legacy
validator or actual authority profile. The namespace/volume/socket observations
and complete owned-resource cleanup are summarized in the canonical Task1 spec.
No signing/caller/current-state/auth-custody or Ubuntu CI credit follows.
