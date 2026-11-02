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
             'true', 'refusal:requested-sha'),
            ('missing files', 'source_proof.py', 'if seen != set(expected):', 'if False:', 'refusal:missing'),
            ('source bytes', 'source_proof.py', 'if mode != actual_mode or not want:', 'if False:', 'refusal:bytes'),
            ('delivery head', 'witness.sh', '[ -z "$DELIVERY_SHA" ] || export ASSAY_DELIVERY_SHA="$DELIVERY_SHA"', ':', 'delivery:head'),
            ('delivery base', 'witness.sh', '[ -z "$DELIVERY_BASE" ] || export ASSAY_DELIVERY_BASE_SHA="$DELIVERY_BASE"', ':', 'delivery:base'),
            ('ambient delivery', 'witness.sh', 'unset ASSAY_DELIVERY_SHA ASSAY_DELIVERY_BASE_SHA', ':', 'delivery:ambient'),
            ('receipt gate', 'witness.sh', '[ -f \"$RECEIPT\" ] && [ -r \"$RECEIPT\" ] || die', 'true || die', 'refusal:receipt'),
            ('requested source', 'witness.sh', '--check-clone "$WORK" "$SOURCE_SHA"',
             '--check-clone "$WORK" "$(/usr/bin/git -C "$WORK" rev-parse HEAD)"', 'refusal:requested-sha'),
            ('required source', 'witness.sh', '[ -n "$SOURCE_SHA" ] || usage', ':', 'usage:no-sha'),
            ('delivery proof guard', 'witness.sh', '[ -z "$SOURCE_SHA$DELIVERY_SHA$DELIVERY_BASE" ] || usage', ':', 'refusal:delivery-without-proof'),
            ('seed flags', 'witness.sh', '[ "$SOURCE_PROOF$SOURCE_SHA$DELIVERY_SHA$DELIVERY_BASE" = 0 ] || usage', ':', 'usage:seed-flags'),
            ('source shape', 'witness.sh', '[[ $SOURCE_SHA =~ ^[0-9a-f]{40}$ ]] || usage', ':', 'usage:malformed-sha'),
            ('probe-only flags', 'witness.sh',
             'seed|seed-probe|probe) [ "$SOURCE_PROOF$SOURCE_SHA$DELIVERY_SHA$DELIVERY_BASE" = 0 ] || usage ;;',
             'seed|seed-probe) [ "$SOURCE_PROOF$SOURCE_SHA$DELIVERY_SHA$DELIVERY_BASE" = 0 ] || usage ;;\n  probe) ;;',
             'refusal:seed-flags'),
        ]
        self.assertEqual(len(mutants), 13)
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
                with self.assertRaises(check.ProbeFailure) as failure:
                    check.probe(root / 'source_proof.py', root / 'witness.sh')
                self.assertEqual(message, failure.exception.code)
                print('RED ' + name + ' code=' + failure.exception.code)

    def test_invalid_candidates(self):
        with tempfile.TemporaryDirectory(prefix='proof-invalid-') as tmp:
            root = Path(tmp)
            (root / 'source_proof.py').write_bytes((HERE / 'source_proof.py').read_bytes())
            (root / 'witness.sh').write_text(WITNESS.read_text() + "\n'broken syntax\n")
            with self.assertRaises(RuntimeError):
                check.probe(root / 'source_proof.py', root / 'witness.sh')
            (root / 'witness.sh').write_bytes(WITNESS.read_bytes())
            (root / 'source_proof.py').write_text('def invalid(:\n')
            with self.assertRaises(SyntaxError):
                check.probe(root / 'source_proof.py', root / 'witness.sh')


if __name__ == '__main__':
    unittest.main()
