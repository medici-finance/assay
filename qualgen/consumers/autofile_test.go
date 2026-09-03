package consumers

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/qualgen/filer"
)

// TestAutofile_AboveThresholdFilesExactlyOne is Verify #3 (DEREFERENCE): a
// single hotspot above threshold, routed through the GitHub reference filer in
// DRY-RUN, produces EXACTLY ONE item, and that item's composed body references
// the hotspot's own file path — proving the filer dereferenced the right
// hotspot, not merely that some item was produced.
func TestAutofile_AboveThresholdFilesExactlyOne(t *testing.T) {
	g := &filer.GitHubFiler{Owner: "medici-finance", Repo: "assay", ForceDryRun: true}
	cfg := AutofileConfig{HotspotThreshold: 1.0, Budget: 10, Filer: g}

	hotspots := []HotspotSignal{
		{Path: "qualgen/hotspot.go", Score: Measured(2.5)},                      // above 1.0 -> files
		{Path: "qualgen/quiet.go", Score: Measured(0.4)},                        // below -> skipped
		{Path: "qualgen/cnm.go", Score: CouldNotMeasure[float64]("no content")}, // never files
	}

	report, err := Autofile(cfg, hotspots, nil)
	if err != nil {
		t.Fatalf("Autofile: %v", err)
	}
	if got := len(report.Items); got != 1 {
		t.Fatalf("want exactly 1 item, got %d", got)
	}
	item := report.Items[0]
	if item.Filed {
		t.Fatalf("dry-run filer must file nothing, Filed=true")
	}
	if item.Item.TargetPath != "qualgen/hotspot.go" {
		t.Fatalf("item dereferenced the wrong hotspot: target=%q", item.Item.TargetPath)
	}
	if !strings.Contains(item.Item.Body, "qualgen/hotspot.go") {
		t.Fatalf("composed body does not reference the hotspot path; body=%q", item.Item.Body)
	}
	if !item.Item.Advisory {
		t.Fatalf("auto-filed item must be advisory")
	}
}

// TestAutofile_OverBudgetDegradesToDryRun is Verify #4 (budget/negative): with a
// filer that CAN file, three candidates and a budget of one file exactly one and
// degrade the rest to dry-run/logged — nothing filed beyond the budget.
func TestAutofile_OverBudgetDegradesToDryRun(t *testing.T) {
	posted := []string{}
	g := &filer.GitHubFiler{
		Owner: "medici-finance", Repo: "assay",
		Post: func(owner, repo string, item filer.RefactorItem) (string, error) {
			posted = append(posted, item.TargetPath)
			return "https://example/issues/" + item.TargetPath, nil
		},
	}
	cfg := AutofileConfig{HotspotThreshold: 1.0, Budget: 1, Filer: g}

	hotspots := []HotspotSignal{
		{Path: "a.go", Score: Measured(3.0)},
		{Path: "b.go", Score: Measured(3.0)},
		{Path: "c.go", Score: Measured(3.0)},
	}
	report, err := Autofile(cfg, hotspots, nil)
	if err != nil {
		t.Fatalf("Autofile: %v", err)
	}
	if len(report.Items) != 3 {
		t.Fatalf("want 3 composed items, got %d", len(report.Items))
	}
	if report.Filed != 1 {
		t.Fatalf("budget=1 must file exactly 1, filed=%d", report.Filed)
	}
	if report.DryRun != 2 {
		t.Fatalf("2 items should degrade to dry-run, got %d", report.DryRun)
	}
	if len(posted) != 1 {
		t.Fatalf("Post must be called exactly once (budget), got %d: %v", len(posted), posted)
	}
	// The one filed must be the lexically-first target (deterministic order).
	if posted[0] != "a.go" {
		t.Fatalf("budget should be spent on the first-sorted target, got %q", posted[0])
	}
}

// TestAutofile_ZeroBudgetFilesNothing: a zero budget is the legitimate
// "logged only" posture — every candidate dry-runs, nothing files, no error.
func TestAutofile_ZeroBudgetFilesNothing(t *testing.T) {
	posted := 0
	g := &filer.GitHubFiler{
		Owner: "o", Repo: "r",
		Post: func(string, string, filer.RefactorItem) (string, error) { posted++; return "x", nil },
	}
	cfg := AutofileConfig{HotspotThreshold: 0, Budget: 0, Filer: g}
	report, err := Autofile(cfg, []HotspotSignal{{Path: "a.go", Score: Measured(1.0)}}, nil)
	if err != nil {
		t.Fatalf("Autofile: %v", err)
	}
	if report.Filed != 0 || report.DryRun != 1 {
		t.Fatalf("zero budget: want 0 filed / 1 dry-run, got %d / %d", report.Filed, report.DryRun)
	}
	if posted != 0 {
		t.Fatalf("zero budget must never call Post, called %d", posted)
	}
}

// TestAutofile_DedupesHotspotAndCluster: a path that is BOTH an above-threshold
// hotspot and a cluster target yields exactly ONE item (deduped by path).
func TestAutofile_DedupesHotspotAndCluster(t *testing.T) {
	g := &filer.GitHubFiler{Owner: "o", Repo: "r", ForceDryRun: true}
	cfg := AutofileConfig{HotspotThreshold: 1.0, Budget: 10, Filer: g}

	hotspots := []HotspotSignal{{Path: "shared.go", Score: Measured(2.0)}}
	clusters := []DuplicateCluster{{ID: "blk1", Paths: []string{"shared.go", "z.go"}}}

	report, err := Autofile(cfg, hotspots, clusters)
	if err != nil {
		t.Fatalf("Autofile: %v", err)
	}
	targets := map[string]int{}
	for _, it := range report.Items {
		targets[it.Item.TargetPath]++
	}
	if targets["shared.go"] != 1 {
		t.Fatalf("shared.go must be deduped to exactly 1 item, got %d", targets["shared.go"])
	}
	// The cluster's primary path is the lexically-first path: "shared.go" < "z.go".
	// So the whole run yields exactly one item, keyed shared.go.
	if len(report.Items) != 1 {
		t.Fatalf("want 1 deduped item, got %d: %v", len(report.Items), targets)
	}
}

// TestAutofile_ClusterPrimaryPathIsLexFirst: a cluster with no hotspot files one
// item keyed on its lexically-first path, and the body names the cluster.
func TestAutofile_ClusterPrimaryPathIsLexFirst(t *testing.T) {
	g := &filer.GitHubFiler{Owner: "o", Repo: "r", ForceDryRun: true}
	cfg := AutofileConfig{HotspotThreshold: 1.0, Budget: 10, Filer: g}
	clusters := []DuplicateCluster{{ID: "blkX", Paths: []string{"m.go", "a.go", "z.go"}}}

	report, err := Autofile(cfg, nil, clusters)
	if err != nil {
		t.Fatalf("Autofile: %v", err)
	}
	if len(report.Items) != 1 {
		t.Fatalf("want 1 item, got %d", len(report.Items))
	}
	if report.Items[0].Item.TargetPath != "a.go" {
		t.Fatalf("cluster target should be lex-first path a.go, got %q", report.Items[0].Item.TargetPath)
	}
	if !strings.Contains(report.Items[0].Item.Body, "blkX") {
		t.Fatalf("cluster item body should name the cluster id")
	}
}

// TestAutofile_NoFilerErrors: a missing filer is a configuration error.
func TestAutofile_NoFilerErrors(t *testing.T) {
	_, err := Autofile(AutofileConfig{Budget: 1}, nil, nil)
	if err == nil {
		t.Fatalf("want error for nil filer")
	}
}
