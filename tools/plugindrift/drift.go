package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Statuses. `unreachable` is deliberately distinct from `in-sync`: a source the
// checker could not read is an unknown, never a pass.
const (
	StatusInSync      = "in-sync"
	StatusBehind      = "behind"
	StatusMoved       = "moved"
	StatusUnreachable = "unreachable"
)

// ErrNotFound is returned by a SourceClient when a path does not exist at a ref.
var ErrNotFound = errors.New("not found")

// Manifest is plugins/assay/SOURCES.yaml.
type Manifest struct {
	Version       int             `yaml:"version"`
	Bundle        string          `yaml:"bundle"`
	BundleVersion string          `yaml:"bundle-version"`
	Files         []BundleFile    `yaml:"files"`
	Canonical     []CanonicalFile `yaml:"canonical"`
	Unported      []UnportedFile  `yaml:"unported"`
}

// CanonicalFile is a bundled file whose HOME is the bundle itself: it was
// ported historically but the authority flip (harness-portability/02) ended
// the port relationship, so there is no upstream to drift against and nothing
// to re-fetch. Unlike UnportedFile ("authored here, never had a source"), a
// canonical file HAS a porting history — carried in Notes — and is the single
// home of its method text. Coverage counts it as accounted, like unported.
type CanonicalFile struct {
	Path   string `yaml:"path"`
	Reason string `yaml:"reason"`
	Notes  string `yaml:"notes"`
}

// UnportedFile is a bundled file with NO upstream — authored in this repo, so
// there is nothing to drift against. Declaring it is what lets the coverage
// check tell "has no source" apart from "somebody forgot to pin it".
type UnportedFile struct {
	Path   string `yaml:"path"`
	Reason string `yaml:"reason"`
}

// BundleFile is one bundled file plus where it came from.
type BundleFile struct {
	Path           string  `yaml:"path"`
	Source         Source  `yaml:"source"`
	CrossReference *Source `yaml:"cross-reference"`
	Notes          string  `yaml:"notes"`
}

// Source is one origin: a repo, a path inside it, and the commit/blob pinned.
type Source struct {
	Kind   string `yaml:"kind"`
	Repo   string `yaml:"repo"`
	Path   string `yaml:"path"`
	Commit string `yaml:"commit"`
	Blob   string `yaml:"blob"`
	AsOf   string `yaml:"as-of"`
}

// Result is one checked (file, origin) pair.
type Result struct {
	File   string // bundled path
	Origin string // "source" | "cross-reference"
	Status string
	// Commits is the number of commits touching the source path since the
	// recorded commit. -1 means "could not be determined".
	Commits int
	Detail  string
	// Advisory marks a result that must not, on its own, decide the verdict.
	Advisory bool
}

// SourceClient reads a remote source repo. Split out so the check logic is
// testable without a network or a `gh` binary.
type SourceClient interface {
	// DefaultBranch returns the repo's default branch name.
	DefaultBranch(repo string) (string, error)
	// BlobSHA returns the git blob sha of path at ref, or ErrNotFound.
	BlobSHA(repo, path, ref string) (string, error)
	// CommitDate returns the RFC3339 committer date of commit, or ErrNotFound.
	CommitDate(repo, commit string) (string, error)
	// TagCommit resolves a tag name to the commit sha it points at (peeling
	// annotated tags), or ErrNotFound when the tag does not exist.
	TagCommit(repo, tag string) (string, error)
	// CommitsSince counts commits touching path on ref with a committer date
	// at or after since (RFC3339). capped is true when the count hit the
	// maxPages ceiling and is therefore a lower bound.
	//
	// The count is anchored on a DATE, not on walking back to the recorded
	// commit: the recorded commit is the tree the file was copied from, and
	// need not be a commit that touched this particular path.
	//
	// KNOWN LIMITATION — the date anchor UNDER-COUNTS. A commit whose own date
	// precedes the recorded commit's timestamp but which entered the default
	// branch afterwards is reachable from head and is nonetheless filtered out.
	// So count is a lower bound on the true ancestry distance in both senses:
	// pagination (capped) and this filter. It does not affect the in-sync /
	// behind / moved verdict, which comes from blob shas. See main.go's package
	// comment and tools/README.md.
	CommitsSince(repo, path, ref, since string, maxPages int) (count int, capped bool, err error)
}

