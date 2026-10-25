package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ToolCaller is the bound read surface the chat dispatches through — deskd's
// MCP server, in production. It is an interface, and a NARROW one, so this
// package's import graph stays small enough to audit (see the package doc's
// fourth control).
type ToolCaller interface {
	// Tools reports what the server advertises. The chat binds a closed subset
	// of it; see [Bind].
	Tools() []ToolDescriptor
	// CallTool dispatches one read and returns its raw JSON result.
	CallTool(ctx context.Context, name string, args []byte) ([]byte, error)
}

// ToolCall is one dispatched (or refused) read, recorded on the answer so the
// transcript shows what was actually asked of the index.
type ToolCall struct {
	Tool string `json:"tool"`
	// Args is the model's requested arguments, recorded verbatim for audit.
	Args json.RawMessage `json:"args,omitempty"`
	// OK is false when the call was refused by the binding or failed.
	OK bool `json:"ok"`
	// Refused states why, when it was. A refusal is RECORDED, never silently
	// dropped: a model reaching for a write tool is a thing an operator should
	// be able to see happened.
	Refused string `json:"refused,omitempty"`
}

// Answer is one chat turn's result.
//
// Note the separation that the whole design rests on. Text is the model's
// words and is never parsed for numbers. Figures come from [ToolCall] results
// through [FigureFromToolResult] and from nowhere else. There is no field, and
// no method, that moves a value from Text into Figures.
type Answer struct {
	// Backend is the model SERVICE that served this answer.
	Backend string `json:"backend"`
	// Model is the ladder rung that served it (design §13.8: the answer
	// records which backend served it).
	Model string `json:"model"`
	// Text is the model's narration. DATA — escaped at render (§6.6).
	Text string `json:"text"`
	// ToolCalls is every read requested this turn, dispatched or refused.
	ToolCalls []ToolCall `json:"tool_calls"`
	// Figures is every cited number, each governed by the imported numbers
	// rule and each carrying the query key it decomposes to.
	Figures []Figure `json:"figures"`
	// Escalated records the ladder rungs tried before this one, when the
	// client escalated.
	Escalated []string `json:"escalated,omitempty"`
}

// Client is the Ask Assay chat client.
type Client struct {
	tools   ToolCaller
	backend Backend
	ladder  Ladder
	now     func() time.Time
	// maxTurns bounds the agent loop. A model that keeps asking for reads
	// forever is a cost and latency fault, not a correctness one, so it is
	// bounded here rather than trusted to stop.
	maxTurns int
}

// Options configure a [Client].
type Options struct {
	Tools   ToolCaller
	Backend Backend
	// Models is the cell's configured rung selection; it is validated against
	// the closed ladder by [NewLadder].
	Models   []string
	Now      func() time.Time
	MaxTurns int
}

// DefaultMaxTurns is the agent loop's bound: one narration turn plus the reads
// it asks for, then a second pass to narrate over the results.
const DefaultMaxTurns = 3

// New builds a client, refusing a model outside the §13.8 ladder and a nil
// tool surface. It does NOT default a backend in: a client with no backend is
// a pane that renders could-not-check, which is the honest OFF state.
func New(o Options) (*Client, error) {
	if o.Tools == nil {
		return nil, errors.New("chat: no tool surface — the chat answers from tool results, so a client with no read " +
			"surface could only answer from model text, which is exactly what this package exists to prevent")
	}
	if o.Backend == nil {
		return nil, errors.New("chat: no model backend configured")
	}
	ladder, err := NewLadder(o.Models)
	if err != nil {
		return nil, err
	}
	now := o.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	turns := o.MaxTurns
	if turns <= 0 {
		turns = DefaultMaxTurns
	}
	return &Client{tools: o.Tools, backend: o.Backend, ladder: ladder, now: now, maxTurns: turns}, nil
}

// Ask answers one question.
//
// The order is load-bearing:
//
//  1. BIND FIRST. If the server's advertised tool set does not satisfy the
//     closed read-only allow-list, nothing runs — no prompt is sent, no data
//     leaves. A binding refusal is a hard error, never a degraded mode.
//  2. Narrate. The model sees the question and the bound tool DESCRIPTIONS.
//  3. Dispatch only bound reads. Every requested call is re-checked against
//     the binding at dispatch time, so a name the model invented cannot be
//     called even if it appeared in a reply.
//  4. Build figures from RESULTS. The model's own text never reaches this step.
func (c *Client) Ask(ctx context.Context, question string) (Answer, error) {
	bound, err := Bind(c.tools.Tools())
	if err != nil {
		return Answer{}, err
	}
	boundSet := map[string]bool{}
	for _, t := range bound {
		boundSet[t.Name] = true
	}

	ans := Answer{Backend: c.backend.Name(), ToolCalls: []ToolCall{}, Figures: []Figure{}}
	prompt := Prompt{Question: question, Tools: bound}

	rungs := c.ladder.Models()
	model := rungs[0]
	rung := 0

	for turn := 0; turn < c.maxTurns; turn++ {
		reply, err := c.backend.Complete(ctx, model, prompt)
		if err != nil {
			return ans, fmt.Errorf("chat: %s/%s did not answer: %w", c.backend.Name(), model, err)
		}
		ans.Model = model
		ans.Text = reply.Text

		// Escalation is the CLIENT's own signal and nothing else. It is read
		// off the typed field, never inferred from the reply's prose.
		if reply.Escalate && rung+1 < len(rungs) {
			ans.Escalated = append(ans.Escalated, model)
			rung++
			model = rungs[rung]
			continue
		}
		if len(reply.ToolCalls) == 0 {
			// No reads requested. Whatever the model said, this answer cites
			// no figures — and the pane will render no digit for it.
			return ans, nil
		}

		var results []ToolResult
		for _, rc := range reply.ToolCalls {
			call := ToolCall{Tool: rc.Tool, Args: json.RawMessage(rc.Args)}
			if !boundSet[rc.Tool] {
				call.Refused = "not on the chat's closed read-only tool binding"
				ans.ToolCalls = append(ans.ToolCalls, call)
				continue
			}
			raw, err := c.tools.CallTool(ctx, rc.Tool, rc.Args)
			if err != nil {
				call.Refused = "the read failed: " + err.Error()
				ans.ToolCalls = append(ans.ToolCalls, call)
				continue
			}
			call.OK = true
			ans.ToolCalls = append(ans.ToolCalls, call)
			results = append(results, ToolResult{Tool: rc.Tool, Raw: raw})
			if fig, ok := FigureFromToolResult(rc.Tool, raw, c.now()); ok {
				ans.Figures = append(ans.Figures, fig)
			}
		}
		prompt.Results = append(prompt.Results, results...)
	}
	return ans, nil
}
