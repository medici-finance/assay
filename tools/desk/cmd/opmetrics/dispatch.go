package main

// dispatch.go — the dispatch split and the subagent token/no-op block.
//
// The autonomy ladder's rungs 3 and 4 (statusgen ladder.go, autonomy.go opDayFile)
// read two blocks this collector did not write before: dispatch.{loop_initiated,
// operator_initiated} and token_efficiency.{subagent_tokens,merged_prs,
// noop_dispatches,total_dispatches}. Both are computed here from the same
// transcript pass as the operator block, and both obey the day-file's one rule:
// what leaves this file is a COUNT. No prompt, no report, no agent id, no tool
// input is carried into any emitted structure.
//
// TWO POPULATIONS, STATED. The two blocks count different things, and each block's
// numbers share one population so every ratio a reader forms is over like with like:
//
//	dispatch          main-session (isSidechain != true) assistant tool_use parts named
//	                  Agent (or its older name, Task), timestamped on the day. Each is
//	                  attributed to the origin of the most recent preceding prompt
//	                  record in the same transcript file.
//	token_efficiency  subagent RUNS: sidechain records grouped by (sessionId, agentId),
//	                  whether they sit in <session>/subagents/agent-<id>.jsonl or inline
//	                  in the parent transcript, whose first record is on the day.
//	                  total_dispatches is the run count; subagent_tokens and
//	                  noop_dispatches are over the same runs.
//
// A nested dispatch (a subagent dispatching a subagent) is a run in the second
// population but not a dispatch in the first: it has no operator-or-loop origin of
// its own to attribute.
//
// ORIGIN, NEVER GUESSED. The harness stamps each prompt record with turnOrigin (and,
// on newer records, origin.kind — hyphenated). The mapping is a closed table:
//
//	human                                                  operator
//	scheduled, peer, task_notification, auto_continuation  loop
//	anything else, or absent (sdk, older records)          unattributed
//
// An explicit origin is honoured on a meta prompt record too (the harness marks
// scheduled and peer prompts isMeta). Unattributed dispatches are counted, never
// folded into either side. A day whose
// dispatches are ALL unattributed reports could-not-check with
// dispatch-origin-unrecorded and leaves both sides null: a split over zero
// attributed dispatches does not exist.

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

// NoopClassifierVersion is emitted in every day-file next to the no-op count. Bump it
// on ANY change to noopOpening or to the lines reportOpening skips — a changed ruler
// is a changed series, exactly as ClassifierVersion is for the relay ratio. (/1 had
// not been emitted to any day-file when its rule was anchored, so it was not bumped.)
const NoopClassifierVersion = "opmetrics-noop/1"

// Dispatch origins — internal only, never emitted.
type dispatchOrigin int

const (
	originUnattributed dispatchOrigin = iota
	originOperator
	originLoop
)

// mapOrigin is the closed origin table. Hyphenated origin.kind values are normalised
// to the underscore turnOrigin spelling before lookup.
func mapOrigin(turnOrigin, originKind string) dispatchOrigin {
	v := turnOrigin
	if v == "" {
		v = originKind
	}
	switch strings.ReplaceAll(v, "-", "_") {
	case "human":
		return originOperator
	case "scheduled", "peer", "task_notification", "auto_continuation":
		return originLoop
	}
	return originUnattributed
}

// DispatchTally is the main-session dispatch count on the day, by origin.
type DispatchTally struct {
	Total        int
	Loop         int
	Operator     int
	Unattributed int
}

func (d *DispatchTally) add(o DispatchTally) {
	d.Total += o.Total
	d.Loop += o.Loop
	d.Operator += o.Operator
	d.Unattributed += o.Unattributed
}

// AgentRun is one subagent run's contribution — counts and one bit, no text.
type AgentRun struct {
	Tokens    int
	Noop      bool
	HasReport bool
}

// noopOpening is the closed no-op vocabulary, matched case-insensitively and ANCHORED
// at the start of the report's opening line. A heuristic, and versioned as one
// (NoopClassifierVersion): it counts runs whose report OPENS by saying the work was
// already there or that nothing needed doing, optionally after a short subject
// ("This dispatch was a no-op.", "The work is already implemented."). The same
// phrase later in the report ("the helper already exists so I reused it", a list of
// prior work that was "already merged") is not an opening and does not count.
var noopOpening = regexp.MustCompile(`(?i)^` +
	`(?:(?:this|the) (?:dispatch|task|run|work|change|fix|item|issue|request)(?: (?:is|was))? )?` +
	`(?:already (?:done|merged|delivered|implemented|landed|fixed|resolved|shipped|on main|in place|exists|present)` +
	`|nothing (?:to do|to change|to fix|left to do)` +
	`|no (?:changes?|work|action) (?:is |was )?(?:needed|required|necessary)` +
	`|(?:an? )?no-op)\b`)

