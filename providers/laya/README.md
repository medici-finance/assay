# Optional local Laya provider — preparatory boundary

This change supplies a dependency-free Python validation seam and an optional Go
`LayaAdvisor` subprocess boundary. It does **not** ship an inference loader, model,
verified artifact bundle, calibration or activation. Nothing is wired into default
routing. The example manifest deliberately fails validation. No real checkpoint
has been executed. Pending owner decisions prevent completing graph-execution/11.

## Activation policy

Per the driver's ruling
(<https://github.com/medici-finance/assay/issues/1992#issuecomment-6399433100>):

- The provider stays disabled until a reviewed local read-only artifact bundle and
  a network-denying sandbox exist. This change is preparation only.
- CPU only. CUDA and MPS are refused, and no device fallback is accepted in either
  direction.

Both boundaries enforce this without a setting that could relax it:

- Go `LayaAdvisor` refuses any backend other than the exact string `cpu` before a
  process starts. Case and whitespace variants, an empty value, `cuda` and `mps` are
  refused, not normalised. The response check is a separate layer: it compares against
  `cpu` itself, not the advisor's field, and refuses a non-CPU requested or actual
  backend or any `fallback_reason`. The child environment hides CUDA devices
  (`CUDA_VISIBLE_DEVICES` set empty) and inherits nothing from the caller. That is
  defense in depth, not a sandbox, and MPS has no equivalent switch. A test fails if
  any production file in the module names `LayaAdvisor`, so nothing routes to it.
- Python `assess` refuses any backend other than `cpu` before reading artifacts or
  calling the runtime. A device failure (OOM or device unavailable) is refused and
  never retried elsewhere. Packet and policy keys are closed sets, so a key such as
  `allowed_backends` or `allow_cpu_fallback` is refused rather than ignored. Approval
  must be the boolean `true`. The package ships no entry point.

Allowing CUDA, MPS or fallback later needs a new owner decision and its own change.

An owner must choose an exact code revision, checkpoint revision, local tokenizer
and encoder assets, separately documented code/weight/base license provenance,
and a dependency lock with complete checksums. The inspected candidate runtime
revision is `1161ff639204388b1e576a7c5d56a0f6df470455` at
[NandhaKishorM/laya](https://github.com/NandhaKishorM/laya).
It is an inspection reference, not an approved bundle. Checkpoints are candidates
from [convaiinnovations/laya](https://huggingface.co/convaiinnovations/laya).
No fabricated checksum or moving revision is accepted by the validator.

The operator must supply a trusted executable by absolute path in an OS sandbox
with network denied and approved assets mounted read-only. The Go child environment
contains offline flags and no inherited credentials. Environment flags and injected
Python stubs cannot prove a hostile process has no network or write capability.
No process is launched without explicit caller approval; the caller, not model
output, owns this approval. There is no external fallback or effect callback.

Go sends one JSON object on stdin: `request`, `consultation` and `backend`
(always `cpu`). The child returns `prediction`, `fallback_reason` and optional
`diagnostics`; diagnostics belong on stderr for a deployed executable. Bytes and
wall time are bounded independently. Both stdout and stderr copy through a
checked writer that exposes no unchecked buffer mutation methods. Go drains
all three pipes itself, bounded only by the context deadline: output is read to
end-of-file with no shorter drain window, and a pipe that a descendant still
holds at the deadline is closed and the call rejected (fail closed).
The context deadline also covers response decoding and validation; an expired
valid record is rejected. Timing assertions allow scheduler overhead, not a
hard real-time guarantee. Python `assess` provides the record-building
seam; executable launch and real model loading remain held for review.
The reviewed runtime must count tokens with its exact local tokenizer before
inference and pass `local_only=True, truncate=False`. It must enforce read-only
loading and never download. All question fields and option order are retained;
context selection is unsupported and omissions are empty. Device failure is
refused, never answered by a fallback, and a runtime that silently reports another
backend is rejected (see Activation policy). Confidence/action stay uncalibrated,
shadow-only and abstaining; they cannot produce usable `Advice` or authorize effects.

Planning envelope: an existing 8–16 GB RAM CPU machine for short-input trials;
optional 4–8 GB CUDA VRAM, subject to measurement and to a new owner decision,
since CUDA is refused under the activation policy. No purchase, speed or
calibration guarantee. Real CPU smoke execution remains could-not-check until
an approved bundle, loader and sandbox exist. No hardware probe or download is
required for ordinary tests. Unavailable hardware must be reported could-not-check.

Run dependency-light fixture checks:

```
python3 -m unittest discover -s providers/laya/tests -p test_adapter.py -v
cd tools/desk
GOWORK=off go test -timeout 60s -run '^TestLayaAdvisor' ./internal/deskkit
```

Reproduce fail-first evidence with `python3 providers/laya/tests/mutations.py`
and `GOWORK=off python3 providers/laya/tests/go_mutations.py`. Each runs an
unmutated control first and prints how many mutants were caught. The Go script
restores production source after each bounded mutant and removes the temporary
file its wiring mutant adds; run it only in an owned clean worktree.

Fixtures measure boundary behavior, never model quality, memory or CPU/GPU speed.
The downstream evaluation work must obtain independent labels and calibration;
it receives abstaining predictions until then.
