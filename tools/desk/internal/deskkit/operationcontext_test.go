package deskkit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/domain"
)

// deliveryCheckout is a separate real repository whose origin is owner/product.
func deliveryCheckout(t *testing.T, origin string) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"remote", "add", "origin", origin}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s %v", args, out, err)
		}
	}
	return dir
}

func operationCandidate(repo string) *domain.Candidate {
	return &domain.Candidate{Repository: repo, BaseCommit: strings.Repeat("b", 40), BaseTree: strings.Repeat("d", 40), Commit: strings.Repeat("c", 40), Tree: strings.Repeat("e", 40), ManifestDigest: "sha256:" + strings.Repeat("f", 64)}
}

// operationAdapterCensus enumerates every adapter call of the domain constructor
// and checks that the decision target and Evidence destination it supplies are
// the tracking observation, never the delivery checkout or another value.
func operationAdapterCensus(fset *token.FileSet, files []*ast.File) (callers, violations []string) {
	for _, f := range files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "NewOperationSubject" {
					return true
				}
				callers = append(callers, fn.Name.Name)
				pos := fset.Position(call.Pos()).String()
				var lit *ast.CompositeLit
				if len(call.Args) == 1 {
					lit, _ = call.Args[0].(*ast.CompositeLit)
				}
				if lit == nil {
					violations = append(violations, pos+": observations not built at the call")
					return true
				}
				seen := map[string]bool{}
				for _, elt := range lit.Elts {
					kv, ok := elt.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					key, _ := kv.Key.(*ast.Ident)
					if key == nil || (key.Name != "Decision" && key.Name != "Evidence") {
						continue
					}
					inner, ok := kv.Value.(*ast.CompositeLit)
					if !ok {
						violations = append(violations, pos+": "+key.Name+" not built from observations")
						continue
					}
					for _, ie := range inner.Elts {
						ikv, ok := ie.(*ast.KeyValueExpr)
						if !ok {
							continue
						}
						if k, _ := ikv.Key.(*ast.Ident); k == nil || k.Name != "Repository" {
							continue
						}
						// Name-based: the census trusts the identifier spelled
						// `tracking` to be the tracking observation. A delivery
						// value renamed `tracking` would pass; the planted
						// control covers the realistic wrong-repository mistake.
						v, ok := ikv.Value.(*ast.SelectorExpr)
						if !ok {
							continue
						}
						if base, _ := v.X.(*ast.Ident); base != nil && base.Name == "tracking" && v.Sel.Name == "Repository" {
							seen[key.Name] = true
						}
					}
					if !seen[key.Name] {
						violations = append(violations, pos+": "+fn.Name.Name+" "+key.Name+" repository is not the tracking observation")
					}
				}
				if !seen["Decision"] || !seen["Evidence"] {
					violations = append(violations, pos+": "+fn.Name.Name+" omits a tracking-bound decision or Evidence coordinate")
				}
				return true
			})
		}
	}
	return callers, violations
}

