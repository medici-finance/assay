package main

// delta.go — the --delta (changed-rows-only) and --quiet (one-line summary) modes
// for deskboard's read subcommands (deskquiet).
//
// The standing desks narrate every sweep: full board tables each iteration, even
// when nothing moved. The signal — a needs-decision item, a verdict, an exception —
// drowns in iteration noise. These two flags reshape console stdout so a quiet loop
// earns ONE line and a delta sweep prints only what CHANGED:
//
//   --delta   print only rows CHANGED since the previous invocation (new, removed,
//             or field-changed), against a snapshot keyed by (subcommand, repo set)
//             in the desk-tools state dir. First run = full output, labeled
//             `first run — no prior snapshot`. A missing, corrupt, or
//             schema-mismatched snapshot = full output with a `snapshot reset` label
//             — NEVER a silent empty diff.
//   --quiet   one summary line — counts per state bucket + delta count vs snapshot.
//             Composable with --delta (quiet line first, changed rows after).
//             --quiet ALONE does not consume the snapshot: it prints no rows, so
//             the Δ segment is an unread badge that persists until a run renders
//             them.
//
// Fail-dangerous direction (brief exec-tier-why): a delta mode that wrongly diffs
// against a stale/corrupt snapshot silently HIDES a new actionable row, and the
// tests pass while the desk goes blind. Every unassessable snapshot path therefore
// falls back to FULL output — the tool may be noisy by accident but never quiet by
// accident (#236 lesson applied to stdout). A partial/errored read never reaches
// the snapshot-write step: deskboard's fetchers are already fail-closed, so
// a gh/API error aborts (exit 6) before a snapshot could be advanced on bad data.
//
// Snapshots advance ONLY on a successful full read that also RENDERED the rows
// (i.e. --delta was set), and only the rows the read returned — so the next diff is
// always against a known-good baseline the desk has actually seen.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// snapshotSchema versions the on-disk snapshot shape. A bump (or a snapshot written
// by a shape this build did not produce) yields full output + "snapshot reset" —
// never a diff against a shape we cannot interpret.
const snapshotSchema = 1

// deltaItem is one normalized item for delta computation: a stable ID, a signature
// capturing every tracked field (a changed field changes the signature), and a
// Display line for the delta rendering. Subcommands build these from their own row
// types; the machinery here is subcommand-agnostic.
type deltaItem struct {
	ID        string
	Signature string
	Display   string
}

// deltaSet is the normalized view of one report's rows. Summary is the
// subcommand-specific state-bucket string for the --quiet line (e.g.
// "51 open (48 draft)"); Actionable is the count of rows the desk must act on now,
// and ActionableLabel names WHAT that count counts (review finding, Minor 1: a
// count labelled "actionable" that is just len(rows) restates the summary and reads
// as a claim the tool cannot make — each subcommand names the subset it can prove).
// RepoSet is the sorted set of repos the report covered, so the snapshot is keyed
// by the same set the diff ran against (a changed DESK_ROOTS never diffs against
// the wrong baseline).
type deltaSet struct {
	Items           []deltaItem
	Summary         string
	Actionable      int
	ActionableLabel string
	RepoSet         []string
}

// actionableLabel returns the third-segment label, defaulting to "actionable" so an
// extractor that does not set one still renders a sane line.
func (d deltaSet) actionableLabel() string {
	if d.ActionableLabel == "" {
		return "actionable"
	}
	return d.ActionableLabel
}

// snapshot is the persisted prior state for one (subcommand, repo set).
type snapshot struct {
	Schema int               `json:"schema"`
	Items  map[string]string `json:"items"` // ID → signature
}

// snapshotAssessment is how the prior snapshot was interpreted on load.
type snapshotAssessment int

const (
	snapOK       snapshotAssessment = iota // usable — diff against it
	snapFirstRun                           // no prior file (first run for this key)
	snapReset                              // file present but corrupt/schema-mismatched
)

// label returns the human label for the assessment, printed above full output.
func (a snapshotAssessment) label() string {
	switch a {
	case snapFirstRun:
		return "first run — no prior snapshot"
	case snapReset:
		return "snapshot reset — prior snapshot was missing, corrupt, or schema-mismatched; showing full output"
	default:
		return ""
	}
}

// deltaResult is the diff of the current set against the prior snapshot.
type deltaResult struct {
	assessment snapshotAssessment
	added      []deltaItem // in current, not in prior
	removed    []deltaItem // in prior, not in current (Display is the PRIOR display —
	// best-effort; the snapshot only stores IDs+signatures, so removed rows render as
	// their ID, not a full display line)
	changed []deltaItem // in both, signature differs
}

