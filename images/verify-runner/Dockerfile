# The release's own statusgen bytes plus the fleet's digest-pinned toolchain.
#
# PLATFORMS - linux/amd64 and linux/arm64 (#4323). The release builds this file
#   once per platform and publishes one image index over both, so an arm64 host
#   runs it natively instead of under amd64 emulation. RUNNER_BASE is a
#   multi-platform index digest, so each build takes that platform's base. The
#   tool checksums below are pinned per TARGETARCH and any other arch fails the
#   build. statusgen is the release asset statusgen-linux-${TARGETARCH}, staged
#   by tools/release/images.py, so a platform without one fails at its COPY.
#
# GIT FLOOR - git 2.41 or later at /usr/bin/git (#4207). The admitted verify
#   runner calls /usr/bin/git with a fixed PATH and renders a converted file from
#   the attested commit's own attributes via `git --attr-source`, which arrived in
#   git 2.41. The bookworm git is 2.39.5, so this image builds git from the
#   upstream release tarball in the `gitbuild` stage, from the SAME pinned
#   version and sha256 as containers/base/Dockerfile (#2318): one git for the
#   fleet, not a second pin that can drift. Only the install tree reaches the
#   final image, over the distro git, which is purged. The last RUN of the final
#   stage runs containers/scripts/git-floor-check.sh with the runner's fixed
#   PATH, so a git below the floor fails the build instead of shipping.
#   tools/desk/internal/deskkit/verifyrunner_gitfloor_test.go holds the pair to
#   the base image's and holds the check in place.
#
# One reference for both stages: git is built against the libraries of the very
# image it ships in, so the two stages cannot be bumped apart.
ARG RUNNER_BASE=golang:1.27.3-bookworm@sha256:972b4110ba0f47539a975c31e52c64765e1ff19f2d74469ae4a595c6676e80a5

