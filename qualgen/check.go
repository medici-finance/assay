package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// check.go is the `check <paths>` mode (spec §9.2): an authoring-time or
// CI-time brittleness screen over a NAMED file set. It joins each screened
// path to the shared FileFeatures assembly (features.go, factored here
// since brief 08's `pr` mode has not yet landed on the branch — either
// order builds it once) and, per file, decides which of four advisory
// NOTICEs apply. It is READ-ONLY against both the tracking root (--out) and
// the target repo (--repo, consulted only for the live reference-rot
// check) and writes NO artifact: the screen result is printed to stdout and
// discarded.
//
// ADVISORY posture (spec §9.2): this mode never gates. Its default exit
// code is a screen result, not a verdict — it returns 0 whether or not a
// file was flagged. Hard gating over this signal is explicitly a later,
// separate decision; this mode carries no non-zero exit path for a flagged
// file at all.

// hotspotAdvisoryPercentile is the relative split the stronger-tier
// advisory fires above: strictly more brittle than the median of the OTHER
// files this screen could rank it against. This is a self-relative signal
// derived from the screened corpus, not a fixed numeric cutoff — like
// brief 08, this mode ships no hard threshold of its own (spec §9.1/§9.2);
// any hard gate is consumer config.
const hotspotAdvisoryPercentile = 0.5

// AdvisoryKind names which of the advisory NOTICEs (spec §9.2) fired.
type AdvisoryKind string

const (
	// AdvisoryStrongerTier: touch this hotspot with a stronger execution tier.
	AdvisoryStrongerTier AdvisoryKind = "stronger-tier"
	// AdvisoryAddCoverage: add coverage over the hotspot's traced defect history.
	AdvisoryAddCoverage AdvisoryKind = "add-coverage"
	// AdvisoryCouplingPartner: an explicit coupling-partner check — name the
	// historical partners not in the screened set.
	AdvisoryCouplingPartner AdvisoryKind = "coupling-partner"
	// AdvisoryReferenceRot: this instruction/config/brief file itself carries
	// a dead reference (spec §4.6, brief 04's reference-validity decay).
	AdvisoryReferenceRot AdvisoryKind = "reference-rot"
)

// Advisory is one NOTICE-level flag on one screened file.
type Advisory struct {
	Kind   AdvisoryKind
	Detail string // human-readable; names the number/partner/reference
}

// FileScreenResult is one screened path's outcome. CouldNotScreen is the
// three-state marker (spec §3.2, fact "three-state"): a path with no
// measurable mined history sets it, distinct from a path that WAS
// measurable and simply raised no advisory.
type FileScreenResult struct {
	Path           string
	CouldNotScreen bool
	Reason         string // populated iff CouldNotScreen
	Advisories     []Advisory
}

// CheckResult is the whole screen's outcome — one FileScreenResult per
// expanded path.
type CheckResult struct {
	Files []FileScreenResult
}

// runCheck is the `check <paths>` mode entry point.
func runCheck(args []string, stdout, stderr io.Writer) int {
	// Leading positionals are the file/glob set to screen (`check a.go b.go --out x`).
	var rawPaths []string
	for len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		rawPaths = append(rawPaths, args[0])
		args = args[1:]
	}
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("out", "", "tracking root to read mined artifacts from (required)")
	repo := fs.String("repo", ".", "path to the git repository the screened paths live in (read-only; consulted only for the live reference-rot check)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *out == "" {
		fmt.Fprintln(stderr, "qualgen check: --out <dir> is required (the tracking root to read mined artifacts from)")
		return 2
	}
	if len(rawPaths) == 0 {
		fmt.Fprintln(stderr, "qualgen check: at least one path or glob is required")
		return 2
	}

	paths, err := expandCheckPaths(*repo, rawPaths)
	if err != nil {
		fmt.Fprintln(stderr, "qualgen check:", err)
		return 1
	}
	if len(paths) == 0 {
		fmt.Fprintln(stderr, "qualgen check: no path matched the given set")
		return 1
	}

	store := NewStore(*out)
	result, err := screenPaths(*repo, store, paths)
	if err != nil {
		fmt.Fprintln(stderr, "qualgen check:", err)
		return 1
	}
	renderCheckResult(stdout, result)

	// Advisory posture (spec §9.2): the screen result above is exactly what
	// this run found, but the mode itself never turns that into a verdict —
	// it returns 0 whether or not any file raised a NOTICE. A consumer that
	// wants to act on the result reads stdout; deciding what to do with it
	// is out of scope here.
	return 0
}

// expandCheckPaths resolves the leading-positional argument set into a
// deduplicated, sorted list of repo-relative paths: a literal path passes
// through unchanged; a glob (containing *, ?, or [) is expanded against
// repoPath and converted back to a slash-separated repo-relative path.
func expandCheckPaths(repoPath string, args []string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, a := range args {
		if !strings.ContainsAny(a, "*?[") {
			add(a)
			continue
		}
		matches, err := filepath.Glob(filepath.Join(repoPath, a))
		if err != nil {
			return nil, fmt.Errorf("invalid glob %q: %w", a, err)
		}
		for _, m := range matches {
			rel, err := filepath.Rel(repoPath, m)
			if err != nil {
				return nil, fmt.Errorf("resolve glob match %q: %w", m, err)
			}
			add(filepath.ToSlash(rel))
		}
	}
	sort.Strings(out)
	return out, nil
}

