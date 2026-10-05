package main

// Tests for the scaffolded CI's base-ref precondition (statusgen/06, review
// finding sg06-scaffold-shallow-lint-red). Since the findings-register guard
// fails closed when the exact ref refs/remotes/origin/main does not resolve,
// every `statusgen --lint` job that `statusgen init` scaffolds must check out
// enough history for that ref to exist. A default shallow checkout has no such
// ref, so an adopter's lint would go red on every PR once its first finding is
// recorded, whether or not the PR touches the register.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// section returns tmpl from the first line equal to start up to (not
// including) the first later line equal to end; "" when start is absent.
func section(tmpl, start, end string) string {
	i := strings.Index(tmpl, "\n"+start+"\n")
	if i < 0 {
		return ""
	}
	rest := tmpl[i+1:]
	if j := strings.Index(rest, "\n"+end+"\n"); j >= 0 {
		return rest[:j+1]
	}
	return rest
}

// ghLintFetchesBase reports why the GitHub workflow's lint job would run
// statusgen --lint without refs/remotes/origin/main, or "" when it would not.
// actions/checkout fetches every branch into refs/remotes/origin/* only with
// fetch-depth: 0; its default is one commit of the PR merge ref alone.
func ghLintFetchesBase(wf string) string {
	job := section(wf, "  lint:", "  regen:")
	if job == "" {
		return "no lint job"
	}
	at := strings.Index(job, "- uses: actions/checkout@")
	if at < 0 {
		return "lint job has no actions/checkout step"
	}
	step := job[at+len("- uses: "):]
	if k := strings.Index(step, "\n      - "); k >= 0 {
		step = step[:k]
	}
	if !regexp.MustCompile(`(?m)^\s+with:\s*$`).MatchString(step) ||
		!regexp.MustCompile(`(?m)^\s+fetch-depth:\s*0\s*(#.*)?$`).MatchString(step) {
		return "lint job's actions/checkout step has no `with: fetch-depth: 0`"
	}
	lint := strings.Index(job, "run: statusgen --lint")
	if lint < 0 || lint < at {
		return "lint job runs no statusgen --lint after its checkout"
	}
	return ""
}

// glLintFetchesBase is the GitLab twin. A merge-request pipeline is a shallow
// clone (GIT_DEPTH) that fetches the pipeline ref only, so the job must turn
// shallow cloning off and fetch main into refs/remotes/origin/main before it
// runs statusgen --lint.
func glLintFetchesBase(gl string) string {
	job := section(gl, "statusgen-lint:", "statusgen-regen:")
	if job == "" {
		return "no statusgen-lint job"
	}
	if !regexp.MustCompile(`(?m)^  variables:\s*$`).MatchString(job) ||
		!regexp.MustCompile(`(?m)^    GIT_DEPTH:\s*"0"\s*$`).MatchString(job) {
		return `statusgen-lint has no job-level variables: GIT_DEPTH: "0"`
	}
	fetch := strings.Index(job, `git fetch --no-tags origin "+refs/heads/main:refs/remotes/origin/main"`)
	lint := strings.Index(job, "- statusgen --lint")
	if fetch < 0 {
		return "statusgen-lint does not fetch main into refs/remotes/origin/main"
	}
	if lint < 0 || fetch > lint {
		return "statusgen-lint fetches main after (or without) its statusgen --lint"
	}
	return ""
}

// scaffoldLintChecks names every scaffolded CI template that runs statusgen
// --lint and the check that proves its lint job resolves the base.
var scaffoldLintChecks = map[string]func(string) string{
	"initWorkflow": ghLintFetchesBase,
	"initGitlabCI": glLintFetchesBase,
}

func TestInitLintFetchesBase(t *testing.T) {
	if why := ghLintFetchesBase(initWorkflow); why != "" {
		t.Errorf("statusgen init's GitHub workflow: %s; a default shallow checkout has no refs/remotes/origin/main, so --lint fails closed on every PR once a finding exists", why)
	}
}

