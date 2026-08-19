package topology

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// example_test.go — the publication-boundary check for Ian's 2026-08-13 ruling
// on this brief's staged question (`risk.sensitive-data: yes`).
//
// THE RULING. `topology.yaml` is WITHHELD PERMANENTLY (`do-not-copy`): the file
// is by design the sink every topological fact ends up in, so `copy` on that
// PATH would be a standing decision to publish whatever it accumulates. The
// public tree instead ships `topology.example.yaml`, which the stager relocates
// in-tree to the root `topology.yaml` — the name SourceFile reads — so the
// published loader has a real source instead of being furniture.
//
// THE DEFECT THAT ARRANGEMENT INVITES, and what this file is for. A withheld
// real file plus a hand-written public twin is EXACTLY the second-copy defect
// this brief exists to kill (#276): two files stating one schema, no generator,
// no cross-check, and drift that is invisible from inside either one. There were
// two ways to close it.
//
//	DERIVE the example from the source. REJECTED. Two reasons, one practical and
//	one about the ruling. Practically, the load-bearing content of the example is
//	the PROSE — `note:`, `purpose:`, `summary:`, `why:` — which the compiled
//	derivations deliberately drop and which Parse REFUSES to do without for a
//	label, so a generator over the parsed model emits a file the loader will not
//	read. And on the ruling: a generator with a `copy` disposition is a standing
//	decision to publish whatever the withheld sink grows, in a new costume. The
//	ruling rejected that when it rejected publishing the path.
//
//	DIFF the example's STRUCTURE against the source's, and fail on drift. CHOSEN.
//	It fails CLOSED in the direction that matters: a section or a field added to
//	the withheld source reddens CI until a person writes the public placeholder,
//	so the disclosure call is re-affirmed by a human on every schema change
//	rather than executed silently by a filter.
//
// WHAT IT DIFFS, AND WHAT IT DELIBERATELY DOES NOT. It compares SHAPE — which
// sections are stated, which fields are exercised — never VALUES. Requiring
// equal values would make the example a copy of the source by construction,
// which is the thing being prevented. `apps[].id` and `humans[]` are the two
// disclosure-bearing shapes and are EXEMPT from coverage in the source->example
// direction: if the source ever states them, the correct example does NOT follow.
// Part 3 asserts that directly.
//
// THE PER-CELL FIELDS, AND WHY THEIR VALUES ARE COUPLED.
// `cell:` and `repos[].relationship` state facts that are true of ONE CELL, so
// the instinct is that the example — another cell's starting point — should
// carry different values. It cannot, and the reason is structural rather than a
// preference: `compiled.go` is COPIED into the public tree, where the relocated
// example becomes the file TestTopologyDriftRegistry part 1 compares it against.
// So every field the derivation carries must hold the SAME value in both the
// withheld source and the example, or the published repo's own suite is red on
// arrival. deskroster must print the cell name and the relationship, so both are
// compiled, so both are value-coupled.
//
// The honest consequence is stated in the example's own header rather than
// hidden here: the public example ships as the toolkit CELL'S instance
// (which is true — the public tree is that cell's), and its first documented
// edits for an adopting cell are to rename `cell:` and flip
// `medici-finance/assay` to `upstream`. The shape rows below still earn their
// place: they catch a SOURCE that grows a per-cell field the example never
// demonstrates, with a message that says what to do.
//
// THREE-STATE. Both files are read and both reads are checked. An unreadable or
// unparseable file is COULD-NOT-CHECK and FAILS — a checker that could not look
// never reports green.
//
// PROOF IT CAN FAIL. TestPublicExampleBoundaryPositiveControl feeds the pure
// cores a source that grew a field the example lacks, and an example carrying a
// house repo slug, an App id and a human — and asserts each is caught.
//
// THIS FILE IS WITHHELD (`do-not-copy` override in docs/publication-manifest.yaml)
// because it CANNOT run in the public tree, not because of what it says: it opens
// two files, one of which is absent there by design and the other renamed.

