// Package record adapts existing fenced YAML to the pure readercaps JSON
// contract. It uses the repository's existing YAML codec, with no effects.
package record

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/medici-finance/assay/statusgen/briefdoc"
	"github.com/medici-finance/assay/statusgen/readercaps"
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
	block, _, err := briefdoc.Split(string(raw))
	if err != nil {
		return empty, fmt.Errorf("unreadable requirements frontmatter: %w", err)
	}
	fields, err := briefdoc.ParseMapping(block)
	if err != nil {
		return empty, err
	}
	value, present := fields["reader-requirements"]
	if !present {
		return empty, nil
	}
	// A capability check does not reclassify unrelated legacy risk metadata.
	// Once a decoded envelope is present, retain the existing brief owner’s
	// duplicate/document/key and policy validation before assessing it.
	if _, err := briefdoc.ParseFields(block); err != nil {
		return empty, err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return empty, err
	}
	return readercaps.DecodeRequirements(encoded)
}
