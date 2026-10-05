package main

import (
	"encoding/json"
	"fmt"
)

// usage prints the same help the oracle prints — its own header comment, which the shell reads
// out of its own source with awk. A compiled binary cannot read its source, so the text is
// embedded (usage.go) and usage_test.go proves the embedded copy still matches the oracle's
// header, which is what keeps the two from drifting.
func usage(code int) {
	fmt.Print(usageText)
	fmt.Print(cadenceUsage)
	fmt.Print("\n  scratch <cell> run|ack|sweep|inventory [options]\n    Managed task scratch and evidence handoff; see docs/cellctl-scratch.md.\n")
	fmt.Print("\n  comms <cell> check|run|recover --confirm-stopped\n    Configure with CELL_COMMS_CONFIG; see docs/cellctl-comms.md.\n")
	exitWith(code)
}

// pluginListHasAssay parses `claude plugin list --json` the way the oracle's python3 one-liner
// does: the assay@assay entry must be present AND enabled. A response that is not the expected
// shape answers false — never true-by-default.
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
