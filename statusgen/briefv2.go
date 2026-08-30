package main

// briefv2.go — the brief-v2 parser and validator (derived-board/03).
//
// brief-v2 is a fleet-wide schema flag-day (docs/streams/derived-board/spec.md §5):
// the hand-edited lifecycle cell becomes a generated surface, and the same bump
// RESERVES the dependency-graph keys (docs/dependency-graph-design.md §3.3–3.6) so
// the fleet takes one migration instead of two. "Reserved" is precise: every key
// below is PARSED, type-checked and lint-validated here, but its GATING behaviour
// is deferred to the graph stream. An old pinned statusgen refuses a v2 tree
// (fail-closed, #271, in parseBriefFile) rather than misreading it.
//
// This file holds:
//   - the reserved-key YAML parsers (parseBriefV2Keys, edgeList),
//   - the alias registry loader (loadGraphRepos over docs/streams/graph-repos.yaml),
//   - the §3.3 reference-grammar validator (validGraphRef),
//   - the semantic validator run from checkBriefFiles (checkBriefV2Semantics).
//
// Nothing here reads the network or the git index; it is pure over the tree, the
// same offline envelope every other --lint check keeps.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// GraphEdge is one reserved `gates:` or `feathers:` edge. Ref is the target
// reference in the §3.3 grammar; Type is the edge taxonomy (§3.1); Reason is the
// machine-attached why. For a `feathers:` scalar entry Type defaults to build-dep
// and Reason is empty; every `gates:` entry carries both explicitly.
type GraphEdge struct {
	Ref    string
	Type   string
	Reason string
}

// VerifyRowMeta is the OPTIONAL per-Verify-row identity record carried in a
// brief-v2 `verify:` frontmatter list — {id, target}. id is a short row label
// (v1, v2, …); target names the verify substrate (a sibling repo, a live cluster)
// so the could-not-check-by-design class becomes a field instead of a NOTICE.
// Shape-only under v2 (reserved).
type VerifyRowMeta struct {
	ID     string
	Target string
}

// validEdgeTypes is the edge-type taxonomy from docs/dependency-graph-design.md
// §3.1. An edge whose type is outside this set is a PROBLEM (the reason the field
// is typed at all is to make an unknown type loud rather than silently dispatched
// past).
var validEdgeTypes = map[string]bool{
	"build-dep":        true,
	"ordering-gate":    true,
	"behavioural-gate": true,
	"human-gate":       true,
	"external-env":     true,
	"external-repo":    true,
}

