package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitcore"
)

func TestPreviewStampsExistingFileLocations(t *testing.T) {
	root := t.TempDir()
	initRepoAt(t, root, fixedDate)
	const brief = "docs/streams/widget/brief-01-widget.md"
	var lines []string
	for i := 1; i <= 20; i++ {
		lines = append(lines, fmt.Sprintf("Evidence item %d pending.", i))
	}
	writeFile(t, root, brief, strings.Join(lines, "\n")+"\n")
	runGit(t, root, "add", "--", brief)
	runGit(t, root, "commit", "-qm", "record pending evidence")
	runGit(t, root, "update-ref", remoteMainRef, "HEAD")

	// Two separate hunks each start with unchanged context before the stamp.
	lines[4] = "Approved by human:alex"
	lines[15] = "Verified by human:alex"
	writeFile(t, root, brief, strings.Join(lines, "\n")+"\n")
	runGit(t, root, "commit", "-qm", "record two sign-offs", "--", brief)

	var out bytes.Buffer
	if code := runPreviewStamps([]string{"--root", root}, &out, &out); code != 1 {
		t.Fatalf("exit %d, want 1: %s", code, &out)
	}
	var locations []string
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(line, brief+":") {
			locations = append(locations, line)
		}
	}
	want := brief + ":5: human:alex needs corroboration\n" + brief + ":16: human:alex needs corroboration"
	if got := strings.Join(locations, "\n"); got != want {
		t.Fatalf("destination lines:\n%s\nwant:\n%s", got, want)
	}
}

func TestPreviewStampsLocalDiff(t *testing.T) {
	root := t.TempDir()
	initRepoAt(t, root, fixedDate)
	const board = "docs/streams/widget/README.md"
	const row = "| 01 | [Widget](brief-01-widget.md) | 0 | S | done | verified | human:alex |\n"
	const dr = "docs/streams/decisions/DR-test.md"
	writeFile(t, root, board, row)
	writeFile(t, root, dr, "---\nid: DR-test\ndecided-by: human:alex\n---\nold\n")
	runGit(t, root, "add", "--", board, dr)
	runGit(t, root, "commit", "-qm", "original records")
	runGit(t, root, "update-ref", remoteMainRef, "HEAD")
	const req = "docs/streams/requirements/new request.md"
	writeFile(t, root, req, "---\nasked-by: human:alex\nstatus: accepted\n---\n\nApproved by human:alex\n")
	writeFile(t, root, board, strings.Replace(row, "| S |", "| M |", 1))
	writeFile(t, root, dr, "---\nid: DR-test\ndecided-by: human:alex\n---\nnew\n")
	writeFile(t, root, "example_test.go", "// human:alex\n")
	const fixture = "docs/streams/widget/fixtures"
	writeFile(t, root, fixture+"/.statusgen-fixtures", "")
	writeFile(t, root, fixture+"/demo.md", "human:alex\n")
	runGit(t, root, "add", "--", req, dr, board, "example_test.go", fixture)
	runGit(t, root, "commit", "-qm", "record claims")
	// Neither local edits nor changed declaration files describe the pushed head.
	writeFile(t, root, req, "---\nasked-by: Project driver\nstatus: proposed\n---\n")
	runGit(t, root, "add", "--", req)
	writeFile(t, root, dr, "---\nid: DR-test\ndecided-by: human:other\n---\nnew\n")
	writeFile(t, root, "untracked.md", "human:alex\n")
	writeFile(t, root, "docs/streams/requirements/.statusgen-fixtures", "")
	if err := os.Remove(filepath.Join(root, fixture, ".statusgen-fixtures")); err != nil {
		t.Fatal(err)
	}

	// A native Git launch or a forge CLI read would hit the sentinel.
	bin := t.TempDir()
	marker := filepath.Join(bin, "called")
	t.Setenv("PREVIEW_TEST_MARKER", marker)
	for _, name := range []string{"git", "gh", "glab", "deskread"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\necho called > \"$PREVIEW_TEST_MARKER\"\nexit 1\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	originalPath := os.Getenv("PATH")
	previewPath := bin + string(os.PathListSeparator) + originalPath
	t.Setenv("PATH", previewPath)
	var out bytes.Buffer
	check := func(want int, args ...string) string {
		t.Helper()
		out.Reset()
		if got := runPreviewStamps(append([]string{"--root", root}, args...), &out, &out); got != want {
			t.Fatalf("code %d, want %d: %s", got, want, &out)
		}
		return out.String()
	}
	report := check(1)
	for _, want := range []string{"Base " + remoteMainRef, "to committed HEAD", req + ":2: human:alex", req + ":6: human:alex", dr + " (decided-by): human:alex", "2 claim(s)"} {
		if !strings.Contains(report, want) {
			t.Errorf("missing %q in %s", want, report)
		}
	}
	for _, absent := range []string{board, "example_test.go", "untracked.md", fixture, "human:other"} {
		if strings.Contains(report, absent) {
			t.Errorf("unexpected %q in %s", absent, report)
		}
	}
	t.Setenv("PATH", originalPath)
	writeFile(t, root, dr, "---\nid: DR-test\ndecided-by: human:alex\n---\nold\n")
	runGit(t, root, "commit", "-qm", "attribute proposed ask", "--", req, dr)
	t.Setenv("PATH", previewPath)
	check(0)
	check(2, "--base-ref", "refs/heads/missing")
	// A promisor checkout can lack an object which the changed tree needs.
	// It must report an incomplete preview instead of consulting a remote.
	t.Setenv("PATH", originalPath)
	runGit(t, root, "config", "remote.origin.promisor", "true")
	runGit(t, root, "config", "remote.origin.url", "https://example.invalid/repository")
	repo, err := gitcore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	blob, err := repo.TreeishID("HEAD", req)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, ".git", "objects", blob[:2], blob[2:])); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", previewPath)
	check(2)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("preview unexpectedly launched a Git or forge command: %v", err)
	}
}
