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
			outcomeGuardFn = func(string, string, []byte, []byte, deskkit.Forge, deskkit.ForgeRepo, string) error {
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
	bin := os.Getenv("ASSAY_TEST_STATUSGEN_DIR")
	if bin == "" {
		t.Skip("set ASSAY_TEST_STATUSGEN_DIR to freshly built statusgen directory")
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "sample")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	board := "---\nstream: sample\nstatus: active\n---\n\n| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n|---|---|---|---|---|---|---|\n| 01 | [check](brief-01-check.md) | 1 | S | verified | 2026-09-22 verifier | — |\n"
	brief := "# Check\n\n## Verify\n| # | Command | Expect |\n|---|---|---|\n| 1 | `true` | exit 0 |\n\n## Evidence\n| # | Command | Result | Output | Date | Runner |\n|---|---|---|---|---|---|\n| 1 | `true` | pass exit=0 | sha256:abcdef | 2026-09-22 | verifier |\n"
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(board), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "brief-01-check.md"), []byte(brief), 0644); err != nil {
		t.Fatal(err)
	}
	file, err := checkVerifiedClosure(root, "sample/01")
	if err != nil || file != "docs/streams/sample/brief-01-check.md" {
		t.Fatalf("file=%q error=%v", file, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(strings.Replace(board, "| verified |", "| implemented |", 1)), 0644); err != nil {
		t.Fatal(err)
	}
	_, err = checkVerifiedClosure(root, "sample/01")
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
	}{
		{name: "valid", after: success},
		{name: "unclosed", after: success, closureErr: true, wantErr: true},
		{name: "local-only", after: success, drift: true, wantErr: true},
		{name: "lint-refused", after: success, lintRed: true, wantErr: true},
		{name: "failure-after-history", before: success, after: success + failure, closureErr: true},
		{name: "duplicate-new-success", before: success, after: success + success, closureErr: true, wantErr: true},
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
			oldCheck, oldLint := closureCheckFn, statusgenLintFn
			t.Cleanup(func() { closureCheckFn = oldCheck; statusgenLintFn = oldLint })
			closureCheckFn = func(string, string) (string, error) {
				if tc.closureErr {
					return "", deskkit.Refused("not verified")
				}
				return brief, nil
			}
			statusgenLintFn = func(string) ([]string, error) {
				if tc.lintRed {
					return []string{"PROBLEM: bad closure"}, nil
				}
				return nil, nil
			}
			err := guardVerifiedOutcomes(root, "docs/streams/verify-outcomes-2026-09.jsonl", []byte(tc.before), []byte(tc.after), f, deskkit.ForgeRepo{}, "main")
			if (err != nil) != tc.wantErr {
				t.Fatalf("error=%v wantErr=%v", err, tc.wantErr)
			}
		})
	}
}
