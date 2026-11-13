# Anthropic OSS Scanner build image for medici-finance/assay.
# Enrollment: github.com/anthropics/oss-scanner, projects/assay/.
#
# Contract (per the oss-scanner templates):
#   - COPY . /src puts the repository checkout at /src; WORKDIR /src.
#   - Network is available ONLY during `docker build` — every dependency
#     must be fetched here; the scanner agent runs fully offline after.
#   - The image build MUST succeed; a failing build emails the primary
#     contact and stalls scanning.
#   - Tests run at build time but must not abort the build on failure —
#     warn and continue, so a flaky test never blocks the scanner.
#
# This repository carries 20 independent Go modules (14 under tools/, 6
# top-level; no go.work), plus shell tooling and a Claude Code plugin
# bundle. There is no single root module, so the build loops over every
# module.
#
# LOCAL-BUILD CAVEAT: with no .dockerignore in this repository, building
# from a working checkout copies everything in the context — including
# .git/config and ignored local state that may hold credentials
# (.gitignore:37-44). Build from a CLEAN CLONE when checking locally, and
# never share a locally built image. In the scanner's environment the
# context is always a fresh clone.

# Git release archive verified against the upstream maintainer signature.
# Keep the version and checksum together when upgrading.
FROM golang:1.27.3-bookworm AS gitbuild
USER root
ARG GIT_VERSION=2.56.1
ARG GIT_TARBALL_SHA256=f0b0bf89264b9bd2e1c640363e7529643989da76281600473c50745cd34a607f
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends ca-certificates curl gcc libc6-dev libcurl4-gnutls-dev libexpat1-dev libpcre2-dev make xz-utils zlib1g-dev; \
    rm -rf /var/lib/apt/lists/*
RUN set -eux; \
    curl -fsSL --proto '=https' --proto-redir '=https' \
        "https://www.kernel.org/pub/software/scm/git/git-${GIT_VERSION}.tar.xz" -o /tmp/git.txz; \
    echo "${GIT_TARBALL_SHA256}  /tmp/git.txz" | sha256sum -c -; \
    tar -C /tmp -xJf /tmp/git.txz; \
    cd "/tmp/git-${GIT_VERSION}"; \
    opts="prefix=/usr gitexecdir=/usr/lib/git-core NO_OPENSSL=YesPlease NO_RUST=YesPlease NO_GETTEXT=YesPlease NO_TCLTK=YesPlease NO_PERL=YesPlease NO_PYTHON=YesPlease USE_LIBPCRE2=YesPlease"; \
    make -j"$(nproc)" $opts all; \
    make $opts DESTDIR=/out install; \
    test "$(/out/usr/bin/git version)" = "git version ${GIT_VERSION}"

FROM golang:1.27.3-bookworm

ENV CGO_ENABLED=0

# CVE-2026-98432 stopgap: Go >= 1.24 opens MPTCP listeners by default; turn
# them off. A fragile stopgap only; patched node kernels are the real fix.
ENV GODEBUG=multipathtcp=0

# Shell tooling dependencies: bash for the tools/*.sh suites, python3 for
# the scanner's own helpers and the repo's python scripts, git because
# several tools shell out to it.
RUN apt-get update && apt-get install -y --no-install-recommends \
        bash python3 ca-certificates libcurl3-gnutls libexpat1 libpcre2-8-0 zlib1g \
    && apt-get purge -y git git-man \
    && rm -rf /var/lib/apt/lists/*

# Replace the distro Git executable and helper tree together.
COPY --from=gitbuild /out/usr/ /usr/
RUN set -eu; \
    test "$(/usr/bin/git version)" = "git version 2.56.1"; \
    test -x /usr/lib/git-core/git-remote-https; \
    ldd /usr/lib/git-core/git-remote-https > /tmp/git-ldd; \
    if grep 'not found' /tmp/git-ldd; then exit 1; fi; \
    rm /tmp/git-ldd

COPY . /src
WORKDIR /src

# Build every Go module. A hard build failure fails the image build — that
# is the signal the scanner relies on to know the tree is buildable.
RUN set -eu; \
    for gomod in $(find . -name go.mod -not -path './.git/*' | sort); do \
        d=$(dirname "$gomod"); \
        echo "=== go build: $d"; \
        ( cd "$d" && go build ./... ); \
    done

# Vet is cheap and catches a different class than build; advisory only.
RUN set -u; \
    for gomod in $(find . -name go.mod -not -path './.git/*' | sort); do \
        d=$(dirname "$gomod"); \
        ( cd "$d" && go vet ./... ) \
            || echo "WARNING: go vet reported findings in $d"; \
    done

# Test suites: run but never abort the image build (scanner-template
# convention — a red test suite must not stall scanning). Parts of the
# tools/desk suite expect a desk operator environment (roster config, git
# identity, credentials) and fail closed without it — see
# .oss-scanner/threat_model.md §"How to exercise it".
RUN set -u; \
    failed=0; \
    for gomod in $(find . -name go.mod -not -path './.git/*' | sort); do \
        d=$(dirname "$gomod"); \
        ( cd "$d" && go test ./... ) || failed=1; \
    done; \
    if [ "$failed" -ne 0 ]; then \
        echo "WARNING: go test reported failures in one or more modules"; \
    fi

# Shell entry points: syntax-check every checked-in script, advisory only.
RUN set -u; \
    failed=0; \
    for f in $(find . -name '*.sh' -not -path './.git/*' | sort); do \
        bash -n "$f" || failed=1; \
    done; \
    if [ "$failed" -ne 0 ]; then \
        echo "WARNING: bash -n reported syntax errors"; \
    fi
