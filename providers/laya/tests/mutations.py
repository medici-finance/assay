"""Reproducible guard mutations. Each must redden the unchanged fixture suite."""
from pathlib import Path
import subprocess
import sys
import tempfile

base = Path(__file__).parents[1]
source = (base / "assay_laya/adapter.py").read_text()
tests = (base / "tests/test_adapter.py").read_text()
mutations = {
    "approval": ('if not policy.get("approved", False):', 'if False:'),
    "tamper": ('if hashlib.sha256(path.read_bytes()).hexdigest() != asset["sha256"]:', 'if False:'),
    "overflow": ('any(n < 0 or n > policy["max_tokens"] for n in counts)', 'False'),
    "fallback": ('not policy.get("allow_cpu_fallback", False)', 'False'),
    "backend": ('if result.get("actual_backend") != actual:', 'if False:'),
    "malformed": ('if label not in options:', 'if False:'),
    "network": ('result = runtime.infer(root, inputs, backend, local_only=True, truncate=False)', 'try:\n            result = runtime.infer(root, inputs, backend, local_only=True, truncate=False)\n        except PermissionError:\n            result = {"label":"hold", "actual_backend":backend}'),
    "roundtrip": ('"Abstained": True', '"Abstained": False'),
}
for name, (old, new) in mutations.items():
    if source.count(old) != 1:
        raise SystemExit(f"mutation {name}: matcher changed")
    with tempfile.TemporaryDirectory(dir=base) as tmp:
        root = Path(tmp)
        (root/"assay_laya").mkdir()
        (root/"tests").mkdir()
        (root/"assay_laya/adapter.py").write_text(source.replace(old, new))
        (root/"tests/test_adapter.py").write_text(tests)
        result = subprocess.run([sys.executable, "-m", "unittest", "discover", "-s", str(root/"tests"), "-p", "test_adapter.py"], capture_output=True, text=True)
        if result.returncode == 0 or "FAILED" not in result.stderr:
            raise SystemExit(f"mutation {name} not caught: {result.stderr}")
        print(f"checked-failed as expected: {name}")
