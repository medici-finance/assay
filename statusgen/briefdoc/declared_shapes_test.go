package briefdoc

import (
	"fmt"
	"testing"
)

// The Context files: grammar is characterised here, at its owner, for the three
// shapes a verifier home records rather than refuses (a trailing ** glob, a
// sibling path, one trailing prose sentence) and for every neighbouring shape
// that stays refused. tools/desk keeps the same tables through its projection
// (declaredscope_ctxfiles_test.go); these are the owner's own copy, so a change
// to the grammar fails here without needing the projection, and the mutation
// specs for the moved code land on tests that live beside it.

func shapesDeclared(files string) ([]string, error) {
	return DeclaredFiles("# Brief\n\n## Context\n\n" + files + "\n\nfacts:\n- none\n")
}

// regression: F-tool-refactoring-03
func TestDeclaredFilesGlobShapes(t *testing.T) {
	for files, want := range map[string]string{
		"files:\n- `docs/guide/**`":                     "[docs/guide/**]",
		"files:\n- `tools/a.go`\n- `docs/guide/**`":     "[tools/a.go docs/guide/**]",
		"files:\n- `../one/docs/**`":                    "[../one/docs/**]",
		"files:\n- `../one/docs/*.md`\n- `../one/x/**`": "[../one/docs/*.md ../one/x/**]",
		"files: `docs/{a,b}/**`":                        "[docs/a/** docs/b/**]",
	} {
		got, err := shapesDeclared(files)
		if err != nil || fmt.Sprint(got) != want {
			t.Errorf("%q: got %v %v, want %s", files, got, err, want)
		}
	}
	for _, decl := range []string{
		"**", "/**", "docs/**/a.md", "docs/**x", "docs/x**", "docs/***", "docs/**/**",
		"docs/*/**", "docs/a?/**", "docs/[ab]/**", "../one/**", "../**/x", "../one/**/x",
		"{docs/**,tools/a.go}", "docs/../x/**", ".git/**",
	} {
		if _, err := shapesDeclared("files:\n- `" + decl + "`"); !isRefusal(err) {
			t.Errorf("%q: got %v, want refusal", decl, err)
		}
	}
}

// regression: F-tool-refactoring-03
func TestDeclaredFilesAnnotationShapes(t *testing.T) {
	for _, tc := range []struct{ list, note string }{
		{"files:\n- `tools/runner/a.sh`\n- `tools/runner/b.sh`", "The short paths above are relative to the runner."},
		{"files: `tools/runner/a.sh`, `tools/runner/b.sh`,\n`docs/notes.md`.", "The runner paths above are listed in full."},
		{"files:\n- `tools/a.go` — the reader\n- `docs/guide/**`", "Everything under docs/guide/ is regenerated."},
		{"files:\n- `../one/x.go`", "The sibling path names the deliverable checkout."},
	} {
		want, err := shapesDeclared(tc.list)
		if err != nil {
			t.Fatalf("control %q: %v", tc.list, err)
		}
		got, err := shapesDeclared(tc.list + "\n" + tc.note)
		if err != nil || fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%q: got %v %v, want %v", tc.note, got, err, want)
		}
	}

	const list = "files:\n- `tools/a.go`\n- `tools/b.go`"
	for name, files := range map[string]string{
		"prose inside list":     "files:\n- `tools/a.go`\nThe next item is also part of the list.\n- `tools/b.go`",
		"short names inline":    "files: tools/runner/a.sh, b.sh, c.sh,\nd.sh (new); docs/notes.md.\nThe abbreviated runner paths above are relative to tools/runner/.",
		"short names bulleted":  "files:\n- `tools/runner/a.sh`\n- `b.sh`\n- `c.sh`\nThe short paths above are relative to tools/runner/.",
		"short names in words":  "files:\n- `tools/runner/a.sh`\n- `b.sh`\nThe short paths above are relative to the runner directory.",
		"root name with note":   "files:\n- `tools/a.go`\n- `Makefile`\nThe build file is listed by its short name.",
		"dot on inline tail":    "files: tools/a.go, docs/notes.md.\nThe docs path above is the deliverable page.",
		"second prose line":     list + "\nThe short paths are relative here.\nAnd this line continues it.",
		"list still open":       "files: tools/a.go, tools/b.go,\nThe rest are relative to tools/.",
		"files line empty":      "files:\nThe paths are listed somewhere else.",
		"hidden path":           list + "\nAlso update tools/hidden.go when needed.",
		"hidden sibling":        list + "\nThe change also reaches ../two/x.go here.",
		"hidden bare name":      list + "\nThe change also edits README.md as well.",
		"quoted path":           list + "\nThe change also edits `tools/c.go` here.",
		"comma list":            list + "\nThe tests, the docs and the tools change.",
		"lower case":            list + "\nthe short paths are relative here.",
		"no full stop":          list + "\nThe short paths are relative here",
		"too short":             list + "\nSee the notes.",
		"glob in prose":         list + "\nThe tools/*.go files are generated here.",
		"em dash":               list + "\nThe paths are relative — see tools/.",
		"list item":             list + "\n1. The paths are relative to tools.",
		"quote":                 list + "\n> The paths are relative to tools.",
		"indented":              list + "\n  The paths are relative to tools here.",
		"colon":                 list + "\nThe paths are relative to: tools.",
		"parenthesis":           list + "\nThe paths are relative (to tools).",
		"field-looking":         list + "\nNote: the paths are relative to tools.",
		"ellipsis":              list + "\nThe paths are relative to tools...",
		"word with digits":      list + "\nTools2 paths are relative to tools.",
		"title case":            list + "\nTHE paths are relative to tools.",
		"annotation only":       "files: The paths are relative to tools.",
		"brace":                 list + "\nThe paths {a,b} are relative here.",
		"sentence then fence":   list + "\nThe paths are relative to tools.\n```\nx\n```",
		"sentence then heading": list + "\nThe paths are relative to tools.\n### more",
	} {
		if got, err := shapesDeclared(files); !isRefusal(err) {
			t.Errorf("%s: got %v %v, want refusal", name, got, err)
		}
	}

	for name, entry := range map[string]string{
		"empty":          "- ``",
		"absolute":       "- `/etc/passwd`",
		"escape":         "- `../../x.go`",
		"inner escape":   "- `tools/../x.go`",
		"git dir":        "- `.git/config`",
		"control":        "- `tools/a\x01.go`",
		"nul":            "- `tools/a\x00.go`",
		"backslash":      "- `tools\\a.go`",
		"placeholder":    "- `docs/<slug>.md`",
		"ellipsis":       "- `tools/.../a.go`",
		"unclosed quote": "- `tools/a.go",
		"bad brace":      "- `tools/{a,,b}.go`",
		"prose tail":     "- tools/a.go is edited with tools/b.go",
	} {
		if got, err := shapesDeclared("files:\n" + entry); !isRefusal(err) {
			t.Errorf("%s %q: got %v %v, want refusal", name, entry, got, err)
		}
	}
}
