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
		planPath := filepath.Join(dir, e.Name())
		// Symlink containment, fail-CLOSED. --lint runs automatically over the PR's
		// OWN checked-out tree on a shared PUBLIC runner (assay-statusgen.yml,
		// pull_request → `statusgen --root . --lint`), so a plan .md here is
		// attacker-plantable: a symlink at docs/roadmap/drives/<slug>.md would make
		// the os.ReadFile below follow it and load an out-of-tree file (e.g. a
		// secret) into the lint. Refuse any symlink/escape LOUDLY (a PROBLEM, never
		// silent, never followed) before touching the file — the same surface
		// parseDrive already hardened for the sibling <slug>.yaml manifest
		// (escapesRoot + filepath.EvalSymlinks, drives.go).
		if prob := drivePlanEscapeProblem(dir, planPath, e.Name()); prob != "" {
			problems = append(problems, prob)
			continue
		}
		mfPath := filepath.Join(dir, slug+".yaml")
		// Lstat (not Stat): a manifest committed as a symlink must not be laundered
		// into "manifest present" by following it. Absent ⇒ drive-without-manifest;
		// a symlinked manifest ⇒ its own loud PROBLEM (parseDrive contains the read
		// itself, but the lint refuses to treat an escape as a legitimate manifest).
		mfInfo, statErr := os.Lstat(mfPath)
		if statErr != nil {
			problems = append(problems, fmt.Sprintf(
				"drive-without-manifest: drive-plan file docs/roadmap/drives/%s has no %s.yaml manifest beside it — a drive the dispatcher cannot load is not a drive; add the manifest or remove the plan file",
				e.Name(), slug))
			continue
		}
		if mfInfo.Mode()&os.ModeSymlink != 0 {
			problems = append(problems, fmt.Sprintf(
				"drive-manifest-symlink: docs/roadmap/drives/%s.yaml is a symlink — a drive manifest must be a regular in-tree file; --lint refuses to follow it (symlink escape)",
				slug))
			continue
		}
		content, readErr := os.ReadFile(planPath)
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

// drivePlanEscapeProblem reports a fail-closed PROBLEM string when the plan-file
// entry at path is not a plain, in-tree regular file safe to read during --lint,
// else "". --lint runs automatically over the PR's OWN checked-out tree on a
// shared PUBLIC runner, so an entry under docs/roadmap/drives is attacker-
// plantable: a symlink there could point at an out-of-tree secret and os.ReadFile
// would follow it. Refuse any symlink LOUDLY (never silent, never followed) and
// reject any path that resolves outside root — the same containment parseDrive
// uses for the sibling <slug>.yaml manifest (escapesRoot + filepath.EvalSymlinks,
// drives.go). root is the drives dir; name is the base name for the message.
func drivePlanEscapeProblem(root, path, name string) string {
	fi, err := os.Lstat(path)
	if err != nil {
		// A genuine stat failure is handled by the caller's own os.ReadFile error
		// path as a could-not-check NOTICE — this guard only refuses symlink/escape,
		// it never swallows an ordinary unreadable file into a false PROBLEM.
		return ""
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Sprintf(
			"drive-plan-symlink: docs/roadmap/drives/%s is a symlink — a drive-plan file must be a regular in-tree file; --lint refuses to follow it (symlink escape)",
			name)
	}
	// Belt-and-suspenders: even a non-symlink entry whose real path escapes root
	// (e.g. reached through a symlinked parent) is refused before it is read.
	if real, rerr := filepath.EvalSymlinks(path); rerr == nil && escapesRoot(root, real) {
		return fmt.Sprintf(
			"drive-plan-symlink: docs/roadmap/drives/%s resolves outside the repository (symlink escape) — a drive-plan file must live under the repo root",
			name)
	}
	return ""
}
