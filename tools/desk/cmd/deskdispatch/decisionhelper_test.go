package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// decisionhelper_test.go — the decision helper (tools/decision-issue.sh) resolves the way the
// claim tool does: under the resolved root (--claim-root when given, else --root), then PATH.
// It has no PATH port, so a target repo that does not carry the script and was dispatched with
// no --claim-root must still REFUSE a human-gated item — never skip the gate — and the refusal
// must name every location tried plus the --claim-root way out. The field failure: a
// human-gated dispatch from a repo without the script claimed fine through the PATH claim
// binary, then stopped at decision-gate naming only "<root>", with nothing pointing at the flag
// that resolves it.

// plantOnly creates root/tools/<rel> for each rel given, and nothing else.
func plantOnly(t *testing.T, root string, rels ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "tools"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, rel := range rels {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte("#!/bin/sh\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
}

// NEGATIVE. A target repo that carries no decision helper, dispatched with no --claim-root and
// the claim taken through the PATH binary (the shape that reached the field), is REFUSED before
// any child runs, and the refusal names both places looked: the file under --root, and PATH.
func TestDecisionHelperAbsentRefuses(t *testing.T) {
	s := &stub{}
	_, root := s.install(t) // no scripts in the target repo
	withGoClaimOnPath(t)    // the claim resolves; only the decision helper is missing
	s.replies = happyReplies(filepath.Join(t.TempDir(), "worker-home"))

	err := cmdDispatch([]string{"item-1", "--root", root, "--repo", allowedRepo,
		"--gate-human", "--brief", "spec.md"})
	if err == nil {
		t.Fatal("a human-gated dispatch with no decision helper anywhere returned nil — the gate was skipped")
	}
	if got := deskkit.ExitCodeOf(err); got != deskkit.ExitUnverifiable {
		t.Fatalf("rc = %d, want %d (fail closed)", got, deskkit.ExitUnverifiable)
	}
	msg := err.Error()
	for _, want := range []string{
		filepath.Join(root, filepath.FromSlash(decisionScriptRel)), // place 1: the file under --root, absolute
		"no --claim-root was given",                                // which flag chose that root
		"PATH",                                                     // place 2: PATH, and that it holds no port
		"point --claim-root at the checkout that does",             // the way out
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("the refusal does not name %q:\n%s", want, msg)
		}
	}
	if len(s.calls) != 0 {
		t.Fatalf("the refusal came after %d child process(es): %v", len(s.calls), s.calls)
	}
}

// An explicit --claim-root is AUTHORITATIVE: one that lacks the helper refuses even when --root
// carries it, and says --root was not consulted — a silent fall-back would turn a mispointed
// flag into a dispatch that only looked configured.
func TestDecisionClaimRootStrict(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantOnly(t, root, decisionScriptRel) // the target repo HAS it ...
	trk := t.TempDir()                    // ... the authoritative --claim-root does not
	withGoClaimOnPath(t)
	s.replies = happyReplies(filepath.Join(t.TempDir(), "worker-home"))

	err := cmdDispatch([]string{"item-1", "--root", root, "--claim-root", trk, "--repo", allowedRepo,
		"--gate-human", "--brief", "spec.md"})
	if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("an authoritative --claim-root without the helper must fail closed (exit 6): %v", err)
	}
	absTrk, _ := filepath.Abs(trk)
	for _, want := range []string{filepath.Join(absTrk, filepath.FromSlash(decisionScriptRel)),
		"--root is not consulted", "PATH"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q:\n%s", want, err.Error())
		}
	}
	if len(s.calls) != 0 {
		t.Fatalf("the refusal came after %d child process(es): %v", len(s.calls), s.calls)
	}
}

