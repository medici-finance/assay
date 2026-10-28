// Command cellsvalidate parses and validates a cells.yaml file against the
// config/schema package. Unknown fields and any semantic-rule violation are
// errors. It is the source validation helper used by the schema control harness.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/medici-finance/assay/deskd/config/schema"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: cellsvalidate <cells.yaml> [more.yaml ...]")
		os.Exit(2)
	}

	ok := true
	for _, path := range os.Args[1:] {
		f, err := schema.Load(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: FAIL: %v\n", path, err)
			ok = false
			continue
		}
		fmt.Printf("%s: OK: %d cell(s) validated%s\n", path, len(f.Cells), perUserIdentitySummary(f))
	}

	if !ok {
		os.Exit(1)
	}
}

// perUserIdentitySummary reports which cells the per-user identity path is
// live on. A validator that only ever printed "OK"
// would leave an operator unable to tell an opted-in file from an OFF one
// without re-reading the YAML — and the state of this gate is exactly what
// the operator needs to inspect. The OFF case is stated
// explicitly rather than left silent: "no cell" is the answer, not the
// absence of one.
func perUserIdentitySummary(f *schema.File) string {
	var on []string
	for i := range f.Cells {
		if f.Cells[i].PerUserIdentityEnabled() {
			on = append(on, f.Cells[i].Name)
		}
	}
	if len(on) == 0 {
		return "; per-user identity path: OFF on every cell (default)"
	}
	return "; per-user identity path: ENABLED on " + strings.Join(on, ", ")
}
