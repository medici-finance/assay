#!/usr/bin/env python3
"""Execute the staged poll's real shell steps with a stub API, without credentials."""
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import unittest
import importlib.util
from unittest.mock import patch

sys.dont_write_bytecode = True

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
            self.assert_projected_workflows(workflows)

    def assert_projected_workflows(self, workflows):
        count = 0
        for path in workflows.glob("*.yml"):
            jobs = re.split(r"^  [\w-]+:\n", path.read_text().split("\njobs:\n", 1)[1], flags=re.M)[1:]
            for job in jobs:
                if "secrets.BOARD_APP_PRIVATE_KEY" in job:
                    self.assertIn("    environment: board-writer\n", job, path.name)
                    count += 1
        self.assertEqual(count, 7)
        # The seventh key holder is statusgen's hourly reconcile job (derived-board/04).
        # Pin what makes it acceptable: schedule-only, this repository only, the
        # board-writer environment, and an App token narrowed to contents and
        # pull-requests write and nothing else.
        statusgen = (workflows / "assay-statusgen.yml").read_text()
        reconcile = re.split(r"^  [\w-]+:\n", statusgen.split("\n  reconcile:\n", 1)[1], maxsplit=1, flags=re.M)[0]
        self.assertTrue(reconcile.startswith("    environment: board-writer\n"), reconcile[:200])
        self.assertIn("    if: ${{ github.event_name == 'schedule' && github.repository == 'medici-finance/assay' }}\n", reconcile)
        self.assertEqual(sorted(re.findall(r"^\s+permission-[\w-]+: \w+$", reconcile, re.M)),
                         ["          permission-contents: write", "          permission-pull-requests: write"])
        poll = (workflows / "evidence-automerge.yml").read_text()
        # These fixtures project the existing board-writer migration, not an
        # unrelated pending workflow proposal. Staged proposals are tested below.
        self.assertEqual(poll, (ROOT / ".github/workflows/evidence-automerge.yml").read_text())
        self.assertNotIn("  pull_request:", poll)
        self.assertNotIn("  pull_request_review:", poll)
        self.assertIn("          ref: ${{ github.sha }}\n", poll)
        self.assertIn("    if: github.ref == 'refs/heads/main'\n", poll)
        self.assertIn("github.ref == 'refs/heads/main'", (workflows / "release.yml").read_text().split("  changelog-roll:\n", 1)[1])

    def test_environment_and_parity_pins_reject_mutations(self):
        names = ("assay-statusgen.yml", "assay-qualgen.yml", "verify-gate-close.yml",
                 "release.yml", "evidence-automerge.yml")
        mutations = (
            ("assay-qualgen.yml", "    environment: board-writer\n", ""),
            ("assay-qualgen.yml", "secrets.BOARD_APP_PRIVATE_KEY", "secrets.ANOTHER_KEY"),
            ("release.yml", " && github.ref == 'refs/heads/main'", ""),
            ("evidence-automerge.yml", "name: evidence-automerge", "name: altered-poll"),
        )
        for name, before, after in mutations:
            with self.subTest(workflow=name, mutation=before):
                with tempfile.TemporaryDirectory() as directory:
                    workflows = Path(directory)
                    for filename in names:
                        shutil.copyfile(ROOT / ".github/workflows" / filename, workflows / filename)
                    # This control operates on the final promoted shape even
                    # when the suite is run before maintainer promotion.
                    patch = ROOT / "ci/board-writer-migration/workflows.patch"
                    if "    environment: board-writer\n" not in (workflows / "assay-qualgen.yml").read_text():
                        tree = workflows / ".github/workflows"
                        tree.mkdir(parents=True)
                        for filename in names:
                            shutil.copyfile(workflows / filename, tree / filename)
                        result = subprocess.run(["git", "apply", str(patch)], cwd=workflows,
                                                text=True, capture_output=True, timeout=10)
                        self.assertEqual(result.returncode, 0, result.stderr)
                        for filename in names:
                            shutil.copyfile(tree / filename, workflows / filename)
                    self.assert_projected_workflows(workflows)
                    path = workflows / name
                    original = path.read_text()
                    self.assertIn(before, original)
                    path.write_text(original.replace(before, after, 1))
                    with self.assertRaises(AssertionError):
                        self.assert_projected_workflows(workflows)

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


