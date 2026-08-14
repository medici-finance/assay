package main

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// topologyvalues_test.go — the statusgen half of ground-truth/04's derive-or-diff
// enforcement. topologyvalues.go is a DERIVATION of `topology.yaml`; this binds
// the two so they cannot drift silently, and the desk module's
// TestTopologyDriftRegistry binds the other derivation to the same source.
//
// CROSS-MODULE READ. This test reads `../topology.yaml`, outside statusgen's own
// module, so it is registered in tools/desk/internal/deskkit/citrigger_test.go's
// ciCrossModuleRegistry — the guard that proves an edit to what a test READS
// actually triggers the job that RUNS it. A cross-module guard whose CI filter
// excludes what it reads is advisory, not enforced.
//
// THREE-STATE. An unreadable or unparseable source is a FAILURE naming what could
// not be checked, never a pass.

// topologySource is the shape this test reads out of topology.yaml. It models
// only the fields statusgen derives — the desk module's reader models the rest.
type topologySource struct {
	Schema      string `yaml:"schema"`
	ReleaseRepo string `yaml:"release_repo"`
	Labels      struct {
		SystemState []struct {
			Name string `yaml:"name"`
			Why  string `yaml:"why"`
		} `yaml:"system_state"`
		DecisionOwed []struct {
			Name string `yaml:"name"`
			Why  string `yaml:"why"`
		} `yaml:"decision_owed"`
	} `yaml:"labels"`
}

func loadTopologySource(t *testing.T) topologySource {
	t.Helper()
	path := filepath.Join("..", topologySourceFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("COULD-NOT-CHECK: reading the declared topology source %s: %v\n"+
			"  topologyvalues.go is a DERIVATION of that file. Without it this test verifies "+
			"nothing, so it fails rather than passing quietly.", path, err)
	}
	var src topologySource
	if err := yaml.Unmarshal(raw, &src); err != nil {
		t.Fatalf("COULD-NOT-CHECK: parsing %s: %v", path, err)
	}
	return src
}

// TestTopologyValuesMatchSource is the diff. It is what makes topologyvalues.go a
// derivation rather than a sixth hand table.
func TestTopologyValuesMatchSource(t *testing.T) {
	src := loadTopologySource(t)

	if src.Schema != topologySchema {
		t.Fatalf("schema drift: %s declares %q, this derivation was taken from %q.\n"+
			"  Refusing to compare across schema versions — re-derive topologyvalues.go against the new schema.",
			topologySourceFile, src.Schema, topologySchema)
	}

	if src.ReleaseRepo != topologyReleaseRepo {
		t.Errorf("release_repo drift: source %q, topologyvalues.go %q — the source wins",
			src.ReleaseRepo, topologyReleaseRepo)
	}

	var wantSystem, wantDecision []string
	for _, l := range src.Labels.SystemState {
		if strings.TrimSpace(l.Why) == "" {
			t.Errorf("%s: labels.system_state entry %q has no `why:` — a label with no stated "+
				"rationale is unreviewable, which is how a set nobody can audit accretes entries",
				topologySourceFile, l.Name)
		}
		wantSystem = append(wantSystem, l.Name)
	}
	for _, l := range src.Labels.DecisionOwed {
		wantDecision = append(wantDecision, l.Name)
	}

	if len(wantSystem) == 0 {
		t.Fatalf("COULD-NOT-CHECK: %s declares no labels.system_state — an empty read is not an "+
			"empty set, and comparing against it would pass any derivation", topologySourceFile)
	}

	if !reflect.DeepEqual(sortedCopy(wantSystem), sortedCopy(topologySystemStateLabels)) {
		t.Errorf("DERIVATION DRIFT — labels.system_state\n"+
			"  %s says %v\n"+
			"  statusgen/topologyvalues.go says %v\n"+
			"  Edit the SOURCE first, then mirror it into the derivation. The source wins.\n"+
			"  This is the exact divergence #829 filed: two copies of one set, kept equal by a comment.",
			topologySourceFile, sortedCopy(wantSystem), sortedCopy(topologySystemStateLabels))
	}
	if !reflect.DeepEqual(sortedCopy(wantDecision), sortedCopy(topologyDecisionOwedLabels)) {
		t.Errorf("DERIVATION DRIFT — labels.decision_owed\n  %s says %v\n  statusgen/topologyvalues.go says %v",
			topologySourceFile, sortedCopy(wantDecision), sortedCopy(topologyDecisionOwedLabels))
	}
}

// TestTopologyValuesDiffCanFail is the positive control: it proves the comparison
// above discriminates. Without it, a green run is consistent with a comparison
// that always passes — the unfailable-check defect (#488).
func TestTopologyValuesDiffCanFail(t *testing.T) {
	src := loadTopologySource(t)
	var fromSource []string
	for _, l := range src.Labels.SystemState {
		fromSource = append(fromSource, l.Name)
	}
	bent := sortedCopy(topologySystemStateLabels)
	bent = bent[:len(bent)-1] // a derivation that dropped one label
	if reflect.DeepEqual(sortedCopy(fromSource), bent) {
		t.Fatal("POSITIVE CONTROL FAILED: dropping a label from the derivation still compared EQUAL " +
			"to the declared source. The drift comparison is vacuous.")
	}
}

// TestScanExcludedLabelSetIsDerived pins the CONSUMER, not just the value: the
// scanner's matcher must answer from the derived set. A derivation nothing reads
// protects nothing.
func TestScanExcludedLabelSetIsDerived(t *testing.T) {
	set := scanExcludedLabelSet()
	for _, name := range topologySystemStateLabels {
		if !set[strings.ToLower(name)] {
			t.Errorf("scanExcludedLabelSet() omits %q, which the derivation declares", name)
		}
		if !hasExcludedLabel([]string{strings.ToUpper(name)}) {
			t.Errorf("hasExcludedLabel is not case-insensitive for %q", name)
		}
	}
	if hasExcludedLabel([]string{"bug"}) {
		t.Error("hasExcludedLabel matched a label outside the system-state set")
	}
	// review-request is the entry issueboard's retired hand copy was MISSING
	// (#829). Pinned by name so a future edit that drops it fails HERE, with the
	// issue number in the message, rather than as a mystery board regression.
	if !set["review-request"] {
		t.Error("review-request is absent from the system-state set — that omission IS #829")
	}
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}
