package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// pubmanifest.go — the *-private → do-not-copy PUBLICATION ASSERTION
// (distribution/13 Task E-b).
//
// THE INVARIANT. The methodology-stream split (distribution/13) moves the
// private planning briefs of a stream into a sibling directory named
// `<stream>-private/`, relying on the wholesale `docs/streams/` do-not-copy row
// in docs/publication-manifest.yaml to withhold it from the public tree. The
// naming convention (`*-private` == withheld) is the whole safety story, and a
// convention a human must remember is a convention that eventually ships a leak:
// someone adds a narrower `copy` row, or removes the wholesale withhold, and a
// private operating stream silently lands in a public copy. This lint makes the
// convention a MACHINE-CHECKED INVARIANT: any stream directory whose basename
// matches `*-private` MUST resolve, under the publication manifest, to a
// do-not-copy disposition. Defense-in-depth for the split — a second, mechanical
// lock behind the human's wholesale withhold.
//
// THREE-STATE (docs/three-state-instrument-rule.md):
//   - checked-clean  — a `*-private` dir that resolves to do-not-copy (silent).
//   - checked-failed — a `*-private` dir that resolves to copy/relocate, or that
//     no row covers at all (the default is withhold, but an UNROWED private dir
//     is not a MACHINE-GUARANTEED withhold — the assertion is "covered by a
//     do-not-copy disposition", so an uncovered private dir is a PROBLEM). Hard.
//   - could-not-check — `*-private` dirs exist but the manifest is absent or
//     unparseable: the invariant cannot be verified, so a NOTICE names it rather
//     than passing silently.
//
// INERT BY DEFAULT. A repo with no `*-private` stream (e.g. the public assay
// repo, whose private streams live elsewhere) never loads the manifest and emits
// nothing — adding this lint changes nothing for a tree that has not adopted the
// `-private` split.

// pubManifestRow is the SUBSET of a publication-manifest row this lint reads: a
// path and its disposition. The full schema (kind, reason, target, readme:,
// briefs:, overrides:, flag:, generated:, needs-ruling:) is the pubmanifest
// tool's jurisdiction; unread keys are ignored by yaml.Unmarshal, so this reader
// never fails on a row that carries fields it does not model.
type pubManifestRow struct {
	Path        string `yaml:"path"`
	Kind        string `yaml:"kind"`
	Disposition string `yaml:"disposition"`
}

type pubManifest struct {
	Schema string           `yaml:"schema"`
	Rows   []pubManifestRow `yaml:"rows"`
}

// pubManifestPath is the repo-relative location of the publication manifest.
const pubManifestPath = "docs/publication-manifest.yaml"

// loadPubManifest reads and parses the publication manifest under root.
// present=false with a nil error means the file is simply absent (the repo has
// not adopted a publication manifest); a non-nil error means it exists but could
// not be read or parsed (a could-not-check input).
func loadPubManifest(root string) (m *pubManifest, present bool, err error) {
	raw, rerr := os.ReadFile(filepath.Join(root, filepath.FromSlash(pubManifestPath)))
	if rerr != nil {
		if os.IsNotExist(rerr) {
			return nil, false, nil
		}
		return nil, false, rerr
	}
	var parsed pubManifest
	if uerr := yaml.Unmarshal(raw, &parsed); uerr != nil {
		return nil, true, uerr
	}
	return &parsed, true, nil
}

// resolveDir returns the manifest row that governs a directory path, applying
// the manifest's "narrowest statement wins" rule for a directory target: an
// exact-path row wins over a prefix row, and among prefix rows the LONGEST
// (most specific) wins. dirPath is repo-relative and ends with "/". ok=false
// when no row covers the directory (the default-withhold case, which for THIS
// assertion is treated as "not machine-guaranteed do-not-copy").
func (m *pubManifest) resolveDir(dirPath string) (row *pubManifestRow, ok bool) {
	best := -1
	for i := range m.Rows {
		p := strings.TrimSpace(m.Rows[i].Path)
		if p == "" {
			continue
		}
		match := p == dirPath || (strings.HasSuffix(p, "/") && strings.HasPrefix(dirPath, p))
		if match && len(p) > best {
			best = len(p)
			row = &m.Rows[i]
		}
	}
	return row, row != nil
}

// privateStreamDoNotCopyProblems asserts every `*-private` stream directory in
// `streams` resolves to a do-not-copy publication disposition. See the file
// header for the three-state contract.
func privateStreamDoNotCopyProblems(root string, streams []*Stream) (problems, notices []string) {
	var privDirs []*Stream
	for _, s := range streams {
		if strings.HasSuffix(filepath.Base(s.Dir), "-private") {
			privDirs = append(privDirs, s)
		}
	}
	if len(privDirs) == 0 {
		return nil, nil // inert: nothing named *-private to assert
	}

	m, present, err := loadPubManifest(root)
	if err != nil {
		for _, s := range privDirs {
			notices = append(notices, fmt.Sprintf(
				"%s: *-private stream dir exists but %s could not be parsed (%v) — the do-not-copy publication invariant is could-not-check, not verified",
				s.Name, pubManifestPath, err))
		}
		sort.Strings(notices)
		return nil, notices
	}
	if !present {
		for _, s := range privDirs {
			notices = append(notices, fmt.Sprintf(
				"%s: *-private stream dir exists but there is no %s to assert it is withheld — the do-not-copy publication invariant is could-not-check (defense-in-depth for the split cannot run here)",
				s.Name, pubManifestPath))
		}
		sort.Strings(notices)
		return nil, notices
	}

	for _, s := range privDirs {
		rel, rerr := filepath.Rel(root, s.Dir)
		if rerr != nil {
			notices = append(notices, fmt.Sprintf(
				"%s: could not compute the repo-relative path of the *-private stream dir (%v) — do-not-copy publication invariant could-not-check", s.Name, rerr))
			continue
		}
		dirPath := filepath.ToSlash(rel) + "/"
		row, ok := m.resolveDir(dirPath)
		if !ok {
			problems = append(problems, fmt.Sprintf(
				"%s: *-private stream dir %q is covered by NO %s disposition — a *-private stream must resolve to a do-not-copy disposition so the naming convention cannot silently ship a private stream to a public tree (distribution/13 E-b). Add a do-not-copy row (or confirm the wholesale docs/streams/ withhold covers it).",
				s.Name, dirPath, pubManifestPath))
			continue
		}
		if row.Disposition != "do-not-copy" {
			problems = append(problems, fmt.Sprintf(
				"%s: *-private stream dir %q resolves to disposition %q via manifest row %q, but a *-private stream MUST be do-not-copy — the naming convention is a machine-checked publication invariant and a *-private stream may never be copied or relocated into a public tree (distribution/13 E-b).",
				s.Name, dirPath, row.Disposition, row.Path))
		}
	}
	sort.Strings(problems)
	sort.Strings(notices)
	return problems, notices
}
