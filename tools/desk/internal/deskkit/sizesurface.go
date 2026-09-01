package deskkit

// sizesurface.go — the two MECHANICAL, verdict-time PR labels: a diff SIZE class and a
// SURFACE tier. Both are advisory triage aids for the single human merge gate, never a
// gate themselves.
//
// WHAT THIS IS, AND WHAT IT DELIBERATELY IS NOT. Every approved PR arrives in the human's
// merge queue looking identical; two mechanical labels let that human order the queue by
// exposure instead of by arrival. The labels are DERIVED, not judged — a size class is
// `wc -l` over the changed lines, a surface tier is a glob match over the changed paths —
// so a reviewer can recompute either by hand and a wrong label traces to a wrong input,
// not to a hidden model. There is NO scoring here: no numeric risk score, no signed
// verdict payload, no model. That is a different, currently-inert lane; this file must
// never grow toward it. If a change here starts to weigh, rank, or score rather than
// classify by `wc -l` + glob, it has left this file's remit.
//
// SIZE. The changed-line count is additions+deletions summed over the NON-generated files
// of the diff (generated/vendored churn is excluded so a regenerated lockfile does not
// inflate the class — IsGeneratedPath). Classes: size:S < 100, size:M < 400, size:L ≥ 400.
//
// SURFACE, and its THREE states. A repo declares its risk surfaces in a reviewed
// `.assay-surfaces` file (one glob per line). The tier is:
//
//	surface:core   at least one changed path matched a declared surface glob. The applied
//	               label is accompanied by a comment naming the matched globs, so the human
//	               can spot a misclassification at a glance.
//	surface:std    the config is present and NO changed path matched it.
//	(no label)     the repo declares no `.assay-surfaces` at all — SurfaceUnknown, the zero
//	               value. This is NEVER guessed as std: a repo that never declared its
//	               surfaces has said nothing about them, so the labeler says nothing either.
//
// The single-point-of-failure is a wrong label misleading the one human the whole system
// routes through; the layers behind it are (a) the labels are a deterministic function a
// reviewer recomputes, (b) the surface list is a diffable reviewed file, and (c) the
// core-tier comment names the matched globs. The zero-value SurfaceUnknown is the fail-safe
// underneath the whole step: a broken or absent config yields no surface label, never a
// wrong one.

import (
	"bufio"
	"bytes"
	"sort"
	"strings"
)

// SizeLabelPrefix and SurfaceLabelPrefix are the ONE spelling of each label family. The
// labeler builds every label through the constructors below and strips a stale label of a
// family by this prefix, so no consumer restates the string and the replace-not-duplicate
// idempotence is keyed on the family, not on an enumerated value list that could drift.
const (
	SizeLabelPrefix    = "size:"
	SurfaceLabelPrefix = "surface:"

	// SurfaceCoreLabel / SurfaceStdLabel are the two surface labels a repo WITH an
	// `.assay-surfaces` file can carry. A repo without one carries neither (SurfaceUnknown).
	SurfaceCoreLabel = SurfaceLabelPrefix + "core"
	SurfaceStdLabel  = SurfaceLabelPrefix + "std"
)

// Size-class thresholds, in changed lines (additions+deletions, generated files excluded).
// S is < sizeSmallMax, M is < sizeMediumMax, L is everything at or above sizeMediumMax.
const (
	sizeSmallMax  = 100
	sizeMediumMax = 400
)

// SizeClassLabel returns the size label for a changed-line count. The count is
// additions+deletions over the non-generated files of the diff (see ChangedLineCount). A
// negative count cannot occur from a real diff, but is clamped to the small class rather
// than panicking — a mislabel is advisory, an abort at verdict time is not.
func SizeClassLabel(changedLines int) string {
	switch {
	case changedLines < sizeSmallMax:
		return SizeLabelPrefix + "S"
	case changedLines < sizeMediumMax:
		return SizeLabelPrefix + "M"
	default:
		return SizeLabelPrefix + "L"
	}
}

// FileDelta is one changed file's contribution to the size count: its path (for the
// generated-exclusion test) and its additions+deletions. It mirrors the two fields the
// GitHub /pulls/{n}/files entries carry that this classifier consumes.
type FileDelta struct {
	Path    string
	Changed int // additions + deletions
}

// ChangedLineCount sums additions+deletions across the diff, EXCLUDING generated/vendored
// files (IsGeneratedPath). This is the input to SizeClassLabel — a regenerated lockfile or
// a vendored tree must not inflate the class the human triages on.
func ChangedLineCount(files []FileDelta) int {
	total := 0
	for _, f := range files {
		if IsGeneratedPath(f.Path) {
			continue
		}
		if f.Changed > 0 {
			total += f.Changed
		}
	}
	return total
}

// SurfaceState is the three-state answer to "does this PR touch a declared risk surface?".
type SurfaceState int

const (
	// SurfaceUnknown is the ZERO VALUE and means the repo declares NO `.assay-surfaces`
	// file. It carries no surface label — never guessed as std. A repo that never declared
	// its surfaces has said nothing about them.
	SurfaceUnknown SurfaceState = iota
	// SurfaceStd means the config is present and no changed path matched it.
	SurfaceStd
	// SurfaceCore means at least one changed path matched a declared surface glob.
	SurfaceCore
)

func (s SurfaceState) String() string {
	switch s {
	case SurfaceCore:
		return "core"
	case SurfaceStd:
		return "std"
	default:
		return "unknown"
	}
}

// Label returns the surface label for the state and whether one exists. SurfaceUnknown has
// no label (ok=false) — a consumer that applies s.Label() unconditionally still applies
// nothing for the unknown state rather than a guessed default.
func (s SurfaceState) Label() (string, bool) {
	switch s {
	case SurfaceCore:
		return SurfaceCoreLabel, true
	case SurfaceStd:
		return SurfaceStdLabel, true
	default:
		return "", false
	}
}

