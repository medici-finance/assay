package streamview

import (
	"errors"
	"fmt"
	"sort"
)

// ErrUnknownRepo and ErrUnknownRevision are the sentinel errors a Resolver
// returns when the owning repository or the recorded revision is not there.
var (
	ErrUnknownRepo     = errors.New("repository not resolvable")
	ErrUnknownRevision = errors.New("revision not present in repository")
)

// Resolver reads source records from their owning repositories. The package
// ships no implementation: a consumer supplies one over the checkouts it has
// (a git object store, a forge mirror), so this package stays offline and
// dependency-free.
type Resolver interface {
	// Blob returns the object id of path at revision in repo, or "" with a
	// nil error when the revision exists but the path does not.
	Blob(repo, revision, path string) (string, error)
	// Head returns the repository's current revision.
	Head(repo string) (string, error)
}

// BindingReport is the result of dereferencing every binding a view records.
// Problems is empty only when every recorded binding resolved and is current;
// Unchecked lists external references (forge numbers, URLs) this resolver
// cannot see — residue, never a pass.
type BindingReport struct {
	Resolved  []string
	Planned   []string
	Unchecked []string
	Problems  []string
}

// CheckBindings dereferences every SourceRef and evidence reference in v.
//
//   - A SourceRef must name a resolvable repo, a revision present in it, and a
//     path present at that revision; it is STALE when the path's object at the
//     repository head differs from the recorded one.
//   - A path evidence ref resolves at the revision its record pins (a
//     decision's or evidence item's Revision), else at the revision the
//     mission section recorded for that repository. An absent path is a problem; a ref marked
//     planned is reported as planned and is a problem only if it already
//     exists (the planned marker is then stale).
//   - Forge and URL refs are reported as unchecked.
func CheckBindings(v *StreamView, r Resolver) BindingReport {
	var rep BindingReport
	seen := map[string]bool{}
	checkSource := func(where string, s SourceRef) {
		k := where + "|" + s.Repo + "|" + s.Path + "|" + s.Revision
		if seen[k] {
			return
		}
		seen[k] = true
		id, err := r.Blob(s.Repo, s.Revision, s.Path)
		switch {
		case err != nil:
			rep.Problems = append(rep.Problems, fmt.Sprintf("%s: %s:%s@%s: %v", where, s.Repo, s.Path, s.Revision, err))
			return
		case id == "":
			rep.Problems = append(rep.Problems, fmt.Sprintf("%s: %s:%s absent at %s", where, s.Repo, s.Path, s.Revision))
			return
		}
		head, err := r.Head(s.Repo)
		if err != nil {
			rep.Problems = append(rep.Problems, fmt.Sprintf("%s: %s: head: %v", where, s.Repo, err))
			return
		}
		headID, err := r.Blob(s.Repo, head, s.Path)
		if err != nil || headID != id {
			rep.Problems = append(rep.Problems, fmt.Sprintf("%s: %s:%s@%s is stale (changed or removed at head %s)", where, s.Repo, s.Path, s.Revision, head))
			return
		}
		rep.Resolved = append(rep.Resolved, fmt.Sprintf("%s: %s:%s@%s", where, s.Repo, s.Path, s.Revision))
	}

	sections := []struct {
		name string
		p    Provenance
	}{
		{"mission", v.Mission.Provenance},
		{"current_state", v.CurrentState.Provenance},
		{"changes", v.Changes.Provenance},
		{"needs_you", v.NeedsYou.Provenance},
		{"evidence", v.Evidence.Provenance},
		{"outcomes", v.Outcomes.Provenance},
	}
	for _, sec := range sections {
		for _, s := range sec.p.Sources {
			checkSource(sec.name, s)
		}
	}
	for i, e := range v.Changes.Events {
		checkSource(fmt.Sprintf("changes.events[%d]", i), e.Source)
	}

	missionRev := map[string]string{}
	for _, s := range v.Mission.Provenance.Sources {
		if _, ok := missionRev[s.Repo]; !ok {
			missionRev[s.Repo] = s.Revision
		}
	}
	checkRef := func(where string, e EvidenceRef, pinned string) {
		switch e.Kind {
		case EvidenceForge:
			rep.Unchecked = append(rep.Unchecked, fmt.Sprintf("%s: %s#%d", where, e.Repo, e.Number))
			return
		case EvidenceURL:
			rep.Unchecked = append(rep.Unchecked, fmt.Sprintf("%s: %s", where, e.URL))
			return
		case EvidencePath:
		default:
			rep.Problems = append(rep.Problems, fmt.Sprintf("%s: evidence kind %q", where, e.Kind))
			return
		}
		rev, ok := missionRev[e.Repo]
		if pinned != "" {
			rev, ok = pinned, true
		}
		if !ok {
			rep.Problems = append(rep.Problems, fmt.Sprintf("%s: %s:%s has no recorded revision for its repository", where, e.Repo, e.Path))
			return
		}
		id, err := r.Blob(e.Repo, rev, e.Path)
		switch {
		case err != nil:
			rep.Problems = append(rep.Problems, fmt.Sprintf("%s: %s:%s@%s: %v", where, e.Repo, e.Path, rev, err))
		case e.Planned && id != "":
			rep.Problems = append(rep.Problems, fmt.Sprintf("%s: %s:%s is marked planned but exists at %s", where, e.Repo, e.Path, rev))
		case e.Planned:
			rep.Planned = append(rep.Planned, fmt.Sprintf("%s: %s:%s", where, e.Repo, e.Path))
		case id == "":
			rep.Problems = append(rep.Problems, fmt.Sprintf("%s: %s:%s absent at %s", where, e.Repo, e.Path, rev))
		default:
			rep.Resolved = append(rep.Resolved, fmt.Sprintf("%s: %s:%s@%s", where, e.Repo, e.Path, rev))
		}
	}
	for i, c := range v.Mission.Success {
		for j, e := range c.Evidence {
			checkRef(fmt.Sprintf("mission.success[%d].evidence[%d]", i, j), e, "")
		}
	}
	for i, d := range v.NeedsYou.Decisions {
		for j, e := range d.Evidence {
			checkRef(fmt.Sprintf("needs_you.decisions[%d].evidence[%d]", i, j), e, d.Revision)
		}
	}
	for i, it := range v.Evidence.Items {
		checkRef(fmt.Sprintf("evidence.items[%d]", i), it.Ref, it.Revision)
	}
	sort.Strings(rep.Problems)
	return rep
}
