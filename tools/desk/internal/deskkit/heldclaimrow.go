package deskkit

import "strings"

// HeldClaimID pulls the claim id out of a `dispatch-claim: HELD <id> — …` list/show line. ok=false
// for any line that is not a HELD row (the tool's `(no dispatch claims…)`, a FREE row, a blank).
//
// The marker must start a word: the claim tool's `show` also prints `ALIAS-HELD <id> — …` for
// another spelling of the same PR, and that row is not a second occupied slot.
func HeldClaimID(line string) (string, bool) {
	i := heldRowMarker(line)
	if i < 0 {
		return "", false
	}
	rest := strings.TrimSpace(line[i+len("HELD "):])
	if rest == "" {
		return "", false
	}
	// The id is the first whitespace-delimited token after HELD.
	if j := strings.IndexAny(rest, " \t"); j >= 0 {
		return rest[:j], true
	}
	return rest, true
}

// heldRowMarker is the index of the first "HELD " in line that is not the tail of a longer
// hyphenated word (ALIAS-HELD), or -1.
func heldRowMarker(line string) int {
	for off := 0; ; {
		j := strings.Index(line[off:], "HELD ")
		if j < 0 {
			return -1
		}
		i := off + j
		if i == 0 || (line[i-1] != '-' && !isWordByte(line[i-1])) {
			return i
		}
		off = i + 1
	}
}

// OwnsPreparedClaim requires the canonical show row for the exact key and owner.
// Capacity readers allow a short HELD row; custody also requires the claim record.
func OwnsPreparedClaim(output, key, session string) bool {
	line := strings.TrimSpace(output)
	id, held := HeldClaimID(line)
	if !held || id != key || strings.ContainsAny(line, "\r\n") {
		return false
	}
	prefix := "dispatch-claim: HELD " + id + " — dispatch-claim " + id + " owner=" + session + " state=claimed branch="
	return strings.HasPrefix(line, prefix)
}
