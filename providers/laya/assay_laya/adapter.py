"""Validation seam for an operator-reviewed local runtime. No bundled model loader.

The injected runtime must load local files only and expose exact token counts with
truncation disabled. A network-denying OS sandbox is required for real execution;
Python environment flags are defense in depth, never a network sandbox.
"""
import hashlib
import json
from pathlib import Path
import re
import time


class Refused(ValueError):
    pass


def validate_artifacts(root, manifest):
    root = Path(root).resolve()
    if manifest.get("schema") != 1:
        raise Refused("artifact schema")
    for name in ("code_revision", "model_revision"):
        if not re.fullmatch(r"[0-9a-f]{40}", manifest.get(name, "")):
            raise Refused("unpinned revision")
    if not all(manifest.get("licenses", {}).get(k) for k in ("code", "weights", "base")):
        raise Refused("missing license provenance")
    assets = manifest.get("assets", [])
    if not {"weights", "tokenizer", "encoder", "lock", "code"}.issubset({a["kind"] for a in assets}):
        raise Refused("incomplete artifact bundle")
    for asset in assets:
        rel = Path(asset["path"])
        if rel.is_absolute() or ".." in rel.parts:
            raise Refused("unsafe artifact path")
        path = root / rel
        if path.is_symlink() or not path.is_file() or not path.resolve().is_relative_to(root):
            raise Refused("missing or escaping asset")
        if hashlib.sha256(path.read_bytes()).hexdigest() != asset["sha256"]:
            raise Refused("tampered asset")
    return root


def assess(packet, root, manifest, runtime, policy):
    if not policy.get("approved", False):
        raise Refused("owner approval required")
    validate_artifacts(root, manifest)
    backend = packet["backend"]
    if backend not in ("cpu", "cuda"):
        raise Refused("unqualified backend")
    c = packet["consultation"]
    options = c["Vocabulary"]
    if options != packet["request"]["Vocabulary"] or not options or len(set(options)) != len(options):
        raise Refused("option order mismatch")
    inputs = [c["Prompt"], c["Detail"], c["Context"], *options]
    if len(json.dumps(packet).encode()) > policy["max_bytes"]:
        raise Refused("input byte overflow")
    # Counts come from the reviewed local tokenizer, never whitespace estimates.
    counts = runtime.token_counts(inputs)
    if len(counts) != len(inputs) or any(n < 0 or n > policy["max_tokens"] for n in counts) or len(options) > policy["max_options"]:
        raise Refused("token or option overflow")
    actual, reason = backend, ""
    started = time.monotonic()
    try:
        result = runtime.infer(root, inputs, backend, local_only=True, truncate=False)
    except (MemoryError, DeviceUnavailable) as exc:
        if backend != "cuda" or not packet["allow_cpu_fallback"] or not policy.get("allow_cpu_fallback", False):
            raise Refused("unauthorized CPU fallback") from exc
        actual, reason = "cpu", type(exc).__name__
        result = runtime.infer(root, inputs, actual, local_only=True, truncate=False)
    if result.get("actual_backend") != actual:
        raise Refused("runtime silently changed backend")
    label = result.get("label")
    if label not in options:
        raise Refused("malformed output")
    req = packet["request"]
    return {"prediction": {
        "Subject": req["Subject"], "InputDigest": req["InputDigest"], "SchemaDigest": req["SchemaDigest"],
        "Abstained": True, "ShadowLabels": [label], "LabelProbabilities": {},
        "RequestedBackend": backend, "ActualBackend": actual,
        "ProviderVersion": manifest["code_revision"], "EvidenceRefs": ["sha256:" + hashlib.sha256(json.dumps(manifest, sort_keys=True).encode()).hexdigest()],
    }, "fallback_reason": reason, "diagnostics": {"latency_seconds": time.monotonic() - started, "omissions": [], "precision": result.get("precision", "unknown"), "memory_bytes": result.get("memory_bytes")}}


class DeviceUnavailable(RuntimeError):
    pass
