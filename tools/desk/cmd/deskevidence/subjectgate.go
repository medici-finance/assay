package main

// subjectgate.go — the in-process ruling-subject check on an Evidence landing
// (design point 5 of the ruling-subject bridge).
//
// It runs beside the statusgen --decision-gate run, never in place of it. For a
// landing into a gate: human brief that records a GitHub ruling link, it reads the
// linked comment and the current decision issue through the landing's own forge
// and asks deskkit.RulingSubjectGate whether that ruling is bound to the brief's
// exact work subject. It can only refuse; register gates (R-N) are unchanged.

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/medici-finance/assay/statusgen/briefdoc"
	"github.com/medici-finance/assay/statusgen/contexthandoff"
	"github.com/medici-finance/assay/statusgen/domain"
	"github.com/medici-finance/assay/statusgen/workref"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// rulingSubjectGateFn is the seam cmdEvidence calls; a test stubs it.
var rulingSubjectGateFn = rulingSubjectGateAt

// rulingHumansFn returns the configured human name → login map.
var rulingHumansFn = func() map[string]string { return deskkit.EffectiveConfig().HumanLogins }

var (
	briefPathRe = regexp.MustCompile(`^docs/streams/([^/]+)/brief-([0-9]+)-[^/]*\.md$`)
	// typedBriefTokenRe finds candidate typed brief references (a b- number, with an
	// optional revision); workref decides which of them are references.
	typedBriefTokenRe = regexp.MustCompile(`(?:[A-Za-z0-9_.-]+[:/]){1,3}b-[0-9]+(?:@[0-9]+)?`)
)

// rulingSubjectGateAt judges the brief bytes this landing writes. A brief that is
// not gate: human, or records no GitHub comment permalink as its ruling, is left to
// the statusgen hold. Refusal is deskkit.Refused; could-not-check is Unverifiable.
func rulingSubjectGateAt(fg deskkit.Forge, fr deskkit.ForgeRepo, root, target string, remote, content []byte) error {
	p := path.Clean(filepath.ToSlash(target))
	if !isBriefFile(p) {
		return nil
	}
	block, _, err := briefdoc.Split(string(content))
	if err != nil {
		return nil // no frontmatter: the lint pair owns malformed briefs
	}
	fields, err := briefdoc.ParseFields(block)
	if err != nil {
		return nil
	}
	v := fields.Values
	if err := evidenceSubjectGate(root, fr.Slug(), target, p, v, remote, content); err != nil {
		return err
	}
	if strings.TrimSpace(v["gate"]) != "human" || strings.TrimSpace(v["ruling"]) == "" {
		return nil
	}
	m := deskkit.CommentPermalinkRe.FindStringSubmatch(strings.Trim(strings.TrimSpace(v["ruling"]), `"'`))
	if m == nil || m[3] != "issues" {
		return nil // not a GitHub issue-comment permalink: the statusgen hold judges it
	}
	linkRepo := deskkit.ForgeRepo{Owner: m[1], Name: m[2]}
	issueNo, _ := strconv.Atoi(m[4])
	commentID, _ := strconv.ParseInt(m[5], 10, 64)
	expected, _ := strconv.Atoi(strings.TrimSpace(v["decision-issue"]))
	var version uint64
	if s := strings.TrimSpace(v["version"]); s != "" {
		n, perr := strconv.ParseUint(s, 10, 64)
		if perr != nil || n == 0 {
			return deskkit.Refused(fmt.Sprintf("refused: landing %s: the brief's version: %q is not a positive integer, so it cannot carry a revision-bound ruling", target, s))
		}
		version = n
	}
	declared := strings.TrimSpace(v["brief"])
	if declared == "" {
		bm := briefPathRe.FindStringSubmatch(p)
		if bm == nil {
			return nil
		}
		declared = bm[1] + "/" + bm[2]
	}
	declaring, registry, rerr := rulingSubjectRegistry(root, fr.Slug(), declared, strings.TrimSpace(v["id"]))
	if rerr != nil {
		return deskkit.Unverifiable(fmt.Sprintf("could-not-check: landing %s: the ruling-subject registry", target), rerr)
	}

	in := deskkit.RulingSubjectInput{
		ExpectedRepo:  fr.Slug(),
		ExpectedIssue: expected,
		ExpectedID:    commentID,
		Humans:        rulingHumansFn(),
		Declared:      declared,
		Version:       version,
		Declaring:     declaring,
		Registry:      registry,
	}
	if comments, cerr := fg.ListCommentsTyped(linkRepo, issueNo, deskkit.TargetIssue); cerr == nil {
		for _, c := range comments {
			if c.DatabaseID == commentID {
				in.Comment = &deskkit.RulingCommentObservation{Repo: linkRepo.Slug(), Issue: issueNo, ID: c.DatabaseID,
					Author: c.Author, Body: c.Body, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}
				break
			}
		}
		if in.Comment == nil {
			return deskkit.Refused(fmt.Sprintf("refused: landing %s: the ruling comment %d is not on %s#%d (deleted or never there)", target, commentID, linkRepo.Slug(), issueNo))
		}
	}
	if iss, ierr := fg.GetIssue(linkRepo, issueNo); ierr == nil && iss != nil {
		in.Issue = &deskkit.RulingIssueObservation{Repo: linkRepo.Slug(), Number: iss.Number, Body: iss.Body}
	}
	r := deskkit.RulingSubjectGate(in)
	switch r.State {
	case workref.Bound:
		return nil
	case workref.CouldNotCheck:
		return deskkit.Unverifiable(fmt.Sprintf("could-not-check: landing %s: ruling subject (%s): %s", target, r.Reason, r.Detail), nil)
	default:
		return deskkit.Refused(fmt.Sprintf("refused: landing %s: the recorded ruling does not bind this brief's work subject (%s): %s", target, r.Reason, r.Detail))
	}
}

