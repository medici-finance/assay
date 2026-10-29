package deskkit

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// regression: #2798
func TestDeclaredReceiptForms(t *testing.T) {
	for _, decl := range []string{
		"files:\n- tools/a.go\n- tools/pkg/",
		"files: `tools/a.go` (edit), `tools/pkg/` and tests",
		"files: `tools/a.go` and tests,\n`tools/pkg/` and tests (new),\ntheir tests,",
		"files:\n- `tools/a.go`: commentary `tools/not-declared.go`\n- tools/pkg/",
		"files:\n- `tools/{a.go,pkg/}` — commentary `tools/not-declared.go`",
		"files:\n- `tools/*.go` — the `plan` command\n- tools/pkg/*",
		// regression: medici-finance/assay#3756 — annotation before a ':' description.
		"files:\n- `tools/a.go` (new): commentary `tools/not-declared.go`\n- `tools/pkg/` (new): `one.go`, `sub/two.go` (x)",
		// regression: #3817 — annotation inside the path's code span.
		"files:\n- `tools/a.go (planned)`\n- `tools/pkg/ (new synthetic fixtures)`",
		"files: `tools/a.go (new)`, `tools/pkg/ (planned)`: commentary `tools/not-declared.go`",
	} {
		t.Run(decl, func(t *testing.T) {
			tree := wakeTestTree()
			tree.files[wakeTestBriefPath] = "# Brief\n\n## Context\n\n" + decl + "\n\nfacts:\n- `tools/not-declared.go`\n"
			// Keep glob's independent second site separate from the reported fixture.
			if strings.Contains(decl, "tools/*.go") {
				delete(tree.files, "tools/not-declared.go")
			}
			raw, err := BuildOutcomeRecord(wakeTestRecord("blocked", BlockerUnknown, "#1"), wakeTestInput(), tree)
			if err != nil {
				t.Fatal(err)
			}
			var r WakeReceipt
			if err := json.Unmarshal(raw, &r); err != nil {
				t.Fatal(err)
			}
			for _, p := range []string{"tools/a.go", "tools/pkg/one.go", "tools/pkg/sub/two.go"} {
				if !coveredByDigest(r.Inputs, p) {
					t.Errorf("omitted declared deliverable %s", p)
				}
			}
			if _, ok := r.Inputs["file:tools/not-declared.go"]; ok {
				t.Fatal("commentary became a declaration")
			}
		})
	}
}

// coveredByDigest: p has its own file: key, or a directory holding it has a file:<dir>/ key.
func coveredByDigest(inputs map[string]string, p string) bool {
	if inputs["file:"+p] != "" {
		return true
	}
	for d := path.Dir(p); d != "." && d != "/"; d = path.Dir(d) {
		if inputs[DirInputKey(d)] != "" {
			return true
		}
	}
	return false
}

// regression: #2798
func TestDeclaredRefusesAmbiguity(t *testing.T) {
	for _, decl := range []string{"docs/<slug>.md", "tools/.../a.go", "tools/**/a.go", "../outside", "/outside", "tools/[broken", "tools/{a,,b}.go", "tools/{../a,b}.go", "tools/" + strings.Repeat("{a,b}", 9), strings.Repeat("x", 4097)} {
		tree := wakeTestTree()
		tree.files[wakeTestBriefPath] = "## Context\nfiles:\n- `" + decl + "`\n"
		_, err := BuildOutcomeRecord(wakeTestRecord("blocked", BlockerUnknown, "#1"), wakeTestInput(), tree)
		if ExitCodeOf(err) != ExitRefused {
			t.Errorf("%q: got %v; want explicit refusal", decl, err)
		}
	}
}

