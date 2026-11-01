package deskkit

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"reflect"
	"strings"
)

// Bounded, shared native reads: callers keep their existing observation/cache lifetime.
// Never follow a Link URL (it is only a continuation signal), nor claim completeness at
// GitHub's 3000-file ceiling. Each page still uses the ordinary custody/observer/transport.
//
// GetCommit and CompareRefs are shared with metadata-only callers (SHA, parents, author,
// committed date, compare status). Page 1 carries all of that metadata, so a failure on a
// LATER page never discards it: the result comes back with FilesComplete/CommitsComplete
// false, which every completeness consumer already treats as could-not-check. Only an
// identity or metadata change between pages, where the metadata itself is in doubt, and a
// page-1 failure, where there is no metadata yet, return an error.
const (
	historyPerPage     = 100
	historyMaxPages    = 10 // at most 1000 compare commits
	commitFileMaxPages = 30 // GitHub's hard 3000-file ceiling
)

func immutableCommitRef(ref string) bool {
	if len(ref) != 40 && len(ref) != 64 {
		return false
	}
	_, err := hex.DecodeString(ref)
	return err == nil
}

func (g *GitHubForge) commitWithFiles(repo ForgeRepo, ref string) (*RepoCommit, error) {
	var out *RepoCommit
	var identity ghCommitWire
	seen := map[string]bool{}
	pinned := ref
	for page := 1; page <= commitFileMaxPages; page++ {
		var w ghCommitWire
		path := fmt.Sprintf("/repos/%s/%s/commits/%s?per_page=%d&page=%d", repo.Owner, repo.Name, pinned, historyPerPage, page)
		hdr, err := g.doJSONHeader(http.MethodGet, path, nil, &w)
		if err != nil {
			if page == 1 {
				return nil, err
			}
			return out, nil // later-page failure: keep page-1 metadata, files stay incomplete
		}
		if page == 1 {
			if immutableCommitRef(ref) && !strings.EqualFold(w.SHA, ref) {
				return nil, Unverifiable("commit response does not match pinned reference", nil)
			}
			c := w.toRepoCommit()
			c.Files, c.FilesComplete = nil, false
			out, identity = &c, w
		} else if w.SHA != identity.SHA || !reflect.DeepEqual(w.Parents, identity.Parents) {
			return nil, Unverifiable("commit identity changed during file pagination", nil)
		}
		// Missing differs from an explicitly empty file list, even on a later page.
		if w.Files == nil || len(w.Files) > historyPerPage {
			return out, nil
		}
		for _, f := range w.Files {
			if f.Filename == "" || seen[f.Filename] {
				return out, nil // a repeated or omitted file identity: the file list stays incomplete
			}
			seen[f.Filename] = true
			out.Files = append(out.Files, ChangedFile{Filename: f.Filename, PreviousFilename: f.PreviousFilename, Status: f.Status})
		}
		more := hasNextLink(hdr)
		if len(w.Files) == 0 && more {
			return out, nil // continuation without progress: the file list stays incomplete
		}
		if len(w.Files) < historyPerPage && !more {
			out.FilesComplete = true
			return out, nil
		}
		// GetCommit accepts a branch for existing metadata consumers. Further pages must
		// use the immutable commit the first response resolved, never re-read that branch.
		if !immutableCommitRef(identity.SHA) {
			return out, nil
		}
		pinned = identity.SHA
	}
	return out, nil // budget exhausted; FilesComplete remains false
}

func (g *GitHubForge) compareWithHistory(repo ForgeRepo, base, head string) (*RefComparison, error) {
	var out *RefComparison
	var first ghCompareWire
	seen := map[string]bool{}
	for page := 1; page <= historyMaxPages; page++ {
		var w ghCompareWire
		path := fmt.Sprintf("/repos/%s/%s/compare/%s...%s?per_page=%d&page=%d", repo.Owner, repo.Name, base, head, historyPerPage, page)
		hdr, err := g.doJSONHeader(http.MethodGet, path, nil, &w)
		if err != nil {
			if page == 1 {
				return nil, err
			}
			return out, nil // later-page failure: keep page-1 status, commits stay incomplete
		}
		if page == 1 {
			first = w
			out = &RefComparison{Status: w.Status, AheadBy: w.AheadBy, BehindBy: w.BehindBy}
			// The compare API returns its bounded aggregate file window only on page 1.
			// This field retains its existing meaning; it never proves interval history.
			for _, f := range w.Files {
				out.Files = append(out.Files, ChangedFile{Filename: f.Filename, PreviousFilename: f.PreviousFilename, Status: f.Status})
			}
		} else if w.TotalCommits == nil || first.TotalCommits == nil || *w.TotalCommits != *first.TotalCommits || w.Status != first.Status || w.AheadBy != first.AheadBy || w.BehindBy != first.BehindBy {
			return nil, Unverifiable("compare metadata changed during history pagination", nil)
		}
		if w.TotalCommits == nil || *w.TotalCommits < 0 || w.Commits == nil || len(w.Commits) > historyPerPage {
			return out, nil
		}
		for _, c := range w.Commits {
			if strings.TrimSpace(c.SHA) == "" || seen[c.SHA] {
				return out, nil // a repeated or omitted commit identity: the commit list stays incomplete
			}
			seen[c.SHA] = true
			out.Commits = append(out.Commits, c.toRepoCommit())
		}
		// A declared interval larger than the whole budget cannot become complete.
		// Preserve the cheap incomplete result instead of spending nine futile pages.
		if *w.TotalCommits > historyMaxPages*historyPerPage {
			return out, nil
		}
		if len(out.Commits) > *w.TotalCommits {
			return out, nil // more commits than declared: the commit list stays incomplete
		}
		more := hasNextLink(hdr)
		if !more && len(out.Commits) == *w.TotalCommits {
			out.CommitsComplete = true
			return out, nil
		}
		if len(w.Commits) == 0 || (!more && len(w.Commits) < historyPerPage) {
			return out, nil
		}
		// Shared callers also compare branch names. Do not join pages across moving refs;
		// that path stays conservatively incomplete until a caller supplies pinned endpoints.
		if !immutableCommitRef(base) || !immutableCommitRef(head) {
			return out, nil
		}
	}
	return out, nil // budget exhausted; CommitsComplete remains false
}