// hasDelta reports whether any row changed.
func (d deltaResult) hasDelta() bool {
	return len(d.added)+len(d.removed)+len(d.changed) > 0
}

// ---------------------------------------------------------------------------
// snapshot path + load / save
// ---------------------------------------------------------------------------

// snapshotsSubdir is the state-dir subpath snapshots live under. One directory for
// every deskboard read subcommand keeps them together and easy to inspect/clean.
const snapshotsSubdir = "snapshots"

// snapshotFile returns the absolute path of the snapshot for (sub, repoSet). The
// repo set is hashed (sorted, NUL-joined) so a changed DESK_ROOTS or a different
// compiled-in set produces a DIFFERENT file rather than diffing against the wrong
// baseline. The hash is short (16 hex) — it is a disambiguator, not a secret.
func snapshotFile(sub string, repoSet []string) (string, error) {
	dir, err := deskkit.StateDir()
	if err != nil {
		return "", deskkit.Unverifiable("cannot resolve desk-tools state dir for snapshot (HOME missing?)", err)
	}
	key := repoSetKey(repoSet)
	sum := sha256.Sum256([]byte(key))
	short := hex.EncodeToString(sum[:])[:16]
	return filepath.Join(dir, snapshotsSubdir, sub+"-"+short+".json"), nil
}

// repoSetKey is the stable string a repo set hashes to. Sorted so the same set in a
// different order keys the same file; NUL-joined so a boundary can't be forged by
// concatenation (same rule as deskkit.ArgsDigest).
func repoSetKey(repoSet []string) string {
	s := append([]string(nil), repoSet...)
	sort.Strings(s)
	return strings.Join(s, "\x00")
}

// loadSnapshot reads the prior snapshot for path. Fail-open semantics (brief facts):
//   - file MISSING  → snapFirstRun (full output + first-run label). NOT an error.
//   - unreadable or unparseable or schema ≠ snapshotSchema → snapReset (full output
//     plus a reset label). NOT an error: a corrupt snapshot is the one case the desk
//     must NOT trust a diff, so it falls back to full output. The bad file is left
//     in place — saveSnapshot overwrites it on this successful full read, healing it.
//
// This function NEVER returns an error for a bad snapshot: every unassessable path
// is snapReset, because the fail-dangerous direction is hiding a row behind a bad
// diff, and a reset surfaces everything.
func loadSnapshot(path string) (snapshot, snapshotAssessment) {
	raw, rerr := os.ReadFile(path)
	if rerr != nil {
		if os.IsNotExist(rerr) {
			return snapshot{Items: map[string]string{}}, snapFirstRun
		}
		// unreadable — reset, never an empty diff
		return snapshot{Items: map[string]string{}}, snapReset
	}
	var snap snapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		// Load-bearing, not redundant with the schema check below (review 6b): an
		// UNPARSEABLE file leaves Schema at zero (encoding/json validates before
		// assigning), but a TYPE-mismatched one — valid JSON, `items` not a map —
		// assigns Schema=1 and then fails, so only this branch stops a diff against
		// a half-decoded snapshot. Pinned by TestSnapshot_TypeMismatchIsReset.
		return snapshot{Items: map[string]string{}}, snapReset
	}
	if snap.Schema != snapshotSchema {
		return snapshot{Items: map[string]string{}}, snapReset
	}
	if snap.Items == nil {
		snap.Items = map[string]string{}
	}
	return snap, snapOK
}

// saveSnapshot writes the snapshot (best-effort). A write failure is NOT a read
// failure — the board read succeeded and the output already went out; the worst a
// failed write does is make the next run reset again (full output), which is the
// safe fallback anyway. So the error is returned for the caller to WARN, not abort.
func saveSnapshot(path string, snap snapshot) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	snap.Schema = snapshotSchema
	b, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ---------------------------------------------------------------------------
// diff
// ---------------------------------------------------------------------------

// diffSets computes the delta of cur against prev. The assessment carried by the
// result mirrors the snapshot assessment: on first-run/reset the added/removed/
// changed lists are EMPTY (the caller prints full output, not a misleading diff).
func diffSets(cur deltaSet, prev snapshot, assessment snapshotAssessment) deltaResult {
	res := deltaResult{assessment: assessment}
	if assessment != snapOK {
		return res // no meaningful diff against a baseline we could not trust
	}
	prevSigs := prev.Items
	curIDs := map[string]bool{}
	for _, it := range cur.Items {
		curIDs[it.ID] = true
		prevSig, existed := prevSigs[it.ID]
		if !existed {
			res.added = append(res.added, it)
		} else if prevSig != it.Signature {
			res.changed = append(res.changed, it)
		}
	}
	for id := range prevSigs {
		if !curIDs[id] {
			res.removed = append(res.removed, deltaItem{ID: id})
		}
	}
	// Stable order for deterministic output.
	sortDelta(res.added)
	sortDelta(res.changed)
	sortDelta(res.removed)
	return res
}

