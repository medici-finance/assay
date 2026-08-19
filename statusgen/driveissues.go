package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

// driveissues.go — methodology-metrics phase 2 (runsheet 2.4): the drive
// TRACKING ISSUE + AGING operator-act issues + the exactly-one @operator ping.
//
// It follows the --decision-issues architecture EXACTLY (decisionissues.go):
// statusgen computes self-contained GitHub issue payloads and emits them as JSON;
// an outside workflow feeds title/labels/body to `gh issue create`. statusgen
// itself opens nothing, comments on nothing, and reaches no network — so the board
// generator stays deterministic and byte-stable, and a drive can never freeze the
// board. Idempotency is a hidden `<!-- … -->` marker plus a --drive-markers file
// of the markers already present in open issues (loadDriveMarkers), the same
// mechanism as needs-decision.
//
// The one subtlety over --decision-issues is the PING. brief-44: "On entering
// WAITING-ON-OPERATOR: exactly one ping … Re-ping ONLY on a state-change or an age
// threshold (24h → 72h), with a dedup marker in the issue's last bot comment." The
// ping decision (drivePingDecision) is therefore a PURE function of (state, oldest
// operator-act age, the ping markers already posted): it fires iff the drive is
// WAITING-ON-OPERATOR and the (state, age-bucket) marker is not already present.
// A new entry, or a 24h→72h escalation, yields a new marker → one ping; an
// unchanged state+bucket dedups to silence (EEMUA-191 no-per-tick-spam).

const (
	// driveLabelPrefix + slug is the per-drive label (brief-44: "drive:<slug>
	// label"). driveTrackingLabel tags the single tracking issue itself.
	driveLabelPrefix     = "drive:"
	driveTrackingLabel   = "drive-tracking"
	driveOperatorLabel   = "drive-operator-act"
	waitingOnYouTitleTag = "[WAITING ON YOU]"

	// operatorHandle is the ratified push channel (brief-44 decision #2): the drive
	// tracking issue @-mentions @operator — no email / Slack hook.
	operatorHandle = "@operator"

	// F-09 TUNABLE HEURISTICS (aging thresholds, not truths):
	//   driveActAgingThresholdDays — an operator-act at or past this age gets its
	//     OWN aging issue (brief-44 "aging issues for the operator-act rows"); a
	//     brand-new act rides only the tracking issue's WAITING-ON-YOU table.
	//   drivePingEscalateDays — the 24h→72h re-ping step: crossing it re-pings once.
	driveActAgingThresholdDays = 1
	drivePingEscalateDays      = 3
)

// driveIssue is one emitted GitHub issue payload for the drive lifecycle. Kind is
// "tracking" (one per active drive) or "operator-act" (one per aged act). Marker is
// the create-time idempotency key. For a tracking issue that is also pinging,
// Ping is set and PingComment/PingMarker carry the @operator comment and ITS dedup
// key (posted only when PingMarker is absent from the issue's existing comments).
type driveIssue struct {
	Slug        string   `json:"slug"`
	Kind        string   `json:"kind"`
	State       string   `json:"state,omitempty"`
	Title       string   `json:"title"`
	Labels      []string `json:"labels"`
	Marker      string   `json:"marker"`
	Body        string   `json:"body"`
	Ping        bool     `json:"ping,omitempty"`
	PingMarker  string   `json:"pingMarker,omitempty"`
	PingComment string   `json:"pingComment,omitempty"`
}

// driveMarkerRe matches any hidden drive marker (tracking, per-act, or ping) in an
// issue body or comment. Tolerant of raw bodies so a workflow can pipe
// `gh issue list --json body` / `gh issue view --json comments` straight in.
var driveMarkerRe = regexp.MustCompile(`<!-- drive(?:-act|-ping)?: [^>]*? -->`)

