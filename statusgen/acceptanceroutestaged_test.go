package main

import (
	"os"
	"strings"
	"testing"
)

func TestStagedAcceptanceRouteWorkflowPair(t *testing.T) {
	for path, command := range map[string]string{
		"verify-gate-open.yml":  "acceptance-route issue",
		"verify-gate-close.yml": "acceptance-route close",
	} {
		t.Run(path, func(t *testing.T) {
			raw, err := os.ReadFile("../ci/staged-workflows/" + path)
			if err != nil {
				t.Fatal(err)
			}
			wf := parseWorkflow(t, raw, path)
			count := 0
			for _, job := range wf.Jobs {
				for _, step := range job.Steps {
					if !strings.Contains(step.Run, command) {
						continue
					}
					count++
					if !strings.Contains(step.Run, "verification-handoff/03") || !strings.Contains(step.Run, "--acceptance-forge") {
						t.Errorf("unexpected route scope in step %s", step.Name)
					}
					if step.Env["DESKREAD_CI_WORKFLOW_TOKEN"] != "${{ github.token }}" ||
						step.Env["GH_TOKEN"] != "${{ github.token }}" ||
						step.Env["ASSAY_HUMAN_LOGIN_MAP"] != "${{ vars.ASSAY_HUMAN_LOGIN_MAP }}" {
						t.Errorf("missing explicit CI token or mapped-human context in step %s", step.Name)
					}
					for _, value := range step.Env {
						if strings.Contains(value, "steps.app-token.outputs.token") {
							t.Errorf("route reader has board writer credentials in step %s", step.Name)
						}
					}
				}
			}
			if count != 1 {
				t.Fatalf("found %d %q invocations, want one", count, command)
			}
		})
	}
}
