package deskkit

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/medici-finance/assay/statusgen/briefdoc"
	"github.com/medici-finance/assay/statusgen/readercaps"
	"github.com/medici-finance/assay/statusgen/workref"
	"gopkg.in/yaml.v3"
)

// LocalWorkCatalog reads one immutable checked-out commit, using the existing
// go-git adapter. It performs no network/credential lookup and never treats an
// edited file, origin basename or fuzzy filename match as owning-root evidence.
func LocalWorkCatalog(root, repository string) (WorkCatalog, error) {
	repo, err := git.PlainOpenWithOptions(root, &git.PlainOpenOptions{DetectDotGit: true, EnableDotGitCommonDir: true})
	if err != nil {
		return WorkCatalog{}, Unverifiable("work source repository unreadable", err)
	}
	head, err := repo.Head()
	if err != nil {
		return WorkCatalog{}, Unverifiable("work source commit unavailable", err)
	}
	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return WorkCatalog{}, Unverifiable("work source commit unreadable", err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return WorkCatalog{}, Unverifiable("work source tree unreadable", err)
	}
	if repository == "" {
		remote, e := repo.Remote("origin")
		if e != nil || len(remote.Config().URLs) != 1 {
			return WorkCatalog{}, Unverifiable("unambiguous work source origin unavailable", e)
		}
		repository, err = RemoteRepoSlug(remote.Config().URLs[0])
		if err != nil {
			return WorkCatalog{}, err
		}
	}
	file, err := tree.File("docs/streams/graph-repos.yaml")
	if err != nil {
		return WorkCatalog{}, Unverifiable("work source graph registry unavailable", err)
	}
	raw, err := file.Contents()
	if err != nil {
		return WorkCatalog{}, Unverifiable("work source graph registry unreadable", err)
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
		return WorkCatalog{}, Unverifiable("source repository has no published owning root", nil)
	}
	var records []WorkRecord
	var bindings []workref.Binding
	err = tree.Files().ForEach(func(file *object.File) error {
		dir, base := path.Split(file.Name)
		if !strings.HasPrefix(dir, "docs/streams/") || !strings.HasPrefix(base, "brief-") || !strings.HasSuffix(base, ".md") || strings.Count(strings.TrimPrefix(dir, "docs/streams/"), "/") != 1 {
			return nil
		}
		raw, e := file.Contents()
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
			return fmt.Errorf("invalid work definition in %s", file.Name)
		}
		if parsed.Cell != declaring.Cell || parsed.Repo != declaring.Repo {
			return fmt.Errorf("foreign work definition in %s", file.Name)
		}
		if dir != "docs/streams/"+parsed.Stream+"/" || !strings.HasPrefix(base, "brief-"+parsed.Number+"-") {
			return fmt.Errorf("work definition path mismatch in %s", file.Name)
		}
		version, e := strconv.ParseUint(fields.Values["version"], 10, 64)
		if e != nil {
			return fmt.Errorf("invalid work version in %s", file.Name)
		}
		subject := BriefSubject{UUID: fields.Values["id"], Version: version, Repository: repository, Commit: head.Hash().String(), Path: file.Name}
		if e := subject.Validate(); e != nil {
			return e
		}
		records = append(records, WorkRecord{Identity: parsed.Identity, Subject: subject, Label: RepoShortLabel(repository)})
		bindings = append(bindings, workref.Binding{Identity: parsed.Identity, Key: subject.UUID})
		return nil
	})
	if err != nil {
		return WorkCatalog{}, Unverifiable("incomplete work source observations", err)
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
	return []readercaps.Observation{{State: readercaps.Current, Advertisement: ReaderCapabilities(ReleaseTagOrDev(), "desk-tools")}}, false
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
