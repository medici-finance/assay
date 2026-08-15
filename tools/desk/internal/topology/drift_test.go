package topology

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// drift_test.go — the enforcement half of ground-truth/04.
//
// topology.yaml is the ONE declared source; compiled.go is this module's
// derivation; statusgen/topologyvalues.go is statusgen's. Nothing stops a future
// edit from re-introducing a sixth hand table except a check that goes red and
// NAMES THE SITE. This file is that check, in three parts:
//
//	part 1  the derivation matches the declared source, field by field
//	part 2  every RETIRED table site is GONE — not shadowed, not commented out
//	part 3  no NEW topology hand table exists anywhere outside the loader
//
// THREE-STATE. Every read here is checked. A source it cannot read, a tree it
// cannot walk, a file it cannot open — each is a FAILURE naming what could not
// be checked, never a pass. A checker that could not look never reports green
// (docs/streams/ground-truth/README.md).
//
// PROOF IT CAN FAIL. Part 2 and part 3 both carry positive controls
// (TestTopologyDriftRegistryPositiveControl) that feed the same detectors a
// deliberately re-introduced hand table and assert they go red. A check that has
// never been observed failing is an assertion, not a check.

// repoRoot is this package's path back to the repo root
// (tools/desk/internal/topology).
const repoRoot = "../../../.."

// retiredSite is one hand table this brief RETIRED. The registry is the
// enumeration the brief asks for: a surviving legacy table is a test failure
// that names the site.
type retiredSite struct {
	// file is repo-relative.
	file string
	// literal is the exact source text the retired table opened with. It must no
	// longer appear anywhere in the file.
	literal string
	// replacement names where the fact lives now, so the failure message tells
	// the reader what to do instead of only what is wrong.
	replacement string
	// issue is the issue the site's drift was filed as.
	issue string
}

// retiredTableSites is the registry. Adding a row RETIRES a site; removing one
// is a deliberate act a reviewer must weigh, not a cleanup.
func retiredTableSites() []retiredSite {
	return []retiredSite{
		{
			file:        "statusgen/scanissues.go",
			literal:     "var scanExcludedLabels = map[string]bool{",
			replacement: "statusgen/topologyvalues.go's topologySystemStateLabels, derived from topology.yaml labels.system_state",
			issue:       "#829",
		},
		{
			file:        "tools/desk/cmd/issueboard/board.go",
			literal:     "var excludedLabels = map[string]bool{",
			replacement: "topology.Compiled().SystemStateLabelSet(), derived from topology.yaml labels.system_state",
			issue:       "#829",
		},
		{
			file:        "tools/desk/cmd/issueboard/board.go",
			literal:     "var decisionLabels = map[string]bool{",
			replacement: "topology.Compiled().DecisionOwedLabelSet(), derived from topology.yaml labels.decision_owed",
			issue:       "#829",
		},
		{
			file:        "tools/desk/internal/deskkit/roots.go",
			literal:     "var defaultRoots = map[string]string{",
			replacement: "topology.Compiled().Roots(), derived from topology.yaml repos[].root",
			issue:       "#276",
		},
		{
			file:        "tools/desk/internal/deskkit/riskpath.go",
			literal:     "var repoRiskPathTriggers = map[string][]string{",
			replacement: "topology.Compiled().RiskPathTriggersByRepo(), derived from topology.yaml repos[].risk_path_triggers",
			issue:       "#276",
		},
		{
			file:        "tools/desk/cmd/deskrelease/cut.go",
			literal:     `const defaultReleaseRepo = "medici-finance/assay"`,
			replacement: "topology.Compiled().ReleaseRepo, derived from topology.yaml release_repo",
			issue:       "#276",
		},
	}
}

