package main

import (
	"fmt"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// plainFileRead is nil when fc, a forge read of file at ref, is a plain file the forge
// shows to be the entry's own bytes, and could-not-check (exit 6) otherwise (S-2266-8).
// A landing that takes the branch's copy of a file as a record — a merge base, the
// content a judge compares against, a hash or a byte comparison — must not take bytes
// the forge did not prove: a symbolic link served as its target's bytes or as nothing, a
// submodule, or a file served with its content left out (the GitHub Contents API serves
// a file over its size limit with an empty body, which reads as an empty file).
//
// untypedOK admits a read the forge did not type (Kind ""). Only the outcome-record
// lane passes it: the GitLab backend types no file read, and on GitLab that lane lands
// today, while the main landing there already stops as could-not-check at the branch
// board. The GitHub backend types every read whose response names a type and a path.
func plainFileRead(fc *deskkit.FileContent, fr deskkit.ForgeRepo, file, ref, use string, untypedOK bool) error {
	if fc == nil {
		return deskkit.Unverifiable(fmt.Sprintf("the forge returned no content for %s at %s on %s, so %s cannot be judged", file, ref, fr.Slug(), use), nil)
	}
	if fc.Kind == deskkit.FileKindRegular || (untypedOK && fc.Kind == "") {
		return nil
	}
	kind := fc.Kind
	switch kind {
	case "":
		kind = "an entry the forge did not type"
	case deskkit.FileKindUnproven:
		kind = "an entry whose served bytes are not shown to be its own blob (a link served as its target, or content the forge left out)"
	case deskkit.FileKindSymlink:
		kind = "a symbolic link"
	case deskkit.FileKindSubmodule:
		kind = "a submodule"
	}
	return deskkit.Unverifiable(fmt.Sprintf("%s at %s on %s is %s, not a plain file, so %s cannot be judged from it; nothing was written",
		file, ref, fr.Slug(), kind, use), nil)
}
