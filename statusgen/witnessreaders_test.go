package main

// witnessreaders_test.go — statusgen/36 Verify row 13: every non-test Go file
// in statusgen and tools/desk that reads a Runner cell, a witness record, a
// verify-outcome record or a claim's revision is listed in
// docs/witness-readers.md with a disposition, so a reader that keeps its own
// interpretation of the v2 schema cannot go unnoticed.

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

const witnessReadersDoc = "../docs/witness-readers.md"

// witnessCensusPattern is the census grep, mirrored in the doc's prose.
var witnessCensusPattern = regexp.MustCompile(`Runner cell|runnerCell|\.Runner\b|"Runner"|witnessCells|witnessTreeOf|ParseRunnerCell|ParseWitnessRecord|WitnessRecord|witness_schema_version|WitnessCell|verify-outcome|VerifyOutcome|OutcomeRecord|outcome-record|outcomerecord|Claim\.Revision|\bcl\.Revision|claim\.Revision|coverageClaimJSON|evaluation\.Claim\b`)

var witnessReaderDispositions = map[string]bool{
	"judges through the library":      true,
	"reads informational fields only": true,
	"migrates in statusgen/37":        true,
}

// censusFiles returns the repo-relative files whose source matches the census
// pattern. The scan covers every non-test Go file, including the library.
func censusFiles(files map[string]string) []string {
	var out []string
	for p, src := range files {
		if witnessCensusPattern.MatchString(src) {
			out = append(out, p)
		}
	}
	return out
}

// repoCensusSources is repoWitnessSources plus the library.
func repoCensusSources(t *testing.T) map[string]string {
	t.Helper()
	files := repoWitnessSources(t)
	entries, err := os.ReadDir("evaluation")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		b, err := os.ReadFile("evaluation/" + n)
		if err != nil {
			t.Fatal(err)
		}
		files["statusgen/evaluation/"+n] = string(b)
	}
	return files
}

// parseReadersDoc returns file → disposition from the doc's table.
func parseReadersDoc(t *testing.T, text string) map[string]string {
	t.Helper()
	listed := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "| "), "|")
		if len(cells) < 3 {
			t.Fatalf("malformed census row %q", line)
		}
		file := strings.Trim(strings.TrimSpace(cells[0]), "`")
		disp := strings.TrimSpace(cells[1])
		if !witnessReaderDispositions[disp] {
			t.Errorf("%s: disposition %q is not one of the three allowed", file, disp)
		}
		if strings.TrimSpace(cells[2]) == "" {
			t.Errorf("%s: a disposition needs a note", file)
		}
		if _, dup := listed[file]; dup {
			t.Errorf("%s is listed twice", file)
		}
		listed[file] = disp
	}
	return listed
}

// TestWitnessReaderCensus: the doc lists exactly the files the census grep
// finds, and each exists.
// regression: #2026
func TestWitnessReaderCensus(t *testing.T) {
	raw, err := os.ReadFile(witnessReadersDoc)
	if err != nil {
		t.Fatal(err)
	}
	listed := parseReadersDoc(t, string(raw))
	files := repoCensusSources(t)
	found := censusFiles(files)
	if len(found) < 40 {
		t.Fatalf("non-vacuity: the census found only %d files", len(found))
	}
	foundSet := map[string]bool{}
	for _, f := range found {
		foundSet[f] = true
		if _, ok := listed[f]; !ok {
			t.Errorf("%s matches the census grep but is not listed in docs/witness-readers.md — give it a disposition", f)
		}
	}
	for f := range listed {
		if _, err := os.Stat("../" + f); err != nil {
			t.Errorf("%s is listed but does not exist", f)
		} else if !foundSet[f] {
			t.Errorf("%s is listed but no longer matches the census grep — remove the row", f)
		}
	}
}

// TestWitnessReaderCensusFlagsPlant: a file with a Runner-cell reader that is
// absent from the doc is reported.
// regression: #2026
func TestWitnessReaderCensusFlagsPlant(t *testing.T) {
	files := map[string]string{
		"statusgen/plant.go": "package main\n\nfunc f(er EvidenceRow) string { return er.Runner }\n",
		"statusgen/clean.go": "package main\n\nfunc g() int { return 1 }\n",
	}
	got := censusFiles(files)
	if len(got) != 1 || got[0] != "statusgen/plant.go" {
		t.Fatalf("census over the plant: %v", got)
	}
	listed := parseReadersDoc(t, "| `statusgen/other.go` | migrates in statusgen/37 | x |\n")
	if _, ok := listed[got[0]]; ok {
		t.Fatal("the plant must be unlisted")
	}
}
