package main

import (
	"fmt"
	"sort"
	"strings"
)

// mainhealth.go — the INJECTED main-health input the critical tier's main-red arm
// reads (drives phase 3, statusgen/05 row 5).
//
// WHY INJECTED. The brief's premise — "statusgen already knows CI-red" — does not
// hold for the board path: statusgen is offline and deterministic, and the only
// red/green read it has is the network-backed DORA timing path. Whether main is
// red is a LIVE forge fact, so statusgen does not read it; the caller that already
// reads the forge supplies it with `--main-health`, and statusgen stays offline.
//
// THREE STATES, never two:
//
//	unset             → could-not-check. The main-red arm cannot fire, and while a
//	                    drive is active the board and the --next-up JSON SAY SO
//	                    (mainRedUnknownText) — never a silent "main is green".
//	green             → checked: main is not red; the arm stays off.
//	red:<ref>[,<ref>] → checked: main is red, tracked by these issues. Each <ref> is a
//	                    FULL `owner/repo#N` (a bare #N is refused: a drive spans
//	                    repos, the same rule the drive manifest applies).
//
// WHAT THE ARM MARKS. A main-red FIX — never "every drive pick" and never the
// whole board. A brief qualifies when it addresses one of the named tracking
// issues: an issue placeholder (placeholder-v1) whose `repo:`/`issue:` IS that issue, or a
// brief-v1/v2 brief whose `issues:` lists it (resolved against the stream's own
// `repo:`; a stream that declares no repo cannot resolve a bare issue number, so
// its briefs never match — the fail-safe direction for a tier above every score).
// WHERE EACH INPUT COMES FROM. The red and its tracking issue come from the
// caller's forge read. The FIX linkage does not: it is the brief's own `issues:`
// list (or a placeholder's own `issue:`), repo text an ordinary PR can write. So a
// brief that adds the tracking issue's number to `issues:` is lifted while main is
// red, gated only by the review and human merge of that PR. That residual is named,
// not derived — see the RESIDUAL note in drivecritical.go.

// MainHealth is the resolved --main-health input for one run. The zero value is
// the could-not-check state.
type MainHealth struct {
	// State is "" (not supplied — could-not-check), "green" or "red".
	State string
	// Refs is the set of full `owner/repo#N` tracking-issue refs when State is red.
	Refs map[string]bool
}

// known reports whether main health was supplied at all.
func (m MainHealth) known() bool { return m.State != "" }

// red reports whether main was supplied as red.
func (m MainHealth) red() bool { return m.State == "red" }

// String renders the state for the --next-up JSON and the board.
func (m MainHealth) String() string {
	switch m.State {
	case "":
		return "could-not-check"
	case "red":
		refs := make([]string, 0, len(m.Refs))
		for r := range m.Refs {
			refs = append(refs, r)
		}
		sort.Strings(refs)
		return "red:" + strings.Join(refs, ",")
	default:
		return m.State
	}
}

// activeMainHealth is the main-health input for the current run. main() wires it
// from --main-health before nextUp; tests set it via withMainHealth. The zero value
// is could-not-check, which leaves the main-red arm off and — only while a drive is
// active — is named on the board.
var activeMainHealth MainHealth

// parseMainHealth parses the --main-health flag value. An empty value is the
// could-not-check state; anything malformed is an error (a usage refusal), never a
// silent fallback to either state.
func parseMainHealth(v string) (MainHealth, error) {
	v = strings.TrimSpace(v)
	switch {
	case v == "":
		return MainHealth{}, nil
	case v == "green":
		return MainHealth{State: "green"}, nil
	case strings.HasPrefix(v, "red:"):
		refs := map[string]bool{}
		for _, r := range strings.FieldsFunc(strings.TrimPrefix(v, "red:"), func(c rune) bool { return c == ',' || c == ' ' }) {
			if !fullIssueRefRe.MatchString(r) {
				return MainHealth{}, fmt.Errorf("--main-health: %q is not a full owner/repo#N issue ref (a bare #N is ambiguous across repos)", r)
			}
			refs[r] = true
		}
		if len(refs) == 0 {
			return MainHealth{}, fmt.Errorf("--main-health: red: needs at least one owner/repo#N tracking issue — a red main with no tracking issue names no fix to rank")
		}
		return MainHealth{State: "red", Refs: refs}, nil
	default:
		return MainHealth{}, fmt.Errorf("--main-health: %q is not one of green | red:<owner/repo#N>[,...] (omit the flag for could-not-check)", v)
	}
}

// mainRedUnknownText is the could-not-check line rendered (board + --lint NOTICE)
// while a drive is active and no main-health input was supplied.
const mainRedUnknownText = "main-red arm could not check — no `--main-health` input was supplied, so whether main is red is unknown here " +
	"(statusgen does not read live CI). A main-red fix cannot be lifted into the critical tier on this run; this is not a reading that main is green."
