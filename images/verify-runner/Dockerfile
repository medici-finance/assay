# The release's own statusgen bytes plus the fleet's digest-pinned toolchain.
FROM golang:1.27.2-bookworm@sha256:5cf287a799e6b94384bad13d16b14904c531f51ba65792237e122ce42b392f61
# Dependency admission and update policy: issue #3469. Install only at build time.
# Exact direct versions fail closed if the archive changes; review a pin update.
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      python3-yaml=6.0-3+b2 \
      python3-jsonschema=4.10.3-1 \
 && mkdir -p /usr/local/share/verify-runner \
 && dpkg-query -W -f='${binary:Package}=${Version}\n' \
      > /usr/local/share/verify-runner/packages.txt \
 && rm -rf /var/lib/apt/lists/*
COPY .release-images/harness/statusgen /usr/local/bin/statusgen