// rulingSubjectRegistry reads docs/streams/graph-repos.yaml under root into the
// typed-reference registry, with the brief's permanent id as its logical key. The
// declaring root is the registry's self alias, else the alias whose repository is
// slug, else the one published alias. A tree with no registry is resolved under
// the brief's own typed identity, or a local root for an unqualified board id.
func rulingSubjectRegistry(root, slug, declared, permID string) (*workref.Context, workref.Registry, error) {
	var roots []workref.Root
	var declaring *workref.Context
	raw, err := os.ReadFile(filepath.Join(root, "docs", "streams", "graph-repos.yaml"))
	switch {
	case errors.Is(err, os.ErrNotExist):
		ctx := workref.Context{Cell: "local", Repo: "local"}
		if parsed, perr := workref.Parse(declared, workref.TypedBrief); perr == nil && parsed.Cell != "" {
			ctx = workref.Context{Cell: parsed.Cell, Repo: parsed.Repo}
		}
		roots = []workref.Root{{Cell: ctx.Cell, Repo: ctx.Repo}}
		declaring = &ctx
	case err != nil:
		return nil, workref.Registry{}, err
	default:
		var graph struct {
			Schema string `yaml:"schema"`
			Cell   string `yaml:"cell"`
			Self   string `yaml:"self"`
			Repos  map[string]struct {
				Cell        string  `yaml:"cell"`
				Repo        *string `yaml:"repo"`
				Unpublished bool    `yaml:"unpublished"`
			} `yaml:"repos"`
		}
		if err := yaml.Unmarshal(raw, &graph); err != nil || graph.Schema != "graph-repos-v1" {
			return nil, workref.Registry{}, fmt.Errorf("invalid docs/streams/graph-repos.yaml")
		}
		aliases := make([]string, 0, len(graph.Repos))
		for a := range graph.Repos {
			aliases = append(aliases, a)
		}
		sort.Strings(aliases)
		var bySlug, published []string
		cellOf := func(a string) string {
			if c := graph.Repos[a].Cell; c != "" {
				return c
			}
			return graph.Cell
		}
		for _, a := range aliases {
			e := graph.Repos[a]
			roots = append(roots, workref.Root{Cell: cellOf(a), Repo: a})
			if e.Repo != nil && strings.EqualFold(*e.Repo, slug) {
				bySlug = append(bySlug, a)
			}
			if !e.Unpublished {
				published = append(published, a)
			}
		}
		self := graph.Self
		switch {
		case self != "":
		case len(bySlug) == 1:
			self = bySlug[0]
		case len(published) == 1:
			self = published[0]
		default:
			return nil, workref.Registry{}, fmt.Errorf("the alias registry names no declaring alias for %s", slug)
		}
		if _, ok := graph.Repos[self]; !ok {
			return nil, workref.Registry{}, fmt.Errorf("self alias %q is not in the alias registry", self)
		}
		declaring = &workref.Context{Cell: cellOf(self), Repo: self}
	}
	plain, err := workref.NewRegistry(roots, nil)
	if err != nil {
		return nil, workref.Registry{}, err
	}
	if permID == "" {
		return declaring, plain, nil
	}
	r, err := workref.Resolve(declared, declaring, plain, workref.TypedBrief)
	if err != nil {
		return nil, workref.Registry{}, fmt.Errorf("the brief's identity %q does not resolve: %v", declared, err)
	}
	full, err := workref.NewRegistry(roots, []workref.Binding{{Identity: r.Identity, Key: permID}})
	if err != nil {
		return nil, workref.Registry{}, err
	}
	return declaring, full, nil
}

