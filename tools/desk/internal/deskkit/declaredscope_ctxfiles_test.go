package deskkit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Synthetic fixtures for the three Context files: shapes a verifier home must
// record rather than refuse (#5097): a sibling-repository path in a
// single-repository run, a trailing ** glob, and one trailing prose sentence.
// Every case outside those shapes stays refused.

func ctxDeclared(t *testing.T, files string) ([]string, error) {
	t.Helper()
	return BriefDeclaredFiles("# Brief\n\n## Context\n\n" + files + "\n\nfacts:\n- none\n")
}

// regression: #5097
func TestCtxGlobAccepted(t *testing.T) {
	for files, want := range map[string]string{
		"files:\n- `docs/guide/**`":                     "[docs/guide/**]",
		"files:\n- `tools/a.go`\n- `docs/guide/**`":     "[tools/a.go docs/guide/**]",
		"files:\n- `../one/docs/**`":                    "[../one/docs/**]",
		"files:\n- `../one/docs/*.md`\n- `../one/x/**`": "[../one/docs/*.md ../one/x/**]",
		"files: `docs/{a,b}/**`":                        "[docs/a/** docs/b/**]",
	} {
		got, err := ctxDeclared(t, files)
		if err != nil || fmt.Sprint(got) != want {
			t.Errorf("%q: got %v %v, want %s", files, got, err, want)
		}
	}
}

// Only a final /** after a literal directory is a recursive glob. Every other
// placement, and a bare ** that names no directory, stays refused.
// regression: #5097
func TestCtxGlobShapesRefused(t *testing.T) {
	for _, decl := range []string{
		"**", "/**", "docs/**/a.md", "docs/**x", "docs/x**", "docs/***", "docs/**/**",
		"docs/*/**", "docs/a?/**", "docs/[ab]/**", "../one/**", "../**/x", "../one/**/x",
		"{docs/**,tools/a.go}", "docs/../x/**", ".git/**",
	} {
		if _, err := ctxDeclared(t, "files:\n- `"+decl+"`"); ExitCodeOf(err) != ExitRefused {
			t.Errorf("%q: got %v, want refusal", decl, err)
		}
	}
}

// A trailing /** selects every file below its directory, nested ones included,
// and its scope: key wakes when a member is added.
// regression: #5097
func TestCtxGlobResolves(t *testing.T) {
	tree := scopeKeyFixture()
	scope, err := ResolveDeclaredScope([]string{"tools/pkg/**"}, tree)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(scope.Files); got != "[tools/pkg/one.go tools/pkg/sub/two.go]" {
		t.Fatalf("files = %s", got)
	}
	const key = "scope:tools/pkg/**"
	if _, ok := scope.Shapes[key]; !ok || len(scope.Shapes) != 1 {
		t.Fatalf("shapes = %v, want one %s", scope.Shapes, key)
	}
	root := scopeKeyRoot(t, tree.files)
	rr := NewRootRevisionReader(root, "v1")
	before, ok := rr.Revision(key)
	if !ok || before != scope.Shapes[key] {
		t.Fatalf("offline read %q %v, want %q", before, ok, scope.Shapes[key])
	}
	if err := os.WriteFile(filepath.Join(root, "tools/pkg/sub/three.go"), []byte("package sub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if after, ok := rr.Revision(key); !ok || after == before {
		t.Fatalf("an added nested member did not change %s", key)
	}
}

// One plain sentence after a closed list is an annotation: it declares
// nothing, so the scope equals the list's without it.
// regression: #5097
func TestCtxAnnotationAccepted(t *testing.T) {
	for _, tc := range []struct{ list, note string }{
		{"files:\n- `tools/runner/a.sh`\n- `tools/runner/b.sh`", "The short paths above are relative to the runner."},
		{"files: `tools/runner/a.sh`, `tools/runner/b.sh`,\n`docs/notes.md`.", "The runner paths above are listed in full."},
		{"files:\n- `tools/a.go` — the reader\n- `docs/guide/**`", "Everything under docs/guide/ is regenerated."},
		{"files:\n- `../one/x.go`", "The sibling path names the deliverable checkout."},
	} {
		want, err := ctxDeclared(t, tc.list)
		if err != nil {
			t.Fatalf("control %q: %v", tc.list, err)
		}
		got, err := ctxDeclared(t, tc.list+"\n"+tc.note)
		if err != nil || fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%q: got %v %v, want %v", tc.note, got, err, want)
		}
	}
}

// Anything that is not exactly one plain sentence ending a closed list stays
// refused: prose inside the list, a list still open, a second line, or a
// sentence naming a path the list does not already declare.
// regression: #5097
func TestCtxAnnotationRefused(t *testing.T) {
	const list = "files:\n- `tools/a.go`\n- `tools/b.go`"
	for name, files := range map[string]string{
		"prose inside list": "files:\n- `tools/a.go`\nThe next item is also part of the list.\n- `tools/b.go`",
		// F1 (#5097 round 1): a sentence must not bless a short name or a
		// dot-corrupted inline tail; either would bind the wrong path.
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
		if got, err := ctxDeclared(t, files); ExitCodeOf(err) != ExitRefused {
			t.Errorf("%s: got %v %v, want refusal", name, got, err)
		}
	}
}

// The malformed in-list entries the grammar already refused stay refused.
// regression: #5097
func TestCtxMalformedRefused(t *testing.T) {
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
		if got, err := ctxDeclared(t, "files:\n"+entry); ExitCodeOf(err) != ExitRefused {
			t.Errorf("%s %q: got %v %v, want refusal", name, entry, got, err)
		}
	}
}

