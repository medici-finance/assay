"""Offline process tests. Use the real reviewloop and recorded provider observations."""
import fcntl
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import unittest
from datetime import datetime, timedelta, timezone

HERE = Path(__file__).resolve().parent
CASES = HERE.parents[1] / "forgeread/testdata/service/review-observations.json"
REPO = "example-org/tracker"


class ReviewServiceTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.fixtures = json.loads(CASES.read_text())

    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.home = Path(tmp.name)
        self.bin = self.home / "bin"
        self.bin.mkdir()
        self.config = self.home / ".config/assay"
        self.config.mkdir(parents=True)
        (self.config / "READY").touch()
        (self.home / "auth.json").touch()
        (self.home / "fixture.pem").touch()
        self.env = dict(os.environ, HOME=str(self.home), ASSAY_CONFIG_HOME=str(self.config),
                        CODEX_HOME=str(self.home), ASSAY_CELL="sample", ASSAY_DESK="pr-review-desk",
                        ASSAY_REVIEW_MODEL="fixture", ASSAY_CLAIM_ROOT=str(self.home),
                        PATH=f"{self.bin}:{os.environ['PATH']}", KUBECONFIG="/dev/null",
                        ASSAY_APP_PEM_FILE=str(self.home / "fixture.pem"))
        self.tool("deskboard", 'test "$1" = review-queue && test "$2" = --out || exit 5\n'
                  'echo collected >> "$HOME/reads"\ncp "$HOME/observation" "$3"')
        self.tool("codex", 'test -z "$GH_TOKEN$GITHUB_TOKEN" || exit 5\n'
                  'cat >> "$HOME/prompts"\necho >> "$HOME/prompts"\nexit "${FAKE_EXIT:-0}"')
        self.roots = {}
        self.enroll(REPO)
        self.env.update(GH_TOKEN="fixture-inherited", GITHUB_TOKEN="fixture-inherited")

    def tool(self, name, body):
        path = self.bin / name
        path.write_text("#!/bin/sh\nset -e\n" + body + "\n")
        path.chmod(0o755)

    def enroll(self, repo):
        root = self.home / repo
        subprocess.run(["git", "init", "-q", str(root)], check=True)
        subprocess.run(["git", "-C", str(root), "remote", "add", "origin",
                        "https://github.com/" + repo], check=True)
        self.roots[repo] = str(root)
        self.write_roster(self.roots)
        profile = dict(schema="cell-review-v1", cell="sample", repositories=self.roots,
                       selection_policy=dict(Owner="fixture", Revision="v1", Evaluator="review-selection-v1",
                                             Unmapped="observed-score", Priorities=[]))
        (self.home / "profile").write_text(json.dumps(profile))

    def write_roster(self, repos):
        roster = self.config / "roster.env"
        roster.write_text("ASSAY_BLESS_LOGIN=ada:2001\nASSAY_TRUSTED_LOGINS=ada:2001\n"
                          "ASSAY_TRUSTED_BOT_SLUGS=reviewer=fixture-app:300000004\nASSAY_ALLOWED_REPOS="
                          + ",".join(r + ":ci:private" for r in repos) + "\n")
        roster.chmod(0o600)

    def fixture(self, name, head=None):
        raw = next(v["observation"] for v in self.fixtures
                   if v["case"].endswith("/" + name) and isinstance(v.get("observation"), dict))
        data = json.dumps(raw)
        # Preserve intervals and causality, updating only the wall-clock origin.
        ref = datetime(2026, 10, 17, 12, tzinfo=timezone.utc)
        shift = datetime.now(timezone.utc).replace(microsecond=0) - timedelta(seconds=1) - ref
        def current(m):
            return (datetime.fromisoformat(m[0].replace("Z", "+00:00")) + shift).strftime("%Y-%m-%dT%H:%M:%SZ")
        data = re.sub(r"2026-10-\d{2}T\d\d:\d\d:\d\dZ", current, data)
        if head:
            data = data.replace("a" * 40, head)
        (self.home / "observation").write_text(data)
        return json.loads(data)

    def run_once(self, expected, *args):
        command = [sys.executable, str(HERE / "cell-review.py")]
        if os.environ.get("TEST_BAKED_REVIEW") == "1":
            command = ["/usr/local/bin/desk-entrypoint", "/usr/local/bin/cell-review"]
        run = subprocess.run([*command, "--once", "--allow-forge-reads",
                              "--profile", str(self.home / "profile"), "--state-dir", str(self.home / "state"),
                              *args], env=self.env, text=True, capture_output=True, timeout=15)
        self.assertEqual(run.returncode, expected, run.stdout + run.stderr)
        return run

    def prompts(self):
        file = self.home / "prompts"
        return file.read_text().splitlines() if file.exists() else []

    def test_new_head_fix_restart(self):
        self.fixture("idle-and-contradictions")
        self.run_once(0)
        self.assertEqual([], self.prompts())
        self.fixture("unreviewed")
        self.run_once(0)
        self.run_once(2)
        self.fixture("blocked")
        self.run_once(0)
        self.fixture("own-change", "d" * 40)
        self.run_once(0)
        self.run_once(2)
        self.fixture("draft", "d" * 40)
        self.run_once(0)
        one, two = self.prompts()
        self.assertIn(REPO + "#1 at " + "a" * 40, one)
        self.assertIn(REPO + "#1 at " + "d" * 40, two)

    def test_uncertain_exit_and_reconciled_retry(self):
        self.fixture("unreviewed")
        self.env["FAKE_EXIT"] = "9"
        self.run_once(1)
        self.env.pop("FAKE_EXIT")
        self.run_once(2)
        self.run_once(1, "--retry-held", REPO + "#1@" + "d" * 40)
        self.run_once(0, "--retry-held", REPO + "#1@" + "a" * 40)
        self.assertEqual(len(self.prompts()), 2)

    def test_misbound_roster_busy_owner_or_failed_read(self):
        self.fixture("unreviewed")
        self.write_roster(["foreign/repo"])
        self.run_once(1)
        self.assertFalse((self.home / "reads").exists())
        lock = self.home / "state/sample/pr-review-desk/owner.lock"
        with lock.open("a") as owner:
            fcntl.flock(owner, fcntl.LOCK_EX)
            self.run_once(1)
        self.write_roster(self.roots)
        self.tool("deskboard", "exit 6")
        self.run_once(1)
        self.assertEqual([], self.prompts())

    def test_two_repositories_with_same_pr_and_head(self):
        sibling = "example-org/second"
        self.enroll(sibling)
        observation = self.fixture("unreviewed")
        inputs = observation["inputs"]
        def both(rows):
            encoded = json.dumps(rows)
            return json.loads(encoded.replace(REPO, sibling)) + rows
        for field in ("admitted", "requested", "changes", "population"):
            inputs[field] = both(inputs[field])
        result = observation["result"]
        result["rows"] = both(result["rows"])
        for i, row in enumerate(result["rows"], 1):
            row["rank"] = i
        depths = result["depths"]
        depths["total"] = 2
        for field in ("actions", "dispositions"):
            depths[field] = {k: 2*v for k, v in depths[field].items()}
        (self.home / "observation").write_text(json.dumps(observation))
        self.run_once(0)
        self.run_once(0)
        self.run_once(2)
        one, two = self.prompts()
        self.assertIn(sibling + "#1", one)
        self.assertIn(REPO + "#1", two)


if __name__ == "__main__":
    unittest.main()
