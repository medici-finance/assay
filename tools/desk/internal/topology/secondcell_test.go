package topology

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// secondcell_test.go — the executed acceptance test for "adding a cell is
// CONFIG, not CODE" (brief desk-console-2/06).
//
// THE CLAIM UNDER TEST. The multi-cell adoption model rests on one property: a
// cell is DATA. Adding one should be a different instance of a config file and
// nothing else. That claim was asserted in prose (docs/adopting-assay.md
// § "Multi-cell topology contract", topology.example.yaml's header) before
// anything executed it, and an unexecuted architecture claim is exactly the
// class this repo's three-state instrument rule exists to catch.
//
// SO THIS FILE FALSIFIES IT RATHER THAN ILLUSTRATING IT. It measures the claim
// in the two places it can be false, and they answer DIFFERENTLY:
//
//	SCHEMA LAYER  — can a second cell be EXPRESSED as config alone?
//	                YES, and TestSecondCellIsExpressibleAsConfig executes it: a
//	                real second cell's instance, parsed by the SHIPPED reader,
//	                serving the second cell's facts and none of this cell's.
//
//	RUNTIME LAYER — do the shipped BINARIES read that config?
//	                NO. Every runtime consumer reads topology.Compiled() — the
//	                Go derivation of THIS cell's file — and LoadFile has zero
//	                non-test callers. So a second cell's edited topology.yaml
//	                changes nothing any desk tool does until compiled.go is
//	                edited and the binaries are rebuilt. That is a CODE change,
//	                and TestSecondCellRuntimeCapIsDeclaredAndNamed pins it.
//
// WHY THE RUNTIME CAP IS PINNED RATHER THAN "FIXED". Compiling the derivation in
// is a deliberate, defended decision (compiled.go's header, and the example
// file's "HOW CONSUMERS READ IT"): these tools ship as pinned standalone
// binaries run from an arbitrary working directory, and a gate that needs the
// filesystem is a gate that fails open when the filesystem is not the repo.
// The defect is not the compilation — it is the cap going UNNAMED while the
// adopter contract says "copy the example and edit it in place". So the check
// asserts the MEASURED boundary equals the DECLARED boundary, and reddens when
// they diverge in EITHER direction: a new compiled-topology consumer that
// nobody named, or a runtime file-read wired in without the cap shrinking.
//
// WHAT THIS FILE DOES NOT CLAIM, stated rather than implied. It does not prove a
// second cell WORKS end-to-end — no second cell is deployed, and no board, inbox
// or portfolio render is exercised here. Those parts of desk-console-2/06's
// original Verify table are COULD-NOT-CHECK: they name `cells.yaml` and a
// `deskcli` binary in the console repo, neither of which exists in any tree this
// suite can reach. What it proves is narrower and executable: the config layer
// admits a second cell, and the runtime layer does not read it.

// secondCellFixture is a COMPLETE second cell's topology instance — a different
// `cell:`, its own owned repo, and `medici-finance/assay` flipped to `upstream`,
// which are exactly the two edits topology.example.yaml's header instructs an
// adopting cell to make. It is a fixture, not a second declared source: no
// derivation is taken from it and it states nothing about this repo.
const secondCellFixture = "tools/desk/internal/topology/testdata/second-cell.topology.yaml"

// adopterContract is the document an adopting cell actually reads. A cap named
// only in a Go comment is a cap the adopter never sees, so the naming half of
// the runtime check asserts against THIS file. It is `copy` disposition, so the
// assertion holds in the public tree too.
const adopterContract = "docs/adopting-assay.md"

// thisCell is the cell this tree's topology.yaml speaks for. Named here so the
// leak check below can assert the second cell's read is not contaminated by it.
const thisCell = "assay"

// ---------------------------------------------------------------------------
// SCHEMA LAYER — a second cell IS expressible as config alone.
// ---------------------------------------------------------------------------

