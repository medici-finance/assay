package purity_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/deskd/internal/purity"
)

// writePkg materialises a one-file Go package in a temp dir.
func writePkg(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "types.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestLintCatchesTheSpecNamedLeaks is the POSITIVE CONTROL for the
// domain-purity lint — the read surface Verify row 10, automated.
//
// Row 10 describes a manual mutation: add `HTMLURL string` to a domain type,
// run the lint, revert. A manual mutation is a claim someone made once. This
// runs the same mutation on every CI invocation, for each identifier spec
// §1.5 names by hand, and fails if any of them slips through — so the lint
// ships with proof it can fail, not merely proof it can pass.
func TestLintCatchesTheSpecNamedLeaks(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string // substring of the offending declaration name
	}{
		{
			name: "HTMLURL field — the row-10 mutation verbatim",
			src:  "package d\n\ntype Escalation struct {\n\tHTMLURL string\n}\n",
			want: "HTMLURL",
		},
		{
			name: "NodeID field — spec §1.5's second named pattern",
			src:  "package d\n\ntype Escalation struct {\n\tNodeID string\n}\n",
			want: "NodeID",
		},
		{
			name: "the numeric platform key, spelled as a field",
			src:  "package d\n\ntype Escalation struct {\n\tIssueNumber int\n}\n",
			want: "IssueNumber",
		},
		{
			name: "snake_case spelling of the same concept",
			src:  "package d\n\ntype Escalation struct {\n\tnode_id string\n}\n",
			want: "node_id",
		},
		{
			name: "renamed field, leaked through its json tag",
			src:  "package d\n\ntype Escalation struct {\n\tLink string `json:\"html_url\"`\n}\n",
			want: "html_url",
		},
		{
			name: "type name, not just a field",
			src:  "package d\n\ntype IssueNumberMap map[string]int\n",
			want: "IssueNumberMap",
		},
		{
			name: "method name",
			src:  "package d\n\ntype E struct{}\n\nfunc (E) HTMLURL() string { return \"\" }\n",
			want: "HTMLURL",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings, err := purity.CheckDir(writePkg(t, tc.src))
			if err != nil {
				t.Fatalf("could-not-check: lint errored: %v", err)
			}
			if len(findings) == 0 {
				t.Fatalf("POSITIVE CONTROL SURVIVED: the lint passed a package containing %q.\n"+
					"This lint is not load-bearing — a real leak would ship green.", tc.want)
			}
			var joined []string
			for _, f := range findings {
				joined = append(joined, f.Name)
			}
			if !strings.Contains(strings.Join(joined, " "), tc.want) {
				t.Fatalf("lint fired, but on the wrong declaration: got %v, want one naming %q", joined, tc.want)
			}
			// Every finding must explain the rule, or the reader renames
			// around it instead of learning it.
			for _, f := range findings {
				if f.Pattern.Why == "" {
					t.Errorf("finding %q carries no rationale", f.Name)
				}
			}
		})
	}
}

// TestLintDoesNotFireOnCleanDomainShapes guards the other direction: a lint
// that fires on everything gets silenced, and a silenced lint catches nothing.
func TestLintDoesNotFireOnCleanDomainShapes(t *testing.T) {
	src := `package d

// Nothing here names the platform. Prose may discuss GitHub freely — this is
// a declaration lint, not a grep, which is the reason it can afford to be
// aggressive on identifiers.
type EscalationID string

type Escalation struct {
	ID    EscalationID ` + "`json:\"id\"`" + `
	Title string       ` + "`json:\"title\"`" + `
	// A number that is ours, not the platform's.
	Repeat int ` + "`json:\"repeat\"`" + `
}

func (e Escalation) Attributable() bool { return true }
`
	findings, err := purity.CheckDir(writePkg(t, src))
	if err != nil {
		t.Fatalf("could-not-check: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("false positives on a clean package: %v", findings)
	}
}

// TestLintSkipsTestFiles — an adapter test may legitimately construct a
// platform payload to prove the adapter strips it.
func TestLintSkipsTestFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x_test.go"),
		[]byte("package d\n\ntype fixture struct{ HTMLURL string }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, err := purity.CheckDir(dir)
	if err != nil {
		t.Fatalf("could-not-check: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("lint fired inside a _test.go file: %v", findings)
	}
}
