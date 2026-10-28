package contexthandoff

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// Snapshot is the resolver's whole input: the target, the authorized source
// universe with the files of every available source, the declared semantic
// edges, the observed claims and the rival proposals. The resolver reads
// nothing else, so two runs over equal snapshots give equal receipts.
type Snapshot struct {
	DeclaredBy   string // four-segment identity of the declaring brief
	Target       Target
	Terms        []string // discovery terms; the target's contract names are added
	DecidingRole string
	Sources      []SnapshotSource
	Edges        []DeclaredEdge
	Claims       []Claim
	Proposals    []Proposal
	Limits       Limits
	// Product is the optional product-target input (statusgen/23). Nil means
	// the snapshot supplies none, and every receipt over it stays
	// architecture-only.
	Product *SnapshotProduct
}

// SnapshotSource is one member of the source universe. Files is nil when the
// source is unavailable: an unavailable source is never read, and nothing
// about its content (names or counts) reaches a receipt.
type SnapshotSource struct {
	ID          string
	Cell        string
	Repo        string
	Role        string // RolePlan or RoleGuideline
	Required    bool
	Unavailable bool
	Files       map[string][]byte
}

// Source roles.
const (
	RolePlan      = "plan"      // briefs, stream READMEs and plan prose
	RoleGuideline = "guideline" // project engineering guidelines
)

// DeclaredEdge is a semantic edge the snapshot declares: the semantic owner of
// a term, a producer at a version, a planned replacement.
type DeclaredEdge struct {
	Subject string // four-segment brief identity
	Edge    string // one of the context-handoff-v1 relationship edges
	Term    string
	Locator string
	Version string // "" when the edge carries no version
}

// Claim is an observed dispatch claim, read from the snapshot, never a forge.
type Claim struct {
	Subject       string
	Claim         string
	BriefRevision string
	ObservedAt    string
	Owner         string
	Checkpoint    string
}

// Proposal is a rival handoff in the snapshot. Only an approved one can
// invalidate or compete with approved work.
type Proposal struct {
	Locator string
	Handoff *Handoff
}

// Limits bound what a run reads from each source: MaxFiles and MaxTotalBytes
// are per source, MaxFileBytes per file, and MaxResults per query. A source
// over a read limit is unavailable (scoped could-not-check); a query over
// MaxResults is truncated (partial coverage). Neither is ever read as empty.
type Limits struct {
	MaxFiles      int
	MaxFileBytes  int64
	MaxTotalBytes int64
	MaxResults    int
}

// DefaultLimits are the limits LoadSnapshot applies when given a zero Limits.
var DefaultLimits = Limits{MaxFiles: 20000, MaxFileBytes: 1 << 20, MaxTotalBytes: 256 << 20, MaxResults: 2000}

func (l Limits) orDefault() Limits {
	if l == (Limits{}) {
		return DefaultLimits
	}
	return l
}

// snapshotFile is the on-disk shape of <snapshot>/snapshot.json.
type snapshotFile struct {
	DeclaredBy   string          `json:"declared_by"`
	Target       json.RawMessage `json:"target"`
	Terms        []string        `json:"terms"`
	DecidingRole string          `json:"deciding_role"`
	Sources      []struct {
		ID       string `json:"id"`
		Cell     string `json:"cell"`
		Repo     string `json:"repo"`
		Role     string `json:"role"`
		Required bool   `json:"required"`
	} `json:"sources"`
	Edges []struct {
		Subject string `json:"subject"`
		Edge    string `json:"edge"`
		Term    string `json:"term"`
		Locator string `json:"locator"`
		Version string `json:"version"`
	} `json:"edges"`
	Claims []struct {
		Subject       string `json:"subject"`
		Claim         string `json:"claim"`
		BriefRevision string `json:"brief_revision"`
		ObservedAt    string `json:"observed_at"`
		Owner         string `json:"owner"`
		Checkpoint    string `json:"checkpoint"`
	} `json:"claims"`
	Proposals []string `json:"proposals"`
	Product   *struct {
		Targets []struct {
			Outcome   string `json:"outcome"`
			Revision  string `json:"revision"`
			Approval  string `json:"approval"`
			Reference string `json:"reference"`
		} `json:"targets"`
		Relationships []struct {
			Subject string `json:"subject"`
			Outcome string `json:"outcome"`
		} `json:"relationships"`
	} `json:"product"`
}

