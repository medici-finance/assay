package deskkit

// barecallers_test.go — the CLASS guard for #6267: a caller that already knows which kind of
// object it means (an issue, or a pull/merge request) must not ask the bare-number resolver.
//
// GetIssue(repo, n) and PostComment(repo, n, body) take a bare number and, on GitLab, resolve
// its kind themselves: issues and merge requests are numbered in SEPARATE sequences, so a
// project routinely carries issue #N and merge request !N at once, and the resolver refuses
// such a number could-not-check. That refusal is correct for a caller that genuinely does not
// know the kind. For a caller that does (#6264: the issue board's RETIRE read and the scan
// loop's trust read, both of which ask about ISSUES) it turns every colliding number into a
// permanent could-not-check. The typed operations (GetIssueTyped, PostCommentTyped) take the
// kind and read or write only that object.
//
// This scan enumerates every `.GetIssue(` and `.PostComment(` call in the desk tree's non-test
// sources and fails on any call not on the allow-list below. A new caller has to be ruled on
// here — converted to the typed operation, or listed with the reason its kind is genuinely
// unknown — before it can land; a listed caller that disappears has to be removed, so the
// list cannot go stale.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// bareResolverMethods are the bare-number entry points the guard counts.
var bareResolverMethods = map[string]bool{"GetIssue": true, "PostComment": true}

// bareResolverAllowList maps "<file relative to tools/desk>#<method>" to the number of calls
// and WHY each one is not a known-kind caller.
var bareResolverAllowList = map[string]struct {
	calls  int
	reason string
}{
	// --- callers whose kind is genuinely unknown: the resolver decides it, or refuses ---
	"cmd/deskclose/github.go#GetIssue": {1, "only when no --kind was given; with --kind the read is GetIssueTyped"},
	"cmd/desklabel/verbs.go#GetIssue":  {1, "only when no --kind was given; with --kind the read is GetIssueTyped"},
	"cmd/deskpathguard/check.go#GetIssue": {1, "only when no --kind was given; with --kind mr the read is " +
		"GetIssueTyped(TargetChange)"},
	"cmd/deskpost/forgeclient.go#GetIssue": {1, "kind discovery: resolveTarget/requirePRErr read the number to " +
		"LEARN whether it is an issue or a PR"},
	"cmd/deskpost/comment.go#PostComment": {1, "only when no --kind was given; a forced kind writes through " +
		"PostCommentTyped"},
	"cmd/deskreply/exec.go#PostComment": {1, "only when no --kind was given; with a kind postReply writes " +
		"through PostCommentTyped"},
	"cmd/deskdispatch/reviewgate.go#GetIssue": {1, "a bare #N cited in a review verdict's prerequisite " +
		"declaration names no kind"},
	"cmd/verifyloop/blockerread.go#GetIssue": {1, "only for a bare #N blocker_ref; /issues/ and /pull/ URLs " +
		"read through GetIssueTyped"},
	"cmd/deskevidence/receiptvalidate.go#GetIssue": {1, "only for a bare #N blocker_ref; /issues/ and /pull/ " +
		"URLs read through GetIssueTyped"},

	// --- the decision-gate family: held fail-closed on a both-kinds number ---
	// TestDecisionMarkerSearchSingleAndSweepParity/gitlab-collision pins that a GitLab kind
	// ambiguity on a decision-issue read is NOT waived. Typing these reads would waive it, so
	// they stay on the bare resolver until that is ruled on; this sweep does not loosen a gate.
	"internal/deskkit/briefadmission.go#GetIssue": {2, "decision-gate read, pinned fail-closed on a GitLab " +
		"both-kinds number (TestDecisionMarkerSearchSingleAndSweepParity)"},
	"internal/deskkit/decisionsweep.go#GetIssue": {1, "the shared decision sweep's cached read; the same " +
		"pinned fail-closed decision-gate read"},
	"internal/deskkit/decisionensure.go#GetIssue": {3, "decision-issue dedupe reads, kept on the same " +
		"fail-closed resolver as the decision-gate wait so the two cannot disagree on one number"},

	// --- the entry points themselves: implementations and pure delegation ---
	"internal/deskkit/readonlyforge.go#GetIssue": {1, "the read-only decorator delegating the bare entry point"},
	"internal/deskkit/outboundforge.go#PostComment": {1, "the outbound decorator delegating the bare " +
		"entry point after its text check"},
	"internal/deskkit/forge_github.go#GetIssue": {1, "GitHub's GetIssueTyped: one number sequence, so the " +
		"one read answers both kinds and the stated kind is then validated"},
	"internal/deskkit/forge_github.go#PostComment": {1, "GitHub's PostCommentTyped: one comments endpoint " +
		"serves both kinds"},
	"internal/deskkit/forge_gitlab.go#GetIssue": {3, "the GitLab client library's issue-endpoint call " +
		"(Issues.GetIssue), which addresses the issue sequence only"},
}

