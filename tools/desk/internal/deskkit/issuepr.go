package deskkit

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/statusgen/briefdoc"
	"gopkg.in/yaml.v3"
)

// Placeholder stems may include the source repository's short name. It is a display
// label, not a repository identity. Read the actual repository from the placeholder.
var issuePlaceholderStem = regexp.MustCompile(`^(?:[a-zA-Z0-9_.-]+-)?issue-([1-9][0-9]*)$`)

func PlaceholderIssueNumber(stem string) int {
	m := issuePlaceholderStem.FindStringSubmatch(stem)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

type DispatchIssue struct {
	Repo    string
	Number  int
	partial map[int]bool
}

// DispatchPlaceholder resolves the source repo and number declared by the board file.
// Even a legacy bare filename can describe an issue from a different repository.
func DispatchPlaceholder(root, localRepo, id string) (DispatchIssue, error) {
	stream, stem, ok := strings.Cut(id, "/")
	n := PlaceholderIssueNumber(stem)
	if !ok || n == 0 {
		return DispatchIssue{}, nil
	}
	if stream == "" || stream == "." || stream == ".." || strings.ContainsAny(stream, `/\`) {
		return DispatchIssue{}, fmt.Errorf("invalid placeholder stream %q", stream)
	}
	path := filepath.Join("docs", "streams", stream, stem+".md")
	raw, err := readPlaceholder(root, path)
	if err != nil {
		return DispatchIssue{}, fmt.Errorf("cannot read issue placeholder %s: %w", id, err)
	}
	var meta struct {
		Schema, Brief, Repo string
		Issue               int
		Delivery            []interface{}
	}
	if err := yaml.Unmarshal([]byte(briefdoc.Frontmatter(string(raw))), &meta); err != nil {
		return DispatchIssue{}, fmt.Errorf("invalid placeholder %s: %w", id, err)
	}
	if meta.Schema != "placeholder-v1" || meta.Brief != id || meta.Issue != n || !ValidRepoSlug(meta.Repo) {
		return DispatchIssue{}, fmt.Errorf("placeholder %s has no matching repository and issue identity", id)
	}
	if !strings.EqualFold(meta.Repo, localRepo) && !IsAllowedRepo(meta.Repo) {
		return DispatchIssue{}, fmt.Errorf("source repo %s is outside the admitted scope", meta.Repo)
	}
	issue := DispatchIssue{Repo: meta.Repo, Number: n}
	if len(meta.Delivery) > 0 {
		var why string
		issue.partial, why = declaredPartialNumbers(meta.Delivery, meta.Repo, LoadDeliveryAliases(root))
		if why != "" {
			return DispatchIssue{}, fmt.Errorf("cannot resolve %s delivery: %s", id, why)
		}
	}
	return issue, nil
}

func readPlaceholder(root, path string) ([]byte, error) {
	dir, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	return dir.ReadFile(path)
}

type issuePRWindow struct {
	byNumber   map[int][]RepresentedPR
	searched   map[int]bool
	incomplete bool
	err        error
}

// IssuePRIndex is scoped to one sweep. Each source repo's list is read once. An
// incomplete window needs a complete search for each missing source issue.
type IssuePRIndex struct {
	List      func(string) ([]PRRef, bool, error)
	Lookup    func(string, string) ([]PRRef, error)
	windows   map[string]issuePRWindow
	exhausted error
}

func (idx *IssuePRIndex) RecordWindow(repo string, prs []PRRef, incomplete bool, err error) {
	if idx.windows == nil {
		idx.windows = map[string]issuePRWindow{}
	}
	idx.windows[strings.ToLower(repo)] = issuePRWindow{byNumber: issueLinks(repo, prs), searched: map[int]bool{}, incomplete: incomplete, err: err}
}

func issueLinks(repo string, prs []PRRef) map[int][]RepresentedPR {
	byNum := map[int][]RepresentedPR{}
	for _, pr := range prs {
		if !prRepresents(pr.State) {
			continue
		}
		ts, err := ParseTrailers([]byte(pr.Body))
		if err != nil {
			continue
		}
		for _, t := range ts {
			if t.Kind != TrailerIssue || (t.Repo != "" && !strings.EqualFold(t.Repo, repo)) {
				continue
			}
			n, err := strconv.Atoi(t.Value)
			if err == nil && n > 0 {
				byNum[n] = append(byNum[n], RepresentedPR{Number: pr.Number, Merged: strings.EqualFold(pr.State, "MERGED")})
			}
		}
	}
	return byNum
}

func (idx *IssuePRIndex) Match(issue DispatchIssue) (RepresentedPR, error) {
	key := strings.ToLower(issue.Repo)
	w, ok := idx.windows[key]
	if !ok {
		prs, incomplete, err := idx.List(issue.Repo)
		idx.RecordWindow(issue.Repo, prs, incomplete, err)
		w = idx.windows[key]
	}
	if w.err != nil {
		return RepresentedPR{}, w.err
	}
	links := DropDeclaredPartials(w.byNumber[issue.Number], issue.partial)
	if len(links) == 0 && !w.searched[issue.Number] && w.incomplete {
		if idx.exhausted != nil {
			return RepresentedPR{}, idx.exhausted
		}
		if idx.Lookup == nil {
			return RepresentedPR{}, fmt.Errorf("no complete issue link search for %s", issue.Repo)
		}
		prs, err := idx.Lookup(issue.Repo, fmt.Sprintf("issue-%d", issue.Number))
		if err != nil {
			if IsForgeRateLimited(err) {
				idx.exhausted = err
			}
			return RepresentedPR{}, err
		}
		w.searched[issue.Number] = true
		w.byNumber[issue.Number] = issueLinks(issue.Repo, prs)[issue.Number]
		links = DropDeclaredPartials(w.byNumber[issue.Number], issue.partial)
	}
	// Active work is resumable even when an earlier linked change also merged.
	for _, link := range links {
		if !link.Merged {
			return link, nil
		}
	}
	if len(links) > 0 {
		return links[0], nil
	}
	return RepresentedPR{}, nil
}

func (i DispatchIssue) Representation(pr RepresentedPR) string {
	if pr.Merged {
		return fmt.Sprintf("issue %s#%d has merged PR %s#%d; reconcile the open issue before starting fresh work", i.Repo, i.Number, i.Repo, pr.Number)
	}
	return fmt.Sprintf("issue %s#%d has open PR %s#%d; resume that PR", i.Repo, i.Number, i.Repo, pr.Number)
}

// PlaceholderHold is the read used by both queue views. The issue may live in a
// different repo from the board; a bare PR number alone is ambiguous.
func (idx *IssuePRIndex) PlaceholderHold(root, boardRepo, id string) (reason string, unknown bool) {
	issue, err := DispatchPlaceholder(root, boardRepo, id)
	if err != nil {
		return "issue PR could-not-check: " + err.Error(), true
	}
	if issue.Number == 0 {
		return "", false
	}
	pr, err := idx.Match(issue)
	if err != nil {
		return fmt.Sprintf("issue PR could-not-check for %s#%d: %v", issue.Repo, issue.Number, err), true
	}
	if pr.Number == 0 {
		return "", false
	}
	return issue.Representation(pr), false
}
