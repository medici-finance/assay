package workspace

// repository.go — the pure half of cell-owned repository stores: where a cell's own clone of a
// repository lives, what a provisioning request for one names, and the managed-store mapping a
// cell publishes once every store in it has been verified. Like the rest of this package it
// performs no effect; the local adapter (package local) clones, observes and verifies, and a
// command composes the transport and decides when to publish.

import (
	"path/filepath"
	"sort"
	"strconv"
)

// The cell-relative layout of spec/cell-workspaces-v1.md section 5.4. Every managed directory
// sits under the cell root, outside the legacy `worktrees/` directory.
const (
	// ReposDir holds one directory per repository key; its `source/` is the cell's own clone.
	ReposDir = "repos"
	// SourceDir is the clone inside a repository's directory.
	SourceDir = "source"
	// ControlDir holds protected control records: the cell binding and the store mapping.
	ControlDir = "run/workspaces"
	// MappingFile is the managed-store mapping inside ControlDir.
	MappingFile = "stores.json"
)

// StorePath is the clone a cell owns for one repository: <root>/repos/<repo-key>/source. The
// key, not the repository's name, places it, so two repositories with the same base name never
// share a directory.
func StorePath(cell CellRef, repo RepoRef) string {
	return filepath.Join(cell.Root, ReposDir, RepoKey(repo), SourceDir)
}

// MappingPath is where a cell's managed-store mapping is published.
func MappingPath(cell CellRef) string {
	return filepath.Join(cell.Root, filepath.FromSlash(ControlDir), MappingFile)
}

// StoreRequest asks for a cell's own clone of one repository, verified to hold an exact commit.
// The commit is what makes a clone of the wrong repository detectable: a transport that hands
// back some other history cannot produce it.
type StoreRequest struct {
	Repo    RepoRef `json:"repo"`
	Subject string  `json:"subject"`
}

// StoreEntry is one verified store in a published mapping. Path and CommonDir are the resolved
// physical directories the effect boundary observed; CommonDir is always Path's own `.git`.
type StoreEntry struct {
	Repo      RepoRef `json:"repo"`
	RepoKey   string  `json:"repo_key"`
	Path      string  `json:"path"`
	CommonDir string  `json:"common_dir"`
	Subject   string  `json:"subject"`
}

// StoreMapping is the managed-store mapping: the separately named, generation-stamped document
// that tells a managed-aware reader where a cell's own stores are. It is never exported under,
// or folded into, the legacy CELL_REPO / CELL_ROOTS keys.
type StoreMapping struct {
	Schema     string       `json:"schema"`
	Cell       CellRef      `json:"cell"`
	Generation uint64       `json:"generation"`
	Stores     []StoreEntry `json:"stores"`
}

// PlanStores validates a set of store requests for one cell and returns them deduplicated and
// ordered by repository key. Two requests for the same repository are one store; asking for two
// different subjects of it is refused, since a store is verified against one.
func PlanStores(cell CellRef, reqs []StoreRequest) ([]StoreRequest, error) {
	if err := cell.validate("cell"); err != nil {
		return nil, err
	}
	if len(reqs) == 0 {
		return nil, refuse(ReasonInvalid, "stores", "want at least one repository")
	}
	byKey := map[string]StoreRequest{}
	for i, q := range reqs {
		at := "stores[" + strconv.Itoa(i) + "]"
		if err := q.Repo.validate(at + ".repo"); err != nil {
			return nil, err
		}
		if !commitRe.MatchString(q.Subject) {
			return nil, refuse(ReasonInvalid, at+".subject", "want a full lowercase commit id")
		}
		k := RepoKey(q.Repo)
		if prior, ok := byKey[k]; ok {
			if prior.Repo != q.Repo {
				// Unreachable unless sha256 collides on 64 bits of prefix; refuse rather than merge.
				return nil, refuse(ReasonIdentityMismatch, at+".repo", "two repositories derive one key")
			}
			if prior.Subject != q.Subject {
				return nil, refuse(ReasonInvalid, at+".subject", "one repository requested at two subjects")
			}
			continue
		}
		byKey[k] = q
	}
	out := make([]StoreRequest, 0, len(byKey))
	for _, q := range byKey {
		out = append(out, q)
	}
	sort.Slice(out, func(i, j int) bool { return RepoKey(out[i].Repo) < RepoKey(out[j].Repo) })
	return out, nil
}