// bareResolverCallers walks tops under root (non-test Go files, testdata skipped) and counts
// the bare-resolver selector calls per "<file>#<method>", keyed relative to root with forward
// slashes. It returns the number of files parsed, so "found none" is distinguishable from
// "looked at nothing".
func bareResolverCallers(root string, tops []string) (map[string]int, int, error) {
	hits := map[string]int{}
	scanned := 0
	fset := token.NewFileSet()
	for _, top := range tops {
		base := filepath.Join(root, top)
		if _, err := os.Stat(base); err != nil {
			return nil, 0, err
		}
		err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				return rerr
			}
			af, perr := parser.ParseFile(fset, path, nil, 0) // no ParseComments: only code decides
			if perr != nil {
				return perr
			}
			scanned++
			ast.Inspect(af, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && bareResolverMethods[sel.Sel.Name] {
					hits[filepath.ToSlash(rel)+"#"+sel.Sel.Name]++
				}
				return true
			})
			return nil
		})
		if err != nil {
			return nil, 0, err
		}
	}
	return hits, scanned, nil
}

// regression: class #6267
func TestBareResolverCallers(t *testing.T) {
	root := filepath.Join("..", "..") // tools/desk, from tools/desk/internal/deskkit
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("cannot find the desk module root at %s: %v", root, err)
	}
	hits, scanned, err := bareResolverCallers(root, []string{"cmd", "internal"})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if scanned == 0 {
		t.Fatal("scanned 0 source files — the enumeration proved nothing")
	}
	var problems []string
	for key, n := range hits {
		want, listed := bareResolverAllowList[key]
		switch {
		case !listed:
			problems = append(problems, key+": a NEW bare-number resolver call. If the caller knows the kind "+
				"(an issue, or a pull/merge request), use GetIssueTyped / PostCommentTyped — on GitLab issue #N "+
				"and merge request !N coexist and the bare resolver refuses such a number (#6264). Only a "+
				"caller whose kind is genuinely unknown belongs on bareResolverAllowList, with its reason")
		case n != want.calls:
			problems = append(problems, key+": "+strconv.Itoa(n)+" call(s), allow-list says "+
				strconv.Itoa(want.calls)+" — rule on the change and update the entry")
		}
	}
	for key := range bareResolverAllowList {
		if hits[key] == 0 {
			problems = append(problems, key+": on the allow-list but no longer makes that call — remove the stale entry")
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		t.Fatalf("bare-resolver caller allow-list is out of step with the tree:\n  %s", strings.Join(problems, "\n  "))
	}
}

// The scanner is not vacuous: a planted caller of either entry point is found, the typed
// operations are not counted, and test files and testdata are skipped.
func TestBareResolverPlanted(t *testing.T) {
	root := t.TempDir()
	write := func(rel, src string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const iface = "interface{ GetIssue(string, int) error; GetIssueTyped(string, int, string) error; " +
		"PostComment(string, int, string) error; PostCommentTyped(string, int, string, string) error }"
	write("cmd/planted/read.go", "package main\n\nfunc r(f "+iface+") { _ = f.GetIssue(\"o/r\", 9) }\n")
	write("internal/planted/post.go", "package planted\n\nfunc p(f "+iface+") { _ = f.PostComment(\"o/r\", 9, \"x\") }\n")
	write("cmd/typed/typed.go", "package main\n\nfunc t(f "+iface+") { _ = f.GetIssueTyped(\"o/r\", 9, \"issue\"); "+
		"_ = f.PostCommentTyped(\"o/r\", 9, \"change\", \"x\") }\n")
	write("cmd/planted/read_test.go", "package main\n\nfunc u(f "+iface+") { _ = f.GetIssue(\"o/r\", 9) }\n")
	write("cmd/planted/testdata/fixture.go", "package fixture\n\nfunc v(f "+iface+") { _ = f.PostComment(\"o/r\", 9, \"x\") }\n")
	hits, scanned, err := bareResolverCallers(root, []string{"cmd", "internal"})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	want := map[string]int{"cmd/planted/read.go#GetIssue": 1, "internal/planted/post.go#PostComment": 1}
	if scanned != 3 {
		t.Fatalf("scanned %d files, want 3 (test files and testdata excluded)", scanned)
	}
	if len(hits) != len(want) {
		t.Fatalf("hits = %v, want %v", hits, want)
	}
	for key, n := range want {
		if hits[key] != n {
			t.Fatalf("hits = %v, want %v", hits, want)
		}
	}
}
