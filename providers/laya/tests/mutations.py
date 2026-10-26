"""Reproducible guard mutations. Each must redden the unchanged fixture suite.

An unmutated control copy must pass first, so a broken copy layout cannot make every
mutant look caught.
"""
from pathlib import Path
import subprocess
import sys
import tempfile

base = Path(__file__).parents[1]
files = {
    "adapter": ("assay_laya/adapter.py", (base / "assay_laya/adapter.py").read_text()),
    "project": ("pyproject.toml", (base / "pyproject.toml").read_text()),
}
tests = (base / "tests/test_adapter.py").read_text()
mutations = {
    "approval": ("adapter", 'if policy.get("approved") is not True:', 'if False:'),
    "approval-truthy": ("adapter", 'if policy.get("approved") is not True:', 'if not policy.get("approved"):'),
    "policy-keys": ("adapter", 'if set(policy) - POLICY_KEYS:', 'if False:'),
    "packet-keys": ("adapter", 'if set(packet) != PACKET_KEYS:', 'if False:'),
    "cpu-only": ("adapter", 'if packet["backend"] != BACKEND:', 'if False:'),
    "normalise": ("adapter", 'if packet["backend"] != BACKEND:', 'if str(packet["backend"]).strip().lower() != BACKEND:'),
    # Still refuses, but only after touching the runtime: the refusal must come first.
    "late-backend": ("adapter", 'if packet["backend"] != BACKEND:', 'if (runtime.token_counts([]) or True) and packet["backend"] != BACKEND:'),
    "tamper": ("adapter", 'if hashlib.sha256(path.read_bytes()).hexdigest() != asset["sha256"]:', 'if False:'),
    "overflow": ("adapter", 'any(n < 0 or n > policy["max_tokens"] for n in counts)', 'False'),
    "fallback": ("adapter", 'raise Refused("device failure; no fallback permitted") from exc', 'result = {"label": options[0], "actual_backend": BACKEND}'),
    "backend": ("adapter", 'if result.get("actual_backend") != BACKEND:', 'if False:'),
    "malformed": ("adapter", 'if label not in options:', 'if False:'),
    "network": ("adapter", 'result = runtime.infer(root, inputs, BACKEND, local_only=True, truncate=False)', 'try:\n            result = runtime.infer(root, inputs, BACKEND, local_only=True, truncate=False)\n        except PermissionError:\n            result = {"label":"hold", "actual_backend":BACKEND}'),
    "roundtrip": ("adapter", '"Abstained": True', '"Abstained": False'),
    "entrypoint": ("project", 'dependencies = []', 'dependencies = []\n\n[project.scripts]\nassay-laya = "assay_laya.adapter:assess"'),
}


def run(name, target=None, old=None, new=None):
    with tempfile.TemporaryDirectory(dir=base) as tmp:
        root = Path(tmp)
        (root/"assay_laya").mkdir()
        (root/"tests").mkdir()
        for key, (rel, text) in files.items():
            if key == target:
                if text.count(old) != 1:
                    raise SystemExit(f"mutation {name}: matcher changed")
                text = text.replace(old, new)
            (root/rel).write_text(text)
        (root/"tests/test_adapter.py").write_text(tests)
        return subprocess.run([sys.executable, "-I", "-m", "unittest", "discover", "-s", str(root/"tests"), "-p", "test_adapter.py"], capture_output=True, text=True)


control = run("control")
if control.returncode != 0:
    raise SystemExit(f"control copy fails unmutated: {control.stderr}")
print("control passed unmutated")
for name, (target, old, new) in mutations.items():
    result = run(name, target, old, new)
    if result.returncode == 0 or "FAILED" not in result.stderr:
        raise SystemExit(f"mutation {name} not caught: {result.stderr}")
    print(f"checked-failed as expected: {name}")
print(f"{len(mutations)} of {len(mutations)} mutants caught")