// Validate rejects a manifest the checker cannot act on. A malformed manifest is
// an error, never a silent pass.
func (m *Manifest) Validate() error {
	if m.Version != 1 {
		return fmt.Errorf("unsupported manifest version %d (want 1)", m.Version)
	}
	// Post-flip a manifest may legitimately list zero `files:` — every bundled
	// skill is canonical-here or authored-here. It must list SOMETHING across
	// the three lists, or there is nothing for coverage to check against.
	if len(m.Files) == 0 && len(m.Canonical) == 0 && len(m.Unported) == 0 {
		return errors.New("manifest lists no files, canonical files, or unported files")
	}
	seen := map[string]bool{}
	for i, f := range m.Files {
		if f.Path == "" {
			return fmt.Errorf("files[%d]: missing path", i)
		}
		if seen[f.Path] {
			return fmt.Errorf("%s: listed twice", f.Path)
		}
		seen[f.Path] = true
		if err := f.Source.validate(); err != nil {
			return fmt.Errorf("%s: source: %w", f.Path, err)
		}
		if f.CrossReference != nil {
			if err := f.CrossReference.validate(); err != nil {
				return fmt.Errorf("%s: cross-reference: %w", f.Path, err)
			}
		}
	}
	for i, c := range m.Canonical {
		if c.Path == "" {
			return fmt.Errorf("canonical[%d]: missing path", i)
		}
		if seen[c.Path] {
			return fmt.Errorf("%s: listed in more than one of files:, canonical:, unported:", c.Path)
		}
		seen[c.Path] = true
		// A canonical entry claims the bundle is the home of this file's method
		// text. Requiring a reason keeps it a disclosure rather than a way to
		// silence the coverage check.
		if strings.TrimSpace(c.Reason) == "" {
			return fmt.Errorf("%s: canonical entries need a reason", c.Path)
		}
	}
	for i, u := range m.Unported {
		if u.Path == "" {
			return fmt.Errorf("unported[%d]: missing path", i)
		}
		if seen[u.Path] {
			return fmt.Errorf("%s: listed in more than one of files:, canonical:, unported:", u.Path)
		}
		seen[u.Path] = true
		// An unported entry is a claim that a file has no upstream. Requiring a
		// reason keeps it a disclosure rather than a way to silence the
		// coverage check.
		if strings.TrimSpace(u.Reason) == "" {
			return fmt.Errorf("%s: unported entries need a reason", u.Path)
		}
	}
	return nil
}

// coverageGlob is the set of bundled files the manifest must account for,
// relative to the bundle directory. SKILL.md is the unit of porting: one skill,
// one file, one upstream (or an explicit "no upstream").
const coverageGlob = "skills/*/SKILL.md"

// Coverage is the result of matching the manifest against the bundle on disk.
type Coverage struct {
	Pinned      int      // accounted for by files:
	Canonical   int      // accounted for by canonical:
	Unported    int      // accounted for by unported:
	OnDisk      int      // files matching coverageGlob
	Unaccounted []string // on disk, in neither list
	Missing     []string // in the manifest, not on disk
}

