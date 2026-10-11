package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
)

// Run states run.sh records per package in the manifest.
const (
	stateOK         = "ok"             // Gremlins finished and wrote its JSON report
	statePartial    = "partial"        // the budget ran out; finished mutants come from the log
	stateFailed     = "failed"         // Gremlins exited non-zero on its own
	stateProbeRed   = "probe-red"      // the package's tests fail in the mutant copy layout
	stateDryRunFail = "dry-run-failed" // the dry run or plan step failed
)

// manifestRow is one tab-separated manifest line:
// pkg, state, wall seconds, exit code, report path, log path, bin.
type manifestRow struct {
	Pkg, State string
	Wall       float64
	Exit       int
	Report     string
	Log        string
	Bin        string
}

func readManifest(r io.Reader) ([]manifestRow, error) {
	var rows []manifestRow
	sc := bufio.NewScanner(r)
	n := 0
	for sc.Scan() {
		n++
		line := sc.Text()
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 7 {
			return nil, fmt.Errorf("manifest line %d: want 7 tab-separated fields, got %d", n, len(f))
		}
		wall, err := strconv.ParseFloat(f[2], 64)
		if err != nil {
			return nil, fmt.Errorf("manifest line %d: wall %q: %v", n, f[2], err)
		}
		code, err := strconv.Atoi(f[3])
		if err != nil {
			return nil, fmt.Errorf("manifest line %d: exit %q: %v", n, f[3], err)
		}
		switch f[1] {
		case stateOK, statePartial, stateFailed, stateProbeRed, stateDryRunFail:
		default:
			return nil, fmt.Errorf("manifest line %d: unknown state %q", n, f[1])
		}
		rows = append(rows, manifestRow{Pkg: f[0], State: f[1], Wall: wall, Exit: code, Report: f[4], Log: f[5], Bin: f[6]})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("manifest holds no packages")
	}
	return rows, nil
}