// ExampleFile is the repo-relative path of the PUBLIC tree's topology source.
// The stager relocates it in-tree to SourceFile, so in the published repo this
// file's content is what the loader and TestTopologyDriftRegistry read.
const ExampleFile = "topology.example.yaml"

// publishableRepoOwners are the slug prefixes the public example may state.
// `example-org/` is the illustrative placeholder namespace; the toolkit's own
// public repo is already public by definition. Anything else in the example is a
// house repo slug reaching the public tree.
var publishableRepoPrefixes = []string{"example-org/", "medici-finance/assay"}

func TestPublicExampleTracksTheSourceShape(t *testing.T) {
	// The separate example file exists only in the source repository; the stager
	// relocates it in-tree to the root topology.yaml, so a published/consumer tree
	// carries topology.yaml but no topology.example.yaml. Skip when the example is
	// absent; where it is present this source-vs-example drift check runs in full.
	// Only os.ErrNotExist skips — an example that exists but cannot be read fails.
	if _, err := os.Stat(filepath.Join(repoRoot, ExampleFile)); errors.Is(err, os.ErrNotExist) {
		t.Skipf("%s not present in this tree", ExampleFile)
	}
	src := loadSourceOrFail(t)
	ex := loadExampleOrFail(t)

	t.Run("the example is a valid instance of the same schema", func(t *testing.T) {
		// LoadFile already refused anything that is not Schema, so reaching here
		// proves it. Asserting it anyway keeps the failure legible if Parse ever
		// grows a lenient path.
		if ex.Schema != src.Schema {
			t.Errorf("SCHEMA DRIFT — %s declares %q, %s declares %q.\n"+
				"  The public tree reads the example under the same reader as the source; a "+
				"different schema version there is a different file format shipping to adopters.",
				ExampleFile, ex.Schema, SourceFile, src.Schema)
		}
	})

	t.Run("every shape the source states is stated by the example", func(t *testing.T) {
		for _, d := range exampleShapeDiffs(src, ex) {
			t.Errorf("EXAMPLE SHAPE DRIFT — %s\n"+
				"  %s is the WITHHELD declared source; %s is what the public tree ships in its "+
				"place (relocated to %s at staging). The example must demonstrate every shape the "+
				"source uses, or the published loader ships a schema its own example does not show.\n"+
				"  FIX: state the shape in %s with an illustrative `example-org/*` value. Do NOT "+
				"copy the source's value — the example is placeholders, and part 3 enforces that.",
				d, SourceFile, ExampleFile, SourceFile, ExampleFile)
		}
	})

	t.Run("the example discloses nothing", func(t *testing.T) {
		for _, f := range examplePublishabilityFindings(ex) {
			t.Errorf("EXAMPLE DISCLOSURE — %s\n"+
				"  %s is COPIED into the public tree. It carries schema and structure only: "+
				"illustrative placeholders, App ROLE names with no ids, and no human identities.\n"+
				"  FIX: replace the value with an `example-org/*` placeholder, or drop the entry. "+
				"If a real fact genuinely must be stated somewhere, operator configuration "+
				"(ASSAY_*) is its home, not a tracked file.",
				f, ExampleFile)
		}
	})

	t.Run("the example satisfies the drift check the public tree runs on it", func(t *testing.T) {
		// In the staged tree the example IS the root topology.yaml, so the SHIPPED
		// TestTopologyDriftRegistry compares it against compiled.go. If they
		// disagree, the public repo's own suite is red on arrival, which is not a
		// thing to discover after publication.
		//
		// This holds TODAY because the source states only publishable illustrative
		// topology, so the example can state the same values without disclosing
		// anything. It is NOT a licence to keep the example equal to the source:
		// the day the source states house topology, this assertion and part 3 above
		// become mutually unsatisfiable and CI goes red — which is the correct place
		// for that decision to surface, in front of a person, rather than inside a
		// copy nobody re-reads.
		for _, d := range topologyDiffs(ex, Compiled()) {
			t.Errorf("STAGED-TREE DRIFT — %s\n"+
				"  compiled.go is PUBLIC and %s becomes the public tree's %s, so the shipped "+
				"TestTopologyDriftRegistry compares exactly these two. A disagreement here is a "+
				"red suite in the published repo.\n"+
				"  FIX: if the value is publishable, mirror it into the example. If it is NOT, "+
				"it does not belong in compiled.go either — that file is copied.",
				d, ExampleFile, SourceFile)
		}
	})
}

