package main

import "strings"

// The trailer-only writer asks only about rows it can move. Full audits,
// backfill and regen drift never use this scope. Filter BEFORE admission and
// approval folding: leaving an unclassified trailer in either is false evidence.
func trailerWriterKeys(root string) (map[string]bool, error) {
	boardRoot, found := findBoardRoot(root)
	keys := map[string]bool{}
	if !found {
		return keys, nil
	}
	streams, _, err := loadStreams(boardRoot)
	if err != nil {
		return nil, err
	}
	for _, s := range streams {
		for _, b := range s.Briefs {
			switch strings.ToLower(strings.TrimSpace(b.Status)) {
			case "todo", "in-progress":
				keys[canonicalBriefKey(s.Name+"/"+b.Num)] = true
			}
		}
	}
	return keys, nil
}

func trailerWriterPRs(prs []PRRecord, keys map[string]bool) []PRRecord {
	var selected []PRRecord
	for _, p := range prs {
		if keys[canonicalBriefKey(p.BriefRef)] {
			selected = append(selected, p)
		}
	}
	return selected
}

const trailerWriterScopeReason = "outside trailer-writer scope (only todo/in-progress rows requested); use read-only reconcile for a full audit"

// A deliberately unrequested row is not an unread/failed request and cannot
// acquire provenance from a raw trailer or a local witness overlay. Keep it in
// the output as explicitly unknown rather than implying a full-board audit.
func markOutsideWriterScope(cells []BriefCell, keys map[string]bool) {
	for i, c := range cells {
		if !keys[canonicalBriefKey(c.ID)] {
			cells[i] = BriefCell{ID: c.ID, Version: c.Version, Cell: "unknown", Source: "none", Reason: trailerWriterScopeReason}
		}
	}
}
