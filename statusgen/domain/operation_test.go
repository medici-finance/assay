package domain

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/workref"
)

const (
	opUUID    = "7a0c0968-a001-4aab-acea-681909b30340"
	opTracker = "owner/tracker"
	opProduct = "owner/product"
	opPath    = "docs/streams/stream/brief-01-work.md"
)

var (
	opBriefCommit = strings.Repeat("a", 40)
	opHead        = strings.Repeat("c", 40)
)

// opWork resolves through the real workref owner, so the subject binds the
// same logical key the adapter's selection does.
func opWork(t *testing.T, spelling string) workref.Result {
	t.Helper()
	id := workref.Identity{Kind: workref.Brief, Cell: "cell", Repo: "tracker", Stream: "stream", Number: "01"}
	reg, err := workref.NewRegistry([]workref.Root{{Cell: "cell", Repo: "tracker"}}, []workref.Binding{{Identity: id, Key: opUUID}})
	if err != nil {
		t.Fatal(err)
	}
	r, err := workref.Resolve(spelling, &workref.Context{Cell: "cell", Repo: "tracker"}, reg, workref.TypedBrief)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func opCandidate(repo string) *Candidate {
	return &Candidate{repo, strings.Repeat("b", 40), strings.Repeat("d", 40), opHead, strings.Repeat("e", 40), "sha256:" + strings.Repeat("f", 64)}
}

// opObservations is the fully observed subject for a brief tracked in tracker
// and delivered to deliver; review is of the delivery pull request.
func opObservations(t *testing.T, deliver string) OperationObservations {
	return OperationObservations{
		Work:     opWork(t, "stream:b-01@1"),
		Brief:    BriefRevision{opUUID, 1, opTracker, opBriefCommit, opPath},
		Tracking: TrackingSource{opTracker, opBriefCommit},
		Delivery: DeliveryTarget{deliver, opCandidate(deliver)},
		Review:   ReviewTarget{ReviewDelivery, deliver, 42, opHead},
		Decision: DecisionTarget{opTracker},
		Evidence: EvidenceDestination{opTracker, opPath},
	}
}

// regression: #5525
func TestOperationSubjectMatrix(t *testing.T) {
	for _, deliver := range []string{opTracker, opProduct} {
		cross := deliver != opTracker
		s, err := NewOperationSubject(opObservations(t, deliver))
		if err != nil {
			t.Fatalf("deliver=%s: %v", deliver, err)
		}
		d, err := s.Dispatch()
		if err != nil || d.TrackingRepository != opTracker || d.DecisionRepository != opTracker || d.DeliveryRepository != deliver || d.CrossRepository != cross {
			t.Fatalf("deliver=%s dispatch: %+v %v", deliver, d, err)
		}
		r, err := s.Review()
		if err != nil || r.Repository != deliver || r.Number != 42 || r.Head != opHead || r.Kind != ReviewDelivery {
			t.Fatalf("deliver=%s review of the delivery PR: %+v %v", deliver, r, err)
		}
		p, err := s.PRPublication()
		if err != nil || p.Repository != deliver || p.TrackingRepository != opTracker || p.CrossRepository != cross || p.Work != "stream:b-01@1" {
			t.Fatalf("deliver=%s PR publication: %+v %v", deliver, p, err)
		}
		e, err := s.EvidencePublication()
		if err != nil || e.Repository != opTracker || e.Path != opPath || e.Delivery.Repository != deliver || e.Delivery.Commit != opHead {
			t.Fatalf("deliver=%s outcome publication: %+v %v", deliver, e, err)
		}
		if s.Work().Compatibility != "cell:tracker:stream:01" || s.Brief().Version != 1 {
			t.Fatalf("legacy identifier or exact version lost: %+v", s.Work())
		}
	}

	// Authoring review of the brief PR targets the tracking repository.
	in := opObservations(t, opProduct)
	in.Review = ReviewTarget{ReviewAuthoring, opTracker, 7, opBriefCommit}
	s, err := NewOperationSubject(in)
	if err != nil {
		t.Fatal(err)
	}
	if r, err := s.Review(); err != nil || r.Repository != opTracker || r.Kind != ReviewAuthoring {
		t.Fatalf("authoring review: %+v %v", r, err)
	}

	// Supplied observations are copied: mutating the caller's candidate after
	// construction cannot move the subject.
	in = opObservations(t, opProduct)
	s, err = NewOperationSubject(in)
	if err != nil {
		t.Fatal(err)
	}
	in.Delivery.Candidate.Repository = opTracker
	if e, _ := s.EvidencePublication(); e.Delivery.Repository != opProduct {
		t.Fatalf("subject not immutable: %+v", e)
	}

	// Missing coordinates stay unknown in the projection that needs them, and
	// never fall back to a sibling coordinate (#5525: an unknown tracking or
	// decision repository refuses instead of using the delivery repository).
	missing := []struct {
		name, code string
		edit       func(*OperationObservations)
		project    func(OperationSubject) error
	}{
		{"tracking", "tracking-source-unavailable", func(o *OperationObservations) { o.Tracking = TrackingSource{} }, nil},
		{"brief", "brief-revision-unavailable", func(o *OperationObservations) { o.Brief.Commit = "" }, nil},
		{"decision", "decision-target-unavailable", func(o *OperationObservations) { o.Decision = DecisionTarget{} }, func(s OperationSubject) error { _, e := s.Dispatch(); return e }},
		{"delivery-dispatch", "delivery-target-unavailable", func(o *OperationObservations) { o.Delivery = DeliveryTarget{}; o.Review = ReviewTarget{} }, func(s OperationSubject) error { _, e := s.Dispatch(); return e }},
		{"delivery-pr", "delivery-target-unavailable", func(o *OperationObservations) { o.Delivery = DeliveryTarget{}; o.Review = ReviewTarget{} }, func(s OperationSubject) error { _, e := s.PRPublication(); return e }},
		{"review", "review-target-unavailable", func(o *OperationObservations) { o.Review = ReviewTarget{} }, func(s OperationSubject) error { _, e := s.Review(); return e }},
		{"review-head", "review-head-unavailable", func(o *OperationObservations) { o.Review.Head = "" }, func(s OperationSubject) error { _, e := s.Review(); return e }},
		{"evidence", "evidence-destination-unavailable", func(o *OperationObservations) { o.Evidence = EvidenceDestination{} }, func(s OperationSubject) error { _, e := s.EvidencePublication(); return e }},
		{"candidate", "delivery-candidate-unavailable", func(o *OperationObservations) { o.Delivery.Candidate = nil }, func(s OperationSubject) error { _, e := s.EvidencePublication(); return e }},
		{"review-delivery", "delivery-target-unavailable", func(o *OperationObservations) { o.Delivery = DeliveryTarget{} }, nil},
	}
	for _, m := range missing {
		in := opObservations(t, opProduct)
		m.edit(&in)
		s, err := NewOperationSubject(in)
		if m.project != nil {
			if err != nil {
				t.Fatalf("%s: construction: %v", m.name, err)
			}
			err = m.project(s)
		}
		requireCode(t, err, m.code)
		if err.(*Refusal).State != "could-not-check" {
			t.Fatalf("%s: missing coordinate reported as %s", m.name, err.(*Refusal).State)
		}
	}
}

// operationCensus enumerates operation-subject constructors and every read of a
// subject coordinate in one package's non-test files. A composite literal of the
// subject outside NewOperationSubject is a second constructor; a coordinate read
// outside the named projections is a consumer re-deriving its own coordinate.
func operationCensus(fset *token.FileSet, files []*ast.File) (constructors, readers, violations []string) {
	allowedReaders := map[string]bool{"Work": true, "Brief": true, "Dispatch": true, "Review": true, "PRPublication": true, "EvidencePublication": true}
	fields := map[string]bool{"work": true, "brief": true, "tracking": true, "delivery": true, "review": true, "decision": true, "evidence": true}
	isSubject := func(e ast.Expr) bool {
		if st, ok := e.(*ast.StarExpr); ok {
			e = st.X
		}
		id, ok := e.(*ast.Ident)
		return ok && id.Name == "OperationSubject"
	}
	for _, f := range files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			name := fn.Name.Name
			pos := fset.Position(fn.Pos()).String()
			subjects := map[string]bool{}
			method := fn.Recv != nil && len(fn.Recv.List) == 1 && isSubject(fn.Recv.List[0].Type)
			for _, list := range []*ast.FieldList{fn.Recv, fn.Type.Params} {
				if list == nil {
					continue
				}
				for _, field := range list.List {
					if isSubject(field.Type) {
						for _, n := range field.Names {
							subjects[n.Name] = true
						}
					}
				}
			}
			if fn.Type.Results != nil && name != "NewOperationSubject" {
				for _, r := range fn.Type.Results.List {
					if isSubject(r.Type) && !method {
						violations = append(violations, pos+": "+name+" returns an operation subject outside NewOperationSubject")
					}
				}
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.ValueSpec:
					if x.Type != nil && isSubject(x.Type) {
						for _, n := range x.Names {
							subjects[n.Name] = true
						}
					}
				case *ast.AssignStmt:
					if len(x.Rhs) == 1 {
						if call, ok := x.Rhs[0].(*ast.CallExpr); ok {
							if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "NewOperationSubject" {
								if lhs, ok := x.Lhs[0].(*ast.Ident); ok {
									subjects[lhs.Name] = true
								}
							}
						}
					}
				case *ast.CompositeLit:
					if isSubject(x.Type) {
						if name == "NewOperationSubject" {
							if len(x.Elts) > 0 { // zero literals accompany a refusal
								constructors = append(constructors, name)
							}
						} else {
							violations = append(violations, fset.Position(x.Pos()).String()+": "+name+" constructs an operation subject")
						}
					}
				case *ast.SelectorExpr:
					id, ok := x.X.(*ast.Ident)
					if ok && subjects[id.Name] && fields[x.Sel.Name] {
						if method && allowedReaders[name] {
							readers = append(readers, name+"."+x.Sel.Name)
						} else {
							violations = append(violations, fset.Position(x.Pos()).String()+": "+name+" reads coordinate "+x.Sel.Name+" outside a projection")
						}
					}
				}
				return true
			})
		}
	}
	sort.Strings(readers)
	return constructors, readers, violations
}

