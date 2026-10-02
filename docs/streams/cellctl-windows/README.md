---
stream: cellctl-windows
repo: medici-finance/assay
serves: assay
status: active
priority: P1
track: platform
issues: []
board: generated
---

# Native Windows cellctl

Deliver native Windows host lifecycle in Go, using **Orca or Herdr** for consoles and
**Docker Desktop with a Linux-container engine** for container cells. No Bash, tmux,
Git-Bash, Python or user-facing WSL command is required by these cellctl paths.
Docker's internal VM/backend is distinct from running the host launcher inside WSL.
PowerShell may invoke the executable and drive CI; orchestration and wrappers live in Go.
If a cockpit exposes only a command-string API, a bounded native-shell invocation of
the Go runner is permitted at that adapter boundary. It must not embed lifecycle logic
or interpolate user data; this is not a fallback to Bash or a second script implementation.

Supported kinds: `house` and `scrubbed` host cells, the local launcher portion of `k8s`
cells, and native `container` cells. Tests for `k8s` use inert local fixtures and never
contact a cluster; deployment/cluster administration is outside the Windows runtime claim.

This is the implementation plan requested on 2026-10-01. It is authoring work: every
brief below is `todo`, and no Windows runtime success is claimed by this PR.
Freshness: `origin/main` **1c67aa717**, including the merged Go container runtime
[#1981](https://github.com/medici-finance/assay/pull/1981)
(`d4a7a053a0acf392e72a9f772573ee48a243be03`).

## Scope and existing work

The [windows-port stream](../windows-port/README.md) already owns general installation,
release builds, pollers and hooks, and expressly excludes cockpit composition. This stream
owns the missing `cellctl` host/console/container runtime. Existing Windows locks, custody
checks, packaging and release targets are inputs to reuse, not work to repeat or unrelated
board-status dependencies. Source paths and dated gaps are recorded in each brief.

Orca and Herdr are assumed target products, **not presumed proven Windows APIs**. Record
actual supported versions and command contracts, then prove both. Native Windows amd64 is
the initial runtime acceptance target; arm64 remains explicitly build-only until its own
native runtime witness exists. Windows-container images, remote Docker daemons, new model
providers, fleet credential provisioning and comms-plane deployment are outside this scope.
Old external-launcher registrations remain compatibility paths with their own prerequisites.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 00 | [Go launch and session contracts for native Windows](brief-00-launch-session-contract.md) | 0 | M | implemented | — | — |
| 01 | [Go wrappers and native Windows cell environment](brief-01-go-wrappers-environment.md) | 1 | M | todo | — | — |
| 02 | [Native process ownership and lifetime on Windows](brief-02-windows-process-ownership.md) | 1 | M | todo | — | — |
| 03 | [Windows Orca and Herdr console adapters](brief-03-orca-herdr-adapters.md) | 1 | L | todo | — | — |
| 04 | [Windows-local Docker endpoint and runtime selection](brief-04-docker-local-endpoint.md) | 1 | M | todo | — | — |
| 05 | [Windows Docker bind paths and credential custody](brief-05-windows-docker-custody.md) | 2 | M | todo | — | — |
| 06 | [Integrate shell-free Windows host and Docker lifecycle](brief-06-integrated-cell-lifecycle.md) | 3 | L | todo | — | — |
| 07 | [Native Windows acceptance, packaged runtime proof and support documentation](brief-07-native-windows-proof.md) | 4 | M | todo | — | — |
<!-- statusgen:briefs:end -->

## Waves and critical path

- Wave 0: **00** freezes launch/session interfaces and ownership semantics.
- Wave 1: **01** wrappers/environment, **02** process ownership, **03** cockpit adapters,
  and **04** Docker endpoints can run concurrently in separate worktrees.
- Wave 2: **05** Docker filesystem/custody follows **04**; both touch the same engine package.
- Wave 3: **06** owns the CLI integration after **01**, **02**, **03** and **05**.
- Wave 4: **07** proves native Windows and packaged artifacts after **06**.

Structural critical path: **00 → 04 → 05 → 06 → 07**. The larger adapter/integration
units can also dominate elapsed time. The smallest unblocking move is the Go launch/session
contract, not another Windows cross-compile. Actual Windows cockpit capability and a Windows
Docker Desktop test host are external prerequisites for **03**'s capability evidence and
**07**'s runtime proof; provision the evidence environment while coding proceeds. If either
product lacks a usable native Windows API, record that blocker rather than substitute tmux.

## Fanout ownership

One worker per brief and one branch/PR per implementation. Wave 1 workers consume the
versioned contract; interface changes go back through 00's owner before consumers advance.
Only 06 rewires the shared up/down/status paths. 01 owns environment helpers in launch.go
but not lifecycle functions. 04 and 05 are serialized; new files alone do not make their
shared package safe to mutate concurrently. No intra-brief shard declaration is implied.

## Acceptance and design gates

Completion requires a native Windows `new → check → up → reconnect → status → down`
flow for both real cockpits and Docker, with no shell fallback, stable container identity,
retained work volumes and unrelated processes/terminals preserved. Host roles and container roles are each
proven through both actual cockpits. Cross-compilation,
unit fakes and Linux Docker runs are separate evidence, never substitutes. Security policy,
credential precedence, custody, model restrictions and immutable ownership checks survive.
Human-gated briefs remain todo until the approved design record required by brief-rules
exists; their concrete design questions are authored at the spec stage, not silently decided
by an implementation worker. Ordinary code preparation and draft PRs remain authorized.

Release publication and live credential/configuration changes are separate existing gates.
No workflow, production endpoint, credential or installed binary changes in this plan PR.
Candidate-artifact evidence permits implementation review; a public released-support claim
waits for an independent witness against an identified published archive and checksum.
