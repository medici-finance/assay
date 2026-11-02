"""Portable behavioural tests for the shared proof and fixed witness entrypoint."""
import importlib.util
from pathlib import Path
import tempfile
import unittest

HERE = Path(__file__).resolve().parent
WITNESS = HERE / 'witness.sh' if (HERE / 'witness.sh').is_file() else HERE / 'k8s/witness.sh'
spec = importlib.util.spec_from_file_location('proof_probe', HERE / 'source_proof_check.py')
check = importlib.util.module_from_spec(spec)
spec.loader.exec_module(check)


class ProofTests(unittest.TestCase):
    def test_joined_source(self):
        self.assertEqual(check.probe(HERE / 'source_proof.py', WITNESS), 'SOURCE-PROOF PASS')

    def test_control_mutants(self):
        mutants = [
            ('proof call', 'witness.sh',
             '/usr/bin/python3 -I /usr/local/lib/verify-runner/source_proof.py --check-clone "$WORK" "$SOURCE_SHA"',
             'true', 'refused source executed rows'),
            ('missing files', 'source_proof.py', 'if seen != set(expected):', 'if False:', 'refused source executed rows'),
            ('source bytes', 'source_proof.py', 'if mode != actual_mode or not want:', 'if False:', 'refused source executed rows'),
            ('delivery head', 'witness.sh', '[ -z "$DELIVERY_SHA" ] || export ASSAY_DELIVERY_SHA="$DELIVERY_SHA"', ':', None),
            ('delivery base', 'witness.sh', '[ -z "$DELIVERY_BASE" ] || export ASSAY_DELIVERY_BASE_SHA="$DELIVERY_BASE"', ':', None),
            ('ambient delivery', 'witness.sh', 'unset ASSAY_DELIVERY_SHA ASSAY_DELIVERY_BASE_SHA', ':', None),
            ('receipt gate', 'witness.sh', '[ -f \"$RECEIPT\" ] && [ -r \"$RECEIPT\" ] || die', 'true || die', 'refused source executed rows'),
        ]
        for name, file, old, new, message in mutants:
            with self.subTest(name=name), tempfile.TemporaryDirectory(prefix='proof-mutant-') as tmp:
                root = Path(tmp)
                for item in ('source_proof.py', 'witness.sh'):
                    (root / item).write_bytes((WITNESS if item == 'witness.sh' else HERE / item).read_bytes())
                path = root / file
                text = path.read_text()
                self.assertEqual(text.count(old), 1, 'mutation site not unique: '+name)
                path.write_text(text.replace(old, new, 1))
                # Only a behavioural assertion is a killed mutant. Imports, syntax,
                # fixture setup errors and a zero-test execution do not count.
                with self.assertRaises(AssertionError) as failure:
                    check.probe(root / 'source_proof.py', root / 'witness.sh')
                if message:
                    self.assertIn(message, str(failure.exception))


if __name__ == '__main__':
    unittest.main()
