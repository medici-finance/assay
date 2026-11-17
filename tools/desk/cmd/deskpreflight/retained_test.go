package main

// Tests for the retained-baseline allowlist (issue #2813).
//
// The positive control is that an allowlisted, unchanged, within-bound baseline
// passes. Everything else here is a NEGATIVE control: each one proves the
// exemption does not reach a case it must not — a non-allowlisted oversize file,
// an allowlisted path added on the branch, a baseline changed (committed,
// staged, or in the working tree), a baseline grown past its pinned bound, a
// junk name, an unreadable file, and a base with no merge-base. The real-repo
// cases build a throwaway repository with a real remote-tracking ref and run
// real git; they skip, never fail, when git is absent.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	metricsPath  = "docs/quality/metrics.jsonl"
	baselinePath = "tools/go-analysis/baseline.json"
)

// The compiled allowlist names exactly the two retained baselines, by exact
// path, each with a bound above the global threshold. A new entry is a change a
// reviewer must see; this pins that the list did not quietly widen.
// regression: #2813
func TestRetainedAllowlistExact(t *testing.T) {
	want := map[string]bool{metricsPath: true, baselinePath: true}
	if len(retainedBaselines) != len(want) {
		t.Fatalf("allowlist has %d entries, want %d: %+v", len(retainedBaselines), len(want), retainedBaselines)
	}
	for _, e := range retainedBaselines {
		if !want[e.path] {
			t.Fatalf("unexpected allowlist entry %q", e.path)
		}
		if e.maxBytes <= oversizeBytes {
			t.Fatalf("%s: bound %d is not above the global threshold", e.path, e.maxBytes)
		}
		if strings.ContainsAny(e.path, "*?[") {
			t.Fatalf("%s: allowlist entries are exact paths, never globs", e.path)
		}
	}
	for _, p := range []string{"docs/quality/metrics.jsonl.bak", "x/" + metricsPath, "docs/quality/"} {
		if _, ok := findRetained(retainedBaselines, p); ok {
			t.Fatalf("%q must not match the allowlist", p)
		}
	}
}

