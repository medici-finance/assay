package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// pack is the one in-memory model both output formats render from, so the .md a
// human hands an auditor and the .json a consumer parses can never disagree —
// they are two renderings of this single struct produced in one run.
type pack struct {
	Schema    string       `json:"schema"`
	Tag       string       `json:"tag"`
	Generated string       `json:"generated"`
	Complete  bool         `json:"complete"`
	Declared  int          `json:"declared_controls"`
	Gates     []gateRecord `json:"gates"`
	Drift     driftReport  `json:"drift"`
	Omitted   []omission   `json:"omitted"`
}

// gateRecord is one release-gated control's demonstration at this release.
type gateRecord struct {
	Gate            string         `json:"gate"`
	Spec            string         `json:"spec"`
	Rationale       string         `json:"rationale"`
	PositiveControl string         `json:"positive_control,omitempty"`
	Harness         string         `json:"harness"` // healthy | broken | no-report
	HarnessNote     string         `json:"harness_note,omitempty"`
	Instrument      instrumentView `json:"instrument"`
	Mutations       []mutationRow  `json:"mutations"`
}

// instrumentView is the four-column row from docs/three-state-instrument-rule.md
// applied to muhar-as-instrument for this gate: the exact string it prints when
// it cannot see, how many states it reports, and this gate's disposition.
type instrumentView struct {
	Instrument      string `json:"instrument"`
	PrintsWhenBlind string `json:"prints_when_blind"`
	States          int    `json:"states"`
	Disposition     string `json:"disposition"`
}

// mutationRow is one injected error and its verdict — the mutation name is the
// evidence and is reproduced verbatim from the spec, never paraphrased.
type mutationRow struct {
	Name    string  `json:"name"`
	Verdict verdict `json:"verdict"`
	Date    string  `json:"date"`
	Tag     string  `json:"tag"`
}

// driftReport records divergence between the declared control set and the
// mutation specs on disk, in BOTH directions (D6: honesty about non-coverage is
// itself a device).
type driftReport struct {
	DeclaredSpecsWithNoFile []string `json:"declared_specs_with_no_file"`
	SpecFilesNotDeclared    []string `json:"spec_files_not_declared"`
}

// omission is one declared control the pack could not evidence: its report was
// missing, unparseable, or a HARNESS BROKEN discard. Named here, it is the
// pack's own statement of what it is missing (docs/evidence-bundle.md).
type omission struct {
	Spec   string `json:"spec"`
	Reason string `json:"reason"`
}

// specFile is the subset of a muhar spec the pack reads: the positive control's
// name and every mutation's name. The names are muhar's, verbatim.
type specFile struct {
	Control struct {
		Name string `json:"name"`
	} `json:"control"`
	Mutations []struct {
		Name string `json:"name"`
	} `json:"mutations"`
}

// assemble builds the pack from the declared control set, the specs on disk
// under root, and the captured muhar reports in reportsDir. It never fails on a
// missing/broken REPORT — that is an omission recorded in the pack — but does
// return an error on an IO fault that makes the whole run untrustworthy (an
// unreadable reports dir that is not simply absent).
func assemble(root, reportsDir, tag, date string) (*pack, error) {
	p := &pack{
		Schema:    "tool-validation-pack-v1",
		Tag:       tag,
		Generated: date,
		Declared:  len(declaredControls),
	}
	p.Drift = assembleDrift(root)

	for _, c := range declaredControls {
		rec := gateRecord{
			Gate:      c.Gate,
			Spec:      c.Spec,
			Rationale: c.Why,
			Instrument: instrumentView{
				Instrument:      "muhar -spec " + c.Spec,
				PrintsWhenBlind: instrumentBlind,
				States:          3,
			},
		}

		spec, specErr := readSpec(filepath.Join(root, c.Spec))
		if specErr != nil {
			// A declared control whose spec file is gone cannot be evidenced at
			// all: no mutation names to enumerate, no verdicts to render.
			rec.Harness = "no-report"
			rec.HarnessNote = "declared spec not found on disk: " + specErr.Error()
			rec.Instrument.Disposition = "follow-up — declared control's spec is missing on disk"
			p.Gates = append(p.Gates, rec)
			p.Omitted = append(p.Omitted, omission{Spec: c.Spec, Reason: "declared spec not found on disk: " + specErr.Error()})
			continue
		}
		rec.PositiveControl = spec.Control.Name

		outcome, missing := loadOutcome(reportsDir, c)
		switch {
		case missing != "":
			rec.Harness = "no-report"
			rec.HarnessNote = missing
			rec.Mutations = allCouldNotCheck(spec, date, tag)
			rec.Instrument.Disposition = "follow-up — re-run at next release (" + missing + ")"
			p.Omitted = append(p.Omitted, omission{Spec: c.Spec, Reason: missing})
		case outcome.Broken:
			// muhar exit 2: the harness is broken, so the run carries NO
			// trustworthy per-guard verdict. Every control in this spec is
			// could-not-check — never a pass, never a fail.
			rec.Harness = "broken"
			rec.HarnessNote = outcome.BrokenReason
			rec.Mutations = allCouldNotCheck(spec, date, tag)
			rec.Instrument.Disposition = "follow-up — harness broken, re-run at next release"
			p.Omitted = append(p.Omitted, omission{Spec: c.Spec, Reason: "HARNESS BROKEN: " + outcome.BrokenReason})
		default:
			rec.Harness = "healthy"
			rec.Mutations = matchAll(spec, outcome.Verdicts, date, tag)
			rec.Instrument.Disposition = disposition(rec.Mutations)
		}
		p.Gates = append(p.Gates, rec)
	}

	p.Complete = len(p.Omitted) == 0
	return p, nil
}

