package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// --owed-issues — the authoring-owed emitter (spec-routing/01, spec/lifecycle-v1.md §8.5).
//
// A NEW member of the marker-deduped issue-emitter family (decisionissues.go,
// verifyissues.go): a hidden per-item `<!-- … -->` idempotency marker rendered
// as the first body line, an existing-markers set loaded from the open issues
// (tolerant of raw `gh issue list --json body` output), and a JSON payload array
// a workflow feeds straight to `gh issue create`. This reuses that shape exactly
// — it does NOT invent a second dedup convention.
//
// One issue per approved-but-uncited spec/scoping document. Re-running the
// emitter while the issue is already open emits nothing (its marker is in the
// existing set), so there is exactly one open owed issue per owed document.

// authoringOwedLabel marks the emitted issue as a work-ready authoring-owed unit
// (author a brief whose `sources:` dereferences the document). Deliberately NOT
// a system_state / decision_owed label (topology.yaml): the owed issue is a unit
// of dispatchable work, not a closeable desk state or a human-decision fork.
const authoringOwedLabel = "authoring-owed"

// owedIssue is one emitted element of the --owed-issues JSON array — a
// self-contained GitHub issue payload for an approved-but-uncited document. The
// workflow feeds title/labels/body straight to `gh issue create`; marker is the
// idempotency key.
type owedIssue struct {
	Doc    string   `json:"doc"` // the document's repo-relative path (the marker key)
	Title  string   `json:"title"`
	Labels []string `json:"labels"`
	Marker string   `json:"marker"`
	Body   string   `json:"body"`
}

// owedMarkerRe matches a hidden authoring-owed marker in an issue body (or a
// bare marker line). Tolerant of raw issue bodies so the workflow can pipe
// `gh issue list --json body` output straight in.
var owedMarkerRe = regexp.MustCompile(`<!-- authoring-owed: [^>]*? -->`)

// owedMarker renders the hidden per-document marker (keyed on the document's
// repo-relative path). It is the first line of the issue body (idempotency).
func owedMarker(rel string) string {
	return "<!-- authoring-owed: " + rel + " -->"
}

// loadOwedMarkers reads the --owed-markers file and returns the set of markers
// already present in existing issues. A missing/empty path or a non-existent
// file yields an empty set (nothing filed yet). It extracts every
// `<!-- authoring-owed: … -->` occurrence, so it accepts either one marker per
// line OR raw issue bodies. (Mirror of loadDecisionMarkers.)
func loadOwedMarkers(path string) (map[string]bool, error) {
	set := map[string]bool{}
	if path == "" {
		return set, nil
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return set, nil
	}
	if err != nil {
		return nil, err
	}
	for _, m := range owedMarkerRe.FindAllString(string(raw), -1) {
		set[strings.TrimSpace(m)] = true
	}
	return set, nil
}

// owedIssueTitle builds the issue title for an owed document, truncated to
// GitHub's title limit exactly as the sibling emitters do.
func owedIssueTitle(rel string) string {
	t := "authoring-owed: " + rel + " — approved spec has no brief"
	if utf8.RuneCountInString(t) <= maxIssueTitleLen {
		return t
	}
	r := []rune(t)
	return string(r[:maxIssueTitleLen-1]) + "…"
}

// renderOwedBody builds the self-contained markdown issue body for an
// approved-but-uncited document. Fully offline and self-contained (public-repo
// safe): everything is lifted from the parsed header plus the static repo slug.
func renderOwedBody(h *specLifecycleHeader) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", owedMarker(h.Rel))

	fmt.Fprintf(&b, "## Authoring owed\n\n")
	fmt.Fprintf(&b, "**`%s`** is **approved** (the plan of record) but no brief's `sources:` frontmatter dereferences its path, so brief-authoring against it is **owed** (spec/lifecycle-v1.md §8.5).\n\n", h.Rel)

	if h.RoutesTo != "" {
		fmt.Fprintf(&b, "It routes into **`%s`** (`**Routes-to:**`).\n\n", h.RoutesTo)
	}

	fmt.Fprintf(&b, "## What closes this\n\n")
	fmt.Fprintf(&b, "Author at least one brief whose `sources:` frontmatter contains the repo-relative path `%s`. Once a brief cites the path, the document is **routed** (§8.4) and this issue's condition no longer holds — a re-run of the emitter files nothing.\n\n", h.Rel)
	fmt.Fprintf(&b, "This issue does not judge WHICH stream the work belongs in, nor auto-author the brief; it only records that the approved→authored edge is open.\n\n")

	fmt.Fprintf(&b, "## Links\n\n")
	if slug := verifyRepoSlug(); slug != "" {
		fmt.Fprintf(&b, "- **Document:** [`%s`](https://github.com/%s/blob/main/%s)\n", h.Rel, slug, filepath.ToSlash(h.Rel))
	} else {
		fmt.Fprintf(&b, "- **Document:** `%s`\n", h.Rel)
	}
	fmt.Fprintf(&b, "- **Convention:** `spec/lifecycle-v1.md` §8 (the spec/scoping-doc lifecycle)\n")

	return b.String()
}

// owedIssues computes the newly-eligible authoring-owed issues: approved
// documents no brief cites, whose marker is not already in the supplied
// existing-markers set. draft and routed documents are never owed. Output is
// sorted by document path for deterministic emission.
func owedIssues(headers []*specLifecycleHeader, sources []string, existing map[string]bool) []owedIssue {
	out := []owedIssue{}
	for _, h := range headers {
		if h.State != lifecycleApproved {
			continue // only approved documents are owed-candidates (§8.5)
		}
		if documentCited(h.Rel, sources) {
			continue // cited ⇒ routed ⇒ not owed
		}
		marker := owedMarker(h.Rel)
		if existing[marker] {
			continue // already an open issue for this document — emit nothing
		}
		out = append(out, owedIssue{
			Doc:    h.Rel,
			Title:  owedIssueTitle(h.Rel),
			Labels: []string{authoringOwedLabel},
			Marker: marker,
			Body:   renderOwedBody(h),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Doc < out[j].Doc })
	return out
}

// runOwedIssues is the --owed-issues entrypoint: emit the eligible-document JSON
// array to stdout. Returns a process exit code. Self-contained and STATUS.md-free,
// the same discipline as --decision-issues / --verify-issues.
func runOwedIssues(root, markersPath string) int {
	streams, _, err := loadStreams(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	headers, _ := loadLifecycleDocs(root)
	existing, err := loadOwedMarkers(markersPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: reading owed markers:", err)
		return 1
	}
	issues := owedIssues(headers, allBriefSources(streams), existing)
	enc, err := json.MarshalIndent(issues, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	fmt.Println(string(enc))
	return 0
}