// screenPaths runs the whole screen: it opens repoPath's HEAD once (for the
// live reference-rot check) and screens every path in the set against
// store's persisted M1/M2 families plus that HEAD tree.
func screenPaths(repoPath string, store *Store, paths []string) (CheckResult, error) {
	r, err := git.PlainOpenWithOptions(repoPath, &git.PlainOpenOptions{EnableDotGitCommonDir: true})
	if err != nil {
		return CheckResult{}, fmt.Errorf("open repository %q: %w", repoPath, err)
	}
	head, err := r.Head()
	if err != nil {
		return CheckResult{}, fmt.Errorf("resolve HEAD: %w", err)
	}
	commit, err := r.CommitObject(head.Hash())
	if err != nil {
		return CheckResult{}, fmt.Errorf("resolve HEAD commit: %w", err)
	}
	typedIDRe, err := regexp.Compile(defaultTypedIDPattern)
	if err != nil {
		return CheckResult{}, err
	}

	screened := map[string]bool{}
	for _, p := range paths {
		screened[p] = true
	}

	var result CheckResult
	for _, p := range paths {
		fr, err := screenFile(store, commit, typedIDRe, p, screened)
		if err != nil {
			return CheckResult{}, err
		}
		result.Files = append(result.Files, fr)
	}
	return result, nil
}

// screenFile joins one path's shared FileFeatures to the four advisory
// rules (spec §9.2, task 2) and the three-state could-not-screen marker
// (task 4).
func screenFile(store *Store, commit *object.Commit, typedIDRe *regexp.Regexp, path string, screened map[string]bool) (FileScreenResult, error) {
	features, err := AssembleFileFeatures(store, path)
	if err != nil {
		return FileScreenResult{}, err
	}

	res := FileScreenResult{Path: path}
	if features.Unmeasured() {
		res.CouldNotScreen = true
		res.Reason = couldNotScreenReason(features)
	}

	// (a) stronger execution tier — this file is above the screen's own
	// relative hotspot signal (median of the corpus it could be ranked
	// against), never a fixed cutoff.
	if features.HotspotPercentile.State == StateMeasured && features.HotspotPercentile.Value > hotspotAdvisoryPercentile {
		res.Advisories = append(res.Advisories, Advisory{
			Kind: AdvisoryStrongerTier,
			Detail: fmt.Sprintf(
				"hotspot percentile %.2f (above the median of this screen's relative signal) — use a stronger execution tier on this change",
				features.HotspotPercentile.Value),
		})
	}

	// (b) add coverage over the hotspot's traced defect history — the
	// density number and its trace-rate travel together (honest-claims).
	if features.DefectDensity.State == StateMeasured && features.DefectDensity.Value > 0 {
		rate := "unmeasured"
		switch features.DefectTraceRate.State {
		case StateMeasured:
			rate = fmt.Sprintf("%.2f", features.DefectTraceRate.Value)
		case StateMeasuredZero:
			rate = "0"
		}
		res.Advisories = append(res.Advisories, Advisory{
			Kind: AdvisoryAddCoverage,
			Detail: fmt.Sprintf(
				"%d traced defect(s) over this file's history (trace-rate %s) — add coverage over its defect history",
				int(features.DefectDensity.Value), rate),
		})
	}

	// (c) explicit coupling-partner check — name the historical partners
	// not also present in the screened set.
	var missing []string
	for _, partner := range features.CouplingPartners {
		if !screened[partner] {
			missing = append(missing, partner)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		res.Advisories = append(res.Advisories, Advisory{
			Kind: AdvisoryCouplingPartner,
			Detail: fmt.Sprintf(
				"historically coupled to %s, not in this screened set — check that partner too",
				strings.Join(missing, ", ")),
		})
	}

	// Reference-rot — a screened instruction/config/brief file with a dead
	// reference (spec §4.6, brief 04's driftdetect capability, reused live
	// against HEAD rather than the repo-wide trended window family, which
	// carries no per-path detail). A path that does not resolve at HEAD at
	// all (e.g. a brand-new, never-committed file) has nothing to check
	// this way — that is not itself an error for the whole screen.
	if refs, err := ResolveDocReferences(commit, path, typedIDRe); err == nil {
		var dead []string
		for _, ref := range refs {
			if ref.Validity.State == StateMeasured && !ref.Validity.Value {
				dead = append(dead, fmt.Sprintf("%s %q", ref.Target.Kind, ref.Token))
			}
		}
		if len(dead) > 0 {
			sort.Strings(dead)
			res.Advisories = append(res.Advisories, Advisory{
				Kind: AdvisoryReferenceRot,
				Detail: fmt.Sprintf(
					"dead reference(s) found in this file: %s (spec §4.6 reference-validity decay)",
					strings.Join(dead, ", ")),
			})
		}
	}

	return res, nil
}

// couldNotScreenReason names why a path carries no measurable mined
// history, preferring the FileFeatures' own most specific reason.
func couldNotScreenReason(f FileFeatures) string {
	if f.HotspotPercentile.State == StateCouldNotMeasure && f.HotspotPercentile.Reason != "" {
		return f.HotspotPercentile.Reason
	}
	return "no measurable mined history for this path"
}

// renderCheckResult writes the screen result as plain NOTICE lines, one per
// advisory, plus a could-not-screen or clean line per file that raised
// none.
func renderCheckResult(w io.Writer, result CheckResult) {
	for _, f := range result.Files {
		if f.CouldNotScreen {
			fmt.Fprintf(w, "%s: could-not-screen — %s\n", f.Path, f.Reason)
		}
		for _, a := range f.Advisories {
			fmt.Fprintf(w, "%s: NOTICE %s — %s\n", f.Path, a.Kind, a.Detail)
		}
		if !f.CouldNotScreen && len(f.Advisories) == 0 {
			fmt.Fprintf(w, "%s: no advisory signal in this screen\n", f.Path)
		}
	}
}
