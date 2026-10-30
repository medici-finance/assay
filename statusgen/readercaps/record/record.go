// Package record adapts existing fenced YAML to the pure readercaps JSON
// contract. It uses the repository's existing YAML codec, with no effects.
package record

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/medici-finance/assay/statusgen/briefdoc"
	"github.com/medici-finance/assay/statusgen/readercaps"
	"gopkg.in/yaml.v3"
)

// Requirements returns an explicit empty requirement for a legacy document.
// Only the top-level reader-requirements envelope opts a document in. Body prose
// is never interpreted as a declaration.
func Requirements(raw []byte) (readercaps.Requirements, error) {
	empty := readercaps.Requirements{Schema: readercaps.RequirementsSchema, Required: []readercaps.Operation{}}
	first, _, _ := strings.Cut(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	if strings.TrimSpace(first) != "---" {
		return empty, nil
	}
	if !strings.Contains(string(raw), "reader-requirements") {
		return empty, nil
	}
	block, _, err := briefdoc.Split(string(raw))
	if err != nil {
		return empty, fmt.Errorf("unreadable requirements frontmatter: %w", err)
	}
	// ParseFields validates duplicates, multiple documents and invisible keys;
	// this adapter does not invent a second YAML interpretation.
	if _, err := briefdoc.ParseFields(block); err != nil {
		return empty, err
	}
	var fields map[string]any
	if err := yaml.Unmarshal([]byte(block), &fields); err != nil {
		return empty, err
	}
	value, present := fields["reader-requirements"]
	if !present {
		return empty, nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return empty, err
	}
	return readercaps.DecodeRequirements(encoded)
}