func driveTrackingMarker(slug string) string { return "<!-- drive: " + slug + " -->" }
func driveActMarker(slug string, idx int) string {
	return fmt.Sprintf("<!-- drive-act: %s#%d -->", slug, idx)
}
func drivePingMarker(slug, state string, bucket int) string {
	return fmt.Sprintf("<!-- drive-ping: %s %s %d -->", slug, state, bucket)
}

// drivePingBucket maps the oldest operator-act's age to a re-ping bucket:
// 0 = fresh (< threshold), 1 = past the aging threshold (~24h), 2 = past the
// escalation step (~72h). A ping marker embeds the bucket, so a bucket change is a
// re-ping trigger while a steady bucket dedups.
func drivePingBucket(oldestActDays int) int {
	switch {
	case oldestActDays >= drivePingEscalateDays:
		return 2
	case oldestActDays >= driveActAgingThresholdDays:
		return 1
	default:
		return 0
	}
}

// drivePingDecision decides whether to post the @operator ping and returns its dedup
// marker. It fires ONLY in WAITING-ON-OPERATOR (the operator is the bottleneck),
// and only when the (state, age-bucket) marker is not already present — so a fresh
// entry pings once and a 24h→72h escalation re-pings once, but a steady wait is
// silent (dedup). Any other state never pings.
func drivePingDecision(slug, state string, oldestActDays int, existing map[string]bool) (ping bool, marker string) {
	if state != driveStateWaitingOp {
		return false, ""
	}
	m := drivePingMarker(slug, state, drivePingBucket(oldestActDays))
	return !existing[m], m
}

// loadDriveMarkers reads the --drive-markers file and returns the set of drive
// markers (tracking, per-act, and ping) already present in open issues. A missing
// / empty path yields an empty set. Mirrors loadDecisionMarkers.
func loadDriveMarkers(path string) (map[string]bool, error) {
	set := map[string]bool{}
	if path == "" {
		return set, nil
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return set, nil
	}
	if err != nil {
		return nil, err
	}
	for _, m := range driveMarkerRe.FindAllString(string(raw), -1) {
		set[strings.TrimSpace(m)] = true
	}
	return set, nil
}

// driveWindowLine renders the one-line campaign header shared by the issue bodies.
func driveWindowLine(d Drive) string {
	w, _ := driveIntensityWeight(d.Intensity)
	return fmt.Sprintf("`%s` · %s (+%d) · window %s→%s · declared-by %s",
		d.Slug, d.Intensity, w, d.Starts, d.Expires, d.DeclaredBy)
}

// waitingOnYouTable renders the operator-act slice as a markdown table
// (act/unblocks · age), oldest first. Empty string when there are no acts.
func waitingOnYouTable(acts []FrontierItem) string {
	if len(acts) == 0 {
		return ""
	}
	sorted := append([]FrontierItem(nil), acts...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].AgeDays > sorted[j].AgeDays })
	var b strings.Builder
	b.WriteString("| Operator act — unblocks | Age |\n|---|---|\n")
	for _, a := range sorted {
		b.WriteString(fmt.Sprintf("| %s | %s |\n", a.Unblocks, ageLabel(a.AgeDays)))
	}
	return b.String()
}

func ageLabel(days int) string {
	switch days {
	case 0:
		return "today"
	case 1:
		return "1 day"
	default:
		return fmt.Sprintf("%d days", days)
	}
}