// TestSecondCellIsExpressibleAsConfig parses a second cell's instance with the
// shipped reader and asserts every cell-varying fact comes back as the SECOND
// cell's. No new code path is involved: it calls LoadFile, the same entry point
// the drift check uses on the real source.
func TestSecondCellIsExpressibleAsConfig(t *testing.T) {
	path := filepath.Join(repoRoot, filepath.FromSlash(secondCellFixture))
	second, err := LoadFile(path)
	if err != nil {
		// COULD-NOT-CHECK. A fixture that will not parse says nothing about the
		// claim either way, so this fails rather than passing quietly.
		t.Fatalf("COULD-NOT-CHECK: parsing %s: %v\n"+
			"  This fixture IS the config-not-code claim's subject. Without it the test\n"+
			"  verifies nothing — it does not report green.", secondCellFixture, err)
	}

	t.Run("the shipped reader serves the second cell's identity", func(t *testing.T) {
		if second.Cell != "example-lending" {
			t.Errorf("cell = %q, want %q — the reader did not serve the fixture's own identity", second.Cell, "example-lending")
		}
		if second.Cell == thisCell {
			t.Errorf("cell = %q, which is THIS tree's cell — a second cell that reads back as the first "+
				"is not a second cell", thisCell)
		}
		if second.Schema != Schema {
			t.Errorf("schema = %q, want %q — a second cell uses the same schema, not a forked one", second.Schema, Schema)
		}
	})

	t.Run("edit 2 — the toolkit is upstream to this cell", func(t *testing.T) {
		r, ok := second.Repo("medici-finance/assay")
		if !ok {
			t.Fatal("the fixture does not state medici-finance/assay — the load-bearing case of the " +
				"one-owner boundary is exactly that every cell but one states it `upstream`")
		}
		if r.Relationship != RelationshipUpstream {
			t.Errorf("medici-finance/assay relationship = %s, want upstream", r.Relationship)
		}
		if !r.RelationshipStated {
			t.Error("medici-finance/assay's `upstream` is not STATED — the example header instructs the " +
				"adopter to write the flip, and a default that happens to agree does not demonstrate " +
				"the edit was made")
		}
	})

	t.Run("absent relationship reads as upstream and is distinguishable from stated", func(t *testing.T) {
		r, ok := second.Repo("example-org/loans-docs")
		if !ok {
			t.Fatal("the fixture lost its absent-relationship entry — the default-reading arm of the claim is untested without it")
		}
		if r.Relationship != RelationshipUpstream {
			t.Errorf("absent relationship read as %s — absent must be the LEAST authority (upstream)", r.Relationship)
		}
		if r.RelationshipStated {
			t.Error("an ABSENT relationship reported as STATED — the flag would then say nothing, and " +
				"the shape checks that depend on it would pass for the wrong reason")
		}
	})

	t.Run("the owned set is this cell's, not the toolkit cell's", func(t *testing.T) {
		got := second.OwnedRepos()
		want := []string{"example-org/loans"}
		if !equalStringSlices(got, want) {
			t.Errorf("OwnedRepos() = %v, want %v", got, want)
		}
		for _, slug := range compiledOwnedSlugs() {
			if slug == "medici-finance/assay" {
				// Deliberately allowed to appear in BOTH files — the point is
				// that only one states it `owned`, asserted above.
				continue
			}
			if _, ok := second.Repo(slug); ok {
				t.Errorf("the second cell's read contains %s, which is a repo only THIS cell's "+
					"topology states — a leak from the compiled derivation into a file read", slug)
			}
		}
	})

	t.Run("the whole derivation is the second cell's", func(t *testing.T) {
		if second.ReleaseRepo == Compiled().ReleaseRepo {
			t.Errorf("release_repo = %q for both cells — the fixture is not exercising a DIFFERENT cell", second.ReleaseRepo)
		}
		roots := second.Roots()
		if roots["example-org/loans"] != "." {
			t.Errorf("Roots()[example-org/loans] = %q, want %q", roots["example-org/loans"], ".")
		}
		if len(second.SystemStateLabelNames()) == 0 {
			t.Error("the second cell's system-state label set is empty — a cell states the labels it " +
				"reasons about; inheriting them silently is the hand-copy defect")
		}
	})

	t.Run("positive control — the comparison discriminates", func(t *testing.T) {
		// A second cell whose file was NOT actually edited (still this tree's
		// cell name, still `owned` on the toolkit) must be REPORTED as not a
		// second cell. Without this, every assertion above could be passing
		// because the reader returns whatever it is handed.
		unedited := Topology{Schema: Schema, Cell: thisCell, Repos: []Repo{
			{Slug: "medici-finance/assay", Relationship: RelationshipOwned, RelationshipStated: true},
		}}
		gaps := secondCellGaps(unedited)
		if len(gaps) == 0 {
			t.Error("an UNEDITED copy of this cell's file was reported as a valid second cell — the " +
				"check does not discriminate and its green above means nothing")
		}
		if gaps := secondCellGaps(second); len(gaps) != 0 {
			t.Errorf("the real fixture was reported as NOT a second cell: %v — the control is inverted", gaps)
		}
	})
}

