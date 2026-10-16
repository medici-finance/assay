package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// crossRepoRegistryRel is the alias registry (graph-repos-v1) under --root — the same file
// deskdispatch resolves every repo alias through.
const crossRepoRegistryRel = "docs/streams/graph-repos.yaml"

// requireCrossRepoBrief is requireTrailer's check for a `Cross-repo-brief: <alias>:<stream>/<NN>`
// trailer (medici-finance/assay#2748): a PR in THIS repo delivering a brief homed in the repo the
// alias names. The brief is not in this tree, so it is never resolved to a file here (that is the
// refusal the kind exists to end). Instead the value must have the one explicit grammar, and the
// alias must be registered in --root's alias registry and must not be the registry's own `self`
// — a brief homed HERE is a plain `Brief:` line, whose meaning this kind leaves unchanged. Every
// failure is an exit-5 refusal; an absent or unreadable registry refuses too, since the alias
// cannot then be checked at all.
func requireCrossRepoBrief(value, root string) error {
	alias, _, _, ok := deskkit.SplitCrossRepoBriefTrailer(value)
	if !ok {
		return deskkit.Refused(fmt.Sprintf("refused: `Cross-repo-brief: %s` is not of the form "+
			"<alias>:<stream>/<NN> (a lowercase registry alias, then the brief id in its home repo)", value))
	}
	p := filepath.Join(root, filepath.FromSlash(crossRepoRegistryRel))
	raw, err := os.ReadFile(p)
	if err != nil {
		return deskkit.Refused(fmt.Sprintf("refused: `Cross-repo-brief: %s` names alias %q, but there is no alias "+
			"registry readable at %s (%v), so the alias cannot be checked", value, alias, p, err))
	}
	var reg struct {
		Self  string               `yaml:"self"`
		Repos map[string]yaml.Node `yaml:"repos"`
	}
	if err := yaml.Unmarshal(raw, &reg); err != nil {
		return deskkit.Refused(fmt.Sprintf("refused: the alias registry %s does not parse (%v), so `Cross-repo-brief: %s` "+
			"cannot be checked", p, err, value))
	}
	if _, known := reg.Repos[alias]; !known {
		return deskkit.Refused(fmt.Sprintf("refused: `Cross-repo-brief: %s` names alias %q, which is not registered in %s",
			value, alias, p))
	}
	if alias == reg.Self {
		return deskkit.Refused(fmt.Sprintf("refused: `Cross-repo-brief: %s` names %q, this repo's own alias in %s — a "+
			"brief homed here is linked with a `Brief:` trailer", value, alias, p))
	}
	return nil
}
