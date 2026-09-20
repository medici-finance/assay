package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

// printPlan emits the [plan] grammar the scrubbed-cell brief froze: one `env` line per exported
// variable (KEY-sorted), then argv, cwd, lock. It is printed after the [dry-run] line under
// DRY_RUN=1, and this exact grammar is what the parity harness diffs — part of the contract,
// not a courtesy. A change to it is a change to BOTH implementations in one PR or it is a
// parity failure.
func (c *Cell) printPlan(env ComposedEnv, cwd string, argv []string) {
	for _, kv := range env.Sorted {
		// parityDropsPlanEnv is the test-only divergence injector. It is compiled in ONLY under
		// `-tags parity` (see parity_on.go / parity_off.go): the release build's copy is a
		// constant false, so a shipped binary carries neither the check nor the variable that
		// would drive it. Rows 13 and 14 prove that, rather than asserting it.
		if parityDropsPlanEnv(kv) {
			continue
		}
		fmt.Printf("[plan] env %s\n", kv)
	}
	var q strings.Builder
	for _, a := range argv {
		q.WriteString(" ")
		q.WriteString(bashQuote(a))
	}
	fmt.Printf("[plan] argv%s\n", q.String())
	fmt.Printf("[plan] cwd %s\n", cwd)
	fmt.Printf("[plan] lock %s\n", filepath.Join(c.Dir, "run", "lock.d"))
}

// harnessArgv is the command a role window runs under a scrubbed cell, identical on the dry-run
// and the live path so the plan is a plan of exactly the launch that follows it.
func harnessArgv(harness, role, model, session, wt string) []string {
	if harness == "codex" {
		return []string{"codex", "--sandbox", "danger-full-access", "-C", wt, "-m", model,
			fmt.Sprintf("Invoke the %q skill now.", "assay:"+role)}
	}
	return []string{"claude", "--name", session, "--model", model, "/assay:" + role}
}
