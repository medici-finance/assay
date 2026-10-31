package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestVerifiedOutcomeGateBeforeWrite(t *testing.T) {
	for _, dry := range []bool{false, true} {
		t.Run(map[bool]string{false: "write", true: "dry-run"}[dry], func(t *testing.T) {
			f, _ := setupFake(t)
			outcomeGuardFn = func(string, string, string, []byte, []byte, deskkit.Forge, deskkit.ForgeRepo, string, childOptions) error {
				return deskkit.Refused("closure still implemented")
			}
			file := "docs/streams/verify-outcomes.jsonl"
			root := rootWithFile(t, file, "{\"brief\":\"test/01\",\"outcome\":\"verified\"}\n")
			args := []string{"example-org/tracker", "main", "--root", root, "--evidence-file", file}
			if dry {
				args = append(args, "--dry-run")
			}
			code := run(args)
			if code != 5 || f.putCalls != 0 {
				t.Fatalf("exit=%d writes=%d; want refused before write", code, f.putCalls)
			}
		})
	}
}

// Run with a freshly built statusgen on PATH, exercising the real JSON and exit
// contract rather than accepting a stub shaped to this caller's assumptions.
func TestVerifiedOutcomeRealStatusgen(t *testing.T) {
	setupFake(t)
	landedStatusgenOnPath(t)
	root, _, head, files := landedFixture(t, false)
	flGit(t, root, "checkout", "-q", "-f", "--detach", head)
	file, err := checkVerifiedClosure(root, "vf/01", childOptions{})
	if err != nil || file != "docs/streams/vf/brief-01-check.md" {
		t.Fatalf("file=%q error=%v", file, err)
	}
	board := "docs/streams/vf/README.md"
	if err := os.WriteFile(filepath.Join(root, board), []byte(strings.Replace(files[board], "| verified |", "| implemented |", 1)), 0644); err != nil {
		t.Fatal(err)
	}
	_, err = checkVerifiedClosure(root, "vf/01", childOptions{})
	var de *deskkit.DeskError
	if !asDeskError(err, &de) || de.Code != deskkit.ExitRefused {
		t.Fatalf("expected refused closure, got %v", err)
	}
}

func TestVerifiedOutcomeClosureGuard(t *testing.T) {
	const success = "{\"brief\":\"test/01\",\"outcome\":\"verified\"}\n"
	const failure = "{\"brief\":\"test/01\",\"outcome\":\"verify-fail\"}\n"
	for _, tc := range []struct {
		name, before, after                 string
		closureErr, drift, lintRed, wantErr bool
		// kind is what the branch holds the closure brief as (S-2266-8); "" leaves it a
		// plain file.
		kind string
	}{
		{name: "valid", after: success},
		{name: "unclosed", after: success, closureErr: true, wantErr: true},
		{name: "local-only", after: success, drift: true, wantErr: true},
		{name: "lint-refused", after: success, lintRed: true, wantErr: true},
		{name: "failure-after-history", before: success, after: success + failure, closureErr: true},
		{name: "duplicate-new-success", before: success, after: success + success, closureErr: true, wantErr: true},
		// regression: S-2266-8 — a link served as its target's bytes matches the local
		// closure byte for byte, but is not the brief the branch holds.
		{name: "closure-served-through-a-link", after: success, kind: deskkit.FileKindUnproven, wantErr: true},
		{name: "closure-is-a-link", after: success, kind: deskkit.FileKindSymlink, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, _ := setupFake(t)
			root := t.TempDir()
			brief := "docs/streams/test/brief-01-test.md"
			for _, file := range []string{brief, "docs/streams/test/README.md"} {
				if err := os.MkdirAll(filepath.Dir(filepath.Join(root, file)), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, file), []byte("closed\n"), 0644); err != nil {
					t.Fatal(err)
				}
				f.setFile(file, "closed\n")
			}
			if tc.drift {
				f.setFile("docs/streams/test/README.md", "implemented\n")
			}
			if tc.kind != "" {
				f.fileKinds = map[string]string{brief: tc.kind}
			}
			oldCheck, oldLint := closureCheckFn, statusgenLintFn
			t.Cleanup(func() { closureCheckFn = oldCheck; statusgenLintFn = oldLint })
			closureCheckFn = func(string, string, childOptions) (string, error) {
				if tc.closureErr {
					return "", deskkit.Refused("not verified")
				}
				return brief, nil
			}
			statusgenLintFn = func(string, childOptions) ([]string, error) {
				if tc.lintRed {
					return []string{"PROBLEM: bad closure"}, nil
				}
				return nil, nil
			}
			err := guardVerifiedOutcomes(root, root, "docs/streams/verify-outcomes-2026-09.jsonl", []byte(tc.before), []byte(tc.after), f, deskkit.ForgeRepo{}, "main", childOptions{})
			if (err != nil) != tc.wantErr {
				t.Fatalf("error=%v wantErr=%v", err, tc.wantErr)
			}
			if tc.kind != "" && !deskkit.IsUnverifiable(err) {
				t.Fatalf("error=%v, want could-not-check for a closure the branch holds as %q", err, tc.kind)
			}
		})
	}
}

// regression: #3343 — older statusgen must not silently degrade publication.
func TestVerifiedOutcomeAttributionCapability(t *testing.T) {
	fakeStatusgenOnPath(t, `for arg do
if [ "$arg" = "--check-attribution" ]; then
    echo 'brief: unknown flag --check-attribution' >&2
    exit 2
fi
done
echo '{"key":"vf/01","file":"docs/streams/vf/brief-01-check.md"}'`)
	_, err := checkVerifiedClosure(t.TempDir(), "vf/01", childOptions{})
	var de *deskkit.DeskError
	if !asDeskError(err, &de) || de.Code != deskkit.ExitUnverifiable {
		t.Fatalf("old capability did not fail closed: %v", err)
	}
}
