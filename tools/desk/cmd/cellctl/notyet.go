package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// This file holds the verbs still being widened onto, in the Task's own order
// (desk-containers/10 step 3: "widen verb by verb … each widening is its own commit with the
// harness green at that width"). Each one REFUSES loudly and names itself rather than doing
// something approximate: a verb that half-works is worse than one that says it is not here yet,
// and the parity harness reports the cell as divergent either way.
//
// As each verb lands, its function moves to its own file and disappears from here. When this
// file is empty the port is whole.
func notYetPorted(verb string) {
	die("%s: not yet ported to the Go cellctl (desk-containers/10 is widening verb by verb) — use the shell oracle at tools/cellctl/cellctl for this verb", verb)
}

func cmdCheck(cell, cfg string) { notYetPorted("check") }
func cmdUp(cell string, args []string) {
	notYetPorted("up")
}
func cmdDown(cell string, args []string) { notYetPorted("down") }
func cmdNew(args []string)               { notYetPorted("new") }
func cmdShow(cell string, args []string) { notYetPorted("show") }
func cmdDeskd(cell string)               { notYetPorted("deskd") }

// usage prints the same help the oracle prints — its own header comment. The Go binary cannot
// read its source, so the text is embedded (see usage.go) and a unit test proves the embedded
// copy still matches the oracle's header, which is what keeps the two from drifting.
func usage(code int) {
	fmt.Print(usageText)
	exitWith(code)
}

// pluginListHasAssay parses `claude plugin list --json` the way the oracle's python3 one-liner
// does: the assay@assay entry must be present AND enabled.
func pluginListHasAssay(raw []byte) bool {
	var ps []struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.Unmarshal(raw, &ps); err != nil {
		return false
	}
	for _, p := range ps {
		if p.ID == "assay@assay" && p.Enabled {
			return true
		}
	}
	return false
}

var _ = os.Stderr
