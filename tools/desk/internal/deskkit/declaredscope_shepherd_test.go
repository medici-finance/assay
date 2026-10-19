package deskkit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// regression: #2798 / R3429-F3. Metadata and prose must not change a valid scope.
func TestScopeAcceptedFieldBoundaries(t *testing.T) {
	for _, content := range []string{
		"## Context\nfiles: tools/a.go\n\nfacts (date; check at pickup):\n- `tools/comment.go` is a fact\n",
		"## Context\nfiles: tools/a.go\nfacts (source: local):\n- tools/comment.go\n",
		"## Context\nfiles: tools/a.go\n\nfacts:\n- tools/comment.go\n\n## Task\nFiles are written atomically.\n  files are discarded.\nFILES the signed result today.\n",
		"## Context\nfiles: tools/a.go\n\nlayout: root module owns the interface\n  implementation detail\n\nfacts:\n- value\n\n## Task\n`files:` entry is described here.\n",
		"## Context\nfiles: tools/a.go\n\nout-of-repo files: handled by a separate actor\n  external scope is not read here\n\nfacts:\n- value\n",
		"## Context\nfiles: tools/a.go   \n\nfacts:\n- value\n",
		"## Context\r\nfiles: tools/a.go\r\n\r\nfacts (date):\r\n- value\r\n",
		"## Context\nfacts:\n````text\n```\nfiles: tools/comment.go\n````\nfiles: tools/a.go\n",
	} {
		t.Run(content, func(t *testing.T) {
			paths, err := BriefDeclaredFiles(content)
			if err != nil || strings.Join(paths, ",") != "tools/a.go" {
				t.Fatalf("valid declaration rejected or changed: %v %v", paths, err)
			}
		})
	}
}

// regression: #2798 / sec-3429-1. Formatting cannot hide a declaration marker.
func TestScopeFormattedMarkersRefuse(t *testing.T) {
	for _, marker := range []string{
		"- files:", "* files:", "+ files:", "1. files:", "2) files:",
		"> files:", "`files:`", "file:", "-\tfiles:", "> **Files**:",
		"__file__:", "* `files:`", "**files (planned)**:",
	} {
		content := "## Context\n" + marker + " `tools/a.go`\n"
		if paths, err := BriefDeclaredFiles(content); ExitCodeOf(err) != ExitRefused {
			t.Errorf("unsupported marker silently ignored: %q: %v %v", marker, paths, err)
		}
	}
}

// regression: #2798 / sec-3429-1. A loose list must never lose its later members.
func TestScopeLooseMarkdownContinuationsRefuse(t *testing.T) {
	for _, continuation := range []string{
		"- `tools/b.go`", "-\t`tools/b.go`", "* `tools/b.go`", "+ `tools/b.go`",
		"1. `tools/b.go`", "2) `tools/b.go`", "`tools/b.go`", "  `tools/b.go`",
		"\ttools/b.go", "> `tools/b.go`", "-\u2003`tools/b.go`",
	} {
		content := "## Context\nfiles: tools/a.go\n\n" + continuation + "\n"
		if paths, err := BriefDeclaredFiles(content); ExitCodeOf(err) != ExitRefused {
			t.Errorf("loose continuation silently dropped: %q: %v %v", continuation, paths, err)
		}
	}
	for _, separator := range []string{"\n", "\n\n"} {
		content := "## Context\nfiles: tools/a.go" + separator + "see: section 4\n- tools/b.go\n"
		if paths, err := BriefDeclaredFiles(content); ExitCodeOf(err) != ExitRefused {
			t.Errorf("unknown lazy key silently ends field: %v %v", paths, err)
		}
	}
}

// regression: #2798 / R3429-F3. The recovered syntax must reach the real writer
// and wake reader with every declared file, rather than merely parse successfully.
func TestScopeAnnotatedReceiptFlow(t *testing.T) {
	tree := wakeTestTree()
	brief := "## Context\nfiles: tools/a.go\n\nfacts (date; recheck at pickup):\n- `tools/not-declared.go` is prose\n\n## Task\nFiles are written atomically.\n"
	tree.files[wakeTestBriefPath] = brief
	input := wakeTestInput()
	input.LandedBrief = []byte(brief)
	raw, err := BuildOutcomeRecord(wakeTestRecord("blocked", BlockerUnknown, "#1"), input, tree)
	if err != nil {
		t.Fatal(err)
	}
	var receipt WakeReceipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Inputs["file:tools/a.go"] != Sha256Hex([]byte(tree.files["tools/a.go"])) {
		t.Fatal("accepted declaration lost its observed file hash")
	}
	if _, ok := receipt.Inputs["file:tools/not-declared.go"]; ok {
		t.Fatal("facts became receipt input")
	}
	receipt.Brief = "demo/01"
	root := t.TempDir()
	for p, content := range tree.files {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	reader := NewRootRevisionReader(root, input.ToolVersion)
	if state, why := Unchanged(receipt, reader); state != InputsUnchanged {
		t.Fatalf("complete accepted scope cannot hold: %v %s", state, why)
	}
	if err := os.WriteFile(filepath.Join(root, "tools/a.go"), []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	if receipt.EvaluateWake(reader, time.Now()).State != WakeFire {
		t.Fatal("declared file change did not wake accepted receipt")
	}
}

// regression: #2798 / sec-3429-1. Malformed fences are not evidence of no inputs.
func TestScopeFenceIntegrity(t *testing.T) {
	for _, content := range []string{
		"## Context\nfacts:\n```text\nfiles: tools/a.go\n",
		"## Context\nfacts:\n````text\n```\nfiles: tools/a.go\n",
		"## Context\nfiles: tools/a.go\n````text\n```\n- tools/b.go\n",
		"## Context\nfiles: tools/a.go\n~~~text\n```\n- tools/b.go\n",
	} {
		if paths, err := BriefDeclaredFiles(content); ExitCodeOf(err) != ExitRefused {
			t.Errorf("unclosed fence silently hides declarations: %v %v", paths, err)
		}
	}
}
