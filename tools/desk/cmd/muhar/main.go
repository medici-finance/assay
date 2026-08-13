// muhar — mutation-testing harness with a proven-landed guarantee (#34).
//
// Usage:
//
//	muhar -spec mutations.json
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

	h := &Harness{
		Root:      s.Root,
		Control:   s.Control,
		Mutations: s.Mutations,
		Test: func() bool {
			// s.Test is an operator-authored suite command from the spec file
			// (e.g. "go test ./... && ./verify.sh") — it legitimately needs a
			// shell for pipes/&&, and the spec author already controls the box
			// this runs on. It is configuration, not end-user input, so `sh -c`
			// is the intended execution path, not an injection surface.
			cmd := exec.Command("sh", "-c", s.Test) //nolint:gosec // operator-supplied suite command, by design

			if s.Root != "" {
				cmd.Dir = s.Root
			}
			cmd.Stdout = os.Stderr // suite chatter goes to stderr; report to stdout
			cmd.Stderr = os.Stderr
			// Non-zero exit == suite failed == mutation caught.
			return cmd.Run() != nil
		},
	}

	rep := h.Run()
	fmt.Print(rep.Summary())
	if rep.Broken {
		os.Exit(2)
	}
}
