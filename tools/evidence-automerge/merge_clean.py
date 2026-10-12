"""Complete a clean Evidence PR using the non-bypass workflow token.

Called only by the reviewed main workflow after auto-merge reports CLEAN.
Every merge carries the inspected head SHA; GitHub enforces branch protection
again at the write, including changes made after these read-only checks.
"""
import json
import os
import re
import subprocess
from datetime import datetime


# Unicode White_Space used by Go strings.TrimSpace; Python also strips
# four ASCII separators which are content in the canonical declaration reader.
GO_SPACE = " \t\n\v\f\r\x85\xa0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000"

FIELDS = "id,headRefOid,state,isDraft,baseRefName,mergeable,mergeStateStatus,reviewDecision"


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


def marker_values(body, pattern, skip_fenced=False):
    # Same delimiter/emphasis rules as deskkit/verdictmarker.go; the Go
    # regression exercises this adapter against the canonical readers.
    values, fenced = [], False
    for line in body.split("\n"):
        if re.match(r"^[ \t]*(```|~~~)", line):
            fenced = not fenced
            continue
        if fenced and skip_fenced:
            continue
        line = line.strip(" \t\r")
        def unwrap(match):
            i, j = match.span()
            if i == 0 and j == len(line):
                return match[0]
            if i == 0:
                remove = line[j] not in " \t"
            elif j == len(line):
                remove = line[i - 1] not in " \t"
            else:
                remove = not (line[i - 1].isascii() and line[i - 1].isalnum()
                              and line[j].isascii() and line[j].isalnum())
            return "" if remove else match[0]
        normalized = re.sub(r"[*_`]+", unwrap, line)
        # Go simple-folding of ASCII letters includes long-s and Kelvin K,
        # but not Python IGNORECASE's extra dotted/dotless-I matches.
        folded = normalized.translate({0x017f: ord("s"), 0x212a: ord("k")})
        match = re.fullmatch(pattern, folded, re.I | re.ASCII)
        if match:
            values.append(match[1] if match.lastindex else normalized)
    return values


def declaration_clear(body, labels):
    # Fences affect section headings only. Keep the raw section contents:
    # the canonical declaration parser reads every numbered item in it.
    section, fenced = None, False
    for line in body.split("\n"):
        delimiter = bool(re.match(r"^[ \t]*(```|~~~)", line))
        if delimiter:
            fenced = not fenced
        if not delimiter and not fenced:
            if section is None and line.strip(GO_SPACE) == "## Desk-decided":
                section = []
                continue
            if section is not None and re.match(r"^#{1,6}[ \t]", line):
                break
        if section is not None:
            section.append(line)
    labelled = any(label.translate({0x017f: ord("s"), 0x212a: ord("k")}).lower() == "desk-decided"
                   for label in labels)
    if labelled != (section is not None):
        return False
    if section is None:
        return True
    text = "\n".join(section)
    marker = "<!-- desk-r3-decision v1 -->"
    if marker not in text:
        return False
    text = text.split(marker, 1)[1]
    items = list(re.finditer(r"(?m)^[ \t]*[0-9]+\.[ \t]*decision:[ \t]*(.*)$", text))
    if not items:
        return False
    for i, item in enumerate(items):
        chunk = text[item.start():items[i + 1].start() if i + 1 < len(items) else len(text)]
        if not item[1].strip(GO_SPACE):
            return False
        for field in ("alternative", "cost"):
            value = re.search(r"(?im)^[ \t]*" + field + r":[ \t]*(.*)$",
                              chunk.translate({0x017f: ord("s"), 0x212a: ord("k")}), re.ASCII)
            if not value or not value[1].strip(GO_SPACE):
                return False
    return True