// Validate checks a mapping on its own: every entry's key derives from its repository, its path
// is the store path the layout gives that repository, its common directory is that path's own
// `.git`, and no repository appears twice.
func (m StoreMapping) Validate() error {
	if err := checkSchema(m.Schema); err != nil {
		return err
	}
	if err := m.Cell.validate("cell"); err != nil {
		return err
	}
	if m.Generation < 1 {
		return refuse(ReasonInvalid, "generation", "generations start at 1")
	}
	if len(m.Stores) == 0 {
		return refuse(ReasonInvalid, "stores", "a mapping names at least one store")
	}
	for i, s := range m.Stores {
		at := "stores[" + strconv.Itoa(i) + "]"
		if err := s.Repo.validate(at + ".repo"); err != nil {
			return err
		}
		if s.RepoKey != RepoKey(s.Repo) {
			return refuse(ReasonIdentityMismatch, at+".repo_key", "the key does not derive from the repository")
		}
		if i > 0 && m.Stores[i-1].RepoKey >= s.RepoKey {
			return refuse(ReasonInvalid, at+".repo_key", "stores are ordered by key, each once")
		}
		if err := checkAbs(at+".path", s.Path); err != nil {
			return err
		}
		if err := checkAbs(at+".common_dir", s.CommonDir); err != nil {
			return err
		}
		if s.CommonDir != filepath.Join(s.Path, ".git") {
			return refuse(ReasonIdentityMismatch, at+".common_dir", "a store's common directory is its own .git")
		}
		if !commitRe.MatchString(s.Subject) {
			return refuse(ReasonInvalid, at+".subject", "want a full lowercase commit id")
		}
	}
	return nil
}

// NextMapping is the compare-and-set step of publication: given the mapping currently published
// (nil when there is none) and the generation the caller read (0 for none), it returns the
// mapping with entries replacing those of the same repositories, at the next generation. A
// different stored generation is a stale-generation conflict; a mapping of another cell is an
// identity mismatch.
func NextMapping(prior *StoreMapping, expected uint64, cell CellRef, entries []StoreEntry) (StoreMapping, error) {
	have := uint64(0)
	merged := map[string]StoreEntry{}
	if prior != nil {
		if prior.Cell != cell {
			return StoreMapping{}, refuse(ReasonIdentityMismatch, "cell", "the published mapping belongs to another cell or root")
		}
		have = prior.Generation
		for _, s := range prior.Stores {
			merged[s.RepoKey] = s
		}
	}
	if have != expected {
		return StoreMapping{}, refuse(ReasonStaleGeneration, "generation", "published generation %d, caller read %d", have, expected)
	}
	for _, s := range entries {
		merged[s.RepoKey] = s
	}
	next := StoreMapping{Schema: Schema, Cell: cell, Generation: have + 1}
	for _, s := range merged {
		next.Stores = append(next.Stores, s)
	}
	sort.Slice(next.Stores, func(i, j int) bool { return next.Stores[i].RepoKey < next.Stores[j].RepoKey })
	return next, next.Validate()
}

// EncodeStoreMapping is the canonical encoding of a valid mapping.
func EncodeStoreMapping(m StoreMapping) ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return canonical(m)
}

// DecodeStoreMapping strictly decodes and validates a mapping document.
func DecodeStoreMapping(data []byte) (StoreMapping, error) {
	var m StoreMapping
	if err := decode(data, &m); err != nil {
		return StoreMapping{}, err
	}
	return m, m.Validate()
}

// ValidateCell checks an admitted cell reference: a lowercase UUID and an absolute root.
func ValidateCell(c CellRef) error { return c.validate("cell") }
