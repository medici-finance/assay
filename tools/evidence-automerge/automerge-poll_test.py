#!/usr/bin/env python3
"""Execute the staged poll's real shell steps with a stub API, without credentials."""
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / "ci/staged-workflows/evidence-automerge.yml"


def step(name):
    lines = WORKFLOW.read_text().splitlines()
    start = next(i for i, line in enumerate(lines) if line.strip() == "- name: " + name)
    run = next(i for i in range(start + 1, len(lines)) if lines[i].strip() == "run: |")
    indent = len(lines[run]) - len(lines[run].lstrip()) + 2
    body = []
    for line in lines[run + 1:]:
        if line.strip() and len(line) - len(line.lstrip()) < indent:
            break
        body.append(line[indent:])
    return "\n".join(body) + "\n"


STUB = r'''#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
args = sys.argv[1:]
fixture = json.loads(Path(os.environ["API_FIXTURE"]).read_text())
with open(os.environ["API_LOG"], "a") as out:
    out.write(json.dumps(args) + "\n")
if args == ["api", "--paginate", "--slurp", "repos/example/project/pulls?state=open&base=main&per_page=100"]:
    if fixture.get("list_failure"):
        raise SystemExit(1)
    print(json.dumps(fixture["pages"]))
elif args[:2] == ["api", "repos/example/project/pulls/1"]:
    if fixture.get("metadata_failure"):
        raise SystemExit(1)
    assert args[2:] == ["--jq", "[.user.login, (.draft | tostring), .state, .base.ref, .node_id] | @tsv"]
    pr = fixture["current"]
    draft = json.dumps(pr.get("draft"), separators=(",", ":"))
    print("\t".join([pr.get("user", {}).get("login", ""), draft, pr.get("state", ""), pr.get("base", {}).get("ref", ""), pr.get("node_id", "")]))
elif args == ["api", "--paginate", "repos/example/project/pulls/1/files", "--jq", ".[ ].filename".replace(" ", "")]:
    if fixture.get("files_failure"):
        raise SystemExit(1)
    for filename in fixture["files"]:
        print(filename)
else:
    raise SystemExit("unexpected API call: " + repr(args))
'''


def pr(**changes):
    data = {"number": 1, "node_id": "PR_example", "user": {"login": "assay-verifier-app[bot]"},
            "draft": False, "state": "open", "base": {"ref": "main"}}
    data.update(changes)
    return data


