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

// driveRegionLintProblems checks every drive-plan .md under root.
//
// readClaims is the claim read the fresh render may make. run()'s --lint passes
// its own (lintClaims for the run's mode), so a plain --lint reads no claims
// here either: no remote branch list, no dead-claim decay (#3323,
// docs/statusgen-lint-reach.md). With no claim set read, a region's In-flight
// list is compared by assuming the claims it implies, and every other brief a
// claim would move to in-flight is assumed unclaimed. A region that matches
// only under such an assumption is a could-not-check NOTICE naming the assumed
// refs — never drift decided on claims nobody read, and never a clean pass
// either; a region no claim set explains is still drift.
//
// nil readClaims is the live read (resolveClaims, as --drive-snapshot renders)
// with nothing assumed: the reconcile lint hold's form, kept identical to its
// incremental evaluation (driveIncremental), which renders the same way.
//
// streams, when non-nil, is the full board the caller already hydrated; nil
// loads it (a product-scoped run hydrated only its scope).
func driveRegionLintProblems(root string, streams []*Stream, readClaims claimReader, now time.Time) (problems, notices []string) {
	ctx := &driveLintCtx{root: root, readClaims: readClaims}
	if streams != nil {
		ctx.loaded, ctx.streams = true, streams
	}
	if readClaims == nil {
		return driveRegionLint(root, func(slug string) (string, int) { return driveLintSection(ctx, slug, now) })
	}
	return driveRegionLintAssuming(root, func(slug, region string) (string, int, driveClaimAssumption) {
		return driveLintRender(ctx, slug, region, now)
	})
}

// driveRegionLint is driveRegionLintProblems with the fresh render supplied by
// the caller: section(slug) must return exactly what driveSnapshotSection(root,
// slug, now) would. The reconcile lint hold supplies one that reuses a single
// board load across the rows it evaluates (reconcileholdinc.go).
func driveRegionLint(root string, section func(slug string) (string, int)) (problems, notices []string) {
	return driveRegionLintAssuming(root, func(slug, _ string) (string, int, driveClaimAssumption) {
		fresh, rc := section(slug)
		return fresh, rc, driveClaimAssumption{}
	})
}

// driveRegionLintAssuming is driveRegionLint with a render that also sees the
// plan file's current region and returns the claims it had to assume rather
// than read (driveLintRender): refs assumed claimed and refs assumed unclaimed.
// A region that matches only under an assumption is a could-not-check NOTICE
// naming every assumed ref — never a clean pass; a region no render matches is
// drift.
func driveRegionLintAssuming(root string, render func(slug, region string) (fresh string, rc int, assumed driveClaimAssumption)) (problems, notices []string) {
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
		// attacker-plantable: a symlink at docs/roadmap/drives/<slug>.md — OR a
		// symlinked ANCESTOR directory of it — would make the os.ReadFile below
		// follow it and load an out-of-tree file (e.g. a secret) into the lint.
		// Refuse any symlink/escape LOUDLY (a PROBLEM, never silent, never followed)
		// before touching the file. Containment is rooted at the REPO ROOT — the
		// same rooting parseDrive uses for the sibling <slug>.yaml manifest
		// (escapesRoot + filepath.EvalSymlinks, drives.go); a sub-dir root would let
		// a symlinked ancestor resolve the containment root and the plan path to the
		// same out-of-tree place and slip through.
		if prob := drivePlanEscapeProblem(root, planPath, e.Name()); prob != "" {
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
		// readStaged: the reconcile lint hold stages a re-rendered region
		// here alongside the README move that changes it (#2694); with nothing
		// staged it is os.ReadFile.
		content, readErr := readStaged(planPath)
		if readErr != nil {
			notices = append(notices, fmt.Sprintf("drive-region-drift: docs/roadmap/drives/%s unreadable (%v) — could-not-check", e.Name(), readErr))
			continue
		}
		region, ok := extractDriveRegion(string(content), slug)
		if !ok {
			// A plan file with a manifest but no snapshot region yet: could-not-
			// check, not drift. The pen holder splices one with
			// `statusgen --drive-snapshot <slug>`.
			notices = append(notices, fmt.Sprintf("drive-region-drift: docs/roadmap/drives/%s carries no drive-snapshot:begin region — could-not-check (splice one with `statusgen --drive-snapshot %s`)", e.Name(), slug))
			continue
		}
		fresh, rc, assumed := render(slug, region)
		if rc != 0 {
			// Manifest present but not active/in-window/resolvable: the region
			// cannot be freshly rendered to compare. Three-state NOTICE.
			notices = append(notices, fmt.Sprintf("drive-region-drift: docs/roadmap/drives/%s has a region but %s renders no active drive (expired/held/unresolvable) — could-not-check", e.Name(), slug))
			continue
		}
		if driveRegionVerdict(fresh, string(content), slug) == 1 {
			problems = append(problems, driveRegionDriftProblem(e.Name(), slug))
			continue
		}
		if assumed.any() {
			notices = append(notices, fmt.Sprintf(
				"drive-region-drift: docs/roadmap/drives/%s matches a fresh render only if %s by an open branch; this run did not read the claim set, so that was assumed, not checked — could-not-check (the claim-filtering notice says why; --forge reads it)",
				e.Name(), driveAssumptionText(assumed)))
		}
	}
	return problems, notices
}

