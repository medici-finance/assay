#!/usr/bin/env python3
"""Changelog-fragment aggregation — the release-time engine. A transparent adapter.

It runs ``deskchangelog <argv...>`` with every argument passed through unchanged
and exits with its status. The engine — the fragment parser, the bucket rules,
the cutover fold, the credits map, the dated-section roll and the git-only
pull-request resolution — lives in tools/desk/cmd/deskchangelog
(desktools-v2/52); ``deskchangelog <verb> --help`` documents each verb:

unreleased-bullets <changelog>
    The normalized bullet entries under ``## Unreleased``, one per line.
highlights <fragment-dir> <changelog> [--credits <map-file>]
    The aggregated ``### <bucket>`` blocks; exit 2 when there is nothing to
    aggregate.
roll <fragment-dir> <changelog> <tag> <date> [--credits <map-file>]
    Rewrite <changelog> in place with a dated section; exit 2 (file untouched)
    when there is nothing to aggregate or no ``## Unreleased`` heading.
credits <fragment-dir>
    ``<fragment><TAB><pull-request-number|unresolved>`` per fragment, from git
    alone; always exit 0.

Exit codes: 0 ok · 1 an unreadable, non-UTF-8 or unwritable input · 2 usage or
a refusal · 127 when no deskchangelog can be found or built.

Which deskchangelog runs: the one on PATH (desk-tools release archive), else one
built from this checkout's tools/desk with ``go build`` into a temporary
directory that is removed afterwards.
"""

import os
import shutil
import subprocess
import sys
import tempfile


def main(argv):
    found = shutil.which("deskchangelog")
    if found:
        return subprocess.run([found, *argv[1:]]).returncode
    desk_dir = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "desk")
    go = shutil.which("go")
    if not go or not os.path.isdir(os.path.join(desk_dir, "cmd", "deskchangelog")):
        sys.stderr.write(
            "error: deskchangelog is not on PATH, and it cannot be built here "
            "(needs go and %s/cmd/deskchangelog).\n"
            "       Install the desk-tools release, or run from an assay checkout "
            "with Go installed.\n" % desk_dir)
        return 127
    with tempfile.TemporaryDirectory(prefix="deskchangelog.") as build_dir:
        exe = os.path.join(build_dir, "deskchangelog")
        if os.name == "nt":
            exe += ".exe"
        built = subprocess.run([go, "-C", desk_dir, "build", "-buildvcs=false",
                                "-o", exe, "./cmd/deskchangelog"])
        if built.returncode != 0:
            sys.stderr.write("error: building deskchangelog from %s failed; nothing was run\n" % desk_dir)
            return 127
        return subprocess.run([exe, *argv[1:]]).returncode


if __name__ == "__main__":
    sys.exit(main(sys.argv))
