// muhar — mutation-testing harness with a proven-landed guarantee (#34).
//
// Usage:
//
//	muhar -spec mutations.json [-j N] [-shard i/n] [-select]
//
// -j is how many mutations to have in flight at once. The default, 1, runs the
// sweep sequentially in place — the historical behaviour. -j 0 sizes the pool to
// the machine's CPU count, CAPPED at maxAutoJobs. With -j > 1 each worker gets
// its own full COPY of the spec's root in memory, because a sweep whose workers
// shared a tree would be editing each other's source and every verdict would be
// a coin flip — but that means peak RSS scales with mutations-in-flight, not the
// core count, so an uncapped "one per CPU" on a many-core CI box drove peak RSS
// past the runner's memory ceiling and OOMKilled the sweep; the cap bounds peak
// RSS by construction. The verdicts, their order, and the report text are
// unchanged by -j — EVERY mutation in the spec still runs, only fewer at a time;
// only the wall time changes.
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
// -select (opt-in, #2636) runs each mutation against only the tests that can
// catch it: after the baseline, one coverage profile per top-level test records
// which tests execute which functions, and each mutant runs the tests that
// executed the function it lands in, plus the spec's select.always list. It
// only ever REMOVES tests, and each distinct narrowed set must pass on pristine
// source first (a red one runs the full command); the control runs under
// selection too. selection.go states the safety property, the one case it does
// not close, and what falls back to the full command. Baseline, control, exit
// codes, the report and -shard/-j are unchanged. It needs the spec's `test` to
// be a plain `go test [flags] <packages>` command and refuses anything else.
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
//	  "select": {"always": ["TestReadsSource"]}, // optional; -select only
//	  "mutations": [
//	    {"name": "breaker-active check", "file": "...", "old": "...", "new": "..."}
//	  ],
//	  "schemata": true                    // optional; build eligible mutants once per worker (schemata.go)
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
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

type spec struct {
	Root      string     `json:"root"`
	Test      string     `json:"test"`
	Control   Mutation   `json:"control"`
	Mutations []Mutation `json:"mutations"`
	// Select configures -select; it has no effect without the flag.
	Select *selectSpec `json:"select,omitempty"`
	// Schemata opts this spec into mutant schemata (#2635). Off unless set:
	// schemata assumes the suite never cross-checks the running code against
	// its source on disk or against a child that drops the environment (see
	// schemata.go), so a spec turns it on only after a parity run on its suite.
	Schemata bool `json:"schemata,omitempty"`
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

// maxAutoJobs bounds the pool that `-j 0` ("one per CPU") sizes itself to. Each
// in-flight muhar worker holds its OWN full copy of the module tree in memory
// (see Harness.Workspace / copyTree), so peak RSS scales with mutations-in-flight
// rather than with the core count. On a many-core CI runner an uncapped
// runtime.NumCPU() put enough tree-copies in memory at once to exceed the pool's
// memory ceiling and OOMKill the sweep — doubling the memory limit only doubled
// survival, the signature of unbounded growth, not a spike. Capping the
// auto-sized pool bounds peak RSS by construction. The cap governs ONLY how many
// mutations run at once; it never changes WHICH mutations run — Run() still
// sweeps every mutation in the spec (harness.go), just <= maxAutoJobs at a time.
const maxAutoJobs = 2

// resolveJobs maps the -j flag value to the number of mutations to have in
// flight, given the machine's CPU count. -j 0 means "size to the machine", which
// is CAPPED at maxAutoJobs so an auto-sized sweep on a many-core box cannot grow
// peak RSS without bound; on a box with fewer CPUs than the cap the cap is a
// ceiling, not a floor, so it never inflates a small box. Any explicit -j N
// (including the local default of 1) passes through verbatim — a caller that
// asks for N in flight is making its own memory/time trade-off, and a negative N
// falls straight through to main's `-j must be >= 0` rejection.
func resolveJobs(requested, numCPU int) int {
	if requested != 0 {
		return requested
	}
	if numCPU < maxAutoJobs {
		return numCPU
	}
	return maxAutoJobs
}

// envWithGOCACHE returns env with GOCACHE pinned to dir, overriding any
// inherited value: the quarantine only holds if the suite's build artifacts
// land in the throwaway per-run cache even when the environment already names
// a GOCACHE. GOMODCACHE is never touched — module downloads are immutable and
// shared-safe, and re-downloading them would cost network per run.
func envWithGOCACHE(env []string, dir string) []string {
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if !strings.HasPrefix(e, "GOCACHE=") {
			out = append(out, e)
		}
	}
	return append(out, "GOCACHE="+dir)
}