// Lines a report may open with that make no statement: a markdown heading, a line
// that is only an emphasised label ("**Result**", "__Summary:__"), or a rule. The
// opening line is the first line after them.
var (
	reportHeadingLine = regexp.MustCompile(`^#{1,6}(?:\s|$)`)
	reportLabelLine   = regexp.MustCompile(`^(?:\*\*|__|\*|_)[^*_.;!?\n]{1,40}(?:\*\*|__|\*|_):?$`)
	reportRuleLine    = regexp.MustCompile(`^(?:[-*_]\s*){3,}$`)
	// An emphasised label that starts the opening line itself ("**Result:** …").
	reportLeadLabel = regexp.MustCompile(`^(?:\*\*|__)[^*_.;!?\n]{1,40}(?:\*\*|__):?\s+`)
)

// reportOpening is the report's first line that makes a statement: leading blank,
// heading, label and rule lines skipped, then a leading emphasised label and any
// list, quote or emphasis markers stripped. "" when the report has no such line.
func reportOpening(report string) string {
	for _, line := range strings.Split(report, "\n") {
		l := strings.TrimSpace(line)
		if l == "" || reportHeadingLine.MatchString(l) || reportLabelLine.MatchString(l) || reportRuleLine.MatchString(l) {
			continue
		}
		l = reportLeadLabel.ReplaceAllString(l, "")
		return strings.TrimLeft(l, "*_>-+` \t")
	}
	return ""
}

// isNoopReport classifies a final report. The text goes in; one bit comes out.
func isNoopReport(report string) bool {
	return noopOpening.MatchString(reportOpening(report))
}

// rawAssistant is the assistant-message subset the dispatch scan reads.
type rawAssistant struct {
	ID      string          `json:"id"`
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
	Usage   *struct {
		Input         int `json:"input_tokens"`
		Output        int `json:"output_tokens"`
		CacheCreation int `json:"cache_creation_input_tokens"`
		CacheRead     int `json:"cache_read_input_tokens"`
	} `json:"usage"`
}

type rawToolPart struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Text  string `json:"text"`
	Input *struct {
		Message string `json:"message"`
	} `json:"input"`
}

// isDispatchTool names the tool_use parts that are a subagent dispatch.
func isDispatchTool(name string) bool { return name == "Agent" || name == "Task" }

// handbackTool is the subagent's explicit final-report tool; its input.message is
// the report when present, else the run's last assistant text is.
const handbackTool = "SubagentHandback"

// agentAcc accumulates one run inside a file scan. Text is held only until the file
// ends, then reduced to the Noop/HasReport bits and dropped.
type agentAcc struct {
	first    time.Time
	usage    map[string]int // message id -> token total (streamed copies share an id)
	anon     int            // usage on records with no message id
	lastText string
	handback string
}

// dispatchScan is the per-file state for the dispatch and run counts.
type dispatchScan struct {
	day, dayEnd time.Time
	current     dispatchOrigin
	tally       DispatchTally
	agents      map[string]*agentAcc
	order       []string
}

func newDispatchScan(dayStart, dayEnd time.Time) *dispatchScan {
	return &dispatchScan{day: dayStart, dayEnd: dayEnd, agents: map[string]*agentAcc{}}
}

func (s *dispatchScan) onDay(ts time.Time) bool { return !ts.Before(s.day) && ts.Before(s.dayEnd) }

// observe folds one parsed, timestamped record into the scan.
func (s *dispatchScan) observe(r rawRecord, ts time.Time) {
	sidechain := r.IsSidechain != nil && *r.IsSidechain
	if sidechain {
		if r.AgentID != "" {
			s.observeRun(r, ts)
		}
		return
	}
	switch r.Type {
	case "user":
		if !isPromptRecord(r) {
			return
		}
		kind := ""
		if r.Origin != nil {
			kind = r.Origin.Kind
		}
		// The harness marks scheduled and peer prompts isMeta, so an explicit
		// origin is honoured on any prompt record. An origin-less meta record is
		// injected context inside a turn and leaves the turn's origin alone; an
		// origin-less typed record starts a turn nobody stamped — unattributed.
		switch {
		case r.TurnOrigin != "" || kind != "":
			s.current = mapOrigin(r.TurnOrigin, kind)
		case r.IsMeta == nil || !*r.IsMeta:
			s.current = originUnattributed
		}
	case "assistant":
		if !s.onDay(ts) {
			return
		}
		var m rawAssistant
		if json.Unmarshal(r.Message, &m) != nil {
			return
		}
		var parts []rawToolPart
		if json.Unmarshal(m.Content, &parts) != nil {
			return
		}
		for _, p := range parts {
			if p.Type != "tool_use" || !isDispatchTool(p.Name) {
				continue
			}
			s.tally.Total++
			switch s.current {
			case originOperator:
				s.tally.Operator++
			case originLoop:
				s.tally.Loop++
			default:
				s.tally.Unattributed++
			}
		}
	}
}

