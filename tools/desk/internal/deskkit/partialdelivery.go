package deskkit

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/statusgen/briefdoc"
	"gopkg.in/yaml.v3"
)

// partialdelivery.go — a MERGED PR the brief's own `delivery:` list declares `covers: partial` is
// work in progress, not the brief's delivery (#6751). statusgen reads it that way
// (statusgen/briefdelivery.go): the merged trailer PR is marked Partial, it is never the witness that
// the brief is implemented, and the row stays on Next-up. The desk's represented reconciliation
// (fanoutloop's landed-unreconciled routing, deskdispatch's phantom check) keyed on the `Brief:`
// trailer alone, so the same row read as delivered there and was never dispatched. This file is the
// one shared reading the desk side applies, mirroring statusgen's rules:
//
//   - a claim is `{in: "<alias>#<N>", covers: full|partial, note: ...}`; the alias resolves through
//     the tree's docs/streams/graph-repos.yaml, and only a claim naming THIS repo applies;
//   - a PR the list also declares `full` is the delivery (the list is append-only history);
//   - a partial claim that cannot be resolved (malformed ref, unknown or withheld alias, no
//     registry) is could-not-check. statusgen derives that brief unknown; here it sets NOTHING aside,
//     so the reconciliation keeps the answer it gave before this reading existed (the row is not
//     offered) and the reason is returned for the caller to disclose;
//   - an OPEN PR is never set aside: a declared partial says what merged, not what is in flight.

// deliveryRegistryRel is the alias registry statusgen resolves `delivery:` refs through.
const deliveryRegistryRel = "docs/streams/graph-repos.yaml"

// DeliveryAliases is the alias → owner/name map read from a tree's graph-repos.yaml. A withheld
// alias (repo null or unpublished) maps to "". Why is non-empty when the registry could not be read;
// every partial claim then resolves to could-not-check.
type DeliveryAliases struct {
	repos map[string]string
	why   string
}

// LoadDeliveryAliases reads root's docs/streams/graph-repos.yaml (schema graph-repos-v1).
func LoadDeliveryAliases(root string) DeliveryAliases {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(deliveryRegistryRel)))
	if err != nil {
		if os.IsNotExist(err) {
			return DeliveryAliases{why: "the tree has no " + deliveryRegistryRel + " to resolve the alias against"}
		}
		return DeliveryAliases{why: fmt.Sprintf("the alias registry could not be read (%v)", err)}
	}
	return ParseDeliveryAliases(raw)
}

// ParseDeliveryAliases parses a graph-repos.yaml body.
func ParseDeliveryAliases(raw []byte) DeliveryAliases {
	var doc struct {
		Schema string `yaml:"schema"`
		Repos  map[string]struct {
			Repo        *string `yaml:"repo"`
			Unpublished bool    `yaml:"unpublished"`
		} `yaml:"repos"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return DeliveryAliases{why: fmt.Sprintf("the alias registry could not be parsed (%v)", err)}
	}
	if doc.Schema != "graph-repos-v1" {
		return DeliveryAliases{why: fmt.Sprintf("the alias registry's schema is %q, not graph-repos-v1", doc.Schema)}
	}
	out := DeliveryAliases{repos: make(map[string]string, len(doc.Repos))}
	for alias, e := range doc.Repos {
		repo := ""
		if e.Repo != nil && !e.Unpublished {
			repo = strings.TrimSpace(*e.Repo)
		}
		out.repos[alias] = repo
	}
	return out
}

// resolve answers one `in:` ref against repo: n and whether the alias names repo, or why the
// question cannot be answered.
func (a DeliveryAliases) resolve(ref, repo string) (n int, here bool, why string) {
	alias, num, ok := strings.Cut(strings.TrimSpace(ref), "#")
	if !ok || alias == "" {
		return 0, false, "it is not an <alias>#<N> ref"
	}
	n, err := strconv.Atoi(num)
	if err != nil || n <= 0 {
		return 0, false, "it is not an <alias>#<N> ref"
	}
	if a.repos == nil {
		return 0, false, a.why
	}
	target, known := a.repos[alias]
	switch {
	case !known:
		return 0, false, fmt.Sprintf("alias %q is not in the registry", alias)
	case target == "":
		return 0, false, fmt.Sprintf("alias %q is withheld in this tree's registry", alias)
	}
	return n, strings.EqualFold(target, repo), ""
}

// DeclaredPartialDeliveries returns the numbers of repo's PRs that briefContent's `delivery:` list
// declares `covers: partial` (and not also `full`). why is non-empty when the list cannot be read
// with certainty — an unparseable frontmatter, a malformed list, or a partial claim that does not
// resolve — and the set is then nil: could-not-check sets nothing aside.
func DeclaredPartialDeliveries(briefContent, repo string, aliases DeliveryAliases) (map[int]bool, string) {
	block, _, err := briefdoc.Split(briefContent)
	if err != nil {
		return nil, fmt.Sprintf("the brief's frontmatter could not be read (%v)", err)
	}
	values, err := briefdoc.ParseMapping(block)
	if err != nil {
		return nil, fmt.Sprintf("the brief's frontmatter could not be parsed (%v)", err)
	}
	raw, ok := values["delivery"]
	if !ok || raw == nil {
		return nil, ""
	}
	list, ok := raw.([]interface{})
	if !ok {
		return nil, "the brief's delivery: is not a list"
	}
	full := map[int]bool{}
	partial := map[int]bool{}
	for _, item := range list {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, "a delivery: entry is not a mapping"
		}
		in, _ := m["in"].(string)
		cov, _ := m["covers"].(string)
		cov = strings.TrimSpace(cov)
		if cov != "full" && cov != "partial" {
			continue // not a claim statusgen acts on either
		}
		n, here, why := aliases.resolve(in, repo)
		if why != "" {
			if cov == "partial" {
				return nil, fmt.Sprintf("could-not-check the delivery: claim %q: %s", in, why)
			}
			continue
		}
		if !here {
			continue
		}
		if cov == "full" {
			full[n] = true
		} else {
			partial[n] = true
		}
	}
	var out map[int]bool
	for n := range partial {
		if full[n] {
			continue
		}
		if out == nil {
			out = map[int]bool{}
		}
		out[n] = true
	}
	return out, ""
}

// DropDeclaredPartials returns list without the MERGED PRs whose numbers partial holds, in the
// original order. An OPEN PR is always kept. A nil or empty partial returns list unchanged.
func DropDeclaredPartials(list []RepresentedPR, partial map[int]bool) []RepresentedPR {
	if len(partial) == 0 {
		return list
	}
	out := make([]RepresentedPR, 0, len(list))
	for _, rp := range list {
		if rp.Merged && partial[rp.Number] {
			continue
		}
		out = append(out, rp)
	}
	return out
}
