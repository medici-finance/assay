# Local performance context

The local recorder remains off unless the operator selects ASSAY_TELEMETRY=1.
A selected tool or workflow profile uses perf-v3; absent or off profile retains
the existing producer schema. No profile uploads data or grants authority.
The field definitions, bounds and consent boundary are in
[the attribution specification](performance-attribution-spec.md).

The shared Cobra adapter registers the canonical flags and resolves them from
real parsed pflag state. Options.Performance receives one immutable context
before handlers. A producer uses PerfRunMeasured and selects that context on its
own measurement session before effects; the same typed session can reach an
existing provider observation seam. ProcessEntry remains the timing boundary.
No package-global work context is installed. Current Go FlagSets use
cli.GoPerformance; the current claim-ref switch uses cli.ManualPerformance.
Their parser exemptions, claims, custody and operational DESK_LOOP remain intact.

Flags override only named environment sources. Present empty clears; repeated
--perf-work values replace the entire environment array. Each work flag is a
bounded JSON object, as defined by perfrec. Invalid optional values are discarded
without echoing them or changing the operation's output or exit. Workflow
context in an inherited environment cannot make the tool profile store linkage.

An existing launcher creates context with perfrec.ChildContext and composes a
copy of the child environment with perfrec.ContextEnv. New or replacement agents
receive new opaque session/agent IDs. Tool children retain agent/session context
and receive the spawning invocation as parent; they mint their own invocation.
A standing desk carries no last-work subject. Dispatch work must be selected for
that launch with explicit --perf-work flags; inherited desk work is cleared.
Do not export child context into a shared shell or reuse a prior assignment's
work labels for a different task. The dispatcher writes selected handoff values
through its existing private packet cache and owner-only writer; assignment
output contains only a fresh local sidecar path. Handoffs declare an expiration within seven
UTC days; the launcher prunes expired handoff files on subsequent writes. A missing, unsafe or unwritable cache leaves an explicit runtime
propagation gap and never fails dispatch.

Originating desk attribution and operational authority can differ. The only
environment source for deskRole is DESK_LOOP. When a new agent changes its
operational loop, its tool calls must use the explicit --perf-loop-role flag
to preserve the originating desk attribution. Never change DESK_LOOP for
measurement and never invent a second environment source for this field.

The runner carries optional context beside the existing Adapter.Start call,
through context.Context. It does not modify LaunchRequest, authority, required
capabilities, accounting or journal fields. Native session IDs map to opaque
identities only in the Client's private memory, expiring after seven UTC days.
Explicit resume preserves a retained mapping; unknown/expired mappings mint
fresh opaque identities and expose ContinuityGap. Process restart loses that
mapping and is an explicit continuity gap. No transcript scraping is used.

[The maintained-entrypoint inventory](performance-entrypoints.json) records
actual parser owners and unsupported boundaries. It includes tools whose shared
binding still needs a local recorder callback, standalone modules, implicit
cell convergence and containers. A bound flag alone is not evidence that an
existing outer recorder consumes it. Native harness launchers here expose no
qualified native resume identifier; they mint fresh opaque identities. Runtime
adoption and uninstrumented descendants remain unavailable coverage.

Local workflow records contain identifiable work linkage and require the
existing private local file permissions and retention. Do not publish local
records, environment handoffs or native mappings. Opt out of workflow attribution by clearing/off
profile; disable ASSAY_TELEMETRY to stop recording. A failed optional writer does not block
launch or change mandatory audit/accounting. No service protocol propagation
is admitted here: client/server context remains unavailable until its service
owner admits an explicit option with consent at both ends.

Scrubbed `env -i` launches deliberately admit validated, bounded performance
metadata from the launching shell as a descriptive exception to the ambient
scrub. These values are invocation-only, never persistent cell/default levers
or operational authority. Codex forwards its explicit environment policy via
`-c` process arguments: local process inspection can expose these opaque IDs and
work references. Do not put secrets in attribution; local-only storage is not a
promise that attribution is hidden from other local process observers.

Cell launches reuse the CLI's already selected invocation under the launching
process's ASSAY_TELEMETRY consent; they never resolve a second parent ID. When
consent exists only in cell.env and not in that process, the child can collect
under its cell consent but has an unknown parent; no parent record is invented.
Untyped tool children started through deskkit.Run clear inherited parent invocation
IDs, because an inherited grandparent is not an observed parent. Direct executor
launches do not pass through this helper and remain separately owned boundaries.
Protocol-option-off and unknown-service-context cases are not applicable until
the service protocol owner admits that interface; local consent-off tests do not
qualify those future service cases. Handoff expiry bounds permitted use; physical
pruning requires a subsequent write. A hard deletion deadline is unqualified and
is disclosed in the recorded human acceptance on #3537; no hard-deletion guarantee
is inferred from that ruling.