func validEdgeTypeList() string {
	names := make([]string, 0, len(validEdgeTypes))
	for k := range validEdgeTypes {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// uuidV4Re matches the canonical uuid v4 shape: 8-4-4-4-12 lowercase hex with the
// version nibble `4` and the variant nibble in {8,9,a,b}. Shape only — no registry
// lookup; a uuid is an identity, not a reference.
var uuidV4Re = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// Reference-grammar (§3.3) recognizers. The grammar is hierarchical and
// elision-based; each form below is one legal shape a ref may take.
var (
	// <stream>/<NN>  — in-repo brief (the existing brief-v1 form).
	refInRepoBriefRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*/[0-9]+[a-z]?$`)
	// <alias>:<stream>/<NN>  — cross-repo brief.
	refCrossRepoBriefRe = regexp.MustCompile(`^[a-z0-9]+:[a-z0-9][a-z0-9-]*/[0-9]+[a-z]?$`)
	// <cell>:<alias>:<stream>/<NN>  — cross-cell brief (reserved form).
	refCrossCellBriefRe = regexp.MustCompile(`^[a-z0-9]+:[a-z0-9]+:[a-z0-9][a-z0-9-]*/[0-9]+[a-z]?$`)
	// #<issue>  — in-repo issue.
	refInRepoIssueRe = regexp.MustCompile(`^#[0-9]+$`)
	// <alias>#<issue>  — cross-repo issue.
	refCrossRepoIssueRe = regexp.MustCompile(`^[a-z0-9]+#[0-9]+$`)
)

// graphRepos is the parsed docs/streams/graph-repos.yaml alias registry
// (schema graph-repos-v1). Aliases map to their cell + (published) repo; an
// entry whose repo is withheld carries Unpublished.
type graphRepos struct {
	Cell    string
	Aliases map[string]graphRepoEntry
}

type graphRepoEntry struct {
	Cell        string
	Repo        string
	Unpublished bool
}

// loadGraphRepos reads docs/streams/graph-repos.yaml from a root. It returns
// (nil, false, nil) when the file is absent (a v1 tree needs no registry), a
// non-nil error on a malformed/ill-typed file, and (registry, true, nil) when it
// parses. A v2 tree REQUIRES the registry; that requirement is enforced by the
// caller (checkBriefV2Semantics), which turns absence into a PROBLEM only when a
// v2 brief is actually present.
func loadGraphRepos(root string) (*graphRepos, bool, error) {
	path := filepath.Join(root, "docs", "streams", "graph-repos.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("%s: %w", path, err)
	}
	var doc struct {
		Schema string `yaml:"schema"`
		Cell   string `yaml:"cell"`
		Repos  map[string]struct {
			Cell        string `yaml:"cell"`
			Repo        string `yaml:"repo"`
			Unpublished bool   `yaml:"unpublished"`
		} `yaml:"repos"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, false, fmt.Errorf("%s: %w", path, err)
	}
	if doc.Schema != "graph-repos-v1" {
		return nil, false, fmt.Errorf("%s: schema must be graph-repos-v1, got %q", path, doc.Schema)
	}
	if doc.Cell == "" {
		return nil, false, fmt.Errorf("%s: cell must be non-empty", path)
	}
	reg := &graphRepos{Cell: doc.Cell, Aliases: map[string]graphRepoEntry{}}
	for alias, e := range doc.Repos {
		reg.Aliases[alias] = graphRepoEntry{Cell: e.Cell, Repo: e.Repo, Unpublished: e.Unpublished}
	}
	return reg, true, nil
}

// validGraphRef reports whether ref is well-formed under the §3.3 grammar AND,
// when it names a repo alias or a cell, whether that alias/cell resolves in the
// registry. A shape-valid ref to an UNKNOWN alias is invalid (the closed-set
// property the grammar exists to give). The returned reason is "" on success.
func validGraphRef(ref string, reg *graphRepos) (ok bool, reason string) {
	switch {
	case refInRepoBriefRe.MatchString(ref), refInRepoIssueRe.MatchString(ref):
		// No alias/cell segment — always in the declaring repo. Shape is enough.
		return true, ""
	case refCrossRepoBriefRe.MatchString(ref):
		alias := ref[:strings.IndexByte(ref, ':')]
		return aliasKnown(alias, reg)
	case refCrossRepoIssueRe.MatchString(ref):
		alias := ref[:strings.IndexByte(ref, '#')]
		return aliasKnown(alias, reg)
	case refCrossCellBriefRe.MatchString(ref):
		// <cell>:<alias>:<stream>/<NN> — validate the alias segment (2nd).
		first := strings.IndexByte(ref, ':')
		rest := ref[first+1:]
		alias := rest[:strings.IndexByte(rest, ':')]
		return aliasKnown(alias, reg)
	default:
		return false, fmt.Sprintf("%q is not a valid reference (want <stream>/<NN>, <alias>:<stream>/<NN>, <alias>#<NNN>, #<NNN> or <cell>:<alias>:<stream>/<NN>)", ref)
	}
}

func aliasKnown(alias string, reg *graphRepos) (bool, string) {
	if reg == nil {
		return false, fmt.Sprintf("alias %q cannot be resolved — no docs/streams/graph-repos.yaml registry", alias)
	}
	if _, ok := reg.Aliases[alias]; !ok {
		return false, fmt.Sprintf("alias %q is not in docs/streams/graph-repos.yaml", alias)
	}
	return true, ""
}

// parseBriefV2Keys parses the brief-v2 reserved keys off the raw frontmatter map
// into bf. Only TYPE/SHAPE errors are raised here (via addBad, which fails the
// parse); semantic validation (registry resolution, ref grammar, id uniqueness,
// hierarchical brief: form) runs later in checkBriefV2Semantics where the whole
// tree and the registry are in hand.
func parseBriefV2Keys(data map[string]any, bf *BriefFile, addBad func(string, ...any)) {
	// version: an int >= 1. Absent defaults to 1 (a brief is at revision 1 at
	// authoring). A wrong type is a parse error; a present int < 1 is caught in
	// the semantic pass (it needs no tree, but keeping every value-range check in
	// one place keeps parse strictly about shape).
	if v, ok := data["version"]; ok {
		switch n := v.(type) {
		case int:
			bf.Version = n
		case int64:
			bf.Version = int(n)
		default:
			addBad("version must be an integer")
		}
	} else {
		bf.Version = 1
	}
	// gates: a list of mappings, each REQUIRING on/type/reason (§3.4).
	if v, ok := data["gates"]; ok {
		edges, err := edgeList(v, true)
		if err != nil {
			addBad("gates: %v", err)
		} else {
			bf.Gates = edges
		}
	}
	// feathers: a list of scalars (build-dep, no reason) or mappings {ref, type,
	// reason} (§3.3).
	if v, ok := data["feathers"]; ok {
		edges, err := edgeList(v, false)
		if err != nil {
			addBad("feathers: %v", err)
		} else {
			bf.Feathers = edges
		}
	}
	// id: a string (uuid v4 shape validated in the semantic pass).
	if v, ok := data["id"]; ok {
		if s, ok := v.(string); ok {
			bf.ID = s
		} else {
			addBad("id must be a string")
		}
	}
	// supersedes: a list of refs (grammar validated in the semantic pass).
	if v, ok := data["supersedes"]; ok {
		if list, err := stringList(v); err == nil {
			bf.Supersedes = list
		} else {
			addBad("supersedes: %v", err)
		}
	}
	// verify: an OPTIONAL list of {id, target} per-Verify-row identity records.
	if v, ok := data["verify"]; ok {
		rows, err := verifyRowMetaList(v)
		if err != nil {
			addBad("verify: %v", err)
		} else {
			bf.VerifyRows = rows
		}
	}
}

// edgeList coerces a YAML value into []GraphEdge. When requireMapping is true
// (gates:), every entry must be a mapping carrying on/type/reason. When false
// (feathers:), a scalar entry is accepted as a build-dep with no reason, and a
// mapping entry carries {ref, type, reason} with type defaulting to build-dep.
// An unknown key in a mapping is REJECTED rather than ignored — a silently
// dropped key is an edge whose meaning nobody reads.
func edgeList(v any, requireMapping bool) ([]GraphEdge, error) {
	if v == nil {
		return nil, nil
	}
	items, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("must be a list")
	}
	out := make([]GraphEdge, 0, len(items))
	for i, it := range items {
		switch e := it.(type) {
		case string:
			if requireMapping {
				return nil, fmt.Errorf("entry %d must be a mapping with on, type and reason", i+1)
			}
			out = append(out, GraphEdge{Ref: e, Type: "build-dep"})
		case map[string]any:
			var edge GraphEdge
			refKey := "ref"
			if requireMapping {
				refKey = "on"
			}
			for k := range e {
				if k != refKey && k != "type" && k != "reason" {
					return nil, fmt.Errorf("entry %d has unknown key %q (only %s, type and reason are defined)", i+1, k, refKey)
				}
			}
			if r, ok := e[refKey].(string); ok && r != "" {
				edge.Ref = r
			} else {
				return nil, fmt.Errorf("entry %d: %s must be a non-empty string", i+1, refKey)
			}
			if t, present := e["type"]; present {
				s, ok := t.(string)
				if !ok {
					return nil, fmt.Errorf("entry %d: type must be a string", i+1)
				}
				edge.Type = s
			} else if requireMapping {
				return nil, fmt.Errorf("entry %d: type is required", i+1)
			} else {
				edge.Type = "build-dep"
			}
			if rz, present := e["reason"]; present {
				s, ok := rz.(string)
				if !ok {
					return nil, fmt.Errorf("entry %d: reason must be a string", i+1)
				}
				edge.Reason = s
			} else if requireMapping {
				return nil, fmt.Errorf("entry %d: reason is required", i+1)
			}
			out = append(out, edge)
		default:
			return nil, fmt.Errorf("entry %d must be a %s", i+1, mappingOrScalar(requireMapping))
		}
	}
	return out, nil
}

func mappingOrScalar(requireMapping bool) string {
	if requireMapping {
		return "mapping with on, type and reason"
	}
	return "reference string or a mapping with ref, type and reason"
}

// verifyRowMetaList coerces a YAML value into []VerifyRowMeta. Each entry is a
// mapping with an optional id and target; an unknown key is rejected.
func verifyRowMetaList(v any) ([]VerifyRowMeta, error) {
	if v == nil {
		return nil, nil
	}
	items, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("must be a list of {id, target} mappings")
	}
	out := make([]VerifyRowMeta, 0, len(items))
	for i, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("entry %d must be a mapping with id and/or target", i+1)
		}
		var row VerifyRowMeta
		for k := range m {
			if k != "id" && k != "target" {
				return nil, fmt.Errorf("entry %d has unknown key %q (only id and target are defined)", i+1, k)
			}
		}
		if s, ok := m["id"]; ok {
			str, ok := s.(string)
			if !ok {
				return nil, fmt.Errorf("entry %d: id must be a string", i+1)
			}
			row.ID = str
		}
		if s, ok := m["target"]; ok {
			str, ok := s.(string)
			if !ok {
				return nil, fmt.Errorf("entry %d: target must be a string", i+1)
			}
			row.Target = str
		}
		out = append(out, row)
	}
	return out, nil
}

// parseBriefV2ID splits a hierarchical brief-v2 `brief:` id into its four
// segments — <cell>:<repo>:<stream>:<NN>. ok is false when the shape is wrong.
func parseBriefV2ID(id string) (cell, repoAlias, stream, num string, ok bool) {
	parts := strings.Split(id, ":")
	if len(parts) != 4 {
		return "", "", "", "", false
	}
	for _, p := range parts {
		if p == "" {
			return "", "", "", "", false
		}
	}
	return parts[0], parts[1], parts[2], parts[3], true
}

// checkBriefV2Semantics runs the brief-v2 semantic validations for one brief file
// that has already parsed: the hierarchical brief: form against the registry and
// the path, the uuid v4 shape of id:, the §3.3 grammar of every reserved edge and
// supersedes ref, and the version range. It appends PROBLEMs via add and the
// reserved-not-gating summary via notice. id-uniqueness across the tree is
// handled by the caller (it needs every brief's id at once).
//
// reg is the loaded registry (may be nil — absence is itself a PROBLEM for a v2
// brief, raised here). stream and num are the path-derived identity from
// expectedBriefID.
func checkBriefV2Semantics(add, notice func(string, ...any), path string, bf *BriefFile, reg *graphRepos, stream, num string) {
	// A v2 tree REQUIRES the registry.
	if reg == nil {
		add("%s: brief is schema brief-v2 but docs/streams/graph-repos.yaml (schema graph-repos-v1) is absent — a v2 tree requires the alias registry", path)
	}
	// The v2 `brief:` id is the full hierarchical form <cell>:<repo>:<stream>:<NN>.
	// The brief-v1 `/` form is refused in a v2 file (the migration rewrites it).
	if strings.Contains(bf.Brief, "/") && !strings.Contains(bf.Brief, ":") {
		add("%s: brief %q uses the brief-v1 <stream>/<NN> form, but this is a brief-v2 file — v2 ids are the hierarchical <cell>:<repo>:<stream>:<NN> form", path, bf.Brief)
	} else {
		cell, alias, brStream, brNum, ok := parseBriefV2ID(bf.Brief)
		if !ok {
			add("%s: brief %q is not a hierarchical <cell>:<repo>:<stream>:<NN> id", path, bf.Brief)
		} else {
			if brStream != stream || brNum != num {
				add("%s: brief %q stream/NN (%s/%s) does not match the file path (%s/%s)", path, bf.Brief, brStream, brNum, stream, num)
			}
			if reg != nil {
				if cell != reg.Cell {
					add("%s: brief %q cell %q does not match docs/streams/graph-repos.yaml cell %q", path, bf.Brief, cell, reg.Cell)
				}
				if _, known := reg.Aliases[alias]; !known {
					add("%s: brief %q repo alias %q is not in docs/streams/graph-repos.yaml", path, bf.Brief, alias)
				}
			}
		}
	}
	// version range.
	if bf.Version < 1 {
		add("%s: version must be an integer >= 1, got %d", path, bf.Version)
	}
	// id: uuid v4 shape (uniqueness is a tree-wide check in the caller).
	if bf.ID != "" && !uuidV4Re.MatchString(bf.ID) {
		add("%s: id %q is not a uuid v4", path, bf.ID)
	}
	// Reserved edges: validate the ref grammar + edge type. gates and feathers
	// alike; reason presence was already enforced at parse for gates.
	for _, e := range bf.Gates {
		validateReservedEdge(add, path, "gates", e, reg)
	}
	for _, e := range bf.Feathers {
		validateReservedEdge(add, path, "feathers", e, reg)
	}
	// supersedes refs.
	for _, ref := range bf.Supersedes {
		if ok, reason := validGraphRef(ref, reg); !ok {
			add("%s: supersedes ref %s", path, reason)
		}
	}
	// Reserved-not-gating summary: make it visible on --lint that the edges were
	// read and are deliberately inert this schema. NOTICE severity — it never
	// changes the exit code, and it is what distinguishes "parsed and reserved"
	// from "silently ignored".
	if n := len(bf.Gates); n > 0 {
		notice("%s: gates: %s (reserved, not gating)", path, edgeCountPhrase(n))
	}
	if n := len(bf.Feathers); n > 0 {
		notice("%s: feathers: %s (reserved, not gating)", path, edgeCountPhrase(n))
	}
}

func validateReservedEdge(add func(string, ...any), path, field string, e GraphEdge, reg *graphRepos) {
	if !validEdgeTypes[e.Type] {
		add("%s: %s edge on %q has unknown type %q (want one of: %s)", path, field, e.Ref, e.Type, validEdgeTypeList())
	}
	if ok, reason := validGraphRef(e.Ref, reg); !ok {
		add("%s: %s edge ref %s", path, field, reason)
	}
}

// edgeCountPhrase renders the reserved-edge count with correct grammar:
// "1 edge", "2 edges", … so the lint line reads naturally.
func edgeCountPhrase(n int) string {
	if n == 1 {
		return "1 edge"
	}
	return fmt.Sprintf("%d edges", n)
}