// POSITIVE. The helper is found through --claim-root while --root lacks it, and the ensure call
// runs THAT file.
func TestDecisionHelperViaClaimRoot(t *testing.T) {
	s := &stub{}
	_, root := s.install(t) // the target repo carries neither script
	trk := t.TempDir()
	plantOnly(t, trk, claimScriptRel, decisionScriptRel)
	s.replies = append(happyReplies(filepath.Join(t.TempDir(), "worker-home")),
		reply{match: "decision-issue.sh ensure", stdout: "created: decision issue #4"})

	if err := cmdDispatch([]string{"item-1", "--root", root, "--claim-root", trk, "--repo", allowedRepo,
		"--gate-human", "--brief", "spec.md", "--quiet",
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")}); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	absTrk, _ := filepath.Abs(trk)
	helper := filepath.Join(absTrk, filepath.FromSlash(decisionScriptRel))
	if !s.ran(helper + " ensure spec.md") {
		t.Errorf("the decision gate did not run the --claim-root helper %s; calls: %v", helper, s.calls)
	}
}

// CLASS GUARD — the defect class is "a wrapped consumer helper whose missing-helper refusal
// names one directory, not every location tried and the --claim-root way out". Two halves:
//
//  1. structural: every `tools/*.sh` string constant declared in this package's non-test source
//     is a wrapped consumer helper and must be on consumerHelpers (the list the refusal phrasing
//     reads), and every entry must have a behavioural case below. A new helper added without
//     either fails here, naming it.
//  2. behavioural: for EVERY entry, a dispatch where that helper alone is missing refuses
//     (exit 6) and names the absolute path tried, PATH, and --claim-root.
func TestHelperRefusalsNameAll(t *testing.T) {
	// How to make each helper REQUIRED in a dispatch. A helper with no case fails the guard.
	required := map[string][]string{
		claimScriptRel:    nil,                                    // the claim is always taken
		decisionScriptRel: {"--gate-human", "--brief", "spec.md"}, // only a human-gated item needs it
	}

	declared := declaredHelperConsts(t)
	// Positive control: the matcher must see the two helpers known to exist, or a broken
	// matcher would report "nothing undeclared" and pass.
	for _, known := range []string{claimScriptRel, decisionScriptRel} {
		if !declared[known] {
			t.Fatalf("positive control: the const scan did not find %q — the guard's matcher is broken", known)
		}
	}
	listed := map[string]bool{}
	for _, h := range consumerHelpers {
		listed[h.rel] = true
	}
	var unlisted []string
	for rel := range declared {
		if !listed[rel] {
			unlisted = append(unlisted, rel)
		}
	}
	sort.Strings(unlisted)
	for _, rel := range unlisted {
		t.Errorf("%s is a wrapped helper constant but is not on consumerHelpers — its missing-helper "+
			"refusal would not be phrased through helperLocationsTried", rel)
	}

	for _, h := range consumerHelpers {
		args, ok := required[h.rel]
		if !ok {
			t.Errorf("consumerHelpers entry %s has no behavioural case in this guard", h.rel)
			continue
		}
		t.Run(filepath.Base(h.rel), func(t *testing.T) {
			s := &stub{}
			_, root := s.install(t) // no PATH ports resolve
			var others []string
			for _, o := range consumerHelpers {
				if o.rel != h.rel {
					others = append(others, o.rel)
				}
			}
			plantOnly(t, root, others...)
			s.replies = happyReplies(filepath.Join(t.TempDir(), "worker-home"))

			err := cmdDispatch(append([]string{"item-1", "--root", root, "--repo", allowedRepo}, args...))
			if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
				t.Fatalf("missing %s must fail closed (exit 6): %v", h.rel, err)
			}
			for _, want := range []string{filepath.Join(root, filepath.FromSlash(h.rel)), "PATH", "--claim-root"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the refusal for a missing %s does not name %q:\n%s", h.rel, want, err.Error())
				}
			}
			if len(s.calls) != 0 {
				t.Errorf("the refusal came after %d child process(es): %v", len(s.calls), s.calls)
			}
		})
	}
}

// helperConstRe is the shape of a wrapped consumer helper's repo-relative path.
var helperConstRe = regexp.MustCompile(`^tools/[A-Za-z0-9._-]+\.sh$`)

// declaredHelperConsts returns every string constant in this package's non-test .go files whose
// value is a `tools/*.sh` path.
func declaredHelperConsts(t *testing.T) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			gd, ok := n.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				return true
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, v := range vs.Values {
					lit, ok := v.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					if s, err := strconv.Unquote(lit.Value); err == nil && helperConstRe.MatchString(s) {
						out[s] = true
					}
				}
			}
			return true
		})
	}
	return out
}
