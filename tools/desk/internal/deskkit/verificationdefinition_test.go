package deskkit

import (
	"reflect"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/briefdoc"
	"github.com/medici-finance/assay/tools/desk/internal/verifycmd"
)

const verificationFlowBrief = "---\nbrief: demo\n---\n\n# Demo\n\n## Context\n\nfiles:\n" +
	"- `tools/a.go` (edit)\n- `tools/pkg/` (new dir)\n- `../sibling/x/y.go`\n\n" +
	"## Verify\n\n" +
	"| # | Class | Command | Expect |\n|---|-------|---------|--------|\n" +
	"| 1 | check:ci +mutation | `go test ./tools -count=1` | exit 0 |\n" +
	"| 2 | check | In `PublicRepoGate` change it, then `cmd: go test ./tools -run X` | exit 0 |\n" +
	"| 3 | check | In `PublicRepoGate` change it, then `go test ./tools` | exit 0 |\n" +
	"| 4 | check | `## Coverage` | answered |\n"

// TestVerificationDefinitionFlow follows one brief through the compiled record
// from the runner's side to the landing scope's: the Verify table compiles once
// in briefdoc, the tools/desk executors reach the same verdict on every row
// through verifycmd, and the Context files: declaration projects into the
// declared scope with the sibling split deskkit applies at landing. No stage
// reads the brief with a grammar of its own.
// regression: F-tool-refactoring-03
func TestVerificationDefinitionFlow(t *testing.T) {
	_, body, err := briefdoc.Split(verificationFlowBrief)
	if err != nil {
		t.Fatal(err)
	}
	rows := briefdoc.CompileVerification(briefdoc.Section(body, "Verify", true))
	if len(rows) != 4 {
		t.Fatalf("compiled %d rows, want 4: %+v", len(rows), rows)
	}

	// Runner side: every executor verdict is the compiled record's verdict.
	wantRun := []struct {
		cmd     string
		refused bool
	}{
		{"go test ./tools -count=1", false},
		{"go test ./tools -run X", false}, // the explicit marker overrides the mention
		{"PublicRepoGate", true},          // a prose-led mention is refused, never run
		{"## Coverage", true},             // a shell comment is refused, never run
	}
	for i, r := range rows {
		cell := r.CommandCell
		if got := verifycmd.Lift(cell); got != r.Command || got != wantRun[i].cmd {
			t.Errorf("row %s: verifycmd.Lift = %q, compiled %q, want %q", r.ID, got, r.Command, wantRun[i].cmd)
		}
		refused := verifycmd.Refused(cell) != ""
		if refused != wantRun[i].refused || refused != (r.ProseLed != "") {
			t.Errorf("row %s: verifycmd refused=%v, compiled prose-led=%q, want refused=%v", r.ID, refused, r.ProseLed, wantRun[i].refused)
		}
	}
	if rows[0].ExecClass != "check:ci" || !reflect.DeepEqual(rows[0].Obligations, []string{"mutation"}) {
		t.Errorf("row 1 class = %q %v", rows[0].ExecClass, rows[0].Obligations)
	}

	// Landing side: the same document's declared inputs project into a scope.
	decls, err := BriefDeclaredFiles(body)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"tools/a.go", "tools/pkg", "../sibling/x/y.go"}; !reflect.DeepEqual(decls, want) {
		t.Fatalf("declarations = %v, want %v", decls, want)
	}
	local, siblings := SplitSiblingDeclarations(decls)
	if !reflect.DeepEqual(siblings, []string{"../sibling/x/y.go"}) || len(local) != 2 {
		t.Fatalf("split = %v / %v", local, siblings)
	}
	tree := fakeTree{files: map[string]string{"tools/a.go": "a", "tools/pkg/b.go": "b"}}
	scope, err := ResolveDeclaredScope(local, tree)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"tools/a.go", "tools/pkg/b.go"}; !reflect.DeepEqual(scope.Files, want) {
		t.Errorf("scope files = %v, want %v", scope.Files, want)
	}

	// The projection and the owner agree: deskkit adds meaning, not grammar.
	owner, oerr := briefdoc.DeclaredFiles(body)
	if oerr != nil || !reflect.DeepEqual(owner, decls) {
		t.Errorf("briefdoc.DeclaredFiles = %v, %v; deskkit projection %v", owner, oerr, decls)
	}

	// A refusal keeps its exit code through the projection.
	_, err = BriefDeclaredFiles(strings.Replace(body, "files:\n", "files: a/b.go then c\n", 1))
	if ExitCodeOf(err) != ExitRefused {
		t.Errorf("an unsupported tail must stay a refusal, got %v", err)
	}
}
