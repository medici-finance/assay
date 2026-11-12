package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/gitcore/gitstub"
	"github.com/medici-finance/assay/gitquiet/goquiet"
	"github.com/medici-finance/assay/tools/desk/internal/clicontract"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestCLIHelpDoesNotDiscoverManifests runs the compiled command, from a checkout
// holding a large manifest population, with malformed configuration, hostile
// environment values and recording stubs for every child process an entrypoint
// step could start. Root and nested help, and version, must print and exit 0
// with zero effects: no config echo, no activation discovery, no file or index
// write, no credential lookup and no child process. A positive control from the
// same tree proves the fixture is discoverable, so a help form that discovered
// it would be observed rather than silently skipped.
//
// regression: #4706
func TestCLIHelpDoesNotDiscoverManifests(t *testing.T) {
	bin := hdmBuild(t)
	tree := hdmManifestTree(t)

	// Positive control: an execution form from the same tree reaches the
	// entrypoint's activation discovery and is refused before any forge read.
	var out, errs bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ctl := exec.CommandContext(ctx, bin, "prs")
	ctl.Dir, ctl.Env, ctl.Stdout, ctl.Stderr = tree, hdmEnv(hdmHome(t, "", false), ""), &out, &errs
	err := ctl.Run()
	if ctl.ProcessState == nil {
		t.Fatalf("control did not start: %v", err)
	}
	if code := ctl.ProcessState.ExitCode(); err == nil || code != deskkit.ExitUnverifiable || !strings.Contains(errs.String(), "could-not-check: test/desk-verbs inactive") {
		t.Fatalf("control did not discover the fixture: code=%d err=%v\n%s", code, err, &errs)
	}

	for _, cfg := range clicontract.MalformedConfigs {
		t.Run(cfg.Name, func(t *testing.T) {
			for _, args := range hdmForms() {
				t.Run(strings.Join(args, "_"), func(t *testing.T) {
					for _, consent := range []string{"", "1"} {
						hdmRunHelp(t, bin, hdmHome(t, cfg.Content, cfg.Name != "absent"), tree, args, consent)
					}
				})
			}
		})
	}
}

// hdmForms is every root and nested help, and version, spelling. The single-dash
// Go-flag spellings are refused by this command and pinned by TestCLILegacyForms.
func hdmForms() [][]string {
	forms := [][]string{{"-h"}, {"--help"}, {"help"}, {"--version"}, {"version"}}
	for _, v := range cliVerbs {
		forms = append(forms, clicontract.HelpForms(v)...)
	}
	return append(forms, []string{"--throughput", "--help"}, []string{"prs", "--table", "--help"})
}

func hdmBuild(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "deskboard")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if out, err := goquiet.Cmd(exec.Command("go", "build", "-o", bin, ".")).CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return bin
}

// hdmHome is a fresh home whose roster.env holds one malformed configuration.
func hdmHome(t *testing.T, content string, write bool) string {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if write {
		if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

// hdmEnv is hermetic: malformed values everywhere a loader would look, the
// config echo armed, and a PATH whose every tool records its own start.
func hdmEnv(home, consent string) []string {
	env := []string{"HOME=" + home, "USERPROFILE=" + home, "PATH=" + filepath.Join(filepath.Dir(home), "shims"), "KUBECONFIG=/dev/null", "DESK_TOOLS_DISABLED=1", "ASSAY_CONFIG_ECHO=full", "DESK_MIN_FREE_BYTES=invalid", "CELL_GO_CACHE_ROOT=" + filepath.Join(home, "unreadable-cache"), "CLI_SHIM_MARK=" + filepath.Join(filepath.Dir(home), "child-started")}
	if consent != "" {
		env = append(env, "ASSAY_TELEMETRY="+consent)
	}
	return env
}

func hdmRunHelp(t *testing.T, bin, home, dir string, args []string, consent string) {
	t.Helper()
	shims := filepath.Join(filepath.Dir(home), "shims")
	if err := os.MkdirAll(shims, 0o755); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		for _, name := range []string{"git", "gh", "go", "deskclaim-ref", "desktoken", "kubectl", "ssh", "bash"} {
			shim := "#!/bin/sh\necho \"$0\" >> \"$CLI_SHIM_MARK\"\nexit 1\n"
			if err := os.WriteFile(filepath.Join(shims, name), []byte(shim), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	mark := filepath.Join(filepath.Dir(home), "child-started")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir, cmd.Env = dir, hdmEnv(home, consent)
	before := hdmStamp(t, home)
	var out, errs bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errs
	if err := cmd.Run(); err != nil {
		t.Fatalf("help/version %v consent=%q: %v\n%s\n%s", args, consent, err, &out, &errs)
	}
	if out.Len() == 0 {
		t.Errorf("missing generated help/version output: %v", args)
	}
	if errs.Len() != 0 {
		t.Errorf("help %v read config or ran activation: %s", args, &errs)
	}
	if started, err := os.ReadFile(mark); err == nil {
		t.Errorf("help %v started a child process: %s", args, started)
	}
	// Help and version never select a measurement, so even a consented run
	// writes no performance record, lock, cache or configuration state.
	if after := hdmStamp(t, home); !reflect.DeepEqual(before, after) {
		t.Errorf("help/version %v consent=%q wrote configuration, lock, cache or telemetry state", args, consent)
	}
}

func hdmStamp(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			files[rel] = "directory"
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[rel] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// hdmManifestTree is a checkout with a large component.yaml population,
// malformed manifests, and a desk-verbs owner whose required key nothing
// provides (inactive): any activation discovery from this tree prints a refusal
// and exits unverifiable.
func hdmManifestTree(t *testing.T) string {
	t.Helper()
	tree := t.TempDir()
	write := func(rel, body string) {
		path := filepath.Join(tree, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitstub.Repo(t, tree)
	write("verbs/component.yaml", "component: test/desk-verbs\nversion: 0.0.1\nprovides:\n  - assay.desk.verbs\ninject:\n  required:\n    - key: test.never.provided\n  optional: []\n")
	for i := 0; i < 600; i++ {
		write(fmt.Sprintf("c/%02d/m%03d/component.yaml", i/40, i), fmt.Sprintf("component: test/bulk-%03d\nversion: 0.0.1\nprovides:\n  - test.bulk.%03d\ninject:\n  required: []\n  optional: []\n", i, i))
	}
	write("broken/a/component.yaml", "component: [unterminated\n\x00")
	write("broken/b/component.yaml", "version: 0.0.1\n")
	return tree
}