// CheckCoverage asserts that every file matching coverageGlob under bundleDir is
// accounted for — pinned in `files:`, declared canonical-here in `canonical:`,
// or declared in `unported:` — and that every manifest entry points at a file
// that exists.
//
// Without this, a skill ported from upstream can be added with no manifest entry
// and no signal: the checker would report every pinned file in-sync and say
// nothing about the one nobody pinned. That is precisely the silent rot this
// tool exists to catch, so an unaccounted file is an error, not a warning.
func CheckCoverage(bundleDir string, m *Manifest) (Coverage, error) {
	cov := Coverage{Pinned: len(m.Files), Canonical: len(m.Canonical), Unported: len(m.Unported)}

	matches, err := filepath.Glob(filepath.Join(bundleDir, filepath.FromSlash(coverageGlob)))
	if err != nil {
		return cov, fmt.Errorf("coverage glob %s: %w", coverageGlob, err)
	}
	cov.OnDisk = len(matches)
	if len(matches) == 0 {
		// Fail closed: an empty bundle means the bundle directory is wrong, not
		// that everything is accounted for.
		return cov, fmt.Errorf("no files match %s under %s — nothing to check, which is never a pass", coverageGlob, bundleDir)
	}

	accounted := map[string]bool{}
	for _, f := range m.Files {
		accounted[f.Path] = true
	}
	for _, c := range m.Canonical {
		accounted[c.Path] = true
	}
	for _, u := range m.Unported {
		accounted[u.Path] = true
	}

	onDisk := map[string]bool{}
	for _, abs := range matches {
		rel, err := filepath.Rel(bundleDir, abs)
		if err != nil {
			return cov, fmt.Errorf("coverage: %w", err)
		}
		rel = filepath.ToSlash(rel)
		onDisk[rel] = true
		if !accounted[rel] {
			cov.Unaccounted = append(cov.Unaccounted, rel)
		}
	}
	for p := range accounted {
		// Only entries inside the coverage scope can be checked for existence;
		// the manifest may legitimately pin files outside it later.
		if matched, _ := filepath.Match(coverageGlob, p); matched && !onDisk[p] {
			cov.Missing = append(cov.Missing, p)
		}
	}
	sort.Strings(cov.Unaccounted)
	sort.Strings(cov.Missing)

	switch {
	case len(cov.Unaccounted) > 0 && len(cov.Missing) > 0:
		return cov, fmt.Errorf("not in files:, canonical:, or unported: %s; in the manifest but not on disk: %s",
			strings.Join(cov.Unaccounted, ", "), strings.Join(cov.Missing, ", "))
	case len(cov.Unaccounted) > 0:
		return cov, fmt.Errorf("bundled but unaccounted for (add to files: with a source, to canonical: as the home, or to unported: with a reason): %s",
			strings.Join(cov.Unaccounted, ", "))
	case len(cov.Missing) > 0:
		return cov, fmt.Errorf("listed in the manifest but not on disk (renamed or deleted?): %s",
			strings.Join(cov.Missing, ", "))
	}
	return cov, nil
}

func (s Source) validate() error {
	switch s.Kind {
	case "github":
		if !strings.Contains(s.Repo, "/") {
			return fmt.Errorf("kind github needs an owner/name repo, got %q", s.Repo)
		}
		// The blob is the drift baseline. Without it the check degrades to
		// "has anything touched this path since the recorded date", which
		// reports in-sync on a path nobody has edited WITHOUT ever comparing
		// content — a pass that proves nothing. Required, not optional.
		if s.Blob == "" {
			return errors.New("kind github needs a blob (the drift baseline); read it back with `gh api repos/<repo>/contents/<path>?ref=<commit> -q .sha`")
		}
	case "local-git":
		// Not remotely addressable; always reported unreachable.
	case "":
		return errors.New("missing kind")
	default:
		return fmt.Errorf("unknown kind %q (want github or local-git)", s.Kind)
	}
	if s.Path == "" {
		return errors.New("missing path")
	}
	if s.Commit == "" {
		return errors.New("missing commit")
	}
	return nil
}

// Check runs every origin in the manifest and returns one Result per origin,
// in manifest order (source before cross-reference).
func Check(c SourceClient, m *Manifest, maxPages int) []Result {
	var out []Result
	for _, f := range m.Files {
		r := checkSource(c, f.Source, maxPages)
		r.File, r.Origin = f.Path, "source"
		out = append(out, r)
		if f.CrossReference != nil {
			r := checkSource(c, *f.CrossReference, maxPages)
			r.File, r.Origin, r.Advisory = f.Path, "cross-reference", true
			out = append(out, r)
		}
	}
	return out
}

