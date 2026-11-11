package cursorbundle

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	if r.Clean() || r.Path != "" {
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
