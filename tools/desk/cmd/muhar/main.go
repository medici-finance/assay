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
// selection too. selection.go states the safety property, the residual it does
// not close (a narrowed run changes which tests precede each selected test, so
// -select can report CAUGHT for a mutant the full command survives; re-run a
// parity check against the full sweep whenever the suite or spec changes), and
// what falls back to the full command. Baseline, control, exit
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
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/tools/desk/internal/cli"
)

type spec struct {
	Root      string      `json:"root"`
	Test      string      `json:"test"`
	Build     string      `json:"build,omitempty"`
	Timeout   TimeoutSpec `json:"timeout,omitempty"`
	Control   Mutation    `json:"control"`
	Mutations []Mutation  `json:"mutations"`
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
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// helpLong is the root help's prose; the usage line and the flags are generated from the
// command tree (desktools-v2/44).
const helpLong = `muhar — mutation-testing harness with a proven-landed guarantee.

It proves the suite green on pristine source, proves the spec's positive control is
caught, then plants each mutation and reports whether the suite caught it.

Exit codes: 0 run healthy (see the per-mutation verdicts on stdout) · 2 HARNESS BROKEN
(baseline red or control not caught; also a command line muhar cannot parse) · 1 usage,
spec or IO error.`

// options is the parsed command line the sweep runs with.
type options struct {
	spec      string
	jobs      int
	shard     string
	useSelect bool
}

// sweepFn is the typed handler the command tree calls; the contract tests replace it to
// observe the values that reach it.
var sweepFn = sweep

// onResolve, when set, sees every invocation's resolved settings (the contract tests' window on
// which source won).
var onResolve func(*cli.Values)

// exitErr is a handler's exit code carried as an error, so cli.Run keeps it.
type exitErr int

func (e exitErr) Error() string { return "exit " + strconv.Itoa(int(e)) }
func (e exitErr) ExitCode() int { return int(e) }

// bindings are every setting muhar declares: flags and defaults only. muhar has no
// configuration file and reads no environment variable for them.
func bindings() []cli.Binding {
	return []cli.Binding{
		{Key: "spec", Kind: cli.String, Flag: "spec",
			Usage: "`path` to the mutation spec JSON (required)"},
		{Key: "j", Kind: cli.Int, Flag: "j", Short: "j", Default: 1,
			Usage: "mutations in flight at once (1 = sequential, in place; 0 = one per CPU, capped at 2 to bound peak memory)"},
		{Key: "shard", Kind: cli.String, Flag: "shard",
			Usage: `run only this invocation's share of the spec: "i/n" (0-based shard i of n; the n shards partition the spec's mutations). Baseline + control still run in full per shard.`},
		{Key: "select", Kind: cli.Bool, Flag: "select", Default: false,
			Usage: "run each mutation against only the tests that execute the mutated function (plus the spec's select.always list); opt-in"},
	}
}

// run parses args against a fresh command tree and runs the sweep. Callers write the Go-flag
// spellings (-spec F -j 0 -shard i/n), so GoFlagCompat keeps them; a command line muhar
// cannot parse exits 2, as the flag package's ExitOnError did.
func run(args []string, stdout, stderr io.Writer) int {
	build := func() *cobra.Command {
		root := cli.NewRoot("muhar", "mutation-testing harness with a proven-landed guarantee")
		root.Long = helpLong
		set := cli.Declare(root, bindings()...)
		// Words after the flags were ignored by the flag package; they still are.
		root.Args = cobra.ArbitraryArgs
		root.RunE = func(*cobra.Command, []string) error {
			v, err := set.Resolve(cli.Inputs{})
			if err != nil {
				fmt.Fprintln(stderr, "muhar:", err)
				return exitErr(1)
			}
			if onResolve != nil {
				onResolve(v)
			}
			o := options{spec: v.String("spec"), jobs: v.Int("j"), shard: v.String("shard"), useSelect: v.Bool("select")}
			if code := sweepFn(o, stdout, stderr); code != 0 {
				return exitErr(code)
			}
			return nil
		}
		return root
	}
	return cli.Run(build, args, cli.Options{
		IO:           cli.IO{Out: stdout, Err: stderr},
		GoFlagCompat: true,
	})
}

// sweep is the run itself: read and check the spec, then baseline, control and mutations.
func sweep(o options, stdout, stderr io.Writer) int {
	if o.spec == "" {
		fmt.Fprintln(stderr, "muhar: -spec is required")
		return 1
	}
	s, code := loadSpec(o.spec, stderr)
	if code != 0 {
		return code
	}
	// Sharding narrows the mutation list — and ONLY the mutation list. The
	// baseline and the positive control are untouched, so every shard still
	// proves the instrument before reporting a single verdict. Echo the
	// selection to stderr: a shard's report is a partial sweep by design, and
	// the echo is what keeps it from being read as a complete one.
	if o.shard != "" {
		i, n, err := parseShard(o.shard)
		if err != nil {
			fmt.Fprintf(stderr, "muhar: %v\n", err)
			return 1
		}
		total := len(s.Mutations)
		s.Mutations = shardSelect(s.Mutations, i, n)
		fmt.Fprintf(stderr, "muhar: shard %d/%d — running %d of %d mutations (indices ≡ %d mod %d); the other shards own the rest\n",
			i, n, len(s.Mutations), total, i, n)
	}

	n := resolveJobs(o.jobs, runtime.NumCPU())
	if n < 1 {
		fmt.Fprintln(stderr, "muhar: -j must be >= 0")
		return 1
	}

	// Quarantine the Go build cache for this run. Every mutant is a unique
	// source tree, so each suite run injects hundreds of never-reused entries
	// into whatever GOCACHE the suite command inherits; in a long-lived cell
	// that cache grows without bound (Go's own 5-day trim horizon is a
	// compile-time constant, not a knob). One throwaway cache for the WHOLE
	// run — mutants of one package share most build artifacts, so per-run
	// keeps the intra-run speedup — removed when the run ends, whatever the
	// exit path: the defer covers every return, and the interrupt handler below,
	// which ends the process with os.Exit (skipping defers), removes it itself.
	gocache, err := os.MkdirTemp("", "assay-mutant-gocache-*")
	if err != nil {
		fmt.Fprintf(stderr, "muhar: create per-run GOCACHE: %v\n", err)
		return 1
	}
	defer func() { _ = os.RemoveAll(gocache) }()
	// An interrupt mid-run must not strand the cache either. Every suite and
	// build runs in its own process group, which a terminal Ctrl-C does not
	// reach, so kill those groups first: an interrupted run leaves no suite
	// running behind it.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)
	go func() {
		<-sig
		liveGroups.killAll()
		_ = os.RemoveAll(gocache)
		selectTemps.removeAll() // -select's coverage binaries, if a pass is running
		os.Exit(130)            // the interrupt ends the process; nothing returns to the caller
	}()

	runEnv := envWithGOCACHE(os.Environ(), gocache)
	h := &Harness{
		Root:      s.Root,
		Control:   s.Control,
		Mutations: s.Mutations,
		Jobs:      n,
		Timeout:   s.Timeout,
		RunTest: func(root string, timeout time.Duration) CommandResult {
			return runShell(root, s.Test, runEnv, timeout)
		},
		Build: func(root string, m Mutation, timeout time.Duration) CommandResult {
			return buildMutation(root, m, s.Build, s.Test, runEnv, timeout)
		},
	}
	if o.useSelect {
		cmd, err := parseGoTest(s.Test)
		if err != nil {
			fmt.Fprintf(stderr, "muhar: %v\n", err)
			return 1
		}
		env := envWithGOCACHE(os.Environ(), gocache)
		h.Selector = newCoverSelector(s, cmd, n, env, stderr, func(root string, args []string) bool {
			c := exec.Command("go", args...)
			if root != "" {
				c.Dir = root
			}
			c.Env = env
			c.Stdout = stderr
			c.Stderr = stderr
			return c.Run() != nil // non-zero exit during the pristine selection proof
		}, func(root string, args []string, timeout time.Duration) CommandResult {
			return runGoArgs(root, withTimeout(args, timeout), env, timeout)
		})
	}
	// Mutant schemata (#2635), for a spec that opts in: eligible mutants share
	// one build per worker and are picked per run by an environment selector,
	// instead of each one recompiling its package. Only a suite run that goes
	// RED is taken from it (see schemata.go); every other verdict still comes
	// from the classic run.
	if s.Schemata && !schemataUsable(s.Test, os.Getenv("GOFLAGS"), os.TempDir()) {
		fmt.Fprintln(stderr, "muhar: schemata: the spec asks for it, but needs a go test -count=1 suite, no -overlay of its own and a TMPDIR without spaces; every mutation rebuilds the suite")
	}
	if s.Schemata && schemataUsable(s.Test, os.Getenv("GOFLAGS"), os.TempDir()) {
		h.Log = stderr
		h.SchemaTest = func(root, overlay, selector string) bool {
			out := h.SchemaRun(root, overlay, selector, h.MutationTimeout)
			return out.Failed || out.TimedOut || out.Err != nil
		}
		h.SchemaRun = func(root, overlay, selector string, timeout time.Duration) CommandResult {
			env := envWithGOCACHE(os.Environ(), gocache)
			env = append(envWithout(env, "GOFLAGS", schemaEnv),
				"GOFLAGS="+schemaGoflags(os.Getenv("GOFLAGS"), overlay),
				schemaEnv+"="+selector)
			return runShell(root, s.Test, env, timeout)
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
				fmt.Fprintf(stderr, "muhar: cwd: %v\n", err)
				return 1
			}
			srcRoot = wd
		}
		// Each copy keeps its go.mod's relative replace targets resolvable
		// (#2550): see copyModuleTree.
		h.Workspace = newWorkspace(srcRoot)
		fmt.Fprintf(stderr, "muhar: %d mutations in flight, one isolated copy of %s per worker\n", n, srcRoot)
	}

	rep := h.Run()
	fmt.Fprint(stdout, rep.Summary())
	if rep.Broken {
		return 2
	}
	return 0
}

// loadSpec reads and checks the mutation spec: the only configuration muhar reads.
func loadSpec(path string, stderr io.Writer) (spec, int) {
	var s spec
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "muhar: read spec: %v\n", err)
		return s, 1
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		fmt.Fprintf(stderr, "muhar: parse spec: %v\n", err)
		return s, 1
	}
	if s.Test == "" {
		fmt.Fprintln(stderr, "muhar: spec.test (the suite command) is required")
		return s, 1
	}
	if _, err := s.Timeout.policy(); err != nil {
		fmt.Fprintf(stderr, "muhar: %v\n", err)
		return s, 1
	}
	if s.Control.Name == "" || s.Control.Old == "" {
		fmt.Fprintln(stderr, "muhar: spec.control is required (a mutation the suite MUST catch) — a harness with no failing control is a green light with no bulb (#34)")
		return s, 1
	}
	return s, 0
}
