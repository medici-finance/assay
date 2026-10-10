package main

// deliveryfiles.go — verify-reset/07 Task 2: the delivering PR is one whose
// diff touches a path the brief's `files:` line declares.
//
// A `Brief:` trailer says which brief a PR is FOR; it does not say the PR
// delivered it. A follow-up, a doc fix or a board edit can carry the same
// trailer and merge after the delivery, and crediting the newest credited PR
// then stamped the wrong one. When the brief declares `files:`, a credited PR
// that touches none of them is not the delivering PR. It is still held to its
// own App approval (nothing here removes an approval requirement), but it is
// never the PR a stamp cites. When no PR touches the declared files the result
// is COULD-NOT-CHECK, "no PR touches the brief's files", and nothing flips.
//
// A brief with no `files:` declaration keeps the trailer-and-history rule.

import (
	"fmt"
	"strings"
)

// briefFiles is a brief's `files:` declaration as the delivering-PR resolvers
// read it. The zero value (declared false) imposes no overlap rule.
type briefFiles struct {
	entries  []string
	declared bool
	// dropped are declared entries that cannot name a path in the PR's repo:
	// another repo's `../<repo>/…` path, `.`, an absolute path, brace syntax.
	dropped []string
}

// forRepo normalises the declaration against the repo whose PRs are read:
// `./` is dropped, a trailing `/**` reads as its directory, a `../<name>/`
// prefix naming this repo itself is removed, and an entry that still cannot
// name a path here (declaredEntrySupported) is dropped and remembered. A
// verify-written path (STATUS.md, a stream README, a brief file, a
// verify-outcome record) is dropped too: touching it never delivers a brief.
func (b briefFiles) forRepo(repo string) briefFiles {
	if !b.declared {
		return b
	}
	name := repo
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	out := briefFiles{declared: true}
	for _, d := range appendDeclaredEntries(nil, b.entries) {
		e := d
		if name != "" {
			e = strings.TrimPrefix(e, "../"+name+"/")
		}
		if !declaredEntrySupported(e) || isVerifyWrittenPath(strings.TrimSuffix(e, "/")) {
			out.dropped = append(out.dropped, d)
			continue
		}
		out.entries = append(out.entries, e)
	}
	return out
}

// overlaps reports whether shape's diff touches a declared path. A
// verify-written path in the diff never counts.
func (b briefFiles) overlaps(shape prShape) bool {
	if !b.declared {
		return true
	}
	for _, p := range append(append([]string(nil), shape.Files...), shape.RenamedFrom...) {
		if isVerifyWrittenPath(p) {
			continue
		}
		for _, d := range b.entries {
			if declaredEntryMatches(d, p) {
				return true
			}
		}
	}
	return false
}

// describe names the declaration for a reason line.
func (b briefFiles) describe() string {
	if len(b.entries) == 0 {
		return fmt.Sprintf("none of its declared entries names a path in this repo: %s", strings.Join(b.dropped, ", "))
	}
	return strings.Join(b.entries, ", ")
}

// missWhy is the walked-list note for a credited PR that touches none of the
// declared files.
func (b briefFiles) missWhy(briefID string) string {
	return fmt.Sprintf("it names %s but touches none of its files: (%s) — not the delivering PR", canonicalBriefKey(briefID), b.describe())
}
