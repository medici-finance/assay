// Package filer is quality/14's pluggable issue-filing seam (spec §9.5). It
// defines the IssueFiler interface a consumer routes advisory refactor work
// through, and ships one generic GitHub Issues REFERENCE adapter in-tree
// (githubadapter.go). Other filers are configuration behind this interface,
// never a fork of the consumer.
//
// Two invariants the whole package is built around:
//
//   - Advisory, never self-dispatching. A RefactorItem is something a human or
//     an intake process TRIAGES. Nothing here assigns, starts, or dispatches
//     work; the type carries no assignee and no "start" verb, by design.
//   - Dry-run is first-class. Every File call takes a dryRun flag, and any
//     adapter may additionally force dry-run (its own config, or an unwired
//     reference adapter). Dry-run composes the exact item that WOULD be filed
//     and writes to no tracker, so filing can be proven — in tests and over
//     budget — without touching a live Issues API.
//
// The budget itself lives with the CALLER (the autofile consumer): the filer
// exposes the dry-run degrade path the caller drives when it is over budget,
// it does not count budget on its own.
package filer

import "strings"

// RefactorItem is one composed, advisory refactor work item — the thing an
// IssueFiler files or dry-runs. It is deduplicated by TargetPath: a consumer
// composes at most one item per distinct target (spec §9.5).
type RefactorItem struct {
	// Title is the one-line summary a tracker shows in its list.
	Title string
	// Body is the full description. For a hotspot- or cluster-derived item it
	// references the target path(s) so a reader can find the code without the
	// mined artifact in hand (Verify #3 asserts the body dereferences the right
	// hotspot).
	Body string
	// Labels are the tracker labels to apply. The "advisory" label is always
	// present on an item composed through NewAdvisoryItem.
	Labels []string
	// TargetPath is the repo-relative path this item is about — the dedup key.
	// A cluster-derived item uses its primary (lexically first) path.
	TargetPath string
	// Advisory marks the item as advisory-only. It is always true for items a
	// consumer composes; a false here is a bug in the composer, and File
	// refuses it so a non-advisory item can never reach a tracker.
	Advisory bool
}

// AdvisoryLabel is the label every advisory item carries, so a human triaging
// the tracker can filter the auto-filed advisory queue from hand-filed work.
const AdvisoryLabel = "advisory"

// NewAdvisoryItem composes an advisory RefactorItem, guaranteeing the advisory
// flag is set and the advisory label present exactly once. extraLabels are
// appended (deduped) after the advisory label.
func NewAdvisoryItem(title, body, targetPath string, extraLabels ...string) RefactorItem {
	labels := []string{AdvisoryLabel}
	seen := map[string]bool{AdvisoryLabel: true}
	for _, l := range extraLabels {
		if l == "" || seen[l] {
			continue
		}
		seen[l] = true
		labels = append(labels, l)
	}
	return RefactorItem{
		Title:      title,
		Body:       body,
		Labels:     labels,
		TargetPath: targetPath,
		Advisory:   true,
	}
}

// FiledResult reports what an IssueFiler.File call did with an item. Filed is
// true only when the item was actually written to a tracker; on any dry-run
// (caller-forced OR adapter-forced) Filed is false, DryRun is true, and Ref is
// empty. Item echoes the exact composed item, so a dry-run caller can log or
// assert on precisely what WOULD have been filed.
type FiledResult struct {
	Item   RefactorItem
	Filed  bool
	DryRun bool
	// Ref is the tracker reference (e.g. an issue URL or number) when Filed is
	// true; empty on a dry-run.
	Ref string
}

// IssueFiler files, or dry-runs, one advisory refactor item.
//
// File composes the item into a FiledResult. When dryRun is true — or when the
// concrete filer forces dry-run for its own reasons — nothing is written to any
// tracker and the returned FiledResult has Filed == false, DryRun == true. The
// budget cap is the CALLER's: over budget, the caller passes dryRun = true so
// the item is composed and logged but never filed (spec §9.5).
type IssueFiler interface {
	File(item RefactorItem, dryRun bool) (FiledResult, error)
}

// ErrNotAdvisory is returned by a filer handed a non-advisory item. Auto-filed
// refactor work is advisory by construction; a filer refuses anything else so
// the never-self-dispatch posture cannot be bypassed by handing it a bare item.
var ErrNotAdvisory = errNotAdvisory{}

type errNotAdvisory struct{}

func (errNotAdvisory) Error() string {
	return "filer: refusing a non-advisory refactor item; auto-filed work is advisory only (spec §9.5)"
}

// validateItem is the shared guard every adapter runs before composing or
// filing: the item must be advisory and carry a target path.
func validateItem(item RefactorItem) error {
	if !item.Advisory {
		return ErrNotAdvisory
	}
	if strings.TrimSpace(item.TargetPath) == "" {
		return errNoTarget{}
	}
	return nil
}

type errNoTarget struct{}

func (errNoTarget) Error() string {
	return "filer: refactor item has no target path (the dedup key is required)"
}
