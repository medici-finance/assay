// Package main implements muhar, a mutation-testing harness that PROVES each
// mutation actually landed before it interprets a green test suite.
//
// Background. An ad-hoc harness built on BSD `sed -i ”` ran
// on a box with GNU sed, where `-i ”` means something else. Every mutation
// silently no-op'd, the suite therefore ran against pristine source on every
// mutation, every guard came back green, and the harness reported — with
// complete confidence — that EVERY guard was NOT CAUGHT by any test.
// A false NOT-CAUGHT is indistinguishable from a real one:
//
//	Real finding    : suite stays green because the guard has no discriminating test.
//	Broken harness  : suite stays green because the guard was never actually removed.
//
// Both produce "NOT CAUGHT"; the false one is more alarming, so more likely to
// be believed and escalated. This harness makes that impossible by construction:
//
//  1. A mutation is not APPLIED until it is PROVEN applied. applyEdit hard-fails
//     if the edit is a no-op (Old absent, Old == New, or content unchanged), and
//     runOne re-reads the file from disk after writing to confirm the intended
//     bytes actually landed. No proof, no result — the state becomes CouldNotMutate,
//     never NotCaught.
//  2. There are THREE result states, never two: Caught (checked-failed) /
//     NotCaught (checked-clean) / CouldNotMutate (could-not-check). "Could not
//     mutate" never collapses into "not caught".
//  3. A POSITIVE CONTROL — a mutation the suite MUST catch (a structural
//     invariant, an always-asserted equality) — runs every time. If the control
//     is not Caught, the harness is broken and the ENTIRE run is discarded, not
//     reported as a clean gate. A test harness with no failing control is not a
//     harness; it is a green light with no bulb.
//  4. A BASELINE run on pristine source must be green. If the suite is already
//     red before any mutation, every subsequent "caught" is meaningless, so the
//     run is discarded as broken.
//
// Parallelism. A sweep is N independent suite runs, and the suite is the whole
// cost. With Jobs > 1 the mutations are dealt out to a pool of workers, each
// owning an ISOLATED COPY of the tree (Workspace), so no two workers ever share
// a file being mutated — the one thing that would make a parallel sweep lie.
// Nothing about the verdicts changes: the baseline and the positive control
// still run first and still gate the whole run, and Results stay in spec order
// regardless of which worker got which mutation or in what order they finished.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Result is the outcome of planting one mutation and running the suite.
// Three states, never two (#34): a mutation that could not be planted is its
// OWN state and must never be read as "the guard has no test".
type Result int

const (
	// Caught: the suite went red with the mutation in place — the guard the
	// mutation broke has a discriminating test. "checked-failed."
	Caught Result = iota
	// NotCaught: the suite stayed green with the mutation PROVEN in place —
	// the guard has no discriminating test. A REAL finding. "checked-clean."
	NotCaught
	// CouldNotMutate: the edit did not land (no-op edit, target text absent,
	// on-disk bytes did not change). "could-not-check." This is the #34 state
	// that must never masquerade as NotCaught.
	CouldNotMutate
)

func (r Result) String() string {
	switch r {
	case Caught:
		return "CAUGHT"
	case NotCaught:
		return "NOT_CAUGHT"
	case CouldNotMutate:
		return "COULD_NOT_MUTATE"
	default:
		return "UNKNOWN"
	}
}

// Mutation describes one edit to plant in a source file: replace the first
// occurrence of Old with New in File. Old must be present and differ from New,
// or applyEdit refuses the mutation as a no-op.
type Mutation struct {
	Name string `json:"name"`
	File string `json:"file"` // path, resolved against Harness.Root when relative
	Old  string `json:"old"`  // exact text to remove (must be present)
	New  string `json:"new"`  // replacement text (must differ from Old)
}

// applyEdit returns content with the first occurrence of m.Old replaced by
// m.New, or an error if the edit would be a silent no-op. This is the heart of
// the #34 fix: an edit that changes nothing must ERROR, never be handed to the
// test suite as though the source were mutated.
//
// It hard-fails when:
//   - m.Old is empty (nothing to match against),
//   - m.Old == m.New (the "edit" changes nothing by definition — the exact
//     shape a mis-quoted `sed -i ”` degenerates into), or
//   - m.Old is absent from content (the target text is not there to remove).
//
// The final belt-and-braces check compares the result to the input and errors
// if they are byte-identical, so no degenerate replacement can slip through.
func applyEdit(content string, m Mutation) (string, error) {
	if m.Old == "" {
		return "", fmt.Errorf("mutation %q: Old is empty — nothing to match, edit cannot land", m.Name)
	}
	if m.Old == m.New {
		return "", fmt.Errorf("mutation %q: Old == New — edit is a no-op", m.Name)
	}
	if !strings.Contains(content, m.Old) {
		return "", fmt.Errorf("mutation %q: Old text not found in %s — edit did not land", m.Name, m.File)
	}
	mutated := strings.Replace(content, m.Old, m.New, 1)
	if mutated == content {
		return "", fmt.Errorf("mutation %q: content unchanged after replace — silent no-op", m.Name)
	}
	return mutated, nil
}

