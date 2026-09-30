package deskkit

// outbound_numberhint_test.go — the bare-`#N` reference notice keeps its number.
//
// Defect class: a write that targets a NUMBERED item (a PR or issue being commented on,
// reviewed or edited) runs the outbound check WITHOUT that number, so the self-containment
// scan's bare-`#N` heuristic degrades from "`#812` is above #40, a number known to exist on
// this repo" to a generic NOT CHECKED line. Two guards cover the class:
//
//   - the decorator: every text-carrying Forge method whose signature takes a number must
//     name a bare reference above it (found by reflection, so a number-bearing method
//     added later is covered without a new row);
//   - the verbs: every deskkit.OutboundWrite literal under cmd/ whose Kind is not a
//     number-less write (a new issue, a file, a commit, a ref) must set NumberHint.

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// obNumber is the item number every number-bearing decorator call targets.
const obNumber = 40

// obBareRefBody names one bare reference above obNumber and one below it.
const obBareRefBody = "Fixed in #812, see also #3."

func obNamedRefNotice(n, hint int) string {
	return fmt.Sprintf("bare `#%d` is above #%d", n, hint)
}

// TestOutboundNumberHintOnDecorator is the decorator half of the class guard.
func TestOutboundNumberHintOnDecorator(t *testing.T) {
	obRoster(t)
	it := reflect.TypeOf((*Forge)(nil)).Elem()
	numbered := 0
	for i := 0; i < it.NumMethod(); i++ {
		m := it.Method(i)
		tc, text := obTextMethods[m.Name]
		if !text {
			continue
		}
		takesNumber := false
		for j := 0; j < m.Type.NumIn(); j++ {
			if m.Type.In(j).Kind() == reflect.Int {
				takesNumber = true
			}
		}
		t.Run(m.Name, func(t *testing.T) {
			var notices bytes.Buffer
			defer SetOutboundNoticeWriter(&notices)()
			SetOutboundContext(OutboundContext{Tool: "numberhint", Verb: m.Name})
			if err := tc.call(OutboundChecked(&recordingForge{}, "worker"), obRepo(obPublic), obBareRefBody); err != nil {
				t.Fatalf("%s: a bare-reference body must pass (it is a notice, never a refusal): %v", m.Name, err)
			}
			got := notices.String()
			named := strings.Contains(got, obNamedRefNotice(812, obNumber))
			if takesNumber && !named {
				t.Fatalf("%s targets item #%d but its check ran without the number: want a NOTICE "+
					"containing %q, got %q", m.Name, obNumber, obNamedRefNotice(812, obNumber), got)
			}
			if strings.Contains(got, "#3` is above") {
				t.Fatalf("%s: #3 is below the hint and must not be named: %q", m.Name, got)
			}
		})
		if takesNumber {
			numbered++
		}
	}
	// Positive control on the reflection itself: the decorator has number-bearing methods
	// today, so a reflection walk that finds none has stopped looking.
	if numbered == 0 {
		t.Fatal("found no number-bearing text method on Forge — the reflection walk is broken")
	}
}

// TestOutboundNumberHintRow is the conformance row for OutboundCheck itself: with a hint the
// notice names the reference above it; without one it says NOT CHECKED.
func TestOutboundNumberHintRow(t *testing.T) {
	obRoster(t)
	for _, tc := range []struct {
		hint int
		want string
	}{
		{obNumber, obNamedRefNotice(812, obNumber)},
		{0, "NOT CHECKED"},
	} {
		var notices bytes.Buffer
		restore := SetOutboundNoticeWriter(&notices)
		SetOutboundContext(OutboundContext{Tool: "numberhint", Verb: "row"})
		err := outboundCheckHinted(obPublic, tc.hint, obBareRefBody)
		restore()
		if err != nil {
			t.Fatalf("hint %d: want pass, got %v", tc.hint, err)
		}
		if !strings.Contains(notices.String(), tc.want) {
			t.Fatalf("hint %d: want a NOTICE containing %q, got %q", tc.hint, tc.want, notices.String())
		}
	}
}

// obNumberlessKinds are the OutboundWrite kinds that target no pre-existing numbered item.
var obNumberlessKinds = map[string]bool{
	"OutboundKindIssue": true, "OutboundKindFile": true,
	"OutboundKindCommit": true, "OutboundKindRef": true,
}

// hintlessOutboundWrites returns, for one parsed file, every OutboundWrite composite literal
// whose Kind is not a number-less constant and which sets no NumberHint. A Kind that is not
// a constant selector (a variable) is treated as numbered: the guard fails closed.
func hintlessOutboundWrites(fset *token.FileSet, f *ast.File) []string {
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		cl, ok := n.(*ast.CompositeLit)
		if !ok || compositeLitTypeName(cl.Type) != "OutboundWrite" {
			return true
		}
		kind, hinted := "", false
		for _, e := range cl.Elts {
			kv, ok := e.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, _ := kv.Key.(*ast.Ident)
			if key == nil {
				continue
			}
			switch key.Name {
			case "Kind":
				kind = compositeLitTypeName(kv.Value)
			case "NumberHint":
				hinted = true
			}
		}
		if !hinted && !obNumberlessKinds[kind] {
			out = append(out, fmt.Sprintf("%s (Kind %s)", fset.Position(cl.Pos()), kind))
		}
		return true
	})
	return out
}

// TestOutboundWritesCarryNumber is the verb half of the class guard, with a planted positive
// control so a matcher that silently stops matching fails instead of reporting clean.
func TestOutboundWritesCarryNumber(t *testing.T) {
	const plant = `package p
func f() { _ = deskkit.OutboundCheck(deskkit.OutboundWrite{Repo: r, Kind: deskkit.OutboundKindComment}) }
func g() { _ = deskkit.OutboundCheck(deskkit.OutboundWrite{Repo: r, Kind: deskkit.OutboundKindIssue}) }
func h() { _ = deskkit.OutboundCheck(deskkit.OutboundWrite{Repo: r, Kind: k, NumberHint: 7}) }
`
	fset := token.NewFileSet()
	pf, err := parser.ParseFile(fset, "plant.go", plant, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := hintlessOutboundWrites(fset, pf); len(got) != 1 || !strings.Contains(got[0], "plant.go:2") {
		t.Fatalf("positive control: want exactly the planted comment write flagged, got %v", got)
	}

	var offenders []string
	root := filepath.Join(deskTreeRoot, "cmd")
	err = filepath.Walk(root, func(path string, info os.FileInfo, werr error) error {
		if werr != nil {
			return werr
		}
		if info.IsDir() {
			if info.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fs := token.NewFileSet()
		f, perr := parser.ParseFile(fs, path, nil, 0)
		if perr != nil {
			return perr
		}
		offenders = append(offenders, hintlessOutboundWrites(fs, f)...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(offenders)
	if len(offenders) > 0 {
		t.Fatalf("outbound writes to a numbered item with no NumberHint (the bare-`#N` notice "+
			"degrades to NOT CHECKED):\n  %s", strings.Join(offenders, "\n  "))
	}
}

// outboundCheckHinted is one comment write to repo, checked with hint as its item number.
func outboundCheckHinted(repo string, hint int, body string) error {
	return OutboundCheck(OutboundWrite{Repo: repo, Kind: OutboundKindComment, NumberHint: hint,
		Fields: []OutboundField{{Name: "body", Text: body}}})
}
