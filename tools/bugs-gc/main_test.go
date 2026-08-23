package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollect_ClosedPruned_OpenKept(t *testing.T) {
	orig := issueLister
	defer func() { issueLister = orig }()
	issueLister = func(number string) string {
		switch number {
		case "1":
			return "CLOSED"
		case "2":
			return "OPEN"
		case "3":
			return "UNKNOWN"
		default:
			return "UNKNOWN"
		}
	}

	dir := t.TempDir()
	bugsDir := filepath.Join(dir, "bugs")
	if err := os.MkdirAll(bugsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(bugsDir, "1.md"), "closed issue")
	writeFile(t, filepath.Join(bugsDir, "2.md"), "open issue")
	writeFile(t, filepath.Join(bugsDir, "3.md"), "unknown issue")
	writeFile(t, filepath.Join(bugsDir, "README.md"), "convention doc")

	prune, keep, err := collect(bugsDir)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(prune) != 1 || prune[0] != "1.md" {
		t.Errorf("prune: want [1.md], got %v", prune)
	}
	if len(keep) != 1 || keep[0] != "2.md" {
		t.Errorf("keep: want [2.md], got %v", keep)
	}
}

func TestCollect_EmptyDir(t *testing.T) {
	orig := issueLister
	defer func() { issueLister = orig }()
	issueLister = func(string) string { return "OPEN" }
	dir := t.TempDir()
	bugsDir := filepath.Join(dir, "bugs")
	if err := os.MkdirAll(bugsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	prune, keep, err := collect(bugsDir)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(prune) != 0 || len(keep) != 0 {
		t.Errorf("empty dir: want 0/0, got prune=%v keep=%v", prune, keep)
	}
}

func TestCollect_OnlyReadme(t *testing.T) {
	orig := issueLister
	defer func() { issueLister = orig }()
	issueLister = func(string) string { return "OPEN" }
	dir := t.TempDir()
	bugsDir := filepath.Join(dir, "bugs")
	if err := os.MkdirAll(bugsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(bugsDir, "README.md"), "convention")
	prune, keep, err := collect(bugsDir)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(prune) != 0 || len(keep) != 0 {
		t.Errorf("only README: want 0/0, got prune=%v keep=%v", prune, keep)
	}
}

func TestRun_DryRun_DeletesNothing(t *testing.T) {
	orig := issueLister
	defer func() { issueLister = orig }()
	issueLister = func(number string) string { return "CLOSED" }
	dir := t.TempDir()
	bugsDir := filepath.Join(dir, "bugs")
	if err := os.MkdirAll(bugsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(bugsDir, "999.md"), "should be pruned")
	writeFile(t, filepath.Join(bugsDir, "README.md"), "convention")
	code := run(dir, true)
	if code != 0 {
		t.Errorf("dry-run exit code: want 0, got %d", code)
	}
	if _, err := os.Stat(filepath.Join(bugsDir, "999.md")); os.IsNotExist(err) {
		t.Error("dry-run deleted 999.md -- should have kept it")
	}
}

func TestRun_Default_DeletesClosed(t *testing.T) {
	orig := issueLister
	defer func() { issueLister = orig }()
	issueLister = func(number string) string { return "CLOSED" }
	dir := t.TempDir()
	bugsDir := filepath.Join(dir, "bugs")
	if err := os.MkdirAll(bugsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(bugsDir, "42.md"), "closed")
	writeFile(t, filepath.Join(bugsDir, "README.md"), "convention")
	code := run(dir, false)
	if code != 0 {
		t.Errorf("run exit code: want 0, got %d", code)
	}
	if _, err := os.Stat(filepath.Join(bugsDir, "42.md")); !os.IsNotExist(err) {
		t.Error("42.md should have been deleted (issue CLOSED)")
	}
	if _, err := os.Stat(filepath.Join(bugsDir, "README.md")); os.IsNotExist(err) {
		t.Error("README.md should not have been deleted")
	}
}

func TestRun_OpenKept(t *testing.T) {
	orig := issueLister
	defer func() { issueLister = orig }()
	issueLister = func(number string) string { return "OPEN" }
	dir := t.TempDir()
	bugsDir := filepath.Join(dir, "bugs")
	if err := os.MkdirAll(bugsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(bugsDir, "7.md"), "still open")
	code := run(dir, false)
	if code != 0 {
		t.Errorf("run exit code: want 0, got %d", code)
	}
	if _, err := os.Stat(filepath.Join(bugsDir, "7.md")); os.IsNotExist(err) {
		t.Error("7.md should have been kept (issue OPEN)")
	}
}

func TestRun_IgnoresNonMd(t *testing.T) {
	orig := issueLister
	defer func() { issueLister = orig }()
	issueLister = func(string) string { return "CLOSED" }
	dir := t.TempDir()
	bugsDir := filepath.Join(dir, "bugs")
	if err := os.MkdirAll(bugsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(bugsDir, "notes.txt"), "not a carrier")
	writeFile(t, filepath.Join(bugsDir, "README.md"), "convention")
	prune, keep, err := collect(bugsDir)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(prune) != 0 || len(keep) != 0 {
		t.Errorf("non-md file: want 0/0, got prune=%v keep=%v", prune, keep)
	}
}

func TestRun_SubdirsIgnored(t *testing.T) {
	orig := issueLister
	defer func() { issueLister = orig }()
	issueLister = func(string) string { return "CLOSED" }
	dir := t.TempDir()
	bugsDir := filepath.Join(dir, "bugs")
	subDir := filepath.Join(bugsDir, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(subDir, "5.md"), "in subdir")
	writeFile(t, filepath.Join(bugsDir, "README.md"), "convention")
	prune, keep, err := collect(bugsDir)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(prune) != 0 || len(keep) != 0 {
		t.Errorf("subdir: want 0/0, got prune=%v keep=%v", prune, keep)
	}
}

// TestCollect_DirectoryNamedLikeCarrier_Ignored closes a coverage gap:
// TestRun_SubdirsIgnored's subdirectory is named "sub",
// which fails the ".md" suffix check regardless of e.IsDir(), so it never
// actually exercised the e.IsDir() guard in collect(). A directory entry
// whose NAME matches the carrier pattern (e.g. "5.md") is the only input
// that does: mutating away the e.IsDir() guard makes this test — and only
// this test — fail, because with the guard removed the entry falls through
// to resolveState("5") and gets classified as prune or keep like any other
// carrier.
func TestCollect_DirectoryNamedLikeCarrier_Ignored(t *testing.T) {
	orig := issueLister
	defer func() { issueLister = orig }()
	called := false
	issueLister = func(number string) string {
		called = true
		return "CLOSED"
	}
	dir := t.TempDir()
	bugsDir := filepath.Join(dir, "bugs")
	dirLikeCarrier := filepath.Join(bugsDir, "5.md")
	if err := os.MkdirAll(dirLikeCarrier, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(bugsDir, "README.md"), "convention")
	prune, keep, err := collect(bugsDir)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(prune) != 0 || len(keep) != 0 {
		t.Errorf("directory named 5.md: want 0/0, got prune=%v keep=%v", prune, keep)
	}
	if called {
		t.Error("collect should skip a directory entry before ever asking resolveState about it")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCollect_MissingDir(t *testing.T) {
	prune, keep, err := collect("/nonexistent/path/bugs")
	if err == nil {
		t.Error("expected error for missing dir")
	}
	if prune != nil || keep != nil {
		t.Errorf("missing dir: want nil,nil, got prune=%v keep=%v", prune, keep)
	}
}

func TestCollect_OnlyMdFilesCollected(t *testing.T) {
	orig := issueLister
	defer func() { issueLister = orig }()
	issueLister = func(number string) string {
		if number == "10" {
			return "CLOSED"
		}
		return "OPEN"
	}
	dir := t.TempDir()
	bugsDir := filepath.Join(dir, "bugs")
	if err := os.MkdirAll(bugsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(bugsDir, "10.md"), "a")
	writeFile(t, filepath.Join(bugsDir, "10.md.bak"), "not md")
	writeFile(t, filepath.Join(bugsDir, "README.md"), "convention")
	prune, keep, err := collect(bugsDir)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(prune) != 1 || prune[0] != "10.md" {
		t.Errorf("prune: want [10.md], got %v", prune)
	}
	if len(keep) != 0 {
		t.Errorf("keep: want [], got %v", keep)
	}
}

func TestResolveState_MockInjected(t *testing.T) {
	orig := issueLister
	defer func() { issueLister = orig }()
	issueLister = func(number string) string {
		if number == "1" {
			return "CLOSED"
		}
		return "OPEN"
	}
	if s := resolveState("1"); s != "CLOSED" {
		t.Errorf("resolveState 1: want CLOSED, got %s", s)
	}
	if s := resolveState("2"); s != "OPEN" {
		t.Errorf("resolveState 2: want OPEN, got %s", s)
	}
}

func TestResolveState_ExecCommandGhFailure(t *testing.T) {
	// When issueLister is nil, resolveState shells out to `gh`. This test
	// puts a failing stub on PATH to exercise the exec.Command error branch,
	// which the mock-injected tests never reach.
	orig := issueLister
	issueLister = nil // force exec.Command path
	defer func() { issueLister = orig }()

	dir := t.TempDir()
	ghPath := filepath.Join(dir, "gh")
	if err := os.WriteFile(ghPath, []byte("#!/bin/sh\necho 'gh: command not found' >&2\nexit 127\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(filepath.ListSeparator)+os.Getenv("PATH"))

	state := resolveState("42")
	if state != "UNKNOWN" {
		t.Errorf("resolveState with failing gh: want UNKNOWN, got %s", state)
	}
}

func TestParseIssueNumber(t *testing.T) {
	number := strings.TrimSuffix("42.md", ".md")
	if number != "42" {
		t.Errorf("want 42, got %s", number)
	}
	number = strings.TrimSuffix("123.md", ".md")
	if number != "123" {
		t.Errorf("want 123, got %s", number)
	}
}
