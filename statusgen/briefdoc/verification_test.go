package briefdoc

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// verifyTable renders a Verify section from table lines.
func verifyTable(lines ...string) string {
	return "\n" + strings.Join(lines, "\n") + "\n"
}

func loadVectorTable(t *testing.T, name string, into interface{}) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "testdata", name))
	if err != nil {
		t.Fatalf("could-not-check: the shared vector table %s is unreadable: %v", name, err)
	}
	if err := json.Unmarshal(b, into); err != nil {
		t.Fatal(err)
	}
}

// TestVerificationDefinitionMatrix pins the document and row grammar the one
// compilation accepts: row identity, the Class cell, optional Shell and Env
// columns, escaped pipes, the explicit command marker, prose-led and comment
// refusal, and the shared vector tables the tools/desk executors are held to.
// regression: F-tool-refactoring-03
func TestVerificationDefinitionMatrix(t *testing.T) {
	type want struct {
		id          string
		numbered    bool
		command     string
		execClass   string
		obligations []string
		classed     bool
		proseLed    bool
	}
	cases := []struct {
		name  string
		table []string
		want  []want
	}{
		{
			name: "numeric id and compound class",
			table: []string{
				"| # | Class | Command | Expect |",
				"|---|-------|---------|--------|",
				"| 1 | check:ci +mutation +neighbour | `go test ./x -count=1` | exit 0 |",
			},
			want: []want{{"1", true, "go test ./x -count=1", "check:ci", []string{"mutation", "neighbour"}, true, false}},
		},
		{
			name: "suffixed, lettered and decorated ids",
			table: []string{
				"| # | Command | Expect |",
				"|---|---------|--------|",
				"| 6a | `true` | exit 0 |",
				"| A | `echo a` | exit 0 |",
				"| `3` | `echo b` | exit 0 |",
				"| **4** | `echo c` | exit 0 |",
			},
			want: []want{
				{"6a", true, "true", "", nil, false, false},
				{"A", true, "echo a", "", nil, false, false},
				{"3", true, "echo b", "", nil, false, false},
				{"4", true, "echo c", "", nil, false, false},
			},
		},
		{
			name: "no id column falls back to the ordinal",
			table: []string{
				"| Command | Expect |",
				"|---------|--------|",
				"| `echo a` | exit 0 |",
				"| `echo b` | exit 0 |",
			},
			want: []want{
				{"1", false, "echo a", "", nil, false, false},
				{"2", false, "echo b", "", nil, false, false},
			},
		},
		{
			name: "escaped pipe stays one cell",
			table: []string{
				"| # | Command | Expect |",
				"|---|---------|--------|",
				"| 1 | `ls x \\| grep -c y` | exit 0 |",
			},
			want: []want{{"1", true, `ls x \| grep -c y`, "", nil, false, false}},
		},
		{
			name: "explicit marker beats a mention ahead of it",
			table: []string{
				"| # | Class | Command | Expect |",
				"|---|-------|---------|--------|",
				"| 1 | check:ci | In `PublicRepoGate` change it, then `cmd: go test ./y -count=1` | exit 0 |",
			},
			want: []want{{"1", true, "go test ./y -count=1", "check:ci", nil, true, false}},
		},
		{
			name: "a prose-led mention is flagged, never a command",
			table: []string{
				"| # | Class | Command | Expect |",
				"|---|-------|---------|--------|",
				"| 1 | check:ci | In `PublicRepoGate` change it, then `go test ./y` | exit 0 |",
			},
			want: []want{{"1", true, "PublicRepoGate", "check:ci", nil, true, true}},
		},
		{
			name: "a shell comment is flagged",
			table: []string{
				"| # | Command | Expect |",
				"|---|---------|--------|",
				"| 1 | `## Coverage` | answered |",
			},
			want: []want{{"1", true, "## Coverage", "", nil, false, true}},
		},
		{
			name: "gate:model prose row compiles with its class and no span",
			table: []string{
				"| # | Class | Command | Expect |",
				"|---|-------|---------|--------|",
				"| 1 | gate:model | Reviewer reads the diff against the brief | verdict recorded |",
			},
			want: []want{{"1", true, "Reviewer reads the diff against the brief", "gate:model", nil, true, false}},
		},
		{
			name: "a row with neither command nor expect is dropped; a short row is skipped",
			table: []string{
				"| # | Class | Command | Expect |",
				"|---|-------|---------|--------|",
				"| 1 | check |  |  |",
				"| 2 |",
				"| 3 | check | `echo z` | exit 0 |",
			},
			want: []want{{"3", true, "echo z", "check", nil, true, false}},
		},
		{
			name: "a second table re-reads its own header",
			table: []string{
				"| # | Command | Expect |",
				"|---|---------|--------|",
				"| 1 | `echo a` | exit 0 |",
				"",
				"prose between the tables",
				"",
				"| # | Expect | Command |",
				"|---|--------|---------|",
				"| 2 | exit 0 | `echo b` |",
			},
			want: []want{
				{"1", true, "echo a", "", nil, false, false},
				{"2", true, "echo b", "", nil, false, false},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rows := CompileVerification(verifyTable(c.table...))
			var got []want
			for _, r := range rows {
				got = append(got, want{r.ID, r.Numbered, r.Command, r.ExecClass, r.Obligations, r.Classed, r.ProseLed != ""})
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("CompileVerification =\n  %+v\nwant\n  %+v", got, c.want)
			}
		})
	}

	t.Run("shell and env columns are located by header name", func(t *testing.T) {
		rows := CompileVerification(verifyTable(
			"| Env | # | Shell | Class | Command | Expect |",
			"|-----|---|-------|-------|---------|--------|",
			"| ci | 1 | pwsh | check | `Get-Item .` | exit 0 |",
		))
		if len(rows) != 1 || rows[0].ShellCell != "pwsh" || rows[0].EnvCell != "ci" || !rows[0].EnvCol || rows[0].ID != "1" {
			t.Fatalf("rows = %+v", rows)
		}
	})

	t.Run("a document compiles through its Verify section", func(t *testing.T) {
		doc := "---\nbrief: x\n---\n\n# Brief\n\n## Task\n\n| # | Command | Expect |\n|---|---------|--------|\n| 9 | `echo task` | not a verify row |\n\n## Verify\n\n| # | Command | Expect |\n|---|---------|--------|\n| 1 | `echo v` | exit 0 |\n\n## Evidence\n\n| # | Command | Expect |\n|---|---------|--------|\n| 2 | `echo e` | not a verify row |\n"
		_, body, err := Split(doc)
		if err != nil {
			t.Fatal(err)
		}
		rows := CompileVerification(Section(body, "Verify", true))
		if len(rows) != 1 || rows[0].ID != "1" || rows[0].Command != "echo v" {
			t.Fatalf("rows = %+v", rows)
		}
	})

	t.Run("cmd-marker shared vectors", func(t *testing.T) {
		var doc struct {
			Vectors []struct {
				Name   string  `json:"name"`
				Cell   string  `json:"cell"`
				Marked *string `json:"marked"`
			} `json:"vectors"`
		}
		loadVectorTable(t, "cmd-marker-vectors.json", &doc)
		if len(doc.Vectors) < 10 {
			t.Fatalf("positive control: only %d vectors loaded", len(doc.Vectors))
		}
		for _, v := range doc.Vectors {
			m := MarkedCommands(v.Cell)
			switch {
			case v.Marked == nil && len(m) != 0:
				t.Errorf("%s: MarkedCommands(%q) = %q, want no honoured marker", v.Name, v.Cell, m)
			case v.Marked != nil && (len(m) == 0 || m[0] != *v.Marked):
				t.Errorf("%s: MarkedCommands(%q) = %q, want first %q", v.Name, v.Cell, m, *v.Marked)
			}
			want := CodeSpan(v.Cell)
			if v.Marked != nil {
				want = *v.Marked
			}
			if got := VerifyCommand(v.Cell); got != want {
				t.Errorf("%s: VerifyCommand(%q) = %q, want %q", v.Name, v.Cell, got, want)
			}
		}
	})

	t.Run("shell-comment shared vectors", func(t *testing.T) {
		var doc struct {
			Vectors []struct {
				Name        string `json:"name"`
				Cmd         string `json:"cmd"`
				CommentOnly bool   `json:"commentOnly"`
			} `json:"vectors"`
			Cells []struct {
				Name      string `json:"name"`
				Cell      string `json:"cell"`
				FirstSpan string `json:"firstSpan"`
				Marked    bool   `json:"marked"`
				Lift      string `json:"lift"`
				Statusgen string `json:"statusgen"`
			} `json:"cells"`
		}
		loadVectorTable(t, "shell-comment-vectors.json", &doc)
		if len(doc.Vectors) < 8 || len(doc.Cells) < 30 {
			t.Fatalf("positive control: %d vectors, %d cells loaded", len(doc.Vectors), len(doc.Cells))
		}
		for _, v := range doc.Vectors {
			if got := ShellCommentOnly(v.Cmd); got != v.CommentOnly {
				t.Errorf("%s: ShellCommentOnly(%q) = %v, want %v", v.Name, v.Cmd, got, v.CommentOnly)
			}
		}
		for _, v := range doc.Cells {
			if got := CodeSpan(v.Cell); got != v.FirstSpan {
				t.Errorf("%s: CodeSpan(%q) = %q, want %q", v.Name, v.Cell, got, v.FirstSpan)
			}
			if got := len(MarkedCommands(v.Cell)) > 0; got != v.Marked {
				t.Errorf("%s: marked = %v, want %v", v.Name, got, v.Marked)
			}
			if got := VerifyCommand(v.Cell); got != v.Lift {
				t.Errorf("%s: VerifyCommand(%q) = %q, want %q", v.Name, v.Cell, got, v.Lift)
			}
			_, why := ProseLedCommandWhy(v.Cell)
			got := "runs"
			switch {
			case strings.Contains(why, "shell comment"):
				got = "comment-refused"
			case why != "" || strings.TrimSpace(VerifyCommand(v.Cell)) == "":
				got = "refused"
			}
			if got != v.Statusgen {
				t.Errorf("%s: verdict on %q = %s (%q), want %s", v.Name, v.Cell, got, why, v.Statusgen)
			}
		}
	})
}