func TestGitLabLintFetchesBase(t *testing.T) {
	if why := glLintFetchesBase(initGitlabCI); why != "" {
		t.Errorf("statusgen init's .gitlab-ci.yml: %s; a merge-request pipeline has no refs/remotes/origin/main, so --lint fails closed on every MR once a finding exists", why)
	}
}

// TestLintChecksRejectShallow is the negative control for both checks: the
// shapes this finding was raised against (no fetch-depth; no GIT_DEPTH and no
// fetch) must be refused, so a check that stops matching cannot pass.
func TestLintChecksRejectShallow(t *testing.T) {
	gh := strings.Replace(initWorkflow, "        with:\n          fetch-depth: 0\n", "", 1)
	if gh == initWorkflow {
		t.Fatal("initWorkflow carries no `with: fetch-depth: 0` block to remove")
	}
	if ghLintFetchesBase(gh) == "" {
		t.Error("ghLintFetchesBase passed a lint job with a default (shallow) checkout")
	}
	gl := regexp.MustCompile(`(?m)^    GIT_DEPTH:.*\n`).ReplaceAllString(initGitlabCI, "")
	if glLintFetchesBase(gl) == "" {
		t.Error("glLintFetchesBase passed a lint job with no GIT_DEPTH")
	}
	gl = strings.Replace(initGitlabCI, `git fetch --no-tags origin "+refs/heads/main:refs/remotes/origin/main"`, "true", 1)
	if glLintFetchesBase(gl) == "" {
		t.Error("glLintFetchesBase passed a lint job that never fetches main")
	}
}

var lintRunLine = regexp.MustCompile(`(?m)^\s*(?:-\s+)?(?:run:\s*)?statusgen\s[^\n]*--lint`)

// lintTemplateConsts returns the name of every package-level const or var in
// f whose string value carries a CI line that runs statusgen --lint.
func lintTemplateConsts(f *ast.File) []string {
	var names []string
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || (g.Tok != token.CONST && g.Tok != token.VAR) {
			continue
		}
		for _, s := range g.Specs {
			vs, ok := s.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, v := range vs.Values {
				var b strings.Builder
				ast.Inspect(v, func(n ast.Node) bool {
					if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						if u, err := strconv.Unquote(lit.Value); err == nil {
							b.WriteString(u)
						}
					}
					return true
				})
				if i < len(vs.Names) && lintRunLine.MatchString(b.String()) {
					names = append(names, vs.Names[i].Name)
				}
			}
		}
	}
	return names
}

// TestScaffoldLintJobsGuarded is the class guard: every template in the
// statusgen sources that runs statusgen --lint must be in scaffoldLintChecks,
// so a new scaffolded lint job cannot ship without a base-ref check. The
// planted source is the positive control.
func TestScaffoldLintJobsGuarded(t *testing.T) {
	fset := token.NewFileSet()
	plant, err := parser.ParseFile(fset, "plant.go",
		"package p\nconst initPlanted = \"jobs:\\n  lint:\\n    steps:\\n      - uses: actions/checkout@v4\\n      - run: statusgen --lint\\n\"\n", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := lintTemplateConsts(plant); len(got) != 1 || got[0] != "initPlanted" {
		t.Fatalf("positive control: the planted lint template must be found once; got %v", got)
	}
	matches, err := filepath.Glob("*.go")
	if err != nil || len(matches) == 0 {
		t.Fatalf("no statusgen sources found (err %v)", err)
	}
	var found []string
	for _, m := range matches {
		if strings.HasSuffix(m, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, m, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", m, err)
		}
		for _, n := range lintTemplateConsts(f) {
			found = append(found, n)
			if _, ok := scaffoldLintChecks[n]; !ok {
				t.Errorf("%s: %s runs statusgen --lint but has no base-ref check in scaffoldLintChecks", m, n)
			}
		}
	}
	sort.Strings(found)
	for n := range scaffoldLintChecks {
		if i := sort.SearchStrings(found, n); i >= len(found) || found[i] != n {
			t.Errorf("scaffoldLintChecks names %s, but the guard found no lint template by that name (found %v)", n, found)
		}
	}
}
