package gittest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// regression: #2716
func TestFixtureConfigIsolation(t *testing.T) {
	for _, mode := range []string{"global", "includeIf", "system"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			canary, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			log := filepath.Join(root, "signing.log")
			write := func(name, body string) string {
				t.Helper()
				path := filepath.Join(root, name)
				if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
				return path
			}
			// Every external config path is owned, including the positive control.
			// The signing program is this test binary's recording/refusing canary,
			// never a host key manager, signing agent or real signing key.
			global := write("global", "")
			system := write("system", "")
			signing := fmt.Sprintf("[commit]\n\tgpgsign = true\n[gpg]\n\tformat = openpgp\n\tprogram = %q\n[user]\n\tsigningkey = fixture-only\n", filepath.ToSlash(canary))
			switch mode {
			case "global":
				write("global", signing)
			case "includeIf":
				included := write("included", signing)
				write("global", fmt.Sprintf("[includeIf \"gitdir:**\"]\n\tpath = %q\n", filepath.ToSlash(included)))
			case "system":
				write("system", signing)
			}
			t.Setenv("GIT_CONFIG_GLOBAL", global)
			t.Setenv("GIT_CONFIG_SYSTEM", system)
			t.Setenv("GIT_CONFIG_NOSYSTEM", "0")
			t.Setenv("GIT_CONFIG_COUNT", "0")
			t.Setenv("GIT_CONFIG_PARAMETERS", "")
			t.Setenv("GITTEST_SIGN_CANARY", log)
			raw := func(args ...string) error {
				t.Helper()
				cmd := exec.Command("git", args...)
				cmd.Dir = root
				return cmd.Run()
			}
			if err := raw("init", "-q", "-b", "main"); err != nil {
				t.Fatal(err)
			}
			if err := raw("-c", "user.name=test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-qm", "control"); err == nil {
				t.Fatal("positive control: signing config was not active")
			}
			if b, err := os.ReadFile(log); err != nil || string(b) != "signing invoked\n" {
				t.Fatalf("positive control: signing canary did not record invocation: %q, %v", b, err)
			}
			if err := os.Remove(log); err != nil {
				t.Fatal(err)
			}

			// A new caller of the shared fixture must get the existing seed SHA,
			// rather than inherit signing or regenerate a golden around it.
			f := NewFixture(t)
			if got, err := f.Git("rev-parse", "HEAD"); err != nil || got != "d394df7f3b666094898c14b7be6bb39fc709d929" {
				t.Fatalf("seed SHA = %q, %v", got, err)
			}
			for _, scope := range []string{"--global", "--system"} {
				if got, err := f.Git("config", scope, "--list"); err != nil || strings.TrimSpace(got) != "" {
					t.Errorf("fixture %s config = %q, %v; want empty", scope, got, err)
				}
			}
			if b, err := os.ReadFile(log); !os.IsNotExist(err) {
				t.Errorf("fixture invoked signing canary: %q, %v", b, err)
			}
		})
	}
}
