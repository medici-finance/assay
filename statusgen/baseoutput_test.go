package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestBaseOutputRetainsPrimaryBytes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("private POSIX-mode output directories")
	}
	root, _, base, _, _, _ := ffRepo(t)
	path := filepath.Join(canonicalPath(t.TempDir()), "primary")
	log, err := newBaseOutputLog(root, path)
	if err != nil {
		t.Fatal(err)
	}
	defer log.close()
	rows := []verifyRow{
		{ID: "1", Command: "printf 'named base failure\\000tail\\n'; printf 'stderr\\n' >&2; exit 1", Expect: "exit 0"},
		{ID: "2", Command: "command-that-does-not-exist", Expect: "exit 0"},
		{ID: "3", Command: "printf skipped", Expect: "exit 0", Class: classCheck, Classed: true},
	}
	got, err := runFailFirstBase(resolveShellPlan(), sandboxUnshare, root, base, rows,
		map[string]bool{"1": true, "2": true, "3": true}, time.Minute, true, log)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte("named base failure\x00tail\nstderr\n")
	record := got["1"]
	if record.LogName == "" || record.OutputDigest == "" {
		t.Fatal("executed base row has no retained primary output metadata")
	}
	raw, err := os.ReadFile(filepath.Join(path, record.LogName))
	if err != nil || string(raw) != string(want) {
		t.Fatalf("raw=%q err=%v", raw, err)
	}
	digest := fmt.Sprintf("sha256:%x", sha256.Sum256(want))
	if record.State != baseRed || record.OutputDigest != digest {
		t.Fatalf("record=%+v", record)
	}
	info, _ := os.Stat(filepath.Join(path, record.LogName))
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%v", info.Mode())
	}
	for _, id := range []string{"2", "3"} {
		if got[id].LogName != "" || got[id].OutputDigest != "" {
			t.Fatalf("no-run metadata: %+v", got[id])
		}
	}
	entries, _ := os.ReadDir(path)
	if len(entries) != 1 {
		t.Fatalf("retained %d files, want only executed row", len(entries))
	}
	cell := failFirstCell(record.Base, record.State, record.Exit, "aaaaaaaaaaaa") +
		" output=" + record.OutputDigest + " log=" + record.LogName
	w := witness{ID: "1", Command: "check", State: statePass, Exit: 0, OutHash: "0123456789ab",
		Date: "2026-11-01", Runner: "fixture", Tree: "aaaaaaaaaaaa", Base: cell}
	parsed, ok := failFirstOf(w.row())
	if !ok || parsed.OutputDigest != digest || parsed.LogName != record.LogName {
		t.Fatalf("metadata lost: %+v ok=%v", parsed, ok)
	}
	if witnessStateOf(w.row()) != statePass {
		t.Fatal("optional Base output changed head witness")
	}
	legacy := w
	legacy.Base = failFirstCell(record.Base, record.State, record.Exit, "aaaaaaaaaaaa")
	if _, ok := failFirstOf(legacy.row()); !ok {
		t.Fatal("legacy Base cell refused")
	}
	for _, bad := range []string{
		strings.Replace(w.Base, "log=row-000001.log", "log=../outside", 1),
		strings.Replace(w.Base, digest, "sha256:short", 1),
		w.Base + " output=" + digest,
		failFirstCell("", baseNotSelected, -1, "aaaaaaaaaaaa") + " output=" + digest + " log=row-000001.log",
	} {
		w.Base = bad
		if _, ok := failFirstOf(w.row()); ok {
			t.Fatalf("malformed output metadata accepted: %s", bad)
		}
	}
}

func TestBaseOutputUnprovenDoesNotEmitMetadata(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("private POSIX-mode logs")
	}
	root, _, base, _, _, _ := ffRepo(t)
	path := filepath.Join(canonicalPath(t.TempDir()), "primary")
	log, err := newBaseOutputLog(root, path)
	if err != nil {
		t.Fatal(err)
	}
	defer log.close()
	rows := []verifyRow{{ID: "1", Command: "printf 'no numbers here\\n'", Expect: "≥ `1`"}}
	got, err := runFailFirstBase(resolveShellPlan(), sandboxUnshare, root, base, rows, map[string]bool{"1": true}, time.Minute, false, log)
	if err != nil {
		t.Fatal(err)
	}
	f := got["1"]
	if f.State != baseUnproven || f.OutputDigest != "" || f.LogName != "" {
		t.Fatalf("unproven record: %+v", f)
	}
	w := witness{ID: "1", Command: "check", State: stateCouldNotRun, Exit: 0, OutHash: "0123456789ab", Date: "2026-11-01", Runner: "fixture", Tree: "aaaaaaaaaaaa", Base: failFirstCell(f.Base, f.State, f.Exit, "aaaaaaaaaaaa")}
	if parsed, ok := failFirstOf(w.row()); !ok || parsed.State != baseUnproven {
		t.Fatalf("unreadable unproven record: %+v ok=%v", parsed, ok)
	}
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) != 0 {
		t.Fatalf("unproven output artifacts: %v err=%v", entries, err)
	}
}

