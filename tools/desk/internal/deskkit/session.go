package deskkit

import (
	"os"
	"strings"
)

// ResolveSessionName is the one place a desk tool turns an explicit --session value and the
// two session environment variables into "this session's name". Order:
//
//	explicit flag → $DESK_SESSION → $CLAUDE_SESSION_ID → ""
//
// The explicit flag wins. An environment variable is inherited from whatever launched the
// process (a dispatched agent inherits its dispatcher's), while a flag is what the caller
// typed for THIS invocation, so a set variable must never silently override it: before
// this helper, deskroster and deskack each resolved the environment first, and
// `deskroster drop --session b` under an inherited DESK_SESSION=a dropped a's entry and
// never touched b.
//
// Each value is whitespace-trimmed; a blank one falls through to the next source. "" means
// nothing resolved — the caller decides whether that is a refusal (deskroster, deskack) or
// a legitimate answer (the principal resolver's unattended case).
func ResolveSessionName(flagValue string) string {
	if s := strings.TrimSpace(flagValue); s != "" {
		return s
	}
	if s := strings.TrimSpace(os.Getenv("DESK_SESSION")); s != "" {
		return s
	}
	return strings.TrimSpace(os.Getenv("CLAUDE_SESSION_ID"))
}
