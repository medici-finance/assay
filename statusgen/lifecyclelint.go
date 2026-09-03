package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// §8 spec/scoping-doc lifecycle lint + the authoring-owed detector.
//
// This file holds the two coupled devices spec-routing/01 builds over the §8
// header (see lifecycleheader.go for the reader):
//
//   - the LINTER: an approved/routed document missing `**Routes-to:**` is a
//     PROBLEM (§8.3); an unclassified `**Status:**` first token is a NOTICE
//     (§8.1); an approved document no brief cites is an advisory owed NOTICE
//     (§8.5). Every emitted line carries a stable [rule-tag] so lintaudit.go can
//     attribute it (see statusgen/lintaudit.go — untagged lines fall into the
//     "unattributed:" bucket).
//   - the OWED DETECTOR: documentCited/citing dereference, reused by the
//     --owed-issues emit-mode (owedissues.go).
//
// Three states, always: an unreadable candidate file is could-not-check (a
// NOTICE), never a silent pass; a classified-but-Routes-to-less approved/routed
// doc is a hard PROBLEM; everything else PASSes silently.

// Stable rule tags (statusgen/lintaudit.go convention).
const (
	tagLifecycleRoutesTo     = "lifecycle-routes-to"
	tagLifecycleUnclassified = "lifecycle-unclassified"
	tagLifecycleOwed         = "lifecycle-owed"
	tagLifecycleUnreadable   = "lifecycle-unreadable"
)

// lifecycleCandidateDocs enumerates the markdown files that MAY carry a §8
// header: everything docFiles already collects (docs/** plus CLAUDE.md, minus
// generated register views and declared fixture corpora) PLUS spec/**, the
// canonical spec home docFiles does not walk (it is not under docs/). Only files
// carrying a `**Status:**` line are treated as lifecycle documents downstream;
// scanning a superset is harmless because a non-opted-in file is ignored.
func lifecycleCandidateDocs(root string) (files []string, walkProblems []string) {
	files, walkProblems = docFiles(root)
	specDir := filepath.Join(root, "spec")
	_ = filepath.WalkDir(specDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			// Could-not-check a spec subtree: an absent spec/ is a legitimate
			// empty (skip); anything else is surfaced so the scan fails loud
			// rather than silently enumerating fewer files.
			if !os.IsNotExist(err) {
				walkProblems = append(walkProblems, fmt.Sprintf("spec walk: %s: %v — could-not-check, the §8 lifecycle scan's file set is a floor, not a total [%s]", p, err, tagLifecycleUnreadable))
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(p, ".md") {
			files = append(files, p)
		}
		return nil
	})
	return files, walkProblems
}

// loadLifecycleDocs parses every candidate document and returns the ones that
// opted into the §8 lifecycle (carry a `**Status:**` line), sorted by their
// repo-relative path for deterministic output. A per-file read error is
// could-not-check (a NOTICE), never a silent drop.
func loadLifecycleDocs(root string) (headers []*specLifecycleHeader, notices []string) {
	files, walkProblems := lifecycleCandidateDocs(root)
	notices = append(notices, walkProblems...)
	seen := map[string]bool{}
	for _, f := range files {
		if seen[f] {
			continue
		}
		seen[f] = true
		h, err := parseSpecLifecycleHeader(root, f)
		if err != nil {
			rel := f
			if r, e := filepath.Rel(root, f); e == nil {
				rel = filepath.ToSlash(r)
			}
			notices = append(notices, fmt.Sprintf("%s: could-not-check §8 lifecycle header: %v [%s]", rel, err, tagLifecycleUnreadable))
			continue
		}
		if !h.HasStatus {
			continue // not opted into the §8 lifecycle — ignored (never defaulted up)
		}
		headers = append(headers, h)
	}
	sort.Slice(headers, func(i, j int) bool { return headers[i].Rel < headers[j].Rel })
	return headers, notices
}

// allBriefSources collects the `sources:` entries of every brief-v1 file across
// the given streams — the authored provenance the §8.5 citation rule
// dereferences against. Malformed/legacy files are skipped (reported elsewhere).
func allBriefSources(streams []*Stream) []string {
	var out []string
	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok {
				continue
			}
			out = append(out, bf.Sources...)
		}
	}
	return out
}

