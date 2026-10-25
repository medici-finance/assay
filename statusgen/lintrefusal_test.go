package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/gitquiet/goquiet"
)

// regression: #3057
func TestLintConfigRefusal(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "statusgen")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if out, err := goquiet.Cmd(exec.CommandContext(ctx, "go", "build", "-o", bin, ".")).CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	// Exercise the actual file-class configuration loader, independently of the
	// package fixture and the caller's Actions variables, roster and telemetry.
	var env []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "ASSAY_") || strings.HasPrefix(key, "GITHUB_") || key == "HOME" || key == "USERPROFILE" || key == "DESK_ROOTS" {
			continue
		}
		env = append(env, entry)
	}
	for _, tc := range []struct{ name, roster, reason string }{
		{"unknown key", fixtureRoster + "ASSAY_FUTURE_TEST_KEY=1\n", "unknown key"},
		{"schema mismatch", fixtureRoster + "ASSAY_ROSTER_SCHEMA=unsupported-test-schema\n", "schema version"},
		{"missing file", "", "no roster configured"},
		{"valid control", fixtureRoster, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			if tc.roster != "" {
				dir := filepath.Join(home, ".config", "assay")
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(tc.roster), 0600); err != nil {
					t.Fatal(err)
				}
			}
			for _, mode := range []string{"plain", "multiple roots", "differential"} {
				t.Run(mode, func(t *testing.T) {
					root := t.TempDir()
					if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
						t.Fatal(err)
					}
					args := []string{"--root", root, "--lint"}
					switch mode {
					case "multiple roots":
						other := t.TempDir()
						if err := os.MkdirAll(filepath.Join(other, "docs", "streams"), 0700); err != nil {
							t.Fatal(err)
						}
						args = append(args, "--root", other, "--allow-empty-root")
					case "differential":
						args = append(args, "--diff-base", "refs/remotes/origin/main")
					}
					cmd := exec.CommandContext(ctx, bin, args...)
					cmd.Env = append(append([]string{}, env...), "HOME="+home, "USERPROFILE="+home)
					var out, diag strings.Builder
					cmd.Stdout, cmd.Stderr = &out, &diag
					err := cmd.Run()
					code := 0
					if err != nil {
						ee, ok := err.(*exec.ExitError)
						if !ok {
							t.Fatalf("run: %v", err)
						}
						code = ee.ExitCode()
					}
					if tc.reason == "" {
						if code != 0 || strings.TrimSpace(out.String()) != "LINT: PASS" {
							t.Fatalf("valid control: exit=%d stdout=%q stderr=%s", code, out.String(), diag.String())
						}
					} else {
						if code != 5 || strings.Contains(out.String(), "LINT: PASS") || strings.TrimSpace(out.String()) != "LINT: REFUSED configuration; trust-dependent checks could-not-check" {
							t.Errorf("refused configuration must exit 5 without PASS: exit=%d stdout=%q", code, out.String())
						}
						for _, want := range []string{"assay-config: REFUSED", tc.reason, "trust-dependent checks could-not-check"} {
							if !strings.Contains(diag.String(), want) {
								t.Errorf("stderr missing %q: %s", want, diag.String())
							}
						}
					}
					if _, err := os.Stat(filepath.Join(root, "STATUS.md")); !os.IsNotExist(err) {
						t.Errorf("lint wrote STATUS.md: %v", err)
					}
				})
			}
		})
	}
}