// evidenceSubjectGate is design point 3 at Evidence time. When an Evidence line
// this landing adds names this brief by a typed reference, the approved side is
// rebuilt from the brief being landed (its resolved key and Evidence revision:
// explicit version:, else 1 for brief-v2 and unrevisioned for brief-v1) and the
// presented side is that reference. A landing whose Evidence names work only by
// legacy unprefixed ids is unchanged.
func evidenceSubjectGate(root, slug, target, p string, v map[string]string, remote, content []byte) error {
	added := addedEvidenceLines(remote, content)
	var refs []string
	for _, line := range added {
		refs = append(refs, typedBriefTokenRe.FindAllString(line, -1)...)
	}
	if len(refs) == 0 {
		return nil
	}
	declared := strings.TrimSpace(v["brief"])
	if declared == "" {
		bm := briefPathRe.FindStringSubmatch(p)
		if bm == nil {
			return nil
		}
		declared = bm[1] + "/" + bm[2]
	}
	declaring, registry, err := rulingSubjectRegistry(root, slug, declared, strings.TrimSpace(v["id"]))
	if err != nil {
		return deskkit.Unverifiable(fmt.Sprintf("could-not-check: landing %s: the Evidence subject registry", target), err)
	}
	var version uint64
	if s := strings.TrimSpace(v["version"]); s != "" {
		n, perr := strconv.ParseUint(s, 10, 64)
		if perr != nil || n == 0 {
			return deskkit.Refused(fmt.Sprintf("refused: landing %s: the brief's version: %q is not a positive integer", target, s))
		}
		version = n
	}
	approved, err := workref.BriefSubject(declared, domain.EvidenceRevision(strings.TrimSpace(v["schema"]), version), declaring, registry)
	if err != nil {
		return deskkit.Unverifiable(fmt.Sprintf("could-not-check: landing %s: the brief's identity %q does not resolve", target, declared), err)
	}
	for _, ref := range refs {
		r, rerr := workref.Resolve(ref, declaring, registry, workref.TypedBrief)
		if rerr != nil || r.LogicalKey != approved.LogicalKey {
			continue // another brief, or not a reference: not this record's subject
		}
		_, presented, werr := contexthandoff.ResolveWorkSubject(ref, declaring, registry)
		if werr != nil {
			return deskkit.Unverifiable(fmt.Sprintf("could-not-check: landing %s: the Evidence reference %q", target, ref), werr)
		}
		if cerr := domain.CheckSubject(approved, presented); cerr != nil {
			var refusal *domain.Refusal
			if errors.As(cerr, &refusal) && refusal.State == "could-not-check" {
				return deskkit.Unverifiable(fmt.Sprintf("could-not-check: landing %s: the Evidence names %q, which does not establish revision %d of this brief", target, ref, approved.Revision), cerr)
			}
			return deskkit.Refused(fmt.Sprintf("refused: landing %s: the Evidence names %q, not this brief's subject %s: %v", target, ref, approved, cerr))
		}
	}
	return nil
}

// addedEvidenceLines returns the lines of content's Evidence section that the
// remote brief's Evidence section does not already carry.
func addedEvidenceLines(remote, content []byte) []string {
	section := func(b []byte) []string {
		var out []string
		in := false
		for _, line := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
			if strings.HasPrefix(line, "## ") {
				in = strings.TrimSpace(line) == "## Evidence"
				continue
			}
			if in {
				out = append(out, line)
			}
		}
		return out
	}
	have := map[string]bool{}
	for _, l := range section(remote) {
		have[l] = true
	}
	var added []string
	for _, l := range section(content) {
		if !have[l] {
			added = append(added, l)
		}
	}
	return added
}
