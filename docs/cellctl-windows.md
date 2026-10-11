# Windows cellctl launch contract

Command-line forms (flags, precedence, exit codes) are listed in [the `cellctl` command reference](cellctl-cli-reference.md); what changed from the earlier hand-written parser is in [the compatibility table](cellctl-cli-compat.md).

The `internal/celllaunch` Go package defines the versioned handoff for native Windows
work in [cellctl-windows/00](streams/cellctl-windows/brief-00-launch-session-contract.md).
It is not connected to production `cellctl` yet. The current implementation supplies
parsers, validation, immutable permits and interfaces, plus a compiled test runner and
child. It supplies no Windows process supervisor, production credential resolver,
filesystem store, cockpit adapter or Docker transport. Native Windows runtime support
remains unproven until [brief 07](streams/cellctl-windows/brief-07-native-windows-proof.md).

## Launch data and authority

`LaunchSpec`, schema `cell-launch-v1`, contains a launch generation, cell, role, target
OS, host/container intent, cockpit, operation, absolute executable, exact argument
vector, nonsecret environment, working directory and typed credential references.
The host operations are `harness` and `deskd`; Linux-container presentation uses
`container-console`. Windows accepts Orca/Herdr, never tmux. Windows paths in this
initial contract must be absolute local drive paths; UNC paths, drive-relative paths
and device namespaces need a separate policy before support is claimed.

The nonsecret environment has an explicit name vocabulary. Environment-name aliases
such as `PATH` and `Path` cannot coexist. A vocabulary is not a secret scanner: a token
placed in an argument or under `LANG` cannot be recognised from its bytes. Trusted
callers must construct these values from known nonsecret sources. Credential references
carry a closed kind, forge name or model catalog profile identifier, and the same role
as the launch. Existing profiles such as `anthropic`, `glm`, `kimi` and `codex` remain
references to the existing provider catalog; this package does not add providers or
prove that a named profile exists. Permit construction/resolution must establish that.
References cannot carry a
token, arbitrary path, command or another role. The downstream runner resolves them
through existing role authority just before execution; tokens never enter durable
specifications, console command strings, logs or the session record.

`NewPermit` freezes a separately prepared expected launch, including executable,
operation, arguments, environment and credential references. `Permit.Check` requires an
exact match. Building the permit from the same untrusted bytes being checked defeats
that boundary and is forbidden. The constructor does not discover authority on its own.
Permits are in-memory values, not a second manifest decoded beside an untrusted spec.

`ParseLaunch` and `ParseSession` reject unknown schema/fields, duplicate JSON members,
case aliases of typed fields, trailing JSON, invalid input UTF-8, wrong value types and
oversized records (128 KiB). Arguments/environment are explicit collections; optional
empty collections are omitted. Whitespace and object-member ordering are immaterial.
`LaunchSpec.MarshalJSON` validates and enforces the same size limit. Errors omit source
values. Parsing foreign-platform plans is permitted; execution calls `NativePaths` on
the target OS and refuses a platform mismatch.

## Private handoff — specified, not implemented here

`SpecRef` holds a safe opaque identifier and a SHA-256 digest of the approved bytes.
It contains no directory, filename extension, reserved DOS device basename, path traversal, Windows alternate stream
or absolute-path escape. `RunnerRequest` passes only a separately approved runner
executable, this reference and cell/role/OS to the console. Its structural `Validate`
method cannot grant authority to the runner executable; the caller must independently
bind that executable to its approved installation.

The `Store` interface contract requires downstream implementations to:

1. Start from a separately trusted cell-owned private directory, never a path supplied
   in a record. Establish owner identity and owner-only access with platform custody
   checks, including Windows SID/DACL checks rather than synthetic POSIX mode bits.
2. Reject links, reparse aliases and escaped roots; bind validation to the actual opened
   file and root identities. An earlier path check alone cannot prove a later open safe.
3. Publish a new immutable generation by writing a private temporary sibling, syncing,
   closing and atomically exposing it. Never overwrite an existing generation. Readers
   see a complete record or no published reference; a failed publication preserves
   previous records and cleans only its own temporary file.
4. Verify custody before reading bytes, then call `BindLaunch` with the independently
   known reference and permit. The digest proves byte identity, not filesystem custody.

The production store/runner belongs to briefs 01/02. The fixture runner deliberately
uses stdin for the record and a separate test-driver permit. It exercises the
serialization/authorization/exec boundary, not NTFS permissions or atomic publication.
Same-user hostile code can generally exercise the operator's authority; this protocol
does not claim to sandbox the operator or an administrator.

