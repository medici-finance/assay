package main

import (
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Ambiguous-base guard — a three-state instrument fix (docs/three-state-instrument-rule.md,
// sub-rule 1 "absence of evidence is not evidence of absence").
//
// THE DEFECT. `deskwt add` resolves --base with `git rev-parse --verify --quiet
// <base>^{commit}`. When the short name matches MORE THAN ONE ref, git does not fail:
// it silently picks the first candidate in gitrevisions disambiguation order and writes
// `warning: refname '<base>' is ambiguous.` to STDERR at EXIT 0. runGit discards stderr
// on success, so the caller sees a clean resolve and the worktree is created at whichever
// ref git happened to prefer. The observed instance: a clone carrying a stray LOCAL branch
// literally named `origin/main` alongside the real `refs/remotes/origin/main`. The local
// branch wins (refs/heads/ precedes refs/remotes/), so `deskwt add --base origin/main`
// — the DEFAULT — produced a worktree 61 commits behind the true remote tip and reported
// success. That is checked-clean printed where the true state is could-not-check: the tool
// cannot know which ref the caller meant, and MUST NOT guess.
//
// THE FIX. Enumerate every ref the short name could resolve to before using it. Two or
// more candidates is could-not-check (deskkit.Unverifiable, exit 6), naming the candidates
// so the caller can re-run with a fully-qualified ref. Exactly one is checked-clean. Zero
// is left to the existing rev-parse gate, which already refuses an unresolvable base.
//
// A fully-qualified name (`refs/remotes/origin/main`) is unambiguous by construction and
// is checked as itself — this is the escape hatch the error message points at.

// disambiguationOrder mirrors the gitrevisions(7) resolution rules for a short refname,
// in the order git tries them. It deliberately omits the `$GIT_DIR/<name>` pseudo-ref rule
// (HEAD, FETCH_HEAD, MERGE_HEAD): refRe already rejects those spellings for --base.
var disambiguationOrder = []string{
	"refs/%s",
	"refs/tags/%s",
	"refs/heads/%s",
	"refs/remotes/%s",
	"refs/remotes/%s/HEAD",
}

// refCandidates returns every full ref name that the (possibly short) name `base` could
// resolve to in the repository at dir, in git's own disambiguation order. A result of
// length >= 2 means git will pick candidates[0] and warn on stderr at exit 0.
//
// The lookup is a single `git for-each-ref` over the candidate patterns: it lists only
// refs that actually exist, needs no network, and cannot itself be fooled by ambiguity
// because every pattern it is given is already fully qualified.
func refCandidates(dir, base string) ([]string, error) {
	patterns := []string{base}
	if !strings.HasPrefix(base, "refs/") {
		patterns = patterns[:0]
		for _, f := range disambiguationOrder {
			patterns = append(patterns, strings.Replace(f, "%s", base, 1))
		}
	}
	out, err := runGit(dir, append([]string{"for-each-ref", "--format=%(refname)"}, patterns...)...)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	found := map[string]bool{}
	for _, ln := range strings.Split(out, "\n") {
		if ln = strings.TrimSpace(ln); ln != "" {
			found[ln] = true
		}
	}
	// Return in git's preference order, not for-each-ref's alphabetical order, so
	// candidates[0] is the ref git would actually have chosen.
	var ordered []string
	for _, f := range patterns {
		if found[f] {
			ordered = append(ordered, f)
			delete(found, f)
		}
	}
	return ordered, nil
}

// checkBaseUnambiguous refuses an ambiguous --base as could-not-check (exit 6) instead of
// letting git guess. A failure to enumerate the refs at all is likewise could-not-check —
// this guard never falls back to "probably fine".
func checkBaseUnambiguous(dir, base string) error {
	cands, err := refCandidates(dir, base)
	if err != nil {
		return deskkit.Unverifiable("cannot enumerate refs matching --base "+base, err)
	}
	if len(cands) < 2 {
		return nil
	}
	return deskkit.Unverifiable("refused: --base "+base+" is AMBIGUOUS — it matches "+
		strings.Join(cands, " and ")+"; git would silently pick "+cands[0]+
		" and warn only on stderr at exit 0. Re-run with the fully-qualified ref you mean.", nil)
}