// snapshotErr is a refusal of the snapshot itself (not of a record).
func snapshotErr(format string, a ...any) error {
	return fmt.Errorf("snapshot: "+format, a...)
}

// LoadSnapshot reads a snapshot from fsys: snapshot.json at its root and each
// source's files under sources/<id>/. A source with no directory is
// unavailable. Only regular .md files are read; symbolic links and other
// non-regular entries are skipped, so a source cannot reach outside its own
// directory. The caller chooses fsys (and so the root); this function opens
// nothing else.
func LoadSnapshot(fsys fs.FS, lim Limits) (*Snapshot, error) {
	lim = lim.orDefault()
	raw, err := readLimited(fsys, "snapshot.json", lim.MaxFileBytes)
	if err != nil {
		return nil, snapshotErr("snapshot.json: %v", err)
	}
	var sf snapshotFile
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&sf); err != nil {
		return nil, snapshotErr("snapshot.json: %v", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, snapshotErr("snapshot.json: trailing data after the snapshot object")
	}
	s := &Snapshot{DeclaredBy: sf.DeclaredBy, Terms: sf.Terms, DecidingRole: sf.DecidingRole, Limits: lim}
	if err := s.decodeTarget(sf.Target); err != nil {
		return nil, err
	}
	for _, src := range sf.Sources {
		s.Sources = append(s.Sources, SnapshotSource{ID: src.ID, Cell: src.Cell, Repo: src.Repo, Role: src.Role, Required: src.Required})
	}
	for _, e := range sf.Edges {
		s.Edges = append(s.Edges, DeclaredEdge(e))
	}
	for _, c := range sf.Claims {
		s.Claims = append(s.Claims, Claim(c))
	}
	for _, p := range sf.Proposals {
		if !fs.ValidPath(p) || p == "." {
			return nil, snapshotErr("proposal %q is not a path inside the snapshot", p)
		}
		b, err := readLimited(fsys, p, lim.MaxFileBytes)
		if err != nil {
			return nil, snapshotErr("proposal %s: %v", p, err)
		}
		h, err := DecodeHandoff(b)
		if err != nil {
			return nil, snapshotErr("proposal %s: %v", p, err)
		}
		s.Proposals = append(s.Proposals, Proposal{Locator: p, Handoff: h})
	}
	if sf.Product != nil {
		s.Product = &SnapshotProduct{Targets: []ProductTarget{}, Relationships: []ProductRelationship{}}
		for _, t := range sf.Product.Targets {
			s.Product.Targets = append(s.Product.Targets, ProductTarget(t))
		}
		for _, rel := range sf.Product.Relationships {
			s.Product.Relationships = append(s.Product.Relationships, ProductRelationship(rel))
		}
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	for i := range s.Sources {
		src := &s.Sources[i]
		files, ok := readSource(fsys, "sources/"+src.ID, lim)
		if !ok {
			src.Unavailable, src.Files = true, nil
			continue
		}
		src.Files = files
	}
	return s, nil
}

func (s *Snapshot) decodeTarget(raw json.RawMessage) error {
	if len(raw) == 0 {
		return snapshotErr("target: required field is absent")
	}
	tree, perr := parseStrict(raw)
	if perr != nil {
		return snapshotErr("target: %v", perr)
	}
	r := &reader{}
	s.Target = r.target(tree, "$.target")
	if r.err != nil {
		return snapshotErr("%v", r.err)
	}
	return nil
}

// Validate checks the snapshot's own declarations. It does not read files.
func (s *Snapshot) Validate() error {
	decl, err := ResolveQualifiedID(s.DeclaredBy, nil)
	if err != nil {
		return snapshotErr("declared_by: %v", err)
	}
	if err := checkTarget(s.Target); err != nil {
		return snapshotErr("%v", err)
	}
	if !nameRe.MatchString(s.DecidingRole) {
		return snapshotErr("deciding_role %q is not a role name", s.DecidingRole)
	}
	for _, t := range s.Terms {
		if !locatorRe.MatchString(t) {
			return snapshotErr("term %q is empty or carries whitespace", t)
		}
	}
	ids := map[string]bool{}
	for _, src := range s.Sources {
		switch {
		case !nameRe.MatchString(src.ID):
			return snapshotErr("source id %q is not a name", src.ID)
		case ids[src.ID]:
			return snapshotErr("source %q is listed twice", src.ID)
		case !idSegmentRe.MatchString(src.Cell) || !idSegmentRe.MatchString(src.Repo):
			return snapshotErr("source %q: cell and repo are lower-case names", src.ID)
		case src.Role != RolePlan && src.Role != RoleGuideline:
			return snapshotErr("source %q: role %q is not %s or %s", src.ID, src.Role, RolePlan, RoleGuideline)
		}
		ids[src.ID] = true
	}
	for i := range s.Edges {
		e := &s.Edges[i]
		q, err := ResolveQualifiedID(e.Subject, &decl)
		if err != nil {
			return snapshotErr("edge %d subject: %v", i, err)
		}
		e.Subject = q.String()
		if !contains(edges, e.Edge) {
			return snapshotErr("edge %d: %q is not one of %v", i, e.Edge, edges)
		}
		if !locatorRe.MatchString(e.Term) || !locatorRe.MatchString(e.Locator) {
			return snapshotErr("edge %d: term and locator are non-empty and carry no whitespace", i)
		}
		if e.Version != "" && !locatorRe.MatchString(e.Version) {
			return snapshotErr("edge %d: version carries whitespace", i)
		}
	}
	for i := range s.Claims {
		c := &s.Claims[i]
		q, err := ResolveQualifiedID(c.Subject, &decl)
		if err != nil {
			return snapshotErr("claim %d subject: %v", i, err)
		}
		c.Subject = q.String()
		switch {
		case !locatorRe.MatchString(c.Claim):
			return snapshotErr("claim %d: claim is a non-empty locator", i)
		case !revisionRe.MatchString(c.BriefRevision):
			return snapshotErr("claim %d: brief_revision is 7-64 lower-case hex", i)
		case !timestampRe.MatchString(c.ObservedAt):
			return snapshotErr("claim %d: observed_at is a UTC timestamp", i)
		case !nameRe.MatchString(c.Owner):
			return snapshotErr("claim %d: owner is a role name", i)
		case !textRe.MatchString(c.Checkpoint):
			return snapshotErr("claim %d: checkpoint is non-blank text", i)
		}
	}
	if s.Product != nil {
		return s.Product.validate(decl)
	}
	return nil
}

func readLimited(fsys fs.FS, name string, max int64) ([]byte, error) {
	f, err := fsys.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, fmt.Errorf("larger than the %d-byte read limit", max)
	}
	return b, nil
}

// readSource reads every regular .md file under dir. It reports false when the
// directory is absent or any read limit is exceeded: the source is then
// unavailable as a whole, never partially read as if complete.
func readSource(fsys fs.FS, dir string, lim Limits) (map[string][]byte, bool) {
	st, err := fs.Stat(fsys, dir)
	if err != nil || !st.IsDir() {
		return nil, false
	}
	files := map[string][]byte{}
	var total int64
	ok := true
	err = fs.WalkDir(fsys, dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if p != dir && strings.HasPrefix(name, ".") {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || path.Ext(name) != ".md" {
			return nil
		}
		if len(files) >= lim.MaxFiles {
			ok = false
			return fs.SkipAll
		}
		b, err := readLimited(fsys, p, lim.MaxFileBytes)
		if err != nil {
			ok = false
			return fs.SkipAll
		}
		total += int64(len(b))
		if total > lim.MaxTotalBytes {
			ok = false
			return fs.SkipAll
		}
		files[strings.TrimPrefix(p, dir+"/")] = b
		return nil
	})
	if err != nil || !ok {
		return nil, false
	}
	return files, true
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
