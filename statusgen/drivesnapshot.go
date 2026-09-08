package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

// drivesnapshot.go — the `--drive-snapshot <slug> [--check <file>]` sub-command.
//
// The drive dashboard (methodology-metrics phase 4, drivedash.go) renders the
// `## Drive: <slug>` section INTO STATUS.md when a drive is active. This command
// prints EXACTLY that section on its own — same render function, same inputs,
// offline, STATUS.md-free — so a drive-plan file can carry a fenced snapshot of
// the board's own truth instead of a hand-maintained "are we done yet" table
// that drifts and then lies.
//
//   --drive-snapshot <slug>            prints the section to stdout (exit 0), or
//                                      exit 2 when <slug> names no active,
//                                      in-window, resolvable drive under --root.
//   --drive-snapshot <slug> --check F  compares F's snapshot region against a
//                                      fresh render: exit 0 identical, 1 drift,
//                                      2 could-not-check (no drive, no region,
//                                      or F unreadable). Nothing outside the
//                                      region is read or compared.
//
// `--check` here reuses the existing boolean --check flag as a modifier and
// takes the file as a trailing positional argument (statusgen --drive-snapshot
// <slug> --check <file>) — the shipped board-check `--check` is untouched.
//
// The render is driveDashboardSection VERBATIM: this file adds NOTHING to it.
// That is the whole point — the snapshot and the STATUS.md section can never
// disagree because they are the same bytes from the same function.

// The fenced region a drive-plan .md carries around its generated snapshot:
//
//	<!-- drive-snapshot:begin <slug> @<sha> <date> -->
//	## Drive: `<slug>`
//	…
//	<!-- drive-snapshot:end -->
//
// The begin marker's `@<sha> <date>` is provenance (when/what tree the region
// was last spliced at); it is NOT part of the compared body — only the text
// between the markers is compared, so a re-splice that only refreshes the
// provenance stamp is not counted as drift.
const (
	driveSnapshotBeginPrefix  = "<!-- drive-snapshot:begin "
	driveSnapshotEndMarker    = "<!-- drive-snapshot:end -->"
	driveSnapshotMarkerSuffix = " -->"
)

// driveHeartbeatBodyRe matches the git-derived heartbeat line inside a rendered
// section. The heartbeat tracks the committed STATUS.md blob's provenance, so it
// moves whenever STATUS.md is re-committed — independently of any edit to the
// drive itself. Normalising it out of the comparison means `--check` flags a
// HAND EDIT to the drive's state/table/frontier (the drift the region exists to
// catch) without firing on every unrelated board regen.
var driveHeartbeatBodyRe = regexp.MustCompile(`(?m)^_last regen: .*_[[:space:]]*$`)

// driveSnapshotRender renders the `## Drive: <slug>` section for one active
// drive from an already-loaded board, or (\"\", false) when the set carries no
// active drive with that slug. Pure — no disk, no clock read of its own — so the
// render is unit-testable without a fixture tree.
func driveSnapshotRender(ds DriveSet, streams []*Stream, claimed map[string]bool, slug, heartbeat string, now time.Time) (string, bool) {
	for _, st := range driveStatuses(ds, streams, claimed, now) {
		if st.Drive.Slug == slug {
			// Match driveSections' single trailing newline so the on-stdout form
			// and the spliced-into-a-file form are byte-identical.
			return strings.TrimRight(driveDashboardSection(st, heartbeat), "\n") + "\n", true
		}
	}
	return "", false
}

// driveSnapshotSection loads the board under root and renders the section for
// slug. rc is 0 (rendered) or 2 (could-not-check: no active drive with that slug,
// or the board could not be read). The heartbeat is git-derived from root, the
// same source drivedash uses for STATUS.md.
func driveSnapshotSection(root, slug string, now time.Time) (section string, rc int) {
	streams, _, err := loadHydratedStreams(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: --drive-snapshot could not read the board:", err)
		return "", 2
	}
	// Claim-aware frontier, degraded-tolerant: a failed claim read only makes a
	// todo look ready. Mirrors runDriveIssues.
	claimed, _ := resolveClaims(root, streams)
	ds := loadDrives(root, streams, now)
	for _, w := range ds.Warnings {
		fmt.Fprintln(os.Stderr, "NOTICE:", w)
	}
	section, ok := driveSnapshotRender(ds, streams, claimed, slug, driveHeartbeatLine(root), now)
	if !ok {
		return "", 2
	}
	return section, 0
}

