package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #2798
func TestScopeCoverageForms(t *testing.T) {
	for _, decl := range []string{"files:\n- foo/", "files: `foo/*.go`", "files:\n- `foo/{bar,baz}.go`"} {
		t.Run(decl, func(t *testing.T) {
			root := t.TempDir()
			rvGitIn(t, root, "init", "-q", "-b", "main")
			rvWriteAndAdd(t, root, receiptBriefPath, "## Context\n"+decl+"\n")
			rvWriteAndAdd(t, root, "foo/bar.go", "bar")
			rvWriteAndAdd(t, root, "foo/baz.go", "baz")
			rvGitIn(t, root, "commit", "-qm", "declared inputs")
			wr := deskkit.WakeReceipt{Brief: "example-stream/14", SHA: rvGitIn(t, root, "rev-parse", "HEAD"), Inputs: map[string]string{"file:foo/bar.go": "present"}}
			if err := validateReceiptInputsCoverDeliverables(root, wr); deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(err.Error(), "foo/baz.go") {
				t.Fatalf("partial scope accepted: %v", err)
			}
		})
	}
}

// regression: #2798
func TestScopeShapeKeepsAdmission(t *testing.T) {
	for _, decl := range []string{"files:\n- foo/", "files: `foo/*.go`", "files:\n- `foo/{bar,baz}.go`"} {
		t.Run(decl, func(t *testing.T) {
			root := t.TempDir()
			rvGitIn(t, root, "init", "-q", "-b", "main")
			rvWriteAndAdd(t, root, receiptBriefPath, "## Context\n"+decl+"\n")
			rvWriteAndAdd(t, root, "foo/bar.go", "observed failure")
			rvGitIn(t, root, "commit", "-qm", "observed source")
			receipt := deskkit.VerifierReceipt{Binding: deskkit.VerifierBinding{Brief: receiptBriefPath, Source: rvGitIn(t, root, "rev-parse", "HEAD")}}
			rvWriteAndAdd(t, root, "foo/bar.go", "repaired bytes")
			rvGitIn(t, root, "commit", "-qm", "content repair")
			if err := validateOutcomeInputShape(root, receipt, rvGitIn(t, root, "rev-parse", "HEAD")); err != nil {
				t.Fatalf("content repair changed membership: %v", err)
			}
			rvWriteAndAdd(t, root, "foo/baz.go", "new member")
			rvGitIn(t, root, "commit", "-qm", "new member")
			if err := validateOutcomeInputShape(root, receipt, rvGitIn(t, root, "rev-parse", "HEAD")); deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(err.Error(), "fresh independent verification") {
				t.Fatalf("new member reused old admission: %v", err)
			}
		})
	}
}

// Only the ../<sibling>/ list changes between Source and Evidence; the local
// shapes stay equal, so the sibling-set comparison alone must refuse.
// regression: #3483
func TestSiblingSetKeepsAdmission(t *testing.T) {
	const local = "## Context\nfiles:\n- `foo/bar.go`\n"
	for _, tc := range []struct{ name, after string }{
		{"swapped", local + "- `../two/x.go`\n"},
		{"added", local + "- `../one/x.go`\n- `../one/y.go`\n"},
		{"dropped", local},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			rvGitIn(t, root, "init", "-q", "-b", "main")
			rvWriteAndAdd(t, root, receiptBriefPath, local+"- `../one/x.go`\n")
			rvWriteAndAdd(t, root, "foo/bar.go", "observed failure")
			rvGitIn(t, root, "commit", "-qm", "observed source")
			receipt := deskkit.VerifierReceipt{Binding: deskkit.VerifierBinding{Brief: receiptBriefPath, Source: rvGitIn(t, root, "rev-parse", "HEAD")}}
			rvWriteAndAdd(t, root, "foo/bar.go", "repaired bytes")
			rvGitIn(t, root, "commit", "-qm", "content repair")
			if err := validateOutcomeInputShape(root, receipt, rvGitIn(t, root, "rev-parse", "HEAD")); err != nil {
				t.Fatalf("unchanged sibling set refused: %v", err)
			}
			rvWriteAndAdd(t, root, receiptBriefPath, tc.after)
			rvGitIn(t, root, "commit", "-qm", "sibling list edited")
			if err := validateOutcomeInputShape(root, receipt, rvGitIn(t, root, "rev-parse", "HEAD")); deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(err.Error(), "fresh independent verification") {
				t.Fatalf("changed sibling set reused old admission: %v", err)
			}
		})
	}
}

// On this validator path the sibling coverage check is the only thing that
// binds a ../<sibling>/ declaration: an unbound or wrongly bound sibling is
// refused even though every local file is covered.
// regression: #3483
func TestSiblingCoverageValidator(t *testing.T) {
	root := t.TempDir()
	rvGitIn(t, root, "init", "-q", "-b", "main")
	rvWriteAndAdd(t, root, receiptBriefPath, "## Context\nfiles:\n- `foo/bar.go`\n- `../one/x.go`\n")
	rvWriteAndAdd(t, root, "foo/bar.go", "bar")
	rvGitIn(t, root, "commit", "-qm", "declared inputs")
	sha := rvGitIn(t, root, "rev-parse", "HEAD")
	const source = "0123456789abcdef0123456789abcdef01234567"
	for _, tc := range []struct {
		name, key string
		ok        bool
	}{
		{"bound", "repo-source:example-org/one", true},
		{"unbound", "", false},
		{"wrong-sibling", "repo-source:example-org/other", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inputs := map[string]string{"file:foo/bar.go": "present"}
			if tc.key != "" {
				inputs[tc.key] = source
			}
			wr := deskkit.WakeReceipt{Brief: "example-stream/14", SHA: sha, Inputs: inputs}
			err := validateReceiptInputsCoverDeliverables(root, wr)
			if tc.ok && err != nil {
				t.Fatalf("bound sibling refused: %v", err)
			}
			if !tc.ok && (deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(err.Error(), "../one/x.go")) {
				t.Fatalf("sibling admitted without its binding: %v", err)
			}
		})
	}
}
