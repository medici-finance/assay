#!/usr/bin/env python3
"""RETIRED changelog aggregation, preserved ONLY as fail-first evidence for
aggregate_test.sh. This is the pre-fragment engine: its `highlights` reads the
`## Unreleased` section of CHANGELOG.md and ignores the fragment directory
entirely. So when the pending changes live as FRAGMENTS and `## Unreleased` is
empty, it refuses (exit 2) where the new engine aggregates. Running
  AGG_IMPL=testdata/old-aggregate.py ./aggregate_test.sh
reddens the fragment cases; the default (aggregate.py) greens them. Not wired
into any workflow.
"""
import sys


def _unreleased(path):
    lines = open(path, encoding="utf-8").read().splitlines()
    out, cap = [], False
    for ln in lines:
        if ln.strip().lower() == "## unreleased":
            cap = True
            continue
        if cap and ln.startswith("## "):
            break
        if cap:
            out.append(ln)
    while out and not out[0].strip():
        out.pop(0)
    while out and not out[-1].strip():
        out.pop()
    return "\n".join(out)


def main(argv):
    if len(argv) >= 2 and argv[1] == "highlights":
        # argv[2] = fragment-dir (IGNORED — the retired engine never looked here)
        changelog = argv[3]
        body = _unreleased(changelog)
        if not body.strip():
            sys.stderr.write("(retired) '## Unreleased' is empty — refusing.\n")
            return 2
        sys.stdout.write(body + "\n")
        return 0
    sys.stderr.write("(retired) only 'highlights' is modelled here.\n")
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv))
