// This source-checkout example renders data only. It never posts or mints a
// credential; pass its output through deskreply's normal checks and dry run.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func main() {
	data, err := io.ReadAll(os.Stdin)
	var w deskkit.Workpad
	if err == nil {
		err = json.Unmarshal(data, &w)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "workpad JSON:", err)
		os.Exit(1)
	}
	fmt.Print(deskkit.Render(w))
}
