// Package workref owns pure work-reference interpretation. It grants no authority
// and does not establish record existence, revision availability or acceptance.
package workref

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Kind describes logical work, independently of spelling or revision.
type Kind string

const (
	Brief     Kind = "brief"
	Milestone Kind = "milestone"
)

// Profile preserves each existing boundary's acceptance set. Typed is only for
// explicitly negotiated adapters; old entrypoints must select their legacy profile.
type Profile uint8

const (
	Typed Profile = iota
	LegacyQualified
	LegacyTrailer
	LegacyCrossRepo
	TypedBrief
	TypedMilestone
)

// Identity is logical work. Version selectors and source locators are separate.
type Identity struct {
	Kind                       Kind
	Cell, Repo, Stream, Number string
}

func (id Identity) String() string {
	prefix := "b-"
	if id.Kind == Milestone {
		prefix = "m-"
	}
	return id.Cell + ":" + id.Repo + ":" + id.Stream + ":" + prefix + id.Number
}

// Compatibility returns the existing brief fullId; milestones have no brief alias.
func (id Identity) Compatibility() string {
	if id.Kind != Brief {
		return ""
	}
	return id.Cell + ":" + id.Repo + ":" + id.Stream + ":" + id.Number
}

// Reference retains the exact input and optional exact positive revision selector.
// Empty Cell/Repo indicate elision. Revision zero means no supplied selector.
type Reference struct {
	Identity
	Revision uint64
	Spelling string
}

// Context is the declaration's actual owning root, never a search hint.
type Context struct{ Cell, Repo string }

// Root is a canonical graph-registry root and its repository aliases in that cell.
type Root struct {
	Cell, Repo string
	Aliases    []string
}

// Binding supplies an existing stable key/UUID and historical identity aliases.
// Neither Parse nor Resolve invents or remints a UUID.
type Binding struct {
	Identity Identity
	Key      string
	Aliases  []Identity
}

// Registry is an immutable copy of the supplied graph and identity registry.
// Its zero value has no known roots. Host adapters own registry I/O.
type Registry struct {
	roots    map[Context]Context
	bindings map[Identity]Binding
}

// NewRegistry refuses namespace collisions, including aliases of different keys.
func NewRegistry(roots []Root, bindings []Binding) (Registry, error) {
	r := Registry{roots: map[Context]Context{}, bindings: map[Identity]Binding{}}
	for _, root := range roots {
		if !nameRe.MatchString(root.Cell) || !nameRe.MatchString(root.Repo) {
			return Registry{}, fmt.Errorf("invalid registry root")
		}
		canonical := Context{root.Cell, root.Repo}
		for _, repo := range append([]string{root.Repo}, root.Aliases...) {
			if !nameRe.MatchString(repo) {
				return Registry{}, fmt.Errorf("invalid repository alias %q", repo)
			}
			alias := Context{root.Cell, repo}
			if prior, ok := r.roots[alias]; ok && prior != canonical {
				return Registry{}, fmt.Errorf("repository alias collision: %s:%s", root.Cell, repo)
			}
			r.roots[alias] = canonical
		}
	}
	keys := map[string]Identity{}
	for _, binding := range bindings {
		if binding.Key == "" {
			return Registry{}, fmt.Errorf("empty logical key")
		}
		canonical, err := r.canonical(binding.Identity)
		if err != nil {
			return Registry{}, err
		}
		if prior, ok := keys[binding.Key]; ok && prior != canonical {
			return Registry{}, fmt.Errorf("logical key has multiple canonical identities")
		}
		keys[binding.Key] = canonical
		copyBinding := Binding{Identity: canonical, Key: binding.Key}
		for _, id := range append([]Identity{binding.Identity}, binding.Aliases...) {
			id, err = r.canonical(id)
			if err != nil {
				return Registry{}, err
			}
			if id.Kind != canonical.Kind {
				return Registry{}, fmt.Errorf("alias changes work kind")
			}
			if prior, ok := r.bindings[id]; ok && (prior.Key != binding.Key || prior.Identity != canonical) {
				return Registry{}, fmt.Errorf("work alias collision: %s", id)
			}
			r.bindings[id] = copyBinding
		}
	}
	return r, nil
}

var (
	nameRe      = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	numberRe    = regexp.MustCompile(`^[0-9]{2,4}[a-z]?$`)
	milestoneRe = regexp.MustCompile(`^[0-9]{2,}$`)
	issueRe     = regexp.MustCompile(`^[A-Za-z0-9._/-]*#[0-9]+$`)
	crossRepoRe = regexp.MustCompile(`^([a-z][a-z0-9_-]{0,31}):([^:/\s]+)/([0-9]+)$`)
)