// ParseSurfaceGlobs parses `.assay-surfaces` content into its glob patterns. The file is
// one glob per line; a line whose first non-space character is '#' is a comment, and blank
// lines are skipped. Surrounding whitespace is trimmed. Inline trailing comments are NOT
// stripped (a path may legitimately contain '#'), so a comment must be on its own line.
func ParseSurfaceGlobs(data []byte) []string {
	var out []string
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

// ClassifySurface decides the surface tier for a set of changed paths.
//
// cfgPresent reports whether the repo declares an `.assay-surfaces` file at all. When it is
// false the answer is SurfaceUnknown regardless of the paths — the absent-config state is
// distinct from "present but nothing matched" and MUST NOT be collapsed into SurfaceStd.
//
// When cfgPresent is true: every changed path is tested against every glob; if any pair
// matches, the state is SurfaceCore and the returned slice names the DISTINCT globs that
// matched at least one path (sorted), for the accompanying comment. Otherwise SurfaceStd
// with a nil slice. A blank path entry never matches and is simply skipped — surface labels
// are advisory, so a malformed listing degrades to std, not to a fail-closed core.
func ClassifySurface(cfgPresent bool, globs, changedFiles []string) (SurfaceState, []string) {
	if !cfgPresent {
		return SurfaceUnknown, nil
	}
	matched := map[string]bool{}
	for _, g := range globs {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		for _, p := range changedFiles {
			if strings.TrimSpace(p) == "" {
				continue
			}
			if MatchSurfaceGlob(g, p) {
				matched[g] = true
				break
			}
		}
	}
	if len(matched) == 0 {
		return SurfaceStd, nil
	}
	names := make([]string, 0, len(matched))
	for g := range matched {
		names = append(names, g)
	}
	sort.Strings(names)
	return SurfaceCore, names
}

// defaultGeneratedGlobs is the minimal, house-agnostic default set of generated/vendored
// path globs excluded from the size count. It is deliberately conservative — a full
// linguist `.gitattributes` (`linguist-generated`) parse is a documented follow-up, not
// this mechanical step's job — and covers the churn that most distorts a size class:
// vendored trees, protobuf output, dependency lockfiles, and minified assets.
var defaultGeneratedGlobs = []string{
	"vendor/**",
	"**/vendor/**",
	"**/*.pb.go",
	"**/go.sum",
	"**/package-lock.json",
	"**/yarn.lock",
	"**/pnpm-lock.yaml",
	"**/*.min.js",
	"**/*.min.css",
	"**/*_generated.go",
	"**/zz_generated.*",
	"**/generated/**",
}

// IsGeneratedPath reports whether a path is generated/vendored and so excluded from the
// size count. It matches the conservative defaultGeneratedGlobs with the same
// MatchSurfaceGlob syntax the surface tier uses.
func IsGeneratedPath(path string) bool {
	for _, g := range defaultGeneratedGlobs {
		if MatchSurfaceGlob(g, path) {
			return true
		}
	}
	return false
}

// MatchSurfaceGlob reports whether a slash-separated path matches an `.assay-surfaces`
// glob. The syntax is a deliberately small, gitignore-flavoured subset:
//
//	*   matches any run of non-'/' characters WITHIN one path segment (including empty)
//	**  as a whole segment matches zero or more path segments
//
// Every other character is literal; leading and trailing '/' are ignored. This is a RICHER
// syntax than deskkit's risk-path matchTrigger (single-segment '*', trailing-'/' directory
// prefix) on PURPOSE: the `.assay-surfaces` contract uses '**' and in-segment wildcards
// like '*guard*', neither of which matchTrigger can express. The two matchers are kept
// separate rather than widening the security gate's — surface labels are advisory and
// never gate a flip, so their matcher carries no fail-closed obligation.
func MatchSurfaceGlob(pattern, path string) bool {
	return globSegs(splitGlobPath(pattern), splitGlobPath(path))
}

func splitGlobPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

// globSegs matches a pattern's segments against a path's segments, with '**' spanning zero
// or more segments.
func globSegs(pat, name []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			// Collapse consecutive '**' — they are equivalent to one.
			for len(pat) > 1 && pat[1] == "**" {
				pat = pat[1:]
			}
			// A trailing '**' matches whatever remains, including nothing.
			if len(pat) == 1 {
				return true
			}
			// Otherwise try to consume 0..len(name) segments here.
			for i := 0; i <= len(name); i++ {
				if globSegs(pat[1:], name[i:]) {
					return true
				}
			}
			return false
		}
		if len(name) == 0 {
			return false
		}
		if !matchSeg(pat[0], name[0]) {
			return false
		}
		pat, name = pat[1:], name[1:]
	}
	return len(name) == 0
}

// matchSeg matches ONE segment pattern (with '*' wildcards) against one path segment. '*'
// matches any run of characters (the caller has already split on '/', so a segment never
// contains one). It is the classic linear wildcard match with backtracking on the last
// star, which is linear in practice for the short segments a path carries.
func matchSeg(pat, s string) bool {
	px, sx := 0, 0
	star, mark := -1, 0
	for sx < len(s) {
		switch {
		case px < len(pat) && pat[px] == s[sx]:
			px++
			sx++
		case px < len(pat) && pat[px] == '*':
			star = px
			mark = sx
			px++
		case star != -1:
			px = star + 1
			mark++
			sx = mark
		default:
			return false
		}
	}
	for px < len(pat) && pat[px] == '*' {
		px++
	}
	return px == len(pat)
}
