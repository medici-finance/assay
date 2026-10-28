package gitmaintenance

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func casGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "gc.auto=0", "-c", "maintenance.auto=false", "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// regression: #2513
func TestTempDeleteUsesCAS(t *testing.T) {
	dir := t.TempDir()
	casGit(t, dir, "init", "-q", "-b", "main")
	casGit(t, dir, "commit", "--allow-empty", "-qm", "one")
	one := casGit(t, dir, "rev-parse", "HEAD")
	casGit(t, dir, "commit", "--allow-empty", "-qm", "two")
	two := casGit(t, dir, "rev-parse", "HEAD")
	ref := "refs/deskmerge/tmp/example"
	casGit(t, dir, "update-ref", ref, two)
	if err := deleteTempRef(context.Background(), dir, ref, one); err == nil {
		t.Fatal("moved ref deleted without expected hash")
	}
	if got := casGit(t, dir, "rev-parse", ref); got != two {
		t.Fatal("moved ref lost")
	}
	if err := deleteTempRef(context.Background(), dir, ref, two); err != nil {
		t.Fatal(err)
	}
	if out := casGit(t, dir, "for-each-ref", "--format=%(refname)", ref); out != "" {
		t.Fatalf("matched ref not deleted: %s", out)
	}
}

// regression: #2513
func TestTempDeleteNoAmbient(t *testing.T) {
	dir := t.TempDir()
	casGit(t, dir, "init", "-q", "-b", "main")
	casGit(t, dir, "commit", "--allow-empty", "-qm", "one")
	oid := casGit(t, dir, "rev-parse", "HEAD")
	ref := "refs/deskmerge/tmp/example"
	casGit(t, dir, "update-ref", ref, oid)
	marker := filepath.Join(t.TempDir(), "hook-ran")
	hook := filepath.Join(t.TempDir(), "reference-transaction")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\n: > '"+marker+"'\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "wrong-repo"))
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.hooksPath")
	t.Setenv("GIT_CONFIG_VALUE_0", filepath.Dir(hook))
	if err := deleteTempRef(context.Background(), dir, ref, oid); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("injected reference transaction hook ran")
	}
}

// regression: #2513
func TestTempDeleteHonorsHook(t *testing.T) {
	for _, configured := range []bool{false, true} {
		dir := t.TempDir()
		casGit(t, dir, "init", "-q", "-b", "main")
		casGit(t, dir, "commit", "--allow-empty", "-qm", "one")
		oid := casGit(t, dir, "rev-parse", "HEAD")
		ref := "refs/deskmerge/tmp/example"
		casGit(t, dir, "update-ref", ref, oid)
		hookDir := filepath.Join(dir, ".git", "hooks")
		if configured {
			hookDir = t.TempDir()
			casGit(t, dir, "config", "core.hooksPath", hookDir)
		}
		marker := filepath.Join(t.TempDir(), "hook-ran")
		if err := os.WriteFile(filepath.Join(hookDir, "reference-transaction"), []byte("#!/bin/sh\n: > '"+marker+"'\nexit 1\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := deleteTempRef(context.Background(), dir, ref, oid); err == nil {
			t.Fatal("existing hook refusal bypassed")
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatal("maintenance refusal unexpectedly ran existing hook", err)
		}
		if got := casGit(t, dir, "rev-parse", ref); got != oid {
			t.Fatal("hook-refused reference was deleted")
		}
	}
}

// regression: #2513
func TestTempDeleteCustodyResult(t *testing.T) {
	prev := refProcess
	t.Cleanup(func() { refProcess = prev })
	dir := t.TempDir()
	casGit(t, dir, "init", "-q", "-b", "main")
	casGit(t, dir, "commit", "--allow-empty", "-qm", "one")
	ref := "refs/deskmerge/tmp/example"
	refProcess = func(_ context.Context, argv []string, _ []string, _ string, _ io.Writer, _ io.Writer) (int, bool, error) {
		for _, arg := range argv {
			if strings.HasPrefix(arg, "core.hooksPath=") {
				t.Error("native primitive overrides effective hooks")
			}
		}
		return 0, true, nil
	}
	if err := deleteTempRef(context.Background(), dir, ref, strings.Repeat("1", 40)); !errors.Is(err, errCleanupUnknown) {
		t.Fatalf("cleanup uncertainty lost: %v", err)
	}
	refProcess = func(_ context.Context, _ []string, _ []string, _ string, _ io.Writer, stderr io.Writer) (int, bool, error) {
		_, _ = stderr.Write([]byte("native warning"))
		return 0, false, nil
	}
	if err := deleteTempRef(context.Background(), dir, ref, strings.Repeat("1", 40)); !errors.Is(err, ErrRefDiagnostic) || !strings.Contains(err.Error(), "native warning") {
		t.Fatalf("stderr uncertainty lost: %v", err)
	}
}

// regression: #2513
func TestTempDeleteRejectsInputs(t *testing.T) {
	dir := t.TempDir()
	casGit(t, dir, "init", "-q", "-b", "main")
	casGit(t, dir, "commit", "--allow-empty", "-qm", "one")
	oid := casGit(t, dir, "rev-parse", "HEAD")
	casGit(t, dir, "update-ref", "refs/dispatch/active", oid)
	for _, ref := range []string{"refs/heads/main", "refs/dispatch/active", "refs/deskmerge/tmp/../other", "refs/deskmerge/tmp/--bad\nref"} {
		if err := deleteTempRef(context.Background(), dir, ref, oid); err == nil {
			t.Fatalf("invalid ref %q admitted", ref)
		}
	}
	for _, ref := range []string{"refs/heads/main", "refs/dispatch/active"} {
		if got := casGit(t, dir, "rev-parse", ref); got != oid {
			t.Fatalf("durable or claimed ref %s changed", ref)
		}
	}
	if err := deleteTempRef(context.Background(), t.TempDir(), "refs/deskmerge/tmp/example", "HEAD"); err == nil {
		t.Fatal("non-hash admitted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := deleteTempRef(ctx, t.TempDir(), "refs/deskmerge/tmp/example", strings.Repeat("1", 40)); err == nil {
		t.Fatal("cancel ignored")
	}
}
