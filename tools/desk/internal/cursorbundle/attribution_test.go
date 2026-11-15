package cursorbundle

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain pins the config-location overrides to unset, so no test inherits the host's
// CURSOR_CONFIG_DIR or XDG_CONFIG_HOME.
func TestMain(m *testing.M) {
	HostEnv = func(string) string { return "" }
	os.Exit(m.Run())
}

// pinHost points the check at home, with env as the only set overrides and goos as the OS.
func pinHost(t *testing.T, home string, env map[string]string, goos string) {
	t.Helper()
	sh, se, so := HostHome, HostEnv, HostOS
	t.Cleanup(func() { HostHome, HostEnv, HostOS = sh, se, so })
	HostHome = func() (string, error) { return home, nil }
	HostEnv = func(k string) string { return env[k] }
	HostOS = goos
}

func writeCLIConfig(t *testing.T, home, body string) string {
	t.Helper()
	p := CLIConfigPath(home)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func states(r AttributionReport) []AttributionState {
	var out []AttributionState
	for _, s := range r.Settings {
		out = append(out, s.State)
	}
	return out
}

// regression: #5892
func TestAttributionStates(t *testing.T) {
	on, off, cnc := AttributionOn, AttributionOff, AttributionCouldNotCheck
	cases := []struct {
		name string
		body string // "" = no file
		want []AttributionState
	}{
		{"absent file", "", []AttributionState{on, on}},
		{"no block", `{"version":1}`, []AttributionState{on, on}},
		{"null block", `{"attribution":null}`, []AttributionState{on, on}},
		{"both false", `{"attribution":{"attributeCommitsToAgent":false,"attributePRsToAgent":false}}`, []AttributionState{off, off}},
		{"both true", `{"attribution":{"attributeCommitsToAgent":true,"attributePRsToAgent":true}}`, []AttributionState{on, on}},
		{"commits only off", `{"attribution":{"attributeCommitsToAgent":false}}`, []AttributionState{off, on}},
		{"string value", `{"attribution":{"attributeCommitsToAgent":"false","attributePRsToAgent":false}}`, []AttributionState{cnc, off}},
		{"malformed", `{"attribution":`, []AttributionState{cnc, cnc}},
		{"block not object", `{"attribution":false}`, []AttributionState{cnc, cnc}},
		{"top not object", `[]`, []AttributionState{cnc, cnc}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := t.TempDir()
			path := CLIConfigPath(home)
			if c.body != "" {
				path = writeCLIConfig(t, home, c.body)
			}
			r := CheckAttributionAt(path)
			got := states(r)
			if len(got) != 2 || got[0] != c.want[0] || got[1] != c.want[1] {
				t.Fatalf("states = %v, want %v", got, c.want)
			}
			if r.Settings[0].Key != AttrCommitsKey || r.Settings[1].Key != AttrPRsKey {
				t.Fatalf("keys = %q, %q", r.Settings[0].Key, r.Settings[1].Key)
			}
			if want := c.want[0] == off && c.want[1] == off; r.Clean() != want {
				t.Fatalf("Clean() = %v, want %v", r.Clean(), want)
			}
		})
	}
}

// regression: #5892
func TestAttributionOnWarns(t *testing.T) {
	home := t.TempDir()
	path := writeCLIConfig(t, home, `{"attribution":{"attributeCommitsToAgent":true,"attributePRsToAgent":false}}`)
	var out, errb bytes.Buffer
	CheckAttributionAt(path).Write(&out, &errb)
	if out.Len() != 0 {
		t.Fatalf("an on setting printed to stdout: %q", out.String())
	}
	e := errb.String()
	for _, want := range []string{
		"WARNING: Cursor CLI attribution: " + AttrCommitsKey + " is on (set to true)",
		path,
		`"attributeCommitsToAgent": false`,
		`"attributePRsToAgent": false`,
		"both must be false",
		"does not edit that file",
	} {
		if !strings.Contains(e, want) {
			t.Errorf("stderr lacks %q:\n%s", want, e)
		}
	}
	if strings.Contains(e, AttrPRsKey+" is on") {
		t.Errorf("an off setting was reported on:\n%s", e)
	}
}

// The clean report is one stdout line and nothing on stderr.
func TestAttributionOffQuiet(t *testing.T) {
	home := t.TempDir()
	path := writeCLIConfig(t, home, `{"attribution":{"attributeCommitsToAgent":false,"attributePRsToAgent":false}}`)
	var out, errb bytes.Buffer
	CheckAttributionAt(path).Write(&out, &errb)
	if errb.Len() != 0 {
		t.Fatalf("clean report wrote stderr: %q", errb.String())
	}
	if !strings.HasPrefix(out.String(), "Cursor CLI attribution: off") || strings.Count(out.String(), "\n") != 1 {
		t.Fatalf("stdout = %q", out.String())
	}
}

// Could-not-check is printed as itself, never as off or clean.
func TestAttributionCNCNotOff(t *testing.T) {
	home := t.TempDir()
	path := writeCLIConfig(t, home, `not json`)
	var out, errb bytes.Buffer
	r := CheckAttributionAt(path)
	r.Write(&out, &errb)
	if r.Clean() || out.Len() != 0 {
		t.Fatalf("could-not-check read as clean: stdout %q", out.String())
	}
	if !strings.Contains(errb.String(), "could-not-check "+AttrCommitsKey) {
		t.Fatalf("stderr = %q", errb.String())
	}
}

// An unknown home is could-not-check, not off.
func TestAttributionNoHome(t *testing.T) {
	saved := HostHome
	t.Cleanup(func() { HostHome = saved })
	HostHome = func() (string, error) { return "", errors.New("no home") }
	r := CheckAttribution()
	if r.Clean() || len(r.Files) != 1 || r.Files[0].Path != "" {
		t.Fatalf("report = %+v", r)
	}
	for _, s := range r.Settings {
		if s.State != AttributionCouldNotCheck {
			t.Fatalf("state = %s, want could-not-check", s.State)
		}
	}
}

