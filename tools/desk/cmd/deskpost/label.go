package main

// label.go — the MECHANICAL verdict-time labeler. After a verdict review is posted,
// postVerdictReview calls applyVerdictLabels to stamp two ADVISORY, triage-only labels on
// the PR: a diff SIZE class and a SURFACE tier. Both let the single human merge gate order
// its queue by exposure instead of by arrival.
//
// ADVISORY, NEVER A GATE. Every path here reports a problem by RETURNING it to the caller,
// which logs a WARNING and proceeds — the verdict's own success and exit code are already
// decided by the time this runs and are never disturbed by a labeling failure. Nothing here
// blocks, gates, or scores; the classification is deskkit's `wc -l` + glob only.
//
// THREE-STATE on could-not-classify. A family is only managed when its value is DEFINITE:
// the size label is skipped (and no stale size label removed) when the diff could not be
// read in full; the surface family is left entirely untouched when the repo declares no
// `.assay-surfaces` (SurfaceUnknown) or its config could not be read. A missing signal is
// reported as itself, never rounded to a guessed label.
//
// IDEMPOTENT. Same-family stale labels (a `size:*` other than the current class, a
// `surface:*` other than the current tier) are removed before the current ones are applied,
// so a re-run REPLACES rather than stacks, and the surface-tier comment carries a marker so
// a re-run does not post a duplicate.

