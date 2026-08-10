package main

// multiroot.go — multi-repo board support (assay-selfcontain/01).
//
// statusgen was born single-root: one `--root`, one `docs/streams/`, one
// STATUS.md. The methodology now spans two repos, so `--root` is REPEATABLE and
// emits one STATUS.md per root, each filtered to that root's own streams.
//
// Two design rules hold the whole thing up:
//
//  1. PER-ROOT, NEVER roots[0]. Every derived datum — findings, intake alarms,
//     awaiting ages, briefTouch, Next-up counts, register integrity, budget,
//     link/register-ref lint — is computed from THAT root's own registers,
//     historian and doc tree. Multi-root is implemented as the single-root
//     pipeline run once per root (see runRoots), not as a merged universe with
//     first-root registers bolted on: a merged universe silently attributes one
//     repo's findings to another repo's briefs, and a stream that quietly stops
//     appearing on Next-up is invisible by construction.
//
//  2. UNAMBIGUOUS OWNERSHIP. A stream lives in exactly ONE repo. A stream name
//     appearing under two roots is an authoring error and a HARD ERROR — never a
//     silent merge or a last-one-wins overwrite of a STATUS.md row. The optional
//     `repo:` frontmatter records the owning repo explicitly; it is validated
//     (form + intra-root agreement + cross-root uniqueness) and surfaced in
//     STATUS.md and `--gate-scores`, so it can never rot into dead metadata.
//
// Single-root runs take the ORIGINAL code path unchanged (run() is untouched by
// this file except for the repo: validation, which is inert when no stream
// declares one) — the board is load-bearing, so 1-arg behavior stays
// byte-identical.

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// rootFlags is the repeatable --root flag. Its zero value means "no --root was
// given", which callers resolve to the historical default of ".".
type rootFlags []string

func (r *rootFlags) String() string { return strings.Join(*r, ", ") }

func (r *rootFlags) Set(v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		return fmt.Errorf("--root: empty value")
	}
	*r = append(*r, v)
	return nil
}

// resolveRoots applies the historical default: no --root means ".". Duplicate
// roots are a usage error — running the same root twice would write its
// STATUS.md twice and double-count it on any aggregate.
func resolveRoots(r rootFlags) ([]string, error) {
	if len(r) == 0 {
		return []string{"."}, nil
	}
	seen := map[string]bool{}
	for _, root := range r {
		if seen[root] {
			return nil, fmt.Errorf("--root %q given twice — each root must be distinct", root)
		}
		seen[root] = true
	}
	return []string(r), nil
}

// repoFrontmatterRe is the accepted form of a `repo:` value: <owner>/<name>,
// the same shape GitHub uses and the same shape the deskboard keys its
// configured-roots table on.
var repoFrontmatterRe = regexp.MustCompile(`^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$`)

// rootRepo returns the repo a root's streams declare via `repo:` frontmatter,
// plus any problems with those declarations. "" means no stream declared one
// (the field is optional — a single-repo house never needs it).
//
// Problems are hard PROBLEMs, wired into the per-root check pass so `repo:` is
// LIVE metadata rather than a decorative field:
//   - a malformed value (not <owner>/<name>);
//   - two streams under the SAME root naming different repos — one root is one
//     repo, so disagreement means at least one of them is wrong.
func rootRepo(streams []*Stream) (string, []string) {
	var problems []string
	byRepo := map[string][]string{}
	for _, s := range streams {
		if s.Repo == "" {
			continue
		}
		if !repoFrontmatterRe.MatchString(s.Repo) {
			problems = append(problems, fmt.Sprintf(
				"%s: invalid repo %q — want <owner>/<name>, the repo this stream lives in (assay-selfcontain/01)",
				s.Name, s.Repo))
			continue
		}
		byRepo[s.Repo] = append(byRepo[s.Repo], s.Name)
	}
	if len(byRepo) > 1 {
		names := make([]string, 0, len(byRepo))
		for repo, streamNames := range byRepo {
			sort.Strings(streamNames)
			names = append(names, fmt.Sprintf("%s (%s)", repo, strings.Join(streamNames, ", ")))
		}
		sort.Strings(names)
		problems = append(problems, fmt.Sprintf(
			"conflicting repo: declarations under one root — %s; every stream under a root belongs to the same repo (assay-selfcontain/01)",
			strings.Join(names, " vs ")))
	}
	sort.Strings(problems)
	if len(byRepo) != 1 {
		return "", problems
	}
	for repo := range byRepo {
		return repo, problems
	}
	return "", problems
}

// crossRootProblems is the multi-root pre-pass: the checks that can only be made
// by looking at more than one root at once. It runs BEFORE any per-root
// pipeline, because a duplicate stream name means the roots disagree about who
// owns a stream and every downstream number is then attributed to the wrong
// repo.
//
// Roots that fail to LOAD are skipped here on purpose: the per-root pipeline
// reports load errors with full context, and reporting them twice would make an
// unreadable root look like two separate faults.
func crossRootProblems(roots []string) []string {
	var problems []string
	ownerOfStream := map[string]string{} // stream name → first root that defined it
	ownerOfRepo := map[string]string{}   // declared repo → first root that declared it
	for _, root := range roots {
		streams, _, err := loadStreams(root)
		if err != nil {
			continue // reported, with context, by this root's own pipeline
		}
		for _, s := range streams {
			if prev, ok := ownerOfStream[s.Name]; ok {
				problems = append(problems, fmt.Sprintf(
					"stream %q is defined under two roots (%s and %s) — a stream lives in exactly one repo; move or rename one, never let both boards claim it (assay-selfcontain/01)",
					s.Name, prev, root))
				continue
			}
			ownerOfStream[s.Name] = root
		}
		repo, _ := rootRepo(streams) // malformed/conflicting values are reported per-root
		if repo == "" {
			continue
		}
		if prev, ok := ownerOfRepo[repo]; ok {
			problems = append(problems, fmt.Sprintf(
				"repo %q is declared by two roots (%s and %s) — each root is a distinct repo; fix the repo: frontmatter (assay-selfcontain/01)",
				repo, prev, root))
			continue
		}
		ownerOfRepo[repo] = root
	}
	sort.Strings(problems)
	return problems
}