// A single-repository run records each sibling declaration as declared and
// unbound: never resolved locally, never a repo-source pin it does not hold.
// regression: #5097
func TestCtxSiblingSingleRepo(t *testing.T) {
	for _, files := range []string{
		"files:\n- `../one/x.go`",
		"files:\n- `tools/a.go`\n- `../one/docs/**`\n- `../one/docs/*.md`",
		"files:\n- `../one/x.go`\n- `../two/y/`\nThe sibling paths name other checkouts.",
	} {
		r, err := siblingTestBuild(t, files, wakeTestInput())
		if err != nil {
			t.Fatalf("%q: single-repository record refused: %v", files, err)
		}
		declared, _ := ctxDeclared(t, files)
		_, siblings := SplitSiblingDeclarations(declared)
		for _, decl := range siblings {
			if got := r.Inputs[InputKeyDeclaredPrefix+decl]; got != DeclaredUnbound {
				t.Errorf("%q: %s = %q, want %q", files, decl, got, DeclaredUnbound)
			}
		}
		for k := range r.Inputs {
			if strings.HasPrefix(k, InputKeyRepoSourcePrefix) {
				t.Errorf("%q: single-repository record pins %s", files, k)
			}
			if strings.Contains(k, "..") && !strings.HasPrefix(k, InputKeyDeclaredPrefix) {
				t.Errorf("%q: sibling path became a local input key %q", files, k)
			}
		}
		if strings.Contains(files, "tools/a.go") && r.Inputs["file:tools/a.go"] == "" {
			t.Errorf("%q: local deliverable lost beside a sibling one", files)
		}
	}
}

// The unbound marker is accepted only where nothing binds any repository; it
// never stands beside a repo-source key, never names an undeclared path, and
// carries exactly one value.
// regression: #5097
func TestCtxSiblingMarkerRules(t *testing.T) {
	mark := InputKeyDeclaredPrefix + "../one/x.go"
	one := InputKeyRepoSourcePrefix + "example-org/one"
	other := InputKeyRepoSourcePrefix + "example-org/other"
	for _, tc := range []struct {
		name   string
		decls  []string
		inputs map[string]string
		ok     bool
	}{
		{"marked", []string{"../one/x.go"}, map[string]string{mark: DeclaredUnbound}, true},
		{"unmarked", []string{"../one/x.go"}, map[string]string{}, false},
		{"marker and pin", []string{"../one/x.go"}, map[string]string{mark: DeclaredUnbound, one: siblingTestSource}, false},
		{"marker in cross run", []string{"../one/x.go"}, map[string]string{mark: DeclaredUnbound, other: siblingTestSource}, false},
		{"wrong value", []string{"../one/x.go"}, map[string]string{mark: "bound"}, false},
		{"empty value", []string{"../one/x.go"}, map[string]string{mark: ""}, false},
		{"stray marker", nil, map[string]string{mark: DeclaredUnbound}, false},
		{"marker for local", []string{"tools/a.go"}, map[string]string{InputKeyDeclaredPrefix + "tools/a.go": DeclaredUnbound}, false},
		{"other sibling marked", []string{"../one/x.go"}, map[string]string{InputKeyDeclaredPrefix + "../two/x.go": DeclaredUnbound}, false},
	} {
		_, siblings := SplitSiblingDeclarations(tc.decls)
		err := CheckSiblingCoverage(siblings, tc.inputs)
		if (err == nil) != tc.ok || (err != nil && ExitCodeOf(err) != ExitRefused) {
			t.Errorf("%s: got %v, want ok=%v", tc.name, err, tc.ok)
		}
	}
	// A cross-repository run that binds no source for a declared sibling is
	// still refused: the marker is a single-repository record only.
	crossRepoRoster(t)
	if _, err := siblingTestBuild(t, "files:\n- `../other/x.go`", siblingTestInput()); ExitCodeOf(err) != ExitRefused {
		t.Errorf("cross-repository unbound sibling: got %v, want refusal", err)
	}
}

// The planner never reads an unbound sibling as unchanged.
// regression: #5097
func TestCtxSiblingPlanner(t *testing.T) {
	rr := NewRootRevisionReader(t.TempDir(), "v1")
	if _, ok := rr.Revision(InputKeyDeclaredPrefix + "../one/x.go"); ok {
		t.Fatal("an unbound sibling marker read as a revision")
	}
	const brief = "docs/streams/sample/brief-01-plan.md"
	body := "# Plan\n\n## Context\n\nfiles:\n- `../one/source.txt`\n"
	root := scopeKeyRoot(t, map[string]string{brief: body})
	input := map[string]string{
		"file:" + brief: Sha256Hex([]byte(body)), "tool": "test-tool",
		InputKeyDeclaredPrefix + "../one/source.txt": DeclaredUnbound,
	}
	r := NewWakeReceipt("fixture", "example-org/tracking", "sample/01", "verifier", "verify-fail", siblingTestSource, nil, input, "test-tool", "implementation", "#1", WakeRelevantInputChanged, "", "", "2026-10-13T20:00:00Z")
	if state, why := Unchanged(r, NewRootRevisionReader(root, "test-tool")); state != InputsCouldNotCheck {
		t.Fatalf("unbound sibling read %v %s, want could-not-check", state, why)
	}
}
