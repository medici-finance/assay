// muhar — mutation-testing harness with a proven-landed guarantee (#34).
//
// Usage:
//
//	muhar -spec mutations.json [-j N] [-shard i/n]
//
// -j is how many mutations to have in flight at once. The default, 1, runs the
// sweep sequentially in place — the historical behaviour. -j 0 sizes the pool to
// the machine's CPU count. With -j > 1 each worker gets its own COPY of the
// spec's root, because a sweep whose workers shared a tree would be editing each
// other's source and every verdict would be a coin flip. The verdicts, their
// order, and the report text are unchanged by -j; only the wall time is.
//
// -shard i/n splits ONE spec across n independent invocations (for CI legs on
// separate machines): shard i runs only the mutations at spec index ≡ i (mod n),
// 0-based, 0 <= i < n. Round-robin by index, so for a fixed n the n shards are
// DISJOINT and their union is EXACTLY the spec — every mutation runs once across
// the set, none twice (TestShardSelectPartitions proves it). The baseline check
// and the positive control still run IN FULL in every shard, so each shard is
// independently trustworthy: its Totals line covers its own mutations and only
// those, and a broken harness reddens every shard, not just one. A shard's
// report is a PARTIAL sweep by design — the caller owns running all n shards
// (the selection is echoed to stderr so a partial report is never mistaken for
// a complete one).
//
// The spec is JSON:
//
//	{
//	  "root": "/abs/path/to/checkout",   // optional; relative file paths resolve here
//	  "test": "go test ./...",           // suite command; NON-ZERO exit == mutation caught
//	  "control": {                        // a mutation the suite MUST catch
//	    "name": "value-conservation invariant",
//	    "file": "config/Example/Core.json",
//	    "old": "n == p",
//	    "new": "n == p + 1"
//	  },
//	  "mutations": [
//	    {"name": "breaker-active check", "file": "...", "old": "...", "new": "..."}
//	  ]
//	}
//
// Exit codes:
//
//	0  run healthy (baseline green, control caught); see stdout for per-guard verdicts.
//	2  HARNESS BROKEN — baseline red or control not caught; the run is discarded and
//	   carries NO trustworthy per-guard verdict. Never read this as "the gate is clean".
//	1  usage / spec / IO error.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

type spec struct {
	Root      string     `json:"root"`
	Test      string     `json:"test"`
	Control   Mutation   `json:"control"`
	Mutations []Mutation `json:"mutations"`
}

// parseShard parses a -shard value of the form "i/n": n total shards, this
// invocation is 0-based shard i. Anything else — including i >= n, a negative
// i, or n < 1 — is a usage error: a malformed shard must refuse loudly, never
// degrade into "run everything" or "run nothing".
func parseShard(s string) (i, n int, err error) {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf(`-shard must be "i/n" (0-based shard i of n), got %q`, s)
	}
	i, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf(`-shard %q: shard index %q is not an integer`, s, parts[0])
	}
	n, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf(`-shard %q: shard count %q is not an integer`, s, parts[1])
	}
	if n < 1 {
		return 0, 0, fmt.Errorf(`-shard %q: shard count must be >= 1`, s)
	}
	if i < 0 || i >= n {
		return 0, 0, fmt.Errorf(`-shard %q: shard index must satisfy 0 <= i < n`, s)
	}
	return i, n, nil
}

// shardSelect returns the mutations whose spec index ≡ i (mod n), in spec
// order. Round-robin rather than contiguous chunks, so the spec's file-level
// grouping (neighbouring mutations tend to hit the same file and cost alike)
// spreads evenly across shards. For a fixed n the n selections PARTITION the
// input — pairwise disjoint, union the whole slice — which is what lets a CI
// caller run the n shards on n machines and still claim every mutation ran
// exactly once (TestShardSelectPartitions holds this).
func shardSelect(ms []Mutation, i, n int) []Mutation {
	var out []Mutation
	for idx, m := range ms {
		if idx%n == i {
			out = append(out, m)
		}
	}
	return out
}

