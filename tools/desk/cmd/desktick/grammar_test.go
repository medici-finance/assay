package main

// grammar_test.go — the grammar's cases, and its equality with the bash oracle while the oracle is
// kept: `desktick regexp` prints the same string `tick-summary.sh regexp` prints, and every case
// below gets the same verdict (exit code AND reason) from both.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type tickCase struct {
	line string
	ok   bool
	why  string // a substring of the reason, for a rejected line
}

// cases mirrors plugins/assay/scripts/tick-summary.test.sh's corpus: the published examples, every
// role, every shape refusal, and every cross-field rule in both directions.
var cases = []tickCase{
	{"tick role=pr-review-desk outcome=ok swept=14 acted=2 filed=0 duration=612", true, ""},
	{"tick role=pr-review-desk outcome=noop swept=14 acted=0 filed=0 duration=47", true, ""},
	{"tick role=worker-desk outcome=could-not-check swept=- acted=0 filed=1 duration=39", true, ""},
	{"tick role=verify-desk outcome=refused swept=0 acted=0 filed=0 duration=8", true, ""},
	{"tick role=the-desk outcome=noop swept=0 acted=0 filed=0 duration=1", true, ""},
	{"tick role=intake-desk outcome=noop swept=0 acted=0 filed=0 duration=1", true, ""},
	{"tick role=the-desk outcome=could-not-check swept=- acted=- filed=- duration=3", true, ""},
	{"tick role=pr-review-desk outcome=ok swept=3 acted=1 filed=0 duration=12", true, ""},

	{"tick role=the-desk outcome=maybe swept=3 acted=1 filed=0 duration=12", false, "published grammar"},
	{"tick role=the-desk outcome=OK swept=3 acted=1 filed=0 duration=12", false, "published grammar"},
	{"tick role=the-desk outcome= swept=3 acted=1 filed=0 duration=12", false, "published grammar"},
	{"tick role=pr-shepherd outcome=noop swept=0 acted=0 filed=0 duration=2", false, "published grammar"},
	{"tick role=nope outcome=noop swept=0 acted=0 filed=0 duration=2", false, "published grammar"},
	{"tick role=the-desk outcome=noop acted=0 swept=0 filed=0 duration=2", false, "published grammar"},
	{"tick outcome=noop role=the-desk swept=0 acted=0 filed=0 duration=2", false, "published grammar"},
	{"role=the-desk outcome=noop swept=0 acted=0 filed=0 duration=2", false, "published grammar"},
	{"TICK role=the-desk outcome=noop swept=0 acted=0 filed=0 duration=2", false, "published grammar"},
	{"summary: tick role=the-desk outcome=noop swept=0 acted=0 filed=0 duration=2", false, "published grammar"},
	{"tick role=the-desk outcome=noop swept=0 acted=0 filed=0", false, "published grammar"},
	{"tick role=the-desk outcome=noop swept=0 acted=0 duration=2", false, "published grammar"},
	{"tick role=the-desk outcome=noop swept=0 acted=0 filed=0 duration=", false, "published grammar"},
	{"tick role=the-desk outcome=noop swept=0 acted=0 filed=0 duration=-", false, "published grammar"},
	{"tick role=the desk outcome=noop swept=0 acted=0 filed=0 duration=2", false, "published grammar"},
	{"tick role=the-desk outcome=could not check swept=- acted=0 filed=0 duration=2", false, "published grammar"},
	{"tick\trole=the-desk outcome=noop swept=0 acted=0 filed=0 duration=2", false, "contains a tab"},
	{"tick  role=the-desk outcome=noop swept=0 acted=0 filed=0 duration=2", false, "published grammar"},

	{"tick role=pr-review-desk outcome=could-not-check swept=0 acted=0 filed=0 duration=39", false, "requires swept=-"},
	{"tick role=the-desk outcome=could-not-check swept=7 acted=0 filed=0 duration=5", false, "requires swept=-"},
	{"tick role=pr-review-desk outcome=noop swept=- acted=0 filed=0 duration=47", false, "never noop"},
	{"tick role=pr-review-desk outcome=noop swept=3 acted=2 filed=0 duration=47", false, "requires acted=0"},
	{"tick role=pr-review-desk outcome=ok swept=- acted=2 filed=0 duration=47", false, "requires a numeric swept"},
	{"tick role=pr-review-desk outcome=ok swept=3 acted=0 filed=0 duration=12", false, "acted >= 1"},
	{"tick role=pr-review-desk outcome=ok swept=3 acted=- filed=0 duration=12", false, "acted >= 1"},
	{"tick role=the-desk outcome=refused swept=4 acted=0 filed=0 duration=2", false, "swept=0 and acted=0"},
	{"tick role=the-desk outcome=refused swept=0 acted=1 filed=0 duration=2", false, "swept=0 and acted=0"},
	{"tick role=the-desk outcome=refused swept=- acted=0 filed=0 duration=2", false, "swept=0 and acted=0"},
}

