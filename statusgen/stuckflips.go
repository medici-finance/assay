package main

// stuckflips.go — verify-reset/07 Task 3: a gate:model brief left at `verified`
// is a row in STATUS.md's roll-up, never only a line in a green CI log.
//
// The model auto-flip (`statusgen --auto-flip-model`) judges exactly the
// gate:model briefs whose README row is `verified`, and moves each one it can
// prove to `done`. Any such row still on the board was therefore REFUSED or
// COULD-NOT-CHECK by the last flip run (or no run has happened since it
// reached `verified`). The roll-up names each one. It carries no forge
// verdict: STATUS.md is rendered offline and byte-compared by --check, so the
// reason lives in the flip's own output, which `--auto-flip-model --dry-run`
// prints and `--auto-flip-model --check` fails on (exit 2).

import "fmt"

// stuckFlipLines renders the roll-up's stuck-flip table, or nothing when no
// gate:model brief sits at `verified`.
func stuckFlipLines(streams []*Stream, ages map[string]string) []string {
	var rows []string
	for _, s := range streams { // parked streams too: the flip judges them
		for i := range s.Briefs {
			br := &s.Briefs[i]
			if br.Gate != "model" || br.Status != "verified" {
				continue
			}
			age := ages[s.Name+"/"+br.Num]
			if age == "" {
				age = "—"
			}
			rows = append(rows, fmt.Sprintf("| [%s/%s](docs/streams/%s/README.md) | %s | %s | REFUSED or COULD-NOT-CHECK — `statusgen --auto-flip-model --dry-run` names which and why |",
				s.Name, br.Num, s.Name, br.Title, age))
		}
	}
	if len(rows) == 0 {
		return nil
	}
	out := []string{
		"",
		fmt.Sprintf("### Stuck auto-flips (%d)", len(rows)),
		"",
		"_gate:model briefs at `verified` that the model auto-flip has not moved to `done`. See docs/board-stuck-autoflips.md._",
		"",
		"| Brief | Title | Waiting | Flip result |",
		"|---|---|---|---|",
	}
	return append(out, rows...)
}