// muhar does NOT call deskkit.Guard(): it is a local diagnostic that makes no
// outward writes (no GitHub, no shared-state mutation — it edits a source file
// and restores it within the same run), so the kill switch and outward-write
// rate limit do not apply. This is the same documented exemption class as
// writeguard; halting local mutation testing serves no safety purpose.
func main() {
	specPath := flag.String("spec", "", "path to the mutation spec JSON")
	jobs := flag.Int("j", 1, "mutations in flight at once (1 = sequential, in place; 0 = one per CPU)")
	shard := flag.String("shard", "", `run only this invocation's share of the spec: "i/n" (0-based shard i of n; the n shards partition the spec's mutations). Baseline + control still run in full per shard.`)
	flag.Parse()

	if *specPath == "" {
		fmt.Fprintln(os.Stderr, "muhar: -spec is required")
		os.Exit(1)
	}

	raw, err := os.ReadFile(*specPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "muhar: read spec: %v\n", err)
		os.Exit(1)
	}
	var s spec
	if err := json.Unmarshal(raw, &s); err != nil {
		fmt.Fprintf(os.Stderr, "muhar: parse spec: %v\n", err)
		os.Exit(1)
	}
	if s.Test == "" {
		fmt.Fprintln(os.Stderr, "muhar: spec.test (the suite command) is required")
		os.Exit(1)
	}
	if s.Control.Name == "" || s.Control.Old == "" {
		fmt.Fprintln(os.Stderr, "muhar: spec.control is required (a mutation the suite MUST catch) — a harness with no failing control is a green light with no bulb (#34)")
		os.Exit(1)
	}

	// Sharding narrows the mutation list — and ONLY the mutation list. The
	// baseline and the positive control are untouched, so every shard still
	// proves the instrument before reporting a single verdict. Echo the
	// selection to stderr: a shard's report is a partial sweep by design, and
	// the echo is what keeps it from being read as a complete one.
	if *shard != "" {
		i, n, err := parseShard(*shard)
		if err != nil {
			fmt.Fprintf(os.Stderr, "muhar: %v\n", err)
			os.Exit(1)
		}
		total := len(s.Mutations)
		s.Mutations = shardSelect(s.Mutations, i, n)
		fmt.Fprintf(os.Stderr, "muhar: shard %d/%d — running %d of %d mutations (indices ≡ %d mod %d); the other shards own the rest\n",
			i, n, len(s.Mutations), total, i, n)
	}

	n := *jobs
	if n == 0 {
		n = runtime.NumCPU()
	}
	if n < 1 {
		fmt.Fprintln(os.Stderr, "muhar: -j must be >= 0")
		os.Exit(1)
	}

	h := &Harness{
		Root:      s.Root,
		Control:   s.Control,
		Mutations: s.Mutations,
		Jobs:      n,
		Test: func(root string) bool {
			// s.Test is an operator-authored suite command from the spec file
			// (e.g. "go test ./... && ./verify.sh") — it legitimately needs a
			// shell for pipes/&&, and the spec author already controls the box
			// this runs on. It is configuration, not end-user input, so `sh -c`
			// is the intended execution path, not an injection surface.
			cmd := exec.Command("sh", "-c", s.Test) //nolint:gosec // operator-supplied suite command, by design

			if root != "" {
				cmd.Dir = root
			}
			cmd.Stdout = os.Stderr // suite chatter goes to stderr; report to stdout
			cmd.Stderr = os.Stderr
			// Non-zero exit == suite failed == mutation caught.
			return cmd.Run() != nil
		},
	}
	if n > 1 {
		// The tree to clone is the one the mutations and the suite command
		// already resolve against: the spec's root, or the process cwd when the
		// spec leaves it empty.
		srcRoot := s.Root
		if srcRoot == "" {
			wd, err := os.Getwd()
			if err != nil {
				fmt.Fprintf(os.Stderr, "muhar: cwd: %v\n", err)
				os.Exit(1)
			}
			srcRoot = wd
		}
		h.Workspace = func(id int) (string, func(), error) {
			dir, err := os.MkdirTemp("", fmt.Sprintf("muhar-%d-", id))
			if err != nil {
				return "", nil, err
			}
			cleanup := func() { _ = os.RemoveAll(dir) }
			if err := copyTree(srcRoot, dir); err != nil {
				cleanup()
				return "", nil, err
			}
			return dir, cleanup, nil
		}
		fmt.Fprintf(os.Stderr, "muhar: %d mutations in flight, one isolated copy of %s per worker\n", n, srcRoot)
	}

	rep := h.Run()
	fmt.Print(rep.Summary())
	if rep.Broken {
		os.Exit(2)
	}
}