// documentCited implements the §8.5 citation rule: a document is cited when at
// least one brief's `sources:` frontmatter CONTAINS the document's repo-relative
// path. It is a dereference against authored provenance, not a title match — a
// prose mention of the title, or a citation of the title without its path, does
// NOT count, because neither contains the path token.
func documentCited(rel string, sources []string) bool {
	for _, src := range sources {
		if sourceContainsPath(src, rel) {
			return true
		}
	}
	return false
}

// sourceContainsPath reports whether a source string contains `rel` as a path
// token — matched on a path boundary so a document is not falsely counted as
// cited by a citation of a DIFFERENT, longer path it is a substring of
// (`spec/lifecycle-v1.md` must not match `spec/lifecycle-v1.md-notes` or
// `sub/spec/lifecycle-v1.md`). Backticks, spaces, `§`, `#`, quotes and
// parentheses are all boundaries, so the ordinary written forms
// (`+"`spec/lifecycle-v1.md` §8"+`) match.
func sourceContainsPath(source, rel string) bool {
	if rel == "" {
		return false
	}
	for from := 0; ; {
		i := strings.Index(source[from:], rel)
		if i < 0 {
			return false
		}
		i += from
		beforeOK := i == 0 || !isPathTokenByte(source[i-1])
		end := i + len(rel)
		afterOK := end >= len(source) || !isPathTokenByte(source[end])
		if beforeOK && afterOK {
			return true
		}
		from = i + 1
	}
}

// isPathTokenByte reports whether a byte can extend a path token — the boundary
// alphabet for sourceContainsPath.
func isPathTokenByte(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9':
		return true
	}
	switch b {
	case '/', '.', '-', '_':
		return true
	}
	return false
}

// lifecycleLintChecks runs the §8 lint over the repo's spec/scoping documents.
// It returns BLOCKING problems (an approved/routed document with no
// `**Routes-to:**`) and advisory notices (unclassified headers; the
// authoring-owed condition; could-not-check reads). `streams` is the FULL house
// set, so citation is evaluated against every brief regardless of product scope.
func lifecycleLintChecks(root string, streams []*Stream) (problems, notices []string) {
	headers, readNotices := loadLifecycleDocs(root)
	notices = append(notices, readNotices...)
	sources := allBriefSources(streams)

	for _, h := range headers {
		if !h.Classified() {
			// §8.1: an unparseable first token is unclassified (legacy). At most a
			// NOTICE — it is ignored, never rounded up to a real state (§8.6).
			notices = append(notices, fmt.Sprintf("%s: **Status:** first token %q is not one of draft/approved/routed — the document is unclassified (legacy) and is ignored, not defaulted up to a real state (spec/lifecycle-v1.md §8.1) [%s]", h.Rel, h.StateRaw, tagLifecycleUnclassified))
			continue
		}
		// §8.3: approved/routed MUST carry a `**Routes-to:**` destination. Presence
		// is the control; whether the destination is a GOOD stream stays review.
		if (h.State == lifecycleApproved || h.State == lifecycleRouted) && !h.Routes {
			problems = append(problems, fmt.Sprintf("%s: **Status:** %s but no **Routes-to:** destination — an approved/routed document MUST name the stream directory the work routes into (spec/lifecycle-v1.md §8.3) [%s]", h.Rel, h.State, tagLifecycleRoutesTo))
		}
		// §8.5: an approved document that no brief cites has authoring OWED. Only
		// approved documents are owed-candidates (a draft watches nothing; a routed
		// document is by definition already cited). Advisory NOTICE — the emitter
		// (--owed-issues) is what files the work-ready issue.
		if h.State == lifecycleApproved && !documentCited(h.Rel, sources) {
			notices = append(notices, fmt.Sprintf("%s: approved but no brief's sources: dereferences its path — brief-authoring is OWED (spec/lifecycle-v1.md §8.5); file the work-ready issue with `statusgen --owed-issues` [%s]", h.Rel, tagLifecycleOwed))
		}
	}
	return problems, notices
}
