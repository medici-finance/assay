"""Regression: a content gate must not pass content it never parsed."""
import importlib.util
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location("staging_lint", ROOT / "tools/staging_lint.py")
linter = importlib.util.module_from_spec(spec)
spec.loader.exec_module(linter)

class ListSyntaxTests(unittest.TestCase):
    def test_supported_fixture_still_passes(self):
        self.assertEqual(linter.lint((ROOT / "tests/fixtures/good.yaml").read_text()), [])

    def test_unsupported_rows_cannot_silently_pass(self):
        for block in (
            'slots:\n  - n: 1\n    claim: "Unchecked claim"\n',
            'slots: [{n: 1, claim: "Unchecked claim"}]\n',
            'slots:\n  - {n: 1, claim: "Checked claim"}\n  - n: 2\n    claim: "Unchecked claim"\n',
            # column-0 block sequence: valid YAML, must not end the block unseen
            'slots:\n- {n: 1, claim: "Checked claim"}\n- n: 2\n  claim: "Churn is 17% and costs $99,999 a month."\n',
            'slots:\n- n: 1\n  claim: "Unchecked claim"\n',
        ):
            with self.subTest(block=block):
                self.assertIn("list-syntax", [f[0] for f in linter.lint(block)])

    def test_comments_do_not_hide_later_rows(self):
        text = 'slots:\n  - {n: 1}\n  # comment\n  - {n: 2}\n'
        self.assertEqual(linter.check_list_syntax(text), [])
        self.assertEqual(len(linter.block_list(text, "slots")[0]), 2)

    def test_column_zero_flow_rows_are_parsed(self):
        text = 'slots:\n- {n: 1}\n- {n: 2}\nnumber_gate:\n'
        self.assertEqual(linter.check_list_syntax(text), [])
        self.assertEqual(len(linter.block_list(text, "slots")[0]), 2)

    def test_trailing_comment_on_list_key_is_not_a_value(self):
        text = 'slots: # rows\n  - {n: 1}\n  - {n: 2}\n'
        self.assertEqual(linter.check_list_syntax(text), [])
        self.assertEqual(linter.block_list(text, "slots"), ([{"n": "1"}, {"n": "2"}], True))

    def test_handoff_template_uses_supported_list_forms(self):
        skill = (ROOT / "SKILL.md").read_text()
        fence = skill[skill.index("### 6. HANDOFF"):].split("```yaml\n", 1)[1].split("```", 1)[0]
        self.assertEqual(linter.check_list_syntax(fence), [])

    def test_empty_slots_fail(self):
        self.assertIn("slots-empty", [f[0] for f in linter.lint('slots:\nnumber_gate:\nglossary:\n')])

if __name__ == "__main__":
    unittest.main()
