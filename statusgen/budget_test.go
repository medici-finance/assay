package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseBudgetSpec(t *testing.T) {
	tests := []struct {
		name        string
		spec        string
		wantPath    string
		wantMax     int
		wantErrText string
	}{
		{
			name:     "valid spec",
			spec:     "CLAUDE.md:2850",
			wantPath: "CLAUDE.md",
			wantMax:  2850,
		},
		{
			name:     "path with directories",
			spec:     "docs/guides/README.md:5000",
			wantPath: "docs/guides/README.md",
			wantMax:  5000,
		},
		{
			name:        "missing colon",
			spec:        "CLAUDE.md",
			wantErrText: "malformed value",
		},
		{
			name:        "non-integer maxwords",
			spec:        "CLAUDE.md:abc",
			wantErrText: "malformed maxwords",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, max, err := parseBudgetSpec(tt.spec)
			if tt.wantErrText != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrText)
				}
				if !strings.Contains(err.Error(), tt.wantErrText) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErrText, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if path != tt.wantPath {
				t.Errorf("path = %q, want %q", path, tt.wantPath)
			}
			if max != tt.wantMax {
				t.Errorf("max = %d, want %d", max, tt.wantMax)
			}
		})
	}
}

func TestCheckBudget(t *testing.T) {
	tmp := t.TempDir()

	// Set up files for test cases.
	writeFile := func(rel, content string) {
		p := filepath.Join(tmp, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writeFile("under.md", "one two three")            // 3 words, budget 10
	writeFile("exact.md", "one two three four five")  // 5 words, budget 5
	writeFile("over.md", strings.Repeat("word ", 15)) // 15 words, budget 10

	tests := []struct {
		name        string
		specs       []string
		wantProblem string // substring that must appear in at least one problem
		wantNoProb  string // substring that must NOT appear in problems
		wantErr     bool   // whether parseBudgetSpec itself errors
	}{
		{
			name:       "under budget — clean",
			specs:      []string{"under.md:10"},
			wantNoProb: "",
		},
		{
			name:       "at budget exactly — clean",
			specs:      []string{"exact.md:5"},
			wantNoProb: "",
		},
		{
			name:        "over budget — PROBLEM with count and cap",
			specs:       []string{"over.md:10"},
			wantProblem: "over.md: 15 words exceeds budget of 10",
		},
		{
			name:        "budgeted file missing — PROBLEM",
			specs:       []string{"nonexistent.md:100"},
			wantProblem: "budgeted file is missing",
		},
		{
			name:    "flag absent — no check",
			specs:   nil,
			wantErr: false,
		},
		{
			name:    "malformed flag — usage error",
			specs:   []string{"CLAUDE.md"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			problems, err := checkBudget(tmp, tt.specs)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected usage error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantProblem != "" {
				found := false
				for _, p := range problems {
					if strings.Contains(p, tt.wantProblem) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("no problem containing %q in %v", tt.wantProblem, problems)
				}
			}
			if tt.wantNoProb != "" {
				for _, p := range problems {
					if strings.Contains(p, tt.wantNoProb) {
						t.Errorf("unexpected problem %q", p)
					}
				}
			}
			if tt.wantProblem == "" && tt.wantNoProb == "" && len(problems) > 0 {
				t.Errorf("expected no problems, got %v", problems)
			}
		})
	}
}

// TestResolveBudgetSpecs covers the case where bare --lint must default to the
// same budget spec CI enforces (--lint --budget CLAUDE.md:2850,
// .github/workflows/statusgen.yml), while an explicit --budget stays a full
// override — not additive, and not limited to lint mode.
func TestResolveBudgetSpecs(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		explicit []string
		want     []string
	}{
		{
			name:     "bare --lint defaults to the CI budget spec",
			mode:     "lint",
			explicit: nil,
			want:     []string{defaultBudgetSpec},
		},
		{
			name:     "--lint --budget overrides, does not add to, the default",
			mode:     "lint",
			explicit: []string{"docs/other.md:100"},
			want:     []string{"docs/other.md:100"},
		},
		{
			name:     "write mode gets no default budget check",
			mode:     "write",
			explicit: nil,
			want:     nil,
		},
		{
			name:     "check mode gets no default budget check",
			mode:     "check",
			explicit: nil,
			want:     nil,
		},
		{
			name:     "record mode gets no default budget check",
			mode:     "record",
			explicit: nil,
			want:     nil,
		},
		{
			name:     "write mode with explicit --budget still honors it",
			mode:     "write",
			explicit: []string{"CLAUDE.md:2850"},
			want:     []string{"CLAUDE.md:2850"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveBudgetSpecs(tt.mode, tt.explicit)
			if len(got) != len(tt.want) {
				t.Fatalf("resolveBudgetSpecs(%q, %v) = %v, want %v", tt.mode, tt.explicit, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("resolveBudgetSpecs(%q, %v) = %v, want %v", tt.mode, tt.explicit, got, tt.want)
				}
			}
		})
	}
}

// TestLintDefaultBudgetCatchesOverage is an end-to-end demonstration of the
// fix: a repo whose CLAUDE.md is over the CI budget must fail
// `run(root, "lint", resolveBudgetSpecs("lint", nil))` — i.e. bare --lint as
// wired through main() — the same way CI's explicit
// `--lint --budget CLAUDE.md:2850` does, with no --budget flag needed locally.
func TestLintDefaultBudgetCatchesOverage(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	// goodrepo ships no CLAUDE.md; write one 1 word over the 2850 cap.
	over := strings.Repeat("word ", 2851)
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte(over), 0o644); err != nil {
		t.Fatal(err)
	}

	// Before the fix, bare --lint (budget=nil) would not run the check at
	// all — assert that directly to keep the regression visible.
	if problems, err := checkBudget(root, nil); err != nil || len(problems) != 0 {
		t.Fatalf("sanity: nil budget spec must run no check, got problems=%v err=%v", problems, err)
	}

	// What main() now actually wires up for bare `--lint`:
	specs := resolveBudgetSpecs("lint", nil)
	if code := run(root, "lint", specs, nil, ""); code != 1 {
		t.Errorf("bare --lint on an over-budget CLAUDE.md exited %d, want 1", code)
	}
}