// Harness runs a set of mutations against real files, restoring each file after
// each run. Test reports whether the suite FAILED with the current on-disk
// state of the tree rooted at the root it is handed (true == red == the
// mutation was caught); it is a func so the core logic is exercisable without
// spawning a real process.
type Harness struct {
	Root      string                 // directory relative File paths resolve against
	Test      func(root string) bool // true == suite FAILED (mutation caught) against the tree at root
	Control   Mutation               // a mutation the suite MUST catch; guards against a broken harness
	Mutations []Mutation             // the guards under test

	// Jobs is how many mutations to have in flight at once. <= 1 runs the
	// sweep sequentially, in place, against Root — the original behaviour.
	// > 1 requires Workspace: each worker mutates its OWN copy of the tree,
	// because two workers editing one file would invalidate every verdict.
	Jobs int

	// Workspace provisions an isolated copy of the tree for worker id and
	// returns its root plus a cleanup. It is only consulted when Jobs > 1;
	// when it is nil the sweep falls back to running in place.
	Workspace func(id int) (root string, cleanup func(), err error)
}

// Report is the outcome of a whole run.
type Report struct {
	// Broken is set when the run cannot be trusted at all: the pristine
	// baseline was red, or the positive control was not caught. When Broken
	// is true every per-mutation Result is meaningless and must be discarded.
	Broken       bool
	BrokenReason string

	BaselineGreen bool   // suite passed on pristine source
	ControlResult Result // must be Caught for a trustworthy run

	Results []MutationResult
}

// MutationResult pairs a mutation with its three-state outcome and any error
// explaining a CouldNotMutate.
type MutationResult struct {
	Mutation Mutation
	Result   Result
	Err      error
}

// resolve locates m.File inside the tree rooted at root. An absolute path is
// taken as-is; an empty root leaves the path relative to the process cwd (the
// in-place sequential default).
func resolve(root, path string) string {
	if filepath.IsAbs(path) || root == "" {
		return path
	}
	return filepath.Join(root, path)
}

// runOne plants a single mutation in the tree at root, PROVES it landed on
// disk, runs the suite there, then restores the file. A mutation that cannot be
// proven on disk returns CouldNotMutate with an explanatory error — it never
// runs the suite, so it can never be mistaken for NotCaught.
func (h *Harness) runOne(root string, m Mutation) (Result, error) {
	path := resolve(root, m.File)
	orig, err := os.ReadFile(path)
	if err != nil {
		return CouldNotMutate, fmt.Errorf("mutation %q: read %s: %w", m.Name, path, err)
	}

	mutated, err := applyEdit(string(orig), m)
	if err != nil {
		return CouldNotMutate, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return CouldNotMutate, fmt.Errorf("mutation %q: stat %s: %w", m.Name, path, err)
	}
	if err := os.WriteFile(path, []byte(mutated), info.Mode().Perm()); err != nil {
		return CouldNotMutate, fmt.Errorf("mutation %q: write %s: %w", m.Name, path, err)
	}
	// Always restore, whatever happens next.
	defer func() { _ = os.WriteFile(path, orig, info.Mode().Perm()) }()

	// PROVE the edit landed on disk, not merely in memory. This is the check
	// the #34 harness lacked: had it re-read the file it would have seen its
	// `sed -i ''` no-op immediately, instead of running the suite on pristine
	// source and trusting the green result.
	onDisk, err := os.ReadFile(path)
	if err != nil {
		return CouldNotMutate, fmt.Errorf("mutation %q: re-read %s: %w", m.Name, path, err)
	}
	if string(onDisk) != mutated {
		return CouldNotMutate, fmt.Errorf("mutation %q: post-write verify failed — on-disk content does not match the intended mutation", m.Name)
	}
	if string(onDisk) == string(orig) {
		return CouldNotMutate, fmt.Errorf("mutation %q: post-write verify failed — file is byte-identical to the original", m.Name)
	}

	if h.Test(root) {
		return Caught, nil
	}
	return NotCaught, nil
}

// preflight runs the baseline check and then the positive control against the
// tree at root. The returned report is either Broken — in which case NO
// per-mutation verdict may be believed and the caller must stop — or a healthy
// header the caller fills the per-mutation results into.
func (h *Harness) preflight(root string) Report {
	var rep Report

	// (4) Baseline: pristine source must be green. A suite that is already red
	// makes every later "caught" meaningless.
	if h.Test(root) {
		rep.Broken = true
		rep.BrokenReason = "baseline suite is RED on pristine source — cannot distinguish a caught mutation from a pre-existing failure; discard the run"
		return rep
	}
	rep.BaselineGreen = true

	// (3) Positive control: a mutation the suite MUST catch. If it is not
	// caught, the harness is not mutating (or the suite cannot see the source)
	// and the whole run is worthless — exactly the #34 failure, made loud.
	cres, cerr := h.runOne(root, h.Control)
	rep.ControlResult = cres
	if cres != Caught {
		rep.Broken = true
		switch cres {
		case CouldNotMutate:
			rep.BrokenReason = fmt.Sprintf("positive control could not be planted (%v) — the harness is not mutating; discard the run", cerr)
		default:
			rep.BrokenReason = "positive control was NOT CAUGHT — a mutation the suite must catch stayed green, so the harness is broken (not mutating, or the suite cannot see the source); discard the run"
		}
		return rep
	}
	return rep
}