// lexicalLiftSites is the class census: every non-test function under roots
// whose body scans a table cell for code spans or names the `cmd:` marker. The
// match is syntactic on purpose: a copy of the lift is recognised by the act of
// scanning for a backtick or the marker, not by its name. Entries are
// "slash/path.go:Func".
func lexicalLiftSites(t *testing.T, roots ...string) []string {
	t.Helper()
	fset := token.NewFileSet()
	var out []string
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				switch d.Name() {
				case "testdata", "vendor", "node_modules", ".git":
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("could-not-check: %s does not parse: %v", path, err)
			}
			out = append(out, scanLiftSites(f, filepath.ToSlash(path))...)
			return nil
		})
		if err != nil {
			t.Fatalf("could-not-check: census walk of %s: %v", root, err)
		}
	}
	sort.Strings(out)
	return out
}

// scanLiftSites returns "name:Func" for each function in f that scans for a
// backtick (strings.Index or strings.IndexByte against "`" or '`') or carries
// the "cmd:" marker literal.
func scanLiftSites(f *ast.File, name string) []string {
	var out []string
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		hit := false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.BasicLit:
				if x.Value == `"cmd:"` {
					hit = true
				}
			case *ast.CallExpr:
				sel, ok := x.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "strings" || (sel.Sel.Name != "Index" && sel.Sel.Name != "IndexByte") {
					return true
				}
				if len(x.Args) == 2 {
					if lit, ok := x.Args[1].(*ast.BasicLit); ok && (lit.Value == "\"`\"" || lit.Value == "'`'") {
						hit = true
					}
				}
			}
			return true
		})
		if hit {
			out = append(out, name+":"+fn.Name.Name)
		}
	}
	return out
}