// driveAssumptionText states an assumption for the region NOTICE, e.g.
// "alpha/02 is claimed and alpha/03, alpha/04 are not claimed".
func driveAssumptionText(a driveClaimAssumption) string {
	verb := func(refs []string) string {
		if len(refs) == 1 {
			return "is"
		}
		return "are"
	}
	var parts []string
	if len(a.claimed) > 0 {
		parts = append(parts, fmt.Sprintf("%s %s claimed", strings.Join(a.claimed, ", "), verb(a.claimed)))
	}
	if len(a.unclaimed) > 0 {
		parts = append(parts, fmt.Sprintf("%s %s not claimed", strings.Join(a.unclaimed, ", "), verb(a.unclaimed)))
	}
	return strings.Join(parts, " and ")
}

// driveRegionDriftProblem is the drift PROBLEM for the plan file name (under
// docs/roadmap/drives) whose region for slug differs from a fresh render. The
// reconcile hold matches it exactly to find the regions a row move changes.
func driveRegionDriftProblem(name, slug string) string {
	return fmt.Sprintf(
		"drive-region-drift: docs/roadmap/drives/%s — the drive-snapshot region differs from a fresh `statusgen --drive-snapshot %s` render. Never hand-edit inside the fences; re-splice the region",
		name, slug)
}

// drivePlanEscapeProblem reports a fail-closed PROBLEM string when the plan-file
// entry at path is not a plain, in-tree regular file safe to read during --lint,
// else "". --lint runs automatically over the PR's OWN checked-out tree on a
// shared PUBLIC runner, so an entry under docs/roadmap/drives is attacker-
// plantable: a symlink there — the leaf OR any ANCESTOR directory — could point
// at an out-of-tree secret and os.ReadFile would follow it. The refusal must land
// BEFORE any bytes are read, on EVERY path:
//
//	(a) the leaf entry itself is a symlink;
//	(b) any ancestor directory between repoRoot and the leaf is a symlink; and
//	(c) the fully symlink-resolved path escapes repoRoot.
//
// repoRoot is the REPOSITORY ROOT (the same rooting parseDrive uses for the
// sibling <slug>.yaml manifest, drives.go) — NOT the docs/roadmap/drives sub-dir:
// rooting the containment at the sub-dir lets a symlinked ancestor resolve the
// containment root and the plan path to the same out-of-tree location so the
// escape is not flagged. name is the base name for the message. Nothing is read
// here; only Lstat/EvalSymlinks (metadata) run.
func drivePlanEscapeProblem(repoRoot, path, name string) string {
	fi, err := os.Lstat(path)
	if err != nil {
		// A genuine stat failure is handled by the caller's own os.ReadFile error
		// path as a could-not-check NOTICE — this guard only refuses symlink/escape,
		// it never swallows an ordinary unreadable file into a false PROBLEM.
		return ""
	}
	// (a) The leaf entry must be a regular in-tree file, never a symlink.
	if fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Sprintf(
			"drive-plan-symlink: docs/roadmap/drives/%s is a symlink — a drive-plan file must be a regular in-tree file; --lint refuses to follow it (symlink escape)",
			name)
	}
	// (b) No ANCESTOR directory on the way down from repoRoot to the leaf may be a
	// symlink: a symlinked parent would let os.ReadFile follow OUT of tree even
	// though the leaf itself is a regular file (the leaf Lstat above cannot see a
	// parent symlink). Walk each component and Lstat it — refuse the first symlink
	// LOUDLY, before any bytes are read.
	if rel, relErr := filepath.Rel(repoRoot, path); relErr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		cur := repoRoot
		for _, seg := range strings.Split(filepath.Dir(rel), string(os.PathSeparator)) {
			if seg == "" || seg == "." {
				continue
			}
			cur = filepath.Join(cur, seg)
			li, lerr := os.Lstat(cur)
			if lerr != nil {
				// An absent ancestor is not an escape; the caller's os.ReadFile will
				// fail as a could-not-check NOTICE. Keep walking the rest.
				continue
			}
			if li.Mode()&os.ModeSymlink != 0 {
				anc := seg
				if r, rErr := filepath.Rel(repoRoot, cur); rErr == nil {
					anc = r
				}
				return fmt.Sprintf(
					"drive-plan-symlink: docs/roadmap/drives/%s has a symlinked ancestor directory (%s) — a drive-plan file must live under real in-tree directories; --lint refuses to follow it (symlink escape)",
					name, anc)
			}
		}
	}
	// (c) Final containment, rooted at the REPO ROOT: even a path that reaches
	// outside repoRoot through any symlink is refused before it is read.
	if real, rerr := filepath.EvalSymlinks(path); rerr == nil && escapesRoot(repoRoot, real) {
		return fmt.Sprintf(
			"drive-plan-symlink: docs/roadmap/drives/%s resolves outside the repository (symlink escape) — a drive-plan file must live under the repo root",
			name)
	}
	return ""
}