func TestPublicExampleBoundaryPositiveControl(t *testing.T) {
	t.Run("the shape diff catches a field the example does not demonstrate", func(t *testing.T) {
		src := Topology{
			Schema:      Schema,
			ReleaseRepo: "example-org/tracker",
			Repos: []Repo{{
				Slug:             "example-org/tracker",
				Visibility:       VisibilityPublic,
				CIRequired:       true,
				Root:             ".",
				RiskPathTriggers: []string{"base/"},
				Note:             "why this repo is stated",
			}},
			Apps:               []App{{Role: "desk", Purpose: "p"}},
			Products:           []Product{{Name: "assay", Summary: "s", Repos: []string{"example-org/tracker"}}},
			SystemStateLabels:  []Label{{Name: "verify-gate", Why: "w"}},
			DecisionOwedLabels: []Label{{Name: "question", Why: "w"}},
		}
		// The example demonstrates every shape EXCEPT repos[].risk_path_triggers.
		ex := src
		ex.Repos = []Repo{{
			Slug:       "example-org/tracker",
			Visibility: VisibilityPublic,
			CIRequired: true,
			Root:       ".",
			Note:       "why this repo is stated",
		}}
		got := exampleShapeDiffs(src, ex)
		if len(got) == 0 {
			t.Fatal("POSITIVE CONTROL FAILED: the shape diff did NOT flag an example missing " +
				"repos[].risk_path_triggers — it would not notice the source growing a field")
		}
		if !strings.Contains(strings.Join(got, "\n"), "risk_path_triggers") {
			t.Errorf("POSITIVE CONTROL FAILED: the shape diff fired but did not name the field: %v", got)
		}

		// And the negative direction: an example that DOES demonstrate everything
		// must produce no findings, or the check is noise rather than a signal.
		if got := exampleShapeDiffs(src, src); len(got) != 0 {
			t.Errorf("false positive: a shape-complete example was flagged: %v", got)
		}
	})

	t.Run("the shape diff catches an example that defaults the per-cell fields", func(t *testing.T) {
		// The per-cell-fields positive control. The trap it guards is specific: relationship's
		// ZERO VALUE is upstream, so an example that states nothing at all still
		// PARSES as upstream everywhere. A shape row written against the value
		// rather than against statedness would call that "demonstrated" and report
		// green over an example that teaches the field to nobody.
		src := Topology{
			Schema: Schema,
			Cell:   "some-cell",
			Repos: []Repo{
				{Slug: "example-org/tracker", Relationship: RelationshipOwned, RelationshipStated: true},
				{Slug: "example-org/example-k8s", Relationship: RelationshipUpstream, RelationshipStated: true},
			},
			SystemStateLabels: []Label{{Name: "verify-gate", Why: "w"}},
		}
		// The example names no cell and states no relationship — every repo in it
		// DEFAULTS to upstream.
		ex := Topology{
			Schema: Schema,
			Repos: []Repo{
				{Slug: "example-org/tracker"},
				{Slug: "example-org/example-k8s"},
			},
			SystemStateLabels: []Label{{Name: "verify-gate", Why: "w"}},
		}
		joined := strings.Join(exampleShapeDiffs(src, ex), "\n")
		for _, want := range []string{"cell", "relationship stated as owned", "relationship stated as upstream"} {
			if !strings.Contains(joined, want) {
				t.Errorf("POSITIVE CONTROL FAILED: the shape diff did not name %q. Got:\n%s", want, joined)
			}
		}
		if got := exampleShapeDiffs(src, src); len(got) != 0 {
			t.Errorf("false positive: a shape-complete example was flagged: %v", got)
		}
	})

	t.Run("the disclosure check catches a house slug, an App id and a human", func(t *testing.T) {
		bad := Topology{
			Schema: Schema,
			Repos: []Repo{
				{Slug: "example-org/tracker"},
				{Slug: "some-house-org/private-thing"},
			},
			Apps:   []App{{Role: "reviewer", ID: 4331323, IDSet: true}},
			Humans: []Human{{Name: "someone", Login: "someone"}},
		}
		joined := strings.Join(examplePublishabilityFindings(bad), "\n")
		for _, want := range []string{"some-house-org/private-thing", "reviewer", "humans"} {
			if !strings.Contains(joined, want) {
				t.Errorf("POSITIVE CONTROL FAILED: the disclosure check did not name %q. Got:\n%s", want, joined)
			}
		}

		clean := Topology{
			Schema: Schema,
			Repos: []Repo{
				{Slug: "example-org/tracker"},
				{Slug: "medici-finance/assay"},
			},
			Apps: []App{{Role: "reviewer"}},
		}
		if got := examplePublishabilityFindings(clean); len(got) != 0 {
			t.Errorf("false positive: a clean example was flagged: %v", got)
		}
	})
}

