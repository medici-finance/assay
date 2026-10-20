package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const detectionMap = "\nPre-mortem → detection map:\n\n| Failure mode of the work | Caught by |\n|---|---|\n| Missing original assertion | row 1 |\n"

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

// regression: #3460
func TestRederiveMovedOriginal(t *testing.T) {
	aux := "\n| Failure mode | Caught by |\n|---|---|\n| # | Command | Expect |\n| 1 | true | pass |\n"
	head := strings.Replace(headTableMD, "| 1 | check | true | pass |\n", "", 1)
	head = strings.Replace(head, "\n## Evidence", aux+"\n## Evidence", 1)
	out, err := fixtureRederive(t, baseTableMD, head)
	if err == nil || out != "" {
		t.Fatalf("edited original row hidden by auxiliary header: %q %v", out, err)
	}
}

// regression: #3460
func TestAuxHeaderAmbiguity(t *testing.T) {
	for _, cell := range []string{"Command", "Expect", " cOmMaNd ", " eXpEcT "} {
		for _, side := range []string{"base", "head"} {
			t.Run(cell+side, func(t *testing.T) {
				bad := strings.Replace(baseTableMD, "\n## Evidence", detectionMap+"| text | "+cell+" |\n\n## Evidence", 1)
				base, head := baseTableMD, headTableMD
				if side == "base" {
					base = bad
				} else {
					head = bad
				}
				out, err := fixtureRederive(t, base, head)
				if err == nil || out != "" {
					t.Fatalf("ambiguous auxiliary content accepted: %q %v", out, err)
				}
			})
		}
	}
}

// regression: #3460
func TestRederiveNestedRoot(t *testing.T) {
	out, err := fixtureRederivePath(t, baseTableMD, headTableMD, "brief.md", true)
	if err == nil || out != "" {
		t.Fatalf("nested root compared different files: %q %v", out, err)
	}
}

// regression: #3460
func TestBaseLiteralPattern(t *testing.T) {
	old := runGitFn
	t.Cleanup(func() { runGitFn = old })
	t.Setenv("GIT_GLOB_PATHSPECS", "1")
	runGitFn = func(root string, args ...string) (string, error) {
		if args[0] == "show" {
			return "", fmt.Errorf("simulated blob read failure")
		}
		return runGit(root, args...)
	}
	out, err := fixtureRederivePath(t, baseTableMD, headTableMD, "brief[1].md", false)
	if err == nil || out != "" {
		t.Fatalf("literal filename became absent: %q %v", out, err)
	}
	if runtime.GOOS == "windows" {
		return
	} // A colon is not a Windows filename character.
	t.Setenv("GIT_GLOB_PATHSPECS", "0")
	out, err = fixtureRederivePath(t, "absent-fixture", headTableMD, ":(exclude)brief.md", false)
	if err != nil || !strings.Contains(out, "3 row(s), 3 author-added") {
		t.Fatalf("literal new filename not proven absent: %q %v", out, err)
	}
}

// Auxiliary column labels carry no Verify semantics and may repeat.
func TestAuxDuplicateLabels(t *testing.T) {
	aux := strings.Replace(detectionMap, "Failure mode of the work | Caught by", "Note | Note", 1)
	md := strings.Replace(baseTableMD, "\n## Evidence", aux+"\n## Evidence", 1)
	rows, err := RederiveTable(md, md)
	if err != nil || len(rows) != 2 {
		t.Fatalf("duplicate auxiliary labels: %v %+v", err, rows)
	}
}
