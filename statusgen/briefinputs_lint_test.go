package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// regression: #5684 — row 15 checks the inputs parser and its lint adapter
// against the brief template and witness fixtures, independently of protected
// acceptance records elsewhere in the repository.
func TestBriefInputsLint(t *testing.T) {
	template, err := os.ReadFile("../docs/brief-template.md")
	if err != nil {
		t.Fatal(err)
	}
	field := regexp.MustCompile("(?m)^inputs: <OPTIONAL[^\n]*$")
	if len(field.FindAll(template, -1)) != 1 {
		t.Fatal("brief template must expose one optional inputs field")
	}
	context := extractSectionByPrefix(field.ReplaceAllString(string(template), "inputs: `tools/x/pkg/`"), "Context")
	entries, found, problem := evaluation.ParseInputsDeclaration(context)
	if !found || problem != "" || strings.Join(entries, ",") != "tools/x/pkg/" {
		t.Fatalf("template inputs: %v, found=%v, problem=%q", entries, found, problem)
	}

	var fixtures []string
	err = filepath.WalkDir("testdata", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "brief-") && strings.HasSuffix(entry.Name(), ".md") {
			fixtures = append(fixtures, path)
		}
		return nil
	})
	if err != nil || len(fixtures) == 0 {
		t.Fatalf("fixture briefs: %v, %v", fixtures, err)
	}
	for _, path := range fixtures {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		_, _, problem := evaluation.ParseInputsDeclaration(extractSectionByPrefix(string(body), "Context"))
		if problem != "" {
			t.Errorf("%s: %s", path, problem)
		}
	}

	f := newGitV2Fixture(t).build()
	if problems := witnessInputsProblems(f.Root, gv2StreamOf(f)); len(problems) != 0 {
		t.Fatalf("valid fixture inputs: %v", problems)
	}
	body, err := os.ReadFile(f.BriefPath)
	if err != nil {
		t.Fatal(err)
	}
	broken := strings.Replace(string(body), "inputs: "+f.Inputs, "inputs: `tools/**`", 1)
	if broken == string(body) {
		t.Fatal("fixture inputs field was absent")
	}
	relative, err := filepath.Rel(f.Root, f.BriefPath)
	if err != nil {
		t.Fatal(err)
	}
	gv2Write(t, f.Root, relative, broken)
	if problems := witnessInputsProblems(f.Root, gv2StreamOf(f)); len(problems) != 1 || !strings.Contains(problems[0], "inputs-undeclared") {
		t.Fatalf("malformed fixture inputs: %v", problems)
	}
}
