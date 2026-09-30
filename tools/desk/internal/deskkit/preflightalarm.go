package deskkit

import (
	"fmt"
	"regexp"
	"strings"
)

// preflightalarm.go — the PUBLISHED shape of deskboot's red-preflight alarm.
//
// deskboot files one issue when the operating-envelope preflight comes back red. That issue
// is an outward write to whatever repository the boot resolves — possibly a public one — so
// what it may carry is decided HERE, in one place, and not by whatever the roster happened to
// print. Two contracts meet in this file:
//
//   - REDACTION. The roster's red output carries each blocking check's DETAIL and
//     REMEDIATION verbatim, and those name local specifics: the ambient login, the
//     credential-helper command line, remote URLs, absolute config paths, probe stderr.
//     None of that is token-shaped, so deskfile's body scan does not catch it. The alarm
//     therefore publishes ONLY the role, the red tally, and each check's NAME and STATE —
//     every one of them drawn from a fixed vocabulary this package owns — and points the
//     reader at re-running preflight locally for the detail. A token the extractor does not
//     recognise is counted, never echoed.
//   - THE BLOCKER-EVIDENCE GATE. The alarm is labelled `help wanted`, which deskfile treats
//     as a blocker claim and refuses (exit 5) unless the body carries an `### Evidence`
//     heading followed by a fenced block. The body composed here always does, with the
//     command that produced the verdict as the fence's first line. deskfile's own tests run
//     this composer through that gate, so the two cannot drift apart silently.

// PreflightAlarmLabel is the escalation label the red-preflight alarm files under.
const PreflightAlarmLabel = "help wanted"

// DedupeRefusalPrefix opens deskfile's title-dedupe refusal for `deskfile new`. It is the
// ONE exit-5 refusal a caller may read as "an equivalent issue is already filed" — every
// other exit 5 (the blocker-evidence gate, the filing budget, an unbound role, a scan) is a
// real refusal. deskfile formats its refusal from this constant, so a caller matching it
// matches the text deskfile actually prints.
const DedupeRefusalPrefix = "refused: likely duplicate of #"

// preflightCheckNames is the closed set of check names the alarm may publish. A name outside
// it is counted as an unrecognised check and never echoed, so a detail string that happens to
// contain `<word>=checked-failed` cannot smuggle text into a public issue.
var preflightCheckNames = map[string]bool{
	CheckColdMint:       true,
	CheckAppScopes:      true,
	CheckWriteTransport: true,
	CheckCommitIdentity: true,
	CheckSiblings:       true,
	CheckAmbientID:      true,
}

// preflightStates is the closed state vocabulary (CheckState.String) the alarm may publish.
var preflightStates = map[string]bool{
	CheckedClean.String():         true,
	CheckedFailed.String():        true,
	CouldNotCheck.String():        true,
	CheckedNotApplicable.String(): true,
}

var (
	preflightTallyRe = regexp.MustCompile(`preflight role=([a-z][a-z0-9-]*) (RED|GREEN) ([0-9]+)/([0-9]+) checked-clean`)
	// A check verdict in SummaryLine is always introduced by the ` · ` separator.
	preflightVerdictRe = regexp.MustCompile(`· ([a-z][a-z0-9-]*)=([a-z-]+)`)
)

// PreflightVerdict is one check's published verdict: its name and state, nothing else.
type PreflightVerdict struct {
	Name  string
	State string
}

// RedactedPreflight is everything the alarm may publish about a red roster run.
type RedactedPreflight struct {
	// Tally is "RED n/m checked-clean", or "" when no summary line was found.
	Tally string
	// Verdicts are the recognised non-clean check verdicts, in the order printed.
	Verdicts []PreflightVerdict
	// Unrecognised counts verdict-shaped tokens whose name or state is outside the closed
	// vocabulary; they are withheld, and the count says so rather than dropping them silently.
	Unrecognised int
}

// RedactPreflightOutput reduces a red `deskroster preflight` output to the closed-vocabulary
// fields the alarm may publish. It never returns any substring of a check's detail or
// remediation.
func RedactPreflightOutput(out string) RedactedPreflight {
	var r RedactedPreflight
	if m := preflightTallyRe.FindStringSubmatch(out); m != nil {
		r.Tally = fmt.Sprintf("%s %s/%s checked-clean", m[2], m[3], m[4])
	}
	seen := map[string]bool{}
	for _, m := range preflightVerdictRe.FindAllStringSubmatch(out, -1) {
		name, state := m[1], m[2]
		if !preflightCheckNames[name] || !preflightStates[state] {
			r.Unrecognised++
			continue
		}
		key := name + "=" + state
		if seen[key] {
			continue
		}
		seen[key] = true
		r.Verdicts = append(r.Verdicts, PreflightVerdict{Name: name, State: state})
	}
	return r
}

// PreflightAlarmTitle is the alarm's issue title: role and calendar day, the pair the per-day
// dedupe is keyed on.
func PreflightAlarmTitle(role, day string) string {
	return fmt.Sprintf("deskboot: red operating-envelope preflight (role %s, %s)", role, day)
}

// PreflightAlarmBody composes the alarm's issue body from the raw roster output. Only the
// redacted fields reach the body; the `### Evidence` fence opens with the command that
// produced the verdict (the boot root is withheld — it is a local path).
func PreflightAlarmBody(role, rosterOutput string) string {
	red := RedactPreflightOutput(rosterOutput)
	var ev strings.Builder
	fmt.Fprintf(&ev, "deskroster preflight --role %s --root <boot root, withheld>\n", role)
	if red.Tally != "" {
		fmt.Fprintf(&ev, "preflight role=%s %s\n", role, red.Tally)
	} else {
		ev.WriteString("(no preflight summary line was recognised in the roster output)\n")
	}
	for _, v := range red.Verdicts {
		fmt.Fprintf(&ev, "%s=%s\n", v.Name, v.State)
	}
	if red.Unrecognised > 0 {
		fmt.Fprintf(&ev, "(%d further verdict token(s) outside the known check vocabulary withheld)\n", red.Unrecognised)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "deskboot's operating-envelope preflight came back RED for role `%s` and the boot stopped "+
		"(could-not-run for the whole pass — nothing was claimed). A red envelope otherwise reaches no one "+
		"but the window it printed to, so deskboot files this one alarm, addressed to the desk, deduped by a "+
		"marker per role per day.\n\n", role)
	b.WriteString("### Evidence\n\n")
	b.WriteString("Check names and states only. Each check's detail and remediation name local specifics " +
		"(identities, credential-helper commands, paths) and are deliberately NOT published here.\n\n")
	b.WriteString("```\n")
	b.WriteString(ev.String())
	b.WriteString("```\n\n")
	fmt.Fprintf(&b, "Re-run `deskroster preflight --role %s --verbose` in the booting checkout for each failing "+
		"check's detail and its remediation. Fix the envelope, then re-run deskboot for role `%s`.\n", role, role)
	return b.String()
}
