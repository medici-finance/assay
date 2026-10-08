### Fixed
- `deskdispatch` accepts a `--pr N` dispatch onto a brief's tracking repo when the brief's deliverable resolves elsewhere, so a brief-authoring change can be reviewed with `--repo`/`--root` on its own repo and `--brief` still passed (a `gate: human` brief stays detected). Every other registry mismatch is still the hard fail. (#2355)
- `deskdispatch` releases its claim on every abort after the claim is placed — including a failed decision-issue gate, model stamp, prompt assembly or prompt write — so a re-run is no longer refused behind a claim no agent holds. (#2355)
