"""Complete a clean Evidence PR using the non-bypass workflow token.

Called only by the reviewed main workflow after auto-merge reports CLEAN.
Every merge carries the inspected head SHA; GitHub enforces branch protection
again at the write, including changes made after these read-only checks.
"""
import json
import os
import re
import subprocess


FIELDS = "id,headRefOid,state,isDraft,baseRefName,mergeable,mergeStateStatus,reviewDecision,statusCheckRollup"


def command_json(args, body=None):
    result = subprocess.run(["gh", *args], input=None if body is None else json.dumps(body),
                            text=True, capture_output=True, timeout=60)
    if result.returncode:
        raise RuntimeError("GitHub read/write refused: " + result.stderr.strip())
    return json.loads(result.stdout)


def pages(value):
    if not isinstance(value, list) or not all(isinstance(page, list) for page in value):
        raise ValueError("invalid paginated GitHub response")
    return [item for page in value for item in page]


def reviewed(reviews, head):
    lanes = {}
    for review in reviews:
        if not isinstance(review, dict):
            raise ValueError("invalid review")
        if review.get("commit_id") != head or review.get("user", {}).get("login") != "assay-reviewer-app[bot]":
            continue
        if review.get("user", {}).get("type") != "Bot":
            continue
        body = review.get("body", "")
        if not isinstance(body, str) or not isinstance(review.get("submitted_at"), str):
            raise ValueError("incomplete current-head review")
        for lane, pattern in (("security", r"^Security-Review: (pass|fail)\s*$"),
                              ("correctness", r"^Verdict: (approve|request-changes)\s*$")):
            verdicts = re.findall(pattern, body, re.M)
            if not verdicts:
                continue
            if len(set(verdicts)) != 1:
                raise ValueError("conflicting review verdicts")
            stamp = review["submitted_at"]
            if lane not in lanes or stamp >= lanes[lane][0]:
                lanes[lane] = (stamp, verdicts[0], review.get("state"))
    security = lanes.get("security", (None, None, None))
    correctness = lanes.get("correctness", (None, None, None))
    return security[1] == "pass" and security[2] in ("COMMENTED", "APPROVED") and \
        correctness[1:] == ("approve", "APPROVED")


def latest_run_per_check(entries):
    if not isinstance(entries, list) or not entries:
        raise ValueError("missing check rollup")
    latest = {}
    for entry in entries:
        if not isinstance(entry, dict):
            raise ValueError("invalid check entry")
        status = entry.get("__typename") == "StatusContext"
        name = entry.get("context") if status else entry.get("name")
        workflow = "status" if status else entry.get("workflowName")
        if not name or not workflow:
            raise ValueError("check identity cannot be established")
        key = (workflow, name)
        stamp = next((entry[k] for k in ("completedAt", "startedAt", "createdAt")
                      if isinstance(entry.get(k), str) and not entry[k].startswith("0001-")), "")
        if key not in latest or stamp > latest[key][0]:
            latest[key] = (stamp, entry)
    return [entry for _, entry in latest.values()]


def checks_green(entries):
    latest = latest_run_per_check(entries)
    if not any(entry.get("__typename") == "StatusContext" and entry.get("context") == "leak-sweep" for entry in latest):
        return False
    for entry in latest:
        if entry.get("__typename") == "StatusContext":
            if entry.get("state") != "SUCCESS":
                return False
        elif entry.get("status") != "COMPLETED" or entry.get("conclusion") not in ("SUCCESS", "NEUTRAL", "SKIPPED"):
            return False
    return True


def docs_only(files):
    if not files:
        return False
    for file in files:
        if not isinstance(file, dict):
            raise ValueError("invalid changed-file entry")
        for path in [file.get("filename"), *([file.get("previous_filename")] if file.get("status") == "renamed" else [])]:
            if not isinstance(path, str) or not path.startswith("docs/streams/"):
                return False
            if path in ("docs/streams/FINDINGS.md", "docs/streams/INTAKE.md"):
                return False
            if any(part in (".", "..", "") for part in path.split("/")):
                return False
    return True


def run(call=command_json):
    try:
        repo, pr, node = (os.environ[k] for k in ("REPO", "PR", "PR_NODE_ID"))
        if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repo) or not re.fullmatch(r"[1-9][0-9]*", pr) or not node:
            raise ValueError("invalid candidate identity")
        prefix = "repos/" + repo + "/pulls/" + pr
        metadata = call(["api", prefix])
        view = call(["pr", "view", pr, "--repo", repo, "--json", FIELDS])
        head = metadata.get("head", {}).get("sha")
        eligible = (metadata.get("node_id") == node == view.get("id")
                    and metadata.get("user", {}).get("login") == "assay-verifier-app[bot]"
                    and metadata.get("user", {}).get("type") == "Bot"
                    and metadata.get("head", {}).get("repo", {}).get("full_name") == repo
                    and metadata.get("state") == "open" and metadata.get("draft") is False
                    and metadata.get("base", {}).get("ref") == "main"
                    and view.get("state") == "OPEN" and view.get("isDraft") is False
                    and view.get("baseRefName") == "main" and view.get("headRefOid") == head
                    and view.get("mergeable") == "MERGEABLE" and view.get("mergeStateStatus") == "CLEAN"
                    and view.get("reviewDecision") == "APPROVED"
                    and isinstance(head, str) and re.fullmatch(r"[a-f0-9]{40}", head))
        if not eligible:
            print("clean merge held: candidate state changed or approvals are incomplete")
            return 0
        files = pages(call(["api", "--paginate", "--slurp", prefix + "/files?per_page=100"]))
        reviews = pages(call(["api", "--paginate", "--slurp", prefix + "/reviews?per_page=100"]))
        if not docs_only(files) or not reviewed(reviews, head) or not checks_green(view.get("statusCheckRollup")):
            print("clean merge held: Evidence scope, current-head review lanes or checks are incomplete")
            return 0
        # Do not use the board-writer App here: it bypasses main's rulesets.
        # This step receives only github.token; the REST write independently
        # enforces protection and atomically rejects a different head SHA.
        result = call(["api", "--method", "PUT", prefix + "/merge", "--input", "-"],
                      {"sha": head, "merge_method": "merge"})
        if result.get("merged") is not True:
            raise RuntimeError("GitHub did not confirm the Evidence merge")
        print("merged reviewed Evidence PR " + pr + " at inspected head " + head)
        return 0
    except (KeyError, TypeError, ValueError, AttributeError, RuntimeError, subprocess.TimeoutExpired) as error:
        print("::error::could-not-check or refused clean Evidence merge: " + str(error))
        return 1
