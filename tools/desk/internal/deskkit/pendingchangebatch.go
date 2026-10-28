package deskkit

import (
	"encoding/json"
	"slices"
	"sort"
	"strings"
)

// PendingChangeObservation is a planning-only projection. Files carry current
// paths and change kinds, not patches or rename ancestry. General Forge readers
// for risk and publication consumers retain their existing complete payloads.
type PendingChangeObservation struct {
	PR    *PullRequest
	Files []ChangedFile
	Err   error
}

// PendingChangeBatchReader grants no new authority. Unknown and read-only
// decorators and other backends retain CompleteChangedFilesOf.
func PendingChangeBatchReader(f Forge) func(ForgeRepo, []int) map[int]PendingChangeObservation {
	g := githubPlanningBatchForge(f)
	if g == nil {
		return nil
	}
	return func(repo ForgeRepo, numbers []int) map[int]PendingChangeObservation {
		return g.pendingChangesBatch(f, repo, numbers)
	}
}

const pendingChangeSelection = `number state merged headRefOid baseRefName changedFiles author{` + verifierActorSelection + `} headRepository{nameWithOwner} baseRepository{nameWithOwner}`
const pendingFilesSelection = `number headRefOid changedFiles files(first:100){totalCount pageInfo{hasNextPage} nodes{path changeType}}`

var pendingChangeBatchQuery = githubNumberedBatchQuery("pullRequest", pendingChangeSelection)
var pendingFilesBatchQuery = githubNumberedBatchQuery("pullRequest", pendingFilesSelection)

func (g *GitHubForge) pendingChangesBatch(f Forge, repo ForgeRepo, numbers []int) map[int]PendingChangeObservation {
	out := map[int]PendingChangeObservation{}
	ns := slices.Clone(numbers)
	sort.Ints(ns)
	ns = slices.Compact(ns)
	var valid []int
	for _, n := range ns {
		if !ValidRepoSlug(repo.Slug()) || g.onlyRepo != "" && !strings.EqualFold(repo.Slug(), g.onlyRepo) || n <= 0 || int64(n) > 1<<31-1 {
			out[n] = PendingChangeObservation{Err: Refused("pending-change batch target is outside credential scope or GraphQL range")}
		} else {
			valid = append(valid, n)
		}
	}
	// One target cannot save a round trip; retain its existing native reader
	// instead of shifting two REST reads into two GraphQL point charges.
	if len(valid) == 1 {
		n := valid[0]
		pr, files, err := CompleteChangedFilesOf(f, repo, n)
		out[n] = PendingChangeObservation{PR: pr, Files: files, Err: err}
		return out
	}
	for len(valid) > 0 {
		count := min(len(valid), verifierAttestationBatchCap)
		chunk := valid[:count]
		valid = valid[count:]
		raw, failures, err := g.githubReadBatch(repo, chunk, pendingChangeBatchQuery)
		if err != nil {
			for _, n := range append(slices.Clone(chunk), valid...) {
				out[n] = PendingChangeObservation{Err: err}
			}
			break
		}
		var ready []int
		for _, n := range chunk {
			observation := PendingChangeObservation{Err: failures[n]}
			if observation.Err == nil {
				observation.PR, observation.Err = githubPendingChange(raw[n], n)
			}
			out[n] = observation
			if observation.Err == nil {
				ready = append(ready, n)
			}
		}
		if len(ready) == 0 {
			continue
		}
		// File observations are genuinely later and must bind to the same head/count.
		raw, failures, err = g.githubReadBatch(repo, ready, pendingFilesBatchQuery)
		if err != nil {
			for _, n := range append(slices.Clone(ready), valid...) {
				out[n] = PendingChangeObservation{Err: err}
			}
			break
		}
		for _, n := range ready {
			observation := out[n]
			observation.Err = failures[n]
			if observation.Err == nil {
				var more bool
				observation.Files, more, observation.Err = githubPendingFiles(raw[n], observation.PR)
				if observation.Err == nil && more {
					// Overflow keeps the complete native walk and its existing count proof.
					pr, files, e := CompleteChangedFilesOf(f, repo, n)
					if e == nil && (pr.HeadSHA != observation.PR.HeadSHA || pr.ChangedFiles != observation.PR.ChangedFiles) {
						e = Unverifiable("pending change moved during complete file fallback", nil)
					}
					observation.PR, observation.Files, observation.Err = pr, files, e
				}
				if observation.Err == nil {
					observation.PR, observation.Files, observation.Err = completeChangedFilesObservation(observation.PR, observation.Files, repo, n)
				}
			}
			out[n] = observation
		}
	}
	return out
}

