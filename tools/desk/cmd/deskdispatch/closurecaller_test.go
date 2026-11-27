package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #4823
func TestOutcomeAuthorityRecipes(t *testing.T) {
	prompt := dispatchPrompt(t, "verifier", "verify-item-9")
	commands := 0
	for _, line := range strings.Split(prompt, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "deskevidence ") || !strings.Contains(line, "--outcome-record ") {
			continue
		}
		commands++
		permitted := false
		for _, arg := range strings.Fields(line) {
			permitted = permitted || arg == "--acceptance-forge"
		}
		if !permitted {
			t.Errorf("outcome caller omits fresh approval permission: %s", line)
		}
	}
	if commands == 0 {
		t.Fatal("no executable outcome caller was emitted")
	}
}

// regression: #4823
func TestOutcomeForgeRecipes(t *testing.T) {
	for _, forge := range []deskkit.ForgeKind{deskkit.ForgeGitHub, deskkit.ForgeGitLab, ""} {
		var out strings.Builder
		target := deskkit.ForgeResolution{Repo: deskkit.ForgeRepo{Owner: "example", Name: "repo"}, Kind: forge}
		writeOutcomeRecordLine(&out, dispatchPlan{outcomePath: "outcomes", outcomeTarget: target}, "home")
		permitted := false
		for _, line := range strings.Split(out.String(), "\n") {
			if strings.HasPrefix(line, "deskevidence ") {
				permitted = strings.Contains(line, " --acceptance-forge")
			}
		}
		if permitted != (forge == deskkit.ForgeGitHub) {
			t.Errorf("forge %q: explicit approval recipe permission=%v", forge, permitted)
		}
		if paired := strings.Contains(out.String(), "--flip-verified"); paired != permitted {
			t.Errorf("forge %q: paired flip recipe=%v, want %v", forge, paired, permitted)
		}
	}
}

// regression: #4823
func TestVerifierForgeRecipes(t *testing.T) {
	for _, tc := range []struct {
		name, host string
		code       int
		permission bool
	}{
		{"github", "github.com", deskkit.ExitOK, true},
		{"gitlab", "gitlab.com", deskkit.ExitOK, false},
		{"unknown", "unmapped.invalid", deskkit.ExitUnverifiable, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prompt, code := dispatchPromptOriginRC(t, "https://"+tc.host+"/medici-finance/assay.git", "verifier", "verify-item-9")
			if code != tc.code {
				t.Fatalf("verifier forge dispatch exit=%d want=%d", code, tc.code)
			}
			if code != deskkit.ExitOK {
				if prompt != "" {
					t.Fatal("unknown forge emitted an outcome recipe")
				}
				return
			}
			found := false
			for _, line := range strings.Split(prompt, "\n") {
				if strings.HasPrefix(line, "deskevidence ") {
					found = true
					if got := strings.Contains(line, " --acceptance-forge"); got != tc.permission {
						t.Errorf("verifier forge %s: approval permission=%v want=%v", tc.name, got, tc.permission)
					}
				}
			}
			if !found {
				t.Fatal("verifier emitted no outcome recipe")
			}
		})
	}
}