// trackingIssue builds the ONE tracking issue payload for a drive, plus the ping
// decision. The title gains the [WAITING ON YOU] tag iff the drive is
// WAITING-ON-OPERATOR (brief-44), and in that state the ping comment @-mentions
// @operator.
func trackingIssue(st DriveStatus, existing map[string]bool) driveIssue {
	d := st.Drive
	prefix := "drive: "
	if st.State == driveStateWaitingOp {
		prefix = waitingOnYouTitleTag + " drive: "
	}
	title := issueTitle(prefix, d.Slug, firstLine(d.Why))

	done, total := st.progress()
	acts := st.operatorActs()

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", driveTrackingMarker(d.Slug))
	fmt.Fprintf(&b, "## Drive `%s` — %s\n\n", d.Slug, st.State)
	if w := strings.TrimSpace(d.Why); w != "" {
		fmt.Fprintf(&b, "> %s\n\n", w)
	}
	fmt.Fprintf(&b, "%s\n\n", driveWindowLine(d))
	if total > 0 {
		fmt.Fprintf(&b, "**Progress:** %d/%d brief items done.\n\n", done, total)
	}
	fmt.Fprintf(&b, "**State:** `%s` — %s\n\n", st.State, driveStateExplain(st.State))
	if len(acts) > 0 {
		fmt.Fprintf(&b, "## ⏳ Waiting on you\n\n%s\n", waitingOnYouTable(acts))
		if st.State == driveStateWaitingOp {
			fmt.Fprintf(&b, "%s — the operator acts above are the only thing standing between the fleet and progress on this drive.\n\n", operatorHandle)
		}
	}
	fmt.Fprintf(&b, "## Frontier\n\n%s\n", frontierSummary(st.Frontier))
	fmt.Fprintf(&b, "\n---\n_This issue is auto-maintained by statusgen (methodology-metrics drives). "+
		"The boost evaporates and this issue goes quiet the moment the manifest's `state:` is no longer `active`._\n")

	iss := driveIssue{
		Slug:   d.Slug,
		Kind:   "tracking",
		State:  st.State,
		Title:  title,
		Labels: []string{driveTrackingLabel, driveLabelPrefix + d.Slug},
		Marker: driveTrackingMarker(d.Slug),
		Body:   b.String(),
	}
	if ping, marker := drivePingDecision(d.Slug, st.State, st.oldestActDays(), existing); ping {
		iss.Ping = true
		iss.PingMarker = marker
		iss.PingComment = pingComment(st, marker)
	}
	return iss
}

// pingComment is the @operator escalation comment posted (once, deduped by
// PingMarker) when a drive is WAITING-ON-OPERATOR.
func pingComment(st DriveStatus, marker string) string {
	acts := st.operatorActs()
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", marker)
	fmt.Fprintf(&b, "%s — drive `%s` is **WAITING ON YOU**. %d operator act(s) are the only thing blocking the fleet",
		operatorHandle, st.Drive.Slug, len(acts))
	if od := st.oldestActDays(); od > 0 {
		fmt.Fprintf(&b, "; the oldest has waited %s", ageLabel(od))
	}
	b.WriteString(".\n\n")
	b.WriteString(waitingOnYouTable(acts))
	return b.String()
}

// agingActIssues emits one aging issue per operator-act that has reached the aging
// threshold and does not already have an open issue (marker absent). idx is the
// act's ordinal among the drive's operator-act rows, so its marker is stable
// across regens.
func agingActIssues(st DriveStatus, existing map[string]bool) []driveIssue {
	var out []driveIssue
	idx := 0
	for _, f := range st.Frontier {
		if f.Kind != "operator-act" {
			continue
		}
		idx++
		if f.AgeDays < driveActAgingThresholdDays {
			continue // brand-new act — carried only on the tracking issue's table
		}
		marker := driveActMarker(st.Drive.Slug, idx)
		if existing[marker] {
			continue
		}
		var b strings.Builder
		fmt.Fprintf(&b, "%s\n\n", marker)
		fmt.Fprintf(&b, "## Operator act — drive `%s`\n\n", st.Drive.Slug)
		fmt.Fprintf(&b, "**Unblocks:** %s\n\n", f.Unblocks)
		fmt.Fprintf(&b, "**Waiting since %s (%s).**\n\n", f.Since, ageLabel(f.AgeDays))
		fmt.Fprintf(&b, "%s\n\n", driveWindowLine(st.Drive))
		fmt.Fprintf(&b, "Only an operator can perform this act. Close this issue when it is done — the drive tracking issue (`%s`) mirrors it.\n",
			driveTrackingMarker(st.Drive.Slug))
		out = append(out, driveIssue{
			Slug:   st.Drive.Slug,
			Kind:   "operator-act",
			Title:  issueTitle("drive-act: ", st.Drive.Slug, f.Unblocks),
			Labels: []string{driveOperatorLabel, driveLabelPrefix + st.Drive.Slug},
			Marker: marker,
			Body:   b.String(),
		})
	}
	return out
}