func sortDelta(s []deltaItem) {
	sort.SliceStable(s, func(i, j int) bool { return s[i].ID < s[j].ID })
}

// ---------------------------------------------------------------------------
// rendering
// ---------------------------------------------------------------------------

// quietLine renders the one-line --quiet summary. Format (brief facts):
//
//	<sub>: <summary> | Δ +<added> ~<changed> -<removed> | <n> <actionableLabel>
//
// On first-run/reset the delta is meaningless (no trusted baseline), so the Δ
// segment names it instead of quoting a number that reads as "nothing changed":
//
//	<sub>: <summary> | Δ first run | <actionable> actionable
//	<sub>: <summary> | Δ reset | <actionable> actionable
func quietLine(sub string, ds deltaSet, dr deltaResult) string {
	delta := deltaSegment(dr)
	return fmt.Sprintf("%s: %s | %s | %d %s", sub, ds.Summary, delta, ds.Actionable, ds.actionableLabel())
}

// deltaSegment renders the Δ portion of the quiet line, omitting zero-count
// categories so a quiet board stays quiet.
func deltaSegment(dr deltaResult) string {
	if dr.assessment == snapFirstRun {
		return "Δ first run"
	}
	if dr.assessment == snapReset {
		return "Δ reset"
	}
	var parts []string
	if a := len(dr.added); a > 0 {
		parts = append(parts, fmt.Sprintf("+%d", a))
	}
	if c := len(dr.changed); c > 0 {
		parts = append(parts, fmt.Sprintf("~%d", c))
	}
	if r := len(dr.removed); r > 0 {
		parts = append(parts, fmt.Sprintf("-%d", r))
	}
	if len(parts) == 0 {
		return "Δ 0"
	}
	return "Δ " + strings.Join(parts, " ")
}

// renderDelta prints the changed-rows table for --delta. Each row is prefixed by its
// change marker (+ added, ~ field-changed, - removed). Removed rows carry only the
// ID (the prior snapshot stores no display text).
func renderDelta(w io.Writer, sub string, dr deltaResult) {
	fmt.Fprintf(w, "%s delta — %s\n", sub, deltaSegment(dr))
	if a := len(dr.added); a > 0 {
		fmt.Fprintf(w, "+ added (%d):\n", a)
		for _, it := range dr.added {
			fmt.Fprintf(w, "  + %s\n", it.Display)
		}
	}
	if c := len(dr.changed); c > 0 {
		fmt.Fprintf(w, "~ changed (%d):\n", c)
		for _, it := range dr.changed {
			fmt.Fprintf(w, "  ~ %s\n", it.Display)
		}
	}
	if r := len(dr.removed); r > 0 {
		fmt.Fprintf(w, "- removed (%d):\n", r)
		for _, it := range dr.removed {
			fmt.Fprintf(w, "  - %s\n", it.ID)
		}
	}
}

// renderNoChange prints the one-line "nothing moved" result for --delta when the
// snapshot was usable and the diff is empty. Counts the unchanged items so the desk
// knows the sweep covered N rows, not zero.
func renderNoChange(w io.Writer, sub string, ds deltaSet) {
	fmt.Fprintf(w, "%s: no change (%d items unchanged)\n", sub, len(ds.Items))
}

// ---------------------------------------------------------------------------
// orchestration (called from main.go after a successful dispatch)
// ---------------------------------------------------------------------------

// deltaDeps is the (subcommand → deltaSet extractor) dispatch that main.go consults.
// A subcommand not in this map does not support --delta/--quiet; passing either
// flag for it is a Refused (exit 5) — fail closed rather than silently ignore the
// flag a desk relied on for noise discipline.
//
// Each extractor returns the normalized set from the already-built report value, so
// the delta path never re-reads the network.
type deltaExtractor func(rep any) (deltaSet, bool)

var deltaExtractors = map[string]deltaExtractor{
	"prs":     prsDeltaSet,
	"actions": actionsDeltaSet,
	"queue":   queueDeltaSet,
	"nextup":  nextupDeltaSet,
}

