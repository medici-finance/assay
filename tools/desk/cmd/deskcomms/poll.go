package main

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// poll.go — `deskcomms poll`: read this session's own per-role mailbox at the
// gateway. Read-side plumbing (the-desk reading status reports and held-message
// notices). Like send, the mailbox it reads is its OWN — the (cell, role) come
// from session context, never from an argument, so a caller cannot poll another
// desk's inbox by naming it.
//
// There is no local fallback: an unreachable gateway is could-not-poll (fail
// closed), never an empty "you have no messages" that a caller would misread as a
// clean, checked inbox.
func cmdPoll(d *deps, args []string) (*outcome, error) {
	fs := flag.NewFlagSet("poll", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return &outcome{detail: "bad arguments"}, deskkit.Refused("poll: " + err.Error())
	}
	oc := &outcome{}
	if d.cell == "" || d.self == "" {
		return oc, deskkit.Refused("poll: session identity is not established (see DESK_CELL / DESK_ROLE)")
	}

	notices, err := d.gateway.Poll(d.cell, d.self)
	if err != nil {
		if errors.Is(err, ErrGatewayUnreachable) {
			oc.detail = "gateway-unreachable"
			return oc, deskkit.Unverifiable("could-not-poll: "+err.Error()+" (no local fallback — fail closed)", err)
		}
		oc.detail = "gateway-refused"
		return oc, deskkit.Refused("refused: " + err.Error())
	}

	for _, n := range notices {
		fmt.Fprintf(d.stdout, "%s\t%s/%s\t%s\t%s\n", n.ID, n.From.Cell, n.From.Role, n.Verb, n.Class)
	}
	oc.detail = fmt.Sprintf("polled %s/%s: %d notice(s)", d.cell, d.self, len(notices))
	return oc, nil
}
