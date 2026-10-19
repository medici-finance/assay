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
		"## Context\nfacts:\n````text\n```\ntools/comment.go\n````\nfiles: tools/a.go\n",
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
	cases := []string{
		"## Context\nfacts:\n```text\nfiles: tools/a.go\n",
		"## Context\nfiles: tools/a.go\nfacts:\n```text\nexample\n",
		"## Context\nfacts:\n````text\n```\nfiles: tools/a.go\n",
		"## Context\nfiles: tools/a.go\n````text\n```\n- tools/b.go\n",
		"## Context\nfiles: tools/a.go\n~~~text\n```\n- tools/b.go\n",
		"## Context\n```text``` blocks are examples only.\nfiles: tools/a.go\n\n## Task\n```text\nz\n```\n",
		"## Context\nfacts:\n    ```\nfiles: tools/a.go\n\n## Task\n```text\nz\n```\n",
		"## Context\nfiles:\n- tools/a.go\n```` ```go ```` snippets live in:\n- tools/b.go\n\n## Task\n````text\nz\n````\n",
		"## Context\nfacts:\n\t```\nfiles: tools/a.go\n\n## Task\n```text\nz\n```\n",
		"## Context\nfacts:\n```text\n    ```\nfiles: tools/a.go\n",
	}
	cases = append(cases, scopeContainerFenceCases()...)
	cases = append(cases,
		"## Context\nfacts:\n````text\n```\nfiles: tools/comment.go\n````\nfiles: tools/a.go\n",
		"## Context\nfacts:\n   ```text\nfiles: tools/not-declared.go\n    ```\nfiles: tools/not-declared-either.go\n   ```\nfiles: tools/a.go\n",
		"## Context\nfacts:\n~~~example `code`\nfiles: tools/not-declared.go\n~~~\nfiles: tools/a.go\n",
		"## Context\nfiles: tools/a.go\nfacts:\n- item\n  ```\n  example\n  ```\n",
		"## Context\nfiles: tools/a.go\n```text\nexample\n```\nfacts: x\n",
		"## Context\nfiles: tools/a.go\n\n```text\nexample\n```\nfacts: x\n",
		"# T\n```text\n## Example\n```\n## Context\nfiles: tools/a.go\n",
		"## Task\n```text\nfiles: tools/example.go\n```\n",
	)
	for _, content := range cases {
		if paths, err := BriefDeclaredFiles(content); ExitCodeOf(err) != ExitRefused {
			t.Errorf("unclosed fence silently hides declarations: %v %v", paths, err)
		}
	}
}

// regression: #2798 / sec-3429-1. Inline spans outside Context are prose;
// valid fence indentation and info strings must keep their Markdown meaning.
func TestScopeFenceOpeners(t *testing.T) {
	for _, content := range []string{
		"```` ```go ```` snippets are examples.\n\n## Context\nfiles: tools/a.go\n",
		"    ```\n\n## Context\nfiles: tools/a.go\n",
		"   ```text\ntools/not-declared.go\n    ```\ntools/not-declared-either.go\n   ```\n## Context\nfiles: tools/a.go\n",
		"## Context\nfacts:\n~~~example `code`\ntools/not-declared.go\n~~~\nfiles: tools/a.go\n",
	} {
		paths, err := BriefDeclaredFiles(content)
		if err != nil || strings.Join(paths, ",") != "tools/a.go" {
			t.Errorf("fence opener changed declared scope: %v %v", paths, err)
		}
	}
}

// regression: #2798 / R3429-F4. A paragraph before Context cannot open a
// false fence that a later real code block closes over both declared inputs.
func TestScopeInlineSpanBeforeContext(t *testing.T) {
	brief := "# T\n```verdict-payload``` blocks are described below.\n\n## Context\nfiles:\n- `tools/a.go`\n- `tools/b.go`\n\n## Task\n```go\nz\n```\n"
	paths, err := BriefDeclaredFiles(brief)
	if err != nil || strings.Join(paths, ",") != "tools/a.go,tools/b.go" {
		t.Fatalf("line-leading inline span hid Context: %v %v", paths, err)
	}
}

// regression: #2798 / sec-3429-1. The real writer must refuse the malformed
// opener instead of publishing a receipt with its deliverable omitted.
func TestScopeMalformedFenceReceiptRefuses(t *testing.T) {
	for _, marker := range []string{"```text``` examples only.", "    ```", "\t```"} {
		brief := "## Context\nfacts:\n" + marker + "\nfiles: tools/a.go\n\n## Task\n```text\nz\n```\n"
		tree := wakeTestTree()
		tree.files[wakeTestBriefPath] = brief
		input := wakeTestInput()
		input.LandedBrief = []byte(brief)
		if raw, err := BuildOutcomeRecord(wakeTestRecord("blocked", BlockerUnknown, "#1"), input, tree); ExitCodeOf(err) != ExitRefused || len(raw) != 0 {
			t.Errorf("malformed fence published incomplete receipt: %q: %s %v", marker, raw, err)
		}
	}
}

