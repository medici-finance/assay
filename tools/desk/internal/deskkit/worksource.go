package deskkit

import (
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/statusgen/briefdoc"
	"github.com/medici-finance/assay/statusgen/readercaps"
	"github.com/medici-finance/assay/statusgen/workref"
	"gopkg.in/yaml.v3"
)

// ErrNoPublishedOwningRoot is the cause of the refusal a work-source read gives when the
// repository's graph registry publishes no owning root for it. It is the one cause a
// caller that does not need a catalog (a progress write to a claim that carries no
// negotiated subject) may set aside; every other work-source failure stays fail-closed.
var ErrNoPublishedOwningRoot = errors.New("source repository has no published owning root")

// NoPublishedOwningRoot is the unverifiable refusal carrying ErrNoPublishedOwningRoot.
func NoPublishedOwningRoot() *DeskError {
	return Unverifiable("work source registry", ErrNoPublishedOwningRoot)
}

// LocalWorkCatalog reads one immutable checked-out commit, using the existing
// in-process git adapter (gitcore). It performs no network/credential lookup and never
// treats an edited file, origin basename or fuzzy filename match as owning-root evidence.
//
// The repository is opened through gitcore.Open, never go-git's PlainOpen directly:
// PlainOpen refuses every repository whose config carries extensions.worktreeConfig
// (go-git lowercases the extension name before checking its own mixed-case allowlist), and
// git sets exactly that extension wherever per-worktree config is in use (#6606). gitcore
// tolerates that one git-native extension and still refuses any other unsupported one, so
// an unreadable repository format stays fail-closed.
func LocalWorkCatalog(root, repository string) (WorkCatalog, error) {
	top, err := gitcore.Toplevel(root)
	if err != nil {
		return WorkCatalog{}, Unverifiable("work source repository unreadable", err)
	}
	repo, err := gitcore.Open(top)
	if err != nil {
		return WorkCatalog{}, Unverifiable("work source repository unreadable", err)
	}
	head, err := repo.HeadID()
	if err != nil {
		return WorkCatalog{}, Unverifiable("work source commit unavailable", err)
	}
	names, err := repo.Files(head)
	if err != nil {
		return WorkCatalog{}, Unverifiable("work source tree unreadable", err)
	}
	if repository == "" {
		// The slug decides which repository's work this is, so the read refuses when
		// config it does not merge could change origin as native git resolves it.
		urls, e := repo.ScopedRemoteURLs("origin")
		if errors.Is(e, gitcore.ErrRemoteScopeUnmerged) {
			return WorkCatalog{}, Unverifiable("work source origin could be changed by worktree-scope or include config", e)
		}
		if e != nil || len(urls) != 1 {
			return WorkCatalog{}, Unverifiable("unambiguous work source origin unavailable", e)
		}
		repository, err = RemoteRepoSlug(urls[0])
		if err != nil {
			return WorkCatalog{}, err
		}
	}
	raw, err := repo.FileAt(head, "docs/streams/graph-repos.yaml")
	if err != nil {
		return WorkCatalog{}, Unverifiable("work source graph registry unavailable", err)
	}
	var graph struct {
		Schema string `yaml:"schema"`
		Cell   string `yaml:"cell"`
		Repos  map[string]struct {
			Cell string  `yaml:"cell"`
			Repo *string `yaml:"repo"`
		} `yaml:"repos"`
	}
	if err := yaml.Unmarshal([]byte(raw), &graph); err != nil || graph.Schema != "graph-repos-v1" {
		return WorkCatalog{}, Refused("invalid work source graph registry")
	}
	var roots []workref.Root
	var declaring workref.Context
	for alias, entry := range graph.Repos {
		cell := entry.Cell
		if cell == "" {
			cell = graph.Cell
		}
		roots = append(roots, workref.Root{Cell: cell, Repo: alias})
		if entry.Repo != nil && *entry.Repo == repository {
			if declaring.Repo != "" {
				return WorkCatalog{}, Refused("ambiguous source repository aliases")
			}
			declaring = workref.Context{Cell: cell, Repo: alias}
		}
	}
	if declaring.Repo == "" {
		return WorkCatalog{}, NoPublishedOwningRoot()
	}
	var records []WorkRecord
	var bindings []workref.Binding
	addWork := func(name string) error {
		dir, base := path.Split(name)
		if !strings.HasPrefix(dir, "docs/streams/") || !strings.HasPrefix(base, "brief-") || !strings.HasSuffix(base, ".md") || strings.Count(strings.TrimPrefix(dir, "docs/streams/"), "/") != 1 {
			return nil
		}
		raw, e := repo.FileAt(head, name)
		if e != nil {
			return e
		}
		block, _, e := briefdoc.Split(raw)
		if e != nil {
			return e
		}
		fields, e := briefdoc.ParseFields(block)
		if e != nil {
			return e
		}
		identity := fields.Values["brief"]
		if identity == "" {
			return nil
		} // legacy v1 has no qualified executable definition
		parsed, e := workref.Parse(identity, workref.TypedBrief)
		if e != nil || parsed.Cell == "" || parsed.Repo == "" || parsed.Revision != 0 {
			return fmt.Errorf("invalid work definition in %s", name)
		}
		if parsed.Cell != declaring.Cell || parsed.Repo != declaring.Repo {
			return fmt.Errorf("foreign work definition in %s", name)
		}
		if dir != "docs/streams/"+parsed.Stream+"/" || !strings.HasPrefix(base, "brief-"+parsed.Number+"-") {
			return fmt.Errorf("work definition path mismatch in %s", name)
		}
		version, e := strconv.ParseUint(fields.Values["version"], 10, 64)
		if e != nil {
			return fmt.Errorf("invalid work version in %s", name)
		}
		subject := BriefSubject{UUID: fields.Values["id"], Version: version, Repository: repository, Commit: head, Path: name}
		if e := subject.Validate(); e != nil {
			return e
		}
		records = append(records, WorkRecord{Identity: parsed.Identity, Subject: subject, Label: RepoShortLabel(repository)})
		bindings = append(bindings, workref.Binding{Identity: parsed.Identity, Key: subject.UUID})
		return nil
	}
	for _, name := range names {
		if err := addWork(name); err != nil {
			return WorkCatalog{}, Unverifiable("incomplete work source observations", err)
		}
	}
	registry, err := workref.NewRegistry(roots, bindings)
	if err != nil {
		return WorkCatalog{}, Refused("work source registry: " + err.Error())
	}
	return WorkCatalog{Declaring: declaring, Registry: registry, Records: records}, nil
}

// LocalWorkPeers deliberately advertises only delivered legacy operations. A
// host must supply a qualified complete writer census before negotiated effects;
// this change does not activate /10 or /14 by claiming syntax implies support.
func LocalWorkPeers() ([]readercaps.Observation, bool) {
	return []readercaps.Observation{{State: readercaps.Current, Advertisement: ReaderCapabilities(ReleaseTagOrDev(), "desktools")}}, false
}

func CheckLocalWorkTrailer(body []byte, root string) error {
	catalog, err := LocalWorkCatalog(root, "")
	if err != nil {
		return err
	}
	_, selected, err := ResolveTrailerWork(body, catalog)
	if err != nil {
		return err
	}
	for _, work := range selected {
		if err := work.Validate(); err != nil {
			return err
		}
	}
	peers, complete := LocalWorkPeers()
	return CheckWorkOperation("trailer", peers, complete)
}
