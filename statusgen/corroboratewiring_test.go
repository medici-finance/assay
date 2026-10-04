package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestCorroborateIsWiredIntoStatusgenWorkflow pins the register-authorization
// corroboration wiring: WHERE a repo's CI invokes `statusgen --corroborate`, it
// must invoke it AS A LIVE GATE, or the register-authorization anchor
// (`authorized-by: human:<name>`) is one line an agent writes for itself and the
// "verified online by statusgen --corroborate" claim in registers.go's HARD GATE
// comment describes an intent, not an enforced property.
//
// This is a WIRING test, the same layer as version_test.go's "release workflow
// stamps the tag": the corroboration LOGIC has its own unit coverage
// (corroborate_test.go), but every one of those tests passes with the command
// invoked by nothing. Only reading the workflow proves the gate actually runs.
//
// It is workflow-name-agnostic: it scans every file under .github/workflows/ for
// the invocation rather than hardcoding one filename, so it holds regardless of
// what a given adopter names its CI. If NO workflow wires `--corroborate --pr`
// (the gate is not adopted in this repo's CI), it SKIPS — there is nothing to
// pin — rather than dictating that a repo must adopt the gate.
//
// It parses the workflow rather than substring-matching it, because a substring
// match on the invocation is one level below the property that matters: it is not
// enough that `--corroborate --pr` APPEARS somewhere — its exit code must reach a
// job conclusion on a pull_request. So WHERE the invocation is present it asserts
// the four ways the gate can be present-but-dead, each a plausible one-line edit:
//   - the containing job is gated on pull_request (an `if: false`, or a flip to a
//     push-only condition, would mean the step never runs on a PR);
//   - the step carries no truthy `continue-on-error` (which would let its failure
//     pass the job);
//   - the run line does not swallow the status with `|| true` / `|| :`;
//   - the invocation binds the event's own PR number, not a frozen one.
func TestCorroborateIsWiredIntoStatusgenWorkflow(t *testing.T) {
	// Scan every workflow file rather than a single hardcoded name, so the test is
	// robust to how a repo names its CI.
	var files []string
	for _, pat := range []string{"*.yml", "*.yaml"} {
		m, _ := filepath.Glob(filepath.Join("..", ".github", "workflows", pat))
		files = append(files, m...)
	}
	if len(files) == 0 {
		t.Skip("no .github/workflows/*.yml files — cannot check corroboration wiring in this tree")
	}

	// Minimal shape of the parts of a GitHub Actions workflow this test reasons
	// about. Everything else in the file is ignored.
	type workflow struct {
		Jobs map[string]struct {
			If    string `yaml:"if"`
			Steps []struct {
				Name            string            `yaml:"name"`
				Run             string            `yaml:"run"`
				ContinueOnError any               `yaml:"continue-on-error"`
				Env             map[string]string `yaml:"env"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}

	// Matches `... --corroborate --pr ...` with any run-line spacing/prefix (e.g. a
	// leading `cd statusgen && `).
	invoke := regexp.MustCompile(`--corroborate\s+--pr\b`)

	var found bool
	for _, wfPath := range files {
		raw, err := os.ReadFile(wfPath)
		if err != nil {
			t.Fatalf("workflow not readable: %v", err)
		}
		var wf workflow
		if err := yaml.Unmarshal(raw, &wf); err != nil {
			// A workflow this test does not care about may use YAML shapes the
			// minimal struct above cannot decode; skip it rather than fail.
			continue
		}
		for jobName, job := range wf.Jobs {
			for _, step := range job.Steps {
				if !invoke.MatchString(step.Run) {
					continue
				}
				found = true

				// The step's exit code only reaches a conclusion if its job runs on a
				// pull_request. A bare `if: false`, or a flip to a push-only condition,
				// leaves the invocation present but never executed on a PR.
				if !strings.Contains(job.If, "pull_request") {
					t.Errorf("%s: the --corroborate step lives in job %q whose `if:` (%q) is not gated on "+
						"pull_request — the gate would never fire on a PR, so the anchor stays "+
						"self-issuable in CI", filepath.Base(wfPath), jobName, job.If)
				}

				// It must run against a PR number the event provides, not a hardcoded
				// one — a literal `--pr 223` would corroborate one frozen PR forever.
				if !prBoundToEvent(step.Run, step.Env) {
					t.Errorf("%s: `--corroborate` is present but not bound to github.event.pull_request.number — "+
						"it must corroborate THIS PR's diff, not a hardcoded PR", filepath.Base(wfPath))
				}

				// A truthy continue-on-error lets the step's failure pass the job — the
				// gate goes red for nobody.
				if isTruthyContinueOnError(step.ContinueOnError) {
					t.Errorf("%s: the --corroborate step sets continue-on-error: %v — a MISSING-CORROBORATION "+
						"would not reach the job conclusion, so the gate is decoration", filepath.Base(wfPath), step.ContinueOnError)
				}

				// A trailing `|| true` (or `|| :`) on the run line swallows the exit
				// status the same way.
				if strings.Contains(step.Run, "|| true") || strings.Contains(step.Run, "|| :") {
					t.Errorf("%s: the --corroborate run line swallows its exit status (`|| true` / `|| :`) — "+
						"a MISSING-CORROBORATION would not redden CI", filepath.Base(wfPath))
				}

				// The step needs the human-login map to resolve a stamped name to the
				// GitHub account whose action corroborates it (HumanLogin reads
				// ASSAY_HUMAN_LOGIN_MAP under CI); without it every risk-flagged name is
				// MISSING-CORROBORATION for the wrong reason.
				if step.Env["ASSAY_HUMAN_LOGIN_MAP"] == "" {
					t.Errorf("%s: the --corroborate step does not forward ASSAY_HUMAN_LOGIN_MAP — an unmapped "+
						"name would report MISSING-CORROBORATION for lack of config, not for lack of a real "+
						"human action", filepath.Base(wfPath))
				}
				if step.Env["GH_TOKEN"] == "" {
					t.Errorf("%s: the --corroborate step does not set GH_TOKEN — `gh pr diff`/`gh pr view` "+
						"cannot read the PR without it", filepath.Base(wfPath))
				}
			}
		}
	}

	if !found {
		t.Skip("no workflow invokes `--corroborate --pr <pr>` — the human-stamp corroboration " +
			"gate is not adopted in this repo's CI; nothing to pin")
	}
}

// eventPRNumber is the expression for the triggering PR's own number.
const eventPRNumber = "${{ github.event.pull_request.number }}"

var prFromEnv = regexp.MustCompile(`--pr\s+"?\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?"?(\s|$)`)

// prBoundToEvent reports whether a --corroborate run line passes the event's
// own PR number to --pr. Two forms bind it: the expression inline, or a shell
// variable the step's env sets to exactly that expression (the form GitHub
// recommends, since nothing is interpolated into the script). A variable the
// script itself assigns is refused, because the assignment could replace the
// event's number before the guard runs.
func prBoundToEvent(run string, env map[string]string) bool {
	if strings.Contains(run, "--pr "+eventPRNumber) {
		return true
	}
	m := prFromEnv.FindStringSubmatch(run)
	if m == nil || env[m[1]] != eventPRNumber {
		return false
	}
	reassigned := regexp.MustCompile(`(^|[\s;&|(])(export\s+|readonly\s+|declare\s+(-\S+\s+)*)?` + m[1] + `=`)
	return !reassigned.MatchString(run)
}

// TestPRBindingForms pins prBoundToEvent: both binding forms pass, and a frozen
// number, an env var bound to anything else, or a variable the script
// reassigns all fail.
func TestPRBindingForms(t *testing.T) {
	ev := map[string]string{"PR_NUMBER": eventPRNumber}
	cases := []struct {
		name string
		run  string
		env  map[string]string
		want bool
	}{
		{"inline expression", "statusgen --corroborate --pr " + eventPRNumber, nil, true},
		{"env quoted braces", `statusgen --corroborate --pr "${PR_NUMBER}"`, ev, true},
		{"env bare", "statusgen --corroborate --pr $PR_NUMBER", ev, true},
		{"frozen number", "statusgen --corroborate --pr 223", ev, false},
		{"env unset", `statusgen --corroborate --pr "${PR_NUMBER}"`, nil, false},
		{"env frozen", `statusgen --corroborate --pr "${PR_NUMBER}"`, map[string]string{"PR_NUMBER": "223"}, false},
		{"env other event field", `statusgen --corroborate --pr "${PR_NUMBER}"`, map[string]string{"PR_NUMBER": "${{ github.event.number }}x"}, false},
		{"script reassigns", "PR_NUMBER=223\nstatusgen --corroborate --pr \"${PR_NUMBER}\"", ev, false},
		{"script exports", "export PR_NUMBER=223; statusgen --corroborate --pr $PR_NUMBER", ev, false},
		{"other var named", `statusgen --corroborate --pr "${PR}"`, ev, false},
	}
	for _, c := range cases {
		if got := prBoundToEvent(c.run, c.env); got != c.want {
			t.Errorf("%s: prBoundToEvent = %v, want %v", c.name, got, c.want)
		}
	}
}

// isTruthyContinueOnError reports whether a step-level continue-on-error value
// disables the gate. yaml.v3 decodes a bare `true` as bool; an expression such as
// `${{ ... }}` decodes as a non-empty string, which we also treat as suspect
// because it can evaluate true at runtime. A missing key (nil) or an explicit
// false/empty is fine.
func isTruthyContinueOnError(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case string:
		s := strings.TrimSpace(t)
		return s != "" && s != "false"
	default:
		return true
	}
}