// TestBudgetFailureDoesNotSkipLaterPhases pins the fix: a blown
// budget used to short-circuit every later phase, so the run reported `FAIL 1`
// and the link/stream checks never ran. Anyone diffing lint output between two
// trees — the dominant review use — read the drop in count as problems fixed.
// Both problems must now appear, and the count must be their union.
func TestBudgetFailureDoesNotSkipLaterPhases(t *testing.T) {
	root := verdictRepo(t)
	if err := os.WriteFile(filepath.Join(root, "WORDY.md"), []byte(strings.Repeat("word ", 50)), 0o644); err != nil {
		t.Fatal(err)
	}
	// A second, independent problem in a later phase.
	readme := filepath.Join(root, "docs/streams/alpha/README.md")
	b, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(readme, []byte(strings.Replace(string(b), "status: active", "status: bogus", 1)), 0o644); err != nil {
		t.Fatal(err)
	}

	var code int
	stdout := captureStdout(t, func() { code = run(root, "lint", []string{"WORDY.md:10"}, nil, "") })
	if code != 1 {
		t.Fatalf("lint exited %d, want 1", code)
	}
	if got := finalStdoutLine(t, stdout); got != "LINT: FAIL 2 problem(s)" {
		t.Errorf("final stdout line = %q, want %q — a budget failure must not hide the phases after it", got, "LINT: FAIL 2 problem(s)")
	}
}

func TestWordCount(t *testing.T) {
	tests := []struct {
		content string
		want    int
	}{
		{"", 0},
		{"   ", 0},
		{"hello", 1},
		{"hello world", 2},
		{"one\n\ttwo  three", 3},
	}
	for _, tt := range tests {
		got := wordCount(tt.content)
		if got != tt.want {
			t.Errorf("wordCount(%q) = %d, want %d", tt.content, got, tt.want)
		}
	}
}
