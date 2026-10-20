package cellop

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// DeskSessionMarkers are the environment variables a desk session carries. A state-changing
// invocation refuses desk-session when any of them is set. That is a mistake guard (an operator
// shell that inherited a cell's environment, a desk that ran the wrong command), never a control:
// a determined process can unset them.
var DeskSessionMarkers = []string{"ASSAY_CELL", "DESK_LOOP", "DESK_SESSION"}

// SetMarkers is the sorted subset of DeskSessionMarkers that lookup reports set (to any value,
// the empty string included).
func SetMarkers(lookup func(string) (string, bool)) []string {
	var set []string
	for _, k := range DeskSessionMarkers {
		if _, ok := lookup(k); ok {
			set = append(set, k)
		}
	}
	sort.Strings(set)
	return set
}

// Session is one interactive apply: who is applying (the invoking cell and verb), what the
// terminal said, and the verb's measure and run hooks. Every field is required except Now.
type Session struct {
	Store Store
	Cell  string // the cell the invocation names; it must be one of the plan's cells
	Verb  string // the invoked verb; the plan must have been made by it

	// Interactive is true only when stdin and stdout are both terminals. The adapter measures it;
	// Apply refuses not-interactive when it is false.
	Interactive bool
	// DeskMarkers are the desk-session markers set in the invoking environment (SetMarkers).
	DeskMarkers []string

	// Confirm shows the plan (and a batch's children) to the operator and returns what the
	// operator typed. Apply compares it with the plan's confirm code.
	Confirm func(p Plan, children []Plan) (string, error)
	// Measure takes the current value of every precondition key of one action. An error is
	// could-not-check: the plan is neither consumed nor run.
	Measure func(a Action) ([]Precondition, error)
	// Run performs one action and returns a detail line for the result.
	Run func(a Action) (string, error)

	Now func() time.Time
}

func (s Session) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Apply applies the stored plan id, in the contract's order:
//
//  1. refuse not-interactive off a terminal, and desk-session when a marker is set;
//  2. load and verify the plan; refuse a plan made by another verb, a plan that does not list the
//     invoking cell (foreign-cell), an expired plan, and a plan already consumed;
//  3. show the plan, its cells and its confirm code, and require the operator to type the code;
//  4. re-measure every precondition, refusing plan-stale (could-not-check if a measurement fails),
//     without consuming;
//  5. consume atomically (exclusive creation of the marker), refusing already-consumed;
//  6. only then run the actions. A failure after the consume leaves the plan consumed.
//
// A batch plan is consumed in step 5 before its first child; each child is then re-measured and
// consumed immediately before its own step, and a child found already consumed (or stale, or past
// the batch's expiry) stops the sequence — never skip-and-continue.
func (s Session) Apply(id string) Result {
	r := NewResult(s.Verb, s.Cell, s.now())
	r.PlanID = optional(id)
	refuse := func(o Outcome, reason, detail string) Result {
		r.Finish(o, reason, detail, s.now())
		return r
	}
	if !s.Interactive {
		return refuse(Refused, ReasonNotInteractive, "applying a plan needs a terminal on stdin and stdout; the --plan form works anywhere")
	}
	if len(s.DeskMarkers) > 0 {
		return refuse(Refused, ReasonDeskSession, "desk-session marker(s) set: "+strings.Join(s.DeskMarkers, ", ")+"; clear them in an operator shell first")
	}
	p, err := s.Store.Load(id)
	if reason, o, failed := loadFailure(err); failed {
		return refuse(o, reason, err.Error())
	}
	var children []Plan
	for _, ref := range p.Children {
		c, err := s.Store.Load(ref.ID)
		if reason, o, failed := loadFailure(err); failed {
			return refuse(o, reason, fmt.Sprintf("child %s: %v", ref.ID, err))
		}
		if c.Digest != ref.Digest || c.IsBatch() {
			return refuse(Refused, ReasonPlanInvalid, fmt.Sprintf("child %s does not match the batch (digest changed, or a nested batch)", ref.ID))
		}
		for _, cell := range c.Cells {
			if !p.HasCell(cell) {
				return refuse(Refused, ReasonPlanInvalid, fmt.Sprintf("child %s affects cell %s, which the batch does not list", ref.ID, cell))
			}
		}
		children = append(children, c)
	}
	if p.Verb != s.Verb {
		return refuse(Refused, ReasonWrongVerb, fmt.Sprintf("plan %s was made by %s, not %s", id, p.Verb, s.Verb))
	}
	if !p.HasCell(s.Cell) {
		return refuse(Refused, ReasonForeignCell, fmt.Sprintf("plan %s does not affect cell %s", id, s.Cell))
	}
	if p.Expired(s.now()) {
		return refuse(Refused, ReasonPlanExpired, "plan expired at "+p.Expires)
	}
	if done, err := s.Store.Consumed(id); err != nil {
		return refuse(CouldNotCheck, ReasonUnmeasured, err.Error())
	} else if done {
		return refuse(Refused, ReasonAlreadyConsumed, "plan "+id+" was already applied")
	}
	r.Actions = notRun(p, children)
	typed, err := s.Confirm(p, children)
	if err != nil || strings.TrimSpace(typed) != p.ConfirmCode {
		return refuse(Refused, ReasonConfirmMismatch, "the confirm code was not typed; nothing was changed")
	}
	// The operator may have waited at the prompt: the expiry binds the moment of apply.
	if p.Expired(s.now()) {
		return refuse(Refused, ReasonPlanExpired, "plan expired at "+p.Expires)
	}
	if !p.IsBatch() {
		if o, reason, detail := s.remeasure(p); o != OK {
			return refuse(o, reason, detail)
		}
	}
	if err := s.Store.Consume(id, s.now()); errors.Is(err, ErrAlreadyConsumed) {
		return refuse(Refused, ReasonAlreadyConsumed, "plan "+id+" was already applied")
	} else if err != nil {
		return refuse(CouldNotCheck, ReasonUnmeasured, "consume: "+err.Error())
	}
	if !p.IsBatch() {
		ran, o, reason, detail := s.run(&r, p, 0)
		if o != OK {
			return refuse(stopOutcome(ran, o), reason, detail)
		}
		r.Finish(OK, "", "", s.now())
		return r
	}
	ran, offset := 0, 0
	for _, c := range children {
		// The batch expiry bounds the start of every step.
		if p.Expired(s.now()) {
			return refuse(stopOutcome(ran, Refused), ReasonPlanExpired, "batch expired at "+p.Expires+" before step "+c.ID)
		}
		if c.Expired(s.now()) {
			return refuse(stopOutcome(ran, Refused), ReasonPlanExpired, "step "+c.ID+" expired at "+c.Expires)
		}
		if o, reason, detail := s.remeasure(c); o != OK {
			return refuse(stopOutcome(ran, o), reason, "step "+c.ID+": "+detail)
		}
		if err := s.Store.Consume(c.ID, s.now()); errors.Is(err, ErrAlreadyConsumed) {
			return refuse(stopOutcome(ran, Refused), ReasonAlreadyConsumed, "step "+c.ID+" was already consumed; the sequence stops")
		} else if err != nil {
			return refuse(stopOutcome(ran, CouldNotCheck), ReasonUnmeasured, "consume step "+c.ID+": "+err.Error())
		}
		n, o, reason, detail := s.run(&r, c, offset)
		ran += n
		offset += len(c.Actions)
		if o != OK {
			return refuse(stopOutcome(ran, o), reason, "step "+c.ID+": "+detail)
		}
	}
	r.Finish(OK, "", "", s.now())
	return r
}