// Negative: the check never writes — the user's file is byte-identical
// afterwards and an absent file stays absent.
func TestAttributionNeverWrites(t *testing.T) {
	saved := HostHome
	t.Cleanup(func() { HostHome = saved })

	home := t.TempDir()
	HostHome = func() (string, error) { return home, nil }
	body := "{\n  \"attribution\": {\"attributeCommitsToAgent\": true}\n}\n"
	path := writeCLIConfig(t, home, body)
	var out, errb bytes.Buffer
	CheckAttribution().Write(&out, &errb)
	got, err := os.ReadFile(path)
	if err != nil || string(got) != body {
		t.Fatalf("config changed: %q, %v", got, err)
	}

	empty := t.TempDir()
	HostHome = func() (string, error) { return empty, nil }
	CheckAttribution().Write(&out, &errb)
	if _, err := os.Stat(filepath.Join(empty, ".cursor")); !os.IsNotExist(err) {
		t.Fatalf("check created %s/.cursor: %v", empty, err)
	}
}

const bothOff = `{"attribution":{"attributeCommitsToAgent":false,"attributePRsToAgent":false}}`

// writeAt writes body as cli-config.json under dir and returns its path.
func writeAt(t *testing.T, dir, body string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "cli-config.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// regression: #5892
// An environment override can point the Cursor CLI at a file other than the home default;
// a home file reading false must never make the report clean while an override names a file
// that does not.
func TestAttributionConfigOverride(t *testing.T) {
	on, off, cnc := AttributionOn, AttributionOff, AttributionCouldNotCheck
	cases := []struct {
		name  string
		env   func(home, alt string) map[string]string
		goos  string
		alt   string // body of alt/cli-config.json ("" = absent)
		want  []AttributionState
		files int
	}{
		{"CURSOR_CONFIG_DIR empty dir", func(h, a string) map[string]string { return map[string]string{CursorConfigDirEnv: a} },
			"darwin", "", []AttributionState{on, on}, 2},
		{"CURSOR_CONFIG_DIR both false", func(h, a string) map[string]string { return map[string]string{CursorConfigDirEnv: a} },
			"darwin", bothOff, []AttributionState{off, off}, 2},
		{"CURSOR_CONFIG_DIR commits true", func(h, a string) map[string]string { return map[string]string{CursorConfigDirEnv: a} },
			"windows", `{"attribution":{"attributeCommitsToAgent":true,"attributePRsToAgent":false}}`, []AttributionState{on, off}, 2},
		{"CURSOR_CONFIG_DIR relative", func(h, a string) map[string]string { return map[string]string{CursorConfigDirEnv: "rel/dir"} },
			"darwin", "", []AttributionState{cnc, cnc}, 2},
		{"CURSOR_CONFIG_DIR is the home default", func(h, a string) map[string]string {
			return map[string]string{CursorConfigDirEnv: filepath.Join(h, ".cursor")}
		}, "darwin", "", []AttributionState{off, off}, 1},
		{"XDG_CONFIG_HOME on linux", func(h, a string) map[string]string { return map[string]string{XDGConfigHomeEnv: filepath.Dir(a)} },
			"linux", "", []AttributionState{on, on}, 2},
		{"XDG_CONFIG_HOME on freebsd", func(h, a string) map[string]string { return map[string]string{XDGConfigHomeEnv: filepath.Dir(a)} },
			"freebsd", "", []AttributionState{on, on}, 2},
		{"XDG_CONFIG_HOME ignored on darwin", func(h, a string) map[string]string { return map[string]string{XDGConfigHomeEnv: filepath.Dir(a)} },
			"darwin", "", []AttributionState{off, off}, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := t.TempDir()
			homeCfg := writeCLIConfig(t, home, bothOff)
			// alt is <scratch>/cursor, so it is both a CURSOR_CONFIG_DIR value and the
			// $XDG_CONFIG_HOME/cursor directory for XDG_CONFIG_HOME=<scratch>.
			alt := filepath.Join(t.TempDir(), "cursor")
			if c.alt != "" {
				writeAt(t, alt, c.alt)
			}
			pinHost(t, home, c.env(home, alt), c.goos)
			r := CheckAttribution()
			got := states(r)
			if len(got) != 2 || got[0] != c.want[0] || got[1] != c.want[1] {
				t.Fatalf("states = %v, want %v (%+v)", got, c.want, r)
			}
			if len(r.Files) != c.files {
				t.Fatalf("files = %d, want %d: %+v", len(r.Files), c.files, r.Files)
			}
			if want := c.want[0] == off && c.want[1] == off; r.Clean() != want {
				t.Fatalf("Clean() = %v, want %v", r.Clean(), want)
			}
			var out, errb bytes.Buffer
			r.Write(&out, &errb)
			if r.Clean() {
				if !strings.Contains(out.String(), homeCfg) || (c.files == 2 && !strings.Contains(out.String(), alt)) {
					t.Fatalf("clean line does not name every file read: %q", out.String())
				}
				return
			}
			e := errb.String()
			if !strings.Contains(e, homeCfg) || !strings.Contains(e, "Cursor states no precedence") {
				t.Fatalf("the instruction does not name every file the CLI may read:\n%s", e)
			}
			if c.env(home, alt)[CursorConfigDirEnv] != "rel/dir" && !strings.Contains(e, filepath.Join(alt, "cli-config.json")) {
				t.Fatalf("the warning does not name the override file:\n%s", e)
			}
		})
	}
}
