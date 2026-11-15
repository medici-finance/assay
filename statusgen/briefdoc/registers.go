package briefdoc

import (
	"regexp"

	"gopkg.in/yaml.v3"
)

// ReservedRegisterNames returns the register directory roster fixed by
// registers-v1. Each caller gets its own map; there is one declaration owner.
func ReservedRegisterNames() map[string]bool {
	return map[string]bool{
		"intake":       true,
		"findings":     true,
		"requirements": true,
		"decisions":    true,
		"deploys":      true,
		// verify-outcomes (#882): one file per verify outcome, per-stream subdirectory — a
		// register of per-brief records, not a stream board (no README brief-status table, no
		// waves). Its own README additionally self-declares "register, not a stream" per
		// spec/registers-v1.md §7, so an adopter tree that has not yet re-pinned this literal
		// name still skips it via that declaration.
		"verify-outcomes": true,
		// witnesses (statusgen/36): one v2 witness record per run under
		// <stream>/<NN>/ (evaluation.WitnessRecordDir) — a register of records, not
		// a stream board.
		"witnesses": true,
	}
}

var registerDeclarationRe = regexp.MustCompile(`(?i)register,\s+not\s+a\s+stream`)

// RegisterDeclarationPattern is the existing register marker owner. Callers
// consult it only after stream parsing fails, never to override a stream.
func RegisterDeclarationPattern() *regexp.Regexp { return registerDeclarationRe }

// IsRegisterIndex recognizes a self-declaring register whose frontmatter cannot
// be read as the existing stream shape. A valid stream cannot opt out by prose.
func IsRegisterIndex(raw []byte) bool {
	if !registerDeclarationRe.Match(raw) {
		return false
	}
	block, _, err := Split(string(raw))
	if err != nil {
		return true
	}
	var fields StreamFields
	return yaml.Unmarshal([]byte(block), &fields) != nil
}
