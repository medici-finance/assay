//go:build cellopfixture && (darwin || linux)

package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/medici-finance/assay/tools/desk/internal/cellop"
)

// Rows 8 and 8b of desk-containers/15, against the cellctl binary built with the fixture
// (`-tags cellopfixture`), on a real pseudo-terminal. The fixture registers stand-ins for every
// verb of the enumerated list whose brief has not landed, so the list is checked end to end today
// and each real verb, once registered, is checked in its stand-in's place.

// cellopArgs is the command line that drives inv on the example cell, with a stored plan id for a
// string-valued flag.
func cellopArgs(t *testing.T, w *cellopWorld, root *cobra.Command, inv cellopInvocation) []string {
	t.Helper()
	c := commandAt(root, inv.path)
	args := append(strings.Fields(strings.TrimPrefix(inv.path, "cellctl ")), "example")
	if inv.flag == "" {
		return args
	}
	f := declaredFlag(c, inv.flag)
	if f.Value.Type() == "bool" {
		return append(args, "--"+inv.flag)
	}
	return append(args, "--"+inv.flag+"="+w.fixturePlan(t, c.Name()).ID)
}

// parallel runs fn for each item with at most n at once.
func parallel[T any](items []T, n int, fn func(T)) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, n)
	for _, it := range items {
		wg.Add(1)
		sem <- struct{}{}
		go func(it T) {
			defer wg.Done()
			defer func() { <-sem }()
			fn(it)
		}(it)
	}
	wg.Wait()
}

// walkRun is one run of the behavioural walk and what it left at the apply path.
type walkRun struct {
	args  []string
	trace []string
	run   cellopRun
}

