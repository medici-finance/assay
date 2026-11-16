package main

// receipt.go — the one place a receipt (or a refusal) spells the object it names.
//
// A receipt is the tab-separated line a lane prints to stdout, and a desk transcribes it as
// evidence of WHICH object was acted on. GitHub numbers issues and pull requests in one
// sequence and writes both `#N`; GitLab numbers them apart and writes a merge request `!N`,
// so a GitLab merge request spelled `#N` names the issue sharing its number (#6595). Every
// receipt goes through itemRef; TestNoRawReceiptSigil refuses one that writes its own sigil.

import (
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// itemRef renders repo + number + kind the way the forge serving repo writes that object.
// Only a change can differ by forge, so an issue or an unstated kind never consults the
// resolver. A forge that cannot be named keeps the kind-stating `!N` (deskkit.ItemRef
// ForgeString): an explicit change sigil never names the issue sharing the number.
func itemRef(repo string, n int, kind deskkit.TargetKind) string {
	ref := deskkit.ItemRef{Repo: deskkit.StripControl(repo), Number: n, Kind: kind}
	if kind != deskkit.TargetChange {
		return ref.String()
	}
	owner, name, _ := strings.Cut(repo, "/")
	forge, err := forgeKindFn(deskkit.ForgeRepo{Owner: owner, Name: name})
	if err != nil {
		forge = ""
	}
	return ref.ForgeString(forge)
}

// linkItemRef renders the object a comment link names, in the sigil of the forge the link's
// own shape names (a refusal quotes the link back, so it speaks the link's forge).
func linkItemRef(link deskkit.CommentLink) string {
	ref := link.Item
	ref.Repo = deskkit.StripControl(ref.Repo)
	return ref.ForgeString(link.Forge)
}
