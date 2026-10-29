package clicontract

import (
	"io/fs"
	"strings"
	"testing"
)

// regression: statusgen/26
func TestCLIPerfWorkloadRoute(t *testing.T) {
	base, raw := realTree(t)
	reg := decode(t, raw)
	const script = "statusgen/bench/perfworkload.sh"
	const ownerID = "statusgen/26"
	owner, err := LoadBrief(base, ownerID)
	if err != nil || owner == nil || !strings.Contains(owner.Text, script) {
		t.Fatalf("workload route must name its actual owning brief: %v", err)
	}
	index := -1
	for i, rule := range reg.ExclusionRules {
		if rule.Pattern == script && rule.Class == "test" && strings.Contains(rule.Reason, ownerID) {
			index = i
		}
	}
	if index < 0 {
		t.Fatal("workload needs an exact test-class route naming statusgen/26")
	}
	if problems := Validate(base, reg); len(problems) != 0 {
		t.Fatalf("baseline routing: %v", problems)
	}
	// The neighboring operator benchmark retains its distinct migration owner.
	neighbor := rowFor(t, reg, "statusgen/bench/lintbench.sh")
	if neighbor.Owner != "desktools-v2/53" || neighbor.State != StatePending {
		t.Fatalf("operator benchmark route changed: %+v", neighbor)
	}
	for _, planted := range []string{
		"statusgen/bench/other.sh",
		"statusgen/other-bench/perfworkload.sh",
	} {
		t.Run(planted, func(t *testing.T) {
			fsys := newOverlay(base, map[string]string{planted: "#!/bin/sh\necho planted\n"})
			problems := Validate(fsys, decode(t, raw))
			want := "unrouted entrypoint " + planted + " (script)"
			if !strings.Contains(strings.Join(problems, "\n"), want) {
				t.Fatalf("neighboring script must remain unrouted: %v", problems)
			}
			t.Log(want)
		})
	}
	t.Run("removed-route", func(t *testing.T) {
		mutant := decode(t, raw)
		mutant.ExclusionRules = append(mutant.ExclusionRules[:index], mutant.ExclusionRules[index+1:]...)
		problems := Validate(base, mutant)
		want := "unrouted entrypoint " + script + " (script)"
		if !strings.Contains(strings.Join(problems, "\n"), want) {
			t.Fatalf("removing the route must fail discovery: %v", problems)
		}
		t.Log(want)
	})
	t.Run("shipped-workload", func(t *testing.T) {
		makefile, err := fs.ReadFile(base, "Makefile")
		if err != nil {
			t.Fatal(err)
		}
		fsys := newOverlay(base, map[string]string{"Makefile": string(makefile) + "\nworkload-cli:\n\tcp " + script + " workload-cli\n"})
		problems := Validate(fsys, decode(t, raw))
		want := "shipped entrypoint " + script + " is excluded"
		if !strings.Contains(strings.Join(problems, "\n"), want) {
			t.Fatalf("shipping the workload must invalidate its test route: %v", problems)
		}
		t.Log(want)
	})
}
