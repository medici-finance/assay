package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The real binary prepares local Git worktrees and invokes a harmless harness.
// Native execution of this same bootstrap is separately qualified; text checks
// alone cannot prove that an installed model creates or preserves a Goal.
// regression: #2696
func TestGoalHostLaunch(t *testing.T) {
	for _, mode := range []string{"default", "off"} {
		t.Run(mode, func(t *testing.T) {
			f := catalogFixture(t)
			f.prepareLocalLaunch(t)
			if err := os.WriteFile(filepath.Join(f.binDir, "codex"), []byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\n"), 0755); err != nil {
				t.Fatal(err)
			}
			writeCatalogJSON(t, filepath.Join(f.cellDir, "providers.json"), map[string]any{"roles": map[string]string{"worker-desk": "codex"}})
			args := []string{"desk", "example", "worker-desk"}
			if mode == "off" {
				args = append(args, "--cadence", "off")
			}
			r := f.run(t, []string{"CELLCTL_DESKWT=0"}, args...)
			if r.code != 0 {
				t.Fatalf("launch: %+v", r)
			}
			for _, want := range []string{
				"Call get_goal first.", "Only if no Goal exists, call create_goal", "omit token_budget, then call get_goal again",
				"preserve its objective, status, budget, and usage", "otherwise report the existing state and wait for user direction",
				"could-not-check: standing Goal not established", "do not resume or recreate a Goal after the user pauses or clears it",
				"An inactive Goal gates standing desk continuation, not separately authorized corrective work.",
				"complete the authorized correction first, then call get_goal again",
				"If readback is still inactive and no native resume tool is available, give the user the exact /goal resume control",
				"A plain-language resume request is authorization, not evidence that native status changed.",
				`Invoke the "assay:worker-desk" skill now.`, "model_reasoning_effort=", "login=false",
			} {
				if !strings.Contains(r.stdout, want) {
					t.Errorf("missing %q: %s", want, r.stdout)
				}
			}
			for _, forbidden := range []string{"\nexec\n", "features.goals=false", "ASSAY_TICK=1", "exactly one bounded pass"} {
				if strings.Contains(r.stdout, forbidden) {
					t.Errorf("interactive launch contains %q", forbidden)
				}
			}
		})
	}
}

// regression: #2696
func TestGoalCadenceChoices(t *testing.T) {
	f := newPolicyFixture(t, "2.1.278")
	r := f.dryRunDesk(t, "intake-desk", "--tick-budget", "40m")
	if r.code == 0 || !strings.Contains(r.stderr, "requires --cadence") {
		t.Fatalf("budget-only accepted: %+v", r)
	}
	r = f.run(t, []string{"DRY_RUN=1", "CELL_CADENCE=11m", "CELL_TICK_BUDGET=35m"}, "desk", "example", "intake-desk")
	if r.code != 0 || !strings.Contains(r.stdout, "interval=11m0s budget=35m0s") {
		t.Fatalf("environment cadence lost: %+v", r)
	}
}
