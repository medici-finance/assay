package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// landverb.go — `scanloop land`, the landing verb for JUDGMENT exits.
//
// A judgment item is parked by the drain: no production Feeder exists, so the exit a session
// chooses for it by hand would otherwise be written nowhere. The session runs this verb after it
// has routed the item, and the verb writes the intake-exit-v1 record (decided_by: judgment) and the
// same audit `land` line Land writes for a mechanical exit.

// landNow is the clock seam for the verb.
var landNow = func() time.Time { return time.Now().UTC() }

// landRole resolves the record's triager_role from $DESK_LOOP: the canonical name of the running
// loop, and only if that name is in the closed role set. A login-shaped value, or any name the loop
// registry does not know, is refused, never written.
func landRole() (string, error) {
	raw := strings.TrimSpace(os.Getenv("DESK_LOOP"))
	canonical, known := deskkit.CanonicalLoopName(raw)
	if !known || canonical == "driver" || !memberOf(canonical, intakeExitRoles) {
		return "", deskkit.Refused("scanloop land: $DESK_LOOP is not a canonical desk loop name in the closed role set (" +
			strings.Join(intakeExitRoles[:len(intakeExitRoles)-1], ", ") + "); got " +
			clipValue(raw, len("pr-review-desk")) + ". Run it from a desk window that declares its loop.")
	}
	return canonical, nil
}

func cmdLand(args []string, w io.Writer) error {
	fs := flag.NewFlagSet("land", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	item := fs.String("item", "", "the parked item, owner/repo#N")
	exitName := fs.String("exit", "", "the exit chosen: "+strings.Join(exitNames(), " | "))
	artifact := fs.String("artifact", "", "what the item became, as a typed ref")
	tier := fs.String("tier", "", "the deciding session's tier: any | strong")
	detail := fs.String("detail", "", "rejected | watching (exit rejected-watching only)")
	kind := fs.String("kind", "", "the classifier reason the item was parked with")
	dispatchRef := fs.String("dispatch-ref", "", "desk-supervision/28 dispatch_ref, when known")
	if err := fs.Parse(args); err != nil {
		return deskkit.Refused("scanloop land: bad flags: " + err.Error())
	}
	if fs.NArg() != 0 {
		return deskkit.Refused("scanloop land: unexpected arguments: " + strings.Join(fs.Args(), " "))
	}

	role, err := landRole()
	if err != nil {
		return err
	}
	// The same reconciliation Land uses. The drain parked this item UNROUTED; the verb's exit is
	// the routing result, and "unrouted" itself is refused.
	exit, err := ExitOf(ExitUnrouted, Exit(strings.TrimSpace(*exitName)))
	if err != nil {
		return err
	}

	itemID := strings.TrimSpace(*item)
	rec := IntakeExitRecord{
		Schema:      intakeExitSchema,
		Source:      "issue",
		Item:        itemID,
		Repo:        repoOfItemID(itemID),
		Exit:        exit,
		Detail:      strings.TrimSpace(*detail),
		Artifact:    strings.TrimSpace(*artifact),
		DecidedBy:   decidedJudgment,
		TriagerRole: role,
		TriagerTier: strings.TrimSpace(*tier),
		Triaged:     landNow().UTC().Format(time.RFC3339),
		Kind:        strings.TrimSpace(*kind),
		SessionTag:  recordSessionTag(),
		DispatchRef: strings.TrimSpace(*dispatchRef),
	}
	wrote, err := appendExitRecord(rec, func() error {
		return auditExit("scanloop", ExitRecord{ItemID: rec.Item, Exit: exit, Lane: LaneRouting, Artifact: rec.Artifact})
	})
	if err != nil {
		return err
	}
	if !wrote {
		fmt.Fprintf(w, "scanloop land: %s already left by %s; nothing written\n", rec.Item, exit)
		return nil
	}
	fmt.Fprintf(w, "scanloop land: %s -> %s (%s) recorded as a %s-tier judgment by %s\n",
		rec.Item, exit, firstNonEmpty(rec.Artifact, rec.Detail), rec.TriagerTier, role)
	return nil
}