func githubPendingChange(raw json.RawMessage, number int) (*PullRequest, error) {
	var w struct {
		Number                         *int
		State                          *string
		Merged                         *bool
		HeadRefOID                     *string
		BaseRefName                    *string
		ChangedFiles                   *int
		Author                         *gqlActor
		HeadRepository, BaseRepository *struct{ NameWithOwner string }
	}
	if json.Unmarshal(raw, &w) != nil || w.Number == nil || *w.Number != number || w.State == nil || w.Merged == nil || w.HeadRefOID == nil || !workerHexCommit.MatchString(*w.HeadRefOID) || len(*w.HeadRefOID) != 40 || w.BaseRefName == nil || *w.BaseRefName == "" || w.ChangedFiles == nil || *w.ChangedFiles < 0 {
		return nil, Unverifiable("incomplete pending-change metadata", nil)
	}
	state := strings.ToLower(*w.State)
	if state != "open" && state != "closed" && state != "merged" {
		return nil, Unverifiable("unknown pending-change state", nil)
	}
	if state == "merged" {
		state = "closed"
		if !*w.Merged {
			return nil, Unverifiable("inconsistent pending-change merge state", nil)
		}
	}
	wire := ghPullWire{Number: number, State: state, ChangedFiles: *w.ChangedFiles, Merged: *w.Merged}
	author := gqlActorAccount(w.Author)
	wire.User.Login, wire.User.ID = author.Login, author.ID
	wire.Head.SHA, wire.Base.Ref = *w.HeadRefOID, *w.BaseRefName
	if w.HeadRepository != nil {
		wire.Head.Repo = &ghPullRepoWire{FullName: w.HeadRepository.NameWithOwner}
	}
	if w.BaseRepository != nil {
		wire.Base.Repo = &ghPullRepoWire{FullName: w.BaseRepository.NameWithOwner}
	}
	return ghPullFromWire(wire), nil
}
func githubPendingFiles(raw json.RawMessage, pr *PullRequest) ([]ChangedFile, bool, error) {
	bad := func() ([]ChangedFile, bool, error) {
		return nil, false, Unverifiable("incomplete or moved pending-change files", nil)
	}
	var w struct {
		Number       *int
		HeadRefOID   *string
		ChangedFiles *int
		Files        *struct {
			TotalCount *int
			PageInfo   *struct{ HasNextPage *bool }
			Nodes      *[]*struct {
				Path       *string
				ChangeType *string
			}
		}
	}
	if json.Unmarshal(raw, &w) != nil || pr == nil || w.Number == nil || *w.Number != pr.Number || w.HeadRefOID == nil || *w.HeadRefOID != pr.HeadSHA || w.ChangedFiles == nil || *w.ChangedFiles != pr.ChangedFiles || w.Files == nil || w.Files.TotalCount == nil || *w.Files.TotalCount != pr.ChangedFiles || w.Files.PageInfo == nil || w.Files.PageInfo.HasNextPage == nil || w.Files.Nodes == nil {
		return bad()
	}
	var files []ChangedFile
	seen := map[string]bool{}
	for _, n := range *w.Files.Nodes {
		if n == nil || n.Path == nil || *n.Path == "" || n.ChangeType == nil || seen[*n.Path] {
			return bad()
		}
		status := ""
		switch *n.ChangeType {
		case "ADDED":
			status = "added"
		case "MODIFIED", "CHANGED":
			status = "modified"
		case "DELETED":
			status = "removed"
		case "RENAMED":
			status = "renamed"
		case "COPIED":
			status = "copied"
		default:
			return bad()
		}
		seen[*n.Path] = true
		files = append(files, ChangedFile{Filename: *n.Path, Status: status, PatchAbsent: true})
	}
	more := *w.Files.PageInfo.HasNextPage
	if len(files) > 100 || len(files) > pr.ChangedFiles || more && len(files) == 0 || !more && len(files) != pr.ChangedFiles {
		return bad()
	}
	return files, more, nil
}
