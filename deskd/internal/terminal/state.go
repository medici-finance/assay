package terminal

import (
	"bytes"
	"regexp"
	"strings"
	"time"
)

// AgentState is the derived state a terminal read reports for a desk. It is
// derived from TWO independent signals — the OSC title the desk skill sets and
// the PTY's own prompt/idle detection — and is [StateCouldNotCheck] when they
// disagree, never the optimistic reading (spec §5.1, design §1.2).
type AgentState string

const (
	// StateLive: attached, output within the idle window, no contrary signal.
	StateLive AgentState = "live"
	// StateBusy: title says working, or output is flowing.
	StateBusy AgentState = "busy"
	// StateWaiting: a prompt is detected AND the title or last line asks a
	// human (the desk skill's ?/needs-decision title convention).
	StateWaiting AgentState = "waiting"
	// StateIdle: a prompt is detected and nothing is being asked.
	StateIdle AgentState = "idle"
	// StateCouldNotCheck: not attached, the driver is unreachable, or the two
	// signals disagree. Never rounded up to an optimistic state.
	StateCouldNotCheck AgentState = "could-not-check"
)

// TitleSignal is what the OSC title the desk skill set says the desk is doing.
type TitleSignal int

const (
	// TitleUnknown: no title has been seen, or it did not classify.
	TitleUnknown TitleSignal = iota
	// TitleWorking: the title says the desk is actively working.
	TitleWorking
	// TitleAsking: the title carries the ?/needs-decision convention — a human
	// is being asked for something.
	TitleAsking
	// TitleIdle: the title says the desk is idle/done.
	TitleIdle
)

// PromptSignal is what the PTY byte stream shows about whether the desk is at a
// settled prompt or actively producing output.
type PromptSignal int

const (
	// PromptUnknown: neither a settled prompt nor flowing output was observed.
	PromptUnknown PromptSignal = iota
	// PromptDetected: output has been silent for the idle window AND the tail
	// ends at a shell prompt (spike ANSWER 2's silence-plus-regex fallback).
	PromptDetected
	// OutputFlowing: output arrived within the idle window — the desk is
	// producing bytes right now.
	OutputFlowing
)

// DefaultIdleWindow is the output-silence threshold above which a trailing
// prompt counts as a settled prompt rather than a momentary pause. Spike
// ANSWER 2 measured ~200 ms of silence plus the trailing-prompt regex as the
// heuristic that achieved 0 interleavings / 0.0% false positives against a
// stock bash that emits no OSC 133 marks.
const DefaultIdleWindow = 200 * time.Millisecond

// trailingPrompt is the spike ANSWER 2 regex: a shell prompt sigil ($ / # / >)
// at the very end of the stream, allowing trailing whitespace.
var trailingPrompt = regexp.MustCompile(`[\$#>]\s*$`)

// deriveState is the two-signal state machine. It is a pure function so the
// invariant it encodes — "when the signals disagree, could-not-check, never the
// optimistic one" — is unit-testable without a PTY.
//
// The disagreement rule fires FIRST: a title that asserts activity (working)
// while the PTY shows a settled prompt, or a title that asserts settledness
// (idle/asking) while output is flowing, is two instruments telling different
// stories, and the honest answer is could-not-check rather than believing the
// cheerier one.
func deriveState(title TitleSignal, prompt PromptSignal, lastLineAsks bool) AgentState {
	titleActive := title == TitleWorking
	titleSettled := title == TitleIdle || title == TitleAsking
	ptyActive := prompt == OutputFlowing
	ptySettled := prompt == PromptDetected

	if (titleActive && ptySettled) || (titleSettled && ptyActive) {
		return StateCouldNotCheck
	}

	switch prompt {
	case PromptDetected:
		if title == TitleAsking || lastLineAsks {
			return StateWaiting
		}
		return StateIdle
	case OutputFlowing:
		if title == TitleWorking {
			return StateBusy
		}
		return StateLive
	default: // PromptUnknown — lean on the title alone
		switch title {
		case TitleWorking:
			return StateBusy
		case TitleAsking:
			return StateWaiting
		case TitleIdle:
			return StateIdle
		default:
			return StateCouldNotCheck
		}
	}
}

// oscTitle is ESC ] 0 ; <text> BEL  or  ESC ] 2 ; <text> (BEL | ESC \). It
// captures the last title in a byte stream — the desk skill sets the window
// title to advertise identity and status.
var oscTitle = regexp.MustCompile("\x1b\\][02];([^\x07\x1b]*)(?:\x07|\x1b\\\\)")

// parseOSCTitle returns the LAST OSC 0/2 title in b, and whether one was found.
func parseOSCTitle(b []byte) (string, bool) {
	m := oscTitle.FindAllSubmatch(b, -1)
	if len(m) == 0 {
		return "", false
	}
	return string(m[len(m)-1][1]), true
}

// classifyTitle maps a raw OSC title to a [TitleSignal] using the desk skill's
// title conventions: a `?` or `needs-decision` marker means a human is being
// asked; `working`/`running`/`busy` means active; `idle`/`done`/`ready` means
// settled. An unrecognised title is TitleUnknown, which lets the PTY signal
// decide.
func classifyTitle(title string) TitleSignal {
	t := strings.ToLower(title)
	switch {
	case strings.Contains(t, "needs-decision") || strings.Contains(t, "?"):
		return TitleAsking
	case strings.Contains(t, "working") || strings.Contains(t, "running") || strings.Contains(t, "busy"):
		return TitleWorking
	case strings.Contains(t, "idle") || strings.Contains(t, "done") || strings.Contains(t, "ready") || strings.Contains(t, "waiting for input"):
		return TitleIdle
	default:
		return TitleUnknown
	}
}

// detectPrompt is the spike ANSWER 2 heuristic as a pure function: within the
// idle window, output is flowing; past it, a trailing shell-prompt regex means
// a settled prompt, and anything else is unknown. OSC 133 prompt marks are
// preferred when a shell emits them (the send route wires that), but a stock shell
// emits none, so silence-plus-regex is the shipped fallback.
func detectPrompt(tail []byte, silence, idleWindow time.Duration) PromptSignal {
	if silence < idleWindow {
		return OutputFlowing
	}
	if trailingPrompt.Match(bytes.TrimRight(tail, " \t")) {
		return PromptDetected
	}
	return PromptUnknown
}

// lastLineAsks reports whether the last non-empty line of the tail is a
// question — the fallback for [StateWaiting] when the title did not carry the
// asking convention.
func lastLineAsks(tail []byte) bool {
	lines := bytes.Split(tail, []byte{'\n'})
	for i := len(lines) - 1; i >= 0; i-- {
		s := strings.TrimRight(string(lines[i]), " \t\r")
		if s == "" {
			continue
		}
		return strings.HasSuffix(s, "?") || strings.Contains(strings.ToLower(s), "needs-decision")
	}
	return false
}