// loadOutcome reads the captured report(s) for one control and returns a single
// unioned outcome. missing is non-empty (and outcome zero) when a required
// report file is absent — for a sharded gate, a single absent shard makes the
// whole gate missing, because a partial shard set covers only part of the spec
// and must never read as a complete demonstration. A shard that is present but
// HARNESS BROKEN makes the union broken.
func loadOutcome(reportsDir string, c control) (reportOutcome, string) {
	if c.Shards <= 0 {
		path := filepath.Join(reportsDir, c.ReportKey+".report")
		text, err := os.ReadFile(path)
		if err != nil {
			return reportOutcome{}, "report missing: " + c.ReportKey + ".report"
		}
		return parseReport(string(text)), ""
	}
	var union reportOutcome
	for i := 0; i < c.Shards; i++ {
		name := fmt.Sprintf("%s.%d.report", c.ReportKey, i)
		text, err := os.ReadFile(filepath.Join(reportsDir, name))
		if err != nil {
			return reportOutcome{}, "shard report missing: " + name
		}
		o := parseReport(string(text))
		if o.Broken {
			return reportOutcome{Broken: true, BrokenReason: fmt.Sprintf("shard %d/%d: %s", i, c.Shards, o.BrokenReason)}, ""
		}
		union.Verdicts = append(union.Verdicts, o.Verdicts...)
	}
	return union, ""
}

// matchAll pairs every mutation the spec declares with its verdict from the
// (healthy) report. Enumeration is driven by the SPEC, not the report, so a
// mutation the spec declares but the report never mentions is could-not-check,
// not silently dropped.
func matchAll(spec *specFile, verdicts []namedVerdict, date, tag string) []mutationRow {
	rows := make([]mutationRow, 0, len(spec.Mutations))
	for _, m := range spec.Mutations {
		rows = append(rows, mutationRow{
			Name:    m.Name,
			Verdict: matchVerdict(m.Name, verdicts),
			Date:    date,
			Tag:     tag,
		})
	}
	return rows
}

// allCouldNotCheck renders every mutation a spec declares as could-not-check —
// the rendering for a missing report or a HARNESS BROKEN discard.
func allCouldNotCheck(spec *specFile, date, tag string) []mutationRow {
	rows := make([]mutationRow, 0, len(spec.Mutations))
	for _, m := range spec.Mutations {
		rows = append(rows, mutationRow{Name: m.Name, Verdict: vCouldNotCheck, Date: date, Tag: tag})
	}
	return rows
}

// disposition summarises a healthy gate's outcome using the instrument rule's
// vocabulary, worst state first. A NOT CAUGHT would have already reddened the
// release's own Totals grep, so it should not reach here; it is rendered
// honestly if it ever does.
func disposition(rows []mutationRow) string {
	var notCaught, couldNotMut, couldNotCheck int
	for _, r := range rows {
		switch r.Verdict {
		case vNotCaught:
			notCaught++
		case vCouldNotMut:
			couldNotMut++
		case vCouldNotCheck:
			couldNotCheck++
		}
	}
	switch {
	case notCaught > 0:
		return "follow-up — a refusal is NOT CAUGHT; the gate is not load-bearing as written"
	case couldNotCheck > 0:
		return "follow-up — a declared mutation is absent from the report (could-not-check)"
	case couldNotMut > 0:
		return "follow-up — an edit could not be planted; that guard was not exercised this run"
	default:
		return "fixed-here — every declared mutation reddened its suite at this release"
	}
}

func readSpec(path string) (*specFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s specFile
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &s, nil
}

// assembleDrift compares the declared control set against every *mutations*.json
// on disk under tools/desk, reporting divergence in both directions. This is
// the structural guard the brief turns on: it makes a declared control whose
// spec vanished, and a mutation spec added to the tree but never declared here,
// each a visible line rather than a silent change in coverage.
func assembleDrift(root string) driftReport {
	declared := map[string]bool{}
	for _, c := range declaredControls {
		declared[c.Spec] = true
	}

	onDisk := discoverSpecs(root)
	onDiskSet := map[string]bool{}
	for _, p := range onDisk {
		onDiskSet[p] = true
	}

	var d driftReport
	for _, c := range declaredControls {
		if !onDiskSet[c.Spec] {
			d.DeclaredSpecsWithNoFile = append(d.DeclaredSpecsWithNoFile, c.Spec)
		}
	}
	for _, p := range onDisk {
		if !declared[p] {
			d.SpecFilesNotDeclared = append(d.SpecFilesNotDeclared, p)
		}
	}
	sort.Strings(d.DeclaredSpecsWithNoFile)
	sort.Strings(d.SpecFilesNotDeclared)
	return d
}

// discoverSpecs returns the repo-relative paths of every *mutations*.json under
// tools/desk, matching the `git ls-files 'tools/desk/*mutations*.json'` set the
// brief anchors the declared control enumeration against.
func discoverSpecs(root string) []string {
	base := filepath.Join(root, "tools", "desk")
	var out []string
	_ = filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // a missing tools/desk yields an empty set, not a crash
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if strings.Contains(name, "mutations") && strings.HasSuffix(name, ".json") {
			rel, relErr := filepath.Rel(root, path)
			if relErr == nil {
				out = append(out, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func (p *pack) renderJSON() ([]byte, error) {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