// secondCellGaps is the pure core of "is this actually a DIFFERENT cell's
// instance?", extracted so the positive control can prove it discriminates. A
// check that cannot be shown to fail is not a check.
func secondCellGaps(t Topology) []string {
	var out []string
	if t.Cell == "" {
		out = append(out, "states no `cell:` — an instance with no identity")
	}
	if t.Cell == thisCell {
		out = append(out, "states cell "+thisCell+" — that is THIS tree's cell, not a second one")
	}
	if r, ok := t.Repo("medici-finance/assay"); ok && r.Relationship == RelationshipOwned {
		out = append(out, "states medici-finance/assay `owned` — exactly one cell may, and it is not this one")
	}
	return out
}

// compiledOwnedSlugs is this tree's compiled repo set, used to detect a leak of
// this cell's topology into a read of another cell's file.
func compiledOwnedSlugs() []string { return Compiled().RepoSlugs() }

// ---------------------------------------------------------------------------
// RUNTIME LAYER — the cap: config alone does not reach the shipped binaries.
// ---------------------------------------------------------------------------

// runtimeCapSite is one place a desk binary reads the COMPILED derivation of
// this cell's topology.yaml rather than reading a file. Every entry here is a
// place a second cell's edited topology.yaml has NO effect until compiled.go is
// edited and the binaries are rebuilt.
type runtimeCapSite struct {
	// file is the repo-relative non-test Go source holding the call.
	file string
	// calls is how many topology.Compiled() call sites the file holds. Counted,
	// not "at least one": a second call appearing in a file already on the list
	// is still cap growth, and a list that only asked "does it appear" would
	// wave it through.
	calls int
	// what names the fact that stays fixed at build time, in the adopter's terms.
	what string
}

// compiledTopologyCapSites is the DECLARED boundary — the honest, complete list
// of what a second cell does NOT get by editing config. It is hand-written on
// purpose: WHICH fact each site freezes is a human judgement, and the scan below
// forces the list to stay true to the tree.
//
// TO CHANGE THIS LIST: change the code first, then this table, then the
// corresponding lines in docs/adopting-assay.md. The naming check reads that
// document, so a cap that grows without the adopter being told is a red test.
func compiledTopologyCapSites() []runtimeCapSite {
	return []runtimeCapSite{
		{
			file:  "tools/desk/cmd/issueboard/board.go",
			calls: 2,
			what:  "the system-state and decision-owed label sets the board excludes and escalates on",
		},
		{
			file:  "tools/desk/cmd/deskroster/sets.go",
			calls: 2,
			what:  "the cell name, per-repo relationship and App roles `deskroster repos` prints",
		},
		{
			file:  "tools/desk/cmd/deskrelease/cut.go",
			calls: 1,
			what:  "the default repo `deskrelease` cuts a release from",
		},
		{
			file:  "tools/desk/internal/deskkit/riskpath.go",
			calls: 1,
			what:  "per-repo visibility and risk-path triggers, which decide a diff's risk class",
		},
		{
			file:  "tools/desk/internal/deskkit/roots.go",
			calls: 1,
			what:  "the repo -> local checkout root map the multi-repo board walks",
		},
	}
}

