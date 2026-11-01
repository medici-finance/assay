package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestLibraryStaleFailRange(t *testing.T) {
	root, tip := staleHistoryFixture(t)
	base := strings.TrimSpace(string(parityGitBytes(t, root, "rev-parse", tip+"^1")))
	end := beginGitReadSession()
	defer end()
	calls := 0
	native := func(args ...string) ([]byte, error) {
		calls++
		return exec.Command("git", append([]string{"-C", root}, args...)...).Output()
	}
	got, err := staleFailRangeMessages(ordinaryGitCore(root), base, tip[:7], tip, false, native)
	if err != nil || calls != 0 {
		t.Fatalf("library range used native or failed: calls=%d %v", calls, err)
	}
	want := parityGitBytes(t, root, "log", base+".."+tip[:7], "--format=%B")
	for _, pattern := range []string{`#72\b`, `#73\b`, `landing`, `initial`, `fix`, `side`} {
		rx := regexp.MustCompile(pattern)
		if rx.Match(got) != rx.Match(want) {
			t.Fatalf("query %q changed: %q want %q", pattern, got, want)
		}
	}
	_, err = staleFailRangeMessages(ordinaryGitCore(root), base, tip[:7], tip, true, native)
	if err != nil || calls != 1 {
		t.Fatalf("formatting observation lost: calls=%d %v", calls, err)
	}
	runGit(t, root, "branch", tip[:7], base)
	got, err = staleFailRangeMessages(ordinaryGitCore(root), base, tip[:7], tip, false, native)
	want = parityGitBytes(t, root, "log", base+".."+tip[:7], "--format=%B")
	if err != nil || calls != 2 || !bytes.Equal(got, want) {
		t.Fatalf("shadowing name did not retain native: calls=%d got=%q want=%q err=%v", calls, got, want, err)
	}
}

func TestLibraryStaleFailRangeEncodingAndGap(t *testing.T) {
	root, base := staleHistoryFixture(t)
	tree := strings.TrimSpace(string(parityGitBytes(t, root, "rev-parse", "HEAD^{tree}")))
	raw := []byte(fmt.Sprintf("tree %s\nparent %s\nauthor Fixture <fixture@example.invalid> 1 +0000\ncommitter Fixture <fixture@example.invalid> 1 +0000\nencoding ISO-8859-1\n\nfix #72 caf", tree, base))
	raw = append(raw, 0xe9, '\n')
	cmd := exec.Command("git", "-C", root, "hash-object", "-t", "commit", "-w", "--stdin")
	cmd.Stdin = bytes.NewReader(raw)
	encoded, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	tip := strings.TrimSpace(string(encoded))
	end := beginGitReadSession()
	defer end()
	calls := 0
	native := func(args ...string) ([]byte, error) {
		calls++
		return exec.Command("git", append([]string{"-C", root}, args...)...).Output()
	}
	got, err := staleFailRangeMessages(ordinaryGitCore(root), base, tip[:7], tip, false, native)
	want := parityGitBytes(t, root, "log", base+".."+tip[:7], "--format=%B")
	if err != nil || calls != 1 || !bytes.Equal(got, want) {
		t.Fatalf("encoding rendering changed: calls=%d %q want %q err=%v", calls, got, want, err)
	}
	_, err = staleFailRangeMessages(ordinaryGitCore(root), base, tip[:7], tip, true, native)
	if err != nil || calls != 2 {
		t.Fatalf("native formatting profile lost: calls=%d %v", calls, err)
	}
	// A fresh reader sees the missing excluded endpoint and never retries native.
	activeGitReads = nil
	end2 := beginGitReadSession()
	defer end2()
	if err := os.Remove(filepath.Join(root, ".git", "objects", base[:2], base[2:])); err != nil {
		t.Fatal(err)
	}
	before := calls
	if _, err := staleFailRangeMessages(ordinaryGitCore(root), base, tip[:7], tip, false, native); err == nil || calls != before {
		t.Fatalf("graph failure retried native or became success: calls=%d err=%v", calls, err)
	}
}

func TestStaleFailHistorySymbolicReplace(t *testing.T) {
	root, landing := staleHistoryFixture(t)
	side := strings.TrimSpace(string(parityGitBytes(t, root, "rev-parse", landing+"^2")))
	tree := strings.TrimSpace(string(parityGitBytes(t, root, "rev-parse", side+"^{tree}")))
	parent := strings.TrimSpace(string(parityGitBytes(t, root, "rev-parse", side+"^")))
	replacement := strings.TrimSpace(string(parityGitBytes(t, root, "commit-tree", tree, "-p", parent, "-m", "no issue reference")))
	runGit(t, root, "update-ref", "refs/heads/replacement-candidate", replacement)
	runGit(t, root, "symbolic-ref", "refs/replace/"+side, "refs/heads/replacement-candidate")
	previous := activeGitReads
	activeGitReads = nil
	want, wok := newestFailFix(root, &BriefFile{Issues: []int{72}}, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	activeGitReads = previous
	if !wok || want != "" {
		t.Fatalf("native symbolic replacement fixture failed: %q %v", want, wok)
	}
	end := beginGitReadSession()
	defer end()
	got, gok := newestFailFix(root, &BriefFile{Issues: []int{72}}, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	if got != want || gok != wok {
		t.Fatalf("symbolic replacement changed main-history verdict: got=%q/%v want=%q/%v", got, gok, want, wok)
	}
}
