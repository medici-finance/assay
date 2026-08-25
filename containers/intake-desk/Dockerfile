# syntax=docker/dockerfile:1

# =============================================================================
# assay intake-desk — a thin per-desk image over the shared desk-base.
#
# This layer adds nothing but the desk's IDENTITY and its BOOT: it declares
# which desk this is (ASSAY_DESK) and installs the shared fail-closed
# entrypoint. Everything else — Go, Python, git, gh, the desk-tools suite,
# statusgen, the baked assay plugin, the non-root 'desk' user, the /work
# volume — comes unchanged from desk-base. Published as
# ghcr.io/medici-finance/assay/intake-desk.
#
# NO credential is added here (none is added in the base either). The entrypoint
# enforces the runtime credential contract in containers/secrets.md at start-up;
# it never bakes a secret. See containers/README.md for the image-name decision.
#
# BUILD CONTEXT is the REPOSITORY ROOT (as for the base image), because the
# shared entrypoint lives at containers/entrypoint.sh — one level above this
# desk's directory:
#     docker build -f containers/intake-desk/Dockerfile -t assay/intake-desk:dev .
# The base image must exist locally as the tag named in BASE_IMAGE first:
#     docker build -f containers/base/Dockerfile -t desk-base:dev .
# =============================================================================

# The base image, version-locked. A desk image is ONLY ever built against a
# pinned base tag — never a floating 'latest'. The default is a local dev tag
# for a hand build; CI overrides it with the exact version/sha of the base it
# just built (see .github/workflows/docker-publish.yml).
ARG BASE_IMAGE=desk-base:dev
FROM ${BASE_IMAGE}

# This desk's public name. It is what the shared entrypoint reads to boot the
# right desk and surface its skill by name, and it is the one thing that differs
# between the five per-desk images.
ENV ASSAY_DESK=intake-desk

# The shared interactive boot: fail-closed credential preflight, then the desk
# identity + skill pointer, then the interactive session. One script, identical
# across all five desks (containers/entrypoint.sh). Its 0755 mode is carried
# from git, so this works with or without BuildKit.
COPY containers/entrypoint.sh /usr/local/bin/desk-entrypoint

ENTRYPOINT ["/usr/local/bin/desk-entrypoint"]
# Default command the entrypoint execs into: an interactive shell from which the
# operator invokes /intake-desk. Overridable at 'docker run'.
CMD ["/bin/bash"]