def reviewed(reviews, head, body="", labels=()):
    if not declaration_clear(body, labels):
        return False
    security, correctness, correctness_blocked = None, None, False
    holds = {"security": False, "correctness": False}
    timeline, identities = [], set()
    for review in reviews:
        if not isinstance(review, dict):
            raise ValueError("invalid review")
        if review.get("user", {}).get("login") != "assay-reviewer-app[bot]" or review.get("user", {}).get("type") != "Bot":
            continue
        if not isinstance(review.get("body"), str) or not isinstance(review.get("submitted_at"), str):
            raise ValueError("incomplete review")
        identity = review.get("id")
        if type(identity) is not int or identity < 1 or identity in identities:
            raise ValueError("review artifact identity cannot be established")
        identities.add(identity)
        try:
            datetime.fromisoformat(review["submitted_at"].replace("Z", "+00:00"))
        except ValueError as error:
            raise ValueError("review submission time cannot be established") from error
        timeline.append(review)
    # Stable sort retains the forge's submission order for equal timestamps.
    for review in sorted(timeline, key=lambda r: r["submitted_at"]):
        text, state, current = review["body"], review.get("state"), review.get("commit_id") == head
        sec_pass = bool(marker_values(text, r"Security-Review:[ \t]*pass", True))
        sec_fail = bool(marker_values(text, r"Security-Review:[ \t]*fail"))
        cor_grant = marker_values(text, r"Verdict:[ \t]*(approve|request-changes)", True)
        cor_block = bool(marker_values(text, r"Verdict:[ \t]*request-changes"))
        # Artifact kinds read both fence directions. A quoted grant is not
        # authority, and a fenced other-lane verdict cannot hide a block or
        # make a dual-kind artifact count as either required grant.
        sec_lane = bool(marker_values(text, r"Security-Review:[ \t]*(pass|fail)"))
        cor_kind = marker_values(text, r"Verdict:[ \t]*(approve|request-changes)")
        mixed = sec_lane and bool(cor_kind)
        # Declaration holds use the canonical grant/fail lane discriminator,
        # separately from the stricter dual-kind artifact check above.
        hold_security_lane = sec_pass or sec_fail
        lane = "security" if hold_security_lane else "correctness"
        if current:
            if marker_values(text, r"Undeclared-desk-decision:[ \t]*(.+)"):
                holds[lane] = True
            elif hold_security_lane or state in ("APPROVED", "CHANGES_REQUESTED"):
                holds[lane] = False
        if sec_fail:
            security = False  # retractions survive a head move and every state
        elif sec_pass and current and not mixed:
            security = state in ("COMMENTED", "APPROVED")
        if current and (not sec_lane or mixed):
            if state == "CHANGES_REQUESTED" or cor_block:
                correctness_blocked = True
                correctness = False
            elif not mixed and cor_grant:
                correctness = state == "APPROVED" and not correctness_blocked
    return security is True and correctness is True and not any(holds.values())


def latest_run_per_check(entries):
    if not isinstance(entries, list) or not entries:
        raise ValueError("missing check rollup")
    groups = {}
    for entry in entries:
        if not isinstance(entry, dict):
            raise ValueError("invalid check entry")
        status = entry.get("__typename") == "StatusContext"
        name = entry.get("context") if status else entry.get("name")
        workflow = "status" if status else entry.get("workflowId", entry.get("workflowName"))
        if not name or not workflow:
            raise ValueError("check identity cannot be established")
        groups.setdefault((entry.get("appId"), workflow, name), []).append(entry)
    selected = []
    for group in groups.values():
        if len(group) == 1:
            selected.extend(group)
            continue
        status = group[0].get("__typename") == "StatusContext"
        field = "createdAt" if status else "checkSuiteCreatedAt"
        # Completion/start time never orders attempts. The adapter reads
        # LATEST within each suite. Across rerun suites there is no comparable
        # attempt creation time, so inspect every report rather than guess.
        if not status and any(entry.get("runAttempt", 0) != 1 for entry in group):
            selected.extend(group)
            continue
        stamps = [entry.get(field) for entry in group]
        if any(not isinstance(stamp, str) or not re.fullmatch(r"[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:.]+Z", stamp) or stamp.startswith("0001-") for stamp in stamps):
            selected.extend(group)
            continue
        newest = max(stamps)
        selected.extend(entry for entry in group if entry[field] == newest)
    return selected