func parseDeskkit(t *testing.T, fset *token.FileSet, planted string) []*ast.File {
	t.Helper()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	if planted != "" {
		f, err := parser.ParseFile(fset, "planted.go", planted, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	return files
}

// regression: #5525
func TestOperationContextAdapterFlow(t *testing.T) {
	tracking, linked, commit := workSourceFixture(t, false)
	catalog, err := LocalWorkCatalog(tracking, "")
	if err != nil {
		t.Fatal(err)
	}
	work, err := catalog.Select("stream:b-08@2")
	if err != nil {
		t.Fatal(err)
	}
	product := deliveryCheckout(t, "https://github.com/owner/product.git")

	// Two repositories: the brief and its decision and Evidence live in the
	// tracking repository; the delivery PR and candidate live in product.
	candidate := operationCandidate("owner/product")
	subject, err := OperationContext(work, tracking, product, OperationTargets{Review: domain.ReviewTarget{Kind: domain.ReviewDelivery, Repository: "owner/product", Number: 9, Head: candidate.Commit}, Candidate: candidate})
	if err != nil {
		t.Fatal(err)
	}
	d, err := subject.Dispatch()
	if err != nil || d.TrackingRepository != "owner/repo" || d.DecisionRepository != "owner/repo" || d.DeliveryRepository != "owner/product" || !d.CrossRepository || d.Brief.Commit != commit {
		t.Fatalf("dispatch: %+v %v", d, err)
	}
	if r, err := subject.Review(); err != nil || r.Repository != "owner/product" || r.Number != 9 {
		t.Fatalf("review: %+v %v", r, err)
	}
	if p, err := subject.PRPublication(); err != nil || p.Repository != "owner/product" || !p.CrossRepository || p.Work != "stream:b-08@2" {
		t.Fatalf("PR publication: %+v %v", p, err)
	}
	if e, err := subject.EvidencePublication(); err != nil || e.Repository != "owner/repo" || e.Path != work.Subject.Path || e.Delivery.Repository != "owner/product" {
		t.Fatalf("Evidence publication: %+v %v", e, err)
	}

	// One repository: a linked worktree of the tracking checkout delivers.
	subject, err = OperationContext(work, tracking, linked, OperationTargets{})
	if err != nil {
		t.Fatal(err)
	}
	if d, err := subject.Dispatch(); err != nil || d.CrossRepository || d.DeliveryRepository != "owner/repo" {
		t.Fatalf("same-repository dispatch: %+v %v", d, err)
	}

	// Unobserved delivery stays unknown; it is not the tracking repository.
	subject, err = OperationContext(work, tracking, "", OperationTargets{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := subject.Dispatch(); err == nil {
		t.Fatal("dispatch without an observed delivery checkout succeeded")
	} else if ExitCodeOf(operationError(err)) != ExitUnverifiable {
		t.Fatalf("missing delivery not unverifiable: %v", err)
	}
	if _, err := OperationContext(work, tracking, "", OperationTargets{Candidate: candidate}); ExitCodeOf(err) != ExitUnverifiable {
		t.Fatalf("candidate without delivery checkout: %v", err)
	}

	// Wrong coordinates refuse (exit 5): a delivery review aimed at the tracking
	// repository, and a candidate from another repository.
	for name, targets := range map[string]OperationTargets{
		"review-in-tracker": {Review: domain.ReviewTarget{Kind: domain.ReviewDelivery, Repository: "owner/repo", Number: 9, Head: candidate.Commit}},
		"foreign-candidate": {Candidate: operationCandidate("owner/repo")},
	} {
		if _, err := OperationContext(work, tracking, product, targets); ExitCodeOf(err) != ExitRefused {
			t.Fatalf("%s: %v", name, err)
		}
	}

	// The tracking checkout moved after selection: the observed commit no longer
	// matches the selected brief revision, so the subject refuses.
	moved := filepath.Join(tracking, "moved.txt")
	if err := os.WriteFile(moved, []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "moved.txt"}, {"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-q", "-m", "move"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = tracking
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s %v", args, out, err)
		}
	}
	if _, err := OperationContext(work, tracking, product, OperationTargets{}); ExitCodeOf(err) != ExitRefused || !strings.Contains(err.Error(), "tracking-source-mismatch") {
		t.Fatalf("moved tracking checkout: %v", err)
	}

	// A tracking checkout with no commit is could-not-check, never a substituted subject.
	if _, err := OperationContext(work, deliveryCheckout(t, "https://github.com/owner/repo.git"), product, OperationTargets{}); ExitCodeOf(err) != ExitUnverifiable {
		t.Fatalf("tracking checkout without HEAD: %v", err)
	}

	// Census: the adapter is the only caller, and it binds decision and Evidence
	// to the tracking observation. A planted second caller that takes the
	// decision target from the delivery checkout is caught.
	fset := token.NewFileSet()
	callers, violations := operationAdapterCensus(fset, parseDeskkit(t, fset, ""))
	if strings.Join(callers, ",") != "OperationContext" || len(violations) != 0 {
		t.Fatalf("adapter census: callers=%v violations=%v", callers, violations)
	}
	planted := `package deskkit
func plantedDispatch(w WorkSelection, tracking, delivery domain.TrackingSource) {
	domain.NewOperationSubject(domain.OperationObservations{Tracking: tracking, Decision: domain.DecisionTarget{Repository: delivery.Repository}, Evidence: domain.EvidenceDestination{Repository: tracking.Repository}})
}
`
	fset = token.NewFileSet()
	callers, violations = operationAdapterCensus(fset, parseDeskkit(t, fset, planted))
	if len(callers) != 2 || len(violations) == 0 || !strings.HasPrefix(violations[0], "planted.go:") {
		t.Fatalf("planted wrong-coordinate caller not caught: callers=%v violations=%v", callers, violations)
	}
}