class PollTests(unittest.TestCase):
    def test_patch_fences_every_live_key_consumer(self):
        names = {"assay-statusgen.yml", "assay-qualgen.yml", "verify-gate-close.yml",
                 "release.yml", "evidence-automerge.yml"}
        patch = ROOT / "ci/board-writer-migration/workflows.patch"
        targets = set(re.findall(r"^\+\+\+ b/\.github/workflows/(.+)$", patch.read_text(), re.M))
        self.assertEqual(targets, names)
        with tempfile.TemporaryDirectory() as directory:
            tmp = Path(directory)
            workflows = tmp / ".github/workflows"
            workflows.mkdir(parents=True)
            for name in names:
                shutil.copyfile(ROOT / ".github/workflows" / name, workflows / name)
            result = subprocess.run(["git", "apply", str(patch)], cwd=tmp,
                                    text=True, capture_output=True, timeout=10)
            if result.returncode != 0:
                # After maintainer activation the source files already contain
                # the patch. Prove that exact state instead of applying twice.
                reverse = subprocess.run(["git", "apply", "--reverse", "--check", str(patch)], cwd=tmp,
                                         text=True, capture_output=True, timeout=10)
                self.assertEqual(reverse.returncode, 0, result.stderr + reverse.stderr)
            count = 0
            for path in workflows.glob("*.yml"):
                jobs = re.split(r"^  [\w-]+:\n", path.read_text().split("\njobs:\n", 1)[1], flags=re.M)[1:]
                for job in jobs:
                    if "secrets.BOARD_APP_PRIVATE_KEY" in job:
                        self.assertIn("    environment: board-writer\n", job, path.name)
                        count += 1
            self.assertEqual(count, 6)
            self.assertNotIn("  reconcile:\n", (workflows / "assay-statusgen.yml").read_text())
            poll = (workflows / "evidence-automerge.yml").read_text()
            self.assertEqual(poll, WORKFLOW.read_text())
            self.assertNotIn("  pull_request:", poll)
            self.assertNotIn("  pull_request_review:", poll)
            self.assertIn("          ref: ${{ github.sha }}\n", poll)
            self.assertIn("    if: github.ref == 'refs/heads/main'\n", poll)
            self.assertIn("github.ref == 'refs/heads/main'", (workflows / "release.yml").read_text().split("  changelog-roll:\n", 1)[1])

    def run_step(self, name, **changes):
        fixture = {"pages": [[pr()]], "current": pr(), "files": ["docs/streams/example/brief-01.md"]}
        fixture.update(changes)
        with tempfile.TemporaryDirectory() as directory:
            tmp = Path(directory)
            (tmp / "gh").write_text(STUB)
            (tmp / "gh").chmod(0o700)
            (tmp / "fixture.json").write_text(json.dumps(fixture))
            (tmp / "step.sh").write_text(step(name))
            env = dict(os.environ, PATH=str(tmp) + os.pathsep + os.environ["PATH"],
                       API_FIXTURE=str(tmp / "fixture.json"), API_LOG=str(tmp / "calls.jsonl"),
                       GH_TOKEN="offline-stub", REPO="example/project", PR="1", PR_NODE_ID="PR_example",
                       EVIDENCE_AUTHOR="assay-verifier-app[bot]", RUNNER_TEMP=str(tmp), GITHUB_OUTPUT=str(tmp / "output"))
            result = subprocess.run(["bash", "-e", str(tmp / "step.sh")], env=env,
                                    text=True, capture_output=True, timeout=10)
            outputs = {}
            if (tmp / "output").exists():
                for line in (tmp / "output").read_text().splitlines():
                    key, value = line.split("=", 1)
                    outputs[key] = value
            return result, outputs

    def test_discovery_reads_later_pages_and_deduplicates(self):
        result, out = self.run_step("Find ready verifier PRs on main", pages=[
            [pr(user={"login": "another-author"})], [pr(), pr(number=2, node_id="PR_second"), pr()]])
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(out["eligible"], "true")
        self.assertEqual(json.loads(out["matrix"])["include"], [
            {"number": 1, "node_id": "PR_example"}, {"number": 2, "node_id": "PR_second"}])

    def test_discovery_skips_ineligible_prs_and_empty_queue(self):
        for pages in ([], [[]], [[pr(user={"login": "another-author"}), pr(draft=True),
                                  pr(base={"ref": "another-branch"}), pr(state="closed")]]):
            with self.subTest(pages=pages):
                result, out = self.run_step("Find ready verifier PRs on main", pages=pages)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(out, {"matrix": '{"include":[]}', "eligible": "false"})

    def test_discovery_failures_never_emit_eligible_output(self):
        cases = [{"list_failure": True}, {"pages": {}}, {"pages": [[None]]},
                 {"pages": [[pr(number=True)]]}, {"pages": [[pr(node_id="")]]},
                 {"pages": [[pr(), pr(node_id="different")]]},
                 {"pages": [[pr(number=i, node_id="PR_" + str(i)) for i in range(1, 258)]]}]
        for fixture in cases:
            with self.subTest(fixture=fixture):
                result, out = self.run_step("Find ready verifier PRs on main", **fixture)
                self.assertNotEqual(result.returncode, 0)
                self.assertNotEqual(out.get("eligible"), "true")

    def test_guard_accepts_ready_verifier_docs_pr(self):
        result, out = self.run_step("Evaluate the Evidence-PR guards")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(out["eligible"], "true")

    def test_guard_rechecks_current_metadata_and_files(self):
        cases = [{"current": pr(user={"login": "another-author"})}, {"current": pr(draft=True)},
                 {"current": pr(state="closed")}, {"current": pr(base={"ref": "another-branch"})},
                 {"current": pr(node_id="different")}, {"files": []},
                 {"files": ["docs/streams/example/brief-01.md", "tools/run.sh"]},
                 {"files": ["docs/streams/FINDINGS.md"]}, {"files": ["STATUS.md"]}]
        for fixture in cases:
            with self.subTest(fixture=fixture):
                result, out = self.run_step("Evaluate the Evidence-PR guards", **fixture)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(out["eligible"], "false")

    def test_guard_unreadable_or_invalid_data_fails_closed(self):
        for fixture in ({"metadata_failure": True}, {"files_failure": True},
                        {"current": pr(draft=None)}, {"current": pr(user={})}):
            with self.subTest(fixture=fixture):
                result, out = self.run_step("Evaluate the Evidence-PR guards", **fixture)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(out["eligible"], "false")

    def test_shared_evidence_guard_matches_changelog_gate(self):
        def block(path):
            text = path.read_text()
            start = text.index('          # ─── SHARED BLOCK:')
            end = text.index('          # ─── END SHARED BLOCK', start)
            return text[start:end]
        self.assertEqual(block(WORKFLOW), block(ROOT / ".github/workflows/changelog-check.yml"))


if __name__ == "__main__":
    unittest.main()
