package main

import (
	"path"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
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

// isHomeRepo reports whether repo is the configured home repo (ASSAY_HOME_REPO), the one repository
// statusgen names with the bare `issue-N.md`. Unset, no repository is: statusgen then gives no repo
// the bare filename either.
func isHomeRepo(repo string) bool {
	home := deskkit.EffectiveConfig().HomeRepo
	return home != "" && strings.EqualFold(repo, home)
}

// placeholderNamesFor is every file name that carries THIS repository's issue N: the repo-stemmed
// form, and — for the home repo only — the bare `issue-N.md` form the scanner uses for it. The
// scanner's own name for the issue comes first.
//
// THE CLASS (#7023, the same class as #6601): crediting a member from evidence that is not scoped to
// the member. A bare `issue-N.md` names a number, not a repository; matched on the number alone it
// credited issue N of every repository in the batch. This is the one place a placeholder file name
// is tied to a member — creditMembers and HasPlaceholder both go through matchesPlaceholder.
func placeholderNamesFor(repo string, number int) []string {
	n := strconv.Itoa(number)
	stemmed := placeholderStemOf(repo) + "-issue-" + n + ".md"
	if isHomeRepo(repo) {
		return []string{"issue-" + n + ".md", stemmed}
	}
	return []string{stemmed}
}

// matchesPlaceholder reports whether the file name (a base name, any case) carries this repository's
// issue N placeholder.
func matchesPlaceholder(name, repo string, number int) bool {
	for _, want := range placeholderNamesFor(repo, number) {
		if strings.EqualFold(name, want) {
			return true
		}
	}
	return false
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
	credited = map[string]string{}
	for _, m := range members {
		repo, number, ok := splitMember(m)
		if ok {
			for _, c := range changed {
				if matchesPlaceholder(path.Base(c.path), repo, number) {
					credited[m] = c.path
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