// extractDriveRegion returns the text strictly BETWEEN the drive-snapshot begin
// and end markers for slug, or (\"\", false) when the file carries no such region.
// The begin marker must name the slug (a region for another drive is not this
// drive's region). Nothing outside the markers is returned.
func extractDriveRegion(content, slug string) (string, bool) {
	begin := driveSnapshotBeginPrefix + slug + " "
	bi := strings.Index(content, begin)
	if bi < 0 {
		// Tolerate an exact `begin <slug> -->` with no provenance stamp too.
		alt := driveSnapshotBeginPrefix + slug + driveSnapshotMarkerSuffix
		if bi = strings.Index(content, alt); bi < 0 {
			return "", false
		}
	}
	// Advance past the end of the begin marker line.
	rest := content[bi:]
	nl := strings.IndexByte(rest, '\n')
	if nl < 0 {
		return "", false
	}
	rest = rest[nl+1:]
	ei := strings.Index(rest, driveSnapshotEndMarker)
	if ei < 0 {
		return "", false
	}
	return rest[:ei], true
}

// normalizeDriveSnapshot canonicalises a snapshot for comparison: the heartbeat
// provenance line is neutralised (see driveHeartbeatBodyRe) and surrounding
// blank space is trimmed, so the verdict reflects the drive's CONTENT, not the
// board's regen clock.
func normalizeDriveSnapshot(s string) string {
	s = driveHeartbeatBodyRe.ReplaceAllString(s, "_last regen: _")
	return strings.TrimSpace(s)
}

// driveRegionVerdict compares a file's snapshot region for slug against a fresh
// render. 0 identical, 1 drift, 2 could-not-check (no region for slug). fresh is
// the freshly-rendered section; fileContent is the whole plan file.
func driveRegionVerdict(fresh, fileContent, slug string) int {
	region, ok := extractDriveRegion(fileContent, slug)
	if !ok {
		return 2
	}
	if normalizeDriveSnapshot(region) == normalizeDriveSnapshot(fresh) {
		return 0
	}
	return 1
}

// driveSnapshotCheck runs `--drive-snapshot <slug> --check <file>`: 0 identical,
// 1 drift, 2 could-not-check.
func driveSnapshotCheck(root, slug, file string, now time.Time) int {
	fresh, rc := driveSnapshotSection(root, slug, now)
	if rc == 2 {
		fmt.Fprintf(os.Stderr, "statusgen: --drive-snapshot --check: no active drive %q under %s (could-not-check)\n", slug, root)
		return 2
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "statusgen: --drive-snapshot --check: %v (could-not-check)\n", err)
		return 2
	}
	switch v := driveRegionVerdict(fresh, string(raw), slug); v {
	case 0:
		return 0
	case 1:
		fmt.Fprintf(os.Stderr, "statusgen: --drive-snapshot --check: %s — the drive-snapshot region differs from the fresh render (drift). Re-splice with `statusgen --drive-snapshot %s`.\n", file, slug)
		return 1
	default:
		fmt.Fprintf(os.Stderr, "statusgen: --drive-snapshot --check: %s has no drive-snapshot:begin region for %q (could-not-check)\n", file, slug)
		return 2
	}
}

// runDriveSnapshot dispatches the sub-command: print the section, or (with the
// --check modifier and a trailing file positional) compare a file's region.
func runDriveSnapshot(root, slug string, check bool, args []string, now time.Time) int {
	if check {
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "statusgen: --drive-snapshot --check needs a <file> argument (statusgen --drive-snapshot <slug> --check <file>)")
			return 2
		}
		return driveSnapshotCheck(root, slug, args[0], now)
	}
	section, rc := driveSnapshotSection(root, slug, now)
	if rc != 0 {
		fmt.Fprintf(os.Stderr, "statusgen: no active drive %q under %s (nothing to render)\n", slug, root)
		return 2
	}
	fmt.Print(section)
	return 0
}
