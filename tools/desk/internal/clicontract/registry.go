// Package clicontract holds the CLI migration registry, the discovery that checks it
// against the tracked source tree, and the reusable Windows/POSIX fixtures every
// migration child runs its command through (desktools-v2/15).
//
// The registry is docs/streams/desktools-v2/cli-migration.json. Discovery is
// independent of it: every Go main package and every script launcher in the tree must be
// either a registry row or matched by an explicit exclusion rule with a class and a
// reason, and every row must be owned by a brief that depends on the reference migration
// and is a dependency of the final gate. The contract itself is
// docs/streams/desktools-v2/cli-contract.md.
package clicontract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
)

// RegistryPath is the registry's location relative to the repository root.
const RegistryPath = "docs/streams/desktools-v2/cli-migration.json"

// Row states.
const (
	StatePending  = "pending"  // owned, not yet migrated
	StateMigrated = "migrated" // the owner's black-box suite passes on the migrated command
	StateRetired  = "retired"  // removed, with its consumers, by a merged change of the owner
	StateExcluded = "excluded" // not an operator entrypoint; class and reason required
)

// Registry is the parsed cli-migration.json.
type Registry struct {
	Schema         string  `json:"schema"`
	Contract       string  `json:"contract"`
	Stream         string  `json:"stream"`
	Reference      string  `json:"reference"`
	FinalGate      string  `json:"final_gate"`
	MeasuredAt     string  `json:"measured_at"`
	ExclusionRules []Rule  `json:"exclusion_rules"`
	Entrypoints    []Entry `json:"entrypoints"`
}

// Rule excludes every discovered entrypoint whose path matches Pattern ("**" spans
// directories, "*" does not).
type Rule struct {
	Class   string `json:"class"`
	Pattern string `json:"pattern"`
	Reason  string `json:"reason"`
}

// Entry is one inventoried entrypoint.
type Entry struct {
	Executable string   `json:"executable"`
	Path       string   `json:"path"`
	Kind       string   `json:"kind"`   // go | script
	Module     string   `json:"module"` // owning Go module dir, or the script interpreter
	Release    string   `json:"release"`
	Consumers  []string `json:"consumers"`
	Parser     string   `json:"parser"`
	Complexity string   `json:"complexity"` // simple | complex
	Basis      string   `json:"basis"`
	Owner      string   `json:"owner"`
	State      string   `json:"state"`
	Class      string   `json:"class,omitempty"`
	Reason     string   `json:"reason,omitempty"`
	MigratedTo string   `json:"migrated_to,omitempty"`
}

// LoadRegistry reads and decodes the registry from fsys, refusing unknown fields so a
// misspelled key cannot silently drop a fact.
func LoadRegistry(fsys fs.FS) (*Registry, error) {
	raw, err := fs.ReadFile(fsys, RegistryPath)
	if err != nil {
		return nil, err
	}
	return DecodeRegistry(raw)
}

// DecodeRegistry decodes registry JSON strictly.
func DecodeRegistry(raw []byte) (*Registry, error) {
	var r Registry
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return nil, fmt.Errorf("%s: %w", RegistryPath, err)
	}
	return &r, nil
}