func TestBaseOutputRejectsHomeDirectoryIdentity(t *testing.T) {
	home := canonicalPath(t.TempDir())
	child := filepath.Join(home, "child")
	if err := os.Mkdir(child, 0o700); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(home)
	if err != nil {
		t.Fatal(err)
	}
	if parent, err := openBaseLogParent(child, info); err == nil {
		parent.Close()
		t.Fatal("opened a parent chain through the home inode")
	}
}

func TestBaseOutputRejectsCaseInsensitiveHomeAlias(t *testing.T) {
	parent := canonicalPath(t.TempDir())
	home := filepath.Join(parent, "SourceHome")
	if err := os.Mkdir(home, 0o700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(parent, "sourcehome")
	a, err := os.Stat(alias)
	if os.IsNotExist(err) {
		t.Skip("case-sensitive filesystem")
	}
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.Stat(home)
	if err != nil || !os.SameFile(a, b) {
		t.Fatal("case alias does not name the home")
	}
	if log, err := newBaseOutputLog(home, filepath.Join(alias, "logs")); err == nil {
		log.close()
		t.Fatal("accepted a case alias inside the home")
	}
	if _, err := os.Stat(filepath.Join(home, "logs")); !os.IsNotExist(err) {
		t.Fatal("containment refusal created log directory")
	}
}

func TestBaseOutputRefusesUnsafePaths(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("private POSIX-mode output directories and symlink controls")
	}
	home := canonicalPath(t.TempDir())
	outside := canonicalPath(t.TempDir())
	if err := os.Mkdir(filepath.Join(outside, "existing"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(home, filepath.Join(outside, "link")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(home, "logs"), filepath.Join(outside, "existing"),
		filepath.Join(outside, "link", "logs"), outside + string(filepath.Separator) + ".." + string(filepath.Separator) + "logs", "relative-logs",
	} {
		if l, err := newBaseOutputLog(home, path); err == nil {
			l.close()
			t.Fatalf("accepted unsafe path %q", path)
		}
	}
	if l, err := newBaseOutputLog(filepath.Join(outside, "link"), filepath.Join(home, "alias-logs")); err == nil {
		l.close()
		t.Fatal("symlink spelling of home allowed an in-home log")
	}
	if l, err := newBaseOutputLog(outside, filepath.Join(home, "history-logs"), home); err == nil {
		l.close()
		t.Fatal("log inside separate baseline history home accepted")
	}
	path := filepath.Join(outside, "new")
	l, err := newBaseOutputLog(home, path)
	if err != nil {
		t.Fatal(err)
	}
	defer l.close()
	if err := os.WriteFile(filepath.Join(path, "row-000001.log"), []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := l.record("../untrusted-row-id", []byte("overwrite")); err == nil {
		t.Fatal("overwrote existing file")
	}
	raw, _ := os.ReadFile(filepath.Join(path, "row-000001.log"))
	if string(raw) != "existing" {
		t.Fatal("clobbered existing file")
	}
	if err := l.record("../untrusted-row-id", []byte("new")); err != nil {
		t.Fatal(err)
	}
	if l.rows["../untrusted-row-id"].name != "row-000002.log" {
		t.Fatal("row ID controlled path")
	}
}

func TestBaseOutputWriteFailureStopsRows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("private POSIX-mode output directories")
	}
	root, _, base, _, _, _ := ffRepo(t)
	path := filepath.Join(canonicalPath(t.TempDir()), "primary")
	l, err := newBaseOutputLog(root, path)
	if err != nil {
		t.Fatal(err)
	}
	defer l.close()
	if err := os.WriteFile(filepath.Join(path, "row-000001.log"), []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(canonicalPath(t.TempDir()), "must-not-run")
	rows := []verifyRow{
		{ID: "1", Command: "printf first; exit 1", Expect: "exit 0"},
		{ID: "2", Command: fmt.Sprintf("touch %q", marker), Expect: "exit 0"},
	}
	got, err := runFailFirstBase(resolveShellPlan(), sandboxUnshare, root, base, rows,
		map[string]bool{"1": true, "2": true}, time.Minute, false, l)
	if err == nil || got != nil {
		t.Fatalf("retention failure returned witness: %+v err=%v", got, err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("row ran after output retention failed")
	}
}

func TestBaseOutputFlagPreflight(t *testing.T) {
	for _, mode := range [][]string{{}, {"--check"}, {"--open-bracket"}, {"--close-bracket"}, {"--in-container"}, {"--bracket-digest"}} {
		args := append([]string{"--base-log", filepath.Join(canonicalPath(t.TempDir()), "logs")}, mode...)
		if len(mode) > 0 {
			args = append(args, "--fail-first")
		}
		res, out := captureVerifyrunOut(t, args)
		if res.code != verifyrunExitUsageError || !strings.Contains(out, "--base-log requires") {
			t.Fatalf("args=%v code=%d out=%s", args, res.code, out)
		}
	}
}

func TestBaseOutputVerifyrunIntegration(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell fixture and private-mode logs")
	}
	f := newRowsFixture(t, "| # | Command | Expect |\n|---|---|---|\n| 1 | `cat flag.txt; cat error.txt >&2; grep -q new flag.txt` | exit 0 (live) |\n")
	putFile(t, filepath.Join(f.root, "flag.txt"), "private-base-output\n")
	putFile(t, filepath.Join(f.root, "error.txt"), "private-base-error\n")
	runGit(t, f.root, "add", "flag.txt", "error.txt")
	runGit(t, f.root, "commit", "-qm", "base")
	base := mustGitOut(t, f.root, "rev-parse", "HEAD")
	putFile(t, filepath.Join(f.root, "flag.txt"), "new\n")
	putFile(t, filepath.Join(f.root, "error.txt"), "head-error\n")
	runGit(t, f.root, "add", "flag.txt", "error.txt")
	runGit(t, f.root, "commit", "-qm", "head")
	saved := failFirstMergeBase
	failFirstMergeBase = func(string) string { return base }
	t.Cleanup(func() { failFirstMergeBase = saved })
	logs := filepath.Join(canonicalPath(f.outside), "base-primary")
	res, out := captureVerifyrunOut(t, f.args("--fail-first", "--base", base, "--base-log", logs))
	if res.code != verifyrunExitPass {
		t.Fatalf("code=%d out=%s", res.code, out)
	}
	if strings.Contains(out, "private-base-output") || strings.Contains(out, "private-base-error") {
		t.Fatal("primary output leaked to console")
	}
	raw, err := os.ReadFile(filepath.Join(logs, "row-000001.log"))
	want := "private-base-output\nprivate-base-error\n"
	if err != nil || string(raw) != want {
		t.Fatalf("raw=%q err=%v", raw, err)
	}
	digest := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(want)))
	if !strings.Contains(out, "output="+digest+" log=row-000001.log") {
		t.Fatalf("missing digest association: %s", out)
	}
	if res, out := captureVerifyrunOut(t, []string{"--check", f.brief}); res.code != verifyrunExitPass {
		t.Fatalf("check code=%d out=%s", res.code, out)
	}
	// Existing evidence cannot be overwritten by another run using the same
	// output destination; neither the head command nor Evidence may change.
	before, err := os.ReadFile(f.brief)
	if err != nil {
		t.Fatal(err)
	}
	putFile(t, filepath.Join(f.root, "flag.txt"), "would-fail-at-head\n")
	res, out = captureVerifyrunOut(t, f.args("--fail-first", "--base", base, "--base-log", logs))
	if res.code != verifyrunExitCouldNot || !strings.Contains(out, "--base-log:") {
		t.Fatalf("code=%d out=%s", res.code, out)
	}
	after, _ := os.ReadFile(f.brief)
	if string(before) != string(after) {
		t.Fatal("retention preflight failure wrote a witness")
	}
}

