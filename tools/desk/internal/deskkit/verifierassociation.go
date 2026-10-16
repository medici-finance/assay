package deskkit

import (
	"gopkg.in/yaml.v3"
	"path/filepath"
	"regexp"
	"strings"
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
	var fm struct {
		Deliverable string `yaml:"deliverable_repo"`
		Homed       string `yaml:"homed-in"`
	}
	lines := strings.Split(string(raw), "\n")
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
	tags := BriefFilesTags(string(raw))
	if fm.Deliverable == "" && fm.Homed == "" && len(tags) == 0 {
		return nil, nil
	}
	registry, err := verifierGitBytes(root, "cat-file", "blob", revision+":docs/streams/graph-repos.yaml")
	if err != nil {
		return nil, Unverifiable("missing immutable verifier association registry", err)
	}
	var graph struct {
		Schema string `yaml:"schema"`
		Self   string `yaml:"self"`
		Repos  map[string]struct {
			Repo *string `yaml:"repo"`
		} `yaml:"repos"`
	}
	if err := yaml.Unmarshal(registry, &graph); err != nil || graph.Schema != "graph-repos-v1" {
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