// retainedLiftSites names every function outside the briefdoc owner that still
// scans for code spans, with the reason it is NOT a Verify-row command lift. A
// site that lifts a Verify command must call briefdoc.CompileVerification or
// briefdoc.VerifyCommand; a legacy difference needs a named profile here, not a
// second grammar. The key is the repository-relative path (from the statusgen
// directory) and function.
var retainedLiftSites = map[string]string{
	"bottleneck.go:detectShift":                                     "reads a report's prose for the constraint stage, never a Verify cell",
	"../tools/desk/cmd/deskevidence/principalstamp.go:codeSpanEnd":  "scans Evidence prose for a stamp span, never a Verify Command cell",
	"verifyissues.go:fenceLineValid":                                "judges a Markdown code-fence line in an issue body",
	"admittedrowbind.go:admittedRowBindings":                        "skips a shell backquote substitution in an already-lifted Verify command, never a Markdown code span",
	"../tools/desk/internal/deskkit/readybackstop.go:revertedState": "reads a forge comment's merge-state sentence, never a Verify cell",
}

// TestVerificationDefinitionClassGuard — the tool-refactoring/03 class guard.
//
// The class: "a site reads a Verify Command cell by a rule other than the one
// compilation" (a second lexical lift accepted a prose code span as a command,
// or honoured a marker the rendered table does not show). The guard has three
// halves, each with a positive control so a blind guard cannot report clean:
//
//  1. census: every function in statusgen and tools/desk that scans for code
//     spans or names the marker is either the owner or a documented retention;
//  2. behaviour: a planted divergent parser (first span wins, no prose check)
//     disagrees with the compilation on the prose-led matrix and is caught;
//  3. callers: the migrated callers (statusgen's verifyRowsOf, the verifycmd
//     executors' Lift) reach the compilation, and nothing else lifts.
//
// regression: F-tool-refactoring-03
func TestVerificationDefinitionClassGuard(t *testing.T) {
	statusgen := filepath.Join("..")
	desk := filepath.Join("..", "..", "tools", "desk")
	for _, r := range []string{statusgen, desk} {
		if _, err := os.Stat(r); err != nil {
			t.Fatalf("could-not-check: census root %s is not present: %v", r, err)
		}
	}

	t.Run("census of lexical copies", func(t *testing.T) {
		sites := lexicalLiftSites(t, statusgen, desk)
		if len(sites) == 0 {
			t.Fatal("positive control: the census saw no span-scanning function at all, so it is blind")
		}
		seen := map[string]bool{}
		for _, s := range sites {
			rel := s
			if i := strings.Index(s, ":"); i >= 0 {
				if r, err := filepath.Rel(statusgen, s[:i]); err == nil {
					rel = filepath.ToSlash(r) + s[i:]
				}
			}
			seen[rel] = true
			if strings.HasPrefix(rel, "briefdoc/verification.go:") || strings.HasPrefix(rel, "briefdoc/declared.go:") {
				continue // the owner
			}
			if _, ok := retainedLiftSites[rel]; !ok {
				t.Errorf("%s scans for code spans or the cmd: marker outside the briefdoc owner: compile the row through briefdoc.CompileVerification (or call briefdoc.VerifyCommand), or add the site to retainedLiftSites with the reason it is not a Verify-row lift", rel)
			}
		}
		for rel := range retainedLiftSites {
			if !seen[rel] {
				t.Errorf("retainedLiftSites lists %s but the census no longer finds it: delete the stale entry", rel)
			}
		}
		owner := 0
		for s := range seen {
			if strings.HasPrefix(s, "briefdoc/verification.go:") || strings.HasPrefix(s, "briefdoc/declared.go:") {
				owner++
			}
		}
		if owner == 0 {
			t.Error("positive control: the census does not see the briefdoc owner's own lift")
		}
	})

	t.Run("a planted second parser is caught by the census", func(t *testing.T) {
		const planted = "package planted\n\nimport \"strings\"\n\nfunc liftFirstSpan(cell string) string {\n\ti := strings.Index(cell, \"`\")\n\treturn cell[i:]\n}\n\nfunc markerOf(cell string) bool { return strings.HasPrefix(cell, \"cmd:\") }\n"
		f, err := parser.ParseFile(token.NewFileSet(), "planted.go", planted, 0)
		if err != nil {
			t.Fatal(err)
		}
		got := scanLiftSites(f, "planted.go")
		want := []string{"planted.go:liftFirstSpan", "planted.go:markerOf"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("census of the planted parser = %v, want %v", got, want)
		}
	})

	t.Run("a planted divergent parser fails the behaviour control", func(t *testing.T) {
		// plantedLift is the defect the class names: the first code span wins,
		// with no marker scan and no prose-led check, so a mention executes.
		plantedLift := func(cell string) string { return CodeSpan(cell) }
		prose := "In `PublicRepoGate` change the check, then `cmd: go test ./y -count=1`"
		if got, want := plantedLift(prose), VerifyCommand(prose); got == want {
			t.Fatalf("positive control: the planted parser and the compilation agree on %q (%q), so the control cannot tell them apart", prose, got)
		}
		mention := "In `PublicRepoGate` change the check, then `go test ./y -count=1`"
		rows := CompileVerification(verifyTable(
			"| # | Class | Command | Expect |",
			"|---|-------|---------|--------|",
			"| 1 | check:ci | "+mention+" | exit 0 |",
		))
		if len(rows) != 1 || rows[0].ProseLed == "" {
			t.Fatalf("the compilation must flag a prose-led mention, never hand it over as a command: %+v", rows)
		}
		if planted := plantedLift(mention); planted != rows[0].Command {
			t.Fatalf("the planted parser lifted %q, the compilation %q: both must reach the same first span so only the verdict differs", planted, rows[0].Command)
		}
	})

	t.Run("migrated callers reach the compilation", func(t *testing.T) {
		for _, c := range []struct{ file, fn, call string }{
			{filepath.Join(statusgen, "verifyrun.go"), "verifyRowsOf", "CompileVerification"},
			{filepath.Join(statusgen, "verifyrows.go"), "verifyRowTable", "VerifyTable"},
			{filepath.Join(desk, "internal", "verifycmd", "verifycmd.go"), "Lift", "VerifyCommand"},
			{filepath.Join(desk, "internal", "verifycmd", "verifycmd.go"), "Marked", "MarkedCommands"},
			{filepath.Join(desk, "internal", "verifycmd", "verifycmd.go"), "Refused", "ProseLedMention"},
		} {
			if !callsBriefdoc(t, c.file, c.fn, c.call) {
				t.Errorf("%s:%s no longer calls briefdoc.%s: a caller that stops delegating is a second reader of the Verify table", c.file, c.fn, c.call)
			}
		}
		// Positive control: the matcher rejects a function that does not call it.
		if callsBriefdoc(t, filepath.Join(desk, "internal", "verifycmd", "verifycmd.go"), "CommandRefused", "CompileVerification") {
			t.Error("positive control: callsBriefdoc reported a call that is not there")
		}
	})
}

// callsBriefdoc reports whether function fn in file calls briefdoc.<call>.
func callsBriefdoc(t *testing.T, file, fn, call string) bool {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
	if err != nil {
		t.Fatalf("could-not-check: %s does not parse: %v", file, err)
	}
	found := false
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil || fd.Name.Name != fn {
			continue
		}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok {
				if sel, ok := c.Fun.(*ast.SelectorExpr); ok {
					if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "briefdoc" && sel.Sel.Name == call {
						found = true
					}
				}
			}
			return true
		})
	}
	return found
}
