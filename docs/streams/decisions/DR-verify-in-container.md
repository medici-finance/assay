---
id: DR-verify-in-container
date: "2026-09-21"
title: "The supported execution-witness runner on Windows is the pinned Linux harness container, driven by `statusgen verifyrun --in-container`, not a native Windows shell"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Make a native Windows shell the runner — teach verifyrun to run Verify rows under PowerShell/cmd (or require Git-for-Windows bash on every adopter). Ruled out: verifyrun's rows are POSIX `bash -o pipefail` commands whose quoting and exit/pipefail semantics PowerShell and cmd silently reinterpret (verifyrun.go already refuses to fall back to them for exactly this reason), and requiring Git-for-Windows bash on every host reintroduces the WSL-launcher fragility (#1418) the container removes. The witness would then be produced by a shell that runs the row differently from every Linux/macOS runner, so a green on Windows would not mean the same thing as a green anywhere else."
  - "Ship a second, Windows-specific harness image. Ruled out: the harness is Linux whichever host launches it; a Windows-container image cannot run the Linux desk-tools binaries, and building/maintaining a parallel Windows image doubles the supply-chain surface for no gain. The Windows value is that a Windows adopter USES the existing Linux image, not that a Windows-native image exists."
  - "Pin the harness image by tag (`:v1.0.6`) rather than by digest. Ruled out: a tag is mutable — the image it resolves can change under the pin, which is the floating-reference hazard `paired-versions.yaml` already forbids for the release binaries. A digest is immutable; the launcher runs the image ONLY by `<image>@sha256:<64hex>` and refuses a bare/`latest` tag."
  - "Put the launcher in a shell/PowerShell script the install skill places, instead of a `statusgen` sub-flag. Ruled out for THIS house on maintenance grounds: a `.sh` wrapper cannot run on native Windows (the target host), so it would need a `.ps1` twin kept in parity — the parity burden the stream elsewhere fights — and a PowerShell script cannot be hermetically unit-tested on the Linux/macOS CI host. A single cross-platform Go flag runs identically from PowerShell/bash/cmd, ships in the binary the adopter already has on PATH, and is fully testable with a fake `docker` on PATH. (Reasonable adopters may still prefer a script; this is a house maintenance choice, not a methodology invariant.)"
accepted:
  - "`statusgen verifyrun --in-container` is the supported way to run a brief's Verify table on native Windows. It composes a `docker run` that bind-mounts the checkout at /work and re-invokes `statusgen verifyrun` inside the pinned Linux harness container, where a real pipefail bash exists. The host statusgen does no Verify-row execution in this mode — it is a launcher — so verifyrun.go's invariant (only its own controlled subshell executes lifted markdown commands) is preserved."
  - "The harness image is pinned by TAG AND DIGEST in `plugins/assay/paired-versions.yaml`'s `harness:` block and run by its digest. The launcher REFUSES fail-closed on a `latest` tag, an absent digest, or a placeholder digest — an un-digest-pinned image is never run. The digest is a REGISTRY digest harvested online by a maintainer, never hand-invented; until it is harvested the committed placeholder keeps the launcher refusing, so nothing runs an unpinned image in the meantime."
  - "Credentials reach the container ONLY as an `--env-file <path>` passthrough (the operator role env-file, per `containers/secrets.md`). The launcher forwards the PATH and never reads, logs, or bakes its contents; no credential appears on the argv. On a POSIX host `--user <uid>:<gid>` maps container writes to the invoking user so the Evidence the container appends lands host-owned; on Windows there is no POSIX uid and `--user` is omitted (Docker Desktop maps ownership to the host user)."
  - "This runner is the DEFAULT for ordinary offline Verify rows on Windows. A row whose subject genuinely IS native-Windows shell behaviour uses the per-row `Shell` marker (#1427) as a narrow exception — never as the default path."
  - "Cost accepted: the CI proof of the witness LANDING is apply-gated — it needs a Docker + Linux-container runner and the harvested registry digest — and the native-Windows-host proof (Docker Desktop on Windows with a Linux-container backend) is BLOCKED for want of such a runner on GitHub-hosted `windows-latest`. Both are recorded as held/apply-gated rather than faked, consistent with the stream's `blocked is a state` contract."
---

**Status:** draft

**This record is NOT approved** (draft — awaiting the driver's ruling). No design-approval ruling has been recorded. It is proposed for the
driver (`human:<name>`) to weigh; when a ruling is made, the decision-issue reference and the outcome
are recorded here, and `decided-by` above is confirmed. It is cited by `windows-port/10`, which is
`gate: human` and stays at `implemented` until the driver rules — the human gate the brief's
`## Human decision` section enumerates (the digest harvest and the staged-copy landing-mechanism
choice given `desk-supervision/12`'s in-flight retirement of that pattern).

## The decision

windows-port/00–05 made a pinned Assay release installable and CI-proven on Windows, but left no way
to RUN a brief's Verify table there: `statusgen verifyrun` executes each row under `bash -o pipefail`,
and on native Windows that shell is often the WSL launcher shim, which exits before the row runs
(#1418). verifyrun records that honestly as could-not-run — but the row still did not run.

This record fixes the RUNNER, not verifyrun's execution model: the supported execution-witness runner
on Windows is the pinned Linux harness container, driven by `statusgen verifyrun --in-container`. The
alternatives above (a native-shell runner, a Windows-specific image, a tag-only pin, a placed script)
were weighed and declined for the reasons stated. The `accepted:` list is the contract the brief's
launcher, its `harness:` pin, its CI leg, and its adoption-doc delta all implement.