# ---- git, built from the fleet's pinned upstream release -----------------
# Its own stage, so the -dev packages never reach the final image. The tarball
# is checked against the pinned sha256 before it is unpacked. The pair below is
# copied from containers/base/Dockerfile; to bump it, follow the "To bump" note
# above that file's gitbuild stage and change both Dockerfiles in one change.
FROM ${RUNNER_BASE} AS gitbuild
ARG GIT_VERSION=2.56.0
ARG GIT_TARBALL_SHA256=26c56c296b38c0695b26fa95f475f1d01704d2d38e73465ca30b0b2f5dc789d3
ENV DEBIAN_FRONTEND=noninteractive
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        gcc \
        libc6-dev \
        libcurl4-gnutls-dev \
        libexpat1-dev \
        libpcre2-dev \
        make \
        xz-utils \
        zlib1g-dev; \
    rm -rf /var/lib/apt/lists/*
RUN set -eux; \
    curl -fsSL --proto '=https' --proto-redir '=https' \
        "https://www.kernel.org/pub/software/scm/git/git-${GIT_VERSION}.tar.xz" -o /tmp/git.txz; \
    echo "${GIT_TARBALL_SHA256}  /tmp/git.txz" | sha256sum -c -; \
    tar -C /tmp -xJf /tmp/git.txz; \
    cd "/tmp/git-${GIT_VERSION}"; \
    # prefix=/usr: the runner calls /usr/bin/git, and the install tree replaces
    # the distro git file for file. NO_OPENSSL: hashing uses git's own
    # sha1dc/sha256 code and https goes through libcurl (GnuTLS), so no OpenSSL
    # is linked or shipped. NO_RUST: Rust is still optional in this release, so
    # no cargo toolchain is fetched.
    opts="prefix=/usr NO_OPENSSL=YesPlease NO_RUST=YesPlease NO_GETTEXT=YesPlease NO_TCLTK=YesPlease NO_PERL=YesPlease NO_PYTHON=YesPlease USE_LIBPCRE2=YesPlease"; \
    make -j"$(nproc)" $opts all; \
    make $opts DESTDIR=/out install; \
    /out/usr/bin/git version

# ---- The runner ------------------------------------------------------------
FROM ${RUNNER_BASE}
# Dependency admission and update policy: issue #3469. Install only at build time.
# Exact direct versions fail closed if the archive changes; review a pin update.
# The distro git (2.39.5) is purged here, before packages.txt is written, so that
# file does not list a git the image does not run; the source-built git below is
# not a Debian package and is not listed in it. Its runtime libraries (libcurl3-
# gnutls, libexpat1, libpcre2-8-0, zlib1g) stay installed: purge, without
# --auto-remove, removes only the named packages.
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      python3-yaml=6.0-3+b2 \
      python3-jsonschema=4.10.3-1 \
 && apt-get purge -y git git-man \
 && mkdir -p /usr/local/share/verify-runner \
 && dpkg-query -W -f='${binary:Package}=${Version}\n' \
      > /usr/local/share/verify-runner/packages.txt \
 && rm -rf /var/lib/apt/lists/*

# Rows run with no network and a read-only root, so everything they resolve is
# provisioned here, at build time only.
#
# Offline packaging-check tools, at the exact versions containers/k8s/check.py
# requires. Tool assessment: issue #3725. Each archive is compared with the
# sha256 pinned below before it is unpacked; a mismatch, an unpinned platform or
# a version other than the pinned one fails the build.
ARG TARGETARCH
RUN set -eu; \
    case "${TARGETARCH:-}" in \
      amd64) gl_arch=x64 \
             kustomize_sha=cb9e31198d3f63b44848bd0afc4c8efc56e6cfef9c93a4a39a211940e6decd61 \
             kubeconform_sha=9bc2bffbf71f261128533edaf912153948b7ff238f9a531ae6d34466ec287883 \
             gitleaks_sha=551f6fc83ea457d62a0d98237cbad105af8d557003051f41f3e7ca7b3f2470eb ;; \
      arm64) gl_arch=arm64 \
             kustomize_sha=9867b76482cfc6d25b546abc1d2b5a32bd137505aba640649b4b378f64f6d68f \
             kubeconform_sha=1f53fc8e81258197a35e8603054162a5af1de8c5af13746c71ab680d9534ed87 \
             gitleaks_sha=e4a487ee7ccd7d3a7f7ec08657610aa3606637dab924210b3aee62570fb4b080 ;; \
      *) echo "verify-runner: no pinned tool checksums for '${TARGETARCH:-}'" >&2; exit 1 ;; \
    esac; \
    fetch() { \
      curl -fsSL --proto '=https' --tlsv1.2 -o /tmp/tool.tar.gz "$1"; \
      echo "$2  /tmp/tool.tar.gz" | sha256sum -c -; \
      tar -xzf /tmp/tool.tar.gz -C /usr/local/bin --no-same-owner "$3"; \
      chmod 0755 "/usr/local/bin/$3"; \
      rm -f /tmp/tool.tar.gz; \
    }; \
    fetch "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv5.8.3/kustomize_v5.8.3_linux_${TARGETARCH}.tar.gz" \
      "$kustomize_sha" kustomize; \
    fetch "https://github.com/yannh/kubeconform/releases/download/v0.8.0/kubeconform-linux-${TARGETARCH}.tar.gz" \
      "$kubeconform_sha" kubeconform; \
    fetch "https://github.com/gitleaks/gitleaks/releases/download/v8.30.1/gitleaks_8.30.1_linux_${gl_arch}.tar.gz" \
      "$gitleaks_sha" gitleaks; \
    test "$(kustomize version)" = v5.8.3; \
    test "$(kubeconform -v)" = v0.8.0; \
    test "$(gitleaks version)" = 8.30.1

# Go module cache for tools/desk (which also holds the gopkg.in/yaml.v3 module
# the containers/k8s YAML adapter runs on) and for statusgen, which tools/desk
# tests build from source. Only module files are copied; the downloads are
# checked against each module's go.sum. Rows need no proxy, and the cache stays
# read-only and world-readable for a non-root row user.
#
# The cache matches the go.mod files of THIS build's source. A tree verified
# later may require versions this cache does not hold, and offline rows cannot
# fetch them (#4683). go-module-roots.txt records the roots baked here, one per
# line; a container-netns `statusgen verifyrun` reads it, compares each root's
# go.mod in the verified tree with this cache, and records a row that stops on
# a missing module as could-not-run, naming the module and both versions.
# Every root downloaded below is listed in it, and only those.
ENV GOMODCACHE=/go/pkg/mod
COPY tools/desk/go.mod tools/desk/go.sum /tmp/modsrc/tools/desk/
COPY statusgen/go.mod statusgen/go.sum /tmp/modsrc/statusgen/
COPY comms/go.mod /tmp/modsrc/comms/
COPY deskcore/go.mod /tmp/modsrc/deskcore/
COPY forgeread/go.mod /tmp/modsrc/forgeread/
COPY gitquiet/go.mod /tmp/modsrc/gitquiet/
COPY perfrec/go.mod /tmp/modsrc/perfrec/
RUN set -eu; \
    export GOWORK=off GOFLAGS=-mod=readonly GOTOOLCHAIN=local; \
    cd /tmp/modsrc/tools/desk; \
    go mod download; \
    go mod download gopkg.in/yaml.v3; \
    cd /tmp/modsrc/statusgen; \
    go mod download; \
    cd /; \
    mkdir -p /usr/local/share/verify-runner; \
    printf '%s\n' tools/desk statusgen > /usr/local/share/verify-runner/go-module-roots.txt; \
    rm -rf /tmp/modsrc /root/.cache

COPY .release-images/verify-runner/statusgen-linux-${TARGETARCH} /usr/local/bin/statusgen

# git from the gitbuild stage, over /usr. The check script stays in the image so a
# running home can re-check itself.
COPY --from=gitbuild /out/usr/ /usr/
COPY containers/scripts/git-floor-check.sh /usr/local/bin/git-floor-check

# The cluster backend's fixed entrypoints. An admission policy that pins each
# container's command (or the absence of one) to a single script path can only
# admit a Job whose scripts are already in the image, so they ship here, at a
# fixed path, root-owned and not writable by the non-root row user. Each one
# prints its usage and exits 2 when run with no argument; check.py proves that
# by executing each path directly. The image's ENTRYPOINT and CMD are unchanged.
COPY images/verify-runner/stage-source.sh images/verify-runner/stage-cache.sh \
     images/verify-runner/witness.sh /usr/local/bin/
RUN set -eu; \
    for f in stage-source.sh stage-cache.sh witness.sh; do \
      chown 0:0 "/usr/local/bin/$f"; \
      chmod 0755 "/usr/local/bin/$f"; \
      test "$(head -n 1 "/usr/local/bin/$f")" = '#!/bin/bash'; \
    done

# GIT FLOOR CHECK: the last RUN of the final stage, so no later layer can bring in
# a git unchecked. The test -x lines make a missing binary fail (ldd alone would
# print its error to stderr and let the grep pass); the ldd gate catches a shared
# library the image stopped providing (git-remote-http carries the libcurl link);
# the owner gate keeps the install tree root-owned; the last test proves the
# distro git's exec directory is gone. git-floor-check then proves every git on
# PATH is 2.41 or later and that `--attr-source` renders a converted file from a
# committed tree. It runs with the admitted runner's own fixed PATH, so it is the
# git the runner reaches that is checked, not whichever one this image's PATH
# finds first.
RUN set -eux; \
    test -x /usr/bin/git; \
    test -x /usr/libexec/git-core/git-remote-http; \
    if ldd /usr/bin/git /usr/libexec/git-core/git-remote-http | grep 'not found'; then \
        echo "source-built git is missing a shared library" >&2; \
        exit 1; \
    fi; \
    if find /usr/bin/git /usr/libexec/git-core /usr/share/git-core -xdev \( ! -user 0 -o ! -group 0 \) -print | grep .; then \
        echo "the source-built git install tree holds files not owned by root:root" >&2; \
        exit 1; \
    fi; \
    test ! -e /usr/lib/git-core; \
    PATH=/usr/bin:/bin:/usr/sbin:/sbin /usr/local/bin/git-floor-check
