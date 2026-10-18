package main

// Tests for the board REGEN job's history precondition (#3364). The regen job
// runs the same checks as --lint, and some of them read git history: the
// consumed-changelog-fragment exemption (#722, linkcheck.go) and the Evidence
// staleness read. On a shallow checkout those checks are could-not-check by
// design (#999) and the fragment check still reports its PROBLEM, so a release
// that consumes a fragment a brief names turns the regen red on the default
// branch while the full-history PR lint passes. Every regen job a statusgen
// workflow carries must therefore check out full history, exactly as the lint
// job already must (scaffoldbase_test.go).

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// jobBlock returns the YAML block of the job whose key line is exactly
// header, up to the next line at the same or a shallower indent that is not a
// comment or blank; "" when the job is absent.
func jobBlock(doc, header string) string {
	i := strings.Index(doc, "\n"+header+"\n")
	if i < 0 {
		return ""
	}
	indent := len(header) - len(strings.TrimLeft(header, " "))
	rest := doc[i+1+len(header)+1:]
	var b strings.Builder
	b.WriteString(header + "\n")
	for _, line := range strings.SplitAfter(rest, "\n") {
		t := strings.TrimLeft(line, " ")
		lead := len(line) - len(t)
		if t != "" && t != "\n" && !strings.HasPrefix(t, "#") && lead <= indent {
			break
		}
		b.WriteString(line)
	}
	return b.String()
}

var fetchDepthZero = regexp.MustCompile(`(?m)^\s+fetch-depth:\s*0\s*(#.*)?$`)

// ghRegenFullHistory reports why a GitHub workflow's regen job would run
// statusgen on a shallow checkout, or "" when it would not.
func ghRegenFullHistory(wf string) string {
	job := jobBlock(wf, "  regen:")
	if job == "" {
		return "no regen job"
	}
	at := strings.Index(job, "- uses: actions/checkout@")
	if at < 0 {
		return "regen job has no actions/checkout step"
	}
	step := job[at+len("- uses: "):]
	if k := strings.Index(step, "\n      - "); k >= 0 {
		step = step[:k]
	}
	if !regexp.MustCompile(`(?m)^\s+with:\s*$`).MatchString(step) || !fetchDepthZero.MatchString(step) {
		return "regen job's actions/checkout step has no `with: fetch-depth: 0`"
	}
	if !strings.Contains(job[at:], "statusgen\" --root .") && !strings.Contains(job[at:], "statusgen --root .") {
		return "regen job runs no statusgen --root . after its checkout"
	}
	return ""
}

// glRegenFullHistory is the GitLab twin: the statusgen-regen job must turn
// shallow cloning off with a job-level GIT_DEPTH "0".
func glRegenFullHistory(gl string) string {
	job := jobBlock(gl, "statusgen-regen:")
	if job == "" {
		return "no statusgen-regen job"
	}
	if !regexp.MustCompile(`(?m)^  variables:\s*$`).MatchString(job) ||
		!regexp.MustCompile(`(?m)^    GIT_DEPTH:\s*"0"\s*$`).MatchString(job) {
		return `statusgen-regen has no job-level variables: GIT_DEPTH: "0"`
	}
	if !strings.Contains(job, "- statusgen --root .") {
		return "statusgen-regen runs no statusgen --root ."
	}
	return ""
}

// regression: #3364
func TestInitRegenFullHistory(t *testing.T) {
	if why := ghRegenFullHistory(initWorkflow); why != "" {
		t.Errorf("statusgen init's GitHub workflow: %s; a shallow regen reads a consumed changelog fragment as a missing path and reds the default branch", why)
	}
}

// regression: #3364
func TestGitLabRegenFullHistory(t *testing.T) {
	if why := glRegenFullHistory(initGitlabCI); why != "" {
		t.Errorf("statusgen init's .gitlab-ci.yml: %s; a shallow regen reads a consumed changelog fragment as a missing path and reds the default branch", why)
	}
}

// regenWorkflowCopies are this repository's own board workflow and the copies
// a maintainer promotes over it. The live file carries the full-history
// checkout, and a promotion from either copy keeps it.
var regenWorkflowCopies = []string{
	"../.github/workflows/assay-statusgen.yml",
	"../ci/staged-workflows/assay-statusgen.yml",
	"../tools/ci-load/activation/assay-statusgen.yml",
}

// regression: #3364
func TestBoardRegenFullHistory(t *testing.T) {
	for _, p := range regenWorkflowCopies {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if why := ghRegenFullHistory(string(b)); why != "" {
			t.Errorf("%s: %s", p, why)
		}
	}
}

// TestRegenChecksRejectShallow is the negative control: with the full-history
// setting removed, both checks must refuse, so a matcher that silently stops
// matching cannot report clean.
func TestRegenChecksRejectShallow(t *testing.T) {
	gh := fetchDepthZero.ReplaceAllString(jobBlock(initWorkflow, "  regen:"), "          fetch-depth: 1")
	if gh == jobBlock(initWorkflow, "  regen:") {
		t.Fatal("initWorkflow's regen job carries no fetch-depth: 0 to remove")
	}
	if ghRegenFullHistory("jobs:\n"+gh) == "" {
		t.Error("ghRegenFullHistory passed a regen job with a shallow checkout")
	}
	if ghRegenFullHistory("jobs:\n  regen:\n    steps:\n      - uses: actions/checkout@v4\n      - run: statusgen --root .\n") == "" {
		t.Error("ghRegenFullHistory passed a regen job with a default checkout")
	}
	gl := regexp.MustCompile(`(?m)^    GIT_DEPTH:.*\n`).ReplaceAllString(jobBlock(initGitlabCI, "statusgen-regen:"), "")
	if glRegenFullHistory("\n"+gl) == "" {
		t.Error("glRegenFullHistory passed a statusgen-regen job with no GIT_DEPTH")
	}
	// The lint job's GIT_DEPTH must not satisfy the regen check.
	lintOnly := "\nstatusgen-lint:\n  variables:\n    GIT_DEPTH: \"0\"\n  script:\n    - statusgen --lint\n\nstatusgen-regen:\n  script:\n    - statusgen --root .\n"
	if glRegenFullHistory(lintOnly) == "" {
		t.Error("glRegenFullHistory credited the lint job's GIT_DEPTH to the regen job")
	}
}