func TestGrammarCases(t *testing.T) {
	for _, c := range cases {
		err := Validate(c.line)
		if c.ok && err != nil {
			t.Errorf("rejected a valid line %q: %v", c.line, err)
		}
		if !c.ok {
			if err == nil {
				t.Errorf("accepted an invalid line %q", c.line)
			} else if !strings.Contains(err.Error(), c.why) {
				t.Errorf("%q: reason %q does not say %q", c.line, err.Error(), c.why)
			}
		}
	}
}

func TestCheckReadsTheLastNonBlankLine(t *testing.T) {
	good := "tick role=the-desk outcome=noop swept=0 acted=0 filed=0 duration=1"
	for in, ok := range map[string]bool{
		"working...\n" + good + "\n":            true,
		"working...\n" + good + "\n\n   \n":     true,
		good + "\nkept printing after the line": false,
		"":                                      false,
		"\n\n":                                  false,
	} {
		if err := Check(in); (err == nil) != ok {
			t.Errorf("Check(%q) = %v, want ok=%v", in, err, ok)
		}
	}
	if err := Check(""); err != errEmptyInput {
		t.Errorf("empty input must say the pass did not complete, got %v", err)
	}
}

func tickOracle(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("could-not-check: bash is not on PATH — the tick-summary.sh oracle cannot run on this runner")
	}
	p, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "plugins", "assay", "scripts", "tick-summary.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("the oracle is missing at %s: %v — it is kept until windows-port/14 retires it", p, err)
	}
	return p
}

// TestRegexpEqualsScript — the published regexp is ONE string across both implementations.
func TestRegexpEqualsScript(t *testing.T) {
	script := tickOracle(t)
	want, err := exec.Command("bash", script, "regexp").Output()
	if err != nil {
		t.Fatalf("tick-summary.sh regexp: %v", err)
	}
	var got bytes.Buffer
	if code := run([]string{"regexp"}, nil, &got, &bytes.Buffer{}); code != 0 {
		t.Fatalf("desktick regexp exited %d", code)
	}
	if got.String() != string(want) {
		t.Fatalf("the published regexp differs from the oracle's\n desktick: %q\n script:   %q", got.String(), string(want))
	}
}

// TestValidateParityWithScript — every case gets the same exit code AND the same reason from both.
func TestValidateParityWithScript(t *testing.T) {
	script := tickOracle(t)
	for _, c := range cases {
		cmd := exec.Command("bash", script, "validate", c.line)
		var serr bytes.Buffer
		cmd.Stderr = &serr
		scode := 0
		if err := cmd.Run(); err != nil {
			ee, ok := err.(*exec.ExitError)
			if !ok {
				t.Fatal(err)
			}
			scode = ee.ExitCode()
		}
		var verr bytes.Buffer
		vcode := run([]string{"validate", c.line}, nil, &bytes.Buffer{}, &verr)
		if scode != vcode {
			t.Errorf("%q: exit script=%d desktick=%d", c.line, scode, vcode)
			continue
		}
		s := strings.TrimPrefix(serr.String(), "tick-summary: ")
		v := strings.TrimPrefix(verr.String(), "desktick: ")
		if s != v {
			t.Errorf("%q: reason differs\n script:   %q\n desktick: %q", c.line, s, v)
		}
	}
}
