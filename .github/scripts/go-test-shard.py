#!/usr/bin/env python3
"""go-test-shard.py — split one Go module's packages across N CI shards.

Used by ci.yml's build-test-shard matrix: each desk-K leg pipes the output of
`go list ./...` (run in tools/desk) into this script and runs `go test` on
exactly the packages it prints.

THE PROPERTY THAT MATTERS: every package is tested exactly once. A partition
that drops a package turns a red test into a silent pass; one that repeats a
package wastes a runner. So on EVERY call the script computes the full
partition — all N shards, not only the one asked for — and refuses (exit 1)
unless:

  * the shards are pairwise disjoint (no package tested twice), and
  * their union equals the input set, package for package (none dropped), and
  * no shard is empty (an empty argument list would make `go test` fall back to
    the current directory's package, which another shard already tests).

The input is the `go list ./...` of the commit under test, so a package added or
removed by the PR is partitioned like any other; the union check is against
that list, never against a list kept in this file.

The split is deterministic (longest-processing-time first, ties broken by
import path), so every leg of a run computes the same partition from the same
input. WEIGHTS below are measured per-package `go test` seconds from recent
build-test runs; they only balance the legs. A stale or missing weight changes
which leg runs a package, never whether it runs: an unlisted package weighs
DEFAULT_WEIGHT.

Usage:
  go list ./... | go-test-shard.py --shards N --index K    # print shard K (1..N)
  go-test-shard.py --self-test                             # negative controls

Exit: 0 printed a verified shard (or self-test passed); 1 the partition failed
its check; 2 bad arguments or input.
"""

import argparse
import sys

# Measured `go test` seconds per package (import-path suffix under the module),
# for packages above ~3s. Everything else is DEFAULT_WEIGHT.
WEIGHTS = {
    "internal/deskkit": 184.0,
    "cmd/muhar": 177.4,
    "internal/regression": 129.3,
    "cmd/deskwt": 76.0,
    "cmd/deskdispatch": 67.3,
    "cmd/cellctl": 58.3,
    "internal/reviewtest": 50.6,
    "cmd/deskevidence": 40.0,
    "cmd/deskboard": 35.3,
    "cmd/issueboard": 33.9,
    "internal/clicontract": 33.2,
    "cmd/writeguard": 17.3,
    "internal/cellcache": 17.3,
    "cmd/deskpr": 15.4,
    "cmd/deskgit": 13.7,
    "cmd/deskmerge": 13.1,
    "internal/gitmaintenance": 10.9,
    "cmd/deskmonitor": 9.3,
    "cmd/verifyloop": 7.1,
    "internal/cellprocess": 6.4,
    "internal/avatar": 6.2,
    "cmd/deskcomms": 6.0,
    "internal/cellstop": 5.7,
    "internal/gremlintriage": 5.2,
    "cmd/deskpushguard": 4.5,
    "cmd/deskread": 4.0,
    "cmd/desktoken": 3.1,
}
DEFAULT_WEIGHT = 1.0


def weight(pkg):
    for suffix, seconds in WEIGHTS.items():
        if pkg == suffix or pkg.endswith("/" + suffix):
            return seconds
    return DEFAULT_WEIGHT


def partition(packages, shards):
    """Longest-processing-time-first split of packages into `shards` lists."""
    loads = [0.0] * shards
    out = [[] for _ in range(shards)]
    for pkg in sorted(packages, key=lambda p: (-weight(p), p)):
        k = min(range(shards), key=lambda i: (loads[i], i))
        out[k].append(pkg)
        loads[k] += weight(pkg)
    return [sorted(s) for s in out], loads


def check(packages, split):
    """Return the problems with `split` as a partition of `packages` (empty = sound)."""
    problems = []
    want = set(packages)
    seen = {}
    for k, shard in enumerate(split, 1):
        if not shard:
            problems.append(f"shard {k} is empty")
        for pkg in shard:
            if pkg in seen:
                problems.append(f"{pkg} is in shard {seen[pkg]} and shard {k}")
            seen.setdefault(pkg, k)
    union = set(seen)
    for pkg in sorted(want - union):
        problems.append(f"{pkg} is in no shard (dropped)")
    for pkg in sorted(union - want):
        problems.append(f"{pkg} is in a shard but not in the input")
    return problems


def read_packages(stream):
    pkgs = [ln.strip() for ln in stream.read().splitlines() if ln.strip()]
    if not pkgs:
        raise ValueError("no packages on stdin")
    dups = sorted({p for p in pkgs if pkgs.count(p) > 1})
    if dups:
        raise ValueError("duplicate packages on stdin: " + ", ".join(dups))
    return pkgs


def self_test():
    pkgs = [f"example.test/m/p{i:02d}" for i in range(40)] + ["example.test/m/internal/deskkit"]
    failures = []
    for n in (1, 2, 3, 5):
        split, _ = partition(pkgs, n)
        if check(pkgs, split):
            failures.append(f"sound {n}-way split reported unsound: {check(pkgs, split)}")
        if split != partition(list(reversed(pkgs)), n)[0]:
            failures.append(f"{n}-way split depends on input order")
    split, _ = partition(pkgs, 3)
    # Negative controls: each corruption MUST be reported.
    dropped = [s[:] for s in split]
    dropped[1].pop()
    if not any("dropped" in p for p in check(pkgs, dropped)):
        failures.append("a dropped package was not reported")
    doubled = [s[:] for s in split]
    doubled[2].append(doubled[0][0])
    if not any("and shard" in p for p in check(pkgs, doubled)):
        failures.append("a package in two shards was not reported")
    extra = [s[:] for s in split]
    extra[0].append("example.test/m/not-listed")
    if not any("not in the input" in p for p in check(pkgs, extra)):
        failures.append("a package outside the input was not reported")
    if not any("empty" in p for p in check(pkgs[:2], partition(pkgs[:2], 3)[0])):
        failures.append("an empty shard was not reported")
    for f in failures:
        print("self-test FAIL: " + f, file=sys.stderr)
    if not failures:
        print("self-test: ok (sound splits verified; drop, double, foreign and empty shards all refused)")
    return 1 if failures else 0


def main(argv):
    ap = argparse.ArgumentParser(description="Split go list ./... output across CI shards.")
    ap.add_argument("--shards", type=int)
    ap.add_argument("--index", type=int, help="1-based shard to print")
    ap.add_argument("--self-test", action="store_true")
    args = ap.parse_args(argv)
    if args.self_test:
        return self_test()
    if args.shards is None or args.index is None or args.shards < 1 or not 1 <= args.index <= args.shards:
        print("go-test-shard: need --shards N >= 1 and --index K in 1..N", file=sys.stderr)
        return 2
    try:
        pkgs = read_packages(sys.stdin)
    except ValueError as err:
        print(f"go-test-shard: {err}", file=sys.stderr)
        return 2
    split, loads = partition(pkgs, args.shards)
    problems = check(pkgs, split)
    if problems:
        for p in problems:
            print("go-test-shard: partition unsound: " + p, file=sys.stderr)
        return 1
    print(f"go-test-shard: {len(pkgs)} packages over {args.shards} shards; "
          f"union == input, pairwise disjoint, none empty", file=sys.stderr)
    for k, (shard, load) in enumerate(zip(split, loads), 1):
        mark = " <- this leg" if k == args.index else ""
        print(f"go-test-shard:   shard {k}: {len(shard)} packages, ~{load:.0f}s{mark}", file=sys.stderr)
    sys.stdout.write("".join(p + "\n" for p in split[args.index - 1]))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
