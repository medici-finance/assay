// desks.go — the desk registry read (the service contract): the join of a
// cell's configured loops (what exists), each loop's App identity (who it
// files as), and the live attach state this package's [Registry] already
// derives (how it is doing) into one [contract.DeskRow] per desk.
//
// The single-point-of-failure the brief's Context names is "one row per
// configured desk even when unreachable": [Registry.DeskRows] builds the row
// set from cell.Loops FIRST and joins state onto it, so a missing/unreachable
// handle can only change a row's State to could-not-check — it can never
// remove the row (layer a). [TestDeskRowsNeverDropUnreachable] is the negative
// path that proves it with every handle ErrUnreachable.
//
// Peers are listed (group: peer) but never dereferenced here: this brief has
// no cross-deskd read (peer reachability adds one), so a peer row is always
// could-not-check and carries only the identity + url a client needs to go
// there directly (spec §7: deskd never proxies). Custom sessions (the send route,
// sessions.go) are appended after the configured rows as group "custom"; with
// none open the row count is exactly len(loops)+len(peers), as before.
package terminal

import (
	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/contract"
)

// DeskRows builds the desk registry read for one cell: master and desk rows
// for every configured loop (in cells.yaml loop order), then a peer row for
// each entry in the cell's OPTIONAL `peers:` list. r may be nil (no terminal
// registry wired on this build, exactly the [Registry.Resolve] nil-safe
// shape callers already rely on) — every configured-loop row is still
// produced, all could-not-check, never an empty list.
func (r *Registry) DeskRows(cell *schema.Cell) []contract.DeskRow {
	if cell == nil {
		return nil
	}
	masterRole := cell.MasterRoleOrDefault()
	rows := make([]contract.DeskRow, 0, len(cell.Loops)+len(cell.Peers))
	for _, l := range cell.Loops {
		id := DeskID(cell.Name, string(l.Role))
		group := "desk"
		if l.Role == masterRole {
			group = "master"
		}
		row := contract.DeskRow{
			ID:    id,
			Cell:  cell.Name,
			Role:  string(l.Role),
			Group: group,
			App:   l.WriteAppOrDefault(),
			State: string(StateCouldNotCheck),
			// No registry wired: nothing attaches this desk (off / disabled).
			Attach: NotAttachedStatus(ReasonDisabled, l.Target),
		}
		if r != nil {
			if h := r.handle(id); h != nil {
				h.Refresh()
				view := h.view()
				row.TargetKind = view.TargetKind
				row.AgentIdentity = view.AgentIdentity
				row.State = view.State
				row.Cursor = view.Cursor
				row.LastTransition = view.AsOf
				row.Attach = view.Attach
			}
		}
		rows = append(rows, row)
	}
	// Custom sessions (the service contract's session store): one row each, in
	// open order. They add rows; they can never remove a configured one.
	for _, h := range r.customHandles(cell.Name) {
		h.Refresh()
		view := h.view()
		rows = append(rows, contract.DeskRow{
			ID:             view.DeskID,
			Cell:           cell.Name,
			Role:           "custom",
			Group:          "custom",
			TargetKind:     view.TargetKind,
			AgentIdentity:  view.AgentIdentity,
			State:          view.State,
			Cursor:         view.Cursor,
			LastTransition: view.AsOf,
		})
	}
	for _, p := range cell.Peers {
		rows = append(rows, contract.DeskRow{
			ID:    p.Cell + "/master",
			Cell:  p.Cell,
			Role:  "master",
			Group: "peer",
			State: string(StateCouldNotCheck),
			URL:   p.URL,
			// This deskd never attaches a peer's desk.
			Attach: NotAttachedStatus(ReasonNoTarget, nil),
		})
	}
	return rows
}

// NotAttachedStatus is the attach status of a row this deskd does not attach
// at all: status "off" with the given reason (disabled when no registry or
// supervisor is running; no-target for a peer row). Its capability is the
// host's answer for spec's kind.
func NotAttachedStatus(reason string, spec *TargetSpec) contract.AttachStatus {
	return contract.AttachStatus{
		Status:     AttachOff,
		Reason:     reason,
		Capability: CapabilityFor(spec, Host{}),
	}
}