// candidate is a LIVED mutant in a function no muhar spec mutates.
type candidate struct {
	Pkg         string `json:"pkg"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Column      int    `json:"column"`
	Func        string `json:"func"`
	Mutator     string `json:"mutator"`
	Source      string `json:"source"`
	FileSpecced bool   `json:"file_specced"`
}

type pkgResult struct {
	Pkg        string      `json:"pkg"`
	State      string      `json:"state"`
	Bin        string      `json:"bin"`
	Wall       float64     `json:"wall_seconds"`
	Counts     counts      `json:"counts"`
	SpecLine   int         `json:"lived_spec_line"`
	SpecFunc   int         `json:"lived_spec_func"`
	Candidates []candidate `json:"candidates"`
	Broke      string      `json:"broke,omitempty"`
	Note       string      `json:"note,omitempty"`
}

type triageResult struct {
	Schema   string       `json:"schema"`
	Specs    int          `json:"specs"`
	Packages []pkgResult  `json:"packages"`
	Stale    []staleEntry `json:"stale_spec_entries"`
	Broke    bool         `json:"broke"`
}

// triage turns the manifest into results. A package "broke" when its evidence
// cannot be trusted; running out of budget is not breaking, it is partial.
func triage(rows []manifestRow, idx *specIndex) triageResult {
	res := triageResult{Schema: "gremlins-triage/v1", Specs: len(idx.Specs), Stale: idx.Stale}
	evidence := 0
	for _, row := range rows {
		pr := pkgResult{Pkg: row.Pkg, State: row.State, Bin: row.Bin, Wall: row.Wall}
		var ms []mutant
		switch row.State {
		case stateOK:
			var err error
			if ms, err = readReport(row.Report); err != nil {
				pr.Broke = "report unreadable: " + err.Error()
			}
		case statePartial:
			f, err := os.Open(row.Log)
			if err != nil {
				pr.Broke = "log unreadable: " + err.Error()
				break
			}
			ms, err = readLog(f)
			f.Close()
			if err != nil {
				pr.Broke = "log unreadable: " + err.Error()
			}
			pr.Note = "budget ran out; counts are the mutants that finished"
		case stateFailed:
			pr.Broke = fmt.Sprintf("gremlins exited %d", row.Exit)
		case stateProbeRed:
			pr.Broke = "package tests fail in the mutant copy layout; kills would be false"
		case stateDryRunFail:
			pr.Broke = fmt.Sprintf("dry run or plan failed (exit %d)", row.Exit)
		}
		pr.Counts = tally(ms)
		c := pr.Counts
		if pr.Broke == "" && row.State == stateOK {
			switch {
			case len(ms) == 0:
				pr.Broke = "report holds no mutants"
			case c.Killed+c.Lived+c.TimedOut == 0 && c.NotViable > 0:
				pr.Broke = "every mutant was not viable"
			}
		}
		if pr.Broke == "" && c.Killed+c.Lived+c.TimedOut > 0 {
			evidence++
		}
		pr.Candidates = []candidate{}
		for _, m := range ms {
			if m.Status != stLived {
				continue
			}
			file := path.Join(row.Pkg, m.File)
			class, fn, specced := idx.classify(file, m.Line)
			switch class {
			case classSpecLine:
				pr.SpecLine++
			case classSpecFunc:
				pr.SpecFunc++
			default:
				src := ""
				if sf, err := idx.src.get(file); err == nil {
					src = trimSource(sf.line(m.Line))
				}
				pr.Candidates = append(pr.Candidates, candidate{
					Pkg: row.Pkg, File: file, Line: m.Line, Column: m.Column, Func: fn,
					Mutator: m.Type, Source: src, FileSpecced: specced,
				})
			}
		}
		sort.Slice(pr.Candidates, func(i, j int) bool {
			a, b := pr.Candidates[i], pr.Candidates[j]
			if a.File != b.File {
				return a.File < b.File
			}
			if a.Line != b.Line {
				return a.Line < b.Line
			}
			return a.Column < b.Column
		})
		if pr.Broke != "" {
			res.Broke = true
		}
		res.Packages = append(res.Packages, pr)
	}
	if evidence == 0 {
		res.Broke = true
	}
	return res
}

func trimSource(s string) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > 100 {
		s = string(r[:100]) + "…"
	}
	return s
}

// markdown renders the job summary. maxPer caps candidate rows per package; the
// JSON artifact always carries all of them.
func markdown(res triageResult, maxPer int) string {
	var b strings.Builder
	b.WriteString("# Gremlins daily sweep\n\n")
	b.WriteString("Discovery, not a gate: survivors are leads, and this run never fails on them. ")
	b.WriteString("A muhar spec is the proof that a guard's tests catch its removal; ")
	b.WriteString("a candidate below is a surviving mutant in a function no muhar spec mutates.\n\n")
	if res.Broke {
		b.WriteString("**The sweep broke** — see the Broke column. Evidence from a broken package is not counted.\n\n")
	}
	b.WriteString("| Package | State | Bin | Wall | Killed | Lived | Not covered | Timed out | Not viable | Efficacy | Lived near a spec | Candidates |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, p := range res.Packages {
		c := p.Counts
		eff := "n/a"
		if e, ok := c.efficacy(); ok {
			eff = fmt.Sprintf("%.1f%%", e)
		}
		state := p.State
		if p.Broke != "" {
			state = "broke: " + p.Broke
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s | %.0fs | %d | %d | %d | %d | %d | %s | %d | %d |\n",
			p.Pkg, mdCell(state), p.Bin, p.Wall, c.Killed, c.Lived, c.NotCovered, c.TimedOut, c.NotViable,
			eff, p.SpecLine+p.SpecFunc, len(p.Candidates))
	}
	b.WriteString("\n")
	for _, p := range res.Packages {
		if p.Note != "" {
			fmt.Fprintf(&b, "- `%s`: %s.\n", p.Pkg, p.Note)
		}
	}
	b.WriteString("\n## Candidate guards\n\n")
	listed := false
	for _, p := range res.Packages {
		if len(p.Candidates) == 0 {
			continue
		}
		listed = true
		fmt.Fprintf(&b, "### `%s` — %d\n\n", p.Pkg, len(p.Candidates))
		for i, c := range p.Candidates {
			if i == maxPer {
				fmt.Fprintf(&b, "- … and %d more in the JSON artifact\n", len(p.Candidates)-maxPer)
				break
			}
			fn := c.Func
			if fn == "" {
				fn = "package scope"
			}
			note := ""
			if !c.FileSpecced {
				note = " (no muhar spec touches this file)"
			}
			fmt.Fprintf(&b, "- `%s:%d:%d` `%s` %s — `%s`%s\n", c.File, c.Line, c.Column, fn, c.Mutator, mdCode(c.Source), note)
		}
		b.WriteString("\n")
	}
	if !listed {
		b.WriteString("None today.\n\n")
	}
	if len(res.Stale) > 0 {
		b.WriteString("## Spec entries that did not resolve\n\n")
		for _, s := range res.Stale {
			fmt.Fprintf(&b, "- `%s` entry `%s` → `%s`: %s\n", s.Spec, s.Entry, s.File, s.Reason)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "Cross-referenced against %d muhar specs.\n", res.Specs)
	return b.String()
}

func mdCell(s string) string { return strings.ReplaceAll(s, "|", `\|`) }
func mdCode(s string) string { return strings.ReplaceAll(s, "`", "'") }

func runTriage(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("triage", flag.ContinueOnError)
	fs.SetOutput(stderr)
	manifest := fs.String("manifest", "", "run manifest (tab-separated, one package per line)")
	root := fs.String("module-root", ".", "the Go module root the packages and specs are relative to")
	mdOut := fs.String("md", "", "write the Markdown summary here (default stdout)")
	jsonOut := fs.String("json", "", "write the JSON result here")
	maxPer := fs.Int("max-candidates", 25, "candidate rows per package in the Markdown")
	if err := fs.Parse(args); err != nil {
		return exitBroke
	}
	if *manifest == "" {
		fmt.Fprintln(stderr, "triage: -manifest is required")
		return exitBroke
	}
	f, err := os.Open(*manifest)
	if err != nil {
		fmt.Fprintln(stderr, "triage:", err)
		return exitBroke
	}
	rows, err := readManifest(f)
	f.Close()
	if err != nil {
		fmt.Fprintln(stderr, "triage:", err)
		return exitBroke
	}
	idx, err := loadSpecs(*root, newSourceCache(*root))
	if err != nil {
		fmt.Fprintln(stderr, "triage:", err)
		return exitBroke
	}
	res := triage(rows, idx)
	md := markdown(res, *maxPer)
	if *mdOut == "" {
		fmt.Fprint(stdout, md)
	} else if err := os.WriteFile(*mdOut, []byte(md), 0o644); err != nil {
		fmt.Fprintln(stderr, "triage:", err)
		return exitBroke
	}
	if *jsonOut != "" {
		b, _ := json.MarshalIndent(res, "", "  ")
		if err := os.WriteFile(*jsonOut, append(b, '\n'), 0o644); err != nil {
			fmt.Fprintln(stderr, "triage:", err)
			return exitBroke
		}
	}
	if res.Broke {
		fmt.Fprintln(stderr, "triage: the sweep broke; see the summary")
		return exitBroke
	}
	return 0
}