// ---------------------------------------------------------------------------
// pure cores — so the positive controls can prove the checks discriminate
// ---------------------------------------------------------------------------

// exampleShapeDiffs reports every SHAPE the source states that the example does
// not demonstrate. It compares which sections are populated and which fields are
// exercised — never values.
//
// `apps[].id` and `humans[]` are absent from the table on purpose: they are the
// two disclosure-bearing shapes, and a source that states them must NOT be
// followed by the example. examplePublishabilityFindings owns them.
func exampleShapeDiffs(src, ex Topology) []string {
	shapes := []struct {
		name    string
		present func(Topology) bool
	}{
		{"release_repo", func(t Topology) bool { return t.ReleaseRepo != "" }},
		{"cell (the per-cell instance's own name)", func(t Topology) bool { return t.Cell != "" }},
		// STATED, not defaulted. Relationship's zero value IS upstream, so an
		// example that says nothing would otherwise read as "demonstrates
		// upstream" — a shape check satisfied by silence is not a shape check.
		{"repos[].relationship stated as owned", func(t Topology) bool {
			return anyRepo(t, func(r Repo) bool { return r.RelationshipStated && r.Relationship == RelationshipOwned })
		}},
		{"repos[].relationship stated as upstream", func(t Topology) bool {
			return anyRepo(t, func(r Repo) bool { return r.RelationshipStated && r.Relationship == RelationshipUpstream })
		}},
		{"repos", func(t Topology) bool { return len(t.Repos) > 0 }},
		{"repos[].visibility stated as public", func(t Topology) bool {
			return anyRepo(t, func(r Repo) bool { return r.Visibility == VisibilityPublic })
		}},
		{"repos[].visibility left unknown (the fail-closed default)", func(t Topology) bool {
			return anyRepo(t, func(r Repo) bool { return r.Visibility == VisibilityUnknown })
		}},
		{"repos[].root", func(t Topology) bool {
			return anyRepo(t, func(r Repo) bool { return r.Root != "" })
		}},
		{"repos[].risk_path_triggers", func(t Topology) bool {
			return anyRepo(t, func(r Repo) bool { return len(r.RiskPathTriggers) > 0 })
		}},
		{"repos[].note", func(t Topology) bool {
			return anyRepo(t, func(r Repo) bool { return r.Note != "" })
		}},
		{"apps", func(t Topology) bool { return len(t.Apps) > 0 }},
		{"apps[].purpose", func(t Topology) bool {
			for _, a := range t.Apps {
				if a.Purpose != "" {
					return true
				}
			}
			return false
		}},
		{"products", func(t Topology) bool { return len(t.Products) > 0 }},
		{"products[].summary", func(t Topology) bool {
			for _, p := range t.Products {
				if p.Summary != "" {
					return true
				}
			}
			return false
		}},
		{"products[].repos", func(t Topology) bool {
			for _, p := range t.Products {
				if len(p.Repos) > 0 {
					return true
				}
			}
			return false
		}},
		{"labels.system_state", func(t Topology) bool { return len(t.SystemStateLabels) > 0 }},
		{"labels.decision_owed", func(t Topology) bool { return len(t.DecisionOwedLabels) > 0 }},
		{"labels[].why", func(t Topology) bool {
			for _, l := range append(append([]Label(nil), t.SystemStateLabels...), t.DecisionOwedLabels...) {
				if l.Why != "" {
					return true
				}
			}
			return false
		}},
	}

	var out []string
	for _, s := range shapes {
		if s.present(src) && !s.present(ex) {
			out = append(out, fmt.Sprintf("the source states %s; the example does not demonstrate it", s.name))
		}
	}
	return out
}

