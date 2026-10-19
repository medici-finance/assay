package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const detectionMap = "\nPre-mortem → detection map:\n\n| Failure mode of the work | Caught by |\n|---|---|\n| Missing original assertion | row 1 |\n| Command | Expect |\n"

// regression: #3460
func TestRederiveAuxTables(t *testing.T) {
	for _, position := range []string{"before", "after", "both"} {
		t.Run(position, func(t *testing.T) {
			base, head := baseTableMD, headTableMD
			if position != "before" {
				base = strings.Replace(base, "\n## Evidence", detectionMap+"\n## Evidence", 1)
				head = strings.Replace(head, "\n## Evidence", detectionMap+"\n## Evidence", 1)
			}
			if position != "after" {
				base = strings.Replace(base, "## Verify\n", "## Verify\n"+detectionMap+"\n", 1)
				head = strings.Replace(head, "## Verify\n", "## Verify\n"+detectionMap+"\n", 1)
			}
			out, err := fixtureRederive(t, base, head)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, "3 row(s), 1 author-added") || !strings.Contains(out, `command="go test ./foo/..."`) || strings.Contains(out, "Missing original") {
				t.Fatalf("auxiliary table changed provenance: %s", out)
			}
		})
	}
}

// regression: #3460
func TestRederiveSecondTable(t *testing.T) {
	first := "| # | Command | Expect |\n|---|---|---|\n| A | old | old result |\n\n"
	for _, end := range []string{"", "\n\nprose"} {
		for _, second := range []string{"| # | Command | Expect |", "| # | Command | Expect |\n|---|---|---|", "| Command | Other |", "| Other | Expect |"} {
			t.Run(fmt.Sprintf("%q/%q", second, end), func(t *testing.T) {
				rows, err := parseVerifyTable(first + second + end)
				if err == nil || len(rows) != 0 {
					t.Fatalf("partial second Verify table accepted: %+v %v", rows, err)
				}
			})
		}
	}
	// Complete tables merge only when their IDs are unambiguous across the section.
	second := "| # | Command | Expect |\n|---|---|---|\n| B | second | second result |"
	rows, err := parseVerifyTable(first + second)
	if err != nil || len(rows) != 2 || rows[1].Num != "B" {
		t.Fatalf("unique second table: %+v %v", rows, err)
	}
	if _, err := parseVerifyTable(first + strings.Replace(second, "| B |", "| A |", 1)); err == nil {
		t.Fatal("duplicate ID across tables accepted")
	}
}

// regression: #3460
func TestBaseProbeFullTree(t *testing.T) {
	old := runGitFn
	t.Cleanup(func() { runGitFn = old })
	t.Setenv("GIT_LITERAL_PATHSPECS", "1")
	runGitFn = func(root string, args ...string) (string, error) {
		if args[0] == "show" {
			return "", fmt.Errorf("simulated blob read failure")
		}
		if strings.Contains(strings.Join(args, " "), "ls-tree") {
			sub := filepath.Join(root, "nested")
			if err := os.MkdirAll(sub, 0700); err != nil {
				return "", err
			}
			return runGit(sub, args...)
		}
		return runGit(root, args...)
	}
	out, err := fixtureRederive(t, baseTableMD, headTableMD)
	if err == nil || out != "" {
		t.Fatalf("existing base from nested cwd became absent: %q %v", out, err)
	}
}
