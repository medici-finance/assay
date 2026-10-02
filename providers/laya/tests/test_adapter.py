import copy
import hashlib
import importlib.util
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("adapter", Path(__file__).parents[1] / "assay_laya/adapter.py")
a = importlib.util.module_from_spec(spec)
spec.loader.exec_module(a)

class Runtime:
    fail = None
    overflow = False
    bad = False
    silent = False
    def token_counts(self, inputs):
        return [100 if self.overflow else 1] * len(inputs)
    def infer(self, root, inputs, backend, *, local_only, truncate):
        assert local_only and not truncate
        if backend == "cuda" and self.fail:
            raise self.fail("unavailable")
        return {"label": "unknown" if self.bad else "hold", "actual_backend": "cpu" if self.silent else backend}

class AdapterTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.manifest = {"schema": 1, "code_revision": "a"*40, "model_revision": "b"*40, "licenses": {k:"synthetic" for k in ("code", "weights", "base")}, "assets": []}
        for kind in ("weights", "tokenizer", "encoder", "lock", "code"):
            (self.root/kind).write_bytes(b"fixture")
            self.manifest["assets"].append({"kind": kind, "path": kind, "sha256": hashlib.sha256(b"fixture").hexdigest()})
        self.packet = {"request": {"Subject":"example-org/item", "InputDigest":"input", "SchemaDigest":"schema", "Vocabulary":["hold","retry"]}, "consultation":{"Prompt":"question", "Detail":"detail", "Context":"state", "Vocabulary":["hold","retry"]}, "backend":"cuda", "allow_cpu_fallback": True}
        self.policy = {"approved":True, "max_bytes":4096, "max_tokens":32, "max_options":4, "allow_cpu_fallback":True}
        self.runtime = Runtime()
    def run_assess(self):
        return a.assess(self.packet, self.root, self.manifest, self.runtime, self.policy)
    def test_roundtrip(self):
        self.packet["backend"] = "cpu"
        result = self.run_assess()
        self.assertEqual(result["prediction"]["ActualBackend"], "cpu")
        self.assertTrue(result["prediction"]["Abstained"])
    def test_fallback(self):
        for failure in (MemoryError, a.DeviceUnavailable):
            self.runtime.fail = failure
            self.assertEqual(self.run_assess()["prediction"]["ActualBackend"], "cpu")
            self.policy["allow_cpu_fallback"] = False
            with self.assertRaisesRegex(a.Refused, "unauthorized"):
                self.run_assess()
            self.policy["allow_cpu_fallback"] = True
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
        self.policy["approved"] = False
        with self.assertRaisesRegex(a.Refused, "approval"):
            self.run_assess()