import (
	"fmt"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// surfaceConfigPath is the repo-relative path of the declared risk-surface globs.
const surfaceConfigPath = ".assay-surfaces"

// surfaceCommentMarker tags the surface-tier comment so a re-run does not post a duplicate.
// It is an HTML comment, invisible in the rendered thread.
const surfaceCommentMarker = "<!-- assay:surface-tier -->"

// verdictLabelOutcome records what the labeler did, for the caller's audit/warning line.
type verdictLabelOutcome struct {
	sizeLabel    string // the size label applied, "" when the diff could not be read
	surface      deskkit.SurfaceState
	surfaceLabel string   // the surface label applied, "" when unknown/could-not-check
	matched      []string // the surface globs that matched, when surface:core
	applied      []string // labels added this run
	removed      []string // stale same-family labels removed this run
	notes        []string // could-not-check notes (skipped families), reported as themselves
}

// String renders the outcome for the caller's log line.
func (o verdictLabelOutcome) String() string {
	parts := []string{}
	if len(o.applied) > 0 {
		parts = append(parts, "applied "+strings.Join(o.applied, "+"))
	}
	if len(o.removed) > 0 {
		parts = append(parts, "removed "+strings.Join(o.removed, "+"))
	}
	for _, n := range o.notes {
		parts = append(parts, "skipped: "+n)
	}
	if len(parts) == 0 {
		return "no label change"
	}
	return strings.Join(parts, "; ")
}

// applyVerdictLabels computes and applies the two mechanical labels on the PR. reportedFiles
// is GitHub's own changed_files count (from the prInfo the caller already read), used to
// detect a short files read — the SAME reconciliation the risk-path gate uses, here to
// mark the size/surface determination could-not-check rather than to fail closed.
//
// It returns the outcome and the FIRST error encountered while applying labels. A non-nil
// error is advisory: the caller logs it and does not change the verdict result. A skipped
// family (short read, absent config) is NOT an error — it is a note on the outcome.
//
// THE LABEL WRITES GO THROUGH THE RESOLVER, not through this package's own client. Which
// forge serves the repo is deskkit.ForgeFor's answer (the roster binding, else an
// unambiguous origin host, else a refusal), and the whole reconciliation — ensure the
// labels exist, drop the stale same-family ones, apply the current ones — is ONE typed
// operation on the seam rather than four hand-built requests here. That is what makes this
// path forge-neutral: nothing below constructs a label endpoint, and a forge whose label
// mapping does not fit refuses by name inside the backend rather than being approximated
// here.
func applyVerdictLabels(c *ghClient, pr, reportedFiles int) (verdictLabelOutcome, error) {
	var out verdictLabelOutcome

	files, err := c.listFiles(pr)
	if err != nil {
		return out, fmt.Errorf("listing changed files for size/surface labels: %w", err)
	}

	// Short read → the diff could not be read in full, so NEITHER the size count nor the
	// surface match is trustworthy (a hidden core path would misclassify as std). Skip both
	// families and report could-not-check; never apply a label derived from a partial diff.
	if reportedFiles > 0 && len(files) < reportedFiles {
		out.notes = append(out.notes, fmt.Sprintf(
			"read %d changed files but GitHub reports %d — size+surface labels not applied (could-not-check)",
			len(files), reportedFiles))
		return out, nil
	}

	// --- size class (always computable from a complete diff) ---
	deltas := make([]deskkit.FileDelta, 0, len(files))
	for _, f := range files {
		deltas = append(deltas, deskkit.FileDelta{Path: f.Filename, Changed: f.Additions + f.Deletions})
	}
	out.sizeLabel = deskkit.SizeClassLabel(deskkit.ChangedLineCount(deltas))

	// --- surface tier (three-state on the .assay-surfaces config) ---
	cfg, found, cerr := c.getRepoFile(surfaceConfigPath)
	switch {
	case cerr != nil:
		out.surface = deskkit.SurfaceUnknown
		out.notes = append(out.notes, "could not read "+surfaceConfigPath+" — no surface label ("+cerr.Error()+")")
	case !found:
		out.surface = deskkit.SurfaceUnknown
		out.notes = append(out.notes, "no "+surfaceConfigPath+" in repo — size label only (absent-config)")
	default:
		globs := deskkit.ParseSurfaceGlobs(cfg)
		out.surface, out.matched = deskkit.ClassifySurface(true, globs, prFilePaths(files))
	}
	if lbl, ok := out.surface.Label(); ok {
		out.surfaceLabel = lbl
	}

	// Desired labels this run: the size class always, the surface label only when definite.
	desired := []string{out.sizeLabel}
	if out.surfaceLabel != "" {
		desired = append(desired, out.surfaceLabel)
	}

	// ONE reconciliation request. The FAMILIES named here are the ones this run has a
	// definite value for: size always (a complete diff always yields one), surface only when
	// out.surfaceLabel != "" — never when Unknown, because an absent or unreadable
	// `.assay-surfaces` has no opinion and must therefore remove nothing. Naming a family is
	// what licenses the backend to drop its stale members; not naming it leaves it untouched.
	change := deskkit.LabelChange{
		Add:            []deskkit.LabelSpec{},
		RemoveFamilies: []string{deskkit.SizeLabelPrefix},
	}
	for _, l := range desired {
		change.Add = append(change.Add, deskkit.LabelSpec{
			Name: l, Color: labelColorFor(l), Description: labelDescFor(l),
		})
	}
	if out.surfaceLabel != "" {
		change.RemoveFamilies = append(change.RemoveFamilies, deskkit.SurfaceLabelPrefix)
	}
	fg, ferr := deskkit.ForgeFor(deskkit.ForgeRepo{Owner: c.owner, Name: c.repo}, "reviewer")
	if ferr != nil {
		return out, fmt.Errorf("resolving the forge for the verdict labels: %w", ferr)
	}
	res, aerr := fg.ApplyLabels(deskkit.ForgeRepo{Owner: c.owner, Name: c.repo}, pr, change)
	if aerr != nil {
		return out, fmt.Errorf("applying labels %v: %w", desired, aerr)
	}
	if res != nil {
		out.applied = res.Added
		out.removed = res.Removed
	}

	// surface:core → post the dereferencing comment naming the matched globs, once.
	if out.surface == deskkit.SurfaceCore {
		if e := c.postSurfaceCoreComment(pr, out.matched); e != nil {
			return out, fmt.Errorf("posting surface:core comment: %w", e)
		}
	}
	return out, nil
}

// postSurfaceCoreComment posts (once) the advisory comment naming the matched surface
// globs, so the human can spot a misclassification at a glance. Idempotent via
// surfaceCommentMarker: if a comment already carries the marker, it is not re-posted.
func (c *ghClient) postSurfaceCoreComment(pr int, matched []string) error {
	bodies, err := c.listCommentBodies(pr)
	if err != nil {
		return err
	}
	for _, b := range bodies {
		if strings.Contains(b, surfaceCommentMarker) {
			return nil // already posted
		}
	}
	quoted := make([]string, 0, len(matched))
	for _, g := range matched {
		quoted = append(quoted, "`"+g+"`")
	}
	body := surfaceCommentMarker + "\n" +
		"`surface:core` — this PR's diff touches a declared risk surface. Matched surface glob(s): " +
		strings.Join(quoted, ", ") + ".\n\n" +
		"This is a mechanical, advisory label for merge-queue triage (derived from `.assay-surfaces` " +
		"by glob match); it gates nothing."
	return c.postComment(pr, body)
}

// labelColorFor / labelDescFor supply cosmetic defaults for label creation. They are
// advisory metadata only; the label NAME is the load-bearing part.
func labelColorFor(label string) string {
	switch {
	case label == deskkit.SurfaceCoreLabel:
		return "b60205" // red — touches a declared risk surface
	case label == deskkit.SurfaceStdLabel:
		return "0e8a16" // green — standard surface
	case strings.HasPrefix(label, deskkit.SizeLabelPrefix):
		return "c5def5" // light blue — size class
	default:
		return "ededed"
	}
}

func labelDescFor(label string) string {
	switch {
	case label == deskkit.SurfaceCoreLabel:
		return "Advisory: diff touches a declared risk surface (.assay-surfaces)"
	case label == deskkit.SurfaceStdLabel:
		return "Advisory: diff touches no declared risk surface"
	case strings.HasPrefix(label, deskkit.SizeLabelPrefix):
		return "Advisory diff-size class (changed lines, generated files excluded)"
	default:
		return ""
	}
}
