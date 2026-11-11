package cursorbundle

// attribution.go — a read-only check of the host's Cursor CLI commit attribution.
//
// The Cursor CLI appends an attribution trailer to every commit an agent makes, and an
// attribution footer to every PR it opens, unless the host's global CLI config sets
// `attribution.attributeCommitsToAgent` and `attribution.attributePRsToAgent` to false. Both
// default to true. Assay's resident rules forbid attribution lines in commits and PRs, so a
// desk worker on a host with the default posture produces commits its own rules reject, and
// the harness re-adds the trailer on every commit, amends included (#5892).
//
// The setting lives only in the GLOBAL config (`<home>/.cursor/cli-config.json`); Cursor reads
// nothing but permissions from a project's `.cursor/cli.json`, so placing a project file can
// never turn it off. The installer does not own the global file, so this check REPORTS and
// INSTRUCTS — it never edits the file. It is three-state: each setting is off, on, or
// could-not-check, and could-not-check is printed as itself, never as off.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// AttributionState is one setting's verdict.
type AttributionState string

const (
	AttributionOff           AttributionState = "off"
	AttributionOn            AttributionState = "on"
	AttributionCouldNotCheck AttributionState = "could-not-check"
)

// The two settings, spelled as Cursor's CLI configuration reference spells them.
const (
	AttrCommitsKey = "attribution.attributeCommitsToAgent"
	AttrPRsKey     = "attribution.attributePRsToAgent"
)

// attrFields maps each reported key to its field name inside the `attribution` object.
var attrFields = []struct{ key, field string }{
	{AttrCommitsKey, "attributeCommitsToAgent"},
	{AttrPRsKey, "attributePRsToAgent"},
}

// AttributionSetting is one setting's verdict and the reason for it.
type AttributionSetting struct {
	Key   string
	State AttributionState
	Why   string
}

// AttributionReport is the check's result for one host.
type AttributionReport struct {
	// Path is the global CLI config the check read ("" when the home directory is unknown).
	Path     string
	Settings []AttributionSetting
}

// HostHome resolves the home directory the global Cursor CLI config lives under. It is
// os.UserHomeDir (the user profile directory on Windows), and a variable so a test can point
// it at a scratch directory.
var HostHome = os.UserHomeDir

// CLIConfigPath is the global Cursor CLI config under home.
func CLIConfigPath(home string) string { return filepath.Join(home, ".cursor", "cli-config.json") }

// CheckAttribution reads the host's global Cursor CLI config and reports both settings.
func CheckAttribution() AttributionReport {
	home, err := HostHome()
	if err != nil || home == "" {
		why := "the home directory is unknown"
		if err != nil {
			why += ": " + err.Error()
		}
		return allSettings("", AttributionCouldNotCheck, why)
	}
	return CheckAttributionAt(CLIConfigPath(home))
}

// CheckAttributionAt is CheckAttribution against an explicit config path.
func CheckAttributionAt(path string) AttributionReport {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return allSettings(path, AttributionOn, "the file is absent and Cursor's default is true")
	}
	if err != nil {
		return allSettings(path, AttributionCouldNotCheck, "cannot read the file: "+err.Error())
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil || top == nil {
		return allSettings(path, AttributionCouldNotCheck, "the file is not a JSON object")
	}
	block, ok := top["attribution"]
	if !ok || isNull(block) {
		return allSettings(path, AttributionOn, "no attribution block is set and Cursor's default is true")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(block, &fields); err != nil || fields == nil {
		return allSettings(path, AttributionCouldNotCheck, "the attribution value is not a JSON object")
	}
	r := AttributionReport{Path: path}
	for _, f := range attrFields {
		v, ok := fields[f.field]
		switch {
		case !ok || isNull(v):
			r.Settings = append(r.Settings, AttributionSetting{f.key, AttributionOn, "unset, and Cursor's default is true"})
		case bytes.Equal(bytes.TrimSpace(v), []byte("false")):
			r.Settings = append(r.Settings, AttributionSetting{f.key, AttributionOff, "set to false"})
		case bytes.Equal(bytes.TrimSpace(v), []byte("true")):
			r.Settings = append(r.Settings, AttributionSetting{f.key, AttributionOn, "set to true"})
		default:
			r.Settings = append(r.Settings, AttributionSetting{f.key, AttributionCouldNotCheck,
				"the value is not a boolean"})
		}
	}
	return r
}

func isNull(v json.RawMessage) bool { return bytes.Equal(bytes.TrimSpace(v), []byte("null")) }

func allSettings(path string, state AttributionState, why string) AttributionReport {
	r := AttributionReport{Path: path}
	for _, f := range attrFields {
		r.Settings = append(r.Settings, AttributionSetting{f.key, state, why})
	}
	return r
}

// Clean reports whether every setting was checked and is off.
func (r AttributionReport) Clean() bool {
	for _, s := range r.Settings {
		if s.State != AttributionOff {
			return false
		}
	}
	return len(r.Settings) > 0
}

// Write prints the report. A clean report is one line on stdout. Anything else goes to stderr:
// each setting that is on or could not be checked, then the instruction. It never changes an
// exit code; the caller's own result stands.
func (r AttributionReport) Write(stdout, stderr io.Writer) {
	where := r.Path
	if where == "" {
		where = "the global Cursor CLI config"
	}
	if r.Clean() {
		fmt.Fprintf(stdout, "Cursor CLI attribution: off (%s and %s are false in %s)\n", AttrCommitsKey, AttrPRsKey, where)
		return
	}
	for _, s := range r.Settings {
		switch s.State {
		case AttributionOn:
			fmt.Fprintf(stderr, "WARNING: Cursor CLI attribution: %s is on (%s) in %s\n", s.Key, s.Why, where)
		case AttributionCouldNotCheck:
			fmt.Fprintf(stderr, "WARNING: Cursor CLI attribution: could-not-check %s (%s) in %s\n", s.Key, s.Why, where)
		}
	}
	fmt.Fprintf(stderr, "  On a host that runs desks, both must be false. With them on, the Cursor CLI adds an\n"+
		"  attribution trailer to every agent commit and a footer to every agent PR, which Assay's\n"+
		"  rules forbid, and the push guard can refuse those commits. Set this in\n"+
		"  %s:\n"+
		"    \"attribution\": { \"attributeCommitsToAgent\": false, \"attributePRsToAgent\": false }\n"+
		"  Only the global file holds this setting; a project's .cursor/cli.json cannot. This installer\n"+
		"  does not edit that file.\n", where)
}
