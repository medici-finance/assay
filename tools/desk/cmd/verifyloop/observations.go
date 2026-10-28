package main

import "github.com/medici-finance/assay/tools/desk/internal/deskkit"

// Only the default offline scan reader is memoized. Its scope-validation and
// action methods stay on the existing reader through embedding; none of these
// observations survives a scan or grants execution/landing authority.
type planningRevisionReader struct {
	*deskkit.RootRevisionReader
	revisions map[string]planningRevision
}
type planningRevision struct {
	value string
	known bool
}

func (r *planningRevisionReader) Revision(input string) (string, bool) {
	if seen, ok := r.revisions[input]; ok {
		return seen.value, seen.known
	}
	value, known := r.RootRevisionReader.Revision(input)
	r.revisions[input] = planningRevision{value, known}
	return value, known
}
