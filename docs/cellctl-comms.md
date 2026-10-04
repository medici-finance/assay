# Host desk communications

`cellctl comms <cell> run` supervises the existing Go `commsgw` and `commsloop`
processes for a house cell. Claude and Codex desks launched by `cellctl desk` or
`cellctl up` receive their own `DESK_CELL`, `DESK_ROLE`, `DESK_COMMS_GATEWAY` and
`DESK_COMMS_KEY`. The transport is a private Unix socket on macOS/Linux and a
current-user-only local named pipe on Windows. It requires no shell script.

This is **interim mailbox delivery**: the receiving desk polls and acknowledges
messages during its sweep. It does not wake a stopped desk or fire autonomous
sessions. The service has no cross-cell listener; foreign-cell messages and
mailbox requests are refused. Existing standalone gateway deployments keep
their mTLS requirements unless explicitly configured with
`ASSAY_COMMS_LOCAL_ONLY=1`.

## Configure before cutover

Install matching `cellctl`, `deskcomms`, `commsgw` and `commsloop` binaries. The
cell's desk-tools directory must contain the service binaries. Provision these
operator-owned files outside the repository:

- A private Ed25519 signing seed (32 bytes, hex encoded), and a trust-store JSON
  object mapping the cell name to its base64-encoded public key. The existing
  trust format has one public key per cell: all role key files must correspond
  to that key. Role paths are explicit; this is not per-role cryptographic key
  isolation. Desks running as the same OS account share that trust boundary.
- A pinned ACP reader command and model with `isolate:true` and
  `contained:true`, using the existing `ASSAY_RUNNER_DECIDER` contract. Both
  outbound filtering and inbound routing use this reader; a missing or unsafe
  entry refuses startup, and failed consultations hold/quarantine messages.
  A plain interactive `claude` or `codex` command is not an ACP reader.
- An absolute JSON manifest, shaped as follows. Replace the example paths,
  repository, reader command, version pin and model with provisioned values.

```json
{
  "cell": "example",
  "mode": "interim",
  "repo": "example/project",
  "socket": "/srv/cells/example/comms/gateway.sock",
  "queue_dir": "/srv/cells/example/comms/queue",
  "trust_store": "/srv/cells/example/comms/trust.json",
  "signing_keys": {
    "the-desk": "/srv/cells/example/comms/signing.key",
    "intake-desk": "/srv/cells/example/comms/signing.key",
    "worker-desk": "/srv/cells/example/comms/signing.key",
    "pr-review-desk": "/srv/cells/example/comms/signing.key",
    "verify-desk": "/srv/cells/example/comms/signing.key"
  },
  "decider": {
    "cmd": ["/opt/readers/pinned-acp-reader"],
    "model": "configured-reader-model",
    "pin": "installed-reader-version",
    "isolate": true,
    "contained": true
  },
  "claude_config_dir": "/srv/agent-config/claude"
}
```

`repo` is the configured quarantine-issue destination. `claude_config_dir` is
optional and selects the reader's Claude login directory; each desk's login
directory remains the final positional argument to `cellctl desk`.
The manifest, trust store and signing files must pass owner-only custody
checks (0600 files on Unix; protected current-user ACLs on Windows), with no
symlink/reparse-point substitution. Keep the queue and endpoint directory
private too. Do not put keys inline in the manifest, in argv, or in git.

On Windows, use absolute Windows paths and a local pipe, for example the JSON
value `"socket": "\\\\.\\pipe\\assay-example-comms"`. Remote pipe addresses are
refused. Provision owner-only ACLs using the same custody procedure as the
cell's other credentials; Unix chmod alone does not establish Windows custody.

Set the absolute manifest path and check it:

```text
cellctl set example CELL_COMMS_CONFIG=/absolute/path/to/comms.json
cellctl comms example check
cellctl comms example run
```

`check` checks manifest structure, custody, required role paths, reader entry
and service binary availability. It does not probe model credentials, verify
the installed reader version, parse signing/trust material, or prove delivery.
The service validates trust material at boot and senders validate keys on send.
Run the service in its own terminal or supervisor; `up` does not start it.
Keep the existing recorded comms cutover decision aligned with this manifest
before asking desks to poll. A release or a missing environment variable is
not a cutover decision.

Then launch each desk with its selected harness and login directory. Existing
sessions must restart through `cellctl` to acquire the new context. When no
manifest is configured, or its mode is `disabled`, no gateway is started and
ambient comms identity is removed from child launches.

## Poll, acknowledge and recover

Use `deskcomms send` with the existing lane ACL and payload contract. Receivers
use `deskcomms poll --json` to read complete messages, including payloads;
plain `poll` retains its compact tabular output. Polling does not consume a
message. After acting, `deskcomms ack <id>` moves it to the acknowledged store.
An accepted send is durable queue acceptance, not proof of mailbox delivery:
the drain still applies its contained routing consultation. An idle drain
checks again after one minute.

If either service exits, `cellctl` stops its peer and owns child-tree cleanup.
A crash or uncertain cleanup leaves an unfinished checkpoint that blocks
restart. Inspect and stop the prior gateway, drain and their children before:

```text
cellctl comms example recover --confirm-stopped
cellctl comms example run
```

Recovery removes the service checkpoint and a stale Unix socket, preserving
queued, held and acknowledged messages. It never takes over an active lease.
