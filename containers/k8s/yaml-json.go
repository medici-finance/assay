// YAML decoding adapter; reuse the repository's existing pinned codec.
// Run from tools/desk so this needs no separate module or dependency.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

func main() {
	decoder := yaml.NewDecoder(os.Stdin)
	var docs []map[string]any
	for {
		var doc map[string]any
		err := decoder.Decode(&doc)
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "invalid YAML:", err)
			os.Exit(1)
		}
		if len(doc) != 0 {
			docs = append(docs, doc)
		}
	}
	if err := json.NewEncoder(os.Stdout).Encode(docs); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
