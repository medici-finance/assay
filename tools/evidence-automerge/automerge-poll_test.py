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
                # A later reviewed job can change historical patch context.
                # Prove every live security pin below instead of requiring a
                # one-time migration patch to reverse forever.
                self.assert_projected_workflows(workflows)
            self.assert_projected_workflows(workflows)

    def assert_projected_workflows(self, workflows, expected_poll=None):
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
        self.assertEqual(poll, expected_poll or (ROOT / ".github/workflows/evidence-automerge.yml").read_text())
        self.assertEqual(re.findall(r"^\s+EVIDENCE_AUTHOR: (.+)$", poll, re.M),
                         ["assay-verifier-app[bot]", "assay-verifier-app[bot]"])
        self.assertIn("permissions:\n  contents: read\n  pull-requests: read\n", poll.split("\njobs:\n", 1)[0])
        enable = poll.split("\n  enable:\n", 1)[1]
        self.assertIn("    needs: discover\n", enable)
        self.assertIn("    if: github.ref == 'refs/heads/main' && needs.discover.outputs.eligible == 'true'\n", enable)
        self.assertNotIn("  pull_request:", poll)
        self.assertNotIn("  pull_request_review:", poll)
        self.assertIn("          ref: ${{ github.sha }}\n", poll)
        self.assertIn("    if: github.ref == 'refs/heads/main'\n", poll)
        self.assertIn("github.ref == 'refs/heads/main'", (workflows / "release.yml").read_text().split("  changelog-roll:\n", 1)[1])

    def test_proposed_activation_preserves_all_security_pins(self):
        with tempfile.TemporaryDirectory() as directory:
            workflows = Path(directory)
            for name in ("assay-statusgen.yml", "assay-qualgen.yml", "verify-gate-close.yml", "release.yml", "evidence-automerge.yml"):
                shutil.copyfile(ROOT / ".github/workflows" / name, workflows / name)
            shutil.copyfile(WORKFLOW, workflows / "evidence-automerge.yml")
            self.assert_projected_workflows(workflows, WORKFLOW.read_text())
            # A matching staged/live edit must still fail the independently
            # pinned author and main-only gates, not merely the parity check.
            path = workflows / "evidence-automerge.yml"
            original = path.read_text()
            for before, after in (("EVIDENCE_AUTHOR: assay-verifier-app[bot]", "EVIDENCE_AUTHOR: another-author"),
                                  ("github.ref == 'refs/heads/main' &&", "github.ref != 'refs/heads/main' &&")):
                changed = original.replace(before, after)
                path.write_text(changed)
                with self.assertRaises(AssertionError):
                    self.assert_projected_workflows(workflows, changed)
            path.write_text(original)

    def test_environment_and_parity_pins_reject_mutations(self):
        names = ("assay-statusgen.yml", "assay-qualgen.yml", "verify-gate-close.yml",
                 "release.yml", "evidence-automerge.yml")
        # Each mutation names the region of the file holding the pinned control
        # ("" = whole file). The pinned text must occur exactly once in that
        # region, so a later edit that repeats it elsewhere cannot silently
        # retarget the mutation away from the control it exists to prove.
        mutations = (
            ("assay-qualgen.yml", "", "    environment: board-writer\n", ""),
            ("assay-qualgen.yml", "", "secrets.BOARD_APP_PRIVATE_KEY", "secrets.ANOTHER_KEY"),
            ("release.yml", "  changelog-roll:\n", " && github.ref == 'refs/heads/main'", ""),
            ("evidence-automerge.yml", "", "name: evidence-automerge", "name: altered-poll"),
        )
        for name, anchor, before, after in mutations:
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
                    head, found, region = original.partition(anchor) if anchor else ("", "", original)
                    self.assertEqual(found, anchor)
                    # An anchored region ends at the next top-level job key.
                    scope = re.split(r"^  [\w-]+:\n", region, maxsplit=1, flags=re.M)[0] if anchor else region
                    self.assertEqual(scope.count(before), 1, (name, anchor, before))
                    path.write_text(head + found + region.replace(before, after, 1))
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

    KEPT_STEPS = ["Ensure gh CLI is available (pinned, checksum-verified)",
                  "Evaluate the Evidence-PR guards", "Mint board-writer App token",
                  "Check out the decision script (default branch, never the PR tree)",
                  "Request auto-merge"]

    def enable_job(self, path):
        return path.read_text().split("\n  enable:\n", 1)[1]

    def step_names(self, job):
        return re.findall(r"^      - name: (.+)$", job, re.M)

    # regression: #997
    # Main's push/merge restriction refuses github.token, and the driver ruled
    # that clean Evidence PRs wait for a human merge instead of a direct merge.
    # Red on a workflow that still carries the direct-merge step or the job's
    # write grant; green once both are gone, in the staged and live copies.
    def test_no_direct_merge_step(self):
        for path in (WORKFLOW, ROOT / ".github/workflows/evidence-automerge.yml"):
            with self.subTest(workflow=str(path.relative_to(ROOT))):
                text = path.read_text()
                enable = self.enable_job(path)
                self.assertEqual(self.step_names(enable), self.KEPT_STEPS)
                self.assertNotIn("Complete clean Evidence merge", text)
                self.assertNotIn("merge_clean", text)
                self.assertNotIn("steps.request.outputs.clean", text)
                self.assertNotIn(": write\n", text)
                self.assertNotIn("    permissions:", enable.split("    steps:\n", 1)[0])

    # Positive control for the test above: the jobs and the steps that remain
    # keep their identities, tokens and gates, so an absent-step assertion can
    # never pass because the parser stopped finding the job.
    def test_enable_steps_unchanged(self):
        for path in (WORKFLOW, ROOT / ".github/workflows/evidence-automerge.yml"):
            with self.subTest(workflow=str(path.relative_to(ROOT))):
                text = path.read_text()
                self.assertIn("permissions:\n  contents: read\n  pull-requests: read\n\njobs:\n", text)
                discover = text.split("\n  discover:\n", 1)[1].split("\n  enable:\n", 1)[0]
                self.assertEqual(self.step_names(discover), [
                    "Ensure gh CLI is available (pinned, checksum-verified)", "Find ready verifier PRs on main"])
                enable = self.enable_job(path)
                self.assertTrue(enable.startswith(
                    "    needs: discover\n"
                    "    if: github.ref == 'refs/heads/main' && needs.discover.outputs.eligible == 'true'\n"
                    "    environment: board-writer\n"), enable[:300])
                self.assertEqual(self.step_names(enable)[:5], self.KEPT_STEPS)
                steps = re.split(r"^      - name: ", enable, flags=re.M)[1:]
                guard, mint, checkout, request = steps[1:5]
                self.assertIn("        id: guard\n", guard)
                self.assertIn("          GH_TOKEN: ${{ github.token }}\n", guard)
                self.assertIn("        id: app-token\n", mint)
                self.assertIn("        if: steps.guard.outputs.eligible == 'true'\n", mint)
                self.assertIn("          ref: ${{ github.sha }}\n", checkout)
                self.assertIn("          persist-credentials: false\n", checkout)
                self.assertIn("          sparse-checkout: tools/evidence-automerge\n", checkout)
                self.assertIn("        if: steps.guard.outputs.eligible == 'true'\n", request)
                self.assertIn("          GH_TOKEN: ${{ steps.app-token.outputs.token }}\n", request)
                self.assertIn("enablePullRequestAutoMerge", request)
                self.assertIn("bash tools/evidence-automerge/automerge-refusal.sh", request)

    # regression: #997
    # A clean-status refusal is a benign skip that leaves the PR for a human
    # merge. It emits no step output, because no later step consumes one.
    def test_clean_refusal_is_benign(self):
        impl = Path(os.environ.get("REFUSAL_IMPL", str(ROOT / "tools/evidence-automerge/automerge-refusal.sh")))
        with tempfile.TemporaryDirectory() as directory:
            tmp = Path(directory)
            (tmp / "answer").write_text("gh: Pull request Pull request is in clean status")
            env = dict(os.environ, MUTATION_RC="1", MUTATION_OUT=str(tmp / "answer"),
                       SELF_CHECK="enable", PR="2627", GITHUB_OUTPUT=str(tmp / "output"))
            result = subprocess.run(["bash", str(impl)], env=env, text=True, capture_output=True, timeout=10)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("It waits for a human merge.", result.stdout)
            self.assertFalse((tmp / "output").exists(), "clean refusal emitted an unconsumed step output")


if __name__ == "__main__":
    unittest.main()
