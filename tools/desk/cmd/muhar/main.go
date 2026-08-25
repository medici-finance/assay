// muhar — mutation-testing harness with a proven-landed guarantee (#34).
//
// Usage:
//
//	muhar -spec mutations.json [-j N]
//
// -j is how many mutations to have in flight at once. The default, 1, runs the
// sweep sequentially in place — the historical behaviour. -j 0 sizes the pool to
// the machine's CPU count. With -j > 1 each worker gets its own COPY of the
// spec's root, because a sweep whose workers shared a tree would be editing each
// other's source and every verdict would be a coin flip. The verdicts, their
// order, and the report text are unchanged by -j; only the wall time is.
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
)

type spec struct {
	Root      string     `json:"root"`
	Test      string     `json:"test"`
	Control   Mutation   `json:"control"`
	Mutations []Mutation `json:"mutations"`
}

// muhar does NOT call deskkit.Guard(): it is a local diagnostic that makes no
// outward writes (no GitHub, no shared-state mutation — it edits a source file
// and restores it within the same run), so the kill switch and outward-write
// rate limit do not apply. This is the same documented exemption class as
// writeguard; halting local mutation testing serves no safety purpose.
func main() {
	specPath := flag.String("spec", "", "path to the mutation spec JSON")
	jobs := flag.Int("j", 1, "mutations in flight at once (1 = sequential, in place; 0 = one per CPU)")
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