// TestTopologyDriftRegistry is the check Verify row 6 runs.
func TestTopologyDriftRegistry(t *testing.T) {
	t.Run("derivation matches the declared source", func(t *testing.T) {
		src := loadSourceOrFail(t)
		compareTopology(t, src, Compiled())
	})

	t.Run("every retired table site is gone", func(t *testing.T) {
		for _, site := range retiredTableSites() {
			path := filepath.Join(repoRoot, filepath.FromSlash(site.file))
			raw, err := os.ReadFile(path)
			if err != nil {
				// COULD-NOT-CHECK. A site file that vanished is either a rename
				// this registry must follow or a deletion someone must confirm;
				// neither is a pass.
				t.Errorf("COULD-NOT-CHECK: %s (retired %s site, %s) is unreadable: %v\n"+
					"  If the file MOVED, re-point this registry row. If it was DELETED, delete the row deliberately.",
					site.file, site.issue, site.literal, err)
				continue
			}
			if strings.Contains(string(raw), site.literal) {
				t.Errorf("SURVIVING HAND TABLE at %s: %s\n"+
					"  This table was retired by ground-truth/04 (%s). The fact now lives in %s,\n"+
					"  whose single declared source is %s. A second hand-maintained copy is the\n"+
					"  defect whatever it contains — it will drift, and the drift becomes its own issue.\n"+
					"  Delete the table and read the loader; do not weaken this check.",
					site.file, site.literal, site.issue, site.replacement, SourceFile)
			}
		}
	})

	t.Run("no new hand table outside the loader", func(t *testing.T) {
		src := loadSourceOrFail(t)
		files, err := goSourceFiles(filepath.Join(repoRoot))
		if err != nil {
			t.Fatalf("COULD-NOT-CHECK: walking the tree for Go sources: %v", err)
		}
		if len(files) < 50 {
			// A walk that found almost nothing scanned almost nothing. Reporting
			// that as clean is the vacuous-green failure this stream exists to end.
			t.Fatalf("COULD-NOT-CHECK: the tree walk found only %d Go files — that is not this repo, so this scan proves nothing", len(files))
		}
		tokens := topologyTokens(src)
		optOutSeen := map[string]bool{}
		for _, f := range files {
			raw, err := os.ReadFile(f)
			if err != nil {
				t.Errorf("COULD-NOT-CHECK: reading %s: %v", relTo(f), err)
				continue
			}
			for ident := range handTableOptOut {
				if strings.Contains(string(raw), ident+" = ") {
					optOutSeen[ident] = true
				}
			}
			for _, hit := range scanForHandTables(string(raw), tokens) {
				t.Errorf("NEW TOPOLOGY HAND TABLE at %s:%d — %s\n"+
					"  It states topology facts (%s) that already have a declared source: %s.\n"+
					"  Read the loader (tools/desk/internal/topology for the desk module,\n"+
					"  statusgen/topologyvalues.go for statusgen) instead of restating them.\n"+
					"  If this literal is legitimately NOT topology, add it to handTableOptOut with a reason.",
					relTo(f), hit.line, hit.opener, strings.Join(hit.tokens, ", "), SourceFile)
			}
		}
		// A stale opt-out is how an exemption outlives the thing it exempted and
		// then silently covers something new that happens to reuse the name.
		for ident, why := range handTableOptOut {
			if !optOutSeen[ident] {
				t.Errorf("STALE OPT-OUT: handTableOptOut names %q (%s), but no literal declares it anywhere in the tree.\n"+
					"  Delete the row deliberately, or re-point it — an exemption for something that no longer exists "+
					"is an exemption waiting to cover the wrong thing.", ident, why)
			}
		}
	})
}

