package main

// summary.go — the terminating summary every mode prints, ported from the oracle's
// finish() (assay-inbox.sh:1286-1305): a POSITIVE statement of "0 items" so it reads
// distinguishably from a run that died before printing anything (finding 1, quoted in the
// oracle's own header comment).
//
// EXIT-CODE CONTRACT DIVERGES FROM THE ORACLE ON PURPOSE. The oracle's own exit codes (0 ok
// / 1 precondition / 2 partial) are bespoke to this one script; every other desk verb in
// this tree — including cmd/issueboard, whose read shape this command otherwise mirrors —
// uses deskkit's shared taxonomy (0 ok, 5 refused, 6 unverifiable). A caller scripting
// against deskinbox already knows that taxonomy from every other verb it runs; keeping the
// oracle's private 0/1/2 would be the one command in the tree that means something
// different by the same numbers. testdata/spec.md records the mapping (oracle 1 → refused
// 5 for bad arguments/preconditions, oracle 2 → unverifiable 6 for partial results).

import "fmt"

func summaryText(itemCount, repoCount int, failures []queryFailure) string {
	s := fmt.Sprintf("deskinbox: %d item(s) across %d repo(s)", itemCount, repoCount)
	if len(failures) > 0 {
		s += fmt.Sprintf("; %d query(s) FAILED — THIS INBOX IS INCOMPLETE (see stderr)", len(failures))
	}
	return s
}