// retainedRepo builds a real repository whose base branch (published as
// refs/remotes/origin/main, the fixed base) carries the given files, then cuts
// a feature branch from it with one unrelated commit — the shape of an ordinary
// pull request that does not touch the baselines.
func retainedRepo(t *testing.T, files map[string]int) string {
	t.Helper()
	requireRealTools(t, "git")
	dir := realRepo(t)
	for rel, n := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		writeIncompressible(t, p, n)
	}
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "--allow-empty", "-m", "seed baselines")
	gitIn(t, dir, "update-ref", defaultBase, "HEAD")
	gitIn(t, dir, "checkout", "-q", "-b", "feature")
	if err := os.WriteFile(filepath.Join(dir, "change.txt"), []byte("unrelated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "add", "change.txt")
	gitIn(t, dir, "commit", "-q", "-m", "unrelated change")
	return dir
}

const sixMiB = 6 * 1024 * 1024

// regression: #2813
func TestRetainedUnchangedPasses(t *testing.T) {
	dir := retainedRepo(t, map[string]int{metricsPath: sixMiB, baselinePath: sixMiB})
	c := runJunkOversize(dir, realTools())
	if c.state != deskkit.CheckedClean {
		t.Fatalf("unchanged retained baselines must pass, got %v (%s)", c.state, c.detail)
	}
	for _, p := range []string{metricsPath, baselinePath} {
		if !strings.Contains(c.detail, p) {
			t.Fatalf("the clean line must name the retained baseline %s, got %q", p, c.detail)
		}
	}
}

// A non-allowlisted oversize file that is ALSO unchanged since the base is still
// a hit: being already on main is not, by itself, an exemption.
// regression: #2813
func TestRetainedUnlistedRejected(t *testing.T) {
	dir := retainedRepo(t, map[string]int{metricsPath: sixMiB, "docs/other.jsonl": sixMiB})
	c := runJunkOversize(dir, realTools())
	if c.state != deskkit.CheckedFailed || !strings.Contains(c.detail, "docs/other.jsonl") {
		t.Fatalf("an unlisted oversize file must be rejected, got %v (%s)", c.state, c.detail)
	}
	if strings.Contains(c.detail, "junk/oversize file(s): "+metricsPath) {
		t.Fatalf("the retained baseline must not be reported as a hit, got %q", c.detail)
	}
}

// An allowlisted NAME does not exempt an addition: a file that is not at the
// merge-base is new content, whatever it is called.
// regression: #2813
func TestRetainedAdditionRejected(t *testing.T) {
	dir := retainedRepo(t, nil)
	p := filepath.Join(dir, filepath.FromSlash(metricsPath))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	writeIncompressible(t, p, sixMiB)
	for _, stage := range []string{"untracked", "committed"} {
		if stage == "committed" {
			gitIn(t, dir, "add", metricsPath)
			gitIn(t, dir, "commit", "-q", "-m", "add a large file under a listed name")
		}
		c := runJunkOversize(dir, realTools())
		if c.state != deskkit.CheckedFailed || !strings.Contains(c.detail, metricsPath+" (allowlisted path is not present at the merge-base") {
			t.Fatalf("%s: an addition under an allowlisted name must be rejected, got %v (%s)", stage, c.state, c.detail)
		}
	}
}

// A changed baseline is rejected wherever the change sits: in a branch commit,
// staged, or only in the working tree.
// regression: #2813
func TestRetainedChangedRejected(t *testing.T) {
	for _, where := range []string{"committed", "staged", "working tree"} {
		t.Run(where, func(t *testing.T) {
			dir := retainedRepo(t, map[string]int{metricsPath: sixMiB})
			writeIncompressible(t, filepath.Join(dir, filepath.FromSlash(metricsPath)), sixMiB)
			switch where {
			case "committed":
				gitIn(t, dir, "commit", "-q", "-a", "-m", "edit the baseline")
			case "staged":
				gitIn(t, dir, "add", metricsPath)
			}
			c := runJunkOversize(dir, realTools())
			if c.state != deskkit.CheckedFailed || !strings.Contains(c.detail, metricsPath+" (retained baseline changed") {
				t.Fatalf("a %s change must be rejected, got %v (%s)", where, c.state, c.detail)
			}
		})
	}
}

// A baseline past its pinned bound is rejected even when it is byte-identical
// to the base: growth past the bound is a decision for a reviewer of the list.
// regression: #2813
func TestRetainedPastBoundRejected(t *testing.T) {
	dir := retainedRepo(t, map[string]int{metricsPath: sixMiB})
	tight := []retainedBaseline{{path: metricsPath, maxBytes: sixMiB - 1}}
	c := junkOversize(dir, defaultBase, tight, realTools())
	if c.state != deskkit.CheckedFailed || !strings.Contains(c.detail, "pinned") {
		t.Fatalf("a baseline past its pinned bound must be rejected, got %v (%s)", c.state, c.detail)
	}
	exact := []retainedBaseline{{path: metricsPath, maxBytes: sixMiB}}
	if c := junkOversize(dir, defaultBase, exact, realTools()); c.state != deskkit.CheckedClean {
		t.Fatalf("a baseline exactly at its bound must pass, got %v (%s)", c.state, c.detail)
	}
}

// A junk name is never exempt, even if a list entry names it.
// regression: #2813
func TestRetainedJunkNeverExempt(t *testing.T) {
	dir := retainedRepo(t, map[string]int{"data/keep.orig": sixMiB})
	allow := []retainedBaseline{{path: "data/keep.orig", maxBytes: 16 * 1024 * 1024}}
	c := junkOversize(dir, defaultBase, allow, realTools())
	if c.state != deskkit.CheckedFailed || !strings.Contains(c.detail, "data/keep.orig") {
		t.Fatalf("a junk-named file must be rejected, got %v (%s)", c.state, c.detail)
	}
}

// An unreadable baseline cannot be shown unchanged, so it is could-not-check —
// never an exemption.
// regression: #2813
func TestRetainedUnreadableCNC(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("could-not-check: running as root, file modes do not deny reads")
	}
	dir := retainedRepo(t, map[string]int{metricsPath: sixMiB})
	p := filepath.Join(dir, filepath.FromSlash(metricsPath))
	if err := os.Chmod(p, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
	c := runJunkOversize(dir, realTools())
	if c.state != deskkit.CouldNotCheck {
		t.Fatalf("an unreadable baseline must be could-not-check, got %v (%s)", c.state, c.detail)
	}
}

// With no base to measure "unchanged" against, provenance is unverifiable and
// the check is could-not-check — fail-closed.
// regression: #2813
func TestRetainedNoBaseCNC(t *testing.T) {
	dir := retainedRepo(t, map[string]int{metricsPath: sixMiB})
	gitIn(t, dir, "update-ref", "-d", defaultBase)
	c := runJunkOversize(dir, realTools())
	if c.state != deskkit.CouldNotCheck {
		t.Fatalf("an unresolvable base must be could-not-check, got %v (%s)", c.state, c.detail)
	}
}

// A tree with no oversize file never consults the base at all, so a repository
// without the remote-tracking ref is unaffected by the allowlist.
// regression: #2813
func TestRetainedNoOversizeNoBase(t *testing.T) {
	var calls []string
	c := runJunkOversize("/root", fakeTools(map[string]bool{"git": true},
		func(dir, name string, args ...string) ([]byte, int, bool) {
			calls = append(calls, strings.Join(args, " "))
			if len(args) > 0 && args[0] == "ls-files" {
				return nulJoin("README.md"), 0, true
			}
			return nil, 128, true
		}))
	if c.state != deskkit.CheckedClean {
		t.Fatalf("got %v (%s)", c.state, c.detail)
	}
	if len(calls) != 1 {
		t.Fatalf("only ls-files may run on a tree with no oversize file, ran %v", calls)
	}
}

// Every git command the allowlist path runs is a read: hash-object is never
// handed -w, and nothing outside rev-parse/merge-base/ls-files/hash-object runs.
// regression: #2813
func TestRetainedGitReadOnly(t *testing.T) {
	dir := retainedRepo(t, map[string]int{metricsPath: sixMiB})
	tl := realTools()
	inner := tl.output
	tl.output = func(d, name string, args ...string) ([]byte, int, bool) {
		if name == "git" && len(args) > 0 {
			switch args[0] {
			case "rev-parse", "merge-base", "ls-files":
			case "hash-object":
				for _, a := range args {
					if a == "-w" {
						t.Fatalf("hash-object must never write: %v", args)
					}
				}
			default:
				t.Fatalf("unexpected git subcommand %v", args)
			}
		}
		return inner(d, name, args...)
	}
	if c := runJunkOversize(dir, tl); c.state != deskkit.CheckedClean {
		t.Fatalf("got %v (%s)", c.state, c.detail)
	}
}
