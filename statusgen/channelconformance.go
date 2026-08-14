package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Channel-conformance sweep (distribution/05, Task 4) — an ADVISORY `--lint`
// NOTICE that flags any adopter-facing surface still teaching a non-sanctioned
// acquisition channel. The sanctioned set it judges against is declared once,
// in channels.go.
//
// THREE-STATE INSTRUMENT (desk-hardening/01). Every path in the declared scope
// lands in exactly one of:
//
//	checked-clean    — read, matched nothing (or matched only a line that marks
//	                   the channel retired)
//	checked-failed   — read, matched: a finding is emitted with path:line
//	could-not-check  — could not read, could not decode as text, or could not
//	                   parse the accepted-drift registry that would have
//	                   classified it
//
// A could-not-check NEVER reports as clean and never disappears into a
// summary. The failure mode this exists to prevent is a sweep that cannot open
// half its corpus and prints a green line anyway.
//
// NO SILENT CAPS. The scan scope is a DECLARED, bounded list — not the whole
// tree — and every NOTICE says so, names the size of the scope, and names what
// is outside it. The excluded corpus (brief files, triage records, research
// notes) quotes the retired channels verbatim BY DESIGN, as the defect being
// described; scanning it would produce ~180 findings that are all correct
// prose, and the check would be turned off within a week.

// channelScanScope is the declared set of adopter-facing surfaces, relative to
// the lint root. These are the files an adopter is actually told to follow.
//
// A path here that does not exist is reported as `absent`, not as clean and
// not as could-not-check: an adopter repo legitimately has no tools/desk/README.md.
var channelScanScope = []string{
	"README.md",
	"docs/adopting-assay.md",
	"docs/distribution.md",
	"statusgen/README.md",
	"tools/README.md",
	"tools/desk/README.md",
}

// acceptedDriftRelPath is where the known-accepted register lives, relative to
// the lint root. Absent is a legitimate state (it is a do-not-copy stream doc,
// so it does not exist in the published tree) and is reported explicitly
// rather than assumed empty.
const acceptedDriftRelPath = "docs/streams/distribution/accepted-channel-drift.yml"

// channelFinding is one checked-failed result.
type channelFinding struct {
	Path    string
	Line    int
	Pattern channelPattern
	Text    string
}

// channelBlind is one could-not-check result. Reason is always populated —
// "could not check" with no stated cause is not materially better than silence.
type channelBlind struct {
	Path   string
	Reason string
}

// acceptedDrift is one KNOWN, ACCEPTED violation: a channel deviation that has
// already been decided, with an owner and a tracking reference. It is reported
// distinctly from an unknown finding for one reason — an instrument that
// re-reports a decision already taken is noise, and noise is how a real new
// finding gets skimmed past. Complete is false when the entry is missing a
// field that makes it auditable, in which case it is downgraded to
// could-not-check rather than silently trusted.
type acceptedDrift struct {
	ID       string `yaml:"id"`
	Scope    string `yaml:"scope"`
	Where    string `yaml:"where"`
	Channel  string `yaml:"channel"`
	Why      string `yaml:"why"`
	Tracking string `yaml:"tracking"`
	ReviewBy string `yaml:"review-by"`
}

type acceptedDriftFile struct {
	Accepted []acceptedDrift `yaml:"accepted"`
}

// channelSweep is the whole result. Counts are kept separately from the slices
// so a summary can never claim a coverage it did not achieve.
type channelSweep struct {
	Clean    []string
	Absent   []string
	Findings []channelFinding
	Blind    []channelBlind
	Accepted []acceptedDrift
}

// runChannelConformance executes the sweep over the declared scope.
func runChannelConformance(root string) channelSweep {
	var sw channelSweep

	for _, rel := range channelScanScope {
		path := filepath.Join(root, filepath.FromSlash(rel))
		st, err := os.Stat(path)
		switch {
		case os.IsNotExist(err):
			sw.Absent = append(sw.Absent, rel)
			continue
		case err != nil:
			sw.Blind = append(sw.Blind, channelBlind{rel, fmt.Sprintf("stat failed: %v", err)})
			continue
		case st.IsDir():
			sw.Blind = append(sw.Blind, channelBlind{rel, "is a directory, not a readable document"})
			continue
		}

		f, err := os.Open(path)
		if err != nil {
			sw.Blind = append(sw.Blind, channelBlind{rel, fmt.Sprintf("open failed: %v", err)})
			continue
		}
		found, scanErr := scanChannelDrift(rel, f)
		f.Close()
		if scanErr != nil {
			// A partial read is NOT a partial pass. Whatever matched before the
			// error is still reported (it is real), and the file is ALSO
			// recorded blind, because the unread remainder was never judged.
			sw.Findings = append(sw.Findings, found...)
			sw.Blind = append(sw.Blind, channelBlind{rel, fmt.Sprintf("read failed after %d line(s): %v", len(found), scanErr)})
			continue
		}
		if len(found) == 0 {
			sw.Clean = append(sw.Clean, rel)
			continue
		}
		sw.Findings = append(sw.Findings, found...)
	}

	accepted, blind := loadAcceptedDrift(root)
	sw.Accepted = accepted
	sw.Blind = append(sw.Blind, blind...)

	return sw
}

