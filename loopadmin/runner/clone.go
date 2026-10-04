package runner

import "encoding/json"

// Deep copies across the adapter boundary. A request, an observation or a
// result shares slices, maps and pointers with whoever built it. The Client
// keeps its own copy of everything it records and hands out copies of
// everything it returns, so neither the caller nor the adapter can change a
// recorded request, observation or accepted result after the fact. A copy keeps
// a nil slice or map nil and an empty one empty, so a repeated report or
// request still compares equal to the recorded one.

func cloneSlice[T any](s []T) []T {
	if s == nil {
		return nil
	}
	out := make([]T, len(s))
	copy(out, s)
	return out
}

func clonePtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func (r LaunchRequest) clone() LaunchRequest {
	r.Desk = clonePtr(r.Desk)
	r.Work = clonePtr(r.Work)
	r.Resume = clonePtr(r.Resume)
	r.Profile.Tools = cloneSlice(r.Profile.Tools)
	r.Require = cloneSlice(r.Require)
	if r.Extensions != nil {
		ext := make(map[string]json.RawMessage, len(r.Extensions))
		for k, v := range r.Extensions {
			ext[k] = cloneSlice(v)
		}
		r.Extensions = ext
	}
	return r
}

func (r Result) clone() Result {
	r.Artifacts = cloneSlice(r.Artifacts)
	r.ToolRequests = cloneSlice(r.ToolRequests)
	return r
}

func (u Usage) clone() Usage {
	return Usage{InputTokens: clonePtr(u.InputTokens), OutputTokens: clonePtr(u.OutputTokens), CostMicros: clonePtr(u.CostMicros)}
}

func (o Observation) clone() Observation {
	o.Usage = o.Usage.clone()
	if o.Result != nil {
		res := o.Result.clone()
		o.Result = &res
	}
	return o
}

func (c Capabilities) clone() Capabilities {
	c.BudgetScopes = cloneSlice(c.BudgetScopes)
	c.Extensions = cloneSlice(c.Extensions)
	return c
}
