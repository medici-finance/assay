package clicontract

import (
	"io/fs"
	"strings"
	"testing"
)

// regression: #2395
func TestCLIGitLaunchRoutes(t *testing.T) {
	base, raw := realTree(t)
	reg := decode(t, raw)
	owner, err := LoadBrief(base, "desktools-v2/76")
	if err != nil || owner == nil {
		t.Fatalf("Git ratchet owner unavailable: %v", err)
	}
	paths := []string{"statusgen/scripts/count-git-launch.sh", "statusgen/internal/gitlaunch"}
	for _, p := range paths {
		if !strings.Contains(owner.Text, p) {
			t.Fatalf("owner does not declare %s", p)
		}
		found := false
		for _, rule := range reg.ExclusionRules {
			if rule.Pattern == p {
				if rule.Class != "test" || !strings.Contains(rule.Reason, "desktools-v2/76") {
					t.Fatalf("wrong Git test route: %+v", rule)
				}
				found = true
			}
		}
		if !found {
			t.Fatalf("missing exact Git test route %s", p)
		}
	}
	if problems := Validate(base, reg); len(problems) != 0 {
		t.Fatalf("healthy routing: %v", problems)
	}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			removed := decode(t, raw)
			var rules []Rule
			for _, r := range removed.ExclusionRules {
				if r.Pattern != p {
					rules = append(rules, r)
				}
			}
			removed.ExclusionRules = rules
			if problems := Validate(base, removed); !strings.Contains(strings.Join(problems, "\n"), "unrouted entrypoint "+p) {
				t.Fatalf("route removal not caught: %v", problems)
			}
			makefile, err := fs.ReadFile(base, "Makefile")
			if err != nil {
				t.Fatal(err)
			}
			shipped := newOverlay(base, map[string]string{"Makefile": string(makefile) + "\nratchet-cli:\n\tcp " + p + " installed-ratchet\n"})
			if problems := Validate(shipped, decode(t, raw)); !strings.Contains(strings.Join(problems, "\n"), "shipped entrypoint "+p+" is excluded") {
				t.Fatalf("shipping did not invalidate source-only route: %v", problems)
			}
		})
	}
	for p, source := range map[string]string{"statusgen/scripts/neighbor.sh": "#!/bin/sh\nexit 0\n", "statusgen/internal/gitlaunch-neighbor/main.go": "package main\nfunc main(){}\n"} {
		tree := newOverlay(base, map[string]string{p: source})
		want := p
		if strings.HasSuffix(p, "/main.go") {
			want = strings.TrimSuffix(p, "/main.go")
		}
		if problems := Validate(tree, decode(t, raw)); !strings.Contains(strings.Join(problems, "\n"), "unrouted entrypoint "+want) {
			t.Fatalf("neighbor route was broadened: %v", problems)
		}
	}
}
