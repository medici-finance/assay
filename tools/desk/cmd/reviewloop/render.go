package main

import (
	"fmt"
	"io"

	"github.com/medici-finance/assay/tools/desk/internal/reviewflow"
)

// RenderRows writes one line per board row — including every WAIT and NO-OP row. The
// waiting states are printed on purpose: a row that vanishes from the reactor's output
// because there was nothing to do about it is indistinguishable, downstream, from a row
// that was never on the board.
func renderRows(b *Board, w io.Writer) {
	for _, r := range b.Rows {
		_, _ = fmt.Fprintf(w, "%-10s %-24s %-9s head=%-12s score=%-4d %s\n",
			r.ID(), r.Action, r.Rule.Disposition, reviewflow.Short(r.Head), r.Score, r.Rule.Why)
	}
}