// examplePublishabilityFindings reports everything in the example that must not
// reach the public tree: a repo slug outside the publishable set, an App id, or
// any human identity at all.
func examplePublishabilityFindings(ex Topology) []string {
	var out []string
	for _, r := range ex.Repos {
		if !publishableSlug(r.Slug) {
			out = append(out, fmt.Sprintf(
				"repos[%s]: not a publishable slug (allowed: %s)", r.Slug, strings.Join(publishableRepoPrefixes, ", ")))
		}
	}
	for _, p := range ex.Products {
		for _, slug := range p.Repos {
			if !publishableSlug(slug) {
				out = append(out, fmt.Sprintf(
					"products[%s].repos: %s is not a publishable slug", p.Name, slug))
			}
		}
	}
	if ex.ReleaseRepo != "" && !publishableSlug(ex.ReleaseRepo) {
		out = append(out, fmt.Sprintf("release_repo: %s is not a publishable slug", ex.ReleaseRepo))
	}
	for _, a := range ex.Apps {
		if a.IDSet {
			out = append(out, fmt.Sprintf(
				"apps[%s].id is SET — the App fleet inventory is operator configuration "+
					"(ASSAY_TRUSTED_BOT_SLUGS), never a published file", a.Role))
		}
	}
	if len(ex.Humans) > 0 {
		out = append(out, fmt.Sprintf(
			"humans states %d entr(y/ies) — the public example ships `humans: []`. "+
				"A human roster in a copied file publishes an org chart", len(ex.Humans)))
	}
	return out
}

func publishableSlug(slug string) bool {
	for _, p := range publishableRepoPrefixes {
		if slug == p || strings.HasPrefix(slug, p) {
			return true
		}
	}
	return false
}

func anyRepo(t Topology, pred func(Repo) bool) bool {
	for _, r := range t.Repos {
		if pred(r) {
			return true
		}
	}
	return false
}

func loadExampleOrFail(t *testing.T) Topology {
	t.Helper()
	path := filepath.Join(repoRoot, ExampleFile)
	ex, err := LoadFile(path)
	if err != nil {
		t.Fatalf("COULD-NOT-CHECK: %v\n"+
			"  %s is what the public tree ships in place of the withheld %s (Ian's 2026-08-13 "+
			"ruling). Without it this test verifies nothing, so it fails rather than passing "+
			"quietly — and a public tree whose topology source does not parse is a published "+
			"loader with nothing to read.", err, ExampleFile, SourceFile)
	}
	return ex
}
