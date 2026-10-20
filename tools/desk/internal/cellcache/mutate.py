#!/usr/bin/env python3
"""Cache mutation gate: plant each mutation in a TEMPORARY COPY of the module,
run the suite, and require a behaviour test to catch it.

Run from tools/desk:  python3 internal/cellcache/mutate.py [--shard I/N]

The source tree this script lives in is never written. The module (and any
sibling module its go.mod replaces with a relative path) is copied into a
temporary directory and every mutation is planted in the copy, so a SIGKILL or
a timeout mid-run cannot leave a mutated source file behind; the copy is
removed on exit.

Every mutation ends in exactly one of three states, never two:

  CAUGHT           the suite went red with a behaviour failure ("--- FAIL:")
  MISSED           the suite stayed green, or went red without a behaviour
                   failure: no test notices the mutation
  COULD-NOT-MUTATE the mutation could not be planted or judged: its `file`
                   escapes the module, its `old` text is absent or equal to
                   `new`, the mutant does not compile ("[build failed]" /
                   "[setup failed]" in the suite output), or the suite timed
                   out. A non-compiling mutant proves nothing about the
                   tests, so it is never counted as caught.

The spec's `control` mutation is a mutation the suite MUST catch. It runs after
the baseline and before any verdict; if it is not caught the harness itself is
broken and no per-mutation verdict is printed.

Exit status:
  0  every mutation CAUGHT
  1  at least one MISSED or COULD-NOT-MUTATE
  2  harness broken (baseline red, control not caught, bad spec) or bad usage
"""
import argparse
import json
import os
from pathlib import Path
import re
import shlex
import shutil
import signal
import subprocess
import sys
import tempfile

CAUGHT, MISSED, COULD_NOT_MUTATE = "CAUGHT", "MISSED", "COULD-NOT-MUTATE"
BUILD_FAILED = re.compile(r"\[(?:build|setup) failed\]")
# Seconds per suite run; a hang is could-not-mutate, never caught. The
# environment override exists so a test can prove the timeout path in seconds;
# main() validates it (a number, > 0, <= MAX_SUITE_TIMEOUT; so never nan or
# inf) before any suite runs; a bad value is HARNESS BROKEN, exit 2.
DEFAULT_SUITE_TIMEOUT = 300.0
MAX_SUITE_TIMEOUT = 3600.0
SUITE_TIMEOUT = DEFAULT_SUITE_TIMEOUT
TIMEOUT_ENV = "CELLCACHE_MUTATE_SUITE_TIMEOUT"
ACTIVE = []  # the suite process currently running, if any, for the signal handler
SKIP_DIRS = (".git", "dist", "node_modules", "__pycache__")


class HarnessBroken(Exception):
    pass


def replace_targets(module):
    """Relative `replace ... => ./path` targets of module/go.mod."""
    gomod = module / "go.mod"
    if not gomod.is_file():
        return []
    found = []
    for line in gomod.read_text().splitlines():
        m = re.search(r"=>\s*(\.{1,2}(?:/\S*)?)\s*$", line.split("//")[0])
        if m:
            found.append((module / m.group(1)).resolve())
    return found


def copy_module(module, dest):
    """Copy module (plus relative replace targets, at their relative position)
    under dest. Returns the copy of module."""
    roots = [module]
    for target in replace_targets(module):
        if not target.is_dir():
            raise HarnessBroken("go.mod replaces a directory that does not exist: %s" % target)
        if not any(r == target or r in target.parents for r in roots):
            roots.append(target)
    base = Path(os.path.commonpath([str(r) for r in roots]))
    ignore = shutil.ignore_patterns(*SKIP_DIRS)
    for root in roots:
        shutil.copytree(root, dest / root.relative_to(base), symlinks=True, ignore=ignore, dirs_exist_ok=True)
    return dest / module.relative_to(base)


def run_suite(command, cwd, env):
    """Returns (exit code or None on timeout, combined output)."""
    proc = subprocess.Popen(command, cwd=cwd, env=env, text=True,
                            stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                            start_new_session=True)
    ACTIVE.append(proc)
    try:
        out, _ = proc.communicate(timeout=SUITE_TIMEOUT)
    except subprocess.TimeoutExpired:
        kill_group(proc)
        out, _ = proc.communicate()
        return None, out
    except BaseException:
        kill_group(proc)  # whatever went wrong, never leave the suite running
        raise
    finally:
        ACTIVE.remove(proc)
    return proc.returncode, out


def suite_timeout(environ):
    """The per-suite timeout from the environment, or raise HarnessBroken."""
    raw = environ.get(TIMEOUT_ENV)
    if raw is None:
        return DEFAULT_SUITE_TIMEOUT
    try:
        value = float(raw)
    except ValueError:
        value = float("nan")
    if not 0 < value <= MAX_SUITE_TIMEOUT:  # false for nan and inf as well
        raise HarnessBroken("%s=%r must be a number greater than 0 and at most %g seconds"
                            % (TIMEOUT_ENV, raw, MAX_SUITE_TIMEOUT))
    return value


def kill_group(proc):
    """SIGKILL the suite's whole process group (it runs in its own session, so
    a terminal interrupt does not reach it) and reap the leader."""
    try:
        os.killpg(proc.pid, signal.SIGKILL)
    except ProcessLookupError:
        pass
    try:
        proc.wait(timeout=10)
    except subprocess.TimeoutExpired:
        pass


