# commsgw — the cell gateway

`commsgw` is the one chokepoint every inbound cell message crosses. Every
message — cross-cell (A2A + mTLS, `a2a.go`) and
within-cell (a loopback Unix socket, `socket.go`) alike — runs through the same
deterministic pre-check pipeline (`precheck.go`) before it is durably queued
(`mailbox.go`) for `../commsloop` (the paired drain consumer) to land.

## Enablement — config-off default

A cell's comms plane is enabled only when **all three** of the following are
present; any one absent leaves the cell inert (independent off-switches by
design — each is checked by a DIFFERENT component, so a defect in one can
never silently disarm the others):

1. A `comms:` key in that cell's `topology.yaml` (strict parse; absence reads
   as disabled, an unrecognised value is a parse error —
   `tools/desk/internal/topology`'s `CommsMode`). This key is declarative
   only: this package never reads it, and it wires nothing here.
2. Every key below (`config.go`) is REQUIRED. Absence of any one of them
   refuses to serve; there is no partially-enabled state.
3. The gateway process actually deployed and reachable for that cell.

As of the 2026-09-17 human ruling on this key's decision-gate issue (Option 2,
"Interim rung first", over the recorded full-enable target), the recorded mode
is `comms: interim` — receive-and-route live, but
every execution lands as a proposed dispatch a person fires, never an
autonomous session-firing (`../commsloop`'s `Loop.Native` stays at its zero
value; see `dispatch_native.go`'s doc and
`TestDispatchZeroValueNativeStaysInertForSessionTier`). Full enablement
(`comms: full`, `Loop.Native = true`) is NOT implemented by this change.

| Key | Purpose |
|---|---|
| `ASSAY_COMMS_GATEWAY_ENABLE` | must be exactly `1` |
| `ASSAY_COMMS_CELL` | this gateway's own cell name |
| `ASSAY_COMMS_QUEUE_DIR` | durable accepted-queue + mailboxes root |
| `ASSAY_COMMS_SOCKET` | within-cell loopback Unix-domain socket path |
| `ASSAY_COMMS_LISTEN` | cross-cell A2A network listen address |
| `ASSAY_COMMS_TLS_CERT` / `ASSAY_COMMS_TLS_KEY` | this gateway's own mTLS identity |
| `ASSAY_COMMS_CLIENT_CA` | house trust store verifying a peer gateway's client cert |
| `ASSAY_COMMS_TRUST_STORE` | JSON `{cell: base64 ed25519 pubkey}` for signed-assertion verification |

## The pre-check pipeline (deterministic, in order)

1. mTLS peer accept (cross-cell) / loopback socket trust (within-cell)
2. envelope parse-or-refuse (`internal/comms.ParseEnvelope`)
3. signed-assertion verify (`internal/comms.VerifyEnvelope`) — unknown cell /
   bad signature / expired / not-yet-valid / replayed are distinct refusals
4. lane ACL — within-cell reach, or cross-cell PAIR + VERB allow-set (two
   distinct refusal codes: `ErrCrossCellPair`, `ErrCrossCellVerb`)
5. `Claim()` dedupe — a message id is claimed exactly once (`deskkit.Acquire`)
6. per-sender rate/budget
7. kill switch (`deskkit.Guard`)

No deterministic routing: PreCheck answers accept-or-refuse only. An accepted
message quarantines until the (not-yet-landed) prose router lands.

## The outbound prose gate — every send (`prosegate.go`)

Every send crosses a second, INDEPENDENT layer after the deterministic
pre-checks pass: a quarantined prose gate on the gateway send path
(`socket.go`'s submit handler — the one chokepoint every local sender submits
through, so a non-Claude sender meets it too). Within-cell and cross-cell sends
alike are consulted, with no risk-trigger predicate selecting which — filtering
is symmetric, because the deterministic body scan is tokens-only and slug-blind
(checked-clean is not leak-free). The gate fails on a different signal than that
scanner, so it catches a slug-shaped leak the scanner passes.

- **Order is one-directional.** The deterministic pre-checks run FIRST and a
  deterministic refusal is TERMINAL — the gate is never consulted for a message
  the pipeline already refused, and it never overrides a deterministic refusal.
- **Advise-only, three verdicts.** The gate consults the read-only decider
  (`deskkit.Decide`) for one of `clean-send` / `hold-for-human` / `refuse` and
  acts in its own code; it CANNOT rewrite content (a message that would need
  editing is held instead). Default is `hold-for-human`.
- **Held, never dropped.** Any non-clean verdict holds the message (held mailbox
  + a filed issue carrying the message DIGEST, never the raw payload — the
  public-tree self-containment rule binds an issue body). Never a silent drop,
  never an auto-retry.
