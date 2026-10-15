package main

import (
	"encoding/json"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestTelemetryConfigurationPrecedence(t *testing.T) {
	for _, tc := range []struct{ name, parent, defaults, cell, want string }{
		{"cell on", "0", "0", "1", "1"}, {"cell off", "1", "1", "0", "0"},
		{"empty off", "1", "1", "", "0"}, {"defaults off", "1", "0", "absent", "0"},
		{"parent on", "1", "absent", "absent", "1"}, {"exact opt in only", "true", "absent", "absent", "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(envAssayTelemetry, tc.parent)
			e := newEnvFromProcess()
			root := t.TempDir()
			if tc.defaults != "absent" {
				p := filepath.Join(root, "defaults.env")
				if err := os.WriteFile(p, []byte(envAssayTelemetry+"="+tc.defaults+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
				if _, _, err := overlayCellDefaults(e, p); err != nil {
					t.Fatal(err)
				}
			}
			if tc.cell != "absent" {
				p := filepath.Join(root, "cell.env")
				if err := os.WriteFile(p, []byte(envAssayTelemetry+"="+tc.cell+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := parseCellEnv(e, p); err != nil {
					t.Fatal(err)
				}
			}
			c := &Cell{Env: e}
			if got := envValue(c.telemetryEnv([]string{envAssayTelemetry + "=1"}), envAssayTelemetry); got != tc.want {
				t.Fatalf("effective=%q want=%q", got, tc.want)
			}
		})
	}
}

func TestTelemetryAllRoleLaunchEnvironments(t *testing.T) {
	for _, value := range []string{"1", "0", "", "true", "untrusted-value"} {
		c := codexEnvironmentCell(t)
		c.Env.Put(envAssayTelemetry, value)
		want := "0"
		if value == "1" {
			want = "1"
		}
		for _, role := range knownRoles {
			env := c.telemetryEnv([]string{envAssayTelemetry + "=1", "DESK_LOOP=" + role, "PRIVATE_SECRET=never-copy"})
			if envValue(env, envAssayTelemetry) != want {
				t.Fatal("house telemetry missing")
			}
			values, err := c.codexCommandEnvironment(env)
			if err != nil {
				t.Fatal(err)
			}
			if values[envAssayTelemetry] != want || values["PRIVATE_SECRET"] != "" {
				t.Fatal("Codex opt-in or secret boundary wrong")
			}
			args, err := c.codexEnvironmentArgs(env)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(strings.Join(args, " "), "shell_environment_policy.set."+envAssayTelemetry+"=\""+want+"\"") {
				t.Fatal("Codex command opt-in absent")
			}
			for _, h := range []string{"claude", "codex", "cursor"} {
				_, child, err := prepareTickLaunch(h, []string{h, "prompt"}, env, 90*time.Second)
				if err != nil {
					t.Fatal(err)
				}
				if envValue(child, envAssayTelemetry) != want {
					t.Fatal("cadence lost telemetry")
				}
			}
			if envValue(cursorLaunchEnv(env), envAssayTelemetry) != want {
				t.Fatal("Cursor lost telemetry")
			}
		}
		s := scrubbedFixture(t)
		s.Env.Put(envAssayTelemetry, value)
		for _, h := range []string{"claude", "codex"} {
			if envValue(s.scrubbedComposeEnv("worker-desk", h, "fixture").Pairs, envAssayTelemetry) != want {
				t.Fatal("scrubbed telemetry missing")
			}
		}
	}
}

func TestTelemetryChildProcess(t *testing.T) {
	externalFixture := os.Getenv("CELL_KIND") == "container" && slices.Contains(os.Args[1:], "-test.run=^TestTelemetryChildProcess$")
	if os.Getenv("ASSAY_TEST_TELEMETRY_CHILD") != "1" && !externalFixture {
		t.Skip("subprocess fixture")
	}
	if code := deskkit.PerfRun("deskboard", []string{"--version"}, []string{"--version"}, func([]string) int { return 0 }); code != 0 {
		t.Fatal(code)
	}
}

func telemetryRows(t *testing.T, home string) int {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(home, ".config", "assay", "perf", "????-??-??.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, p := range files {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
			if line == "" {
				continue
			}
			var r struct{ Schema, Tool string }
			if err := json.Unmarshal([]byte(line), &r); err != nil {
				t.Fatal(err)
			}
			if r.Schema != "perf-v1" || r.Tool != "deskboard" {
				t.Fatalf("bad record: %s", line)
			}
			count++
		}
	}
	return count
}

func TestTelemetryLaunchedChildLogGrowth(t *testing.T) {
	c := codexEnvironmentCell(t)
	for i, value := range []string{"1", "0", "1"} {
		c.Env.Put(envAssayTelemetry, value)
		env := c.telemetryEnv(envSet(envSet(os.Environ(), "HOME", c.Home), "USERPROFILE", c.Home))
		env = envSet(env, "ASSAY_TEST_TELEMETRY_CHILD", "1")
		cmd := exec.Command(os.Args[0], "-test.run=^TestTelemetryChildProcess$")
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("child: %v %s", err, out)
		}
		if n := telemetryRows(t, c.Home); n != []int{1, 1, 2}[i] {
			t.Fatalf("opt-in %q: rows=%d", value, n)
		}
	}
}

func TestTelemetryContainerPaths(t *testing.T) {
	c, _ := nativeFixture(t, "worker-desk")
	for _, value := range []string{"1", "0"} {
		c.Env.Put(envAssayTelemetry, value)
		p := c.nativeContainerPlan("worker-desk", "codex", "test-model")
		if p.Env[envAssayTelemetry] != value {
			t.Fatal("native container lost opt-in")
		}
	}
	if runtime.GOOS == "windows" {
		t.Skip("external Linux launcher fixture requires host HOME semantics")
	}
	home := t.TempDir()
	e := envWith(map[string]string{"HOME": home, "PATH": os.Getenv("PATH"), "CELL_CONTAINER_LAUNCHER": os.Args[0]})
	external := &Cell{Env: e, Name: "fixture", Kind: "container"}
	for i, value := range []string{"1", "0", "1"} {
		external.Env.Put(envAssayTelemetry, value)
		external.containerRun("-test.run=^TestTelemetryChildProcess$")
		if n := telemetryRows(t, home); n != []int{1, 1, 2}[i] {
			t.Fatalf("external rows=%d", n)
		}
	}
}

func TestTelemetryHelperAndServiceLogGrowth(t *testing.T) {
	for _, path := range []string{"helper", "comms"} {
		t.Run(path, func(t *testing.T) {
			c, cfg, _ := commsFixture(t)
			for i, value := range []string{"1", "0", "1"} {
				c.Env.Put(envAssayTelemetry, value)
				var env []string
				if path == "helper" {
					env = c.deskHelperEnv(os.Environ(), "worker-desk", "fixture")
				} else {
					env = c.commsServiceEnv(cfg)
				}
				cmd := exec.Command(os.Args[0], "-test.run=^TestTelemetryChildProcess$")
				cmd.Env = envSet(env, "ASSAY_TEST_TELEMETRY_CHILD", "1")
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("child: %v %s", err, out)
				}
				if got := telemetryRows(t, c.Home); got != []int{1, 1, 2}[i] {
					t.Fatalf("opt-in %q: rows=%d", value, got)
				}
			}
		})
	}
}

func TestTelemetryShowProcessSource(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("policy fixture uses Unix shell scripts")
	}
	f := newPolicyFixture(t, "2.1.278")
	for _, value := range []string{"1", "0", "true"} {
		want := "0"
		if value == "1" {
			want = "1"
		}
		r := f.run(t, []string{envAssayTelemetry + "=" + value}, "show", "example")
		if r.code != 0 || !strings.Contains(r.stdout, "[show] ASSAY_TELEMETRY="+want+" (process environment)") {
			t.Fatalf("value=%q code=%d stdout=%s stderr=%s", value, r.code, r.stdout, r.stderr)
		}
	}
}
