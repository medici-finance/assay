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

FROM golang:1.25-bookworm

ENV CGO_ENABLED=0

# Shell tooling dependencies: bash for the tools/*.sh suites, python3 for
# the scanner's own helpers and the repo's python scripts, git because
# several tools shell out to it.
RUN apt-get update && apt-get install -y --no-install-recommends \
        bash git python3 ca-certificates \
    && rm -rf /var/lib/apt/lists/*

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
