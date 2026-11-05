package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Class guard for the cross-root delivery witness.
//
// DEFECT CLASS: a consumer of merged-delivery evidence that reads its own
// root's history only, so a `Brief:` trailer merged in another configured root
// is never seen. The guard pins the two choke points crossroot.go names:
//
//   - the raw merge-history reader, mergedPRsFromGit, is referenced only from
//     deliveryReadSites — every planner read goes through deliveryHistory;
//   - every function that derives lifecycle cells (DeriveLifecycle) first admits
//     its PR witnesses through admitPRWitnesses, which appends the cross-root
//     records, and admitPRWitnesses itself calls crossRootPRRecords.
var deliveryReadSites = map[string]bool{
	"deliveryHistory":     true,
	"crossRootDeliveries": true,
}

// deliveryFindings returns one line per violation in file: a reference to
// mergedPRsFromGit outside deliveryReadSites (a package-level alias counts), a
// function calling DeriveLifecycle without calling admitPRWitnesses, and an
// admitPRWitnesses that does not call crossRootPRRecords. sites records each
// sanctioned site the walker saw, so a broken matcher cannot read as clean.
func deliveryFindings(name string, file *ast.File, sites map[string]bool) []string {
	var out []string
	refs := func(n ast.Node, ident string) bool {
		found := false
		ast.Inspect(n, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == ident {
				found = true
			}
			return !found
		})
		return found
	}
	for _, d := range file.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Body == nil {
				continue
			}
			fn := d.Name.Name
			if refs(d.Body, "mergedPRsFromGit") {
				if deliveryReadSites[fn] {
					sites[fn] = true
				} else {
					out = append(out, name+":"+fn+" reads mergedPRsFromGit directly — own-root history only; read through deliveryHistory")
				}
			}
			if refs(d.Body, "DeriveLifecycle") {
				if refs(d.Body, "admitPRWitnesses") {
					sites["derive:"+fn] = true
				} else {
					out = append(out, name+":"+fn+" calls DeriveLifecycle without admitPRWitnesses — no cross-root PR witness is admitted")
				}
			}
			if fn == "admitPRWitnesses" {
				if refs(d.Body, "crossRootPRRecords") {
					sites[fn] = true
				} else {
					out = append(out, name+":admitPRWitnesses does not call crossRootPRRecords — reconcile reads its own repo's PRs only")
				}
			}
		case *ast.GenDecl:
			for _, sp := range d.Specs {
				vs, ok := sp.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, v := range vs.Values {
					if refs(v, "mergedPRsFromGit") {
						out = append(out, name+":var "+vs.Names[0].Name+" aliases mergedPRsFromGit — read through deliveryHistory")
					}
				}
			}
		}
	}
	return out
}

func deliveryFindingsIn(t *testing.T, dir string) ([]string, map[string]bool) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	sites := map[string]bool{}
	var out []string
	fset := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, deliveryFindings(filepath.Base(f), file, sites)...)
	}
	sort.Strings(out)
	return out, sites
}

// TestDeliveryGuardSeesPlant is the guard's positive control: a planted
// own-root reader, an alias, and a lifecycle derivation that skips witness
// admission must each be reported.
func TestDeliveryGuardSeesPlant(t *testing.T) {
	const src = `package main

var plantedAlias = mergedPRsFromGit

func plantedHold(root string) { merged, err := mergedPRsFromGit(root, "HEAD"); applyPhantomHolds(root, nil, merged, err) }

func plantedDerive(in LifecycleInput) []BriefCell { return DeriveLifecycle(in) }
`
	file, err := parser.ParseFile(token.NewFileSet(), "plant.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := deliveryFindings("plant.go", file, map[string]bool{})
	if len(got) != 3 {
		t.Fatalf("guard walker reported %d findings, want 3:\n%s", len(got), strings.Join(got, "\n"))
	}
	for i, want := range []string{"plant.go:var plantedAlias ", "plant.go:plantedHold ", "plant.go:plantedDerive "} {
		if !strings.HasPrefix(got[i], want) {
			t.Errorf("finding %d = %q, want prefix %q", i, got[i], want)
		}
	}
}

// regression: #4614
func TestDeliveryReadChokePoint(t *testing.T) {
	findings, sites := deliveryFindingsIn(t, ".")
	for _, f := range findings {
		t.Error(f)
	}
	for _, want := range []string{"deliveryHistory", "crossRootDeliveries", "admitPRWitnesses", "derive:runReconcile", "derive:regenDriftNotices"} {
		if !sites[want] {
			t.Errorf("guard saw no sanctioned site %q (sites=%v) — the matcher is broken, not the tree clean", want, sites)
		}
	}
}
