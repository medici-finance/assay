package main

import (
	"flag"
	"fmt"
	"github.com/medici-finance/assay/deskcore/issueflow"
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

// landRole resolves the record's triager_role from the loop the SESSION declared in $DESK_LOOP
// (raw is read before the drain's own default fills the variable in): the canonical name of the
// running loop, and only if that name is in the closed role set. An unset value, a login-shaped
// value, or any name the loop registry does not know, is refused, never written.
func landRole(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	canonical, known := deskkit.CanonicalLoopName(raw)
	if !known || canonical == "driver" || !memberOf(canonical, intakeExitRoles) {
		return "", deskkit.Refused("scanloop land: $DESK_LOOP is not a canonical desk loop name in the closed role set (" +
			strings.Join(intakeExitRoles[:len(intakeExitRoles)-1], ", ") + "); got " +
			clipValue(raw, len("pr-review-desk")) + ". Run it from a desk window that declares its loop.")
	}
	return canonical, nil
}

// landOpened parses --opened, the inbound event's time as the drain's EMITTED line printed it. It is
// half of the per-event key (item + opened): an issue that came in once as a new issue and again as
// an update is two events, and each leaves by its own exit. Empty stays empty (the event's time is
// unknown); anything that does not parse as RFC 3339 is refused.
func landOpened(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return "", deskkit.Refused("scanloop land: --opened does not parse as an RFC 3339 time; got " + clipValue(raw, 40))
	}
	return t.UTC().Format(time.RFC3339), nil
}

func cmdLand(args []string, declaredLoop string, w io.Writer) error {
	fs := flag.NewFlagSet("land", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	item := fs.String("item", "", "the parked item, owner/repo#N")
	exitName := fs.String("exit", "", "the exit chosen: "+strings.Join(exitNames(), " | "))
	artifact := fs.String("artifact", "", "what the item became, as a typed ref")
	tier := fs.String("tier", "", "the deciding session's tier: any | strong")
	detail := fs.String("detail", "", "rejected | watching (exit rejected-watching only)")
	kind := fs.String("kind", "", "the classifier reason the item was parked with")
	dispatchRef := fs.String("dispatch-ref", "", "the dispatch_ref join key, when known")
	handoffPath := fs.String("handoff", "", "optional intake-handoff-v1 JSON companion; preserves existing exit history")
	handoffReader := fs.String("handoff-reader", "", "explicit companion reader version (required with --handoff)")
	openedFlag := fs.String("opened", "", "the inbound event's time, RFC 3339, as the EMITTED line printed it")
	if err := fs.Parse(args); err != nil {
		return deskkit.Refused("scanloop land: bad flags: " + clipValue(err.Error(), 80))
	}
	if fs.NArg() != 0 {
		return deskkit.Refused("scanloop land: unexpected arguments: " + clipValue(strings.Join(fs.Args(), " "), 80))
	}

	role, err := landRole(declaredLoop)
	if err != nil {
		return err
	}
	// The same reconciliation Land uses. The drain parked this item UNROUTED; the verb's exit is
	// the routing result, and "unrouted" itself is refused.
	exit, err := ExitOf(ExitUnrouted, Exit(strings.TrimSpace(*exitName)))
	if err != nil {
		return err
	}

	opened, err := landOpened(*openedFlag)
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
		Opened:      opened,
		Kind:        strings.TrimSpace(*kind),
		SessionTag:  recordSessionTag(),
		DispatchRef: strings.TrimSpace(*dispatchRef),
	}
	if *handoffPath != "" {
		if !issueflow.SupportsIntakeHandoff(*handoffReader) {
			return deskkit.Refused("--handoff requires a supported --handoff-reader version")
		}
		raw, err := os.ReadFile(*handoffPath)
		if err != nil {
			return deskkit.Unverifiable("cannot read intake handoff", err)
		}
		h, err := issueflow.DecodeIntakeHandoff(raw)
		if err != nil {
			return deskkit.Refused("invalid intake handoff: " + err.Error())
		}
		prior, err := landHandoff(rec, h)
		if err != nil {
			return err
		}
		if prior != "" {
			fmt.Fprintf(w, "scanloop land: %s already left by %s; that exit is immutable and --exit %s was NOT recorded\n", rec.Item, prior, rec.Exit)
		}
		fmt.Fprintf(w, "scanloop land: %s disposition revision %d recorded; successor admission remains independent\n", rec.Item, h.Revision)
		return nil
	}
	if *handoffReader != "" {
		return deskkit.Refused("--handoff-reader requires --handoff")
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
	if exit == ExitRejectedWatching && rec.Opened == "" {
		// The replay discharge keys on item + opened (judgedRejection), so a rejection
		// with no event time is recorded but never ends replay. Say so, rather than let
		// the issue re-present with no visible reason.
		fmt.Fprintf(w, "scanloop land: NOTE %s was recorded without --opened, so it does not end replay of the issue; land it again with the EMITTED line's --opened\n", rec.Item)
	}
	return nil
}