- **Fail closed.** `DESK_DECIDE_DISABLED=1`, a spent budget, a timeout, an
  advisor error, or an injected/malformed answer all resolve to the
  `hold-for-human` default — outbound comms halts safely rather than flowing
  ungated.
- **Containment.** The reader runs under the pinned decider runner entry's
  refuse-everything profile: empty filesystem root, and callback policies that
  refuse every fs / terminal / tool request and file the attempt as an anomaly.
  A configured decider entry not declared for that profile refuses at boot; an
  unconfigured one leaves the valve off (every send holds).

The mirror-image INBOUND prose layer — a peer's message arriving over the A2A
server — is the symmetric inbound router's concern, a separate layer landing
elsewhere; this gate is the outbound half only.

## Refusal journal (#1165)

Every refused inbound — on either transport — writes **one line** to the same
`<queue-dir>/journal.log` the drain's landings go to and the daily
`commsloop sweep` reads. Before this, a refusal was visible only on the
client receipt, so a sustained probe against the gateway (forged peers,
out-of-lane submits, replayed nonces) left no out-of-band trace and the
sweep's "no violations" was could-not-check rather than a measurement.

- **Shape** (`internal/commsqueue` `RefusalRecord`, the one definition both
  binaries share): `kind:"refused"`, the distinct **refusal kind** (one per
  typed refusal: `peer-unauthenticated`, `carrier-malformed`,
  `envelope-parse`, `unknown-cell`, `bad-signature`, `expired`,
  `not-yet-valid`, `replay`, `identity-mismatch`, `assertion-invalid`,
  `lane-denied`, `cross-cell-pair`, `cross-cell-verb`, `duplicate`,
  `budget-exhausted`, `rate-limiter-unconfigured`, `kill-switch`, `other`),
  the **lane pair and sender identity as presented** (unverified — a refused
  message has by definition not passed verification), this gateway's own
  cell, the gateway clock's timestamp, and a sha256 digest of the raw bytes.
- **Never the payload.** No payload bytes, no assertion, no free-text error
  detail (which can echo untrusted field values) — the digest-only ruling
  that binds a prose consult binds a refusal. The record type has no such
  field by construction and a test pins it.
- **The refusal never depends on the write.** A journal write failure is
  appended to the receipt detail (`…; commsgw: refusal journal write
  failed: …`) so the sender sees the gateway refused AND could not record
  it; the refusal itself stands unchanged.
- **The sweep counts them** per presented sender and per presented
  destination lane; at or over `--refusal-threshold` (default 10 per sweep
  window) it reports a `refusal-threshold` finding — see
  `../commsloop/sweep.go`.

## Replay window and clock skew (#1951)

This gateway's assertion verification runs against two values from
`internal/comms/identity.go`, documented here (`deps.go`) at the gateway's own
config surface per #1951:

- **`comms.DefaultTTL` = 2m** — an assertion's validity window. Short by
  design: it binds one message, consumed once, so it need only cover
  mint-to-delivery.
- **`comms.DefaultSkew` = 1m** — how far ahead of this gateway's clock a mint
  may legitimately claim to be issued. Absorbs ordinary NTP drift without
  meaningfully widening a 2-minute-lifetime assertion's exposure.

`NewPreCheckDeps` (this gateway's one construction site for `PreCheckDeps`)
always supplies a non-nil `comms.ReplayGuard` — a nil guard disables replay
refusal entirely (identity.go), so a real gateway must never construct one
without it. `TestReplayGuardWired` pins this through the real construction
path, not a hand-built `Verify` call.

## Cross-cell verb allow-set (#1896)

Exactly four verbs, enumerated in `internal/comms/laneacl.yaml`
`cross_cell.verbs` (no wildcard): `status`, `metrics`, `help-offered`,
`focus-on`. The pair set is `the-desk` <-> `the-desk` only. Any other
cross-cell verb refuses with a distinct code from an out-of-pair refusal.

On every ACCEPTED cross-cell message the gateway emits one deskd inbox item of
kind `cross-cell` (`inbox.go`) into the message's destination cell's inbox —
the same rule for the original request and for a reply (a reply is itself an
accepted cross-cell message, just addressed the other way). Emission failure
quarantines the message (held mailbox + a filed issue via `deskfile`); it is
never dropped.

## Quarantine

`Quarantine = held mailbox + a filed issue via deskfile` (the silent-desk
rule). The held write is unconditional; a filing failure is reported but never
undoes it — the message stays held either way.
