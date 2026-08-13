package deskkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// P3 coverage guard.
//
// A correctness review found the effective-config echo wired into only some of the
// mains that read a configured control surface, while the consumer
// documentation claimed "every desk tool echoes its effective roster ... on every
// run". It also found a mutation — "P3 echo removed from deskpost's main" —
// SURVIVING the whole suite. Both are the same gap: nothing asserted the set.
//
// This enumerates cmd/ from the tree rather than carrying a hand-written list, so a
// NEW tool that reads the roster is covered the day it is added instead of the day
// someone remembers to extend a literal.
//
// A source-text control is the honest instrument here: the property is "this call
// exists in this main", and there is no observable behaviour to drive from a unit
// test without executing each binary. It is deliberately paired with the behavioural
// echo tests (TestEffectiveConfigLines*) that pin what the echo SAYS.
const cmdDir = "../../cmd"

// exemptFromRoster are commands that legitimately never read a configured control
// surface. Each is listed with the reason, so adding a name here is a decision
// someone has to write down rather than a silent opt-out.
var exemptFromRoster = map[string]string{
	"desktoken":       "mints App tokens from apps.env + PEM; makes no trust decision",
	"muhar":           "harvest/report utility; consults no roster",
	"deskpushguard":   "PR-state guard; consults no roster",
	"repohardenguard": "GET-only hardening checker; reads the checklist doc + the repo's own live settings, deliberately NOT the roster (see its main.go § write-authorisation set)",
}

func TestEveryRosterReadingMainDeclaresClassAndEchoes(t *testing.T) {
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		t.Fatalf("cannot enumerate %s: %v", cmdDir, err)
	}
	checked := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		dir := filepath.Join(cmdDir, name)

		files, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("cannot read %s: %v", dir, err)
		}
		// Does this command use the roster at all?
		usesRoster := false
		var mainPath string
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".go") || strings.HasSuffix(f.Name(), "_test.go") {
				continue
			}
			p := filepath.Join(dir, f.Name())
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatalf("cannot read %s: %v", p, err)
			}
			src := string(b)
			for _, sym := range []string{"EffectiveConfig(", "TrustedAuthor", "IsAllowedRepo", "RoleAppLogin",
				"IsBlessAuthority", "ItemTrusted", "RosterUnconfiguredError", "CIRequired(", "AllowedRepos("} {
				if strings.Contains(src, sym) {
					usesRoster = true
				}
			}
			if strings.Contains(src, "\nfunc main() {") {
				mainPath = p
			}
		}
		if reason, ok := exemptFromRoster[name]; ok {
			if usesRoster {
				t.Errorf("cmd/%s is listed exempt (%q) but reads a configured control surface — "+
					"remove the exemption or stop reading the roster", name, reason)
			}
			continue
		}
		if !usesRoster || mainPath == "" {
			continue
		}
		checked++

		b, _ := os.ReadFile(mainPath)
		src := string(b)
		if !strings.Contains(src, "SetToolClass(") {
			t.Errorf("cmd/%s reads the roster but its main does not DECLARE a tool class. "+
				"ClassWrite-by-omission is the failure this guards against: the class was never chosen, "+
				"only defaulted. Call deskkit.SetToolClass(deskkit.ClassForTool(<ciEligible>)).", name)
		}
		if !strings.Contains(src, "EchoEffectiveConfig(") {
			t.Errorf("cmd/%s reads the roster but its main does not echo the effective config (P3). "+
				"A control surface that lives in settings rather than in a diff is visible only at "+
				"RUN time — without the echo a NARROWING is invisible.", name)
		}
	}
	if checked < 8 {
		t.Fatalf("only %d roster-reading commands were checked — the enumeration is not finding "+
			"them, so this guard would pass with the echo removed everywhere", checked)
	}
}
