//go:build unix

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Security regression #4845. Only the cell-declared destination crosses these
// credential-path allowlists; these tests use no real harness or credential.
func TestForgeEndpointComposedEnvironments(t *testing.T) {
	const base = "https://gitlab.example.com/api/v4"
	const other = "https://other.example.com/api/v4"
	cases := []struct{ name, forge, gitlab, fallback, layer, want string }{
		{"gitlab key", "gitlab", base, "", layerCellEnv, base},
		{"HTTP policy unchanged", "gitlab", "http://gitlab.example.com/api/v4", "", layerCellEnv, "http://gitlab.example.com/api/v4"},
		{"forge key", "gitlab", "", base, layerCellEnv, base},
		{"precedence", "gitlab", base, other, layerCellEnv, base},
		{"empty primary falls back", "gitlab", "", base, layerCellEnv, base},
		{"whitespace primary falls back", "gitlab", " \t", base, layerCellEnv, base},
		{"empty values", "gitlab", "", "", layerCellEnv, ""},
		{"whitespace values", "gitlab", " \t", " \t", layerCellEnv, ""},
		{"compiled defaults", "gitlab", base, base, layerCompiled, ""},
		{"host defaults", "gitlab", base, base, layerDefaults, ""},
		{"inherited values", "gitlab", base, base, layerProcess, ""},
		{"github cell", "github", base, other, layerCellEnv, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := codexEnvironmentCell(t)
			c.Forge = tc.forge
			c.Env.putFrom("GITLAB_API_BASE", tc.gitlab, tc.layer)
			c.Env.putFrom("FORGE_API_BASE", tc.fallback, tc.layer)
			c.Env.putFrom("CUSTOM_SECRET", "fixture-secret", layerCellEnv)
			input := []string{"GITLAB_API_BASE=" + other, "FORGE_API_BASE=" + other, "CUSTOM_SECRET=fixture-secret", "GH_TOKEN=fixture-token"}
			before := append([]string(nil), input...)
			args, err := c.codexEnvironmentArgs(input)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(input, before) {
				t.Fatal("operator harness environment mutated")
			}
			values := map[string]string{}
			for _, arg := range args {
				if rest, ok := strings.CutPrefix(arg, "shell_environment_policy.set."); ok {
					key, raw, _ := strings.Cut(rest, "=")
					var value string
					if err := json.Unmarshal([]byte(raw), &value); err != nil {
						t.Fatal(err)
					}
					values[key] = value
				}
			}
			if got, ok := values["GITLAB_API_BASE"]; got != tc.want || ok != (tc.want != "") {
				t.Errorf("Codex endpoint = %q, present=%v; want %q", got, ok, tc.want)
			}
			for _, key := range []string{"FORGE_API_BASE", "CUSTOM_SECRET", "GH_TOKEN"} {
				if _, ok := values[key]; ok {
					t.Fatalf("Codex argv exports %s", key)
				}
			}
			if values["HOME"] != c.Home || values["CODEX_HOME"] != filepath.Join(c.Env.Get("HOME"), ".codex") {
				t.Fatal("Codex home boundary changed")
			}

			// Apply the emitted Codex command settings to the inherited subprocess
			// environment, as Codex does, and check a real local child. Absence of a
			// declaration preserves inheritance; #4855 owns any clearing policy.
			commandEnv := append([]string(nil), input...)
			for key, value := range values {
				commandEnv = envSet(commandEnv, key, value)
			}
			wantChild := tc.want
			if wantChild == "" {
				wantChild = other
			}
			child := exec.Command("/bin/sh", "-c", `printf '%s' "${GITLAB_API_BASE-}"`)
			child.Env = commandEnv
			output, err := child.CombinedOutput()
			if err != nil || string(output) != wantChild {
				t.Fatalf("Codex composed child: %q, %v; want %q", output, err, wantChild)
			}

			stubHarnessesOnPath(t)
			c.Kind = "scrubbed"
			for _, harness := range []string{"claude", "codex"} {
				composed := c.scrubbedComposeEnv("worker-desk", harness, "fixture")
				got, ok := envLookup(composed.Pairs, "GITLAB_API_BASE")
				if got != tc.want || ok != (tc.want != "") {
					t.Errorf("scrubbed %s endpoint = %q, present=%v; want %q", harness, got, ok, tc.want)
				}
				for _, key := range []string{"FORGE_API_BASE", "CUSTOM_SECRET", "GH_TOKEN"} {
					if _, ok := envLookup(composed.Pairs, key); ok {
						t.Fatalf("scrubbed exports %s", key)
					}
				}
				if envValue(composed.Pairs, "HOME") != c.Home || envValue(composed.Pairs, "KUBECONFIG") != "/dev/null" {
					t.Fatal("scrubbed isolation changed")
				}
				if harness == "codex" && envValue(composed.Pairs, "CODEX_HOME") != filepath.Join(c.Home, ".codex") {
					t.Fatal("scrubbed Codex uses operator home")
				}
				cmd := exec.Command("/bin/sh", "-c", `printf '%s' "${GITLAB_API_BASE-}"`)
				cmd.Env = composed.Pairs
				out, err := cmd.CombinedOutput()
				if err != nil || string(out) != tc.want {
					t.Errorf("composed child: %q, %v", out, err)
				}
			}
		})
	}
}

