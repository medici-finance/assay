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
// The setting lives only in the GLOBAL config; Cursor reads nothing but permissions from a
// project's `.cursor/cli.json`, so placing a project file can never turn it off. The global
// file is `<home>/.cursor/cli-config.json` unless an environment override names another:
// `CURSOR_CONFIG_DIR` (a custom directory) or, on Linux/BSD, `XDG_CONFIG_HOME`
// (`$XDG_CONFIG_HOME/cursor/cli-config.json`). Cursor's reference states no precedence between
// them, so the check reads every file the CLI may read and calls a setting off only when it
// is false in all of them. The installer does not own the global file, so this check REPORTS
// and INSTRUCTS — it never edits a file. It is three-state: each setting is off, on, or
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
	"runtime"
	"strings"
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

// AttributionFile is one candidate config file and the verdicts read from it.
type AttributionFile struct {
	// Path is the file read ("" when the home directory is unknown).
	Path     string
	Settings []AttributionSetting
}

// AttributionReport is the check's result for one host.
type AttributionReport struct {
	// Files holds every global config the Cursor CLI may read on this host, in check order.
	Files []AttributionFile
	// Settings is the combined verdict per key: off only when the key is off in every file.
	Settings []AttributionSetting
}

// HostHome resolves the home directory the global Cursor CLI config lives under. It is
// os.UserHomeDir (the user profile directory on Windows), and a variable so a test can point
// it at a scratch directory.
var HostHome = os.UserHomeDir

// HostEnv reads the environment overrides of the config location. It is a variable so a test
// can pin the overrides instead of inheriting the host's.
var HostEnv = os.Getenv

// HostOS is the operating system the XDG_CONFIG_HOME override applies on (Linux/BSD only).
var HostOS = runtime.GOOS

// The environment overrides of the global config location, as Cursor's reference names them.
const (
	CursorConfigDirEnv = "CURSOR_CONFIG_DIR"
	XDGConfigHomeEnv   = "XDG_CONFIG_HOME"
)

// CLIConfigPath is the default global Cursor CLI config under home.
func CLIConfigPath(home string) string { return filepath.Join(home, ".cursor", "cli-config.json") }

// candidate is one file the CLI may read, or the reason it cannot be located.
type candidate struct{ path, unlocatable string }

// cliConfigCandidates lists every global config the Cursor CLI may read: each override that
// is set, then the home default. The home default stays in the list when an override is set,
// because the reference states no precedence, so a file the CLI might read is never dropped.
func cliConfigCandidates(home string, homeErr error) []candidate {
	var out []candidate
	seen := map[string]bool{}
	add := func(c candidate) {
		if c.path != "" {
			if seen[filepath.Clean(c.path)] {
				return
			}
			seen[filepath.Clean(c.path)] = true
		}
		out = append(out, c)
	}
	override := func(env, dir string, tail ...string) {
		switch {
		case dir == "":
		case !filepath.IsAbs(dir):
			add(candidate{unlocatable: env + " is set to a relative path, so the file the CLI reads is unknown"})
		default:
			add(candidate{path: filepath.Join(append([]string{dir}, tail...)...)})
		}
	}
	override(CursorConfigDirEnv, HostEnv(CursorConfigDirEnv), "cli-config.json")
	if xdgOS(HostOS) {
		override(XDGConfigHomeEnv, HostEnv(XDGConfigHomeEnv), "cursor", "cli-config.json")
	}
	if homeErr != nil || home == "" {
		why := "the home directory is unknown"
		if homeErr != nil {
			why += ": " + homeErr.Error()
		}
		add(candidate{unlocatable: why})
	} else {
		add(candidate{path: CLIConfigPath(home)})
	}
	return out
}

// xdgOS reports whether Cursor honours XDG_CONFIG_HOME on goos ("Linux/BSD" in its reference).
func xdgOS(goos string) bool {
	switch goos {
	case "linux", "freebsd", "openbsd", "netbsd", "dragonfly":
		return true
	}
	return false
}

