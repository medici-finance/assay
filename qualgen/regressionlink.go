package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/medici-finance/assay/qualgen/adapters"
)

// regressionlink.go — the RegressionLinkage seam (brief quality/19, Task 1): the
// two paths that can tie a traced fix F to an earlier fix E for the re-fix
// metric (refix.go). Mirrors fixlinkage.go's LinkageAdapter convention exactly:
// an interface plus a reference adapter, with an adapter error mapped to
// could-not-measure and NEVER to a silent "no link" (spec-equivalent honesty
// discipline carried over from quality/06).

// RegressionRef identifies the earlier fix E that a linkage path names. Exactly
// one of Issue / CommitSHA is populated by the reference adapter today (the
// frontmatter format accepts an issue ref or a commit sha — see fact "the exact
// value format" below); PRNumber is carried for a caller (or a future linkage
// source) that resolves straight to a PR, and refMatches in refix.go matches on
// whichever field is set.
type RegressionRef struct {
	Issue     *IssueRef
	CommitSHA string
	PRNumber  int
}

// RegressionLinkage is the pluggable seam (brief quality/19 Task 1) resolving
// the two paths that can link a traced fix F to an earlier fix E:
//   - RegressionOf: F's brief's explicit `regression-of:` naming (author-brief
//     SKILL.md rule 14) — the prior fix's issue reference or commit sha.
//   - DefectClass: the defect-class key read off an issue's labels through a
//     CONFIGURED label prefix (e.g. "class:"); two fixes whose closed issues
//     carry the SAME class key are linked. There is no built-in default prefix
//     (spec: "Unconfigured, class linkage is could-not-measure and never 'no
//     class'") — the reference adapter enforces that by erroring, never by
//     silently answering ok=false, when it has no prefix configured.
//
// A non-nil error from either method means the linkage could not be resolved
// (an unreachable brief file, a rate-limited/permission-denied label read, an
// unconfigured prefix) — refix.go turns that into could-not-measure, never a
// silent non-link. ok=false with a nil error means the check ran cleanly and
// found nothing: a genuine, measured absence.
type RegressionLinkage interface {
	RegressionOf(f DefectFix) (refs []RegressionRef, ok bool, err error)
	DefectClass(ref IssueRef) (class string, ok bool, err error)
}

// briefPathPattern matches a stream brief file's committed path
// (docs/streams/<stream>/brief-<NN>-<slug>.md — this repo's own convention;
// this brief's own file, docs/streams/quality/brief-19-refix-metric.md, is an
// instance of it).
var briefPathPattern = regexp.MustCompile(`^docs/streams/[^/]+/brief-\d+-[^/]+\.md$`)

// briefTrailerPattern matches the `Brief: <stream>/<NN>` or `Brief: <stream>:<NN>`
// trailer a worker's fix commit/PR body carries (deskpr's own trailer — see
// tools/desk/cmd/deskboard's owning-brief resolution, which this mirrors for the
// two accepted separator forms).
var briefTrailerPattern = regexp.MustCompile(`(?m)^Brief:\s*([A-Za-z0-9][A-Za-z0-9_-]*)[/:](\d+)\s*$`)

// regressionOfPattern reads the `regression-of:` frontmatter scalar (author-brief
// SKILL.md rule 14's format: `regression-of: <issue ref or sha>`) out of a
// brief's frontmatter block only — never a `regression-of` occurrence in the
// prose body, which frontmatterBlock excludes.
var regressionOfPattern = regexp.MustCompile(`(?m)^regression-of:\s*(.+?)\s*$`)

// issueRefPattern / repoIssueRefPattern / shaPattern classify a `regression-of:`
// value into the two forms the pickup precondition documents it accepts: a bare
// `#N` or `owner/repo#N` issue reference, or a commit sha.
var (
	repoIssueRefPattern = regexp.MustCompile(`^([\w.-]+/[\w.-]+)#(\d+)$`)
	issueRefPattern     = regexp.MustCompile(`^#(\d+)$`)
	shaPattern          = regexp.MustCompile(`^[0-9a-fA-F]{7,40}$`)
)