// TestSecondCellRuntimeCapIsDeclaredAndNamed is the falsifier. It executes the
// "config not code" claim against the tree and fails when the measured boundary
// and the declared one disagree — in EITHER direction.
func TestSecondCellRuntimeCapIsDeclaredAndNamed(t *testing.T) {
	measured, loadFileCallers, err := scanCompiledTopologyUse()
	if err != nil {
		t.Fatalf("COULD-NOT-CHECK: scanning tools/desk for compiled-topology use: %v\n"+
			"  A scan that could not run is not a clean tree.", err)
	}

	t.Run("the compiled-topology call sites are exactly the declared cap", func(t *testing.T) {
		for _, d := range capDiffs(compiledTopologyCapSites(), measured) {
			t.Errorf("CAP DRIFT — %s\n"+
				"  This list is the answer to \"what does a second cell NOT get by editing config?\".\n"+
				"  A site missing from it is a SILENT CAP: an adopting cell would edit topology.yaml,\n"+
				"  see no change, and have nothing to read that told them why.\n"+
				"  Update compiledTopologyCapSites AND the matching lines in %s. Do not delete the row.",
				d, adopterContract)
		}
	})

	t.Run("no runtime path reads the topology file", func(t *testing.T) {
		// This is the assertion that makes the cap a CAP rather than a
		// preference. If it ever goes red, that is GOOD NEWS that still has to
		// be declared: someone wired a runtime read, so the cap shrank and both
		// this table and the adopter contract must shrink with it.
		if loadFileCallers != 0 {
			t.Errorf("topology.LoadFile now has %d non-test caller(s) in tools/desk.\n"+
				"  The cap this file pins is that it has NONE — that a shipped binary never reads\n"+
				"  topology.yaml, so a second cell's edits do not reach it. If a runtime read was\n"+
				"  added deliberately, SHRINK compiledTopologyCapSites and the corresponding\n"+
				"  paragraph in %s to match. Do not adjust this number to make the test pass.",
				loadFileCallers, adopterContract)
		}
	})

	t.Run("every capped site is named in the adopter contract", func(t *testing.T) {
		docPath := filepath.Join(repoRoot, filepath.FromSlash(adopterContract))
		raw, err := os.ReadFile(docPath)
		if err != nil {
			t.Fatalf("COULD-NOT-CHECK: reading %s: %v\n"+
				"  The adopter contract is where a cap is NAMED. Unable to read it, this test cannot\n"+
				"  tell a named cap from a silent one, so it fails rather than passing quietly.",
				adopterContract, err)
		}
		for _, gap := range capNamingGaps(compiledTopologyCapSites(), string(raw)) {
			t.Errorf("UNNAMED CAP — %s\n"+
				"  %s tells an adopting cell to copy topology.example.yaml and edit it in place.\n"+
				"  That instruction is incomplete for any fact frozen at build time, and an\n"+
				"  incomplete instruction in a front-door runbook is a silent cap.",
				gap, adopterContract)
		}
	})

	t.Run("positive control — the cap checks discriminate", func(t *testing.T) {
		declared := compiledTopologyCapSites()

		// (a) a NEW consumer nobody declared.
		grown := map[string]int{}
		for _, d := range declared {
			grown[d.file] = d.calls
		}
		grown["tools/desk/cmd/example/new.go"] = 1
		if len(capDiffs(declared, grown)) == 0 {
			t.Error("an undeclared compiled-topology consumer was not reported — cap growth would be silent")
		}

		// (b) a declared site whose call count moved.
		moved := map[string]int{}
		for _, d := range declared {
			moved[d.file] = d.calls
		}
		moved[declared[0].file] = declared[0].calls + 1
		if len(capDiffs(declared, moved)) == 0 {
			t.Error("a changed call count was not reported — a second freeze in an already-listed file would be silent")
		}

		// (c) a site the tree no longer holds, left stale in the table.
		shrunk := map[string]int{}
		for _, d := range declared[1:] {
			shrunk[d.file] = d.calls
		}
		if len(capDiffs(declared, shrunk)) == 0 {
			t.Error("a stale declared site was not reported — the table could outlive the code and still read green")
		}

		// (d) a contract that names none of them.
		if len(capNamingGaps(declared, "a document that names nothing")) == 0 {
			t.Error("a contract naming no capped site was reported as complete — the naming check does not discriminate")
		}
		// ...and one that names them all must be clean, or (d) passes for the wrong reason.
		var all strings.Builder
		for _, d := range declared {
			all.WriteString(d.file + "\n")
		}
		all.WriteString(capContractAnchor + "\n")
		if gaps := capNamingGaps(declared, all.String()); len(gaps) != 0 {
			t.Errorf("a contract naming every capped site was still reported incomplete: %v", gaps)
		}
	})
}

// capContractAnchor is the heading the adopter contract must carry. Matching the
// file paths alone would be satisfied by a stray mention anywhere in the
// document; the anchor asserts there is a SECTION about it.
const capContractAnchor = "What config alone does NOT cover"

