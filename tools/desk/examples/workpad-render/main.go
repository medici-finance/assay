// This source-checkout example renders data only. It never posts or mints a
// credential; pass its output through deskreply's normal checks and dry run.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func main() {
	// Workpad has no json tags, so a misspelt key would silently render "_none yet_".
	dec := json.NewDecoder(os.Stdin)
	dec.DisallowUnknownFields()
	var w deskkit.Workpad
	if err := dec.Decode(&w); err != nil {
		fmt.Fprintln(os.Stderr, "workpad JSON:", err)
		os.Exit(1)
	}
	fmt.Print(deskkit.Render(w))
}