// regression: #2798
func TestDeclaredMembershipWakes(t *testing.T) {
	for _, decl := range []string{"tools/pkg/", "tools/pkg/*", "tools/planned.go"} {
		t.Run(decl, func(t *testing.T) {
			tree := wakeTestTree()
			tree.files[wakeTestBriefPath] = "## Context\nfiles:\n- " + decl + "\n"
			tree.files["tools/pkg/[one].go"] = "literal bracket file name\n"
			in := wakeTestInput()
			in.LandedBrief = []byte(tree.files[wakeTestBriefPath])
			raw, err := BuildOutcomeRecord(wakeTestRecord("blocked", BlockerUnknown, "#1"), in, tree)
			if err != nil {
				t.Fatal(err)
			}
			var r WakeReceipt
			if err := json.Unmarshal(raw, &r); err != nil {
				t.Fatal(err)
			}
			r.Brief = "demo/01"
			root := t.TempDir()
			for p, b := range tree.files {
				full := filepath.Join(root, p)
				if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(b), 0644); err != nil {
					t.Fatal(err)
				}
			}
			rr := NewRootRevisionReader(root, in.ToolVersion)
			if state, why := Unchanged(r, rr); state != InputsUnchanged {
				t.Fatalf("unchanged: %v %s", state, why)
			}
			newFile := "tools/pkg/new.go"
			if strings.Contains(decl, "planned") {
				newFile = "tools/planned.go"
			}
			if err := os.WriteFile(filepath.Join(root, newFile), []byte("package added\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if state, why := Unchanged(r, rr); state != InputsChanged {
				t.Errorf("new declared member held: %v %s", state, why)
			}
			if d := r.EvaluateWake(rr, time.Now()); d.State != WakeFire {
				t.Errorf("new declared member did not wake: %v", d)
			}
		})
	}
}

// regression: #2798
func TestScopeSecondDeclaration(t *testing.T) {
	tree := wakeTestTree()
	tree.files[wakeTestBriefPath] = "## Context\nfiles: `tools/a.go`\n- planted/second.go\nfacts:\n- `tools/not-declared.go` is commentary\n"
	tree.files["planted/second.go"] = "second independent declaration\n"
	raw, err := BuildOutcomeRecord(wakeTestRecord("blocked", BlockerUnknown, "#1"), wakeTestInput(), tree)
	if err != nil {
		t.Fatal(err)
	}
	var receipt WakeReceipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Inputs["file:planted/second.go"] != Sha256Hex([]byte(tree.files["planted/second.go"])) {
		t.Fatal("planted second site omitted: planted/second.go")
	}
	if _, ok := receipt.Inputs["file:tools/not-declared.go"]; ok {
		t.Fatal("facts became declared input")
	}
}

// regression: #2798
func TestScopeReaderContainment(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "value"), []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "ancestor")); err != nil {
		t.Skip(err)
	}
	rr := NewRootRevisionReader(root, "v1")
	for _, key := range []string{"file:ancestor/value", "scope:ancestor/value", "scope:ancestor/*", "scope:../outside", "scope:C:/outside", "scope:foo\\bar", "scope:foo/<slug>"} {
		if _, ok := rr.Revision(key); ok {
			t.Errorf("unsafe input read: %s", key)
		}
	}
	if got, ok := rr.Revision("file:ancestor"); !ok || got != Sha256Hex([]byte(outside)) {
		t.Fatal("final symlink did not hash its link text")
	}
}

// regression: #2798
func TestHistoricalScopeNotHeld(t *testing.T) {
	for _, decl := range []string{"files:\n- tools/a.go", "files: `tools/a.go`", "files: `tools/planned.go`", "files: `docs/<slug>.md`", "files (created by phase):\n- tools/a.go", "facts: no declared files"} {
		t.Run(decl, func(t *testing.T) {
			root := t.TempDir()
			brief := "## Context\n" + decl + "\n"
			for p, b := range map[string]string{wakeTestBriefPath: brief, "tools/a.go": "observed"} {
				full := filepath.Join(root, p)
				if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(b), 0644); err != nil {
					t.Fatal(err)
				}
			}
			r := WakeReceipt{Brief: "demo/01", Outcome: "blocked", Schema: SchemaWakeV1, ID: "historical", BlockerKind: BlockerUnknown, BlockerRef: "#1", WakePredicate: WakeRelevantInputChanged, ToolVersion: "v1", Inputs: map[string]string{"file:" + wakeTestBriefPath: Sha256Hex([]byte(brief)), "tool": "v1"}}
			rr := NewRootRevisionReader(root, "v1")
			want, wake := InputsCouldNotCheck, WakeCouldNotCheck
			if strings.HasPrefix(decl, "facts:") {
				want, wake = InputsUnchanged, WakeHold
			}
			if state, why := Unchanged(r, rr); state != want {
				t.Errorf("historical scope: %v %s, want %v", state, why, want)
			}
			if got := r.EvaluateWake(rr, time.Now()); got.State != wake {
				t.Errorf("historical wake: %v, want %v", got, wake)
			}
			r.Inputs["file:tools/a.go"] = "old observation"
			if state, why := Unchanged(r, rr); state != InputsChanged {
				t.Errorf("observed change lost to scope gap: %v %s", state, why)
			}
			if got := r.EvaluateWake(rr, time.Now()); got.State != WakeFire {
				t.Errorf("observed change did not fire: %v", got)
			}
		})
	}
}

