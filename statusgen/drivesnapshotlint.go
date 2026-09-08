package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// drivesnapshotlint.go — the `--lint` rules that keep a drive-plan .md honest,
// so "never hand-edit inside the fences" is code, not a sentence a pen holder
// has to remember, and the dispatcher-visibility rule is mechanical: a drive
// the dispatcher cannot see (a plan file with no manifest) is not a drive.
//
// Two rules, over every docs/roadmap/drives/<slug>.md drive-plan file:
//
//   drive-without-manifest  — a plan .md with no <slug>.yaml manifest beside it
//                             is a drive nothing loads: PROBLEM.
//   drive-region-drift      — a plan .md whose drive-snapshot region differs
//                             from a fresh render has been hand-edited inside
//                             the fences: PROBLEM. (A region that is merely
//                             absent, or that could not be rendered because the
//                             manifest is not active/in-window, is a NOTICE —
//                             three-state, never a false PROBLEM.)
//
// Absent docs/roadmap/drives ⇒ INERT: no plan files, no problems, no notices —
// the byte-identical baseline every drive check preserves. Run in --lint only
// (the PR gate); the STATUS.md write path never reads these files.

// driveRegionLintProblems checks every drive-plan .md under root. It never reads
// or writes STATUS.md and takes no live infrastructure.
func driveRegionLintProblems(root string, now time.Time) (problems, notices []string) {
	dir := filepath.Join(root, "docs", "roadmap", "drives")
	entries, err := os.ReadDir(dir)
	if err != nil {
		// Absent ⇒ inert. Unreadable ⇒ a single could-not-check NOTICE (never a
		// PROBLEM: a drive artifact must not freeze the lint any more than it
		// freezes the board).
		if !os.IsNotExist(err) {
			notices = append(notices, fmt.Sprintf("drive-region-drift: docs/roadmap/drives could not be read (%v) — could-not-check", err))
		}
		return nil, notices
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		slug := strings.TrimSuffix(e.Name(), ".md")
		mfPath := filepath.Join(dir, slug+".yaml")
		if _, statErr := os.Stat(mfPath); statErr != nil {
			problems = append(problems, fmt.Sprintf(
				"drive-without-manifest: drive-plan file docs/roadmap/drives/%s has no %s.yaml manifest beside it — a drive the dispatcher cannot load is not a drive; add the manifest or remove the plan file",
				e.Name(), slug))
			continue
		}
		content, readErr := os.ReadFile(filepath.Join(dir, e.Name()))
		if readErr != nil {
			notices = append(notices, fmt.Sprintf("drive-region-drift: docs/roadmap/drives/%s unreadable (%v) — could-not-check", e.Name(), readErr))
			continue
		}
		if _, ok := extractDriveRegion(string(content), slug); !ok {
			// A plan file with a manifest but no snapshot region yet: could-not-
			// check, not drift. The pen holder splices one with
			// `statusgen --drive-snapshot <slug>`.
			notices = append(notices, fmt.Sprintf("drive-region-drift: docs/roadmap/drives/%s carries no drive-snapshot:begin region — could-not-check (splice one with `statusgen --drive-snapshot %s`)", e.Name(), slug))
			continue
		}
		fresh, rc := driveSnapshotSection(root, slug, now)
		if rc != 0 {
			// Manifest present but not active/in-window/resolvable: the region
			// cannot be freshly rendered to compare. Three-state NOTICE.
			notices = append(notices, fmt.Sprintf("drive-region-drift: docs/roadmap/drives/%s has a region but %s renders no active drive (expired/held/unresolvable) — could-not-check", e.Name(), slug))
			continue
		}
		if driveRegionVerdict(fresh, string(content), slug) == 1 {
			problems = append(problems, fmt.Sprintf(
				"drive-region-drift: docs/roadmap/drives/%s — the drive-snapshot region differs from a fresh `statusgen --drive-snapshot %s` render. Never hand-edit inside the fences; re-splice the region",
				e.Name(), slug))
		}
	}
	return problems, notices
}