// isPromptRecord is a user record that starts a turn: a message whose content is not
// a tool result. Tool results are the harness answering a tool call mid-turn, and
// must not reset the turn's origin.
func isPromptRecord(r rawRecord) bool {
	if len(r.Message) == 0 {
		return false
	}
	var m rawMessage
	if json.Unmarshal(r.Message, &m) != nil || m.Role != "user" {
		return false
	}
	var s string
	if json.Unmarshal(m.Content, &s) == nil {
		return true
	}
	var parts []rawPart
	if json.Unmarshal(m.Content, &parts) != nil {
		return false
	}
	for _, p := range parts {
		if p.Type == "tool_result" {
			return false
		}
	}
	return true
}

func (s *dispatchScan) observeRun(r rawRecord, ts time.Time) {
	key := r.SessionID + "#" + r.AgentID
	acc := s.agents[key]
	if acc == nil {
		acc = &agentAcc{first: ts, usage: map[string]int{}}
		s.agents[key] = acc
		s.order = append(s.order, key)
	}
	if ts.Before(acc.first) {
		acc.first = ts
	}
	if r.Type != "assistant" {
		return
	}
	var m rawAssistant
	if json.Unmarshal(r.Message, &m) != nil {
		return
	}
	if m.Usage != nil {
		n := m.Usage.Input + m.Usage.Output + m.Usage.CacheCreation + m.Usage.CacheRead
		if m.ID == "" {
			acc.anon += n
		} else if n > acc.usage[m.ID] {
			acc.usage[m.ID] = n
		}
	}
	var parts []rawToolPart
	if json.Unmarshal(m.Content, &parts) != nil {
		return
	}
	var text strings.Builder
	for _, p := range parts {
		switch {
		case p.Type == "tool_use" && p.Name == handbackTool && p.Input != nil && p.Input.Message != "":
			acc.handback = p.Input.Message
		case p.Type == "text" && strings.TrimSpace(p.Text) != "":
			if text.Len() > 0 {
				text.WriteByte('\n')
			}
			text.WriteString(p.Text)
		}
	}
	if text.Len() > 0 {
		acc.lastText = text.String()
	}
}

// runs reduces the file's runs that started on the day to counts and bits. The text
// held in each accumulator is dropped here.
func (s *dispatchScan) runs() []AgentRun {
	var out []AgentRun
	for _, k := range s.order {
		acc := s.agents[k]
		if !s.onDay(acc.first) {
			continue
		}
		run := AgentRun{Tokens: acc.anon}
		for _, n := range acc.usage {
			run.Tokens += n
		}
		report := acc.handback
		if report == "" {
			report = acc.lastText
		}
		if strings.TrimSpace(report) != "" {
			run.HasReport = true
			run.Noop = isNoopReport(report)
		}
		out = append(out, run)
	}
	return out
}

// buildDispatchBlock turns the read into the dispatch block. blind carries the
// operator block's reason when the transcripts could not be read at all.
func buildDispatchBlock(tr TranscriptRead, blind Reason) DispatchBlock {
	if blind != ReasonNone {
		return DispatchBlock{Status: StatusCouldNotCheck, Reason: blind}
	}
	d := tr.Dispatch
	b := DispatchBlock{
		Status:          StatusOK,
		TotalDispatches: iptr(d.Total),
		Unattributed:    iptr(d.Unattributed),
	}
	if d.Total > 0 && d.Loop+d.Operator == 0 {
		// Dispatches happened, but not one carries an origin: the split is blind,
		// not 0/0. Leave both sides null so no reader computes a share from it.
		b.Status, b.Reason = StatusCouldNotCheck, ReasonDispatchOriginUnrecorded
		return b
	}
	b.LoopInitiated = iptr(d.Loop)
	b.OperatorInitiated = iptr(d.Operator)
	return b
}

// buildTokenBlock turns the read plus the gh merged count into the token block.
// merged is nil when the gh export was absent or unreadable; ghReason says which.
func buildTokenBlock(tr TranscriptRead, blind Reason, merged *int, ghReason Reason) TokenEfficiencyBlock {
	b := TokenEfficiencyBlock{NoopClassifier: NoopClassifierVersion}
	if blind != ReasonNone {
		// Blind means every count is null, merged_prs included: the block is one
		// measurement, and a lone gh count beside null spend is not half of one.
		b.Status, b.Reason = StatusCouldNotCheck, blind
		return b
	}
	b.MergedPRs = merged
	tokens, noop, unread := 0, 0, 0
	for _, r := range tr.Agents {
		tokens += r.Tokens
		if !r.HasReport {
			unread++
		}
		if r.Noop {
			noop++
		}
	}
	b.Status = StatusOK
	b.Reason = ghReason
	b.SubagentTokens = iptr(tokens)
	b.NoopDispatches = iptr(noop)
	b.TotalDispatches = iptr(len(tr.Agents))
	b.ReportsUnread = iptr(unread)
	return b
}