// regression: #2798
func TestScopeDeclarationPositions(t *testing.T) {
	for _, extra := range []string{"files:\n- tools/a.go\n\nfiles:\n- tools/pkg/", "files:\n- tools/a.go\n\n## Other\n\n## Context\nfiles: tools/pkg/"} {
		tree := wakeTestTree()
		tree.files[wakeTestBriefPath] = "## Context\n" + extra
		if _, err := BuildOutcomeRecord(wakeTestRecord("blocked", BlockerUnknown, "#1"), wakeTestInput(), tree); ExitCodeOf(err) != ExitRefused {
			t.Errorf("ambiguous declarations accepted: %v", err)
		}
	}
	tree := wakeTestTree()
	tree.files[wakeTestBriefPath] = "## Context\n```text\nfiles:\n- `tools/not-declared.go`\n```\n\nfiles:\n- tools/a.go\n"
	if raw, err := BuildOutcomeRecord(wakeTestRecord("blocked", BlockerUnknown, "#1"), wakeTestInput(), tree); ExitCodeOf(err) != ExitRefused || len(raw) != 0 {
		t.Fatalf("ambiguous fenced declaration published: %s %v", raw, err)
	}
	// A non-structural example remains supported and contributes no file inputs.
	tree.files[wakeTestBriefPath] = "## Context\n```text\n- tools/not-declared.go\n```\n\nfiles:\n- tools/a.go\n"
	raw, err := BuildOutcomeRecord(wakeTestRecord("blocked", BlockerUnknown, "#1"), wakeTestInput(), tree)
	if err != nil {
		t.Fatal(err)
	}
	var r WakeReceipt
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	if r.Inputs["file:tools/a.go"] == "" || r.Inputs["file:tools/not-declared.go"] != "" {
		t.Fatalf("fenced example became declaration: %v", r.Inputs)
	}
}

// regression: #2798 / sec-3429-1. These in-module excerpts preserve the
// declaration positions from the five reviewed briefs without a cross-module read.
func TestScopeReviewedCorpusRefusesPartial(t *testing.T) {
	for _, name := range []string{"derived-board-06", "composability-04", "desk-tools-31", "desk-tools-33", "desk-tools-36"} {
		t.Run(name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("testdata", "declaredscope", name+".md"))
			if err != nil {
				t.Fatal(err)
			}
			if paths, err := BriefDeclaredFiles(string(content)); ExitCodeOf(err) != ExitRefused {
				t.Fatalf("unsupported real declaration silently narrowed: %v, %v", paths, err)
			}
		})
	}
}

// regression: #2798 / sec-3429-1.
func TestScopeContinuationPositions(t *testing.T) {
	for _, content := range []string{
		"## Context\nfiles: `tools/a.go`\n  args: [example]\n- tools/b.go\n",
		"## Context\nfiles: `tools/a.go` - `tools/b.go`\n",
		"## Context\nfiles: `tools/a.go` – `tools/b.go`\n",
		"files: tools/a.go\n",
		"## Task\nfiles: tools/a.go\n",
		"## Context\n**files**: tools/a.go\n",
		"## Context\nfiles tools/a.go\n",
		"## Context\nFiles: tools/a.go\n",
		"## Context\nFILES: tools/a.go\n",
		"## Context\n__files__: tools/a.go\n",
		"## Context\nfiles: tools/a.go, **unclosed\n",
		"## Context\nfiles: tools/a.go\n  facts: tools/b.go\n",
	} {
		if paths, err := BriefDeclaredFiles(content); ExitCodeOf(err) != ExitRefused {
			t.Errorf("silent partial or empty scope: %q: %v %v", content, paths, err)
		}
	}
	paths, err := BriefDeclaredFiles("## Context\nfiles:\n - `tools/a.go`\n `tools/b.go`: desc\nfacts: commentary\n")
	if err != nil || strings.Join(paths, ",") != "tools/a.go,tools/b.go" {
		t.Fatalf("colon declaration lost: %v %v", paths, err)
	}
	paths, err = BriefDeclaredFiles("## Task\n```text\nfiles: tools/example.go\n```\n")
	if ExitCodeOf(err) != ExitRefused {
		t.Fatalf("ambiguous fenced field accepted: %v %v", paths, err)
	}
	paths, err = BriefDeclaredFiles("## Task\n```text\ntools/example.go\n```\n")
	if err != nil || len(paths) != 0 {
		t.Fatalf("fenced example became declaration: %v %v", paths, err)
	}
	paths, err = BriefDeclaredFiles("## Context\nfacts: no deliverables\n")
	if err != nil || len(paths) != 0 {
		t.Fatalf("legitimate absence refused: %v %v", paths, err)
	}
}