// BriefRegressionLinkage is the reference RegressionLinkage adapter (Task 1):
//   - RegressionOf reads `regression-of:` from the brief(s) tied to the fix —
//     the brief named by the fix commit message's `Brief:` trailer, or any
//     brief file the fix commit touches — at the FIX COMMIT'S TREE (so a
//     later edit to the brief file never changes what an already-landed fix is
//     read as having said).
//   - DefectClass reads the configured label prefix through the shared
//     adapters.IssueLabelSource — the same seam fixlinkage.go's
//     GithubLabelsLinkage already reads issue labels through.
type BriefRegressionLinkage struct {
	Repo   *git.Repository
	Labels adapters.IssueLabelSource
	// ClassLabelPrefix configures the class-key label prefix (e.g. "class:").
	// Empty means UNCONFIGURED: DefectClass always errors (could-not-measure),
	// never answers a silent "no class" — this is the fact 3's requirement,
	// enforced here rather than left to refix.go to remember.
	ClassLabelPrefix string
}

func (b BriefRegressionLinkage) RegressionOf(f DefectFix) ([]RegressionRef, bool, error) {
	commit, err := b.Repo.CommitObject(plumbing.NewHash(f.FixCommitSHA))
	if err != nil {
		return nil, false, fmt.Errorf("regressionlink: resolve fix commit %s: %w", f.FixCommitSHA, err)
	}

	var briefPaths []string
	if stream, nn, ok := parseBriefTrailer(commit.Message); ok {
		p, err := findBriefFile(commit, stream, nn)
		if err != nil {
			return nil, false, fmt.Errorf("regressionlink: locate brief %s/%s at fix commit %s's tree: %w", stream, nn, f.FixCommitSHA, err)
		}
		if p != "" {
			briefPaths = append(briefPaths, p)
		}
	}
	if len(briefPaths) == 0 {
		// No (or no resolvable) Brief: trailer — fall back to any brief file
		// this commit itself TOUCHED (its parent diff), never every brief file
		// that happens to be lying around in the tree; that would attribute
		// regression-of from an unrelated brief.
		touched, err := touchedBriefFiles(commit)
		if err != nil {
			return nil, false, fmt.Errorf("regressionlink: scan fix commit %s's changed files: %w", f.FixCommitSHA, err)
		}
		briefPaths = touched
	}

	var refs []RegressionRef
	for _, p := range briefPaths {
		content, err := readFileAtCommit(commit, p)
		if err != nil {
			return nil, false, fmt.Errorf("regressionlink: read %s at fix commit %s's tree: %w", p, f.FixCommitSHA, err)
		}
		v, ok := regressionOfValue(content)
		if !ok {
			continue
		}
		if ref, ok := parseRegressionRef(v); ok {
			refs = append(refs, ref)
		}
	}
	if len(refs) == 0 {
		return nil, false, nil
	}
	return refs, true, nil
}

func (b BriefRegressionLinkage) DefectClass(ref IssueRef) (string, bool, error) {
	if strings.TrimSpace(b.ClassLabelPrefix) == "" {
		return "", false, fmt.Errorf("regressionlink: defect-class label prefix not configured — class linkage is could-not-measure, never 'no class'")
	}
	labels, err := b.Labels.IssueLabels(ref.Repo, ref.Number)
	if err != nil {
		return "", false, fmt.Errorf("regressionlink: reading labels for issue #%d: %w", ref.Number, err)
	}
	prefix := strings.ToLower(strings.TrimSpace(b.ClassLabelPrefix))
	for _, l := range labels {
		ll := strings.ToLower(strings.TrimSpace(l))
		if !strings.HasPrefix(ll, prefix) {
			continue
		}
		class := strings.TrimSpace(strings.TrimPrefix(ll, prefix))
		if class != "" {
			return class, true, nil
		}
	}
	return "", false, nil
}

