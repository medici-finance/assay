package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// ack.go — `deskcomms ack <id>`: acknowledge one notice in this session's own
// mailbox. Acknowledgement MOVES the notice (to an acked partition), it never
// deletes — the durability of that move is the gateway's; the client only names
// the id. As with poll, the mailbox is this session's own (cell, role) from
// context, so a caller cannot ack out of another desk's inbox.
func cmdAck(d *deps, args []string) (*outcome, error) {
	fs := flag.NewFlagSet("ack", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return &outcome{detail: "bad arguments"}, deskkit.Refused("ack: " + err.Error())
	}
	rest := fs.Args()
	if len(rest) != 1 || strings.TrimSpace(rest[0]) == "" {
		return &outcome{detail: "bad arguments"}, deskkit.Refused("ack: exactly one <id> argument is required")
	}
	id := strings.TrimSpace(rest[0])

	oc := &outcome{}
	if d.cell == "" || d.self == "" {
		return oc, deskkit.Refused("ack: session identity is not established (see DESK_CELL / DESK_ROLE)")
	}

	if err := d.gateway.Ack(d.cell, d.self, id); err != nil {
		if errors.Is(err, ErrGatewayUnreachable) {
			oc.detail = "gateway-unreachable"
			return oc, deskkit.Unverifiable("could-not-ack: "+err.Error()+" (no local fallback — fail closed)", err)
		}
		oc.detail = "gateway-refused"
		return oc, deskkit.Refused("refused: " + err.Error())
	}
	oc.detail = fmt.Sprintf("acked %s in %s/%s", id, d.cell, d.self)
	fmt.Fprintf(d.stdout, "%s\n", oc.detail)
	return oc, nil
}