// Run executes the baseline check, the positive control, then every mutation.
// It short-circuits to a Broken report the moment the run becomes untrustworthy
// (red baseline or an uncaught control), because past that point NO per-mutation
// verdict can be believed.
//
// With Jobs > 1 and a Workspace provisioner, the per-mutation phase is dealt out
// to a worker pool over isolated copies of the tree. The verdicts are identical
// either way: the same baseline gate, the same control gate, and Results indexed
// by spec position so the report does not depend on completion order.
func (h *Harness) Run() Report {
	jobs := h.Jobs
	// Never provision more workspaces than there is work for them.
	if jobs > len(h.Mutations) {
		jobs = len(h.Mutations)
	}
	// No provisioner means no isolation, and an un-isolated pool would have
	// workers editing each other's source — fall back to the in-place sweep.
	if jobs < 1 || h.Workspace == nil {
		jobs = 1
	}
	if jobs == 1 {
		rep := h.preflight(h.Root)
		if rep.Broken {
			return rep
		}
		// The instrument is proven. Now every NotCaught is a real finding.
		for _, m := range h.Mutations {
			res, err := h.runOne(h.Root, m)
			rep.Results = append(rep.Results, MutationResult{Mutation: m, Result: res, Err: err})
		}
		return rep
	}
	return h.runParallel(jobs)
}

// runParallel provisions `jobs` isolated workspaces, preflights in the first one
// — so the baseline and the control are checked against a copy, proving the copy
// itself is sound before any verdict rests on it — then deals the mutations out
// to one worker per workspace.
func (h *Harness) runParallel(jobs int) Report {
	roots := make([]string, 0, jobs)
	var cleanups []func()
	defer func() {
		for _, c := range cleanups {
			c()
		}
	}()
	for i := 0; i < jobs; i++ {
		root, cleanup, err := h.Workspace(i)
		if err != nil {
			return Report{
				Broken:       true,
				BrokenReason: fmt.Sprintf("could not provision isolated workspace %d (%v) — a parallel sweep cannot share a tree; discard the run", i, err),
			}
		}
		if cleanup != nil {
			cleanups = append(cleanups, cleanup)
		}
		roots = append(roots, root)
	}

	rep := h.preflight(roots[0])
	if rep.Broken {
		return rep
	}

	// Results are indexed by spec position, never appended, so the report is
	// byte-identical to a sequential run whatever order the workers finish in.
	rep.Results = make([]MutationResult, len(h.Mutations))
	next := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < jobs; w++ {
		wg.Add(1)
		go func(root string) {
			defer wg.Done()
			for i := range next {
				m := h.Mutations[i]
				res, err := h.runOne(root, m)
				rep.Results[i] = MutationResult{Mutation: m, Result: res, Err: err}
			}
		}(roots[w])
	}
	for i := range h.Mutations {
		next <- i
	}
	close(next)
	wg.Wait()
	return rep
}

// copyTree makes a deep copy of src at dst: regular files (mode preserved),
// directories, and symlinks recreated as symlinks. Anything else (sockets,
// devices) is skipped — a source tree is not expected to contain them, and
// silently copying one is worse than not having it.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			info, err := d.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(target, info.Mode().Perm()|0o700)
		case d.Type()&os.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		case d.Type().IsRegular():
			return copyFile(path, target, d)
		default:
			return nil
		}
	})
}

func copyFile(path, target string, d os.DirEntry) error {
	info, err := d.Info()
	if err != nil {
		return err
	}
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// Summary renders a human/agent-readable report. A Broken run is reported ONLY
// as broken — it never lists per-guard verdicts, because those verdicts are the
// very thing that cannot be trusted.
func (r Report) Summary() string {
	var b strings.Builder
	if r.Broken {
		fmt.Fprintf(&b, "HARNESS BROKEN — run discarded.\n  %s\n", r.BrokenReason)
		return b.String()
	}
	fmt.Fprintf(&b, "Harness healthy: baseline GREEN, positive control %s.\n", r.ControlResult)
	var caught, notCaught, could int
	for _, m := range r.Results {
		line := fmt.Sprintf("  %-16s %s", m.Result, m.Mutation.Name)
		if m.Err != nil {
			line += fmt.Sprintf("  (%v)", m.Err)
		}
		fmt.Fprintln(&b, line)
		switch m.Result {
		case Caught:
			caught++
		case NotCaught:
			notCaught++
		case CouldNotMutate:
			could++
		}
	}
	fmt.Fprintf(&b, "Totals: %d caught, %d NOT CAUGHT, %d could-not-mutate.\n", caught, notCaught, could)
	if could > 0 {
		fmt.Fprintln(&b, "NOTE: could-not-mutate is NOT a clean gate — those guards were never actually tested.")
	}
	return b.String()
}