// parseBriefTrailer extracts the `Brief: <stream>/<NN>` (or `:`-separated)
// trailer from a commit message, if present.
func parseBriefTrailer(msg string) (stream, nn string, ok bool) {
	m := briefTrailerPattern.FindStringSubmatch(msg)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

// findBriefFile locates the brief file for (stream, nn) at commit's tree —
// docs/streams/<stream>/brief-<nn>-*.md.
func findBriefFile(commit *object.Commit, stream, nn string) (string, error) {
	tree, err := commit.Tree()
	if err != nil {
		return "", err
	}
	prefix := fmt.Sprintf("docs/streams/%s/brief-%s-", stream, nn)
	var found string
	err = tree.Files().ForEach(func(f *object.File) error {
		if found == "" && strings.HasPrefix(f.Name, prefix) && strings.HasSuffix(f.Name, ".md") {
			found = f.Name
		}
		return nil
	})
	return found, err
}

// touchedBriefFiles returns the brief-file paths (matching briefPathPattern)
// that commit's own diff against its first parent touched (added/modified). A
// root commit's whole tree is treated as "touched" (there is no parent to diff
// against).
func touchedBriefFiles(commit *object.Commit) ([]string, error) {
	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}
	if commit.NumParents() == 0 {
		var paths []string
		err := tree.Files().ForEach(func(f *object.File) error {
			if briefPathPattern.MatchString(f.Name) {
				paths = append(paths, f.Name)
			}
			return nil
		})
		return paths, err
	}
	parent, err := commit.Parent(0)
	if err != nil {
		return nil, err
	}
	parentTree, err := parent.Tree()
	if err != nil {
		return nil, err
	}
	changes, err := object.DiffTree(parentTree, tree)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, ch := range changes {
		to := ch.To.Name
		if to != "" && briefPathPattern.MatchString(to) {
			paths = append(paths, to)
		}
	}
	return paths, nil
}

// readFileAtCommit reads path's content as it stood in commit's tree.
func readFileAtCommit(commit *object.Commit, path string) (string, error) {
	f, err := commit.File(path)
	if err != nil {
		return "", err
	}
	return f.Contents()
}

// frontmatterBlock returns the YAML frontmatter body (between the first two
// `---` delimiter lines), or "" when content does not open with one — so
// regressionOfValue never matches a `regression-of` mention in the prose body.
func frontmatterBlock(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n")
		}
	}
	return ""
}

// regressionOfValue reads the `regression-of:` scalar out of content's
// frontmatter.
func regressionOfValue(content string) (string, bool) {
	fm := frontmatterBlock(content)
	if fm == "" {
		return "", false
	}
	m := regressionOfPattern.FindStringSubmatch(fm)
	if m == nil {
		return "", false
	}
	v := strings.Trim(strings.TrimSpace(m[1]), `"'`)
	if v == "" {
		return "", false
	}
	return v, true
}

// parseRegressionRef classifies a `regression-of:` value into an issue
// reference (`#N` or `owner/repo#N`) or a commit sha — the two forms the
// pickup precondition documents.
func parseRegressionRef(v string) (RegressionRef, bool) {
	if m := repoIssueRefPattern.FindStringSubmatch(v); m != nil {
		n, err := strconv.Atoi(m[2])
		if err != nil {
			return RegressionRef{}, false
		}
		return RegressionRef{Issue: &IssueRef{Repo: m[1], Number: n}}, true
	}
	if m := issueRefPattern.FindStringSubmatch(v); m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return RegressionRef{}, false
		}
		return RegressionRef{Issue: &IssueRef{Number: n}}, true
	}
	if shaPattern.MatchString(v) {
		return RegressionRef{CommitSHA: strings.ToLower(v)}, true
	}
	return RegressionRef{}, false
}
