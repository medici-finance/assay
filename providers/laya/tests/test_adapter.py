import hashlib
import importlib.util
from pathlib import Path
import tempfile
import unittest

base = Path(__file__).parents[1]
spec = importlib.util.spec_from_file_location("adapter", base / "assay_laya/adapter.py")
a = importlib.util.module_from_spec(spec)
spec.loader.exec_module(a)

class Runtime:
    fail = None
    overflow = False
    bad = False
    silent = False
    def __init__(self):
        self.calls = []
    def token_counts(self, inputs):
        self.calls.append("token_counts")
        return [100 if self.overflow else 1] * len(inputs)
    def infer(self, root, inputs, backend, *, local_only, truncate):
        self.calls.append(("infer", backend))
        assert local_only and not truncate
        if self.fail:
            raise self.fail("unavailable")
        return {"label": "unknown" if self.bad else "hold", "actual_backend": "cuda" if self.silent else backend}

class AdapterTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.manifest = {"schema": 1, "code_revision": "a"*40, "model_revision": "b"*40, "licenses": {k:"synthetic" for k in ("code", "weights", "base")}, "assets": []}
        for kind in ("weights", "tokenizer", "encoder", "lock", "code"):
            (self.root/kind).write_bytes(b"fixture")
            self.manifest["assets"].append({"kind": kind, "path": kind, "sha256": hashlib.sha256(b"fixture").hexdigest()})
        self.packet = {"request": {"Subject":"example-org/item", "InputDigest":"input", "SchemaDigest":"schema", "Vocabulary":["hold","retry"]}, "consultation":{"Prompt":"question", "Detail":"detail", "Context":"state", "Vocabulary":["hold","retry"]}, "backend":"cpu"}
        self.policy = {"approved":True, "max_bytes":4096, "max_tokens":32, "max_options":4}
        self.runtime = Runtime()
    def run_assess(self):
        return a.assess(self.packet, self.root, self.manifest, self.runtime, self.policy)
    def test_roundtrip(self):
        result = self.run_assess()
        self.assertEqual(result["prediction"]["ActualBackend"], "cpu")
        self.assertEqual(result["fallback_reason"], "")
        self.assertTrue(result["prediction"]["Abstained"])
    def test_cpu_only(self):
        # Per the driver's ruling: CPU only. Exact match, refused before the runtime is touched.
        for backend in ("cuda", "mps", "CPU", "Cpu", " cpu", "cpu ", "cpu\n", "", "cpu,cuda", "cuda:0", "auto", None):
            self.packet["backend"] = backend
            self.runtime.calls.clear()
            with self.assertRaisesRegex(a.Refused, "unqualified backend", msg=repr(backend)):
                self.run_assess()
            self.assertEqual(self.runtime.calls, [], repr(backend))
    def test_fallback(self):
        # A device failure on CPU is refused; the runtime is never retried on another device.
        for failure in (MemoryError, a.DeviceUnavailable):
            self.runtime.fail = failure
            self.runtime.calls.clear()
            with self.assertRaisesRegex(a.Refused, "no fallback"):
                self.run_assess()
            self.assertEqual([c for c in self.runtime.calls if c != "token_counts"], [("infer", "cpu")])
    def test_policy_override(self):
        # A policy key cannot enable another backend or a fallback; unknown keys are refused.
        for key, value in (("allowed_backends", ("cpu", "cuda")), ("allow_cpu_fallback", True), ("backend", "cuda"), ("allow_cuda", True)):
            policy = dict(self.policy, **{key: value})
            with self.assertRaisesRegex(a.Refused, "unknown policy key", msg=key):
                a.assess(self.packet, self.root, self.manifest, self.runtime, policy)
    def test_packet_override(self):
        for key, value in (("allow_cpu_fallback", True), ("device", "cuda")):
            packet = dict(self.packet, **{key: value})
            with self.assertRaisesRegex(a.Refused, "packet field", msg=key):
                a.assess(packet, self.root, self.manifest, self.runtime, self.policy)
    def test_assets(self):
        (self.root/"weights").write_bytes(b"tamper")
        with self.assertRaisesRegex(a.Refused, "tampered"):
            self.run_assess()
        (self.root/"weights").unlink()
        with self.assertRaisesRegex(a.Refused, "missing"):
            self.run_assess()
    def test_overflow(self):
        self.runtime.overflow = True
        with self.assertRaisesRegex(a.Refused, "overflow"):
            self.run_assess()
    def test_malformed(self):
        self.runtime.bad = True
        with self.assertRaisesRegex(a.Refused, "malformed"):
            self.run_assess()
    def test_silent_fallback(self):
        self.runtime.silent = True
        with self.assertRaisesRegex(a.Refused, "silently"):
            self.run_assess()
    def test_network_attempt(self):
        def network(*args, **kwargs):
            raise PermissionError("sandbox denied attempted network")
        self.runtime.infer = network
        with self.assertRaisesRegex(PermissionError, "network"):
            self.run_assess()
    def test_unapproved(self):
        # Approval is the boolean True; a truthy string such as "false" is not approval.
        for value in (False, None, "false", "yes", 1):
            self.policy["approved"] = value
            with self.assertRaisesRegex(a.Refused, "approval", msg=repr(value)):
                self.run_assess()
        del self.policy["approved"]
        with self.assertRaisesRegex(a.Refused, "approval"):
            self.run_assess()
    def test_not_executable(self):
        # Per the driver's ruling the provider stays disabled: no entry point or loader ships.
        package = base / "assay_laya"
        self.assertFalse((package / "__main__.py").exists())
        for source in package.glob("*.py"):
            self.assertNotIn("__main__", source.read_text(), source.name)
        project = (base / "pyproject.toml").read_text()
        for marker in ("[project.scripts]", "[project.gui-scripts]", "[project.entry-points"):
            self.assertNotIn(marker, project)
