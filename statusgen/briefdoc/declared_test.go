package briefdoc

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func ctx(lines ...string) string {
	return "## Context\n\n" + strings.Join(lines, "\n") + "\n\n## Task\n\nx\n"
}

// TestDeclaredFilesMatrix characterises the accepted and refused forms of a
// brief's Context `files:` declaration. The reader is the one grammar the
// tools/desk declared-scope projection consumes (tool-refactoring/03).
// regression: F-tool-refactoring-03
func TestDeclaredFilesMatrix(t *testing.T) {
	ok := []struct {
		name string
		doc  string
		want []string
	}{
		{"inline list", ctx("files: a/b.go, c/d.go"), []string{"a/b.go", "c/d.go"}},
		{"inline backticked", ctx("files: `a/b.go`, `c/d.go`"), []string{"a/b.go", "c/d.go"}},
		{"bulleted list", ctx("files:", "- a/b.go", "- `c/d.go`"), []string{"a/b.go", "c/d.go"}},
		{"bullet description after a dash", ctx("files:", "- a/b.go — the owner", "- c/d.go"), []string{"a/b.go", "c/d.go"}},
		{"NEW annotation in parens", ctx("files: a/b.go (NEW), c/d.go"), []string{"a/b.go", "c/d.go"}},
		{"annotation inside the span", ctx("files: `a/b.go (planned)`"), []string{"a/b.go"}},
		{"bold action annotation", ctx("files: **NEW** a/b.go"), []string{"a/b.go"}},
		{"and tests tail", ctx("files: a/b.go and tests"), []string{"a/b.go"}},
		{"brace group", ctx("files: a/{b,c}.go"), []string{"a/b.go", "a/c.go"}},
		{"recursive glob", ctx("files: docs/guide/**"), []string{"docs/guide/**"}},
		{"sibling path", ctx("files: ../other/x/y.go"), []string{"../other/x/y.go"}},
		{"duplicates collapse", ctx("files: a/b.go, a/b.go"), []string{"a/b.go"}},
		{"next field ends the block", ctx("files: a/b.go", "facts: x"), []string{"a/b.go"}},
	}
	for _, c := range ok {
		t.Run(c.name, func(t *testing.T) {
			got, err := DeclaredFiles(c.doc)
			if err != nil || !reflect.DeepEqual(got, c.want) {
				t.Fatalf("DeclaredFiles = %v, %v; want %v", got, err, c.want)
			}
		})
	}

	refused := []struct{ name, doc string }{
		{"empty declaration", ctx("files:")},
		{"unclosed quote", ctx("files: `a/b.go")},
		{"unclosed annotation", ctx("files: a/b.go (NEW")},
		{"unsupported tail", ctx("files: a/b.go then c")},
		{"duplicate files field", ctx("files: a/b.go", "", "files: c/d.go")},
		{"mid-path recursive glob", ctx("files: a/**/b.go")},
		{"parent escape", ctx("files: ../../x")},
		{"absolute path", ctx("files: /etc/x")},
		{"duplicate Context section", "## Context\n\nfiles: a\n\n## Context\n\nfiles: b\n"},
		{"indented files marker", ctx("  files: a/b.go")},
	}
	for _, c := range refused {
		t.Run("refuses "+c.name, func(t *testing.T) {
			got, err := DeclaredFiles(c.doc)
			var r *Refusal
			if !errors.As(err, &r) || got != nil {
				t.Fatalf("DeclaredFiles = %v, %v; want a *Refusal and no paths", got, err)
			}
		})
	}

	t.Run("no files field is not an error", func(t *testing.T) {
		got, err := DeclaredFiles(ctx("facts: x"))
		if err != nil || len(got) != 0 {
			t.Fatalf("DeclaredFiles = %v, %v; want empty and no error", got, err)
		}
	})
}
