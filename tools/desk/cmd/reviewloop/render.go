package main

import (
	"fmt"
	"io"
)

// RenderRows writes one line per board row — including every WAIT and NO-OP row. The
// waiting states are printed on purpose: a row that vanishes from the reactor's output
// because there was nothing to do about it is indistinguishable, downstream, from a row
// that was never on the board.
func renderRows(b *Board, w io.Writer) {
	for _, r := range b.Rows {
		fmt.Fprintf(w, "%-10s %-24s %-9s head=%-12s score=%-4d %s\n",
			r.ID(), r.Action, r.Rule.Disposition, short(r.Head), r.Score, r.Rule.Why)
	}
}

func short(sha string) string {
	if sha == HeadUnresolved {
		return sha
	}
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
