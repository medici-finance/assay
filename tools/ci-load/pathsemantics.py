#!/usr/bin/env python3
"""pathsemantics.py — reproduce GitHub's `paths-ignore` decision for the staged filter set.

WHY THIS EXISTS. The whole safety argument for the `paths-ignore` filters staged in
`activation/` is one sentence of GitHub's semantics: a workflow is skipped ONLY when
EVERY changed file is ignored, so a mixed diff always runs in full and a filter can
never half-skip. That sentence is load-bearing and it is easy to get backwards — the
opposite reading ("skip when ANY file is ignored") would silently drop the Go build
from every pull request that also touched a brief.

So it is asserted here as a NEGATIVE CONTROL rather than trusted: the docs-only case
must skip, and the mixed and code-only cases must NOT. Two of the three expectations
are inversions, which is what makes a green run mean something.

The filter list is READ FROM the staged workflow files, not retyped — a divergence
between this check and what actually ships is the failure it would otherwise miss.

Usage:  python3 tools/ci-load/pathsemantics.py
Exit:   0 when all cases match, 1 otherwise (each case printed either way).
"""

import re
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
STAGED = HERE / "activation"

# The workflows whose skip decision this reproduces, and the diff shapes to try.
TARGETS = ["ci.yml", "plugin-drift.yml"]

CASES = [
    # (label, changed files, expected "workflow is skipped")
    ("docs-only", ["docs/streams/desk-supervision/brief-09-ci-fanout-per-push.md",
                   "docs/streams/desk-supervision/README.md",
                   "changelog/ci-per-push-fanout.md"], True),
    ("mixed", ["docs/streams/desk-supervision/README.md",
               "tools/desk/cmd/deskflip/flip.go"], False),
    ("go-only", ["statusgen/main.go"], False),
    ("status-regen", ["STATUS.md"], True),
    ("plugin-only", ["plugins/assay/skills/worker-desk/SKILL.md"], False),
]


def read_paths_ignore(path: Path) -> list[str]:
    """Pull the FIRST `paths-ignore:` list out of a workflow file, as written."""
    lines = path.read_text().splitlines()
    out: list[str] = []
    inside = False
    for ln in lines:
        if re.match(r"^\s*paths-ignore:\s*$", ln):
            if inside:
                break  # only the first block; the second leg repeats it
            inside = True
            continue
        if inside:
            m = re.match(r"""^\s*-\s*["']?([^"'#]+?)["']?\s*$""", ln)
            if m:
                out.append(m.group(1))
                continue
            break
    return out


def ignored(pattern: str, path: str) -> bool:
    """One `paths-ignore` pattern against one changed file.

    Only the two glob shapes the staged filters actually use are modelled: a
    `dir/**` prefix and a literal path. Anything else raises rather than guessing —
    a pattern this cannot model must not be reported as a clean result.
    """
    if pattern.endswith("/**"):
        return path.startswith(pattern[:-2])
    if "*" in pattern or "?" in pattern or "[" in pattern:
        raise ValueError(f"unmodelled glob shape: {pattern!r}")
    return path == pattern


def skipped(patterns: list[str], files: list[str]) -> bool:
    """GitHub's rule: skip ONLY when EVERY changed file is ignored."""
    if not files:
        return False
    return all(any(ignored(p, f) for p in patterns) for f in files)


def main() -> int:
    rc = 0
    for wf in TARGETS:
        path = STAGED / wf
        pats = read_paths_ignore(path)
        if not pats:
            print(f"FAIL {wf}: no paths-ignore block found — cannot check")
            rc = 1
            continue
        print(f"{wf}: paths-ignore = {pats}")
        for label, files, want in CASES:
            got = skipped(pats, files)
            ok = got == want
            rc = rc or (0 if ok else 1)
            print(f"  {'ok  ' if ok else 'FAIL'} {label}: skipped={got} (want {want})")
    print("PASS" if rc == 0 else "FAIL")
    return rc


if __name__ == "__main__":
    sys.exit(main())