def judge(code, out):
    """(state, reason) for one suite run on a planted mutant."""
    if code is None:
        return COULD_NOT_MUTATE, "suite timed out after %gs" % SUITE_TIMEOUT
    if BUILD_FAILED.search(out):
        return COULD_NOT_MUTATE, "mutant does not compile (suite reports a build/setup failure)"
    if code != 0 and "--- FAIL:" in out:
        return CAUGHT, ""
    return MISSED, ""


def plant_and_run(work, mutation, command, env):
    """Plant one mutation in the copy at `work`, run the suite, restore the
    file. Returns (state, reason, output)."""
    rel = mutation.get("file", "")
    old, new = mutation.get("old", ""), mutation.get("new", "")
    if not rel or os.path.isabs(rel) or "\0" in rel:
        return COULD_NOT_MUTATE, "file %r is not a relative path inside the module" % rel, ""
    root = os.path.realpath(work)
    path = os.path.realpath(os.path.join(root, rel))
    if os.path.commonpath([root, path]) != root or path == root:
        return COULD_NOT_MUTATE, "file %r escapes the module" % rel, ""
    if not os.path.isfile(path):
        return COULD_NOT_MUTATE, "file %r is not a file in the module" % rel, ""
    if old == "":
        return COULD_NOT_MUTATE, "`old` is empty: nothing to match", ""
    if old == new:
        return COULD_NOT_MUTATE, "`old` equals `new`: the edit is a no-op", ""
    original = Path(path).read_text()
    if old not in original:
        return COULD_NOT_MUTATE, "`old` text not found in %s" % rel, ""
    mutated = original.replace(old, new, 1)
    try:
        Path(path).write_text(mutated)
        if Path(path).read_text() != mutated:
            return COULD_NOT_MUTATE, "the edit did not read back from disk", ""
        code, out = run_suite(command, work, env)
    finally:
        Path(path).write_text(original)
    state, reason = judge(code, out)
    return state, reason, out


def main(argv):
    here = Path(__file__).resolve()
    parser = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    parser.add_argument("--spec", default=str(here.with_name("mutations.json")))
    parser.add_argument("--module", default=str(here.parents[2]),
                        help="module directory the spec's test command and files resolve against")
    parser.add_argument("--shard", default="",
                        help="I/N: run only mutations at spec index = I (mod N); baseline and control run in every shard")
    args = parser.parse_args(argv)

    global SUITE_TIMEOUT
    try:
        SUITE_TIMEOUT = suite_timeout(os.environ)
    except HarnessBroken as err:
        print("HARNESS BROKEN: " + str(err))
        return 2

    try:
        spec = json.loads(Path(args.spec).read_text())
        command = shlex.split(spec["test"])
        control = spec["control"]
        mutations = list(spec["mutations"])
    except (OSError, ValueError, KeyError) as err:
        print("HARNESS BROKEN: unreadable spec (%s: %s); a spec needs test, control and mutations" % (type(err).__name__, err))
        return 2
    if args.shard:
        try:
            i, n = (int(x) for x in args.shard.split("/"))
            assert n >= 1 and 0 <= i < n
        except (ValueError, AssertionError):
            print("HARNESS BROKEN: --shard must be I/N with 0 <= I < N")
            return 2
        mutations = [m for idx, m in enumerate(mutations) if idx % n == i]
        print("shard %d/%d: %d mutations" % (i, n, len(mutations)), flush=True)

    env = dict(os.environ, GOPROXY="off", GOTOOLCHAIN="local", KUBECONFIG="/dev/null")
    env.pop("BASH_ENV", None)
    env.pop("ENV", None)

    def terminate(signum, frame):
        for proc in list(ACTIVE):  # no orphaned suite against a deleted copy
            kill_group(proc)
        sys.exit(128 + signum)  # unwinds, so the temporary copy is removed
    signal.signal(signal.SIGTERM, terminate)
    signal.signal(signal.SIGINT, terminate)

    module = Path(args.module).resolve()
    with tempfile.TemporaryDirectory(prefix="cellcache-mutate-") as tmp:
        try:
            work = copy_module(module, Path(tmp))
        except (HarnessBroken, OSError) as err:
            print("HARNESS BROKEN: cannot copy the module (%s)" % err)
            return 2

        code, out = run_suite(command, work, env)
        if code != 0 or BUILD_FAILED.search(out or ""):
            print(out)
            print("HARNESS BROKEN: baseline suite is not green on pristine source")
            return 2

        state, reason, out = plant_and_run(work, control, command, env)
        if state != CAUGHT:
            print(out)
            print("HARNESS BROKEN: control %r is %s%s; a mutation the suite must catch did not "
                  "go red, so no per-mutation verdict can be trusted"
                  % (control.get("name"), state, (" (" + reason + ")") if reason else ""))
            return 2
        print("control caught: " + str(control.get("name")), flush=True)

        tally = {CAUGHT: 0, MISSED: 0, COULD_NOT_MUTATE: 0}
        for mutation in mutations:
            state, reason, out = plant_and_run(work, mutation, command, env)
            tally[state] += 1
            print("%s %s%s" % (state, mutation.get("name"), (": " + reason) if reason else ""), flush=True)
            for line in out.splitlines():
                if "--- FAIL:" in line:
                    print(line, flush=True)
            if state != CAUGHT and out:
                print(out, flush=True)

    print("Totals: %d caught, %d NOT CAUGHT, %d could-not-mutate."
          % (tally[CAUGHT], tally[MISSED], tally[COULD_NOT_MUTATE]))
    if tally[MISSED] or tally[COULD_NOT_MUTATE]:
        return 1
    print("All cache mutants caught; the source tree was never written.")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