// runDeltaQuiet carries out the --delta/--quiet reshaping of stdout for one
// successful dispatch. It returns the audit Detail suffix to append (the delta
// summary) and writes the reshaped output to stdout. A snapshot write failure is a
// WARNING to stderr, never an abort (the read succeeded). The function returns
// (detail, ok) where ok=false means ONLY that the subcommand is not in
// deltaExtractors — every other unassessable path fails OPEN to full output here
// rather than handing the caller a "nothing to print" it might honor literally.
func runDeltaQuiet(stdout, stderr io.Writer, sub string, rep *Report, delta, quiet bool) (detail string, ok bool) {
	extractor, supported := deltaExtractors[sub]
	if !supported {
		return "", false
	}
	ds, extracted := extractor(rep.value)
	if !extracted {
		// The subcommand claims delta support but its report value did not match the
		// shape the extractor expects (a refactor drifted the two apart). That is
		// exactly the "quiet by accident" direction: fail OPEN — full output + a
		// reset-shaped label + a loud warning — never an empty console.
		fmt.Fprintf(stderr, "deskboard: WARNING %s report shape did not match its delta extractor; showing full output\n", sub)
		fmt.Fprintf(stdout, "%s — snapshot reset (delta unavailable for this report shape); showing full output\n", sub)
		rep.render(stdout)
		return " delta=unavailable(shape)", true
	}
	path, perr := snapshotFile(sub, ds.RepoSet)
	if perr != nil {
		// Could not locate the state dir — fail OPEN: print full output (the flag
		// could not be honored, but the read was good) + a reset-shaped label, and
		// warn. Never hide rows behind an unlocatable snapshot.
		fmt.Fprintf(stderr, "deskboard: WARNING could not resolve snapshot path for %s: %v\n", sub, perr)
		fmt.Fprintf(stdout, "%s — snapshot reset (state dir unavailable); showing full output\n", sub)
		rep.render(stdout)
		return " delta=reset(no-state-dir)", true
	}
	prev, assessment := loadSnapshot(path)
	dr := diffSets(ds, prev, assessment)

	// --quiet line first (composable with --delta: quiet line, then changed rows).
	if quiet {
		fmt.Fprintln(stdout, quietLine(sub, ds, dr))
	}
	// --delta: full output on first-run/reset; changed rows (or "no change") otherwise.
	if delta {
		switch {
		case assessment != snapOK:
			fmt.Fprintf(stdout, "%s — %s\n", sub, assessment.label())
			// Full output: defer to the report's own table renderer.
			rep.render(stdout)
		case !dr.hasDelta():
			renderNoChange(stdout, sub, ds)
		default:
			renderDelta(stdout, sub, dr)
		}
	}
	// Advance the snapshot ONLY on a run that RENDERED the rows (a review
	// finding). The baseline is "the last state the desk was SHOWN", not
	// "the last state read": a --quiet-only run prints counts, never rows, so
	// consuming the baseline there would answer the desk's own follow-up --delta
	// with "no change" and make the new row unrecoverable through the delta path —
	// quiet by accident, the exact direction this file exists to prevent. So a
	// quiet-only run HOLDS the baseline; its Δ segment is an unread badge that
	// persists (and keeps growing) until a --delta run actually shows the rows.
	//
	// Corollary, documented in the README: a quiet-only loop with no trusted
	// baseline keeps reporting `Δ first run` / `Δ reset` rather than silently
	// adopting the unseen board as "already seen". Noisy, never quiet.
	if !delta {
		return deltaAuditDetail(dr) + " snapshot=held(quiet-only)", true
	}
	// The fetchers already abort (exit 6) on any partial/errored read, so reaching
	// here means the read was good. Overwriting a prior corrupt file HEALS it.
	cur := snapshot{Schema: snapshotSchema, Items: map[string]string{}}
	for _, it := range ds.Items {
		cur.Items[it.ID] = it.Signature
	}
	if werr := saveSnapshot(path, cur); werr != nil {
		fmt.Fprintf(stderr, "deskboard: WARNING could not write %s snapshot: %v\n", sub, werr)
	}
	return deltaAuditDetail(dr), true
}

// deltaAuditDetail is the suffix appended to the audit Detail line so a later
// forensic read of the log sees whether the run carried a delta and what it found.
func deltaAuditDetail(dr deltaResult) string {
	switch dr.assessment {
	case snapFirstRun:
		return " delta=first-run"
	case snapReset:
		return " delta=reset"
	default:
		return fmt.Sprintf(" delta=+%d~%d-%d", len(dr.added), len(dr.changed), len(dr.removed))
	}
}