// TestTopologyDriftRegistryPositiveControl is the proof the registry can fail.
// It feeds each detector a deliberately re-introduced hand table and asserts a
// red result. Without it, a green TestTopologyDriftRegistry would be consistent
// with a detector that matches nothing.
func TestTopologyDriftRegistryPositiveControl(t *testing.T) {
	src := loadSourceOrFail(t)
	tokens := topologyTokens(src)

	t.Run("retired-site detector catches a re-introduced table", func(t *testing.T) {
		// The exact text of the table this brief deleted from issueboard.
		reintroduced := "// excludedLabels are the system-emitted labels …\n" +
			"var excludedLabels = map[string]bool{\n\t\"verify-gate\": true,\n}\n"
		var site retiredSite
		for _, s := range retiredTableSites() {
			if s.file == "tools/desk/cmd/issueboard/board.go" && strings.Contains(s.literal, "excludedLabels") {
				site = s
			}
		}
		if site.literal == "" {
			t.Fatal("registry no longer carries the issueboard excludedLabels row — the control has nothing to prove against")
		}
		if !strings.Contains(reintroduced, site.literal) {
			t.Fatalf("POSITIVE CONTROL FAILED: the retired-site detector did NOT match a verbatim re-introduction of %s.\n"+
				"The registry row is therefore unfailable: it would pass over the exact table it retired.", site.literal)
		}
	})

	t.Run("hand-table scanner catches a new table", func(t *testing.T) {
		reintroduced := "package main\n\n" +
			"var myOwnLabels = map[string]bool{\n" +
			"\t\"verify-gate\":    true,\n" +
			"\t\"needs-decision\": true,\n" +
			"}\n"
		hits := scanForHandTables(reintroduced, tokens)
		if len(hits) == 0 {
			t.Fatal("POSITIVE CONTROL FAILED: the hand-table scanner did not flag a fresh map literal " +
				"restating two system-state labels. An unfailable scanner reports green over the defect it exists to catch.")
		}
	})

	t.Run("drift detector catches a changed derivation", func(t *testing.T) {
		bent := Compiled()
		bent.SystemStateLabels = bent.SystemStateLabels[:len(bent.SystemStateLabels)-1]
		diffs := topologyDiffs(src, bent)
		if len(diffs) == 0 {
			t.Fatal("POSITIVE CONTROL FAILED: dropping a system-state label from the derivation produced NO diff " +
				"against the declared source — the comparison is vacuous.")
		}
	})

	t.Run("scanner ignores prose and unrelated literals", func(t *testing.T) {
		// A false positive here would push the next author to weaken the check,
		// so the control asserts the negative direction too.
		benign := "package main\n\n" +
			"const usage = `deskboard — lists medici-finance/assay and example-org/tracker`\n\n" +
			"var httpCodes = map[string]bool{\"200\": true, \"404\": true}\n"
		if hits := scanForHandTables(benign, tokens); len(hits) != 0 {
			t.Errorf("false positive: scanner flagged %+v in prose/unrelated literals", hits)
		}
	})
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func loadSourceOrFail(t *testing.T) Topology {
	t.Helper()
	path := filepath.Join(repoRoot, SourceFile)
	src, err := LoadFile(path)
	if err != nil {
		t.Fatalf("COULD-NOT-CHECK: %v\n"+
			"  %s is the ONE declared source for repo/App/product topology. Without it this test "+
			"verifies nothing, so it fails rather than passing quietly.", err, SourceFile)
	}
	return src
}

// compareTopology reports every disagreement between the declared source and a
// derivation, naming the field.
func compareTopology(t *testing.T, src, derived Topology) {
	t.Helper()
	for _, d := range topologyDiffs(src, derived) {
		t.Errorf("DERIVATION DRIFT — %s\n"+
			"  tools/desk/internal/topology/compiled.go disagrees with %s.\n"+
			"  Edit the SOURCE first, then mirror it into compiled.go. The source wins.",
			d, SourceFile)
	}
}

// topologyDiffs is compareTopology's pure core, so the positive control can
// assert the comparison actually discriminates.
func topologyDiffs(src, derived Topology) []string {
	var out []string
	add := func(format string, args ...any) { out = append(out, fmt.Sprintf(format, args...)) }

	if src.Schema != derived.Schema {
		add("schema: source %q, derivation %q", src.Schema, derived.Schema)
	}
	if src.ReleaseRepo != derived.ReleaseRepo {
		add("release_repo: source %q, derivation %q", src.ReleaseRepo, derived.ReleaseRepo)
	}
	// cell — one topology file per cell, so the derivation states WHOSE file it
	// was taken from. A derivation carrying another cell's name is not a stale
	// value, it is a claim about the wrong org.
	if src.Cell != derived.Cell {
		add("cell: source %q, derivation %q", src.Cell, derived.Cell)
	}
	if got, want := derived.RepoSlugs(), src.RepoSlugs(); !reflect.DeepEqual(got, want) {
		add("repos: source %v, derivation %v", want, got)
	}
	for _, want := range src.Repos {
		got, ok := derived.Repo(want.Slug)
		if !ok {
			add("repos[%s]: stated in the source, ABSENT from the derivation", want.Slug)
			continue
		}
		if got.Visibility != want.Visibility {
			add("repos[%s].visibility: source %s, derivation %s", want.Slug, want.Visibility, got.Visibility)
		}
		if got.CIRequired != want.CIRequired {
			add("repos[%s].ci: source %v, derivation %v", want.Slug, want.CIRequired, got.CIRequired)
		}
		if got.Root != want.Root {
			add("repos[%s].root: source %q, derivation %q", want.Slug, want.Root, got.Root)
		}
		if !reflect.DeepEqual(nonNil(got.RiskPathTriggers), nonNil(want.RiskPathTriggers)) {
			add("repos[%s].risk_path_triggers: source %v, derivation %v", want.Slug, want.RiskPathTriggers, got.RiskPathTriggers)
		}
		// relationship — the VALUE only. RelationshipStated is parse metadata a
		// derivation cannot carry (nothing is "absent" in a Go literal), so
		// comparing it would make every derivation permanently wrong.
		if got.Relationship != want.Relationship {
			add("repos[%s].relationship: source %s, derivation %s", want.Slug, want.Relationship, got.Relationship)
		}
	}
	if got, want := derived.AppRoles(), src.AppRoles(); !reflect.DeepEqual(got, want) {
		add("apps: source roles %v, derivation %v", want, got)
	}
	for i, want := range src.Apps {
		if i >= len(derived.Apps) {
			break
		}
		got := derived.Apps[i]
		if got.IDSet != want.IDSet || got.ID != want.ID {
			add("apps[%s].id: source set=%v id=%d, derivation set=%v id=%d", want.Role, want.IDSet, want.ID, got.IDSet, got.ID)
		}
	}
	if len(src.Humans) != len(derived.Humans) {
		add("humans: source has %d, derivation has %d", len(src.Humans), len(derived.Humans))
	}
	if got, want := productNames(derived), productNames(src); !reflect.DeepEqual(got, want) {
		add("products: source %v, derivation %v", want, got)
	}
	for _, want := range src.Products {
		for _, got := range derived.Products {
			if got.Name != want.Name {
				continue
			}
			if !reflect.DeepEqual(nonNil(got.Repos), nonNil(want.Repos)) {
				add("products[%s].repos: source %v, derivation %v", want.Name, want.Repos, got.Repos)
			}
		}
	}
	if got, want := derived.SystemStateLabelNames(), src.SystemStateLabelNames(); !reflect.DeepEqual(got, want) {
		add("labels.system_state: source %v, derivation %v", want, got)
	}
	if got, want := derived.DecisionOwedLabelNames(), src.DecisionOwedLabelNames(); !reflect.DeepEqual(got, want) {
		add("labels.decision_owed: source %v, derivation %v", want, got)
	}
	return out
}

func nonNil(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func productNames(t Topology) []string {
	out := make([]string, 0, len(t.Products))
	for _, p := range t.Products {
		out = append(out, p.Name)
	}
	sort.Strings(out)
	return out
}

// topologyTokens is the vocabulary the hand-table scanner looks for: every fact
// the declared source states that a second copy would have to restate.
func topologyTokens(t Topology) []string {
	var out []string
	out = append(out, t.RepoSlugs()...)
	out = append(out, t.SystemStateLabelNames()...)
	out = append(out, t.DecisionOwedLabelNames()...)
	sort.Strings(out)
	return out
}

type handTableHit struct {
	line   int
	opener string
	tokens []string
}

// handTableOptOut names composite literals that DO restate topology tokens but
// are legitimately not a second source. Every entry states why. The opt-out is
// keyed on the declaring identifier, so it cannot silence a whole file.
// Every entry names a DERIVATION that is bound to the declared source by its own
// diff test. That binding is the whole reason it is not a second source, so each
// reason states WHICH test binds it — an opt-out whose reason is "it's fine"
// is how the registry stops meaning anything.
var handTableOptOut = map[string]string{
	"compiled": "the desk module's derivation — bound by part 1 of this test " +
		"(TestTopologyDriftRegistry/derivation matches the declared source)",
	"topologySystemStateLabels": "statusgen's derivation — bound by " +
		"statusgen's TestTopologyValuesMatchSource, which parses topology.yaml and fails on drift. " +
		"statusgen is a separate Go module that ships as a pinned binary running against an arbitrary " +
		"--root, so it cannot read the source at run time and must carry a derivation",
	"topologyDecisionOwedLabels": "statusgen's derivation — same binding as " +
		"topologySystemStateLabels (TestTopologyValuesMatchSource)",
}

// scanForHandTables finds composite literals that restate two or more distinct
// topology tokens. Two is the threshold on purpose: one repo slug or one label
// name in a literal is ordinary code (a test fixture, a single-repo constant),
// while two together is a TABLE — the shape that drifts.
//
// LIMITS, stated rather than implied. This is a line scanner, not a Go parser:
// it recognises `<ident> = map[...]{` and `<ident> = []...{` openers and reads
// to the closing brace at the opener's indentation. It does not see literals
// built by function calls, appended to in a loop, or spread across files. It is
// a forcing function, not an oracle — the retired-site registry above is the
// backstop for the sites that actually drifted, and a reviewer remains the
// backstop for novel shapes. Where it cannot tell, it stays SILENT rather than
// guessing: a false red here teaches the next author to weaken it.
func scanForHandTables(content string, tokens []string) []handTableHit {
	lines := strings.Split(content, "\n")
	var hits []handTableHit
	for open := 0; open < len(lines); open++ {
		ident, ok := literalOpener(lines[open])
		if !ok {
			continue
		}
		indent := len(lines[open]) - len(strings.TrimLeft(lines[open], " \t"))

		// Body = every line up to the closing brace at the opener's indent. A
		// one-line literal (`x = map[string]bool{"a": true}`) has no body lines
		// and cannot be a table, which is why the single-line form below is read
		// from the opener itself.
		body := []string{lines[open]}
		closeIdx := -1
		for j := open + 1; j < len(lines); j++ {
			trimmed := strings.TrimLeft(lines[j], " \t")
			lineIndent := len(lines[j]) - len(trimmed)
			if lineIndent == indent && strings.HasPrefix(trimmed, "}") {
				closeIdx = j
				break
			}
			body = append(body, lines[j])
		}
		if closeIdx < 0 {
			continue // unbalanced: say nothing rather than guess
		}
		openIdx := open
		open = closeIdx // never rescan a literal's own body
		if _, skip := handTableOptOut[ident]; skip {
			continue
		}
		found := map[string]bool{}
		joined := strings.Join(body, "\n")
		for _, tok := range tokens {
			if strings.Contains(joined, `"`+tok+`"`) {
				found[tok] = true
			}
		}
		if len(found) < 2 {
			continue
		}
		names := make([]string, 0, len(found))
		for f := range found {
			names = append(names, f)
		}
		sort.Strings(names)
		hits = append(hits, handTableHit{
			line:   openIdx + 1,
			opener: strings.TrimSpace(lines[openIdx]),
			tokens: names,
		})
	}
	return hits
}

// literalOpener recognises `x = map[...]{` / `x = []...{` (with or without
// var/const) and returns the declared identifier.
func literalOpener(line string) (ident string, ok bool) {
	t := strings.TrimSpace(line)
	if !strings.HasSuffix(t, "{") {
		return "", false
	}
	eq := strings.Index(t, " = ")
	if eq < 0 {
		return "", false
	}
	lhs := strings.TrimSpace(t[:eq])
	rhs := strings.TrimSpace(t[eq+3:])
	if !strings.HasPrefix(rhs, "map[") && !strings.HasPrefix(rhs, "[]") {
		return "", false
	}
	lhs = strings.TrimPrefix(lhs, "var ")
	lhs = strings.TrimPrefix(lhs, "const ")
	lhs = strings.TrimSpace(lhs)
	if lhs == "" || strings.ContainsAny(lhs, " \t,()") {
		return "", false
	}
	return lhs, true
}

// goSourceFiles lists every non-test .go file in the tree, excluding vendored
// and generated trees and this loader package itself.
func goSourceFiles(root string) ([]string, error) {
	var out []string
	skipDirs := map[string]bool{
		".git": true, "node_modules": true, "vendor": true, "dist": true, "testdata": true,
	}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		out = append(out, path)
		return nil
	})
	sort.Strings(out)
	return out, err
}

// relTo renders a walked path repo-relative for the failure message.
func relTo(path string) string {
	rel, err := filepath.Rel(repoRoot, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}