// regression: medici-finance/assay#3756
func TestScopeAnnotationThenColon(t *testing.T) {
	for entry, want := range map[string]string{
		// The reported shape: annotation, then a ':' description.
		"- `tools/desk/cmd/deskapps/` (new): `main.go`, `server.go` (loopback HTTP), `manifest.go`": "tools/desk/cmd/deskapps",
		"- `tools/a.go` (new) (edit): desc":            "tools/a.go",
		"- tools/a.go (planned): NEW. desc":            "tools/a.go",
		"- **edit** `tools/a.go` (new): desc":          "tools/a.go",
		"- `tools/a.go` (new), `tools/b.go` (edit): x": "tools/a.go,tools/b.go",
		// Sibling orderings that already parsed keep their scope.
		"- `tools/a.go`: desc (x)":                 "tools/a.go",
		"- `tools/a.go` (new) — desc `tools/b.go`": "tools/a.go",
		"- `tools/a.go` (new) and tests":           "tools/a.go",
		"- `tools/a.go` and tests (new),":          "tools/a.go",
		// An annotation after "and tests" never turns the word into a path.
		"- `tools/a.go` and tests (new)":               "tools/a.go",
		"- `tools/a.go` and tests (new).":              "tools/a.go",
		"- `tools/a.go` and tests (new), `tools/b.go`": "tools/a.go,tools/b.go",
		"- `tools/a.go` and tests (new): desc":         "tools/a.go",
		"- `tools/a.go` and tests.go":                  "tools/a.go,tests.go",
		"- `tools/a.go` and tests/":                    "tools/a.go,tests",
		// A glued annotation is not part of an unquoted path.
		"- tools/a.go(planned)":             "tools/a.go",
		"- tools/a.go(planned): desc":       "tools/a.go",
		"- tools/{a,b}.go(new), tools/c.go": "tools/a.go,tools/b.go,tools/c.go",
	} {
		paths, err := BriefDeclaredFiles("## Context\nfiles:\n" + entry + "\nfacts: x\n")
		if err != nil || strings.Join(paths, ",") != want {
			t.Errorf("%q: got %v %v; want %s", entry, paths, err, want)
		}
	}
	// Garbage around an annotation is still refused, never read as a description.
	for _, entry := range []string{
		"- `tools/a.go` (new) garbage",
		"- `tools/a.go` (new) garbage: desc",
		"- `tools/a.go` (new) `tools/b.go`: desc",
		"- `tools/a.go` (x (y)): desc",
		"- `tools/a.go` (new: desc",
		"- `tools/a.go` and tests garbage",
		"- `tools/a.go` and tests: desc",
		// An annotation in path position is never itself a path.
		"- (new)",
		"- `tools/a.go`\n  (new)",
		"- (new) `tools/a.go`",
	} {
		if paths, err := BriefDeclaredFiles("## Context\nfiles:\n" + entry + "\n"); ExitCodeOf(err) != ExitRefused {
			t.Errorf("%q: got %v %v; want refusal", entry, paths, err)
		}
	}
}

