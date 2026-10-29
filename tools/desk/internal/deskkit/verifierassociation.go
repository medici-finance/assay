package deskkit

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/perfrec"
	"gopkg.in/yaml.v3"
)

// VerifierDeliverables resolves declarations from the immutable tracking commit.
// An absent declaration means same-repository work. Alias values always come
// from that commit's graph registry; neither a queued payload nor live edited
// frontmatter can substitute another deliverable.
func VerifierDeliverables(root, revision, brief string) ([]string, error) {
	raw, err := verifierGitBytes(root, "cat-file", "blob", revision+":"+filepath.ToSlash(brief))
	if err != nil {
		return nil, Unverifiable("cannot read verifier association brief", err)
	}
	return resolveVerifierDeliverables(string(raw), func() (*verifierAssociationRegistry, error) {
		raw, err := verifierGitBytes(root, "cat-file", "blob", revision+":docs/streams/graph-repos.yaml")
		if err != nil {
			return nil, Unverifiable("missing immutable verifier association registry", err)
		}
		return parseVerifierAssociationRegistry(string(raw))
	})
}

// resolveVerifierDeliverables keeps declaration policy shared by planning and
// admission. Only the caller's immutable registry observation may resolve aliases.
func resolveVerifierDeliverables(raw string, registry func() (*verifierAssociationRegistry, error)) ([]string, error) {
	var fm struct {
		Deliverable string `yaml:"deliverable_repo"`
		Homed       string `yaml:"homed-in"`
	}
	lines := strings.Split(raw, "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		end := 0
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				end = i
				break
			}
		}
		if end == 0 {
			return nil, Refused("unclosed verifier brief frontmatter")
		}
		if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &fm); err != nil {
			return nil, Refused("invalid verifier brief frontmatter")
		}
	}
	tags := BriefFilesTags(raw)
	if fm.Deliverable == "" && fm.Homed == "" && len(tags) == 0 {
		return nil, nil
	}
	graph, err := registry()
	if err != nil {
		return nil, err
	}
	resolve := func(alias string) (string, error) {
		entry, ok := graph.Repos[alias]
		if !ok || entry.Repo == nil || !ValidRepoSlug(strings.TrimSpace(*entry.Repo)) {
			return "", Unverifiable("verifier deliverable alias is unavailable in its immutable registry", nil)
		}
		return strings.TrimSpace(*entry.Repo), nil
	}
	if fm.Deliverable != "" {
		repo, err := resolve(fm.Deliverable)
		if err != nil {
			return nil, err
		}
		if fm.Homed != "" && !strings.EqualFold(fm.Homed, repo) {
			return nil, Refused("conflicting verifier deliverable declarations")
		}
		return []string{repo}, nil
	}
	if fm.Homed != "" {
		for alias := range graph.Repos {
			if repo, err := resolve(alias); err == nil && strings.EqualFold(repo, fm.Homed) {
				return []string{repo}, nil
			}
		}
		return nil, Refused("verifier homed-in repository is not registered")
	}
	var repos []string
	for _, tag := range tags {
		repo, err := resolve(tag)
		if err != nil {
			return nil, err
		}
		found := false
		for _, existing := range repos {
			found = found || strings.EqualFold(existing, repo)
		}
		if !found {
			repos = append(repos, repo)
		}
	}
	return repos, nil
}

func verifierAssociation(root, revision, brief, sourceRepo string) error {
	repos, err := VerifierDeliverables(root, revision, brief)
	if err != nil {
		return err
	}
	for _, repo := range repos {
		if strings.EqualFold(repo, sourceRepo) {
			return nil
		}
	}
	return Refused("immutable tracking brief does not declare the bound deliverable repository")
}

// VerifierRepositoryHead identifies a configured checkout and the commit it has
// checked out, with no claim about how that commit relates to fetched main. It
// is enough to read a brief's declarations from an immutable commit and decide
// whether the work is cross-repository at all; it grants nothing else.
func VerifierRepositoryHead(root string) (repo, revision string, err error) {
	repo, err = verifierOrigin(root)
	if err != nil {
		return
	}
	if !IsAllowedRepo(repo) {
		err = Refused("verifier source outside configured repositories")
		return
	}
	revision, err = verifierGit(root, "rev-parse", "HEAD")
	return
}

// VerifierRepositorySnapshot identifies an offline source at fetched main. It
// grants no execution admission; Prepare/Check still validate the full trees.
// Only a resolved cross-repository association needs it: a same-repository
// brief in a checkout that lags fetched main keeps its legacy representation.
func VerifierRepositorySnapshot(root string) (repo, revision string, err error) {
	repo, revision, err = VerifierRepositoryHead(root)
	if err != nil {
		return
	}
	main, e := verifierGit(root, "rev-parse", "refs/remotes/origin/main")
	if e != nil {
		err = e
		return
	}
	if main != revision {
		err = Refused("verifier queue source is not fetched origin/main")
	}
	return
}

type verifierAssociationRegistry struct {
	Schema string `yaml:"schema"`
	Self   string `yaml:"self"`
	Repos  map[string]struct {
		Repo *string `yaml:"repo"`
	} `yaml:"repos"`
}