// loadFailure classifies a Load error: a missing or invalid plan is a refusal, anything else
// (an unreadable store) is could-not-check.
func loadFailure(err error) (reason string, o Outcome, failed bool) {
	switch {
	case err == nil:
		return "", OK, false
	case errors.Is(err, ErrNotFound):
		return ReasonPlanNotFound, Refused, true
	case errors.Is(err, ErrInvalid):
		return ReasonPlanInvalid, Refused, true
	}
	return ReasonUnmeasured, CouldNotCheck, true
}

// stopOutcome is the outcome of a sequence stopped after ran actions completed: partial once
// anything ran, otherwise the stop's own outcome.
func stopOutcome(ran int, o Outcome) Outcome {
	if ran > 0 {
		return Partial
	}
	return o
}

// notRun lists every planned action, not run yet, in the order Apply would run them.
func notRun(p Plan, children []Plan) []ActionResult {
	out := []ActionResult{}
	add := func(actions []Action) {
		for _, a := range actions {
			out = append(out, ActionResult{ID: a.ID, Cell: a.Cell, Outcome: ActionNotRun})
		}
	}
	add(p.Actions)
	for _, c := range children {
		add(c.Actions)
	}
	return out
}

// remeasure takes every precondition of every action again and compares it with the plan's
// evidence: OK, plan-stale on any difference, could-not-check when a measurement fails.
func (s Session) remeasure(p Plan) (Outcome, string, string) {
	for _, a := range p.Actions {
		now, err := s.Measure(a)
		if err != nil {
			return CouldNotCheck, ReasonUnmeasured, fmt.Sprintf("action %s: %v", a.ID, err)
		}
		got := map[string]string{}
		for _, pc := range now {
			got[pc.Key] = pc.Value
		}
		for _, pc := range a.Preconditions {
			v, ok := got[pc.Key]
			if !ok {
				return CouldNotCheck, ReasonUnmeasured, fmt.Sprintf("action %s: %s was not measured", a.ID, pc.Key)
			}
			if v != pc.Value {
				return Refused, ReasonPlanStale, fmt.Sprintf("action %s: %s is now %q, planned %q", a.ID, pc.Key, v, pc.Value)
			}
		}
	}
	return OK, "", ""
}

// run performs p's actions in order, recording each in r.Actions from offset. It stops at the
// first failure and returns how many actions completed.
func (s Session) run(r *Result, p Plan, offset int) (int, Outcome, string, string) {
	for i, a := range p.Actions {
		detail, err := s.Run(a)
		res := &r.Actions[offset+i]
		if err != nil {
			res.Outcome, res.Detail = ActionFailed, optional(err.Error())
			return i, Failed, ReasonActionFailed, fmt.Sprintf("action %s: %v", a.ID, err)
		}
		res.Outcome, res.Detail = ActionOK, optional(detail)
	}
	return len(p.Actions), OK, "", ""
}

// ReadConfirm writes the plan's rendering and a prompt to out and reads one line from in: what
// the operator typed. It reads byte by byte, so nothing after the line is consumed.
func ReadConfirm(in io.Reader, out io.Writer, rendering string) (string, error) {
	fmt.Fprint(out, rendering)
	fmt.Fprint(out, "This confirm is a guard against mistakes and races, not an authorization control.\n")
	fmt.Fprint(out, "Type the confirm code to apply: ")
	var line []byte
	var b [1]byte
	for {
		n, err := in.Read(b[:])
		if n == 1 {
			if b[0] == '\n' {
				return strings.TrimRight(string(line), "\r"), nil
			}
			line = append(line, b[0])
			if len(line) > 256 {
				return "", errors.New("confirm line too long")
			}
		}
		if err != nil {
			if err == io.EOF && len(line) > 0 {
				return string(line), nil
			}
			return "", err
		}
	}
}
