package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/medici-finance/assay/deskcore/issueflow"
)

// handoffReferencePath is the intake handoff procedure reference whose
// examples must stay in step with the shipped assessment.
const handoffReferencePath = "../../../../plugins/assay/references/intake-handoff.md"

// handoffExample is one `expect:`-labelled json fence of the reference.
type handoffExample struct {
	line   int    // 1-based line of the expect label
	expect string // "<state> <route>[ direct]"
	body   string
}

var handoffExpectLine = regexp.MustCompile("^expect: (\\S+) (\\S+)( direct)?$")

// handoffExamples returns every `expect:` label of doc with the json fence that
// follows it. A label with no fence is itself reported.
func handoffExamples(doc string) ([]handoffExample, []string) {
	var out []handoffExample
	var problems []string
	lines := strings.Split(doc, "\n")
	for i := 0; i < len(lines); i++ {
		if !strings.HasPrefix(lines[i], "expect:") {
			continue
		}
		ex := handoffExample{line: i + 1, expect: strings.TrimPrefix(lines[i], "expect: ")}
		if !handoffExpectLine.MatchString(lines[i]) {
			problems = append(problems, fmt.Sprintf("line %d: malformed expect label %q", ex.line, lines[i]))
			continue
		}
		j := i + 1
		for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
			j++
		}
		if j >= len(lines) || strings.TrimSpace(lines[j]) != "```json" {
			problems = append(problems, fmt.Sprintf("line %d: expect label is not followed by a json fence", ex.line))
			continue
		}
		var body []string
		k := j + 1
		for k < len(lines) && strings.TrimSpace(lines[k]) != "```" {
			body = append(body, lines[k])
			k++
		}
		if k >= len(lines) {
			problems = append(problems, fmt.Sprintf("line %d: json fence is not closed", ex.line))
			continue
		}
		ex.body = strings.Join(body, "\n")
		out = append(out, ex)
		i = k
	}
	return out, problems
}

// checkHandoffExamples decodes each example strictly and compares the shipped
// assessment of it with its label. It returns one message per disagreement.
func checkHandoffExamples(doc string) (int, []string) {
	examples, problems := handoffExamples(doc)
	for _, ex := range examples {
		h, err := issueflow.DecodeIntakeHandoff([]byte(ex.body))
		if err != nil {
			problems = append(problems, fmt.Sprintf("line %d: example does not decode: %v", ex.line, err))
			continue
		}
		a := issueflow.AssessIntakeHandoff(h, issueflow.IntakeFacts{Item: h.Item, Opened: h.Opened, Complete: true})
		got := a.State + " " + a.Route
		if a.DirectRepair {
			got += " direct"
		}
		if got != ex.expect {
			problems = append(problems, fmt.Sprintf("line %d: example is labelled %q but assesses as %q (%s)", ex.line, ex.expect, got, a.Reason))
		}
	}
	return len(examples), problems
}

// TestHandoffProcedureExamples keeps every record the reference teaches in step
// with the shipped assessment, and requires one example per judgment row.
func TestHandoffProcedureExamples(t *testing.T) {
	raw, err := os.ReadFile(handoffReferencePath)
	if err != nil {
		t.Fatal(err)
	}
	n, problems := checkHandoffExamples(string(raw))
	for _, p := range problems {
		t.Error(p)
	}
	if n != 6 {
		t.Fatalf("reference carries %d labelled examples, want one per judgment row (6)", n)
	}
	examples, _ := handoffExamples(string(raw))
	seen := map[string]bool{}
	for _, ex := range examples {
		h, err := issueflow.DecodeIntakeHandoff([]byte(ex.body))
		if err != nil {
			continue
		}
		seen[fmt.Sprintf("%s/%s/%t", h.Disposition, h.Route, h.Delivered)] = true
	}
	for _, want := range []string{"repair/repair/false", "duplicate/duplicate/false", "needs-scoping/authoring/false", "external-prerequisite/external/false", "repair/qualification/true"} {
		if !seen[want] {
			t.Errorf("no example covers judgment %s", want)
		}
	}
}

// TestHandoffExamplesFlagPlant proves the check above fails on a reference whose
// example disagrees with the shipped assessment, so a green run is not vacuous.
func TestHandoffExamplesFlagPlant(t *testing.T) {
	raw, err := os.ReadFile("testdata/handoff-procedure-plant.md")
	if err != nil {
		t.Fatal(err)
	}
	n, problems := checkHandoffExamples(string(raw))
	if n != 3 {
		t.Fatalf("plant carries %d examples, want 3", n)
	}
	if len(problems) != 3 {
		t.Fatalf("plant produced %d problems, want 3 (mislabel, unknown field, unsettled direct repair): %v", len(problems), problems)
	}
}
