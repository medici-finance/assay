package deskkit

// represented_forge.go — the typed-seam transport behind the already-represented / phantom
// reconciliation.
//
// The reconciliation (prrepr.go) reduces a repo's OPEN and MERGED changes to the brief→PR map,
// keyed on each change body's `Brief:` trailer. It reads them through a transport of shape
// `func(repo string) ([]PRRef, error)` that is NIL in the offline reference build (the phantom
// check and fanoutloop's plan both default it off). This file is the LIVE transport's body: it
// reads the changes through the typed Forge seam (ListChanges), never a forge CLI, so wiring the
// check live keeps the closed forge surface (internal/forgeban; TestNoForgeCLIShellout) closed.

// RepresentedPRRefs reads a repo's OPEN and MERGED changes through the typed Forge seam and
// reduces them to the []PRRef the reconciliation consumes (number, state, body-with-trailers).
//
// It returns the read window AND whether that window was INCOMPLETE (the change population
// exceeded the seam's bounded page ceiling). Incompleteness is surfaced, never swallowed: a
// caller that reads absence from the list as evidence must decide what an incomplete window
// means for its question, rather than treat a truncated read as a confident negative. A
// transport error (could not reach the forge, GraphQL/tier failure) is returned as-is — an
// unreadable list is could-not-check, never "no PR exists".
func RepresentedPRRefs(f Forge, repo ForgeRepo) (refs []PRRef, incomplete bool, err error) {
	cl, err := f.ListChanges(repo, OpenAndMerged())
	if err != nil {
		return nil, false, err
	}
	refs = make([]PRRef, 0, len(cl.Changes))
	for _, c := range cl.Changes {
		refs = append(refs, PRRef{Number: c.Number, State: c.State, Body: c.Body})
	}
	return refs, cl.Incomplete, nil
}
