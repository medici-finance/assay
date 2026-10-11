package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Gremlins v0.6.0 report statuses. TIMED OUT has no top-level count in the
// report, so every count here is taken from the per-file list instead.
const (
	stKilled     = "KILLED"
	stLived      = "LIVED"
	stNotCovered = "NOT COVERED"
	stTimedOut   = "TIMED OUT"
	stNotViable  = "NOT VIABLE"
	stSkipped    = "SKIPPED"
	stRunnable   = "RUNNABLE"
)

var knownStatus = map[string]bool{
	stKilled: true, stLived: true, stNotCovered: true, stTimedOut: true,
	stNotViable: true, stSkipped: true, stRunnable: true,
}

// mutant is one Gremlins mutation result. File is relative to the package dir,
// as Gremlins reports it.
type mutant struct {
	File   string
	Type   string
	Status string
	Line   int
	Column int
}

type reportFile struct {
	FileName  string `json:"file_name"`
	Mutations []struct {
		Type   string `json:"type"`
		Status string `json:"status"`
		Line   int    `json:"line"`
		Column int    `json:"column"`
	} `json:"mutations"`
}

type gremlinsReport struct {
	GoModule *string      `json:"go_module"`
	Files    []reportFile `json:"files"`
}

// readReport parses a Gremlins JSON report into a flat mutant list. A report
// without go_module is not a Gremlins report; an unknown status means the
// report shape changed under us. Both are errors, never an empty result.
func readReport(path string) ([]mutant, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r gremlinsReport
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("%s: %v", path, err)
	}
	if r.GoModule == nil {
		return nil, fmt.Errorf("%s: no go_module field — not a Gremlins report", path)
	}
	var out []mutant
	for _, f := range r.Files {
		for _, m := range f.Mutations {
			if !knownStatus[m.Status] {
				return nil, fmt.Errorf("%s: %s:%d: unknown status %q", path, f.FileName, m.Line, m.Status)
			}
			out = append(out, mutant{File: f.FileName, Type: m.Type, Status: m.Status, Line: m.Line, Column: m.Column})
		}
	}
	return out, nil
}

var (
	ansiRE    = regexp.MustCompile("\x1b\\[[0-9;]*m")
	logLineRE = regexp.MustCompile(`^\s*(KILLED|LIVED|NOT COVERED|TIMED OUT|NOT VIABLE|SKIPPED)\s+([A-Z_]+) at (.+):(\d+):(\d+)\s*$`)
)

// readLog recovers the mutants that completed before a run was interrupted.
// Gremlins writes no JSON report on SIGINT, but prints one line per finished
// mutant, so a budget-stopped run is still partial evidence.
func readLog(r io.Reader) ([]mutant, error) {
	var out []mutant
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		m := logLineRE.FindStringSubmatch(ansiRE.ReplaceAllString(sc.Text(), ""))
		if m == nil {
			continue
		}
		line, _ := strconv.Atoi(m[4])
		col, _ := strconv.Atoi(m[5])
		out = append(out, mutant{File: strings.TrimSpace(m[3]), Type: m[2], Status: m[1], Line: line, Column: col})
	}
	return out, sc.Err()
}

// counts is one package's tally.
type counts struct {
	Killed     int `json:"killed"`
	Lived      int `json:"lived"`
	NotCovered int `json:"not_covered"`
	TimedOut   int `json:"timed_out"`
	NotViable  int `json:"not_viable"`
	Skipped    int `json:"skipped"`
}

func tally(ms []mutant) counts {
	var c counts
	for _, m := range ms {
		switch m.Status {
		case stKilled:
			c.Killed++
		case stLived:
			c.Lived++
		case stNotCovered:
			c.NotCovered++
		case stTimedOut:
			c.TimedOut++
		case stNotViable:
			c.NotViable++
		case stSkipped:
			c.Skipped++
		}
	}
	return c
}

// efficacy is caught over caught+lived, where caught counts timed-out mutants
// too: a mutant that hangs the suite is noticed, so it is not a survivor.
func (c counts) efficacy() (float64, bool) {
	caught := c.Killed + c.TimedOut
	if caught+c.Lived == 0 {
		return 0, false
	}
	return 100 * float64(caught) / float64(caught+c.Lived), true
}