// An unannotated invocation, or one exempted from the terminal rule, could apply off a terminal (desk-containers/15).
// regression: #2890
func TestEveryStateChangingInvocationRequiresTTY(t *testing.T) {
	root := buildRoot()
	invs := stateChangingInvocations(root)

	t.Run("enumerated list registered and annotated", func(t *testing.T) {
		if got := enumeratedProblems(root); len(got) != 0 {
			t.Fatalf("the enumerated state-changing list is not fully registered and annotated: %v", got)
		}
	})
	t.Run("apply, run, now and foreign annotated", func(t *testing.T) {
		if got := consentNameProblems(root); len(got) != 0 {
			t.Fatal(got)
		}
	})

	t.Run("refused off a terminal", func(t *testing.T) {
		type arm struct {
			inv  cellopInvocation
			mode ttyMode
		}
		var arms []arm
		for _, inv := range invs {
			for _, m := range []ttyMode{ttyNone, ttyStdin, ttyStdout} {
				arms = append(arms, arm{inv, m})
			}
		}
		var mu sync.Mutex
		var fails []string
		parallel(arms, 8, func(a arm) {
			w := newCellopWorld(t)
			args := cellopArgs(t, w, root, a.inv)
			r := runCellctl(t, w, a.mode, w.env("DRY_RUN=1"), nil, args...)
			var why []string
			if r.code != 5 || !strings.Contains(r.out, cellop.ReasonNotInteractive) {
				why = append(why, fmt.Sprintf("exit %d, want 5 not-interactive", r.code))
			}
			if tr := w.traced(t); len(tr) != 0 {
				why = append(why, fmt.Sprintf("reached the apply path: %v", tr))
			}
			if e := w.effects(t); len(e) != 0 {
				why = append(why, fmt.Sprintf("ran %v", e))
			}
			if len(why) != 0 {
				mu.Lock()
				fails = append(fails, fmt.Sprintf("%v (%s): %s\n%s", args, a.mode, strings.Join(why, "; "), r.out))
				mu.Unlock()
			}
		})
		for _, f := range fails {
			t.Error(f)
		}
	})

	t.Run("--plan works off a terminal", func(t *testing.T) {
		var planners []*cobra.Command
		walkTree(root, func(c *cobra.Command) {
			if declaredFlag(c, "plan") != nil {
				planners = append(planners, c)
			}
		})
		if len(planners) < len(cellopEnumerated)-2 {
			t.Fatalf("only %d verbs have --plan", len(planners))
		}
		for _, c := range planners {
			w := newCellopWorld(t)
			args := append(strings.Fields(strings.TrimPrefix(c.CommandPath(), "cellctl ")), "example", "--plan", "--json")
			r := runCellctl(t, w, ttyNone, w.env("DRY_RUN=1"), nil, args...)
			if r.code != 0 {
				t.Errorf("%v off a terminal: exit %d\n%s", args, r.code, r.out)
				continue
			}
			p, err := cellop.DecodePlan([]byte(r.stdout))
			if err != nil || p.Verb != c.Name() {
				t.Errorf("%v: stdout is not a %s plan (%v)\n%s", args, c.Name(), err, r.stdout)
				continue
			}
			if _, err := w.store().Load(p.ID); err != nil {
				t.Errorf("%v: the printed plan is not in the store: %v", args, err)
			}
			if len(w.traced(t)) != 0 || len(w.effects(t)) != 0 {
				t.Errorf("%v: --plan reached the apply path or ran an effect", args)
			}
		}
	})

	t.Run("bare read-only forms stay read-only off a terminal", func(t *testing.T) {
		for _, verb := range []string{"doctor", "gc", "upgrade", "cellop-fixture"} {
			w := newCellopWorld(t)
			r := runCellctl(t, w, ttyNone, w.env("DRY_RUN=1"), nil, verb, "example")
			if r.code != 0 || w.storedPlans(t) != 0 || len(w.traced(t)) != 0 {
				t.Errorf("bare %s off a terminal: exit %d, %d plans written, trace %v\n%s", verb, r.code, w.storedPlans(t), w.traced(t), r.out)
			}
		}
	})

	// The behavioural arm: every form of every verb that can reach the apply path — each verb with
	// a --plan flag or an annotation, bare and with each of its flags — is run on a terminal, and
	// the fixture records which forms arrived at cellopApply (TestOneApplyPath pins that as the
	// only apply path). Every arrival must be annotated; every annotated invocation must arrive.
	var runs []walkRun
	t.Run("every arrival at apply is annotated", func(t *testing.T) {
		type job struct {
			c    *cobra.Command
			flag *pflag.Flag
		}
		var jobs []job
		walkTree(root, func(c *cobra.Command) {
			adopter := commandStateChanging(c) || declaredFlag(c, "plan") != nil
			eachFlag(c, func(f *pflag.Flag) {
				if flagStateChanging(f) {
					adopter = true
				}
			})
			if !adopter || c.DisableFlagParsing {
				return
			}
			jobs = append(jobs, job{c: c})
			eachFlag(c, func(f *pflag.Flag) {
				if f.Name != "help" && f.Name != "cells-root" {
					jobs = append(jobs, job{c: c, flag: f})
				}
			})
		})
		var mu sync.Mutex
		parallel(jobs, 8, func(j job) {
			w := newCellopWorld(t)
			inv := cellopInvocation{path: j.c.CommandPath()}
			if j.flag != nil {
				inv.flag = j.flag.Name
			}
			args := cellopArgs(t, w, root, inv)
			r := runCellctl(t, w, ttyBoth, w.env("DRY_RUN=1"), func(string) string { return "not-the-code" }, args...)
			mu.Lock()
			runs = append(runs, walkRun{args: args, trace: w.traced(t), run: r})
			mu.Unlock()
		})
		var trace []string
		arrived := map[string]bool{}
		for _, r := range runs {
			if r.run.timedOut {
				t.Errorf("%v timed out on the terminal\n%s", r.args, r.run.out)
			}
			// An arrival met the typed confirm on the terminal, and the wrong code stopped it.
			if len(r.trace) != 0 && (r.run.code != 5 || !strings.Contains(r.run.out, cellop.ReasonConfirmMismatch)) {
				t.Errorf("%v reached apply but did not stop at the typed confirm: exit %d\n%s", r.args, r.run.code, r.run.out)
			}
			trace = append(trace, r.trace...)
			for _, l := range r.trace {
				arrived[l] = true
			}
		}
		if got := applyTraceViolations(root, trace); len(got) != 0 {
			t.Fatalf("forms reached the apply path without an annotation: %v", got)
		}
		var missing []string
		for _, inv := range invs {
			found := false
			for l := range arrived {
				path, given, _ := strings.Cut(l, "\t")
				if path != inv.path {
					continue
				}
				if inv.flag == "" || strings.Contains(","+given+",", ","+inv.flag+",") {
					found = true
				}
			}
			if !found {
				missing = append(missing, inv.String())
			}
		}
		sort.Strings(missing)
		if len(missing) != 0 {
			t.Fatalf("annotated invocations never reached the apply path on a terminal (the walk is vacuous for them): %v", missing)
		}
	})

	t.Run("exempting any one invocation fails", func(t *testing.T) {
		var trace []string
		for _, r := range runs {
			trace = append(trace, r.trace...)
		}
		if len(trace) == 0 {
			t.Fatal("no traces from the behavioural walk")
		}
		for i := range invs {
			m := buildRoot()
			inv := stateChangingInvocations(m)[i]
			c := commandAt(m, inv.path)
			if inv.flag == "" {
				delete(c.Annotations, stateChanging)
			} else {
				delete(declaredFlag(c, inv.flag).Annotations, stateChanging)
			}
			caught := len(enumeratedProblems(m)) + len(consentNameProblems(m)) + len(applyTraceViolations(m, trace))
			if caught == 0 {
				t.Errorf("exempting %s is not caught by any check", inv)
			}
			if len(applyTraceViolations(m, trace)) == 0 {
				t.Errorf("exempting %s is not caught by the behavioural arm", inv)
			}
		}
	})
}

// A desk session (ASSAY_CELL, DESK_LOOP, DESK_SESSION) must not apply even on a terminal (desk-containers/15).
// regression: #2890
func TestApplyRefusesDeskSession(t *testing.T) {
	root := buildRoot()
	type arm struct {
		inv    cellopInvocation
		marker string
	}
	var arms []arm
	for _, inv := range stateChangingInvocations(root) {
		for _, m := range cellop.DeskSessionMarkers {
			arms = append(arms, arm{inv, m})
		}
	}
	if len(arms) == 0 {
		t.Fatal("no state-changing invocations")
	}
	var mu sync.Mutex
	var fails []string
	parallel(arms, 8, func(a arm) {
		w := newCellopWorld(t)
		args := cellopArgs(t, w, root, a.inv)
		r := runCellctl(t, w, ttyBoth, w.env("DRY_RUN=1", a.marker+"=fixture"), func(string) string { return "not-the-code" }, args...)
		if r.code != 5 || !strings.Contains(r.out, cellop.ReasonDeskSession) || len(w.traced(t)) != 0 || len(w.effects(t)) != 0 {
			mu.Lock()
			fails = append(fails, fmt.Sprintf("%v with %s set on a terminal: exit %d, trace %v, effects %v\n%s", args, a.marker, r.code, w.traced(t), w.effects(t), r.out))
			mu.Unlock()
		}
	})
	for _, f := range fails {
		t.Error(f)
	}
}