// CheckAttribution reads every global Cursor CLI config the CLI may read on this host and
// reports both settings.
func CheckAttribution() AttributionReport {
	home, err := HostHome()
	var r AttributionReport
	for _, c := range cliConfigCandidates(home, err) {
		if c.unlocatable != "" {
			r.Files = append(r.Files, allSettings("", AttributionCouldNotCheck, c.unlocatable))
			continue
		}
		r.Files = append(r.Files, checkFile(c.path))
	}
	r.Settings = combine(r.Files)
	return r
}

// combine folds per-file verdicts into one per key: off only when off in every file, on when
// on in any file, could-not-check otherwise.
func combine(files []AttributionFile) []AttributionSetting {
	var out []AttributionSetting
	for i, f := range attrFields {
		var on, cnc []string
		for _, file := range files {
			s := file.Settings[i]
			switch s.State {
			case AttributionOn:
				on = append(on, s.Why)
			case AttributionCouldNotCheck:
				cnc = append(cnc, s.Why)
			}
		}
		switch {
		case len(files) == 0:
			out = append(out, AttributionSetting{f.key, AttributionCouldNotCheck, "no config file could be located"})
		case len(files) == 1:
			out = append(out, files[0].Settings[i])
		case len(on) > 0:
			out = append(out, AttributionSetting{f.key, AttributionOn, "on in a file the CLI may read: " + strings.Join(on, "; ")})
		case len(cnc) > 0:
			out = append(out, AttributionSetting{f.key, AttributionCouldNotCheck, strings.Join(cnc, "; ")})
		default:
			out = append(out, AttributionSetting{f.key, AttributionOff, "set to false in every file the CLI may read"})
		}
	}
	return out
}

// CheckAttributionAt is CheckAttribution against one explicit config path.
func CheckAttributionAt(path string) AttributionReport {
	f := checkFile(path)
	return AttributionReport{Files: []AttributionFile{f}, Settings: f.Settings}
}

func checkFile(path string) AttributionFile {
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
	r := AttributionFile{Path: path}
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

func allSettings(path string, state AttributionState, why string) AttributionFile {
	r := AttributionFile{Path: path}
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
// each setting that is on or could not be checked, per file, then the instruction naming every
// file the CLI may read. It never changes an exit code; the caller's own result stands.
func (r AttributionReport) Write(stdout, stderr io.Writer) {
	var paths []string
	for _, f := range r.Files {
		if f.Path != "" {
			paths = append(paths, f.Path)
		}
	}
	if r.Clean() {
		fmt.Fprintf(stdout, "Cursor CLI attribution: off (%s and %s are false in %s)\n", AttrCommitsKey, AttrPRsKey, strings.Join(paths, " and "))
		return
	}
	for _, f := range r.Files {
		where := f.Path
		if where == "" {
			where = "the global Cursor CLI config"
		}
		for _, s := range f.Settings {
			switch s.State {
			case AttributionOn:
				fmt.Fprintf(stderr, "WARNING: Cursor CLI attribution: %s is on (%s) in %s\n", s.Key, s.Why, where)
			case AttributionCouldNotCheck:
				fmt.Fprintf(stderr, "WARNING: Cursor CLI attribution: could-not-check %s (%s) in %s\n", s.Key, s.Why, where)
			}
		}
	}
	target, list := "the global Cursor CLI config", ""
	switch {
	case len(r.Files) > 1 && len(paths) > 0:
		target = "each of these files, since the CLI may read any of them (" + CursorConfigDirEnv + ",\n  " +
			XDGConfigHomeEnv + " on Linux/BSD, or the home default; Cursor states no precedence)"
		list = "    - " + strings.Join(paths, "\n    - ") + "\n  the value:\n"
	case len(paths) == 1:
		target = paths[0]
	}
	fmt.Fprintf(stderr, "  On a host that runs desks, both must be false. With them on, the Cursor CLI adds an\n"+
		"  attribution trailer to every agent commit and a footer to every agent PR, which Assay's\n"+
		"  rules forbid, and the push guard can refuse those commits. Set this in\n"+
		"  %s:\n%s"+
		"    \"attribution\": { \"attributeCommitsToAgent\": false, \"attributePRsToAgent\": false }\n"+
		"  Only the global file holds this setting; a project's .cursor/cli.json cannot. This installer\n"+
		"  does not edit that file.\n", target, list)
}