// envWithout returns env minus every entry for the named variables.
func envWithout(env []string, names ...string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		keep := true
		for _, n := range names {
			if strings.HasPrefix(e, n+"=") {
				keep = false
			}
		}
		if keep {
			out = append(out, e)
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
	jobs := flag.Int("j", 1, "mutations in flight at once (1 = sequential, in place; 0 = one per CPU, capped at 2 to bound peak memory)")
	shard := flag.String("shard", "", `run only this invocation's share of the spec: "i/n" (0-based shard i of n; the n shards partition the spec's mutations). Baseline + control still run in full per shard.`)
	useSelect := flag.Bool("select", false, "run each mutation against only the tests that execute the mutated function (plus the spec's select.always list); opt-in")
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

	n := resolveJobs(*jobs, runtime.NumCPU())
	if n < 1 {
		fmt.Fprintln(os.Stderr, "muhar: -j must be >= 0")
		os.Exit(1)
	}

	// Quarantine the Go build cache for this run. Every mutant is a unique
	// source tree, so each suite run injects hundreds of never-reused entries
	// into whatever GOCACHE the suite command inherits; in a long-lived cell
	// that cache grows without bound (Go's own 5-day trim horizon is a
	// compile-time constant, not a knob). One throwaway cache for the WHOLE
	// run — mutants of one package share most build artifacts, so per-run
	// keeps the intra-run speedup — removed when the run ends, whatever the
	// exit path. os.Exit skips defers, so the Broken exit below removes it
	// explicitly; the defer covers a normal return.
	gocache, err := os.MkdirTemp("", "assay-mutant-gocache-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "muhar: create per-run GOCACHE: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = os.RemoveAll(gocache) }()
	// An interrupt mid-run must not strand the cache either.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		_ = os.RemoveAll(gocache)
		selectTemps.removeAll() // -select's coverage binaries, if a pass is running
		os.Exit(130)
	}()

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
			cmd.Env = envWithGOCACHE(os.Environ(), gocache)
			cmd.Stdout = os.Stderr // suite chatter goes to stderr; report to stdout
			cmd.Stderr = os.Stderr
			// Non-zero exit == suite failed == mutation caught.
			return cmd.Run() != nil
		},
	}
	if *useSelect {
		cmd, err := parseGoTest(s.Test)
		if err != nil {
			_ = os.RemoveAll(gocache) // os.Exit skips the deferred cleanup
			fmt.Fprintf(os.Stderr, "muhar: %v\n", err)
			os.Exit(1)
		}
		env := envWithGOCACHE(os.Environ(), gocache)
		h.Selector = newCoverSelector(s, cmd, n, env, os.Stderr, func(root string, args []string) bool {
			c := exec.Command("go", args...)
			if root != "" {
				c.Dir = root
			}
			c.Env = env
			c.Stdout = os.Stderr
			c.Stderr = os.Stderr
			return c.Run() != nil // non-zero exit == narrowed suite failed == caught
		})
	}
	// Mutant schemata (#2635), for a spec that opts in: eligible mutants share
	// one build per worker and are picked per run by an environment selector,
	// instead of each one recompiling its package. Only a suite run that goes
	// RED is taken from it (see schemata.go); every other verdict still comes
	// from the classic run.
	if s.Schemata && !schemataUsable(s.Test, os.Getenv("GOFLAGS"), os.TempDir()) {
		fmt.Fprintln(os.Stderr, "muhar: schemata: the spec asks for it, but needs a go test -count=1 suite, no -overlay of its own and a TMPDIR without spaces; every mutation rebuilds the suite")
	}
	if s.Schemata && schemataUsable(s.Test, os.Getenv("GOFLAGS"), os.TempDir()) {
		h.Log = os.Stderr
		h.SchemaTest = func(root, overlay, selector string) bool {
			cmd := exec.Command("sh", "-c", s.Test) //nolint:gosec // operator-supplied suite command, by design
			if root != "" {
				cmd.Dir = root
			}
			env := envWithGOCACHE(os.Environ(), gocache)
			cmd.Env = append(envWithout(env, "GOFLAGS", schemaEnv),
				"GOFLAGS="+schemaGoflags(os.Getenv("GOFLAGS"), overlay),
				schemaEnv+"="+selector)
			cmd.Stdout = os.Stderr
			cmd.Stderr = os.Stderr
			return cmd.Run() != nil
		}
		h.SchemaBuild = func(root, overlay string, dirs []string) (string, bool) {
			// -e: report every compile error, so one probe finds every mutant
			// that does not compile. One package per build, its output written
			// beside the overlay, never into the tree.
			env := envWithGOCACHE(os.Environ(), gocache)
			env = append(envWithout(env, "GOFLAGS"), "GOFLAGS="+schemaGoflags(os.Getenv("GOFLAGS"), overlay))
			var all strings.Builder
			clean := true
			for i, d := range dirs {
				out := filepath.Join(filepath.Dir(overlay), fmt.Sprintf("probe-%d.out", i))
				cmd := exec.Command("go", "build", "-gcflags=-e", "-o", out, d)
				if root != "" {
					cmd.Dir = root
				}
				cmd.Env = env
				b, err := cmd.CombinedOutput()
				all.Write(b)
				if err != nil {
					clean = false
				}
			}
			return all.String(), clean
		}
	}
	if n > 1 {
		// The tree to clone is the one the mutations and the suite command
		// already resolve against: the spec's root, or the process cwd when the
		// spec leaves it empty.
		srcRoot := s.Root
		if srcRoot == "" {
			wd, err := os.Getwd()
			if err != nil {
				_ = os.RemoveAll(gocache) // os.Exit skips the deferred cleanup
				fmt.Fprintf(os.Stderr, "muhar: cwd: %v\n", err)
				os.Exit(1)
			}
			srcRoot = wd
		}
		// Each copy keeps its go.mod's relative replace targets resolvable
		// (#2550): see copyModuleTree.
		h.Workspace = newWorkspace(srcRoot)
		fmt.Fprintf(os.Stderr, "muhar: %d mutations in flight, one isolated copy of %s per worker\n", n, srcRoot)
	}

	rep := h.Run()
	fmt.Print(rep.Summary())
	_ = os.RemoveAll(gocache) // os.Exit skips the deferred cleanup
	if rep.Broken {
		os.Exit(2)
	}
}
