# Optional local Laya provider — preparatory boundary

This change supplies a dependency-free Python validation seam and an optional Go
`LayaAdvisor` subprocess boundary. It does **not** ship an inference loader, model,
verified artifact bundle, calibration or activation. Nothing is wired into default
routing. The example manifest deliberately fails validation. No real checkpoint
has been executed. Pending owner decisions prevent completing graph-execution/11.

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

Go sends one JSON object on stdin: `request`, `consultation`, `backend`, and
`allow_cpu_fallback`. The child returns `prediction`, `fallback_reason` and optional
`diagnostics`; diagnostics belong on stderr for a deployed executable. Bytes and
wall time are bounded independently. Python `assess` provides the record-building
seam; executable launch and real model loading remain held for review.
The reviewed runtime must count tokens with its exact local tokenizer before
inference and pass `local_only=True, truncate=False`. It must enforce read-only
loading and never download. All question fields and option order are retained;
context selection is unsupported and omissions are empty. CUDA availability or
OOM can fall back to CPU only when both caller and local policy permit it; silent
fallback is rejected. MPS is unqualified. Confidence/action stay uncalibrated,
shadow-only and abstaining; they cannot produce usable `Advice` or authorize effects.

Planning envelope: an existing 8–16 GB RAM CPU machine for short-input trials;
optional 4–8 GB CUDA VRAM, subject to measurement. No purchase, speed or
calibration guarantee. Real CPU/CUDA smoke execution remains could-not-check until
an approved bundle, loader and sandbox exist. No hardware probe or download is
required for ordinary tests. Unavailable hardware must be reported could-not-check.

Run dependency-light fixture checks:

```
python3 -m unittest discover -s providers/laya/tests -p test_adapter.py -v
cd tools/desk
GOWORK=off go test -timeout 60s -run '^TestLayaAdvisor' ./internal/deskkit
```

Reproduce fail-first evidence with `python3 providers/laya/tests/mutations.py`
and `GOWORK=off python3 providers/laya/tests/go_mutations.py`. The latter restores
production Go source after each bounded mutant; run it only in an owned clean
worktree.

Fixtures measure boundary behavior, never model quality, memory or CPU/GPU speed.
The downstream evaluation work must obtain independent labels and calibration;
it receives abstaining predictions until then.
