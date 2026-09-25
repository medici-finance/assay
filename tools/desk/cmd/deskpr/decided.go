package main

// decided.go — `deskpr create|edit --decided <file>`: the desk-decided declaration
// (attention-budget/19). The driver holds merge on every PR, so a desk taking a reversible
// default needs no ruling first — it needs the PR to SAY, at merge time, that this is a
// choice the desk made. --decided is how a worker says that: a file of decision/
// alternative/cost triples, written into the PR body under the fixed `## Desk-decided`
// heading and mirrored as the `desk-decided` label.
//
// A PR that only transcribes recorded rulings passes no --decided and carries neither the
// label nor the block — the tool cannot know "this PR declares nothing because it needed to
// declare nothing" by itself; that is the author's call, and the reviewer is the check (the
// `Undeclared-desk-decision:` finding line deskflip reads).

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// maxDecidedBytes bounds the --decided input file, mirroring maxBodyBytes's role for
// --body-file: a cap exists so an oversized file fails fast and locally, before any network
// call, rather than surfacing as an opaque forge-side rejection later.
const maxDecidedBytes = 16 * 1024

// readDecidedItems reads and parses a --decided file. Every failure — unreadable file, an
// oversized one, an empty list, or an item missing a field — maps to a REFUSAL (exit 5):
// the brief's own contract for "an empty file and an item with no cost: both exit 5 and no
// PR call is made".
func readDecidedItems(path string) ([]deskkit.DecidedItem, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, deskkit.Refused("refused: cannot read --decided file: " + err.Error())
	}
	if len(b) > maxDecidedBytes {
		return nil, deskkit.Refused(fmt.Sprintf("refused: --decided file exceeds %d bytes (%d)", maxDecidedBytes, len(b)))
	}
	items, perr := deskkit.ParseDecidedItems(b)
	if perr != nil {
		return nil, deskkit.Refused("refused: " + perr.Error())
	}
	return items, nil
}

// injectDecidedBlock folds a --decided file into body and returns the body to publish. It is
// a no-op (body, nil) when decidedFile is empty — every existing create/edit call site that
// passes no --decided is therefore byte-for-byte unchanged.
//
// create REFUSES when the CALLER-SUPPLIED body already carries a hand-written
// `## Desk-decided` heading: a brand-new PR has no prior block of its own, so a heading
// already present means a human typed one by hand (or pasted the retired interim comment
// form into the body) instead of using --decided, and the tool declines to silently
// duplicate or shadow it — the fix is to remove it and pass --decided.
//
// edit does NOT refuse on the same shape: the replacement body it is handed is routinely a
// copy of the PR's own current body (the ordinary "take the body, correct one thing,
// resubmit" edit shape), which — if a prior --decided already wrote one — already carries
// the heading. --decided's block REPLACES that section in place, exactly where it was,
// rather than being refused or duplicated beside it.
func injectDecidedBlock(body []byte, decidedFile, verb string) ([]byte, error) {
	if decidedFile == "" {
		return body, nil
	}
	if verb == "create" && deskkit.HasDeskDecidedHeading(string(body)) {
		return nil, deskkit.Refused(fmt.Sprintf(
			"refused: the PR body already carries a hand-written `%s` heading — remove it and use "+
				"--decided instead (create refuses a hand-written block and --decided together; a fresh "+
				"PR has no prior block of its own to replace)", deskkit.DeskDecidedHeading))
	}
	items, err := readDecidedItems(decidedFile)
	if err != nil {
		return nil, err
	}
	block := deskkit.RenderDecidedBlock(items)
	return []byte(deskkit.ReplaceOrAppendDeskDecidedBlock(string(body), block)), nil
}

// applyDeskDecidedLabel ensures the desk-decided label is present on the change, through
// deskkit's LabelChange — the same create-if-missing reconciliation deskflip's queue-label
// swap and deskpost's verdict labels already use, never a second label-provisioning path.
var applyDeskDecidedLabel = func(fg deskkit.Forge, fr deskkit.ForgeRepo, number int) error {
	_, err := fg.ApplyLabels(fr, number, deskkit.LabelChange{
		Target: deskkit.TargetChange,
		Add: []deskkit.LabelSpec{{
			Name:        deskkit.DeskDecidedLabel,
			Color:       "5319e7",
			Description: "A desk took a reversible default here — see the PR's Desk-decided block",
		}},
	})
	return err
}

// deskDecidedLabelFailure wraps a label-write failure that follows an already-LANDED
// create/edit, in the same shape edit.go's re-review-notice failure uses: the write that
// mattered (the body carrying the block) already happened, so this reports the label
// failure AS ITSELF rather than rolling it into, or masking it behind, the create/edit's own
// result. The block is the record; the label is only the at-a-glance view.
func deskDecidedLabelFailure(url string, cause error) error {
	return deskkit.Unverifiable(fmt.Sprintf(
		"%s was written with its Desk-decided block, but the %s label could NOT be applied: %v — the "+
			"block is the record; add the label by hand (or re-run this step) once the forge is reachable.",
		url, deskkit.DeskDecidedLabel, cause), cause)
}