// scanChannelDrift reads r line by line and returns every checked-failed match.
// A line carrying a retired/not-sanctioned marker is clean: it is describing
// the channel, not teaching it (see retiredMarkerRe).
func scanChannelDrift(rel string, r *os.File) ([]channelFinding, error) {
	var out []channelFinding
	sc := bufio.NewScanner(r)
	// Adopter docs contain long single-line tables; the default 64KiB token
	// limit would abort the scan mid-file. A larger buffer means a real read
	// error is a real read error, not a line-length artefact — and if the
	// limit IS hit, bufio returns an error, which becomes could-not-check
	// rather than a short clean pass.
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if retiredMarkerRe.MatchString(line) {
			continue
		}
		for _, p := range channelDriftPatterns {
			if p.Re.MatchString(line) {
				out = append(out, channelFinding{Path: rel, Line: n, Pattern: p, Text: strings.TrimSpace(line)})
			}
		}
	}
	return out, sc.Err()
}

// loadAcceptedDrift reads the known-accepted register.
//
// Three states, again: absent → no exceptions, reported as such; unreadable or
// unparseable → could-not-check on the WHOLE exception layer, and no
// suppression is applied anywhere (an unreadable allowlist must never be read
// as an empty one, and must never quietly forgive a finding); parsed → the
// entries, with incomplete ones downgraded to could-not-check.
func loadAcceptedDrift(root string) ([]acceptedDrift, []channelBlind) {
	path := filepath.Join(root, filepath.FromSlash(acceptedDriftRelPath))
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, []channelBlind{{acceptedDriftRelPath, fmt.Sprintf("accepted-drift register unreadable: %v — no entry was treated as accepted", err)}}
	}
	var file acceptedDriftFile
	if err := yaml.Unmarshal(raw, &file); err != nil {
		return nil, []channelBlind{{acceptedDriftRelPath, fmt.Sprintf("accepted-drift register does not parse: %v — no entry was treated as accepted", err)}}
	}
	var ok []acceptedDrift
	var blind []channelBlind
	for i, a := range file.Accepted {
		var missing []string
		if strings.TrimSpace(a.ID) == "" {
			missing = append(missing, "id")
		}
		if strings.TrimSpace(a.Why) == "" {
			missing = append(missing, "why")
		}
		if strings.TrimSpace(a.Tracking) == "" {
			missing = append(missing, "tracking")
		}
		if strings.TrimSpace(a.Where) == "" {
			missing = append(missing, "where")
		}
		if len(missing) > 0 {
			label := a.ID
			if label == "" {
				label = fmt.Sprintf("entry #%d", i+1)
			}
			blind = append(blind, channelBlind{
				acceptedDriftRelPath,
				fmt.Sprintf("accepted-drift entry %s is missing %s — recorded but not auditable, so it is NOT counted as an accepted exception", label, strings.Join(missing, "+")),
			})
			continue
		}
		ok = append(ok, a)
	}
	return ok, blind
}

// channelConformanceNotices renders the sweep as advisory --lint NOTICE lines.
//
// The phrase "non-sanctioned channel" appears ONLY on a checked-failed finding
// line, never in the summary or the scope line. That is load-bearing: the
// brief's mutation test counts occurrences of that phrase before and after
// reintroducing a violation, and a summary line carrying it would make the
// clean baseline read as 1 instead of 0.
func channelConformanceNotices(root string) []string {
	sw := runChannelConformance(root)
	var out []string

	for _, f := range sw.Findings {
		ch, ok := channelByID(f.Pattern.Channel)
		why := "channel " + f.Pattern.Channel + " is not in the declared set (statusgen/channels.go) — this is a code bug in the pattern table"
		if ok {
			why = fmt.Sprintf("channel %s (%s) — %s", ch.ID, ch.Name, ch.Why)
		}
		out = append(out, fmt.Sprintf(
			"channel-conformance: %s:%d teaches a non-sanctioned channel [%s]: %s. Sanctioned today: %s. Declared source: statusgen/channels.go; pin spec: %s",
			f.Path, f.Line, f.Pattern.ID, why, sanctionedIDs(), pinSpecRef))
	}

	for _, b := range sw.Blind {
		out = append(out, fmt.Sprintf(
			"channel-conformance COULD-NOT-CHECK: %s — %s. This path was NOT judged clean; the sweep's coverage is %d of %d in-scope surface(s)",
			b.Path, b.Reason, len(sw.Clean)+len(sw.Findings), len(channelScanScope)-len(sw.Absent)))
	}

	for _, a := range sw.Accepted {
		out = append(out, fmt.Sprintf(
			"channel-conformance KNOWN-ACCEPTED [%s]: %s (%s) sits on channel %s — %s. Tracking: %s; review-by: %s. Recorded as an already-taken decision, not a new finding",
			a.ID, a.Where, a.Scope, a.Channel, strings.TrimRight(strings.TrimSpace(a.Why), "."), a.Tracking, orDash(a.ReviewBy)))
	}

	// The summary is emitted UNCONDITIONALLY, including on a fully clean run.
	// A check that prints nothing when it passes is indistinguishable from a
	// check that did not run.
	out = append(out, fmt.Sprintf(
		"channel-conformance summary: %d clean, %d off-channel instruction(s), %d could-not-check, %d absent, %d known-accepted, out of a DECLARED scope of %d adopter-facing surface(s) [%s]. "+
			"Scope is bounded and this is the bound: brief files, triage records and research notes under docs/streams/ and docs/triage/ are NOT scanned — they quote the retired channels verbatim as the defect they describe",
		len(sw.Clean), len(sw.Findings), len(sw.Blind), len(sw.Absent), len(sw.Accepted),
		len(channelScanScope), strings.Join(channelScanScope, ", ")))

	sort.Strings(out)
	return out
}

// sanctionedIDs renders the currently-sanctioned channel letters, derived from
// the declared set rather than written out.
func sanctionedIDs() string {
	var ids []string
	for _, c := range sanctionedChannelSet {
		if c.Sanctioned {
			ids = append(ids, c.ID)
		}
	}
	if len(ids) == 0 {
		return "(none)"
	}
	return strings.Join(ids, ", ")
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}
