package main

import "encoding/json"

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
