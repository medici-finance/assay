// Package celllaunch defines the shell-independent handoff between cellctl, a
// runner and a console adapter. It does not implement an OS process supervisor.
package celllaunch

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	LaunchSchema   = "cell-launch-v1"
	SessionSchema  = "cell-session-v1"
	MaxRecordBytes = 128 * 1024
)

var namePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,95}$`)
var fullID = regexp.MustCompile(`^[a-f0-9]{64}$`)

// CredentialRef requests resolution by existing role authority at execution
// time. It cannot carry a token, filename, environment value or command.
type CredentialRef struct {
	Kind     string `json:"kind"` // forge-role or model-provider
	Role     string `json:"role"`
	Provider string `json:"provider"` // github/gitlab, or an existing model catalog profile
}

// LaunchSpec is durable nonsecret launch intent. Args and Env must be prepared
// by the trusted launcher; validation cannot recognise secrets hidden in strings.
// No credential value belongs in this structure, including Args.
type LaunchSpec struct {
	Schema      string            `json:"schema"`
	LaunchID    string            `json:"launch_id"`
	Cell        string            `json:"cell"`
	Role        string            `json:"role"`
	OS          string            `json:"os"`
	Intent      string            `json:"intent"` // host or linux-container
	Cockpit     string            `json:"cockpit"`
	Operation   string            `json:"operation"` // harness, deskd or container-console
	Executable  string            `json:"executable"`
	Args        []string          `json:"args"`
	Env         map[string]string `json:"env"`
	Credentials []CredentialRef   `json:"credentials,omitempty"`
	Cwd         string            `json:"cwd"`
}

// Permit is independently constructed by the launcher/runner's trusted policy,
// never decoded from a LaunchSpec. Exact equality binds a permitted operation to
// executable, arguments, environment and authority, rather than accepting any
// command that happens to be stored in an owner-readable file.
type Permit struct{ approved string }

func NewPermit(expected LaunchSpec) (Permit, error) {
	b, err := json.Marshal(expected)
	if err != nil {
		return Permit{}, errors.New("invalid independently supplied launch permit")
	}
	return Permit{approved: string(b)}, nil
}

func (s LaunchSpec) Validate() error {
	if s.Schema != LaunchSchema || !namePattern.MatchString(s.LaunchID) || !namePattern.MatchString(s.Cell) || !namePattern.MatchString(s.Role) {
		return errors.New("invalid launch schema or identity")
	}
	if !validOS(s.OS) || !validCockpit(s.Cockpit, s.OS) || (s.Intent != "host" && s.Intent != "linux-container") {
		return errors.New("invalid launch platform or intent")
	}
	if (s.Intent == "linux-container" && s.Operation != "container-console") || (s.Intent == "host" && s.Operation != "harness" && s.Operation != "deskd") {
		return errors.New("operation does not match launch intent")
	}
	if !absolute(s.Executable, s.OS) || !absolute(s.Cwd, s.OS) || badControl(s.Executable) || badControl(s.Cwd) {
		return errors.New("launch requires absolute executable and working directory")
	}
	if s.Args == nil || s.Env == nil || len(s.Args) > 256 || len(s.Env) > 64 || len(s.Credentials) > 8 {
		return errors.New("launch exceeds field limits or omits args/env")
	}
	for _, a := range s.Args {
		if len(a) > 16384 || !utf8.ValidString(a) || strings.ContainsRune(a, 0) {
			return errors.New("invalid launch argument")
		}
	}
	seen := map[string]bool{}
	for k, v := range s.Env {
		fold := strings.ToUpper(k)
		if seen[fold] || !publicEnv[fold] || len(v) > 16384 || !utf8.ValidString(v) || strings.ContainsRune(v, 0) {
			return errors.New("invalid or duplicate nonsecret environment entry")
		}
		seen[fold] = true
	}
	refs := map[CredentialRef]bool{}
	for _, r := range s.Credentials {
		valid := r.Kind == "forge-role" && (r.Provider == "github" || r.Provider == "gitlab") || r.Kind == "model-provider" && namePattern.MatchString(r.Provider)
		if !valid || r.Role != s.Role || refs[r] {
			return errors.New("invalid credential authority reference")
		}
		refs[r] = true
	}
	return nil
}

// The names are an explicit nonsecret transport vocabulary, not a secret-name
// denylist. Their VALUES still require an independently built Permit.
var publicEnv = map[string]bool{
	"HOME": true, "USERPROFILE": true, "SYSTEMROOT": true, "WINDIR": true,
	"APPDATA": true, "LOCALAPPDATA": true, "PATH": true, "PATHEXT": true,
	"TEMP": true, "TMP": true, "TMPDIR": true, "LANG": true, "TERM": true,
	"KUBECONFIG": true, "ASSAY_CONFIG_HOME": true, "GH_CONFIG_DIR": true,
	"GIT_CONFIG_GLOBAL": true, "GIT_CONFIG_NOSYSTEM": true, "GIT_TERMINAL_PROMPT": true,
	"CODEX_HOME": true, "CLAUDE_CONFIG_DIR": true, "DESK_LOOP": true, "DESK_SESSION": true,
	"DESK_ROOTS": true, "ASSAY_COCKPIT": true,
}

func (p Permit) Check(s LaunchSpec) error {
	if err := s.Validate(); err != nil {
		return err
	}
	a, err := json.Marshal(s)
	if err != nil || p.approved == "" || string(a) != p.approved {
		return errors.New("launch does not match independently permitted operation")
	}
	return nil
}

func ParseLaunch(data []byte) (LaunchSpec, error) {
	var s LaunchSpec
	if err := decode(data, &s); err != nil {
		return s, err
	}
	return s, s.Validate()
}

func (s LaunchSpec) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	type plain LaunchSpec
	b, err := json.Marshal(plain(s))
	if len(b) > MaxRecordBytes {
		return nil, errors.New("launch exceeds record limit")
	}
	return b, err
}

func validOS(s string) bool { return s == "windows" || s == "darwin" || s == "linux" }
func validCockpit(s, os string) bool {
	return s == "orca" || s == "herdr" || s == "tmux" && os != "windows"
}
func badControl(s string) bool {
	if !utf8.ValidString(s) {
		return true
	}
	for _, r := range s {
		if r < 32 || r == 127 {
			return true
		}
	}
	return false
}

// Only local drive paths are part of the initial Windows host contract. UNC,
// device namespaces and drive-relative paths need their own approved policy.
func absolute(s, os string) bool {
	if os != "windows" {
		return strings.HasPrefix(s, "/") && !strings.HasPrefix(s, "//")
	}
	return len(s) > 3 && ((s[0] >= 'A' && s[0] <= 'Z') || (s[0] >= 'a' && s[0] <= 'z')) && s[1] == ':' && (s[2] == '\\' || s[2] == '/') && !strings.Contains(s[2:], ":")
}

// decode rejects duplicates at every nesting level as well as unknown fields,
// trailing documents and oversized input. Errors omit input values/paths.
func decode(data []byte, dst any) error {
	if len(data) == 0 || len(data) > MaxRecordBytes || !utf8.Valid(data) {
		return errors.New("invalid record size or encoding")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	if err := jsonValue(d, 0); err != nil {
		return errors.New("invalid or duplicate JSON member")
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("trailing JSON data")
	}
	d = json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return errors.New("record does not match schema")
	}
	// encoding/json accepts case aliases and null string values. Require the
	// same typed JSON shape after decoding, while allowing arbitrary whitespace
	// and member order. Optional empty collections must be omitted.
	canonical, err := json.Marshal(dst)
	if err != nil {
		return errors.New("invalid typed record")
	}
	var before, after any
	if json.Unmarshal(data, &before) != nil || json.Unmarshal(canonical, &after) != nil || !reflect.DeepEqual(before, after) {
		return errors.New("noncanonical record member or value type")
	}
	return nil
}

func jsonValue(d *json.Decoder, depth int) error {
	if depth > 16 {
		return errors.New("nesting limit")
	}
	t, err := d.Token()
	if err != nil {
		return err
	}
	switch t {
	case json.Delim('{'):
		seen := map[string]bool{}
		for d.More() {
			k, e := d.Token()
			if e != nil {
				return e
			}
			key, ok := k.(string)
			if !ok || seen[key] {
				return errors.New("duplicate key")
			}
			seen[key] = true
			if e = jsonValue(d, depth+1); e != nil {
				return e
			}
		}
		end, e := d.Token()
		if e != nil || end != json.Delim('}') {
			return errors.New("object end")
		}
	case json.Delim('['):
		for d.More() {
			if e := jsonValue(d, depth+1); e != nil {
				return e
			}
		}
		end, e := d.Token()
		if e != nil || end != json.Delim(']') {
			return errors.New("array end")
		}
	case json.Delim('}'), json.Delim(']'):
		return errors.New("unexpected delimiter")
	}
	return nil
}

// NativePaths refuses execution of a foreign-OS spec. Parsing for planning is
// portable; actual execution must resolve paths using the target OS itself.
func (s LaunchSpec) NativePaths(goos string) error {
	if s.OS != goos || !filepath.IsAbs(s.Executable) || !filepath.IsAbs(s.Cwd) {
		return errors.New("launch platform does not match runner")
	}
	return nil
}
