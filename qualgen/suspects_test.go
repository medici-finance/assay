package main

import (
	"regexp"
	"testing"
)

func TestFingerprint_StableAndDistinct(t *testing.T) {
	a := Fingerprint("dead-code", "pkg/x.go", 10, 12)
	b := Fingerprint("dead-code", "pkg/x.go", 10, 12)
	if a != b {
		t.Errorf("fingerprint not stable: %s != %s", a, b)
	}
	// A different category, path, or line-window is a different suspect.
	for _, other := range []string{
		Fingerprint("duplication", "pkg/x.go", 10, 12),
		Fingerprint("dead-code", "pkg/y.go", 10, 12),
		Fingerprint("dead-code", "pkg/x.go", 11, 12),
	} {
		if other == a {
			t.Errorf("fingerprint collision with %s", other)
		}
	}
}

func TestRunCategory_NoTool_CouldNotMeasure(t *testing.T) {
	res := runCategory(".", "dead-code", ToolConfig{}, cannedRunner(map[string]string{}))
	if res.State.State != StateCouldNotMeasure {
		t.Errorf("no-tool category is %q, want could-not-measure", res.State.State)
	}
	if len(res.Suspects) != 0 {
		t.Errorf("no-tool category nominated %d suspects", len(res.Suspects))
	}
}

func TestRunCategory_ToolNotInstalled_CouldNotMeasure(t *testing.T) {
	// cannedRunner reports a not-installed tool for an unmapped category.
	res := runCategory(".", "dead-code", ToolConfig{Command: []string{"canned", "dead-code"}}, cannedRunner(map[string]string{}))
	if res.State.State != StateCouldNotMeasure {
		t.Errorf("not-installed tool is %q, want could-not-measure", res.State.State)
	}
}

func TestRunCategory_RanButEmpty_MeasuredZero(t *testing.T) {
	res := runCategory(".", "dead-code", ToolConfig{Command: []string{"canned", "dead-code"}},
		cannedRunner(map[string]string{"dead-code": "\n  \n"}))
	if res.State.State != StateMeasuredZero {
		t.Errorf("tool that flagged nothing is %q, want measured-zero", res.State.State)
	}
}

func TestParseSuspects_NormalizesDiagnosticLines(t *testing.T) {
	re := regexp.MustCompile(defaultSuspectPattern)
	out := "pkg/a.go:10:6: func f is unused (U1000)\nsummary: 1 issue\npkg/b.go:3: bad\n"
	tc := ToolConfig{Command: []string{"lint"}, Rule: "U1000"}
	suspects, err := parseSuspects("dead-code", tc, re, out)
	if err != nil {
		t.Fatalf("parseSuspects: %v", err)
	}
	// The "summary:" line does not match file:line and is skipped.
	if len(suspects) != 2 {
		t.Fatalf("want 2 suspects, got %d: %+v", len(suspects), suspects)
	}
	if suspects[0].File != "pkg/a.go" || suspects[0].LineStart != 10 {
		t.Errorf("first suspect mis-parsed: %+v", suspects[0])
	}
	if suspects[0].Tool != "lint" || suspects[0].Rule != "U1000" {
		t.Errorf("tool/rule not carried: %+v", suspects[0])
	}
	if suspects[0].RawEvidence == "" {
		t.Errorf("raw evidence dropped")
	}
}
