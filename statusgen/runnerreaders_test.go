package main

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// runnerreaders_test.go: the class #6767 closes is a second reader of a
// latest PASS run's Runner cells that judges them by its own rule, so
// verifyflip and `brief --check-verified` answer one Evidence differently.
// Every reader reaches those cells through flipDateRunnerRows; a site that
// judges whether the cells name one runner does it through runnerCellsAgree.
// This census pins the callers: a new one fails here until it is listed with
// what it reads, and a new judgement goes through runnerCellsAgree.

var runnerRowsCallRe = regexp.MustCompile(`\bflipDateRunnerRows\(`)

// runnerRowReaders is every non-test caller of flipDateRunnerRows, and why
// it does not judge runner agreement on its own.
var runnerRowReaders = map[string]string{
	"verifyflip.go":         "flipReadRun judges through runnerCellsAgree (check-verified reads its refusals, latestPassRunnerRefusal); flipReadLooseRun reads cells with no <login> @ <sha>; latestPassSpan counts rows after the verdict",
	"verifyissues.go":       "evidenceVerifierInfo reads row 0's Date only (flipReadRun accepted one date for the run); its runner is flipPass.Cell, the runnerCellsAgree reading, never a row's cell",
	"heldscope.go":          "reads Date cells and row presence; no Runner judgement",
	"coverage_crossrepo.go": "reads one qualifier per row (an uncommitted-source mark); no agreement judgement",
}

// runnerRowCallers returns the files in src (name → source) that call
// flipDateRunnerRows, the function's own file excepted only for its
// definition line.
func runnerRowCallers(src map[string]string) []string {
	var out []string
	for name, text := range src {
		n := len(runnerRowsCallRe.FindAllStringIndex(text, -1))
		if strings.Contains(text, "func flipDateRunnerRows(") {
			n--
		}
		if n > 0 {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// regression: #6767
func TestRunnerReadersCensus(t *testing.T) {
	// Positive control: a planted caller must be found.
	plant := map[string]string{"plant.go": "rows, _ := flipDateRunnerRows(x)\nif rows[0][1] != rows[1][1] {}"}
	if got := runnerRowCallers(plant); len(got) != 1 {
		t.Fatalf("the census missed a planted caller: %v", got)
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	src := map[string]string{}
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		b, err := os.ReadFile(n)
		if err != nil {
			t.Fatal(err)
		}
		src[n] = string(b)
	}
	found := runnerRowCallers(src)
	if len(found) < 3 {
		t.Fatalf("non-vacuity: the census found only %v", found)
	}
	seen := map[string]bool{}
	for _, f := range found {
		seen[f] = true
		if runnerRowReaders[f] == "" {
			t.Errorf("%s reads a run's Runner cells (flipDateRunnerRows) but is not in runnerRowReaders: "+
				"list what it reads, and judge runner agreement only through runnerCellsAgree (#6767)", f)
		}
	}
	for f := range runnerRowReaders {
		if !seen[f] {
			t.Errorf("%s is listed in runnerRowReaders but no longer calls flipDateRunnerRows", f)
		}
	}
}