func parseVerifierAssociationRegistry(raw string) (*verifierAssociationRegistry, error) {
	var graph verifierAssociationRegistry
	if err := yaml.Unmarshal([]byte(raw), &graph); err != nil || graph.Schema != "graph-repos-v1" {
		return nil, Refused("invalid verifier association registry")
	}
	aliasPattern := regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
	for alias, entry := range graph.Repos {
		if !aliasPattern.MatchString(alias) || (entry.Repo != nil && !ValidRepoSlug(strings.TrimSpace(*entry.Repo))) {
			return nil, Refused("invalid verifier association registry entry")
		}
	}
	if graph.Self != "" {
		if _, ok := graph.Repos[graph.Self]; !ok || !aliasPattern.MatchString(graph.Self) {
			return nil, Refused("invalid verifier association registry self")
		}
	}
	return &graph, nil
}

// VerifierAssociationPass owns one planning pass's local observations. Mutable
// checkout facts are read once per root; committed declarations and registries
// are keyed by root, exact revision and path. Errors also last only this pass.
// It grants no admission, is not concurrent, and must never survive into dispatch.
type VerifierAssociationPass struct {
	roots       map[string]*verifierAssociationRoot
	performance *perfrec.Session
}

type verifierAssociationRoot struct {
	git          *gitcore.Repo
	openErr      error
	headRead     bool
	repo, head   string
	headErr      error
	mainRead     bool
	mainErr      error
	registries   map[string]verifierRegistryResult
	declarations map[verifierDeclarationKey]verifierDeclarationResult
}
type verifierRegistryResult struct {
	graph *verifierAssociationRegistry
	err   error
}
type verifierDeclarationKey struct{ revision, path string }
type verifierDeclarationResult struct {
	repos []string
	err   error
}

func NewVerifierAssociationPass(performance *perfrec.Session) *VerifierAssociationPass {
	return &VerifierAssociationPass{roots: map[string]*verifierAssociationRoot{}, performance: performance}
}

func (p *VerifierAssociationPass) root(root string) *verifierAssociationRoot {
	// Abs is lexical: do not accidentally merge distinct worktrees sharing objects.
	key, err := filepath.Abs(root)
	if err != nil {
		return &verifierAssociationRoot{openErr: err}
	}
	if r, ok := p.roots[key]; ok {
		return r
	}
	defer p.performance.Phase("association-root")()
	g, err := gitcore.Open(key)
	r := &verifierAssociationRoot{git: g, openErr: err,
		registries:   map[string]verifierRegistryResult{},
		declarations: map[verifierDeclarationKey]verifierDeclarationResult{}}
	p.roots[key] = r
	return r
}

func (p *VerifierAssociationPass) Head(root string) (string, string, error) {
	r := p.root(root)
	if r.openErr != nil {
		return "", "", Unverifiable("cannot open verifier source", r.openErr)
	}
	if !r.headRead {
		r.headRead = true
		// Identity is read once per root through the existing scoped Git reader.
		// gitcore handles objects/refs; its local-only remote helper cannot
		// preserve includes or worktree overrides used by fresh admission.
		var err error
		r.repo, err = verifierOrigin(root)
		if err == nil && !IsAllowedRepo(r.repo) {
			err = Refused("verifier source outside configured repositories")
		}
		if err == nil {
			hash, e := r.git.Resolve("HEAD")
			err = e
			if e != nil {
				err = Unverifiable("cannot establish verifier source HEAD", e)
			}
			if e == nil {
				r.head = hash.String()
			}
		}
		r.headErr = err
	}
	return r.repo, r.head, r.headErr
}

func (p *VerifierAssociationPass) Snapshot(root string) (string, string, error) {
	repo, head, err := p.Head(root)
	if err != nil {
		return repo, head, err
	}
	r := p.root(root)
	if !r.mainRead {
		r.mainRead = true
		main, e := r.git.Resolve("refs/remotes/origin/main")
		r.mainErr = e
		if e != nil {
			r.mainErr = Unverifiable("cannot establish verifier source fetched main", e)
		}
		if e == nil && main.String() != head {
			r.mainErr = Refused("verifier queue source is not fetched origin/main")
		}
	}
	return repo, head, r.mainErr
}

func (p *VerifierAssociationPass) Deliverables(root, revision, brief string) ([]string, error) {
	defer p.performance.Phase("association")()
	r := p.root(root)
	if r.openErr != nil {
		return nil, Unverifiable("cannot read verifier association brief", r.openErr)
	}
	// Callers provide an exact commit, never a moving ref as an immutable key.
	if !verifierAssociationSHA.MatchString(revision) {
		return nil, Refused("verifier association requires an exact revision")
	}
	key := verifierDeclarationKey{revision, filepath.ToSlash(brief)}
	if cached, ok := r.declarations[key]; ok {
		return append([]string(nil), cached.repos...), cached.err
	}
	raw, err := r.git.FileAt(revision, key.path)
	var repos []string
	if err != nil {
		err = Unverifiable("cannot read verifier association brief", err)
	} else {
		repos, err = resolveVerifierDeliverables(raw, func() (*verifierAssociationRegistry, error) {
			if cached, ok := r.registries[revision]; ok {
				return cached.graph, cached.err
			}
			defer p.performance.Phase("association-registry")()
			raw, e := r.git.FileAt(revision, "docs/streams/graph-repos.yaml")
			var graph *verifierAssociationRegistry
			if e != nil {
				e = Unverifiable("missing immutable verifier association registry", e)
			} else {
				graph, e = parseVerifierAssociationRegistry(raw)
			}
			r.registries[revision] = verifierRegistryResult{graph, e}
			return graph, e
		})
	}
	r.declarations[key] = verifierDeclarationResult{repos, err}
	return append([]string(nil), repos...), err
}

var verifierAssociationSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)
