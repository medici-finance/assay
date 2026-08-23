package main

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// drivedash.go — methodology-metrics phase 4 (brief-48): the `## Drive: <slug>`
// STATUS.md dashboard section. It renders phase 2's frontier/state (pure render
// — no new classification, no scoring) in brief-44's fixed Dashboard order:
//
//  1. the OPERATOR slice FIRST — the state banner + the last-regen heartbeat
//     line, then the WAITING-ON-YOU table (`act · unblocks · age · issue`) — so
//     the human sees their own blockers before anything else;
//  2. progress `done/total`;
//  3. in-flight;
//  4. blocked-on-review;
//  5. frontier-next.
//
// The section renders ONLY when a drive is active, so a no-manifest board stays
// byte-identical to the pre-drives baseline (TestDriveAbsentIsInert).
//
// The heartbeat is git-derived (the committed STATUS.md blob's HEAD SHA + commit
// date) — deterministic per commit, never a fresh wall-clock read into the board
// bytes. A heartbeat is written only on a successful regen, so a PROBLEM that
// aborts the write leaves the previous, now-stale heartbeat in place — which is
// precisely the signal the --watchdog meta-alarm reads.

// activeDriveStatuses and activeDriveHeartbeat are the phase-4 render inputs for
// emit(): the active drives' frontier/state and the git-derived heartbeat line.
// Set by run() immediately before the emit call; nil/empty when no drive is
// active or before the board build. (Package vars, not emit parameters, so the
// many emit() call sites — tests included — keep their signatures; the same
// convention as activeDriveSet in drives.go.)
var (
	activeDriveStatuses  []DriveStatus
	activeDriveHeartbeat string
)

// driveHeartbeatLine renders the one-line last-regen heartbeat. Sourced from the
// committed STATUS.md blob's provenance (its last commit's abbreviated SHA and
// committer date), so it is deterministic per committed tree and bakes no fresh
// wall-clock read into the board. "unknown" when git is unavailable or the blob
// has no committed history yet.
func driveHeartbeatLine(root string) string {
	cmd := exec.Command("git", "-C", root, "log", "-1", "--format=%h %cI", "--", "STATUS.md")
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	line := strings.TrimSpace(string(out))
	if line == "" {
		return "unknown"
	}
	return line
}

// driveActIssues maps operator-act ordinals to their issue reference cell: an
// aged act has its own aging issue (driveissues.go), a fresh act rides the
// tracking issue. Rendered as text — statusgen emits intents, not issue numbers.
func driveActIssueRef(act FrontierItem, idx int) string {
	if act.AgeDays >= driveActAgingThresholdDays {
		return fmt.Sprintf("aging issue (act %d)", idx)
	}
	return "tracking issue"
}

// driveWaitingOnYouTable renders the phase-4 WAITING-ON-YOU table with the
// brief-44 column set: act · unblocks · age · issue. Oldest act first.
func driveWaitingOnYouTable(acts []FrontierItem) string {
	if len(acts) == 0 {
		return ""
	}
	type actRow struct {
		act FrontierItem
		idx int
	}
	rows := make([]actRow, 0, len(acts))
	for i, a := range acts {
		rows = append(rows, actRow{act: a, idx: i + 1})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].act.AgeDays > rows[j].act.AgeDays })
	var b strings.Builder
	b.WriteString("| Act | Unblocks | Age | Issue |\n")
	b.WriteString("|---|---|---|---|\n")
	for _, r := range rows {
		b.WriteString(fmt.Sprintf("| act %d | %s | %s | %s |\n",
			r.idx, r.act.Unblocks, ageLabel(r.act.AgeDays), driveActIssueRef(r.act, r.idx)))
	}
	return b.String()
}

// driveOperatorSlice renders the operator-first slice shared by the STATUS.md
// dashboard section and the tracking-issue mirror (brief-44 decision #2 — the
// push channel and the dashboard show ONE truth): the state banner + the
// heartbeat line, then the WAITING-ON-YOU table.
func driveOperatorSlice(st DriveStatus, heartbeat string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "**State:** `%s` — %s\n\n", st.State, driveStateExplain(st.State))
	fmt.Fprintf(&b, "_last regen: %s_\n\n", heartbeat)
	if table := driveWaitingOnYouTable(st.operatorActs()); table != "" {
		fmt.Fprintf(&b, "**Waiting on you:**\n\n%s\n", table)
	}
	return b.String()
}

// frontierRefs returns the refs of frontier items in the given state, in
// frontier order.
func frontierRefs(frontier []FrontierItem, state string) []string {
	var out []string
	for _, f := range frontier {
		if f.State == state {
			out = append(out, f.Ref)
		}
	}
	return out
}

// bulletList renders refs as a markdown bullet list, or _none_ when empty.
func bulletList(refs []string) string {
	if len(refs) == 0 {
		return "_none_\n"
	}
	var b strings.Builder
	for _, r := range refs {
		fmt.Fprintf(&b, "- %s\n", r)
	}
	return b.String()
}

// driveDashboardSection renders the whole `## Drive: <slug>` section for one
// active drive, in brief-44's fixed Dashboard order.
func driveDashboardSection(st DriveStatus, heartbeat string) string {
	d := st.Drive
	done, total := st.progress()

	var b strings.Builder
	fmt.Fprintf(&b, "## Drive: `%s`\n\n", d.Slug)
	b.WriteString(driveOperatorSlice(st, heartbeat))
	if total > 0 {
		fmt.Fprintf(&b, "**Progress:** %d/%d brief items done.\n\n", done, total)
	}
	fmt.Fprintf(&b, "**In-flight:**\n\n%s\n", bulletList(frontierRefs(st.Frontier, fsInFlight)))
	fmt.Fprintf(&b, "**Blocked on review:**\n\n%s\n", bulletList(frontierRefs(st.Frontier, fsBlockedReview)))
	fmt.Fprintf(&b, "**Frontier next:**\n\n%s\n", bulletList(frontierRefs(st.Frontier, fsReady)))
	return b.String()
}

// driveSections renders the dashboard sections for every active drive, in the
// loader's deterministic slug order. Empty string when no drive is active — the
// absent-is-inert safety bar.
func driveSections(statuses []DriveStatus, heartbeat string) string {
	var b strings.Builder
	for _, st := range statuses {
		b.WriteString("\n")
		b.WriteString(driveDashboardSection(st, heartbeat))
	}
	return b.String()
}