// regression: #3817
func TestScopeAnnotationInSpan(t *testing.T) {
	for entry, want := range map[string]string{
		// The reported real shapes (desktools-v2/63, comms-core, desk-improvement).
		"- `perfrec/context.go (planned)`":                                      "perfrec/context.go",
		"- `comms/server (new)`":                                                "comms/server",
		"- `comms/server/testdata (new legacy-state and host fixtures)`":        "comms/server/testdata",
		"- `wf/testdata/evidence/ (planned synthetic/disposable fixtures)`":     "wf/testdata/evidence",
		"- `schemas/b.json (planned binding manifest, not an event lifecycle)`": "schemas/b.json",
		// Same rule at every other quoted position on the line.
		"- `tools/a.go (new) (edit)`":                  "tools/a.go",
		"- `tools/a.go (new)`: desc `tools/b.go`":      "tools/a.go",
		"- `tools/a.go (new)` (edit): desc":            "tools/a.go",
		"- `tools/a.go (new)` — desc `tools/b.go`":     "tools/a.go",
		"- `tools/a.go (planned)`, `tools/b.go (new)`": "tools/a.go,tools/b.go",
		"- `tools/a.go (new)` and tests":               "tools/a.go",
		"- **edit** `tools/a.go (new)`":                "tools/a.go",
		"- `tools/{a,b}.go (new)`":                     "tools/a.go,tools/b.go",
		// The outside-the-quotes form keeps its scope.
		"- `tools/a.go` (planned)": "tools/a.go",
		// A glued or leading parenthetical is part of the literal path, even
		// after a multibyte character whose last byte is 0x85 or 0xA0.
		"- `tools/a.go(x)`": "tools/a.go(x)",
		"- `tools/à(x)`":    "tools/à(x)",
		"- `tools/Å(x)`":    "tools/Å(x)",
		"- `(new)`":         "(new)",
	} {
		for _, content := range []string{"## Context\nfiles:\n" + entry + "\nfacts: x\n", "## Context\nfiles: " + strings.TrimPrefix(entry, "- ") + "\nfacts: x\n"} {
			paths, err := BriefDeclaredFiles(content)
			if err != nil || strings.Join(paths, ",") != want {
				t.Errorf("%q: got %v %v; want %s", content, paths, err, want)
			}
		}
	}
	// Only a trailing, non-empty, unnested parenthetical after whitespace is an
	// annotation; every other space inside the span is still refused.
	for _, entry := range []string{
		"- `tools/my dir/a.go (new)`",
		"- `tools/a.go (new) trailing`",
		"- `tools/a.go (new`",
		"- `tools/a.go new)`",
		"- `tools/a.go (x (y))`",
		"- `tools/a.go (x) y)`",
		"- `tools/a.go ()`",
		"- `tools/a.go ( )`",
		"- `tools/a.go (new) `",
		"- ` (new)`",
		"- `tools/a.go (new)` garbage",
	} {
		if paths, err := BriefDeclaredFiles("## Context\nfiles:\n" + entry + "\n"); ExitCodeOf(err) != ExitRefused {
			t.Errorf("%q: got %v %v; want refusal", entry, paths, err)
		}
	}
}

type scopeWorkTree struct {
	OutcomeTree
	calls int
}

func (s *scopeWorkTree) PathKind(p string) (string, error) {
	s.calls++
	return s.OutcomeTree.PathKind(p)
}

// regression: #2798 / sec-3429-2. Repeated overlapping matches must have a
// total work bound even when the distinct-file cap never trips.
func TestScopeResolutionWorkBound(t *testing.T) {
	tree := fakeTree{files: map[string]string{}}
	for i := 0; i < 100; i++ {
		tree.files[fmt.Sprintf("pkg/f%03d.go", i)] = "content"
	}
	counted := &scopeWorkTree{OutcomeTree: tree}
	declarations := make([]string, 4096)
	for i := range declarations {
		declarations[i] = "pkg/*"
	}
	_, err := ResolveDeclaredScope(declarations, counted)
	if ExitCodeOf(err) != ExitRefused {
		t.Fatalf("unbounded duplicate work accepted: %v", err)
	}
	if counted.calls > 32768 {
		t.Fatalf("work limit checked too late: %d reads", counted.calls)
	}
}

// regression: #2798 / R3429-A3.
func TestScopeLooseContinuation(t *testing.T) {
	if paths, err := BriefDeclaredFiles("## Context\nfiles: tools/a.go\n\n- tools/b.go\n"); ExitCodeOf(err) != ExitRefused {
		t.Fatalf("loose list silently narrowed: %v %v", paths, err)
	}
	paths, err := BriefDeclaredFiles("## Context\nfiles: tools/a.go\n\nfacts:\n- `tools/not-declared.go` is commentary\n")
	if err != nil || strings.Join(paths, ",") != "tools/a.go" {
		t.Fatalf("next field confused with loose list: %v %v", paths, err)
	}
}
