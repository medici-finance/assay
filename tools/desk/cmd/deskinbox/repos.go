package main

// repos.go — the repo-resolution order, ported verbatim from assay-inbox.sh's
// resolve_repos(): (1) repo args on the command line, (2) ./.assay/repos.txt, (3) the
// current repo's `origin` remote. The third source is the one place this file differs from
// the oracle in MECHANISM, not in observable behaviour: the oracle shells `git remote
// get-url origin` + `sed`; this reads the same fact through deskkit.RepoSlugForDir, the
// pure-Go (go-git) equivalent every other migrated desk command already uses — this
// package shells out to nothing (Verify row 4).

import (
	"bufio"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// repoSlugRe is the GitHub owner/name alphabet the oracle validates every repo token
// against before it reaches a query — a metacharacter-laden token must never reach a forge
// call, and garbage from a non-GitHub origin remote is clearer rejected here than as a
// downstream API failure.
var repoSlugRe = regexp.MustCompile(`^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$`)

// resolveRepos mirrors resolve_repos(): args win outright when given; else
// ./.assay/repos.txt (blank lines and #-comments ignored); else the cwd's origin remote,
// parsed to owner/name. An empty result (no args, no repos.txt, no resolvable origin) is
// reported by the caller, exactly as the oracle's own "no repos to query" refusal is.
func resolveRepos(args []string, cwd string) ([]string, error) {
	if len(args) > 0 {
		return args, nil
	}

	repoTxt := cwd + "/.assay/repos.txt"
	if f, err := os.Open(repoTxt); err == nil {
		defer f.Close()
		var out []string
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			out = append(out, line)
		}
		if err := sc.Err(); err != nil {
			return nil, deskkit.Unverifiable("cannot read "+repoTxt, err)
		}
		return out, nil
	}

	slug := deskkit.RepoSlugForDir(cwd)
	if slug == "" {
		return nil, nil
	}
	return []string{slug}, nil
}

// validateRepos refuses the first token that is not owner/name in the GitHub slug
// alphabet, naming it — the same shape as the oracle's own per-token refusal.
func validateRepos(repos []string) error {
	for _, r := range repos {
		if !repoSlugRe.MatchString(r) {
			return deskkit.Refused("deskinbox: invalid repo " + strconv.Quote(r) + " (expected owner/name)")
		}
	}
	return nil
}