func parseDomain(t *testing.T, fset *token.FileSet, planted map[string]string) []*ast.File {
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
	for name, src := range planted {
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	return files
}

// regression: #5525
func TestOperationSubjectClassGuard(t *testing.T) {
	// The real package: one constructor, coordinates read only by projections.
	fset := token.NewFileSet()
	constructors, readers, violations := operationCensus(fset, parseDomain(t, fset, nil))
	if len(violations) != 0 {
		t.Fatalf("operation-subject class violations: %v", violations)
	}
	if strings.Join(constructors, ",") != "NewOperationSubject" {
		t.Fatalf("constructor census: %v", constructors)
	}
	for _, want := range []string{"Dispatch.decision", "Dispatch.delivery", "Dispatch.tracking", "EvidencePublication.evidence", "PRPublication.delivery", "Review.review"} {
		if !contains(readers, want) {
			t.Fatalf("census does not see the real projection read %s: %v", want, readers)
		}
	}

	// Planted second consumers: one re-derives the decision target from the
	// delivery repository, one mints a subject without the constructor's checks.
	planted := map[string]string{"planted.go": `package domain
func plantedDecisionTarget(s OperationSubject) string { return s.delivery.Repository }
func plantedEvidence(b BriefRevision) OperationSubject { return OperationSubject{brief: b} }
func plantedLocal(in OperationObservations) string { s, _ := NewOperationSubject(in); return s.tracking.Repository }
`}
	fset = token.NewFileSet()
	_, _, violations = operationCensus(fset, parseDomain(t, fset, planted))
	var hits []string
	for _, v := range violations {
		if strings.HasPrefix(v, "planted.go:") {
			hits = append(hits, v)
		}
	}
	if len(hits) != 4 || len(hits) != len(violations) {
		t.Fatalf("planted consumers not all caught: %v", violations)
	}

	// Behavioural neighbour: a supplied coordinate that names the wrong repository,
	// or a declared alias/selector that names other work, is refused at construction.
	wrong := []struct {
		name, code string
		edit       func(*OperationObservations)
	}{
		{"decision-in-delivery", "decision-target-mismatch", func(o *OperationObservations) { o.Decision.Repository = opProduct }},
		{"evidence-in-delivery", "evidence-destination-mismatch", func(o *OperationObservations) { o.Evidence.Repository = opProduct }},
		{"evidence-path-locator", "evidence-destination-mismatch", func(o *OperationObservations) { o.Evidence.Path = "docs/streams/other/brief-01-work.md" }},
		{"tracking-repo", "tracking-source-mismatch", func(o *OperationObservations) { o.Tracking.Repository = opProduct }},
		{"tracking-commit", "tracking-source-mismatch", func(o *OperationObservations) { o.Tracking.Commit = strings.Repeat("9", 40) }},
		{"review-in-tracker", "review-target-mismatch", func(o *OperationObservations) { o.Review.Repository = opTracker }},
		{"authoring-in-delivery", "review-target-mismatch", func(o *OperationObservations) { o.Review.Kind = ReviewAuthoring }},
		{"review-head", "review-head-mismatch", func(o *OperationObservations) { o.Review.Head = strings.Repeat("8", 40) }},
		{"candidate-repo", "delivery-candidate-mismatch", func(o *OperationObservations) { o.Delivery.Candidate = opCandidate(opTracker) }},
		{"other-work", "work-subject-mismatch", func(o *OperationObservations) { o.Work.LogicalKey = "00000000-0000-4000-8000-000000000000" }},
		{"other-version", "work-revision-mismatch", func(o *OperationObservations) { o.Work.Revision = 2 }},
	}
	for _, w := range wrong {
		in := opObservations(t, opProduct)
		w.edit(&in)
		_, err := NewOperationSubject(in)
		requireCode(t, err, w.code)
		if err.(*Refusal).State != "checked-failed" {
			t.Fatalf("%s: wrong coordinate reported as %s", w.name, err.(*Refusal).State)
		}
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
