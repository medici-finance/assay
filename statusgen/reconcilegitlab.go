package main

// reconcile on a GitLab-hosted repository (#7252).
//
// `statusgen reconcile --repo <group>/<project>` reads its merge requests,
// their file lists and the open issues' labels through deskread — the seam the
// GitLab auto-flip reads through (autoflip_gitlab.go, #7246) — never the
// GitHub REST API, which answers 404 for a GitLab project. The forge is the one
// the roster binds the review identity to (configuredForgeFn). Every way a list
// went unread is a could-not-check, never a short list.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// noGitHubDoer refuses every GitHub REST call a GitLab-backed client is asked
// to make: on GitLab there is no GitHub read to fall back to.
type noGitHubDoer struct{}

func (noGitHubDoer) Do(*http.Request) (*http.Response, error) {
	return nil, errors.New("this repository is GitLab-hosted (roster forge gitlab): no GitHub API read is made")
}

// newGitLabReconcileClient is reconcile's client for a GitLab repository: its
// pull, file and label reads go through deskread, and no token is sent anywhere.
func newGitLabReconcileClient() *ghClient {
	r := deskreadReaderFn()
	return &ghClient{doer: noGitHubDoer{}, base: githubAPIBase, gl: &r}
}

// gitlabAllPulls is fetchAllPulls on GitLab: `deskread changes --state all`,
// mapped onto the REST pull shape the derivation reads. A merged or open
// change carrying exactly one Brief: trailer is read once more for its merge
// commit or draft flag, which the cell's witness names.
func gitlabAllPulls(r deskreadReader, repo string) ([]ghPull, bool, string) {
	what := fmt.Sprintf("%s changes --repo %s --state all", r.bin, repo)
	out, err := r.readVerb([]string{"changes", "--repo", repo, "--state", "all"})
	var env deskreadChangesEnvelope
	jerr := json.Unmarshal(out, &env)
	switch {
	case jerr == nil && env.Schema != deskreadSchema:
		return nil, false, fmt.Sprintf("%s answered envelope schema %d, this build understands %d", r.bin, env.Schema, deskreadSchema)
	case jerr == nil && env.Kind != "changes":
		return nil, false, fmt.Sprintf("%s: could-not-check: the envelope is kind %q, not \"changes\"", what, env.Kind)
	}
	if jerr == nil {
		for _, p := range env.Partial {
			if p.Repo == repo {
				return nil, false, fmt.Sprintf("%s: could-not-check: %s", what, p.Reason)
			}
		}
		for _, e := range env.Repos {
			if e.Repo != repo {
				continue
			}
			switch {
			case e.Changes == nil:
				return nil, false, what + ": could-not-check: the entry carries no change list"
			case e.Incomplete == nil:
				return nil, false, what + ": could-not-check: the entry carries no completeness field"
			case *e.Incomplete:
				return nil, false, fmt.Sprintf("%s: could-not-check: the list stopped at its %d-page bound, so it is not provably complete", what, e.PageCap)
			}
			src := deskreadModelFlipSource{r: r}
			pulls := make([]ghPull, 0, len(*e.Changes))
			for _, c := range *e.Changes {
				p := ghPull{Number: c.Number, Body: c.Body}
				p.Head.SHA, p.Head.Ref = c.HeadSHA, c.HeadRef
				_, trailer := singleBriefTrailer(c.Body)
				switch c.State {
				case "MERGED":
					if c.MergedAt == "" {
						return nil, false, fmt.Sprintf("%s: could-not-check: MR !%d is listed merged with no merge time", what, c.Number)
					}
					p.State, p.MergedAt = "closed", c.MergedAt
				case "OPEN":
					p.State = "open"
				case "CLOSED":
					p.State = "closed"
				default:
					return nil, false, fmt.Sprintf("%s: could-not-check: MR !%d has state %q", what, c.Number, c.State)
				}
				if trailer && p.State != "closed" || trailer && p.MergedAt != "" {
					d, derr := src.change(repo, c.Number)
					if derr != nil {
						return nil, false, derr.Error()
					}
					p.Draft, p.MergeCommitSHA = d.Draft, d.MergeCommitSHA
				}
				pulls = append(pulls, p)
			}
			return pulls, true, ""
		}
	}
	if err != nil {
		return nil, false, fmt.Sprintf("%s: could-not-check: %v", what, err)
	}
	if jerr != nil {
		return nil, false, fmt.Sprintf("parsing the %s envelope: %v", what, jerr)
	}
	return nil, false, what + ": could-not-check: the envelope returned no result and no reason for this repo"
}

// gitlabPRFiles is ListPRFiles on GitLab: `deskread change-files`. A list the
// verb does not mark complete is an error, never a partial answer.
func gitlabPRFiles(r deskreadReader, repo string, n int) ([]prFile, error) {
	env, i, err := r.readItemAt("--change", "change-files", repo, n)
	if err != nil {
		return nil, err
	}
	cf := env.Items[i].ChangeFiles
	if cf == nil || cf.Complete == nil || !*cf.Complete {
		return nil, fmt.Errorf("%s change-files --change %s#%d: could-not-check: the file list is absent or not complete", r.bin, repo, n)
	}
	files := make([]prFile, 0, len(cf.Files))
	for _, f := range cf.Files {
		files = append(files, prFile{Filename: f.Filename, Status: f.Status, PreviousFilename: f.PreviousFilename})
	}
	return files, nil
}

// gitlabOpenIssueLabels is listOpenIssueLabels on GitLab: `deskread
// issue-list --state open`, which already refuses an incomplete list.
func gitlabOpenIssueLabels(r deskreadReader, repo string) (map[int][]string, bool, string) {
	issues, err := r.IssueList(repo, "open", "")
	if err != nil {
		return nil, false, err.Error()
	}
	byNumber := map[int][]string{}
	for _, is := range issues {
		var names []string
		for _, l := range is.Labels {
			if n := strings.TrimSpace(l); n != "" {
				names = append(names, n)
			}
		}
		if len(names) > 0 {
			byNumber[is.Number] = names
		}
	}
	return byNumber, true, ""
}
