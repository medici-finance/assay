package main

import (
	"path"
	"strconv"
	"strings"
)

// written.go — a placeholder exit is recorded only for a member the scan actually wrote (#6601).
//
// THE CLASS THIS CLOSES. The scan-carrier lane runs `statusgen --scan-issues` once for a whole
// batch and used to credit every batch member with the placeholder exit whatever the scan wrote.
// The scanner skips an issue for its own reasons — an excluded label, a placeholder already on the
// default branch, a hold — and a skipped member was still ledgered as `placeholder`, the leak check
// read CLEAN, and a zero-delta carrier could be opened. A later `land` for the real disposition was
// then refused as already exited. Nothing about this is forge-specific: it happened on a GitLab
// project and the same code path serves GitHub.
//
// THE RULE. After the scan, the lane reads what changed under the scan directory and credits a
// member only when the file the scanner names for it is in that set. An uncredited member is
// returned as Unwritten: Land records no exit for it and parks it for a routing decision, so it is
// named under EMITTED instead of being ledgered with an artifact that does not carry it. A scan
// that changed nothing opens no carrier.

// placeholderStemOf is the stem the scanner names a foreign repo's placeholder with: the LAST path
// segment of the slug. It mirrors statusgen's placeholderStem; a GitLab nested group
// (`group/sub/project`) stems as `project`, never as `sub/project`.
func placeholderStemOf(repo string) string {
	if i := strings.LastIndex(repo, "/"); i >= 0 {
		return repo[i+1:]
	}
	return repo
}

// placeholderNamesFor is every file name that carries this issue's placeholder: the repo-stemmed
// form, and the bare `issue-N.md` form the scanner uses for the home repo. The bare form is accepted
// for any repo, as HasPlaceholder accepts it — this package has no reader for the home repo.
func placeholderNamesFor(repo string, number int) []string {
	n := strconv.Itoa(number)
	return []string{placeholderStemOf(repo) + "-issue-" + n + ".md", "issue-" + n + ".md"}
}

// splitMember parses an inbound item id `owner/repo#N`. ok is false for anything else.
func splitMember(id string) (repo string, number int, ok bool) {
	i := strings.LastIndex(id, "#")
	if i <= 0 {
		return "", 0, false
	}
	n, err := strconv.Atoi(id[i+1:])
	if err != nil || n <= 0 {
		return "", 0, false
	}
	return id[:i], n, true
}

// creditMembers maps each batch member to the scan-directory path the scan wrote for it. A member
// with no such path is returned in unwritten, in input order. Only changed paths count: a file the
// scan did not touch is not this pass's write.
func creditMembers(members []string, changed []statusEntry) (credited map[string]string, unwritten []string) {
	byName := map[string]string{}
	for _, c := range changed {
		byName[strings.ToLower(path.Base(c.path))] = c.path
	}
	credited = map[string]string{}
	for _, m := range members {
		repo, number, ok := splitMember(m)
		if ok {
			for _, name := range placeholderNamesFor(repo, number) {
				if p, hit := byName[strings.ToLower(name)]; hit {
					credited[m] = p
					break
				}
			}
		}
		if _, hit := credited[m]; !hit {
			unwritten = append(unwritten, m)
		}
	}
	return credited, unwritten
}
