package perfrec

import (
	"encoding/json"
	"runtime"
	"strings"
)

// ChildContext selects a new agent's context at an existing launcher boundary.
// An explicit resume may preserve only previously mapped opaque identities; work
// and assigned role always belong to this launch. Missing mapping means fresh IDs.
// This function never enables collection, reads operational state or grants authority.
func ChildContext(parent Context, role string, work []WorkRef, resume *Attribution) Context {
	if parent.Profile() == "" {
		return Context{}
	}
	p := parent.Data()
	session, agent := "", ""
	if resume != nil && IDValid(resume.SessionID) && IDValid(resume.AgentID) {
		session, agent = resume.SessionID, resume.AgentID
	} else {
		session, _ = NewID()
		agent, _ = NewID()
	}
	value := func(s string) Value { return Value{Value: s, Present: true} }
	in := ContextInput{Profile: value(string(parent.Profile())), SessionID: value(session), AgentID: value(agent), ParentAgentID: value(p.AgentID), ParentInvocationID: value(parent.InvocationID()), DeskRole: value(p.DeskRole), Role: value(role), WorkPresent: true, LookupEnv: func(k string) (string, bool) {
		if k == EnvVar && parent.Profile() != "" {
			return "1", true
		}
		return "", false
	}}
	for _, w := range work {
		b, err := json.Marshal(w)
		if err == nil {
			in.Work = append(in.Work, string(b))
		}
	}
	return ResolveContext(in)
}

// ContextEnv composes a COPY of an existing child's environment. It forwards
// only selected, validated metadata. It never adds ASSAY_TELEMETRY or touches
// operational DESK_LOOP. A tool child receives this invocation as its parent;
// its own resolver mints a fresh invocation ID.
func ContextEnv(base []string, c Context, toolChild bool) []string {
	names := map[string]bool{"ASSAY_PERF_PROFILE": true, "ASSAY_PERF_SESSION_ID": true, "ASSAY_PERF_AGENT_ID": true, "ASSAY_PERF_PARENT_AGENT_ID": true, "ASSAY_PERF_PARENT_INVOCATION_ID": true, "ASSAY_PERF_ROLE": true, "ASSAY_PERF_WORK": true}
	out := make([]string, 0, len(base)+len(names))
	for _, kv := range base {
		k, _, _ := strings.Cut(kv, "=")
		if runtime.GOOS == "windows" {
			k = strings.ToUpper(k)
		}
		if !names[k] {
			out = append(out, kv)
		}
	}
	if c.Profile() == "" {
		return out
	}
	out = append(out, "ASSAY_PERF_PROFILE="+string(c.Profile()))
	if c.Profile() != ProfileWorkflow {
		return out
	}
	d := c.Data()
	parent := d.ParentInvocationID
	if toolChild {
		parent = c.InvocationID()
	}
	for _, kv := range []struct{ k, v string }{
		{"ASSAY_PERF_SESSION_ID", d.SessionID}, {"ASSAY_PERF_AGENT_ID", d.AgentID},
		{"ASSAY_PERF_PARENT_AGENT_ID", d.ParentAgentID}, {"ASSAY_PERF_PARENT_INVOCATION_ID", parent}, {"ASSAY_PERF_ROLE", d.Role},
	} {
		out = append(out, kv.k+"="+kv.v)
	}
	work, _ := json.Marshal(d.Work)
	if d.Work == nil {
		work = []byte("[]")
	}
	return append(out, "ASSAY_PERF_WORK="+string(work))
}
