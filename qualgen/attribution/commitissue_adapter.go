package attribution

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
)

// commitissue_adapter.go — the generic commit->issue reference adapter (task item
// 1), the reference ProvenanceLinkage registered as the DEFAULT. It resolves the
// two rungs any bare-git-plus-forge target carries: the inducing PR itself, and the
// issue(s) the inducing commit/PR message references with a `Fixes/Closes/Refs #N`
// keyword. It reaches no further — a house that records brief/stream/spec rungs
// registers a RICHER adapter as configuration; this one never invents a rung it
// cannot read.
//
// It is self-contained (regexp over the message text an Inducing already carries),
// so it needs no forge access at Resolve time: the miner captured the message when
// it captured the trace, and an offline attribution run reads it from there.

// The default adapter's registered name — also the value DefaultAdapter resolves to
// unless a config selects another.
const CommitIssueAdapterName = "commit-issue"

// issueRefRe matches the conventional issue-closing / reference keywords GitHub and
// GitLab honour, capturing the issue number. It is deliberately generic: it names
// no repository, so a cross-repo `owner/repo#N` form is captured by number only and
// its repo qualifier is preserved separately (a bare `#N` is same-repo). Case is
// insensitive; the keyword set is the union GitHub documents plus the bare `refs`.
var issueRefRe = regexp.MustCompile(`(?i)\b(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?|ref(?:s|erences)?)\b[:\s]+((?:[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9._-]+)?#\d+)`)

// CommitIssueAdapter is the reference commit->issue linkage adapter. It holds no
// state; every answer is a pure function of the Inducing it is given.
type CommitIssueAdapter struct{}

// Resolve builds the chain for one inducing change from its message text alone:
//   - the PR rung is LinkResolved when the Inducing carries a PR ref, else
//     LinkAbsent (a bare-git inducing commit with no PR metadata — a measured
//     absence, never could-not-resolve).
//   - the issue rung is LinkResolved when the message references at least one issue
//     with a closing/reference keyword, carrying every referenced number sorted and
//     de-duplicated; LinkAbsent when the message carries no such reference.
//
// It returns no error: a missing rung is a link STATE, not a failure (spec §3.2).
func (CommitIssueAdapter) Resolve(inducing Inducing) (Chain, error) {
	c := Chain{Inducing: inducing}

	// PR rung.
	if inducing.PR != "" {
		c.Links = append(c.Links, ChainLink{
			Kind:  LinkKindPR,
			Ref:   inducing.PR,
			State: LinkResolved,
		})
	} else {
		c.Links = append(c.Links, ChainLink{
			Kind:   LinkKindPR,
			State:  LinkAbsent,
			Detail: "bare-git inducing commit: no PR metadata to resolve",
		})
	}

	// Issue rung, parsed from the message text.
	refs := parseIssueRefs(inducing.Message)
	if len(refs) > 0 {
		c.Links = append(c.Links, ChainLink{
			Kind:   LinkKindIssue,
			Ref:    joinRefs(refs),
			State:  LinkResolved,
			Detail: fmt.Sprintf("%d issue reference(s) parsed from the inducing message", len(refs)),
		})
	} else {
		c.Links = append(c.Links, ChainLink{
			Kind:   LinkKindIssue,
			State:  LinkAbsent,
			Detail: "no Fixes/Closes/Refs issue reference in the inducing message",
		})
	}

	return c, nil
}

// parseIssueRefs extracts the referenced issue tokens from a message, sorted and
// de-duplicated so the same message always yields the same chain (determinism is
// load-bearing for the dossier hash downstream). A same-repo `#N` and a cross-repo
// `owner/repo#N` are both preserved verbatim; sorting is by numeric issue number
// then by the full token so cross-repo qualifiers order stably.
func parseIssueRefs(message string) []string {
	if message == "" {
		return nil
	}
	seen := map[string]struct{}{}
	var refs []string
	for _, m := range issueRefRe.FindAllStringSubmatch(message, -1) {
		tok := m[1]
		if _, dup := seen[tok]; dup {
			continue
		}
		seen[tok] = struct{}{}
		refs = append(refs, tok)
	}
	sort.Slice(refs, func(i, j int) bool {
		ni, nj := issueNum(refs[i]), issueNum(refs[j])
		if ni != nj {
			return ni < nj
		}
		return refs[i] < refs[j]
	})
	return refs
}

// issueNum returns the numeric part of an issue token (`#42` -> 42,
// `owner/repo#42` -> 42) for stable ordering; 0 when unparseable (kept, ordered
// first, never dropped).
func issueNum(tok string) int {
	for i := len(tok) - 1; i >= 0; i-- {
		if tok[i] == '#' {
			n, err := strconv.Atoi(tok[i+1:])
			if err != nil {
				return 0
			}
			return n
		}
	}
	return 0
}

func joinRefs(refs []string) string {
	out := ""
	for i, r := range refs {
		if i > 0 {
			out += ", "
		}
		out += r
	}
	return out
}

// init registers the reference adapter as the default. A house that records richer
// chains calls attribution.Register with its own adapter and selects it via config;
// this default is always present so an out-of-the-box run resolves the two rungs a
// bare forge target carries.
func init() {
	if err := Register(CommitIssueAdapterName, CommitIssueAdapter{}); err != nil {
		panic(err) // a duplicate default registration is a programming error
	}
	if err := setDefault(CommitIssueAdapterName); err != nil {
		panic(err)
	}
}