Prefer a cockpit argv API. If its API accepts command text only, the adapter encodes just
the approved Go runner invocation and opaque reference using that exact backend's
Windows contract. It must not splice model names, repository paths, user text or a
serialized environment into executable shell text. A generated Bash/PowerShell
lifecycle program is outside the design.

## Session and resource lifetime

`SessionRecord`, schema `cell-session-v1`, records cell/role/generation/OS, intent,
spec reference, transaction state and distinct console/workload identities. `ready`
requires a console and exactly one workload kind. `pending` and `exited` may preserve
partial handles as diagnostics; a malformed present handle still fails validation.

- Console identity is backend, workspace, exact terminal handle and launch generation.
- Host process identity is PID, OS creation identity, executable, owner and owned tree.
- Container identity is frozen local endpoint, engine identity, full immutable container
  ID, cell/role labels and the expected isolation-plan digest. Endpoint selection,
  image/mount validation and digest construction remain the engine adapter's work.

Each adapter inspects only its resource and returns `owned`, `missing`, `foreign` or
`unknown`, with observed identity and established scope. It never claims to reconstruct
another adapter's resource or the entire session record. `MatchConsole`, `MatchProcess`
and `MatchContainer` compare that evidence independently, including during partial-start
rollback or container recovery when its console is gone. `Match` is a coordinator-only
comparison of an already assembled ready snapshot; it is not an adapter API.

Neither a record nor a successful earlier comparison authorizes a destructive operation.
Attach/close/interrupt/terminate must revalidate identity inside the operation, immediately
before acting. An absent/unreadable resource is not owned. PID reuse, another engine,
different full container ID, changed generation or an unrelated terminal refuses the act.
No process-name substring match or worktree-wide terminal close is permitted.

Lifecycle expectations for the downstream implementations:

| Event | Required behavior |
|---|---|
| Repeated `up` | Reconnect the owned generation after inspection; never create a duplicate. |
| Model/harness override | Record the selected launch; status/down manage that selection. A different requested selection requires explicit restart. |
| Cockpit selection changes | Use the recorded backend/handle for existing resources. |
| Partial creation | Retain diagnostics; roll back only handles created by that attempt and independently proved owned. |
| Console or app closes | Separate presentation loss from workload stop. A Docker container and its volume survive; attach cancellation never implies Docker stop. |
| Coordinator console crashes | Preserve or safely recover exclusion independently of the console; unknown ownership refuses a competing launch. |
| Host shutdown | Request graceful exit, wait a bounded interval, then terminate only the verified owned tree. |
| `down` | Validate each role independently; aggregate failures, preserve unrelated resources and named work volumes. |

## Proposed design decision — not approved

The proposed choice is a Go-owned launch record and independently bound permit, distinct
console/workload identities, and an atomic store accessible only to the owner. Alternative designs are
direct shell command strings (quoting and environment coupling), PID-only state (reuse),
and a console lifetime lock for container ownership (lost exclusion after console exit).
The accepted costs proposed for review are a versioned store, explicit recovery semantics
and platform custody/process implementations. There is no approved design-decision record
in this document. The human-gated briefs must link a ratified `design:` record before
implementation; the credential resolver, NTFS publication and process supervision
alternatives remain subject to that review.

## Capability and evidence checklist

For **each** real native Windows Orca/Herdr version, capture supported commands or API
schema, argv versus command-text encoding, exact returned handles, ownership inspection,
single-terminal close, reconnect and app-restart behavior. An unavailable Windows build
or unsupported capability is a named blocker, not permission to substitute tmux/WSL.

For Docker Desktop capture version, local named-pipe/explicit-context selection, Linux
engine identity/platform, image identity and observed Windows bind-source normalization.
Prove credential containment, isolation checks, container ID stability and volume retention
on that real environment; a Linux-host fixture cannot supply this evidence.

The current compiled fixture proves exact argv/environment/cwd and child exit propagation
on the OS where it is run, plus refusal of malformed or unauthorized records. It does not
prove any third-party API. The `celllaunch-windows` pull-request workflow runs the three
contract witnesses on native Windows amd64 at the exact PR head, records the platform and
toolchain, and requires each selected PASS with no skips. Its logs establish only those
contract runs, not production lifecycle support. Native Windows acceptance must separately run host-process and
Docker lifecycles through each real cockpit with Unix shells and lifecycle-script fallbacks
unavailable, preserving only the bounded native-shell adapter invocation described above, and repeat
with checksum-verified packaged artifacts. Record source SHA, OS/architecture, versions,
command outputs and immutable identities. ARM64 remains build-only without its own native
runtime witness. The staged workflow and release evidence belong to brief 07.