func checkSource(c SourceClient, s Source, maxPages int) Result {
	if s.Kind != "github" {
		return Result{
			Status:  StatusUnreachable,
			Commits: -1,
			Detail:  fmt.Sprintf("%s source %s is not remotely addressable — drift cannot be measured", s.Kind, s.Repo),
		}
	}

	// Belt to Validate()'s braces: a blob-less github source is an UNKNOWN, not
	// a pass. Manifests loaded from disk cannot reach here (Validate rejects
	// them); a programmatically built one still must not get a free in-sync.
	if s.Blob == "" {
		return Result{
			Status:  StatusUnreachable,
			Commits: -1,
			Detail:  fmt.Sprintf("%s: %s has no recorded blob — content cannot be compared, so drift is unknown, not absent", s.Repo, s.Path),
		}
	}

	branch, err := c.DefaultBranch(s.Repo)
	if err != nil {
		return unreachable(s, fmt.Sprintf("default branch: %v", err))
	}

	head, err := c.BlobSHA(s.Repo, s.Path, branch)
	switch {
	case errors.Is(err, ErrNotFound):
		return Result{
			Status:  StatusMoved,
			Commits: -1,
			Detail:  fmt.Sprintf("%s: %s no longer exists on %s (moved, renamed, or deleted since %s)", s.Repo, s.Path, branch, short(s.Commit)),
		}
	case err != nil:
		return unreachable(s, fmt.Sprintf("head blob: %v", err))
	}

	if head == s.Blob {
		return Result{
			Status:  StatusInSync,
			Commits: 0,
			Detail:  fmt.Sprintf("%s: %s on %s is byte-identical to the recorded blob", s.Repo, s.Path, branch),
		}
	}

	// Anchor the commit count on the recorded commit's own date. The recorded
	// commit is the tree the copy was taken from; it need not be a commit that
	// touched this path, so walking path history back to it finds nothing.
	stamp, err := c.CommitDate(s.Repo, s.Commit)
	switch {
	case errors.Is(err, ErrNotFound):
		return unreachable(s, fmt.Sprintf("recorded commit %s does not resolve in %s — SOURCES.yaml may be wrong", short(s.Commit), s.Repo))
	case err != nil:
		return unreachable(s, fmt.Sprintf("recorded commit date: %v", err))
	}
	since, err := afterInstant(stamp)
	if err != nil {
		return unreachable(s, fmt.Sprintf("recorded commit date %q: %v", stamp, err))
	}

	n, capped, err := c.CommitsSince(s.Repo, s.Path, branch, since, maxPages)
	if err != nil {
		return unreachable(s, fmt.Sprintf("commit history: %v", err))
	}

	if n == 0 {
		return Result{
			Status:  StatusBehind,
			Commits: 0,
			Detail: fmt.Sprintf("%s: %s content differs from the recorded blob but no commits touching it were found on %s since %s — the recorded blob may be wrong",
				s.Repo, s.Path, branch, stamp),
		}
	}

	atLeast := ""
	if capped {
		atLeast = "at least "
	}
	return Result{
		Status:  StatusBehind,
		Commits: n,
		Detail: fmt.Sprintf("%s: %s is %s%d commit(s) ahead of %s (as-of %s) on %s",
			s.Repo, s.Path, atLeast, n, short(s.Commit), orUnknown(s.AsOf), branch),
	}
}

// afterInstant returns the RFC3339 timestamp one second after stamp, so that a
// `since` filter excludes the recorded commit itself.
func afterInstant(stamp string) (string, error) {
	t, err := time.Parse(time.RFC3339, stamp)
	if err != nil {
		return "", err
	}
	return t.Add(time.Second).UTC().Format(time.RFC3339), nil
}

func unreachable(s Source, why string) Result {
	return Result{
		Status:  StatusUnreachable,
		Commits: -1,
		Detail:  fmt.Sprintf("%s: %s could not be read — %s", s.Repo, s.Path, why),
	}
}

func short(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

// Summary tallies results by status.
type Summary struct {
	Counts map[string]int
	// Drift is true when any NON-advisory origin is not in-sync. Advisory
	// origins (cross-references) are reported but never decide the verdict.
	Drift bool
	Total int
}

// Summarize tallies results across every origin.
func Summarize(results []Result) Summary {
	s := Summary{Counts: map[string]int{}}
	for _, r := range results {
		s.Counts[r.Status]++
		s.Total++
		if !r.Advisory && r.Status != StatusInSync {
			s.Drift = true
		}
	}
	return s
}

// Verdict renders the trailing machine-readable line.
func (s Summary) Verdict() string {
	keys := make([]string, 0, len(s.Counts))
	for k := range s.Counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s %d", k, s.Counts[k]))
	}
	state := "CLEAN"
	if s.Drift {
		state = "DRIFT"
	}
	return fmt.Sprintf("PLUGINDRIFT: %s (%s) across %d origin(s)", state, strings.Join(parts, ", "), s.Total)
}