// capDiffs is the pure core of the cap comparison, so the positive control can
// prove it discriminates. It reports every disagreement between the declared cap
// table and what the tree actually holds.
func capDiffs(declared []runtimeCapSite, measured map[string]int) []string {
	var out []string
	declaredBy := map[string]int{}
	for _, d := range declared {
		declaredBy[d.file] = d.calls
		got, ok := measured[d.file]
		if !ok {
			out = append(out, fmt.Sprintf("declared cap site %s holds no topology.Compiled() call — "+
				"the code moved and the table did not", d.file))
			continue
		}
		if got != d.calls {
			out = append(out, fmt.Sprintf("%s: declared %d topology.Compiled() call(s), tree holds %d",
				d.file, d.calls, got))
		}
	}
	var extra []string
	for file := range measured {
		if _, ok := declaredBy[file]; !ok {
			extra = append(extra, file)
		}
	}
	sort.Strings(extra)
	for _, file := range extra {
		out = append(out, fmt.Sprintf("UNDECLARED compiled-topology consumer: %s (%d call(s)) — "+
			"a fact this file freezes at build time is a fact a second cell cannot change with config",
			file, measured[file]))
	}
	return out
}

// capNamingGaps is the pure core of the contract-naming check.
func capNamingGaps(declared []runtimeCapSite, doc string) []string {
	var out []string
	if !strings.Contains(doc, capContractAnchor) {
		out = append(out, fmt.Sprintf("the contract carries no %q section — the caps may be scattered "+
			"through it, but an adopter has nowhere to read them as a list", capContractAnchor))
	}
	for _, d := range declared {
		if !strings.Contains(doc, d.file) {
			out = append(out, fmt.Sprintf("%s is capped (%s) but the contract never names it", d.file, d.what))
		}
	}
	return out
}

// scanCompiledTopologyUse measures the tree: how many topology.Compiled() calls
// each non-test file under tools/desk holds, and how many non-test callers
// topology.LoadFile has.
//
// It counts the QUALIFIED forms only. Inside this package Compiled and LoadFile
// are called unqualified, and this package's own definitions are not consumers —
// counting them would make the measurement disagree with the question being
// asked ("which BINARIES freeze this cell's topology?").
func scanCompiledTopologyUse() (compiledCalls map[string]int, loadFileCallers int, err error) {
	files, err := goSourceFiles(repoRoot)
	if err != nil {
		return nil, 0, err
	}
	if len(files) < 50 {
		// A walk that found almost nothing scanned almost nothing; reporting
		// that as a clean boundary is the vacuous green this stream exists to end.
		return nil, 0, fmt.Errorf("the tree walk found only %d Go files — that is not this repo, "+
			"so this scan proves nothing", len(files))
	}
	compiledCalls = map[string]int{}
	seenDeskFile := false
	for _, f := range files {
		rel := relTo(f)
		if !strings.HasPrefix(rel, "tools/desk/") {
			continue
		}
		seenDeskFile = true
		raw, rerr := os.ReadFile(f)
		if rerr != nil {
			return nil, 0, fmt.Errorf("reading %s: %w", rel, rerr)
		}
		src := string(raw)
		src = stripLineComments(src)
		if n := strings.Count(src, "topology.Compiled()"); n > 0 {
			compiledCalls[rel] = n
		}
		loadFileCallers += strings.Count(src, "topology.LoadFile(")
	}
	if !seenDeskFile {
		return nil, 0, fmt.Errorf("the walk matched no tools/desk/ source at all — re-point the scan")
	}
	return compiledCalls, loadFileCallers, nil
}

// stripLineComments removes `//` line comments so a call named in PROSE — and
// compiled.go's header names both symbols repeatedly — is not counted as a call
// site. Block comments are not stripped: tools/desk holds none around these
// symbols, and a stripper that mishandled one would under-count, which is the
// fail-open direction. If that ever changes, extend this rather than loosening
// the assertion.
func stripLineComments(src string) string {
	lines := strings.Split(src, "\n")
	for i, ln := range lines {
		if idx := strings.Index(ln, "//"); idx >= 0 {
			// Only strip when the `//` is not inside a string literal. A crude
			// but sufficient test here: no odd number of quotes before it.
			if strings.Count(ln[:idx], `"`)%2 == 0 {
				lines[i] = ln[:idx]
			}
		}
	}
	return strings.Join(lines, "\n")
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