class CleanMergeTests(unittest.TestCase):
    def setUp(self):
        spec = importlib.util.spec_from_file_location("merge_clean", ROOT / "tools/evidence-automerge/merge_clean.py")
        self.impl = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.impl)
        self.head = "a" * 40
        self.metadata = {"node_id": "PR_example", "user": {"login": "assay-verifier-app[bot]", "type": "Bot"},
                         "head": {"sha": self.head, "repo": {"full_name": "example/project"}},
                         "state": "open", "draft": False, "base": {"ref": "main"}}
        self.view = {"id": "PR_example", "headRefOid": self.head, "state": "OPEN", "isDraft": False,
                     "baseRefName": "main", "mergeable": "MERGEABLE", "mergeStateStatus": "CLEAN",
                     "reviewDecision": "APPROVED", "statusCheckRollup": [
                         {"__typename": "StatusContext", "context": "leak-sweep", "state": "SUCCESS", "createdAt": "2026-10-12T00:00:00Z"},
                         {"__typename": "CheckRun", "name": "test", "workflowName": "ci", "status": "COMPLETED",
                          "conclusion": "SUCCESS", "completedAt": "2026-10-12T00:00:00Z"}]}
        self.files = [[{"filename": "docs/streams/example/brief-01.md", "status": "modified"}]]
        self.reviews = [[self.review("Security-Review: pass", "COMMENTED"), self.review("Verdict: approve", "APPROVED")]]
        self.calls = []
        self.merge_result = {"merged": True}
        self.pr_number = "1"

    def review(self, body, state, stamp="2026-10-12T00:00:00Z"):
        return {"commit_id": self.head, "user": {"login": "assay-reviewer-app[bot]", "type": "Bot"},
                "submitted_at": stamp, "body": body, "state": state}

    def call(self, args, body=None):
        self.calls.append((args, body))
        if args[:2] == ["api", "--method"]:
            self.assertEqual(args, ["api", "--method", "PUT", "repos/example/project/pulls/" + self.pr_number + "/merge", "--input", "-"])
            self.assertEqual(body, {"sha": self.head, "merge_method": "merge"})
            return self.merge_result
        if args[:2] == ["pr", "view"]:
            return self.view
        if args[-1].endswith("/files?per_page=100"):
            return self.files
        if args[-1].endswith("/reviews?per_page=100"):
            return self.reviews
        return self.metadata

    def run_merge(self):
        with patch.dict(os.environ, REPO="example/project", PR=self.pr_number, PR_NODE_ID=self.metadata["node_id"]):
            return self.impl.run(self.call)

    def writes(self):
        return [call for call in self.calls if call[1] is not None]

    def test_clean_merge_completes_at_exact_reviewed_head(self):
        self.assertEqual(self.run_merge(), 0)
        self.assertEqual(len(self.writes()), 1)

    def test_clean_merge_second_candidate_uses_its_own_identity(self):
        self.pr_number = "2685"
        self.metadata["node_id"] = self.view["id"] = "PR_second"
        self.assertEqual(self.run_merge(), 0)
        self.assertEqual(len(self.writes()), 1)

    def test_clean_refusal_signals_completion(self):
        impl = Path(os.environ.get("REFUSAL_IMPL", str(ROOT / "tools/evidence-automerge/automerge-refusal.sh")))
        with tempfile.TemporaryDirectory() as directory:
            tmp = Path(directory)
            (tmp / "answer").write_text("gh: Pull request Pull request is in clean status")
            env = dict(os.environ, MUTATION_RC="1", MUTATION_OUT=str(tmp / "answer"),
                       SELF_CHECK="enable", PR="2627", GITHUB_OUTPUT=str(tmp / "output"))
            result = subprocess.run(["bash", str(impl)], env=env, text=True, capture_output=True, timeout=10)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertTrue((tmp / "output").exists(), "clean refusal discarded: no completion signal")
            self.assertEqual((tmp / "output").read_text(), "clean=true\n")

    def test_clean_merge_rechecks_candidate_state(self):
        for target, key, value in (("metadata", "draft", True), ("metadata", "state", "closed"),
                                   ("metadata", "node_id", "PR_other"), ("view", "isDraft", True),
                                   ("view", "headRefOid", "b" * 40), ("view", "baseRefName", "other"),
                                   ("view", "reviewDecision", "CHANGES_REQUESTED"),
                                   ("view", "mergeStateStatus", "BLOCKED"), ("view", "mergeable", "UNKNOWN")):
            with self.subTest(target=target, key=key):
                obj = getattr(self, target)
                old = obj[key]
                obj[key] = value
                self.calls.clear()
                self.assertEqual(self.run_merge(), 0)
                self.assertEqual(self.writes(), [])
                obj[key] = old
        for key, value in (("user", {"login": "other", "type": "Bot"}),
                           ("user", {"login": "assay-verifier-app[bot]", "type": "User"}),
                           ("base", {"ref": "other"}),
                           ("head", {"sha": self.head, "repo": {"full_name": "other/project"}})):
            with self.subTest(key=key, value=value):
                old = self.metadata[key]
                self.metadata[key] = value
                self.calls.clear()
                self.assertEqual(self.run_merge(), 0)
                self.assertEqual(self.writes(), [])
                self.metadata[key] = old

    def test_clean_merge_requires_both_current_head_review_lanes(self):
        cases = [[], [self.review("Verdict: approve", "APPROVED")],
                 [self.review("Security-Review: pass", "COMMENTED")],
                 [self.review("Security-Review: fail", "COMMENTED"), self.review("Verdict: approve", "APPROVED")],
                 [self.review("Security-Review: pass", "COMMENTED"), self.review("Verdict: request-changes", "CHANGES_REQUESTED")]]
        stale = self.review("Verdict: approve", "APPROVED")
        stale["commit_id"] = "b" * 40
        cases.append([self.review("Security-Review: pass", "COMMENTED"), stale])
        cases.append(self.reviews[0] + [self.review("Security-Review: fail", "COMMENTED", "2026-10-12T01:00:00Z")])
        dismissed = self.review("Verdict: approve", "DISMISSED", "2026-10-12T01:00:00Z")
        cases.append(self.reviews[0] + [dismissed])
        impostor = self.review("Verdict: approve", "APPROVED")
        impostor["user"]["login"] = "other[bot]"
        cases.append([self.review("Security-Review: pass", "COMMENTED"), impostor])
        for reviews in cases:
            with self.subTest(reviews=reviews):
                self.reviews = [reviews]
                self.calls.clear()
                self.assertEqual(self.run_merge(), 0)
                self.assertEqual(self.writes(), [])

    def test_clean_merge_requires_docs_scope_on_every_page(self):
        for file in ({"filename": "tools/run.sh"}, {"filename": "docs/streams/FINDINGS.md"},
                     {"filename": "docs/streams/INTAKE.md"}, {"filename": "docs/streams/../run.sh"},
                     {"filename": "docs/streams/example/x.md", "status": "renamed", "previous_filename": "tools/run.sh"}):
            with self.subTest(file=file):
                self.files = [[{"filename": "docs/streams/example/brief-01.md"}], [file]]
                self.calls.clear()
                self.assertEqual(self.run_merge(), 0)
                self.assertEqual(self.writes(), [])

    def test_clean_merge_holds_on_bad_or_pending_checks(self):
        original = json.loads(json.dumps(self.view["statusCheckRollup"]))
        for status, conclusion in (("COMPLETED", "FAILURE"), ("IN_PROGRESS", None), ("QUEUED", None), ("COMPLETED", "CANCELLED")):
            with self.subTest(status=status, conclusion=conclusion):
                self.view["statusCheckRollup"] = json.loads(json.dumps(original))
                self.view["statusCheckRollup"][1].update(status=status, conclusion=conclusion)
                self.calls.clear()
                self.assertEqual(self.run_merge(), 0)
                self.assertEqual(self.writes(), [])
        self.view["statusCheckRollup"] = original[1:]
        self.calls.clear()
        self.assertEqual(self.run_merge(), 0)
        self.assertEqual(self.writes(), [])

    def test_clean_merge_superseded_checks_do_not_hide_other_workflows(self):
        old = dict(self.view["statusCheckRollup"][1], conclusion="CANCELLED", completedAt="2026-10-11T00:00:00Z")
        self.view["statusCheckRollup"].append(old)
        reduced = self.impl.latest_run_per_check(self.view["statusCheckRollup"])
        self.assertEqual(len(reduced), 2)
        self.assertTrue(all(entry.get("conclusion") != "CANCELLED" for entry in reduced))
        self.assertEqual(self.run_merge(), 0)
        other = dict(old, workflowName="other", completedAt="2026-10-12T01:00:00Z")
        self.view["statusCheckRollup"].append(other)
        self.calls.clear()
        self.assertEqual(self.run_merge(), 0)
        self.assertEqual(self.writes(), [])

    def test_clean_merge_read_errors_and_server_refusal_fail_closed(self):
        with patch.dict(os.environ, REPO="example/project", PR="1", PR_NODE_ID="PR_example"):
            self.assertEqual(self.impl.run(lambda *args: (_ for _ in ()).throw(RuntimeError("read refused"))), 1)
        self.assertEqual(self.writes(), [])
        self.files = {"not": "pages"}
        self.assertEqual(self.run_merge(), 1)
        self.assertEqual(self.writes(), [])
        self.files = [[{"filename": "docs/streams/example/brief-01.md"}]]
        self.merge_result = {"merged": False, "message": "head or protection changed"}
        self.assertEqual(self.run_merge(), 1)

    def test_clean_merge_wiring(self):
        text = WORKFLOW.read_text()
        enable = text.split("\n  enable:\n", 1)[1]
        self.assertIn("    permissions:\n      contents: write\n      pull-requests: write\n", enable)
        self.assertIn("        id: request\n", enable)
        merge = enable.split("      - name: Complete clean Evidence merge\n", 1)[1]
        self.assertIn("steps.guard.outputs.eligible == 'true' && steps.request.outputs.clean == 'true'", merge)
        self.assertIn("          GH_TOKEN: ${{ github.token }}\n", merge)
        self.assertNotIn("steps.app-token.outputs.token", merge)
        self.assertIn("merge_clean.run()", merge)

    def test_real_completion_step_uses_protected_token_and_head(self):
        for head_changed in (False, True):
            with self.subTest(head_changed=head_changed), tempfile.TemporaryDirectory() as directory:
                tmp = Path(directory)
                fixture = {"metadata": self.metadata, "view": self.view, "files": self.files,
                           "reviews": self.reviews, "head_changed": head_changed}
                (tmp / "fixture").write_text(json.dumps(fixture))
                (tmp / "step").write_text(step("Complete clean Evidence merge"))
                (tmp / "gh").write_text(r'''#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
assert os.environ["GH_TOKEN"] == "workflow-token", "bypass token reached direct merge"
data = json.loads(Path(os.environ["MERGE_FIXTURE"]).read_text())
args = sys.argv[1:]
if args[:2] == ["api", "--method"]:
    body = json.load(sys.stdin)
    assert args == ["api", "--method", "PUT", "repos/example/project/pulls/1/merge", "--input", "-"]
    assert body == {"sha": data["metadata"]["head"]["sha"], "merge_method": "merge"}
    Path(os.environ["MERGE_ATTEMPT"]).write_text(json.dumps(body))
    if data["head_changed"]:
        print("head changed: merge rejected", file=sys.stderr)
        raise SystemExit(1)
    print(json.dumps({"merged": True}))
elif args[:2] == ["pr", "view"]:
    print(json.dumps(data["view"]))
elif args[-1].endswith("/files?per_page=100"):
    print(json.dumps(data["files"]))
elif args[-1].endswith("/reviews?per_page=100"):
    print(json.dumps(data["reviews"]))
elif args == ["api", "repos/example/project/pulls/1"]:
    print(json.dumps(data["metadata"]))
else:
    raise SystemExit("unexpected API call")
''')
                (tmp / "gh").chmod(0o700)
                env = dict(os.environ, PATH=str(tmp) + os.pathsep + os.environ["PATH"],
                           GH_TOKEN="workflow-token", REPO="example/project", PR="1", PR_NODE_ID="PR_example",
                           MERGE_FIXTURE=str(tmp / "fixture"), MERGE_ATTEMPT=str(tmp / "attempt"))
                result = subprocess.run(["bash", "-e", str(tmp / "step")], env=env, cwd=ROOT,
                                        text=True, capture_output=True, timeout=10)
                self.assertEqual(result.returncode, 1 if head_changed else 0, result.stdout + result.stderr)
                self.assertEqual(json.loads((tmp / "attempt").read_text())["sha"], self.head)


if __name__ == "__main__":
    unittest.main()