// frontierSummary renders a compact per-state count line for the tracking body.
func frontierSummary(frontier []FrontierItem) string {
	if len(frontier) == 0 {
		return "_no resolvable items_\n"
	}
	counts := map[string]int{}
	for _, f := range frontier {
		counts[f.State]++
	}
	order := []string{fsInFlight, fsReady, fsNeedsBrief, fsBlockedReview, fsBlockedOperator, fsBlockedItem, fsTracked, fsDone}
	var parts []string
	for _, s := range order {
		if counts[s] > 0 {
			parts = append(parts, fmt.Sprintf("%s: %d", s, counts[s]))
		}
	}
	return strings.Join(parts, " · ") + "\n"
}

// driveStateExplain is the one-line human gloss for each state.
func driveStateExplain(state string) string {
	switch state {
	case driveStateRolling:
		return "the fleet has dispatchable or in-flight work; no operator action is needed."
	case driveStateWaitingOp:
		return "nothing is dispatchable until an operator performs the act(s) below — you are the bottleneck."
	case driveStateWaitingRev:
		return "progress is gated only on review / sign-off; no operator action is needed."
	case driveStateStuck:
		return "the frontier is blocked with nothing dispatchable and no operator/review path — needs attention."
	}
	return ""
}

// driveIssues computes the drive-lifecycle issue payloads for every active drive:
// its tracking issue (+ ping decision) and any aged operator-act issues. Sorted
// for deterministic emission.
func driveIssues(streams []*Stream, ds DriveSet, claimed map[string]bool, now time.Time, existing map[string]bool) []driveIssue {
	out := []driveIssue{}
	for _, st := range driveStatuses(ds, streams, claimed, now) {
		out = append(out, trackingIssue(st, existing))
		out = append(out, agingActIssues(st, existing)...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Slug != out[j].Slug {
			return out[i].Slug < out[j].Slug
		}
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind // "operator-act" < "tracking"
		}
		return out[i].Marker < out[j].Marker
	})
	return out
}

// runDriveIssues is the --drive-issues entrypoint: emit the drive-lifecycle issue
// JSON array to stdout. Self-contained, STATUS.md-free, offline-deterministic (the
// same discipline as --decision-issues). Fail-neutral warnings from a malformed /
// expired manifest surface as NOTICEs on stderr and yield an empty array — a
// broken drive manifest never fails this command, exactly as it never fails the
// board.
func runDriveIssues(root, markersPath string) int {
	// loadHydratedStreams (not bare loadStreams): the frontier reads Brief.Depends
	// and Brief.Status, hydrated from brief-file frontmatter only via checkBriefFiles
	// — the same reason --gate-scores hydrates (issue #266).
	streams, _, err := loadHydratedStreams(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	// Claim-aware frontier: an open origin branch/PR marks a brief in-flight. A
	// degraded claim read is NOT fatal here — the emitter is advisory and an actor
	// reconciles against live issues; a missed claim only makes a todo look ready.
	claimed, _ := resolveClaims(root, streams)
	ds := loadDrives(root, streams, nowFunc())
	for _, w := range ds.Warnings {
		fmt.Fprintln(os.Stderr, "NOTICE:", w)
	}
	existing, err := loadDriveMarkers(markersPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: reading drive markers:", err)
		return 1
	}
	issues := driveIssues(streams, ds, claimed, nowFunc(), existing)
	enc, err := json.MarshalIndent(issues, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	fmt.Println(string(enc))
	return 0
}