func TestBaseOutputBracketDigestCompatibility(t *testing.T) {
	fx := newContainerFixture(t)
	fx.writeBracketReceipt(t, validCheck)
	if res, out := captureVerifyrunOut(t, fx.runArgs()); res.code != verifyrunExitPass {
		t.Fatalf("container fixture code=%d out=%s", res.code, out)
	}
	legacy := getFile(t, fx.witness)
	tree, err := homeTreeDigest(fx.home)
	if err != nil {
		t.Fatal(err)
	}
	seal := func(span string) string {
		digest := digestOf(t, span)
		return span + admissionBracketPrefix + fmt.Sprintf("run=%s source=%s tree=%s witness=%s opened=%s closed=2026-10-23T12:00:00Z before=receipt after=check-verifier\n", fixtureRun, fixtureSource, tree, digest, fixtureOpened)
	}
	if lines := verbLines(t, seal(legacy), verifyrunExitPass); len(lines) != 1 || !strings.Contains(lines[0], "verdict=match") {
		t.Fatalf("legacy bracket=%v", lines)
	}
	digest := "sha256:" + strings.Repeat("a", 64)
	base := failFirstCell(strings.Repeat("b", 12), baseRed, 1, strings.Repeat("c", 12)) + " output=" + digest + " log=row-000001.log"
	lines := strings.Split(legacy, "\n")
	lines[0], lines[1] = strings.Split(witnessHeaderFailFirst, "\n")[0], strings.Split(witnessHeaderFailFirst, "\n")[1]
	lines[2] = strings.TrimSuffix(lines[2], " |") + " | " + base + " |"
	span := strings.Join(lines, "\n")
	sealed := seal(span)
	if lines := verbLines(t, sealed, verifyrunExitPass); len(lines) != 1 || !strings.Contains(lines[0], "verdict=match") {
		t.Fatalf("output bracket=%v", lines)
	}
	tampered := strings.Replace(sealed, "output="+digest, "output=sha256:"+strings.Repeat("d", 64), 1)
	if lines := verbLines(t, tampered, verifyrunExitFail); len(lines) != 1 || !strings.Contains(lines[0], "verdict=mismatch") {
		t.Fatalf("output edit not bound: %v", lines)
	}
}
