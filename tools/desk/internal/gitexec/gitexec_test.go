package gitexec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAllowlistRefusesUnknownToolVerb(t *testing.T) {
	if Allowed("deskgit", "merge") {
		t.Fatal("deskgit:merge must not be allowlisted")
	}
	if Allowed("nosuchtool", "fetch") {
		t.Fatal("unknown tool must not be allowlisted")
	}
	if !Allowed("deskmerge", "merge") {
		t.Fatal("deskmerge:merge is the single sanctioned caller — must be allowlisted")
	}
	if !Allowed("deskgit", "ls-remote") {
		t.Fatal("deskgit:ls-remote is a seeded baseline verb — must be allowlisted")
	}
}

func TestScrubbedEnvDropsGitVars(t *testing.T) {
	parent := []string{
		"PATH=/bin", "HOME=/tmp", "TERM=xterm",
		"GIT_SSH_COMMAND=/tmp/evil", "GIT_ASKPASS=/tmp/evil2",
		"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.sshCommand", "GIT_CONFIG_VALUE_0=/tmp/evil3",
		"LC_ALL=C", "EVIL=1",
	}
	got := strings.Join(scrubbedEnv(parent), "\n")
	for _, mustDrop := range []string{"GIT_SSH_COMMAND", "GIT_ASKPASS", "GIT_CONFIG_COUNT", "GIT_CONFIG_KEY_0", "GIT_CONFIG_VALUE_0", "EVIL"} {
		if strings.Contains(got, mustDrop) {
			t.Fatalf("scrubbed env must drop %s; got:\n%s", mustDrop, got)
		}
	}
	for _, mustKeep := range []string{"PATH=/bin", "HOME=/tmp", "TERM=xterm", "LC_ALL=C", "GIT_TERMINAL_PROMPT=0"} {
		if !strings.Contains(got, mustKeep) {
			t.Fatalf("scrubbed env must keep %s; got:\n%s", mustKeep, got)
		}
	}
}

func TestRunRefusesNonAllowlistedVerbWithoutSpawning(t *testing.T) {
	if _, err := Run("deskgit", t.TempDir(), "merge"); err == nil {
		t.Fatal("expected refusal for non-allowlisted verb")
	} else if !strings.Contains(err.Error(), "not allowlisted") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunExecutesAllowlistedVerbInFixture(t *testing.T) {
	dir := t.TempDir()
	init := []string{"init", "-q", "-b", "main"}
	if _, err := Run("deskadvisory", dir, init...); err != nil {
		t.Fatalf("fixture init: %v", err)
	}
	for _, kv := range [][2]string{
		{"user.name", "test"},
		{"user.email", "test@example.invalid"},
	} {
		if _, err := Run("deskwt", dir, "config", kv[0], kv[1]); err != nil {
			t.Fatalf("fixture config: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Run("deskmerge", dir, "add", "f.txt"); err != nil {
		t.Fatalf("fixture add: %v", err)
	}
	if _, err := Run("deskmerge", dir, "commit", "-q", "-m", "seed"); err != nil {
		t.Fatalf("fixture commit: %v", err)
	}
	out, err := Run("deskgit", dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
	if len(out) != 40 {
		t.Fatalf("rev-parse HEAD: want 40-hex sha, got %q", out)
	}
}

func TestRunReportsStderrOnFailure(t *testing.T) {
	dir := t.TempDir()
	if _, err := Run("deskgit", dir, "rev-parse", "HEAD"); err == nil {
		t.Fatal("expected failure in empty dir")
	} else if !strings.Contains(err.Error(), "gitexec: deskgit: git rev-parse") {
		t.Fatalf("unexpected error shape: %v", err)
	}
}