// regression: #2798 / sec-3429-1. A synthetic receipt with the old omitted
// scope must become could-not-check, without rewriting its observed inputs.
func TestScopeMalformedFenceHistoricalHold(t *testing.T) {
	brief := "## Context\n```text``` examples only.\nfiles: tools/a.go\n\n## Task\n```text\nz\n```\n"
	root := t.TempDir()
	full := filepath.Join(root, wakeTestBriefPath)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(brief), 0644); err != nil {
		t.Fatal(err)
	}
	receipt := WakeReceipt{
		Brief: "demo/01", Schema: SchemaWakeV1, ID: "synthetic-incomplete-scope",
		BlockerKind: BlockerUnknown, WakePredicate: WakeRelevantInputChanged,
		Inputs: map[string]string{"file:" + wakeTestBriefPath: Sha256Hex([]byte(brief)), "tool": "v9.9.9"},
	}
	reader := NewRootRevisionReader(root, "v9.9.9")
	if state, why := Unchanged(receipt, reader); state != InputsCouldNotCheck {
		t.Fatalf("malformed historical scope established unchangedness: %v %s", state, why)
	}
	if got := receipt.EvaluateWake(reader, time.Now()); got.State != WakeCouldNotCheck {
		t.Fatalf("malformed historical scope silently held work: %+v", got)
	}
}

// The six security probes and three correctness indentation probes cannot yield a partial
// successful scope. They remain byte-for-byte probes of the reported grammar.
func scopeContainerFenceCases() []string {
	return []string{
		"## Context\nfacts:\n- item\n  ```\n  example\nfiles: tools/a.go\n\n## Task\n```text\nz\n```\n",
		"## Context\nfiles:\n- tools/a.go\n  ```\n  example\n- tools/b.go\n\n## Task\n```text\nz\n```\n",
		"## Context\nfiles:\n- tools/a.go\n  ```\n  ex\n- tools/b.go\nfacts: x\n```text\nz\n```\n",
		"## Context\nfacts:\n<!--\n```\n-->\nfiles: tools/a.go\n\n## Task\n```text\nz\n```\n",
		"# T\n1. step\n   ```sh\n   run\n2. next\n\n## Context\nfiles: tools/a.go\n\n## Task\n```text\nz\n```\n",
		"# T\n<!--\n```\n-->\n\n## Context\nfiles: tools/a.go\n\n## Task\n```text\nz\n```\n",
		"# T\n- step:\n    ```sh\n    run\n  ```\n\n## Context\nfiles:\n- `tools/a.go`\n- `tools/b.go`\n\n## Task\n```go\nz\n```\n",
		"# T\n- step:\n  ```sh\n  run\n    ```\n\n## Context\nfiles:\n- `tools/a.go`\n- `tools/b.go`\n\n## Task\n```go\nz\n```\n",
		"# T\n- step:\n  ```sh\n  run\n\t```\n\n## Context\nfiles:\n- `tools/a.go`\n- `tools/b.go`\n\n## Task\n```go\nz\n```\n",
	}
}

// regression: #2798 / sec-3429-1 round4 / R3429-F4. The real writer must refuse all
// container cases rather than publishing successful empty or partial receipts.
func TestScopeContainerFenceReceiptRefuses(t *testing.T) {
	for _, brief := range scopeContainerFenceCases() {
		tree := wakeTestTree()
		tree.files[wakeTestBriefPath] = brief
		input := wakeTestInput()
		input.LandedBrief = []byte(brief)
		if raw, err := BuildOutcomeRecord(wakeTestRecord("blocked", BlockerUnknown, "#1"), input, tree); ExitCodeOf(err) != ExitRefused || len(raw) != 0 {
			t.Errorf("container fence published partial receipt: %s %v", raw, err)
		}
	}
}

// regression: #2798 / R3429-F4. Symmetric examples that leave both declaration
// lines visible keep their complete scope; asymmetric skipping must refuse above.
func TestScopeBalancedNestedExamples(t *testing.T) {
	for _, indent := range []string{"  ", "    "} {
		brief := "# T\n- step:\n" + indent + "```sh\n" + indent + "run\n" + indent + "```\n\n## Context\nfiles:\n- `tools/a.go`\n- `tools/b.go`\n\n## Task\n```go\nz\n```\n"
		paths, err := BriefDeclaredFiles(brief)
		if err != nil || strings.Join(paths, ",") != "tools/a.go,tools/b.go" {
			t.Fatalf("balanced example changed visible declarations: %v %v", paths, err)
		}
	}
}
