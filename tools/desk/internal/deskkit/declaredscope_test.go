package deskkit

import (
	"encoding/json"
	"os"
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
				if _, ok := r.Inputs["file:"+p]; !ok {
					t.Errorf("omitted declared deliverable %s", p)
				}
			}
			if _, ok := r.Inputs["file:tools/not-declared.go"]; ok {
				t.Fatal("commentary became a declaration")
			}
		})
	}
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
