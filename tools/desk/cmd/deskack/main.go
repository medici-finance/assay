// Command deskack prints — and RECORDS — a desk's receipt for a human-typed message.
//
// WHY IT EXISTS. A desk that acts silently on a typed message gives the operator no
// signal the message was read, so the message gets re-sent (122 duplicate sends in the
// measured window). The receipt is the fix, and it is CODE rather than a sentence a skill
// remembers to write: `deskack "<restatement>"` prints the one fixed line
//
//	ack <role>@<repo-short>: <restatement>
//
// and appends a `{ts, role, repo, restatement}` record to this session's roster beacon.
// The printed line is the ONE line the desk's silent-output contract permits after a
// human message; the beacon record is what the receipt-pairing metric reads to pair a correction
// with the receipt it corrects, and what opmetrics can pair against duplicate sends (a
// re-send with no receipt between == an unacknowledged message).
//
// THE RESTATEMENT IS THE DESK'S OWN READING, capped at twelve words (a receipt is a
// confirmation, not a transcript — and a re-quote defeats the whole point, which is to
// show the operator what the desk UNDERSTOOD so a misread can be corrected on the next
// turn). Over twelve words is refused (exit 5).
//
// ROLE from $DESK_LOOP (the desk loop this session is running); REPO-SHORT from the
// roster alias (deskkit.RepoShortLabel) when --repo names one. The roster read here is the
// DISPLAY-ONLY alias, never a control surface, so this main declares its tool class (to
// read the config-home roster like its siblings) but does not echo the effective config —
// there is no authority decision on this path to make visible, and the receipt line is
// meant to be the only thing on the desk's turn.
//
// Exit codes (deskkit contract): 0 ok · 3 disabled · 5 refused · 6 unverifiable.
package main

import (
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func main() {
	// Read the config-home roster like the acting tools do (ciEligible=false), so
	// RepoShortLabel resolves the same alias map the boards print through. No P3 echo: the
	// only roster read on this path is the display-only alias, not an authority surface.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
