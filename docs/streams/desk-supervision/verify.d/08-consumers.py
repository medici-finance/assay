#!/usr/bin/env python3
"""Corroborate brief 08's three routes against its delivered commit, offline.

The original brief predates its implementation, so statusgen's brief-in-diff
precondition cannot hold for that delivery. This bounded check uses the actual
parent-to-delivery diff, never unrelated changes in a wider history window.
"""
import json
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[4]
DELIVERY = "badd7b58c0e1ed6f344384a72e3eb7381fb42d6a"
BRIEF = "docs/streams/desk-supervision/brief-08-objectives-over-transitions.md"
KIT = "tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md"
COMMON = "tools/desk/cmd/deskdispatch/references/common-clauses.md"
SKILL = "plugins/assay/skills/worker-desk/SKILL.md"


def git(*args):
    return subprocess.check_output(["git", "-C", str(ROOT), *args], timeout=20)


def check():
    # These are the complete declared routes, not a general YAML parser.
    text = (ROOT / BRIEF).read_text()
    block = text.split("\nconsumers:\n", 1)[1].split("\nversion:", 1)[0]
    entries = [json.loads(line[4:]) for line in block.splitlines()]
    routes = {entry.split(": ", 1)[0].split(" (", 1)[0]: entry.split(": ", 1)[1] for entry in entries}
    assert len(entries) == len(routes) == 3, "expected exactly three unique consumer routes"
    assert set(routes) == {KIT, COMMON, SKILL}, "consumer inventory changed; review all routes"
    assert routes[KIT].startswith("fixed-here"), "objective kit must be routed fixed-here"
    assert routes[COMMON].startswith("fixed-here"), "common-clause markers changed in delivery; out-of-scope is false"
    assert routes[SKILL].startswith("out-of-scope"), "worker skill must stay out-of-scope"
    paths = set(git("diff", "--name-only", DELIVERY + "^", DELIVERY).decode().splitlines())
    assert KIT in paths and COMMON in paths and SKILL not in paths, "delivery diff contradicts consumer routes"
    assert git("show", DELIVERY + "^:" + COMMON) == git("show", DELIVERY + ":" + COMMON).replace(
        b"<!-- common-clauses:begin -->\n", b""
    ).replace(b"<!-- common-clauses:end -->\n", b""), "common-clause edit was more than extraction markers"
    print("CORROBORATED: 3 consumer routes; common-clause change is markers only; no guard prose changed")


if __name__ == "__main__":
    try:
        check()
    except (AssertionError, ValueError, IndexError, KeyError) as error:
        print("DISPROVED:", error, file=sys.stderr)
        sys.exit(1)
    except (OSError, subprocess.SubprocessError) as error:
        print("could-not-check: historical consumer diff:", error, file=sys.stderr)
        sys.exit(2)
