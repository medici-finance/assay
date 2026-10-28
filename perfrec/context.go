package perfrec

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"regexp"
	"strings"
)

// Profile selects local disclosure; it never authorizes sending or operational effects.
type Profile string

const (
	ProfileTool     Profile = "tool"
	ProfileWorkflow Profile = "workflow"
)

// Coverage is deliberately separate from a numeric zero.
type Coverage string

const (
	CoverageMeasured    Coverage = "measured"
	CoverageMissing     Coverage = "missing"
	CoverageInvalid     Coverage = "invalid"
	CoverageUnsupported Coverage = "unsupported"
	CoverageUnknown     Coverage = "unknown"
	CoverageFailed      Coverage = "failed"
	CoverageEstimated   Coverage = "estimated"
)

type ContextField string

const (
	FieldSession          ContextField = "sessionId"
	FieldAgent            ContextField = "agentId"
	FieldParentAgent      ContextField = "parentAgentId"
	FieldParentInvocation ContextField = "parentInvocationId"
	FieldDeskRole         ContextField = "deskRole"
	FieldRole             ContextField = "role"
	FieldWork             ContextField = "work"
)

var contextFields = []ContextField{FieldSession, FieldAgent, FieldParentAgent, FieldParentInvocation, FieldDeskRole, FieldRole, FieldWork}

// Value distinguishes an absent flag from a present empty flag (explicit clearing).
type Value struct {
	Value   string
	Present bool
}

// ContextInput binds parsed values. LookupEnv reads only the eight documented keys.
type ContextInput struct {
	Profile, SessionID, AgentID, ParentAgentID, ParentInvocationID, DeskRole, Role Value
	Work                                                                           []string
	WorkPresent                                                                    bool
	LookupEnv                                                                      func(string) (string, bool)
}

// WorkRef is a bounded LOCAL workflow locator. It must be aliased before future export.
// Brief uses repo + canonical qualified ID; issue additionally requires forge/instance.
type WorkRef struct {
	Kind      string `json:"kind"`
	Forge     string `json:"forge,omitempty"`
	Instance  string `json:"instance,omitempty"`
	Repo      string `json:"repo"`
	Brief     string `json:"brief,omitempty"`
	BriefUUID string `json:"briefUuid,omitempty"`
	Issue     int64  `json:"issue,omitempty"`
	Primary   bool   `json:"primary,omitempty"`
}

const MaxWork = 8
const MaxWorkBytes = 4096

// Attribution has no native harness identifier, free text, path or credential field.
type Attribution struct {
	InvocationID       string                    `json:"invocationId"`
	SessionID          string                    `json:"sessionId,omitempty"`
	AgentID            string                    `json:"agentId,omitempty"`
	ParentAgentID      string                    `json:"parentAgentId,omitempty"`
	ParentInvocationID string                    `json:"parentInvocationId,omitempty"`
	DeskRole           string                    `json:"deskRole,omitempty"`
	Role               string                    `json:"role,omitempty"`
	Work               []WorkRef                 `json:"work,omitempty"`
	Coverage           map[ContextField]Coverage `json:"coverage"`
}

// Context is a frozen per-invocation value. Getters return defensive copies.
type Context struct {
	profile Profile
	data    Attribution
	invalid bool
}

func (c Context) Profile() Profile     { return c.profile }
func (c Context) InvocationID() string { return c.data.InvocationID }
func (c Context) InvalidProfile() bool { return c.invalid }
func (c Context) Data() Attribution {
	d := c.data
	d.Work = append([]WorkRef(nil), d.Work...)
	d.Coverage = map[ContextField]Coverage{}
	for k, v := range c.data.Coverage {
		d.Coverage[k] = v
	}
	return d
}

// NewID uses 128 random bits, in the recorder's existing lowercase hex format.
func NewID() (string, error) {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		return "", e
	}
	return hex.EncodeToString(b[:]), nil
}
func IDValid(s string) bool { return InstallIDPattern.MatchString(s) }

var repoRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,99}/[a-zA-Z0-9][a-zA-Z0-9_.-]{0,99}$`)
var hostRE = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{0,251}[a-z0-9]$`)
var briefRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}:[a-z][a-z0-9-]{0,39}:[a-z][a-z0-9-]{0,39}:[1-9][0-9]{0,8}$`)
var uuidRE = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func validWork(ws []WorkRef) bool {
	if len(ws) > MaxWork {
		return false
	}
	primary := false
	seen := map[WorkRef]bool{}
	for _, w := range ws {
		key := w
		key.Primary = false
		if !repoRE.MatchString(w.Repo) || strings.Contains(w.Repo, "..") || seen[key] {
			return false
		}
		seen[key] = true
		if w.Primary {
			if primary {
				return false
			}
			primary = true
		}
		switch w.Kind {
		case "brief":
			if !briefRE.MatchString(w.Brief) || w.Issue != 0 || w.Forge != "" || w.Instance != "" || (w.BriefUUID != "" && !uuidRE.MatchString(w.BriefUUID)) {
				return false
			}
		case "issue":
			if (w.Forge != "github" && w.Forge != "gitlab") || !hostRE.MatchString(w.Instance) || strings.Contains(w.Instance, "..") || w.Issue < 1 || w.Issue > 2147483647 || w.Brief != "" || w.BriefUUID != "" {
				return false
			}
		default:
			return false
		}
	}
	return true
}
func strictJSON(b []byte, v any) bool {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		return false
	}
	return d.Decode(new(any)) == io.EOF
}
func roleValid(s string) bool {
	switch s {
	case "coordinate", "implement", "review", "verify", "triage", "investigate", "other":
		return true
	}
	return false
}
func deskRoleValid(s string) bool {
	switch s {
	case "the-desk", "worker-desk", "pr-review-desk", "verify-desk", "intake-desk":
		return true
	}
	return false
}

// ResolveContext never returns raw invalid input or an operational error. A present empty
// value clears inheritance; repeated work flags replace the entire environment array.
func ResolveContext(in ContextInput) Context {
	lookup := in.LookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}
	get := func(v Value, key string) string {
		if v.Present {
			return v.Value
		}
		s, _ := lookup(key)
		return s
	}
	selected := get(in.Profile, "ASSAY_PERF_PROFILE")
	c := Context{}
	if selected == "" || selected == "off" {
		return c
	}
	if selected != "tool" && selected != "workflow" {
		c.invalid = true
		return c
	}
	if enabled, _ := lookup(EnvVar); enabled != "1" {
		return c
	}
	id, e := NewID()
	if e != nil {
		return c
	}
	c.profile = Profile(selected)
	c.data = Attribution{InvocationID: id, Coverage: map[ContextField]Coverage{}}
	for _, f := range contextFields {
		c.data.Coverage[f] = CoverageUnsupported
	}
	if c.profile == ProfileTool {
		return c
	}
	for _, x := range []struct {
		f     ContextField
		v     Value
		env   string
		dst   *string
		valid func(string) bool
	}{
		{FieldSession, in.SessionID, "ASSAY_PERF_SESSION_ID", &c.data.SessionID, IDValid},
		{FieldAgent, in.AgentID, "ASSAY_PERF_AGENT_ID", &c.data.AgentID, IDValid},
		{FieldParentAgent, in.ParentAgentID, "ASSAY_PERF_PARENT_AGENT_ID", &c.data.ParentAgentID, IDValid},
		{FieldParentInvocation, in.ParentInvocationID, "ASSAY_PERF_PARENT_INVOCATION_ID", &c.data.ParentInvocationID, IDValid},
		{FieldDeskRole, in.DeskRole, "DESK_LOOP", &c.data.DeskRole, deskRoleValid},
		{FieldRole, in.Role, "ASSAY_PERF_ROLE", &c.data.Role, roleValid},
	} {
		s := get(x.v, x.env)
		state := CoverageMissing
		if s != "" {
			state = CoverageInvalid
			if x.valid(s) {
				*x.dst = s
				state = CoverageMeasured
			}
		}
		c.data.Coverage[x.f] = state
	}
	c.data.Coverage[FieldWork] = CoverageMissing
	var ws []WorkRef
	ok := true
	present := false
	if in.WorkPresent {
		if len(in.Work) > MaxWork {
			c.data.Coverage[FieldWork] = CoverageInvalid
			return c
		}
		present = len(in.Work) > 0
		total := 0
		for _, raw := range in.Work {
			total += len(raw)
			if raw == "" && len(in.Work) == 1 {
				present = false
				continue
			}
			var w WorkRef
			if len(raw) > MaxWorkBytes || !strictJSON([]byte(raw), &w) {
				ok = false
				break
			}
			ws = append(ws, w)
		}
		if total > MaxWorkBytes {
			ok = false
		}
	} else if raw, _ := lookup("ASSAY_PERF_WORK"); raw != "" {
		present = true
		ok = len(raw) <= MaxWorkBytes && strictJSON([]byte(raw), &ws) && ws != nil
		if ok && len(ws) == 0 {
			present = false
		}
	}
	if present {
		if ok && validWork(ws) {
			c.data.Work = append([]WorkRef(nil), ws...)
			c.data.Coverage[FieldWork] = CoverageMeasured
		} else {
			c.data.Coverage[FieldWork] = CoverageInvalid
		}
	}
	return c
}
func validAttribution(p Profile, d Attribution) bool {
	if !IDValid(d.InvocationID) || len(d.Coverage) != len(contextFields) {
		return false
	}
	for _, f := range contextFields {
		v, ok := d.Coverage[f]
		if !ok {
			return false
		}
		if p == ProfileTool {
			if v != CoverageUnsupported {
				return false
			}
		} else {
			switch v {
			case CoverageMeasured, CoverageMissing, CoverageInvalid, CoverageUnsupported:
			default:
				return false
			}
		}
	}
	if p == ProfileTool {
		return d.SessionID == "" && d.AgentID == "" && d.ParentAgentID == "" && d.ParentInvocationID == "" && d.DeskRole == "" && d.Role == "" && len(d.Work) == 0
	}
	if p != ProfileWorkflow {
		return false
	}
	for _, x := range []struct {
		f     ContextField
		s     string
		valid func(string) bool
	}{{FieldSession, d.SessionID, IDValid}, {FieldAgent, d.AgentID, IDValid}, {FieldParentAgent, d.ParentAgentID, IDValid}, {FieldParentInvocation, d.ParentInvocationID, IDValid}, {FieldDeskRole, d.DeskRole, deskRoleValid}, {FieldRole, d.Role, roleValid}} {
		if x.s != "" {
			if !x.valid(x.s) || d.Coverage[x.f] != CoverageMeasured {
				return false
			}
		} else if d.Coverage[x.f] == CoverageMeasured {
			return false
		}
	}
	return validWork(d.Work) && ((len(d.Work) > 0) == (d.Coverage[FieldWork] == CoverageMeasured))
}
