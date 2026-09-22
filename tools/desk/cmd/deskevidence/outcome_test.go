package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestVerifiedOutcomeGateBeforeWrite(t *testing.T) {
	f, _ := setupFake(t)
	outcomeGuardFn = func(string, string, []byte, []byte, deskkit.Forge, deskkit.ForgeRepo, string) error {
		return deskkit.Refused("closure still implemented")
	}
	file := "docs/streams/verify-outcomes.jsonl"
	root := rootWithFile(t, file, "{\"brief\":\"test/01\",\"outcome\":\"verified\"}\n")
	code := run([]string{"example-org/tracker", "main", "--root", root, "--evidence-file", file})
	if code != 5 || f.putCalls != 0 {
		t.Fatalf("exit=%d writes=%d; want refused before write", code, f.putCalls)
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