def read_checks(call, repo, head):
    owner, name = repo.split("/")
    query = """query($owner:String!,$name:String!,$head:String!,$after:String){
      repository(owner:$owner,name:$name){object(expression:$head){... on Commit{
        oid status{contexts{__typename context state createdAt}}
        checkSuites(first:100,after:$after){totalCount pageInfo{hasNextPage endCursor} nodes{
          id createdAt app{id} workflowRun{runAttempt workflow{id name}}
          checkRuns(first:100,filterBy:{checkType:LATEST}){
            totalCount pageInfo{hasNextPage} nodes{id name status conclusion}}}}}}}}
    """
    cursor, seen, entries, suites = None, set(), [], []
    while True:
        args = ["api", "graphql", "-f", "query=" + query, "-f", "owner=" + owner,
                "-f", "name=" + name, "-f", "head=" + head]
        if cursor:
            args += ["-f", "after=" + cursor]
        response = call(args)
        if response.get("errors"):
            raise ValueError("check provenance query failed")
        commit = response["data"]["repository"]["object"]
        if commit.get("oid") != head:
            raise ValueError("check provenance read at another head")
        if cursor is None:
            entries.extend((commit.get("status") or {}).get("contexts", []))
        connection = commit["checkSuites"]
        suites.extend(connection["nodes"])
        page = connection["pageInfo"]
        if not page["hasNextPage"]:
            if len(suites) != connection["totalCount"]:
                raise ValueError("incomplete check suites")
            break
        cursor = page["endCursor"]
        if not cursor or cursor in seen:
            raise ValueError("incomplete check pagination")
        seen.add(cursor)
    if len({suite["id"] for suite in suites}) != len(suites):
        raise ValueError("duplicate suite identities in the provenance read")
    identities = set()
    for suite in suites:
        connection = suite["checkRuns"]
        if connection["pageInfo"]["hasNextPage"] or len(connection["nodes"]) != connection["totalCount"]:
            raise ValueError("incomplete current suite checks")
        workflow = suite.get("workflowRun")
        for check in connection["nodes"]:
            if not check.get("id") or check["id"] in identities:
                raise ValueError("check artifact identity cannot be established")
            identities.add(check["id"])
            entries.append(dict(check, __typename="CheckRun", appId=suite["app"]["id"],
                checkSuiteId=suite["id"], checkSuiteCreatedAt=suite["createdAt"],
                workflowId=workflow["workflow"]["id"] if workflow else suite["id"],
                workflowName=workflow["workflow"]["name"] if workflow else suite["id"],
                runAttempt=workflow["runAttempt"] if workflow else 0))
    return entries


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


def candidate_eligible(metadata, view, node, repo):
    head = metadata.get("head", {}).get("sha")
    return (metadata.get("node_id") == node == view.get("id")
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


def run(call=command_json):
    try:
        repo, pr, node = (os.environ[k] for k in ("REPO", "PR", "PR_NODE_ID"))
        if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repo) or not re.fullmatch(r"[1-9][0-9]*", pr) or not node:
            raise ValueError("invalid candidate identity")
        prefix = "repos/" + repo + "/pulls/" + pr
        metadata = call(["api", prefix])
        view = call(["pr", "view", pr, "--repo", repo, "--json", FIELDS])
        head = metadata.get("head", {}).get("sha")
        eligible = candidate_eligible(metadata, view, node, repo)
        if not eligible:
            print("clean merge held: candidate state changed or approvals are incomplete")
            return 0
        for snapshot in range(2):
            if snapshot:
                metadata = call(["api", prefix])
                view = call(["pr", "view", pr, "--repo", repo, "--json", FIELDS])
                if metadata.get("head", {}).get("sha") != head or not candidate_eligible(metadata, view, node, repo):
                    print("clean merge held: candidate changed during clearance reads")
                    return 0
            if not isinstance(metadata.get("body"), (str, type(None))) or "body" not in metadata or not isinstance(metadata.get("labels"), list):
                raise ValueError("incomplete declaration metadata")
            labels = [label["name"] for label in metadata["labels"]]
            if not all(isinstance(label, str) for label in labels):
                raise ValueError("invalid declaration labels")
            files = pages(call(["api", "--paginate", "--slurp", prefix + "/files?per_page=100"]))
            if type(metadata.get("changed_files")) is not int or len(files) != metadata["changed_files"]:
                raise ValueError("incomplete changed-file read")
            reviews = pages(call(["api", "--paginate", "--slurp", prefix + "/reviews?per_page=100"]))
            if not docs_only(files) or not reviewed(reviews, head, metadata["body"] or "", labels) or not checks_green(read_checks(call, repo, head)):
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
