package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// The five intake exits, in report order. These are the classification labels;
// any other expected or predicted exit is invalid.
var exits = []string{"spec/brief", "bug/issue", "finding", "needs-decision", "rejected/watching"}

func validExit(s string) bool {
	for _, e := range exits {
		if e == s {
			return true
		}
	}
	return false
}

// Provenance is the confirmation source a fixture's label rests on. The scorer
// checks that every field is present; the vocabularies of type, tier and
// confirmed_by belong to the corpus owner's own lint.
type Provenance struct {
	Type         string `yaml:"type"`
	Tier         string `yaml:"tier"`
	Ratification string `yaml:"ratification"`
	Ref          string `yaml:"ref"`
	ConfirmedBy  string `yaml:"confirmed_by"`
	Date         string `yaml:"date"`
	Quote        string `yaml:"quote"`
}

// Fixture is one gold-set file: one inbound and the exit it should have taken.
type Fixture struct {
	ID                 string      `yaml:"id"`
	Inbound            string      `yaml:"inbound"`
	ExpectedExit       string      `yaml:"expected_exit"`
	MethodologyVersion string      `yaml:"methodology_version"`
	Scoring            string      `yaml:"scoring"`
	Provenance         *Provenance `yaml:"provenance"`
	Notes              string      `yaml:"notes"`

	file string
}

// GoldSet is a linted corpus. Scored holds the fixtures that enter a score;
// everything else is reported but never scored.
type GoldSet struct {
	Fixtures       []Fixture
	Scored         []Fixture
	PendingHuman   []string // excluded fixtures awaiting human confirmation
	Stale          []string // fixtures behind the current methodology version
	CurrentVersion string
}

// LintError carries every gold-set violation found, so a consumer sees the
// whole list at once instead of fixing one per run.
type LintError struct{ Problems []string }

func (e *LintError) Error() string {
	return fmt.Sprintf("gold set failed preflight lint (%d problem(s)):\n  %s",
		len(e.Problems), strings.Join(e.Problems, "\n  "))
}

// loadGold reads every *.yaml / *.yml file directly under dir as one fixture,
// enforces the preflight lint rules, and returns the scored subset.
// currentVersion, when non-empty, is the corpus's methodology version; when
// empty the newest version found on any fixture is used. Versions are compared
// as strings, so they must sort lexically (ISO dates such as 2026-01-31 do).
func loadGold(dir, currentVersion string) (*GoldSet, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read gold set: %w", err)
	}
	var problems []string
	gs := &GoldSet{}
	seen := map[string]string{}
	for _, ent := range entries {
		name := ent.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ent.IsDir() || (ext != ".yaml" && ext != ".yml") {
			continue
		}
		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read fixture: %w", err)
		}
		var f Fixture
		if err := yaml.Unmarshal(raw, &f); err != nil {
			problems = append(problems, fmt.Sprintf("%s: not valid YAML: %v", name, err))
			continue
		}
		f.file = name
		problems = append(problems, lintFixture(f)...)
		if f.ID != "" {
			if prev, dup := seen[f.ID]; dup {
				problems = append(problems, fmt.Sprintf("%s: duplicate id %q (also in %s)", name, f.ID, prev))
			}
			seen[f.ID] = name
		}
		gs.Fixtures = append(gs.Fixtures, f)
	}
	if len(gs.Fixtures) == 0 {
		return nil, fmt.Errorf("gold set %s holds no *.yaml fixtures", dir)
	}

	// Rule 3: every exit is represented somewhere in the corpus files.
	present := map[string]bool{}
	for _, f := range gs.Fixtures {
		present[f.ExpectedExit] = true
	}
	for _, e := range exits {
		if !present[e] {
			problems = append(problems, fmt.Sprintf("corpus: exit %q has no fixture at all (scored or excluded)", e))
		}
	}
	if len(problems) > 0 {
		return nil, &LintError{Problems: problems}
	}

	// Rule 4: STALE detection against the current methodology version.
	gs.CurrentVersion = currentVersion
	if gs.CurrentVersion == "" {
		for _, f := range gs.Fixtures {
			if f.MethodologyVersion > gs.CurrentVersion {
				gs.CurrentVersion = f.MethodologyVersion
			}
		}
	}
	for _, f := range gs.Fixtures {
		if f.MethodologyVersion < gs.CurrentVersion {
			gs.Stale = append(gs.Stale, f.ID)
		}
		if f.Scoring == "scored" {
			gs.Scored = append(gs.Scored, f)
		} else {
			gs.PendingHuman = append(gs.PendingHuman, f.ID)
		}
	}
	sort.Strings(gs.Stale)
	sort.Strings(gs.PendingHuman)
	return gs, nil
}

// lintFixture applies the per-fixture preflight rules: provenance present and
// complete, expected_exit valid, methodology_version present, scoring present
// and consistent with ratification. A violation makes the fixture invalid; it
// is never skipped silently.
func lintFixture(f Fixture) []string {
	var p []string
	add := func(format string, a ...any) {
		p = append(p, f.file+": "+fmt.Sprintf(format, a...))
	}
	if strings.TrimSpace(f.ID) == "" {
		add("missing id")
	}
	if !validExit(f.ExpectedExit) {
		add("expected_exit %q is not one of the five exits (%s)", f.ExpectedExit, strings.Join(exits, ", "))
	}
	if strings.TrimSpace(f.MethodologyVersion) == "" {
		add("missing methodology_version")
	}
	if f.Provenance == nil {
		add("missing provenance (an unconfirmed label is invalid)")
	} else {
		pv := f.Provenance
		for _, fld := range []struct{ name, val string }{
			{"type", pv.Type}, {"tier", pv.Tier}, {"ratification", pv.Ratification},
			{"ref", pv.Ref}, {"confirmed_by", pv.ConfirmedBy}, {"date", pv.Date}, {"quote", pv.Quote},
		} {
			if strings.TrimSpace(fld.val) == "" {
				add("provenance.%s is missing or empty", fld.name)
			}
		}
		if pv.Ratification != "" && pv.Ratification != "confirmed" && pv.Ratification != "pending-human" {
			add("provenance.ratification %q must be confirmed or pending-human", pv.Ratification)
		}
	}
	switch f.Scoring {
	case "scored":
		if f.Provenance != nil && f.Provenance.Ratification != "confirmed" {
			add("scoring: scored requires provenance.ratification: confirmed")
		}
	case "excluded":
		if f.Provenance != nil && f.Provenance.Ratification == "confirmed" {
			add("scoring: excluded with provenance.ratification: confirmed is inconsistent")
		}
	case "":
		add("missing scoring (must be scored or excluded)")
	default:
		add("scoring %q must be scored or excluded", f.Scoring)
	}
	return p
}

// loadPredictions reads a YAML or JSON mapping of fixture id → predicted exit.
func loadPredictions(path string) (map[string]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read predictions: %w", err)
	}
	var m map[string]string
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("predictions %s: want a mapping of fixture id to exit: %w", path, err)
	}
	if m == nil {
		return nil, errors.New("predictions file is empty")
	}
	var bad []string
	for id, e := range m {
		if !validExit(e) {
			bad = append(bad, fmt.Sprintf("%s → %q", id, e))
		}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		return nil, fmt.Errorf("predictions name exits outside the five: %s", strings.Join(bad, "; "))
	}
	return m, nil
}
