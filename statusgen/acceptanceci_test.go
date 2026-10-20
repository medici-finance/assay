package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

// Use the existing workflow parser and credential fixtures. Scan every invocation,
// including retry bodies and newly added steps, rather than a step-name allowlist.
var acceptanceCall = regexp.MustCompile(`"(?:\$\{RUNNER_TEMP\}/statusgen|\$statusgen)"\s+((?:--root|reconcile|regen)\b[^\n]*)`)

func acceptanceCIProblems(wf workflow) ([]string, int) {
	var problems []string
	count := 0
	for name, job := range wf.Jobs {
		perms, _ := job.Permissions.(map[string]any)
		if job.Permissions == nil {
			perms, _ = wf.Permissions.(map[string]any)
		}
		for _, step := range job.Steps {
			env := map[string]string{}
			for _, source := range []map[string]string{wf.Env, job.Env, step.Env} {
				for k, v := range source {
					env[k] = v
				}
			}
			for _, line := range strings.Split(step.Run, "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "#") {
					continue
				}
				for _, call := range acceptanceCall.FindAllStringSubmatch(line, -1) {
					args := call[1]
					where := name + "/" + step.Name
					report := func(s string) { problems = append(problems, where+": "+s) }
					if strings.HasPrefix(args, "regen --readmes ") {
						if strings.Contains(args, "--acceptance-forge") {
							report("README-only regen rejects acceptance flag")
						}
						continue
					}
					count++
					if !strings.Contains(args, "--acceptance-forge") {
						report("missing acceptance opt-in")
					}
					if env["ASSAY_HUMAN_LOGIN_MAP"] != "${{ vars.ASSAY_HUMAN_LOGIN_MAP }}" {
						report("missing human map")
					}
					token := env["GH_TOKEN"]
					if token == "" {
						token = env["GITHUB_TOKEN"]
					}
					if token != "${{ github.token }}" {
						report("acceptance resolver lacks explicit default token")
					}
					for _, v := range env {
						if strings.Contains(v, "steps.app-token.outputs.token") {
							report("acceptance step holds publisher token")
						}
					}
					if perms["issues"] != "read" && !(name == "close" && perms["issues"] == "write") {
						report("missing issue read permission")
					}
					if name == "model-autoflip" && perms["pull-requests"] != "read" {
						report("missing review read permission")
					}
				}
			}
		}
	}
	return problems, count
}

// regression: #3493
func TestAcceptanceCI(t *testing.T) {
	for path, want := range map[string]int{
		"../.github/workflows/assay-statusgen.yml":        7,
		"../ci/staged-workflows/assay-statusgen.yml":      7,
		"../tools/ci-load/activation/assay-statusgen.yml": 5,
		"../.github/workflows/verify-gate-close.yml":      2,
		"../ci/staged-workflows/verify-gate-close.yml":    2,
	} {
		t.Run(path, func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			problems, count := acceptanceCIProblems(parseWorkflow(t, raw, path))
			if count != want {
				t.Errorf("found %d acceptance calls, want %d", count, want)
			}
			for _, p := range problems {
				t.Error(p)
			}
		})
	}
}

// An independent second caller exercises the same invariant without modifying
// any deployed step. Each mutation must identify that planted caller.
// regression: #3493
func TestAcceptanceCIPlant(t *testing.T) {
	const good = `permissions:
  contents: read
  issues: read
jobs:
  planted:
    steps:
      - name: second-caller
        env:
          GH_TOKEN: ${{ github.token }}
          ASSAY_HUMAN_LOGIN_MAP: ${{ vars.ASSAY_HUMAN_LOGIN_MAP }}
        run: |
          "${RUNNER_TEMP}/statusgen" --root . --lint --acceptance-forge
`
	for _, tc := range []struct{ name, before, after, want string }{
		{"positive", "", "", ""},
		{"opt-in", " --acceptance-forge", "", "missing acceptance opt-in"},
		{"map", "${{ vars.ASSAY_HUMAN_LOGIN_MAP }}", "", "missing human map"},
		{"token", "${{ github.token }}", "", "lacks explicit default token"},
		{"publisher", "${{ github.token }}", appTokenExpr, "holds publisher token"},
		{"permission", "issues: read", "issues: none", "missing issue read permission"},
		{"readmes", "--root . --lint", "regen --readmes --root .", "README-only regen rejects"},
		{"retry", " --acceptance-forge", " --acceptance-forge\n          \"${RUNNER_TEMP}/statusgen\" --root .", "missing acceptance opt-in"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := good
			if tc.before != "" {
				raw = strings.Replace(raw, tc.before, tc.after, 1)
			}
			problems, _ := acceptanceCIProblems(parseWorkflow(t, []byte(raw), "plant"))
			got := strings.Join(problems, "\n")
			if tc.want == "" {
				if got != "" {
					t.Fatal(got)
				}
			} else if !strings.Contains(got, "planted/second-caller: ") || !strings.Contains(got, tc.want) {
				t.Fatal(fmt.Sprintf("plant escaped %q: %s", tc.want, got))
			} else {
				t.Log(got)
			}
		})
	}
}
