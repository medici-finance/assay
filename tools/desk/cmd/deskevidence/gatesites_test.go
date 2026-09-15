package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"testing"
)

// gatesites_test.go — the CLASS-level control assay#1066 asked for, distinct from the two
// per-verb identity assertions (gatewired_test.go here and in cmd/deskreply). Those prove the
// TWO named sites are fixed; this proves the CLASS is closed: no construction site for the
// public-repo gate's fetcher, ANYWHERE in the command tree, builds the single-forge
// deskkit.HTTPRepoInfoFetcher outside the one site this brief explicitly ruled to stay
// single-forge (deskrelease/cut.go — see the RULING comment at its call site).
//
// This is the SECOND, independent layer the brief's single-point-of-failure note calls for:
// the per-verb swap is one control, and repeating it at each call site is redundancy, not
// depth. This enumeration is a DIFFERENT signal (a source-tree scan, not a runtime call) in a
// DIFFERENT component (a test walking the AST, not the command being tested) — it goes red the
// moment a NEW verb is added that builds its own hardcoded fetcher, which no per-verb test can
// catch because a per-verb test only exists for verbs someone remembered to write one for.
//
// FAIL-FIRST: this test's own value is proven by the fact that it goes red on today's tree
// BEFORE assay#1060/assay#1066's fixes land (deskpr's three sites plus deskevidence's and
// deskreply's two both built the hardcoded fetcher) and passes once they are swapped and the
// superseded GitLabRepoInfoFetcher shim is deleted; row 6 (`grep -rn GitLabRepoInfoFetcher`)
// covers that shim's removal directly, and TestPublicRepoGateFetcherRoutesThroughResolvedForge
// in this package and in cmd/deskreply already show each swap's own fail-first red.
func TestPublicRepoGateSitesUseResolvedForge(t *testing.T) {
	// ruledExceptions names every (relative-to-cmd/) file allowed to build the single-forge
	// HTTPRepoInfoFetcher, and WHY — read at the site itself, not restated here beyond the
	// pointer. Silence is not a third answer: Task step 2 requires a route-or-rule decision
	// for every site, and this map is the rule half of that pair made machine-checked.
	ruledExceptions := map[string]string{
		"deskrelease/cut.go": "RULING (assay#1066/forge-gitlab-14): this tool never resolves a " +
			"Forge backend at all — it is a deliberately minimal, GitHub-only REST client for a " +
			"fixed compiled-in/configured release repo, with no already-resolved backend to route " +
			"the gate's read through. See the RULING comment at the construction site.",
	}

	cmdFiles, err := filepath.Glob(filepath.Join("..", "*", "*.go"))
	if err != nil {
		t.Fatalf("glob the command tree: %v", err)
	}
	if len(cmdFiles) == 0 {
		t.Fatal("globbed 0 files under ../*/*.go — the enumeration proved nothing")
	}

	fset := token.NewFileSet()
	scanned := 0
	foundRuled := map[string]int{}
	var unruledHits []string

	for _, f := range cmdFiles {
		if filepath.Ext(f) != ".go" || isTestFile(f) {
			continue
		}
		scanned++
		af, perr := parser.ParseFile(fset, f, nil, 0) // no ParseComments: only code decides
		if perr != nil {
			t.Fatalf("parse %s: %v", f, perr)
		}
		rel := relToCmd(f)
		ast.Inspect(af, func(n ast.Node) bool {
			cl, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			sel, ok := cl.Type.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "HTTPRepoInfoFetcher" {
				return true
			}
			pos := fset.Position(cl.Pos())
			if _, ruled := ruledExceptions[rel]; ruled {
				foundRuled[rel]++
				return true
			}
			unruledHits = append(unruledHits, rel+":"+strconv.Itoa(pos.Line))
			return true
		})
	}

	if scanned == 0 {
		t.Fatal("scanned 0 non-test source files under the command tree — the enumeration proved nothing")
	}

	if len(unruledHits) > 0 {
		t.Fatalf("the single-forge deskkit.HTTPRepoInfoFetcher is constructed outside every ruled "+
			"exception, at: %v — route the public-repo gate's fetcher through the already-resolved "+
			"forge backend (deskkit.ForgeRepoInfoFetcher{Forge: fg}) at each site, or add it to "+
			"ruledExceptions with a recorded reason at the call site", unruledHits)
	}

	// Sanity: the scanner actually saw the one site this brief ruled out, so a broken glob or a
	// renamed/moved file cannot silently make this test vacuous (a rename that drops the file
	// from ruledExceptions' coverage must surface as a hit above, not as a quiet pass here).
	for rel := range ruledExceptions {
		if foundRuled[rel] == 0 {
			t.Fatalf("ruled exception %s was never seen constructing HTTPRepoInfoFetcher — either "+
				"the ruling is stale (the site no longer builds one and the exception should be "+
				"removed) or the scanner never reached the file", rel)
		}
	}
}

// isTestFile reports whether path is a _test.go file.
func isTestFile(path string) bool {
	base := filepath.Base(path)
	return len(base) > 8 && base[len(base)-8:] == "_test.go"
}

// relToCmd renders a glob match like "../deskrelease/cut.go" as "deskrelease/cut.go" — the
// form ruledExceptions keys on, independent of which cmd/ package this test happens to run
// from.
func relToCmd(path string) string {
	dir, file := filepath.Split(path)
	dir = filepath.Base(filepath.Clean(dir))
	return dir + "/" + file
}