// Exercise the real launch through a stub Codex, without starting a role.
func TestCodexLaunchForwardsCellForgeBase(t *testing.T) {
	for _, tc := range forgeEnvCases {
		t.Run(tc.name, func(t *testing.T) {
			f := newPolicyFixture(t, "")
			f.prepareLocalLaunch(t)
			stub := "#!/bin/sh\nif [ \"$1\" = --version ]; then echo 'codex-cli 1.0.0'; exit 0; fi\nprintf 'ARG=%s\\n' \"$@\"\nprintf 'OPERATOR_HOME=%s\\n' \"$HOME\"\n"
			must(t, os.WriteFile(filepath.Join(f.binDir, "codex"), []byte(stub), 0755))
			writeForgeCellEnv(t, f, "CELL_HARNESS=codex\nCODEX_MODEL_default=fixture-model\n"+tc.cellEnv)
			r := f.run(t, append([]string{"CELLCTL_DESKWT=0"}, tc.shell...), "desk", "example", "worker-desk", "--cadence", "off")
			if r.code != 0 {
				t.Fatalf("launch exit=%d\n%s\n%s", r.code, r.stdout, r.stderr)
			}
			want := ""
			if strings.Contains(tc.cellEnv, "CELL_FORGE=gitlab") && strings.Contains(tc.cellEnv, "="+forgeFixtureBase) {
				want = forgeFixtureBase
			}
			prefix := "ARG=shell_environment_policy.set.GITLAB_API_BASE="
			if want == "" {
				if strings.Contains(r.stdout, prefix) {
					t.Fatal(r.stdout)
				}
			} else {
				quoted, _ := json.Marshal(want)
				if !strings.Contains(r.stdout, prefix+string(quoted)+"\n") {
					t.Fatal(r.stdout)
				}
			}
			if strings.Contains(r.stdout, "ARG=shell_environment_policy.set.FORGE_API_BASE=") {
				t.Fatal("FORGE_API_BASE exported")
			}
			if !strings.Contains(r.stdout, "OPERATOR_HOME="+f.cellDir+"\n") {
				t.Fatal("operator home changed", r.stdout)
			}
		})
	}
}

// Refuse credential carriers before either argv or a scrubbed plan exists.
func TestForgeEndpointSecretRefusal(t *testing.T) {
	for _, key := range []string{"GITLAB_API_BASE", "FORGE_API_BASE"} {
		for name, base := range map[string]string{
			"userinfo":  "https://fixture-user:fixture-secret@gitlab.example.com/api/v4",
			"query":     "https://gitlab.example.com/api/v4?token=fixture-secret",
			"fragment":  "https://gitlab.example.com/api/v4#fixture-secret",
			"malformed": "https://gitlab.example.com/%zz/fixture-secret",
		} {
			t.Run(key+"/"+name, func(t *testing.T) {
				c := codexEnvironmentCell(t)
				c.Forge = "gitlab"
				c.Env.putFrom(key, base, layerCellEnv)
				args, err := c.codexEnvironmentArgs([]string{"GITLAB_API_BASE=https://inherited.example/api/v4"})
				if err == nil || len(args) != 0 {
					t.Errorf("Codex must refuse before emitting argv")
				} else if strings.Contains(err.Error(), "fixture-secret") || strings.Contains(err.Error(), base) {
					t.Fatal("Codex refusal discloses endpoint")
				}
				stubHarnessesOnPath(t)
				c.Kind = "scrubbed"
				for _, harness := range []string{"claude", "codex"} {
					code := -1
					var plan string
					stderr := captureStderr(t, func() {
						plan = captureStdout(t, func() {
							code = dieRun(func() { c.printPlan(c.scrubbedComposeEnv("worker-desk", harness, "s"), "/wt", nil) })
						})
					})
					if code != 3 || plan != "" || !strings.Contains(stderr, "cell forge endpoint") {
						t.Errorf("scrubbed %s must refuse before plan: code=%d", harness, code)
					}
					if strings.Contains(stderr+plan, "fixture-secret") || strings.Contains(stderr+plan, base) {
						t.Fatal("scrubbed refusal discloses endpoint")
					}
				}
			})
		}
	}
}

func TestCodexRejectsURLSecret(t *testing.T) {
	for _, key := range []string{"GITLAB_API_BASE", "FORGE_API_BASE"} {
		t.Run(key, func(t *testing.T) {
			f := newPolicyFixture(t, "")
			f.prepareLocalLaunch(t)
			stub := "#!/bin/sh\nif [ \"$1\" = --version ]; then echo 'codex-cli 1.0.0'; exit 0; fi\necho HARNESS_STARTED\n"
			must(t, os.WriteFile(filepath.Join(f.binDir, "codex"), []byte(stub), 0755))
			writeForgeCellEnv(t, f, "CELL_HARNESS=codex\nCODEX_MODEL_default=fixture-model\nCELL_FORGE=gitlab\n"+key+"=https://fixture-user:fixture-secret@gitlab.example.com/api/v4\n")
			r := f.run(t, []string{"CELLCTL_DESKWT=0", "GITLAB_API_BASE=https://inherited.example/api/v4"}, "desk", "example", "worker-desk", "--cadence", "off")
			if r.code != 3 || !strings.Contains(r.stderr, "cell forge endpoint") {
				t.Fatalf("expected safe refusal: exit=%d", r.code)
			}
			if strings.Contains(r.stdout+r.stderr, "fixture-secret") || strings.Contains(r.stdout, "HARNESS_STARTED") {
				t.Fatal("unsafe endpoint reached output or harness")
			}
		})
	}
}