// Parse interprets syntax only. Typed accepts the contract's legacy aliases;
// legacy profiles retain their deliberately different historical acceptance sets.
func Parse(ref string, profile Profile) (Reference, error) {
	if profile == LegacyTrailer {
		return parseTrailer(ref)
	}
	if profile == LegacyCrossRepo {
		m := crossRepoRe.FindStringSubmatch(strings.TrimSpace(ref))
		if m == nil {
			return Reference{}, fmt.Errorf("not a cross-repository brief trailer")
		}
		return Reference{Identity: Identity{Kind: Brief, Repo: m[1], Stream: m[2], Number: m[3]}, Spelling: ref}, nil
	}
	if profile == LegacyQualified {
		return parseQualified(ref)
	}
	if profile != Typed && profile != TypedBrief && profile != TypedMilestone {
		return Reference{}, fmt.Errorf("unknown work-reference profile")
	}
	if ref == "" || strings.IndexFunc(ref, func(r rune) bool { return unicode.IsSpace(r) || r == '\ufeff' }) >= 0 {
		return Reference{}, fmt.Errorf("work reference is empty or carries whitespace")
	}
	text := ref
	var revision uint64
	if i := strings.IndexByte(text, '@'); i >= 0 {
		raw := text[i+1:]
		if !digits(raw) {
			return Reference{}, fmt.Errorf("revision must be an exact positive integer")
		}
		var err error
		revision, err = strconv.ParseUint(raw, 10, 64)
		if err != nil || revision == 0 {
			return Reference{}, fmt.Errorf("revision must be an exact positive integer within uint64")
		}
		text = text[:i]
	}
	slash := strings.Contains(text, "/")
	if slash {
		if strings.Count(text, "/") != 1 {
			return Reference{}, fmt.Errorf("work reference is not a path")
		}
		if strings.Contains(text[strings.IndexByte(text, '/')+1:], ":") {
			return Reference{}, fmt.Errorf("slash must precede the final segment")
		}
		text = strings.Replace(text, "/", ":", 1)
	}
	parts := strings.Split(text, ":")
	if len(parts) < 2 || len(parts) > 4 {
		return Reference{}, fmt.Errorf("expected two to four work-reference segments")
	}
	last := parts[len(parts)-1]
	kind := Brief
	prefixed := strings.HasPrefix(last, "b-") || strings.HasPrefix(last, "m-")
	if strings.HasPrefix(last, "m-") {
		kind = Milestone
	}
	if (profile == TypedBrief && kind != Brief) || (profile == TypedMilestone && kind != Milestone) {
		return Reference{}, fmt.Errorf("wrong work kind")
	}
	number := last
	if prefixed {
		number = last[2:]
	}
	if !prefixed && revision != 0 {
		return Reference{}, fmt.Errorf("legacy unprefixed references carry no revision selector")
	}
	// Draft prefixed slash aliases are only stream/ID or full cell:repo:stream/ID.
	if slash && prefixed && len(parts) == 3 {
		return Reference{}, fmt.Errorf("unsupported prefixed slash alias")
	}
	if kind == Milestone {
		n, err := strconv.ParseUint(number, 10, 64)
		if !milestoneRe.MatchString(number) || err != nil || n == 0 || revision == 0 {
			return Reference{}, fmt.Errorf("milestone needs a padded positive number and exact positive version")
		}
	} else if !numberRe.MatchString(number) {
		return Reference{}, fmt.Errorf("invalid brief number %q", number)
	}
	for _, name := range parts[:len(parts)-1] {
		if !nameRe.MatchString(name) {
			return Reference{}, fmt.Errorf("invalid work-reference name %q", name)
		}
	}
	id := Identity{Kind: kind, Stream: parts[len(parts)-2], Number: number}
	if len(parts) >= 3 {
		id.Repo = parts[len(parts)-3]
	}
	if len(parts) == 4 {
		id.Cell = parts[0]
	}
	return Reference{Identity: id, Revision: revision, Spelling: ref}, nil
}

// Result keeps logical identity/key, exact revision and compatibility spelling
// separate. Without a supplied key, LogicalKey is the canonical typed identity;
// this does not assert that the record or requested revision exists.
type Result struct {
	Reference
	LogicalKey    string
	Compatibility string
}

// Resolve expands against the declaration then normalizes supplied aliases.
// Typed requires known roots; the legacy QID boundary historically did not.
func Resolve(ref string, declaring *Context, registry Registry, profile Profile) (Result, error) {
	parsed, err := Parse(ref, profile)
	if err != nil {
		return Result{}, err
	}
	if profile != Typed && profile != TypedBrief && profile != TypedMilestone && profile != LegacyQualified {
		return Result{}, fmt.Errorf("trailer reduction is not qualified resolution")
	}
	if parsed.Cell == "" {
		if declaring == nil {
			return Result{}, fmt.Errorf("%q: an elided reference needs a declaring record; the declaring identity itself is always <cell>:<repo>:<stream>:<NN>", ref)
		}
		parsed.Cell = declaring.Cell
		if parsed.Repo == "" {
			parsed.Repo = declaring.Repo
		}
	}
	key := parsed.String()
	if profile != LegacyQualified {
		id, err := registry.canonical(parsed.Identity)
		if err != nil {
			return Result{}, err
		}
		parsed.Identity = id
		key = id.String()
		if binding, ok := registry.bindings[id]; ok {
			parsed.Identity, key = binding.Identity, binding.Key
		}
	}
	return Result{Reference: parsed, LogicalKey: key, Compatibility: parsed.Compatibility()}, nil
}
func (r Registry) canonical(id Identity) (Identity, error) {
	if id.Kind != Brief && id.Kind != Milestone {
		return Identity{}, fmt.Errorf("unknown work kind")
	}
	if !nameRe.MatchString(id.Cell) || !nameRe.MatchString(id.Repo) || !nameRe.MatchString(id.Stream) {
		return Identity{}, fmt.Errorf("invalid owning context")
	}
	if id.Kind == Brief && !numberRe.MatchString(id.Number) {
		return Identity{}, fmt.Errorf("invalid brief identity")
	}
	if id.Kind == Milestone {
		n, err := strconv.ParseUint(id.Number, 10, 64)
		if !milestoneRe.MatchString(id.Number) || err != nil || n == 0 {
			return Identity{}, fmt.Errorf("invalid milestone identity")
		}
	}
	root, ok := r.roots[Context{id.Cell, id.Repo}]
	if !ok {
		return Identity{}, fmt.Errorf("unknown owning root %s:%s", id.Cell, id.Repo)
	}
	id.Cell, id.Repo = root.Cell, root.Repo
	return id, nil
}
func digits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
